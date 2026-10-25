// azerothcore-websignup: a small, self-contained sign-up site for an
// AzerothCore (WotLK 3.3.5a) server. Configuration comes from environment
// variables; see README.md and .env.example.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kamakazedan/azerothcore-websignup/internal/game"
	"github.com/kamakazedan/azerothcore-websignup/internal/pow"
	"github.com/kamakazedan/azerothcore-websignup/internal/site"
)

// env reads a setting. Set but empty counts as not set, so a Portainer
// stack can pass every setting through as ${NAME} and still get the
// defaults for the ones left blank.
func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func envInt(key string, def int) int {
	if v := env(key, ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		slog.Warn("not a number, using the default", "setting", key, "value", v, "default", def)
	}
	return def
}

func envBool(key string, def bool) bool {
	switch strings.ToLower(env(key, "")) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func envList(key string) []string {
	var out []string
	for _, s := range strings.Split(env(key, ""), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// secretFromEnv reads a secret from KEY, or from the file KEY_FILE names
// (Docker secrets).
func secretFromEnv(key string) (string, error) {
	if f := env(key+"_FILE", ""); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	}
	return os.Getenv(key), nil
}

// healthcheck asks the running server whether it is healthy, for Docker's
// HEALTHCHECK: the image has no shell or curl, so the binary does it.
func healthcheck() int {
	host, port, err := net.SplitHostPort(env("LISTEN_ADDR", ":8080"))
	if err != nil {
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, resp.Status)
		return 1
	}
	return 0
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-healthcheck" || os.Args[1] == "--healthcheck") {
		os.Exit(healthcheck())
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)
	if err := run(log); err != nil {
		log.Error("stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	demo := envBool("DEMO_MODE", false)

	proxies, err := site.ParseProxies(env("TRUSTED_PROXIES", "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.1/32,::1/128"))
	if err != nil {
		return fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}

	rules := game.Rules{
		MinPassword: envInt("MIN_PASSWORD_LENGTH", 8),
		BotPrefix:   env("BOT_ACCOUNT_PREFIX", "RNDBOT"),
	}

	var store game.Store
	if demo {
		log.Warn("DEMO MODE: no database is used and no account is really created")
		store = game.NewDemo()
	} else {
		password, err := secretFromEnv("DB_PASSWORD")
		if err != nil {
			return fmt.Errorf("DB_PASSWORD_FILE: %w", err)
		}
		if password == "" {
			return errors.New("DB_PASSWORD is not set (see .env.example)")
		}
		db, err := game.OpenMySQL(
			net.JoinHostPort(env("DB_HOST", "ac-database"), env("DB_PORT", "3306")),
			env("DB_USER", "websignup"), password, env("DB_TLS", "preferred"))
		if err != nil {
			return err
		}
		defer db.Close()
		mysqlStore, err := game.NewMySQL(db, game.Options{
			AuthDB:           env("AUTH_DB", "acore_auth"),
			CharactersDB:     env("CHARACTERS_DB", "acore_characters"),
			RealmID:          envInt("REALM_ID", 1),
			Expansion:        envInt("EXPANSION", 2),
			BotPrefix:        rules.BotPrefix,
			HideAccounts:     envList("HIDE_ACCOUNTS"),
			AllowSharedEmail: envBool("ALLOW_SHARED_EMAIL", false),
			OnlineLimit:      envInt("ONLINE_LIST_SIZE", 50),
			TopLimit:         envInt("TOP_LIST_SIZE", 10),
		})
		if err != nil {
			return err
		}
		store = mysqlStore

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := store.Ping(ctx); err != nil {
			log.Warn("database not reachable yet; the site starts anyway and keeps trying", "err", err)
		}
		cancel()
	}

	key := []byte(env("SECRET_KEY", ""))
	if len(key) == 0 {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return err
		}
	}
	bits := envInt("POW_DIFFICULTY", 18)
	if bits < 0 {
		bits = 0
	}
	issuer := pow.New(key, uint8(min(bits, 30)), 10*time.Minute)

	name := env("SITE_NAME", env("REALM_NAME", "AzerothCore"))
	cfg := site.Config{
		SiteName:           name,
		Tagline:            env("TAGLINE", "Northrend awaits. Your legend begins here."),
		Realmlist:          env("REALMLIST", ""),
		DownloadURL:        env("CLIENT_DOWNLOAD_URL", ""),
		ClientHasRealmlist: envBool("CLIENT_HAS_REALMLIST", true),
		GameVersion:        env("GAME_VERSION", "3.3.5a (12340)"),
		PublicURL:          strings.TrimRight(env("PUBLIC_URL", ""), "/"),
		Registration:       envBool("REGISTRATION", true),
		ChangePassword:     envBool("CHANGE_PASSWORD", true),
		ShowOnline:         envBool("SHOW_ONLINE_PLAYERS", true),
		ShowTop:            envBool("SHOW_TOP_PLAYERS", true),
		Rules:              rules,
		WorldAddr:          env("WORLD_ADDR", "ac-worldserver:8085"),
		AuthAddr:           env("AUTH_ADDR", "ac-authserver:3724"),
		ContentDir:         env("CONTENT_DIR", "/content"),
		ThemeDir:           env("THEME_DIR", "/theme"),
		Proxies:            proxies,
		AccountsPerIPDay:   envInt("ACCOUNTS_PER_IP_PER_DAY", 3),
		AIPlayers:          envInt("AI_PLAYERS", 0),
		HSTS:               envBool("HSTS", false),
		Demo:               demo,
	}
	srv, err := site.New(cfg, store, issuer, log)
	if err != nil {
		return err
	}

	addr := env("LISTEN_ADDR", ":8080")
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				srv.Housekeeping()
			}
		}
	}()

	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr, "site", name, "registration", cfg.Registration, "ai_players", cfg.AIPlayers)
		errc <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdown)
}

// Package site is the web front end: one page, a few JSON endpoints, and
// the static files they use, all built into the binary.
package site

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kamakazedan/azerothcore-websignup/internal/game"
	"github.com/kamakazedan/azerothcore-websignup/internal/pow"
	"github.com/kamakazedan/azerothcore-websignup/internal/ratelimit"
)

//go:embed templates static content
var files embed.FS

// Config is what the page says and allows.
type Config struct {
	SiteName           string // shown in the header and title; the realm's name by default
	Tagline            string
	Realmlist          string // set realmlist ...
	DownloadURL        string // a ready-to-play client, if there is one
	ClientHasRealmlist bool   // that client already points at this realm
	GameVersion        string
	PublicURL          string // https://wow.example.com, for links shared elsewhere
	DiscordURL         string // an invite to the community's Discord, if there is one
	Registration       bool
	ChangePassword     bool
	ShowOnline         bool
	ShowTop            bool
	Rules              game.Rules
	WorldAddr          string // ac-worldserver:8085, to tell whether the realm is up
	AuthAddr           string // ac-authserver:3724
	ContentDir         string
	ThemeDir           string // your own backgrounds and particles; see theme.go
	Proxies            Proxies
	AccountsPerIPDay   int
	Playerbots         bool // the server runs playerbots; off, the page never mentions bots or AI players
	AIPlayers          int  // AI players to mention when the realm counts none online (AI_PLAYERS)
	HSTS               bool
	Demo               bool
}

type Server struct {
	cfg    Config
	store  game.Store
	pow    *pow.Issuer
	log    *slog.Logger
	page   *template.Template
	pages  *content
	theme  *themeDir
	static http.Handler
	asset  string            // version stamp for static URLs
	etags  map[string]string // static file -> ETag
	public string            // host part of PublicURL, also accepted as an Origin

	// Rate limits, per client address (an IPv6 /64) unless noted.
	limChallenge  *ratelimit.Limiter
	limRegister   *ratelimit.Limiter // attempts
	limCreated    *ratelimit.Limiter // accounts made
	limCreatedAll *ratelimit.Limiter // accounts made, whole site
	limPassword   *ratelimit.Limiter // attempts
	limBadLogin   *ratelimit.Limiter // failed old passwords, per username

	statusMu   sync.Mutex
	status     statusView
	statusTime time.Time

	healthMu  sync.Mutex
	healthAt  time.Time
	healthErr error
}

func New(cfg Config, store game.Store, issuer *pow.Issuer, log *slog.Logger) (*Server, error) {
	tmpl, err := template.New("index.html").Funcs(template.FuncMap{
		"commas": commas,
		"dict":   dict,
	}).ParseFS(files, "templates/index.html")
	if err != nil {
		return nil, err
	}
	static, err := fs.Sub(files, "static")
	if err != nil {
		return nil, err
	}
	defaults, err := fs.Sub(files, "content")
	if err != nil {
		return nil, err
	}

	if !cfg.Playerbots {
		cfg.Rules.BotPrefix = "" // no account is a bot
	}
	var public string
	if cfg.PublicURL != "" {
		if u, err := url.Parse(cfg.PublicURL); err == nil {
			public = u.Host
		}
	}
	asset, etags := hashFS(static)

	s := &Server{
		cfg:   cfg,
		store: store,
		pow:   issuer,
		log:   log,
		page:  tmpl,
		pages: newContent(defaults, cfg.ContentDir, map[string]string{
			"REALM":        cfg.SiteName,
			"REALMLIST":    cfg.Realmlist,
			"DOWNLOAD_URL": cfg.DownloadURL,
			"DISCORD_URL":  cfg.DiscordURL,
			"GAME_VERSION": cfg.GameVersion,
		}, !cfg.Playerbots),
		theme: newThemeDir(cfg.ThemeDir, log,
			"/static/img/background.jpg?v="+asset, "/static/img/background-phone.jpg?v="+asset),
		static: http.FileServer(http.FS(static)),
		asset:  asset,
		etags:  etags,
		public: public,

		limChallenge:  ratelimit.New(60, 10*time.Minute),
		limRegister:   ratelimit.New(20, time.Hour),
		limCreated:    ratelimit.New(max(cfg.AccountsPerIPDay, 1), 24*time.Hour),
		limCreatedAll: ratelimit.New(60, time.Hour),
		limPassword:   ratelimit.New(10, 15*time.Minute),
		limBadLogin:   ratelimit.New(5, 15*time.Minute),
	}
	return s, nil
}

// Handler is the whole site.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handlePage)
	mux.Handle("GET /static/", http.StripPrefix("/static/", s.staticHandler()))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		if th, _, _ := s.theme.get(); th.Favicon != "" {
			http.Redirect(w, r, th.Favicon, http.StatusFound)
			return
		}
		http.Redirect(w, r, "/static/img/favicon.svg?v="+s.asset, http.StatusFound)
	})
	mux.HandleFunc("GET /theme.css", s.serveThemeCSS)
	mux.HandleFunc("GET /theme/{name}", s.serveThemeFile)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/live", s.handleLive)
	mux.HandleFunc("GET /api/challenge", s.handleChallenge)
	mux.HandleFunc("POST /api/register", s.handleRegister)
	mux.HandleFunc("POST /api/password", s.handlePassword)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})
	return s.secure(mux)
}

// Housekeeping drops rate-limit and challenge state that has expired.
func (s *Server) Housekeeping() {
	s.pow.Forget()
	for _, l := range []*ratelimit.Limiter{s.limChallenge, s.limRegister, s.limCreated, s.limCreatedAll, s.limPassword, s.limBadLogin} {
		l.Prune()
	}
}

// --- headers -----------------------------------------------------------------

const csp = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; " +
	"connect-src 'self'; worker-src 'self'; manifest-src 'self'; frame-ancestors 'none'; base-uri 'none'; " +
	"form-action 'self'; object-src 'none'"

func (s *Server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if s.cfg.HSTS {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) staticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") || r.URL.Path == "" {
			http.NotFound(w, r)
			return
		}
		if t, ok := fontTypes[path.Ext(r.URL.Path)]; ok {
			w.Header().Set("Content-Type", t)
		}
		if r.URL.Query().Get("v") == s.asset {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=300")
		}
		// Built-in files have no modification time, so an ETag is what lets
		// a browser revalidate (fonts the stylesheet loads, say) with a 304.
		if tag, ok := s.etags[strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")]; ok {
			w.Header().Set("ETag", tag)
		}
		s.static.ServeHTTP(w, r)
	})
}

// Go's own list of types has none for fonts, and the container image has
// no /etc/mime.types to fill the gap.
var fontTypes = map[string]string{".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf"}

// hashFS returns a stamp for the whole tree and an ETag per file.
func hashFS(fsys fs.FS) (string, map[string]string) {
	all := sha256.New()
	tags := map[string]string{}
	var names []string
	fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, p)
		}
		return nil
	})
	sort.Strings(names)
	for _, n := range names {
		b, _ := fs.ReadFile(fsys, n)
		sum := sha256.Sum256(b)
		tags[n] = `"` + hex.EncodeToString(sum[:8]) + `"`
		all.Write([]byte(n))
		all.Write(sum[:])
	}
	return hex.EncodeToString(all.Sum(nil))[:12], tags
}

// --- the page ----------------------------------------------------------------

type pageData struct {
	Cfg         Config
	Theme       *Theme
	Asset       string
	Content     map[string]template.HTML
	Status      statusView
	MinPassword int
	Year        int
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	th, _, _ := s.theme.get()
	data := pageData{
		Cfg:         s.cfg,
		Theme:       th,
		Asset:       s.asset,
		Content:     s.pages.get(),
		Status:      s.currentStatus(r.Context()),
		MinPassword: min(max(s.cfg.Rules.MinPassword, 4), game.MaxPassword),
		Year:        time.Now().Year(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if err := s.page.Execute(w, data); err != nil {
		s.log.Error("page", "err", err)
	}
}

// handleHealth says whether the site can reach its database. The answer is
// kept for a few seconds, so asking often doesn't mean pinging often.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	s.healthMu.Lock()
	if time.Since(s.healthAt) > 5*time.Second {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 3*time.Second)
		s.healthErr = s.store.Ping(ctx)
		cancel()
		s.healthAt = time.Now()
	}
	err := s.healthErr
	s.healthMu.Unlock()
	if err != nil {
		http.Error(w, "database unreachable", http.StatusServiceUnavailable)
		return
	}
	io.WriteString(w, "ok\n")
}

// --- realm status ------------------------------------------------------------

type statusView struct {
	Known      bool             `json:"known"` // false until the database answered once
	Realm      string           `json:"realm"`
	Type       string           `json:"type"`
	WorldUp    bool             `json:"worldOnline"`
	AuthUp     bool             `json:"authOnline"`
	Uptime     string           `json:"uptime"`
	Players    int              `json:"players"`
	Bots       int              `json:"bots"`
	Population string           `json:"population"`    // as the realm list says it: Low, Medium, High, Offline
	PopKey     string           `json:"populationKey"` // low, medium, high, online or offline, for colour
	Summary    string           `json:"summary"`       // one or two sentences for people
	Online     []game.Character `json:"online"`
	Top        []game.Character `json:"top"`
	CheckedAt  int64            `json:"checkedAt"`
}

const statusTTL = 30 * time.Second

// population puts a number of characters in the world (players and bots)
// in the realm list's words.
func population(online int) (label, key string) {
	switch {
	case online >= 500:
		return "High", "high"
	case online >= 100:
		return "Medium", "medium"
	default:
		return "Low", "low"
	}
}

// describe fills in the words the page shows for the realm's state.
// canSignUp says whether a new account could be made right now: sign-ups
// are open and the database answered.
func (v *statusView) describe(canSignUp bool) {
	switch {
	case !v.WorldUp:
		v.Population, v.PopKey = "Offline", "offline"
		v.Summary = "The realm is offline right now."
		if canSignUp {
			v.Summary += " You can still make an account, ready for when it's back."
		}
	case !v.AuthUp:
		v.Population, v.PopKey = "Offline", "offline"
		v.Summary = "The login server is down, so nobody can log in right now."
	case !v.Known:
		v.Population, v.PopKey = "Online", "online"
		v.Summary = "The realm is up. Player numbers aren't available just now."
	default:
		v.Population, v.PopKey = population(v.Players + v.Bots)
		v.Summary = census(v.Players, v.Bots)
		if v.Uptime != "" {
			v.Summary = "Up for " + v.Uptime + ". " + v.Summary
		}
	}
}

func census(players, bots int) string {
	p, b := count(players, "player", "players"), count(bots, "AI player that never sleeps", "AI players that never sleep")
	switch {
	case players > 0 && bots > 0:
		return p + " online, alongside " + b + "."
	case players > 0:
		return p + " in the world."
	case bots > 0:
		return b + "."
	default:
		return "The world turns, day and night. Your place in it is waiting."
	}
}

func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return commas(n) + " " + many
}

func (s *Server) currentStatus(ctx context.Context) statusView {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if time.Since(s.statusTime) < statusTTL {
		return s.status
	}
	s.statusTime = time.Now()

	// The result is shared, so one visitor leaving mustn't cut it short.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 4*time.Second)
	defer cancel()

	v := s.status // keep the last good lists if the database hiccups
	v.Realm = s.cfg.SiteName
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); v.WorldUp = s.cfg.Demo || game.Reachable(ctx, s.cfg.WorldAddr) }()
	go func() { defer wg.Done(); v.AuthUp = s.cfg.Demo || game.Reachable(ctx, s.cfg.AuthAddr) }()
	snap, err := s.store.Snapshot(ctx)
	wg.Wait()

	if err != nil {
		s.log.Warn("realm status", "err", err)
	} else {
		v.Known = true
		if snap.RealmName != "" {
			v.Realm = snap.RealmName
		}
		v.Type = snap.RealmType
		v.Players, v.Bots = snap.Players, snap.Bots
		// The world never sounds empty: when no bots are counted online
		// (they're between logins, or their accounts aren't recognised),
		// say how many there are.
		if v.Bots == 0 {
			v.Bots = s.cfg.AIPlayers
		}
		if !s.cfg.Playerbots {
			v.Bots = 0
		}
		v.Online, v.Top = snap.Online, snap.Top
		v.Uptime = ""
		if v.WorldUp && !snap.UpSince.IsZero() {
			v.Uptime = game.PlayedTime(time.Since(snap.UpSince))
		}
	}
	if !s.cfg.ShowOnline {
		v.Online = nil
	}
	if !s.cfg.ShowTop {
		v.Top = nil
	}
	if !v.WorldUp {
		v.Players, v.Bots, v.Online = 0, 0, nil
	}
	v.describe(s.cfg.Registration && err == nil)
	v.CheckedAt = time.Now().Unix()
	s.status = v
	return v
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, s.currentStatus(r.Context()))
}

// handleLive is the page's realm status, online list and top list as HTML,
// rendered by the same templates as the page, so the page's script can
// refresh them without a second copy of the markup.
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	th, _, _ := s.theme.get()
	data := pageData{Cfg: s.cfg, Theme: th, Asset: s.asset, Status: s.currentStatus(r.Context())}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := s.page.ExecuteTemplate(w, "live", data); err != nil {
		s.log.Error("live status", "err", err)
	}
}

// --- forms -------------------------------------------------------------------

type reply struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, field, msg string) {
	writeJSON(w, code, reply{OK: false, Field: field, Message: msg})
}

// sameOrigin keeps other sites from posting forms here. A cross-site page
// cannot send JSON with these headers without a CORS preflight, which this
// server never answers; Origin and Sec-Fetch-Site are checked as well.
func (s *Server) sameOrigin(r *http.Request) bool {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		return false
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			return false
		}
		if !strings.EqualFold(u.Host, r.Host) && !(s.public != "" && strings.EqualFold(u.Host, s.public)) {
			return false
		}
	}
	return true
}

func (s *Server) readForm(w http.ResponseWriter, r *http.Request, v any) bool {
	if !s.sameOrigin(r) {
		fail(w, http.StatusForbidden, "", "Please use the form on this page.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "", "Something went wrong sending the form. Please try again.")
		return false
	}
	return true
}

func (s *Server) handleChallenge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	// Only this site's page asks for challenges. Otherwise another site
	// could spend a visitor's allowance with a few image tags.
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		fail(w, http.StatusForbidden, "", "Please use the form on this page.")
		return
	}
	if !s.limChallenge.Allow(limitKey(s.cfg.Proxies.ClientIP(r))) {
		fail(w, http.StatusTooManyRequests, "", "Slow down a little and try again in a few minutes.")
		return
	}
	c, err := s.pow.Issue()
	if err != nil {
		fail(w, http.StatusInternalServerError, "", "Something went wrong. Please try again.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"challenge": c, "bits": s.pow.Bits()})
}

// checkWork verifies the proof of work every form carries.
func (s *Server) checkWork(w http.ResponseWriter, ip, challenge, nonce string) bool {
	if err := s.pow.Verify(challenge, nonce); err != nil {
		s.log.Info("proof of work refused", "ip", ip, "err", err)
		msg := "The anti-bot check failed. Please try again."
		switch {
		case errors.Is(err, pow.ErrExpired) || errors.Is(err, pow.ErrUsed):
			msg = "The page sat open a while. Please try again."
		case errors.Is(err, pow.ErrBusy):
			msg = "The site is very busy. Please try again in a few minutes."
		}
		fail(w, http.StatusBadRequest, "", msg)
		return false
	}
	return true
}

type registerForm struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Password2 string `json:"password2"`
	Website   string `json:"website"` // honeypot: people never see it
	Challenge string `json:"challenge"`
	Nonce     string `json:"nonce"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Registration {
		fail(w, http.StatusForbidden, "", "Sign-ups are closed at the moment.")
		return
	}
	ip := s.cfg.Proxies.ClientIP(r)
	key := limitKey(ip)
	var f registerForm
	if !s.readForm(w, r, &f) {
		return
	}
	if f.Website != "" {
		s.log.Info("sign-up honeypot", "ip", ip)
		fail(w, http.StatusBadRequest, "", "Something went wrong. Please try again.")
		return
	}
	// Mistakes in the form cost nothing to point out, and count against
	// nothing.
	if err := s.cfg.Rules.Username(f.Username); err != nil {
		fieldFail(w, err)
		return
	}
	email, err := game.Email(f.Email)
	if err != nil {
		fieldFail(w, err)
		return
	}
	if err := s.cfg.Rules.Password(f.Username, f.Password); err != nil {
		fieldFail(w, err)
		return
	}
	if f.Password != f.Password2 {
		fail(w, http.StatusBadRequest, "password2", "The two passwords don't match.")
		return
	}
	// From here on each try needs a solved challenge, and is counted.
	if !s.checkWork(w, ip, f.Challenge, f.Nonce) {
		return
	}
	if !s.limRegister.Allow(key) {
		fail(w, http.StatusTooManyRequests, "", "Too many tries. Please wait an hour and try again.")
		return
	}
	// Take this account's place in the limits before making it, so sign-ups
	// sent all at once can't all get in; give it back if it isn't made.
	if !s.limCreated.Allow(key) {
		fail(w, http.StatusTooManyRequests, "", "No more accounts can be made from here today. Ask the server admin if you need one.")
		return
	}
	if !s.limCreatedAll.Allow("*") {
		s.limCreated.Undo(key)
		fail(w, http.StatusTooManyRequests, "", "A lot of accounts have been made in the last hour. Please try again later.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	err = s.store.CreateAccount(ctx, f.Username, f.Password, email)
	if err != nil {
		s.limCreated.Undo(key)
		s.limCreatedAll.Undo("*")
	}
	switch {
	case errors.Is(err, game.ErrUsernameTaken):
		fail(w, http.StatusConflict, "username", "That username is taken. Please pick another.")
	case errors.Is(err, game.ErrEmailTaken):
		fail(w, http.StatusConflict, "email", "That email address already has an account.")
	case err != nil:
		s.log.Error("create account", "user", strings.ToUpper(f.Username), "ip", ip, "err", err)
		fail(w, http.StatusInternalServerError, "", "The account couldn't be created just now. Please try again in a minute.")
	default:
		s.log.Info("account created", "user", strings.ToUpper(f.Username), "ip", ip)
		writeJSON(w, http.StatusOK, reply{OK: true, Message: "Your account " + strings.ToUpper(f.Username) + " is ready."})
	}
}

type passwordForm struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	NewPassword  string `json:"newPassword"`
	NewPassword2 string `json:"newPassword2"`
	Challenge    string `json:"challenge"`
	Nonce        string `json:"nonce"`
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.ChangePassword {
		fail(w, http.StatusForbidden, "", "Password changes are switched off here.")
		return
	}
	ip := s.cfg.Proxies.ClientIP(r)
	key := limitKey(ip)
	var f passwordForm
	if !s.readForm(w, r, &f) {
		return
	}
	user := strings.ToUpper(strings.TrimSpace(f.Username))
	if user == "" || len(user) > 32 || f.Password == "" || len(f.Password) > 64 {
		fail(w, http.StatusBadRequest, "", "Please fill in your username and current password.")
		return
	}
	// Playerbots' own accounts aren't anyone's to change. Unless playerbots
	// is set to give them random passwords, each one's password is its name.
	if p := strings.ToUpper(s.cfg.Rules.BotPrefix); p != "" && strings.HasPrefix(user, p) {
		s.log.Info("password change for a bot account refused", "user", user, "ip", ip)
		fail(w, http.StatusForbidden, "username", "That account can't be changed here.")
		return
	}
	if err := s.cfg.Rules.Password(user, f.NewPassword); err != nil {
		msg := "Please choose a different new password."
		if fe, ok := game.AsFieldError(err); ok {
			msg = fe.Message
		}
		fail(w, http.StatusBadRequest, "newPassword", msg)
		return
	}
	if f.NewPassword != f.NewPassword2 {
		fail(w, http.StatusBadRequest, "newPassword2", "The two new passwords don't match.")
		return
	}
	if !s.checkWork(w, ip, f.Challenge, f.Nonce) {
		return
	}
	if !s.limPassword.Allow(key) {
		fail(w, http.StatusTooManyRequests, "", "Too many tries. Please wait a few minutes and try again.")
		return
	}
	// Each try counts as a wrong password until it proves right, so tries
	// sent all at once can't get past the limit.
	if !s.limBadLogin.Allow(user) {
		fail(w, http.StatusTooManyRequests, "", "Too many wrong passwords for that account. Please wait 15 minutes.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	switch err := s.store.ChangePassword(ctx, user, f.Password, f.NewPassword); {
	case errors.Is(err, game.ErrBadLogin):
		s.log.Info("password change refused", "user", user, "ip", ip)
		fail(w, http.StatusUnauthorized, "", "That username and password don't match.")
	case err != nil:
		s.limBadLogin.Undo(user)
		s.log.Error("change password", "user", user, "ip", ip, "err", err)
		fail(w, http.StatusInternalServerError, "", "The password couldn't be changed just now. Please try again in a minute.")
	default:
		s.limBadLogin.Reset(user)
		s.log.Info("password changed", "user", user, "ip", ip)
		writeJSON(w, http.StatusOK, reply{OK: true, Message: "Password changed. Use the new one next time you log in."})
	}
}

func fieldFail(w http.ResponseWriter, err error) {
	if fe, ok := game.AsFieldError(err); ok {
		fail(w, http.StatusBadRequest, fe.Field, fe.Message)
		return
	}
	fail(w, http.StatusBadRequest, "", "Please check the form and try again.")
}

// dict builds a map from key, value pairs, to hand several values to a
// template piece.
func dict(kv ...any) (map[string]any, error) {
	if len(kv)%2 != 0 {
		return nil, errors.New("dict wants key, value pairs")
	}
	m := make(map[string]any, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			return nil, errors.New("dict keys must be strings")
		}
		m[k] = kv[i+1]
	}
	return m, nil
}

// commas formats 1214 as "1,214".
func commas(n int) string {
	s := strconv.Itoa(n)
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return sign + b.String()
}

package game

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/kamakazedan/azerothcore-websignup/internal/srp6"
)

var (
	ErrUsernameTaken = errors.New("username taken")
	ErrEmailTaken    = errors.New("email already used")
	ErrBadLogin      = errors.New("wrong username or password")
)

// Store is everything the site needs from the server's databases.
type Store interface {
	CreateAccount(ctx context.Context, username, password, email string) error
	ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error
	Snapshot(ctx context.Context) (Snapshot, error)
	Ping(ctx context.Context) error
}

// Snapshot is the realm as the site shows it.
type Snapshot struct {
	RealmName string      `json:"realm"`
	RealmType string      `json:"type"` // PvE, PvP, RP or RP-PvP
	UpSince   time.Time   `json:"-"`
	Players   int         `json:"players"`
	Bots      int         `json:"bots"`
	Online    []Character `json:"online"`
	Top       []Character `json:"top"`
}

// Options for the MySQL store.
type Options struct {
	AuthDB           string // acore_auth
	CharactersDB     string // acore_characters
	RealmID          int
	Expansion        int    // 2 = Wrath
	BotPrefix        string // account names playerbots owns
	HideAccounts     []string
	AllowSharedEmail bool
	OnlineLimit      int // 0 hides the list
	TopLimit         int // 0 hides the list
}

type MySQL struct {
	db   *sql.DB
	opt  Options
	auth string // `acore_auth`
	char string // `acore_characters`
	bots string // LIKE pattern for bot accounts

	// create makes one account at a time. The account table has no unique
	// index on email, so without this two sign-ups sent together could both
	// pass the "email already used" check.
	create sync.Mutex
}

var identRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// NewMySQL wraps an open database. Schema names are checked here because
// they end up in the SQL text (only values can be query parameters).
func NewMySQL(db *sql.DB, opt Options) (*MySQL, error) {
	if !identRe.MatchString(opt.AuthDB) || !identRe.MatchString(opt.CharactersDB) {
		return nil, fmt.Errorf("database names may only use letters, digits and _")
	}
	return &MySQL{
		db:   db,
		opt:  opt,
		auth: "`" + opt.AuthDB + "`",
		char: "`" + opt.CharactersDB + "`",
		bots: likePrefix(opt.BotPrefix),
	}, nil
}

// likePrefix makes a LIKE pattern matching names that start with p. An empty
// prefix matches nothing, so no account counts as a bot.
func likePrefix(p string) string {
	if p == "" {
		return "\x00never\x00"
	}
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(p) + "%"
}

// OpenMySQL connects with settings that suit a small web app.
func OpenMySQL(addr, user, password, tlsMode string) (*sql.DB, error) {
	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = addr
	cfg.User = user
	cfg.Passwd = password
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 10 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	cfg.Collation = "utf8mb4_unicode_ci"
	cfg.Loc = time.UTC
	// MySQL 8's default login (caching_sha2_password) needs either TLS or the
	// server's RSA key the first time a user logs in after a restart. The
	// official image has TLS on with its own certificate, so "preferred"
	// uses that; without TLS the driver asks the server for its key.
	cfg.TLSConfig = tlsMode

	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

func (s *MySQL) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// CreateAccount adds an account the way AccountMgr::CreateAccount does:
// uppercased name and email, a fresh salt and verifier, the server's
// expansion, then a realmcharacters row per realm. Inputs must already have
// passed Rules.Username, Rules.Password and Email.
func (s *MySQL) CreateAccount(ctx context.Context, username, password, email string) error {
	username = strings.ToUpper(username)
	s.create.Lock()
	defer s.create.Unlock()

	var one int
	err := s.db.QueryRowContext(ctx,
		"SELECT 1 FROM "+s.auth+".`account` WHERE `username` = ? LIMIT 1", username).Scan(&one)
	if err == nil {
		return ErrUsernameTaken
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if !s.opt.AllowSharedEmail {
		err := s.db.QueryRowContext(ctx,
			"SELECT 1 FROM "+s.auth+".`account` WHERE `email` = ? OR `reg_mail` = ? LIMIT 1", email, email).Scan(&one)
		if err == nil {
			return ErrEmailTaken
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}

	salt, verifier, err := srp6.New(username, password)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		"INSERT INTO "+s.auth+".`account` (`username`, `salt`, `verifier`, `expansion`, `reg_mail`, `email`, `joindate`) "+
			"VALUES (?, ?, ?, ?, ?, ?, NOW())",
		username, salt, verifier, s.opt.Expansion, email, email)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 { // duplicate key: someone was quicker
			return ErrUsernameTaken
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT IGNORE INTO "+s.auth+".`realmcharacters` (`realmid`, `acctid`, `numchars`) "+
			"SELECT `id`, ?, 0 FROM "+s.auth+".`realmlist`", id); err != nil {
		return err
	}
	return tx.Commit()
}

// ChangePassword sets a new salt and verifier, as `.account password` does,
// once the old password checks out.
func (s *MySQL) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	username = strings.ToUpper(username)

	var (
		id               uint32
		salt, verifierDB []byte
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT `id`, `salt`, `verifier` FROM "+s.auth+".`account` WHERE `username` = ?", username).
		Scan(&id, &salt, &verifierDB)
	if errors.Is(err, sql.ErrNoRows) {
		// Same work as a real check, so a missing account takes as long.
		srp6.Check(username, oldPassword, make([]byte, srp6.SaltLength), make([]byte, srp6.VerifierLength))
		return ErrBadLogin
	}
	if err != nil {
		return err
	}
	if !srp6.Check(username, oldPassword, salt, verifierDB) {
		return ErrBadLogin
	}

	newSalt, newVerifier, err := srp6.New(username, newPassword)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		"UPDATE "+s.auth+".`account` SET `salt` = ?, `verifier` = ? WHERE `id` = ?", newSalt, newVerifier, id)
	return err
}

// hidden returns " AND a.username NOT IN (?, ...)" and its arguments.
func (s *MySQL) hidden() (string, []any) {
	if len(s.opt.HideAccounts) == 0 {
		return "", nil
	}
	marks := make([]string, len(s.opt.HideAccounts))
	args := make([]any, len(s.opt.HideAccounts))
	for i, a := range s.opt.HideAccounts {
		marks[i] = "?"
		args[i] = strings.ToUpper(a)
	}
	return " AND a.`username` NOT IN (" + strings.Join(marks, ", ") + ")", args
}

func (s *MySQL) Snapshot(ctx context.Context) (Snapshot, error) {
	var snap Snapshot

	var icon int
	err := s.db.QueryRowContext(ctx,
		"SELECT `name`, `icon` FROM "+s.auth+".`realmlist` WHERE `id` = ?", s.opt.RealmID).Scan(&snap.RealmName, &icon)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return snap, err
	}
	snap.RealmType = RealmType(icon)

	var start sql.NullInt64
	err = s.db.QueryRowContext(ctx,
		"SELECT `starttime` FROM "+s.auth+".`uptime` WHERE `realmid` = ? ORDER BY `starttime` DESC LIMIT 1",
		s.opt.RealmID).Scan(&start)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return snap, err
	}
	if start.Valid && start.Int64 > 0 {
		snap.UpSince = time.Unix(start.Int64, 0)
	}

	join := " FROM " + s.char + ".`characters` c JOIN " + s.auth + ".`account` a ON a.`id` = c.`account`"
	hide, hideArgs := s.hidden()

	var players, bots sql.NullInt64
	err = s.db.QueryRowContext(ctx,
		"SELECT SUM(a.`username` NOT LIKE ?), SUM(a.`username` LIKE ?)"+join+" WHERE c.`online` = 1",
		s.bots, s.bots).Scan(&players, &bots)
	if err != nil {
		return snap, err
	}
	snap.Players, snap.Bots = int(players.Int64), int(bots.Int64)

	if s.opt.OnlineLimit > 0 {
		args := append([]any{s.bots}, hideArgs...)
		args = append(args, s.opt.OnlineLimit)
		snap.Online, err = s.characters(ctx,
			"SELECT c.`name`, c.`race`, c.`class`, c.`level`, c.`totaltime`"+join+
				" WHERE c.`online` = 1 AND a.`username` NOT LIKE ?"+hide+
				" ORDER BY c.`level` DESC, c.`name` LIMIT ?", false, args...)
		if err != nil {
			return snap, err
		}
	}

	if s.opt.TopLimit > 0 {
		args := append([]any{s.bots}, hideArgs...)
		args = append(args, s.opt.TopLimit)
		snap.Top, err = s.characters(ctx,
			"SELECT c.`name`, c.`race`, c.`class`, c.`level`, c.`totaltime`"+join+
				" WHERE a.`username` NOT LIKE ?"+hide+
				" ORDER BY c.`level` DESC, c.`totaltime` DESC LIMIT ?", true, args...)
		if err != nil {
			return snap, err
		}
	}
	return snap, nil
}

func (s *MySQL) characters(ctx context.Context, query string, played bool, args ...any) ([]Character, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Character{}
	for rows.Next() {
		var (
			name        string
			race, class uint8
			level       int
			total       uint32
		)
		if err := rows.Scan(&name, &race, &class, &level, &total); err != nil {
			return nil, err
		}
		out = append(out, newCharacter(name, race, class, level, total, played))
	}
	return out, rows.Err()
}

// Reachable reports whether something accepts TCP connections at addr.
func Reachable(ctx context.Context, addr string) bool {
	if addr == "" {
		return false
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

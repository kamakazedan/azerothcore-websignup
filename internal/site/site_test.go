package site

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kamakazedan/azerothcore-websignup/internal/game"
	"github.com/kamakazedan/azerothcore-websignup/internal/pow"
)

const testBits = 6

func testConfig() Config {
	proxies, _ := ParseProxies("10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.1")
	return Config{
		SiteName:         "Frosthold",
		Tagline:          "A test realm.",
		Realmlist:        "wow.example.com",
		DownloadURL:      "https://example.com/wow.7z",
		GameVersion:      "3.3.5a (12340)",
		PublicURL:        "https://signup.example.com",
		Registration:     true,
		ChangePassword:   true,
		ShowOnline:       true,
		ShowTop:          true,
		Playerbots:       true,
		Rules:            game.Rules{MinPassword: 8, BotPrefix: "RNDBOT"},
		Proxies:          proxies,
		AccountsPerIPDay: 3,
		Demo:             true,
	}
}

func newTestServer(t *testing.T, change func(*Config)) http.Handler {
	t.Helper()
	cfg := testConfig()
	if change != nil {
		change(&cfg)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := New(cfg, game.NewDemo(), pow.New([]byte("test key"), testBits, 10*time.Minute), log)
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

func get(t *testing.T, h http.Handler, target string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// post sends a form the way the page does: JSON, same origin.
func post(t *testing.T, h http.Handler, target string, body any, headers ...string) (*httptest.ResponseRecorder, reply) {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(b))
	req.Host = "example.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	for i := 0; i+1 < len(headers); i += 2 {
		if headers[i+1] == "" {
			req.Header.Del(headers[i])
		} else {
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var r reply
	json.Unmarshal(rec.Body.Bytes(), &r)
	return rec, r
}

// work fetches a challenge and solves it.
func work(t *testing.T, h http.Handler) (string, string) {
	t.Helper()
	rec := get(t, h, "/api/challenge")
	if rec.Code != http.StatusOK {
		t.Fatalf("challenge: %d %s", rec.Code, rec.Body)
	}
	var c struct {
		Challenge string `json:"challenge"`
		Bits      int    `json:"bits"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	for n := 0; ; n++ {
		nonce := strconv.Itoa(n)
		if pow.LeadingZeroBits(c.Challenge, nonce) >= c.Bits {
			return c.Challenge, nonce
		}
	}
}

func registration(t *testing.T, h http.Handler, user, email string) map[string]string {
	ch, nonce := work(t, h)
	return map[string]string{
		"username": user, "email": email, "password": "icecrown25", "password2": "icecrown25",
		"website": "", "challenge": ch, "nonce": nonce,
	}
}

func TestPage(t *testing.T) {
	h := newTestServer(t, nil)
	rec := get(t, h, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<title>Frosthold: create your account</title>",
		`id="register-form"`, `id="password-form"`, `role="tablist"`,
		"set realmlist wow.example.com", `href="https://example.com/wow.7z"`,
		`id="realm-plaque"`, `id="online-list"`, `id="top-list"`,
		"What's running on Frosthold", // content token filled in
		"Brelac", "c-paladin", "Players only, not the 1,214 AI players.",
		`data-min="8"`, "Demo mode",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page is missing %q", want)
		}
	}
	for _, bad := range []string{"{{", "ZgotmplZ", `id="rules"`, "style="} {
		if strings.Contains(body, bad) {
			t.Errorf("page contains %q", bad)
		}
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe") {
		t.Errorf("CSP = %q", csp)
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("security headers missing")
	}
}

func TestPageWithoutPlayerbots(t *testing.T) {
	h := newTestServer(t, func(c *Config) { c.Playerbots = false; c.AIPlayers = 1500 })
	for _, path := range []string{"/", "/api/live", "/api/status"} {
		body := strings.ToLower(strings.ReplaceAll(get(t, h, path).Body.String(), `"bots":0`, ""))
		for _, bad := range []string{"bots", "ai player", "playerbot", "1,214", "1,500"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s mentions %q with playerbots off", path, bad)
			}
		}
	}
}

func TestPageDiscord(t *testing.T) {
	if body := get(t, newTestServer(t, nil), "/").Body.String(); strings.Contains(body, "Discord") {
		t.Error("Discord shows with no DISCORD_URL")
	}
	h := newTestServer(t, func(c *Config) { c.DiscordURL = "https://discord.gg/frost" })
	body := get(t, h, "/").Body.String()
	if n := strings.Count(body, `href="https://discord.gg/frost"`); n != 3 {
		t.Errorf("the invite is linked %d times, want 3 (menu, How to connect, after sign-up)", n)
	}
}

func TestPageWithFormsClosed(t *testing.T) {
	h := newTestServer(t, func(c *Config) { c.Registration, c.ChangePassword = false, false })
	body := get(t, h, "/").Body.String()
	if strings.Contains(body, "<form") || strings.Contains(body, `role="tablist"`) {
		t.Error("closed forms are still on the page")
	}
	if !strings.Contains(body, "Sign-ups are closed") {
		t.Error("no closed notice")
	}

	h = newTestServer(t, func(c *Config) { c.Registration = false })
	body = get(t, h, "/").Body.String()
	if strings.Contains(body, `id="register-form"`) || !strings.Contains(body, `id="password-form"`) {
		t.Error("with sign-ups closed, only the password form should be there")
	}
	if strings.Contains(body, `role="tablist"`) {
		t.Error("a single form needs no tabs")
	}
	_, r := post(t, h, "/api/register", registration(t, h, "Jaina", "jaina@example.com"))
	if r.OK {
		t.Error("registered with sign-ups closed")
	}
}

func TestLiveAndStatus(t *testing.T) {
	h := newTestServer(t, nil)
	live := get(t, h, "/api/live")
	if live.Code != http.StatusOK || !strings.HasPrefix(live.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("live: %d %s", live.Code, live.Header().Get("Content-Type"))
	}
	for _, id := range []string{"realm-plaque", "online-list", "top-list"} {
		if !strings.Contains(live.Body.String(), `id="`+id+`"`) {
			t.Errorf("live fragment has no %s", id)
		}
	}

	rec := get(t, h, "/api/status")
	var st statusView
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Known || st.Population != "High" || st.Type != "PvE" || st.Bots != 1214 {
		t.Errorf("status = %+v", st)
	}
	if !strings.Contains(st.Summary, "3 players online, alongside 1,214 AI players that never sleep.") || !strings.HasPrefix(st.Summary, "Up for ") {
		t.Errorf("summary = %q", st.Summary)
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		v       statusView
		reg     bool
		pop     string
		summary string
	}{
		{statusView{Known: true, WorldUp: true, AuthUp: true, Players: 1, Bots: 0}, true, "Low", "1 player in the world."},
		{statusView{Known: true, WorldUp: true, AuthUp: true, Players: 0, Bots: 150, Uptime: "3h 2m"}, true, "Medium", "Up for 3h 2m. 150 AI players that never sleep."},
		{statusView{Known: true, WorldUp: true, AuthUp: true}, true, "Low", "The world turns, day and night. Your place in it is waiting."},
		{statusView{Known: true, WorldUp: false, AuthUp: true}, true, "Offline", "The realm is offline right now. You can still make an account, ready for when it's back."},
		{statusView{Known: true, WorldUp: false}, false, "Offline", "The realm is offline right now."},
		{statusView{Known: true, WorldUp: true, AuthUp: false}, true, "Offline", "The login server is down, so nobody can log in right now."},
		{statusView{Known: false, WorldUp: true, AuthUp: true}, true, "Online", "The realm is up. Player numbers aren't available just now."},
	}
	for _, c := range cases {
		c.v.describe(c.reg)
		if c.v.Population != c.pop || c.v.Summary != c.summary {
			t.Errorf("describe(%+v) = %q, %q; want %q, %q", c.v, c.v.Population, c.v.Summary, c.pop, c.summary)
		}
	}
}

func TestStatic(t *testing.T) {
	cfg := testConfig()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := New(cfg, game.NewDemo(), pow.New([]byte("k"), 1, time.Minute), log)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	rec := get(t, h, "/static/css/site.css?v="+s.asset)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("css: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Error("a versioned asset should be cached for good")
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}

	rec = get(t, h, "/static/fonts/cinzel.woff")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "font/woff" {
		t.Errorf("font: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Error("an unversioned asset shouldn't be immutable")
	}
	if rec := get(t, h, "/static/fonts/cinzel.woff", "If-None-Match", rec.Header().Get("ETag")); rec.Code != http.StatusNotModified {
		t.Errorf("revalidation = %d, want 304", rec.Code)
	}
	// No directory listings, and nothing outside static/.
	for _, p := range []string{"/static/", "/static/fonts/", "/static/nope.css", "/templates/index.html", "/content/about.html"} {
		if rec := get(t, h, p); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, rec.Code)
		}
	}
	// The router cleans ".." away first: /go.mod, which doesn't exist.
	if rec := get(t, h, "/static/../go.mod"); rec.Code != http.StatusMovedPermanently || get(t, h, rec.Header().Get("Location")).Code != http.StatusNotFound {
		t.Errorf("GET /static/../go.mod = %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := get(t, h, "/static/fonts"); rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "fonts/" {
		t.Errorf("GET /static/fonts = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := get(t, h, "/favicon.ico"); rec.Code != http.StatusFound {
		t.Errorf("favicon = %d", rec.Code)
	}
}

func TestRegister(t *testing.T) {
	h := newTestServer(t, nil)

	form := registration(t, h, "Jaina", "jaina@example.com")
	rec, r := post(t, h, "/api/register", form)
	if rec.Code != http.StatusOK || !r.OK || r.Message != "Your account JAINA is ready." {
		t.Fatalf("register: %d %+v", rec.Code, r)
	}

	// The same solved challenge can't be used twice.
	form["username"], form["email"] = "Thrall", "thrall@example.com"
	if _, r := post(t, h, "/api/register", form); r.OK || !strings.Contains(r.Message, "sat open") {
		t.Errorf("replayed challenge: %+v", r)
	}

	// Taken names and emails, whatever their case.
	if rec, r := post(t, h, "/api/register", registration(t, h, "jAINA", "other@example.com")); rec.Code != http.StatusConflict || r.Field != "username" {
		t.Errorf("taken name: %d %+v", rec.Code, r)
	}
	if rec, r := post(t, h, "/api/register", registration(t, h, "Sylvanas", "JAINA@example.com")); rec.Code != http.StatusConflict || r.Field != "email" {
		t.Errorf("taken email: %d %+v", rec.Code, r)
	}
}

func TestRegisterChecks(t *testing.T) {
	h := newTestServer(t, nil)
	cases := []struct {
		change func(map[string]string)
		field  string
	}{
		{func(f map[string]string) { f["username"] = "ab" }, "username"},
		{func(f map[string]string) { f["username"] = "has space" }, "username"},
		{func(f map[string]string) { f["username"] = "RndBot42" }, "username"},
		{func(f map[string]string) { f["email"] = "not-an-email" }, "email"},
		{func(f map[string]string) { f["password"], f["password2"] = "short", "short" }, "password"},
		{func(f map[string]string) { f["password"], f["password2"] = "seventeen-chars!!", "seventeen-chars!!" }, "password"},
		{func(f map[string]string) { f["password"], f["password2"] = "two words", "two words" }, "password"},
		{func(f map[string]string) {
			f["password"], f["password2"] = "MEDIVH42", "MEDIVH42"
			f["username"] = "medivh42"
		}, "password"},
		{func(f map[string]string) { f["password2"] = "different1" }, "password2"},
	}
	for _, c := range cases {
		form := registration(t, h, "Medivh", "medivh@example.com")
		c.change(form)
		rec, r := post(t, h, "/api/register", form)
		if rec.Code != http.StatusBadRequest || r.Field != c.field || r.OK {
			t.Errorf("%v: got %d %+v, want a %q error", form, rec.Code, r, c.field)
		}
	}

	// The honeypot.
	form := registration(t, h, "Medivh", "medivh@example.com")
	form["website"] = "http://spam.example"
	if _, r := post(t, h, "/api/register", form); r.OK {
		t.Error("honeypot ignored")
	}

	// No work, or wrong work.
	form = registration(t, h, "Medivh", "medivh@example.com")
	form["nonce"] = "x"
	if _, r := post(t, h, "/api/register", form); r.OK {
		t.Error("accepted without proof of work")
	}
	form = registration(t, h, "Medivh", "medivh@example.com")
	form["challenge"] = strings.Replace(form["challenge"], ".", ".A", 1)
	if _, r := post(t, h, "/api/register", form); r.OK {
		t.Error("accepted a forged challenge")
	}
}

func TestSameOriginOnly(t *testing.T) {
	h := newTestServer(t, nil)
	cases := [][]string{
		{"Origin", "https://evil.example"},
		{"Origin", "null"},
		{"Sec-Fetch-Site", "cross-site"},
		{"Content-Type", "text/plain"},
		{"Content-Type", "application/x-www-form-urlencoded"},
	}
	for _, hd := range cases {
		rec, r := post(t, h, "/api/register", registration(t, h, "Uther", "uther@example.com"), hd...)
		if rec.Code != http.StatusForbidden || r.OK {
			t.Errorf("%v: %d %+v", hd, rec.Code, r)
		}
	}
	// The public address is fine even when the proxy rewrites Host.
	rec, r := post(t, h, "/api/register", registration(t, h, "Uther", "uther@example.com"), "Origin", "https://signup.example.com")
	if rec.Code != http.StatusOK || !r.OK {
		t.Errorf("public origin refused: %d %+v", rec.Code, r)
	}
	// Old browsers send neither header; JSON alone still needs a preflight
	// from another site, so that's allowed.
	rec, r = post(t, h, "/api/register", registration(t, h, "Tirion", "tirion@example.com"), "Origin", "", "Sec-Fetch-Site", "")
	if rec.Code != http.StatusOK || !r.OK {
		t.Errorf("headerless request refused: %d %+v", rec.Code, r)
	}

	// Unknown fields and oversized bodies are refused.
	form := registration(t, h, "Varian", "varian@example.com")
	form["admin"] = "1"
	if rec, _ := post(t, h, "/api/register", form); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown field: %d", rec.Code)
	}
	form = registration(t, h, "Varian", "varian@example.com")
	form["email"] = strings.Repeat("a", 5000) + "@example.com"
	if rec, _ := post(t, h, "/api/register", form); rec.Code != http.StatusBadRequest {
		t.Errorf("huge body: %d", rec.Code)
	}
}

func TestAccountsPerIP(t *testing.T) {
	h := newTestServer(t, func(c *Config) { c.AccountsPerIPDay = 2 })
	for i, name := range []string{"Anduin", "Genn", "Malfurion"} {
		form := registration(t, h, name, strings.ToLower(name)+"@example.com")
		rec, r := post(t, h, "/api/register", form)
		if i < 2 && !r.OK {
			t.Fatalf("account %d refused: %d %+v", i, rec.Code, r)
		}
		if i == 2 && (r.OK || rec.Code != http.StatusTooManyRequests) {
			t.Fatalf("third account from one address: %d %+v", rec.Code, r)
		}
	}
	// Another address behind the same proxy is counted on its own.
	form := registration(t, h, "Malfurion", "malfurion@example.com")
	b, _ := json.Marshal(form)
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "172.18.0.2:5555"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("a different player was refused: %d %s", rec.Code, rec.Body)
	}
}

func TestChangePassword(t *testing.T) {
	h := newTestServer(t, nil)
	form := func(user, old, new1, new2 string) map[string]string {
		ch, nonce := work(t, h)
		return map[string]string{"username": user, "password": old, "newPassword": new1, "newPassword2": new2, "challenge": ch, "nonce": nonce}
	}

	if rec, r := post(t, h, "/api/password", form("arthas", "wrong", "lichking80", "lichking80")); rec.Code != http.StatusUnauthorized || r.OK {
		t.Errorf("wrong password: %d %+v", rec.Code, r)
	}
	if rec, r := post(t, h, "/api/password", form("nobody", "wrong", "lichking80", "lichking80")); rec.Code != http.StatusUnauthorized || r.Message != "That username and password don't match." {
		t.Errorf("unknown account should look like a wrong password: %d %+v", rec.Code, r)
	}
	if _, r := post(t, h, "/api/password", form("arthas", "frostmourne", "short", "short")); r.Field != "newPassword" {
		t.Errorf("short new password: %+v", r)
	}
	if _, r := post(t, h, "/api/password", form("arthas", "frostmourne", "lichking80", "lichking81")); r.Field != "newPassword2" {
		t.Errorf("mismatch: %+v", r)
	}
	// Passwords are case-insensitive in the game.
	if rec, r := post(t, h, "/api/password", form("Arthas", "FrostMourne", "lichking80", "lichking80")); rec.Code != http.StatusOK || !r.OK {
		t.Fatalf("change: %d %+v", rec.Code, r)
	}
	if _, r := post(t, h, "/api/password", form("arthas", "lichking80", "frostmourne", "frostmourne")); !r.OK {
		t.Errorf("new password doesn't work: %+v", r)
	}
}

func TestBadLoginLimit(t *testing.T) {
	h := newTestServer(t, nil)
	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		ch, nonce := work(t, h)
		b, _ := json.Marshal(map[string]string{"username": "arthas", "password": "guess" + strconv.Itoa(i),
			"newPassword": "lichking80", "newPassword2": "lichking80", "challenge": ch, "nonce": nonce})
		req := httptest.NewRequest(http.MethodPost, "/api/password", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		// From different addresses: the limit is per account too.
		req.RemoteAddr = "198.51.100." + strconv.Itoa(i+1) + ":1234"
		last = httptest.NewRecorder()
		h.ServeHTTP(last, req)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Errorf("sixth wrong password = %d, want 429", last.Code)
	}
}

func TestClientIP(t *testing.T) {
	p, err := ParseProxies("172.16.0.0/12, 127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		remote, xff, real, want string
	}{
		{"203.0.113.5:1000", "1.2.3.4", "", "203.0.113.5"},            // not from a proxy: headers ignored
		{"172.18.0.3:1000", "1.2.3.4", "", "1.2.3.4"},                 // from the proxy
		{"172.18.0.3:1000", "6.6.6.6, 1.2.3.4", "", "1.2.3.4"},        // a forged first hop is skipped
		{"172.18.0.3:1000", "1.2.3.4, 172.18.0.9", "", "1.2.3.4"},     // two proxies
		{"172.18.0.3:1000", "", "1.2.3.4", "1.2.3.4"},                 // X-Real-IP only
		{"172.18.0.3:1000", "", "", "172.18.0.3"},                     // nothing to go on
		{"[::ffff:172.18.0.3]:1000", "1.2.3.4", "", "1.2.3.4"},        // IPv4-mapped proxy address
		{"172.18.0.3:1000", "garbage, 1.2.3.4", "", "1.2.3.4"},        // junk to the left is never reached
		{"172.18.0.3:1000", "1.2.3.4, garbage", "9.9.9.9", "9.9.9.9"}, // junk at the right: fall back
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = c.remote
		if c.xff != "" {
			req.Header.Set("X-Forwarded-For", c.xff)
		}
		if c.real != "" {
			req.Header.Set("X-Real-IP", c.real)
		}
		if got := p.ClientIP(req); got != c.want {
			t.Errorf("%+v: got %s", c, got)
		}
	}
}

func TestContentOverrides(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("rules.html", "<h2>Rules</h2><p>Be kind in {{REALM}}.</p>")
	write("bots.html", "   \n ")
	h := newTestServer(t, func(c *Config) { c.ContentDir = dir; c.SiteName = `Frost & "Fire"` })
	body := get(t, h, "/").Body.String()
	if !strings.Contains(body, `id="rules"`) || !strings.Contains(body, "Be kind in Frost &amp; &#34;Fire&#34;.") {
		t.Error("rules.html wasn't used, or its token wasn't filled in safely")
	}
	if strings.Contains(body, `id="bots"`) || strings.Contains(body, `href="#bots"`) {
		t.Error("an empty bots.html should hide the section and its link")
	}
	if !strings.Contains(body, `id="about"`) {
		t.Error("about.html should fall back to the built-in text")
	}
}

func TestHelpers(t *testing.T) {
	for in, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1214: "1,214", 1234567: "1,234,567", -4321: "-4,321"} {
		if got := commas(in); got != want {
			t.Errorf("commas(%d) = %q", in, got)
		}
	}
	if _, err := dict("a"); err == nil {
		t.Error("dict with an odd count should fail")
	}
	if _, err := dict(1, 2); err == nil {
		t.Error("dict with a non-string key should fail")
	}
}

func TestPasswordFormRefusesBotAccounts(t *testing.T) {
	h := newTestServer(t, nil)
	for _, user := range []string{"rndbot1", "RNDBOT42", " RndBot7 "} {
		ch, nonce := work(t, h)
		rec, r := post(t, h, "/api/password", map[string]string{"username": user, "password": strings.TrimSpace(user),
			"newPassword": "stolen-pass1", "newPassword2": "stolen-pass1", "challenge": ch, "nonce": nonce})
		if rec.Code != http.StatusForbidden || r.Field != "username" || r.OK {
			t.Errorf("%q: %d %+v", user, rec.Code, r)
		}
	}
}

func TestChallengeSameOriginOnly(t *testing.T) {
	h := newTestServer(t, nil)
	for site, want := range map[string]int{"cross-site": 403, "same-site": 403, "none": 403, "same-origin": 200, "": 200} {
		if rec := get(t, h, "/api/challenge", "Sec-Fetch-Site", site); rec.Code != want {
			t.Errorf("Sec-Fetch-Site %q: %d, want %d", site, rec.Code, want)
		}
	}
	// Refused requests don't use up the allowance: 100 more still leave room.
	for i := 0; i < 100; i++ {
		get(t, h, "/api/challenge", "Sec-Fetch-Site", "cross-site")
	}
	if rec := get(t, h, "/api/challenge"); rec.Code != http.StatusOK {
		t.Errorf("after cross-site requests: %d", rec.Code)
	}
}

func TestLimitKey(t *testing.T) {
	for in, want := range map[string]string{
		"203.0.113.9":          "203.0.113.9",
		"::ffff:203.0.113.9":   "203.0.113.9",
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:2::/64",
		"2001:db8:1:2:ffff::1": "2001:db8:1:2::/64",
		"2001:db8:1:3::1":      "2001:db8:1:3::/64",
		"not an address":       "not an address",
	} {
		if got := limitKey(in); got != want {
			t.Errorf("limitKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// postFrom sends a form from a given address, with no proxy in between.
func postFrom(h http.Handler, addr, target string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = addr
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Fresh IPv6 addresses from one /64 count as one address.
func TestIPv6CountsBySlash64(t *testing.T) {
	h := newTestServer(t, func(c *Config) { c.AccountsPerIPDay = 1 })
	if rec := postFrom(h, "[2001:db8:5:6::1]:4000", "/api/register", registration(t, h, "Alleria", "alleria@example.com")); rec.Code != http.StatusOK {
		t.Fatalf("first account: %d %s", rec.Code, rec.Body)
	}
	if rec := postFrom(h, "[2001:db8:5:6::abcd]:4000", "/api/register", registration(t, h, "Vereesa", "vereesa@example.com")); rec.Code != http.StatusTooManyRequests {
		t.Errorf("same /64, new address: %d, want 429", rec.Code)
	}
	if rec := postFrom(h, "[2001:db8:5:7::1]:4000", "/api/register", registration(t, h, "Vereesa", "vereesa@example.com")); rec.Code != http.StatusOK {
		t.Errorf("another /64: %d %s", rec.Code, rec.Body)
	}
}

// Sign-ups sent all at once get no more accounts than the limit, and the
// ones refused by the database give their place back.
func TestSimultaneousSignUps(t *testing.T) {
	h := newTestServer(t, func(c *Config) { c.AccountsPerIPDay = 3 })
	forms := make([]map[string]string, 12)
	for i := range forms {
		name := "Racer" + strconv.Itoa(i)
		forms[i] = registration(t, h, name, strings.ToLower(name)+"@example.com")
	}
	codes := make(chan int, len(forms))
	for _, f := range forms {
		go func(f map[string]string) { codes <- postFrom(h, "198.51.100.20:1000", "/api/register", f).Code }(f)
	}
	made := 0
	for range forms {
		if <-codes == http.StatusOK {
			made++
		}
	}
	if made != 3 {
		t.Fatalf("%d accounts made from one address at once, want 3", made)
	}

	// A sign-up the database refuses doesn't use up a place.
	h = newTestServer(t, func(c *Config) { c.AccountsPerIPDay = 1 })
	if rec := postFrom(h, "198.51.100.21:1000", "/api/register", registration(t, h, "Arthas", "a@example.com")); rec.Code != http.StatusConflict {
		t.Fatalf("taken name: %d", rec.Code)
	}
	if rec := postFrom(h, "198.51.100.21:1000", "/api/register", registration(t, h, "Kael", "kael@example.com")); rec.Code != http.StatusOK {
		t.Errorf("after a refused sign-up: %d %s", rec.Code, rec.Body)
	}
}

// Wrong passwords sent all at once still stop at the limit.
func TestSimultaneousWrongPasswords(t *testing.T) {
	h := newTestServer(t, nil)
	forms := make([]map[string]string, 15)
	for i := range forms {
		ch, nonce := work(t, h)
		forms[i] = map[string]string{"username": "arthas", "password": "guess" + strconv.Itoa(i),
			"newPassword": "lichking80", "newPassword2": "lichking80", "challenge": ch, "nonce": nonce}
	}
	codes := make(chan int, len(forms))
	for i, f := range forms {
		go func(i int, f map[string]string) {
			codes <- postFrom(h, "198.51.100."+strconv.Itoa(30+i)+":1000", "/api/password", f).Code
		}(i, f)
	}
	checked := 0
	for range forms {
		if <-codes == http.StatusUnauthorized {
			checked++
		}
	}
	if checked != 5 {
		t.Fatalf("%d wrong passwords checked at once, want 5", checked)
	}
}

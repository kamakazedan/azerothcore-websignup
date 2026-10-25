package site

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kamakazedan/azerothcore-websignup/internal/game"
	"github.com/kamakazedan/azerothcore-websignup/internal/pow"
)

func TestThemeConf(t *testing.T) {
	s, problems := parseThemeConf([]byte(`
# a comment
background_focus = center top
background_shade = 0.5      # darker
background-motion: zoom
page_shade = .8
particles = Hover
particle_count = 7
particle_size = 10-50px
particle_speed = 2
particle_opacity = .6
particle_spin = no
particle_target = 40%
`))
	if len(problems) > 0 {
		t.Fatalf("problems: %v", problems)
	}
	want := ThemeSettings{Focus: "50% 0%", Shade: .5, Motion: "zoom", PageShade: .8, Particles: "float", Count: 7,
		SizeMin: 10, SizeMax: 50, Speed: 2, Opacity: .6, Spin: false, Target: 40}
	if s != want {
		t.Errorf("got  %+v\nwant %+v", s, want)
	}

	s, problems = parseThemeConf([]byte(`
background_shade = 3
particles = sideways
particle_count = 1000
particle_size = 50-10
particle_speed = fast
colour = red
just some words
`))
	if len(problems) != 7 {
		t.Errorf("want 7 problems, got %d: %v", len(problems), problems)
	}
	if s != defaultSettings() {
		t.Errorf("bad values should leave the defaults: %+v", s)
	}
}

func TestFocus(t *testing.T) {
	for in, want := range map[string]string{
		"50% 30%": "50% 30%", "30%": "30% 50%", "20 80": "20% 80%", "center": "50% 50%", "top": "50% 0%",
		"left bottom": "0% 100%", "right 25%": "100% 25%", "12.34% 0": "12.3% 0%",
	} {
		if got, ok := parseFocus(in); !ok || got != want {
			t.Errorf("parseFocus(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "150%", "-5%", "a b", "1% 2% 3%", "top bottom 3%", "url(x)", "50%;color:red"} {
		if got, ok := parseFocus(bad); ok {
			t.Errorf("parseFocus(%q) = %q, want a refusal", bad, got)
		}
	}
}

func themeServer(t *testing.T, dir string) (*Server, http.Handler) {
	t.Helper()
	cfg := testConfig()
	cfg.ThemeDir = dir
	s, err := New(cfg, game.NewDemo(), pow.New([]byte("k"), 1, time.Minute), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return s, s.Handler()
}

func write(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNoTheme(t *testing.T) {
	_, h := themeServer(t, filepath.Join(t.TempDir(), "missing"))
	body := get(t, h, "/").Body.String()
	for _, bad := range []string{"<video", `class="particles`, "/theme/"} {
		if strings.Contains(body, bad) {
			t.Errorf("an empty theme still puts %q on the page", bad)
		}
	}
	if !strings.Contains(body, `href="/theme.css?v=`) || !strings.Contains(body, "/static/img/emblem.svg") {
		t.Error("the page should link /theme.css and show the built-in emblem")
	}
	css := get(t, h, "/theme.css").Body.String()
	if strings.Contains(css, "url(") || strings.Contains(css, "nth-child") {
		t.Errorf("an empty theme makes a stylesheet with pictures or particles:\n%s", css)
	}
}

func TestThemeFolder(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"background.jpg", "background-phone.png", "background.mp4", "background.webm", "page.webp",
		"particle.png", "particle-2.svg", "logo.svg", "favicon.png", "theme.css", "notes.txt"} {
		write(t, dir, n, "data for "+n)
	}
	write(t, dir, "particle bad name.png", "x")
	write(t, dir, "theme.conf", "particles = float\nparticle_count = 7\nbackground_motion = drift\n")
	srv, h := themeServer(t, dir)
	th, _, _ := srv.theme.get()

	body := get(t, h, "/").Body.String()
	for _, want := range []string{
		`<video class="sky-video motion-drift"`, `poster="/theme/background.jpg?v=` + th.Stamp + `"`,
		`<source src="/theme/background.webm?v=`, `type="video/webm"`, `<source src="/theme/background.mp4?v=`,
		`<div class="sky motion-drift"`, `class="particles particles-float has-sprite"`,
		`<link rel="icon" href="/theme/favicon.png?v=`, `<img src="/theme/logo.svg?v=`,
		`<link rel="stylesheet" href="/theme/theme.css?v=`, `media="(max-width: 640px)"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page is missing %q", want)
		}
	}
	if n := strings.Count(body, "<i></i>"); n != 7 {
		t.Errorf("%d particles on the page, want 7", n)
	}

	css := get(t, h, "/theme.css?v="+th.Stamp)
	if !strings.Contains(css.Header().Get("Cache-Control"), "immutable") {
		t.Error("a versioned /theme.css should be cached for good")
	}
	sheet := css.Body.String()
	for _, want := range []string{
		`.sky { background: url("/theme/background.jpg?v=`, `@media (max-width: 640px) { .sky { background: url("/theme/background-phone.png?v=`,
		`body::before`, `url("/theme/page.webp?v=`, `.particles i:nth-child(7)`, `--sprite: url("/theme/particle-2.svg?v=`,
		`--sprite: url("/theme/particle.png?v=`,
	} {
		if !strings.Contains(sheet, want) {
			t.Errorf("/theme.css is missing %q", want)
		}
	}
	if strings.Contains(sheet, "nth-child(8)") || strings.Contains(sheet, "bad name") {
		t.Error("/theme.css has more particles than asked for, or a file it should have skipped")
	}

	// Files the theme uses are served with their types; nothing else is.
	for name, ctype := range map[string]string{"background.jpg": "image/jpeg", "background.mp4": "video/mp4",
		"particle-2.svg": "image/svg+xml", "theme.css": "text/css; charset=utf-8", "favicon.png": "image/png"} {
		rec := get(t, h, "/theme/"+name+"?v="+th.Stamp)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != ctype || rec.Body.String() != "data for "+name {
			t.Errorf("/theme/%s: %d %q %q", name, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
		}
	}
	for _, name := range []string{"theme.conf", "notes.txt", "particle%20bad%20name.png", "..%2Fgo.mod", "%2Fetc%2Fpasswd"} {
		if rec := get(t, h, "/theme/"+name); rec.Code != http.StatusNotFound {
			t.Errorf("/theme/%s = %d, want 404", name, rec.Code)
		}
	}
	// Videos can be seeked: range requests work.
	rec := get(t, h, "/theme/background.mp4", "Range", "bytes=0-3")
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "data" {
		t.Errorf("range request: %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(t, h, "/favicon.ico"); rec.Header().Get("Location") != th.Favicon {
		t.Errorf("/favicon.ico goes to %q, want the theme's", rec.Header().Get("Location"))
	}
}

// Files that look meant for the theme but aren't used are pointed out in
// the log, with the reason; other files are left alone.
func TestThemeHints(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"background.jpg", "background.png", "background.mp4", "logo.gif", "Frozen Throne.jpg",
		"particle.jpg", "notes.txt", "README.md", "theme.conf.example"} {
		write(t, dir, n, "x")
	}
	var logs bytes.Buffer
	newThemeDir(dir, slog.New(slog.NewTextHandler(&logs, nil)))
	out := logs.String()
	for _, want := range []string{
		"background.png isn't used: background.jpg is used instead",
		"logo.gif isn't used: a logo can be .svg, .png, .webp",
		"Frozen Throne.jpg isn't used: the theme only uses files called background",
		"particle.jpg isn't used: particle images can be",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the log doesn't say %q:\n%s", want, out)
		}
	}
	for _, quiet := range []string{"notes.txt", "README.md isn't", "theme.conf.example", "background.jpg isn't", "background.mp4 isn't", "can't be read"} {
		if strings.Contains(out, quiet) {
			t.Errorf("the log mentions %q:\n%s", quiet, out)
		}
	}
}

// Particle images alone are enough to set particles rising; theme.conf can
// turn them off again, and edits show up without a restart.
func TestThemeReload(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "particle.png", "x")
	srv, h := themeServer(t, dir)
	if !strings.Contains(get(t, h, "/").Body.String(), `class="particles particles-rise has-sprite"`) {
		t.Fatal("particle images on their own should rise")
	}
	before, _, _ := srv.theme.get()

	write(t, dir, "theme.conf", "particles = none\n")
	srv.theme.mu.Lock()
	srv.theme.checked = time.Time{} // don't wait the few seconds
	srv.theme.mu.Unlock()
	after, _, _ := srv.theme.get()
	if after.Stamp == before.Stamp {
		t.Error("the stamp didn't change after an edit")
	}
	if body := get(t, h, "/").Body.String(); strings.Contains(body, `class="particles`) {
		t.Error("particles = none should take them off the page")
	}
}

// Without images, the particles are drawn shards, and every mode makes one
// rule per particle.
func TestParticleModes(t *testing.T) {
	for _, mode := range []string{"rise", "fall", "float"} {
		th := &Theme{Settings: defaultSettings()}
		th.Settings.Particles = mode
		th.Settings.Count = 5
		th.Settings.Target = 50
		css := string(th.stylesheet())
		if strings.Count(css, "nth-child(") != 5 || !strings.Contains(css, "--shape: polygon(") {
			t.Errorf("%s: unexpected stylesheet:\n%s", mode, css)
		}
		if mode == "float" && !strings.Contains(css, "px;") {
			t.Errorf("float should move by pixels:\n%s", css)
		}
		if again := string(th.stylesheet()); again != css {
			t.Errorf("%s: the stylesheet changes from one call to the next", mode)
		}
	}
}

// The test server's own requests shouldn't see the theme folder of the
// machine running the tests.
func TestThemeDirIsolation(t *testing.T) {
	rec := httptest.NewRecorder()
	_, h := themeServer(t, "")
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/theme/background.jpg", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("no theme folder: %d", rec.Code)
	}
}

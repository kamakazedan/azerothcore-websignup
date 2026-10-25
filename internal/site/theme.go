package site

// The look of the top of the page comes from a folder of your own files
// (THEME_DIR, /theme in the container). Every file is optional; with no
// background picture, the built-in one (static/img, made by art/background.py)
// is used:
//
//	background.jpg      the picture behind the name and form (or .png, .webp, .avif)
//	background-phone.jpg  a different crop of it for phones
//	background.mp4      a looping video instead (or .webm; the picture is shown until it plays)
//	lich-king.1920x1080.mp4  any other video is a scene: one is picked for each visit,
//	                    in the size that suits the screen (see theme/README.md)
//	page.jpg            a picture behind the rest of the page, dimmed
//	particle.png        images that float across the top; particle-2.png and so on for variety
//	logo.svg            replaces the emblem in the corner (or .png, .webp)
//	favicon.png         replaces the browser tab icon (or .svg, .ico)
//	theme.css           extra styles of your own, loaded last
//	theme.conf          settings: where to crop the picture, how dark to make it,
//	                    and how the particles move (see theme/README.md)
//
// Changes are picked up within a few seconds, without a restart.

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	pictureExts = []string{".avif", ".webp", ".jpg", ".jpeg", ".png"}
	videoExts   = []string{".webm", ".mp4"}
	spriteExts  = []string{".svg", ".png", ".webp", ".avif", ".gif"}
	logoExts    = []string{".svg", ".png", ".webp"}
	iconExts    = []string{".svg", ".png", ".ico"}

	// Names the theme uses, and what each can be.
	themeNames = map[string][]string{
		"background":       append(append([]string{}, pictureExts...), videoExts...),
		"background-phone": pictureExts,
		"page":             pictureExts,
		"logo":             logoExts,
		"favicon":          iconExts,
		"theme":            {".css"},
	}
	// Files that look meant for the theme, for saying why one isn't used.
	mediaExts = []string{".avif", ".webp", ".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico", ".bmp", ".tif", ".tiff",
		".heic", ".webm", ".mp4", ".mov", ".mkv", ".avi", ".css"}

	themeTypes = map[string]string{
		".avif": "image/avif", ".webp": "image/webp", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
		".png": "image/png", ".gif": "image/gif", ".svg": "image/svg+xml", ".ico": "image/x-icon",
		".webm": "video/webm", ".mp4": "video/mp4", ".css": "text/css; charset=utf-8",
	}

	// Theme files go into URLs and CSS as they are, so their names are kept plain.
	safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

	// A scene's files: its name, then optionally its size (lich-king.1920x1080.mp4,
	// lich-king-720x1280.webm, lich-king.jpg).
	sceneFile = regexp.MustCompile(`^([a-z0-9][a-z0-9._-]*?)(?:[._-]([0-9]{2,5})x([0-9]{2,5}))?(\.[a-z0-9]+)$`)
	sceneName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
)

const maxSprites = 12

type themeVideo struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

// A scene is one background video, in as many sizes as you have of it. The
// page picks a scene, then the size that best fits the screen.
type themeScene struct {
	Name   string         `json:"name"`
	Poster string         `json:"poster,omitempty"`
	Flip   bool           `json:"flip,omitempty"` // mirrored left to right (video_flip)
	Sizes  []themeVariant `json:"sizes"`
}

type themeVariant struct {
	W   int          `json:"w"` // 0 when the file name doesn't say
	H   int          `json:"h"`
	Src []themeVideo `json:"src"`
}

// Theme is what the page uses of the theme folder: URLs (versioned, so
// browsers can keep them) and the settings from theme.conf.
type Theme struct {
	Stamp           string
	Background      string
	BackgroundPhone string
	Scenes          []themeScene
	Video           string // the scenes and settings as JSON, for the page's script
	Page            string
	Sprites         []string
	Logo            string
	Favicon         string
	CSS             string
	Settings        ThemeSettings
	Slots           []struct{} // one per particle, for the template to range over
}

// ThemeSettings are the knobs in theme.conf.
type ThemeSettings struct {
	Focus     string  // which part of the picture stays in view: "50% 40%"
	Shade     float64 // how much to darken the picture, 0 to 0.9
	Motion    string  // none, zoom or drift
	PageShade float64 // how much to darken page.*, 0 to 0.95
	Particles string  // none, rise, fall or float
	Count     int
	SizeMin   int // pixels
	SizeMax   int
	Speed     float64 // 1 is normal
	Opacity   float64
	Spin      bool
	Target    float64  // rise: how far across (0-100) they drift towards; -1 for straight up
	Scene     string   // video_scene: random, or the name of one
	Change    int      // video_change: seconds before moving on to the next scene; 0 for never
	Preload   int      // video_preload: how early the next scene starts loading, as a percentage of Change
	Quality   int      // video_quality: the largest size to send, by its short side (720, 1080...); 0 for any
	Flip      []string // video_flip: scenes to mirror left to right; "all" for every one
}

func defaultSettings() ThemeSettings {
	return ThemeSettings{
		Focus: "50% 40%", Shade: .35, Motion: "none", PageShade: .75,
		Particles: "", Count: 18, SizeMin: 12, SizeMax: 36, Speed: 1, Opacity: .9, Spin: true, Target: -1,
		Scene: "random", Preload: 50,
	}
}

type themeDir struct {
	dir string
	log *slog.Logger
	// The built-in pictures, used when the folder has no background picture
	// of its own.
	fallback, fallbackPhone string

	mu      sync.Mutex
	checked time.Time
	sig     string
	dirErr  string // the last error reading the folder, so it's logged once
	theme   *Theme
	files   map[string]string // name in the URL -> file on disk
	css     []byte
}

func newThemeDir(dir string, log *slog.Logger, fallback, fallbackPhone string) *themeDir {
	t := &themeDir{dir: dir, log: log, fallback: fallback, fallbackPhone: fallbackPhone}
	t.refresh(true)
	return t
}

// get returns the current theme, rescanning the folder every few seconds.
func (t *themeDir) get() (*Theme, map[string]string, []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if time.Since(t.checked) > 5*time.Second {
		t.refreshLocked(false)
	}
	return t.theme, t.files, t.css
}

func (t *themeDir) refresh(first bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.refreshLocked(first)
}

// refreshLocked reloads the theme if anything in the folder changed.
func (t *themeDir) refreshLocked(first bool) {
	t.checked = time.Now()
	var entries []os.DirEntry
	if t.dir != "" {
		var err error
		entries, err = os.ReadDir(t.dir)
		msg := ""
		if err != nil && !os.IsNotExist(err) {
			msg = err.Error()
		}
		if msg != "" && msg != t.dirErr {
			t.log.Warn("theme folder can't be read", "dir", t.dir, "err", err)
		}
		t.dirErr = msg
	}

	// The folder's signature: names, sizes, times and permissions of the
	// files in it (permissions, so a chmod that fixes a file is noticed).
	h := sha256.New()
	regular := map[string]os.DirEntry{}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		regular[strings.ToLower(e.Name())] = e
		fmt.Fprintf(h, "%s|%d|%d|%o\n", e.Name(), info.Size(), info.ModTime().UnixNano(), info.Mode().Perm())
	}
	sig := hex.EncodeToString(h.Sum(nil))[:12]
	if !first && sig == t.sig && t.theme != nil {
		return
	}
	t.sig = sig

	files := map[string]string{}
	url := func(name string) string {
		files[name] = filepath.Join(t.dir, name)
		return "/theme/" + name + "?v=" + sig
	}
	pick := func(base string, exts []string) string {
		for _, ext := range exts {
			if e, ok := regular[base+ext]; ok && safeName.MatchString(e.Name()) {
				return e.Name()
			}
		}
		return ""
	}

	th := &Theme{Stamp: sig, Settings: defaultSettings()}
	if n := pick("background", pictureExts); n != "" {
		th.Background = url(n)
	}
	if n := pick("background-phone", pictureExts); n != "" {
		th.BackgroundPhone = url(n)
	}
	own := th.Background != ""
	if !own {
		th.Background = t.fallback
		if th.BackgroundPhone == "" {
			th.BackgroundPhone = t.fallbackPhone
		}
	}
	if n := pick("page", pictureExts); n != "" {
		th.Page = url(n)
	}
	if n := pick("logo", logoExts); n != "" {
		th.Logo = url(n)
	}
	if n := pick("favicon", iconExts); n != "" {
		th.Favicon = url(n)
	}
	if n := pick("theme", []string{".css"}); n != "" {
		th.CSS = url(n)
	}

	th.Scenes = scenes(regular, url, th.Background)

	var sprites, skipped []string
	for lower, e := range regular {
		if !strings.HasPrefix(lower, "particle") || !hasExt(lower, spriteExts) {
			continue
		}
		if !safeName.MatchString(e.Name()) {
			skipped = append(skipped, e.Name())
			continue
		}
		sprites = append(sprites, e.Name())
	}
	sort.Strings(sprites)
	if len(sprites) > maxSprites {
		skipped = append(skipped, sprites[maxSprites:]...)
		sprites = sprites[:maxSprites]
	}
	for _, n := range sprites {
		th.Sprites = append(th.Sprites, url(n))
	}

	var problems []string
	if e, ok := regular["theme.conf"]; ok {
		data, err := os.ReadFile(filepath.Join(t.dir, e.Name()))
		if err != nil {
			problems = append(problems, unreadable(e.Name(), err))
		} else {
			var p []string
			th.Settings, p = parseThemeConf(data)
			problems = append(problems, p...)
		}
	}
	// Particle images on their own mean "make them float"; theme.conf can
	// still say otherwise.
	if th.Settings.Particles == "" {
		th.Settings.Particles = "none"
		if len(th.Sprites) > 0 {
			th.Settings.Particles = "rise"
		}
	}
	if th.Settings.Particles != "none" {
		th.Slots = make([]struct{}, th.Settings.Count)
	}
	if len(th.Scenes) > 0 {
		s := th.Settings
		if s.Scene != "random" && !hasScene(th.Scenes, s.Scene) {
			problems = append(problems, fmt.Sprintf("theme.conf: video_scene = %s, but there are no videos called that; "+
				"picking one at random", s.Scene))
			th.Settings.Scene = "random"
		}
		for _, name := range s.Flip {
			if name != "all" && !hasScene(th.Scenes, name) {
				problems = append(problems, fmt.Sprintf("theme.conf: video_flip names %s, but there are no videos called that", name))
			}
		}
		for i := range th.Scenes {
			for _, name := range s.Flip {
				if name == "all" || name == th.Scenes[i].Name {
					th.Scenes[i].Flip = true
				}
			}
		}
		js, _ := json.Marshal(struct {
			Scene   string       `json:"scene"`
			Change  int          `json:"change"`
			Preload int          `json:"preload"`
			Quality int          `json:"quality"`
			Scenes  []themeScene `json:"scenes"`
		}{th.Settings.Scene, s.Change, s.Preload, s.Quality, th.Scenes})
		th.Video = string(js)
	}

	t.theme, t.files, t.css = th, files, th.stylesheet()

	for _, n := range skipped {
		problems = append(problems, "skipped "+n+": particle images need plain names (letters, digits, - _ .), at most 12 of them")
	}
	// The usual reasons a new picture doesn't show: the site can't read it,
	// or it isn't called something the theme uses.
	names := make([]string, 0, len(regular))
	for lower := range regular {
		names = append(names, lower)
	}
	sort.Strings(names)
	var used []string
	for n := range files {
		used = append(used, strings.ToLower(n))
	}
	for _, lower := range names {
		name := regular[lower].Name()
		if p, ok := files[name]; ok {
			f, err := os.Open(p)
			if err != nil {
				problems = append(problems, unreadable(name, err))
				continue
			}
			f.Close()
		} else if why := unusedReason(lower, used); why != "" {
			problems = append(problems, name+" isn't used: "+why)
		}
	}
	if t.dir != "" && (len(entries) > 0 || !first) {
		t.log.Info("theme loaded", "dir", t.dir, "background", own, "videos", sceneList(th.Scenes),
			"particles", th.Settings.Particles, "particle images", len(th.Sprites))
	}
	for _, p := range problems {
		t.log.Warn("theme", "problem", p)
	}
}

// scenes groups the folder's videos by name, each in every size there is
// of it, with a picture of the same name (lich-king.jpg) to show while it
// loads, or else the background picture.
func scenes(regular map[string]os.DirEntry, url func(string) string, background string) []themeScene {
	names := make([]string, 0, len(regular))
	for lower := range regular {
		names = append(names, lower)
	}
	sort.Strings(names)

	byName := map[string]*themeScene{}
	for _, lower := range names {
		e := regular[lower]
		m := sceneFile.FindStringSubmatch(lower)
		if !hasExt(lower, videoExts) || !safeName.MatchString(e.Name()) || m == nil {
			continue
		}
		sc := byName[m[1]]
		if sc == nil {
			sc = &themeScene{Name: m[1]}
			byName[m[1]] = sc
		}
		w, _ := strconv.Atoi(m[2])
		h, _ := strconv.Atoi(m[3])
		i := 0
		for i < len(sc.Sizes) && (sc.Sizes[i].W != w || sc.Sizes[i].H != h) {
			i++
		}
		if i == len(sc.Sizes) {
			sc.Sizes = append(sc.Sizes, themeVariant{W: w, H: h})
		}
		sc.Sizes[i].Src = append(sc.Sizes[i].Src, themeVideo{URL: url(e.Name()), Type: themeTypes[m[4]]})
	}
	for _, lower := range names {
		e := regular[lower]
		m := sceneFile.FindStringSubmatch(lower)
		if !hasExt(lower, pictureExts) || !safeName.MatchString(e.Name()) || m == nil || m[1] == "background" {
			continue
		}
		if sc := byName[m[1]]; sc != nil && sc.Poster == "" {
			sc.Poster = url(e.Name())
		}
	}

	out := make([]themeScene, 0, len(byName))
	for _, sc := range byName {
		if sc.Poster == "" {
			sc.Poster = background
		}
		// Smallest first; WebM before MP4, as browsers try them in order.
		sort.Slice(sc.Sizes, func(i, j int) bool { return sc.Sizes[i].W*sc.Sizes[i].H < sc.Sizes[j].W*sc.Sizes[j].H })
		for _, v := range sc.Sizes {
			sort.Slice(v.Src, func(i, j int) bool { return v.Src[i].Type > v.Src[j].Type })
		}
		out = append(out, *sc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func hasScene(scenes []themeScene, name string) bool {
	for _, sc := range scenes {
		if sc.Name == name {
			return true
		}
	}
	return false
}

// sceneList is for the log: "lich-king (720x1280, 1920x1080), night-elf".
func sceneList(scenes []themeScene) string {
	var parts []string
	for _, sc := range scenes {
		var sizes []string
		for _, v := range sc.Sizes {
			if v.W > 0 {
				sizes = append(sizes, fmt.Sprintf("%dx%d", v.W, v.H))
			}
		}
		if len(sizes) > 0 {
			parts = append(parts, sc.Name+" ("+strings.Join(sizes, ", ")+")")
		} else {
			parts = append(parts, sc.Name)
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func unreadable(name string, err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	return fmt.Sprintf("%s can't be read (%v). The site doesn't run as root, so the files in the theme folder "+
		"must be readable by everyone: chmod -R a+rX on the folder", name, err)
}

// unusedReason says why a file that looks meant for the theme isn't used,
// or "" for files that don't (notes, README.md, theme.conf.example).
func unusedReason(lower string, used []string) string {
	ext := path.Ext(lower)
	base := strings.TrimSuffix(lower, ext)
	if !hasExt(lower, mediaExts) {
		return ""
	}
	if strings.HasPrefix(base, "particle") {
		if hasExt(lower, spriteExts) {
			return "" // skipped for its name or the count, said above
		}
		return "particle images can be " + strings.Join(spriteExts, ", ") + ", with a see-through background"
	}
	if hasExt(lower, videoExts) {
		return "video names must be plain: letters, digits, - _ and ., with no spaces"
	}
	exts, ok := themeNames[base]
	if !ok {
		return "the theme only uses files called background, background-phone, page, particle, logo, favicon " +
			"and theme, videos, and pictures named after a video (see theme/README.md)"
	}
	if !hasExt(lower, exts) {
		return "a " + base + " can be " + strings.Join(exts, ", ")
	}
	for _, u := range used {
		if strings.TrimSuffix(u, path.Ext(u)) == base && (base != "background" || hasExt(u, pictureExts)) {
			return u + " is used instead (only one of them is)"
		}
	}
	return ""
}

func hasExt(name string, exts []string) bool {
	ext := strings.ToLower(path.Ext(name))
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

// parseThemeConf reads "key = value" lines; # starts a comment. Anything it
// can't use is reported and left at its default.
func parseThemeConf(data []byte) (ThemeSettings, []string) {
	s := defaultSettings()
	var problems []string
	bad := func(key, val, want string) {
		problems = append(problems, fmt.Sprintf("theme.conf: %s = %q isn't %s; using the default", key, val, want))
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if i := strings.Index(text, "#"); i >= 0 {
			text = strings.TrimSpace(text[:i])
		}
		if text == "" || strings.HasPrefix(text, ";") {
			continue
		}
		k, v, ok := strings.Cut(text, "=")
		if !ok {
			k, v, ok = strings.Cut(text, ":")
		}
		if !ok {
			problems = append(problems, fmt.Sprintf("theme.conf line %d: expected key = value", line))
			continue
		}
		key := strings.NewReplacer("-", "_", " ", "_").Replace(strings.ToLower(strings.TrimSpace(k)))
		val := strings.TrimSpace(strings.Trim(strings.TrimSpace(v), `"'`))
		switch key {
		case "background_focus", "focus":
			if f, ok := parseFocus(val); ok {
				s.Focus = f
			} else {
				bad(key, val, "two percentages like 50% 30%, or words like center top")
			}
		case "background_shade", "shade":
			if f, ok := parseFloat(val, 0, .9); ok {
				s.Shade = f
			} else {
				bad(key, val, "a number from 0 to 0.9")
			}
		case "background_motion", "motion":
			switch v := strings.ToLower(val); v {
			case "none", "off", "no", "zoom", "drift":
				if v == "off" || v == "no" {
					v = "none"
				}
				s.Motion = v
			default:
				bad(key, val, "none, zoom or drift")
			}
		case "page_shade":
			if f, ok := parseFloat(val, 0, .95); ok {
				s.PageShade = f
			} else {
				bad(key, val, "a number from 0 to 0.95")
			}
		case "particles":
			switch v := strings.ToLower(val); v {
			case "none", "off", "no":
				s.Particles = "none"
			case "rise", "up":
				s.Particles = "rise"
			case "fall", "down":
				s.Particles = "fall"
			case "float", "hover":
				s.Particles = "float"
			default:
				bad(key, val, "rise, fall, float or none")
			}
		case "particle_count", "count":
			if n, err := strconv.Atoi(val); err == nil && n >= 0 && n <= 60 {
				s.Count = n
			} else {
				bad(key, val, "a whole number from 0 to 60")
			}
		case "particle_size", "size":
			lo, hi, found := strings.Cut(val, "-")
			a, err1 := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(lo), "px"))
			b := a
			var err2 error
			if found {
				b, err2 = strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(hi), "px"))
			}
			if err1 == nil && err2 == nil && a >= 2 && b <= 300 && a <= b {
				s.SizeMin, s.SizeMax = a, b
			} else {
				bad(key, val, "a size in pixels like 24, or a range like 12-40")
			}
		case "particle_speed", "speed":
			if f, ok := parseFloat(val, .1, 5); ok {
				s.Speed = f
			} else {
				bad(key, val, "a number from 0.1 to 5")
			}
		case "particle_opacity", "opacity":
			if f, ok := parseFloat(val, .05, 1); ok {
				s.Opacity = f
			} else {
				bad(key, val, "a number from 0.05 to 1")
			}
		case "particle_spin", "spin":
			switch strings.ToLower(val) {
			case "yes", "true", "on", "1":
				s.Spin = true
			case "no", "false", "off", "0":
				s.Spin = false
			default:
				bad(key, val, "yes or no")
			}
		case "particle_target", "target":
			switch strings.ToLower(val) {
			case "", "none", "off", "no":
				s.Target = -1
			default:
				if f, ok := parseFloat(strings.TrimSuffix(val, "%"), 0, 100); ok {
					s.Target = f
				} else {
					bad(key, val, "a number from 0 to 100, or none")
				}
			}
		case "video_scene", "scene":
			switch v := strings.ToLower(val); {
			case v == "random" || v == "any":
				s.Scene = "random"
			case sceneName.MatchString(v):
				s.Scene = v
			default:
				bad(key, val, "random, or the name of one of your videos")
			}
		case "video_change", "scene_change":
			switch v := strings.ToLower(val); v {
			case "never", "none", "off", "no":
				s.Change = 0
			default:
				n, err := strconv.Atoi(strings.TrimSuffix(v, "s"))
				if err == nil && (n == 0 || n >= 10 && n <= 3600) {
					s.Change = n
				} else {
					bad(key, val, "a number of seconds from 10 to 3600, or 0 for never")
				}
			}
		case "video_flip", "flip":
			s.Flip = nil
			ok := true
			for _, name := range strings.FieldsFunc(strings.ToLower(val), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
				switch {
				case name == "none" || name == "no" || name == "off":
				case name == "all" || sceneName.MatchString(name):
					s.Flip = append(s.Flip, name)
				default:
					ok = false
				}
			}
			if !ok {
				s.Flip = nil
				bad(key, val, "a list of your videos' names, like lich-king, night-elf-archer, or all")
			}
		case "video_preload", "preload":
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(val, "%"))); err == nil && n >= 0 && n <= 100 {
				s.Preload = n
			} else {
				bad(key, val, "a percentage from 0 to 100")
			}
		case "video_quality", "video_max":
			switch v := strings.ToLower(val); v {
			case "auto", "any", "best":
				s.Quality = 0
			default:
				n, err := strconv.Atoi(strings.TrimSuffix(v, "p"))
				if v == "4k" {
					n, err = 2160, nil
				}
				if err == nil && n >= 240 && n <= 4320 {
					s.Quality = n
				} else {
					bad(key, val, "auto, or a height like 720, 1080 or 2160")
				}
			}
		default:
			problems = append(problems, fmt.Sprintf("theme.conf line %d: %q isn't a setting", line, strings.TrimSpace(k)))
		}
	}
	return s, problems
}

func parseFloat(s string, lo, hi float64) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) || f < lo || f > hi {
		return 0, false
	}
	return f, true
}

// parseFocus accepts "50% 30%", "30%" (across only), or the keywords CSS
// uses for background-position, and returns a safe "X% Y%".
func parseFocus(s string) (string, bool) {
	words := strings.Fields(strings.ToLower(s))
	if len(words) == 0 || len(words) > 2 {
		return "", false
	}
	x, y := 50.0, 50.0
	setX, setY := false, false
	for i, w := range words {
		switch w {
		case "left":
			x, setX = 0, true
		case "right":
			x, setX = 100, true
		case "top":
			y, setY = 0, true
		case "bottom":
			y, setY = 100, true
		case "center", "centre", "middle":
		default:
			// 30% or plain 30: both mean 30% of the way
			f, ok := parseFloat(strings.TrimSuffix(w, "%"), 0, 100)
			if !ok {
				return "", false
			}
			if i == 0 && !setX {
				x, setX = f, true
			} else if !setY {
				y, setY = f, true
			} else {
				return "", false
			}
		}
	}
	return fmt.Sprintf("%s%% %s%%", trimNum(x), trimNum(y)), true
}

func trimNum(f float64) string {
	return strconv.FormatFloat(math.Round(f*10)/10, 'f', -1, 64)
}

// The particles drawn when there are no images of your own: angular shards.
var shardShapes = []string{
	"polygon(18% 6%, 62% 0, 100% 34%, 84% 82%, 38% 100%, 0 58%)",
	"polygon(40% 0, 88% 18%, 100% 66%, 60% 100%, 10% 84%, 0 30%)",
	"polygon(8% 22%, 54% 0, 92% 12%, 100% 58%, 70% 92%, 22% 100%, 0 64%)",
	"polygon(30% 0, 78% 10%, 100% 52%, 72% 100%, 20% 88%, 0 40%)",
	"polygon(50% 0, 100% 38%, 80% 100%, 18% 92%, 0 34%)",
	"polygon(14% 0, 70% 6%, 100% 44%, 90% 86%, 46% 100%, 0 70%)",
}

// stylesheet is /theme.css: the theme's pictures and one rule per particle.
// The animations themselves are in site.css; this only sets where each
// particle starts, its size, speed and image. It's the same every time for
// the same settings, so the page doesn't reshuffle on every visit.
func (th *Theme) stylesheet() []byte {
	var b strings.Builder
	s := th.Settings
	fmt.Fprintf(&b, "/* Made from the theme folder. Edit theme.conf there, not this. */\n")
	fmt.Fprintf(&b, ":root { --hero-shade: %s; --hero-focus: %s; }\n", trimNum(s.Shade), s.Focus)
	if th.Background != "" {
		fmt.Fprintf(&b, ".sky { background: url(%q) var(--hero-focus) / cover no-repeat; }\n", th.Background)
	}
	if th.BackgroundPhone != "" {
		fmt.Fprintf(&b, "@media (max-width: 640px) { .sky { background: url(%q) var(--hero-focus) / cover no-repeat; } }\n", th.BackgroundPhone)
	}
	if th.Page != "" {
		fmt.Fprintf(&b, "body { background: transparent; }\n")
		fmt.Fprintf(&b, "body::before { content: \"\"; position: fixed; inset: 0; z-index: -1; pointer-events: none; "+
			"background: url(%q) center / cover no-repeat; opacity: %s; }\n", th.Page, trimNum(1-s.PageShade))
		fmt.Fprintf(&b, ".band-alt { background: rgba(7, 12, 22, .6); }\n")
	}
	if s.Particles == "none" || s.Count == 0 {
		return []byte(b.String())
	}

	rnd := rand.New(rand.NewSource(int64(s.Count)*7919 + int64(len(th.Sprites))))
	span := func(lo, hi float64) float64 { return lo + (hi-lo)*rnd.Float64() }
	for i := 1; i <= s.Count; i++ {
		// Spread them across the width without bunching: one per slot, jittered.
		x := (float64(i-1) + span(.15, .85)) / float64(s.Count) * 100
		depth := math.Pow(rnd.Float64(), 1.6) // most are small and far
		w := float64(s.SizeMin) + float64(s.SizeMax-s.SizeMin)*depth
		o := s.Opacity * (.55 + .45*depth)

		var d, dx, dy, r0, r1, s1 float64
		unit := "cqw"
		switch s.Particles {
		case "rise", "fall":
			d = span(22, 38) * (1.15 - .3*depth) / s.Speed
			if s.Particles == "rise" && s.Target >= 0 {
				dx, s1 = (s.Target-x)*.6, .35
			} else {
				dx, s1 = span(-6, 6), 1
			}
			if s.Spin {
				r0 = span(-30, 30)
				r1 = r0 + span(60, 160)*float64(1-2*rnd.Intn(2))
			} else {
				r0 = span(-15, 15)
				r1 = r0
			}
		case "float":
			d = span(5, 11) / s.Speed
			dx, dy, unit, s1 = span(-12, 12), -span(8, 26), "px", 1
			r0 = span(-12, 12)
			r1 = r0
			if s.Spin {
				r1 = r0 + span(-10, 10)
			}
		}
		fmt.Fprintf(&b, ".particles i:nth-child(%d) { --x: %s%%; --w: %spx; --o: %s; --d: %ss; --delay: -%ss; "+
			"--dx: %s%s; --dy: %spx; --r0: %sdeg; --r1: %sdeg; --s1: %s; --y: %s%%; --still: %s;",
			i, trimNum(x), trimNum(w), trimNum(o), trimNum(d), trimNum(span(0, d)),
			trimNum(dx), unit, trimNum(dy), trimNum(r0), trimNum(r1), trimNum(s1), trimNum(span(6, 72)), trimNum(span(12, 80)))
		if len(th.Sprites) > 0 {
			fmt.Fprintf(&b, " --sprite: url(%q);", th.Sprites[(i-1)%len(th.Sprites)])
		} else {
			fmt.Fprintf(&b, " --shape: %s;", shardShapes[(i*5)%len(shardShapes)])
		}
		b.WriteString(" }\n")
	}
	return []byte(b.String())
}

// serveThemeFile sends one file from the theme folder. Only files the
// theme uses are served; nothing else in the folder is reachable.
func (s *Server) serveThemeFile(w http.ResponseWriter, r *http.Request) {
	th, files, _ := s.theme.get()
	name := r.PathValue("name")
	p, ok := files[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	if t, ok := themeTypes[strings.ToLower(path.Ext(name))]; ok {
		w.Header().Set("Content-Type", t)
	}
	if r.URL.Query().Get("v") == th.Stamp {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=60")
	}
	http.ServeContent(w, r, name, info.ModTime(), f)
}

func (s *Server) serveThemeCSS(w http.ResponseWriter, r *http.Request) {
	th, _, css := s.theme.get()
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	if r.URL.Query().Get("v") == th.Stamp {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Write(css)
}

package site

import (
	"html"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The page's own words live in a few HTML files, so they can be changed
// without touching the code: the defaults are built in, and a file of the
// same name in CONTENT_DIR replaces one. Edits are picked up within a few
// seconds; no restart needed.
//
// They are trusted HTML, written by whoever runs the site, with a few
// placeholders filled in: {{REALM}}, {{REALMLIST}}, {{DOWNLOAD_URL}},
// {{DISCORD_URL}} and {{GAME_VERSION}}.
var contentFiles = []string{"connect.html", "about.html", "bots.html", "rules.html"}

// botFiles are the parts about playerbots. Without playerbots the bots
// part never shows, and the about part shows only if the site has its
// own, since the built-in one describes a playerbots server.
var botFiles = map[string]bool{"about.html": true, "bots.html": true}

type content struct {
	defaults fs.FS
	dir      string
	tokens   *strings.Replacer
	noBots   bool

	mu      sync.Mutex
	checked time.Time
	stamps  map[string]time.Time
	pages   map[string]template.HTML
}

func newContent(defaults fs.FS, dir string, tokens map[string]string, noBots bool) *content {
	var pairs []string
	for k, v := range tokens {
		pairs = append(pairs, "{{"+k+"}}", html.EscapeString(v))
	}
	c := &content{defaults: defaults, dir: dir, tokens: strings.NewReplacer(pairs...), noBots: noBots, stamps: map[string]time.Time{}}
	c.load()
	return c
}

// get returns the current fragments.
func (c *content) get() map[string]template.HTML {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.checked) > 5*time.Second {
		c.checked = time.Now()
		if c.changed() {
			c.loadLocked()
		}
	}
	return c.pages
}

func (c *content) changed() bool {
	if c.dir == "" {
		return false
	}
	for _, name := range contentFiles {
		var mod time.Time
		if st, err := os.Stat(filepath.Join(c.dir, name)); err == nil {
			mod = st.ModTime()
		}
		if !mod.Equal(c.stamps[name]) {
			return true
		}
	}
	return false
}

func (c *content) load() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checked = time.Now()
	c.loadLocked()
}

func (c *content) loadLocked() {
	pages := map[string]template.HTML{}
	for _, name := range contentFiles {
		var data []byte
		c.stamps[name] = time.Time{}
		if c.dir != "" {
			p := filepath.Join(c.dir, name)
			if st, err := os.Stat(p); err == nil {
				if b, err := os.ReadFile(p); err == nil {
					data = b
					c.stamps[name] = st.ModTime()
				}
			}
		}
		if data == nil && !(c.noBots && botFiles[name]) {
			data, _ = fs.ReadFile(c.defaults, name)
		}
		if c.noBots && name == "bots.html" {
			data = nil
		}
		// An empty file (or one with only spaces) hides its section.
		text := strings.TrimSpace(string(data))
		// Trusted: the site owner's own HTML.
		pages[strings.TrimSuffix(name, ".html")] = template.HTML(c.tokens.Replace(text))
	}
	c.pages = pages
}

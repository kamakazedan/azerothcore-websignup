package game

import (
	"strings"
	"testing"
	"time"
)

func TestUsername(t *testing.T) {
	r := Rules{MinPassword: 8, BotPrefix: "RNDBOT"}
	for _, ok := range []string{"abc", "Arthas", "Jaina1999", "SIXTEENCHARSLONG"} {
		if err := r.Username(ok); err != nil {
			t.Errorf("Username(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "ab", "SEVENTEENCHARSXXX", "has space", "dash-name", "naïve", "rndbot1", "RndBot", "RNDBOTANYTHING"} {
		if err := r.Username(bad); err == nil {
			t.Errorf("Username(%q) passed", bad)
		} else if fe, ok := AsFieldError(err); !ok || fe.Field != "username" {
			t.Errorf("Username(%q) error isn't for the username field: %v", bad, err)
		}
	}
	if err := (Rules{}).Username("rndbot1"); err != nil {
		t.Errorf("with no bot prefix, any name is allowed: %v", err)
	}
}

func TestPassword(t *testing.T) {
	r := Rules{MinPassword: 8}
	for _, ok := range []string{"icecrown", "sixteen-chars!!!", "P@ss~w0rd"} {
		if err := r.Password("arthas", ok); err != nil {
			t.Errorf("Password(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"short", "seventeen-chars!!", "has space", "tab\there", "pässword", "ARTHAS", "Arthas"} {
		if err := r.Password("arthas", bad); err == nil {
			t.Errorf("Password(%q) passed", bad)
		}
	}
	// The minimum is kept between 4 and 16.
	if err := (Rules{MinPassword: 1}).Password("x", "abc"); err == nil {
		t.Error("a 3-character password passed")
	}
	if err := (Rules{MinPassword: 40}).Password("x", "sixteen-chars!!!"); err != nil {
		t.Errorf("a minimum over 16 should mean 16: %v", err)
	}
}

func TestEmail(t *testing.T) {
	for in, want := range map[string]string{
		"jaina@example.com":       "JAINA@EXAMPLE.COM",
		"  thrall@orgrimmar.net ": "THRALL@ORGRIMMAR.NET",
		"a.b+c@sub.example.co.uk": "A.B+C@SUB.EXAMPLE.CO.UK",
	} {
		if got, err := Email(in); err != nil || got != want {
			t.Errorf("Email(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "plain", "a@b", "a@.com", "a@com.", "@example.com", "Jaina <jaina@example.com>",
		"jäina@example.com", "a b@example.com", strings.Repeat("a", 250) + "@example.com"} {
		if _, err := Email(bad); err == nil {
			t.Errorf("Email(%q) passed", bad)
		}
	}
}

func TestNames(t *testing.T) {
	c := newCharacter("Lahu", 8, 7, 71, 3*86400+4*3600, true)
	if c.Race != "Troll" || c.Class != "Shaman" || c.ClassID != "shaman" || c.Faction != "horde" || c.FactionName() != "Horde" || c.Played != "3d 4h" {
		t.Errorf("newCharacter = %+v", c)
	}
	c = newCharacter("Odd", 99, 99, 1, 0, false)
	if c.Race != "Unknown" || c.ClassID != "unknown" || c.Played != "" || c.FactionName() != "Alliance" {
		t.Errorf("unknown race and class = %+v", c)
	}
	for d, want := range map[time.Duration]string{40 * time.Minute: "40m", 5*time.Hour + 12*time.Minute: "5h 12m", 52 * time.Hour: "2d 4h"} {
		if got := PlayedTime(d); got != want {
			t.Errorf("PlayedTime(%v) = %q", d, got)
		}
	}
	for icon, want := range map[int]string{0: "PvE", 1: "PvP", 4: "PvE", 6: "RP", 8: "RP-PvP", 16: "PvP"} {
		if got := RealmType(icon); got != want {
			t.Errorf("RealmType(%d) = %q", icon, got)
		}
	}
}

func TestLikePrefix(t *testing.T) {
	for in, want := range map[string]string{"RNDBOT": "RNDBOT%", "A_B%C": `A\_B\%C%`, `x\y`: `x\\y%`} {
		if got := likePrefix(in); got != want {
			t.Errorf("likePrefix(%q) = %q", in, got)
		}
	}
	if p := likePrefix(""); strings.HasSuffix(p, "%") {
		t.Errorf("an empty prefix must match nothing, got %q", p)
	}
}

func TestSchemaNames(t *testing.T) {
	if _, err := NewMySQL(nil, Options{AuthDB: "acore_auth", CharactersDB: "acore_characters"}); err != nil {
		t.Error(err)
	}
	for _, bad := range []string{"", "acore`auth", "acore auth", "a;DROP", strings.Repeat("a", 65)} {
		if _, err := NewMySQL(nil, Options{AuthDB: bad, CharactersDB: "acore_characters"}); err == nil {
			t.Errorf("schema name %q accepted", bad)
		}
	}
}

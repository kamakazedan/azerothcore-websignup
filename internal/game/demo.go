package game

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Demo is a store with no database behind it, for previewing the site
// (DEMO_MODE=1). Accounts "created" here exist until the process stops.
type Demo struct {
	mu       sync.Mutex
	accounts map[string][2]string // NAME -> {password, EMAIL}
	started  time.Time
}

func NewDemo() *Demo {
	return &Demo{accounts: map[string][2]string{"ARTHAS": {"FROSTMOURNE", "ARTHAS@EXAMPLE.COM"}}, started: time.Now().Add(-52 * time.Hour)}
}

func (d *Demo) Ping(context.Context) error { return nil }

func (d *Demo) CreateAccount(_ context.Context, username, password, email string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	name := strings.ToUpper(username)
	if _, ok := d.accounts[name]; ok {
		return ErrUsernameTaken
	}
	for _, a := range d.accounts {
		if a[1] == email {
			return ErrEmailTaken
		}
	}
	d.accounts[name] = [2]string{strings.ToUpper(password), email}
	return nil
}

func (d *Demo) ChangePassword(_ context.Context, username, oldPassword, newPassword string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	name := strings.ToUpper(username)
	a, ok := d.accounts[name]
	if !ok || a[0] != strings.ToUpper(oldPassword) {
		return ErrBadLogin
	}
	a[0] = strings.ToUpper(newPassword)
	d.accounts[name] = a
	return nil
}

func (d *Demo) Snapshot(context.Context) (Snapshot, error) {
	c := func(name string, race, class uint8, level int, hours uint32) Character {
		return newCharacter(name, race, class, level, hours*3600, true)
	}
	online := []Character{c("Brelac", 1, 2, 80, 0), c("Lahu", 8, 7, 71, 0), c("Frostwhisper", 10, 8, 52, 0)}
	for i := range online {
		online[i].Played = ""
	}
	return Snapshot{
		RealmName: "Dangaming",
		RealmType: "PvE",
		UpSince:   d.started,
		Players:   len(online),
		Bots:      1214,
		Online:    online,
		Top: []Character{
			c("Brelac", 1, 2, 80, 412), c("Morvaine", 5, 6, 80, 288), c("Lahu", 8, 7, 71, 190),
			c("Thistlewick", 7, 9, 64, 120), c("Frostwhisper", 10, 8, 52, 96), c("Oakenhide", 6, 11, 38, 41),
		},
	}, nil
}

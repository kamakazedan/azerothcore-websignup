// Package ratelimit counts events per key in a sliding window, in memory.
//
// One process, one site: there is nothing to share between replicas, and a
// restart forgetting the counts is fine.
//
// A limiter tracks at most MaxKeys keys at once. Past that it refuses new
// keys rather than grow, so a flood of addresses costs a spell of refusals,
// not the server's memory.
package ratelimit

import (
	"sync"
	"time"
)

// MaxKeys is how many keys one limiter tracks at most: about 2 MB.
const MaxKeys = 20000

type Limiter struct {
	max     int
	window  time.Duration
	maxKeys int
	now     func() time.Time

	mu        sync.Mutex
	hits      map[string][]time.Time
	lastPrune time.Time
}

// New allows max events per key in any window.
func New(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, maxKeys: MaxKeys, now: time.Now, hits: map[string][]time.Time{}}
}

func (l *Limiter) recent(key string, now time.Time) []time.Time {
	cutoff := now.Add(-l.window)
	h := l.hits[key]
	i := 0
	for i < len(h) && !h[i].After(cutoff) {
		i++
	}
	return h[i:]
}

// Allow records an event for key if it is within the limit, and says
// whether it was. The check and the record are one step, so requests that
// arrive together can't all slip in under the limit.
func (l *Limiter) Allow(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	h := l.recent(key, now)
	if len(h) >= l.max {
		l.hits[key] = h
		return false
	}
	if !l.room(key, now) {
		return false
	}
	l.hits[key] = append(h, now)
	return true
}

// room says whether key can be tracked: it already is, or there's space.
// A full map is pruned first, at most once a second.
func (l *Limiter) room(key string, now time.Time) bool {
	if _, ok := l.hits[key]; ok || len(l.hits) < l.maxKeys {
		return true
	}
	if now.Sub(l.lastPrune) >= time.Second {
		l.pruneLocked(now)
	}
	return len(l.hits) < l.maxKeys
}

// Undo takes back the latest event for key: a slot Allow reserved for
// something that then didn't happen.
func (l *Limiter) Undo(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch h := l.hits[key]; len(h) {
	case 0:
	case 1:
		delete(l.hits, key)
	default:
		l.hits[key] = h[:len(h)-1]
	}
}

// Blocked says whether key is at its limit, without recording anything.
func (l *Limiter) Blocked(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key, now)) >= l.max
}

// Reset forgets a key: a successful login clears its failures.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}

// Prune drops keys with nothing left in the window.
func (l *Limiter) Prune() {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)
}

func (l *Limiter) pruneLocked(now time.Time) {
	l.lastPrune = now
	for k := range l.hits {
		if h := l.recent(k, now); len(h) == 0 {
			delete(l.hits, k)
		} else {
			l.hits[k] = h
		}
	}
}

package ratelimit

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	l := New(3, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("event %d refused", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("the fourth event in a minute is allowed")
	}
	if !l.Allow("b") {
		t.Fatal("keys are counted separately")
	}
	if !l.Blocked("a") || l.Blocked("b") {
		t.Fatal("Blocked disagrees with Allow")
	}

	now = now.Add(61 * time.Second)
	if !l.Allow("a") {
		t.Fatal("the window does not slide")
	}
}

func TestUndoResetPrune(t *testing.T) {
	now := time.Unix(1000, 0)
	l := New(2, time.Minute)
	l.now = func() time.Time { return now }

	l.Allow("u")
	l.Allow("u")
	if !l.Blocked("u") {
		t.Fatal("two events should block")
	}
	l.Undo("u")
	if l.Blocked("u") {
		t.Fatal("Undo does not give the slot back")
	}
	l.Undo("u")
	l.Undo("u") // nothing left: no harm
	if _, ok := l.hits["u"]; ok {
		t.Fatal("an emptied key should go")
	}

	l.Allow("v")
	l.Allow("v")
	l.Reset("v")
	if l.Blocked("v") {
		t.Fatal("Reset does not clear")
	}

	l.Allow("x")
	now = now.Add(2 * time.Minute)
	l.Prune()
	if len(l.hits) != 0 {
		t.Fatalf("Prune kept %d keys", len(l.hits))
	}
}

// Requests that arrive together get no more than the limit between them.
func TestConcurrent(t *testing.T) {
	l := New(5, time.Hour)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("k") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 5 {
		t.Fatalf("%d of 200 simultaneous events allowed, want 5", allowed)
	}
}

// A flood of new keys is refused once the limiter is full, instead of
// growing; known keys carry on, and room comes back as keys expire.
func TestMaxKeys(t *testing.T) {
	now := time.Unix(1000, 0)
	l := New(3, time.Minute)
	l.now = func() time.Time { return now }
	l.maxKeys = 100

	l.Allow("regular")
	for i := 0; i < 500; i++ {
		l.Allow("flood" + strconv.Itoa(i))
	}
	if len(l.hits) != 100 {
		t.Fatalf("tracking %d keys, cap is 100", len(l.hits))
	}
	if l.Allow("newcomer") {
		t.Fatal("a new key was let in while full")
	}
	if !l.Allow("regular") {
		t.Fatal("a known key was refused")
	}

	now = now.Add(2 * time.Minute)
	if !l.Allow("newcomer") {
		t.Fatal("no room once the flood expired")
	}
	if len(l.hits) != 1 {
		t.Fatalf("%d keys after pruning, want 1", len(l.hits))
	}
}

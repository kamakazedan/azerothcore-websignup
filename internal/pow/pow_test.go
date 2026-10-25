package pow

import (
	"strconv"
	"testing"
	"time"
)

func solve(t *testing.T, challenge string, bits int) string {
	t.Helper()
	for n := 0; n < 1<<26; n++ {
		s := strconv.Itoa(n)
		if LeadingZeroBits(challenge, s) >= bits {
			return s
		}
	}
	t.Fatal("no solution found")
	return ""
}

func TestRoundTrip(t *testing.T) {
	is := New([]byte("k"), 12, time.Minute)
	c, err := is.Issue()
	if err != nil {
		t.Fatal(err)
	}
	n := solve(t, c, 12)
	if err := is.Verify(c, n); err != nil {
		t.Fatalf("a solved challenge is refused: %v", err)
	}
	if err := is.Verify(c, n); err != ErrUsed {
		t.Fatalf("a challenge can be used twice: %v", err)
	}
}

func TestNotEnoughWork(t *testing.T) {
	is := New([]byte("k"), 20, time.Minute)
	c, _ := is.Issue()
	for n := 0; n < 1000; n++ {
		s := strconv.Itoa(n)
		if LeadingZeroBits(c, s) < 20 {
			if err := is.Verify(c, s); err != ErrWork {
				t.Fatalf("an unsolved nonce gives %v", err)
			}
			return
		}
	}
}

func TestForgedAndExpired(t *testing.T) {
	is := New([]byte("k"), 4, time.Minute)
	other := New([]byte("other key"), 4, time.Minute)
	c, _ := other.Issue()
	if err := is.Verify(c, solve(t, c, 4)); err != ErrSignature {
		t.Fatalf("someone else's challenge gives %v", err)
	}

	c, _ = is.Issue()
	n := solve(t, c, 4)
	is.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if err := is.Verify(c, n); err != ErrExpired {
		t.Fatalf("an old challenge gives %v", err)
	}

	for _, bad := range [][2]string{{"", "1"}, {"abc", "1"}, {c, ""}, {c, "-1"}, {c, "1e9"}, {"a.b", "1"}, {c + "x", "1"}} {
		if err := is.Verify(bad[0], bad[1]); err == nil {
			t.Errorf("%q/%q accepted", bad[0], bad[1])
		}
	}
}

func TestDifficultyIsSigned(t *testing.T) {
	is := New([]byte("k"), 16, time.Minute)
	c, _ := is.Issue()
	// Lowering the difficulty byte breaks the signature.
	raw := []byte(c)
	raw[30] ^= 1
	if err := is.Verify(string(raw), "1"); err == nil {
		t.Fatal("a tampered challenge is accepted")
	}
}

func TestForget(t *testing.T) {
	is := New([]byte("k"), 1, time.Minute)
	c, _ := is.Issue()
	if err := is.Verify(c, solve(t, c, 1)); err != nil {
		t.Fatal(err)
	}
	is.now = func() time.Time { return time.Now().Add(time.Hour) }
	is.Forget()
	if len(is.used) != 0 {
		t.Fatal("expired challenges are kept")
	}
}

func TestLeadingZeroBits(t *testing.T) {
	// SHA-256("abc:0") = 5f36efce...: 0x5f is 0101 1111, one leading zero bit.
	if got := LeadingZeroBits("abc", "0"); got != 1 {
		t.Fatalf("got %d", got)
	}
}

// One issued challenge can't be dressed up as several: base64 decoding
// forgives line breaks and spare low bits, so other spellings decode to the
// same bytes. None of them may pass, before or after the real one is used.
func TestOneSpellingOnly(t *testing.T) {
	is := New([]byte("k"), 4, time.Minute)
	c, _ := is.Issue()
	dot := 34
	if c[dot] != '.' {
		t.Fatalf("unexpected challenge shape %q", c)
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var variants []string
	// The last character of each part carries spare bits: try every value.
	for _, at := range []int{dot - 1, len(c) - 1} {
		for _, ch := range alphabet {
			if byte(ch) != c[at] {
				variants = append(variants, c[:at]+string(ch)+c[at+1:])
			}
		}
	}
	// Line breaks, which the decoder skips.
	variants = append(variants, c[:10]+"\n"+c[10:], c[:10]+"\r\n"+c[10:], c+"\n", c[:dot]+"\n."+c[dot+1:])

	check := func(when string) {
		for _, v := range variants {
			n := solve(t, v, 4)
			if err := is.Verify(v, n); err == nil {
				t.Errorf("%s: variant %q accepted", when, v)
			}
		}
	}
	check("before use")
	if err := is.Verify(c, solve(t, c, 4)); err != nil {
		t.Fatalf("the real challenge is refused: %v", err)
	}
	check("after use")
}

// Raising the difficulty retires challenges issued at the old one.
func TestDifficultyRaised(t *testing.T) {
	low := New([]byte("k"), 2, time.Minute)
	c, _ := low.Issue()
	high := New([]byte("k"), 10, time.Minute)
	if err := high.Verify(c, solve(t, c, 2)); err == nil {
		t.Fatal("an easier challenge than the current difficulty passed")
	}
}

func TestUsedListCap(t *testing.T) {
	is := New([]byte("k"), 0, time.Minute)
	for i := 0; i < maxUsed; i++ {
		is.used[strconv.Itoa(i)] = time.Now().Add(time.Minute)
	}
	c, _ := is.Issue()
	if err := is.Verify(c, "0"); err != ErrBusy {
		t.Fatalf("with the list full: %v, want ErrBusy", err)
	}
	is.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	c, _ = is.Issue()
	if err := is.Verify(c, "0"); err != nil {
		t.Fatalf("expired entries should make room: %v", err)
	}
}

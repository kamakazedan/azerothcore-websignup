// Package pow is a small proof-of-work check that stands in for a captcha.
//
// The server hands out a signed challenge; the browser finds a number n such
// that SHA-256(challenge ":" n) starts with Bits zero bits, which takes a
// second or two of a phone's time and nothing of the person's. A script that
// wants a thousand accounts has to pay for a thousand of those, on top of the
// rate limits. No third party is involved and nothing is stored in the
// browser.
//
// A challenge is good for one use and expires after TTL.
package pow

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math/bits"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrMalformed = errors.New("malformed challenge")
	ErrSignature = errors.New("challenge was not issued here")
	ErrExpired   = errors.New("challenge expired")
	ErrUsed      = errors.New("challenge already used")
	ErrWork      = errors.New("not enough work")
	ErrBusy      = errors.New("too many challenges in use")
)

type Issuer struct {
	key  []byte
	bits uint8
	ttl  time.Duration
	now  func() time.Time

	mu   sync.Mutex
	used map[string]time.Time // a challenge's random bytes -> its expiry
}

// maxUsed caps the used-challenge list (about 8 MB). Each entry took a
// solved challenge to make, so it fills only under a determined flood, and
// then new submissions wait until old challenges expire.
const maxUsed = 100000

// New makes an issuer. bits is the difficulty: each extra bit doubles the
// average work (16 is about 65 000 hashes).
func New(key []byte, bits uint8, ttl time.Duration) *Issuer {
	if bits > 30 {
		bits = 30
	}
	return &Issuer{key: key, bits: bits, ttl: ttl, now: time.Now, used: map[string]time.Time{}}
}

// Bits is the current difficulty.
func (s *Issuer) Bits() uint8 { return s.bits }

// payload: 16 random bytes | expiry, unix seconds (8) | bits (1)
const payloadLen = 16 + 8 + 1

// A challenge is base64url(payload) "." base64url(HMAC-SHA256): always
// this long.
var challengeLen = base64.RawURLEncoding.EncodedLen(payloadLen) + 1 + base64.RawURLEncoding.EncodedLen(sha256.Size)

// Issue returns a fresh challenge.
func (s *Issuer) Issue() (string, error) {
	p := make([]byte, payloadLen)
	if _, err := rand.Read(p[:16]); err != nil {
		return "", err
	}
	binary.BigEndian.PutUint64(p[16:24], uint64(s.now().Add(s.ttl).Unix()))
	p[24] = s.bits
	enc := base64.RawURLEncoding
	return enc.EncodeToString(p) + "." + enc.EncodeToString(s.sign(p)), nil
}

func (s *Issuer) sign(p []byte) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte("pow-v1"))
	m.Write(p)
	return m.Sum(nil)
}

// Verify checks a solved challenge and uses it up.
//
// Only the exact text Issue produced is accepted: base64 decoding forgives
// line breaks and stray low bits, and without this check one challenge
// could be spelled many ways and used once per spelling.
func (s *Issuer) Verify(challenge, nonce string) error {
	if len(challenge) != challengeLen || len(nonce) == 0 || len(nonce) > 20 {
		return ErrMalformed
	}
	if _, err := strconv.ParseUint(nonce, 10, 64); err != nil {
		return ErrMalformed
	}
	dot := strings.IndexByte(challenge, '.')
	if dot != base64.RawURLEncoding.EncodedLen(payloadLen) {
		return ErrMalformed
	}
	for i := 0; i < len(challenge); i++ {
		if c := challenge[i]; i != dot && !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return ErrMalformed
		}
	}
	enc := base64.RawURLEncoding.Strict()
	p, err1 := enc.DecodeString(challenge[:dot])
	sig, err2 := enc.DecodeString(challenge[dot+1:])
	if err1 != nil || err2 != nil || len(p) != payloadLen {
		return ErrMalformed
	}
	if !hmac.Equal(sig, s.sign(p)) {
		return ErrSignature
	}
	now := s.now()
	exp := time.Unix(int64(binary.BigEndian.Uint64(p[16:24])), 0)
	if !now.Before(exp) {
		return ErrExpired
	}
	// p[24] is the difficulty it was issued at; it must still be enough.
	if p[24] < s.bits || LeadingZeroBits(challenge, nonce) < int(p[24]) {
		return ErrWork
	}

	id := string(p[:16])
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, seen := s.used[id]; seen {
		return ErrUsed
	}
	if len(s.used) >= maxUsed {
		s.forgetLocked(now)
		if len(s.used) >= maxUsed {
			return ErrBusy
		}
	}
	s.used[id] = exp
	return nil
}

// Forget drops used challenges that have expired anyway.
func (s *Issuer) Forget() {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forgetLocked(now)
}

func (s *Issuer) forgetLocked(now time.Time) {
	for c, exp := range s.used {
		if !now.Before(exp) {
			delete(s.used, c)
		}
	}
}

// LeadingZeroBits counts the zero bits at the start of SHA-256(challenge ":" nonce).
func LeadingZeroBits(challenge, nonce string) int {
	sum := sha256.Sum256([]byte(challenge + ":" + nonce))
	n := 0
	for _, b := range sum {
		if b == 0 {
			n += 8
			continue
		}
		n += bits.LeadingZeros8(b)
		break
	}
	return n
}

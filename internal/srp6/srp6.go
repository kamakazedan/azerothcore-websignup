// Package srp6 makes and checks AzerothCore account passwords.
//
// AzerothCore never stores a password, only a random 32-byte salt and an SRP6
// verifier made from it (src/common/Cryptography/Authentication/SRP6.cpp):
//
//	x = SHA1(salt || SHA1(USERNAME ":" PASSWORD)), read as a little-endian number
//	v = g^x mod N, stored as 32 little-endian bytes
//
// with g = 7 and N the 256-bit WoW modulus. Username and password are
// uppercased first, the way the game client and the core both do it.
package srp6

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"math/big"
	"strings"
)

const (
	SaltLength     = 32
	VerifierLength = 32
)

var (
	g = big.NewInt(7)
	n = mustHex("894B645E89E1535BBDAD5B8B290650530801B18EBFBF5E8FAB3C82872A3E9BB7")
)

func mustHex(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("srp6: bad modulus")
	}
	return v
}

// Normalize uppercases a username or password the way the core does
// (Utf8ToUpperOnlyLatin). Callers only allow ASCII, where that is plain
// uppercasing.
func Normalize(s string) string {
	return strings.ToUpper(s)
}

// Verifier returns the verifier for a username, password and salt.
func Verifier(username, password string, salt []byte) []byte {
	inner := sha1.Sum([]byte(Normalize(username) + ":" + Normalize(password)))

	h := sha1.New()
	h.Write(salt)
	h.Write(inner[:])
	x := new(big.Int).SetBytes(reverse(h.Sum(nil))) // little-endian digest

	v := new(big.Int).Exp(g, x, n)
	return littleEndian(v, VerifierLength)
}

// New makes a fresh random salt and its verifier for a new password.
func New(username, password string) (salt, verifier []byte, err error) {
	salt = make([]byte, SaltLength)
	if _, err = rand.Read(salt); err != nil {
		return nil, nil, err
	}
	return salt, Verifier(username, password, salt), nil
}

// Check reports whether password is the one the stored salt and verifier
// were made from.
func Check(username, password string, salt, verifier []byte) bool {
	if len(salt) != SaltLength || len(verifier) != VerifierLength {
		return false
	}
	return subtle.ConstantTimeCompare(Verifier(username, password, salt), verifier) == 1
}

func reverse(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return out
}

// littleEndian writes v as exactly size little-endian bytes.
func littleEndian(v *big.Int, size int) []byte {
	be := v.FillBytes(make([]byte, size))
	return reverse(be)
}

package srp6

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// Made by AzerothCore's own SRP6::MakeRegistrationData (core commit 3f2969ddb),
// so these pin the exact bytes the authserver expects.
var coreVectors = []struct {
	user, pass, salt, verifier string
}{
	{"ARTHAS", "FROSTMOURNE",
		"1fcccd6f2350c69f0902f252d5fd720f9e0b4f651349d15211eb2f678a7f50d1",
		"05d11f354dad4c30abf67b18e8296d0108333b872979e2f772e2a0172d2acb6b"},
	{"JAINA", "PROUDMOORE1",
		"56b494380ba58f030e07cbb9d4d51e40ed8b945d558def3f59d882b6d00dbee7",
		"a8996ca7ed7ad4361b8820baab2cbe09552556d5f2b918e1c4aa5a3cfd24be03"},
	{"A", "B",
		"4481796766a2f229f5164fd56df3261d82617a6b329c170831d971af89183051",
		"c9e3e31af5094cbc98c5fbe29349db4d9d09b608f614cc9c353006224be01610"},
	{"DANGAMING17CHARSX", "P@SS-WORD_16CHRS",
		"aed0d97e6ded1e86f1bc5d6177cf2444979bbd5f3de142a764a24a5e2b15b7ca",
		"9cacf305967fa8d7f0ffe0a1fc3216451c2ac666d19d4b0884c64808fe47986b"},
	{"THRALL", "12345678",
		"dc62c26ac0d6609a26eb547b8aee2c32daea9bb34ca66830022491815181a6a0",
		"40d83e767e99425cc98390431470c59d8ec092ea85f2a2c3e182f224e34b0071"},
	// The verifier's top byte is zero: it must still be 32 bytes.
	{"PADTEST", "PASSWORD1",
		"93355bdb7c9fbc3a12a9c330133cdc14a766bdace3135b0647b6446d2778d4cd",
		"f17842821210d41c3141a2ebf50c63a5a06a7ed447077fc68c46c9e1a4fe9b00"},
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMatchesTheCore(t *testing.T) {
	for _, v := range coreVectors {
		got := Verifier(v.user, v.pass, unhex(t, v.salt))
		if want := unhex(t, v.verifier); !bytes.Equal(got, want) {
			t.Errorf("%s/%s: verifier %x, the core made %x", v.user, v.pass, got, want)
		}
		if len(got) != VerifierLength {
			t.Errorf("%s: verifier is %d bytes", v.user, len(got))
		}
	}
}

func TestCaseDoesNotMatter(t *testing.T) {
	v := coreVectors[0]
	salt, want := unhex(t, v.salt), unhex(t, v.verifier)
	if !Check("arthas", "frostmourne", salt, want) || !Check("Arthas", "FrostMourne", salt, want) {
		t.Error("the client uppercases both, so any case must log in")
	}
}

func TestCheck(t *testing.T) {
	salt, verifier, err := New("Tester", "Secret12")
	if err != nil {
		t.Fatal(err)
	}
	if len(salt) != SaltLength || len(verifier) != VerifierLength {
		t.Fatalf("lengths %d/%d", len(salt), len(verifier))
	}
	if !Check("TESTER", "SECRET12", salt, verifier) {
		t.Error("the right password is refused")
	}
	if Check("TESTER", "SECRET13", salt, verifier) {
		t.Error("a wrong password is accepted")
	}
	if Check("TESTER2", "SECRET12", salt, verifier) {
		t.Error("the username is part of the verifier")
	}
	if Check("TESTER", "SECRET12", salt[:31], verifier) || Check("TESTER", "SECRET12", salt, nil) {
		t.Error("short salt or verifier must not pass")
	}
	salt2, verifier2, _ := New("Tester", "Secret12")
	if bytes.Equal(salt, salt2) || bytes.Equal(verifier, verifier2) {
		t.Error("every password gets its own salt")
	}
}

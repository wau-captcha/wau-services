package biz

import (
	"crypto/sha256"
	"strconv"
	"strings"
	"testing"
)

// solvePow counts up from 0 like the SDKs and returns the first nonce that solves the proof of work.
func solvePow(salt []byte, challengeID string, difficulty int) string {
	for nonce := 0; ; nonce++ {
		if candidate := strconv.Itoa(nonce); powSolved(salt, challengeID, candidate, difficulty) {
			return candidate
		}
	}
}

// failingPow returns a decimal nonce that does not solve the proof of work.
func failingPow(salt []byte, challengeID string, difficulty int) string {
	for nonce := 0; ; nonce++ {
		if candidate := strconv.Itoa(nonce); !powSolved(salt, challengeID, candidate, difficulty) {
			return candidate
		}
	}
}

func TestLeadingZeroBits(t *testing.T) {
	tests := []struct {
		sum  []byte
		want int
	}{
		{[]byte{0x80, 0x00}, 0},
		{[]byte{0x01}, 7},
		{[]byte{0x00, 0x0f}, 12},
		{[]byte{0x00, 0x00, 0x7f}, 17},
		{[]byte{0x00, 0x00}, 16},
		{nil, 0},
	}
	for _, test := range tests {
		if got := leadingZeroBits(test.sum); got != test.want {
			t.Errorf("leadingZeroBits(%x) = %d, want %d", test.sum, got, test.want)
		}
	}
}

// The vector of the SDK test (packages/client/test/pow.test.mjs): both must find the same nonce.
func TestPowSolved(t *testing.T) {
	salt := []byte("0123456789abcdef")
	const challengeID = "0f9c6b5e-1d2a-4c3b-8e7f-9a0b1c2d3e4f"
	nonce := solvePow(salt, challengeID, 12)
	// SHA-256 = 00092f9d…: exactly 12 zero bits.
	if nonce != "1233" {
		t.Fatalf("first nonce at 12 bits = %s, want the SDK's 1233", nonce)
	}
	sum := sha256.Sum256([]byte(string(salt) + challengeID + nonce))
	if leadingZeroBits(sum[:]) < 12 {
		t.Fatalf("SHA-256(salt ‖ id ‖ %s) = %x", nonce, sum)
	}
	tests := []struct {
		name       string
		nonce      string
		difficulty int
		solved     bool
	}{
		{"the nonce found", nonce, 12, true},
		{"too few zero bits for more", nonce, 24, false},
		{"no work asked", "0", 0, true},
		{"empty", "", 0, false},
		{"not decimal", "12a", 0, false},
		{"signed", "+1", 0, false},
		{"negative", "-1", 0, false},
		{"21 digits", strings.Repeat("1", 21), 0, false},
		{"20 digits", strings.Repeat("1", 20), 0, true},
	}
	for _, test := range tests {
		if got := powSolved(salt, challengeID, test.nonce, test.difficulty); got != test.solved {
			t.Errorf("%s: powSolved(%q, %d) = %v, want %v", test.name, test.nonce, test.difficulty, got, test.solved)
		}
	}
	if powSolved(salt, "1f9c6b5e-1d2a-4c3b-8e7f-9a0b1c2d3e4f", nonce, 12) {
		t.Error("the nonce of one challenge solves another")
	}
}

package biz

import (
	"crypto/rand"
	"crypto/sha256"
	"math/bits"
)

const (
	powSaltBytes = 16
	// Each bit doubles the work: 32 would keep a phone busy for minutes.
	maxPowDifficulty = 32
	// The SDKs count up from 0 in decimal; no search gets near 20 digits.
	maxPowNonceLength = 20
)

func newPowSalt() []byte {
	salt := make([]byte, powSaltBytes)
	rand.Read(salt)
	return salt
}

// powSolved reports whether nonce, a decimal number, solves the proof of work of a challenge:
// SHA-256(salt ‖ challengeID ‖ nonce) starts with difficulty zero bits.
func powSolved(salt []byte, challengeID, nonce string, difficulty int) bool {
	if nonce == "" || len(nonce) > maxPowNonceLength {
		return false
	}
	for _, digit := range nonce {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	hash := sha256.New()
	hash.Write(salt)
	hash.Write([]byte(challengeID))
	hash.Write([]byte(nonce))
	return leadingZeroBits(hash.Sum(nil)) >= difficulty
}

func leadingZeroBits(sum []byte) int {
	zeros := 0
	for _, b := range sum {
		if b != 0 {
			return zeros + bits.LeadingZeros8(b)
		}
		zeros += 8
	}
	return zeros
}

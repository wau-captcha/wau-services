package biz

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"time"
)

const tokenIDSize = 16

// newToken returns a random token id and the token
// base64url(id | expiry) "." base64url(HMAC-SHA256).
func newToken(secret []byte, expiresAt time.Time) (id, token string) {
	payload := make([]byte, tokenIDSize, tokenIDSize+8)
	rand.Read(payload)
	payload = binary.BigEndian.AppendUint64(payload, uint64(expiresAt.Unix()))
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	return hex.EncodeToString(payload[:tokenIDSize]), encodedPayload + "." + base64.RawURLEncoding.EncodeToString(tokenMAC(secret, encodedPayload))
}

// parseToken returns the token id when the signature is valid and the token has not expired.
func parseToken(secret []byte, token string, now time.Time) (string, bool) {
	encodedPayload, signature, _ := strings.Cut(token, ".")
	mac, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !hmac.Equal(mac, tokenMAC(secret, encodedPayload)) {
		return "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil || len(payload) != tokenIDSize+8 || now.Unix() >= int64(binary.BigEndian.Uint64(payload[tokenIDSize:])) {
		return "", false
	}
	return hex.EncodeToString(payload[:tokenIDSize]), true
}

func tokenMAC(secret []byte, payload string) []byte {
	hasher := hmac.New(sha256.New, secret)
	hasher.Write([]byte(payload))
	return hasher.Sum(nil)
}

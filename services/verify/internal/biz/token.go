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
	p := base64.RawURLEncoding.EncodeToString(payload)
	return hex.EncodeToString(payload[:tokenIDSize]), p + "." + base64.RawURLEncoding.EncodeToString(tokenMAC(secret, p))
}

// parseToken returns the token id when the signature is valid and the token has not expired.
func parseToken(secret []byte, token string, now time.Time) (string, bool) {
	p, sig, _ := strings.Cut(token, ".")
	mac, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(mac, tokenMAC(secret, p)) {
		return "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil || len(payload) != tokenIDSize+8 || now.Unix() >= int64(binary.BigEndian.Uint64(payload[tokenIDSize:])) {
		return "", false
	}
	return hex.EncodeToString(payload[:tokenIDSize]), true
}

func tokenMAC(secret []byte, payload string) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	return h.Sum(nil)
}

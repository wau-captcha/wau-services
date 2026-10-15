package biz

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"challenge/internal/pkg/quota"

	"github.com/go-kratos/kratos/v3/log"
)

// A clearance lets a browser that solved a puzzle of a key pass auto_check on that key without
// one, for clearance_ttl and a few passes an hour and a day. Every probe of auto_check runs in a
// browser a bot controls and can be faked; a solved puzzle cannot, and each one buys only a few
// passes. Check hands one out with a right answer to an interactive type, the SDK keeps it under
// the site's origin and sends it back in its auto_check answers ("c"). It is signed, not stored:
// Redis only counts the passes each one gave.
//
// A clearance is base64url(payload) "." base64url(HMAC-SHA256 of the encoded payload), the payload
// being the version, a random id, the API key id after its length in one byte, the first bytes of
// the SHA-256 of the User-Agent (userAgentDigest), then the issue time and the expiry in Unix
// seconds, big-endian.
const (
	clearanceVersion = 1
	clearanceIDSize  = 16
	clearanceUASize  = 16
	// API key ids are UUIDs, 36 characters; up to 64 a clearance stays under 200 characters.
	maxClearanceKeyID = 64
	// The longest clearance an answer may carry.
	maxClearanceLength = 256
	// The signing key is HMAC-SHA256(secret, clearanceKeyLabel), and verify signs its tokens with
	// the secret itself: a clearance never passes for a token, nor a token for a clearance.
	clearanceKeyLabel = "wau-auto-check-clearance-v1"
)

// clearanceFixedSize is the payload without its API key id.
const clearanceFixedSize = 1 + clearanceIDSize + 1 + clearanceUASize + 8 + 8

// ClearanceRepo counts the automatic passes each clearance gave, in Redis (see data/clearance.go).
type ClearanceRepo interface {
	// Passes counts the passes of the clearance id in the six buckets of HistoryBucket up to now,
	// and on day, the local day of now.
	Passes(ctx context.Context, id string, now time.Time, day string) (lastHour, onDay int64, err error)
	CountPass(ctx context.Context, id string, now time.Time, day string) error
}

// clearance is what a valid clearance says.
type clearance struct {
	// The random id its passes are counted under, in hex.
	id                  string
	issuedAt, expiresAt time.Time
}

// clearanceKey derives the key clearances are signed with from clearance_secret.
func clearanceKey(secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(clearanceKeyLabel))
	return mac.Sum(nil)
}

func clearanceMAC(key []byte, encodedPayload string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(encodedPayload))
	return mac.Sum(nil)
}

// issueClearance signs a clearance, valid for ttl from now, for the browser with userAgent that
// solved a puzzle of apiKeyID.
func issueClearance(key []byte, apiKeyID, userAgent string, now time.Time, ttl time.Duration) (string, error) {
	if apiKeyID == "" || len(apiKeyID) > maxClearanceKeyID {
		return "", errors.New("the api key id does not fit in a clearance")
	}
	id := make([]byte, clearanceIDSize)
	rand.Read(id)
	digest := userAgentDigest(userAgent)
	payload := make([]byte, 0, clearanceFixedSize+len(apiKeyID))
	payload = append(payload, clearanceVersion)
	payload = append(payload, id...)
	payload = append(payload, byte(len(apiKeyID)))
	payload = append(payload, apiKeyID...)
	payload = append(payload, digest[:clearanceUASize]...)
	payload = binary.BigEndian.AppendUint64(payload, uint64(now.Unix()))
	payload = binary.BigEndian.AppendUint64(payload, uint64(now.Add(ttl).Unix()))
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(clearanceMAC(key, encoded)), nil
}

// parseClearance checks a clearance sent with an answer to a challenge of apiKeyID, from a browser
// with userAgent: its signature, compared in constant time, its expiry, its key and its browser.
// Whichever fails, it only reports false: nobody learns which part to fix. A browser update
// changes the User-Agent, which costs one more puzzle.
func parseClearance(key []byte, text, apiKeyID, userAgent string, now time.Time) (clearance, bool) {
	if len(text) > maxClearanceLength {
		return clearance{}, false
	}
	encoded, signature, found := strings.Cut(text, ".")
	if !found {
		return clearance{}, false
	}
	mac, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !hmac.Equal(mac, clearanceMAC(key, encoded)) {
		return clearance{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(payload) < clearanceFixedSize || payload[0] != clearanceVersion ||
		len(payload) != clearanceFixedSize+int(payload[1+clearanceIDSize]) {
		return clearance{}, false
	}
	id, rest := payload[1:1+clearanceIDSize], payload[1+clearanceIDSize:]
	keyID, rest := string(rest[1:1+int(rest[0])]), rest[1+int(rest[0]):]
	browser, rest := rest[:clearanceUASize], rest[clearanceUASize:]
	issuedAt, expiresAt := int64(binary.BigEndian.Uint64(rest)), int64(binary.BigEndian.Uint64(rest[8:]))
	digest := userAgentDigest(userAgent)
	if now.Unix() >= expiresAt || keyID != apiKeyID || !bytes.Equal(browser, digest[:clearanceUASize]) {
		return clearance{}, false
	}
	return clearance{id: hex.EncodeToString(id), issuedAt: time.Unix(issuedAt, 0), expiresAt: time.Unix(expiresAt, 0)}, true
}

// clearanceFor is the clearance Check hands out with a right answer to an interactive type. None
// when the answer came without the client's address or its User-Agent, which the clearance could
// not be tied to, or from a hosting address while those are blocked: auto_check would never let
// that address through, and a clearance must not carry a puzzle solved from a data centre to
// another address.
func (uc *ChallengeUsecase) clearanceFor(ctx context.Context, challenge *Challenge, answer Answer) string {
	if challenge.Type == TypeAutoCheck || !answer.Client.IsValid() || answer.UserAgent == "" ||
		(uc.auto.blockHosting && uc.hostingAddress(answer.Client)) {
		return ""
	}
	issued, err := issueClearance(uc.auto.clearanceKey, challenge.APIKeyID, answer.UserAgent, uc.now(), uc.auto.clearanceTTL)
	if err != nil {
		log.WarnContext(ctx, "no clearance issued", "err", err)
		return ""
	}
	return issued
}

// clearanceCapped reports whether the clearance id gave its clearance_passes_per_hour passes in
// the last hour or its clearance_passes_per_day today. A failure of Redis counts as over a cap:
// the clearance is all that lets its visitor skip the puzzle.
func (uc *ChallengeUsecase) clearanceCapped(ctx context.Context, id string) bool {
	perHour, perDay := uc.auto.clearancePerHour, uc.auto.clearancePerDay
	if perHour <= 0 && perDay <= 0 {
		return false
	}
	now := uc.now()
	lastHour, onDay, err := uc.clearances.Passes(ctx, id, now, quota.Day(now, uc.location))
	if err != nil {
		log.WarnContext(ctx, "passes of a clearance not read, counted as over its cap", "err", err)
		return true
	}
	return (perHour > 0 && lastHour >= perHour) || (perDay > 0 && onDay >= perDay)
}

// countClearancePass counts a pass against its clearance. A failure is logged: the pass is decided.
func (uc *ChallengeUsecase) countClearancePass(ctx context.Context, id string) {
	now := uc.now()
	if err := uc.clearances.CountPass(ctx, id, now, quota.Day(now, uc.location)); err != nil {
		log.WarnContext(ctx, "auto pass not counted for its clearance", "err", err)
	}
}

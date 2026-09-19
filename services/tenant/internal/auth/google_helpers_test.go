package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"tenant/internal/conf"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testClientID = "test-client.apps.googleusercontent.com"
	testNonce    = "nonce-1"
)

// fakeGoogle publishes signing keys like Google's JWKS endpoint and mints ID tokens with them.
type fakeGoogle struct {
	server *httptest.Server

	mu        sync.Mutex
	published map[string]*rsa.PrivateKey
	fetches   int
	failing   bool
	maxAge    int
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	f := &fakeGoogle{published: map[string]*rsa.PrivateKey{}, maxAge: 3600}
	f.server = httptest.NewServer(http.HandlerFunc(f.serveKeys))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGoogle) serveKeys(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetches++
	if f.failing {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	keys := []map[string]string{}
	for kid, key := range f.published {
		keys = append(keys, map[string]string{
			"kid": kid,
			"kty": "RSA",
			"alg": "RS256",
			"use": "sig",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		})
	}
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(f.maxAge)+", must-revalidate, no-transform")
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
}

func (f *fakeGoogle) publish(t *testing.T, kid string) *rsa.PrivateKey {
	t.Helper()
	key := newRSAKey(t)
	f.mu.Lock()
	f.published[kid] = key
	f.mu.Unlock()
	return key
}

func (f *fakeGoogle) setFailing(failing bool) {
	f.mu.Lock()
	f.failing = failing
	f.mu.Unlock()
}

func (f *fakeGoogle) fetchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fetches
}

func newRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

// testClock is a settable time source shared by the verifier and its key cache.
type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time          { return c.now }
func (c *testClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func tokenClaims(now time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":            "https://accounts.google.com",
		"aud":            testClientID,
		"azp":            testClientID,
		"sub":            "110169484474386276334",
		"email":          "Ann.Lee@gmail.com",
		"email_verified": true,
		"name":           " Ann Lee ",
		"nonce":          testNonce,
		"iat":            now.Unix(),
		"exp":            now.Add(time.Hour).Unix(),
	}
}

func signRS256(t *testing.T, kid string, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// newTestVerifier builds a verifier backed by f whose clock the test controls.
func newTestVerifier(t *testing.T, f *fakeGoogle) (*Google, *testClock) {
	t.Helper()
	g, err := NewGoogle(&conf.Google{
		ClientId: testClientID,
		JwksUrl:  f.server.URL,
		Issuers:  []string{"accounts.google.com", "https://accounts.google.com"},
	})
	if err != nil {
		t.Fatalf("NewGoogle: %v", err)
	}
	clock := &testClock{now: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)}
	g.now = clock.Now
	g.keys.now = clock.Now
	return g, clock
}

package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const (
	// Refreshes, including those triggered by unknown key ids, happen at most this often,
	// so forged tokens cannot turn the service into a flood of requests to Google.
	minKeyRefresh = time.Minute
	// A failed refresh is retried sooner, so a brief outage does not block sign-in for long.
	failedKeyRefresh = 5 * time.Second
	// Used when the key response carries no usable Cache-Control max-age.
	defaultKeyTTL = time.Hour
	fetchTimeout  = 5 * time.Second
	minRSABits    = 2048
)

var maxAgePattern = regexp.MustCompile(`max-age=(\d+)`)

// keySet caches the RSA signing keys published at a JWKS URL.
type keySet struct {
	url    string
	client *http.Client
	now    func() time.Time

	mu          sync.Mutex
	keys        map[string]*rsa.PublicKey
	expires     time.Time
	nextRefresh time.Time
}

func newKeySet(url string) *keySet {
	return &keySet{url: url, client: &http.Client{Timeout: fetchTimeout}, now: time.Now}
}

// key returns the public key with this id, refreshing the set when it expired or lacks the id.
func (s *keySet) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	key, known := s.keys[kid]
	if known && now.Before(s.expires) {
		return key, nil
	}
	if !now.Before(s.nextRefresh) {
		err := s.refresh(ctx, now)
		switch {
		case err == nil:
			s.nextRefresh = now.Add(minKeyRefresh)
			key, known = s.keys[kid]
		case !known:
			s.nextRefresh = now.Add(failedKeyRefresh)
			return nil, err
		default:
			// A known key keeps working until Google is reachable again.
			s.nextRefresh = now.Add(failedKeyRefresh)
		}
	}
	if !known {
		return nil, fmt.Errorf("unknown signing key %q", kid)
	}
	return key, nil
}

func (s *keySet) refresh(ctx context.Context, now time.Time) error {
	// The keys serve every request, so a caller giving up must not abort the fetch.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch signing keys: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch signing keys: %s", resp.Status)
	}

	var body struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return fmt.Errorf("decode signing keys: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(body.Keys))
	for _, k := range body.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		if key, err := parseRSAKey(k.N, k.E); err == nil {
			keys[k.Kid] = key
		}
	}
	if len(keys) == 0 {
		return errors.New("no usable signing keys")
	}
	s.keys = keys
	s.expires = now.Add(cacheTTL(resp.Header.Get("Cache-Control")))
	return nil
}

func parseRSAKey(modulus, exponent string) (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(modulus)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(exponent)
	if err != nil || len(e) == 0 || len(e) > 4 {
		return nil, errors.New("invalid rsa exponent")
	}
	key := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	if key.N.BitLen() < minRSABits {
		return nil, errors.New("rsa key too small")
	}
	return key, nil
}

// cacheTTL reads max-age from a Cache-Control header, as Google asks clients to do.
func cacheTTL(header string) time.Duration {
	match := maxAgePattern.FindStringSubmatch(header)
	if match == nil {
		return defaultKeyTTL
	}
	seconds, err := strconv.Atoi(match[1])
	if err != nil {
		return defaultKeyTTL
	}
	return time.Duration(seconds) * time.Second
}

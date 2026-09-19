package auth

import (
	"context"
	"testing"
	"time"
)

func TestKeysAreCachedUntilTheyExpire(t *testing.T) {
	f := newFakeGoogle(t)
	f.maxAge = 600
	key := f.publish(t, "k1")
	g, clock := newTestVerifier(t, f)
	verify := func() {
		t.Helper()
		if _, err := g.Verify(context.Background(), signRS256(t, "k1", key, tokenClaims(clock.now)), testNonce); err != nil {
			t.Fatalf("Verify: %v", err)
		}
	}

	verify()
	verify()
	if got := f.fetchCount(); got != 1 {
		t.Fatalf("fetches = %d, want 1 while cached", got)
	}
	clock.Advance(601 * time.Second)
	verify()
	if got := f.fetchCount(); got != 2 {
		t.Fatalf("fetches = %d, want 2 after max-age", got)
	}
}

func TestUnknownKeyRefreshIsRateLimited(t *testing.T) {
	f := newFakeGoogle(t)
	f.publish(t, "k1")
	g, clock := newTestVerifier(t, f)
	unpublished := newRSAKey(t)
	token := func() string { return signRS256(t, "k2", unpublished, tokenClaims(clock.now)) }

	for range 3 {
		if _, err := g.Verify(context.Background(), token(), testNonce); err == nil {
			t.Fatal("unknown key accepted")
		}
	}
	if got := f.fetchCount(); got != 1 {
		t.Fatalf("fetches = %d, want 1 within a minute", got)
	}

	clock.Advance(61 * time.Second)
	if _, err := g.Verify(context.Background(), token(), testNonce); err == nil {
		t.Fatal("unknown key accepted after refresh")
	}
	if got := f.fetchCount(); got != 2 {
		t.Fatalf("fetches = %d, want 2 after a minute", got)
	}
}

func TestRotatedKeyIsPickedUp(t *testing.T) {
	f := newFakeGoogle(t)
	oldKey := f.publish(t, "k1")
	g, clock := newTestVerifier(t, f)
	if _, err := g.Verify(context.Background(), signRS256(t, "k1", oldKey, tokenClaims(clock.now)), testNonce); err != nil {
		t.Fatalf("Verify with first key: %v", err)
	}

	newKey := f.publish(t, "k2")
	clock.Advance(2 * time.Minute)
	if _, err := g.Verify(context.Background(), signRS256(t, "k2", newKey, tokenClaims(clock.now)), testNonce); err != nil {
		t.Fatalf("Verify with rotated key: %v", err)
	}
}

func TestKnownKeyServedWhileGoogleIsDown(t *testing.T) {
	f := newFakeGoogle(t)
	f.maxAge = 60
	key := f.publish(t, "k1")
	g, clock := newTestVerifier(t, f)
	if _, err := g.Verify(context.Background(), signRS256(t, "k1", key, tokenClaims(clock.now)), testNonce); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	f.setFailing(true)
	clock.Advance(5 * time.Minute)
	if _, err := g.Verify(context.Background(), signRS256(t, "k1", key, tokenClaims(clock.now)), testNonce); err != nil {
		t.Fatalf("Verify with stale keys: %v", err)
	}
}

func TestFailedRefreshIsRetriedSoon(t *testing.T) {
	f := newFakeGoogle(t)
	key := f.publish(t, "k1")
	g, clock := newTestVerifier(t, f)
	token := func() string { return signRS256(t, "k1", key, tokenClaims(clock.now)) }

	f.setFailing(true)
	if _, err := g.Verify(context.Background(), token(), testNonce); err == nil {
		t.Fatal("token accepted without keys")
	}
	f.setFailing(false)
	if _, err := g.Verify(context.Background(), token(), testNonce); err == nil {
		t.Fatal("refresh retried before the failure delay")
	}
	clock.Advance(failedKeyRefresh)
	if _, err := g.Verify(context.Background(), token(), testNonce); err != nil {
		t.Fatalf("Verify after the failure delay: %v", err)
	}
	if got := f.fetchCount(); got != 2 {
		t.Fatalf("fetches = %d, want 2", got)
	}
}

func TestCanceledCallerDoesNotAbortFetch(t *testing.T) {
	f := newFakeGoogle(t)
	key := f.publish(t, "k1")
	g, clock := newTestVerifier(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.Verify(ctx, signRS256(t, "k1", key, tokenClaims(clock.now)), testNonce); err != nil {
		t.Fatalf("Verify with canceled context: %v", err)
	}
}

func TestCacheTTL(t *testing.T) {
	tests := map[string]time.Duration{
		"public, max-age=19612, must-revalidate, no-transform": 19612 * time.Second,
		"max-age=0":      0,
		"no-cache":       defaultKeyTTL,
		"":               defaultKeyTTL,
		"max-age=banana": defaultKeyTTL,
	}
	for header, want := range tests {
		if got := cacheTTL(header); got != want {
			t.Errorf("cacheTTL(%q) = %v, want %v", header, got, want)
		}
	}
}

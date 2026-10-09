package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeCounter is Redis in memory; down makes every call fail.
type fakeCounter struct {
	counts  map[string]int64
	expires map[string]time.Duration
	down    bool
}

func newFakeCounter() *fakeCounter {
	return &fakeCounter{counts: map[string]int64{}, expires: map[string]time.Duration{}}
}

func (c *fakeCounter) Incr(_ context.Context, key string) (int64, error) {
	if c.down {
		return 0, errors.New("dial tcp 10.0.0.1:6379: connection refused")
	}
	c.counts[key]++
	return c.counts[key], nil
}

func (c *fakeCounter) Expire(_ context.Context, key string, ttl time.Duration) error {
	c.expires[key] = ttl
	return nil
}

func TestAllow(t *testing.T) {
	start := time.Date(2026, 10, 8, 10, 0, 15, 0, time.UTC)
	perMinute := Rule{Scope: "new_ip", Limit: 3, Window: time.Minute}
	other := Rule{Scope: "verify_ip", Limit: 1, Window: time.Minute}

	tests := []struct {
		name string
		run  func(t *testing.T, limiter *Limiter, counter *fakeCounter, clock *time.Time)
	}{
		{"within the limit, then refused until the window ends", func(t *testing.T, limiter *Limiter, _ *fakeCounter, _ *time.Time) {
			for i := range 3 {
				if wait := limiter.Allow(context.Background(), perMinute, "203.0.113.4"); wait != 0 {
					t.Fatalf("call %d: refused for %v", i+1, wait)
				}
			}
			if wait := limiter.Allow(context.Background(), perMinute, "203.0.113.4"); wait != 45*time.Second {
				t.Fatalf("4th call: wait %v, want 45s", wait)
			}
		}},
		{"a new window starts over", func(t *testing.T, limiter *Limiter, _ *fakeCounter, clock *time.Time) {
			for range 4 {
				limiter.Allow(context.Background(), perMinute, "203.0.113.4")
			}
			*clock = clock.Add(45 * time.Second)
			if wait := limiter.Allow(context.Background(), perMinute, "203.0.113.4"); wait != 0 {
				t.Fatalf("first call of the next window refused for %v", wait)
			}
		}},
		{"scopes and subjects are counted apart", func(t *testing.T, limiter *Limiter, _ *fakeCounter, _ *time.Time) {
			for range 3 {
				limiter.Allow(context.Background(), perMinute, "203.0.113.4")
			}
			if wait := limiter.Allow(context.Background(), other, "203.0.113.4"); wait != 0 {
				t.Fatalf("another scope shares the count: refused for %v", wait)
			}
			if wait := limiter.Allow(context.Background(), perMinute, "198.51.100.7"); wait != 0 {
				t.Fatalf("another subject shares the count: refused for %v", wait)
			}
		}},
		{"only the first call of a window sets the expiry", func(t *testing.T, limiter *Limiter, counter *fakeCounter, _ *time.Time) {
			limiter.Allow(context.Background(), perMinute, "203.0.113.4")
			counter.expires = map[string]time.Duration{}
			limiter.Allow(context.Background(), perMinute, "203.0.113.4")
			if len(counter.expires) != 0 {
				t.Fatalf("second call set an expiry: %v", counter.expires)
			}
			const key = "wau:rl:{new_ip:203.0.113.4}:29857560"
			if counter.counts[key] != 2 {
				t.Fatalf("counter %s = %d, keys %v", key, counter.counts[key], counter.counts)
			}
		}},
		{"redis down lets every call through", func(t *testing.T, limiter *Limiter, counter *fakeCounter, _ *time.Time) {
			counter.down = true
			for range 10 {
				if wait := limiter.Allow(context.Background(), other, "203.0.113.4"); wait != 0 {
					t.Fatalf("refused for %v while redis is down", wait)
				}
			}
		}},
		{"a rule that is off counts nothing", func(t *testing.T, limiter *Limiter, counter *fakeCounter, _ *time.Time) {
			off := Rule{Scope: "new_key", Limit: 0, Window: time.Minute}
			for range 5 {
				if wait := limiter.Allow(context.Background(), off, "key"); wait != 0 {
					t.Fatalf("off rule refused for %v", wait)
				}
			}
			if wait := limiter.Allow(context.Background(), perMinute, ""); wait != 0 || len(counter.counts) != 0 {
				t.Fatalf("empty subject counted: wait %v, keys %v", wait, counter.counts)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clock := start
			counter := newFakeCounter()
			limiter := &Limiter{counter: counter, now: func() time.Time { return clock }}
			test.run(t, limiter, counter, &clock)
		})
	}
}

func TestHash(t *testing.T) {
	hashed := Hash("sk_4a43813384268b62be376cf7b81eb47ca9619c23e2b4c198")
	if len(hashed) != 64 || hashed == Hash("sk_other") {
		t.Fatalf("Hash = %q", hashed)
	}
}

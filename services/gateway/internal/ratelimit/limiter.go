package ratelimit

import (
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"gateway/internal/httpx"

	"common/clientip"
)

// Limiter counts requests per key in fixed windows.
type Limiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	counters  map[string]*counter
	lastSweep time.Time
}

type counter struct {
	start time.Time
	hits  int
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, counters: map[string]*counter{}}
}

// Allow records a hit and reports whether it fits in the current window, plus
// how long the caller should wait when it does not.
func (l *Limiter) Allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)

	current, ok := l.counters[key]
	if !ok || now.Sub(current.start) >= l.window {
		l.counters[key] = &counter{start: now, hits: 1}
		return true, 0
	}
	if current.hits >= l.limit {
		return false, l.window - now.Sub(current.start)
	}
	current.hits++
	return true, 0
}

// sweep drops finished windows so the map cannot grow without bound. It runs at
// most once per window and the caller holds the lock.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for key, current := range l.counters {
		if now.Sub(current.start) >= l.window {
			delete(l.counters, key)
		}
	}
}

// Middleware limits the given path prefixes and passes everything else through.
func Middleware(limiter *Limiter, prefixes []string, trusted []netip.Prefix, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLimitedPath(r.URL.Path, prefixes) {
			next.ServeHTTP(w, r)
			return
		}
		if ok, retryAfter := limiter.Allow(clientip.Key(r, trusted), time.Now()); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			httpx.WriteError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests, try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLimitedPath(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

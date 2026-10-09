// Package ratelimit counts calls in fixed windows in Redis: one counter per scope, subject and
// window, wau:rl:{<scope>:<subject>}:<window>. Each call touches that one key, so the limiter
// works on a cluster. challenge and verify each keep a copy of this package: keep the copies
// identical.
package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/redis/go-redis/v9"
)

// Rule is one limit: at most Limit calls per Window for each subject.
type Rule struct {
	// Scope keeps the counters of different rules apart, e.g. "new_ip" and "new_key".
	Scope string
	// Limit of 0 or less turns the rule off.
	Limit  int64
	Window time.Duration
}

// counter is the part of Redis the limiter uses; tests replace it.
type counter interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

type redisCounter struct{ client redis.Cmdable }

func (c redisCounter) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

func (c redisCounter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.client.Expire(ctx, key, ttl).Err()
}

// Limiter counts calls against rules.
type Limiter struct {
	counter counter
	now     func() time.Time
}

// New counts in the Redis behind client.
func New(client redis.Cmdable) *Limiter {
	return &Limiter{counter: redisCounter{client: client}, now: time.Now}
}

// Allow counts one call of subject under rule and returns 0 while the subject is within the
// limit, or else how long until its window resets. A rule that is off, or an empty subject, is
// never counted. When Redis fails the call is let through and the failure logged: a blip of the
// store must not take the API down with it.
func (l *Limiter) Allow(ctx context.Context, rule Rule, subject string) time.Duration {
	if rule.Limit <= 0 || rule.Window <= 0 || subject == "" {
		return 0
	}
	now := l.now()
	window := now.UnixNano() / int64(rule.Window)
	key := fmt.Sprintf("wau:rl:{%s:%s}:%d", rule.Scope, subject, window)
	count, err := l.counter.Incr(ctx, key)
	if err != nil {
		log.WarnContext(ctx, "rate limit not counted, call let through", "scope", rule.Scope, "err", err)
		return 0
	}
	// Only the call that created the counter sets its expiry, which drops it once its window is over.
	if count == 1 {
		if err := l.counter.Expire(ctx, key, rule.Window); err != nil {
			log.WarnContext(ctx, "rate limit counter has no expiry", "scope", rule.Scope, "err", err)
		}
	}
	if count <= rule.Limit {
		return 0
	}
	return time.Unix(0, (window+1)*int64(rule.Window)).Sub(now)
}

// Hash stands for a subject that must not appear in a key, such as a secret key, or whose length
// the caller does not control.
func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

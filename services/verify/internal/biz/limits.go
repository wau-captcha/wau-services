package biz

import (
	"context"
	"math"
	"strconv"
	"time"

	"verify/internal/conf"
	"verify/internal/pkg/ratelimit"

	"github.com/google/uuid"
)

// RateLimiter counts calls in fixed windows; internal/pkg/ratelimit does it in Redis.
type RateLimiter interface {
	// Allow returns 0 while subject is within rule, or else how long until its window resets.
	Allow(ctx context.Context, rule ratelimit.Rule, subject string) time.Duration
}

// limits are the rate limits of conf.RateLimit, per minute.
type limits struct {
	verifyPerIP, siteVerifyPerKey ratelimit.Rule
}

func newLimits(rateConf *conf.RateLimit) limits {
	return limits{
		verifyPerIP:      ratelimit.Rule{Scope: "verify_ip", Limit: int64(rateConf.GetVerifyPerIp()), Window: time.Minute},
		siteVerifyPerKey: ratelimit.Rule{Scope: "siteverify_key", Limit: int64(rateConf.GetSiteverifyPerSecretKey()), Window: time.Minute},
	}
}

// rateLimited is ErrRateLimited with the whole seconds until the window resets, which the HTTP
// server also sends as Retry-After.
func rateLimited(wait time.Duration) error {
	seconds := max(int(math.Ceil(wait.Seconds())), 1)
	return ErrRateLimited.WithMetadata(map[string]string{"retry_after": strconv.Itoa(seconds)})
}

// validChallengeID reports whether id is a challenge id as the challenge service issues them: a
// UUID in its canonical form, 36 characters, lowercase.
func validChallengeID(id string) bool {
	if len(id) != 36 {
		return false
	}
	parsed, err := uuid.Parse(id)
	return err == nil && parsed.String() == id
}

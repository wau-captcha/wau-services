package biz

import (
	"context"
	"math"
	"strconv"
	"time"

	"challenge/internal/conf"
	"challenge/internal/pkg/ratelimit"

	"github.com/google/uuid"
)

// RateLimiter counts calls in fixed windows; internal/pkg/ratelimit does it in Redis.
type RateLimiter interface {
	// Allow returns 0 while subject is within rule, or else how long until its window resets.
	Allow(ctx context.Context, rule ratelimit.Rule, subject string) time.Duration
}

// limits are the rate limits of conf.RateLimit, per minute.
type limits struct {
	newPerIP, newPerKey, newPerFlaggedKey ratelimit.Rule
}

func newLimits(rateConf *conf.RateLimit) limits {
	return limits{
		newPerIP:  ratelimit.Rule{Scope: "new_ip", Limit: int64(rateConf.GetNewPerIp()), Window: time.Minute},
		newPerKey: ratelimit.Rule{Scope: "new_key", Limit: int64(rateConf.GetNewPerSiteKey()), Window: time.Minute},
		// The same counter as newPerKey, held to a lower limit.
		newPerFlaggedKey: ratelimit.Rule{Scope: "new_key", Limit: int64(rateConf.GetNewPerFlaggedSiteKey()), Window: time.Minute},
	}
}

// perKey is the per-key limit of a key, lower while the scraping check has it flagged.
func (l limits) perKey(flagged bool) ratelimit.Rule {
	if flagged {
		return l.newPerFlaggedKey
	}
	return l.newPerKey
}

// rateLimited is ErrRateLimited with the whole seconds until the window resets, which the HTTP
// server also sends as Retry-After.
func rateLimited(wait time.Duration) error {
	seconds := max(int(math.Ceil(wait.Seconds())), 1)
	return ErrRateLimited.WithMetadata(map[string]string{"retry_after": strconv.Itoa(seconds)})
}

// validChallengeID reports whether id is a challenge id as New issues them: a UUID in its
// canonical form, 36 characters, lowercase. Anything else is refused before it reaches Redis,
// where a long id makes a key too large to look up.
func validChallengeID(id string) bool {
	if len(id) != 36 {
		return false
	}
	parsed, err := uuid.Parse(id)
	return err == nil && parsed.String() == id
}

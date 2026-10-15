package biz

import (
	"context"
	"fmt"
	"net/http"
	"time"

	v1 "verify/api/verify/v1"
	"verify/internal/conf"
	"verify/internal/pkg/ratelimit"

	"github.com/go-kratos/kratos/v3/errors"
)

var (
	// ErrSecretKeyInvalid is returned when a secret key is unknown, inactive or deleted.
	ErrSecretKeyInvalid   = errors.Unauthorized(v1.ErrorReason_SECRET_KEY_INVALID.String(), "secret key is invalid")
	ErrApiKeySuspended    = errors.Forbidden(v1.ErrorReason_API_KEY_SUSPENDED.String(), "api key is suspended")
	ErrAccountBanned      = errors.Forbidden(v1.ErrorReason_ACCOUNT_BANNED.String(), "the owner of this api key is banned")
	ErrChallengeIDInvalid = errors.BadRequest(v1.ErrorReason_CHALLENGE_ID_INVALID.String(), "challenge_id is not a valid challenge id")
	ErrRateLimited        = errors.New(http.StatusTooManyRequests, v1.ErrorReason_RATE_LIMITED.String(), "too many requests")
	ErrPayloadTooLarge    = errors.New(http.StatusRequestEntityTooLarge, v1.ErrorReason_PAYLOAD_TOO_LARGE.String(), "request body too large")
	ErrBehaviorRequired   = errors.BadRequest(v1.ErrorReason_BEHAVIOR_REQUIRED.String(),
		"this site key requires behaviour data, which the client did not send: update @wau-captcha/client to the latest version")
	// ErrInternal stands in for any failure of Redis, Postgres, the challenge service or the code:
	// their text stays in the log.
	ErrInternal = errors.InternalServer(v1.ErrorReason_INTERNAL.String(), "internal error")
)

// maxTokenLength bounds what SiteVerify reads as a token. Tokens are 76 characters
// (base64url of 24 bytes, a dot, base64url of 32); a longer one cannot be ours.
const maxTokenLength = 512

// Site is the active API key behind a secret key.
type Site struct {
	APIKeyID    string
	Suspended   bool
	OwnerBanned bool
}

// Grant is what a token proves: a challenge solved for a site, and how human the person who
// solved it behaved.
type Grant struct {
	APIKeyID      string
	ChallengeType string
	SolvedAt      time.Time
	Behavior      BehaviorResult
	// The host name of the website the challenge was asked for from; empty when unknown.
	Hostname string
}

// Token is a signed one-time token.
type Token struct {
	Value string
	TTL   time.Duration
	// The challenge service's clearance for the browser, relayed to it; empty after auto_check.
	Clearance string
}

// GrantRepo stores the grants behind issued tokens.
type GrantRepo interface {
	Save(ctx context.Context, id string, grant *Grant, ttl time.Duration) error
	// Take loads and deletes a grant atomically; it returns nil when absent.
	Take(ctx context.Context, id string) (*Grant, error)
}

// SiteRepo resolves secret keys and records usage.
type SiteRepo interface {
	FindBySecretKey(ctx context.Context, secretKey string) (*Site, error)
	// RecordAnswer writes the outcome of a checked answer into the usage history of its challenge.
	RecordAnswer(ctx context.Context, challengeID string, solution *Solution) error
}

// VerifyUsecase issues and validates tokens.
type VerifyUsecase struct {
	challenges ChallengeClient
	grants     GrantRepo
	sites      SiteRepo
	counters   CounterRepo
	limiter    RateLimiter
	limits     limits
	secret     []byte
	ttl        time.Duration
	location   *time.Location
	now        func() time.Time
}

// NewVerifyUsecase creates a VerifyUsecase.
func NewVerifyUsecase(challenges ChallengeClient, grants GrantRepo, sites SiteRepo, counters CounterRepo, limiter RateLimiter,
	tokenConf *conf.Token, quotaConf *conf.Quota, rateConf *conf.RateLimit) (*VerifyUsecase, error) {
	if tokenConf.GetSecret() == "" || tokenConf.GetTtl().AsDuration() <= 0 {
		return nil, fmt.Errorf("token secret and positive ttl are required")
	}
	if quotaConf.GetTimeZone() == "" {
		return nil, fmt.Errorf("quota time_zone is required")
	}
	location, err := time.LoadLocation(quotaConf.GetTimeZone())
	if err != nil {
		return nil, err
	}
	return &VerifyUsecase{
		challenges: challenges,
		grants:     grants,
		sites:      sites,
		counters:   counters,
		limiter:    limiter,
		limits:     newLimits(rateConf),
		secret:     []byte(tokenConf.GetSecret()),
		ttl:        tokenConf.GetTtl().AsDuration(),
		location:   location,
		now:        time.Now,
	}, nil
}

// VerifyChallenge checks an answer with the challenge service and returns a token, or nil when
// the answer is wrong or was refused. A challenge_id that cannot be one is refused before it
// reaches the challenge service, and the per-address limit before anything else is done. An
// answer without behaviour to a key that requires it is counted like any other, then refused
// with ErrBehaviorRequired, so the integrator learns why.
func (uc *VerifyUsecase) VerifyChallenge(ctx context.Context, answer Answer) (*Token, error) {
	if !validChallengeID(answer.ChallengeID) {
		return nil, ErrChallengeIDInvalid
	}
	if answer.Client.IsValid() {
		if wait := uc.limiter.Allow(ctx, uc.limits.verifyPerIP, answer.Client.String()); wait > 0 {
			return nil, rateLimited(wait)
		}
	}
	solution, err := uc.challenges.CheckAnswer(ctx, boundAnswer(answer))
	if err != nil {
		return nil, err
	}
	uc.recordAnswer(ctx, answer.ChallengeID, solution)
	uc.recordCounters(ctx, solution)
	if solution.BehaviorRequired {
		return nil, ErrBehaviorRequired
	}
	if !solution.Success {
		return nil, nil
	}
	now := uc.now()
	id, token := newToken(uc.secret, now.Add(uc.ttl))
	grant := &Grant{APIKeyID: solution.APIKeyID, ChallengeType: solution.ChallengeType, SolvedAt: now, Behavior: solution.Behavior,
		Hostname: solution.Hostname}
	if err := uc.grants.Save(ctx, id, grant, uc.ttl); err != nil {
		return nil, err
	}
	return &Token{Value: token, TTL: uc.ttl, Clearance: solution.Clearance}, nil
}

// SiteVerify consumes a token for the site owning secretKey. It returns nil
// when the token is invalid, expired, already used or issued for another site.
// A suspended key, or a key whose owner is banned, cannot redeem tokens.
func (uc *VerifyUsecase) SiteVerify(ctx context.Context, secretKey, token string) (*Grant, error) {
	// The secret key is counted by its hash: a secret never appears in a Redis key.
	if wait := uc.limiter.Allow(ctx, uc.limits.siteVerifyPerKey, ratelimit.Hash(secretKey)); wait > 0 {
		return nil, rateLimited(wait)
	}
	site, err := uc.sites.FindBySecretKey(ctx, secretKey)
	if err != nil {
		return nil, err
	}
	switch {
	case site.OwnerBanned:
		return nil, ErrAccountBanned
	case site.Suspended:
		return nil, ErrApiKeySuspended
	}
	if len(token) > maxTokenLength {
		return nil, nil
	}
	id, ok := parseToken(uc.secret, token, uc.now())
	if !ok {
		return nil, nil
	}
	grant, err := uc.grants.Take(ctx, id)
	if err != nil || grant == nil || grant.APIKeyID != site.APIKeyID {
		return nil, err
	}
	return grant, nil
}

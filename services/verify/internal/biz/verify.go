package biz

import (
	"context"
	"fmt"
	"time"

	"verify/internal/conf"

	v1 "common/proto/verify/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
)

var (
	// ErrSecretKeyInvalid is returned when a secret key is unknown, inactive or deleted.
	ErrSecretKeyInvalid = errors.Unauthorized(v1.ErrorReason_SECRET_KEY_INVALID.String(), "secret key is invalid")
	ErrApiKeySuspended  = errors.Forbidden(v1.ErrorReason_API_KEY_SUSPENDED.String(), "api key is suspended")
	ErrAccountBanned    = errors.Forbidden(v1.ErrorReason_ACCOUNT_BANNED.String(), "the owner of this api key is banned")
)

// Solution is a checked challenge answer.
type Solution struct {
	Success       bool
	APIKeyID      string
	ChallengeType string
	// Owner of the API key; empty for challenges issued before owners were recorded.
	UserID string
}

// Site is the active API key behind a secret key.
type Site struct {
	APIKeyID    string
	Suspended   bool
	OwnerBanned bool
}

// Grant is what a token proves: a challenge solved for a site.
type Grant struct {
	APIKeyID      string
	ChallengeType string
	SolvedAt      time.Time
}

// Token is a signed one-time token.
type Token struct {
	Value string
	TTL   time.Duration
}

// ChallengeClient checks answers with the challenge service.
type ChallengeClient interface {
	CheckAnswer(ctx context.Context, challengeID, answer string) (*Solution, error)
}

// GrantRepo stores the grants behind issued tokens.
type GrantRepo interface {
	Save(ctx context.Context, id string, g *Grant, ttl time.Duration) error
	// Take loads and deletes a grant atomically; it returns nil when absent.
	Take(ctx context.Context, id string) (*Grant, error)
}

// SiteRepo resolves secret keys and records usage.
type SiteRepo interface {
	FindBySecretKey(ctx context.Context, secretKey string) (*Site, error)
	RecordUsage(ctx context.Context, apiKeyID, challengeType string, success bool) error
}

// VerifyUsecase issues and validates tokens.
type VerifyUsecase struct {
	challenges ChallengeClient
	grants     GrantRepo
	sites      SiteRepo
	counters   CounterRepo
	secret     []byte
	ttl        time.Duration
	location   *time.Location
	now        func() time.Time
}

// NewVerifyUsecase creates a VerifyUsecase.
func NewVerifyUsecase(challenges ChallengeClient, grants GrantRepo, sites SiteRepo, counters CounterRepo,
	c *conf.Token, q *conf.Quota) (*VerifyUsecase, error) {
	if c.GetSecret() == "" || c.GetTtl().AsDuration() <= 0 {
		return nil, fmt.Errorf("token secret and positive ttl are required")
	}
	if q.GetTimeZone() == "" {
		return nil, fmt.Errorf("quota time_zone is required")
	}
	location, err := time.LoadLocation(q.GetTimeZone())
	if err != nil {
		return nil, err
	}
	return &VerifyUsecase{
		challenges: challenges,
		grants:     grants,
		sites:      sites,
		counters:   counters,
		secret:     []byte(c.GetSecret()),
		ttl:        c.GetTtl().AsDuration(),
		location:   location,
		now:        time.Now,
	}, nil
}

// VerifyChallenge checks an answer and returns a token, or nil when the answer is wrong.
func (uc *VerifyUsecase) VerifyChallenge(ctx context.Context, challengeID, answer string) (*Token, error) {
	sol, err := uc.challenges.CheckAnswer(ctx, challengeID, answer)
	if err != nil {
		return nil, err
	}
	if err := uc.sites.RecordUsage(ctx, sol.APIKeyID, sol.ChallengeType, sol.Success); err != nil {
		log.ErrorContext(ctx, "failed recording usage", "err", err)
	}
	uc.count(ctx, sol)
	if !sol.Success {
		return nil, nil
	}
	now := uc.now()
	id, token := newToken(uc.secret, now.Add(uc.ttl))
	g := &Grant{APIKeyID: sol.APIKeyID, ChallengeType: sol.ChallengeType, SolvedAt: now}
	if err := uc.grants.Save(ctx, id, g, uc.ttl); err != nil {
		return nil, err
	}
	return &Token{Value: token, TTL: uc.ttl}, nil
}

// SiteVerify consumes a token for the site owning secretKey. It returns nil
// when the token is invalid, expired, already used or issued for another site.
// A suspended key, or a key whose owner is banned, cannot redeem tokens.
func (uc *VerifyUsecase) SiteVerify(ctx context.Context, secretKey, token string) (*Grant, error) {
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
	id, ok := parseToken(uc.secret, token, uc.now())
	if !ok {
		return nil, nil
	}
	g, err := uc.grants.Take(ctx, id)
	if err != nil || g == nil || g.APIKeyID != site.APIKeyID {
		return nil, err
	}
	return g, nil
}

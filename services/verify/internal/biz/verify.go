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

// ErrSecretKeyInvalid is returned when a secret key is unknown, inactive or deleted.
var ErrSecretKeyInvalid = errors.Unauthorized(v1.ErrorReason_SECRET_KEY_INVALID.String(), "secret key is invalid")

// Solution is a checked challenge answer.
type Solution struct {
	Success       bool
	APIKeyID      string
	ChallengeType string
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
	FindAPIKeyID(ctx context.Context, secretKey string) (string, error)
	RecordUsage(ctx context.Context, apiKeyID, challengeType string, success bool) error
}

// VerifyUsecase issues and validates tokens.
type VerifyUsecase struct {
	challenges ChallengeClient
	grants     GrantRepo
	sites      SiteRepo
	secret     []byte
	ttl        time.Duration
}

// NewVerifyUsecase creates a VerifyUsecase.
func NewVerifyUsecase(challenges ChallengeClient, grants GrantRepo, sites SiteRepo, c *conf.Token) (*VerifyUsecase, error) {
	if c.GetSecret() == "" || c.GetTtl().AsDuration() <= 0 {
		return nil, fmt.Errorf("token secret and positive ttl are required")
	}
	return &VerifyUsecase{
		challenges: challenges,
		grants:     grants,
		sites:      sites,
		secret:     []byte(c.GetSecret()),
		ttl:        c.GetTtl().AsDuration(),
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
	if !sol.Success {
		return nil, nil
	}
	now := time.Now()
	id, token := newToken(uc.secret, now.Add(uc.ttl))
	g := &Grant{APIKeyID: sol.APIKeyID, ChallengeType: sol.ChallengeType, SolvedAt: now}
	if err := uc.grants.Save(ctx, id, g, uc.ttl); err != nil {
		return nil, err
	}
	return &Token{Value: token, TTL: uc.ttl}, nil
}

// SiteVerify consumes a token for the site owning secretKey. It returns nil
// when the token is invalid, expired, already used or issued for another site.
func (uc *VerifyUsecase) SiteVerify(ctx context.Context, secretKey, token string) (*Grant, error) {
	apiKeyID, err := uc.sites.FindAPIKeyID(ctx, secretKey)
	if err != nil {
		return nil, err
	}
	id, ok := parseToken(uc.secret, token, time.Now())
	if !ok {
		return nil, nil
	}
	g, err := uc.grants.Take(ctx, id)
	if err != nil || g == nil || g.APIKeyID != apiKeyID {
		return nil, err
	}
	return g, nil
}

package biz

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"challenge/internal/conf"

	v1 "common/proto/challenge/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

const (
	TypeImageDistort = "image_distort"
	TypeMotionNoise  = "motion_noise"
)

// renderers draws a code as the media of each supported challenge type.
var renderers = map[string]func(code string) (Media, error){
	TypeImageDistort: renderImageDistort,
	TypeMotionNoise:  renderMotionNoise,
}

var (
	ErrSiteKeyInvalid           = errors.Unauthorized(v1.ErrorReason_SITE_KEY_INVALID.String(), "site key is invalid")
	ErrChallengeTypeUnsupported = errors.BadRequest(v1.ErrorReason_CHALLENGE_TYPE_UNSUPPORTED.String(), "challenge type is not supported")
	ErrChallengeTypeNotAllowed  = errors.Forbidden(v1.ErrorReason_CHALLENGE_TYPE_NOT_ALLOWED.String(), "challenge type is not allowed")
	ErrChallengeNotFound        = errors.NotFound(v1.ErrorReason_CHALLENGE_NOT_FOUND.String(), "challenge not found")
)

type Site struct {
	APIKeyID       string
	ChallengeTypes []string
}

type Media struct {
	Data     []byte
	MIMEType string
}

type Challenge struct {
	ID       string
	Type     string
	APIKeyID string
	Answer   string
	Media    Media
	TTL      time.Duration
}

type SiteRepo interface {
	FindBySiteKey(ctx context.Context, siteKey string) (*Site, error)
}
type ChallengeRepo interface {
	Save(ctx context.Context, c *Challenge) error
	Take(ctx context.Context, id string) (*Challenge, error)
}

type ChallengeUsecase struct {
	sites      SiteRepo
	challenges ChallengeRepo
	ttl        time.Duration
}

func NewChallengeUsecase(sites SiteRepo, challenges ChallengeRepo, c *conf.Challenge) (*ChallengeUsecase, error) {
	ttl := c.GetTtl().AsDuration()
	if ttl <= 0 {
		return nil, fmt.Errorf("challenge ttl must be positive")
	}
	return &ChallengeUsecase{sites: sites, challenges: challenges, ttl: ttl}, nil
}

// New issues a challenge of challengeType, image_distort when empty.
func (uc *ChallengeUsecase) New(ctx context.Context, siteKey, challengeType string) (*Challenge, error) {
	if challengeType == "" {
		challengeType = TypeImageDistort
	}
	render, ok := renderers[challengeType]
	if !ok {
		return nil, ErrChallengeTypeUnsupported
	}
	site, err := uc.sites.FindBySiteKey(ctx, siteKey)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(site.ChallengeTypes, challengeType) {
		return nil, ErrChallengeTypeNotAllowed
	}
	code := newCode()
	media, err := render(code)
	if err != nil {
		return nil, err
	}
	c := &Challenge{
		ID:       uuid.NewString(),
		Type:     challengeType,
		APIKeyID: site.APIKeyID,
		Answer:   code,
		Media:    media,
		TTL:      uc.ttl,
	}
	if err := uc.challenges.Save(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (uc *ChallengeUsecase) Check(ctx context.Context, id, answer string) (*Challenge, bool, error) {
	c, err := uc.challenges.Take(ctx, id)
	if err != nil {
		return nil, false, err
	}
	return c, strings.EqualFold(strings.TrimSpace(answer), c.Answer), nil
}

package biz

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	v1 "challenge/api/challenge/v1"
	"challenge/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

const (
	TypeImageDistort = "image_distort"
	TypeMotionNoise  = "motion_noise"
)

// challengeKind is how a challenge type builds its code and draws it as media.
type challengeKind struct {
	chars  string
	render func(code string) (Media, error)
}

var challengeKinds = map[string]challengeKind{
	TypeImageDistort: {chars: alphanumericChars, render: renderImageDistort},
	TypeMotionNoise:  {chars: uppercaseChars, render: renderMotionNoise},
}

var (
	ErrSiteKeyInvalid           = errors.Unauthorized(v1.ErrorReason_SITE_KEY_INVALID.String(), "site key is invalid")
	ErrChallengeTypeUnsupported = errors.BadRequest(v1.ErrorReason_CHALLENGE_TYPE_UNSUPPORTED.String(), "challenge type is not supported")
	ErrChallengeTypeNotAllowed  = errors.Forbidden(v1.ErrorReason_CHALLENGE_TYPE_NOT_ALLOWED.String(), "challenge type is not allowed")
	ErrChallengeNotFound        = errors.NotFound(v1.ErrorReason_CHALLENGE_NOT_FOUND.String(), "challenge not found")
	ErrApiKeySuspended          = errors.Forbidden(v1.ErrorReason_API_KEY_SUSPENDED.String(), "api key is suspended")
	ErrAccountBanned            = errors.Forbidden(v1.ErrorReason_ACCOUNT_BANNED.String(), "the owner of this api key is banned")
	ErrDailyQuotaExceeded       = errors.New(http.StatusTooManyRequests, v1.ErrorReason_DAILY_QUOTA_EXCEEDED.String(), "daily verification quota reached")
	ErrOriginNotAllowed         = errors.Forbidden(v1.ErrorReason_ORIGIN_NOT_ALLOWED.String(), "this address may not use this site key")
)

// Site is an active API key together with what its owner may do.
type Site struct {
	APIKeyID       string
	UserID         string
	ChallengeTypes []string
	Suspended      bool
	OwnerBanned    bool
	// Addresses and networks allowed to use the site key; empty allows every address.
	AllowedOrigins []netip.Prefix
	// Successful verifications the owner's tier allows per day.
	DailyQuota int64
}

type Media struct {
	Data     []byte
	MIMEType string
	// Width and Height are the display size in CSS pixels, which may be smaller than the media.
	Width, Height int
}

type Challenge struct {
	ID       string
	Type     string
	APIKeyID string
	// Owner of the key; a correct answer counts against their daily quota.
	UserID string
	Answer string
	Media  Media
	TTL    time.Duration
}

type SiteRepo interface {
	FindBySiteKey(ctx context.Context, siteKey string) (*Site, error)
	// SuspendForScraping suspends the key unless it already is.
	SuspendForScraping(ctx context.Context, apiKeyID string) error
}
type ChallengeRepo interface {
	Save(ctx context.Context, c *Challenge) error
	Take(ctx context.Context, id string) (*Challenge, error)
}

type ChallengeUsecase struct {
	sites      SiteRepo
	challenges ChallengeRepo
	counters   CounterRepo
	usage      UsageRepo
	ttl        time.Duration
	location   *time.Location
	now        func() time.Time
}

func NewChallengeUsecase(sites SiteRepo, challenges ChallengeRepo, counters CounterRepo, usage UsageRepo,
	challengeConf *conf.Challenge, quotaConf *conf.Quota) (*ChallengeUsecase, error) {
	ttl := challengeConf.GetTtl().AsDuration()
	if ttl <= 0 {
		return nil, fmt.Errorf("challenge ttl must be positive")
	}
	if quotaConf.GetTimeZone() == "" {
		return nil, fmt.Errorf("quota time_zone is required")
	}
	location, err := time.LoadLocation(quotaConf.GetTimeZone())
	if err != nil {
		return nil, err
	}
	return &ChallengeUsecase{
		sites:      sites,
		challenges: challenges,
		counters:   counters,
		usage:      usage,
		ttl:        ttl,
		location:   location,
		now:        time.Now,
	}, nil
}

// New issues a challenge of challengeType, image_distort when empty. client is the address
// the request comes from, which must be one the key allows.
func (uc *ChallengeUsecase) New(ctx context.Context, siteKey, challengeType string, client netip.Addr) (*Challenge, error) {
	if challengeType == "" {
		challengeType = TypeImageDistort
	}
	kind, ok := challengeKinds[challengeType]
	if !ok {
		return nil, ErrChallengeTypeUnsupported
	}
	site, err := uc.sites.FindBySiteKey(ctx, siteKey)
	if err != nil {
		return nil, err
	}
	switch {
	case site.OwnerBanned:
		return nil, ErrAccountBanned
	case site.Suspended:
		return nil, ErrApiKeySuspended
	case !addressAllowed(site.AllowedOrigins, client):
		return nil, ErrOriginNotAllowed
	case !slices.Contains(site.ChallengeTypes, challengeType):
		return nil, ErrChallengeTypeNotAllowed
	}
	if err := uc.admitChallenge(ctx, site); err != nil {
		return nil, err
	}
	code := newCode(kind.chars)
	media, err := kind.render(code)
	if err != nil {
		return nil, err
	}
	challenge := &Challenge{
		ID:       uuid.NewString(),
		Type:     challengeType,
		APIKeyID: site.APIKeyID,
		UserID:   site.UserID,
		Answer:   code,
		Media:    media,
		TTL:      uc.ttl,
	}
	if err := uc.challenges.Save(ctx, challenge); err != nil {
		return nil, err
	}
	uc.recordIssued(ctx, challenge)
	return challenge, nil
}

// addressAllowed reports whether a request may use the key. A key with no allowed origins
// is open to every address; an unspecified entry ("0.0.0.0" or "::") also stands for every
// address, and any other entry covers the addresses of its network. A request whose address
// is unknown only gets through those two.
func addressAllowed(allowed []netip.Prefix, client netip.Addr) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, prefix := range allowed {
		if prefix.Addr().IsUnspecified() {
			return true
		}
		if client.IsValid() && prefix.Contains(client.Unmap()) {
			return true
		}
	}
	return false
}

func (uc *ChallengeUsecase) Check(ctx context.Context, id, answer string) (*Challenge, bool, error) {
	challenge, err := uc.challenges.Take(ctx, id)
	if err != nil {
		return nil, false, err
	}
	return challenge, strings.EqualFold(strings.TrimSpace(answer), challenge.Answer), nil
}

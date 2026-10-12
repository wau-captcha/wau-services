package biz

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"time"

	v1 "challenge/api/challenge/v1"
	"challenge/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
)

const (
	TypeImageDistort = "image_distort"
	TypeMotionNoise  = "motion_noise"
	TypeSlidePuzzle  = "slide_puzzle"
	TypeAnimalJump   = "animal_jump"
	TypeObjectRotate = "object_rotate"
	// TypeAutoCheck shows no puzzle: it judges the browser, and sends who it does not let through to
	// an interactive type (see autocheck.go).
	TypeAutoCheck = "auto_check"
)

var (
	ErrSiteKeyInvalid           = errors.Unauthorized(v1.ErrorReason_SITE_KEY_INVALID.String(), "site key is invalid")
	ErrChallengeTypeUnsupported = errors.BadRequest(v1.ErrorReason_CHALLENGE_TYPE_UNSUPPORTED.String(), "challenge type is not supported")
	ErrChallengeTypeNotAllowed  = errors.Forbidden(v1.ErrorReason_CHALLENGE_TYPE_NOT_ALLOWED.String(), "challenge type is not allowed")
	ErrChallengeNotFound        = errors.NotFound(v1.ErrorReason_CHALLENGE_NOT_FOUND.String(), "challenge not found")
	ErrApiKeySuspended          = errors.Forbidden(v1.ErrorReason_API_KEY_SUSPENDED.String(), "api key is suspended")
	ErrAccountBanned            = errors.Forbidden(v1.ErrorReason_ACCOUNT_BANNED.String(), "the owner of this api key is banned")
	ErrDailyQuotaExceeded       = errors.New(http.StatusTooManyRequests, v1.ErrorReason_DAILY_QUOTA_EXCEEDED.String(), "daily verification quota reached")
	ErrOriginNotAllowed         = errors.Forbidden(v1.ErrorReason_ORIGIN_NOT_ALLOWED.String(), "this address may not use this site key")
	ErrDomainNotAllowed         = errors.Forbidden(v1.ErrorReason_DOMAIN_NOT_ALLOWED.String(), "this website may not use this site key")
	ErrChallengeIDInvalid       = errors.BadRequest(v1.ErrorReason_CHALLENGE_ID_INVALID.String(), "challenge_id is not a valid challenge id")
	ErrRateLimited              = errors.New(http.StatusTooManyRequests, v1.ErrorReason_RATE_LIMITED.String(), "too many requests")
	// ErrInternal stands in for any failure of Redis, Postgres or the code: their text stays in the log.
	ErrInternal = errors.InternalServer(v1.ErrorReason_INTERNAL.String(), "internal error")
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
	// Websites allowed to use the site key, as tenant stores them; empty allows any website.
	AllowedDomains []string
	// Answers without behaviour data are refused.
	RequireBehavior bool
	// When the scraping check last flagged the key; zero when it never did.
	FlaggedAt time.Time
	// Successful verifications the owner's tier allows per day.
	DailyQuota int64
}

// Media is what a challenge shows; empty for auto_check, which shows nothing to solve.
type Media struct {
	Data     []byte
	MIMEType string
	// Width and Height are the display size in CSS pixels, which may be smaller than the media.
	Width, Height int
	// Piece is the cut-out to drag over Data, set for slide_puzzle only.
	Piece *Piece
	// SecondRound is the second round, Data being the first: animal_jump's GIF, object_rotate's
	// strip; nil for the other types.
	SecondRound []byte
	// Frames is how many frames of Width × Height stand side by side in Data and SecondRound: 8
	// for object_rotate, 0 for the other types.
	Frames int
}

// Piece is a PNG with alpha shown over the background at x = 0, y = Y, and dragged sideways.
// Y, Width and Height are in CSS pixels of the background.
type Piece struct {
	Data             []byte
	Y, Width, Height int
}

type Challenge struct {
	ID       string
	Type     string
	APIKeyID string
	// The site key it was issued for, against which the site_key of the answer is checked.
	SiteKey string
	// Owner of the key; a correct answer counts against their daily quota.
	UserID string
	Answer string
	Media  Media
	TTL    time.Duration
	// IssuedAt is zero for a challenge stored by a version that did not record it.
	IssuedAt time.Time
	// The /challenge/new request, compared with the answer's: the website (scheme://host[:port]),
	// the client's address and a SHA-256 of its User-Agent. Empty when unknown, or stored by a
	// version that did not record them.
	Origin        string
	ClientIP      string
	UserAgentHash string
	// The key required behaviour data when the challenge was issued.
	RequireBehavior bool
	// The proof of work asked for; a difficulty of 0 asks for none.
	PowSalt       []byte
	PowDifficulty int
	// auto_check only: the interactive type the client loads when the check does not pass; empty
	// when the key allows none. Sent to the client with the challenge, not stored.
	FallbackType string
}

type SiteRepo interface {
	FindBySiteKey(ctx context.Context, siteKey string) (*Site, error)
	// SuspendForScraping suspends the key unless it already is, and reports whether it did.
	SuspendForScraping(ctx context.Context, apiKeyID string) (bool, error)
	// FlagForScraping flags the key unless it was flagged since dayStart, and reports whether it did.
	FlagForScraping(ctx context.Context, apiKeyID string, dayStart time.Time) (bool, error)
	// RecordEvent adds an entry by the system to the activity log of the key.
	RecordEvent(ctx context.Context, apiKeyID, action string, detail map[string]any) error
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
	limiter    RateLimiter
	history    IPHistory
	limits     limits
	scraping   scrapingRules
	ipLimits   ipLimits
	auto       autoCheckRules
	ttl        time.Duration
	powBits    int
	location   *time.Location
	now        func() time.Time
}

func NewChallengeUsecase(sites SiteRepo, challenges ChallengeRepo, counters CounterRepo, usage UsageRepo, limiter RateLimiter,
	history IPHistory, challengeConf *conf.Challenge, quotaConf *conf.Quota, rateConf *conf.RateLimit,
	scrapingConf *conf.Scraping, historyConf *conf.IPHistory, autoConf *conf.AutoCheck) (*ChallengeUsecase, error) {
	ttl := challengeConf.GetTtl().AsDuration()
	if ttl <= 0 {
		return nil, fmt.Errorf("challenge ttl must be positive")
	}
	if bits := challengeConf.GetPowDifficulty(); bits < 0 || bits > maxPowDifficulty {
		return nil, fmt.Errorf("challenge pow_difficulty must be between 0 and %d", maxPowDifficulty)
	}
	if quotaConf.GetTimeZone() == "" {
		return nil, fmt.Errorf("quota time_zone is required")
	}
	location, err := time.LoadLocation(quotaConf.GetTimeZone())
	if err != nil {
		return nil, err
	}
	auto, err := newAutoCheckRules(autoConf)
	if err != nil {
		return nil, err
	}
	return &ChallengeUsecase{
		sites:      sites,
		challenges: challenges,
		counters:   counters,
		usage:      usage,
		limiter:    limiter,
		history:    history,
		limits:     newLimits(rateConf),
		scraping:   newScrapingRules(scrapingConf),
		ipLimits:   newIPLimits(historyConf),
		auto:       auto,
		ttl:        ttl,
		powBits:    int(challengeConf.GetPowDifficulty()),
		location:   location,
		now:        time.Now,
	}, nil
}

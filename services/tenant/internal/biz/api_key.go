package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"tenant/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/uuid"
)

const (
	maxKeyNameLen = 64
	// Grace period of a replaced secret key when the config names none.
	defaultPreviousSecretTTL = 24 * time.Hour
	// 24 random bytes render as the 48 hex characters the seeded keys use.
	keyRandomBytes  = 24
	defaultMaxKeys  = 20
	siteKeyPrefix   = "pk_"
	secretKeyPrefix = "sk_"
)

// ApiKey is a captcha key pair owned by a user.
type ApiKey struct {
	ID         uuid.UUID
	Name       string
	SiteKey    string
	SecretKey  string
	IsActive   bool
	CreateDate time.Time
	// Set by an administrator or the scraping check; the key stops working whatever
	// IsActive says, and its owner cannot lift it.
	IsSuspended bool
	// SuspendedByAdmin or SuspendedForScraping while suspended, empty otherwise.
	SuspendReason string
	// Addresses allowed to use the site key; empty lets every address use it. Only read
	// for one key at a time, so List leaves it empty.
	AllowedOrigins []AllowedOrigin
	// Websites allowed to use the site key; empty lets any website use it. Left empty by List,
	// like AllowedOrigins.
	AllowedDomains []AllowedDomain
	// Answers without behaviour data are refused.
	RequireBehavior bool
	// When the scraping check last flagged the key; zero when it never did. ScrapingFlagged
	// tells whether that was today, when the key is throttled.
	ScrapingFlaggedAt time.Time
	ScrapingFlagged   bool
	// Today's counters of the scraping check; nil when they were not read.
	Today *ScrapingCount
	// The latest appeal of the key; nil when there is none or it was not read.
	LatestAppeal *Appeal
	// The secret key the last reset replaced, while it still works; empty otherwise.
	PreviousSecretKey string
	// When PreviousSecretKey stops working; zero when there is none.
	PreviousSecretExpiresAt time.Time
}

// Reasons a key can be suspended for, as stored in api_keys.suspend_reason.
const (
	SuspendedByAdmin     = "ADMIN"
	SuspendedForScraping = "SCRAPING"
)

// ApiKeyPatch holds the fields a partial update touches; nil means "leave as is".
type ApiKeyPatch struct {
	Name            *string
	IsActive        *bool
	RequireBehavior *bool
}

// ApiKeyRepo stores keys. Every method takes the owner so a user can only ever
// reach their own keys.
type ApiKeyRepo interface {
	List(ctx context.Context, userID uuid.UUID) ([]*ApiKey, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error)
	// Create returns ErrApiKeyLimitReached when the user already holds max keys. The domains go
	// in with the key.
	Create(ctx context.Context, userID uuid.UUID, name, siteKey, secretKey string, domains []string, maxKeys int) (*ApiKey, error)
	Update(ctx context.Context, userID, id uuid.UUID, patch ApiKeyPatch) (*ApiKey, error)
	// RotateSecret stores secretKey and keeps the replaced one working for grace, in one
	// statement, so the key is never left without a working secret.
	RotateSecret(ctx context.Context, userID, id uuid.UUID, secretKey string, grace time.Duration) (*ApiKey, error)
	RevokePreviousSecret(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error)
	// AllowedOrigins lists the addresses allowed to use the key, oldest first.
	AllowedOrigins(ctx context.Context, userID, id uuid.UUID) ([]AllowedOrigin, error)
	// SetAllowedOrigins replaces the whole list in one statement; an address that stays
	// keeps the date it was added on.
	SetAllowedOrigins(ctx context.Context, userID, id uuid.UUID, origins []AllowedOrigin) ([]AllowedOrigin, error)
	// AllowedDomains and SetAllowedDomains do the same for the websites allowed to use the key.
	AllowedDomains(ctx context.Context, userID, id uuid.UUID) ([]AllowedDomain, error)
	SetAllowedDomains(ctx context.Context, userID, id uuid.UUID, domains []string) ([]AllowedDomain, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

// ApiKeyUsecase issues and maintains the keys of one user.
type ApiKeyUsecase struct {
	keys        ApiKeyRepo
	events      EventRepo
	appeals     AppealRepo
	counters    CounterRepo
	calendar    *Calendar
	maxPerUser  int
	previousTTL time.Duration
}

func NewApiKeyUsecase(keys ApiKeyRepo, events EventRepo, appeals AppealRepo, counters CounterRepo, calendar *Calendar,
	keyConf *conf.ApiKey) *ApiKeyUsecase {
	maxPerUser := int(keyConf.GetMaxPerUser())
	if maxPerUser <= 0 {
		maxPerUser = defaultMaxKeys
	}
	previousTTL := keyConf.GetPreviousSecretTtl().AsDuration()
	if previousTTL <= 0 {
		previousTTL = defaultPreviousSecretTTL
	}
	return &ApiKeyUsecase{keys: keys, events: events, appeals: appeals, counters: counters, calendar: calendar,
		maxPerUser: maxPerUser, previousTTL: previousTTL}
}

// MaxPerUser is how many keys a user may hold at once.
func (uc *ApiKeyUsecase) MaxPerUser() int { return uc.maxPerUser }

func (uc *ApiKeyUsecase) List(ctx context.Context, userID uuid.UUID) ([]*ApiKey, error) {
	keys, err := uc.keys.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		uc.markFlagged(key)
	}
	return keys, nil
}

// Get returns one key with the addresses and websites allowed to use it, today's counters of
// the scraping check and its latest appeal.
func (uc *ApiKeyUsecase) Get(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error) {
	key, err := uc.keys.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if key.AllowedOrigins, err = uc.keys.AllowedOrigins(ctx, userID, id); err != nil {
		return nil, err
	}
	if key.AllowedDomains, err = uc.keys.AllowedDomains(ctx, userID, id); err != nil {
		return nil, err
	}
	if key.LatestAppeal, err = uc.appeals.Latest(ctx, userID, id); err != nil {
		return nil, err
	}
	// The counters live in Redis; without them the page still works.
	if counts, err := uc.counters.ScrapingCounts(ctx, []uuid.UUID{id}, uc.calendar.Today()); err != nil {
		log.WarnContext(ctx, "scraping counters not read", "err", err, "api_key_id", id)
	} else {
		key.Today = &counts[0]
	}
	uc.markFlagged(key)
	return key, nil
}

// Create issues a fresh site key and secret key, allowed on the websites given (any when none).
func (uc *ApiKeyUsecase) Create(ctx context.Context, userID uuid.UUID, name string, rawDomains []string) (*ApiKey, error) {
	name, err := normalizeKeyName(name)
	if err != nil {
		return nil, err
	}
	domains, err := NormalizeDomains(rawDomains)
	if err != nil {
		return nil, err
	}
	siteKey, err := newKeyValue(siteKeyPrefix)
	if err != nil {
		return nil, err
	}
	secretKey, err := newKeyValue(secretKeyPrefix)
	if err != nil {
		return nil, err
	}
	key, err := uc.keys.Create(ctx, userID, name, siteKey, secretKey, domains, uc.maxPerUser)
	if err != nil {
		return nil, err
	}
	// Inserted in the same statement as the key, so on the same date.
	for _, domain := range domains {
		key.AllowedDomains = append(key.AllowedDomains, AllowedDomain{Domain: domain, CreateDate: key.CreateDate})
	}
	detail := map[string]any{"name": name}
	if len(domains) > 0 {
		detail["allowed_domains"] = domains
	}
	recordEvent(ctx, uc.events, key.ID, Actor{ActorUser, userID}, EventCreated, detail)
	return key, nil
}

func (uc *ApiKeyUsecase) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if err := uc.keys.Delete(ctx, userID, id); err != nil {
		return err
	}
	recordEvent(ctx, uc.events, id, Actor{ActorUser, userID}, EventDeleted, nil)
	return nil
}

// Events returns the latest entries of the activity log of a key, newest first.
func (uc *ApiKeyUsecase) Events(ctx context.Context, userID, id uuid.UUID) ([]ApiKeyEvent, error) {
	// Reading the key first answers API_KEY_NOT_FOUND for another user's key, not an empty log.
	if _, err := uc.keys.Get(ctx, userID, id); err != nil {
		return nil, err
	}
	return uc.events.List(ctx, userID, id, maxEvents)
}

// markFlagged tells whether the scraping check flagged the key today.
func (uc *ApiKeyUsecase) markFlagged(key *ApiKey) {
	key.ScrapingFlagged = uc.calendar.IsToday(key.ScrapingFlaggedAt)
}

func normalizeKeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxKeyNameLen {
		return "", InvalidField("name", "name must be 1-64 characters")
	}
	return name, nil
}

// newKeyValue is a variable so tests can make generation predictable.
var newKeyValue = func(prefix string) (string, error) {
	randomBytes := make([]byte, keyRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(randomBytes), nil
}

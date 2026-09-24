package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/netip"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"tenant/internal/conf"

	"github.com/google/uuid"
)

const (
	maxKeyNameLen = 64
	// A key may name this many addresses, each with a description of this length.
	maxOrigins           = 50
	maxOriginDescription = 100
	// Grace period of a replaced secret key when the config names none.
	defaultPreviousSecretTTL = 24 * time.Hour
	// 24 random bytes render as the 48 hex characters the seeded keys use.
	keyRandomBytes  = 24
	defaultMaxKeys  = 10
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

// AllowedOrigin is one address, or network, allowed to use a site key.
type AllowedOrigin struct {
	// Canonical network, e.g. "203.0.113.4/32", "203.0.113.0/24" or "2001:db8::/32".
	IPAddress string
	// What the address is, chosen by the user; may be empty.
	Description string
	CreateDate  time.Time
}

// ApiKeyPatch holds the fields a partial update touches; nil means "leave as is".
type ApiKeyPatch struct {
	Name     *string
	IsActive *bool
}

// ApiKeyRepo stores keys. Every method takes the owner so a user can only ever
// reach their own keys.
type ApiKeyRepo interface {
	List(ctx context.Context, userID uuid.UUID) ([]*ApiKey, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error)
	// Create returns ErrApiKeyLimitReached when the user already holds max keys.
	Create(ctx context.Context, userID uuid.UUID, name, siteKey, secretKey string, maxKeys int) (*ApiKey, error)
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
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

// ApiKeyUsecase issues and maintains the keys of one user.
type ApiKeyUsecase struct {
	keys        ApiKeyRepo
	maxPerUser  int
	previousTTL time.Duration
}

func NewApiKeyUsecase(keys ApiKeyRepo, keyConf *conf.ApiKey) *ApiKeyUsecase {
	maxPerUser := int(keyConf.GetMaxPerUser())
	if maxPerUser <= 0 {
		maxPerUser = defaultMaxKeys
	}
	previousTTL := keyConf.GetPreviousSecretTtl().AsDuration()
	if previousTTL <= 0 {
		previousTTL = defaultPreviousSecretTTL
	}
	return &ApiKeyUsecase{keys: keys, maxPerUser: maxPerUser, previousTTL: previousTTL}
}

// MaxPerUser is how many keys a user may hold at once.
func (uc *ApiKeyUsecase) MaxPerUser() int { return uc.maxPerUser }

func (uc *ApiKeyUsecase) List(ctx context.Context, userID uuid.UUID) ([]*ApiKey, error) {
	return uc.keys.List(ctx, userID)
}

// Get returns one key together with the addresses allowed to use it.
func (uc *ApiKeyUsecase) Get(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error) {
	key, err := uc.keys.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	key.AllowedOrigins, err = uc.keys.AllowedOrigins(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// SetAllowedOrigins replaces the addresses allowed to use the key. An empty list lets
// every address use it again.
func (uc *ApiKeyUsecase) SetAllowedOrigins(ctx context.Context, userID, id uuid.UUID, origins []AllowedOrigin) (*ApiKey, error) {
	normalized, err := NormalizeOrigins(origins)
	if err != nil {
		return nil, err
	}
	// Reading the key first keeps another user's key indistinguishable from a missing one.
	key, err := uc.keys.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	key.AllowedOrigins, err = uc.keys.SetAllowedOrigins(ctx, userID, id, normalized)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// Create issues a fresh site key and secret key.
func (uc *ApiKeyUsecase) Create(ctx context.Context, userID uuid.UUID, name string) (*ApiKey, error) {
	name, err := normalizeKeyName(name)
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
	return uc.keys.Create(ctx, userID, name, siteKey, secretKey, uc.maxPerUser)
}

func (uc *ApiKeyUsecase) Update(ctx context.Context, userID, id uuid.UUID, patch ApiKeyPatch) (*ApiKey, error) {
	if patch.Name == nil && patch.IsActive == nil {
		return nil, InvalidField("update_mask", "nothing to update")
	}
	if patch.Name != nil {
		name, err := normalizeKeyName(*patch.Name)
		if err != nil {
			return nil, err
		}
		patch.Name = &name
	}
	return uc.keys.Update(ctx, userID, id, patch)
}

// RotateSecret issues a new secret key and keeps the replaced one working for the grace
// period, so a site can roll the new one out without downtime. A reset inside that window
// overwrites the previous key: only one is ever kept.
func (uc *ApiKeyUsecase) RotateSecret(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error) {
	secretKey, err := newKeyValue(secretKeyPrefix)
	if err != nil {
		return nil, err
	}
	return uc.keys.RotateSecret(ctx, userID, id, secretKey, uc.previousTTL)
}

// RevokePreviousSecret stops the replaced secret key before its grace period is over.
func (uc *ApiKeyUsecase) RevokePreviousSecret(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error) {
	return uc.keys.RevokePreviousSecret(ctx, userID, id)
}

func (uc *ApiKeyUsecase) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return uc.keys.Delete(ctx, userID, id)
}

func normalizeKeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxKeyNameLen {
		return "", InvalidField("name", "name must be 1-64 characters")
	}
	return name, nil
}

// NormalizeOrigins checks the addresses a key accepts and puts them in the form they are
// stored and shown in: a network, with a plain address covering only itself. An empty list
// is allowed and means every address may use the key.
func NormalizeOrigins(rawOrigins []AllowedOrigin) ([]AllowedOrigin, error) {
	if len(rawOrigins) > maxOrigins {
		return nil, InvalidField("ip_address", "at most 50 addresses")
	}
	origins := make([]AllowedOrigin, 0, len(rawOrigins))
	for _, entry := range rawOrigins {
		address, err := NormalizeAddress(entry.IPAddress)
		if err != nil {
			return nil, err
		}
		description := strings.TrimSpace(entry.Description)
		if utf8.RuneCountInString(description) > maxOriginDescription {
			return nil, InvalidField("description", "a description may be at most 100 characters")
		}
		if slices.ContainsFunc(origins, func(origin AllowedOrigin) bool { return origin.IPAddress == address }) {
			return nil, InvalidField("ip_address", "each address may be listed only once")
		}
		origins = append(origins, AllowedOrigin{IPAddress: address, Description: description})
	}
	return origins, nil
}

// NormalizeAddress reads an IPv4 or IPv6 address, with an optional prefix length, and
// returns its network: "203.0.113.4" becomes "203.0.113.4/32" and "203.0.113.4/24" the
// network it lies in, since only the network decides who gets through.
func NormalizeAddress(rawAddress string) (string, error) {
	value := strings.TrimSpace(rawAddress)
	invalid := InvalidField("ip_address", "an address is an IPv4 or IPv6 address with an optional prefix length, e.g. 203.0.113.4 or 203.0.113.0/24")
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String(), nil
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return "", invalid
	}
	address = address.Unmap()
	return netip.PrefixFrom(address, address.BitLen()).String(), nil
}

// newKeyValue is a variable so tests can make generation predictable.
var newKeyValue = func(prefix string) (string, error) {
	randomBytes := make([]byte, keyRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(randomBytes), nil
}

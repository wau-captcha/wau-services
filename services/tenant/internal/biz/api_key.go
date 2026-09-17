package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"tenant/internal/conf"

	"github.com/google/uuid"
)

const (
	maxKeyNameLen = 64
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
	Create(ctx context.Context, userID uuid.UUID, name, siteKey, secretKey string, max int) (*ApiKey, error)
	Update(ctx context.Context, userID, id uuid.UUID, patch ApiKeyPatch) (*ApiKey, error)
	RotateSecret(ctx context.Context, userID, id uuid.UUID, secretKey string) (*ApiKey, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

// ApiKeyUsecase issues and maintains the keys of one user.
type ApiKeyUsecase struct {
	keys ApiKeyRepo
	max  int
}

func NewApiKeyUsecase(keys ApiKeyRepo, c *conf.ApiKey) *ApiKeyUsecase {
	max := int(c.GetMaxPerUser())
	if max <= 0 {
		max = defaultMaxKeys
	}
	return &ApiKeyUsecase{keys: keys, max: max}
}

// MaxPerUser is how many keys a user may hold at once.
func (uc *ApiKeyUsecase) MaxPerUser() int { return uc.max }

func (uc *ApiKeyUsecase) List(ctx context.Context, userID uuid.UUID) ([]*ApiKey, error) {
	return uc.keys.List(ctx, userID)
}

func (uc *ApiKeyUsecase) Get(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error) {
	return uc.keys.Get(ctx, userID, id)
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
	return uc.keys.Create(ctx, userID, name, siteKey, secretKey, uc.max)
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

// RotateSecret replaces the secret key; the previous one stops working at once.
func (uc *ApiKeyUsecase) RotateSecret(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error) {
	secretKey, err := newKeyValue(secretKeyPrefix)
	if err != nil {
		return nil, err
	}
	return uc.keys.RotateSecret(ctx, userID, id, secretKey)
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

// newKeyValue is a variable so tests can make generation predictable.
var newKeyValue = func(prefix string) (string, error) {
	buf := make([]byte, keyRandomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}

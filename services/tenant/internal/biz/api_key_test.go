package biz

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"tenant/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

var (
	siteKeyPattern   = regexp.MustCompile(`^pk_[0-9a-f]{48}$`)
	secretKeyPattern = regexp.MustCompile(`^sk_[0-9a-f]{48}$`)
)

type fakeKeys struct {
	key       *ApiKey
	listErr   error
	getErr    error
	createErr error
	updateErr error
	deleteErr error
	created   struct {
		name, siteKey, secretKey string
		max                      int
	}
	patch     ApiKeyPatch
	newSecret string
}

func (f *fakeKeys) List(context.Context, uuid.UUID) ([]*ApiKey, error) {
	return []*ApiKey{f.key}, f.listErr
}

func (f *fakeKeys) Get(_ context.Context, _, id uuid.UUID) (*ApiKey, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.key == nil || f.key.ID != id {
		return nil, ErrApiKeyNotFound
	}
	return f.key, nil
}

func (f *fakeKeys) Create(_ context.Context, _ uuid.UUID, name, siteKey, secretKey string, max int) (*ApiKey, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created.name, f.created.siteKey, f.created.secretKey, f.created.max = name, siteKey, secretKey, max
	return &ApiKey{ID: uuid.New(), Name: name, SiteKey: siteKey, SecretKey: secretKey, IsActive: true}, nil
}

func (f *fakeKeys) Update(_ context.Context, _, _ uuid.UUID, patch ApiKeyPatch) (*ApiKey, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.patch = patch
	return f.key, nil
}

func (f *fakeKeys) RotateSecret(_ context.Context, _, _ uuid.UUID, secretKey string) (*ApiKey, error) {
	f.newSecret = secretKey
	return f.key, nil
}

func (f *fakeKeys) Delete(context.Context, uuid.UUID, uuid.UUID) error { return f.deleteErr }

func newKeyFixture() (*ApiKeyUsecase, *fakeKeys) {
	keys := &fakeKeys{key: &ApiKey{ID: uuid.New(), Name: "Web", SiteKey: "pk_old", SecretKey: "sk_old", IsActive: true}}
	return NewApiKeyUsecase(keys, &conf.ApiKey{MaxPerUser: 10}), keys
}

func TestCreateApiKeyValidatesName(t *testing.T) {
	for name, value := range map[string]string{
		"empty":      "",
		"whitespace": "   ",
		"too long":   strings.Repeat("k", 65),
	} {
		t.Run(name, func(t *testing.T) {
			uc, _ := newKeyFixture()
			_, err := uc.Create(context.Background(), uuid.New(), value)
			if !errors.Is(err, ErrInvalidArgument) || fieldOf(err) != "name" {
				t.Fatalf("err = %v (field %q), want invalid name", err, fieldOf(err))
			}
		})
	}
}

func TestCreateApiKeyGeneratesKeys(t *testing.T) {
	uc, keys := newKeyFixture()

	key, err := uc.Create(context.Background(), uuid.New(), "  Web  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if keys.created.name != "Web" {
		t.Fatalf("name = %q, want trimmed Web", keys.created.name)
	}
	if keys.created.max != 10 {
		t.Fatalf("max = %d, want 10", keys.created.max)
	}
	if !siteKeyPattern.MatchString(key.SiteKey) || !secretKeyPattern.MatchString(key.SecretKey) {
		t.Fatalf("keys = %q / %q, want pk_/sk_ plus 48 hex", key.SiteKey, key.SecretKey)
	}
	if key.SiteKey[3:] == key.SecretKey[3:] {
		t.Fatal("site key and secret key share the same random part")
	}
}

func TestCreateApiKeyPropagatesLimit(t *testing.T) {
	uc, keys := newKeyFixture()
	keys.createErr = ErrApiKeyLimitReached

	if _, err := uc.Create(context.Background(), uuid.New(), "Web"); !errors.Is(err, ErrApiKeyLimitReached) {
		t.Fatalf("err = %v, want ErrApiKeyLimitReached", err)
	}
}

func TestUpdateApiKey(t *testing.T) {
	t.Run("empty patch", func(t *testing.T) {
		uc, _ := newKeyFixture()
		_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), ApiKeyPatch{})
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("err = %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("blank name", func(t *testing.T) {
		uc, _ := newKeyFixture()
		blank := "  "
		_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), ApiKeyPatch{Name: &blank})
		if !errors.Is(err, ErrInvalidArgument) || fieldOf(err) != "name" {
			t.Fatalf("err = %v (field %q), want invalid name", err, fieldOf(err))
		}
	})

	t.Run("trims name and passes flags", func(t *testing.T) {
		uc, keys := newKeyFixture()
		name := "  Mobile  "
		active := false
		if _, err := uc.Update(context.Background(), uuid.New(), keys.key.ID, ApiKeyPatch{Name: &name, IsActive: &active}); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if keys.patch.Name == nil || *keys.patch.Name != "Mobile" {
			t.Fatalf("name = %v, want Mobile", keys.patch.Name)
		}
		if keys.patch.IsActive == nil || *keys.patch.IsActive {
			t.Fatalf("is_active = %v, want false", keys.patch.IsActive)
		}
	})

	t.Run("propagates not found", func(t *testing.T) {
		uc, keys := newKeyFixture()
		keys.updateErr = ErrApiKeyNotFound
		active := true
		_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), ApiKeyPatch{IsActive: &active})
		if !errors.Is(err, ErrApiKeyNotFound) {
			t.Fatalf("err = %v, want ErrApiKeyNotFound", err)
		}
	})
}

func TestRotateSecretKey(t *testing.T) {
	uc, keys := newKeyFixture()

	if _, err := uc.RotateSecret(context.Background(), uuid.New(), keys.key.ID); err != nil {
		t.Fatalf("RotateSecret: %v", err)
	}
	if !secretKeyPattern.MatchString(keys.newSecret) {
		t.Fatalf("secret = %q, want sk_ plus 48 hex", keys.newSecret)
	}
	if keys.newSecret == keys.key.SecretKey {
		t.Fatal("secret key did not change")
	}
}

func TestDeleteApiKeyPropagates(t *testing.T) {
	uc, keys := newKeyFixture()
	keys.deleteErr = ErrApiKeyNotFound

	if err := uc.Delete(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrApiKeyNotFound) {
		t.Fatalf("err = %v, want ErrApiKeyNotFound", err)
	}
}

func TestMaxPerUser(t *testing.T) {
	uc, _ := newKeyFixture()
	if uc.MaxPerUser() != 10 {
		t.Fatalf("MaxPerUser = %d, want 10", uc.MaxPerUser())
	}
}

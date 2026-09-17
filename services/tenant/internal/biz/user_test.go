package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type fakeCredential struct {
	id   uuid.UUID
	hash string
}

type fakeUsers struct {
	createErr error
	created   struct{ username, email, hash string }
	creds     map[string]fakeCredential
	lookedUp  string
	profile   *User
}

func (f *fakeUsers) Create(_ context.Context, username, email, hash string) (uuid.UUID, error) {
	if f.createErr != nil {
		return uuid.Nil, f.createErr
	}
	f.created.username, f.created.email, f.created.hash = username, email, hash
	return f.profile.ID, nil
}

func (f *fakeUsers) FindCredentials(_ context.Context, identifier string) (uuid.UUID, string, error) {
	f.lookedUp = identifier
	c, ok := f.creds[identifier]
	if !ok {
		return uuid.Nil, "", ErrInvalidCredentials
	}
	return c.id, c.hash, nil
}

func (f *fakeUsers) Profile(_ context.Context, id uuid.UUID) (*User, error) {
	if f.profile == nil || f.profile.ID != id {
		return nil, ErrUnauthenticated
	}
	return f.profile, nil
}

type fakeHasher struct{ compares int }

func (h *fakeHasher) Hash(password string) (string, error) { return "hash:" + password, nil }

func (h *fakeHasher) Compare(hash, password string) bool {
	h.compares++
	return hash == "hash:"+password
}

type fakeTokens struct{}

func (fakeTokens) Issue(id uuid.UUID) (string, time.Duration, error) {
	return "token-" + id.String(), 24 * time.Hour, nil
}

func newAuthFixture() (*AuthUsecase, *fakeUsers, *fakeHasher) {
	users := &fakeUsers{
		creds:   map[string]fakeCredential{},
		profile: &User{ID: uuid.New(), Username: "ann", Email: "a@b.com", TierID: "free", TierName: "Free"},
	}
	hasher := &fakeHasher{}
	return NewAuthUsecase(users, hasher, fakeTokens{}), users, hasher
}

func fieldOf(err error) string {
	return errors.FromError(err).Metadata["field"]
}

func TestRegisterNormalizesInput(t *testing.T) {
	uc, users, _ := newAuthFixture()

	session, err := uc.Register(context.Background(), "  Ann  ", " A@B.COM ", "password1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if users.created.username != "ann" || users.created.email != "a@b.com" {
		t.Fatalf("stored %q / %q, want ann / a@b.com", users.created.username, users.created.email)
	}
	if users.created.hash != "hash:password1" {
		t.Fatalf("hash = %q, want hash:password1", users.created.hash)
	}
	if session.AccessToken != "token-"+users.profile.ID.String() || session.ExpiresIn != 24*time.Hour {
		t.Fatalf("session = %+v", session)
	}
	if session.User.TierName != "Free" {
		t.Fatalf("user = %+v, want the profile from the repo", session.User)
	}
}

func TestRegisterValidation(t *testing.T) {
	tests := map[string]struct{ username, email, password, field string }{
		"short username":   {"ab", "a@b.com", "password1", "username"},
		"bad characters":   {"Ann!", "a@b.com", "password1", "username"},
		"long username":    {strings.Repeat("a", 33), "a@b.com", "password1", "username"},
		"bad email":        {"ann", "nope", "password1", "email"},
		"email with name":  {"ann", "Ann <a@b.com>", "password1", "email"},
		"short password":   {"ann", "a@b.com", "pass123", "password"},
		"long password":    {"ann", "a@b.com", strings.Repeat("x", 73), "password"},
		"empty everything": {"", "", "", "username"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			uc, _, _ := newAuthFixture()
			_, err := uc.Register(context.Background(), tt.username, tt.email, tt.password)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("err = %v, want ErrInvalidArgument", err)
			}
			if got := fieldOf(err); got != tt.field {
				t.Fatalf("field = %q, want %q", got, tt.field)
			}
		})
	}
}

func TestRegisterPropagatesConflict(t *testing.T) {
	uc, users, _ := newAuthFixture()
	users.createErr = ErrUsernameTaken

	if _, err := uc.Register(context.Background(), "ann", "a@b.com", "password1"); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("err = %v, want ErrUsernameTaken", err)
	}
}

func TestLogin(t *testing.T) {
	t.Run("by username", func(t *testing.T) {
		uc, users, _ := newAuthFixture()
		users.creds["ann"] = fakeCredential{id: users.profile.ID, hash: "hash:password1"}

		session, err := uc.Login(context.Background(), " Ann ", "password1")
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		if session.User.ID != users.profile.ID {
			t.Fatalf("user = %+v", session.User)
		}
	})

	t.Run("by email lowercased", func(t *testing.T) {
		uc, users, _ := newAuthFixture()
		users.creds["a@b.com"] = fakeCredential{id: users.profile.ID, hash: "hash:password1"}

		if _, err := uc.Login(context.Background(), "A@B.COM", "password1"); err != nil {
			t.Fatalf("Login: %v", err)
		}
		if users.lookedUp != "a@b.com" {
			t.Fatalf("looked up %q, want a@b.com", users.lookedUp)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		uc, users, _ := newAuthFixture()
		users.creds["ann"] = fakeCredential{id: users.profile.ID, hash: "hash:password1"}

		if _, err := uc.Login(context.Background(), "ann", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("err = %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("unknown identifier still hashes", func(t *testing.T) {
		uc, _, hasher := newAuthFixture()

		if _, err := uc.Login(context.Background(), "nobody", "password1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("err = %v, want ErrInvalidCredentials", err)
		}
		if hasher.compares != 1 {
			t.Fatalf("compares = %d, want 1 (constant-ish time)", hasher.compares)
		}
	})
}

func TestMe(t *testing.T) {
	uc, users, _ := newAuthFixture()

	user, err := uc.Me(context.Background(), users.profile.ID)
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if user.Username != "ann" {
		t.Fatalf("user = %+v", user)
	}
	if _, err := uc.Me(context.Background(), uuid.New()); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}

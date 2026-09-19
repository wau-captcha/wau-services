package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

// fakeAdmins mimics FindAdminByEmail and GetAdmin in data/queries/admin.sql.
type fakeAdmins struct {
	admins []*Admin
}

func (f *fakeAdmins) FindByEmail(_ context.Context, email string) (*Admin, error) {
	for _, admin := range f.admins {
		if strings.EqualFold(admin.Email, email) {
			copied := *admin
			return &copied, nil
		}
	}
	return nil, ErrNotAdmin
}

func (f *fakeAdmins) Get(_ context.Context, id uuid.UUID) (*Admin, error) {
	for _, admin := range f.admins {
		if admin.ID == id {
			copied := *admin
			return &copied, nil
		}
	}
	return nil, ErrUnauthenticated
}

type fakeAdminTokens struct{}

func (fakeAdminTokens) IssueAdmin(id uuid.UUID) (string, time.Duration, error) {
	return "admin-token-" + id.String(), 24 * time.Hour, nil
}

func newAdminAuthFixture(identity GoogleIdentity, admins ...*Admin) (*AdminAuthUsecase, *fakeVerifier) {
	verifier := &fakeVerifier{identity: identity}
	return NewAdminAuthUsecase(&fakeAdmins{admins: admins}, verifier, fakeAdminTokens{}), verifier
}

func TestAdminSignInMatchesTheEmailCaseInsensitively(t *testing.T) {
	admin := &Admin{ID: uuid.New(), Username: "Admin", Email: "Ann@Gmail.com"}
	uc, verifier := newAdminAuthFixture(ann, admin)

	session, err := uc.SignInWithGoogle(context.Background(), "id-token", "nonce")
	if err != nil {
		t.Fatalf("SignInWithGoogle: %v", err)
	}
	if verifier.gotToken != "id-token" || verifier.gotNon != "nonce" {
		t.Fatalf("verifier got %q/%q", verifier.gotToken, verifier.gotNon)
	}
	if session.Admin.ID != admin.ID || session.Admin.Username != "Admin" {
		t.Fatalf("admin = %+v", session.Admin)
	}
	if session.AccessToken != "admin-token-"+admin.ID.String() || session.ExpiresIn != 24*time.Hour {
		t.Fatalf("session = %+v", session)
	}
}

func TestAdminSignInRefusals(t *testing.T) {
	unverified := ann
	unverified.EmailVerified, unverified.EmailAuthoritative = false, false
	// A Google account can carry an address of any domain, but only Google-owned ones prove who holds it.
	notOwned := ann
	notOwned.Email, notOwned.EmailAuthoritative = "ann@example.com", false
	stranger := ann
	stranger.Email = "bob@gmail.com"

	tests := map[string]struct {
		identity GoogleIdentity
		admin    string
		want     error
	}{
		"not an administrator":        {stranger, "ann@gmail.com", ErrNotAdmin},
		"unverified email":            {unverified, "ann@gmail.com", ErrGoogleEmailUnverified},
		"address Google does not own": {notOwned, "ann@example.com", ErrNotAdmin},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			uc, _ := newAdminAuthFixture(tt.identity, &Admin{ID: uuid.New(), Email: tt.admin})
			if _, err := uc.SignInWithGoogle(context.Background(), "id-token", "nonce"); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestAdminSignInPropagatesVerifierError(t *testing.T) {
	uc, verifier := newAdminAuthFixture(ann, &Admin{ID: uuid.New(), Email: "ann@gmail.com"})
	verifier.err = ErrGoogleTokenInvalid
	if _, err := uc.SignInWithGoogle(context.Background(), "bad", "nonce"); !errors.Is(err, ErrGoogleTokenInvalid) {
		t.Fatalf("err = %v, want ErrGoogleTokenInvalid", err)
	}
}

func TestAdminMe(t *testing.T) {
	admin := &Admin{ID: uuid.New(), Username: "Admin", Email: "ann@gmail.com"}
	uc, _ := newAdminAuthFixture(ann, admin)

	if got, err := uc.Me(context.Background(), admin.ID); err != nil || got.Email != admin.Email {
		t.Fatalf("Me = %+v, %v", got, err)
	}
	if _, err := uc.Me(context.Background(), uuid.New()); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("removed administrator: err = %v, want ErrUnauthenticated", err)
	}
}

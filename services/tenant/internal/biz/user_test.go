package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

func TestSignInCreatesAccount(t *testing.T) {
	uc, repo, verifier := newAuthFixture(ann)

	session := signIn(t, uc)
	if verifier.gotToken != "id-token" || verifier.gotNon != "nonce" {
		t.Fatalf("verifier got %q/%q", verifier.gotToken, verifier.gotNon)
	}
	if repo.creates != 1 || len(repo.users) != 1 {
		t.Fatalf("creates = %d, users = %d, want 1/1", repo.creates, len(repo.users))
	}
	if session.User.Username != "Ann Lee" || session.User.Email != "ann@gmail.com" {
		t.Fatalf("user = %+v", session.User)
	}
	if session.AccessToken != "token-"+session.User.ID.String() || session.ExpiresIn != 24*time.Hour {
		t.Fatalf("session = %+v", session)
	}
}

func TestSignInReturnsExistingAccount(t *testing.T) {
	existing := &memUser{id: uuid.New(), sub: "sub-ann", email: "old@gmail.com", name: "Old name"}
	uc, repo, _ := newAuthFixture(ann, existing)

	session := signIn(t, uc)
	if session.User.ID != existing.id || repo.creates != 0 || repo.updates != 1 {
		t.Fatalf("id=%v creates=%d updates=%d", session.User.ID, repo.creates, repo.updates)
	}
	if session.User.Username != "Ann Lee" || session.User.Email != "ann@gmail.com" {
		t.Fatalf("profile not refreshed: %+v", session.User)
	}
}

func TestSignInKeepsEmailOwnedByAnotherAccount(t *testing.T) {
	existing := &memUser{id: uuid.New(), sub: "sub-ann", email: "ann@gmail.com", name: "Ann"}
	other := &memUser{id: uuid.New(), email: "shared@gmail.com", name: "Legacy"}
	identity := ann
	identity.Email = "shared@gmail.com"
	uc, _, _ := newAuthFixture(identity, existing, other)

	if got := signIn(t, uc).User.Email; got != "ann@gmail.com" {
		t.Fatalf("email = %q, want the old one kept", got)
	}
}

func TestSignInAllowsExistingAccountWithUnverifiedEmail(t *testing.T) {
	existing := &memUser{id: uuid.New(), sub: "sub-ann", email: "ann@gmail.com", name: "Ann"}
	identity := ann
	identity.Email = "someone-else@example.com"
	identity.EmailVerified, identity.EmailAuthoritative = false, false
	uc, _, _ := newAuthFixture(identity, existing)

	session := signIn(t, uc)
	if session.User.ID != existing.id {
		t.Fatalf("id = %v, want %v", session.User.ID, existing.id)
	}
	if existing.email != "ann@gmail.com" || existing.name != "Ann Lee" {
		t.Fatalf("unverified email stored or name not refreshed: %+v", existing)
	}
}

func TestSignInLinksLegacyAccountWithGoogleOwnedEmail(t *testing.T) {
	legacy := &memUser{id: uuid.New(), email: "ann@gmail.com", name: "john_doe"}
	uc, repo, _ := newAuthFixture(ann, legacy)

	session := signIn(t, uc)
	if session.User.ID != legacy.id || repo.links != 1 || repo.creates != 0 {
		t.Fatalf("id=%v links=%d creates=%d", session.User.ID, repo.links, repo.creates)
	}
	if legacy.sub != "sub-ann" || session.User.Username != "Ann Lee" {
		t.Fatalf("legacy account not linked: %+v", legacy)
	}
}

func TestSignInDoesNotLinkEmailGoogleDoesNotOwn(t *testing.T) {
	legacy := &memUser{id: uuid.New(), email: "ann@yahoo.com", name: "john_doe"}
	identity := ann
	identity.Email, identity.EmailAuthoritative = "ann@yahoo.com", false
	uc, repo, _ := newAuthFixture(identity, legacy)

	_, err := uc.SignInWithGoogle(context.Background(), "id-token", "nonce")
	if !errors.Is(err, ErrGoogleAccountConflict) {
		t.Fatalf("err = %v, want ErrGoogleAccountConflict", err)
	}
	if repo.links != 0 || legacy.sub != "" {
		t.Fatalf("legacy account was linked: links=%d sub=%q", repo.links, legacy.sub)
	}
}

func TestSignInRejectsUnverifiedEmailForNewAccount(t *testing.T) {
	identity := ann
	identity.EmailVerified, identity.EmailAuthoritative = false, false
	uc, repo, _ := newAuthFixture(identity)

	if _, err := uc.SignInWithGoogle(context.Background(), "id-token", "nonce"); !errors.Is(err, ErrGoogleEmailUnverified) {
		t.Fatalf("err = %v, want ErrGoogleEmailUnverified", err)
	}
	if repo.creates != 0 {
		t.Fatalf("creates = %d, want 0", repo.creates)
	}
}

func TestSignInRecoversFromConcurrentFirstSignIn(t *testing.T) {
	for _, raceErr := range []error{ErrGoogleSubTaken, ErrGoogleAccountConflict} {
		uc, repo, _ := newAuthFixture(ann)
		repo.raceSub, repo.raceErr = "sub-ann", raceErr

		session := signIn(t, uc)
		if session.User.Username != "Racer" || len(repo.users) != 1 {
			t.Fatalf("%v: want the account created by the other request, got %+v (users=%d)", raceErr, session.User, len(repo.users))
		}
	}
}

func TestSignInPropagatesVerifierError(t *testing.T) {
	uc, repo, verifier := newAuthFixture(ann)
	verifier.err = ErrGoogleTokenInvalid

	if _, err := uc.SignInWithGoogle(context.Background(), "bad", "nonce"); !errors.Is(err, ErrGoogleTokenInvalid) {
		t.Fatalf("err = %v, want ErrGoogleTokenInvalid", err)
	}
	if repo.creates+repo.links+repo.updates != 0 {
		t.Fatal("repository touched for an invalid token")
	}
}

func TestDisplayName(t *testing.T) {
	long := strings.Repeat("é", 70)
	tests := map[string]struct{ name, email, want string }{
		"google name":     {"  Nguyễn Văn A ", "a@gmail.com", "Nguyễn Văn A"},
		"email fallback":  {"   ", "bob.smith@gmail.com", "bob.smith"},
		"truncated runes": {long, "a@gmail.com", strings.Repeat("é", 64)},
		"nothing known":   {"", "", ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := displayName(&GoogleIdentity{Name: tt.name, Email: tt.email}); got != tt.want {
				t.Fatalf("displayName = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMe(t *testing.T) {
	existing := &memUser{id: uuid.New(), sub: "sub-ann", email: "ann@gmail.com", name: "Ann"}
	uc, _, _ := newAuthFixture(ann, existing)

	user, err := uc.Me(context.Background(), existing.id)
	if err != nil || user.Username != "Ann" {
		t.Fatalf("Me = %+v, %v", user, err)
	}
	if _, err := uc.Me(context.Background(), uuid.New()); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestBannedAccountCannotSignIn(t *testing.T) {
	existing := &memUser{id: uuid.New(), sub: "sub-ann", email: "ann@gmail.com", name: "Ann", banned: true}
	uc, _, _ := newAuthFixture(ann, existing)

	if _, err := uc.SignInWithGoogle(context.Background(), "id-token", "nonce"); !errors.Is(err, ErrAccountBanned) {
		t.Fatalf("err = %v, want ErrAccountBanned", err)
	}
}

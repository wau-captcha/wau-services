package biz

import (
	"context"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type fakeVerifier struct {
	identity         GoogleIdentity
	err              error
	gotToken, gotNon string
}

func (f *fakeVerifier) Verify(_ context.Context, token, nonce string) (*GoogleIdentity, error) {
	f.gotToken, f.gotNon = token, nonce
	if f.err != nil {
		return nil, f.err
	}
	identity := f.identity
	return &identity, nil
}

type memUser struct {
	id               uuid.UUID
	sub, email, name string
	banned           bool
}

// fakeUsers mimics the SQL in data/queries/users.sql.
type fakeUsers struct {
	users []*memUser
	// raceSub makes the next create lose to a concurrent insert of that Google account,
	// reported as raceErr: PostgreSQL may name either unique index.
	raceSub                 string
	raceErr                 error
	creates, links, updates int
}

func (f *fakeUsers) find(match func(*memUser) bool) *memUser {
	for _, u := range f.users {
		if match(u) {
			return u
		}
	}
	return nil
}

func (f *fakeUsers) FindByGoogleSub(_ context.Context, sub string) (uuid.UUID, error) {
	if u := f.find(func(u *memUser) bool { return u.sub == sub }); u != nil {
		return u.id, nil
	}
	return uuid.Nil, ErrUserNotFound
}

func (f *fakeUsers) LinkGoogleAccount(_ context.Context, email, sub, name string) (uuid.UUID, error) {
	u := f.find(func(u *memUser) bool { return u.email == email && u.sub == "" })
	if u == nil {
		return uuid.Nil, ErrUserNotFound
	}
	f.links++
	u.sub, u.name = sub, name
	return u.id, nil
}

func (f *fakeUsers) CreateGoogleUser(_ context.Context, sub, email, name string) (uuid.UUID, error) {
	f.creates++
	if f.raceSub == sub {
		f.users = append(f.users, &memUser{id: uuid.New(), sub: sub, email: email, name: "Racer"})
		f.raceSub = ""
		return uuid.Nil, f.raceErr
	}
	if f.find(func(u *memUser) bool { return u.email == email }) != nil {
		return uuid.Nil, ErrGoogleAccountConflict
	}
	u := &memUser{id: uuid.New(), sub: sub, email: email, name: name}
	f.users = append(f.users, u)
	return u.id, nil
}

func (f *fakeUsers) UpdateGoogleProfile(_ context.Context, id uuid.UUID, email, name string) error {
	f.updates++
	u := f.find(func(u *memUser) bool { return u.id == id })
	if name != "" {
		u.name = name
	}
	if email != "" && f.find(func(o *memUser) bool { return o.email == email && o.id != id }) == nil {
		u.email = email
	}
	return nil
}

func (f *fakeUsers) Profile(_ context.Context, id uuid.UUID) (*User, error) {
	u := f.find(func(u *memUser) bool { return u.id == id })
	if u == nil {
		return nil, ErrUnauthenticated
	}
	return &User{ID: u.id, Username: u.name, Email: u.email, TierID: "free", TierName: "Free",
		DailyQuota: 1000, IsBanned: u.banned}, nil
}

func (f *fakeUsers) Status(_ context.Context, id uuid.UUID) (AccountStatus, error) {
	u := f.find(func(u *memUser) bool { return u.id == id })
	if u == nil {
		return AccountStatus{}, nil
	}
	return AccountStatus{Exists: true, Banned: u.banned}, nil
}

type fakeTokens struct{}

func (fakeTokens) Issue(id uuid.UUID) (string, time.Duration, error) {
	return "token-" + id.String(), 24 * time.Hour, nil
}

var ann = GoogleIdentity{Subject: "sub-ann", Email: "ann@gmail.com", EmailVerified: true, EmailAuthoritative: true, Name: "Ann Lee"}

func newAuthFixture(identity GoogleIdentity, users ...*memUser) (*AuthUsecase, *fakeUsers, *fakeVerifier) {
	repo := &fakeUsers{users: users}
	verifier := &fakeVerifier{identity: identity}
	return NewAuthUsecase(repo, verifier, fakeTokens{}), repo, verifier
}

func signIn(t *testing.T, uc *AuthUsecase) *Session {
	t.Helper()
	session, err := uc.SignInWithGoogle(context.Background(), "id-token", "nonce")
	if err != nil {
		t.Fatalf("SignInWithGoogle: %v", err)
	}
	return session
}

// fieldOf reads the input name an InvalidField error points at.
func fieldOf(err error) string {
	return errors.FromError(err).Metadata["field"]
}

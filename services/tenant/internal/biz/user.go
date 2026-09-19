package biz

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// User is a dashboard account with the tier it belongs to.
type User struct {
	ID uuid.UUID
	// Username is the display name taken from Google; it is not unique and not used to sign in.
	Username   string
	Email      string
	TierID     string
	TierName   string
	CreateDate time.Time
	// Successful verifications the tier allows per local day.
	DailyQuota int64
	IsBanned   bool
}

// AccountStatus is what every authenticated request checks: tokens cannot be revoked, so
// a deleted or banned account is only stopped by looking it up.
type AccountStatus struct {
	Exists bool
	Banned bool
}

// Session is an issued access token together with its owner.
type Session struct {
	AccessToken string
	ExpiresIn   time.Duration
	User        *User
}

// Repository outcomes that are handled inside the service and never reach clients.
var (
	ErrUserNotFound   = errors.New("user not found")
	ErrGoogleSubTaken = errors.New("google account already has a user")
)

// UserRepo stores accounts.
type UserRepo interface {
	// FindByGoogleSub returns ErrUserNotFound when no account belongs to the Google account.
	FindByGoogleSub(ctx context.Context, sub string) (uuid.UUID, error)
	// LinkGoogleAccount attaches the Google account to the account with this email if it has
	// none yet; ErrUserNotFound otherwise.
	LinkGoogleAccount(ctx context.Context, email, sub, name string) (uuid.UUID, error)
	// CreateGoogleUser returns ErrGoogleAccountConflict when the email is taken and
	// ErrGoogleSubTaken when the Google account already has a user.
	CreateGoogleUser(ctx context.Context, sub, email, name string) (uuid.UUID, error)
	// UpdateGoogleProfile refreshes the display name, and the email unless another account
	// uses it. Empty values leave the stored ones unchanged.
	UpdateGoogleProfile(ctx context.Context, id uuid.UUID, email, name string) error
	Profile(ctx context.Context, id uuid.UUID) (*User, error)
	// Status reports whether the account is still there, and whether it is banned.
	Status(ctx context.Context, id uuid.UUID) (AccountStatus, error)
}

// TokenIssuer mints access tokens.
type TokenIssuer interface {
	Issue(userID uuid.UUID) (string, time.Duration, error)
}

// AuthUsecase signs users in with Google.
type AuthUsecase struct {
	users  UserRepo
	google GoogleVerifier
	tokens TokenIssuer
}

func NewAuthUsecase(users UserRepo, google GoogleVerifier, tokens TokenIssuer) *AuthUsecase {
	return &AuthUsecase{users: users, google: google, tokens: tokens}
}

const maxDisplayNameLen = 64

// SignInWithGoogle turns a Google ID token into a session, creating the account on the
// first sign-in of that Google account.
func (uc *AuthUsecase) SignInWithGoogle(ctx context.Context, idToken, nonce string) (*Session, error) {
	identity, err := uc.google.Verify(ctx, idToken, nonce)
	if err != nil {
		return nil, err
	}
	name := displayName(identity)

	id, err := uc.users.FindByGoogleSub(ctx, identity.Subject)
	switch {
	case err == nil:
		// Keep the display name and email in step with the Google profile. An unverified
		// address is never stored: it could block the real owner from signing up.
		email := ""
		if identity.EmailVerified {
			email = identity.Email
		}
		if err := uc.users.UpdateGoogleProfile(ctx, id, email, name); err != nil {
			return nil, err
		}
	case errors.Is(err, ErrUserNotFound):
		if id, err = uc.firstSignIn(ctx, identity, name); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	return uc.session(ctx, id)
}

// Me returns the signed-in account.
func (uc *AuthUsecase) Me(ctx context.Context, userID uuid.UUID) (*User, error) {
	return uc.users.Profile(ctx, userID)
}

func (uc *AuthUsecase) firstSignIn(ctx context.Context, identity *GoogleIdentity, name string) (uuid.UUID, error) {
	if !identity.EmailVerified {
		return uuid.Nil, ErrGoogleEmailUnverified
	}
	id, err := uc.claimOrCreate(ctx, identity, name)
	if errors.Is(err, ErrGoogleSubTaken) || errors.Is(err, ErrGoogleAccountConflict) {
		// A concurrent first sign-in of the same Google account may have got there first;
		// PostgreSQL then reports whichever unique index it checked first.
		existing, findErr := uc.users.FindByGoogleSub(ctx, identity.Subject)
		switch {
		case findErr == nil:
			return existing, nil
		case !errors.Is(findErr, ErrUserNotFound):
			return uuid.Nil, findErr
		}
	}
	return id, err
}

func (uc *AuthUsecase) claimOrCreate(ctx context.Context, identity *GoogleIdentity, name string) (uuid.UUID, error) {
	// Accounts from before Google sign-in are only claimed through addresses Google owns;
	// for any other address Google cannot vouch that the person still controls it.
	if identity.EmailAuthoritative {
		id, err := uc.users.LinkGoogleAccount(ctx, identity.Email, identity.Subject, name)
		if !errors.Is(err, ErrUserNotFound) {
			return id, err
		}
	}
	return uc.users.CreateGoogleUser(ctx, identity.Subject, identity.Email, name)
}

func (uc *AuthUsecase) session(ctx context.Context, id uuid.UUID) (*Session, error) {
	user, err := uc.users.Profile(ctx, id)
	if err != nil {
		return nil, err
	}
	if user.IsBanned {
		return nil, ErrAccountBanned
	}
	token, ttl, err := uc.tokens.Issue(id)
	if err != nil {
		return nil, err
	}
	return &Session{AccessToken: token, ExpiresIn: ttl, User: user}, nil
}

// displayName prefers the Google profile name and falls back to the email's local part.
func displayName(identity *GoogleIdentity) string {
	name := strings.TrimSpace(identity.Name)
	if name == "" {
		name, _, _ = strings.Cut(identity.Email, "@")
	}
	if utf8.RuneCountInString(name) > maxDisplayNameLen {
		name = string([]rune(name)[:maxDisplayNameLen])
	}
	return name
}

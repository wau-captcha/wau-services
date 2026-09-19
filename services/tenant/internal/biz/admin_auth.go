package biz

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Admin is an administrator of the admin console. Administrators are not dashboard users:
// they have their own table, changed only with SQL, their own sign-in and their own
// tokens. One person may be both, with the same Gmail.
type Admin struct {
	ID         uuid.UUID
	Username   string
	Email      string
	CreateDate time.Time
}

// AdminSession is an issued administrator token together with its owner.
type AdminSession struct {
	AccessToken string
	ExpiresIn   time.Duration
	Admin       *Admin
}

// AdminAccountRepo reads administrators.
type AdminAccountRepo interface {
	// FindByEmail matches case-insensitively and returns ErrNotAdmin when no one does.
	FindByEmail(ctx context.Context, email string) (*Admin, error)
	// Get returns ErrUnauthenticated for an administrator that was removed.
	Get(ctx context.Context, id uuid.UUID) (*Admin, error)
}

// AdminTokenIssuer mints administrator tokens, which user endpoints never accept.
type AdminTokenIssuer interface {
	IssueAdmin(adminID uuid.UUID) (string, time.Duration, error)
}

// AdminAuthUsecase signs administrators in with Google.
type AdminAuthUsecase struct {
	admins AdminAccountRepo
	google GoogleVerifier
	tokens AdminTokenIssuer
}

func NewAdminAuthUsecase(admins AdminAccountRepo, google GoogleVerifier, tokens AdminTokenIssuer) *AdminAuthUsecase {
	return &AdminAuthUsecase{admins: admins, google: google, tokens: tokens}
}

// SignInWithGoogle turns a Google ID token into an administrator session. The admins table
// keeps no Google account id, so the email is the only link; it is trusted only when Google
// owns the address (Gmail or Workspace), so no other Google account can present it.
func (uc *AdminAuthUsecase) SignInWithGoogle(ctx context.Context, idToken, nonce string) (*AdminSession, error) {
	identity, err := uc.google.Verify(ctx, idToken, nonce)
	if err != nil {
		return nil, err
	}
	if !identity.EmailVerified {
		return nil, ErrGoogleEmailUnverified
	}
	if !identity.EmailAuthoritative {
		return nil, ErrNotAdmin
	}
	admin, err := uc.admins.FindByEmail(ctx, identity.Email)
	if err != nil {
		return nil, err
	}
	token, ttl, err := uc.tokens.IssueAdmin(admin.ID)
	if err != nil {
		return nil, err
	}
	return &AdminSession{AccessToken: token, ExpiresIn: ttl, Admin: admin}, nil
}

// Me returns the signed-in administrator.
func (uc *AdminAuthUsecase) Me(ctx context.Context, id uuid.UUID) (*Admin, error) {
	return uc.admins.Get(ctx, id)
}

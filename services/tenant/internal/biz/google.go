package biz

import (
	"context"

	v1 "common/proto/tenant/v1"

	"github.com/go-kratos/kratos/v3/errors"
)

var (
	ErrGoogleTokenInvalid    = errors.Unauthorized(v1.ErrorReason_GOOGLE_TOKEN_INVALID.String(), "google id token is invalid")
	ErrGoogleEmailUnverified = errors.Forbidden(v1.ErrorReason_GOOGLE_EMAIL_UNVERIFIED.String(), "google account email is not verified")
	ErrGoogleAccountConflict = errors.Conflict(v1.ErrorReason_GOOGLE_ACCOUNT_CONFLICT.String(), "an account with this email cannot be linked to this google account")
)

// GoogleIdentity is what a verified Google ID token says about the person signing in.
type GoogleIdentity struct {
	// Subject is the stable, never reused Google account id; the only safe identifier.
	Subject       string
	Email         string
	EmailVerified bool
	// EmailAuthoritative is true when Google owns the address (Gmail or a Workspace domain),
	// which is required before an existing account is linked by email.
	EmailAuthoritative bool
	Name               string
}

// GoogleVerifier checks Google ID tokens.
type GoogleVerifier interface {
	// Verify returns ErrGoogleTokenInvalid unless the token is genuine, meant for this app,
	// unexpired and carries the given nonce.
	Verify(ctx context.Context, idToken, nonce string) (*GoogleIdentity, error)
}

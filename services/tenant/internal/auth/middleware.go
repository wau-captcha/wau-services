package auth

import (
	"context"
	"strings"

	"tenant/internal/biz"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/google/uuid"
)

type (
	userIDKey  struct{}
	adminIDKey struct{}
)

// Accounts tells what the account a token names is now.
type Accounts interface {
	Status(ctx context.Context, id uuid.UUID) (biz.AccountStatus, error)
}

// Admins looks up the administrator a token names.
type Admins interface {
	// Get returns ErrUnauthenticated for an administrator that was removed.
	Get(ctx context.Context, id uuid.UUID) (*biz.Admin, error)
}

// Middleware rejects requests without a valid bearer token and carries the user id to
// the service layer. Tokens cannot be revoked, so the account is looked up on every
// request: a deleted account gets UNAUTHENTICATED and a banned one ACCOUNT_BANNED at
// once, instead of their tokens working until they expire.
func Middleware(tokens *JWT, accounts Accounts) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			id, err := tokens.Parse(bearerToken(ctx))
			if err != nil {
				return nil, err
			}
			status, err := accounts.Status(ctx, id)
			switch {
			case err != nil:
				return nil, err
			case !status.Exists:
				return nil, biz.ErrUnauthenticated
			case status.Banned:
				return nil, biz.ErrAccountBanned
			}
			return handler(WithUserID(ctx, id), req)
		}
	}
}

// AdminMiddleware guards AdminService the way Middleware guards the user API, with
// administrator tokens only. The administrator is looked up on every request, so removing
// the row from admins ends their sessions at once.
func AdminMiddleware(tokens *JWT, admins Admins) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			id, err := tokens.ParseAdmin(bearerToken(ctx))
			if err != nil {
				return nil, err
			}
			if _, err := admins.Get(ctx, id); err != nil {
				return nil, err
			}
			return handler(context.WithValue(ctx, adminIDKey{}, id), req)
		}
	}
}

// WithUserID stores the authenticated user in the context.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

// UserID returns the user put in the context by Middleware.
func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return id, ok
}

// AdminID returns the administrator put in the context by AdminMiddleware.
func AdminID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(adminIDKey{}).(uuid.UUID)
	return id, ok
}

// bearerToken extracts the credential of an "Authorization: Bearer" header.
func bearerToken(ctx context.Context) string {
	serverTransport, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	scheme, credential, _ := strings.Cut(serverTransport.RequestHeader().Get("Authorization"), " ")
	if !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return credential
}

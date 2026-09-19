package auth

import (
	"context"
	"strings"

	"tenant/internal/biz"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/google/uuid"
)

type userIDKey struct{}

// Accounts tells whether the user a token names still exists.
type Accounts interface {
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// Middleware rejects requests without a valid bearer token and carries the user
// id to the service layer. Tokens cannot be revoked, so a token whose account was
// deleted is rejected here; otherwise it would keep working until it expires.
func Middleware(j *JWT, accounts Accounts) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			id, err := j.Parse(bearerToken(ctx))
			if err != nil {
				return nil, err
			}
			exists, err := accounts.Exists(ctx, id)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, biz.ErrUnauthenticated
			}
			return handler(WithUserID(ctx, id), req)
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

// bearerToken extracts the credential of an "Authorization: Bearer" header.
func bearerToken(ctx context.Context) string {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	scheme, credential, _ := strings.Cut(tr.RequestHeader().Get("Authorization"), " ")
	if !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return credential
}

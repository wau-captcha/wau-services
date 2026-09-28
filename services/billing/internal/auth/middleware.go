package auth

import (
	"context"
	"strings"

	"billing/internal/biz"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/google/uuid"
	"github.com/google/wire"
)

// ProviderSet is auth providers.
var ProviderSet = wire.NewSet(NewTokens)

type userIDKey struct{}

// Middleware rejects requests without a valid user token and carries the user id to the
// service layer. As in wau-tenant, the account is looked up on every request: a deleted account
// gets UNAUTHENTICATED and a banned one ACCOUNT_BANNED at once.
func Middleware(tokens *Tokens, accounts biz.AccountRepo) middleware.Middleware {
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
			return handler(context.WithValue(ctx, userIDKey{}, id), req)
		}
	}
}

// UserID returns the user put in the context by Middleware.
func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
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

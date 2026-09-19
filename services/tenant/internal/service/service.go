package service

import (
	"context"

	"tenant/internal/auth"
	"tenant/internal/biz"

	"github.com/google/uuid"
	"github.com/google/wire"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewAuthService, NewApiKeyService, NewUsageService, NewCatalogService, NewAdminService)

// currentUser reads the caller put in the context by the auth middleware.
func currentUser(ctx context.Context) (uuid.UUID, error) {
	id, ok := auth.UserID(ctx)
	if !ok {
		return uuid.Nil, biz.ErrUnauthenticated
	}
	return id, nil
}

// currentAdmin reads the administrator put in the context by the admin middleware.
func currentAdmin(ctx context.Context) (uuid.UUID, error) {
	id, ok := auth.AdminID(ctx)
	if !ok {
		return uuid.Nil, biz.ErrUnauthenticated
	}
	return id, nil
}

// parseKeyID keeps unparseable ids indistinguishable from keys of other users.
func parseKeyID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, biz.ErrApiKeyNotFound
	}
	return parsed, nil
}

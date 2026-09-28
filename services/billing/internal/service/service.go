package service

import (
	"context"
	"time"

	"billing/internal/auth"
	"billing/internal/biz"

	"github.com/google/uuid"
	"github.com/google/wire"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewBillingService, NewSepayIPN)

// currentUser reads the caller put in the context by the auth middleware.
func currentUser(ctx context.Context) (uuid.UUID, error) {
	id, ok := auth.UserID(ctx)
	if !ok {
		return uuid.Nil, biz.ErrUnauthenticated
	}
	return id, nil
}

// optionalTime leaves a zero time out of the reply.
func optionalTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

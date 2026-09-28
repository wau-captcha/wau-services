package data

import (
	"context"
	"errors"
	"time"

	"billing/internal/biz"
	"billing/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type accountRepo struct {
	data *Data
}

func NewAccountRepo(data *Data) biz.AccountRepo { return &accountRepo{data: data} }

func (r *accountRepo) Status(ctx context.Context, userID uuid.UUID) (biz.AccountStatus, error) {
	banned, err := r.data.query(ctx).GetAccountStatus(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return biz.AccountStatus{}, nil
	}
	if err != nil {
		return biz.AccountStatus{}, err
	}
	return biz.AccountStatus{Exists: true, Banned: banned}, nil
}

func (r *accountRepo) Plan(ctx context.Context, userID uuid.UUID) (biz.AccountPlan, error) {
	row, err := r.data.query(ctx).GetAccountPlan(ctx, userID)
	if err != nil {
		return biz.AccountPlan{}, accountError(err)
	}
	return biz.AccountPlan{TierID: row.TierID, MonthlyPrice: int64(row.MonthlyPrice), ExpiresAt: timeOf(row.TierExpiresAt)}, nil
}

func (r *accountRepo) LockPlan(ctx context.Context, userID uuid.UUID) (biz.AccountPlan, error) {
	row, err := r.data.query(ctx).LockAccountPlan(ctx, userID)
	if err != nil {
		return biz.AccountPlan{}, accountError(err)
	}
	return biz.AccountPlan{TierID: row.TierID, MonthlyPrice: int64(row.MonthlyPrice), ExpiresAt: timeOf(row.TierExpiresAt)}, nil
}

func (r *accountRepo) SetPlan(ctx context.Context, userID uuid.UUID, tierID string, expiresAt time.Time) error {
	return r.data.query(ctx).SetAccountPlan(ctx, db.SetAccountPlanParams{
		UserID:        userID,
		TierID:        tierID,
		TierExpiresAt: timestamp(expiresAt),
	})
}

func (r *accountRepo) EndLapsedPlans(ctx context.Context) (int64, error) {
	return r.data.query(ctx).EndLapsedTiers(ctx)
}

// accountError: the auth middleware already turned away deleted accounts, so a missing one
// was deleted in the meantime.
func accountError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return biz.ErrUnauthenticated
	}
	return err
}

package data

import (
	"context"
	"errors"

	"tenant/internal/biz"
	"tenant/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type adminRepo struct {
	data *Data
}

func NewAdminRepo(data *Data) biz.AdminRepo { return &adminRepo{data: data} }

func (r *adminRepo) Stats(ctx context.Context) (*biz.AdminStats, error) {
	row, err := r.data.queries.GetAdminStats(ctx)
	if err != nil {
		return nil, err
	}
	return &biz.AdminStats{
		Users:            row.Users,
		BannedUsers:      row.BannedUsers,
		ApiKeys:          row.APIKeys,
		SuspendedApiKeys: row.SuspendedAPIKeys,
		ScrapingApiKeys:  row.ScrapingAPIKeys,
	}, nil
}

func (r *adminRepo) ListUsers(ctx context.Context, pattern string, limit, offset int) ([]*biz.AdminUser, int, error) {
	total, err := r.data.queries.CountAdminUsers(ctx, pattern)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.data.queries.ListAdminUsers(ctx, db.ListAdminUsersParams{
		Pattern:    pattern,
		PageLimit:  int64(limit),
		PageOffset: int64(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	users := make([]*biz.AdminUser, 0, len(rows))
	for _, row := range rows {
		users = append(users, toAdminUser(db.GetAdminUserRow(row)))
	}
	return users, int(total), nil
}

func (r *adminRepo) GetUser(ctx context.Context, id uuid.UUID) (*biz.AdminUser, error) {
	row, err := r.data.queries.GetAdminUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrNoSuchUser
	}
	if err != nil {
		return nil, err
	}
	return toAdminUser(row), nil
}

func (r *adminRepo) SetUserBanned(ctx context.Context, id uuid.UUID, banned bool) error {
	affected, err := r.data.queries.SetUserBanned(ctx, db.SetUserBannedParams{Banned: banned, UserID: id})
	if err == nil && affected == 0 {
		return biz.ErrNoSuchUser
	}
	return err
}

func (r *adminRepo) ListApiKeys(ctx context.Context, userID uuid.UUID) ([]*biz.AdminApiKey, error) {
	rows, err := r.data.queries.ListAdminAPIKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	keys := make([]*biz.AdminApiKey, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, toAdminApiKey(db.SetAPIKeySuspendedRow(row)))
	}
	return keys, nil
}

func (r *adminRepo) SetApiKeySuspended(ctx context.Context, id uuid.UUID, suspended bool) (*biz.AdminApiKey, error) {
	row, err := r.data.queries.SetAPIKeySuspended(ctx, db.SetAPIKeySuspendedParams{Suspended: suspended, APIKeyID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrApiKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return toAdminApiKey(row), nil
}

func toAdminUser(row db.GetAdminUserRow) *biz.AdminUser {
	return &biz.AdminUser{
		User: biz.User{
			ID:         row.UserID,
			Username:   row.Username,
			Email:      row.Email,
			TierID:     row.TierID,
			TierName:   row.TierName,
			CreateDate: row.CreateDate.Time,
			DailyQuota: int64(row.DailyQuota),
			IsBanned:   row.IsBanned,
		},
		ApiKeyCount: int(row.APIKeyCount),
	}
}

func toAdminApiKey(row db.SetAPIKeySuspendedRow) *biz.AdminApiKey {
	return &biz.AdminApiKey{
		ID:            row.APIKeyID,
		Name:          row.Name,
		SiteKey:       row.SiteKey,
		IsActive:      row.IsActive,
		IsSuspended:   row.IsSuspended,
		SuspendReason: row.SuspendReason.String,
		CreateDate:    row.CreateDate.Time,
	}
}

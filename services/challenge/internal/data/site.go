package data

import (
	"context"
	"errors"

	"challenge/internal/biz"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type siteRepo struct {
	data *Data
}

func NewSiteRepo(data *Data) biz.SiteRepo {
	return &siteRepo{data: data}
}

func (r *siteRepo) FindBySiteKey(ctx context.Context, siteKey string) (*biz.Site, error) {
	row, err := r.data.queries.GetSiteBySiteKey(ctx, siteKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrSiteKeyInvalid
	}
	if err != nil {
		return nil, err
	}
	return &biz.Site{
		APIKeyID:       row.APIKeyID.String(),
		UserID:         row.UserID.String(),
		ChallengeTypes: row.ChallengeTypes,
		Suspended:      row.IsSuspended,
		OwnerBanned:    row.IsBanned,
		DailyQuota:     int64(row.DailyQuota),
	}, nil
}

func (r *siteRepo) SuspendForScraping(ctx context.Context, apiKeyID string) error {
	id, err := uuid.Parse(apiKeyID)
	if err != nil {
		return err
	}
	return r.data.queries.SuspendAPIKeyForScraping(ctx, id)
}

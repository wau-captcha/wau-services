package data

import (
	"context"
	"errors"

	"verify/internal/biz"
	"verify/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type siteRepo struct {
	data *Data
}

func NewSiteRepo(data *Data) biz.SiteRepo {
	return &siteRepo{data: data}
}

func (r *siteRepo) FindBySecretKey(ctx context.Context, secretKey string) (*biz.Site, error) {
	row, err := r.data.queries.GetSiteBySecretKey(ctx, secretKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrSecretKeyInvalid
	}
	if err != nil {
		return nil, err
	}
	return &biz.Site{APIKeyID: row.APIKeyID.String(), Suspended: row.IsSuspended, OwnerBanned: row.IsBanned}, nil
}

func (r *siteRepo) RecordUsage(ctx context.Context, apiKeyID, challengeType string, success bool) error {
	id, err := uuid.Parse(apiKeyID)
	if err != nil {
		return err
	}
	return r.data.queries.CreateUsageLog(ctx, db.CreateUsageLogParams{
		APIKeyID:        id,
		ChallengeTypeID: challengeType,
		Success:         success,
	})
}

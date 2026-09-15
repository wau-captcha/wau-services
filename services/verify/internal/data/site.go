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

func (r *siteRepo) FindAPIKeyID(ctx context.Context, secretKey string) (string, error) {
	id, err := r.data.queries.GetAPIKeyIDBySecretKey(ctx, secretKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", biz.ErrSecretKeyInvalid
	}
	if err != nil {
		return "", err
	}
	return id.String(), nil
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

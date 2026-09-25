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

func (r *siteRepo) RecordAnswer(ctx context.Context, challengeID string, solution *biz.Solution) error {
	parsedChallengeID, err := uuid.Parse(challengeID)
	if err != nil {
		return err
	}
	apiKeyID, err := uuid.Parse(solution.APIKeyID)
	if err != nil {
		return err
	}
	return r.data.queries.RecordAnsweredUsageLog(ctx, db.RecordAnsweredUsageLogParams{
		ChallengeID:     parsedChallengeID,
		APIKeyID:        apiKeyID,
		ChallengeTypeID: solution.ChallengeType,
		Success:         solution.Success,
	})
}

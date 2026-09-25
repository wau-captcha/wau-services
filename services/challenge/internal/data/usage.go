package data

import (
	"context"

	"challenge/internal/biz"
	"challenge/internal/data/db"

	"github.com/google/uuid"
)

type usageRepo struct {
	data *Data
}

func NewUsageRepo(data *Data) biz.UsageRepo {
	return &usageRepo{data: data}
}

func (r *usageRepo) RecordIssued(ctx context.Context, challenge *biz.Challenge) error {
	challengeID, err := uuid.Parse(challenge.ID)
	if err != nil {
		return err
	}
	apiKeyID, err := uuid.Parse(challenge.APIKeyID)
	if err != nil {
		return err
	}
	return r.data.queries.CreateIssuedUsageLog(ctx, db.CreateIssuedUsageLogParams{
		ChallengeID:     challengeID,
		APIKeyID:        apiKeyID,
		ChallengeTypeID: challenge.Type,
	})
}

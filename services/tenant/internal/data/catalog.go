package data

import (
	"context"

	"tenant/internal/biz"

	"github.com/google/uuid"
)

type catalogRepo struct {
	data *Data
}

func NewCatalogRepo(data *Data) biz.CatalogRepo { return &catalogRepo{data: data} }

func (r *catalogRepo) ListChallengeTypes(ctx context.Context, userID uuid.UUID) ([]biz.ChallengeType, error) {
	rows, err := r.data.queries.ListChallengeTypes(ctx, userID)
	if err != nil {
		return nil, err
	}
	types := make([]biz.ChallengeType, 0, len(rows))
	for _, row := range rows {
		types = append(types, biz.ChallengeType{ID: row.ChallengeTypeID, Code: row.Code})
	}
	return types, nil
}

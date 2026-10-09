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

func (r *catalogRepo) ListPublicPlans(ctx context.Context) ([]biz.PublicPlan, error) {
	rows, err := r.data.queries.ListPublicPlans(ctx)
	if err != nil {
		return nil, err
	}
	plans := make([]biz.PublicPlan, 0, len(rows))
	for _, row := range rows {
		plans = append(plans, biz.PublicPlan{
			ID:             row.TierID,
			Name:           row.TierName,
			Description:    row.Description,
			MonthlyPrice:   int64(row.MonthlyPrice),
			YearlyPrice:    int64(row.YearlyPrice),
			DailyQuota:     int64(row.DailyQuota),
			ChallengeTypes: row.ChallengeTypes,
		})
	}
	return plans, nil
}

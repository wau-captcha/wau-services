package data

import (
	"context"
	"errors"

	"billing/internal/biz"

	"github.com/jackc/pgx/v5"
)

type planRepo struct {
	data *Data
}

func NewPlanRepo(data *Data) biz.PlanRepo { return &planRepo{data: data} }

func (r *planRepo) List(ctx context.Context) ([]*biz.Plan, error) {
	rows, err := r.data.query(ctx).ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	plans := make([]*biz.Plan, 0, len(rows))
	for _, row := range rows {
		plans = append(plans, &biz.Plan{
			TierID:         row.TierID,
			TierName:       row.TierName,
			Description:    row.Description,
			MonthlyPrice:   int64(row.MonthlyPrice),
			YearlyPrice:    int64(row.YearlyPrice),
			DailyQuota:     int64(row.DailyQuota),
			ChallengeTypes: row.ChallengeTypes,
		})
	}
	return plans, nil
}

func (r *planRepo) Get(ctx context.Context, tierID string) (*biz.Plan, error) {
	row, err := r.data.query(ctx).GetPlan(ctx, tierID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	return &biz.Plan{
		TierID:       row.TierID,
		TierName:     row.TierName,
		MonthlyPrice: int64(row.MonthlyPrice),
		YearlyPrice:  int64(row.YearlyPrice),
	}, nil
}

package data

import (
	"context"
	"time"

	"tenant/internal/biz"
	"tenant/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type usageRepo struct {
	data *Data
}

func NewUsageRepo(data *Data) biz.UsageRepo { return &usageRepo{data: data} }

func (r *usageRepo) Daily(ctx context.Context, userID uuid.UUID, apiKeyID uuid.NullUUID, since time.Time, timeZone string) ([]biz.DailyUsage, error) {
	rows, err := r.data.queries.DailyUsage(ctx, db.DailyUsageParams{
		TimeZone: timeZone,
		UserID:   userID,
		Since:    pgtype.Timestamptz{Time: since, Valid: true},
		APIKeyID: apiKeyID,
	})
	if err != nil {
		return nil, err
	}
	days := make([]biz.DailyUsage, 0, len(rows))
	for _, row := range rows {
		days = append(days, biz.DailyUsage{
			Date:       row.Day.Time.Format(time.DateOnly),
			UsageCount: biz.UsageCount{Total: row.Total, Success: row.Success},
		})
	}
	return days, nil
}

func (r *usageRepo) ByChallengeType(ctx context.Context, userID uuid.UUID, apiKeyID uuid.NullUUID, since time.Time) ([]biz.ChallengeTypeUsage, error) {
	rows, err := r.data.queries.ChallengeTypeUsage(ctx, db.ChallengeTypeUsageParams{
		UserID:   userID,
		Since:    pgtype.Timestamptz{Time: since, Valid: true},
		APIKeyID: apiKeyID,
	})
	if err != nil {
		return nil, err
	}
	types := make([]biz.ChallengeTypeUsage, 0, len(rows))
	for _, row := range rows {
		types = append(types, biz.ChallengeTypeUsage{
			ChallengeType: row.ChallengeTypeID,
			UsageCount:    biz.UsageCount{Total: row.Total, Success: row.Success},
		})
	}
	return types, nil
}

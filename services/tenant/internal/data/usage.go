package data

import (
	"context"
	"time"

	"tenant/internal/biz"
	"tenant/internal/data/db"

	"github.com/jackc/pgx/v5/pgtype"
)

type usageRepo struct {
	data *Data
}

func NewUsageRepo(data *Data) biz.UsageRepo { return &usageRepo{data: data} }

func (r *usageRepo) Breakdown(ctx context.Context, filter biz.UsageFilter, since time.Time, timeZone string) ([]biz.UsageRecord, error) {
	// A zero since reads the whole history.
	from := pgtype.Timestamptz{Time: since, Valid: !since.IsZero()}
	if !filter.UserID.Valid && !filter.APIKeyID.Valid {
		return r.system(ctx, from, timeZone)
	}
	rows, err := r.data.queries.UsageBreakdown(ctx, db.UsageBreakdownParams{
		TimeZone: timeZone,
		Since:    from,
		UserID:   filter.UserID,
		APIKeyID: filter.APIKeyID,
	})
	if err != nil {
		return nil, err
	}
	records := make([]biz.UsageRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, biz.UsageRecord{
			Date: row.Day.Time.Format(time.DateOnly),
			Key: biz.UsageKey{
				ID:         row.APIKeyID,
				Name:       row.APIKeyName,
				Deleted:    row.APIKeyDeleted,
				CreateDate: row.APIKeyCreateDate.Time,
			},
			ChallengeType: row.ChallengeTypeID,
			UsageCount:    biz.UsageCount{Challenges: row.Challenges, Verifications: row.Verifications, Successes: row.Successes},
		})
	}
	return records, nil
}

func (r *usageRepo) system(ctx context.Context, since pgtype.Timestamptz, timeZone string) ([]biz.UsageRecord, error) {
	rows, err := r.data.queries.SystemUsage(ctx, db.SystemUsageParams{TimeZone: timeZone, Since: since})
	if err != nil {
		return nil, err
	}
	records := make([]biz.UsageRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, biz.UsageRecord{
			Date:          row.Day.Time.Format(time.DateOnly),
			ChallengeType: row.ChallengeTypeID,
			UsageCount:    biz.UsageCount{Challenges: row.Challenges, Verifications: row.Verifications, Successes: row.Successes},
		})
	}
	return records, nil
}

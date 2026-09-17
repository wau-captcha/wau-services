package biz

import (
	"context"
	"time"

	"tenant/internal/conf"

	"github.com/google/uuid"
)

const (
	defaultUsageDays = 30
	maxUsageDays     = 90
)

// UsageCount is how many checks ran and how many of them passed.
type UsageCount struct {
	Total   int64
	Success int64
}

// DailyUsage is one local day, formatted as YYYY-MM-DD.
type DailyUsage struct {
	Date string
	UsageCount
}

type ChallengeTypeUsage struct {
	ChallengeType string
	UsageCount
}

// Usage is a window of verification activity.
type Usage struct {
	UsageCount
	Days           []DailyUsage
	ChallengeTypes []ChallengeTypeUsage
}

// UsageRepo reads verification records. A null apiKeyID means every key of the
// user, deleted ones included.
type UsageRepo interface {
	Daily(ctx context.Context, userID uuid.UUID, apiKeyID uuid.NullUUID, since time.Time, timeZone string) ([]DailyUsage, error)
	ByChallengeType(ctx context.Context, userID uuid.UUID, apiKeyID uuid.NullUUID, since time.Time) ([]ChallengeTypeUsage, error)
}

// UsageUsecase reports usage over a window of local days.
type UsageUsecase struct {
	usage    UsageRepo
	keys     ApiKeyRepo
	location *time.Location
	timeZone string
	now      func() time.Time
}

func NewUsageUsecase(usage UsageRepo, keys ApiKeyRepo, c *conf.Usage) (*UsageUsecase, error) {
	zone := c.GetTimeZone()
	if zone == "" {
		zone = "UTC"
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	return &UsageUsecase{usage: usage, keys: keys, location: location, timeZone: zone, now: time.Now}, nil
}

// Get returns the last `days` days, oldest first, with empty days filled in.
func (uc *UsageUsecase) Get(ctx context.Context, userID uuid.UUID, apiKeyID uuid.NullUUID, days int) (*Usage, error) {
	if days == 0 {
		days = defaultUsageDays
	}
	if days < 1 || days > maxUsageDays {
		return nil, InvalidField("days", "days must be between 1 and 90")
	}
	// Reading one key requires owning it; the repo filters by user either way.
	if apiKeyID.Valid {
		if _, err := uc.keys.Get(ctx, userID, apiKeyID.UUID); err != nil {
			return nil, err
		}
	}

	today := uc.now().In(uc.location)
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, uc.location).AddDate(0, 0, -(days - 1))

	daily, err := uc.usage.Daily(ctx, userID, apiKeyID, start, uc.timeZone)
	if err != nil {
		return nil, err
	}
	byType, err := uc.usage.ByChallengeType(ctx, userID, apiKeyID, start)
	if err != nil {
		return nil, err
	}

	counts := make(map[string]UsageCount, len(daily))
	for _, day := range daily {
		counts[day.Date] = day.UsageCount
	}
	usage := &Usage{Days: make([]DailyUsage, 0, days), ChallengeTypes: byType}
	for i := range days {
		date := start.AddDate(0, 0, i).Format(time.DateOnly)
		count := counts[date]
		usage.Days = append(usage.Days, DailyUsage{Date: date, UsageCount: count})
		usage.Total += count.Total
		usage.Success += count.Success
	}
	return usage, nil
}

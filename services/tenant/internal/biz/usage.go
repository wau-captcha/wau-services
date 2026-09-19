package biz

import (
	"cmp"
	"context"
	"slices"
	"time"

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

func (c *UsageCount) add(other UsageCount) {
	c.Total += other.Total
	c.Success += other.Success
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

// UsageSeries is the usage of one API key or one challenge type, day by day.
type UsageSeries struct {
	ID string
	UsageCount
	// The days of the window, oldest first; empty days are zero.
	Days []DailyUsage
}

// Usage is a window of verification activity.
type Usage struct {
	UsageCount
	Days           []DailyUsage
	ChallengeTypes []ChallengeTypeUsage
	// Busiest first. ByApiKey is empty for the whole system.
	ByApiKey        []UsageSeries
	ByChallengeType []UsageSeries
}

// UsageFilter narrows usage to one account and/or one key; invalid means "any".
type UsageFilter struct {
	UserID   uuid.NullUUID
	APIKeyID uuid.NullUUID
}

// UsageRecord is the verifications of one local day, API key and challenge type.
type UsageRecord struct {
	Date string
	// uuid.Nil when usage of the whole system is not split by key.
	APIKeyID      uuid.UUID
	ChallengeType string
	UsageCount
}

// UsageRepo reads verification records since a moment, grouped in a time zone. Deleted
// keys are included. With neither a user nor a key the filter covers the whole system,
// which is not split by key.
type UsageRepo interface {
	Breakdown(ctx context.Context, filter UsageFilter, since time.Time, timeZone string) ([]UsageRecord, error)
}

// UsageUsecase reports usage over a window of local days.
type UsageUsecase struct {
	usage    UsageRepo
	keys     ApiKeyRepo
	calendar *Calendar
}

func NewUsageUsecase(usage UsageRepo, keys ApiKeyRepo, calendar *Calendar) *UsageUsecase {
	return &UsageUsecase{usage: usage, keys: keys, calendar: calendar}
}

// Get returns the last `days` days of the user's keys, or of one of them.
func (uc *UsageUsecase) Get(ctx context.Context, userID uuid.UUID, apiKeyID uuid.NullUUID, days int) (*Usage, error) {
	days, err := usageDays(days)
	if err != nil {
		return nil, err
	}
	// Reading one key requires owning it; the repo filters by user either way.
	if apiKeyID.Valid {
		if _, err := uc.keys.Get(ctx, userID, apiKeyID.UUID); err != nil {
			return nil, err
		}
	}
	return uc.report(ctx, UsageFilter{UserID: uuid.NullUUID{UUID: userID, Valid: true}, APIKeyID: apiKeyID}, days)
}

// ForAnyone returns usage of any account or key, for administrators.
func (uc *UsageUsecase) ForAnyone(ctx context.Context, filter UsageFilter, days int) (*Usage, error) {
	days, err := usageDays(days)
	if err != nil {
		return nil, err
	}
	return uc.report(ctx, filter, days)
}

func usageDays(days int) (int, error) {
	if days == 0 {
		return defaultUsageDays, nil
	}
	if days < 1 || days > maxUsageDays {
		return 0, InvalidField("days", "days must be between 1 and 90")
	}
	return days, nil
}

// report fills every day of the window, oldest first, and folds the records into totals
// per day, per challenge type and per key.
func (uc *UsageUsecase) report(ctx context.Context, filter UsageFilter, days int) (*Usage, error) {
	start := uc.calendar.WindowStart(days)
	records, err := uc.usage.Breakdown(ctx, filter, start, uc.calendar.Zone())
	if err != nil {
		return nil, err
	}

	dates := make([]string, days)
	index := make(map[string]int, days)
	for i := range dates {
		dates[i] = start.AddDate(0, 0, i).Format(time.DateOnly)
		index[dates[i]] = i
	}
	usage := &Usage{Days: emptyDays(dates)}
	byKey := map[string]*UsageSeries{}
	byType := map[string]*UsageSeries{}
	for _, record := range records {
		i, ok := index[record.Date]
		if !ok {
			continue
		}
		usage.add(record.UsageCount)
		usage.Days[i].add(record.UsageCount)
		if record.APIKeyID != uuid.Nil {
			seriesFor(byKey, record.APIKeyID.String(), dates).addDay(i, record.UsageCount)
		}
		seriesFor(byType, record.ChallengeType, dates).addDay(i, record.UsageCount)
	}

	usage.ByApiKey = busiestFirst(byKey)
	usage.ByChallengeType = busiestFirst(byType)
	for _, series := range usage.ByChallengeType {
		usage.ChallengeTypes = append(usage.ChallengeTypes, ChallengeTypeUsage{ChallengeType: series.ID, UsageCount: series.UsageCount})
	}
	return usage, nil
}

func emptyDays(dates []string) []DailyUsage {
	days := make([]DailyUsage, len(dates))
	for i, date := range dates {
		days[i].Date = date
	}
	return days
}

func seriesFor(all map[string]*UsageSeries, id string, dates []string) *UsageSeries {
	series, ok := all[id]
	if !ok {
		series = &UsageSeries{ID: id, Days: emptyDays(dates)}
		all[id] = series
	}
	return series
}

func (s *UsageSeries) addDay(i int, count UsageCount) {
	s.add(count)
	s.Days[i].add(count)
}

func busiestFirst(all map[string]*UsageSeries) []UsageSeries {
	list := make([]UsageSeries, 0, len(all))
	for _, series := range all {
		list = append(list, *series)
	}
	slices.SortFunc(list, func(a, b UsageSeries) int {
		return cmp.Or(cmp.Compare(b.Total, a.Total), cmp.Compare(a.ID, b.ID))
	})
	return list
}

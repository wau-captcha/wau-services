package biz

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	defaultUsageDays = 30
	maxUsageDays     = 90
	// All time covers at least this many days, so a chart always has a line to draw.
	minAllTimeDays = 7
)

// UsageWindow is the stretch of local days a report covers: the last Days days, or every day
// since the first recorded challenge (at least the last minAllTimeDays).
type UsageWindow struct {
	Days    int
	AllTime bool
}

// UsageCount is what became of a set of challenges: issued (/challenge/new), answered
// (/challenge/verify) and answered right. Only right answers count as usage.
type UsageCount struct {
	Challenges    int64
	Verifications int64
	Successes     int64
}

func (c *UsageCount) addCount(other UsageCount) {
	c.Challenges += other.Challenges
	c.Verifications += other.Verifications
	c.Successes += other.Successes
}

// DailyUsage is one local day, formatted as YYYY-MM-DD.
type DailyUsage struct {
	Date string
	UsageCount
}

// UsageKey is the API key behind usage: its name, whether it was deleted and when it was created.
type UsageKey struct {
	ID         uuid.UUID
	Name       string
	Deleted    bool
	CreateDate time.Time
}

// UsageSeries is the usage of one API key or one challenge type, day by day.
type UsageSeries struct {
	ID string
	// The API key of a series by key; zero for a series by challenge type.
	Key UsageKey
	UsageCount
	// The days of the window, oldest first; empty days are zero.
	Days []DailyUsage
}

// Usage is a window of challenge activity.
type Usage struct {
	UsageCount
	Days []DailyUsage
	// Busiest first. ByApiKey is empty for the whole system.
	ByApiKey        []UsageSeries
	ByChallengeType []UsageSeries
}

// UsageFilter narrows usage to one account and/or one key; invalid means "any".
type UsageFilter struct {
	UserID   uuid.NullUUID
	APIKeyID uuid.NullUUID
}

// UsageRecord is the challenges of one local day, API key and challenge type.
type UsageRecord struct {
	Date string
	// Zero when usage of the whole system is not split by key.
	Key           UsageKey
	ChallengeType string
	UsageCount
}

// UsageRepo reads the usage history since a moment, all of it when since is zero, grouped in a
// time zone. Deleted keys are included. With neither a user nor a key the filter covers the
// whole system, which is not split by key.
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

// Get returns the usage of the user's keys, or of one of them. Only the user's own keys are in
// it, deleted ones included.
func (uc *UsageUsecase) Get(ctx context.Context, userID uuid.UUID, apiKeyID uuid.NullUUID, window UsageWindow) (*Usage, error) {
	window, err := checkWindow(window)
	if err != nil {
		return nil, err
	}
	// Reading one key requires owning it; the repo filters by user either way.
	if apiKeyID.Valid {
		if _, err := uc.keys.Get(ctx, userID, apiKeyID.UUID); err != nil {
			return nil, err
		}
	}
	return uc.report(ctx, UsageFilter{UserID: uuid.NullUUID{UUID: userID, Valid: true}, APIKeyID: apiKeyID}, window)
}

// ForAnyone returns usage of any account or key, for administrators.
func (uc *UsageUsecase) ForAnyone(ctx context.Context, filter UsageFilter, window UsageWindow) (*Usage, error) {
	window, err := checkWindow(window)
	if err != nil {
		return nil, err
	}
	return uc.report(ctx, filter, window)
}

func checkWindow(window UsageWindow) (UsageWindow, error) {
	switch {
	case window.AllTime:
		return window, nil
	case window.Days == 0:
		return UsageWindow{Days: defaultUsageDays}, nil
	case window.Days < 1 || window.Days > maxUsageDays:
		return window, InvalidField("days", "days must be between 1 and 90")
	}
	return window, nil
}

// report fills every day of the window, oldest first, and folds the records into totals
// per day, per challenge type and per key. All time starts on the day of the first record, or
// minAllTimeDays ago when that is later.
func (uc *UsageUsecase) report(ctx context.Context, filter UsageFilter, window UsageWindow) (*Usage, error) {
	var since time.Time
	if !window.AllTime {
		since = uc.calendar.WindowStart(window.Days)
	}
	records, err := uc.usage.Breakdown(ctx, filter, since, uc.calendar.Zone())
	if err != nil {
		return nil, err
	}

	first := uc.calendar.WindowStart(minAllTimeDays).Format(time.DateOnly)
	if !window.AllTime {
		first = since.Format(time.DateOnly)
	}
	for _, record := range records {
		if window.AllTime && record.Date < first {
			first = record.Date
		}
	}
	dates := uc.calendar.DatesFrom(first)
	index := make(map[string]int, len(dates))
	for i, date := range dates {
		index[date] = i
	}

	usage := &Usage{Days: emptyDays(dates)}
	byKey := map[string]*UsageSeries{}
	byType := map[string]*UsageSeries{}
	for _, record := range records {
		i, ok := index[record.Date]
		if !ok {
			continue
		}
		usage.addCount(record.UsageCount)
		usage.Days[i].addCount(record.UsageCount)
		if record.Key.ID != uuid.Nil {
			series := seriesFor(byKey, record.Key.ID.String(), dates)
			series.Key = record.Key
			series.addDay(i, record.UsageCount)
		}
		seriesFor(byType, record.ChallengeType, dates).addDay(i, record.UsageCount)
	}
	usage.ByApiKey = busiestFirst(byKey)
	usage.ByChallengeType = busiestFirst(byType)
	return usage, nil
}

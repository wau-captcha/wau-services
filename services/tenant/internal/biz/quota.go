package biz

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Quota is how much of the daily verification quota a user has used.
type Quota struct {
	Used  int64
	Limit int64
	// Local date being counted, YYYY-MM-DD.
	Date     string
	ResetsAt time.Time
}

// ScrapingCount is what the scraping check of a key has seen today: challenges issued, answers
// checked, right answers and answers judged automated.
type ScrapingCount struct {
	Challenges    int64
	Verifications int64
	Solved        int64
	Automated     int64
}

// CounterRepo reads the daily counters the challenge and verify services keep in Redis
// (see internal/pkg/quota). Counters that do not exist yet read as zero.
type CounterRepo interface {
	// QuotaUsed returns the successful verifications of each user on day, in order.
	QuotaUsed(ctx context.Context, userIDs []uuid.UUID, day string) ([]int64, error)
	// ScrapingCounts returns the scraping counters of each key on day, in order.
	ScrapingCounts(ctx context.Context, apiKeyIDs []uuid.UUID, day string) ([]ScrapingCount, error)
	// ResetScraping clears the scraping counters of a key for day.
	ResetScraping(ctx context.Context, apiKeyID uuid.UUID, day string) error
}

// QuotaUsecase reports the daily verification quota of the signed-in user.
type QuotaUsecase struct {
	users    UserRepo
	counters CounterRepo
	calendar *Calendar
}

func NewQuotaUsecase(users UserRepo, counters CounterRepo, calendar *Calendar) *QuotaUsecase {
	return &QuotaUsecase{users: users, counters: counters, calendar: calendar}
}

func (uc *QuotaUsecase) Get(ctx context.Context, userID uuid.UUID) (*Quota, error) {
	user, err := uc.users.Profile(ctx, userID)
	if err != nil {
		return nil, err
	}
	day := uc.calendar.Today()
	used, err := uc.counters.QuotaUsed(ctx, []uuid.UUID{userID}, day)
	if err != nil {
		return nil, err
	}
	return &Quota{Used: used[0], Limit: user.DailyQuota, Date: day, ResetsAt: uc.calendar.NextReset()}, nil
}

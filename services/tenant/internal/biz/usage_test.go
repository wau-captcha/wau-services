package biz

import (
	"context"
	"testing"
	"time"

	"tenant/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type fakeUsage struct {
	daily    []DailyUsage
	byType   []ChallengeTypeUsage
	since    time.Time
	timeZone string
	apiKeyID uuid.NullUUID
}

func (f *fakeUsage) Daily(_ context.Context, _ uuid.UUID, apiKeyID uuid.NullUUID, since time.Time, timeZone string) ([]DailyUsage, error) {
	f.since, f.timeZone, f.apiKeyID = since, timeZone, apiKeyID
	return f.daily, nil
}

func (f *fakeUsage) ByChallengeType(_ context.Context, _ uuid.UUID, _ uuid.NullUUID, _ time.Time) ([]ChallengeTypeUsage, error) {
	return f.byType, nil
}

// fixedNow is 15/09/2026 02:00 in Ho Chi Minh time, i.e. the previous day in UTC.
var fixedNow = time.Date(2026, 9, 14, 19, 0, 0, 0, time.UTC)

func newUsageFixture(t *testing.T) (*UsageUsecase, *fakeUsage, *fakeKeys) {
	t.Helper()
	usage := &fakeUsage{}
	keys := &fakeKeys{key: &ApiKey{ID: uuid.New()}}
	uc, err := NewUsageUsecase(usage, keys, &conf.Usage{TimeZone: "Asia/Ho_Chi_Minh"})
	if err != nil {
		t.Fatalf("NewUsageUsecase: %v", err)
	}
	uc.now = func() time.Time { return fixedNow }
	return uc, usage, keys
}

func TestUsageDefaultsAndValidatesDays(t *testing.T) {
	uc, _, _ := newUsageFixture(t)

	usage, err := uc.Get(context.Background(), uuid.New(), uuid.NullUUID{}, 0)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(usage.Days) != 30 {
		t.Fatalf("days = %d, want 30", len(usage.Days))
	}

	for _, days := range []int{-1, 91} {
		if _, err := uc.Get(context.Background(), uuid.New(), uuid.NullUUID{}, days); !errors.Is(err, ErrInvalidArgument) || fieldOf(err) != "days" {
			t.Fatalf("days %d: err = %v (field %q), want invalid days", days, err, fieldOf(err))
		}
	}
}

func TestUsageFillsMissingDays(t *testing.T) {
	uc, usage, _ := newUsageFixture(t)
	usage.daily = []DailyUsage{
		{Date: "2026-09-10", UsageCount: UsageCount{Total: 4, Success: 3}},
		{Date: "2026-09-15", UsageCount: UsageCount{Total: 2, Success: 1}},
	}

	got, err := uc.Get(context.Background(), uuid.New(), uuid.NullUUID{}, 7)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Days) != 7 {
		t.Fatalf("days = %d, want 7", len(got.Days))
	}
	want := []string{"2026-09-09", "2026-09-10", "2026-09-11", "2026-09-12", "2026-09-13", "2026-09-14", "2026-09-15"}
	for i, date := range want {
		if got.Days[i].Date != date {
			t.Fatalf("day %d = %q, want %q", i, got.Days[i].Date, date)
		}
	}
	if got.Days[0].Total != 0 || got.Days[1].Total != 4 || got.Days[6].Success != 1 {
		t.Fatalf("filled days = %+v", got.Days)
	}
	if got.Total != 6 || got.Success != 4 {
		t.Fatalf("totals = %d/%d, want 6/4", got.Total, got.Success)
	}
}

func TestUsageWindowStartsAtLocalMidnight(t *testing.T) {
	uc, usage, _ := newUsageFixture(t)

	if _, err := uc.Get(context.Background(), uuid.New(), uuid.NullUUID{}, 7); err != nil {
		t.Fatalf("Get: %v", err)
	}
	location, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	want := time.Date(2026, 9, 9, 0, 0, 0, 0, location)
	if !usage.since.Equal(want) {
		t.Fatalf("since = %s, want %s", usage.since, want)
	}
	if usage.timeZone != "Asia/Ho_Chi_Minh" {
		t.Fatalf("time zone = %q", usage.timeZone)
	}
}

func TestUsageChecksKeyOwnership(t *testing.T) {
	uc, usage, keys := newUsageFixture(t)
	owned := uuid.NullUUID{UUID: keys.key.ID, Valid: true}

	if _, err := uc.Get(context.Background(), uuid.New(), owned, 7); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if usage.apiKeyID != owned {
		t.Fatalf("api key filter = %+v, want %+v", usage.apiKeyID, owned)
	}

	foreign := uuid.NullUUID{UUID: uuid.New(), Valid: true}
	if _, err := uc.Get(context.Background(), uuid.New(), foreign, 7); !errors.Is(err, ErrApiKeyNotFound) {
		t.Fatalf("err = %v, want ErrApiKeyNotFound", err)
	}
}

func TestUsageRejectsUnknownTimeZone(t *testing.T) {
	if _, err := NewUsageUsecase(&fakeUsage{}, &fakeKeys{}, &conf.Usage{TimeZone: "Mars/Olympus"}); err == nil {
		t.Fatal("want error for an unknown time zone")
	}
}

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
	records  []UsageRecord
	filter   UsageFilter
	since    time.Time
	timeZone string
}

func (f *fakeUsage) Breakdown(_ context.Context, filter UsageFilter, since time.Time, timeZone string) ([]UsageRecord, error) {
	f.filter, f.since, f.timeZone = filter, since, timeZone
	return f.records, nil
}

// fixedNow is 15/09/2026 02:00 in Ho Chi Minh time, i.e. the previous day in UTC.
var fixedNow = time.Date(2026, 9, 14, 19, 0, 0, 0, time.UTC)

func newTestCalendar(t *testing.T) *Calendar {
	t.Helper()
	calendar, err := NewCalendar(&conf.Usage{TimeZone: "Asia/Ho_Chi_Minh"})
	if err != nil {
		t.Fatalf("NewCalendar: %v", err)
	}
	calendar.now = func() time.Time { return fixedNow }
	return calendar
}

func newUsageFixture(t *testing.T) (*UsageUsecase, *fakeUsage, *fakeKeys) {
	t.Helper()
	usage := &fakeUsage{}
	keys := &fakeKeys{key: &ApiKey{ID: uuid.New()}}
	return NewUsageUsecase(usage, keys, newTestCalendar(t)), usage, keys
}

func record(date string, key uuid.UUID, challengeType string, total, success int64) UsageRecord {
	return UsageRecord{Date: date, APIKeyID: key, ChallengeType: challengeType, UsageCount: UsageCount{Total: total, Success: success}}
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
		if _, err := uc.ForAnyone(context.Background(), UsageFilter{}, days); fieldOf(err) != "days" {
			t.Fatalf("ForAnyone days %d: err = %v, want invalid days", days, err)
		}
	}
}

func TestUsageFillsMissingDays(t *testing.T) {
	uc, usage, _ := newUsageFixture(t)
	key := uuid.New()
	usage.records = []UsageRecord{
		record("2026-09-10", key, "image_distort", 4, 3),
		record("2026-09-15", key, "image_distort", 2, 1),
		// Outside the window: ignored rather than misplaced.
		record("2026-09-01", key, "image_distort", 9, 9),
	}

	got, err := uc.Get(context.Background(), uuid.New(), uuid.NullUUID{}, 7)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := []string{"2026-09-09", "2026-09-10", "2026-09-11", "2026-09-12", "2026-09-13", "2026-09-14", "2026-09-15"}
	if len(got.Days) != len(want) {
		t.Fatalf("days = %d, want 7", len(got.Days))
	}
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
	owner := uuid.New()
	owned := uuid.NullUUID{UUID: keys.key.ID, Valid: true}

	if _, err := uc.Get(context.Background(), owner, owned, 7); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if usage.filter.APIKeyID != owned || usage.filter.UserID != (uuid.NullUUID{UUID: owner, Valid: true}) {
		t.Fatalf("filter = %+v, want the owner and the key", usage.filter)
	}

	foreign := uuid.NullUUID{UUID: uuid.New(), Valid: true}
	if _, err := uc.Get(context.Background(), owner, foreign, 7); !errors.Is(err, ErrApiKeyNotFound) {
		t.Fatalf("err = %v, want ErrApiKeyNotFound", err)
	}
}

func TestUsageSeriesAreZeroFilledAndBusiestFirst(t *testing.T) {
	uc, usage, _ := newUsageFixture(t)
	quiet, busy := uuid.New(), uuid.New()
	usage.records = []UsageRecord{
		record("2026-09-13", quiet, "image_distort", 3, 2),
		record("2026-09-14", quiet, "motion_noise", 1, 1),
		record("2026-09-14", busy, "image_distort", 5, 4),
	}

	got, err := uc.Get(context.Background(), uuid.New(), uuid.NullUUID{}, 3)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.ByApiKey) != 2 || got.ByApiKey[0].ID != busy.String() || got.ByApiKey[1].ID != quiet.String() {
		t.Fatalf("by key = %+v, want busy then quiet", got.ByApiKey)
	}
	if b := got.ByApiKey[0]; b.Total != 5 || len(b.Days) != 3 || b.Days[0].Total != 0 || b.Days[1].Total != 5 {
		t.Fatalf("busy series = %+v", b)
	}
	if q := got.ByApiKey[1]; q.Total != 4 || q.Days[0].Total != 3 || q.Days[1].Total != 1 || q.Days[2].Total != 0 {
		t.Fatalf("quiet series = %+v", q)
	}
	if len(got.ByChallengeType) != 2 || got.ByChallengeType[0].ID != "image_distort" || got.ByChallengeType[0].Total != 8 {
		t.Fatalf("by type = %+v", got.ByChallengeType)
	}
	if len(got.ChallengeTypes) != 2 || got.ChallengeTypes[1].ChallengeType != "motion_noise" || got.ChallengeTypes[1].Success != 1 {
		t.Fatalf("challenge types = %+v", got.ChallengeTypes)
	}
}

func TestSystemWideUsageIsNotSplitByKey(t *testing.T) {
	uc, usage, _ := newUsageFixture(t)
	usage.records = []UsageRecord{record("2026-09-15", uuid.Nil, "image_distort", 7, 5)}

	got, err := uc.ForAnyone(context.Background(), UsageFilter{}, 7)
	if err != nil {
		t.Fatalf("ForAnyone: %v", err)
	}
	if usage.filter != (UsageFilter{}) {
		t.Fatalf("filter = %+v, want none", usage.filter)
	}
	if len(got.ByApiKey) != 0 || len(got.ByChallengeType) != 1 || got.Total != 7 {
		t.Fatalf("usage = %+v", got)
	}
}

func TestCalendarRejectsUnknownTimeZone(t *testing.T) {
	if _, err := NewCalendar(&conf.Usage{TimeZone: "Mars/Olympus"}); err == nil {
		t.Fatal("want error for an unknown time zone")
	}
}

package quota

import (
	"testing"
	"time"
)

func TestDayFollowsTheLocalDate(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	// 17:30 UTC is 00:30 the next day in Ho Chi Minh City (UTC+7).
	instant := time.Date(2026, 9, 18, 17, 30, 0, 0, time.UTC)
	if got := Day(instant, hcm); got != "2026-09-19" {
		t.Fatalf("Day = %s, want 2026-09-19", got)
	}
	if got := Day(instant, time.UTC); got != "2026-09-18" {
		t.Fatalf("Day in UTC = %s, want 2026-09-18", got)
	}
}

func TestNextResetIsTheNextLocalMidnight(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct{ at, want time.Time }{
		"midday":         {time.Date(2026, 9, 19, 12, 0, 0, 0, hcm), time.Date(2026, 9, 20, 0, 0, 0, 0, hcm)},
		"exact midnight": {time.Date(2026, 9, 19, 0, 0, 0, 0, hcm), time.Date(2026, 9, 20, 0, 0, 0, 0, hcm)},
		"end of month":   {time.Date(2026, 9, 30, 23, 59, 0, 0, hcm), time.Date(2026, 10, 1, 0, 0, 0, 0, hcm)},
		"end of year":    {time.Date(2026, 12, 31, 8, 0, 0, 0, hcm), time.Date(2027, 1, 1, 0, 0, 0, 0, hcm)},
	}
	for name, tt := range tests {
		if got := NextReset(tt.at, hcm); !got.Equal(tt.want) {
			t.Errorf("%s: NextReset = %v, want %v", name, got, tt.want)
		}
	}
}

func TestKeys(t *testing.T) {
	tests := map[string]string{
		UsedKey("u1", "2026-09-19"):          "quota:daily:u1:2026-09-19",
		ChallengesKey("k1", "2026-09-19"):    "abuse:new:k1:2026-09-19",
		VerificationsKey("k1", "2026-09-19"): "abuse:verify:k1:2026-09-19",
	}
	for got, want := range tests {
		if got != want {
			t.Errorf("key = %s, want %s", got, want)
		}
	}
}

func TestTTLOutlivesTheDay(t *testing.T) {
	if TTL < 24*time.Hour {
		t.Fatalf("TTL %v would expire a counter created at midnight before its day ends", TTL)
	}
}

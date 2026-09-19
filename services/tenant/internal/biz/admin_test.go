package biz

import (
	"context"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type fakeAdmin struct {
	users   map[uuid.UUID]*AdminUser
	keys    map[uuid.UUID]*AdminApiKey
	pattern string
	limit   int
	offset  int
}

func (f *fakeAdmin) Stats(context.Context) (*AdminStats, error) {
	return &AdminStats{Users: int64(len(f.users))}, nil
}

func (f *fakeAdmin) ListUsers(_ context.Context, pattern string, limit, offset int) ([]*AdminUser, int, error) {
	f.pattern, f.limit, f.offset = pattern, limit, offset
	var users []*AdminUser
	for _, user := range f.users {
		copied := *user
		users = append(users, &copied)
	}
	return users, len(users), nil
}

func (f *fakeAdmin) GetUser(_ context.Context, id uuid.UUID) (*AdminUser, error) {
	user, ok := f.users[id]
	if !ok {
		return nil, ErrNoSuchUser
	}
	copied := *user
	return &copied, nil
}

func (f *fakeAdmin) SetUserBanned(_ context.Context, id uuid.UUID, banned bool) error {
	user, ok := f.users[id]
	if !ok {
		return ErrNoSuchUser
	}
	user.IsBanned = banned
	return nil
}

func (f *fakeAdmin) ListApiKeys(_ context.Context, userID uuid.UUID) ([]*AdminApiKey, error) {
	var keys []*AdminApiKey
	for _, key := range f.keys {
		copied := *key
		keys = append(keys, &copied)
	}
	return keys, nil
}

func (f *fakeAdmin) SetApiKeySuspended(_ context.Context, id uuid.UUID, suspended bool) (*AdminApiKey, error) {
	key, ok := f.keys[id]
	if !ok {
		return nil, ErrApiKeyNotFound
	}
	key.IsSuspended = suspended
	key.SuspendReason = ""
	if suspended {
		key.SuspendReason = SuspendedByAdmin
	}
	copied := *key
	return &copied, nil
}

type fakeCounters struct {
	used    map[uuid.UUID]int64
	counts  map[uuid.UUID]ScrapingCount
	resets  []uuid.UUID
	lastDay string
}

func (f *fakeCounters) QuotaUsed(_ context.Context, ids []uuid.UUID, day string) ([]int64, error) {
	f.lastDay = day
	used := make([]int64, len(ids))
	for i, id := range ids {
		used[i] = f.used[id]
	}
	return used, nil
}

func (f *fakeCounters) ScrapingCounts(_ context.Context, ids []uuid.UUID, day string) ([]ScrapingCount, error) {
	f.lastDay = day
	counts := make([]ScrapingCount, len(ids))
	for i, id := range ids {
		counts[i] = f.counts[id]
	}
	return counts, nil
}

func (f *fakeCounters) ResetScraping(_ context.Context, id uuid.UUID, day string) error {
	f.lastDay = day
	f.resets = append(f.resets, id)
	return nil
}

func newAdminFixture(t *testing.T) (*AdminUsecase, *fakeAdmin, *fakeCounters) {
	t.Helper()
	repo := &fakeAdmin{users: map[uuid.UUID]*AdminUser{}, keys: map[uuid.UUID]*AdminApiKey{}}
	counters := &fakeCounters{used: map[uuid.UUID]int64{}, counts: map[uuid.UUID]ScrapingCount{}}
	calendar := newTestCalendar(t)
	usage := NewUsageUsecase(&fakeUsage{}, &fakeKeys{}, calendar)
	return NewAdminUsecase(repo, counters, usage, calendar), repo, counters
}

func addUser(repo *fakeAdmin) uuid.UUID {
	id := uuid.New()
	repo.users[id] = &AdminUser{User: User{ID: id, DailyQuota: 1000}}
	return id
}

func TestListUsersPagesAndFillsTodaysQuota(t *testing.T) {
	uc, repo, counters := newAdminFixture(t)
	id := addUser(repo)
	counters.used[id] = 321

	page, err := uc.ListUsers(context.Background(), "  50%_off\\  ", 3, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if repo.pattern != `%50\%\_off\\%` {
		t.Fatalf("pattern = %q, want the query taken literally", repo.pattern)
	}
	if repo.limit != 20 || repo.offset != 40 {
		t.Fatalf("limit/offset = %d/%d, want 20/40", repo.limit, repo.offset)
	}
	if page.TotalSize != 1 || page.Users[0].QuotaUsed != 321 {
		t.Fatalf("page = %+v", page.Users[0])
	}
	// fixedNow is already the 15th in Ho Chi Minh City.
	if counters.lastDay != "2026-09-15" {
		t.Fatalf("quota day = %s, want 2026-09-15", counters.lastDay)
	}

	if _, err := uc.ListUsers(context.Background(), "", 0, 0); err != nil || repo.pattern != "" || repo.offset != 0 {
		t.Fatalf("first page: err = %v, pattern %q, offset %d", err, repo.pattern, repo.offset)
	}
}

func TestListUsersValidatesPaging(t *testing.T) {
	uc, _, _ := newAdminFixture(t)
	tests := map[string]struct {
		query          string
		page, pageSize int
		field          string
	}{
		"negative page": {"", -1, 0, "page"},
		"page too big":  {"", 1, 101, "page_size"},
		"long query":    {string(make([]rune, 101)), 1, 10, "query"},
	}
	for name, tt := range tests {
		if _, err := uc.ListUsers(context.Background(), tt.query, tt.page, tt.pageSize); fieldOf(err) != tt.field {
			t.Errorf("%s: err = %v, want field %s", name, err, tt.field)
		}
	}
}

func TestBanAndUnban(t *testing.T) {
	uc, repo, _ := newAdminFixture(t)
	id := addUser(repo)

	banned, err := uc.SetUserBanned(context.Background(), id, true)
	if err != nil || !banned.IsBanned {
		t.Fatalf("ban = %+v, %v", banned, err)
	}
	unbanned, err := uc.SetUserBanned(context.Background(), id, false)
	if err != nil || unbanned.IsBanned {
		t.Fatalf("unban = %+v, %v", unbanned, err)
	}
	if _, err := uc.SetUserBanned(context.Background(), uuid.New(), true); !errors.Is(err, ErrNoSuchUser) {
		t.Fatalf("unknown user: err = %v, want ErrNoSuchUser", err)
	}
}

func TestLiftingASuspensionResetsTodaysScrapingCounters(t *testing.T) {
	uc, repo, counters := newAdminFixture(t)
	key := uuid.New()
	repo.keys[key] = &AdminApiKey{ID: key, IsSuspended: true, SuspendReason: SuspendedForScraping}
	counters.counts[key] = ScrapingCount{Challenges: 250, Verifications: 10}

	suspended, err := uc.SetApiKeySuspended(context.Background(), key, true)
	if err != nil || len(counters.resets) != 0 || suspended.Challenges != 250 {
		t.Fatalf("suspend = %+v, %v, resets %v", suspended, err, counters.resets)
	}
	lifted, err := uc.SetApiKeySuspended(context.Background(), key, false)
	if err != nil || lifted.IsSuspended {
		t.Fatalf("lift = %+v, %v", lifted, err)
	}
	if len(counters.resets) != 1 || counters.resets[0] != key || counters.lastDay != "2026-09-15" {
		t.Fatalf("resets = %v on %s, want the key today", counters.resets, counters.lastDay)
	}
}

func TestListApiKeysOfUnknownUser(t *testing.T) {
	uc, _, _ := newAdminFixture(t)
	if _, err := uc.ListApiKeys(context.Background(), uuid.New()); !errors.Is(err, ErrNoSuchUser) {
		t.Fatalf("err = %v, want ErrNoSuchUser", err)
	}
}

func TestQuotaReportsTodayAndTheNextReset(t *testing.T) {
	existing := &memUser{id: uuid.New(), email: "ann@gmail.com", name: "Ann"}
	users := &fakeUsers{users: []*memUser{existing}}
	counters := &fakeCounters{used: map[uuid.UUID]int64{existing.id: 42}}
	uc := NewQuotaUsecase(users, counters, newTestCalendar(t))

	quota, err := uc.Get(context.Background(), existing.id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	location, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	if quota.Used != 42 || quota.Limit != 1000 || quota.Date != "2026-09-15" ||
		!quota.ResetsAt.Equal(time.Date(2026, 9, 16, 0, 0, 0, 0, location)) {
		t.Fatalf("quota = %+v", quota)
	}
}

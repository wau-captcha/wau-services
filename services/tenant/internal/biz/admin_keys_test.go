package biz

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

var admin = Actor{ActorAdmin, uuid.MustParse("019a0000-0000-7000-8000-0000000000ad")}

// memoryAdmin is the admin view of the keys appeals share.
type memoryAdmin struct {
	AdminRepo
	keys map[uuid.UUID]*AdminApiKey
}

func (m *memoryAdmin) GetApiKey(_ context.Context, id uuid.UUID) (*AdminApiKey, error) {
	key, ok := m.keys[id]
	if !ok {
		return nil, ErrApiKeyNotFound
	}
	copied := *key
	return &copied, nil
}

func (m *memoryAdmin) SetApiKeySuspended(ctx context.Context, id uuid.UUID, suspended bool) (*AdminApiKey, error) {
	key := m.keys[id]
	switch {
	case !suspended:
		key.IsSuspended, key.SuspendReason, key.ScrapingFlaggedAt = false, "", time.Time{}
	case !key.IsSuspended:
		key.IsSuspended, key.SuspendReason = true, SuspendedByAdmin
	}
	return m.GetApiKey(ctx, id)
}

type adminFixture struct {
	usecase  *AdminUsecase
	keys     map[uuid.UUID]*AdminApiKey
	appeals  *memoryAppeals
	events   *memoryEvents
	counters *memoryCounters
}

func newAdminFixture(keys ...*AdminApiKey) *adminFixture {
	f := &adminFixture{keys: map[uuid.UUID]*AdminApiKey{}, events: &memoryEvents{}, counters: &memoryCounters{counts: map[uuid.UUID]ScrapingCount{}}}
	for _, key := range keys {
		f.keys[key.ID] = key
	}
	f.appeals = &memoryAppeals{appeals: map[uuid.UUID]*AdminAppeal{}, keys: f.keys}
	f.usecase = NewAdminUsecase(&memoryAdmin{keys: f.keys}, f.counters, f.events, f.appeals, nil, testCalendar())
	return f
}

func (f *adminFixture) appeal(t *testing.T, key *AdminApiKey) uuid.UUID {
	t.Helper()
	appeal, err := f.appeals.Create(context.Background(), owner, key.ID, "please")
	if err != nil {
		t.Fatal(err)
	}
	return appeal.ID
}

func TestDecideAppeal(t *testing.T) {
	tests := []struct {
		name      string
		suspended bool
		approve   bool
		actions   []string
		reset     bool
		status    string
	}{
		{"approve a suspended key", true, true, []string{EventAppealApproved, EventUnsuspended}, true, AppealApproved},
		{"approve a flagged key", false, true, []string{EventAppealApproved}, true, AppealApproved},
		{"reject", true, false, []string{EventAppealRejected}, false, AppealRejected},
	}
	for _, test := range tests {
		key := &AdminApiKey{ID: uuid.New(), IsSuspended: test.suspended, ScrapingFlaggedAt: testNow}
		if test.suspended {
			key.SuspendReason = SuspendedForScraping
		}
		f := newAdminFixture(key)
		f.counters.counts[key.ID] = ScrapingCount{Challenges: 6000, Verifications: 10}
		id := f.appeal(t, key)
		decided, err := f.usecase.DecideAppeal(context.Background(), admin.ID, id, test.approve, "  Looks like a prefetching app.  ")
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		lifted := !decided.ApiKey.IsSuspended && !decided.ApiKey.ScrapingFlagged
		if decided.Status != test.status || decided.AdminNote != "Looks like a prefetching app." || lifted != test.approve ||
			!slices.Equal(f.events.actions(), test.actions) || (len(f.counters.resets) == 1) != test.reset {
			t.Errorf("%s: got %s %q, key %+v, events %q, resets %d", test.name, decided.Status, decided.AdminNote, decided.ApiKey,
				f.events.actions(), len(f.counters.resets))
		}
		if f.events.recorded[0].actor != admin || f.events.recorded[0].detail["appeal_id"] != id.String() {
			t.Errorf("%s: event %+v", test.name, f.events.recorded[0])
		}
		if _, err := f.usecase.DecideAppeal(context.Background(), admin.ID, id, true, ""); !errors.Is(err, ErrAppealNotFound) {
			t.Errorf("%s: deciding twice: %v", test.name, err)
		}
	}
}

func TestListAppeals(t *testing.T) {
	key := &AdminApiKey{ID: uuid.New(), IsSuspended: true, ScrapingFlaggedAt: testNow}
	f := newAdminFixture(key)
	f.counters.counts[key.ID] = ScrapingCount{Challenges: 6000, Verifications: 10, Solved: 4, Automated: 1}
	f.appeal(t, key)
	appeals, err := f.usecase.ListAppeals(context.Background(), "")
	if err != nil || len(appeals) != 1 || appeals[0].ApiKey.Challenges != 6000 || !appeals[0].ApiKey.ScrapingFlagged {
		t.Fatalf("got %+v, %v", appeals, err)
	}
	if _, err := f.usecase.ListAppeals(context.Background(), "open"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("unknown status: %v", err)
	}
}

func TestSetApiKeySuspendedEvents(t *testing.T) {
	key := &AdminApiKey{ID: uuid.New()}
	f := newAdminFixture(key)
	ctx := context.Background()
	for _, suspended := range []bool{true, true, false, false} {
		if _, err := f.usecase.SetApiKeySuspended(ctx, admin.ID, key.ID, suspended); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.events.actions(); !slices.Equal(got, []string{EventSuspended, EventUnsuspended}) {
		t.Fatalf("events %q", got)
	}
	if f.events.recorded[0].detail["reason"] != SuspendedByAdmin || len(f.counters.resets) != 2 {
		t.Fatalf("detail %v, %d resets", f.events.recorded[0].detail, len(f.counters.resets))
	}
}

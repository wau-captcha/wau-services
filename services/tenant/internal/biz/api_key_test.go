package biz

import (
	"context"
	stderrors "errors"
	"slices"
	"strings"
	"testing"
	"time"

	"tenant/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

var owner = uuid.MustParse("019a0000-0000-7000-8000-000000000001")

type keyFixture struct {
	usecase  *ApiKeyUsecase
	keys     *memoryKeys
	events   *memoryEvents
	appeals  *memoryAppeals
	counters *memoryCounters
}

func newKeyFixture(keys ...*ApiKey) *keyFixture {
	f := &keyFixture{
		keys:     newMemoryKeys(keys...),
		events:   &memoryEvents{},
		appeals:  &memoryAppeals{appeals: map[uuid.UUID]*AdminAppeal{}, keys: map[uuid.UUID]*AdminApiKey{}},
		counters: &memoryCounters{counts: map[uuid.UUID]ScrapingCount{}},
	}
	f.usecase = NewApiKeyUsecase(f.keys, f.events, f.appeals, f.counters, testCalendar(), &conf.ApiKey{})
	return f
}

func shopKey() *ApiKey {
	return &ApiKey{ID: uuid.New(), Name: "Shop", SiteKey: "pk_shop", SecretKey: "sk_shop", IsActive: true}
}

func TestCreateWithDomains(t *testing.T) {
	f := newKeyFixture()
	key, err := f.usecase.Create(context.Background(), owner, " Shop ", []string{"Shop.example.com", "*.example.org"})
	if err != nil {
		t.Fatal(err)
	}
	if len(key.AllowedDomains) != 2 || key.AllowedDomains[0].Domain != "shop.example.com" || !key.RequireBehavior {
		t.Fatalf("created %+v", key)
	}
	event := f.events.recorded[0]
	if event.action != EventCreated || event.actor != (Actor{ActorUser, owner}) || event.detail["name"] != "Shop" ||
		!slices.Equal(event.detail["allowed_domains"].([]string), []string{"shop.example.com", "*.example.org"}) {
		t.Fatalf("event %+v", event)
	}
	// A bad domain is refused before any key is made.
	if _, err := f.usecase.Create(context.Background(), owner, "Other", []string{"https://shop.example.com"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("got %v, want INVALID_ARGUMENT", err)
	}
	if len(f.keys.keys) != 1 {
		t.Fatalf("%d keys after a refused create", len(f.keys.keys))
	}
}

func TestUpdateRecordsWhatChanged(t *testing.T) {
	key := shopKey()
	f := newKeyFixture(key)
	name, active, required := "Store", false, true
	tests := []struct {
		patch   ApiKeyPatch
		actions []string
	}{
		{ApiKeyPatch{Name: &name}, []string{EventRenamed}},
		{ApiKeyPatch{Name: &name}, nil},
		{ApiKeyPatch{IsActive: &active, RequireBehavior: &required}, []string{EventPaused, EventRequireBehaviorChanged}},
		{ApiKeyPatch{IsActive: &active, RequireBehavior: &required}, nil},
	}
	for i, test := range tests {
		f.events.recorded = nil
		if _, err := f.usecase.Update(context.Background(), owner, key.ID, test.patch); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(f.events.actions(), test.actions) {
			t.Errorf("update %d: events %q, want %q", i, f.events.actions(), test.actions)
		}
	}
	if _, err := f.usecase.Update(context.Background(), owner, key.ID, ApiKeyPatch{}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("empty patch: %v", err)
	}
}

func TestSecretEventsHoldNoSecret(t *testing.T) {
	key := shopKey()
	f := newKeyFixture(key)
	ctx := context.Background()
	if _, err := f.usecase.RevokePreviousSecret(ctx, owner, key.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.usecase.RotateSecret(ctx, owner, key.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.usecase.RevokePreviousSecret(ctx, owner, key.ID); err != nil {
		t.Fatal(err)
	}
	// The first revocation had nothing to revoke.
	if got := f.events.actions(); !slices.Equal(got, []string{EventSecretRotated, EventPreviousSecretRevoked}) {
		t.Fatalf("events %q", got)
	}
	for _, event := range f.events.recorded {
		for _, value := range event.detail {
			if text, ok := value.(string); ok && strings.HasPrefix(text, "sk_") {
				t.Fatalf("%s holds a secret key", event.action)
			}
		}
	}
}

func TestSetAllowedListsRecordChanges(t *testing.T) {
	key := shopKey()
	f := newKeyFixture(key)
	ctx := context.Background()
	for range 2 {
		if _, err := f.usecase.SetAllowedDomains(ctx, owner, key.ID, []string{"shop.example.com"}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.usecase.SetAllowedOrigins(ctx, owner, key.ID, []AllowedOrigin{{IPAddress: "203.0.113.4"}}); err != nil {
			t.Fatal(err)
		}
	}
	// Saving the same lists again changes nothing.
	if got := f.events.actions(); !slices.Equal(got, []string{EventAllowedDomainsChanged, EventAllowedIPsChanged}) {
		t.Fatalf("events %q", got)
	}
	if got := f.events.recorded[1].detail["ip_addresses"]; !slices.Equal(got.([]string), []string{"203.0.113.4/32"}) {
		t.Fatalf("ip_addresses %v", got)
	}
}

func TestCreateAppeal(t *testing.T) {
	tests := []struct {
		name string
		key  func(*ApiKey)
		err  error
	}{
		{"active key", func(*ApiKey) {}, ErrAppealNotAllowed},
		{"suspended", func(k *ApiKey) { k.IsSuspended, k.SuspendReason = true, SuspendedForScraping }, nil},
		{"flagged today", func(k *ApiKey) { k.ScrapingFlaggedAt = testNow.Add(-2 * time.Hour) }, nil},
		// 23:30 on 7 October in Ho Chi Minh City: yesterday, the throttle is over.
		{"flagged yesterday", func(k *ApiKey) { k.ScrapingFlaggedAt = time.Date(2026, 10, 7, 16, 30, 0, 0, time.UTC) }, ErrAppealNotAllowed},
	}
	for _, test := range tests {
		key := shopKey()
		test.key(key)
		f := newKeyFixture(key)
		appeal, err := f.usecase.CreateAppeal(context.Background(), owner, key.ID, "  We prefetch challenges on our SPA.  ")
		if test.err != nil {
			if !errors.Is(err, test.err) || len(f.events.recorded) != 0 {
				t.Errorf("%s: got %v, events %q", test.name, err, f.events.actions())
			}
			continue
		}
		if err != nil || appeal.Message != "We prefetch challenges on our SPA." || !slices.Equal(f.events.actions(), []string{EventAppealCreated}) {
			t.Errorf("%s: got %+v, %v, events %q", test.name, appeal, err, f.events.actions())
		}
		if _, err := f.usecase.CreateAppeal(context.Background(), owner, key.ID, "again"); !errors.Is(err, ErrAppealPending) {
			t.Errorf("%s: second appeal %v", test.name, err)
		}
	}
	key := shopKey()
	key.IsSuspended = true
	f := newKeyFixture(key)
	for _, message := range []string{"   ", strings.Repeat("é", 2001)} {
		if _, err := f.usecase.CreateAppeal(context.Background(), owner, key.ID, message); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("message of %d characters: %v", len([]rune(message)), err)
		}
	}
}

func TestGetCounters(t *testing.T) {
	key := shopKey()
	key.ScrapingFlaggedAt = testNow.Add(-time.Hour)
	f := newKeyFixture(key)
	f.counters.counts[key.ID] = ScrapingCount{Challenges: 600, Verifications: 40, Solved: 30, Automated: 2}
	got, err := f.usecase.Get(context.Background(), owner, key.ID)
	if err != nil || got.Today == nil || *got.Today != f.counters.counts[key.ID] || !got.ScrapingFlagged {
		t.Fatalf("got %+v, %v", got, err)
	}
	// Without Redis the page still loads, without the counters.
	f.counters.err = stderrors.New("redis: connection refused")
	got, err = f.usecase.Get(context.Background(), owner, key.ID)
	if err != nil || got.Today != nil {
		t.Fatalf("got %+v, %v", got, err)
	}
}

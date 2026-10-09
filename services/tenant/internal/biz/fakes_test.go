package biz

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// In-memory repositories of one user's keys, for the tests of this package.

type memoryKeys struct {
	keys    map[uuid.UUID]*ApiKey
	origins map[uuid.UUID][]AllowedOrigin
	domains map[uuid.UUID][]AllowedDomain
}

func newMemoryKeys(keys ...*ApiKey) *memoryKeys {
	m := &memoryKeys{keys: map[uuid.UUID]*ApiKey{}, origins: map[uuid.UUID][]AllowedOrigin{}, domains: map[uuid.UUID][]AllowedDomain{}}
	for _, key := range keys {
		m.keys[key.ID] = key
	}
	return m
}

func (m *memoryKeys) get(id uuid.UUID) (*ApiKey, error) {
	key, ok := m.keys[id]
	if !ok {
		return nil, ErrApiKeyNotFound
	}
	copied := *key
	return &copied, nil
}

func (m *memoryKeys) List(context.Context, uuid.UUID) ([]*ApiKey, error) {
	var keys []*ApiKey
	for id := range m.keys {
		key, _ := m.get(id)
		keys = append(keys, key)
	}
	return keys, nil
}

func (m *memoryKeys) Get(_ context.Context, _, id uuid.UUID) (*ApiKey, error) { return m.get(id) }

func (m *memoryKeys) Create(_ context.Context, _ uuid.UUID, name, siteKey, secretKey string, domains []string, _ int) (*ApiKey, error) {
	key := &ApiKey{ID: uuid.New(), Name: name, SiteKey: siteKey, SecretKey: secretKey, IsActive: true, RequireBehavior: true, CreateDate: time.Now()}
	m.keys[key.ID] = key
	for _, domain := range domains {
		m.domains[key.ID] = append(m.domains[key.ID], AllowedDomain{Domain: domain, CreateDate: key.CreateDate})
	}
	return m.get(key.ID)
}

func (m *memoryKeys) Update(_ context.Context, _, id uuid.UUID, patch ApiKeyPatch) (*ApiKey, error) {
	key, ok := m.keys[id]
	if !ok {
		return nil, ErrApiKeyNotFound
	}
	if patch.Name != nil {
		key.Name = *patch.Name
	}
	if patch.IsActive != nil {
		key.IsActive = *patch.IsActive
	}
	if patch.RequireBehavior != nil {
		key.RequireBehavior = *patch.RequireBehavior
	}
	return m.get(id)
}

func (m *memoryKeys) RotateSecret(_ context.Context, _, id uuid.UUID, secretKey string, _ time.Duration) (*ApiKey, error) {
	key := m.keys[id]
	key.PreviousSecretKey, key.SecretKey = key.SecretKey, secretKey
	return m.get(id)
}

func (m *memoryKeys) RevokePreviousSecret(_ context.Context, _, id uuid.UUID) (*ApiKey, error) {
	m.keys[id].PreviousSecretKey = ""
	return m.get(id)
}

func (m *memoryKeys) AllowedOrigins(_ context.Context, _, id uuid.UUID) ([]AllowedOrigin, error) {
	return m.origins[id], nil
}

func (m *memoryKeys) SetAllowedOrigins(_ context.Context, _, id uuid.UUID, origins []AllowedOrigin) ([]AllowedOrigin, error) {
	m.origins[id] = origins
	return origins, nil
}

func (m *memoryKeys) AllowedDomains(_ context.Context, _, id uuid.UUID) ([]AllowedDomain, error) {
	return m.domains[id], nil
}

func (m *memoryKeys) SetAllowedDomains(_ context.Context, _, id uuid.UUID, domains []string) ([]AllowedDomain, error) {
	m.domains[id] = nil
	for _, domain := range domains {
		m.domains[id] = append(m.domains[id], AllowedDomain{Domain: domain})
	}
	return m.domains[id], nil
}

func (m *memoryKeys) Delete(_ context.Context, _, id uuid.UUID) error {
	delete(m.keys, id)
	return nil
}

type recordedEvent struct {
	actor  Actor
	action string
	detail map[string]any
}

type memoryEvents struct{ recorded []recordedEvent }

func (m *memoryEvents) Record(_ context.Context, _ uuid.UUID, actor Actor, action string, detail map[string]any) error {
	m.recorded = append(m.recorded, recordedEvent{actor, action, detail})
	return nil
}

func (m *memoryEvents) List(context.Context, uuid.UUID, uuid.UUID, int) ([]ApiKeyEvent, error) {
	return nil, nil
}

func (m *memoryEvents) actions() []string {
	actions := make([]string, len(m.recorded))
	for i, event := range m.recorded {
		actions[i] = event.action
	}
	return actions
}

// memoryAppeals keeps appeals of keys whose admin view is in keys; deciding one acts on it.
type memoryAppeals struct {
	appeals map[uuid.UUID]*AdminAppeal
	keys    map[uuid.UUID]*AdminApiKey
}

func (m *memoryAppeals) Create(_ context.Context, userID, apiKeyID uuid.UUID, message string) (*Appeal, error) {
	for _, appeal := range m.appeals {
		if appeal.APIKeyID == apiKeyID && appeal.Status == AppealPending {
			return nil, ErrAppealPending
		}
	}
	appeal := &AdminAppeal{Appeal: Appeal{ID: uuid.New(), APIKeyID: apiKeyID, UserID: userID, Status: AppealPending, Message: message}}
	m.appeals[appeal.ID] = appeal
	return &appeal.Appeal, nil
}

func (m *memoryAppeals) Latest(context.Context, uuid.UUID, uuid.UUID) (*Appeal, error) {
	return nil, nil
}

func (m *memoryAppeals) List(_ context.Context, status string, _ int) ([]*AdminAppeal, error) {
	var appeals []*AdminAppeal
	for _, appeal := range m.appeals {
		if status == "all" || appeal.Status == status {
			appeals = append(appeals, m.withKey(appeal))
		}
	}
	return appeals, nil
}

func (m *memoryAppeals) Get(_ context.Context, id uuid.UUID) (*AdminAppeal, error) {
	appeal, ok := m.appeals[id]
	if !ok {
		return nil, ErrAppealNotFound
	}
	return m.withKey(appeal), nil
}

func (m *memoryAppeals) withKey(appeal *AdminAppeal) *AdminAppeal {
	copied := *appeal
	key := *m.keys[appeal.APIKeyID]
	copied.ApiKey = &key
	return &copied
}

func (m *memoryAppeals) Decide(_ context.Context, id uuid.UUID, approve bool, note string) error {
	appeal, ok := m.appeals[id]
	if !ok || appeal.Status != AppealPending {
		return ErrAppealNotFound
	}
	appeal.Status, appeal.AdminNote = AppealRejected, note
	if approve {
		appeal.Status = AppealApproved
		key := m.keys[appeal.APIKeyID]
		key.IsSuspended, key.SuspendReason, key.ScrapingFlaggedAt = false, "", time.Time{}
	}
	return nil
}

type memoryCounters struct {
	counts map[uuid.UUID]ScrapingCount
	resets []uuid.UUID
	err    error
}

func (m *memoryCounters) QuotaUsed(_ context.Context, ids []uuid.UUID, _ string) ([]int64, error) {
	return make([]int64, len(ids)), nil
}

func (m *memoryCounters) ScrapingCounts(_ context.Context, ids []uuid.UUID, _ string) ([]ScrapingCount, error) {
	if m.err != nil {
		return nil, m.err
	}
	counts := make([]ScrapingCount, len(ids))
	for i, id := range ids {
		counts[i] = m.counts[id]
	}
	return counts, nil
}

func (m *memoryCounters) ResetScraping(_ context.Context, id uuid.UUID, _ string) error {
	m.resets = append(m.resets, id)
	delete(m.counts, id)
	return nil
}

// testNow is 10:00 on 8 October 2026 in Ho Chi Minh City.
var testNow = time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)

func testCalendar() *Calendar {
	location, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	return &Calendar{location: location, zone: "Asia/Ho_Chi_Minh", now: func() time.Time { return testNow }}
}

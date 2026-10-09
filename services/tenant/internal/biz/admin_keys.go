package biz

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// maxAppeals is how many appeals the admin console lists at once.
const maxAppeals = 200

// AdminApiKey is a key as administrators see it: never with its secret key.
type AdminApiKey struct {
	ID            uuid.UUID
	Name          string
	SiteKey       string
	IsActive      bool
	IsSuspended   bool
	SuspendReason string
	CreateDate    time.Time
	// When the scraping check last flagged the key, and whether that was today.
	ScrapingFlaggedAt time.Time
	ScrapingFlagged   bool
	// Today's scraping counters.
	ScrapingCount
}

// ListApiKeys returns the keys of an account that are not deleted, with today's counters.
func (uc *AdminUsecase) ListApiKeys(ctx context.Context, userID uuid.UUID) ([]*AdminApiKey, error) {
	if _, err := uc.accounts.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	keys, err := uc.accounts.ListApiKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := uc.fillScraping(ctx, keys...); err != nil {
		return nil, err
	}
	return keys, nil
}

// SetApiKeySuspended suspends a key or lifts its suspension, which also clears its flag and
// today's scraping counters: they would flag or suspend the key again on its next challenge.
func (uc *AdminUsecase) SetApiKeySuspended(ctx context.Context, adminID, id uuid.UUID, suspended bool) (*AdminApiKey, error) {
	before, err := uc.accounts.GetApiKey(ctx, id)
	if err != nil {
		return nil, err
	}
	if !suspended {
		if err := uc.counters.ResetScraping(ctx, id, uc.calendar.Today()); err != nil {
			return nil, err
		}
	}
	key, err := uc.accounts.SetApiKeySuspended(ctx, id, suspended)
	if err != nil {
		return nil, err
	}
	actor := Actor{ActorAdmin, adminID}
	switch {
	case key.IsSuspended && !before.IsSuspended:
		recordEvent(ctx, uc.events, id, actor, EventSuspended, map[string]any{"reason": key.SuspendReason})
	case !key.IsSuspended && before.IsSuspended:
		recordEvent(ctx, uc.events, id, actor, EventUnsuspended, nil)
	}
	if err := uc.fillScraping(ctx, key); err != nil {
		return nil, err
	}
	return key, nil
}

// ListAppeals returns the appeals with status, "pending" when empty or "all", newest first.
func (uc *AdminUsecase) ListAppeals(ctx context.Context, status string) ([]*AdminAppeal, error) {
	switch status {
	case "":
		status = AppealPending
	case AppealPending, AppealApproved, AppealRejected, "all":
	default:
		return nil, InvalidField("status", "status is pending, approved, rejected or all")
	}
	appeals, err := uc.appeals.List(ctx, status, maxAppeals)
	if err != nil {
		return nil, err
	}
	keys := make([]*AdminApiKey, len(appeals))
	for i, appeal := range appeals {
		keys[i] = appeal.ApiKey
	}
	if err := uc.fillScraping(ctx, keys...); err != nil {
		return nil, err
	}
	return appeals, nil
}

// DecideAppeal approves a pending appeal, which lifts the suspension of its key, clears its flag
// and today's counters, or rejects it. The note is shown to the owner.
func (uc *AdminUsecase) DecideAppeal(ctx context.Context, adminID, id uuid.UUID, approve bool, note string) (*AdminAppeal, error) {
	note, err := appealText("note", note, false)
	if err != nil {
		return nil, err
	}
	appeal, err := uc.appeals.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if appeal.Status != AppealPending {
		return nil, ErrAppealNotFound
	}
	keyID := appeal.ApiKey.ID
	// Cleared first: should the decision then fail, the counters of a key under review are lost,
	// rather than a lifted suspension coming back at the next challenge.
	if approve {
		if err := uc.counters.ResetScraping(ctx, keyID, uc.calendar.Today()); err != nil {
			return nil, err
		}
	}
	if err := uc.appeals.Decide(ctx, id, approve, note); err != nil {
		return nil, err
	}
	actor := Actor{ActorAdmin, adminID}
	action := EventAppealRejected
	if approve {
		action = EventAppealApproved
	}
	recordEvent(ctx, uc.events, keyID, actor, action, map[string]any{"appeal_id": id.String(), "note": note})
	if approve && appeal.ApiKey.IsSuspended {
		recordEvent(ctx, uc.events, keyID, actor, EventUnsuspended, map[string]any{"appeal_id": id.String()})
	}
	decided, err := uc.appeals.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := uc.fillScraping(ctx, decided.ApiKey); err != nil {
		return nil, err
	}
	return decided, nil
}

// fillScraping adds today's counters of the scraping check to keys, and whether it flagged them
// today.
func (uc *AdminUsecase) fillScraping(ctx context.Context, keys ...*AdminApiKey) error {
	if len(keys) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(keys))
	for i, key := range keys {
		ids[i] = key.ID
	}
	counts, err := uc.counters.ScrapingCounts(ctx, ids, uc.calendar.Today())
	if err != nil {
		return err
	}
	for i, key := range keys {
		key.ScrapingCount = counts[i]
		key.ScrapingFlagged = uc.calendar.IsToday(key.ScrapingFlaggedAt)
	}
	return nil
}

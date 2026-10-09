package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/uuid"
)

// maxEvents is how many entries of the activity log of a key are shown.
const maxEvents = 100

// Who changed a key, as api_key_events.actor stores it; the challenge service writes as "system".
const (
	ActorUser  = "user"
	ActorAdmin = "admin"
)

// Actions of the activity log. The challenge service also writes EventFlagged and EventSuspended.
const (
	EventCreated                = "created"
	EventRenamed                = "renamed"
	EventActivated              = "activated"
	EventPaused                 = "paused"
	EventSecretRotated          = "secret_rotated"
	EventPreviousSecretRevoked  = "previous_secret_revoked"
	EventAllowedIPsChanged      = "allowed_ips_changed"
	EventAllowedDomainsChanged  = "allowed_domains_changed"
	EventRequireBehaviorChanged = "require_behavior_changed"
	EventFlagged                = "flagged"
	EventSuspended              = "suspended"
	EventUnsuspended            = "unsuspended"
	EventAppealCreated          = "appeal_created"
	EventAppealApproved         = "appeal_approved"
	EventAppealRejected         = "appeal_rejected"
	EventDeleted                = "deleted"
)

// ApiKeyEvent is one entry of the activity log of a key.
type ApiKeyEvent struct {
	ID     int64
	Actor  string
	Action string
	// What changed, e.g. {"from": "Shop", "to": "Store"}; never a secret key.
	Detail     map[string]any
	CreateDate time.Time
}

// Actor is who makes a change: a user on their own key, or an administrator.
type Actor struct {
	Kind string
	ID   uuid.UUID
}

// EventRepo keeps the activity log of keys in api_key_events.
type EventRepo interface {
	Record(ctx context.Context, apiKeyID uuid.UUID, actor Actor, action string, detail map[string]any) error
	// List returns the latest limit events of a key of the user, newest first.
	List(ctx context.Context, userID, apiKeyID uuid.UUID, limit int) ([]ApiKeyEvent, error)
}

// recordEvent adds an entry to the activity log of a key. The change it describes is made
// already, so a failure is logged rather than returned.
func recordEvent(ctx context.Context, events EventRepo, apiKeyID uuid.UUID, actor Actor, action string, detail map[string]any) {
	if detail == nil {
		detail = map[string]any{}
	}
	if err := events.Record(ctx, apiKeyID, actor, action, detail); err != nil {
		log.ErrorContext(ctx, "failed recording key event", "err", err, "api_key_id", apiKeyID, "action", action)
	}
}

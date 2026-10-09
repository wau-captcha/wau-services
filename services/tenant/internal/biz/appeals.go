package biz

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	v1 "tenant/api/tenant/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

// maxAppealText bounds the message of an owner and the note of an administrator.
const maxAppealText = 2000

// Statuses of an appeal, as api_key_appeals.status stores them.
const (
	AppealPending  = "pending"
	AppealApproved = "approved"
	AppealRejected = "rejected"
)

var (
	ErrAppealNotAllowed = errors.Conflict(v1.ErrorReason_APPEAL_NOT_ALLOWED.String(), "only a suspended or flagged key can be reviewed")
	ErrAppealPending    = errors.Conflict(v1.ErrorReason_APPEAL_PENDING.String(), "a review of this key is already pending")
	ErrAppealNotFound   = errors.NotFound(v1.ErrorReason_APPEAL_NOT_FOUND.String(), "appeal not found or already decided")
)

// Appeal is a review an owner asked for, of a suspended or flagged key.
type Appeal struct {
	ID         uuid.UUID
	APIKeyID   uuid.UUID
	UserID     uuid.UUID
	Status     string
	Message    string
	AdminNote  string
	CreateDate time.Time
	// Zero while pending.
	DecidedAt time.Time
}

// AdminAppeal is an appeal with the key and the owner, as administrators see it.
type AdminAppeal struct {
	Appeal
	ApiKey    *AdminApiKey
	UserEmail string
	UserName  string
}

// AppealRepo keeps appeals in api_key_appeals.
type AppealRepo interface {
	// Create returns ErrAppealPending while the key has a pending appeal, and ErrApiKeyNotFound
	// for a key that is not the user's.
	Create(ctx context.Context, userID, apiKeyID uuid.UUID, message string) (*Appeal, error)
	// Latest returns the latest appeal of a key of the user; nil when there is none.
	Latest(ctx context.Context, userID, apiKeyID uuid.UUID) (*Appeal, error)
	// List returns the appeals with status, or all of them for "all", newest first.
	List(ctx context.Context, status string, limit int) ([]*AdminAppeal, error)
	// Get returns ErrAppealNotFound when there is no such appeal.
	Get(ctx context.Context, id uuid.UUID) (*AdminAppeal, error)
	// Decide approves or rejects a pending appeal; approving lifts the suspension of its key and
	// clears its flag in the same statement. ErrAppealNotFound when it is not pending.
	Decide(ctx context.Context, id uuid.UUID, approve bool, note string) error
}

// CreateAppeal asks an administrator to review a key that is suspended, or that the scraping
// check flagged today.
func (uc *ApiKeyUsecase) CreateAppeal(ctx context.Context, userID, id uuid.UUID, message string) (*Appeal, error) {
	message, err := appealText("message", message, true)
	if err != nil {
		return nil, err
	}
	key, err := uc.keys.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if !key.IsSuspended && !uc.calendar.IsToday(key.ScrapingFlaggedAt) {
		return nil, ErrAppealNotAllowed
	}
	appeal, err := uc.appeals.Create(ctx, userID, id, message)
	if err != nil {
		return nil, err
	}
	recordEvent(ctx, uc.events, id, Actor{ActorUser, userID}, EventAppealCreated, map[string]any{"appeal_id": appeal.ID.String()})
	return appeal, nil
}

// appealText trims a message or a note and checks its length.
func appealText(field, text string, required bool) (string, error) {
	text = strings.TrimSpace(text)
	switch {
	case required && text == "":
		return "", InvalidField(field, field+" must not be empty")
	case utf8.RuneCountInString(text) > maxAppealText:
		return "", InvalidField(field, field+" must be at most 2000 characters")
	}
	return text, nil
}

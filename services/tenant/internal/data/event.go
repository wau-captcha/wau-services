package data

import (
	"context"
	"encoding/json"

	"tenant/internal/biz"
	"tenant/internal/data/db"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/uuid"
)

type eventRepo struct {
	data *Data
}

func NewEventRepo(data *Data) biz.EventRepo { return &eventRepo{data: data} }

func (r *eventRepo) Record(ctx context.Context, apiKeyID uuid.UUID, actor biz.Actor, action string, detail map[string]any) error {
	encoded, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	return r.data.queries.CreateAPIKeyEvent(ctx, db.CreateAPIKeyEventParams{
		APIKeyID:    apiKeyID,
		Actor:       actor.Kind,
		ActorUserID: uuid.NullUUID{UUID: actor.ID, Valid: actor.ID != uuid.Nil},
		Action:      action,
		Detail:      encoded,
	})
}

func (r *eventRepo) List(ctx context.Context, userID, apiKeyID uuid.UUID, limit int) ([]biz.ApiKeyEvent, error) {
	rows, err := r.data.queries.ListAPIKeyEvents(ctx, db.ListAPIKeyEventsParams{UserID: userID, APIKeyID: apiKeyID, MaxEvents: int64(limit)})
	if err != nil {
		return nil, err
	}
	events := make([]biz.ApiKeyEvent, 0, len(rows))
	for _, row := range rows {
		detail := map[string]any{}
		// The column is JSONB, so this only fails on a row written by hand; it is shown empty.
		if err := json.Unmarshal(row.Detail, &detail); err != nil {
			log.WarnContext(ctx, "unreadable key event detail", "err", err, "api_key_event_id", row.APIKeyEventID)
		}
		events = append(events, biz.ApiKeyEvent{
			ID:         row.APIKeyEventID,
			Actor:      row.Actor,
			Action:     row.Action,
			Detail:     detail,
			CreateDate: row.CreateDate.Time,
		})
	}
	return events, nil
}

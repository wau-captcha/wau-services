package data

import (
	"context"
	"errors"

	"tenant/internal/biz"
	"tenant/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type appealRepo struct {
	data *Data
}

func NewAppealRepo(data *Data) biz.AppealRepo { return &appealRepo{data: data} }

func (r *appealRepo) Create(ctx context.Context, userID, apiKeyID uuid.UUID, message string) (*biz.Appeal, error) {
	row, err := r.data.queries.CreateAPIKeyAppeal(ctx, db.CreateAPIKeyAppealParams{APIKeyID: apiKeyID, UserID: userID, Message: message})
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, biz.ErrApiKeyNotFound
	// The partial unique index: a second pending appeal.
	case errors.As(err, &pgErr) && pgErr.Code == uniqueViolation:
		return nil, biz.ErrAppealPending
	case err != nil:
		return nil, err
	}
	return toAppeal(row), nil
}

func (r *appealRepo) Latest(ctx context.Context, userID, apiKeyID uuid.UUID) (*biz.Appeal, error) {
	row, err := r.data.queries.GetLatestAPIKeyAppeal(ctx, db.GetLatestAPIKeyAppealParams{UserID: userID, APIKeyID: apiKeyID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toAppeal(row), nil
}

func (r *appealRepo) List(ctx context.Context, status string, limit int) ([]*biz.AdminAppeal, error) {
	rows, err := r.data.queries.ListAPIKeyAppeals(ctx, db.ListAPIKeyAppealsParams{Status: status, MaxAppeals: int64(limit)})
	if err != nil {
		return nil, err
	}
	appeals := make([]*biz.AdminAppeal, 0, len(rows))
	for _, row := range rows {
		appeals = append(appeals, toAdminAppeal(db.GetAPIKeyAppealRow(row)))
	}
	return appeals, nil
}

func (r *appealRepo) Get(ctx context.Context, id uuid.UUID) (*biz.AdminAppeal, error) {
	row, err := r.data.queries.GetAPIKeyAppeal(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrAppealNotFound
	}
	if err != nil {
		return nil, err
	}
	return toAdminAppeal(row), nil
}

func (r *appealRepo) Decide(ctx context.Context, id uuid.UUID, approve bool, note string) error {
	_, err := r.data.queries.DecideAPIKeyAppeal(ctx, db.DecideAPIKeyAppealParams{APIKeyAppealID: id, Approve: approve, AdminNote: note})
	if errors.Is(err, pgx.ErrNoRows) {
		return biz.ErrAppealNotFound
	}
	return err
}

func toAppeal(row db.APIKeyAppeal) *biz.Appeal {
	return &biz.Appeal{
		ID:         row.APIKeyAppealID,
		APIKeyID:   row.APIKeyID,
		UserID:     row.UserID,
		Status:     row.Status,
		Message:    row.Message,
		AdminNote:  row.AdminNote.String,
		CreateDate: row.CreateDate.Time,
		// NULL scans as the zero time: not decided yet.
		DecidedAt: row.DecidedAt.Time,
	}
}

func toAdminAppeal(row db.GetAPIKeyAppealRow) *biz.AdminAppeal {
	return &biz.AdminAppeal{
		Appeal: biz.Appeal{
			ID:         row.APIKeyAppealID,
			APIKeyID:   row.APIKeyID,
			UserID:     row.UserID,
			Status:     row.Status,
			Message:    row.Message,
			AdminNote:  row.AdminNote.String,
			CreateDate: row.CreateDate.Time,
			DecidedAt:  row.DecidedAt.Time,
		},
		ApiKey: &biz.AdminApiKey{
			ID:                row.APIKeyID,
			Name:              row.Name,
			SiteKey:           row.SiteKey,
			IsActive:          row.IsActive,
			IsSuspended:       row.IsSuspended,
			SuspendReason:     row.SuspendReason.String,
			CreateDate:        row.KeyCreateDate.Time,
			ScrapingFlaggedAt: row.ScrapingFlaggedAt.Time,
		},
		UserEmail: row.Email,
		UserName:  row.Username,
	}
}

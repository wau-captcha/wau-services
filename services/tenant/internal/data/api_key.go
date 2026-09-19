package data

import (
	"context"
	"errors"

	"tenant/internal/biz"
	"tenant/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// foreignKeyViolation is the PostgreSQL SQLSTATE for a missing referenced row.
const foreignKeyViolation = "23503"

type apiKeyRepo struct {
	data *Data
}

func NewApiKeyRepo(data *Data) biz.ApiKeyRepo { return &apiKeyRepo{data: data} }

func (r *apiKeyRepo) List(ctx context.Context, userID uuid.UUID) ([]*biz.ApiKey, error) {
	rows, err := r.data.queries.ListAPIKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	keys := make([]*biz.ApiKey, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, toApiKey(row))
	}
	return keys, nil
}

func (r *apiKeyRepo) Get(ctx context.Context, userID, id uuid.UUID) (*biz.ApiKey, error) {
	row, err := r.data.queries.GetAPIKey(ctx, db.GetAPIKeyParams{APIKeyID: id, UserID: userID})
	if err != nil {
		return nil, notFoundError(err)
	}
	return toApiKey(row), nil
}

func (r *apiKeyRepo) Create(ctx context.Context, userID uuid.UUID, name, siteKey, secretKey string, max int) (*biz.ApiKey, error) {
	row, err := r.data.queries.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		UserID:    userID,
		Name:      name,
		SiteKey:   siteKey,
		SecretKey: secretKey,
		MaxKeys:   int64(max),
	})
	// The insert is guarded by the key count, so no row means the limit was hit.
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrApiKeyLimitReached
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation {
		// The owner was removed while the session was still valid.
		return nil, biz.ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	return toApiKey(row), nil
}

func (r *apiKeyRepo) Update(ctx context.Context, userID, id uuid.UUID, patch biz.ApiKeyPatch) (*biz.ApiKey, error) {
	params := db.UpdateAPIKeyParams{APIKeyID: id, UserID: userID}
	if patch.Name != nil {
		params.Name = pgtype.Text{String: *patch.Name, Valid: true}
	}
	if patch.IsActive != nil {
		params.IsActive = pgtype.Bool{Bool: *patch.IsActive, Valid: true}
	}
	row, err := r.data.queries.UpdateAPIKey(ctx, params)
	if err != nil {
		return nil, notFoundError(err)
	}
	return toApiKey(row), nil
}

func (r *apiKeyRepo) RotateSecret(ctx context.Context, userID, id uuid.UUID, secretKey string) (*biz.ApiKey, error) {
	row, err := r.data.queries.RotateAPIKeySecret(ctx, db.RotateAPIKeySecretParams{
		SecretKey: secretKey,
		APIKeyID:  id,
		UserID:    userID,
	})
	if err != nil {
		return nil, notFoundError(err)
	}
	return toApiKey(row), nil
}

func (r *apiKeyRepo) Delete(ctx context.Context, userID, id uuid.UUID) error {
	affected, err := r.data.queries.DeleteAPIKey(ctx, db.DeleteAPIKeyParams{APIKeyID: id, UserID: userID})
	if err != nil {
		return err
	}
	if affected == 0 {
		return biz.ErrApiKeyNotFound
	}
	return nil
}

func toApiKey(row db.APIKey) *biz.ApiKey {
	return &biz.ApiKey{
		ID:            row.APIKeyID,
		Name:          row.Name,
		SiteKey:       row.SiteKey,
		SecretKey:     row.SecretKey,
		IsActive:      row.IsActive,
		CreateDate:    row.CreateDate.Time,
		IsSuspended:   row.IsSuspended,
		SuspendReason: row.SuspendReason.String,
	}
}

// notFoundError hides keys of other users behind the same error as missing ones.
func notFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return biz.ErrApiKeyNotFound
	}
	return err
}

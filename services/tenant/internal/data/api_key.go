package data

import (
	"context"
	"errors"
	"net/netip"
	"time"

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

func (r *apiKeyRepo) Create(ctx context.Context, userID uuid.UUID, name, siteKey, secretKey string, maxKeys int) (*biz.ApiKey, error) {
	row, err := r.data.queries.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		UserID:    userID,
		Name:      name,
		SiteKey:   siteKey,
		SecretKey: secretKey,
		MaxKeys:   int64(maxKeys),
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

func (r *apiKeyRepo) RotateSecret(ctx context.Context, userID, id uuid.UUID, secretKey string, grace time.Duration) (*biz.ApiKey, error) {
	row, err := r.data.queries.RotateAPIKeySecret(ctx, db.RotateAPIKeySecretParams{
		SecretKey:    secretKey,
		GraceSeconds: int32(grace.Seconds()),
		APIKeyID:     id,
		UserID:       userID,
	})
	if err != nil {
		return nil, notFoundError(err)
	}
	return toApiKey(row), nil
}

func (r *apiKeyRepo) RevokePreviousSecret(ctx context.Context, userID, id uuid.UUID) (*biz.ApiKey, error) {
	row, err := r.data.queries.RevokeAPIKeyPreviousSecret(ctx, db.RevokeAPIKeyPreviousSecretParams{
		APIKeyID: id,
		UserID:   userID,
	})
	if err != nil {
		return nil, notFoundError(err)
	}
	return toApiKey(row), nil
}

func (r *apiKeyRepo) AllowedOrigins(ctx context.Context, userID, id uuid.UUID) ([]biz.AllowedOrigin, error) {
	rows, err := r.data.queries.ListAllowedOrigins(ctx, db.ListAllowedOriginsParams{UserID: userID, APIKeyID: id})
	if err != nil {
		return nil, err
	}
	origins := make([]biz.AllowedOrigin, 0, len(rows))
	for _, row := range rows {
		origins = append(origins, biz.AllowedOrigin{
			IPAddress:   toNetwork(row.IpAddress),
			Description: row.Description.String,
			CreateDate:  row.CreateDate.Time,
		})
	}
	return origins, nil
}

func (r *apiKeyRepo) SetAllowedOrigins(ctx context.Context, userID, id uuid.UUID, origins []biz.AllowedOrigin) ([]biz.AllowedOrigin, error) {
	addresses := make([]string, 0, len(origins))
	descriptions := make([]string, 0, len(origins))
	for _, origin := range origins {
		addresses = append(addresses, origin.IPAddress)
		descriptions = append(descriptions, origin.Description)
	}
	err := r.data.queries.ReplaceAllowedOrigins(ctx, db.ReplaceAllowedOriginsParams{
		APIKeyID:     id,
		UserID:       userID,
		IpAddresses:  addresses,
		Descriptions: descriptions,
	})
	if err != nil {
		return nil, err
	}
	return r.AllowedOrigins(ctx, userID, id)
}

// toNetwork shows a stored address the way it is set: PostgreSQL leaves out a /32 or /128
// netmask, and the API always names one.
func toNetwork(address string) string {
	if prefix, err := netip.ParsePrefix(address); err == nil {
		return prefix.String()
	}
	if address, err := netip.ParseAddr(address); err == nil {
		return netip.PrefixFrom(address, address.BitLen()).String()
	}
	return address
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
	key := &biz.ApiKey{
		ID:            row.APIKeyID,
		Name:          row.Name,
		SiteKey:       row.SiteKey,
		SecretKey:     row.SecretKey,
		IsActive:      row.IsActive,
		CreateDate:    row.CreateDate.Time,
		IsSuspended:   row.IsSuspended,
		SuspendReason: row.SuspendReason.String,
	}
	// An expired previous secret already works nowhere, so it is reported as gone; that is
	// why no job has to clear it out.
	if row.PreviousSecretKey.Valid && row.PreviousSecretKeyExpiresAt.Time.After(time.Now()) {
		key.PreviousSecretKey = row.PreviousSecretKey.String
		key.PreviousSecretExpiresAt = row.PreviousSecretKeyExpiresAt.Time
	}
	return key
}

// notFoundError hides keys of other users behind the same error as missing ones.
func notFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return biz.ErrApiKeyNotFound
	}
	return err
}

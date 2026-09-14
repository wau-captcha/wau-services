package data

import (
	"context"
	"errors"

	"verify/internal/biz"

	"github.com/jackc/pgx/v5"
)

type siteRepo struct {
	data *Data
}

// NewSiteRepo creates a SiteRepo backed by PostgreSQL.
func NewSiteRepo(data *Data) biz.SiteRepo {
	return &siteRepo{data: data}
}

func (r *siteRepo) FindAPIKeyID(ctx context.Context, secretKey string) (string, error) {
	var id string
	err := r.data.db.QueryRow(ctx,
		`SELECT api_key_id::text FROM api_keys WHERE secret_key = $1 AND is_active AND NOT is_deleted`,
		secretKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", biz.ErrSecretKeyInvalid
	}
	return id, err
}

func (r *siteRepo) RecordUsage(ctx context.Context, apiKeyID, challengeType string, success bool) error {
	_, err := r.data.db.Exec(ctx,
		`INSERT INTO usage_logs (api_key_id, challenge_type_id, success) VALUES ($1, $2, $3)`,
		apiKeyID, challengeType, success)
	return err
}

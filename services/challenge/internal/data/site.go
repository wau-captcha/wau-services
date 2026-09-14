package data

import (
	"context"
	"errors"

	"challenge/internal/biz"

	"github.com/jackc/pgx/v5"
)

// findSiteSQL resolves an active site key to its API key and the active challenge types of its tier.
const findSiteSQL = `
SELECT k.api_key_id::text,
       COALESCE(array_agg(tct.challenge_type_id) FILTER (WHERE t.is_active AND ct.is_active), '{}')
FROM api_keys k
JOIN users u ON u.user_id = k.user_id
JOIN tiers t ON t.tier_id = u.tier_id
LEFT JOIN tier_challenge_types tct ON tct.tier_id = t.tier_id
LEFT JOIN challenge_types ct ON ct.challenge_type_id = tct.challenge_type_id
WHERE k.site_key = $1 AND k.is_active AND NOT k.is_deleted
GROUP BY k.api_key_id`

type siteRepo struct {
	data *Data
}

// NewSiteRepo creates a SiteRepo backed by PostgreSQL.
func NewSiteRepo(data *Data) biz.SiteRepo {
	return &siteRepo{data: data}
}

func (r *siteRepo) FindBySiteKey(ctx context.Context, siteKey string) (*biz.Site, error) {
	var s biz.Site
	err := r.data.db.QueryRow(ctx, findSiteSQL, siteKey).Scan(&s.APIKeyID, &s.ChallengeTypes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrSiteKeyInvalid
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

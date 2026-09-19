-- name: GetSiteBySiteKey :one
SELECT
    k.api_key_id,
    k.user_id,
    k.is_suspended,
    u.is_banned,
    t.daily_quota,
    ARRAY(
        SELECT ct.challenge_type_id
        FROM tier_challenge_types tct
        JOIN challenge_types ct ON ct.challenge_type_id = tct.challenge_type_id
        WHERE tct.tier_id = t.tier_id
          AND t.is_active
          AND ct.is_active
    )::text[] AS challenge_types
FROM api_keys k
JOIN users u ON u.user_id = k.user_id
JOIN tiers t ON t.tier_id = u.tier_id
WHERE k.site_key = $1
  AND k.is_active
  AND NOT k.is_deleted;

-- A key already suspended keeps the reason it was suspended for.
-- name: SuspendAPIKeyForScraping :exec
UPDATE api_keys
SET is_suspended = true,
    suspend_reason = 'scraping'
WHERE api_key_id = $1
  AND NOT is_suspended;

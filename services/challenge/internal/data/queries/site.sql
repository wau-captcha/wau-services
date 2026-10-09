-- The key, its account and the tier the account has now: a paid tier past its end counts as free.
-- name: GetSiteBySiteKey :one
SELECT
    k.api_key_id,
    k.user_id,
    k.is_suspended,
    k.require_behavior,
    k.scraping_flagged_at,
    ARRAY(
        SELECT o.ip_address::text
        FROM api_key_allowed_origins o
        WHERE o.api_key_id = k.api_key_id
    )::text[] AS allowed_origins,
    ARRAY(
        SELECT d.domain
        FROM api_key_allowed_domains d
        WHERE d.api_key_id = k.api_key_id
    )::text[] AS allowed_domains,
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
JOIN tiers t ON t.tier_id = CASE WHEN u.tier_expires_at <= now() THEN 'free' ELSE u.tier_id END
WHERE k.site_key = $1
  AND k.is_active
  AND NOT k.is_deleted;

-- A key already suspended keeps the reason it was suspended for. No row changed means it already was.
-- name: SuspendAPIKeyForScraping :execrows
UPDATE api_keys
SET is_suspended = true,
    suspend_reason = 'SCRAPING'
WHERE api_key_id = $1
  AND NOT is_suspended;

-- Flags a key once a day: no row changed means it already was flagged since day_start.
-- name: FlagAPIKeyForScraping :execrows
UPDATE api_keys
SET scraping_flagged_at = now()
WHERE api_key_id = @api_key_id
  AND (scraping_flagged_at IS NULL OR scraping_flagged_at < @day_start::timestamptz);

-- What the scraping check did to a key, for its activity log.
-- name: CreateSystemAPIKeyEvent :exec
INSERT INTO api_key_events (api_key_id, actor, action, detail)
VALUES (@api_key_id, 'system', @action, @detail::jsonb);

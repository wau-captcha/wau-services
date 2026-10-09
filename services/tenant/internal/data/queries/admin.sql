-- Administrators are known by email only: the admins table keeps no Google account id.
-- Addresses are compared case-insensitively, as they are typed in by hand.
-- name: FindAdminByEmail :one
SELECT admin_id, username, email, create_date
FROM admins
WHERE lower(email) = lower(@email::text);

-- name: GetAdmin :one
SELECT admin_id, username, email, create_date
FROM admins
WHERE admin_id = $1;

-- name: GetAdminStats :one
SELECT (SELECT count(*) FROM users) AS users,
       (SELECT count(*) FROM users WHERE is_banned) AS banned_users,
       (SELECT count(*) FROM api_keys WHERE NOT is_deleted) AS api_keys,
       (SELECT count(*) FROM api_keys WHERE NOT is_deleted AND is_suspended) AS suspended_api_keys,
       (SELECT count(*) FROM api_keys WHERE NOT is_deleted AND suspend_reason = 'SCRAPING') AS scraping_api_keys;

-- An empty pattern matches everyone; otherwise email or display name, case-insensitively.
-- name: ListAdminUsers :many
SELECT u.user_id, u.username, u.email, u.create_date, t.tier_id, t.tier_name, t.daily_quota, u.is_banned,
       (SELECT count(*) FROM api_keys k WHERE k.user_id = u.user_id AND NOT k.is_deleted) AS api_key_count,
       (CASE WHEN u.tier_expires_at > now() THEN u.tier_expires_at END)::timestamptz AS tier_expires_at
FROM users u
JOIN tiers t ON t.tier_id = CASE WHEN u.tier_expires_at <= now() THEN 'free' ELSE u.tier_id END
WHERE @pattern::text = '' OR u.email ILIKE @pattern::text OR u.username ILIKE @pattern::text
ORDER BY u.create_date DESC, u.user_id
LIMIT @page_limit::bigint OFFSET @page_offset::bigint;

-- name: CountAdminUsers :one
SELECT count(*)
FROM users u
WHERE @pattern::text = '' OR u.email ILIKE @pattern::text OR u.username ILIKE @pattern::text;

-- name: GetAdminUser :one
SELECT u.user_id, u.username, u.email, u.create_date, t.tier_id, t.tier_name, t.daily_quota, u.is_banned,
       (SELECT count(*) FROM api_keys k WHERE k.user_id = u.user_id AND NOT k.is_deleted) AS api_key_count,
       (CASE WHEN u.tier_expires_at > now() THEN u.tier_expires_at END)::timestamptz AS tier_expires_at
FROM users u
JOIN tiers t ON t.tier_id = CASE WHEN u.tier_expires_at <= now() THEN 'free' ELSE u.tier_id END
WHERE u.user_id = $1;

-- name: SetUserBanned :execrows
UPDATE users
SET is_banned = @banned::boolean
WHERE user_id = @user_id;

-- name: ListAdminAPIKeys :many
SELECT api_key_id, name, site_key, is_active, is_suspended, suspend_reason, create_date, scraping_flagged_at
FROM api_keys
WHERE user_id = $1
  AND NOT is_deleted
ORDER BY create_date DESC;

-- name: GetAdminAPIKey :one
SELECT api_key_id, name, site_key, is_active, is_suspended, suspend_reason, create_date, scraping_flagged_at
FROM api_keys
WHERE api_key_id = $1
  AND NOT is_deleted;

-- A key suspended for scraping keeps that reason when an administrator confirms it. Lifting a
-- suspension also clears the scraping flag.
-- name: SetAPIKeySuspended :one
UPDATE api_keys
SET is_suspended = @suspended::boolean,
    suspend_reason = CASE
        WHEN NOT @suspended::boolean THEN NULL
        WHEN is_suspended THEN suspend_reason
        ELSE 'ADMIN'
    END,
    scraping_flagged_at = CASE WHEN @suspended::boolean THEN scraping_flagged_at END
WHERE api_key_id = @api_key_id
  AND NOT is_deleted
RETURNING api_key_id, name, site_key, is_active, is_suspended, suspend_reason, create_date, scraping_flagged_at;

-- status 'all' lists every appeal.
-- name: ListAPIKeyAppeals :many
SELECT a.api_key_appeal_id, a.status, a.message, a.admin_note, a.create_date, a.decided_at,
       u.user_id, u.email, u.username,
       k.api_key_id, k.name, k.site_key, k.is_active, k.is_suspended, k.suspend_reason,
       k.create_date AS key_create_date, k.scraping_flagged_at
FROM api_key_appeals a
JOIN api_keys k ON k.api_key_id = a.api_key_id
JOIN users u ON u.user_id = a.user_id
WHERE @status::text = 'all' OR a.status = @status::text
ORDER BY a.create_date DESC, a.api_key_appeal_id DESC
LIMIT @max_appeals::bigint;

-- name: GetAPIKeyAppeal :one
SELECT a.api_key_appeal_id, a.status, a.message, a.admin_note, a.create_date, a.decided_at,
       u.user_id, u.email, u.username,
       k.api_key_id, k.name, k.site_key, k.is_active, k.is_suspended, k.suspend_reason,
       k.create_date AS key_create_date, k.scraping_flagged_at
FROM api_key_appeals a
JOIN api_keys k ON k.api_key_id = a.api_key_id
JOIN users u ON u.user_id = a.user_id
WHERE a.api_key_appeal_id = $1;

-- Approving lifts the suspension of the key and clears its flag in the same statement. No row
-- means the appeal does not exist or was decided already.
-- name: DecideAPIKeyAppeal :one
WITH decided AS (
    UPDATE api_key_appeals
    SET status = CASE WHEN @approve::boolean THEN 'approved' ELSE 'rejected' END,
        admin_note = NULLIF(@admin_note::text, ''),
        decided_at = now()
    WHERE api_key_appeal_id = @api_key_appeal_id
      AND status = 'pending'
    RETURNING api_key_id
), lifted AS (
    UPDATE api_keys k
    SET is_suspended = false,
        suspend_reason = NULL,
        scraping_flagged_at = NULL
    FROM decided d
    WHERE @approve::boolean
      AND k.api_key_id = d.api_key_id
)
SELECT api_key_id FROM decided;

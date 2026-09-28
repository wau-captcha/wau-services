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
SELECT api_key_id, name, site_key, is_active, is_suspended, suspend_reason, create_date
FROM api_keys
WHERE user_id = $1
  AND NOT is_deleted
ORDER BY create_date DESC;

-- A key suspended for scraping keeps that reason when an administrator confirms it.
-- name: SetAPIKeySuspended :one
UPDATE api_keys
SET is_suspended = @suspended::boolean,
    suspend_reason = CASE
        WHEN NOT @suspended::boolean THEN NULL
        WHEN is_suspended THEN suspend_reason
        ELSE 'ADMIN'
    END
WHERE api_key_id = @api_key_id
  AND NOT is_deleted
RETURNING api_key_id, name, site_key, is_active, is_suspended, suspend_reason, create_date;

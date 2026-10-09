-- name: ListAPIKeys :many
SELECT *
FROM api_keys
WHERE user_id = $1
  AND NOT is_deleted
ORDER BY create_date DESC;

-- name: GetAPIKey :one
SELECT *
FROM api_keys
WHERE api_key_id = @api_key_id
  AND user_id = @user_id
  AND NOT is_deleted;

-- Inserts only while the user stays under the key limit, so the check and the
-- insert cannot race; no row returned means the limit was reached. The allowed domains go
-- in with the key. require_behavior is left to its default, on for new keys.
-- name: CreateAPIKey :one
WITH created AS (
    INSERT INTO api_keys (user_id, name, site_key, secret_key)
    SELECT @user_id::uuid, @name::text, @site_key::text, @secret_key::text
    WHERE (
        SELECT count(*) FROM api_keys WHERE user_id = @user_id::uuid AND NOT is_deleted
    ) < @max_keys::bigint
    RETURNING *
), domains AS (
    INSERT INTO api_key_allowed_domains (api_key_id, domain)
    SELECT c.api_key_id, d.domain
    FROM created c, unnest(@domains::text[]) AS d(domain)
)
SELECT * FROM created;

-- name: UpdateAPIKey :one
UPDATE api_keys
SET name = COALESCE(sqlc.narg(name)::text, name),
    is_active = COALESCE(sqlc.narg(is_active)::boolean, is_active),
    require_behavior = COALESCE(sqlc.narg(require_behavior)::boolean, require_behavior)
WHERE api_key_id = @api_key_id
  AND user_id = @user_id
  AND NOT is_deleted
RETURNING *;

-- One statement moves the current secret to previous_secret_key and stores the new one,
-- so the key is never left without a working secret. An unexpired previous key from an
-- earlier reset is overwritten: only one is ever kept.
-- name: RotateAPIKeySecret :one
UPDATE api_keys
SET secret_key = @secret_key,
    previous_secret_key = secret_key,
    previous_secret_key_expires_at = now() + make_interval(secs => @grace_seconds::int)
WHERE api_key_id = @api_key_id
  AND user_id = @user_id
  AND NOT is_deleted
RETURNING *;

-- name: RevokeAPIKeyPreviousSecret :one
UPDATE api_keys
SET previous_secret_key = NULL,
    previous_secret_key_expires_at = NULL
WHERE api_key_id = @api_key_id
  AND user_id = @user_id
  AND NOT is_deleted
RETURNING *;

-- name: ListAllowedOrigins :many
SELECT o.ip_address::text AS ip_address, o.description, o.create_date
FROM api_key_allowed_origins o
JOIN api_keys k ON k.api_key_id = o.api_key_id AND k.user_id = @user_id
WHERE o.api_key_id = @api_key_id
ORDER BY o.create_date, o.ip_address;

-- One statement for the whole list: addresses left out are deleted, the rest are inserted
-- or keep their row, so an address that stays keeps the date it was added on. Nothing
-- happens when the key is not this user's.
-- name: ReplaceAllowedOrigins :exec
WITH owned AS (
    SELECT k.api_key_id
    FROM api_keys k
    WHERE k.api_key_id = @api_key_id
      AND k.user_id = @user_id
      AND NOT k.is_deleted
), input AS (
    SELECT unnest(@ip_addresses::text[])::inet AS ip_address,
           unnest(@descriptions::text[]) AS description
), removed AS (
    DELETE FROM api_key_allowed_origins o
    USING owned w
    WHERE o.api_key_id = w.api_key_id
      AND NOT EXISTS (SELECT 1 FROM input i WHERE i.ip_address = o.ip_address)
)
INSERT INTO api_key_allowed_origins (api_key_id, ip_address, description)
SELECT w.api_key_id, i.ip_address, NULLIF(i.description, '')
FROM owned w, input i
ON CONFLICT (api_key_id, ip_address) DO UPDATE SET description = EXCLUDED.description;

-- name: ListAllowedDomains :many
SELECT d.domain, d.create_date
FROM api_key_allowed_domains d
JOIN api_keys k ON k.api_key_id = d.api_key_id AND k.user_id = @user_id
WHERE d.api_key_id = @api_key_id
ORDER BY d.create_date, d.domain;

-- The whole list in one statement, as for the addresses: domains left out are deleted, the
-- others inserted or kept with the date they were added on.
-- name: ReplaceAllowedDomains :exec
WITH owned AS (
    SELECT k.api_key_id
    FROM api_keys k
    WHERE k.api_key_id = @api_key_id
      AND k.user_id = @user_id
      AND NOT k.is_deleted
), input AS (
    SELECT unnest(@domains::text[]) AS domain
), removed AS (
    DELETE FROM api_key_allowed_domains d
    USING owned w
    WHERE d.api_key_id = w.api_key_id
      AND NOT EXISTS (SELECT 1 FROM input i WHERE i.domain = d.domain)
)
INSERT INTO api_key_allowed_domains (api_key_id, domain)
SELECT w.api_key_id, i.domain
FROM owned w, input i
ON CONFLICT (api_key_id, domain) DO NOTHING;

-- name: CreateAPIKeyEvent :exec
INSERT INTO api_key_events (api_key_id, actor, actor_user_id, action, detail)
VALUES (@api_key_id, @actor, sqlc.narg(actor_user_id), @action, @detail::jsonb);

-- name: ListAPIKeyEvents :many
SELECT e.api_key_event_id, e.actor, e.action, e.detail, e.create_date
FROM api_key_events e
JOIN api_keys k ON k.api_key_id = e.api_key_id AND k.user_id = @user_id AND NOT k.is_deleted
WHERE e.api_key_id = @api_key_id
ORDER BY e.create_date DESC, e.api_key_event_id DESC
LIMIT @max_events::bigint;

-- Only for a key of the user that is not deleted; the partial unique index refuses a second
-- pending appeal.
-- name: CreateAPIKeyAppeal :one
INSERT INTO api_key_appeals (api_key_id, user_id, message)
SELECT k.api_key_id, k.user_id, @message::text
FROM api_keys k
WHERE k.api_key_id = @api_key_id
  AND k.user_id = @user_id
  AND NOT k.is_deleted
RETURNING *;

-- name: GetLatestAPIKeyAppeal :one
SELECT a.api_key_appeal_id, a.api_key_id, a.user_id, a.message, a.status, a.admin_note, a.create_date, a.decided_at
FROM api_key_appeals a
JOIN api_keys k ON k.api_key_id = a.api_key_id AND k.user_id = @user_id
WHERE a.api_key_id = @api_key_id
ORDER BY a.create_date DESC
LIMIT 1;

-- name: DeleteAPIKey :execrows
UPDATE api_keys
SET is_deleted = true,
    is_active = false
WHERE api_key_id = @api_key_id
  AND user_id = @user_id
  AND NOT is_deleted;

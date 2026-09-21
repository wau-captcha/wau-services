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
-- insert cannot race; no row returned means the limit was reached.
-- name: CreateAPIKey :one
INSERT INTO api_keys (user_id, name, site_key, secret_key)
SELECT @user_id::uuid, @name::text, @site_key::text, @secret_key::text
WHERE (
    SELECT count(*) FROM api_keys WHERE user_id = @user_id::uuid AND NOT is_deleted
) < @max_keys::bigint
RETURNING *;

-- name: UpdateAPIKey :one
UPDATE api_keys
SET name = COALESCE(sqlc.narg(name)::text, name),
    is_active = COALESCE(sqlc.narg(is_active)::boolean, is_active)
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

-- name: DeleteAPIKey :execrows
UPDATE api_keys
SET is_deleted = true,
    is_active = false
WHERE api_key_id = @api_key_id
  AND user_id = @user_id
  AND NOT is_deleted;

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

-- name: RotateAPIKeySecret :one
UPDATE api_keys
SET secret_key = @secret_key
WHERE api_key_id = @api_key_id
  AND user_id = @user_id
  AND NOT is_deleted
RETURNING *;

-- name: DeleteAPIKey :execrows
UPDATE api_keys
SET is_deleted = true,
    is_active = false
WHERE api_key_id = @api_key_id
  AND user_id = @user_id
  AND NOT is_deleted;

-- The secret replaced by the last reset stays valid until it expires, so a site can roll
-- the new one out without downtime.
-- name: GetSiteBySecretKey :one
SELECT k.api_key_id, k.is_suspended, u.is_banned
FROM api_keys k
JOIN users u ON u.user_id = k.user_id
WHERE (k.secret_key = @secret_key::text
       OR (k.previous_secret_key = @secret_key::text AND k.previous_secret_key_expires_at > now()))
  AND k.is_active
  AND NOT k.is_deleted;

-- name: CreateUsageLog :exec
INSERT INTO usage_logs (api_key_id, challenge_type_id, success)
VALUES ($1, $2, $3);

-- name: GetSiteBySecretKey :one
SELECT k.api_key_id, k.is_suspended, u.is_banned
FROM api_keys k
JOIN users u ON u.user_id = k.user_id
WHERE k.secret_key = $1
  AND k.is_active
  AND NOT k.is_deleted;

-- name: CreateUsageLog :exec
INSERT INTO usage_logs (api_key_id, challenge_type_id, success)
VALUES ($1, $2, $3);

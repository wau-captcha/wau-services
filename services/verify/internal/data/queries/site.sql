-- name: GetAPIKeyIDBySecretKey :one
SELECT api_key_id
FROM api_keys
WHERE secret_key = $1
  AND is_active
  AND NOT is_deleted;

-- name: CreateUsageLog :exec
INSERT INTO usage_logs (api_key_id, challenge_type_id, success)
VALUES ($1, $2, $3);

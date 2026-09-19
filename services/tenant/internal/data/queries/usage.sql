-- Verifications per local day, API key and challenge type. A null user_id or api_key_id
-- means "any"; deleted keys are included. Days are grouped in the caller's time zone.
-- name: UsageBreakdown :many
SELECT (l.timestamp AT TIME ZONE @time_zone::text)::date AS day,
       l.api_key_id,
       l.challenge_type_id,
       count(*) AS total,
       count(*) FILTER (WHERE l.success) AS success
FROM usage_logs l
JOIN api_keys k ON k.api_key_id = l.api_key_id
WHERE l.timestamp >= @since::timestamptz
  AND (sqlc.narg(user_id)::uuid IS NULL OR k.user_id = sqlc.narg(user_id)::uuid)
  AND (sqlc.narg(api_key_id)::uuid IS NULL OR l.api_key_id = sqlc.narg(api_key_id)::uuid)
GROUP BY day, l.api_key_id, l.challenge_type_id;

-- The whole system has too many keys to break down by key, so only by day and type.
-- name: SystemUsage :many
SELECT (l.timestamp AT TIME ZONE @time_zone::text)::date AS day,
       l.challenge_type_id,
       count(*) AS total,
       count(*) FILTER (WHERE l.success) AS success
FROM usage_logs l
WHERE l.timestamp >= @since::timestamptz
GROUP BY day, l.challenge_type_id;

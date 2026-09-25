-- Challenges per local day, API key and challenge type: every /challenge/new call, the ones
-- answered through /challenge/verify (success is set) and the right answers. A null since means
-- all time; a null user_id or api_key_id means "any". Deleted keys are included and say so.
-- Days are grouped in the caller's time zone.
-- name: UsageBreakdown :many
SELECT (l.timestamp AT TIME ZONE @time_zone::text)::date AS day,
       k.api_key_id,
       k.name AS api_key_name,
       k.is_deleted AS api_key_deleted,
       k.create_date AS api_key_create_date,
       l.challenge_type_id,
       count(*) AS challenges,
       count(l.success) AS verifications,
       count(*) FILTER (WHERE l.success) AS successes
FROM usage_logs l
JOIN api_keys k ON k.api_key_id = l.api_key_id
WHERE (sqlc.narg(since)::timestamptz IS NULL OR l.timestamp >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(user_id)::uuid IS NULL OR k.user_id = sqlc.narg(user_id)::uuid)
  AND (sqlc.narg(api_key_id)::uuid IS NULL OR l.api_key_id = sqlc.narg(api_key_id)::uuid)
GROUP BY day, k.api_key_id, l.challenge_type_id;

-- The whole system has too many keys to break down by key, so only by day and type.
-- name: SystemUsage :many
SELECT (l.timestamp AT TIME ZONE @time_zone::text)::date AS day,
       l.challenge_type_id,
       count(*) AS challenges,
       count(l.success) AS verifications,
       count(*) FILTER (WHERE l.success) AS successes
FROM usage_logs l
WHERE sqlc.narg(since)::timestamptz IS NULL OR l.timestamp >= sqlc.narg(since)::timestamptz
GROUP BY day, l.challenge_type_id;

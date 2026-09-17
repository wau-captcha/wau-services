-- Usage of every key of the user, deleted keys included; a null api_key_id
-- means "all keys". Days are grouped in the caller's time zone.
-- name: DailyUsage :many
SELECT (l.timestamp AT TIME ZONE @time_zone::text)::date AS day,
       count(*) AS total,
       count(*) FILTER (WHERE l.success) AS success
FROM usage_logs l
JOIN api_keys k ON k.api_key_id = l.api_key_id
WHERE k.user_id = @user_id::uuid
  AND l.timestamp >= @since::timestamptz
  AND (sqlc.narg(api_key_id)::uuid IS NULL OR l.api_key_id = sqlc.narg(api_key_id)::uuid)
GROUP BY day
ORDER BY day;

-- name: ChallengeTypeUsage :many
SELECT l.challenge_type_id,
       count(*) AS total,
       count(*) FILTER (WHERE l.success) AS success
FROM usage_logs l
JOIN api_keys k ON k.api_key_id = l.api_key_id
WHERE k.user_id = @user_id::uuid
  AND l.timestamp >= @since::timestamptz
  AND (sqlc.narg(api_key_id)::uuid IS NULL OR l.api_key_id = sqlc.narg(api_key_id)::uuid)
GROUP BY l.challenge_type_id
ORDER BY total DESC;

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

-- The answer to a challenge fills success and the behaviour verdict on the row /challenge/new
-- wrote, or writes that row when the answer arrives before it. The score is NULL when nothing
-- was measured.
-- name: RecordAnsweredUsageLog :exec
INSERT INTO usage_logs (challenge_id, api_key_id, challenge_type_id, success, behavior_verdict, behavior_score)
VALUES (@challenge_id::uuid, @api_key_id, @challenge_type_id, @success::boolean, @behavior_verdict::text,
        sqlc.narg(behavior_score)::real)
ON CONFLICT (challenge_id) DO UPDATE SET success = EXCLUDED.success,
                                         behavior_verdict = EXCLUDED.behavior_verdict,
                                         behavior_score = EXCLUDED.behavior_score;

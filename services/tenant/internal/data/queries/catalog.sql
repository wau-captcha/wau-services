-- Challenge types the tier of the user includes, active ones only; a paid tier past its end is free.
-- name: ListChallengeTypes :many
SELECT ct.challenge_type_id, ct.code
FROM users u
JOIN tiers t ON t.tier_id = CASE WHEN u.tier_expires_at <= now() THEN 'free' ELSE u.tier_id END AND t.is_active
JOIN tier_challenge_types tct ON tct.tier_id = t.tier_id
JOIN challenge_types ct ON ct.challenge_type_id = tct.challenge_type_id AND ct.is_active
WHERE u.user_id = $1
ORDER BY ct.challenge_type_id;

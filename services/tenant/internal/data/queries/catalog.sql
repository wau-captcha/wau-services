-- Challenge types the tier of the user includes, active ones only; a paid tier past its end is free.
-- name: ListChallengeTypes :many
SELECT ct.challenge_type_id, ct.code
FROM users u
JOIN tiers t ON t.tier_id = CASE WHEN u.tier_expires_at <= now() THEN 'free' ELSE u.tier_id END AND t.is_active
JOIN tier_challenge_types tct ON tct.tier_id = t.tier_id
JOIN challenge_types ct ON ct.challenge_type_id = tct.challenge_type_id AND ct.is_active
WHERE u.user_id = $1
ORDER BY ct.challenge_type_id;

-- The active tiers, cheapest first, with their active challenge types: what the pricing page shows.
-- name: ListPublicPlans :many
SELECT t.tier_id,
       t.tier_name,
       COALESCE(t.description, '')::text AS description,
       t.daily_quota,
       t.monthly_price,
       t.yearly_price,
       ARRAY(
           SELECT ct.challenge_type_id
           FROM tier_challenge_types tct
           JOIN challenge_types ct ON ct.challenge_type_id = tct.challenge_type_id
           WHERE tct.tier_id = t.tier_id
             AND ct.is_active
           ORDER BY ct.challenge_type_id
       )::text[] AS challenge_types
FROM tiers t
WHERE t.is_active
ORDER BY t.monthly_price, t.daily_quota, t.tier_id;

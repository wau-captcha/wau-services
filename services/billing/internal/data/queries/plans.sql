-- Active tiers, cheapest first, with their active challenge types.
-- name: ListPlans :many
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

-- name: GetPlan :one
SELECT tier_id, tier_name, monthly_price, yearly_price
FROM tiers
WHERE tier_id = $1
  AND is_active;

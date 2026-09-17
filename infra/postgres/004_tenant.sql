-- Tenant dashboard: named keys; every challenge type is free for now.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '';

INSERT INTO tier_challenge_types (tier_id, challenge_type_id)
SELECT 'free', challenge_type_id FROM challenge_types
ON CONFLICT DO NOTHING;

-- Not implemented by the challenge service yet.
UPDATE challenge_types SET is_active = false
WHERE challenge_type_id IN ('slide_puzzle', 'behavioral_advanced');

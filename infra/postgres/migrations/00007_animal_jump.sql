-- +goose Up
INSERT INTO challenge_types (challenge_type_id, code, is_active) VALUES
    ('animal_jump',     'ANIMAL_JUMP',      true)
ON CONFLICT DO NOTHING;

INSERT INTO tier_challenge_types (tier_id, challenge_type_id) VALUES
    ('free',        'animal_jump'),
    ('plus',        'animal_jump'),
    ('pro',         'animal_jump'),
    ('enterprise',  'animal_jump')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM tier_challenge_types
WHERE challenge_type_id = 'animal_jump'
  AND tier_id IN ('free', 'plus', 'pro', 'enterprise');

DELETE FROM challenge_types
WHERE challenge_type_id = 'animal_jump'
  AND NOT EXISTS (SELECT 1 FROM usage_logs WHERE challenge_type_id = 'animal_jump');

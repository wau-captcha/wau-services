-- +goose Up
INSERT INTO challenge_types (challenge_type_id, code, is_active) VALUES
    ('object_rotate',   'OBJECT_ROTATE',    true)
ON CONFLICT DO NOTHING;

INSERT INTO tier_challenge_types (tier_id, challenge_type_id) VALUES
    ('free',        'object_rotate'),
    ('plus',        'object_rotate'),
    ('pro',         'object_rotate'),
    ('enterprise',  'object_rotate')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM tier_challenge_types
WHERE challenge_type_id = 'object_rotate'
  AND tier_id IN ('free', 'plus', 'pro', 'enterprise');

DELETE FROM challenge_types
WHERE challenge_type_id = 'object_rotate'
  AND NOT EXISTS (SELECT 1 FROM usage_logs WHERE challenge_type_id = 'object_rotate');

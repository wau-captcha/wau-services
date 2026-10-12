-- +goose Up
INSERT INTO challenge_types (challenge_type_id, code, is_active) VALUES
    ('auto_check',      'AUTO_CHECK',       true)
ON CONFLICT DO NOTHING;

INSERT INTO tier_challenge_types (tier_id, challenge_type_id) VALUES
    ('free',        'auto_check'),
    ('plus',        'auto_check'),
    ('pro',         'auto_check'),
    ('enterprise',  'auto_check')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM tier_challenge_types
WHERE challenge_type_id = 'auto_check'
  AND tier_id IN ('free', 'plus', 'pro', 'enterprise');

DELETE FROM challenge_types
WHERE challenge_type_id = 'auto_check'
  AND NOT EXISTS (SELECT 1 FROM usage_logs WHERE challenge_type_id = 'auto_check');

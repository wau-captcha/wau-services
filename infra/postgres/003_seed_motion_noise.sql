INSERT INTO challenge_types (challenge_type_id, code, is_active) VALUES
    ('motion_noise', 'MOTION_NOISE', true)
ON CONFLICT (challenge_type_id) DO NOTHING;

INSERT INTO tier_challenge_types (tier_id, challenge_type_id) VALUES
    ('premium', 'motion_noise')
ON CONFLICT DO NOTHING;

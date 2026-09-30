-- +goose Up
INSERT INTO users (user_id, username, email, tier_id) VALUES
    (uuidv7(), 'John Doe',    'john.doe@example.com',    'pro'),
    (uuidv7(), 'Jane Smith',  'jane.smith@example.com',  'free'),
    (uuidv7(), 'Acme Shop',   'contact@acmeshop.example','free');

INSERT INTO admins (admin_id, username, email) VALUES
    (uuidv7(), 'Admin', 'admin@gmail.com');

INSERT INTO api_keys (site_key, secret_key, user_id) VALUES
    ('pk_26b842dfa84e187d45bfe73655182748471351d8ecb93b36', 'sk_c582fa0b9d443ebe604ae50b3756c8b0560dda11c15e3c2a',
        (SELECT user_id FROM users WHERE email = 'john.doe@example.com')),
    ('pk_2439afc3901b63928624704a93fda564102959a6d8e7aea9', 'sk_ee06e26da3dd1e365e842ba29e4813917da1246f36becbec',
        (SELECT user_id FROM users WHERE email = 'john.doe@example.com')),
    ('pk_a834b1bc44ccc6acc68b8b78d552b30cefac0d73ff6ade0e', 'sk_4a43813384268b62be376cf7b81eb47ca9619c23e2b4c198',
        (SELECT user_id FROM users WHERE email = 'jane.smith@example.com')),
    ('pk_09eb63b2940720bfd832887c8fdc1973658317f2ae8d322e', 'sk_d628fcee5254443b091842f178f28ded2589356b6a7dc09e',
        (SELECT user_id FROM users WHERE email = 'contact@acmeshop.example'));

-- +goose Down
DELETE FROM admins WHERE email = 'admin@gmail.com';
DELETE FROM users WHERE email IN ('john.doe@example.com', 'jane.smith@example.com', 'contact@acmeshop.example');

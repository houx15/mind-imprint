-- +goose Up
-- Deterministic seed: one school, one class, one org-bound student.
-- Fixed UUIDs keep the seed idempotent-friendly and referenceable by tests/dev.
INSERT INTO schools (id, name)
VALUES ('00000000-0000-0000-0000-000000000001', 'Demo School');

INSERT INTO classes (id, school_id, name, join_code)
VALUES ('00000000-0000-0000-0000-000000000002',
        '00000000-0000-0000-0000-000000000001',
        'Demo Class', 'DEMO-0001');

INSERT INTO users (id, email, email_verified_at, password_hash, role, school_id, display_name, avatar_color)
VALUES ('00000000-0000-0000-0000-000000000003',
        'phoebe@demo.mindimprint.local',
        now(),
        'SEED_NO_LOGIN',
        'student',
        '00000000-0000-0000-0000-000000000001',
        'Phoebe',
        '#7C9CF0');

INSERT INTO enrollments (id, user_id, class_id, role_in_class)
VALUES ('00000000-0000-0000-0000-000000000004',
        '00000000-0000-0000-0000-000000000003',
        '00000000-0000-0000-0000-000000000002',
        'student');

-- +goose Down
DELETE FROM enrollments WHERE id = '00000000-0000-0000-0000-000000000004';
DELETE FROM users       WHERE id = '00000000-0000-0000-0000-000000000003';
DELETE FROM classes     WHERE id = '00000000-0000-0000-0000-000000000002';
DELETE FROM schools     WHERE id = '00000000-0000-0000-0000-000000000001';

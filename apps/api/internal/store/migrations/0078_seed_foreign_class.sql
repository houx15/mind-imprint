-- +goose Up
-- Tenancy isolation fixture: a class + student that are FOREIGN to wu.teacher.
--
-- Background: 0029 makes wu.teacher ('...910') teach class '...902', and 0073
-- enrolls her as teacher of Demo Class '...002' too (where Phoebe '...003'
-- lives) — so neither '...002' nor Phoebe is "foreign" to her any more. The
-- tenancy e2e needs a class she does NOT teach and a student she cannot see, to
-- assert every cross-tenant read 404s.
--
-- Adds, under the existing Demo School '...001' (seeded by 0002):
--   class   '...903'  外部班级（租户隔离测试） — wu.teacher is NOT enrolled here.
--   user    '...950'  a student in '...903' only (not in '...902'/'...002').
-- Column shapes mirror 0002/0029/0073. Idempotent (ON CONFLICT) so re-running
-- migrate-up is safe. Fixed UUIDs in the otherwise-unused 09xx range:
-- 0029 used users 900-919, enrollments 930-939, projects 951-957; 0073 used
-- enrollment 940. So class 903, user 950, and enrollment 941 are all free.

INSERT INTO classes (id, school_id, name, join_code) VALUES
  ('00000000-0000-0000-0000-000000000903',
   '00000000-0000-0000-0000-000000000001',
   '外部班级（租户隔离测试）', 'EXTERN-01')
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (id, email, email_verified_at, password_hash, role, school_id, display_name, avatar_color) VALUES
  ('00000000-0000-0000-0000-000000000950',
   'foreign.student@demo.mindimprint.local', now(), 'SEED_NO_LOGIN', 'student',
   '00000000-0000-0000-0000-000000000001', '外部学员', '#8A8F9C')
ON CONFLICT (id) DO NOTHING;

INSERT INTO enrollments (id, user_id, class_id, role_in_class) VALUES
  ('00000000-0000-0000-0000-000000000941',
   '00000000-0000-0000-0000-000000000950',
   '00000000-0000-0000-0000-000000000903',
   'student')
ON CONFLICT (user_id, class_id) DO NOTHING;

-- +goose Down
DELETE FROM enrollments WHERE id = '00000000-0000-0000-0000-000000000941';
DELETE FROM users       WHERE id = '00000000-0000-0000-0000-000000000950';
DELETE FROM classes     WHERE id = '00000000-0000-0000-0000-000000000903';

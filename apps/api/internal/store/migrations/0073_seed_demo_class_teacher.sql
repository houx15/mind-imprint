-- +goose Up
-- Demo teacher for the walkthrough class. Phoebe (0002 seed) and the 10 walked
-- persona students (created on PROD via join code DEMO-0001) all live in Demo
-- Class '...002', but no teacher was enrolled there — so the teacher console had
-- no login that could see them. Reuse the already-seeded 吴老师 ('...910', from
-- 0029) by enrolling her as teacher in Demo Class too; she then teaches both her
-- research class ('...902') and this demo class, and assertTeacherOwnsClass /
-- ListClassRosterCounts surface Phoebe + every persona student to her.
--
-- Enrollment is the only teacher↔class join (no classes.teacher_id, no teacher
-- table). ON CONFLICT (user_id, class_id) makes this idempotent if an enrollment
-- for this pair already exists. Enrollment id in the unused 09xx range (0940;
-- 0029 used 0930-0939).
INSERT INTO enrollments (id, user_id, class_id, role_in_class) VALUES
  ('00000000-0000-0000-0000-000000000940',
   '00000000-0000-0000-0000-000000000910',  -- 吴老师
   '00000000-0000-0000-0000-000000000002',  -- Demo Class (Phoebe + personas)
   'teacher')
ON CONFLICT (user_id, class_id) DO UPDATE SET role_in_class = 'teacher';

-- +goose Down
DELETE FROM enrollments WHERE id = '00000000-0000-0000-0000-000000000940';

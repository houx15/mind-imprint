-- name: GetClassByID :one
SELECT * FROM classes WHERE id = $1;

-- name: GetEnrollment :one
SELECT * FROM enrollments WHERE user_id = $1 AND class_id = $2;

-- name: CreateTeacherInvite :one
INSERT INTO teacher_invites (school_id, code, email, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListActiveTeacherInvitesBySchool :many
SELECT * FROM teacher_invites
WHERE school_id = $1 AND consumed_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: GetActiveTeacherInviteByCode :one
SELECT * FROM teacher_invites
WHERE code = $1 AND consumed_at IS NULL AND expires_at > now();

-- name: ConsumeTeacherInvite :exec
UPDATE teacher_invites SET consumed_at = now(), consumed_by = $2 WHERE id = $1;

-- name: CreateClass :one
INSERT INTO classes (school_id, name, join_code, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListClassesForTeacher :many
SELECT c.* FROM classes c
JOIN enrollments e ON e.class_id = c.id
WHERE e.user_id = $1 AND e.role_in_class = 'teacher'
ORDER BY c.name;

-- name: ListClassesBySchool :many
SELECT * FROM classes WHERE school_id = $1 ORDER BY name;

-- name: GetUserByIDInSchool :one
SELECT * FROM users WHERE id = $1 AND school_id = $2;

-- name: GetClassRoster :many
SELECT u.id, u.display_name, u.email,
       MAX(t.last_active_at) AS last_active_at,
       COUNT(DISTINCT t.id)  AS task_count,
       COUNT(DISTINCT ev.id) AS evaluation_count,
       COUNT(DISTINCT ci.id) AS card_count
FROM enrollments e
JOIN users u             ON u.id = e.user_id
LEFT JOIN tasks t        ON t.user_id = u.id
LEFT JOIN evaluations ev ON ev.task_id = t.id
LEFT JOIN card_instances ci ON ci.task_id = t.id
WHERE e.class_id = $1 AND e.role_in_class = 'student'
GROUP BY u.id, u.display_name, u.email
ORDER BY u.display_name;

-- name: UpdateClassName :one
UPDATE classes SET name = $2 WHERE id = $1 RETURNING *;

-- name: SetClassJoinCode :one
UPDATE classes SET join_code = $2 WHERE id = $1 RETURNING *;

-- name: DeleteEnrollment :execrows
DELETE FROM enrollments WHERE class_id = $1 AND user_id = $2 AND role_in_class = 'student';

-- name: GetClassBySchoolAndName :one
SELECT * FROM classes WHERE school_id = $1 AND name = $2;

-- name: GetSchoolCounts :one
SELECT
  (SELECT count(*) FROM users         WHERE users.school_id = $1 AND users.role = 'student')   AS student_count,
  (SELECT count(*) FROM users         WHERE users.school_id = $1 AND users.role = 'teacher')   AS teacher_count,
  (SELECT count(*) FROM classes       WHERE classes.school_id = $1)                            AS class_count,
  (SELECT count(*) FROM tasks t JOIN users u ON u.id = t.user_id WHERE u.school_id = $1)       AS task_count,
  (SELECT count(*) FROM evaluations e JOIN tasks t ON t.id = e.task_id JOIN users u ON u.id = t.user_id WHERE u.school_id = $1) AS evaluation_count,
  (SELECT count(DISTINCT t.user_id) FROM tasks t JOIN users u ON u.id = t.user_id WHERE u.school_id = $1) AS active_student_count;

-- name: GetSchoolUsageByTier :many
SELECT COALESCE(tier, 'unknown') AS tier,
       COALESCE(SUM(prompt_tokens), 0)::bigint     AS prompt_tokens,
       COALESCE(SUM(completion_tokens), 0)::bigint AS completion_tokens,
       COALESCE(SUM(cost_estimate), 0)::numeric    AS cost
FROM llm_usage
WHERE school_id = $1
GROUP BY tier
ORDER BY tier;

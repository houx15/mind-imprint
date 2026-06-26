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

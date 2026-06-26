-- name: GetClassByID :one
SELECT * FROM classes WHERE id = $1;

-- name: GetEnrollment :one
SELECT * FROM enrollments WHERE user_id = $1 AND class_id = $2;

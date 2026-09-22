-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetClassByJoinCode :one
SELECT * FROM classes WHERE join_code = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color, email_verified_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: CreateEnrollment :one
INSERT INTO enrollments (user_id, class_id, role_in_class)
VALUES ($1, $2, $3)
RETURNING *;

-- name: MarkEmailVerified :one
UPDATE users SET email_verified_at = now() WHERE id = $1 RETURNING *;

-- name: CreateEmailVerificationToken :one
INSERT INTO email_verification_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetActiveEmailVerificationToken :one
SELECT * FROM email_verification_tokens
WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > now();

-- name: ConsumeEmailVerificationToken :exec
UPDATE email_verification_tokens SET consumed_at = now() WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at, user_agent, ip)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSessionWithUserByHash :one
SELECT u.id, u.school_id, u.role, u.display_name
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > now();

-- name: DeleteSessionByHash :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: GetSchool :one
SELECT * FROM schools WHERE id = $1;

-- name: ListClassesForUser :many
SELECT c.id, c.name, e.role_in_class
FROM enrollments e
JOIN classes c ON c.id = e.class_id
WHERE e.user_id = $1
ORDER BY c.name;

-- name: ListClassGradesForUser :many
SELECT c.grade FROM enrollments e
JOIN classes c ON c.id = e.class_id
WHERE e.user_id = $1 AND e.role_in_class = 'student';

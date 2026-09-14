-- name: CreateLiteAssignment :one
INSERT INTO lite_assignment (class_id, created_by, kind, title, instructions, payload, due_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: AddLiteAssignmentRecipient :exec
INSERT INTO lite_assignment_recipient (assignment_id, user_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: GetLiteAssignment :one
SELECT * FROM lite_assignment WHERE id = $1;

-- name: ListLiteAssignmentsByClass :many
-- 带每种状态的计数所需的原始列；状态在 Go 里推。
SELECT a.*,
       COALESCE((SELECT count(*) FROM lite_assignment_recipient r WHERE r.assignment_id = a.id), 0)::int AS recipient_count
FROM lite_assignment a
WHERE a.class_id = $1 AND a.archived_at IS NULL
ORDER BY a.due_at DESC;

-- name: ListLiteAssignmentRecipients :many
-- 一份作业的每个学生，连同她那一项的完成时间。
SELECT r.assignment_id, r.user_id, r.seen_at, r.atom_id, r.started_at,
       u.display_name, u.avatar_color,
       COALESCE(rd.finished_at, w.finished_at, p.finished_at) AS finished_at
FROM lite_assignment_recipient r
JOIN users u ON u.id = r.user_id
LEFT JOIN reading rd ON rd.atom_id = r.atom_id
LEFT JOIN writing w ON w.atom_id = r.atom_id
LEFT JOIN pbl_project p ON p.atom_id = r.atom_id
WHERE r.assignment_id = ANY(sqlc.arg(assignment_ids)::uuid[])
ORDER BY u.display_name;

-- name: GetLiteAssignmentRecipientForUpdate :one
SELECT * FROM lite_assignment_recipient WHERE assignment_id = $1 AND user_id = $2 FOR UPDATE;

-- name: GetLiteAssignmentRecipient :one
SELECT * FROM lite_assignment_recipient WHERE assignment_id = $1 AND user_id = $2;

-- name: SetLiteAssignmentStarted :exec
UPDATE lite_assignment_recipient
SET atom_id = $3, started_at = now(), seen_at = COALESCE(seen_at, now())
WHERE assignment_id = $1 AND user_id = $2;

-- name: MarkLiteAssignmentSeen :exec
UPDATE lite_assignment_recipient SET seen_at = COALESCE(seen_at, now())
WHERE assignment_id = $1 AND user_id = $2;

-- name: RemoveLiteAssignmentRecipient :execrows
DELETE FROM lite_assignment_recipient
WHERE assignment_id = $1 AND user_id = $2 AND atom_id IS NULL AND started_at IS NULL;

-- name: CountStartedLiteAssignmentRecipients :one
SELECT count(*)::int FROM lite_assignment_recipient WHERE assignment_id = $1 AND started_at IS NOT NULL;

-- name: UpdateLiteAssignment :one
UPDATE lite_assignment
SET title = $2, instructions = $3, due_at = $4, kind = $5, payload = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveLiteAssignment :exec
UPDATE lite_assignment SET archived_at = COALESCE(archived_at, now()), updated_at = now() WHERE id = $1;

-- name: ListLiteInboxAssignments :many
-- 她的作业，未读在前，其后按截止时间。
SELECT a.id, a.kind, a.title, a.instructions, a.payload, a.due_at, a.created_at,
       r.seen_at, r.atom_id, r.started_at,
       c.name AS class_name,
       COALESCE(rd.finished_at, w.finished_at, p.finished_at) AS finished_at
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
JOIN classes c ON c.id = a.class_id
LEFT JOIN reading rd ON rd.atom_id = r.atom_id
LEFT JOIN writing w ON w.atom_id = r.atom_id
LEFT JOIN pbl_project p ON p.atom_id = r.atom_id
WHERE r.user_id = $1 AND a.archived_at IS NULL
ORDER BY (r.seen_at IS NULL) DESC, a.due_at ASC;

-- name: GetLiteAssignmentForAtom :one
SELECT a.id, a.title, a.due_at
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
WHERE r.atom_id = $1 AND r.user_id = $2 AND a.archived_at IS NULL;

-- name: IsEnrolledStudent :one
SELECT EXISTS (SELECT 1 FROM enrollments WHERE class_id = $1 AND user_id = $2 AND role_in_class = 'student')::bool;

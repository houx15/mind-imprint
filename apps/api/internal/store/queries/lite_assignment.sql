-- name: CreateLiteAssignment :one
INSERT INTO lite_assignment (class_id, created_by, kind, title, instructions, payload, due_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: AddLiteAssignmentRecipient :exec
INSERT INTO lite_assignment_recipient (assignment_id, user_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: GetLiteAssignment :one
SELECT * FROM lite_assignment WHERE id = $1;

-- name: GetLiteAssignmentForShare :one
-- 学生「开始」时读作业：挡住同一时刻教师改设置。
SELECT * FROM lite_assignment WHERE id = $1 FOR SHARE;

-- name: GetLiteAssignmentForUpdate :one
-- 教师改作业时先锁住它，等进行中的「开始」落定后再数已开始的人数。
SELECT * FROM lite_assignment WHERE id = $1 FOR UPDATE;

-- name: ListLiteAssignmentsByClass :many
-- 带每种状态的计数所需的原始列；状态在 Go 里推。
SELECT a.*,
       COALESCE((SELECT count(*) FROM lite_assignment_recipient r WHERE r.assignment_id = a.id), 0)::int AS recipient_count
FROM lite_assignment a
WHERE a.class_id = $1 AND a.archived_at IS NULL
ORDER BY a.due_at DESC;

-- name: ListLiteAssignmentRecipients :many
-- 一份作业的每个学生，连同她那一项的完成时间、退回信息和提交版本数。
-- resubmitted：退回之后提交过新版本（returned_at 为 NULL 时比较结果为 NULL，EXISTS 为 false）。
SELECT r.assignment_id, r.user_id, r.seen_at, r.atom_id, r.started_at,
       r.returned_at, r.return_due_at, r.return_note,
       u.display_name, u.avatar_color,
       COALESCE(rd.finished_at, w.finished_at, p.finished_at) AS finished_at,
       COALESCE((SELECT count(*) FROM writing_version v WHERE v.atom_id = r.atom_id), 0)::int AS version_count,
       EXISTS (SELECT 1 FROM writing_version v
               WHERE v.atom_id = r.atom_id AND v.submitted_at > r.returned_at)::bool AS resubmitted,
       -- 老师看「做得多深」：学习时长、阅读步骤完成数、最新提交版本的字数。
       COALESCE(at.active_seconds, 0)::int AS active_seconds,
       (SELECT count(*) FROM reading_task t WHERE t.atom_id = r.atom_id AND t.status = 'done')::int AS steps_done,
       (SELECT count(*) FROM reading_task t WHERE t.atom_id = r.atom_id)::int AS steps_total,
       (SELECT count(*) FROM atom_card c WHERE c.atom_id = r.atom_id AND c.status = 'submitted')::int AS cards_submitted,
       COALESCE((SELECT v.word_count FROM writing_version v WHERE v.atom_id = r.atom_id
                 ORDER BY v.number DESC LIMIT 1), 0)::int AS latest_word_count,
       -- 待批改：最新提交版本还没有已发送的批改。
       EXISTS (SELECT 1 FROM writing_version v
               WHERE v.atom_id = r.atom_id
                 AND v.number = (SELECT max(x.number) FROM writing_version x WHERE x.atom_id = r.atom_id)
                 AND NOT EXISTS (SELECT 1 FROM lite_grading g WHERE g.version_id = v.id AND g.status = 'sent'))::bool AS to_grade
FROM lite_assignment_recipient r
JOIN users u ON u.id = r.user_id
LEFT JOIN atom at ON at.id = r.atom_id
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
       r.returned_at, r.return_due_at, r.return_note,
       c.name AS class_name,
       COALESCE(rd.finished_at, w.finished_at, p.finished_at) AS finished_at,
       (SELECT count(*) FROM reading_task t WHERE t.atom_id = r.atom_id AND t.status = 'done')::int AS steps_done,
       (SELECT count(*) FROM reading_task t WHERE t.atom_id = r.atom_id)::int AS steps_total,
       EXISTS (SELECT 1 FROM writing_version v
               WHERE v.atom_id = r.atom_id AND v.submitted_at > r.returned_at)::bool AS resubmitted
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
JOIN classes c ON c.id = a.class_id
LEFT JOIN reading rd ON rd.atom_id = r.atom_id
LEFT JOIN writing w ON w.atom_id = r.atom_id
LEFT JOIN pbl_project p ON p.atom_id = r.atom_id
WHERE r.user_id = $1 AND a.archived_at IS NULL
  -- 她离开了这个班，这个班的作业就不再出现。
  AND EXISTS (SELECT 1 FROM enrollments e
              WHERE e.class_id = a.class_id AND e.user_id = r.user_id AND e.role_in_class = 'student')
ORDER BY (r.seen_at IS NULL) DESC, a.due_at ASC;

-- name: GetLiteAssignmentForAtom :one
-- 写作的锁定判断也读这一条：归档的作业不算。
-- 🚨 选出来的列是两条线的并集（2026-09-16）：退回修改那几列来自已经上线的
-- 教师端，instructions 来自作业材料那条线。少一列，对面那个功能就在这一个
-- 端点上静默失效。
SELECT a.id, a.kind, a.title, a.due_at, a.instructions,
       r.returned_at, r.return_due_at, r.return_note,
       EXISTS (SELECT 1 FROM writing_version v
               WHERE v.atom_id = r.atom_id AND v.submitted_at > r.returned_at)::bool AS resubmitted
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
WHERE r.atom_id = $1 AND r.user_id = $2 AND a.archived_at IS NULL;

-- name: IsEnrolledStudent :one
SELECT EXISTS (SELECT 1 FROM enrollments WHERE class_id = $1 AND user_id = $2 AND role_in_class = 'student')::bool;

-- name: SetLiteAssignmentReturned :one
-- 退回修改。再次退回时覆盖四列，其中 seen_at 清空，使这份作业在学生收件箱里
-- 重新计入未读；她此前可能已经打开过它，退回是需要她重新看到的新事件。
UPDATE lite_assignment_recipient
SET returned_at = now(), return_due_at = $3, return_note = $4, seen_at = NULL
WHERE assignment_id = $1 AND user_id = $2
RETURNING *;

-- name: GetPblAssignmentInstructions :one
-- Archived assignments still describe the constraints of an existing project.
SELECT a.instructions
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
WHERE r.atom_id = $1 AND r.user_id = $2 AND a.kind = 'project';

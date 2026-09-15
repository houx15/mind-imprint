-- Lite AI 批改（0154）。

-- name: CreateLiteGrading :one
-- 一个版本只有一行：已有一行时不插入，返回 no rows。
INSERT INTO lite_grading (atom_id, version_id, user_id, class_id, assignment_id, rubric, requested_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (version_id) DO NOTHING
RETURNING *;

-- name: GetLiteGrading :one
SELECT * FROM lite_grading WHERE id = $1;

-- name: GetLiteGradingByVersion :one
SELECT * FROM lite_grading WHERE version_id = $1;

-- name: ListLiteGradingsForVersions :many
SELECT * FROM lite_grading WHERE version_id = ANY(sqlc.arg(version_ids)::uuid[]);

-- name: ListLatestWritingVersionsForAtoms :many
-- 每篇写作的最新提交版本，不带正文。
SELECT DISTINCT ON (atom_id) id, atom_id, number, submitted_at
FROM writing_version
WHERE atom_id = ANY(sqlc.arg(atom_ids)::uuid[])
ORDER BY atom_id, number DESC;

-- name: GetLiteGradingSource :one
-- 批改读的那一版正文，以及写作的语言、老师的题目和目标字数。
SELECT v.id AS version_id, v.atom_id, v.number, v.title, v.body,
       w.lang, w.assigned_prompt, w.target_words
FROM writing_version v
JOIN writing w ON w.atom_id = v.atom_id
WHERE v.id = $1;

-- name: ClaimLiteGrading :one
-- 只有排队中的行可以开始，同一行不会被两个任务同时批改。
UPDATE lite_grading SET status = 'running', error = NULL, updated_at = now()
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: SetLiteGradingDraft :execrows
-- 批改成功：ai 与 content 都是这次的结果；之前的审阅作废。
UPDATE lite_grading
SET status = 'draft', ai = sqlc.arg(result), content = sqlc.arg(result),
    reviewed_at = NULL, error = NULL, updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'running';

-- name: SetLiteGradingFailed :execrows
-- 失败不丢掉已有草稿：这一行如果已经有内容（重新批改失败），退回 draft 并保留
-- 旧的 ai/content/reviewed_at，只把这次的失败原因写进 error；第一次批改（还没有
-- 内容）才真正落到 failed。
UPDATE lite_grading
SET status = CASE WHEN content IS NOT NULL THEN 'draft' ELSE 'failed' END,
    error = sqlc.arg(error), updated_at = now()
WHERE id = sqlc.arg(id) AND status IN ('queued', 'running');

-- name: RequeueLiteGrading :one
-- 重新批改：草稿或失败的行回到排队，评分标准换成当前的。已发送的行不重新批改。
UPDATE lite_grading
SET status = 'queued', error = NULL, rubric = $2, requested_by = $3, updated_at = now()
WHERE id = $1 AND status IN ('draft', 'failed')
RETURNING *;

-- name: MarkStaleLiteGradingsFailed :exec
-- 任务只跑一次（MaxAttempts 1），超时 6 分钟。进程在批改中途退出时这一行会停在 running，
-- 超过 15 分钟没有更新就判为失败；已有内容的行（重新批改途中卡住）退回 draft 并保留
-- 内容，只标错误。排队中的行不动：队列可能还没轮到它。
UPDATE lite_grading
SET status = CASE WHEN content IS NOT NULL THEN 'draft' ELSE 'failed' END,
    error = '批改超时：任务未完成', updated_at = now()
WHERE status = 'running' AND updated_at < now() - interval '15 minutes'
  AND id = ANY(sqlc.arg(ids)::uuid[]);

-- name: UpdateLiteGradingContent :one
-- 老师保存即算审阅，并清掉上一次的失败原因。已发送的行保存后重新发送：学生那边变为未读。
UPDATE lite_grading
SET content = COALESCE(sqlc.narg(content)::jsonb, content),
    reviewed_at = now(),
    error = NULL,
    sent_at = CASE WHEN status = 'sent' THEN now() ELSE sent_at END,
    student_seen_at = CASE WHEN status = 'sent' THEN NULL ELSE student_seen_at END,
    updated_at = now()
WHERE id = sqlc.arg(id) AND status IN ('draft', 'sent') AND content IS NOT NULL
RETURNING *;

-- name: SendLiteGrading :one
UPDATE lite_grading
SET status = 'sent', sent_at = now(), student_seen_at = NULL,
    reviewed_at = COALESCE(reviewed_at, now()), updated_at = now()
WHERE id = $1 AND status IN ('draft', 'sent') AND content IS NOT NULL
RETURNING *;

-- name: SendReviewedLiteGradings :execrows
-- 发送全部已审阅：只发这份作业里已审阅的草稿。
UPDATE lite_grading
SET status = 'sent', sent_at = now(), student_seen_at = NULL, updated_at = now()
WHERE assignment_id = sqlc.arg(assignment_id) AND id = ANY(sqlc.arg(ids)::uuid[])
  AND status = 'draft' AND reviewed_at IS NOT NULL AND content IS NOT NULL;

-- name: ListSentLiteGradingsForAtom :many
-- 学生读的批改：只有已发送的行。
SELECT g.id, g.rubric, g.content, g.sent_at, g.student_seen_at, v.number AS version_number
FROM lite_grading g
JOIN writing_version v ON v.id = g.version_id
WHERE g.atom_id = $1 AND g.status = 'sent'
ORDER BY v.number DESC;

-- name: ListLiteInboxGradings :many
SELECT g.id, g.atom_id, g.sent_at, g.student_seen_at, w.title
FROM lite_grading g
JOIN writing w ON w.atom_id = g.atom_id
WHERE g.user_id = $1 AND g.status = 'sent'
ORDER BY g.sent_at DESC;

-- name: MarkLiteGradingSeen :execrows
UPDATE lite_grading SET student_seen_at = COALESCE(student_seen_at, now())
WHERE id = $1 AND user_id = $2 AND status = 'sent';

-- Lite AI 批改（0154）。

-- name: CreateLiteGrading :one
-- 一个版本只有一行：已有一行时不插入，返回 no rows。
INSERT INTO lite_grading (atom_id, version_id, user_id, class_id, assignment_id, rubric, requested_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (version_id) DO NOTHING
RETURNING *;

-- name: CreateManualLiteGrading :one
-- 人工批改：老师自己写，不调用模型，所以直接落成草稿。content 是按评分标准
-- 生成的空白表（等级和评语都为空），不能为 NULL —— UpdateLiteGradingContent
-- 要求 content IS NOT NULL，否则老师第一次保存就会被拒。ai 保持为 NULL，
-- 学生那边据此显示这份批改不是 AI 起草的。
-- 一个版本只有一行：已有一行时不插入，返回 no rows。
INSERT INTO lite_grading (atom_id, version_id, user_id, class_id, assignment_id, rubric, requested_by, status, content)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'draft', sqlc.arg(content)::jsonb)
ON CONFLICT (version_id) DO NOTHING
RETURNING *;

-- name: StartManualOnFailedLiteGrading :one
-- AI 批改失败（没有内容）之后改为人工批改：同一行变成老师的空白草稿。
-- 否则老师在 AI 连续失败时既不能人工批改（这一版已有一行），也改不了这一行。
UPDATE lite_grading
SET status = 'draft', content = sqlc.arg(content)::jsonb, ai = NULL, error = NULL,
    requested_by = sqlc.arg(requested_by), updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'failed' AND content IS NULL
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
-- 批改成功：ai 与 content 都是这次的结果；之前的审阅作废。rubric 写成这次批改
-- 用的评分标准，与 content 一起更新（重新批改排队时不改 rubric，见 RequeueLiteGrading）。
-- rubric 为 NULL 时保留原值。
UPDATE lite_grading
SET status = 'draft', ai = sqlc.arg(result), content = sqlc.arg(result),
    rubric = COALESCE(sqlc.narg(rubric)::jsonb, rubric),
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
-- 重新批改：草稿或失败的行回到排队。已发送的行不重新批改。
-- 已有内容的行不在这里改 rubric：批改失败时这一行退回 draft 并保留旧内容，
-- rubric 必须仍是描述旧内容的那一份，否则老师保存时维度对不上。新的评分标准
-- 随任务参数传给 worker，批改成功时由 SetLiteGradingDraft 与内容一起写入。
-- 没有内容的行（第一次批改失败）直接换成新的评分标准。
UPDATE lite_grading
SET status = 'queued', error = NULL,
    rubric = CASE WHEN content IS NULL THEN sqlc.arg(rubric)::jsonb ELSE rubric END,
    requested_by = sqlc.arg(requested_by), updated_at = now()
WHERE id = sqlc.arg(id) AND status IN ('draft', 'failed')
RETURNING *;

-- name: LiteTeacherSeesStudent :one
-- 调用者是否教这名学生当前所在的某个班（学生身份）；学校管理员看本校班级里的学生。
-- 用于不属于任何作业的批改的可见性。
SELECT EXISTS (
  SELECT 1
  FROM enrollments s
  JOIN classes c ON c.id = s.class_id
  WHERE s.user_id = sqlc.arg(student_id) AND s.role_in_class = 'student'
    AND (
      EXISTS (
        SELECT 1 FROM enrollments t
        WHERE t.class_id = s.class_id AND t.user_id = sqlc.arg(caller_id) AND t.role_in_class = 'teacher'
      )
      OR (sqlc.arg(caller_is_admin)::bool AND c.school_id = sqlc.arg(caller_school_id)::uuid)
    )
)::bool;

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

-- name: SaveLiteGradingDraft :one
-- 暂存未完成的批改，不标记审阅，也不向学生发送。
UPDATE lite_grading
SET content = sqlc.arg(content)::jsonb, reviewed_at = NULL,
    error = NULL, updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'draft' AND content IS NOT NULL
RETURNING *;

-- name: SendLiteGrading :one
UPDATE lite_grading
SET status = 'sent', sent_at = now(), student_seen_at = NULL,
    reviewed_at = COALESCE(reviewed_at, now()), updated_at = now()
WHERE id = $1 AND status IN ('draft', 'sent') AND content IS NOT NULL
RETURNING *;

-- name: SendReviewedLiteGradings :execrows
-- 发送全部已审阅：只发这份作业里已审阅的草稿，且学生仍在班里
-- （离班学生的草稿即便已审阅也不发送——她已经不是这个老师的学生了）。
UPDATE lite_grading g
SET status = 'sent', sent_at = now(), student_seen_at = NULL, updated_at = now()
WHERE g.assignment_id = sqlc.arg(assignment_id) AND g.id = ANY(sqlc.arg(ids)::uuid[])
  AND g.status = 'draft' AND g.reviewed_at IS NOT NULL AND g.content IS NOT NULL
  AND EXISTS (
    SELECT 1 FROM enrollments e
    WHERE e.class_id = g.class_id AND e.user_id = g.user_id AND e.role_in_class = 'student'
  );

-- name: ListSentLiteGradingsForAtom :many
-- 学生读的批改：只有已发送的行。
SELECT g.id, g.rubric, g.content, g.sent_at, g.student_seen_at, v.number AS version_number,
       (g.ai IS NOT NULL)::bool AS ai_drafted
FROM lite_grading g
JOIN writing_version v ON v.id = g.version_id
WHERE g.atom_id = $1 AND g.status = 'sent'
ORDER BY v.number DESC;

-- name: ListLiteInboxGradings :many
-- g.id 是 sent_at 相同时（发送全部已审阅一次发出多行）的次序打散。
SELECT g.id, g.atom_id, g.sent_at, g.student_seen_at, w.title
FROM lite_grading g
JOIN writing w ON w.atom_id = g.atom_id
WHERE g.user_id = $1 AND g.status = 'sent'
ORDER BY g.sent_at DESC, g.id DESC;

-- name: MarkLiteGradingSeen :execrows
UPDATE lite_grading SET student_seen_at = COALESCE(student_seen_at, now())
WHERE id = $1 AND user_id = $2 AND status = 'sent';

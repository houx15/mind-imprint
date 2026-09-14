-- 教师端（lite）家长报告：老师为一名学生、一段日期生成。
--
-- 日期范围两端都含，按北京日期算。事实查询收的边界与周总结（lite_weekly.sql）同一个形状：
--   range_start / range_end：timestamptz，[开始日 00:00 北京, 结束日次日 00:00 北京)；
--   start_day / end_day：同一时刻的北京日期，[开始日, 结束日次日)。
--
-- 「完成」的判定、标题兜底、金句来源、作业状态都照抄 lite_weekly.sql：
--   reading / writing：status = 'finished'，完成时间取 finished_at；project：pbl_project.finished_at。
--   标题：她的标题 → 项目名 → 她自己开的项目的 idea → 布置这一项的作业标题 → 种类名，
--   永远不是空串。lite_assignment_recipient.atom_id 是 UNIQUE，关联不会多出行。
-- 老师写的字不进事实：从不读 writing.assigned_prompt 作标题或金句（只在 Go 侧用来剔除与题目
-- 重合的句子），老师布置的项目（assigned = true）的 idea 不作标题兜底。
-- 只取产出与计数，不取对话正文。

-- name: CreateLiteParentReport :one
INSERT INTO lite_parent_report (user_id, class_id, created_by, range_start, range_end, facts)
VALUES (sqlc.arg(user_id), sqlc.arg(class_id), sqlc.arg(created_by), sqlc.arg(range_start), sqlc.arg(range_end), sqlc.arg(facts))
RETURNING *;

-- name: SetLiteParentReportDraft :one
-- 写入模型草稿。body 只在还是 NULL 时取草稿（第一次生成），老师改过的文字不被覆盖。
-- 已发布的报告不再改草稿：status 条件与调用方加锁后的检查一致。
UPDATE lite_parent_report
SET draft = sqlc.arg(draft)::jsonb,
    body = COALESCE(body, sqlc.arg(draft)::jsonb),
    updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'draft'
RETURNING *;

-- name: ReplaceLiteParentReportBody :one
-- 老师明确要求重新生成并覆盖：草稿与 body 都换成新草稿。
UPDATE lite_parent_report
SET draft = sqlc.arg(draft)::jsonb,
    body = sqlc.arg(draft)::jsonb,
    updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'draft'
RETURNING *;

-- name: UpdateLiteParentReportBody :one
-- 老师编辑文字。发布之后也可以改，公开页立即显示改后的文字。
UPDATE lite_parent_report
SET body = sqlc.arg(body)::jsonb, updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: GetLiteParentReport :one
SELECT * FROM lite_parent_report WHERE id = sqlc.arg(id);

-- name: GetLiteParentReportForUpdate :one
-- 所有改动报告的路由先用这一条锁住报告行。
SELECT * FROM lite_parent_report WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: ListLiteParentReportsByStudent :many
SELECT pr.id, pr.user_id, u.display_name AS student_name, pr.class_id,
       pr.range_start, pr.range_end, pr.status, pr.share_token, pr.published_at, pr.created_at
FROM lite_parent_report pr
JOIN users u ON u.id = pr.user_id
WHERE pr.user_id = sqlc.arg(user_id) AND pr.class_id = sqlc.arg(class_id)
ORDER BY pr.created_at DESC, pr.id;

-- name: ListLiteParentReportsByClass :many
SELECT pr.id, pr.user_id, u.display_name AS student_name, pr.class_id,
       pr.range_start, pr.range_end, pr.status, pr.share_token, pr.published_at, pr.created_at
FROM lite_parent_report pr
JOIN users u ON u.id = pr.user_id
WHERE pr.class_id = sqlc.arg(class_id)
ORDER BY pr.created_at DESC, pr.id;

-- name: PublishLiteParentReport :one
-- 已有链接时保留原链接；撤销（share_token 置空）之后再发布，COALESCE 取新传入的链接。
-- published_at 记第一次发布的时刻。
UPDATE lite_parent_report
SET status = 'published',
    share_token = COALESCE(share_token, sqlc.arg(share_token)::text),
    published_at = COALESCE(published_at, now()),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: RevokeLiteParentReportShare :one
-- 撤销链接：公开页下一次请求即 404。报告仍是已发布，学生在应用内仍可查看。
UPDATE lite_parent_report
SET share_token = NULL, updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: GetLiteParentReportByToken :one
-- 公开路由唯一读的那一条。share_token = NULL 永远不成立，撤销后的报告读不到。
SELECT * FROM lite_parent_report
WHERE share_token = sqlc.arg(share_token)::text AND status = 'published';

-- name: GetStudentParentReport :one
-- 学生读自己的报告：只看归属与发布状态，不看是否仍在班（离开班级后报告仍是她的）。
SELECT * FROM lite_parent_report
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND status = 'published';

-- name: MarkParentReportSeen :exec
UPDATE lite_parent_report
SET student_seen_at = COALESCE(student_seen_at, now())
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND status = 'published';

-- name: ListStudentPublishedParentReports :many
SELECT pr.id, pr.class_id, c.name AS class_name, pr.range_start, pr.range_end,
       pr.published_at, pr.student_seen_at
FROM lite_parent_report pr
JOIN classes c ON c.id = pr.class_id
WHERE pr.user_id = sqlc.arg(user_id) AND pr.status = 'published'
ORDER BY pr.published_at DESC, pr.id;

-- name: ParentRangeActivity :one
-- 活跃天数：秒数 > 0 的日格 ∪ 她发消息的北京日期（与 ListLiteWeekActivity 同一个形状）。
-- has_bucket_in_range = false 时 Go 侧把学习时长记为 -1（界面显示无记录）。
SELECT
  (SELECT count(DISTINCT x.day) FROM (
     SELECT d.day FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
      WHERE a.user_id = sqlc.arg(user_id)::uuid AND d.seconds > 0
        AND d.day >= sqlc.arg(start_day)::date AND d.day < sqlc.arg(end_day)::date
     UNION
     SELECT (m.created_at AT TIME ZONE 'Asia/Shanghai')::date FROM atom_message m JOIN atom a ON a.id = m.atom_id
      WHERE a.user_id = sqlc.arg(user_id)::uuid AND m.role = 'student'
        AND m.created_at >= sqlc.arg(range_start)::timestamptz AND m.created_at < sqlc.arg(range_end)::timestamptz
   ) x)::int AS active_days,
  COALESCE((SELECT sum(d.seconds) FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
     WHERE a.user_id = sqlc.arg(user_id)::uuid
       AND d.day >= sqlc.arg(start_day)::date AND d.day < sqlc.arg(end_day)::date), 0)::int AS seconds,
  EXISTS (SELECT 1 FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
     WHERE a.user_id = sqlc.arg(user_id)::uuid
       AND d.day >= sqlc.arg(start_day)::date AND d.day < sqlc.arg(end_day)::date) AS has_bucket_in_range,
  (SELECT count(*) FROM atom_message m JOIN atom a ON a.id = m.atom_id
     WHERE a.user_id = sqlc.arg(user_id)::uuid AND m.role = 'student'
       AND m.created_at >= sqlc.arg(range_start)::timestamptz AND m.created_at < sqlc.arg(range_end)::timestamptz)::int AS turns;

-- name: ParentRangeFinished :many
-- 范围内完成的阅读、写作、项目，按完成时间排序。
-- atom_id 供 Go 侧为还没有报告的阅读 / 写作先生成报告（只跑第一阶段，不调用模型）。
SELECT a.id AS atom_id, a.kind,
       COALESCE(
         NULLIF(btrim(COALESCE(r.title, w.title, NULLIF(p.name, ''), CASE WHEN p.assigned THEN NULL ELSE p.idea END, '')), ''),
         NULLIF(btrim(la.title), ''),
         CASE a.kind WHEN 'reading' THEN '阅读' WHEN 'writing' THEN '写作' ELSE '项目' END
       )::text AS title,
       COALESCE(r.finished_at, w.finished_at, p.finished_at)::timestamptz AS finished_at
FROM atom a
LEFT JOIN reading r ON r.atom_id = a.id
LEFT JOIN writing w ON w.atom_id = a.id
LEFT JOIN pbl_project p ON p.atom_id = a.id
LEFT JOIN lite_assignment_recipient lr ON lr.atom_id = a.id
LEFT JOIN lite_assignment la ON la.id = lr.assignment_id
WHERE a.user_id = sqlc.arg(user_id)::uuid
  AND (
    (a.kind = 'reading' AND r.status = 'finished' AND r.finished_at >= sqlc.arg(range_start)::timestamptz AND r.finished_at < sqlc.arg(range_end)::timestamptz)
    OR (a.kind = 'writing' AND w.status = 'finished' AND w.finished_at >= sqlc.arg(range_start)::timestamptz AND w.finished_at < sqlc.arg(range_end)::timestamptz)
    OR (a.kind = 'project' AND p.finished_at >= sqlc.arg(range_start)::timestamptz AND p.finished_at < sqlc.arg(range_end)::timestamptz)
  )
ORDER BY COALESCE(r.finished_at, w.finished_at, p.finished_at), a.id;

-- name: ParentRangeMoments :many
-- 范围内完成的阅读 / 写作的报告里的金句（与 ListLiteWeekMoments 同一个来源）。
-- prose_pending = true 的报告文字还没生成，Go 侧跳过；moments 读不出来的报告记日志后跳过；
-- 空白金句跳过；assigned_prompt 只用来剔除与老师题目重合的句子，从不作为金句。
SELECT a.id AS atom_id,
       COALESCE(
         NULLIF(btrim(COALESCE(r.title, w.title, '')), ''),
         NULLIF(btrim(la.title), ''),
         CASE a.kind WHEN 'reading' THEN '阅读' ELSE '写作' END
       )::text AS title,
       COALESCE(ar.report -> 'moments', 'null'::jsonb)::jsonb AS moments,
       COALESCE((ar.report ->> 'prosePending')::boolean, false)::boolean AS prose_pending,
       w.assigned_prompt
FROM atom_report ar
JOIN atom a ON a.id = ar.atom_id
LEFT JOIN reading r ON r.atom_id = a.id
LEFT JOIN writing w ON w.atom_id = a.id
LEFT JOIN lite_assignment_recipient lr ON lr.atom_id = a.id
LEFT JOIN lite_assignment la ON la.id = lr.assignment_id
WHERE a.user_id = sqlc.arg(user_id)::uuid
  AND (
    (a.kind = 'reading' AND r.status = 'finished' AND r.finished_at >= sqlc.arg(range_start)::timestamptz AND r.finished_at < sqlc.arg(range_end)::timestamptz)
    OR (a.kind = 'writing' AND w.status = 'finished' AND w.finished_at >= sqlc.arg(range_start)::timestamptz AND w.finished_at < sqlc.arg(range_end)::timestamptz)
  )
ORDER BY COALESCE(r.finished_at, w.finished_at), a.id;

-- name: ParentRangeAssignmentStates :many
-- 截止时间落在范围内的作业（本班、到范围结束仍未归档）。状态在 Go 里用
-- liteassign.Status(…, now = range_end) 推出；范围结束之后才完成的记为已逾期。
SELECT r.started_at, a.due_at,
       COALESCE(rd.finished_at, w.finished_at, p.finished_at) AS finished_at
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
LEFT JOIN reading rd ON rd.atom_id = r.atom_id
LEFT JOIN writing w ON w.atom_id = r.atom_id
LEFT JOIN pbl_project p ON p.atom_id = r.atom_id
WHERE a.class_id = sqlc.arg(class_id)
  AND (a.archived_at IS NULL OR a.archived_at >= sqlc.arg(range_end)::timestamptz)
  AND r.user_id = sqlc.arg(user_id)
  AND a.due_at >= sqlc.arg(range_start)::timestamptz AND a.due_at < sqlc.arg(range_end)::timestamptz
ORDER BY a.due_at, a.id;

-- name: ParentRangeKeywords :many
-- 范围内第一次出现在她树上的词，按强度取前 12 个。
SELECT k.text_zh, k.field
FROM interest_keyword k
WHERE k.user_id = sqlc.arg(user_id)
  AND k.first_seen_at >= sqlc.arg(range_start)::timestamptz AND k.first_seen_at < sqlc.arg(range_end)::timestamptz
ORDER BY k.strength DESC, k.first_seen_at, k.id
LIMIT 12;

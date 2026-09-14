-- 教师端（lite）周总结的事实数据。只读；只数、只取产出，不取对话正文。
-- 一周 = [week_start, week_end)，北京时间周一 00:00 起。*_day 参数是同一时刻的北京日期。
-- 所有查询按 user_ids 批量取，Go 侧按 user_id 分组。
--
-- 「完成」的判定按种类（与 lite_teacher.sql 的名单一致）：
--   reading / writing：status = 'finished'，完成时间取 finished_at；
--   project：pbl_project.finished_at（第一次进入回顾或保留时写入）。
--
-- 标题：她的标题 → 项目名 → 她自己开的项目的 idea → 布置这一项的作业标题 → 种类名
-- （阅读 / 写作 / 项目），永远不是空串。老师布置的项目（assigned = true）的 idea 是老师的
-- 驱动问题，不作标题兜底。布置的写作题目在 writing.assigned_prompt，从不当作标题或金句。

-- name: ListLiteWeekClassStudents :many
-- 当前在班的学生（班内角色与账号角色都是 student），教师与管理员不在内。
SELECT u.id, u.display_name
FROM enrollments e
JOIN users u ON u.id = e.user_id
WHERE e.class_id = sqlc.arg(class_id) AND e.role_in_class = 'student' AND u.role = 'student'
ORDER BY u.display_name, u.id;

-- name: ListLiteWeekActivity :many
-- 活跃天数与名单查询（ListLiteClassRoster）同一个形状：秒数 > 0 的日格 ∪ 她发消息的北京日期。
-- has_any_bucket_by_week_end 区分「从没记过时长」（界面显示无记录）与「记过，这周是 0」。
SELECT u.id AS user_id,
       (SELECT count(DISTINCT x.day) FROM (
          SELECT d.day FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
           WHERE a.user_id = u.id AND d.seconds > 0 AND d.day >= sqlc.arg(week_start_day)::date AND d.day < sqlc.arg(week_end_day)::date
          UNION
          SELECT (m.created_at AT TIME ZONE 'Asia/Shanghai')::date FROM atom_message m JOIN atom a ON a.id = m.atom_id
           WHERE a.user_id = u.id AND m.role = 'student' AND m.created_at >= sqlc.arg(week_start)::timestamptz AND m.created_at < sqlc.arg(week_end)::timestamptz
        ) x)::int AS active_days,
       (SELECT count(DISTINCT x.day) FROM (
          SELECT d.day FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
           WHERE a.user_id = u.id AND d.seconds > 0 AND d.day >= sqlc.arg(prev_start_day)::date AND d.day < sqlc.arg(week_start_day)::date
          UNION
          SELECT (m.created_at AT TIME ZONE 'Asia/Shanghai')::date FROM atom_message m JOIN atom a ON a.id = m.atom_id
           WHERE a.user_id = u.id AND m.role = 'student' AND m.created_at >= sqlc.arg(prev_start)::timestamptz AND m.created_at < sqlc.arg(week_start)::timestamptz
        ) x)::int AS prev_active_days,
       COALESCE((SELECT sum(d.seconds) FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
         WHERE a.user_id = u.id AND d.day >= sqlc.arg(week_start_day)::date AND d.day < sqlc.arg(week_end_day)::date), 0)::int AS seconds,
       EXISTS (SELECT 1 FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
         WHERE a.user_id = u.id AND d.day < sqlc.arg(week_end_day)::date) AS has_any_bucket_by_week_end,
       (SELECT count(*) FROM atom_message m JOIN atom a ON a.id = m.atom_id
         WHERE a.user_id = u.id AND m.role = 'student' AND m.created_at >= sqlc.arg(week_start)::timestamptz AND m.created_at < sqlc.arg(week_end)::timestamptz)::int AS turns
FROM users u
WHERE u.id = ANY(sqlc.arg(user_ids)::uuid[]);

-- name: ListLiteWeekFinished :many
-- 这一周内完成的阅读、写作、项目，按完成时间排序。
SELECT a.user_id, a.kind,
       COALESCE(
         NULLIF(btrim(COALESCE(r.title, w.title, NULLIF(p.name, ''), CASE WHEN p.assigned THEN NULL ELSE p.idea END, '')), ''),
         NULLIF(btrim(la.title), ''),
         CASE a.kind WHEN 'reading' THEN '阅读' WHEN 'writing' THEN '写作' ELSE '项目' END
       )::text AS title
FROM atom a
LEFT JOIN reading r ON r.atom_id = a.id
LEFT JOIN writing w ON w.atom_id = a.id
LEFT JOIN pbl_project p ON p.atom_id = a.id
LEFT JOIN lite_assignment_recipient lr ON lr.atom_id = a.id
LEFT JOIN lite_assignment la ON la.id = lr.assignment_id
WHERE a.user_id = ANY(sqlc.arg(user_ids)::uuid[])
  AND (
    (a.kind = 'reading' AND r.status = 'finished' AND r.finished_at >= sqlc.arg(week_start)::timestamptz AND r.finished_at < sqlc.arg(week_end)::timestamptz)
    OR (a.kind = 'writing' AND w.status = 'finished' AND w.finished_at >= sqlc.arg(week_start)::timestamptz AND w.finished_at < sqlc.arg(week_end)::timestamptz)
    OR (a.kind = 'project' AND p.finished_at >= sqlc.arg(week_start)::timestamptz AND p.finished_at < sqlc.arg(week_end)::timestamptz)
  )
ORDER BY COALESCE(r.finished_at, w.finished_at, p.finished_at), a.id;

-- name: ListLiteWeekAssignmentStates :many
-- 截止时间落在这一周的作业（本班、未归档）。finished_at 的取法与 ListLiteClassRecipientStates 一致；
-- 状态在 Go 里用 liteassign.Status(…, now = week_end) 推出，周末之后才完成的记为已逾期。
SELECT r.user_id, r.started_at, a.due_at,
       COALESCE(rd.finished_at, w.finished_at, p.finished_at) AS finished_at
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
LEFT JOIN reading rd ON rd.atom_id = r.atom_id
LEFT JOIN writing w ON w.atom_id = r.atom_id
LEFT JOIN pbl_project p ON p.atom_id = r.atom_id
WHERE a.class_id = sqlc.arg(class_id) AND a.archived_at IS NULL
  AND r.user_id = ANY(sqlc.arg(user_ids)::uuid[])
  AND a.due_at >= sqlc.arg(week_start)::timestamptz AND a.due_at < sqlc.arg(week_end)::timestamptz;

-- name: ListLiteWeekStalled :many
-- 停滞：到周末仍未完成、在周末前 7 天之前就已创建、且周末前 7 天里没有任何活动
-- （没有秒数 > 0 的日格，也没有她发的消息）。只看周末之前的数据，不看 atom.last_activity_at，
-- 所以周末之后她再动这一项，也不会改写那一周的事实。
-- 「到周末已完成」：阅读 / 写作 status = 'finished' 且完成时间早于周末（没有 finished_at 的旧数据
-- 按早已完成）；项目 finished_at 早于周末，或没有 finished_at 但处于 review/keeping（旧项目）。
-- 已归档的项目不算停滞；归档没有时间戳，所以任何已归档的项目都排除。
-- 整个判定包在 COALESCE(…, false) 里：进行中的项目 finished_at 为 NULL，比较结果是 NULL，
-- 不包的话 NOT(NULL) 会把这一行丢掉。
SELECT a.user_id, a.kind,
       COALESCE(
         NULLIF(btrim(COALESCE(r.title, w.title, NULLIF(p.name, ''), CASE WHEN p.assigned THEN NULL ELSE p.idea END, '')), ''),
         NULLIF(btrim(la.title), ''),
         CASE a.kind WHEN 'reading' THEN '阅读' WHEN 'writing' THEN '写作' ELSE '项目' END
       )::text AS title
FROM atom a
LEFT JOIN reading r ON r.atom_id = a.id
LEFT JOIN writing w ON w.atom_id = a.id
LEFT JOIN pbl_project p ON p.atom_id = a.id
LEFT JOIN lite_assignment_recipient lr ON lr.atom_id = a.id
LEFT JOIN lite_assignment la ON la.id = lr.assignment_id
WHERE a.user_id = ANY(sqlc.arg(user_ids)::uuid[])
  AND a.kind IN ('reading','writing','project')
  AND a.created_at < sqlc.arg(week_end)::timestamptz - interval '7 days'
  AND NOT COALESCE(CASE a.kind
        WHEN 'reading' THEN r.status = 'finished' AND (r.finished_at IS NULL OR r.finished_at < sqlc.arg(week_end)::timestamptz)
        WHEN 'writing' THEN w.status = 'finished' AND (w.finished_at IS NULL OR w.finished_at < sqlc.arg(week_end)::timestamptz)
        WHEN 'project' THEN p.status = 'archived'
                            OR COALESCE(p.finished_at < sqlc.arg(week_end)::timestamptz, false)
                            OR (p.finished_at IS NULL AND p.status IN ('review','keeping'))
      END, false)
  AND NOT EXISTS (
    SELECT 1 FROM atom_active_day d
    WHERE d.atom_id = a.id AND d.seconds > 0
      AND d.day >= sqlc.arg(week_end_day)::date - 7 AND d.day < sqlc.arg(week_end_day)::date)
  AND NOT EXISTS (
    SELECT 1 FROM atom_message m
    WHERE m.atom_id = a.id AND m.role = 'student'
      AND m.created_at >= sqlc.arg(week_end)::timestamptz - interval '7 days' AND m.created_at < sqlc.arg(week_end)::timestamptz)
ORDER BY a.created_at, a.id;

-- name: ListLiteWeekNewKeywords :many
-- 这一周第一次出现在她树上的词（interest_keyword 里的词都是她认过的）。
SELECT k.user_id, k.text_zh
FROM interest_keyword k
WHERE k.user_id = ANY(sqlc.arg(user_ids)::uuid[])
  AND k.first_seen_at >= sqlc.arg(week_start)::timestamptz AND k.first_seen_at < sqlc.arg(week_end)::timestamptz
ORDER BY k.first_seen_at, k.id;

-- name: ListLiteWeekMoments :many
-- 这一周完成的阅读 / 写作的报告里的金句（atom_report 只有这两种）。金句在生成报告时
-- 已逐字核对过是她的原话，这里原样取出；prose_pending = true 的报告文字还没生成，Go 侧跳过。
-- assigned_prompt 只用来在 Go 侧剔除与老师题目重合的句子，从不作为金句。
SELECT a.id AS atom_id, a.user_id,
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
WHERE a.user_id = ANY(sqlc.arg(user_ids)::uuid[])
  AND (
    (a.kind = 'reading' AND r.status = 'finished' AND r.finished_at >= sqlc.arg(week_start)::timestamptz AND r.finished_at < sqlc.arg(week_end)::timestamptz)
    OR (a.kind = 'writing' AND w.status = 'finished' AND w.finished_at >= sqlc.arg(week_start)::timestamptz AND w.finished_at < sqlc.arg(week_end)::timestamptz)
  )
ORDER BY COALESCE(r.finished_at, w.finished_at), a.id;

-- name: InsertLiteStudentWeeklyProse :exec
INSERT INTO lite_student_weekly_prose (user_id, week_start, body)
VALUES (sqlc.arg(user_id), sqlc.arg(week_start), sqlc.arg(body))
ON CONFLICT (user_id, week_start) DO NOTHING;

-- name: GetLiteStudentWeeklyProse :one
SELECT * FROM lite_student_weekly_prose
WHERE user_id = sqlc.arg(user_id) AND week_start = sqlc.arg(week_start);

-- name: InsertLiteClassWeeklyProse :exec
INSERT INTO lite_class_weekly_prose (class_id, week_start, body)
VALUES (sqlc.arg(class_id), sqlc.arg(week_start), sqlc.arg(body))
ON CONFLICT (class_id, week_start) DO NOTHING;

-- name: GetLiteClassWeeklyProse :one
SELECT * FROM lite_class_weekly_prose
WHERE class_id = sqlc.arg(class_id) AND week_start = sqlc.arg(week_start);

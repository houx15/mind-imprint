-- 教师端（lite）周总结的事实数据。只读；只数、只取产出，不取对话正文。
-- 一周 = [week_start, week_end)，北京时间周一 00:00 起。*_day 参数是同一时刻的北京日期。
-- 所有查询按 user_ids 批量取，Go 侧按 user_id 分组。
--
-- 「完成」的判定按种类（与 lite_teacher.sql 的名单一致）：
--   reading / writing：status = 'finished'，完成时间取 finished_at；
--   project：pbl_project.finished_at（第一次进入回顾或保留时写入）。
--
-- 标题：老师布置的项目（assigned = true）的 idea 是老师的驱动问题，不作标题兜底。
-- 布置的写作题目在 writing.assigned_prompt，这里从不读它当作她的话。

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
       COALESCE(r.title, w.title, NULLIF(p.name, ''), CASE WHEN p.assigned THEN NULL ELSE p.idea END, '')::text AS title
FROM atom a
LEFT JOIN reading r ON r.atom_id = a.id
LEFT JOIN writing w ON w.atom_id = a.id
LEFT JOIN pbl_project p ON p.atom_id = a.id
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
-- 到周末仍未完成、且 7 天以上没有动过的一项。已归档的项目不算。
-- 状态是 finished 但没有 finished_at 的旧数据按「早已完成」处理；项目沿用名单的
-- review/keeping 判定兜底 finished_at 之前的旧项目。
SELECT a.user_id, a.kind,
       COALESCE(r.title, w.title, NULLIF(p.name, ''), CASE WHEN p.assigned THEN NULL ELSE p.idea END, '')::text AS title
FROM atom a
LEFT JOIN reading r ON r.atom_id = a.id
LEFT JOIN writing w ON w.atom_id = a.id
LEFT JOIN pbl_project p ON p.atom_id = a.id
WHERE a.user_id = ANY(sqlc.arg(user_ids)::uuid[])
  AND a.kind IN ('reading','writing','project')
  AND a.created_at < sqlc.arg(week_end)::timestamptz
  AND a.last_activity_at < sqlc.arg(week_end)::timestamptz - interval '7 days'
  AND NOT (
    (a.kind = 'reading' AND r.status = 'finished' AND (r.finished_at IS NULL OR r.finished_at < sqlc.arg(week_end)::timestamptz))
    OR (a.kind = 'writing' AND w.status = 'finished' AND (w.finished_at IS NULL OR w.finished_at < sqlc.arg(week_end)::timestamptz))
    OR (a.kind = 'project' AND (p.status = 'archived' OR p.finished_at < sqlc.arg(week_end)::timestamptz
                               OR (p.finished_at IS NULL AND p.status IN ('review','keeping'))))
  )
ORDER BY a.last_activity_at, a.id;

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
SELECT a.user_id,
       COALESCE(r.title, w.title, '')::text AS title,
       COALESCE(ar.report -> 'moments', 'null'::jsonb)::jsonb AS moments,
       COALESCE((ar.report ->> 'prosePending')::boolean, false)::boolean AS prose_pending,
       w.assigned_prompt
FROM atom_report ar
JOIN atom a ON a.id = ar.atom_id
LEFT JOIN reading r ON r.atom_id = a.id
LEFT JOIN writing w ON w.atom_id = a.id
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

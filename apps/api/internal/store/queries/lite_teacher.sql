-- 教师端（lite）读学生数据。只读；只数、只取产出，不取对话正文。

-- name: ListLiteClassRoster :many
-- Every aggregate here is COALESCEd to a deterministic value: sqlc's static
-- nullability inference (no live DB connection configured in sqlc.yaml)
-- treats a bare aggregate-in-a-subquery as NOT NULL regardless of whether
-- max()/sum() over zero rows can genuinely be NULL — an un-COALESCEd
-- max(atom.last_activity_at) generated a non-pointer time.Time that panicked
-- scanning a real NULL for a student with zero atoms. 'epoch'::timestamptz
-- (1970-01-01) is the sentinel for "never active" — liteRosterRow (Go) checks
-- against it rather than against a Go nil. seconds_this_week's 0 needs no
-- sentinel: it is already an honest answer for "measured before, nothing
-- this week".
SELECT u.id, u.display_name, u.avatar_color,
       COALESCE((SELECT max(a.last_activity_at) FROM atom a WHERE a.user_id = u.id), 'epoch'::timestamptz)::timestamptz AS last_active_at,
       COALESCE((SELECT sum(a.active_seconds) FROM atom a WHERE a.user_id = u.id), 0)::int AS seconds_total,
       COALESCE((SELECT sum(d.seconds) FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
         WHERE a.user_id = u.id AND d.day >= sqlc.arg(week_start_day)::date AND d.day < sqlc.arg(week_end_day)::date), 0)::int AS seconds_this_week,
       (SELECT count(*) FROM atom_active_day d JOIN atom a ON a.id = d.atom_id WHERE a.user_id = u.id)::int AS bucket_count,
       (SELECT count(DISTINCT x.day) FROM (
          SELECT d.day FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
           WHERE a.user_id = u.id AND d.seconds > 0 AND d.day >= sqlc.arg(week_start_day)::date AND d.day < sqlc.arg(week_end_day)::date
          UNION
          SELECT (m.created_at AT TIME ZONE 'Asia/Shanghai')::date FROM atom_message m JOIN atom a ON a.id = m.atom_id
           WHERE a.user_id = u.id AND m.role = 'student' AND m.created_at >= sqlc.arg(week_start)::timestamptz AND m.created_at < sqlc.arg(week_end)::timestamptz
        ) x)::int AS active_days_this_week,
       (SELECT count(*) FROM atom_message m JOIN atom a ON a.id = m.atom_id WHERE a.user_id = u.id AND m.role = 'student')::int AS turns,
       (SELECT count(*) FROM reading r JOIN atom a ON a.id = r.atom_id WHERE a.user_id = u.id AND r.status = 'finished')::int AS readings_done,
       (SELECT count(*) FROM atom a WHERE a.user_id = u.id AND a.kind = 'reading')::int AS readings_total,
       (SELECT count(*) FROM writing w JOIN atom a ON a.id = w.atom_id WHERE a.user_id = u.id AND w.status = 'finished')::int AS writings_done,
       (SELECT count(*) FROM atom a WHERE a.user_id = u.id AND a.kind = 'writing')::int AS writings_total,
       (SELECT count(*) FROM pbl_project p JOIN atom a ON a.id = p.atom_id WHERE a.user_id = u.id AND p.status IN ('review','keeping','archived'))::int AS projects_done,
       (SELECT count(*) FROM atom a WHERE a.user_id = u.id AND a.kind = 'project')::int AS projects_total
FROM enrollments e
JOIN users u ON u.id = e.user_id
WHERE e.class_id = sqlc.arg(class_id) AND e.role_in_class = 'student'
ORDER BY u.display_name;

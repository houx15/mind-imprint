-- +goose Up
-- D2: one definition of "this evaluation belongs to this student", across all
-- three scopes. Before this, every caller re-JOINed project/course_session/
-- chat_thread by hand and D1's roster silently read project scope only.
-- Casts are explicit so sqlc infers concrete types (a bare CASE/COALESCE comes
-- back as interface{} — see D1's ListClassRosterReport.HasReport).
CREATE VIEW student_evaluation AS
SELECT
  e.id,
  COALESCE(p.user_id, cs.user_id, t.user_id)::uuid            AS user_id,
  e.scores,
  e.created_at,
  (CASE WHEN e.project_id IS NOT NULL THEN 'project'
        WHEN e.session_id IS NOT NULL THEN 'course'
        ELSE 'chat' END)::text                                AS surface,
  COALESCE(e.project_id, e.session_id, e.thread_id)::uuid     AS scope_id
FROM evaluations e
LEFT JOIN project        p  ON p.id  = e.project_id
LEFT JOIN course_session cs ON cs.id = e.session_id
LEFT JOIN chat_thread    t  ON t.id = e.thread_id
WHERE COALESCE(p.user_id, cs.user_id, t.user_id) IS NOT NULL;

-- +goose Down
DROP VIEW IF EXISTS student_evaluation;

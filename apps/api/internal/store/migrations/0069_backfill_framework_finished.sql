-- +goose Up
-- G1 · backfill the 立题完成 (framework-finished) milestone for projects that
-- already have a plan but predate the going-forward instrumentation in
-- regeneratePlan. The milestone timestamp is derived from the earliest
-- plan_item (the plan was generated the moment 立题 finished — see the gaps
-- doc's decision), written as one append-only `milestone:framework_finished`
-- event per project that lacks one.
INSERT INTO event (id, project_id, user_id, surface, type, payload, created_at)
SELECT gen_random_uuid(), p.id, p.user_id, 'studio', 'milestone:framework_finished', '{}'::jsonb, MIN(pi.created_at)
FROM project p
JOIN plan_item pi ON pi.project_id = p.id
WHERE NOT EXISTS (
    SELECT 1 FROM event e
    WHERE e.project_id = p.id AND e.type = 'milestone:framework_finished'
)
GROUP BY p.id, p.user_id;

-- +goose Down
-- Best-effort: removes ALL framework-finished milestones (backfilled and
-- going-forward alike). A dev-only down migration.
DELETE FROM event WHERE type = 'milestone:framework_finished';

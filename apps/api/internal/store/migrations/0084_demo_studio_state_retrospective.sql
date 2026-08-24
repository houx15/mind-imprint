-- +goose Up
-- Guided-tour demo project (P6 follow-up): the demo project (…0200, seeded
-- 'finished' by 0082 Task 4) still carried a STALE studio_state.stage of
-- 'topic_discussion' — set once by 0082's initial INSERT and never advanced,
-- even though the project is finished. A finished project's correct terminal
-- stage is 'retrospective'; the wrong stage makes the frontend compute
-- docOptions=["essay"] (instead of including the proposal doc), so the
-- guided tour's PROPOSAL 片段 card (writing-aicard) silently no-ops and the
-- reading room arms a spurious "开始探索?" prompt. Advance ONLY the demo
-- project's stage via jsonb_set — every other key in studio_state (openTool,
-- widthTier, reference, updatedAtTurn, started) is preserved untouched.
-- Idempotent (jsonb_set to the same value is a no-op re-run; mirrors 0082/0083).
UPDATE project
SET studio_state = jsonb_set(studio_state, '{stage}', '"retrospective"')
WHERE id = '00000000-0000-0000-0000-000000000200' AND is_demo = true;

-- +goose Down
UPDATE project
SET studio_state = jsonb_set(studio_state, '{stage}', '"topic_discussion"')
WHERE id = '00000000-0000-0000-0000-000000000200' AND is_demo = true;

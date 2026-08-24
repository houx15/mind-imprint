-- +goose Up
-- Guided-tour P7 Task 4b: the warren-map "已读" badge (built in
-- apps/web/src/workspace/blocks/exploration/warrenLayout.ts,
-- anyReferenceDoneByRoot) lights a root up when ANY reference it contains has
-- reading_status='done'. Reference …0260 (Chen et al. 2019, Nature
-- Sustainability — the tour's reading-room target, DEMO_READING_REFERENCE_ID)
-- was seeded 'done' by 0082_seed_demo_project_finished.sql:106-115, and its
-- two containing roots …0290/…0291 (0082:169-182, connected_reference_id)
-- badge already at page-load — so the tour can never SHOW the transition from
-- unread to 已读 after "reading" it.
--
-- Flip it to 'reading' (a real, valid reading_status per 0053's CHECK
-- ('to_read','reading','done') — not a demo-only sentinel): the node starts
-- un-badged, and a later tour task (Task 9) calls the new
-- TourNavContext.markDemoNodeRead("…0290") client-side override when the
-- tour returns from the read-only reading room, badging the node WITHOUT a
-- write (the demo project is write-blocked, 0081). 'reading' also renders a
-- coherent 在读 pill in the 列表 library view (READING_STATUS_LABEL in
-- ReadingBlock.tsx) instead of 读完 — consistent with "not yet finished".
--
-- Scoped to this one demo row only (id + project_id both pinned) so this can
-- never touch a real student's reference.
UPDATE reference
SET reading_status = 'reading'
WHERE id = '00000000-0000-0000-0000-000000000260'
  AND project_id = '00000000-0000-0000-0000-000000000200';

-- +goose Down
UPDATE reference
SET reading_status = 'done'
WHERE id = '00000000-0000-0000-0000-000000000260'
  AND project_id = '00000000-0000-0000-0000-000000000200';

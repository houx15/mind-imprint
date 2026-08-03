-- Course v2 active-focus time (points 2b): secondsSpent must reflect the time
-- the student was actively focused on the course page, not the wall-clock span
-- from first-open to finish (which counted idle/away time). The client accrues
-- active seconds (only while the tab is visible+focused) and reports deltas;
-- this column accumulates them additively across visits.

-- +goose Up
ALTER TABLE course_progress ADD COLUMN active_seconds integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE course_progress DROP COLUMN active_seconds;

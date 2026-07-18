-- +goose Up
-- A3: close project.status to a known enum. project.status has existed since
-- 0016 (text NOT NULL DEFAULT 'active') with NO CHECK and no writer; A3's
-- finish endpoint (SetProjectFinished) becomes its first writer, setting
-- 'finished'. Every existing row is 'active', so a validated CHECK adds cleanly
-- (no NOT VALID needed). Mirrors the enum shape course_session already carries
-- (0023: CHECK (status IN ('active','finished'))).
ALTER TABLE project ADD CONSTRAINT project_status_ck
  CHECK (status IN ('active', 'finished'));

-- +goose Down
ALTER TABLE project DROP CONSTRAINT IF EXISTS project_status_ck;

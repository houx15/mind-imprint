-- +goose Up
CREATE TABLE evaluation_report (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
  version     int  NOT NULL DEFAULT 1,
  report      jsonb NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX evaluation_report_project_created_idx
  ON evaluation_report (project_id, created_at DESC);

-- +goose Down
DROP TABLE evaluation_report;

-- +goose Up
-- A student can report an unreadable assigned material. One open report per
-- student and assignment keeps retries from flooding the teacher's list.
CREATE TABLE lite_assignment_issue (
  assignment_id uuid NOT NULL REFERENCES lite_assignment(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  detail text NOT NULL,
  reported_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,
  PRIMARY KEY (assignment_id, user_id)
);
CREATE INDEX lite_assignment_issue_open_idx ON lite_assignment_issue (assignment_id) WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE lite_assignment_issue;

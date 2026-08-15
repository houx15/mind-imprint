-- +goose Up
-- Course Runtime Slice 8: CourseSession (§16) persistence for the 2.0 runtime.
-- The runtime holds authoritative session state client-side and snapshots the
-- whole CourseSession blob here per (user, course) — richer than course_progress
-- (page position) and separate from it. One row per student per course (get-or-
-- create). `status` mirrors the blob's own status for cheap querying/analytics.
-- NB: the same table name existed pre-0050 (the retired phase runtime); it was
-- fully dropped there, so this is a fresh, unrelated table.
CREATE TABLE course_session (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id  uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
    session    jsonb NOT NULL,
    status     text NOT NULL DEFAULT 'created',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, course_id)
);
CREATE INDEX course_session_user_idx ON course_session(user_id);

-- +goose Down
DROP TABLE IF EXISTS course_session;

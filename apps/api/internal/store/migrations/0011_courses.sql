-- +goose Up
CREATE TABLE course (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    branch      text NOT NULL,
    title       text NOT NULL,
    blurb       text NOT NULL DEFAULT '',
    tasks_count int  NOT NULL DEFAULT 0,
    tools_count int  NOT NULL DEFAULT 0,
    time_label  text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE course_step (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id        uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
    ordinal          int  NOT NULL,
    kind             text NOT NULL CHECK (kind IN ('teaching','challenge')),
    purpose          text NOT NULL DEFAULT '',
    assets           jsonb NOT NULL DEFAULT '[]',
    challenge_type   text,
    authored_content jsonb NOT NULL DEFAULT '{}',
    UNIQUE (course_id, ordinal)
);
CREATE INDEX course_step_course_ordinal_idx ON course_step (course_id, ordinal);

CREATE TABLE course_progress (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id          uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
    current_ordinal    int  NOT NULL DEFAULT 0,
    completed_ordinals int[] NOT NULL DEFAULT '{}',
    updated_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, course_id)
);

-- +goose Down
DROP TABLE course_progress;
DROP TABLE course_step;
DROP TABLE course;

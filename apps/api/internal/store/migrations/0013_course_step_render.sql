-- +goose Up
CREATE TABLE course_step_render (
    course_step_id uuid PRIMARY KEY REFERENCES course_step(id) ON DELETE CASCADE,
    content        jsonb NOT NULL,
    source         text  NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE course_step_render;

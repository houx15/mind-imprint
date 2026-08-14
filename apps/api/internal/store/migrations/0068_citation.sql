-- +goose Up
-- G3 · the source→section citation link. Recorded at material-insert time in
-- the writing room (piggybacks the existing "insert a fragment at the caret"
-- action, no new UI) so the (deferred) report generator can resolve a
-- material's `usedIn` to the essay claim/section it actually supported, not
-- just the exploration question it hung under. `section` uses the machine
-- section convention already in use for guided writing: `claim:<uuid>` /
-- `subq:<id>` / `prop:<step>`. Append-only; a source may be cited into several
-- sections.
CREATE TABLE citation (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    reference_id uuid NOT NULL REFERENCES reference(id) ON DELETE CASCADE,
    section      text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX citation_project_idx ON citation(project_id);

-- +goose Down
DROP TABLE citation;

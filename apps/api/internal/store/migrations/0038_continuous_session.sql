-- +goose Up
-- S1 · one continuous per-project session. The per-project chat_thread already
-- exists (0016, seeded_project_id); S1 makes the four-room coach USE it instead
-- of the stateless /coach. Two additive columns on chat_message + a first-open
-- prose table for summary-on-return.
--
--   surface   — the room/moment a turn happened on. Display filters by it (D1:
--               one thread, surface-tagged view); the coach's CONTEXT never
--               does (continuity sees the whole thread). NULL = untagged/legacy.
--   folded_at — set when a turn has been folded into the spine (lever 1,
--               compaction). NULL = live in the rolling window. A folded turn
--               STAYS in the thread (still shown on reload) but leaves the
--               coach's active context window. Nothing is lost — the spine
--               (proposal/plan) now carries the result.
ALTER TABLE chat_message ADD COLUMN surface   text;
ALTER TABLE chat_message ADD COLUMN folded_at timestamptz;

-- The active-window read (coach context) filters folded turns per thread.
CREATE INDEX chat_message_thread_active_idx
  ON chat_message (thread_id, created_at) WHERE folded_at IS NULL;

-- Summary-on-return · generated prose, first-open-wins (project_mirror_prose /
-- parent_report_prose pattern). The deterministic spine snapshot is computed
-- live on every read; only the LLM-gentled paragraph is stored, once.
CREATE TABLE project_summary_prose (
    project_id uuid PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    prose      text NOT NULL,
    model      text NOT NULL DEFAULT '',
    tier       text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS project_summary_prose;
DROP INDEX IF EXISTS chat_message_thread_active_idx;
ALTER TABLE chat_message DROP COLUMN IF EXISTS folded_at;
ALTER TABLE chat_message DROP COLUMN IF EXISTS surface;

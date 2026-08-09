-- +goose Up
-- Phase B (status-router redesign) · multi-document writing. The 立项 proposal
-- (写研究提案) and the essay (写正文) are now DISTINCT documents, not one shared
-- buffer. We key writing state on `doc_kind` ('proposal'|'essay') so each status
-- edits its own document, and each document has its own snapshots + 完成 milestone.
--
-- All existing writing is the essay/final paper, so the new column defaults to
-- 'essay' and existing rows collapse cleanly onto the essay document.

ALTER TABLE edit_buffer ADD COLUMN doc_kind text NOT NULL DEFAULT 'essay';
ALTER TABLE draft_snapshot ADD COLUMN doc_kind text NOT NULL DEFAULT 'essay';

-- One live buffer PER (project, doc); one snapshot sequence PER (project, doc).
DROP INDEX edit_buffer_project_key;
CREATE UNIQUE INDEX edit_buffer_project_doc_key ON edit_buffer (project_id, doc_kind);
DROP INDEX draft_snapshot_project_seq_key;
CREATE UNIQUE INDEX draft_snapshot_project_doc_seq_key ON draft_snapshot (project_id, doc_kind, seq);

-- The 完成写作 milestone becomes per-document (replacing the scalar
-- project.writing_finished_at). One finish row per (project, doc).
CREATE TABLE writing_finish (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    doc_kind    text NOT NULL,
    finished_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX writing_finish_project_doc_key ON writing_finish (project_id, doc_kind);

-- Backfill: every project whose writing was already finished carries an essay
-- finish row (the finished document WAS the essay/paper).
INSERT INTO writing_finish (project_id, doc_kind, finished_at)
SELECT id, 'essay', writing_finished_at
FROM project
WHERE writing_finished_at IS NOT NULL;

ALTER TABLE project DROP COLUMN writing_finished_at;

-- +goose Down
ALTER TABLE project ADD COLUMN writing_finished_at timestamptz;
UPDATE project p
SET writing_finished_at = wf.finished_at
FROM writing_finish wf
WHERE wf.project_id = p.id AND wf.doc_kind = 'essay';

DROP TABLE writing_finish;

DROP INDEX draft_snapshot_project_doc_seq_key;
CREATE UNIQUE INDEX draft_snapshot_project_seq_key ON draft_snapshot (project_id, seq);
DROP INDEX edit_buffer_project_doc_key;
CREATE UNIQUE INDEX edit_buffer_project_key ON edit_buffer (project_id);

ALTER TABLE draft_snapshot DROP COLUMN doc_kind;
ALTER TABLE edit_buffer DROP COLUMN doc_kind;

-- +goose Up
-- B3: a block-scoped thread for the 深入一层 sub-agent. Mirrors atom_card.block_id
-- (0092), which already scopes a row to one block of an atom.
--
-- NULL = the room's own thread. Set = a sub-agent conversation about one
-- writing_outline node. Seq stays in ONE space per atom, so atom_message_seq_idx
-- (UNIQUE (atom_id, seq)) is untouched and global ordering stays well-defined.
ALTER TABLE atom_message ADD COLUMN block_id text;
CREATE INDEX atom_message_block_idx ON atom_message (atom_id, block_id, seq);

-- B1: guidance persisted so opening 段落 does not re-spend a model call.
-- Sibling of `role` (0100), NOT of `text`: role/guide are scaffold, text is hers.
-- Nothing composes a draft from this column — writing_draft is built from
-- writing_snippet.text only.
ALTER TABLE writing_outline ADD COLUMN guide jsonb;

-- B4 + B7: one shape at two zoom levels. snippet_id NULL = a comment on the
-- whole draft. points is [{"text":"...","quote":"..."}]; every quote has already
-- been verified as a literal substring of the commented text before insert.
CREATE TABLE writing_comment (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  snippet_id uuid REFERENCES writing_snippet(id) ON DELETE CASCADE,
  scope      text NOT NULL CHECK (scope IN ('block','draft')),
  summary    text NOT NULL,
  points     jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX writing_comment_atom_idx ON writing_comment (atom_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS writing_comment;
ALTER TABLE writing_outline DROP COLUMN IF EXISTS guide;
DROP INDEX IF EXISTS atom_message_block_idx;
ALTER TABLE atom_message DROP COLUMN IF EXISTS block_id;

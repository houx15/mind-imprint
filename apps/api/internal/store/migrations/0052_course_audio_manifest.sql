-- Course voice narration (Task 2): audio_manifest carries the published
-- per-teaching-segment TTS manifest for one course, keyed by pieceId
-- ("<stepId>#<segIdx>", a JSON map key — not part of any object storage
-- path) to OSS object key. Generated at publish time (seed / admin upload,
-- later tasks); empty ('{}') until then, never null.

-- +goose Up
ALTER TABLE course ADD COLUMN audio_manifest jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE course DROP COLUMN audio_manifest;

-- +goose Up
CREATE TABLE voice_tts_cache (
    key        text PRIMARY KEY,
    audio      bytea NOT NULL,
    voice      text  NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE voice_tts_cache;

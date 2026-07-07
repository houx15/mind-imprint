-- name: GetVoiceTTSCache :one
SELECT * FROM voice_tts_cache WHERE key = $1;

-- name: UpsertVoiceTTSCache :one
INSERT INTO voice_tts_cache (key, audio, voice)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE SET audio = EXCLUDED.audio, voice = EXCLUDED.voice, created_at = now()
RETURNING *;

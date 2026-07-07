# Voice (TTS + ASR) — Design Spec

> **Status:** approved design, 2026-07-07. Authoritative for the voice feature.
> **Goal (MVP = all of it):** course-wide TTS narration + a full spoken loop
> (ASR in ⇄ TTS out) in the workspace chaperone, built on a server-side
> Volcano Engine proxy. Chinese-first.

## 1. Purpose & product framing

思维印记 is an AI chaperone that guides IB students through structured
thinking. Voice serves two moments:

- **Course narration (TTS):** a student can have a teaching step read aloud
  (`朗读本节`), for pacing and accessibility.
- **Workspace spoken loop (ASR + TTS):** a student speaks a reflection
  instead of typing; the chaperone can read its reply back. A back-and-forth
  voice conversation with the same restraint the text chaperone already has.

The MVP is **both**, sequenced but shipped together.

### Design-law compliance (the four iron laws)

- **AI 克制 (restraint):** the ASR transcript lands in the composer as the
  student's own editable words — never auto-committed. TTS playback is
  opt-in (a play button) or a per-session autoplay toggle, never forced.
- **不操纵 (no manipulation):** **push-to-talk only.** No always-listening
  mic, no VAD auto-capture, no voice-driven engagement hooks. Mirrors
  "工具卡触发自动，打开由学生确认."
- **一次只问一个:** unaffected — voice is a transport, not a change to the
  turn cadence.
- **过程即数据:** a voice-originated message carries a `source:"voice"`
  marker so the evaluator can see the student *chose to speak*. Friction
  and choice become signal, not noise.

## 2. Hard constraints (inherited)

- **Client never holds keys and never talks to Volcano directly.** The
  browser speaks only to `apps/api`, which holds the Volcano credentials and
  bridges to Volcano's WebSocket endpoints. (Key-isolation iron law.)
- Volcano keys live server-side only (env), never logged, never in git,
  never in a rendered error or stored payload. `.env.volcano` is already
  matched by the `.env.*` gitignore rule — keep it that way.
- New card/renderer rules are untouched; voice adds endpoints, not cards.

## 3. Architecture & data flow

One new Go package `internal/voice` ports the two Volcano binary protocols
and exposes two clients. Three paths, all through `apps/api` (prefix
`/api/v1`):

```
Course narration:  step text ─POST /voice/tts─▶ api ─WS─▶ Volcano TTS ─▶ mp3 ─▶ cache ─▶ ▶ play
Workspace TTS:     AI reply  ─POST /voice/tts─▶ (same)               ─▶ mp3 ─▶ ▶ / autoplay
Workspace ASR:     🎙 PCM16k ═WS /voice/asr═▶ api ═WS═▶ Volcano ASR ─▶ partial/final ─▶ composer
```

- **TTS is request/response.** The api opens a short-lived Volcano TTS WS,
  requests `format:"mp3"`, collects `AudioOnlyServer` frames until
  `SessionFinished`, returns one mp3 body. Small enough to cache as `bytea`
  and play in a plain `<audio>`.
- **ASR is a persistent bridge.** The browser opens a WS to the api for the
  duration of one utterance; the api opens a Volcano ASR WS, relays PCM up
  and transcripts down.
- The **loop** = ASR fills the composer → student sends → existing SSE turn
  runs unchanged → AI reply text → TTS play/autoplay. TTS never has to hook
  the SSE engine (that is the deferred full-duplex evolution).

## 4. Reference source

Port from the working Python reference at
`~/08Coding/zrobot/learning-lamp-v2/backend/app/services/`:

- `volc_tts_protocol.py` → TTS binary framing (3-byte bitfield header +
  optional event/session/sequence fields + uint32-length payload; `MsgType`,
  `MsgTypeFlagBits`, `EventType`).
- `tts.py` → TTS request shape (`req_params`: speaker, audio_params, text,
  speed_ratio, volume_ratio), endpoint
  `wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream`,
  resource `seed-tts-2.0`, header auth.
- `asr.py` → ASR *separate, simpler* protocol: 4-byte header
  `(version<<4|hdrsize)`, gzip-compressed JSON payload, pos/neg sequence,
  `CLIENT_FULL_REQUEST` then `CLIENT_AUDIO_ONLY_REQUEST` frames, endpoint
  `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel`, resource
  `volc.bigasr.sauc.duration`, audio `pcm/raw 16000/16/1`.

The two protocols are **different** and get two separate Go files.

## 5. Backend (Go, module `mindimprint/api`)

### 5.1 New package `internal/voice`

- `protocol.go` — TTS framing (`Message` marshal/unmarshal, `MsgType`,
  `EventType`). Round-trip unit-tested against known byte vectors.
- `asr_protocol.go` — ASR framing (header builder, response parser, gzip).
- `tts.go` — `type Client` with
  `Synthesize(ctx context.Context, text string, speed float64) ([]byte, error)`
  returning mp3. Dials Volcano with `github.com/coder/websocket`, sends the
  full-client request, drains audio frames until `SessionFinished`.
- `asr.go` — `type Client` with a streaming session:
  `Stream(ctx, audioIn <-chan []byte) (<-chan Transcript, error)` where
  `Transcript = { Text string; Final bool }`. Sends full-client request,
  spawns an audio sender and a response reader, emits transcripts.
- `config.go` — `type Config { AppID, AccessKey, TTSVoice, TTSResourceID,
  ASRResourceID string }`.

Auth headers on both dials: `X-Api-App-Key: AppID`,
`X-Api-Access-Key: AccessKey`, `X-Api-Resource-Id: <resource>`,
`X-Api-Connect-Id`/`X-Api-Request-Id: uuid`.

### 5.2 Config (`internal/config/config.go`)

Add fields (loaded via the existing `caarlos0/env` mechanism):

```go
VoiceAppID       string `env:"VOICE_APP_ID"`
VoiceAccessKey   string `env:"VOICE_ACCESS_KEY"`
VoiceTTSVoice    string `env:"VOICE_TTS_VOICE"`
VoiceTTSResource string `env:"VOICE_TTS_RESOURCE_ID" envDefault:"seed-tts-2.0"`
VoiceASRResource string `env:"VOICE_ASR_RESOURCE_ID" envDefault:"volc.bigasr.sauc.duration"`
```

`.env.volcano` maps: `APP_ID → VOICE_APP_ID`, `ACCESS_TOKEN → VOICE_ACCESS_KEY`.
(`SECRET_KEY` is Volcano AK/SK HMAC signing, unused by the openspeech WS
endpoints; `LLM_API_KEY`/`MODEL_NAME` are the Ark LLM, unrelated.) During
setup, copy the two needed values into the gitignored `apps/api/.env.local`.
If `VoiceAppID`/`VoiceAccessKey` are empty, voice endpoints return
`503 voice_unavailable` — voice is optional, the platform boots without it.

### 5.3 Injection seam (`internal/api/api.go` `Deps`)

Mirror the provider/resolver pattern. Add to `Deps`:

```go
Voice VoiceService // nil when unconfigured
```

with an interface defined in the api package (so tests inject a stub):

```go
type VoiceService interface {
    Synthesize(ctx context.Context, text string, speed float64) ([]byte, error)
    ASRStream(ctx context.Context, audioIn <-chan []byte) (<-chan voice.Transcript, error)
}
```

`cmd/api/main.go` constructs the real `*voice.Client`-backed impl from `cfg`
(only when configured) and passes it into `Deps`.

### 5.4 Endpoints (`internal/api/voice.go`, registered in `Handler()`)

Both are `protected(...)` (RequireUser via session cookie) and gate on
`HasEntitlement` in the handler body — same shape as `renderCourseStep`.

**`POST /api/v1/voice/tts`** — body `{ "text": string, "speed"?: number }`.

1. `RequireUser` + `HasEntitlement` (→ `ErrNotEntitled` / `503` if no Voice).
2. `key = sha256(voice + "|" + speed + "|" + text)`.
3. Cache read `GetVoiceTTSCache(ctx, key)`. Hit → write bytes.
4. Miss → `Voice.Synthesize(...)` → `UpsertVoiceTTSCache(ctx, {key, audio,
   voice})` → write bytes.
5. Response: `Content-Type: audio/mpeg`, raw mp3 body.

Course narration reuses this endpoint (the client sends the step's readable
text; identical text hits the cache across replays — no course-specific
route needed).

**`GET /api/v1/voice/asr`** — WebSocket upgrade (`coder/websocket` `Accept`
with `OriginPatterns` derived from `CORSOrigins`).

1. `RequireUser` before upgrade; `HasEntitlement`; 503 if no Voice.
2. Read binary frames (Int16 PCM 16k mono) from the browser into an
   `audioIn` channel; a text frame `{"type":"stop"}` (or client close) ends
   input.
3. `Voice.ASRStream(ctx, audioIn)` → forward each `Transcript` to the
   browser as a JSON text frame `{"type":"partial"|"final","text":...}`.
4. Close on final / error / context cancel. Errors sent as
   `{"type":"error","message":<safe>}` — never leak keys or upstream detail.

### 5.5 Persistence

New migration `internal/store/migrations/0014_voice.sql`:

```sql
-- +goose Up
CREATE TABLE voice_tts_cache (
    key        text PRIMARY KEY,
    audio      bytea NOT NULL,
    voice      text  NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE messages ADD COLUMN source text; -- 'voice' when spoken; NULL otherwise

-- +goose Down
ALTER TABLE messages DROP COLUMN source;
DROP TABLE voice_tts_cache;
```

Queries in `internal/store/queries/voice.sql` (regenerate into
`internal/store/sqlc/` via `go tool sqlc generate`):

- `GetVoiceTTSCache :one` → `SELECT * FROM voice_tts_cache WHERE key = $1;`
- `UpsertVoiceTTSCache :one` → INSERT … `ON CONFLICT (key) DO UPDATE SET
  audio=EXCLUDED.audio, voice=EXCLUDED.voice, created_at=now() RETURNING *;`

The cache stores **AI-generated audio only**. Student mic audio is never
stored; only its transcript persists (as message `content`, exactly like a
typed message).

**Voice-origin marker:** `POST /turn` gains an optional body field
`"source"?: "voice"`, threaded through `agent.RunTurn` → the user-message
append (`internal/agent/turn.go`, user branch) → `AppendMessage` (`$11`
`source`). The `Message` Zod schema (`packages/contracts/src/task.ts`) gains
`source: z.enum(["voice"]).nullable().optional()`.

## 6. Frontend (`apps/web`)

- `src/api/voice.ts`
  - `synthesize(text, opts?): Promise<string>` — POST `/voice/tts`, read the
    blob, return an object URL.
  - `class AsrStream` — wraps a WS to `/voice/asr`; `onPartial(cb)`,
    `onFinal(cb)`, `sendPCM(Int16Array)`, `stop()`.
- `src/audio/capture.ts` — mic capture via **AudioWorklet**: `getUserMedia`
  → `AudioContext` → worklet → downsample to 16k → Int16 PCM chunks. Worklet
  processor shipped as a static module (`public/pcm-worklet.js`).
- `src/audio/player.ts` — plays an object URL through a single shared
  `<audio>`; enforces one-at-a-time playback + the autoplay policy.
- **UI touchpoints (lucide icons, never emoji — project rule):**
  - Workspace composer → `Mic` push-to-talk button; live partial transcript
    fills the composer; student reviews/edits and sends. Sending a
    voice-originated message passes `source:"voice"`.
  - AI message bubble → `Volume2` play button + a per-session autoplay
    toggle (defaults on only when the student entered the turn by voice).
  - Course teaching step → `Volume2` `朗读本节` button (extracts the step's
    readable text and calls `synthesize`).

## 7. Entitlement & metering

Both endpoints sit behind the existing `HasEntitlement` seam (they consume
Volcano quota). Per-second/char metering is **out of scope** for the MVP —
noted, not built. When entitlement later gates real quota, voice is already
behind the same seam.

## 8. Testing

**Go**

- `protocol_test.go` / `asr_protocol_test.go` — marshal↔unmarshal round
  trips and fixed byte-vector assertions.
- `tts_test.go` — `voice.Client.Synthesize` against a **fake Volcano WS**
  (httptest + `coder/websocket` `Accept`) that replays a canned
  audio+`SessionFinished` sequence; assert joined mp3 bytes.
- `asr_test.go` — `voice.Client.Stream` against a fake Volcano WS that
  returns partial+final; assert transcript order/finality.
- `voice_test.go` (api, testcontainers PG) — `POST /voice/tts` with a **stub
  `VoiceService`**: miss synthesizes + caches; hit returns bytes without
  calling the stub. ASR WS handler: a ws client streams bytes → assert
  relayed transcripts; unconfigured Voice → 503.

**Web**

- `voice.test.ts` — `synthesize` mocks `fetch` returning an audio blob;
  `AsrStream` mocks `WebSocket`, drives partial/final callbacks.
- AudioWorklet capture verified in the live E2E run (not unit-testable).

## 9. Build sequence (each independently testable)

1. `internal/voice` TTS framing + `Client.Synthesize` (fake-WS tested).
2. `0014` migration + `voice.sql` + `POST /voice/tts` + cache (stub-tested).
3. Course `朗读本节` button — proves TTS end to end in the browser.
4. `internal/voice` ASR framing + `Client.Stream` (fake-WS tested).
5. `GET /voice/asr` bridge + `src/audio/capture.ts` + composer mic → live
   transcript in the workspace.
6. Workspace TTS play/autoplay + `source:"voice"` marker → the loop closes.

## 10. Deferred (explicitly not MVP)

- Full-duplex streaming (TTS hooked into the SSE turn engine, barge-in). The
  shared server-side proxy makes this a later evolution, not a rewrite.
- English / mixed-language ASR (needs a different `model_name`); MVP is
  Chinese-first.
- Raw-audio storage / object storage; per-second usage metering; voice-persona
  selection wired to Settings › AI 形象 (one default voice for now).

## 11. Dependencies added

- Go: `github.com/coder/websocket` (maintained nhooyr successor; used for
  both the Volcano client dials and the browser-facing `/voice/asr` upgrade).
- Web: none (native `WebSocket`, `AudioWorklet`, `<audio>`).

## 12. Open items to confirm at build time

- A valid Volcano **speaker id** for `seed-tts-2.0` (`VOICE_TTS_VOICE`) — the
  reference used a configured `voice_type`; confirm a real value before the
  course-narration demo.
- Whether `seed-tts-2.0` accepts `format:"mp3"` directly (reference used
  `pcm`); if mp3 is unavailable, request `pcm` and wrap/encode server-side,
  or store pcm and let the client play via Web Audio. Decide in Task 1 with a
  live probe.

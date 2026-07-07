# Voice (TTS + ASR) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship course TTS narration + a full workspace spoken loop (ASR in ⇄ TTS out) on a server-side Volcano Engine proxy.

**Architecture:** A new Go `internal/voice` package ports Volcano's two binary WebSocket protocols (TTS + ASR). `POST /voice/tts` returns cached mp3; `GET /voice/asr` bridges the browser to Volcano ASR. The browser holds no keys. The spoken loop reuses the existing SSE turn engine unchanged.

**Tech Stack:** Go 1.26 (`net/http`, `pgx/v5`, `sqlc`, `goose`, `github.com/coder/websocket`), React 18 + Vite + TS (`AudioWorklet`, native `WebSocket`), Zod contracts.

**Design source of truth:** `docs/superpowers/specs/2026-07-07-voice-tts-asr-design.md`.
**Python port reference (read, do not run):** `~/08Coding/zrobot/learning-lamp-v2/backend/app/services/{volc_tts_protocol.py,tts.py,asr.py}`.

## Global Constraints

- **Client never talks to Volcano; keys are server-side only.** The browser calls only `apps/api`. Volcano credentials come from env (`VOICE_APP_ID`, `VOICE_ACCESS_KEY`) and must never be logged, returned in an error body, or committed. `.env.volcano` stays gitignored (`.env.*` rule).
- **UI uses lucide icons, never emoji.** (`Mic`, `Volume2`, `Loader2`, `MicOff`.)
- **Push-to-talk only.** No always-listening mic, no VAD auto-capture.
- **TTS cache stores AI-generated audio only.** Student mic audio is never persisted; only its transcript (as message `content`).
- **API route prefix is `/api/v1`.** Protected routes wrap with `protected(...)` (RequireUser); entitlement is checked in the handler body via `HasEntitlement`, not middleware.
- **sqlc:** after any `.sql` query/migration change, run `go tool sqlc generate` from `apps/api`; generated code lands in `internal/store/sqlc` (package `sqlc`).
- **Voice is optional:** when `VOICE_APP_ID`/`VOICE_ACCESS_KEY` are empty, `Deps.Voice` is nil and voice endpoints return `503`. The platform must boot without voice configured.
- **Audio format:** request mp3 from Volcano TTS; PCM fallback per spec §12 is decided live in Task 2.
- **Go module:** `mindimprint/api`. Migrations dir: `internal/store/migrations` (latest existing = `0013`).

---

## File Structure

**New (Go):**
- `apps/api/internal/voice/protocol.go` — TTS binary framing.
- `apps/api/internal/voice/asr_protocol.go` — ASR binary framing (separate, simpler).
- `apps/api/internal/voice/tts.go` — TTS client (`Synthesize`).
- `apps/api/internal/voice/asr.go` — ASR client (`Stream`).
- `apps/api/internal/voice/config.go` — `voice.Config`.
- `apps/api/internal/api/voice.go` — `POST /voice/tts`, `GET /voice/asr`, `VoiceService` interface.
- `apps/api/internal/store/migrations/0014_voice_tts_cache.sql`, `0015_message_source.sql`.
- `apps/api/internal/store/queries/voice.sql`.
- `*_test.go` alongside each.

**New (web):**
- `apps/web/src/api/voice.ts` — `synthesize`, `AsrStream`.
- `apps/web/src/audio/capture.ts` — AudioWorklet mic capture.
- `apps/web/src/audio/player.ts` — shared audio playback.
- `apps/web/public/pcm-worklet.js` — the worklet processor.

**Modified:**
- `apps/api/internal/config/config.go` — voice env fields.
- `apps/api/cmd/api/main.go` — construct + inject `Voice`.
- `apps/api/internal/api/api.go` — `Deps.Voice`, route registration.
- `apps/api/internal/store/queries/messages.sql` + `internal/agent/turn.go` + `internal/api/turn.go` — `source` column.
- `packages/contracts/src/task.ts` — `Message.source`.
- Course teaching-step component + workspace composer + message bubble (paths resolved by implementer via the existing components).

---

## Task 1: Volcano TTS binary protocol (Go, pure)

**Files:**
- Create: `apps/api/internal/voice/protocol.go`
- Test: `apps/api/internal/voice/protocol_test.go`

**Interfaces:**
- Produces: `Message` struct with `Marshal() []byte` and `ParseMessage([]byte) (Message, error)`; enums `MsgType`, `MsgTypeFlag`, `EventType`; constructor `FullClientRequest(payload []byte) Message`. Consumed by Task 2 (`tts.go`).

**Port faithfully from** `volc_tts_protocol.py` — same bit layout: byte0 `(version<<4)|headerSize` (version=1, headerSize=1 ⇒ `0x11`), byte1 `(type<<4)|flag`, byte2 `(serialization<<4)|compression` (JSON=1, none=0 ⇒ `0x10`), byte3 padding `0x00`. Then, in order: if flag==`WithEvent` write int32 event + (uint32-len + bytes) session_id; for full/audio types with Positive/Negative seq write int32 sequence; for Error type write uint32 error_code; always end with uint32-len + payload. Readers mirror this. Use `encoding/binary.BigEndian`.

Enums (values are exact): `MsgType`: Invalid=0, FullClientRequest=0b1, AudioOnlyClient=0b10, FullServerResponse=0b1001, AudioOnlyServer=0b1011, FrontEndResultServer=0b1100, Error=0b1111. `MsgTypeFlag`: NoSeq=0, PositiveSeq=0b1, LastNoSeq=0b10, NegativeSeq=0b11, WithEvent=0b100. `EventType`: TTSSentenceStart=350, TTSSentenceEnd=351, SessionFinished=152 (others per the Python `EventType` enum — copy them all).

- [ ] **Step 1: Write failing tests**

```go
package voice

import (
	"bytes"
	"testing"
)

func TestFullClientRequestRoundTrip(t *testing.T) {
	payload := []byte(`{"hello":"world"}`)
	raw := FullClientRequest(payload).Marshal()
	// Header: 0x11, (FullClientRequest<<4)|NoSeq = 0x10, 0x10, 0x00
	if !bytes.Equal(raw[:4], []byte{0x11, 0x10, 0x10, 0x00}) {
		t.Fatalf("header = % x, want 11 10 10 00", raw[:4])
	}
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != FullClientRequest || !bytes.Equal(msg.Payload, payload) {
		t.Fatalf("round trip lost data: type=%v payload=%q", msg.Type, msg.Payload)
	}
}

func TestParseAudioOnlyServerFrame(t *testing.T) {
	// AudioOnlyServer (0b1011) + NoSeq, JSON/gzip irrelevant, 3-byte audio payload.
	audio := []byte{0xAA, 0xBB, 0xCC}
	m := Message{Type: AudioOnlyServer, Flag: NoSeq, Payload: audio}
	msg, err := ParseMessage(m.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != AudioOnlyServer || !bytes.Equal(msg.Payload, audio) {
		t.Fatalf("got type=%v payload=% x", msg.Type, msg.Payload)
	}
}

func TestParseServerEventFrame(t *testing.T) {
	m := Message{Type: FullServerResponse, Flag: WithEvent, Event: SessionFinished, Payload: []byte(`{}`)}
	msg, err := ParseMessage(m.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if msg.Event != SessionFinished {
		t.Fatalf("event = %d, want %d", msg.Event, SessionFinished)
	}
}
```

- [ ] **Step 2: Run, verify fail** — `cd apps/api && go test ./internal/voice/ -run TestFullClientRequestRoundTrip -v` → FAIL (undefined symbols).
- [ ] **Step 3: Implement `protocol.go`** — port the enums, `Message` struct (`Type`, `Flag`, `Event`, `SessionID`, `Sequence`, `ErrorCode`, `Payload`), `Marshal`, `ParseMessage`, `FullClientRequest`. Faithful bit-for-bit to the Python reference; return errors instead of panicking on short buffers.
- [ ] **Step 4: Run, verify pass** — `go test ./internal/voice/ -v` → PASS.
- [ ] **Step 5: Commit** — `git add apps/api/internal/voice/protocol.go apps/api/internal/voice/protocol_test.go && git commit -m "feat(voice): port Volcano TTS binary protocol framing"`

---

## Task 2: TTS client + coder/websocket + voice.Config

**Files:**
- Create: `apps/api/internal/voice/tts.go`, `apps/api/internal/voice/config.go`
- Test: `apps/api/internal/voice/tts_test.go`
- Modify: `apps/api/go.mod` (add `github.com/coder/websocket`)

**Interfaces:**
- Consumes: Task 1 protocol (`FullClientRequest`, `ParseMessage`, `MsgType`, `EventType`).
- Produces:
  - `type Config struct { AppID, AccessKey, TTSVoice, TTSResourceID, ASRResourceID string }`
  - `type Client struct { cfg Config; httpClient *http.Client }` + `func New(cfg Config) *Client`
  - `func (c *Client) Synthesize(ctx context.Context, text string, speed float64) ([]byte, error)` — returns the concatenated audio bytes (mp3). Consumed by Task 4.
  - Exposes a package var/const `ttsWSURL = "wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream"` overridable in tests.

**Request shape** (from `tts.py`): JSON `{"user":{"uid":<uuid>},"req_params":{"speaker":cfg.TTSVoice,"audio_params":{"format":"mp3","sample_rate":24000,"enable_timestamp":false},"text":text,"speed_ratio":speed,"volume_ratio":1.0,"additions":"{\"disable_markdown_filter\":false}"}}`. Headers: `X-Api-App-Key`, `X-Api-Access-Key`, `X-Api-Resource-Id: cfg.TTSResourceID`, `X-Api-Connect-Id: <uuid>`. Send as one `FullClientRequest`; then loop `conn.Read`, `ParseMessage`: on `AudioOnlyServer` append `Payload`; on `FullServerResponse` with `Event==SessionFinished` break; on `Error` return an error **without** the payload contents in the message (log-safe). **Live probe (spec §12):** if the fake test passes but a real call rejects `format:"mp3"`, fall back to `"pcm"` and note it in the task report.

- [ ] **Step 1: Write failing test (fake Volcano WS)**

```go
package voice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

func fakeVolcanoTTS(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // client full request
			return
		}
		// Two audio frames then SessionFinished.
		for _, chunk := range [][]byte{{0x01, 0x02}, {0x03, 0x04}} {
			_ = c.Write(ctx, websocket.MessageBinary, Message{Type: AudioOnlyServer, Flag: NoSeq, Payload: chunk}.Marshal())
		}
		_ = c.Write(ctx, websocket.MessageBinary, Message{Type: FullServerResponse, Flag: WithEvent, Event: SessionFinished, Payload: []byte("{}")}.Marshal())
	}))
}

func TestSynthesizeConcatenatesAudio(t *testing.T) {
	srv := fakeVolcanoTTS(t)
	defer srv.Close()
	ttsWSURL = "ws" + strings.TrimPrefix(srv.URL, "http")

	c := New(Config{AppID: "a", AccessKey: "k", TTSVoice: "v", TTSResourceID: "seed-tts-2.0"})
	audio, err := c.Synthesize(context.Background(), "你好", 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if string(audio) != string([]byte{0x01, 0x02, 0x03, 0x04}) {
		t.Fatalf("audio = % x", audio)
	}
}
```

- [ ] **Step 2: Add dep + run** — `cd apps/api && go get github.com/coder/websocket && go test ./internal/voice/ -run TestSynthesize -v` → FAIL.
- [ ] **Step 3: Implement `config.go` + `tts.go`** — dial with `websocket.Dial(ctx, ttsWSURL, &websocket.DialOptions{HTTPHeader: headers})`, `c.SetReadLimit(10<<20)`, the send/receive loop above. Make `ttsWSURL` a package var.
- [ ] **Step 4: Run, verify pass** — `go test ./internal/voice/ -v` → PASS.
- [ ] **Step 5: Commit** — `git add -A apps/api/internal/voice apps/api/go.mod apps/api/go.sum && git commit -m "feat(voice): Volcano TTS client over coder/websocket"`

---

## Task 3: TTS cache — migration 0014 + queries + sqlc

**Files:**
- Create: `apps/api/internal/store/migrations/0014_voice_tts_cache.sql`, `apps/api/internal/store/queries/voice.sql`
- Test: `apps/api/internal/store/voice_cache_test.go`
- Generated: `apps/api/internal/store/sqlc/*` (via `go tool sqlc generate`)

**Interfaces:**
- Produces sqlc methods `GetVoiceTTSCache(ctx, key string) (VoiceTtsCache, error)` and `UpsertVoiceTTSCache(ctx, arg UpsertVoiceTTSCacheParams) (VoiceTtsCache, error)`. Consumed by Task 4.

Migration `0014_voice_tts_cache.sql`:
```sql
-- +goose Up
CREATE TABLE voice_tts_cache (
    key        text PRIMARY KEY,
    audio      bytea NOT NULL,
    voice      text  NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE voice_tts_cache;
```

Queries `voice.sql`:
```sql
-- name: GetVoiceTTSCache :one
SELECT * FROM voice_tts_cache WHERE key = $1;

-- name: UpsertVoiceTTSCache :one
INSERT INTO voice_tts_cache (key, audio, voice)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE SET audio = EXCLUDED.audio, voice = EXCLUDED.voice, created_at = now()
RETURNING *;
```

- [ ] **Step 1: Write failing test** (follows the existing testcontainers store-test pattern — copy the harness from `internal/store`'s existing tests, e.g. `course_*_test.go`, to spin PG + migrate):

```go
func TestVoiceTTSCacheRoundTrip(t *testing.T) {
	q, cleanup := newTestQueries(t) // existing testcontainers helper
	defer cleanup()
	ctx := context.Background()
	_, err := q.UpsertVoiceTTSCache(ctx, sqlc.UpsertVoiceTTSCacheParams{Key: "k1", Audio: []byte{1, 2, 3}, Voice: "v"})
	if err != nil { t.Fatal(err) }
	got, err := q.GetVoiceTTSCache(ctx, "k1")
	if err != nil { t.Fatal(err) }
	if string(got.Audio) != string([]byte{1, 2, 3}) { t.Fatalf("audio = % x", got.Audio) }
}
```

- [ ] **Step 2: Run, verify fail** — `cd apps/api && go test ./internal/store/ -run TestVoiceTTSCache -v` → FAIL (method undefined).
- [ ] **Step 3: Add migration + query, generate** — write the two files, then `go tool sqlc generate`.
- [ ] **Step 4: Run, verify pass** — `go test ./internal/store/ -run TestVoiceTTSCache -v` → PASS.
- [ ] **Step 5: Commit** — `git add -A apps/api/internal/store && git commit -m "feat(voice): voice_tts_cache table + sqlc queries"`

---

## Task 4: POST /voice/tts endpoint + VoiceService seam + wiring

**Files:**
- Create: `apps/api/internal/api/voice.go`
- Test: `apps/api/internal/api/voice_test.go`
- Modify: `apps/api/internal/api/api.go` (`Deps.Voice`, route), `apps/api/internal/config/config.go` (env fields), `apps/api/cmd/api/main.go` (construct + inject)

**Interfaces:**
- Produces:
  - `type VoiceService interface { Synthesize(ctx context.Context, text string, speed float64) ([]byte, error); ASRStream(ctx context.Context, audioIn <-chan []byte) (<-chan voice.Transcript, error) }` (the `ASRStream` method is stubbed to `nil, errors.New("not implemented")` by the real client until Task 8 — declare the full interface now so `Deps` is stable).
  - Handler `(*API) postVoiceTTS(w, r)` at `POST /api/v1/voice/tts`.
- Consumes: Task 2 `voice.Client`, Task 3 cache queries, existing `HasEntitlement`, `httpx.WriteError`/`ErrNotEntitled`.

Config fields (add to `Config` in `config.go`):
```go
VoiceAppID       string `env:"VOICE_APP_ID"`
VoiceAccessKey   string `env:"VOICE_ACCESS_KEY"`
VoiceTTSVoice    string `env:"VOICE_TTS_VOICE"`
VoiceTTSResource string `env:"VOICE_TTS_RESOURCE_ID" envDefault:"seed-tts-2.0"`
VoiceASRResource string `env:"VOICE_ASR_RESOURCE_ID" envDefault:"volc.bigasr.sauc.duration"`
```
`main.go`: if `cfg.VoiceAppID != "" && cfg.VoiceAccessKey != ""`, build a real impl wrapping `voice.New(voice.Config{...})` + `Queries` for caching, assign to `Deps.Voice`; else leave nil.

Handler logic: `RequireUser` (route is `protected`); `if a.d.Voice == nil { WriteError(w, r, httpx.NewError(503,"voice_unavailable","语音服务未启用")) }`; `HasEntitlement` gate; decode `{text, speed?}` (default speed 1.0, reject empty text with 400); `key = hex(sha256(voice+"|"+speed+"|"+text))`; cache get → on hit write `audio/mpeg`; on `pgx.ErrNoRows` → `Voice.Synthesize` → `UpsertVoiceTTSCache` → write; other error → 500. **Never** include upstream/key detail in error bodies.

> Note: keep `Synthesize` (network) and the cache read/write in the handler; inject a **stub `VoiceService`** in tests so no network is needed. The cache is exercised with testcontainers PG.

- [ ] **Step 1: Write failing test** — stub `VoiceService` counting `Synthesize` calls; testcontainers PG:

```go
type stubVoice struct{ calls int; audio []byte }
func (s *stubVoice) Synthesize(_ context.Context, _ string, _ float64) ([]byte, error) { s.calls++; return s.audio, nil }
func (s *stubVoice) ASRStream(context.Context, <-chan []byte) (<-chan voice.Transcript, error) { return nil, errors.New("n/a") }

func TestVoiceTTSCachesAndServes(t *testing.T) {
	// build API with Deps.Voice = &stubVoice{audio: []byte("MP3")}, authenticated request
	// 1st POST /api/v1/voice/tts {"text":"你好"} → 200, body "MP3", Content-Type audio/mpeg, stub.calls==1
	// 2nd identical POST → 200, body "MP3", stub.calls==1 (served from cache)
}

func TestVoiceTTSUnavailableWhenNil(t *testing.T) {
	// Deps.Voice == nil → POST → 503
}
```

- [ ] **Step 2: Run, verify fail** — `go test ./internal/api/ -run TestVoiceTTS -v` → FAIL.
- [ ] **Step 3: Implement** — config fields, `voice.go` handler + `VoiceService` interface + real impl in `main.go`, register route in `api.go` `Handler()`: `mux.Handle("POST /api/v1/voice/tts", protected(a.postVoiceTTS))`.
- [ ] **Step 4: Run, verify pass** — `go test ./internal/api/ -run TestVoiceTTS -v` → PASS; `go build ./...` clean.
- [ ] **Step 5: Commit** — `git add -A apps/api && git commit -m "feat(voice): POST /voice/tts with mp3 cache + VoiceService seam"`

---

## Task 5: Web — synthesize + player + course 朗读 button

**Files:**
- Create: `apps/web/src/api/voice.ts`, `apps/web/src/audio/player.ts`
- Test: `apps/web/src/api/voice.test.ts`
- Modify: the course teaching-step component (implementer locates it under `apps/web/src/**` — the component that renders a rendered teaching step; search for the course player teaching view).

**Interfaces:**
- Produces: `async function synthesize(text: string, opts?: { speed?: number }): Promise<string>` (returns an object URL); `class AudioPlayer { play(url: string): Promise<void>; stop(): void }` (single shared instance, one-at-a-time). Consumed by Task 10 for workspace playback.

`synthesize`: `POST /api/v1/voice/tts` with credentials, body `{text, speed}`, `Accept: audio/mpeg`; on non-2xx throw; `const blob = await res.blob(); return URL.createObjectURL(blob)`.

- [ ] **Step 1: Write failing test**

```ts
import { describe, it, expect, vi } from "vitest";
import { synthesize } from "./voice";

describe("synthesize", () => {
  it("POSTs text and returns an object URL", async () => {
    const blob = new Blob([new Uint8Array([1, 2, 3])], { type: "audio/mpeg" });
    vi.stubGlobal("fetch", vi.fn(async () => new Response(blob, { status: 200 })));
    vi.stubGlobal("URL", { createObjectURL: vi.fn(() => "blob:xyz") } as never);
    const url = await synthesize("你好", { speed: 1 });
    expect(url).toBe("blob:xyz");
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining("/voice/tts"), expect.objectContaining({ method: "POST" }));
  });
});
```

- [ ] **Step 2: Run, verify fail** — `pnpm --filter web test -- voice` → FAIL.
- [ ] **Step 3: Implement** `voice.ts` (`synthesize`), `player.ts` (`AudioPlayer` wrapping one `<audio>`/`Audio()` element), and add a `朗读本节` button (lucide `Volume2` idle / `Loader2` spinner / `Volume2` playing) to the teaching-step component: on click extract the step's readable text, `synthesize` → `player.play`.
- [ ] **Step 4: Run, verify pass** — `pnpm --filter web test -- voice` → PASS; `pnpm --filter web exec tsc --noEmit` clean.
- [ ] **Step 5: Commit** — `git add -A apps/web && git commit -m "feat(voice): course narration — synthesize + AudioPlayer + 朗读本节"`

---

## Task 6: Volcano ASR binary protocol (Go, pure)

**Files:**
- Create: `apps/api/internal/voice/asr_protocol.go`
- Test: `apps/api/internal/voice/asr_protocol_test.go`

**Interfaces:**
- Produces: `buildFullClientRequest(seq int32, payload []byte) []byte`, `buildAudioOnlyRequest(seq int32, segment []byte, last bool) []byte`, `parseASRResponse([]byte) (asrResponse, error)` where `asrResponse` has `Code int32`, `IsLast bool`, `Seq int32`, `PayloadMsg map[string]any`. Consumed by Task 7. (Unexported — same package as `asr.go`.)

**Port faithfully from** `asr.py`: header byte0 `(0b0001<<4)|1`, byte1 `(messageType<<4)|flags`, byte2 `(serialization<<4)|compression` (JSON=1, gzip=1 ⇒ `0x11`), byte3 `0x00`. Full request: `CLIENT_FULL_REQUEST=0b0001` + `POS_SEQUENCE=0b0001`, then int32 seq, uint32 len, gzip(payload). Audio-only: `CLIENT_AUDIO_ONLY_REQUEST=0b0010`; last packet uses `NEG_WITH_SEQUENCE=0b0011` and negates seq. Parse: `headerSize=msg[0]&0x0F`; flags from `msg[1]&0x0F`; payload at `msg[headerSize*4:]`; if flag&0x01 read int32 seq; if flag&0x02 set IsLast; server-full (`0b1001`) reads uint32 size; error (`0b1111`) reads int32 code + uint32 size; gzip-decompress; JSON-unmarshal into `PayloadMsg`.

- [ ] **Step 1: Write failing test** — round-trip a full request through parse where possible, and assert exact header bytes:

```go
func TestASRFullRequestHeader(t *testing.T) {
	raw := buildFullClientRequest(1, []byte(`{"a":1}`))
	if raw[0] != 0x11 || raw[1] != 0x11 || raw[2] != 0x11 {
		t.Fatalf("header = % x, want 11 11 11 ..", raw[:3])
	}
}

func TestParseASRServerResponse(t *testing.T) {
	// Hand-build a SERVER_FULL_RESPONSE (0b1001) with POS_SEQUENCE flag, gzip JSON {"result":{"text":"你好","is_final":true}}
	// assert parseASRResponse returns PayloadMsg["result"]["text"]=="你好"
}
```

- [ ] **Step 2: Run, verify fail** — `go test ./internal/voice/ -run TestASR -v` → FAIL.
- [ ] **Step 3: Implement `asr_protocol.go`** — the builders + parser with `compress/gzip`, `encoding/binary`, `encoding/json`.
- [ ] **Step 4: Run, verify pass** — `go test ./internal/voice/ -v` → PASS.
- [ ] **Step 5: Commit** — `git add apps/api/internal/voice/asr_protocol*.go && git commit -m "feat(voice): port Volcano ASR binary protocol framing"`

---

## Task 7: ASR client

**Files:**
- Create: `apps/api/internal/voice/asr.go`
- Test: `apps/api/internal/voice/asr_test.go`

**Interfaces:**
- Produces: `type Transcript struct { Text string; Final bool }`; method `func (c *Client) Stream(ctx context.Context, audioIn <-chan []byte) (<-chan Transcript, error)`. Consumed by Task 4's real impl (`ASRStream`) and Task 8. Package var `asrWSURL` overridable in tests.
- Consumes: Task 6 builders/parser.

Behavior (from `asr.py`): dial `asrWSURL` with headers `X-Api-Resource-Id: cfg.ASRResourceID`, `X-Api-Request-Id: <uuid>`, `X-Api-Access-Key`, `X-Api-App-Key`; send full-client request (audio config `pcm/raw 16000/16/1`, request `{model_name:"bigmodel", enable_itn:true, enable_punc:true, show_utterances:true}`), read the init response (non-zero code ⇒ error). Then spawn a sender goroutine (each `audioIn` chunk → `buildAudioOnlyRequest(seq++, chunk, false)`; on channel close send `buildAudioOnlyRequest(seq, nil, true)`) and a reader that parses responses, emits `Transcript{result.text, result.is_final}` on the returned channel, and closes it on `IsLast`/error/ctx done.

- [ ] **Step 1: Write failing test (fake Volcano ASR WS)** — the fake reads the full request + audio frames and replies with one partial then one final (IsLast) frame; assert the client emits `{Text:"你", Final:false}` then `{Text:"你好", Final:true}`.
- [ ] **Step 2: Run, verify fail** — `go test ./internal/voice/ -run TestASRStream -v` → FAIL.
- [ ] **Step 3: Implement `asr.go`.**
- [ ] **Step 4: Run, verify pass** — `go test ./internal/voice/ -v` → PASS. Wire the real `ASRStream` on the Task-4 impl to call `Stream`.
- [ ] **Step 5: Commit** — `git add -A apps/api/internal/voice && git commit -m "feat(voice): Volcano ASR streaming client"`

---

## Task 8: GET /voice/asr WebSocket bridge

**Files:**
- Modify: `apps/api/internal/api/voice.go` (add handler), `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/voice_asr_test.go`

**Interfaces:**
- Produces: `(*API) getVoiceASR(w, r)` at `GET /api/v1/voice/asr` (WS upgrade). Consumes: `Deps.Voice.ASRStream`, `CORSOrigins` for `OriginPatterns`.

Handler: `RequireUser`; `if Voice==nil → 503` (before upgrade); `HasEntitlement`; `websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: originsFrom(a.d.CORSOrigins)})`; make `audioIn := make(chan []byte, 32)`; read loop: binary frame → `audioIn <- data`; text frame `{"type":"stop"}` or read error/close → close `audioIn`. Concurrently `out, err := Voice.ASRStream(ctx, audioIn)`; forward each `Transcript` as a text frame `{"type": final?"final":"partial", "text": t.Text}`. On error send `{"type":"error","message":"语音识别失败"}` (never leak detail) and close. Use `context` tied to the request; ensure both goroutines exit on close.

- [ ] **Step 1: Write failing test** — stub `VoiceService.ASRStream` returns a channel emitting one partial + one final; connect a `coder/websocket` client to the test server, send a binary frame + `{"type":"stop"}`, assert it receives `{"type":"partial",...}` then `{"type":"final",...}`. Add: nil Voice → handshake rejected/closes with 503-equivalent (assert the pre-upgrade 503 via a plain GET without upgrade headers).
- [ ] **Step 2: Run, verify fail** — `go test ./internal/api/ -run TestVoiceASR -v` → FAIL.
- [ ] **Step 3: Implement** the handler + route `mux.Handle("GET /api/v1/voice/asr", protected(a.getVoiceASR))` + an `originsFrom([]string) []string` helper.
- [ ] **Step 4: Run, verify pass** — `go test ./internal/api/ -run TestVoiceASR -v` → PASS; `go build ./...` clean.
- [ ] **Step 5: Commit** — `git add -A apps/api && git commit -m "feat(voice): GET /voice/asr websocket bridge to Volcano ASR"`

---

## Task 9: Web — mic capture + AsrStream + composer push-to-talk

**Files:**
- Create: `apps/web/src/audio/capture.ts`, `apps/web/public/pcm-worklet.js`
- Modify: `apps/web/src/api/voice.ts` (add `AsrStream`), the workspace composer component (implementer locates it — the chat input under `apps/web/src/**`)
- Test: `apps/web/src/api/voice.test.ts` (extend)

**Interfaces:**
- Produces:
  - `class AsrStream { constructor(); onPartial(cb:(t:string)=>void): void; onFinal(cb:(t:string)=>void): void; sendPCM(pcm: Int16Array): void; stop(): void }` — wraps a `WebSocket` to `/api/v1/voice/asr`.
  - `class MicCapture { start(onPcm:(pcm:Int16Array)=>void): Promise<void>; stop(): void }` — `getUserMedia({audio})` → `AudioContext` → `AudioWorkletNode('pcm-worklet')` → downsample to 16k → `Int16Array` chunks.

`pcm-worklet.js`: an `AudioWorkletProcessor` that posts Float32 frames to the main thread (downsampling + Int16 conversion done in `capture.ts` to keep the worklet minimal). WS URL derives from the API base, `ws(s)://…/api/v1/voice/asr`, `credentials` implied by cookie.

- [ ] **Step 1: Write failing test** — mock `WebSocket`; assert `AsrStream` calls `onPartial`/`onFinal` when the socket emits the corresponding JSON text frames, and that `sendPCM` sends binary. (MicCapture/worklet are covered by the live E2E, not unit tests — note this in the report.)
- [ ] **Step 2: Run, verify fail** — `pnpm --filter web test -- voice` → FAIL.
- [ ] **Step 3: Implement** `AsrStream`, `capture.ts`, `pcm-worklet.js`, and add a `Mic` push-to-talk button to the composer: press → `MicCapture.start` + open `AsrStream`, stream PCM, live partial transcript fills the input; release → `stop()`, final transcript stays in the input for the student to edit/send. Mark the message `source:"voice"` when sent (consumed by Task 10).
- [ ] **Step 4: Run, verify pass** — `pnpm --filter web test -- voice` → PASS; `tsc --noEmit` clean.
- [ ] **Step 5: Commit** — `git add -A apps/web && git commit -m "feat(voice): workspace push-to-talk — mic capture + ASR stream + live transcript"`

---

## Task 10: Workspace TTS playback + voice-origin marker (the loop closes)

**Files:**
- Create: `apps/api/internal/store/migrations/0015_message_source.sql`
- Modify: `apps/api/internal/store/queries/messages.sql` (`AppendMessage` +`source`), regenerate sqlc; `apps/api/internal/agent/turn.go` (thread `source` to the user-message append); `apps/api/internal/api/turn.go` (accept optional `source` in the turn body); `packages/contracts/src/task.ts` (`Message.source`); the workspace message-bubble component + composer (play button, autoplay toggle, pass `source`).
- Test: extend `apps/api/internal/agent/turn_test.go` (source persisted) and `packages/contracts` message test.

**Interfaces:**
- Consumes: Task 5 `synthesize` + `AudioPlayer`; Task 9 voice-origin flag.

Migration `0015_message_source.sql`:
```sql
-- +goose Up
ALTER TABLE messages ADD COLUMN source text;
-- +goose Down
ALTER TABLE messages DROP COLUMN source;
```
`AppendMessage` query gains `source` as `$11` (nullable). `RunTurn`/`postTurn`: optional `source` string on the request → set on the **user** message only. `Message` Zod: `source: z.enum(["voice"]).nullable().optional()`.

- [ ] **Step 1: Write failing tests** — (a) `turn_test.go`: a turn with `source:"voice"` persists the user message with `source=="voice"`; default is null. (b) contracts: `Message.parse({..., source:"voice"})` ok; `source` omitted ok.
- [ ] **Step 2: Run, verify fail** — `go test ./internal/agent/ -run TestRunTurn -v` and `pnpm --filter contracts test` → FAIL.
- [ ] **Step 3: Implement** migration + query + sqlc gen + `turn.go`/`postTurn` threading + contracts; then the web: AI message bubble gets a `Volume2` play button (`synthesize`+`player.play`) and a per-session autoplay toggle (default on only when the last student turn was voice); composer passes `source:"voice"`.
- [ ] **Step 4: Run, verify pass** — Go turn tests PASS; contracts PASS; `pnpm --filter web test` + `tsc` clean; `go build ./...` clean.
- [ ] **Step 5: Commit** — `git add -A && git commit -m "feat(voice): workspace TTS playback + voice-origin message marker — loop closes"`

---

## Self-Review

- **Spec coverage:** §3 paths → Tasks 1-2,4,5 (TTS), 6-8 (ASR), 9-10 (loop). §5 backend → 1-4,6-8,10. §6 frontend → 5,9,10. §5.5 persistence → 3 (cache) + 10 (source). §8 testing → every task's fake-WS/stub/mock. §7 entitlement → gated in 4 & 8. §10 deferred items excluded. §12 probes → resolved in Task 2.
- **Type consistency:** `voice.Config`, `voice.Client`, `voice.Transcript`, `VoiceService{Synthesize, ASRStream}` are named identically across Tasks 2/4/7/8. `Message`/`ParseMessage`/`FullClientRequest` (Task 1) reused in Task 2. `synthesize`/`AudioPlayer`/`AsrStream` consistent across web tasks.
- **Placeholder scan:** protocol ports point to the exact reference file + exact bit layout + byte-vector tests rather than inline transcription (correct approach for a faithful port; the tests are the executable spec). No TBD/TODO.
- **Ordering:** each task builds only on earlier ones; `VoiceService` declares `ASRStream` in Task 4 (stubbed) so `Deps` never changes shape later.

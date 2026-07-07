package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/voice"
)

// VoiceService is the seam for the two Volcano Engine voice APIs: TTS
// (request/response, synchronous from the handler's point of view) and ASR
// (bidirectional streaming). The real implementation wraps voice.Client;
// tests inject a stub so no network call happens. Declaring ASRStream now
// (Task 8 implements it for real) keeps this interface — and Deps — stable
// across tasks.
type VoiceService interface {
	Synthesize(ctx context.Context, text string, speed float64) ([]byte, error)
	ASRStream(ctx context.Context, audioIn <-chan []byte) (<-chan voice.Transcript, error)
	// Voice returns the configured voice name, used to key the TTS cache so
	// rotating VOICE_TTS_VOICE never silently serves stale audio.
	Voice() string
}

// synthesizeTimeout bounds the upstream TTS call. voice.Client.Synthesize has
// no internal timeout of its own — the caller must bound it, since a stuck
// WebSocket read would otherwise hang the request indefinitely.
const synthesizeTimeout = 30 * time.Second

// voiceClient adapts voice.Client to VoiceService for production wiring.
type voiceClient struct {
	c     *voice.Client
	voice string
}

// NewVoiceService builds the production VoiceService from Volcano Engine
// credentials/config. Returns a non-nil VoiceService unconditionally; the
// caller (main.go) decides whether to construct one at all based on whether
// credentials are configured, leaving Deps.Voice nil otherwise.
func NewVoiceService(cfg voice.Config) VoiceService {
	return &voiceClient{c: voice.New(cfg), voice: cfg.TTSVoice}
}

func (vc *voiceClient) Synthesize(ctx context.Context, text string, speed float64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, synthesizeTimeout)
	defer cancel()
	return vc.c.Synthesize(ctx, text, speed)
}

// ASRStream is intentionally not given its own request timeout — ASR is a
// long-lived stream, and it's the caller's (Task 8's) request context that
// bounds its lifetime.
func (vc *voiceClient) ASRStream(ctx context.Context, audioIn <-chan []byte) (<-chan voice.Transcript, error) {
	return vc.c.Stream(ctx, audioIn)
}

func (vc *voiceClient) Voice() string { return vc.voice }

// voiceTTSRequest is the wire body for POST /api/v1/voice/tts.
type voiceTTSRequest struct {
	Text  string   `json:"text"`
	Speed *float64 `json:"speed,omitempty"`
}

// postVoiceTTS synthesizes speech for text, caching mp3 bytes by a hash of
// (voice, speed, text) so repeat requests never re-hit the upstream. The
// voice feature is optional: if Deps.Voice is nil (no credentials
// configured), every request gets a 503 rather than the platform failing to
// boot.
func (a *API) postVoiceTTS(w http.ResponseWriter, r *http.Request) {
	if a.d.Voice == nil {
		httpx.WriteError(w, r, httpx.ErrVoiceUnavailable())
		return
	}

	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var req voiceTTSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_request", "请求格式错误", nil))
		return
	}
	if req.Text == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_request", "文本不能为空", nil))
		return
	}
	speed := 1.0
	if req.Speed != nil {
		speed = *req.Speed
	}

	// The cache key is derived from the configured voice name (not a literal
	// placeholder) so rotating VOICE_TTS_VOICE can never silently serve
	// audio synthesized under a different voice.
	voiceLabel := a.d.Voice.Voice()
	sum := sha256.Sum256([]byte(voiceLabel + "|" + strconv.FormatFloat(speed, 'f', -1, 64) + "|" + req.Text))
	key := hex.EncodeToString(sum[:])

	if cached, err := a.d.Queries.GetVoiceTTSCache(r.Context(), key); err == nil {
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write(cached.Audio)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}

	audio, err := a.d.Voice.Synthesize(r.Context(), req.Text, speed)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if _, err := a.d.Queries.UpsertVoiceTTSCache(r.Context(), sqlc.UpsertVoiceTTSCacheParams{
		Key: key, Audio: audio, Voice: voiceLabel,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	_, _ = w.Write(audio)
}

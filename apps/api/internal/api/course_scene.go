package api

// course_scene.go — runtime Opening/Closing scene generation endpoint
// (Course Runtime Slice 7). POST /api/v1/courses/{slug}/scene generates the
// narration text for one prepared scene (via agent.GenerateCourseScene on the
// chaperone tier), meters the call, and — when Voice + OSS are both wired —
// synthesizes narration audio and stores it under a scene key namespace,
// returning a signed URL. Every generation/synthesis failure degrades to
// usable text (the caller-supplied static fallback on LLM failure, text-only
// on TTS/OSS failure) and returns HTTP 200: the player must never stall on a
// 500. Persisting the result onto CourseSession is Slice 8's concern — this
// endpoint is stateless and safe to call repeatedly (audio is content-hash
// idempotent).

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
)

// sceneFacts mirrors the teacher-approved facts the frontend derives from a
// CourseDefinition's Opening/Closing config. Only the fields relevant to the
// requested slot are populated.
type sceneFacts struct {
	Title                string   `json:"title"`
	EstimatedMinutes     int      `json:"estimatedMinutes"`
	Objectives           []string `json:"objectives"`
	LearningPreview      []string `json:"learningPreview"`
	PreparedSummary      string   `json:"preparedSummary"`
	Takeaways            []string `json:"takeaways"`
	TransferApplications []string `json:"transferApplications"`
}

// sceneRequest is the POST body. `fallback` is the authored static text the
// player shows when generation fails — the server echoes it (fallbackUsed=true)
// rather than erroring, so the caller never has to special-case a 500.
type sceneRequest struct {
	Which          string            `json:"which"`
	Facts          sceneFacts        `json:"facts"`
	AllowedSignals []string          `json:"allowedSignals"`
	SignalEvidence map[string]string `json:"signalEvidence"`
	Fallback       struct {
		Text     string `json:"text"`
		AudioURL string `json:"audioUrl"`
	} `json:"fallback"`
}

// sceneResultDTO serializes to exactly @mind-imprint/course-contract's
// RuntimeSceneResult ({ text, audioUrl?, generatedAt, usedSignalTypes,
// fallbackUsed }). audioUrl is omitted when empty (text-only fallback).
type sceneResultDTO struct {
	Text            string   `json:"text"`
	AudioURL        string   `json:"audioUrl,omitempty"`
	GeneratedAt     string   `json:"generatedAt"`
	UsedSignalTypes []string `json:"usedSignalTypes"`
	FallbackUsed    bool     `json:"fallbackUsed"`
}

// sceneForCourse handles POST /api/v1/courses/{slug}/scene.
func (a *API) sceneForCourse(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !a.requireVisibleCourse(w, r, slug) {
		return
	}
	u, _ := UserFromContext(r.Context())

	// Entitlement gate — a scene generation spends tokens, so it sits behind the
	// same seam every token endpoint does (currently恒 true). JSON error, no body
	// committed yet.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var body sceneRequest
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.Which != "opening" && body.Which != "closing" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "which 必须是 opening 或 closing", nil))
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Generation failure (resolver or LLM) → the authored fallback, never a 500.
	fallback := func() {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"result": sceneResultDTO{
			Text:            body.Fallback.Text,
			AudioURL:        body.Fallback.AudioURL,
			GeneratedAt:     now,
			UsedSignalTypes: []string{},
			FallbackUsed:    true,
		}})
	}

	resolved, err := a.routeE(r.Context(), gateway.ClassCompose) // chaperone — narration is a light task, not the flagship evaluator
	if err != nil {
		slog.Warn("scene: resolver failed, using fallback text", "err", err, "slug", slug,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		fallback()
		return
	}

	in := agent.SceneGenInput{
		Which:                body.Which,
		CourseTitle:          body.Facts.Title,
		EstimatedMinutes:     body.Facts.EstimatedMinutes,
		Objectives:           body.Facts.Objectives,
		LearningPreview:      body.Facts.LearningPreview,
		PreparedSummary:      body.Facts.PreparedSummary,
		Takeaways:            body.Facts.Takeaways,
		TransferApplications: body.Facts.TransferApplications,
		AllowedSignals:       body.AllowedSignals,
		SignalEvidence:       body.SignalEvidence,
	}
	out, usage, gerr := agent.GenerateCourseScene(r.Context(), a.d.Provider, resolved, in)

	// Meter whenever the model call itself produced tokens, even on a downstream
	// error — the tokens were spent (mirrors course.go's coach metering).
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
		if merr := store.RecordCourseLLMCall(r.Context(), u.ID, "scene", resolved, int32(usage.InputTokens), int32(usage.OutputTokens)); merr != nil {
			slog.Warn("scene: record llm usage failed", "err", merr, "slug", slug,
				"request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	if gerr != nil {
		slog.Warn("scene: generation failed, using fallback text", "err", gerr, "slug", slug, "which", body.Which,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		fallback()
		return
	}

	// Text generated — attempt narration audio, but never fail the response if
	// TTS/OSS is absent or errors (text-only is a valid, fallbackUsed=false
	// result per §6.1/§6.2).
	audioURL := a.synthesizeSceneAudio(r, slug, body.Which, out.Text)

	usedTypes := out.UsedSignalTypes
	if usedTypes == nil {
		usedTypes = []string{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"result": sceneResultDTO{
		Text:            out.Text,
		AudioURL:        audioURL,
		GeneratedAt:     now,
		UsedSignalTypes: usedTypes,
		FallbackUsed:    false,
	}})
}

// synthesizeSceneAudio synthesizes narration for `text` and uploads it to OSS
// under a scene key namespace, returning a signed GET URL. Any absence (Voice
// or OSS unwired) or failure (synth/put/sign) returns "" — the caller then
// serves text-only, never a 500. Idempotent: an object already present under
// the content-hash key is not re-synthesized.
func (a *API) synthesizeSceneAudio(r *http.Request, slug, which, text string) string {
	if a.d.Voice == nil || a.d.OSS == nil {
		return ""
	}
	key := sceneAudioObjectKey(slug, which, a.d.Voice.Voice(), text)

	exists, err := a.d.OSS.Exists(r.Context(), key)
	if err != nil {
		slog.Warn("scene: OSS Exists failed, serving text-only", "err", err, "slug", slug,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		return ""
	}
	if !exists {
		audio, serr := a.d.Voice.Synthesize(r.Context(), text, 1.0)
		if serr != nil {
			slog.Warn("scene: TTS synth failed, serving text-only", "err", serr, "slug", slug,
				"request_id", httpx.RequestIDFromContext(r.Context()))
			return ""
		}
		if perr := a.d.OSS.PutObject(r.Context(), key, "audio/mpeg", audio); perr != nil {
			slog.Warn("scene: OSS put failed, serving text-only", "err", perr, "slug", slug,
				"request_id", httpx.RequestIDFromContext(r.Context()))
			return ""
		}
	}

	url, err := a.d.OSS.SignDownload(key)
	if err != nil {
		slog.Warn("scene: sign download failed, serving text-only", "err", err, "slug", slug,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		return ""
	}
	return url
}

// sceneAudioObjectKey builds the OSS key for one scene's narration:
// courses/audio/<slug>/scene/<which>_<hash8>.mp3, hash8 = sha256(voice+"\n"+
// text)[:8] — mixing the voice name in invalidates the cache on a voice
// rotation, the same scheme course_audio.go uses for teaching segments.
func sceneAudioObjectKey(slug, which, voice, text string) string {
	sum := sha256.Sum256([]byte(voice + "\n" + text))
	hash8 := hex.EncodeToString(sum[:])[:8]
	return "courses/audio/" + slug + "/scene/" + which + "_" + hash8 + ".mp3"
}

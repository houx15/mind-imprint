package agent

// course_audio.go — publish-time TTS pre-generation for course teaching
// segments (course voice narration, Task 3). GenerateCourseAudio walks a
// published course's render_cache and, for every non-empty "teaching"
// segment, synthesizes one mp3 and uploads it to OSS, idempotently (an
// object that already exists is never re-synthesized) and tolerantly (a
// single piece's synth/upload failure is warned and skipped, never aborts
// the whole course). The caller (Task 4: seed / admin upload) is expected to
// store the returned pieceId->objectKey manifest on the course row
// (coursestore.go's CoursePlayerPayload/UpsertCourseInput.AudioManifest,
// migration 0052) — this function does no persistence of its own.
//
// CourseAudioSynth and CourseAudioStore are deliberately narrow (not
// api.VoiceService / *oss.Service directly) so tests can stub them without a
// real TTS backend or OSS bucket; in production api.VoiceService already
// satisfies CourseAudioSynth and *oss.Service already satisfies
// CourseAudioStore, so no adapter type is needed at the call site.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
)

// CourseAudioSynth synthesizes speech for one teaching segment's text.
type CourseAudioSynth interface {
	Synthesize(ctx context.Context, text string, speed float64) ([]byte, error)
	// Voice returns the configured voice name, mixed into the object-key hash
	// so a voice change invalidates the cache instead of silently serving
	// stale audio under the same key.
	Voice() string
}

// CourseAudioStore is the narrow slice of OSS operations GenerateCourseAudio
// needs: an existence check for idempotency, and an upload.
type CourseAudioStore interface {
	Exists(ctx context.Context, key string) (bool, error)
	PutObject(ctx context.Context, key, contentType string, data []byte) error
}

// courseAudioRenderCache is the narrow slice of course.render_cache this
// function reads: every step's stepId and its content.segments — deliberately
// ignoring every other authored field (titles, subtitles, interactions), the
// same "parse only what you need" discipline as courseAskRenderCache in
// internal/api/course.go.
type courseAudioRenderCache struct {
	Steps []struct {
		StepID  string `json:"stepId"`
		Content struct {
			Segments []struct {
				Kind string `json:"kind"`
				Text string `json:"text"`
			} `json:"segments"`
		} `json:"content"`
	} `json:"steps"`
}

// synthSpeed is the fixed narration speed passed to Synthesize — v1 has no
// per-piece speed control (see plan's Global Constraints, v1 scope).
const synthSpeed = 1.0

// GenerateCourseAudio synthesizes and uploads one mp3 per non-empty teaching
// segment in renderCache, returning a pieceId ("<stepId>#<segIdx>") ->
// objectKey ("courses/audio/<slug>/<stepId>_<segIdx>_<hash8>.mp3") manifest.
//
// Degrades gracefully in three ways, per the plan's Global Constraints:
//   - synth or store nil (voice/OSS not configured) -> empty map, nil error;
//     course publishing must not fail just because narration is unavailable.
//   - a piece whose object already exists in store is never re-synthesized
//     (idempotent republish) but its key is still recorded in the manifest.
//   - a piece whose Synthesize or PutObject call fails is warned and skipped
//     (not included in the manifest); it never aborts the remaining pieces
//     or returns an error, so one bad TTS call can't block a whole publish.
func GenerateCourseAudio(ctx context.Context, synth CourseAudioSynth, store CourseAudioStore, slug string, renderCache []byte) (map[string]string, error) {
	manifest := map[string]string{}
	if synth == nil || store == nil {
		return manifest, nil
	}

	var rc courseAudioRenderCache
	if err := json.Unmarshal(renderCache, &rc); err != nil {
		slog.Warn("course_audio: render_cache unmarshal failed, skipping narration", "slug", slug, "error", err)
		return manifest, nil
	}

	voice := synth.Voice()
	for _, step := range rc.Steps {
		for i, seg := range step.Content.Segments {
			if seg.Kind != "teaching" || strings.TrimSpace(seg.Text) == "" {
				continue
			}

			idx := strconv.Itoa(i)
			pieceID := step.StepID + "#" + idx
			key := courseAudioObjectKey(slug, step.StepID, idx, voice, seg.Text)

			exists, err := store.Exists(ctx, key)
			if err != nil {
				slog.Warn("course_audio: Exists check failed, skipping piece", "slug", slug, "pieceId", pieceID, "error", err)
				continue
			}
			if !exists {
				audio, err := synth.Synthesize(ctx, seg.Text, synthSpeed)
				if err != nil {
					slog.Warn("course_audio: Synthesize failed, skipping piece", "slug", slug, "pieceId", pieceID, "error", err)
					continue
				}
				if err := store.PutObject(ctx, key, "audio/mpeg", audio); err != nil {
					slog.Warn("course_audio: PutObject failed, skipping piece", "slug", slug, "pieceId", pieceID, "error", err)
					continue
				}
			}

			manifest[pieceID] = key
		}
	}

	return manifest, nil
}

// courseAudioObjectKey builds the URL-safe (no '#') OSS object key for one
// teaching piece: courses/audio/<slug>/<stepId>_<segIdx>_<hash8>.mp3, where
// hash8 is the first 8 hex chars of sha256(voice + "\n" + text) — mixing the
// voice name in means switching VOICE_TTS_VOICE naturally invalidates every
// cached object instead of silently serving a stale voice under an unchanged
// key.
func courseAudioObjectKey(slug, stepID, segIdx, voice, text string) string {
	sum := sha256.Sum256([]byte(voice + "\n" + text))
	hash8 := hex.EncodeToString(sum[:])[:8]
	return "courses/audio/" + slug + "/" + stepID + "_" + segIdx + "_" + hash8 + ".mp3"
}

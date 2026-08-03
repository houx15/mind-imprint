package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

var errFakeSynth = errors.New("fake synth failure")

// stubCourseAudioSynth records every Synthesize call and returns fixed bytes
// (or a configured error) so tests can assert exactly which pieces were
// synthesized without hitting a real TTS backend.
type stubCourseAudioSynth struct {
	voice    string
	calls    []string // texts passed to Synthesize
	err      error
	audioOut []byte
}

func (s *stubCourseAudioSynth) Synthesize(ctx context.Context, text string, speed float64) ([]byte, error) {
	s.calls = append(s.calls, text)
	if s.err != nil {
		return nil, s.err
	}
	if s.audioOut != nil {
		return s.audioOut, nil
	}
	return []byte("fake-mp3-bytes"), nil
}

func (s *stubCourseAudioSynth) Voice() string { return s.voice }

// stubCourseAudioStore records Exists/PutObject calls; existingKeys lets a
// test simulate objects already present in OSS (idempotency).
type stubCourseAudioStore struct {
	existingKeys map[string]bool
	existsErr    error
	putErr       error
	putCalls     []putCall
}

type putCall struct {
	key         string
	contentType string
	data        []byte
}

func (s *stubCourseAudioStore) Exists(ctx context.Context, key string) (bool, error) {
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return s.existingKeys[key], nil
}

func (s *stubCourseAudioStore) PutObject(ctx context.Context, key, contentType string, data []byte) error {
	s.putCalls = append(s.putCalls, putCall{key: key, contentType: contentType, data: data})
	if s.putErr != nil {
		return s.putErr
	}
	return nil
}

const sampleRenderCache = `{
  "steps": [
    {
      "stepId": "s0",
      "content": {
        "segments": [
          {"kind": "teaching", "text": "hello"},
          {"kind": "structure", "text": "should be skipped"},
          {"kind": "teaching", "text": "   "},
          {"kind": "teaching", "text": "world"}
        ]
      }
    }
  ]
}`

func hash8(voice, text string) string {
	sum := sha256.Sum256([]byte(voice + "\n" + text))
	return hex.EncodeToString(sum[:])[:8]
}

func TestGenerateCourseAudio_GeneratesOnlyNonEmptyTeachingSegments(t *testing.T) {
	synth := &stubCourseAudioSynth{voice: "test-voice"}
	store := &stubCourseAudioStore{existingKeys: map[string]bool{}}

	manifest, err := GenerateCourseAudio(context.Background(), synth, store, "demo-slug", []byte(sampleRenderCache))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(manifest) != 2 {
		t.Fatalf("expected 2 manifest entries, got %d: %+v", len(manifest), manifest)
	}

	wantKey0 := "courses/audio/demo-slug/s0_0_" + hash8("test-voice", "hello") + ".mp3"
	wantKey3 := "courses/audio/demo-slug/s0_3_" + hash8("test-voice", "world") + ".mp3"

	if got := manifest["s0#0"]; got != wantKey0 {
		t.Errorf("manifest[s0#0] = %q, want %q", got, wantKey0)
	}
	if got := manifest["s0#3"]; got != wantKey3 {
		t.Errorf("manifest[s0#3] = %q, want %q", got, wantKey3)
	}
	if _, ok := manifest["s0#1"]; ok {
		t.Errorf("structure segment s0#1 should not be in manifest")
	}
	if _, ok := manifest["s0#2"]; ok {
		t.Errorf("blank teaching segment s0#2 should not be in manifest")
	}

	if len(store.putCalls) != 2 {
		t.Fatalf("expected 2 PutObject calls, got %d: %+v", len(store.putCalls), store.putCalls)
	}
	for _, pc := range store.putCalls {
		if pc.contentType != "audio/mpeg" {
			t.Errorf("PutObject contentType = %q, want audio/mpeg", pc.contentType)
		}
	}
	if store.putCalls[0].key != wantKey0 || store.putCalls[1].key != wantKey3 {
		t.Errorf("PutObject keys = [%q, %q], want [%q, %q]", store.putCalls[0].key, store.putCalls[1].key, wantKey0, wantKey3)
	}

	if len(synth.calls) != 2 || synth.calls[0] != "hello" || synth.calls[1] != "world" {
		t.Errorf("Synthesize calls = %+v, want [hello, world]", synth.calls)
	}
}

func TestGenerateCourseAudio_ExistingObjectSkipsSynthesize(t *testing.T) {
	synth := &stubCourseAudioSynth{voice: "test-voice"}
	wantKey0 := "courses/audio/demo-slug/s0_0_" + hash8("test-voice", "hello") + ".mp3"
	store := &stubCourseAudioStore{existingKeys: map[string]bool{wantKey0: true}}

	manifest, err := GenerateCourseAudio(context.Background(), synth, store, "demo-slug", []byte(sampleRenderCache))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// s0#0 already exists in the store, so Synthesize must not be called for
	// "hello" — but the manifest still records its key (idempotent re-run).
	for _, text := range synth.calls {
		if text == "hello" {
			t.Fatalf("Synthesize should not be called for already-existing object, calls=%+v", synth.calls)
		}
	}
	if got := manifest["s0#0"]; got != wantKey0 {
		t.Errorf("manifest[s0#0] = %q, want %q (still recorded even though skipped)", got, wantKey0)
	}
	// s0#3 ("world") did not exist, so it should still be synthesized.
	found := false
	for _, text := range synth.calls {
		if text == "world" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Synthesize called for world, calls=%+v", synth.calls)
	}
	// Only one PutObject call (for the piece that didn't already exist).
	if len(store.putCalls) != 1 {
		t.Fatalf("expected 1 PutObject call, got %d: %+v", len(store.putCalls), store.putCalls)
	}
}

func TestGenerateCourseAudio_NilSynthOrStoreReturnsEmptyManifest(t *testing.T) {
	store := &stubCourseAudioStore{existingKeys: map[string]bool{}}
	manifest, err := GenerateCourseAudio(context.Background(), nil, store, "demo-slug", []byte(sampleRenderCache))
	if err != nil {
		t.Fatalf("unexpected error with nil synth: %v", err)
	}
	if len(manifest) != 0 {
		t.Errorf("expected empty manifest with nil synth, got %+v", manifest)
	}

	synth := &stubCourseAudioSynth{voice: "test-voice"}
	manifest, err = GenerateCourseAudio(context.Background(), synth, nil, "demo-slug", []byte(sampleRenderCache))
	if err != nil {
		t.Fatalf("unexpected error with nil store: %v", err)
	}
	if len(manifest) != 0 {
		t.Errorf("expected empty manifest with nil store, got %+v", manifest)
	}
	if len(synth.calls) != 0 {
		t.Errorf("Synthesize should not be called when store is nil, calls=%+v", synth.calls)
	}
}

func TestGenerateCourseAudio_SynthesizeErrorSkipsPieceNoOverallError(t *testing.T) {
	synth := &stubCourseAudioSynth{voice: "test-voice", err: errFakeSynth}
	store := &stubCourseAudioStore{existingKeys: map[string]bool{}}

	manifest, err := GenerateCourseAudio(context.Background(), synth, store, "demo-slug", []byte(sampleRenderCache))
	if err != nil {
		t.Fatalf("individual synth failure should not surface as an overall error, got %v", err)
	}
	if len(manifest) != 0 {
		t.Errorf("expected no manifest entries when every synth call fails, got %+v", manifest)
	}
	if len(store.putCalls) != 0 {
		t.Errorf("PutObject should not be called when Synthesize failed, got %+v", store.putCalls)
	}
}

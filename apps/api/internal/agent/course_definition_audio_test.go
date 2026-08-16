package agent

// course_definition_audio_test.go — Task 5 of the course authoring & publish
// lifecycle: GenerateDefinitionAudio. Reuses stubCourseAudioSynth /
// stubCourseAudioStore (course_audio_test.go, same package) — no DB, no real
// TTS/OSS backend.

import (
	"context"
	"testing"
)

// definitionAudioFixture is a small CourseDefinition 2.0 document: opening
// fallback (with audio), closing fallback (WITHOUT audio — must be skipped),
// and one part/slice with two narrations.
const definitionAudioFixture = `{
	"schemaVersion": "2.0",
	"course": {
		"id": "gen-course",
		"opening": {"fallback": {"text": "Welcome to the course.", "audio": "opening.mp3"}},
		"closing": {"fallback": {"text": "You're done.", "audio": ""}},
		"parts": [
			{
				"slices": [
					{
						"narrations": [
							{"text": "First narration.", "audio": "p1/s1/n1.mp3"},
							{"text": "Second narration.", "audio": "p1/s1/n2.mp3"}
						]
					}
				]
			}
		]
	}
}`

func TestGenerateDefinitionAudio(t *testing.T) {
	synth := &stubCourseAudioSynth{voice: "test-voice"}
	store := &stubCourseAudioStore{existingKeys: map[string]bool{}}

	n, err := GenerateDefinitionAudio(context.Background(), synth, store, "gen-course", []byte(definitionAudioFixture))
	if err != nil {
		t.Fatalf("GenerateDefinitionAudio: %v", err)
	}
	if n != 3 {
		t.Fatalf("generated = %d, want 3 (opening fallback + 2 narrations; closing fallback has no audio path)", n)
	}
	if len(store.putCalls) != 3 {
		t.Fatalf("PutObject calls = %d, want 3", len(store.putCalls))
	}

	wantKeys := map[string]bool{
		"courses/gen-course/opening.mp3":  false,
		"courses/gen-course/p1/s1/n1.mp3": false,
		"courses/gen-course/p1/s1/n2.mp3": false,
	}
	for _, c := range store.putCalls {
		if _, ok := wantKeys[c.key]; !ok {
			t.Fatalf("unexpected PutObject key %q", c.key)
		}
		wantKeys[c.key] = true
		if c.contentType != "audio/mpeg" {
			t.Fatalf("PutObject(%q) contentType = %q, want audio/mpeg", c.key, c.contentType)
		}
	}
	for key, called := range wantKeys {
		if !called {
			t.Fatalf("expected PutObject for key %q, never called", key)
		}
	}

	if len(synth.calls) != 3 {
		t.Fatalf("Synthesize calls = %d, want 3", len(synth.calls))
	}
}

// TestGenerateDefinitionAudioNilDegrades asserts a nil synth or nil store
// (voice/OSS not configured) degrades to (0, nil) — ship must never fail
// publish just because narration is unavailable.
func TestGenerateDefinitionAudioNilDegrades(t *testing.T) {
	store := &stubCourseAudioStore{existingKeys: map[string]bool{}}
	n, err := GenerateDefinitionAudio(context.Background(), nil, store, "gen-course", []byte(definitionAudioFixture))
	if err != nil || n != 0 {
		t.Fatalf("nil synth: got (%d, %v), want (0, nil)", n, err)
	}
	if len(store.putCalls) != 0 {
		t.Fatalf("nil synth: store.PutObject called %d times, want 0", len(store.putCalls))
	}

	synth := &stubCourseAudioSynth{voice: "test-voice"}
	n, err = GenerateDefinitionAudio(context.Background(), synth, nil, "gen-course", []byte(definitionAudioFixture))
	if err != nil || n != 0 {
		t.Fatalf("nil store: got (%d, %v), want (0, nil)", n, err)
	}
	if len(synth.calls) != 0 {
		t.Fatalf("nil store: synth.Synthesize called %d times, want 0", len(synth.calls))
	}
}

// TestGenerateDefinitionAudioEmptyClipsSkipped asserts a narration with empty
// text or empty audio path is skipped, not synthesized/uploaded.
func TestGenerateDefinitionAudioEmptyClipsSkipped(t *testing.T) {
	doc := `{
		"schemaVersion": "2.0",
		"course": {
			"id": "gen-course-empty",
			"parts": [
				{"slices": [{"narrations": [
					{"text": "", "audio": "p1/s1/n1.mp3"},
					{"text": "has text no audio", "audio": ""},
					{"text": "keep me", "audio": "p1/s1/n3.mp3"}
				]}]}
			]
		}
	}`
	synth := &stubCourseAudioSynth{voice: "test-voice"}
	store := &stubCourseAudioStore{existingKeys: map[string]bool{}}

	n, err := GenerateDefinitionAudio(context.Background(), synth, store, "gen-course-empty", []byte(doc))
	if err != nil {
		t.Fatalf("GenerateDefinitionAudio: %v", err)
	}
	if n != 1 {
		t.Fatalf("generated = %d, want 1", n)
	}
	if len(store.putCalls) != 1 || store.putCalls[0].key != "courses/gen-course-empty/p1/s1/n3.mp3" {
		t.Fatalf("putCalls = %+v, want single call for n3.mp3", store.putCalls)
	}
}

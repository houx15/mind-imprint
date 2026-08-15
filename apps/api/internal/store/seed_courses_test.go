package store_test

// seed_courses_test.go — Task 12's real-DB pass for the course example seed:
// after SeedCourses runs against a fresh (migrated, empty) database,
// ListCourses must return both a-mid and b-mid with the card_ids/step_count/
// title the brief specifies. Mirrors internal/agent/coursestore_test.go's
// pattern (real embedded JSON, real store, testcontainers), but exercises
// the seed ENTRY POINT (store.SeedCourses) rather than UpsertCourse directly.

import (
	"context"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
)

func TestSeedCourses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)

	// nil, nil: no voice/OSS configured — narration (Task 4) degrades to an
	// empty manifest per course, exactly like an unconfigured production
	// deploy; seeding itself must still succeed unchanged.
	n, err := store.SeedCourses(ctx, pool, nil, nil)
	if err != nil {
		t.Fatalf("SeedCourses: %v", err)
	}
	// 3 = a-mid + b-mid (legacy render_cache) + the golden CourseDefinition 2.0
	// course seeded for the runtime player (Course Runtime Slice 8).
	if n != 3 {
		t.Fatalf("SeedCourses returned %d, want 3", n)
	}

	q := sqlc.New(pool)
	agentStore := agent.NewSqlcAgentStore(q, pool)
	courses, err := agentStore.ListCourses(ctx)
	if err != nil {
		t.Fatalf("ListCourses: %v", err)
	}
	if len(courses) != 3 {
		t.Fatalf("ListCourses len = %d, want 3", len(courses))
	}

	bySlug := map[string]agent.CourseSummaryRow{}
	for _, c := range courses {
		bySlug[c.Slug] = c
	}

	aMid, ok := bySlug["a-mid"]
	if !ok {
		t.Fatalf("a-mid not seeded; got slugs %v", bySlug)
	}
	if aMid.Branch != "A" {
		t.Fatalf("a-mid.Branch = %q, want \"A\"", aMid.Branch)
	}
	if aMid.Title == "" {
		t.Fatalf("a-mid.Title is empty")
	}
	if len(aMid.CardIDs) != 1 || aMid.CardIDs[0] != "craap" {
		t.Fatalf("a-mid.CardIDs = %v, want [craap]", aMid.CardIDs)
	}
	if aMid.StepCount <= 0 {
		t.Fatalf("a-mid.StepCount = %d, want > 0", aMid.StepCount)
	}

	// synth/audioStore were nil above, so narration generation must have
	// degraded to an empty manifest — the seed must not error or leave a
	// non-empty manifest just because GenerateCourseAudio was skipped.
	payload, _, err := agentStore.GetCoursePayload(ctx, "a-mid")
	if err != nil {
		t.Fatalf("GetCoursePayload(a-mid): %v", err)
	}
	if len(payload.AudioManifest) != 0 {
		t.Fatalf("a-mid.AudioManifest = %v, want empty (nil voice/OSS)", payload.AudioManifest)
	}

	bMid, ok := bySlug["b-mid"]
	if !ok {
		t.Fatalf("b-mid not seeded; got slugs %v", bySlug)
	}
	if bMid.Branch != "B" {
		t.Fatalf("b-mid.Branch = %q, want \"B\"", bMid.Branch)
	}
	if bMid.Title == "" {
		t.Fatalf("b-mid.Title is empty")
	}
	wantCards := []string{"sift", "craap", "argument-map", "concession", "metacognition"}
	if len(bMid.CardIDs) != len(wantCards) {
		t.Fatalf("b-mid.CardIDs = %v, want %v", bMid.CardIDs, wantCards)
	}
	for i, id := range wantCards {
		if bMid.CardIDs[i] != id {
			t.Fatalf("b-mid.CardIDs = %v, want %v", bMid.CardIDs, wantCards)
		}
	}
	if bMid.StepCount <= 0 {
		t.Fatalf("b-mid.StepCount = %d, want > 0", bMid.StepCount)
	}

	// Idempotent: calling SeedCourses again does not error and does not
	// duplicate rows (upsert-by-slug).
	n2, err := store.SeedCourses(ctx, pool, nil, nil)
	if err != nil {
		t.Fatalf("SeedCourses (second call): %v", err)
	}
	if n2 != 3 {
		t.Fatalf("SeedCourses (second call) returned %d, want 3", n2)
	}
	coursesAgain, err := agentStore.ListCourses(ctx)
	if err != nil {
		t.Fatalf("ListCourses (second call): %v", err)
	}
	if len(coursesAgain) != 3 {
		t.Fatalf("ListCourses (second call) len = %d, want 3 (upsert should not duplicate)", len(coursesAgain))
	}
}

// stubCourseAudioSynth/stubCourseAudioStore are minimal, fully in-memory
// stand-ins for agent.CourseAudioSynth/CourseAudioStore — no real TTS call
// or network I/O, per this task's own constraint against hitting real
// services from tests. They let TestSeedCoursesGeneratesAudioManifest prove
// SeedCourses actually calls agent.GenerateCourseAudio per course and stores
// its result, something TestSeedCourses's nil/nil pass can't exercise.
type stubCourseAudioSynth struct{ calls int }

func (s *stubCourseAudioSynth) Synthesize(_ context.Context, _ string, _ float64) ([]byte, error) {
	s.calls++
	return []byte("fake-mp3-bytes"), nil
}

func (s *stubCourseAudioSynth) Voice() string { return "stub-voice" }

type stubCourseAudioStore struct {
	existing map[string]bool
	puts     int
}

func (s *stubCourseAudioStore) Exists(_ context.Context, key string) (bool, error) {
	return s.existing[key], nil
}

func (s *stubCourseAudioStore) PutObject(_ context.Context, key, _ string, _ []byte) error {
	s.puts++
	if s.existing == nil {
		s.existing = map[string]bool{}
	}
	s.existing[key] = true
	return nil
}

// TestSeedCoursesGeneratesAudioManifest asserts SeedCourses's Task-4 wiring:
// given a non-nil synth/store, it calls agent.GenerateCourseAudio for each
// seeded course's render_cache and persists the resulting manifest via
// UpsertCourse — readable back through GetCoursePayload. a-mid's embedded
// render cache has 21 non-empty "teaching" segments (verified by inspection),
// so a non-empty manifest here is a real assertion, not a coincidence of an
// empty course.
func TestSeedCoursesGeneratesAudioManifest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)

	synth := &stubCourseAudioSynth{}
	audioStore := &stubCourseAudioStore{}

	n, err := store.SeedCourses(ctx, pool, synth, audioStore)
	if err != nil {
		t.Fatalf("SeedCourses: %v", err)
	}
	// 3 = a-mid + b-mid (legacy render_cache) + the golden CourseDefinition 2.0
	// course seeded for the runtime player (Course Runtime Slice 8).
	if n != 3 {
		t.Fatalf("SeedCourses returned %d, want 3", n)
	}
	if synth.calls == 0 {
		t.Fatalf("stub synth was never called; SeedCourses did not invoke GenerateCourseAudio")
	}
	if audioStore.puts == 0 {
		t.Fatalf("stub store never received a PutObject; SeedCourses did not invoke GenerateCourseAudio")
	}

	q := sqlc.New(pool)
	agentStore := agent.NewSqlcAgentStore(q, pool)
	payload, _, err := agentStore.GetCoursePayload(ctx, "a-mid")
	if err != nil {
		t.Fatalf("GetCoursePayload(a-mid): %v", err)
	}
	if len(payload.AudioManifest) == 0 {
		t.Fatalf("a-mid.AudioManifest is empty, want a non-empty pieceId->objectKey manifest")
	}
	for pieceID, key := range payload.AudioManifest {
		if key == "" {
			t.Fatalf("a-mid.AudioManifest[%q] is empty", pieceID)
		}
	}
}

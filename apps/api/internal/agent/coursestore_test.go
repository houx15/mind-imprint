package agent_test

// coursestore_test.go — Task 4's real-DB pass for the course v2 content
// store: upsert-from-embedded-JSON, list/payload reads, progress union +
// started_at-set-once, and the report's step-titles + quiz tally. Replaces
// the pre-course-v2 coursestore_test.go (session-table era, deleted by
// Task 3 along with the table it exercised).

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	courses "mindimprint/api/internal/store/seed/courses"
	"mindimprint/api/internal/store/sqlc"
)

// amidAuthoredInteractionTotal sums len(content.interactions) across every
// render-cache step — the same computation CourseReport does — so this test
// asserts against the REAL a-mid count rather than the brief's placeholder.
func amidAuthoredInteractionTotal(t *testing.T, renderCache []byte) int {
	t.Helper()
	var cache struct {
		Steps []struct {
			Content struct {
				Interactions []json.RawMessage `json:"interactions"`
			} `json:"content"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(renderCache, &cache); err != nil {
		t.Fatalf("parse render cache: %v", err)
	}
	total := 0
	for _, s := range cache.Steps {
		total += len(s.Content.Interactions)
	}
	return total
}

func TestCourseStoreV2(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	st := agent.NewSqlcAgentStore(q, pool)
	ctx := context.Background()
	userID := seededStudentID

	raw, err := courses.FS.ReadFile("a-mid.json")
	if err != nil {
		t.Fatalf("read a-mid.json: %v", err)
	}
	rc, err := courses.FS.ReadFile("a-mid-render-cache.json")
	if err != nil {
		t.Fatalf("read a-mid-render-cache.json: %v", err)
	}
	wantTotal := amidAuthoredInteractionTotal(t, rc)

	wantAudioManifest := map[string]string{"s0#0": "courses/audio/a-mid/s0_0_ab12cd34.mp3"}
	if err := st.UpsertCourse(ctx, agent.UpsertCourseInput{
		Slug: "a-mid", Branch: "A", Title: "CRRAAB 信源评估：从机构到亲历者到专家",
		CardIDs: []string{"craap"}, Structure: raw, RenderCache: rc, StepCount: 4,
		AudioManifest: wantAudioManifest,
	}); err != nil {
		t.Fatalf("UpsertCourse: %v", err)
	}

	sums, err := st.ListCourses(ctx)
	if err != nil {
		t.Fatalf("ListCourses: %v", err)
	}
	if len(sums) != 1 {
		t.Fatalf("ListCourses len = %d, want 1", len(sums))
	}
	if sums[0].StepCount != 4 {
		t.Fatalf("StepCount = %d, want 4", sums[0].StepCount)
	}
	if len(sums[0].CardIDs) != 1 || sums[0].CardIDs[0] != "craap" {
		t.Fatalf("CardIDs = %v, want [craap]", sums[0].CardIDs)
	}

	payload, cu, err := st.GetCoursePayload(ctx, "a-mid")
	if err != nil {
		t.Fatalf("GetCoursePayload: %v", err)
	}
	if cu == uuid.Nil {
		t.Fatalf("GetCoursePayload course uuid is nil")
	}
	if len(payload.Structure) == 0 || len(payload.RenderCache) == 0 {
		t.Fatalf("GetCoursePayload structure/renderCache empty")
	}
	if !reflect.DeepEqual(payload.AudioManifest, wantAudioManifest) {
		t.Fatalf("GetCoursePayload AudioManifest = %v, want %v", payload.AudioManifest, wantAudioManifest)
	}

	// First step done: current=0 resume, mark step 0 complete, +10 active s.
	// stepCount=2, so 1 of 2 done → NOT finished yet (completed_at nil).
	ord0 := 0
	firstProgress, err := st.SaveProgress(ctx, userID, cu, 0, &ord0, 10, 2)
	if err != nil {
		t.Fatalf("SaveProgress(0): %v", err)
	}
	if firstProgress.StartedAt == nil {
		t.Fatalf("SaveProgress(0): started_at not set")
	}
	if firstProgress.CompletedAt != nil {
		t.Fatalf("SaveProgress(0): completed_at set with only 1/2 steps done")
	}
	if firstProgress.ActiveSeconds != 10 {
		t.Fatalf("ActiveSeconds = %d, want 10", firstProgress.ActiveSeconds)
	}
	firstStartedAt := *firstProgress.StartedAt

	// Second step done: current=1 resume, mark step 1 complete, +5 active s.
	// Now 2 of 2 done → finished (completed_at set). active accumulates to 15.
	ord1 := 1
	secondProgress, err := st.SaveProgress(ctx, userID, cu, 1, &ord1, 5, 2)
	if err != nil {
		t.Fatalf("SaveProgress(1): %v", err)
	}
	if secondProgress.StartedAt == nil || !secondProgress.StartedAt.Equal(firstStartedAt) {
		t.Fatalf("started_at changed on second SaveProgress: first=%v second=%v", firstStartedAt, secondProgress.StartedAt)
	}
	if secondProgress.CompletedAt == nil {
		t.Fatalf("SaveProgress(1): completed_at not set when all steps done")
	}
	if secondProgress.ActiveSeconds != 15 {
		t.Fatalf("ActiveSeconds = %d, want 15 (10+5 accumulated)", secondProgress.ActiveSeconds)
	}
	wantOrdinals := map[int]bool{0: true, 1: true}
	if len(secondProgress.CompletedOrdinals) != 2 {
		t.Fatalf("CompletedOrdinals = %v, want union of {0,1}", secondProgress.CompletedOrdinals)
	}
	for _, o := range secondProgress.CompletedOrdinals {
		if !wantOrdinals[o] {
			t.Fatalf("CompletedOrdinals = %v, want union of {0,1}", secondProgress.CompletedOrdinals)
		}
	}

	if err := st.LogCourseEvent(ctx, userID, cu, "course_quiz_answered",
		[]byte(`{"stepId":"step_01","interactionId":"intro_q1","selected":["B"],"correct":true}`)); err != nil {
		t.Fatalf("LogCourseEvent(correct): %v", err)
	}
	if err := st.LogCourseEvent(ctx, userID, cu, "course_quiz_answered",
		[]byte(`{"stepId":"step_01","interactionId":"intro_q2","selected":["A"],"correct":false}`)); err != nil {
		t.Fatalf("LogCourseEvent(incorrect): %v", err)
	}

	rep, err := st.CourseReport(ctx, userID, "a-mid")
	if err != nil {
		t.Fatalf("CourseReport: %v", err)
	}
	if rep.Quiz.Correct != 1 {
		t.Fatalf("Quiz.Correct = %d, want 1", rep.Quiz.Correct)
	}
	if rep.Quiz.Total != wantTotal {
		t.Fatalf("Quiz.Total = %d, want %d (real a-mid authored interaction count)", rep.Quiz.Total, wantTotal)
	}
	found := false
	for _, title := range rep.CompletedStepTitles {
		if title == "为什么需要信息过滤神器？" {
			found = true
		}
	}
	if !found {
		t.Fatalf("CompletedStepTitles = %v, want to contain step_01's title", rep.CompletedStepTitles)
	}
	if len(rep.CardIDs) != 1 || rep.CardIDs[0] != "craap" {
		t.Fatalf("CardIDs = %v, want [craap]", rep.CardIDs)
	}
}

// TestCourseStoreV2_LogCourseEventDedupesLastAttemptWins proves the report's
// documented dedup rule: two answered events for the SAME interactionId only
// count once, and the LAST attempt (by created_at) wins — a student who got
// it wrong then right is credited correct, not double-counted.
func TestCourseStoreV2_LogCourseEventDedupesLastAttemptWins(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	st := agent.NewSqlcAgentStore(q, pool)
	ctx := context.Background()
	userID := seededStudentID

	raw, err := courses.FS.ReadFile("a-mid.json")
	if err != nil {
		t.Fatalf("read a-mid.json: %v", err)
	}
	rc, err := courses.FS.ReadFile("a-mid-render-cache.json")
	if err != nil {
		t.Fatalf("read a-mid-render-cache.json: %v", err)
	}
	if err := st.UpsertCourse(ctx, agent.UpsertCourseInput{
		Slug: "a-mid-dedup", Branch: "A", Title: "dedup test",
		CardIDs: []string{"craap"}, Structure: raw, RenderCache: rc, StepCount: 4,
	}); err != nil {
		t.Fatalf("UpsertCourse: %v", err)
	}
	payload, cu, err := st.GetCoursePayload(ctx, "a-mid-dedup")
	if err != nil {
		t.Fatalf("GetCoursePayload: %v", err)
	}
	if payload.AudioManifest == nil || len(payload.AudioManifest) != 0 {
		t.Fatalf("GetCoursePayload AudioManifest = %v, want empty non-nil map when not upserted", payload.AudioManifest)
	}

	if err := st.LogCourseEvent(ctx, userID, cu, "course_quiz_answered",
		[]byte(`{"stepId":"step_01","interactionId":"intro_q1","selected":["A"],"correct":false}`)); err != nil {
		t.Fatalf("LogCourseEvent(first attempt): %v", err)
	}
	if err := st.LogCourseEvent(ctx, userID, cu, "course_quiz_answered",
		[]byte(`{"stepId":"step_01","interactionId":"intro_q1","selected":["B"],"correct":true}`)); err != nil {
		t.Fatalf("LogCourseEvent(retry, correct): %v", err)
	}

	rep, err := st.CourseReport(ctx, userID, "a-mid-dedup")
	if err != nil {
		t.Fatalf("CourseReport: %v", err)
	}
	if rep.Quiz.Correct != 1 {
		t.Fatalf("Quiz.Correct = %d, want 1 (dedup by interactionId, last attempt wins)", rep.Quiz.Correct)
	}
}

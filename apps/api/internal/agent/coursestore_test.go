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
	"strconv"
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

	sums, err := st.ListCourses(ctx, false)
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

// TestUpsertCourseDefinitionPersistsCatalogMetadata proves the new catalog
// taxonomy columns (category/introduction, migration in Task 1) round-trip
// through UpsertCourseDefinition → ListCourses.
func TestUpsertCourseDefinitionPersistsCatalogMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)
	ctx := context.Background()

	cat := "source-check"
	intro := []byte(`{"hook":"h","takeaways":["a","b"]}`)
	if _, err := store.UpsertCourseDefinition(ctx, agent.UpsertCourseDefinitionInput{
		Slug: "cat-meta", Branch: "Runtime", Title: "T", Blurb: "b",
		CardIDs: []string{"craap"}, Definition: []byte(`{"schemaVersion":"2.0","course":{"id":"cat-meta","title":"T"}}`),
		Category: &cat, Introduction: intro,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	rows, err := store.ListCourses(ctx, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var got *agent.CourseSummaryRow
	for i := range rows {
		if rows[i].Slug == "cat-meta" {
			got = &rows[i]
		}
	}
	if got == nil {
		t.Fatal("course not listed")
	}
	if got.Category == nil || *got.Category != "source-check" {
		t.Fatalf("category = %v, want source-check", got.Category)
	}
	if len(got.Introduction) == 0 {
		t.Fatal("introduction not persisted")
	}
}

// TestGetProgressDerivesFromSession proves a 2.0 course's catalog progress comes
// from course_session.sliceStates (not course_progress, which it never writes):
// the completed-slice count drives CompletedOrdinals' length, and a finished
// session clamps to step_count so the ring reads 100%.
func TestGetProgressDerivesFromSession(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)
	ctx := context.Background()

	// A 2.0 course with 4 authored steps.
	if _, err := store.UpsertCourseDefinition(ctx, agent.UpsertCourseDefinitionInput{
		Slug: "sess-prog", Branch: "Runtime", Title: "T", Blurb: "b",
		CardIDs:    []string{},
		Definition: []byte(`{"schemaVersion":"2.0","course":{"id":"sess-prog","title":"T"}}`),
		StepCount:  4,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	_, courseID, err := store.GetCoursePayload(ctx, "sess-prog")
	if err != nil {
		t.Fatalf("GetCoursePayload: %v", err)
	}
	userID := seededStudentID

	// Session with 2 of 4 slices completed → progress length 2.
	session := func(status string, completedSlices int) []byte {
		states := ""
		for i := 0; i < completedSlices; i++ {
			if i > 0 {
				states += ","
			}
			states += `"s` + strconv.Itoa(i) + `":{"status":"completed"}`
		}
		return []byte(`{"id":"x","courseId":"sess-prog","courseSchemaVersion":"2.0","studentId":"u",` +
			`"status":"` + status + `","sliceStates":{` + states + `},"events":[]}`)
	}
	// Insert the session row first (SaveCourseSession is UPDATE-only; production
	// get-or-creates before any snapshot save).
	if _, err := store.CreateCourseSession(ctx, userID, courseID, session("in-progress", 2), "in-progress"); err != nil {
		t.Fatalf("CreateCourseSession: %v", err)
	}
	p, err := store.GetProgress(ctx, userID, "sess-prog")
	if err != nil {
		t.Fatalf("GetProgress: %v", err)
	}
	if len(p.CompletedOrdinals) != 2 {
		t.Fatalf("in-progress: CompletedOrdinals len = %d, want 2", len(p.CompletedOrdinals))
	}
	if p.CompletedAt != nil {
		t.Fatalf("in-progress: CompletedAt should be nil, got %v", p.CompletedAt)
	}

	// A finished session clamps to step_count (4) and sets CompletedAt, even
	// though only 3 slice states are marked (the closing flip can outrun the
	// last slice's own state).
	if err := store.SaveCourseSession(ctx, userID, courseID, session("completed", 3), "completed"); err != nil {
		t.Fatalf("SaveCourseSession (completed): %v", err)
	}
	p2, err := store.GetProgress(ctx, userID, "sess-prog")
	if err != nil {
		t.Fatalf("GetProgress (completed): %v", err)
	}
	if len(p2.CompletedOrdinals) != 4 {
		t.Fatalf("completed: CompletedOrdinals len = %d, want 4 (clamped to step_count)", len(p2.CompletedOrdinals))
	}
	if p2.CompletedAt == nil {
		t.Fatalf("completed: CompletedAt should be set")
	}

	// Learning history reflects the same completed-slice count for the runtime
	// course (was hardcoded 0 for runtime before this change). The session is
	// now "completed" → clamps to step_count 4.
	hist, err := store.ListCourseHistory(ctx, userID)
	if err != nil {
		t.Fatalf("ListCourseHistory: %v", err)
	}
	var found bool
	for _, h := range hist {
		if h.Slug == "sess-prog" {
			found = true
			if h.CompletedCount != 4 {
				t.Fatalf("history CompletedCount = %d, want 4", h.CompletedCount)
			}
		}
	}
	if !found {
		t.Fatalf("sess-prog not in learning history")
	}
}

// TestCourseAttemptLog proves the 0077 attempt-log model: relearning a finished
// 2.0 course KEEPS the finished attempt (with a frozen completion date and its own
// report, fetched by attempt id) and mints a NEW current attempt beside it, while
// the catalog ring and resume always read the latest attempt.
func TestCourseAttemptLog(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)
	ctx := context.Background()
	userID := seededStudentID

	// A 2.0 course with two named slices, so a report carries real titles.
	def := []byte(`{"schemaVersion":"2.0","course":{"id":"attempt-log","title":"T",` +
		`"objectives":[{"text":"the goal"}],"parts":[{"slices":[` +
		`{"id":"s0","title":"Slice Zero"},{"id":"s1","title":"Slice One"}]}]}}`)
	if _, err := store.UpsertCourseDefinition(ctx, agent.UpsertCourseDefinitionInput{
		Slug: "attempt-log", Branch: "Runtime", Title: "T", Blurb: "b",
		CardIDs: []string{}, Definition: def, StepCount: 2,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	_, courseID, err := store.GetCoursePayload(ctx, "attempt-log")
	if err != nil {
		t.Fatalf("GetCoursePayload: %v", err)
	}

	// Attempt #1: create, then finish it (both slices completed, 15s spent).
	if _, err := store.CreateCourseSession(ctx, userID, courseID,
		[]byte(`{"id":"a1","courseId":"attempt-log","courseSchemaVersion":"2.0","studentId":"u","status":"in-progress","sliceStates":{},"events":[]}`),
		"in-progress"); err != nil {
		t.Fatalf("CreateCourseSession #1: %v", err)
	}
	finished := []byte(`{"id":"a1","courseId":"attempt-log","courseSchemaVersion":"2.0","studentId":"u","status":"completed",` +
		`"sliceStates":{"s0":{"status":"completed","elapsedSeconds":10},"s1":{"status":"completed","elapsedSeconds":5}},"events":[]}`)
	if err := store.SaveCourseSession(ctx, userID, courseID, finished, "completed"); err != nil {
		t.Fatalf("SaveCourseSession completed: %v", err)
	}

	// History now has one finished attempt with a FROZEN completion date.
	hist1, err := store.ListCourseHistory(ctx, userID)
	if err != nil {
		t.Fatalf("ListCourseHistory #1: %v", err)
	}
	var attempt1ID string
	var seen int
	for _, h := range hist1 {
		if h.Slug != "attempt-log" {
			continue
		}
		seen++
		attempt1ID = h.AttemptID
		if h.Status != "completed" || h.CompletedAt == nil {
			t.Fatalf("attempt #1: want completed with a completion date, got status=%q completedAt=%v", h.Status, h.CompletedAt)
		}
	}
	if seen != 1 {
		t.Fatalf("history after finish: want 1 attempt-log row, got %d", seen)
	}
	if attempt1ID == "" {
		t.Fatalf("attempt #1 has no attempt id")
	}

	// The finished run's report is reachable by its attempt id: both titles, 15s.
	rep1, ok, err := store.CourseReport20(ctx, userID, "attempt-log", attempt1ID)
	if err != nil || !ok {
		t.Fatalf("CourseReport20(attempt1): ok=%v err=%v", ok, err)
	}
	if len(rep1.Data.CompletedStepTitles) != 2 || rep1.Data.SecondsSpent != 15 {
		t.Fatalf("attempt #1 report: titles=%v seconds=%d, want 2 titles / 15s", rep1.Data.CompletedStepTitles, rep1.Data.SecondsSpent)
	}

	// RELEARN: an already-completed current attempt is kept (delete is a no-op),
	// then a fresh 'created' attempt is minted — exactly what postCourseRestart does.
	if err := store.DeleteIncompleteLatestCourseSession(ctx, userID, courseID); err != nil {
		t.Fatalf("DeleteIncompleteLatest (completed → no-op): %v", err)
	}
	if _, err := store.CreateCourseSession(ctx, userID, courseID,
		[]byte(`{"id":"a2","courseId":"attempt-log","courseSchemaVersion":"2.0","studentId":"u","status":"created","sliceStates":{},"events":[]}`),
		"created"); err != nil {
		t.Fatalf("CreateCourseSession #2 (relearn): %v", err)
	}

	// Resume reads the LATEST attempt — the fresh one, not the finished #1.
	if _, status, found, gerr := store.GetCourseSession(ctx, userID, "attempt-log"); gerr != nil || !found || status != "created" {
		t.Fatalf("GetCourseSession after relearn: status=%q found=%v err=%v, want the fresh 'created' attempt", status, found, gerr)
	}

	// History now shows BOTH attempts; the catalog ring shows only ONE (latest).
	hist2, err := store.ListCourseHistory(ctx, userID)
	if err != nil {
		t.Fatalf("ListCourseHistory #2: %v", err)
	}
	var completedRows, inProgressRows int
	for _, h := range hist2 {
		if h.Slug != "attempt-log" {
			continue
		}
		if h.CompletedAt != nil {
			completedRows++
		} else {
			inProgressRows++
		}
	}
	if completedRows != 1 || inProgressRows != 1 {
		t.Fatalf("history after relearn: want 1 completed + 1 in-progress attempt-log row, got %d + %d", completedRows, inProgressRows)
	}
	prog, err := store.ListProgressForUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListProgressForUser: %v", err)
	}
	if p, ok := prog["attempt-log"]; !ok {
		t.Fatalf("catalog progress missing attempt-log")
	} else if p.Status != "in-progress" {
		t.Fatalf("catalog ring should reflect the CURRENT attempt (in-progress), got %q", p.Status)
	}

	// A finished attempt #1's frozen report still resolves after the relearn — the
	// new attempt did not clobber it.
	if _, ok, err := store.CourseReport20(ctx, userID, "attempt-log", attempt1ID); err != nil || !ok {
		t.Fatalf("attempt #1 report after relearn: ok=%v err=%v", ok, err)
	}
	// An unknown attempt id is a 404 (pgx.ErrNoRows), never a fall-through to latest.
	if _, _, err := store.CourseReport20(ctx, userID, "attempt-log", uuid.NewString()); err == nil {
		t.Fatalf("CourseReport20 with an unknown attempt id: want error, got nil")
	}
}

// TestCourseAnswerReport20 proves the per-attempt answer detail reads the
// student's recorded final answers + per-slice time back out of the stored
// session blob + the definition: a graded singleChoice maps the stored option id
// to its label and grades it against correctOptionId; a graded fillBlank grades
// its stored text (case-insensitive) against acceptedAnswers; a non-assessment
// block is omitted; and a Slice with no assessment blocks drops out entirely.
func TestCourseAnswerReport20(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)
	ctx := context.Background()
	userID := seededStudentID

	// A 2.0 course: one graded singleChoice + one graded fillBlank + a text block
	// in slice s0, plus a second slice s1 with only a text block (must drop out).
	def := []byte(`{"schemaVersion":"2.0","course":{"id":"ans","title":"T","parts":[{"slices":[` +
		`{"id":"s0","title":"Slice Zero","blocks":[` +
		`{"id":"q1","type":"singleChoice","prompt":"选哪个来源？","options":[{"id":"a","label":"看作者"},{"id":"b","label":"看标题"}],"assessment":{"mode":"graded","correctOptionId":"a"}},` +
		`{"id":"q2","type":"fillBlank","prompt":"横向溯源叫什么？","assessment":{"mode":"graded","acceptedAnswers":["lateral reading"]}},` +
		`{"id":"t1","type":"text","content":"hi"}` +
		`]},` +
		`{"id":"s1","title":"Slice One","blocks":[{"id":"t2","type":"text","content":"bye"}]}` +
		`]}]}}`)
	if _, err := store.UpsertCourseDefinition(ctx, agent.UpsertCourseDefinitionInput{
		Slug: "ans", Branch: "Runtime", Title: "T", Blurb: "b",
		CardIDs: []string{}, Definition: def, StepCount: 2,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	_, courseID, err := store.GetCoursePayload(ctx, "ans")
	if err != nil {
		t.Fatalf("GetCoursePayload: %v", err)
	}

	// The session: q1 answered "a" (correct, 1 attempt); q2 answered "Lateral Reading"
	// (correct via case-insensitive match, 2 attempts); slice s0 spent 42s.
	sess := []byte(`{"id":"x","courseId":"ans","courseSchemaVersion":"2.0","studentId":"u","status":"completed",` +
		`"sliceStates":{"s0":{"status":"completed","elapsedSeconds":42,"blockStates":{` +
		`"q1":{"completed":true,"attempts":1,"answer":"a"},` +
		`"q2":{"completed":true,"attempts":2,"answer":"Lateral Reading"}` +
		`}}},"events":[]}`)
	if _, err := store.CreateCourseSession(ctx, userID, courseID, sess, "completed"); err != nil {
		t.Fatalf("CreateCourseSession: %v", err)
	}

	rep, ok, err := store.CourseAnswerReport20(ctx, userID, "ans", "")
	if err != nil || !ok {
		t.Fatalf("CourseAnswerReport20: ok=%v err=%v", ok, err)
	}
	if len(rep.Slices) != 1 {
		t.Fatalf("slices = %d, want 1 (s1 has no assessment blocks and must drop out)", len(rep.Slices))
	}
	s0 := rep.Slices[0]
	if s0.Title != "Slice Zero" || s0.TimeSpentSeconds != 42 {
		t.Fatalf("slice = %q / %ds, want Slice Zero / 42s", s0.Title, s0.TimeSpentSeconds)
	}
	if len(s0.Items) != 2 {
		t.Fatalf("items = %d, want 2 (text block omitted)", len(s0.Items))
	}
	q1 := s0.Items[0]
	if q1.Prompt != "选哪个来源？" || q1.YourAnswer != "看作者" || !q1.Answered || q1.Correct == nil || !*q1.Correct || q1.Attempts != 1 {
		t.Fatalf("q1 = %+v, want prompt/label=看作者/correct=true/attempts=1", q1)
	}
	q2 := s0.Items[1]
	if q2.YourAnswer != "Lateral Reading" || q2.Correct == nil || !*q2.Correct || q2.Attempts != 2 {
		t.Fatalf("q2 = %+v, want answer=Lateral Reading/correct=true (case-insensitive)/attempts=2", q2)
	}

	// Legacy course (no 2.0 definition) → found=false, empty report (never an error).
	if _, ok, err := store.CourseAnswerReport20(ctx, userID, "does-not-exist", ""); err == nil && ok {
		t.Fatalf("unknown slug: want ok=false")
	}
	// A malformed attempt id is a 404, never a fall-through to the latest attempt.
	if _, _, err := store.CourseAnswerReport20(ctx, userID, "ans", "not-a-uuid"); err == nil {
		t.Fatalf("CourseAnswerReport20 with a malformed attempt id: want error, got nil")
	}
}

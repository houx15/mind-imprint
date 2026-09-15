package api

// lite_grading_jobs_internal_test.go — DB-backed tests for runLiteGrading.
//
// gradeWithRetry's own tests (lite_grading_run_internal_test.go) are pure and
// prove the retry loop; they cannot prove what happens to the ROW, because
// that needs a real lite_grading row and ClaimLiteGrading/SetLiteGradingFailed/
// SetLiteGradingDraft's compare-and-swap. These tests cover three controller
// rulings that only show up at the row level: a failed REGRADE keeps the
// previous draft editable instead of losing it (ruling 1); the final write
// survives a cancelled context (ruling 2); and PersonJudging is actually
// wired through the worker, not just liteGradingInput (ruling 3).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/store/sqlc"
)

// liteGradingWorkerFixture is one class with a teacher, a student, and one
// submitted writing version — everything runLiteGrading needs a lite_grading
// row to point at.
type liteGradingWorkerFixture struct {
	pool      *pgxpool.Pool
	q         *sqlc.Queries
	teacherID uuid.UUID
	studentID uuid.UUID
	classID   uuid.UUID
	atomID    uuid.UUID
	versionID uuid.UUID
}

func newLiteGradingWorkerFixture(t *testing.T) liteGradingWorkerFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := NewTestDB(t)
	q := sqlc.New(pool)

	teacher, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Email: "lgw-teacher-" + uuid.NewString() + "@demo.local", PasswordHash: "x", Role: "teacher", SchoolID: SeedSchoolID,
		DisplayName: "T", AvatarColor: "#888888", EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create teacher: %v", err)
	}
	student, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Email: "lgw-student-" + uuid.NewString() + "@demo.local", PasswordHash: "x", Role: "student", SchoolID: SeedSchoolID,
		DisplayName: "S", AvatarColor: "#888888", EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create student: %v", err)
	}
	class, err := q.CreateClass(ctx, sqlc.CreateClassParams{
		SchoolID: SeedSchoolID, Name: "LGW Class", JoinCode: "LGW-" + uuid.NewString()[:8],
		CreatedBy: pgtype.UUID{Bytes: teacher.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	if _, err := q.CreateEnrollment(ctx, sqlc.CreateEnrollmentParams{UserID: student.ID, ClassID: class.ID, RoleInClass: "student"}); err != nil {
		t.Fatalf("enroll student: %v", err)
	}
	atom, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: student.ID})
	if err != nil {
		t.Fatalf("create atom: %v", err)
	}
	if _, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: atom.ID, Title: "雨水去哪儿了", Lang: "zh"}); err != nil {
		t.Fatalf("create writing: %v", err)
	}
	version, err := q.CreateWritingVersion(ctx, sqlc.CreateWritingVersionParams{
		AtomID: atom.ID, Number: 1, Title: "雨水去哪儿了", Body: runTestBody, WordCount: int32(len([]rune(runTestBody))),
	})
	if err != nil {
		t.Fatalf("create writing version: %v", err)
	}
	return liteGradingWorkerFixture{
		pool: pool, q: q, teacherID: teacher.ID, studentID: student.ID,
		classID: class.ID, atomID: atom.ID, versionID: version.ID,
	}
}

// createGrading inserts one queued lite_grading row against the fixture's
// version, with the zh default rubric.
func (f liteGradingWorkerFixture) createGrading(t *testing.T) sqlc.LiteGrading {
	t.Helper()
	rubric, err := json.Marshal(liteassign.DefaultRubric("zh"))
	if err != nil {
		t.Fatalf("marshal rubric: %v", err)
	}
	g, err := f.q.CreateLiteGrading(context.Background(), sqlc.CreateLiteGradingParams{
		AtomID: f.atomID, VersionID: f.versionID, UserID: f.studentID, ClassID: f.classID,
		Rubric: rubric, RequestedBy: f.teacherID,
	})
	if err != nil {
		t.Fatalf("create grading: %v", err)
	}
	return g
}

// newTestAPIForLiteGrading wires prov behind gateway.ClassReview — the only
// class runLiteGrading routes through.
func newTestAPIForLiteGrading(pool *pgxpool.Pool, prov gateway.Provider) *API {
	return New(Deps{
		Queries: sqlc.New(pool), Pool: pool, Provider: prov,
		EvalResolver: func(context.Context) (gateway.Resolved, error) {
			return gateway.Resolved{Provider: "stub", Model: "stub-review", Tier: "flagship"}, nil
		},
	})
}

func TestRunLiteGradingFirstGradingSuccessWritesDraft(t *testing.T) {
	f := newLiteGradingWorkerFixture(t)
	g := f.createGrading(t)

	prov := gateway.NewSequenceStubProvider(runTestScript(runTestValid))
	a := newTestAPIForLiteGrading(f.pool, prov)
	a.runLiteGrading(context.Background(), g.ID)

	got, err := f.q.GetLiteGrading(context.Background(), g.ID)
	if err != nil {
		t.Fatalf("get grading: %v", err)
	}
	if got.Status != "draft" || got.Error != nil {
		t.Fatalf("status=%s error=%v", got.Status, got.Error)
	}
	var content litegrade.Content
	if err := json.Unmarshal(got.Content, &content); err != nil {
		t.Fatalf("decode content: %v", err)
	}
	if content.Overall.Grade != "B+" || len(content.Points) != 3 {
		t.Fatalf("content = %+v", content)
	}
	if string(got.Ai) != string(got.Content) {
		t.Fatalf("ai and content must both be the fresh result on first grading")
	}
}

// Controller ruling 1: a regrade whose model reply fails Check twice must not
// lose the previous successful draft — the row returns to draft with its
// prior ai/content/reviewed_at kept, and the new failure reason shown.
func TestRunLiteGradingFailedRegradeKeepsPreviousDraft(t *testing.T) {
	f := newLiteGradingWorkerFixture(t)
	g := f.createGrading(t)

	firstProv := gateway.NewSequenceStubProvider(runTestScript(runTestValid))
	newTestAPIForLiteGrading(f.pool, firstProv).runLiteGrading(context.Background(), g.ID)

	before, err := f.q.GetLiteGrading(context.Background(), g.ID)
	if err != nil || before.Status != "draft" {
		t.Fatalf("setup: status=%s err=%v", before.Status, err)
	}
	// Simulate a teacher having already reviewed this draft, so the test can
	// also prove a failed regrade leaves reviewed_at alone.
	reviewedAt := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET reviewed_at = $1 WHERE id = $2`, reviewedAt, g.ID); err != nil {
		t.Fatalf("set reviewed_at: %v", err)
	}

	rubric, _ := json.Marshal(liteassign.DefaultRubric("zh"))
	if _, err := f.q.RequeueLiteGrading(context.Background(), sqlc.RequeueLiteGradingParams{
		ID: g.ID, Rubric: rubric, RequestedBy: f.teacherID,
	}); err != nil {
		t.Fatalf("requeue: %v", err)
	}

	// The regrade's reply is unparseable on both attempts.
	badProv := gateway.NewSequenceStubProvider(runTestScript("抱歉，我无法批改。"))
	newTestAPIForLiteGrading(f.pool, badProv).runLiteGrading(context.Background(), g.ID)

	after, err := f.q.GetLiteGrading(context.Background(), g.ID)
	if err != nil {
		t.Fatalf("get after: %v", err)
	}
	if after.Status != "draft" {
		t.Fatalf("status after failed regrade = %q, want draft (keep the previous content editable)", after.Status)
	}
	if after.Error == nil || *after.Error == "" {
		t.Fatal("error must be set after a failed regrade")
	}
	if string(after.Content) != string(before.Content) {
		t.Fatalf("content must survive a failed regrade:\nbefore=%s\nafter=%s", before.Content, after.Content)
	}
	if string(after.Ai) != string(before.Ai) {
		t.Fatalf("ai must survive a failed regrade:\nbefore=%s\nafter=%s", before.Ai, after.Ai)
	}
	if !after.ReviewedAt.Valid || !after.ReviewedAt.Time.Equal(reviewedAt) {
		t.Fatalf("reviewed_at must survive a failed regrade: want %v, got %v", reviewedAt, after.ReviewedAt)
	}
}

// cancelAfterStreamProvider forwards every event from inner's Stream, then
// calls cancel — only after the forwarding loop is done and right before
// closing the output channel. gateway.Collect's `for ev := range stream`
// cannot return until that channel closes, so cancel is guaranteed to have
// already run by the time the caller (gradeWithRetry, then runLiteGrading)
// gets control back — a deterministic stand-in for a job timeout landing
// exactly between "the model replied" and "the row is written".
type cancelAfterStreamProvider struct {
	inner  gateway.Provider
	cancel context.CancelFunc
}

func (p *cancelAfterStreamProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	in, err := p.inner.Stream(ctx, r, req)
	if err != nil {
		return nil, err
	}
	out := make(chan gateway.StreamEvent)
	go func() {
		defer close(out)
		for ev := range in {
			out <- ev
		}
		p.cancel()
	}()
	return out, nil
}

// Controller ruling 2: the worker's final write uses context.WithoutCancel,
// so a context cancelled right after the model call still lands the outcome
// instead of leaving the row stuck on running.
func TestRunLiteGradingFinalWriteSurvivesCancelledContext(t *testing.T) {
	f := newLiteGradingWorkerFixture(t)
	g := f.createGrading(t)

	ctx, cancel := context.WithCancel(context.Background())
	prov := &cancelAfterStreamProvider{inner: gateway.NewSequenceStubProvider(runTestScript(runTestValid)), cancel: cancel}
	a := newTestAPIForLiteGrading(f.pool, prov)

	a.runLiteGrading(ctx, g.ID)

	if ctx.Err() == nil {
		t.Fatal("test setup: ctx should already be cancelled by the time runLiteGrading returns")
	}
	got, err := f.q.GetLiteGrading(context.Background(), g.ID)
	if err != nil {
		t.Fatalf("get grading: %v", err)
	}
	if got.Status != "draft" || got.Content == nil {
		t.Fatalf("a cancelled ctx must not stop the final write: status=%s content=%s", got.Status, got.Content)
	}
	// A cancelled ctx must not stop metering either: this call cost money.
	var llmCalls int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE user_id = $1 AND atom_id = $2 AND purpose = $3`,
		f.studentID, f.atomID, liteGradingPurpose,
	).Scan(&llmCalls); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if llmCalls != 1 {
		t.Fatalf("llm_call rows for this grading = %d, want 1 (a cancelled ctx must not drop metering of a billed call)", llmCalls)
	}
}

// Controller ruling 3: the worker must always set Input.PersonJudging. This
// content is otherwise valid — right dimension names, in-scale grades, 3
// points with real quotes from her body — except one issue's text judges her
// instead of her writing. If PersonJudging were ever left nil, Check would
// not catch it and this row would end up draft, not failed.
const runTestPersonJudging = `{"overall":{"grade":"B+","comment":"用「去年秋天，我在那里摔过一跤。」引出问题。"},
"dimensions":[{"name":"内容","grade":"B+","comment":"问题来自亲身经历。"},{"name":"结构","grade":"B","comment":"两段之间没有过渡句。"},{"name":"语言","grade":"A-","comment":"表达清楚。"},{"name":"书写规范","grade":"A","comment":"标点使用正确。"}],
"points":[{"kind":"good","quote":"去年秋天，我在那里摔过一跤。","text":"用具体经历引出问题。","action":null},
{"kind":"issue","quote":"我读到城市里的雨水花园：用下凹的绿地先把雨水接住。","text":"你很懒，材料引用得不认真。","action":"在这句后面写一句说明雨水花园和后门空地的关系。"},
{"kind":"issue","quote":"学校后门那片空地一下雨就积水。","text":"积水的程度没有数据。","action":"补充一次积水的深度或持续时间。"}]}`

func TestRunLiteGradingRejectsPersonJudgingReplyThroughTheWorker(t *testing.T) {
	f := newLiteGradingWorkerFixture(t)
	g := f.createGrading(t)

	prov := gateway.NewSequenceStubProvider(runTestScript(runTestPersonJudging))
	a := newTestAPIForLiteGrading(f.pool, prov)
	a.runLiteGrading(context.Background(), g.ID)

	got, err := f.q.GetLiteGrading(context.Background(), g.ID)
	if err != nil {
		t.Fatalf("get grading: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("status = %q, want failed (a person-directed reply must never reach draft)", got.Status)
	}
	if got.Error == nil || !containsPersonJudgingReason(*got.Error) {
		t.Fatalf("error = %v, want it to name the person_judging reason", got.Error)
	}
}

func containsPersonJudgingReason(msg string) bool {
	want := (litegrade.Reason{Code: litegrade.ReasonPersonJudging, Where: "第 2 条意见的说明"}).Message()
	return strings.Contains(msg, want)
}

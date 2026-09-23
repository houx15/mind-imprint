package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/store/sqlc"
)

const gradingBody = "学校后门那片空地一下雨就积水。去年秋天，我在那里摔过一跤。\n\n我读到城市里的雨水花园：用下凹的绿地先把雨水接住。"

// gradingValidReply passes litegrade.Check against gradingBody and the zh default rubric.
const gradingValidReply = `{"overall":{"grade":"B+","comment":"用「去年秋天，我在那里摔过一跤。」引出问题。"},
"dimensions":[{"name":"内容","grade":"B+","comment":"问题来自亲身经历。"},{"name":"结构","grade":"B","comment":"两段之间没有过渡句。"},{"name":"语言","grade":"A-","comment":"表达清楚。"},{"name":"书写规范","grade":"A","comment":"标点使用正确。"}],
"points":[{"kind":"good","quote":"去年秋天，我在那里摔过一跤。","text":"用具体经历引出问题。","action":null},
{"kind":"issue","quote":"我读到城市里的雨水花园：用下凹的绿地先把雨水接住。","text":"材料与后门空地之间没有说明联系。","action":"在这句后面写一句说明雨水花园和后门空地的关系。"},
{"kind":"issue","quote":"学校后门那片空地一下雨就积水。","text":"积水的程度没有数据。","action":"补充一次积水的深度或持续时间。"}]}`

var gradingBadReply = strings.Replace(gradingValidReply, `"quote":"去年秋天，我在那里摔过一跤。"`, `"quote":"去年冬天，我在那里摔过一跤。"`, 1)

// fakeEnqueuer stands in for river: it records jobs, and the test runs them
// through the real worker.
type fakeEnqueuer struct {
	mu   sync.Mutex
	args []LiteGradingArgs
	fail error
	// failFirstN, when set, fails only the first N InsertTx calls (then
	// succeeds) — for testing a queue-all batch where some recipients'
	// enqueues fail and others don't. txCalls counts InsertTx calls only,
	// separate from Insert (interest harvest also shares this fake).
	failFirstN int
	txCalls    int
}

func (f *fakeEnqueuer) Insert(_ context.Context, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	if a, ok := args.(LiteGradingArgs); ok {
		f.args = append(f.args, a)
	}
	return &rivertype.JobInsertResult{Job: &rivertype.JobRow{}}, nil
}

// InsertTx: the fake does not itself persist anything transactionally (it
// has no river_job table to roll back), but the caller's real Postgres tx
// around it still rolls back the row change when this returns an error —
// that is the behavior TestLiteGradingRegradeEnqueueFailureRollsBack pins.
func (f *fakeEnqueuer) InsertTx(_ context.Context, _ pgx.Tx, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.txCalls++
	if f.fail != nil {
		return nil, f.fail
	}
	if f.failFirstN > 0 && f.txCalls <= f.failFirstN {
		return nil, errors.New("connection refused")
	}
	if a, ok := args.(LiteGradingArgs); ok {
		f.args = append(f.args, a)
	}
	return &rivertype.JobInsertResult{Job: &rivertype.JobRow{}}, nil
}

func (f *fakeEnqueuer) drain() []LiteGradingArgs {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.args
	f.args = nil
	return out
}

type gradingFixture struct {
	h         http.Handler
	a         *API
	pool      *pgxpool.Pool
	teacher   *http.Cookie
	classID   string
	studentID uuid.UUID
	enq       *fakeEnqueuer
	prov      *gateway.SequenceStubProvider
}

func gradingScript(text string) []gateway.StreamEvent {
	return []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 100, OutputTokens: 50}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
}

// newGradingFixture: liteTeacherFixture with a queue and a scripted model.
// Replies are played in order; the last one repeats.
func newGradingFixture(t *testing.T, replies ...string) *gradingFixture {
	t.Helper()
	if len(replies) == 0 {
		replies = []string{gradingValidReply}
	}
	scripts := make([][]gateway.StreamEvent, 0, len(replies))
	for _, r := range replies {
		scripts = append(scripts, gradingScript(r))
	}
	f := &gradingFixture{pool: newAPITestPool(t), enq: &fakeEnqueuer{}, prov: gateway.NewSequenceStubProvider(scripts...)}
	f.a = New(Deps{
		Queries: sqlc.New(f.pool), Pool: f.pool, Provider: f.prov,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID, River: f.enq,
	})
	f.h = f.a.Handler()
	if _, err := f.pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatal(err)
	}
	f.teacher = signInAs(t, f.pool, createTeacher(t, f.pool, SeedSchoolID, "gr-teacher@demo.local"))
	f.classID = createClassViaAPI(t, f.h, f.teacher, "Grading Class")
	f.studentID = createStudent(t, f.pool, SeedSchoolID, "gr-student@demo.local")
	enrollStudent(t, f.pool, f.studentID, f.classID)
	return f
}

// submit: the fixture student starts a writing homework, writes gradingBody and finishes (v1).
func (f *gradingFixture) submit(t *testing.T) (aid, atomID string, student *http.Cookie) {
	t.Helper()
	aid, atomID, student = startWritingHomework(t, f.h, f.pool, f.teacher, f.classID, f.studentID)
	if code := assignJSON(t, f.h, student, "PUT", "/api/v1/writings/"+atomID+"/draft", map[string]any{"body": gradingBody}, nil); code != http.StatusOK {
		t.Fatalf("draft = %d", code)
	}
	if code := assignJSON(t, f.h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	return aid, atomID, student
}

func (f *gradingFixture) runJobs(t *testing.T) {
	t.Helper()
	w := &LiteGradingWorker{API: f.a}
	for _, args := range f.enq.drain() {
		if err := w.Work(context.Background(), &river.Job[LiteGradingArgs]{JobRow: &rivertype.JobRow{}, Args: args}); err != nil {
			t.Fatalf("work: %v", err)
		}
	}
}

type gradingRowView struct {
	UserID  string  `json:"userId"`
	AtomID  *string `json:"atomId"`
	Version *struct {
		Number int `json:"number"`
	} `json:"version"`
	Grading *struct {
		ID           string  `json:"id"`
		Status       string  `json:"status"`
		OverallGrade *string `json:"overallGrade"`
		Error        *string `json:"error"`
		ReviewedAt   *string `json:"reviewedAt"`
	} `json:"grading"`
}

type teacherGradingView struct {
	ID                  string          `json:"id"`
	AssignmentID        *string         `json:"assignmentId"`
	VersionNumber       int             `json:"versionNumber"`
	LatestVersionNumber int             `json:"latestVersionNumber"`
	Body                string          `json:"body"`
	Lang                string          `json:"lang"`
	Rubric              json.RawMessage `json:"rubric"`
	Status              string          `json:"status"`
	Content             json.RawMessage `json:"content"`
	Error               *string         `json:"error"`
	ReviewedAt          *string         `json:"reviewedAt"`
	SentAt              *string         `json:"sentAt"`
	StudentSeenAt       *string         `json:"studentSeenAt"`
}

func (f *gradingFixture) rows(t *testing.T, aid string) []gradingRowView {
	t.Helper()
	var resp struct {
		Rows []gradingRowView `json:"rows"`
	}
	if code := getJSON(t, f.h, f.teacher, "/api/v1/lite/teacher/assignments/"+aid+"/gradings", &resp); code != http.StatusOK {
		t.Fatalf("list gradings = %d", code)
	}
	return resp.Rows
}

func (f *gradingFixture) grading(t *testing.T, gid string) teacherGradingView {
	t.Helper()
	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := getJSON(t, f.h, f.teacher, "/api/v1/lite/teacher/gradings/"+gid, &resp); code != http.StatusOK {
		t.Fatalf("get grading = %d", code)
	}
	return resp.Grading
}

// queueAllResp is the queue-all route's full response: queued/failed counts
// plus the first backend error text when failed > 0 (never silently lost —
// AI/backend errors must surface, per AGENTS.md).
type queueAllResp struct {
	Queued int     `json:"queued"`
	Failed int     `json:"failed"`
	Error  *string `json:"error"`
}

func (f *gradingFixture) queueAllFull(t *testing.T, aid string, retryFailed bool) queueAllResp {
	t.Helper()
	var resp queueAllResp
	if code := assignJSON(t, f.h, f.teacher, "POST", "/api/v1/lite/teacher/assignments/"+aid+"/gradings", map[string]any{"retryFailed": retryFailed}, &resp); code != http.StatusOK {
		t.Fatalf("queue = %d", code)
	}
	return resp
}

func (f *gradingFixture) queueAll(t *testing.T, aid string, retryFailed bool) int {
	t.Helper()
	return f.queueAllFull(t, aid, retryFailed).Queued
}

func countGradingCalls(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM llm_call WHERE purpose = 'teacher_grading'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLiteGradingQueueRunsOnlyEligible(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	aid, _, _ := f.submit(t)
	// A second recipient who never started: no version, nothing to queue.
	other := createStudent(t, f.pool, SeedSchoolID, "gr-other-student@demo.local")
	enrollStudent(t, f.pool, other, f.classID)
	if code := assignJSON(t, f.h, f.teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid, map[string]any{"addUserIds": []string{other.String()}}, nil); code != http.StatusOK {
		t.Fatalf("add recipient = %d", code)
	}

	rows := f.rows(t, aid)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if n := f.queueAll(t, aid, false); n != 1 {
		t.Fatalf("queued = %d, want 1", n)
	}
	if n := f.queueAll(t, aid, false); n != 0 {
		t.Fatalf("second queue = %d, want 0 (the version already has a row)", n)
	}
	var gid string
	for _, r := range f.rows(t, aid) {
		if r.UserID == f.studentID.String() {
			if r.Grading == nil || r.Grading.Status != "queued" || r.Version == nil || r.Version.Number != 1 {
				t.Fatalf("queued row = %+v", r)
			}
			gid = r.Grading.ID
		} else if r.Version != nil || r.Grading != nil {
			t.Fatalf("unstarted row = %+v", r)
		}
	}

	f.runJobs(t)
	g := f.grading(t, gid)
	if g.Status != "draft" || g.Error != nil || g.Body != gradingBody || g.VersionNumber != 1 || g.LatestVersionNumber != 1 || g.AssignmentID == nil || *g.AssignmentID != aid {
		t.Fatalf("draft = %+v", g)
	}
	var content struct {
		Overall struct{ Grade string }    `json:"overall"`
		Points  []struct{ Source string } `json:"points"`
	}
	if err := json.Unmarshal(g.Content, &content); err != nil || content.Overall.Grade != "B+" || len(content.Points) != 3 || content.Points[0].Source != "ai" {
		t.Fatalf("content = %s err=%v", g.Content, err)
	}
	var ai, stored []byte
	if err := f.pool.QueryRow(context.Background(), `SELECT ai, content FROM lite_grading WHERE id = $1`, gid).Scan(&ai, &stored); err != nil || string(ai) != string(stored) {
		t.Fatalf("ai must equal content after a run: ai=%s content=%s err=%v", ai, stored, err)
	}
	if f.prov.Calls != 1 || countGradingCalls(t, f.pool) != 1 {
		t.Fatalf("provider calls = %d, llm_call rows = %d", f.prov.Calls, countGradingCalls(t, f.pool))
	}
	for _, r := range f.rows(t, aid) {
		if r.Grading != nil && (r.Grading.OverallGrade == nil || *r.Grading.OverallGrade != "B+") {
			t.Fatalf("list overall grade = %+v", r.Grading)
		}
	}
}

func TestLiteGradingRetriesOnceThenFails(t *testing.T) {
	f := newGradingFixture(t, gradingBadReply)
	aid, _, _ := f.submit(t)
	f.queueAll(t, aid, false)
	f.runJobs(t)
	gid := f.rows(t, aid)[0].Grading.ID
	g := f.grading(t, gid)
	if g.Status != "failed" || g.Error == nil || !strings.Contains(*g.Error, "的引文不在正文中：「去年冬天，我在那里摔过一跤。」") || g.Content != nil && string(g.Content) != "null" {
		t.Fatalf("failed = %+v", g)
	}
	if f.prov.Calls != 2 || countGradingCalls(t, f.pool) != 2 {
		t.Fatalf("provider calls = %d, llm_call rows = %d, want 2 and 2", f.prov.Calls, countGradingCalls(t, f.pool))
	}
	if n := f.queueAll(t, aid, false); n != 0 {
		t.Fatalf("queue without retryFailed = %d", n)
	}
	if n := f.queueAll(t, aid, true); n != 1 {
		t.Fatalf("retryFailed = %d", n)
	}
	if g := f.grading(t, gid); g.Status != "queued" || g.Error != nil {
		t.Fatalf("requeued = %+v", g)
	}
}

func TestLiteGradingSingleWritingAndRegrade(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	_, atomID, student := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"

	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK || resp.Grading.Status != "queued" {
		t.Fatalf("single = %d %+v", code, resp.Grading)
	}
	gid := resp.Grading.ID
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", single, nil); code != http.StatusConflict || ec != "grading_in_progress" {
		t.Fatalf("single while queued = %d %s", code, ec)
	}
	f.runJobs(t)

	// A reviewed draft is regraded: new ai and content, reviewed_at cleared.
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET reviewed_at = now(), content = '{"overall":{"grade":"C","comment":"x"},"dimensions":[],"points":[]}' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	regrade := "/api/v1/lite/teacher/gradings/" + gid + "/regrade"
	if code := assignJSON(t, f.h, f.teacher, "POST", regrade, nil, &resp); code != http.StatusOK || resp.Grading.Status != "queued" {
		t.Fatalf("regrade draft = %d %+v", code, resp.Grading)
	}
	f.runJobs(t)
	if g := f.grading(t, gid); g.Status != "draft" || g.ReviewedAt != nil || !strings.Contains(string(g.Content), `"B+"`) {
		t.Fatalf("after regrade = %+v %s", g, g.Content)
	}

	// A sent version is never regraded.
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'sent', sent_at = now() WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", regrade, nil); code != http.StatusConflict || ec != "grading_sent" {
		t.Fatalf("regrade sent = %d %s", code, ec)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", single, nil); code != http.StatusConflict || ec != "grading_sent" {
		t.Fatalf("single on sent version = %d %s", code, ec)
	}

	// Her v2 is a new version with no row: it can be graded.
	w := "/api/v1/writings/" + atomID
	if code := assignJSON(t, f.h, student, "POST", w+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("revise = %d", code)
	}
	if code := assignJSON(t, f.h, student, "PUT", w+"/draft", map[string]any{"body": gradingBody + "\n\n我打算问问学校谁负责清理。"}, nil); code != http.StatusOK {
		t.Fatalf("draft v2 = %d", code)
	}
	if code := assignJSON(t, f.h, student, "POST", w+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish v2 = %d", code)
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK || resp.Grading.ID == gid || resp.Grading.VersionNumber != 2 {
		t.Fatalf("single on v2 = %d %+v", code, resp.Grading)
	}
	if g := f.grading(t, gid); g.LatestVersionNumber != 2 || g.VersionNumber != 1 {
		t.Fatalf("old row versions = %+v", g)
	}
}

func TestLiteGradingStaleRunningIsFailed(t *testing.T) {
	f := newGradingFixture(t)
	aid, _, _ := f.submit(t)
	f.queueAll(t, aid, false)
	gid := f.rows(t, aid)[0].Grading.ID
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'running', updated_at = now() - interval '20 minutes' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if g := f.grading(t, gid); g.Status != "failed" || g.Error == nil || *g.Error != "批改超时：任务未完成" {
		t.Fatalf("stale running = %+v", g)
	}
}

func TestLiteGradingQueueUnavailableAndEnqueueFailure(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t) // River is nil here
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if code, ec := writeErrorCode(t, h, teacher, "POST", "/api/v1/lite/teacher/assignments/"+aid+"/gradings", nil); code != http.StatusServiceUnavailable || ec != "grading_queue_unavailable" {
		t.Fatalf("no queue = %d %s", code, ec)
	}

	// Every recipient's enqueue fails (queue down mid-request): queue-all
	// answers 503 grading_enqueue_failed instead of a fake 200 {"queued":0}
	// that would read as "nothing to grade" (Task 5 review round 2, ruling
	// b: m > 0 and n == 0).
	f := newGradingFixture(t)
	f.enq.fail = errors.New("connection refused")
	aid2, _, _ := f.submit(t)
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", "/api/v1/lite/teacher/assignments/"+aid2+"/gradings", map[string]any{"retryFailed": false}); code != http.StatusServiceUnavailable || ec != "grading_enqueue_failed" {
		t.Fatalf("all recipients fail = %d %s, want 503 grading_enqueue_failed", code, ec)
	}
	// The row create and the job insert are one Postgres transaction (Task 5
	// review round 1 ruling 3): a failed insert rolls the row create back
	// too, so nothing is left behind — no "failed" row a teacher could
	// puzzle over. The next 一键AI批改 retries her from scratch.
	r := f.rows(t, aid2)[0]
	if r.Grading != nil {
		t.Fatalf("row after a rolled-back enqueue = %+v, want none", r.Grading)
	}
}

// TestLiteGradingQueueAllPartialFailureReportsCounts: when only some
// recipients' enqueues fail, queue-all still answers 200 (the ones that
// succeeded really did queue), but reports how many failed and why — never
// silently drops the failure the way a bare {"queued": n} would (Task 5
// review round 2, ruling a).
func TestLiteGradingQueueAllPartialFailureReportsCounts(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	aid, _, _ := f.submit(t)
	other := createStudent(t, f.pool, SeedSchoolID, "gr-other-student@demo.local")
	enrollStudent(t, f.pool, other, f.classID)
	if code := assignJSON(t, f.h, f.teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid, map[string]any{"addUserIds": []string{other.String()}}, nil); code != http.StatusOK {
		t.Fatalf("add recipient = %d", code)
	}
	otherStudent := signInAs(t, f.pool, other)
	atomID2 := startAssignment(t, f.h, otherStudent, aid).AtomID
	if code := assignJSON(t, f.h, otherStudent, "PUT", "/api/v1/writings/"+atomID2+"/draft", map[string]any{"body": gradingBody}, nil); code != http.StatusOK {
		t.Fatalf("draft2 = %d", code)
	}
	if code := assignJSON(t, f.h, otherStudent, "POST", "/api/v1/writings/"+atomID2+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish2 = %d", code)
	}

	// Both recipients are now eligible; the first enqueue attempted fails,
	// the second succeeds.
	f.enq.failFirstN = 1
	resp := f.queueAllFull(t, aid, false)
	if resp.Queued != 1 || resp.Failed != 1 || resp.Error == nil || !strings.Contains(*resp.Error, "入队失败：connection refused") {
		t.Fatalf("partial failure response = %+v", resp)
	}
}

// TestLiteGradingStaleRunningWithContentKeepsDraft: a regrade that gets stuck
// (status running, but the row still holds the previous successful content)
// times out to draft, not failed — the teacher's earlier draft is not lost.
func TestLiteGradingStaleRunningWithContentKeepsDraft(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	_, atomID, _ := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"
	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK {
		t.Fatalf("single = %d", code)
	}
	gid := resp.Grading.ID
	f.runJobs(t)
	before := f.grading(t, gid)
	if before.Status != "draft" || before.Content == nil {
		t.Fatalf("before regrade = %+v", before)
	}

	regrade := "/api/v1/lite/teacher/gradings/" + gid + "/regrade"
	if code := assignJSON(t, f.h, f.teacher, "POST", regrade, nil, &resp); code != http.StatusOK || resp.Grading.Status != "queued" {
		t.Fatalf("regrade = %d %+v", code, resp.Grading)
	}
	// Simulate a worker that claimed the row and then crashed mid-call: still
	// running, content untouched from the last successful grading.
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'running', updated_at = now() - interval '20 minutes' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	g := f.grading(t, gid)
	if g.Status != "draft" || g.Error == nil || *g.Error != "批改超时：任务未完成" || !strings.Contains(string(g.Content), `"B+"`) {
		t.Fatalf("stale running with content = %+v %s", g, g.Content)
	}
}

// TestLiteGradingRegradeEnqueueFailureRollsBack: a regrade whose job insert
// fails (queue down mid-request) rolls the whole transaction back — the row
// change (RequeueLiteGrading) never happened, so the row is exactly the
// draft it was before, and the route answers an error instead of a 200 with
// a fabricated row (Task 5 review ruling 3: the row requeue and the job
// insert are one Postgres transaction, so a committed queued row always has
// its job — there is no longer a half-succeeded "queued but no job" state).
func TestLiteGradingRegradeEnqueueFailureRollsBack(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	_, atomID, _ := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"
	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK {
		t.Fatalf("single = %d", code)
	}
	gid := resp.Grading.ID
	f.runJobs(t)
	before := f.grading(t, gid)
	if before.Status != "draft" || before.Content == nil {
		t.Fatalf("before regrade = %+v", before)
	}

	f.enq.fail = errors.New("connection refused")
	regrade := "/api/v1/lite/teacher/gradings/" + gid + "/regrade"
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", regrade, nil); code != http.StatusServiceUnavailable || ec != "grading_enqueue_failed" {
		t.Fatalf("regrade with a failing queue = %d %s, want 503 grading_enqueue_failed", code, ec)
	}

	after := f.grading(t, gid)
	if after.Status != "draft" || after.Error != nil || string(after.Content) != string(before.Content) {
		t.Fatalf("row after a rolled-back regrade = %+v, want unchanged from %+v", after, before)
	}
}

// TestLiteGradingSingleWritingRefusesExistingDraft: the single-writing POST
// must not silently overwrite an existing draft — only the explicit regrade
// route, which the UI puts behind a confirm, may do that.
func TestLiteGradingSingleWritingRefusesExistingDraft(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	_, atomID, _ := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, nil); code != http.StatusOK {
		t.Fatalf("single = %d", code)
	}
	f.runJobs(t)
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", single, nil); code != http.StatusConflict || ec != "grading_exists" {
		t.Fatalf("single on an existing draft = %d %s, want 409 grading_exists", code, ec)
	}
}

// TestLiteGradingRegradeSweepsStaleRunningBeforeRefusing: a row stuck
// `running` for 20+ minutes must not read as 「批改中」 forever just because
// nobody happened to GET it first — regrade itself sweeps it before checking
// status (Task 5 review promoted minor / ruling 4).
func TestLiteGradingRegradeSweepsStaleRunningBeforeRefusing(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	_, atomID, _ := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"
	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK {
		t.Fatalf("single = %d", code)
	}
	gid := resp.Grading.ID
	f.runJobs(t)
	// A regrade that got stuck 20 minutes ago: still `running`, content kept
	// from the last successful grading — nobody has GET the row since.
	regrade := "/api/v1/lite/teacher/gradings/" + gid + "/regrade"
	if code := assignJSON(t, f.h, f.teacher, "POST", regrade, nil, &resp); code != http.StatusOK || resp.Grading.Status != "queued" {
		t.Fatalf("first regrade = %d %+v", code, resp.Grading)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'running', updated_at = now() - interval '20 minutes' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	// Regrade again, straight away — no GET in between to sweep it first.
	if code := assignJSON(t, f.h, f.teacher, "POST", regrade, nil, &resp); code != http.StatusOK || resp.Grading.Status != "queued" {
		t.Fatalf("regrade on a stale running row = %d %+v, want it to proceed (swept first)", code, resp.Grading)
	}
}

// TestLiteGradingSingleWritingSweepsStaleRunningBeforeRefusing: the same
// sweep-before-check as regrade, but through the single-writing POST route,
// and for a FIRST grading that got stuck (no content yet, not a regrade) —
// it must proceed, not read as 「批改中」 forever (Task 5 review round 2,
// small gap).
func TestLiteGradingSingleWritingSweepsStaleRunningBeforeRefusing(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	_, atomID, _ := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"
	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK {
		t.Fatalf("single = %d", code)
	}
	gid := resp.Grading.ID
	// A first grading that got stuck 20 minutes ago, before any model reply
	// ever landed — no content, not a regrade. Nobody has GET the row since.
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'running', updated_at = now() - interval '20 minutes' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK || resp.Grading.Status != "queued" {
		t.Fatalf("single on a stale running row (no content) = %d %+v, want it to proceed (swept first)", code, resp.Grading)
	}
}

// TestLiteGradingQueueUsesHomeworkRubric: a homework with a custom rubric
// (points scale, not the zh default) is queued through 一键AI批改, and the
// stored lite_grading.rubric is that homework's EffectiveRubric, not the
// default (Task 5 review ruling 1a).
func TestLiteGradingQueueUsesHomeworkRubric(t *testing.T) {
	f := newGradingFixture(t)
	aid, _, _ := f.submit(t)
	points := map[string]any{
		"scale": "points", "max": 20,
		"dimensions": []map[string]any{{"name": "论证", "note": "重点看论证是否清楚"}},
		"focus":      "重点看论证",
	}
	if code := assignJSON(t, f.h, f.teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid, map[string]any{"rubric": points}, nil); code != http.StatusOK {
		t.Fatalf("patch rubric = %d", code)
	}
	if n := f.queueAll(t, aid, false); n != 1 {
		t.Fatalf("queued = %d, want 1", n)
	}
	gid := f.rows(t, aid)[0].Grading.ID
	var stored []byte
	if err := f.pool.QueryRow(context.Background(), `SELECT rubric FROM lite_grading WHERE id = $1`, gid).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var got liteassign.Rubric
	if err := json.Unmarshal(stored, &got); err != nil {
		t.Fatal(err)
	}
	want := liteassign.Rubric{
		Scale: "points", Max: 20,
		Dimensions: []liteassign.RubricDimension{{Name: "论证", Note: "重点看论证是否清楚"}},
		Focus:      "重点看论证",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored rubric = %+v, want %+v", got, want)
	}
}

// TestLiteGradingSingleWritingUsesDefaultRubricForNonHomework: a writing she
// brought in herself (not assigned) graded through the single-writing route
// stores DefaultRubric(lang), and its assignmentId is null (Task 5 review
// ruling 1b).
func TestLiteGradingSingleWritingUsesDefaultRubricForNonHomework(t *testing.T) {
	f := newGradingFixture(t)
	student := signInAs(t, f.pool, f.studentID)
	atomID := newBroughtWriting(t, f.h, student, "我读了 NASA 的报告，数据是 2024 年的。")
	if code := assignJSON(t, f.h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"
	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK {
		t.Fatalf("single = %d", code)
	}
	if resp.Grading.AssignmentID != nil {
		t.Fatalf("assignmentId = %v, want nil for a non-homework writing", *resp.Grading.AssignmentID)
	}
	var got liteassign.Rubric
	if err := json.Unmarshal(resp.Grading.Rubric, &got); err != nil {
		t.Fatal(err)
	}
	if want := liteassign.DefaultRubric("zh"); !reflect.DeepEqual(got, want) {
		t.Fatalf("rubric = %+v, want the zh default %+v", got, want)
	}
}

// TestLiteGradingListSweepsStaleRunning: the list route (used by the
// assignment's grading table) must sweep a stale `running` row too, not only
// the single-row GET — otherwise a crashed job shows 批改中 forever there
// (Task 5 review ruling 2/5).
func TestLiteGradingListSweepsStaleRunning(t *testing.T) {
	f := newGradingFixture(t)
	aid, _, _ := f.submit(t)
	f.queueAll(t, aid, false)
	gid := f.rows(t, aid)[0].Grading.ID
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'running', updated_at = now() - interval '20 minutes' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	rows := f.rows(t, aid)
	var found bool
	for _, r := range rows {
		if r.Grading != nil && r.Grading.ID == gid {
			found = true
			if r.Grading.Status != "failed" || r.Grading.Error == nil {
				t.Fatalf("swept row via list = %+v, want failed with the timeout error", r.Grading)
			}
		}
	}
	if !found {
		t.Fatalf("gid missing from the list: %+v", rows)
	}
}

// TestLiteGradingExcludesStudentWhoLeftTheClass: a recipient row survives
// unenrollment (lite_assignment_recipient is not cleaned up), so both the
// list and 一键AI批改 must filter her out by current enrollment — she is no
// longer this teacher's to grade.
func TestLiteGradingExcludesStudentWhoLeftTheClass(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	aid, _, _ := f.submit(t)
	if len(f.rows(t, aid)) != 1 {
		t.Fatalf("rows before leaving = %+v", f.rows(t, aid))
	}

	if _, err := f.pool.Exec(context.Background(), `DELETE FROM enrollments WHERE user_id = $1 AND class_id = $2`, f.studentID, f.classID); err != nil {
		t.Fatal(err)
	}

	if rows := f.rows(t, aid); len(rows) != 0 {
		t.Fatalf("rows after leaving = %+v, want none listed", rows)
	}
	if n := f.queueAll(t, aid, false); n != 0 {
		t.Fatalf("queued after leaving = %d, want 0", n)
	}
}

func TestLiteGradingTeacherRoutesAreOwned(t *testing.T) {
	f := newGradingFixture(t)
	aid, atomID, student := f.submit(t)
	f.queueAll(t, aid, false)
	gid := f.rows(t, aid)[0].Grading.ID

	other := signInAs(t, f.pool, createTeacher(t, f.pool, SeedSchoolID, "gr-other-teacher@demo.local"))
	createClassViaAPI(t, f.h, other, "Other Class")
	paths := []struct{ method, path string }{
		{"GET", "/api/v1/lite/teacher/assignments/" + aid + "/gradings"},
		{"POST", "/api/v1/lite/teacher/assignments/" + aid + "/gradings"},
		{"POST", "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"},
		{"GET", "/api/v1/lite/teacher/gradings/" + gid},
		{"POST", "/api/v1/lite/teacher/gradings/" + gid + "/regrade"},
		{"GET", "/api/v1/lite/teacher/gradings/not-a-uuid"},
	}
	for _, p := range paths {
		if code, _ := writeErrorCode(t, f.h, other, p.method, p.path, nil); code != http.StatusNotFound {
			t.Errorf("other teacher %s %s = %d, want 404", p.method, p.path, code)
		}
		if code, _ := writeErrorCode(t, f.h, student, p.method, p.path, nil); code != http.StatusForbidden && code != http.StatusNotFound {
			t.Errorf("student %s %s = %d, want 403 or 404", p.method, p.path, code)
		}
	}

	// A student who left the class: her grading is no longer this teacher's.
	if _, err := f.pool.Exec(context.Background(), `DELETE FROM enrollments WHERE user_id = $1`, f.studentID); err != nil {
		t.Fatal(err)
	}
	if code, _ := writeErrorCode(t, f.h, f.teacher, "GET", "/api/v1/lite/teacher/gradings/"+gid, nil); code != http.StatusNotFound {
		t.Fatalf("left the class = %d, want 404", code)
	}
}

// gradingPointsReply passes litegrade.Check against gradingBody and
// gradingPointsRubric.
const gradingPointsReply = `{"overall":{"grade":"15","comment":"用「去年秋天，我在那里摔过一跤。」引出问题。"},
"dimensions":[{"name":"论证","grade":"14","comment":"问题来自亲身经历。"}],
"points":[{"kind":"good","quote":"去年秋天，我在那里摔过一跤。","text":"用具体经历引出问题。","action":null},
{"kind":"issue","quote":"我读到城市里的雨水花园：用下凹的绿地先把雨水接住。","text":"材料与后门空地之间没有说明联系。","action":"在这句后面写一句说明雨水花园和后门空地的关系。"},
{"kind":"issue","quote":"学校后门那片空地一下雨就积水。","text":"积水的程度没有数据。","action":"补充一次积水的深度或持续时间。"}]}`

var gradingPointsRubric = liteassign.Rubric{
	Scale: "points", Max: 20,
	Dimensions: []liteassign.RubricDimension{{Name: "论证", Note: "重点看论证是否清楚"}},
	Focus:      "重点看论证",
}

// TestLiteGradingRegradeAfterRubricChange (FB-1): the homework's rubric is
// changed after a draft exists. A regrade that fails keeps the old content
// and the rubric that describes it, so the teacher can still save that
// draft. A regrade that succeeds stores the new rubric with the new content.
func TestLiteGradingRegradeAfterRubricChange(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply, gradingBadReply, gradingBadReply, gradingPointsReply)
	aid, _, _ := f.submit(t)
	f.queueAll(t, aid, false)
	f.runJobs(t)
	gid := f.rows(t, aid)[0].Grading.ID
	path := "/api/v1/lite/teacher/gradings/" + gid

	if code := assignJSON(t, f.h, f.teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid, map[string]any{"rubric": gradingPointsRubric}, nil); code != http.StatusOK {
		t.Fatalf("patch rubric = %d", code)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", path+"/regrade", nil); code != http.StatusOK {
		t.Fatalf("regrade = %d %s", code, ec)
	}
	f.runJobs(t)

	g := f.grading(t, gid)
	if g.Status != "draft" || g.Error == nil {
		t.Fatalf("after failed regrade = %+v, want draft with an error", g)
	}
	var kept liteassign.Rubric
	if err := json.Unmarshal(g.Rubric, &kept); err != nil || !reflect.DeepEqual(kept, liteassign.DefaultRubric("zh")) {
		t.Fatalf("rubric after failed regrade = %s err=%v, want the zh default the content was graded with", g.Rubric, err)
	}
	teacherPoint := []map[string]any{{"kind": "issue", "quote": nil, "text": "第二段请补充数据来源。", "action": nil, "source": "teacher"}}
	if rec := doJSON(t, f.h, f.teacher, "PATCH", path, mustJSON(t, map[string]any{"content": gradingContent("B", teacherPoint)})); rec.Code != http.StatusOK {
		t.Fatalf("save the kept draft = %d %s", rec.Code, rec.Body)
	}

	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", path+"/regrade", nil); code != http.StatusOK {
		t.Fatalf("second regrade = %d %s", code, ec)
	}
	f.runJobs(t)
	g = f.grading(t, gid)
	if g.Status != "draft" || g.Error != nil || !strings.Contains(string(g.Content), `"15"`) {
		t.Fatalf("after successful regrade = %+v %s", g, g.Content)
	}
	var stored liteassign.Rubric
	if err := json.Unmarshal(g.Rubric, &stored); err != nil || !reflect.DeepEqual(stored, gradingPointsRubric) {
		t.Fatalf("rubric after successful regrade = %s err=%v, want the homework's new rubric", g.Rubric, err)
	}
}

// TestLiteGradingVisibleAcrossTheStudentsClasses (FB-2): the student is in
// two classes with different teachers. A grading of a writing she brought in
// herself is visible to the teacher of either class; a homework grading only
// to the teacher of the homework's class.
func TestLiteGradingVisibleAcrossTheStudentsClasses(t *testing.T) {
	f := newGradingFixture(t)
	aid, hwAtom, student := f.submit(t)
	f.queueAll(t, aid, false)
	hwGid := f.rows(t, aid)[0].Grading.ID

	own := newBroughtWriting(t, f.h, student, gradingBody)
	if code := assignJSON(t, f.h, student, "POST", "/api/v1/writings/"+own+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish own writing = %d", code)
	}
	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	ownSingle := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + own + "/gradings"
	if code := assignJSON(t, f.h, f.teacher, "POST", ownSingle, nil, &resp); code != http.StatusOK {
		t.Fatalf("grade own writing = %d", code)
	}
	ownGid := resp.Grading.ID

	second := signInAs(t, f.pool, createTeacher(t, f.pool, SeedSchoolID, "gr-second-teacher@demo.local"))
	secondClass := createClassViaAPI(t, f.h, second, "Second Class")
	enrollStudent(t, f.pool, f.studentID, secondClass)
	itemBase := "/api/v1/lite/teacher/classes/" + secondClass + "/students/" + f.studentID.String() + "/items/"
	itemGrading := func(atomID string) *gradingSummaryView {
		t.Helper()
		var item struct {
			Writing struct {
				Grading *gradingSummaryView `json:"grading"`
			} `json:"writing"`
		}
		if code := getJSON(t, f.h, second, itemBase+atomID, &item); code != http.StatusOK {
			t.Fatalf("item %s = %d", atomID, code)
		}
		return item.Writing.Grading
	}

	// Her own writing: the second teacher opens it and sees it on the item page.
	if code, ec := writeErrorCode(t, f.h, second, "GET", "/api/v1/lite/teacher/gradings/"+ownGid, nil); code != http.StatusOK {
		t.Fatalf("second teacher GET own-writing grading = %d %s, want 200", code, ec)
	}
	if g := itemGrading(own); g == nil || g.ID != ownGid || g.Status != "queued" {
		t.Fatalf("item page grading for own writing = %+v, want %s queued", g, ownGid)
	}
	if code, ec := writeErrorCode(t, f.h, second, "POST", itemBase+own+"/gradings", nil); code != http.StatusConflict || ec != "grading_in_progress" {
		t.Fatalf("second teacher POST on own writing = %d %s, want 409 grading_in_progress", code, ec)
	}

	// The homework of the first class: 404, hidden, and not requeued.
	if code, _ := writeErrorCode(t, f.h, second, "GET", "/api/v1/lite/teacher/gradings/"+hwGid, nil); code != http.StatusNotFound {
		t.Fatalf("second teacher GET homework grading = %d, want 404", code)
	}
	if g := itemGrading(hwAtom); g != nil {
		t.Fatalf("item page grading for another class's homework = %+v, want hidden", g)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'failed', error = 'x' WHERE id = $1`, hwGid); err != nil {
		t.Fatal(err)
	}
	if code, _ := writeErrorCode(t, f.h, second, "POST", itemBase+hwAtom+"/gradings", nil); code != http.StatusNotFound {
		t.Fatalf("second teacher POST on homework = %d, want 404", code)
	}
	var status string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM lite_grading WHERE id = $1`, hwGid).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("homework row status = %q err=%v, want still failed", status, err)
	}
	// The first teacher still has both.
	for _, gid := range []string{hwGid, ownGid} {
		if code, _ := writeErrorCode(t, f.h, f.teacher, "GET", "/api/v1/lite/teacher/gradings/"+gid, nil); code != http.StatusOK {
			t.Fatalf("first teacher GET %s = %d", gid, code)
		}
	}
}

type gradingSummaryView struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func TestLiteGradingRejectsReadingHomework(t *testing.T) {
	f := newGradingFixture(t)
	reading := createAssignment(t, f.h, f.teacher, f.classID, readingAssignmentBody("读", map[string]any{"source": "text", "text": "一段正文。"}, []string{f.studentID.String()}))
	for _, method := range []string{"GET", "POST"} {
		if code, ec := writeErrorCode(t, f.h, f.teacher, method, "/api/v1/lite/teacher/assignments/"+reading+"/gradings", nil); code != http.StatusBadRequest || ec != "not_writing_assignment" {
			t.Fatalf("%s reading homework = %d %s", method, code, ec)
		}
	}
}

func gradingContent(grade string, points []map[string]any) map[string]any {
	return map[string]any{
		"overall": map[string]any{"grade": grade, "comment": "第二段请补充数据来源。"},
		"dimensions": []map[string]any{
			{"name": "内容", "grade": "B", "comment": ""}, {"name": "结构", "grade": "B", "comment": ""},
			{"name": "语言", "grade": "B", "comment": ""}, {"name": "书写规范", "grade": "B", "comment": ""},
		},
		"points": points,
	}
}

func TestLiteGradingPatchAndSend(t *testing.T) {
	f := newGradingFixture(t)
	aid, _, _ := f.submit(t)
	f.queueAll(t, aid, false)
	gid := f.rows(t, aid)[0].Grading.ID
	path := "/api/v1/lite/teacher/gradings/" + gid

	// Queued: not editable, not sendable.
	if code, ec := writeErrorCode(t, f.h, f.teacher, "PATCH", path, map[string]any{}); code != http.StatusConflict || ec != "grading_not_editable" {
		t.Fatalf("patch queued = %d %s", code, ec)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", path+"/send", nil); code != http.StatusConflict || ec != "grading_not_sendable" {
		t.Fatalf("send queued = %d %s", code, ec)
	}
	f.runJobs(t)

	teacherPoint := []map[string]any{{
		"kind": "issue", "quote": nil, "text": "第二段请补充数据来源。", "action": nil, "source": "teacher",
		// 🚨 **送的是老师屏幕上那个名字，不是 id。**
		//
		// 这一格必须和 gradingLogic.ts 的 contentForSave 真正发出去的东西一样：
		// 服务端渲染给她的是名字（gradingContentForView），她按保存时客户端把
		// 收到的那份原样送回来。上一版这里送的是 id —— 一条真客户端发不出的
		// 形状 —— 于是「每保存一次就把对应毛病清空一次」这个缺陷一次都没被
		// 看见（2026-09-23 复查）。
		"dimension": "内容", "symptom": "只有主题，没有问题",
	}}
	rec := doJSON(t, f.h, f.teacher, "PATCH", path, mustJSON(t, map[string]any{"content": gradingContent("E", teacherPoint)}))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_grading") || !strings.Contains(rec.Body.String(), "总评的等级不在评分标准内：E") {
		t.Fatalf("bad grade = %d %s", rec.Code, rec.Body)
	}
	notHers := []map[string]any{{"kind": "issue", "quote": "雨一直下。", "text": "说明", "action": nil, "source": "teacher"}}
	rec = doJSON(t, f.h, f.teacher, "PATCH", path, mustJSON(t, map[string]any{"content": gradingContent("B", notHers)}))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "引文不在正文中") {
		t.Fatalf("teacher quote not hers = %d %s", rec.Code, rec.Body)
	}

	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "PATCH", path, map[string]any{"content": gradingContent("B", teacherPoint)}, &resp); code != http.StatusOK {
		t.Fatalf("patch = %d", code)
	}
	if resp.Grading.ReviewedAt == nil || resp.Grading.Status != "draft" || !strings.Contains(string(resp.Grading.Content), `"source":"teacher"`) {
		t.Fatalf("patched = %+v %s", resp.Grading, resp.Grading.Content)
	}
	// A rubric-matching dimension survives verbatim; the symptom comes back
	// as the name the teacher reads, never as a raw code.
	if !strings.Contains(string(resp.Grading.Content), `"dimension":"内容"`) {
		t.Fatalf("dimension must survive a PATCH: %s", resp.Grading.Content)
	}
	if !strings.Contains(string(resp.Grading.Content), `"symptom":"只有主题，没有问题"`) {
		t.Fatalf("symptom must render as its name on a PATCH: %s", resp.Grading.Content)
	}
	if strings.Contains(string(resp.Grading.Content), "topic_without_question") {
		t.Fatalf("the teacher must never see a raw id: %s", resp.Grading.Content)
	}
	// 🚨 **存的是 id。** 名字只在渲染那一步换上去 —— 存名字的那一版不是
	// 幂等的，第二次 SanitizeProvenance 认不出自己写下的东西，把这一格清空。
	var stored []byte
	if err := f.pool.QueryRow(context.Background(), `SELECT content FROM lite_grading WHERE id = $1`, gid).Scan(&stored); err != nil {
		t.Fatalf("read stored content: %v", err)
	}
	if !strings.Contains(string(stored), "topic_without_question") || strings.Contains(string(stored), "只有主题，没有问题") {
		t.Fatalf("the id is what gets stored, not the display name: %s", stored)
	}
	// 再保存一次同一份内容 —— 对应毛病必须还在（这就是那条 blocker）。
	var again struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "PATCH", path, map[string]any{"content": gradingContent("B", teacherPoint)}, &again); code != http.StatusOK {
		t.Fatalf("second patch = %d", code)
	}
	if !strings.Contains(string(again.Grading.Content), `"symptom":"只有主题，没有问题"`) {
		t.Fatalf("a second save must not wipe 对应毛病: %s", again.Grading.Content)
	}
	var ai []byte
	if err := f.pool.QueryRow(context.Background(), `SELECT ai FROM lite_grading WHERE id = $1`, gid).Scan(&ai); err != nil || !strings.Contains(string(ai), `"B+"`) {
		t.Fatalf("ai must not change on a teacher edit: %s err=%v", ai, err)
	}

	if code := assignJSON(t, f.h, f.teacher, "POST", path+"/send", nil, &resp); code != http.StatusOK || resp.Grading.Status != "sent" || resp.Grading.SentAt == nil {
		t.Fatalf("send = %d %+v", code, resp.Grading)
	}
	// Regrading a sent row is refused (Task 5); editing it is allowed and re-sends.
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET student_seen_at = now() WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if code := assignJSON(t, f.h, f.teacher, "PATCH", path, map[string]any{"content": gradingContent("A-", teacherPoint)}, &resp); code != http.StatusOK || resp.Grading.Status != "sent" || resp.Grading.StudentSeenAt != nil {
		t.Fatalf("edit sent = %d %+v", code, resp.Grading)
	}

	// Failed rows are not editable.
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'failed' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "PATCH", path, map[string]any{}); code != http.StatusConflict || ec != "grading_not_editable" {
		t.Fatalf("patch failed row = %d %s", code, ec)
	}
}

func TestLiteGradingSendAllReviewed(t *testing.T) {
	f := newGradingFixture(t)
	s2 := createStudent(t, f.pool, SeedSchoolID, "gr-s2@demo.local")
	s3 := createStudent(t, f.pool, SeedSchoolID, "gr-s3@demo.local")
	enrollStudent(t, f.pool, s2, f.classID)
	enrollStudent(t, f.pool, s3, f.classID)
	ids := []uuid.UUID{f.studentID, s2, s3}
	idStrings := []string{f.studentID.String(), s2.String(), s3.String()}
	aid := createAssignment(t, f.h, f.teacher, f.classID, writingAssignmentBody(idStrings))
	for _, uid := range ids {
		c := signInAs(t, f.pool, uid)
		atomID := startAssignment(t, f.h, c, aid).AtomID
		if code := assignJSON(t, f.h, c, "PUT", "/api/v1/writings/"+atomID+"/draft", map[string]any{"body": gradingBody}, nil); code != http.StatusOK {
			t.Fatalf("draft = %d", code)
		}
		if code := assignJSON(t, f.h, c, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
			t.Fatalf("finish = %d", code)
		}
	}
	if n := f.queueAll(t, aid, false); n != 3 {
		t.Fatalf("queued = %d", n)
	}
	f.runJobs(t)
	rows := f.rows(t, aid)
	gids := make([]string, 0, 3)
	for _, r := range rows {
		gids = append(gids, r.Grading.ID)
	}
	// Mark one reviewed (PATCH with no content).
	if code := assignJSON(t, f.h, f.teacher, "PATCH", "/api/v1/lite/teacher/gradings/"+gids[0], map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("mark reviewed = %d", code)
	}
	var resp struct {
		Sent int `json:"sent"`
	}
	sendAll := "/api/v1/lite/teacher/assignments/" + aid + "/gradings/send"
	if code := assignJSON(t, f.h, f.teacher, "POST", sendAll, map[string]any{"ids": gids}, &resp); code != http.StatusOK || resp.Sent != 1 {
		t.Fatalf("send all = %d sent=%d, want 1", code, resp.Sent)
	}
	statuses := map[string]string{}
	for _, r := range f.rows(t, aid) {
		statuses[r.Grading.ID] = r.Grading.Status
	}
	if statuses[gids[0]] != "sent" || statuses[gids[1]] != "draft" || statuses[gids[2]] != "draft" {
		t.Fatalf("statuses = %v", statuses)
	}
	// The class's assignment list counts 待批改: submitted, nothing sent yet.
	var list struct {
		Assignments []struct {
			ID      string `json:"id"`
			ToGrade int    `json:"toGrade"`
		} `json:"assignments"`
	}
	getJSON(t, f.h, f.teacher, "/api/v1/lite/teacher/classes/"+f.classID+"/assignments", &list)
	if len(list.Assignments) != 1 || list.Assignments[0].ID != aid || list.Assignments[0].ToGrade != 2 {
		t.Fatalf("list = %+v, want toGrade 2", list.Assignments)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", sendAll, map[string]any{"ids": []string{"x"}}); code != http.StatusBadRequest || ec != "invalid_ids" {
		t.Fatalf("bad ids = %d %s", code, ec)
	}

	other := signInAs(t, f.pool, createTeacher(t, f.pool, SeedSchoolID, "gr-other2@demo.local"))
	for _, p := range []struct{ method, path string }{
		{"PATCH", "/api/v1/lite/teacher/gradings/" + gids[1]},
		{"POST", "/api/v1/lite/teacher/gradings/" + gids[1] + "/send"},
		{"POST", sendAll},
	} {
		if code, _ := writeErrorCode(t, f.h, other, p.method, p.path, map[string]any{"ids": gids}); code != http.StatusNotFound {
			t.Errorf("other teacher %s %s = %d, want 404", p.method, p.path, code)
		}
	}
}

// TestLiteGradingSendAllSkipsDepartedStudent — controller ruling 3: a listed
// id whose student has left the class is skipped even if her draft is
// otherwise reviewed and belongs to this assignment; the response's
// "skipped" count says so instead of silently sending everything else and
// dropping the mismatch, or sending a row for a student who is no longer
// this teacher's to grade.
func TestLiteGradingSendAllSkipsDepartedStudent(t *testing.T) {
	f := newGradingFixture(t)
	s2 := createStudent(t, f.pool, SeedSchoolID, "gr-s4@demo.local")
	enrollStudent(t, f.pool, s2, f.classID)
	idStrings := []string{f.studentID.String(), s2.String()}
	aid := createAssignment(t, f.h, f.teacher, f.classID, writingAssignmentBody(idStrings))
	for _, uid := range []uuid.UUID{f.studentID, s2} {
		c := signInAs(t, f.pool, uid)
		atomID := startAssignment(t, f.h, c, aid).AtomID
		if code := assignJSON(t, f.h, c, "PUT", "/api/v1/writings/"+atomID+"/draft", map[string]any{"body": gradingBody}, nil); code != http.StatusOK {
			t.Fatalf("draft = %d", code)
		}
		if code := assignJSON(t, f.h, c, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
			t.Fatalf("finish = %d", code)
		}
	}
	if n := f.queueAll(t, aid, false); n != 2 {
		t.Fatalf("queued = %d", n)
	}
	f.runJobs(t)
	byUser := map[string]string{}
	for _, r := range f.rows(t, aid) {
		byUser[r.UserID] = r.Grading.ID
	}
	gids := []string{byUser[f.studentID.String()], byUser[s2.String()]}
	// Both reviewed (PATCH with no content).
	for _, gid := range gids {
		if code := assignJSON(t, f.h, f.teacher, "PATCH", "/api/v1/lite/teacher/gradings/"+gid, map[string]any{}, nil); code != http.StatusOK {
			t.Fatalf("mark reviewed = %d", code)
		}
	}
	// s2 leaves the class after her draft was reviewed.
	if _, err := f.pool.Exec(context.Background(), `DELETE FROM enrollments WHERE user_id = $1 AND class_id = $2`, s2, f.classID); err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Sent    int `json:"sent"`
		Skipped int `json:"skipped"`
	}
	sendAll := "/api/v1/lite/teacher/assignments/" + aid + "/gradings/send"
	if code := assignJSON(t, f.h, f.teacher, "POST", sendAll, map[string]any{"ids": gids}, &resp); code != http.StatusOK || resp.Sent != 1 || resp.Skipped != 1 {
		t.Fatalf("send all = %d %+v, want sent=1 skipped=1", code, resp)
	}
	// The departed student's row is no longer listed at all (Task 5 ruling 2
	// filters the list to enrolled students), so read her row straight from
	// the DB to confirm it was skipped, not sent.
	gradingStatus := func(gid string) string {
		var s string
		if err := f.pool.QueryRow(context.Background(), `SELECT status FROM lite_grading WHERE id = $1`, gid).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if s := gradingStatus(byUser[f.studentID.String()]); s != "sent" {
		t.Fatalf("still-enrolled student not sent: %s", s)
	}
	if s := gradingStatus(byUser[s2.String()]); s != "draft" {
		t.Fatalf("departed student's reviewed draft was sent: %s", s)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

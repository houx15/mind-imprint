package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

type parentStageForTest struct {
	Cover struct {
		Name, Subject, Klass, TypeLabel, DateStr, WarmLine string
	} `json:"cover"`
	Stats []struct {
		Value, Label string
	} `json:"stats"`
	StageGrowth, StageHighlight, StageForward string
	Advice                                    []struct{ Title, Text string } `json:"advice"`
	Prose                                     *string                        `json:"prose"`
}

// seedStageStudent creates a teacher-owned class + enrolled student, one project
// report (surfaces in student_evaluation → reports=1 and feeds ability), and
// this-week usage events. Returns the class id, student id, and teacher cookie.
func seedStageStudent(t *testing.T, pool *pgxpool.Pool, h http.Handler, teacherEmail, studentEmail string) (classID, studentID string, teacherCookie *http.Cookie) {
	t.Helper()
	q := mustNewQueries(pool)
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, teacherEmail))
	cls := createClassViaAPI(t, h, teacher, "Parent Stage Class")
	student := createStudent(t, pool, SeedSchoolID, studentEmail)
	enrollStudent(t, pool, student, cls)

	proj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: student, Qualification: "0457", Title: "嵌入式体育博彩广告与博彩正常化",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	report := agent.Report{
		DepthAxis: []agent.DepthDim{
			{Code: "D1", Name: "任务理解与问题表述", Level: "L3", Evidence: "e1"},
			{Code: "D2", Name: "证据与信源", Level: "L3", Evidence: "e2"},
		},
		AutonomyAxis: []agent.AutonomySignal{
			{Code: "A3", Name: "边界主权", Level: 3, Opportunity: "given_taken", Evidence: "a3"},
			{Code: "A4", Name: "对抗与检验", Level: 2, Opportunity: "given_not_taken", Evidence: "a4"},
		},
	}
	scores, _ := json.Marshal(report)
	if _, err := q.InsertProjectEvaluation(context.Background(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgtype.UUID{Bytes: proj.ID, Valid: true},
		Scores:    scores, Narrative: "n", Model: "test-model", Tier: "flagship",
	}); err != nil {
		t.Fatalf("insert project evaluation: %v", err)
	}

	// This-week usage: 3 prompt_sent (turns=3) + 2 course step_viewed
	// (course_steps=2), all today (active_days=1).
	for i := 0; i < 3; i++ {
		if _, err := q.AppendEvent(context.Background(), sqlc.AppendEventParams{
			ProjectID: pgtype.UUID{Bytes: proj.ID, Valid: true},
			UserID:    student,                                              // event.user_id is a non-null uuid.UUID
			Surface:   "studio", Type: "prompt_sent", Payload: []byte(`{}`), // event_surface_check only allows studio/course/chat
		}); err != nil {
			t.Fatalf("append prompt_sent: %v", err)
		}
	}
	// event.course_id has an FK to course(id). Migration 0050 (course v2)
	// deleted every pre-existing seeded course row (`DELETE FROM course`), so
	// there is no longer a course guaranteed to exist — upsert a throwaway one
	// purely to satisfy the FK; its content is irrelevant, only its id.
	// distinctness for course_steps is by (course,ordinal), not which course.
	if _, err := q.UpsertCourse(context.Background(), sqlc.UpsertCourseParams{
		Slug: "parent-stage-report-fixture", Branch: "A", Title: "t", Blurb: "b",
		TimeLabel: "5 分钟", CardIds: []string{}, StepCount: 1,
		Structure: []byte(`{}`), RenderCache: []byte(`{}`),
	}); err != nil {
		t.Fatalf("upsert fixture course: %v", err)
	}
	course, cerr := q.GetCourseBySlug(context.Background(), "parent-stage-report-fixture")
	if cerr != nil {
		t.Fatalf("get fixture course: %v", cerr)
	}
	courseID := pgtype.UUID{Bytes: course.ID, Valid: true}
	for _, ord := range []string{"1", "2"} {
		if _, err := q.AppendEvent(context.Background(), sqlc.AppendEventParams{
			UserID:   student,
			CourseID: courseID, Surface: "course", Type: "step_viewed",
			Payload: []byte(fmt.Sprintf(`{"ordinal":%q}`, ord)),
		}); err != nil {
			t.Fatalf("append step_viewed: %v", err)
		}
	}
	return cls, student.String(), teacher
}

func stageURL(classID, studentID, week string) string {
	return fmt.Sprintf("/api/v1/classes/%s/students/%s/parent-stage-report/%s", classID, studentID, week)
}

func TestGetParentStageReport_DeterministicStats(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	classID, studentID, teacher := seedStageStudent(t, pool, h, "ps-teacher@demo.local", "ps-student@demo.local")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", stageURL(classID, studentID, "current"), nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var dto parentStageForTest
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if dto.Prose != nil {
		t.Errorf("prose should be nil before composition, got %v", *dto.Prose)
	}
	if dto.Cover.TypeLabel != "阶段报告" {
		t.Errorf("typeLabel = %q", dto.Cover.TypeLabel)
	}
	if len(dto.Stats) != 4 {
		t.Fatalf("want 4 stat cards, got %d", len(dto.Stats))
	}
	want := map[string]string{"本周活跃": "1 天", "对话轮次": "3", "生成报告": "1 份", "完成课程": "2 节"}
	for _, s := range dto.Stats {
		if w, ok := want[s.Label]; !ok || s.Value != w {
			t.Errorf("stat %q = %q, want %q", s.Label, s.Value, w)
		}
	}
	// Week label shape: 第 N 周（M.D–M.D）. Byte-slicing "第 " would split a
	// multi-byte rune mid-character, so compare with a prefix check instead.
	if !strings.HasPrefix(dto.Cover.Subject, "第 ") {
		t.Errorf("cover.subject not a week label: %q", dto.Cover.Subject)
	}
}

// stageProseStubReply is a valid agent.ParentStageProse JSON — no bare codes,
// no A numbers, no level codes; exactly 3 advice items; highlight non-empty.
const stageProseStubReply = `{
  "warmLine":"这一阶段，孩子在自己拿主意上表现突出。",
  "stageGrowth":"这段时间他更愿意先自己想清楚，再请 AI 帮忙检查，而不是一上来就要答案。",
  "stageHighlight":"本周他主动请 AI 扮演反方，来挑自己论证里的问题。",
  "stageForward":"可以给他更高一点的目标，鼓励他把研究的意义讲得更具体。",
  "advice":[
    {"title":"请他讲给你听","text":"让他用一句话说清这份研究不能说明什么。"},
    {"title":"保护他的自主","text":"鼓励他先自己判断，再去问 AI。"},
    {"title":"给一点挑战","text":"问他如果要再进一步，还差哪一步。"}
  ]
}`

func TestPostParentStageProse_ComposesOnceThenCostFree(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(pool)
	deps.Provider = assessStubProvider(stageProseStubReply)
	deps.EvalResolver = fakeEvalResolver()
	h := New(deps).Handler()

	classID, studentID, teacher := seedStageStudent(t, pool, h, "pss-teacher@demo.local", "pss-student@demo.local")
	url := stageURL(classID, studentID, "current") + "/prose"

	post := func() parentStageForTest {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest(http.MethodPost, url, nil), teacher))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var dto parentStageForTest
		if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
			t.Fatalf("decode: %v — body=%s", err, rec.Body)
		}
		return dto
	}

	first := post()
	if first.Prose == nil {
		t.Fatalf("first POST should compose: %+v", first)
	}
	if len(first.Advice) != 3 || first.StageGrowth == "" {
		t.Errorf("composed prose incomplete: %+v", first)
	}
	if n := countParentReportLLMCalls(t, pool); n != 1 {
		t.Fatalf("want 1 parent_report llm_call, got %d", n)
	}
	second := post()
	if second.Prose == nil {
		t.Fatal("second POST should return stored prose")
	}
	if n := countParentReportLLMCalls(t, pool); n != 1 {
		t.Fatalf("second POST must not spend; llm_calls=%d", n)
	}
}

func TestPostParentStageProse_RejectionNeverWalls(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(pool)
	deps.Provider = assessStubProvider(`not json`)
	deps.EvalResolver = fakeEvalResolver()
	h := New(deps).Handler()

	classID, studentID, teacher := seedStageStudent(t, pool, h, "pssf-teacher@demo.local", "pssf-student@demo.local")
	url := stageURL(classID, studentID, "current") + "/prose"

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(http.MethodPost, url, nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("rejected compose must never wall: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var dto parentStageForTest
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.Prose != nil {
		t.Errorf("prose must be nil after rejection, got %v", *dto.Prose)
	}
	if len(dto.Stats) != 4 {
		t.Fatalf("deterministic stats must still render: %d", len(dto.Stats))
	}
	if n := countParentReportLLMCalls(t, pool); n != 1 {
		t.Fatalf("cost recorded even on rejection; llm_calls=%d, want 1", n)
	}
}

// TestGetParentStageReport_ForeignTeacher404 — a teacher who does not own the
// student's class gets 404 (existence-hidden).
func TestGetParentStageReport_ForeignTeacher404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	classID, studentID, _ := seedStageStudent(t, pool, h, "ps-owner@demo.local", "ps-student2@demo.local")
	outsider := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ps-outsider@demo.local"))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", stageURL(classID, studentID, "current"), nil), outsider))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign teacher got %d, want 404: body=%s", rec.Code, rec.Body.String())
	}
	_ = time.Now
}

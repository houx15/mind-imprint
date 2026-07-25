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
	// event.course_id has an FK to course(id); use a real seeded course —
	// distinctness for course_steps is by (course,ordinal), not which course.
	courses, cerr := q.ListCourses(context.Background())
	if cerr != nil || len(courses) == 0 {
		t.Fatalf("list courses: %v (len=%d)", cerr, len(courses))
	}
	courseID := pgtype.UUID{Bytes: courses[0].ID, Valid: true}
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

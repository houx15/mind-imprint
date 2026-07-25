package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// parentReportForTest mirrors ParentReportDTO's JSON shape for decoding.
type parentReportForTest struct {
	Cover struct {
		Name      string `json:"name"`
		Subject   string `json:"subject"`
		Klass     string `json:"klass"`
		TypeLabel string `json:"typeLabel"`
		DateStr   string `json:"dateStr"`
		WarmLine  string `json:"warmLine"`
	} `json:"cover"`
	DRows []struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Badge   string `json:"badge"`
		Reading string `json:"reading"`
	} `json:"dRows"`
	ARows []struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		State   string `json:"state"`
		Reading string `json:"reading"`
	} `json:"aRows"`
	ProseReady bool    `json:"proseReady"`
	Prose      *string `json:"prose"`
}

func TestGetParentReport_DeterministicNoProse(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	q := mustNewQueries(pool)

	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pr-teacher@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "Parent Report Class")

	studentID := createStudent(t, pool, SeedSchoolID, "pr-student@demo.local")
	enrollStudent(t, pool, studentID, classID)

	proj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: studentID, Qualification: "0457", Title: "中国是否让地球更可持续？",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	report := agent.Report{
		DepthAxis: []agent.DepthDim{
			{Code: "D1", Name: "论证深度", Level: "L1", Evidence: "e1"},
			{Code: "D2", Name: "证据运用", Level: "L2", Evidence: "e2"},
			{Code: "D3", Name: "视角覆盖", Level: "L3", Evidence: "e3"},
			{Code: "D4", Name: "结构组织", Level: "L4", Evidence: "e4"},
			{Code: "D5", Name: "反思迭代", Level: "NA", Evidence: ""},
			{Code: "D6", Name: "真实性", Level: "L2", Evidence: "e6"},
		},
		AutonomyAxis: []agent.AutonomySignal{
			{Code: "A1", Name: "主动提问", Level: 0, Opportunity: "not_supplied", Evidence: "a1"},
			{Code: "A2", Name: "主动质疑", Level: 1, Opportunity: "given_not_taken", Evidence: "a2"},
			{Code: "A3", Name: "主动求证", Level: 2, Opportunity: "given_taken", Evidence: "a3"},
			{Code: "A4", Name: "主动修订", Level: 3, Opportunity: "given_taken", Evidence: "a4"},
			{Code: "A5", Name: "主动拓展", Level: 4, Opportunity: "given_taken", Evidence: "a5"},
			{Code: "A6", Name: "主动核查", Level: 5, Opportunity: "given_taken", Evidence: "a6"},
		},
	}
	scores, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if _, err := q.InsertProjectEvaluation(context.Background(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgtype.UUID{Bytes: proj.ID, Valid: true},
		Scores:    scores, Narrative: "n", Model: "test-model", Tier: "flagship",
	}); err != nil {
		t.Fatalf("insert project evaluation: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(
		"GET", fmt.Sprintf("/api/v1/classes/%s/students/%s/parent-report/project/%s", classID, studentID, proj.ID), nil,
	), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var dto parentReportForTest
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if dto.ProseReady {
		t.Error("prose should be absent before composition")
	}
	if dto.Prose != nil {
		t.Errorf("prose sentinel should be nil before composition, got %v", *dto.Prose)
	}
	if len(dto.DRows) != 6 || len(dto.ARows) != 6 {
		t.Fatalf("want 6 D + 6 A rows, got %d/%d", len(dto.DRows), len(dto.ARows))
	}
	// Badges are the parent 四台阶 labels, never L-codes or numbers.
	for _, d := range dto.DRows {
		switch d.Badge {
		case "起步", "发展", "熟练", "优秀", "暂无":
		default:
			t.Errorf("D badge %q not a 四台阶 label", d.Badge)
		}
	}
	for _, a := range dto.ARows {
		switch a.State {
		case "观察到主动信号", "偶有·多在引导后", "暂未观察到":
		default:
			t.Errorf("A state %q not a 三态 label", a.State)
		}
	}
	if dto.Cover.Subject != "研究项目 · 中国是否让地球更可持续？" {
		t.Errorf("cover.subject = %q", dto.Cover.Subject)
	}
	if dto.Cover.Klass != "Parent Report Class" {
		t.Errorf("cover.klass = %q", dto.Cover.Klass)
	}
	if dto.Cover.WarmLine != "" {
		t.Errorf("cover.warmLine should be empty before composition, got %q", dto.Cover.WarmLine)
	}
}

// TestGetParentReport_ForeignTeacher404 — a teacher who does not own the
// student's class gets 404 (existence-hidden), mirroring
// TestStudentReportProjectHappyPath's sibling guard in teacher_read_test.go.
func TestGetParentReport_ForeignTeacher404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	q := mustNewQueries(pool)

	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pr-owner@demo.local"))
	classID := createClassViaAPI(t, h, owner, "Parent Report Owned Class")

	studentID := createStudent(t, pool, SeedSchoolID, "pr-student2@demo.local")
	enrollStudent(t, pool, studentID, classID)

	proj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: studentID, Qualification: "0457", Title: "另一个项目",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	report := agent.Report{
		DepthAxis:    []agent.DepthDim{{Code: "D1", Name: "论证深度", Level: "L1", Evidence: "e1"}},
		AutonomyAxis: []agent.AutonomySignal{{Code: "A1", Name: "主动提问", Level: 1, Opportunity: "given_taken", Evidence: "a1"}},
	}
	scores, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if _, err := q.InsertProjectEvaluation(context.Background(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgtype.UUID{Bytes: proj.ID, Valid: true},
		Scores:    scores, Narrative: "n", Model: "test-model", Tier: "flagship",
	}); err != nil {
		t.Fatalf("insert project evaluation: %v", err)
	}

	outsider := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pr-outsider@demo.local"))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(
		"GET", fmt.Sprintf("/api/v1/classes/%s/students/%s/parent-report/project/%s", classID, studentID, proj.ID), nil,
	), outsider))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign teacher got %d, want 404: body=%s", rec.Code, rec.Body.String())
	}
}

package api_test

// evaluation_report_test.go — Task 5: GET/POST /api/v1/projects/{id}/evaluation-report(/generate)
// + GET /api/v1/evaluation-reports. Read paths never call a model; POST
// generate is first-open-wins (a second call returns the same stored report,
// never regenerates). Today's generation core emits evalreport.Placeholder —
// a full, deterministic fixture — standing in for the real algorithm.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// TestEvaluationReport_GenerateThenRead walks the whole pipeline against the
// seeded demo project (materialsTestProjectID, owned by Phoebe/SeedUserID):
// null before generate, POST generate succeeds, GET then returns an abundant
// report (6 depth dims, >=6 materials), and the timeline lists the project.
func TestEvaluationReport_GenerateThenRead(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: mustNewQueries(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	// initially null — no report has been generated yet.
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/evaluation-report", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET evaluation-report (empty) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("GET evaluation-report (empty) body = %s, want literal null", rec.Body)
	}

	// generate
	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/evaluation-report/generate", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST evaluation-report/generate = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// now present + abundant
	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/evaluation-report", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET evaluation-report (after generate) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	rep := decodeReadyEnvelope(t, rec.Body.Bytes())
	if len(rep.Depth) != 6 {
		t.Fatalf("report.Depth len = %d, want 6", len(rep.Depth))
	}
	if len(rep.Materials) < 6 {
		t.Fatalf("report.Materials len = %d, want >= 6", len(rep.Materials))
	}
	if rep.ProjectID != projectID {
		t.Fatalf("report.ProjectID = %q, want %q", rep.ProjectID, projectID)
	}

	// second generate call is first-open-wins: same report, no regeneration.
	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/evaluation-report/generate", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("second POST evaluation-report/generate = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	rep2 := decodeReadyEnvelope(t, rec.Body.Bytes())
	if rep2.ReportID != rep.ReportID {
		t.Fatalf("second generate produced a different report (reportId %q vs %q) — want first-open-wins", rep2.ReportID, rep.ReportID)
	}

	// list includes it
	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("GET", "/api/v1/evaluation-reports", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET evaluation-reports (list) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var listBody struct {
		Entries []struct {
			ProjectID string `json:"projectId"`
			Title     string `json:"title"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode evaluation-reports list: %v — body=%s", err, rec.Body)
	}
	found := false
	for _, e := range listBody.Entries {
		if e.ProjectID == projectID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("timeline missing the generated report for project %s: %+v", projectID, listBody.Entries)
	}
}

// TestEvaluationReport_RejectsOtherUsersProject — ownership hidden as
// not-found, same idiom as every other project-scoped route (loadOwnedProject).
func TestEvaluationReport_RejectsOtherUsersProject(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: mustNewQueries(pool), Pool: pool}).Handler()
	other := createStudent(t, pool, SeedSchoolID, "evaluation-report-other@demo.local")
	cookie := signInAs(t, pool, other)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/evaluation-report", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET evaluation-report (other user) = %d, want 404 (ownership hidden as not-found); body=%s", rec.Code, rec.Body)
	}
}

// TestEvaluationReport_TeacherReadsStudentReport — Task 13: a teacher who
// owns the student's class can read the SAME EvaluationReport the student
// sees, via the teacher-scoped route. Read-no-call — the report must already
// exist (generated here via the student's own POST, mirroring
// TestEvaluationReport_GenerateThenRead's use of the placeholder generator).
func TestEvaluationReport_TeacherReadsStudentReport(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: mustNewQueries(pool), Pool: pool}).Handler()
	q := mustNewQueries(pool)

	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ter-teacher@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "Evaluation Report Class")

	studentID := createStudent(t, pool, SeedSchoolID, "ter-student@demo.local")
	enrollStudent(t, pool, studentID, classID)
	studentCookie := signInAs(t, pool, studentID)

	proj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: studentID, Qualification: "0457", Title: "teacher-readable project",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// The student generates their own report first (first-open-wins) — the
	// teacher route never generates.
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+proj.ID.String()+"/evaluation-report/generate", strings.NewReader("")), studentCookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("student POST evaluation-report/generate = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID+"/students/"+studentID.String()+"/evaluation-report/"+proj.ID.String(), nil), teacher)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("teacher GET evaluation-report = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	rep := decodeReadyEnvelope(t, rec.Body.Bytes())
	if rep.ProjectID != proj.ID.String() {
		t.Fatalf("report.ProjectID = %q, want %q", rep.ProjectID, proj.ID.String())
	}
	if len(rep.Depth) != 6 {
		t.Fatalf("report.Depth len = %d, want 6", len(rep.Depth))
	}
}

// TestEvaluationReport_TeacherOfOtherClass404s — a teacher who does not own
// the student's class gets 404 (cross-class access hidden as not-found, same
// idiom as every other teacher-scoped route).
func TestEvaluationReport_TeacherOfOtherClass404s(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: mustNewQueries(pool), Pool: pool}).Handler()
	q := mustNewQueries(pool)

	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ter-owner@demo.local"))
	classID := createClassViaAPI(t, h, owner, "Owned Evaluation Report Class")

	studentID := createStudent(t, pool, SeedSchoolID, "ter-other-student@demo.local")
	enrollStudent(t, pool, studentID, classID)
	studentCookie := signInAs(t, pool, studentID)

	proj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: studentID, Qualification: "0457", Title: "cross-class project",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+proj.ID.String()+"/evaluation-report/generate", strings.NewReader("")), studentCookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("student POST evaluation-report/generate = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	stranger := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ter-stranger@demo.local"))
	rec2 := httptest.NewRecorder()
	req2 := withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID+"/students/"+studentID.String()+"/evaluation-report/"+proj.ID.String(), nil), stranger)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("teacher of another class got %d, want 404; body=%s", rec2.Code, rec2.Body)
	}
}

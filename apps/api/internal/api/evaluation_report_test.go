package api_test

// evaluation_report_test.go — Task 5: GET/POST /api/v1/projects/{id}/evaluation-report(/generate)
// + GET /api/v1/evaluation-reports. Read paths never call a model; POST
// generate is first-open-wins (a second call returns the same stored report,
// never regenerates). Today's generation core emits evalreport.Placeholder —
// a full, deterministic fixture — standing in for the real algorithm.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/evalreport"
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
	var rep evalreport.Report
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatalf("decode evaluation report: %v — body=%s", err, rec.Body)
	}
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
	var rep2 evalreport.Report
	if err := json.Unmarshal(rec.Body.Bytes(), &rep2); err != nil {
		t.Fatalf("decode second evaluation report: %v — body=%s", err, rec.Body)
	}
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

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// rosterReportEntryForTest mirrors RosterReportEntry's JSON shape for decoding
// in these tests.
type rosterReportEntryForTest struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarColor string `json:"avatarColor"`
	DBadge      string `json:"dBadge"`
	ABadge      string `json:"aBadge"`
	ActiveDays  int32  `json:"activeDays"`
	Turns       int32  `json:"turns"`
	HasReport   bool   `json:"hasReport"`
	Unrated     bool   `json:"unrated"`
}

// TestRosterReportHappyPath — a teacher of the class sees one rated student
// (dBadge derived from her latest project report) and one unrated student
// ("—", unrated=true).
func TestRosterReportHappyPath(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	q := mustNewQueries(pool)

	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "rr-teacher@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "Roster Report Class")

	// Rated student: seed a project + a project evaluation.
	ratedID := createStudent(t, pool, SeedSchoolID, "rr-rated@demo.local")
	enrollStudent(t, pool, ratedID, classID)
	proj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID:        ratedID,
		Qualification: "0457",
		Title:         "rated student's project",
		Deadline:      pgtype.Timestamptz{},
		BoardCfgVer:   1,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	report := agent.Report{
		DepthAxis: []agent.DepthDim{
			{Code: "D1", Level: "L2"},
			{Code: "D2", Level: "L4"},
		},
		AutonomyAxis: []agent.AutonomySignal{
			{Code: "A1", Level: 4, Opportunity: "given_taken"},
		},
	}
	scores, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if _, err := q.InsertProjectEvaluation(context.Background(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgtype.UUID{Bytes: proj.ID, Valid: true},
		Scores:    scores,
		Narrative: "n",
		Model:     "test-model",
		Tier:      "flagship",
	}); err != nil {
		t.Fatalf("insert evaluation: %v", err)
	}

	// Unrated student: enrolled, no project/evaluation at all.
	unratedID := createStudent(t, pool, SeedSchoolID, "rr-unrated@demo.local")
	enrollStudent(t, pool, unratedID, classID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID+"/roster-report", nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("roster-report got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Roster []rosterReportEntryForTest `json:"roster"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(resp.Roster) != 2 {
		t.Fatalf("want 2 roster entries, got %d: %+v", len(resp.Roster), resp.Roster)
	}

	var rated, unrated *rosterReportEntryForTest
	for i := range resp.Roster {
		switch resp.Roster[i].ID {
		case ratedID.String():
			rated = &resp.Roster[i]
		case unratedID.String():
			unrated = &resp.Roster[i]
		}
	}
	if rated == nil || unrated == nil {
		t.Fatalf("missing expected students in roster: %+v", resp.Roster)
	}
	if rated.Unrated {
		t.Fatalf("rated student should have unrated=false: %+v", rated)
	}
	if rated.DBadge != "L2–L4" {
		t.Fatalf("rated student dBadge = %q, want L2–L4: %+v", rated.DBadge, rated)
	}
	if !rated.HasReport {
		t.Fatalf("rated student hasReport should be true: %+v", rated)
	}
	if rated.DisplayName == "" || rated.AvatarColor == "" {
		t.Fatalf("rated student missing display fields: %+v", rated)
	}

	if !unrated.Unrated {
		t.Fatalf("unrated student should have unrated=true: %+v", unrated)
	}
	if unrated.DBadge != "—" || unrated.ABadge != "—" {
		t.Fatalf("unrated student badges should be em-dash: %+v", unrated)
	}
	if unrated.HasReport {
		t.Fatalf("unrated student hasReport should be false: %+v", unrated)
	}
}

// TestRosterReportCrossClassTeacher404s — a teacher who does not own the
// class gets 404 (existence hidden), same as the plain roster route.
func TestRosterReportCrossClassTeacher404s(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()

	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "rr-owner@demo.local"))
	classID := createClassViaAPI(t, h, owner, "Owned Roster Report Class")

	otherSchool := seedSecondSchool(t, pool)
	stranger := signInAs(t, pool, createTeacher(t, pool, otherSchool, "rr-stranger@other.local"))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID+"/roster-report", nil), stranger))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-class teacher got %d, want 404", rec.Code)
	}
}

// studentDetailForTest mirrors getStudentDetail's JSON envelope for decoding.
type studentDetailForTest struct {
	Student struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		AvatarColor string `json:"avatarColor"`
		DBadge      string `json:"dBadge"`
		ABadge      string `json:"aBadge"`
		Unrated     bool   `json:"unrated"`
	} `json:"student"`
	Usage struct {
		ActiveDays  int64 `json:"activeDays"`
		Turns       int64 `json:"turns"`
		ReportCount int   `json:"reportCount"`
		CourseCount int   `json:"courseCount"`
	} `json:"usage"`
	Records []struct {
		Surface   string `json:"surface"`
		ScopeID   string `json:"scopeId"`
		Title     string `json:"title"`
		Date      string `json:"date"`
		Status    string `json:"status"`
		HasReport bool   `json:"hasReport"`
	} `json:"records"`
}

// TestStudentDetailHappyPath — a teacher opens a member student's detail page:
// one project has a report (hasReport:true, feeds the head D/A badges), a
// second project has none (hasReport:false, "进行中").
func TestStudentDetailHappyPath(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	q := mustNewQueries(pool)

	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "sd-teacher@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "Student Detail Class")

	studentID := createStudent(t, pool, SeedSchoolID, "sd-student@demo.local")
	enrollStudent(t, pool, studentID, classID)

	reportedProj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: studentID, Qualification: "0457", Title: "reported project",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create reported project: %v", err)
	}
	report := agent.Report{
		DepthAxis:    []agent.DepthDim{{Code: "D1", Level: "L3"}},
		AutonomyAxis: []agent.AutonomySignal{{Code: "A1", Level: 3, Opportunity: "given_taken"}},
	}
	scores, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if _, err := q.InsertProjectEvaluation(context.Background(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgtype.UUID{Bytes: reportedProj.ID, Valid: true},
		Scores:    scores, Narrative: "n", Model: "test-model", Tier: "flagship",
	}); err != nil {
		t.Fatalf("insert evaluation: %v", err)
	}

	unreportedProj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: studentID, Qualification: "0457", Title: "unreported project",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create unreported project: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID+"/students/"+studentID.String(), nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("student detail got %d body=%s", rec.Code, rec.Body)
	}
	var resp studentDetailForTest
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}

	if resp.Student.ID != studentID.String() {
		t.Fatalf("student.id = %q, want %q", resp.Student.ID, studentID.String())
	}
	if resp.Student.Unrated {
		t.Fatalf("student should be rated (has a project report): %+v", resp.Student)
	}
	if resp.Student.DBadge != "L3" {
		t.Fatalf("dBadge = %q, want L3: %+v", resp.Student.DBadge, resp.Student)
	}
	if resp.Student.DisplayName == "" || resp.Student.AvatarColor == "" {
		t.Fatalf("student missing display fields: %+v", resp.Student)
	}
	if resp.Usage.ReportCount != 1 {
		t.Fatalf("reportCount = %d, want 1: %+v", resp.Usage.ReportCount, resp.Usage)
	}
	if resp.Usage.CourseCount != 0 {
		t.Fatalf("courseCount = %d, want 0: %+v", resp.Usage.CourseCount, resp.Usage)
	}
	if len(resp.Records) != 2 {
		t.Fatalf("want 2 records, got %d: %+v", len(resp.Records), resp.Records)
	}
	var reportedRec, unreportedRec *struct {
		Surface   string `json:"surface"`
		ScopeID   string `json:"scopeId"`
		Title     string `json:"title"`
		Date      string `json:"date"`
		Status    string `json:"status"`
		HasReport bool   `json:"hasReport"`
	}
	for i := range resp.Records {
		switch resp.Records[i].ScopeID {
		case reportedProj.ID.String():
			reportedRec = &resp.Records[i]
		case unreportedProj.ID.String():
			unreportedRec = &resp.Records[i]
		}
	}
	if reportedRec == nil || unreportedRec == nil {
		t.Fatalf("missing expected records: %+v", resp.Records)
	}
	if !reportedRec.HasReport {
		t.Fatalf("reported project record hasReport = false, want true: %+v", reportedRec)
	}
	if unreportedRec.HasReport {
		t.Fatalf("unreported project record hasReport = true, want false: %+v", unreportedRec)
	}
	if unreportedRec.Status != "进行中" {
		t.Fatalf("unreported project status = %q, want 进行中: %+v", unreportedRec.Status, unreportedRec)
	}
}

// TestStudentDetailCrossClassOrNonMember404s — a student who belongs to a
// different class, and a random non-member userId, both 404 for the teacher.
func TestStudentDetailCrossClassOrNonMember404s(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()

	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "sd-owner@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "Owned Student Detail Class")
	otherClassID := createClassViaAPI(t, h, teacher, "Other Student Detail Class")

	outsider := createStudent(t, pool, SeedSchoolID, "sd-outsider@demo.local")
	enrollStudent(t, pool, outsider, otherClassID) // member of a DIFFERENT class, not this one

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID+"/students/"+outsider.String(), nil), teacher))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-class student got %d, want 404", rec.Code)
	}

	nonMember := createStudent(t, pool, SeedSchoolID, "sd-nonmember@demo.local") // not enrolled anywhere
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID+"/students/"+nonMember.String(), nil), teacher))
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("non-member student got %d, want 404", rec2.Code)
	}
}

// TestRosterReportDeniedToStudent — the route requires teacher/admin role.
func TestRosterReportDeniedToStudent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()

	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "rr-owner2@demo.local"))
	classID := createClassViaAPI(t, h, owner, "Student Denied Class")

	student := signInSeed(t, pool) // Phoebe, a plain student

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID+"/roster-report", nil), student))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student got %d, want 403", rec.Code)
	}
}

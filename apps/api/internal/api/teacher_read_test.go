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

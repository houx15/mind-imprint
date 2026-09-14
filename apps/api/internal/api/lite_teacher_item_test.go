package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// TestLiteItemDetailNeverReturnsChat is spec DEC-1's guard: teachers see
// outputs and moments, never the chat with 印记. No code path in this
// endpoint may read atom_message.content into the response.
func TestLiteItemDetailNeverReturnsChat(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atom := seedLiteReadingForUser(t, pool, studentID, "active", 0)
	seedAtomMessage(t, pool, atom, "student", "SECRET-CHAT-LINE-7731")
	seedAtomMessage(t, pool, atom, "ai", "SECRET-AI-LINE-7731")
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO atom_annotation (atom_id, block_id, span, quote, note) VALUES ($1, 'b1', '{}', 'article words', 'her note')`, atom); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET",
		"/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("item = %d body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if strings.Contains(body, "SECRET-CHAT-LINE-7731") || strings.Contains(body, "SECRET-AI-LINE-7731") {
		t.Fatalf("chat content leaked: %s", body)
	}
	if !strings.Contains(body, "her note") {
		t.Fatalf("highlight note missing: %s", body)
	}
}

func TestLiteItemDetailAtomOfAnotherStudent404(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	stranger := createStudent(t, pool, SeedSchoolID, "lt-item-stranger@demo.local")
	atom := seedLiteReadingForUser(t, pool, stranger, "active", 0)
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), nil); code != http.StatusNotFound {
		t.Fatalf("foreign atom = %d, want 404", code)
	}
}

func TestLiteItemDetailWritingDraft(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atom := seedLiteWritingForUser(t, pool, studentID, "active")
	if _, err := pool.Exec(context.Background(), `INSERT INTO writing_draft (atom_id, body) VALUES ($1, 'HER-DRAFT-BODY')`, atom); err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Writing struct {
			Draft string `json:"draft"`
		} `json:"writing"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), &resp); code != http.StatusOK {
		t.Fatalf("item = %d", code)
	}
	if resp.Writing.Draft != "HER-DRAFT-BODY" {
		t.Fatalf("draft = %q", resp.Writing.Draft)
	}
}

func TestLiteItemDetailDoesNotTouchActivity(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atom := seedLiteReadingForUser(t, pool, studentID, "active", 0)
	var before, after string
	_ = pool.QueryRow(context.Background(), `SELECT last_activity_at::text FROM atom WHERE id=$1`, atom).Scan(&before)
	getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), nil)
	_ = pool.QueryRow(context.Background(), `SELECT last_activity_at::text FROM atom WHERE id=$1`, atom).Scan(&after)
	if before != after {
		t.Fatalf("teacher read bumped last_activity_at %s → %s", before, after)
	}
}

// TestLiteItemDetailReportTwoPhase is fix round 1's I1 regression test: the
// teacher report slice must surface the two-phase report protocol
// (atom_report.go) rather than silently presenting a phase-1 report — no
// moments yet, prose still owed — as if it were finished.
//
// Reuses reportStubProvider (atom_report_test.go) and the same
// corpus-overlap trick finishedReadingID uses (atom_report_share_test.go):
// the canned moment's quote must live somewhere in the corpus OTHER than
// her takeaway, or dedupeMomentsAgainstKeep (F4) drops it as a duplicate of
// `keep`.
func TestLiteItemDetailReportTwoPhase(t *testing.T) {
	prov := reportStubProvider()
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	ctx := context.Background()
	atom := seedLiteReadingForUser(t, pool, studentID, "finished", 0)
	if _, err := pool.Exec(ctx,
		`INSERT INTO reading_takeaway (atom_id, text) VALUES ($1, $2)`, atom,
		"我觉得应该多看数据来源，而不是只看结论"); err != nil {
		t.Fatal(err)
	}
	seedAtomMessage(t, pool, atom, "student", "我又想了想，数据来源要能查到出处，这样才可信。")

	path := "/api/v1/lite/teacher/classes/" + classID + "/students/" + studentID.String() + "/items/" + atom.String()

	var first struct {
		Report struct {
			Stats        []struct{ Key string } `json:"stats"`
			ProsePending bool                   `json:"prosePending"`
		} `json:"report"`
		ReportError *string `json:"reportError"`
	}
	if code := getJSON(t, h, teacher, path, &first); code != http.StatusOK {
		t.Fatalf("first call = %d", code)
	}
	if len(first.Report.Stats) == 0 {
		t.Fatalf("phase 1 must carry her stats — resp=%+v", first)
	}
	if !first.Report.ProsePending {
		t.Fatalf("phase 1 must flag prosePending — resp=%+v", first)
	}
	if first.ReportError != nil {
		t.Fatalf("reportError = %q, want nil on phase 1", *first.ReportError)
	}
	var reportRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM atom_report WHERE atom_id = $1`, atom).Scan(&reportRows); err != nil {
		t.Fatal(err)
	}
	if reportRows != 1 {
		t.Fatalf("atom_report rows = %d, want 1 after phase 1", reportRows)
	}

	var second struct {
		Report struct {
			ProsePending bool              `json:"prosePending"`
			Moments      []json.RawMessage `json:"moments"`
		} `json:"report"`
	}
	if code := getJSON(t, h, teacher, path, &second); code != http.StatusOK {
		t.Fatalf("second call = %d", code)
	}
	if second.Report.ProsePending {
		t.Fatalf("phase 2 must clear prosePending once the prose lands — resp=%+v", second)
	}
	if second.Report.Moments == nil {
		t.Fatalf("phase 2 must carry a moments array — resp=%+v", second)
	}
}

// seedLiteWebsiteProject creates a project atom of kind 'website' for
// studentID, owned by the fixture's pool/queries. Migrations read for the
// NOT NULL columns: 0108 (pbl_project: idea, kind required), 0109
// (pbl_plan_version/pbl_plan_step: decide required, no default).
func seedLiteWebsiteProject(t *testing.T, pool *pgxpool.Pool, studentID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	q := sqlc.New(pool)
	atom, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "project", UserID: studentID})
	if err != nil {
		t.Fatalf("seedLiteWebsiteProject: create atom: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO pbl_project (atom_id, idea, kind, status) VALUES ($1, $2, 'website', 'running')`,
		atom.ID, "我想做一个关于校园回收的主页"); err != nil {
		t.Fatalf("seedLiteWebsiteProject: create pbl_project: %v", err)
	}
	return atom.ID
}

// TestLiteItemDetailProjectPlanAndSite is fix round 1's I2 regression test
// (case 1): an approved live plan with two steps (one done) and a published
// site with a share token.
func TestLiteItemDetailProjectPlanAndSite(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	q := sqlc.New(pool)
	atomID := seedLiteWebsiteProject(t, pool, studentID)

	version, err := q.CreatePblPlanVersion(ctx, sqlc.CreatePblPlanVersionParams{
		AtomID: atomID, Version: 1, Summary: "v1", Reason: "她批准了第一版", DecidedBy: "student",
	})
	if err != nil {
		t.Fatalf("create plan version: %v", err)
	}
	if _, err := q.ApprovePblPlanVersion(ctx, version.ID); err != nil {
		t.Fatalf("approve plan version: %v", err)
	}
	if _, err := q.CreatePblPlanStep(ctx, sqlc.CreatePblPlanStepParams{
		VersionID: version.ID, Ordinal: 1, Title: "调研现状", Decide: "先看数据", Status: "done",
	}); err != nil {
		t.Fatalf("create step 1: %v", err)
	}
	if _, err := q.CreatePblPlanStep(ctx, sqlc.CreatePblPlanStepParams{
		VersionID: version.ID, Ordinal: 2, Title: "写页面文案", Decide: "还没定", Status: "tentative",
	}); err != nil {
		t.Fatalf("create step 2: %v", err)
	}

	if _, err := q.EnsurePblSite(ctx, sqlc.EnsurePblSiteParams{
		UserID: studentID, AtomID: pgtype.UUID{Bytes: atomID, Valid: true},
	}); err != nil {
		t.Fatalf("ensure site: %v", err)
	}
	token := "SITE-TOKEN-ABC"
	if _, err := q.SetPblSiteShare(ctx, sqlc.SetPblSiteShareParams{
		UserID: studentID, ShareToken: &token, PublishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		t.Fatalf("set site share: %v", err)
	}

	var resp struct {
		Project struct {
			StepsDone  int     `json:"stepsDone"`
			StepsTotal int     `json:"stepsTotal"`
			SiteToken  *string `json:"siteToken"`
		} `json:"project"`
	}
	path := "/api/v1/lite/teacher/classes/" + classID + "/students/" + studentID.String() + "/items/" + atomID.String()
	if code := getJSON(t, h, teacher, path, &resp); code != http.StatusOK {
		t.Fatalf("item = %d", code)
	}
	if resp.Project.StepsDone != 1 {
		t.Fatalf("stepsDone = %d, want 1", resp.Project.StepsDone)
	}
	if resp.Project.StepsTotal != 2 {
		t.Fatalf("stepsTotal = %d, want 2", resp.Project.StepsTotal)
	}
	if resp.Project.SiteToken == nil || *resp.Project.SiteToken != token {
		t.Fatalf("siteToken = %v, want %q", resp.Project.SiteToken, token)
	}
}

// TestLiteItemDetailProjectSiteTokenNilWhenUnpublished is fix round 1's I2
// regression test (case 2): the same website project, but the site row has
// never been published (share_token NULL, published_at NULL) — siteToken
// must be null, not an empty string.
func TestLiteItemDetailProjectSiteTokenNilWhenUnpublished(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	q := sqlc.New(pool)
	atomID := seedLiteWebsiteProject(t, pool, studentID)

	if _, err := q.EnsurePblSite(ctx, sqlc.EnsurePblSiteParams{
		UserID: studentID, AtomID: pgtype.UUID{Bytes: atomID, Valid: true},
	}); err != nil {
		t.Fatalf("ensure site: %v", err)
	}

	var resp struct {
		Project struct {
			SiteToken *string `json:"siteToken"`
		} `json:"project"`
	}
	path := "/api/v1/lite/teacher/classes/" + classID + "/students/" + studentID.String() + "/items/" + atomID.String()
	if code := getJSON(t, h, teacher, path, &resp); code != http.StatusOK {
		t.Fatalf("item = %d", code)
	}
	if resp.Project.SiteToken != nil {
		t.Fatalf("siteToken = %q, want nil for an unpublished site", *resp.Project.SiteToken)
	}
}

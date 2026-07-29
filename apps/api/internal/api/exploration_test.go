package api_test

// exploration_test.go — S3 rabbit-hole Task 4: the exploration lead lifecycle
// handlers (create/list+dangling projection/patch(connect·prune·edit)/delete),
// no LLM spend. Mirrors workspace_library_test.go's harness/seed conventions.

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// explorationLeadView is the test's decode shape for the lead wire DTO.
type explorationLeadView struct {
	ID                   string  `json:"id"`
	Text                 string  `json:"text"`
	Status               string  `json:"status"`
	Origin               string  `json:"origin"`
	SourceReferenceID    *string `json:"sourceReferenceId"`
	ConnectedReferenceID *string `json:"connectedReferenceId"`
	Position             int32   `json:"position"`
}

type explorationViewBody struct {
	Leads             []explorationLeadView `json:"leads"`
	DanglingSourceIDs []string               `json:"danglingSourceIds"`
}

// TestExplorationLeadCRUD covers the manual-add → list-projects → connect →
// delete lifecycle end to end over HTTP.
func TestExplorationLeadCRUD(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	// Create a manual lead.
	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"中国的碳排放总量会不会推翻论点？"}`)
	if rec.Code != 201 {
		t.Fatalf("create lead = %d, want 201: %s", rec.Code, rec.Body)
	}
	var created struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created lead: %v — %s", err, rec.Body)
	}
	if created.Lead.Text != "中国的碳排放总量会不会推翻论点？" {
		t.Fatalf("lead text = %q", created.Lead.Text)
	}
	if created.Lead.Origin != "manual" || created.Lead.Status != "open" {
		t.Fatalf("lead origin/status = %q/%q, want manual/open", created.Lead.Origin, created.Lead.Status)
	}
	if created.Lead.SourceReferenceID != nil || created.Lead.ConnectedReferenceID != nil {
		t.Fatalf("manual lead should have no source/connected ref: %+v", created.Lead)
	}
	lid := created.Lead.ID

	// Empty text → 400.
	recBlank := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"   "}`)
	if recBlank.Code != 400 {
		t.Fatalf("create blank lead = %d, want 400: %s", recBlank.Code, recBlank.Body)
	}

	// GET /exploration lists it.
	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	if rec.Code != 200 {
		t.Fatalf("get exploration = %d: %s", rec.Code, rec.Body)
	}
	var view explorationViewBody
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode exploration view: %v — %s", err, rec.Body)
	}
	if len(view.Leads) != 1 || view.Leads[0].ID != lid {
		t.Fatalf("exploration leads = %+v, want [%s]", view.Leads, lid)
	}

	// Seed a reference to connect the lead to.
	rec = doJSON(t, h, cookie, "POST", base+"/references", `{"title":"NASA 卫星数据"}`)
	var refWrap struct {
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)
	rid := refWrap.Reference.ID

	// PATCH to connect.
	rec = doJSON(t, h, cookie, "PATCH", base+"/exploration/leads/"+lid,
		`{"status":"connected","connectedReferenceId":"`+rid+`"}`)
	if rec.Code != 200 {
		t.Fatalf("patch connect = %d: %s", rec.Code, rec.Body)
	}
	var patched struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patched lead: %v — %s", err, rec.Body)
	}
	if patched.Lead.Status != "connected" {
		t.Fatalf("status after connect = %q, want connected", patched.Lead.Status)
	}
	if patched.Lead.ConnectedReferenceID == nil || *patched.Lead.ConnectedReferenceID != rid {
		t.Fatalf("connectedReferenceId = %v, want %q", patched.Lead.ConnectedReferenceID, rid)
	}

	// GET reflects the connection.
	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	if len(view.Leads) != 1 || view.Leads[0].Status != "connected" {
		t.Fatalf("exploration after connect = %+v", view.Leads)
	}

	// Unknown status → 400.
	rec = doJSON(t, h, cookie, "PATCH", base+"/exploration/leads/"+lid, `{"status":"archived"}`)
	if rec.Code != 400 {
		t.Fatalf("patch unknown status = %d, want 400: %s", rec.Code, rec.Body)
	}

	// connectedReferenceId that doesn't resolve in this project → 400.
	rec = doJSON(t, h, cookie, "PATCH", base+"/exploration/leads/"+lid,
		`{"connectedReferenceId":"`+uuid.New().String()+`"}`)
	if rec.Code != 400 {
		t.Fatalf("patch bad connectedReferenceId = %d, want 400: %s", rec.Code, rec.Body)
	}

	// DELETE removes it.
	rec = doJSON(t, h, cookie, "DELETE", base+"/exploration/leads/"+lid, "")
	if rec.Code != 204 {
		t.Fatalf("delete lead = %d, want 204: %s", rec.Code, rec.Body)
	}
	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	if len(view.Leads) != 0 {
		t.Fatalf("leads after delete = %+v, want none", view.Leads)
	}
}

// TestExplorationLead_PartialPatchNoClobber locks in the S2 "full-replace data
// loss" bug class for PATCH lead: a partial JSON body must merge onto the
// existing row, never wipe fields the caller didn't mention. Absent field ≠
// clear; only an explicit JSON null clears.
func TestExplorationLead_PartialPatchNoClobber(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	// Seed a lead.
	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"这条论证站得住脚吗？"}`)
	if rec.Code != 201 {
		t.Fatalf("create lead = %d, want 201: %s", rec.Code, rec.Body)
	}
	var created struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created lead: %v — %s", err, rec.Body)
	}
	lid := created.Lead.ID

	// Seed a reference to connect it to.
	rec = doJSON(t, h, cookie, "POST", base+"/references", `{"title":"Nature Sustainability 论文"}`)
	var refWrap struct {
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &refWrap); err != nil {
		t.Fatalf("decode created reference: %v — %s", err, rec.Body)
	}
	rid := refWrap.Reference.ID

	// 1) PATCH both fields to connect the lead.
	rec = doJSON(t, h, cookie, "PATCH", base+"/exploration/leads/"+lid,
		`{"status":"connected","connectedReferenceId":"`+rid+`"}`)
	if rec.Code != 200 {
		t.Fatalf("patch connect = %d: %s", rec.Code, rec.Body)
	}
	var patched struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patched lead: %v — %s", err, rec.Body)
	}
	if patched.Lead.Status != "connected" {
		t.Fatalf("status after connect = %q, want connected", patched.Lead.Status)
	}
	if patched.Lead.ConnectedReferenceID == nil || *patched.Lead.ConnectedReferenceID != rid {
		t.Fatalf("connectedReferenceId after connect = %v, want %q", patched.Lead.ConnectedReferenceID, rid)
	}

	// 2) PATCH with ONLY status (connectedReferenceId field absent from the
	// JSON body entirely) must NOT clobber text or the existing connection.
	rec = doJSON(t, h, cookie, "PATCH", base+"/exploration/leads/"+lid, `{"status":"open"}`)
	if rec.Code != 200 {
		t.Fatalf("patch status-only = %d: %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode status-only patched lead: %v — %s", err, rec.Body)
	}
	if patched.Lead.Status != "open" {
		t.Fatalf("status after status-only patch = %q, want open", patched.Lead.Status)
	}
	if patched.Lead.Text != "这条论证站得住脚吗？" {
		t.Fatalf("text after status-only patch = %q, want unchanged", patched.Lead.Text)
	}
	if patched.Lead.ConnectedReferenceID == nil || *patched.Lead.ConnectedReferenceID != rid {
		t.Fatalf("connectedReferenceId after status-only patch = %v, want preserved %q (field-absent must not wipe)", patched.Lead.ConnectedReferenceID, rid)
	}

	// 3) PATCH with ONLY an explicit connectedReferenceId:null must disconnect
	// the reference without touching status (absent from this body).
	rec = doJSON(t, h, cookie, "PATCH", base+"/exploration/leads/"+lid, `{"connectedReferenceId":null}`)
	if rec.Code != 200 {
		t.Fatalf("patch disconnect-only = %d: %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode disconnect-only patched lead: %v — %s", err, rec.Body)
	}
	if patched.Lead.ConnectedReferenceID != nil {
		t.Fatalf("connectedReferenceId after explicit-null patch = %v, want nil", patched.Lead.ConnectedReferenceID)
	}
	if patched.Lead.Status != "open" {
		t.Fatalf("status after disconnect-only patch = %q, want preserved open (status was absent from this body)", patched.Lead.Status)
	}
}

// TestExploration_IDOR — a lead in project A must not be GET/PATCH/DELETE-able
// via project B (404 in all cases, no mutation leaks across the boundary).
func TestExploration_IDOR(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	owner := signInSeed(t, pool)
	pidA := createProjectForTest(t, h, owner)
	baseA := "/api/v1/projects/" + pidA

	rec := doJSON(t, h, owner, "POST", baseA+"/exploration/leads", `{"text":"A 项目的线索"}`)
	var created struct {
		Lead explorationLeadView `json:"lead"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	lid := created.Lead.ID

	otherID := createStudent(t, pool, SeedSchoolID, "exploration-idor@demo.local")
	other := signInAs(t, pool, otherID)
	pidB := createProjectForTest(t, h, other)
	baseB := "/api/v1/projects/" + pidB

	for _, tc := range []struct{ method, path string }{
		{"PATCH", baseB + "/exploration/leads/" + lid},
		{"DELETE", baseB + "/exploration/leads/" + lid},
	} {
		rec := doJSON(t, h, other, tc.method, tc.path, `{"status":"pruned"}`)
		if rec.Code != 404 {
			t.Errorf("%s %s cross-project = %d, want 404: %s", tc.method, tc.path, rec.Code, rec.Body)
		}
	}

	// The lead survives untouched in project A.
	rec = doJSON(t, h, owner, "GET", baseA+"/exploration", "")
	var view explorationViewBody
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	if len(view.Leads) != 1 || view.Leads[0].Status != "open" {
		t.Fatalf("project A lead after cross-project attempts = %+v, want untouched open", view.Leads)
	}
}

// TestComputeDanglingSourceIds is a pure unit test of the exported test seam
// wrapping computeDanglingSourceIds: a reference is dangling iff its
// material_id is set AND its decision is null/"drop" AND no non-pruned lead
// connects it. Pure function — struct literals directly, no DB round-trip.
func TestComputeDanglingSourceIds(t *testing.T) {
	projectID := uuid.New()
	decision := "drop"
	useDecision := "use"

	// A reference with material_id + decision=drop, no connecting lead → dangling.
	dropped := sqlc.Reference{
		ID: uuid.New(), ProjectID: projectID, Title: "读过但决定弃用的来源",
		MaterialID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Decision: &decision,
	}

	// A reference with decision=use, engaged → never dangling regardless of leads.
	used := sqlc.Reference{
		ID: uuid.New(), ProjectID: projectID, Title: "被采用的来源",
		MaterialID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Decision: &useDecision,
	}

	// A reference with no material_id (未读) → never dangling.
	unread := sqlc.Reference{ID: uuid.New(), ProjectID: projectID, Title: "还没读的来源"}

	refs := []sqlc.Reference{dropped, used, unread}

	// No leads at all: the dropped reference is dangling.
	dangling := ComputeDanglingSourceIdsForTest(refs, nil)
	if !containsID(dangling, dropped.ID.String()) {
		t.Fatalf("dangling = %v, want it to contain dropped reference %s", dangling, dropped.ID)
	}
	if containsID(dangling, used.ID.String()) {
		t.Fatalf("dangling = %v, used reference must never be dangling", dangling)
	}
	if containsID(dangling, unread.ID.String()) {
		t.Fatalf("dangling = %v, unread (no material) reference must never be dangling", dangling)
	}

	// Once a non-pruned lead connects the dropped reference, it's no longer dangling.
	connectedRef := pgtype.UUID{Bytes: dropped.ID, Valid: true}
	leads := []sqlc.ExplorationLead{
		{ID: uuid.New(), ProjectID: projectID, Text: "追问", Status: "open", Origin: "manual", ConnectedReferenceID: connectedRef},
	}
	dangling = ComputeDanglingSourceIdsForTest(refs, leads)
	if containsID(dangling, dropped.ID.String()) {
		t.Fatalf("dangling = %v, connected reference must no longer be dangling", dangling)
	}

	// A pruned lead connecting it does NOT count — still dangling.
	prunedLeads := []sqlc.ExplorationLead{
		{ID: uuid.New(), ProjectID: projectID, Text: "追问", Status: "pruned", Origin: "manual", ConnectedReferenceID: connectedRef},
	}
	dangling = ComputeDanglingSourceIdsForTest(refs, prunedLeads)
	if !containsID(dangling, dropped.ID.String()) {
		t.Fatalf("dangling = %v, a pruned-lead connection must not un-dangle the source", dangling)
	}
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

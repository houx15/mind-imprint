package api_test

// exploration_test.go — S3 rabbit-hole Task 4: the exploration lead lifecycle
// handlers (create/list+dangling projection/patch(connect·prune·edit)/delete),
// no LLM spend. Mirrors workspace_library_test.go's harness/seed conventions.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/materialize"
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
	ParentLeadID         *string `json:"parentLeadId"`
}

// #12 · a 线索 can hang 分支 (child leads) under it via parentLeadId; a foreign
// parent is rejected (IDOR).
func TestExplorationLead_Branch(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"父线索"}`)
	var parent struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parent); err != nil {
		t.Fatalf("decode parent: %v — %s", err, rec.Body)
	}
	if parent.Lead.ParentLeadID != nil {
		t.Fatalf("top-level lead should have nil parentLeadId: %+v", parent.Lead)
	}
	parentID := parent.Lead.ID

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"子分支","parentLeadId":"`+parentID+`"}`)
	if rec.Code != 201 {
		t.Fatalf("create branch = %d, want 201: %s", rec.Code, rec.Body)
	}
	var child struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &child); err != nil {
		t.Fatalf("decode child: %v — %s", err, rec.Body)
	}
	if child.Lead.ParentLeadID == nil || *child.Lead.ParentLeadID != parentID {
		t.Fatalf("branch parentLeadId = %v, want %s", child.Lead.ParentLeadID, parentID)
	}

	// A parent that isn't a lead in this project → 400 (IDOR guard).
	recBad := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"x","parentLeadId":"`+uuid.NewString()+`"}`)
	if recBad.Code != 400 {
		t.Fatalf("foreign parent = %d, want 400: %s", recBad.Code, recBad.Body)
	}

	// GET reflects both leads, the child carrying its parent.
	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	var view explorationViewBody
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode view: %v — %s", err, rec.Body)
	}
	if len(view.Leads) != 2 {
		t.Fatalf("want 2 leads, got %d: %+v", len(view.Leads), view.Leads)
	}
}

// TestCreateLead_FromNoteSetsOrigin is Task C1: promoting a reading note into
// a question. A lead created with a sourceReferenceId (a reference that
// really belongs to this project) gets origin "note" and carries that
// sourceReferenceId — distinct from the plain manual-add path which leaves
// both origin "manual" and sourceReferenceId nil.
func TestCreateLead_FromNoteSetsOrigin(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"NASA 卫星数据"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create reference = %d, want 201: %s", rec.Code, rec.Body)
	}
	var refWrap struct {
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &refWrap); err != nil {
		t.Fatalf("decode reference: %v — %s", err, rec.Body)
	}
	rid := refWrap.Reference.ID

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/leads",
		`{"text":"这条笔记里的说法站得住吗？","sourceReferenceId":"`+rid+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create lead from note = %d, want 201: %s", rec.Code, rec.Body)
	}
	var created struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created lead: %v — %s", err, rec.Body)
	}
	if created.Lead.Origin != "note" {
		t.Fatalf("origin = %q, want note", created.Lead.Origin)
	}
	if created.Lead.SourceReferenceID == nil || *created.Lead.SourceReferenceID != rid {
		t.Fatalf("sourceReferenceId = %v, want %q", created.Lead.SourceReferenceID, rid)
	}
}

// TestCreateLead_ForeignSourceReference_400 pins the IDOR guard on the new
// sourceReferenceId param: a reference id that's real, but lives in a
// DIFFERENT project, must be rejected rather than silently attached (the same
// convention parentLeadId already follows above).
func TestCreateLead_ForeignSourceReference_400(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	otherPID := createProjectForTest(t, h, cookie)

	rec := doJSON(t, h, cookie, "POST", "/api/v1/projects/"+otherPID+"/references", `{"title":"别处的来源"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create foreign reference = %d, want 201: %s", rec.Code, rec.Body)
	}
	var refWrap struct {
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &refWrap); err != nil {
		t.Fatalf("decode foreign reference: %v — %s", err, rec.Body)
	}
	foreignRid := refWrap.Reference.ID

	rec = doJSON(t, h, cookie, "POST", "/api/v1/projects/"+pid+"/exploration/leads",
		`{"text":"x","sourceReferenceId":"`+foreignRid+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create lead with foreign sourceReferenceId = %d, want 400: %s", rec.Code, rec.Body)
	}
}

type explorationViewBody struct {
	Leads             []explorationLeadView `json:"leads"`
	DanglingSourceIDs []string              `json:"danglingSourceIds"`
	Edges             []questionEdgeView    `json:"edges"`
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
// connects it as EITHER the connected_reference_id OR the
// source_reference_id (a source that spawned branches is used, not
// dangling). Pure function — struct literals directly, no DB round-trip.
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

	// The NASA-source happy path: finalized takeaway, material engaged,
	// decision still unset (nil, neither "use" nor "drop") — but it spawned
	// a takeaway lead. Must NOT be dangling despite having no decision yet.
	spawner := sqlc.Reference{
		ID: uuid.New(), ProjectID: projectID, Title: "牵出了新线索的来源",
		MaterialID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
	}

	refs := []sqlc.Reference{dropped, used, unread, spawner}

	// No leads at all: the dropped reference is dangling; spawner (no leads
	// yet) is dangling too — spawning requires an actual lead.
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
	if !containsID(dangling, spawner.ID.String()) {
		t.Fatalf("dangling = %v, want it to contain spawner (no leads yet) %s", dangling, spawner.ID)
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

	// A source that SPAWNED an open takeaway lead (source_reference_id, not
	// connected_reference_id) is not dangling, even with material engaged and
	// decision null — Finding 1's bug case.
	spawnedRef := pgtype.UUID{Bytes: spawner.ID, Valid: true}
	spawnLeads := []sqlc.ExplorationLead{
		{ID: uuid.New(), ProjectID: projectID, Text: "新线索：中国碳排放全球第一", Status: "open", Origin: "takeaway", SourceReferenceID: spawnedRef},
	}
	dangling = ComputeDanglingSourceIdsForTest(refs, spawnLeads)
	if containsID(dangling, spawner.ID.String()) {
		t.Fatalf("dangling = %v, a source that spawned an open lead must not be dangling", dangling)
	}

	// Once that spawned lead is pruned, the source falls back to dangling —
	// every branch it produced was abandoned, same as never branching.
	spawnLeadsPruned := []sqlc.ExplorationLead{
		{ID: uuid.New(), ProjectID: projectID, Text: "新线索：中国碳排放全球第一", Status: "pruned", Origin: "takeaway", SourceReferenceID: spawnedRef},
	}
	dangling = ComputeDanglingSourceIdsForTest(refs, spawnLeadsPruned)
	if !containsID(dangling, spawner.ID.String()) {
		t.Fatalf("dangling = %v, want spawner dangling again once its only spawned lead is pruned", dangling)
	}
}

// digCandidatesView is the test's decode shape for POST .../exploration/dig.
type digCandidatesView struct {
	Candidates []struct {
		DOI      string `json:"doi"`
		Title    string `json:"title"`
		Authors  string `json:"authors"`
		Year     string `json:"year"`
		Journal  string `json:"journal"`
		Abstract string `json:"abstract"`
		URL      string `json:"url"`
	} `json:"candidates"`
}

// TestExplorationDig_KeywordReturnsCandidates — #A3: a free-text keyword digs
// OpenAlex (via the Fetcher seam, faked here) and returns candidates straight
// to the client tray. Not persisted (no DB row asserted), not metered (no LLM
// call — OpenAlex isn't an LLM, so there is no llm_call assertion here at all).
func TestExplorationDig_KeywordReturnsCandidates(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID,
		Fetcher: fakeFetcher{works: []materialize.WorkMeta{{DOI: "10.1/x", Title: "T", Year: "2023"}}},
	}).Handler()
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/dig", `{"keyword":"china carbon"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("dig = %d, want 200: %s", rec.Code, rec.Body)
	}
	var out digCandidatesView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode dig response: %v — %s", err, rec.Body)
	}
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want 1", out.Candidates)
	}
	if out.Candidates[0].Title != "T" || out.Candidates[0].DOI != "10.1/x" || out.Candidates[0].Year != "2023" {
		t.Fatalf("unexpected candidate: %+v", out.Candidates[0])
	}
}

// TestExplorationDig_RefinesQueryBeforeSearch — Task A8: with a provider
// configured, 印记 refines the raw (Chinese, sentence-shaped) question into a
// short English keyword query BEFORE it reaches OpenAlex, and that refine is
// metered as one llm_call with purpose "dig_query" (a real LLM call, unlike
// the OpenAlex fetch itself).
func TestExplorationDig_RefinesQueryBeforeSearch(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	var captured string
	h := New(Deps{
		Queries: q, Pool: pool, SpecByID: cards.ByID,
		Provider:     readingStubProvider("China greening carbon accounting attribution"),
		ChatResolver: fakeResolver(),
		Fetcher: fakeFetcher{
			works:     []materialize.WorkMeta{{DOI: "10.1/y", Title: "T2", Year: "2024"}},
			lastQuery: &captured,
		},
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/dig", `{"keyword":"中国单独 vs 中印合计的口径差异"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("dig = %d, want 200: %s", rec.Code, rec.Body)
	}
	if captured != "China greening carbon accounting attribution" {
		t.Fatalf("SearchWorks query = %q, want the refined English query", captured)
	}
	var out digCandidatesView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode dig response: %v — %s", err, rec.Body)
	}
	if len(out.Candidates) != 1 || out.Candidates[0].Title != "T2" {
		t.Fatalf("unexpected candidates: %+v", out.Candidates)
	}
	if n := countLLMCallsByPurpose(t, pool, pid, "dig_query"); n != 1 {
		t.Fatalf("want 1 dig_query llm_call, got %d", n)
	}
}

// TestExplorationDig_NoProviderFallsBackToRawQuery — the no-provider path
// (mirrors TestExplorationDig_KeywordReturnsCandidates but pins the fallback
// explicitly): with no ChatResolver configured, the refine block must be
// skipped entirely — the raw keyword reaches SearchWorks unchanged, and
// nothing is metered.
func TestExplorationDig_NoProviderFallsBackToRawQuery(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	var captured string
	h := New(Deps{
		Queries: q, Pool: pool, SpecByID: cards.ByID,
		Fetcher: fakeFetcher{
			works:     []materialize.WorkMeta{{DOI: "10.1/x", Title: "T", Year: "2023"}},
			lastQuery: &captured,
		},
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/dig", `{"keyword":"china carbon"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("dig = %d, want 200: %s", rec.Code, rec.Body)
	}
	if captured != "china carbon" {
		t.Fatalf("SearchWorks query = %q, want the raw keyword (no provider configured)", captured)
	}
	if n := countLLMCallsByPurpose(t, pool, pid, "dig_query"); n != 0 {
		t.Fatalf("want 0 dig_query llm_call with no provider configured, got %d", n)
	}
}

// TestExplorationDig_CitationMode — GVe: mode:"citation" resolves the paper
// lead's DOI (via connectedReferenceId -> reference.url, a doi.org link) and
// routes to Fetcher.ReferencedWorks instead of the keyword/SearchWorks path —
// no query refine, no LLM call at all (unlike "similar").
func TestExplorationDig_CitationMode(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)
	var captured string
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID,
		Fetcher: fakeFetcher{
			works:     []materialize.WorkMeta{{DOI: "10.9/ref", Title: "Cited Source", Year: "2021"}},
			lastQuery: &captured,
		},
	}).Handler()
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	// Seed a reference with a doi.org URL + a lead connected to it — adopt is
	// the one call that creates both rows atomically (mirrors
	// TestExplorationAdopt_CreatesReferenceAndLead's seeding, reused here
	// rather than re-deriving the same DB writes by hand).
	rec := doJSON(t, h, cookie, "POST", base+"/exploration/adopt",
		`{"candidate":{"doi":"10.1000/x","title":"论文源","authors":"","year":"2023","journal":"","abstract":"","url":""}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("adopt = %d, want 201: %s", rec.Code, rec.Body)
	}
	var adopted adoptView
	if err := json.Unmarshal(rec.Body.Bytes(), &adopted); err != nil {
		t.Fatalf("decode adopt response: %v — %s", err, rec.Body)
	}
	if adopted.Reference.URL != "https://doi.org/10.1000/x" {
		t.Fatalf("reference.url = %q, want the derived doi.org link", adopted.Reference.URL)
	}
	leadID := adopted.Lead.ID

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/dig", `{"leadId":"`+leadID+`","mode":"citation"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("dig citation = %d, want 200: %s", rec.Code, rec.Body)
	}
	if captured != "10.1000/x" {
		t.Fatalf("ReferencedWorks doi = %q, want the resolved doi 10.1000/x", captured)
	}
	var out digCandidatesView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode dig response: %v — %s", err, rec.Body)
	}
	if len(out.Candidates) != 1 || out.Candidates[0].Title != "Cited Source" {
		t.Fatalf("unexpected candidates: %+v", out.Candidates)
	}
	// citation mode does no query refine — no LLM call, unlike "similar".
	if n := countLLMCallsByPurpose(t, pool, pid, "dig_query"); n != 0 {
		t.Fatalf("want 0 dig_query llm_call for citation mode, got %d", n)
	}
}

// TestExplorationDig_CitationMode_NoDOIReturnsEmpty — citation/cited modes
// are best-effort: a lead with no connected reference (or a reference whose
// URL carries no DOI) degrades to an empty tray, never a 4xx.
func TestExplorationDig_CitationMode_NoDOIReturnsEmpty(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID,
		Fetcher: fakeFetcher{works: []materialize.WorkMeta{{DOI: "10.9/ref", Title: "Should not appear"}}},
	}).Handler()
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"没有连来源的问题"}`)
	var lead struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &lead); err != nil {
		t.Fatalf("decode lead: %v — %s", err, rec.Body)
	}

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/dig", `{"leadId":"`+lead.Lead.ID+`","mode":"cited"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("dig cited (no doi) = %d, want 200: %s", rec.Code, rec.Body)
	}
	var out digCandidatesView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode dig response: %v — %s", err, rec.Body)
	}
	if len(out.Candidates) != 0 {
		t.Fatalf("candidates = %+v, want empty (no DOI to resolve)", out.Candidates)
	}
}

// adoptView is the test's decode shape for POST .../exploration/adopt.
type adoptView struct {
	Lead      explorationLeadView `json:"lead"`
	Reference referenceView       `json:"reference"`
}

// TestExplorationAdopt_CreatesReferenceAndLead — Task A4: adopting a tray
// candidate is the ONLY way a dig result becomes a node on the map (铁律①:
// student confirms). One transaction must create BOTH the reference
// (auto-shelved reading_status="reading") and the lead that connects to it
// (origin="guide", connectedReferenceId=the new reference, parentLeadId=the
// dug node) — the map should never show a floating half-created row.
func TestExplorationAdopt_CreatesReferenceAndLead(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: fakeFetcher{}}).Handler()
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	// A parent lead to dig from — mirrors the tray's "从这条线索深挖" origin.
	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"中国的碳排放到底有多少"}`)
	var parent struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parent); err != nil {
		t.Fatalf("decode parent: %v — %s", err, rec.Body)
	}
	parentID := parent.Lead.ID

	body := `{"parentLeadId":"` + parentID + `","candidate":{"doi":"10.1/x","title":"Nature Sust 2023","authors":"Li","year":"2023","journal":"Nature Sustainability","abstract":"...","url":"https://n/x"}}`
	rec = doJSON(t, h, cookie, "POST", base+"/exploration/adopt", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("adopt = %d, want 201: %s", rec.Code, rec.Body)
	}
	var out adoptView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode adopt response: %v — %s", err, rec.Body)
	}
	if out.Reference.ReadingStatus != "reading" {
		t.Fatalf("reference.readingStatus = %q, want reading", out.Reference.ReadingStatus)
	}
	if out.Reference.Title != "Nature Sust 2023" {
		t.Fatalf("reference.title = %q, want Nature Sust 2023", out.Reference.Title)
	}
	if out.Reference.Author != "Li" || out.Reference.Year != "2023" || out.Reference.Journal != "Nature Sustainability" {
		t.Fatalf("reference meta not filled from candidate: %+v", out.Reference)
	}
	if out.Lead.Origin != "guide" {
		t.Fatalf("lead.origin = %q, want guide", out.Lead.Origin)
	}
	if out.Lead.Status != "connected" {
		t.Fatalf("lead.status = %q, want connected", out.Lead.Status)
	}
	if out.Lead.ConnectedReferenceID == nil || *out.Lead.ConnectedReferenceID != out.Reference.ID {
		t.Fatalf("lead.connectedReferenceId = %v, want %s", out.Lead.ConnectedReferenceID, out.Reference.ID)
	}
	if out.Lead.ParentLeadID == nil || *out.Lead.ParentLeadID != parentID {
		t.Fatalf("lead.parentLeadId = %v, want %s", out.Lead.ParentLeadID, parentID)
	}

	// GET /exploration reflects the newly connected lead — both rows really
	// landed, not just echoed in the response.
	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	var view explorationViewBody
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode view: %v — %s", err, rec.Body)
	}
	found := false
	for _, l := range view.Leads {
		if l.ID == out.Lead.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("adopted lead not present in GET /exploration: %+v", view.Leads)
	}
}

// An empty candidate title is rejected before any DB write (400) — the
// tray must always carry at least a title from OpenAlex/materialize.
func TestExplorationAdopt_RejectsEmptyTitle(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: fakeFetcher{}}).Handler()
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/adopt", `{"candidate":{"doi":"10.1/x"}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("adopt empty title = %d, want 400: %s", rec.Code, rec.Body)
	}
}

// A parentLeadId that isn't a lead in this project is rejected (IDOR guard),
// mirroring createExplorationLead's own parentLeadId check.
func TestExplorationAdopt_RejectsForeignParentLead(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: fakeFetcher{}}).Handler()
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	body := `{"parentLeadId":"` + uuid.NewString() + `","candidate":{"title":"T"}}`
	rec := doJSON(t, h, cookie, "POST", base+"/exploration/adopt", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("adopt foreign parent = %d, want 400: %s", rec.Code, rec.Body)
	}
}

// TestQuestionEdge_CRUD — B2: the edge lifecycle endpoints (create/relabel/
// confirm/dismiss). Two root leads, POST an edge (student-created →
// confirmed), GET reflects it, PATCH relabels + (re)confirms, DELETE removes
// it. Also pins the carry-forward IDOR guard (CreateQuestionEdge itself does
// NOT check project ownership of from/to lead ids — the handler must) plus
// the root-only and closed-label-set validations.
func TestQuestionEdge_CRUD(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"中国的碳排放会不会推翻论点？"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create lead A = %d: %s", rec.Code, rec.Body)
	}
	var leadA struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &leadA); err != nil {
		t.Fatalf("decode lead A: %v — %s", err, rec.Body)
	}

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"可再生能源占比是否足以抵消？"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create lead B = %d: %s", rec.Code, rec.Body)
	}
	var leadB struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &leadB); err != nil {
		t.Fatalf("decode lead B: %v — %s", err, rec.Body)
	}

	// POST an edge between the two roots → 201, student-created so confirmed
	// (not proposed).
	rec = doJSON(t, h, cookie, "POST", base+"/exploration/edges",
		`{"fromLeadId":"`+leadA.Lead.ID+`","toLeadId":"`+leadB.Lead.ID+`","label":"支持"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create edge = %d, want 201: %s", rec.Code, rec.Body)
	}
	var created struct {
		Edge questionEdgeView `json:"edge"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created edge: %v — %s", err, rec.Body)
	}
	if created.Edge.FromLeadID != leadA.Lead.ID || created.Edge.ToLeadID != leadB.Lead.ID {
		t.Fatalf("edge from/to = %q/%q, want %q/%q", created.Edge.FromLeadID, created.Edge.ToLeadID, leadA.Lead.ID, leadB.Lead.ID)
	}
	if created.Edge.Label != "支持" {
		t.Fatalf("edge label = %q, want 支持", created.Edge.Label)
	}
	if created.Edge.Status != "confirmed" {
		t.Fatalf("student-created edge status = %q, want confirmed", created.Edge.Status)
	}
	eid := created.Edge.ID

	// GET /exploration shows it.
	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	var view explorationViewBody
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode view: %v — %s", err, rec.Body)
	}
	if len(view.Edges) != 1 || view.Edges[0].ID != eid {
		t.Fatalf("exploration edges = %+v, want [%s]", view.Edges, eid)
	}

	// PATCH relabels (and would re-confirm a proposed edge; here already
	// confirmed, so this also pins that re-confirming is idempotent).
	rec = doJSON(t, h, cookie, "PATCH", base+"/exploration/edges/"+eid, `{"label":"反驳/张力","status":"confirmed"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch edge = %d, want 200: %s", rec.Code, rec.Body)
	}
	var patched struct {
		Edge questionEdgeView `json:"edge"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patched edge: %v — %s", err, rec.Body)
	}
	if patched.Edge.Label != "反驳/张力" {
		t.Fatalf("edge label after patch = %q, want 反驳/张力", patched.Edge.Label)
	}
	if patched.Edge.Status != "confirmed" {
		t.Fatalf("edge status after patch = %q, want confirmed", patched.Edge.Status)
	}

	// DELETE dismisses it.
	rec = doJSON(t, h, cookie, "DELETE", base+"/exploration/edges/"+eid, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete edge = %d, want 204: %s", rec.Code, rec.Body)
	}
	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode view after delete: %v — %s", err, rec.Body)
	}
	if len(view.Edges) != 0 {
		t.Fatalf("edges after delete = %+v, want none", view.Edges)
	}

	// -- validation / IDOR cases --------------------------------------------

	// A foreign-project lead as fromLeadId → 400 (closes the CreateQuestionEdge
	// IDOR gap: the sqlc query itself doesn't check project ownership).
	otherID := createStudent(t, pool, SeedSchoolID, "question-edge-idor@demo.local")
	other := signInAs(t, pool, otherID)
	pidOther := createProjectForTest(t, h, other)
	recOtherLead := doJSON(t, h, other, "POST", "/api/v1/projects/"+pidOther+"/exploration/leads", `{"text":"别的项目的线索"}`)
	var otherLead struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(recOtherLead.Body.Bytes(), &otherLead); err != nil {
		t.Fatalf("decode other-project lead: %v — %s", err, recOtherLead.Body)
	}
	recForeign := doJSON(t, h, cookie, "POST", base+"/exploration/edges",
		`{"fromLeadId":"`+otherLead.Lead.ID+`","toLeadId":"`+leadB.Lead.ID+`","label":"支持"}`)
	if recForeign.Code != http.StatusBadRequest {
		t.Fatalf("edge with foreign-project lead = %d, want 400: %s", recForeign.Code, recForeign.Body)
	}

	// A non-root (child) lead as toLeadId → 400 (edges connect top-level
	// question nodes only).
	recChild := doJSON(t, h, cookie, "POST", base+"/exploration/leads",
		`{"text":"子分支","parentLeadId":"`+leadA.Lead.ID+`"}`)
	var child struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(recChild.Body.Bytes(), &child); err != nil {
		t.Fatalf("decode child lead: %v — %s", err, recChild.Body)
	}
	recNonRoot := doJSON(t, h, cookie, "POST", base+"/exploration/edges",
		`{"fromLeadId":"`+leadA.Lead.ID+`","toLeadId":"`+child.Lead.ID+`","label":"支持"}`)
	if recNonRoot.Code != http.StatusBadRequest {
		t.Fatalf("edge with non-root lead = %d, want 400: %s", recNonRoot.Code, recNonRoot.Body)
	}

	// A label outside the closed 5-value set → 400.
	recBadLabel := doJSON(t, h, cookie, "POST", base+"/exploration/edges",
		`{"fromLeadId":"`+leadA.Lead.ID+`","toLeadId":"`+leadB.Lead.ID+`","label":"随便什么"}`)
	if recBadLabel.Code != http.StatusBadRequest {
		t.Fatalf("edge with bad label = %d, want 400: %s", recBadLabel.Code, recBadLabel.Body)
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

// explorationGuideView is the test's decode shape for POST .../exploration/guide.
type explorationGuideView struct {
	Directions []struct {
		Direction string `json:"direction"`
		Why       string `json:"why"`
	} `json:"directions"`
}

// TestExplorationGuide_ReturnsDirections — Task 6: the guide assembles the
// project's exploration graph (here, one engaged reference) and runs ONE
// isolated mid-tier compose (agent.ComposeExplorationGuide) to point at the
// next necessary direction. Exactly one llm_call row with purpose
// "exploration_guide" is metered for the call that actually happened.
func TestExplorationGuide_ReturnsDirections(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	guideReply := `{"directions":[{"direction":"找中国碳排放绝对量的一手数据","why":"你缺反例检验的来源"}]}`
	h := New(Deps{
		Queries: q, Pool: pool,
		Provider: readingStubProvider(guideReply), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"NASA 卫星数据"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed reference = %d: %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/guide", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("guide = %d, want 200: %s", rec.Code, rec.Body)
	}
	var out explorationGuideView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode guide response: %v — %s", err, rec.Body)
	}
	if len(out.Directions) != 1 {
		t.Fatalf("directions = %+v, want 1 (the stub's)", out.Directions)
	}
	if out.Directions[0].Direction != "找中国碳排放绝对量的一手数据" || out.Directions[0].Why != "你缺反例检验的来源" {
		t.Fatalf("unexpected direction: %+v", out.Directions[0])
	}

	if n := countLLMCallsByPurpose(t, pool, pid, "exploration_guide"); n != 1 {
		t.Fatalf("want 1 exploration_guide llm_call, got %d", n)
	}
}

// TestExplorationGuide_EmptyGraphNoSpend — the no-spend guarantee (克制):
// a fresh project with no references and no leads has nothing to point a
// direction from (agent.HasGraphContent false), so the guide must return
// 200 with empty directions WITHOUT ever resolving a provider or metering a
// call — mirrors TestGetTakeawayDraft_EmptyRecordNoSpend's regression pin.
func TestExplorationGuide_EmptyGraphNoSpend(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool,
		Provider: readingStubProvider(`{"directions":[]}`), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid
	// Deliberately no references, no leads created for this project.

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/guide", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("guide = %d, want 200: %s", rec.Code, rec.Body)
	}
	var out explorationGuideView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode guide response: %v — %s", err, rec.Body)
	}
	if len(out.Directions) != 0 {
		t.Fatalf("directions = %+v, want empty for an empty graph", out.Directions)
	}

	// The regression pin: an empty graph must never touch the resolver/
	// compose/meter block, so NO llm_call row is written for this purpose.
	if n := countLLMCallsByPurpose(t, pool, pid, "exploration_guide"); n != 0 {
		t.Fatalf("want 0 exploration_guide llm_call for an empty graph, got %d", n)
	}
}

// TestExplorationGuide_ComposeFailureNoSpend — Task 6 review fix (metering
// precision): a NON-EMPTY graph (one engaged reference, so HasGraphContent
// is true and the resolve+compose path is actually taken — unlike
// EmptyGraphNoSpend, which never reaches the resolver at all) whose compose
// call fails (the stub replies with non-JSON, so ComposeExplorationGuide's
// json.Unmarshal errors and cerr != nil) must still return 200 with empty
// directions, but must NOT record an llm_call row — an llm_call row
// represents a COMPLETED call, and resolved.Provider != "" alone (the
// resolver succeeding) is not proof of that. Contrast
// TestExplorationGuide_ReturnsDirections, which pins that a SUCCESSFUL
// compose records exactly one row.
func TestExplorationGuide_ComposeFailureNoSpend(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool,
		// Not valid JSON — ComposeExplorationGuide's stripFences+Unmarshal
		// will fail, giving cerr != nil despite resolved.Provider != "".
		Provider: readingStubProvider("not json"), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"NASA 卫星数据"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed reference = %d: %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/guide", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("guide = %d, want 200 even on compose failure: %s", rec.Code, rec.Body)
	}
	var out explorationGuideView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode guide response: %v — %s", err, rec.Body)
	}
	if len(out.Directions) != 0 {
		t.Fatalf("directions = %+v, want empty when compose fails", out.Directions)
	}

	// The fix's regression pin: a resolved provider with a FAILED compose
	// must not phantom-record a 0-token/$0 llm_call row.
	if n := countLLMCallsByPurpose(t, pool, pid, "exploration_guide"); n != 0 {
		t.Fatalf("want 0 exploration_guide llm_call on compose failure, got %d", n)
	}
}

// questionEdgeView is the test's decode shape for the question_edge wire DTO
// (B1: the two-level exploration graph's edge foundation).
type questionEdgeView struct {
	ID         string `json:"id"`
	FromLeadID string `json:"fromLeadId"`
	ToLeadID   string `json:"toLeadId"`
	Label      string `json:"label"`
	Status     string `json:"status"`
}

// TestExploration_IncludesEdges seeds two top-level leads, inserts a
// question_edge directly via the store (B2 adds the POST endpoint; this task
// only needs the table + GET projection), then asserts GET /exploration
// returns the edge in its `edges` array with the right label + endpoints.
func TestExploration_IncludesEdges(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"中国的碳排放会不会推翻论点？"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create lead A = %d: %s", rec.Code, rec.Body)
	}
	var leadA struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &leadA); err != nil {
		t.Fatalf("decode lead A: %v — %s", err, rec.Body)
	}

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"可再生能源占比是否足以抵消？"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create lead B = %d: %s", rec.Code, rec.Body)
	}
	var leadB struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &leadB); err != nil {
		t.Fatalf("decode lead B: %v — %s", err, rec.Body)
	}

	q := sqlc.New(pool)
	projectID, err := uuid.Parse(pid)
	if err != nil {
		t.Fatalf("parse project id: %v", err)
	}
	fromID, err := uuid.Parse(leadA.Lead.ID)
	if err != nil {
		t.Fatalf("parse lead A id: %v", err)
	}
	toID, err := uuid.Parse(leadB.Lead.ID)
	if err != nil {
		t.Fatalf("parse lead B id: %v", err)
	}
	edge, err := q.CreateQuestionEdge(context.Background(), sqlc.CreateQuestionEdgeParams{
		ProjectID:  projectID,
		FromLeadID: fromID,
		ToLeadID:   toID,
		Label:      "反驳/张力",
		Status:     "confirmed",
	})
	if err != nil {
		t.Fatalf("CreateQuestionEdge: %v", err)
	}

	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /exploration = %d: %s", rec.Code, rec.Body)
	}
	var view explorationViewBody
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode view: %v — %s", err, rec.Body)
	}
	if len(view.Edges) != 1 {
		t.Fatalf("want 1 edge, got %d: %+v", len(view.Edges), view.Edges)
	}
	got := view.Edges[0]
	if got.ID != edge.ID.String() {
		t.Fatalf("edge id = %q, want %q", got.ID, edge.ID.String())
	}
	if got.FromLeadID != leadA.Lead.ID || got.ToLeadID != leadB.Lead.ID {
		t.Fatalf("edge from/to = %q/%q, want %q/%q", got.FromLeadID, got.ToLeadID, leadA.Lead.ID, leadB.Lead.ID)
	}
	if got.Label != "反驳/张力" {
		t.Fatalf("edge label = %q, want 反驳/张力", got.Label)
	}
	if got.Status != "confirmed" {
		t.Fatalf("edge status = %q, want confirmed", got.Status)
	}
}

// -- B3 · POST /exploration/edges/propose -----------------------------------

// TestProposeEdges_PersistsProposed — B3's happy path: with a stub provider
// returning one valid index-based proposal connecting the project's two root
// questions, the endpoint persists it as a REAL question_edge with
// status="proposed" (never "confirmed" straight from the model — 铁律①, the
// student still confirms via the existing PATCH .../edges/{eid}) and records
// exactly one "edge_propose" llm_call row.
func TestProposeEdges_PersistsProposed(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool, SpecByID: cards.ByID,
		Provider:     readingStubProvider(`{"proposals":[{"from":1,"to":2,"label":"反驳/张力","why":"中国碳排放总量全球第一，是反例"}]}`),
		ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	recA := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"中国是否让地球更可持续？"}`)
	if recA.Code != http.StatusCreated {
		t.Fatalf("create lead A = %d: %s", recA.Code, recA.Body)
	}
	var leadA struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(recA.Body.Bytes(), &leadA); err != nil {
		t.Fatalf("decode lead A: %v — %s", err, recA.Body)
	}

	recB := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"中国碳排放总量全球第一，是否推翻论点？"}`)
	if recB.Code != http.StatusCreated {
		t.Fatalf("create lead B = %d: %s", recB.Code, recB.Body)
	}
	var leadB struct {
		Lead explorationLeadView `json:"lead"`
	}
	if err := json.Unmarshal(recB.Body.Bytes(), &leadB); err != nil {
		t.Fatalf("decode lead B: %v — %s", err, recB.Body)
	}

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/edges/propose", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("propose = %d, want 200: %s", rec.Code, rec.Body)
	}
	var out struct {
		Edges []questionEdgeView `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode propose response: %v — %s", err, rec.Body)
	}
	if len(out.Edges) != 1 {
		t.Fatalf("edges = %+v, want 1", out.Edges)
	}
	if out.Edges[0].FromLeadID != leadA.Lead.ID || out.Edges[0].ToLeadID != leadB.Lead.ID {
		t.Fatalf("edge from/to = %q/%q, want %q/%q", out.Edges[0].FromLeadID, out.Edges[0].ToLeadID, leadA.Lead.ID, leadB.Lead.ID)
	}
	if out.Edges[0].Label != "反驳/张力" {
		t.Fatalf("edge label = %q, want 反驳/张力", out.Edges[0].Label)
	}
	if out.Edges[0].Status != "proposed" {
		t.Fatalf("edge status = %q, want proposed (AI-proposed edges await student confirmation)", out.Edges[0].Status)
	}

	// The edge must be a REAL persisted row, not just echoed in the response.
	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	var view explorationViewBody
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode view: %v — %s", err, rec.Body)
	}
	if len(view.Edges) != 1 || view.Edges[0].ID != out.Edges[0].ID {
		t.Fatalf("proposed edge not persisted: view.Edges = %+v, want [%s]", view.Edges, out.Edges[0].ID)
	}

	if n := countLLMCallsByPurpose(t, pool, pid, "edge_propose"); n != 1 {
		t.Fatalf("want 1 edge_propose llm_call, got %d", n)
	}
}

// TestProposeEdges_FewerThanTwoRoots_NoSpend — with only one root question,
// there's nothing to connect: the endpoint must return 200 with an empty
// edges array WITHOUT ever resolving a provider or metering a call, even
// though one IS configured — mirrors TestExplorationGuide_EmptyGraphNoSpend's
// no-spend discipline.
func TestProposeEdges_FewerThanTwoRoots_NoSpend(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool, SpecByID: cards.ByID,
		Provider:     readingStubProvider(`{"proposals":[]}`),
		ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"唯一的问题"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create lead = %d: %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/edges/propose", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("propose = %d, want 200: %s", rec.Code, rec.Body)
	}
	var out struct {
		Edges []questionEdgeView `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode propose response: %v — %s", err, rec.Body)
	}
	if len(out.Edges) != 0 {
		t.Fatalf("edges = %+v, want empty with fewer than 2 root questions", out.Edges)
	}
	if n := countLLMCallsByPurpose(t, pool, pid, "edge_propose"); n != 0 {
		t.Fatalf("want 0 edge_propose llm_call with fewer than 2 roots, got %d", n)
	}
}

// TestProposeEdges_NoProviderNoSpend — with no ChatResolver configured at
// all (libraryTestHandler's default), even a graph with 2 root questions
// must return 200 with empty edges and record nothing — mirrors
// TestExplorationDig_NoProviderFallsBackToRawQuery's no-provider guarantee.
func TestProposeEdges_NoProviderNoSpend(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"问题一"}`)
	doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"问题二"}`)

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/edges/propose", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("propose = %d, want 200: %s", rec.Code, rec.Body)
	}
	var out struct {
		Edges []questionEdgeView `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode propose response: %v — %s", err, rec.Body)
	}
	if len(out.Edges) != 0 {
		t.Fatalf("edges = %+v, want empty with no provider configured", out.Edges)
	}
	if n := countLLMCallsByPurpose(t, pool, pid, "edge_propose"); n != 0 {
		t.Fatalf("want 0 edge_propose llm_call with no provider configured, got %d", n)
	}
}

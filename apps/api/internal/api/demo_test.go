package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// TestDemoProjectReadOnly verifies the loadOwnedProject demo semantics
// (guided-tour P2): a project flagged is_demo is world-readable to ANY
// authenticated user — not just its owner, since a later tour walks a
// non-owner through it — but rejects every mutation with 403 demo_readonly,
// for the owner and non-owners alike. A non-demo project keeps the ordinary
// ownership rule: hidden as 404 to everyone but its owner, writable by its
// owner.
func TestDemoProjectReadOnly(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	q := sqlc.New(pool)
	ctx := t.Context()

	ownerCookie := signInSeed(t, pool)
	otherID := createStudent(t, pool, SeedSchoolID, "demo-readonly-other@demo.local")
	otherCookie := signInAs(t, pool, otherID)

	// A normal project owned by SeedUserID.
	normal, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: SeedUserID, Qualification: "IB", Title: "普通项目", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create normal project: %v", err)
	}

	// The demo project — owned by SeedUserID, flagged is_demo. Ownership
	// still needs to resolve to *some* user row (the schema requires it), but
	// is_demo makes it world-readable regardless of who's asking.
	demo, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: SeedUserID, Qualification: "IB", Title: "演示项目", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create demo project: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE project SET is_demo = true WHERE id = $1`, demo.ID); err != nil {
		t.Fatalf("flag is_demo: %v", err)
	}

	renameBody, _ := json.Marshal(map[string]string{"title": "改名"})

	// (a) A NON-owner GET on the demo → 200, isDemo:true (world-readable).
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+demo.ID.String(), nil), otherCookie))
	if rr.Code != 200 {
		t.Fatalf("non-owner GET demo: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var proj struct {
		IsDemo bool `json:"isDemo"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &proj); err != nil {
		t.Fatalf("decode non-owner GET demo: %v", err)
	}
	if !proj.IsDemo {
		t.Fatalf("non-owner GET demo: want isDemo=true, got body=%s", rr.Body.String())
	}

	// (b) A non-owner WRITE on the demo → 403 demo_readonly.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PATCH", "/api/v1/projects/"+demo.ID.String(), bytes.NewReader(renameBody)), otherCookie))
	assertDemoReadonly(t, rr, "non-owner PATCH demo")

	// (c) The OWNER's own write on the demo → 403 too (demo is read-only for
	// everyone, ownership doesn't grant a bypass).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PATCH", "/api/v1/projects/"+demo.ID.String(), bytes.NewReader(renameBody)), ownerCookie))
	assertDemoReadonly(t, rr, "owner PATCH demo")

	// (d) A NON-demo project owned by someone else → 404 on GET (ownership
	// still enforced for non-demo projects).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+normal.ID.String(), nil), otherCookie))
	if rr.Code != 404 {
		t.Fatalf("non-owner GET normal: want 404, got %d — %s", rr.Code, rr.Body.String())
	}

	// The owner's write on the normal project still succeeds (guard is
	// scoped to is_demo, not a blanket lock).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PATCH", "/api/v1/projects/"+normal.ID.String(), bytes.NewReader(renameBody)), ownerCookie))
	if rr.Code != 200 {
		t.Fatalf("owner PATCH normal: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
}

// TestDemoTokenFixtures verifies the guided-tour P2 Task 2 seam: every
// token-consuming endpoint short-circuits to a canned fixture for a demo
// project — 200, no live model call (works with a nil Provider — newTestAPI
// wires none), and NO persistence. A NON-owner drives it, which also proves the
// world-readable token access loadOwnedProjectRow grants (Task 1).
func TestDemoTokenFixtures(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	q := sqlc.New(pool)
	ctx := t.Context()

	otherID := createStudent(t, pool, SeedSchoolID, "demo-fixtures-other@demo.local")
	otherCookie := signInAs(t, pool, otherID)

	// Demo project owned by SeedUserID (someone OTHER than the caller).
	demo, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: SeedUserID, Qualification: "IB", Title: "演示项目", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create demo project: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE project SET is_demo = true WHERE id = $1`, demo.ID); err != nil {
		t.Fatalf("flag is_demo: %v", err)
	}

	countAll := func(table string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	beforeMsgs := countAll("chat_message")
	beforeLLM := countAll("llm_call")

	// (a) POST /coach → 200 with the canned reply (non-owner, nil provider).
	coachBody, _ := json.Marshal(map[string]string{"user_input": "演示项目里我随便问一句"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+demo.ID.String()+"/coach", bytes.NewReader(coachBody)), otherCookie))
	if rr.Code != 200 {
		t.Fatalf("demo POST /coach: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var coach struct {
		Narrate string `json:"narrate"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &coach); err != nil {
		t.Fatalf("decode coach reply: %v", err)
	}
	if coach.Narrate == "" {
		t.Fatalf("demo POST /coach: want a canned narrate, got empty — %s", rr.Body.String())
	}

	// (b) POST /exploration/dig → 200 with 3 canned, real candidate sources
	// (never a live OpenAlex call) so the read-only tour can populate the
	// real 采纳/丢弃 panel.
	digBody, _ := json.Marshal(map[string]string{"keyword": "sustainability"})
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+demo.ID.String()+"/exploration/dig", bytes.NewReader(digBody)), otherCookie))
	if rr.Code != 200 {
		t.Fatalf("demo POST /exploration/dig: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var dig struct {
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
	if err := json.Unmarshal(rr.Body.Bytes(), &dig); err != nil {
		t.Fatalf("decode dig reply: %v", err)
	}
	if len(dig.Candidates) != 3 {
		t.Fatalf("demo POST /exploration/dig: want 3 candidates, got %d — %s", len(dig.Candidates), rr.Body.String())
	}
	for i, c := range dig.Candidates {
		if c.Title == "" || c.Authors == "" || c.Year == "" || c.Journal == "" || c.Abstract == "" || c.URL == "" {
			t.Fatalf("demo dig candidate[%d]: expected all fields populated (real content, no lorem), got %+v", i, c)
		}
	}

	// The demo path spent nothing and wrote nothing: no chat_message, no llm_call.
	if got := countAll("chat_message"); got != beforeMsgs {
		t.Fatalf("demo endpoints wrote chat_message rows: before=%d after=%d", beforeMsgs, got)
	}
	if got := countAll("llm_call"); got != beforeLLM {
		t.Fatalf("demo endpoints wrote llm_call rows: before=%d after=%d", beforeLLM, got)
	}
}

// TestDemoTakeawayAndAIUseDraftNeverCallLLM verifies the fix for the two GET
// token-spending endpoints that slipped past loadOwnedProject's non-GET-only
// write-guard: getTakeawayDraft (references/{rid}/takeaway-draft) and
// getAIUseDraft (ai-use-draft). Both must now resolve the SEEDED demo project
// (…0200, migration 0082) via loadOwnedProjectRow and short-circuit on
// row.IsDemo BEFORE ever touching a.d.Provider. Driven as a NON-owner (proving
// world-readability) against a real *API built with newTestAPI — which wires
// NO provider (Deps.Provider stays nil) — so any accidental compose call would
// nil-pointer-panic rather than silently succeed; a clean 200 is direct proof
// no model call happened.
func TestDemoTakeawayAndAIUseDraftNeverCallLLM(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	ctx := t.Context()

	otherID := createStudent(t, pool, SeedSchoolID, "demo-drafts-other@demo.local")
	otherCookie := signInAs(t, pool, otherID)

	var beforeLLM int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM llm_call`).Scan(&beforeLLM); err != nil {
		t.Fatalf("count llm_call before: %v", err)
	}

	base := "/api/v1/projects/" + demoProjectID

	// (a) GET .../ai-use-draft as a non-owner → 200, a valid draft envelope.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/ai-use-draft", nil), otherCookie))
	if rr.Code != 200 {
		t.Fatalf("demo GET /ai-use-draft: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var aiUse struct {
		Record map[string]any `json:"record"`
		Draft  struct {
			UsedFor    string `json:"usedFor"`
			NotUsedFor string `json:"notUsedFor"`
		} `json:"draft"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &aiUse); err != nil {
		t.Fatalf("decode ai-use-draft: %v — body=%s", err, rr.Body.String())
	}
	if aiUse.Record == nil {
		t.Fatalf("demo ai-use-draft: want a record object, got none — %s", rr.Body.String())
	}

	// (b) GET .../references/{rid}/takeaway-draft, for a demo reference the
	// seed gave material content (…0260, linked to material …0271 with a
	// confirmed CRAAP card instance) → 200, a valid (possibly empty) draft.
	rid := "00000000-0000-0000-0000-000000000260"
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/references/"+rid+"/takeaway-draft", nil), otherCookie))
	if rr.Code != 200 {
		t.Fatalf("demo GET /references/%s/takeaway-draft: want 200, got %d — %s", rid, rr.Code, rr.Body.String())
	}
	var takeaway struct {
		Record                  map[string]any `json:"record"`
		SuggestedNewLeads       []string       `json:"suggestedNewLeads"`
		SuggestedProposalImpact string         `json:"suggestedProposalImpact"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &takeaway); err != nil {
		t.Fatalf("decode takeaway-draft: %v — body=%s", err, rr.Body.String())
	}
	if takeaway.Record == nil {
		t.Fatalf("demo takeaway-draft: want a record object, got none — %s", rr.Body.String())
	}
	if takeaway.SuggestedNewLeads == nil {
		t.Fatalf("demo takeaway-draft: want suggestedNewLeads as [] (never null), got null — %s", rr.Body.String())
	}

	// Neither call spent anything: no llm_call rows written, proving the demo
	// never reached the compose/provider block (a.d.Provider is nil on this
	// test API — an unguarded call here would have panicked, not just spent).
	var afterLLM int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM llm_call`).Scan(&afterLLM); err != nil {
		t.Fatalf("count llm_call after: %v", err)
	}
	if afterLLM != beforeLLM {
		t.Fatalf("demo draft endpoints wrote llm_call rows: before=%d after=%d", beforeLLM, afterLLM)
	}
}

// TestGetMaterialSourceDemoReadOnly verifies the guided-tour P6 read-only
// endpoint (getMaterialSource): a NON-owner GET against the seeded demo
// material …0271 (Chen et al. 2019, linked into demo project …0200 by
// migration 0082) returns a 200 MaterialSource with every required field
// populated — matching enter-reading's own projection, just without its
// write side effects — and a non-GET request to the very same path is either
// 403 demo_readonly (if some other verb happened to be routed) or, as is the
// actual case here since the route is registered GET-only, unrouted (405),
// proving the endpoint never funnels through the mutation chokepoint.
func TestGetMaterialSourceDemoReadOnly(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()

	otherID := createStudent(t, pool, SeedSchoolID, "demo-material-source-other@demo.local")
	otherCookie := signInAs(t, pool, otherID)

	base := "/api/v1/projects/" + demoProjectID + "/materials/00000000-0000-0000-0000-000000000271/source"

	// (a) Non-owner GET → 200, a fully-populated MaterialSource.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base, nil), otherCookie))
	if rr.Code != 200 {
		t.Fatalf("demo GET material source: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var src struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		SourceURL string `json:"sourceUrl"`
		Kind      string `json:"kind"`
		Origin    string `json:"origin"`
		Blocks    []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"blocks"`
		Locked              bool   `json:"locked"`
		Role                string `json:"role"`
		Tier                string `json:"tier"`
		Takeaway            string `json:"takeaway"`
		Anchors             []any  `json:"anchors"`
		TimeSpentS          int    `json:"timeSpentS"`
		LateralRead         bool   `json:"lateralRead"`
		IsLateralInstrument bool   `json:"isLateralInstrument"`
		SiftSkipped         bool   `json:"siftSkipped"`
		LateralRelation     string `json:"lateralRelation"`
		LateralJudgment     string `json:"lateralJudgment"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &src); err != nil {
		t.Fatalf("decode material source: %v — body=%s", err, rr.Body.String())
	}
	if src.ID == "" || src.Title == "" || src.SourceURL == "" || src.Kind == "" || src.Origin == "" {
		t.Fatalf("demo material source: expected id/title/sourceUrl/kind/origin all populated, got %+v", src)
	}
	// 0086 (P7 Task 2) replaced the 3-sentence stub with a full-length ~10-
	// paragraph article and seeded real inline anchors (via a completed CRAAP
	// card_instances row) so the tour can point at highlighted spans.
	if len(src.Blocks) < 10 {
		t.Fatalf("demo material source: want >=10 blocks (full-length article), got %d — %s", len(src.Blocks), rr.Body.String())
	}
	for i, b := range src.Blocks {
		if b.ID == "" || b.Text == "" {
			t.Fatalf("demo material source: block[%d] missing id/text — %+v", i, b)
		}
	}
	if len(src.Anchors) == 0 {
		t.Fatalf("demo material source: want non-empty anchors (real inline highlights), got 0 — %s", rr.Body.String())
	}

	// (b) A non-GET verb on the exact same path is not routed to a mutation
	// handler: the route is registered GET-only, so Go's net/http mux returns
	// 405 method-not-allowed rather than ever reaching loadOwnedProject's
	// non-GET demo_readonly guard.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base, nil), otherCookie))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST material source: want 405 (unrouted, not funneled through mutation guard), got %d — %s", rr.Code, rr.Body.String())
	}
}

// TestGetStudioStateDemoProjectIsRetrospective verifies migration 0084: the
// seeded demo project (…0200, status 'finished' since 0082 Task 4) must
// report studio_state.stage = "retrospective", not the stale
// 'topic_discussion' left over from 0082's initial INSERT. The wrong stage
// made the frontend compute docOptions=["essay"] and broke the guided tour's
// PROPOSAL 片段 card. Driven as a non-owner GET (demo is world-readable).
func TestGetStudioStateDemoProjectIsRetrospective(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()

	otherID := createStudent(t, pool, SeedSchoolID, "demo-studio-state-other@demo.local")
	otherCookie := signInAs(t, pool, otherID)

	base := "/api/v1/projects/" + demoProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/studio-state", nil), otherCookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("demo GET /studio-state: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var st struct {
		Stage string `json:"stage"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode studio-state: %v — body=%s", err, rr.Body.String())
	}
	if st.Stage != "retrospective" {
		t.Fatalf("demo studio-state.stage: want %q, got %q — %s", "retrospective", st.Stage, rr.Body.String())
	}
}

// TestGetDraftDemoEssayHasRealNewlines verifies migration 0085: the seeded
// demo essay's edit_buffer content (…02f0, 0082) must contain REAL newline
// characters between paragraphs, not the literal 4-character sequence \n\n.
// 0082's `||` concatenation had only its first segment as an E'...' escape
// string and every following segment as a plain '...' literal, so under
// standard_conforming_strings the \n\n inside those plain segments was stored
// as literal backslash-n-backslash-n rather than a real line break — the demo
// essay rendered with visible "\n\n" in the writing room and reading views
// (this is demo-seed-only; real finished projects store real typed newlines).
// Driven as a non-owner GET (demo is world-readable, per TestDemoProjectReadOnly).
func TestGetDraftDemoEssayHasRealNewlines(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()

	otherID := createStudent(t, pool, SeedSchoolID, "demo-draft-newlines-other@demo.local")
	otherCookie := signInAs(t, pool, otherID)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+demoProjectID+"/draft", nil), otherCookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("demo GET /draft: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var draft struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &draft); err != nil {
		t.Fatalf("decode draft: %v — body=%s", err, rr.Body.String())
	}
	if draft.Content == "" {
		t.Fatalf("demo draft: want non-empty essay content, got empty")
	}
	if !strings.Contains(draft.Content, "\n") {
		t.Fatalf("demo draft: want a real newline between paragraphs, got none — body=%s", draft.Content)
	}
	if strings.Contains(draft.Content, `\n`) {
		t.Fatalf("demo draft: want NO literal backslash-n substring, but content still contains it — body=%s", draft.Content)
	}
}

// TestDemoWarrenHasSecondLayerAndResourceNeeds verifies migration 0087: the
// guided tour (P7 Task 9) clicks a root lead on the demo warren map and needs
// a real 2nd layer to reveal, and the 还需要探索 box needs to be non-empty.
// Before 0087 every one of the demo's 4 exploration_lead rows (0082:169-182)
// was a root (parent_lead_id NULL) and studio_state.resourceNeeds was absent.
// Driven as a non-owner GET (demo is world-readable, TestDemoProjectReadOnly).
func TestDemoWarrenHasSecondLayerAndResourceNeeds(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()

	otherID := createStudent(t, pool, SeedSchoolID, "demo-warren-other@demo.local")
	otherCookie := signInAs(t, pool, otherID)

	base := "/api/v1/projects/" + demoProjectID

	// (a) GET .../exploration -> at least 2 leads whose parentLeadId is the
	// same existing root ("既然是最大碳排放国，为什么还能说治理有决心？", …0292).
	rootID := "00000000-0000-0000-0000-000000000292"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/exploration", nil), otherCookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("demo GET /exploration: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var exp struct {
		Leads []struct {
			ID           string  `json:"id"`
			Text         string  `json:"text"`
			ParentLeadID *string `json:"parentLeadId"`
		} `json:"leads"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &exp); err != nil {
		t.Fatalf("decode exploration: %v — body=%s", err, rr.Body.String())
	}
	var children int
	for _, l := range exp.Leads {
		if l.ParentLeadID != nil && *l.ParentLeadID == rootID {
			children++
			if l.Text == "" {
				t.Fatalf("demo warren child lead has empty text: %+v", l)
			}
		}
	}
	if children < 2 {
		t.Fatalf("demo warren: want >=2 children under root %s, got %d — leads=%+v", rootID, children, exp.Leads)
	}

	// (b) GET .../resource-needs -> the seeded 还需要探索 rows, non-empty.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/resource-needs", nil), otherCookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("demo GET /resource-needs: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var needs struct {
		Needs []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
			Done bool   `json:"done"`
		} `json:"needs"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &needs); err != nil {
		t.Fatalf("decode resource-needs: %v — body=%s", err, rr.Body.String())
	}
	if len(needs.Needs) < 2 {
		t.Fatalf("demo resource-needs: want >=2 seeded rows, got %d — %+v", len(needs.Needs), needs.Needs)
	}
	for _, n := range needs.Needs {
		if n.ID == "" || n.Text == "" {
			t.Fatalf("demo resource-needs: row missing id/text — %+v", n)
		}
	}
}

// TestDemoSearchGuidanceRealDirections verifies guided-tour P8 Task 4: the
// demo project's POST .../search-guidance short-circuit (cannedSearchGuidance)
// must return >=2 real, on-topic keyword+why directions — not the old
// single "示例检索方向（演示）" placeholder — so the read-only tour's「让印记
// 建议检索方向」button has genuine content to show. Driven as a non-owner
// (demo is world-readable) against a real *API wired with NO provider, so a
// 200 with real content also proves no live model/network call happened.
func TestDemoSearchGuidanceRealDirections(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	ctx := t.Context()

	otherID := createStudent(t, pool, SeedSchoolID, "demo-search-guidance-other@demo.local")
	otherCookie := signInAs(t, pool, otherID)

	var beforeLLM int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM llm_call`).Scan(&beforeLLM); err != nil {
		t.Fatalf("count llm_call before: %v", err)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+demoProjectID+"/search-guidance", bytes.NewReader([]byte("{}"))), otherCookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("demo POST /search-guidance: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Suggestions []struct {
			Keyword string `json:"keyword"`
			Why     string `json:"why"`
		} `json:"suggestions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode search-guidance: %v — body=%s", err, rr.Body.String())
	}
	if len(out.Suggestions) < 2 {
		t.Fatalf("demo search-guidance: want >=2 directions, got %d — %s", len(out.Suggestions), rr.Body.String())
	}
	for i, s := range out.Suggestions {
		if s.Keyword == "" || s.Why == "" {
			t.Fatalf("demo search-guidance suggestion[%d]: expected non-empty keyword+why, got %+v", i, s)
		}
		if strings.Contains(s.Keyword, "演示") || strings.Contains(s.Keyword, "示例") {
			t.Fatalf("demo search-guidance suggestion[%d]: keyword still looks like a placeholder — %+v", i, s)
		}
		if strings.Contains(s.Why, "演示") || strings.Contains(s.Why, "示例") {
			t.Fatalf("demo search-guidance suggestion[%d]: why still looks like a placeholder — %+v", i, s)
		}
	}

	var afterLLM int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM llm_call`).Scan(&afterLLM); err != nil {
		t.Fatalf("count llm_call after: %v", err)
	}
	if afterLLM != beforeLLM {
		t.Fatalf("demo search-guidance wrote llm_call rows: before=%d after=%d", beforeLLM, afterLLM)
	}
}

// assertDemoReadonly asserts rr is a 403 carrying error.code = "demo_readonly".
func assertDemoReadonly(t *testing.T, rr *httptest.ResponseRecorder, label string) {
	t.Helper()
	if rr.Code != 403 {
		t.Fatalf("%s: want 403, got %d — %s", label, rr.Code, rr.Body.String())
	}
	var errBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("%s: decode error body: %v", label, err)
	}
	if errBody.Error.Code != "demo_readonly" {
		t.Fatalf("%s: want code=demo_readonly, got %q — %s", label, errBody.Error.Code, rr.Body.String())
	}
}

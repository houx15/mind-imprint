package api_test

// workspace_journey_test.go — Slice 7 Task C: the redesign's acceptance
// mainline as ONE integration test. It walks a project through every workspace
// room over the real HTTP handlers against a testcontainer DB, signed in as the
// seeded student, with mock providers/resolvers/fetcher injected for the LLM
// and network seams (same doubles the per-room slice tests use). It asserts the
// PERSISTED data flow at each hop — not just status codes — so a regression in
// any room's storage surfaces here.
//
//	projection → proposal → coach(forming) → plan(+patch)+log →
//	references+enter-reading+library → outline+draft →
//	reflection-doc(done) → finish → assessment → growth/history → mirror(once)

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func TestWorkspaceJourney_Mainline(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)

	// A source server for enter-reading's URL fetch path (fakeFetcher GETs it).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>全球变绿研究</title></head><body>
			<p>NASA 卫星数据显示，2000 到 2017 年间全球叶面积指数上升了约 5%。</p>
			<p>中国和印度贡献了净增量的三分之一。</p></body></html>`))
	}))
	defer srv.Close()

	// hCore serves every non-flagship endpoint: fakeProvider covers the coach
	// turn (gateway.Collect), fakeFetcher covers enter-reading's URL fetch.
	hCore := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(),
		Fetcher: fakeFetcher{}, SpecByID: cards.ByID,
	}).Handler()

	pid := createProjectForTest(t, hCore, cookie)
	base := "/api/v1/projects/" + pid

	// -- 1. GET /projects/{id} → the lean workspace projection ---------------
	var proj struct {
		ID            string `json:"id"`
		Title         string `json:"title"`
		Qualification string `json:"qualification"`
		Proposal      struct {
			Objective  string `json:"objective"`
			Reason     string `json:"reason"`
			Activities string `json:"activities"`
			Resources  string `json:"resources"`
		} `json:"proposal"`
	}
	rec := doJSON(t, hCore, cookie, "GET", base, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET projection = %d: %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &proj); err != nil {
		t.Fatalf("decode projection: %v — %s", err, rec.Body)
	}
	if proj.ID != pid {
		t.Fatalf("projection id = %q, want %q", proj.ID, pid)
	}
	if proj.Proposal.Objective != "" {
		t.Fatalf("fresh project should have an empty proposal, got %+v", proj.Proposal)
	}

	// -- 2. PUT /proposal → GET reflects it ----------------------------------
	doOK(t, hCore, cookie, "PUT", base+"/proposal",
		`{"objective":"论证中国是否让地球更可持续","reason":"我关心气候与国家责任","activities":"读一手数据、搭论证、写稿","resources":"NASA 数据、Nature Sustainability"}`)
	rec = doJSON(t, hCore, cookie, "GET", base, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &proj)
	if proj.Proposal.Objective != "论证中国是否让地球更可持续" || proj.Proposal.Resources == "" {
		t.Fatalf("proposal not reflected after PUT: %+v", proj.Proposal)
	}

	// -- 3. POST /coach → orchestrator narration + a coach_turn event --------
	rec = doJSON(t, hCore, cookie, "POST", base+"/coach",
		`{"user_input":"我想聊聊这个题目从哪儿下手"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST coach = %d: %s", rec.Code, rec.Body)
	}
	var coachResp struct {
		Narrate string `json:"narrate"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &coachResp)
	if strings.TrimSpace(coachResp.Narrate) == "" {
		t.Fatalf("coach narrate empty: %s", rec.Body)
	}
	if n := countEvents(t, pool, pid, "coach_turn"); n != 1 {
		t.Fatalf("coach_turn events = %d, want 1", n)
	}

	// -- 4. Plan items ×2 + PATCH (move/reschedule/resize) + log -------------
	ridRead := createPlanItem(t, hCore, cookie, base, `{"title":"精读 NASA 数据","tag":"read","column":"todo","stage":"信源","start":0,"days":2}`)
	_ = createPlanItem(t, hCore, cookie, base, `{"title":"写论证初稿","tag":"write","column":"todo","stage":"写作","start":2,"days":4}`)

	// Move column + reschedule start + resize days on the first item.
	doOK(t, hCore, cookie, "PATCH", base+"/plan/items/"+ridRead, `{"column":"doing","start":3,"days":5}`)

	var plan struct {
		Items []struct {
			ID     string `json:"id"`
			Column string `json:"column"`
			Start  int32  `json:"start"`
			Days   int32  `json:"days"`
		} `json:"items"`
	}
	rec = doJSON(t, hCore, cookie, "GET", base+"/plan", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &plan)
	if len(plan.Items) != 2 {
		t.Fatalf("plan items = %d, want 2", len(plan.Items))
	}
	var moved bool
	for _, it := range plan.Items {
		if it.ID == ridRead {
			moved = true
			if it.Column != "doing" || it.Start != 3 || it.Days != 5 {
				t.Fatalf("PATCH not reflected: column=%q start=%d days=%d, want doing/3/5", it.Column, it.Start, it.Days)
			}
		}
	}
	if !moved {
		t.Fatalf("patched item %s not present in GET /plan", ridRead)
	}

	if r := doJSON(t, hCore, cookie, "POST", base+"/log", `{"text":"今天定了开题四问"}`); r.Code != http.StatusCreated {
		t.Fatalf("POST log = %d, want 201: %s", r.Code, r.Body)
	}
	var logResp struct {
		Entries []struct {
			Text   string `json:"text"`
			Source string `json:"source"`
		} `json:"entries"`
	}
	rec = doJSON(t, hCore, cookie, "GET", base+"/log", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &logResp)
	var sawMine bool
	for _, e := range logResp.Entries {
		if e.Source == "me" && e.Text == "今天定了开题四问" {
			sawMine = true
		}
	}
	if !sawMine {
		t.Fatalf("manual log entry not persisted; entries=%+v", logResp.Entries)
	}

	// -- 5. References + enter-reading → MaterialSource + library ------------
	rec = doJSON(t, hCore, cookie, "POST", base+"/references", fmt.Sprintf(`{"title":"全球变绿","url":%q}`, srv.URL))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST reference = %d: %s", rec.Code, rec.Body)
	}
	var refWrap struct {
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)
	rid := refWrap.Reference.ID
	if rid == "" {
		t.Fatalf("reference id missing: %s", rec.Body)
	}

	rec = doJSON(t, hCore, cookie, "POST", base+"/references/"+rid+"/enter-reading", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("enter-reading = %d: %s", rec.Code, rec.Body)
	}
	var ms struct {
		ID     string            `json:"id"`
		Blocks []json.RawMessage `json:"blocks"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ms)
	if ms.ID == "" || len(ms.Blocks) == 0 {
		t.Fatalf("MaterialSource missing blocks: %s", rec.Body)
	}

	rec = doJSON(t, hCore, cookie, "GET", base+"/library", "")
	var lib struct {
		References []struct {
			ID         string  `json:"id"`
			MaterialID *string `json:"materialId"`
		} `json:"references"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &lib)
	var linked bool
	for _, r := range lib.References {
		if r.ID == rid && r.MaterialID != nil && *r.MaterialID == ms.ID {
			linked = true
		}
	}
	if !linked {
		t.Fatalf("library does not show reference %s linked to material %s: %+v", rid, ms.ID, lib.References)
	}

	// -- 6. Outline (nested) + draft buffer ----------------------------------
	doOK(t, hCore, cookie, "PUT", base+"/outline",
		`{"nodes":[{"text":"引言","depth":0},{"text":"中国的绿化贡献","depth":1},{"text":"但排放总量仍第一","depth":1},{"text":"数据细节","depth":2}]}`)
	var outline struct {
		Nodes []struct {
			Text  string `json:"text"`
			Depth int32  `json:"depth"`
		} `json:"nodes"`
	}
	rec = doJSON(t, hCore, cookie, "GET", base+"/outline", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &outline)
	if len(outline.Nodes) != 4 || outline.Nodes[3].Depth != 2 {
		t.Fatalf("outline not persisted with nested depths: %+v", outline.Nodes)
	}

	if r := doJSON(t, hCore, cookie, "PUT", base+"/buffer", `{"content":"中国的可再生能源投资规模已连续五年全球第一。"}`); r.Code != http.StatusNoContent {
		t.Fatalf("PUT buffer = %d: %s", r.Code, r.Body)
	}
	rec = doJSON(t, hCore, cookie, "GET", base+"/draft", "")
	var draft struct {
		Content string `json:"content"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &draft)
	if !strings.Contains(draft.Content, "可再生能源投资规模") {
		t.Fatalf("draft buffer not persisted: %q", draft.Content)
	}

	// -- 7. 完成写作 (#20) → reflection-doc done → finish → assessment ----------
	// The two-stage 写作→回顾 flow: writing must be finished before finalize.
	if r := doJSON(t, hCore, cookie, "POST", base+"/finish-writing", ""); r.Code != http.StatusOK {
		t.Fatalf("POST finish-writing = %d, want 200: %s", r.Code, r.Body)
	}
	doOK(t, hCore, cookie, "PUT", base+"/reflection-doc",
		`{"answers":["我学会了先收窄问题","读到的一手数据真的进了论证","下次动笔前先写一句 thesis","","反例让我把结论收紧了"],"done":true}`)

	// finish generates the flagship terminal report — its own handler wired
	// with the assessment JSON stub + flagship resolver.
	hFinish := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(),
		SpecByID: cards.ByID,
	}).Handler()
	// Finish is async (BE5): 202 evaluating, then a goroutine drives it to
	// finished — wait for that before reading the assessment.
	rec = doJSON(t, hFinish, cookie, "POST", base+"/finish", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST finish = %d, want 202: %s", rec.Code, rec.Body)
	}
	waitProjectStatus(t, pool, pid, "finished")

	rec = doJSON(t, hCore, cookie, "GET", base+"/assessment", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET assessment = %d: %s", rec.Code, rec.Body)
	}
	if strings.TrimSpace(rec.Body.String()) == "null" {
		t.Fatalf("assessment is null after finish; want a report")
	}
	var report struct {
		GeneratedAt string `json:"generatedAt"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &report)
	if report.GeneratedAt == "" {
		t.Fatalf("assessment report missing generatedAt: %s", rec.Body)
	}

	rec = doJSON(t, hCore, cookie, "GET", "/api/v1/growth/history", "")
	var history struct {
		Entries []struct {
			Surface string `json:"surface"`
			ScopeID string `json:"scopeId"`
		} `json:"entries"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &history)
	var inHistory bool
	for _, e := range history.Entries {
		if e.Surface == "project" && e.ScopeID == pid {
			inHistory = true
		}
	}
	if !inHistory {
		t.Fatalf("growth/history missing the finished project %s: %+v", pid, history.Entries)
	}

	// -- 8. Mirror composes then first-open-wins (no second spend) -----------
	// Finish's own goroutine already best-effort-attempted a mirror (BE5) with
	// the assessment provider, which isn't valid mirror JSON — so it spent but
	// stored nothing. This step composes the real mirror with a mirror provider;
	// we assert first-open-wins as a DELTA (the second POST adds no new call)
	// rather than an absolute count, since the finish attempt confounds it.
	hMirror := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(mirrorReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(),
		SpecByID: cards.ByID,
	}).Handler()
	rec = doJSON(t, hMirror, cookie, "POST", base+"/mirror", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("POST mirror = %d: %s", rec.Code, rec.Body)
	}
	var mirror struct {
		Sections []struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		} `json:"sections"`
		CarryForwards []string `json:"carryForwards"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &mirror)
	if len(mirror.Sections) == 0 || len(mirror.CarryForwards) == 0 {
		t.Fatalf("mirror missing sections/carryForwards: %s", rec.Body)
	}
	afterFirst := countLLMCallsByPurpose(t, pool, pid, "mirror")
	// Second POST is a no-spend read of the stored row (first-open-wins).
	if r := doJSON(t, hMirror, cookie, "POST", base+"/mirror", ""); r.Code != http.StatusOK {
		t.Fatalf("second POST mirror = %d: %s", r.Code, r.Body)
	}
	if n := countLLMCallsByPurpose(t, pool, pid, "mirror"); n != afterFirst {
		t.Fatalf("mirror llm_call rows after second POST = %d, want still %d (first-open-wins)", n, afterFirst)
	}
}

// -- local helpers ----------------------------------------------------------

// doOK is doJSON asserting a 200 OK (the common workspace write response).
func doOK(t *testing.T, h http.Handler, cookie *http.Cookie, method, path, body string) {
	t.Helper()
	rec := doJSON(t, h, cookie, method, path, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s = %d, want 200: %s", method, path, rec.Code, rec.Body)
	}
}

// createPlanItem POSTs one plan item and returns its id.
func createPlanItem(t *testing.T, h http.Handler, cookie *http.Cookie, base, body string) string {
	t.Helper()
	rec := doJSON(t, h, cookie, "POST", base+"/plan/items", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST plan item = %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		Item struct {
			ID string `json:"id"`
		} `json:"item"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Item.ID == "" {
		t.Fatalf("plan item id missing: %s", rec.Body)
	}
	return out.Item.ID
}

// countEvents counts event rows of a given type for a project.
func countEvents(t *testing.T, pool *pgxpool.Pool, projectID, eventType string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE project_id=$1 AND type=$2`, projectID, eventType).Scan(&n); err != nil {
		t.Fatalf("count %s events: %v", eventType, err)
	}
	return n
}

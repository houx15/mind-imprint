package api_test

// workspace_plan_test.go — Slice 2 (Project Management room) backend tests:
// the proposal PUT round-trip, the plan-item CRUD lifecycle, the activity log,
// ownership 404, and the restrained /coach turn against an injected fake
// provider. Uses the seeded demo project (00000000-0000-0000-0000-000000000101,
// owned by Phoebe) via signInSeed, mirroring studioturn_test.go.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

const seedProjectID = "00000000-0000-0000-0000-000000000101"

func planTestHandler(t *testing.T) (http.Handler, *http.Cookie, *sqlc.Queries) {
	t.Helper()
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	return h, cookie, sqlc.New(pool)
}

func TestPutProposal_RoundTrip(t *testing.T) {
	h, cookie, _ := planTestHandler(t)

	body := `{"objective":"看中国是否让地球更可持续","reason":"我关心气候","activities":"读 NASA/Nature，写论证","resources":"Zotero + 学校图书馆"}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", "/api/v1/projects/"+seedProjectID+"/proposal", strings.NewReader(body)), cookie))
	if rr.Code != 200 {
		t.Fatalf("put proposal: %d — %s", rr.Code, rr.Body.String())
	}
	for _, want := range []string{`"objective":"看中国是否让地球更可持续"`, `"resources":"Zotero + 学校图书馆"`} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Fatalf("put response missing %s — %s", want, rr.Body.String())
		}
	}

	// The projection (GET /projects/{id}) must now carry the saved dims.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+seedProjectID, nil), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"reason":"我关心气候"`) {
		t.Fatalf("proposal did not persist into the projection: %d — %s", rr.Code, rr.Body.String())
	}

	// First save must have dropped an auto-log line.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+seedProjectID+"/log", nil), cookie))
	if !strings.Contains(rr.Body.String(), `"source":"auto"`) {
		t.Fatalf("expected an auto log entry after first proposal save: %s", rr.Body.String())
	}
}

// TestCoach_ProposalReviewScope — BE2: the proposal_review scope produces a
// restrained reply (the frontend passes the four dims as user_input).
func TestCoach_ProposalReviewScope(t *testing.T) {
	h, cookie, _ := planTestHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"scope":"proposal_review","user_input":"目标：看中国是否让地球更可持续；缘由：我关心气候；活动：读NASA/Nature；资源：Zotero"}`)), cookie))
	if rr.Code != 200 {
		t.Fatalf("coach proposal_review = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		Reply string `json:"reply"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || strings.TrimSpace(resp.Reply) == "" {
		t.Fatalf("coach proposal_review reply empty (err=%v): %s", err, rr.Body)
	}
}

// planGenReply is a valid plan/generate JSON array (5 tasks, tags balanced).
const planGenReply = `[
  {"title":"通读并溯源 NASA 与 Nature 数据","tag":"read","stage":"阶段一 · 研究","start":0,"days":4},
  {"title":"整理正反两方证据","tag":"read","stage":"阶段一 · 研究","start":4,"days":3},
  {"title":"写论点与提纲","tag":"write","stage":"阶段二 · 写作","start":7,"days":3},
  {"title":"写第一版正文","tag":"write","stage":"阶段二 · 写作","start":10,"days":5},
  {"title":"回顾并检查反例","tag":"review","stage":"阶段二 · 写作","start":15,"days":3}]`

// TestPlanGenerate_ProposalEmpty — BE3: with an empty kick-off, plan/generate
// refuses with 422 proposal_empty and creates no plan items.
func TestPlanGenerate_ProposalEmpty(t *testing.T) {
	h, cookie, _ := planTestHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/plan/generate", strings.NewReader("")), cookie))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("plan/generate (empty proposal) = %d, want 422; body=%s", rr.Code, rr.Body)
	}
	var perr struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &perr); err != nil || perr.Error.Code != "proposal_empty" {
		t.Fatalf("plan/generate code = %+v (err=%v), want proposal_empty; body=%s", perr, err, rr.Body)
	}
}

// TestPlanGenerate_CreatesItems — BE3: with a filled proposal, plan/generate
// persists the model's tasks as todo plan_items, returns them, meters one
// plan_gen call, and drops the auto-log line.
func TestPlanGenerate_CreatesItems(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(planGenReply), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := seedProjectID
	base := "/api/v1/projects/" + pid

	// Fill the proposal so the kick-off isn't empty.
	rrProp := httptest.NewRecorder()
	h.ServeHTTP(rrProp, withCookie(httptest.NewRequest("PUT", base+"/proposal",
		strings.NewReader(`{"objective":"论证国内新能源投资","reason":"关心气候","activities":"读NASA/Nature","resources":"Zotero"}`)), cookie))
	if rrProp.Code != 200 {
		t.Fatalf("PUT proposal = %d — %s", rrProp.Code, rrProp.Body)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/plan/generate", strings.NewReader("")), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("plan/generate = %d, want 200; body=%s", rr.Code, rr.Body)
	}
	var out struct {
		Items []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Tag    string `json:"tag"`
			Column string `json:"column"`
			Stage  string `json:"stage"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode items: %v — %s", err, rr.Body)
	}
	if len(out.Items) != 5 {
		t.Fatalf("generated items = %d, want 5 (the model reply)", len(out.Items))
	}
	for _, it := range out.Items {
		if it.Column != "todo" {
			t.Fatalf("item %q column = %q, want todo", it.Title, it.Column)
		}
		if !map[string]bool{"read": true, "write": true, "review": true}[it.Tag] {
			t.Fatalf("item %q tag = %q, invalid", it.Title, it.Tag)
		}
	}

	// GET /plan reflects the persisted items.
	rrList := httptest.NewRecorder()
	h.ServeHTTP(rrList, withCookie(httptest.NewRequest("GET", base+"/plan", nil), cookie))
	var listed struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rrList.Body.Bytes(), &listed)
	if len(listed.Items) != 5 {
		t.Fatalf("plan list after generate = %d, want 5", len(listed.Items))
	}

	if n := countLLMCallsByPurpose(t, pool, pid, "plan_gen"); n != 1 {
		t.Fatalf("plan_gen llm_call rows = %d, want 1", n)
	}
	var autolog int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM activity_log_entry WHERE project_id=$1 AND text=$2`, pid, "印记根据开题生成了项目计划").Scan(&autolog)
	if autolog != 1 {
		t.Fatalf("plan_gen auto-log rows = %d, want 1", autolog)
	}
}

func TestPlanItem_CRUDLifecycle(t *testing.T) {
	h, cookie, _ := planTestHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	// Create.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/plan/items",
		strings.NewReader(`{"title":"读 NASA 报告","tag":"read","column":"todo","stage":"研究","start":0,"days":2}`)), cookie))
	if rr.Code != 201 {
		t.Fatalf("create: %d — %s", rr.Code, rr.Body.String())
	}
	var created struct {
		Item struct {
			ID     string `json:"id"`
			Column string `json:"column"`
			Start  int    `json:"start"`
			Days   int    `json:"days"`
		} `json:"item"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v — %s", err, rr.Body.String())
	}
	if created.Item.ID == "" || created.Item.Column != "todo" {
		t.Fatalf("bad created item: %+v", created.Item)
	}
	iid := created.Item.ID

	// List includes it.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/plan", nil), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), iid) {
		t.Fatalf("list missing created item: %d — %s", rr.Code, rr.Body.String())
	}

	// Patch: move column + reschedule start + resize days in one go.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PATCH", base+"/plan/items/"+iid,
		strings.NewReader(`{"column":"doing","start":3,"days":5}`)), cookie))
	if rr.Code != 200 {
		t.Fatalf("patch: %d — %s", rr.Code, rr.Body.String())
	}
	var patched struct {
		Item struct {
			Column string `json:"column"`
			Start  int    `json:"start"`
			Days   int    `json:"days"`
			Title  string `json:"title"`
		} `json:"item"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patch: %v — %s", err, rr.Body.String())
	}
	if patched.Item.Column != "doing" || patched.Item.Start != 3 || patched.Item.Days != 5 {
		t.Fatalf("patch did not apply move/reschedule/resize: %+v", patched.Item)
	}
	// Merge preserved the untouched title.
	if patched.Item.Title != "读 NASA 报告" {
		t.Fatalf("patch clobbered the untouched title: %+v", patched.Item)
	}

	// Delete -> 204, then gone from the list.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("DELETE", base+"/plan/items/"+iid, nil), cookie))
	if rr.Code != 204 {
		t.Fatalf("delete: %d — %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/plan", nil), cookie))
	if strings.Contains(rr.Body.String(), iid) {
		t.Fatalf("deleted item still listed: %s", rr.Body.String())
	}
}

func TestActivityLog_PostThenList(t *testing.T) {
	h, cookie, _ := planTestHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/log", strings.NewReader(`{"text":"今天把研究问题定下来了"}`)), cookie))
	if rr.Code != 201 {
		t.Fatalf("post log: %d — %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"source":"me"`) {
		t.Fatalf("manual log must be source=me: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/log", nil), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "今天把研究问题定下来了") {
		t.Fatalf("list log missing manual entry: %d — %s", rr.Code, rr.Body.String())
	}
	// date rendered "MM-DD".
	var out struct {
		Entries []struct {
			Date string `json:"date"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if len(out.Entries) == 0 || len(out.Entries[len(out.Entries)-1].Date) != 5 {
		t.Fatalf("date not MM-DD: %+v", out.Entries)
	}

	// empty text -> 400.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/log", strings.NewReader(`{"text":"  "}`)), cookie))
	if rr.Code != 400 {
		t.Fatalf("empty log text: want 400, got %d", rr.Code)
	}
}

// TestWorkspacePlan_Ownership404 proves every Slice 2 route hides a foreign
// (here: non-existent, same 404-no-leak path loadOwnedProject enforces) project
// as not-found rather than leaking its existence.
func TestWorkspacePlan_Ownership404(t *testing.T) {
	h, cookie, _ := planTestHandler(t)
	foreign := "/api/v1/projects/00000000-0000-0000-0000-0000000009ff"

	cases := []struct {
		method, path, body string
	}{
		{"PUT", foreign + "/proposal", `{"objective":"x","reason":"","activities":"","resources":""}`},
		{"GET", foreign + "/plan", ""},
		{"POST", foreign + "/plan/items", `{"title":"t","tag":"read","column":"todo","stage":"","start":0,"days":1}`},
		{"PATCH", foreign + "/plan/items/" + uuid.NewString(), `{"column":"doing"}`},
		{"DELETE", foreign + "/plan/items/" + uuid.NewString(), ""},
		{"GET", foreign + "/log", ""},
		{"POST", foreign + "/log", `{"text":"hi"}`},
		{"POST", foreign + "/coach", `{"scope":"forming","user_input":"帮我想想"}`},
	}
	for _, c := range cases {
		var r *http.Request
		if c.body == "" {
			r = httptest.NewRequest(c.method, c.path, nil)
		} else {
			r = httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(r, cookie))
		if rr.Code != 404 {
			t.Fatalf("%s %s: want 404, got %d — %s", c.method, c.path, rr.Code, rr.Body.String())
		}
	}
}

// TestPatchPlanItem_ForeignItem404 proves an item id that belongs to a
// DIFFERENT owned project cannot be bumped through another project's path (the
// GetPlanItem(id, projectID) scope hides it as 404).
func TestPatchPlanItem_ForeignItem404(t *testing.T) {
	h, cookie, _ := planTestHandler(t)

	// Create an item under a second, freshly-created project owned by the same
	// user, then try to PATCH it through the seeded project's path.
	otherPID := createProjectForTest(t, h, cookie)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+otherPID+"/plan/items",
		strings.NewReader(`{"title":"别处的任务","tag":"read","column":"todo","stage":"","start":0,"days":1}`)), cookie))
	if rr.Code != 201 {
		t.Fatalf("create in other project: %d — %s", rr.Code, rr.Body.String())
	}
	var created struct {
		Item struct {
			ID string `json:"id"`
		} `json:"item"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PATCH", "/api/v1/projects/"+seedProjectID+"/plan/items/"+created.Item.ID,
		strings.NewReader(`{"column":"done"}`)), cookie))
	if rr.Code != 404 {
		t.Fatalf("patch foreign item through wrong project: want 404, got %d — %s", rr.Code, rr.Body.String())
	}
}

func TestCoach_HappyPath(t *testing.T) {
	h, cookie, q := planTestHandler(t)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+seedProjectID+"/coach",
		strings.NewReader(`{"scope":"forming","user_input":"我想研究中国的可持续发展但不知道从哪开始"}`)), cookie))
	if rr.Code != 200 {
		t.Fatalf("coach: %d — %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Reply string `json:"reply"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode coach: %v — %s", err, rr.Body.String())
	}
	// The fake provider scripts a non-empty reply — the fallback must NOT fire.
	if out.Reply == "" {
		t.Fatalf("empty reply")
	}

	// The call must be metered as surface=studio purpose=coach.
	calls, err := q.ListLLMCallsByProject(context.Background(), pgUUID(uuid.MustParse(seedProjectID)))
	if err != nil {
		t.Fatalf("ListLLMCallsByProject: %v", err)
	}
	var found bool
	for _, c := range calls {
		if c.Purpose == "coach" && c.Surface == "studio" {
			found = true
			if c.PromptTokens == 0 && c.CompletionTokens == 0 {
				t.Fatalf("coach call not metered with token counts: %+v", c)
			}
		}
	}
	if !found {
		t.Fatalf("no purpose=coach llm_call recorded, got %d calls", len(calls))
	}

	// Empty user_input -> 400.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+seedProjectID+"/coach",
		strings.NewReader(`{"scope":"forming","user_input":"   "}`)), cookie))
	if rr.Code != 400 {
		t.Fatalf("empty user_input: want 400, got %d", rr.Code)
	}
}

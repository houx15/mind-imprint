package api_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
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

	// (b) POST /exploration/dig → 200 with a (canned, empty) candidates array.
	digBody, _ := json.Marshal(map[string]string{"keyword": "sustainability"})
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+demo.ID.String()+"/exploration/dig", bytes.NewReader(digBody)), otherCookie))
	if rr.Code != 200 {
		t.Fatalf("demo POST /exploration/dig: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var dig struct {
		Candidates []any `json:"candidates"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &dig); err != nil {
		t.Fatalf("decode dig reply: %v", err)
	}
	if dig.Candidates == nil {
		t.Fatalf("demo POST /exploration/dig: want a candidates array (even if empty), got null — %s", rr.Body.String())
	}

	// The demo path spent nothing and wrote nothing: no chat_message, no llm_call.
	if got := countAll("chat_message"); got != beforeMsgs {
		t.Fatalf("demo endpoints wrote chat_message rows: before=%d after=%d", beforeMsgs, got)
	}
	if got := countAll("llm_call"); got != beforeLLM {
		t.Fatalf("demo endpoints wrote llm_call rows: before=%d after=%d", beforeLLM, got)
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

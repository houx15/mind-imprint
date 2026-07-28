package api_test

// workspace_write_test.go — Slice 4 (Write room): the outline (PUT replaces the
// whole set, GET reads it back in position order) and the draft read side
// (GET /draft reflects a prior PUT /buffer). All plain owned-project REST: no
// model call, no entitlement gate. Ownership is hidden as 404.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

type outlineNodeWire struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Depth    int32  `json:"depth"`
	Position int32  `json:"position"`
}

func writeTestHandler(t *testing.T) (http.Handler, *http.Cookie) {
	t.Helper()
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool) // Phoebe, owns materialsTestProjectID
	return h, cookie
}

func putOutline(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, body string) []outlineNodeWire {
	t.Helper()
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("PUT", "/api/v1/projects/"+projectID+"/outline",
		strings.NewReader(body)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT outline = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Nodes []outlineNodeWire `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode PUT outline response: %v — %s", err, rec.Body.String())
	}
	return resp.Nodes
}

func getOutline(t *testing.T, h http.Handler, cookie *http.Cookie, projectID string) []outlineNodeWire {
	t.Helper()
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/outline", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET outline = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Nodes []outlineNodeWire `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode GET outline response: %v — %s", err, rec.Body.String())
	}
	return resp.Nodes
}

// TestPutOutline_RoundTripsWithNestedDepths — a PUT with nested depths persists
// and a following GET returns the same nodes in order, with server-assigned
// positions (= array index), fresh ids, and depth preserved.
func TestPutOutline_RoundTripsWithNestedDepths(t *testing.T) {
	h, cookie := writeTestHandler(t)
	pid := materialsTestProjectID

	put := putOutline(t, h, cookie, pid, `{"nodes":[
		{"text":"引言","depth":0},
		{"text":"中国的可再生能源投资","depth":1},
		{"text":"具体：光伏装机全球第一","depth":2},
		{"text":"反例：碳排放总量全球第一","depth":1},
		{"text":"结论","depth":0}
	]}`)
	if len(put) != 5 {
		t.Fatalf("PUT returned %d nodes, want 5", len(put))
	}

	got := getOutline(t, h, cookie, pid)
	if len(got) != 5 {
		t.Fatalf("GET returned %d nodes, want 5", len(got))
	}
	wantText := []string{"引言", "中国的可再生能源投资", "具体：光伏装机全球第一", "反例：碳排放总量全球第一", "结论"}
	wantDepth := []int32{0, 1, 2, 1, 0}
	for i, n := range got {
		if n.Text != wantText[i] {
			t.Errorf("node[%d].text = %q, want %q", i, n.Text, wantText[i])
		}
		if n.Depth != wantDepth[i] {
			t.Errorf("node[%d].depth = %d, want %d", i, n.Depth, wantDepth[i])
		}
		if n.Position != int32(i) {
			t.Errorf("node[%d].position = %d, want %d", i, n.Position, i)
		}
		if n.ID == "" {
			t.Errorf("node[%d] has empty id", i)
		}
	}
	// The PUT response must match what GET returns (same ids, same order).
	for i := range got {
		if put[i].ID != got[i].ID {
			t.Errorf("PUT/GET id mismatch at %d: %q vs %q", i, put[i].ID, got[i].ID)
		}
	}
}

// TestPutOutline_ReplaceSemanticsShrinks — a second PUT with fewer nodes
// replaces the WHOLE set (delete-then-insert), so the outline shrinks; stale
// rows from the first PUT do not linger.
func TestPutOutline_ReplaceSemanticsShrinks(t *testing.T) {
	h, cookie := writeTestHandler(t)
	pid := materialsTestProjectID

	putOutline(t, h, cookie, pid, `{"nodes":[
		{"text":"A","depth":0},
		{"text":"B","depth":1},
		{"text":"C","depth":1},
		{"text":"D","depth":0}
	]}`)
	if n := getOutline(t, h, cookie, pid); len(n) != 4 {
		t.Fatalf("after first PUT, GET = %d nodes, want 4", len(n))
	}

	putOutline(t, h, cookie, pid, `{"nodes":[
		{"text":"只剩一条","depth":0}
	]}`)
	got := getOutline(t, h, cookie, pid)
	if len(got) != 1 {
		t.Fatalf("after shrinking PUT, GET = %d nodes, want 1 (replace semantics)", len(got))
	}
	if got[0].Text != "只剩一条" || got[0].Position != 0 {
		t.Fatalf("surviving node = %+v, want the single new node at position 0", got[0])
	}
}

// TestPutOutline_ClampsDepth — depth out of the 0..2 range is clamped, never
// rejected.
func TestPutOutline_ClampsDepth(t *testing.T) {
	h, cookie := writeTestHandler(t)
	pid := materialsTestProjectID

	putOutline(t, h, cookie, pid, `{"nodes":[
		{"text":"太深","depth":9},
		{"text":"负的","depth":-3}
	]}`)
	got := getOutline(t, h, cookie, pid)
	if len(got) != 2 {
		t.Fatalf("GET = %d nodes, want 2", len(got))
	}
	if got[0].Depth != 2 {
		t.Errorf("depth 9 clamped to %d, want 2", got[0].Depth)
	}
	if got[1].Depth != 0 {
		t.Errorf("depth -3 clamped to %d, want 0", got[1].Depth)
	}
}

// TestGetDraft_ReflectsPriorBuffer — GET /draft returns the content a prior
// PUT /buffer wrote; an empty draft (no buffer row) reads as "".
func TestGetDraft_ReflectsPriorBuffer(t *testing.T) {
	h, cookie := writeTestHandler(t)
	pid := materialsTestProjectID

	// Fresh project: no buffer row yet → "".
	otherPID := createProjectForTest(t, h, cookie)
	recEmpty := httptest.NewRecorder()
	h.ServeHTTP(recEmpty, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+otherPID+"/draft", nil), cookie))
	if recEmpty.Code != http.StatusOK {
		t.Fatalf("GET draft (no buffer) = %d, want 200; body=%s", recEmpty.Code, recEmpty.Body)
	}
	var emptyResp struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(recEmpty.Body.Bytes(), &emptyResp); err != nil {
		t.Fatalf("decode empty draft: %v", err)
	}
	if emptyResp.Content != "" {
		t.Fatalf("draft with no buffer = %q, want empty string", emptyResp.Content)
	}

	// Write the buffer, then read the draft back.
	recPut := httptest.NewRecorder()
	h.ServeHTTP(recPut, withCookie(httptest.NewRequest("PUT", "/api/v1/projects/"+pid+"/buffer",
		strings.NewReader(`{"content":"中国的能源转型正在重塑全球格局。"}`)), cookie))
	if recPut.Code != http.StatusNoContent {
		t.Fatalf("PUT buffer = %d, want 204; body=%s", recPut.Code, recPut.Body)
	}

	recGet := httptest.NewRecorder()
	h.ServeHTTP(recGet, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+pid+"/draft", nil), cookie))
	if recGet.Code != http.StatusOK {
		t.Fatalf("GET draft = %d, want 200; body=%s", recGet.Code, recGet.Body)
	}
	var resp struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(recGet.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if resp.Content != "中国的能源转型正在重塑全球格局。" {
		t.Fatalf("draft content = %q, want the buffer content", resp.Content)
	}
}

// TestWorkspaceWrite_Ownership404 — every Slice 4 route hides a foreign (here:
// non-existent, same 404-no-leak path loadOwnedProject enforces) project as
// not-found rather than leaking its existence.
func TestWorkspaceWrite_Ownership404(t *testing.T) {
	h, cookie := writeTestHandler(t)
	foreign := "/api/v1/projects/00000000-0000-0000-0000-0000000009ff"

	cases := []struct {
		method, path, body string
	}{
		{"GET", foreign + "/outline", ""},
		{"PUT", foreign + "/outline", `{"nodes":[{"text":"x","depth":0}]}`},
		{"GET", foreign + "/draft", ""},
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
		if rr.Code != http.StatusNotFound {
			t.Fatalf("%s %s: want 404, got %d — %s", c.method, c.path, rr.Code, rr.Body.String())
		}
	}
}

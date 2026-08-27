package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// writing_outline_test.go — Task 5: 大纲 (outline). Reuses writing_turn_test.go's
// helpers (createWritingAtomHTTP, writingTextStubProvider,
// writingStreamErrorProvider, liteHandlerWithProvider) — same package
// (api_test), same lite-writing test harness.

type writingOutlineItem struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Role     string `json:"role"`
	Depth    int32  `json:"depth"`
	Position int32  `json:"position"`
}

func getWritingOutlineHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id string) []writingOutlineItem {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/outline", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET outline = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Outline []writingOutlineItem `json:"outline"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode outline: %v — body=%s", err, rec.Body)
	}
	return out.Outline
}

func putWritingOutlineHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/outline", strings.NewReader(body)), cookie))
	return rec
}

// postWritingPlanTurn drives the planning conversation — the endpoint that
// replaced BOTH the retired POST /outline/generate and the short-lived
// structure picker that briefly stood in for it. The ownership test below
// guards this one, so that assertion keeps testing ownership instead of
// quietly passing because the route it named no longer exists at all.
func postWritingPlanTurn(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/plan/turn", strings.NewReader(body)), cookie))
	return rec
}

// TestWritingOutline_PutIsFullReplaceNotMerge — the task's central assertion:
// PUT is 全量替换 ("full replace"), never 合并 ("merge"). Three items, then
// two — only the second PUT's two items survive, position = array index.
func TestWritingOutline_PutIsFullReplaceNotMerge(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	first := `{"outline":[{"text":"引言","depth":0},{"text":"论点一：碳排放","depth":1},{"text":"结论","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, first); rec.Code != http.StatusOK {
		t.Fatalf("first PUT = %d; body=%s", rec.Code, rec.Body)
	}
	after1 := getWritingOutlineHTTP(t, h, cookie, id)
	if len(after1) != 3 {
		t.Fatalf("after first PUT = %d items, want 3: %+v", len(after1), after1)
	}

	second := `{"outline":[{"text":"新引言","depth":0},{"text":"新结论","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, second); rec.Code != http.StatusOK {
		t.Fatalf("second PUT = %d; body=%s", rec.Code, rec.Body)
	}
	after2 := getWritingOutlineHTTP(t, h, cookie, id)
	if len(after2) != 2 {
		t.Fatalf("after second PUT = %d items, want 2 (full replace, not merge): %+v", len(after2), after2)
	}
	if after2[0].Text != "新引言" || after2[0].Position != 0 {
		t.Fatalf("item 0 = %+v, want 新引言 at position 0", after2[0])
	}
	if after2[1].Text != "新结论" || after2[1].Position != 1 {
		t.Fatalf("item 1 = %+v, want 新结论 at position 1", after2[1])
	}
	// Neither surviving row is one of the first PUT's ids — a real replace
	// mints fresh rows, it does not edit the old ones in place.
	for _, it := range after2 {
		for _, old := range after1 {
			if it.ID == old.ID {
				t.Fatalf("row %+v reused an id from the replaced outline %+v — not a real full replace", it, old)
			}
		}
	}
}

// TestWritingOutline_DepthClampedTo0To2 — depth is clamped, never rejected.
func TestWritingOutline_DepthClampedTo0To2(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	body := `{"outline":[{"text":"太深","depth":9},{"text":"太浅","depth":-3}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, body); rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d; body=%s", rec.Code, rec.Body)
	}
	items := getWritingOutlineHTTP(t, h, cookie, id)
	if len(items) != 2 || items[0].Depth != 2 || items[1].Depth != 0 {
		t.Fatalf("depths = %+v, want [2, 0] (clamped)", items)
	}
}

// TestWritingOutline_PutEmptyArrayClearsOutline — an empty PUT is a valid
// full replace too: it clears the outline entirely.
func TestWritingOutline_PutEmptyArrayClearsOutline(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := putWritingOutlineHTTP(t, h, cookie, id, `{"outline":[{"text":"一","depth":0}]}`); rec.Code != http.StatusOK {
		t.Fatalf("first PUT = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := putWritingOutlineHTTP(t, h, cookie, id, `{"outline":[]}`); rec.Code != http.StatusOK {
		t.Fatalf("empty PUT = %d; body=%s", rec.Code, rec.Body)
	}
	items := getWritingOutlineHTTP(t, h, cookie, id)
	if len(items) != 0 {
		t.Fatalf("outline after empty PUT = %+v, want none", items)
	}
}

// TestWritingOutline_RequiresOwnWriting — cross-kind isolation, mirrors
// TestWritingTurn_RequiresOwnWriting: a nonexistent/foreign id is a flat 404.
func TestWritingOutline_RequiresOwnWriting(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	bogus := "00000000-0000-0000-0000-000000000000"
	if rec := putWritingOutlineHTTP(t, h, cookie, bogus, `{"outline":[]}`); rec.Code != http.StatusNotFound {
		t.Fatalf("PUT on nonexistent writing = %d, want 404; body=%s", rec.Code, rec.Body)
	}
	if rec := postWritingPlanTurn(t, h, cookie, bogus, `{"text":"在吗"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("plan turn on nonexistent writing = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

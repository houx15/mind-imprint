package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func putSource(t *testing.T, h http.Handler, cookie *http.Cookie, id, text string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"text": text})
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/source", strings.NewReader(string(body))), cookie))
	return rec
}

// The source URL is rendered as a link on the teacher's item page, so only
// http/https may be stored (final review F1). A javascript: URL is refused
// before any fetch or write.
func TestPutSource_RejectsNonHTTPURL(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	for _, u := range []string{"javascript:alert(1)", "data:text/html,<b>x</b>", "ftp://example.org/a"} {
		body, _ := json.Marshal(map[string]string{"url": u, "text": "正文第一段。"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(
			httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/source", strings.NewReader(string(body))), cookie))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_url") {
			t.Fatalf("PUT url=%q = %d body=%s, want 400 invalid_url", u, rec.Code, rec.Body)
		}
	}
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM reading_source WHERE atom_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("reading_source rows = %d, want 0 after refused URLs", n)
	}

	// An https URL with a pasted body is still accepted.
	body, _ := json.Marshal(map[string]string{"url": "https://example.org/a", "text": "正文第一段。"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/source", strings.NewReader(string(body))), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT https url = %d body=%s, want 200", rec.Code, rec.Body)
	}
}

func TestPutSource_StoresAndSegments(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := putSource(t, h, cookie, id, "太阳能装机十年增长十倍。\n\n但储能仍是瓶颈。")
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT source = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Blocks []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Blocks) != 2 || out.Blocks[0].ID != "b1" {
		t.Fatalf("blocks = %+v, want two starting at b1", out.Blocks)
	}
}

func TestPutSource_RejectsEmptyBody(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := putSource(t, h, cookie, id, "   \n\n  ")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

func TestPutSource_ReplacesOnSecondPut(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	putSource(t, h, cookie, id, "第一版。")
	putSource(t, h, cookie, id, "第二版。\n\n多了一段。")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/source", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET source = %d, want 200", rec.Code)
	}
	var out struct {
		Blocks []struct {
			Text string `json:"text"`
		} `json:"blocks"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Blocks) != 2 || out.Blocks[0].Text != "第二版。" {
		t.Fatalf("blocks = %+v, want the second version", out.Blocks)
	}
}

func TestGetSource_BeforePasteIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/source", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET source before paste = %d, want 404", rec.Code)
	}
}

func TestPutSource_ForeignAtomIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := putSource(t, h, cookie, "00000000-0000-0000-0000-0000000009ff", "正文")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign atom = %d, want 404", rec.Code)
	}
}

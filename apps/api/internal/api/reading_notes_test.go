package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrief_EmptyBeforeSetThenRoundTrips(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	// A reading with no brief yet is a legitimate state, not a 404 — the brief
	// is optional context and the room asks for it the moment it opens, so a
	// 404 here would make the frontend treat "normal" as "broken".
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/brief", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET brief before set = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	body := strings.NewReader(`{"phaseTag":null,"readingReason":"想弄清储能瓶颈","readingFocus":"看数据口径"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/brief", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT brief = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/brief", nil), cookie))
	var out struct {
		ReadingReason string `json:"readingReason"`
		ReadingFocus  string `json:"readingFocus"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.ReadingReason != "想弄清储能瓶颈" || out.ReadingFocus != "看数据口径" {
		t.Fatalf("brief = %+v, want what we just PUT", out)
	}
}

func TestTakeaway_EmptyBeforeSetThenRoundTrips(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/takeaway", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET takeaway before set = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	body := strings.NewReader(`{"text":"作者其实没证明因果，只给了相关。"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/takeaway", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT takeaway = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/takeaway", nil), cookie))
	var out struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if !strings.Contains(out.Text, "没证明因果") {
		t.Fatalf("takeaway = %q, want what we PUT", out.Text)
	}
}

func TestAnnotations_AppendAndList(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"blockId":"b1","span":{"start":0,"end":6},"quote":"太阳能装机","note":"这个数字要查来源"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/annotations", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST annotation = %d, want 201; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/annotations", nil), cookie))
	var out struct {
		Annotations []struct {
			BlockID string `json:"blockId"`
			Note    string `json:"note"`
		} `json:"annotations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Annotations) != 1 || out.Annotations[0].BlockID != "b1" {
		t.Fatalf("annotations = %+v, want the one we appended", out.Annotations)
	}
}

func TestAnnotations_RequiresBlockID(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"blockId":"","span":{"start":0,"end":1},"quote":"x","note":"y"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/annotations", body), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("blank blockId = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

func TestAnnotations_ForeignAtomIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"blockId":"b1","span":{"start":0,"end":1},"quote":"x","note":"y"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/00000000-0000-0000-0000-0000000009ff/annotations", body), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign atom = %d, want 404", rec.Code)
	}
}

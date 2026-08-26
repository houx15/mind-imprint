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

// TestBrief_PutIsFullReplacementNotPatch guards the second load-bearing
// decision in this task: PUT must replace all three fields wholesale, not
// merge onto the existing row. A buggy read-modify-write "patch"
// implementation would pass TestBrief_EmptyBeforeSetThenRoundTrips (there's
// no prior value there for a bad merge to preserve) but would fail HERE: the
// second PUT sends an empty readingReason where the first PUT had left a
// non-empty one, and under partial-patch semantics an empty string is
// commonly (wrongly) treated as "field not provided" and the stale value
// survives. Under full-replace it must come back genuinely empty.
func TestBrief_PutIsFullReplacementNotPatch(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	firstBody := strings.NewReader(`{"phaseTag":"S1","readingReason":"想弄清储能瓶颈","readingFocus":"看数据口径"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/brief", firstBody), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("first PUT brief = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// Second PUT: readingFocus gets a NEW value, readingReason is sent EMPTY
	// (was non-empty a moment ago), phaseTag changes too.
	rec = httptest.NewRecorder()
	secondBody := strings.NewReader(`{"phaseTag":"S2","readingReason":"","readingFocus":"看数据来源是否可靠"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/brief", secondBody), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("second PUT brief = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/brief", nil), cookie))
	var out struct {
		PhaseTag      *string `json:"phaseTag"`
		ReadingReason string  `json:"readingReason"`
		ReadingFocus  string  `json:"readingFocus"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	// The assertion that matters: readingReason must be genuinely empty now,
	// not the stale "想弄清储能瓶颈" from the first PUT.
	if out.ReadingReason != "" {
		t.Fatalf("readingReason = %q after PUTting empty, want \"\" (full replace, not patch)", out.ReadingReason)
	}
	if out.ReadingFocus != "看数据来源是否可靠" {
		t.Fatalf("readingFocus = %q, want the second PUT's value", out.ReadingFocus)
	}
	if out.PhaseTag == nil || *out.PhaseTag != "S2" {
		t.Fatalf("phaseTag = %v, want \"S2\"", out.PhaseTag)
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

// TestAnnotations_QuoteAndNoteMayBeEmpty covers the other half of the
// blockId-required contract: a student may anchor a note before writing it,
// so quote and note must both be allowed to be empty — only blockId is
// mandatory.
func TestAnnotations_QuoteAndNoteMayBeEmpty(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"blockId":"b2","span":{"start":3,"end":9},"quote":"","note":""}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/annotations", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST annotation with empty quote/note = %d, want 201; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/annotations", nil), cookie))
	var out struct {
		Annotations []struct {
			BlockID string `json:"blockId"`
			Quote   string `json:"quote"`
			Note    string `json:"note"`
		} `json:"annotations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Annotations) != 1 || out.Annotations[0].BlockID != "b2" {
		t.Fatalf("annotations = %+v, want the one we appended", out.Annotations)
	}
	if out.Annotations[0].Quote != "" || out.Annotations[0].Note != "" {
		t.Fatalf("annotation = %+v, want empty quote and note preserved", out.Annotations[0])
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

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// reading_hardening_test.go — the whole-branch review's four smaller
// evidence-integrity fixes, each of which is invisible from inside the task
// that introduced it:
//
//	· the JSON-`null` boundary hole, in the two places it was still open
//	· last_activity_at, so 上次读到 means what it says
//	· PUT /source refusing to swap the article out from under live anchors
//	· POST /finish being a no-op the second time instead of a re-stamp

func postJSONReq(t *testing.T, h http.Handler, cookie *http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", path, strings.NewReader(body)), cookie))
	return rec
}

func lastActivityOf(t *testing.T, pool *pgxpool.Pool, atomID string) time.Time {
	t.Helper()
	var ts time.Time
	if err := pool.QueryRow(t.Context(),
		`SELECT last_activity_at FROM atom WHERE id = $1`, mustUUID(atomID)).Scan(&ts); err != nil {
		t.Fatalf("read last_activity_at: %v", err)
	}
	return ts
}

// --- 4 · the JSON-null boundary hole --------------------------------------

// TestCreateAnnotation_RejectsNullSpan — json.Unmarshal([]byte("null"), &m)
// returns NO error and leaves the map nil, so the unguarded unmarshal
// liteCreateAnnotation used to do waved a literal `null` straight into the
// jsonb column. It survives every Go read and only detonates later, at a
// strict client-side parse — the exact failure that once blanked a teacher
// report.
func TestCreateAnnotation_RejectsNullSpan(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := postJSONReq(t, h, cookie, "/api/v1/readings/"+id+"/annotations",
		`{"blockId":"b1","span":null,"quote":"x","note":"y"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("span:null = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "invalid_span") {
		t.Fatalf("want invalid_span, got %s", rec.Body)
	}

	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM atom_annotation WHERE atom_id = $1`, mustUUID(id)).Scan(&n); err != nil {
		t.Fatalf("count annotations: %v", err)
	}
	if n != 0 {
		t.Fatalf("atom_annotation rows = %d, want 0 — a literal null span must never be stored", n)
	}
}

// TestCardSubmit_RejectsNullAnchors — the third spelling of the same check
// (`arr == nil`) lived here. A `null` anchors array stored on a submitted card
// is worse than the annotation case: 阅读成果 re-parses anchors on every room
// reload.
func TestCardSubmit_RejectsNullAnchors(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	rec := postJSONReq(t, h, cookie, "/api/v1/readings/"+atomID+"/cards/"+cardID+"/submit",
		`{"fieldValues":{},"eventTrace":[],"anchors":null}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("anchors:null = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "invalid_anchors") {
		t.Fatalf("want invalid_anchors, got %s", rec.Body)
	}

	var status string
	if err := pool.QueryRow(t.Context(),
		`SELECT status FROM atom_card WHERE id = $1`, mustUUID(cardID)).Scan(&status); err != nil {
		t.Fatalf("read card status: %v", err)
	}
	if status != "proposed" {
		t.Fatalf("card status = %q, want it left untouched at proposed", status)
	}
}

// --- 5 · last_activity_at --------------------------------------------------

// TestReading_LastActivityAdvancesWhileReading — the bug: reading.updated_at
// is written ONLY by rename and finish, yet the history panel sorts unfinished
// by it and labels it 上次读到, and the landing's 「你有 N 篇还没读完」 opens
// the most recently touched one. So an hour of actual reading moved neither.
//
// The fix puts last-activity on the ATOM (0098), where the writing kind
// inherits it instead of inventing its own, and bumps it at the single write
// chokepoint every lite per-id route funnels through.
func TestReading_LastActivityAdvancesWhileReading(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	atCreate := lastActivityOf(t, pool, id)

	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	afterSource := lastActivityOf(t, pool, id)
	if !afterSource.After(atCreate) {
		t.Fatalf("last_activity_at after pasting the article = %v, want later than %v",
			afterSource, atCreate)
	}

	// A margin note is the quietest thing she can do to a reading, and it is
	// exactly the kind of hour-long activity the old column never saw.
	if rec := postJSONReq(t, h, cookie, "/api/v1/readings/"+id+"/annotations",
		`{"blockId":"b1","span":{"start":0,"end":3},"quote":"中国的","note":"存疑"}`); rec.Code != http.StatusCreated {
		t.Fatalf("POST annotation = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	afterNote := lastActivityOf(t, pool, id)
	if !afterNote.After(afterSource) {
		t.Fatalf("last_activity_at after a margin note = %v, want later than %v",
			afterNote, afterSource)
	}

	// …and reading.updated_at must NOT have moved: it still means "the title /
	// status metadata changed", and the two consumers now read the other field.
	var updated time.Time
	if err := pool.QueryRow(t.Context(),
		`SELECT updated_at FROM reading WHERE atom_id = $1`, mustUUID(id)).Scan(&updated); err != nil {
		t.Fatalf("read reading.updated_at: %v", err)
	}
	if updated.After(afterSource) {
		t.Fatalf("reading.updated_at = %v moved on a plain read-room write; "+
			"last-activity belongs on the atom", updated)
	}

	// The list is what the landing actually paints, so the field has to be on
	// the wire — and hasSource has to survive the LEFT JOIN that replaced the
	// per-reading GetReadingSource.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET readings = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var listed struct {
		Readings []struct {
			ID             string `json:"id"`
			HasSource      bool   `json:"hasSource"`
			LastActivityAt string `json:"lastActivityAt"`
		} `json:"readings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode readings: %v — body=%s", err, rec.Body)
	}
	if len(listed.Readings) != 1 {
		t.Fatalf("readings = %+v, want exactly one", listed.Readings)
	}
	if !listed.Readings[0].HasSource {
		t.Fatalf("hasSource = false after pasting an article — the LEFT JOIN that "+
			"replaced the N+1 must answer the same question; row=%+v", listed.Readings[0])
	}
	if listed.Readings[0].LastActivityAt == "" {
		t.Fatalf("lastActivityAt missing from the list DTO; row=%+v", listed.Readings[0])
	}
}

// --- 6 · the article cannot be swapped under live anchors ------------------

// TestPutSource_RefusedOnceAnchored — block ids are POSITIONAL and anchors
// carry rune offsets, so a replacement does not orphan the evidence, it
// silently re-points it at unrelated prose. 铁律④ makes those rows evidence;
// 409 is the honest answer.
func TestPutSource_RefusedOnceAnchored(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("first put source = %d; body=%s", rec.Code, rec.Body)
	}
	// A second paste is still fine while nothing points into the text — that
	// is the ordinary "pasted the wrong thing" correction.
	if rec := putSource(t, h, cookie, id, turnArticle+"\n\n第三段。"); rec.Code != http.StatusOK {
		t.Fatalf("re-paste with no anchors yet = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	if rec := postJSONReq(t, h, cookie, "/api/v1/readings/"+id+"/annotations",
		`{"blockId":"b2","span":{"start":0,"end":4},"quote":"但同一时期","note":"反例"}`); rec.Code != http.StatusCreated {
		t.Fatalf("POST annotation = %d, want 201; body=%s", rec.Code, rec.Body)
	}

	rec := putSource(t, h, cookie, id, "完全不同的一篇文章。\n\n第二段也不同。")
	if rec.Code != http.StatusConflict {
		t.Fatalf("replace under a live anchor = %d, want 409; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "source_locked") {
		t.Fatalf("want source_locked, got %s", rec.Body)
	}

	// And the article really is untouched.
	grec := httptest.NewRecorder()
	h.ServeHTTP(grec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/source", nil), cookie))
	if !strings.Contains(grec.Body.String(), "太阳能装机量") {
		t.Fatalf("source changed despite the 409; body=%s", grec.Body)
	}
}

// --- 7 · a degraded review is marked as one ---------------------------------

// TestEvaluateSelection_MarksDegradedFallback — agent.EvaluateSelection NEVER
// returns an error; on a resolver failure, a provider failure or an
// unparseable reply it silently degrades to fallbackEval's canned text
// ("你选了这句作为证据。"). That text was then written to framework_fill and
// read straight back as her `finding` — and the known MaxTokens truncation
// makes this the COMMON path, not a rare one. A report generated off the
// process record would have counted a canned sentence as her own reading
// (铁律①/④). It has to say so on the row.
func TestEvaluateSelection_MarksDegradedFallback(t *testing.T) {
	// A reply the eval parser cannot make sense of — the truncation case.
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider("这不是 JSON。"))
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	cid := seedCard(t, pool, id, "b1")

	rec := postJSONReq(t, h, cookie, "/api/v1/readings/"+id+"/cards/"+cid+"/evaluate",
		`{"block_id":"b2","start":0,"end":20,"quote":"`+disconnectStudentQuote+`","dimension":"currency"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("evaluate = %d, want 200 (the loop stays alive); body=%s", rec.Code, rec.Body)
	}
	var wire struct {
		Finding  string `json:"finding"`
		Degraded bool   `json:"degraded"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &wire); err != nil {
		t.Fatalf("decode evaluate: %v — body=%s", err, rec.Body)
	}
	if !wire.Degraded {
		t.Fatalf("degraded = false on a fallback review (finding=%q) — a review no "+
			"model produced must be marked, or a report will read it as her work", wire.Finding)
	}

	var raw []byte
	if err := pool.QueryRow(t.Context(),
		`SELECT framework_fill FROM atom_card WHERE id = $1`, mustUUID(cid)).Scan(&raw); err != nil {
		t.Fatalf("read framework_fill: %v", err)
	}
	var stored struct {
		Degraded bool `json:"degraded"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("decode framework_fill: %v — raw=%s", err, raw)
	}
	if !stored.Degraded {
		t.Fatalf("framework_fill.degraded = false — the PERSISTED payload is what a "+
			"report reads; raw=%s", raw)
	}
}

// --- 9 · finish is a no-op the second time ---------------------------------

// TestFinishReading_SecondCallDoesNotRestamp — idempotent must mean no-op.
// finished_at is a fact about when she finished; a replayed request (a
// double-click, a retry, a stale tab) must not move it hours later.
func TestFinishReading_SecondCallDoesNotRestamp(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/takeaway",
		strings.NewReader(`{"text":"我的收获。"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("put takeaway = %d; body=%s", rec.Code, rec.Body)
	}

	if r1 := postJSONReq(t, h, cookie, "/api/v1/readings/"+id+"/finish", "{}"); r1.Code != http.StatusOK {
		t.Fatalf("finish #1 = %d, want 200; body=%s", r1.Code, r1.Body)
	}
	var first time.Time
	if err := pool.QueryRow(t.Context(),
		`SELECT finished_at FROM reading WHERE atom_id = $1`, mustUUID(id)).Scan(&first); err != nil {
		t.Fatalf("read finished_at: %v", err)
	}

	if r2 := postJSONReq(t, h, cookie, "/api/v1/readings/"+id+"/finish", "{}"); r2.Code != http.StatusOK {
		t.Fatalf("finish #2 = %d, want 200 (still idempotent); body=%s", r2.Code, r2.Body)
	}
	var second time.Time
	if err := pool.QueryRow(t.Context(),
		`SELECT finished_at FROM reading WHERE atom_id = $1`, mustUUID(id)).Scan(&second); err != nil {
		t.Fatalf("read finished_at: %v", err)
	}
	if !second.Equal(first) {
		t.Fatalf("finished_at moved on a second POST /finish: %v → %v — a completion "+
			"time must not drift after the fact", first, second)
	}
}

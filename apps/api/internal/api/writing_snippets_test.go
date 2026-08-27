package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/gateway"
)

// writing_snippets_test.go — Task 6: 段落 (paragraphs) + the English
// exemplar. Reuses writing_turn_test.go's harness (writingTextStubProvider,
// writingStreamErrorProvider, liteHandlerWithProvider) and
// writing_outline_test.go's createWritingAtomHTTP — same package (api_test),
// same lite-writing test harness.
//
// TestWritingSnippetExemplar_NeverPersisted is the task's central assertion —
// 铁律① is upheld or broken right here. See its comment for exactly what it
// checks.

// createWritingAtomHTTPLang is createWritingAtomHTTP with an explicit lang —
// the shared helper hardcodes "zh", and the exemplar tests need an "en"
// writing. A new, differently-named helper rather than adding a parameter to
// the shared one, so every existing call site (Tasks 3-5's tests) stays
// untouched.
func createWritingAtomHTTPLang(t *testing.T, h http.Handler, cookie *http.Cookie, idea, lang string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"idea":"` + idea + `","lang":"` + lang + `"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create writing = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	return out.ID
}

type writingSnippetItem struct {
	ID        string  `json:"id"`
	OutlineID *string `json:"outlineId"`
	Position  int32   `json:"position"`
	Text      string  `json:"text"`
	UpdatedAt string  `json:"updatedAt"`
}

func getWritingSnippetsHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id string) []writingSnippetItem {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/snippets", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET snippets = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Snippets []writingSnippetItem `json:"snippets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode snippets: %v — body=%s", err, rec.Body)
	}
	return out.Snippets
}

func putWritingSnippetsHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/snippets", strings.NewReader(body)), cookie))
	return rec
}

func postWritingSnippetExemplar(t *testing.T, h http.Handler, cookie *http.Cookie, id, sid string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/writings/"+id+"/snippets/"+sid+"/exemplar", nil), cookie))
	return rec
}

// countingProvider wraps another provider and counts how many times Stream
// was invoked — used to prove the exemplar endpoint never reaches the model
// at all when it should have refused before spending anything (the lang
// gate).
type countingProvider struct {
	inner gateway.Provider
	calls int
}

func (p *countingProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.calls++
	return p.inner.Stream(ctx, r, req)
}

// assertExemplarNeverPersisted is the mechanical proof of 铁律① for this
// task: it scans every table this atom could plausibly have written text
// into — atom_message, writing (title), writing_outline, writing_snippet,
// writing_draft, atom_card (both jsonb columns, cast to text), atom_annotation
// (quote + note) — for the exemplar's distinctive needle text, scoped to this
// atom_id. A hit ANYWHERE fails the test by name of the column it found it
// in. This is deliberately broader than "check writing_snippet.text is
// unchanged": the brief asks for "no table anywhere stores this exemplar",
// so the test enumerates every atom-scoped table with a free-text or jsonb
// column, not just the one an implementation would obviously reach for.
//
// If a later change started persisting the exemplar into ANY of these
// columns for THIS atom, this assertion fails — that is the whole point of
// naming it a 铁律① test rather than a plain regression test.
func assertExemplarNeverPersisted(t *testing.T, pool *pgxpool.Pool, atomID, needle string) {
	t.Helper()
	checks := []struct{ label, query string }{
		{"atom_message.content", `SELECT count(*) FROM atom_message WHERE atom_id = $1 AND content LIKE '%' || $2 || '%'`},
		{"writing.title", `SELECT count(*) FROM writing WHERE atom_id = $1 AND title LIKE '%' || $2 || '%'`},
		{"writing_outline.text", `SELECT count(*) FROM writing_outline WHERE atom_id = $1 AND text LIKE '%' || $2 || '%'`},
		{"writing_snippet.text", `SELECT count(*) FROM writing_snippet WHERE atom_id = $1 AND text LIKE '%' || $2 || '%'`},
		{"writing_draft.body", `SELECT count(*) FROM writing_draft WHERE atom_id = $1 AND body LIKE '%' || $2 || '%'`},
		{"atom_card.field_values", `SELECT count(*) FROM atom_card WHERE atom_id = $1 AND field_values::text LIKE '%' || $2 || '%'`},
		{"atom_card.event_trace", `SELECT count(*) FROM atom_card WHERE atom_id = $1 AND event_trace::text LIKE '%' || $2 || '%'`},
		{"atom_annotation.quote", `SELECT count(*) FROM atom_annotation WHERE atom_id = $1 AND quote LIKE '%' || $2 || '%'`},
		{"atom_annotation.note", `SELECT count(*) FROM atom_annotation WHERE atom_id = $1 AND note LIKE '%' || $2 || '%'`},
	}
	for _, c := range checks {
		var n int
		if err := pool.QueryRow(t.Context(), c.query, mustUUID(atomID), needle).Scan(&n); err != nil {
			t.Fatalf("scanning %s: %v", c.label, err)
		}
		if n != 0 {
			t.Fatalf("铁律① VIOLATION: exemplar text found in %s (%d matching row(s)) — "+
				"the exemplar must NEVER be written into any draft/snippet/message table", c.label, n)
		}
	}
}

// TestWritingSnippets_PutUpsertsByPositionNotFullReplace — the PUT contract
// this task's brief describes: "one fragment at a time", never a wholesale
// wipe of positions the caller did not mention (unlike outline's PUT, which
// IS a full replace — see writing_outline.go's comment on the distinction).
func TestWritingSnippets_PutUpsertsByPositionNotFullReplace(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"第一段的内容"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put position 0 = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":1,"text":"第二段的内容"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put position 1 = %d; body=%s", rec.Code, rec.Body)
	}
	after := getWritingSnippetsHTTP(t, h, cookie, id)
	if len(after) != 2 {
		t.Fatalf("snippets after two independent PUTs = %d, want 2 (upsert-by-position, not full replace): %+v", len(after), after)
	}

	var firstID string
	for _, s := range after {
		if s.Position == 0 {
			firstID = s.ID
		}
	}
	if firstID == "" {
		t.Fatalf("position 0 missing from %+v", after)
	}

	// Re-PUT position 0 with new text: edits IN PLACE (same id), same as
	// UpsertWritingSnippet's documented (atom_id, position) conflict target.
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"修改后的第一段"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("re-put position 0 = %d; body=%s", rec.Code, rec.Body)
	}
	after2 := getWritingSnippetsHTTP(t, h, cookie, id)
	if len(after2) != 2 {
		t.Fatalf("snippets after re-put = %d, want still 2 (edit in place, no duplicate): %+v", len(after2), after2)
	}
	var found bool
	for _, s := range after2 {
		if s.Position == 0 {
			found = true
			if s.ID != firstID {
				t.Fatalf("position 0's id changed from %q to %q — re-PUT must edit in place, not mint a new row", firstID, s.ID)
			}
			if s.Text != "修改后的第一段" {
				t.Fatalf("position 0 text = %q, want the re-PUT text", s.Text)
			}
		}
	}
	if !found {
		t.Fatalf("position 0 vanished after re-put: %+v", after2)
	}
	// Position 1 must be untouched — this is what makes PUT NOT a full
	// replace, unlike outline's.
	for _, s := range after2 {
		if s.Position == 1 && s.Text != "第二段的内容" {
			t.Fatalf("position 1 was disturbed by a PUT that only mentioned position 0: %+v", s)
		}
	}
}

// TestWritingSnippets_TextIsExactlyWhatWasPosted — the FIRST half of 铁律①:
// no endpoint anywhere massages, rewrites, or substitutes model output for
// what the student actually typed into PUT /snippets. This test does not
// touch the model at all (nil provider) — proving the ordinary write path is
// clean is a prerequisite before the exemplar test proves the model path
// stays out of it.
func TestWritingSnippets_TextIsExactlyWhatWasPosted(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	studentText := "This is exactly what she typed, verbatim, nothing added."
	body := `{"snippets":[{"position":0,"text":"` + studentText + `"}]}`
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, body); rec.Code != http.StatusOK {
		t.Fatalf("put = %d; body=%s", rec.Code, rec.Body)
	}

	var stored string
	if err := pool.QueryRow(t.Context(),
		`SELECT text FROM writing_snippet WHERE atom_id = $1 AND position = 0`, mustUUID(id)).Scan(&stored); err != nil {
		t.Fatalf("read snippet text: %v", err)
	}
	if stored != studentText {
		t.Fatalf("writing_snippet.text = %q, want exactly %q — the student's text must not be altered", stored, studentText)
	}
}

// TestWritingSnippetExemplar_RequiresEnglish — lang='zh' refuses with 400
// exemplar_not_available, BEFORE any model call. The counting provider
// proves the refusal happens ahead of spending anything, not merely that the
// eventual reply gets discarded.
func TestWritingSnippetExemplar_RequiresEnglish(t *testing.T) {
	inner := writingTextStubProvider(`{"exemplar":"should never be reached","prompts":["x"]}`)
	counter := &countingProvider{inner: inner}
	h, cookie, _, _ := liteHandlerWithProvider(t, counter)
	id := createWritingAtomHTTPLang(t, h, cookie, "写一篇关于气候变化的中文议论文", "zh")

	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"占位段落"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put snippet = %d; body=%s", rec.Code, rec.Body)
	}
	snippets := getWritingSnippetsHTTP(t, h, cookie, id)
	if len(snippets) != 1 {
		t.Fatalf("snippets = %+v, want 1", snippets)
	}

	rec := postWritingSnippetExemplar(t, h, cookie, id, snippets[0].ID)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("exemplar on a zh writing = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "exemplar_not_available") {
		t.Fatalf("want exemplar_not_available in body, got %s", rec.Body)
	}
	if counter.calls != 0 {
		t.Fatalf("provider was called %d time(s) for a zh writing — the lang gate must refuse BEFORE any model call", counter.calls)
	}
}

// TestWritingSnippetExemplar_ModelFailureIs502 — the honest-failure rule
// (2026-08-25, mirrored by every other writing endpoint that calls a model):
// a provider error surfaces as 502 ai_dialogue_failed, never a canned
// exemplar and never a 200.
func TestWritingSnippetExemplar_ModelFailureIs502(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, writingStreamErrorProvider{})
	id := createWritingAtomHTTPLang(t, h, cookie, "Climate change essay", "en")
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"placeholder"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put snippet = %d; body=%s", rec.Code, rec.Body)
	}
	snippets := getWritingSnippetsHTTP(t, h, cookie, id)

	rec := postWritingSnippetExemplar(t, h, cookie, id, snippets[0].ID)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("exemplar on provider failure = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "ai_dialogue_failed") {
		t.Fatalf("want ai_dialogue_failed in body, got %s", rec.Body)
	}
	var stored string
	if err := pool.QueryRow(t.Context(),
		`SELECT text FROM writing_snippet WHERE atom_id = $1 AND position = 0`, mustUUID(id)).Scan(&stored); err != nil {
		t.Fatalf("read snippet text: %v", err)
	}
	if stored != "placeholder" {
		t.Fatalf("writing_snippet.text = %q after a failed exemplar call, want unchanged %q", stored, "placeholder")
	}
}

// TestWritingSnippetExemplar_NeverPersisted — THE central test of Task 6.
// 铁律① is upheld or broken right here.
//
// What it asserts, precisely:
//  1. The exemplar text sits in its own {exemplar: "..."} field in the HTTP
//     response — never inside anything shaped like writingSnippetItem (no
//     "text" field on the response envelope itself carries it).
//  2. The guiding questions sit in their own {prompts: [...]} field.
//  3. writing_snippet.text for the snippet the exemplar was generated for is
//     BYTE-IDENTICAL, before vs. after the call — not "similar", not
//     "unchanged in the visible parts": an exact string comparison.
//  4. No table anywhere — atom_message, writing, writing_outline,
//     writing_snippet, writing_draft, atom_card (both jsonb columns),
//     atom_annotation — contains the exemplar's distinctive text, scoped to
//     this atom.
//
// This test WOULD FAIL the moment a later change started persisting the
// exemplar anywhere: if generateWritingSnippetExemplar (or anything else)
// ever called UpsertWritingSnippet, UpsertWritingDraft, AppendAtomMessage, or
// any card/annotation-creating query with the exemplar text, assertion (4)
// above finds it via the corresponding LIKE scan and fails by naming the
// exact column. If the exemplar's text somehow overwrote the snippet's own
// text (e.g. an implementation that "helpfully" fills the fragment in for
// her), assertion (3) fails first, since it compares the exact string.
func TestWritingSnippetExemplar_NeverPersisted(t *testing.T) {
	// Markers at BOTH ends, deliberately. A single trailing marker leaves this
	// guard — the one that enforces 铁律① — bypassable by the most ordinary
	// persistence bug there is: storing a truncated prefix into a
	// length-capped column, or a "first N characters" preview. That would cut
	// the marker off and the test would pass while the exemplar sat in the
	// database. Two markers mean a prefix bug trips the leading one and a
	// suffix bug trips the trailing one.
	const exemplarText = "EXEMPLAR-HEAD-8f2c1a-Photosynthesis converts sunlight into chemical energy, a process with cascading consequences for global carbon cycles-EXEMPLAR-MARKER-8f2c1a."
	stub := writingTextStubProvider(`{"exemplar":"` + exemplarText + `","prompts":["What evidence could open this paragraph?","How does this connect to your thesis?"]}`)
	h, cookie, _, pool := liteHandlerWithProvider(t, stub)
	id := createWritingAtomHTTPLang(t, h, cookie, "Photosynthesis and climate", "en")

	studentText := "My own unfinished draft sentence about plants."
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"`+studentText+`"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put snippet = %d; body=%s", rec.Code, rec.Body)
	}
	before := getWritingSnippetsHTTP(t, h, cookie, id)
	if len(before) != 1 {
		t.Fatalf("snippets before exemplar = %+v, want 1", before)
	}
	sid := before[0].ID

	rec := postWritingSnippetExemplar(t, h, cookie, id, sid)
	if rec.Code != http.StatusOK {
		t.Fatalf("exemplar = %d; body=%s", rec.Code, rec.Body)
	}

	// (1) + (2): separate fields, structurally distinct from the snippet DTO.
	var out struct {
		Exemplar string   `json:"exemplar"`
		Prompts  []string `json:"prompts"`
		Text     *string  `json:"text"` // must NOT exist on this envelope
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode exemplar response: %v — body=%s", err, rec.Body)
	}
	if out.Exemplar != exemplarText {
		t.Fatalf("response exemplar = %q, want %q verbatim", out.Exemplar, exemplarText)
	}
	if len(out.Prompts) != 2 {
		t.Fatalf("response prompts = %+v, want 2 guiding questions", out.Prompts)
	}
	if out.Text != nil {
		t.Fatalf("exemplar response carries a bare \"text\" field (%q) — it must never look like a snippet", *out.Text)
	}

	// (3): the snippet this was generated FOR is byte-identical.
	after := getWritingSnippetsHTTP(t, h, cookie, id)
	if len(after) != 1 {
		t.Fatalf("snippets after exemplar = %+v, want still 1 (no new row minted)", after)
	}
	if after[0].ID != sid {
		t.Fatalf("snippet id changed from %q to %q — exemplar must not touch the snippet row", sid, after[0].ID)
	}
	if after[0].Text != studentText {
		t.Fatalf("writing_snippet.text = %q after exemplar, want unchanged %q — "+
			"not a single character may come from the model", after[0].Text, studentText)
	}
	var storedDirect string
	if err := pool.QueryRow(t.Context(),
		`SELECT text FROM writing_snippet WHERE id = $1`, mustUUID(sid)).Scan(&storedDirect); err != nil {
		t.Fatalf("read snippet text directly: %v", err)
	}
	if storedDirect != studentText {
		t.Fatalf("direct DB read of writing_snippet.text = %q, want exactly %q", storedDirect, studentText)
	}

	// (4): no table anywhere stores the exemplar. THE assertion.
	assertExemplarNeverPersisted(t, pool, id, "EXEMPLAR-MARKER-8f2c1a")
	// The leading marker catches a truncated-prefix persistence bug that the
	// trailing one would silently miss. See the exemplarText comment.
	assertExemplarNeverPersisted(t, pool, id, "EXEMPLAR-HEAD-8f2c1a")
}

// TestWritingSnippetExemplar_UnknownSnippet404s — {sid} must name a real
// snippet belonging to this writing; a bad id is a plain 404, not a 400 or a
// silent empty-topic generation.
func TestWritingSnippetExemplar_UnknownSnippet404s(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(`{"exemplar":"x","prompts":[]}`))
	id := createWritingAtomHTTPLang(t, h, cookie, "Climate change essay", "en")
	rec := postWritingSnippetExemplar(t, h, cookie, id, "00000000-0000-0000-0000-000000000000")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("exemplar for unknown sid = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

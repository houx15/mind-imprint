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

func postWritingOutlineGenerate(t *testing.T, h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/outline/generate", nil), cookie))
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
	if rec := postWritingOutlineGenerate(t, h, cookie, bogus); rec.Code != http.StatusNotFound {
		t.Fatalf("generate on nonexistent writing = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestWritingOutlineGenerate_DoesNotAutoOverwrite — the task's other central
// assertion: generation NEVER writes to writing_outline. It returns a
// candidate (text/depth only — no id, no position, distinguishing it from a
// persisted row) that the student must confirm via PUT.
func TestWritingOutlineGenerate_DoesNotAutoOverwrite(t *testing.T) {
	stub := writingTextStubProvider(`[{"text":"引言：气候变化的紧迫性","depth":0},{"text":"论点：中国的新能源政策","depth":1},{"text":"结论","depth":0}]`)
	h, cookie, _, _ := liteHandlerWithProvider(t, stub)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")
	if rec := postWritingTurn(t, h, cookie, id, `{"text":"我想重点写中国的新能源政策"}`); rec.Code != http.StatusOK {
		t.Fatalf("seed turn = %d; body=%s", rec.Code, rec.Body)
	}
	msgsBefore := listWritingMessages(t, h, cookie, id)
	outlineBefore := getWritingOutlineHTTP(t, h, cookie, id)
	if len(outlineBefore) != 0 {
		t.Fatalf("outline before generate = %+v, want none yet", outlineBefore)
	}

	rec := postWritingOutlineGenerate(t, h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("generate = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Outline []struct {
			ID    string `json:"id"`
			Text  string `json:"text"`
			Depth int32  `json:"depth"`
		} `json:"outline"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode generate response: %v — body=%s", err, rec.Body)
	}
	if len(out.Outline) != 3 {
		t.Fatalf("candidate items = %d, want 3: %+v", len(out.Outline), out.Outline)
	}
	if out.Outline[0].Text != "引言：气候变化的紧迫性" || out.Outline[0].Depth != 0 {
		t.Fatalf("item 0 = %+v, want the model's first item verbatim", out.Outline[0])
	}
	for _, it := range out.Outline {
		if it.ID != "" {
			t.Fatalf("candidate item %+v carries an id — it must not look like a persisted row", it)
		}
	}

	// The database assertion: generate wrote NOTHING to writing_outline, and
	// touched no atom_message either (it is not a coach turn).
	outlineAfter := getWritingOutlineHTTP(t, h, cookie, id)
	if len(outlineAfter) != 0 {
		t.Fatalf("outline table after generate = %+v, want still empty — generate must never auto-persist", outlineAfter)
	}
	msgsAfter := listWritingMessages(t, h, cookie, id)
	if len(msgsAfter) != len(msgsBefore) {
		t.Fatalf("atom_message count changed by generate: before=%d after=%d", len(msgsBefore), len(msgsAfter))
	}
}

// TestWritingOutlineGenerate_DerivesFromStudentMessagesOnly — the prompt sent
// to the model carries her own words (and the title), not the AI's replies.
func TestWritingOutlineGenerate_DerivesFromStudentMessagesOnly(t *testing.T) {
	stub := writingTextStubProvider(`[{"text":"占位","depth":0}]`)
	h, cookie, _, _ := liteHandlerWithProvider(t, stub)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文-独特标记")
	if rec := postWritingTurn(t, h, cookie, id, `{"text":"我想重点写光伏产业-学生独特词"}`); rec.Code != http.StatusOK {
		t.Fatalf("seed turn = %d; body=%s", rec.Code, rec.Body)
	}

	if rec := postWritingOutlineGenerate(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("generate = %d; body=%s", rec.Code, rec.Body)
	}
	sentUser := stub.LastRequest.Messages[len(stub.LastRequest.Messages)-1].Content
	if !strings.Contains(sentUser, "气候变化的议论文-独特标记") {
		t.Fatalf("prompt missing the title: %s", sentUser)
	}
	if !strings.Contains(sentUser, "光伏产业-学生独特词") {
		t.Fatalf("prompt missing what the student said: %s", sentUser)
	}
	if strings.Contains(sentUser, "占位") {
		t.Fatalf("prompt must not contain the AI's own reply text: %s", sentUser)
	}
}

// TestWritingOutlineGenerate_TargetWordsIsOptionalSignal — set: it rides
// along as a granularity signal. Unset: generation still works, no gate.
func TestWritingOutlineGenerate_TargetWordsIsOptionalSignal(t *testing.T) {
	stub := writingTextStubProvider(`[{"text":"一","depth":0}]`)
	h, cookie, _, _ := liteHandlerWithProvider(t, stub)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	// No targetWords set yet — generation must still succeed (no gate).
	if rec := postWritingOutlineGenerate(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("generate without targetWords = %d, want 200 (no gate); body=%s", rec.Code, rec.Body)
	}
	sentNoTarget := stub.LastRequest.Messages[len(stub.LastRequest.Messages)-1].Content
	if strings.Contains(sentNoTarget, "目标字数") {
		t.Fatalf("target words must not be invented when unset: %s", sentNoTarget)
	}

	// Now set targetWords, and confirm it rides along as a signal.
	twRec := httptest.NewRecorder()
	h.ServeHTTP(twRec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/target-words",
		strings.NewReader(`{"targetWords":800}`)), cookie))
	if twRec.Code != http.StatusOK {
		t.Fatalf("set target words = %d; body=%s", twRec.Code, twRec.Body)
	}
	if rec := postWritingOutlineGenerate(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("generate with targetWords = %d; body=%s", rec.Code, rec.Body)
	}
	sentWithTarget := stub.LastRequest.Messages[len(stub.LastRequest.Messages)-1].Content
	if !strings.Contains(sentWithTarget, "800") {
		t.Fatalf("target words signal missing from prompt: %s", sentWithTarget)
	}
}

// TestWritingOutlineGenerate_ModelFailureSurfacesAsError — USER RULE: a model
// failure is a real 502 ai_dialogue_failed, never a canned/deterministic
// fallback outline.
func TestWritingOutlineGenerate_ModelFailureSurfacesAsError(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingStreamErrorProvider{})
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	rec := postWritingOutlineGenerate(t, h, cookie, id)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("model failure = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "ai_dialogue_failed") {
		t.Fatalf("want ai_dialogue_failed in body, got %s", rec.Body)
	}
	if items := getWritingOutlineHTTP(t, h, cookie, id); len(items) != 0 {
		t.Fatalf("a failed generate must not persist anything: %+v", items)
	}
}

// TestWritingOutlineGenerate_UnparseableReplySurfacesAsError — the other half
// of the same rule: a non-JSON / empty-array reply is treated as a failure,
// not a silent empty "success".
func TestWritingOutlineGenerate_UnparseableReplySurfacesAsError(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("这不是 JSON，只是一句话。"))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	rec := postWritingOutlineGenerate(t, h, cookie, id)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unparseable reply = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "ai_dialogue_failed") {
		t.Fatalf("want ai_dialogue_failed in body, got %s", rec.Body)
	}
}

// TestWritingOutlineGenerate_MetersTheCall — surface='lite',
// purpose='outline_gen', flagship tier (EvalResolver — reviewer-tier work,
// never downgraded, mirroring plan_gen's routing choice).
func TestWritingOutlineGenerate_MetersTheCall(t *testing.T) {
	stub := writingTextStubProvider(`[{"text":"一","depth":0}]`)
	h, cookie, _, pool := liteHandlerWithProvider(t, stub)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := postWritingOutlineGenerate(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("generate = %d; body=%s", rec.Code, rec.Body)
	}

	var surface, purpose, tier string
	var projectNull bool
	if err := pool.QueryRow(t.Context(),
		`SELECT surface, purpose, tier, project_id IS NULL
		   FROM llm_call WHERE atom_id = $1 AND purpose = 'outline_gen'`, mustUUID(id)).
		Scan(&surface, &purpose, &tier, &projectNull); err != nil {
		t.Fatalf("read llm_call: %v", err)
	}
	if surface != "lite" || purpose != "outline_gen" {
		t.Fatalf("llm_call = %q/%q, want lite/outline_gen", surface, purpose)
	}
	if tier != "flagship" {
		t.Fatalf("llm_call tier = %q, want flagship (never-downgrade, reviewer-tier work)", tier)
	}
	if !projectNull {
		t.Fatalf("lite llm_call must leave project_id NULL")
	}
}

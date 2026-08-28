package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// writing_compose_test.go — Task 7: 成稿 (compose), draft, review, finish.
// Reuses writing_turn_test.go's harness (liteHandlerWithProvider,
// createWritingAtomHTTP, writingTextStubProvider, writingStreamErrorProvider)
// and writing_snippets_test.go's putWritingSnippetsHTTP — same package
// (api_test), same lite-writing test harness.
//
// Two requirements this file exists to prove, carried forward from earlier
// task reviews rather than written into the brief:
//
//  1. compose adds NO characters of its own. Every test below that exercises
//     compose passes a nil Provider (liteHandlerWithProvider(t, nil) — the
//     SAME nil-provider convention writing_snippets_test.go already uses for
//     "this call must never reach the model": a nil gateway.Provider panics
//     with a nil-pointer dereference the instant .Stream is invoked, so a
//     200 response is only reachable if compose never called it). And
//     TestWritingCompose_ConcatenatesFragmentsVerbatim asserts BYTE-LEVEL
//     equality between the composed body and strings.Join of the exact
//     fragment texts — not merely "contains" — which is the standard the
//     task holds this to.
//  2. the finished-write gate, inherited from loadOwnedWritingAtom since
//     Task 3 but never swept end-to-end until now:
//     TestWritingFinishedGate_RejectsWritesButAllowsReads sweeps PUT
//     /snippets, POST /turn, PUT /draft, POST /review (all 403
//     writing_finished), GET /draft and GET /messages (still 200), and
//     POST /finish itself (still 200 — exempt, so it stays idempotent).

func postWritingCompose(t *testing.T, h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/compose", nil), cookie))
	return rec
}

type writingDraftResp struct {
	Body      string  `json:"body"`
	UpdatedAt *string `json:"updatedAt"`
}

func getWritingDraftHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id string) (writingDraftResp, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/draft", nil), cookie))
	var out writingDraftResp
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode draft: %v — body=%s", err, rec.Body)
		}
	}
	return out, rec
}

func putWritingDraftHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/draft", strings.NewReader(body)), cookie))
	return rec
}

func postWritingReview(t *testing.T, h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/review", nil), cookie))
	return rec
}

func postWritingFinish(t *testing.T, h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/finish", nil), cookie))
	return rec
}

// commentResp / commentPointResp mirror Comment / CommentPoint's JSON shape
// (writing_comment.go) for decoding in this file's package (api_test) —
// same "local mirror struct, not an import of the internal type" convention
// as writingDraftResp/writingResp above.
type commentPointResp struct {
	Text  string `json:"text"`
	Quote string `json:"quote"`
}

type commentResp struct {
	ID        string             `json:"id"`
	Scope     string             `json:"scope"`
	SnippetID *string            `json:"snippetId"`
	Summary   string             `json:"summary"`
	Points    []commentPointResp `json:"points"`
	CreatedAt string             `json:"createdAt"`
}

type writingResp struct {
	ID         string  `json:"id"`
	Stage      string  `json:"stage"`
	Status     string  `json:"status"`
	FinishedAt *string `json:"finishedAt"`
}

func getWritingHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id string) writingResp {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id, nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET writing = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out writingResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode writing: %v — body=%s", err, rec.Body)
	}
	return out
}

// TestWritingCompose_NeverCallsModel — the mechanical proof of 铁律①'s
// "只拼接、不新造一个字" for this task: a nil Provider panics on any .Stream
// call, so a 200 here is only reachable if compose never touched the model
// at all.
func TestWritingCompose_NeverCallsModel(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":0,"text":"第一段：气候变化正在加速。"},{"position":1,"text":"第二段：中国的应对政策。"}]}`,
	); rec.Code != http.StatusOK {
		t.Fatalf("put snippets = %d; body=%s", rec.Code, rec.Body)
	}

	rec := postWritingCompose(t, h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("compose = %d, want 200 (and it must not have called the nil/panicking provider); body=%s", rec.Code, rec.Body)
	}
}

// TestWritingCompose_ConcatenatesFragmentsVerbatim — the byte-level standard
// the task holds compose to: the composed body is EXACTLY
// strings.Join(fragmentTexts, "\n\n"), in position order, not merely
// "contains every fragment somewhere". Snippets are PUT out of position
// order (2, then 0, then 1) to prove assembly follows `position`, not
// insertion order. Uses a nil provider too — this test would panic on any
// model call just as surely as the one above.
func TestWritingCompose_ConcatenatesFragmentsVerbatim(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	para0 := "引言：气候变化已经是全球共识，但各国的应对速度参差不齐。"
	para1 := "论点一：中国在光伏和风电产业上的投入是过去十年最显著的转变。"
	para2 := "结论：政策的连续性比单一年份的投入更重要。"

	// Posted out of order on purpose.
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":2,"text":"`+para2+`"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put position 2 = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"`+para0+`"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put position 0 = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":1,"text":"`+para1+`"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put position 1 = %d; body=%s", rec.Code, rec.Body)
	}

	if rec := postWritingCompose(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("compose = %d; body=%s", rec.Code, rec.Body)
	}

	draft, rec := getWritingDraftHTTP(t, h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET draft = %d; body=%s", rec.Code, rec.Body)
	}

	want := strings.Join([]string{para0, para1, para2}, "\n\n")
	if draft.Body != want {
		t.Fatalf("compose body byte-mismatch:\n got  = %q\n want = %q", draft.Body, want)
	}
	// Belt-and-braces: every paragraph must also be findable verbatim, the
	// brief's own phrasing of the assertion.
	for i, p := range []string{para0, para1, para2} {
		if !strings.Contains(draft.Body, p) {
			t.Fatalf("paragraph %d not found verbatim in composed body", i)
		}
	}
}

// TestWritingCompose_SkipsEmptySlots — an unwritten position contributes
// nothing to the assembled draft (no blank paragraph, no stray blank line
// pair) rather than being joined in as an empty string.
func TestWritingCompose_SkipsEmptySlots(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	para0 := "第一段有内容。"
	para2 := "第三段有内容。"
	if rec := putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":0,"text":"`+para0+`"},{"position":1,"text":"   "},{"position":2,"text":"`+para2+`"}]}`,
	); rec.Code != http.StatusOK {
		t.Fatalf("put snippets = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := postWritingCompose(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("compose = %d; body=%s", rec.Code, rec.Body)
	}
	draft, _ := getWritingDraftHTTP(t, h, cookie, id)
	want := para0 + "\n\n" + para2
	if draft.Body != want {
		t.Fatalf("compose body = %q, want %q (blank slot skipped)", draft.Body, want)
	}
}

// TestWritingCompose_ThenPutDraftKeepsEditing — after compose runs, PUT
// /draft is still the way she keeps polishing it; a second, unrelated
// compose call is never issued by this test, so the only way the PUT's text
// could end up overwritten is if some other code path re-ran compose behind
// her back.
func TestWritingCompose_ThenPutDraftKeepsEditing(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"原始段落。"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put snippets = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := postWritingCompose(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("compose = %d; body=%s", rec.Code, rec.Body)
	}

	edited := "原始段落，经过我自己润色之后的版本。"
	if rec := putWritingDraftHTTP(t, h, cookie, id, `{"body":"`+edited+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("put draft = %d; body=%s", rec.Code, rec.Body)
	}

	draft, _ := getWritingDraftHTTP(t, h, cookie, id)
	if draft.Body != edited {
		t.Fatalf("draft body = %q, want the edited text %q", draft.Body, edited)
	}
}

// TestWritingReview_ReturnsCommentaryDoesNotModifyDraft — review's other
// half of the brief (Task 5 / B4+B7 shape): it returns a structured
// {summary, points} comment whose quote is a real sentence of her draft, and
// the draft body itself is byte-for-byte unchanged afterward.
func TestWritingReview_ReturnsCommentaryDoesNotModifyDraft(t *testing.T) {
	draftText := "这是我写的第一段内容。"
	reply := `{"summary":"结构清楚，但论证需要更具体的数据支撑。","points":[{"text":"这句话缺一个可核实的来源。","quote":"` + draftText + `"}]}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(reply))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"`+draftText+`"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put snippets = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := postWritingCompose(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("compose = %d; body=%s", rec.Code, rec.Body)
	}
	before, _ := getWritingDraftHTTP(t, h, cookie, id)

	rec := postWritingReview(t, h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("review = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Comment commentResp `json:"comment"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode review: %v — body=%s", err, rec.Body)
	}
	if out.Comment.Scope != "draft" {
		t.Fatalf("comment scope = %q, want draft", out.Comment.Scope)
	}
	if out.Comment.SnippetID != nil {
		t.Fatalf("draft-scope comment must have a nil snippetId, got %v", *out.Comment.SnippetID)
	}
	if len(out.Comment.Points) != 1 || out.Comment.Points[0].Quote != draftText {
		t.Fatalf("points = %+v, want exactly one point quoting %q verbatim", out.Comment.Points, draftText)
	}

	after, _ := getWritingDraftHTTP(t, h, cookie, id)
	if after.Body != before.Body {
		t.Fatalf("review modified the draft body: before=%q after=%q — review must never write writing_draft.body", before.Body, after.Body)
	}
}

// TestWritingReview_MissingDraftIs400 — reviewing nothing is refused before
// any model call: a writing with no draft yet has 400 missing_draft.
func TestWritingReview_MissingDraftIs400(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	rec := postWritingReview(t, h, cookie, id)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("review with empty draft = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "missing_draft") {
		t.Fatalf("want missing_draft, got %s", rec.Body)
	}
}

// TestWritingReview_ModelFailureSurfacesAsError — USER RULE: an AI-dialogue
// failure must reach the student as a real 502 ai_dialogue_failed, never a
// canned stand-in comment, and the draft is left untouched.
func TestWritingReview_ModelFailureSurfacesAsError(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingStreamErrorProvider{})
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"这是我写的第一段内容。"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put snippets = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := postWritingCompose(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("compose = %d; body=%s", rec.Code, rec.Body)
	}
	before, _ := getWritingDraftHTTP(t, h, cookie, id)

	rec := postWritingReview(t, h, cookie, id)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("review on provider failure = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "ai_dialogue_failed") {
		t.Fatalf("want ai_dialogue_failed, got %s", rec.Body)
	}

	after, _ := getWritingDraftHTTP(t, h, cookie, id)
	if after.Body != before.Body {
		t.Fatalf("failed review modified the draft body: before=%q after=%q", before.Body, after.Body)
	}
}

// TestWritingFinish_GatesOnMissingDraft — mirrors finishReading's
// missing_takeaway gate: an empty (or never-composed) draft cannot be
// finished.
func TestWritingFinish_GatesOnMissingDraft(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	rec := postWritingFinish(t, h, cookie, id)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("finish with empty draft = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "missing_draft") {
		t.Fatalf("want missing_draft, got %s", rec.Body)
	}

	wr := getWritingHTTP(t, h, cookie, id)
	if wr.Status != "active" {
		t.Fatalf("status = %q after a refused finish, want unchanged \"active\"", wr.Status)
	}
}

// TestWritingFinish_SetsStatusNotStage_AndIsIdempotent — this task's two
// carried-forward requirements at the API layer: finish sets status but
// leaves stage exactly as she left it (a skip-straight-to-draft stays
// recorded, 铁律④), and a second POST /finish is a genuine no-op — same
// finished_at, not a moved one.
func TestWritingFinish_SetsStatusNotStage_AndIsIdempotent(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	before := getWritingHTTP(t, h, cookie, id)
	if before.Stage != "outline" || before.Status != "active" {
		t.Fatalf("baseline = %+v, want stage=outline status=active", before)
	}

	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"她跳过大纲和多段，直接写了一段就完成。"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put snippets = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := postWritingCompose(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("compose = %d; body=%s", rec.Code, rec.Body)
	}

	first := postWritingFinish(t, h, cookie, id)
	if first.Code != http.StatusOK {
		t.Fatalf("finish = %d, want 200; body=%s", first.Code, first.Body)
	}
	var out1 writingResp
	if err := json.Unmarshal(first.Body.Bytes(), &out1); err != nil {
		t.Fatalf("decode finish: %v — body=%s", err, first.Body)
	}
	if out1.Status != "finished" {
		t.Fatalf("status after finish = %q, want finished", out1.Status)
	}
	if out1.Stage != "outline" {
		t.Fatalf("finish forced stage to %q — status and stage must stay independent (she never touched 提纲/片段/成稿 stage transitions)", out1.Stage)
	}
	if out1.FinishedAt == nil || *out1.FinishedAt == "" {
		t.Fatalf("finishedAt not set after finish")
	}

	// Idempotent: POST /finish again succeeds (never 403 — it is the one
	// mutating route the finished gate exempts) and finished_at does not
	// drift to a later timestamp.
	second := postWritingFinish(t, h, cookie, id)
	if second.Code != http.StatusOK {
		t.Fatalf("second finish = %d, want 200 (idempotent, not 403); body=%s", second.Code, second.Body)
	}
	var out2 writingResp
	if err := json.Unmarshal(second.Body.Bytes(), &out2); err != nil {
		t.Fatalf("decode second finish: %v — body=%s", err, second.Body)
	}
	if out2.Status != "finished" || out2.Stage != "outline" {
		t.Fatalf("second finish = %+v, want status=finished stage=outline unchanged", out2)
	}
	if out2.FinishedAt == nil || *out2.FinishedAt != *out1.FinishedAt {
		t.Fatalf("finished_at drifted on a second finish: first=%v second=%v", out1.FinishedAt, out2.FinishedAt)
	}
}

// TestWritingFinishedGate_RejectsWritesButAllowsReads — this task's
// collected debt: the finished-write gate (loadOwnedWritingAtom,
// readings.go/writings.go) has applied to every writing route since Task 3,
// but no test ever swept it end-to-end until now. This is that sweep.
//
// Routes checked as 403 writing_finished: PUT /snippets, POST /turn,
// PUT /draft, POST /review.
// Routes checked as still 200: GET /draft, GET /messages.
// POST /finish itself: still 200 on a second call — the gate's own
// documented exemption, proven so finish stays idempotent rather than
// 403-ing on a repeat.
func TestWritingFinishedGate_RejectsWritesButAllowsReads(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"完成前写的一段。"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put snippets = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := postWritingCompose(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("compose = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := postWritingFinish(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("finish = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	assertWritingFinished403 := func(label string, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s on a finished writing = %d, want 403; body=%s", label, rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), `"code":"writing_finished"`) {
			t.Fatalf("%s body missing writing_finished code — got %s", label, rec.Body)
		}
	}

	assertWritingFinished403("PUT /snippets",
		putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":1,"text":"完成后想再加一段。"}]}`))
	assertWritingFinished403("POST /turn",
		postWritingTurn(t, h, cookie, id, `{"text":"我还想聊聊这篇稿子"}`))
	assertWritingFinished403("PUT /draft",
		putWritingDraftHTTP(t, h, cookie, id, `{"body":"完成后想改成稿"}`))
	assertWritingFinished403("POST /review",
		postWritingReview(t, h, cookie, id))

	// Reads still work — she can always read her own finished piece.
	if rec := postWritingCompose(t, h, cookie, id); rec.Code != http.StatusForbidden {
		t.Fatalf("POST /compose on a finished writing = %d, want 403 (it is a mutating route too); body=%s", rec.Code, rec.Body)
	}
	if _, rec := getWritingDraftHTTP(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("GET /draft on a finished writing = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if rec := listWritingMessagesRaw(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("GET /messages on a finished writing = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// POST /finish itself is exempt — a second call must stay 200, not 403,
	// or finish could never be idempotent once the gate is in place.
	if rec := postWritingFinish(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("second POST /finish on an already-finished writing = %d, want 200 (exempt from the gate); body=%s", rec.Code, rec.Body)
	}
}

// listWritingMessagesRaw is listWritingMessages (writing_turn_test.go)
// without the t.Fatalf-on-non-200 assertion baked in — this file needs the
// raw recorder to assert the 200 itself with its own message.
func listWritingMessagesRaw(t *testing.T, h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/messages", nil), cookie))
	return rec
}

// TestComposeDraft_NeverIncludesGuideText — the test Task 4's persistence
// decision rests on (writing_guide.go's "PERSISTED, deliberately" comment on
// guideWritingBlock): if this ever fails, revert to not persisting the guide.
// composeWritingDraft reads ONLY writing_snippet (see composeSnippetsIntoDraft's
// doc comment) — it never touches writing_outline at all — so this test seeds
// a distinctive string directly into writing_outline.guide (bypassing the
// model entirely, via the *sqlc.Queries liteHandlerWithProvider hands back)
// and proves it can never surface in the composed draft body.
func TestComposeDraft_NeverIncludesGuideText(t *testing.T) {
	h, cookie, q, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")
	atomID, err := uuid.Parse(id)
	if err != nil {
		t.Fatalf("parse atom id %q: %v", id, err)
	}

	block, err := q.InsertWritingOutlineNode(context.Background(), sqlc.InsertWritingOutlineNodeParams{
		AtomID: atomID, Text: "开头", Role: "开头", Depth: 0, Position: 0,
	})
	if err != nil {
		t.Fatalf("seed outline node: %v", err)
	}

	const guideMarker = "GUIDE_MARKER_绝不能出现在正文里"
	guidePayload := []byte(`{"job":"` + guideMarker + `","methods":[],"questions":["她见过这种事吗？"]}`)
	if err := q.SetWritingOutlineGuide(context.Background(), sqlc.SetWritingOutlineGuideParams{
		ID: block.ID, Guide: guidePayload,
	}); err != nil {
		t.Fatalf("seed guide: %v", err)
	}

	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"这是她自己写的第一段。"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put snippets = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := postWritingCompose(t, h, cookie, id); rec.Code != http.StatusOK {
		t.Fatalf("compose = %d; body=%s", rec.Code, rec.Body)
	}

	draft, rec := getWritingDraftHTTP(t, h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET draft = %d; body=%s", rec.Code, rec.Body)
	}
	if strings.Contains(draft.Body, guideMarker) {
		t.Fatalf("composed draft contains the guide marker — 铁律① breach: %q", draft.Body)
	}
}

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Outline-relink tests. Every outline save is a full replace that mints fresh
// row ids, and writing_snippet.outline_id is ON DELETE SET NULL — so without
// repair, saving the outline detaches every paragraph she has already written.
// Revising the outline while drafting is the normal thing to do, not an edge
// case, so the repair has to be right.
//
// The repair matches on heading TEXT, not position. The reorder test below is
// the reason: an earlier version resolved headings by position, which is
// correct only while the outline never changes shape, and silently wrong —
// confidently, specifically wrong — the moment she drags a point.

func putOutlineHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/outline", strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT outline = %d; body=%s", rec.Code, rec.Body)
	}
}

func outlineIDsByText(t *testing.T, h http.Handler, cookie *http.Cookie, id string) map[string]string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/outline", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET outline = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Outline []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"outline"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode outline: %v — body=%s", err, rec.Body)
	}
	byText := make(map[string]string, len(out.Outline))
	for _, o := range out.Outline {
		byText[o.Text] = o.ID
	}
	return byText
}

func headingByText(snips []writingSnippetItem) map[string]string {
	out := make(map[string]string, len(snips))
	for _, s := range snips {
		out[s.Text] = s.OutlineHeading
	}
	return out
}

// TestWritingOutline_RelinkSurvivesReorder — the case that broke the previous
// approach, and the reason the repair matches on text.
//
// She outlines [因, 果], writes a paragraph under each, then drags 果 above 因.
// Position-matching would now hand the 因-paragraph the heading 果 and vice
// versa: specific, confident, and exactly backwards, with nothing marking it a
// guess. Text-matching keeps each paragraph with the heading it was actually
// written under, wherever that heading moved to.
func TestWritingOutline_RelinkSurvivesReorder(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	putOutlineHTTP(t, h, cookie, id, `{"outline":[{"text":"原因","depth":0},{"text":"结果","depth":0}]}`)
	ids := outlineIDsByText(t, h, cookie, id)

	putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":0,"text":"这段写原因","outlineId":"`+ids["原因"]+`"}]}`)
	putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":1,"text":"这段写结果","outlineId":"`+ids["结果"]+`"}]}`)

	before := headingByText(getWritingSnippetsHTTP(t, h, cookie, id))
	if before["这段写原因"] != "原因" || before["这段写结果"] != "结果" {
		t.Fatalf("precondition: headings wrong before reorder: %+v", before)
	}

	// The drag: 结果 now comes first. Same two headings, opposite order.
	putOutlineHTTP(t, h, cookie, id, `{"outline":[{"text":"结果","depth":0},{"text":"原因","depth":0}]}`)

	after := headingByText(getWritingSnippetsHTTP(t, h, cookie, id))
	if after["这段写原因"] != "原因" {
		t.Fatalf("after reorder, the 原因 paragraph reports heading %q, want 原因 — headings were swapped by position",
			after["这段写原因"])
	}
	if after["这段写结果"] != "结果" {
		t.Fatalf("after reorder, the 结果 paragraph reports heading %q, want 结果 — headings were swapped by position",
			after["这段写结果"])
	}
}

// TestWritingOutline_RelinkDropsRewordedHeading — the accepted cost of
// refusing to guess.
//
// She keeps the point but improves its wording: 原因 → 原因：排放结构. Her
// paragraph is arguably still about it, and two earlier versions of the
// matcher tried to recover that — first by pairing leftovers by position, then
// by pairing when exactly one row was unmatched on each side. Both produced a
// WRONG heading in ordinary cases (reorder-plus-insert; delete-one-add-one),
// because a full-replace PUT carries no per-row intent: "I reworded this
// point" and "I replaced it with a different one" are the same bytes.
//
// So the link drops, visibly, and she can carry on or rewrite the heading
// back. A blank label costs her nothing; a confident wrong one misleads her
// about her own work. If reword-survival is wanted later, the request must
// carry row identity — the fix belongs in the wire shape, not in a guess here.
func TestWritingOutline_RelinkDropsRewordedHeading(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	putOutlineHTTP(t, h, cookie, id, `{"outline":[{"text":"原因","depth":0}]}`)
	ids := outlineIDsByText(t, h, cookie, id)
	putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":0,"text":"这段写原因","outlineId":"`+ids["原因"]+`"}]}`)

	putOutlineHTTP(t, h, cookie, id, `{"outline":[{"text":"原因：排放结构","depth":0}]}`)

	after := headingByText(getWritingSnippetsHTTP(t, h, cookie, id))
	if after["这段写原因"] != "" {
		t.Fatalf("heading = %q after she reworded that outline point; want \"\" — a reworded point and a replaced point are indistinguishable in a full-replace PUT, so nothing may be guessed",
			after["这段写原因"])
	}
}

// TestWritingOutline_RelinkRefusesToGuessOnDeleteAndAdd — the case that killed
// the "exactly one leftover on each side" heuristic.
//
// She deletes 结果 and adds an unrelated 反驳 in the same save. That leaves
// exactly one unmatched row on each side — identical in shape to a reword — so
// the previous version paired them and reattached her 结果 paragraph to 反驳, a
// heading she never wrote it under. Swapping one point for another is an
// entirely ordinary edit, not a corner case.
func TestWritingOutline_RelinkRefusesToGuessOnDeleteAndAdd(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	putOutlineHTTP(t, h, cookie, id,
		`{"outline":[{"text":"原因","depth":0},{"text":"结果","depth":0}]}`)
	ids := outlineIDsByText(t, h, cookie, id)
	putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":1,"text":"这段写结果","outlineId":"`+ids["结果"]+`"}]}`)

	putOutlineHTTP(t, h, cookie, id,
		`{"outline":[{"text":"原因","depth":0},{"text":"反驳","depth":0}]}`)

	after := headingByText(getWritingSnippetsHTTP(t, h, cookie, id))
	if after["这段写结果"] == "反驳" {
		t.Fatal("her 结果 paragraph was reattached to 反驳 — a different point she added in the same save, not a rewording of the one she deleted")
	}
	if after["这段写结果"] != "" {
		t.Fatalf("heading = %q; want \"\"", after["这段写结果"])
	}
}

// TestWritingOutline_RelinkDropsDeletedHeading — the honest half. When a point
// is genuinely GONE (she deleted it and the outline is shorter), there is no
// leftover row for it to pair with, so nothing is invented: "" is a real
// answer meaning "that heading no longer exists", never a shrug.
func TestWritingOutline_RelinkDropsDeletedHeading(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	putOutlineHTTP(t, h, cookie, id,
		`{"outline":[{"text":"原因","depth":0},{"text":"结果","depth":0}]}`)
	ids := outlineIDsByText(t, h, cookie, id)
	putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":1,"text":"这段写结果","outlineId":"`+ids["结果"]+`"}]}`)

	// She drops 结果 entirely. 原因 survives by text; nothing is left over for
	// 结果 to pair with.
	putOutlineHTTP(t, h, cookie, id, `{"outline":[{"text":"原因","depth":0}]}`)

	afterDeleted := headingByText(getWritingSnippetsHTTP(t, h, cookie, id))
	if afterDeleted["这段写结果"] != "" {
		t.Fatalf("heading = %q after she deleted that outline point; want \"\" — attaching her paragraph to 原因, a point she never wrote it under, would be worse than showing nothing",
			afterDeleted["这段写结果"])
	}
}

// TestWritingOutline_RelinkRefusesToGuessOnReorderPlusInsert — the case that
// killed the previous version of the second pass, kept as a permanent guard.
//
// She reorders AND adds a new opening point in one save. Text matching places
// 结果. That leaves her old 原因 unmatched, and TWO new rows unmatched: the
// freshly written 引言 and the reworded 原因：排放结构. Pairing leftovers by
// position — which the previous version did, on the reasoning that a reorder
// leaves nothing to mispair — hands her 原因 paragraph the heading 引言, a
// point she had just written and never wrote that paragraph under.
//
// With more than one leftover on either side, a reword and an insertion are
// indistinguishable. Silence is then the correct answer.
func TestWritingOutline_RelinkRefusesToGuessOnReorderPlusInsert(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	putOutlineHTTP(t, h, cookie, id,
		`{"outline":[{"text":"原因","depth":0},{"text":"结果","depth":0}]}`)
	ids := outlineIDsByText(t, h, cookie, id)
	putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":0,"text":"这段写原因","outlineId":"`+ids["原因"]+`"}]}`)

	putOutlineHTTP(t, h, cookie, id,
		`{"outline":[{"text":"引言","depth":0},{"text":"原因：排放结构","depth":0},{"text":"结果","depth":0}]}`)

	after := headingByText(getWritingSnippetsHTTP(t, h, cookie, id))
	if after["这段写原因"] == "引言" {
		t.Fatal("her 原因 paragraph was attached to 引言 — a heading she had just written and never wrote that paragraph under")
	}
	if after["这段写原因"] != "" {
		t.Fatalf("heading = %q; want \"\" — with two candidates a reword and an insertion are indistinguishable, so nothing should be guessed",
			after["这段写原因"])
	}
}

// TestWritingOutline_RelinkSurvivesTextEdit — the ordinary case must still
// work: she resaves the outline untouched (or edits some OTHER point), and her
// paragraphs keep their headings even though every id has changed.
func TestWritingOutline_RelinkSurvivesTextEdit(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	putOutlineHTTP(t, h, cookie, id, `{"outline":[{"text":"原因","depth":0},{"text":"结果","depth":0}]}`)
	ids := outlineIDsByText(t, h, cookie, id)
	putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":0,"text":"这段写原因","outlineId":"`+ids["原因"]+`"}]}`)

	// She adds a third point and edits the second; the first is untouched.
	putOutlineHTTP(t, h, cookie, id,
		`{"outline":[{"text":"原因","depth":0},{"text":"结果与影响","depth":0},{"text":"对策","depth":0}]}`)

	after := headingByText(getWritingSnippetsHTTP(t, h, cookie, id))
	if after["这段写原因"] != "原因" {
		t.Fatalf("heading = %q after an unrelated outline edit; want 原因 — the ids all changed, but her paragraph's own heading did not",
			after["这段写原因"])
	}
}

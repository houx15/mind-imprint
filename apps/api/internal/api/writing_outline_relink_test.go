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

// TestWritingOutline_RelinkFollowsRewordedHeading — the second pass, and the
// reason there is one.
//
// She keeps the point but improves its wording: 原因 → 原因：排放结构. Text
// matching alone would call that a different point and drop her paragraph's
// heading, which is wrong — it is the same point, better said, and her
// paragraph is still about it. Position among the rows text-matching could not
// place recovers it. That fallback is safe here precisely because it only sees
// leftovers: on a reorder, text matching consumes everything first, so
// position never gets the chance to mispair.
func TestWritingOutline_RelinkFollowsRewordedHeading(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	putOutlineHTTP(t, h, cookie, id, `{"outline":[{"text":"原因","depth":0}]}`)
	ids := outlineIDsByText(t, h, cookie, id)
	putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":0,"text":"这段写原因","outlineId":"`+ids["原因"]+`"}]}`)

	putOutlineHTTP(t, h, cookie, id, `{"outline":[{"text":"原因：排放结构","depth":0}]}`)

	after := headingByText(getWritingSnippetsHTTP(t, h, cookie, id))
	if after["这段写原因"] != "原因：排放结构" {
		t.Fatalf("heading = %q after she reworded that outline point; want the reworded text — rewording is not deleting, and her paragraph is still about that point",
			after["这段写原因"])
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

	after := headingByText(getWritingSnippetsHTTP(t, h, cookie, id))
	if after["这段写结果"] != "" {
		t.Fatalf("heading = %q after she deleted that outline point; want \"\" — attaching her paragraph to 原因, a point she never wrote it under, would be worse than showing nothing",
			after["这段写结果"])
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

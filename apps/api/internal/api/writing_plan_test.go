package api_test

// writing_plan_test.go — 结构 as a planning conversation.
//
// The hard guarantee this file pins: a planning turn can only ever ADD to the
// map. It cannot rewrite a node and it cannot delete one — not because the
// prompt asks nicely, but because writing_plan.go contains no such call. That
// is asserted here against a real database rather than trusted.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type planTurnResult struct {
	Reply    string               `json:"reply"`
	Outline  []writingOutlineItem `json:"outline"`
	AddedIDs []string             `json:"addedIds"`
}

func planTurn(t *testing.T, h http.Handler, cookie *http.Cookie, id, text string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	body := `{"text":` + strconv.Quote(text) + `}`
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/plan/turn", strings.NewReader(body)), cookie))
	return rec
}

func decodePlanTurn(t *testing.T, rec *httptest.ResponseRecorder) planTurnResult {
	t.Helper()
	var out planTurnResult
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode plan turn: %v — body=%s", err, rec.Body)
	}
	return out
}

// TestWritingPlanTurn_GrowsTheMapFromWhatSheSaid — the ordinary case: she
// speaks, 印记 replies, and what she said becomes a node.
func TestWritingPlanTurn_GrowsTheMapFromWhatSheSaid(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(
		`{"reply":"你打算用哪几件事来说明？","add":[{"parentId":"","text":"不该一刀切禁手机","role":"中心论点"}]}`))
	id := createWritingAtomHTTP(t, h, cookie, "学校该不该禁手机。")

	rec := planTurn(t, h, cookie, id, "我觉得不该一刀切禁手机。")
	if rec.Code != http.StatusOK {
		t.Fatalf("plan turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodePlanTurn(t, rec)
	if out.Reply == "" {
		t.Fatalf("plan turn returned no reply")
	}
	if len(out.Outline) != 1 {
		t.Fatalf("outline = %+v, want exactly the one node she produced", out.Outline)
	}
	if out.Outline[0].Text != "不该一刀切禁手机" || out.Outline[0].Role != "中心论点" {
		t.Fatalf("node = %+v, want her sentence with a plain-language role", out.Outline[0])
	}
	if len(out.AddedIDs) != 1 || out.AddedIDs[0] != out.Outline[0].ID {
		t.Fatalf("addedIds = %v, want the id of the node that just appeared", out.AddedIDs)
	}

	// And it persisted — the map survives a reload.
	if got := getWritingOutlineHTTP(t, h, cookie, id); len(got) != 1 || got[0].Text != "不该一刀切禁手机" {
		t.Fatalf("GET outline = %+v, want the node to have persisted", got)
	}
}

// Nesting (a node hanging one level under its parent) is NOT tested through
// a plan turn here: the stub provider is built with a fixed reply, so it
// cannot echo back a uuid the test only learns afterwards. The tree shape is
// covered where it actually renders — buildMindMap in the lite frontend
// suite — and the depth column itself is covered by writing_outline_test.go.

// TestWritingPlanTurn_CannotTouchWhatIsAlreadyOnTheMap is the load-bearing
// one. A turn that tries to restate the whole map must still only ADD: every
// node that was there before is byte-identical afterwards.
func TestWritingPlanTurn_CannotTouchWhatIsAlreadyOnTheMap(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(
		`{"reply":"记下了。","add":[{"parentId":"","text":"她自己写的那一条","role":"中心论点"}]}`))
	id := createWritingAtomHTTP(t, h, cookie, "随便写点什么。")

	before := decodePlanTurn(t, planTurn(t, h, cookie, id, "我想说的是这个。"))
	if len(before.Outline) != 1 {
		t.Fatalf("setup: outline = %+v, want 1 node", before.Outline)
	}
	original := before.Outline[0]

	after := decodePlanTurn(t, planTurn(t, h, cookie, id, "再补一句。"))
	var found *writingOutlineItem
	for i := range after.Outline {
		if after.Outline[i].ID == original.ID {
			found = &after.Outline[i]
		}
	}
	if found == nil {
		t.Fatalf("the node she already had was DELETED by a planning turn; outline=%+v", after.Outline)
	}
	if found.Text != original.Text || found.Role != original.Role {
		t.Fatalf("a planning turn rewrote her node: %+v → %+v", original, *found)
	}
}

// TestWritingPlanTurn_DropsANodeWithAnInventedParent — a parentId the model
// made up must lose the node, not attach it somewhere she never put it.
// Specific-and-wrong is worse than absent here: she can always say it again.
func TestWritingPlanTurn_DropsANodeWithAnInventedParent(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(
		`{"reply":"嗯。","add":[{"parentId":"11111111-2222-3333-4444-555555555555","text":"孤儿节点","role":"理由"}]}`))
	id := createWritingAtomHTTP(t, h, cookie, "写点什么。")

	out := decodePlanTurn(t, planTurn(t, h, cookie, id, "我说点什么。"))
	if len(out.Outline) != 0 {
		t.Fatalf("outline = %+v, want the orphan dropped rather than reparented", out.Outline)
	}
	if len(out.AddedIDs) != 0 {
		t.Fatalf("addedIds = %v, want empty", out.AddedIDs)
	}
	// The turn itself still succeeded — losing one node must not cost her the
	// reply she was waiting for.
	if out.Reply == "" {
		t.Fatalf("the reply was dropped along with the node")
	}
}

// TestWritingPlanTurn_ModelFailureSurfaces — USER RULE: an AI-dialogue
// failure is a real 502, never a canned stand-in.
func TestWritingPlanTurn_ModelFailureSurfaces(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, streamErrorProvider{})
	id := createWritingAtomHTTP(t, h, cookie, "写点什么。")
	if rec := planTurn(t, h, cookie, id, "在吗"); rec.Code != http.StatusBadGateway {
		t.Fatalf("plan turn on model failure = %d, want 502; body=%s", rec.Code, rec.Body)
	}
}

// TestWritingPlanTurn_UnparseableReplyIsAnError — a reply that decodes to
// nothing usable must not render as an empty coach bubble.
func TestWritingPlanTurn_UnparseableReplyIsAnError(t *testing.T) {
	for _, reply := range []string{`不是 JSON`, `{"add":[]}`, `{"reply":"   ","add":[]}`} {
		h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(reply))
		id := createWritingAtomHTTP(t, h, cookie, "写点什么。")
		if rec := planTurn(t, h, cookie, id, "在吗"); rec.Code != http.StatusBadGateway {
			t.Fatalf("reply %q → %d, want 502; body=%s", reply, rec.Code, rec.Body)
		}
	}
}

// TestWritingPlanTurn_RequiresText — an empty turn is a 400, not a wasted
// model call.
func TestWritingPlanTurn_RequiresText(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "写点什么。")
	if rec := planTurn(t, h, cookie, id, "   "); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank plan turn = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

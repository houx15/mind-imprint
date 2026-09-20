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

	"mindimprint/api/internal/gateway"
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

// writingTextSequenceStubProvider replays a DIFFERENT plain-text plan reply
// per model call, in order — planTurnWith uses it to seed a thesis on the
// first turn, then exercise the scripted reply under test on the second.
func writingTextSequenceStubProvider(replies ...string) *gateway.SequenceStubProvider {
	scripts := make([][]gateway.StreamEvent, len(replies))
	for i, reply := range replies {
		scripts[i] = []gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: reply},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 90, OutputTokens: 30}},
			{Kind: gateway.EventDone, StopReason: gateway.StopStop},
		}
	}
	return gateway.NewSequenceStubProvider(scripts...)
}

// planTurnWith spins up a fresh writing atom, seeds a depth-0 thesis with an
// ordinary first plan turn, then runs a second plan turn scripted with
// `reply` and returns its decoded result. Shared by tests that need an
// existing thesis already on the map before asserting on what a further turn
// does around it.
func planTurnWith(t *testing.T, reply string) planTurnResult {
	t.Helper()
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextSequenceStubProvider(
		`{"reply":"你打算用哪几件事来说明？","add":[{"kind":"thesis","text":"不该一刀切禁手机"}]}`,
		reply,
	))
	id := createWritingAtomHTTP(t, h, cookie, "学校该不该禁手机。")

	planTurn(t, h, cookie, id, "我觉得不该一刀切禁手机。")
	rec := planTurn(t, h, cookie, id, "开头我想这样写。")
	if rec.Code != http.StatusOK {
		t.Fatalf("plan turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	return decodePlanTurn(t, rec)
}

// B0: an opening is a top-level sibling of the thesis, ordered before it — not a
// child of it. The old prompt taught the model that the only depth-0 node IS the
// thesis, so this is the assertion that the teaching changed.
func TestPlanTurn_AcceptsATopLevelOpeningBesideTheThesis(t *testing.T) {
	// Existing thesis at depth 0, position 0.
	// Model adds an opening — it must be stored at depth 0 and
	// must NOT be reparented under the thesis or dropped.
	out := planTurnWith(t,
		`{"reply":"记下了。","add":[{"kind":"opening","text":"夏天路上晒得受不了"}]}`)

	var roots []string
	for _, n := range out.Outline {
		if n.Depth == 0 {
			roots = append(roots, n.Role)
		}
	}
	if len(roots) < 2 {
		t.Fatalf("depth-0 nodes = %v, want the thesis AND the opening as siblings", roots)
	}
}

// TestWritingPlanTurn_GrowsTheMapFromWhatSheSaid — the ordinary case: she
// speaks, 印记 replies, and what she said becomes a node.
func TestWritingPlanTurn_GrowsTheMapFromWhatSheSaid(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(
		`{"reply":"你打算用哪几件事来说明？","add":[{"kind":"thesis","text":"不该一刀切禁手机"}]}`))
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
		`{"reply":"记下了。","add":[{"kind":"thesis","text":"她自己写的那一条"}]}`))
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

// TestWritingPlanTurn_DropsANodeWithAnInventedKind — 模型编一个不在闭表里的
// kind，这一条要整条丢掉，不能猜一个最近的把她的话放到她没放过的地方。
// 这里「具体而错」比「没有」更糟：她随时可以再说一遍。
//
// 🚨 2026-09-20：这个用例原来叫 DropsANodeWithAnInventedParent，测的是模型
// 编一个 parentId。模型不再给 parentId 了，能编的只剩 kind，所以这条不变量
// 搬到了 kind 上 —— 丢掉的那一类没有消失，只是换了个字段。
func TestWritingPlanTurn_DropsANodeWithAnInventedKind(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(
		`{"reply":"嗯。","add":[{"kind":"我编的一种","text":"孤儿节点"}]}`))
	id := createWritingAtomHTTP(t, h, cookie, "写点什么。")

	out := decodePlanTurn(t, planTurn(t, h, cookie, id, "我说点什么。"))
	if len(out.Outline) != 0 {
		t.Fatalf("outline = %+v, want the node dropped rather than given a guessed kind", out.Outline)
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

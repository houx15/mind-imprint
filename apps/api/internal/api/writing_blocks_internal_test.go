package api

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// Same cases as apps/lite-web/src/writings/slots.test.ts — the two sides must
// number cards the same way.

func blockLink(o uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: o, Valid: true} }

func cardKinds(cards []writingCard) []string {
	out := make([]string, len(cards))
	for i, c := range cards {
		out[i] = c.Kind
	}
	return out
}

func TestWritingBlockNumbersFoldsMaterial(t *testing.T) {
	tID, aID, a1ID, bID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	outline := []sqlc.WritingOutline{
		{ID: tID, Depth: 0, Position: 0, Text: "论点", Role: "中心论点"},
		{ID: aID, Depth: 1, Position: 1, Text: "理由A"},
		{ID: a1ID, Depth: 2, Position: 2, Text: "例子a"},
		{ID: bID, Depth: 1, Position: 3, Text: "理由B"},
	}
	sT := sqlc.WritingSnippet{ID: uuid.New(), OutlineID: blockLink(tID), Position: 0, Text: "论点段"}
	sB := sqlc.WritingSnippet{ID: uuid.New(), OutlineID: blockLink(bID), Position: 3, Text: "B段"}
	free := sqlc.WritingSnippet{ID: uuid.New(), Position: 1000, Text: "加的一段"}

	// 开头(论点) · 理由A · 理由B · 结尾 · 自由段落
	got := writingBlockNumbers(outline, []sqlc.WritingSnippet{sT, sB, free})
	if got[sT.ID] != 1 || got[sB.ID] != 3 || got[free.ID] != 5 {
		t.Fatalf("numbers = %v; want 开头=1 B=3 free=5 (例子a folded, 结尾=4)", got)
	}

	// A material node she already wrote on keeps its card.
	sA1 := sqlc.WritingSnippet{ID: uuid.New(), OutlineID: blockLink(a1ID), Position: 2, Text: "例子段"}
	got = writingBlockNumbers(outline, []sqlc.WritingSnippet{sT, sA1, sB})
	if got[sA1.ID] != 3 || got[sB.ID] != 4 {
		t.Fatalf("numbers = %v; want 例子=3 B=4", got)
	}
}

// 2026-09-18 产品负责人：「我有两个例子，结果就变成了两段。」
// 两个例子直接挂在中心论点下面 ⇒ 开头 · 一段主体（两个例子都在里面）· 结尾。
func TestWritingCardsTwoExamplesAreNotTwoParts(t *testing.T) {
	outline := []sqlc.WritingOutline{
		{ID: uuid.New(), Depth: 0, Position: 0, Text: "苦乐在心不在事", Role: "中心论点"},
		{ID: uuid.New(), Depth: 1, Position: 1, Text: "跳绳从烦到乐", Role: "你经历过的事"},
		{ID: uuid.New(), Depth: 1, Position: 2, Text: "苏轼被贬黄州", Role: "历史上的例子"},
	}
	cards := writingCards(outline, nil)
	want := []string{writingCardOpening, writingCardPoint, writingCardClosing}
	if got := cardKinds(cards); len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("kinds = %v; want %v", got, want)
	}
	if cards[2].Position != writingClosingPosition || cards[2].Node != nil {
		t.Fatalf("结尾 should be virtual at %d, got %+v", writingClosingPosition, cards[2])
	}
}

// 显式的开头/结尾节点各自绑一张卡；中心论点没写过字就不单独成卡。
func TestWritingCardsExplicitOpeningAndClosing(t *testing.T) {
	thesis := uuid.New()
	outline := []sqlc.WritingOutline{
		{ID: uuid.New(), Depth: 0, Position: 0, Text: "从一件小事说起", Role: "开头"},
		{ID: thesis, Depth: 0, Position: 1, Text: "论点", Role: "中心论点"},
		{ID: uuid.New(), Depth: 1, Position: 2, Text: "理由", Role: "分论点"},
		{ID: uuid.New(), Depth: 0, Position: 3, Text: "回到开头那件事", Role: "结尾"},
	}
	cards := writingCards(outline, nil)
	if got := cardKinds(cards); len(got) != 3 || got[0] != writingCardOpening || got[2] != writingCardClosing {
		t.Fatalf("kinds = %v", got)
	}
	if cards[2].Node == nil || cards[2].Node.Text != "回到开头那件事" {
		t.Fatalf("结尾 should bind the closing node, got %+v", cards[2])
	}
	// 中心论点上写过字 ⇒ 它的字不能消失。
	s := sqlc.WritingSnippet{ID: uuid.New(), OutlineID: blockLink(thesis), Position: 1, Text: "论点段"}
	cards = writingCards(outline, []sqlc.WritingSnippet{s})
	if len(cards) != 4 || cards[1].Snippet == nil || cards[1].Snippet.ID != s.ID {
		t.Fatalf("written thesis must keep its own card: %v", cardKinds(cards))
	}
}

// 拼成稿按卡片顺序：虚拟开头存在 998，结尾存在 999，都不能按位置排错。
func TestWritingSnippetsInCardOrder(t *testing.T) {
	a := uuid.New()
	outline := []sqlc.WritingOutline{
		{ID: uuid.New(), Depth: 1, Position: 0, Text: "理由"},
		{ID: a, Depth: 1, Position: 1, Text: "理由二"},
	}
	open := sqlc.WritingSnippet{ID: uuid.New(), Position: writingOpeningPosition, Text: "开头段"}
	body := sqlc.WritingSnippet{ID: uuid.New(), OutlineID: blockLink(a), Position: 1, Text: "主体段"}
	end := sqlc.WritingSnippet{ID: uuid.New(), Position: writingClosingPosition, Text: "结尾段"}
	free := sqlc.WritingSnippet{ID: uuid.New(), Position: 1000, Text: "自由段"}
	got := composeSnippetsIntoDraft(writingSnippetsInCardOrder(outline, []sqlc.WritingSnippet{body, free, end, open}))
	if got != "开头段\n\n主体段\n\n结尾段\n\n自由段" {
		t.Fatalf("draft = %q", got)
	}
}

func TestResolvePlanParent(t *testing.T) {
	thesis := sqlc.WritingOutline{ID: uuid.New(), Depth: 0, Text: "脆弱不可怕，人是可以脆弱的"}
	reason := sqlc.WritingOutline{ID: uuid.New(), Depth: 1, Text: "没有人不经历脆弱就能成长"}
	rows := []sqlc.WritingOutline{thesis, reason}
	byID := map[string]sqlc.WritingOutline{
		thesis.ID.String(): thesis, reason.ID.String(): reason,
		planHandle(0): thesis, planHandle(1): reason,
	}
	full := thesis.ID.String()
	segs := strings.Split(full, "-")

	for _, raw := range []string{
		full,
		"id=" + full,
		" " + full[:13] + " ",
		"脆弱不可怕，人是可以脆弱的。",
		"n1",
		"id=N1",
		// 2026-09-18 实测抄丢第四段的那种。
		segs[0] + "-" + segs[1] + "-" + segs[2] + "-" + segs[4],
	} {
		if got, ok := resolvePlanParent(byID, rows, raw); !ok || got.ID != thesis.ID {
			t.Errorf("%q should resolve to the thesis", raw)
		}
	}
	if _, ok := resolvePlanParent(byID, rows, "n9"); ok {
		t.Error("an invented id must not resolve")
	}
}

// 截图那一轮：她说「脆弱让人区别于机器」，印记 引了它、说放进图里了，add 却是空的。
func TestPlanReplyUnplaced(t *testing.T) {
	thesis := sqlc.WritingOutline{ID: uuid.New(), Depth: 0, Text: "脆弱不可怕，人是可以脆弱的"}
	rows := []sqlc.WritingOutline{thesis}
	byID := map[string]sqlc.WritingOutline{thesis.ID.String(): thesis}
	said := "脆弱让人区别于机器"
	reply := "底下两条分论点方向不同——「脆弱让人区别于机器」从另一个角度说明脆弱本身有价值。可以开始写了。"

	if got := planReplyUnplaced(reply, said, rows, byID, nil); len(got) != 1 {
		t.Fatalf("should flag the unplaced point, got %v", got)
	}
	// parentId 认不出来的那条也算没放上去。
	bad := []writingPlanAdd{{ParentID: "n1", Text: "脆弱让人区别于机器"}}
	if got := planReplyUnplaced(reply, said, rows, byID, bad); len(got) != 1 {
		t.Fatalf("an add with an unknown parent is dropped, so still unplaced; got %v", got)
	}
	ok := []writingPlanAdd{{ParentID: thesis.ID.String(), Text: "脆弱让人区别于机器"}}
	if got := planReplyUnplaced(reply, said, rows, byID, ok); len(got) != 0 {
		t.Fatalf("placed this turn, got %v", got)
	}
	// 引的是图上已有的、或者不是她这一轮的话，都不算。
	if got := planReplyUnplaced("你的中心论点是「脆弱不可怕，人是可以脆弱的」，这是「并列论证」。", said, rows, byID, nil); len(got) != 0 {
		t.Fatalf("quotes of the map or of method names are fine, got %v", got)
	}
}

// 2026-09-18 走查：例子落在了最上层（同一轮加的分论点它引用不到）。
// 最上层的例子也是例子：不当中心论点，不单独成一张「分论点」。
func TestWritingCardsTopLevelExampleIsMaterial(t *testing.T) {
	outline := []sqlc.WritingOutline{
		{ID: uuid.New(), Depth: 0, Position: 0, Text: "人可以脆弱", Role: "中心论点"},
		{ID: uuid.New(), Depth: 1, Position: 1, Text: "脆弱没有打垮我", Role: "分论点"},
		{ID: uuid.New(), Depth: 1, Position: 2, Text: "脆弱让人区别于机器", Role: "分论点"},
		{ID: uuid.New(), Depth: 0, Position: 3, Text: "爸爸入狱、妹妹抑郁", Role: "你经历过的事"},
	}
	got := cardKinds(writingCards(outline, nil))
	want := []string{writingCardOpening, writingCardPoint, writingCardPoint, writingCardClosing}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("kinds = %v; want %v", got, want)
	}
	if s := writingPlanShapeOf(outline); s.Top != 1 || s.Points != 2 || s.Material != 1 || s.Wider != 0 {
		t.Fatalf("shape = %+v; the top-level example is personal material", s)
	}
}

func TestExamplePlanParent(t *testing.T) {
	a := sqlc.WritingOutline{ID: uuid.New(), Depth: 1, Position: 1, Text: "理由A", Role: "分论点"}
	b := sqlc.WritingOutline{ID: uuid.New(), Depth: 1, Position: 3, Text: "理由B", Role: "分论点"}
	ex := sqlc.WritingOutline{ID: uuid.New(), Depth: 1, Position: 4, Text: "一个例子", Role: "历史上的例子"}
	live := []sqlc.WritingOutline{{ID: uuid.New(), Depth: 0, Position: 0, Text: "论点"}, a, b, ex}

	if p := examplePlanParent(&a, live); p == nil || p.ID != a.ID {
		t.Fatal("the point added this turn wins")
	}
	if p := examplePlanParent(nil, live); p == nil || p.ID != b.ID {
		t.Fatal("otherwise the last point on the map — never an example")
	}
	if p := examplePlanParent(nil, live[:1]); p != nil {
		t.Fatal("no point at all: stays where it is")
	}
}

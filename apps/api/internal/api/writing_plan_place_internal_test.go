package api

import (
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// 2026-09-18 本地实测：她说「还有 脆弱的感受往往也带来很多关于自己渴望的信息」，
// 印记在 reply 里转述了它，add 是空的 —— 截图 1 里「多说了一条，图上没有」的那一幕。
func TestPlanTurnDroppedHerPoint(t *testing.T) {
	rows := []sqlc.WritingOutline{
		{ID: uuid.New(), Depth: 0, Text: "脆弱不可怕，人是可以脆弱的"},
		{ID: uuid.New(), Depth: 1, Text: "脆弱让人区别于机器"},
	}
	if !planTurnDroppedHerPoint("还有 脆弱的感受往往也带来很多关于自己渴望的信息", rows, 0) {
		t.Error("a new point with nothing placed this turn is dropped")
	}
	for _, said := range []string{
		"还有 脆弱的感受往往也带来很多关于自己渴望的信息", // placed this turn → see below
	} {
		if planTurnDroppedHerPoint(said, rows, 1) {
			t.Error("something landed this turn: not dropped")
		}
	}
	for _, said := range []string{
		"可以",  // 太短
		"不知道", // 等于没答
		"那我要怎么找历史上的例子呢？",    // 问句
		"好了，我想开始写了，先这样吧",    // 要去写了
		"脆弱让人区别于机器。",        // 图上已经有
		"脆弱让人区别于机器，这条我想放第二", // 包含图上的那一句
	} {
		if planTurnDroppedHerPoint(said, rows, 0) {
			t.Errorf("%q should not count as a dropped point", said)
		}
	}
}

func TestStudentWantsToWrite(t *testing.T) {
	for _, s := range []string{"我想开始写了", "先这样，去写吧", "ok let me write now"} {
		if !studentWantsToWrite(s) {
			t.Errorf("%q wants to write", s)
		}
	}
	for _, s := range []string{"脆弱让人区别于机器", "我还想再加一个例子"} {
		if studentWantsToWrite(s) {
			t.Errorf("%q does not", s)
		}
	}
}

func TestPlanAddsThatLand(t *testing.T) {
	thesis := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Text: "论点"}
	rows := []sqlc.WritingOutline{thesis}
	// 🚨 「挂在一个认不出的父亲上所以被丢掉」那两条没有了：模型不再给位置。
	// 剩下会被丢掉的只有空白和重复。
	add := []writingPlanAdd{
		{Kind: writingKindPoint, Text: "理由A"},
		{Kind: writingKindPoint, Text: "理由B"},
		{Kind: writingKindReference, Text: "司马迁受宫刑"},
		{Kind: writingKindThesis, Text: "论点"}, // duplicate → dropped
		{Kind: writingKindPoint, Text: "  "},  // blank → dropped
	}
	if n := planAddsThatLand(rows, add); n != 3 {
		t.Fatalf("lands = %d, want 3", n)
	}
}

// 2026-09-18 线上验证第二轮：第三条分论点被挂在第二条下面（深度 2），被当成了一个
// 「不是个人经历的例子」，于是只有她自己那一件事就放她去写了。
func TestDeepPointIsNotAnExample(t *testing.T) {
	// 🚨 第三条分论点被挂在第二条下面（深度 2）。0182 之后这种摆法建不出来了，
	// 但老数据里有，而且判据仍然必须认它是一条分论点 —— kind 说了算，深度不算数。
	rows := []sqlc.WritingOutline{
		{Kind: writingKindThesis, Depth: 0, Text: "人可以脆弱", Role: "中心论点"},
		{Kind: writingKindPoint, Depth: 1, Text: "经历脆弱让人更沉着", Role: "分论点"},
		{Kind: writingKindEvidence, Depth: 2, Text: "爸爸入狱、妹妹抑郁", Role: "论据 · 你见过的事"},
		{Kind: writingKindPoint, Depth: 1, Text: "脆弱让人区别于机器", Role: "分论点"},
		{Kind: writingKindPoint, Depth: 2, Text: "脆弱的感受带来渴望的信息", Role: "分论点"},
	}
	s := writingPlanShapeOf(rows)
	if s.Material != 1 || s.Wider != 0 || s.ready(writingPlanNeedOf(sqlc.Writing{})) {
		t.Fatalf("shape = %+v; a deeper 分论点 is not an example, and one personal example is not enough", s)
	}
}

// 2026-09-18 线上验证：一条「分论点」落在了最上层，和中心论点平级。
//
// 🚨 2026-09-20：thesisPlanNode 和 writingRoleIsPoint 都删掉了 —— 那两个函数
// 是在「模型把分论点摆到了最上层」之后把它认回来。现在分论点只能在深度 1，
// 这个用例改成钉住那条不变量本身。
func TestAPointCanOnlyEverBeDepthOne(t *testing.T) {
	open := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindOpening, Depth: 0, Position: 0, Text: "从一件小事说起"}
	thesis := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 1, Text: "人可以脆弱"}
	rows := []sqlc.WritingOutline{open, thesis}

	if d := writingKindDepth(writingKindPoint); d != 1 {
		t.Fatalf("分论点的深度 = %d，只能是 1", d)
	}
	if p := writingKindParentOf(writingKindPoint, rows); p == nil || p.ID != thesis.ID {
		t.Fatal("分论点挂在中心论点下面，不和它平级")
	}
	// 图上还没有中心论点：挂不上，由 insertPlanNode 决定怎么办（它会让这一条
	// 自己当中心论点），而不是悄悄挂到开篇下面。
	if p := writingKindParentOf(writingKindPoint, []sqlc.WritingOutline{open}); p != nil {
		t.Fatalf("没有中心论点时不该挂到 %v 上", p.Kind)
	}
}

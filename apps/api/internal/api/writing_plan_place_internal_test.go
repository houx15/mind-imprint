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
	thesis := sqlc.WritingOutline{ID: uuid.New(), Depth: 0, Text: "论点"}
	rows := []sqlc.WritingOutline{thesis}
	byID := map[string]sqlc.WritingOutline{thesis.ID.String(): thesis}
	add := []writingPlanAdd{
		{ParentID: thesis.ID.String(), Text: "理由A", Role: "分论点"},
		{ParentID: "n1", Text: "理由B", Role: "分论点"},       // unknown parent → dropped
		{ParentID: "n1", Text: "司马迁受宫刑", Role: "历史上的例子"}, // example → re-homed, lands
		{ParentID: "", Text: "论点", Role: "中心论点"},         // duplicate → dropped
		{ParentID: "", Text: "  ", Role: "分论点"},          // blank → dropped
	}
	if n := planAddsThatLand(rows, byID, add); n != 2 {
		t.Fatalf("lands = %d, want 2", n)
	}
}

// 2026-09-18 线上验证：一条「分论点」落在了最上层，和中心论点平级。
func TestThesisPlanNodeAndPointRole(t *testing.T) {
	open := sqlc.WritingOutline{ID: uuid.New(), Depth: 0, Position: 0, Text: "从一件小事说起", Role: "开头"}
	thesis := sqlc.WritingOutline{ID: uuid.New(), Depth: 0, Position: 1, Text: "人可以脆弱", Role: "中心论点"}
	stray := sqlc.WritingOutline{ID: uuid.New(), Depth: 0, Position: 2, Text: "变故不必然打垮人", Role: "分论点"}
	if p := thesisPlanNode([]sqlc.WritingOutline{open, stray, thesis}); p == nil || p.ID != thesis.ID {
		t.Fatal("the thesis is the first top-level node that is not an opening, example or point")
	}
	if thesisPlanNode([]sqlc.WritingOutline{open}) != nil {
		t.Fatal("no thesis yet")
	}
	for role, want := range map[string]bool{
		"分论点": true, "一条理由（讲道理）": true, "中心论点": false, "你经历过的事": false, "开头": false,
	} {
		if writingRoleIsPoint(role) != want {
			t.Errorf("writingRoleIsPoint(%q) = %v", role, !want)
		}
	}
}

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

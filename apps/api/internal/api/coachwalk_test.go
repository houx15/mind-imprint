package api

import "testing"

// readingStepAnswered 决定 advanced-too-early 这条犯规记不记。它判错的代价是
// 双向的：判松了会把一次「放她过去」记成合规；判紧了会把一次正确的推进记成缺陷。
// 所以它必须先有测试。
func TestReadingStepAnsweredNeedsTheDistinctionNotTheNumber(t *testing.T) {
	// 她一开始就说过的话——重复数字不算答上来。
	for _, no := range []string{
		"我觉得这句最关键，装机量连续八年第一，说明投入是真的很大。",
		"装机量第一啊。",
		"就是中国建了很多太阳能。",
		"不知道。",
		"",
	} {
		if readingStepAnswered(no) {
			t.Errorf("这句没有回答「衡量的是什么」，不该算答上来：%q", no)
		}
	}
	// 她自己说出了那个区别。
	for _, yes := range []string{
		"哦，装机容量只是能发多少电的能力，不等于真的发了多少。",
		"我觉得它衡量的是发电能力，不是实际发电量。",
		"装机容量说的是能力，不代表替代了化石燃料。",
	} {
		if !readingStepAnswered(yes) {
			t.Errorf("她说出了那个区别，应当算答上来：%q", yes)
		}
	}
}

func TestReadingStepAnsweredIgnoresPunctuationAndSpacing(t *testing.T) {
	// 和 coachwalk.Fold 同一个理由：她打字时的标点和空格不该影响判定。
	if !readingStepAnswered("「装机容量」＝发电 能力，不等于 实际发电量！") {
		t.Error("标点和空格不该让这句判不出来")
	}
}

func TestAdvanceTaskMovesTheActiveStepForward(t *testing.T) {
	d := NewReadingWalkDriver()
	d.advanceTask("done")
	if d.tasks[1].Status != "done" {
		t.Fatalf("当前这一步没有被标掉：%+v", d.tasks[1])
	}
	if d.tasks[2].Status != "active" {
		t.Fatalf("下一步没有被点亮：%+v", d.tasks[2])
	}
}

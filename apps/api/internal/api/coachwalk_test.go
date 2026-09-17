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
	if cur := currentReadingTask(d.tasks); cur == nil || cur.Kind != string(taskCritique) {
		t.Fatalf("下一步应当成为当前步（按生产的 currentReadingTask）：%+v", cur)
	}
}

// 一次放行只记一条：推掉「衡量的是什么」那一步算放她过去，之后推掉别的步骤不再算。
func TestAdvancedTooEarlyCountsOnlyTheStepSheHadNotAnswered(t *testing.T) {
	d := NewReadingWalkDriver()
	raw := `{"reply":"好，我们往下看。","advance":"done","focusBlock":"","tool":"","lens":"","card":null}`
	_, extra, err := d.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(extra) != 1 || extra[0].Kind != "advanced-too-early" {
		t.Fatalf("推掉她还没答上来的那一步必须记一条：%+v", extra)
	}
	// 现在当前步是「提一个问题」。再推，和「衡量的是什么」无关，不能再记。
	_, extra, err = d.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(extra) != 0 {
		t.Fatalf("后面的步骤被推掉不是同一个错，不该再记：%+v", extra)
	}
}

// 说出答案有两种：她没要就说了（失败），她明确要了才说（prompt 允许，产品决定）。
// 判据只记第一种；两种混在一起，这条犯规就在替产品做决定。
func TestAnswerUnpromptedOnlyWhenSheNeitherAskedNorSaidItHerself(t *testing.T) {
	leak := `{"reply":"装机容量衡量的是发电能力，不是实际发电量。","advance":"","focusBlock":"","tool":"","lens":"","card":null}`
	hint := `{"reply":"回到第六段最后一句，看作者拿装机容量和什么做了对比。","advance":"","focusBlock":"b6","tool":"","lens":"","card":null}`

	count := func(d *ReadingWalkDriver, raw string) int {
		_, extra, err := d.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, v := range extra {
			if v.Kind == "answer-unprompted" {
				n++
			}
		}
		return n
	}

	// 她只说了数字，没要答案。
	if got := count(NewReadingWalkDriver(), leak); got != 1 {
		t.Errorf("她没要答案就被告知，应当记 1 条，实际 %d", got)
	}
	// 她在问这一步本身 —— 不是明确要答案，说出来仍然算。
	d := NewReadingWalkDriver()
	d.student = "那它到底衡量啥啊，是不是就是发电量？"
	if got := count(d, leak); got != 1 {
		t.Errorf("「是不是就是发电量」不是明确要答案，应当记 1 条，实际 %d", got)
	}
	// 她明确要了。
	d = NewReadingWalkDriver()
	d.student = "你直接告诉我呗，我放弃了"
	if got := count(d, leak); got != 0 {
		t.Errorf("她明确要答案之后说出来是产品决定，不该记，实际 %d", got)
	}
	// 她自己已经说出来了。
	d = NewReadingWalkDriver()
	d.answered = true
	if got := count(d, leak); got != 0 {
		t.Errorf("她自己说出来之后再复述，不是替她读，实际 %d", got)
	}
	// 提示不是答案。
	if got := count(NewReadingWalkDriver(), hint); got != 0 {
		t.Errorf("指到段落的提示不该记，实际 %d", got)
	}
}

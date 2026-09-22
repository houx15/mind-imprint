package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 通读那一步给 done 要有真凭据。产品负责人 2026-09-22 报的第 4 条：
// 她问了一句关于内容的问题，那一步就被判读完了。

func readTasks() []sqlc.ReadingTask {
	return []sqlc.ReadingTask{
		{Kind: string(taskRead), Label: "通读第12–16段·大脑的成本", Status: "pending", Position: 1},
		{Kind: string(taskRead), Label: "通读第17–20段·作者的结论", Status: "pending", Position: 2},
	}
}

func TestReadStepNeedsEvidenceToSettle(t *testing.T) {
	tasks := readTasks()
	cur := &tasks[0]

	t.Run("她只是问了一句关于内容的问题", func(t *testing.T) {
		got := guardReadStepAdvance("done", cur, nil,
			"那和选项多导致我大脑累有啥关系",
			"这是个好问题，而且它接上的正是第14段：解决问题比记忆更费能量。", tasks)
		if got != "" {
			t.Fatalf("a content question settled the read step: advance=%q", got)
		}
	})

	t.Run("她答了这一步的卡", func(t *testing.T) {
		ans := &coachCardAnswer{Type: coachCardChooseSpan, Choice: "解决问题比记忆更费能量"}
		if got := guardReadStepAdvance("done", cur, ans, "", "接住了。", tasks); got != "done" {
			t.Fatalf("a real card answer must settle the step, got %q", got)
		}
	})

	t.Run("她自己说读完了", func(t *testing.T) {
		if got := guardReadStepAdvance("done", cur, nil, "读完了", "好。", tasks); got != "done" {
			t.Fatalf("her own 读完了 must settle the step, got %q", got)
		}
	})

	t.Run("她说还没读完", func(t *testing.T) {
		if got := guardReadStepAdvance("done", cur, nil, "我还没读完", "好。", tasks); got != "" {
			t.Fatalf("还没读完 must NOT settle the step, got %q", got)
		}
	})

	t.Run("印记已经把她领去下一个部分", func(t *testing.T) {
		got := guardReadStepAdvance("done", cur, nil, "嗯",
			"接下来读第17到20段，看作者最后给出了什么结论。", tasks)
		if got != "done" {
			t.Fatalf("a reply that sends her to the next part must settle the step, got %q", got)
		}
	})

	t.Run("skipped 照放行", func(t *testing.T) {
		if got := guardReadStepAdvance("skipped", cur, nil, "这步跳过吧", "好，跳过。", tasks); got != "skipped" {
			t.Fatalf("an explicit skip must stand, got %q", got)
		}
	})

	t.Run("别的步不受这条管", func(t *testing.T) {
		hunt := sqlc.ReadingTask{Kind: string(taskHunt), Label: "找出关键句", Status: "pending"}
		if got := guardReadStepAdvance("done", &hunt, nil, "随便说一句", "好。", tasks); got != "done" {
			t.Fatalf("the guard must only touch read steps, got %q", got)
		}
	})
}

// 🚨 否定要先查：「还没读完」里含着「读完」。判错的方向不对称 ——
// 她还没读、产品说她读了，是最伤的那个。
func TestStudentSaysSheFinishedReading(t *testing.T) {
	yes := []string{"读完了", "这几段我看完了", "都读了", "看了一遍", "finished reading", "I have read it"}
	no := []string{"", "还没读完", "我没看完", "这段什么意思", "not yet", "第14段在说什么"}
	for _, s := range yes {
		if !studentSaysSheFinishedReading(s) {
			t.Errorf("should read as finished: %q", s)
		}
	}
	for _, s := range no {
		if studentSaysSheFinishedReading(s) {
			t.Errorf("should NOT read as finished: %q", s)
		}
	}
}

// 耗满六轮那条替她推进的规则排在这道闸后面，所以这道闸不会把她钉在原地。
// 这里只钉住调用顺序这个事实本身：guard 不认识 stalled，stalled 不经过 guard。
func TestReadStepGuardLeavesTheStallRuleAlone(t *testing.T) {
	tasks := readTasks()
	cur := &tasks[0]
	// 模型这一轮什么都没给（advance 空）——闸不动它，耗满六轮那条才有机会开口。
	if got := guardReadStepAdvance("", cur, nil, "不知道", "再看一眼第13段。", tasks); got != "" {
		t.Fatalf("the guard must not invent an advance, got %q", got)
	}
}

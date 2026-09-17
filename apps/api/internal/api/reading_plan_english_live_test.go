package api

// reading_plan_english_live_test.go —— 排读法这一步的实测。
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveEnglishReadingPlan -v -count=1
//
// 2026-09-08 线上验证时发现：同一个「开始」按钮上有**两个**互不相干的毛病。
// 教练那一轮修好之后，四次里仍有三次 502，日志里换成了另一句：
//
//	WARN reading plan: unparseable or out-of-library reply
//
// 那一句把三种失败裹成一个 bool —— JSON 读不了 / routineKey 不在库里 /
// 挑的读法语言不对。三种的修法完全不同，所以这个测试先把它们分开数。

import (
	"context"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
)

func TestLiveEnglishReadingPlanParses(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassCompose)
	blocks := liveEnglishBlocks()
	prompt := buildReadingPlanPrompt("en", "Is skipping breakfast a moral failure?", blocks)

	bad, noOutline, noGist, noParts := 0, 0, 0, 0
	for i := 0; i < 6; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: readingPlanSystem},
				{Role: gateway.RoleUser, Content: prompt},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("sample %d: live call failed (%s): %v", i, r.ModelID, err)
		}
		plan, routine, reject := parseReadingPlan(res.Text, "en")
		t.Logf("sample %d — out=%d stop=%q len=%d", i, res.Usage.OutputTokens, res.StopReason, len(res.Text))
		if reject != planOK {
			bad++
			t.Logf("sample %d: REJECTED (%s)\n--- raw ---\n%s\n--- end ---", i, reject, res.Text)
			continue
		}
		// 导读（2026-09-16 加了 gist 和 parts）。它和读法清单同一次调用产出，
		// 所以在这里一起验 —— 它**过不了校验是静默的**：屏幕上少一张卡片，
		// 日志里一行 info，而通读那一步会退回没有台阶的那一种。
		//
		// 🚨 顺序：2026-09-17 起清单要用切法（通读一步一个部分），所以导读得先
		// 校验出来 —— 和 planReadingTasks 里的顺序一致。
		out, okOutline := validateOutline(plan.outline(), blocks)

		positions, kinds, labels, _, _ := buildReadingTasks(routine, plan, blocks, out.Parts)
		t.Logf("sample %d ok — routine=%s focus=%v steps=%d", i, routine.Key, plan.FocusBlocks, len(positions))
		if len(positions) == 0 {
			t.Errorf("sample %d: routine produced no usable steps", i)
		}
		reads, focuses := 0, 0
		for n, k := range kinds {
			switch k {
			case string(taskRead):
				reads++
				t.Logf("           read → %s", labels[n])
			case string(taskFocusBlock):
				focuses++
				t.Logf("           focus → %s", labels[n])
			}
		}
		// 🚨 这两条是 2026-09-17 那两个 bug 的实测门槛。两者都**不会报错**：
		// 通读退回一步就是「读完告诉我一声」那个死锁，精读只走一步就是
		// 「整篇的交互集中在两三段」。
		if len(out.Parts) >= outlinePartsMin && reads < 2 {
			t.Errorf("sample %d: 切出了 %d 个部分，清单上却只有 %d 个通读步",
				i, len(out.Parts), reads)
		}
		if len(blocks) >= 10 && focuses < 2 {
			t.Logf("sample %d: %d 段的文章只挑了 %d 段精读", i, len(blocks), focuses)
		}

		if !okOutline {
			noOutline++
			// 🚨 数出核心段有几段。四条作废理由里只有「核心段超了」是**程度**
			// 问题（另外三条是「没写中文」「一段核心都没有」「整份是空的」），
			// 而日志分不出是哪一条的时候，只能猜。2026-09-17 实测 3/6 作废，
			// 就是靠这一行才看出全部是同一条。
			core := 0
			for _, b := range blocks {
				if plan.Load[b.ID] == loadCore {
					core++
				}
			}
			t.Logf("sample %d: 导读被整份丢掉 —— oneLine=%q gist=%q load=%d 核心段=%d/%d",
				i, plan.OneLine, plan.Gist, len(plan.Load), core, len(blocks))
			continue
		}
		t.Logf("sample %d 导读 — 在问=%q", i, out.OneLine)
		t.Logf("           中心思想=%q", out.Gist)
		t.Logf("           结构=%q 核心段=%d", out.Shape, len(out.coreBlockIDs(blocks)))
		if out.Gist == "" {
			noGist++
			t.Logf("sample %d: 没有中心思想 —— 她一进来看到的还是只有问题没有答案", i)
		}
		if len(out.Parts) == 0 {
			noParts++
			t.Logf("sample %d: 切法没留下来 —— 通读那一步没有台阶。模型给的是 %+v", i, plan.Parts)
			continue
		}
		for _, pt := range out.Parts {
			t.Logf("           · %s（%s–%s）%s", pt.Title, pt.From, pt.To, pt.Does)
		}
	}
	t.Logf("RESULT: %d/6 rejected · 导读没了 %d/6 · 没有中心思想 %d/6 · 没有切法 %d/6",
		bad, noOutline, noGist, noParts)
	if bad > 0 {
		t.Errorf("%d/6 English plans unparseable — 「开始」按下去是 502", bad)
	}
	// 🚨 这三条是**这一轮新 prompt 的实测门槛**，不是可选的。一份没有中心思想
	// 或者没有切法的导读不会报错，它只是少了半张卡片和整条通读的台阶 ——
	// 而那正是 2026-09-16 那次走查要修的东西。半数以上拿不到就是 prompt 没写对。
	if noOutline*2 > 6 {
		t.Errorf("%d/6 的导读整份作废 —— 她一进阅读室看不到地图", noOutline)
	}
	if noGist*2 > 6 {
		t.Errorf("%d/6 没有中心思想 —— 没读过这篇的学生还是跟不上", noGist)
	}
	if noParts*2 > 6 {
		t.Errorf("%d/6 没有切法 —— 通读又变回「读完告诉我一声」", noParts)
	}
}

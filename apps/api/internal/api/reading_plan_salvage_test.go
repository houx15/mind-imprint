package api

// reading_plan_salvage_test.go —— 写坏了的排读法回复，能不能救回那张清单。
//
// 第一个样本是 2026-09-08 线上日志里**原样抄下来**的那一份（reply_len=92，
// finish_reason "stop"）：写完了，但 steps 里一个冒号该是逗号，detail 留成了
// 占位符。她按下「开始」，得到 502。

import "testing"

func TestReadingPlanSurvivesBrokenSteps(t *testing.T) {
	blocks := liveEnglishBlocks()
	raw := `{"routineKey":"en-argument","focusBlocks":["b5","b8"],` +
		`"steps":[{"kind":"read":"detail..."}]}`

	plan, routine, reject := parseReadingPlan(raw, "en")
	if reject != planOK {
		t.Fatalf("plan thrown away (%s) — 「开始」按下去是 502，而读法和精读段都是齐的", reject)
	}
	if routine.Key != "en-argument" {
		t.Errorf("routine = %q, want en-argument", routine.Key)
	}
	if len(plan.FocusBlocks) != 2 || plan.FocusBlocks[0] != "b5" {
		t.Errorf("focusBlocks lost: %v", plan.FocusBlocks)
	}
	// steps 写坏了就当它没给 —— 清单照样排得出来，detail 用读法库自己那一句。
	positions, kinds, labels, details, _ := buildReadingTasks(routine, plan, blocks)
	if len(positions) != len(routine.Steps) {
		t.Fatalf("got %d steps, want the routine's %d", len(positions), len(routine.Steps))
	}
	// 第一步的 kind 来自读法库自己，不是写死的 "read" —— 库里加一步（2026-09-10
	// 加了「先预测」）不该让这条测试变红：它测的是「steps 写坏了，清单照样排得
	// 出来」，不是「第一步一定叫 read」。
	if want := string(routine.Steps[0].Kind); kinds[0] != want {
		t.Errorf("first step kind = %q, want the routine's own %q", kinds[0], want)
	}
	if labels[0] == "" || details[0] == "" {
		t.Errorf("first step is not usable: label=%q detail=%q", labels[0], details[0])
	}
}

// 没有 routineKey 就没有读法，也就没有清单 —— 这一种仍然要失败。
func TestReadingPlanRejectedWithoutRoutineKey(t *testing.T) {
	if _, _, reject := parseReadingPlan(`{"focusBlocks":["b1"],"steps":[`, "en"); reject != planRejectUnparseable {
		t.Fatalf("reject = %q, want unparseable", reject)
	}
}

// 挑了一套中文读法给英文文章，仍然要拒 —— 救援不许绕过库的校验。
func TestReadingPlanStillChecksTheLibrary(t *testing.T) {
	broken := `{"routineKey":"zh-argument","focusBlocks":["b1"],"steps":[{"kind":"read":"x"}]}`
	if _, _, reject := parseReadingPlan(broken, "en"); reject == planOK {
		t.Fatalf("a zh routine was accepted for an English article")
	}
}

package api

// reading_plan_parts_internal_test.go —— 清单的形状：通读一步一个部分，
// 精读一步一段。
//
// 这两条守的是 2026-09-17 那两个 bug 的判据，而且都是**结构**上的，不是 prompt
// 里的一句话：
//
//   - 通读：在这之前「一部分一部分地走」只是 system prompt 里的一段散文，而
//     每一轮末尾的推进判据写着「答了通读卡片就 done」—— 散文跨不过判据，一张卡
//     答完，整个通读当场结束。产品负责人在一篇 17 段的文章上逐字指出了这一幕
//     （「马上就转到精读了」）。
//   - 精读：排读法被要求挑 1–2 段，而读法库里只有一个精读槽，多出来的那一段
//     **静默丢掉**。后果是「整篇的交互就集中在 2-3 个段落，其他的段落完全放置了」。

import (
	"strings"
	"testing"
)

func TestBuildReadingTasks_ReadSplitsIntoParts(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "一"}, {ID: "b2", Text: "二"}, {ID: "b3", Text: "三"},
		{ID: "b4", Text: "四"}, {ID: "b5", Text: "五"}, {ID: "b6", Text: "六"},
	}
	routine, ok := findReadingRoutine("zh-scan-focus-lens")
	if !ok {
		t.Fatal("the default zh routine is gone")
	}
	parts := []readingPart{
		{Title: "提出争议", From: "b1", To: "b2", Does: "摆出两方的说法"},
		{Title: "实测数据", From: "b3", To: "b5", Does: "用一组数据支持前面那个判断"},
		{Title: "收束", From: "b6", To: "b6"},
	}
	_, kinds, labels, details, blockIDs := buildReadingTasks(
		routine, readingPlanReply{FocusBlocks: []string{"b4"}}, blocks, parts)

	var readIdx []int
	for i, k := range kinds {
		if k == string(taskRead) {
			readIdx = append(readIdx, i)
		}
	}
	if len(readIdx) != len(parts) {
		t.Fatalf("read steps = %d, want one per part (%d)", len(readIdx), len(parts))
	}
	// 它们必须是连着的：通读的几个部分不该被别的步骤插开。
	for n := 1; n < len(readIdx); n++ {
		if readIdx[n] != readIdx[n-1]+1 {
			t.Errorf("read steps are not contiguous: %v", readIdx)
		}
	}
	wantLabels := []string{"通读第1–2段·提出争议", "通读第3–5段·实测数据", "通读第6段·收束"}
	for n, i := range readIdx {
		if labels[i] != wantLabels[n] {
			t.Errorf("read step %d label = %q, want %q", n, labels[i], wantLabels[n])
		}
		// 每一步都得说清它管哪几段 —— 她照着它去读，说错了她就读错地方。
		if !strings.Contains(details[i], "请通读第") {
			t.Errorf("read step %d detail does not name its paragraphs: %q", n, details[i])
		}
		// blockId 让「定位原文」跳到这一部分的头一段。
		if blockIDs[i] != parts[n].From {
			t.Errorf("read step %d blockId = %q, want %q", n, blockIDs[i], parts[n].From)
		}
	}
}

// 没有切法（短文章，或者模型那份切法没过校验）→ 通读退回整篇一步。
func TestBuildReadingTasks_NoPartsKeepsOneReadStep(t *testing.T) {
	blocks := []Block{{ID: "b1", Text: "一"}, {ID: "b2", Text: "二"}}
	routine, ok := findReadingRoutine("zh-scan-focus-lens")
	if !ok {
		t.Fatal("the default zh routine is gone")
	}
	_, kinds, labels, _, _ := buildReadingTasks(
		routine, readingPlanReply{FocusBlocks: []string{"b2"}}, blocks, nil)
	reads := 0
	for i, k := range kinds {
		if k == string(taskRead) {
			reads++
			if labels[i] != "通读全文" {
				t.Errorf("read label = %q, want the routine's own 通读全文", labels[i])
			}
		}
	}
	if reads != 1 {
		t.Errorf("read steps = %d, want 1 when there are no parts", reads)
	}
}

// 精读挑几段就走几步，上限是 maxFocusSteps；同一段挑两次只走一步。
func TestBuildReadingTasks_FocusStepPerBlock(t *testing.T) {
	blocks := []Block{{ID: "b1"}, {ID: "b2"}, {ID: "b3"}, {ID: "b4"}, {ID: "b5"}}
	routine, ok := findReadingRoutine("zh-scan-focus-lens")
	if !ok {
		t.Fatal("the default zh routine is gone")
	}
	_, kinds, labels, _, blockIDs := buildReadingTasks(routine, readingPlanReply{
		// 五个候选，其中一个重复：存活的应该是前三段，各走一步。
		FocusBlocks: []string{"b2", "b3", "b2", "b4", "b5"},
	}, blocks, nil)
	got := []string{}
	for i, k := range kinds {
		if k == string(taskFocusBlock) {
			got = append(got, blockIDs[i])
			if !strings.Contains(labels[i], "第") {
				t.Errorf("focus label carries no paragraph number: %q", labels[i])
			}
		}
	}
	if len(got) != maxFocusSteps {
		t.Fatalf("focus steps = %d (%v), want %d", len(got), got, maxFocusSteps)
	}
	for _, want := range []string{"b2", "b3", "b4"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("focus steps %v are missing %s", got, want)
		}
	}
}

// 精读的每一步都要带上真实的段号，而且两步不能指同一段 —— 她在进度盘上看到
// 两个一模一样的「精读重点段落第4段」，会以为产品重复了。
//
// 顺带守住 detail：模型那句是对着**一段**写的，复制到第二步上就是一句关于
// 别的段落的假话。
func TestBuildReadingTasks_FocusLabelsAreDistinct(t *testing.T) {
	blocks := []Block{{ID: "b1"}, {ID: "b2"}, {ID: "b3"}}
	routine, ok := findReadingRoutine("zh-scan-focus-lens")
	if !ok {
		t.Fatal("the default zh routine is gone")
	}
	_, kinds, labels, details, _ := buildReadingTasks(routine, readingPlanReply{
		FocusBlocks: []string{"b2", "b3"},
		Steps: []struct {
			Kind   string `json:"kind"`
			Detail string `json:"detail"`
		}{
			{Kind: "predict", Detail: "只看标题猜一猜。"},
			{Kind: "read", Detail: "先通读一遍。"},
			{Kind: "focus_block", Detail: "这一段是全文唯一给出数据的地方。"},
		},
	}, blocks, nil)
	seen := map[string]bool{}
	var focusDetails []string
	for i, k := range kinds {
		if k != string(taskFocusBlock) {
			continue
		}
		if seen[labels[i]] {
			t.Errorf("two focus steps share the label %q", labels[i])
		}
		seen[labels[i]] = true
		focusDetails = append(focusDetails, details[i])
	}
	if len(seen) != 2 {
		t.Fatalf("distinct focus labels = %d, want 2", len(seen))
	}
	if focusDetails[0] != "这一段是全文唯一给出数据的地方。" {
		t.Errorf("the first focus step should keep the model's own line, got %q", focusDetails[0])
	}
	if focusDetails[1] == focusDetails[0] {
		t.Errorf("the second focus step reuses a line written about another paragraph: %q", focusDetails[1])
	}
}

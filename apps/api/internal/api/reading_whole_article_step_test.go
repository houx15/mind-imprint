package api

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// 产品负责人 2026-09-23 第 5 条：
//
//	「when reading a 记叙文, it always focuses on very detailed things and
//	  ignores the general structure, the writing format etc. to give guidance.」
//
// 🚨 这条测试钉的是**那个毛病的形状**，不是记叙文那一套的步数。
//
// 当时 zh-narrative 七步里除了 sequence 之外每一步都在句子或段落那一层
// （精读一段、给几句话贴描写类型、说一个人物的动机、点一句写得好的），
// 而它是九套读法里唯一既没有 predict 也没有 reflect 的一套 —— 整篇那一层
// 根本没有位置。陪练不是不想谈结构，是清单上没有一步请它谈。
//
// 一套读法少了整篇那一层**不报错、也不会让任何别的测试变红**，它只是让那个
// 体裁的学生永远谈不到结构。所以要有一条判据盯着它
// （同 reading_lang_coverage_test.go 的理由）。

// 整篇那一层的步骤：它们问的是「这篇作为一篇文章是怎么回事」，
// 而不是某一段、某一句。
var wholeArticleTaskKinds = map[readingTaskKind]bool{
	taskPredict: true, // 读之前先就整篇下一个赌注
	taskShape:   true, // 作者怎么安排这一篇
	taskReflect: true, // 读完之后就整篇说一句
	taskRecall:  true, // 合上文章，整篇复述
}

// 🚨 议论文那三套**不在这条判据的范围里**，而且不是因为它们合格。
//
// en-argument 整篇那一层只有 predict（读之前那一次），读完之后一步都没有 ——
// 同门的 zh-scan-focus-lens 有「总结论点」，en-close-read 有复述，只有它两头
// 都缺。这条判据第一次跑就把它抓出来了，我也确实先补了一条，又撤了回来。
//
// **产品负责人有一条在先的话**，逐字钉在 TestArgumentRoutinesAreUnchanged
// 上面：「don't bother the current experience of argument papers」。
// 所以这里记下这个缺口，不动那三套 —— 要不要补是她的决定，不是我的。
// 记在这里而不是删掉，是为了下一个人读到这条判据时知道：豁免不等于合格。
var wholeArticleExempt = map[string]string{
	"zh-scan-focus-lens": "议论文，产品负责人要求不动",
	"en-close-read":      "议论文，产品负责人要求不动",
	"en-argument":        "议论文，产品负责人要求不动；🚨 它整篇那一层只有 predict，读完之后一步都没有",
}

func TestEveryRoutineHasAWholeArticleStep(t *testing.T) {
	for _, r := range readingRoutines {
		if why, skip := wholeArticleExempt[r.Key]; skip {
			t.Logf("跳过 %s：%s", r.Key, why)
			continue
		}
		var whole []string
		for _, s := range r.Steps {
			if wholeArticleTaskKinds[s.Kind] {
				whole = append(whole, string(s.Kind)+"/"+s.Label)
			}
		}
		if len(whole) < 2 {
			t.Errorf("读法 %s 只有 %d 步在整篇那一层（%v）—— 这一套的学生谈不到结构。"+
				"整篇那一层的步骤有：predict / shape / reflect / recall",
				r.Key, len(whole), whole)
		}
	}
}

// 🚨 记叙文两套都要有 shape。这是第 5 条的直接判据：sequence 排的是**事情**
// 的先后，shape 问的是**作者**为什么这样排 —— 两件事，缺了后一件，
// 「作者怎么写的」就没人问。
func TestBothNarrativeRoutinesAskHowTheWriterArrangedIt(t *testing.T) {
	for _, key := range []string{"zh-narrative", "en-narrative"} {
		var found bool
		var steps int
		for _, r := range readingRoutines {
			if r.Key != key {
				continue
			}
			steps = len(r.Steps)
			for _, s := range r.Steps {
				if s.Kind == taskShape {
					found = true
				}
			}
		}
		if steps == 0 {
			t.Fatalf("找不到读法 %s", key)
		}
		if !found {
			t.Errorf("%s 里没有 shape 那一步 —— 整篇的安排没人问", key)
		}
	}
}

// 陪练对 shape 那一步的规矩里，最要紧的一句是**不要回到某一句上**。
// 清单上别的每一步都在句子或段落那一层，少了这句话，这一步会立刻退回去
// 变成又一次精读 —— 而那正是被报的那个毛病。
//
// 走的是真的那条路（readingCurrentStepInstruction），不是另抄一份规矩。
func TestShapeStepRuleKeepsItAtWholeArticleLevel(t *testing.T) {
	tasks := []sqlc.ReadingTask{
		{ID: uuid.New(), Kind: string(taskShape), Label: "看作者怎么安排这篇", Status: "pending", Position: 1},
	}
	rule := readingCurrentStepInstruction(tasks)
	if rule == "" {
		t.Fatal("shape 那一步没有规矩")
	}
	for _, want := range []string{"整篇的安排", "不回到某一个句子"} {
		if !strings.Contains(rule, want) {
			t.Errorf("shape 的规矩里没有「%s」", want)
		}
	}
	// 三样安排都要点名，否则这一步会塌成只谈顺序。
	for _, want := range []string{"顺序", "详略", "线索"} {
		if !strings.Contains(rule, want) {
			t.Errorf("shape 的规矩里没有「%s」", want)
		}
	}
	// 🚨 它不要求学生同意陪练的说法 —— 和别的「说出你的判断」那几步同一条。
	if !strings.Contains(rule, "不要求") {
		t.Error("shape 的规矩没说「不要求和你一致」")
	}
}

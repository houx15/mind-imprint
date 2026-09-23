package vocab

import (
	"strings"
	"testing"
)

// 产品负责人 2026-09-23：
//
//	「英文的四个（Thesis-body-conclusion、Claim-counterargument-refutation、
//	  Point-by-point comparison、Block comparison）只有定义和例子，
//	  没有判断方法。」
//
// 🚨 她点的是英文那四条，查下来十条整篇结构**一条都没有**。
//
// 这一屏要她做的决定是「我这一篇该用哪一条」，而定义回答的是「它是什么」、
// 例子回答的是「它长什么样」—— 两样都答不了那个问题。少了判断办法不报错、
// 也不会让任何别的测试变红，它只是让这一屏没法用，所以要有一条判据盯着。

func TestEveryWholeArticleStructureCarriesAJudgmentMethod(t *testing.T) {
	var n int
	for _, m := range All() {
		if m.Category != "structure" {
			continue
		}
		n++
		if strings.TrimSpace(m.WhenToUse) == "" {
			t.Errorf("结构 %s（%s）没有判断办法 —— 她挑不出该用哪一条", m.ID, m.Name)
			continue
		}
		// 判断办法要落到她自己做得了的一个动作上，不是又一句定义的改写。
		// 「什么时候用它」这件事在中文里总要说出一个条件或一个检验动作。
		if !strings.Contains(m.WhenToUse, "时用它") && !strings.Contains(m.WhenToUse, "判断办法") {
			t.Errorf("结构 %s 的判断办法读起来像定义，没有说清什么时候用：%s", m.ID, m.WhenToUse)
		}
		// 🚨 和定义不许是同一句话。抄一遍定义等于这一栏没做。
		if strings.TrimSpace(m.WhenToUse) == strings.TrimSpace(m.Definition) {
			t.Errorf("结构 %s 的判断办法就是它的定义", m.ID)
		}
	}
	if n < 10 {
		t.Fatalf("只找到 %d 条整篇结构，这条判据大概查错了东西", n)
	}
}

// 🚨 英文那四条是被点名的那几条，单独再钉一次 —— 它们是这条 bug 的现场。
func TestTheFourEnglishStructuresSheNamedAllHaveOne(t *testing.T) {
	named := []string{
		"struct_en_thesis_body_conclusion",
		"struct_en_counterargument",
		"struct_en_point_by_point",
		"struct_en_block",
	}
	byID := map[string]Method{}
	for _, m := range All() {
		byID[m.ID] = m
	}
	for _, id := range named {
		m, ok := byID[id]
		if !ok {
			t.Fatalf("%s 不在词表里了", id)
		}
		if strings.TrimSpace(m.WhenToUse) == "" {
			t.Errorf("%s 仍然没有判断方法 —— 这正是被报的那一条", id)
		}
	}
}

// 别的条目留空是**对的**：今天只有整篇结构那一层要她在几条之间挑一条。
// 钉住它，免得下一个人以为每一条都该填，顺手写一堆凑数的话。
func TestOnlyStructuresCarryAJudgmentMethodForNow(t *testing.T) {
	for _, m := range All() {
		if m.Category == "structure" {
			continue
		}
		if strings.TrimSpace(m.WhenToUse) != "" {
			t.Errorf("%s（category=%s）也填了判断办法。"+
				"如果这是有意的，把这条判据改掉并写清楚为什么", m.ID, m.Category)
		}
	}
}

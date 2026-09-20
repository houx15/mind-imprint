package api

import (
	"testing"

	"mindimprint/api/internal/vocab"
)

// 行文是第四步（同事 2026-09-20 的意见 4）。
//
// 🚨 服务端这张表和 0184 放开的那条 CHECK 必须一起看：只改一处，
// 她第一次保存就会撞上一个数据库层的冲突，以一个读不懂的 500 出现。
func TestWritingStageAcceptsFlow(t *testing.T) {
	if !validWritingStages["flow"] {
		t.Error("flow 该是一个合法的 stage")
	}
	for _, s := range []string{"outline", "snippets", "draft", "finished"} {
		if !validWritingStages[s] {
			t.Errorf("原有的 %q 不该消失", s)
		}
	}
	if validWritingStages["我编的一步"] {
		t.Error("编出来的 stage 不该合法")
	}
}

// 编出来的结构 id 落库前要丢掉 —— 同 kind、同 verdict，一个道理。
func TestFlowRejectsAnInventedStructure(t *testing.T) {
	if writingFlowStructureValid("我编的一种") {
		t.Error("不在闭表里的结构 id 不该合法")
	}
	if !writingFlowStructureValid("struct_total_part") {
		t.Error("总分式该合法")
	}
	if writingFlowStructureValid("") {
		t.Error("空串不是一个结构")
	}

	// 🚨 认不出的**清空**，不拒绝整次保存：她在板上点了一下，服务端不认那个
	// id，这时候把她别的改动（顺序、每块的方法）一起退回去，代价和收益完全
	// 不成比例。
	if got := writingValidStructureKey("我编的一种"); got != "" {
		t.Errorf("认不出的结构该清空，得到 %q", got)
	}
	if got := writingValidStructureKey("struct_parallel"); got != "struct_parallel" {
		t.Errorf("合法的结构该留着，得到 %q", got)
	}
}

// 一块上只能标**论证方法**那一层。
func TestFlowMethodOnlyAcceptsTheMethodLayer(t *testing.T) {
	// 真的论证方法：留着。
	for _, id := range []string{"point_pee", "point_quote", "point_metaphor", "point_analogy", "point_absurd"} {
		if got := writingValidMethodID(id); got != id {
			t.Errorf("%q 是论证方法，该留着，得到 %q", id, got)
		}
	}

	// 🚨 整篇层的结构标在一块上是句错话 ——「这一段用总分式」讲不通。
	for _, id := range []string{"struct_total_part", "struct_parallel", "struct_progressive", "struct_contrast"} {
		if got := writingValidMethodID(id); got != "" {
			t.Errorf("整篇层的 %q 不该能标在某一块上，得到 %q", id, got)
		}
	}

	// 开篇 / 结尾那几种开法收法也不在这里：它们由 kind 决定，不由她在板上挑。
	for _, id := range []string{"opening_direct", "closing_return"} {
		if got := writingValidMethodID(id); got != "" {
			t.Errorf("%q 不是论证方法，不该能标在一块上，得到 %q", id, got)
		}
	}

	if got := writingValidMethodID("我编的一个"); got != "" {
		t.Errorf("编出来的 id 该清空，得到 %q", got)
	}
	if got := writingValidMethodID("  "); got != "" {
		t.Errorf("空白该清空，得到 %q", got)
	}
}

// 板上那四张卡都要有一句示范 —— 光给定义，「层进式」和「并列式」
// 在一个中学生眼里是同一句话。
func TestFlowStructuresAllCarryAnExample(t *testing.T) {
	got := vocab.Structures()
	if len(got) != 4 {
		t.Fatalf("论证结构应当有 4 条，得到 %d", len(got))
	}
	for _, m := range got {
		if len(m.Examples) == 0 {
			t.Errorf("%s 没有示范", m.Name)
		}
		// 🚨 示范讲的必须是**别的题目**（借来的材料，铁律①）。
		for _, e := range m.Examples {
			if e.Topic == "" {
				t.Errorf("%s 的示范没写它讲的是哪个题目 —— 那一行是它不被当成她自己文章的全部依据", m.Name)
			}
		}
	}
}

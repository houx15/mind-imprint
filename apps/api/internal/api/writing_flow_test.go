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

// 板上那几张卡都要有一句示范 —— 光给定义，「层进式」和「并列式」
// 在一个中学生眼里是同一句话。
//
// 🚨 数目按**文体**数（R4 起库里同时有两种文体的结构）。
// 原来这里只写了一个 4，那时候整篇层只有议论文那四条；加进记叙文的
// 抑扬转情法之后它就成了 5，而那条测试本来要守的是「每张卡有示范」，
// 不是「库里一共几条」。分文体数回答的才是她那一块屏幕上看见几张卡。
func TestFlowStructuresAllCarryAnExample(t *testing.T) {
	// 语言这条轴 2026-09-22 补上（同事的意见 7）：一篇英文议论文要看的是
	// thesis-body-conclusion 那一套，不是总分式。
	if n := len(vocab.Structures(genreArgument, "zh")); n != 4 {
		t.Fatalf("中文议论文的论证结构应当有 4 条（总分/并列/层进/对照），得到 %d", n)
	}
	// 🚨 2026-09-23 从 1 条变成 7 条。产品负责人：「记叙文 has many kinds of
	// 叙事结构, e.g. 时间顺序, or 倒叙, or 插叙, 一波三折, 双线并进,
	// 以物为线索……if we always ask students to write one thing, it is too
	// fixed.」在那之前中文记叙文整篇那一层只有抑扬转情法一条 ——
	// 「行文」那一屏对一篇记叙文来说是一道单选题。
	if n := len(vocab.Structures(genreNarrative, "zh")); n != 7 {
		t.Fatalf("中文记叙文的结构应当有 7 条（抑扬转情法 + 六种叙事结构），得到 %d", n)
	}
	if n := len(vocab.Structures(genreArgument, "en")); n != 4 {
		t.Fatalf("英文议论文的结构应当有 4 条，得到 %d", n)
	}
	if n := len(vocab.Structures(genreNarrative, "en")); n != 1 {
		t.Fatalf("英文记叙文的结构应当有 1 条（narrative arc），得到 %d", n)
	}
	// 🚨 两边不许交叉：一篇记叙文里没有分论点，「它们之间是并列还是层进」
	// 是句问不出口的话。
	//
	// 判据从「只能是 struct_yiyang」改成「每一条都标着记叙文」—— 原来那一版
	// 钉的是**名单**，加一条新结构它就红，而它真正要守的是不许混进议论文那几条。
	for _, m := range vocab.Structures(genreNarrative, "zh") {
		if m.Genre != genreNarrative {
			t.Errorf("记叙文的结构里混进了 %q（genre=%q）", m.ID, m.Genre)
		}
	}
	for _, m := range vocab.Structures(genreArgument, "zh") {
		if m.Genre != genreArgument {
			t.Errorf("议论文的结构里混进了 %q（genre=%q）", m.ID, m.Genre)
		}
	}
	got := vocab.Structures("", "")
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

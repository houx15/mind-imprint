package api

import (
	"strings"
	"testing"
)

// 年级门槛（2026-09-25）。
//
// 🚨 这条轴 2026-09-22 就建好了 —— Key.Grade、classes.grade、
// writingPlanSystemFor 的第三个参数、writing_plan.go 从她的班里查年级再传进去，
// 一样不缺 —— 但**没有任何一行按年级登记过内容**，所以它一直跑空车：
// 印记照样会建议一个初一学生用虚拟语气。
//
// 源里的红线逐字：「七上以一般现在时为主；一般过去时从七下后期至八上系统
// 学习；现在完成时在八下引入；被动语态、宾语从句在九年级学习。」

func TestJuniorGetsTheGrammarCeiling(t *testing.T) {
	for _, grade := range []string{"junior1", "junior2", "junior3"} {
		s := writingPlanSystemFor(genreArgument, "en", grade)
		for _, want := range []string{"被动语态和宾语从句在九年级", "一般现在时"} {
			if !strings.Contains(s, want) {
				t.Errorf("%s：语法红线没装进去（找不到 %q）", grade, want)
			}
		}
		// 🚨 高考那三样不许推给初中生 —— 这是这一条存在的直接理由。
		if !strings.Contains(s, "不要建议给初中生") {
			t.Errorf("%s：没有拦住倒装/虚拟语气这些高中写法", grade)
		}
		// 升格建议要带年级，否则「给建议」和「给超纲建议」分不开。
		for _, want := range []string{"八年级起", "九年级"} {
			if !strings.Contains(s, want) {
				t.Errorf("%s：升格表没有年级列（找不到 %q）", grade, want)
			}
		}
	}
}

// ruling #2：结尾那句谚语按学段分。
//
// 初中那份 rubric 把它当升格建议，高考那份把同一件事标成背诵痕迹 ——
// 而高考那句原话是「与上下文**脱节**的背诵痕迹」，被判的是脱节。
// 所以差别只剩「要不要主动推荐它」，那正是学段的差别。
func TestProverbAdviceSplitsByBand(t *testing.T) {
	junior := writingPlanSystemFor(genreArgument, "en", "junior2")
	senior := writingPlanSystemFor(genreArgument, "en", "senior2")

	if !strings.Contains(junior, "可以建议她加一句谚语") {
		t.Error("初中那一侧没有把谚语当升格项")
	}
	if !strings.Contains(senior, "不要主动建议") {
		t.Error("高中那一侧还在主动推荐谚语")
	}
	// 🚨 高中那一侧判的是**脱节**，不是谚语本身 —— 写成「不许用谚语」
	// 就把两份 rubric 里更讲道理的那一份丢了。
	if !strings.Contains(senior, "脱节") {
		t.Error("高中那一侧没说清被判的是脱节")
	}
	if strings.Contains(senior, "不许用") {
		t.Error("高中那一侧把「脱节才标」写成了「不许用」")
	}
}

// 高中没有年级上的语法门槛，但「多样 ≠ 堆生僻词」仍然成立。
func TestSeniorHasNoGrammarCeilingButStillNoShowingOff(t *testing.T) {
	s := writingPlanSystemFor(genreArgument, "en", "senior1")
	if strings.Contains(s, "被动语态和宾语从句在九年级") {
		t.Error("高中生拿到了初中的语法红线")
	}
	if !strings.Contains(s, "堆生僻词") {
		t.Error("高中那一侧没有拦住堆砌")
	}
}

// 🚨 反方向，而且这是今天库里的多数情况：她的班没填年级。
//
// 那时这一节取不到，装出来的提示词要和这一节存在之前**逐字节相同** ——
// 否则每一篇老作文的缓存前缀都被打碎（块的顺序是成本契约）。
func TestNoGradeMeansNoCeilingSection(t *testing.T) {
	s := writingPlanSystemFor(genreArgument, "en", "")
	for _, gone := range []string{"她是初中生", "她是高中生", "@@CEILING@@"} {
		if strings.Contains(s, gone) {
			t.Errorf("没有年级时还是装上了 %q", gone)
		}
	}
}

// 门槛不按文体分 —— 她学到哪儿了和这一篇写什么无关。
func TestCeilingIsGenreIndependent(t *testing.T) {
	for _, genre := range []string{genreArgument, genreNarrative, genreLetter, genreProse, genreContinuation, genreSummary} {
		if s := writingPlanSystemFor(genre, "en", "junior2"); !strings.Contains(s, "她是初中生") {
			t.Errorf("体裁 %q 上没有年级门槛", genre)
		}
	}
}

package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 概要写作（2026-09-24 新加）。高考英语写作 16% 的题量、10 分。
//
// 🚨 这一档的教学内容是**补**出来的：两份 skill 只做应用文和读后续写，
// 源文件没有概要写作的骨架，而且缺口清单明写「不要从书信骨架外推」。
// 记在 docs/2026-09-24-teaching-rulings.md 里等教研组过目。

// 那串逐字固定的 Directions —— 语料里 16 道概要写作题全长这样。
const summaryDirections = "Directions: Read the following passage. Summarize the main idea " +
	"and the main point(s) of the passage in no more than 60 words. Use your own words as far as possible."

func TestSummaryIsRecognisedFromItsDirections(t *testing.T) {
	for _, title := range []string{
		summaryDirections,
		"概要写作：When Questions Turn Self-Centered",
		"Summary writing practice",
		"Summarize the main idea of the passage in no more than 60 words.",
	} {
		if got := writingGenreOf(sqlc.Writing{Title: title}, nil); got != genreSummary {
			t.Errorf("「%.50s」判成了 %q，要的是概要写作", title, got)
		}
	}
}

// 🚨 反方向：光有 summarize 或者光有词数上限都不算。
//
// 「summarize」在一道议论文题的题面里也会出现（"summarize your view in the
// last paragraph"），而词数上限每一道题都有。两样同时出现才是概要写作。
func TestSummaryDoesNotStealOtherGenres(t *testing.T) {
	for _, title := range []string{
		"Write no more than 120 words about your school life",
		"In your last paragraph, summarize your view on the problem",
		"读后续写：根据材料续写两段",
		"给外婆写一封信",
	} {
		if got := writingGenreOf(sqlc.Writing{Title: title}, nil); got == genreSummary {
			t.Errorf("「%.50s」被判成了概要写作", title)
		}
	}
}

// 🚨 概要有自己的两种块，不借书信的也不借议论文的。
//
// writingGenreOf 第 1 步是**看板上有什么块**来倒推文体 —— 借书信的
// purpose/matter，一篇概要摆上节点就会被判成信；借议论文的 thesis/point
// 就会被判成议论文。这条测试守的正是这个倒推。
func TestSummaryBoardIsNotMistakenForALetterOrAnArgument(t *testing.T) {
	rows := []sqlc.WritingOutline{
		{Text: "这篇文章讲提问方式的变化", Kind: writingKindGist, Depth: 0, Position: 0},
		{Text: "人们越来越多地只问和自己有关的问题", Kind: writingKindKey, Depth: 1, Position: 1},
	}
	// 题目认不出来（她自己敲的标题）时，板上的块必须把它带回概要写作。
	got := writingGenreOf(sqlc.Writing{Title: "我的作业"}, rows)
	if got != genreSummary {
		t.Errorf("摆着主旨和要点的板被判成了 %q", got)
	}
	// 两种块都要有自己的深度和文体归属，否则它们会被当成「不认识的块」。
	for _, k := range []string{writingKindGist, writingKindKey} {
		if !writingKindValid(k) {
			t.Errorf("%q 不在块的闭表里 —— 模型回它会被丢掉", k)
		}
		if writingKindGenre(k) != genreSummary {
			t.Errorf("%q 的文体归属是 %q", k, writingKindGenre(k))
		}
		if writingKindLabel(k, "") == "" {
			t.Errorf("%q 在图上没有名字", k)
		}
	}
	// 主旨在上，要点挂在它底下。
	if writingKindDepths[writingKindGist] != 0 || writingKindDepths[writingKindKey] != 1 {
		t.Error("主旨和要点的层次不对")
	}
}

// 概要那一档的提示词要是概要的。
func TestSummarySystemPromptIsItsOwn(t *testing.T) {
	s := writingPlanSystemFor(genreSummary, "en", "")
	for _, want := range []string{"主旨", "要点", "自己的话", "60 词"} {
		if !strings.Contains(s, want) {
			t.Errorf("概要的提示词里没有 %q", want)
		}
	}
	// 这一档最要紧的两条反面规矩。
	if !strings.Contains(s, "照抄") {
		t.Error("没有拦住照抄原文")
	}
	if !strings.Contains(s, "In my opinion") && !strings.Contains(s, "个人看法") &&
		!strings.Contains(s, "不加她自己") {
		t.Error("没有拦住学生往概要里加自己的看法")
	}
	if strings.Contains(s, "「thesis」") || strings.Contains(s, "「point」") {
		t.Error("概要的提示词里把中心论点/分论点列成了可选的节点类型")
	}
	if strings.Contains(s, "总—分—总") {
		t.Error("概要的提示词里混进了议论文的篇章结构")
	}
}

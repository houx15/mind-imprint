package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/guidance"
	"mindimprint/api/internal/prompts"
)

// 🚨 这条测试 2026-09-25 **按计划报废了**，换成了它的反面。
//
// 原来它叫 TestGradeDoesNotChangeAnythingYet，断言「填不填年级结果一模一样」，
// 并且在注释里写清楚了报废的条件：
//
//	第一个人登记 Grades 行的时候，这条测试会红 —— 那是计划内的报废，
//	不是回归。同时要做的是把 coverage_test.go 的 want 表从 lang/genre
//	两轴改成三轴，否则那一行内容会输给已在的文体行（26 分对 28 分），
//	一次都不出现而测试全绿。
//
// 那一天到了（年级门槛，见 prompts.WritingCeilingJunior）。两件事分别处理：
//
//  1. **断言换方向**：现在按年级分叉的是新的 SlotCeiling，所以初中拿到红线、
//     高中拿到另一份、不填年级的仍然逐字节不变（下面三条）。
//  2. **coverage_test.go 的 want 不用改成三轴**，因为那张表管的是
//     kinds / material / skeleton 三个槽，而年级内容住在**自己的槽**里 ——
//     它不和任何文体行争同一个槽，26 分对 28 分那道坎不存在。
//     真到了有人按年级登记 kinds 或 skeleton 的那天，那条警告仍然成立。
//
// 🚨 benchcases_lite_writing.go 的 writingPlanCase() 传的 grade 还是 ""。
// 那仍然是生产会发的一份（她的班没填年级时就是这一支，今天是多数），
// 所以 cmd/routebench 量的东西没有失真。真要按年级挑模型，那是另一件事。
func TestGradeNowOnlyChangesTheCeiling(t *testing.T) {
	for _, lang := range []string{"zh", langEnglish} {
		for _, genre := range []string{genreArgument, genreNarrative} {
			base := writingPlanSystemFor(genre, lang, "")
			for _, grade := range classGrades {
				got := writingPlanSystemFor(genre, lang, grade)
				if got == base {
					t.Errorf("%s/%s/%s：填了年级之后提示词没变 —— 年级门槛没装上",
						lang, genre, grade)
					continue
				}
				// 变的**只有**门槛那一节：把那一整段原样减掉之后，
				// 必须和不填年级时逐字节相同。否则年级悄悄改到了别的教学内容，
				// 而那是没人审过的。
				ceiling := prompts.WritingCeilingJunior
				if guidance.GradeBand(grade) == "senior" {
					ceiling = prompts.WritingCeilingSenior
				}
				if without := strings.Replace(got, ceiling+"\n\n", "", 1); without != base {
					t.Errorf("%s/%s/%s：除了门槛那一节，年级还改到了别的东西", lang, genre, grade)
				}
			}
		}
	}
}

// 提示词里不许出现内部的年级取值 —— 那是给代码看的标记，她的屏幕上没有。
func TestGradeCodeNeverLeaksIntoThePrompt(t *testing.T) {
	for _, grade := range classGrades {
		s := writingPlanSystemFor(genreArgument, "zh", grade)
		if strings.Contains(s, grade) {
			t.Errorf("提示词里出现了内部取值 %q", grade)
		}
	}
}

package api

import "strings"
import "testing"

// 🚨 到二期 b 为止，注册表里**一行按年级分的内容都没有**，所以填不填年级、
// 填哪个年级，结果必须一模一样。这条测试是二期 a 的验收，二期 b 复核过一次
// 仍然成立（见二期 b 计划里那条裁定：学段今天只用来记录，不用来分叉教学内容）。
//
// 第一个人登记 Grades 行的时候，这条测试会红 —— 那是计划内的报废，不是回归。
// 同时要做的是把 coverage_test.go 的 want 表从 lang/genre 两轴改成三轴，
// 否则那一行内容会输给已在的文体行（26 分对 28 分），一次都不出现而测试全绿。
//
// 🚨 同一天还要看一眼 benchcases_lite_writing.go：writingPlanCase() 传的 grade
// 是 ""（fixture 没有班级）。年级一旦真的改变装配结果，那条 bench 用例就又
// 变成「量的不是生产会发的那份」了，而 cmd/routebench 拿它挑模型。
func TestGradeDoesNotChangeAnythingYet(t *testing.T) {
	for _, lang := range []string{"zh", langEnglish} {
		for _, genre := range []string{genreArgument, genreNarrative} {
			base := writingPlanSystemFor(genre, lang, "")
			for _, grade := range classGrades {
				if got := writingPlanSystemFor(genre, lang, grade); got != base {
					t.Errorf("%s/%s：填了年级 %s 之后提示词变了，但这一期还没有任何年级内容",
						lang, genre, grade)
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

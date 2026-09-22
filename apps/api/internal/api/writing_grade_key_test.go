package api

import "strings"
import "testing"

// 🚨 这一期不加任何一行年级专属的内容，所以**填不填年级，结果必须一模一样**。
// 这条测试就是这期的验收：轴通了，而她看到的东西没变。
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

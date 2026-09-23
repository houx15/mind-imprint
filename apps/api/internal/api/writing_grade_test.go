package api

import "testing"

func TestGradeFromClassesTakesTheOnlyOne(t *testing.T) {
	if got, _ := gradeFromClasses([]string{"junior2"}); got != "junior2" {
		t.Errorf("只有一个班时该直接用它，拿到 %q", got)
	}
}

// 没进班、或者班上没填年级 —— 都是「不知道」，不是错误。
func TestGradeFromClassesUnknownWhenNothingToGoOn(t *testing.T) {
	for _, in := range [][]string{nil, {}, {""}, {"", ""}} {
		if got, _ := gradeFromClasses(in); got != "" {
			t.Errorf("%v 该是不知道，拿到 %q", in, got)
		}
	}
}

// 🚨 两个班两个年级 —— **不猜**。
//
// 猜错的代价是她整篇拿到另一个年级的教学内容，而屏幕上什么异常都没有。
// 退回「不知道」只是少一条线索：她拿到的是今天那一份通用内容。
func TestGradeFromClassesRefusesToGuessWhenClassesDisagree(t *testing.T) {
	if got, _ := gradeFromClasses([]string{"junior2", "senior1"}); got != "" {
		t.Errorf("两个年级对不上时该退回不知道，拿到 %q", got)
	}
}

// 两个班但年级一样（或者其中一个没填）—— 那就没有歧义，用那个年级。
func TestGradeFromClassesAgreesIsFine(t *testing.T) {
	if got, _ := gradeFromClasses([]string{"junior2", "junior2"}); got != "junior2" {
		t.Errorf("两个班同一个年级时该用它，拿到 %q", got)
	}
	if got, _ := gradeFromClasses([]string{"", "senior1"}); got != "senior1" {
		t.Errorf("一个没填一个填了，该用填了的那个，拿到 %q", got)
	}
}

func TestGradeFromClassesReportsTheDisagreement(t *testing.T) {
	if _, conflict := gradeFromClasses([]string{"junior2", "senior1"}); !conflict {
		t.Error("两个班年级对不上，应该报出来")
	}
	if _, conflict := gradeFromClasses([]string{"junior2", "junior2", ""}); conflict {
		t.Error("年级一致（外加一个没填的班），不该报对不上")
	}
	if _, conflict := gradeFromClasses(nil); conflict {
		t.Error("一个班都没有，不该报对不上")
	}
}

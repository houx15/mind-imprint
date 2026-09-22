package api

import (
	"testing"

	"mindimprint/api/internal/guidance"
)

func TestValidateClassGradeAcceptsTheClosedSetAndEmpty(t *testing.T) {
	for _, ok := range []string{"", "junior1", "junior2", "junior3", "senior1", "senior2", "senior3"} {
		if got, valid := validateClassGrade(ok); !valid || got != ok {
			t.Errorf("%q 该被接受，拿到 got=%q valid=%v", ok, got, valid)
		}
	}
}

// 🚨 空串是「没填」，是合法的；别的不认识的字一律拒掉，不要悄悄存进去。
// 存进去的那一刻它就会被当成年级去挑教学内容，而没有任何一行登记服务它。
func TestValidateClassGradeRejectsAnythingElse(t *testing.T) {
	for _, bad := range []string{"junior", "senior", "JUNIOR1", "初二", "primary4", "junior0", "junior4", " junior1"} {
		if _, valid := validateClassGrade(bad); valid {
			t.Errorf("%q 不该被接受", bad)
		}
	}
}

// 🚨 年级要能折成学段，「整个初中通用」的内容才只写一份。
// 这一条同时钉住：闭表里的每一个年级，guidance 那边都折得出学段来。
func TestEveryClassGradeFoldsToABand(t *testing.T) {
	for _, g := range classGrades {
		if band := guidance.GradeBand(g); band != "junior" && band != "senior" {
			t.Errorf("年级 %q 折出来的学段是 %q，既不是 junior 也不是 senior", g, band)
		}
	}
}

func TestClassGradeLabelIsChineseAndEmptyStaysEmpty(t *testing.T) {
	if got := classGradeLabel(""); got != "" {
		t.Errorf("没填年级时标签该是空串，拿到 %q", got)
	}
	if got := classGradeLabel("junior2"); got != "初二" {
		t.Errorf("junior2 的标签该是「初二」，拿到 %q", got)
	}
	if got := classGradeLabel("senior3"); got != "高三" {
		t.Errorf("senior3 的标签该是「高三」，拿到 %q", got)
	}
}

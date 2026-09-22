package api

import "strings"

// class_grade.go —— 一个班是几年级。
//
// # 为什么闭表在这儿，不在数据库上
//
// 和 writing.origin、writing.stage 同一条纪律（见 0186 迁移里的说明）：
// 执行点在 Go，数据库那边留着余地。将来要加「初四」或者小学，改这一张表
// 就够了，不用做一次锁表的迁移。
//
// # 🚨 空串是「没填」，而且它必须一路是合法的
//
// 老班全是空串，批量导入建的班也是空串。空串到了 guidance 那边匹配不上任何
// 一行带年级的登记，于是退回不限年级的那一行 —— 也就是今天的行为。
// **所以「不知道年级」永远不是错误，只是少一条线索。**
var classGrades = []string{"junior1", "junior2", "junior3", "senior1", "senior2", "senior3"}

// classGradeLabels 是她和老师在屏幕上看见的说法。
//
// 名词，不是句子（AGENTS.md 界面文案第 1 条）。
var classGradeLabels = map[string]string{
	"junior1": "初一", "junior2": "初二", "junior3": "初三",
	"senior1": "高一", "senior2": "高二", "senior3": "高三",
}

// validateClassGrade 收下一个年级。第二个返回值是「这个值能不能存」。
//
// 🚨 不认识的值一律拒掉，不做大小写或中文的宽容匹配：存进去的那一刻它就会
// 被当成年级去挑教学内容，而没有任何一行登记服务它 —— 症状是她悄悄少拿到
// 一块内容，而日志上什么都看不见。
func validateClassGrade(s string) (string, bool) {
	if s == "" {
		return "", true
	}
	for _, g := range classGrades {
		if s == g {
			return g, true
		}
	}
	return "", false
}

// classGradeLabel 是界面上那个词。没填就还它一个空串。
func classGradeLabel(grade string) string {
	return classGradeLabels[strings.TrimSpace(grade)]
}

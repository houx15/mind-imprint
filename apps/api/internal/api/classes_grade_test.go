package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// DTO 要把年级带出去，否则老师那边建完就再也看不到自己填了什么。
func TestClassDTOCarriesGrade(t *testing.T) {
	dto := toClassDTO(sqlcClassWithGrade("junior2"))
	if dto.Grade != "junior2" {
		t.Errorf("classDTO.Grade = %q，想要 junior2", dto.Grade)
	}
	if dto.GradeLabel != "初二" {
		t.Errorf("classDTO.GradeLabel = %q，想要「初二」", dto.GradeLabel)
	}
}

// 🚨 没填年级的班（老班、批量导入建的）要原样出去，不要在这儿编一个默认值。
// 编一个出来，她就会按一个没人填过的年级拿教学内容。
func TestClassDTOKeepsUnknownGradeEmpty(t *testing.T) {
	dto := toClassDTO(sqlcClassWithGrade(""))
	if dto.Grade != "" || dto.GradeLabel != "" {
		t.Errorf("没填年级时该是两个空串，拿到 %q / %q", dto.Grade, dto.GradeLabel)
	}
}

func sqlcClassWithGrade(grade string) sqlc.Class {
	return sqlc.Class{Name: "试用班级", JoinCode: "AAAA-BBBB", Grade: grade}
}

// 🚨 闭表里每一个年级都必须有中文说法。
//
// 少一个不会报错 —— map 取不到就是空串，老师的屏幕上那一格直接是空的，
// 而测试全绿。Task 3 的评审指出这条缺口时，原来的测试只抽查了六个里的两个。
func TestEveryClassGradeHasALabel(t *testing.T) {
	for _, g := range classGrades {
		if classGradeLabel(g) == "" {
			t.Errorf("年级 %q 没有中文说法", g)
		}
	}
}

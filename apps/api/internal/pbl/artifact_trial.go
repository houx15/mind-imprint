package pbl

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ArtifactTrial is unsubmitted student input until explicitly converted to a keep entry.
type ArtifactTrial struct {
	Mode     string `json:"mode"`
	Version  string `json:"version"`
	Task     string `json:"task"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Next     string `json:"next"`
}

func EmptyArtifactTrial() ArtifactTrial { return ArtifactTrial{Mode: "self"} }

func (t ArtifactTrial) Validate(complete bool) error {
	if t.Mode != "self" && t.Mode != "other" && t.Mode != "not_tested" {
		return fmt.Errorf("请选择有效的试用方式")
	}
	for _, field := range []string{t.Version, t.Task, t.Expected, t.Actual, t.Next} {
		if utf8.RuneCountInString(field) > 1500 {
			return fmt.Errorf("每项内容不得超过1500字")
		}
	}
	if complete {
		if strings.TrimSpace(t.Version) == "" || strings.TrimSpace(t.Task) == "" || strings.TrimSpace(t.Expected) == "" {
			return fmt.Errorf("请填写版本、操作任务和预期结果")
		}
		if t.Mode != "not_tested" && strings.TrimSpace(t.Actual) == "" {
			return fmt.Errorf("请填写实际结果；尚未操作时请选择未测试")
		}
	}
	return nil
}

func (t ArtifactTrial) Body(id, title string) string {
	mode := map[string]string{"self": "学生自己实际操作（不代表其他用户试用）", "other": "学生记录的他人试用（尚未经系统独立验证）", "not_tested": "尚未测试，以下仅是试用计划"}[t.Mode]
	actual := strings.TrimSpace(t.Actual)
	if t.Mode == "not_tested" {
		actual = "未测试，没有实际结果"
	}
	next := strings.TrimSpace(t.Next)
	if next == "" {
		next = "尚未决定"
	}
	return strings.Join([]string{
		fmt.Sprintf("关联制作材料：%s（%s）", title, id), "试用方式：" + mode,
		"版本或预览地址：" + strings.TrimSpace(t.Version), "操作任务：" + strings.TrimSpace(t.Task),
		"预期结果：" + strings.TrimSpace(t.Expected), "实际结果：" + actual,
		"保留或修改的想法与理由：" + next,
		"记录预览地址不代表页面已被系统访问、验证或公开发布。",
	}, "\n\n")
}

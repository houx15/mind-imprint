package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"strings"
)

func writingPlanReadyTooSoonNudge(missing string) string {
	return "你刚才请学生去写了，但系统数量检查显示仍需补充：" + missing +
		"\n请重新输出完整的 JSON：ready 给 false，reply 回应学生刚说的内容，然后针对尚需补充的内容提问（一次只问一个）。add 照旧。"
}
func writingPlanPlaceNudge(unplaced []string) string {
	return "你在 reply 里提到了学生这一轮说的「" + strings.Join(unplaced, "」「") +
		"」，但图上没有它，add 里也没有。\n" +
		"请重新输出完整的 JSON：将学生本轮已表达的新内容加入 add，按其作用选择有效 kind；不输出 parentId 或 role；" +
		"节点位置由系统根据 kind 安排。reply 可以不变。"
}

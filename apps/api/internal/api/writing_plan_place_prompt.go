package api

// Prompt assembly for writing_plan_place.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"strings"
)

// writingPlanReadyTooSoonNudge：模型说「可以写了」，而服务端数出来还差。
func writingPlanReadyTooSoonNudge(missing string) string {
	return "你刚才请她去写了，但这份计划还没过线：" + missing +
		"\n请重新输出完整的 JSON：ready 给 false，reply 里接住她刚说的话，然后按缺口问下一个问题（一次只问一个）。add 照旧。"
}

// writingPlanPlaceNudge 是重试那一轮补的话。只说犯的那一处。
func writingPlanPlaceNudge(unplaced []string) string {
	return "你在 reply 里提到了她这一轮说的「" + strings.Join(unplaced, "」「") +
		"」，但图上没有它，add 里也没有（或者 parentId 不是【当前的图】里的 id，那一条会被丢掉）。\n" +
		"请重新输出完整的 JSON：把它加进 add；parentId 逐字复制【当前的图】里那串 id；" +
		"分论点挂在中心论点下面，例子放在它所说明的那条分论点下面。reply 可以不变。"
}

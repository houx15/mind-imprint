package api

import (
	"regexp"
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// writing_plan_place.go —— 印记 说「我把它放进图里了」，图上就得真的有它。
//
// 🚨 2026-09-18 产品负责人带截图报的：她又说了一条分论点，印记 回
// 「底下三条分论点方向各不相同……现在可以开始写了」，而图上只有**一条**。
// 两处可能丢：
//
//  1. 模型给的 parentId 不是【当前的图】里那串 id（少几位、带了「id=」、
//     或者干脆写成父节点的文字）→ 落库那一路按「不认识的 parentId」整条丢掉；
//  2. 模型在 reply 里复述了她的话，add 却是空的。
//
// 第一种在 resolvePlanParent 里认回来；第二种能验：reply 里用「」引的、
// 出自她这一轮原话的那句，要么在图上，要么在这一轮的 add 里。
// 都不在就重试一次（planReplyUnplaced + writingPlanPlaceNudge）。

// 🚨 2026-09-20：这里原来住着五个函数 —— planHandle、resolvePlanParent、
// examplePlanParent、writingRoleIsPoint、thesisPlanNode —— 全部删掉了。
//
// 它们做的是同一件事：**把模型给的位置修回来**。模型抄丢 UUID 的一段、
// 同一轮新建的分论点没有 id 可引用、把结尾挂到中心论点底下……每一个函数都是
// 一次线上事故的补丁，而补丁只能在错误发生之后纠正它。
//
// 现在模型不给位置了，只说这一块**是什么**（kind），位置由 writing_kind.go
// 算出来。要修的东西没有了，修它们的代码也就没有了。

var planReplyQuote = regexp.MustCompile(`「([^「」]{2,60})」`)

// planReplyUnplaced 列出 reply 里引着她**这一轮原话**、却不在图上也不在
// 这一轮 add 里的那几句。
//
// 只认出自她这一轮原话的引文：印记 引方法名（「并列论证」）、引图上早就有的
// 节点、引它自己的话，都不算。短于 4 个字的不算（「好」「对」）。
// 🚨 2026-09-20 起不再有「parentId 认不出来所以落库时被丢掉」这一类：
// 每一条通过解析的 add 都一定落得上去（位置由 kind 算，不会认不出来）。
func planReplyUnplaced(
	reply, studentText string,
	rows []sqlc.WritingOutline,
	add []writingPlanAdd,
) []string {
	said := normalizeOutlineText(studentText)
	if said == "" {
		return nil
	}
	var placed []string
	for _, r := range rows {
		if t := normalizeOutlineText(r.Text); t != "" {
			placed = append(placed, t)
		}
	}
	for _, n := range add {
		if t := normalizeOutlineText(n.Text); t != "" {
			placed = append(placed, t)
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range planReplyQuote.FindAllStringSubmatch(reply, -1) {
		q := normalizeOutlineText(m[1])
		if len([]rune(q)) < 4 || seen[q] || !strings.Contains(said, q) {
			continue
		}
		seen[q] = true
		onMap := false
		for _, p := range placed {
			// 节点是她那句话的精简：互相包含就算放上去了。
			if strings.Contains(p, q) || strings.Contains(q, p) {
				onMap = true
				break
			}
		}
		if !onMap {
			out = append(out, m[1])
		}
	}
	return out
}

// planTurnDroppedHerPoint：她这一轮说了一句像样的话，图上没有它，这一轮也什么
// 都没加 —— 第二种丢法（reply 里只是转述，没用「」引）。
//
// 2026-09-18 本地实测复现了截图那一幕：她说「还有 脆弱的感受往往也带来很多关于
// 自己渴望的信息」，印记 回「第二条……还带着关于自己渴望的信息」，add 是空的。
// 只看引文的 planReplyUnplaced 抓不到它。
//
// 不算的：短句和「等于没答」的那几类（lowSubstanceReply）、问句、她说要去写了、
// 图上已经有这句（互相包含）。短于 8 个字的也不算 —— 「好的，就用这个」不是新的点。
func planTurnDroppedHerPoint(studentText string, rows []sqlc.WritingOutline, placedThisTurn int) bool {
	if placedThisTurn > 0 {
		return false
	}
	t := strings.TrimSpace(studentText)
	if len([]rune(t)) < 8 || lowSubstanceReply(t) || strings.ContainsAny(t, "？?") || studentWantsToWrite(t) {
		return false
	}
	said := normalizeOutlineText(t)
	for _, r := range rows {
		n := normalizeOutlineText(r.Text)
		if n != "" && (strings.Contains(n, said) || strings.Contains(said, n)) {
			return false
		}
	}
	return true
}

// planAddsThatLand 数这一轮的 add 里真能落到图上的几条（文字不空、不和图上
// 重复）。和落库那一路丢的是同一批。
//
// kind 不合法的那一条在 parseWritingPlanReply 就已经被丢掉了，到这里不会出现。
func planAddsThatLand(rows []sqlc.WritingOutline, add []writingPlanAdd) int {
	n := 0
	for _, a := range add {
		if strings.TrimSpace(a.Text) == "" || outlineHasText(rows, a.Text) {
			continue
		}
		n++
	}
	return n
}

// studentWantsToWrite：她这一句是在说「我要去写了」。只有这时，模型自己的
// ready 才能越过服务端数出来的那条线（见 postWritingPlanTurn 的 ready）。
func studentWantsToWrite(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	for _, kw := range []string{
		"开始写", "去写", "动笔", "可以写了", "想写了", "写吧", "先这样", "就这些", "就这样吧", "不想再想",
		"start writing", "let me write", "ready to write", "that's enough", "thats enough",
	} {
		if strings.Contains(t, kw) {
			return true
		}
	}
	return false
}

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

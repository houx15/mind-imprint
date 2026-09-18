package api

import (
	"regexp"
	"strconv"
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

// planHandle 是 prompt 里给模型看的节点短号：n1、n2……（按这一轮开始时图上的顺序）。
//
// 🚨 2026-09-18 实测：模型抄 36 位的 UUID 会抄丢一整段
// （`c968eb1b-0b62-4214-e2555921adf8`，少了第四段），那个节点于是按「不认识的
// parentId」被丢掉，重试一次还是同样抄错。短号抄不错。
func planHandle(i int) string { return "n" + strconv.Itoa(i+1) }

// resolvePlanParent 把模型给的 parentId 对到一个真实的节点上。
//
// 从严到宽，每一步都只接受**唯一**的命中：逐字 id → 去掉「id=」和空白 →
// 至少 8 位的 id 前缀 → 节点文字完全相同。都不中就不认 —— 猜一个父节点会把
// 她的话放到她没放过的地方，比丢掉更糟（原来那条规矩不变）。
//
// 先查 rows（落库循环里的 live，位置是最新的），再查 byID：
// byID 里的行是这一轮开始时读的，同一轮前面插过节点之后它们的 Position 已经旧了，
// 拿旧位置去算插入点会把节点插错地方。
func resolvePlanParent(byID map[string]sqlc.WritingOutline, rows []sqlc.WritingOutline, raw string) (sqlc.WritingOutline, bool) {
	find := func(id string) (sqlc.WritingOutline, bool) {
		for _, r := range rows {
			if r.ID.String() == id {
				return r, true
			}
		}
		r, ok := byID[id]
		if !ok {
			return r, false
		}
		// byID 里的行是这一轮开始时的，位置可能已经旧了：换成 rows 里那一行。
		for _, x := range rows {
			if x.ID == r.ID {
				return x, true
			}
		}
		return r, true
	}
	if r, ok := find(raw); ok {
		return r, true
	}
	id := strings.ToLower(strings.TrimSpace(raw))
	id = strings.TrimPrefix(id, "id=")
	id = strings.TrimPrefix(id, "id:")
	id = strings.TrimSpace(id)
	if r, ok := find(id); ok {
		return r, true
	}
	all := append([]sqlc.WritingOutline(nil), rows...)
	for _, r := range byID {
		dup := false
		for _, x := range all {
			if x.ID == r.ID {
				dup = true
				break
			}
		}
		if !dup {
			all = append(all, r)
		}
	}
	unique := func(match func(sqlc.WritingOutline) bool) (sqlc.WritingOutline, bool) {
		var hit sqlc.WritingOutline
		n := 0
		for _, r := range all {
			if match(r) {
				hit = r
				n++
			}
		}
		return hit, n == 1
	}
	if len(id) >= 8 {
		if r, ok := unique(func(r sqlc.WritingOutline) bool { return strings.HasPrefix(r.ID.String(), id) }); ok {
			return r, true
		}
	}
	// 抄丢了中间一段的 UUID：头尾两段对得上、每一段都在。
	if segs := strings.Split(id, "-"); len(segs) >= 3 && len(segs[0]) >= 8 {
		if r, ok := unique(func(r sqlc.WritingOutline) bool {
			full := r.ID.String()
			if !strings.HasPrefix(full, segs[0]) || !strings.HasSuffix(full, segs[len(segs)-1]) {
				return false
			}
			for _, sg := range segs {
				if !strings.Contains(full, sg) {
					return false
				}
			}
			return true
		}); ok {
			return r, true
		}
	}
	if want := normalizeOutlineText(raw); want != "" {
		if r, ok := unique(func(r sqlc.WritingOutline) bool { return normalizeOutlineText(r.Text) == want }); ok {
			return r, true
		}
	}
	return sqlc.WritingOutline{}, false
}

// examplePlanParent 是一个没有可用父节点的例子该挂到哪里：这一轮刚加的那条
// 分论点；这一轮没加，就是图上（按位置）最后一条分论点；一条都没有就 nil
// （那就只能留在最上层，段落那一步会把它排成一张「先说清它证明了什么」的卡）。
// 返回的是 live 里的那一行（位置是最新的）。
func examplePlanParent(pointThisTurn *sqlc.WritingOutline, live []sqlc.WritingOutline) *sqlc.WritingOutline {
	if pointThisTurn != nil {
		for i := range live {
			if live[i].ID == pointThisTurn.ID {
				return &live[i]
			}
		}
	}
	var last *sqlc.WritingOutline
	for i := range live {
		r := live[i]
		if r.Depth == 1 && !writingRoleIsExample(r.Role, r.Source) && strings.TrimSpace(r.Text) != "" {
			if last == nil || r.Position > last.Position {
				last = &live[i]
			}
		}
	}
	return last
}

// writingRoleIsPoint：role 说这一块是一条分论点 / 理由（不是中心论点本身）。
func writingRoleIsPoint(role string) bool {
	r := strings.ToLower(role)
	if strings.Contains(r, "中心") {
		return false
	}
	for _, kw := range []string{"分论点", "理由", "论点", "sub-point", "subpoint", "reason", "point"} {
		if strings.Contains(r, kw) {
			return true
		}
	}
	return false
}

// thesisPlanNode 是图上的中心论点：第一个最上层、不是开头/结尾/例子/分论点的节点。
// 返回 live 里的那一行；没有就 nil（分论点只能留在最上层）。
func thesisPlanNode(live []sqlc.WritingOutline) *sqlc.WritingOutline {
	var best *sqlc.WritingOutline
	for i := range live {
		r := live[i]
		if r.Depth != 0 || strings.TrimSpace(r.Text) == "" ||
			roleHasAny(r.Role, writingOpeningRoleWords) || roleHasAny(r.Role, writingClosingRoleWords) ||
			writingRoleIsExample(r.Role, r.Source) || writingRoleIsPoint(r.Role) {
			continue
		}
		if best == nil || r.Position < best.Position {
			best = &live[i]
		}
	}
	return best
}

var planReplyQuote = regexp.MustCompile(`「([^「」]{2,60})」`)

// planReplyUnplaced 列出 reply 里引着她**这一轮原话**、却不在图上也不在
// 这一轮 add 里的那几句。
//
// 只认出自她这一轮原话的引文：印记 引方法名（「并列论证」）、引图上早就有的
// 节点、引它自己的话，都不算。短于 4 个字的不算（「好」「对」）。
// add 里 parentId 认不出来的那一条也算「没放上去」—— 它落库时会被丢掉。
func planReplyUnplaced(
	reply, studentText string,
	rows []sqlc.WritingOutline,
	byID map[string]sqlc.WritingOutline,
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
		if n.ParentID != "" {
			if _, ok := resolvePlanParent(byID, rows, n.ParentID); !ok {
				continue
			}
		}
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

// planAddsThatLand 数这一轮的 add 里真能落到图上的几条（parentId 认得出来、
// 文字不空、不和图上重复）。和落库那一路丢的是同一批。
func planAddsThatLand(rows []sqlc.WritingOutline, byID map[string]sqlc.WritingOutline, add []writingPlanAdd) int {
	n := 0
	for _, a := range add {
		if strings.TrimSpace(a.Text) == "" || outlineHasText(rows, a.Text) {
			continue
		}
		if a.ParentID != "" && !writingRoleIsExample(a.Role, a.Source) {
			if _, ok := resolvePlanParent(byID, rows, a.ParentID); !ok {
				continue
			}
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
		"分论点挂在中心论点下面，例子挂在它支撑的那条分论点下面。reply 可以不变。"
}

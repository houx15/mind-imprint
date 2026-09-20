package api

// writing_kind.go —— 图上一个节点「是什么」的闭表，以及由它决定的一切。
//
// # 为什么这一列存在（2026-09-20）
//
// 在这之前，模型自己挑 `parentId`、自己用散文写 `role`，下游再拿关键词把意思
// 猜回来。同事试用时截到的那张图里，`结尾` 挂在 `中心论点` 底下（深度 1），
// 而 slots.ts 只在 `depth === 0` 时认结尾，于是它落进最后那个 else，
// 印成 **「分论点 3」**。产品负责人转来的原话：「这个是总结，不是分论点」。
//
// 标题错是摆放错的下游。所以修的是摆放：**深度和父节点由 kind 算出来，
// 不采信模型给的位置**。于是那个摆法不再可表示，而不是摆错之后被纠正。
//
// 骨架就是同事那张参考图：中心论点 → 分论点 1..n → 论据 1..2，
// 外加开篇与结尾各自成块。
//
// 🚨 这是**单一真相源**。apps/lite-web/src/writings/outlineKind.ts 是它的 TS
// 孪生，两边的取值、深度、标题、兜底顺序必须逐条一致。下游
//（writing_blocks.go、writing_guide.go、writing_plan.go）都从这里取，
// 不要在别处再写一张关键词表 —— 删掉的那四张正是这个 bug 的来源。

import (
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// 闭表。和 outlineKind.ts 的 OutlineKind 一致。
const (
	writingKindOpening  = "opening"  // 开篇
	writingKindThesis   = "thesis"   // 中心论点
	writingKindPoint    = "point"    // 分论点
	writingKindEvidence = "evidence" // 论据
	writingKindCounter  = "counter"  // 反方观点
	writingKindRebuttal = "rebuttal" // 对反方的回应
	writingKindGap      = "gap"      // 待补的材料
	writingKindClosing  = "closing"  // 结尾
)

var writingKindDepths = map[string]int32{
	writingKindOpening: 0, writingKindThesis: 0, writingKindClosing: 0,
	writingKindPoint: 1, writingKindCounter: 1,
	writingKindEvidence: 2, writingKindRebuttal: 2, writingKindGap: 2,
}

func writingKindValid(k string) bool {
	_, ok := writingKindDepths[k]
	return ok
}

// writingKindDepth 是这种块在图上的深度。**强制**，不采信模型给的位置。
//
// 不认识的 kind 当分论点处理（深度 1）—— 中间那一层是议论文里最常见的块，
// 也是错了代价最小的一层。真正不认识的 kind 在解析那一步就已经被丢掉了
//（parseWritingPlanReply），这里只是不让一个空字符串把节点送到深度 0。
func writingKindDepth(k string) int32 {
	if d, ok := writingKindDepths[k]; ok {
		return d
	}
	return 1
}

// writingKindLabel 是印在她屏幕上的那个小标题。
//
// 用的是语文课上的正式词（AGENTS.md 文案规则 6：学生来这儿就是要学这套词），
// 而且是名词（规则 1）。同事的意见 2：「部分论据的标题不规范」。
//
// 论据分两种后缀，靠 source 定：非空 = 她找回来的材料，空 = 她自己见过的事。
// 这条区分承重 —— 印记 下一轮正是照着 source 去查这份材料。
func writingKindLabel(k, source string) string {
	switch k {
	case writingKindOpening:
		return "开篇"
	case writingKindThesis:
		return "中心论点"
	case writingKindPoint:
		return "分论点"
	case writingKindCounter:
		return "反方观点"
	case writingKindRebuttal:
		return "对反方的回应"
	case writingKindGap:
		return "待补的材料"
	case writingKindClosing:
		return "结尾"
	case writingKindEvidence:
		if strings.TrimSpace(source) != "" {
			return "论据 · 你找来的材料"
		}
		return "论据 · 你见过的事"
	}
	return ""
}

// writingKindParentOf 选父节点：**按 kind，不按模型给的 parentId**。
//
// 深度 0 的三种（开篇 / 中心论点 / 结尾）没有父。
// 分论点和反方观点挂在中心论点上。
// 论据和待补挂在前面最近的分论点上；对反方的回应挂在前面最近的反方观点上。
//
// 🚨 找不到该挂的那一种就返回 nil，**不退而求其次挂到中心论点上**。
// 一条没有分论点可挂的论据挂到中心论点下面就变成了深度 1，下游会把它当成
// 一条理由 —— 这正是 2026-09-18 记下的那个毛病（「例子落在最上层，被印成
// 分论点 3」）的另一面。挂不上的由调用方处理，见 insertPlanNode。
func writingKindParentOf(k string, rows []sqlc.WritingOutline) *sqlc.WritingOutline {
	switch k {
	case writingKindOpening, writingKindThesis, writingKindClosing:
		return nil
	case writingKindPoint, writingKindCounter:
		return lastWritingKind(rows, writingKindThesis)
	case writingKindEvidence, writingKindGap:
		return lastWritingKind(rows, writingKindPoint)
	case writingKindRebuttal:
		return lastWritingKind(rows, writingKindCounter)
	}
	return nil
}

// lastWritingKind 按 position 找最后一个该种块。
//
// 取「最后一个」而不是「第一个」：她一条一条往下说，新的论据属于她刚说过的
// 那条分论点，不属于第一条。
func lastWritingKind(rows []sqlc.WritingOutline, kind string) *sqlc.WritingOutline {
	var found *sqlc.WritingOutline
	for i := range rows {
		if writingKindOf(rows[i]) != kind {
			continue
		}
		if found == nil || rows[i].Position > found.Position {
			found = &rows[i]
		}
	}
	return found
}

// writingKindOf 读一行的 kind，老行（0182 之前，kind 是空串）现算一个。
//
// 每一处读 Kind 的地方都要走这里，否则老稿子会拿到空字符串去 switch，
// 一路掉进 default。
func writingKindOf(row sqlc.WritingOutline) string {
	if row.Kind != "" {
		return row.Kind
	}
	return writingKindFromRole(row.Role, row.Depth)
}

// writingKindIsMaterial：这个节点算不算「撑得住一条理由的材料」。
//
// 🚨 **gap 不算。** 截图里那个「暂时还没找到的材料」是深度 2，而
// writingPlanShapeOf 只按深度数材料，于是它被当成一条真材料计入 Material，
// 可以把 ready() 推过线 —— 她手上一条材料都没有，产品却说这份计划站得住。
func writingKindIsMaterial(k string) bool {
	return k == writingKindEvidence
}

// writingKindAppliesTo 把 kind 映射成 vocab 的三个位置桶。
// 取代 writingGuideAppliesTo 里那张对自由散文做子串匹配的关键词表。
func writingKindAppliesTo(k string) string {
	switch k {
	case writingKindOpening:
		return "opening"
	case writingKindClosing:
		return "closing"
	}
	return "body"
}

// —— 以下只服务迁移 0182 的回填，以及那之前存下的老行 ——

var (
	writingRoleWordsOpening  = []string{"开头", "引言", "开篇", "钩子", "导入", "opening", "hook", "introduction", "intro"}
	writingRoleWordsClosing  = []string{"结尾", "结论", "总结", "收尾", "落点", "结语", "closing", "conclusion", "ending"}
	writingRoleWordsCounter  = []string{"反方", "对方", "反对", "质疑", "counter", "objection"}
	writingRoleWordsRebuttal = []string{"回应", "反驳", "rebuttal", "response"}
	writingRoleWordsGap      = []string{"还没找到", "没找到", "待补", "暂时没有", "缺一份"}
	writingRoleWordsEvidence = []string{
		"例", "经历", "的事", "事件", "故事", "材料", "数据", "研究", "报道", "访谈", "调查",
		"案例", "引用", "名言", "人物", "史实", "素材", "证据", "新闻", "实验", "统计", "场景", "现象",
		"example", "experience", "evidence", "data", "study", "research", "report",
		"story", "quote", "case", "survey", "statistic", "source",
	}
)

func roleContainsAny(role string, words []string) bool {
	r := strings.ToLower(role)
	for _, w := range words {
		if strings.Contains(r, w) {
			return true
		}
	}
	return false
}

// writingKindFromRole 把 0182 之前那些自由散文的 role 映射成一个 kind。
//
// 顺序有讲究：先认最具体的（待补、回应、反方），再认开篇 / 结尾，最后才是论据。
// 「对方举的例子」同时命中反方和论据，而它更该是一条论据 —— 但「反方会说的话」
// 只命中反方，所以反方排在论据前面是对的；真正会错的那一类（「对方的例子」）
// 在闭表落地之后不会再产生。认不出来的按深度兜底。
//
// 🚨 兜底**一行都不丢**：任何 role 都会得到一个 kind。
func writingKindFromRole(role string, depth int32) string {
	switch {
	case roleContainsAny(role, writingRoleWordsGap):
		return writingKindGap
	case roleContainsAny(role, writingRoleWordsRebuttal):
		return writingKindRebuttal
	case roleContainsAny(role, writingRoleWordsCounter):
		return writingKindCounter
	case roleContainsAny(role, writingRoleWordsOpening):
		return writingKindOpening
	case roleContainsAny(role, writingRoleWordsClosing):
		return writingKindClosing
	case roleContainsAny(role, writingRoleWordsEvidence):
		return writingKindEvidence
	}
	switch depth {
	case 0:
		return writingKindThesis
	case 1:
		return writingKindPoint
	default:
		return writingKindEvidence
	}
}

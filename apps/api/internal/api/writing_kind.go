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
	writingKindEvidence = "evidence" // 论据 · 她自己见过、经历过的事
	// 论据 · 她从别处来的：一份研究、一条报道、一组数据、一次访谈，
	// 也包括社会上的、历史上的例子。
	//
	// 🚨 为什么和 evidence 分成两个 kind，而不是看 source 空不空：
	// 一个司马迁的例子没有链接可填，但它显然不是「她见过的事」。
	// 2026-09-18 产品负责人：「个人经历是信效度最低的，最好是使用社会上的、
	// 历史上的例子」—— 那条判据（writingPlanShape.Wider，至少要有一条）
	// 靠的就是这个区分，用 source 来推会把每一个历史例子都算成她的亲身经历。
	writingKindReference = "reference"
	// 道理：撑住一条分论点的推理，而不是一件事。
	//
	// 🚨 它**不是材料**。2026-09-18 实测：「脆弱的感受带来关于自己渴望的信息」
	// 是一条道理，却被当成了一个「不是个人经历的例子」，于是只有她自己那一件事
	// 也过了线。道理站得住是好事，但它撑不起「你有什么证据」那一问。
	// 对应 vocab 里的 point_reasoning（道理论证），是语文课上真有的那个东西。
	writingKindReasoning = "reasoning"
	writingKindCounter   = "counter"  // 反方观点
	writingKindRebuttal  = "rebuttal" // 对反方的回应
	writingKindGap       = "gap"      // 待补的材料
	writingKindClosing   = "closing"  // 结尾

	// —— 记叙文的四种（R4，2026-09-21）——
	//
	// 上面十种是按议论文的骨架定的。产品负责人拿来的那位语文老师的讲义里
	// 还有一整套记叙文的教法，而在这之前，一篇记叙文进了这间屋子会被硬塞进
	// 中心论点 / 分论点 / 论据 —— 那是一副用不上的骨架。
	//
	// 只有四种，因为记叙文的骨架本来就比议论文平：一件事、事里的细节、
	// 让情感反过来的那个转折、最后落下来的那句。
	writingKindScene   = "scene"   // 场景：一件事，有时间有地点
	writingKindDetail  = "detail"  // 细节：场景里的动作、神态、话、环境
	writingKindTurn    = "turn"    // 转折：让你改观的那一下（讲义的「转」）
	writingKindFeeling = "feeling" // 感悟：这件事之后你明白了什么（讲义的「扬」）

	// —— 书信的三种（2026-09-23）——
	//
	// 产品负责人：「书信 is a very important format in junior english. but
	// currently we would guide students to write a letter under the structure
	// of 议论文.」在这之前一封信必然被判成议论文，于是一个初中生被要求给一封
	// 信写中心论点、分论点和论据。
	//
	// 只有三种，而且**不包括称呼和落款**：那两样是格式不是内容，段落那一步的
	// 开头卡和结尾卡已经各占一张（writingCards 会给没有开篇/结尾节点的一篇
	// 补上虚拟卡），在图上再摆一个「称呼」只是让她多拖一个不用想的块。
	//
	// 骨架来自同事那份应用文讲义：一封信要办成一件事（purpose），
	// 为此要说清几件事（matter），最后要有一句给收信人的话（courtesy）。
	// 讲义里那句「结尾是真诚的交际收束，还是观点总结式套话」指的就是最后这一种 ——
	// 把给朋友的信写成议论文总结，是这一档最常见的失分。
	writingKindPurpose  = "purpose"  // 写信目的：这封信要办成的那件事
	writingKindMatter   = "matter"   // 要点：为办成那件事，要说清楚的一件事
	writingKindCourtesy = "courtesy" // 结尾的话：给收信人的那一句（期待回复 / 致谢 / 祝愿）
)

var writingKindDepths = map[string]int32{
	writingKindOpening: 0, writingKindThesis: 0, writingKindClosing: 0,
	writingKindPoint: 1, writingKindCounter: 1,
	writingKindEvidence: 2, writingKindReference: 2, writingKindReasoning: 2,
	writingKindRebuttal: 2, writingKindGap: 2,
	// 记叙文：场景和转折是主干（深度 1），细节挂在场景底下（深度 2），
	// 感悟收在主干那一层。
	writingKindScene: 1, writingKindTurn: 1, writingKindFeeling: 1,
	writingKindDetail: 2,
	// 书信：目的和结尾的话各自成块（深度 0，和开篇/结尾同层），
	// 要点是中间那一层（深度 1）—— 一封信的主体就是几件要说清的事。
	writingKindPurpose: 0, writingKindCourtesy: 0,
	writingKindMatter:  1,
}

// writingKindGenre 说这一种块属于哪一种文体。空串 = 两种都用
//（开篇和结尾：一篇记叙文也要开头结尾）。
//
// 🚨 表里没有的 kind 返回空串，也就是「两种都用」。方向和
// writingKindDepth 的兜底一致：不认识的东西不该因为认不出来就被藏掉。
func writingKindGenre(k string) string {
	switch k {
	case writingKindThesis, writingKindPoint, writingKindEvidence,
		writingKindReference, writingKindReasoning, writingKindCounter,
		writingKindRebuttal, writingKindGap:
		return genreArgument
	case writingKindScene, writingKindDetail, writingKindTurn, writingKindFeeling:
		return genreNarrative
	case writingKindPurpose, writingKindMatter, writingKindCourtesy:
		return genreLetter
	}
	return ""
}

func writingKindValid(k string) bool {
	_, ok := writingKindDepths[k]
	return ok
}

// writingKindDepth 是这种块在图上的深度。**强制**，不采信模型给的位置。
//
// 不认识的 kind 当分论点处理（深度 1）—— 中间那一层是议论文里最常见的块，
// 也是错了代价最小的一层。真正不认识的 kind 在解析那一步就已经被丢掉了
// （parseWritingPlanReply），这里只是不让一个空字符串把节点送到深度 0。
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
// 论据分两种，各自是一个 kind（见上面 writingKindReference 的注释）。
// 这条区分承重 —— 一篇只拿她自己两件事去撑的议论文，老师读到的是「我觉得」。
//
// source 这个参数留着是为了调用点不必关心哪一种 kind 用得上它；
// 标题本身只看 kind。
func writingKindLabel(k, source string) string {
	_ = source
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
		return "论据 · 你见过的事"
	case writingKindReference:
		return "论据 · 你找来的材料"
	case writingKindReasoning:
		return "道理"
	case writingKindScene:
		return "场景"
	case writingKindDetail:
		return "细节"
	case writingKindTurn:
		return "转折"
	case writingKindFeeling:
		return "感悟"
	case writingKindPurpose:
		return "写信目的"
	case writingKindMatter:
		return "要点"
	case writingKindCourtesy:
		return "结尾的话"
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
	case writingKindOpening, writingKindThesis, writingKindClosing,
		// 书信：写信目的和结尾的话各自成块，和开篇/结尾同层，没有父。
		writingKindPurpose, writingKindCourtesy:
		return nil
	case writingKindPoint, writingKindCounter:
		return lastWritingKind(rows, writingKindThesis)
	case writingKindEvidence, writingKindReference, writingKindReasoning, writingKindGap:
		return lastWritingKind(rows, writingKindPoint)
	case writingKindRebuttal:
		return lastWritingKind(rows, writingKindCounter)
	// 记叙文：场景、转折、感悟都是主干（深度 1），挂在开篇下面 ——
	// 记叙文没有中心论点。细节挂在前面最近的场景上。
	case writingKindScene, writingKindTurn, writingKindFeeling:
		return lastWritingKind(rows, writingKindOpening)
	case writingKindDetail:
		if p := lastWritingKind(rows, writingKindScene); p != nil {
			return p
		}
		// 转折本身也是一个场景，细节可以挂在它下面。
		return lastWritingKind(rows, writingKindTurn)
	// 书信：要点挂在写信目的下面 —— 一封信的几件事都是为那个目的服务的。
	// 还没有写信目的时挂在开篇（称呼）上；两样都没有就留在顶层，
	// 和记叙文的场景同一条（insertPlanNode 的兜底不会把它改成分论点）。
	case writingKindMatter:
		if p := lastWritingKind(rows, writingKindPurpose); p != nil {
			return p
		}
		return lastWritingKind(rows, writingKindOpening)
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
	// 有出处的一定是她找来的 —— 和 0182 的回填同一条规则，同一个顺序。
	if strings.TrimSpace(row.Source) != "" && row.Depth > 0 {
		return writingKindReference
	}
	return writingKindFromRole(row.Role, row.Depth)
}

// writingKindIsMaterial：这个节点算不算「撑得住一条理由的材料」。
//
// 🚨 **gap 不算。** 截图里那个「暂时还没找到的材料」是深度 2，而
// writingPlanShapeOf 只按深度数材料，于是它被当成一条真材料计入 Material，
// 可以把 ready() 推过线 —— 她手上一条材料都没有，产品却说这份计划站得住。
func writingKindIsMaterial(k string) bool {
	return k == writingKindEvidence || k == writingKindReference
}

// writingKindIsWider：这条材料不是她的个人经历 —— 一份研究、一条报道、
// 一组数据，或者社会上、历史上的一个例子。判据里至少要有一条。
func writingKindIsWider(k string) bool {
	return k == writingKindReference
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
	// 只在深度 ≥ 2 上用：「一条理由」在深度 1 是一条分论点，在深度 2 才是
	// 撑着它的一条道理。老的 writingRoleIsReasoning 就是这张表。
	writingRoleWordsReasoning = []string{
		"道理", "解释", "推理", "分析", "原因", "理由",
		"reasoning", "explanation", "analysis", "reason",
	}
	// 🚨 这两张表分工和别的几张不一样：**先判它是不是一条材料，再判它是谁的。**
	//
	// 老的 writingRoleIsPersonal 只在 role 里出现「你 / 自己 / 身边 / 经历过 /
	// 见过」时才算她的亲身经历，**其余一律算「不是个人经历」**。那个方向是故意的，
	// 它当时的注释写着：这个数只用来提醒「还缺一条更有说服力的例子」，
	// 少提醒一次比冤枉她强。回填必须保住同一个方向，否则一批老稿子会在
	// 她没做错任何事的情况下，突然被判成「还缺一条社会上的例子」。
	writingRoleWordsPersonal = []string{
		"你", "自己", "亲身", "个人", "身边", "经历过", "见过",
		"your own", "personal", "my own", "you saw", "you did",
	}
	writingRoleWordsMaterial = []string{
		"例", "经历", "的事", "事件", "故事", "案例", "人物", "证据", "场景", "现象",
		"材料", "数据", "研究", "报道", "访谈", "调查", "引用", "名言", "史实", "素材",
		"新闻", "实验", "统计", "文献", "论文",
		"example", "experience", "evidence", "story", "case",
		"data", "study", "research", "report", "quote", "survey", "statistic", "source", "paper",
	}
)

// writingMaterialKindFromRole：一条材料是她见过的，还是她找来的。
// 只有 role 明说是她的（「你经历过的事」）才算她见过的；其余算她找来的。
func writingMaterialKindFromRole(role string) string {
	if roleContainsAny(role, writingRoleWordsPersonal) {
		return writingKindEvidence
	}
	return writingKindReference
}

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
	case depth >= writingMaterialDepth && roleContainsAny(role, writingRoleWordsReasoning):
		return writingKindReasoning
	case roleContainsAny(role, writingRoleWordsMaterial):
		return writingMaterialKindFromRole(role)
	}
	switch depth {
	case 0:
		return writingKindThesis
	case 1:
		return writingKindPoint
	default:
		// 深度 2 而 role 说不出它是什么：当成她找来的。见上面那段注释 ——
		// 这是老 writingRoleIsPersonal 的默认方向，不是随手挑的。
		return writingKindReference
	}
}

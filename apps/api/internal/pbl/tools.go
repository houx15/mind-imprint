package pbl

import "strings"

// tools.go —— 工具箱。
//
// 产品负责人 2026-09-01 给了七个阶段（docs/2026-09-01-pbl-detail.md）。它们在
// 这里是一张表：印记能递出来的每一件工具，它属于哪一类，界面上叫什么。
//
// 两类（产品负责人原话）：
//
//	thinking —— 在她和 AI 协作当中支持设计、判断、决策。当场做完。
//	world    —— 支持屏幕之外的真实活动。她会离开，几天后才回来。
//
// 🚨 这个区分决定两件事，所以它是数据不是叫法：
//
//	递完 thinking 工具，印记接着说；递完 world 工具，这轮对话就停——她要出门了。
//	挂了三天的 thinking 工具是放弃了；挂了三天的 world 工具是正常的。
//
// 🚨 表是开放的。不在表里的名字照样能召出来（算 thinking），因为新工具不该
// 需要改代码、更不该需要一次迁移。这张表管的是"我们已经想清楚的那些"。

const (
	KindThinking = "thinking"
	KindWorld    = "world"
)

// Tool 是一件工具的全部定义：一个名字，一类，和她在屏幕上看到的那行字。
//
// Label 用日常说法，不用方法论名字。她看到的是「把问题说清楚」，不是
// 「Reframe」；是「先看结构」，不是「信息架构」。方法的名字留在 spec 和
// 注释里。
type Tool struct {
	Name  string
	Kind  string
	Label string
	// Needs 是这件工具的界面**读**的那种产出（ProduceKinds 里的一个）。
	//
	// 🚨 这四件工具的界面本身是空的——理性决策摆的是印记做的那个选择，结构
	// 审查摆的是印记给的那棵提纲，分工建议摆的是印记拆的那几件小事，审核助手
	// 摆的是印记交上来的那份东西。工具是「审」的地方，不是无中生有的地方。
	//
	// 所以递这几件而不同时做出对应的东西，她打开看到的是一块白板加一句「到
	// 对话里请印记先给一个」——她刚照着印记说的点进来，却被打发回去求印记再
	// 做一遍。2026-09-02 线上实测三件全是这样。
	//
	// 空 = 这件工具自己就有内容（观察日记、便签板、复盘…），随时可以递。
	Needs string
	// Only 把这件工具限定在某一类项目里，空 = 所有项目都能用。
	//
	// 🚨 目录是照着这张表渲染进 prompt 的，所以一件只在主页项目里说得通的工具
	// （受众画像、站点采集、视觉基调）如果不限定，印记会在一个讲课间垃圾的
	// 项目里递「受众画像」。工具箱是开放的这件事不变——限定的只是**目录**，
	// 也就是印记会主动想到什么。
	Only string
}

// registry —— 七个阶段展开成的工具。
//
// 阶段一拆成了三件（出去看看 / 便签板 / 把问题说清楚 / 想办法），因为它们是
// 四个不同的时刻，合成一件就又变回了一条固定的流程。
var registry = map[string]Tool{
	"observe":   {Name: "observe", Kind: KindWorld, Label: "观察日记"},
	"board":     {Name: "board", Kind: KindThinking, Label: "头脑风暴"},
	"reframe":   {Name: "reframe", Kind: KindThinking, Label: "问题识别"},
	"ideas":     {Name: "ideas", Kind: KindThinking, Label: "解决方案"},
	"review":    {Name: "review", Kind: KindThinking, Label: "审核助手", Needs: "artifact"},
	"decide":    {Name: "decide", Kind: KindThinking, Label: "理性决策", Needs: "decision"},
	"structure": {Name: "structure", Kind: KindThinking, Label: "结构审查", Needs: "structure"},
	"split":     {Name: "split", Kind: KindThinking, Label: "分工建议", Needs: "substeps"},
	"lookback":  {Name: "lookback", Kind: KindThinking, Label: "项目复盘"},
	"keep":      {Name: "keep", Kind: KindThinking, Label: "长期迭代"},

	// 主页项目那三件。都没有 Needs：它们自己会去生成第一屏的内容（受众候选、
	// 站点分析、配色与头图），所以不需要印记同一轮先做一份东西。这也正是它们
	// 不会变成表单的原因——她打开就有东西可判。
	"persona": {Name: "persona", Kind: KindThinking, Label: "受众画像", Only: "website"},
	"sites":   {Name: "sites", Kind: KindThinking, Label: "站点采集", Only: "website"},
	"look":    {Name: "look", Kind: KindThinking, Label: "视觉基调", Only: "website"},
}

// ToolNeeds 返回这件工具的界面读的那种产出，没有就是空。
//
// 表外的名字一律没有依赖——我们不知道它的界面长什么样，那就退回到朴素卡片，
// 她自己在上面写结果，不会开出一块白板。
func ToolNeeds(name string) string {
	return registry[name].Needs
}

// LookupTool 返回已知工具的定义。
func LookupTool(name string) (Tool, bool) {
	t, ok := registry[name]
	return t, ok
}

// ResolveToolKind 决定一件工具算哪一类。
//
// 已知的工具以表为准——同一件工具在不同项目里属于不同类别是说不通的，那会让
// 「她是在做还是放弃了」这个判断随机化。表外的名字才听请求里说的，因为除了
// 请求也没别处可问。都问不出来就算 thinking：当场做完是更安全的假设，最坏
// 情况是印记多问一句，而不是漏掉一件她其实在外面做着的事。
func ResolveToolKind(name, requested string) string {
	if t, ok := registry[name]; ok {
		return t.Kind
	}
	if requested == KindWorld {
		return KindWorld
	}
	return KindThinking
}

// WaitsForStudent 说的是：递完这件工具，印记该不该停下来。
//
// world 工具意味着她要离开屏幕。继续追问就是在催一个不在场的人。
func WaitsForStudent(kind string) bool { return kind == KindWorld }

// ToolNames 给决策层的目录用，顺序固定，方便快照测试。
func ToolNames() []string {
	return []string{
		"observe", "board", "reframe", "ideas", "review",
		"decide", "structure", "split", "lookback", "keep",
		"persona", "sites", "look",
	}
}

// DefaultProjectName —— 刚建出来的项目先有个名字。
//
// 产品负责人 2026-09-02：「we should let students modify the project'''s title
// when the question is fully defined. or we by default gives one.」
//
// 取她那句话的第一小节（到第一个标点为止），最多 14 个字。整句原文太长，顶在
// 页头上会把整个房间挤没；而完全没有名字，看板上就只能显示一大段话。
//
// 这只是个起名的起点，不是判断——她随时能改，改完 idea 仍然原样留着。
func DefaultProjectName(idea string) string {
	const maxRunes = 14
	trimmed := strings.TrimSpace(idea)
	if trimmed == "" {
		return "新项目"
	}
	cut := strings.IndexAny(trimmed, "，。！？；,.!?;\n")
	if cut > 0 {
		trimmed = trimmed[:cut]
	}
	rs := []rune(strings.TrimSpace(trimmed))
	if len(rs) > maxRunes {
		return string(rs[:maxRunes]) + "…"
	}
	if len(rs) == 0 {
		return "新项目"
	}
	return string(rs)
}

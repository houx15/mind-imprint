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
	"review":    {Name: "review", Kind: KindThinking, Label: "审核助手"},
	"decide":    {Name: "decide", Kind: KindThinking, Label: "理性决策"},
	"structure": {Name: "structure", Kind: KindThinking, Label: "结构审查"},
	"split":     {Name: "split", Kind: KindThinking, Label: "分工建议"},
	"lookback":  {Name: "lookback", Kind: KindThinking, Label: "项目复盘"},
	"keep":      {Name: "keep", Kind: KindThinking, Label: "长期迭代"},
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

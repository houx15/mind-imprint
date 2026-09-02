package pbl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
)

// coach.go — 印记 talking inside a project.
//
// The turn is deliberately thin: it produces prose and, at most, ONE hook
// question. It does not decide the plan, propose roads or mint tools — those
// are separate, gated moves (see spec §8), and folding them into the reply is
// how an assistant starts making decisions in passing.

// Turn is one exchange in a thread.
type Turn struct {
	Role    string // "student" | "ai"
	Content string
}

// CoachInput is everything the turn reads. Assembled by the caller from rows;
// plain values so the prompt building is testable without a database.
type CoachInput struct {
	// Idea is her own opening sentence, kept verbatim.
	Idea string
	// Kind is the project type 印记 judged.
	Kind string
	// Steps is the live plan, if there is one. Empty before she approves.
	Steps []string
	// Recent is the tail of this thread, oldest first.
	Recent []Turn
	// SessionKind is set when this turn happens INSIDE a session; empty on the
	// main thread. It changes what 印记 is doing, so it changes the prompt.
	SessionKind string
	// SessionQuestion is the question that opened the session.
	SessionQuestion string
	// ToolWork is what she actually produced in the tools: the problem she
	// reframed, the notes she wrote on the board, the option she settled on,
	// the artifact she sent back for revision.
	//
	// 🚨 少了这一项，工具就白做了。她在便签板上摆十五分钟，回到对话，而印记
	// 收到的 prompt 和上一轮一模一样——它没看见任何一张便签，只能接着聊上一轮
	// 那件事。学生那边的感受很直接：这个东西没在听我说话。
	//
	// 这是 AGENTS.md 主线里的「回灌陪练」那个箭头（见 api/pbl_refeed.go）。
	ToolWork []string
	// WriteBacks are the conclusions of closed sessions on this thread. Per
	// spec §10.3 the main thread sees CONCLUSIONS, never every turn of every
	// session — that is what keeps a deep dig from flooding the project.
	WriteBacks []string
}

// CoachOutput is one turn's result.
type CoachOutput struct {
	// Reply is what she reads.
	Reply string
	// Hook is an optional question that opens a think-deeply session when she
	// taps it. At most one — 铁律③ is one question at a time, and a message
	// carrying three hooks is three questions wearing a coat.
	Hook string
	// HookKind is the session kind the hook would open.
	HookKind string
	// Tool is a tool 印记 offers her this turn, or "". At most one, and it is
	// an OFFER: she opens it or she does not (铁律②).
	Tool string
	// ToolReason is why this tool, right now, in 印记's own words. A tool with
	// no reason is an ambush, so the server refuses to record one without it.
	ToolReason string
	// Produce is a thing 印记 makes this turn, or nil.
	//
	// 🚨 2026-09-02 之前这一格不存在，于是印记**没有任何办法**做出计划、决定、
	// 成果、分工、结构。审核助手 / 理性决策 / 分工建议 / 结构审查 / 计划这五块
	// 界面因此永远是空的，按钮永远是灰的，而其中三块还写着「到对话里请印记给
	// 一个」——让学生去求一件印记结构上做不到的事。端点、客户端函数、表全都
	// 写好了，链子断在这一格上。
	//
	// 和 tool 一样只有一格、一次一件：多做几件就是一次把五张卡拍在她面前。
	Produce *Produced
}

// Produced is one thing 印记 makes: 用哪种、内容是什么。
//
// Payload 在这里不解释，由 api 层按 Kind 分派给对应的创建逻辑——那边本来就有
// 各自的校验（比如"每一步都要说清楚这一步你判断什么"），不该在这儿抄第二份。
type Produced struct {
	Kind    string
	Payload json.RawMessage
}

// ProduceKinds 是印记能做的东西，也是 prompt 里那份目录的来源。
var ProduceKinds = []struct{ Kind, About string }{
	{"plan", "一份计划：每一步写清楚这一步做什么、你带什么、我带什么、**她判断什么**、做完交回什么"},
	{"decision", "一个要她拿主意的选择：一句话说清在选什么，再给两到四个选项，每个选项写明它意味着什么"},
	{"artifact", "一份你写出来交给她审的东西：草稿、方案、或一个网址"},
	{"substeps", "某一步的分工：拆成几件小事，每件写清楚谁做、为什么是他做"},
	{"structure", "一份结构：一棵两到三层的提纲，让她看得见整件东西的形状"},
}

func IsProduceKind(k string) bool {
	for _, x := range ProduceKinds {
		if x.Kind == k {
			return true
		}
	}
	return false
}

// toolCatalogue renders the toolbox for the prompt.
//
// 🚨 派生自 registry，不手写第二份。手写的目录一定会和工具箱漂移，而漂移的
// 那一天，印记会开始召一件界面上不存在的工具。
func toolCatalogue() string {
	var b strings.Builder
	for _, name := range ToolNames() {
		t, ok := LookupTool(name)
		if !ok {
			continue
		}
		where := "当场和他一起做完"
		if t.Kind == KindWorld {
			where = "他要离开屏幕去做，几天后才回来"
		}
		fmt.Fprintf(&b, "  %s（%s）—— %s\n", t.Name, t.Label, where)
	}
	return b.String()
}

// recentWindow bounds how much thread goes into the prompt.
//
// Load-bearing, not a nicety: lite has no compaction layer, so this is the only
// thing between a long project and an unbounded prompt. Same reasoning and same
// size as the reading room's recentTurnsWindow.
const recentWindow = 12

const coachSystem = `你是「印记」，在陪一个中学生做他自己的项目。

你实际能做的只有两件事：跟他说话，以及在合适的时候把一件工具递到他手边。
你不能替他判断，不能替他决定，也不要说你能查资料、做图、写网站——你现在还
做不了这些，说了他一试就知道。项目是他的。

【语气】
你是一个对他这件事真的感兴趣的人。他说了一件事，你想知道的是那件事本身：
它发生在什么场合、多久出现一次、他当时在做什么、他还看到过什么。

不要核实他凭什么这么说。不要把他的话再说一遍。不要给他的说法贴上
「这是猜的」「这是事实」这类标签——你心里有数就行。

他可能语气很冲，那通常是因为他觉得自己在被考。别端着，也别道歉，接着聊
那件事本身就好。

一次只说一件事，说完停下来等他。句子短一点，用他自己的词。

【怎么问】
🚨 **不要给他两个选项让他挑。**「大多是没动过的，还是吃了一半的？」这种问法，
是把你自己猜的两种可能塞给他，他只能在你的框里选一个——而他真正看到的东西，
很可能两个都不是。

要问得让他必须**自己描述**。先给一个敞开的邀请（多讲讲那天的情况），再顺着这
件事的几个侧面往下问：什么时候发生、有多少、都是些什么类型、涉及的是哪些人。
一次问一个侧面。

他说得具体了，就往前走一步：这件事对谁有影响？他打算先弄清楚什么？

真遇到一个值得单独坐下来想的问题——两句话互相矛盾、一个问题大到没法下手、
一个已经把答案藏在里面的问题——才给一个钩子。平时不用找。

【工具】
你手边有这些，一次最多递一件：

` + "%s" + `

递之前先想清楚这一刻他卡在哪，然后用一句话说明为什么现在需要它。
他可以不用，不用劝。大多数时候一件也不用递。

【你自己动手做的东西】
有些东西该由你做出来，交给他看、由他判断——这不是替他做作业，是把一个具体的
东西摆到他面前，好让他有得可判。他要写的正文永远是他自己写。

你能做的：

` + "%s" + `

什么时候做：

- 他把要做的事说清楚了，还没有计划 → 出一份计划。**每一步都要写清楚这一步
  他判断什么**；一步他什么都不用判断，那一步就不该占他的时间。
- 谈到一个岔路口，往哪边走会影响后面 → 给他一个选择，把选项和各自意味着什么
  摆开，由他定。不要替他选。
- 他需要一份东西才能往下走（一版方案、一份草稿） → 你写出来交给他审，并且
  老实说清楚你猜了什么、这一版你自己觉得哪里还不对。
- 某一步要好几个人一起做 → 给这一步的分工，每件小事写清楚谁做、为什么是他做。
- 要做的东西大到看不见形状 → 给一份结构，两三层就够。

一轮最多做一件。大多数轮次一件也不做——先把话聊清楚。

【输出】
只返回一个 JSON 对象，不要别的字：
{"reply": "你说的话", "hook": "钩子问题，没有就空字符串", "hook_kind": "free",
 "tool": "工具名，不递就空字符串", "tool_reason": "为什么是现在",
 "produce": null}

hook_kind 只能是 free / reframe / brainstorm / observation。

produce 不做就是 null。要做就写成 {"kind": "…", "payload": {…}}，payload 的
形状按 kind：

plan:      {"summary": "一句话概括这版计划", "reason": "为什么是这样安排",
            "steps": [{"title": "", "blurb": "", "goal": "", "youBring": "",
                       "iBring": "", "decide": "他在这一步判断什么", "thenBring": ""}]}
decision:  {"subject": "在选什么", "options": [{"label": "", "description": "它意味着什么"}]}
artifact:  {"kind": "draft|spec|site", "title": "", "body": "正文，site 时留空",
            "url": "网址，只有 site 用", "guessed": ["我猜了什么"], "admits": ["这一版哪里还不对"]}
substeps:  {"stepTitle": "这是计划里哪一步", "items": [{"title": "", "owner": "yinji|student|both", "why": "为什么是他做"}]}
structure: {"nodes": [{"title": "", "body": "", "children": [{"title": "", "body": ""}]}]}`

var errNoReply = errors.New("pbl: coach produced no reply")

// produceCatalogue 把印记能做的东西渲染进 prompt。
//
// 🚨 和 toolCatalogue 一样派生自那份表，不手写第二份——手写的目录一定会漂移，
// 而漂移的那天，印记会开始做一件服务端认不出的东西。
func produceCatalogue() string {
	var b strings.Builder
	for _, k := range ProduceKinds {
		fmt.Fprintf(&b, "  %s —— %s\n", k.Kind, k.About)
	}
	return b.String()
}

// buildCoachContext renders the project state the turn reasons over.
func buildCoachContext(in CoachInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "他一开始是这么说的：%s\n", strings.TrimSpace(in.Idea))
	if in.Kind != "" {
		fmt.Fprintf(&b, "项目类型：%s\n", in.Kind)
	}
	if len(in.Steps) > 0 {
		b.WriteString("现在的计划：\n")
		for i, s := range in.Steps {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, s)
		}
	} else {
		b.WriteString("还没有计划——你们还在把这件事聊清楚。\n")
	}
	// 🚨 她在工具里真做出来的东西。这一段是整个 prompt 里最该被用上的部分：
	// 她写下的句子在这儿，印记就能指着其中一句往下说，而不是把她刚做完的事
	// 再问一遍。
	if len(in.ToolWork) > 0 {
		b.WriteString("\n【他在工具里已经做出来的东西】\n")
		for _, w := range in.ToolWork {
			fmt.Fprintf(&b, "  · %s\n", strings.TrimSpace(w))
		}
		b.WriteString("这些是他自己写下的原话。接着往下说的时候，" +
			"指着其中具体的一句说，不要笼统地夸他「做得不错」，" +
			"更不要把他刚做完的事再问一遍。\n")
	}
	// Conclusions of closed digs, never their working.
	if len(in.WriteBacks) > 0 {
		b.WriteString("他之前深挖过，挖出来的结论：\n")
		for _, w := range in.WriteBacks {
			fmt.Fprintf(&b, "  · %s\n", strings.TrimSpace(w))
		}
	}
	if in.SessionKind != "" {
		fmt.Fprintf(&b, "\n【你们现在在一条支线里】要单独想清楚的是：%s\n",
			strings.TrimSpace(in.SessionQuestion))
		b.WriteString("在支线里不要拉回整个项目，就把这一个问题想透。" +
			"这里不要再给钩子，也不要递工具——支线就是为了想一件事。\n")
		if len(in.Recent) == 0 || in.Recent[len(in.Recent)-1].Role != "student" {
			b.WriteString("这条支线刚开，由你先说第一句：接住他上面说的那件具体的事，" +
				"说清楚这一层要一起看的是什么，然后问他一个问题。" +
				"不要把上面那个问题原样重复一遍。\n")
		}
	}
	tail := in.Recent
	if len(tail) > recentWindow {
		tail = tail[len(tail)-recentWindow:]
	}
	if len(tail) > 0 {
		b.WriteString("\n刚才说到：\n")
		for _, t := range tail {
			who := "印记"
			if t.Role == "student" {
				who = "学生"
			}
			fmt.Fprintf(&b, "%s：%s\n", who, strings.TrimSpace(t.Content))
		}
	}
	return b.String()
}

// parseCoachOutput reads the model's JSON.
//
// A hook naming a kind we do not have is dropped rather than rejected: the
// reply is still worth showing, and a silently missing hook costs her far less
// than a dead turn.
func parseCoachOutput(raw string) (CoachOutput, error) {
	s := strings.TrimSpace(raw)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return CoachOutput{}, errNoReply
	}
	var out struct {
		Reply      string `json:"reply"`
		Hook       string `json:"hook"`
		HookKind   string `json:"hook_kind"`
		Tool       string `json:"tool"`
		ToolReason string `json:"tool_reason"`
		Produce    *struct {
			Kind    string          `json:"kind"`
			Payload json.RawMessage `json:"payload"`
		} `json:"produce"`
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &out); err != nil {
		return CoachOutput{}, fmt.Errorf("pbl: %w", err)
	}
	reply := strings.TrimSpace(out.Reply)
	if reply == "" {
		return CoachOutput{}, errNoReply
	}
	hook := strings.TrimSpace(out.Hook)
	kind := strings.TrimSpace(strings.ToLower(out.HookKind))
	if hook == "" || !IsSessionKind(kind) || kind == "plan_check" {
		// plan_check is never opened by a hook — it is opened by a staged
		// structural change, which is a different trigger entirely.
		hook, kind = "", ""
	}
	// 🚨 没有理由就当没递。一件说不出为什么的工具，对她是一次打断；而且服务端
	// 本来就会拒绝落库，与其让它半路失败，不如在这里就当它没发生。
	tool := strings.TrimSpace(out.Tool)
	reason := strings.TrimSpace(out.ToolReason)
	if tool == "" || reason == "" {
		tool, reason = "", ""
	}
	// 🚨 认不出的 kind、空 payload，一律当作没做——半个产出比没有产出更糟：
	// 界面会为它腾出位置，然后摆一块空白。
	var produced *Produced
	if p := out.Produce; p != nil {
		k := strings.TrimSpace(strings.ToLower(p.Kind))
		if IsProduceKind(k) && len(p.Payload) > 0 {
			produced = &Produced{Kind: k, Payload: p.Payload}
		}
	}
	return CoachOutput{
		Reply: reply, Hook: hook, HookKind: kind,
		Tool: tool, ToolReason: reason, Produce: produced,
	}, nil
}

const maxCoachAttempts = 3

// backoffBeforeRetry 在两次尝试之间等一小会儿。
//
// 🚨 上游 503（服务过载）是会自己好的那种错。两次尝试贴着发出去，撞的是同一
// 波过载——等于只试了一次。2026-09-02 那轮 walk 里连着五个 503，学生看到的是
// 「AI 暂时没接上」，而其实隔一秒再问就有了。
//
// 等待跟着 ctx 走：她把页面关了，就别再等下去。
func backoffBeforeRetry(ctx context.Context, attempt int) {
	d := time.Duration(1<<attempt) * time.Second
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// Coach runs one turn. Usage is returned even on failure so the caller meters:
// a call that produced nothing still cost money.
func Coach(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in CoachInput) (CoachOutput, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: fmt.Sprintf(coachSystem, toolCatalogue(), produceCatalogue())},
			{Role: gateway.RoleUser, Content: buildCoachContext(in)},
		},
		// 🚨 推理模型（deepseek-reasoner）会先花掉一大截 completion token 想事情，
		// 之后才吐出可见内容。原来是 1200，被想事情吃光，返回空 content →
		// 解析失败 → 她看到一句"接口错误"。
		//
		// 16384 是产品负责人 2026-09-02 定的：对话是这个产品的主干，不该在这里
		// 省。见 [[llm-reasoning-model-budgets]]。
		MaxTokens: 16384,
	}
	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxCoachAttempts; attempt++ {
		if attempt > 0 {
			backoffBeforeRetry(ctx, attempt-1)
		}
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		out, perr := parseCoachOutput(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return out, lastUsage, nil
	}
	return CoachOutput{}, lastUsage, lastErr
}

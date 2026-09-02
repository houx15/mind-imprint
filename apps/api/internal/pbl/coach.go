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

【输出】
只返回一个 JSON 对象，不要别的字：
{"reply": "你说的话", "hook": "钩子问题，没有就空字符串", "hook_kind": "free",
 "tool": "工具名，不递就空字符串", "tool_reason": "为什么是现在"}

hook_kind 只能是 free / reframe / brainstorm / observation。`

var errNoReply = errors.New("pbl: coach produced no reply")

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
	return CoachOutput{
		Reply: reply, Hook: hook, HookKind: kind,
		Tool: tool, ToolReason: reason,
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
			{Role: gateway.RoleSystem, Content: fmt.Sprintf(coachSystem, toolCatalogue())},
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

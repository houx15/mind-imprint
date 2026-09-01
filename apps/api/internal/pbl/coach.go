package pbl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
}

// recentWindow bounds how much thread goes into the prompt.
//
// Load-bearing, not a nicety: lite has no compaction layer, so this is the only
// thing between a long project and an unbounded prompt. Same reasoning and same
// size as the reading room's recentTurnsWindow.
const recentWindow = 12

const coachSystem = `你是「印记」，正在和一个中学生一起做他自己的项目。

你可以动手做事：查资料、写文档、出方案、做图、搭静态页面。你不做的是**替他判断**。
每一步都要说得出「这一步他判断什么」。

怎么说话：
- 一次只问一个问题。问完就停下来等他答，不要一口气问三个。
- 先说你已经弄明白或者已经做完的东西，再邀请他做一个动作。
- 用他自己的话来命名他的观察和问题。你改写了，就要让他能改回去。
- 短句。不要教学名词考他。不要长篇总结换他一个「好」。
- 他说的是猜的，就标成猜的；是他亲眼看见的，才叫看见的。

什么时候给钩子问题：
当这一刻真的有一个值得单独想一层的问题——一个矛盾、一个太大的问题、一个已经把
答案写进去的问题——就给一个钩子。他点开就进一条支线，单独想这一个。
没有就不给。为了显得深刻而挂一个钩子，比不挂更糟。

只返回一个 JSON 对象：
{"reply": "你要说的话", "hook": "钩子问题，没有就空字符串", "hook_kind": "free"}

hook_kind 只能是 free / reframe / brainstorm / observation。
只回 JSON，不要代码块以外的任何字。`

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
		b.WriteString("在支线里不要拉回整个项目，就把这一个问题想透。这里不要再给钩子。\n")
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
		Reply    string `json:"reply"`
		Hook     string `json:"hook"`
		HookKind string `json:"hook_kind"`
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
	return CoachOutput{Reply: reply, Hook: hook, HookKind: kind}, nil
}

const maxCoachAttempts = 2

// Coach runs one turn. Usage is returned even on failure so the caller meters:
// a call that produced nothing still cost money.
func Coach(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in CoachInput) (CoachOutput, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: coachSystem},
			{Role: gateway.RoleUser, Content: buildCoachContext(in)},
		},
		MaxTokens: 1200,
	}
	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxCoachAttempts; attempt++ {
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

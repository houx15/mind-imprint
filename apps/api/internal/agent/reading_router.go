package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

type FocusSpan struct {
	BlockID string
	Quote   string
}

type PacingState struct {
	OpenCard              bool
	TurnsSinceLastPropose int
	RecentlySkipped       []string
	CompletedCards        []string
	HasNewFocus           bool
}

type ReadingRouteInput struct {
	StudentText    string
	Article        string // the material's rendered article text, for a grounded reply
	FocusedSpans   []FocusSpan
	RecentTurns    []string
	Catalog        []ReadingCard
	ScaffoldLevels map[string]int
	Pacing         PacingState
	Brief          ReadingBrief // S2 · brief-in
}

type ReadingDecision struct {
	Decision       string // "respond" | "hint" | "summon"
	CardID         string
	Reason         string // student-facing nudge text
	Reply          string // ALWAYS-present short conversational answer, grounded in the article
	ExampleBlockID string // summon only
	ExampleQuote   string // summon only; must be verbatim from the block
	ExampleWhy     string
	FollowupPlan   []string // <=2 secondary card ids, queued not shown
	// Thinking is the model's reasoning for this turn, when the route emitted
	// any. It exists to be shown FOLDED next to the reply — the student can
	// open it if she wants to see how the thing that is asking her questions
	// arrived at this one.
	//
	// It is never persisted and never fed back into a later turn: it is the
	// model's scratch work, and the student's record is the process tree.
	Thinking string
}

// routerReply is the model's raw JSON contract (snake_case on the wire).
type routerReply struct {
	Decision       string   `json:"decision"`
	CardID         string   `json:"card_id"`
	Reason         string   `json:"reason"`
	Reply          string   `json:"reply"`
	ExampleBlockID string   `json:"example_block_id"`
	ExampleQuote   string   `json:"example_quote"`
	ExampleWhy     string   `json:"example_why"`
	FollowupPlan   []string `json:"followup_plan"`
}

// respond is the safe fallback: reply in chat, summon nothing.
var respond = ReadingDecision{Decision: "respond"}

// RouteReading asks the flagship model which reading card (if any) to summon on
// this turn. It NEVER errors on a bad reply or a resolver failure — it degrades
// to "respond" so the reading room stays usable. The program-side gate
// (ApplyReadingGate) is authoritative over pacing/ordering; this function only
// produces the model's raw proposal.
func RouteReading(ctx context.Context, p gateway.Provider, resolver gateway.KeyResolver, in ReadingRouteInput) (ReadingDecision, gateway.Resolved, gateway.ChatUsage, error) {
	resolved, err := resolver(ctx)
	if err != nil {
		return respond, gateway.Resolved{}, gateway.ChatUsage{}, nil
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildRouterPrompt(in)},
			{Role: gateway.RoleUser, Content: buildReadingRouteUserPrompt(in)},
		},
		// 3000, not 400: the flagship is a REASONING model (deepseek-v4-pro) —
		// on a real article + lens catalog its reasoning_content alone exceeds a
		// 400-token budget, truncating before any final JSON, so the router
		// returned an empty Reply and the read-together turn ALWAYS fell back to
		// the generic "which sentence?" default (live bug-hunt 2026-07-30). The
		// visible output is tiny; the headroom is for the reasoning that precedes it.
		MaxTokens: 3000,
		// "low" reasoning, not full: a live A/B on the real router prompt showed
		// full thinking costs ~19–49s/turn while thinking-OFF breaks the router
		// (empty replies, stops offering cards). "low" roughly halves latency
		// (~8–24s) yet keeps a valid decision + non-empty reply + card offers —
		// the middle gear. Evaluation still runs full flagship reasoning elsewhere.
		ReasoningEffort: "low",
	}
	// Up to 2 attempts. The flagship is a REASONING model that intermittently
	// returns an empty or unparseable reply on a real article + lens catalog
	// (~1 in N turns) — which lands the read-together loop on the generic
	// "具体是哪一句" fallback (live bug-hunt 2026-07-30). One retry cuts that
	// tail; usage accumulates across attempts so 档位+token+成本 stays accurate.
	var total gateway.ChatUsage
	for attempt := 0; attempt < 2; attempt++ {
		res, err := gateway.Collect(ctx, p, resolved, req)
		if err != nil {
			return respond, resolved, total, nil // network error: don't hammer the API
		}
		total.InputTokens += res.Usage.InputTokens
		total.OutputTokens += res.Usage.OutputTokens
		var reply routerReply
		if perr := json.Unmarshal([]byte(stripFences(res.Text)), &reply); perr != nil {
			continue // truncated/garbage — retry once, then degrade to respond
		}
		d := ReadingDecision{
			Decision: reply.Decision, CardID: reply.CardID, Reason: reply.Reason, Reply: reply.Reply,
			ExampleBlockID: reply.ExampleBlockID, ExampleQuote: reply.ExampleQuote,
			ExampleWhy: reply.ExampleWhy, FollowupPlan: reply.FollowupPlan,
			Thinking: res.Reasoning,
		}
		switch d.Decision {
		case "respond", "hint", "summon":
		default:
			d = respond
		}
		// An empty reply is exactly what triggers the generic fallback downstream
		// — retry once for a real one before giving up.
		if strings.TrimSpace(d.Reply) == "" && attempt == 0 {
			continue
		}
		if len(d.FollowupPlan) > 2 {
			d.FollowupPlan = d.FollowupPlan[:2]
		}
		return d, resolved, total, nil
	}
	return respond, resolved, total, nil
}

func buildRouterPrompt(in ReadingRouteInput) string {
	var b strings.Builder
	b.WriteString("你是一名批判性阅读教练，正在和学生一起读一篇文章。基于学生此刻的表达和她正在看的原文，先给她一个简短的正常回答，再判断是否要请出一张\"思维卡\"，帮助她更深入地读这篇（不是替她下结论）。\n")
	b.WriteString("克制阶梯：多数时候只需正常回答(respond)；表达和某张卡有合理联系但意图还不明确时给轻提示(hint)；表达清楚、且能在原文里找到一处示范句时才正式请出(summon)。一次只请一张。\n")
	b.WriteString("当学生的问题明显对应某副卡、但你还不确定她此刻是否想用时，优先给 hint 并在 card_id 填上那张卡——这会变成一个「要不要用它看看」的邀请，由她决定，而不是干脆不提。\n")
	b.WriteString("reply 字段是你必须始终填写的——用不超过三句话、口语化、温暖的中文，紧扣文章内容回答学生，绝不替她下结论、只引导她自己往下想；一次只问一个问题。decision 是 respond 时 reply 就是这一轮的全部回答；decision 是 summon/hint 时 reply 是请卡前的简短过渡语。\n")
	b.WriteString("如果给了「阅读目的」，你的每一次追问都要服务这个目的（是在找反驳、印证，还是背景）。\n")
	b.WriteString("可用的卡（只能从这些里选）：\n")
	for _, c := range in.Catalog {
		b.WriteString("- " + c.CardID + "（" + c.Name + "）：" + c.Trigger + "\n")
	}
	b.WriteString("\n只输出 JSON：{\"decision\":\"respond|hint|summon\",\"card_id\":\"...\",\"reason\":\"给学生看的一句话，说明为什么此刻值得看这张卡\",\"reply\":\"始终填写：给学生的简短正常回答，扣紧文章\",\"example_block_id\":\"summon时给出示范句所在的block id\",\"example_quote\":\"summon时给出该block里的一句原文（必须逐字来自原文）\",\"example_why\":\"用不超过两句话解释这句为什么适合这张卡\",\"followup_plan\":[\"最多两张后续卡的id\"]}。respond/hint 时 card_id 可留空、example 字段留空；reply 永远不留空。不要输出任何多余文字。")
	return b.String()
}

// buildReadingRouteUserPrompt assembles the router's user message. Pure, no
// I/O — this is what makes the brief-in behavior (S2 §6) testable without a
// model call. Preserves the pre-brief prompt content exactly; the brief block
// is only added when at least one field is non-empty, so a zero ReadingBrief
// degrades to today's prompt byte-for-byte.
func buildReadingRouteUserPrompt(in ReadingRouteInput) string {
	var b strings.Builder
	if br := in.Brief; strings.TrimSpace(br.Reason+br.Focus+br.PhaseTag+br.ProposalSnap) != "" {
		fmt.Fprintf(&b, "【阅读目的】\n")
		if br.Reason != "" {
			fmt.Fprintf(&b, "- 为什么读这篇：%s\n", br.Reason)
		}
		if br.PhaseTag != "" {
			fmt.Fprintf(&b, "- 服务于：%s\n", br.PhaseTag)
		}
		if br.ProposalSnap != "" {
			fmt.Fprintf(&b, "- 当前论点：%s\n", br.ProposalSnap)
		}
		if br.Focus != "" {
			fmt.Fprintf(&b, "- 学生关注：%s\n", br.Focus)
		}
		b.WriteString("让你的追问围绕这个目的，而不是泛泛而读。\n\n")
	}
	if in.Article != "" {
		b.WriteString("文章：\n" + in.Article + "\n")
	}
	if in.StudentText != "" {
		b.WriteString("学生说：" + in.StudentText + "\n")
	}
	if len(in.FocusedSpans) > 0 {
		b.WriteString("她正在看的原文：\n")
		for _, s := range in.FocusedSpans {
			b.WriteString("[" + s.BlockID + "] " + s.Quote + "\n")
		}
	}
	if len(in.RecentTurns) > 0 {
		b.WriteString("最近对话：\n" + strings.Join(in.RecentTurns, "\n") + "\n")
	}
	return b.String()
}

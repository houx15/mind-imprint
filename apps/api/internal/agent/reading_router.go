package agent

import (
	"context"
	"encoding/json"
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
	FocusedSpans   []FocusSpan
	RecentTurns    []string
	Catalog        []ReadingCard
	ScaffoldLevels map[string]int
	Pacing         PacingState
}

type ReadingDecision struct {
	Decision       string   // "respond" | "hint" | "summon"
	CardID         string
	Reason         string   // student-facing nudge text
	ExampleBlockID string   // summon only
	ExampleQuote   string   // summon only; must be verbatim from the block
	ExampleWhy     string
	FollowupPlan   []string // <=2 secondary card ids, queued not shown
}

// routerReply is the model's raw JSON contract (snake_case on the wire).
type routerReply struct {
	Decision       string   `json:"decision"`
	CardID         string   `json:"card_id"`
	Reason         string   `json:"reason"`
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
			{Role: gateway.RoleUser, Content: buildRouterUser(in)},
		},
		MaxTokens: 400,
	}
	res, err := gateway.Collect(ctx, p, resolved, req)
	if err != nil {
		return respond, gateway.Resolved{}, gateway.ChatUsage{}, nil
	}
	var reply routerReply
	if perr := json.Unmarshal([]byte(stripFences(res.Text)), &reply); perr != nil {
		return respond, resolved, res.Usage, nil
	}
	d := ReadingDecision{
		Decision: reply.Decision, CardID: reply.CardID, Reason: reply.Reason,
		ExampleBlockID: reply.ExampleBlockID, ExampleQuote: reply.ExampleQuote,
		ExampleWhy: reply.ExampleWhy, FollowupPlan: reply.FollowupPlan,
	}
	switch d.Decision {
	case "respond", "hint", "summon":
	default:
		d = respond
	}
	if len(d.FollowupPlan) > 2 {
		d.FollowupPlan = d.FollowupPlan[:2]
	}
	return d, resolved, res.Usage, nil
}

func buildRouterPrompt(in ReadingRouteInput) string {
	var b strings.Builder
	b.WriteString("你是一名批判性阅读教练，正在和学生一起读一篇文章。基于学生此刻的表达和她正在看的原文，判断是否要请出一张\"思维卡\"，帮助她更深入地读这篇（不是替她下结论）。\n")
	b.WriteString("克制阶梯：多数时候只需正常回答(respond)；表达和某张卡有合理联系但意图还不明确时给轻提示(hint)；表达清楚、且能在原文里找到一处示范句时才正式请出(summon)。一次只请一张。\n")
	b.WriteString("可用的卡（只能从这些里选）：\n")
	for _, c := range in.Catalog {
		b.WriteString("- " + c.CardID + "（" + c.Name + "）：" + c.Trigger + "\n")
	}
	b.WriteString("\n只输出 JSON：{\"decision\":\"respond|hint|summon\",\"card_id\":\"...\",\"reason\":\"给学生看的一句话，说明为什么此刻值得看这张卡\",\"example_block_id\":\"summon时给出示范句所在的block id\",\"example_quote\":\"summon时给出该block里的一句原文（必须逐字来自原文）\",\"example_why\":\"用不超过两句话解释这句为什么适合这张卡\",\"followup_plan\":[\"最多两张后续卡的id\"]}。respond/hint 时 card_id 可留空、example 字段留空。不要输出任何多余文字。")
	return b.String()
}

func buildRouterUser(in ReadingRouteInput) string {
	var b strings.Builder
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

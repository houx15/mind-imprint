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

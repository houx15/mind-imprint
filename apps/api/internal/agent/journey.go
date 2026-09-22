package agent

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// JourneyDecision is the model's per-contract verdict when composing a
// student's journey (N6-E). Reason is the LLM's own rationale — recorded as a
// claim in the journey_composed event, never a measurement.
type JourneyDecision struct {
	ID     string `json:"id"`
	Keep   bool   `json:"keep"`
	Reason string `json:"reason"`
}

// ComposeResult is ComposeJourney's return: the ids to waive, the full decision
// list (for the event), and the call's Resolved/Usage for metering. Resolved
// is populated whenever a real model call happened — callers meter on
// Resolved.Provider != "" even when Waived is empty (fail-safe or all-keep).
type ComposeResult struct {
	Waived    []string
	Decisions []JourneyDecision
	Resolved  gateway.Resolved
	Usage     gateway.ChatUsage
}

// ComposeJourney asks a mid-tier model which of the template's stations this
// student can skip, given what she pasted. FAIL-SAFE: any failure — resolver
// error, provider error, unparseable reply, a decision set that does not
// EXACTLY cover the skill's contracts, or an unknown id — yields an empty
// waived-set (today's full journey). It never errors; the caller always gets a
// result it can meter. 铁律 2: waiving is only ever an offer, and re-open (Task
// 5) is the student's escape, so a permissive composer is safe.
func ComposeJourney(ctx context.Context, provider gateway.Provider, resolver gateway.KeyResolver, sk skills.Skill, pasted string) ComposeResult {
	resolved, err := resolver(ctx)
	if err != nil {
		return ComposeResult{}
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: composePrompt(sk)},
			{Role: gateway.RoleUser, Content: "学生贴进来的内容：\n" + pasted},
		},
		MaxTokens: 3000, // reasoning-model headroom (deepseek-v4-pro) — see reading_router.go
	}
	res, cerr := gateway.Collect(ctx, provider, resolved, req)
	if cerr != nil {
		// A resolver succeeded but the call failed before returning usage: there
		// is nothing meaningful to meter, and nothing to waive.
		return ComposeResult{Resolved: resolved}
	}
	out := ComposeResult{Resolved: resolved, Usage: res.Usage}

	var decisions []JourneyDecision
	if json.Unmarshal([]byte(stripFences(res.Text)), &decisions) != nil {
		return out // malformed → full journey
	}
	// The decision set must EXACTLY cover the skill's contracts — no missing, no
	// extra, no unknown id. Anything else is treated as a malformed reply and
	// falls back to the full journey rather than acting on a partial verdict.
	want := map[string]bool{}
	for id := range sk.Contracts {
		want[id] = false
	}
	for _, d := range decisions {
		seen, ok := want[d.ID]
		if !ok || seen {
			return out // unknown or duplicate id → full journey
		}
		want[d.ID] = true
	}
	for _, covered := range want {
		if !covered {
			return out // a contract went unmentioned → full journey
		}
	}

	var waived []string
	for _, d := range decisions {
		if !d.Keep {
			waived = append(waived, d.ID)
		}
	}
	sort.Strings(waived)
	out.Decisions = decisions
	// An all-waived reply is not credible — the model claiming every station is
	// already done leaves no active station and lets canFinish fire on nothing
	// done, so it degrades to the full journey instead (铁律: an offer is never
	// a wall, and finishing must mean something was done).
	if len(waived) == len(sk.Contracts) {
		return out
	}
	out.Waived = waived
	return out
}

func composePrompt(sk skills.Skill) string {
	order, _ := sk.TopoOrder()
	var b strings.Builder
	b.WriteString("你是一名批判性思维写作教练。下面是一条完整的写作项目流程，共若干环节，按先后顺序排列。")
	b.WriteString("学生带着自己的任务进来，可能已经完成了其中一些环节。请根据学生贴进来的内容判断：哪些环节学生已经实质做过、可以跳过（keep=false），哪些还需要走一遍（keep=true）。\n")
	b.WriteString("拿不准时保留（keep=true）——跳过只是一个建议，学生随时能把某个环节重新打开。\n\n环节：\n")
	for _, id := range order {
		c := sk.Contracts[id]
		b.WriteString("- id=" + id + "：" + c.Title)
		if len(c.Produces) > 0 {
			b.WriteString("（产出：" + strings.Join(c.Produces, "、") + "）")
		}
		b.WriteString("\n")
	}
	b.WriteString("\n只输出一个 JSON 数组，必须恰好包含上面每一个 id，各一次，形如 ")
	b.WriteString(`[{"id":"环节id","keep":true,"reason":"一句话理由"}]。不要输出任何多余文字。`)
	return b.String()
}

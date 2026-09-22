package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
)

// project_coach.go — S1's ONE continuous per-project coach. There is one agent,
// one session; the four rooms (计划/阅读/写作/回顾 …) are views, not separate
// agents. This is the always-reply conversational coach (mirrors chat_coach's
// ProposeChatReply enforcement) lifted to project scope: it sees the WHOLE
// thread (continuity) plus a compact spine projection, and is told which room
// the student is in. Card offers / interventions are deliberately OFF here —
// cross-phase card proposing is S4; the intervention loop (RunAgentStep) is
// left intact as its substrate.

// ProposeProjectCoachReply asks the coach model (mid-tier, downgradeable) for
// one conversational reply, then runs the chat enforcement subset
// (ValidateOutput + BannedPhrasing) — identical to ProposeChatReply, minus the
// card/flag machinery. Usage is populated whenever Collect succeeded, INCLUDING
// when enforcement then rejects (a rejected reply still cost money; the caller
// must still meter it).
func ProposeProjectCoachReply(ctx context.Context, prov gateway.Provider, r gateway.Resolved, history []ChatTurn, spineProjection, activeSurface string) (enforcement.AgentOutput, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: projectCoachPosturePrompt},
			{Role: gateway.RoleUser, Content: BuildProjectCoachContext(history, spineProjection, activeSurface)},
		},
	}
	res, err := gateway.Collect(ctx, prov, r, req)
	if err != nil {
		return enforcement.AgentOutput{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	out := enforcement.AgentOutput{Type: "reply", Body: strings.TrimSpace(res.Text)}
	if err := enforcement.ValidateOutput(out); err != nil {
		return enforcement.AgentOutput{}, usage, err
	}
	if rule := enforcement.BannedPhrasing(out.Body); rule != nil {
		return enforcement.AgentOutput{}, usage, fmt.Errorf("agent: project coach reply rejected by banned-phrasing rule %q", rule.Name)
	}
	return out, usage, nil
}

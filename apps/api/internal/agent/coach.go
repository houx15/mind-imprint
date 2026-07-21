package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
)

// ProposeIntervention asks the flagship model (via prov/r) for one anchored
// question body for the given candidate move, then runs the full
// enforcement stack (design §5) before returning it:
//
//  1. ValidateOutput — the typed-output shape guard.
//  2. BannedPhrasing — a non-nil match rejects the output outright (error,
//     never emitted, never persisted).
//  3. OutputCheck — a declarative echo of the anchored node's own content is
//     intercepted and rewritten as a question.
//
// The anchor and criterion come from c (the classifier's candidate), never
// from parsing the model's reply — the model only ever supplies the
// question body text. sim is the injected embedding-similarity seam (Slice
// 0's enforcement.Similarity); production wires a real embedding-backed
// implementation, tests pass a stub/heuristic — no real embedding call
// happens here.
//
// Returns the enforced AgentOutput, the output-check verdict ("pass" |
// "intercept"), and the LLM call's token usage (design's "记录档位 + token +
// 成本" hard constraint — this is one of the three live gateway.Collect call
// sites, agent/anchors.go and agent/course.go being the other two). Usage is
// populated whenever gateway.Collect succeeded — including when enforcement
// then rejects the output (err != nil): a rejected reply still cost money,
// so the caller must still record it even though it is never persisted or
// emitted. Usage is the zero value only when Collect itself errored (no
// tokens were spent).
//
// refeed is threaded straight through to BuildCoachContext (N3b Seam B): it
// is non-nil only when c is the refeed candidate (AnchorKind ==
// "card_instance"); every other caller passes nil.
func ProposeIntervention(ctx context.Context, prov gateway.Provider, r gateway.Resolved, g GraphView, c Candidate, history []ChatTurn, sim enforcement.Similarity, refeed *RefeedPayload) (enforcement.AgentOutput, string, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: coachPosturePrompt},
			{Role: gateway.RoleUser, Content: BuildCoachContext(g, c, history, refeed)},
		},
	}
	res, err := gateway.Collect(ctx, prov, r, req)
	if err != nil {
		return enforcement.AgentOutput{}, "", gateway.ChatUsage{}, err
	}
	usage := res.Usage

	out := enforcement.AgentOutput{
		Type:      "question",
		Anchor:    enforcement.OutputAnchor{Kind: c.AnchorKind, ID: c.AnchorID},
		Criterion: c.Criterion,
		Body:      strings.TrimSpace(res.Text),
	}
	if err := enforcement.ValidateOutput(out); err != nil {
		return enforcement.AgentOutput{}, "", usage, err
	}
	if rule := enforcement.BannedPhrasing(out.Body); rule != nil {
		return enforcement.AgentOutput{}, "", usage, fmt.Errorf("agent: coach output rejected by banned-phrasing rule %q", rule.Name)
	}

	verdict := enforcement.OutputCheck(out.Body, enforcement.Context{Topic: anchorTopic(g, c)}, sim)
	if verdict.Verdict == "intercept" {
		out.Body = verdict.Rewrite
	}
	return out, verdict.Verdict, usage, nil
}

// anchorTopic is the OutputCheck comparison topic: the anchored node's own
// text (the student's claim/artifact content), so a declarative echo of it
// is caught. Empty when the neighborhood view carries no text for the node.
func anchorTopic(g GraphView, c Candidate) string {
	if n, ok := findNode(g, c.AnchorID); ok {
		return n.Text
	}
	return ""
}

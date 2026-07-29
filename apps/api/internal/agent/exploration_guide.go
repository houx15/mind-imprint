package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"mindimprint/api/internal/gateway"
)

// exploration_guide.go — S3 · the rabbit-hole exploration surface's "guide".
// Points the student at the next necessary research direction from her own
// exploration graph (sources she's engaged + open leads she's logged). Never
// fetches anything, never decides for her which source is worth it, never
// concludes her research — 铁律 · 克制. Mirrors ComposeReadingTakeawaySuggestions:
// pure input → one isolated call → struct. Does not persist; the student
// decides whether to turn a direction into a lead.

// ExplorationGraphSource is one reference the student has engaged with,
// summarized for the guide prompt — never the full material.
type ExplorationGraphSource struct {
	Title       string // truncated
	Decision    string // use|maybe|drop|"" (未决)
	Credibility string // strong|mixed|weak|""
	PhaseTag    string // "" if unset
	State       string // 未读|在读|已归纳
}

// ExplorationGuideInput is ComposeExplorationGuide's sole input: the
// project's exploration graph as it stands right now.
type ExplorationGuideInput struct {
	ProposalObjective string // from spine; may be ""
	Sources           []ExplorationGraphSource
	OpenLeads         []string // open lead texts
	PrunedCount       int
	ConnectedCount    int
}

// GuideDirection is one next-necessary-direction suggestion — a prompt to
// chase, not an answer. Wire shape matches packages/contracts's GuideDirection.
type GuideDirection struct {
	Direction string `json:"direction"` // the next necessary direction, as a prompt
	Why       string `json:"why"`       // one line: the gap it fills
}

const explorationGuideSystem = `你在帮学生看清研究的森林，而不是替他做研究。指出下一个必要的方向或没接上的缺口，用一句话说明它填补什么。绝不替学生检索、绝不替他下结论、绝不替他判定某个来源的价值。最多给 3 条，每条是一个可追问的方向，不是一个答案。只输出 JSON：{"directions":[{"direction":"...","why":"..."}]}。`

// HasGraphContent reports whether there is anything in the exploration graph
// to point from. An empty graph (no engaged sources, no open leads) means
// there is nothing to read a direction off of — 克制 requires erroring
// rather than fabricating a direction out of nothing. Exported so callers
// (api.postExplorationGuide) can gate the resolver/compose/meter block on it
// BEFORE ever resolving a provider — an empty graph must never touch the
// network or the llm_call audit trail.
func HasGraphContent(in ExplorationGuideInput) bool {
	return len(in.Sources) > 0 || len(in.OpenLeads) > 0
}

// ComposeExplorationGuide points the student at the next necessary research
// direction(s) from her own exploration graph via one isolated mid-tier LLM
// call. It never fetches, never concludes her research, and errors (rather
// than fabricates) when the graph is empty.
func ComposeExplorationGuide(ctx context.Context, prov gateway.Provider, r gateway.Resolved, in ExplorationGuideInput) ([]GuideDirection, gateway.ChatUsage, error) {
	if !HasGraphContent(in) {
		return nil, gateway.ChatUsage{}, fmt.Errorf("agent: empty exploration graph — nothing to point from")
	}

	var b strings.Builder
	if strings.TrimSpace(in.ProposalObjective) != "" {
		b.WriteString("论点目标：" + in.ProposalObjective + "\n")
	}
	if len(in.Sources) > 0 {
		b.WriteString("已接触的来源（标题｜状态｜决定｜可信度｜阶段）：\n")
		for _, s := range in.Sources {
			b.WriteString("- " + s.Title + "｜" + s.State + "｜" + s.Decision + "｜" + s.Credibility + "｜" + s.PhaseTag + "\n")
		}
	}
	if len(in.OpenLeads) > 0 {
		b.WriteString("尚未追的线索：\n")
		for _, lead := range in.OpenLeads {
			b.WriteString("- " + lead + "\n")
		}
	}
	b.WriteString("已跳过：" + strconv.Itoa(in.PrunedCount) + " 条；已接上：" + strconv.Itoa(in.ConnectedCount) + " 条。\n")

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: explorationGuideSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
	})
	if err != nil {
		return nil, gateway.ChatUsage{}, err
	}

	var out struct {
		Directions []GuideDirection `json:"directions"`
	}
	if err := json.Unmarshal([]byte(stripFences(res.Text)), &out); err != nil {
		return nil, res.Usage, fmt.Errorf("agent: exploration guide parse: %w", err)
	}
	directions := out.Directions
	if len(directions) > 3 {
		directions = directions[:3]
	}
	return directions, res.Usage, nil
}

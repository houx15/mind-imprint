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
	// #12/#13 · when the student asks to dig deeper from ONE lead, FocusLead is
	// that lead's text and Thought is what she typed she's thinking. Both "" for
	// the whole-graph 深挖一层. A focus lead is itself graph content to point from.
	FocusLead string
	Thought   string
}

// GuideDirection is one next-necessary-direction suggestion — a prompt to
// chase, not an answer. Wire shape matches packages/contracts's GuideDirection.
type GuideDirection struct {
	Direction string `json:"direction"` // the next necessary direction, as a prompt
	Why       string `json:"why"`       // one line: the gap it fills
}

const explorationGuideSystem = `你帮助学生根据已有研究记录选择下一步研究方向。direction 和 why 会直接展示给学生，用“你”称呼学生。依据已记录的问题、来源和待探索线索，提出最多 3 个具体、可继续追问的方向，并说明各自补充哪一项信息。只有标题或状态时，不假装读过来源正文，不替学生判定来源价值或得出研究结论。本次不执行检索。只输出 JSON：{"directions":[{"direction":"...","why":"..."}]}。`

// HasGraphContent reports whether there is anything in the exploration graph
// to point from. An empty graph (no engaged sources, no open leads) means
// there is nothing to read a direction off of — 克制 requires erroring
// rather than fabricating a direction out of nothing. Exported so callers
// (api.postExplorationGuide) can gate the resolver/compose/meter block on it
// BEFORE ever resolving a provider — an empty graph must never touch the
// network or the llm_call audit trail.
func HasGraphContent(in ExplorationGuideInput) bool {
	return len(in.Sources) > 0 || len(in.OpenLeads) > 0 || strings.TrimSpace(in.FocusLead) != ""
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
	// #12/#13 · a focused dig: center the directions on this one lead + the
	// student's own thinking, instead of the whole graph.
	if strings.TrimSpace(in.FocusLead) != "" {
		b.WriteString("\n学生想重点深挖这条线索：" + in.FocusLead + "\n")
		if strings.TrimSpace(in.Thought) != "" {
			b.WriteString("学生此刻的想法：" + in.Thought + "\n")
		}
		b.WriteString("请围绕这条线索给出下一步可追问的方向。\n")
	}

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

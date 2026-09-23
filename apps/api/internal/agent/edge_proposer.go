package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"mindimprint/api/internal/gateway"
)

// edge_proposer.go — Task B3: 印记 proposes labeled edges between the
// student's own top-level question nodes (AI proposes, student confirms —
// 铁律②). LLMs cannot reliably echo UUIDs, so the wire protocol with the
// model is entirely INDEX-based: the project's root questions are numbered
// 1..N in the prompt, and the model's proposals reference those same
// 1-based indices. ProposeQuestionEdges maps nothing back to real ids
// itself (the caller, api.proposeQuestionEdges, owns that index<->leadID
// mapping) — it only validates the model's output against the closed label
// set, the index range, and the from==to degenerate case, dropping anything
// that fails rather than surfacing a bad edge for the student to confirm
// (铁律① 克制).

// EdgeProposerQuestion is one root (top-level) question node, in the exact
// order the prompt numbers them — a slice's position IS its 1-based index.
type EdgeProposerQuestion struct {
	Text string
}

// EdgeProposerExistingEdge is one edge already on the graph, described by
// index pair (not lead id) so the model can see what's already connected
// and is asked not to re-propose it.
type EdgeProposerExistingEdge struct {
	FromIndex int // 1-based, into the same Questions slice
	ToIndex   int // 1-based
	Label     string
}

// EdgeProposerInput is ProposeQuestionEdges's sole input.
type EdgeProposerInput struct {
	Questions     []EdgeProposerQuestion
	ExistingEdges []EdgeProposerExistingEdge
}

// EdgeProposal is one surviving proposed edge, still index-based (1-based,
// into the caller's own root-lead slice) — the caller
// (api.proposeQuestionEdges) maps FromIndex/ToIndex back to real lead ids.
type EdgeProposal struct {
	FromIndex int
	ToIndex   int
	Label     string
	Why       string
}

// validEdgeProposalLabel is the closed 5-value vocabulary (constraints.md).
// Duplicated from api.validQuestionEdgeLabel rather than imported — agent
// must not depend on api, and this closed set is small and stable enough
// that duplication is cheaper than a shared-constants package for one map.
var validEdgeProposalLabel = map[string]bool{
	"子问题": true, "支持": true, "反驳/张力": true, "细化": true, "依赖/前提": true,
}

const edgeProposerSystem = `请分析学生已提出的问题之间的关系，帮助学生组织研究思路。依据问题文字判断关联，不替学生回答这些问题。每个问题已经编号（从 1 开始）。找出问题之间真实存在的关系，每条关系只能用这五个固定标签之一描述：子问题、支持、反驳/张力、细化、依赖/前提——不要发明新标签，没有充分依据时不添加关系。不要重复"已经存在的关系"里列出的连接。每条关系给一句话说明理由，理由直接展示给学生，应说明两个问题的具体联系，不重复关系标签。只输出 JSON，不要任何其他文字：{"proposals":[{"from":<问题编号>,"to":<问题编号>,"label":"...","why":"..."}]}。`

// ProposeQuestionEdges proposes labeled edges between the student's own root
// question nodes via one isolated LLM call. Every proposal is defensively
// filtered before it's returned: a label outside the closed set, an
// out-of-range index, or a from==to self-edge is silently dropped — the
// caller never sees it, so it can never be persisted as a proposal for the
// student to confirm.
func ProposeQuestionEdges(ctx context.Context, prov gateway.Provider, r gateway.Resolved, in EdgeProposerInput) ([]EdgeProposal, gateway.ChatUsage, error) {
	var b strings.Builder
	b.WriteString("学生的问题（已编号）：\n")
	for i, q := range in.Questions {
		b.WriteString(strconv.Itoa(i+1) + ". " + q.Text + "\n")
	}
	if len(in.ExistingEdges) > 0 {
		b.WriteString("\n已经存在的关系（不要重复提议）：\n")
		for _, e := range in.ExistingEdges {
			b.WriteString(strconv.Itoa(e.FromIndex) + " -> " + strconv.Itoa(e.ToIndex) + "：" + e.Label + "\n")
		}
	}

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: edgeProposerSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
	})
	if err != nil {
		return nil, gateway.ChatUsage{}, err
	}

	var out struct {
		Proposals []struct {
			From  int    `json:"from"`
			To    int    `json:"to"`
			Label string `json:"label"`
			Why   string `json:"why"`
		} `json:"proposals"`
	}
	if err := json.Unmarshal([]byte(stripFences(res.Text)), &out); err != nil {
		return nil, res.Usage, fmt.Errorf("agent: edge proposer parse: %w", err)
	}

	n := len(in.Questions)
	proposals := make([]EdgeProposal, 0, len(out.Proposals))
	for _, p := range out.Proposals {
		if !validEdgeProposalLabel[p.Label] {
			continue
		}
		if p.From < 1 || p.From > n || p.To < 1 || p.To > n {
			continue
		}
		if p.From == p.To {
			continue
		}
		proposals = append(proposals, EdgeProposal{FromIndex: p.From, ToIndex: p.To, Label: p.Label, Why: p.Why})
	}
	return proposals, res.Usage, nil
}

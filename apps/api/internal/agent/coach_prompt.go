package agent

import (
	"fmt"
	"strings"
)

// coachPosturePrompt is the Slice-2 runtime coach's own restraint-ladder
// posture prompt. It is a NEW, distinct prompt — not extracted from
// promptTemplate (prompt.go is golden-fixture pinned for legacy RunTurn,
// see the package doc). It carries the same pedagogy (克制、一次一问、绝不
// 代笔) but is scoped to this runtime's single move: one anchored question,
// body only, no preamble, no tool call.
const coachPosturePrompt = `# 角色
你是“思维印记”的思维教练，帮助国际课程学生进一步思考。系统已提供本轮关注的节点、提问原因和批判性思维维度。请针对该节点提出一个具体问题，回复直接展示给学生，用“你”称呼学生。

# 任务要求
- 只提问，不提供结论、反例、候选例子或可直接提交的正文。
- 先看对话记录，避免重复已经问过的问题或学生已经回答的内容。
- 依据学生最新表达，选择一个值得澄清的说法、依据或推理步骤。回答偏离原问题时，先澄清偏离处，不重复原问题。
- 学生要求直接给答案时，把当前问题缩小到一个可依据已有文字回答的具体问题。
- 明确指出所讨论的节点内容，让学生知道要看哪句话或哪个想法，避免泛泛询问“其他角度”。
- 围绕提供的提问原因和思维维度，用学生能理解的话提问，不照搬维度标签。

# 输出格式
只输出一句中文问句，以问号结尾，不附引号、前言、编号、选项或其他文字。`

// findNode looks up a graph node by id within the shallow neighborhood view.
func findNode(g GraphView, id string) (GraphNodeView, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return GraphNodeView{}, false
}

// neighborEdges returns the edges directly touching nodeID (design §9 open
// question 1: the changed node + its direct edges).
func neighborEdges(g GraphView, nodeID string) []GraphEdgeView {
	var out []GraphEdgeView
	for _, e := range g.Edges {
		if e.FromID == nodeID || e.ToID == nodeID {
			out = append(out, e)
		}
	}
	return out
}

// BuildCoachContext assembles the coach's USER-turn context: the anchored node
// + its direct edges (the graph neighborhood) and the classifier's reason + CT
// criterion for this candidate move. The posture ladder is sent separately as
// the system message (coachPosturePrompt) so the request always carries a
// user turn — a system-only request is rejected by real providers. The model
// only ever sees these two — the anchor and criterion on the returned
// AgentOutput come from c, not from parsing the model's reply.
//
// refeed is non-nil only for the N3b Seam B refeed candidate (c.AnchorKind ==
// "card_instance"): in that case the just-submitted card's own contents
// replace the graph-node/edges blocks below — the coach's one question must
// be about what the student just wrote, not a stale graph neighborhood. For
// every other anchor kind, refeed is ignored and the output is unchanged.
func BuildCoachContext(g GraphView, c Candidate, history []ChatTurn, refeed *RefeedPayload) string {
	var b strings.Builder
	if len(history) > 0 {
		b.WriteString("# 对话记录（最近在前为旧、在后为新）\n")
		for _, t := range history {
			who := "学生"
			if t.Role == "assistant" {
				who = "教练"
			}
			fmt.Fprintf(&b, "%s：%s\n", who, t.Content)
		}
		b.WriteString("\n")
	}

	if c.AnchorKind == "card_instance" && refeed != nil {
		b.WriteString("# 学生刚完成的工具卡\n")
		fmt.Fprintf(&b, "卡片：%s\n", refeed.CardName)
		for _, step := range refeed.Steps {
			fmt.Fprintf(&b, "## %s\n", step.Title)
			for _, a := range step.Answers {
				fmt.Fprintf(&b, "- %s：%v\n", a.Label, a.Value)
			}
		}
	} else {
		b.WriteString("# 当前锚点节点\n")
		if n, ok := findNode(g, c.AnchorID); ok {
			fmt.Fprintf(&b, "- id=%s type=%s author=%s", n.ID, n.Type, n.Author)
			if n.Text != "" {
				fmt.Fprintf(&b, " text=%q", n.Text)
			}
			b.WriteString("\n")
		} else {
			fmt.Fprintf(&b, "- id=%s（该节点内容未在当前视图中）\n", c.AnchorID)
		}

		if edges := neighborEdges(g, c.AnchorID); len(edges) > 0 {
			b.WriteString("\n# 相关边\n")
			for _, e := range edges {
				fmt.Fprintf(&b, "- %s:%s --%s--> %s:%s\n", e.FromKind, e.FromID, e.Type, e.ToKind, e.ToID)
			}
		}
	}

	b.WriteString("\n# 为什么此刻需要介入\n" + c.Reason + "\n")
	b.WriteString("\n# CT 维度\n" + c.Criterion + "\n")
	return b.String()
}

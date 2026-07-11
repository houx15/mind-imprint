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
你是「思维印记」新运行时里的思维教练，服务国际课程（IB）方向的学生。系统已经判定「此刻贴合」需要介入——你唯一的动作，是就下面给定的锚点节点，向学生提出**一个**问题。

# 铁律（不可违反）
- **绝不替学生定论、绝不代笔。** 不给结论、不给反例、不给候选例子，也不要把学生已有的论断换个说法复述回去当成你自己的话。
- **一次只问一个问题。** 不铺垫、不客套、不编号、不给选项列表——只输出这一句问题本身。
- **必须锚定到给定的节点**，让学生一眼看出你在说哪一处，不要问泛泛的、无锚点的套话（例如不要问「你有没有考虑过其他角度？」这类问题）。
- **用批判性思维（CT）维度的语言**，聚焦在下面「为什么此刻需要介入」和 CT 维度标签给出的那一点，不要跑题。

# 输出格式
只回复这一句中文问句，以问号结尾；不要引号、不要前言、不要任何其他文字。`

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
func BuildCoachContext(g GraphView, c Candidate) string {
	var b strings.Builder
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

	b.WriteString("\n# 为什么此刻需要介入\n" + c.Reason + "\n")
	b.WriteString("\n# CT 维度\n" + c.Criterion + "\n")
	return b.String()
}

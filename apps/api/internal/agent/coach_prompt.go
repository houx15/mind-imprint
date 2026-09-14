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
- **看对话记录，不要重问。** 你已经问过的问题、她已经回答过的事，都不要再问——换个说法再问一遍也算重问，她会觉得你没在听。
- **顺着她最新的回答往下一层。** 她答了，就拿她刚说的那句话里的一个具体说法做落点，问下一层；她答偏了，就问她答偏的那一处，不要退回去把原来的问题再问一遍。
- **她要你直接告诉她答案时，不给答案，把问题缩小一步。** 换成一个更小、更具体、她看着自己写的那句话马上答得出的问题。
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

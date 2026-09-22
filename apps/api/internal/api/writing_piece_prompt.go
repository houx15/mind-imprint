package api

import "mindimprint/api/internal/promptassembly"

// renderWritingPieceContext renders already-selected facts. Instructions and
// omission markers remain byte-identical to the pre-refactor shared builder.
func renderWritingPieceContext(c writingPieceContext) promptassembly.Document {
	var b promptassembly.Builder
	b.Mark("assignment", "context", "internal/api/writing_piece_context.go")
	b.WriteString("\n【这一篇】\n")
	b.WriteString(writingTopicLine(c.Writing, "题目："))
	b.WriteString(writingLangLine(c.Writing))
	b.WriteString(writingLengthLine(c.Writing, "目标篇幅"))
	if c.Thesis != "" {
		b.WriteString("这一篇的中心论点：" + c.Thesis + "\n")
	}
	b.Mark("piece", "context", "internal/api/writing_piece_context.go")
	b.WriteString("\n【整篇的结构，以及她在每一块写下的字】\n")
	if c.TotalCards == 0 {
		b.WriteString("（还没有结构。）\n")
	}
	selections := []promptassembly.Selection{{ID: "piece-blocks", Total: c.TotalCards, Included: len(c.Cards), Unit: "blocks", Reason: "first 12 blocks"}}
	for i, card := range c.Cards {
		b.WriteString("- 第 " + itoa(i+1) + " 张 · " + card.Name)
		if card.NodeText != "" {
			b.WriteString("：" + card.NodeText)
		}
		if card.Focus {
			b.WriteString("   ← **她现在停在这一块**")
		}
		b.WriteString("\n")
		b.WriteString("    " + renderWritingPieceBody(card.Body) + "\n")
		selections = append(selections, promptassembly.Selection{ID: "block-" + itoa(i+1) + "-body", Total: card.Body.TotalRunes, Included: len([]rune(card.Body.Text)), Unit: "runes", Reason: "first 300 runes; explicit omission marker when truncated"})
	}
	if c.TotalCards > len(c.Cards) {
		b.WriteString("（后面还有几块，没有全部列出。）\n")
	}
	b.Mark("prior-feedback", "mixed", "internal/api/writing_piece_context.go")
	if c.Previous != "" {
		b.WriteString("\n【她这一块之前收到过的意见】\n" + c.Previous)
		b.WriteString("🚨 她已经改过的那几条**不要再说一遍**；还没动的那几条，" +
			"这一轮优先接着说那一条，而不是另起一个新问题。\n")
	}
	return b.Document(selections...)
}

func renderWritingPieceBody(body writingPieceBody) string {
	if !body.Written {
		return "（这一段还没写）"
	}
	if body.TotalRunes <= writingPieceBlockRunes {
		return "她写的：" + body.Text
	}
	return "她写的：" + body.Text + "……（这一段一共 " + itoa(body.TotalRunes) + " 字，后面还有，这里没有全列）"
}

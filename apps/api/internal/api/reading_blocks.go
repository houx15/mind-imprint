package api

import (
	"strconv"
	"strings"
)

// reading_blocks.go — 正文分段。工具卡悬挂在某一段上、AI 的 ExampleBlockID 也
// 指向段 id，所以这个函数必须是纯的、确定的：同样的正文永远得到同样的 id，
// 否则已经悬挂的卡片会集体移位到错误的段落。
//
// Block ⟷ agent.MaterialBlock (internal/agent/prompt.go): the same two
// fields, ID and Text, both plain strings with the same JSON tags. A []Block
// converts to []agent.MaterialBlock element-by-element — that conversion is
// what ResolveExampleAnchor (internal/agent/reading_gate.go) consumes when it
// resolves the AI's ExampleBlockID/ExampleQuote back onto a paragraph.

type Block struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// SplitBlocks splits an article body into paragraphs on blank lines, trims
// each, drops the empties, and numbers what SURVIVES b1, b2, … — so the
// numbering stays stable for as long as the body does.
func SplitBlocks(body string) []Block {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	raw := strings.Split(body, "\n\n")
	out := make([]Block, 0, len(raw))
	for _, p := range raw {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		out = append(out, Block{ID: "b" + strconv.Itoa(len(out)+1), Text: t})
	}
	return out
}

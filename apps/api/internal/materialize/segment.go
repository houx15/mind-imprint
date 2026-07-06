// Package materialize fetches a URL's readable text (SSRF-guarded) and segments
// text into paragraph blocks. It has no DB or api dependency.
package materialize

import (
	"fmt"
	"regexp"
	"strings"
)

// Block is one paragraph of a material; ID is stable within a material.
type Block struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

var blankLine = regexp.MustCompile(`\n[ \t]*\n`)

// Segment splits text on blank-line boundaries into paragraph blocks, collapsing
// internal whitespace and dropping empties. Ids are b0, b1, … in order.
func Segment(text string) []Block {
	blocks := make([]Block, 0)
	for _, part := range blankLine.Split(text, -1) {
		collapsed := strings.Join(strings.Fields(part), " ")
		if collapsed == "" {
			continue
		}
		blocks = append(blocks, Block{ID: fmt.Sprintf("b%d", len(blocks)), Text: collapsed})
	}
	return blocks
}

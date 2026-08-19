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

// Segment splits text into paragraph blocks, collapsing internal whitespace and
// dropping empties. Ids are b0, b1, … in order.
//
// Primary boundary is the blank line (one paragraph = text between blank lines),
// which is what well-formed articles, the URL extractor, and the PDF/DOCX
// extractors emit (the PDF path inserts a blank line per page on purpose). But a
// very common paste — copied out of a PDF viewer, Word, or a chat box —
// separates paragraphs with a SINGLE newline and has no blank lines at all;
// under blank-line-only splitting that whole passage collapses into one giant
// block, and the per-paragraph reading interaction (pick a sentence, reference a
// paragraph) degrades to a single wall of text. So when blank-line splitting
// finds no paragraph structure (≤1 non-empty part) yet the text does contain
// multiple newline-separated lines, fall back to treating each single newline as
// a paragraph break. This never affects sources that already use blank lines
// (PDF pages, extracted HTML) because those take the primary path.
func Segment(text string) []Block {
	parts := splitNonEmpty(blankLine.Split(text, -1))
	if len(parts) <= 1 {
		if byLine := splitNonEmpty(strings.Split(text, "\n")); len(byLine) > 1 {
			parts = byLine
		}
	}
	blocks := make([]Block, 0, len(parts))
	for _, part := range parts {
		collapsed := strings.Join(strings.Fields(part), " ")
		if collapsed == "" {
			continue
		}
		blocks = append(blocks, Block{ID: fmt.Sprintf("b%d", len(blocks)), Text: collapsed})
	}
	return blocks
}

// splitNonEmpty drops parts that are empty or whitespace-only, so a
// blank-line/newline boundary count reflects real paragraphs — not the empty
// fragments a run of consecutive newlines produces.
func splitNonEmpty(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

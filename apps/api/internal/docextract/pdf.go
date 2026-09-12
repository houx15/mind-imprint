// Package docextract turns uploaded document bytes (PDF / DOCX) into readable
// plain text the reading pipeline can segment into blocks. It is pure Go (no
// CGO) so it composes with the CGO_ENABLED=0 build.
package docextract

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDF extracts readable plain text from a PDF's bytes using a pure-Go reader.
//
// Limitations, by design (documented so callers degrade gracefully):
//   - Table row/column structure is NOT preserved — PDF tables are absolutely
//     positioned text runs with no semantic markup — but the cell text still
//     appears in reading order.
//   - Scanned / image-only PDFs (no text layer) yield an empty string; the
//     caller must fall back to the paste-body flow rather than storing nothing.
//
// title is always empty (PDFs rarely carry a trustworthy title); the caller
// falls back to the reference title / filename. Pages are separated by a blank
// line so materialize.Segment produces a block per page rather than one blob.
func PDF(b []byte) (title, text string, err error) {
	if len(b) == 0 {
		return "", "", fmt.Errorf("docextract: empty pdf")
	}
	// ledongthuc/pdf can panic on malformed input; contain it as an error so a
	// bad upload degrades to the paste fallback instead of crashing the handler.
	defer func() {
		if r := recover(); r != nil {
			title, text, err = "", "", fmt.Errorf("docextract: pdf parse panicked: %v", r)
		}
	}()

	r, rerr := pdf.NewReader(bytes.NewReader(b), int64(len(b)))
	if rerr != nil {
		return "", "", fmt.Errorf("docextract: open pdf: %w", rerr)
	}

	fonts := make(map[string]*pdf.Font)
	total := r.NumPage()
	pages := make([]string, 0, total)
	for i := 1; i <= total; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		pageText, perr := p.GetPlainText(fonts)
		if perr != nil {
			continue // skip an unreadable page rather than fail the whole doc
		}
		if s := strings.TrimSpace(pageText); s != "" {
			pages = append(pages, s)
		}
	}
	// 🚨 去页眉页脚页码，**就在这里**：页的边界只在这一层存在。文本一旦拼成
	// 一整篇，「这一行是第 3 页的页眉」和「这一句她写了两遍」就再也分不开了。
	// 见 furniture.go。
	return "", normalizeText(strings.Join(stripFurniture(pages), "\n\n")), nil
}

// normalizeText collapses runs of spaces/tabs to a single space and runs of 3+
// newlines to a paragraph break, so extractor output segments cleanly while
// keeping paragraph/page boundaries intact.
func normalizeText(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.Join(strings.Fields(ln), " ")
	}
	joined := strings.Join(lines, "\n")
	// Collapse 3+ blank lines to exactly one blank line.
	for strings.Contains(joined, "\n\n\n") {
		joined = strings.ReplaceAll(joined, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(joined)
}

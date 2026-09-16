package pbl

import (
	"strings"
	"unicode"
)

// originalQuote resolves typography differences to a unique source span. It
// returns the student's original punctuation, never the model's altered copy.
func originalQuote(source, candidate string) (string, bool) {
	raw := []rune(source)
	var normalized []rune
	var starts, ends []int
	for i, r := range raw {
		if isQuoteGlyph(r) {
			continue
		}
		if unicode.IsSpace(r) {
			if len(normalized) > 0 && normalized[len(normalized)-1] == ' ' {
				ends[len(ends)-1] = i + 1
				continue
			}
			r = ' '
		}
		normalized = append(normalized, r)
		starts = append(starts, i)
		ends = append(ends, i+1)
	}
	corpus := string(normalized)
	target := foldSpace(strings.Map(func(r rune) rune {
		if isQuoteGlyph(r) {
			return -1
		}
		return r
	}, candidate))
	if target == "" || strings.Count(corpus, target) != 1 {
		return "", false
	}
	byteStart := strings.Index(corpus, target)
	start := len([]rune(corpus[:byteStart]))
	end := start + len([]rune(target)) - 1
	return string(raw[starts[start]:ends[end]]), true
}

func isQuoteGlyph(r rune) bool {
	switch r {
	case '“', '”', '‘', '’', '\'', '"':
		return true
	}
	return false
}

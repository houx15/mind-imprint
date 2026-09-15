// Package quotematch answers one question: is this quoted span really hers?
//
// Two callers ask it in different settings — a coaching reply that restates
// a sentence from her writing (package api, writing_ghostquote.go) and an AI
// grading that quotes a sentence to praise or flag it (package litegrade) —
// but the failure mode is the same one, first measured on 2026-09-12: a
// model restates a real sentence of hers with a different trailing 。 or a
// half-width comma, or puts a short term in 「」, and a naive substring
// check calls it invented. Both callers need the same three rules:
//
//   - Normalize before comparing — the most common way a restated quote
//     diverges from the source is spacing or trailing punctuation, not the
//     words themselves.
//   - Skip short quoted spans — under MinRunes, a quoted span is usually a
//     term or label (「让步」「主张」), not a copied sentence, and comparing
//     it against a corpus produces false positives.
//   - Extract only 「」, 『』 and “” pairs — never English straight quotes.
//     They are unpaired-in-spirit (a body of English writing already uses
//     `"…"` for other things — 2026-09-12: `By "more" I mean two things`),
//     so counting them pairwise slices real sentences into fragments that
//     match nothing.
package quotematch

import "strings"

// MinRunes is the shortest quoted span worth comparing against a corpus.
const MinRunes = 8

// Normalize strips whitespace and common punctuation and lowercases, so a
// quote that differs from its source only by spacing or a trailing 。/.
// still matches.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '　',
			'。', '，', '、', '；', '：', '！', '？', '…',
			'.', ',', ';', ':', '!', '?':
		default:
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

// ExtractQuotedSpans returns the text inside each 「」, 『』 or “” pair in s,
// in the order they appear. Unpaired quotes — English straight quotes
// included — are not recognised.
func ExtractQuotedSpans(s string) []string {
	pairs := []struct{ open, close rune }{
		{'「', '」'},
		{'『', '』'},
		{'“', '”'},
	}
	var out []string
	for _, p := range pairs {
		rs := []rune(s)
		for i := 0; i < len(rs); i++ {
			if rs[i] != p.open {
				continue
			}
			for j := i + 1; j < len(rs); j++ {
				if rs[j] == p.close {
					out = append(out, string(rs[i+1:j]))
					i = j
					break
				}
			}
		}
	}
	return out
}

// StripQuotedSpans removes every 「…」, 『…』 and “…” span — delimiters
// included — from s, leaving the surrounding text untouched. A caller that
// judges the writer's own prose (not the quote) uses this so a long quoted
// sentence of hers does not skew that judgement: litegrade's language check
// is the first user of this.
func StripQuotedSpans(s string) string {
	pairs := []struct{ open, close rune }{
		{'「', '」'},
		{'『', '』'},
		{'“', '”'},
	}
	rs := []rune(s)
	out := make([]rune, 0, len(rs))
outer:
	for i := 0; i < len(rs); i++ {
		for _, p := range pairs {
			if rs[i] != p.open {
				continue
			}
			for j := i + 1; j < len(rs); j++ {
				if rs[j] == p.close {
					i = j
					continue outer
				}
			}
		}
		out = append(out, rs[i])
	}
	return string(out)
}

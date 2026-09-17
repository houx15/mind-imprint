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

// Locate finds q in source the way Normalize compares, and returns the span of
// source it matched — verbatim, so a caller can store it and a client can
// highlight it with a plain substring search. The span runs from the first to
// the last matched rune; punctuation or spacing the quote left off either end
// is not included. ok is false when q normalizes to nothing or is not there.
//
// 2026-09-18 写作入口走查：通篇审阅的那条肯定两次都被丢掉，只剩三条问题
// —— 它引的那句和原文只差一个句号。意见的锚点要逐字，但「逐字」应该由我们
// 从原文里取，不该要求模型一个标点不差。
func Locate(source, q string) (string, bool) {
	nq := []rune(Normalize(q))
	if len(nq) == 0 {
		return "", false
	}
	src := []rune(source)
	// normalized rune → index into src
	idx := make([]int, 0, len(src))
	norm := make([]rune, 0, len(src))
	for i, r := range src {
		if skipRune(r) {
			continue
		}
		for _, lr := range strings.ToLower(string(r)) {
			norm = append(norm, lr)
			idx = append(idx, i)
		}
	}
	for start := 0; start+len(nq) <= len(norm); start++ {
		match := true
		for k := range nq {
			if norm[start+k] != nq[k] {
				match = false
				break
			}
		}
		if match {
			return string(src[idx[start] : idx[start+len(nq)-1]+1]), true
		}
	}
	return "", false
}

func skipRune(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '　',
		'。', '，', '、', '；', '：', '！', '？', '…',
		'.', ',', ';', ':', '!', '?':
		return true
	}
	return false
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

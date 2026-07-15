package agent

import "unicode"

// CountWords counts words for the S5 word-budget gate, CJK-aware: each CJK
// ideograph counts as one word, and each maximal run of non-CJK, non-space
// characters (a latin word, a number, an acronym) counts as one word.
// Punctuation attached to a run does not add a word; a standalone punctuation
// run does (rare, acceptable for a budget estimate). The rule is documented
// here because it is the single source of truth for every caller.
func CountWords(s string) int {
	count := 0
	inRun := false
	for _, r := range s {
		switch {
		case isCJK(r):
			count++
			inRun = false
		case unicode.IsSpace(r):
			inRun = false
		default:
			if !inRun {
				count++
				inRun = true
			}
		}
	}
	return count
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		(r >= 0x3040 && r <= 0x30FF) || // hiragana + katakana
		(r >= 0xFF00 && r <= 0xFFEF) // full-width forms
}

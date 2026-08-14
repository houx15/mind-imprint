package evalreport

import (
	"strings"
	"unicode"
)

// CountWords is G6's one canonical body-text length rule for mixed
// Chinese/English writing, where whitespace-tokenizing alone undercounts CJK (a
// paragraph of Chinese has ~0 spaces). The count is:
//
//	(number of CJK ideographs) + (number of non-CJK whitespace-delimited tokens)
//
// Each Han codepoint counts as 1; CJK punctuation and symbols do not count. The
// non-CJK residue (everything that is not a Han ideograph) is split on Unicode
// whitespace and each token counts as 1 — so runs of Latin/digits/etc. that are
// glued to CJK ("中国GDP") still yield their own token. Implemented once,
// server-side (counters are computed here), reused wherever a body-text length
// is shown.
func CountWords(text string) int {
	cjk := 0
	var residue strings.Builder
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			cjk++
			// Break the token run so "中国GDP" -> CJK(中国) + token(GDP), and so a
			// Han char never fuses two Latin tokens across it.
			residue.WriteByte(' ')
			continue
		}
		residue.WriteRune(r)
	}
	tokens := 0
	for _, tok := range strings.Fields(residue.String()) {
		// A residue token counts only if it carries an actual word — a
		// letter or a digit. This drops standalone full-width CJK
		// punctuation ("，" "。"), which the space-insertion above would
		// otherwise leave as its own token, while keeping glued punctuation
		// ("hello,") and numbers ("2026") as one token each.
		if strings.IndexFunc(tok, func(r rune) bool {
			return unicode.IsLetter(r) || unicode.IsNumber(r)
		}) >= 0 {
			tokens++
		}
	}
	return cjk + tokens
}

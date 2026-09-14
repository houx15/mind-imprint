package liteweekly

import (
	"fmt"
	"regexp"
	"strings"
)

// ProseCheck holds the facts a piece of model-written prose about a
// student's week is checked against.
type ProseCheck struct {
	AllowedCodes []string
	Corpus       string // her words: moment quotes + new keywords (see Corpus)
	Titles       string // item titles: finished + stalled (see TitleCorpus)
	FactsText    string
	OtherNames   []string
}

var digitRunRe = regexp.MustCompile(`[0-9]+`)

// span is one bracketed fragment found in the prose: a quote (「…」 or
// curly “…”) or a title (《…》), with its rune-index bounds in the source
// text (bounds are exclusive of end, inclusive of the brackets).
type span struct {
	start, end int
	content    string
	kind       string // "quote" | "title"
}

// extractSpans walks text (as runes) once, collecting every 「…」, “…” and
// 《…》 span in source order. Straight ASCII quotes are not brackets here
// and are left untouched, per Ruling 3: the student's own text contains
// them, and pairing them would misjudge real quotes.
func extractSpans(runes []rune) []span {
	var spans []span
	i := 0
	for i < len(runes) {
		var closeCh rune
		var kind string
		switch runes[i] {
		case '「':
			closeCh, kind = '」', "quote"
		case '“':
			closeCh, kind = '”', "quote"
		case '《':
			closeCh, kind = '》', "title"
		default:
			i++
			continue
		}
		j := i + 1
		for j < len(runes) && runes[j] != closeCh {
			j++
		}
		if j >= len(runes) {
			// unmatched opening bracket: nothing to pair, move on.
			i++
			continue
		}
		spans = append(spans, span{start: i, end: j + 1, content: string(runes[i+1 : j]), kind: kind})
		i = j + 1
	}
	return spans
}

// normalizeDigits rewrites full-width digits ０-９ to ASCII 0-9, leaving
// everything else untouched.
func normalizeDigits(s string) string {
	runes := []rune(s)
	changed := false
	for i, r := range runes {
		if r >= '０' && r <= '９' {
			runes[i] = r - '０' + '0'
			changed = true
		}
	}
	if !changed {
		return s
	}
	return string(runes)
}

func digitRunSet(s string) map[string]bool {
	set := make(map[string]bool)
	for _, run := range digitRunRe.FindAllString(s, -1) {
		set[run] = true
	}
	return set
}

func containsCode(codes []string, code string) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

// CheckProse validates text (all of a card's prose fields, joined) against
// c. text = model-written prose; codes = the evidence codes it used.
//
// codes == nil and c.AllowedCodes == nil together (plan 4's parent report,
// which carries no evidence codes) skip the code check; the quote, digit
// and name checks still run.
//
// CheckProse returns an error naming the failing rule and the offending
// fragment, e.g. "quote not in corpus: 雨是天空的眼泪".
func CheckProse(text string, codes []string, c ProseCheck) error {
	for _, code := range codes {
		if !containsCode(c.AllowedCodes, code) {
			return fmt.Errorf("unknown evidence code: %s", code)
		}
	}

	runes := []rune(text)
	spans := extractSpans(runes)

	for _, sp := range spans {
		switch sp.kind {
		case "quote":
			if sp.content != "" && !strings.Contains(c.Corpus, sp.content) {
				return fmt.Errorf("quote not in corpus: %s", sp.content)
			}
		case "title":
			if sp.content != "" && !strings.Contains(c.Titles, sp.content) {
				return fmt.Errorf("title not in titles: %s", sp.content)
			}
		}
	}

	// Remove verified quote/title spans before extracting digits: a real
	// quote of hers can contain digits that are not in FactsText.
	cleaned := make([]rune, 0, len(runes))
	last := 0
	for _, sp := range spans {
		cleaned = append(cleaned, runes[last:sp.start]...)
		last = sp.end
	}
	cleaned = append(cleaned, runes[last:]...)

	allowedDigits := digitRunSet(normalizeDigits(c.FactsText))
	for _, run := range digitRunRe.FindAllString(normalizeDigits(string(cleaned)), -1) {
		if !allowedDigits[run] {
			return fmt.Errorf("digit not in facts: %s", run)
		}
	}

	for _, name := range c.OtherNames {
		if name != "" && strings.Contains(text, name) {
			return fmt.Errorf("mentions other student: %s", name)
		}
	}

	return nil
}

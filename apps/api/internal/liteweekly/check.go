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
	// SelfName is the name of the student the prose is about. Every
	// occurrence is removed before the other-names check, so a classmate
	// whose name is part of hers (王丽 in 王丽华) does not fail prose that
	// names her. Empty means nothing is removed.
	SelfName string
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

// maxFragmentRunes bounds how much of the surrounding text an unclosed- or
// stray-mark error quotes back.
const maxFragmentRunes = 20

// headFragment returns runes, capped to its first maxFragmentRunes runes.
func headFragment(runes []rune) string {
	if len(runes) > maxFragmentRunes {
		return string(runes[:maxFragmentRunes])
	}
	return string(runes)
}

// tailFragment returns runes, capped to its last maxFragmentRunes runes.
func tailFragment(runes []rune) string {
	if len(runes) > maxFragmentRunes {
		return string(runes[len(runes)-maxFragmentRunes:])
	}
	return string(runes)
}

// extractSpans walks text (as runes) once, collecting every 「…」, “…” and
// 《…》 span in source order. Straight ASCII quotes are not brackets here
// and are left untouched, per Ruling 3: the student's own text contains
// them, and pairing them would misjudge real quotes.
//
// Per Ruling 7: an opening mark with no matching close is an error
// ("unclosed quote: <fragment>") rather than being silently skipped —
// truncated model output looks exactly like an unclosed mark, and this
// check exists to catch invented quotes. A closing mark with no opening
// mark before it is likewise an error ("unmatched closing mark:
// <fragment>"). Marks of a different family found while scanning for a
// close stay literal content of the outer span (e.g. 「…《x》…」 is one
// 「」 span), and an empty 「」 is allowed.
func extractSpans(runes []rune) ([]span, error) {
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
		case '」', '”', '》':
			return nil, fmt.Errorf("unmatched closing mark: %s", tailFragment(runes[:i]))
		default:
			i++
			continue
		}
		j := i + 1
		for j < len(runes) && runes[j] != closeCh {
			j++
		}
		if j >= len(runes) {
			return nil, fmt.Errorf("unclosed quote: %s", headFragment(runes[i:]))
		}
		spans = append(spans, span{start: i, end: j + 1, content: string(runes[i+1 : j]), kind: kind})
		i = j + 1
	}
	return spans, nil
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
	spans, err := extractSpans(runes)
	if err != nil {
		return err
	}

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

	// The name check reads the same cleaned text: a classmate's name inside
	// her verified quote or inside a verified title is not the prose naming
	// that classmate. Her own name is replaced with a newline rather than
	// deleted, so the text on either side cannot join into another name.
	named := string(cleaned)
	if c.SelfName != "" {
		named = strings.ReplaceAll(named, c.SelfName, "\n")
	}
	for _, name := range c.OtherNames {
		if name != "" && strings.Contains(named, name) {
			return fmt.Errorf("mentions other student: %s", name)
		}
	}

	return nil
}

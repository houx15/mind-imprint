// Package textstat computes objective, countable measures of a piece of
// writing — sentence count and length, vocabulary variety, and how densely
// it uses subordinate clauses and connective words. These are facts a
// program can count exactly, so per this repo's standing rule (see the
// package comment of internal/api/writing_repeats.go — 「数得出来的事交给
// 服务端，不要模型去感觉」) they are computed here once, rather than asked of
// the grading model, whose count varies run to run.
//
// 🚨 This is the repo's FIRST shared sentence splitter. Six other call sites
// currently reimplement sentence-terminator logic ad hoc (found by
// inspection 2026-09-23):
//   - internal/api/writing_plan.go:250
//   - internal/api/reading_coach_board_build.go:165
//   - internal/api/reading_coach.go:2095
//   - internal/api/reading_coach_card.go:319
//   - internal/api/reading_coach_card.go:795
//   - internal/teacher/weekly.go:119
//   - internal/api/atom_report.go:583
//
// Consolidating those six onto SplitSentences is future work and explicitly
// OUT OF SCOPE here: touching seven files' splitting behaviour while adding
// a new feature is how a task becomes unreviewable. This package does not
// import, call or modify any of them.
package textstat

import (
	"strings"
	"unicode"

	"mindimprint/api/internal/agent"
)

// SplitSentences splits text into sentences, recognising both Chinese
// terminators (。！？ and the ellipsis …) and English terminators (. ! ?).
//
// It does not split on a '.' that is:
//   - a decimal point between two digits ("3.14"),
//   - part of a known abbreviation ("Mr." "e.g." "etc."),
//   - a single-letter initial ("J. K. Rowling", "U.S."),
//   - glued to a following lowercase letter with no space ("www.example.com").
//
// A run of adjoining terminators ("...", "？！", "……") is folded into one
// boundary, and closing quotes/brackets immediately after a boundary are
// folded into the sentence that just ended ("她说。」" is one sentence, not
// a sentence plus a stray quote — the same rule
// internal/api/reading_coach_board_build.go's narrower splitSentences
// already applies).
//
// This is a best-effort split for measuring writing, not a citation-grade
// tokenizer. Two known misses, both rare in student writing and left
// unhandled rather than special-cased: a sentence that starts with a bare
// capital initial ("A. good day begins...") is not recognised as a new
// sentence; an abbreviation that is also the last word of the whole text
// ("...at Acme Inc.") is folded into whatever text follows it, or, if
// nothing follows, left un-terminated.
func SplitSentences(text string) []string {
	runes := []rune(strings.TrimSpace(text))
	n := len(runes)
	var out []string
	start := 0
	for i := 0; i < n; i++ {
		r := runes[i]
		if !isTerminatorRune(r) {
			continue
		}
		if r == '.' && !isSentenceEndingDot(runes, i) {
			continue
		}
		end := i + 1
		// Fold a run of adjoining terminators ("...", "？！") into one boundary.
		for end < n && (isTerminatorRune(runes[end]) || runes[end] == '.') {
			end++
		}
		// Fold trailing closing quotes/brackets into the sentence that ended.
		for end < n {
			switch runes[end] {
			case '」', '』', '"', '\'', '）', ')', '”', '’':
				end++
				continue
			}
			break
		}
		if sent := strings.TrimSpace(string(runes[start:end])); sent != "" {
			out = append(out, sent)
		}
		start = end
		i = end - 1
	}
	if tail := strings.TrimSpace(string(runes[start:])); tail != "" {
		out = append(out, tail)
	}
	return out
}

func isTerminatorRune(r rune) bool {
	switch r {
	case '。', '！', '？', '…', '.', '!', '?':
		return true
	}
	return false
}

// isSentenceEndingDot decides whether the '.' at runes[i] ends a sentence,
// as opposed to being a decimal point, an abbreviation, an initial, or part
// of an ellipsis/URL. See SplitSentences' doc comment for the rules.
func isSentenceEndingDot(runes []rune, i int) bool {
	prev := runeAt(runes, i-1)
	next := runeAt(runes, i+1)

	// An adjoining dot on either side makes this part of an ellipsis
	// ("...") or a multi-dot abbreviation like "e.g." (handled below by the
	// single-letter-word rule on each of its two dots) — either way this
	// specific dot is a boundary within the run the caller then folds.
	if prev == '.' || next == '.' {
		return true
	}
	if unicode.IsDigit(prev) && unicode.IsDigit(next) {
		return false // decimal point: "3.14"
	}
	word := precedingLetters(runes, i)
	if len(word) == 1 {
		return false // initial, or one half of "e.g." / "i.e." / "U.S."
	}
	if enAbbreviations[strings.ToLower(string(word))] {
		return false
	}
	if next != 0 && !unicode.IsSpace(next) && unicode.IsLower(next) {
		return false // glued to a lowercase continuation: "www.example.com"
	}
	return true
}

func runeAt(runes []rune, i int) rune {
	if i < 0 || i >= len(runes) {
		return 0
	}
	return runes[i]
}

// precedingLetters returns the run of letters immediately before runes[i]
// (i.e. the word the '.' at i is attached to).
func precedingLetters(runes []rune, i int) []rune {
	j := i
	for j > 0 && unicode.IsLetter(runes[j-1]) {
		j--
	}
	return runes[j:i]
}

// enAbbreviations is a closed list of common English abbreviations whose
// trailing '.' does not end a sentence. Not exhaustive — this is a
// best-effort split for measuring writing (see SplitSentences), not a
// citation-grade tokenizer.
var enAbbreviations = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true,
	"sr": true, "jr": true, "st": true, "vs": true, "etc": true,
	"approx": true, "no": true, "cf": true, "viz": true, "inc": true,
	"ltd": true, "co": true, "vol": true, "fig": true, "figs": true,
	"pp": true,
	"jan": true, "feb": true, "mar": true, "apr": true, "jun": true,
	"jul": true, "aug": true, "sep": true, "sept": true, "oct": true,
	"nov": true, "dec": true,
}

// Stats is the four countable measures for one piece of writing.
type Stats struct {
	// TypeTokenRatio 不重复词数 / 总词数。
	TypeTokenRatio float64
	// MeanSentenceLength 平均句长（词）。
	MeanSentenceLength float64
	// ComplexSentenceRatio 含从句的句子占比。
	ComplexSentenceRatio float64
	// ConnectiveDensity 连接词数 / 句数。
	ConnectiveDensity float64
}

// Compute runs all four measures over text. lang selects the Chinese or
// English subordinate-clause and connective word lists ("en" for English,
// anything else for Chinese, matching litegrade.Input.Lang's convention).
func Compute(text, lang string) Stats {
	return Stats{
		TypeTokenRatio:       TypeTokenRatio(text),
		MeanSentenceLength:   MeanSentenceLength(text),
		ComplexSentenceRatio: ComplexSentenceRatio(text, lang),
		ConnectiveDensity:    ConnectiveDensity(text, lang),
	}
}

// TypeTokenRatio 不重复词数 / 总词数。
//
// Types are counted after normalizeToken folds case and strips attached
// punctuation, so "Cats." and "cats" are the same type; tokens are not
// folded, so the denominator still counts every occurrence.
func TypeTokenRatio(text string) float64 {
	tokens := wordTokens(text)
	if len(tokens) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		seen[normalizeToken(t)] = struct{}{}
	}
	return float64(len(seen)) / float64(len(tokens))
}

// MeanSentenceLength 平均句长（词）。
//
// Word count per sentence is agent.CountWords (see wordTokens' doc comment
// for why that counter and not internal/evalreport's).
func MeanSentenceLength(text string) float64 {
	sentences := SplitSentences(text)
	if len(sentences) == 0 {
		return 0
	}
	total := 0
	for _, s := range sentences {
		total += agent.CountWords(s)
	}
	return float64(total) / float64(len(sentences))
}

// ComplexSentenceRatio 含从句的句子占比。
//
// A sentence counts as complex when it contains a subordinating conjunction
// or relative pronoun from complexClauseMarkersZH / EN — a closed list, the
// same shape as this repo's other closed vocabularies (e.g. the writing
// room's symptom catalog). This is a lexical proxy for "has a subordinate
// clause", not a parse: a sentence that uses one of these words some other
// way (e.g. "if" inside a quoted title) is still counted.
func ComplexSentenceRatio(text, lang string) float64 {
	sentences := SplitSentences(text)
	if len(sentences) == 0 {
		return 0
	}
	markers := complexClauseMarkersZH
	matchFn := containsZH
	if lang == "en" {
		markers = complexClauseMarkersEN
		matchFn = containsWordEN
	}
	complex := 0
	for _, s := range sentences {
		if matchFn(s, markers) {
			complex++
		}
	}
	return float64(complex) / float64(len(sentences))
}

// ConnectiveDensity 连接词数 / 句数。
//
// Counts every occurrence of a word/phrase in connectivesZH / EN across the
// whole text (a sentence can use more than one connective), divided by the
// sentence count. Both lists are closed, same reasoning as
// ComplexSentenceRatio's marker lists.
func ConnectiveDensity(text, lang string) float64 {
	sentences := SplitSentences(text)
	if len(sentences) == 0 {
		return 0
	}
	list := connectivesZH
	countFn := countZH
	if lang == "en" {
		list = connectivesEN
		countFn = countWordsEN
	}
	total := 0
	for _, s := range sentences {
		total += countFn(s, list)
	}
	return float64(total) / float64(len(sentences))
}

// complexClauseMarkersZH / EN — closed lists of subordinating conjunctions
// and (for English) relative pronouns.
var complexClauseMarkersZH = []string{
	"虽然", "尽管", "即使", "纵然", "如果", "假如", "假使", "只要", "只有",
	"除非", "因为", "由于", "既然", "不管", "无论", "一旦", "虽说",
}

var complexClauseMarkersEN = []string{
	"although", "though", "because", "since", "while", "if", "unless",
	"whereas", "when", "before", "after", "that", "which", "who", "whom",
	"whose",
}

// connectivesZH / EN — closed lists of transitional/logical connectives.
var connectivesZH = []string{
	"因此", "所以", "但是", "然而", "不过", "而且", "并且", "此外", "另外",
	"总之", "总而言之", "首先", "其次", "最后", "例如", "比如", "换言之",
	"也就是说", "与此同时", "由此可见", "综上所述", "相反", "反之", "可见", "于是", "从而",
}

var connectivesEN = []string{
	"however", "therefore", "moreover", "furthermore", "nevertheless",
	"meanwhile", "consequently", "thus", "hence", "additionally",
	"besides", "similarly", "likewise", "otherwise", "instead", "finally",
	"firstly", "secondly", "overall", "specifically",
	"in addition", "for example", "for instance", "in other words",
	"in conclusion", "as a result", "on the other hand", "in contrast",
}

func containsZH(sentence string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(sentence, m) {
			return true
		}
	}
	return false
}

func countZH(sentence string, markers []string) int {
	n := 0
	for _, m := range markers {
		n += strings.Count(sentence, m)
	}
	return n
}

// containsWordEN / countWordsEN match English markers case-insensitively on
// word boundaries (for single words) or as a substring (for the multi-word
// phrases in connectivesEN), so "if" does not match inside "gift".
func containsWordEN(sentence string, markers []string) bool {
	return countWordsEN(sentence, markers) > 0
}

func countWordsEN(sentence string, markers []string) int {
	lower := strings.ToLower(sentence)
	n := 0
	for _, m := range markers {
		if strings.Contains(m, " ") {
			n += strings.Count(lower, m)
			continue
		}
		n += countWordEN(lower, m)
	}
	return n
}

// countWordEN counts occurrences of a single lowercase word in lower as
// whole words (bounded by non-letters on both sides).
func countWordEN(lower, word string) int {
	n := 0
	idx := 0
	for {
		pos := strings.Index(lower[idx:], word)
		if pos < 0 {
			return n
		}
		start := idx + pos
		end := start + len(word)
		before := rune(0)
		if start > 0 {
			before = rune(lower[start-1])
		}
		after := rune(0)
		if end < len(lower) {
			after = rune(lower[end])
		}
		if !unicode.IsLetter(before) && !unicode.IsLetter(after) {
			n++
		}
		idx = start + 1
	}
}

// wordTokens splits text into word tokens using the same CJK-aware rule as
// agent.CountWords (internal/agent/wordcount.go): each Han ideograph is one
// token, and each maximal run of non-CJK, non-space characters is one token.
//
// 🚨 This repo has two independent, differently-shaped word counters:
// agent.CountWords (chosen here) and internal/evalreport.CountWords, which
// tokenizes its non-CJK residue on Unicode whitespace after inserting a
// space around every Han character (a different rule for how a Han
// character glued to Latin text, e.g. "中国GDP", is split). Do not add a
// third — agent.CountWords was picked because it is already the rule wired
// to the writing room's other word-count-facing surfaces (the S5 word-budget
// gate), and because it is the simpler of the two to keep in lock-step with.
//
// agent.CountWords itself returns only a count, not the token list
// TypeTokenRatio needs to test membership, so its splitting rule is mirrored
// here rather than called directly (its CJK predicate is unexported).
// textstat_test.go asserts len(wordTokens(s)) == agent.CountWords(s) across
// a table of inputs so the two rules cannot silently drift apart.
func wordTokens(s string) []string {
	var out []string
	var run []rune
	flush := func() {
		if len(run) > 0 {
			out = append(out, string(run))
			run = run[:0]
		}
	}
	for _, r := range s {
		switch {
		case isCJK(r):
			flush()
			out = append(out, string(r))
		case unicode.IsSpace(r):
			flush()
		default:
			run = append(run, r)
		}
	}
	flush()
	return out
}

// isCJK mirrors agent.isCJK (unexported, internal/agent/wordcount.go) —
// see wordTokens' doc comment for why this is a mirror rather than a call.
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		(r >= 0x3040 && r <= 0x30FF) || // hiragana + katakana
		(r >= 0xFF00 && r <= 0xFFEF) // full-width forms
}

// normalizeToken folds a token for TypeTokenRatio's type comparison:
// lowercase, with leading/trailing non-letter, non-number runes (attached
// punctuation) stripped so "Cats," and "cats" count as one type.
func normalizeToken(t string) string {
	runes := []rune(t)
	start, end := 0, len(runes)
	for start < end && !isWordRune(runes[start]) {
		start++
	}
	for end > start && !isWordRune(runes[end-1]) {
		end--
	}
	return strings.ToLower(string(runes[start:end]))
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

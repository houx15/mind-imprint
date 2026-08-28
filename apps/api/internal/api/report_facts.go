package api

// report_facts.go — the deterministic half of an end-of-session report: the
// corpus a later step may quote 金句 from, word/paragraph counts for the
// stats line, and a wall-clock estimate that never lets an idle tab inflate
// her focus time.
//
// R4 (binding product ruling): every 金句 printed on an exported report
// picture MUST be the STUDENT's own sentence — never the source article's,
// never the AI's. This file is where that promise is either kept or lost.
// buildReadingCorpus and buildWritingCorpus decide exactly which text is
// even REACHABLE for a later step to quote: a later step validates every
// model-returned quote as a literal substring of reportCorpus.Text, so a
// sentence that never enters this corpus structurally cannot reach the
// picture, no matter what any prompt asks the model to do. Read each
// builder's doc comment before changing what it feeds in — the exclusion
// list there is the whole enforcement mechanism.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"mindimprint/api/internal/store/sqlc"
)

// reportCorpus is the substring oracle a later step checks every proposed
// 金句 against: a quote is only usable if it appears verbatim in Text.
// Text is every corpus fragment newline-joined, in the order fragments were
// added. Where maps a fragment back to a short human label ("我的收获",
// "第 2 段", ...) so a later step can attribute a chosen quote to where it
// came from; Where is NOT itself consulted for the R4 check — only Text is.
type reportCorpus struct {
	Text  string
	Where map[string]string
}

// add appends a trimmed fragment to the corpus and records its label.
// Empty/whitespace-only fragments are silently dropped: they can never be
// quoted anyway, and keeping them out of Where keeps every key a real
// candidate sentence.
func (c *reportCorpus) add(fragment, label string) {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return
	}
	if c.Where == nil {
		c.Where = make(map[string]string)
	}
	c.Where[fragment] = label
	if c.Text != "" {
		c.Text += "\n"
	}
	c.Text += fragment
}

// stripQuotedLines removes leading blockquote lines — any line whose
// trimmed form starts with ">" — from a chat message's content, and trims
// the result.
//
// R4's enforcement point. Since sub-project A, a student chat message
// carries the sentences she POINTED AT in the article as leading "> " lines
// (she quotes a passage, then asks 印记 about it). Those rows carry
// role='student' in atom_message — but the quoted lines are the ARTICLE's
// words, not hers. Without this function, an article sentence would sit
// inside a "hers-only" corpus and could be picked as her own 金句 on an
// exported picture under her name. This function is the only thing that
// separates the two before either buildReadingCorpus or buildWritingCorpus
// ever sees the message content.
func stripQuotedLines(s string) string {
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), ">") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// stripArticleLines is R4's second enforcement point (F1's server-side
// half): it drops any line of s — prefixed with "> " or not — that is a
// literal substring of some block's Text, the same "real paragraph, quoted
// literally" test quotedLinesCiteArticle (reading_coach.go) applies to
// `> `-prefixed lines, generalized here to every line so it also catches an
// article sentence that reached a student message with NO "> " prefix at
// all (see buildReadingCorpus's doc comment for how that happens).
//
// A line of her own prose that happens to ALSO be a literal substring of
// the article is dropped too — this function cannot tell "she typed a
// sentence that coincidentally matches the article" apart from "the client
// failed to prefix an article line", and between those two, dropping is the
// safe choice: R4's whole promise is that nothing on the exported picture
// can be the article's words, and a false-negative drop of a rare
// coincidental match costs her nothing a report already needs, while a
// false-positive keep would be exactly the leak this file exists to close.
func stripArticleLines(s string, blocks []Block) string {
	if len(blocks) == 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			kept = append(kept, line)
			continue
		}
		cited := false
		for _, blk := range blocks {
			if strings.Contains(blk.Text, t) {
				cited = true
				break
			}
		}
		if cited {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// cjkFullwidthPunct covers common CJK fullwidth/ideographic punctuation
// marks, checked in addition to unicode.IsPunct so countWordsForLang does
// not undercount-exclude (i.e. accidentally count as a "word") any mark in
// this set on a Unicode/Go version where it falls outside IsPunct's table.
var cjkFullwidthPunct = map[rune]bool{
	'，': true, '。': true, '！': true, '？': true, '；': true, '：': true,
	'、': true, '「': true, '」': true, '『': true, '』': true,
	'（': true, '）': true, '《': true, '》': true, '—': true, '…': true,
}

// countWordsForLang counts "words" for the report's stats line, branching
// on lang: English is space-delimited (strings.Fields already collapses
// runs of whitespace), Chinese — the default for anything not "en" — has no
// spaces between words, so it counts non-space, non-punctuation RUNES
// (roughly, characters) instead. This repo has already shipped a counter
// that counted characters for English text and was ~5x wrong; the lang
// branch is the entire point of this function.
func countWordsForLang(text, lang string) int {
	if lang == "en" {
		return len(strings.Fields(text))
	}
	count := 0
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		if unicode.IsPunct(r) || cjkFullwidthPunct[r] {
			continue
		}
		count++
	}
	return count
}

// cappedGapSeconds sums the gaps between consecutive (sorted) timestamps,
// each gap clamped to at most capPerGap, and returns the total in seconds.
// This is the same "don't let an idle tab inflate focus time" idea as
// clampHeartbeatSeconds (atom_heartbeat.go), applied to a list of event
// timestamps instead of a single reported duration: a 40-minute silence
// between two events counts for at most capPerGap, not the full 40 minutes.
// Fewer than two timestamps is not a duration at all, so it returns 0.
func cappedGapSeconds(stamps []time.Time, capPerGap time.Duration) int {
	if len(stamps) < 2 {
		return 0
	}
	sorted := make([]time.Time, len(stamps))
	copy(sorted, stamps)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

	var total time.Duration
	for i := 1; i < len(sorted); i++ {
		gap := sorted[i].Sub(sorted[i-1])
		if gap < 0 {
			continue
		}
		if gap > capPerGap {
			gap = capPerGap
		}
		total += gap
	}
	return int(total.Seconds())
}

// collectFieldValueLeaves walks a card's field_values JSON — an arbitrary
// object/array shape determined by that card's own schema — and returns
// every STRING value found anywhere in it, trimmed, empties dropped, in a
// deterministic order (object keys visited sorted, array elements in
// order).
//
// Everything a student types into a lens card's fields is a string
// somewhere in this structure, and nothing AI-authored lives in
// field_values — AI output lives in a card's framework_fill and anchors
// columns, which this function never reads and never receives.
func collectFieldValueLeaves(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	var out []string
	var walk func(any)
	walk = func(node any) {
		switch t := node.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				out = append(out, s)
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(t[k])
			}
		}
	}
	walk(v)
	return out
}

// buildReadingCorpus assembles the ONLY text a reading report's 金句 may be
// picked from. It walks four sources, in this order:
//
//   - the reading takeaway text ("我的收获") — hers: reading_takeaway.text.
//   - every atom_annotation's `note` ("批注") — hers: her commentary on a
//     highlight.
//   - every atom_message with role='student', AFTER stripQuotedLines AND
//     stripArticleLines ("和印记聊的时候") — hers, once the article
//     sentences she quoted at 印记 are stripped out.
//   - every string leaf inside a SUBMITTED atom_card's `field_values`
//     ("读的时候记下的") — hers: everything she typed into a lens card
//     (see collectFieldValueLeaves).
//
// EXCLUDED, deliberately, and never read by this function:
//   - atom_annotation.quote — the ARTICLE's words she highlighted, not
//     hers to be quoted as her own.
//   - atom_card.framework_fill (including .finding) — the AI's 发现 shown
//     at finalize; model output, not her sentence.
//   - atom_card.anchors[].quote — the ARTICLE's words, model-selected.
//   - any atom_card whose status != 'submitted' — a draft/abandoned card
//     may hold half-typed, never-finished text.
//   - any atom_message whose role != 'student' — 印记's own turns.
//
// blocks is the article's own paragraphs (SplitBlocks(src.Body)) — F1's
// server-side half of the fix. stripQuotedLines alone trusts a client
// convention (every quoted line is prefixed with "> "); ReadingCoachPanel's
// send() only prefixes the FIRST line of a multi-line quote, so a
// drag-selection across a hard-wrapped paragraph's internal newlines (a
// PDF/Word paste, a poem, a list with no blank lines — see SplitBlocks'
// blank-line-only split) can leave later lines of the article sitting in a
// student message with no "> " at all. stripArticleLines is the check that
// makes R4 hold regardless of what the client sent, and it ALSO repairs
// transcripts already stored under the old client behaviour, since it runs
// against the article every time a report is (re)generated.
func buildReadingCorpus(takeaway string, notes []sqlc.AtomAnnotation, msgs []sqlc.AtomMessage, cards []sqlc.AtomCard, blocks []Block) reportCorpus {
	var c reportCorpus

	c.add(takeaway, "我的收获")

	for _, n := range notes {
		c.add(n.Note, "批注")
	}

	for _, m := range msgs {
		if m.Role != "student" {
			continue
		}
		c.add(stripArticleLines(stripQuotedLines(m.Content), blocks), "和印记聊的时候")
	}

	for _, card := range cards {
		if card.Status != "submitted" {
			continue
		}
		for _, leaf := range collectFieldValueLeaves(card.FieldValues) {
			c.add(leaf, "读的时候记下的")
		}
	}

	return c
}

// buildWritingCorpus assembles the ONLY text a writing report's 金句 may be
// picked from. It walks four sources, in this order:
//
//   - the writing draft body ("成稿") — hers: writing_draft.body. The
//     writing room's rule is she authors the body herself; AI never writes
//     student prose.
//   - every writing_snippet's `text` ("第 N 段", N = position+1, 1-indexed)
//     — hers.
//   - every writing_outline node's `text` ("提纲") — hers: the outline
//     bullet's WORDING is student-authored even though the outline's
//     scaffolding is system-derived (see AGENTS.md's 铁律 scope ruling —
//     that ruling covers plan/outline generation, not this field itself).
//   - every atom_message with role='student', AFTER stripQuotedLines
//     ("和印记聊的时候") — hers.
//
// EXCLUDED, deliberately, and never read by this function:
//   - writing_outline.role, writing_outline.guide — AI-authored labels/
//     guidance text attached to an outline node, not her wording.
//   - every writing_comment field (scope/summary/points) — this function's
//     signature takes no []sqlc.WritingComment at all, so AI feedback text
//     is structurally unreachable here, not merely filtered out.
//   - any atom_message whose role != 'student' — 印记's own turns.
func buildWritingCorpus(draft string, snippets []sqlc.WritingSnippet, outline []sqlc.WritingOutline, msgs []sqlc.AtomMessage) reportCorpus {
	var c reportCorpus

	c.add(draft, "成稿")

	for _, s := range snippets {
		c.add(s.Text, fmt.Sprintf("第 %d 段", s.Position+1))
	}

	for _, o := range outline {
		c.add(o.Text, "提纲")
	}

	for _, m := range msgs {
		if m.Role != "student" {
			continue
		}
		c.add(stripQuotedLines(m.Content), "和印记聊的时候")
	}

	return c
}

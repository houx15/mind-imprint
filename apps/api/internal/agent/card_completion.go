package agent

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"mindimprint/api/internal/cards"
)

// EvaluateCompletion walks spec.Completion (a closed set of typed
// predicates — agent-spec §3) over the card_instance's anchors and reports
// whether every predicate is satisfied. Pure, no DB. On incomplete, missing
// names the tag/field each unsatisfied predicate was looking for (in
// declaration order); complete carries a nil missing slice.
func EvaluateCompletion(spec cards.Spec, anchors []Anchor) (complete bool, missing []string) {
	for _, pred := range spec.Completion {
		switch pred.Kind {
		case "every_tag_present":
			for _, tag := range pred.Tags {
				if !tagAnswered(anchors, tag) {
					missing = append(missing, tag)
				}
			}
		case "field_written_by":
			if !fieldWrittenBy(anchors, pred.Field, pred.Author) {
				missing = append(missing, pred.Field)
			}
		case "lateral_source_present":
			if !lateralSourcePresent(spec, anchors) {
				missing = append(missing, spec.Params.LateralDimension)
			}
		case "graph_slots_complete":
			for _, slot := range spec.Params.Slots {
				if !slotComplete(anchors, slot) {
					missing = append(missing, slot.ID)
				}
			}
		case "items_bucketed":
			if bucketedCount(anchors, pred.Tags) < pred.Min {
				missing = append(missing, "items")
			}
		case "matrix_complete":
			if row, ok := firstIncompleteMatrixRow(spec, anchors); ok {
				missing = append(missing, row)
			}
		}
	}
	return len(missing) == 0, missing
}

// tagAnswered reports whether some anchor carries the given Dimension with
// a non-empty student Answer (the anchor itself may be AI-authored — the
// question is AI's, the answer is the student's).
func tagAnswered(anchors []Anchor, tag string) bool {
	for _, a := range anchors {
		if a.Dimension == tag && strings.TrimSpace(a.Answer) != "" {
			return true
		}
	}
	return false
}

// fieldWrittenBy reports whether some anchor carries the given
// Dimension/field, was authored by the given author, and has a non-empty
// answer.
func fieldWrittenBy(anchors []Anchor, field, author string) bool {
	for _, a := range anchors {
		if a.Dimension == field && a.Author == author && strings.TrimSpace(a.Answer) != "" {
			return true
		}
	}
	return false
}

// lateralSourcePresent reports whether the student has actually read
// laterally: an anchor on the card's lateral dimension, carrying a material
// that is NOT the one under review, with something written about it. A claim
// of having read laterally is not lateral reading — and the AI cannot satisfy
// this, because no agent path creates a material (RL-2).
func lateralSourcePresent(spec cards.Spec, anchors []Anchor) bool {
	lat, ok := lateralAnchor(spec, anchors)
	if !ok {
		return false
	}
	if strings.TrimSpace(lat.Answer) == "" {
		return false
	}
	return lat.MaterialID != checkedMaterialID(spec, anchors)
}

// slotComplete reports whether a graph-primitive slot is done: some anchor on
// the slot's dimension carries a student sentence of ≥12 runes, and — when the
// slot needs a source — some anchor on the slot's dimension carries a
// non-empty material_id. Text anchors (material_id "") and source anchors
// (answer "") never collide.
func slotComplete(anchors []Anchor, slot cards.Slot) bool {
	hasText := false
	hasSource := false
	for _, a := range anchors {
		if a.Dimension != slot.ID {
			continue
		}
		if utf8.RuneCountInString(strings.TrimSpace(a.Answer)) >= 12 {
			hasText = true
		}
		if strings.TrimSpace(a.MaterialID) != "" {
			hasSource = true
		}
	}
	return hasText && (!slot.NeedSrc || hasSource)
}

// observeWhen is the closed-set "when" clause CRAAP's observe rules use:
// `tag=<dimension> AND note_len<<N>` — matches an anchor of the given
// dimension whose answer is shorter than N bytes.
var observeWhen = regexp.MustCompile(`^tag=(\S+) AND note_len<(\d+)$`)

// ObserveCandidates evaluates spec.Observe (a closed set of typed rules —
// agent-spec §3) over the card_instance's live anchors and yields a
// Candidate for each match, feeding the coach through the same
// enforcement stack as the graph classifier (design §5). Pure, no DB.
func ObserveCandidates(spec cards.Spec, cardInstanceID string, anchors []Anchor) []Candidate {
	var out []Candidate
	for _, rule := range spec.Observe {
		m := observeWhen.FindStringSubmatch(rule.When)
		if m == nil {
			continue
		}
		tag := m[1]
		n, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		for _, a := range anchors {
			if a.Dimension != tag {
				continue
			}
			// Count characters (runes), not bytes — a Chinese note of a few
			// characters must still read as "thin" against a note_len<N rule.
			if utf8.RuneCountInString(strings.TrimSpace(a.Answer)) >= n {
				continue
			}
			out = append(out, Candidate{
				Verb:       rule.Verb,
				AnchorKind: "card_instance",
				AnchorID:   cardInstanceID,
				Criterion:  tag, // the CRAAP dimension that fired (e.g. "authority") — enforcement.ValidateOutput requires a non-empty criterion on a question output
				Reason:     "card dimension answer is thin: " + tag,
				Level:      rule.Level,
			})
		}
	}
	return out
}

// bucketedCount counts anchors the student actually placed: dimension in the
// card's bucket/stop vocabulary AND a non-empty reason. Anchors outside the
// vocabulary are IGNORED, not rejected — certainty-spectrum's `rewrite` anchor
// rides the same anchor list as its five spectrum stops and must not make the
// placement predicate unsatisfiable.
func bucketedCount(anchors []Anchor, vocab []string) int {
	in := make(map[string]bool, len(vocab))
	for _, t := range vocab {
		in[t] = true
	}
	n := 0
	for _, a := range anchors {
		if in[a.Dimension] && strings.TrimSpace(a.Answer) != "" {
			n++
		}
	}
	return n
}

// firstIncompleteMatrixRow reports why a matrix card is not done yet. A row is
// keyed by Anchor.Quote (the student's perspective label — a matrix's rows are
// student-authored, so the row's identity IS what she wrote) and counts only
// when every column in spec.Params.Cols has a non-empty answer for it.
//
// The binding rule is: the card is complete once the COUNT OF COMPLETE ROWS
// reaches spec.Params.MinItems — regardless of how many additional incomplete
// rows also exist. A student who starts (and abandons) an extra perspective
// must not be walled out of a card two other perspectives already satisfy, so
// the full tally is taken BEFORE any row is named as the blocker: count
// complete rows first, and only if that count still falls short do we name
// the first incomplete row (first-seen order) as the most actionable next
// step, falling back to ("rows", true) when no row has even been started.
// Returns ("", false) when the card is done.
func firstIncompleteMatrixRow(spec cards.Spec, anchors []Anchor) (string, bool) {
	var order []string
	filled := map[string]map[string]bool{}
	for _, a := range anchors {
		row := a.Quote
		if row == "" || strings.TrimSpace(a.Answer) == "" {
			continue
		}
		if _, seen := filled[row]; !seen {
			filled[row] = map[string]bool{}
			order = append(order, row)
		}
		filled[row][a.Dimension] = true
	}
	completeRows := 0
	firstIncomplete := ""
	haveIncomplete := false
	for _, row := range order {
		done := true
		for _, col := range spec.Params.Cols {
			if !filled[row][col.ID] {
				done = false
				break
			}
		}
		if done {
			completeRows++
			continue
		}
		if !haveIncomplete {
			firstIncomplete = row
			haveIncomplete = true
		}
	}
	if completeRows >= spec.Params.MinItems {
		return "", false
	}
	if haveIncomplete {
		return firstIncomplete, true
	}
	return "rows", true
}

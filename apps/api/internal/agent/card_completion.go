package agent

import (
	"regexp"
	"strconv"
	"strings"

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
			if len(a.Answer) >= n {
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

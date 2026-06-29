package agent

import "regexp"

type CardSignal struct {
	CardID       string `json:"card_id"`
	Status       string `json:"status"`
	OpCount      int    `json:"op_count"`
	FilledFields int    `json:"filled_fields"`
	EmptyFields  int    `json:"empty_fields"`
}

type EvalSignals struct {
	MessageCount            int          `json:"message_count"`
	StudentTurnCount        int          `json:"student_turn_count"`
	AssistantTurnCount      int          `json:"assistant_turn_count"`
	StudentCharCount        int          `json:"student_char_count"`
	SourceCountStudent      int          `json:"source_count_student"`
	SourceCountAI           int          `json:"source_count_ai"`
	MaxVerbatimOverlapChars int          `json:"max_verbatim_overlap_chars"`
	Cards                   []CardSignal `json:"cards"`
	NACandidates            []string     `json:"na_candidates"`
}

var urlRe = regexp.MustCompile(`https?://[^\s,，。;；)）】]+`)

func distinctURLs(texts []string) int {
	seen := map[string]struct{}{}
	for _, t := range texts {
		for _, u := range urlRe.FindAllString(t, -1) {
			seen[u] = struct{}{}
		}
	}
	return len(seen)
}

// ComputeSignals derives deterministic HARD facts from the transcript + card
// envelopes. No semantic judgement — that stays with the eval LLM. Verbatim
// overlap (Task 3) and N/A-candidates (Task 4) are filled by later passes.
func ComputeSignals(msgs []StoredMessage, cardInsts []CardInstance) EvalSignals {
	var s EvalSignals
	var studentTexts, aiTexts []string
	for _, m := range msgs {
		s.MessageCount++
		switch m.Role {
		case "user":
			s.StudentTurnCount++
			s.StudentCharCount += len([]rune(m.Content))
			studentTexts = append(studentTexts, m.Content)
		case "assistant":
			s.AssistantTurnCount++
			aiTexts = append(aiTexts, m.Content)
		}
	}
	s.SourceCountStudent = distinctURLs(studentTexts)
	s.SourceCountAI = distinctURLs(aiTexts)

	for _, st := range studentTexts {
		for _, ai := range aiTexts {
			if n := longestCommonSubstring([]rune(st), []rune(ai)); n > s.MaxVerbatimOverlapChars {
				s.MaxVerbatimOverlapChars = n
			}
		}
	}

	s.Cards = make([]CardSignal, 0, len(cardInsts))
	for _, c := range cardInsts {
		cs := CardSignal{CardID: c.CardID, Status: c.Status, OpCount: c.EventTraceLen}
		for _, step := range c.FieldValues {
			for _, v := range step {
				if isEmptyValue(v) {
					cs.EmptyFields++
				} else {
					cs.FilledFields++
				}
			}
		}
		s.Cards = append(s.Cards, cs)
	}
	return s
}

func isEmptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return false
	}
}

// longestCommonSubstring returns the rune length of the longest contiguous
// run shared by a and b. Each text is capped at maxLCSRunes to bound cost;
// copy detection only needs to know a long verbatim run exists, not its exact
// length past the cap.
const maxLCSRunes = 4000

func longestCommonSubstring(a, b []rune) int {
	if len(a) > maxLCSRunes {
		a = a[:maxLCSRunes]
	}
	if len(b) > maxLCSRunes {
		b = b[:maxLCSRunes]
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	best := 0
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
				if cur[j] > best {
					best = cur[j]
				}
			} else {
				cur[j] = 0
			}
		}
		prev, cur = cur, prev
		for k := range cur {
			cur[k] = 0
		}
	}
	return best
}

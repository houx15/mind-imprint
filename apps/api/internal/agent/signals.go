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

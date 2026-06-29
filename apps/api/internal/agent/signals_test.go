package agent

import "testing"

func TestComputeSignals_CountsAndSources(t *testing.T) {
	msgs := []StoredMessage{
		{Role: "user", Content: "看看 https://nasa.gov/x 和 https://nasa.gov/x 还有 https://nature.com/y"},
		{Role: "assistant", Content: "我找到一个来源 https://wikipedia.org/z"},
		{Role: "user", Content: "好的"},
		{Role: "system", Content: "ignored"},
	}
	cards := []CardInstance{
		{CardID: "sift", Status: "completed", EventTraceLen: 5,
			FieldValues: map[string]map[string]any{"step1": {"a": "filled", "b": "", "c": "x"}}},
	}
	s := ComputeSignals(msgs, cards)

	if s.MessageCount != 4 {
		t.Errorf("MessageCount=%d want 4", s.MessageCount)
	}
	if s.StudentTurnCount != 2 || s.AssistantTurnCount != 1 {
		t.Errorf("turns student=%d assistant=%d want 2/1", s.StudentTurnCount, s.AssistantTurnCount)
	}
	if s.SourceCountStudent != 2 { // distinct: nasa.gov/x, nature.com/y
		t.Errorf("SourceCountStudent=%d want 2", s.SourceCountStudent)
	}
	if s.SourceCountAI != 1 {
		t.Errorf("SourceCountAI=%d want 1", s.SourceCountAI)
	}
	if len(s.Cards) != 1 || s.Cards[0].OpCount != 5 ||
		s.Cards[0].FilledFields != 2 || s.Cards[0].EmptyFields != 1 {
		t.Errorf("card signal wrong: %+v", s.Cards)
	}
}

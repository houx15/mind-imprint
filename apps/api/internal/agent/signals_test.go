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

func TestComputeSignals_VerbatimOverlap(t *testing.T) {
	shared := "中国太阳能装机容量全球第一这是一个足够长的片段"
	msgs := []StoredMessage{
		{Role: "assistant", Content: "根据资料，" + shared + "，你可以参考。"},
		{Role: "user", Content: "我直接用：" + shared},
	}
	s := ComputeSignals(msgs, nil)
	if s.MaxVerbatimOverlapChars != len([]rune(shared)) {
		t.Errorf("MaxVerbatimOverlapChars=%d want %d", s.MaxVerbatimOverlapChars, len([]rune(shared)))
	}
}

func TestComputeSignals_NoOverlap(t *testing.T) {
	msgs := []StoredMessage{
		{Role: "assistant", Content: "abcdefg"},
		{Role: "user", Content: "完全不同的内容"},
	}
	s := ComputeSignals(msgs, nil)
	if s.MaxVerbatimOverlapChars > 1 {
		t.Errorf("MaxVerbatimOverlapChars=%d want ~0", s.MaxVerbatimOverlapChars)
	}
}

func TestComputeSignals_NACandidates_Empty(t *testing.T) {
	// One short student message, no sources, no AI, no cards.
	s := ComputeSignals([]StoredMessage{{Role: "user", Content: "hi"}}, nil)
	got := map[string]bool{}
	for _, d := range s.NACandidates {
		got[d] = true
	}
	for _, want := range []string{"D2", "D3", "D7", "D8", "D9"} {
		if !got[want] {
			t.Errorf("expected %s in NACandidates, got %v", want, s.NACandidates)
		}
	}
	if got["D1"] || got["D4"] || got["D10"] {
		t.Errorf("LLM-only dims must not be rule-marked N/A: %v", s.NACandidates)
	}
}

func TestComputeSignals_NACandidates_RichTask(t *testing.T) {
	long := "我认为中国在可再生能源上领先，因为装机量数据支持这一点，而且这段足够长以构成实质论证内容。"
	msgs := []StoredMessage{
		{Role: "user", Content: "看 https://nasa.gov/a 与 https://nature.com/b ：" + long},
		{Role: "assistant", Content: "可以参考这两个来源。"},
	}
	s := ComputeSignals(msgs, nil)
	for _, dim := range s.NACandidates {
		if dim == "D2" || dim == "D3" || dim == "D7" || dim == "D9" {
			t.Errorf("did not expect %s N/A-candidate in a rich task: %v", dim, s.NACandidates)
		}
	}
}

package agent

import (
	"os"
	"testing"

	"mindimprint/api/internal/cards"
)

func TestEvalInputGoldenParity(t *testing.T) {
	want, err := os.ReadFile("testdata/eval_input.txt")
	if err != nil {
		t.Fatal(err)
	}

	catalog, err := cards.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	idx := map[string]cards.Spec{}
	for _, s := range catalog {
		idx[s.ID] = s
	}
	specByID := func(id string) (cards.Spec, bool) { s, ok := idx[id]; return s, ok }

	msgs := []StoredMessage{
		{Role: "user", Content: "我想引用这篇公众号文章"},
		{Role: "assistant", Content: "先一起核查来源吧", ToolCall: &SummonCardCall{ID: "tc1", Name: "summon_card", Args: SummonCardArgs{CardID: "sift_craap", Reason: "r", NudgeText: "n"}, CardInstanceID: "ci_1"}},
	}
	cardsIn := []CardInstance{
		{
			ID:          "ci_1",
			CardID:      "sift_craap",
			TaskID:      "t_1",
			Status:      "completed",
			FieldValues: map[string]map[string]any{"sift": {"stop": "证明中国让地球更可持续"}},
		},
	}
	got := AssembleEvalInput(msgs, cardsIn, specByID)
	if got != string(want) {
		t.Errorf("eval input drift from TS golden:\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

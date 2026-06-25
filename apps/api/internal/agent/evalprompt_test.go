package agent

import (
	"os"
	"testing"
)

func TestEvalPromptGoldenParity(t *testing.T) {
	want, err := os.ReadFile("testdata/eval_prompt_full.txt")
	if err != nil {
		t.Fatal(err)
	}
	got := BuildEvalPrompt(FullRubric)
	if got != string(want) {
		t.Errorf("eval prompt drift from TS golden (len got=%d want=%d)", len(got), len(want))
	}
}

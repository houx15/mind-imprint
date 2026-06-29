package agent

import (
	"os"
	"strings"
	"testing"

	"mindimprint/api/internal/cards"
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

func TestBuildEvalPrompt_HasNAAndContract(t *testing.T) {
	p := BuildEvalPrompt(FullRubric)
	for _, want := range []string{"N/A", "客观信号", "D10"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(p, "给 L1 并在 note 里说明") {
		t.Errorf("prompt still tells the model to give L1 on silence")
	}
}

func TestBuildEvalUserInput_AppendsSignals(t *testing.T) {
	sig := EvalSignals{SourceCountStudent: 2, NACandidates: []string{"D9"}}
	out := BuildEvalUserInput(
		[]StoredMessage{{Role: "user", Content: "hi"}}, nil, sig,
		func(string) (cards.Spec, bool) { return cards.Spec{}, false },
	)
	if !strings.Contains(out, "## 客观信号") {
		t.Errorf("missing 客观信号 block")
	}
	if !strings.Contains(out, "source_count_student") {
		t.Errorf("missing serialized signals")
	}
}

func TestRubricVersionConstant(t *testing.T) {
	if RubricVersion != "cognitive-model-v2" {
		t.Errorf("RubricVersion=%q", RubricVersion)
	}
}

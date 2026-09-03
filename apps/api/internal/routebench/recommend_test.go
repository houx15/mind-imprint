package routebench

import (
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
)

// cell builds one (case × model) result with a fixed cost and a judge score.
func cell(caseID, class, model string, judge float64, outTokens int) Result {
	return Result{
		CaseID:  caseID,
		Class:   class,
		ModelID: model,
		Judge:   judge,
		Samples: []Sample{{
			Total: time.Second,
			Out:   outTokens,
			Text:  "x",
			Valid: true,
		}},
	}
}

// The real 2026-09-03 dialogue numbers. deepseek-v4-pro is worse on three of the
// four surfaces and better on the one that decides: the lite reading coach,
// where waving a student past an unfinished step is the failure this class
// exists to avoid.
//
// The first version of Recommend picked qwen3.8-max here — it was inside the
// one-point band and used fewer tokens. A cheaper model that is strictly worse
// where it matters must never win, so cost may only break a TIE.
func TestCostNeverOverridesAWorseWorstCase(t *testing.T) {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	const cls = gateway.ClassDialogue
	results := []Result{
		cell("dialogue/pro-coach", cls, "dashscope/deepseek-v4-pro", 4, 29),
		cell("dialogue/lite-writing", cls, "dashscope/deepseek-v4-pro", 5, 24),
		cell("dialogue/pbl-turn", cls, "dashscope/deepseek-v4-pro", 5, 55),
		cell("dialogue/lite-reading", cls, "dashscope/deepseek-v4-pro", 3, 103),

		cell("dialogue/pro-coach", cls, "dashscope/qwen3.8-max", 3, 37),
		cell("dialogue/lite-writing", cls, "dashscope/qwen3.8-max", 5, 29),
		cell("dialogue/pbl-turn", cls, "dashscope/qwen3.8-max", 5, 47),
		cell("dialogue/lite-reading", cls, "dashscope/qwen3.8-max", 2, 73),
	}

	for _, rec := range Recommend(results, cat) {
		if rec.Class != cls {
			continue
		}
		if rec.ModelID != "dashscope/deepseek-v4-pro" {
			t.Fatalf("dialogue = %q, want deepseek-v4-pro: it wins the worst case 3 vs 2, "+
				"and being cheaper must not buy a worse floor", rec.ModelID)
		}
		return
	}
	t.Fatal("no dialogue recommendation produced")
}

// On review/assess a tie is not evidence to move. Every candidate scoring the
// same means the RUBRIC is not discriminating (on 2026-09-03: a prompt bug that
// made all four judge a half-baked framework "ready") — switching on cost there
// hands a reasoning class to whichever model reasons least.
func TestQualityTieKeepsTheIncumbentOnReview(t *testing.T) {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	const cls = gateway.ClassReview
	incumbent := cat.Lanes[cls].Model
	if incumbent == "" {
		t.Fatal("review must be bound for this test to mean anything")
	}
	results := []Result{
		cell("review/framework", cls, incumbent, 2, 491),
		cell("review/framework", cls, "dashscope/glm-5.3", 2, 181),
	}
	for _, rec := range Recommend(results, cat) {
		if rec.Class != cls {
			continue
		}
		if rec.ModelID != incumbent {
			t.Fatalf("review = %q, want the incumbent %q kept: a tie is not evidence", rec.ModelID, incumbent)
		}
		return
	}
	t.Fatal("no review recommendation produced")
}

// A tie on a class where cost is a legitimate deciding axis SHOULD move. compose
// is the case: all three candidates scored 4, and glm-5.3 is 4.1s against
// kimi-k3's 10.3s. Without this, the fix above would freeze every binding.
func TestQualityTieStillSwitchesOnCostWhereCostDecides(t *testing.T) {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	const cls = gateway.ClassCompose
	results := []Result{
		cell("compose/reading-router", cls, "dashscope/kimi-k3", 4, 222),
		cell("compose/reading-router", cls, "dashscope/glm-5.3", 4, 189),
	}
	for _, rec := range Recommend(results, cat) {
		if rec.Class != cls {
			continue
		}
		if rec.ModelID != "dashscope/glm-5.3" {
			t.Fatalf("compose = %q, want glm-5.3: equal quality, fewer tokens", rec.ModelID)
		}
		return
	}
	t.Fatal("no compose recommendation produced")
}

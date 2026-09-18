package routebench

import (
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
)

// cell builds one (case × model) result with a fixed cost and a judge score.
func cell(caseID, class, model string, judge float64, outTokens int) Result {
	return timedCell(caseID, class, model, judge, outTokens, time.Second)
}

func timedCell(caseID, class, model string, judge float64, outTokens int, total time.Duration) Result {
	return Result{
		CaseID:  caseID,
		Class:   class,
		ModelID: model,
		Judge:   judge,
		Samples: []Sample{{
			Total:    total,
			Out:      outTokens,
			Text:     "x",
			ParseErr: "n/a",
			Valid:    true,
		}},
	}
}

func goldCell(caseID, class, model string, judge float64, gold ...bool) Result {
	samples := make([]Sample, 0, len(gold))
	for _, ok := range gold {
		s := Sample{Total: time.Second, Text: "x", ParseErr: "n/a", Valid: true, Gold: ok}
		if !ok {
			s.GoldErr = "ready = true, want false"
		}
		samples = append(samples, s)
	}
	return Result{CaseID: caseID, Class: class, ModelID: model, Judge: judge, Samples: samples}
}

func TestGoldRateDistinguishesNoCheckFromFailure(t *testing.T) {
	if got := (Result{Samples: []Sample{{Valid: true}}}).GoldRate(); got != -1 {
		t.Fatalf("no GoldCheck rate = %v, want -1", got)
	}
	if got := (Result{Samples: []Sample{{Valid: true, Gold: true}, {Valid: true, Gold: true}, {Valid: true, GoldErr: "wrong verdict"}}}).GoldRate(); got != 2.0/3.0 {
		t.Fatalf("gold rate = %v, want 2/3", got)
	}
}

func TestGoldFailureRejectsCandidateAndIsReportedSeparately(t *testing.T) {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	const cls = gateway.ClassReview
	incumbent := cat.Lanes[cls].Model
	results := []Result{
		goldCell("review/framework-review", cls, incumbent, 5, true, true, true),
		goldCell("review/framework-review", cls, "dashscope/qwen3.8-max", 5, true, false, true),
	}
	for _, rec := range Recommend(results, cat) {
		if rec.Class != cls {
			continue
		}
		if rec.ModelID != incumbent {
			t.Fatalf("review = %q, want gold-passing incumbent %q", rec.ModelID, incumbent)
		}
		if len(rec.Rejected) != 1 || !strings.Contains(rec.Rejected[0], "gold expectation missed on 33%") {
			t.Fatalf("gold failure rejection = %#v", rec.Rejected)
		}
		md := Markdown(results, cat, Config{Samples: 3}, time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC))
		if !strings.Contains(md, "任务命中") || !strings.Contains(md, "任务未命中：ready = true, want false") {
			t.Fatalf("markdown does not distinguish gold failure:\n%s", md)
		}
		return
	}
	t.Fatal("no review recommendation produced")
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

// Equal quality is not a complete tie: the documented ordering is quality,
// speed, then token count, including for review and assess.
func TestSpeedBreaksQualityTieOnReview(t *testing.T) {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	const cls = gateway.ClassReview
	incumbent := cat.Lanes[cls].Model
	if incumbent == "" {
		t.Fatal("review must be bound for this test to mean anything")
	}
	challenger := "dashscope/qwen3.8-max"
	if challenger == incumbent {
		challenger = "dashscope/deepseek-v4-pro"
	}
	results := []Result{
		timedCell("review/framework", cls, incumbent, 5, 200, 2*time.Second),
		timedCell("review/framework", cls, challenger, 5, 200, time.Second),
	}
	for _, rec := range Recommend(results, cat) {
		if rec.Class != cls {
			continue
		}
		if rec.ModelID != challenger {
			t.Fatalf("review = %q, want faster challenger %q after quality tie", rec.ModelID, challenger)
		}
		return
	}
	t.Fatal("no review recommendation produced")
}

func TestExactMetricTieKeepsTheIncumbent(t *testing.T) {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	const cls = gateway.ClassReview
	incumbent := cat.Lanes[cls].Model
	challenger := "dashscope/qwen3.8-max"
	if challenger == incumbent {
		challenger = "dashscope/deepseek-v4-pro"
	}
	results := []Result{
		cell("review/framework", cls, incumbent, 5, 200),
		cell("review/framework", cls, challenger, 5, 200),
	}
	for _, rec := range Recommend(results, cat) {
		if rec.Class != cls {
			continue
		}
		if rec.ModelID != incumbent {
			t.Fatalf("review = %q, want exactly tied incumbent %q", rec.ModelID, incumbent)
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

func TestPartialJudgeFailureDisqualifiesCandidate(t *testing.T) {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	r := cell("dialogue/reading", gateway.ClassDialogue, "dashscope/deepseek-v4-pro", 4, 10)
	r.Samples = append(r.Samples, Sample{
		Text: "y", ParseErr: "n/a", Valid: true,
		JudgeWhy: "judge failed after 2 attempts: returned an empty why",
	})
	for _, rec := range Recommend([]Result{r}, cat) {
		if rec.Class == gateway.ClassDialogue {
			if rec.ModelID != "" || len(rec.Rejected) == 0 {
				t.Fatalf("partial judge failure must reject candidate: %+v", rec)
			}
			return
		}
	}
	t.Fatal("no dialogue recommendation produced")
}

package agent

import (
	"strings"
	"testing"

	"mindimprint/api/internal/cards"
)

// The lens verdict panel prints this prompt's output on the STUDENT's own
// screen. It used to ask for 「她这句读出了什么」, so the model wrote
// 「她读出了幼鸟靠本能…」 — reporting on her, in the third person, while she
// watched. The product owner caught it in a live session on 2026-08-30.
//
// This pins the voice at the only place it can be pinned: the instruction. We
// cannot assert on model output, but we can assert we never again ASK for the
// third person.
func TestBuildEvalPrompt_SpeaksToHer(t *testing.T) {
	spec := cards.Spec{Name: "逻辑学：推理有没有跳步？"}
	got := buildEvalPrompt(spec, "target")

	// The rule itself must be stated, not merely implied by the field examples —
	// a model that ignores one example still has the rule to fall back on.
	if !strings.Contains(got, "称呼学生时使用「你」") {
		t.Errorf("prompt no longer states the second-person rule:\n%s", got)
	}

	// The field templates are what the model copies most literally, so they are
	// where a regression would actually land.
	for _, want := range []string{`"finding":""`, `"judgment":""`} {
		if !strings.Contains(got, want) {
			t.Errorf("field template %q missing — did it revert to 她?\n%s", want, got)
		}
	}

	// The old wording, exactly. Kept as a literal so this test names the defect
	// it exists to prevent.
	for _, forbidden := range []string{"她这句读出了什么", "她的论断"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("prompt asks for third-person output: %q", forbidden)
		}
	}
}

// fallbackEval runs when the model is unavailable, and its text reaches the same
// panel. It was already second person before the fix above — the degraded path
// was more correct than the live one — so this guards it from drifting the other
// way now that the two agree.
func TestFallbackEval_SpeaksToHer(t *testing.T) {
	ev := fallbackEval(Anchor{ID: "sp1", Quote: "天上既没有路牌，也没有人给它们指路。"})

	texts := []string{ev.Finding, ev.NextStep, ev.VerdictReason}
	for _, c := range ev.Checks {
		texts = append(texts, c.Explanation)
	}

	for _, s := range texts {
		if strings.Contains(s, "她") {
			t.Errorf("fallback text speaks about her in the third person: %q", s)
		}
	}
}

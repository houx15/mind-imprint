package api

import (
	"context"
	"errors"
	"testing"

	"mindimprint/api/internal/gateway"
)

// route()'s fallback is the kind of thing that reads as harmless and is not.
// These pin the two rules that matter.

func depsWithClasses(available map[string]string) Deps {
	return Deps{Route: func(class string) gateway.KeyResolver {
		return func(context.Context) (gateway.Resolved, error) {
			if id, ok := available[class]; ok {
				return gateway.Resolved{ModelID: id, Tier: "x"}, nil
			}
			return gateway.Resolved{}, errors.New("no LLM provider configured")
		}
	}}
}

// 过程评估走旗舰模型绝不降级. The catalog refuses a non-flagship binding at boot;
// it would be absurd for this helper to hand back at runtime what the catalog
// refused at startup. The pre-class resolveEval DID fall through to the
// chaperone here — a hole that only opens in a half-configured environment,
// which is precisely where it does real damage: a student's process quietly
// graded on the cheap model.
func TestAssessNeverFallsBackToAnotherClass(t *testing.T) {
	a := &API{d: depsWithClasses(map[string]string{
		gateway.ClassDialogue: "cheap/model",
		// assess deliberately absent
	})}
	if r, ok := a.route(context.Background(), gateway.ClassAssess); ok {
		t.Fatalf("assess fell back to %q — 过程评估绝不降级", r.ModelID)
	}
}

// Every other class does retry on dialogue, so a dev box holding a single key
// still answers instead of failing every feature at once.
func TestOtherClassesFallBackToDialogue(t *testing.T) {
	a := &API{d: depsWithClasses(map[string]string{gateway.ClassDialogue: "the/dialogue-model"})}
	for _, class := range []string{gateway.ClassCompose, gateway.ClassReview, gateway.ClassDigest, gateway.ClassReflex} {
		r, ok := a.route(context.Background(), class)
		if !ok {
			t.Errorf("%s did not fall back, so a single-key environment loses this feature", class)
			continue
		}
		if r.ModelID != "the/dialogue-model" {
			t.Errorf("%s resolved to %q, want the dialogue fallback", class, r.ModelID)
		}
	}
}

// A class that IS configured must use its own binding, never the fallback —
// otherwise every class silently collapses onto dialogue and the whole
// taxonomy is decorative.
func TestAConfiguredClassUsesItsOwnBinding(t *testing.T) {
	a := &API{d: depsWithClasses(map[string]string{
		gateway.ClassDialogue: "the/dialogue-model",
		gateway.ClassReview:   "the/review-model",
	})}
	r, ok := a.route(context.Background(), gateway.ClassReview)
	if !ok || r.ModelID != "the/review-model" {
		t.Fatalf("review resolved to %q (ok=%v), want its own binding", r.ModelID, ok)
	}
}

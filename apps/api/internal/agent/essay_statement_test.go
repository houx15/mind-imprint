package agent

import "testing"

func TestDeriveStatementSteps_NoSubQuestions(t *testing.T) {
	steps := DeriveStatementSteps(nil)
	// outline + synthesis + conclusion + structure + challenges = 5 (反方观点 LAST).
	if len(steps) != 5 {
		t.Fatalf("want 5 steps, got %d: %+v", len(steps), steps)
	}
	if steps[0].Key != "outline" || steps[4].Key != "challenges" {
		t.Fatalf("order wrong (反方观点 must be last): %+v", steps)
	}
	// 反方观点 is a dedicated step.
	found := false
	for _, s := range steps {
		if s.Key == "challenges" {
			found = true
		}
	}
	if !found {
		t.Fatal("challenges (反方观点) must be a dedicated step")
	}
}

func TestDeriveStatementSteps_PerClaim(t *testing.T) {
	steps := DeriveStatementSteps([]SubQuestion{{ID: "a", Text: "q1"}, {ID: "b", Text: "q2"}})
	// outline + 2 claims + 4 tail = 7
	if len(steps) != 7 {
		t.Fatalf("want 7 steps, got %d", len(steps))
	}
	if steps[1].Key != "claim:a" || steps[1].Kind != KindSubq || steps[1].SubQuestionID != "a" {
		t.Fatalf("claim step wrong: %+v", steps[1])
	}
	if steps[2].Key != "claim:b" {
		t.Fatalf("second claim wrong: %+v", steps[2])
	}
	if steps[3].Key != "synthesis" {
		t.Fatalf("tail should follow claims: %+v", steps[3])
	}
}

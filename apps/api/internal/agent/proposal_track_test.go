package agent

import "testing"

func TestDeriveProposalSteps_NoSubQuestions(t *testing.T) {
	steps := DeriveProposalSteps(WritingTrack{})
	// 3 intro + research-plan(define) + 5 tail + polish = 10 steps, no subq cards yet.
	if len(steps) != 10 {
		t.Fatalf("want 10 steps, got %d", len(steps))
	}
	if steps[0].Key != "understanding" || steps[2].Key != "thesis" {
		t.Fatalf("intro order wrong: %+v", steps[:3])
	}
	if steps[3].Key != "research-plan" || steps[3].Kind != KindSubqDefine {
		t.Fatalf("step 3 must be the subq-define step, got %+v", steps[3])
	}
	if steps[4].Key != "resources" || steps[8].Key != "expected" || steps[9].Key != "polish" {
		t.Fatalf("tail order wrong: %+v", steps[4:])
	}
	// every step carries a non-empty title.
	for i, s := range steps {
		if s.Title == "" {
			t.Fatalf("step %d (%s) has empty title", i, s.Key)
		}
	}
}

func TestDeriveProposalSteps_ExpandsPerSubQuestion(t *testing.T) {
	track := WritingTrack{SubQuestions: []SubQuestion{{ID: "a", Text: "q1"}, {ID: "b", Text: "q2"}, {ID: "c", Text: "q3"}}}
	steps := DeriveProposalSteps(track)
	if len(steps) != 13 { // 10 base + 3 subq cards
		t.Fatalf("want 13 steps for 3 sub-questions, got %d", len(steps))
	}
	// subq cards sit between research-plan (index 3) and resources.
	if steps[4].Kind != KindSubq || steps[4].SubQuestionID != "a" {
		t.Fatalf("first subq card wrong: %+v", steps[4])
	}
	if steps[6].SubQuestionID != "c" {
		t.Fatalf("third subq card wrong: %+v", steps[6])
	}
	if steps[4].Key != "subq:a" {
		t.Fatalf("subq step key wrong: %+v", steps[4])
	}
	if steps[7].Key != "resources" {
		t.Fatalf("resources must follow the subq cards, got %+v", steps[7])
	}
}

func TestProposalFixedParts_GoldenOrder(t *testing.T) {
	parts := ProposalFixedParts()
	want := []string{"understanding", "question-scope", "thesis", "research-plan", "resources", "challenges", "method", "feasibility", "expected", "polish"}
	if len(parts) != len(want) {
		t.Fatalf("want %d fixed parts, got %d", len(want), len(parts))
	}
	for i, k := range want {
		if parts[i].Key != k {
			t.Fatalf("fixed part %d: want %s, got %s", i, k, parts[i].Key)
		}
	}
	if parts[3].Kind != KindSubqDefine {
		t.Fatalf("research-plan must be the subq-define step, got %v", parts[3].Kind)
	}
}

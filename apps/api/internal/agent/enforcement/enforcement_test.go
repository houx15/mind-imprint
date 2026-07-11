package enforcement

import "testing"

func stubSim(x float64) Similarity {
	return stubSimilarity{value: x}
}

type stubSimilarity struct {
	value float64
}

func (s stubSimilarity) Cosine(a, b string) float64 {
	return s.value
}

func TestValidateOutput_ReferenceNeedsProvenance(t *testing.T) {
	err := ValidateOutput(AgentOutput{Type: "reference", Quote: "x"}) // no provenance
	if err == nil {
		t.Fatal("reference without provenance must be rejected")
	}
}

func TestValidateOutput_QuestionRequiresAnchorCriterionBody(t *testing.T) {
	good := AgentOutput{Type: "question", Anchor: OutputAnchor{Kind: "graph_node", ID: "n1"},
		Criterion: "D4", Body: "Who holds the opposing view?"}
	if err := ValidateOutput(good); err != nil {
		t.Fatalf("well-formed question must pass: %v", err)
	}
	for name, out := range map[string]AgentOutput{
		"no anchor":    {Type: "question", Criterion: "D4", Body: "b"},
		"no criterion": {Type: "question", Anchor: OutputAnchor{ID: "n1"}, Body: "b"},
		"no body":      {Type: "question", Anchor: OutputAnchor{ID: "n1"}, Criterion: "D4"},
	} {
		if err := ValidateOutput(out); err == nil {
			t.Fatalf("%s: question must be rejected", name)
		}
	}
}

func TestValidateOutput_ReferenceRequiresAnchorAndProvenance(t *testing.T) {
	ok := AgentOutput{Type: "reference", Anchor: OutputAnchor{Kind: "artifact", ID: "a1"},
		Quote: "my earlier claim", Provenance: "artifact:a1"}
	if err := ValidateOutput(ok); err != nil {
		t.Fatalf("well-formed reference must pass: %v", err)
	}
	if err := ValidateOutput(AgentOutput{Type: "reference", Provenance: "artifact:a1"}); err == nil {
		t.Fatal("reference without an anchor must be rejected")
	}
}

func TestOutputCheck_ThresholdOverride(t *testing.T) {
	// A 0.85 similarity passes under a raised 0.9 threshold but intercepts at default 0.8.
	text := "China's transition makes the planet more sustainable."
	if v := OutputCheck(text, Context{Topic: "China sustainability", Threshold: 0.9}, stubSim(0.85)); v.Verdict != "pass" {
		t.Fatalf("raised threshold should pass, got %s", v.Verdict)
	}
	if v := OutputCheck(text, Context{Topic: "China sustainability"}, stubSim(0.85)); v.Verdict != "intercept" {
		t.Fatalf("default threshold should intercept, got %s", v.Verdict)
	}
}

func TestGuardStudentField_RejectsNonStudent(t *testing.T) {
	if err := GuardStudentField("warrant", "ai"); err == nil {
		t.Fatal("ai author must be rejected on a student field")
	}
	if err := GuardStudentField("warrant", "student"); err != nil {
		t.Fatalf("student must pass: %v", err)
	}
}

func TestBannedPhrasing_FlagsUnanchoredQuestion(t *testing.T) {
	if BannedPhrasing("Have you considered other angles?") == nil {
		t.Fatal("banned phrase must be flagged")
	}
}

func TestOutputCheck_InterceptsDeclarativeEcho(t *testing.T) {
	sim := stubSim(0.95)
	v := OutputCheck("China's transition makes the planet more sustainable.",
		Context{Topic: "is China making the planet more sustainable"}, sim)
	if v.Verdict != "intercept" {
		t.Fatalf("expected intercept, got %s", v.Verdict)
	}
}

func TestOutputCheck_CrossDomainExamplePasses(t *testing.T) {
	v := OutputCheck("In chemistry, a catalyst speeds a reaction without being consumed.",
		Context{Topic: "China sustainability", CrossDomainExample: true}, stubSim(0.99))
	if v.Verdict != "pass" {
		t.Fatal("cross-domain example is exempt")
	}
}

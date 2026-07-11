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

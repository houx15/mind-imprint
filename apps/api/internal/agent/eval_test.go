package agent

import (
	"testing"
)

func TestFullRubricHasTenDimensions(t *testing.T) {
	if len(FullRubric) != 10 {
		t.Fatalf("FullRubric has %d dims, want 10", len(FullRubric))
	}
	ids := map[string]bool{}
	for _, d := range FullRubric {
		ids[d.ID] = true
	}
	if !ids["D10"] {
		t.Errorf("missing D10 协作编排")
	}
}

func TestParseEvalOutput_AcceptsNA(t *testing.T) {
	in := `{"scores":[{"dim_id":"D2","level":"NA","note":"本次未涉及外部来源"}],"narrative":"n"}`
	out, err := parseEvalOutput(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.Scores[0].Level != "NA" {
		t.Errorf("level=%q want NA", out.Scores[0].Level)
	}
}

func TestParseEvalOutput_RejectsUnknownLevel(t *testing.T) {
	in := `{"scores":[{"dim_id":"D2","level":"L9","note":"x"}],"narrative":"n"}`
	if _, err := parseEvalOutput(in); err == nil {
		t.Fatalf("expected error on unknown level")
	}
}

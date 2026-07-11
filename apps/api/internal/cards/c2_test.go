package cards

import "testing"

// TestC2FieldsOnCRAAP proves the Go loader exposes the C2 card fields
// (primitive/target_type/params/completion/graph_effects/observe/
// consolidation) authored onto packages/contracts/cards/craap.json, while
// the legacy `steps` stay intact for the old renderer.
func TestC2FieldsOnCRAAP(t *testing.T) {
	s, ok := ByID("craap")
	if !ok {
		t.Fatal("craap not found")
	}

	if s.Primitive != "annotate" {
		t.Fatalf("primitive = %q, want annotate", s.Primitive)
	}

	wantTags := []string{"currency", "relevance", "authority", "accuracy", "purpose"}
	if len(s.Params.Tags) != len(wantTags) {
		t.Fatalf("params.tags = %v, want %v", s.Params.Tags, wantTags)
	}
	for i, tag := range wantTags {
		if s.Params.Tags[i] != tag {
			t.Fatalf("params.tags[%d] = %q, want %q", i, s.Params.Tags[i], tag)
		}
	}
	if s.Params.TagPrompts["authority"] == "" {
		t.Fatal("params.tag_prompts[authority] is empty")
	}

	var hasEveryTag, hasFieldWritten bool
	for _, c := range s.Completion {
		if c.Kind == "every_tag_present" && len(c.Tags) > 0 {
			hasEveryTag = true
		}
		if c.Kind == "field_written_by" && c.Field == "risk_note" && c.Author == "student" {
			hasFieldWritten = true
		}
	}
	if !hasEveryTag {
		t.Fatal("missing every_tag_present completion predicate")
	}
	if !hasFieldWritten {
		t.Fatal("missing field_written_by(risk_note, student) completion predicate")
	}

	var hasPromote bool
	for _, ge := range s.GraphEffects {
		if ge.Kind == "promote" && ge.From == "material" && ge.To == "evidence" {
			hasPromote = true
		}
	}
	if !hasPromote {
		t.Fatal("missing promote graph_effect (material -> evidence)")
	}

	if len(s.Observe) < 1 {
		t.Fatal("missing observe rules")
	}

	if s.Consolidation == "" {
		t.Fatal("consolidation is empty")
	}

	// steps (legacy form path) stay intact.
	if len(s.Steps) == 0 {
		t.Fatal("steps should still be present (legacy form path)")
	}
}

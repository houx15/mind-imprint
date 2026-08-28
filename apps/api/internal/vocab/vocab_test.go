package vocab

import (
	"bytes"
	"os"
	"testing"
)

// The library is the ONLY place 印记 may get a method name from, so a malformed
// entry is a product defect, not a runtime inconvenience — it must fail at load.
func TestLoad_EveryMethodIsUsable(t *testing.T) {
	all := All()
	if len(all) < 10 {
		t.Fatalf("library has %d methods, want at least 10", len(all))
	}
	seen := map[string]bool{}
	for _, m := range all {
		if m.ID == "" || m.Name == "" || m.Definition == "" {
			t.Errorf("method %+v is missing id, name or definition", m)
		}
		if seen[m.ID] {
			t.Errorf("duplicate method id %q", m.ID)
		}
		seen[m.ID] = true
		switch m.AppliesTo {
		case "opening", "body", "closing", "any":
		default:
			t.Errorf("method %q has applies_to %q, want opening|body|closing|any", m.ID, m.AppliesTo)
		}
		// Borrowed material by construction: an example with no topic cannot be
		// checked for being about someone else's subject.
		for _, ex := range m.Examples {
			if ex.Topic == "" || ex.Text == "" {
				t.Errorf("method %q has an example missing topic or text", m.ID)
			}
		}
		for _, p := range m.Patterns {
			if p.Frame == "" {
				t.Errorf("method %q has a pattern with no frame", m.ID)
			}
		}
		if len(m.Examples) == 0 && len(m.Patterns) == 0 {
			t.Errorf("method %q teaches nothing: no examples and no patterns", m.ID)
		}
	}
}

func TestByID_AndFor(t *testing.T) {
	if _, ok := ByID("point_concession"); !ok {
		t.Fatal(`ByID("point_concession") not found`)
	}
	if _, ok := ByID("no_such_method"); ok {
		t.Fatal("ByID returned ok for an unknown id")
	}
	openings := For("opening")
	if len(openings) == 0 {
		t.Fatal(`For("opening") returned nothing`)
	}
	for _, m := range openings {
		if m.AppliesTo != "opening" && m.AppliesTo != "any" {
			t.Errorf("For(\"opening\") returned %q with applies_to %q", m.ID, m.AppliesTo)
		}
	}
}

// TestEmbeddedCopyMatchesSourceOfTruth guards the fork forced by go:embed's
// inability to escape the package directory: packages/contracts/vocab/methods.json
// is the editable source of truth, apps/api/internal/vocab/methods.json is the
// embedded copy. Nothing enforces they stay identical except this test, so if
// someone edits one and forgets the other, this must fail loudly rather than
// let the two ends of the product quietly disagree about what a method is called.
func TestEmbeddedCopyMatchesSourceOfTruth(t *testing.T) {
	sourceOfTruth, err := os.ReadFile("../../../../packages/contracts/vocab/methods.json")
	if err != nil {
		t.Fatalf("could not read source of truth: %v", err)
	}
	if !bytes.Equal(sourceOfTruth, methodsJSON) {
		t.Fatal("apps/api/internal/vocab/methods.json has drifted from packages/contracts/vocab/methods.json — copy the source of truth over the embedded file and rerun")
	}
}

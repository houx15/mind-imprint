package cards

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	specs, err := Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(specs) != 45 {
		t.Fatalf("catalog has %d specs, want 45", len(specs))
	}

	seen := map[string]bool{}
	for _, s := range specs {
		if s.ID == "" {
			t.Fatal("spec with empty id")
		}
		if seen[s.ID] {
			t.Fatalf("duplicate id %q", s.ID)
		}
		seen[s.ID] = true
	}
}

func TestByID(t *testing.T) {
	got, ok := ByID("ai-boundary")
	if !ok {
		t.Fatal("ai-boundary not found")
	}
	if got.NameEN != "AI Boundary & Hallucination Check" {
		t.Fatalf("name_en = %q", got.NameEN)
	}
	if got.Category != "AI伦理" {
		t.Fatalf("category = %q", got.Category)
	}

	if _, ok := ByID("does-not-exist"); ok {
		t.Fatal("unknown id should not resolve")
	}
}

// TestMirrorMatchesCanonical fails if someone edited a canonical card without
// re-running `make sync-cards`. It compares the embedded mirror byte-for-byte
// against packages/contracts/cards.
func TestMirrorMatchesCanonical(t *testing.T) {
	const canonicalRel = "../../../../packages/contracts/cards"
	canonicalEntries, err := os.ReadDir(canonicalRel)
	if err != nil {
		t.Fatalf("read canonical dir: %v", err)
	}

	canonicalJSON := map[string][]byte{}
	for _, e := range canonicalEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(canonicalRel, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		canonicalJSON[e.Name()] = b
	}

	mirrorEntries, err := specFS.ReadDir("specs")
	if err != nil {
		t.Fatalf("read embedded specs: %v", err)
	}
	mirrorJSON := map[string][]byte{}
	for _, e := range mirrorEntries {
		b, err := specFS.ReadFile("specs/" + e.Name())
		if err != nil {
			t.Fatalf("read embedded %s: %v", e.Name(), err)
		}
		mirrorJSON[e.Name()] = b
	}

	if len(canonicalJSON) != len(mirrorJSON) {
		t.Fatalf("file count drift: canonical=%d mirror=%d (run `make sync-cards`)",
			len(canonicalJSON), len(mirrorJSON))
	}
	for name, want := range canonicalJSON {
		got, ok := mirrorJSON[name]
		if !ok {
			t.Fatalf("mirror missing %s (run `make sync-cards`)", name)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("mirror drift in %s (run `make sync-cards`)", name)
		}
	}
}

func TestToulminSlotsParse(t *testing.T) {
	spec, ok := ByID("toulmin")
	if !ok {
		t.Fatal("toulmin not found")
	}
	if spec.Primitive != "graph" {
		t.Fatalf("primitive = %q, want graph", spec.Primitive)
	}
	if got := len(spec.Params.Slots); got != 5 {
		t.Fatalf("slots = %d, want 5", got)
	}
	byID := map[string]Slot{}
	for _, s := range spec.Params.Slots {
		byID[s.ID] = s
	}
	if !byID["evidence"].NeedSrc || byID["claim"].NeedSrc {
		t.Fatalf("needSrc wrong: evidence must need a source, claim must not")
	}
	if byID["claim"].Role != "核心主张" {
		t.Fatalf("claim role = %q, want 核心主张", byID["claim"].Role)
	}
}

func TestSpecParsesSortScaleMatrixParams(t *testing.T) {
	raw := []byte(`{
	  "id": "x", "name": "X",
	  "primitive": "sort",
	  "params": {
	    "buckets": [{"id":"事实","label":"可查证的事实","hint":"能被独立核查的陈述"}],
	    "cols": [{"id":"position","label":"立场主张","q":"这个视角主张什么？"}],
	    "min_items": 3
	  },
	  "completion": [{"kind":"items_bucketed","tags":["事实"],"min":3}]
	}`)
	var s Spec
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s.Params.Buckets) != 1 || s.Params.Buckets[0].ID != "事实" ||
		s.Params.Buckets[0].Label != "可查证的事实" || s.Params.Buckets[0].Hint != "能被独立核查的陈述" {
		t.Fatalf("buckets: %+v", s.Params.Buckets)
	}
	if len(s.Params.Cols) != 1 || s.Params.Cols[0].ID != "position" ||
		s.Params.Cols[0].Label != "立场主张" || s.Params.Cols[0].Q != "这个视角主张什么？" {
		t.Fatalf("cols: %+v", s.Params.Cols)
	}
	if s.Params.MinItems != 3 {
		t.Fatalf("min_items: %d", s.Params.MinItems)
	}
	if len(s.Completion) != 1 || s.Completion[0].Min != 3 {
		t.Fatalf("completion min: %+v", s.Completion)
	}
}

func TestLegacyCardStillParsesWithZeroValuedNewParams(t *testing.T) {
	s, ok := ByID("steelman")
	if !ok {
		t.Fatal("steelman spec missing")
	}
	if len(s.Params.Buckets) != 0 || len(s.Params.Cols) != 0 || s.Params.MinItems != 0 {
		t.Fatalf("legacy card picked up C2 params: %+v", s.Params)
	}
}

// TestNewPrimitiveCardsAreWiredConsistently proves the three C2 sort/scale/
// matrix bindings authored onto fact-opinion-value, certainty-spectrum, and
// the new perspective-matrix card are internally consistent: primitive kind,
// completion predicate, declared vocabulary/columns, and (for the matrix)
// the perspectives graph effect.
func TestNewPrimitiveCardsAreWiredConsistently(t *testing.T) {
	for _, tc := range []struct {
		id, primitive, predicate string
	}{
		{"fact-opinion-value", "sort", "items_bucketed"},
		{"certainty-spectrum", "scale", "items_bucketed"},
		{"perspective-matrix", "matrix", "matrix_complete"},
	} {
		s, ok := ByID(tc.id)
		if !ok {
			t.Fatalf("%s: spec missing", tc.id)
		}
		if s.Primitive != tc.primitive {
			t.Fatalf("%s: primitive = %q, want %q", tc.id, s.Primitive, tc.primitive)
		}
		found := false
		for _, p := range s.Completion {
			if p.Kind == tc.predicate {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: no %s completion predicate", tc.id, tc.predicate)
		}
	}

	// sort/scale: every completion tag must be a declared bucket, else the
	// card can never complete.
	for _, id := range []string{"fact-opinion-value", "certainty-spectrum"} {
		s, _ := ByID(id)
		declared := map[string]bool{}
		for _, b := range s.Params.Buckets {
			declared[b.ID] = true
		}
		if len(declared) == 0 {
			t.Fatalf("%s: no buckets", id)
		}
		for _, p := range s.Completion {
			if p.Kind != "items_bucketed" {
				continue
			}
			if p.Min <= 0 {
				t.Fatalf("%s: items_bucketed min = %d", id, p.Min)
			}
			for _, tag := range p.Tags {
				if !declared[tag] {
					t.Fatalf("%s: completion tag %q is not a declared bucket", id, tag)
				}
			}
		}
	}

	// matrix: cols + a positive MinItems + the perspectives effect.
	m, _ := ByID("perspective-matrix")
	if len(m.Params.Cols) != 3 || m.Params.MinItems < 2 {
		t.Fatalf("perspective-matrix params: %+v", m.Params)
	}
	if len(m.GraphEffects) != 1 || m.GraphEffects[0].Kind != "perspectives" {
		t.Fatalf("perspective-matrix graph_effects: %+v", m.GraphEffects)
	}
}

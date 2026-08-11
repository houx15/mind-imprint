package api

import (
	"encoding/json"
	"testing"

	"mindimprint/api/internal/agent"
)

// toStepRefs attaches each step's cached guide card (so the finished-cards fold
// can re-show a finished part's original guidance), and leaves Card nil for
// steps that were never visited (no cache entry) or whose cache is unparseable.
func TestToStepRefs_AttachesCachedGuideCards(t *testing.T) {
	steps := []agent.Step{
		{Key: "understanding", Title: "对题目的理解", Kind: agent.KindFixed},
		{Key: "thesis", Title: "暂定论点", Kind: agent.KindFixed},
		{Key: "resources", Title: "资源", Kind: agent.KindFixed},
	}
	guides := map[string]string{
		"understanding": string(mustJSON(guideCardDTO{Prompt: "先说清你怎么理解这个题目", Example: "An example"})),
		"resources":     "not json", // unparseable → Card stays nil, never panics
	}

	refs := toStepRefs(steps, guides)
	if len(refs) != 3 {
		t.Fatalf("want 3 refs, got %d", len(refs))
	}

	// Visited step with a valid cached card → guidance is exposed.
	if refs[0].Card == nil {
		t.Fatalf("understanding: want a cached card, got nil")
	}
	if refs[0].Card.Prompt != "先说清你怎么理解这个题目" || refs[0].Card.Example != "An example" {
		t.Fatalf("understanding: card mismatch: %+v", refs[0].Card)
	}
	// Unvisited step (no cache entry) → nil.
	if refs[1].Card != nil {
		t.Fatalf("thesis: want nil card (never visited), got %+v", refs[1].Card)
	}
	// Unparseable cache → nil (best-effort, no panic).
	if refs[2].Card != nil {
		t.Fatalf("resources: want nil card (bad cache), got %+v", refs[2].Card)
	}
}

// A nil guides map must not panic (free mode / never-started tracks).
func TestToStepRefs_NilGuidesMap(t *testing.T) {
	steps := []agent.Step{{Key: "understanding", Title: "对题目的理解", Kind: agent.KindFixed}}
	refs := toStepRefs(steps, nil)
	if len(refs) != 1 || refs[0].Card != nil {
		t.Fatalf("nil guides: want 1 ref with nil card, got %+v", refs)
	}
	// The parsed value round-trips as valid JSON on the wire.
	if _, err := json.Marshal(refs); err != nil {
		t.Fatalf("marshal: %v", err)
	}
}

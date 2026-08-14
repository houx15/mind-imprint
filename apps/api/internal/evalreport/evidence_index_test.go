package evalreport

import "testing"

func TestEvidenceIndex_HasGetLen(t *testing.T) {
	ix := NewEvidenceIndex([]Candidate{
		{ID: "m1", Kind: KindChat, Label: "问：中国最可持续吗"},
		{ID: "e1", Kind: KindEvent, Label: "source_opened"},
		{ID: "", Kind: KindCard, Label: "dropped-empty-id"},
	})
	if ix.Len() != 2 {
		t.Fatalf("Len = %d, want 2 (empty id skipped)", ix.Len())
	}
	if !ix.Has("m1") || !ix.Has("e1") {
		t.Fatal("expected m1 and e1 present")
	}
	if ix.Has("nope") {
		t.Fatal("unexpected id present")
	}
	c, ok := ix.Get("m1")
	if !ok || c.Kind != KindChat {
		t.Fatalf("Get(m1) = %+v, %v", c, ok)
	}
}

func TestValidateRefs_blanksUnknownKeepsLabel(t *testing.T) {
	idx := NewEvidenceIndex([]Candidate{
		{ID: "good-msg", Kind: KindChat, Label: "ok"},
		{ID: "good-ev", Kind: KindEvent, Label: "ok"},
	})

	rep := &Report{
		Events: []EventEntry{
			{Ref: &Ref{ID: "good-ev", Label: "opened"}},
			{Ref: &Ref{ID: "bogus-ev", Label: "hallucinated"}},
			{Ref: nil}, // no ref — untouched, uncounted
		},
		Materials: []MaterialEntry{
			{UsedIn: &Ref{ID: "bogus-node", Label: "node:x"}},
		},
		Depth: []DepthDimResult{
			{Evidence: []EvidenceItem{
				{ID: "good-msg", Quote: "real"},
				{ID: "bogus-msg", Quote: "fake"},
				{ID: "", Quote: "no id"},
			}},
		},
		Autonomy: []AutonomyDimResult{
			{Evidence: []EvidenceItem{{ID: "bogus-msg2"}}},
		},
		PromptLens: PromptLens{Prompts: []PromptItem{
			{Ref: Ref{ID: "good-msg", Label: "keep"}},
			{Ref: Ref{ID: "bogus-prompt", Label: "keep-label"}},
		}},
		Risks: []RiskEntry{
			{Ref: &Ref{ID: "bogus-risk", Label: "risk"}},
		},
	}

	st := ValidateRefs(rep, idx)

	// Examined: good-ev, bogus-ev, bogus-node, good-msg, bogus-msg, bogus-msg2,
	// good-msg(prompt), bogus-prompt, bogus-risk = 9. Empty/nil not counted.
	if st.Total != 9 {
		t.Errorf("Total = %d, want 9", st.Total)
	}
	// Dropped: bogus-ev, bogus-node, bogus-msg, bogus-msg2, bogus-prompt,
	// bogus-risk = 6.
	if st.Dropped != 6 {
		t.Errorf("Dropped = %d, want 6", st.Dropped)
	}

	// Valid ids survive.
	if rep.Events[0].Ref.ID != "good-ev" {
		t.Error("valid event ref id was dropped")
	}
	if rep.Depth[0].Evidence[0].ID != "good-msg" {
		t.Error("valid evidence id was dropped")
	}
	if rep.PromptLens.Prompts[0].Ref.ID != "good-msg" {
		t.Error("valid prompt ref id was dropped")
	}
	// Bogus ids are blanked, labels preserved.
	if rep.Events[1].Ref.ID != "" || rep.Events[1].Ref.Label != "hallucinated" {
		t.Errorf("bogus event ref = %+v, want blank id + kept label", *rep.Events[1].Ref)
	}
	if rep.Materials[0].UsedIn.ID != "" || rep.Materials[0].UsedIn.Label != "node:x" {
		t.Errorf("bogus material usedIn = %+v", *rep.Materials[0].UsedIn)
	}
	if rep.Depth[0].Evidence[1].ID != "" {
		t.Error("bogus evidence id not blanked")
	}
	if rep.PromptLens.Prompts[1].Ref.ID != "" || rep.PromptLens.Prompts[1].Ref.Label != "keep-label" {
		t.Errorf("bogus prompt ref = %+v", rep.PromptLens.Prompts[1].Ref)
	}
	if rep.Risks[0].Ref.ID != "" {
		t.Error("bogus risk ref not blanked")
	}
}

func TestLabel(t *testing.T) {
	if got := Label("hello", 10); got != "hello" {
		t.Errorf("Label short = %q", got)
	}
	if got := Label("中国是否让地球更可持续", 4); got != "中国是否…" {
		t.Errorf("Label cut = %q, want 中国是否…", got)
	}
}

package pbl

import "testing"

func TestComparisonRequiresProcessAndTrial(t *testing.T) {
	doc := CreativeDirection{Stage: "hero", IncludeComparison: true}
	if _, err := NormalizeCreativeDirection(doc, false); err == nil {
		t.Fatal("comparison without process accepted")
	}
	doc.IncludeProcess = true
	if _, err := NormalizeCreativeDirection(doc, false); err == nil {
		t.Fatal("comparison without trial accepted")
	}
	doc.Trial = &HeroTrial{VersionID: "saved-version", Observation: "Flower opens with Enter."}
	if _, err := NormalizeCreativeDirection(doc, false); err != nil {
		t.Fatal(err)
	}
}

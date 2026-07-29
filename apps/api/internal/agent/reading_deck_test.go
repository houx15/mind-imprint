package agent

import (
	"testing"

	"mindimprint/api/internal/cards"
)

func TestReadingDeck_CoversAllIDsFromRegistry(t *testing.T) {
	deck, err := ReadingDeck()
	if err != nil {
		t.Fatalf("ReadingDeck error: %v", err)
	}
	if len(deck) != len(ReadingDeckIDs) {
		t.Fatalf("deck size = %d, want %d", len(deck), len(ReadingDeckIDs))
	}
	byID := map[string]ReadingCard{}
	for _, c := range deck {
		byID[c.CardID] = c
	}
	for _, id := range ReadingDeckIDs {
		c, ok := byID[id]
		if !ok {
			t.Fatalf("deck missing id %q", id)
		}
		if c.Name == "" || c.Trigger == "" {
			t.Fatalf("deck entry %q has empty name/trigger: %+v", id, c)
		}
	}
}

func TestReadingDeck_IsSourceCheckPlusNineLenses(t *testing.T) {
	if len(ReadingDeckIDs) != 11 {
		t.Fatalf("ReadingDeckIDs = %d, want 11 (craap + sift + 9 学科透镜)", len(ReadingDeckIDs))
	}
}

// Every deep-reading deck id must be a lens carrying a reading_lens block —
// that block is what makes buildCardExamplePrompt fit the pick-one-sentence
// mechanic. A tool card sneaking back in (no reading_lens) is the exact
// regression that produced the "没找到好例子" hard-fail.
func TestReadingDeck_LensesCarryReadingLens(t *testing.T) {
	for _, id := range ReadingDeckIDs {
		if id == "craap" || id == "sift" {
			continue
		}
		spec, ok := cards.ByID(id)
		if !ok {
			t.Fatalf("deck id %q not in registry", id)
		}
		if spec.ReadingLens == nil {
			t.Fatalf("deck lens %q has no reading_lens block", id)
		}
		if spec.ReadingLens.TaskPrompt == "" || spec.ReadingLens.SelectionHint == "" || spec.ReadingLens.ExampleFocus == "" {
			t.Fatalf("deck lens %q reading_lens missing task/hint/focus: %+v", id, spec.ReadingLens)
		}
	}
}

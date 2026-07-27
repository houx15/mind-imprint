package agent

import "testing"

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

func TestReadingDeck_HasFifteenCards(t *testing.T) {
	if len(ReadingDeckIDs) != 15 {
		t.Fatalf("ReadingDeckIDs = %d, want 15 (spec §18)", len(ReadingDeckIDs))
	}
}

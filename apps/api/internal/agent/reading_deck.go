package agent

import (
	"fmt"

	"mindimprint/api/internal/cards"
)

// ReadingCard is one entry in the read-together router catalog: the id the
// router may summon, plus the human name and the one-line "when to reach for
// this" the router reasons over. Triggers are sourced from the card spec
// (single source of truth) — never hand-written here.
type ReadingCard struct {
	CardID  string
	Name    string
	Trigger string
}

// ReadingDeckIDs is the fixed set of reading-room cards (spec §18): source-check
// (craap, sift) + deep reading. All are sentence-based, so all fit the
// hang-on-sentence + you-find-the-evidence mechanic. Growing the deck later =
// adding an id here; no page or mechanic change.
var ReadingDeckIDs = []string{
	"craap", "sift",
	"fact-opinion-value", "argument-map", "toulmin", "steelman", "concession",
	"data-literacy", "opcvl", "framing", "spin-detector", "cda",
	"perspective-matrix", "certainty-spectrum", "science-knowing",
}

// ReadingDeck resolves ReadingDeckIDs against the card registry. It errors if
// any id is missing — the deck must never drift from the specs that back it.
func ReadingDeck() ([]ReadingCard, error) {
	out := make([]ReadingCard, 0, len(ReadingDeckIDs))
	for _, id := range ReadingDeckIDs {
		spec, ok := cards.ByID(id)
		if !ok {
			return nil, fmt.Errorf("reading deck id %q not found in registry", id)
		}
		trigger := spec.TriggerCondition
		if trigger == "" {
			trigger = spec.Purpose
		}
		out = append(out, ReadingCard{CardID: spec.ID, Name: spec.Name, Trigger: trigger})
	}
	return out, nil
}

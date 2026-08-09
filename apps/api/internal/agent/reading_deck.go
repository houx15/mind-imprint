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

// ReadingDeckIDs is the fixed set of reading-room cards: source-check (craap,
// sift) + the 9 disciplinary reading lenses (学科透镜). The deep-reading slots
// were the WRITING tool cards (argument-map/toulmin/…), whose purpose ("拆成
// 结构图") doesn't map to the pick-one-sentence mechanic — so a student summon
// could almost never ground a single illustrative sentence and hard-failed to
// the "没找到好例子" coach line. The lenses are purpose-built for it (each carries
// reading_lens.task_prompt/selection_hint/example_focus). Tool cards stay in
// the writing studio; a lens's reading_lens.method_ids point back at them for
// the future methods layer. Growing the deck = adding an id here AND to the web
// READING_DECK_IDS. See docs/2026-07-29-reading-lenses-adopt-demo.md.
var ReadingDeckIDs = []string{
	"craap", "sift",
	"lens-logic", "lens-methods", // 推理与证据
	"lens-society", "lens-law", "lens-economics", "lens-ethics", // 人与制度
	"lens-history", "lens-communication", "lens-systems", // 语境与系统
}

// ReadingToolkitIDs are the source-analysis cards the coach offers INSIDE the
// reading room, contextually, when a relevant source is open (e.g. opcvl for a
// history source, money-trail for a funded report). Re-catalog 2026-08-09
// (bucket 4): these left the writing-flow decks — they're source-critique tools,
// belonging where sources are read. search-plan (检索方向审视) is AI-side here:
// the coach uses it when proposing search directions, not a student summon.
// Wiring the per-source offer is a later reading-room slice; this list is the
// authoritative membership + the placement-tag drift check.
var ReadingToolkitIDs = []string{
	"cda", "money-trail", "multimodal-decode", "spin-detector",
	"data-literacy", "fact-opinion-value", "opcvl", "belief-spectrum",
	"search-plan",
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
		// For a lens, the router reasons better over "when to reach for this
		// angle + what sentence it wants" than over the generic trigger — so
		// prefer trigger_condition + task_prompt when reading_lens is present.
		trigger := spec.TriggerCondition
		if trigger == "" {
			trigger = spec.Purpose
		}
		if spec.ReadingLens != nil && spec.ReadingLens.TaskPrompt != "" {
			trigger = trigger + "。" + spec.ReadingLens.TaskPrompt
		}
		out = append(out, ReadingCard{CardID: spec.ID, Name: spec.Name, Trigger: trigger})
	}
	return out, nil
}

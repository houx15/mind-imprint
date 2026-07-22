package api

import (
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// TestCountDeclaration_CountersFromExistingData proves countDeclaration is a
// pure re-read of data that already exists: prompt_sent events → Asks,
// len(Dispositions) → Dispositions, and the card 自发/提示后 split matches
// projectEquipment's own rule (studio/projection.go) — an intervention
// linking the card_instance means 提示后, no link means 自发. AiWrittenProse
// is always 0 (RL-1: no path ever writes AI text into the draft).
func TestCountDeclaration_CountersFromExistingData(t *testing.T) {
	c1, c2, c3 := uuid.New(), uuid.New(), uuid.New()
	d := studio.ProjectData{
		Events: []studio.Event{
			{Type: "prompt_sent", Surface: "studio", Payload: []byte(`{"text":"a"}`)},
			{Type: "card_surfaced", Surface: "studio", Payload: []byte(`{}`)},
			{Type: "prompt_sent", Surface: "studio", Payload: []byte(`{"text":"b"}`)},
		},
		Dispositions: []sqlc.Disposition{
			{ID: uuid.New(), Action: "accept"},
			{ID: uuid.New(), Action: "reject"},
		},
		Cards: []sqlc.CardInstance{
			{ID: c1, CardID: "sift_craap"},
			{ID: c2, CardID: "concession"},
			{ID: c3, CardID: "steelman"},
		},
		Interventions: []sqlc.Intervention{
			// Links c1 → c1 is 提示后; c2/c3 are unlinked → 自发.
			{ID: uuid.New(), CardInstanceID: pgID(c1), Type: "flag"},
			// No CardInstanceID at all — must not spuriously nudge anything.
			{ID: uuid.New(), Type: "review_item"},
		},
	}

	got := countDeclaration(d)

	if got.Asks != 2 {
		t.Errorf("Asks = %d, want 2", got.Asks)
	}
	if got.Dispositions != 2 {
		t.Errorf("Dispositions = %d, want 2", got.Dispositions)
	}
	if got.CardsPrompted != 1 {
		t.Errorf("CardsPrompted = %d, want 1 (only c1 is linked by an intervention)", got.CardsPrompted)
	}
	if got.CardsSpontaneous != 2 {
		t.Errorf("CardsSpontaneous = %d, want 2 (c2, c3)", got.CardsSpontaneous)
	}
	if got.AiWrittenProse != 0 {
		t.Errorf("AiWrittenProse = %d, want 0 (RL-1: no path writes AI text into the draft)", got.AiWrittenProse)
	}
}

// TestCountDeclaration_Empty proves an untouched project reports honest zeros
// rather than panicking or fabricating any count.
func TestCountDeclaration_Empty(t *testing.T) {
	got := countDeclaration(studio.ProjectData{})
	want := DeclarationCounts{}
	if got != want {
		t.Errorf("countDeclaration(empty) = %+v, want all-zero %+v", got, want)
	}
}

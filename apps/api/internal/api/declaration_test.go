package api

import (
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/skills"
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
			{ID: c1, CardID: "craap"},
			{ID: c2, CardID: "concession"},
			{ID: c3, CardID: "toulmin"},
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

// TestCountDeclaration_AgreesWithLiveProjection is the regression net for the
// constraint that actually matters: countDeclaration (the persisted-snapshot
// path, run once at sign time) and studio.projectDeclaration (the
// live-projection path, run on every read before signing) must NEVER
// disagree about which cards are 自发/提示后. Both derive from the same
// exported studio.NudgedCardInstanceIDs, but this test exercises them
// through their real, independent production entry points
// (api.countDeclaration vs. studio.Project's Coach.Equipment/Declaration)
// over the identical ProjectData, so a future edit that reintroduces a
// second, drifting copy of the rule in either call site fails here — not
// hand-computed expected numbers that can't see the two paths diverge from
// each other.
func TestCountDeclaration_AgreesWithLiveProjection(t *testing.T) {
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill not found")
	}
	c1, c2, c3, c4 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	d := studio.ProjectData{
		Cards: []sqlc.CardInstance{
			{ID: c1, CardID: "craap"},
			{ID: c2, CardID: "concession"},
			{ID: c3, CardID: "toulmin"},
			{ID: c4, CardID: "perspective-matrix"},
		},
		Interventions: []sqlc.Intervention{
			{ID: uuid.New(), CardInstanceID: pgID(c1), Type: "flag"},
			{ID: uuid.New(), CardInstanceID: pgID(c3), Type: "flag"},
			{ID: uuid.New(), Type: "review_item"}, // no CardInstanceID
		},
	}

	fromSnapshot := countDeclaration(d)

	proj, err := studio.Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatalf("studio.Project: %v", err)
	}
	liveSpont, livePrompted := 0, 0
	for _, ec := range proj.Coach.Equipment {
		if ec.Spont == "提示后" {
			livePrompted++
		} else {
			liveSpont++
		}
	}

	if fromSnapshot.CardsPrompted != livePrompted || fromSnapshot.CardsSpontaneous != liveSpont {
		t.Fatalf("split diverged: countDeclaration = {spont:%d prompted:%d}, live projection = {spont:%d prompted:%d}",
			fromSnapshot.CardsSpontaneous, fromSnapshot.CardsPrompted, liveSpont, livePrompted)
	}
	// Also pin the actual numbers so the test fails loudly (not just
	// vacuously agreeing at 0/0) if the shared rule itself regresses.
	if fromSnapshot.CardsPrompted != 2 || fromSnapshot.CardsSpontaneous != 2 {
		t.Fatalf("got {spont:%d prompted:%d}, want {spont:2 prompted:2} (c1,c3 linked; c2,c4 unlinked)",
			fromSnapshot.CardsSpontaneous, fromSnapshot.CardsPrompted)
	}

	// And proj.Declaration itself (unsigned) must report the same split —
	// it's the field the S6 view actually renders before signing.
	if proj.Declaration.CardsPrompted != 2 || proj.Declaration.CardsSpontaneous != 2 {
		t.Fatalf("proj.Declaration = {spont:%d prompted:%d}, want {spont:2 prompted:2}",
			proj.Declaration.CardsSpontaneous, proj.Declaration.CardsPrompted)
	}
}

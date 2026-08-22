package studio

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

func pgUUID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

func strPtr(s string) *string { return &s }

// TestProjectDerivesMaterialState pins projectMaterials' derive-never-decorate
// contract: locked/role come from the CRAAP mint (an evaluated-as edge → an
// evidence node's source_quality.risk_note), tier/takeaway/timeSpentS from the
// student's source-log entry, anchors from the persisted card_instances.anchors
// targeting this material — an untouched material carries none of that state.
// Exercised via the exported ProjectMaterials wrapper (the same entry point
// the Read-library's enter-reading endpoint uses), not the removed Project().
func TestProjectDerivesMaterialState(t *testing.T) {
	matA := uuid.MustParse("00000000-0000-0000-0000-0000000000aa") // evaluated
	matB := uuid.MustParse("00000000-0000-0000-0000-0000000000bb") // untouched
	evid := uuid.MustParse("00000000-0000-0000-0000-0000000000cc")

	d := ProjectData{
		Materials: []sqlc.Material{
			{ID: matA, Title: "《卫星图看中国变绿》", Kind: "article", Source: "fetched",
				Blocks: []byte(`[{"id":"b1","text":"过去二十年……"}]`)},
			{ID: matB, Title: "IEA Renewable Investment", Kind: "article", Source: "pasted",
				Blocks: []byte(`[{"id":"b1","text":"China ranks first."}]`)},
		},
		SourceLog: []sqlc.SourceLogEntry{
			{MaterialID: pgUUID(matA), Tier: strPtr("二手 · 需追源"), Takeaway: "结论被放大了。", TimeSpentS: 240},
		},
		// The CRAAP mint: evidence node + evaluated-as edge from the material.
		Nodes: []sqlc.GraphNode{
			{ID: evid, Type: "evidence", Author: "student",
				Body: []byte(`{"source_quality":{"authority":"只是一个博主","risk_note":"入口来源，不能直接引用。"}}`)},
		},
		Edges: []sqlc.GraphEdge{
			{Type: "evaluated-as", FromKind: "material", FromID: matA, ToKind: "graph_node", ToID: evid},
		},
		// A persisted CRAAP card whose anchors target matA.
		Cards: []sqlc.CardInstance{
			{CardID: "craap", Status: "completed",
				Anchors: []byte(`[{"id":"a1","material_id":"` + matA.String() + `","block_id":"b1","start":0,"end":4,"quote":"过去二十年","dimension":"authority","author":"ai","question":"原始出处是谁？","answer":"只是一个博主"}]`)},
		},
	}

	materials := ProjectMaterials(d)
	if len(materials) != 2 {
		t.Fatalf("materials = %d, want 2", len(materials))
	}

	a := materials[0]
	if !a.Locked {
		t.Error("evaluated material: Locked = false, want true (an evaluated-as edge exists)")
	}
	if a.Role != "入口来源，不能直接引用。" {
		t.Errorf("Role = %q, want the minted evidence node's source_quality.risk_note", a.Role)
	}
	if a.Tier != "二手 · 需追源" || a.Takeaway != "结论被放大了。" {
		t.Errorf("Tier/Takeaway = %q/%q, want the source-log values", a.Tier, a.Takeaway)
	}
	if a.TimeSpentS != 240 {
		t.Errorf("TimeSpentS = %d, want 240 (the source-log entry's accumulated reading time)", a.TimeSpentS)
	}
	if len(a.Anchors) != 1 {
		t.Errorf("Anchors = %d, want 1 (the card anchor targeting this material)", len(a.Anchors))
	}

	b := materials[1]
	if b.Locked || b.Role != "" || b.Tier != "" || b.Takeaway != "" || len(b.Anchors) != 0 || b.TimeSpentS != 0 {
		t.Errorf("untouched material carries state it never earned: %+v", b)
	}
}

// TestProjectMaterials_LateralReadIsDerivedFromTheMint — a material with a
// source_log_entry whose lateral_read is true (Task 7's cross_check mint)
// projects as laterally read; one without does not. Nothing is invented: the
// flag is written by the mint's atomic transaction and merely read here.
func TestProjectMaterials_LateralReadIsDerivedFromTheMint(t *testing.T) {
	blogID := uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	nasaID := uuid.MustParse("00000000-0000-0000-0000-0000000000b2")

	d := ProjectData{
		Materials: []sqlc.Material{
			{ID: blogID, Title: "《卫星图看中国变绿》博客", Kind: "article", Source: "fetched",
				Blocks: []byte(`[{"id":"b1","text":"过去二十年……"}]`)},
			{ID: nasaID, Title: "NASA Earth Observatory", Kind: "article", Source: "pasted",
				Blocks: []byte(`[{"id":"b1","text":"Satellite data shows..."}]`)},
		},
		SourceLog: []sqlc.SourceLogEntry{
			// blog was the subject of the cross-check: its own entry is flipped.
			{MaterialID: pgUUID(blogID), Tier: strPtr("二手 · 需追源"), LateralRead: true},
			// nasa is the instrument (the lateral source she checked against) —
			// its own entry is deliberately untouched by the mint.
			{MaterialID: pgUUID(nasaID), Tier: strPtr("一手报道")},
		},
	}

	got := projectMaterials(d)
	if len(got) != 2 {
		t.Fatalf("materials = %d, want 2", len(got))
	}

	var blog, nasa MaterialDTO
	for _, m := range got {
		switch m.ID {
		case blogID.String():
			blog = m
		case nasaID.String():
			nasa = m
		}
	}
	if !blog.LateralRead {
		t.Fatal("the checked source must project as laterally read")
	}
	if nasa.LateralRead {
		t.Fatal("the lateral source is the instrument, not the subject")
	}
}

// TestProjectMaterials_IsLateralInstrumentDerivedFromCitesEdge — a material
// that is itself the lateral source some OTHER cross_check cited (a
// cross_check node --cites--> this material) projects isLateralInstrument =
// true; the checked material (target of that cross_check's own
// cross-checked-by edge) does not. This is the exact graph fact
// agent.SurfaceCardCandidates' treadmill guard already reads
// (classifier.go) — the dossier's chip must derive from the SAME fact, never
// recompute graph reachability independently (whole-branch review finding
// [4]).
func TestProjectMaterials_IsLateralInstrumentDerivedFromCitesEdge(t *testing.T) {
	blogID := uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
	nasaID := uuid.MustParse("00000000-0000-0000-0000-0000000000c2")
	crossCheckID := uuid.MustParse("00000000-0000-0000-0000-0000000000c3")

	d := ProjectData{
		Materials: []sqlc.Material{
			{ID: blogID, Title: "《卫星图看中国变绿》博客", Kind: "article", Source: "fetched", Blocks: []byte(`[]`)},
			{ID: nasaID, Title: "NASA Earth Observatory", Kind: "article", Source: "pasted", Blocks: []byte(`[]`)},
		},
		Nodes: []sqlc.GraphNode{
			{ID: crossCheckID, Type: "cross_check", Author: "student", Body: []byte(`{}`)},
		},
		Edges: []sqlc.GraphEdge{
			{Type: "cross-checked-by", FromKind: "material", FromID: blogID, ToKind: "graph_node", ToID: crossCheckID},
			{Type: "cites", FromKind: "graph_node", FromID: crossCheckID, ToKind: "material", ToID: nasaID},
		},
	}

	got := projectMaterials(d)
	var blog, nasa MaterialDTO
	for _, m := range got {
		switch m.ID {
		case blogID.String():
			blog = m
		case nasaID.String():
			nasa = m
		}
	}
	if blog.IsLateralInstrument {
		t.Fatal("the checked source is not a lateral instrument")
	}
	if !nasa.IsLateralInstrument {
		t.Fatal("the material a cross_check cites IS the lateral instrument")
	}
}

// TestProjectMaterials_SiftSkippedDerivedFromSkippedCardInstance is FIX 3
// (whole-branch review): a material whose SIFT card_instance was explicitly
// skipped must project siftSkipped = true, so the dossier's 需横向阅读 chip
// (gated on !siftSkipped, SourceDossier.tsx) stops claiming a lateral-read
// workflow agent.SurfaceCardCandidates will in fact never offer again once a
// SIFT has been skipped on that material (its siftSurfaced map treats ANY
// status, including "skipped", as already-surfaced-forever). A material with
// no SIFT card_instance at all, or one that is still active/completed,
// projects siftSkipped = false.
func TestProjectMaterials_SiftSkippedDerivedFromSkippedCardInstance(t *testing.T) {
	skippedID := uuid.MustParse("00000000-0000-0000-0000-0000000000d1")
	untouchedID := uuid.MustParse("00000000-0000-0000-0000-0000000000d2")
	activeID := uuid.MustParse("00000000-0000-0000-0000-0000000000d3")
	skippedCardID := uuid.MustParse("00000000-0000-0000-0000-0000000000d4")
	activeCardID := uuid.MustParse("00000000-0000-0000-0000-0000000000d5")

	d := ProjectData{
		Materials: []sqlc.Material{
			{ID: skippedID, Title: "skipped-sift source", Kind: "article", Source: "pasted", Blocks: []byte(`[]`)},
			{ID: untouchedID, Title: "no sift yet", Kind: "article", Source: "pasted", Blocks: []byte(`[]`)},
			{ID: activeID, Title: "sift in progress", Kind: "article", Source: "pasted", Blocks: []byte(`[]`)},
		},
		Cards: []sqlc.CardInstance{
			{ID: skippedCardID, CardID: "sift", Status: "skipped", Anchors: []byte(`[]`)},
			{ID: activeCardID, CardID: "sift", Status: "active", Anchors: []byte(`[]`)},
		},
		Edges: []sqlc.GraphEdge{
			{Type: "evaluates", FromKind: "card_instance", FromID: skippedCardID, ToKind: "material", ToID: skippedID},
			{Type: "evaluates", FromKind: "card_instance", FromID: activeCardID, ToKind: "material", ToID: activeID},
		},
	}

	got := projectMaterials(d)
	byID := map[string]MaterialDTO{}
	for _, m := range got {
		byID[m.ID] = m
	}

	if !byID[skippedID.String()].SiftSkipped {
		t.Fatal("a material with a skipped SIFT card_instance must project siftSkipped = true")
	}
	if byID[untouchedID.String()].SiftSkipped {
		t.Fatal("a material with no SIFT card_instance at all must not project siftSkipped")
	}
	if byID[activeID.String()].SiftSkipped {
		t.Fatal("a material whose SIFT is still active (not skipped) must not project siftSkipped")
	}
}

// TestProjectMaterials_LateralNoteReadsHerOwnWordsFromTheCrossCheckMint —
// whole-branch review's "written but never read" finding: crossCheckBody
// (agent/card_effects.go) has always written `relation` and
// `revised_judgment` onto the minted cross_check node's body, but nothing
// ever read them back. This proves the dossier now derives them — on the
// CHECKED material (the "cross-checked-by" edge's FromID), never on the
// lateral instrument it cites, and never invented when no cross_check exists.
func TestProjectMaterials_LateralNoteReadsHerOwnWordsFromTheCrossCheckMint(t *testing.T) {
	blogID := uuid.MustParse("00000000-0000-0000-0000-0000000000e1")
	nasaID := uuid.MustParse("00000000-0000-0000-0000-0000000000e2")
	untouchedID := uuid.MustParse("00000000-0000-0000-0000-0000000000e3")
	crossCheckID := uuid.MustParse("00000000-0000-0000-0000-0000000000e4")

	d := ProjectData{
		Materials: []sqlc.Material{
			{ID: blogID, Title: "《卫星图看中国变绿》博客", Kind: "article", Source: "fetched", Blocks: []byte(`[]`)},
			{ID: nasaID, Title: "NASA Earth Observatory", Kind: "article", Source: "pasted", Blocks: []byte(`[]`)},
			{ID: untouchedID, Title: "IEA Renewable Investment", Kind: "article", Source: "pasted", Blocks: []byte(`[]`)},
		},
		Nodes: []sqlc.GraphNode{
			{ID: crossCheckID, Type: "cross_check", Author: "student",
				Body: []byte(`{"relation":"印证","revised_judgment":"从二手转述降级为需要追源的说法"}`)},
		},
		Edges: []sqlc.GraphEdge{
			{Type: "cross-checked-by", FromKind: "material", FromID: blogID, ToKind: "graph_node", ToID: crossCheckID},
			{Type: "cites", FromKind: "graph_node", FromID: crossCheckID, ToKind: "material", ToID: nasaID},
		},
	}

	got := projectMaterials(d)
	var blog, nasa, untouched MaterialDTO
	for _, m := range got {
		switch m.ID {
		case blogID.String():
			blog = m
		case nasaID.String():
			nasa = m
		case untouchedID.String():
			untouched = m
		}
	}
	if blog.LateralRelation != "印证" || blog.LateralJudgment != "从二手转述降级为需要追源的说法" {
		t.Fatalf("checked material's lateral note = %+v, want her own relation/revised_judgment", blog)
	}
	if nasa.LateralRelation != "" || nasa.LateralJudgment != "" {
		t.Fatalf("lateral instrument must not carry the checked material's own note: %+v", nasa)
	}
	if untouched.LateralRelation != "" || untouched.LateralJudgment != "" {
		t.Fatalf("untouched material must not invent a lateral note: %+v", untouched)
	}
}

// TestProjectExcludesAnchorsFromSkippedCards — a card the student explicitly
// declined (status "skipped") must not keep re-asking its question: its
// anchors must not light up the article body on reload. Only "skipped" is
// excluded here — "active"/"completed" cards still surface their anchors
// (TestProjectDerivesMaterialState covers "completed"; an in-progress
// "active" card's anchors are exactly what the student is being asked
// about right now). Exercised via ProjectMaterials, not the removed Project().
func TestProjectExcludesAnchorsFromSkippedCards(t *testing.T) {
	mat := uuid.MustParse("00000000-0000-0000-0000-0000000000dd")

	d := ProjectData{
		Materials: []sqlc.Material{
			{ID: mat, Title: "《卫星图看中国变绿》", Kind: "article", Source: "fetched",
				Blocks: []byte(`[{"id":"b1","text":"过去二十年……"}]`)},
		},
		Cards: []sqlc.CardInstance{
			{CardID: "craap", Status: "skipped",
				Anchors: []byte(`[{"id":"a1","material_id":"` + mat.String() + `","block_id":"b1","start":0,"end":4,"quote":"过去二十年","dimension":"authority","author":"ai","question":"原始出处是谁？","answer":""}]`)},
		},
	}

	materials := ProjectMaterials(d)
	if len(materials) != 1 {
		t.Fatalf("materials = %d, want 1", len(materials))
	}
	if len(materials[0].Anchors) != 0 {
		t.Errorf("Anchors = %d, want 0 — a skipped card's anchors must not persist onto the article", len(materials[0].Anchors))
	}
}

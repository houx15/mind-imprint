package studio

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

func pgUUID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

func strPtr(s string) *string { return &s }

func writingSkill(t *testing.T) skills.Skill {
	t.Helper()
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("writing-project not found")
	}
	return sk
}

// gateStateNode builds a gate_state graph_node marking a contract solid.
func gateStateNode(contract string) sqlc.GraphNode {
	return sqlc.GraphNode{ID: uuid.New(), Type: "gate_state",
		Body: []byte(`{"contract":"` + contract + `","confirmed_solid":true,"items":{}}`)}
}

func planNode(route string) *sqlc.GraphNode {
	n := sqlc.GraphNode{ID: uuid.New(), Type: "plan", Body: []byte(`{"route":` + route + `,"reason":"intake"}`)}
	return &n
}

func TestProjectStations_S4Current(t *testing.T) {
	sk := writingSkill(t)
	d := ProjectData{
		Nodes:      nil,
		GateStates: []sqlc.GraphNode{gateStateNode("decode_task"), gateStateNode("frame_question"), gateStateNode("evaluate_perspectives"), gateStateNode("evaluate_sources")},
		Plan:       planNode(`["build_argument","draft_polish","reflect_archive"]`),
	}
	stations, current, err := projectStations(sk, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 7 {
		t.Fatalf("want 7 stations, got %d", len(stations))
	}
	// Order + names + views from TopoOrder.
	if stations[0].Code != "S0" || stations[0].Name != "任务解码" || stations[0].View != "评估" {
		t.Fatalf("S0 = %+v", stations[0])
	}
	if stations[4].Code != "S4" || stations[4].Name != "论证构建" || stations[4].View != "结构" {
		t.Fatalf("S4 = %+v", stations[4])
	}
	// S0-S3 done (gate_state confirmed), S4 current (plan head), S5-S6 locked.
	for i, want := range []string{"done", "done", "done", "done", "current", "locked", "locked"} {
		if stations[i].State != want {
			t.Errorf("S%d state = %q, want %q", i, stations[i].State, want)
		}
	}
	if current != "S4" {
		t.Fatalf("current = %q, want S4", current)
	}
}

func TestProjectStations_GateProgress(t *testing.T) {
	sk := writingSkill(t)
	// build_argument gate: 2 machine + 2 student_written + 1 human = 5 items.
	d := ProjectData{Plan: planNode(`["decode_task"]`)}
	stations, _, err := projectStations(sk, d)
	if err != nil {
		t.Fatal(err)
	}
	s4 := stations[4]
	if s4.Gate == nil || s4.Gate.Total != 5 {
		t.Fatalf("S4 gate = %+v, want total 5", s4.Gate)
	}
	if s4.Gate.Passed != 0 {
		t.Fatalf("S4 passed = %d, want 0 (empty graph)", s4.Gate.Passed)
	}
}

// TestProjectStations_GateProgress_NoVacuousPassOnNonEmptyGraph is the
// realistic case the empty-graph test above misses: by the time a student
// reaches build_argument (S4), S0-S3 are solid and the graph already has
// unrelated nodes from earlier contracts (rubric_translation,
// research_question) — but no claim/evidence node yet. build_argument's
// negation-style machine predicate (no_unsupported_claim) passes vacuously
// over that non-empty graph because there's still nothing of the scanned type
// (claim) to check. That vacuous pass must not count as progress — Passed must
// stay 0.
func TestProjectStations_GateProgress_NoVacuousPassOnNonEmptyGraph(t *testing.T) {
	sk := writingSkill(t)
	d := ProjectData{
		Nodes: []sqlc.GraphNode{
			{ID: uuid.New(), Type: "rubric_translation", Body: []byte(`{}`)},
			{ID: uuid.New(), Type: "research_question", Body: []byte(`{}`)},
		},
		GateStates: []sqlc.GraphNode{gateStateNode("decode_task"), gateStateNode("frame_question"), gateStateNode("evaluate_perspectives"), gateStateNode("evaluate_sources")},
		Plan:       planNode(`["build_argument","draft_polish","reflect_archive"]`),
	}
	stations, current, err := projectStations(sk, d)
	if err != nil {
		t.Fatal(err)
	}
	if current != "S4" {
		t.Fatalf("current = %q, want S4", current)
	}
	s4 := stations[4]
	if s4.Gate == nil || s4.Gate.Total != 5 {
		t.Fatalf("S4 gate = %+v, want total 5", s4.Gate)
	}
	if s4.Gate.Passed != 0 {
		t.Fatalf("S4 passed = %d, want 0 (no claim/evidence nodes yet — vacuous negation passes must not count on a non-empty graph)", s4.Gate.Passed)
	}
}

func TestProjectCoach_AnchorAndThread(t *testing.T) {
	crit := "D5"
	d := ProjectData{
		Interventions: []sqlc.Intervention{
			// flag carries a criterion too, but its headline is the anchor label.
			{Type: "flag", Body: "图上有一处孤儿证据", Criterion: &crit, Anchor: []byte(`{"label":"孤儿证据"}`)},
			{Type: "diagnostic", Body: "连到治理决心", Criterion: &crit, Anchor: []byte(`{"label":"论证图 · 治理决心主张"}`)},
		},
	}
	coach := projectCoach(d, "论证构建")
	if coach.Anchor != "论证图 · 治理决心主张" { // latest intervention's anchor
		t.Fatalf("anchor = %q", coach.Anchor)
	}
	// flag label = anchor.label (not the criterion), even when a criterion is set.
	if len(coach.Messages) != 2 || coach.Messages[0].Kind != "flag" || coach.Messages[0].Label != "孤儿证据" {
		t.Fatalf("messages = %+v", coach.Messages)
	}
	if coach.Messages[1].Kind != "ai" || coach.Messages[1].Tag != "D5" || coach.Messages[1].Anchor != "论证图 · 治理决心主张" {
		t.Fatalf("ai msg = %+v", coach.Messages[1])
	}
}

func TestProjectCoach_AnchorFallback(t *testing.T) {
	coach := projectCoach(ProjectData{}, "论证构建") // no interventions
	if coach.Anchor != "论证构建" {
		t.Fatalf("fallback anchor = %q, want 论证构建", coach.Anchor)
	}
}

func TestProjectEquipment_SpontAndMeth(t *testing.T) {
	steel := uuid.New()
	d := ProjectData{
		Cards: []sqlc.CardInstance{
			{ID: steel, CardID: "steelman"},
			{ID: uuid.New(), CardID: "concession"},
		},
		Interventions: []sqlc.Intervention{
			{CardInstanceID: pgUUID(steel)}, // steelman was nudged
		},
	}
	spec := func(id string) (cards.Spec, bool) {
		return map[string]cards.Spec{"steelman": {ID: "steelman", Name: "钢人卡"}, "concession": {ID: "concession", Name: "让步段卡"}}[id], true
	}
	eq := projectEquipment(d, spec)
	if len(eq) != 2 {
		t.Fatalf("want 2 equip, got %d", len(eq))
	}
	if eq[0].Name != "钢人卡" || eq[0].Spont != "提示后" || eq[0].Meth != "concession" {
		t.Fatalf("steelman = %+v", eq[0])
	}
	if eq[1].Spont != "自发" {
		t.Fatalf("concession spont = %q, want 自发", eq[1].Spont)
	}
}

// TestProjectEquipment_CarriesMaterialID covers whole-branch review finding
// [5]: an equipment-bar card's materialId must come from the SAME
// card_instance--evaluates-->material graph edge SurfaceCard mints — never
// left blank when that edge exists, and never guessed. A card with no such
// edge (shouldn't happen in practice, but the projection must not panic on
// it) gets an empty materialId.
func TestProjectEquipment_CarriesMaterialID(t *testing.T) {
	sift := uuid.New()
	orphan := uuid.New()
	mat := uuid.New()
	d := ProjectData{
		Cards: []sqlc.CardInstance{
			{ID: sift, CardID: "sift"},
			{ID: orphan, CardID: "craap"},
		},
		Edges: []sqlc.GraphEdge{
			{Type: "evaluates", FromKind: "card_instance", FromID: sift, ToKind: "material", ToID: mat},
		},
	}
	spec := func(id string) (cards.Spec, bool) { return cards.Spec{ID: id, Name: id}, true }
	eq := projectEquipment(d, spec)
	if len(eq) != 2 {
		t.Fatalf("want 2 equip, got %d", len(eq))
	}
	if eq[0].MaterialID != mat.String() {
		t.Fatalf("sift materialId = %q, want %s", eq[0].MaterialID, mat)
	}
	if eq[1].MaterialID != "" {
		t.Fatalf("craap (no evaluates edge) materialId = %q, want empty", eq[1].MaterialID)
	}
}

func TestProjectCoach_MergesStudentMessages(t *testing.T) {
	t1 := time.Now()
	d := ProjectData{
		Interventions: []sqlc.Intervention{
			{Body: "图上有一处孤儿证据", Anchor: []byte(`{"label":"孤儿证据"}`), Type: "flag", CreatedAt: t1},
		},
		ChatMessages: []sqlc.ChatMessage{
			{Role: "user", Content: "它想证明中国在认真转型", CreatedAt: t1.Add(time.Second)},
		},
	}
	coach := projectCoach(d, "论证构建")
	if len(coach.Messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(coach.Messages))
	}
	// time order: flag first, then the student bubble.
	if coach.Messages[0].Kind != "flag" || coach.Messages[1].Kind != "student" || coach.Messages[1].Body != "它想证明中国在认真转型" {
		t.Fatalf("merged = %+v", coach.Messages)
	}
}

// TestProjectDerivesMaterialState is the slice's central claim: a material's
// dossier state (locked/role/tier/takeaway/anchors) is DERIVED from what the
// student actually did, never decorated. matA has earned an evaluated-as
// edge (CRAAP mint) + a source-log entry + a targeting anchor; matB is
// untouched and must carry none of that state.
func TestProjectDerivesMaterialState(t *testing.T) {
	matA := uuid.MustParse("00000000-0000-0000-0000-0000000000aa") // evaluated
	matB := uuid.MustParse("00000000-0000-0000-0000-0000000000bb") // untouched
	evid := uuid.MustParse("00000000-0000-0000-0000-0000000000cc")

	sk := writingSkill(t)
	d := ProjectData{
		Plan: planNode(`["decode_task"]`),
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

	proj, err := Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(proj.Materials) != 2 {
		t.Fatalf("materials = %d, want 2", len(proj.Materials))
	}

	a := proj.Materials[0]
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

	b := proj.Materials[1]
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

// TestProjectActiveCard_NilWhenNoneOpen — the ordinary case: every
// card_instance is terminal (completed/skipped) or there are none at all.
// The reload bug this guards against only exists when a card is left
// proposed/active with nothing projecting it — with none open, ActiveCard
// must be nil (never a stale terminal card resurrected on the client).
func TestProjectActiveCard_NilWhenNoneOpen(t *testing.T) {
	sk := writingSkill(t)
	d := ProjectData{
		Plan: planNode(`["decode_task"]`),
		Cards: []sqlc.CardInstance{
			{ID: uuid.New(), CardID: "craap", Status: "completed"},
			{ID: uuid.New(), CardID: "sift", Status: "skipped"},
		},
	}
	proj, err := Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatal(err)
	}
	if proj.ActiveCard != nil {
		t.Fatalf("ActiveCard = %+v, want nil (no proposed/active card_instance)", proj.ActiveCard)
	}
}

// TestProjectActiveCard_ProjectsTheOpenCard is this fix's central claim: a
// page reload must be able to rehydrate the open card_instance (status
// proposed OR active) — its id, card id, status, anchors, and the material
// it evaluates (via the same card_instance--evaluates-->material edge
// projectEquipment already reads, whole-branch review finding [5]) — so the
// client never has to guess it and FIX-D's project-wide suppression
// (agent.SurfaceCardCandidates) never becomes permanent.
func TestProjectActiveCard_ProjectsTheOpenCard(t *testing.T) {
	sk := writingSkill(t)
	sift := uuid.New()
	mat := uuid.New()
	d := ProjectData{
		Plan: planNode(`["decode_task"]`),
		Cards: []sqlc.CardInstance{
			{ID: sift, CardID: "sift", Status: "active",
				Anchors: []byte(`[{"id":"stop","material_id":"` + mat.String() + `","dimension":"stop","answer":"有点意外"}]`)},
		},
		Edges: []sqlc.GraphEdge{
			{Type: "evaluates", FromKind: "card_instance", FromID: sift, ToKind: "material", ToID: mat},
		},
	}
	proj, err := Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatal(err)
	}
	if proj.ActiveCard == nil {
		t.Fatal("ActiveCard = nil, want the open sift card projected")
	}
	if proj.ActiveCard.CardInstanceID != sift.String() || proj.ActiveCard.CardID != "sift" || proj.ActiveCard.Status != "active" {
		t.Fatalf("ActiveCard = %+v", proj.ActiveCard)
	}
	if proj.ActiveCard.MaterialID != mat.String() {
		t.Fatalf("ActiveCard.MaterialID = %q, want %s (from the evaluates edge)", proj.ActiveCard.MaterialID, mat)
	}
	if len(proj.ActiveCard.Anchors) != 1 {
		t.Fatalf("ActiveCard.Anchors = %d, want 1 (the card's own persisted anchors)", len(proj.ActiveCard.Anchors))
	}
}

// TestProjectActiveCard_IgnoresTerminalCardsEvenAlongsideNone covers a
// "proposed" status too (offered, not yet opened) — the client must be able
// to rehydrate a proposal bubble on reload just as much as an opened card.
func TestProjectActiveCard_ProjectsProposedToo(t *testing.T) {
	craap := uuid.New()
	d := ProjectData{
		Cards: []sqlc.CardInstance{
			{ID: uuid.New(), CardID: "old", Status: "completed"},
			{ID: craap, CardID: "craap", Status: "proposed"},
		},
	}
	got := projectActiveCard(d, map[string]string{})
	if got == nil || got.Status != "proposed" || got.CardID != "craap" {
		t.Fatalf("ActiveCard = %+v, want the proposed craap card", got)
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
// about right now).
func TestProjectExcludesAnchorsFromSkippedCards(t *testing.T) {
	mat := uuid.MustParse("00000000-0000-0000-0000-0000000000dd")

	sk := writingSkill(t)
	d := ProjectData{
		Plan: planNode(`["decode_task"]`),
		Materials: []sqlc.Material{
			{ID: mat, Title: "《卫星图看中国变绿》", Kind: "article", Source: "fetched",
				Blocks: []byte(`[{"id":"b1","text":"过去二十年……"}]`)},
		},
		Cards: []sqlc.CardInstance{
			{CardID: "craap", Status: "skipped",
				Anchors: []byte(`[{"id":"a1","material_id":"` + mat.String() + `","block_id":"b1","start":0,"end":4,"quote":"过去二十年","dimension":"authority","author":"ai","question":"原始出处是谁？","answer":""}]`)},
		},
	}

	proj, err := Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(proj.Materials) != 1 {
		t.Fatalf("materials = %d, want 1", len(proj.Materials))
	}
	if len(proj.Materials[0].Anchors) != 0 {
		t.Errorf("Anchors = %d, want 0 — a skipped card's anchors must not persist onto the article", len(proj.Materials[0].Anchors))
	}
}

func TestProjectStructure(t *testing.T) {
	textNode := func(typ, text string) sqlc.GraphNode {
		return sqlc.GraphNode{Type: typ, Body: []byte(`{"text":"` + text + `"}`)}
	}
	order := []string{"claim", "warrant", "evidence", "counter", "concession"}

	// (a) no Toulmin nodes -> empty slice (pane keeps its placeholder).
	if got := projectStructure(cards.ByID, ProjectData{}); len(got) != 0 {
		t.Fatalf("no nodes: want empty, got %d cards", len(got))
	}

	// (b) all five slot nodes minted -> five done cards in spec-slot order.
	d := ProjectData{Nodes: []sqlc.GraphNode{
		textNode("claim", "主张句"), textNode("warrant", "理据句"),
		textNode("evidence", "证据句"), textNode("counter", "反方句"),
		textNode("concession", "让步句"),
	}}
	got := projectStructure(cards.ByID, d)
	if len(got) != 5 {
		t.Fatalf("five nodes: want 5 cards, got %d", len(got))
	}
	previews := map[string]string{"claim": "主张句", "warrant": "理据句", "evidence": "证据句", "counter": "反方句", "concession": "让步句"}
	for i, c := range got {
		if c.ID != order[i] {
			t.Fatalf("card %d id = %q, want %q (slot order)", i, c.ID, order[i])
		}
		if c.Status != "done" {
			t.Fatalf("card %s status = %q, want done", c.ID, c.Status)
		}
		if c.Preview != previews[c.ID] {
			t.Fatalf("card %s preview = %q, want %q", c.ID, c.Preview, previews[c.ID])
		}
		if c.Role == "" {
			t.Fatalf("card %s has empty role label", c.ID)
		}
	}

	// (c) a CRAAP evidence node (source_quality, no text) with no Toulmin
	// nodes -> still empty (the evidence slot is NOT falsely done).
	craap := ProjectData{Nodes: []sqlc.GraphNode{{Type: "evidence", Body: []byte(`{"source_quality":{"risk_note":"x"}}`)}}}
	if got := projectStructure(cards.ByID, craap); len(got) != 0 {
		t.Fatalf("craap-only: want empty, got %d cards", len(got))
	}

	// (d) claim + evidence minted, rest absent -> those two done, rest empty,
	// list present.
	partial := ProjectData{Nodes: []sqlc.GraphNode{textNode("claim", "主张句"), textNode("evidence", "证据句")}}
	got = projectStructure(cards.ByID, partial)
	if len(got) != 5 {
		t.Fatalf("partial: want 5 cards, got %d", len(got))
	}
	doneSet := map[string]bool{"claim": true, "evidence": true}
	for _, c := range got {
		wantDone := doneSet[c.ID]
		if wantDone && c.Status != "done" {
			t.Fatalf("partial: card %s status = %q, want done", c.ID, c.Status)
		}
		if !wantDone && c.Status != "empty" {
			t.Fatalf("partial: card %s status = %q, want empty", c.ID, c.Status)
		}
	}

	// (e) specByID reports the toulmin spec not found -> empty slice (the
	// `!ok` half of the guard).
	notFound := func(string) (cards.Spec, bool) { return cards.Spec{}, false }
	if got := projectStructure(notFound, d); len(got) != 0 {
		t.Fatalf("spec not found: want empty, got %d cards", len(got))
	}

	// (f) specByID reports the toulmin spec found but with no slots -> empty
	// slice (the `len(spec.Params.Slots)==0` half of the guard).
	noSlots := func(string) (cards.Spec, bool) {
		return cards.Spec{Params: cards.Params{Slots: nil}}, true
	}
	if got := projectStructure(noSlots, d); len(got) != 0 {
		t.Fatalf("spec with no slots: want empty, got %d cards", len(got))
	}
}

// TestProjectWriting is the pure-derivation unit test for the S5 写作 view: no
// buffer, no snapshot, no dispositions, no review interventions yet — the
// projection must not invent any of them, but the word budget still comes
// straight off the skill (it needs no persisted state at all).
func TestProjectWriting(t *testing.T) {
	sk := writingSkill(t)
	d := ProjectData{Project: sqlc.Project{}}
	pw := projectWriting(sk, d)
	if pw.Buffer != "" || pw.LatestSnapshot != nil || len(pw.Review.Items) > 0 {
		t.Fatalf("empty project writing = %+v", pw)
	}
	if pw.WordBudget.Min != 1500 || pw.WordBudget.Max != 2000 {
		t.Fatalf("word budget = %+v", pw.WordBudget)
	}
}

// TestProjectWriting_VoiceAndBudget asserts the projection tags each review
// item with the examiner voice its anchor carries (missing voice key ->
// "board", Task 3) and fills the snapshot's deterministic budget verdict
// (agent.BudgetVerdict, Task 2) alongside the existing InBand bool.
func TestProjectWriting_VoiceAndBudget(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	snapID := uuid.New()
	// One review_item with an explicit sceptic anchor, one keystone-style row
	// with no anchor voice (→ board).
	mk := func(voice string) sqlc.Intervention {
		anchor := map[string]string{"kind": "draft_snapshot", "id": snapID.String()}
		if voice != "" {
			anchor["voice"] = voice
		}
		b, _ := json.Marshal(anchor)
		body, _ := json.Marshal(agent.ReviewItem{CriterionCode: "表E", CriterionName: "分析", Band: "5–6 段", Evidence: "e"})
		return sqlc.Intervention{ID: uuid.New(), Type: "review_item", Anchor: b, Body: string(body)}
	}
	d := ProjectData{
		LatestSnapshot: &sqlc.DraftSnapshot{ID: snapID, Seq: 3, Content: strings.Repeat("字", 2340), CreatedAt: time.Now()},
		Interventions:  []sqlc.Intervention{mk("sceptic"), mk("")},
	}
	out := projectWriting(sk, d)
	if out.LatestSnapshot == nil || out.LatestSnapshot.Budget.State != "over" || out.LatestSnapshot.Budget.Delta != 340 {
		t.Fatalf("budget = %+v, want state=over delta=340", out.LatestSnapshot)
	}
	voices := map[string]int{}
	for _, it := range out.Review.Items {
		voices[it.Voice]++
	}
	if voices["sceptic"] != 1 || voices["board"] != 1 {
		t.Fatalf("voice tags = %v, want one sceptic + one board", voices)
	}
}

// reviewItemIntervention builds one review_item intervention row anchored to
// snapID, mirroring TestProjectWriting_VoiceAndBudget's `mk` helper: the
// anchor carries {kind:"draft_snapshot",id,voice} (voice key omitted ->
// board), the body is the full marshalled agent.ReviewItem (Task 2's Points
// included).
func reviewItemIntervention(snapID uuid.UUID, voice, code, name string, points int) sqlc.Intervention {
	anchor := map[string]string{"kind": "draft_snapshot", "id": snapID.String()}
	if voice != "" {
		anchor["voice"] = voice
	}
	b, _ := json.Marshal(anchor)
	body, _ := json.Marshal(agent.ReviewItem{
		CriterionCode: code, CriterionName: name, Points: points, Missing: "结论段未回应让步",
	})
	return sqlc.Intervention{ID: uuid.New(), Type: "review_item", Anchor: b, Body: string(body)}
}

// TestProjectReadiness_LitFromBoardReview: a board-voice review_item for 表D
// with points 3 (config total 4) lights a partial gauge; the other three
// criteria stay at their pre-review empty default (0/N).
func TestProjectReadiness_LitFromBoardReview(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	snapID := uuid.New()
	d := ProjectData{
		LatestSnapshot: &sqlc.DraftSnapshot{ID: snapID, Seq: 1, Content: "draft", CreatedAt: time.Now()},
		Interventions:  []sqlc.Intervention{reviewItemIntervention(snapID, "", "表D", "来源与证据", 3)},
	}
	proj, err := Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatal(err)
	}
	g := gaugeByCode(proj.Readiness, "表D")
	if g.Lit != 3 || g.Total != 4 || g.Level != "partial" {
		t.Fatalf("表D want 3/4 partial, got %+v", g)
	}
	if g.Note == "" {
		t.Fatalf("表D want the review's missing note carried through, got %+v", g)
	}
	if e := gaugeByCode(proj.Readiness, "表E"); e.Lit != 0 || e.Level != "empty" {
		t.Fatalf("表E want 0 empty, got %+v", e)
	}
	if len(proj.Readiness) != 4 {
		t.Fatalf("want 4 gauges, got %d", len(proj.Readiness))
	}
}

// TestProjectReadiness_BoardVoiceOnly: a SCEPTIC-voice review_item never
// lights readiness — coaching-lens voices are not the assessment of record.
func TestProjectReadiness_BoardVoiceOnly(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	snapID := uuid.New()
	d := ProjectData{
		LatestSnapshot: &sqlc.DraftSnapshot{ID: snapID, Seq: 1, Content: "draft", CreatedAt: time.Now()},
		Interventions:  []sqlc.Intervention{reviewItemIntervention(snapID, "sceptic", "表D", "来源与证据", 4)},
	}
	proj, err := Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatal(err)
	}
	if g := gaugeByCode(proj.Readiness, "表D"); g.Lit != 0 || g.Level != "empty" {
		t.Fatalf("表D want 0 empty (sceptic ignored), got %+v", g)
	}
}

// TestProjectReadiness_EmptyBeforeReview: with a latest snapshot but no
// review_item interventions at all, the gauge set still comes back as the
// full 4-criterion config set, all unlit — the display is stable and honest
// before any review ever runs.
func TestProjectReadiness_EmptyBeforeReview(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	d := ProjectData{
		LatestSnapshot: &sqlc.DraftSnapshot{ID: uuid.New(), Seq: 1, Content: "draft", CreatedAt: time.Now()},
	}
	proj, err := Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(proj.Readiness) != 4 {
		t.Fatalf("want 4 config gauges pre-review, got %d", len(proj.Readiness))
	}
	for _, g := range proj.Readiness {
		if g.Lit != 0 || g.Level != "empty" || g.Total < 1 {
			t.Fatalf("pre-review gauge should be 0/N empty, got %+v", g)
		}
	}
}

// TestProjectReadiness_ClampsAndFull: an out-of-range points value (9, table
// total 3) clamps to the table total and reads as "full" — the projection is
// the single clamp site (Task 2 deliberately left the stored value
// un-clamped).
func TestProjectReadiness_ClampsAndFull(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	snapID := uuid.New()
	d := ProjectData{
		LatestSnapshot: &sqlc.DraftSnapshot{ID: snapID, Seq: 1, Content: "draft", CreatedAt: time.Now()},
		Interventions:  []sqlc.Intervention{reviewItemIntervention(snapID, "board", "表F", "评估", 9)},
	}
	proj, err := Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatal(err)
	}
	if g := gaugeByCode(proj.Readiness, "表F"); g.Lit != 3 || g.Level != "full" {
		t.Fatalf("表F want 3/3 full (clamped), got %+v", g)
	}
}

func gaugeByCode(gs []GaugeDTO, code string) GaugeDTO {
	for _, g := range gs {
		if g.Code == code {
			return g
		}
	}
	return GaugeDTO{}
}

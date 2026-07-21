package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/cards"
)

func newCardFakeStore() *fakeAgentStore {
	return &fakeAgentStore{cardInstances: map[uuid.UUID]CardInstanceRow{}}
}

func TestSurfaceCard_CreatesProposedInstanceAndEdge(t *testing.T) {
	store := newCardFakeStore()
	deps := AgentDeps{Store: store}
	projectID := uuid.New()
	materialID := uuid.New()
	spec := craapSpecFixture()

	action, err := SurfaceCard(context.Background(), deps, projectID, spec, materialID)
	if err != nil {
		t.Fatalf("SurfaceCard: %v", err)
	}
	if action == nil || action.Kind != "surface_card" {
		t.Fatalf("unexpected action: %+v", action)
	}
	if action.CardInstanceID == "" {
		t.Fatal("expected a non-empty CardInstanceID")
	}
	if store.createCardInstanceCalls != 1 {
		t.Fatalf("want CreateCardInstance called once, got %d", store.createCardInstanceCalls)
	}
	if store.lastCreateCardInstance.CardID != "craap" || store.lastCreateCardInstance.MaterialID != materialID {
		t.Fatalf("unexpected CreateCardInstance call: %+v", store.lastCreateCardInstance)
	}
	if store.insertGraphEdgeCalls != 1 {
		t.Fatalf("want InsertGraphEdge called once, got %d", store.insertGraphEdgeCalls)
	}
	if store.lastGraphEdge.FromKind != "card_instance" || store.lastGraphEdge.FromID != action.CardInstanceID ||
		store.lastGraphEdge.ToKind != "material" || store.lastGraphEdge.ToID != materialID.String() {
		t.Fatalf("unexpected mint edge: %+v", store.lastGraphEdge)
	}
	if store.appendEventCalls != 1 || store.lastEvent.Type != "card_surfaced" {
		t.Fatalf("unexpected event: calls=%d event=%+v", store.appendEventCalls, store.lastEvent)
	}

	row := store.cardInstances[uuid.MustParse(action.CardInstanceID)]
	if row.Status != "proposed" {
		t.Fatalf("Status = %q, want proposed", row.Status)
	}
}

func TestCompleteCard_CompleteAppliesGraphEffectsAndFramework(t *testing.T) {
	store := newCardFakeStore()
	deps := AgentDeps{Store: store}
	cardInstanceID := uuid.New()
	materialID := uuid.New()

	spec := craapSpecFixture()
	spec.GraphEffects = []cards.GraphEffect{{Kind: "promote", From: "material", To: "evidence", With: "source_quality"}}
	spec.Consolidation = "reveal_framework_after_completion"

	anchors := completeAnchors()
	for i := range anchors {
		anchors[i].MaterialID = materialID.String()
	}
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	store.cardInstances[cardInstanceID] = CardInstanceRow{
		ID:        cardInstanceID,
		ProjectID: uuid.New(),
		CardID:    spec.ID,
		Status:    "active",
		Anchors:   anchorsJSON,
	}

	complete, err := CompleteCard(context.Background(), deps, spec, cardInstanceID)
	if err != nil {
		t.Fatalf("CompleteCard: %v", err)
	}
	if !complete {
		t.Fatal("want complete = true")
	}
	if store.insertGraphNodeCalls != 1 {
		t.Fatalf("want one minted evidence node, got %d", store.insertGraphNodeCalls)
	}
	if store.insertGraphEdgeCalls != 1 {
		t.Fatalf("want one minted evaluated-as edge, got %d", store.insertGraphEdgeCalls)
	}
	if store.lastGraphEdge.Type != "evaluated-as" || store.lastGraphEdge.FromID != materialID.String() {
		t.Fatalf("unexpected mint edge: %+v", store.lastGraphEdge)
	}
	if store.setFrameworkCalls != 1 {
		t.Fatalf("want SetCardInstanceFramework called once, got %d", store.setFrameworkCalls)
	}
	var framework map[string]any
	if err := json.Unmarshal(store.lastFramework, &framework); err != nil {
		t.Fatalf("unmarshal framework: %v", err)
	}
	if framework["strategy"] != "reveal_framework_after_completion" {
		t.Fatalf("framework = %v, missing strategy", framework)
	}

	row := store.cardInstances[cardInstanceID]
	if row.Status != "active" {
		t.Fatalf("Status = %q, want unchanged (active) — CompleteCard must never set solid/completed", row.Status)
	}
}

func TestCompleteCard_IncompleteIsNoOp(t *testing.T) {
	store := newCardFakeStore()
	deps := AgentDeps{Store: store}
	cardInstanceID := uuid.New()
	spec := craapSpecFixture()

	anchors := completeAnchors()
	var without []Anchor
	for _, a := range anchors {
		if a.Dimension == "authority" {
			continue // drop a required tag -> incomplete
		}
		without = append(without, a)
	}
	anchorsJSON, err := json.Marshal(without)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	store.cardInstances[cardInstanceID] = CardInstanceRow{
		ID: cardInstanceID, ProjectID: uuid.New(), CardID: spec.ID, Status: "active", Anchors: anchorsJSON,
	}

	complete, err := CompleteCard(context.Background(), deps, spec, cardInstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if complete {
		t.Fatal("want complete = false")
	}
	if store.insertGraphNodeCalls != 0 || store.insertGraphEdgeCalls != 0 || store.setFrameworkCalls != 0 {
		t.Fatalf("incomplete card must persist nothing, got nodes=%d edges=%d framework=%d",
			store.insertGraphNodeCalls, store.insertGraphEdgeCalls, store.setFrameworkCalls)
	}
}

func TestRecordDisposition_ShortReasonRejected(t *testing.T) {
	store := newCardFakeStore()
	deps := AgentDeps{Store: store}

	err := RecordDisposition(context.Background(), deps, uuid.New(), "reject", "太短了")
	if err == nil {
		t.Fatal("want an error for a <15-char reason")
	}
	if store.insertDispositionCalls != 0 {
		t.Fatalf("want nothing persisted, got %d calls", store.insertDispositionCalls)
	}
}

func TestRecordDisposition_ValidReasonInserts(t *testing.T) {
	store := newCardFakeStore()
	deps := AgentDeps{Store: store}
	interventionID := uuid.New()
	reason := "这条追问和我原本的方向不一致，我想先按自己的思路推进"
	if len(reason) < 15 {
		t.Fatalf("test fixture reason too short: %d bytes", len(reason))
	}

	if err := RecordDisposition(context.Background(), deps, interventionID, "reject", reason); err != nil {
		t.Fatalf("RecordDisposition: %v", err)
	}
	if store.insertDispositionCalls != 1 {
		t.Fatalf("want InsertDisposition called once, got %d", store.insertDispositionCalls)
	}
	if store.lastDisposition.InterventionID != interventionID || store.lastDisposition.Action != "reject" || store.lastDisposition.Reason != reason {
		t.Fatalf("unexpected disposition: %+v", store.lastDisposition)
	}
}

func TestCompleteCard_IdempotentWhenFrameworkAlreadySet(t *testing.T) {
	store := newCardFakeStore()
	deps := AgentDeps{Store: store}
	cardInstanceID := uuid.New()
	materialID := uuid.New()

	spec := craapSpecFixture()
	spec.GraphEffects = []cards.GraphEffect{{Kind: "promote", From: "material", To: "evidence", With: "source_quality"}}

	anchors := completeAnchors()
	for i := range anchors {
		anchors[i].MaterialID = materialID.String()
	}
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	// framework_fill already populated => the card was already completed.
	store.cardInstances[cardInstanceID] = CardInstanceRow{
		ID: cardInstanceID, ProjectID: uuid.New(), CardID: spec.ID, Status: "active",
		Anchors: anchorsJSON, FrameworkFill: []byte(`{"strategy":"reveal_framework_after_completion"}`),
	}

	complete, err := CompleteCard(context.Background(), deps, spec, cardInstanceID)
	if err != nil {
		t.Fatalf("CompleteCard: %v", err)
	}
	if !complete {
		t.Fatal("want complete = true (already completed)")
	}
	if store.insertGraphNodeCalls != 0 || store.insertGraphEdgeCalls != 0 || store.setFrameworkCalls != 0 {
		t.Fatalf("idempotent re-complete must mint/persist nothing, got nodes=%d edges=%d framework=%d",
			store.insertGraphNodeCalls, store.insertGraphEdgeCalls, store.setFrameworkCalls)
	}
}

func TestRecordDisposition_ShortChineseReasonRejectedByRuneCount(t *testing.T) {
	store := newCardFakeStore()
	deps := AgentDeps{Store: store}
	// 5 Han characters = 15 UTF-8 bytes but only 5 runes — must be rejected
	// (a byte count would have let this clear the ≥15-character gate).
	if err := RecordDisposition(context.Background(), deps, uuid.New(), "reject", "五个汉字哦"); err == nil {
		t.Fatal("want rejection: 5 runes < 15 despite ~15 bytes")
	}
	if store.insertDispositionCalls != 0 {
		t.Fatalf("want nothing persisted, got %d calls", store.insertDispositionCalls)
	}
}

// TestSecondCard_SurfaceAndCompleteWithZeroNewRuntimeCode is agent-spec
// §5.7's acceptance test: a trivial second card (a "note" card over
// `annotate`, one dimension, one every_tag_present predicate, no
// graph_effects) authored inline — never registered in the cards catalog —
// surfaces and completes through the exact same SurfaceCard/CompleteCard
// functions CRAAP uses. No new runtime code.
func TestSecondCard_SurfaceAndCompleteWithZeroNewRuntimeCode(t *testing.T) {
	noteSpec := cards.Spec{
		ID:        "note",
		Primitive: "annotate",
		Params:    cards.Params{Tags: []string{"note"}},
		Completion: []cards.CompletionPredicate{
			{Kind: "every_tag_present", Tags: []string{"note"}},
		},
	}

	store := newCardFakeStore()
	deps := AgentDeps{Store: store}
	projectID := uuid.New()
	materialID := uuid.New()

	action, err := SurfaceCard(context.Background(), deps, projectID, noteSpec, materialID)
	if err != nil {
		t.Fatalf("SurfaceCard: %v", err)
	}
	cardInstanceID := uuid.MustParse(action.CardInstanceID)

	anchors := []Anchor{
		{ID: "a0", MaterialID: materialID.String(), Dimension: "note", Author: "student", Answer: "这段材料值得回头再核对数据来源"},
	}
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	row := store.cardInstances[cardInstanceID]
	row.Anchors = anchorsJSON
	store.cardInstances[cardInstanceID] = row

	complete, err := CompleteCard(context.Background(), deps, noteSpec, cardInstanceID)
	if err != nil {
		t.Fatalf("CompleteCard: %v", err)
	}
	if !complete {
		t.Fatal("want complete = true")
	}
	// No graph_effects declared on this card -> nothing minted beyond
	// SurfaceCard's own card_instance->material edge.
	if store.insertGraphNodeCalls != 0 {
		t.Fatalf("want 0 minted nodes (no graph_effects), got %d", store.insertGraphNodeCalls)
	}
	if store.insertGraphEdgeCalls != 1 {
		t.Fatalf("want 1 edge total (surface-time only), got %d", store.insertGraphEdgeCalls)
	}
	if store.setFrameworkCalls != 1 {
		t.Fatalf("want SetCardInstanceFramework called once, got %d", store.setFrameworkCalls)
	}
}

func TestCheckedMaterialID_IgnoresAnchorOrder(t *testing.T) {
	spec := cards.Spec{ID: "sift", Params: cards.Params{LateralDimension: "find"}}
	// The lateral anchor sorts FIRST — the trap. The checked material must
	// still be the source under review, not the source used to check it.
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-nasa", Dimension: "find", Answer: "NASA 只说绿化面积", Author: "student"},
		{ID: "a2", MaterialID: "mat-blog", Dimension: "stop", Answer: "有点夸张", Author: "student"},
	}
	if got := checkedMaterialID(spec, anchors); got != "mat-blog" {
		t.Fatalf("checked material = %q, want mat-blog (the source under review)", got)
	}
	lat, ok := lateralAnchor(spec, anchors)
	if !ok || lat.MaterialID != "mat-nasa" {
		t.Fatalf("lateral anchor = %+v ok=%v, want mat-nasa", lat, ok)
	}
}

// TestCompleteCard_SiftCrossCheckMarksCheckedSourceNotLateralSource is Task
// 7's review-fix keystone for finding [1]: it drives CompleteCard through the
// REAL "sift" card spec (cards.ByID — LateralDimension="find", a cross_check
// graph_effect) with anchors spanning two materials, and asserts the
// LateralRead CompleteCard builds points at the CHECKED source (the blog
// post under review) — never the LATERAL source (the independent NASA page
// she used as the instrument to check it).
//
// The pre-fix test (TestCommitCardMint_FlipsLateralReadOnTheCheckedSourceOnly,
// internal/store/refactor2_runtime_sqlc_test.go) called store.CommitCardMint
// directly with a hard-coded LateralRead{MaterialID: blog.ID, ...} — it could
// never catch a regression in *how* CompleteCard picks which material that
// struct points at, because the test built the correct answer by hand and
// handed it straight to the write path. This one drives the real selection
// logic in card_lifecycle.go (checkedMaterialID/lateralAnchor) end to end.
func TestCompleteCard_SiftCrossCheckMarksCheckedSourceNotLateralSource(t *testing.T) {
	spec, ok := cards.ByID("sift")
	if !ok {
		t.Fatal("cards.ByID(sift) not found")
	}

	store := newCardFakeStore()
	deps := AgentDeps{Store: store}
	cardInstanceID := uuid.New()
	checkedID := uuid.New() // the suspicious blog post — the source under review
	lateralID := uuid.New() // the independent NASA page she used to check it

	// Seed the checked source's ingestion-time tier, exactly as 6b would have
	// written it — CompleteCard reads this as tier_before before the mint
	// overwrites the row with her post-check re-tier.
	store.sourceLogs = map[uuid.UUID]SourceLogRow{
		checkedID: {Tier: "一手报道"},
	}

	// The lateral anchor ("find") sorts BEFORE the checked-source anchors —
	// the same order trap TestCheckedMaterialID_IgnoresAnchorOrder guards
	// against — so a regression that picks "the first anchor's material"
	// instead of "the non-lateral anchor's material" would be caught here too.
	anchors := []Anchor{
		{ID: "a-find", MaterialID: lateralID.String(), Dimension: "find", Author: "student", Answer: "NASA 数据显示排放仍在上升"},
		{ID: "a-stop", MaterialID: checkedID.String(), Dimension: "stop", Author: "student", Answer: "感觉有点夸张"},
		{ID: "a-investigate", MaterialID: checkedID.String(), Dimension: "investigate", Author: "student", Answer: "个人博客，非机构"},
		{ID: "a-relation", MaterialID: checkedID.String(), Dimension: "relation", Author: "student", Answer: "限定"},
		{ID: "a-trace", MaterialID: checkedID.String(), Dimension: "trace_origin", Author: "student", Answer: "追到 NASA Earth Observatory 原始页面"},
		{ID: "a-tier", MaterialID: checkedID.String(), Dimension: "tier_after", Author: "student", Answer: "二手 · 需追源"},
	}
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	store.cardInstances[cardInstanceID] = CardInstanceRow{
		ID: cardInstanceID, ProjectID: uuid.New(), CardID: spec.ID, Status: "active", Anchors: anchorsJSON,
	}

	complete, err := CompleteCard(context.Background(), deps, spec, cardInstanceID)
	if err != nil {
		t.Fatalf("CompleteCard: %v", err)
	}
	if !complete {
		t.Fatal("want complete = true")
	}

	if store.lateralReadCalls != 1 {
		t.Fatalf("want LateralRead written once, got %d", store.lateralReadCalls)
	}
	if store.lastLateralRead.MaterialID != checkedID {
		t.Fatalf("LateralRead.MaterialID = %s, want the CHECKED source %s (not the lateral instrument %s)",
			store.lastLateralRead.MaterialID, checkedID, lateralID)
	}
	if store.lastLateralRead.TierAfter != "二手 · 需追源" {
		t.Fatalf("LateralRead.TierAfter = %q, want her post-check re-tier", store.lastLateralRead.TierAfter)
	}
	if store.lateralReadMaterials[lateralID] {
		t.Fatal("the LATERAL source is the instrument, not the subject — it must never be marked laterally read")
	}
	if !store.lateralReadMaterials[checkedID] {
		t.Fatal("the CHECKED source must be marked laterally read")
	}
}

// TestCompleteCard_MaterialLessCardCompletes is the whole-branch-review
// CRITICAL 1 regression: every COMPLETE submit of the three new N3a cards
// (sort/scale/matrix) hard-failed on the server, because CompleteCard demanded
// a material-anchored answer unconditionally while those cards correctly write
// material_id "" on every anchor. Perversely, an INCOMPLETE submit worked (it
// returns early at the !complete branch) — the card broke only once the
// student did the whole job.
//
// Every other card test in this package exercises EvaluateCompletion /
// GraphEffects as PURE functions, which is exactly why none of them could see
// this: the guard lives between the two, in CompleteCard. So this drives the
// real "perspective-matrix" spec (cards.ByID) through CompleteCard end to end.
func TestCompleteCard_MaterialLessCardCompletes(t *testing.T) {
	spec, ok := cards.ByID("perspective-matrix")
	if !ok {
		t.Fatal("cards.ByID(perspective-matrix) not found")
	}

	store := newCardFakeStore()
	deps := AgentDeps{Store: store}
	cardInstanceID := uuid.New()

	// Two COMPLETE rows (label = Anchor.Quote, column = Anchor.Dimension),
	// every anchor material-less and student-authored — exactly what
	// matrixStateToAnchors (apps/web/src/primitives/matrix/serialize.ts) writes.
	var anchors []Anchor
	for _, row := range []struct {
		label string
		cells map[string]string
	}{
		{"地方政府", map[string]string{"position": "治理见效，指标逐年改善", "grounds": "本地环境公报的年度数据", "blind_spot": "没算迁出企业转移出去的排放"}},
		{"受影响居民", map[string]string{"position": "空气好了，但生计被砍掉了", "grounds": "关停后本地就业数据下滑", "blind_spot": "看不到全国层面的减排收益"}},
	} {
		for _, col := range spec.Params.Cols {
			anchors = append(anchors, Anchor{
				ID: row.label + "-" + col.ID, Quote: row.label, Dimension: col.ID,
				Author: "student", Answer: row.cells[col.ID], MaterialID: "",
			})
		}
	}
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	store.cardInstances[cardInstanceID] = CardInstanceRow{
		ID: cardInstanceID, ProjectID: uuid.New(), CardID: spec.ID, Status: "active", Anchors: anchorsJSON,
	}

	complete, err := CompleteCard(context.Background(), deps, spec, cardInstanceID)
	if err != nil {
		t.Fatalf("CompleteCard: %v", err)
	}
	if !complete {
		t.Fatal("want complete = true — a material-less card must complete, not error")
	}
	if store.insertGraphNodeCalls != 2 {
		t.Fatalf("want 2 minted perspective nodes, got %d", store.insertGraphNodeCalls)
	}
	for _, n := range store.minted {
		if n.Type != "perspective" {
			t.Fatalf("minted node type = %q, want perspective", n.Type)
		}
		if n.Author != "student" {
			t.Fatalf("minted node author = %q, want student (RL-4)", n.Author)
		}
	}
	// perspectives mints no edges — the nodes are free-standing project nodes.
	if store.insertGraphEdgeCalls != 0 {
		t.Fatalf("want 0 minted edges, got %d", store.insertGraphEdgeCalls)
	}
	if store.setFrameworkCalls != 1 {
		t.Fatalf("want SetCardInstanceFramework called once, got %d", store.setFrameworkCalls)
	}
}

// TestCompleteCard_MaterialConsumingCardStillErrorsWithoutMaterial pins the
// other half of the CRITICAL 1 fix: relaxing the guard must NOT relax it for
// the cards whose graph_effects actually address the checked material. A CRAAP
// (promote) that somehow reaches completion with no material anchor has
// nothing to hang its evaluated-as edge on and must still hard-fail.
func TestCompleteCard_MaterialConsumingCardStillErrorsWithoutMaterial(t *testing.T) {
	spec := craapSpecFixture()
	spec.GraphEffects = []cards.GraphEffect{{Kind: "promote", From: "material", To: "evidence", With: "source_quality"}}

	store := newCardFakeStore()
	deps := AgentDeps{Store: store}
	cardInstanceID := uuid.New()

	anchors := completeAnchors() // complete, but no anchor carries a material_id
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	store.cardInstances[cardInstanceID] = CardInstanceRow{
		ID: cardInstanceID, ProjectID: uuid.New(), CardID: spec.ID, Status: "active", Anchors: anchorsJSON,
	}

	if _, err := CompleteCard(context.Background(), deps, spec, cardInstanceID); err == nil {
		t.Fatal("want an error: a promote effect with no material anchor cannot mint its edge")
	}
	if store.insertGraphNodeCalls != 0 || store.setFrameworkCalls != 0 {
		t.Fatalf("failed completion must persist nothing, got nodes=%d framework=%d",
			store.insertGraphNodeCalls, store.setFrameworkCalls)
	}
}

// TestNeedsMaterialCoversEveryGraphEffectKind is a completeness guard, not a
// bug fix (N3b carry-forward): materialConsumingEffects (above) is a closed
// set hand-maintained against GraphEffects' switch (card_effects.go) — there
// is no compile-time link between the two, so a future graph_effect kind
// added to the switch and forgotten here would break card completion
// silently at runtime (needsMaterial would say "no material required" for a
// kind that actually mints an edge against one, or vice versa).
//
// Whole-branch review MINOR 6: the original version of this test asserted
// against a SECOND hand-written list (wantConsumesMaterial) plus a
// `len(materialConsumingEffects) != 2` count check — so it caught an
// ADDITION to the map (the count would drift) but not the failure it
// actually advertised: a new effect kind arriving in GraphEffects' switch
// (card_effects.go) and simply being omitted from materialConsumingEffects.
// That omission leaves both maps agreeing with each other while a real card
// silently breaks needsMaterial at runtime — and a second hand-written list
// can drift from the real switch exactly as easily as the map under test
// can.
//
// This version reads the SOURCE OF TRUTH instead: every graph_effect kind
// that actually appears in the embedded card catalog (internal/cards,
// Go-embedded JSON — the only place a new kind can arrive from, since "new
// card = new JSON, not new renderer code" is a Global Constraint). Each kind
// found there must be classified in materialConsumingEffects; a new kind
// shipped via card JSON and never classified now fails HERE instead of
// silently breaking card completion in production.
//
//   - "promote": material --evaluated-as--> evidence — consumes the material.
//   - "cross_check": material --cross-checked-by--> cross_check — consumes it.
//   - "toulmin": mints slot nodes + cites edges addressed to PER-ANCHOR
//     materials (each source cited from a slot), never the checked material
//     itself — does not consume it.
//   - "perspectives": mints free-standing project nodes with no edges at
//     all — does not consume it.
func TestNeedsMaterialCoversEveryGraphEffectKind(t *testing.T) {
	knownClassification := map[string]bool{
		"promote":      true,
		"cross_check":  true,
		"toulmin":      false,
		"perspectives": false,
	}

	catalog, err := cards.Catalog()
	if err != nil {
		t.Fatalf("load embedded card catalog: %v", err)
	}
	seen := map[string]bool{}
	for _, spec := range catalog {
		for _, effect := range spec.GraphEffects {
			seen[effect.Kind] = true
			want, ok := knownClassification[effect.Kind]
			if !ok {
				t.Fatalf("card %q ships graph_effect kind %q with NO known classification in materialConsumingEffects — "+
					"a real card just shipped a kind this test's completeness net does not cover", spec.ID, effect.Kind)
			}
			if got := materialConsumingEffects[effect.Kind]; got != want {
				t.Fatalf("materialConsumingEffects[%q] = %v, want %v (card %q)", effect.Kind, got, want, spec.ID)
			}
		}
	}
	// Today's four kinds must all actually be exercised by the catalog —
	// otherwise this test would silently stop testing a kind the moment its
	// one card were deleted, without anyone noticing.
	for kind := range knownClassification {
		if !seen[kind] {
			t.Fatalf("no embedded card ships graph_effect kind %q — this test's coverage claim is stale", kind)
		}
	}
}

func TestCheckedMaterialID_CardWithoutLateralDimension_Unchanged(t *testing.T) {
	// CRAAP and every card that exists today: no lateral_dimension, so this
	// must degenerate to exactly the old first-anchor behavior.
	spec := cards.Spec{ID: "craap"}
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "authority", Answer: "机构", Author: "student"},
	}
	if got := checkedMaterialID(spec, anchors); got != "mat-blog" {
		t.Fatalf("checked material = %q, want mat-blog", got)
	}
	if _, ok := lateralAnchor(spec, anchors); ok {
		t.Fatal("a card with no lateral_dimension must have no lateral anchor")
	}
}

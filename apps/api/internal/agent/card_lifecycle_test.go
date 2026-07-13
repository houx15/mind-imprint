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

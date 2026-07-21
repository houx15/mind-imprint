package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// fakeAgentStore is an in-memory AgentStore double for the loop unit tests —
// no DB, records calls so tests can assert exactly what got persisted.
type fakeAgentStore struct {
	graph GraphView

	insertInterventionCalls int
	lastIntervention        InterventionRow

	appendEventCalls int
	lastEvent        EventRow

	cardInstances map[uuid.UUID]CardInstanceRow

	createCardInstanceCalls int
	lastCreateCardInstance  struct {
		ProjectID   uuid.UUID
		MaterialID  uuid.UUID
		CardID      string
		ContractRef string
	}

	insertGraphNodeCalls int
	insertGraphEdgeCalls int
	lastGraphEdge        MintEdge

	setFrameworkCalls int
	lastFramework     []byte

	insertDispositionCalls int
	lastDisposition        struct {
		InterventionID uuid.UUID
		Action         string
		Reason         string
	}

	gateStates        map[string]RecordedGate // keyed by contract
	upsertPlanCalls   int
	lastPlanBody      []byte
	gateAttemptEvents []EventRow

	minted             []GraphNodeView
	lastImportedAuthor string

	chatTurns              []ChatTurn
	createChatMessageCalls int

	setStatusCalls   int
	lastStatus       string
	setAnchorsCalls  int
	lastAnchorsWrite []byte
	submitCardCalls  int
	lastFieldValues  []byte
	lastEventTrace   []byte

	recordLLMCallCalls int
	lastLLMCall        LLMCallRow

	insertReviewInterventionCalls int
	lastReviewIntervention        ReviewInterventionRow

	// sourceLogs backs GetSourceLogByMaterial and is mutated in-memory by
	// CommitCardMint's LateralRead handling below — a fake that no-opped the
	// write would let a test assert a re-tier "succeeded" while reading back
	// the untouched ingestion-time tier, silently masking the exact drift
	// Task 7 exists to prevent.
	sourceLogs map[uuid.UUID]SourceLogRow

	lateralReadCalls     int
	lastLateralRead      LateralRead
	lateralReadMaterials map[uuid.UUID]bool
}

func (f *fakeAgentStore) LoadGraph(context.Context, uuid.UUID) (GraphView, error) {
	g := f.graph
	if len(f.minted) > 0 {
		g.Nodes = append(append([]GraphNodeView{}, g.Nodes...), f.minted...)
	}
	return g, nil
}

func (f *fakeAgentStore) InsertIntervention(_ context.Context, row InterventionRow) (uuid.UUID, error) {
	f.insertInterventionCalls++
	f.lastIntervention = row
	return uuid.New(), nil
}

func (f *fakeAgentStore) AppendEvent(_ context.Context, row EventRow) error {
	f.appendEventCalls++
	f.lastEvent = row
	return nil
}

// CreateChatMessage/LoadChatHistory: a minimal in-memory double — append to a
// slice, return the stored user turns. Interventions merge is exercised for
// real by the sqlc-backed TestSqlcAgentStore_ChatHistoryMerge; the fake never
// needs to fabricate an intervention side, since no loop test asserts on
// merged history yet.
func (f *fakeAgentStore) CreateChatMessage(_ context.Context, _ uuid.UUID, role, content string) error {
	f.createChatMessageCalls++
	f.chatTurns = append(f.chatTurns, ChatTurn{Role: role, Content: content})
	return nil
}

func (f *fakeAgentStore) LoadChatHistory(_ context.Context, _ uuid.UUID, limit int) ([]ChatTurn, error) {
	turns := f.chatTurns
	if limit > 0 && len(turns) > limit {
		turns = turns[len(turns)-limit:]
	}
	out := make([]ChatTurn, len(turns))
	copy(out, turns)
	return out, nil
}

func (f *fakeAgentStore) CreateCardInstance(_ context.Context, projectID, materialID uuid.UUID, cardID, contractRef string) (CardInstanceRow, error) {
	f.createCardInstanceCalls++
	f.lastCreateCardInstance.ProjectID = projectID
	f.lastCreateCardInstance.MaterialID = materialID
	f.lastCreateCardInstance.CardID = cardID
	f.lastCreateCardInstance.ContractRef = contractRef
	row := CardInstanceRow{ID: uuid.New(), ProjectID: projectID, CardID: cardID, Status: "proposed"}
	if f.cardInstances == nil {
		f.cardInstances = map[uuid.UUID]CardInstanceRow{}
	}
	f.cardInstances[row.ID] = row
	return row, nil
}

func (f *fakeAgentStore) GetCardInstance(_ context.Context, id uuid.UUID) (CardInstanceRow, error) {
	row, ok := f.cardInstances[id]
	if !ok {
		return CardInstanceRow{}, fmt.Errorf("fakeAgentStore: no card_instance %s", id)
	}
	return row, nil
}

// GetSourceLogByMaterial reads the fake's in-memory source-log stand-in.
// Tests seed f.sourceLogs directly (no Create* call exists on the fake); a
// material with no seeded entry behaves like a real missing row — an error,
// which CompleteCard treats as tier_before simply absent, not fatal.
func (f *fakeAgentStore) GetSourceLogByMaterial(_ context.Context, materialID uuid.UUID) (SourceLogRow, error) {
	row, ok := f.sourceLogs[materialID]
	if !ok {
		return SourceLogRow{}, fmt.Errorf("fakeAgentStore: no source_log for material %s", materialID)
	}
	return row, nil
}

func (f *fakeAgentStore) SetCardInstanceFramework(_ context.Context, _, id uuid.UUID, framework []byte) error {
	f.setFrameworkCalls++
	f.lastFramework = framework
	row := f.cardInstances[id]
	row.FrameworkFill = framework
	f.cardInstances[id] = row
	return nil
}

func (f *fakeAgentStore) SetCardInstanceStatus(_ context.Context, _, id uuid.UUID, status string) error {
	f.setStatusCalls++
	f.lastStatus = status
	row := f.cardInstances[id]
	row.Status = status
	f.cardInstances[id] = row
	return nil
}

func (f *fakeAgentStore) SetCardInstanceAnchors(_ context.Context, _, id uuid.UUID, anchors []byte) error {
	f.setAnchorsCalls++
	f.lastAnchorsWrite = anchors
	row := f.cardInstances[id]
	row.Anchors = anchors
	f.cardInstances[id] = row
	return nil
}

func (f *fakeAgentStore) SubmitProjectCardInstance(_ context.Context, _, id uuid.UUID, fieldValues, eventTrace []byte) error {
	f.submitCardCalls++
	f.lastFieldValues = fieldValues
	f.lastEventTrace = eventTrace
	// CardInstanceRow carries no FieldValues/EventTrace (agent-loop-only
	// view, mirroring the real adapter's toCardInstanceRow) — recorded on
	// the fake for assertions only, not round-tripped through the map.
	return nil
}

func (f *fakeAgentStore) InsertGraphNode(_ context.Context, _ uuid.UUID, node MintNode) (uuid.UUID, error) {
	f.insertGraphNodeCalls++
	f.lastImportedAuthor = node.Author
	id := uuid.New()
	text, _ := node.Body["text"].(string)
	f.minted = append(f.minted, GraphNodeView{
		ID:     id.String(),
		Type:   node.Type,
		Author: node.Author,
		Text:   text,
	})
	return id, nil
}

func (f *fakeAgentStore) InsertGraphEdge(_ context.Context, _ uuid.UUID, edge MintEdge) error {
	f.insertGraphEdgeCalls++
	f.lastGraphEdge = edge
	return nil
}

// CommitCardMint is the fake's in-memory stand-in for the real transaction
// (agentstore.go): no DB, so no atomicity to prove here — that guarantee is
// exercised against a real Postgres in
// internal/store/refactor2_runtime_sqlc_test.go. This just has to apply the
// same node/edge/framework writes the real method does, through the fake's
// own InsertGraphNode/InsertGraphEdge/SetCardInstanceFramework, so every
// existing CRAAP mint test keeps asserting on insertGraphNodeCalls /
// lastGraphEdge / lastFramework exactly as before CompleteCard switched to
// calling this instead of the three methods directly.
func (f *fakeAgentStore) CommitCardMint(ctx context.Context, projectID, cardInstanceID uuid.UUID, m CardMint) error {
	nodeIDs := make(map[int]uuid.UUID, len(m.Nodes))
	for i, n := range m.Nodes {
		id, err := f.InsertGraphNode(ctx, projectID, n)
		if err != nil {
			return err
		}
		nodeIDs[i] = id
	}
	for _, e := range m.Edges {
		fromID, err := resolveMintRef(e.FromID, nodeIDs)
		if err != nil {
			return err
		}
		toID, err := resolveMintRef(e.ToID, nodeIDs)
		if err != nil {
			return err
		}
		e.FromID, e.ToID = fromID.String(), toID.String()
		if err := f.InsertGraphEdge(ctx, projectID, e); err != nil {
			return err
		}
	}
	if m.LateralRead != nil {
		f.lateralReadCalls++
		f.lastLateralRead = *m.LateralRead
		if f.lateralReadMaterials == nil {
			f.lateralReadMaterials = map[uuid.UUID]bool{}
		}
		f.lateralReadMaterials[m.LateralRead.MaterialID] = true
		// Mirror the real MarkSourceLateralRead SQL's CASE: an empty
		// TierAfter leaves the ingestion-time tier alone.
		if f.sourceLogs == nil {
			f.sourceLogs = map[uuid.UUID]SourceLogRow{}
		}
		row := f.sourceLogs[m.LateralRead.MaterialID]
		if m.LateralRead.TierAfter != "" {
			row.Tier = m.LateralRead.TierAfter
		}
		f.sourceLogs[m.LateralRead.MaterialID] = row
	}
	return f.SetCardInstanceFramework(ctx, projectID, cardInstanceID, m.Framework)
}

func (f *fakeAgentStore) InsertDisposition(_ context.Context, interventionID uuid.UUID, action, reason string) (uuid.UUID, error) {
	f.insertDispositionCalls++
	f.lastDisposition.InterventionID = interventionID
	f.lastDisposition.Action = action
	f.lastDisposition.Reason = reason
	return uuid.New(), nil
}

func (f *fakeAgentStore) InsertReviewIntervention(_ context.Context, row ReviewInterventionRow) error {
	f.insertReviewInterventionCalls++
	f.lastReviewIntervention = row
	return nil
}

func (f *fakeAgentStore) ListGateStates(context.Context, uuid.UUID) (map[string]RecordedGate, error) {
	if f.gateStates == nil {
		return map[string]RecordedGate{}, nil
	}
	return f.gateStates, nil
}

func (f *fakeAgentStore) UpsertGateState(_ context.Context, _ uuid.UUID, contract string, rec RecordedGate) error {
	if f.gateStates == nil {
		f.gateStates = map[string]RecordedGate{}
	}
	f.gateStates[contract] = rec
	return nil
}

func (f *fakeAgentStore) UpsertPlan(_ context.Context, _ uuid.UUID, body []byte) error {
	f.upsertPlanCalls++
	f.lastPlanBody = body
	return nil
}

func (f *fakeAgentStore) RecordLLMCall(_ context.Context, row LLMCallRow) error {
	f.recordLLMCallCalls++
	f.lastLLMCall = row
	return nil
}

func TestLoop_UnsupportedClaimEmitsInterventionAndPersistsOnce(t *testing.T) {
	g := GraphView{
		Nodes: []GraphNodeView{
			{ID: "n1", Type: "claim", Author: "student", Text: "中国的经济转型正在让地球更可持续"},
		},
	}
	store := &fakeAgentStore{graph: g}
	body := "这条主张现在还没有素材支撑——它的证据是什么？"
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider(body),
		Resolved: testResolved,
		Sim:      constSim(0.0), // low similarity: no declarative-echo intercept
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action == nil {
		t.Fatal("expected a non-nil Action")
	}
	if action.Output.Body != body {
		t.Fatalf("want body %q, got %q", body, action.Output.Body)
	}
	if action.Output.Anchor.Kind != "graph_node" || action.Output.Anchor.ID != "n1" {
		t.Fatalf("unexpected anchor: %+v", action.Output.Anchor)
	}
	if action.InterventionID == "" {
		t.Fatal("expected a non-empty InterventionID")
	}

	if store.insertInterventionCalls != 1 {
		t.Fatalf("want InsertIntervention called once, got %d", store.insertInterventionCalls)
	}
	if store.lastIntervention.Body != body || store.lastIntervention.Criterion != "D5" {
		t.Fatalf("unexpected persisted intervention: %+v", store.lastIntervention)
	}

	if store.appendEventCalls != 1 {
		t.Fatalf("want AppendEvent called once, got %d", store.appendEventCalls)
	}
	if store.lastEvent.Type != "intervention_posted" || store.lastEvent.Surface != "studio" {
		t.Fatalf("unexpected event: %+v", store.lastEvent)
	}
}

func TestLoop_SupportedClaimIsSilentAndPersistsNothing(t *testing.T) {
	g := GraphView{
		Nodes: []GraphNodeView{
			{ID: "n1", Type: "claim", Author: "student"},
			{ID: "e1", Type: "evidence", Author: "student"},
		},
		Edges: []GraphEdgeView{
			{FromKind: "graph_node", FromID: "e1", ToKind: "graph_node", ToID: "n1", Type: "supports"},
		},
	}
	store := &fakeAgentStore{graph: g}
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider("should never be called"),
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != nil {
		t.Fatalf("want silence (nil Action), got %+v", action)
	}
	if store.insertInterventionCalls != 0 || store.appendEventCalls != 0 {
		t.Fatalf("want nothing persisted on silence, got insert=%d append=%d",
			store.insertInterventionCalls, store.appendEventCalls)
	}
}

// TestLoop_EnforcementRejectionIsSilentAndPersistsNothing covers the
// "on enforcement error, log server-side + return silence" branch (design
// §2 act/record) — a banned-phrase output must never reach persistence.
func TestLoop_EnforcementRejectionIsSilentAndPersistsNothing(t *testing.T) {
	g, _ := coachFixture()
	store := &fakeAgentStore{graph: g}
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider("你有没有考虑过其他角度？"),
		Resolved: testResolved,
		Sim:      constSim(0.99),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != nil {
		t.Fatalf("want silence on enforcement rejection, got %+v", action)
	}
	if store.insertInterventionCalls != 0 || store.appendEventCalls != 0 {
		t.Fatalf("rejected output must never be persisted, got insert=%d append=%d",
			store.insertInterventionCalls, store.appendEventCalls)
	}
}

// TestLoop_SurfaceCardOutranksPostIntervention covers Task 5's candidate
// ordering: a project with BOTH an unsupported claim (would fire
// post_intervention) AND an unevaluated source material (fires
// surface_card) must surface the card, never nag about the claim first.
func TestLoop_SurfaceCardOutranksPostIntervention(t *testing.T) {
	g := GraphView{
		Nodes:     []GraphNodeView{{ID: "n1", Type: "claim", Author: "student", Text: "中国的经济转型正在让地球更可持续"}},
		Materials: []MaterialView{{ID: uuid.New().String(), Kind: "article"}},
	}
	store := &fakeAgentStore{graph: g, cardInstances: map[uuid.UUID]CardInstanceRow{}}
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider("should never be called"),
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action == nil || action.Kind != "surface_card" {
		t.Fatalf("want a surface_card action, got %+v", action)
	}
	if action.CardInstanceID == "" {
		t.Fatal("expected a non-empty CardInstanceID")
	}
	if store.createCardInstanceCalls != 1 {
		t.Fatalf("want CreateCardInstance called once, got %d", store.createCardInstanceCalls)
	}
	if store.insertInterventionCalls != 0 {
		t.Fatal("surface_card must outrank post_intervention — an intervention was persisted instead")
	}
}

// TestLoop_UnevaluatedSourceAloneSurfacesCard is the base surface_card
// dispatch case (no competing post_intervention candidate).
func TestLoop_UnevaluatedSourceAloneSurfacesCard(t *testing.T) {
	materialID := uuid.New()
	g := GraphView{Materials: []MaterialView{{ID: materialID.String(), Kind: "article"}}}
	store := &fakeAgentStore{graph: g, cardInstances: map[uuid.UUID]CardInstanceRow{}}
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider("should never be called"),
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action == nil || action.Kind != "surface_card" {
		t.Fatalf("want a surface_card action, got %+v", action)
	}
	if store.lastCreateCardInstance.MaterialID != materialID || store.lastCreateCardInstance.CardID != "craap" {
		t.Fatalf("unexpected CreateCardInstance call: %+v", store.lastCreateCardInstance)
	}
}

// TestLoop_ActiveCardObserveFeedsCoach covers design §5: an active card's
// observe rules are another candidate source feeding the coach through the
// existing enforcement stack — no new coach code.
func TestLoop_ActiveCardObserveFeedsCoach(t *testing.T) {
	g := GraphView{
		CardInstances: []CardInstanceView{
			{
				ID:     uuid.New().String(),
				CardID: "craap",
				Status: "active",
				Anchors: []Anchor{
					{ID: "a0", Dimension: "authority", Author: "ai", Answer: "还可以"}, // <15 bytes -> observe fires
				},
			},
		},
	}
	store := &fakeAgentStore{graph: g}
	body := "这条来源的权威性还没写清楚——它的作者/机构是谁？"
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider(body),
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-C"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action == nil || action.Kind != "intervention" {
		t.Fatalf("want an intervention action, got %+v", action)
	}
	if action.Output.Body != body {
		t.Fatalf("want body %q, got %q", body, action.Output.Body)
	}
	if store.insertInterventionCalls != 1 {
		t.Fatalf("want InsertIntervention called once, got %d", store.insertInterventionCalls)
	}
}

// TestLoop_InactiveCardObserveDoesNotFire covers "observe rules watch
// active cards only" — a proposed (not-yet-opened) or completed card's
// weak-note state must never generate a candidate.
func TestLoop_InactiveCardObserveDoesNotFire(t *testing.T) {
	g := GraphView{
		CardInstances: []CardInstanceView{
			{
				ID:     uuid.New().String(),
				CardID: "craap",
				Status: "proposed",
				Anchors: []Anchor{
					{ID: "a0", Dimension: "authority", Author: "ai", Answer: "还可以"},
				},
			},
		},
	}
	store := &fakeAgentStore{graph: g}
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider("should never be called"),
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-C"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != nil {
		t.Fatalf("want silence for a non-active card, got %+v", action)
	}
}

// TestLoop_CheckGateCandidateEmitsReportNoModelCall covers the check_gate
// wiring (Task 10): when deps.Skill is set and the routed contract's gate is
// not yet machine_clear, RunAgentStep emits a check_gate Action — no model
// call, no enforcement.
func TestLoop_CheckGateCandidateEmitsReportNoModelCall(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	g := GraphView{} // decode_task empty → its gate is not machine_clear
	f := &fakeAgentStore{graph: g}
	deps := AgentDeps{Store: f, Skill: &sk}
	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("RunAgentStep: %v", err)
	}
	if action == nil || action.Kind != "check_gate" {
		t.Fatalf("want a check_gate action, got %+v", action)
	}
}

// TestLoop_PostInterventionOutranksCheckGate pins the corrected candidate
// priority (whole-branch review): a coaching nudge must beat the check_gate
// fallback. With a Skill loaded AND an unsupported claim present, BOTH a
// post_intervention candidate (the D5 nudge) and a check_gate candidate (the
// route's empty first contract) exist — the loop must emit the intervention,
// never let the no-op gate report starve the coaching that moves the student
// toward the gate.
func TestLoop_PostInterventionOutranksCheckGate(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	g := GraphView{
		Nodes: []GraphNodeView{
			{ID: "n1", Type: "claim", Author: "student", Text: "中国的经济转型正在让地球更可持续"},
		},
	}
	body := "这条主张现在还没有素材支撑——它的证据是什么？"
	f := &fakeAgentStore{graph: g}
	deps := AgentDeps{
		Store:    f,
		Provider: scriptedProvider(body),
		Resolved: testResolved,
		Sim:      constSim(0.0),
		Skill:    &sk,
	}
	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("RunAgentStep: %v", err)
	}
	if action == nil || action.Kind != "intervention" {
		t.Fatalf("want an intervention action (nudge outranks check_gate), got %+v", action)
	}
	if action.Output.Body != body {
		t.Fatalf("want the coaching body, got %q", action.Output.Body)
	}
}

// TestRunAgentStep_SkipSurfaceCards covers Slice 5c's conversational-loop
// seam: AgentDeps.SkipSurfaceCards defers card-surfacing entirely. Zero value
// (false) must preserve today's behavior — a project with an unevaluated
// source material still surfaces the card first (Task 5's ordering). Setting
// the flag true removes surface_card from the candidate list, so the same
// project (which also carries an unsupported claim) falls through to the
// post_intervention nudge instead.
func TestRunAgentStep_SkipSurfaceCards(t *testing.T) {
	g := GraphView{
		Nodes:     []GraphNodeView{{ID: "n1", Type: "claim", Author: "student", Text: "中国的经济转型正在让地球更可持续"}},
		Materials: []MaterialView{{ID: uuid.New().String(), Kind: "article"}},
	}
	body := "这条主张现在还没有素材支撑——它的证据是什么？"
	store := &fakeAgentStore{graph: g, cardInstances: map[uuid.UUID]CardInstanceRow{}}
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider(body),
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	// Zero value (SkipSurfaceCards=false) -> existing behavior: surface_card wins.
	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-A"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action == nil || action.Kind != "surface_card" {
		t.Fatalf("default: want surface_card, got %+v", action)
	}

	// SkipSurfaceCards=true -> the card candidate is never produced, so the
	// post_intervention nudge (the other candidate on this graph) fires instead.
	createCallsBefore := store.createCardInstanceCalls
	deps.SkipSurfaceCards = true
	action, err = RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "student_turn"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action == nil {
		t.Fatal("SkipSurfaceCards: want a non-nil Action (post_intervention)")
	}
	if action.Kind == "surface_card" {
		t.Fatalf("SkipSurfaceCards: got a surface_card action %+v", action)
	}
	if action.Kind != "intervention" {
		t.Fatalf("SkipSurfaceCards: want an intervention action, got %+v", action)
	}
	if store.createCardInstanceCalls != createCallsBefore {
		t.Fatalf("SkipSurfaceCards: want no additional card instantiated, before=%d after=%d", createCallsBefore, store.createCardInstanceCalls)
	}
}

// TestSurfaceCard_SetsActionCardID covers Slice 5c-2 Task 2: SurfaceCard's
// emitted Action carries CardID (the card spec id), not just
// CardInstanceID — the SSE emitter (Task 3) needs the card id to tell the
// client which card to render.
func TestSurfaceCard_SetsActionCardID(t *testing.T) {
	g := GraphView{Materials: []MaterialView{{ID: uuid.New().String(), Kind: "article"}}}
	store := &fakeAgentStore{graph: g, cardInstances: map[uuid.UUID]CardInstanceRow{}}
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider("unused"),
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}
	act, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "student_turn"})
	if err != nil {
		t.Fatal(err)
	}
	if act == nil || act.Kind != "surface_card" || act.CardID != "craap" {
		t.Fatalf("want surface_card craap, got %+v", act)
	}
}

// TestRunAgentStep_SurfacesSiftCard_CrossesTheSummonHop is the whole-branch
// review's keystone regression for finding [1]: every pre-fix SIFT test
// (card_lifecycle_test.go) hand-built a cards.Spec{ID:"sift"} and drove
// CompleteCard directly, never once going through RunAgentStep's own
// classify -> decide-one -> act path the way a real turn does. This test
// drives that real path: a material with a finished CRAAP evaluation
// (evaluated-as edge) and no cross-check of its own must make RunAgentStep
// actually call CreateCardInstance for "sift" on that exact material — the
// summon hop SurfaceCardCandidates (classifier.go) alone cannot prove, since
// it never touches the store.
func TestRunAgentStep_SurfacesSiftCard_CrossesTheSummonHop(t *testing.T) {
	materialID := uuid.New()
	g := GraphView{
		Materials: []MaterialView{{ID: materialID.String(), Kind: "article"}},
		Edges: []GraphEdgeView{
			{FromKind: "material", FromID: materialID.String(), ToKind: "graph_node", ToID: "e1", Type: "evaluated-as"},
		},
	}
	store := &fakeAgentStore{graph: g, cardInstances: map[uuid.UUID]CardInstanceRow{}}
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider("should never be called"),
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "student_turn"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action == nil || action.Kind != "surface_card" || action.CardID != "sift" {
		t.Fatalf("want a surface_card(sift) action, got %+v", action)
	}
	if action.MaterialID != materialID.String() {
		t.Fatalf("Action.MaterialID = %q, want %q — the client must never have to guess which material this card is about", action.MaterialID, materialID.String())
	}
	if store.createCardInstanceCalls != 1 {
		t.Fatalf("want CreateCardInstance called once, got %d", store.createCardInstanceCalls)
	}
	if store.lastCreateCardInstance.MaterialID != materialID || store.lastCreateCardInstance.CardID != "sift" {
		t.Fatalf("unexpected CreateCardInstance call: %+v", store.lastCreateCardInstance)
	}
}

// TestLoop_ActiveCardSurvivesACompetingSurfaceCardCandidate is the
// end-to-end proof of the whole-branch-review CRITICAL 1 fix: this is the
// exact scenario proven live on the seeded acceptance project — SIFT is
// active (open) on one material while a SECOND, wholly untouched article
// exists that would otherwise fire its own CRAAP surface_card candidate. One
// more RunAgentStep (e.g. a chat turn while the card sits open) must never
// create a second card_instance — the open card survives silently.
func TestLoop_ActiveCardSurvivesACompetingSurfaceCardCandidate(t *testing.T) {
	g := GraphView{
		CardInstances: []CardInstanceView{
			{ID: uuid.New().String(), CardID: "sift", Status: "active"},
		},
		Materials: []MaterialView{
			{ID: uuid.New().String(), Kind: "article"}, // untouched — would fire its own CRAAP candidate
		},
	}
	store := &fakeAgentStore{graph: g, cardInstances: map[uuid.UUID]CardInstanceRow{}}
	deps := AgentDeps{
		Store:    store,
		Provider: scriptedProvider("should never be called"),
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != nil {
		t.Fatalf("want silence — the open card must not be clobbered by a competing surface_card, got %+v", action)
	}
	if store.createCardInstanceCalls != 0 {
		t.Fatalf("want CreateCardInstance never called while a card is active, got %d call(s) — a second card_instance was minted on top of the open one", store.createCardInstanceCalls)
	}
}

func TestFakeStore_GateStateAndPlanRoundTrip(t *testing.T) {
	f := &fakeAgentStore{}
	pid := uuid.New()
	if err := f.UpsertGateState(context.Background(), pid, "decode_task",
		RecordedGate{Confirmed: true, Items: map[string]string{"milestone_plan": "solid"}}); err != nil {
		t.Fatalf("UpsertGateState: %v", err)
	}
	// upsert again (same contract) must not duplicate
	_ = f.UpsertGateState(context.Background(), pid, "decode_task", RecordedGate{Confirmed: true})
	states, err := f.ListGateStates(context.Background(), pid)
	if err != nil {
		t.Fatalf("ListGateStates: %v", err)
	}
	if len(states) != 1 || !states["decode_task"].Confirmed {
		t.Fatalf("want one confirmed gate_state, got %+v", states)
	}
	if err := f.UpsertPlan(context.Background(), pid, []byte(`{"route":["frame_question"]}`)); err != nil {
		t.Fatalf("UpsertPlan: %v", err)
	}
	if f.upsertPlanCalls != 1 {
		t.Fatalf("want 1 plan upsert, got %d", f.upsertPlanCalls)
	}
}

// countingProvider decorates any gateway.Provider and counts Stream calls, so
// N3b's tests can assert the classifier (or the coach) was never invoked
// without needing a new fake AgentStore/Provider shape — scriptedProvider
// (coach_test.go) has no way to count on its own, so this wraps it rather
// than duplicating it.
type countingProvider struct {
	inner gateway.Provider
	calls int
}

func (c *countingProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	c.calls++
	return c.inner.Stream(ctx, r, req)
}

// erroringProvider is a minimal gateway.Provider double that always fails the
// model call — scriptedProvider's StubProvider has no way to do this, so this
// is a small, non-duplicative extension for the one test that needs it.
type erroringProvider struct{ err error }

func (e erroringProvider) Stream(context.Context, gateway.Resolved, gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	return nil, e.err
}

// longEnoughText clears MinClassifyRunes (12) by a comfortable margin, so the
// N3b classifier tests exercise the semantic path rather than the pre-gate.
const longEnoughText = "这句话足够长，可以触发分类器进行判断。"

// TestRunAgentStepSkipsClassifierWhenStructuralCardWins covers N3b's cost
// rule: when a structural surface_card candidate already exists (here, an
// un-evaluated article material fires CRAAP), the semantic classifier must
// never run — decide-one would discard its answer anyway, so paying for a
// model call would be pure waste.
func TestRunAgentStepSkipsClassifierWhenStructuralCardWins(t *testing.T) {
	g := GraphView{Materials: []MaterialView{{ID: uuid.New().String(), Kind: "article"}}}
	store := &fakeAgentStore{graph: g, cardInstances: map[uuid.UUID]CardInstanceRow{}}
	prov := &countingProvider{inner: scriptedProvider("should never be called")}
	deps := AgentDeps{
		Store:    store,
		Provider: prov,
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "student_turn", StudentText: longEnoughText})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action == nil || action.Kind != "surface_card" || action.CardID != "craap" {
		t.Fatalf("want the structural surface_card(craap) action, got %+v", action)
	}
	if prov.calls != 0 {
		t.Fatalf("want zero provider calls (classifier must not run when a structural card already won), got %d", prov.calls)
	}
}

// TestRunAgentStepSkipsClassifierOnShortText covers the pre-gate: a student
// turn shorter than MinClassifyRunes is never classified, even when nothing
// structural is competing for the turn.
func TestRunAgentStepSkipsClassifierOnShortText(t *testing.T) {
	store := &fakeAgentStore{graph: GraphView{}}
	prov := &countingProvider{inner: scriptedProvider("should never be called")}
	deps := AgentDeps{
		Store:    store,
		Provider: prov,
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "student_turn", StudentText: "嗯"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != nil {
		t.Fatalf("want silence for short text, got %+v", action)
	}
	if prov.calls != 0 {
		t.Fatalf("want zero provider calls for text under MinClassifyRunes, got %d", prov.calls)
	}
}

// TestRunAgentStepSemanticMomentSurfacesItsCard covers the happy path: with no
// structural candidate competing, a named moment ("one_sided") becomes a
// surface_card candidate for that moment's card (steelman, per momentCard).
func TestRunAgentStepSemanticMomentSurfacesItsCard(t *testing.T) {
	store := &fakeAgentStore{graph: GraphView{}, cardInstances: map[uuid.UUID]CardInstanceRow{}}
	prov := &countingProvider{inner: scriptedProvider("one_sided")}
	deps := AgentDeps{
		Store:    store,
		Provider: prov,
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "student_turn", StudentText: longEnoughText})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action == nil || action.Kind != "surface_card" || action.CardID != "steelman" {
		t.Fatalf("want a surface_card(steelman) action, got %+v", action)
	}
	if prov.calls != 1 {
		t.Fatalf("want exactly one classify call, got %d", prov.calls)
	}
	if store.recordLLMCallCalls != 1 || store.lastLLMCall.Purpose != "classify" || store.lastLLMCall.Surface != "studio" {
		t.Fatalf("want one metered classify call, got calls=%d last=%+v", store.recordLLMCallCalls, store.lastLLMCall)
	}
}

// TestRunAgentStepSemanticSuppressedByAnyStatus covers EligibleMoments'
// suppression rule: once every moment's target card already has a
// card_instance in ANY status (here, all three "skipped"), nothing is
// eligible, so the classifier must not even be called.
func TestRunAgentStepSemanticSuppressedByAnyStatus(t *testing.T) {
	g := GraphView{
		CardInstances: []CardInstanceView{
			{ID: uuid.New().String(), CardID: "steelman", Status: "skipped"},
			{ID: uuid.New().String(), CardID: "fact-opinion-value", Status: "skipped"},
			{ID: uuid.New().String(), CardID: "certainty-spectrum", Status: "skipped"},
		},
	}
	store := &fakeAgentStore{graph: g}
	prov := &countingProvider{inner: scriptedProvider("should never be called")}
	deps := AgentDeps{
		Store:    store,
		Provider: prov,
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "student_turn", StudentText: longEnoughText})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != nil {
		t.Fatalf("want silence when nothing is eligible, got %+v", action)
	}
	if prov.calls != 0 {
		t.Fatalf("want zero classify calls when every moment's card already has an instance, got %d", prov.calls)
	}
}

// TestRunAgentStepClassifierErrorIsSilent covers the failure policy: a
// classifier model-call error is silence, never a turn failure.
func TestRunAgentStepClassifierErrorIsSilent(t *testing.T) {
	store := &fakeAgentStore{graph: GraphView{}}
	deps := AgentDeps{
		Store:    store,
		Provider: erroringProvider{err: errors.New("model unavailable")},
		Resolved: testResolved,
		Sim:      constSim(0.0),
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "student_turn", StudentText: longEnoughText})
	if err != nil {
		t.Fatalf("want a classifier error to stay internal (silence), got err=%v", err)
	}
	if action != nil {
		t.Fatalf("want a nil action on classifier error, got %+v", action)
	}
}

// TestRunAgentStepSkipSurfaceCardsSuppressesSemantic covers Slice 5c's
// SkipSurfaceCards seam extended to N3b: when set, the SEMANTIC candidate
// must be suppressed exactly like the structural one, even though the
// classifier would otherwise name a moment.
func TestRunAgentStepSkipSurfaceCardsSuppressesSemantic(t *testing.T) {
	store := &fakeAgentStore{graph: GraphView{}}
	prov := &countingProvider{inner: scriptedProvider("one_sided")}
	deps := AgentDeps{
		Store:            store,
		Provider:         prov,
		Resolved:         testResolved,
		Sim:              constSim(0.0),
		SkipSurfaceCards: true,
	}

	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "student_turn", StudentText: longEnoughText})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != nil {
		t.Fatalf("want silence when SkipSurfaceCards suppresses the semantic candidate, got %+v", action)
	}
	if prov.calls != 0 {
		t.Fatalf("want zero classify calls when SkipSurfaceCards is set, got %d", prov.calls)
	}
}

package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

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

func (f *fakeAgentStore) SetCardInstanceFramework(_ context.Context, _, id uuid.UUID, framework []byte) error {
	f.setFrameworkCalls++
	f.lastFramework = framework
	row := f.cardInstances[id]
	row.FrameworkFill = framework
	f.cardInstances[id] = row
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

func (f *fakeAgentStore) InsertDisposition(_ context.Context, interventionID uuid.UUID, action, reason string) (uuid.UUID, error) {
	f.insertDispositionCalls++
	f.lastDisposition.InterventionID = interventionID
	f.lastDisposition.Action = action
	f.lastDisposition.Reason = reason
	return uuid.New(), nil
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

package agent

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// fakeAgentStore is an in-memory AgentStore double for the loop unit tests —
// no DB, records calls so tests can assert exactly what got persisted.
type fakeAgentStore struct {
	graph GraphView

	insertInterventionCalls int
	lastIntervention        InterventionRow

	appendEventCalls int
	lastEvent        EventRow
}

func (f *fakeAgentStore) LoadGraph(context.Context, uuid.UUID) (GraphView, error) {
	return f.graph, nil
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

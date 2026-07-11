package agent_test

// loop_sqlc_test.go — Task 4's real-DB pass: RunAgentStep over the real
// sqlcAgentStore adapter (agentstore.go), a testcontainers Postgres, and a
// scripted provider (no live model call). Reuses newTurnTestPool from
// turn_test.go (same agent_test package).

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// constSim is a fixed-value enforcement.Similarity stub — no real embedding
// call. Mirrors coach_test.go's unexported constSim, redeclared here because
// this file lives in the external agent_test package.
type constSim float64

func (c constSim) Cosine(string, string) float64 { return float64(c) }

func TestRefactor2LoopSqlcAdapter_UnsupportedClaimPersistsInterventionAndEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q)

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:        seededStudentID,
		Qualification: "EE",
		Title:         "中国是否让地球变得更可持续？",
		BoardCfgVer:   1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	_, err = q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
		ProjectID: project.ID,
		Type:      "claim",
		Body:      []byte(`{"text":"中国的经济转型正在让地球更可持续"}`),
		Author:    "student",
	})
	if err != nil {
		t.Fatalf("InsertGraphNode: %v", err)
	}

	body := "这条主张现在还没有素材支撑——它的证据是什么？"
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: body},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	deps := agent.AgentDeps{
		Store:    store,
		Provider: prov,
		Resolved: gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "coach"},
		Sim:      constSim(0.0),
	}

	action, err := agent.RunAgentStep(ctx, deps, project.ID, agent.Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("RunAgentStep: %v", err)
	}
	if action == nil {
		t.Fatal("expected a non-nil Action")
	}
	if action.Output.Body != body {
		t.Fatalf("want body %q, got %q", body, action.Output.Body)
	}

	interventions, err := q.ListInterventionsByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListInterventionsByProject: %v", err)
	}
	if len(interventions) != 1 {
		t.Fatalf("want 1 persisted intervention, got %d", len(interventions))
	}
	if interventions[0].Body != body {
		t.Fatalf("persisted intervention body = %q, want %q", interventions[0].Body, body)
	}
	if interventions[0].Criterion == nil || *interventions[0].Criterion != "D5" {
		t.Fatalf("persisted intervention criterion = %v, want D5", interventions[0].Criterion)
	}
	if len(interventions[0].Anchor) == 0 {
		t.Fatalf("expected a persisted anchor, got %s", interventions[0].Anchor)
	}

	events, err := q.ListEventsByProject(ctx, pgtype.UUID{Bytes: project.ID, Valid: true})
	if err != nil {
		t.Fatalf("ListEventsByProject: %v", err)
	}
	if len(events) != 1 || events[0].Type != "intervention_posted" || events[0].Surface != "studio" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestRefactor2LoopSqlcAdapter_SupportedClaimPersistsNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q)

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:        seededStudentID,
		Qualification: "EE",
		Title:         "中国是否让地球变得更可持续？",
		BoardCfgVer:   1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	claim, err := q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
		ProjectID: project.ID,
		Type:      "claim",
		Body:      []byte(`{"text":"中国的经济转型正在让地球更可持续"}`),
		Author:    "student",
	})
	if err != nil {
		t.Fatalf("InsertGraphNode(claim): %v", err)
	}
	evidence, err := q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
		ProjectID: project.ID,
		Type:      "evidence",
		Body:      []byte(`{"text":"NASA 卫星数据显示中国光伏装机量全球第一"}`),
		Author:    "student",
	})
	if err != nil {
		t.Fatalf("InsertGraphNode(evidence): %v", err)
	}
	if _, err := q.InsertGraphEdge(ctx, sqlc.InsertGraphEdgeParams{
		ProjectID: project.ID,
		Type:      "supports",
		FromKind:  "graph_node",
		FromID:    evidence.ID,
		ToKind:    "graph_node",
		ToID:      claim.ID,
	}); err != nil {
		t.Fatalf("InsertGraphEdge: %v", err)
	}

	// A provider that must never be called — a supported claim yields no
	// candidate, so RunAgentStep never reaches ProposeIntervention.
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "should never stream"},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	deps := agent.AgentDeps{
		Store:    store,
		Provider: prov,
		Resolved: gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "coach"},
		Sim:      constSim(0.0),
	}

	action, err := agent.RunAgentStep(ctx, deps, project.ID, agent.Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("RunAgentStep: %v", err)
	}
	if action != nil {
		t.Fatalf("want silence (nil Action), got %+v", action)
	}

	interventions, err := q.ListInterventionsByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListInterventionsByProject: %v", err)
	}
	if len(interventions) != 0 {
		t.Fatalf("want 0 persisted interventions on silence, got %d", len(interventions))
	}
	events, err := q.ListEventsByProject(ctx, pgtype.UUID{Bytes: project.ID, Valid: true})
	if err != nil {
		t.Fatalf("ListEventsByProject: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("want 0 appended events on silence, got %d", len(events))
	}
}

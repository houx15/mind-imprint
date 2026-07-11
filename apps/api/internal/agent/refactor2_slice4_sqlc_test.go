package agent_test

// refactor2_slice4_sqlc_test.go — Task 11's real-DB pass: the full Slice-4
// planner round-trip (Intake → reconcile → route → Advance → Replan) over the
// real sqlcAgentStore adapter (agentstore.go) and a testcontainers Postgres.
// Reuses newTurnTestPool / seededStudentID from turn_test.go (same
// agent_test package) — no second harness.

import (
	"testing"

	"context"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

func TestSlice4_IntakeReconcileRouteAdvance_Postgres(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q)
	deps := agent.AgentDeps{Store: store}

	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("skills.ByID(writing-project) not found")
	}

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Intake mints imported nodes (1 rubric_translation + 2
	// weakness_prediction — exactly decode_task's machine gate) and writes
	// the first plan artifact.
	route, err := agent.Intake(ctx, deps, project.ID, sk, []agent.IntakeCandidate{
		{Type: "rubric_translation", Text: "把评分表翻译成人话"},
		{Type: "weakness_prediction", Text: "Table B"},
		{Type: "weakness_prediction", Text: "Table D"},
	})
	if err != nil {
		t.Fatalf("Intake: %v", err)
	}
	if len(route) == 0 || route[0] != "decode_task" {
		t.Fatalf("route = %v, want decode_task first", route)
	}

	// The plan node persisted (one per project).
	plan, err := q.GetPlanNode(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetPlanNode: %v", err)
	}
	if plan.Type != "plan" {
		t.Fatalf("plan node type = %q, want plan", plan.Type)
	}

	// decode_task's machine items now pass (rubric_translation +
	// 2 weakness_prediction); record its lone student_written item
	// (milestone_plan) and advance — the machine-plus-student-written gate
	// must confirm solid.
	if err := store.UpsertGateState(ctx, project.ID, "decode_task",
		agent.RecordedGate{Items: map[string]string{"milestone_plan": "solid"}}); err != nil {
		t.Fatalf("UpsertGateState: %v", err)
	}
	ok2, err := agent.Advance(ctx, deps, project.ID, sk, "decode_task")
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if !ok2 {
		t.Fatal("Advance(decode_task) = false, want true")
	}

	states, err := store.ListGateStates(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGateStates: %v", err)
	}
	if !states["decode_task"].Confirmed {
		t.Fatal("decode_task should be confirmed solid after Advance")
	}

	// Advance itself performs a second UpsertGateState write (rec.Confirmed
	// = true) on the same (project, contract) — the upsert must be
	// idempotent: exactly one gate_state row for decode_task, not two.
	gateNodes, err := q.ListGraphNodesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject: %v", err)
	}
	gateStateCount := 0
	for _, n := range gateNodes {
		if n.Type == "gate_state" {
			gateStateCount++
		}
	}
	if gateStateCount != 1 {
		t.Fatalf("gate_state rows = %d, want exactly 1 (idempotent upsert)", gateStateCount)
	}

	// Re-plan: decode_task is now Solid, so frame_question is the frontier.
	route2, err := agent.Replan(ctx, deps, project.ID, sk, "advanced decode_task")
	if err != nil {
		t.Fatalf("Replan: %v", err)
	}
	if len(route2) == 0 || route2[0] != "frame_question" {
		t.Fatalf("route2 = %v, want frame_question frontier", route2)
	}
}

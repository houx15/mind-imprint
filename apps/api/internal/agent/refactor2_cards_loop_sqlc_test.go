package agent_test

// refactor2_cards_loop_sqlc_test.go — Task 5's real-DB pass: RunAgentStep's
// surface_card dispatch, CompleteCard's graph_effects/framework write, and
// RecordDisposition, all over the real sqlcAgentStore adapter
// (agentstore.go) and a testcontainers Postgres. Reuses newTurnTestPool /
// seededStudentID from turn_test.go (same agent_test package).

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestRefactor2CardsLoop_UnevaluatedSourceSurfacesCraap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q)

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: seededStudentID, Title: "refactor2-cards-loop"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	material, err := q.CreateProjectMaterial(ctx, sqlc.CreateProjectMaterialParams{
		TaskID:    task.ID,
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		Kind:      "article",
		Source:    "fetched",
		Title:     "NASA: China's renewable build-out",
		Blocks:    []byte(`[]`),
	})
	if err != nil {
		t.Fatalf("CreateProjectMaterial: %v", err)
	}

	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "should never be called"},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	deps := agent.AgentDeps{Store: store, Provider: prov, Resolved: gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "coach"}}

	action, err := agent.RunAgentStep(ctx, deps, project.ID, agent.Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("RunAgentStep: %v", err)
	}
	if action == nil || action.Kind != "surface_card" {
		t.Fatalf("want a surface_card action, got %+v", action)
	}
	cardInstanceID, err := uuid.Parse(action.CardInstanceID)
	if err != nil {
		t.Fatalf("parse CardInstanceID: %v", err)
	}

	list, err := q.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: project.ID, Valid: true})
	if err != nil {
		t.Fatalf("ListCardInstancesByProject: %v", err)
	}
	if len(list) != 1 || list[0].ID != cardInstanceID {
		t.Fatalf("ListCardInstancesByProject = %+v, want 1 row matching %s", list, cardInstanceID)
	}
	if list[0].CardID != "craap" || list[0].Status != "proposed" {
		t.Fatalf("unexpected card_instance: card_id=%q status=%q", list[0].CardID, list[0].Status)
	}
	if list[0].TaskID != task.ID {
		t.Fatalf("TaskID = %s, want the material's own task %s", list[0].TaskID, task.ID)
	}

	edges, err := q.ListGraphEdgesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphEdgesByProject: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("want 1 graph_edge (card_instance->material), got %d", len(edges))
	}
	if edges[0].FromKind != "card_instance" || edges[0].FromID != cardInstanceID ||
		edges[0].ToKind != "material" || edges[0].ToID != material.ID {
		t.Fatalf("unexpected edge: %+v", edges[0])
	}

	// A second RunAgentStep pass must not re-propose the same material —
	// it now has an evaluation card_instance.
	action2, err := agent.RunAgentStep(ctx, deps, project.ID, agent.Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("RunAgentStep (2nd pass): %v", err)
	}
	if action2 != nil {
		t.Fatalf("want silence on the 2nd pass, got %+v", action2)
	}
}

func TestRefactor2CardsLoop_CompleteCardMintsEvidenceAndFramework(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q)

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: seededStudentID, Title: "refactor2-cards-complete"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	material, err := q.CreateProjectMaterial(ctx, sqlc.CreateProjectMaterialParams{
		TaskID:    task.ID,
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		Kind:      "article",
		Source:    "fetched",
		Title:     "NASA: China's renewable build-out",
		Blocks:    []byte(`[]`),
	})
	if err != nil {
		t.Fatalf("CreateProjectMaterial: %v", err)
	}

	spec, ok := cards.ByID("craap")
	if !ok {
		t.Fatal("cards.ByID(craap) not found — Task 1's registry config is missing")
	}

	deps := agent.AgentDeps{Store: store}
	surfaced, err := agent.SurfaceCard(ctx, deps, project.ID, spec, material.ID)
	if err != nil {
		t.Fatalf("SurfaceCard: %v", err)
	}
	cardInstanceID, err := uuid.Parse(surfaced.CardInstanceID)
	if err != nil {
		t.Fatalf("parse CardInstanceID: %v", err)
	}

	anchors := []agent.Anchor{
		{ID: "a0", MaterialID: material.ID.String(), Dimension: "currency", Author: "ai", Answer: "2024年发布，数据较新"},
		{ID: "a1", MaterialID: material.ID.String(), Dimension: "relevance", Author: "ai", Answer: "直接支持中国可持续论点"},
		{ID: "a2", MaterialID: material.ID.String(), Dimension: "authority", Author: "ai", Answer: "NASA地球观测团队发布，具备权威性"},
		{ID: "a3", MaterialID: material.ID.String(), Dimension: "accuracy", Author: "ai", Answer: "数据可在Nature Sustainability交叉核对"},
		{ID: "a4", MaterialID: material.ID.String(), Dimension: "purpose", Author: "ai", Answer: "科普告知性质，非商业推广"},
		{ID: "a5", MaterialID: material.ID.String(), Dimension: "risk_note", Author: "student", Answer: "仍需留意样本口径是否一致"},
	}
	anchorsJSON, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal anchors: %v", err)
	}
	if _, err := q.SetCardInstanceAnchors(ctx, sqlc.SetCardInstanceAnchorsParams{
		ID:        cardInstanceID,
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		Anchors:   anchorsJSON,
	}); err != nil {
		t.Fatalf("SetCardInstanceAnchors: %v", err)
	}

	complete, err := agent.CompleteCard(ctx, deps, spec, cardInstanceID)
	if err != nil {
		t.Fatalf("CompleteCard: %v", err)
	}
	if !complete {
		t.Fatal("want complete = true")
	}

	nodes, err := q.ListGraphNodesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Type != "evidence" || nodes[0].Author != "student" {
		t.Fatalf("unexpected minted nodes: %+v", nodes)
	}
	var body map[string]any
	if err := json.Unmarshal(nodes[0].Body, &body); err != nil {
		t.Fatalf("unmarshal node body: %v", err)
	}
	if _, ok := body["source_quality"]; !ok {
		t.Fatalf("evidence node body missing source_quality: %v", body)
	}

	edges, err := q.ListGraphEdgesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphEdgesByProject: %v", err)
	}
	var evaluatedAs int
	for _, e := range edges {
		if e.Type == "evaluated-as" && e.FromKind == "material" && e.FromID == material.ID && e.ToID == nodes[0].ID {
			evaluatedAs++
		}
	}
	if evaluatedAs != 1 {
		t.Fatalf("want 1 evaluated-as edge material->evidence, got %d among %+v", evaluatedAs, edges)
	}

	got, err := q.GetCardInstance(ctx, cardInstanceID)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if got.Status != "proposed" {
		t.Fatalf("Status = %q, want unchanged (proposed) — CompleteCard must never set solid/completed", got.Status)
	}
	var framework map[string]any
	if err := json.Unmarshal(got.FrameworkFill, &framework); err != nil {
		t.Fatalf("unmarshal framework_fill: %v", err)
	}
	if framework["strategy"] != "reveal_framework_after_completion" {
		t.Fatalf("framework_fill = %v, missing strategy", framework)
	}
}

func TestRefactor2CardsLoop_RecordDispositionRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q)

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	criterion := "D5"
	ivn, err := q.InsertIntervention(ctx, sqlc.InsertInterventionParams{
		ProjectID: project.ID, Type: "question",
		Anchor:    []byte(`{"kind":"graph_node","id":"` + project.ID.String() + `"}`),
		Criterion: &criterion, Body: "这条主张现在还没有素材支撑——它的证据是什么？",
	})
	if err != nil {
		t.Fatalf("InsertIntervention: %v", err)
	}

	deps := agent.AgentDeps{Store: store}
	if err := agent.RecordDisposition(ctx, deps, ivn.ID, "reject", "太短"); err == nil {
		t.Fatal("want an error for a <15-char reason")
	}

	reason := "这条追问和我原本的方向不一致，我想先按自己的思路推进"
	if err := agent.RecordDisposition(ctx, deps, ivn.ID, "reject", reason); err != nil {
		t.Fatalf("RecordDisposition: %v", err)
	}
}

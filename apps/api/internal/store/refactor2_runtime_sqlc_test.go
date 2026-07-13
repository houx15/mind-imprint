package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// TestRefactor2RuntimeStoreInterventionQueries exercises the Slice-2 runtime
// sqlc queries: seed a project + a graph_node, InsertIntervention anchored
// to it with a verdict, and confirm ListInterventionsByProject returns it
// with the right fields.
func TestRefactor2RuntimeStoreInterventionQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)
	seededStudentID := refactor2SeededStudentID

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:        seededStudentID,
		Qualification: "EE",
		Title:         "中国是否让地球变得更可持续？",
		BoardCfgVer:   1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	node, err := q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
		ProjectID: project.ID,
		Type:      "claim",
		Body:      []byte(`{"text":"China's build-out is additive"}`),
		Author:    "student",
	})
	if err != nil {
		t.Fatalf("InsertGraphNode: %v", err)
	}

	criterion := "D5"
	level := "I2"
	verdict := "pass"
	anchor := []byte(`{"kind":"graph_node","id":"` + node.ID.String() + `"}`)

	ivn, err := q.InsertIntervention(ctx, sqlc.InsertInterventionParams{
		ProjectID:          project.ID,
		Type:               "question",
		Anchor:             anchor,
		Criterion:          &criterion,
		Body:               "这条主张现在还没有素材支撑——它的证据是什么？",
		Level:              &level,
		OutputCheckVerdict: &verdict,
	})
	if err != nil {
		t.Fatalf("InsertIntervention: %v", err)
	}
	if ivn.ProjectID != project.ID {
		t.Fatalf("InsertIntervention project_id = %s, want %s", ivn.ProjectID, project.ID)
	}
	if ivn.CardInstanceID.Valid {
		t.Fatalf("expected no card_instance_id, got %+v", ivn.CardInstanceID)
	}

	list, err := q.ListInterventionsByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListInterventionsByProject: %v", err)
	}
	if len(list) != 1 || list[0].ID != ivn.ID {
		t.Fatalf("ListInterventionsByProject = %d rows, want 1 matching", len(list))
	}
	got := list[0]
	if got.Type != "question" {
		t.Fatalf("Type = %q, want %q", got.Type, "question")
	}
	if got.Criterion == nil || *got.Criterion != "D5" {
		t.Fatalf("Criterion = %v, want D5", got.Criterion)
	}
	if got.Level == nil || *got.Level != "I2" {
		t.Fatalf("Level = %v, want I2", got.Level)
	}
	if got.OutputCheckVerdict == nil || *got.OutputCheckVerdict != "pass" {
		t.Fatalf("OutputCheckVerdict = %v, want pass", got.OutputCheckVerdict)
	}
	var gotAnchor, wantAnchor map[string]any
	if err := json.Unmarshal(got.Anchor, &gotAnchor); err != nil {
		t.Fatalf("unmarshal got.Anchor: %v", err)
	}
	if err := json.Unmarshal(anchor, &wantAnchor); err != nil {
		t.Fatalf("unmarshal want anchor: %v", err)
	}
	if gotAnchor["kind"] != wantAnchor["kind"] || gotAnchor["id"] != wantAnchor["id"] {
		t.Fatalf("Anchor = %v, want %v", gotAnchor, wantAnchor)
	}
}

// TestCommitCardMint_IsAtomic is Task 6's keystone test: a mint that fails
// partway must leave NOTHING behind — otherwise the retry mints a duplicate
// evidence node, because the idempotency guard (framework_fill) is only
// written at the very end.
//
// graph_edge's (from_kind, from_id)/(to_kind, to_id) endpoints are
// polymorphic (migration 0016) and carry no foreign key — from_id/to_id are
// bare uuid columns validated by nothing but the from_kind/to_kind CHECK
// constraint, so a dangling material id would NOT fail the insert. What
// reliably fails, deterministically, at the database level is an edge whose
// from_kind falls outside that CHECK's enum — exactly the same shape of
// problem (a bad edge insert, after the node insert already succeeded in the
// same transaction), and it is what this test uses to force the rollback.
func TestCommitCardMint_IsAtomic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)
	seededStudentID := refactor2SeededStudentID

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:        seededStudentID,
		Qualification: "EE",
		Title:         "TestCommitCardMint_IsAtomic",
		BoardCfgVer:   1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	contractRef := "craap"
	cardInstance, err := q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		ProjectID:   pgUUID(project.ID),
		CardID:      "craap",
		ContractRef: &contractRef,
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("CreateProjectCardInstance: %v", err)
	}

	store := agent.NewSqlcAgentStore(q, pool)

	// The edge's from_kind ("nonexistent_kind") is outside graph_edge's CHECK
	// constraint enum -> the edge insert fails after the node insert already
	// succeeded inside the same transaction.
	err = store.CommitCardMint(ctx, project.ID, cardInstance.ID, agent.CardMint{
		Nodes:     []agent.MintNode{{Type: "evidence", Author: "student", Body: map[string]any{"x": "y"}}},
		Edges:     []agent.MintEdge{{Type: "evaluated-as", FromKind: "nonexistent_kind", FromID: uuid.New().String(), ToKind: "graph_node", ToID: "$new:0"}},
		Framework: []byte(`{"strategy":"reveal_framework_after_completion"}`),
	})
	if err == nil {
		t.Fatal("expected the mint to fail on the invalid edge endpoint kind")
	}

	// The node must have been rolled back with it.
	nodes, err := q.ListGraphNodesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("partial mint survived: %d node(s) left behind — a retry would duplicate them", len(nodes))
	}

	// The framework_fill guard must also not have been set — a partial mint
	// with the guard set would make a legitimate retry a silent no-op.
	got, err := q.GetCardInstance(ctx, cardInstance.ID)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if fw := string(got.FrameworkFill); fw != "{}" && fw != "" && fw != "null" {
		t.Fatalf("framework_fill = %s, want unset (rolled back)", fw)
	}
}

// pgUUID adapts a uuid.UUID to the pgtype.UUID sqlc params expect for a
// non-null uuid column.
func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

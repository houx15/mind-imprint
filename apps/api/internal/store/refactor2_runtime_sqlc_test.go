package store

import (
	"context"
	"encoding/json"
	"testing"

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

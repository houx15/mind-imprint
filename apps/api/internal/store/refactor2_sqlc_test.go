package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// refactor2SeededStudentID is the fixed UUID from migration 0002_seed.sql.
var refactor2SeededStudentID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

// TestRefactor2SqlcProjectGraphEvent exercises the Slice 0 foundation queries:
// create a project, insert a graph_node + graph_edge, append two events, and
// confirm the list queries return them in insertion order.
func TestRefactor2SqlcProjectGraphEvent(t *testing.T) {
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

	got, err := q.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Title != project.Title {
		t.Fatalf("GetProject title = %q, want %q", got.Title, project.Title)
	}

	projects, err := q.ListProjectsByUser(ctx, seededStudentID)
	if err != nil {
		t.Fatalf("ListProjectsByUser: %v", err)
	}
	// Assert the created project is present rather than that the seed student
	// owns exactly one — migration 0018 seeds a demo project for this same
	// student (Phoebe) so the live Studio's ?trial path has data to render.
	var found bool
	for _, p := range projects {
		if p.ID == project.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ListProjectsByUser (%d rows) did not include the created project %s", len(projects), project.ID)
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

	nodes, err := q.ListGraphNodesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != node.ID {
		t.Fatalf("ListGraphNodesByProject = %d rows, want 1 matching", len(nodes))
	}

	edge, err := q.InsertGraphEdge(ctx, sqlc.InsertGraphEdgeParams{
		ProjectID: project.ID,
		Type:      "supports",
		FromKind:  "graph_node",
		FromID:    node.ID,
		ToKind:    "graph_node",
		ToID:      node.ID,
	})
	if err != nil {
		t.Fatalf("InsertGraphEdge: %v", err)
	}

	edges, err := q.ListGraphEdgesByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListGraphEdgesByProject: %v", err)
	}
	if len(edges) != 1 || edges[0].ID != edge.ID {
		t.Fatalf("ListGraphEdgesByProject = %d rows, want 1 matching", len(edges))
	}

	pgProjectID := pgtype.UUID{Bytes: project.ID, Valid: true}

	first, err := q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgProjectID,
		UserID:    seededStudentID,
		Surface:   "studio",
		Type:      "card_clicked",
		Payload:   []byte(`{"card_id":"craap","unprompted":true}`),
	})
	if err != nil {
		t.Fatalf("AppendEvent(1): %v", err)
	}
	second, err := q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgProjectID,
		UserID:    seededStudentID,
		Surface:   "studio",
		Type:      "gate_attempt",
		Payload:   []byte(`{"result":"pass"}`),
	})
	if err != nil {
		t.Fatalf("AppendEvent(2): %v", err)
	}

	events, err := q.ListEventsByProject(ctx, pgProjectID)
	if err != nil {
		t.Fatalf("ListEventsByProject: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("ListEventsByProject = %d rows, want 2", len(events))
	}
	if events[0].ID != first.ID || events[1].ID != second.ID {
		t.Fatalf("events out of order: got [%s, %s], want [%s, %s]",
			events[0].ID, events[1].ID, first.ID, second.ID)
	}
	if events[0].Type != "card_clicked" || events[1].Type != "gate_attempt" {
		t.Fatalf("event types out of order: got [%s, %s]", events[0].Type, events[1].Type)
	}
}

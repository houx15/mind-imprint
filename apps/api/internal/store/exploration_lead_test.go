package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// TestExplorationLeadRoundTrip exercises the S3 exploration_lead queries end
// to end against a real Postgres: create → list → get (project-scoped) →
// update (connect) → count-for-dedupe → delete → cross-project IDOR guard.
func TestExplorationLeadRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	projectID := seedTestProject(t, ctx, q)

	// A source reference the lead is attributed to, and a second reference
	// it later gets connected to.
	source, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID:   projectID,
		Title:       "NASA 报告",
		Tags:        []byte("[]"),
		SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateReference(source): %v", err)
	}
	answer, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID:   projectID,
		Title:       "Nature Sustainability",
		Tags:        []byte("[]"),
		SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateReference(answer): %v", err)
	}

	sourceID := pgtype.UUID{Bytes: source.ID, Valid: true}

	lead, err := q.CreateExplorationLead(ctx, sqlc.CreateExplorationLeadParams{
		ProjectID:         projectID,
		Text:              "中国的碳排放总量是否抵消了可再生能源投资？",
		Status:            "open",
		Origin:            "takeaway",
		SourceReferenceID: sourceID,
		Position:          0,
	})
	if err != nil {
		t.Fatalf("CreateExplorationLead: %v", err)
	}
	if lead.Status != "open" || lead.Origin != "takeaway" {
		t.Fatalf("lead status/origin = %q/%q, want open/takeaway", lead.Status, lead.Origin)
	}

	// List returns it.
	leads, err := q.ListExplorationLeads(ctx, projectID)
	if err != nil {
		t.Fatalf("ListExplorationLeads: %v", err)
	}
	if len(leads) != 1 || leads[0].ID != lead.ID {
		t.Fatalf("ListExplorationLeads = %+v, want [%v]", leads, lead.ID)
	}

	// Get scoped by project.
	got, err := q.GetExplorationLeadForProject(ctx, sqlc.GetExplorationLeadForProjectParams{
		ID:        lead.ID,
		ProjectID: projectID,
	})
	if err != nil {
		t.Fatalf("GetExplorationLeadForProject: %v", err)
	}
	if got.ID != lead.ID {
		t.Fatalf("GetExplorationLeadForProject id = %v, want %v", got.ID, lead.ID)
	}

	// Count for dedupe: 1 for the (project, source, exact text) tuple, 0 for
	// a different text.
	n, err := q.CountExplorationLeadForSource(ctx, sqlc.CountExplorationLeadForSourceParams{
		ProjectID:         projectID,
		SourceReferenceID: sourceID,
		Text:              lead.Text,
	})
	if err != nil {
		t.Fatalf("CountExplorationLeadForSource: %v", err)
	}
	if n != 1 {
		t.Fatalf("CountExplorationLeadForSource(matching text) = %d, want 1", n)
	}
	n, err = q.CountExplorationLeadForSource(ctx, sqlc.CountExplorationLeadForSourceParams{
		ProjectID:         projectID,
		SourceReferenceID: sourceID,
		Text:              "一个完全不同的线索",
	})
	if err != nil {
		t.Fatalf("CountExplorationLeadForSource(other text): %v", err)
	}
	if n != 0 {
		t.Fatalf("CountExplorationLeadForSource(other text) = %d, want 0", n)
	}

	// Update flips status to connected + sets connected_reference_id.
	updated, err := q.UpdateExplorationLead(ctx, sqlc.UpdateExplorationLeadParams{
		ID:                   lead.ID,
		ProjectID:            projectID,
		Text:                 lead.Text,
		Status:               "connected",
		ConnectedReferenceID: pgtype.UUID{Bytes: answer.ID, Valid: true},
		Position:             1,
	})
	if err != nil {
		t.Fatalf("UpdateExplorationLead: %v", err)
	}
	if updated.Status != "connected" {
		t.Fatalf("updated status = %q, want connected", updated.Status)
	}
	if !updated.ConnectedReferenceID.Valid || updated.ConnectedReferenceID.Bytes != answer.ID {
		t.Fatalf("updated connected_reference_id = %+v, want %v", updated.ConnectedReferenceID, answer.ID)
	}
	if updated.Position != 1 {
		t.Fatalf("updated position = %d, want 1", updated.Position)
	}

	// Get from a different project id returns pgx.ErrNoRows (IDOR guard).
	otherProjectID := seedTestProject(t, ctx, q)
	_, err = q.GetExplorationLeadForProject(ctx, sqlc.GetExplorationLeadForProjectParams{
		ID:        lead.ID,
		ProjectID: otherProjectID,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetExplorationLeadForProject(wrong project) err = %v, want pgx.ErrNoRows", err)
	}

	// Delete removes it.
	if err := q.DeleteExplorationLead(ctx, sqlc.DeleteExplorationLeadParams{
		ID:        lead.ID,
		ProjectID: projectID,
	}); err != nil {
		t.Fatalf("DeleteExplorationLead: %v", err)
	}
	leads, err = q.ListExplorationLeads(ctx, projectID)
	if err != nil {
		t.Fatalf("ListExplorationLeads after delete: %v", err)
	}
	if len(leads) != 0 {
		t.Fatalf("ListExplorationLeads after delete = %+v, want empty", leads)
	}
}

package studio_test

// load_events_test.go — Slice 10 Task 3: Load must project the append-only
// event stream (C4) into ProjectData.Events so a later assessment digest can
// read it. Reuses the same seeded demo project + migrated-pool helper as
// roundtrip_test.go (demoProjectID, newMigratedPool) — no new seed fixture.

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

func TestLoadProjectEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres (testcontainers)")
	}
	pool := newMigratedPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	project, err := q.GetProject(ctx, demoProjectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	pg := pgtype.UUID{Bytes: demoProjectID, Valid: true}

	first, err := q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pg, UserID: project.UserID,
		Surface: "studio", Type: "gate_attempt", Payload: []byte(`{"seq":1}`),
	})
	if err != nil {
		t.Fatalf("append event 1: %v", err)
	}
	second, err := q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pg, UserID: project.UserID,
		Surface: "chat", Type: "snapshot_committed", Payload: []byte(`{"seq":2}`),
	})
	if err != nil {
		t.Fatalf("append event 2: %v", err)
	}
	if !first.CreatedAt.Before(second.CreatedAt) && first.CreatedAt != second.CreatedAt {
		t.Fatalf("test setup: expected event 1 before event 2")
	}

	d, err := studio.Load(ctx, q, demoProjectID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(d.Events) < 2 {
		t.Fatalf("want at least 2 events, got %d", len(d.Events))
	}
	// The two appended events must appear in stream order at the tail.
	n := len(d.Events)
	last, secondLast := d.Events[n-1], d.Events[n-2]
	if secondLast.Type != "gate_attempt" || secondLast.Surface != "studio" {
		t.Errorf("second-last event = %+v, want gate_attempt/studio", secondLast)
	}
	if last.Type != "snapshot_committed" || last.Surface != "chat" {
		t.Errorf("last event = %+v, want snapshot_committed/chat", last)
	}
	if secondLast.CreatedAt.After(last.CreatedAt) {
		t.Errorf("events out of created_at order: %v after %v", secondLast.CreatedAt, last.CreatedAt)
	}
	if len(last.Payload) == 0 {
		t.Errorf("payload not populated")
	}
}

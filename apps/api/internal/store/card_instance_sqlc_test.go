package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// TestSubmitProjectCardInstance exercises the project-scoped writer for a
// card instance's field_values + event_trace (Slice 5c-2 Task 1). Unlike
// SetCardInstanceAnchors/Status/Framework, no such writer existed before —
// the legacy SubmitCard query is task-scoped, not project-scoped. Seeds
// against migration 0018's demo task (…0100) / project (…0101).
func TestSubmitProjectCardInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	projectID := pgtype.UUID{Bytes: uuid.MustParse("00000000-0000-0000-0000-000000000101"), Valid: true}

	ci, err := q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000100"),
		ProjectID:   projectID,
		CardID:      "craap",
		ContractRef: ptr("evaluate_sources"),
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("CreateProjectCardInstance: %v", err)
	}

	fieldValues := []byte(`{"authority_verdict":"存疑"}`)
	eventTrace := []byte(`[{"kind":"submit","at":"2026-07-12T00:00:00Z"}]`)

	got, err := q.SubmitProjectCardInstance(ctx, sqlc.SubmitProjectCardInstanceParams{
		ID:          ci.ID,
		ProjectID:   projectID,
		FieldValues: fieldValues,
		EventTrace:  eventTrace,
	})
	if err != nil {
		t.Fatalf("SubmitProjectCardInstance: %v", err)
	}

	// jsonb reorders keys, so decode-compare rather than assert byte-exact.
	var gotFields map[string]any
	if err := json.Unmarshal(got.FieldValues, &gotFields); err != nil {
		t.Fatalf("unmarshal FieldValues: %v (%s)", err, got.FieldValues)
	}
	if gotFields["authority_verdict"] != "存疑" {
		t.Fatalf("FieldValues[authority_verdict] = %v, want 存疑 (got %s)", gotFields["authority_verdict"], got.FieldValues)
	}
	if len(got.EventTrace) == 0 {
		t.Fatalf("event_trace not persisted")
	}
	var gotTrace []map[string]any
	if err := json.Unmarshal(got.EventTrace, &gotTrace); err != nil {
		t.Fatalf("unmarshal EventTrace: %v (%s)", err, got.EventTrace)
	}
	if len(gotTrace) != 1 || gotTrace[0]["kind"] != "submit" {
		t.Fatalf("EventTrace = %s, want a single submit event", got.EventTrace)
	}

	// Confirm the write is scoped to the right row/project via GetCardInstance.
	reread, err := q.GetCardInstance(ctx, ci.ID)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if string(reread.FieldValues) != string(got.FieldValues) {
		t.Fatalf("GetCardInstance FieldValues = %s, want %s", reread.FieldValues, got.FieldValues)
	}
}

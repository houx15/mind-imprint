package agent_test

// agentstore_cards_sqlc_test.go — Slice 5c-2 Task 2's real-DB pass for the
// card-mutation seam: sqlcAgentStore.SetCardInstanceStatus/
// SetCardInstanceAnchors/SubmitProjectCardInstance over a testcontainers
// Postgres, exercising the round-trip through the raw sqlc row (which,
// unlike agent.CardInstanceRow, also carries field_values/event_trace).

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// jsonEqual compares two jsonb byte payloads by decoded value, not raw
// bytes — Postgres's jsonb storage normalizes key order/whitespace, so a
// byte-for-byte comparison against the literal written in the test would
// spuriously fail.
func jsonEqual(t *testing.T, got, want []byte) bool {
	t.Helper()
	var gv, wv any
	if err := json.Unmarshal(got, &gv); err != nil {
		t.Fatalf("unmarshal got %s: %v", got, err)
	}
	if err := json.Unmarshal(want, &wv); err != nil {
		t.Fatalf("unmarshal want %s: %v", want, err)
	}
	return reflect.DeepEqual(gv, wv)
}

func TestSqlcAgentStore_CardMutationSeamRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: seededStudentID, Title: "agentstore-cards-seam"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	ci, err := q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		TaskID:    pgtype.UUID{Bytes: task.ID, Valid: true},
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		CardID:    "craap",
		Status:    "proposed",
	})
	if err != nil {
		t.Fatalf("CreateProjectCardInstance: %v", err)
	}

	if err := store.SetCardInstanceStatus(ctx, project.ID, ci.ID, "active"); err != nil {
		t.Fatalf("SetCardInstanceStatus: %v", err)
	}
	anchorsJSON := []byte(`[{"id":"a0","material_id":"m1","dimension":"currency","author":"ai","answer":"2024年发布"}]`)
	if err := store.SetCardInstanceAnchors(ctx, project.ID, ci.ID, anchorsJSON); err != nil {
		t.Fatalf("SetCardInstanceAnchors: %v", err)
	}
	fieldValues := []byte(`{"currency":"2024年发布，数据较新"}`)
	eventTrace := []byte(`[{"type":"field_change","field":"currency"}]`)
	if err := store.SubmitProjectCardInstance(ctx, project.ID, ci.ID, fieldValues, eventTrace); err != nil {
		t.Fatalf("SubmitProjectCardInstance: %v", err)
	}

	got, err := q.GetCardInstance(ctx, ci.ID)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("Status = %q, want active", got.Status)
	}
	if !jsonEqual(t, got.Anchors, anchorsJSON) {
		t.Fatalf("Anchors = %s, want %s", got.Anchors, anchorsJSON)
	}
	if !jsonEqual(t, got.FieldValues, fieldValues) {
		t.Fatalf("FieldValues = %s, want %s", got.FieldValues, fieldValues)
	}
	if !jsonEqual(t, got.EventTrace, eventTrace) {
		t.Fatalf("EventTrace = %s, want %s", got.EventTrace, eventTrace)
	}

	// The AgentStore seam's own GetCardInstance also reflects the status/anchors
	// slice of state it exposes (CardInstanceRow carries no field_values/
	// event_trace — those are card-runtime-only, not read by the agent loop).
	row, err := store.GetCardInstance(ctx, ci.ID)
	if err != nil {
		t.Fatalf("store.GetCardInstance: %v", err)
	}
	if row.Status != "active" {
		t.Fatalf("store row Status = %q, want active", row.Status)
	}
	if !jsonEqual(t, row.Anchors, anchorsJSON) {
		t.Fatalf("store row Anchors = %s, want %s", row.Anchors, anchorsJSON)
	}
}

// TestSubmitAndSkipCardInstanceIsAtomic exercises the transactional skip
// write (C2, N6 correctness sweep): the envelope (field_values/event_trace)
// and the "skipped" status must land together, in one commit, mirroring
// CommitCardMint's tx pattern so a crash between the two writes can never
// leave a saved envelope with a stale "active" status (or the reverse).
func TestSubmitAndSkipCardInstanceIsAtomic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: seededStudentID, Title: "agentstore-skip-atomic"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: seededStudentID, Qualification: "EE", Title: "中国是否让地球变得更可持续？", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	ci, err := q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		TaskID:    pgtype.UUID{Bytes: task.ID, Valid: true},
		ProjectID: pgtype.UUID{Bytes: project.ID, Valid: true},
		CardID:    "craap",
		Status:    "proposed",
	})
	if err != nil {
		t.Fatalf("CreateProjectCardInstance: %v", err)
	}

	if err := store.SubmitAndSkipCardInstance(ctx, project.ID, ci.ID, []byte(`{}`), []byte(`[]`)); err != nil {
		t.Fatalf("SubmitAndSkipCardInstance: %v", err)
	}

	got, err := q.GetCardInstance(ctx, ci.ID)
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if got.Status != "skipped" {
		t.Fatalf("Status = %q, want skipped", got.Status)
	}
	if string(got.FieldValues) != "{}" {
		t.Fatalf("FieldValues = %q, want {}", got.FieldValues)
	}
}

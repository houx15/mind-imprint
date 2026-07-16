package store_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// TestChatThreadScope exercises the Slice 11 thread-scoped material/card_instance
// queries introduced by migration 0022: a standalone chat_thread (no project)
// can own materials and card_instances directly (task_id/project_id NULL,
// thread_id set), which the new scope CHECK constraints allow.
func TestChatThreadScope(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	// A standalone thread (no project), owned by the fixed seeded student
	// (seededStudentID is declared in sqlc_test.go, same package).
	th, err := q.CreateStandaloneThread(ctx, sqlc.CreateStandaloneThreadParams{UserID: seededStudentID, Title: "test"})
	if err != nil {
		t.Fatalf("CreateStandaloneThread: %v", err)
	}

	threads, err := q.ListThreadsByUser(ctx, seededStudentID)
	if err != nil || len(threads) == 0 {
		t.Fatalf("ListThreadsByUser: %v n=%d", err, len(threads))
	}

	got, err := q.GetThread(ctx, th.ID)
	if err != nil || got.ID != th.ID {
		t.Fatalf("GetThread: %v, %v", got, err)
	}

	// A thread-scoped material: task_id + project_id NULL, thread_id set — legal.
	tid := pgtype.UUID{Bytes: th.ID, Valid: true}
	sourceURL := "https://e.com"
	m, err := q.CreateThreadMaterial(ctx, sqlc.CreateThreadMaterialParams{
		ThreadID: tid, Kind: "article", Source: "pasted", Title: "x", SourceUrl: &sourceURL, Blocks: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateThreadMaterial: %v", err)
	}
	if m.ThreadID != tid {
		t.Fatalf("material thread_id not set")
	}

	mats, err := q.ListMaterialsByThread(ctx, tid)
	if err != nil || len(mats) != 1 {
		t.Fatalf("ListMaterialsByThread: %v n=%d", err, len(mats))
	}

	// A thread-scoped card_instance flips to completed.
	ci, err := q.CreateThreadCardInstance(ctx, sqlc.CreateThreadCardInstanceParams{ThreadID: tid, CardID: "craap", Status: "proposed"})
	if err != nil {
		t.Fatalf("CreateThreadCardInstance: %v", err)
	}
	done, err := q.SubmitThreadCardInstance(ctx, sqlc.SubmitThreadCardInstanceParams{
		ID: ci.ID, ThreadID: tid, FieldValues: []byte(`{"a":1}`), EventTrace: []byte("[]"), Status: "completed",
	})
	if err != nil || done.Status != "completed" {
		t.Fatalf("SubmitThreadCardInstance: %v status=%s", err, done.Status)
	}

	cis, err := q.ListCardInstancesByThread(ctx, tid)
	if err != nil || len(cis) != 1 {
		t.Fatalf("ListCardInstancesByThread: %v n=%d", err, len(cis))
	}

	// SetThreadCardInstanceStatus is also thread-scoped.
	skipped, err := q.SetThreadCardInstanceStatus(ctx, sqlc.SetThreadCardInstanceStatusParams{ID: ci.ID, ThreadID: tid, Status: "skipped"})
	if err != nil || skipped.Status != "skipped" {
		t.Fatalf("SetThreadCardInstanceStatus: %v status=%s", err, skipped.Status)
	}

	if _, err := q.ListMessagesByThread(ctx, th.ID); err != nil {
		t.Fatalf("ListMessagesByThread: %v", err)
	}
}

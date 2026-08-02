package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

func TestCardLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: seededStudentID, Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	pgTaskID := pgtype.UUID{Bytes: task.ID, Valid: true}

	c1, err := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "craap", TaskID: pgTaskID})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := q.SetCardActive(ctx, sqlc.SetCardActiveParams{ID: c1.ID, TaskID: pgTaskID}); err != nil {
		t.Fatal(err)
	}
	done, err := q.SubmitCard(ctx, sqlc.SubmitCardParams{
		ID:          c1.ID,
		TaskID:      pgTaskID,
		FieldValues: []byte(`{"sift":{"stop":"x"}}`),
		EventTrace:  []byte(`[{"kind":"submit"}]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != "completed" || !done.CompletedAt.Valid {
		t.Fatalf("want completed+completed_at, got %s valid=%v", done.Status, done.CompletedAt.Valid)
	}

	c2, err := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "concession", TaskID: pgTaskID})
	if err != nil {
		t.Fatal(err)
	}
	skip, err := q.SkipCard(ctx, sqlc.SkipCardParams{ID: c2.ID, TaskID: pgTaskID, EventTrace: []byte(`[{"kind":"skip"}]`)})
	if err != nil {
		t.Fatal(err)
	}
	if skip.Status != "skipped" {
		t.Fatalf("want skipped, got %s", skip.Status)
	}

	// Wrong-task scoping returns no rows.
	if _, err := q.SetCardActive(ctx, sqlc.SetCardActiveParams{ID: c1.ID, TaskID: pgtype.UUID{Bytes: uuid.New(), Valid: true}}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("want ErrNoRows for wrong task, got %v", err)
	}
}

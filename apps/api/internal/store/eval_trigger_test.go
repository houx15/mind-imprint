package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"mindimprint/api/internal/store/sqlc"
)

// isUnique reports whether err is a Postgres 23505 unique-constraint violation.
func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func TestEnqueueEvaluation_StampsManual(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	task := seedTaskForEval(t, ctx, pool)

	ev, err := q.EnqueueEvaluation(ctx, task.ID)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if ev.Trigger != "manual" {
		t.Fatalf("trigger = %q, want manual", ev.Trigger)
	}
	if ev.TriggerMilestone != nil {
		t.Fatalf("trigger_milestone = %v, want nil", ev.TriggerMilestone)
	}
}

func TestEvaluations_OneInflightPerTask(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	task := seedTaskForEval(t, ctx, pool)

	if _, err := q.EnqueueEvaluation(ctx, task.ID); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	// A second in-flight eval for the same task must violate the partial unique index.
	_, err := q.EnqueueEvaluation(ctx, task.ID)
	if !isUnique(err) {
		t.Fatalf("second enqueue err = %v, want 23505 unique violation", err)
	}
}

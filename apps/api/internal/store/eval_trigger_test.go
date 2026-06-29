package store_test

import (
	"context"
	"errors"
	"strings"
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

func TestCountSignals(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	task := seedTaskForEval(t, ctx, pool)

	// Two completed cards, one skipped, one active → completed count = 2.
	for i := 0; i < 2; i++ {
		ci, err := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "sift_craap", TaskID: task.ID})
		if err != nil {
			t.Fatalf("create card: %v", err)
		}
		if _, err := q.SubmitCard(ctx, sqlc.SubmitCardParams{
			ID: ci.ID, TaskID: task.ID, FieldValues: []byte("{}"), EventTrace: []byte("[]"),
		}); err != nil {
			t.Fatalf("submit card: %v", err)
		}
	}
	skip, _ := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "concession", TaskID: task.ID})
	if _, err := q.SkipCard(ctx, sqlc.SkipCardParams{ID: skip.ID, TaskID: task.ID, EventTrace: []byte("[]")}); err != nil {
		t.Fatalf("skip card: %v", err)
	}
	act, _ := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "emotional_alignment", TaskID: task.ID})
	if _, err := q.SetCardActive(ctx, sqlc.SetCardActiveParams{ID: act.ID, TaskID: task.ID}); err != nil {
		t.Fatalf("activate card: %v", err)
	}

	// Three substantive user turns (>=20 chars), one short user turn, one assistant turn → count = 3.
	long := strings.Repeat("中", 25)
	for i := 0; i < 3; i++ {
		if _, err := q.AppendMessage(ctx, sqlc.AppendMessageParams{TaskID: task.ID, Role: "user", Content: long}); err != nil {
			t.Fatalf("append long: %v", err)
		}
	}
	if _, err := q.AppendMessage(ctx, sqlc.AppendMessageParams{TaskID: task.ID, Role: "user", Content: "hi"}); err != nil {
		t.Fatalf("append short: %v", err)
	}
	if _, err := q.AppendMessage(ctx, sqlc.AppendMessageParams{TaskID: task.ID, Role: "assistant", Content: long}); err != nil {
		t.Fatalf("append assistant: %v", err)
	}

	cards, err := q.CountCompletedCards(ctx, task.ID)
	if err != nil || cards != 2 {
		t.Fatalf("completed cards = %d (err %v), want 2", cards, err)
	}
	turns, err := q.CountSubstantiveTurns(ctx, task.ID)
	if err != nil || turns != 3 {
		t.Fatalf("substantive turns = %d (err %v), want 3", turns, err)
	}
}

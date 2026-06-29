package store_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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

// markDoneAt forces an eval to done with a given completed_at offset (raw SQL is
// the only way to backdate; no query exists for it and none is warranted).
func markDoneAt(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id any, interval string) {
	t.Helper()
	_, err := pool.Exec(ctx,
		"UPDATE evaluations SET status='done', completed_at = now() - $2::interval WHERE id=$1", id, interval)
	if err != nil {
		t.Fatalf("markDoneAt: %v", err)
	}
}

func TestTryEnqueueMilestone_Guards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	task := seedTaskForEval(t, ctx, pool)

	// (1) First milestone fires.
	ev, err := q.TryEnqueueMilestoneEvaluation(ctx, sqlc.TryEnqueueMilestoneEvaluationParams{TaskID: task.ID, Milestone: 1, Cap: 3})
	if err != nil {
		t.Fatalf("first milestone: %v", err)
	}
	if ev.Trigger != "milestone" || ev.TriggerMilestone == nil || *ev.TriggerMilestone != 1 || ev.Status != "queued" {
		t.Fatalf("bad row: trigger=%q milestone=%v status=%q", ev.Trigger, ev.TriggerMilestone, ev.Status)
	}

	// (2) In-flight: a higher milestone is rejected while the first is still queued.
	_, err = q.TryEnqueueMilestoneEvaluation(ctx, sqlc.TryEnqueueMilestoneEvaluationParams{TaskID: task.ID, Milestone: 2, Cap: 3})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("in-flight guard: err=%v, want ErrNoRows", err)
	}

	// (3) Debounce: finish the first <10min ago → a higher milestone is still rejected.
	markDoneAt(t, ctx, pool, ev.ID, "1 minute")
	_, err = q.TryEnqueueMilestoneEvaluation(ctx, sqlc.TryEnqueueMilestoneEvaluationParams{TaskID: task.ID, Milestone: 2, Cap: 3})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("debounce guard: err=%v, want ErrNoRows", err)
	}

	// (4) Backdate the done past the window → not advanced (same milestone) still rejected.
	markDoneAt(t, ctx, pool, ev.ID, "11 minutes")
	_, err = q.TryEnqueueMilestoneEvaluation(ctx, sqlc.TryEnqueueMilestoneEvaluationParams{TaskID: task.ID, Milestone: 1, Cap: 3})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("not-advanced guard: err=%v, want ErrNoRows", err)
	}

	// (5) Advanced past the window → fires (milestone 2).
	ev2, err := q.TryEnqueueMilestoneEvaluation(ctx, sqlc.TryEnqueueMilestoneEvaluationParams{TaskID: task.ID, Milestone: 2, Cap: 3})
	if err != nil {
		t.Fatalf("advanced milestone: %v", err)
	}

	// (6) Cap: backdate to done, push a 3rd milestone, then a 4th is capped.
	markDoneAt(t, ctx, pool, ev2.ID, "11 minutes")
	ev3, err := q.TryEnqueueMilestoneEvaluation(ctx, sqlc.TryEnqueueMilestoneEvaluationParams{TaskID: task.ID, Milestone: 3, Cap: 3})
	if err != nil {
		t.Fatalf("third milestone: %v", err)
	}
	markDoneAt(t, ctx, pool, ev3.ID, "11 minutes")
	_, err = q.TryEnqueueMilestoneEvaluation(ctx, sqlc.TryEnqueueMilestoneEvaluationParams{TaskID: task.ID, Milestone: 4, Cap: 3})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cap guard: err=%v, want ErrNoRows (3 milestone rows already)", err)
	}
}

func TestTryEnqueueMilestone_ConcurrentExactlyOne(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	task := seedTaskForEval(t, ctx, pool)

	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := q.TryEnqueueMilestoneEvaluation(ctx, sqlc.TryEnqueueMilestoneEvaluationParams{TaskID: task.ID, Milestone: 1, Cap: 3})
			results[idx] = err
		}(i)
	}
	wg.Wait()

	inserted := 0
	for _, err := range results {
		switch {
		case err == nil:
			inserted++
		case errors.Is(err, pgx.ErrNoRows) || isUnique(err):
			// expected loser outcomes
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if inserted != 1 {
		t.Fatalf("inserted = %d, want exactly 1 (unique-index backstop)", inserted)
	}
}

func TestMarkTaskEvaluated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	task := seedTaskForEval(t, ctx, pool)
	if task.Status != "active" {
		t.Fatalf("seed status = %q, want active", task.Status)
	}
	if err := q.MarkTaskEvaluated(ctx, task.ID); err != nil {
		t.Fatalf("mark: %v", err)
	}
	got, err := q.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "evaluated" {
		t.Fatalf("status = %q, want evaluated", got.Status)
	}
}

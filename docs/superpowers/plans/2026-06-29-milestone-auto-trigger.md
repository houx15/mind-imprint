# Milestone Auto-Trigger Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The server auto-enqueues an async evaluation when a task crosses a milestone (first completed card OR 6 substantive student turns), debounced + capped, and the worker flips `task.status='evaluated'` on success.

**Architecture:** A best-effort helper `maybeTriggerMilestoneEval` runs synchronously after the two existing write paths (`putCard` after `SubmitCard`, `postTurn` after a successful `RunTurn`). It computes a single milestone integer from two cheap COUNT queries and calls one guarded conditional-insert query (`TryEnqueueMilestoneEvaluation`); a partial unique index is the cross-transaction backstop. The river `EvaluateWorker` marks the task evaluated on a successful finish.

**Tech Stack:** Go (net/http, pgx/v5, sqlc, goose), river, PostgreSQL. Reuses the shipped P4 async-eval backbone.

## Global Constraints

- The evaluation flagship model is **never downgraded**; eval is the most cost-sensitive path — a duplicate flagship eval is a real cost defect, so the one-in-flight invariant matters.
- Secrets (LLM key / DB DSN / session secret / SMTP) stay **only** in `apps/api` server env; never in errors, logs, the `signals` payload, or any persisted/returned data.
- The trigger helper is **best-effort**: it must never block (beyond a few cheap queries), abort, or error the user's card/turn request. All helper errors are logged via `slog` and swallowed.
- `trigger='milestone'` rows are capped at **3 per task** (auto only). `trigger='manual'` rows are uncapped and never count against the cap.
- Knob values (verbatim): `N` substantive turns per milestone = **6**; substantive student turn = `role='user'` AND `char_length(content) >= 20`; debounce = skip if a `done` eval finished within **10 minutes** OR any eval is `queued`/`running`; lifetime auto cap = **3**.
- `rubric_version` / eval lifecycle semantics from P4 are unchanged.
- **Never stage or commit the repo-root `package.json`** (pre-existing unrelated `M`); use explicit `git add <paths>`.
- Test command: `DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -count=1 ./...` (testcontainers, serialized). sqlc regen: `cd apps/api && make sqlc`. Build/vet: `cd apps/api && CGO_ENABLED=0 go build ./... && go vet ./...`. All commands run from `/Users/houyuxin/08Coding/mind-imprint/apps/api` unless noted.

---

### Task 1: Migration 0008 — trigger columns + one-in-flight unique index

**Files:**
- Create: `apps/api/internal/store/migrations/0008_eval_trigger.sql`
- Modify: `apps/api/internal/store/queries/evaluations.sql` (stamp `EnqueueEvaluation`)
- Regenerate: `apps/api/internal/store/sqlc/*` via `make sqlc`
- Test: `apps/api/internal/store/eval_trigger_test.go` (new)

**Interfaces:**
- Consumes: existing `evaluations` table (cols incl. `status`, `completed_at`, `created_at`), `EnqueueEvaluation(ctx, taskID uuid.UUID) (sqlc.Evaluation, error)`.
- Produces: `sqlc.Evaluation` gains `Trigger string` and `TriggerMilestone *int32`. `EnqueueEvaluation` now stamps `trigger='manual'`. New unique index `evaluations_one_inflight_per_task`.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0008_eval_trigger.sql`:

```sql
-- +goose Up
ALTER TABLE evaluations ADD COLUMN trigger text NOT NULL DEFAULT 'manual';
ALTER TABLE evaluations ADD COLUMN trigger_milestone integer;
CREATE UNIQUE INDEX evaluations_one_inflight_per_task
    ON evaluations (task_id) WHERE status IN ('queued','running');

-- +goose Down
DROP INDEX IF EXISTS evaluations_one_inflight_per_task;
ALTER TABLE evaluations DROP COLUMN trigger_milestone;
ALTER TABLE evaluations DROP COLUMN trigger;
```

- [ ] **Step 2: Stamp `EnqueueEvaluation` with `trigger='manual'`**

In `apps/api/internal/store/queries/evaluations.sql`, replace the `EnqueueEvaluation` block with:

```sql
-- name: EnqueueEvaluation :one
INSERT INTO evaluations (task_id, scores, narrative, model, tier, status, trigger)
VALUES ($1, '[]'::jsonb, '', '', '', 'queued', 'manual')
RETURNING *;
```

- [ ] **Step 3: Regenerate sqlc**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && make sqlc`
Expected: success; `sqlc.Evaluation` now has `Trigger string` and `TriggerMilestone *int32`.

- [ ] **Step 4: Write the failing test**

Create `apps/api/internal/store/eval_trigger_test.go`:

```go
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
```

- [ ] **Step 5: Run the test to verify it passes (migration + regen already in place)**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -count=1 ./internal/store/ -run 'TestEnqueueEvaluation_StampsManual|TestEvaluations_OneInflightPerTask'`
Expected: PASS (both tests).

- [ ] **Step 6: Verify build + full package**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && CGO_ENABLED=0 go build ./... && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -count=1 ./internal/store/`
Expected: build clean; store tests green.

- [ ] **Step 7: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/migrations/0008_eval_trigger.sql \
        apps/api/internal/store/queries/evaluations.sql \
        apps/api/internal/store/sqlc \
        apps/api/internal/store/eval_trigger_test.go
git commit -m "feat(eval): add trigger cols + one-in-flight unique index (milestone P4)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: Count queries — completed cards + substantive turns

**Files:**
- Modify: `apps/api/internal/store/queries/cards.sql` (add `CountCompletedCards`)
- Modify: `apps/api/internal/store/queries/messages.sql` (add `CountSubstantiveTurns`)
- Regenerate: `apps/api/internal/store/sqlc/*` via `make sqlc`
- Test: `apps/api/internal/store/eval_trigger_test.go` (extend)

**Interfaces:**
- Produces: `CountCompletedCards(ctx, taskID uuid.UUID) (int64, error)`, `CountSubstantiveTurns(ctx, taskID uuid.UUID) (int64, error)`.

- [ ] **Step 1: Add the count query for completed cards**

Append to `apps/api/internal/store/queries/cards.sql`:

```sql
-- name: CountCompletedCards :one
SELECT count(*) FROM card_instances
WHERE task_id = $1 AND status = 'completed';
```

- [ ] **Step 2: Add the count query for substantive turns**

Append to `apps/api/internal/store/queries/messages.sql`:

```sql
-- name: CountSubstantiveTurns :one
SELECT count(*) FROM messages
WHERE task_id = $1 AND role = 'user' AND char_length(content) >= 20;
```

- [ ] **Step 3: Regenerate sqlc**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && make sqlc`
Expected: success; `CountCompletedCards` and `CountSubstantiveTurns` methods exist returning `(int64, error)`.

- [ ] **Step 4: Write the failing test**

Append to `apps/api/internal/store/eval_trigger_test.go` (add `"strings"` to its imports):

```go
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
```

- [ ] **Step 5: Run the test**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -count=1 ./internal/store/ -run TestCountSignals`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/queries/cards.sql \
        apps/api/internal/store/queries/messages.sql \
        apps/api/internal/store/sqlc \
        apps/api/internal/store/eval_trigger_test.go
git commit -m "feat(eval): count completed cards + substantive turns (milestone P4)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: `TryEnqueueMilestoneEvaluation` decision query

**Files:**
- Modify: `apps/api/internal/store/queries/evaluations.sql` (add the query)
- Regenerate: `apps/api/internal/store/sqlc/*` via `make sqlc`
- Test: `apps/api/internal/store/eval_trigger_test.go` (extend)

**Interfaces:**
- Produces: `TryEnqueueMilestoneEvaluation(ctx, arg sqlc.TryEnqueueMilestoneEvaluationParams) (sqlc.Evaluation, error)` where `Params{TaskID uuid.UUID; Milestone int32; Cap int32}`. Returns `pgx.ErrNoRows` when the guards reject the insert.

- [ ] **Step 1: Add the decision query**

Append to `apps/api/internal/store/queries/evaluations.sql`:

```sql
-- name: TryEnqueueMilestoneEvaluation :one
INSERT INTO evaluations (task_id, scores, narrative, model, tier, status, trigger, trigger_milestone)
SELECT sqlc.arg(task_id)::uuid, '[]'::jsonb, '', '', '', 'queued', 'milestone', sqlc.arg(milestone)::int
WHERE sqlc.arg(milestone)::int > COALESCE((SELECT max(trigger_milestone) FROM evaluations
                     WHERE task_id = sqlc.arg(task_id)::uuid AND trigger = 'milestone'), 0)
  AND (SELECT count(*) FROM evaluations
       WHERE task_id = sqlc.arg(task_id)::uuid AND trigger = 'milestone') < sqlc.arg(cap)::int
  AND NOT EXISTS (SELECT 1 FROM evaluations
                  WHERE task_id = sqlc.arg(task_id)::uuid AND status IN ('queued','running'))
  AND NOT EXISTS (SELECT 1 FROM evaluations
                  WHERE task_id = sqlc.arg(task_id)::uuid AND status = 'done'
                    AND completed_at > now() - interval '10 minutes')
RETURNING *;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && make sqlc`
Expected: success; `TryEnqueueMilestoneEvaluation` method + `TryEnqueueMilestoneEvaluationParams{TaskID uuid.UUID; Milestone int32; Cap int32}` exist.

- [ ] **Step 3: Write the failing test (guards)**

Append to `apps/api/internal/store/eval_trigger_test.go` (add `"sync"` and `"github.com/jackc/pgx/v5"` to imports):

```go
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
```

Note: the tests call `q.TryEnqueueMilestoneEvaluation` directly with `task.ID` (a `uuid.UUID`). The `markDoneAt` helper's `id any` param accepts `ev.ID` (a `uuid.UUID`) and passes it straight to `pool.Exec`.

- [ ] **Step 4: Run the tests**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -count=1 ./internal/store/ -run 'TestTryEnqueueMilestone'`
Expected: PASS (both guard + concurrency tests).

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/queries/evaluations.sql \
        apps/api/internal/store/sqlc \
        apps/api/internal/store/eval_trigger_test.go
git commit -m "feat(eval): TryEnqueueMilestoneEvaluation guarded decision query (milestone P4)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: Trigger helper + milestone math + hook wiring

**Files:**
- Create: `apps/api/internal/api/eval_trigger.go`
- Modify: `apps/api/internal/api/cards.go:118-122` (`putCard` — trigger after `SubmitCard`)
- Modify: `apps/api/internal/api/turn.go:130-139` (`postTurn` — trigger after successful `RunTurn`)
- Test: `apps/api/internal/api/eval_trigger_test.go` (new)

**Interfaces:**
- Consumes: `a.d.Queries.CountCompletedCards`, `CountSubstantiveTurns`, `TryEnqueueMilestoneEvaluation`, `FailEvaluation` (Task 1-3); `a.d.Enqueuer.EnqueueEvaluate`; `ptrStr` (in `evaluate.go`, same package).
- Produces: `func milestoneIndex(completedCards, substantiveTurns int64) int32`; `func (a *API) maybeTriggerMilestoneEval(ctx context.Context, taskID uuid.UUID)`; `func isUniqueViolation(err error) bool` (reused by Task 6).

- [ ] **Step 1: Write the failing unit test for milestone math**

Create `apps/api/internal/api/eval_trigger_test.go`:

```go
package api_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestMilestoneIndex(t *testing.T) {
	cases := []struct {
		cards, turns int64
		want         int32
	}{
		{0, 0, 0}, {0, 5, 0}, {0, 6, 1}, {0, 11, 1}, {0, 12, 2},
		{1, 0, 1}, {1, 6, 2}, {2, 7, 3},
	}
	for _, c := range cases {
		if got := MilestoneIndex(c.cards, c.turns); got != c.want {
			t.Errorf("MilestoneIndex(%d,%d) = %d, want %d", c.cards, c.turns, got, c.want)
		}
	}
}
```

Note: this test calls an **exported** `MilestoneIndex`. Export the math function (capitalized) so the `api_test` package can unit-test it without a DB; keep `maybeTriggerMilestoneEval` unexported.

- [ ] **Step 2: Run it to verify it fails**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && go test ./internal/api/ -run TestMilestoneIndex`
Expected: FAIL — `undefined: MilestoneIndex`.

- [ ] **Step 3: Write the helper**

Create `apps/api/internal/api/eval_trigger.go`:

```go
package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

const (
	substantiveTurnsPerMilestone = 6
	maxAutoEvals                 = 3
)

// MilestoneIndex collapses task progress into one monotonic integer: one per
// completed card plus one per N substantive student turns.
func MilestoneIndex(completedCards, substantiveTurns int64) int32 {
	return int32(completedCards + substantiveTurns/substantiveTurnsPerMilestone)
}

// maybeTriggerMilestoneEval best-effort enqueues an async evaluation when the
// task crosses a new milestone, subject to the cap/debounce/in-flight guards in
// TryEnqueueMilestoneEvaluation. It NEVER blocks (beyond a few cheap queries),
// aborts, or fails the caller's request — every error is logged and swallowed.
func (a *API) maybeTriggerMilestoneEval(ctx context.Context, taskID uuid.UUID) {
	cards, err := a.d.Queries.CountCompletedCards(ctx, taskID)
	if err != nil {
		slog.Warn("milestone: count cards failed", "task_id", taskID.String(), "err", err.Error())
		return
	}
	turns, err := a.d.Queries.CountSubstantiveTurns(ctx, taskID)
	if err != nil {
		slog.Warn("milestone: count turns failed", "task_id", taskID.String(), "err", err.Error())
		return
	}
	m := MilestoneIndex(cards, turns)
	if m < 1 {
		return // no milestone reached yet
	}
	ev, err := a.d.Queries.TryEnqueueMilestoneEvaluation(ctx, sqlc.TryEnqueueMilestoneEvaluationParams{
		TaskID:    taskID,
		Milestone: m,
		Cap:       maxAutoEvals,
	})
	if err != nil {
		// ErrNoRows = guards rejected the trigger; 23505 = a concurrent trigger
		// won the one-in-flight race. Both are normal "do nothing" outcomes.
		if errors.Is(err, pgx.ErrNoRows) || isUniqueViolation(err) {
			return
		}
		slog.Warn("milestone: try-enqueue failed", "task_id", taskID.String(), "err", err.Error())
		return
	}
	if err := a.d.Enqueuer.EnqueueEvaluate(ctx, agent.EvaluateArgs{EvaluationID: ev.ID, TaskID: taskID}); err != nil {
		// Best-effort: mark the row failed so nothing polls a job that never queued.
		_ = a.d.Queries.FailEvaluation(ctx, sqlc.FailEvaluationParams{ID: ev.ID, Error: ptrStr("入队失败")})
		slog.Error("milestone: enqueue failed", "task_id", taskID.String(), "err", err.Error())
	}
}

// isUniqueViolation reports whether err is a Postgres 23505 unique-constraint
// violation (used here and by the manual postEvaluate idempotency path).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
```

- [ ] **Step 4: Run the unit test to verify it passes**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && go test ./internal/api/ -run TestMilestoneIndex`
Expected: PASS.

- [ ] **Step 5: Wire the helper into `putCard`**

In `apps/api/internal/api/cards.go`, replace the tail of `putCard` (the `SubmitCard` call + `writeCardOrNotFound`) with:

```go
	c, err := a.d.Queries.SubmitCard(r.Context(), sqlc.SubmitCardParams{
		ID: cardID, TaskID: taskID,
		FieldValues: []byte(body.FieldValues), EventTrace: []byte(body.EventTrace),
	})
	if err == nil {
		// A completed card may cross a milestone — best-effort, never blocks the response.
		a.maybeTriggerMilestoneEval(r.Context(), taskID)
	}
	writeCardOrNotFound(w, r, c, err)
```

- [ ] **Step 6: Wire the helper into `postTurn`**

In `apps/api/internal/api/turn.go`, change the final `if err := agent.RunTurn(...)` block to also trigger on success:

```go
	if err := agent.RunTurn(r.Context(), deps, t.ID, body.UserInput); err != nil {
		// Stream already open: report via SSE error event. Log the real cause
		// server-side only — never leak provider/internal detail to the client.
		slog.Error("turn failed",
			"request_id", httpx.RequestIDFromContext(r.Context()),
			"task_id", t.ID.String(),
			"err", err.Error(),
		)
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		return
	}
	// Turn succeeded (assistant message persisted) — a new substantive turn may
	// cross a milestone. Best-effort; must not write to the SSE stream.
	a.maybeTriggerMilestoneEval(r.Context(), t.ID)
```

- [ ] **Step 7: Write the failing handler test (card completion triggers; second is debounced)**

Append to `apps/api/internal/api/eval_trigger_test.go`:

```go
func TestPutCard_TriggersMilestoneEval(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "T"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	card1, _ := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "sift_craap", TaskID: task.ID})
	card2, _ := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "concession", TaskID: task.ID})

	rec := &recordingEnqueuer{}
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Enqueuer: rec}).Handler()
	cookie := signInSeed(t, pool)

	put := func(cid string) int {
		body := strings.NewReader(`{"field_values":{},"event_trace":[],"status":"completed"}`)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", "/api/v1/tasks/"+task.ID.String()+"/cards/"+cid, body), cookie))
		return rr.Code
	}

	// First completed card → milestone 1 → exactly one auto-enqueue.
	if code := put(card1.ID.String()); code != 200 {
		t.Fatalf("put card1 status=%d", code)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("after card1: enqueues=%d, want 1", len(rec.calls))
	}
	if rec.calls[0].TaskID != task.ID {
		t.Fatalf("enqueued wrong task: %s", rec.calls[0].TaskID)
	}

	// Second completed card while the first eval is still queued → debounced (no new enqueue).
	if code := put(card2.ID.String()); code != 200 {
		t.Fatalf("put card2 status=%d", code)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("after card2: enqueues=%d, want 1 (in-flight debounce)", len(rec.calls))
	}
}
```

- [ ] **Step 8: Run the handler test**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -count=1 ./internal/api/ -run 'TestMilestoneIndex|TestPutCard_TriggersMilestoneEval'`
Expected: PASS.

- [ ] **Step 9: Verify build + full api package**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && CGO_ENABLED=0 go build ./... && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -count=1 ./internal/api/`
Expected: build clean; api tests green (existing evaluate_test.go still passes).

- [ ] **Step 10: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/eval_trigger.go \
        apps/api/internal/api/eval_trigger_test.go \
        apps/api/internal/api/cards.go \
        apps/api/internal/api/turn.go
git commit -m "feat(eval): milestone trigger helper + card/turn hooks (milestone P4)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 5: Worker flips `task.status='evaluated'` on success

**Files:**
- Modify: `apps/api/internal/store/queries/tasks.sql` (add `MarkTaskEvaluated`; remove `SetTaskEvaluated`)
- Regenerate: `apps/api/internal/store/sqlc/*` via `make sqlc`
- Modify: `apps/api/internal/agent/eval.go` (add `MarkTaskEvaluated` to `EvalLifecycleStore` + `sqlcEvalStore`)
- Modify: `apps/api/internal/agent/evaljob.go` (call `MarkTaskEvaluated` after `Finish`)
- Test: `apps/api/internal/agent/evaljob_test.go` (extend fake + assertions); `apps/api/internal/store/eval_trigger_test.go` (extend)

**Interfaces:**
- Produces: `MarkTaskEvaluated(ctx, id uuid.UUID) error` on `*sqlc.Queries`, on the `EvalLifecycleStore` interface, and on `*sqlcEvalStore`.
- Consumes: `EvaluateWorker.Work` calls it on the success branch.

- [ ] **Step 1: Swap the task query**

In `apps/api/internal/store/queries/tasks.sql`, **remove** the `SetTaskEvaluated` block and **add**:

```sql
-- name: MarkTaskEvaluated :exec
UPDATE tasks SET status = 'evaluated' WHERE id = $1;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && make sqlc`
Expected: success; `SetTaskEvaluated` gone, `MarkTaskEvaluated(ctx, id uuid.UUID) error` present.

- [ ] **Step 3: Add `MarkTaskEvaluated` to the worker store seam**

In `apps/api/internal/agent/eval.go`, add the method to the `EvalLifecycleStore` interface (after `Fail`):

```go
	Fail(ctx context.Context, evalID uuid.UUID, msg string) error
	MarkTaskEvaluated(ctx context.Context, taskID uuid.UUID) error
```

And add the adapter method on `*sqlcEvalStore` (next to `Fail`):

```go
func (s *sqlcEvalStore) MarkTaskEvaluated(ctx context.Context, taskID uuid.UUID) error {
	return s.q.MarkTaskEvaluated(ctx, taskID)
}
```

- [ ] **Step 4: Call it on the worker success branch**

In `apps/api/internal/agent/evaljob.go`, add `"log/slog"` to the imports, and replace the final `return w.Store.Finish(...)` with a captured-error version that flips the task:

```go
	rv := RubricVersion
	if err := w.Store.Finish(ctx, sqlc.FinishEvaluationParams{
		ID:               id,
		Scores:           scoresJSON,
		Narrative:        r.Out.Narrative,
		Signals:          sigJSON,
		RubricVersion:    &rv,
		Model:            r.Resolved.Model,
		Tier:             r.Resolved.Tier,
		PromptTokens:     &pt,
		CompletionTokens: &ct,
		CostEstimate:     gateway.CostNumeric(cost, ok),
	}); err != nil {
		return err
	}
	// Best-effort: flip the task to 'evaluated' now that a result exists. A failure
	// here must NOT re-run the never-downgrade flagship eval, so swallow + log.
	if err := w.Store.MarkTaskEvaluated(ctx, job.Args.TaskID); err != nil {
		slog.Warn("eval: mark task evaluated failed", "task_id", job.Args.TaskID.String(), "err", err.Error())
	}
	return nil
```

- [ ] **Step 5: Extend the worker fake + assert (failing build first)**

In `apps/api/internal/agent/evaljob_test.go`, add a field to `fakeLifecycleStore`:

```go
	failMsg         string
	evaluatedTaskID uuid.UUID
```

and the method:

```go
func (f *fakeLifecycleStore) MarkTaskEvaluated(_ context.Context, id uuid.UUID) error {
	f.evaluatedTaskID = id
	return nil
}
```

In `TestEvaluateWorker_Work_FinishesDone`, after the existing assertions, add:

```go
	if store.evaluatedTaskID != taskID {
		t.Fatalf("MarkTaskEvaluated not called with task id: got %v want %v", store.evaluatedTaskID, taskID)
	}
```

In both `TestEvaluateWorker_Work_FailsOnParseError` and `TestEvaluateWorker_Work_NonFinalAttempt_DoesNotMarkFailed`, add:

```go
	if store.evaluatedTaskID != uuid.Nil {
		t.Error("MarkTaskEvaluated must not be called on a failure path")
	}
```

- [ ] **Step 6: Run the worker tests**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && go test ./internal/agent/ -run TestEvaluateWorker`
Expected: PASS (all three worker tests).

- [ ] **Step 7: Write the store test for `MarkTaskEvaluated`**

Append to `apps/api/internal/store/eval_trigger_test.go`:

```go
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
```

- [ ] **Step 8: Run store test + verify no `SetTaskEvaluated` references remain**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && CGO_ENABLED=0 go build ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -count=1 ./internal/store/ -run TestMarkTaskEvaluated`
Expected: build clean (confirms nothing referenced the removed `SetTaskEvaluated`); test PASS.

- [ ] **Step 9: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/queries/tasks.sql \
        apps/api/internal/store/sqlc \
        apps/api/internal/agent/eval.go \
        apps/api/internal/agent/evaljob.go \
        apps/api/internal/agent/evaljob_test.go \
        apps/api/internal/store/eval_trigger_test.go
git commit -m "feat(eval): worker marks task evaluated on success (milestone P4)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 6: Manual `postEvaluate` idempotency on in-flight

**Files:**
- Modify: `apps/api/internal/api/evaluate.go:29-45` (`postEvaluate` — handle `23505`)
- Test: `apps/api/internal/api/eval_trigger_test.go` (extend)

**Interfaces:**
- Consumes: `isUniqueViolation` (Task 4, same package); `GetLatestEvaluation`, `toEvaluationDTO`.

- [ ] **Step 1: Handle the unique violation in `postEvaluate`**

In `apps/api/internal/api/evaluate.go`, replace the `EnqueueEvaluation` error handling (the `if err != nil` right after the `EnqueueEvaluation` call) with:

```go
	// Insert a queued evaluation row, then hand the job off to the async queue.
	ev, err := a.d.Queries.EnqueueEvaluation(r.Context(), t.ID)
	if err != nil {
		// One in-flight eval per task (partial unique index). If one is already
		// queued/running, return it — the client polls it — instead of erroring.
		if isUniqueViolation(err) {
			if existing, gerr := a.d.Queries.GetLatestEvaluation(r.Context(), t.ID); gerr == nil {
				httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"evaluation": toEvaluationDTO(existing)})
				return
			}
		}
		httpx.WriteError(w, r, err)
		return
	}
```

- [ ] **Step 2: Write the failing test**

Append to `apps/api/internal/api/eval_trigger_test.go`:

```go
func TestPostEvaluate_InflightIsIdempotent(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "T"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	// Pre-seed an in-flight (queued) eval directly.
	existing, err := q.EnqueueEvaluation(ctx, task.ID)
	if err != nil {
		t.Fatalf("seed enqueue: %v", err)
	}

	rec := &recordingEnqueuer{}
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Enqueuer: rec}).Handler()
	cookie := signInSeed(t, pool)

	// Manual POST /evaluate while one is in flight → 202 with the existing row, no new job.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/evaluate", nil), cookie))
	if rr.Code != 202 {
		t.Fatalf("status=%d want 202; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), existing.ID.String()) {
		t.Fatalf("body should return the in-flight eval %s: %s", existing.ID, rr.Body.String())
	}
	if len(rec.calls) != 0 {
		t.Fatalf("no new job should be enqueued, got %d", len(rec.calls))
	}
}
```

- [ ] **Step 3: Run the test**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -count=1 ./internal/api/ -run TestPostEvaluate`
Expected: PASS (this test + the existing `TestPostEvaluate_EnqueuesQueued`).

- [ ] **Step 4: Full suite + vet**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && CGO_ENABLED=0 go build ./... && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -count=1 ./...`
Expected: build/vet clean; all packages green.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/evaluate.go apps/api/internal/api/eval_trigger_test.go
git commit -m "feat(eval): manual postEvaluate returns in-flight row on 23505 (milestone P4)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Self-Review Notes (for the executor)

- **Spec coverage:** §2 knobs → Task 4 consts + Task 1-3 SQL. §3 helper → Task 4. §4 math → Task 4 `MilestoneIndex`. §5 query + index + 23505 handling → Tasks 1, 3, 4. §6 migration + manual stamp + idempotency → Tasks 1, 6. §7 worker status flip → Task 5. §9 testing → each task's tests.
- **Green build between tasks:** Tasks 1-3 are additive (no caller depends on the new queries yet). Task 4 introduces the helper + wiring and the first caller. Task 5 changes the worker + removes `SetTaskEvaluated` (verified unreferenced). Task 6 changes the manual path. Existing `evaluate_test.go` only enqueues once per task, so the new unique index never trips it before Task 6.
- **Type consistency:** `MilestoneIndex(int64, int64) int32`; `TryEnqueueMilestoneEvaluationParams{TaskID uuid.UUID; Milestone int32; Cap int32}`; `MarkTaskEvaluated(ctx, uuid.UUID) error`; `sqlc.Evaluation.Trigger string` / `TriggerMilestone *int32`. If `make sqlc` emits different nullable mappings (e.g. `pgtype.Int4` instead of `*int32`), follow the generated types and adjust the test assertions accordingly — the existing `RubricVersion *string` / `PromptTokens *int32` precedent indicates pointer mappings.

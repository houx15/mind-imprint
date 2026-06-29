# Milestone Auto-Trigger — Design

> **Status:** design (awaiting review) · **Date:** 2026-06-29
> **Builds on:** `2026-06-29-evaluation-model-design.md` §6 (Trigger + async path, locked) and the shipped
> P4 async-eval backbone (`2026-06-29-p4-async-evaluation-backend.md`).
> **Anchors:** 铁律 #2 不操纵（触发自动，打开由学生确认）· #4 过程即数据.

## 1. Goal & scope

The server **auto-enqueues** an async evaluation at a *milestone* — **first completed card OR 6 substantive
student turns** (whichever comes first) — recurring on later milestones but **debounced + capped to 3 auto-evals
per task**. It also **re-homes** the `task.status='evaluated'` marking (dropped during P4) onto the worker's
successful-finish path.

**In scope (backend only):**
- A shared trigger helper invoked from the two existing write paths (card submit, turn append).
- An atomic, race-free conditional-insert that enforces all trigger guards in one SQL statement.
- A migration adding two bookkeeping columns to `evaluations`.
- Worker flips `tasks.status='evaluated'` on successful eval.

**Out of scope (separate follow-ups):**
- The "你的思维印记有新内容" reveal indicator and the student-facing reveal UI (frontend follow-up plan).
- Cost recalibration of the knobs (carry-forward in the eval spec — revisit with real cost data).

## 2. Locked knobs ("Balanced" preset)

| Knob | Value |
|---|---|
| `N` substantive turns per milestone | **6** |
| substantive student turn | `role='user'` AND `char_length(content) >= 20` |
| debounce: skip if a `done` eval finished within | **10 minutes** |
| debounce: skip if an eval is `queued` or `running` | always |
| lifetime cap on **auto** evals per task | **3** |
| manual `POST /evaluate` | **uncapped** (always available; not debounced by milestone logic) |

The cap counts **only** `trigger='milestone'` rows. Manual evals (`trigger='manual'`) never count against the cap
and are not gated by milestone logic. Failed milestone evals **still consume cap** — this prevents hammering the
never-downgrade flagship on a task whose evaluation keeps breaking.

## 3. The trigger helper

A single best-effort helper — `maybeTriggerMilestoneEval(ctx, taskID)` — called **after the user's write has
committed** from two existing hook points:

- `putCard` (`internal/api/cards.go`) — after `SubmitCard` succeeds. Catches "first completed card" and each later
  completed card.
- `RunTurn` / `postTurn` (`internal/agent/turn.go` / `internal/api/turn.go`) — after the assistant message is
  appended. Catches the "+6 substantive turns" cadence.

**Best-effort contract:** the helper must never block, delay, or fail the user's turn/card request. The user's write
already succeeded; the evaluation is a side effect. Any error (count query, insert, enqueue) is logged via `slog`
and swallowed. It returns no value the caller acts on.

The helper:
1. Computes `currentMilestone` (§4).
2. Calls `TryEnqueueMilestoneEvaluation` (§5) — the guarded SQL decision.
3. If a row was returned, calls `Enqueuer.EnqueueEvaluate(EvaluateArgs{EvaluationID, TaskID})`. If `pgx.ErrNoRows`
   **or a `23505` unique violation** (a concurrent trigger won the one-in-flight race, §5), the trigger was
   rejected — do nothing. If the river enqueue then fails, mark the just-inserted row `failed` (mirroring the manual
   path in `evaluate.go`) and log.

## 4. Milestone math

A single integer derived from two cheap count queries, no per-threshold bookkeeping:

```
currentMilestone = completedCardCount + (substantiveTurnCount / N)        // integer division, N=6
```

- `completedCardCount` — `card_instances` rows for the task with `status='completed'`.
- `substantiveTurnCount` — `messages` rows for the task with `role='user'` AND `char_length(content) >= 20`.

A trigger fires only when `currentMilestone` **exceeds the highest milestone already auto-triggered** for the task
(see §5). Consequences:
- 5 substantive turns → milestone 0 (no fire); the 6th → milestone 1.
- Completing the first card → milestone ≥ 1 immediately (the proactive first "wow").
- A trivial (<20 char) message, or the hook being called again with no new substance, leaves `currentMilestone`
  unchanged → no-op.

Counts are recomputed each call from the DB; there is no in-memory counter to keep in sync.

## 5. The decision query + concurrency backstop — `TryEnqueueMilestoneEvaluation`

The four logical guards live in one conditional insert. It inserts a `queued` row **only if** every guard holds and
returns it; otherwise it returns no rows (`pgx.ErrNoRows` → "not triggered, do nothing").

```sql
-- name: TryEnqueueMilestoneEvaluation :one
INSERT INTO evaluations (task_id, scores, narrative, model, tier, status, trigger, trigger_milestone)
SELECT sqlc.arg(task_id)::uuid, '[]'::jsonb, '', '', '', 'queued', 'milestone', sqlc.arg(milestone)::int
WHERE sqlc.arg(milestone)::int > COALESCE((SELECT max(trigger_milestone) FROM evaluations
                     WHERE task_id = sqlc.arg(task_id)::uuid AND trigger = 'milestone'), 0)  -- new milestone
  AND (SELECT count(*) FROM evaluations
       WHERE task_id = sqlc.arg(task_id)::uuid AND trigger = 'milestone') < sqlc.arg(cap)::int  -- lifetime cap
  AND NOT EXISTS (SELECT 1 FROM evaluations
                  WHERE task_id = sqlc.arg(task_id)::uuid AND status IN ('queued','running'))    -- no in-flight
  AND NOT EXISTS (SELECT 1 FROM evaluations
                  WHERE task_id = sqlc.arg(task_id)::uuid AND status = 'done'
                    AND completed_at > now() - interval '10 minutes')                            -- debounce
RETURNING *;
```

Params: `TaskID uuid.UUID`, `Milestone int32`, `Cap int32 (=3)`. Result: the new evaluation row, or `pgx.ErrNoRows`.

**A single statement is NOT race-free across concurrent transactions.** Two simultaneous triggers (a card-submit
racing a turn, or a manual eval racing an auto one) each evaluate `NOT EXISTS` against a snapshot that excludes the
other's still-uncommitted row, so both pass and both insert — a duplicate flagship eval (the most cost-sensitive
path). The `WHERE` guards correctly handle the **sequential** case plus the logical conditions; a **partial unique
index** is the cross-transaction backstop that makes "≤ 1 in-flight eval per task" a hard invariant:

```sql
CREATE UNIQUE INDEX evaluations_one_inflight_per_task
    ON evaluations (task_id) WHERE status IN ('queued','running');
```

The loser of a concurrent race gets a `23505` unique-constraint violation. The trigger helper treats `23505`
exactly like `pgx.ErrNoRows` — "someone else won, do nothing." The remaining insert→river-enqueue gap is identical
to the existing manual path and handled the same way (mark the row `failed`, log).

## 6. Data-model change — migration `0008_eval_trigger.sql`

Two columns plus the one-in-flight partial unique index on `evaluations`:

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

- `trigger` — `'manual' | 'milestone'`. Default `'manual'` keeps existing rows and the manual path correct.
- `trigger_milestone` — the milestone index an auto-eval fired at; `NULL` for manual.
- `evaluations_one_inflight_per_task` — guarantees at most one `queued`/`running` eval per task (§5 backstop).

Query / handler updates:
- `EnqueueEvaluation` (manual path) — stamp `trigger='manual'` explicitly (matches default; explicit for clarity).
- **`postEvaluate` (manual path) idempotency:** with the unique index, requesting a manual eval while one is already
  in flight raises `23505`. The handler catches it, fetches the in-flight row via `GetLatestEvaluation`, and returns
  it `202` (the client polls it) instead of erroring. This preserves "manual is always available, uncapped" while
  preventing concurrent duplicate flagship evals.
- `toEvaluationDTO` — unchanged; the new columns are internal bookkeeping, never surfaced to the client.

## 7. Re-home `status='evaluated'` (worker success path)

On the worker's successful `FinishEvaluation`, also flip the task status:

```sql
-- name: MarkTaskEvaluated :exec
UPDATE tasks SET status = 'evaluated' WHERE id = $1;
```

- By task id only — the worker is trusted server context and has no `user_id` (unlike the removed `SetTaskEvaluated`,
  which required `id + user_id` for an owner-scoped handler that never shipped).
- The existing unused `SetTaskEvaluated` query is **removed** as dead code.
- Called in `EvaluateWorker.Work` only on the success branch, after `FinishEvaluation`. A failed eval leaves the
  task `'active'`.
- Idempotent: re-evals keep the task `'evaluated'`; the `tasks.status` CHECK already allows only `('active','
  evaluated')`, so no enum change is needed.

`tasks.status='evaluated'` therefore means "this task has at least one completed evaluation" — a coarse marker, not
a per-eval state.

## 8. Components & files

| Unit | File | Change |
|---|---|---|
| Migration | `internal/store/migrations/0008_eval_trigger.sql` | create — `trigger`, `trigger_milestone` cols + one-in-flight unique index |
| Decision + manual queries | `internal/store/queries/evaluations.sql` | add `TryEnqueueMilestoneEvaluation`; stamp `EnqueueEvaluation` with `trigger='manual'` |
| Count queries | `internal/store/queries/cards.sql`, `messages.sql` | add `CountCompletedCards`, `CountSubstantiveTurns` |
| Task marker query | `internal/store/queries/tasks.sql` | add `MarkTaskEvaluated`; remove `SetTaskEvaluated` |
| Trigger helper + milestone math | `internal/api/eval_trigger.go` (new) | `milestoneIndex`, `maybeTriggerMilestoneEval`, `isUniqueViolation`; wire into `putCard` + `postTurn` |
| Manual idempotency | `internal/api/evaluate.go` | `postEvaluate` returns the in-flight eval on `23505` |
| Worker | `internal/agent/eval.go`, `internal/agent/evaljob.go` | add `MarkTaskEvaluated` to `EvalLifecycleStore` + `sqlcEvalStore`; call on success |
| Regenerate sqlc | `make sqlc` | new query methods + `Evaluation.Trigger string` / `TriggerMilestone *int32` fields |

## 9. Testing

- **Unit (milestone math, no DB):** `milestoneIndex(completedCards, substantiveTurns)` — integer-division
  boundaries (5→0, 6→1, 11→1, 12→2); a card pushes ≥1 immediately.
- **Store (testcontainers, serialized):** `CountCompletedCards`/`CountSubstantiveTurns` count the right rows
  (completed-only; user turns ≥20 chars only). `TryEnqueueMilestoneEvaluation` — fires on a new milestone; rejected
  when (a) an eval is `queued`/`running`, (b) cap of 3 milestone rows reached, (c) a `done` eval finished <10 min
  ago, (d) milestone not advanced; two concurrent calls insert **exactly one** row (unique-index backstop).
  `MarkTaskEvaluated` flips `tasks.status`. The unique index rejects a second in-flight `EnqueueEvaluation`.
- **Worker:** successful `Work` calls `MarkTaskEvaluated`; the failure paths do not.
- **Handler (testcontainers):** completing the first card enqueues exactly one milestone eval; a second card while
  the first is still queued is debounced (no second enqueue); the trigger never surfaces an error to the card
  response. Manual `POST /evaluate` while an eval is in flight returns the existing row `202` (no duplicate enqueue).

## 10. Non-goals & carry-forward

- ❌ No in-memory turn counter; counts are recomputed from the DB each call.
- ❌ No change to the manual `POST /evaluate` contract beyond stamping `trigger='manual'` and the in-flight
  idempotency (§6) — it stays uncapped and always returns a pollable evaluation.
- ❌ No reveal indicator / frontend (separate follow-up).
- **Carry-forward:** knob recalibration (N, debounce window, cap) once real cost/latency data exists — shared with
  the eval-model spec's cost-calibration carry-forward.

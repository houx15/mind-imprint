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
2. Calls `TryEnqueueMilestoneEvaluation` (§5) — one atomic SQL decision.
3. If a row was returned, calls `Enqueuer.EnqueueEvaluate(EvaluateArgs{EvaluationID, TaskID})`. If `pgx.ErrNoRows`,
   the guards rejected the trigger — do nothing. If the river enqueue then fails, mark the just-inserted row
   `failed` (mirroring the manual path in `evaluate.go`) and log.

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

## 5. Atomic decision — `TryEnqueueMilestoneEvaluation`

All guards live in one conditional insert so two near-simultaneous hooks (a card submit and a turn append landing
together) cannot both pass and double-enqueue. The statement inserts a `queued` row **only if** every guard holds,
and returns it; otherwise it returns no rows.

```sql
-- name: TryEnqueueMilestoneEvaluation :one
INSERT INTO evaluations (task_id, scores, narrative, model, tier, status, trigger, trigger_milestone)
SELECT $1, '[]'::jsonb, '', '', '', 'queued', 'milestone', $2
WHERE $2 > COALESCE((SELECT max(trigger_milestone) FROM evaluations
                     WHERE task_id = $1 AND trigger = 'milestone'), 0)                 -- new milestone
  AND (SELECT count(*) FROM evaluations
       WHERE task_id = $1 AND trigger = 'milestone') < $3                              -- lifetime cap
  AND NOT EXISTS (SELECT 1 FROM evaluations
                  WHERE task_id = $1 AND status IN ('queued','running'))               -- no in-flight
  AND NOT EXISTS (SELECT 1 FROM evaluations
                  WHERE task_id = $1 AND status = 'done'
                    AND completed_at > now() - interval '10 minutes')                  -- debounce
RETURNING *;
```

Params: `$1 = task_id`, `$2 = currentMilestone`, `$3 = cap (3)`. Result: the new evaluation row, or `pgx.ErrNoRows`.

The only residual race is the gap between the insert and the river enqueue — identical to the existing manual path,
and handled the same way (mark the row `failed`, log). The DB decision itself is atomic.

## 6. Data-model change — migration `0008_eval_trigger.sql`

Two columns on `evaluations`:

```sql
-- +goose Up
ALTER TABLE evaluations ADD COLUMN trigger text NOT NULL DEFAULT 'manual';
ALTER TABLE evaluations ADD COLUMN trigger_milestone integer;

-- +goose Down
ALTER TABLE evaluations DROP COLUMN trigger_milestone;
ALTER TABLE evaluations DROP COLUMN trigger;
```

- `trigger` — `'manual' | 'milestone'`. Default `'manual'` keeps existing rows and the manual path correct.
- `trigger_milestone` — the milestone index an auto-eval fired at; `NULL` for manual.

Query updates:
- `EnqueueEvaluation` (manual path) — stamp `trigger='manual'` explicitly (matches default; explicit for clarity).
- `toEvaluationDTO` — unchanged; these columns are internal bookkeeping, never surfaced to the client.

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
| Migration | `internal/store/migrations/0008_eval_trigger.sql` | create |
| Count + milestone math | `internal/agent/signals.go` or new helper | add `computeMilestone` from counts |
| Atomic insert query | `internal/store/queries/evaluations.sql` | add `TryEnqueueMilestoneEvaluation`; stamp `EnqueueEvaluation` |
| Count queries | `internal/store/queries/cards.sql`, `messages.sql` | add `CountCompletedCards`, `CountSubstantiveTurns` |
| Task marker query | `internal/store/queries/tasks.sql` | add `MarkTaskEvaluated`; remove `SetTaskEvaluated` |
| Trigger helper | `internal/api/` (shared) | add `maybeTriggerMilestoneEval`; wire into `putCard` + turn path |
| Worker | `internal/agent/evaljob.go` | call `MarkTaskEvaluated` on success |
| Regenerate sqlc | `make sqlc` | new query methods + `Evaluation.Trigger/TriggerMilestone` fields |

## 9. Testing

- **Unit (milestone math):** `currentMilestone` from `(completedCards, substantiveTurns)`; integer-division
  boundaries (5→0, 6→1, 11→1, 12→2); a card pushes ≥1 immediately; <20-char turns excluded.
- **Store (testcontainers, serialized):** `TryEnqueueMilestoneEvaluation` — fires on a new milestone; rejected when
  (a) an eval is `queued`/`running`, (b) cap of 3 milestone rows reached, (c) a `done` eval finished <10 min ago,
  (d) milestone not advanced; two concurrent calls insert **exactly one** row.
- **Worker:** successful `Work` flips `tasks.status='evaluated'`; a failing `Work` leaves it `'active'`.
- **Integration:** completing the first card enqueues exactly one milestone eval; a second card within 10 min is
  debounced; manual `POST /evaluate` still enqueues while the auto path is capped; the trigger helper never
  surfaces an error to the card/turn response.

## 10. Non-goals & carry-forward

- ❌ No in-memory turn counter; counts are recomputed from the DB each call.
- ❌ No change to the manual `POST /evaluate` contract beyond stamping `trigger='manual'`.
- ❌ No reveal indicator / frontend (separate follow-up).
- **Carry-forward:** knob recalibration (N, debounce window, cap) once real cost/latency data exists — shared with
  the eval-model spec's cost-calibration carry-forward.

# A2 — Chat Session Report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give a chat thread a student-opt-in, in-surface assessment, and in doing so scope chat's three event writes with a new `event.thread_id` so chat evidence becomes reachable — closing the temporary `surface='chat'` exemption A1 parked in `event_scope_ck`.

**Architecture:** A close parallel to A1 (course session report). A new migration `0025` adds `thread_id` to `event` + `evaluations` and re-adds `event_scope_ck` without the chat arm (still `NOT VALID`). Chat's event writes gain the thread scope; the scopeless `ChatStore.InsertUserEvent` seam is replaced by `InsertThreadEvent`. Two thread-scoped `evaluations` queries + a generalized evidence-input builder feed the existing isolated flagship assessor (`agent.Assess`, `rubric.CT()`), exposed at `GET/POST /api/v1/chat/threads/{id}/assessment`. The web chat surface gets an opt-in button and a `ChatReport` panel modeled on `CourseReport`.

**Tech Stack:** Go (`net/http`, sqlc, goose, pgx/pgtype, testcontainers), TypeScript + React + vitest, Zod contracts. Codegen via `make sqlc` from `apps/api`.

**Spec:** `docs/superpowers/specs/2026-07-18-chat-session-report-design.md` (decisions DEC-A2.1..A2.8).

## Global Constraints

- **Client never calls a model directly; API keys server-side only in `apps/api`.** Secrets never in logs/errors/migrations/payloads.
- **评估走旗舰模型绝不降级** — the assessor uses `a.d.EvalResolver` (flagship, now `deepseek-v4-pro`), never `a.d.ChatResolver` (chaperone). Assert `tier == "flagship"` in a test.
- **记录档位 + token + 成本** — every flagship call records tier/token/cost via `RecordChatLLMCall`, **even on rejection** (a rejected call still cost money).
- **RL-5** — the report is diagnostic evidence, never a grade/rank/verdict: per-dimension levels + narrative only, no total/sum/aggregate field on the DTO or row.
- **铁律 2 (不操纵)** — the report is student-triggered opt-in: no auto-generation on thread load, no nudge, no badge, no streak.
- **铁律 1 (AI 克制)** — the assessor diagnoses process; it never judges a "correct answer".
- **`event_scope_ck` stays `NOT VALID`** — pre-A2 unscoped rows have nothing to backfill FROM; grandfather them, enforce every new insert. From 0025, a chat event without `thread_id` is rejected.
- **Never hand-edit** `apps/api/internal/store/sqlc/*` — run `make sqlc` from `apps/api`. Never hand-edit generated skill/rubric embeds.
- **Go tests:** `CGO_ENABLED=0 go test -p 1 ./...` (needs `DOCKER_HOST=unix:///var/run/docker.sock` for testcontainers). Run **FULL packages**, never `-run` subsets, for card/gate/projection/migration changes.
- **Web/contracts tests** run from their OWN directory (`apps/web`, `packages/contracts`) — never a compound `cd`.
- Icons inline SVG, never lucide-react. Binding UI copy: `docs/design/思维印记_工作区.dc.html` wins over spec/plan for anything it draws; the chat report is a new surface it does not draw, so it follows `CourseReport`'s precedent.
- **NEVER `git add` a whole directory.** Stage named files only. The pre-existing `M package.json` and untracked user files under `docs/` and the repo root are NOT ours.
- Commit messages end with the `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>` trailer.

---

### Task 1: Migration 0025 — thread scope + close the chat exemption arm

**Files:**
- Create: `apps/api/internal/store/migrations/0025_thread_assessment_scope.sql`
- Create: `apps/api/internal/store/migrate_0025_test.go`

**Interfaces:**
- Consumes: `chat_thread(id, user_id, …)` (migration 0016), `event`/`evaluations` scope columns + constraints from migration 0024.
- Produces: `event.thread_id`, `evaluations.thread_id` (both nullable FK → `chat_thread(id) ON DELETE CASCADE`); `event_scope_ck = num_nonnulls(project_id, session_id, thread_id) >= 1` NOT VALID (no chat arm); `evaluations_scope_ck = num_nonnulls(task_id, project_id, session_id, thread_id) >= 1`.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0025_thread_assessment_scope.sql`:

```sql
-- +goose Up
-- A2: give the event stream and evaluations a chat-thread scope, so chat
-- evidence stops being unreachable and A1's temporary surface='chat' exemption
-- in event_scope_ck can be closed. Additive, mirroring 0024's session scope.
ALTER TABLE event       ADD COLUMN thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE;
ALTER TABLE evaluations ADD COLUMN thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE;

CREATE INDEX event_thread_created_idx ON event (thread_id, created_at);
CREATE INDEX evaluations_thread_idx   ON evaluations (thread_id, created_at DESC);

-- evaluations: ordinary widening — every existing row still satisfies it.
ALTER TABLE evaluations DROP CONSTRAINT evaluations_scope_ck;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (num_nonnulls(task_id, project_id, session_id, thread_id) >= 1);

-- event: close A1's surface='chat' exemption. From here a chat event MUST carry
-- thread_id (all three chat writes now do — migration is paired with that code
-- change in Task 2). Still NOT VALID for the same reason A1 gave: the pre-A2
-- unscoped chat/course rows have nothing to backfill FROM and stay
-- grandfathered, while every new insert — chat included — is enforced.
ALTER TABLE event DROP CONSTRAINT event_scope_ck;
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (num_nonnulls(project_id, session_id, thread_id) >= 1) NOT VALID;

-- +goose Down
ALTER TABLE event       DROP CONSTRAINT IF EXISTS event_scope_ck;
ALTER TABLE evaluations DROP CONSTRAINT IF EXISTS evaluations_scope_ck;
-- Thread-scoped reports have no other scope; the restored 0024 CHECK would
-- reject them and abort the whole Down exactly where a chat report exists.
-- Delete them explicitly (mirrors 0024's Down for session-scoped rows).
DELETE FROM evaluations
 WHERE thread_id IS NOT NULL AND task_id IS NULL AND project_id IS NULL AND session_id IS NULL;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (num_nonnulls(task_id, project_id, session_id) >= 1);
-- Restore 0024's event_scope_ck INCLUDING the chat exemption — dropping
-- thread_id reverts chat's writes to unscoped, so the arm must return or the
-- restored constraint would reject every new chat insert.
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (surface = 'chat' OR num_nonnulls(project_id, session_id) >= 1) NOT VALID;
DROP INDEX IF EXISTS event_thread_created_idx;
DROP INDEX IF EXISTS evaluations_thread_idx;
ALTER TABLE event       DROP COLUMN IF EXISTS thread_id;
ALTER TABLE evaluations DROP COLUMN IF EXISTS thread_id;
```

- [ ] **Step 2: Write the migration test (Up + seeded Down)**

Create `apps/api/internal/store/migrate_0025_test.go`. This mirrors `migrate_0024_test.go` (read it first for `newTestPool`, `refactor2SeededStudentID`, `migrationFS`), with the KEY inversion: 0024 proved an unscoped chat event is *accepted* (the exemption); 0025 proves it is now *rejected* (the exemption is closed). The Down test proves the exemption is *restored*.

```go
package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0025ThreadScope: event and evaluations gain a nullable
// thread_id, evaluations_scope_ck widens to include it, and event_scope_ck is
// re-added WITHOUT A1's surface='chat' arm — so a new unscoped chat event is
// now rejected (the hole A2 closes), while pre-A2 unattributable rows stay
// grandfathered (still NOT VALID).
func TestMigration0025ThreadScope(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	var threadID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO chat_thread (user_id, title) VALUES ($1, '') RETURNING id::text`,
		refactor2SeededStudentID).Scan(&threadID); err != nil {
		t.Fatalf("seed chat_thread: %v", err)
	}

	// 1. A thread-scoped event satisfies event_scope_ck with only thread_id.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, thread_id, surface, type, payload)
		VALUES ($1, $2, 'chat', 'prompt_sent', '{}'::jsonb)`,
		refactor2SeededStudentID, threadID); err != nil {
		t.Fatalf("thread-scoped chat event must satisfy event_scope_ck: %v", err)
	}

	// 2. A brand-new UNSCOPED chat event is now REJECTED — the arm A1 parked is
	// gone. This is the whole point of A2's constraint change.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'chat', 'prompt_sent', '{}'::jsonb)`,
		refactor2SeededStudentID); err == nil {
		t.Fatal("a new unscoped chat event should now violate event_scope_ck (exemption closed), got no error")
	}

	// 3. NOT VALID's contract still holds: a pre-existing unattributable row
	// reads fine. Simulate one via the only path it could exist — constraint
	// dropped, row inserted, constraint re-added NOT VALID over it.
	if _, err := pool.Exec(ctx, `ALTER TABLE event DROP CONSTRAINT event_scope_ck`); err != nil {
		t.Fatalf("drop constraint: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'chat', 'prompt_sent', '{}'::jsonb)`, refactor2SeededStudentID); err != nil {
		t.Fatalf("seed legacy unattributable chat row: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		ALTER TABLE event ADD CONSTRAINT event_scope_ck
		CHECK (num_nonnulls(project_id, session_id, thread_id) >= 1) NOT VALID`); err != nil {
		t.Fatalf("re-add NOT VALID over a legacy row — the point of NOT VALID: %v", err)
	}
	var legacy int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM event
		WHERE surface = 'chat' AND project_id IS NULL AND session_id IS NULL AND thread_id IS NULL`).Scan(&legacy); err != nil {
		t.Fatalf("read legacy unattributable chat rows: %v", err)
	}
	if legacy != 1 {
		t.Fatalf("legacy unattributable chat rows = %d, want 1 (grandfathered, never deleted)", legacy)
	}

	// 4. evaluations accepts a thread-scoped row and still rejects a scopeless one.
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (thread_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'chat narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		threadID); err != nil {
		t.Fatalf("thread-scoped evaluation must satisfy evaluations_scope_ck: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (scores, narrative, model, tier, status)
		VALUES ('[]'::jsonb, 'unscoped', 'deepseek-v4-pro', 'flagship', 'done')`); err == nil {
		t.Fatal("evaluation with every scope NULL should violate evaluations_scope_ck, got no error")
	}
}

// TestMigration0025Down verifies 0025 reverses cleanly WITH a thread-scoped
// report present — the load-bearing case (a scopeless Down test would pass for
// the wrong reason, exactly the gap A1's whole-branch review caught in 0024).
// It also asserts the surface='chat' exemption is RESTORED: after Down an
// unscoped chat event must be accepted again.
func TestMigration0025Down(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	var threadID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO chat_thread (user_id, title) VALUES ($1, '') RETURNING id::text`,
		refactor2SeededStudentID).Scan(&threadID); err != nil {
		t.Fatalf("seed chat_thread: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (thread_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'chat narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		threadID); err != nil {
		t.Fatalf("seed thread-scoped evaluation (what a real chat report writes): %v", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}

	if err := goose.DownContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose down 0025 with a thread-scoped evaluation present: %v", err)
	}

	// Columns and the thread indexes are gone.
	for _, q := range []struct{ name, sql string }{
		{"event.thread_id", `SELECT 1 FROM information_schema.columns WHERE table_name='event' AND column_name='thread_id'`},
		{"evaluations.thread_id", `SELECT 1 FROM information_schema.columns WHERE table_name='evaluations' AND column_name='thread_id'`},
	} {
		var one int
		if err := pool.QueryRow(ctx, q.sql).Scan(&one); err == nil {
			t.Fatalf("%s still present after Down", q.name)
		}
	}

	// The surface='chat' exemption is restored: an unscoped chat event is
	// accepted again (0024's state), otherwise chat would silently stop
	// recording between Down and a re-Up.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'chat', 'prompt_sent', '{}'::jsonb)`, refactor2SeededStudentID); err != nil {
		t.Fatalf("after Down the surface='chat' exemption must be restored: %v", err)
	}

	// And Up restores the thread scope.
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose up after down: %v", err)
	}
	var one int
	if err := pool.QueryRow(ctx, `
		SELECT 1 FROM information_schema.columns WHERE table_name='event' AND column_name='thread_id'`).Scan(&one); err != nil {
		t.Fatalf("event.thread_id missing after re-Up: %v", err)
	}
}
```

- [ ] **Step 3: Run the migration tests**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/ -run 'TestMigration0025'`
Expected: PASS (both Up and Down). If Down fails with a CHECK-violation abort, the `DELETE FROM evaluations` ordering is wrong — it must run before the constraint is re-added.

- [ ] **Step 4: Prove the Down DELETE is load-bearing**

Temporarily comment out the `DELETE FROM evaluations …` line in the Down block, re-run `TestMigration0025Down`, confirm it now FAILS (the seeded thread-scoped row aborts the restored CHECK), then restore the line. This guards against the scopeless-Down-passing-for-the-wrong-reason trap.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/migrations/0025_thread_assessment_scope.sql apps/api/internal/store/migrate_0025_test.go
git commit -m "feat(a2): migration 0025 — chat-thread scope; close the surface='chat' exemption

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Scope chat's event writes + `RecordChatLLMCall` purpose

**Files:**
- Modify: `apps/api/internal/store/queries/event.sql` (AppendEvent gains `thread_id`; add `ListEventsByThread`)
- Modify: `apps/api/internal/agent/chatstore.go` (replace `InsertUserEvent` → `InsertThreadEvent`; add `purpose` to `RecordChatLLMCall`)
- Modify: `apps/api/internal/agent/chat_step.go` (interface + call sites)
- Modify: `apps/api/internal/api/chat.go` (two direct `AppendEvent` writes pass `thread_id`)
- Regenerate: `apps/api/internal/store/sqlc/*` via `make sqlc`
- Modify: `apps/api/internal/agent/chat_step_test.go` (fake `ChatStore` implements the new signatures)
- Test: reuse `apps/api/internal/agent/chat_step_test.go`; add scoping assertions in `apps/api/internal/store/` (a `sqlcChatStore`-level test)

**Interfaces:**
- Consumes: `event.thread_id` (Task 1); `sqlc.AppendEventParams` (regenerated with `ThreadID pgtype.UUID`); `q.GetThread(ctx, id) (sqlc.ChatThread, error)` (exists, used by `loadOwnedThread`).
- Produces:
  - `ChatStore.InsertThreadEvent(ctx context.Context, threadID uuid.UUID, typ string, payload []byte) error` — resolves `user_id` from `chat_thread`, hard-codes `surface='chat'`, sets `thread_id`.
  - `ChatStore.RecordChatLLMCall(ctx context.Context, userID uuid.UUID, purpose string, resolved gateway.Resolved, prompt, completion int32) error`
  - `q.ListEventsByThread(ctx, thread_id pgtype.UUID) ([]sqlc.Event, error)`

- [ ] **Step 1: Edit the event queries**

In `apps/api/internal/store/queries/event.sql`, change `AppendEvent` to carry `thread_id` and add `ListEventsByThread`:

```sql
-- name: AppendEvent :one
INSERT INTO event (project_id, user_id, session_id, thread_id, surface, type, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListEventsByProject :many
SELECT * FROM event
WHERE project_id = $1
ORDER BY created_at, id;

-- name: ListEventsBySession :many
SELECT * FROM event
WHERE session_id = $1
ORDER BY created_at, id;

-- name: ListEventsByThread :many
SELECT * FROM event
WHERE thread_id = $1
ORDER BY created_at, id;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc`
Expected: `AppendEventParams` gains a `ThreadID pgtype.UUID` field; `ListEventsByThread` is generated. Do NOT hand-edit `internal/store/sqlc/*`.

- [ ] **Step 3: Verify the build fails only where intended (grep, do not trust the compiler)**

Adding a field to the keyed `AppendEventParams` literal keeps every existing caller compiling with `ThreadID` defaulted to `{Valid:false}`. That is correct for project/course/studio callers but would silently leave a chat write unscoped. Grep every site:

Run: `cd apps/api && grep -rn "AppendEventParams{" internal/`
Expected sites: `internal/agent/chatstore.go` (InsertUserEvent — about to become InsertThreadEvent), `internal/agent/coursestore.go` (InsertSessionEvent — sets SessionID, leave ThreadID default), `internal/api/chat.go` (×2 direct chat writes — MUST set ThreadID), plus any project/studio writer (sets ProjectID, leave ThreadID default). Confirm each chat site sets `ThreadID` after this task; every non-chat site is correct with the default.

- [ ] **Step 4: Rewrite the `chatstore.go` seam**

In `apps/api/internal/agent/chatstore.go`, replace `InsertUserEvent` with `InsertThreadEvent` and add `purpose` to `RecordChatLLMCall`:

```go
func (s *sqlcChatStore) InsertThreadEvent(ctx context.Context, threadID uuid.UUID, typ string, payload []byte) error {
	th, err := s.q.GetThread(ctx, threadID)
	if err != nil {
		return err
	}
	_, err = s.q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false},
		UserID:    th.UserID,
		SessionID: pgtype.UUID{Valid: false},
		ThreadID:  pgUUID(threadID),
		Surface:   "chat",
		Type:      typ,
		Payload:   payload,
	})
	return err
}

func (s *sqlcChatStore) RecordChatLLMCall(ctx context.Context, userID uuid.UUID, purpose string, resolved gateway.Resolved, prompt, completion int32) error {
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, int(prompt), int(completion))
	if !priced {
		slog.Warn("chat llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
	}
	_, err := s.q.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
		UserID: userID, ProjectID: pgtype.UUID{Valid: false},
		Surface: "chat", Purpose: purpose,
		Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
		PromptTokens: prompt, CompletionTokens: completion, CostEstimate: gateway.CostNumeric(cost, true),
	})
	return err
}
```

(Leave the `CostNumeric(cost, true)` hard-coding as-is — the latent-bug fix is an explicit out-of-scope carry-forward, spec §5.)

- [ ] **Step 5: Update the `ChatStore` interface and its call sites**

In `apps/api/internal/agent/chat_step.go`, update the interface (lines ~79-80):

```go
	InsertThreadEvent(ctx context.Context, threadID uuid.UUID, typ string, payload []byte) error
	RecordChatLLMCall(ctx context.Context, userID uuid.UUID, purpose string, resolved gateway.Resolved, prompt, completion int32) error
```

Update the `RecordChatLLMCall` call site (line ~157) to pass `"coach"`:

```go
		if rerr := deps.Store.RecordChatLLMCall(ctx, deps.UserID, "coach", deps.Resolved, int32(usage.InputTokens), int32(usage.OutputTokens)); rerr != nil {
```

Update the `card_surfaced` write (lines ~180-183) to the new seam (drops the `userID`/`surface` args — the impl resolves user from the thread and hard-codes surface):

```go
		payload, _ := json.Marshal(map[string]string{"card_id": cardID})
		if err := deps.Store.InsertThreadEvent(ctx, deps.ThreadID, "card_surfaced", payload); err != nil {
			slog.Warn("chat: append card_surfaced event failed", "thread_id", deps.ThreadID.String(), "err", err.Error())
		}
```

- [ ] **Step 6: Scope the two direct writes in `chat.go`**

In `apps/api/internal/api/chat.go`, the `prompt_sent` write (lines ~195-202) — set `ThreadID`, update the comment:

```go
	if _, err := a.d.Queries.AppendEvent(r.Context(), sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false}, UserID: u.ID,
		SessionID: pgtype.UUID{Valid: false},
		ThreadID:  pgtype.UUID{Bytes: threadID, Valid: true}, // A2: chat events are thread-scoped
		Surface:   "chat", Type: "prompt_sent", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("chat turn: append prompt_sent event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
```

The `card_completed` write (lines ~269-276) — same treatment:

```go
	if _, err := a.d.Queries.AppendEvent(r.Context(), sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false}, UserID: u.ID,
		SessionID: pgtype.UUID{Valid: false},
		ThreadID:  pgtype.UUID{Bytes: threadID, Valid: true}, // A2: chat events are thread-scoped
		Surface:   "chat", Type: "card_completed", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("chat card submit: append card_completed event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
```

- [ ] **Step 7: Update the in-memory fake `ChatStore`**

In `apps/api/internal/agent/chat_step_test.go`, find the fake that implements `ChatStore`. Rename its `InsertUserEvent` method to `InsertThreadEvent(ctx context.Context, threadID uuid.UUID, typ string, payload []byte) error` and record the `threadID`/`typ`/`payload` it receives (so a test can assert `card_surfaced` was written with the thread id). Add the `purpose string` parameter to its `RecordChatLLMCall`. Keep behavior otherwise identical.

- [ ] **Step 8: Assert the fake captured a thread-scoped `card_surfaced`**

In the existing `chat_step_test.go` test that drives a card-offer moment, assert the fake recorded exactly one `InsertThreadEvent` call with `typ == "card_surfaced"` and the deps' `ThreadID`. (If no such test exists, add the assertion to the offer-path test.)

- [ ] **Step 9: Add a store-level scoping test**

Create or extend a `sqlcChatStore` test in `apps/api/internal/store/` (or `internal/agent/`, wherever `sqlcChatStore` is exercised against a testcontainer). Seed a `chat_thread`, call `InsertThreadEvent`, then `ListEventsByThread(threadID)` and assert exactly one row with `Surface=="chat"`, `Type` as written, and `ThreadID.Valid`. This proves the write is reachable — the whole point of A2.

```go
// Sketch — adapt to the package's existing testcontainer harness:
store := agent.NewSqlcChatStore(sqlc.New(pool))
// seed a chat_thread for refactor2SeededStudentID, get threadID (uuid.UUID)
if err := store.InsertThreadEvent(ctx, threadID, "prompt_sent", []byte(`{}`)); err != nil {
	t.Fatalf("InsertThreadEvent: %v", err)
}
rows, err := sqlc.New(pool).ListEventsByThread(ctx, pgtype.UUID{Bytes: threadID, Valid: true})
if err != nil || len(rows) != 1 {
	t.Fatalf("ListEventsByThread = %d rows, err=%v; want 1 reachable event", len(rows), err)
}
```

- [ ] **Step 10: Run the affected packages FULL (not -run subsets)**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/ ./internal/api/ ./internal/store/`
Expected: PASS. (Full packages because this is a card/gate/projection-adjacent change and the AppendEvent signature is cross-cutting.)

- [ ] **Step 11: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/queries/event.sql apps/api/internal/store/sqlc/ apps/api/internal/agent/chatstore.go apps/api/internal/agent/chat_step.go apps/api/internal/agent/chat_step_test.go apps/api/internal/api/chat.go apps/api/internal/store/
git commit -m "feat(a2): scope chat's event writes with thread_id; RecordChatLLMCall gains purpose

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```
(Stage the specific new/changed test files by name; do not `git add` a bare directory that might sweep in unrelated changes — list the exact test file paths you touched.)

---

### Task 3: Evidence-input builder generalization + thread evaluation queries

**Files:**
- Modify: `apps/api/internal/api/course_assessment_input.go` (rename `buildAssessmentInputFromSession` → `buildAssessmentInputFromEvidence`; rename helpers `cardUsesFromSession`/`dispositionUsesFromSession` → `…FromEvidence`)
- Modify: `apps/api/internal/api/course_assessment.go` (update the one caller)
- Modify: `apps/api/internal/api/course_assessment_input_test.go` (update the test to the new names)
- Modify: `apps/api/internal/store/queries/evaluation.sql` (add `InsertThreadEvaluation`, `GetLatestThreadEvaluation`)
- Regenerate: `apps/api/internal/store/sqlc/*` via `make sqlc`

**Interfaces:**
- Consumes: `agent.BuildAssessmentInput`, `studio.Event`, `sqlc.CardInstance`.
- Produces:
  - `buildAssessmentInputFromEvidence(events []studio.Event, cards []sqlc.CardInstance) agent.AssessmentInput` (same body as the old `…FromSession` — course and chat both have events+cards, no gates/graph).
  - `q.InsertThreadEvaluation(ctx, InsertThreadEvaluationParams) (sqlc.Evaluation, error)`
  - `q.GetLatestThreadEvaluation(ctx, thread_id pgtype.UUID) (sqlc.Evaluation, error)`

- [ ] **Step 1: Rename the builder and helpers (DRY — no parallel duplicate)**

A new `buildAssessmentInputFromThread` would be a verbatim copy of `buildAssessmentInputFromSession` — a duplication a reviewer must reject. Instead, generalize the name. In `apps/api/internal/api/course_assessment_input.go`, rename:
- `buildAssessmentInputFromSession` → `buildAssessmentInputFromEvidence`
- `cardUsesFromSession` → `cardUsesFromEvidence`
- `dispositionUsesFromSession` → `dispositionUsesFromEvidence`

Update the doc comment's first line to: `// buildAssessmentInputFromEvidence maps an event stream + cards onto agent.BuildAssessmentInput's primitive slices — shared by the course session (A1) and chat thread (A2) reports, whose evidence is exactly events + cards, no gates/graph.` Keep the body identical (it already calls `eventDigestsFromProject(events)`).

- [ ] **Step 2: Update the course caller**

In `apps/api/internal/api/course_assessment.go`, line ~86, change `buildAssessmentInputFromSession(events, cards)` → `buildAssessmentInputFromEvidence(events, cards)`.

- [ ] **Step 3: Update the builder's existing test**

In `apps/api/internal/api/course_assessment_input_test.go`, update every call/reference to the renamed functions. Then add a chat-shaped case asserting the shared builder produces NA output dimensions for chat evidence:

```go
func TestBuildAssessmentInputFromEvidence_ChatShape(t *testing.T) {
	events := []studio.Event{
		{Type: "prompt_sent", Surface: "chat", Payload: json.RawMessage(`{}`)},
		{Type: "card_surfaced", Surface: "chat", Payload: json.RawMessage(`{"card_id":"sift_craap"}`)},
	}
	cards := []sqlc.CardInstance{{CardID: "sift_craap", Status: "completed"}}
	in := buildAssessmentInputFromEvidence(events, cards)
	// A chat thread has no gates/word-counts/review-bands/graph — those args are
	// empty so Assess reports the writing dimensions NA (spec §DEC-A2.4).
	if len(in.GateProgress) != 0 || len(in.WordCounts) != 0 || in.GraphSummary != "" {
		t.Fatalf("chat evidence must carry no gate/wordcount/graph args: %+v", in)
	}
	if len(in.CardUses) != 1 || in.CardUses[0].CardID != "sift_craap" {
		t.Fatalf("card evidence not mapped: %+v", in.CardUses)
	}
}
```

(Confirm the exact field names on `agent.AssessmentInput` — `GateProgress`/`WordCounts`/`GraphSummary`/`CardUses` — against `agent.BuildAssessmentInput`'s signature; adjust the assertions to the real fields if they differ.)

- [ ] **Step 4: Add the thread evaluation queries**

In `apps/api/internal/store/queries/evaluation.sql`, append (after the session-scoped pair):

```sql
-- Chat thread scope (A2): mirrors the session-scoped pair above. A chat
-- thread's report is one evaluations row scoped by thread_id alone.

-- name: InsertThreadEvaluation :one
INSERT INTO evaluations (thread_id, scores, narrative, model, tier,
  prompt_tokens, completion_tokens, cost_estimate, status)
VALUES (@thread_id, @scores, @narrative, @model, @tier,
  @prompt_tokens, @completion_tokens, @cost_estimate, 'done')
RETURNING *;

-- name: GetLatestThreadEvaluation :one
SELECT * FROM evaluations
WHERE thread_id = @thread_id
ORDER BY created_at DESC
LIMIT 1;
```

- [ ] **Step 5: Regenerate sqlc**

Run: `cd apps/api && make sqlc`
Expected: `InsertThreadEvaluationParams`, `InsertThreadEvaluation`, `GetLatestThreadEvaluation` generated.

- [ ] **Step 6: Run the api + store packages**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ ./internal/store/`
Expected: PASS (the renamed builder + its new chat-shape test; codegen compiles).

- [ ] **Step 7: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/course_assessment_input.go apps/api/internal/api/course_assessment.go apps/api/internal/api/course_assessment_input_test.go apps/api/internal/store/queries/evaluation.sql apps/api/internal/store/sqlc/
git commit -m "refactor(a2): generalize evidence-input builder; add thread evaluation queries

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Chat assessment endpoints + routes

**Files:**
- Create: `apps/api/internal/api/chat_assessment.go` (`getChatAssessment`, `generateChatAssessment`)
- Modify: `apps/api/internal/api/api.go` (register two routes after the existing chat routes)
- Create: `apps/api/internal/api/chat_assessment_test.go`

**Interfaces:**
- Consumes: `a.loadOwnedThread` (chat.go), `HasEntitlement`, `a.d.EvalResolver`, `agent.Assess`, `rubric.CT()`, `agent.EmbeddedAnchors()`, `buildAssessmentInputFromEvidence` (Task 3), `q.ListEventsByThread` (Task 2), `q.ListCardInstancesByThread`, `q.InsertThreadEvaluation`/`q.GetLatestThreadEvaluation` (Task 3), `agent.NewSqlcChatStore(...).RecordChatLLMCall` (Task 2), `dtoFromEvaluationRow` (assessment.go), `gateway.EstimateCost`/`CostNumeric`.
- Produces: `GET /api/v1/chat/threads/{id}/assessment`, `POST /api/v1/chat/threads/{id}/assessment`.

- [ ] **Step 1: Write the failing endpoint test**

Create `apps/api/internal/api/chat_assessment_test.go`. Mirror `course_assessment_test.go`'s harness (`assessStubProvider`, `assessReply`, `fakeResolver`, `fakeEvalResolver`, `signInSeed`, `withCookie`, `newAPITestPool`). Chat needs a thread created via the API (no seeded thread fixture). Helper to create a thread:

```go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func countChatAssessmentLLMCalls(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE surface = 'chat' AND purpose = 'assessment'`).Scan(&n); err != nil {
		t.Fatalf("count chat assessment llm_call: %v", err)
	}
	return n
}

// createThread POSTs /chat/threads and returns the new thread id.
func createThread(t *testing.T, h http.Handler, cookie *http.Cookie) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/chat/threads", strings.NewReader(`{"title":""}`)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create thread = %d; body=%s", rec.Code, rec.Body)
	}
	var dto struct{ ID string `json:"id"` }
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil || dto.ID == "" {
		t.Fatalf("decode thread id: %v; body=%s", err, rec.Body)
	}
	return dto.ID
}

// TestGetChatAssessment_EmptyBeforeGenerate — "not yet assessed" is 200 + null,
// never a 404, and ZERO llm_call rows (the read path never calls a model).
func TestGetChatAssessment_EmptyBeforeGenerate(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	threadID := createThread(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/chat/threads/"+threadID+"/assessment", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("GET body = %s, want literal null", rec.Body)
	}
	if n := countChatAssessmentLLMCalls(t, pool); n != 0 {
		t.Fatalf("llm_call rows after GET = %d, want 0", n)
	}
}

// TestGenerateChatAssessment_PersistsAtThreadScope — one flagship call, metered
// surface=chat/purpose=assessment, persisted so GET replays it with no 2nd call.
func TestGenerateChatAssessment_PersistsAtThreadScope(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(assessReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	threadID := createThread(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/chat/threads/"+threadID+"/assessment", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var dto struct {
		Dimensions []struct{ Code, Name, Level, Evidence string } `json:"dimensions"`
		Narrative   string `json:"narrative"`
		GeneratedAt string `json:"generatedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode DTO: %v — body=%s", err, rec.Body)
	}
	if len(dto.Dimensions) != 10 {
		t.Fatalf("dimensions len = %d, want 10 (every rubric dimension, unevidenced ones NA)", len(dto.Dimensions))
	}
	if dto.Narrative == "" || dto.GeneratedAt == "" {
		t.Fatalf("dto = %+v, want narrative + generatedAt", dto)
	}

	// surface=chat, purpose=assessment, tier=flagship (评估走旗舰模型绝不降级).
	var surface, purpose, tier string
	if err := pool.QueryRow(context.Background(),
		`SELECT surface, purpose, tier FROM llm_call WHERE surface='chat' AND purpose='assessment'
		 ORDER BY created_at DESC LIMIT 1`).Scan(&surface, &purpose, &tier); err != nil {
		t.Fatalf("read llm_call: %v", err)
	}
	if surface != "chat" || purpose != "assessment" {
		t.Fatalf("llm_call = (%s,%s), want (chat,assessment)", surface, purpose)
	}
	if tier != "flagship" {
		t.Fatalf("assessment ran on tier %q, want flagship — 评估走旗舰模型绝不降级", tier)
	}

	// Persisted at thread scope: GET replays it, still exactly one llm_call.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("GET", "/api/v1/chat/threads/"+threadID+"/assessment", nil), cookie))
	if strings.TrimSpace(rec2.Body.String()) == "null" {
		t.Fatal("GET after POST returned null — the report was not persisted at thread scope")
	}
	if n := countChatAssessmentLLMCalls(t, pool); n != 1 {
		t.Fatalf("llm_call rows after POST+GET = %d, want exactly 1", n)
	}
}

// TestGenerateChatAssessment_RecordsCostOnRejection — a rejected call still cost
// money, so the llm_call row exists even though nothing is persisted. The
// narrative contains "你应该这样写" — verified to trip enforcement's
// "rewritten-sentence-zh" rule (banned_phrasing.go), the same fixture A1 used
// after its infidelity fix.
func TestGenerateChatAssessment_RecordsCostOnRejection(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     assessStubProvider(`{"dimensions":[],"narrative":"你应该这样写：先摆结论，再给证据。"}`),
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	threadID := createThread(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/chat/threads/"+threadID+"/assessment", strings.NewReader("")), cookie))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST (banned phrasing) = %d, want 422; body=%s", rec.Code, rec.Body)
	}
	if n := countChatAssessmentLLMCalls(t, pool); n != 1 {
		t.Fatalf("llm_call rows after rejection = %d, want 1 — a rejected call still cost money", n)
	}
}

// TestChatAssessment_UnknownThread404s — an unowned/nonexistent thread id gets
// 404 and leaks nothing (loadOwnedThread resolves by caller).
func TestChatAssessment_UnknownThread404s(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET",
		"/api/v1/chat/threads/00000000-0000-0000-0000-0000000000ff/assessment", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET unknown thread = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run 'TestGetChatAssessment|TestGenerateChatAssessment|TestChatAssessment'`
Expected: FAIL (handlers + routes undefined → 404 on POST/GET assessment, or compile error).

- [ ] **Step 3: Write the handlers**

Create `apps/api/internal/api/chat_assessment.go` (mirrors `course_assessment.go`, thread-scoped):

```go
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/rubric"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// getChatAssessment returns the thread's most recently generated report, or an
// explicit JSON null when none exists — the report view's own empty state,
// never a 404. No model call, ever. Thread-scoped sibling of getCourseAssessment.
func (a *API) getChatAssessment(w http.ResponseWriter, r *http.Request) {
	threadID, ok := a.loadOwnedThread(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetLatestThreadEvaluation(r.Context(), pgtype.UUID{Bytes: threadID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, http.StatusOK, nil)
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	dto, derr := dtoFromEvaluationRow(row)
	if derr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// generateChatAssessment runs the isolated flagship assessor over one chat
// thread's evidence: load the thread's events + cards, digest, ONE flagship call
// (never downgraded), record the cost regardless of outcome, and — only on
// success — persist at thread scope and return. Student-opt-in (铁律 2): only
// ever reached by an explicit POST, never auto-run.
func (a *API) generateChatAssessment(w http.ResponseWriter, r *http.Request) {
	threadID, ok := a.loadOwnedThread(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	tid := pgtype.UUID{Bytes: threadID, Valid: true}
	eventRows, err := a.d.Queries.ListEventsByThread(r.Context(), tid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	events := make([]studio.Event, len(eventRows))
	for i, ev := range eventRows {
		events[i] = studio.Event{
			Type: ev.Type, Surface: ev.Surface,
			Payload: json.RawMessage(ev.Payload), CreatedAt: ev.CreatedAt,
		}
	}
	cards, err := a.d.Queries.ListCardInstancesByThread(r.Context(), tid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	in := buildAssessmentInputFromEvidence(events, cards)

	resolved, rerr := a.d.EvalResolver(r.Context())
	if rerr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	assessment, usage, aerr := agent.Assess(r.Context(), a.d.Provider, resolved, rubric.CT(), in, agent.EmbeddedAnchors())

	// Record cost even if enforcement then rejects — a rejected call still cost
	// money (mirrors generateCourseAssessment).
	store := agent.NewSqlcChatStore(a.d.Queries)
	if resolved.Provider != "" {
		if err := store.RecordChatLLMCall(r.Context(), u.ID, "assessment", resolved,
			int32(usage.InputTokens), int32(usage.OutputTokens)); err != nil {
			slog.Warn("generate_chat_assessment: record llm call", "err", err)
		}
	}
	if aerr != nil {
		slog.Warn("generate_chat_assessment: rejected", "err", aerr)
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "assessment_rejected",
			Message: "这次评估没通过内部校验，请再试一次",
		})
		return
	}

	scoresJSON, merr := json.Marshal(assessment.Dimensions)
	if merr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
	promptTokens := int32(usage.InputTokens)
	completionTokens := int32(usage.OutputTokens)
	row, err := a.d.Queries.InsertThreadEvaluation(r.Context(), sqlc.InsertThreadEvaluationParams{
		ThreadID:         tid,
		Scores:           scoresJSON,
		Narrative:        assessment.Narrative,
		Model:            resolved.Model,
		Tier:             resolved.Tier,
		PromptTokens:     &promptTokens,
		CompletionTokens: &completionTokens,
		CostEstimate:     gateway.CostNumeric(cost, priced),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, derr := dtoFromEvaluationRow(row)
	if derr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}
```

(Confirm `RecordChatLLMCall`'s param order against Task 2: `(ctx, userID, purpose, resolved, prompt, completion)`. Confirm `InsertThreadEvaluationParams` field names against the regenerated sqlc — mirror `InsertSessionEvaluationParams`.)

- [ ] **Step 4: Register the routes**

In `apps/api/internal/api/api.go`, after the existing chat routes (the `POST …/cards/{cid}/skip` line ~91), add:

```go
	mux.Handle("GET /api/v1/chat/threads/{id}/assessment", protected(a.getChatAssessment))
	mux.Handle("POST /api/v1/chat/threads/{id}/assessment", protected(a.generateChatAssessment))
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run 'TestGetChatAssessment|TestGenerateChatAssessment|TestChatAssessment'`
Expected: PASS (all four tests).

- [ ] **Step 6: Run the full api package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/chat_assessment.go apps/api/internal/api/chat_assessment_test.go apps/api/internal/api/api.go
git commit -m "feat(a2): chat assessment endpoints (GET/POST /chat/threads/{id}/assessment)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Web — opt-in button + ChatReport panel

**Files:**
- Create: `apps/web/src/api/chatAssessment.ts` (`getChatAssessment`, `generateChatAssessment`)
- Modify: `apps/web/src/api/index.ts` (import + interface + object entries)
- Create: `apps/web/src/shell/chat/ChatReport.tsx`
- Modify: `apps/web/src/shell/chat/ChatSurface.tsx` (opt-in button in the thread header + two new props)
- Modify: `apps/web/src/shell/chat/ChatContainer.tsx` (report-open state, gate, conditional render)
- Modify/Create: `apps/web/src/shell/chat/ChatReport.test.tsx` and extend `ChatSurface.test.tsx`

**Interfaces:**
- Consumes: `Assessment`, `DimensionScore` from `@mind-imprint/contracts`; `api.getChatAssessment`/`api.generateChatAssessment`.
- Produces: `ChatReport({ threadId, onClose })`; `ChatSurface` props `canOpenReport: boolean`, `onOpenReport: () => void`.

- [ ] **Step 1: Write the api client**

Create `apps/web/src/api/chatAssessment.ts` (mirrors `courseAssessment.ts`, thread-scoped):

```ts
import { Assessment } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// A2: the chat thread's report. Student-opt-in — generateChatAssessment is only
// ever called from an explicit click, never on load (铁律 2).
export async function getChatAssessment(threadId: string): Promise<Assessment | null> {
  const raw = await apiFetch<unknown>(`/api/v1/chat/threads/${threadId}/assessment`);
  if (raw == null) return null;
  return Assessment.parse(raw);
}

export async function generateChatAssessment(threadId: string): Promise<Assessment> {
  const raw = await apiFetch<unknown>(`/api/v1/chat/threads/${threadId}/assessment`, { method: "POST" });
  return Assessment.parse(raw);
}
```

- [ ] **Step 2: Wire the api barrel**

In `apps/web/src/api/index.ts`:
- Add import after line 20: `import { getChatAssessment, generateChatAssessment } from "./chatAssessment";`
- Add to the `ApiClient` interface (after `chatTurn`, ~line 68): 
  ```ts
  getChatAssessment(threadId: string): Promise<Assessment | null>;
  generateChatAssessment(threadId: string): Promise<Assessment>;
  ```
- Add to the `api` object (in the chat group, ~line 89): `getChatAssessment, generateChatAssessment,`

- [ ] **Step 3: Write the ChatReport component**

Create `apps/web/src/shell/chat/ChatReport.tsx`. It owns its own fetch/generate lifecycle — opt-in, so it does NOT auto-generate on load; it GETs, and only POSTs when the student clicks 生成/重新生成. Reuse `CourseReport`'s `LIT_SEGMENT_COUNT` + `DimensionRow` rendering (dc.html:2643 lvlBar; NA → 未涉及; RL-5 diagnostic-only).

```tsx
import { useEffect, useState } from "react";
import type { Assessment, DimensionScore } from "@mind-imprint/contracts";
import { api } from "../../api";

const LIT_SEGMENT_COUNT: Record<DimensionScore["level"], number> = { L1: 1, L2: 2, L3: 3, L4: 4, NA: 0 };

function DimensionRow({ dim }: { dim: DimensionScore }) {
  const lit = LIT_SEGMENT_COUNT[dim.level];
  return (
    <div style={{ padding: "11px 0", borderBottom: "1px solid #F3F4F7" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 8 }}>
        <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{dim.name}</span>
        {dim.level === "NA" ? (
          <span style={{ fontSize: 12, fontWeight: 700, color: "#9AA1B0", background: "#F3F4F7", padding: "2px 10px", borderRadius: 999 }}>未涉及</span>
        ) : (
          <span style={{ fontSize: 12, fontWeight: 700, color: "#D98263", background: "#FBEEE7", padding: "2px 10px", borderRadius: 999 }}>{dim.level}</span>
        )}
      </div>
      <div style={{ display: "flex", gap: 5, marginBottom: 6 }}>
        {[1, 2, 3, 4].map((n) => (
          <span key={n} style={{ flex: 1, height: 6, borderRadius: 3, background: n <= lit ? "#D98263" : "#ECEEF4" }} />
        ))}
      </div>
      <div style={{ fontSize: 12.5, color: "#8A92A3" }}>{dim.evidence}</div>
    </div>
  );
}

export function ChatReport({ threadId, onClose }: { threadId: string; onClose: () => void }) {
  const [assessment, setAssessment] = useState<Assessment | null>(null);
  const [state, setState] = useState<"loading" | "empty" | "generating" | "ready" | "error">("loading");

  // GET on mount — show a stored report if one exists, else the opt-in CTA.
  // Never auto-POST: generation is always the student's explicit click (铁律 2).
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const a = await api.getChatAssessment(threadId);
        if (cancelled) return;
        if (a) { setAssessment(a); setState("ready"); } else { setState("empty"); }
      } catch {
        if (!cancelled) setState("error");
      }
    })();
    return () => { cancelled = true; };
  }, [threadId]);

  async function generate() {
    setState("generating");
    try {
      const a = await api.generateChatAssessment(threadId);
      setAssessment(a);
      setState("ready");
    } catch {
      setState("error");
    }
  }

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 640, margin: "0 auto", padding: "34px 40px 56px" }}>
        <div style={{ background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)", borderRadius: 20, padding: "26px 28px", boxShadow: "0 10px 30px rgba(42,59,122,.20)" }}>
          <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "#AEB8E4" }}>本次对话 · 思维印记</div>
          <div style={{ fontSize: 22, fontWeight: 800, color: "#fff", marginTop: 6, lineHeight: 1.3 }}>你在这次对话里留下的思考痕迹</div>
          <div style={{ fontSize: 13, color: "#C3CBEC", marginTop: 6 }}>按 SOLO 四级 · 只诊断过程，不打总分</div>
        </div>

        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
          <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 4 }}>本次对话评估</div>
          <div style={{ fontSize: 12.5, color: "#8A92A3", marginBottom: 14 }}>来自这次对话里你的追问与返工</div>
          {state === "loading" && <div style={{ fontSize: 13.5, color: "#9AA1B0" }}>正在读取…</div>}
          {state === "error" && <div style={{ fontSize: 13.5, color: "#8A92A3" }}>评估暂时没能生成，稍后再试。</div>}
          {state === "empty" && (
            <div style={{ padding: "10px 0" }}>
              <div style={{ fontSize: 13.5, color: "#6B7384", marginBottom: 14, lineHeight: 1.6 }}>
                为这次对话生成一份思维印记——看看你的问题有没有变得更锋利。
              </div>
              <button type="button" onClick={() => void generate()} style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "11px 18px", borderRadius: 12, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
                生成本次对话的思维印记
              </button>
            </div>
          )}
          {state === "generating" && <div style={{ fontSize: 13.5, color: "#9AA1B0" }}>正在生成本次对话的思维印记…</div>}
          {state === "ready" && assessment && (
            <>
              {assessment.dimensions.map((d) => <DimensionRow key={d.code} dim={d} />)}
              {assessment.narrative && (
                <div style={{ fontSize: 13.5, color: "#2B3346", lineHeight: 1.7, marginTop: 16, background: "#F7F8FB", borderRadius: 12, padding: "14px 16px" }}>
                  {assessment.narrative}
                </div>
              )}
            </>
          )}
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 22 }}>
          <button type="button" onClick={onClose} style={{ flex: "none", background: "#fff", border: "1px solid #E1E4ED", color: "#6B7384", fontSize: 14, fontWeight: 700, padding: "13px 20px", borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>返回对话</button>
          {state === "ready" && (
            <button type="button" onClick={() => void generate()} style={{ flex: "none", background: "none", border: "none", color: "#8A92A3", fontSize: 13, fontWeight: 600, cursor: "pointer", fontFamily: "inherit" }}>重新生成</button>
          )}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Add the opt-in button + props to ChatSurface**

In `apps/web/src/shell/chat/ChatSurface.tsx`, add to `ChatSurfaceProps`:

```ts
  canOpenReport: boolean;
  onOpenReport: () => void;
```

Destructure them in the component signature. In the thread header (the `{/* thread column */}` header row, next to the `计入成长评估` pill, ~line 245), add before the pill:

```tsx
          <button
            type="button"
            onClick={onOpenReport}
            disabled={!canOpenReport}
            title={canOpenReport ? "为这次对话生成思维印记" : "先和 AI 聊几句，再生成"}
            style={{ flex: "none", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12, fontWeight: 700, color: canOpenReport ? "#2A3B7A" : "#C7CCDA", background: canOpenReport ? "#EDEFF9" : "#F3F4F7", border: "none", padding: "6px 12px", borderRadius: 999, cursor: canOpenReport ? "pointer" : "not-allowed", fontFamily: "inherit" }}
          >
            生成本次对话的思维印记
          </button>
```

- [ ] **Step 5: Wire ChatContainer**

In `apps/web/src/shell/chat/ChatContainer.tsx`:
- Import: `import { ChatReport } from "./ChatReport";`
- Add state: `const [reportOpen, setReportOpen] = useState(false);`
- Close the report whenever the active thread changes — add `setReportOpen(false);` inside the `useEffect` that reacts to `activeThreadId` (the block at line 37) and in `handleSelectThread`/`handleNewConversation`.
- Compute the gate: `const canOpenReport = !!activeThreadId && entries.some((e) => e.role === "assistant" && e.text.trim().length > 0);`
- Before the `return <ChatSurface … />`, short-circuit when the report is open:

```tsx
  if (reportOpen && activeThreadId) {
    return <ChatReport threadId={activeThreadId} onClose={() => setReportOpen(false)} />;
  }
```

- Pass the two new props to `ChatSurface`:

```tsx
      canOpenReport={canOpenReport}
      onOpenReport={() => setReportOpen(true)}
```

- [ ] **Step 6: Write the failing tests**

Create `apps/web/src/shell/chat/ChatReport.test.tsx`:

```tsx
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ChatReport } from "./ChatReport";
import { api } from "../../api";

vi.mock("../../api", () => ({ api: { getChatAssessment: vi.fn(), generateChatAssessment: vi.fn() } }));

const sample = {
  dimensions: [{ code: "D1", name: "问题意识", level: "L3", evidence: "你追问了三次" }],
  narrative: "你的问题越来越锋利。",
  generatedAt: "2026-07-18T00:00:00Z",
};

beforeEach(() => vi.clearAllMocks());

describe("ChatReport", () => {
  it("shows the opt-in CTA and never auto-generates when no report exists", async () => {
    (api.getChatAssessment as any).mockResolvedValue(null);
    render(<ChatReport threadId="t1" onClose={() => {}} />);
    await screen.findByText("生成本次对话的思维印记");
    expect(api.generateChatAssessment).not.toHaveBeenCalled(); // 铁律 2: opt-in only
  });

  it("generates on click and renders the dimensions + narrative", async () => {
    (api.getChatAssessment as any).mockResolvedValue(null);
    (api.generateChatAssessment as any).mockResolvedValue(sample);
    render(<ChatReport threadId="t1" onClose={() => {}} />);
    fireEvent.click(await screen.findByText("生成本次对话的思维印记"));
    await waitFor(() => expect(screen.getByText("问题意识")).toBeInTheDocument());
    expect(screen.getByText("你的问题越来越锋利。")).toBeInTheDocument();
  });

  it("rehydrates a stored report on mount without generating", async () => {
    (api.getChatAssessment as any).mockResolvedValue(sample);
    render(<ChatReport threadId="t1" onClose={() => {}} />);
    await waitFor(() => expect(screen.getByText("问题意识")).toBeInTheDocument());
    expect(api.generateChatAssessment).not.toHaveBeenCalled();
  });
});
```

In `apps/web/src/shell/chat/ChatSurface.test.tsx`, add a case that the report button is disabled with no assistant turn and enabled with one (pass `canOpenReport` false/true and assert `onOpenReport` fires only when enabled). Match the existing test file's render helper and prop-passing style.

- [ ] **Step 7: Run the web tests**

Run: `cd apps/web && npx vitest run src/shell/chat/`
Expected: PASS (ChatReport + ChatSurface). If the mock shape mismatches `Assessment`, align the sample object's fields with the Zod schema (`code`/`name`/`level`/`evidence`, `narrative`, `generatedAt`).

- [ ] **Step 8: Typecheck the web app**

Run: `cd apps/web && npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 9: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/src/api/chatAssessment.ts apps/web/src/api/index.ts apps/web/src/shell/chat/ChatReport.tsx apps/web/src/shell/chat/ChatReport.test.tsx apps/web/src/shell/chat/ChatSurface.tsx apps/web/src/shell/chat/ChatSurface.test.tsx apps/web/src/shell/chat/ChatContainer.tsx
git commit -m "feat(a2): chat report panel + opt-in button on the chat surface

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Final verification (before finishing the branch)

- [ ] `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...` — all packages PASS.
- [ ] `cd apps/api && make sqlc` then `git status --short apps/api/internal/store/sqlc/` — ZERO drift (regenerated code already committed).
- [ ] `cd packages/contracts && npm test` (or the repo's contracts test command) — PASS (no contract change expected; confirm nothing broke).
- [ ] `cd apps/web && npx vitest run && npx tsc --noEmit` — PASS + exit 0.
- [ ] Grep confirms the exemption is gone: `grep -rn "surface = 'chat'" apps/api/internal/store/migrations/` shows it only in `0024`'s comment/`0025`'s Down (restoration), never in `0025`'s Up CHECK.
- [ ] Grep confirms no scopeless chat writes remain: `grep -rn "InsertUserEvent" apps/api/internal/` returns nothing (the seam is renamed).

## Self-review notes (author)

- **Spec coverage:** DEC-A2.1/A2.2 → Task 1. DEC-A2.3 → Task 2. DEC-A2.4 → Task 3 (builder) + Task 4 (assessor wiring). DEC-A2.8 → Task 4. DEC-A2.5/A2.6 → Task 5. RL-5 / flagship / cost-on-reject / 铁律 2 asserted in Task 4 + Task 5 tests. DEC-A2.7 (skips stay status-only) → no code, satisfied by omission.
- **Type consistency:** `RecordChatLLMCall(ctx, userID, purpose, resolved, prompt, completion)` used identically in Task 2 (definition + coach call site) and Task 4 (assessment call site). `buildAssessmentInputFromEvidence(events, cards)` defined in Task 3, consumed in Task 4. `InsertThreadEvaluationParams` fields mirror `InsertSessionEvaluationParams`.
- **Known verify-not-trust points flagged inline:** grep all `AppendEventParams{}` sites (Task 2 Step 3); confirm `agent.AssessmentInput` field names (Task 3 Step 3); confirm `InsertThreadEvaluationParams` field names post-codegen (Task 4 Step 3).

# N6 Sweep · Correctness & Hygiene — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the small, low-risk correctness + hygiene subset of the N6 bucket — five surgical fixes that keep metering and the process record honest, plus five test-infra cleanups — with no schema change and no new LLM subagent.

**Architecture:** Correctness items are bounded, independent edits in `apps/api` (a gate-item removal that relies on an existing coach nudge; two transaction-wraps modelled on the existing `CommitCardMint` pattern; a per-project classifier-spend cap via a new count query; two metering-record moves + a semantics test). Hygiene items are dead-code deletion, a one-line `tsc` fix, test de-flaking, one migration-`Down` test, and a mechanical relocation of `apps/web` tests into a `test/` tree.

**Tech Stack:** Go (`net/http`, `pgx`/`sqlc`, `goose`), React + Vite + TypeScript + Vitest, Zod contracts.

## Global Constraints

- **The client NEVER calls a model directly.** All LLM calls go through the backend gateway; keys only in `apps/api` server env; never in git/logs/errors/data/eval payloads.
- **Skill JSON is authored ONLY in `packages/contracts/skills/`**, then mirrored via `cd apps/api && make sync-skills`. **Never hand-edit `apps/api/internal/skills/specs/`.**
- **Never hand-edit `apps/api/internal/store/sqlc/*`.** Regenerate with `cd apps/api && make sqlc` (CGO_ENABLED=0 on macOS).
- **Go tests run FULL packages, never `-run` subsets** for anything touching gate/card/skill/planner/store/course. Command: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
- **Web tests:** `cd apps/web && npm test` and `npx tsc --noEmit`. **Contracts:** `cd packages/contracts && npm test` (tests in `test/`) and `npx tsc --noEmit`.
- **Every LLM call is metered (档位+token+成本), including rejected/empty outputs.** Assessment runs flagship, never downgraded.
- **DEC-3:** machine judgment never marks a gate `solid` by recording a `student_written`/`human` item; solid requires a passed challenge or explicit confirmation.
- **铁律 2 (不操纵):** no gate blocks where a warning suffices — "an offer is never a wall." No streaks/badges/scores.
- **Icons are inline SVG. Never import `lucide-react`.**
- **NEVER `git add` a whole directory.** The pre-existing `M package.json` and untracked `docs/*` files are NOT part of this work — stage only the exact files each task changes.
- The `/dev/null` redirect is blocked by a hook; do not use it in commands.

---

### Task 1 (C1): `no_unsupported_claim` — warn, don't block

**Context:** `build_argument`'s gate lists `no_unsupported_claim` as a blocking machine item. It is a negation scan over `claim` nodes: with zero claims it is vacuously true (an argument-free station clears), and with a present-but-unsupported claim it returns *false* and **blocks** the station. The maintainer's decision (铁律 2): an argument hole should be *surfaced*, not *prohibited*. The warn channels already exist and are unchanged by this task — `CandidateMoves` (`agent/classifier.go:9`) emits a `post_intervention` D5 candidate ("claim has no supporting evidence") for each unsupported claim, and the `anyEvaluated && !hasClaim && !toulminSeen` branch (`classifier.go:306`) already offers the toulmin card when no claim exists. So this task only removes the *blocking* item; the coach still warns.

The system-prompt golden (`agent/testdata/system_prompt_full.txt`) does **not** render gate machine kinds (verified: 0 matches for `no_unsupported`), so it will not change. `no_unsupported_claim` stays a valid kind in `gate.go`'s `evalMachineItem` and in `skills/skill.go`'s allowlist (other skills/future use may need it) — do not delete the evaluator case.

**Files:**
- Modify: `packages/contracts/skills/writing-project.json:83` (remove the `no_unsupported_claim` machine item from `build_argument.gate.machine`)
- Regenerate (do not hand-edit): `apps/api/internal/skills/specs/writing-project.json` via `make sync-skills`
- Test: `apps/api/internal/agent/classifier_test.go` (or the existing CandidateMoves test file — locate it with `grep -rln "CandidateMoves" apps/api/internal/agent`)

**Interfaces:**
- Consumes: existing `CandidateMoves(g GraphView) []Candidate`, `CheckGate(sk, contractID, g, rec) GateReport`.
- Produces: nothing new; a narrower `build_argument` machine gate.

- [ ] **Step 1: Write a failing test pinning that the warn survives and the block is gone**

Add to the CandidateMoves test file. First confirm the D5 warn fires for an unsupported claim (this should already pass — it pins the warn we now rely on), then assert `build_argument`'s gate no longer contains `no_unsupported_claim`:

```go
func TestBuildArgumentGateDropsNoUnsupportedClaim(t *testing.T) {
	sk := loadWritingProjectSkill(t) // use whatever helper the package already uses to load the skill; grep for existing skill-load helpers in *_test.go
	c, ok := sk.Contract("build_argument")
	if !ok {
		t.Fatal("build_argument contract missing")
	}
	for _, m := range c.Gate.Machine {
		if m.Kind == "no_unsupported_claim" {
			t.Fatal("no_unsupported_claim must no longer be a blocking machine gate item on build_argument (warn-not-block)")
		}
	}
}

func TestCandidateMovesStillWarnsUnsupportedClaim(t *testing.T) {
	// One claim node, no supporting evidence -> D5 post_intervention warn.
	g := GraphView{Nodes: []NodeView{{ID: "c1", Type: "claim"}}}
	got := CandidateMoves(g)
	found := false
	for _, cand := range got {
		if cand.Criterion == "D5" && cand.AnchorID == "c1" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a D5 warn candidate for the unsupported claim")
	}
}
```

Adjust `GraphView`/`NodeView` field names and the skill-load helper to the package's actual shapes (grep an existing `classifier_test.go`/`gate_test.go` for the exact constructors before writing).

- [ ] **Step 2: Run the tests — the gate test fails, the warn test passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/ -run 'TestBuildArgumentGateDropsNoUnsupportedClaim|TestCandidateMovesStillWarnsUnsupportedClaim'`
Expected: `TestBuildArgumentGateDropsNoUnsupportedClaim` FAILs (item still present); the warn test PASSes.

- [ ] **Step 3: Remove the machine item from the canonical skill JSON**

In `packages/contracts/skills/writing-project.json`, delete the line:
```json
          { "kind": "no_unsupported_claim" },
```
from `build_argument.gate.machine`, leaving `{ "kind": "node_present", "type": "concession" }` as the sole machine item.

- [ ] **Step 4: Mirror the skill to the Go embed**

Run: `cd apps/api && make sync-skills`
Then verify canonical and mirror are byte-identical:
`diff packages/contracts/skills/writing-project.json apps/api/internal/skills/specs/writing-project.json` (from repo root) — expect no output.

- [ ] **Step 5: Run the FULL agent + skills + contracts suites**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/... ./internal/skills/...`
Run: `cd packages/contracts && npm test`
Expected: all PASS. If any enumeration/gate-count test asserted the old item count for `build_argument`, update it to the new expectation (a machine-gate change legitimately shifts those). Do NOT use `-run` subsets here — a gate change can touch enumeration tests in these packages.

- [ ] **Step 6: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add packages/contracts/skills/writing-project.json apps/api/internal/skills/specs/writing-project.json apps/api/internal/agent/classifier_test.go
git commit -m "fix(n6): no_unsupported_claim warns via existing D5 nudge, no longer blocks build_argument

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
(Stage the exact test file you edited; adjust the path if the CandidateMoves test lives elsewhere.)

---

### Task 2 (C2): card **skip** — envelope + status in one transaction

**Context:** `skipProjectCard` (`api/projectcards.go:84`) performs three sequential, separately-committed writes: `SubmitProjectCardInstance` (envelope) → `SetCardInstanceStatus("skipped")` → `AppendEvent(card_skipped)`. A crash between the first two leaves a card with a saved answer but status still `active` (or vice versa) — the process record (过程即数据) diverges. The `card_skipped` event append is already best-effort (logged, non-fatal) and stays that way. `activateProjectCard` performs a **single** state write (status) plus a best-effort event — it is already atomic and needs **no** change.

The existing transaction pattern to mirror is `CommitCardMint` (`agent/agentstore.go:452`): `tx, err := s.pool.Begin(ctx)` → `defer tx.Rollback(ctx)` → `qtx := s.q.WithTx(tx)` → do writes on `qtx` → `tx.Commit(ctx)`. The skip handler uses the **concrete** `*sqlcAgentStore` (`agent.NewSqlcAgentStore(...)`), so this adds a concrete method — **no `AgentStore` interface change**.

**Files:**
- Modify: `apps/api/internal/agent/agentstore.go` (add `SubmitAndSkipCardInstance`)
- Modify: `apps/api/internal/api/projectcards.go:84-116` (call the new method)
- Test: `apps/api/internal/agent/agentstore_test.go` (or the package's existing store test file — grep `func Test` in `agent/*_test.go` that use a real pool/testcontainer)

**Interfaces:**
- Produces: `func (s *sqlcAgentStore) SubmitAndSkipCardInstance(ctx context.Context, projectID, id uuid.UUID, fieldValues, eventTrace []byte) error` — persists the envelope and sets status `"skipped"` atomically.

- [ ] **Step 1: Write the failing test**

Use the package's existing testcontainer/pool harness (grep for how other `agentstore` tests obtain a `*pgxpool.Pool` and seed a project + card_instance). The test creates a proposed card_instance, calls `SubmitAndSkipCardInstance`, and asserts BOTH the envelope and status landed:

```go
func TestSubmitAndSkipCardInstanceIsAtomic(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)                    // existing helper
	store := NewSqlcAgentStore(New(pool), pool) // match the package's constructor for *sqlc.Queries
	projectID, cid := seedProposedCard(t, pool) // existing/seed helper: a project + one proposed card_instance

	err := store.SubmitAndSkipCardInstance(ctx, projectID, cid, []byte(`{}`), []byte(`[]`))
	if err != nil {
		t.Fatalf("skip: %v", err)
	}
	row, err := store.GetCardInstance(ctx, cid)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "skipped" {
		t.Fatalf("status = %q, want skipped", row.Status)
	}
	if string(row.FieldValues) != "{}" {
		t.Fatalf("field_values = %q, want {}", row.FieldValues)
	}
}
```

Match `newTestPool`/`seedProposedCard`/`New` to the real helpers in the package (grep first; do not invent names).

- [ ] **Step 2: Run it — fails to compile (method undefined)**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/ -run TestSubmitAndSkipCardInstanceIsAtomic`
Expected: build error `store.SubmitAndSkipCardInstance undefined`.

- [ ] **Step 3: Implement the transactional method**

Add to `agentstore.go`, modelled on `CommitCardMint`:

```go
// SubmitAndSkipCardInstance persists the student's (empty, on skip)
// field_values + event_trace AND flips the status to "skipped" in ONE
// transaction, so a crash can never leave a saved envelope with a stale
// "active" status (or the reverse). 过程即数据: the two records of one act
// cannot drift. Mirrors CommitCardMint's tx shape.
func (s *sqlcAgentStore) SubmitAndSkipCardInstance(ctx context.Context, projectID, id uuid.UUID, fieldValues, eventTrace []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	if _, err := qtx.SubmitProjectCardInstance(ctx, sqlc.SubmitProjectCardInstanceParams{
		ID: id, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		FieldValues: fieldValues, EventTrace: eventTrace,
	}); err != nil {
		return err
	}
	if _, err := qtx.SetCardInstanceStatus(ctx, sqlc.SetCardInstanceStatusParams{
		ID: id, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Status: "skipped",
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
```

- [ ] **Step 4: Wire the handler to the new method**

In `api/projectcards.go`, replace the two separate calls (`SubmitProjectCardInstance` then `SetCardInstanceStatus`) in `skipProjectCard` with the single call; leave the best-effort `AppendEvent(card_skipped)` afterward:

```go
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.SubmitAndSkipCardInstance(r.Context(), projectID, cid, []byte("{}"), body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "card_skipped", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("card skip: append card_skipped event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	w.WriteHeader(http.StatusNoContent)
```

- [ ] **Step 5: Run the store test + the FULL api and agent packages**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/... ./internal/api/...`
Expected: all PASS, including the existing skip-handler tests.

- [ ] **Step 6: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/agent/agentstore.go apps/api/internal/api/projectcards.go apps/api/internal/agent/agentstore_test.go
git commit -m "fix(n6): card skip persists envelope+status atomically

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3 (C3): `Advance` — gate-state + passed-event in one transaction

**Context:** `Advance` (`agent/planner.go:120`) confirms a gate by `UpsertGateState` (commit A), then separately `AppendEvent(gate_attempt result:passed)` (commit B). If B fails, the gate is `solid` but the process tree has no record it passed. Wrap A+B atomically. `Advance` reaches persistence only through the `AgentStore` **interface** (`agent/loop.go:92`), so this adds ONE interface method plus its fake implementation. The blocked path (single `AppendEvent`, no upsert) is unchanged.

`AppendEvent`'s sqlc impl resolves the owning user via `GetProject`; the new combined method must do the same inside the tx.

**Files:**
- Modify: `apps/api/internal/agent/loop.go:92` (add `ConfirmGate` to `AgentStore`)
- Modify: `apps/api/internal/agent/agentstore.go` (implement `ConfirmGate` transactionally)
- Modify: `apps/api/internal/agent/planner.go:145-152` (call `ConfirmGate` instead of `UpsertGateState`+`AppendEvent`)
- Modify: `apps/api/internal/agent/loop_test.go:21` (`fakeAgentStore` — add the method)
- Test: `apps/api/internal/agent/planner_test.go`

**Interfaces:**
- Produces: `ConfirmGate(ctx context.Context, projectID uuid.UUID, contract string, rec RecordedGate, passedEvent EventRow) error` on `AgentStore` — upserts the confirmed gate state and appends the passed event atomically.

- [ ] **Step 1: Add the method to the fake and write the failing test**

In `loop_test.go`, add to `fakeAgentStore` (non-transactional is fine for the fake — it just records both):
```go
func (f *fakeAgentStore) ConfirmGate(ctx context.Context, projectID uuid.UUID, contract string, rec RecordedGate, passedEvent EventRow) error {
	if err := f.UpsertGateState(ctx, projectID, contract, rec); err != nil {
		return err
	}
	return f.AppendEvent(ctx, passedEvent)
}
```
In `planner_test.go`, assert `Advance` on a passing gate both confirms state and records the passed event via the single call. If the package's existing `Advance` tests already assert the upsert + event, extend the fake to count `ConfirmGate` calls and assert it is used:
```go
func TestAdvanceUsesConfirmGate(t *testing.T) {
	// Build a fake whose gate has nothing Missing (grep an existing passing-gate
	// Advance test for the exact fixture), call Advance, and assert:
	//   - the gate state is Confirmed, AND
	//   - exactly one gate_attempt result:passed event was appended,
	//   - both via ConfirmGate (add a counter to the fake).
}
```
Write the concrete assertions against the fake's recorded state, matching the existing `planner_test.go` fixtures.

- [ ] **Step 2: Run it — fails (interface method missing / Advance still calls the split path)**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/ -run TestAdvanceUsesConfirmGate`
Expected: compile error (method not in interface) or assertion failure.

- [ ] **Step 3: Declare the interface method**

In `loop.go`, add to `type AgentStore interface` near `UpsertGateState`:
```go
	// ConfirmGate confirms a gate's state AND records its gate_attempt
	// result:passed event in ONE transaction, so a gate can never read solid
	// while the process tree lacks the record that it passed (N6 C3).
	ConfirmGate(ctx context.Context, projectID uuid.UUID, contract string, rec RecordedGate, passedEvent EventRow) error
```

- [ ] **Step 4: Implement it transactionally in the sqlc store**

In `agentstore.go`, modelled on `CommitCardMint` + `AppendEvent`'s user resolution:
```go
func (s *sqlcAgentStore) ConfirmGate(ctx context.Context, projectID uuid.UUID, contract string, rec RecordedGate, passedEvent EventRow) error {
	project, err := s.q.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	if err := upsertGateStateQ(ctx, qtx, projectID, contract, rec); err != nil {
		return err
	}
	if _, err := qtx.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		UserID:    project.UserID,
		SessionID: pgtype.UUID{Valid: false},
		Surface:   passedEvent.Surface,
		Type:      passedEvent.Type,
		Payload:   passedEvent.Payload,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
```
If `UpsertGateState`'s body is not already extractable as a `qtx`-taking helper, refactor its SQL-building core into `func upsertGateStateQ(ctx, q sqlcQuerier, projectID uuid.UUID, contract string, rec RecordedGate) error` and have the existing `UpsertGateState` call it with `s.q`. (`CommitCardMint` already uses this `...Q(ctx, qtx, ...)` helper convention — follow it. Use the same querier-interface type those helpers take.)

- [ ] **Step 5: Switch `Advance` to the single call**

In `planner.go`, replace lines 145-152's `rec.Confirmed = true; UpsertGateState(...); AppendEvent(...)` with:
```go
	rec.Confirmed = true
	payload, _ := json.Marshal(map[string]any{"contract": contractID, "result": "passed"})
	if err := deps.Store.ConfirmGate(ctx, projectID, contractID, rec, EventRow{ProjectID: projectID, Surface: "studio", Type: "gate_attempt", Payload: payload}); err != nil {
		return false, err
	}
	return true, nil
```

- [ ] **Step 6: Run the FULL agent package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/...`
Expected: all PASS (every `fakeAgentStore` user still compiles; `Advance`/`AdvanceAll` tests pass).

- [ ] **Step 7: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/agent/loop.go apps/api/internal/agent/agentstore.go apps/api/internal/agent/planner.go apps/api/internal/agent/loop_test.go apps/api/internal/agent/planner_test.go
git commit -m "fix(n6): Advance confirms gate + passed-event atomically

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4 (C4): bound classifier spend per project

**Context:** `semanticCardCandidate` (`agent/loop.go:479`) runs the moment-classifier every student turn ≥ `MinClassifyRunes` (12) as long as any moment is eligible. If a moment never trips, its card is never used, the moment stays eligible forever, and the classifier fires every turn indefinitely (answering "none," retiring nothing) — roughly doubling per-turn calls. Every classifier call is already recorded as an `llm_call` with `Purpose = "classify"` (`loop.go:502`). Add a count query and short-circuit once a project reaches a generous cap. This is a spend backstop — normal projects trip their moments well within the cap.

**Files:**
- Modify: `apps/api/internal/store/queries/llm_usage.sql` (add count query)
- Regenerate: `apps/api/internal/store/sqlc/*` via `make sqlc` (never hand-edit)
- Modify: `apps/api/internal/agent/loop.go` (add interface method + cap constant + short-circuit)
- Modify: `apps/api/internal/agent/agentstore.go` (implement the method)
- Modify: `apps/api/internal/agent/loop_test.go` (`fakeAgentStore` — add the method + a settable count)
- Test: `apps/api/internal/agent/loop_test.go`

**Interfaces:**
- Consumes: existing `RecordLLMCall` writing `Purpose: "classify"` rows.
- Produces: `CountClassifierCalls(ctx context.Context, projectID uuid.UUID) (int64, error)` on `AgentStore`; `const MaxClassifyCallsPerProject = 20`.

- [ ] **Step 1: Add the sqlc query**

Append to `apps/api/internal/store/queries/llm_usage.sql`:
```sql
-- name: CountLLMCallsByProjectPurpose :one
SELECT count(*) FROM llm_call
WHERE project_id = $1 AND purpose = $2;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc`
Expected: `internal/store/sqlc/llm_usage.sql.go` gains `CountLLMCallsByProjectPurpose`. Do not edit generated files by hand.

- [ ] **Step 3: Write the failing test**

In `loop_test.go`, give `fakeAgentStore` a settable classifier count, then assert the cap suppresses the call. Use the package's existing `RunAgentStep` / `semanticCardCandidate` harness (grep how `TestClassifyMomentNoCallWhenNothingEligible` drives it):
```go
func TestClassifierCappedPerProject(t *testing.T) {
	// A graph with an eligible moment + a student turn >= MinClassifyRunes.
	// With the fake reporting classifyCount >= MaxClassifyCallsPerProject,
	// semanticCardCandidate must return no candidate WITHOUT calling the
	// provider (assert the fake provider's call count did not increase).
}
```
Model the fixture on the existing eligible-moment test; add `classifyCount int64` to the fake and return it from `CountClassifierCalls`.

- [ ] **Step 4: Run it — fails (method missing / no cap applied)**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/ -run TestClassifierCappedPerProject`
Expected: compile error or the provider still called.

- [ ] **Step 5: Add the interface method + implementation + fake**

In `loop.go` `AgentStore`:
```go
	// CountClassifierCalls bounds moment-classifier spend: the 5th subagent
	// runs every turn while a moment stays eligible, so a project where a
	// moment never trips would classify forever. Capped at
	// MaxClassifyCallsPerProject (N6 C4).
	CountClassifierCalls(ctx context.Context, projectID uuid.UUID) (int64, error)
```
Add the constant near `MinClassifyRunes` (`moment.go:120`) or in `loop.go`:
```go
const MaxClassifyCallsPerProject = 20
```
In `agentstore.go`:
```go
func (s *sqlcAgentStore) CountClassifierCalls(ctx context.Context, projectID uuid.UUID) (int64, error) {
	return s.q.CountLLMCallsByProjectPurpose(ctx, sqlc.CountLLMCallsByProjectPurposeParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Purpose:   "classify",
	})
}
```
In `loop_test.go` fake:
```go
func (f *fakeAgentStore) CountClassifierCalls(ctx context.Context, projectID uuid.UUID) (int64, error) {
	return f.classifyCount, nil
}
```

- [ ] **Step 6: Add the short-circuit**

In `semanticCardCandidate` (`loop.go`), after the `eligible` check (line ~496) and before `ClassifyMoment` (line ~499):
```go
	if n, err := deps.Store.CountClassifierCalls(ctx, projectID); err != nil {
		slog.Warn("agent: classifier cap count failed; proceeding", "project_id", projectID.String(), "err", err.Error())
	} else if n >= MaxClassifyCallsPerProject {
		return Candidate{}, false
	}
```
(On a count error, proceed — a transient metering read must not permanently silence coaching.)

- [ ] **Step 7: Run the FULL agent package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/...`
Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/queries/llm_usage.sql apps/api/internal/store/sqlc/ apps/api/internal/agent/loop.go apps/api/internal/agent/agentstore.go apps/api/internal/agent/moment.go apps/api/internal/agent/loop_test.go
git commit -m "fix(n6): cap moment-classifier spend per project

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
(Stage the specific regenerated `internal/store/sqlc/*.go` files that changed — check `git status` — not unrelated ones.)

---

### Task 5 (C5): metering accuracy — empty-result leak + CostNumeric semantics test

**Context:** Two small, unrelated-in-code but same-family touches. (a) `surfaceAnchors` (`api/studioturn.go:351-370`) records the `llm_call` *after* an early `return` on `len(result.Anchors) == 0`, so a real anchor call that yields zero usable anchors is unmetered; `renderChallenge` (`agent/course.go:125-133`) has the same shape (it carries usage onto the fallback only *after* the empty-result bail at line 126). (b) The CostNumeric "bug" is not a value-bug: the `llm_call` recorders write a NOT-NULL column so `CostNumeric(cost, true)` (explicit $0.00) is the only legal choice, while the nullable `evaluation` recorders correctly use `CostNumeric(cost, priced)`. Pin that split with a unit test and add the missing explanatory comments.

**Files:**
- Modify: `apps/api/internal/api/studioturn.go:351-370` (meter before the empty bail)
- Modify: `apps/api/internal/agent/course.go:125-133` (carry usage before the empty bail)
- Modify: `apps/api/internal/agent/chatstore.go:127`, `apps/api/internal/agent/coursestore.go:223` (add the one-line comment `agentstore.go` already carries)
- Test: `apps/api/internal/gateway/pricing_test.go`

**Interfaces:** none new.

- [ ] **Step 1: Write the failing CostNumeric semantics test**

Add to `pricing_test.go`:
```go
func TestCostNumericSemanticsSplit(t *testing.T) {
	// llm_call path (NOT NULL column): an unpriced model records explicit $0.00.
	z := CostNumeric(0, true)
	if !z.Valid {
		t.Fatal("llm_call path: explicit $0.00 must be a valid zero Numeric, not NULL")
	}
	// evaluation path (nullable column): an unpriced model records NULL.
	if CostNumeric(0, false).Valid {
		t.Fatal("evaluation path: unpriced cost must be NULL (invalid) Numeric")
	}
}
```

- [ ] **Step 2: Run it**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/gateway/ -run TestCostNumericSemanticsSplit`
Expected: PASS immediately (this pins existing correct behavior — it is a regression guard, not a fix). If it fails, stop: the semantics differ from the spec's claim and must be investigated before proceeding.

- [ ] **Step 3: Fix the `surfaceAnchors` empty-result leak**

In `studioturn.go`, restructure so metering happens right after `Generate`, before the empty bail. Replace lines 351-370 so the record block moves ABOVE the `len==0` return:
```go
	result, err := gen.Generate(ctx, spec, materials, level)
	// A real call succeeded whenever Resolved is populated — it cost money
	// regardless of whether it yielded usable anchors, so record it BEFORE any
	// empty-result bail (N6 C5). A metering failure never fails the turn.
	if result.Resolved.Provider != "" {
		if rerr := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "anchors",
			Resolved: result.Resolved, PromptTokens: int32(result.Usage.InputTokens), CompletionTokens: int32(result.Usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("surface anchors: record llm usage failed", "err", rerr, "request_id", httpx.RequestIDFromContext(ctx))
		}
	}
	if err != nil || len(result.Anchors) == 0 {
		return nil, false
	}
	if spec.Params.LateralDimension != "" {
		result.Anchors = dropDimension(result.Anchors, spec.Params.LateralDimension)
	}
```
Delete the now-duplicated record block that previously sat at lines 363-370.

- [ ] **Step 4: Fix the `renderChallenge` empty-result leak**

In `course.go`, carry usage onto the fallback BEFORE the empty bail. Replace lines 125-133:
```go
	gen, err := NewAnchorGenerator(provider, resolver).Generate(ctx, spec, materials, GuidanceL1)
	// Carry usage onto the fallback BEFORE any empty-result bail — a populated
	// Resolved means a paid call happened regardless of anchored-ness (N6 C5).
	if gen.Resolved.Provider != "" {
		authored.Resolved = gen.Resolved
		authored.Usage = gen.Usage
	}
	if err != nil || len(gen.Anchors) == 0 {
		return authored
	}
```
Remove the now-redundant `authored.Resolved = gen.Resolved` / `authored.Usage = gen.Usage` at the old lines 132-133.

- [ ] **Step 5: Add the clarifying comments**

Above the `RecordLLMCall` in `chatstore.go` (line ~123) and `coursestore.go` (line ~219), add:
```go
	// llm_call.cost_estimate is NOT NULL, so an unpriced model records an
	// explicit $0.00 via CostNumeric(cost, true) (with the warn above) — NOT
	// the nullable-evaluation NULL. See gateway/pricing.go and agentstore.go.
```

- [ ] **Step 6: Run the FULL gateway, api, and agent packages**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/gateway/... ./internal/api/... ./internal/agent/...`
Expected: all PASS. If an existing metering test asserted the empty-anchor path did NOT record, update it — the new behavior (record) is intended.

- [ ] **Step 7: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/studioturn.go apps/api/internal/agent/course.go apps/api/internal/agent/chatstore.go apps/api/internal/agent/coursestore.go apps/api/internal/gateway/pricing_test.go
git commit -m "fix(n6): meter empty-result anchor/challenge calls; pin CostNumeric semantics

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 6 (H2 + H3): delete dead script + fix contracts `tsc`

**Context:** Two trivial, independent hygiene wins bundled because each is a one-liner. (H2) `apps/web/scripts/gen-go-fixtures.ts` is referenced by no `package.json` script and nothing else (verified). (H3) `packages/contracts` fails `npx tsc --noEmit` with one error: `test/interactionPrimitive.test.ts(49,12): error TS2532: Object is possibly 'undefined'`.

**Files:**
- Delete: `apps/web/scripts/gen-go-fixtures.ts`
- Modify: `packages/contracts/test/interactionPrimitive.test.ts:49`

- [ ] **Step 1: Confirm the script is dead, then delete it**

Run: `cd /Users/houyuxin/08Coding/mind-imprint && grep -rn "gen-go-fixtures" apps/web packages --include="*.json" --include="*.ts"` — expect no references except the file itself.
Then: `git rm apps/web/scripts/gen-go-fixtures.ts`

- [ ] **Step 2: Reproduce the tsc failure**

Run: `cd packages/contracts && npx tsc --noEmit`
Expected: one error at `test/interactionPrimitive.test.ts(49,12)`.

- [ ] **Step 3: Fix the unsafe access**

Read `test/interactionPrimitive.test.ts` around line 49. The value is possibly `undefined` (e.g. an array index or `.find()` result). Add a guard consistent with the surrounding test style — either an assertion the test already implies:
```ts
const row = rows[0];
expect(row).toBeDefined();
// then use row! or restructure so TS sees it as defined
```
or a non-null assertion (`rows[0]!`) if that matches the file's existing convention. Choose whichever the neighboring assertions use; do not introduce a new style.

- [ ] **Step 4: Verify tsc is clean and tests still pass**

Run: `cd packages/contracts && npx tsc --noEmit` → expect no output (clean).
Run: `cd packages/contracts && npm test` → expect all PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/scripts/gen-go-fixtures.ts packages/contracts/test/interactionPrimitive.test.ts
git commit -m "chore(n6): delete dead gen-go-fixtures; fix contracts tsc error

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
(`git rm` already stages the deletion; the `git add` re-affirms it and stages the test fix.)

---

### Task 7 (H4): de-flake `ChatSurface` and cross-pane tests

**Context:** `apps/web/src/shell/chat/ChatSurface.test.tsx` has a load-sensitive 5s-timeout flake, and the N3c cross-pane tests were ~50% flaky under load (they awaited a project-projection label, then *synchronously* queried a conversation-snapshot button that had not rendered yet). A green suite you cannot trust is the documented root cause of shipped Criticals. Make the named tests deterministic by awaiting the actual condition each assertion depends on (`findBy*`/`waitFor`) instead of fixed timeouts or synchronous `getBy*` after an async change.

**Files:**
- Modify: `apps/web/src/shell/chat/ChatSurface.test.tsx`
- Modify: the N3c cross-pane test file(s) — locate with `grep -rln "过程" apps/web/src --include="*.test.tsx"` and by finding the tests that await a projection label then query a snapshot button (the N3c work added three; check `git log --oneline -- apps/web/src | head` around the N3c commit `7b20d14` for the files).

- [ ] **Step 1: Reproduce the flake**

Run the suspect files in a loop to surface intermittency:
`cd apps/web && for i in 1 2 3 4 5; do npx vitest run src/shell/chat/ChatSurface.test.tsx; done`
Note any run that fails or times out.

- [ ] **Step 2: Read and identify the racy assertions**

In each file, find: (a) fixed `{ timeout: 5000 }` or `setTimeout`-based waits, and (b) `getBy*` calls that run immediately after an `await` that changed a *different* element — the classic "await label, then sync-query button" race. These are the two shapes to fix.

- [ ] **Step 3: Replace with condition-based waits**

For each racy assertion, await the specific element/condition it needs:
```ts
// before: await screen.findByText(/项目/); const btn = screen.getByRole("button", { name: /快照/ });
// after:
await screen.findByText(/项目/);
const btn = await screen.findByRole("button", { name: /快照/ });
```
Remove brittle fixed timeouts; let `findBy*`/`waitFor` poll to the default. Do not weaken assertions (no removing checks) — only change *when/how* they wait.

- [ ] **Step 4: Prove stability**

Run each fixed file 5× and confirm all pass:
`cd apps/web && for i in 1 2 3 4 5; do npx vitest run src/shell/chat/ChatSurface.test.tsx <other-file(s)>; done`
Expected: 5/5 clean. Then run the whole suite once: `npm test` → all PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/src/shell/chat/ChatSurface.test.tsx <the cross-pane test file paths>
git commit -m "test(n6): de-flake ChatSurface + cross-pane tests (condition-based waits)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 8 (H5): migration-`Down` test for 0017

**Context:** `migrate_0024`–`migrate_0027_test.go` already round-trip goose `DownTo`/`Up` against a real pool. The tracker flagged `0017_graph_node_open_type` as having a fragile down. Extend the existing pattern to 0017. If its down is genuinely broken (not merely untested), that is a real finding fixed in this task; if it round-trips cleanly, the test pins it.

**Files:**
- Create: `apps/api/internal/store/migrate_0017_test.go`
- Read (pattern): `apps/api/internal/store/migrate_0027_test.go`
- Possibly modify: `apps/api/internal/store/migrations/0017_graph_node_open_type.sql` (only if its down is proven broken)

- [ ] **Step 1: Read the existing pattern and the 0017 migration**

Read `migrate_0027_test.go` (uses `stdlib.OpenDBFromPool` + `goose.DownToContext(ctx, db, "migrations", N)` then `Up`). Read `migrations/0017_graph_node_open_type.sql` — note what its `-- +goose Down` does and whether rows of the affected type could make the down fail (e.g. a `DROP`/`ALTER` that conflicts with existing data).

- [ ] **Step 2: Write the down/up round-trip test**

Mirror `migrate_0027_test.go` exactly, targeting version 16 (down past 0017) then back up:
```go
func TestMigrate0017DownUpRoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := newMigrateTestPool(t) // match the helper the 0024-0027 tests use
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	// migrate down past 0017, then back up — must not error even with
	// graph_node rows of the type 0017 introduced.
	if err := goose.DownToContext(ctx, db, "migrations", 16); err != nil {
		t.Fatalf("down to 16: %v", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("up: %v", err)
	}
}
```
Match helper/import names to the real 0027 test (grep `newMigrateTestPool`/pool-setup in that file first). If the affected table needs a seeded row of the 0017 type to exercise the fragile path, insert it before `DownToContext` (read the migration to know the column/type).

- [ ] **Step 3: Run it**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/store/ -run TestMigrate0017DownUpRoundTrip`
Expected: PASS. If it FAILs on the down, that is the real bug — fix `0017_*.sql`'s `-- +goose Down` (e.g. make a drop idempotent / handle the data that blocks it), re-run to green, and note the fix in the commit body.

- [ ] **Step 4: Run the FULL store package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/...`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/migrate_0017_test.go
# add apps/api/internal/store/migrations/0017_graph_node_open_type.sql ONLY if you fixed a real down bug
git commit -m "test(n6): migration 0017 down/up round-trip

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 9 (H1): relocate `apps/web` tests into a `test/` tree — SEPARABLE

**Context:** `apps/web/src` interleaves **113** `*.test.tsx`/`*.test.ts` files with source, cluttering every source folder; `packages/contracts` already keeps all tests under `test/`. This task brings `apps/web` to the same layout. It is the single largest, purely-mechanical item and is **independent** of Tasks 1–8; it is committed on its own so the whole-branch review can treat it as a mechanical move (proof = unchanged green suite + same test count). If it balloons, it can be lifted into its own follow-up slice without affecting anything else.

**Files:**
- Move: every `apps/web/src/**/*.test.{ts,tsx}` → `apps/web/test/**/` (mirroring the sub-path under `src/`, e.g. `src/studio/Foo.test.tsx` → `test/studio/Foo.test.tsx`)
- Modify: `apps/web/vitest.config.ts` (test `include` globs; any `setupFiles` path)
- Modify: each moved file's relative imports

- [ ] **Step 1: Baseline — record the current test count and green state**

Run: `cd apps/web && npm test` → record the exact passing test count. Run: `npx tsc --noEmit` → record clean/not. This is the invariant the move must preserve.

- [ ] **Step 2: Read the vitest config + a sample of test imports**

Read `apps/web/vitest.config.ts` (note `include`, `setupFiles`, any path aliases like `@/`). Read 3-4 representative test files at different depths to see their relative-import shapes (`./Component`, `../shared/x`, alias imports). Alias imports (`@/...`) survive a move unchanged; **only relative (`./`, `../`) imports need rewriting.**

- [ ] **Step 3: Move the files preserving sub-paths**

Use `git mv` per file (script it), mapping `src/<sub>/<name>.test.<ext>` → `test/<sub>/<name>.test.<ext>`. Example generator (run from `apps/web`, review before executing):
```bash
cd /Users/houyuxin/08Coding/mind-imprint/apps/web
git ls-files 'src/**/*.test.ts' 'src/**/*.test.tsx' | while read f; do
  dest="test/${f#src/}"
  mkdir -p "$(dirname "$dest")"
  git mv "$f" "$dest"
done
```

- [ ] **Step 4: Rewrite relative imports in the moved files**

Each file moved from `src/<sub>/` to `test/<sub>/` gained one directory level of distance from `src/` (from `apps/web/src/...` the file is now at `apps/web/test/...`). A relative import that pointed into `src` must now reach back: `./Foo` → `../../src/<sub>/Foo`; `../shared/x` → `../../src/shared/x`; in general prepend the correct number of `../` to re-enter `src/`. Because every test moved by the SAME transform (`src/X` → `test/X`), the rewrite is uniform: for a file now at `test/<sub>/<name>`, a former `./y` becomes `../../src/<sub>/y` and a former `../<up>/y` becomes `../../src/<up>/y`. Prefer converting relative source imports to the path alias if the config defines one (`@/` → `src/`), which is move-invariant and cleaner — check `tsconfig`/`vitest.config` for the alias and use it if present. Apply via a codemod (a small Node script or `sed` over the moved files); do not hand-edit 113 files.

- [ ] **Step 5: Update vitest config globs (and setup paths)**

In `vitest.config.ts`, point `include` at the new tree:
```ts
include: ['test/**/*.test.{ts,tsx}'],
```
If `setupFiles` referenced a path under `src/` that moved, update it; if the setup file itself is not a `.test.` file it did NOT move — leave it.

- [ ] **Step 6: Prove the invariant — same green suite, same count**

Run: `cd apps/web && npm test` → the SAME passing test count as Step 1, all green.
Run: `npx tsc --noEmit` → same clean state as Step 1 (no new errors from moved imports).
If the count dropped, the `include` glob is missing files — fix the glob, not the count expectation.

- [ ] **Step 7: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web
git commit -m "chore(n6): relocate apps/web tests into test/ tree (mechanical, suite unchanged)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
**Exception to the never-`git add`-a-directory rule:** this single task legitimately touches only `apps/web` (all moves + config), and `M package.json`/untracked `docs/*` are outside it — but still run `git status` first and confirm nothing unrelated (e.g. `apps/web/package.json` if you did not intend to change it) is staged.

---

## Self-Review

**Spec coverage:** C1→Task 1, C2→Task 2, C3→Task 3, C4→Task 4, C5→Task 5, H1→Task 9, H2+H3→Task 6, H4→Task 7, H5→Task 8. Every spec §2/§3 item maps to a task. The spec's "dropped/deferred" items (N6-B/C/E, L1 blockLookup, cosmetics) are correctly absent.

**Placeholder scan:** No "TBD/TODO/handle edge cases." Where a task says "grep for the existing helper," that is a deliberate instruction to match real package names (the plan cannot invent test-harness names it has not read), not a content gap — the code to write is shown; only the local helper *names* are to be confirmed against the package.

**Type consistency:** `ConfirmGate(ctx, projectID, contract, rec RecordedGate, passedEvent EventRow) error` is defined once (Task 3) and used identically in the fake and `Advance`. `CountClassifierCalls(ctx, projectID) (int64, error)` and `MaxClassifyCallsPerProject = 20` (Task 4) are consistent across interface, impl, fake, and call site. `SubmitAndSkipCardInstance(ctx, projectID, id, fieldValues, eventTrace)` (Task 2) matches its handler call. `CountLLMCallsByProjectPurpose` (sqlc) is consumed only inside `CountClassifierCalls`.

**Ordering:** Tasks 1–5 (Go correctness) are mutually independent; 3 and 4 both extend the `AgentStore` interface + `fakeAgentStore` additively (sequential is clean). Task 9 is last and isolated.

# PBL S2 — sessions, the living plan, and the router Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A project can hold a real conversation: a versioned plan that 印记 revises under grading rules, nestable think-deeply sessions that must write something back, and the per-turn router that decides between doing work, asking one thing, and proposing a session.

**Architecture:** Everything hangs off the atom substrate S1 established. Conversation lives in `atom_message` scoped by a nullable `session_id` (mirroring 0102's `block_id`); sessions are one table with a `kind` and a `parent_id`; the plan is versioned with unconfirmed structural changes staged in `pbl_pending_change` so they can never touch the live plan. The pure rules — depth cap, write-back requirement, change grading — live in `internal/pbl` as functions with no database in them, so they can be tested directly.

**Tech Stack:** Go 1.26 (`net/http`, `pgx/v5`, `sqlc`, `goose`), PostgreSQL, React + Vite + TypeScript.

**Spec:** `docs/superpowers/specs/2026-09-01-pbl-project-room-design.md` (revision 3, §9 §10 §12)

## Global Constraints

Everything in S1's plan still applies. Added for this slice:

- **A structural change can never reach the live plan.** It goes to `pbl_pending_change` and moves only when a Plan Check records her decision. Spec §12.7. This is the gate that keeps the plan hers; if only one thing in this slice is right, make it this.
- **A session cannot close without its write-back.** Spec §12.3.
- **Sessions nest at most 3 deep**, parent must exist. Spec §12.8.
- **A closing session writes back to its PARENT**, not to the main thread. Spec §10.3.
- 🚨 **Fix the seq race in this slice, not after it** — `NextAtomMessageSeq` is a read-then-insert with no `FOR UPDATE`, and sessions make concurrent appends to one atom normal. Spec §10.2.
- `pbl`-prefix every new name; pro owns `project`, `/api/v1/projects`, `a.createProject`, `a.listProjects`.
- sqlc on macOS needs `CGO_CFLAGS="-DHAVE_STRCHRNUL"`; the run can exit 0 having generated nothing, so `git status` the sqlc dir afterwards.
- Never run `go test ./...` beside a live e2e stack — packages time out at 1980s under Docker contention and it looks like a failure.

---

### Task 1: Schema — sessions, versioned plan, staged changes

**Files:**
- Create: `apps/api/internal/store/migrations/0109_pbl_sessions_and_plan.sql`
- Create: `apps/api/internal/store/queries/pbl_session.sql`, `apps/api/internal/store/queries/pbl_plan.sql`
- Test: `apps/api/internal/api/pbl_plan_store_test.go`

**Interfaces produced:** sqlc methods for creating/closing a session, appending a scoped message, reading a thread, creating a plan version, staging and applying a pending change.

Key shapes (full DDL written at implementation time, following 0108's commenting):

- `pbl_session(id, atom_id, kind, parent_id, anchor_kind, anchor_ref, question, takeaway, writeback jsonb, closed_at, created_at)` — `kind IN ('free','observation','reframe','brainstorm','plan_check')`, `parent_id` self-FK.
- `atom_message.session_id uuid REFERENCES pbl_session(id)` + `CHECK (block_id IS NULL OR session_id IS NULL)` + index `(atom_id, session_id, seq)`.
- `pbl_plan_version(id, atom_id, version, summary, reason, decided_by, created_at)`.
- `pbl_plan_step(id, atom_id, version_id, ordinal, title, blurb, goal, you_bring, i_bring, decide, then_bring, status, created_at)` — `status` is the **seven** from §9.3.
- `pbl_pending_change(id, atom_id, kind, diff jsonb, evidence, created_at, resolved_at, resolution, reason)` — `kind IN ('add','modify','remove','defer')`, `resolution IN ('accepted','edited','kept','forked','deferred')`.
- `pbl_decision(id, atom_id, session_id, subject, choice, why, gave_up, created_at)`.

- [ ] Write the migration, mirroring 0108's comment density — say WHY the CHECK sets are what they are.
- [ ] Write the queries; every name `Pbl`-prefixed.
- [ ] `CGO_CFLAGS="-DHAVE_STRCHRNUL" go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate`, then `git status` to confirm it actually generated.
- [ ] Store round-trip test: a session nests, a plan version supersedes another, a pending change does NOT alter live steps.
- [ ] `CGO_ENABLED=0 go test ./internal/api/ -run TestPblPlan -timeout 1800s`
- [ ] Commit.

---

### Task 2: The seq race

**Files:**
- Modify: `apps/api/internal/store/queries/atom.sql` (add a locking query)
- Modify: the append paths that call `NextAtomMessageSeq`
- Test: `apps/api/internal/api/pbl_seq_race_test.go`

**Why now:** with sessions, two conversations on one atom is the design, so the read-then-insert race stops being unreachable. It fails AFTER the model call is paid for.

- [ ] Add `LockAtom :one` — `SELECT id FROM atom WHERE id = $1 FOR UPDATE`.
- [ ] Call it at the top of every transaction that allocates a seq, before `NextAtomMessageSeq`.
- [ ] Write a test that runs two concurrent appends against one atom and asserts both land with distinct seqs — it must FAIL before the lock is added. A race test that passes without the fix is testing nothing.
- [ ] Commit.

---

### Task 3: The pure rules

**Files:**
- Create: `apps/api/internal/pbl/session.go`, `apps/api/internal/pbl/plan.go`
- Test: alongside each.

**Interfaces produced:**
- `SessionKinds`, `IsSessionKind(string) bool`
- `ValidateNesting(parentDepth int, parentExists bool) error` — enforces the cap of 3
- `RequiredWriteBack(kind string) []string` — which fields a kind must produce to close
- `ValidateClose(kind string, wb WriteBack) error`
- `GradeChange(c ProposedChange) Grade` — `Progress | Local | Structural | Fork`
- `StepStatuses`, `IsStepStatus(string) bool`

These are where the spec's invariants become executable. No database, no HTTP — so the tests are about behaviour, not wiring.

- [ ] Test first for each: the cap rejects depth 3→4; a close with no write-back is refused per kind; grading puts a changed success criterion in `Structural` and a finished step in `Progress`.
- [ ] Implement.
- [ ] `CGO_ENABLED=0 go test ./internal/pbl/ -v`
- [ ] Commit.

---

### Task 4: Session endpoints

**Files:**
- Create: `apps/api/internal/api/pbl_sessions.go`, `+_test.go`
- Modify: `apps/api/internal/api/api.go`

Routes: `POST /api/v1/pbl/projects/{id}/sessions`, `POST …/sessions/{sid}/close`, `GET …/sessions`, `GET …/thread`.

- [ ] Ownership via the project's atom; a session on someone else's project is a 404, never a 403.
- [ ] Close refuses without the write-back, and appends it to the PARENT thread.
- [ ] Tests: nesting cap, write-back refusal, parent-not-mine, the write-back landing in the right thread.
- [ ] Commit.

---

### Task 5: Plan endpoints and Plan Check

**Files:**
- Create: `apps/api/internal/api/pbl_plan.go`, `+_test.go`
- Modify: `apps/api/internal/api/api.go`

Routes: `GET …/plan`, `POST …/plan/approve`, `PATCH …/plan/steps/{sid}`, `POST …/plan/changes` (stage), `POST …/plan/changes/{cid}/resolve` (Plan Check).

- [ ] 🚨 The test that matters: **stage a structural change, then assert the live plan is byte-identical.** Only `resolve` with a recorded decision creates the new version.
- [ ] `resolve` accepts all five outcomes including `kept`, which records her reason and is a success, not a no-op.
- [ ] Nothing runs before the first plan is approved.
- [ ] Commit.

---

### Task 6: The router

**Files:**
- Create: `apps/api/internal/pbl/router.go`, `+_test.go`

`Route(state) Decision` implementing spec §10.5's six-step order, returning `Boundary | DoWork | AskOne | ProposeSession | PlanCheck | Continue`.

- [ ] Pure over an explicit state struct, so the ordering is testable without a model.
- [ ] Tests: a safety flag beats everything; a confirmed direction produces `DoWork` rather than another question; a declined proposal does not return without a new reason; never two high-load sessions back to back.
- [ ] Commit.

---

### Deferred to S3, deliberately

The concrete interaction — brainstorming room, reframe card, research hint — is the owner's design and not yet made. This slice builds the substrate those surfaces will sit on, and stops there. The frontend in S2 is only what is needed to see a thread and a plan.

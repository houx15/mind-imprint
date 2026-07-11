# Slice 2 — Runtime Loop + Classifier + Coach Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first live agent runtime — the graph-triggered loop, the cheap classifier (pure trigger predicates), and the flagship coach emitting one enforcement-checked `post_intervention`, persisted — over a fixture graph, no cards/planner/UI.

**Architecture:** New files in `apps/api/internal/agent` (evolve, not a parallel package), reusing `gateway` (Provider/collect/keyResolver), Slice 0 `enforcement`, and sqlc. The coach's body text comes from the flagship model; the anchor + criterion come from the classifier's `Candidate`. Design: `docs/superpowers/specs/2026-07-11-slice-2-runtime-coach-design.md`.

**Tech Stack:** Go (pgx/sqlc) + testcontainers + the `gateway` provider abstraction.

## Global Constraints

- **Evolve the `agent` package** — new files only. **Do NOT modify `promptTemplate`/`BuildSystemPrompt` in `prompt.go`** (byte-for-byte golden-fixture parity for legacy `RunTurn`). The new coach gets its **own** posture prompt. **Do NOT modify `turn.go`/`RunTurn`** (legacy, retired in Slice 5) except—if needed—adding a `// LEGACY: retired in Slice 5` comment.
- **Reuse, don't duplicate:** `gateway.Provider` + the stream `collect` helper + `keyResolver`; Slice 0 `enforcement` (`AgentOutput`, `ValidateOutput`, `BannedPhrasing`, `OutputCheck`, `GuardStudentField`).
- **Coach emits ONLY `post_intervention`.** Other verbs exist in the C3 union but are not executed; the decision logic never routes to them.
- **Enforcement under every coach output**, before persist/return; the `output_check_verdict` is stored on the `intervention` row.
- **No secrets** in code/logs/errors. **No new migration** (Slice 0 tables suffice) — only new sqlc queries.
- **Tests:** integration guards with `if testing.Short(){t.Skip()}` + `newTestPool`; the provider is a scripted **stub** (follow the `queueProvider` pattern in `internal/api/e2e_test.go`, or `gateway/stub.go`). No live model call in the suite.
- **Commands** (from `apps/api`): `make sqlc` after editing queries; `go build ./...`; `go vet ./...`; `gofmt -w`; `go test ./... -short`; and the new integration tests without `-short` (Docker is up).

---

## Task 1: sqlc queries — intervention + graph reads

**Files:**
- Create: `apps/api/internal/store/queries/intervention.sql`
- Modify: `apps/api/internal/store/queries/graph.sql` (add a by-project node+edge read if missing)
- Regenerate: `apps/api/internal/store/sqlc/*`
- Test: `apps/api/internal/store/refactor2_runtime_sqlc_test.go`

**Interfaces:**
- Produces: `InsertIntervention` (project_id, card_instance_id?, type, anchor jsonb, criterion, body, level, output_check_verdict → id), `ListInterventionsByProject`; confirm `ListGraphNodesByProject` + add `ListGraphEdgesByProject` if not present.

- [ ] **Step 1: Failing test** (`-short`-guarded, `TestRefactor2RuntimeStore...`): seed a project + a graph_node; `InsertIntervention` anchored to it with a verdict; `ListInterventionsByProject` returns it with the right fields.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** the `-- name:` queries; `make sqlc`; commit generated files. `anchor` is a jsonb column — pass `[]byte`.
- [ ] **Step 4: Test → PASS** (`go test ./internal/store/ -run Refactor2Runtime`).
- [ ] **Step 5: Commit** — `feat(api): sqlc queries for intervention + graph edge reads`.

---

## Task 2: Classifier — trigger predicates

**Files:**
- Create: `apps/api/internal/agent/runtime.go` (shared runtime types), `apps/api/internal/agent/classifier.go`
- Test: `apps/api/internal/agent/classifier_test.go`

**Interfaces:**
- Produces: `GraphView` (`Nodes []GraphNodeView`, `Edges []GraphEdgeView`), `Candidate` (`Verb, AnchorKind, AnchorID, Criterion, Reason, Level string`), and `CandidateMoves(g GraphView) []Candidate`.

- [ ] **Step 1: Failing test**

```go
func TestCandidateMoves_ClaimWithoutEvidence(t *testing.T) {
  g := GraphView{
    Nodes: []GraphNodeView{{ID: "n1", Type: "claim", Author: "student"}},
    Edges: nil, // no evidence supports the claim
  }
  cands := CandidateMoves(g)
  if len(cands) != 1 { t.Fatalf("want 1 candidate, got %d", len(cands)) }
  if cands[0].Verb != "post_intervention" || cands[0].AnchorID != "n1" {
    t.Fatalf("unexpected candidate: %+v", cands[0])
  }
}
func TestCandidateMoves_SupportedClaimIsSilent(t *testing.T) {
  g := GraphView{
    Nodes: []GraphNodeView{{ID: "n1", Type: "claim", Author: "student"}, {ID: "e1", Type: "evidence", Author: "student"}},
    Edges: []GraphEdgeView{{FromKind: "graph_node", FromID: "e1", ToKind: "graph_node", ToID: "n1", Type: "supports"}},
  }
  if got := CandidateMoves(g); len(got) != 0 { t.Fatalf("supported claim must be silent, got %d", len(got)) }
}
```

- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — `runtime.go` defines `GraphView`/`GraphNodeView`/`GraphEdgeView`/`Candidate`/`Trigger`/`Action`. `CandidateMoves` implements the one Slice-2 predicate: for each `claim` node with **no** incoming `supports` edge from an `evidence` node, emit a `Candidate{Verb:"post_intervention", AnchorKind:"graph_node", AnchorID: node.ID, Criterion:"D5", Reason:"claim has no supporting evidence", Level:"I2"}`. Keep it pure (no DB). Return them in stable node order.
- [ ] **Step 4: Test → PASS.** `go vet`, `gofmt -w`.
- [ ] **Step 5: Commit** — `feat(api): classifier trigger predicate (unsupported-claim -> intervention candidate)`.

---

## Task 3: Coach — posture prompt + enforced `post_intervention`

**Files:**
- Create: `apps/api/internal/agent/coach.go`, `apps/api/internal/agent/coach_prompt.go`
- Test: `apps/api/internal/agent/coach_test.go`

**Interfaces:**
- Consumes: `Candidate`/`GraphView` (Task 2), `gateway.Provider`, `enforcement`.
- Produces: `coachPosturePrompt` (const), `BuildCoachPrompt(g GraphView, c Candidate) string`, and `ProposeIntervention(ctx, prov gateway.Provider, r gateway.Resolved, g GraphView, c Candidate) (enforcement.AgentOutput, string /*verdict*/, error)`.

- [ ] **Step 1: Failing test** — with a **stub provider** scripted to stream the body text "这条主张现在还没有素材支撑——它的证据是什么？"：
  - `ProposeIntervention` returns an `AgentOutput{Type:"question", Anchor:{Kind:"graph_node", ID:"n1"}, Criterion:"D5", Body: <that text>}` and a non-empty verdict.
  - A stub that streams a **declarative echo** of the topic is **intercepted** — the returned body ends with "？" (rewritten as a question) and the verdict reflects intercept.
  - A stub that streams a **banned phrase** ("你有没有考虑过其他角度？") causes `ProposeIntervention` to return an error (rejected before persist).
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — `coach_prompt.go` holds `coachPosturePrompt`: the restraint-ladder posture for the NEW model (克制、一次一问、锚定到具体节点、绝不代笔、用 CT 维度语言), instructing the model to output **only the one anchored question body** (no preamble). `BuildCoachPrompt` assembles it with the graph neighborhood (the candidate's node + its context) and `c.Reason`. `ProposeIntervention` calls `prov.Stream` and collects the text (reuse the `gateway` collect helper as `turn.go` does), builds `AgentOutput{Type:"question", Anchor:{Kind:c.AnchorKind, ID:c.AnchorID}, Criterion:c.Criterion, Body: collected}`, then runs the enforcement stack: `ValidateOutput` → `BannedPhrasing(Body)` (non-nil ⇒ return error) → `OutputCheck(Body, enforcement.Context{Topic: <project topic/claim text>}, sim)` (on intercept, replace Body with the rewritten question); return the output + a verdict string ("pass"/"intercept"). The `sim` provider is injected (a simple heuristic or the same stub seam as Slice 0 — no real embedding in Slice 2).
- [ ] **Step 4: Test → PASS.** `go vet`, `gofmt -w`.
- [ ] **Step 5: Commit** — `feat(api): coach — posture prompt + enforced post_intervention`.

---

## Task 4: The loop + store seam + integration

**Files:**
- Create: `apps/api/internal/agent/loop.go`, and an `AgentStore` sqlc adapter (e.g. `apps/api/internal/agent/agentstore.go` or extend an existing adapter)
- Test: `apps/api/internal/agent/loop_test.go` (unit, stubbed store+provider) and `apps/api/internal/store/` or `internal/api/` integration if a real-DB pass is wanted — a stubbed-store unit test is the primary gate; add one testcontainers pass if straightforward.

**Interfaces:**
- Consumes: everything above.
- Produces: `AgentStore` interface (`LoadGraph(ctx, projectID) (GraphView, error)`, `InsertIntervention(ctx, InterventionRow) (uuid.UUID, error)`, `AppendEvent(ctx, EventRow) error`), `AgentDeps` (`Store AgentStore; Provider gateway.Provider; Resolved gateway.Resolved; Sim enforcement.Similarity`), and `RunAgentStep(ctx, deps AgentDeps, projectID uuid.UUID, trigger Trigger) (*Action, error)`.

- [ ] **Step 1: Failing test** (unit, fake `AgentStore` + stub provider):
  - Graph = one unsupported claim → `RunAgentStep` returns a non-nil `Action`, calls `InsertIntervention` once (with the anchor/criterion/verdict) and `AppendEvent` once; the returned `Action.Output` is the coach's question.
  - Graph = a supported claim → `RunAgentStep` returns `(nil, nil)` (silence) and persists nothing.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — `RunAgentStep`: `LoadGraph` → `CandidateMoves` → if empty return `(nil,nil)` (silence) → take the first candidate (decide-one) → `ProposeIntervention` → on enforcement error, log server-side and return silence (never persist a rejected output) → `InsertIntervention` (anchor JSON, criterion, body, level, verdict) → `AppendEvent` (`{surface:"studio", type:"prompt_sent"|"intervention_posted"}` — use an existing/added event type) → return `&Action{Output, InterventionID}`. The sqlc `AgentStore` adapter maps to Task 1's queries + `LoadGraph` from the graph reads.
- [ ] **Step 4: Test → PASS** (`go test ./internal/agent/ -run Loop`). If a testcontainers pass is added, run it without `-short`.
- [ ] **Step 5: Commit** — `feat(api): the agent loop — perceive/classify/decide-one/coach/enforce/record`.

---

## Task 5: Verification + roadmap log

**Files:**
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md` (Slice 2 status ☑ + per-slice log; add the Slice-5 retirement note for `RunTurn`).

- [ ] **Step 1:** From `apps/api`: `go build ./...`; `go vet ./...`; `go test ./... -short`; the Task 1 integration test without `-short`; confirm the legacy `RunTurn` tests still pass and `promptTemplate` is unchanged (`git diff main...HEAD -- internal/agent/prompt.go` empty except any LEGACY comment).
- [ ] **Step 2:** Confirm acceptance criteria (design §10): loop emits one enforced `post_intervention` on a candidate and silence otherwise; enforcement intercepts a declarative echo and rejects a banned phrase; classifier predicates pure + tested; no `surface_card`/planner/UI; no orphaned/duplicated code.
- [ ] **Step 3:** Update the roadmap: Slice 2 ☑, link spec + plan, commit range; add "`RunTurn`/`summon_card` → delete in Slice 5" to the retirement note.
- [ ] **Step 4: Commit** — `docs(refactor2): Slice 2 runtime + coach complete — log + status`.

---

## Self-review notes

- **Spec coverage:** loop (T4) · classifier (T2) · coach + posture prompt (T3) · enforcement wiring (T3) · model routing via gateway (T3/T4) · persistence + sqlc (T1, T4) · verification (T5). All design §2–§8 map to a task.
- **Deferred by design:** `surface_card` (Slice 3) · planner/intake (Slice 4) · UI/SSE (Slice 5) · assessor (Slice 10) · the cheap-model classifier hook (seam only; pure predicate now).
- **No-orphan check:** new files reuse gateway/enforcement/sqlc; the legacy `RunTurn` stays live + compiling, marked for Slice-5 deletion; `promptTemplate` untouched; the new coach prompt is a distinct prompt for a distinct runtime, not a duplicate.
- **Type consistency:** `enforcement.AgentOutput` is the coach's output type end-to-end; `Candidate`/`GraphView` defined once in `runtime.go`; `AgentStore`/`AgentDeps` defined once in `loop.go`.

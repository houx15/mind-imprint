# N3d · Walkable stations (S0–S2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make S0 → S1 → S2 → S3 walkable by a real student for the first time: give every gate item on those stations an honest live producer, build the two views the binding design already specifies, and give gate advancement a live caller.

**Architecture:** `agent.AdvanceAll` walks the contract DAG and confirms every contract whose own gate has zero `Missing` and whose `requires` are all already solid; a best-effort `API.advanceGates` calls it after every gate-affecting write. Two new student-write endpoints (`/framing`, `/perspectives`) persist S1/S2 work as graph nodes carrying an `origin:"station_view"` marker, which scopes their delete-then-insert so they can never eat nodes minted by tool cards. Three of the four `student_written` items are recorded by the endpoint that persists the corresponding writing; one gets an explicit reversible confirm through the existing generic attest endpoint.

**Tech Stack:** Go (`net/http` + `pgx`/`sqlc`), PostgreSQL, React + Vite + TypeScript, Zod contracts.

Spec: `docs/superpowers/specs/2026-07-22-n3d-walkable-stations-design.md`

## Global Constraints

- **NO migration.** `git diff` over the branch must contain no file under `apps/api/internal/store/migrations/`.
- **NO LLM call** anywhere in this slice. No new `llm_call` rows, no metering concerns.
- **NO new gate predicate kind** and **no change to `writing-project.json`'s contract definitions** (neither `packages/contracts/skills/` nor `apps/api/internal/skills/specs/`). Every gate item is satisfied as written.
- **NEVER hand-edit `apps/api/internal/store/sqlc/*`.** Regenerate with `cd apps/api && make sqlc`.
- **NEVER hand-edit `apps/api/internal/cards/specs/*`.** (No card JSON changes in this slice at all.)
- Icons are **inline SVG**. Never import `lucide-react`.
- **NEVER `git add` a whole directory.** `M package.json` and the untracked files under `docs/` and the repo root are pre-existing and NOT ours. Stage named files only.
- Go tests: from `apps/api`, `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`. Run **FULL packages**, never `-run` subsets, for anything touching gates, projections, or card effects.
- Web tests: from `apps/web`, `npm test` and `npx tsc --noEmit`. Contracts: from `packages/contracts`, `npm test` (tests live in `test/`, not `src/`).
- 铁律 2 (不操纵): no streaks, badges, scores, celebration, or congratulatory copy on a station turning `done`. **An offer is never a wall** — every S1/S2 form saves whatever she has written; the gate, not the form, is what is unfinished.
- 铁律 1 (AI 克制): nothing in this slice authors student content.
- DEC-3: `Advance`/`AdvanceAll` never records a `student_written` or `human` gate item.
- The `origin:"station_view"` marker is load-bearing — every node written by the S0/S1/S2 endpoints carries it, and every delete is scoped by it.
- All new copy is Chinese and matches `docs/design/思维印记_工作区.dc.html` verbatim where the design has copy.

---

### Task 1: Contracts — the S1/S2 wire shapes

**Files:**
- Modify: `packages/contracts/src/studioState.ts`
- Test: `packages/contracts/test/studioState.test.ts`

**Interfaces:**
- Produces: `FramingFx`, `TermDefinition`, `PerspectiveLevel`, `PerspectiveRow`, `PerspectivesFx`, `FramingSubmitBody`, `PerspectivesSubmitBody`; two new required members on `StudioProjection.views`: `framing`, `perspectives`.

- [ ] **Step 1: Write the failing test**

Append to `packages/contracts/test/studioState.test.ts`:

```ts
import { FramingFx, PerspectivesFx, FramingSubmitBody, PerspectivesSubmitBody } from "../src/studioState";

describe("N3d station view shapes", () => {
  it("parses a framing projection", () => {
    const fx = FramingFx.parse({
      researchQuestion: "中国在多大程度上让世界更可持续？",
      terms: [{ term: "sustainable", definition: "资源使用不损害后代的能力" }],
      answers: ["趋势变好不等于问题已解决"],
      searchPlan: ["官方一手数据（NASA / IEA / BP）"],
    });
    expect(fx.terms[0].term).toBe("sustainable");
  });

  it("carries card-minted perspectives as non-editable rows with a blank level", () => {
    const fx = PerspectivesFx.parse({
      rows: [
        { text: "中国政府视角", level: "national", editable: true },
        { text: "来自矩阵卡的一行", level: "", editable: false },
      ],
      sourcesPerPerspective: false,
    });
    expect(fx.rows[1].editable).toBe(false);
    expect(fx.rows[1].level).toBe("");
  });

  it("rejects an unknown perspective level in a submit body", () => {
    expect(() =>
      PerspectivesSubmitBody.parse({ perspectives: [{ text: "x", level: "cosmic" }] }),
    ).toThrow();
  });

  it("accepts a framing submit body", () => {
    const b = FramingSubmitBody.parse({ terms: [], answers: [], searchPlan: [] });
    expect(b.answers).toEqual([]);
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run from `packages/contracts`: `npm test`
Expected: FAIL — `FramingFx` is not exported.

- [ ] **Step 3: Add the schemas**

In `packages/contracts/src/studioState.ts`, after the `OnboardingFx` block:

```ts
// N3d: S1 立题. The terms are the STUDENT's own — she names which words in her
// own question need defining (spec §5.2). researchQuestion is the read-only
// banner, minted from the project title at creation.
export const TermDefinition = z.object({ term: z.string(), definition: z.string() });
export type TermDefinition = z.infer<typeof TermDefinition>;

export const FramingFx = z.object({
  researchQuestion: z.string(),
  terms: z.array(TermDefinition),
  answers: z.array(z.string()),
  searchPlan: z.array(z.string()),
});
export type FramingFx = z.infer<typeof FramingFx>;

// N3d: S2 视角与素材. The three levels are the binding design's own
// (dc.html:2158). A row minted by the perspective-matrix card carries NO level
// (that card has no level field) and is not editable here — but it still counts
// toward the station's node_count_at_least gate, so `level` is a plain string
// with "" for those rows rather than the enum.
export const PerspectiveLevel = z.enum(["national", "global_for", "global_against"]);
export type PerspectiveLevel = z.infer<typeof PerspectiveLevel>;

export const PerspectiveRow = z.object({
  text: z.string(),
  level: z.string(),
  editable: z.boolean(),
});
export type PerspectiveRow = z.infer<typeof PerspectiveRow>;

export const PerspectivesFx = z.object({
  rows: z.array(PerspectiveRow),
  // The recorded sources_per_perspective attestation (spec §6.2) — the one
  // explicit confirm in this slice.
  sourcesPerPerspective: z.boolean(),
});
export type PerspectivesFx = z.infer<typeof PerspectivesFx>;
```

And next to `OnboardingSubmitBody`:

```ts
export const FramingSubmitBody = z.object({
  terms: z.array(TermDefinition),
  answers: z.array(z.string()),
  searchPlan: z.array(z.string()),
});
export type FramingSubmitBody = z.infer<typeof FramingSubmitBody>;

export const PerspectivesSubmitBody = z.object({
  perspectives: z.array(z.object({ text: z.string(), level: PerspectiveLevel })),
});
export type PerspectivesSubmitBody = z.infer<typeof PerspectivesSubmitBody>;
```

- [ ] **Step 4: Add the two `views` members**

In the `StudioProjection` object literal in the same file, alongside `onboarding: OnboardingFx,`:

```ts
  framing: FramingFx,
  perspectives: PerspectivesFx,
```

Export the new symbols from `packages/contracts/src/index.ts` following exactly how `OnboardingFx` / `OnboardingSubmitBody` are exported there.

- [ ] **Step 5: Run tests**

Run from `packages/contracts`: `npm test`
Expected: PASS, and the pre-existing `StudioProjection` round-trip tests will now FAIL because their fixtures lack `views.framing` / `views.perspectives`. Add the two members to every failing fixture with the minimal valid value:

```ts
framing: { researchQuestion: "", terms: [], answers: [], searchPlan: [] },
perspectives: { rows: [], sourcesPerPerspective: false },
```

Re-run until green.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/studioState.ts packages/contracts/src/index.ts packages/contracts/test/studioState.test.ts
git commit -m "feat(n3d): contracts — FramingFx/PerspectivesFx + submit bodies"
```

---

### Task 2: The scoped delete query

**Files:**
- Modify: `apps/api/internal/store/queries/graph.sql`
- Generated (do not hand-edit): `apps/api/internal/store/sqlc/graph.sql.go`

**Interfaces:**
- Produces: `Queries.DeleteStationViewNodes(ctx, DeleteStationViewNodesParams{ProjectID, Types []string}) error`

- [ ] **Step 1: Add the query**

Append to `apps/api/internal/store/queries/graph.sql`:

```sql
-- name: DeleteStationViewNodes :exec
-- Deletes ONLY the nodes the S0/S1/S2 station views themselves wrote, identified
-- by the body marker origin='station_view'. This scoping is load-bearing: the
-- perspective-matrix tool card also mints `perspective` nodes (agent/card_effects.go),
-- and a re-save of the S2 view must never delete them. Callers pass an explicit
-- type list; there is deliberately no "delete everything for this project" form.
DELETE FROM graph_node
WHERE project_id = $1
  AND type = ANY(@types::text[])
  AND body->>'origin' = 'station_view';
```

- [ ] **Step 2: Regenerate**

```bash
cd apps/api && CGO_ENABLED=0 make sqlc
git status --short internal/store/sqlc
```

Expected: exactly `M internal/store/sqlc/graph.sql.go`. If `make sqlc` fails, do NOT hand-write the Go — report BLOCKED.

- [ ] **Step 3: Commit**

```bash
git add apps/api/internal/store/queries/graph.sql apps/api/internal/store/sqlc/graph.sql.go
git commit -m "feat(n3d): sqlc — DeleteStationViewNodes (origin-scoped)"
```

---

### Task 3: `agent.AdvanceAll`

**Files:**
- Modify: `apps/api/internal/agent/planner.go`
- Test: `apps/api/internal/agent/planner_test.go`

**Interfaces:**
- Consumes: existing `Advance`, `ReconcileGates`, `Replan`, `skills.Skill.TopoOrder()`.
- Produces: `func AdvanceAll(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill) ([]string, error)`

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/agent/planner_test.go`. Follow the file's existing fake-store construction verbatim — read the top of that file first and reuse whatever helper the existing `Advance` tests use to build `AgentDeps` and seed graph/gate rows; do not invent a second fake.

Three tests:

```go
// A contract whose own gate is fully satisfied but whose predecessor is NOT
// solid must not advance. Without this rule a student could have S2 marked
// done above a still-current S1, because Advance only ever inspects one
// contract's own gate items.
func TestAdvanceAll_HoldsBehindUnsolidPredecessor(t *testing.T) { /* ... */ }

// One write can legitimately close several stations at once, so the walk is
// single-pass in topological order and a contract confirmed earlier in the
// walk counts as solid for its successors.
func TestAdvanceAll_CascadesInTopoOrder(t *testing.T) { /* ... */ }

// Already-solid contracts are skipped (no duplicate gate_attempt events) and
// an empty result means Replan is not called.
func TestAdvanceAll_NoopWhenNothingChanged(t *testing.T) { /* ... */ }
```

Build each fixture so the assertion cannot pass vacuously: for the first test, satisfy `evaluate_perspectives`'s machine + recorded items fully while leaving one `frame_question` item unmet, and assert the returned slice does **not** contain `evaluate_perspectives`.

- [ ] **Step 2: Run to verify they fail**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/
```

Expected: FAIL — `AdvanceAll` undefined.

- [ ] **Step 3: Implement**

Append to `apps/api/internal/agent/planner.go`:

```go
// AdvanceAll confirms every contract that is now genuinely finished, walking
// the contract DAG once in topological order, and returns the ids it newly
// advanced. It is the live caller Advance never had: before N3d nothing in
// production ever set RecordedGate.Confirmed, so no station could become
// `done` for any project whose gate_state rows weren't hand-written by a seed
// migration.
//
// A contract advances iff (a) it is not already solid, (b) every id in its
// Requires is solid — already recorded, or advanced earlier in THIS walk — and
// (c) its own gate has nothing Missing. Rule (b) is deliberately stricter than
// Route's reachability (which admits a machine_clear predecessor): work may
// begin once predecessors are structurally sound, but a station is only
// FINISHED behind finished predecessors.
//
// DEC-3 holds throughout: this delegates the confirm to Advance, which never
// records a student_written or human item itself.
func AdvanceAll(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill) ([]string, error) {
	order, err := sk.TopoOrder()
	if err != nil {
		return nil, err
	}
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}
	recorded, err := deps.Store.ListGateStates(ctx, projectID)
	if err != nil {
		return nil, err
	}
	solid := make(map[string]bool, len(order))
	for _, id := range order {
		solid[id] = recorded[id].Confirmed
	}

	var advanced []string
	for _, id := range order {
		if solid[id] {
			continue
		}
		ready := true
		for _, req := range sk.Contracts[id].Requires {
			if !solid[req] {
				ready = false
				break
			}
		}
		if !ready {
			continue
		}
		if len(CheckGate(sk, id, g, recorded[id]).Missing) > 0 {
			continue
		}
		ok, err := Advance(ctx, deps, projectID, sk, id)
		if err != nil {
			return nil, err
		}
		if ok {
			solid[id] = true
			advanced = append(advanced, id)
		}
	}
	if len(advanced) > 0 {
		if _, err := Replan(ctx, deps, projectID, sk, "advanced"); err != nil {
			return nil, err
		}
	}
	return advanced, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/
```

Expected: PASS (full package, not a `-run` subset).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/planner.go apps/api/internal/agent/planner_test.go
git commit -m "feat(n3d): agent.AdvanceAll — DAG-ordered gate advancement behind solid predecessors"
```

---

### Task 4: `advanceGates` + wire into the existing write endpoints + `recon_logged`

**Files:**
- Create: `apps/api/internal/api/advance.go`
- Modify: `apps/api/internal/api/materials.go` (the `logSourceOpen` handler — locate it by name, the file may differ)
- Modify: `apps/api/internal/api/writing.go` (`attestGate`)
- Modify: `apps/api/internal/api/reflection.go` (`submitReflection`)
- Modify: the file holding `submitProjectCard`
- Test: `apps/api/internal/api/advance_test.go`

**Interfaces:**
- Consumes: `agent.AdvanceAll` (Task 3).
- Produces: `func (a *API) advanceGates(ctx context.Context, projectID uuid.UUID)`; `func (a *API) attestReconLogged(ctx context.Context, projectID uuid.UUID)`

- [ ] **Step 1: Write the failing test**

`apps/api/internal/api/advance_test.go` — use whatever integration harness the existing `internal/api` tests use (read a neighbouring `*_test.go` first; several spin a testcontainer via a shared helper — reuse it, do not build a new one).

```go
// Opening a source records the evaluate_perspectives gate's recon_logged item:
// the 检索日志 entry IS the recon, and logSourceOpen is the moment it happened.
func TestLogSourceOpen_AttestsReconLogged(t *testing.T) { /* ... */ }

// advanceGates must never fail the student's write. With a store that errors,
// the handler still returns 2xx.
func TestAdvanceGates_IsBestEffort(t *testing.T) { /* ... */ }
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/
```

- [ ] **Step 3: Implement `advance.go`**

```go
package api

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/skills"
)

// advanceGates runs agent.AdvanceAll for the project's skill. Gate state is
// DERIVED and fully recomputable from the graph, so a failure here must never
// fail the student's write — the next gate-affecting write recomputes it. This
// is why it returns nothing and logs instead.
func (a *API) advanceGates(ctx context.Context, projectID uuid.UUID) {
	sk, ok := skills.ByID("writing-project")
	if !ok {
		slog.Warn("advance gates: skill missing", "project_id", projectID.String())
		return
	}
	deps := a.agentDeps() // reuse however other handlers build AgentDeps; see studioturn.go
	if _, err := agent.AdvanceAll(ctx, deps, projectID, sk); err != nil {
		slog.Warn("advance gates failed", "err", err, "project_id", projectID.String())
	}
}

// attestReconLogged records evaluate_perspectives' student_written recon_logged
// item once the project's 检索日志 (source log, Slice 6b) has at least one entry.
// Opening and logging a source IS the recon; there is no separate control for
// it (spec §6.2). Merges into any existing recorded gate_state so a
// sources_per_perspective attestation is never clobbered.
func (a *API) attestReconLogged(ctx context.Context, projectID uuid.UUID) { /* ... */ }
```

Look at `studioturn.go` for the exact way `agent.AgentDeps` is assembled in this package and follow it; if there is no existing helper, build the deps inline in `advanceGates` the same way `studioturn.go` does and leave a comment saying so.

`attestReconLogged` follows `submitReflection`'s gate-merge block verbatim (`ListGateStates` → take `recorded["evaluate_perspectives"]` → ensure `Items` map → set `Items["recon_logged"] = "solid"` → `UpsertGateState`), but logs on error rather than failing the request, since it is a side effect of opening a source.

- [ ] **Step 4: Wire the call sites**

Add `a.advanceGates(r.Context(), projectID)` as the **last statement before the success response** in exactly these handlers:

- `submitOnboarding` (`onboarding_submit.go`)
- `logSourceOpen` — call `a.attestReconLogged(...)` first, then `a.advanceGates(...)`
- `attestGate` (`writing.go`)
- `submitReflection` (`reflection.go`)
- `submitProjectCard`

Do NOT add it to `ingestMaterial`, to any GET, or to the turn loop.

- [ ] **Step 5: Run tests**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ ./internal/agent/
```

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/advance.go apps/api/internal/api/advance_test.go <the modified handler files>
git commit -m "feat(n3d): live gate advancement — advanceGates on every gate-affecting write + recon_logged"
```

---

### Task 5: S0's missing producers

**Files:**
- Modify: `apps/api/internal/api/project_create.go`
- Modify: `apps/api/internal/api/onboarding_submit.go`
- Test: `apps/api/internal/api/onboarding_submit_test.go` (create if absent), `apps/api/internal/api/project_create_test.go`

**Interfaces:**
- Consumes: `Queries.DeleteStationViewNodes` (Task 2), `a.advanceGates` (Task 4).

- [ ] **Step 1: Write the failing tests**

```go
// The project title IS her research question — minted at creation so S1's
// read-only banner has a real node behind it and frame_question's
// node_present:research_question item has a producer at all.
func TestCreateProject_MintsResearchQuestion(t *testing.T) { /* ... */ }

// Each weak pick becomes a weakness_prediction node so decode_task's
// node_count_at_least{weakness_prediction,2} has a producer.
func TestSubmitOnboarding_MintsWeaknessPredictions(t *testing.T) { /* ... */ }

// Re-submitting must not inflate the count past the n>=2 gate: two submits of
// ONE pick leave exactly one node, not two. (Without the origin-scoped delete
// a student could clear the gate by pressing save twice.)
func TestSubmitOnboarding_ReplacesWeaknessPredictions(t *testing.T) { /* ... */ }

// milestone_plan is recorded on her submit, like reflection.go records its own.
func TestSubmitOnboarding_AttestsMilestonePlan(t *testing.T) { /* ... */ }
```

- [ ] **Step 2: Run to verify they fail**

- [ ] **Step 3: Implement `project_create.go`**

Add a fourth entry to the existing node loop, inside the same transaction:

```go
	// The title is the research question she typed in the creation funnel —
	// author "student" because she wrote it. S1's banner renders it read-only
	// (dc.html:878–884) and frame_question's node_present item reads it.
	rqBody, _ := json.Marshal(map[string]any{"text": title})
```

then in the slice literal, after `{"milestone_plan", "ai", planBody},`:

```go
		{"research_question", "student", rqBody},
```

- [ ] **Step 4: Implement `onboarding_submit.go`**

After the existing `task_restatement` insert, and before the event append, wrap the weakness-prediction write in a transaction:

```go
	// Delete-then-insert so a re-submit cannot inflate the count past
	// decode_task's node_count_at_least{weakness_prediction,2}: pressing save
	// twice with one pick must leave ONE node. Scoped by origin so it can only
	// ever touch nodes this view wrote.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil { httpx.WriteError(w, r, err); return }
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	if err := qtx.DeleteStationViewNodes(r.Context(), sqlc.DeleteStationViewNodesParams{
		ProjectID: projectID, Types: []string{"weakness_prediction"},
	}); err != nil { httpx.WriteError(w, r, err); return }

	for _, i := range req.WeakPicks {
		body, _ := json.Marshal(map[string]any{
			"index": i, "plain": plainForRow(rows, i), "origin": "station_view",
		})
		if _, err := qtx.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
			ProjectID: projectID, Type: "weakness_prediction", Body: body, Author: "student", SpanRef: nil,
		}); err != nil { httpx.WriteError(w, r, err); return }
	}
	if err := tx.Commit(r.Context()); err != nil { httpx.WriteError(w, r, err); return }
```

`rows` comes from reading the project's existing `rubric_translation` node (via `ListGraphNodesByProject`, unmarshalling the same `{restate_prompt, rows}` shape `projectOnboarding` reads). `plainForRow` returns `""` when the node or index is missing — the node then carries `index` alone rather than fabricating a label.

Then record the gate item, following `submitReflection`'s merge block verbatim:

```go
	// S0's deliverable is her restate + her weakness picks; the milestone plan is
	// the board-static scaffold she accepts by proceeding. Recorded on HER action,
	// exactly as reflection.go records `reflection` — Advance never marks a
	// student_written item itself (DEC-3).
	rec := recorded["decode_task"]
	if rec.Items == nil { rec.Items = map[string]string{} }
	rec.Items["milestone_plan"] = "solid"
	// UpsertGateState ...
```

and finish with `a.advanceGates(r.Context(), projectID)` before the response.

Keep the existing `len(req.WeakPicks) > 2` validation. Do NOT add a minimum — a student who picks one saves fine and the gate simply has not closed.

- [ ] **Step 5: Run tests**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/
```

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/project_create.go apps/api/internal/api/onboarding_submit.go <test files>
git commit -m "feat(n3d): S0 producers — research_question at creation, weakness_prediction nodes, milestone_plan attest"
```

---

### Task 6: `POST /projects/{id}/framing` (S1)

**Files:**
- Create: `apps/api/internal/api/framing.go`
- Modify: `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/framing_test.go`

**Interfaces:**
- Consumes: `Queries.DeleteStationViewNodes` (Task 2), `a.advanceGates` (Task 4).
- Produces: route `POST /api/v1/projects/{id}/framing`.

- [ ] **Step 1: Write the failing tests**

```go
// One transaction replaces the whole S1 set, so editing a definition never
// leaves a duplicate node behind.
func TestSubmitFraming_ReplacesNodes(t *testing.T) { /* ... */ }

// terms_defined is recorded once three definitions clear 15 runes — and is
// CLEARED again when one is emptied, so the gate never reports work that is no
// longer there.
func TestSubmitFraming_AttestsAndUnattestsTermsDefined(t *testing.T) { /* ... */ }

// A half-finished S1 saves fine (an offer is never a wall) — blank rows are
// dropped, not rejected.
func TestSubmitFraming_SavesPartialWork(t *testing.T) { /* ... */ }
```

Use CJK strings when testing the 15-rune threshold so a byte-vs-rune mistake fails the test (`utf8.RuneCountInString`, not `len`).

- [ ] **Step 2: Run to verify they fail**

- [ ] **Step 3: Implement**

`framing.go`, modelled on `reflection.go` (ownership check → entitlement → decode → validate → write → attest → event → advance):

```go
// submitFraming persists the student's S1 立题 work: her own key-term
// definitions, her candidate core arguments, and where she plans to look for
// evidence. Three graph node types, all author "student" — the platform never
// authors any of it (铁律 1). No model call.
func (a *API) submitFraming(w http.ResponseWriter, r *http.Request) {
	// ... ownership + entitlement, exactly as submitReflection does ...
	var req struct {
		Terms []struct {
			Term       string `json:"term"`
			Definition string `json:"definition"`
		} `json:"terms"`
		Answers    []string `json:"answers"`
		SearchPlan []string `json:"searchPlan"`
	}
	// ... decode ...

	// One transaction: the delete and the inserts must not be separable, or a
	// failed insert leaves her with LESS than she had before pressing save.
	// Scoped by origin (spec §6.1) — these three types have no other producer
	// today, but the marker is uniform across every station-view write.
	types := []string{"term_definition", "provisional_answer", "preregistration"}
	// DeleteStationViewNodes(projectID, types)
	// for each term with non-blank Term: insert term_definition
	//   body {"term", "definition", "origin":"station_view"}
	// for each non-blank answer: insert provisional_answer
	//   body {"text", "origin":"station_view"}
	// if any non-blank searchPlan entry: insert ONE preregistration
	//   body {"directions": [...], "origin":"station_view"}
	// commit

	// terms_defined: 3 definitions at >=15 runes, the binding design's own
	// threshold (dc.html:2153 `ok = t.length>=15`). Cleared when it stops
	// holding — an emptied definition must un-attest, not leave a stale solid.
	// ...
	// append `framing_written` event (best-effort, like reflection.go)
	// a.advanceGates(r.Context(), projectID)
}
```

Register in `api.go` next to the other project routes:

```go
	mux.Handle("POST /api/v1/projects/{id}/framing", protected(a.submitFraming))
```

- [ ] **Step 4: Run tests**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/
```

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/framing.go apps/api/internal/api/framing_test.go apps/api/internal/api/api.go
git commit -m "feat(n3d): POST /projects/{id}/framing — S1 term/answer/preregistration writes"
```

---

### Task 7: `POST /projects/{id}/perspectives` (S2)

**Files:**
- Create: `apps/api/internal/api/perspectives.go`
- Modify: `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/perspectives_test.go`

**Interfaces:**
- Consumes: `Queries.DeleteStationViewNodes` (Task 2), `a.advanceGates` (Task 4).
- Produces: route `POST /api/v1/projects/{id}/perspectives`.

- [ ] **Step 1: Write the failing tests**

```go
// THE test for this task. perspective-matrix (agent/card_effects.go:122) mints
// `perspective` nodes too; a re-save of the S2 view must leave them untouched
// AND they must still count toward node_count_at_least{perspective,2}.
func TestSubmitPerspectives_DoesNotDeleteCardMintedPerspectives(t *testing.T) { /* ... */ }

func TestSubmitPerspectives_ReplacesOwnRows(t *testing.T) { /* ... */ }

// An unknown level is a 400 — the three keys are the binding design's own.
func TestSubmitPerspectives_RejectsUnknownLevel(t *testing.T) { /* ... */ }
```

The first test must seed a card-minted perspective with the real mint shape — body `{"text": ..., "cells": ...}` and **no** `origin` key — not a hand-simplified one. A fixture that carries `origin` would pass while the production shape fails.

- [ ] **Step 2: Run to verify they fail**

- [ ] **Step 3: Implement**

Same shape as Task 6. Level validation against exactly `national` / `global_for` / `global_against`; anything else → `httpx.ErrBadRequest("validation_failed", "视角层级不对", nil)`. Body per row: `{"text", "level", "origin":"station_view"}`, author `"student"`. Appends a `perspectives_written` event, then `a.advanceGates`.

`sources_per_perspective` is NOT touched here — it rides the existing generic attest endpoint (spec §6.2).

- [ ] **Step 4: Run tests**

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/perspectives.go apps/api/internal/api/perspectives_test.go apps/api/internal/api/api.go
git commit -m "feat(n3d): POST /projects/{id}/perspectives — S2 perspective writes, origin-scoped"
```

---

### Task 8: Projection — `framing` + `perspectives`

**Files:**
- Modify: `apps/api/internal/studio/dto.go`
- Modify: `apps/api/internal/studio/projection.go`
- Test: `apps/api/internal/studio/framing_projection_test.go` (new), `apps/api/internal/studio/dto_parity_test.go`

**Interfaces:**
- Consumes: the node types written by Tasks 5–7.
- Produces: `StudioProjection.Framing` / `.Perspectives`, JSON keys `framing` / `perspectives`.

- [ ] **Step 1: Write the failing tests**

```go
// Card-minted perspectives (no origin, no level) project as read-only rows with
// a blank level and still appear in the list — the graph does not care which
// surface asserted them.
func TestProjectPerspectives_MarksCardMintedRowsReadOnly(t *testing.T) { /* ... */ }

// sourcesPerPerspective mirrors the RECORDED gate item, the same way canFinish
// mirrors whole_draft_review.
func TestProjectPerspectives_ReadsRecordedAttestation(t *testing.T) { /* ... */ }

// A node whose body fails to unmarshal is skipped, never fabricated into a
// blank row (projectOnboarding's own defensive rule).
func TestProjectFraming_SkipsUnparseableBodies(t *testing.T) { /* ... */ }
```

Extend `dto_parity_test.go`'s fixture with the two new members so the key-set assertion covers them.

- [ ] **Step 2: Run to verify they fail**

- [ ] **Step 3: Implement the DTOs**

In `apps/api/internal/studio/dto.go`, matching the Zod shapes from Task 1 exactly (the parity test enforces this):

```go
type TermDefinitionDTO struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// FramingDTO projects S1 立题. ResearchQuestion is the research_question node's
// text (minted from the project title at creation), not the project row's title
// — so the banner shows what the graph actually asserts.
type FramingDTO struct {
	ResearchQuestion string              `json:"researchQuestion"`
	Terms            []TermDefinitionDTO `json:"terms"`
	Answers          []string            `json:"answers"`
	SearchPlan       []string            `json:"searchPlan"`
}

// PerspectiveRowDTO is one row of S2's list. Editable is false for rows minted
// by the perspective-matrix card: they carry no level and this view must not
// rewrite them (the S2 write is origin-scoped for exactly this reason).
type PerspectiveRowDTO struct {
	Text     string `json:"text"`
	Level    string `json:"level"`
	Editable bool   `json:"editable"`
}

type PerspectivesDTO struct {
	Rows                  []PerspectiveRowDTO `json:"rows"`
	SourcesPerPerspective bool                `json:"sourcesPerPerspective"`
}
```

Add the two fields to `StudioProjection` next to `Onboarding`.

- [ ] **Step 4: Implement the projectors**

In `projection.go`, next to `projectOnboarding`, following its defensive style (every slice initialised non-nil so the JSON is `[]` not `null`):

```go
// projectFraming reads the frame_question graph nodes for the S1 view.
func projectFraming(d ProjectData) FramingDTO { /* switch on n.Type over d.Nodes */ }

// projectPerspectives reads every `perspective` node for the S2 view. Rows the
// station view wrote (origin=="station_view") are editable and carry a level;
// rows minted by perspective-matrix carry neither and render read-only.
func projectPerspectives(d ProjectData) PerspectivesDTO { /* ... */ }
```

`SourcesPerPerspective` reads `agent.RecordedGatesFromNodes(d.GateStates)["evaluate_perspectives"].Items["sources_per_perspective"] == "solid"`. Note `Project()` already computes `recordedGates` for `canFinish` — reuse that variable rather than recomputing.

Wire both into the `StudioProjection{...}` literal in `Project()`.

- [ ] **Step 5: Run tests**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/studio/
```

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/studio/dto.go apps/api/internal/studio/projection.go apps/api/internal/studio/framing_projection_test.go apps/api/internal/studio/dto_parity_test.go
git commit -m "feat(n3d): projection — framing + perspectives views"
```

---

### Task 9: Web plumbing — clients, state, routing, and the placeholder's removal

**Files:**
- Modify: `apps/web/src/api/projects.ts`
- Modify: `apps/web/src/studio/state.ts`
- Modify: `apps/web/src/studio/views/OnboardingView.tsx`
- Modify: `apps/web/src/studio/ViewFrame.tsx`
- Modify: `apps/web/src/studio/StudioContainer.tsx` (the `toStudioState` projection mapping only)
- Test: `apps/web/src/api/projects.test.ts`

**Interfaces:**
- Produces: `submitFraming(projectId, body)`, `submitPerspectives(projectId, body)`; `StudioState.views.framing` / `.perspectives`.

- [ ] **Step 1: Write the failing test**

In `apps/web/src/api/projects.test.ts`, following the file's existing fetch-mock style, assert both clients POST to the right paths with the JSON body verbatim.

- [ ] **Step 2: Run to verify it fails**

Run from `apps/web`: `npm test -- projects`

- [ ] **Step 3: Add the clients**

```ts
export async function submitFraming(
  projectId: string,
  body: { terms: { term: string; definition: string }[]; answers: string[]; searchPlan: string[] },
): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/framing`, { method: "POST", body: JSON.stringify(body) });
}

export async function submitPerspectives(
  projectId: string,
  body: { perspectives: { text: string; level: string }[] },
): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/perspectives`, { method: "POST", body: JSON.stringify(body) });
}
```

Add both to the `ApiName` union in `StudioContainer.tsx` (line ~18) alongside `"submitOnboarding"`.

- [ ] **Step 4: State + routing**

- `state.ts`: import and re-export `FramingFx` / `PerspectivesFx`, add `framing: FramingFx` and `perspectives: PerspectivesFx` to `StudioState["views"]`, and two callbacks to `StudioCallbacks`:

```ts
  // N3d: S1/S2 station views — each saves its whole panel set explicitly
  // (mirrors OnboardingView's submit shape, not autosave).
  onSubmitFraming?: (body: { terms: { term: string; definition: string }[]; answers: string[]; searchPlan: string[] }) => Promise<void>;
  onSubmitPerspectives?: (body: { perspectives: { text: string; level: string }[] }) => Promise<void>;
```

- `StudioContainer.tsx`'s `toStudioState`: map `p.framing` / `p.perspectives` straight through, exactly as `onboarding: p.onboarding` does.

- `OnboardingView.tsx`: **delete `ShellView` entirely**, along with its 「此环节的深入交互将在后续切片接入」 copy, and simplify the export to render `S0View` unconditionally (S1/S2 no longer route here). Delete the now-inaccurate mention of "the S1/S2 stub branches" from `OnboardingViewProps.onSubmit`'s comment.

- `ViewFrame.tsx`: replace the `isOnboarding` collapse with per-station routing:

```tsx
  // N3d: S0/S1/S2 each render their OWN screen. Before this slice all three
  // collapsed onto OnboardingView, which meant S1 and S2 showed a
  // "coming in a later slice" placeholder — while their gates had no
  // producer at all, so neither station could ever complete.
  const stationScreen = active.code === "S0" || active.code === "S1" || active.code === "S2" ? active.code : null;
  const effectiveView = stationScreen ? "station" : active.view;
```

and three branches rendering `OnboardingView` / `FramingView` / `PerspectivesView` (the latter two land in Tasks 10–11; for this task render `null` placeholders that the next tasks fill, and do not ship this task alone).

- [ ] **Step 5: Run tests**

Run from `apps/web`: `npm test` and `npx tsc --noEmit`

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/api/projects.ts apps/web/src/api/projects.test.ts apps/web/src/studio/state.ts apps/web/src/studio/ViewFrame.tsx apps/web/src/studio/views/OnboardingView.tsx apps/web/src/studio/StudioContainer.tsx
git commit -m "feat(n3d): web plumbing — framing/perspectives clients + per-station routing; delete ShellView placeholder"
```

---

### Task 10: `FramingView` (S1 立题)

**Files:**
- Create: `apps/web/src/studio/views/FramingView.tsx`
- Test: `apps/web/src/studio/views/FramingView.test.tsx`

**Interfaces:**
- Consumes: `FramingFx` (Task 1), `onSubmitFraming` (Task 9).
- Produces: `export function FramingView({ data, onSubmit }: { data: FramingFx; onSubmit?: (...) => Promise<void> })`

Binding design: `docs/design/思维印记_工作区.dc.html:874–928`. Read it before writing; copy, colours, radii, and spacing come from there verbatim.

- [ ] **Step 1: Write the failing tests**

```tsx
// The chip is the design's own rune-count rule (dc.html:2153). CJK must count
// as runes, not bytes — a 15-character Chinese definition is 可检验.
it("shows 待定义 / 偏模糊，再具体点 / ✓ 可检验 by definition length", async () => {});

// She names her own key terms — the design's three are the demo's, not a
// product fixture (spec §5.2).
it("lets her add and remove a key term", async () => {});

it("saves partial work — a single filled term submits fine", async () => {});

// 已定义 N/3 counts only definitions at >=15 runes.
it("reports the gate count off the 15-rune threshold", async () => {});
```

- [ ] **Step 2: Run to verify they fail**

- [ ] **Step 3: Implement**

Structure, top to bottom:

1. The RESEARCH QUESTION banner — read-only, `linear-gradient(135deg,#2A3B7A,#34468C)`, `borderRadius: 16`, the 先定义，再动笔 pill, `data.researchQuestion` at `fontSize: 18, fontWeight: 800, color: "#fff"`.
2. 关键概念 · 我的定义 — the gate chip 「本环节门禁 · 已定义 {n}/3」, the sub-line 「自己写清每个关键词——别让读者把「可持续」误当成「变绿」。印记只判断你写得够不够可检验，不替你写。」, then one row per term: term input + status chip + definition `<textarea rows={2}>`, plus a 「添加一个关键词」 control and a per-row remove.
3. A two-column grid (`gridTemplateColumns: "1fr 1fr", gap: 14`):
   - 可能的核心论点 — sub-line 「你自己拟——之后在 S4 逐条验证。」, add/remove list of `<textarea rows={2}>`.
   - 打算去哪找证据 — the same add/remove treatment, rendering each entry as the design's chip (`color:#5B6BB5; background:#EBEDFA; padding:6px 11px; borderRadius:9px`) with an inline editor.
4. One 记下我的立题 save button, right-aligned, styled exactly like `S0View`'s 记下我的理解.

Status chip helper:

```tsx
// The binding design's own rule (dc.html:2153): rune count on the trimmed
// definition. [...s] not s.length — a Chinese definition must not be judged by
// its byte or UTF-16 length.
function defChip(def: string) {
  const n = [...def.trim()].length;
  if (n === 0) return { label: "待定义", color: "#9AA1B0", background: "#F1F2F5" };
  if (n < 15) return { label: "偏模糊，再具体点", color: "#B8892F", background: "#FBF4E2" };
  return { label: "✓ 可检验", color: "#4C9A82", background: "#E7F3EE" };
}
```

No celebratory copy anywhere when the count reaches 3 (铁律 2). The save button is never disabled for incompleteness — only while a submit is in flight.

- [ ] **Step 4: Run tests + wire into `ViewFrame`**

Replace Task 9's S1 placeholder branch with `<FramingView data={state.views.framing} onSubmit={onSubmitFraming} />` and thread the prop through `ViewFrameProps`.

Run from `apps/web`: `npm test` and `npx tsc --noEmit`

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/views/FramingView.tsx apps/web/src/studio/views/FramingView.test.tsx apps/web/src/studio/ViewFrame.tsx
git commit -m "feat(n3d): S1 立题 view — her own key terms, candidate arguments, search plan"
```

---

### Task 11: `PerspectivesView` (S2 视角与素材)

**Files:**
- Create: `apps/web/src/studio/views/PerspectivesView.tsx`
- Test: `apps/web/src/studio/views/PerspectivesView.test.tsx`

**Interfaces:**
- Consumes: `PerspectivesFx` (Task 1), `onSubmitPerspectives` (Task 9), the existing `AddSourceForm` and `MaterialSource[]`, and the existing attest client for `sources_per_perspective`.

Binding design: `docs/design/思维印记_工作区.dc.html:930–958`.

- [ ] **Step 1: Write the failing tests**

```tsx
// dc.html:2160 — national AND at least one non-national.
it("shows the coverage chip only once both layers are covered", async () => {});

// Card-minted rows are hers to keep, not to edit here.
it("renders a card-minted row read-only with no level chips", async () => {});

// The one explicit attestation in the slice — and it must say WHY when it
// can't be used yet, rather than silently vanishing (an offer is never a wall).
it("disables the sources confirm with a reason until 2 perspectives and 1 source exist", async () => {});

it("lets her add, level, and remove a perspective", async () => {});
```

- [ ] **Step 2: Run to verify they fail**

- [ ] **Step 3: Implement**

1. Header row: 「先摆出不同视角，再去找素材」 plus the coverage chip 「✓ 已覆盖 本地/国家 + 全球」 (`color:#4C9A82; background:#E7F3EE`), shown only when covered.
2. Sub-line, verbatim: 「每条视角都由你自己写、自己标层级。0457 要求至少覆盖 本地/国家 与 全球 两层。印记只追问你的检索方向，不替你找来源。」
3. One card per row (`background:#fff; border:1px solid #EAECF2; borderRadius:14px; padding:15px 17px; marginBottom:12px`): the three level chips + a remove control + a `<textarea rows={2}>` with placeholder 「写一条视角：谁、从什么立场、看到什么……」.

```tsx
// The binding design's own three levels and colours (dc.html:2158).
const LEVELS = [
  { key: "national", label: "国家视角", color: "#4C9A82" },
  { key: "global_for", label: "全球视角 · 支持", color: "#2A3B7A" },
  { key: "global_against", label: "全球视角 · 反方", color: "#C96F4F" },
] as const;
```

A row with `editable === false` renders its text read-only with a 「来自工具卡」 tag and no level chips — it has no level, and rewriting it here would fight the card that minted it.
4. 「添加一条视角」 control, then the save button.
5. Below the list: the 素材 block — spec §6.3's deliberate departure. A short heading, the project's material titles as a plain list, and the existing `AddSourceForm` wired to the existing `material.onAdd`. Comment it:

```tsx
// Spec §6.3: the binding design's S2 has no source affordance at all, yet the
// station is 视角与素材 and its own gate demands sources per perspective — as
// drawn the gate is unreachable. This reuses 6b's ingestion path (RL-2: S2
// never ingests on its own). The full dossier stays at S3.
```

6. The one confirm: a checkbox-style control reading 「每条视角我都找到了至少一条素材」, checked from `data.sourcesPerPerspective`, calling the attest client with `{item: "sources_per_perspective", confirmed}` against `evaluate_perspectives`. When fewer than 2 perspectives or 0 materials exist it renders disabled **with the reason visible**, never hidden.

- [ ] **Step 4: Run tests + wire into `ViewFrame`**

Replace Task 9's S2 placeholder branch, threading `material` (already a `ViewFrameProps` member) plus the new callbacks.

Run from `apps/web`: `npm test` and `npx tsc --noEmit`

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/views/PerspectivesView.tsx apps/web/src/studio/views/PerspectivesView.test.tsx apps/web/src/studio/ViewFrame.tsx
git commit -m "feat(n3d): S2 视角与素材 view — perspectives, levels, source intake, one attestation"
```

---

### Task 12: `StudioContainer` wiring

**Files:**
- Modify: `apps/web/src/studio/StudioContainer.tsx`
- Test: `apps/web/src/studio/StudioContainer.test.tsx`

**Interfaces:**
- Consumes: everything from Tasks 9–11.

- [ ] **Step 1: Write the failing tests**

Follow this file's existing conventions exactly — its own `flush()` helper after every state-changing interaction, and wait on the control being asserted rather than on a label from a different projection slice. (Three tests in N3c failed ~50% of runs from precisely that mistake.)

```tsx
// Both submits refetch, so the station rail reflects a gate that just closed.
it("refetches the projection after saving S1", async () => {});
it("refetches the projection after saving S2", async () => {});

// The one attestation goes through the existing generic gate endpoint.
it("attests sources_per_perspective against evaluate_perspectives", async () => {});
```

- [ ] **Step 2: Run to verify they fail**

- [ ] **Step 3: Implement**

Add `onSubmitFraming` / `onSubmitPerspectives` handlers next to the existing `onSubmitOnboarding`, each `await`ing its client then `await refetchProject()` — the same shape and the same error handling the neighbouring submit handlers already use. Thread the `sources_per_perspective` attest through the existing `attestGate` client with contract `evaluate_perspectives`.

- [ ] **Step 4: Run tests**

Run from `apps/web`: `npm test` and `npx tsc --noEmit`. Run the suite **three times** and confirm the new tests pass on all three before reporting.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/StudioContainer.test.tsx
git commit -m "feat(n3d): wire S1/S2 submits + sources attestation through StudioContainer"
```

---

### Task 13: The walkability test + docs

**Files:**
- Test: `apps/api/internal/api/walkable_stations_test.go` (new)
- Modify: `docs/2026-07-20-student-platform-remaining-work.md`
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md`

- [ ] **Step 1: Write the acceptance test**

Spec §10.1 — the test that could not have passed before this slice at all:

```go
// A project created through the funnel walks S0 -> S1 -> S2 -> S3 with NO
// hand-written database rows. Before N3d this was impossible twice over:
// nothing in production called Advance, and S0/S1/S2 between them had six gate
// items with no producer. The seeded demo project only appeared walkable
// because migration 0018 hand-writes confirmed_solid for S0-S3.
func TestWalkableStations_S0ToS3(t *testing.T) {
	// POST /projects            -> research_question exists, S0 is current
	// POST /onboarding (2 picks) -> decode_task solid, S1 current
	// POST /framing (3 terms >=15 runes, 1 answer, 1 search direction)
	//                            -> frame_question solid, S2 current
	// POST /materials + /materials/{mid}/open  -> recon_logged recorded
	// POST /perspectives (2 rows, both layers)
	// POST /gate/evaluate_perspectives/attest {sources_per_perspective}
	//                            -> evaluate_perspectives solid, S3 current
	// Assert on the PROJECTION's station states at each step, not on gate
	// internals — the rail is what the student actually sees.
}
```

- [ ] **Step 2: Run it**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/
```

- [ ] **Step 3: Update the trackers**

In `docs/2026-07-20-student-platform-remaining-work.md`: add an N3d section in the same style as the N3c one above it, mark the N3 summary row, and **move the two deferred items into a new N3e row** (search-plan card; R-9 framework reveal) rather than leaving them implicit. Also record what §1 of the spec found — that `Advance` had no caller and the seed masked it — since that is the kind of finding a future reader needs.

In `docs/2026-07-11-whole-product-refactor-roadmap.md`: mark the Slice 6/6b/6c carry-forwards this slice closed (the S2 perspective view) and note the two that moved to N3e.

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/api/walkable_stations_test.go docs/2026-07-20-student-platform-remaining-work.md docs/2026-07-11-whole-product-refactor-roadmap.md
git commit -m "test(n3d): S0->S3 walkability acceptance + tracker updates"
```

---

## Closing note for the executor

Two things this plan cares about more than speed:

**The `origin:"station_view"` marker.** Task 7's first test is the one that matters — a fixture that hand-adds `origin` to a card-minted perspective would pass while production silently deleted the student's tool-card work. Seed that fixture with the real shape from `agent/card_effects.go:122`.

**Test fixtures must be reachable states.** Test-mock infidelity is the documented root cause of five Criticals in an earlier slice of this project: mocks encoded shapes the backend cannot produce, so a green suite confirmed a belief instead of testing the code. If a fixture in this slice cannot be produced by some real code path, it is wrong.

Where a task's test bodies are sketched rather than written out, the sketch's **intent** governs — write the test that actually proves the stated behaviour against real fixtures, not the shortest thing matching the comment.

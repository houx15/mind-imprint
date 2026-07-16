# Slice 10 — the isolated assessor + growth report (成长报告) · design

> Whole-product refactor #2. Builds on Slices 0–9. Wires the fourth subagent —
> the **assessor** — live for the first time: a few-shot MVP engine that reads a
> projection of one project's **event stream** and scores it against the seeded
> **CT rubric** (per-dimension L1–L4 + evidence + a growth narrative), landing in
> the 成长报告 slot. The assessment moat, in its minimal real form.

## 1. Context — what exists, what this slice changes

- The **成长报告 tab** (`apps/web/src/shell/growth/GrowthPlaceholder.tsx`, left-rail
  `growth` pillar) renders a static "成长报告正在重建" placeholder — no data, no
  fetch. Its own header comment says "the project-backed report lands in Slice 9
  (评估 view) + Slice 10 (growth report)." This slice is that report.
- The **assessor is greenfield.** Of the four subagents in agent-spec §4.5
  (classifier · coach · planner · **assessor, flagship, isolated**), only the first
  three have code (`classifier.go`, `coach.go`, `planner.go`). `rg assessor
  apps/api` is empty.
- The **9+1-dim CT rubric is fully seeded but TS-only.** `packages/contracts/src/
  rubric.ts` holds `FULL_RUBRIC` (D1–D10, each with real L1–L4 SOLO behavior-ladder
  anchors), `CT_RUBRIC`, `SoloLevel`/`ScoredLevel`, `SOLO_LABELS`,
  `assertRubricComplete`. It is wired to **no Go engine**. (The Go
  `skills.ReviewCriterion` — 0457 表D/E/F/H — is a *different* thing: the whole-draft
  review, not the CT rubric.)
- The **event stream exists but is never read.** Slice 0's append-only `event`
  table (`0016`) gets writes (`gate_attempt`, `card_surfaced`, …) but nothing
  consumes it; `studio.ProjectData` does not even load it. The queries file has
  `AppendEvent` + `ListEventsByProject` (the read query exists, unused).
- The **`evaluations` table is dormant** (task-scoped, retired with the task model
  in Slice 5d). The roadmap explicitly earmarks `evaluations.project_id` as "Slice
  10's write" and deliberately did **not** drop the table.
- The **自发/提示后 independence signal is live**: `studio.projectEquipment` tags
  each card use `Spont: "自发"|"提示后"`. This slice's assessment input reuses it.
- **Prose-only, no code:** the anchor samples (Phoebe/Marcus/Ethan/Eliza), the T1–T7
  thinking leaps, the depth-vs-independence radar. This slice authors a *minimal*
  anchor set; leaps + radar are deferred.

## 2. Scope & locked decisions

Locked in brainstorm (all three the recommended path):

- **DEC-10.1 — Rubric + narrative only.** The 成长报告 renders per-dimension L1–L4
  (+ an evidence quote) over the full seeded 10-dim CT rubric, plus one growth
  narrative. The **depth-vs-independence radar** and the **T1–T7 thinking-leap
  detection** are deferred — they are derived views that compose on this same data.
- **DEC-10.2 — Event stream, projected.** The assessor reads a projection of this
  project's **append-only event stream** (the temporal process record), not just the
  current-state studio snapshot. This lands the written-but-never-read event
  plumbing (`ProjectData` gains `Events`) that Slice 10 owes anyway.
- **DEC-10.3 — Minimal anchor set now.** Author 1–2 compact, fully-annotated anchor
  dialogues (a partial Phoebe L1→L4 arc) as `go:embed`ed few-shot fixtures — enough
  to pin the engine's output format + level calibration. The full
  Phoebe/Marcus/Ethan/Eliza golden set + the 100–300-dialogue benchmark (spec §5
  Validation stage) are the deferred validation workstream.

**Defaults confirmed** (not separately questioned, but binding): the full 10-dim
rubric is scored (`NA` allowed where a writing project gives thin evidence — the
engine must be rubric-general, that is the moat); persistence reuses the dormant
`evaluations` table via an additive migration; assessment is **student-triggered**
and inline (not async); the assessor is **isolated** — its own endpoint, never in
the coach turn-loop.

**RL-5 (the assessment is diagnostic evidence, never a grade or verdict)** governs
throughout. This is the one place the system renders a judgment — sanctioned by the
acceptance aorta («评估那一刀跑出 rubric 评级……以你的思维印记呈现») and assessment-spec
§1 principle 2. The invariant: every level carries its behavioral evidence, the
report is framed for growth, and there is **no number-grade, no rank, no overall
score** — «the system scores from behavioral signals; the teacher makes the final
call».

## 3. The rubric goes single-source (contracts + Go)

The CT rubric must exist Go-side to feed the assessor's prompt. Per the AGENTS.md
single-source law (cards are one JSON: TS build-time import + Go `go:embed` the same
files), the seeded rubric is extracted to a canonical JSON, not duplicated in Go.

### 3.1 Canonical JSON (`packages/contracts/rubric/ct-rubric.json`)

The current `FULL_RUBRIC` array (10 dimensions × `{id, name, framework, anchors:{L1,
L2, L3, L4}}`) is moved verbatim into `ct-rubric.json`. Shape:

```json
{
  "id": "ct",
  "name": "AI 批判思维（9+1 维）",
  "dimensions": [
    { "id": "D1", "name": "提问清晰度", "framework": "ATL Thinking · QUEST-Q",
      "anchors": { "L1": "…", "L2": "…", "L3": "…", "L4": "…" } },
    "…D2…D10…"
  ]
}
```

### 3.2 `rubric.ts` imports + validates it

`rubric.ts` keeps its Zod schemas (`RubricDimension`, `Rubric`, `SoloLevel`,
`ScoredLevel`, `SOLO_LABELS`, `assertRubricComplete`) and its exported constants,
but `FULL_RUBRIC`/`CT_RUBRIC` are now **parsed from the JSON** at module load:

```ts
import ctRubricJson from "../rubric/ct-rubric.json";
export const CT_RUBRIC: Rubric = Rubric.parse(ctRubricJson);
export const FULL_RUBRIC: RubricDimension[] = CT_RUBRIC.dimensions;
assertRubricComplete(CT_RUBRIC); // blank-anchor guard preserved
```

The existing `rubric.test.ts` (10 dims D1–D10 in order, all anchors non-empty) must
stay green unchanged — proving the extraction is byte-faithful.

### 3.3 Go embeds the synced copy

Go gets a `rubric` package (`apps/api/internal/rubric/`) that `go:embed`s a synced
copy of `ct-rubric.json` and unmarshals it into typed structs:

```go
type Dimension struct {
    ID        string            `json:"id"`
    Name      string            `json:"name"`
    Framework string            `json:"framework"`
    Anchors   map[string]string `json:"anchors"` // L1..L4
}
type Rubric struct {
    ID         string      `json:"id"`
    Name       string      `json:"name"`
    Dimensions []Dimension `json:"dimensions"`
}
func CT() Rubric // parsed once from the embedded JSON
```

The sync follows the skills precedent: canonical file in `packages/contracts/`,
mirrored into the Go tree, kept in step by an extended `make sync-skills` (or a
sibling `sync-rubric`) target. A Go test asserts `len(CT().Dimensions) == 10` and
every dimension has non-empty L1–L4 anchors — the Go-side `assertRubricComplete`.

## 4. The substrate — assessment projection over the event stream

### 4.1 `ProjectData` loads events

`studio.ProjectData` gains `Events []Event` (an ordered projection of
`ListEventsByProject`), loaded in `studio/load.go`. `Event` is a plain view struct:

```go
type Event struct {
    Type      string          // gate_attempt, card_surfaced, …
    Surface   string          // studio | course | chat
    Payload   json.RawMessage // opaque; the builder reads known keys defensively
    CreatedAt time.Time
}
```

This is the written-but-never-read stream finally consumed. `Project()` does **not**
need events (the studio views don't use them); the field is loaded for the
assessment path and is harmless to the existing projectors.

### 4.2 `BuildAssessmentInput` — pure digest

A pure function digests the process record into a compact, model-ready structure.
Following `ProposeReview`'s house style (the engine takes *primitives*, not a studio
struct), the builder lives in the `agent` package and takes the primitive process
pieces the handler extracts from `ProjectData`; it is unit-tested with literal
inputs (no Docker), exactly like the projection tests.

```go
type CardUse struct {
    CardID    string // craap, sift, concession, …
    Dimension string // the CT dimension it exercises, when known
    Spont     string // 自发 | 提示后   (reused from projectEquipment's signal)
}
type DispositionUse struct {
    Kind   string // accept | rewrite | reject   (feedback-comprehension signal)
    Reason string
}
type AssessmentInput struct {
    CardUses      []CardUse
    Dispositions  []DispositionUse
    GateProgress  []string // e.g. "S3 evaluate: solid", "S5 draft_polish: owed"
    SnapshotCount int
    WordCounts    []int
    ReviewBands   []string // latest board-voice review per table: "表D 熟练", …
    GraphSummary  string   // claims / evidence / concession nodes; student-written warrants
    Timeline      []string // ordered one-line event digests (unprompted-first is visible here)
}
func BuildAssessmentInput(
    events []studio.Event,
    cards []CardUseSource, dispositions []DispositionSource,
    gates []GateSource, snapshots []SnapshotSource,
    reviews []ReviewItem, graphSummary string,
) AssessmentInput
```

(The exact `*Source` shapes are pinned in the plan; each is a thin projection of an
already-loaded `ProjectData` slice. The point held here: the digest is **pure,
testable, and temporal** — the `Timeline` preserves order so unprompted-first
behavior is legible to the model.)

**Isolation note.** If placing `BuildAssessmentInput` in `agent` would import
`studio` and create a cycle (`studio` must not import `agent`), the plan keeps the
input primitives free of `studio` types — the handler maps `ProjectData` → agent
primitives at the call site. The one `studio` dependency (`Event`) may be mirrored
as an `agent`-local struct if needed. This is a plan-level packaging decision; the
engine's *contract* (`AssessmentInput` → `Assessment`) is fixed here.

## 5. The engine (`apps/api/internal/agent/assess.go`)

One **flagship** call (never downgraded — the eval-routing law), **isolated** from
the coach loop.

```go
type DimensionScore struct {
    Code     string `json:"code"`     // D1..D10
    Name     string `json:"name"`     // 提问清晰度 …
    Level    string `json:"level"`    // L1 | L2 | L3 | L4 | NA
    Evidence string `json:"evidence"` // the behavioral evidence for this level
}
type Assessment struct {
    Dimensions []DimensionScore `json:"dimensions"`
    Narrative  string           `json:"narrative"` // the growth narrative
}
func Assess(
    ctx context.Context, prov gateway.Provider, r gateway.Resolved,
    rb rubric.Rubric, input AssessmentInput, anchors []AnchorSample,
) (Assessment, gateway.ChatUsage, error)
```

- **Prompt** = system posture (the assessor's own posture: diagnose the *thinking
  path*, RL-5 — evidence-anchored levels, no grade) + the rubric's dimension ladders
  (L1–L4 anchors per dimension, from `rubric.CT()`) + the anchor few-shot samples +
  the `AssessmentInput` digest. The model returns strict JSON: one entry per rubric
  dimension (`level` + `evidence`) and one `narrative`.
- **Level validation:** each returned `level` is validated against the SoloLevel set
  (`L1..L4|NA`); an unknown value coerces to `NA` (never crashes, never invents a
  level). Every rubric dimension is represented in the output (missing → `NA`),
  scored in the rubric's declared order.
- **Enforcement:** the `narrative` and every `evidence` field pass banned-phrasing
  (the ghostwriting/`rewritten-sentence-zh` guards); **any** match rejects the WHOLE
  assessment (all-or-nothing, like the review). RL-1 is structurally N/A — `Assess`
  returns a value the handler writes to `evaluations`, never to the draft. Output-check
  is N/A (multi-field, no single topic — the review-parity narrowing).
- **Cost** is recorded (a `ChatUsage` returned; the handler logs the `llm_call`).

### 5.1 Anchor few-shot (`apps/api/internal/agent/anchors.json`, `go:embed`)

```go
type AnchorSample struct {
    Name        string           `json:"name"`   // "Phoebe (partial)"
    Digest      string           `json:"digest"` // a compact process-record excerpt
    Dimensions  []DimensionScore `json:"dimensions"`
    Narrative   string           `json:"narrative"`
}
```

Author **1–2** compact samples (a partial Phoebe L1→L4 arc) — each a short digest +
its per-dimension level/evidence + narrative, in the exact output shape. Backend-only
(prompt priming); contracts do not need them.

## 6. Persistence — reuse `evaluations` (migration `0017`)

Additive migration `apps/api/internal/store/migrations/0017_assessment_project.sql`:

```sql
ALTER TABLE evaluations ADD COLUMN project_id uuid REFERENCES project(id);
ALTER TABLE evaluations ALTER COLUMN task_id DROP NOT NULL;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (task_id IS NOT NULL OR project_id IS NOT NULL);
CREATE INDEX evaluations_project_idx ON evaluations (project_id, created_at DESC);
```

Legacy task-scoped rows are untouched (`project_id` NULL, `task_id` set). New
project-scoped rows set `project_id`, leave `task_id` NULL. The existing `scores
jsonb` holds the `[]DimensionScore`; `narrative` holds the growth narrative; the
`model`/`tier`/`prompt_tokens`/`completion_tokens`/`cost_estimate` columns already
exist (exact names from `0001_init.sql`); `status` is set `'done'` inline (async is
deferred).

New sqlc queries (`apps/api/internal/store/queries/evaluation.sql`):

```sql
-- name: InsertProjectEvaluation :one
INSERT INTO evaluations (project_id, scores, narrative, model, tier,
  prompt_tokens, completion_tokens, cost_estimate, status)
VALUES (@project_id, @scores, @narrative, @model, @tier,
  @prompt_tokens, @completion_tokens, @cost_estimate, 'done')
RETURNING *;

-- name: GetLatestProjectEvaluation :one
SELECT * FROM evaluations
WHERE project_id = @project_id
ORDER BY created_at DESC
LIMIT 1;
```

`make sqlc` regenerates. Column names are verbatim from `0001_init.sql`'s
`evaluations` table (`prompt_tokens`, `completion_tokens`, `cost_estimate`).

## 7. Wire → contract → client

### 7.1 Endpoints (isolated, project-scoped)

- `GET /projects/{id}/assessment` — returns the latest stored assessment as
  `AssessmentDTO`, or an empty/`null` body when none exists. **No model call** — the
  slot's read path. Project-scoped via ownership check.
- `POST /projects/{id}/assessment` — generate (or regenerate): load `ProjectData` +
  events → `BuildAssessmentInput` → `Assess` (flagship) → enforce → persist
  `InsertProjectEvaluation` → return the new `AssessmentDTO`. **Entitlement-gated**
  before the model call (`HasEntitlement`), cost recorded even on enforcement
  rejection, ownership-checked. Not SSE for the keystone (a single JSON response;
  streaming is a later polish).

### 7.2 DTO ↔ Zod (parity-guarded)

```go
type AssessmentDimensionDTO struct {
    Code string `json:"code"`; Name string `json:"name"`
    Level string `json:"level"`; Evidence string `json:"evidence"`
}
type AssessmentDTO struct {
    Dimensions []AssessmentDimensionDTO `json:"dimensions"`
    Narrative  string                   `json:"narrative"`
    GeneratedAt string                  `json:"generatedAt"` // RFC3339
}
```

```ts
export const DimensionScore = z.object({
  code: z.string(), name: z.string(),
  level: SoloLevel,              // reuse rubric.ts's enum (L1..L4|NA)
  evidence: z.string(),
});
export const Assessment = z.object({
  dimensions: z.array(DimensionScore),
  narrative: z.string(),
  generatedAt: z.string(),
});
```

Go↔Zod key parity guarded by a parity test (the `dto_parity_test` precedent), added
for `AssessmentDTO`/`AssessmentDimensionDTO`. This DTO is its **own** surface — **not**
folded into `StudioProjection` (the assessor is isolated).

### 7.3 Client — `GrowthReport`

- `apps/web/src/api/assessment.ts`: `getAssessment(projectId)` (GET) +
  `generateAssessment(projectId)` (POST), returning the Zod-parsed `Assessment`.
- `apps/web/src/shell/growth/GrowthReport.tsx` replaces `GrowthPlaceholder`:
  - resolves the student's **active project** (the same project the Studio shows;
    multi-project aggregation deferred),
  - **read path:** GET on mount; renders per-dimension rows — the CT dimension name,
    a **level chip** (L1–L4 with `SOLO_LABELS`, or a muted NA), and the evidence
    quote — followed by the growth narrative.
  - **empty state:** honest "还没有成长报告 — 完成一些思考后生成" + the button. No fake
    data.
  - a **生成成长报告 / 重新生成** button → POST → re-render. Disabled/spinner while
    generating.
  - **RL-5 in the UI:** level chips are diagnostic (SOLO label + evidence on the
    row), never a total/grade/rank; copy frames it as 你的思维印记 growth, not a score.
- The old `GrowthPlaceholder.tsx`/`.test.tsx` are removed; `StudentApp` mounts
  `GrowthReport` on the `growth` tab.

## 8. Testing

- **Contracts:** `ct-rubric.json` parses via `Rubric.parse`; `rubric.test.ts` stays
  green unchanged (extraction is byte-faithful); `Assessment`/`DimensionScore` parse;
  `level` rejects a non-SoloLevel value; Go/Zod key parity.
- **Go — rubric:** `rubric.CT()` has 10 dimensions D1–D10 in order; every dimension
  has non-empty L1–L4 anchors (Go-side completeness guard); embedded JSON matches the
  canonical copy (sync-drift guard).
- **Go — agent (Assess):** the prompt carries the rubric ladders + anchor few-shot +
  the input digest; the model JSON parses into per-dimension scores + narrative in
  rubric order; an unknown `level` coerces to `NA`; a missing dimension → `NA`; a
  banned-phrasing narrative rejects the whole assessment (all-or-nothing, proven
  RED→GREEN); an idempotent GET returns the stored row with no `llm_call` (proven by
  an `llm_call`-count assertion).
- **Go — agent (BuildAssessmentInput):** a literal process record (card uses w/
  自发/提示后 + dimension, dispositions w/ reasons, gate progress, snapshot counts,
  review bands, graph summary) → the expected structured input; `Timeline` preserves
  event order; an empty project → a minimal, non-crashing input.
- **Go — store:** migration `0017` adds `project_id`, makes `task_id` nullable, and
  the CHECK holds; a legacy task-scoped insert still works; a project-scoped insert +
  `GetLatestProjectEvaluation` roundtrips (Docker-backed); the scope CHECK rejects a
  row with neither id.
- **Go — parity:** `AssessmentDTO`/`AssessmentDimensionDTO` key parity.
- **Web:** `GrowthReport` renders dimension rows (name + level chip + evidence) +
  narrative from state; the empty state (no assessment) shows the prompt + button and
  no fake data; the generate button calls the POST client; `getAssessment` maps the
  DTO.

## 9. Non-goals / carry-forwards

- **The depth-vs-independence radar + T1–T7 thinking leaps** — derived views
  composing on this slice's rubric scores + the 自发/提示后 signal; a later 评估/report
  slice.
- **The teacher & parent report versions** (class heatmap, risk column, individual
  trajectory, single-conversation replay) → the teacher-dashboard slice (13), gated
  on the teacher-side design.
- **The full anchor golden set** (Phoebe/Marcus/Ethan/Eliza in full) + the 100–300
  human-annotated benchmark + the fine-tune stage → the standing validation
  workstream (assessment-spec §5).
- **The OPCVL rubric** (HS-D1…D12) — a separate rubric on the same engine, its own
  module.
- **Chat/Course surface aggregation** — the assessment input reads one project's
  studio-surface events; cross-surface aggregation is deferred (the `Surface` field
  is carried but the keystone assesses the studio surface).
- **Async assessment** (river worker + polling) — the keystone is inline; the old
  P4 async-eval design lands later. `status` is set `'done'` synchronously.
- **Multi-project growth** — the slot resolves the active project; aggregating a
  student's growth across projects over 14 weeks is deferred.
- **Feedback-comprehension as a formal dimension** (assessment-spec §9.1 open Q2) —
  the accept/reject/rewrite signal is *fed to the assessor* here via
  `DispositionUse`, but whether it becomes a tenth-and-a-half CT dimension or folds
  into D6 stays an open product decision, not resolved by this slice.

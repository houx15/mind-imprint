# Slice 5b — Wire the Studio Live (read path) · Design

> Second UI slice; the first that talks to the backend. Part of the whole-product
> refactor #2 (see `docs/2026-07-11-whole-product-refactor-roadmap.md`,
> `docs/2026-07-11-agent-spec.md`). Follows Slice 5a (`…-slice-5a-studio-shell-design.md`),
> which shipped the fixture-backed Studio shell.

## Goal

`GET /api/v1/projects/{id}` returns a real `StudioState`, projected **server-side**
from a seeded Project, and the live Studio renders the Station rail, Coach rail,
素材 (material), and S0 onboarding from that data — no fixture. This is the **read
path** only: the interactive turn loop, disposition persistence, and the routing
cutover are later slices (5c/5d).

## Scope

**In:**
1. A shared `StudioProjection` Zod contract (`packages/contracts`) — the lean wire DTO (project/stations/coach/onboarding) + shared view types — with a Go DTO that marshals to the same shape (Go↔Zod parity).
2. A Go read-projection package `apps/api/internal/studio` that turns a Project + its graph/gates/plan/interventions/cards into a `StudioProjection` DTO, using the `skills` DAG + `agent.ReconcileGates` + `TopoOrder`.
3. Two protected, ownership-checked endpoints: `GET /projects` (list) and `GET /projects/{id}` (the `StudioProjection`).
4. A seed migration minting a demo Task + Project (Phoebe, 0457 中国可持续, station S4) whose projection ≈ `STUDIO_FIXTURE`.
5. Frontend: `studio/state.ts` consumes the shared contract view types; a `projects` API client; a live `StudioContainer` reachable via `?studio` in `Root.tsx`; the carried-forward `CoachMessage.ai` `anchor` field + CoachRail render.

**5b is additive** — a new contract, a new `internal/studio` package, new
endpoints, a new seed migration, and new frontend. It changes **no** existing Go
type, migration, or legacy behavior. The only touch to existing code is two
behavior-preserving refactors in the `agent` package: extract pure exported
helpers `GraphViewFromRows` and `RecordedGatesFromNodes` (so the projection can
build a `GraphView` + recorded-gate map from already-loaded rows without a DB) and
have the existing `LoadGraph`/`ListGateStates` methods call them. Covered by
existing agent tests.

**Out (later slices):**
- Live coach/turn loop, `RunAgentStep` HTTP driver, check_gate debounce, composer-send, disposition **persistence** → **5c**.
- **The Slice-3 deferred debt → 5c** (write-path): making `task_id` NULLABLE on `material`/`card_instances`/`evaluations` (it regenerates their sqlc `TaskID` to `pgtype.UUID`, rippling every legacy caller) and project-scoping `GetCardInstance` (it threads `projectID` through the `CompleteCard` seam). Neither is exercised by the read path — equipment uses the already-project-scoped `ListCardInstancesByProject` — so both land with the write path that actually mints project-native rows. The 5b seed satisfies the existing `NOT NULL` FK by creating a demo Task.
- App-routing flip (Studio as the real student surface) + retiring the old chat `workspace/` + legacy `RunTurn`/`turn.go` → **5d**.
- Live projection of the 素材/结构/写作/评估 center-pane views → **Slices 6/7/8/9**.

## Global Constraints (verbatim, bind every task)

- **Client never calls the model/Volcano directly.** All model/audio via the Go gateway; keys only server-side. (Not exercised this slice — read path — but the boundary holds: the frontend only calls the new REST endpoints.)
- **Single source of truth.** The skill contract DAG, gate logic (`ReconcileGates`), and `TopoOrder` live in Go and are **not** duplicated in TS. The projection is server-side; the frontend renders the DTO.
- **Binding Chinese design copy is verbatim** (design `docs/design/思维印记_工作区.dc.html`). Never alter design copy to satisfy a test — fix the test matcher.
- **Icons = inline SVG, never `lucide-react`.**
- **Studio discipline (from 5a/Slice-1):** do not mutate the old chat `workspace/`, `StudentApp`, `AppShell` routing, or `Root`'s existing branches. `?studio` is **additive** alongside `?demo`. The routing flip is 5d.
- **Purely additive; build stays green.** No existing Go type, migration, or legacy code path changes. The seed satisfies the existing `NOT NULL` `task_id` FK with a demo Task; the Slice-3 debt (nullable `task_id`, project-scoped `GetCardInstance`) is deferred to 5c.
- **Docs in English** except literal Chinese UI copy.
- **Fixed-UUID seed pattern** (per `0002_seed.sql`): the demo Task/Project and its nodes use deterministic UUIDs referencing Phoebe (`00000000-0000-0000-0000-000000000003`).

## Architecture

```
GET /api/v1/projects/{id}
  → api.getProject (ownership: project.user_id == caller, else 404)
      → studio.Project(ctx, deps, projectID)                     [internal/studio]
          load: GetProject, ListGraphNodesByProject, ListGraphEdgesByProject,
                ListGateStateNodes, GetPlanNode, ListInterventionsByProject,
                ListCardInstancesByProject
          skill: skills.ByID("writing-project")
          compute: agent.ReconcileGates(skill, graphView, recordedGates)
                   skill.TopoOrder()
          build:  StudioProjection DTO ── json ──▶ StudioProjection (Zod)
  → StudioContainer (?studio): map StudioProjection → StudioState → <StudioShell state callbacks/>
```

The projection is a **pure read**: no writes, no model calls, no loop. `focusMode`
and `activeStation`-after-select are client-local; the server sends their initial
values.

### The contract-DAG ⇄ station mapping (established, verified against the fixture)

`writing-project.json`'s 7 contracts, in `TopoOrder()`, map 1:1 onto S0–S6:

| idx | contract | view (in JSON) | station name (new `title`) |
|----:|----------|----------------|----------------------------|
| 0 | `decode_task` | 评估 | 任务解码 |
| 1 | `frame_question` | 结构 | 立题 |
| 2 | `evaluate_perspectives` | 素材 | 视角与素材 |
| 3 | `evaluate_sources` | 素材 | 信源评估 |
| 4 | `build_argument` | 结构 | 论证构建 |
| 5 | `draft_polish` | 写作 | 成稿打磨 |
| 6 | `reflect_archive` | 评估 | 反思归档 |

- **code** = `"S" + topoIndex`.
- **name** = the contract's new `title` field.
- **view** = the contract's existing `view` field.

## 1. Shared contract: `StudioProjection` (Zod) + Go DTO parity

The wire DTO is **not** the whole frontend `StudioState` — it is a lean
`StudioProjection` carrying only the parts 5b projects live (the rich
center-pane view types stay in the frontend). New file
`packages/contracts/src/studioState.ts` defines Zod for the shared **view types**
(`StationCode`, `StationView`, `StationState`, `Station`, `CoachMessage`,
`EquipCard`, `RubricRow`, `OnboardingFx`) plus the wire object:

```
StudioProjection = {
  project:       { title, qualLabel },
  stations:      Station[],                       // code,name,view,state,gate?{total,passed},backflow?
  activeStation: StationCode,
  coach:         { anchor, messages: CoachMessage[], equipment: EquipCard[] },
  onboarding:    OnboardingFx,
}
```

- `CoachMessage` gains the carried-forward field: the `ai` variant becomes
  `{ kind:"ai"; body:string; tag?:string; anchor?:string }`. (`tag` = criterion
  e.g. `"D5"`; `anchor` = the focus label e.g. `"论证图 · 治理决心主张"`.)
- **Why lean, not full `StudioState`:** the frontend `views.material` element is
  the rich `SourceFixture` shape (annotate spans, CRAAP labels) that has no DB
  backing until the Slice-6 material/source-log data model. Keeping that (and the
  other deferred center-pane view types) out of the wire contract avoids dragging
  Slice-6+ shapes into `packages/contracts` and keeps 5b honest.
- `studio/state.ts` **imports the shared view types** from `@mind-imprint/contracts`
  (Station/CoachMessage/EquipCard/RubricRow/OnboardingFx + code/view/state enums)
  and keeps the deferred view types (`StructureCardFx`, `GaugeFx`, the material
  item type), the composed `StudioState`, and `StudioCallbacks` **local**. No
  DAG/gate logic is duplicated in TS.
- `StudioContainer` maps `StudioProjection → StudioState`, stubbing the deferred
  center-pane views: `material:[]`, `structure:[]`, `writing:{draft:"",mode:"edit"}`,
  `review:[]`, `onboarding` from the projection. Slices 6–9 extend both the DTO
  and this mapper as each view goes live. This is pure field-slotting, not logic.

Go side: `apps/api/internal/studio/dto.go` declares structs matching
`StudioProjection` with explicit `json:"…"` tags copied field-for-field from the
Zod schema. A **parity test** (`dto_parity_test.go`) marshals a fully-populated
DTO and asserts the JSON key set matches the contract — same discipline as the
existing `OutputAnchor` Go↔Zod parity.

## 2. Skill JSON change: contract `title`

Add a `"title"` (Chinese station name, from the table above) to each of the 7
contracts in `packages/contracts/skills/writing-project.json`, and `Title string`
to `skills.Contract` (`skill.go`). The JSON already carries Chinese `view`
strings, so a Chinese `title` is consistent. Re-run `make sync-skills` so the Go
`go:embed` copy matches. `skills.Validate()` is unaffected (title is descriptive).

## 3. Backend projection: `apps/api/internal/studio`

New package. Depends on `skills`, `agent` (for `ReconcileGates`, `GateReport`,
`GraphView`), and the sqlc models. Testable in isolation from a `ProjectData`
input struct (all rows already loaded), so unit tests need no DB.

```go
// ProjectData is the fully-loaded read set for one project.
type ProjectData struct {
    Project       sqlc.Project
    Nodes         []sqlc.GraphNode
    Edges         []sqlc.GraphEdge
    GateStates    []sqlc.GraphNode        // gate_state nodes (recorded)
    Plan          *sqlc.GraphNode         // plan node, nil if none
    Interventions []sqlc.Intervention
    Cards         []sqlc.CardInstance
}

// Project builds the wire DTO. Pure; no I/O. specByID is the card-catalog
// lookup already carried by api.Deps (func(string)(cards.Spec,bool)).
func Project(skill skills.Skill, specByID func(string) (cards.Spec, bool), d ProjectData) (StudioProjection, error)
```

The api handler loads `ProjectData` via the existing sqlc queries and calls
`Project`. (A thin `Load(ctx, q, projectID)` helper in the package assembles
`ProjectData`; keeping it separate from `Project` preserves the pure-function
unit-test seam.)

### Field-by-field projection rules

- **project.title** = `Project.Title`; **project.qualLabel** = `Project.Qualification`.
- **stations** — for each contract in `TopoOrder()` at index `i`:
  - `code="S{i}"`, `name=contract.Title`, `view=contract.View`.
  - Reconcile once: `reports := agent.ReconcileGates(skill, graphView, recorded)`
    where `graphView` is built from `Nodes` and `recorded` from `GateStates`
    (reuse the same adapters the agent store uses).
  - **state**:
    - `done` if `reports[id].Solid`.
    - `current` if the contract is the plan's head — the first contract in the
      plan `route` (decode `Plan.Body.route`), else the first non-`Solid`
      contract in topo order when there is no plan.
    - `locked` otherwise.
  - **gate** `{total,passed}` (omit for contracts with zero gate items):
    - `total` = len(machine) + len(student_written) + len(human) for the contract.
    - `passed` = machine items reported satisfied (`ItemResult.Pass` for machine
      kinds, i.e. `Status=="machine_clear"` counts all machine items;
      `partial`/`empty` counts the individually-passing machine items) **plus**
      student_written/human items present in the recorded gate's confirmed
      `Items`. Deterministic; derived only from `GateReport` + recorded state.
    - *(The fixture's literal `{total:5,passed:2}` was authored for the 5a
      mockup; the projection computes these and the seed authors a believable
      partial for `build_argument`.)*
  - **backflow**: `true` for a `done` station that the plan route still lists
    (revisit reachable). For 5b the seed sets S3 `backflow` implicitly this way;
    a `done` station not in the route has no backflow.
- **activeStation** = the `current` station's code.
- **focusMode** = `false` (client-local thereafter).
- **coach.anchor** = the most recent `Intervention`'s decoded `anchor` label;
  fallback to the current contract's `title` when there are no interventions.
- **coach.messages** — `Interventions` ordered by `created_at`, mapped:
  - `type=="flag"` → `{kind:"flag", label:<criterion or anchor label>, body}`.
  - otherwise → `{kind:"ai", body, tag:criterion?, anchor:<decoded anchor label>?}`.
  - **No `student` messages in 5b** (they are produced by the composer → chat in
    5c). The union keeps the `student` variant for 5c.
- **coach.equipment** — one entry per `CardInstance`:
  - `id` = card instance id; `name` = `specByID(card_id)` display name;
  - `spont` = `"提示后"` if any `Intervention.CardInstanceID` == this instance
    (agent-surfaced/nudged), else `"自发"`;
  - `meth` = card→methodology map: `craap|sift → "sift_craap"`,
    `concession|steelman → "concession"` (the `MethodologyModal` keys).
- **onboarding** (S0 live view) — from `decode_task` graph nodes:
  `rubric_translation` node body → `rubricRows[]`; `milestone_plan` node body →
  `planSteps[]`; `restatePrompt` from the decode node body (seed-authored).
- **Center-pane views deferred** — the projection does **not** carry
  material/structure/writing/review. `StudioContainer` stubs them (empty). Their
  live content lands with each view's slice:
  - **素材 (material) → Slice 6** — `SourceDossier` needs the annotate-span +
    source-log data model (Slice-6 material work); the thin DB `material` row
    can't feed the rich `SourceFixture`. Deferred to keep 5b honest.
  - **结构 / 写作 / 评估 → Slices 7 / 8 / 9** — inert shells in 5a.
  The seed still mints materials + graph nodes (Slice-6 readiness), but 5b does
  not project or render them.

## 4. Endpoints (`apps/api/internal/api/projects.go`)

Both under `RequireUser` (registered in `api.go` alongside the task routes):

- `GET /api/v1/projects` → `a.listProjects`: `ListProjectsByUser(caller.ID)` →
  `[]ProjectListItem{ id, title, qualLabel, activeStation, updatedAt }`.
  `activeStation` for the list item is computed the same way (cheap: needs the
  plan node; acceptable to compute per project, list is small in the demo).
- `GET /api/v1/projects/{id}` → `a.getProject`: parse id; `GetProject(id)`;
  **404 (not 403)** if `project.user_id != caller.ID` (don't leak existence);
  `studio.Load` + `studio.Project` → `StudioStateDTO`; JSON 200.

`api.Deps` already carries `Queries` and the card `Catalog`/`SpecByID` needed for
the equipment name lookup. The skill is loaded via `skills.ByID("writing-project")` (add to
`Deps` if a preloaded skill is cleaner; otherwise load per request — the catalog
is embedded, cost is trivial).

## 5. sqlc / query changes

- **None.** All queries the projection needs already exist: `ListProjectsByUser`,
  `GetProject`, `ListGraphNodesByProject`, `ListGraphEdgesByProject`,
  `ListGateStateNodes`, `GetPlanNode`, `ListInterventionsByProject`,
  `ListCardInstancesByProject`, `ListMaterialsByProject`. No sqlc regen this slice.
  (Project-scoping `GetCardInstance` is deferred to 5c — see Scope/Out.)

## 6. Migration 0018 — seed demo Task + Project

`apps/api/internal/store/migrations/0018_seed_demo_project.sql`. Fixed UUIDs off
Phoebe (`…0003`), idempotent-friendly, authored so the projection ≈ `STUDIO_FIXTURE`
(0457 中国可持续, station **S4** current, mid-orphan-evidence). Content is the real
scenario (no lorem ipsum), drawn from `STUDIO_FIXTURE` + `workspace/material/fixtures`.
Purely additive — no schema change; the existing `NOT NULL` `task_id` FKs are
satisfied by a seeded demo **task**.

Rows (fixed UUIDs, `…01xx` namespace):
- **task** `…0100`: `user_id=…0003`, `title='思维印记 · 0457 演示'`, `status='active'`
  — the legacy FK anchor for the seeded materials/card_instances (task_id is still
  `NOT NULL` until 5c).
- **project** `…0101`: `user_id=…0003`, `qualification='0457 个人报告'`,
  `title='To what extent is China making the world more environmentally sustainable?'`,
  `status='active'`.
- **graph_node** (author `student`/`ai`/`imported` as fits):
  - `decode_task`: `rubric_translation` (body = the 4 rubric rows), `milestone_plan`
    (body = the 6 plan steps + restate prompt) — feed S0 onboarding.
  - `build_argument`: `claim` (核心主张, done preview), `claim` (治理决心 — the "裸主张"),
    `evidence` (可再生投资全球第一 — the orphan), `concession` node absent (gate not yet met),
    `counter`/`steelman` as authored. Enough that `ReconcileGates` reports
    `build_argument` **partial** (an orphan-evidence + unsupported-claim in play).
  - `gate_state` nodes for S0–S3 marking their gates solid (so S0–S3 project `done`),
    none for S4 (so it's `current`/partial).
  - `plan` node body `{route:["build_argument", …downstream], reason:"intake"}` so
    S4 is the plan head → `current`.
  (All graph nodes carry `project_id=…0101`.)
- **material** ×2 (`…0110`, `…0111`): the NASA greening + Nature Sustainability
  sources from the fixture, `project_id=…0101`, `task_id=…0100` (the seed task) to
  satisfy the FK. (5b does not project material; seeded for Slice-6 readiness.)
- **intervention** ×2 (`…0120` flag, `…0121` ai): the 孤儿证据 flag and the
  "connect to 治理决心 / criterion D5 / anchor 论证图 · 治理决心主张" nudge.
- **card_instance** ×2–3 (`…0130`…, `project_id=…0101`, `task_id=…0100`): e.g.
  `steelman` (提示后 — linked by intervention `…0121`), `concession` (自发) — to show
  both `spont` states + `meth`.

`-- +goose Down` deletes all `…01xx`/`…0100` rows (children first).

## 7. Frontend

- **`packages/contracts`**: add the new `studioState` export to the package index.
- **`apps/web/src/studio/state.ts`**: import the shared view types
  (`Station`/`StationCode`/`StationView`/`StationState`/`CoachMessage`/`EquipCard`/
  `RubricRow`/`OnboardingFx`) from `@mind-imprint/contracts`; keep the deferred
  view types (`StructureCardFx`, `GaugeFx`, the material item type), the composed
  `StudioState`, and `StudioCallbacks` **local**. `STUDIO_FIXTURE` still
  type-checks (only addition is optional `CoachMessage.ai.anchor`).
- **`apps/web/src/studio/CoachRail.tsx`**: for `ai` messages, render the criterion
  chip (`tag`) **and** the `锚定 {anchor}` label when `anchor` is present — the
  carried-forward 5a fix (chip + 锚定-label split). Update the fixture's `ai`
  message to include `anchor` so the existing harness shows the final look.
- **`apps/web/src/api/projects.ts`**: `listProjects()` and `getProject(id)` using
  the existing `apiFetch` wrapper; parse `getProject` through the Zod
  `StudioProjection` schema (fail loud on drift). Add to the `api` aggregate +
  `ApiClient` interface.
- **`apps/web/src/studio/StudioContainer.tsx`** (new): the live host. On mount:
  ensure session (reuse the trial-signin path if unauthenticated under `?studio`),
  `listProjects()` → pick the first (demo has one) → `getProject(id)` →
  **map `StudioProjection → StudioState`** (stub the deferred center-pane views)
  → hold it + client-local `activeStation`/`focusMode`. Callbacks:
  `onSelectStation`/`onToggleFocus`/`onOpenMethodology` are **client-live**;
  `onComposerSend`/`onDisposition` are **inert no-ops** (a `// wired in 5c` note).
  Loading + error states (spinner / retry).
- **`apps/web/src/Root.tsx`**: add a `?studio` branch **alongside** `?demo`,
  rendering `<StudioContainer/>`. Does not touch the existing `?demo`/`AppShell`
  branches (routing flip is 5d).

## Testing

- **Go projection unit tests** (`internal/studio/projection_test.go`): table-driven
  over a hand-built `ProjectData` — assert station codes/names/views/states, gate
  `{total,passed}`, coach anchor/messages/equipment (`spont`+`meth`), and
  onboarding. Pure function, no DB.
- **Go↔Zod parity** (`dto_parity_test.go`): marshal a full `StudioProjection` DTO;
  assert key set matches the contract (golden JSON check).
- **Endpoint tests** (`internal/api`): `GET /projects` and `GET /projects/{id}`
  happy path (authed as Phoebe) + **404 for another user's project** + 401
  unauthenticated. Uses the existing api test harness.
- **testcontainers round-trip** (`-short`-gated like the Slice-4 test): run
  migrations (incl. the 0018 seed) against real PG, load the seeded project via the
  real sqlc queries, project it, assert the station rail ≈ fixture (S0–S3 done,
  S4 current partial, S5–S6 locked) + coach anchor/thread.
- **vitest**: `CoachRail` renders the chip + `锚定 {anchor}` for an `ai` message
  with `anchor`; `StudioContainer` renders `StudioShell` from a mocked
  `getProject` and shows loading/error. `projects.ts` client parse test.
- **Gate:** full web suite + `tsc` clean; `go build`/`vet`; `-short` all; the
  testcontainers round-trip green; boundary held; existing 446 web tests + Go
  suite stay green (legacy paths untouched).

## Acceptance (live-verify walk)

Run the stack against a fresh migrated DB. Visit `/?studio`. Expect: the Station
rail shows S0–S3 **done**, **S4 论证构建 current** with a partial gate strip
(`本环节门禁 N 项，已过 M`), S5–S6 **locked**; the Coach rail header shows
`正在看：论证图 · 治理决心主张`, the 孤儿证据 flag + the D5 nudge (chip + 锚定 label),
and the 装备栏 chips; clicking **S0** shows the real rubric rows + plan steps. The
center panes for 素材/结构/写作/评估 show their 5a deferred state (live content lands
in Slices 6–9). The composer and 三键处置 are visible but inert (5c). No fixture
data feeds the live station/coach/onboarding surfaces.

## Deferred / carry-forward → 5c, 5d, 6–9

- **5c:** composer send → chat_message; `RunAgentStep` HTTP driver + SSE; live
  coach interventions; disposition **persistence**; check_gate debounce; student
  coach bubbles.
- **5d:** routing flip (Studio as the real student surface); retire chat
  `workspace/`, `StudentApp` task-workspace, `RunTurn`/`turn.go`, `createConversation`.
- **6–9:** live projection + interaction for the center-pane views, each
  extending `StudioProjection` + the `StudioContainer` mapper: **6** 素材
  source-log (`SourceDossier` live over the annotate/source-log data model), **7**
  结构 graph+Toulmin, **8** 写作 whole-draft, **9** 评估 readiness+reflect.
- **Minor:** onboarding restate-prompt source is seed-authored; a first-class
  decode artifact shape can firm up when S0 is exercised live (5c/6). The demo
  Task (`…0100`) is a legacy-FK anchor only; it disappears once 5c makes
  `task_id` nullable and the seed can mint project-native rows.

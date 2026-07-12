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
1. A shared `studioState` Zod contract (`packages/contracts`) — the wire DTO — with a Go DTO that marshals to the same shape (Go↔Zod parity).
2. A Go read-projection package `apps/api/internal/studio` that turns a Project + its graph/gates/plan/interventions/cards/materials into a `StudioState` DTO, using the `skills` DAG + `agent.ReconcileGates` + `TopoOrder`.
3. Two protected, ownership-checked endpoints: `GET /projects` (list) and `GET /projects/{id}` (full `StudioState`).
4. The Slice-3 deferred debt (prerequisites): migration making `task_id` NULLABLE on `material`/`card_instances`/`evaluations`, and a project-scoped `GetCardInstance`.
5. A seed migration minting a demo Project (Phoebe, 0457 中国可持续, station S4) whose projection ≈ `STUDIO_FIXTURE`.
6. Frontend: refactor `studio/state.ts` to consume the contract type; a `projects` API client; a live `StudioContainer` reachable via `?studio` in `Root.tsx`; the carried-forward `CoachMessage.ai` `anchor` field + CoachRail render.

**Out (later slices):**
- Live coach/turn loop, `RunAgentStep` HTTP driver, check_gate debounce, composer-send, disposition **persistence** → **5c**.
- App-routing flip (Studio as the real student surface) + retiring the old chat `workspace/` + legacy `RunTurn`/`turn.go` → **5d**.
- Deep projection of the 结构/写作/评估 views (their view shells are inert in 5a) → **Slices 7/8/9**.

## Global Constraints (verbatim, bind every task)

- **Client never calls the model/Volcano directly.** All model/audio via the Go gateway; keys only server-side. (Not exercised this slice — read path — but the boundary holds: the frontend only calls the new REST endpoints.)
- **Single source of truth.** The skill contract DAG, gate logic (`ReconcileGates`), and `TopoOrder` live in Go and are **not** duplicated in TS. The projection is server-side; the frontend renders the DTO.
- **Binding Chinese design copy is verbatim** (design `docs/design/思维印记_工作区.dc.html`). Never alter design copy to satisfy a test — fix the test matcher.
- **Icons = inline SVG, never `lucide-react`.**
- **Studio discipline (from 5a/Slice-1):** do not mutate the old chat `workspace/`, `StudentApp`, `AppShell` routing, or `Root`'s existing branches. `?studio` is **additive** alongside `?demo`. The routing flip is 5d.
- **Additive, reversible migrations; build stays green.** Legacy `task_id`-bearing code paths keep working (they still supply `task_id`); the column only becomes *nullable*.
- **Docs in English** except literal Chinese UI copy.
- **Fixed-UUID seed pattern** (per `0002_seed.sql`): the demo Project and its nodes use deterministic UUIDs referencing Phoebe (`00000000-0000-0000-0000-000000000003`).

## Architecture

```
GET /api/v1/projects/{id}
  → api.getProject (ownership: project.user_id == caller, else 404)
      → studio.Project(ctx, deps, projectID)                     [internal/studio]
          load: GetProject, ListGraphNodesByProject, ListGraphEdgesByProject,
                ListGateStateNodes, GetPlanNode, ListInterventionsByProject,
                ListCardInstancesByProject, ListMaterialsByProject
          skill: skills.ByID("writing-project")
          compute: agent.ReconcileGates(skill, graphView, recordedGates)
                   skill.TopoOrder()
          build:  StudioStateDTO  ── json ──▶  studioState (Zod)  ── z.infer ──▶ StudioState (TS)
  → StudioContainer (?studio) renders <StudioShell state callbacks/>
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

## 1. Shared contract: `studioState` (Zod) + Go DTO parity

New file `packages/contracts/src/studioState.ts` defines a Zod schema whose shape
is **exactly** today's `StudioState` (`apps/web/src/studio/state.ts`), so the type
can be lifted with no rendering change. Field names are the wire contract
(camelCase where they already are): `project.{title,qualLabel}`, `stations[]`
(`code,name,view,state,gate?{total,passed},backflow?`), `activeStation`,
`focusMode`, `coach.{anchor,messages[],equipment[]}`, `views.{material,structure,
writing,review,onboarding}`.

- `CoachMessage` gains the carried-forward field: the `ai` variant becomes
  `{ kind:"ai"; body:string; tag?:string; anchor?:string }`. (`tag` = criterion
  e.g. `"D5"`; `anchor` = the focus label e.g. `"论证图 · 治理决心主张"`.)
- `views.material` element type: the material item shape currently imported from
  `workspace/material/fixtures` (`SourceFixture`) is defined **in the contract**
  so Go can produce it. (Move/mirror the field shape; the frontend fixture keeps
  working by conforming to the contract type.)
- `studio/state.ts` re-exports the inferred types (`export type StudioState = z.infer<...>`)
  and keeps `StudioCallbacks` **local** (callbacks are not wire data).

Go side: `apps/api/internal/studio/dto.go` declares matching structs with explicit
`json:"…"` tags copied field-for-field from the Zod schema. A **parity test**
(`dto_parity_test.go`) marshals a fully-populated DTO and asserts the JSON keys
match a checked-in golden (or the frontend parses it) — same discipline as the
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
    Materials     []sqlc.Material
}

// Project builds the wire DTO. Pure; no I/O. specByID is the card-catalog
// lookup already carried by api.Deps (func(string)(cards.Spec,bool)).
func Project(skill skills.Skill, specByID func(string) (cards.Spec, bool), d ProjectData) (StudioStateDTO, error)
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
- **views.material** = `Materials` mapped to the contract material shape (the one
  fully-live view, 素材/`SourceDossier`).
- **views.onboarding** (S0 live view) — from `decode_task` graph nodes:
  `rubric_translation` node body → `rubricRows[]`; `milestone_plan` node body →
  `planSteps[]`; `restatePrompt` from the decode node body (seed-authored).
- **views.structure / writing / review** = **empty** (`[]` / empty draft / `[]`).
  Their view shells are inert placeholders in 5a; deep projection lands with
  Slices 7/8/9. Stated so nothing is built that isn't rendered.

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

- **`GetCardInstance` project-scoped** (`queries/card_instance.sql`): add
  `AND project_id = $2`; regenerate. Update the one caller
  (`agentstore.go`) to pass `projectID`. (Matches the sibling project-scoped
  queries in the same file. This is the Slice-3 debt.)
- No new project/graph queries needed — `ListProjectsByUser`, `GetProject`,
  `ListGraphNodesByProject`, `ListGraphEdgesByProject`, `ListGateStateNodes`,
  `GetPlanNode`, `ListInterventionsByProject`, `ListCardInstancesByProject`,
  `ListMaterialsByProject` all exist.
- `make sqlc` (`CGO_ENABLED=0 go tool sqlc generate`) after the query edit.

## 6. Migration 0018 — `task_id` NULLABLE (Slice-3 debt)

`apps/api/internal/store/migrations/0018_nullable_task_id.sql`, additive/reversible:

```sql
-- +goose Up
ALTER TABLE material       ALTER COLUMN task_id DROP NOT NULL;
ALTER TABLE card_instances ALTER COLUMN task_id DROP NOT NULL;
ALTER TABLE evaluations    ALTER COLUMN task_id DROP NOT NULL;
-- +goose Down
-- Reversible only while no project-native (task_id IS NULL) rows exist; the
-- demo seed (0019) always supplies task_id, so down stays safe in dev.
ALTER TABLE material       ALTER COLUMN task_id SET NOT NULL;
ALTER TABLE card_instances ALTER COLUMN task_id SET NOT NULL;
ALTER TABLE evaluations    ALTER COLUMN task_id SET NOT NULL;
```

Legacy paths still supply `task_id`; nothing breaks. (Down carries the standard
caveat, like 0017's.)

## 7. Migration 0019 — seed demo Project

`apps/api/internal/store/migrations/0019_seed_demo_project.sql`. Fixed UUIDs off
Phoebe (`…0003`), idempotent-friendly, authored so the projection ≈ `STUDIO_FIXTURE`
(0457 中国可持续, station **S4** current, mid-orphan-evidence). Content is the real
scenario (no lorem ipsum), drawn from `STUDIO_FIXTURE` + `workspace/material/fixtures`.

Rows (fixed UUIDs, `…01xx` namespace):
- **project** `…0100`: `user_id=…0003`, `qualification='0457 个人报告'`,
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
- **material** ×2 (`…0110`, `…0111`): the NASA greening + Nature Sustainability
  sources from the fixture, `project_id=…0100`, `task_id` = a seed task (or the
  legacy demo task) to satisfy the FK until callers stop supplying it.
- **intervention** ×2 (`…0120` flag, `…0121` ai): the 孤儿证据 flag and the
  "connect to 治理决心 / criterion D5 / anchor 论证图 · 治理决心主张" nudge.
- **card_instance** ×2–3 (`…0130`…): e.g. `steelman` (提示后 — linked by an
  intervention), `concession` (自发) — to show both `spont` states + `meth`.

`-- +goose Down` deletes all `…01xx` rows (children first).

## 8. Frontend

- **`packages/contracts`**: build the new `studioState` export; ensure it's
  re-exported from the package index.
- **`apps/web/src/studio/state.ts`**: replace the hand-written `StudioState`/
  `CoachMessage`/`EquipCard`/… types with imports from `@mind-imprint/contracts`
  (`z.infer`). Keep `StudioCallbacks` local. `fixtures.ts` (`STUDIO_FIXTURE`) must
  still type-check against the contract type (it will, since the contract mirrors
  the current shape; the only addition is optional `CoachMessage.ai.anchor`).
- **`apps/web/src/studio/CoachRail.tsx`**: for `ai` messages, render the criterion
  chip (`tag`) **and** the `锚定 {anchor}` label when `anchor` is present — the
  carried-forward 5a fix (chip + 锚定-label split). Update the fixture's `ai`
  message to include `anchor` so the existing harness shows the final look.
- **`apps/web/src/api/projects.ts`**: `listProjects()` and `getProject(id)` using
  the existing `apiFetch` wrapper; parse responses through the Zod `studioState`
  schema (fail loud on drift). Add to the `api` aggregate + `ApiClient` interface.
- **`apps/web/src/studio/StudioContainer.tsx`** (new): the live host. On mount:
  ensure session (reuse the trial-signin path if unauthenticated under `?studio`),
  `listProjects()` → pick the first (demo has one) → `getProject(id)` → hold
  `StudioState` + a client-local `activeStation`/`focusMode`. Callbacks:
  `onSelectStation`/`onToggleFocus`/`onOpenMethodology` are **client-live**;
  `onComposerSend`/`onDisposition` are **inert no-ops** (a `// wired in 5c` note).
  Loading + error states (spinner / retry).
- **`apps/web/src/Root.tsx`**: add a `?studio` branch **alongside** `?demo`,
  rendering `<StudioContainer/>`. Does not touch the existing `?demo`/`AppShell`
  branches (routing flip is 5d).

## Testing

- **Go projection unit tests** (`internal/studio/projection_test.go`): table-driven
  over a hand-built `ProjectData` — assert station codes/names/views/states, gate
  `{total,passed}`, coach anchor/messages/equipment (`spont`+`meth`), onboarding,
  and empty structure/writing/review. Pure function, no DB.
- **Go↔Zod parity** (`dto_parity_test.go`): marshal a full DTO; assert key set
  matches the contract (golden JSON or a generated-fixture check).
- **Endpoint tests** (`internal/api`): `GET /projects` and `GET /projects/{id}`
  happy path (authed as Phoebe) + **404 for another user's project** + 401
  unauthenticated. Uses the existing api test harness.
- **testcontainers round-trip** (`-short`-gated like the Slice-4 test): run
  migrations (incl. 0018/0019) against real PG, load the seeded project via the
  real sqlc queries, project it, assert the station rail ≈ fixture (S0–S3 done,
  S4 current partial, S5–S6 locked) + material count.
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
and the 装备栏 chips; clicking **S2/S3** shows the live 素材 dossier with the two real
sources; clicking **S0** shows the real rubric rows + plan steps. The composer and
三键处置 are visible but inert (5c). No fixture import remains in the live path.

## Deferred / carry-forward → 5c, 5d, 6–9

- **5c:** composer send → chat_message; `RunAgentStep` HTTP driver + SSE; live
  coach interventions; disposition **persistence**; check_gate debounce; student
  coach bubbles.
- **5d:** routing flip (Studio as the real student surface); retire chat
  `workspace/`, `StudentApp` task-workspace, `RunTurn`/`turn.go`, `createConversation`.
- **6–9:** deep projection + interaction for 素材 source-log / 结构 graph+Toulmin /
  写作 whole-draft / 评估 readiness+reflect.
- **Minor:** `GetCardInstance` was project-scoped here; confirm no legacy caller
  regressed. Onboarding restate-prompt source is seed-authored; a first-class
  decode artifact shape can firm up when S0 is exercised live (5c/6).

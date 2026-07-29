# S3 · Rabbit-hole exploration surface — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development.
> Steps use checkbox (`- [ ]`) syntax. Spec: `docs/2026-07-29-s3-rabbit-hole-spec.md`.

**Goal:** Turn the flat 文献库 into a living exploration graph — sources branch into first-class
leads, read-but-unused sources dangle, and a metered guide points at the next necessary direction
(never fetching, never deciding).

**Architecture:** New `exploration_lead` table (migration 0040) + sqlc queries; lead-lifecycle
handlers (no spend) + a metered guide handler (mirrors S2 `getTakeawayDraft`) + an isolated
`ComposeExplorationGuide` (mirrors `ComposeReadingTakeawaySuggestions`); leads materialize
idempotently on `postFinalizeReading`; exploration state projects into the coach spine; a
列表⇄探索图谱 view in the web library renders the graph with connect/prune/dig affordances.

**Tech Stack:** Go (net/http ServeMux, pgx, sqlc, goose), Postgres, Zod contracts, React+Vite+TS.

## Global Constraints (every task's requirements implicitly include these)

- **Client NEVER calls a model.** The only LLM call is `POST /exploration/guide` on the server;
  metered with tier+tokens+purpose (`exploration_guide`), gated on `HasEntitlement`, and
  **skipped entirely (no call, no meter) when the graph is empty**. Never 500 on compose failure —
  return 200 with `directions: []`.
- **AI 克制:** the guide points at gaps/directions as questions; it never fetches a source, never
  decides a source's value, never concludes for the student. Nothing is auto-added — the student
  taps [记为线索].
- **New card = new JSON, not renderer code.** S3 adds NO card JSON. The existing `rabbit-hole` card
  is only *reached*, conditionally (Task 9), never modified.
- **Wire is camelCase everywhere.** Go DTO json tags and the model-parse tags: DTOs and the
  `GuideDirection` returned to the client are camelCase and must match the Zod shapes in
  `packages/contracts/src/exploration.ts` exactly. (The model-parse struct inside the compose fn
  stays snake — that's the model's contract, like S2's `new_leads`.)
- **IDOR:** every lead/reference lookup is scoped by project via `GetExplorationLeadForProject` /
  `GetReferenceForProject` (mirror the S2 guards). No cross-project reads.
- **Enum-empty-string trap (S2 lesson):** a contract enum column stored as `""` breaks Zod array
  parse and takes down the whole library. `status`/`origin` are NOT NULL with DB defaults and are
  always written from a validated enum — never `""`. `phaseTag` on the guide-input source uses the
  existing nullable handling.
- Migrations are additive; do not alter existing tables. Next number is **0040**.
- `make sqlc` needs `CGO_ENABLED=0`. `internal/api` tests use testcontainers (Docker required),
  ~340s — run FOREGROUND with a large timeout.

---

## Task 1: Migration 0040 + exploration_lead queries

**Files:**
- Create: `apps/api/internal/store/migrations/0040_exploration_lead.sql`
- Modify: `apps/api/internal/store/queries/workspace.sql` (append the 6 queries)
- Regenerate: `apps/api/internal/store/sqlc/*` via `make sqlc` (CGO_ENABLED=0)
- Test: `apps/api/internal/store/exploration_lead_test.go` (or extend an existing store test)

**Interfaces:**
- Produces (sqlc-generated): `sqlc.ExplorationLead` row struct; `ListExplorationLeads(ctx, projectID)`,
  `CreateExplorationLead(ctx, CreateExplorationLeadParams)`, `GetExplorationLeadForProject(ctx,
  GetExplorationLeadForProjectParams{ID, ProjectID})`, `UpdateExplorationLead(ctx,
  UpdateExplorationLeadParams)`, `DeleteExplorationLead(ctx, DeleteExplorationLeadParams{ID,
  ProjectID})`, `CountExplorationLeadForSource(ctx, CountExplorationLeadForSourceParams{ProjectID,
  SourceReferenceID, Text})`.

- [ ] **Step 1: Write the migration.** Goose up/down. Body = the `CREATE TABLE exploration_lead`
  from spec §3 verbatim (columns, both CHECKs, FK cascade from project, both reference FKs
  `ON DELETE SET NULL`, `exploration_lead_project_idx`). Down = `DROP TABLE exploration_lead`.
  Match the goose annotation style of `0039_reading_takeaway.sql` (read it first).

- [ ] **Step 2: Add the 6 queries** to `workspace.sql` (spec §3), mirroring the existing
  `ListReferences/CreateReference/GetReferenceForProject/UpdateReference/DeleteReference` shapes:
  - `ListExplorationLeads :many` — `WHERE project_id = $1 ORDER BY position, created_at`.
  - `CreateExplorationLead :one` — INSERT (project_id, text, status, origin, source_reference_id,
    connected_reference_id, position) RETURNING *.
  - `GetExplorationLeadForProject :one` — `WHERE id = $1 AND project_id = $2`.
  - `UpdateExplorationLead :one` — `SET text=$3, status=$4, connected_reference_id=$5, position=$6,
    updated_at=now() WHERE id=$1 AND project_id=$2 RETURNING *`.
  - `DeleteExplorationLead :exec` — `WHERE id=$1 AND project_id=$2`.
  - `CountExplorationLeadForSource :one` — `SELECT count(*) FROM exploration_lead WHERE
    project_id=$1 AND source_reference_id=$2 AND text=$3`.

- [ ] **Step 3: Regenerate sqlc.** Run `CGO_ENABLED=0 make sqlc` from repo root. Verify
  `sqlc.ExplorationLead` and the 6 funcs exist in `internal/store/sqlc/`.

- [ ] **Step 4: Write a store round-trip test** (real Postgres via the package's existing
  testcontainer harness — copy the setup from a neighbor like `workspace_library_test.go` or the
  store package's existing test). Seed a school→class→student→project (reuse the existing seed
  helper the api tests use; grep for it). Assert: Create → List returns it; Get scoped by project;
  Update flips status to `connected` + sets connected_reference_id; Count returns 1 for
  (project, source, text) and 0 for a different text; Delete removes it; Get from a *different*
  project id returns `pgx.ErrNoRows`.

- [ ] **Step 5: Run tests foreground.** `cd apps/api && go test ./internal/store/... -run
  ExplorationLead -v` (timeout 600000). Expect PASS. Then `go build ./...`.

- [ ] **Step 6: Commit.** `feat(s3): exploration_lead table + queries (migration 0040)`

---

## Task 2: Contracts — exploration.ts

**Files:**
- Create: `packages/contracts/src/exploration.ts`
- Modify: `packages/contracts/src/index.ts` (add `export * from "./exploration"`)
- Test: `packages/contracts/test/exploration.test.ts`

**Interfaces:**
- Produces: `LeadStatus`, `LeadOrigin`, `ExplorationLead`, `ExplorationView`, `GuideDirection`,
  `ExplorationGuide` (+ inferred TS types). Shapes = spec §5 verbatim.

- [ ] **Step 1: Write failing tests** in `test/exploration.test.ts` (mirror `test/reference.test.ts`
  style — vitest). Assert: a valid `ExplorationLead` parses; `status:"open"|"connected"|"pruned"`
  accepted and `status:""` **rejected** (the enum-empty-string guard); `sourceReferenceId: null`
  accepted; `ExplorationView` with empty arrays parses; `ExplorationGuide` with
  `directions:[{direction,why}]` parses.

- [ ] **Step 2: Run — expect fail** (module not found). `cd packages/contracts && pnpm test
  exploration` → FAIL.

- [ ] **Step 3: Implement `exploration.ts`** = spec §5 verbatim, with `import { z } from "zod"` and
  exported `type` inferences for each (mirror `reference.ts`).

- [ ] **Step 4: Add barrel export** `export * from "./exploration";` to `src/index.ts`.

- [ ] **Step 5: Run — expect pass.** `pnpm test exploration` → PASS. Then `pnpm test` (full
  contracts suite) → all green. Then typecheck: `pnpm -C packages/contracts build` (or the repo's
  tsc command).

- [ ] **Step 6: Commit.** `feat(s3): exploration contracts (leads, view, guide)`

---

## Task 3: Guide compose (isolated agent) — exploration_guide.go

**Files:**
- Create: `apps/api/internal/agent/exploration_guide.go`
- Test: `apps/api/internal/agent/exploration_guide_test.go`

**Interfaces:**
- Consumes: `gateway.Provider`, `gateway.Resolved`, `gateway.Collect`, the real `stripFences`
  helper (grep it in the `agent` package — reuse, don't invent).
- Produces: types `ExplorationGraphSource`, `ExplorationGuideInput`, `GuideDirection` (spec §4c);
  `HasGraphContent(in ExplorationGuideInput) bool` (`len(in.Sources)>0 || len(in.OpenLeads)>0`);
  `ComposeExplorationGuide(ctx, prov gateway.Provider, r gateway.Resolved, in ExplorationGuideInput)
  ([]GuideDirection, gateway.ChatUsage, error)`.

- [ ] **Step 1: Write failing tests.** Copy the provider-stub pattern from a neighbor agent test
  (grep `reading_eval_test.go` / `reading_takeaway`-related test for the stub resolver + stub
  provider double — use the REAL names, e.g. `gateway.NewStubProvider`, `readingStubResolver` or
  equivalent). Tests:
  - `TestComposeExplorationGuide_ParsesDirections`: stub provider returns
    ` ```json\n{"directions":[{"direction":"找中国碳排放绝对量的一手数据","why":"你缺反例检验的来源"}]}\n``` `;
    assert one direction parsed with both fields.
  - `TestComposeExplorationGuide_EmptyGraphErrors`: input with no sources and no leads → returns a
    non-nil error and makes NO provider call (assert the stub's call count is 0).
  - `TestHasGraphContent`: true with a source, true with an open lead, false with neither.

- [ ] **Step 2: Run — expect fail.** `cd apps/api && go test ./internal/agent/ -run
  ExplorationGuide -v` → FAIL (undefined).

- [ ] **Step 3: Implement.** Types per spec §4c. `explorationGuideSystem` const = the 克制 prompt
  from spec §4c verbatim. `ComposeExplorationGuide`: early-return `(nil, ChatUsage{},
  errors.New("empty graph"))` when `!HasGraphContent(in)` (before any provider call); build a user
  message summarizing proposal objective + sources (title｜state｜decision｜credibility｜phaseTag) +
  open leads + pruned/connected counts; `gateway.Collect`; parse `{"directions":[{direction,why}]}`
  via the real `stripFences` + `json.Unmarshal`; cap to 3; return directions + usage. Structure it
  after `ComposeReadingTakeawaySuggestions` (`reading_takeaway.go:80`).

- [ ] **Step 4: Run — expect pass.** `go test ./internal/agent/ -run ExplorationGuide -v` → PASS.

- [ ] **Step 5: Commit.** `feat(s3): ComposeExplorationGuide isolated compose (克制 guide)`

---

## Task 4: Lead lifecycle handlers (no spend) + routes + DTO

**Files:**
- Create: `apps/api/internal/api/exploration.go`
- Modify: `apps/api/internal/api/api.go` (register 4 routes)
- Test: `apps/api/internal/api/exploration_test.go`

**Interfaces:**
- Consumes: Task 1 queries; `a.loadOwnedProject`, `httpx.WriteJSON`/`WriteError`/`ErrNotFound`,
  `parseNullableUUID`, `r.PathValue`, `pgUUIDToStringPtr` (grep these in `workspace_library.go`).
- Produces: `explorationLeadDTO` (camelCase, spec §4a); handlers `getExploration`,
  `createExplorationLead`, `patchExplorationLead`, `deleteExplorationLead`; a pure helper
  `computeDanglingSourceIds(refs []sqlc.Reference, leads []sqlc.ExplorationLead) []string`.

- [ ] **Step 1: Write failing integration tests** (external `api_test` pkg; copy harness +
  seed from `workspace_library_test.go`):
  - `TestExplorationLeadCRUD`: POST create (manual) → GET exploration lists it with
    `origin:"manual"`,`status:"open"`; PATCH to `status:"connected"`+connectedReferenceId (seed a
    reference first) → GET reflects it; DELETE → gone.
  - `TestExploration_IDOR`: a lead in project A is not GET/PATCH/DELETE-able via project B (404).
  - `TestComputeDanglingSourceIds` (pure unit, same file): a reference with material_id set +
    decision `drop` and no connecting lead → dangling; the same reference once a non-pruned lead
    connects it → not dangling; a reference with decision `use` → never dangling; a reference with
    NO material_id (未读) → not dangling.

- [ ] **Step 2: Run — expect fail.**

- [ ] **Step 3: Implement handlers** in `exploration.go`, mirroring `workspace_library.go`
  conventions:
  - `explorationLeadDTO` + `toExplorationLeadDTO(row sqlc.ExplorationLead)` (use
    `pgUUIDToStringPtr` for the two nullable ref ids).
  - `getExploration`: `loadOwnedProject` → `ListExplorationLeads` + `ListReferences` →
    `{leads: [...DTO], danglingSourceIds: computeDanglingSourceIds(refs, leads)}`.
  - `createExplorationLead`: body `{text string}`; reject empty text (400); origin `"manual"`,
    status `"open"`, position = len(existing). Return the DTO.
  - `patchExplorationLead`: partial-merge using `json.RawMessage` fields for
    `text`/`status`/`connectedReferenceId` (mirror `patchReference`'s partial pattern). Validate
    `status` ∈ enum when present (400 otherwise). When `connectedReferenceId` present & non-null,
    verify it's a reference in this project (`GetReferenceForProject`; 400 if not). Load current via
    `GetExplorationLeadForProject`, merge, `UpdateExplorationLead`.
  - `deleteExplorationLead`: `DeleteExplorationLead{ID,ProjectID}`; 204.
  - `computeDanglingSourceIds`: build a set of `connected_reference_id` from leads whose
    status != `"pruned"`; a reference is dangling iff `material_id` valid AND decision ∈ {null,
    "drop"} AND its id ∉ that set. Return string ids in stable order.

- [ ] **Step 4: Register routes** in `api.go` (alongside the library routes, all `protected`):
  `GET .../exploration`, `POST .../exploration/leads`, `PATCH .../exploration/leads/{lid}`,
  `DELETE .../exploration/leads/{lid}` → the 4 handlers.

- [ ] **Step 5: Run — expect pass.** `go test ./internal/api/ -run Exploration -v` (timeout
  600000). Then `go build ./...`.

- [ ] **Step 6: Commit.** `feat(s3): exploration lead lifecycle endpoints + dangling projection`

---

## Task 5: Materialize leads on finalize (idempotent, non-resurrecting)

**Files:**
- Modify: `apps/api/internal/api/reading_takeaway.go` (`postFinalizeReading`)
- Test: `apps/api/internal/api/reading_takeaway_test.go` (add cases)

**Interfaces:**
- Consumes: `CountExplorationLeadForSource`, `CreateExplorationLead`, `ListExplorationLeads` (Task 1).

- [ ] **Step 1: Write failing tests** (extend the reading-takeaway test file; reuse its finalize
  helper that posts `{new_leads, proposal_impact}`):
  - `TestFinalize_MaterializesLeads`: finalize a reference with `new_leads:["A","B"]` → GET
    exploration shows two `open`/`takeaway` leads with `sourceReferenceId` = that ref.
  - `TestFinalize_IdempotentNoDuplicate`: finalize again with the same `new_leads` → still exactly
    two leads (no dupes).
  - `TestFinalize_DoesNotResurrectPruned`: after materializing "A", PATCH it to `status:"pruned"`,
    then finalize again with `new_leads:["A"]` → the lead stays exactly one row, still `pruned`
    (Count sees the pruned row → no re-insert).

- [ ] **Step 2: Run — expect fail.**

- [ ] **Step 3: Implement.** In `postFinalizeReading`, after `FinalizeReadingTakeaway` succeeds and
  before/around the existing `appendAutoLog`: for each `req.NewLeads` text (skip blank/whitespace),
  if `CountExplorationLeadForSource(ctx, {ProjectID, SourceReferenceID: rid, Text: text}) == 0`,
  `CreateExplorationLead(ctx, {ProjectID, Text: text, Status:"open", Origin:"takeaway",
  SourceReferenceID: rid(valid), ConnectedReferenceID: null, Position: nextPos})`. Best-effort:
  wrap in a helper `a.materializeLeads(ctx, projectID, rid, texts)` that logs-and-continues on error
  (do NOT fail the finalize — mirror the `appendAutoLog` best-effort posture). Position = count of
  existing leads at start, incremented per insert.

- [ ] **Step 4: Run — expect pass.** `go test ./internal/api/ -run 'Finalize' -v` (timeout 600000).

- [ ] **Step 5: Commit.** `feat(s3): materialize takeaway new_leads as exploration branches`

---

## Task 6: Guide handler (metered) — postExplorationGuide

**Files:**
- Modify: `apps/api/internal/api/exploration.go` (add the handler + input assembly)
- Modify: `apps/api/internal/api/api.go` (register `POST .../exploration/guide`)
- Test: `apps/api/internal/api/exploration_test.go` (add cases)

**Interfaces:**
- Consumes: `agent.ComposeExplorationGuide` + `agent.HasGraphContent` (Task 3); `a.d.ChatResolver`,
  `a.d.Provider`, `agent.NewSqlcAgentStore(...).RecordLLMCall`, `agent.LLMCallRow`, `HasEntitlement`,
  `GetProjectProposal`, `readingOutcomesFromCards` (grep for read-state), `ListReferences`,
  `ListExplorationLeads`.

- [ ] **Step 1: Write failing tests.** Use the api test harness's stub provider/resolver (grep how
  `TestGetTakeawayDraft_*` wires a fake DeepSeek in the api package — reuse it):
  - `TestExplorationGuide_ReturnsDirections`: seed ≥1 reference (with material) → POST guide → 200
    with `directions` non-empty (stub returns one); assert exactly one `llm_call` row with purpose
    `exploration_guide` was recorded.
  - `TestExplorationGuide_EmptyGraphNoSpend`: project with no references and no leads → POST guide →
    200 with `directions:[]` and **zero** `llm_call` rows (clone `TestGetTakeawayDraft_EmptyRecordNoSpend`).

- [ ] **Step 2: Run — expect fail.**

- [ ] **Step 3: Implement `postExplorationGuide`** — clone `getTakeawayDraft`'s discipline
  (`reading_takeaway.go:144`): gate `HasEntitlement`; assemble `agent.ExplorationGuideInput` from
  `ListReferences` (map each to `ExplorationGraphSource`: title, decision, credibility, phaseTag,
  and State via material presence + `readingOutcomesFromCards` count → 未读/在读/已归纳),
  `ListExplorationLeads` (open texts + pruned/connected counts), and `GetProjectProposal`
  (ProposalObjective, tolerate missing). If `!agent.HasGraphContent(in)` → 200
  `{directions:[]}` with NO resolve/compose/meter. Else resolve `ChatResolver`; call
  `ComposeExplorationGuide`; **meter only when `resolved.Provider != ""`** via `RecordLLMCall`
  (surface `"studio"`, purpose `"exploration_guide"`, tokens from usage); on compose error → 200
  `{directions:[]}` (never 500). No persistence.

- [ ] **Step 4: Register route** `POST .../exploration/guide` → `a.postExplorationGuide`.

- [ ] **Step 5: Run — expect pass.** `go test ./internal/api/ -run Exploration -v` (timeout 600000).
  Then `go build ./...`.

- [ ] **Step 6: Commit.** `feat(s3): metered exploration guide handler (克制, empty=no-spend)`

---

## Task 7: Spine projection — exploration line

**Files:**
- Modify: `apps/api/internal/api/projectcoach.go` (`buildSpineProjection`, 文献库 block)
- Test: `apps/api/internal/api/projectcoach_projection_test.go` (add a case)

**Interfaces:**
- Consumes: `ListExplorationLeads`, `computeDanglingSourceIds` (Task 4 — reuse the exported-for-test
  seam or call the pkg-internal helper directly since same package).

- [ ] **Step 1: Write a failing test** (extend the projection test, use
  `BuildSpineProjectionForTest`): seed a project with 2 open leads + 1 dangling source → projection
  contains `探索：待追 2 条线索 · 1 个悬空来源`; a project with zero of both → projection contains NO
  `探索：` line.

- [ ] **Step 2: Run — expect fail.**

- [ ] **Step 3: Implement.** In the 文献库 block of `buildSpineProjection`, after the per-source
  lines: fetch `ListExplorationLeads` (best-effort), compute open-lead count and dangling count
  (reuse `computeDanglingSourceIds` with the refs already loaded in the block + the leads; if refs
  aren't in scope there, load `ListReferences` once). If `open>0 || dangling>0`, append
  `fmt.Fprintf(&b, "探索：待追 %d 条线索 · %d 个悬空来源\n", open, dangling)`. Omit when both zero.
  Keep the best-effort posture (errors → skip the line, never fail the projection).

- [ ] **Step 4: Run — expect pass.** `go test ./internal/api/ -run Projection -v` (timeout 600000).

- [ ] **Step 5: Commit.** `feat(s3): project exploration state into coach spine`

---

## Task 8: Frontend client — api/exploration.ts

**Files:**
- Create: `apps/web/src/api/exploration.ts`
- Test: (covered via Task 10's component test; no standalone test required for thin client)

**Interfaces:**
- Consumes: `apiFetch` from `./client`; contracts `ExplorationView`, `ExplorationLead`,
  `ExplorationGuide` (Task 2).
- Produces: `getExploration`, `createLead`, `patchLead`, `deleteLead`, `digDeeper` (spec §6a).

- [ ] **Step 1: Implement** mirroring `api/reading.ts` (Zod-parse each response):
  - `getExploration(projectId)` → GET `/projects/{projectId}/exploration` → `ExplorationView.parse`.
  - `createLead(projectId, text)` → POST `/exploration/leads` `{text}` → `ExplorationLead.parse`.
  - `patchLead(projectId, lid, patch)` → PATCH `/exploration/leads/{lid}` → `ExplorationLead.parse`.
    `patch` type = `{text?: string; status?: LeadStatus; connectedReferenceId?: string | null}`.
  - `deleteLead(projectId, lid)` → DELETE → void.
  - `digDeeper(projectId)` → POST `/exploration/guide` `{}` → `ExplorationGuide.parse`.

- [ ] **Step 2: Typecheck.** `pnpm -C apps/web tsc --noEmit` (or the repo's web typecheck) → clean.

- [ ] **Step 3: Commit.** `feat(s3): web exploration api client`

---

## Task 9: Exploration view + library view-mode toggle

**Files:**
- Create: `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx`
- Modify: `apps/web/src/workspace/blocks/ReadingBlock.tsx` (add 列表⇄探索图谱 toggle + mount)
- Test: (Task 10)

**Interfaces:**
- Consumes: Task 8 client; the reference list ReadingBlock already holds (for the connect picker);
  existing chip/badge styling in ReadingBlock (reuse, don't reinvent).

- [ ] **Step 0: Confirm the standalone-card path (D-S3-6).** Grep the runtime for a way to open a
  card by id and log its envelope WITHOUT the reading summon/anchor loop (check
  `apps/web/src/studio/**` card modal / TeachingModal / CardRuntime for a standalone entry). If one
  exists cleanly, include a 「兔子洞·兴趣雷达」 button that opens the `rabbit-hole` card. If not, OMIT
  it and record in the report + ledger that it's deferred to S4. Do not build a new summon loop.

- [ ] **Step 1: Build `ExplorationView`** (spec §6b). Props: `{ projectId, references, onEnterReading? }`.
  On mount, `getExploration`. Render:
  - **Sources with branches:** for each reference that has `takeaway`-origin leads, a node card
    (title + phaseTag chip + state badge, reuse ReadingBlock styling) with its leads beneath as
    chips; each open lead chip → [连接来源] (opens a reference picker from `references`) / [剪枝]
    (`patchLead status:"pruned"`); connected/pruned leads render with a marker.
  - **悬空来源 tray:** references whose id ∈ `danglingSourceIds` — a nudge line + [连接] / [剪枝].
  - **待追线索 tray:** open leads with no source (`sourceReferenceId==null`), prunable; a
    「+ 手动添加线索」 text input → `createLead`.
  - **深挖一层 panel:** a button → `digDeeper()` → render `directions` as cards (direction + why),
    each with [记为线索] → `createLead(direction)`. Loading + empty states. Nothing auto-added.
  - Re-fetch or optimistically update after each mutation so the graph stays live.
  - Real Phoebe/中国可持续 copy in any placeholder/empty text; never lorem ipsum.

- [ ] **Step 2: Add the toggle** in `ReadingBlock`: a 列表 ⇄ 探索图谱 segmented control at the block
  header; 列表 = today's list, 探索图谱 = `<ExplorationView .../>`. Default 列表. Pass the references
  ReadingBlock already has.

- [ ] **Step 3: Typecheck + build.** `pnpm -C apps/web tsc --noEmit` and the web build → clean.

- [ ] **Step 4: Commit.** `feat(s3): 探索图谱 view — branches, dangling, dig-deeper`

---

## Task 10: Web test for the exploration view

**Files:**
- Create: `apps/web/src/workspace/blocks/exploration/ExplorationView.test.tsx`

**Interfaces:**
- Consumes: the web test harness (grep for the existing vitest + testing-library setup in
  `apps/web`; mock `api/exploration` module).

- [ ] **Step 1: Write tests** (mock the exploration client):
  - renders source branches: given a view with one `takeaway` lead under a reference, the lead text
    and [剪枝] appear.
  - prune flow: clicking [剪枝] calls `patchLead` with `status:"pruned"`.
  - dig-deeper: clicking 深挖一层 calls `digDeeper`, renders returned directions, and [记为线索]
    calls `createLead` with the direction text.
  - dangling: a reference id in `danglingSourceIds` renders in the 悬空来源 tray.

- [ ] **Step 2: Run — expect pass.** The repo's web test command (grep `package.json` scripts;
  likely `pnpm -C apps/web test`). Then run the FULL web suite → all green.

- [ ] **Step 3: Commit.** `test(s3): exploration view coverage`

---

## Final (after Task 10)

Whole-branch review on the most capable model (the S2 whole-branch review caught a Critical every
per-task review missed — do not skip). Then `superpowers:finishing-a-development-branch`. Ship
disposition mirrors S2 unless the user says otherwise: **merge to main, hold prod deploy** (0040
not applied to prod). Deploy runbook goes in the ledger.

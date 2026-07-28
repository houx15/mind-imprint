# Writing Studio Redesign · Build Breakdown (Iteration Plan)

> Date: 2026-07-29 · Companion to `docs/2026-07-29-writing-studio-redesign-prd.md` (the spec) and the clickable prototype `apps/web/src/proto/` (the visual/interaction source of truth).
> This doc is the **slice plan** for turning the four-room prototype into a real, fully-functional, backend-wired studio that **replaces** the old six-station studio. The Reading Room (`apps/web/src/studio/reading/*`) is **kept and reused**, never rewritten.

Autonomous build: decisions for anything undecided follow the user's principles — (1) not in prod, ignore backwards-compat; (2) follow the prototype, every control must actually work and every list/plan/gantt must be editable & persisted; (3) retire the old studio but keep the Reading Room; entering a reference goes to the Reading Room; (4) correct persisted data flow is the priority; (5) export formats aligned to the real student deliverables.

---

## 0. Architecture decisions (locked)

| # | Decision |
|---|---|
| A1 | **New module `apps/web/src/workspace/`** (four self-contained rooms) replaces the studio tab. `StudentApp` `studio` tab → `WorkspaceContainer`. |
| A2 | **Retire** `apps/web/src/studio/views/*`, `StudioShell.tsx`, `StationRail.tsx`, `ViewFrame.tsx`, `StudioContainer.tsx`, `Directory.tsx`, and station-only card hosts / material panels. **Keep** `studio/reading/*`, `cards/*` (used by reading room), `conversation.ts` only if reused, the whole `api/*` client, and the `mk-*` Tailwind tokens (already defined in `apps/web/tailwind.config.ts`). |
| A3 | **Real backend persistence** for every room (this is the hard part = "correct data flow"). New Postgres tables + sqlc queries + `/api/v1` handlers. Draft reuses existing `edit_buffer`+`draft_snapshot`; deep reading reuses `material` + the reading-room endpoints. |
| A4 | **Room-scoped `/coach` SSE endpoint** — a restrained, one-question-at-a-time coach turn scoped by room (`forming` / `find_sources` / `writing`), reusing the gateway (`ChatResolver` mid-tier) + `RecordLLMCall`, **without** the retired station/gate machinery. The Reading Room's deep co-reading keeps its existing real gateway (`read-turn` / `summon-card` / `evaluate`). Thinking-card summoning in the Write room is a deferred hook (PRD §6 open decision) — no new agent-brain logic this round. |
| A5 | **Contracts**: new Zod files in `packages/contracts/src/`, re-exported via the barrel; TS types via `z.infer`. Go mirrors DTOs by hand where server-projected (camelCase, `.strict()` on wire mirrors) following existing conventions. |
| A6 | **Assessment continuity**: `finishProject` still generates the evaluation, but `buildAssessmentInputFromProject` is rewired to read the NEW sources (proposal / plan / library / outline / draft / reflection-doc / reading outcomes / card_instances) instead of the retired station fields. Teacher/parent/growth reports keep working unchanged downstream. |
| A7 | **Exports** produced client-side as real files aligned to the school deliverables: **.xlsx** for tables (annotated bibliography ← `资源评估表.xlsx`; plan/timescale ← `timescale.xlsx`), **.docx** for prose (proposal ← `P+A.docx` §1–§4; activity log ← `record form.docx`). Fall back to CSV/Markdown only if a doc lib isn't viable. Verify columns/sections against `docs/reference/real student essay writing/`. |

### Data model (new tables, goose migrations in `apps/api/internal/store/migrations/`)

- `project_proposal(project_id PK/FK, objective, reason, activities, resources, updated_at)` — the four kick-off dimensions (replaces old onboarding/framing/perspectives as the kickoff).
- `plan_item(id, project_id FK, title, tag CHECK(read|write|review), col CHECK(todo|doing|done), stage, ref_material_id NULL, start_day int, days int, position, created_at, updated_at)`.
- `collection(id, project_id FK, name, parent_id NULL self-FK, position, created_at)`.
- `reference(id, project_id FK, title, classification, author, credentials, year, url, tags jsonb, collection_id NULL FK, credibility NULL CHECK(strong|mixed|weak), evaluation, decision NULL CHECK(use|maybe|drop), pending bool, search_hints jsonb, material_id NULL FK, position, created_at, updated_at)` — the library row; `material_id` links to readable content, created lazily on first Reading-Room entry via existing `ingestMaterial`. Reading "notes" (quote→finding) are **projected** from card_instances/reading-outcomes anchored to `material_id`, not stored here.
- `outline_node(id, project_id FK, text, depth int, position, created_at, updated_at)`.
- `activity_log_entry(id, project_id FK, entry_date date, text, source CHECK(auto|me), created_at)` — auto rows appended by platform actions; manual rows via POST.
- `project_reflection(project_id PK/FK, answers jsonb, done bool, updated_at)` — the 5 dimensioned answers.
- `project_mirror_prose(project_id PK/FK, sections jsonb, carry_forwards jsonb, model, tier, created_at)` — generated once, first-open-wins (parent-report pattern).

### Endpoint surface (new, all under `/api/v1/projects/{id}`)

| Room | Endpoints |
|---|---|
| projection | `GET /projects/{id}` extended → `{title, qualification, proposal, roomsMeta}` |
| Project Mgmt | `PUT /proposal`; `GET /plan`, `POST /plan/items`, `PATCH /plan/items/{iid}`, `DELETE /plan/items/{iid}`; `GET /log`, `POST /log` |
| Read library | `GET /library`; `POST /collections`, `PATCH /collections/{cid}`, `DELETE /collections/{cid}`; `POST /references`, `PATCH /references/{rid}`, `DELETE /references/{rid}`; `POST /references/{rid}/enter-reading` (ensure material, return MaterialSource) |
| Write | `GET /outline`, `PUT /outline` (bulk replace — simplest correct reorder/indent); draft via existing `PUT /buffer` + `POST /snapshots` (+ upload→snapshot) |
| Review | `GET /reflection-doc`, `PUT /reflection-doc`; `GET /mirror` (no LLM call), `POST /mirror` (spend once, first-open-wins); finish via existing `POST /finish` |
| Coach | `POST /coach` (SSE), body `{scope, user_input}` |

Every spend endpoint gates with `HasEntitlement`, meters with `RecordLLMCall`. Every read endpoint is call-free (GET-no-call). Generated prose follows first-open-wins / POST-only-spend.

---

## Slices

Each slice: **spec → subagent build → verify (typecheck / build / tests) → integrate**. Slices are ordered; 2–5 each deliver one fully-working room.

### Slice 1 · Foundation — contracts + schema + shell + retirement
**Goal:** the app runs on the new four-room shell; old studio gone; all new tables + contracts exist; rooms render (ported prototype UI, local state) so nothing is broken.
- `packages/contracts`: new Zod files — `proposal.ts`, `planItem.ts`, `collection.ts`, `reference.ts`, `outlineNode.ts`, `activityLog.ts`, `reflectionDoc.ts`, `mirror.ts`, `workspace.ts` (projection) — barrel re-exports; vitest for each.
- `apps/api`: all new migrations (tables above) + models via `make sqlc` (CGO_ENABLED=0). No handlers yet beyond extending `GET /projects/{id}` to include proposal.
- `apps/web`: new `workspace/` — `WorkspaceContainer.tsx` (left rail + room swap + reading-room swap + directory + create), `Icon.tsx`/`BLOCK_META` (ported), four blocks ported from `proto/blocks/*` running on **local state** initially. Wire `StudentApp` studio tab → `WorkspaceContainer`. Delete retired studio files.
- **Verify:** `pnpm --filter web build` + `pnpm --filter contracts test`; `go build ./...` + `make sqlc`; app boots, four rooms switch, create project works, Reading Room still opens.

### Slice 2 · Project Management room (full stack)
- Backend: `PUT /proposal`; `plan_item` CRUD (`GET/POST/PATCH/DELETE`); `activity_log` `GET`/`POST` + auto-seed helper `appendAutoLog(projectId, text)`; `/coach` scope `forming`.
- Frontend `workspace/blocks/PlanBlock.tsx`: forming phase — chat via `/coach` (real SSE), four proposal dims editable & **persisted** (debounced PUT), 中/EN toggle, "生成项目计划" flips to working phase (and seeds nothing destructive), "写开题报告（可选）" writer (student-written, persists dims, **export .docx**). Working phase — kanban drag-between-columns **persists** (PATCH col), gantt drag-move + resize-duration **persists** (PATCH start/days), add-task persists, doorway click → room. Activity log view (auto + mine), 记一笔 persists, **export .xlsx (plan/timescale) + .docx (log)**.
- **Verify:** Go handler tests for proposal/plan/log; web build; manual/Playwright drag persists across reload.

### Slice 3 · Read room / Library (full stack)
- Backend: `GET /library` (collections + references, with projected reading-notes per reference); collections CRUD; references CRUD; `POST /references/{rid}/enter-reading` (lazy-create `material` via `ingestMaterial`, return `MaterialSource`); `/coach` scope `find_sources`.
- Frontend `workspace/blocks/ReadingBlock.tsx`: collections rail (fold/expand, drag-a-reference-onto-collection **persists**), reference table (multi-select, batch **export annotated-bib .xlsx**, drag rows), thin preview (all metadata edited in place & **persisted**: title/author/classification/date/url/credentials/tags/decision/credibility/evaluation), add-source modal (link/DOI/upload/manual → POST), floating coach, empty state, **"进入阅读室" → existing `ReadingRoom`** (via enter-reading). Annotated-bib columns aligned to `资源评估表.xlsx`.
- **Verify:** Go tests for library CRUD + enter-reading; ReadingRoom opens from library and returns; web build.

### Slice 4 · Write room (full stack)
- Backend: `GET /outline` + `PUT /outline` (bulk); draft reuses `PUT /buffer` (write-here) + `POST /snapshots` (upload / commit); `/coach` scope `writing`.
- Frontend `workspace/blocks/WritingBlock.tsx`: goal strip (proposal.objective + "看开题 →" jumps to Project Mgmt); Outline tab — bullets ⇄ mind-map over the **same persisted data**, both editable (edit/indent/promote/add/delete → PUT), two-way synced; Write tab — 写在这里 (Markdown + word count, autosave to buffer) ⇄ 我在别处写了 (upload Word/PDF/MD → snapshot); AI rail via `/coach`. Card-summon hook stubbed for future.
- **Verify:** Go tests for outline; buffer autosave + reload; web build.

### Slice 5 · Review room (full stack) + assessment rewire
- Backend: `GET/PUT /reflection-doc` (5 answers + done); `GET /mirror` (no-call) / `POST /mirror` (spend once, first-open-wins) — mirror sections + carry-forwards composed from the whole process (flagship `EvalResolver`); "完成回顾" → existing `POST /finish` generates the evaluation. **Rewire `buildAssessmentInputFromProject`** to read proposal/plan/library/outline/draft/reflection-doc/reading-outcomes/card_instances (assessment continuity, A6). Verify growth/teacher/parent reports still read it.
- Frontend `workspace/blocks/ReviewBlock.tsx`: five dimensioned prompts (goal anchored to `proposal.objective`), answers **persist**; 完成回顾 archives + triggers assessment (nothing graded to the student); mirror pane (sections + 带走这两点) from `/mirror`.
- **Verify:** Go tests for reflection-doc + mirror first-open-wins + finish→assessment→growth; web build.

### Slice 6 · Cross-cutting — exports, auto-log, data-flow integration, cleanup
- Real **export formats** finalized & verified against `docs/reference/real student essay writing/` (annotated-bib.xlsx, plan/timescale.xlsx, proposal.docx, activity-log.docx/xlsx).
- **Auto activity-log seeding** wired across actions: proposal generated, source opened (reading room), draft snapshot committed, task moved, reading card summoned/submitted.
- **Doorway + data-flow**: plan card (read/write/review) → matching room; reading outcomes → reference notes / annotated-bib relevance; proposal.objective → Write goal strip + Review anchor; reference "读" plan items link `ref_material_id`.
- **Cleanup**: confirm no dead imports of retired studio; remove now-unused old station handlers/queries if safe (guard assessment input).
- **Verify:** full web + api build; grep for retired symbols.

### Slice 7 · Test engineering (function + journey) and run
- **Function-based**: Go handler tests for every new endpoint (happy + auth/ownership + validation + idempotency); contracts vitest; web unit tests for reducers (outline indent, kanban move, gantt clamp, annotated-bib rows).
- **Journey-based (E2E)**: the acceptance mainline — new project → forming chat + proposal dims → generate plan → add/drag tasks → add source → enter Reading Room → outline + draft → reflection → finish → assessment appears in growth. Use the existing live-E2E harness (`apps/web` e2e specs) + Go integration `*_test.go`.
- Run all suites (`go test ./...` with Docker; `pnpm test`; web build). Fix to green.

### Slice 8 · Redeploy
- Build web + api images; run new migrations on prod DB; redeploy per `production-deployment` memory (docker-compose db+api+web on Aliyun ECS 47.93.151.131, host nginx + certbot). Smoke-test the mainline in prod. Creds in git-ignored `.deploy-local/`.

---

## Risks / watch-items
- **sqlc on macOS** needs `CGO_ENABLED=0`; **Go tests** need Docker (testcontainers).
- The `/coach` endpoint must NOT re-enter the retired station/gate loop — it's a fresh restrained turn.
- Retiring station fields breaks `buildAssessmentInputFromProject` — Slice 5 must rewire it before removing old handlers (Slice 6).
- `getProject`/`StudioProjection` is Zod-parsed on the client — extending it must update the contract in lockstep or every load 400s.
- Reading Room props contract is fixed (`projectId`, `source: MaterialSource`, `api`, `onBack`, `onOpenLogged?`) — the library must produce a real `material_id` before entering.
- Export libs: prefer a lightweight browser-safe xlsx/docx generator already in the tree, else add one to `apps/web`.

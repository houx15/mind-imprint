# Guided Tour P6 — Reading & Writing real-scene walk · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to
> implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the read-only demo project's guided tour walk students through the real reading
room (search → AI source suggestions → reading a paper with AI) and a deeper writing room (片段
引导 card, real AI 批注, the finish/lock buttons), by seeding real data and adding two small
read-only-safe backend seams.

**Architecture:** Demo stays read-only. Seed real 批注 (`intervention` rows); make demo `dig`
return real candidates; add a GET material-source projection so the immersive `ReadingRoom` can
open read-only; seed a demo reading transcript into the room's initial messages with send
disabled; expand the tour's projects segments to spotlight the real UI (real-scene, not centered).

**Tech Stack:** Go (`net/http`, sqlc v1.27.0, goose), Postgres; React + Vite + TS; Zod contracts
(`packages/contracts`); custom tour engine (`apps/web/src/tour`); Playwright for prod smoke.

**Spec:** `docs/superpowers/specs/2026-08-23-guided-tour-p6-reading-writing-walk-design.md`

## Global Constraints

- **Demo is read-only.** Never trigger a write from the tour; the backend 403s every non-GET to
  `is_demo` projects (`projects.go:202-212`). New backend reads must be GET and demo-reachable.
- **铁律①:** AI never writes body text. Writing-room copy = AI reviews/annotates/scaffolds only.
- **No live LLM / token spend in the tour.** Use canned/seeded content (mirror `demo.go`).
- **Card `category` / card-spec edits** would restage `internal/agent` goldens — this plan does
  not touch card specs, but run `internal/agent` tests anyway if any card JSON is touched.
- **Anchors:** `[data-tour="<id>"]` convention; a step that spotlights MUST use a non-`center`
  placement (a `center` step ignores its anchor — TourRunner short-circuits).
- **Action steps:** the tour engine delegates action clicks at `document` capture phase via
  `closest(selector)` — selectors must match the real clickable element (or its ancestor).
- Commit after each task. Push to `main` at the end (per repo convention). Deploy is user-run.

---

### Task 1: Seed real AI 批注 on the demo essay

**Files:**
- Create: `apps/api/internal/store/migrations/0083_seed_demo_essay_annotations.sql`
- Reference (read, do not modify): `apps/api/internal/store/migrations/0082_seed_demo_project_finished.sql`
  (the essay `edit_buffer` `…02f0`, `doc_kind='essay'`), `apps/api/internal/agent/agentstore.go:687-738`
  (annotation row shape), `apps/api/internal/api/proposal_annotations.go:82-114` (read/DTO)
- Test: `apps/api/internal/api/proposal_annotations_test.go` (add a demo case) or nearest existing test file

**Interfaces:**
- Produces: `intervention` rows readable via `GET /projects/…0200/proposal-annotations?doc=essay`.

- [ ] **Step 1:** Read the seeded essay body in `0082` (`edit_buffer` `…02f0`). Pick 5–7 real
  sentences to annotate — include the concession段 (a `good`), the China-carbon
  counter-example (a `problem`), and 2–3 `suggest`/`good` across paragraphs. Note each
  sentence's paragraph index for `locator` ("第N段").
- [ ] **Step 2:** Write the migration. Each row:
  `INSERT INTO intervention (id, project_id, card_instance_id, type, anchor, criterion, body, level, created_at)`
  with `project_id='00000000-0000-0000-0000-000000000200'`, `card_instance_id=NULL`,
  `type='essay_annotation'`, `anchor` jsonb =
  `{"docKind":"essay","level":"sentence","nature":"good|suggest|problem","quote":"<exact sentence>","locator":"第N段"}`,
  `criterion='<level>'`, `level='<nature>'`, deterministic `id`s (`…02g0`-style pattern, valid uuid),
  fixed `created_at`. Follow `0082`'s idempotency/ordering style.
- [ ] **Step 3:** Add a test asserting `GET /projects/…0200/proposal-annotations?doc=essay`
  returns the seeded rows with correct `nature`/`quote`/`locator`/`note`, and that a non-owner
  authenticated user can read them (demo is world-readable over GET).
- [ ] **Step 4:** Run `go test ./internal/api/... -run Annotation -timeout 1800s` (CGO_ENABLED=0). Expect PASS.
- [ ] **Step 5:** Commit.

---

### Task 2: Demo literature-search returns real candidates

**Files:**
- Modify: `apps/api/internal/api/demo.go` (`cannedDig`, lines ~57-59)
- Reference: `apps/api/internal/api/exploration.go:633-640` (short-circuit), the `DigCandidate` type
- Test: existing exploration/demo test file (add a case)

**Interfaces:**
- Consumes: `DigCandidate` struct shape (find its definition).
- Produces: `POST /projects/…0200/exploration/dig` returns 3 candidates (still no 403).

- [ ] **Step 1:** Find the `DigCandidate` type and what fields the frontend `ResultsPanel`
  renders (`ExplorationSidebar.tsx:495-573`): title/source/reason/credibility etc.
- [ ] **Step 2:** Change `cannedDig()` to return 3 real, relevant candidate sources for the demo
  research question (中国是否让地球更可持续) — real-looking titles/authors/reasons, no lorem.
- [ ] **Step 3:** Add/extend a test: demo `dig` returns 3 candidates, no 403.
- [ ] **Step 4:** Run `go test ./internal/api/... -run Dig -timeout 1800s`. Expect PASS.
- [ ] **Step 5:** Commit.

---

### Task 3: Read-only material-source GET endpoint

**Files:**
- Modify: `apps/api/internal/api/api.go` (route), a handler file (e.g. `workspace_library.go` or new `material_source.go`)
- Reference: `apps/api/internal/api/workspace_library.go:675-764` (`enter-reading` projection), `packages/contracts/src/studioState.ts:111-161` (`MaterialSource`)
- Test: `apps/api/internal/api/*_test.go`

**Interfaces:**
- Produces: `GET /api/v1/projects/{id}/materials/{mid}/source` → JSON `MaterialSource` (14 fields).

- [ ] **Step 1:** Add handler `getMaterialSource`: load project row (demo-reachable via
  `loadOwnedProjectRow`, GET), project the material to `MaterialSource` **reusing the
  `enter-reading` projection minus the write / `appendAutoLog`**. Return 404 if material not in project.
- [ ] **Step 2:** Register the GET route in `api.go`. GET must NOT go through the non-GET 403 guard.
- [ ] **Step 3:** Test: for the demo material `…0271`, a non-owner GET returns a valid
  `MaterialSource` with all required fields populated from seeded blocks; a non-GET still 403s.
- [ ] **Step 4:** Run `go test ./internal/api/... -run MaterialSource -timeout 1800s`. Expect PASS.
- [ ] **Step 5:** Commit.

---

### Task 4: ReadingRoom demo replay (initial messages + disabled send)

**Files:**
- Modify: `apps/web/src/studio/reading/ReadingRoom.tsx`, `apps/web/src/studio/reading/readingLoop.ts`
- Create: `apps/web/src/tour/fixtures/demoReadingTranscript.ts`
- Test: `apps/web/test/` (a ReadingRoom demo-mode test)

**Interfaces:**
- Produces: `ReadingRoom` props `initialMessages?: ChatMessage[]`, `demoMode?: boolean`.

- [ ] **Step 1:** Add optional `initialMessages`/`demoMode` props; `readingLoop` seeds `messages`
  from `initialMessages` (fallback `[GREETING]`). When `demoMode`, disable the send input with a
  read-only hint; ensure no `read-turn`/`putReadingBrief`/note POST can fire.
- [ ] **Step 2:** Write `demoReadingTranscript.ts` — a short real exchange (2–3 turns) about the
  Nature Sustainability paper using a 思维卡 lens (e.g. CRAAP/溯源), 铁律-safe (AI helps analyse,
  doesn't write the essay). No lorem.
- [ ] **Step 3:** Test: ReadingRoom with `initialMessages`+`demoMode` renders the transcript and
  the send box is disabled.
- [ ] **Step 4:** Run the web test for this file. Expect PASS.
- [ ] **Step 5:** Commit.

---

### Task 5: Wire `openDemoReadingRoom()` to the real immersive room

**Files:**
- Modify: `apps/web/src/shell/StudentApp.tsx` (impl), `apps/web/src/workspace/WorkspaceContainer.tsx` (thread a demo-open path to `setReadingSource`), `apps/web/src/api/` (add `getMaterialSource` client), `apps/web/src/tour/types.ts` (doc comment)
- Reference: `WorkspaceContainer.tsx:298-318` (`openReadingSource`), the P5 stub at `StudentApp.tsx:165-172`
- Test: `apps/web/test/` (openDemoReadingRoom fallback + success)

**Interfaces:**
- Consumes: `getMaterialSource` (Task 3), ReadingRoom demo props (Task 4).
- Produces: `openDemoReadingRoom()` opens the real immersive `ReadingRoom` on `…0271`.

- [ ] **Step 1:** Add `getMaterialSource(projectId, materialId)` api client.
- [ ] **Step 2:** Replace the stub: `openDemoReadingRoom()` switches to the reading room, GETs the
  demo material source, calls the demo-open path into `setReadingSource` with the transcript +
  `demoMode`. On GET failure, fall back to `setReadingView("list")` (no dead-end).
- [ ] **Step 3:** Remove the P5 TODO comment blocks that describe the stub.
- [ ] **Step 4:** Test the success path (opens immersive) and the fallback path (GET fails → list).
- [ ] **Step 5:** Run the web tests for touched files. Expect PASS.
- [ ] **Step 6:** Commit.

---

### Task 6: New `[data-tour]` anchors

**Files:**
- Modify: `apps/web/src/workspace/blocks/WritingBlock.tsx` (`writing-finish` on 完成写作 btn line ~397),
  `apps/web/src/workspace/blocks/ReferencePanel.tsx` (`writing-annotations` on the AI批注 tab/group),
  `apps/web/src/workspace/blocks/ReviewBlock.tsx` (`review-finalize` on 定稿 btn line ~283),
  `apps/web/src/workspace/blocks/exploration/ExplorationSidebar.tsx` (`explore-find` on find-actions row, only if needed)
- Test: none (mechanical); covered by Task 7 segment tests + smoke

- [ ] **Step 1:** Add the four `data-tour` attributes (only add `explore-find` if `explore-keyword`
  can't spotlight the find controls). Do not change behaviour.
- [ ] **Step 2:** Run `typecheck`. Expect clean.
- [ ] **Step 3:** Commit.

---

### Task 7: Expand the projects tour segments

**Files:**
- Modify: `apps/web/src/tour/segments/projects.ts`
- Test: `apps/web/test/tour/projects-segments.test.ts` (extend)

**Interfaces:**
- Consumes: anchors from Task 6, `openDemoReadingRoom` (Task 5), `explore-suggestions`/`warren-question`/`explore-keyword`/`rr-*`/`writing-aicard` (existing).

- [ ] **Step 1:** **reading-warren:** add (a) an action step selecting a question node
  (`actionEvent` click `warren-question`), (b) a spotlight of `explore-keyword` + find controls,
  (c) an action step clicking 找相似 (`explore-find`/find-button selector) → suggestions populate,
  (d) a spotlight of `explore-suggestions` (采纳/丢弃 explanation). Copy: real-scene, one question
  per step, 铁律-safe.
- [ ] **Step 2:** **reading-room (精读):** replace the 2 centered narration steps with
  `onEnter: openDemoReadingRoom()` + spotlights of `rr-article`, `rr-chat`, `rr-deck`, `rr-notes`,
  `rr-finish` (non-center placements).
- [ ] **Step 3:** **writing:** add spotlights of `writing-aicard` (片段引导/写作卡, 铁律① copy),
  `writing-annotations` (real 批注 — green亮点/blue建议/red问题, click→jump), `writing-finish`
  (完成写作 lock copy: 锁定初稿、解锁回顾，仍可重新打开).
- [ ] **Step 4:** **reflection:** add a spotlight of `review-finalize` (定稿并开始评估 — quote:
  "定稿后，正文与回顾都会锁定、无法再修改" — the answer to "after finish you can't change your writing").
- [ ] **Step 5:** Extend `projects-segments.test.ts`: assert the new steps exist, spotlight steps
  are non-center with resolvable anchors, action steps carry `actionEvent` with real selectors,
  and no step both anchors and centers.
- [ ] **Step 6:** Run the tour tests. Expect PASS.
- [ ] **Step 7:** Commit.

---

### Task 8: Full verification + prod-style smoke

**Files:** none (verification only)

- [ ] **Step 1:** Run the full web suite (`cd apps/web && <test cmd>`) + `typecheck`. Expect green.
- [ ] **Step 2:** Run `go test ./internal/api/... ./internal/agent/... -timeout 1800s` (CGO_ENABLED=0). Expect green.
- [ ] **Step 3:** (Controller, after deploy is available or against a local stack) Playwright smoke
  the whole projects journey on the demo: reading graph search/suggestions action clicks advance;
  immersive 精读 opens with real paper + transcript; writing 片段引导 + 批注 + finish spotlights;
  reflection 定稿 warning. Confirm 0 console errors (no stray 403). Record results in the ledger.
- [ ] **Step 4:** No commit unless smoke surfaces a fix.

## Self-review notes

- Spec coverage: B1→T1, B2→T2, B3→T3, F1→T4, F2→T5, F3→T6, F4→T7, testing→T8. ✅
- Type consistency: `MaterialSource` (14 required fields) used in T3/T5; `DigCandidate` in T2;
  `ChatMessage` in T4. Implementers verify exact shapes from the cited files.
- Read-only invariant held: only GET reads added; tour triggers no writes; disabled send in demo.

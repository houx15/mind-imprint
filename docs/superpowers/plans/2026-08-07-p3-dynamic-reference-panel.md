# P3 · Dynamic AI-curated reference panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Turn the writing room's static 3-tab reference sub-pane into 印记's **dynamic, per-stage** left reference panel — driven by the already-stored `studio_state.reference[]`, resolving each ref to rich content, folding the floating 材料 box in, gating 提案要点 by stage — so what the student sees to refer to is what 印记 curated for *this* step.

**Architecture:** The backend already stores `reference: ReferenceRef[]{kind,id,label}` and applies `curate_reference`. P3 (a) feeds real ids into the orchestrator projection so 印记 can cite materials/notes by id (+ drops hallucinated ids), and (b) builds a frontend `ReferencePanel` that reads `studioState.reference`, resolves `material`/`note` ids against `getLibrary`/`getSnippets`, renders grouped-by-kind, adds 提案要点 gated by stage (写提案 shows, 写正文 de-emphasizes), folds the materials browse+insert in, and renders `annotation` gracefully (the annotation ENTITY is a documented deferral — see Global Constraints).

**Tech Stack:** Go (`apps/api`) · Zod contracts (`packages/contracts`) · React+TS+Tailwind (`apps/web`). Branch `studio-p2-focused-but-free` (on top of P2b @`ee4f323`).

## Global Constraints

- **Spec of record:** `docs/superpowers/specs/2026-08-07-agentic-studio-orchestrator-redesign.md` §5 (dynamic reference; materials folded in; 提案要点 only-when-relevant; 批注 only after 体检; 写提案 vs 写正文 = different sets).
- **SCOPE DECISION — annotation entity DEFERRED:** there is no annotation persistence today (整稿体检 output is transient, unaddressable). Building a persisted, addressable annotation entity is its own phase. P3's `ReferencePanel` MUST render `kind:"annotation"` items gracefully **if present**, and show a calm "批注会在印记体检后出现" state otherwise — but P3 does NOT build the annotation entity/persistence. Document this clearly; do not fake annotations.
- **SCOPE — writing room only.** §5's reference panel is the WRITING reference. P3 makes the writing room's left panel dynamic + per-stage; it does NOT add reference panels to plan/reading/reflection.
- **DESIGN PRINCIPLES (enforced — the user flagged violations):** every piece of NEW/edited UI in this phase MUST follow `docs/superpowers/specs/2026-08-06-design-system-design.md`: **regular/body text ≥ 14px (16px for reading/writing body); 12px is the FLOOR and only for hints/meta/caption; uppercase Labels 12px min. NO text below 12px.** Empty states = **vertically + horizontally centered illustration (from `apps/web/src/assets/illustrations/`) + a ≥14px title + ≤one-sentence + optional action.** Never `bg-mk-<token>/<opacity>` / `border-mk-<token>/<NN>` (transparent). One Tailwind class per competing property.
- **Contracts single source of truth:** new reply/projection fields go in `packages/contracts` (client) AND the Go structs or strict parse throws.
- **Go on macOS:** `CGO_ENABLED=0`; sqlc pin `@v1.27.0`; **Go tests FOREGROUND only** (testcontainers stall backgrounded agents); prefer `./internal/agent` (no docker); the docker `internal/api` suite runs once, scoped (`-run`), foreground. Pre-existing fails `TestProjectsEndpoints`/`TestWeeklyReportForSeededClass` are NOT ours.
- **Web:** pnpm; tests `apps/web/test/**`; `pnpm exec tsc --noEmit` 0 + `pnpm exec vitest run`. Never `git add -A`.

---

## File Structure
- `apps/api/internal/api/projectcoach.go` — `buildSpineProjection`: add stable ids (references + snippets) to the 文献库 block (T1).
- `apps/api/internal/agent/orchestrator.go` — prompt: per-stage curate guidance; `filterCurateReferenceCall`: id-existence validation (T1).
- `apps/web/src/workspace/blocks/ReferencePanel.tsx` — NEW dynamic panel (T2/T3).
- `apps/web/src/workspace/blocks/WritingReferencePanel.tsx` — replaced by ReferencePanel (retired) (T3).
- `apps/web/src/workspace/WorkspaceContainer.tsx` — pass `studioState.reference` + `stage` into the writing SplitPane-left (T3).
- `apps/web/src/workspace/blocks/WritingBlock.tsx` + `MaterialsSidebar.tsx` — fold the floating materials into the panel; retire the floating box (T4).

---

## Task 1: Backend — real ids in the projection + id-validated curation + per-stage prompt

**Files:** `apps/api/internal/api/projectcoach.go`, `apps/api/internal/agent/orchestrator.go`; Tests `apps/api/internal/agent/*_test.go` (no docker) + scoped `internal/api` (docker, foreground).

**Interfaces — Produces:** the 文献库 projection block lists each reference/material with a STABLE id the model can cite (`curate_reference` `id`); `filterCurateReferenceCall` drops items whose id matches no known material/note id; prompt guides per-stage curation (写提案 → 提案要点 + sources; 写正文 → sources/annotations, 提案要点 not forced).

- [ ] **Step 1:** In `buildSpineProjection` (`projectcoach.go`, the 文献库 block ~line 277-335), include each reference's stable id alongside its title (e.g. a `[id]` the prompt explains the model may pass to `curate_reference`). Include snippet ids too (currently snippets aren't in the projection — add a compact 片段 line list with ids). Keep it token-lean (ids + short label; you already truncate titles).
- [ ] **Step 2:** In `orchestrator.go` `filterCurateReferenceCall`, add an allowed-id set (materials/references + notes + snippets ids for THIS project — thread them from the projection build or a lightweight query) and drop `curate_reference` items whose id is not in it (keep the existing kind-validation). This prevents hallucinated ids reaching `state.Reference`. **If threading the id-set is too invasive for one task, at minimum validate the `kind` (existing) and log-drop on empty id — but prefer real id-validation.**
- [ ] **Step 3:** Prompt: extend the `curate_reference` bullet + closing principle so 印记 curates per stage: during `proposal_writing`/`proposal_review` surface 提案要点-relevant sources; during `body_writing` surface sources/annotations and does NOT force 提案要点. One or two concise Chinese lines, same voice.
- [ ] **Step 4:** Tests — an agent-package test that `filterCurateReferenceCall` drops an unknown-id item and keeps a known-id one; a scoped docker test (foreground) that a `curate_reference` turn persists only valid ids into `studio_state.reference`. `go build`, `go test ./internal/agent/`, scoped `internal/api -run` foreground.
- [ ] **Step 5:** Commit `feat(api): real ids in coach projection + id-validated curate_reference + per-stage prompt (P3)`.

---

## Task 2: Web — reference resolution helper + contract wiring

**Files:** `apps/web/src/workspace/blocks/referenceResolve.ts` (NEW); Test `apps/web/test/workspace/blocks/referenceResolve.test.ts`.

**Interfaces — Produces:** `resolveReferences(refs: ReferenceRef[], lib: Reference[], snippets: Snippet[]): ResolvedRef[]` where `ResolvedRef` groups by `kind` and carries the rich content (a material's title+takeaway+notes, a note's quote→finding, a snippet's text) or a `missing` flag when the id no longer resolves. Pure function, unit-tested.

- [ ] **Step 1:** Write the failing test: given a `reference[]` with a material id present in `lib`, a note id, a snippet id, and a dangling id → returns resolved rich entries + one `missing`. (Use the `referenceChunks` logic in `MaterialsSidebar.tsx:489-503` as the reference for how to flatten a `Reference` into fragments.)
- [ ] **Step 2:** Implement `resolveReferences` (pure). `ReferenceRef` from `@mind-imprint/contracts`; `Reference`/`Snippet` too.
- [ ] **Step 3:** vitest + tsc green. Commit `feat(web): resolveReferences helper for the dynamic reference panel (P3)`.

---

## Task 3: Web — the dynamic ReferencePanel (replaces the static tabs)

**Files:** `apps/web/src/workspace/blocks/ReferencePanel.tsx` (NEW), `apps/web/src/workspace/WorkspaceContainer.tsx`, retire `WritingReferencePanel.tsx`; Tests.

**Interfaces — Consumes:** `studioState.reference`, `studioState.stage`, `proposal`, `getLibrary`/`getSnippets`, `resolveReferences`. **Produces:** a panel that renders (a) 印记-curated resolved refs grouped by kind (materials/notes/annotations), (b) 提案要点 gated by stage (`proposal_writing`/`proposal_review` → shown; `body_writing` → collapsed/omitted), (c) a calm "批注会在印记体检后出现" state when no annotations.

- [ ] **Step 1:** Build `ReferencePanel({ projectId, reference, stage, proposal })`: fetch library+snippets once (lazy), `resolveReferences`, render groups. Lift `ProposalTab`/`ReadingTab` renders from `WritingReferencePanel.tsx` for the proposal-points + note rendering. **Font compliance:** section headers ≥14px, body ≥14px, only meta/caption 12px; NO sub-12px. Empty state (no refs curated yet): centered illustration (pick an apt one from `apps/web/src/assets/illustrations/`, e.g. reading-notes) + ≥14px title + one sentence.
- [ ] **Step 2:** Per-stage: show 提案要点 group only when `stage` ∈ {proposal_writing, proposal_review} (or proposal has content and stage is pre-body); in `body_writing` omit or collapse it (spec: 写正文 don't force 提案要点).
- [ ] **Step 3:** `WorkspaceContainer.tsx`: in the writing `SplitPane` left, replace `<WritingReferencePanel .../>` with `<ReferencePanel projectId reference={studioState?.reference ?? []} stage={studioState?.stage ?? "body_writing"} proposal={workspace.proposal} />`. Retire `WritingReferencePanel.tsx` (delete + its test, or repoint).
- [ ] **Step 4:** Tests — panel renders curated material/note groups from a mocked `reference[]`+library; 提案要点 shows in proposal stage, hidden in body_writing; empty state renders the illustration + ≥14px copy. Full suite + tsc green.
- [ ] **Step 5:** Commit `feat(studio): dynamic per-stage ReferencePanel driven by 印记's curated reference (P3)`.

---

## Task 4: Web — fold the floating 材料 box into the panel; retire the float

**Files:** `apps/web/src/workspace/blocks/ReferencePanel.tsx`, `apps/web/src/workspace/blocks/WritingBlock.tsx`, `apps/web/src/workspace/blocks/MaterialsSidebar.tsx`; Tests.

**Rationale (spec §5):** the floating draggable 材料 box moves INTO the left reference panel (不再浮动). Its browse + insert-into-draft affordance becomes a section of `ReferencePanel`.

- [ ] **Step 1:** Add a 材料 section to `ReferencePanel` that lists the library materials (reuse `referenceChunks`) with the insert-into-draft / add-snippet actions the floating box had (thread the `onInsertToDraft`/`onAddSnippet` callbacks from WritingBlock into ReferencePanel). Font ≥14px; the insert affordance is a real action (button ≥14px), the fragment previews may be 12px meta.
- [ ] **Step 2:** Remove the floating `<MaterialsSidebar />` mount from `WritingBlock.tsx` (and delete `MaterialsSidebar.tsx` + its test if fully superseded; otherwise strip to the reused chunk-helper). Keep `referenceChunks` (export it for reuse).
- [ ] **Step 3:** Tests — the materials section renders + insert action fires `onInsertToDraft`; the floating box is gone. Full suite + tsc green.
- [ ] **Step 4:** Commit `feat(studio): fold the floating 材料 box into the left ReferencePanel (P3)`.

---

## Self-Review Checklist
- **Spec §5:** dynamic (reads `reference[]`) ✓ · per-stage (提案要点 gated) ✓ · materials folded in (no float) ✓ · 批注 graceful-deferred ✓ (annotation entity documented as a follow-up, NOT faked).
- **Backend:** 印记 can now cite real ids (projection) + hallucinated ids dropped.
- **Design principles:** all new panel text ≥14px; 12px only for meta/hints; empty state = centered illustration + ≥14px copy. (The whole-studio font sweep is a later phase; this phase must not ADD violations.)
- **No orphaned code:** WritingReferencePanel + floating MaterialsSidebar retired cleanly (or reduced to the reused helper).
- **Green:** web tsc 0 + vitest; Go build/vet/agent + scoped internal/api foreground.
- **DEFERRED (documented, flag for user):** the persisted annotation entity (批注 after 体检) — needs its own contract/table/review-produces-annotations design.

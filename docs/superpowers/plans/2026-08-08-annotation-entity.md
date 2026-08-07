# Annotation entity (批注 after 体检) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Close the P3 deferral — make 印记's 整稿体检 (whole-draft review) items **addressable annotations** so 印记 can `curate_reference` them (`kind:"annotation"`) and the ReferencePanel's 批注 group renders them (spec §5: 批注 appears only after 印记 has reviewed the writing).

**Architecture (Route A — reuse existing persistence, NO migration):** Review items are ALREADY durable — persisted as `intervention` rows (`type='review_item'`, the full `agent.ReviewItem` JSON in `intervention.body`, one per criterion). Today their ids are discarded and never surfaced. This plan exposes them as annotations: (1) a `ListReviewItemsByProject` query + a `GET /projects/{id}/annotations` endpoint returning `{id, criterion, band, text}`; (2) a 批注 block in the orchestrator projection listing `[id]` so 印记 can cite them + `filterKnownReferences` allows those ids; (3) a frontend `getAnnotations` + `resolveReferences` annotation branch + `AnnotationGroup` render. The contract (`kind:"annotation"`) and the `ResolvedRef` annotation variant ALREADY exist — no type-surface change.

**Tech Stack:** Go (`apps/api`, sqlc) · Zod contracts (`packages/contracts`) · React/TS (`apps/web`). Branch off `main`.

## Global Constraints
- **Route A (reuse `intervention`), NO new table / migration.** Review items live as `intervention` rows `type='review_item'` (persisted in `writing.go`'s `orderReview` → `InsertReviewIntervention`, `agentstore.go`). Expose, don't duplicate.
- **Design principles (enforced):** any new/edited UI ≥14px body, 12px only for meta/hints, no sub-12px; never `bg-mk-<token>/<opacity>` / `border-mk-<token>/<NN>`; one Tailwind class per property.
- **铁律:** annotations are 印记's read-only feedback on the student's writing (not written FOR her); they only exist after a 体检 ran; 印记 curates which to keep in view (student never forced).
- **Contracts single source of truth.** The `ReferenceRef` `kind:"annotation"` + `ResolvedRef` annotation variant already exist; a new `Annotation` DTO for the `getAnnotations` response goes in `packages/contracts` + Go.
- **Go on macOS:** `CGO_ENABLED=0`; sqlc pin `@v1.27.0` (`CGO_ENABLED=0 go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate`); Go tests FOREGROUND (docker) — prefer `./internal/agent`; scoped `./internal/api -run` foreground. Pre-existing `TestWeeklyReportForSeededClass` flaky (ignore).
- Web: pnpm; tests `apps/web/test/**`; tsc 0 + vitest. Never `git add -A`.

---

## File Structure
- `apps/api/internal/store/queries/intervention.sql` — `ListReviewItemsByProject :many` (+ sqlc regen) (T1).
- `apps/api/internal/api/annotations.go` (NEW) — `GET /projects/{id}/annotations` handler + DTO; route in `api.go` (T1).
- `apps/api/internal/api/projectcoach.go` — `buildSpineProjection` 批注 block + `filterKnownReferences` allows review-item ids (T2).
- `packages/contracts/src/annotation.ts` (NEW) + barrel — `Annotation` DTO (T3).
- `apps/web/src/api/workspace.ts` (or writing.ts) — `getAnnotations(projectId)` client (T3).
- `apps/web/src/workspace/blocks/referenceResolve.ts` — resolve annotation kind from an annotations array (T4).
- `apps/web/src/workspace/blocks/ReferencePanel.tsx` — fetch annotations, render `AnnotationGroup` items (T4).

---

## Task 1: Backend — list review items as annotations + endpoint

**Files:** `apps/api/internal/store/queries/intervention.sql` (+ regen), `apps/api/internal/api/annotations.go` (new), `apps/api/internal/api/api.go`; Test scoped `internal/api` (docker, foreground).

**Interfaces — Produces:** `ListReviewItemsByProject(ctx, projectID) []Intervention` (rows where `type='review_item'`, ordered by created_at). `GET /api/v1/projects/{id}/annotations` → `{annotations: [{id, criterion, band, text}]}` where the item is unmarshalled from `intervention.body` (an `agent.ReviewItem`): `text` = `missing` + " → " + `fix` (the actionable 批注), `criterion` = criterion_name, `band` = band.

- [ ] **Step 1:** Add to `intervention.sql`: `-- name: ListReviewItemsByProject :many` → `SELECT * FROM intervention WHERE project_id = $1 AND type = 'review_item' ORDER BY created_at`. Regen sqlc (pinned v1.27.0), confirm `ListReviewItemsByProject` appears in `sqlc/*.sql.go`.
- [ ] **Step 2:** `annotations.go`: `annotationDTO struct { ID string; Criterion string; Band string; Text string }` (json id/criterion/band/text). `getAnnotations` handler: `loadOwnedProject` gate → `ListReviewItemsByProject` → for each row `json.Unmarshal(row.Body, &agent.ReviewItem{})` → build DTO (Text = strings.TrimSpace(item.Missing + " → " + item.Fix), or just item.Fix if Missing empty; Criterion=item.CriterionName; Band=item.Band) → `{"annotations": dtos}`. Route in `api.go`: `mux.Handle("GET /api/v1/projects/{id}/annotations", protected(a.getAnnotations))`.
- [ ] **Step 3: Test (docker, FOREGROUND, scoped):** seed a project with a review-item intervention (reuse the writing/review test harness — grep `InsertReviewIntervention`/`review_item`) → `GET /annotations` returns it with a non-empty text. `CGO_ENABLED=0 go test ./internal/api/ -run 'TestAnnotations|TestReview' -count=1` foreground.
- [ ] **Step 4: Commit** `feat(api): expose 整稿体检 review items as addressable annotations (GET /annotations)`.

---

## Task 2: Backend — annotations in the projection + curate filter

**Files:** `apps/api/internal/api/projectcoach.go`; Test `internal/api` scoped (docker, foreground) + `internal/agent` if applicable.

**Interfaces — Produces:** `buildSpineProjection` emits a 批注 block listing `[id]` for review items; `filterKnownReferences` includes review-item ids in its allowed-set so `curate_reference kind:"annotation"` survives.

- [ ] **Step 1:** In `filterKnownReferences` (projectcoach.go ~L80-99), add review-item ids to `allowed` (call `ListReviewItemsByProject`, add each `row.ID.String()`). Now a curated `kind:"annotation"` id that matches a real review item is kept.
- [ ] **Step 2:** In `buildSpineProjection` (after the 片段 block, ~L410), add a 批注 block ONLY when review items exist: header `批注（[id] 可传给 curate_reference，kind="annotation"；只有体检过才有）：` then per row `- [<id>] <criterion_name>·<band>｜<truncated fix>`. Token-lean.
- [ ] **Step 3:** Prompt note (orchestrator.go, optional, tiny): mention 印记 may curate 批注 during `body_writing`/`proposal_review` after a 体检. (Skip if it bloats — the projection header already guides.)
- [ ] **Step 4: Test (docker foreground, scoped):** a `curate_reference` turn citing a real review-item id persists it into `studio_state.reference` (extend the P3 drop-unknown test with an annotation-id-kept case). Run scoped foreground.
- [ ] **Step 5: Commit** `feat(api): 批注 in coach projection + curate_reference allows annotation ids`.

---

## Task 3: contracts + web client — Annotation DTO + getAnnotations

**Files:** `packages/contracts/src/annotation.ts` (new) + `src/index.ts`; `apps/web/src/workspace/api/workspace.ts` (or `api/writing.ts`); Test.

**Interfaces — Produces:** `Annotation = z.object({ id, criterion, band, text })`; `getAnnotations(projectId): Promise<Annotation[]>` (GET /annotations, parses `.annotations`).

- [ ] **Step 1:** contracts `Annotation` zod + type; export from barrel. **Step 2:** `getAnnotations` client (mirror `getSnippets` — GET, parse array). Test the client (mock fetch) under `apps/web/test/**`.
- [ ] **Step 3:** contracts + web tsc/vitest green. **Step 4: Commit** `feat(contracts,web): Annotation DTO + getAnnotations client`.

---

## Task 4: Frontend — resolve + render annotations in the 批注 group

**Files:** `apps/web/src/workspace/blocks/referenceResolve.ts`, `apps/web/src/workspace/blocks/ReferencePanel.tsx`; Tests.

**Interfaces — Consumes:** `getAnnotations`, `Annotation`. **Produces:** `resolveReferences(refs, lib, snippets, annotations)` resolves `kind:"annotation"` ids → `{kind:"annotation", id, label, text}` (missing when not found); `ReferencePanel` fetches annotations (like library/snippets), and `AnnotationGroup` renders resolved annotation items (≥14px) — the calm "批注会在印记体检后出现" line ONLY when there are none.

- [ ] **Step 1:** `resolveReferences` — add an `annotations: Annotation[]` param; the annotation branch (referenceResolve.ts ~L62) looks up `annotations.find(a => a.id === ref.id)` → `{kind:"annotation", id, label, text: a.text}`; else `missing:true`. Update the pure test.
- [ ] **Step 2:** `ReferencePanel` — add `getAnnotations` to the lazy fetch (`Promise.all([getLibrary, getSnippets, getAnnotations])`); pass into `resolveReferences`; compute `annotations = resolved.filter(kind==="annotation" && !missing)`; `AnnotationGroup({ items })` renders each `item.text` (≥14px) with the criterion/band as a 12px meta label; keep the "批注会在印记体检你的写作后出现。" line only when `items.length === 0`. Font ≥14px body.
- [ ] **Step 3:** Tests — ReferencePanel with a curated annotation ref + mocked getAnnotations → the 批注 group shows the annotation text; with none → the calm placeholder. Full web suite + tsc green.
- [ ] **Step 4: Commit** `feat(studio): render 印记's curated 批注 (annotations after 体检) in the ReferencePanel`.

---

## Self-Review Checklist
- **Spec §5:** 批注 appears only after 体检 (annotations only exist once review items persisted) ✓; 印记 curates which show (curate_reference) ✓; read-only feedback ✓.
- **Route A:** no new table/migration; review-item interventions exposed as annotations ✓.
- **Contract/type:** reused existing `kind:"annotation"` + `ResolvedRef` variant; new `Annotation` DTO both sides ✓.
- **End-to-end id flow:** intervention id → /annotations + projection `[id]` → 印记 curate_reference → filterKnownReferences keeps it → studio_state.reference → frontend resolve+render ✓.
- **Design:** annotation text ≥14px, criterion/band 12px meta; placeholder only when empty ✓.
- **Green:** web tsc 0 + vitest; Go build/agent + scoped internal/api foreground.

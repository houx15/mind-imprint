# Guided Tour — P2 (Demo Project) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** One shared, read-only, hand-authored **finished** demo writing-project (all five rooms + a valid evaluation report), plus a backend **demo-mode guard** so it can never be mutated or burn LLM tokens — the vehicle the P3 projects tour walks through.

**Architecture:** Add `project.is_demo`. A single method-aware guard in the existing `loadOwnedProject` chokepoint rejects all writes to demo projects; token-consuming endpoints short-circuit to canned fixtures (correlated with the existing `HasEntitlement` seam); the evaluation report is a frozen seed row (no live 4-call generation). The demo content is a `0081` seed migration on the canonical China-sustainability topic. The frontend reads `isDemo` off `WorkspaceProjection` and suppresses obvious write affordances (backend 403 is the real safety net).

**Tech Stack:** Go `net/http` + `pgx`/`sqlc` v1.27.0 + `goose`; React + Vite + TS; Zod contracts in `packages/contracts`.

**Spec:** `docs/superpowers/specs/2026-08-22-new-user-guided-tour-design.md` (§5 Demo data & Mock-AI Strategy).

## Global Constraints

- **No live LLM for the demo** — demo-mode endpoints return canned fixtures; the evaluation report is a seeded row (`status='ready'`), never generated.
- **Demo is read-only** — every write to a demo project is rejected/no-op'd; the shared demo can never be mutated by any viewer.
- **Content-quality bar (hard):** real research question (China → world sustainability), full-length proposal + essay (real paragraphs, not stubs), a real literature list, substantive warren map + reflections, a proper evaluation report. No toy/lorem text.
- **Demo project id = NEW dedicated `00000000-0000-0000-0000-000000000200`** (owner Phoebe `…003`), inserted + flagged `is_demo=true` in `0082`. Do NOT reuse `…0101` — it is the shared WRITE fixture for ~10 existing test files and must stay writable. Use fixed UUIDs in the `…02xx` block for all demo rows; `ON CONFLICT DO NOTHING/UPDATE` idempotency (mirror `0018`/`0020`).
- **Finished-project gating (ALL required):** `project.status='finished'`; a `writing_finish` row `doc_kind='essay'` with non-empty essay `edit_buffer`; `project_reflection.done=true`; an `evaluation_report` row `status='ready'` with a **schema-valid** `report` jsonb.
- **evaluation_report.report** must satisfy the `.strict()` Zod `EvaluationReport` (`packages/contracts/src/evaluationReport.ts:140-156`); the Go mirror is `apps/api/internal/evalreport/report.go`; server `evalreport.Validate` checks only the envelope, but the **frontend Zod is strict** — every field must be present and correctly shaped.
- **sqlc**: `cd apps/api && make sqlc` (CGO_ENABLED=0, pinned v1.27.0); nullable timestamptz → `pgtype.Timestamptz`.
- **Go tests**: `cd apps/api && go test ./... -timeout 1800s` (Docker running, testcontainers). Frontend: `npm run test` / single `npx vitest run <path>`; `npm run typecheck`.
- **Git**: stage specific files (never `git add -A`); pre-existing untracked `docs/*.md` are NOT ours; commit per task with trailer `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`; push handled by the controller.
- **Boundary hook** blocks `/dev/null` redirects and `cd` in compound commands.

---

## File Structure

**New (backend):**
- `apps/api/internal/store/migrations/0081_project_is_demo.sql` — `ALTER TABLE project ADD COLUMN is_demo boolean NOT NULL DEFAULT false;` + flag the demo row.
- `apps/api/internal/store/migrations/0082_seed_demo_project_finished.sql` — the finished-project content seed (all rooms + evaluation_report).
- `apps/api/internal/httpx/errors.go` — add `ErrDemoReadonly()` (403, code `demo_readonly`), mirroring `ErrNotEntitled`.
- `apps/api/internal/api/demo.go` (new) — small helpers: `loadOwnedProjectRow(...) (sqlc.Project, bool)` and canned-fixture builders for demo mode.
- Tests: `apps/api/internal/api/demo_test.go`, `apps/api/internal/api/demo_seed_test.go`.

**Modified (backend):**
- `apps/api/internal/api/projects.go` — `loadOwnedProject` method-aware write-guard; `workspaceProjection` + `getProject` expose `IsDemo`.
- token endpoints (coach.go, exploration.go, exploration_review.go, search_guidance.go, placement.go, proposal_annotations.go, studioturn.go) — demo short-circuit to canned fixtures.
- `apps/api/internal/api/evaluation_generate.go` / `evaluation_report.go` — demo returns the seeded report (never generates).
- `apps/api/internal/api/chat.go` — demo guard on chat-thread writes.

**Modified (frontend):**
- `packages/contracts/src/workspace.ts` — `WorkspaceProjection` gains `isDemo: z.boolean().optional().default(false)`.
- `apps/web/src/workspace/WorkspaceContainer.tsx` (+ small components) — read `isDemo`, show a read-only banner, suppress obvious write affordances.

---

## Task 1: `is_demo` column + write-guard + expose `isDemo`

**Files:**
- Create: `apps/api/internal/store/migrations/0081_project_is_demo.sql`
- Modify: `apps/api/internal/httpx/errors.go` (add `ErrDemoReadonly`)
- Modify: `apps/api/internal/api/projects.go` (`loadOwnedProject` guard; `workspaceProjection`/`getProject` expose IsDemo)
- Modify: `packages/contracts/src/workspace.ts` (`isDemo` field)
- Regenerate: sqlc (`p.IsDemo` on `sqlc.Project`)
- Test: `apps/api/internal/api/demo_test.go`

**Interfaces:**
- Produces: `project.is_demo`; `loadOwnedProject` returns `ok=false` (403 `demo_readonly`) for non-GET on a demo project; `WorkspaceProjection.isDemo: boolean`.

- [ ] **Step 1: Migration**

`apps/api/internal/store/migrations/0081_project_is_demo.sql`:
```sql
-- +goose Up
ALTER TABLE project ADD COLUMN is_demo boolean NOT NULL DEFAULT false;
-- NOTE: do NOT flag project ...0101 here — it is the shared WRITE fixture for ~10
-- existing test files (studioturn_test, projectcards_test, disposition_test, etc.),
-- which would break under the read-only guard. The demo project is a NEW dedicated
-- id (00000000-0000-0000-0000-000000000200), inserted + flagged is_demo in 0082.

-- +goose Down
ALTER TABLE project DROP COLUMN is_demo;
```

- [ ] **Step 2: Regenerate sqlc + build**

Run: `cd apps/api && make sqlc && go build ./...`
Expected: `sqlc.Project` now has `IsDemo bool` (GetProject is `SELECT *`).

- [ ] **Step 3: Add `ErrDemoReadonly`**

In `apps/api/internal/httpx/errors.go`, mirror `ErrNotEntitled` (find it near line 67):
```go
// ErrDemoReadonly — 403 for a write attempt against a read-only demo project.
func ErrDemoReadonly() *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "demo_readonly", Message: "演示项目为只读，无法修改。"}
}
```
(Match the exact `APIError` construction style of the sibling constructors.)

- [ ] **Step 4: Write the failing guard test**

Create `apps/api/internal/api/demo_test.go` (mirror `users_accent_test.go` helpers). Seed/flag a demo project owned by the seed user (or reuse `…0101` if the seed user owns it — check; if `…0101` is owned by Phoebe `…003`, sign in as that seed user via the existing helper, else insert a small demo project owned by `SeedUserID` in the test). Assert: a GET on the demo project's workspace returns 200 with `isDemo:true`; a PUT (e.g. proposal or rename) returns 403 `demo_readonly`; the SAME write on a NON-demo project still succeeds (guard is scoped to is_demo).

```go
func TestDemoProjectReadOnly(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	cookie := signInSeed(t, pool)
	// ... create a normal project (write succeeds) and a demo project (is_demo=true) owned by the seed user ...
	// GET demo workspace → 200, body.project.isDemo == true
	// PATCH/PUT demo (rename/proposal) → 403, error.code == "demo_readonly"
	// same write on the normal project → 200
}
```
(Use the real module import path from an existing `_test.go`; construct project rows via `sqlc` queries or direct pool exec.)

- [ ] **Step 5: Run to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestDemoProjectReadOnly -timeout 900s`
Expected: FAIL (no guard yet; write returns 200/other).

- [ ] **Step 6: Add the demo guard in `loadOwnedProject` (world-readable, write-blocked)**

In `apps/api/internal/api/projects.go` (~line 162), `p` is already loaded (`GetProject`, ~line 169). A demo project must be **readable by ANY authenticated user** (the tour walks it as a non-owner) but **writable by NO ONE**. So handle `is_demo` BEFORE the ownership check:
```go
	if p.IsDemo {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, r, httpx.ErrDemoReadonly())
			return uuid.UUID{}, false
		}
		return id, true // demo projects are world-readable to authenticated users
	}
	// ...existing ownership check (p.UserID != u.ID → 404) stays for non-demo projects...
```
(This covers all ~140 project-scoped handlers; mutations to a demo get 403, reads succeed for everyone.) Keep the `(uuid.UUID, bool)` signature. Also add a sibling **`loadOwnedProjectRow(w, r) (sqlc.Project, bool)`** that returns the full row with the SAME demo/ownership semantics EXCEPT it does NOT auto-403 non-GET (token endpoints call this, then short-circuit to canned fixtures themselves) — i.e. for `is_demo` it returns the row+ok for any user/any method; for non-demo it enforces ownership. `loadOwnedProject` may delegate to it. Match the existing `uuid.UUID{}` zero-value style used by the sibling early returns.

- [ ] **Step 7: Expose `isDemo` on the projection**

In `projects.go`, add `IsDemo bool \`json:"isDemo"\`` to the `workspaceProjection` struct (~line 194-211) and set it from `p.IsDemo` in `getProject` (~line 220). In `packages/contracts/src/workspace.ts` (~line 12), add to `WorkspaceProjection`:
```ts
  isDemo: z.boolean().optional().default(false),
```
(Mirror the `writingFinished` optional/default precedent so old payloads still parse.)

- [ ] **Step 8: Run tests + build + typecheck**

Run: `cd apps/api && go build ./... && go test ./internal/api/ -run TestDemoProjectReadOnly -timeout 900s`
Run: `cd apps/web && npm run typecheck`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/store/migrations/0081_project_is_demo.sql apps/api/internal/store/sqlc/ apps/api/internal/httpx/errors.go apps/api/internal/api/projects.go apps/api/internal/api/demo_test.go packages/contracts/src/workspace.ts
git commit -m "feat(demo): project.is_demo + read-only write-guard + expose isDemo" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: Demo-mode canned fixtures for token endpoints

Short-circuit every token-consuming endpoint to a canned fixture when the project is a demo, so the demo never calls a model. These correlate with the `HasEntitlement` seam.

**Files:**
- Create: `apps/api/internal/api/demo.go` (`loadOwnedProjectRow` helper + `isDemoProject(ctx, id)` + canned-fixture builders)
- Modify: `coach.go` (postCoach/postCoachOpening/postCoachStart/postCoachAdvance), `studioturn.go` (postProjectTurn), `exploration.go` (postExplorationGuide/digExploration/proposeQuestionEdges), `exploration_review.go` (postExplorationReview), `search_guidance.go` (postSearchGuidance), `placement.go` (postSuggestPlacement), `proposal_annotations.go` (reviewProposalAnnotations)
- Modify: `apps/api/internal/api/chat.go` (demo guard on chat-thread writes)
- Test: extend `apps/api/internal/api/demo_test.go`

**Interfaces:**
- Consumes: `p.IsDemo` (Task 1).
- Produces: `func (a *API) isDemoProject(ctx, projectID uuid.UUID) (bool, error)` and per-surface canned responses.

- [ ] **Step 1: Add the demo helpers**

`loadOwnedProjectRow(w, r) (sqlc.Project, bool)` is ALREADY provided by Task 1 (world-readable demo, no write-guard). This task adds to `apps/api/internal/api/demo.go`:
- `isDemoProject(ctx, id)` — `GetProject` → `.IsDemo` (for handlers that only have the id).
- Token handlers switch from `loadOwnedProject` to `loadOwnedProjectRow` (so the write-guard's 403 doesn't pre-empt their POST) and then short-circuit to canned fixtures when the row `IsDemo`.
- Canned builders returning the SAME response shapes the real handlers emit but with fixed, on-topic content (e.g. a short 印记 coach reply; an empty/curated exploration dig result; a benign annotation review with 0 findings). Keep them minimal and clearly demo-flavored.

- [ ] **Step 2: Write the failing token-fixture test**

Extend `demo_test.go`: `POST /projects/{demoId}/coach` returns 200 with a canned reply and (assert) writes NO `chat_message`/`messages` rows and does not depend on a provider; `POST .../exploration/dig` returns 200 canned; the evaluation-report GET returns the seeded report (Task 3/5) — for now assert the coach + one exploration endpoint short-circuit.

- [ ] **Step 3: Run to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestDemo -timeout 900s`
Expected: FAIL (endpoints still try the real path).

- [ ] **Step 4: Short-circuit each token endpoint**

In each handler, right after `loadOwnedProject`/`HasEntitlement` (before the LLM call), add:
```go
	if demo, _ := a.isDemoProject(r.Context(), projectID); demo {
		httpx.WriteJSON(w, http.StatusOK, <canned fixture for this surface>)
		return
	}
```
For streaming handlers (`postProjectTurn` via `streamAction`), emit a minimal canned SSE stream (a single assistant message event + done) instead of calling the provider — or, simplest, return a 200 non-stream canned payload guarded by the demo check before the stream starts. Keep behavior consistent with what the frontend expects for a read-only demo (no state mutation).

- [ ] **Step 5: Chat-thread demo guard**

In `chat.go`, the chat write handlers resolve the thread via a `chat_thread` ownership helper (not `loadOwnedProject`). If the thread's `seeded_project_id` points at a demo project, reject writes with `ErrDemoReadonly()` (and short-circuit `postChatTurn` to a canned reply). Add the check in the thread-resolution helper (chat.go ~:63-84) so it covers all chat writes.

- [ ] **Step 6: Run tests + build**

Run: `cd apps/api && go build ./... && go test ./internal/api/ -run TestDemo -timeout 900s`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/demo.go apps/api/internal/api/coach.go apps/api/internal/api/studioturn.go apps/api/internal/api/exploration.go apps/api/internal/api/exploration_review.go apps/api/internal/api/search_guidance.go apps/api/internal/api/placement.go apps/api/internal/api/proposal_annotations.go apps/api/internal/api/chat.go apps/api/internal/api/demo_test.go
git commit -m "feat(demo): canned fixtures for token endpoints + chat guard (no live LLM)" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: Seed migration — core + 立题/管理 + 阅读

Author the finished demo project's data for the first three rooms in `0082_seed_demo_project_finished.sql`. **INSERT a NEW dedicated demo project `00000000-0000-0000-0000-000000000200`** owned by Phoebe `…003`, with `is_demo=true` and `status` set later to `finished` (Task 4). Do NOT reuse `…0101` (shared write fixture). All demo rows use fixed UUIDs in the `…02xx` block; materials are project-scoped (`project_id=…0200`, no tasks anchor needed — `material_scope_ck` allows project_id alone). Content on the China-sustainability topic; substantive (quality bar).

**Files:**
- Create: `apps/api/internal/store/migrations/0082_seed_demo_project_finished.sql`
- Test: `apps/api/internal/api/demo_seed_test.go` (asserts the rooms render non-empty via the read endpoints)

**Interfaces:**
- Produces: `project_proposal` (4 dims filled), ≥3 `plan_item`, `activity_log_entry` rows, `milestone:framework_finished` event, `graph_node` rows; `reference` rows (≥4) with linked `material` (non-empty `blocks`), `collection`, `exploration_lead` + `question_edge`, `citation`, `source_log_entry`; a `chat_message surface='studio'` + `project.studio_state.started=true`.

- [ ] **Step 1: Write the failing seed-render test**

Create `apps/api/internal/api/demo_seed_test.go`: after migrations run (testcontainers), sign in as the demo owner and GET the workspace + reading/library endpoints; assert the proposal has all 4 dims, ≥1 plan item, ≥4 references, and at least one material with non-empty `blocks`. (This test also covers Tasks 4-5 as they land; scope this task's asserts to rooms 立题/管理/阅读.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestDemoSeed -timeout 900s`
Expected: FAIL (rooms empty).

- [ ] **Step 3: Author the seed (part 1)**

In `0082_...sql` `-- +goose Up`, INSERT (all `ON CONFLICT DO NOTHING`, fixed UUIDs in the `…01xx`/`…02xx` block):
- `project_proposal` (objective/reason/activities/resources[/counterpoints]) — substantive paragraphs on the research question.
- `plan_item` ×≥3 (read/write/review, across todo/doing/done, with start_day/days/position).
- `activity_log_entry` ×≥3 (mix of `auto`/`me`).
- `event`: `milestone:framework_finished` (surface='studio', payload '{}'), plus a few `source_added`/`reading_focus`/`card_completed` events (these feed the report timeline).
- `graph_node`: a claim or two + evidence (open `type`, jsonb `body`), author='student'/'ai'.
- 阅读: `collection` ×1-2; `reference` ×≥4 (real-looking titles/authors/years/urls, credibility, decision='use', reading_status='done', a filled `reading_note` and `takeaway` jsonb on 1-2); `material` ×≥2 with real `blocks` JSON (`[{"id":...,"text":...}]`, follow `0020`); `source_log_entry` ×≥2 (material_id linked); `exploration_lead` ×≥3 + `question_edge` ×≥1 (label from the closed set); `citation` ×≥1.
- `chat_thread` (seeded_project_id=project, user=owner) + ≥1 `chat_message` surface='studio'; `UPDATE project SET studio_state = jsonb_set(studio_state,'{started}','true')` (or set the full started state).
Author `-- +goose Down` deleting these rows by their fixed ids.

- [ ] **Step 4: Run to verify part-1 asserts pass**

Run: `cd apps/api && make sqlc && go build ./... && go test ./internal/api/ -run TestDemoSeed -timeout 900s`
Expected: the 立题/管理/阅读 asserts PASS (写作/回顾/eval asserts, if present, still fail until Tasks 4-5).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0082_seed_demo_project_finished.sql apps/api/internal/api/demo_seed_test.go
git commit -m "feat(demo): seed finished demo project — 立题/管理/阅读 content" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: Seed migration — 写作 + 回顾 + finished status

Extend `0082_...sql` with the writing + review rooms and flip the project to finished.

**Files:**
- Modify: `apps/api/internal/store/migrations/0082_seed_demo_project_finished.sql`
- Modify: `apps/api/internal/api/demo_seed_test.go` (add 写作/回顾/finished asserts)

**Interfaces:**
- Produces: `outline_node` ×≥4, `snippet` ×≥3 (some sectioned), `edit_buffer(doc_kind='essay')` non-empty full essay incl. a `## 反思` section, `draft_snapshot(doc_kind='essay')`, `writing_finish(doc_kind='essay')`, `project_reflection(answers[5], done=true)`, `project_mirror_prose`; `project.status='finished'`.

- [ ] **Step 1: Extend the seed test (write/review/finished)**

Add asserts: essay `edit_buffer` non-empty; `writing_finish` essay row exists; reflection `done=true` with 5 answers; `project.status='finished'`; the workspace GET reports display status `done`.

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestDemoSeed -timeout 900s`
Expected: FAIL on the new asserts.

- [ ] **Step 3: Author the seed (part 2)**

Append INSERTs (fixed UUIDs, ON CONFLICT):
- `outline_node` ×≥4 (depth/position tree); `snippet` ×≥3 (with `section` on some).
- `edit_buffer` (doc_kind='essay') with a **full-length** essay (real paragraphs answering the research question) that INCLUDES a `## 反思` / `## Reflection` heading + reflection paragraph (so D6 is detected); a `draft_snapshot` (doc_kind='essay', seq 1, same/earlier content); `writing_finish` (doc_kind='essay').
- `project_reflection` (`answers` jsonb = 5 substantive strings, `done=true`); `project_mirror_prose` (sections[], carry_forwards[]).
- Flip finished: `UPDATE project SET status='finished' WHERE id=<demo>`; append a `project_finished` event.
Extend `-- +goose Down` accordingly.

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/api && go build ./... && go test ./internal/api/ -run TestDemoSeed -timeout 900s`
Expected: PASS (all room asserts; eval assert still pending Task 5).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0082_seed_demo_project_finished.sql apps/api/internal/api/demo_seed_test.go
git commit -m "feat(demo): seed 写作/回顾 content + flip project to finished" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5: Seed the evaluation report + validate it against the contract

Author a **schema-valid** `evaluation_report` row for the demo project, and prove it parses against the strict Zod so the report renders.

**Files:**
- Modify: `apps/api/internal/store/migrations/0082_seed_demo_project_finished.sql`
- Test: `apps/web/test/tour/demoEvaluationReport.test.ts` (parse the seeded JSON against `EvaluationReport` Zod) — extract the JSON to a fixture the test imports, OR the test reads the migration's JSON literal.

**Interfaces:**
- Produces: `evaluation_report` row (project=demo, version=1, status='ready', `report` jsonb = full `EvaluationReport`).

- [ ] **Step 1: Author the report JSON against the contract**

Read `packages/contracts/src/evaluationReport.ts:140-156` (authoritative `.strict()` schema) and `apps/api/internal/evalreport/report.go`. Author a complete report for the demo project: `version:1`, `reportId`, `projectId` (the demo id), `student:{id,name:"Phoebe"}`, `basics` (title/type/dates/milestones/counters), `abstract` (all fields incl. `recommendedCourses[]`), `events[]`, `materials[]`, `depth[6]` (D1-D6, level 1-4, evidence quotes drawn from the seeded essay/materials), `autonomy[6]` (A1-A6, band 0-5), `promptLens`, `toolUsage[]`, `risks[]`, `generatedAt`. Content must be coherent with the seeded project. Put the exact JSON both in the migration INSERT and in a shared fixture file `apps/web/src/tour/fixtures/demoEvaluationReport.ts` (so the test can validate it and the frontend/demo can reuse if needed) — keep the two byte-identical, or have the test read the migration.

- [ ] **Step 2: Write the failing validation test**

Create `apps/web/test/tour/demoEvaluationReport.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { EvaluationReport } from "@mind-imprint/contracts";
import { demoEvaluationReport } from "@/tour/fixtures/demoEvaluationReport";

describe("demo evaluation report", () => {
  it("satisfies the strict EvaluationReport contract", () => {
    expect(() => EvaluationReport.parse(demoEvaluationReport)).not.toThrow();
  });
});
```

- [ ] **Step 2b: Run to verify it fails, then author until it passes**

Run: `cd apps/web && npx vitest run test/tour/demoEvaluationReport.test.ts`
Iterate on the JSON until `EvaluationReport.parse` passes (strict — every field correct).

- [ ] **Step 3: Seed the row**

Append to `0082_...sql`: `INSERT INTO evaluation_report (id, project_id, version, report, status) VALUES ('…', '<demo>', 1, '<the validated JSON>'::jsonb, 'ready') ON CONFLICT (project_id) DO UPDATE SET report=EXCLUDED.report, status='ready';` Extend `-- +goose Down`.

- [ ] **Step 4: Verify end-to-end**

Run: `cd apps/api && go build ./... && go test ./internal/api/ -run TestDemoSeed -timeout 900s` (add an assert: GET evaluation-report for the demo returns status ready + a report) and `cd apps/web && npx vitest run test/tour/demoEvaluationReport.test.ts`.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0082_seed_demo_project_finished.sql apps/web/src/tour/fixtures/demoEvaluationReport.ts apps/web/test/tour/demoEvaluationReport.test.ts apps/api/internal/api/demo_seed_test.go
git commit -m "feat(demo): seed valid evaluation report + contract-validation test" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 6: Frontend read-only treatment for demo projects

Minimal: read `isDemo` off the workspace projection, show a read-only banner, and suppress the most obvious write affordances (the backend 403 is the real safety net — do NOT attempt to gate all ~140 writes).

**Files:**
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (thread `isDemo`, render banner)
- Modify: a few obvious write entry points (e.g. proposal save button, 完成 project, the chat send box) to disable when `isDemo`
- Test: `apps/web/test/workspace/demoReadonly.test.tsx`

**Interfaces:**
- Consumes: `WorkspaceProjection.isDemo` (Task 1).

- [ ] **Step 1: Failing test**

`apps/web/test/workspace/demoReadonly.test.tsx`: render `WorkspaceContainer` (or the smallest wrapper) with a projection where `isDemo:true`; assert a read-only banner (e.g. text "演示项目 · 只读") is shown; assert a primary write control (e.g. the proposal save / 完成 button) is disabled or absent.

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run test/workspace/demoReadonly.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

Thread `isDemo` from the parsed projection into `WorkspaceContainer`; render a small dismissible-not banner near the top (mk tokens, no alpha-on-token). Pass `isDemo` (or a `readOnly` prop) to the handful of prominent write controls and disable them. Keep it minimal and tasteful — this is defense-in-depth + honesty, not exhaustive gating.

- [ ] **Step 4: Run tests + typecheck**

Run: `cd apps/web && npx vitest run test/workspace/demoReadonly.test.tsx && npm run typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/WorkspaceContainer.tsx apps/web/test/workspace/demoReadonly.test.tsx <the write-control files touched>
git commit -m "feat(demo): read-only banner + suppress obvious writes when isDemo" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Self-Review (author's pass)

- **Spec coverage:** shared read-only demo (Task 1 guard) ✓; no live LLM (Task 2 canned fixtures + Task 5 seeded report) ✓; finished project across 5 rooms (Tasks 3-4) ✓; valid evaluation report proven by a contract test (Task 5) ✓; frontend read-only (Task 6) ✓. Content-quality bar is called out per seed task.
- **Risk flagged for user review:** the demo CONTENT (proposal/essay/reflections/eval narrative) is authored autonomously against the quality bar but is exactly the data-check gate the user wanted — flag it in the final report for their review/refinement. The Task 5 contract test guarantees it *renders*; it does not guarantee the prose is as polished as the user would author.
- **Type consistency:** `is_demo`/`IsDemo`/`isDemo` across migration→sqlc→DTO→contract; `evaluation_report` JSON validated against the same Zod the frontend uses.
- **Placeholder note:** the seed prose is authored by the implementer against a stated topic + quality bar + verifiable acceptance tests (rooms render non-empty; report parses strict). This is the pragmatic reading of "no placeholders" for a large content seed — every task has concrete, testable acceptance.

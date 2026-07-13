# Slice 5d — Routing cutover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Studio becomes the student's real workspace — inside the real shell, under real auth — and every line of the old task-based surface it replaces is deleted.

**Architecture:** Three movements. (1) **Backend retirement**: delete the 13 `/api/v1/tasks/*` routes, their handlers, `agent.RunTurn`, and the task-coupled evaluator + its river worker. (2) **Frontend cutover**: the left rail's 批判思维 tab becomes 工作室 and renders `StudioContainer` (already live); 我的评估 becomes 成长报告 with a deferred placeholder; `?studio` and the silent sign-in-as-Phoebe fallback are deleted, not patched. (3) **Truthfulness**: the teacher/admin console's per-student counts, which all aggregate over `tasks`, are re-pointed at the project model — otherwise every student reads 0-activity forever.

**Tech Stack:** Go (`net/http`, pgx/sqlc/goose, testcontainers) · TypeScript + React + vitest · Zod contracts.

**Spec:** `docs/superpowers/specs/2026-07-11-slice-5d-routing-cutover-design.md` — read it before Task 1.

## Global Constraints

- **Go tests MUST run serialized:** `cd apps/api && CGO_ENABLED=0 go test -p 1 ./...`. Parallel runs hang on Docker/testcontainers contention. `make sqlc` = `CGO_ENABLED=0 go tool sqlc generate`.
- **Test counts will DROP** as suites are deleted. That is expected and correct. Never "restore" a deleted test to keep a count up. The final task records the new baselines.
- **Delete, never comment out.** No `// deprecated` husks, no `_old` files, no feature flags. Git history is the archive.
- **Icons are inline SVG.** Never `lucide-react`.
- **Chinese UI copy is binding where it comes from the design** (`docs/design/思维印记_工作区.dc.html`). Labels `课程` / `工作室` / `成长报告` / `设置` are from the design's left rail (dc.html:101-141) — use them verbatim. New copy (empty state, growth placeholder) is specified verbatim in the tasks below; use it as written.
- **Do not add a 聊天 nav item.** It is in the design's rail but its content is Slice 11.
- **Do not drop the `tasks` or `evaluations` tables.** Destructive, and `evaluations.project_id` is what Slice 10 will write to. Only code paths are retired.
- **Preserved on purpose — do not delete:** `internal/materialize` (URL→blocks fetcher; Slice 6b reuses it), `material.sql`'s `CreateProjectMaterial` + `ListMaterialsByProject`, `agent.NewAnchorGenerator`, `agent.RunAgentStep`, everything under `internal/gateway`, courses, voice, auth, org/console.

---

### Task 1: Backend — retire the old task HTTP surface + `RunTurn`

**Files:**
- Delete: `apps/api/internal/api/turn.go`, `turn_test.go`, `tasks.go`, `tasks_test.go`, `cards.go`, `cards_test.go`, `material.go`, `material_test.go`, `e2e_test.go`
- Delete: `apps/api/internal/agent/turn.go`, `turn_test.go`
- Modify: `apps/api/internal/api/api.go` (drop 11 routes; drop the `Fetcher` interface + `Deps.Fetcher`)
- Modify: `apps/api/internal/api/dto.go` (drop task/material DTOs that lose their only caller)
- Modify: `apps/api/internal/store/queries/tasks.sql`, `material.sql` (drop the task-scoped queries)
- Modify: `apps/api/cmd/api/main.go` (drop `Fetcher: materialize.NewFetcher()` from `api.Deps`)
- Test: `apps/api/internal/api/routes_test.go` (NEW)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: a router with **zero** `/api/v1/tasks/*` routes. `api.Deps` no longer has a `Fetcher` field. Task 2 removes `EvalResolver` + `Enqueuer` from the same struct — do not touch those here.

Note the deleted files may not all exist under exactly these test names; delete whatever test files cover the deleted handlers, and let the compiler find the rest.

**Routes to remove from `api.go` (leave `/api/v1/tasks/{id}/evaluate` and `/evaluation` — Task 2 owns those):**

```go
mux.Handle("GET /api/v1/tasks", protected(a.listTasks))
mux.Handle("POST /api/v1/tasks", protected(a.createTask))
mux.Handle("GET /api/v1/tasks/{id}", protected(a.getTask))
mux.Handle("PATCH /api/v1/tasks/{id}/cards/{cid}", protected(a.patchCard))
mux.Handle("PUT /api/v1/tasks/{id}/cards/{cid}", protected(a.putCard))
mux.Handle("POST /api/v1/tasks/{id}/cards/{cid}/skip", protected(a.skipCard))
mux.Handle("POST /api/v1/tasks/{id}/turn", protected(a.postTurn))
mux.Handle("GET /api/v1/tasks/{id}/materials", protected(a.listMaterials))
mux.Handle("POST /api/v1/tasks/{id}/materials", protected(a.createMaterial))
mux.Handle("POST /api/v1/tasks/{id}/materials/from-seed", protected(a.materialFromSeed))
mux.Handle("PUT /api/v1/tasks/{id}/materials/{mid}/scratch", protected(a.updateMaterialScratch))
```

**`internal/api/api.go` — also delete this interface and its `Deps` field:**

```go
// Fetcher retrieves readable text from a URL. materialize.HTTPFetcher is the
// production impl; tests inject a fake.
type Fetcher interface {
	FetchReadable(ctx context.Context, url string) (title, text string, err error)
}
```

`internal/materialize` itself **stays** (spec §4.2) — it keeps its own tests and Slice 6b wires it to `CreateProjectMaterial`. It will temporarily have no caller. That is deliberate; do not delete it, and do not "use" it to justify keeping it.

**sqlc queries to delete** (`tasks.sql`: the task CRUD — `CreateTask`, `GetTask`, `ListTasks`, `UpdateTask` and any other query in that file whose only caller was a deleted handler; `material.sql`: `CreateMaterial`, `ListMaterialsByTask`, `GetMaterial`, `UpdateMaterialScratch`). **Keep `CreateProjectMaterial` and `ListMaterialsByProject`.** If a query in `tasks.sql` is still referenced from org/console code, keep it — the compiler is the arbiter.

- [ ] **Step 1: Write the failing route-surface test**

`apps/api/internal/api/routes_test.go`:

```go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The old task surface is retired (Slice 5d). This is a regression fence: a
// half-deleted router that still registers a task route would silently keep the
// dead path reachable.
func TestOldTaskRoutesAreGone(t *testing.T) {
	t.Parallel()
	h := newTestAPI(t).Handler() // reuse this package's existing test-API constructor
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/tasks"},
		{"POST", "/api/v1/tasks"},
		{"GET", "/api/v1/tasks/00000000-0000-0000-0000-000000000001"},
		{"POST", "/api/v1/tasks/00000000-0000-0000-0000-000000000001/turn"},
		{"PUT", "/api/v1/tasks/00000000-0000-0000-0000-000000000001/cards/c1"},
		{"GET", "/api/v1/tasks/00000000-0000-0000-0000-000000000001/materials"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: got %d, want 404 (route should be retired)", tc.method, tc.path, rec.Code)
		}
	}
}
```

`newTestAPI(t)` is a placeholder for whatever constructor this package's existing tests use to build an `*API` without a DB (check `classes_test.go` / `studioturn_test.go` for the established pattern and follow it). If every existing helper needs a DB, build the handler directly with a zero-value `api.Deps` — route *registration* is what's under test, and an unregistered route 404s before any handler or middleware touches `Deps`.

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestOldTaskRoutesAreGone -v`
Expected: FAIL — the routes still exist (401 from `RequireUser`, or 200, not 404).

- [ ] **Step 3: Delete the handlers, the routes, `RunTurn`, and the orphaned queries**

Delete the files listed above; remove the 11 route registrations; remove the `Fetcher` interface + `Deps.Fetcher` + its wiring in `main.go`; remove the now-uncalled queries from `tasks.sql` / `material.sql` and regenerate:

```bash
cd apps/api && make sqlc
```

Then let the compiler find every stranded reference (`go build ./...`) and delete those too — including any DTO structs in `dto.go` that only the deleted handlers used.

- [ ] **Step 4: Run the route test + build + full backend suite**

```bash
cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test -p 1 ./...
```
Expected: PASS. The studio/project suites (`studioturn_test.go`, `projectcards_test.go`, `projects_test.go`) must be green and **untouched** — if a deletion broke one of them, the deletion was wrong. Do not edit a project-path test to make it pass.

- [ ] **Step 5: Commit**

```bash
git add -A apps/api && git commit -m "refactor(refactor2): retire the old task HTTP surface + RunTurn (5d T1)"
```

---

### Task 2: Backend — retire the task-coupled evaluator + its river worker

**Files:**
- Delete: `apps/api/internal/api/evaluate.go` (+ test), `eval_trigger.go` (+ test)
- Delete: `apps/api/internal/agent/eval.go`, `evalinput.go`, `evalprompt.go`, `evalrubric.go`, `evaljob.go` and their tests + `apps/api/internal/agent/testdata/eval_input.txt`, `eval_prompt_full.txt`
- Delete: `apps/api/internal/store/queries/evaluations.sql` (the whole file — but see the guard below)
- Modify: `apps/api/internal/api/api.go` (drop the `Enqueuer` interface, `Deps.EvalResolver`, `Deps.Enqueuer`; drop the 2 evaluate routes)
- Modify: `apps/api/cmd/api/main.go` (drop the river worker + client + `riverEnqueuer`)

**Interfaces:**
- Consumes: Task 1's `api.Deps` (already without `Fetcher`).
- Produces: `api.Deps` with no `EvalResolver`, no `Enqueuer`. `cmd/api/main.go` no longer starts a river client.

**Guard on `evaluations.sql`:** Task 3 re-points the org aggregates to count `evaluations` **by `project_id`**, but that lives in `org.sql`, not `evaluations.sql`. Delete only queries whose callers are being deleted here. If `go build ./...` shows a survivor still using one, keep that query.

**Routes to remove:**

```go
mux.Handle("POST /api/v1/tasks/{id}/evaluate", protected(a.postEvaluate))
mux.Handle("GET /api/v1/tasks/{id}/evaluation", protected(a.getEvaluation))
```

**`main.go` — remove this whole block** (lines ~104-125) and the `riverEnqueuer` type at ~line 31, plus the now-unused `river`/`riverpgxv5` imports and the `Enqueuer:` / `EvalResolver:` fields from the `api.Deps` literal:

```go
	// Embedded river client: the EvaluateWorker runs in-process off the default
	// queue, using the FLAGSHIP eval resolver (never downgraded).
	workers := river.NewWorkers()
	river.AddWorker(workers, &agent.EvaluateWorker{ ... })
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{ ... })
	...
	if err := riverClient.Start(ctx); err != nil { ... }
```

Leave `river` in `go.mod` (Slice 10's assessor will re-add a worker); `go mod tidy` is **not** part of this task.

- [ ] **Step 1: Extend the route fence (write the failing assertion first)**

Add to `TestOldTaskRoutesAreGone` in `apps/api/internal/api/routes_test.go`:

```go
		{"POST", "/api/v1/tasks/00000000-0000-0000-0000-000000000001/evaluate"},
		{"GET", "/api/v1/tasks/00000000-0000-0000-0000-000000000001/evaluation"},
```

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestOldTaskRoutesAreGone -v`
Expected: FAIL on the two new rows.

- [ ] **Step 3: Delete the evaluator, the river wiring, and the two routes**

Delete the files above. Remove the `Enqueuer` interface + both `Deps` fields + the routes. Strip `main.go`. Then `CGO_ENABLED=0 go build ./...` and delete every stranded reference the compiler names (including `gateway.NewEvalKeyResolver` call sites — **keep the `gateway` function itself**; Slice 10 needs it, and it is exercised by `gateway`'s own tests).

- [ ] **Step 4: Verify — build, run, full suite**

```bash
cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test -p 1 ./...
```
Expected: PASS. Note the agent package's test count drops (the eval suites are gone) — expected.

- [ ] **Step 5: Commit**

```bash
git add -A apps/api && git commit -m "refactor(refactor2): retire the task-coupled evaluator + river worker (5d T2)"
```

---

### Task 3: Org aggregates — re-point the console at the project model

**Files:**
- Modify: `apps/api/internal/store/queries/org.sql` (`GetClassRoster`, `GetSchoolCounts`)
- Modify: `apps/api/internal/store/queries/project.sql` (add `TouchProject`)
- Modify: `apps/api/internal/api/classes_dto.go` (`rosterEntryDTO.TaskCount` → `ProjectCount`, json `project_count`)
- Modify: `apps/api/internal/api/overview.go` (`"task"` counter key → `"project"`)
- Modify: `apps/api/internal/api/studioturn.go` (call `TouchProject` after a successful step)
- Modify: `apps/web/src/api/classes.ts` (`RosterStudent.task_count` → `project_count`)
- Modify: `apps/web/src/api/admin.ts` (`Overview.counts.task` → `project`)
- Modify: `apps/web/src/console/ClassDetailView.tsx` (column header `任务` → `项目`; cell `s.task_count` → `s.project_count`)
- Modify: `apps/web/src/console/OverviewView.tsx` (`{ key: "task", label: "任务" }` → `{ key: "project", label: "项目" }`)
- Test: `apps/api/internal/api/classes_test.go` (extend), `apps/api/internal/api/studioturn_test.go` (extend), and the console tests' fixtures

**Interfaces:**
- Consumes: Tasks 1-2's `api.Deps`.
- Produces: `rosterEntryDTO{ProjectCount int64 \`json:"project_count"\`}` and overview counter key `"project"`. TS mirrors: `RosterStudent.project_count: number`, `Overview.counts.project: number`.

**Why this task exists.** Every per-student count aggregates over `tasks` (`org.sql:41-52,73-75`). After Tasks 1-2 no student ever gets a `tasks` row again, so the teacher console shows **0 activity for every real student** from now through Slice 13. It is already subtly wrong: Slice-3 debt keeps `card_instances.task_id` NOT NULL, so Studio cards are anchored to the seed's **admin-owned** placeholder task — Phoebe's cards currently count toward the admin.

**`evaluation_count` stays**, re-pointed to `evaluations.project_id`. It will honestly read 0 until Slice 10 writes project-scoped evaluations. That is a true statement about the student, not a broken column. Do not delete it, and do not backfill it.

- [ ] **Step 1: Write the failing roster test**

In `apps/api/internal/api/classes_test.go` (follow the file's existing testcontainers + seeding helpers):

```go
// Slice 5d: the roster counts the PROJECT model, not the retired task model.
// Seeds a student with one project + one project-scoped card_instance and NO
// tasks row — which the pre-5d tasks-based query reports as all-zero.
func TestClassRoster_CountsProjectsNotTasks(t *testing.T) {
	// ... seed: school + class + student enrolled + one project owned by that
	//     student + one card_instance carrying that project_id ...
	// GET /api/v1/classes/{id} as the class teacher, decode the roster.
	// Assert for that student:
	//   project_count    == 1
	//   card_count       == 1
	//   evaluation_count == 0     // honest: no project-scoped evaluations yet
	//   last_active_at   != nil   // comes from project.last_active_at
}
```

Write it out concretely against the helpers already in `classes_test.go`. It must assert `project_count`, not `task_count`.

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestClassRoster_CountsProjectsNotTasks -v`
Expected: FAIL — the current query joins `tasks`, and this student has none, so every count is 0 (and the field is `task_count`, so it will not even compile once the assertion names `project_count`). **This failure is the proof the re-point is real and not cosmetic. If it passes before the change, the test is wrong — fix the test, not the query.**

- [ ] **Step 3: Re-point the queries**

`apps/api/internal/store/queries/org.sql` — replace `GetClassRoster` with:

```sql
-- name: GetClassRoster :many
SELECT u.id, u.display_name, u.email,
       MAX(p.last_active_at) AS last_active_at,
       COUNT(DISTINCT p.id)  AS project_count,
       COUNT(DISTINCT ev.id) AS evaluation_count,
       COUNT(DISTINCT ci.id) AS card_count
FROM enrollments e
JOIN users u                ON u.id = e.user_id
LEFT JOIN project p         ON p.user_id = u.id
LEFT JOIN evaluations ev    ON ev.project_id = p.id
LEFT JOIN card_instances ci ON ci.project_id = p.id
WHERE e.class_id = $1 AND e.role_in_class = 'student'
GROUP BY u.id, u.display_name, u.email
ORDER BY u.display_name;
```

and the three task-based rows of `GetSchoolCounts` with:

```sql
  (SELECT count(*) FROM project p JOIN users u ON u.id = p.user_id WHERE u.school_id = $1)  AS project_count,
  (SELECT count(*) FROM evaluations e JOIN project p ON p.id = e.project_id JOIN users u ON u.id = p.user_id WHERE u.school_id = $1) AS evaluation_count,
  (SELECT count(DISTINCT p.user_id) FROM project p JOIN users u ON u.id = p.user_id WHERE u.school_id = $1) AS active_student_count;
```

Add to `apps/api/internal/store/queries/project.sql`:

```sql
-- name: TouchProject :exec
UPDATE project SET last_active_at = now() WHERE id = $1;
```

Then `cd apps/api && make sqlc`.

- [ ] **Step 4: Update the Go DTOs**

`classes_dto.go`:

```go
type rosterEntryDTO struct {
	ID              string  `json:"id"`
	DisplayName     string  `json:"display_name"`
	Email           string  `json:"email"`
	LastActiveAt    *string `json:"last_active_at"`
	ProjectCount    int64   `json:"project_count"`
	EvaluationCount int64   `json:"evaluation_count"`
	CardCount       int64   `json:"card_count"`
}
```
and in `toRosterEntryDTO`: `ProjectCount: row.ProjectCount,`.

`overview.go` — in the `counts` map, `"task": c.TaskCount` becomes `"project": c.ProjectCount`.

- [ ] **Step 5: Make `last_active_at` real**

`project.last_active_at` is set at creation and never updated, so a roster that reads it would show the project's *birthday* as 最近活跃. In `apps/api/internal/api/studioturn.go`'s `postProjectTurn`, after `RunAgentStep` returns without error and before the stream closes, touch the project:

```go
if err := a.d.Queries.TouchProject(ctx, projectID); err != nil {
	slog.Warn("touch project last_active_at", "project_id", projectID, "err", err)
}
```

A failure here must **not** fail the turn — the student's turn already succeeded.

Add to `apps/api/internal/api/studioturn_test.go`:

```go
// Slice 5d: a turn is activity — the roster's 最近活跃 depends on it.
func TestProjectTurn_TouchesLastActiveAt(t *testing.T) {
	// read project.last_active_at, run a turn via POST /api/v1/projects/{id}/turn,
	// re-read, assert it strictly advanced.
}
```

- [ ] **Step 6: Update the console TS + its tests**

`apps/web/src/api/classes.ts`: `task_count: number;` → `project_count: number;`
`apps/web/src/api/admin.ts`: `task: number;` → `project: number;`
`apps/web/src/console/ClassDetailView.tsx:232`: `<th style={TH}>任务</th>` → `<th style={TH}>项目</th>`; `:244`: `{s.task_count}` → `{s.project_count}`
`apps/web/src/console/OverviewView.tsx:11`: `{ key: "task", label: "任务" }` → `{ key: "project", label: "项目" }`
Update `ClassDetailView.test.tsx` / `OverviewView.test.tsx` fixtures to the new field names.

- [ ] **Step 7: Verify**

```bash
cd apps/api && CGO_ENABLED=0 go test -p 1 ./...
cd apps/web && pnpm test && pnpm exec tsc --noEmit
```
Expected: PASS, including the two new Go tests.

- [ ] **Step 8: Commit**

```bash
git add -A && git commit -m "fix(refactor2): re-point org aggregates at the project model + touch last_active_at (5d T3)"
```

---

### Task 4: Frontend — the left rail moves to the design

**Files:**
- Modify: `apps/web/src/shell/LeftRail.tsx`
- Test: `apps/web/src/shell/LeftRail.test.tsx`

**Interfaces:**
- Produces: `type TabKey = "courses" | "studio" | "growth" | "settings"` — Task 5's `StudentApp` switches on exactly these.

The binding design's rail (`docs/design/思维印记_工作区.dc.html:101-141`) is 课程 · 聊天 · 工作室 · 成长报告 · 设置. We ship four of the five: **聊天 is Slice 11 and is NOT added** — a nav item that goes nowhere is worse than one that isn't there. The icons already in `LeftRail.tsx:29,38` are the design's paths; only labels and keys change.

- [ ] **Step 1: Write the failing test**

In `apps/web/src/shell/LeftRail.test.tsx`:

```tsx
it("renders the four shipped rail items per the binding design", () => {
  render(<LeftRail tab="studio" onTab={() => {}} />);
  expect(screen.getByText("课程")).toBeTruthy();
  expect(screen.getByText("工作室")).toBeTruthy();
  expect(screen.getByText("成长报告")).toBeTruthy();
  expect(screen.getByText("设置")).toBeTruthy();
  // 聊天 is in the design's rail but its content is Slice 11.
  expect(screen.queryByText("聊天")).toBeNull();
  // The retired labels are gone.
  expect(screen.queryByText("批判思维")).toBeNull();
  expect(screen.queryByText("我的评估")).toBeNull();
});
```

Keep the file's existing active/inactive-styling tests working (retarget any that pass `tab="tasks"` to `tab="studio"`).

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && pnpm test -- LeftRail`
Expected: FAIL — `工作室` / `成长报告` not found; also a type error on `tab="studio"`.

- [ ] **Step 3: Rename the keys and labels**

```tsx
// LEFT RAIL — four pillars per binding design 思维印记_工作区.dc.html (left-rail section).
// Tabs: 课程 (courses) / 工作室 (studio) / 成长报告 (growth) / 设置 (settings).
// 聊天 is in the design's rail; its surface is Slice 11.

type TabKey = "courses" | "studio" | "growth" | "settings";
```

In `NAV_ITEMS`: `{ key: "tasks", label: "批判思维", ... }` → `{ key: "studio", label: "工作室", ... }` (same icon), and `{ key: "records", label: "我的评估", ... }` → `{ key: "growth", label: "成长报告", ... }` (same icon).

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/web && pnpm test -- LeftRail`
Expected: PASS. (`tsc` will still fail repo-wide until Task 5 updates `StudentApp` — that is fine within this task; the LeftRail suite itself is green.)

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/LeftRail.tsx apps/web/src/shell/LeftRail.test.tsx
git commit -m "feat(refactor2): left rail moves to the binding design — 工作室 / 成长报告 (5d T4)"
```

---

### Task 5: Frontend — `StudentApp` mounts the Studio; 成长报告 becomes a slot

**Files:**
- Modify: `apps/web/src/shell/StudentApp.tsx`
- Create: `apps/web/src/shell/growth/GrowthPlaceholder.tsx` (+ test)
- Test: `apps/web/src/shell/StudentApp.test.tsx` (create if absent)

**Interfaces:**
- Consumes: Task 4's `TabKey` (`"courses" | "studio" | "growth" | "settings"`); `StudioContainer` from `../studio/StudioContainer` (already live — takes optional `api`, `makeConversation` props; after Task 6 it takes **no** `ensureSession`).
- Produces: a `StudentApp` with no `TaskView` state machine, no `DirectoryView`, no `WorkspaceContainer`, and no `store` prop.

**`StudentApp` no longer needs the `store` prop at all** — its only consumers were `DirectoryView`, `WorkspaceContainer`, and `RecordsView`, all of which go. Drop it from the props and from `AppShell`'s call site (`AppShell.tsx:89`) and delete `AppShell`'s `defaultStore` / `store` prop with it. `AppShell`'s auth boot, `?trial=1` demo path, and console branch are **untouched**.

- [ ] **Step 1: Write the failing tests**

`apps/web/src/shell/growth/GrowthPlaceholder.test.tsx`:

```tsx
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { GrowthPlaceholder } from "./GrowthPlaceholder";

describe("GrowthPlaceholder", () => {
  it("says plainly that the growth report is being rebuilt", () => {
    render(<GrowthPlaceholder />);
    expect(screen.getByText("成长报告正在重建")).toBeTruthy();
  });
});
```

`apps/web/src/shell/StudentApp.test.tsx` — mock `../studio/StudioContainer` so the test doesn't need the API:

```tsx
vi.mock("../studio/StudioContainer", () => ({
  StudioContainer: () => <div data-testid="studio-container" />,
}));

it("renders the Studio on the 工作室 tab (the default landing)", () => {
  render(<StudentApp session={fakeSession} onLogout={() => {}} />);
  expect(screen.getByTestId("studio-container")).toBeTruthy();
});

it("renders the growth placeholder on 成长报告", async () => {
  render(<StudentApp session={fakeSession} onLogout={() => {}} />);
  await userEvent.click(screen.getByText("成长报告"));
  expect(screen.getByText("成长报告正在重建")).toBeTruthy();
  expect(screen.queryByTestId("studio-container")).toBeNull();
});
```

Build `fakeSession` from the shape `SettingsView.test.tsx` already uses.

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && pnpm test -- StudentApp GrowthPlaceholder`
Expected: FAIL — `GrowthPlaceholder` doesn't exist; `StudentApp` still renders `DirectoryView`.

- [ ] **Step 3: Write `GrowthPlaceholder`**

`apps/web/src/shell/growth/GrowthPlaceholder.tsx` — a deferred-shell placeholder, the same pattern the Studio uses for its deferred center-pane views. It must not imply the student has no history; it must say the surface is being rebuilt.

```tsx
// 成长报告 — the rail slot exists (binding design), the surface does not yet.
// The old task-based 我的评估 was retired with the task model in Slice 5d; the
// project-backed report lands in Slice 9 (评估 view) + Slice 10 (growth report).
export function GrowthPlaceholder() {
  return (
    <div style={{ height: "100%", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 10, color: "#6B7384" }}>
      <div style={{ fontSize: 16, fontWeight: 700, color: "#3A4256" }}>成长报告正在重建</div>
      <div style={{ fontSize: 13.5, lineHeight: 1.7, maxWidth: 420, textAlign: "center" }}>
        我们正在按新的过程模型重做评估与成长报告。你在工作室里的每一步都在被记录，报告回来时它们都在。
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Rewrite `StudentApp`**

```tsx
import { useState } from "react";
import type { SessionStore } from "./session";
import { LeftRail } from "./LeftRail";
import { CoursesContainer } from "./courses/CoursesContainer";
import { StudioContainer } from "../studio/StudioContainer";
import { GrowthPlaceholder } from "./growth/GrowthPlaceholder";
import { SettingsView } from "./settings/SettingsView";

type Tab = "courses" | "studio" | "growth" | "settings";

export function StudentApp({
  session,
  onLogout,
}: {
  session: SessionStore;
  onLogout: () => void;
}) {
  const [tab, setTab] = useState<Tab>("studio");

  return (
    <div style={{ display: "flex", height: "100%", width: "100%", background: "#F3F4F8", overflow: "hidden" }}>
      <LeftRail tab={tab} onTab={setTab} />
      <div style={{ flex: 1, overflow: "hidden", position: "relative" }}>
        {tab === "courses" && <CoursesContainer onGoPortal={() => setTab("studio")} />}
        {tab === "studio" && <StudioContainer />}
        {tab === "growth" && <GrowthPlaceholder />}
        {tab === "settings" && (
          <SettingsView session={session} user={session.getUser()} onLogout={onLogout} />
        )}
      </div>
    </div>
  );
}
```

Then update `AppShell.tsx`: drop `store` / `defaultStore` and pass `<StudentApp session={session} onLogout={onLogout} />`. Update `AppShell.test.tsx` if it passes a store.

- [ ] **Step 5: Run — expect PASS**

Run: `cd apps/web && pnpm test -- StudentApp GrowthPlaceholder AppShell`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A apps/web/src/shell && git commit -m "feat(refactor2): 工作室 tab mounts the Studio; 成长报告 slot placeholder (5d T5)"
```

---

### Task 6: Frontend — `StudioContainer` loses `ensureSession`, gains an honest empty state

**Files:**
- Modify: `apps/web/src/studio/StudioContainer.tsx`
- Test: `apps/web/src/studio/StudioContainer.test.tsx`

**Interfaces:**
- Produces: `StudioContainer` props become `{ api?, makeConversation? }` — **no `ensureSession`**. Task 5's call site (`<StudioContainer />`) already matches.

Two bugs, one cause — the container was written to be reachable at `?studio` without a login:

1. `defaultEnsureSession` (`StudioContainer.tsx:46-57`) silently signs in as the seeded Phoebe on **any** `getMe` failure. Mounted inside `StudentApp`, the Studio sits behind `AppShell`'s boot → `getMe` → `AuthScreen` gate, so this is not just unnecessary — it is a live "any failure logs you in as someone else" hazard. **Delete it and the prop, do not gate it.** The `DEMO_EMAIL` / `DEMO_PASSWORD` constants go with it. The marketing `?trial=1` path is unaffected: `AppShell.tsx:54-66` owns it.
2. `if (list.length === 0) throw new Error("no projects")` (`:86`) funnels "you have no projects" into the `加载失败，请重试` **error** state. Post-cutover that is the first thing a real student hits, and telling them to retry is a lie — there is no project creation in 5d (spec §6.3).

- [ ] **Step 1: Write the failing tests**

In `apps/web/src/studio/StudioContainer.test.tsx`:

```tsx
it("shows an honest empty state when the student has no projects", async () => {
  const api = { listProjects: vi.fn(async () => []), getProject: vi.fn() };
  render(<StudioContainer api={api} />);
  await waitFor(() => expect(screen.getByText("还没有项目")).toBeTruthy());
  expect(screen.queryByText("加载失败，请重试")).toBeNull();
  expect(api.getProject).not.toHaveBeenCalled();
});

it("shows the error state when the request fails", async () => {
  const api = { listProjects: vi.fn(async () => { throw new Error("boom"); }), getProject: vi.fn() };
  render(<StudioContainer api={api} />);
  await waitFor(() => expect(screen.getByText("加载失败，请重试")).toBeTruthy());
  expect(screen.queryByText("还没有项目")).toBeNull();
});

it("never signs in on its own — auth is the shell's job", async () => {
  const signin = vi.spyOn(apiModule.api, "signin");
  const api = { listProjects: vi.fn(async () => { throw new Error("401"); }), getProject: vi.fn() };
  render(<StudioContainer api={api} />);
  await waitFor(() => expect(screen.getByText("加载失败，请重试")).toBeTruthy());
  expect(signin).not.toHaveBeenCalled();
});
```

The third test is the regression fence for the deleted fallback — it must fail today.

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && pnpm test -- StudioContainer`
Expected: FAIL — empty renders the error copy; the signin spy fires.

- [ ] **Step 3: Delete `ensureSession`, split the states**

Remove `defaultEnsureSession`, the `ensureSession` prop, the `DEMO_EMAIL`/`DEMO_PASSWORD` constants, and the `await ensureSession()` call. Replace the load effect's body with a state that distinguishes empty from error:

```tsx
const [empty, setEmpty] = useState(false);

useEffect(() => {
  let cancelled = false;
  (async () => {
    try {
      const list = await api.listProjects();
      if (cancelled) return;
      if (list.length === 0) { setEmpty(true); return; }
      const proj = await api.getProject(list[0]!.id);
      if (cancelled) return;
      const s = toStudioState(proj);
      setState(s);
      setActiveStation(s.activeStation);
      setProjectId(list[0]!.id);
    } catch {
      if (!cancelled) setError("加载失败，请重试");
    }
  })();
  return () => { cancelled = true; };
}, [api]);
```

and, before the existing error/loading returns:

```tsx
if (empty) {
  return (
    <div className="mk-studio-empty" style={{ height: "100%", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 10, color: "#6B7384" }}>
      <div style={{ fontSize: 16, fontWeight: 700, color: "#3A4256" }}>还没有项目</div>
      <div style={{ fontSize: 13.5, lineHeight: 1.7, maxWidth: 420, textAlign: "center" }}>
        工作室从一个真实的写作任务开始。创建入口马上就来——在那之前，这里会保持空着。
      </div>
    </div>
  );
}
```

Note the dependency array drops `ensureSession`.

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/web && pnpm test -- StudioContainer`
Expected: PASS (all three new tests + the existing suite).

- [ ] **Step 5: Commit**

```bash
git add -A apps/web/src/studio && git commit -m "fix(refactor2): StudioContainer drops the silent Phoebe sign-in; honest empty state (5d T6)"
```

---

### Task 7: Frontend — `Root` drops `?studio`

**Files:**
- Modify: `apps/web/src/Root.tsx`
- Test: `apps/web/src/Root.test.tsx`

**Interfaces:**
- Consumes: Task 5's `AppShell` (which now renders the Studio inside `StudentApp`).

- [ ] **Step 1: Write the failing test**

In `apps/web/src/Root.test.tsx`:

```tsx
it("no longer has a ?studio side door — it falls through to the app shell", async () => {
  window.history.pushState({}, "", "/?studio");
  render(<Root />);
  // The shell's auth gate, not a bare Studio: ?studio is retired (Slice 5d).
  await waitFor(() => expect(screen.getByRole("button", { name: "登录" })).toBeTruthy());
});
```

The existing `?demo` and default tests stay as they are.

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && pnpm test -- Root`
Expected: FAIL — `?studio` renders `StudioContainer`, not the auth screen.

- [ ] **Step 3: Delete the branch**

```tsx
import { AppShell } from "./shell/AppShell";
import { Harness } from "./dev/Harness";

// `?demo` renders the deterministic card gallery (Harness): flip through every
// card — proposal → active sheet → completed — with no LLM and zero latency.
// This is the reliable surface for demoing cards (the live summon path is, by
// design, restrained and not guaranteed per opening line).
// Everything else renders the real app — which, since Slice 5d, IS the Studio.
export function Root() {
  const params = new URLSearchParams(window.location.search);
  if (params.has("demo")) return <Harness />;
  return <AppShell />;
}
```

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/web && pnpm test -- Root`

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/Root.tsx apps/web/src/Root.test.tsx
git commit -m "refactor(refactor2): retire the ?studio side door (5d T7)"
```

---

### Task 8: Frontend — the great deletion

**Files:**
- Move: `apps/web/src/workspace/Markdown.tsx` (+ any test) → `apps/web/src/cards/Markdown.tsx`
- Move: `apps/web/src/workspace/material/` (`SourceDossier.tsx`, `fixtures.ts` + their tests) → `apps/web/src/studio/material/`
- Delete: **all of** `apps/web/src/workspace/` (after the moves), `apps/web/src/agent/`, `apps/web/src/store/`, `apps/web/src/shell/directory/`, `apps/web/src/shell/records/`, `apps/web/src/shell/WorkspaceContainer.tsx` (+ test), `apps/web/src/dev/StorePanel.tsx` (+ test)
- Delete: `apps/web/src/api/turn.ts`, `tasks.ts`, `cards.ts`, `evaluate.ts`, `materials.ts` (+ their tests)
- Modify: `apps/web/src/api/index.ts` (drop the deleted modules from `ApiClient` + the `api` literal)
- Modify: importers of the moved files — `apps/web/src/cards/CardRenderer.tsx:4`, `apps/web/src/studio/state.ts:1`, `apps/web/src/studio/fixtures.ts:1`, `apps/web/src/studio/ViewFrame.tsx:2`, `apps/web/src/dev/MaterialPanel.tsx:3-4`
- Modify: `apps/web/src/dev/DevApp.tsx` (drop the StorePanel tab)

**Interfaces:**
- Consumes: Tasks 5-7 (nothing renders `WorkspaceContainer` / `DirectoryView` / `RecordsView` / the `?studio` branch any more, so these files now have no live importer).
- Produces: `apps/web/src/workspace/`, `apps/web/src/agent/`, `apps/web/src/store/` **do not exist**. `ApiClient` has no `listTasks` / `createTask` / `getTask` / `activateCard` / `submitCard` / `skipCard` / `runEvaluation` / `getEvaluation` / `runTurn` / `listMaterials` / `createMaterial` / `fetchMaterialFromSeed` / `saveScratch`.

**Keep `api/sse.ts`** — shared by `api/studioTurn.ts` and `api/projectCards.ts`.

Three `workspace/` files are genuinely shared with the Studio, so they **move** rather than die — leaving no ghost directory:

| From | To | Because |
|---|---|---|
| `workspace/Markdown.tsx` | `cards/Markdown.tsx` | used by `cards/CardRenderer.tsx` (→ `StudioCardSheet`, all custom renderers) |
| `workspace/material/SourceDossier.tsx` | `studio/material/SourceDossier.tsx` | used by `studio/ViewFrame.tsx`, `dev/MaterialPanel.tsx` |
| `workspace/material/fixtures.ts` | `studio/material/fixtures.ts` | used by `studio/state.ts`, `studio/fixtures.ts`, `dev/MaterialPanel.tsx` |

Move their test files alongside them. `git mv` so history follows.

- [ ] **Step 1: Move the three survivors and fix their importers**

```bash
cd apps/web/src
git mv workspace/Markdown.tsx cards/Markdown.tsx
mkdir -p studio/material && git mv workspace/material/SourceDossier.tsx studio/material/SourceDossier.tsx
git mv workspace/material/fixtures.ts studio/material/fixtures.ts
# plus any co-located test files in workspace/material/
```

Update the five importers listed above to the new paths, and fix the moved files' own relative imports (`SourceDossier.tsx` imports `primitives/annotate` and its sibling `fixtures`).

- [ ] **Step 2: Run the suites that consume them — expect PASS**

Run: `cd apps/web && pnpm test -- studio cards dev`
Expected: PASS. The moves are pure relocation; if a test fails here, an import is wrong.

- [ ] **Step 3: Delete**

```bash
cd apps/web/src
rm -rf workspace agent store shell/directory shell/records
rm -f shell/WorkspaceContainer.tsx shell/WorkspaceContainer.test.tsx
rm -f dev/StorePanel.tsx dev/StorePanel.test.tsx
rm -f api/turn.ts api/turn.test.ts api/tasks.ts api/tasks.test.ts api/cards.ts api/cards.test.ts \
      api/evaluate.ts api/evaluate.test.ts api/materials.ts api/materials.test.ts
```

Then trim `api/index.ts` to only what survives: remove the deleted imports, their `ApiClient` methods, their entries in the `api` literal, the now-unused contract type imports (`Task`, `Evaluation`, `Material`, `TraceEvent` — keep `TraceEvent` if `projectCards.ts` still needs it), the `TaskDetail` / `TurnEvent` re-exports, and the `MaterialFetchError` re-export. Remove the StorePanel tab from `dev/DevApp.tsx` (+ its test).

- [ ] **Step 4: Verify — typecheck is the real gate here**

```bash
cd apps/web && pnpm exec tsc --noEmit && pnpm test
```
Expected: PASS, with a **lower** test count (the deleted suites are gone). `tsc` clean is the proof that nothing surviving referenced the deleted code.

Then confirm nothing dangles:

```bash
cd apps/web/src && grep -rln "workspace/\|from \"../store\|from \"../agent" . || echo "clean"
```
Expected: `clean`.

- [ ] **Step 5: Commit**

```bash
git add -A apps/web && git commit -m "refactor(refactor2): delete the old workspace, store, agent loop, and task API client (5d T8)"
```

---

### Task 9: Cleanup + whole-suite gate

**Files:**
- Modify (conditionally): `packages/contracts/src/*` — the `Task`, `Message`, `Evaluation` schemas + their tests
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md`

**Interfaces:**
- Consumes: everything from Tasks 1-8.

**The contracts cleanup is conditional.** `Task` / `Message` / `Evaluation` existed for the deleted `store`. Delete them **only if** a repo-wide grep and `tsc` prove no importer remains. If anything still imports one — the console, the dev harness, a card contract — **leave it and say so in your report**. Do not force it, and do not delete a schema by rewriting its consumer.

- [ ] **Step 1: Find out whether they're orphaned**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
grep -rn "Task\b\|Message\b\|Evaluation\b" apps/web/src packages/contracts/src --include=*.ts --include=*.tsx | grep -v "packages/contracts/src/task\|CardInstance\|StudioProjection"
```
(Use `find … | xargs grep` if the shell mangles `--include`.) Record what you find.

- [ ] **Step 2: Delete the orphans (if any) and their tests**

If a schema has no importer, remove it from `packages/contracts/src/index.ts` and delete its module + test.

- [ ] **Step 3: Run the whole gate**

```bash
cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test -p 1 ./...
cd ../../apps/web && pnpm test && pnpm exec tsc --noEmit
cd ../../packages/contracts && pnpm test
```
Expected: all green. **Record the new baselines** (Go packages green; web test count; contracts test count) in your report — the pre-5d numbers were web **475** / contracts **181**, and both will now be lower. A drop is correct; a *failure* is not.

- [ ] **Step 4: Update the roadmap**

In `docs/2026-07-11-whole-product-refactor-roadmap.md`: mark Slice 5 **☑ complete** (5a/5b/5c/5c-2/5d all done), append the 5d entry to the per-slice log (spec + plan links, what was delivered, what was deleted, the accepted gaps from spec §6, and the carry-forwards from spec §9), and set **NEXT = 6b**.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "chore(refactor2): contracts cleanup + roadmap — Slice 5 complete (5d T9)"
```

---

## Self-review notes

- **Spec coverage:** §2.1→T4, §2.2→T5, §2.3→T6+T7, §2.4→T6, §3→T5, §4.1→T8, §4.2→T1+T2, §4.3→T9, §5→T3, §6→documented (no code), §7→every task's verify step + T9's gate, §8→T9, §9→T9's roadmap entry.
- **Ordering constraint:** T8 (deletion) must run after T5-T7, which remove the last importers. T1-T3 (backend) are independent of the frontend tasks and could run in any order among themselves, but T2 depends on T1's `Deps` edit and T3 on both.
- **The two tests that carry this slice** are T3's roster test (must fail against the `tasks`-based query — proof the re-point is real) and T6's signin-spy test (must fail against the current `defaultEnsureSession` — proof the fallback is truly gone).

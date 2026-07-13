# Slice 5d — Routing cutover: the Studio becomes the student surface

> Status: design approved 2026-07-13. Part of the whole-product refactor #2
> (`docs/2026-07-11-whole-product-refactor-roadmap.md`). Closes Slice 5.

## 1. Goal

**The Studio stops being a `?studio` side door and becomes the student's real workspace —
inside the real shell, under real auth — and every line of the surface it replaces is deleted.**

Slices 5a–5c-2 + 6 built the Studio behind a query-param escape hatch, next to a still-live
old workspace. That duplication is now the largest source of drift in the codebase: two turn
loops, two card paths, two data models, two auth stories. 5d ends it.

After 5d there is exactly one student surface, one turn loop, one card path.

## 2. What the cutover is

### 2.1 Navigation (binding design)

The binding design's left rail (`docs/design/思维印记_工作区.dc.html:101-141`) is
**课程 · 聊天 · 工作室 · 成长报告 · 设置**. Today's `LeftRail` is
**课程 · 批判思维 · 我的评估 · 设置**.

5d moves the rail to the design for the tabs whose content exists:

| Rail item | Key | Body after 5d |
|---|---|---|
| 课程 | `courses` | unchanged (`CoursesContainer`) |
| **工作室** (was 批判思维) | `studio` (was `tasks`) | **`StudioContainer`** |
| **成长报告** (was 我的评估) | `growth` (was `records`) | deferred-shell placeholder (Slice 9/10 fills it) |
| 设置 | `settings` | unchanged (`SettingsView`) |

**聊天 is NOT added.** It is in the design's rail, but its content is Slice 11. A nav item that
goes nowhere is worse than one that isn't there yet. Slice 11 adds it.

Icons for 工作室 and 成长报告 are already the design's paths (`LeftRail.tsx:29,38`) — only the
labels and keys change. Icons stay inline SVG (project constraint: no `lucide-react`).

### 2.2 The tasks tab dies

`StudentApp` loses its `TaskView` state machine (`directory` ⇄ `workspace`), `DirectoryView`,
and `WorkspaceContainer`. The 工作室 tab renders `StudioContainer` directly. There is no
directory, no task list, no task creation — by decision (§6.3).

### 2.3 Auth: `ensureSession` is deleted, not gated

`StudioContainer.defaultEnsureSession` (`StudioContainer.tsx:46-57`) silently signs in as the
seeded Phoebe on **any** `getMe` failure. This was acceptable behind `?studio`. It is not
acceptable when the Studio is the student surface.

It is **deleted, not patched**: mounted inside `StudentApp`, the Studio is already behind
`AppShell`'s boot → `getMe` → `AuthScreen` gate (`AppShell.tsx:46-81`). The `ensureSession`
prop, the `DEMO_EMAIL`/`DEMO_PASSWORD` constants, and the `signin` import all go with it. The
container becomes what it should be: a component that assumes an authenticated session.

The marketing site's `?trial=1` demo entrance keeps working untouched — `AppShell` already owns
that path (`AppShell.tsx:54-66`), and it lands on the 工作室 tab like any other student.

`Root.tsx` drops the `?studio` branch. `?demo` (the deterministic card Harness) stays.

### 2.4 An honest empty state

Today `StudioContainer` collapses **every** failure — including "this student has no projects"
(`StudioContainer.tsx:86`: `if (list.length === 0) throw new Error("no projects")`) — into
`加载失败，请重试`. Post-cutover, "no projects" is the *first* thing a real (non-seeded)
student hits, and telling them to retry is a lie.

Split the states:

- **`listProjects()` returns `[]`** → an empty state: the student has no project yet, and 5d
  gives them no way to make one. Copy states that plainly, and does not offer a retry button.
- **the request failed** → the existing error state (`加载失败，请重试`).

## 3. 成长报告 stays as a slot

`RecordsView` derives entirely from the localStorage `store` snapshot
(`RecordsView.tsx:39,47-49` → `deriveActivityCalendar/deriveGrowthReviews/deriveCardUsage/
deriveAbility` over `{messages, cards, evaluations}`), and that store is hydrated **only** by
the old task path (`WorkspaceContainer.hydrateTask`, `DirectoryView.putTask`). With the task
path gone, nothing writes to it again — the view would render an empty radar forever, telling
the student they have no thinking history when in fact the history moved.

So: **`RecordsView` + the whole `store/` package are deleted**, and the 成长报告 rail slot
renders a deferred-shell placeholder. This is the same pattern the Studio already uses for its
deferred center-pane views (the fix from Slice 5b's whole-branch review, which caught a
vacuously-true "门禁通过" banner on a stubbed view). Slice 9 (评估 view) and Slice 10 (growth
report) fill it project-backed.

## 4. The retirement

Everything below is reachable **only** from the old task path. It is not merely unused — it is
task-scoped, and the Studio works on projects, so it is structurally unreachable from the new
surface. Git history is the archive.

### 4.1 Frontend deletions

```
shell/WorkspaceContainer.tsx (+test)
shell/directory/                      (DirectoryView, taskCardView, tests)
shell/records/                        (RecordsView + the 4 derivation modules, tests)
agent/                                (createConversation, createEvaluator, useConversation,
                                       useEvaluator, index — the whole folder)
api/turn.ts  api/tasks.ts  api/cards.ts  api/evaluate.ts  api/materials.ts   (+ tests)
store/                                 (createStore, schema, storage, useStore, tests)
dev/StorePanel.tsx (+test)             (+ its DevApp tab)
workspace/                             — everything EXCEPT the three files below
```

**Kept from `workspace/` (genuinely shared with the Studio — verified importers):**

| File | Consumer |
|---|---|
| `workspace/Markdown.tsx` | `cards/CardRenderer.tsx:4` (→ `StudioCardSheet`, all custom renderers) |
| `workspace/material/SourceDossier.tsx` | `studio/ViewFrame.tsx:2`, `dev/MaterialPanel.tsx:3` |
| `workspace/material/fixtures.ts` | `studio/state.ts:1`, `studio/fixtures.ts:1`, `dev/MaterialPanel.tsx:4` |

These three move out of the retiring folder so no dead directory name survives:
`workspace/Markdown.tsx` → `cards/Markdown.tsx`; `workspace/material/` → `studio/material/`.
Imports update accordingly; `apps/web/src/workspace/` ceases to exist.

`api/sse.ts` stays (shared by `api/studioTurn.ts` and `api/projectCards.ts`).

### 4.2 Backend deletions

```
internal/api/turn.go          (POST /tasks/{id}/turn)
internal/api/tasks.go         (GET|POST /tasks, GET /tasks/{id})
internal/api/cards.go         (PATCH|PUT /tasks/{id}/cards/{cid}, POST .../skip)
internal/api/evaluate.go      (POST /tasks/{id}/evaluate, GET /tasks/{id}/evaluation)
internal/api/material.go      (4 × /tasks/{id}/materials* routes)
internal/api/eval_trigger.go  (maybeTriggerMilestoneEval — called only from turn.go, cards.go)
internal/agent/turn.go        (RunTurn, TurnStore, sqlcTurnStore) + turn_test.go
internal/agent/eval*.go       (eval, evalinput, evalprompt, evalrubric, evaljob + tests + testdata)
internal/api/e2e_test.go      (the old-path Phoebe vertical; the new vertical is
                               projectcards_test.go's TestProjectCardSubmit_SurfaceFillMintE2E)
```

Plus: the 13 old routes in `api.go:69-81`; the river `EvaluateWorker` registration and
`EvalKeyResolver` wiring in `cmd/api/main.go:104-132`; the task-scoped sqlc queries that lose
their only caller (`tasks.sql`'s CRUD, `material.sql`'s `ListMaterialsByTask`/`CreateMaterial`/
`UpdateMaterialScratch`, `evaluations.sql`).

**Explicitly preserved** (needed by the live path or by 6b/9/10):

- the `tasks` and `evaluations` **tables and their data** — dropping tables is destructive, and
  `evaluations.project_id` (nullable, added in Slice 0) is what Slice 10 will write to.
- `internal/materialize` — the URL→blocks fetcher. **6b reuses it** for project-scoped ingestion.
- `material.sql`'s `CreateProjectMaterial` + `ListMaterialsByProject` (Slice 0; read side already
  live, feeding the CRAAP anchor generator). 6b wires the write side.
- `agent.NewAnchorGenerator`, `agent.RunAgentStep`, the whole project/studio path.
- `internal/gateway` LLM clients, courses, voice, auth, org/console.

### 4.3 Contracts cleanup

`Task`, `Message`, `Evaluation` Zod schemas exist for the old store. Delete them **only if**
`tsc` + a repo-wide grep prove no importer remains after §4.1. If anything still imports them,
leave them and say so — do not force it.

## 5. Keeping the teacher/admin console truthful

**The finding.** The console's per-student activity all aggregates over `tasks`
(`org.sql:41-52,73-75`):

```sql
LEFT JOIN tasks t           ON t.user_id = u.id
LEFT JOIN evaluations ev    ON ev.task_id = t.id
LEFT JOIN card_instances ci ON ci.task_id = t.id
```

After the cutover **no student ever gets a `tasks` row again**, so `task_count`,
`card_count`, `active_student_count` and `最近活跃` read **0 / never** for every real student,
from Slice 5 all the way through Slice 13. Worse, it is already subtly wrong today: Slice-3
debt keeps `card_instances.task_id` `NOT NULL`, so a Studio card is anchored to the seed's
admin-owned placeholder task (`0018_seed_demo_project.sql:11-23`) — Phoebe's cards currently
count toward the *admin*.

**The fix.** Re-point the aggregates at the project model. No new tables, no migration.

`GetClassRoster` becomes:

```sql
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

`GetSchoolCounts`'s three task-based rows re-point the same way (`project` for `project_count`
and `active_student_count`; `evaluations.project_id` for `evaluation_count`).

`evaluation_count` **stays as a column and honestly reads 0** until Slice 10 writes
project-scoped evaluations. That is a true statement about the student, not a broken one.

**`last_active_at` must become real.** `project.last_active_at` is set at creation and never
updated (`project.sql` has no writer). A roster column that shows the project's birthday as
"最近活跃" is another quiet lie. Add a `TouchProject` query and call it once per project turn
in `postProjectTurn`.

**Rename through the stack:** `task_count` → `project_count` in the sqlc row, the Go DTO, the
console TS types, `ClassDetailView`'s `任务` column header → `项目`, `OverviewView`'s
`{key:"task", label:"任务"}` → `{key:"project", label:"项目"}`, and their tests.

## 6. Accepted gaps

These are consequences of the cutover, not of the deletion. Each is named here so the next
slice inherits it explicitly.

### 6.1 Material ingestion → Slice 6b

The old surface could paste a source URL and fetch it (`POST /tasks/{id}/materials/from-seed`
+ `MaterialPane`). Those endpoints are **task-scoped**: the Studio cannot call them today and
never could. Deleting them destroys no capability the Studio ever had.

Project-scoped ingestion is **6b**, and the machinery it needs survives 5d untouched:
`internal/materialize` (URL→blocks) and the `CreateProjectMaterial` query. 6b is a wiring job.

In the window, the seeded project ships with its materials, so the CRAAP keystone
(surface → fill → mint) keeps running end-to-end.

### 6.2 Evaluation → Slice 9/10

The old task-scoped rubric evaluator is deleted. The new assessor is a different engine over
event-stream projections (roadmap Slice 10). This is a planned deferral; the 成长报告 slot
(§3) is where it lands.

### 6.3 Project creation → its own slice

There is no `POST /projects` and 5d does not add one (decided: keep 5d a true cutover). A
student with zero projects sees the empty state (§2.4). Creation + intake (title /
qualification / prompt → S0 onboarding) needs a real design and gets its own slice.

## 7. Testing strategy

The bulk of this slice is deletion, and the test suite is the proof that nothing load-bearing
went with it. Three kinds of test carry the weight:

1. **The suites that must stay green untouched** — `studio/*`, `api/projects*`,
   `api/studioturn*`, `api/projectcards*` (incl. the Slice 6 surface→fill→submit→mint E2E),
   `cards/*`, courses, voice, auth. If a deletion breaks one of these, the deletion was wrong.
2. **New tests for the new behavior:**
   - `StudentApp`: the 工作室 tab renders the Studio; there is no directory/workspace path left.
   - `LeftRail`: four items, the two new labels, `studio`/`growth` keys.
   - `StudioContainer`: `listProjects() → []` renders the **empty** state (not the error state);
     a rejected request renders the error state. A test that would fail if the two were merged.
   - `StudioContainer`: no `ensureSession` — an unauthenticated mount does **not** call `signin`.
     (Guards the exact regression the deleted fallback was.)
   - 成长报告 placeholder renders.
   - `GetClassRoster` / `GetSchoolCounts` over a seeded project + card_instances: counts come
     from the **project** model. Written as a testcontainers store test that **fails against the
     current `tasks`-based query** — that failure is the proof the re-point is real, not cosmetic.
   - `TouchProject`: a project turn advances `last_active_at`.
3. **Route-surface assertion:** the old `/api/v1/tasks/**` routes return 404. A cheap regression
   fence against a half-deleted router.

Gate (all must pass before merge):
`CGO_ENABLED=0 go test -p 1 ./...` (serialized — parallel hangs on Docker contention) ·
`pnpm -C apps/web test` · `pnpm -C packages/contracts test` · `pnpm -C apps/web exec tsc --noEmit`.
Test counts will **drop** (deleted suites) — that is expected; the plan records the expected new
baselines rather than treating a drop as a failure.

## 8. Acceptance

A student signs in through the normal auth screen, lands on **工作室**, sees their project's
Studio, talks to the coach, gets a CRAAP card, fills it, and mints an evidence node — with no
`?studio`, no silent Phoebe fallback, and no old workspace anywhere in the tree. A student with
no project sees an honest empty state. A teacher opens their class and sees per-student counts
that reflect the project model. `apps/web/src/workspace/`, `apps/web/src/store/`,
`apps/web/src/agent/`, `agent.RunTurn`, and every `/api/v1/tasks/*` route are gone.

## 9. Carry-forwards out of 5d

- **Slice-3 debt still owed:** `card_instances.task_id` / `material.task_id` are still
  `NOT NULL`, so the seed's admin-owned placeholder task row must stay. With the task *surface*
  gone, the NULLABLE migration is now unblocked and should be done before the seed's placeholder
  confuses anyone. (Not in 5d: it ripples into `Material`/`CardInstance` Go types → `pgtype.UUID`
  and project-scoped `GetCardInstance`.)
- Unique partial index on `chat_thread.seeded_project_id` (getOrCreateThread TOCTOU).
- Onboarding live producer (`agent.Intake` writes `{text}`; the reader wants
  `{restate_prompt, rows}`).
- Live `gate` passed/total counts (`studioturn` hardcodes `0,0`).
- A live anchor label in the coach rail (runtime stores `{kind,id}` → chip-only).

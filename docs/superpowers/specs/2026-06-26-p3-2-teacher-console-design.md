# P3.2 · Teacher/Admin Class Console — Design Spec

> **Phase:** P3.2 (second sub-phase of P3 「完整组织端」). Frontend-only.
> **Authored:** 2026-06-26. **Status:** approved, ready for plan.
> **North-star:** `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`.
> **Consumes:** the P3.1 org backend (`docs/superpowers/specs/2026-06-26-p3-1-org-backend-rbac-design.md`),
> merged to `main` @ 859d8ef.
>
> P3 is decomposed into **P3.1 org backend + RBAC** (done), **P3.2 teacher/admin
> class console** (this spec), **P3.3 admin console** (teacher-invites / import /
> overview). Each is its own spec→plan→build cycle. This spec covers P3.2 only.

## Goal

A frontend console that **teachers and admins** land on after login, built on the
five `teacherOrAdmin` class endpoints shipped in P3.1. Students keep the existing
workspace app unchanged. The console is **aggregate-only**: teachers/admins see
class rosters and per-student activity *signals* (counts, timestamps), never student
work contents — honoring the P3.1 visibility decision and the platform's restraint
iron laws.

## Scope decisions (locked during brainstorming)

1. **Audience:** the console serves both `teacher` and `admin` roles via the shared
   `teacherOrAdmin` endpoints. Admin's `GET /classes` returns school-wide; teacher's
   returns own classes. The console renders whatever the backend scopes — no
   role-specific data branching beyond header copy.
2. **Layout:** mirror the student app's `LeftRail` pattern — a left rail with two
   tabs, **班级 / 设置**. Classes list → click a class → roster detail in the main panel.
3. **Roster presentation:** a **signal table** (姓名 · 邮箱 · 最近活跃 · 任务 · 评估 ·
   卡片) with a remove-student action per row. Rows are **non-clickable** (no
   student-work detail page exists, by design); a caption states 无学生作品详情.
4. **Class creation is teacher-only in P3.2.** The backend's `POST /classes`
   **requires** `teacher_user_id` when the caller is an admin (returns `400 需要指定
   班级教师` otherwise) — it forbids a teacher-less class. Assigning a teacher needs a
   teacher-picker/list endpoint that does not exist yet. So the **[+ 新建班级] action
   is shown for teachers only**; admins land on a read/manage view (list school-wide,
   open detail, rename/rotate-code/remove-student) but create + teacher-assignment
   both defer to P3.3. (Recorded as carry-forward.)

## Non-goals (P3.2)

- No admin-only screens (teacher-invites, bulk import, school overview) — those are
  P3.3, consuming the `adminOnly` endpoints.
- No admin class-creation (the backend requires a teacher assignee; deferred — see
  decision 4).
- No per-student work-detail reads (process tree, transcript, eval narrative) — waits
  on the evaluation-data-model brainstorm, same as P3.1.
- No admin teacher-assignment UI (deferred, see decision 4).
- No new design system — reuse the student palette and `LeftRail` idiom verbatim.
- No class soft-archive (deferred from P3.1; remove-student covers roster correction).
- No CSV upload (that is the P3.3 import flow).

---

## Architecture

### Role-routing refactor (`apps/web/src/shell/AppShell.tsx`)

Today `AppShell` does three jobs in one component: boot (`client.getMe()`), the
auth gate, and renders the student tab UI inline (current lines 44–86). Split by
responsibility:

- **`AppShell`** keeps the boot effect + auth gate, then **routes on
  `session.getUser().role`**:
  - `student` → `<StudentApp …>` (the extracted current UI).
  - `teacher` | `admin` → `<ConsoleShell …>`.
  - Unknown/missing role → fall back to `StudentApp` (defensive; existing behavior).
- **`StudentApp`** (`apps/web/src/shell/StudentApp.tsx`) — the current student tab
  UI (LeftRail tasks/records/settings + DirectoryView/WorkspaceContainer/RecordsView/
  SettingsView), lifted out **verbatim** with the same props (`store`, `session`,
  `client`). No behavior change — this is a pure extraction.

This keeps the boot/auth gate single-sourced (DRY) and gives each role a focused
shell.

### Console shell (`apps/web/src/console/`)

A new directory parallel to `shell/` and `workspace/`:

- **`ConsoleShell.tsx`** — left rail (班级 / 设置) + main panel. Holds the
  view state (`classes` list vs `class-detail`, and the active tab). Mirrors
  `AppShell`'s panel-switch structure.
- **`ConsoleRail.tsx`** — the left rail for the console, styled identically to
  `shell/LeftRail.tsx` (same logo, same nav-item visual treatment, same avatar dot),
  but with the two console tabs. (A focused copy rather than over-generalizing
  `LeftRail`, which is hardcoded to the student tabs.)
- **`ClassesView.tsx`** — the 班级 landing.
- **`ClassDetailView.tsx`** — the roster detail.
- The 设置 tab reuses the existing **`shell/settings/SettingsView`** (already takes
  `session` / `user` / `onLogout`).

### API client (`apps/web/src/api/classes.ts`)

New module alongside `api/auth.ts` / `api/tasks.ts`, all via the existing `apiFetch`
(cookie session, shared error envelope):

These match the P3.1 `classes_dto.go` DTOs **verbatim** (confirmed against the merged
backend — `classDTO` and `rosterEntryDTO`):

```ts
// classDTO — returned by list (in {classes:[…]}), create/patch (in {class}),
// and as the {class} half of detail. NOTE: the list payload carries NO student
// count or last-active — those live only in the detail roster.
export interface ClassSummary {
  id: string;
  name: string;
  join_code: string;
  school_id: string;
  created_at: string; // ISO
}

// rosterEntryDTO — note the field is `id` (the student's user id), not `user_id`.
export interface RosterStudent {
  id: string;
  display_name: string;
  email: string;
  last_active_at: string | null; // null = never
  task_count: number;
  evaluation_count: number;
  card_count: number;
}

export interface ClassDetail {
  class: ClassSummary;
  roster: RosterStudent[];
}

listClasses(): Promise<ClassSummary[]>;              // GET    /api/v1/classes        → {classes:[…]}
createClass(input: { name: string }): Promise<ClassSummary>; // POST   /api/v1/classes        → {class}
getClass(id: string): Promise<ClassDetail>;          // GET    /api/v1/classes/{id}   → {class, roster:[…]}
renameClass(id, name): Promise<ClassSummary>;        // PATCH  /api/v1/classes/{id}    {name}               → {class}
regenerateJoinCode(id): Promise<ClassSummary>;       // PATCH  /api/v1/classes/{id}    {regenerate_join_code:true} → {class}
removeEnrollment(id, userId): Promise<void>;         // DELETE /api/v1/classes/{id}/enrollments/{userId}     → 204
```

The plan task that writes the client must re-read `classes_dto.go` and match
field-for-field; the DTO is authoritative if it has drifted.

---

## Screens

### Classes list (`ClassesView`)

- Card grid from `listClasses()`. The list payload is bare, so each card shows what
  `ClassSummary` carries: class **name**, **join code** (with a copy affordance), and
  **创建于** (humanized `created_at`). **Per-class student count is NOT on the list**
  (the backend doesn't return it there) — it appears in the detail view as the roster
  length. Showing a count on the card would require a backend `COUNT` column;
  recorded as carry-forward, not built here.
- Header: title + **[+ 新建班级]** — the create button is **shown for `teacher` only**
  (admin create defers to P3.3, decision 4). Title is role-aware: **我的班级**
  (teacher) / **全校班级** (admin) — role read from `session.getUser().role`.
- **Empty state** (zero classes): teacher → a calm prompt to create the first class;
  admin → note that no classes exist in the school yet.
- Clicking a card → `ClassDetailView` for that class id.

### Create-class flow (teacher only)

`[+ 新建班级]` → inline name input (or small modal) → `createClass({name})` →
on success the list refreshes and the new class's **join code is surfaced** (it is in
the create response's `class.join_code`) for the teacher to copy and distribute.
Validation: non-empty trimmed name; surface the backend error envelope on failure.
The admin branch of `POST /classes` (which requires `teacher_user_id`) is **not
exercised** by this flow.

### Class detail (`ClassDetailView`)

From `getClass(id)`:

- **Header:** class name + **改名** (inline edit → `renameClass`) + a **join-code
  chip** with **复制** and **轮换** actions. 轮换 → confirm dialog (it invalidates the
  current code) → `regenerateJoinCode` → chip updates.
- **Roster signal table:** columns 姓名 · 邮箱 · 最近活跃 · 任务 · 评估 · 卡片, one row
  per `RosterStudent`, plus a remove **✕** per row. Counts shown as integers;
  最近活跃 humanized ("从未" when null). **Rows are non-clickable**; a caption under
  the table reads 「行不可点入 — 暂无学生作品详情页」.
- **Remove student:** ✕ → confirm dialog → `removeEnrollment(id, userId)` → row
  drops. Removes the enrollment only (never the account or their tasks) — copy in the
  confirm dialog says so.
- **Empty roster** state: note that no students have joined; show the join code to
  share.
- **Back** to the classes list.

---

## Data flow

`ConsoleShell` mounts → `listClasses()` → render grid. Selecting a class → `getClass(id)`
→ render detail. Mutations (`createClass` / `renameClass` / `regenerateJoinCode` /
`removeEnrollment`) re-fetch or patch local state on success. Loading and error states
per fetch (reuse whatever spinner/skeleton idiom the student app already uses; a plain
calm placeholder is acceptable). All requests carry the session cookie via `apiFetch`;
a 401 surfaces by falling back to the auth screen the same way the student app does.

## Error handling

- Network/5xx → inline error with a retry affordance; never a blank screen.
- 403 (shouldn't happen post-role-gate) and 404 (class not owned / not in school) →
  treated as "class unavailable", route back to the list.
- Mutation failures → keep the dialog open, show the backend message; no optimistic
  state left stranded.
- No secrets or internals ever rendered — the backend already strips them; the client
  shows only the envelope's user-facing message.

## Testing

Follow the existing vitest + testing-library pattern (render with an injected fake
`client`, no network):

- **Role routing:** `AppShell` renders `StudentApp` for `student`, `ConsoleShell` for
  `teacher` and `admin`; unknown role falls back to `StudentApp`.
- **`StudentApp` extraction:** existing AppShell behavior tests continue to pass
  (the extraction is behavior-preserving) — migrate/retarget them as needed.
- **ClassesView:** renders classes from a fake `listClasses`; role-aware header copy;
  empty state; create flow calls `createClass` and surfaces the join code.
- **ClassDetailView:** renders the roster table from a fake `getClass`; "从未" for null
  activity; rename calls `renameClass`; 轮换 confirms then calls `regenerateJoinCode`;
  remove confirms then calls `removeEnrollment` and drops the row; empty-roster state.
- **API client:** each function hits the right method + path and parses the response
  shape (mock `apiFetch`).

## Carry-forward (deferred, recorded — to `docs/遗留项追踪_Carryforward.md`)

- **Admin class-creation + teacher-assignment** — the backend requires a
  `teacher_user_id` for admin-created classes, which needs a teacher-list/picker
  endpoint that doesn't exist; both defer to P3.3.
- **Per-class student count on the classes-list card** — `GET /classes` returns bare
  class objects; showing a count needs a backend `COUNT(enrollments)` column on the
  list query. Deferred; the count is available in the detail view meanwhile.
- **Admin-only screens** (teacher-invites, bulk import, school overview) — P3.3.
- **Per-student work-detail visibility** (process tree / transcript / eval narrative)
  — waits on the evaluation-data-model brainstorm.
- **Class soft-archive** — still deferred; remove-student covers roster correction.

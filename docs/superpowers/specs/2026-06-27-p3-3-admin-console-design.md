# P3.3 · Admin Console — Design Spec

> **Phase:** P3.3 (third and final sub-phase of P3 「完整组织端」). Frontend +
> a small backend increment.
> **Authored:** 2026-06-27. **Status:** approved, ready for plan.
> **North-star:** `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`.
> **Consumes:** the P3.1 org backend (`…/2026-06-26-p3-1-org-backend-rbac-design.md`,
> merged @ 859d8ef) and the P3.2 console (`…/2026-06-26-p3-2-teacher-console-design.md`,
> merged @ 9344281).
>
> P3 decomposition: **P3.1 org backend + RBAC** (done), **P3.2 teacher/admin class
> console** (done), **P3.3 admin console** (this spec). This spec finishes P3.

## Goal

Extend the existing console — which admins already reach post-P3.2 — with admin-only
screens: a school **overview** dashboard, **teacher** management (invite minting +
the school's teacher list), bulk **CSV import**, and class **teacher-assignment**.
Most of it sits on the P3.1 `adminOnly` endpoints already shipped; teacher-assignment
needs a small backend increment (a teacher-list endpoint + an assign-to-existing-class
endpoint + surfacing a class's teachers). **No database migration** — the increment
reuses existing tables.

## Scope decisions (locked during brainstorming)

1. **Audience & nav:** the console's left rail becomes **role-aware**. Admin sees
   概览 / 班级 / 教师 / 导入 / 设置 and lands on **概览**; teacher still sees only
   班级 / 设置 and lands on 班级 (unchanged from P3.2).
2. **Teacher-assignment is full-coverage:** admin can assign a teacher both **at class
   creation** (the existing `POST /classes` `teacher_user_id` path, now reachable from
   the admin UI with a teacher-picker) **and to any existing class** (a new endpoint),
   which closes the import gap — bulk-import creates teacher-less classes.
3. **Set semantics for teachers:** a class may have **zero or more** teachers. Assign
   **adds** a teacher (idempotent upsert); remove (×) drops one (reuses the existing
   `DELETE /classes/{id}/enrollments/{userId}`). No single-teacher "replace".
4. **Assignment is `adminOnly`:** the assign endpoint and the class-detail teacher
   section are admin-only. A teacher cannot reassign their own class's co-teachers in
   P3.3 (out of scope).
5. **CSV parsing lives in the frontend** (carry-forward from P3.1/P3.2): the import
   endpoint takes already-parsed JSON rows; `ImportView` parses the file client-side.

## Non-goals (P3.3)

- No new database migration (teacher-assignment reuses `enrollments`/`users`/`classes`).
- No per-student work-detail reads (process tree / transcript / eval narrative) — still
  waits on the evaluation-data-model brainstorm.
- No teacher self-service co-teacher management (assignment is admin-only).
- No real email for invites (Mailer still stubbed); no rate-limiting / CSRF (deferred
  since P2).
- No single-teacher enforcement, no class soft-archive.

---

## Backend increment (no migration)

### New queries (`internal/store/queries/org.sql`)

- **`ListTeachersBySchool`** — `SELECT id, display_name, email FROM users WHERE
  school_id = $1 AND role = 'teacher' ORDER BY display_name`.
- **`GetClassTeachers`** — teachers enrolled in a class:
  `SELECT u.id, u.display_name, u.email FROM enrollments e JOIN users u ON u.id =
  e.user_id WHERE e.class_id = $1 AND e.role_in_class = 'teacher' ORDER BY
  u.display_name`.
- **`AssignClassTeacher`** — idempotent + corrective upsert (the
  `enrollments_user_class_key` UNIQUE(user_id,class_id) makes this safe):
  `INSERT INTO enrollments (user_id, class_id, role_in_class) VALUES ($1,$2,'teacher')
  ON CONFLICT (user_id, class_id) DO UPDATE SET role_in_class = 'teacher'`.
- **`DeleteClassTeacher`** (`:execrows`) — `DELETE FROM enrollments WHERE class_id = $1
  AND user_id = $2 AND role_in_class = 'teacher'`. **A dedicated query is required**:
  the existing `DeleteEnrollment` filters `role_in_class = 'student'` (a P3.2 safety
  property so remove-student can't nuke a teacher), so it cannot remove a teacher.

### New / changed handlers (`internal/api`)

- **`GET /admin/teachers`** (adminOnly) → `200 {teachers:[{id,display_name,email}]}`
  via `ListTeachersBySchool(admin.school_id)`. School from the authenticated admin,
  never the request.
- **`POST /classes/{id}/teachers`** (adminOnly) — body `{teacher_user_id}`. Handler:
  parse `id` (bad → 404); `GetClassByID` (ErrNoRows → 404; real error → 500); if
  `class.school_id != admin.school_id` → **404** (existence-hiding, mirrors the P3.1
  tenancy guards); parse `teacher_user_id` (bad → 400 validation_failed); validate it
  is a teacher in the admin's school via `GetUserByIDInSchool` (not found / wrong role
  → 400 「指定的教师无效」); `AssignClassTeacher`; return `200 {teachers:[…]}` (the
  class's full teacher list after the assign, via `GetClassTeachers`).
- **Extend `GET /classes/{id}`** — the existing `getClass` response gains a
  `teachers` array: `{class, roster, teachers:[{id,display_name,email}]}` via
  `GetClassTeachers`. Additive — the P3.2 frontend ignores the new field.

### Routing & authz

`GET /admin/teachers` joins the existing `adminOnly` group
(`RequireUser(RequireRole("admin"))`). `POST /classes/{id}/teachers` registers under
`adminOnly` (note: it lives on the `/classes` path but is admin-gated, unlike the
`teacherOrAdmin` class routes). Binding RBAC mapping stays: role-gate → 403,
cross-tenant/not-found → 404, unauth → 401; `school_id` always from the authenticated
admin.

### Remove-teacher

- **`DELETE /classes/{id}/teachers/{userId}`** (adminOnly) — same tenancy check as the
  assign handler (load class; `class.school_id != admin.school_id` → 404); calls
  `DeleteClassTeacher`; `204`. This is a **new** endpoint (the existing
  `DELETE …/enrollments/{userId}` is student-only and can't remove a teacher).

---

## Frontend

### Role-aware shell

- **`ConsoleRail`** (`apps/web/src/console/ConsoleRail.tsx`) — takes the tab set by
  role. `ConsoleTab` widens to `"overview" | "classes" | "teachers" | "import" |
  "settings"`. Admin renders all five (概览/班级/教师/导入/设置); teacher renders the
  two it has today (班级/设置). The rail is otherwise the same visual idiom (logo, box
  states, no React import).
- **`ConsoleShell`** (`apps/web/src/console/ConsoleShell.tsx`) — initial tab is
  `overview` for admin, `classes` for teacher. Routes the three new admin tabs to
  `OverviewView` / `TeachersView` / `ImportView`; 班级/设置 unchanged. The admin tabs
  are unreachable for a teacher (rail doesn't render them), and each admin view also
  reads `role` defensively.

### API client (`apps/web/src/api/admin.ts` + extend `classes.ts`)

New `admin.ts`, all via `apiFetch`:
- `getOverview(): Promise<Overview>` — `GET /admin/overview` → `{counts:{student,
  teacher,class,task,evaluation,active_student}, usage_by_tier:[{tier,prompt_tokens,
  completion_tokens,cost}]}` (field names verbatim from `overview.go`; `cost` is a
  string).
- `listTeacherInvites(): Promise<TeacherInvite[]>` — `GET /admin/teacher-invites` →
  `{invites:[{id,code,expires_at,created_at,email?}]}`.
- `createTeacherInvite(input:{email?:string;expires_days?:number}): Promise<{code:
  string;expires_at:string}>` — `POST /admin/teacher-invites`.
- `adminImport(rows): Promise<{classes:[{name,join_code}];teacher_invites:[{email,
  code}]}>` — `POST /admin/import`, body `{rows}` where each row is `{class:string;
  teacher_email?:string;student_email?:string}`.
- `listTeachers(): Promise<Teacher[]>` — `GET /admin/teachers` →
  `{teachers:[{id,display_name,email}]}`.
- `assignTeacher(classId, teacherUserId): Promise<{teachers:Teacher[]}>` —
  `POST /classes/{id}/teachers`.
- `removeTeacher(classId, userId): Promise<void>` — `DELETE /classes/{id}/teachers/
  {userId}` (the new teacher-specific endpoint; NOT the student `removeEnrollment`).

Extend `classes.ts`: `ClassDetail` gains `teachers: Teacher[]`. `Teacher =
{id,display_name,email}`.

### Screens

- **`OverviewView`** — `getOverview()`. Six stat cards (学生 / 教师 / 班级 / 任务 /
  评估 / 活跃学生) from `counts`, then a usage-by-tier table (档位 · 输入 token · 输出
  token · 成本) from `usage_by_tier`; empty `usage_by_tier` shows a 「暂无用量」 note.
  Read-only. Loading + error+retry states.
- **`TeachersView`** — three blocks: (1) the school's teachers from `listTeachers()`
  (name + email); (2) pending invites from `listTeacherInvites()` (code, bound email if
  any, expiry); (3) a **mint form** — optional email, optional expiry-days (placeholder
  「14」), submit → `createTeacherInvite` → surface the new `T-` code prominently for
  the admin to copy. Refresh the invite list on success.
- **`ImportView`** — a file input (accepts `.csv`); on select, parse with `csv.ts`
  into rows, show a **preview table** (class · teacher_email · student_email) with a
  per-row validity hint (class name required). A 「导入」 button posts the parsed rows
  to `adminImport`; on success render the **code sheet** — a classes table
  (名称 · 邀请码) and a teacher-invites table (邮箱 · 邀请码) — copyable. A
  `validation_failed` response with `details.row` highlights the offending preview row.
  Empty/garbled file → inline 「无法解析文件」.
- **`csv.ts`** — `parseCsv(text: string): ImportRow[]`. Splits lines, treats the first
  line as a header mapping the columns `class` / `teacher_email` / `student_email`
  (case-insensitive, order-independent), trims cells, supports double-quoted fields
  containing commas (`"a,b"`) and skips blank lines. Pure + unit-tested. Documented
  limitation: no escaped-quote-within-quote (`""`) handling — out of scope for a
  school roster sheet; a malformed file surfaces as a parse error, never a silent
  mis-parse.

### Class screens — admin extensions

- **`ClassesView`** — the `+ 新建班级` button now also renders for admin, but the admin
  create flow requires a **teacher-picker** (a `<select>` populated from
  `listTeachers()`); on submit it calls `createClass` with `teacher_user_id` (the admin
  branch of the existing endpoint). The teacher path is unchanged (name only, no
  picker). If the school has no teachers yet, the admin create form explains a teacher
  must be invited first (links conceptually to the 教师 tab) and the submit is disabled.
- **`ClassDetailView`** — an admin-only **教师** section above the roster: the class's
  `teachers` (from the extended `getClass`), each with a remove × (→ `removeTeacher`
  → refetch), plus an **assign** control (picker from `listTeachers()` → `assignTeacher`
  → refetch). Teachers see no teacher section (the field may be present but the section
  is role-gated). The aggregate-only roster and all P3.2 behavior are unchanged.

---

## Data flow, errors, testing

- **Data flow:** each view fetches on mount via its injected `client` slice; mutations
  refetch or patch local state on success. Same prop-injection pattern as P3.2 (tests
  inject a fake client; no network).
- **Errors:** inline calm error + 重试 per fetch; surface the backend envelope's
  user-facing `ApiError.message`; never a blank screen. Mutation failures stay inline
  (don't replace the view — the P3.2 `mutationError` lesson).
- **Testing:**
  - **Backend** (testcontainers, `-p 1`): `ListTeachersBySchool` scoping; assign happy
    path + idempotency (re-assign is a no-op) + authz matrix (student → 403; teacher →
    403 on the adminOnly assign; admin cross-school → 404; bad teacher_user_id → 400);
    `getClass` now returns `teachers`.
  - **Frontend** (Vitest + testing-library, fake client): rail role-gating (admin shows
    5 tabs, teacher 2); ConsoleShell admin landing on 概览; OverviewView renders counts +
    usage rows + empty-usage note; TeachersView lists teachers/invites and mint→code;
    `parseCsv` table tests (header order, quoted commas, blank lines, malformed →
    error); ImportView parse→preview→submit→code-sheet + row validation; ClassesView
    admin create-with-picker (calls `createClass` with `teacher_user_id`) + no-teachers
    disabled state; ClassDetailView admin teacher section assign/remove and teacher-role
    sees no section.

## Carry-forward (deferred, recorded — to `docs/遗留项追踪_Carryforward.md`)

- Teacher self-service co-teacher management (assignment is admin-only in P3.3).
- Per-student work-detail visibility — waits on the evaluation-data-model brainstorm.
- Real email for invites; rate-limiting + CSRF (since P2).
- CSV escaped-quote (`""`) handling; class soft-archive; single-teacher enforcement.
- Console a11y (clickable-div cards + rail tabs lack `tablist`/keyboard) — shared with
  the existing `LeftRail`, addressed wholesale later if desired.

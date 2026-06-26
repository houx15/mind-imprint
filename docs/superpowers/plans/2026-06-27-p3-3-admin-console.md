# P3.3 · Admin Console Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the admin console — a school overview dashboard, teacher management (invites + the school's teachers), bulk CSV import, and class teacher-assignment — to the existing console, plus a no-migration backend increment for the teacher list/assign/remove endpoints.

**Architecture:** Backend first (3 Go tasks: new `org.sql` queries via sqlc, a `GET /admin/teachers` endpoint, `getClass` extended with `teachers`, and assign/remove-teacher endpoints under `adminOnly`). Then frontend: an `api/admin.ts` client + `csv.ts` parser, three new admin views (`OverviewView`/`TeachersView`/`ImportView`), a role-aware `ConsoleRail`, admin routing in `ConsoleShell`, and admin extensions to the P3.2 `ClassesView` (create-with-teacher-picker) and `ClassDetailView` (admin teacher section). Build leaves first, integrate last.

**Tech Stack:** Go 1.26 (net/http, pgx/sqlc, goose), testcontainers-go; React 18 + TypeScript + Vite + Vitest + @testing-library/react; the existing `apiFetch` client.

## Global Constraints

- **No database migration** — teacher-assignment reuses `enrollments`/`users`/`classes`. `enrollments` has `UNIQUE(user_id, class_id)` (index `enrollments_user_class_key`), making the assign upsert safe.
- **RBAC mapping (binding):** role-gate fail → **403**; cross-tenant / not-owned / not-found → **404** (existence-hiding); unauthenticated → **401**. `school_id` is ALWAYS from the authenticated admin, NEVER the request body.
- **Teacher-removal needs a dedicated query** — the existing `DeleteEnrollment` filters `role_in_class = 'student'` and cannot remove a teacher; `DeleteClassTeacher` filters `'teacher'`.
- **Assignment is `adminOnly`** (the assign/remove-teacher endpoints + the class-detail teacher section). Set semantics: a class may have ≥0 teachers; assign adds (idempotent upsert), remove drops one.
- **DTO field names match the backend verbatim** — overview `{counts:{student,teacher,class,task,evaluation,active_student}, usage_by_tier:[{tier,prompt_tokens,completion_tokens,cost}]}` (`cost` is a string); teacher-invites `{invites:[{id,code,expires_at,created_at,email?}]}`; import `{classes:[{name,join_code}], teacher_invites:[{email,code}]}`; teacher `{id,display_name,email}`.
- **CSV parsing is client-side** (`csv.ts`); the import endpoint takes parsed JSON rows.
- **Reuse the student palette + console idiom** — inline hex tokens (`#2A3B7A`, `#1C2333`, `#8A92A3`, `#EAECF2`, `#F3F4F8`, `#EDEFF9`, `#C76B6B`, `#D7DCF2`), `React.CSSProperties` via the global `@types/react` namespace (NO React import), prop-injected `client` slices for tests.
- **Backend gate:** from `apps/api`, `go vet ./...` + `DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./...` (testcontainers; `-p 1` required). Regenerate sqlc with `make sqlc` (sets `CGO_ENABLED=0`).
- **Frontend gate:** from `apps/web`, `pnpm typecheck && pnpm test`.
- **Never stage the repo-root `package.json`** (pre-existing `M`). Use explicit `git add` paths.
- **Commit trailer:** every commit ends with `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.

---

## File Structure

**Backend (`apps/api/`):**
- `internal/store/queries/org.sql` (modify) — +4 queries: `ListTeachersBySchool`, `GetClassTeachers`, `AssignClassTeacher`, `DeleteClassTeacher`. Regenerate `internal/store/sqlc/*` via `make sqlc`.
- `internal/api/classes_dto.go` (modify) — +`teacherDTO`.
- `internal/api/admin_teachers.go` (create) — `adminListTeachers` handler.
- `internal/api/classes.go` (modify) — `getClass` includes `teachers`.
- `internal/api/class_teachers.go` (create) — `assignClassTeacher` + `removeClassTeacher`.
- `internal/api/api.go` (modify) — 3 new routes.
- Tests: `internal/api/admin_teachers_test.go`, `class_teachers_test.go`; extend `classes_test.go`.

**Frontend (`apps/web/src/`):**
- `api/admin.ts` (create) — overview/invites/import/teachers/assign/remove client + types.
- `api/classes.ts` (modify) — `Teacher` type, `ClassDetail.teachers`, `createClass` accepts `teacher_user_id`.
- `api/index.ts` (modify) — wire new methods into `ApiClient` + `api`.
- `console/csv.ts` (create) — `parseCsv`.
- `console/OverviewView.tsx`, `console/TeachersView.tsx`, `console/ImportView.tsx` (create).
- `console/ConsoleRail.tsx` (modify) — role-aware tabs.
- `console/ConsoleShell.tsx` (modify) — admin routing + landing; widen `ConsoleClient`.
- `console/ClassesView.tsx` (modify) — admin create-with-teacher-picker.
- `console/ClassDetailView.tsx` (modify) — admin teacher section.
- `shell/AppShell.tsx` (modify) — widen `ShellClient`.
- `docs/遗留项追踪_Carryforward.md` (modify) — P3.3 section (Task 12).
- Tests beside each module; update `shell/AppShell.test.tsx` + `console/ConsoleShell.test.tsx` (Task 10).

---

### Task 1: Backend — org queries + `GET /admin/teachers`

**Files:**
- Modify: `apps/api/internal/store/queries/org.sql`
- Modify: `apps/api/internal/api/classes_dto.go`
- Create: `apps/api/internal/api/admin_teachers.go`
- Create: `apps/api/internal/api/admin_teachers_test.go`
- Modify: `apps/api/internal/api/api.go`
- Regenerate: `apps/api/internal/store/sqlc/*` (via `make sqlc`)

**Interfaces:**
- Produces (sqlc, used by Tasks 2–3 too): `ListTeachersBySchool(ctx, schoolID uuid.UUID) ([]ListTeachersBySchoolRow, error)` where the row has `ID uuid.UUID`, `DisplayName string`, `Email string`; `GetClassTeachers(ctx, classID uuid.UUID) ([]GetClassTeachersRow, error)` (same fields); `AssignClassTeacher(ctx, AssignClassTeacherParams{UserID, ClassID uuid.UUID}) error`; `DeleteClassTeacher(ctx, DeleteClassTeacherParams{ClassID, UserID uuid.UUID}) (int64, error)`.
- Produces (Go): `teacherDTO{ID,DisplayName,Email string}` in `classes_dto.go`; `adminListTeachers` handler; route `GET /api/v1/admin/teachers` under `adminOnly`.

- [ ] **Step 1: Add the four queries**

Append to `apps/api/internal/store/queries/org.sql`:

```sql
-- name: ListTeachersBySchool :many
SELECT id, display_name, email FROM users
WHERE school_id = $1 AND role = 'teacher'
ORDER BY display_name;

-- name: GetClassTeachers :many
SELECT u.id, u.display_name, u.email
FROM enrollments e
JOIN users u ON u.id = e.user_id
WHERE e.class_id = $1 AND e.role_in_class = 'teacher'
ORDER BY u.display_name;

-- name: AssignClassTeacher :exec
INSERT INTO enrollments (user_id, class_id, role_in_class)
VALUES ($1, $2, 'teacher')
ON CONFLICT (user_id, class_id) DO UPDATE SET role_in_class = 'teacher';

-- name: DeleteClassTeacher :execrows
DELETE FROM enrollments WHERE class_id = $1 AND user_id = $2 AND role_in_class = 'teacher';
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc`
Expected: no errors; `internal/store/sqlc/org.sql.go` (or the generated file) now defines the four methods + `AssignClassTeacherParams`, `DeleteClassTeacherParams`, `ListTeachersBySchoolRow`, `GetClassTeachersRow`. Run `go build ./...` to confirm it compiles.

- [ ] **Step 3: Add `teacherDTO`**

Append to `apps/api/internal/api/classes_dto.go`:

```go
type teacherDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}
```

- [ ] **Step 4: Write the failing test**

Create `apps/api/internal/api/admin_teachers_test.go`. **Match the established test idiom exactly** (verified against `overview_test.go`/`classes_test.go`): `package api_test`, dot-import `. "mindimprint/api/internal/api"` (so `New`, `DepsForTest`, `SeedSchoolID` are unqualified), pool via `newAPITestPool(t)`, handler via `New(DepsForTest(pool)).Handler()`:

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestAdminListTeachers(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	createTeacher(t, pool, SeedSchoolID, "t1@demo.local")
	createTeacher(t, pool, SeedSchoolID, "t2@demo.local")
	// A teacher in another school must NOT appear.
	otherSchool := seedSecondSchool(t, pool)
	createTeacher(t, pool, otherSchool, "other@demo.local")

	req := httptest.NewRequest("GET", "/api/v1/admin/teachers", nil)
	req.AddCookie(admin)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Teachers []struct {
			ID, DisplayName, Email string
		} `json:"teachers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Teachers) != 2 {
		t.Fatalf("want 2 teachers in school, got %d", len(body.Teachers))
	}
	for _, te := range body.Teachers {
		if te.Email == "other@demo.local" {
			t.Fatal("leaked a teacher from another school")
		}
		if te.ID == "" || te.DisplayName == "" {
			t.Fatalf("incomplete teacher dto: %+v", te)
		}
	}
}

func TestAdminListTeachersRequiresAdmin(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	tid := createTeacher(t, pool, SeedSchoolID, "t@demo.local")
	teacher := signInAs(t, pool, tid)

	req := httptest.NewRequest("GET", "/api/v1/admin/teachers", nil)
	req.AddCookie(teacher)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 for a teacher, got %d", rec.Code)
	}
}
```

- [ ] **Step 5: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/api/ -run TestAdminListTeachers`
Expected: FAIL — `adminListTeachers` / the route does not exist yet (404, or compile error referencing the missing handler once Step 6 is half-done).

- [ ] **Step 6: Implement the handler**

Create `apps/api/internal/api/admin_teachers.go`:

```go
package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
)

// adminListTeachers returns every teacher in the admin's own school (for the
// admin console's teacher picker + roster). School comes from the session,
// never the request.
func (a *API) adminListTeachers(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListTeachersBySchool(r.Context(), u.SchoolID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]teacherDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, teacherDTO{ID: t.ID.String(), DisplayName: t.DisplayName, Email: t.Email})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"teachers": out})
}
```

- [ ] **Step 7: Register the route**

In `apps/api/internal/api/api.go`, in the admin-only group (after the `GET /api/v1/admin/overview` line), add:

```go
	mux.Handle("GET /api/v1/admin/teachers", adminOnly(a.adminListTeachers))
```

- [ ] **Step 8: Run the test to verify it passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/api/ -run TestAdminListTeachers`
Expected: PASS (both `TestAdminListTeachers` and `TestAdminListTeachersRequiresAdmin`).

- [ ] **Step 9: Vet + commit**

Run: `cd apps/api && go vet ./...`
Then commit:

```bash
git add apps/api/internal/store/queries/org.sql apps/api/internal/store/sqlc apps/api/internal/api/classes_dto.go apps/api/internal/api/admin_teachers.go apps/api/internal/api/admin_teachers_test.go apps/api/internal/api/api.go
git commit -m "feat(p3): org teacher queries + GET /admin/teachers

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: Backend — `getClass` returns `teachers`

**Files:**
- Modify: `apps/api/internal/api/classes.go` (the `getClass` handler)
- Modify: `apps/api/internal/api/classes_test.go`

**Interfaces:**
- Consumes: `GetClassTeachers` + `teacherDTO` (Task 1).
- Produces: `GET /api/v1/classes/{id}` response gains a `teachers` array: `{class, roster, teachers:[{id,display_name,email}]}`.

- [ ] **Step 1: Write the failing test**

Append to `apps/api/internal/api/classes_test.go`:

```go
func TestGetClassIncludesTeachers(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	// A teacher creates a class → they are enrolled as its teacher.
	tid := createTeacher(t, pool, SeedSchoolID, "teach@demo.local")
	teacher := signInAs(t, pool, tid)
	classID := createClassViaAPI(t, h, teacher, "TOK 11A")

	req := httptest.NewRequest("GET", "/api/v1/classes/"+classID, nil)
	req.AddCookie(teacher)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Teachers []struct {
			ID, DisplayName, Email string
		} `json:"teachers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Teachers) != 1 || body.Teachers[0].Email != "teach@demo.local" {
		t.Fatalf("want the creating teacher in teachers[], got %+v", body.Teachers)
	}
}
```

(`classes_test.go` is `package api_test` with the dot-import `. "mindimprint/api/internal/api"` and already imports `encoding/json` / `net/http` / `net/http/httptest` / `uuid` — so `New`, `DepsForTest`, `SeedSchoolID`, `createTeacher`, `signInAs`, `createClassViaAPI` are all in scope. No import changes needed.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/api/ -run TestGetClassIncludesTeachers`
Expected: FAIL — the response has no `teachers` field, so `len(body.Teachers)` is 0.

- [ ] **Step 3: Add the teachers query to `getClass`**

In `apps/api/internal/api/classes.go`, find the `getClass` handler. After the roster loop builds `roster` and before the final `httpx.WriteJSON`, add the teachers fetch, and include `teachers` in the response map. The handler's tail becomes:

```go
	roster := make([]rosterEntryDTO, 0, len(rows))
	for _, row := range rows {
		roster = append(roster, toRosterEntryDTO(row))
	}
	tRows, err := a.d.Queries.GetClassTeachers(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	teachers := make([]teacherDTO, 0, len(tRows))
	for _, te := range tRows {
		teachers = append(teachers, teacherDTO{ID: te.ID.String(), DisplayName: te.DisplayName, Email: te.Email})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"class": toClassDTO(cls), "roster": roster, "teachers": teachers})
```

(Replace the existing final `httpx.WriteJSON(w, http.StatusOK, map[string]any{"class": toClassDTO(cls), "roster": roster})` line with the block above.)

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/api/ -run 'TestGetClass'`
Expected: PASS — `TestGetClassIncludesTeachers` passes and the existing `getClass` tests (roster) still pass.

- [ ] **Step 5: Vet + commit**

```bash
cd apps/api && go vet ./...
git add apps/api/internal/api/classes.go apps/api/internal/api/classes_test.go
git commit -m "feat(p3): getClass response includes class teachers

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: Backend — assign + remove class teacher

**Files:**
- Create: `apps/api/internal/api/class_teachers.go`
- Create: `apps/api/internal/api/class_teachers_test.go`
- Modify: `apps/api/internal/api/api.go`

**Interfaces:**
- Consumes: `AssignClassTeacher`, `DeleteClassTeacher`, `GetClassTeachers`, `GetClassByID`, `GetUserByIDInSchool`, `teacherDTO`.
- Produces: `POST /api/v1/classes/{id}/teachers` (adminOnly) → `200 {teachers:[…]}`; `DELETE /api/v1/classes/{id}/teachers/{userId}` (adminOnly) → `204`.

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/api/class_teachers_test.go`:

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

// helper: admin assigns teacherID to classID; returns the response recorder.
func assignTeacher(t *testing.T, h http.Handler, admin *http.Cookie, classID, teacherID string) *httptest.ResponseRecorder {
	body := strings.NewReader(`{"teacher_user_id":"` + teacherID + `"}`)
	req := httptest.NewRequest("POST", "/api/v1/classes/"+classID+"/teachers", body)
	req.AddCookie(admin)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAssignAndRemoveClassTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	// A class owned by teacher A (in the seed school).
	aID := createTeacher(t, pool, SeedSchoolID, "a@demo.local")
	classID := createClassViaAPI(t, h, signInAs(t, pool, aID), "TOK 12B")
	// Teacher B to be assigned.
	bID := createTeacher(t, pool, SeedSchoolID, "b@demo.local")

	rec := assignTeacher(t, h, admin, classID, bID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("assign: want 200, got %d: %s", rec.Code, rec.Body)
	}
	var got struct {
		Teachers []struct{ Email string } `json:"teachers"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got.Teachers) != 2 {
		t.Fatalf("after assign want 2 teachers, got %d", len(got.Teachers))
	}

	// Idempotent: re-assigning B is a no-op (still 2).
	rec = assignTeacher(t, h, admin, classID, bID.String())
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusOK || len(got.Teachers) != 2 {
		t.Fatalf("re-assign should be idempotent, got %d / %d teachers", rec.Code, len(got.Teachers))
	}

	// Remove B → 204, back to 1 teacher.
	req := httptest.NewRequest("DELETE", "/api/v1/classes/"+classID+"/teachers/"+bID.String(), nil)
	req.AddCookie(admin)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, req)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("remove: want 204, got %d: %s", delRec.Code, delRec.Body)
	}
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest("GET", "/api/v1/classes/"+classID, nil)
	getReq.AddCookie(admin)
	h.ServeHTTP(getRec, getReq)
	_ = json.Unmarshal(getRec.Body.Bytes(), &got)
	if len(got.Teachers) != 1 {
		t.Fatalf("after remove want 1 teacher, got %d", len(got.Teachers))
	}
}

func TestAssignTeacherRejectsBadTarget(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)
	aID := createTeacher(t, pool, SeedSchoolID, "a@demo.local")
	classID := createClassViaAPI(t, h, signInAs(t, pool, aID), "C")

	// A random (non-existent) uuid is not a teacher in the school → 400.
	rec := assignTeacher(t, h, admin, classID, "00000000-0000-0000-0000-0000000000ff")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid teacher, got %d", rec.Code)
	}
}

func TestAssignTeacherCrossSchool404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool) // school = seed (A)

	// A class in another school B.
	otherSchool := seedSecondSchool(t, pool)
	bTeacher := createTeacher(t, pool, otherSchool, "bteach@demo.local")
	classB := createClassViaAPI(t, h, signInAs(t, pool, bTeacher), "B-class")
	someTeacher := createTeacher(t, pool, otherSchool, "x@demo.local")

	rec := assignTeacher(t, h, admin, classB, someTeacher.String())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("admin of A assigning into B's class: want 404, got %d", rec.Code)
	}
}

func TestAssignTeacherRequiresAdmin(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	aID := createTeacher(t, pool, SeedSchoolID, "a@demo.local")
	teacher := signInAs(t, pool, aID)
	classID := createClassViaAPI(t, h, teacher, "C")

	rec := assignTeacher(t, h, teacher, classID, aID.String())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a teacher calling assign: want 403, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/api/ -run 'TestAssign|TestAssignAndRemove'`
Expected: FAIL — the routes/handlers do not exist (404s where 200/204 expected).

- [ ] **Step 3: Implement the handlers**

Create `apps/api/internal/api/class_teachers.go`:

```go
package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// loadAdminClass parses the {id} path value and loads the class, enforcing that
// it belongs to the admin's own school. Cross-school / missing → 404 (existence
// hiding), matching the P3.1 tenancy guards.
func (a *API) loadAdminClass(r *http.Request) (sqlc.Class, error) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
	}
	cls, err := a.d.Queries.GetClassByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
		}
		return sqlc.Class{}, err
	}
	if cls.SchoolID != u.SchoolID {
		return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
	}
	return cls, nil
}

// assignClassTeacher enrolls a teacher (of the admin's school) into the class as
// role_in_class='teacher'. Idempotent (upsert). adminOnly.
func (a *API) assignClassTeacher(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	cls, err := a.loadAdminClass(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var body struct {
		TeacherUserID string `json:"teacher_user_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	tid, err := uuid.Parse(body.TeacherUserID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "teacher_user_id 无效", nil))
		return
	}
	// The target must be a teacher in the admin's school (mirrors createClass).
	tu, err := a.d.Queries.GetUserByIDInSchool(r.Context(), sqlc.GetUserByIDInSchoolParams{ID: tid, SchoolID: u.SchoolID})
	if err != nil || tu.Role != "teacher" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "指定的教师无效", nil))
		return
	}
	if err := a.d.Queries.AssignClassTeacher(r.Context(), sqlc.AssignClassTeacherParams{UserID: tid, ClassID: cls.ID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.writeClassTeachers(w, r, cls.ID)
}

// removeClassTeacher drops a teacher enrollment from the class. adminOnly.
func (a *API) removeClassTeacher(w http.ResponseWriter, r *http.Request) {
	cls, err := a.loadAdminClass(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.d.Queries.DeleteClassTeacher(r.Context(), sqlc.DeleteClassTeacherParams{ClassID: cls.ID, UserID: userID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) writeClassTeachers(w http.ResponseWriter, r *http.Request, classID uuid.UUID) {
	rows, err := a.d.Queries.GetClassTeachers(r.Context(), classID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]teacherDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, teacherDTO{ID: t.ID.String(), DisplayName: t.DisplayName, Email: t.Email})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"teachers": out})
}
```

- [ ] **Step 4: Register the routes**

In `apps/api/internal/api/api.go`, in the admin-only group (after the `GET /api/v1/admin/teachers` line from Task 1), add:

```go
	mux.Handle("POST /api/v1/classes/{id}/teachers", adminOnly(a.assignClassTeacher))
	mux.Handle("DELETE /api/v1/classes/{id}/teachers/{userId}", adminOnly(a.removeClassTeacher))
```

- [ ] **Step 5: Run to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/api/ -run 'TestAssign|TestAssignAndRemove'`
Expected: PASS (all four tests).

- [ ] **Step 6: Full backend gate + commit**

Run: `cd apps/api && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./...`
Expected: all packages PASS.

```bash
git add apps/api/internal/api/class_teachers.go apps/api/internal/api/class_teachers_test.go apps/api/internal/api/api.go
git commit -m "feat(p3): assign/remove class teacher (adminOnly)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: Frontend — `api/admin.ts` + `classes.ts` extension + wiring

**Files:**
- Create: `apps/web/src/api/admin.ts`
- Create: `apps/web/src/api/admin.test.ts`
- Modify: `apps/web/src/api/classes.ts`
- Modify: `apps/web/src/api/index.ts`

**Interfaces:**
- Consumes: `apiFetch` from `./client`.
- Produces: `Teacher`, `Overview`, `TeacherInvite`, `ImportRow`, `ImportResult` types; `getOverview`, `listTeacherInvites`, `createTeacherInvite`, `adminImport`, `listTeachers`, `assignTeacher`, `removeTeacher`; `classes.ts` gains `Teacher` (re-used) + `ClassDetail.teachers` + `createClass({name, teacher_user_id?})`. All added to the `ApiClient` interface + `api` object.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/api/admin.test.ts`:

```ts
import { describe, it, expect, vi } from "vitest";
import {
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
} from "./admin";

const ok = (body: unknown, status = 200) =>
  vi.fn(async () => new Response(status === 204 ? null : JSON.stringify(body), { status }));
const callOf = (spy: ReturnType<typeof vi.fn>, i = 0) => spy.mock.calls[i] as unknown as [string, RequestInit];

describe("admin api", () => {
  it("getOverview returns counts + usage", async () => {
    vi.stubGlobal("fetch", ok({ counts: { student: 3, teacher: 1, class: 2, task: 5, evaluation: 1, active_student: 2 }, usage_by_tier: [{ tier: "chaperone", prompt_tokens: 10, completion_tokens: 20, cost: "0.01" }] }));
    const o = await getOverview();
    expect(o.counts.student).toBe(3);
    expect(o.usage_by_tier[0]!.cost).toBe("0.01");
  });
  it("listTeacherInvites unwraps {invites}", async () => {
    vi.stubGlobal("fetch", ok({ invites: [{ id: "i1", code: "T-AB", expires_at: "z", created_at: "z" }] }));
    expect((await listTeacherInvites())[0]!.code).toBe("T-AB");
  });
  it("createTeacherInvite POSTs body and returns {code,expires_at}", async () => {
    const spy = ok({ code: "T-NEW", expires_at: "z" });
    vi.stubGlobal("fetch", spy);
    const r = await createTeacherInvite({ email: "t@x", expires_days: 7 });
    expect(r.code).toBe("T-NEW");
    expect(callOf(spy)[1].method).toBe("POST");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ email: "t@x", expires_days: 7 }));
  });
  it("adminImport POSTs {rows} and returns code sheet", async () => {
    const spy = ok({ classes: [{ name: "A", join_code: "AB-CD" }], teacher_invites: [{ email: "t@x", code: "T-1" }] });
    vi.stubGlobal("fetch", spy);
    const r = await adminImport([{ class: "A", teacher_email: "t@x" }]);
    expect(r.classes[0]!.join_code).toBe("AB-CD");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ rows: [{ class: "A", teacher_email: "t@x" }] }));
  });
  it("listTeachers unwraps {teachers}", async () => {
    vi.stubGlobal("fetch", ok({ teachers: [{ id: "u1", display_name: "Mr A", email: "a@x" }] }));
    expect((await listTeachers())[0]!.display_name).toBe("Mr A");
  });
  it("assignTeacher POSTs teacher_user_id and returns {teachers}", async () => {
    const spy = ok({ teachers: [{ id: "u1", display_name: "Mr A", email: "a@x" }] });
    vi.stubGlobal("fetch", spy);
    const r = await assignTeacher("c1", "u1");
    expect(r.teachers).toHaveLength(1);
    expect(callOf(spy)[0]).toContain("/api/v1/classes/c1/teachers");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ teacher_user_id: "u1" }));
  });
  it("removeTeacher DELETEs the teacher path", async () => {
    const spy = ok(null, 204);
    vi.stubGlobal("fetch", spy);
    await removeTeacher("c1", "u1");
    expect(callOf(spy)[0]).toContain("/api/v1/classes/c1/teachers/u1");
    expect(callOf(spy)[1].method).toBe("DELETE");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/api/admin.test.ts`
Expected: FAIL — `Cannot find module './admin'`.

- [ ] **Step 3: Extend `classes.ts`**

In `apps/web/src/api/classes.ts`, add the `Teacher` interface (after `ClassSummary`), add `teachers` to `ClassDetail`, and widen `createClass`:

```ts
export interface Teacher {
  id: string;
  display_name: string;
  email: string;
}
```

Change `ClassDetail`:

```ts
export interface ClassDetail {
  class: ClassSummary;
  roster: RosterStudent[];
  teachers: Teacher[];
}
```

Replace `createClass`:

```ts
export async function createClass(input: { name: string; teacher_user_id?: string }): Promise<ClassSummary> {
  const body: Record<string, unknown> = { name: input.name };
  if (input.teacher_user_id) body.teacher_user_id = input.teacher_user_id;
  const r = await apiFetch<{ class: ClassSummary }>("/api/v1/classes", {
    method: "POST",
    body: JSON.stringify(body),
  });
  return r.class;
}
```

- [ ] **Step 4: Create `admin.ts`**

Create `apps/web/src/api/admin.ts`:

```ts
import { apiFetch } from "./client";
import type { Teacher } from "./classes";

export type { Teacher };

export interface Overview {
  counts: {
    student: number;
    teacher: number;
    class: number;
    task: number;
    evaluation: number;
    active_student: number;
  };
  usage_by_tier: { tier: string; prompt_tokens: number; completion_tokens: number; cost: string }[];
}

export interface TeacherInvite {
  id: string;
  code: string;
  expires_at: string;
  created_at: string;
  email?: string;
}

export interface ImportRow {
  class: string;
  teacher_email?: string;
  student_email?: string;
}

export interface ImportResult {
  classes: { name: string; join_code: string }[];
  teacher_invites: { email: string; code: string }[];
}

export async function getOverview(): Promise<Overview> {
  return apiFetch<Overview>("/api/v1/admin/overview");
}

export async function listTeacherInvites(): Promise<TeacherInvite[]> {
  const r = await apiFetch<{ invites: TeacherInvite[] }>("/api/v1/admin/teacher-invites");
  return r.invites;
}

export async function createTeacherInvite(input: { email?: string; expires_days?: number }): Promise<{ code: string; expires_at: string }> {
  return apiFetch<{ code: string; expires_at: string }>("/api/v1/admin/teacher-invites", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function adminImport(rows: ImportRow[]): Promise<ImportResult> {
  return apiFetch<ImportResult>("/api/v1/admin/import", {
    method: "POST",
    body: JSON.stringify({ rows }),
  });
}

export async function listTeachers(): Promise<Teacher[]> {
  const r = await apiFetch<{ teachers: Teacher[] }>("/api/v1/admin/teachers");
  return r.teachers;
}

export async function assignTeacher(classId: string, teacherUserId: string): Promise<{ teachers: Teacher[] }> {
  return apiFetch<{ teachers: Teacher[] }>(`/api/v1/classes/${classId}/teachers`, {
    method: "POST",
    body: JSON.stringify({ teacher_user_id: teacherUserId }),
  });
}

export async function removeTeacher(classId: string, userId: string): Promise<void> {
  await apiFetch<void>(`/api/v1/classes/${classId}/teachers/${userId}`, { method: "DELETE" });
}
```

- [ ] **Step 5: Wire into `index.ts`**

In `apps/web/src/api/index.ts`:

Add imports (after the existing `./classes` import):

```ts
import {
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
  type Overview, type TeacherInvite, type ImportRow, type ImportResult,
} from "./admin";
```

Add `Teacher` to the `./classes` import (it is exported there now) and to the `export type {…}` line, plus the admin types:

```ts
export type { TaskDetail, TurnEvent, MeUser, ClassSummary, RosterStudent, ClassDetail, Teacher, Overview, TeacherInvite, ImportRow, ImportResult };
```

Add to the `ApiClient` interface (after the class methods):

```ts
  getOverview(): Promise<Overview>;
  listTeacherInvites(): Promise<TeacherInvite[]>;
  createTeacherInvite(input: { email?: string; expires_days?: number }): Promise<{ code: string; expires_at: string }>;
  adminImport(rows: ImportRow[]): Promise<ImportResult>;
  listTeachers(): Promise<Teacher[]>;
  assignTeacher(classId: string, teacherUserId: string): Promise<{ teachers: Teacher[] }>;
  removeTeacher(classId: string, userId: string): Promise<void>;
```

Also update the `createClass` line in the `ApiClient` interface to the new signature:

```ts
  createClass(input: { name: string; teacher_user_id?: string }): Promise<ClassSummary>;
```

Add to the `api` object literal:

```ts
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
```

Make sure `Teacher` is imported from `./classes` where the interface needs it — the simplest is to import it in `index.ts` from `./classes`:

```ts
import { listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment, type ClassSummary, type RosterStudent, type ClassDetail, type Teacher } from "./classes";
```

(adjust the existing `./classes` import line to add `type Teacher`).

- [ ] **Step 6: Run the admin api test + typecheck**

Run: `cd apps/web && pnpm test -- src/api/admin.test.ts && pnpm typecheck`
Expected: admin test PASS (7); typecheck clean (the `createClass` signature change is backward compatible — existing callers pass `{name}`).

- [ ] **Step 7: Full web suite + commit**

Run: `cd apps/web && pnpm test`
Expected: all PASS (the `classes.ts` `ClassDetail.teachers` addition is type-only; existing class tests that build a `ClassDetail` fixture without `teachers` would fail typecheck only if they are `.ts`/`.tsx` typed — the P3.2 `ClassDetailView.test.tsx` fixtures construct `ClassDetail` objects; if typecheck flags a missing `teachers`, add `teachers: []` to those fixtures as part of this task and note it).

```bash
git add apps/web/src/api/admin.ts apps/web/src/api/admin.test.ts apps/web/src/api/classes.ts apps/web/src/api/index.ts
# include the test fixture fix if needed:
# git add apps/web/src/console/ClassDetailView.test.tsx
git commit -m "feat(p3): web admin api client + classes teacher types

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

> **Implementer note:** after changing `ClassDetail` to require `teachers`, run `pnpm typecheck` — if `ClassDetailView.test.tsx` (or any other) builds a `ClassDetail` literal, add `teachers: []` to those fixtures and stage that file too. Do not weaken the type to make it optional; the backend always returns the array.

---

### Task 5: Frontend — `csv.ts` parser

**Files:**
- Create: `apps/web/src/console/csv.ts`
- Create: `apps/web/src/console/csv.test.ts`

**Interfaces:**
- Produces: `interface ParsedRow { class: string; teacher_email?: string; student_email?: string }` and `parseCsv(text: string): ParsedRow[]` (throws `Error` on empty file or missing `class` column).

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/console/csv.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { parseCsv } from "./csv";

describe("parseCsv", () => {
  it("maps columns by header, any order, case-insensitive", () => {
    const rows = parseCsv("Student_Email,Class,Teacher_Email\ns@x,11A,t@x\n");
    expect(rows).toEqual([{ class: "11A", teacher_email: "t@x", student_email: "s@x" }]);
  });
  it("omits blank optional cells", () => {
    const rows = parseCsv("class,teacher_email,student_email\n11A,,\n");
    expect(rows).toEqual([{ class: "11A" }]);
  });
  it("supports quoted fields containing commas", () => {
    const rows = parseCsv('class,student_email\n"11A, TOK",s@x\n');
    expect(rows[0]!.class).toBe("11A, TOK");
  });
  it("skips blank lines", () => {
    const rows = parseCsv("class\n11A\n\n12B\n");
    expect(rows).toHaveLength(2);
  });
  it("throws on an empty file", () => {
    expect(() => parseCsv("   ")).toThrow();
  });
  it("throws when the class column is missing", () => {
    expect(() => parseCsv("teacher_email\nt@x\n")).toThrow();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/csv.test.ts`
Expected: FAIL — `Cannot find module './csv'`.

- [ ] **Step 3: Implement `csv.ts`**

Create `apps/web/src/console/csv.ts`:

```ts
export interface ParsedRow {
  class: string;
  teacher_email?: string;
  student_email?: string;
}

// parseCsv: the first non-blank line is a header mapping the columns `class` /
// `teacher_email` / `student_email` (case-insensitive, any order). Cells are
// trimmed; double-quoted fields may contain commas. Blank lines are skipped.
// Throws on an empty file or a missing `class` column. Limitation: no escaped
// quote ("") handling — out of scope for a roster sheet; a malformed file
// surfaces as a thrown error, never a silent mis-parse.
export function parseCsv(text: string): ParsedRow[] {
  const lines = text.split(/\r?\n/).filter((l) => l.trim() !== "");
  if (lines.length === 0) throw new Error("空文件");
  const header = splitCsvLine(lines[0]!).map((h) => h.trim().toLowerCase());
  const ci = header.indexOf("class");
  if (ci === -1) throw new Error("缺少 class 列");
  const ti = header.indexOf("teacher_email");
  const si = header.indexOf("student_email");
  const rows: ParsedRow[] = [];
  for (let i = 1; i < lines.length; i++) {
    const cells = splitCsvLine(lines[i]!);
    const row: ParsedRow = { class: (cells[ci] ?? "").trim() };
    if (ti !== -1) {
      const v = (cells[ti] ?? "").trim();
      if (v) row.teacher_email = v;
    }
    if (si !== -1) {
      const v = (cells[si] ?? "").trim();
      if (v) row.student_email = v;
    }
    rows.push(row);
  }
  return rows;
}

function splitCsvLine(line: string): string[] {
  const out: string[] = [];
  let cur = "";
  let inQuotes = false;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (ch === '"') {
      inQuotes = !inQuotes;
      continue;
    }
    if (ch === "," && !inQuotes) {
      out.push(cur);
      cur = "";
      continue;
    }
    cur += ch;
  }
  out.push(cur);
  return out;
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/csv.test.ts`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/console/csv.ts apps/web/src/console/csv.test.ts
git commit -m "feat(p3): client-side CSV parser for admin import

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 6: Frontend — `OverviewView`

**Files:**
- Create: `apps/web/src/console/OverviewView.tsx`
- Create: `apps/web/src/console/OverviewView.test.tsx`

**Interfaces:**
- Consumes: `Overview`, `Pick<ApiClient, "getOverview">`.
- Produces: `function OverviewView({ client }: { client: Pick<ApiClient, "getOverview"> }): JSX.Element`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/console/OverviewView.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { OverviewView } from "./OverviewView";
import type { Overview } from "../api";

const overview = (over: Partial<Overview> = {}): Overview => ({
  counts: { student: 120, teacher: 8, class: 12, task: 340, evaluation: 95, active_student: 77 },
  usage_by_tier: [{ tier: "chaperone", prompt_tokens: 1000, completion_tokens: 2000, cost: "1.23" }],
  ...over,
});

describe("OverviewView", () => {
  it("renders the six counts and the usage row", async () => {
    const client = { getOverview: vi.fn(async () => overview()) };
    render(<OverviewView client={client} />);
    expect(await screen.findByText("概览")).toBeInTheDocument();
    expect(screen.getByText("120")).toBeInTheDocument();
    expect(screen.getByText("活跃学生")).toBeInTheDocument();
    expect(screen.getByText("chaperone")).toBeInTheDocument();
    expect(screen.getByText("1.23")).toBeInTheDocument();
  });

  it("shows the empty-usage note when usage_by_tier is empty", async () => {
    const client = { getOverview: vi.fn(async () => overview({ usage_by_tier: [] })) };
    render(<OverviewView client={client} />);
    expect(await screen.findByText("暂无用量。")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/OverviewView.test.tsx`
Expected: FAIL — `Cannot find module './OverviewView'`.

- [ ] **Step 3: Implement `OverviewView`**

Create `apps/web/src/console/OverviewView.tsx`:

```tsx
import { useEffect, useState } from "react";
import type { ApiClient, Overview } from "../api";
import { ApiError } from "../api";

type Client = Pick<ApiClient, "getOverview">;

const STATS: { key: keyof Overview["counts"]; label: string }[] = [
  { key: "student", label: "学生" },
  { key: "teacher", label: "教师" },
  { key: "class", label: "班级" },
  { key: "task", label: "任务" },
  { key: "evaluation", label: "评估" },
  { key: "active_student", label: "活跃学生" },
];

const TH: React.CSSProperties = { textAlign: "left", fontSize: 12, fontWeight: 700, color: "#8A92A3", padding: "10px 12px", borderBottom: "1px solid #EAECF2" };
const TD: React.CSSProperties = { fontSize: 13.5, color: "#1C2333", padding: "12px", borderBottom: "1px solid #F2F3F7" };

export function OverviewView({ client }: { client: Client }) {
  const [data, setData] = useState<Overview | null>(null);
  const [error, setError] = useState<string | null>(null);

  function load() {
    setError(null);
    client.getOverview().then(setData).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client]);

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 980, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>概览</div>
        {error && (
          <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
        )}
        {data && (
          <>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 16, marginTop: 24 }}>
              {STATS.map(({ key, label }) => (
                <div key={key} style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "20px 22px", boxShadow: "0 1px 3px rgba(20,30,60,.04)" }}>
                  <div style={{ fontSize: 13, color: "#8A92A3", fontWeight: 600 }}>{label}</div>
                  <div style={{ fontSize: 28, fontWeight: 800, color: "#1C2333", marginTop: 6 }}>{data.counts[key]}</div>
                </div>
              ))}
            </div>
            <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333", margin: "34px 0 14px" }}>用量（按档位）</div>
            {data.usage_by_tier.length === 0 ? (
              <div style={{ color: "#8A92A3", fontSize: 14 }}>暂无用量。</div>
            ) : (
              <table style={{ width: "100%", borderCollapse: "collapse", background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
                <thead>
                  <tr>
                    <th style={TH}>档位</th>
                    <th style={TH}>输入 token</th>
                    <th style={TH}>输出 token</th>
                    <th style={TH}>成本</th>
                  </tr>
                </thead>
                <tbody>
                  {data.usage_by_tier.map((u) => (
                    <tr key={u.tier}>
                      <td style={{ ...TD, fontWeight: 600 }}>{u.tier}</td>
                      <td style={TD}>{u.prompt_tokens}</td>
                      <td style={TD}>{u.completion_tokens}</td>
                      <td style={TD}>{u.cost}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/OverviewView.test.tsx`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/console/OverviewView.tsx apps/web/src/console/OverviewView.test.tsx
git commit -m "feat(p3): OverviewView — school counts + usage-by-tier

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 7: Frontend — `TeachersView`

**Files:**
- Create: `apps/web/src/console/TeachersView.tsx`
- Create: `apps/web/src/console/TeachersView.test.tsx`

**Interfaces:**
- Consumes: `Teacher`, `TeacherInvite`, `Pick<ApiClient, "listTeachers" | "listTeacherInvites" | "createTeacherInvite">`.
- Produces: `function TeachersView({ client }): JSX.Element`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/console/TeachersView.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeachersView } from "./TeachersView";

function makeClient() {
  return {
    listTeachers: vi.fn(async () => [{ id: "u1", display_name: "Ms Chen", email: "chen@x" }]),
    listTeacherInvites: vi.fn(async () => [{ id: "i1", code: "T-ABCD", expires_at: "2026-07-01T00:00:00Z", created_at: "2026-06-20T00:00:00Z", email: "pending@x" }]),
    createTeacherInvite: vi.fn(async () => ({ code: "T-NEW9", expires_at: "2026-07-15T00:00:00Z" })),
  };
}

describe("TeachersView", () => {
  it("lists the school's teachers and pending invites", async () => {
    render(<TeachersView client={makeClient()} />);
    expect(await screen.findByText("Ms Chen")).toBeInTheDocument();
    expect(screen.getByText(/T-ABCD/)).toBeInTheDocument();
    expect(screen.getByText(/pending@x/)).toBeInTheDocument();
  });

  it("mints an invite and surfaces the new code", async () => {
    const client = makeClient();
    render(<TeachersView client={client} />);
    await screen.findByText("Ms Chen");
    await userEvent.click(screen.getByText("生成邀请码"));
    await waitFor(() => expect(client.createTeacherInvite).toHaveBeenCalled());
    expect(await screen.findByText(/T-NEW9/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/TeachersView.test.tsx`
Expected: FAIL — `Cannot find module './TeachersView'`.

- [ ] **Step 3: Implement `TeachersView`**

Create `apps/web/src/console/TeachersView.tsx`:

```tsx
import { useEffect, useState } from "react";
import type { ApiClient, Teacher, TeacherInvite } from "../api";
import { ApiError } from "../api";
import { shortDate } from "./time";

type Client = Pick<ApiClient, "listTeachers" | "listTeacherInvites" | "createTeacherInvite">;

const card: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "18px 20px", boxShadow: "0 1px 3px rgba(20,30,60,.04)" };
const sectionTitle: React.CSSProperties = { fontSize: 15, fontWeight: 700, color: "#1C2333", margin: "30px 0 12px" };

export function TeachersView({ client }: { client: Client }) {
  const [teachers, setTeachers] = useState<Teacher[] | null>(null);
  const [invites, setInvites] = useState<TeacherInvite[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [email, setEmail] = useState("");
  const [days, setDays] = useState("");
  const [busy, setBusy] = useState(false);
  const [minted, setMinted] = useState<{ code: string; expires_at: string } | null>(null);

  function load() {
    setError(null);
    client.listTeachers().then(setTeachers).catch((e) => setError(e instanceof ApiError ? e.message : "加载失败"));
    client.listTeacherInvites().then(setInvites).catch((e) => setError(e instanceof ApiError ? e.message : "加载失败"));
  }
  useEffect(load, [client]);

  async function mint() {
    setBusy(true);
    setError(null);
    try {
      const input: { email?: string; expires_days?: number } = {};
      const e = email.trim();
      if (e) input.email = e;
      const d = parseInt(days, 10);
      if (!Number.isNaN(d) && d > 0) input.expires_days = d;
      const r = await client.createTeacherInvite(input);
      setMinted(r);
      setEmail("");
      setDays("");
      client.listTeacherInvites().then(setInvites).catch(() => {});
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "生成失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 880, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>教师</div>
        {error && (
          <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
        )}

        <div style={sectionTitle}>生成教师邀请码</div>
        <div style={{ ...card, display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
          <input
            placeholder="绑定邮箱（可选）"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            style={{ flex: 1, minWidth: 200, border: "1px solid #E1E4ED", borderRadius: 11, padding: "11px 13px", fontSize: 14, color: "#1C2333", outline: "none", boxSizing: "border-box" }}
          />
          <input
            placeholder="有效天数（默认 14）"
            value={days}
            onChange={(e) => setDays(e.target.value)}
            style={{ width: 160, border: "1px solid #E1E4ED", borderRadius: 11, padding: "11px 13px", fontSize: 14, color: "#1C2333", outline: "none", boxSizing: "border-box" }}
          />
          <button onClick={() => void mint()} disabled={busy} style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "11px 18px", borderRadius: 11, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>生成邀请码</button>
        </div>
        {minted && (
          <div style={{ marginTop: 12, background: "#EDEFF9", border: "1px solid #D7DCF2", borderRadius: 12, padding: "12px 16px", fontSize: 13.5, color: "#2A3B7A", fontWeight: 600 }}>
            新邀请码 {minted.code} · 有效期至 {shortDate(minted.expires_at)}（发给教师注册）
          </div>
        )}

        <div style={sectionTitle}>本校教师</div>
        {teachers && teachers.length === 0 && <div style={{ color: "#8A92A3", fontSize: 14 }}>暂无教师。生成邀请码邀请第一位。</div>}
        {teachers && teachers.length > 0 && (
          <div style={{ ...card, padding: 0 }}>
            {teachers.map((t, i) => (
              <div key={t.id} style={{ display: "flex", justifyContent: "space-between", padding: "14px 18px", borderTop: i === 0 ? "none" : "1px solid #F2F3F7" }}>
                <span style={{ fontSize: 14, fontWeight: 600, color: "#1C2333" }}>{t.display_name}</span>
                <span style={{ fontSize: 13, color: "#8A92A3" }}>{t.email}</span>
              </div>
            ))}
          </div>
        )}

        <div style={sectionTitle}>待使用的邀请码</div>
        {invites && invites.length === 0 && <div style={{ color: "#8A92A3", fontSize: 14 }}>暂无待使用的邀请码。</div>}
        {invites && invites.length > 0 && (
          <div style={{ ...card, padding: 0 }}>
            {invites.map((inv, i) => (
              <div key={inv.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "14px 18px", borderTop: i === 0 ? "none" : "1px solid #F2F3F7" }}>
                <span style={{ fontSize: 14, fontWeight: 700, color: "#2A3B7A" }}>{inv.code}</span>
                <span style={{ fontSize: 13, color: "#8A92A3" }}>{inv.email ? `${inv.email} · ` : ""}有效期至 {shortDate(inv.expires_at)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/TeachersView.test.tsx`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/console/TeachersView.tsx apps/web/src/console/TeachersView.test.tsx
git commit -m "feat(p3): TeachersView — roster, invites, mint

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 8: Frontend — `ImportView`

**Files:**
- Create: `apps/web/src/console/ImportView.tsx`
- Create: `apps/web/src/console/ImportView.test.tsx`

**Interfaces:**
- Consumes: `parseCsv` (Task 5), `ImportRow`, `ImportResult`, `Pick<ApiClient, "adminImport">`, `ApiError`.
- Produces: `function ImportView({ client }): JSX.Element`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/console/ImportView.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ImportView } from "./ImportView";

function file(content: string, name = "roster.csv") {
  return new File([content], name, { type: "text/csv" });
}

describe("ImportView", () => {
  it("parses a chosen file into a preview, then imports and shows the code sheet", async () => {
    const client = {
      adminImport: vi.fn(async () => ({
        classes: [{ name: "11A", join_code: "AB-CD" }],
        teacher_invites: [{ email: "t@x", code: "T-9" }],
      })),
    };
    render(<ImportView client={client} />);
    const input = screen.getByTestId("csv-input") as HTMLInputElement;
    await userEvent.upload(input, file("class,teacher_email,student_email\n11A,t@x,s@x\n"));

    // preview shows the parsed row
    expect(await screen.findByText("11A")).toBeInTheDocument();
    expect(screen.getByText("t@x")).toBeInTheDocument();

    await userEvent.click(screen.getByText("导入"));
    await waitFor(() => expect(client.adminImport).toHaveBeenCalledWith([{ class: "11A", teacher_email: "t@x", student_email: "s@x" }]));

    // code sheet
    expect(await screen.findByText("AB-CD")).toBeInTheDocument();
    expect(screen.getByText("T-9")).toBeInTheDocument();
  });

  it("shows a parse error for a file with no class column", async () => {
    const client = { adminImport: vi.fn() };
    render(<ImportView client={client} />);
    await userEvent.upload(screen.getByTestId("csv-input"), file("teacher_email\nt@x\n"));
    expect(await screen.findByText(/无法解析文件/)).toBeInTheDocument();
    expect(client.adminImport).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/ImportView.test.tsx`
Expected: FAIL — `Cannot find module './ImportView'`.

- [ ] **Step 3: Implement `ImportView`**

Create `apps/web/src/console/ImportView.tsx`:

```tsx
import { useState } from "react";
import type { ApiClient, ImportRow, ImportResult } from "../api";
import { ApiError } from "../api";
import { parseCsv } from "./csv";

type Client = Pick<ApiClient, "adminImport">;

const TH: React.CSSProperties = { textAlign: "left", fontSize: 12, fontWeight: 700, color: "#8A92A3", padding: "10px 12px", borderBottom: "1px solid #EAECF2" };
const TD: React.CSSProperties = { fontSize: 13.5, color: "#1C2333", padding: "10px 12px", borderBottom: "1px solid #F2F3F7" };

export function ImportView({ client }: { client: Client }) {
  const [rows, setRows] = useState<ImportRow[] | null>(null);
  const [parseError, setParseError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [badRow, setBadRow] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);

  async function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    setRows(null); setParseError(null); setError(null); setBadRow(null); setResult(null);
    if (!f) return;
    try {
      const text = await f.text();
      setRows(parseCsv(text));
    } catch {
      setParseError("无法解析文件，请检查是否包含 class 列。");
    }
  }

  async function doImport() {
    if (!rows) return;
    setBusy(true); setError(null); setBadRow(null);
    try {
      const r = await client.adminImport(rows);
      setResult(r);
      setRows(null);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
        const row = (err as ApiError & { details?: { row?: number } }).details?.row;
        if (typeof row === "number") setBadRow(row);
      } else {
        setError("导入失败");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 880, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>批量导入</div>
        <div style={{ fontSize: 14, color: "#6B7384", marginTop: 8, lineHeight: 1.6 }}>
          上传一个 CSV（列：class · teacher_email · student_email）。系统会创建班级与邀请码，所有人凭码自助注册。
        </div>
        <div style={{ marginTop: 18 }}>
          <input data-testid="csv-input" type="file" accept=".csv,text/csv" onChange={(e) => void onFile(e)} />
        </div>

        {parseError && <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{parseError}</div>}
        {error && <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{error}</div>}

        {rows && (
          <>
            <table style={{ width: "100%", borderCollapse: "collapse", marginTop: 20, background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
              <thead><tr><th style={TH}>班级</th><th style={TH}>教师邮箱</th><th style={TH}>学生邮箱</th></tr></thead>
              <tbody>
                {rows.map((r, i) => (
                  <tr key={i} style={badRow === i ? { background: "#FBECEC" } : undefined}>
                    <td style={{ ...TD, fontWeight: 600, color: r.class.trim() ? "#1C2333" : "#C76B6B" }}>{r.class.trim() || "（缺少班级名）"}</td>
                    <td style={{ ...TD, color: "#6B7384" }}>{r.teacher_email ?? ""}</td>
                    <td style={{ ...TD, color: "#6B7384" }}>{r.student_email ?? ""}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <button onClick={() => void doImport()} disabled={busy} style={{ marginTop: 16, background: "#2A3B7A", color: "#fff", border: "none", padding: "11px 20px", borderRadius: 12, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>导入</button>
          </>
        )}

        {result && (
          <>
            <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333", margin: "30px 0 12px" }}>班级与邀请码</div>
            <table style={{ width: "100%", borderCollapse: "collapse", background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
              <thead><tr><th style={TH}>班级</th><th style={TH}>邀请码</th></tr></thead>
              <tbody>
                {result.classes.map((c) => (
                  <tr key={c.name}><td style={{ ...TD, fontWeight: 600 }}>{c.name}</td><td style={{ ...TD, fontWeight: 700, color: "#2A3B7A" }}>{c.join_code}</td></tr>
                ))}
              </tbody>
            </table>
            {result.teacher_invites.length > 0 && (
              <>
                <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333", margin: "24px 0 12px" }}>教师邀请码</div>
                <table style={{ width: "100%", borderCollapse: "collapse", background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
                  <thead><tr><th style={TH}>邮箱</th><th style={TH}>邀请码</th></tr></thead>
                  <tbody>
                    {result.teacher_invites.map((t) => (
                      <tr key={t.code}><td style={{ ...TD, color: "#6B7384" }}>{t.email}</td><td style={{ ...TD, fontWeight: 700, color: "#2A3B7A" }}>{t.code}</td></tr>
                    ))}
                  </tbody>
                </table>
              </>
            )}
          </>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/ImportView.test.tsx`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/console/ImportView.tsx apps/web/src/console/ImportView.test.tsx
git commit -m "feat(p3): ImportView — CSV parse→preview→import→code sheet

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

> **Implementer note:** `ApiError` (in `apps/web/src/api/client.ts`) currently carries `code`, `message`, `status` but may NOT carry `details`. The `badRow` highlight reads `(err as ApiError & { details?: { row?: number } }).details?.row` defensively — if `details` is absent it simply never highlights, which is fine. Do NOT add a hard dependency on `details`; if you find `ApiError` already exposes the envelope's `details`, use it directly instead of the cast.

---

### Task 9: Frontend — role-aware `ConsoleRail`

**Files:**
- Modify: `apps/web/src/console/ConsoleRail.tsx`
- Modify: `apps/web/src/console/ConsoleRail.test.tsx`

**Interfaces:**
- Produces: `ConsoleTab = "overview" | "classes" | "teachers" | "import" | "settings"`; `ConsoleRail({ role, tab, onTab }: { role: string; tab: ConsoleTab; onTab: (t: ConsoleTab) => void })`. Admin sees all five tabs; non-admin (teacher) sees `["classes","settings"]`.

- [ ] **Step 1: Update the test**

Replace the body of `apps/web/src/console/ConsoleRail.test.tsx` with:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConsoleRail } from "./ConsoleRail";

describe("ConsoleRail", () => {
  it("admin sees all five tabs", () => {
    render(<ConsoleRail role="admin" tab="overview" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(5);
    ["概览", "班级", "教师", "导入", "设置"].forEach((t) => expect(screen.getByText(t)).toBeInTheDocument());
    expect(screen.getByText("概览").closest('[role="tab"]')).toHaveAttribute("aria-selected", "true");
  });

  it("teacher sees only 班级 and 设置", () => {
    render(<ConsoleRail role="teacher" tab="classes" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(2);
    expect(screen.getByText("班级")).toBeInTheDocument();
    expect(screen.queryByText("概览")).not.toBeInTheDocument();
    expect(screen.queryByText("导入")).not.toBeInTheDocument();
  });

  it("calls onTab when a tab is clicked", async () => {
    const onTab = vi.fn();
    render(<ConsoleRail role="admin" tab="overview" onTab={onTab} />);
    await userEvent.click(screen.getByText("导入"));
    expect(onTab).toHaveBeenCalledWith("import");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/ConsoleRail.test.tsx`
Expected: FAIL — `ConsoleRail` doesn't accept `role` and renders only two hardcoded tabs.

- [ ] **Step 3: Make the rail role-aware**

Edit `apps/web/src/console/ConsoleRail.tsx`. Widen the `ConsoleTab` type and the `NavItem` key; add the three new nav items; gate by role. Replace the `export type ConsoleTab` line and the `NAV_ITEMS` array with a full five-item list, and rewrite the component signature.

Change the type (line 4):

```tsx
export type ConsoleTab = "overview" | "classes" | "teachers" | "import" | "settings";
```

Replace the `NAV_ITEMS` array (the existing two-item list) with all five items — keep the existing 班级 and 设置 entries verbatim and add 概览 / 教师 / 导入 with these icons:

```tsx
const NAV_ITEMS: NavItem[] = [
  {
    key: "overview",
    label: "概览",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="3" width="7" height="9" rx="1.5" />
        <rect x="14" y="3" width="7" height="5" rx="1.5" />
        <rect x="14" y="12" width="7" height="9" rx="1.5" />
        <rect x="3" y="16" width="7" height="5" rx="1.5" />
      </svg>
    ),
  },
  // ── existing 班级 entry (verbatim) ──
  {
    key: "classes",
    label: "班级",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2" />
        <circle cx="9" cy="7" r="4" />
        <path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75" />
      </svg>
    ),
  },
  {
    key: "teachers",
    label: "教师",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M16 21v-2a4 4 0 00-4-4H6a4 4 0 00-4 4v2" />
        <circle cx="9" cy="7" r="4" />
        <path d="M22 21v-2a4 4 0 00-3-3.87" />
      </svg>
    ),
  },
  {
    key: "import",
    label: "导入",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4" />
        <path d="M7 10l5 5 5-5M12 15V3" />
      </svg>
    ),
  },
  // ── existing 设置 entry (verbatim) ──
  {
    key: "settings",
    label: "设置",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" />
      </svg>
    ),
  },
];

const TEACHER_TABS: ConsoleTab[] = ["classes", "settings"];
```

Rewrite the component signature + the nav map to filter by role:

```tsx
export function ConsoleRail({ role, tab, onTab }: { role: string; tab: ConsoleTab; onTab: (t: ConsoleTab) => void }) {
  const items = role === "admin" ? NAV_ITEMS : NAV_ITEMS.filter((i) => TEACHER_TABS.includes(i.key));
  return (
    <div style={{ width: 74, flexShrink: 0, background: "#FFFFFF", borderRight: "1px solid #EAECF2", display: "flex", flexDirection: "column", alignItems: "center", padding: "16px 0", gap: 4 }}>
      {/* logo SVG — unchanged */}
      <svg viewBox="0 0 48 48" width="38" height="38" style={{ display: "block", marginBottom: 16 }}>
        <rect x="5" y="6" width="38" height="36" rx="13" fill="#2A3B7A" />
        <rect x="5" y="6" width="38" height="17" rx="13" fill="#ffffff" opacity="0.10" />
        <ellipse cx="18.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
        <ellipse cx="29.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
        <circle cx="19.3" cy="25" r="1.5" fill="#1C2333" />
        <circle cx="30.3" cy="25" r="1.5" fill="#1C2333" />
        <path d="M19 31.5 Q24 35 29 31.5" stroke="#fff" strokeWidth="2.2" fill="none" strokeLinecap="round" />
        <circle cx="39" cy="9" r="4.5" fill="#E8A33D" />
      </svg>

      {items.map(({ key, label, icon }) => {
        const isActive = tab === key;
        return (
          <div
            key={key}
            role="tab"
            aria-selected={isActive}
            onClick={() => onTab(key)}
            style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 5, cursor: "pointer", padding: "4px 0" }}
          >
            <div style={isActive ? ACTIVE_BOX : INACTIVE_BOX}>{icon(isActive ? ACTIVE_ICON_STROKE : INACTIVE_ICON_STROKE)}</div>
            <span style={isActive ? ACTIVE_LABEL : INACTIVE_LABEL}>{label}</span>
          </div>
        );
      })}
    </div>
  );
}
```

(The `BOX_BASE`/`ACTIVE_BOX`/… style constants and the `NavItem` interface stay as they are; only `NavItem.key` is now `ConsoleTab` which already widened.)

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/ConsoleRail.test.tsx`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/console/ConsoleRail.tsx apps/web/src/console/ConsoleRail.test.tsx
git commit -m "feat(p3): role-aware ConsoleRail (admin gets 概览/教师/导入)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 10: Frontend — `ConsoleShell` admin routing + landing

**Files:**
- Modify: `apps/web/src/console/ConsoleShell.tsx`
- Modify: `apps/web/src/console/ConsoleShell.test.tsx`
- Modify: `apps/web/src/shell/AppShell.tsx`
- Modify: `apps/web/src/shell/AppShell.test.tsx`

**Interfaces:**
- Consumes: `OverviewView`, `TeachersView`, `ImportView`, the role-aware `ConsoleRail`, the widened `ConsoleClient`.
- Produces: `ConsoleClient` widened to include `getOverview | listTeacherInvites | createTeacherInvite | adminImport | listTeachers | assignTeacher | removeTeacher`; `ConsoleShell` lands admin on `overview`, routes the three admin tabs, passes `role` to `ConsoleRail` and `ClassDetailView`. `ShellClient` (AppShell) widened to match.

- [ ] **Step 1: Update the ConsoleShell test**

In `apps/web/src/console/ConsoleShell.test.tsx`, the fake `client()` factory must gain the admin methods, and the existing tests need their fixtures' `ClassDetail` to include `teachers: []`. Update the `client()` factory and add an admin-landing test. Replace the `detail` fixture + `client()` factory with:

```tsx
const detail: ClassDetail = { class: summary, roster: [], teachers: [] };

function client() {
  return {
    listClasses: vi.fn(async () => [summary]),
    createClass: vi.fn(),
    getClass: vi.fn(async () => detail),
    renameClass: vi.fn(),
    regenerateJoinCode: vi.fn(),
    removeEnrollment: vi.fn(),
    getOverview: vi.fn(async () => ({ counts: { student: 0, teacher: 0, class: 0, task: 0, evaluation: 0, active_student: 0 }, usage_by_tier: [] })),
    listTeacherInvites: vi.fn(async () => []),
    createTeacherInvite: vi.fn(),
    adminImport: vi.fn(),
    listTeachers: vi.fn(async () => []),
    assignTeacher: vi.fn(),
    removeTeacher: vi.fn(),
  };
}
```

The existing teacher-based tests (`TEACHER` user → lands on 我的班级) stay valid. Add an admin-landing test:

```tsx
it("an admin lands on the 概览 overview", async () => {
  const session = createSession({ storage: mem() });
  session.setUser({ ...TEACHER, role: "admin" });
  render(<ConsoleShell session={session} client={client()} onLogout={vi.fn()} />);
  expect(await screen.findByText("概览")).toBeInTheDocument();
});
```

(If the P3.2 "no-role default" test exists and asserts 全校班级, update it: with role defaulting to admin the shell now lands on 概览, so change that test to assert the create button is absent **after** navigating to the 班级 tab — click `screen.getByText("班级")` then assert `screen.queryByText("+ 新建班级")`. The intent — non-teacher gets no teacher-only create — is preserved; admins get a *different* create flow added in Task 11, so for now assert the plain `+ 新建班级` teacher button is absent.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/ConsoleShell.test.tsx`
Expected: FAIL — `ConsoleShell` doesn't render `概览` / doesn't route admin tabs.

- [ ] **Step 3: Rewrite `ConsoleShell`**

Replace `apps/web/src/console/ConsoleShell.tsx` with:

```tsx
import { useState } from "react";
import type { ApiClient } from "../api";
import type { SessionStore } from "../shell/session";
import { SettingsView } from "../shell/settings/SettingsView";
import { ConsoleRail, type ConsoleTab } from "./ConsoleRail";
import { ClassesView } from "./ClassesView";
import { ClassDetailView } from "./ClassDetailView";
import { OverviewView } from "./OverviewView";
import { TeachersView } from "./TeachersView";
import { ImportView } from "./ImportView";

export type ConsoleClient = Pick<
  ApiClient,
  | "listClasses" | "createClass" | "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment"
  | "getOverview" | "listTeacherInvites" | "createTeacherInvite" | "adminImport"
  | "listTeachers" | "assignTeacher" | "removeTeacher"
>;

export function ConsoleShell({
  session,
  client,
  onLogout,
}: {
  session: SessionStore;
  client: ConsoleClient;
  onLogout: () => void;
}) {
  const user = session.getUser();
  const role = user?.role ?? "admin";
  const [tab, setTab] = useState<ConsoleTab>(role === "admin" ? "overview" : "classes");
  const [openClassId, setOpenClassId] = useState<string | null>(null);

  return (
    <div style={{ display: "flex", height: "100%", width: "100%", background: "#F3F4F8", overflow: "hidden" }}>
      <ConsoleRail role={role} tab={tab} onTab={(t) => { setTab(t); if (t === "classes") setOpenClassId(null); }} />
      <div style={{ flex: 1, overflow: "hidden", position: "relative", display: "flex" }}>
        {tab === "overview" && <OverviewView client={client} />}
        {tab === "classes" && openClassId == null && (
          <ClassesView client={client} role={role} onOpenClass={setOpenClassId} />
        )}
        {tab === "classes" && openClassId != null && (
          <ClassDetailView client={client} classId={openClassId} role={role} onBack={() => setOpenClassId(null)} />
        )}
        {tab === "teachers" && <TeachersView client={client} />}
        {tab === "import" && <ImportView client={client} />}
        {tab === "settings" && <SettingsView session={session} user={user} onLogout={onLogout} />}
      </div>
    </div>
  );
}
```

(`ClassDetailView` gains a `role` prop in Task 11/12 — passing it here is forward-compatible; if Task 11 hasn't added the prop yet when this task runs, TypeScript will flag an unknown prop. To keep this task green, the `ClassDetailView` `role` prop is added in **this** task's Step 4 as a no-op optional prop, then Task 12 uses it. See Step 4.)

- [ ] **Step 4: Add a forward-compatible optional `role` prop to `ClassDetailView`**

So `ConsoleShell` compiles now and Task 12 fills in behavior, add `role` as an **optional** prop to `ClassDetailView` (no behavioral use yet). In `apps/web/src/console/ClassDetailView.tsx`, change the component's prop type and signature:

```tsx
export function ClassDetailView({
  client,
  classId,
  onBack,
  now,
  role,
}: {
  client: Client;
  classId: string;
  onBack: () => void;
  now?: number;
  role?: string;
}) {
```

(Do not use `role` yet — Task 12 adds the admin teacher section. A declared-but-unused prop is fine; do not add lint-suppressions.)

- [ ] **Step 5: Widen `ShellClient` in AppShell + fix the admin routing test**

In `apps/web/src/shell/AppShell.tsx`, widen the `ShellClient` type to include the new admin methods so `client` can flow to `ConsoleShell`:

```tsx
type ShellClient = Pick<
  ApiClient,
  | "getMe" | "signout"
  | "listClasses" | "createClass" | "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment"
  | "getOverview" | "listTeacherInvites" | "createTeacherInvite" | "adminImport"
  | "listTeachers" | "assignTeacher" | "removeTeacher"
>;
```

In `apps/web/src/shell/AppShell.test.tsx`, the admin routing test currently asserts the admin sees `全校班级`. Admin now lands on `概览`. Update that test: the admin's fake client needs `getOverview`, and the assertion becomes `概览`. Replace the admin test's client object + assertion:

```tsx
  it("routes an admin to the console (lands on 概览)", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const ADMIN = { ...ME, role: "admin" };
    const client = {
      getMe: vi.fn(async () => ADMIN),
      signout: vi.fn(),
      listClasses: vi.fn(async () => []),
      createClass: vi.fn(), getClass: vi.fn(), renameClass: vi.fn(), regenerateJoinCode: vi.fn(), removeEnrollment: vi.fn(),
      getOverview: vi.fn(async () => ({ counts: { student: 0, teacher: 0, class: 0, task: 0, evaluation: 0, active_student: 0 }, usage_by_tier: [] })),
      listTeacherInvites: vi.fn(async () => []), createTeacherInvite: vi.fn(), adminImport: vi.fn(),
      listTeachers: vi.fn(async () => []), assignTeacher: vi.fn(), removeTeacher: vi.fn(),
    };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getByText("概览")).toBeInTheDocument());
  });
```

(The teacher routing test still asserts `我的班级` and stays valid; the two boot-gate tests are untouched.)

- [ ] **Step 6: Run the full suite + typecheck**

Run: `cd apps/web && pnpm typecheck && pnpm test`
Expected: typecheck clean; ALL tests PASS (ConsoleShell admin-landing + teacher tests, AppShell role-routing, every prior console + api test).

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/console/ConsoleShell.tsx apps/web/src/console/ConsoleShell.test.tsx apps/web/src/console/ClassDetailView.tsx apps/web/src/shell/AppShell.tsx apps/web/src/shell/AppShell.test.tsx
git commit -m "feat(p3): ConsoleShell admin routing + 概览 landing

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 11: Frontend — admin create-class with teacher-picker (`ClassesView`)

**Files:**
- Modify: `apps/web/src/console/ClassesView.tsx`
- Modify: `apps/web/src/console/ClassesView.test.tsx`

**Interfaces:**
- Consumes: `listTeachers`, the widened `createClass({name, teacher_user_id?})`, `Teacher`.
- Produces: admin sees `+ 新建班级` with a required teacher `<select>` (from `listTeachers()`); submit calls `createClass({name, teacher_user_id})`. Teacher flow unchanged. No teachers → admin submit disabled with a hint.

- [ ] **Step 1: Update the test**

Append admin-path tests to `apps/web/src/console/ClassesView.test.tsx` (keep the existing teacher tests). Add an import of `userEvent`/`waitFor` if missing, and a client factory that includes `listTeachers`:

```tsx
  it("admin create flow uses a teacher picker and sends teacher_user_id", async () => {
    const created = cls({ id: "c9", name: "新建", join_code: "EF-GH" });
    const client = {
      listClasses: vi.fn(async () => []),
      createClass: vi.fn(async () => created),
      listTeachers: vi.fn(async () => [{ id: "u1", display_name: "Ms Chen", email: "chen@x" }]),
    };
    render(<ClassesView client={client} role="admin" onOpenClass={() => {}} />);
    await screen.findByText("全校班级");
    await userEvent.click(screen.getByText("+ 新建班级"));
    await userEvent.type(screen.getByPlaceholderText(/班级名称/), "新建");
    await userEvent.selectOptions(await screen.findByTestId("teacher-picker"), "u1");
    await userEvent.click(screen.getByText("创建"));
    await waitFor(() => expect(client.createClass).toHaveBeenCalledWith({ name: "新建", teacher_user_id: "u1" }));
  });

  it("admin with no teachers cannot create (submit disabled)", async () => {
    const client = {
      listClasses: vi.fn(async () => []),
      createClass: vi.fn(),
      listTeachers: vi.fn(async () => []),
    };
    render(<ClassesView client={client} role="admin" onOpenClass={() => {}} />);
    await userEvent.click(await screen.findByText("+ 新建班级"));
    expect(await screen.findByText(/请先在「教师」生成邀请码/)).toBeInTheDocument();
    expect(screen.getByText("创建")).toBeDisabled();
  });
```

The existing teacher tests build `client` without `listTeachers`. Add `listTeachers: vi.fn(async () => [])` to those existing teacher-path client objects so they satisfy the widened prop type (a declared method that the teacher flow never calls).

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/ClassesView.test.tsx`
Expected: FAIL — there is no teacher picker; admin currently shows no create button.

- [ ] **Step 3: Rewrite `ClassesView`**

Replace `apps/web/src/console/ClassesView.tsx` with the version below. It keeps the teacher flow identical, adds the admin create button + a teacher picker loaded on demand, and gates the admin submit on having a selected teacher:

```tsx
import { useEffect, useState } from "react";
import type { ApiClient, ClassSummary, Teacher } from "../api";
import { ApiError } from "../api";
import { shortDate } from "./time";

type Client = Pick<ApiClient, "listClasses" | "createClass" | "listTeachers">;

export function ClassesView({
  client,
  role,
  onOpenClass,
}: {
  client: Client;
  role: string;
  onOpenClass: (id: string) => void;
}) {
  const [classes, setClasses] = useState<ClassSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [lastCreated, setLastCreated] = useState<ClassSummary | null>(null);
  const [teachers, setTeachers] = useState<Teacher[] | null>(null);
  const [teacherId, setTeacherId] = useState("");

  const isTeacher = role === "teacher";
  const isAdmin = role === "admin";
  const canCreate = isTeacher || isAdmin;

  function load() {
    setError(null);
    client.listClasses().then(setClasses).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client]);

  function openCreate() {
    setCreating(true);
    setLastCreated(null);
    if (isAdmin && teachers == null) {
      client.listTeachers().then(setTeachers).catch(() => setTeachers([]));
    }
  }

  async function submit() {
    const trimmed = name.trim();
    if (!trimmed) return;
    if (isAdmin && !teacherId) return;
    setBusy(true);
    setError(null);
    try {
      const input = isAdmin ? { name: trimmed, teacher_user_id: teacherId } : { name: trimmed };
      const c = await client.createClass(input);
      setLastCreated(c);
      setName("");
      setTeacherId("");
      setCreating(false);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "创建失败");
    } finally {
      setBusy(false);
    }
  }

  const adminNoTeachers = isAdmin && teachers != null && teachers.length === 0;

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 980, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>
            {isTeacher ? "我的班级" : "全校班级"}
          </div>
          {canCreate && !creating && (
            <button
              onClick={openCreate}
              style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "10px 18px", borderRadius: 12, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}
            >
              + 新建班级
            </button>
          )}
        </div>

        {canCreate && creating && (
          <div style={{ marginTop: 18, background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "18px 20px", display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
            <input
              autoFocus
              placeholder="班级名称，如「11 年级 A · TOK」"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") void submit(); }}
              style={{ flex: 1, minWidth: 220, border: "1px solid #E1E4ED", borderRadius: 11, padding: "11px 13px", fontSize: 14, color: "#1C2333", outline: "none", boxSizing: "border-box" }}
            />
            {isAdmin && (
              <select
                data-testid="teacher-picker"
                value={teacherId}
                onChange={(e) => setTeacherId(e.target.value)}
                style={{ minWidth: 180, border: "1px solid #E1E4ED", borderRadius: 11, padding: "11px 13px", fontSize: 14, color: "#1C2333", outline: "none", boxSizing: "border-box", background: "#fff" }}
              >
                <option value="">选择教师…</option>
                {(teachers ?? []).map((t) => (
                  <option key={t.id} value={t.id}>{t.display_name}（{t.email}）</option>
                ))}
              </select>
            )}
            <button onClick={() => void submit()} disabled={busy || (isAdmin && !teacherId)} style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "11px 18px", borderRadius: 11, fontSize: 14, fontWeight: 700, cursor: (busy || (isAdmin && !teacherId)) ? "default" : "pointer", opacity: (isAdmin && !teacherId) ? 0.5 : 1, fontFamily: "inherit" }}>创建</button>
            <button onClick={() => { setCreating(false); setName(""); setTeacherId(""); }} style={{ background: "transparent", color: "#8A92A3", border: "none", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit" }}>取消</button>
            {adminNoTeachers && (
              <div style={{ flexBasis: "100%", color: "#C76B6B", fontSize: 13, fontWeight: 600 }}>请先在「教师」生成邀请码，邀请教师注册后再建班。</div>
            )}
          </div>
        )}

        {lastCreated && (
          <div style={{ marginTop: 14, background: "#EDEFF9", border: "1px solid #D7DCF2", borderRadius: 12, padding: "12px 16px", fontSize: 13.5, color: "#2A3B7A", fontWeight: 600 }}>
            已创建「{lastCreated.name}」· 邀请码 {lastCreated.join_code}（分享给学生加入）
          </div>
        )}

        {error && (
          <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
        )}

        {classes && classes.length === 0 && !creating && (
          <div style={{ marginTop: 28, color: "#8A92A3", fontSize: 14.5, lineHeight: 1.7 }}>
            {isTeacher ? "还没有班级，点「+ 新建班级」创建第一个。" : "本校暂无班级。"}
          </div>
        )}

        {classes && classes.length > 0 && (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 16, marginTop: 24 }}>
            {classes.map((c) => (
              <div
                key={c.id}
                onClick={() => onOpenClass(c.id)}
                style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "18px 20px", cursor: "pointer", boxShadow: "0 1px 3px rgba(20,30,60,.04)" }}
              >
                <div style={{ fontSize: 16, fontWeight: 700, color: "#1C2333", lineHeight: 1.45 }}>{c.name}</div>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 14 }}>
                  <span style={{ fontSize: 12.5, color: "#8A92A3", fontWeight: 600 }}>邀请码 {c.join_code}</span>
                  <span style={{ fontSize: 12, color: "#9AA1B0", fontWeight: 500 }}>创建于 {shortDate(c.created_at)}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/ClassesView.test.tsx`
Expected: PASS (existing teacher tests + 2 new admin tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/console/ClassesView.tsx apps/web/src/console/ClassesView.test.tsx
git commit -m "feat(p3): admin create-class with teacher picker

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 12: Frontend — admin teacher section in `ClassDetailView` + carry-forward doc

**Files:**
- Modify: `apps/web/src/console/ClassDetailView.tsx`
- Modify: `apps/web/src/console/ClassDetailView.test.tsx`
- Modify: `docs/遗留项追踪_Carryforward.md`

**Interfaces:**
- Consumes: `detail.teachers` (from the extended `getClass`), `listTeachers`, `assignTeacher`, `removeTeacher`, the `role` prop (added optional in Task 10).
- Produces: an admin-only 教师 section above the roster — lists `detail.teachers` with a remove ×, and an assign control (picker → `assignTeacher`). Teachers (`role !== "admin"`) see no teacher section.

- [ ] **Step 1: Update the test**

The existing `makeClient`/`detail` fixtures need `teachers` + the new methods. Update the fixture factory in `apps/web/src/console/ClassDetailView.test.tsx`:

```tsx
const detail = (over: Partial<ClassDetail> = {}): ClassDetail => ({
  class: { id: "c1", name: "11 年级 A", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-20T00:00:00Z" },
  roster: [
    { id: "u1", display_name: "Phoebe", email: "p@d", last_active_at: "2026-06-26T10:00:00Z", task_count: 3, evaluation_count: 1, card_count: 7 },
    { id: "u2", display_name: "Mia", email: "m@d", last_active_at: null, task_count: 0, evaluation_count: 0, card_count: 0 },
  ],
  teachers: [{ id: "t1", display_name: "Ms Chen", email: "chen@x" }],
  ...over,
});

function makeClient(d: ClassDetail) {
  return {
    getClass: vi.fn(async () => d),
    renameClass: vi.fn(),
    regenerateJoinCode: vi.fn(),
    removeEnrollment: vi.fn(),
    listTeachers: vi.fn(async () => [{ id: "t2", display_name: "Mr Li", email: "li@x" }]),
    assignTeacher: vi.fn(async () => ({ teachers: [{ id: "t1", display_name: "Ms Chen", email: "chen@x" }, { id: "t2", display_name: "Mr Li", email: "li@x" }] })),
    removeTeacher: vi.fn(async () => undefined),
  };
}
```

The existing tests that render `<ClassDetailView client={client} classId="c1" onBack={...} now={NOW} />` keep working (no `role` → not admin → no teacher section). Add admin-specific tests:

```tsx
describe("ClassDetailView admin teacher section", () => {
  it("teacher role sees no teacher-management section", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} role="teacher" />);
    await screen.findByText("11 年级 A");
    expect(screen.queryByText("任课教师")).not.toBeInTheDocument();
  });

  it("admin sees current teachers and can assign another", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} role="admin" />);
    expect(await screen.findByText("任课教师")).toBeInTheDocument();
    expect(screen.getByText("Ms Chen")).toBeInTheDocument();
    await userEvent.selectOptions(await screen.findByTestId("assign-teacher-picker"), "t2");
    await userEvent.click(screen.getByText("添加"));
    await waitFor(() => expect(client.assignTeacher).toHaveBeenCalledWith("c1", "t2"));
    expect(await screen.findByText("Mr Li")).toBeInTheDocument();
  });

  it("admin can remove a teacher", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} role="admin" />);
    await userEvent.click(await screen.findByLabelText("移除教师 Ms Chen"));
    await waitFor(() => expect(client.removeTeacher).toHaveBeenCalledWith("c1", "t1"));
  });
});
```

(Ensure `userEvent` and `waitFor` are imported in the test file — they were added in P3.2 Task 5.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/ClassDetailView.test.tsx`
Expected: FAIL — there is no 教师 section / assign picker.

- [ ] **Step 3: Add the admin teacher section**

In `apps/web/src/console/ClassDetailView.tsx`:

Widen the `Client` type (line 6) to include the new methods:

```tsx
type Client = Pick<ApiClient, "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment" | "listTeachers" | "assignTeacher" | "removeTeacher">;
```

Add `Teacher` to the type import (line 2):

```tsx
import type { ApiClient, ClassDetail, Teacher } from "../api";
```

Add assign state + handlers (after the existing `mutationError` state and the `doRemove` handler):

```tsx
  const [teacherOptions, setTeacherOptions] = useState<Teacher[] | null>(null);
  const [assignId, setAssignId] = useState("");

  function loadTeacherOptions() {
    if (teacherOptions == null) {
      client.listTeachers().then(setTeacherOptions).catch(() => setTeacherOptions([]));
    }
  }

  async function doAssignTeacher() {
    if (!assignId) return;
    setMutationError(null);
    setBusy(true);
    try {
      const { teachers } = await client.assignTeacher(classId, assignId);
      setDetail((d) => (d ? { ...d, teachers } : d));
      setAssignId("");
    } catch (e) {
      setMutationError(e instanceof ApiError ? e.message : "添加教师失败");
    } finally {
      setBusy(false);
    }
  }

  async function doRemoveTeacher(userId: string) {
    setMutationError(null);
    setBusy(true);
    try {
      await client.removeTeacher(classId, userId);
      setDetail((d) => (d ? { ...d, teachers: d.teachers.filter((t) => t.id !== userId) } : d));
    } catch (e) {
      setMutationError(e instanceof ApiError ? e.message : "移除教师失败");
    } finally {
      setBusy(false);
    }
  }
```

Render the section — place it right after the `mutationError` banner block and before the roster (`{detail.roster.length === 0 ? …}`), gated on `role === "admin"`:

```tsx
        {role === "admin" && (
          <div style={{ marginTop: 26 }}>
            <div style={{ fontSize: 14, fontWeight: 700, color: "#1C2333", marginBottom: 10 }}>任课教师</div>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, alignItems: "center" }}>
              {detail.teachers.map((t) => (
                <span key={t.id} style={{ display: "inline-flex", alignItems: "center", gap: 8, background: "#EDEFF9", color: "#2A3B7A", fontSize: 13, fontWeight: 600, padding: "6px 10px", borderRadius: 10 }}>
                  {t.display_name}
                  <button
                    aria-label={`移除教师 ${t.display_name}`}
                    onClick={() => void doRemoveTeacher(t.id)}
                    disabled={busy}
                    style={{ background: "transparent", border: "none", color: "#2A3B7A", fontSize: 14, cursor: "pointer", fontFamily: "inherit", lineHeight: 1, padding: 0 }}
                  >
                    ✕
                  </button>
                </span>
              ))}
              {detail.teachers.length === 0 && <span style={{ color: "#8A92A3", fontSize: 13 }}>暂无任课教师</span>}
            </div>
            <div style={{ display: "flex", gap: 8, alignItems: "center", marginTop: 12 }}>
              <select
                data-testid="assign-teacher-picker"
                value={assignId}
                onClick={loadTeacherOptions}
                onFocus={loadTeacherOptions}
                onChange={(e) => setAssignId(e.target.value)}
                style={{ minWidth: 200, border: "1px solid #E1E4ED", borderRadius: 11, padding: "9px 12px", fontSize: 13.5, color: "#1C2333", outline: "none", background: "#fff" }}
              >
                <option value="">选择教师添加…</option>
                {(teacherOptions ?? []).map((t) => (
                  <option key={t.id} value={t.id}>{t.display_name}（{t.email}）</option>
                ))}
              </select>
              <button onClick={() => void doAssignTeacher()} disabled={busy || !assignId} style={chipBtn}>添加</button>
            </div>
          </div>
        )}
```

Note: the `<select>`'s `onFocus`/`onClick` lazily loads the options so the test's `selectOptions` (which focuses the select) populates it; if the testing harness doesn't fire focus reliably, the picker also renders whatever `teacherOptions` holds — call `loadTeacherOptions()` once when the admin section mounts via a `useEffect`:

```tsx
  useEffect(() => {
    if (role === "admin") loadTeacherOptions();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [role, classId]);
```

Prefer the `useEffect` approach (deterministic in tests) and keep the `onFocus` as a harmless redundancy — but do NOT add the `eslint-disable` line unless the repo's lint actually runs in the gate; this repo's gate is typecheck + vitest only, so omit the disable comment and just write the effect.

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/ClassDetailView.test.tsx`
Expected: PASS (existing roster/mutation tests + 3 new admin teacher-section tests).

- [ ] **Step 5: Append the P3.3 carry-forward section**

Add a P3.3 section to `docs/遗留项追踪_Carryforward.md` (read the file first; match its existing heading/table style). Record: (1) teacher self-service co-teacher management — assignment is admin-only; (2) per-student work-detail visibility still waits on the eval-model brainstorm; (3) real email for invites + rate-limit/CSRF (since P2); (4) CSV escaped-quote (`""`) handling not supported; class soft-archive; single-teacher enforcement not done; (5) console a11y (clickable-div cards + rail tabs lack `tablist`/keyboard) shared with `LeftRail`.

- [ ] **Step 6: Full suite + typecheck + commit**

Run: `cd apps/web && pnpm typecheck && pnpm test`
Expected: typecheck clean; ALL tests PASS.

```bash
git add apps/web/src/console/ClassDetailView.tsx apps/web/src/console/ClassDetailView.test.tsx "docs/遗留项追踪_Carryforward.md"
git commit -m "feat(p3): admin teacher section in ClassDetailView + P3.3 carry-forward

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Final verification (after all tasks)

- [ ] Backend gate: `cd apps/api && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./...` — all green.
- [ ] Frontend gate: `cd apps/web && pnpm typecheck && pnpm test` — all green.
- [ ] Web build: `cd apps/web && pnpm build` — succeeds.
- [ ] Manual smoke (API running): sign in as the seeded admin (`admin@demo.mindimprint.local` / `admin-dev-pass`) → land on 概览 (counts + usage); 教师 → mint an invite code, see it listed; 导入 → upload a small CSV → preview → import → code sheet; 班级 → create a class picking a teacher; open a class → 任课教师 section → add/remove a teacher.

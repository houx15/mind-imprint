# Lite finished writing · versions, revising, lock, 退回修改 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A finished lite writing keeps immutable submitted versions, can be edited again after 完成 until a homework's deadline locks it, can be handed back by the teacher (退回修改), and is shown on a wide page with a version list and a paragraph diff.

**Architecture:** Postgres gets `writing_version`, `writing.revising_at` and three return columns on `lite_assignment_recipient` (migration 0153, with a backfill of version 1). The lock rule and the returned/resubmitted statuses are pure functions in `internal/liteassign`; the writing branch of `loadOwnedAtom` calls the lock rule. `POST /writings/{id}/finish` writes a version in the same transaction that sets status. The lite-web finished page reads the versions and renders a pure-function diff.

**Tech Stack:** Go (`net/http`, pgx v5, sqlc v1.27.0, goose), PostgreSQL, React + Vite + TypeScript + Tailwind (lite-web), vitest (logic only), Playwright (screenshots only).

**Spec:** `docs/superpowers/specs/2026-09-15-lite-homework-grading-and-finished-writing-design.md` (Part A only). Exploration notes: `.superpowers/tmp/explore-homework-grading.md`.

## Deviations from the spec (decided while reading the code)

1. **Teacher version route.** The spec's `GET /lite/teacher/students/{uid}/writings/{atomId}/versions/{n}` does not match the existing teacher item route, which is class-scoped and is what `authTeacherStudent` reads (`{id}` = class, `{userId}` = student). The plan uses `GET /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}/versions/{n}`.
2. **Student versions list shape.** `GET /api/v1/writings/{id}/versions` returns `{"versions": [...], "locked": bool, "lockReason": "past_due" | null}` instead of a bare array. The spec requires `locked` + `lockReason` to reach the client, and the finished page needs both in the same request.
3. **Return route path parameter** is `{userId}` (house naming), not `{uid}`: `POST /api/v1/lite/teacher/assignments/{aid}/recipients/{userId}/return`.
4. **`liteassign.Status` keeps its signature.** A new `StatusWithReturn` adds `returned` / `resubmitted`. The teacher assignment detail, assignment list counts and the student inbox call it. Weekly summary, roster and parent report keep calling `Status`: they never see a returned homework as a separate state (spec §4 keeps grades and returns out of those surfaces).
5. **Archived assignments do not lock.** The lock reads `GetLiteAssignmentForAtom`, which already excludes archived assignments.
6. **Backfill word count is computed in SQL.** Go's `agent.CountWords` cannot run inside a goose SQL migration, so the migration uses a regular expression that follows the same rule. A migration test and an `agent` test pin both on the same fixtures.
7. **The finished page title is plain text.** The old panel's `EditableTitle` calls `PATCH /writings/{id}`, which the finished-write gate already refuses with 403 today. Renaming happens while revising.

## Global Constraints

- Edition: lite only. Pro behaviour must not change. `migrations/`, `queries/`, `store/sqlc/` are shared with pro: create new files, never rename or reuse a pro query name.
- Migration number: `0153` (latest on this branch is `0152_lite_parent_report_export_only.sql`).
- Table `writing_version(id uuid pk, atom_id uuid fk atom, number int, title text, body text, word_count int, submitted_at timestamptz)`, `UNIQUE(atom_id, number)`. Versions are immutable; no delete endpoint.
- Columns: `writing.revising_at timestamptz NULL`; `lite_assignment_recipient.returned_at timestamptz NULL`, `return_due_at timestamptz NULL`, `return_note text NULL` (≤500).
- Endpoints (student, owner only): `POST /api/v1/writings/{id}/revise`, `POST /api/v1/writings/{id}/revise/discard`, `GET /api/v1/writings/{id}/versions`, `GET /api/v1/writings/{id}/versions/{n}`.
- Endpoint (teacher): `POST /api/v1/lite/teacher/assignments/{aid}/recipients/{userId}/return {dueAt, note?}`.
- Error codes: 403 `writing_finished` (finished, not revising), 403 `writing_locked` (locked). Lock message: `已过截止时间，作业已锁定`.
- Lock rule: `locked = linked to a live lite_assignment recipient row AND ≥1 version AND now > effectiveDue`; `effectiveDue = return_due_at if returned_at is set, else assignment.due_at`.
- Statuses: `returned` → `已退回`; `resubmitted` → `已重新提交`; returned and past `return_due_at` with no new version → `overdue`.
- Word count: `agent.CountWords` (the same rule as the writing room's `countWords`).
- Student copy, verbatim: `已提交 v{n} · 修改完成后请再次点击「完成」提交新版本`, `已过截止时间，作业已锁定`, `放弃修改`, `修改`, `与当前版本对比`, version line `v3 · 9月15日 14:20 · 812 字`. Page chips: `已完成 / 已提交 / 已退回 / 已锁定`.
- Layout: `.mk-rp-measure` (1180px); left column about `44rem`; one column below 1024px (Tailwind `lg:`).
- UI copy follows AGENTS.md §界面文案怎么写: labels are nouns, buttons say what they do, errors are `动词+失败：{后台原话}`, no literary prose. Rule 10 applies to comments and commit messages too.
- Tests: logic tests only. Go handler tests use the testcontainers harness (`liteTeacherFixture`, `signInAs`, `assignJSON`, `getJSON`). Frontend: vitest for pure functions and normalizers only; UI is checked with Playwright screenshots at 1440px and 390px saved under `.superpowers/tmp/`.
- Go test command: `cd apps/api && CGO_ENABLED=0 go test <pkg> -run <pattern> -count=1 -timeout 1800s`. Do not run it while an e2e stack is using Docker.
- sqlc: `cd apps/api && CGO_ENABLED=0 go tool sqlc generate` (go.mod pins `github.com/sqlc-dev/sqlc v1.27.0` as a tool). If that fails to build on macOS, run `CGO_CFLAGS="-DHAVE_STRCHRNUL" go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate`. Always check `git status --short internal/store/sqlc`: a failed run can exit 0 and generate nothing.
- Commits: stage specific paths, never `git add -A`. Every commit message ends with `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

## File map

Backend (`apps/api/internal/…`):
- `store/migrations/0153_lite_writing_versions.sql` — new table, columns, backfill.
- `store/queries/writing_version.sql` — new version queries.
- `store/queries/writing_atom.sql` — `GetWritingForUpdate`, `SetWritingRevising`, `ClearWritingRevising`.
- `store/queries/lite_assignment.sql` — return columns in three reads, `SetLiteAssignmentReturned`.
- `store/migrate_0153_test.go`, `agent/wordcount_migration_fixture_test.go` — backfill tests.
- `liteassign/lock.go` (+ test), `liteassign/status.go` (+ test) — pure rules.
- `httpx/errors.go` — `ErrWritingLocked`.
- `api/writing_versions.go` — lock facts, version insert, revise/discard, the writing write gate, version read handlers.
- `api/writing_compose.go` — finish writes a version.
- `api/readings.go` — `loadOwnedAtom` calls the new gate.
- `api/writings.go` — `revisingAt` on the DTO.
- `api/lite_teacher_assignments.go`, `api/lite_student_assignments.go` — statuses and return fields.
- `api/lite_teacher_return.go` — the return endpoint.
- `api/lite_teacher_item.go` — versions in the item payload + teacher version read.
- `api/atom_report.go` — `piece` follows the latest version.
- `api/api.go`, `api/lite_teacher_routes.go` — routes.
- Tests: `api/writing_versions_test.go`, `api/lite_assignment_return_test.go`, `api/atom_report_piece_internal_test.go`.

Frontend (`apps/lite-web/src/…`):
- `api/writings.ts`, `api/assignments.ts`, `shared/deadline.ts`, `teacher/assignmentLogic.ts` — types, clients, labels.
- `writings/versionDiff.ts` (+ test) — paragraph LCS then character marks.
- `writings/finishedWriting.ts` (+ test) — chip, version line, strip text, page choice.
- `writings/FinishedWritingPage.tsx` — the wide page (replaces `FinishedWritingPanel`).
- `writings/RevisingStrip.tsx` — the strip with 放弃修改.
- `writings/WritingRoomHost.tsx` — branch finished page / room.
- `inbox/inboxLogic.ts` (+ test), `inbox/AssignmentStrip.tsx` — 已退回 on the strip.
- `teacher/ReturnDialog.tsx`, `teacher/AssignmentDetailPage.tsx` — 退回修改.
- `e2e/finished-writing-shots.spec.ts` — one-off screenshot harness, not committed.

---

### Task 1: Migration 0153, queries, sqlc, backfill test

**Files:**
- Create: `apps/api/internal/store/migrations/0153_lite_writing_versions.sql`
- Create: `apps/api/internal/store/queries/writing_version.sql`
- Modify: `apps/api/internal/store/queries/writing_atom.sql` (append after `SetWritingFinished`)
- Modify: `apps/api/internal/store/queries/lite_assignment.sql` (`ListLiteAssignmentRecipients`, `ListLiteInboxAssignments`, `GetLiteAssignmentForAtom`; append `SetLiteAssignmentReturned`)
- Modify: `apps/api/internal/api/writings.go:254-258` (the `sqlc.Writing{…}` literal in `listWritings`)
- Regenerate: `apps/api/internal/store/sqlc/*`
- Test: `apps/api/internal/store/migrate_0153_test.go`, `apps/api/internal/agent/wordcount_migration_fixture_test.go`

**Interfaces:**
- Produces (sqlc, package `mindimprint/api/internal/store/sqlc`):
  - `type WritingVersion struct { ID uuid.UUID; AtomID uuid.UUID; Number int32; Title string; Body string; WordCount int32; SubmittedAt time.Time }`
  - `Writing.RevisingAt pgtype.Timestamptz`; `LiteAssignmentRecipient.ReturnedAt, ReturnDueAt pgtype.Timestamptz; ReturnNote *string`
  - `NextWritingVersionNumber(ctx, atomID uuid.UUID) (int32, error)`
  - `CreateWritingVersion(ctx, CreateWritingVersionParams{AtomID, Number int32, Title, Body string, WordCount int32}) (WritingVersion, error)`
  - `ListWritingVersions(ctx, atomID) ([]ListWritingVersionsRow, error)` — row has `Number, Title, WordCount, SubmittedAt`, newest first
  - `GetWritingVersion(ctx, GetWritingVersionParams{AtomID, Number int32}) (WritingVersion, error)`
  - `GetLatestWritingVersion(ctx, atomID) (WritingVersion, error)`
  - `CountWritingVersions(ctx, atomID) (int32, error)`
  - `GetWritingForUpdate(ctx, atomID) (Writing, error)`; `SetWritingRevising(ctx, atomID) (Writing, error)`; `ClearWritingRevising(ctx, atomID) error`
  - `GetLiteAssignmentForAtomRow` gains `Kind string; ReturnedAt, ReturnDueAt pgtype.Timestamptz; ReturnNote *string; Resubmitted bool`
  - `ListLiteAssignmentRecipientsRow` gains `ReturnedAt, ReturnDueAt pgtype.Timestamptz; ReturnNote *string; VersionCount int32; Resubmitted bool`
  - `ListLiteInboxAssignmentsRow` gains `ReturnedAt, ReturnDueAt pgtype.Timestamptz; ReturnNote *string; Resubmitted bool`
  - `SetLiteAssignmentReturned(ctx, SetLiteAssignmentReturnedParams{AssignmentID, UserID uuid.UUID; ReturnDueAt pgtype.Timestamptz; ReturnNote *string}) (LiteAssignmentRecipient, error)`

- [ ] **Step 1: Write the failing migration test**

Create `apps/api/internal/store/migrate_0153_test.go`:

```go
package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0153WritingVersions: 0153 adds writing_version, writing.revising_at
// and the three return columns on lite_assignment_recipient, and backfills
// version 1 for every finished writing from its current draft. The expected
// word counts are agent.CountWords on the same strings; the agent package
// pins them in wordcount_migration_fixture_test.go.
func TestMigration0153WritingVersions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.DownToContext(ctx, db, "migrations", 152); err != nil {
		t.Fatalf("goose down to v152: %v", err)
	}

	seedWriting := func(status, body string, finishedAt *time.Time) string {
		t.Helper()
		var atomID string
		if err := pool.QueryRow(ctx, `INSERT INTO atom (kind, user_id) VALUES ('writing', $1) RETURNING id::text`,
			refactor2SeededStudentID).Scan(&atomID); err != nil {
			t.Fatalf("seed atom: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO writing (atom_id, title, lang, status, finished_at) VALUES ($1, '雨', 'zh', $2, $3)`,
			atomID, status, finishedAt); err != nil {
			t.Fatalf("seed writing: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO writing_draft (atom_id, body) VALUES ($1, $2)`, atomID, body); err != nil {
			t.Fatalf("seed draft: %v", err)
		}
		return atomID
	}
	finishedAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	mixed := seedWriting("finished", "我读了 NASA 的报告，数据是 2024 年的。", &finishedAt)
	english := seedWriting("finished", "It rained all day.\n\n雨停了。", &finishedAt)
	active := seedWriting("active", "还没写完。", nil)

	if err := goose.UpToContext(ctx, db, "migrations", 153); err != nil {
		t.Fatalf("apply 0153: %v", err)
	}

	type version struct {
		number    int
		title     string
		body      string
		words     int
		submitted time.Time
	}
	read := func(atomID string) []version {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT number, title, body, word_count, submitted_at FROM writing_version WHERE atom_id = $1 ORDER BY number`, atomID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []version
		for rows.Next() {
			var v version
			if err := rows.Scan(&v.number, &v.title, &v.body, &v.words, &v.submitted); err != nil {
				t.Fatal(err)
			}
			out = append(out, v)
		}
		return out
	}
	if got := read(mixed); len(got) != 1 || got[0].number != 1 || got[0].title != "雨" ||
		got[0].body != "我读了 NASA 的报告，数据是 2024 年的。" || got[0].words != 15 || !got[0].submitted.Equal(finishedAt) {
		t.Fatalf("mixed backfill = %+v", got)
	}
	if got := read(english); len(got) != 1 || got[0].words != 8 {
		t.Fatalf("english backfill = %+v", got)
	}
	if got := read(active); len(got) != 0 {
		t.Fatalf("active writing must get no version, got %+v", got)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO writing_version (atom_id, number, title, body, word_count) VALUES ($1, 1, 'x', 'x', 1)`, mixed); err == nil {
		t.Fatal("UNIQUE(atom_id, number) must refuse a second version 1")
	}
	var revising *time.Time
	if err := pool.QueryRow(ctx, `SELECT revising_at FROM writing WHERE atom_id = $1`, mixed).Scan(&revising); err != nil || revising != nil {
		t.Fatalf("revising_at = %v err=%v, want NULL", revising, err)
	}

	if err := goose.DownToContext(ctx, db, "migrations", 152); err != nil {
		t.Fatalf("goose down to v152 again: %v", err)
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.writing_version') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("writing_version after Down exists=%v err=%v", exists, err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose up to head: %v", err)
	}
}
```

The `return_note` CHECK is exercised in Task 6 against a real recipient row.

Create `apps/api/internal/agent/wordcount_migration_fixture_test.go`:

```go
package agent

import "testing"

// The 0153 migration backfills writing_version.word_count with a SQL regular
// expression. These are the strings store/migrate_0153_test.go feeds it; if
// CountWords changes, both numbers must be revisited together.
func TestCountWordsMatchesMigration0153Fixtures(t *testing.T) {
	cases := map[string]int{
		"我读了 NASA 的报告，数据是 2024 年的。": 15,
		"It rained all day.\n\n雨停了。":       8,
	}
	for s, want := range cases {
		if got := CountWords(s); got != want {
			t.Errorf("CountWords(%q) = %d, want %d", s, got, want)
		}
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd apps/api && go test ./internal/agent -run TestCountWordsMatchesMigration0153Fixtures -count=1`
Expected: PASS (this pins the current function; it is the reference, not new behaviour).

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/store -run TestMigration0153WritingVersions -count=1 -timeout 1800s`
Expected: FAIL at `apply 0153` (no migration with version 153).

- [ ] **Step 3: Write the migration**

Create `apps/api/internal/store/migrations/0153_lite_writing_versions.sql`:

```sql
-- +goose Up
-- 写作的提交版本。每次点「完成」生成一版；版本不修改、不删除。
-- word_count 与写作室是同一套数法（agent.CountWords）：汉字、假名、全角字符各算一个，
-- 其余连续的非空白字符算一个。
CREATE TABLE writing_version (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id      uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  number       integer NOT NULL CHECK (number >= 1),
  title        text NOT NULL,
  body         text NOT NULL,
  word_count   integer NOT NULL CHECK (word_count >= 0),
  submitted_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (atom_id, number)
);

-- 完成之后重新开始修改的时间。NULL = 不在修改中。
ALTER TABLE writing ADD COLUMN revising_at timestamptz;

-- 老师退回修改：退回时间、新的截止时间、说明。再次退回时覆盖这三列。
ALTER TABLE lite_assignment_recipient
  ADD COLUMN returned_at   timestamptz,
  ADD COLUMN return_due_at timestamptz,
  ADD COLUMN return_note   text CHECK (return_note IS NULL OR char_length(return_note) <= 500);

-- 已完成的写作补第一版：当前草稿，提交时间取 finished_at。
-- 正则的第一段是单个 CJK 字符（agent.isCJK 的范围），第二段是一串非空白、非 CJK 字符。
INSERT INTO writing_version (atom_id, number, title, body, word_count, submitted_at)
SELECT w.atom_id, 1, w.title, COALESCE(d.body, ''),
       (SELECT count(*) FROM regexp_matches(
          COALESCE(d.body, ''),
          '[\u2e80-\u2fdf\u3005\u3007\u3021-\u3029\u3038-\u303b\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\uff00-\uffef\U00020000-\U0003134f]|[^\s\u3000\u2e80-\u2fdf\u3005\u3007\u3021-\u3029\u3038-\u303b\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\uff00-\uffef\U00020000-\U0003134f]+',
          'g'))::int,
       COALESCE(w.finished_at, w.updated_at)
FROM writing w
LEFT JOIN writing_draft d ON d.atom_id = w.atom_id
WHERE w.status = 'finished';

-- +goose Down
DROP TABLE writing_version;
ALTER TABLE writing DROP COLUMN revising_at;
ALTER TABLE lite_assignment_recipient
  DROP COLUMN return_note,
  DROP COLUMN return_due_at,
  DROP COLUMN returned_at;
```

- [ ] **Step 4: Run the migration test**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/store -run TestMigration0153WritingVersions -count=1 -timeout 1800s`
Expected: PASS. If the word counts differ, print `SELECT regexp_matches(body, …, 'g') FROM writing_draft` for the failing row and fix the character class; do not change the expected numbers (they come from `agent.CountWords`).

- [ ] **Step 5: Write the queries**

Create `apps/api/internal/store/queries/writing_version.sql`:

```sql
-- Lite 写作的提交版本（0153）。只插入、只读，没有 UPDATE 或 DELETE。

-- name: NextWritingVersionNumber :one
-- 与 CreateWritingVersion 放在同一个事务里，并先锁住 writing 行（GetWritingForUpdate）。
SELECT (COALESCE(max(number), 0) + 1)::int FROM writing_version WHERE atom_id = $1;

-- name: CreateWritingVersion :one
INSERT INTO writing_version (atom_id, number, title, body, word_count)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListWritingVersions :many
-- 版本列表不带正文，新的在前。
SELECT id, atom_id, number, title, word_count, submitted_at
FROM writing_version
WHERE atom_id = $1
ORDER BY number DESC;

-- name: GetWritingVersion :one
SELECT * FROM writing_version WHERE atom_id = $1 AND number = $2;

-- name: GetLatestWritingVersion :one
SELECT * FROM writing_version WHERE atom_id = $1 ORDER BY number DESC LIMIT 1;

-- name: CountWritingVersions :one
SELECT count(*)::int FROM writing_version WHERE atom_id = $1;
```

Append to `apps/api/internal/store/queries/writing_atom.sql`, directly after the `SetWritingFinished` block:

```sql
-- name: GetWritingForUpdate :one
-- 完成、放弃修改时先锁住这一行，版本号才不会重复。
SELECT * FROM writing WHERE atom_id = $1 FOR UPDATE;

-- name: SetWritingRevising :one
-- 已在修改中时保留原来的时间。
UPDATE writing SET revising_at = COALESCE(revising_at, now()), updated_at = now()
WHERE atom_id = $1
RETURNING *;

-- name: ClearWritingRevising :exec
UPDATE writing SET revising_at = NULL, updated_at = now() WHERE atom_id = $1;
```

In `apps/api/internal/store/queries/lite_assignment.sql`, replace `ListLiteAssignmentRecipients` with:

```sql
-- name: ListLiteAssignmentRecipients :many
-- 一份作业的每个学生，连同她那一项的完成时间、退回信息和提交版本数。
-- resubmitted：退回之后提交过新版本（returned_at 为 NULL 时比较结果为 NULL，EXISTS 为 false）。
SELECT r.assignment_id, r.user_id, r.seen_at, r.atom_id, r.started_at,
       r.returned_at, r.return_due_at, r.return_note,
       u.display_name, u.avatar_color,
       COALESCE(rd.finished_at, w.finished_at, p.finished_at) AS finished_at,
       COALESCE((SELECT count(*) FROM writing_version v WHERE v.atom_id = r.atom_id), 0)::int AS version_count,
       EXISTS (SELECT 1 FROM writing_version v
               WHERE v.atom_id = r.atom_id AND v.submitted_at > r.returned_at)::bool AS resubmitted
FROM lite_assignment_recipient r
JOIN users u ON u.id = r.user_id
LEFT JOIN reading rd ON rd.atom_id = r.atom_id
LEFT JOIN writing w ON w.atom_id = r.atom_id
LEFT JOIN pbl_project p ON p.atom_id = r.atom_id
WHERE r.assignment_id = ANY(sqlc.arg(assignment_ids)::uuid[])
ORDER BY u.display_name;
```

Replace `ListLiteInboxAssignments` with:

```sql
-- name: ListLiteInboxAssignments :many
-- 她的作业，未读在前，其后按截止时间。
SELECT a.id, a.kind, a.title, a.instructions, a.payload, a.due_at, a.created_at,
       r.seen_at, r.atom_id, r.started_at,
       r.returned_at, r.return_due_at, r.return_note,
       c.name AS class_name,
       COALESCE(rd.finished_at, w.finished_at, p.finished_at) AS finished_at,
       EXISTS (SELECT 1 FROM writing_version v
               WHERE v.atom_id = r.atom_id AND v.submitted_at > r.returned_at)::bool AS resubmitted
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
JOIN classes c ON c.id = a.class_id
LEFT JOIN reading rd ON rd.atom_id = r.atom_id
LEFT JOIN writing w ON w.atom_id = r.atom_id
LEFT JOIN pbl_project p ON p.atom_id = r.atom_id
WHERE r.user_id = $1 AND a.archived_at IS NULL
  -- 她离开了这个班，这个班的作业就不再出现。
  AND EXISTS (SELECT 1 FROM enrollments e
              WHERE e.class_id = a.class_id AND e.user_id = r.user_id AND e.role_in_class = 'student')
ORDER BY (r.seen_at IS NULL) DESC, a.due_at ASC;
```

Replace `GetLiteAssignmentForAtom` with:

```sql
-- name: GetLiteAssignmentForAtom :one
-- 写作的锁定判断也读这一条：归档的作业不算。
SELECT a.id, a.kind, a.title, a.due_at,
       r.returned_at, r.return_due_at, r.return_note,
       EXISTS (SELECT 1 FROM writing_version v
               WHERE v.atom_id = r.atom_id AND v.submitted_at > r.returned_at)::bool AS resubmitted
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
WHERE r.atom_id = $1 AND r.user_id = $2 AND a.archived_at IS NULL;
```

Append:

```sql
-- name: SetLiteAssignmentReturned :one
-- 退回修改。再次退回时覆盖三列。
UPDATE lite_assignment_recipient
SET returned_at = now(), return_due_at = $3, return_note = $4
WHERE assignment_id = $1 AND user_id = $2
RETURNING *;
```

- [ ] **Step 6: Regenerate sqlc and check it generated**

Run: `cd apps/api && CGO_ENABLED=0 go tool sqlc generate && git status --short internal/store/sqlc`
Expected: `M internal/store/sqlc/lite_assignment.sql.go`, `M internal/store/sqlc/models.go`, `M internal/store/sqlc/writing_atom.sql.go`, `?? internal/store/sqlc/writing_version.sql.go` (plus any file whose `SELECT *` on `writing` changed). If nothing is listed, use the fallback command in Global Constraints.

Run: `cd apps/api && grep -n "type GetLiteAssignmentForAtomRow" -A10 internal/store/sqlc/lite_assignment.sql.go && grep -n "type SetLiteAssignmentReturnedParams" -A6 internal/store/sqlc/lite_assignment.sql.go`
Expected: `ReturnedAt pgtype.Timestamptz`, `ReturnDueAt pgtype.Timestamptz`, `ReturnNote *string`, `Resubmitted bool`, and params `ReturnDueAt pgtype.Timestamptz`, `ReturnNote *string`. Later tasks use exactly these types.

- [ ] **Step 7: Keep `listWritings` complete**

In `apps/api/internal/api/writings.go`, the literal inside `listWritings` becomes:

```go
		out = append(out, writingDTOOf(sqlc.Writing{
			AtomID: row.AtomID, Title: row.Title, Lang: row.Lang, Stage: row.Stage,
			TargetWords: row.TargetWords, StructureKey: row.StructureKey, SetupAt: row.SetupAt,
			Status:     row.Status,
			UpdatedAt:  row.UpdatedAt, FinishedAt: row.FinishedAt,
			RevisingAt: row.RevisingAt,
		}, row.AtomCreatedAt, row.LastActivityAt))
```

Run: `cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./internal/api/ ./internal/store/...`
Expected: no output.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/store/migrations/0153_lite_writing_versions.sql \
  apps/api/internal/store/queries/writing_version.sql \
  apps/api/internal/store/queries/writing_atom.sql \
  apps/api/internal/store/queries/lite_assignment.sql \
  apps/api/internal/store/sqlc \
  apps/api/internal/store/migrate_0153_test.go \
  apps/api/internal/agent/wordcount_migration_fixture_test.go \
  apps/api/internal/api/writings.go
git commit -m "feat(lite): writing_version table, revising_at and return columns (0153)

Backfills version 1 for finished writings from the current draft.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Lock rule and returned/resubmitted statuses (pure)

**Files:**
- Create: `apps/api/internal/liteassign/lock.go`, `apps/api/internal/liteassign/lock_test.go`
- Modify: `apps/api/internal/liteassign/status.go`, `apps/api/internal/liteassign/status_test.go`

**Interfaces:**
- Produces:
  - `type LockFacts struct { Homework bool; HasVersion bool; DueAt time.Time; ReturnedAt *time.Time; ReturnDueAt *time.Time }`
  - `const LockReasonPastDue = "past_due"`
  - `func EffectiveDue(f LockFacts) time.Time`
  - `func Locked(f LockFacts, now time.Time) bool`
  - `func LockReason(f LockFacts, now time.Time) string` (`"past_due"` or `""`)
  - `type Return struct { DueAt time.Time; Resubmitted bool }`
  - `func StatusWithReturn(started bool, finishedAt *time.Time, dueAt, now time.Time, ret *Return) string`
  - `StatusLabel("returned") == "已退回"`, `StatusLabel("resubmitted") == "已重新提交"`

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/liteassign/lock_test.go`:

```go
package liteassign

import (
	"testing"
	"time"
)

func TestLocked(t *testing.T) {
	due := time.Date(2026, 9, 20, 22, 0, 0, 0, time.UTC)
	returnDue := due.Add(72 * time.Hour)
	returnedAt := due.Add(time.Hour)
	at := func(t time.Time) *time.Time { return &t }

	cases := []struct {
		name string
		f    LockFacts
		now  time.Time
		want bool
	}{
		{"not homework, past due", LockFacts{Homework: false, HasVersion: true, DueAt: due}, due.Add(time.Hour), false},
		{"homework, no version, past due", LockFacts{Homework: true, HasVersion: false, DueAt: due}, due.Add(time.Hour), false},
		{"homework, version, before due", LockFacts{Homework: true, HasVersion: true, DueAt: due}, due.Add(-time.Hour), false},
		{"homework, version, exactly at due", LockFacts{Homework: true, HasVersion: true, DueAt: due}, due, false},
		{"homework, version, one second past due", LockFacts{Homework: true, HasVersion: true, DueAt: due}, due.Add(time.Second), true},
		{"returned, before return due", LockFacts{Homework: true, HasVersion: true, DueAt: due, ReturnedAt: at(returnedAt), ReturnDueAt: at(returnDue)}, returnDue.Add(-time.Hour), false},
		{"returned, past return due", LockFacts{Homework: true, HasVersion: true, DueAt: due, ReturnedAt: at(returnedAt), ReturnDueAt: at(returnDue)}, returnDue.Add(time.Second), true},
		{"return due without returned_at is ignored", LockFacts{Homework: true, HasVersion: true, DueAt: due, ReturnDueAt: at(returnDue)}, due.Add(time.Hour), true},
	}
	for _, c := range cases {
		if got := Locked(c.f, c.now); got != c.want {
			t.Errorf("%s: Locked = %v, want %v", c.name, got, c.want)
		}
		wantReason := ""
		if c.want {
			wantReason = LockReasonPastDue
		}
		if got := LockReason(c.f, c.now); got != wantReason {
			t.Errorf("%s: LockReason = %q, want %q", c.name, got, wantReason)
		}
	}
}

func TestEffectiveDue(t *testing.T) {
	due := time.Date(2026, 9, 20, 22, 0, 0, 0, time.UTC)
	returnDue := due.Add(48 * time.Hour)
	returnedAt := due
	if got := EffectiveDue(LockFacts{DueAt: due}); !got.Equal(due) {
		t.Fatalf("not returned: %v", got)
	}
	if got := EffectiveDue(LockFacts{DueAt: due, ReturnedAt: &returnedAt, ReturnDueAt: &returnDue}); !got.Equal(returnDue) {
		t.Fatalf("returned: %v", got)
	}
}
```

Append to `apps/api/internal/liteassign/status_test.go`:

```go
func TestStatusWithReturn(t *testing.T) {
	due := time.Date(2026, 9, 20, 22, 0, 0, 0, time.UTC)
	finished := due.Add(-time.Hour)
	returnDue := due.Add(72 * time.Hour)

	if got := StatusWithReturn(true, &finished, due, due.Add(time.Hour), nil); got != "done" {
		t.Fatalf("nil return must equal Status: %s", got)
	}
	cases := []struct {
		name string
		ret  Return
		now  time.Time
		want string
	}{
		{"returned, before return due", Return{DueAt: returnDue}, returnDue.Add(-time.Hour), "returned"},
		{"returned, exactly at return due", Return{DueAt: returnDue}, returnDue, "returned"},
		{"returned, past return due, no new version", Return{DueAt: returnDue}, returnDue.Add(time.Second), "overdue"},
		{"resubmitted before return due", Return{DueAt: returnDue, Resubmitted: true}, returnDue.Add(-time.Hour), "resubmitted"},
		{"resubmitted, viewed after return due", Return{DueAt: returnDue, Resubmitted: true}, returnDue.Add(time.Hour), "resubmitted"},
	}
	for _, c := range cases {
		ret := c.ret
		if got := StatusWithReturn(true, &finished, due, c.now, &ret); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}
```

In `TestStatusLabel`, replace the `want` map line with:

```go
	want := map[string]string{"not_started": "未开始", "in_progress": "进行中", "done": "已完成", "done_late": "逾期完成", "overdue": "已逾期", "returned": "已退回", "resubmitted": "已重新提交"}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd apps/api && go test ./internal/liteassign/ -count=1`
Expected: FAIL to compile: `undefined: LockFacts`, `undefined: StatusWithReturn`.

- [ ] **Step 3: Implement**

Create `apps/api/internal/liteassign/lock.go`:

```go
package liteassign

import "time"

// LockReasonPastDue is the only reason a submitted writing homework is locked.
const LockReasonPastDue = "past_due"

// LockFacts are the inputs of the lock rule for one writing.
type LockFacts struct {
	// Homework: the writing is linked to a recipient row of a live (not archived) assignment.
	Homework bool
	// HasVersion: at least one version was submitted.
	HasVersion bool
	// DueAt is the assignment deadline. Unused when Homework is false.
	DueAt time.Time
	// ReturnedAt and ReturnDueAt are set when the teacher returned the writing.
	ReturnedAt  *time.Time
	ReturnDueAt *time.Time
}

// EffectiveDue is the return deadline once the teacher has returned the
// writing, otherwise the assignment deadline.
func EffectiveDue(f LockFacts) time.Time {
	if f.ReturnedAt != nil && f.ReturnDueAt != nil {
		return *f.ReturnDueAt
	}
	return f.DueAt
}

// Locked reports whether a writing can no longer be edited. Only a submitted
// homework past its effective deadline is locked: a homework with no version
// can still be submitted late, and a writing that is not homework never locks.
func Locked(f LockFacts, now time.Time) bool {
	return f.Homework && f.HasVersion && now.After(EffectiveDue(f))
}

// LockReason is LockReasonPastDue when the writing is locked, "" otherwise.
func LockReason(f LockFacts, now time.Time) string {
	if Locked(f, now) {
		return LockReasonPastDue
	}
	return ""
}
```

In `apps/api/internal/liteassign/status.go`, add after `Status`:

```go
// Return is a teacher's 退回修改 on one writing recipient.
type Return struct {
	// DueAt is the new deadline the teacher set.
	DueAt time.Time
	// Resubmitted: a version was submitted after the return.
	Resubmitted bool
}

// StatusWithReturn is Status for a recipient that may have been returned.
// ret == nil (never returned) gives exactly Status. Otherwise:
//   - resubmitted: a version was submitted after the return;
//   - returned: no such version and now is not past ret.DueAt;
//   - overdue: no such version and now is past ret.DueAt.
func StatusWithReturn(started bool, finishedAt *time.Time, dueAt, now time.Time, ret *Return) string {
	if ret == nil {
		return Status(started, finishedAt, dueAt, now)
	}
	if ret.Resubmitted {
		return "resubmitted"
	}
	if now.After(ret.DueAt) {
		return "overdue"
	}
	return "returned"
}
```

Replace the `labels` map with:

```go
var labels = map[string]string{
	"not_started": "未开始",
	"in_progress": "进行中",
	"done":        "已完成",
	"done_late":   "逾期完成",
	"overdue":     "已逾期",
	"returned":    "已退回",
	"resubmitted": "已重新提交",
}
```

- [ ] **Step 4: Run the tests**

Run: `cd apps/api && go test ./internal/liteassign/ -count=1 -v -run 'TestLocked|TestEffectiveDue|TestStatus'`
Expected: PASS for `TestLocked`, `TestEffectiveDue`, `TestStatus`, `TestStatusWithReturn`, `TestStatusLabel`.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/liteassign/lock.go apps/api/internal/liteassign/lock_test.go \
  apps/api/internal/liteassign/status.go apps/api/internal/liteassign/status_test.go
git commit -m "feat(lite): lock rule and returned/resubmitted assignment statuses

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Finish writes a version

**Files:**
- Create: `apps/api/internal/api/writing_versions.go`
- Modify: `apps/api/internal/api/writing_compose.go` (`finishWritingAtom`, currently at the end of the file's finish section)
- Modify: `apps/api/internal/api/writings.go` (`writingDTO`, `writingDTOOf`)
- Modify: `apps/api/internal/httpx/errors.go` (after `ErrWritingFinished`)
- Test: `apps/api/internal/api/writing_versions_test.go`

**Interfaces:**
- Consumes: Task 1 queries; Task 2 `liteassign.LockFacts`, `liteassign.Locked`.
- Produces:
  - `httpx.ErrWritingLocked() *httpx.APIError` (403, code `writing_locked`, message `已过截止时间，作业已锁定`)
  - `writingDTO.RevisingAt *string` (JSON `revisingAt`)
  - `func writingLockFacts(ctx context.Context, q *sqlc.Queries, atomID, ownerID uuid.UUID) (liteassign.LockFacts, error)`
  - `func insertWritingVersion(ctx context.Context, q *sqlc.Queries, atomID uuid.UUID, title, body string) (sqlc.WritingVersion, error)`
  - Test helpers in `writing_versions_test.go` (package `api_test`): `newBroughtWriting(t, h, c, body) string`, `versionRows(t, pool, atomID) []versionRow`, `startWritingHomework(t, h, pool, teacher, classID, studentID) (aid, atomID string, student *http.Cookie)`

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/api/writing_versions_test.go`:

```go
package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type versionRow struct {
	Number      int
	Title       string
	Body        string
	WordCount   int
	SubmittedAt time.Time
}

// newBroughtWriting creates a writing she brought in with a finished body:
// it lands on the draft stage with writing_draft already filled.
func newBroughtWriting(t *testing.T, h http.Handler, c *http.Cookie, body string) string {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	if code := assignJSON(t, h, c, "POST", "/api/v1/writings", map[string]any{"idea": "雨", "lang": "zh", "body": body}, &created); code != http.StatusCreated {
		t.Fatalf("create writing = %d", code)
	}
	return created.ID
}

func versionRows(t *testing.T, pool *pgxpool.Pool, atomID string) []versionRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT number, title, body, word_count, submitted_at FROM writing_version WHERE atom_id = $1 ORDER BY number`, atomID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []versionRow
	for rows.Next() {
		var v versionRow
		if err := rows.Scan(&v.Number, &v.Title, &v.Body, &v.WordCount, &v.SubmittedAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

// startWritingHomework: the teacher assigns a writing to the fixture student,
// she starts it and writes a draft. Nothing is finished yet.
func startWritingHomework(t *testing.T, h http.Handler, pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID) (aid, atomID string, student *http.Cookie) {
	t.Helper()
	student = signInAs(t, pool, studentID)
	aid = createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	atomID = startAssignment(t, h, student, aid).AtomID
	if code := assignJSON(t, h, student, "PUT", "/api/v1/writings/"+atomID+"/draft", map[string]any{"body": "雨下了一整天。"}, nil); code != http.StatusOK {
		t.Fatalf("put draft = %d", code)
	}
	return aid, atomID, student
}

func TestWritingVersionFirstFinishCreatesVersionOne(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "我读了 NASA 的报告，数据是 2024 年的。")

	var first struct {
		Status     string  `json:"status"`
		FinishedAt *string `json:"finishedAt"`
		RevisingAt *string `json:"revisingAt"`
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+id+"/finish", nil, &first); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if first.Status != "finished" || first.FinishedAt == nil || first.RevisingAt != nil {
		t.Fatalf("finish response = %+v", first)
	}
	got := versionRows(t, pool, id)
	if len(got) != 1 || got[0].Number != 1 || got[0].Title != "雨" ||
		got[0].Body != "我读了 NASA 的报告，数据是 2024 年的。" || got[0].WordCount != 15 {
		t.Fatalf("versions = %+v", got)
	}

	// A second finish while not revising changes nothing: no version 2,
	// finished_at stays where it was.
	var again struct {
		FinishedAt *string `json:"finishedAt"`
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+id+"/finish", nil, &again); code != http.StatusOK {
		t.Fatalf("second finish = %d", code)
	}
	if n := len(versionRows(t, pool, id)); n != 1 {
		t.Fatalf("second finish added a version: %d rows", n)
	}
	if again.FinishedAt == nil || *again.FinishedAt != *first.FinishedAt {
		t.Fatalf("finishedAt moved: %v → %v", first.FinishedAt, again.FinishedAt)
	}
}

func TestWritingVersionFinishWithoutDraftAddsNothing(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	var created struct {
		ID string `json:"id"`
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings", map[string]any{"idea": "雨", "lang": "zh"}, &created); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+created.ID+"/finish", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("finish without draft = %d, want 400", code)
	}
	if n := len(versionRows(t, pool, created.ID)); n != 0 {
		t.Fatalf("versions = %d, want 0", n)
	}
}

// A homework she never submitted stays submittable after the deadline.
func TestWritingVersionLateFirstSubmissionIsNotLocked(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	if _, err := pool.Exec(context.Background(), `UPDATE lite_assignment SET due_at = now() - interval '1 hour' WHERE id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("late first finish = %d, want 200", code)
	}
	if n := len(versionRows(t, pool, atomID)); n != 1 {
		t.Fatalf("versions = %d, want 1", n)
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestWritingVersion -count=1 -timeout 1800s`
Expected: FAIL — `versions = []` in `TestWritingVersionFirstFinishCreatesVersionOne` (finish does not write a version yet) and the same in the late-submission test.

- [ ] **Step 3: Add the error and the DTO field**

In `apps/api/internal/httpx/errors.go`, after `ErrWritingFinished`:

```go
// ErrWritingLocked — 403 for a write to a submitted writing homework whose
// effective deadline has passed (liteassign.Locked). A separate code from
// writing_finished: the student cannot unlock it herself, only the teacher
// can, by returning the homework.
func ErrWritingLocked() *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "writing_locked", Message: "已过截止时间，作业已锁定"}
}
```

In `apps/api/internal/api/writings.go`, add to `writingDTO` after `FinishedAt`:

```go
	// RevisingAt is set while she edits a finished writing again (0153).
	// The status stays "finished" the whole time.
	RevisingAt *string `json:"revisingAt"`
```

and in `writingDTOOf`, after the `FinishedAt` block:

```go
	if wr.RevisingAt.Valid {
		s := wr.RevisingAt.Time.Format(time.RFC3339)
		out.RevisingAt = &s
	}
```

- [ ] **Step 4: Add the version helpers**

Create `apps/api/internal/api/writing_versions.go`:

```go
package api

// writing_versions.go — submitted versions of a lite writing (0153): the lock
// facts, inserting a version, editing after 完成, and reading versions.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/store/sqlc"
)

// writingLockFacts reads what liteassign.Locked needs for one writing.
// ownerID is the atom's owner: the recipient row is keyed by her user id.
func writingLockFacts(ctx context.Context, q *sqlc.Queries, atomID, ownerID uuid.UUID) (liteassign.LockFacts, error) {
	var f liteassign.LockFacts
	n, err := q.CountWritingVersions(ctx, atomID)
	if err != nil {
		return f, err
	}
	f.HasVersion = n > 0
	row, err := q.GetLiteAssignmentForAtom(ctx, sqlc.GetLiteAssignmentForAtomParams{
		AtomID: pgtype.UUID{Bytes: atomID, Valid: true}, UserID: ownerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	f.Homework = true
	f.DueAt = row.DueAt
	f.ReturnedAt = tsPtr(row.ReturnedAt)
	f.ReturnDueAt = tsPtr(row.ReturnDueAt)
	return f, nil
}

// insertWritingVersion adds the next version. The caller holds the writing
// row lock (GetWritingForUpdate) in the same transaction, so two finishes
// cannot compute the same number; UNIQUE(atom_id, number) backs that up.
func insertWritingVersion(ctx context.Context, q *sqlc.Queries, atomID uuid.UUID, title, body string) (sqlc.WritingVersion, error) {
	n, err := q.NextWritingVersionNumber(ctx, atomID)
	if err != nil {
		return sqlc.WritingVersion{}, err
	}
	return q.CreateWritingVersion(ctx, sqlc.CreateWritingVersionParams{
		AtomID: atomID, Number: n, Title: title, Body: body,
		WordCount: int32(agent.CountWords(body)),
	})
}
```

If `go build` reports `CountWritingVersions` returns `int64` rather than `int32`, compare with `n > 0` unchanged; the expression compiles for both.

- [ ] **Step 5: Rewrite the finish handler**

In `apps/api/internal/api/writing_compose.go`, replace the whole `finishWritingAtom` function and add to its doc comment the paragraph below. Add imports `time` and `mindimprint/api/internal/liteassign` if the file does not have them.

Doc comment paragraph to append above the function:

```go
// Versions (0153): the first finish sets status and finished_at (which still
// decides 按时 / 逾期) and adds version 1. A finish while revising clears
// revising_at and adds the next version; it is refused with writing_locked
// when the homework's effective deadline has passed. A finish on a finished
// writing that is not being revised is a no-op 200, as before.
```

Function:

```go
func (a *API) finishWritingAtom(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtomRow(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	draft, err := a.d.Queries.GetWritingDraft(ctx, at.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(draft.Body) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_draft", "先完成初稿，再点完成。", nil))
		return
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	wr, err := qtx.GetWritingForUpdate(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	first := wr.Status != "finished"
	revising := wr.RevisingAt.Valid
	if !first && !revising {
		httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt, at.LastActivityAt))
		return
	}
	if revising {
		facts, err := writingLockFacts(ctx, qtx, at.ID, at.UserID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if liteassign.Locked(facts, time.Now()) {
			httpx.WriteError(w, r, httpx.ErrWritingLocked())
			return
		}
	}
	if first {
		if err := qtx.SetWritingFinished(ctx, at.ID); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if revising {
		if err := qtx.ClearWritingRevising(ctx, at.ID); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if _, err := insertWritingVersion(ctx, qtx, at.ID, wr.Title, draft.Body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if first {
		a.EnqueueHarvest(ctx, at.ID) // 见 interest_jobs.go
	}
	updated, err := a.d.Queries.GetWriting(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(updated, at.CreatedAt, at.LastActivityAt))
}
```

The draft is read before the transaction, as before, so an already-finished writing with an empty draft still answers 400 `missing_draft`.

- [ ] **Step 6: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test ./internal/api -run 'TestWritingVersion|TestAssignedWritingFinishStatus|TestLoadOwnedAtom' -count=1 -timeout 1800s`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/writing_versions.go apps/api/internal/api/writing_versions_test.go \
  apps/api/internal/api/writing_compose.go apps/api/internal/api/writings.go apps/api/internal/httpx/errors.go
git commit -m "feat(lite): finishing a writing adds a submitted version

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Revise, discard, and the writing write gate

**Files:**
- Modify: `apps/api/internal/api/writing_versions.go` (add gate + two handlers)
- Modify: `apps/api/internal/api/readings.go` (`loadOwnedAtom`)
- Modify: `apps/api/internal/api/api.go` (after `POST /api/v1/writings/{id}/finish`, line 454)
- Test: `apps/api/internal/api/writing_versions_test.go`

**Interfaces:**
- Consumes: Task 3 `writingLockFacts`, `httpx.ErrWritingLocked`; Task 1 `SetWritingRevising`, `ClearWritingRevising`, `GetLatestWritingVersion`, `GetWritingForUpdate`.
- Produces:
  - `POST /api/v1/writings/{id}/revise` → 200 writing DTO with `revisingAt`; 400 `writing_not_finished`; 403 `writing_locked`.
  - `POST /api/v1/writings/{id}/revise/discard` → 200 writing DTO (draft body and title restored from the latest version, `revisingAt` null); 403 `writing_locked`; 200 no-op when not revising.
  - `func (a *API) writingWriteGate(ctx context.Context, at sqlc.Atom) error`

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/api/writing_versions_test.go`:

```go
func errorCode(t *testing.T, h http.Handler, c *http.Cookie, method, path string, body any) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(method, path, &buf), c))
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env.Error.Code
}

// Not homework: finished → writes refused (writing_finished); revise opens
// writes; discard restores the latest version and closes them again; finish
// while revising adds version 2.
func TestWritingVersionReviseDiscardAndRefinish(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "第一版正文。")
	base := "/api/v1/writings/" + id
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}

	if code, ec := errorCode(t, h, student, "PUT", base+"/draft", map[string]any{"body": "改了"}); code != http.StatusForbidden || ec != "writing_finished" {
		t.Fatalf("draft on finished = %d %s, want 403 writing_finished", code, ec)
	}

	var revised struct {
		Status     string  `json:"status"`
		RevisingAt *string `json:"revisingAt"`
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise", nil, &revised); code != http.StatusOK {
		t.Fatalf("revise = %d", code)
	}
	if revised.Status != "finished" || revised.RevisingAt == nil {
		t.Fatalf("revise response = %+v", revised)
	}
	if code := assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "第二版正文，还没提交。"}, nil); code != http.StatusOK {
		t.Fatalf("draft while revising = %d", code)
	}
	if code := assignJSON(t, h, student, "PATCH", base, map[string]any{"title": "新标题"}, nil); code != http.StatusOK {
		t.Fatalf("rename while revising = %d", code)
	}

	var discarded struct {
		Title      string  `json:"title"`
		RevisingAt *string `json:"revisingAt"`
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise/discard", nil, &discarded); code != http.StatusOK {
		t.Fatalf("discard = %d", code)
	}
	if discarded.RevisingAt != nil || discarded.Title != "雨" {
		t.Fatalf("discard response = %+v", discarded)
	}
	var draft struct {
		Body string `json:"body"`
	}
	getJSON(t, h, student, base+"/draft", &draft)
	if draft.Body != "第一版正文。" {
		t.Fatalf("draft after discard = %q", draft.Body)
	}
	if code, ec := errorCode(t, h, student, "PUT", base+"/draft", map[string]any{"body": "x"}); code != http.StatusForbidden || ec != "writing_finished" {
		t.Fatalf("draft after discard = %d %s", code, ec)
	}

	assignJSON(t, h, student, "POST", base+"/revise", nil, nil)
	assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "第二版正文。"}, nil)
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("refinish = %d", code)
	}
	got := versionRows(t, pool, id)
	if len(got) != 2 || got[1].Number != 2 || got[1].Body != "第二版正文。" || got[0].Body != "第一版正文。" {
		t.Fatalf("versions = %+v", got)
	}
	var wr struct {
		RevisingAt *string `json:"revisingAt"`
	}
	getJSON(t, h, student, base, &wr)
	if wr.RevisingAt != nil {
		t.Fatalf("revisingAt after refinish = %v", *wr.RevisingAt)
	}
}

func TestWritingVersionReviseRefusesUnfinished(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "正文。")
	if code, ec := errorCode(t, h, student, "POST", "/api/v1/writings/"+id+"/revise", nil); code != http.StatusBadRequest || ec != "writing_not_finished" {
		t.Fatalf("revise unfinished = %d %s", code, ec)
	}
}

// Homework past its deadline, submitted, being revised: every write is
// writing_locked, and the unsubmitted draft stays where it is.
func TestWritingVersionLockedHomework(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	base := "/api/v1/writings/" + atomID
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if code := assignJSON(t, h, student, "POST", base+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("revise before due = %d", code)
	}
	if code := assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "没提交的修改。"}, nil); code != http.StatusOK {
		t.Fatalf("draft before due = %d", code)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE lite_assignment SET due_at = now() - interval '1 minute' WHERE id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, path string }{
		{"PUT", base + "/draft"}, {"POST", base + "/finish"}, {"POST", base + "/revise"}, {"POST", base + "/revise/discard"},
	} {
		if code, ec := errorCode(t, h, student, c.method, c.path, map[string]any{"body": "x"}); code != http.StatusForbidden || ec != "writing_locked" {
			t.Fatalf("%s %s after due = %d %s, want 403 writing_locked", c.method, c.path, code, ec)
		}
	}
	var draft struct {
		Body string `json:"body"`
	}
	getJSON(t, h, student, base+"/draft", &draft)
	if draft.Body != "没提交的修改。" {
		t.Fatalf("unsubmitted draft = %q, want it kept", draft.Body)
	}
}
```

Add `"bytes"`, `"encoding/json"` and `"net/http/httptest"` to the file's import block.

- [ ] **Step 2: Run to see them fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestWritingVersion(Revise|Locked)' -count=1 -timeout 1800s`
Expected: FAIL — `revise = 404` (no route yet).

- [ ] **Step 3: Implement the gate and handlers**

Append to `apps/api/internal/api/writing_versions.go` (add imports `net/http`, `time`, `mindimprint/api/internal/httpx`):

```go
// writingWriteGate is the writing branch of loadOwnedAtom's write gate.
// An open writing accepts writes. A finished writing accepts writes only
// while she is revising it and it is not locked.
func (a *API) writingWriteGate(ctx context.Context, at sqlc.Atom) error {
	wr, err := a.d.Queries.GetWriting(ctx, at.ID)
	if err != nil {
		return err
	}
	if wr.Status != "finished" {
		return nil
	}
	if !wr.RevisingAt.Valid {
		return httpx.ErrWritingFinished()
	}
	facts, err := writingLockFacts(ctx, a.d.Queries, at.ID, at.UserID)
	if err != nil {
		return err
	}
	if liteassign.Locked(facts, time.Now()) {
		return httpx.ErrWritingLocked()
	}
	return nil
}

// reviseWriting handles POST /api/v1/writings/{id}/revise: start editing a
// finished writing. Status stays finished, so a homework stays 已提交.
// Calling it while already revising keeps the original revising_at.
func (a *API) reviseWriting(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtomRow(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	wr, err := a.d.Queries.GetWriting(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if wr.Status != "finished" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("writing_not_finished", "这篇写作还没有提交", nil))
		return
	}
	facts, err := writingLockFacts(ctx, a.d.Queries, at.ID, at.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if liteassign.Locked(facts, time.Now()) {
		httpx.WriteError(w, r, httpx.ErrWritingLocked())
		return
	}
	updated, err := a.d.Queries.SetWritingRevising(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(updated, at.CreatedAt, at.LastActivityAt))
}

// discardWritingRevision handles POST /api/v1/writings/{id}/revise/discard
// (放弃修改): the draft body and the title go back to the latest version and
// revising_at is cleared. Refused when locked, so edits made before the
// deadline stay in writing_draft until the teacher returns the homework.
func (a *API) discardWritingRevision(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtomRow(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	wr, err := qtx.GetWritingForUpdate(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !wr.RevisingAt.Valid {
		httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt, at.LastActivityAt))
		return
	}
	facts, err := writingLockFacts(ctx, qtx, at.ID, at.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if liteassign.Locked(facts, time.Now()) {
		httpx.WriteError(w, r, httpx.ErrWritingLocked())
		return
	}
	latest, err := qtx.GetLatestWritingVersion(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.UpsertWritingDraft(ctx, sqlc.UpsertWritingDraftParams{AtomID: at.ID, Body: latest.Body}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := qtx.RenameWriting(ctx, sqlc.RenameWritingParams{AtomID: at.ID, Title: latest.Title}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := qtx.ClearWritingRevising(ctx, at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	updated, err := a.d.Queries.GetWriting(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(updated, at.CreatedAt, at.LastActivityAt))
}
```

In `apps/api/internal/api/readings.go`, inside `loadOwnedAtom`, replace:

```go
		finished, err := a.atomIsFinished(r.Context(), at.ID, kind)
		if err != nil {
			httpx.WriteError(w, r, err)
			return sqlc.Atom{}, false
		}
		if finished {
			httpx.WriteError(w, r, atomFinishedError(kind))
			return sqlc.Atom{}, false
		}
```

with:

```go
		if err := a.atomWriteGate(r.Context(), at, kind); err != nil {
			httpx.WriteError(w, r, err)
			return sqlc.Atom{}, false
		}
```

and add below `loadOwnedAtom`:

```go
// atomWriteGate decides whether a non-GET request may write to the atom.
// Reading: refused once finished. Writing: see writingWriteGate — a finished
// writing accepts writes while she revises it, unless the homework is locked
// (403 writing_locked).
func (a *API) atomWriteGate(ctx context.Context, at sqlc.Atom, kind string) error {
	if kind == "writing" {
		return a.writingWriteGate(ctx, at)
	}
	finished, err := a.atomIsFinished(ctx, at.ID, kind)
	if err != nil {
		return err
	}
	if finished {
		return atomFinishedError(kind)
	}
	return nil
}
```

Leave `atomIsFinished` unchanged: `atom_heartbeat.go` and `atom_report.go` still call it. In the `loadOwnedAtom` doc comment, change the sentence "Every non-GET request against an atom that is already *finished* is additionally refused" to "Every non-GET request against an atom that is already *finished* is additionally refused (a writing being revised is the exception, see atomWriteGate)".

In `apps/api/internal/api/api.go`, after the `/finish` line:

```go
	mux.Handle("POST /api/v1/writings/{id}/revise", liteOnly(a.reviseWriting))
	mux.Handle("POST /api/v1/writings/{id}/revise/discard", liteOnly(a.discardWritingRevision))
```

- [ ] **Step 4: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test ./internal/api -run 'TestWritingVersion|TestLoadOwnedAtom|TestFinishedReading|TestAssignedWriting' -count=1 -timeout 1800s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/writing_versions.go apps/api/internal/api/writing_versions_test.go \
  apps/api/internal/api/readings.go apps/api/internal/api/api.go
git commit -m "feat(lite): edit a finished writing again until its homework locks

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Returned / resubmitted in the teacher detail, list counts and student inbox

**Files:**
- Modify: `apps/api/internal/api/lite_teacher_assignments.go` (`RecipientDTO`, `assignmentStatuses`, `newRecipientDTO`, list counts in `listLiteAssignments`)
- Modify: `apps/api/internal/api/lite_student_assignments.go` (`InboxItemDTO`, `getLiteInbox`)
- Modify: `apps/api/internal/api/lite_teacher_assignments_test.go` (`len(...Counts) != 5` → `7`)
- Test: `apps/api/internal/api/lite_assignment_return_test.go`

**Interfaces:**
- Consumes: Task 1 row fields `ReturnedAt`, `ReturnDueAt`, `ReturnNote`, `VersionCount`, `Resubmitted`; Task 2 `liteassign.StatusWithReturn`, `liteassign.Return`.
- Produces:
  - `func returnOf(returnedAt, returnDueAt pgtype.Timestamptz, resubmitted bool) *liteassign.Return`
  - `RecipientDTO` gains `ReturnedAt *string "returnedAt"`, `ReturnDueAt *string "returnDueAt"`, `ReturnNote *string "returnNote"`, `VersionCount int "versionCount"`
  - `InboxItemDTO` gains `ReturnDueAt *string "returnDueAt"`, `ReturnNote *string "returnNote"`
  - `assignmentStatuses` = `not_started, in_progress, done, done_late, overdue, returned, resubmitted`
  - Test helper `recipientStatus(t, h, teacher, aid) recipientView` in `lite_assignment_return_test.go`

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/api/lite_assignment_return_test.go`:

```go
package api_test

import (
	"context"
	"net/http"
	"testing"
)

type recipientView struct {
	Status       string  `json:"status"`
	StatusLabel  string  `json:"statusLabel"`
	ReturnedAt   *string `json:"returnedAt"`
	ReturnDueAt  *string `json:"returnDueAt"`
	ReturnNote   *string `json:"returnNote"`
	VersionCount int     `json:"versionCount"`
}

func recipientStatus(t *testing.T, h http.Handler, teacher *http.Cookie, aid string) recipientView {
	t.Helper()
	var detail struct {
		Recipients []recipientView `json:"recipients"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+aid, &detail); code != http.StatusOK || len(detail.Recipients) != 1 {
		t.Fatalf("detail = %d %+v", code, detail.Recipients)
	}
	return detail.Recipients[0]
}

// The status path only; the return endpoint arrives in Task 6, so the return
// is written with SQL here.
func TestReturnedAndResubmittedStatuses(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if got := recipientStatus(t, h, teacher, aid); got.Status != "done" || got.VersionCount != 1 || got.ReturnedAt != nil {
		t.Fatalf("after finish = %+v", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE lite_assignment_recipient
		SET returned_at = now(), return_due_at = now() + interval '2 days', return_note = '请补充第二段的论据'
		WHERE assignment_id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	got := recipientStatus(t, h, teacher, aid)
	if got.Status != "returned" || got.StatusLabel != "已退回" || got.ReturnDueAt == nil || got.ReturnNote == nil || *got.ReturnNote != "请补充第二段的论据" {
		t.Fatalf("returned = %+v", got)
	}

	var inbox struct {
		Items []struct {
			Status      string  `json:"status"`
			StatusLabel string  `json:"statusLabel"`
			ReturnDueAt *string `json:"returnDueAt"`
			ReturnNote  *string `json:"returnNote"`
		} `json:"items"`
	}
	getJSON(t, h, student, "/api/v1/lite/inbox", &inbox)
	if len(inbox.Items) != 1 || inbox.Items[0].Status != "returned" || inbox.Items[0].ReturnDueAt == nil || inbox.Items[0].ReturnNote == nil {
		t.Fatalf("inbox = %+v", inbox.Items)
	}

	var list struct {
		Assignments []struct {
			Counts map[string]int `json:"counts"`
		} `json:"assignments"`
	}
	getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/assignments", &list)
	if len(list.Assignments) != 1 || list.Assignments[0].Counts["returned"] != 1 || len(list.Assignments[0].Counts) != 7 {
		t.Fatalf("counts = %+v", list.Assignments)
	}

	// Past the return deadline with no new version → overdue.
	if _, err := pool.Exec(ctx, `UPDATE lite_assignment_recipient SET return_due_at = now() - interval '1 minute' WHERE assignment_id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	if got := recipientStatus(t, h, teacher, aid); got.Status != "overdue" {
		t.Fatalf("past return due = %s, want overdue", got.Status)
	}

	// A version submitted after the return → resubmitted, whatever the deadline.
	if _, err := pool.Exec(ctx, `INSERT INTO writing_version (atom_id, number, title, body, word_count, submitted_at)
		VALUES ($1, 2, '雨', '第二版。', 4, now())`, atomID); err != nil {
		t.Fatal(err)
	}
	if got := recipientStatus(t, h, teacher, aid); got.Status != "resubmitted" || got.StatusLabel != "已重新提交" || got.VersionCount != 2 {
		t.Fatalf("resubmitted = %+v", got)
	}
}
```

In `apps/api/internal/api/lite_teacher_assignments_test.go`, `TestTeacherCreatesAndListsAssignment`, change `len(list.Assignments[0].Counts) != 5` to `len(list.Assignments[0].Counts) != 7`.

- [ ] **Step 2: Run to see it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestReturnedAndResubmittedStatuses|TestTeacherCreatesAndListsAssignment' -count=1 -timeout 1800s`
Expected: FAIL — `returned = {Status:done …}` and `counts` length 5.

- [ ] **Step 3: Implement**

In `apps/api/internal/api/lite_teacher_assignments.go`:

Add to `RecipientDTO` after `SeenAt`:

```go
	// Return fields are set once the teacher has returned the writing (0153).
	ReturnedAt   *string `json:"returnedAt"`
	ReturnDueAt  *string `json:"returnDueAt"`
	ReturnNote   *string `json:"returnNote"`
	VersionCount int     `json:"versionCount"`
```

Replace `assignmentStatuses`:

```go
// assignmentStatuses are the wire statuses liteassign.StatusWithReturn returns.
var assignmentStatuses = []string{"not_started", "in_progress", "done", "done_late", "overdue", "returned", "resubmitted"}
```

Add after `uuidStringPtr`:

```go
// returnOf turns a recipient's return columns into liteassign's input; nil
// when the teacher never returned it.
func returnOf(returnedAt, returnDueAt pgtype.Timestamptz, resubmitted bool) *liteassign.Return {
	if !returnedAt.Valid || !returnDueAt.Valid {
		return nil
	}
	return &liteassign.Return{DueAt: returnDueAt.Time, Resubmitted: resubmitted}
}
```

Replace `newRecipientDTO`:

```go
func newRecipientDTO(row sqlc.ListLiteAssignmentRecipientsRow, dueAt, now time.Time) RecipientDTO {
	status := liteassign.StatusWithReturn(row.StartedAt.Valid, tsPtr(row.FinishedAt), dueAt, now,
		returnOf(row.ReturnedAt, row.ReturnDueAt, row.Resubmitted))
	return RecipientDTO{
		UserID: row.UserID.String(), DisplayName: row.DisplayName, AvatarColor: row.AvatarColor,
		Status: status, StatusLabel: liteassign.StatusLabel(status),
		AtomID: uuidStringPtr(row.AtomID), StartedAt: tsStringPtr(row.StartedAt),
		FinishedAt: tsStringPtr(row.FinishedAt), SeenAt: tsStringPtr(row.SeenAt),
		ReturnedAt: tsStringPtr(row.ReturnedAt), ReturnDueAt: tsStringPtr(row.ReturnDueAt),
		ReturnNote: row.ReturnNote, VersionCount: int(row.VersionCount),
	}
}
```

In `listLiteAssignments`, replace the status line inside the recipients loop:

```go
			status := liteassign.StatusWithReturn(rc.StartedAt.Valid, tsPtr(rc.FinishedAt), rows[i].DueAt, now,
				returnOf(rc.ReturnedAt, rc.ReturnDueAt, rc.Resubmitted))
```

In `apps/api/internal/api/lite_student_assignments.go`, add to `InboxItemDTO` after `Unread`:

```go
	// Set when the teacher returned this writing homework (0153).
	ReturnDueAt *string `json:"returnDueAt"`
	ReturnNote  *string `json:"returnNote"`
```

In `getLiteInbox`, replace the status line and the `append` call:

```go
		status := liteassign.StatusWithReturn(row.StartedAt.Valid, tsPtr(row.FinishedAt), row.DueAt, now,
			returnOf(row.ReturnedAt, row.ReturnDueAt, row.Resubmitted))
```

```go
		items = append(items, InboxItemDTO{
			Type: "assignment", ID: row.ID.String(), Kind: row.Kind, Title: row.Title,
			Instructions: row.Instructions, ClassName: row.ClassName, DueAt: row.DueAt.Format(time.RFC3339),
			Status: status, StatusLabel: liteassign.StatusLabel(status),
			AtomID: uuidStringPtr(row.AtomID), Unread: !row.SeenAt.Valid,
			ReturnDueAt: tsStringPtr(row.ReturnDueAt), ReturnNote: row.ReturnNote,
		})
```

- [ ] **Step 4: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test ./internal/api -run 'TestReturnedAndResubmittedStatuses|TestTeacherCreates|TestTeacherAssignment|TestInbox|TestAssignedWritingFinishStatus|TestLiteRoster|TestLiteWeekly' -count=1 -timeout 1800s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/lite_teacher_assignments.go apps/api/internal/api/lite_student_assignments.go \
  apps/api/internal/api/lite_teacher_assignments_test.go apps/api/internal/api/lite_assignment_return_test.go
git commit -m "feat(lite): returned and resubmitted statuses for writing homework

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: 退回修改 endpoint

**Files:**
- Create: `apps/api/internal/api/lite_teacher_return.go`
- Modify: `apps/api/internal/api/lite_teacher_routes.go` (Assignments block)
- Test: `apps/api/internal/api/lite_assignment_return_test.go`

**Interfaces:**
- Consumes: Task 1 `GetLiteAssignmentRecipientForUpdate` (existing), `CountWritingVersions`, `SetLiteAssignmentReturned`; Task 5 `newRecipientDTO`; existing `loadTeacherAssignment`, `parseAssignmentDueAt`, `writeNotFoundOr`.
- Produces: `POST /api/v1/lite/teacher/assignments/{aid}/recipients/{userId}/return` body `{dueAt: RFC3339, note?: string}` → 200 `{"recipient": RecipientDTO}`. Errors: 400 `not_writing_assignment` 「只有写作作业可以退回修改」; 400 `invalid_due_at` 「截止时间格式错误」 or 「新的截止时间需要晚于现在」; 400 `invalid_note` 「退回说明不超过 500 字」; 404 not a recipient / not the teacher's class; 409 `no_submission` 「这名学生还没有提交」.

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/api/lite_assignment_return_test.go` (add imports `strings`, `time`):

```go
func returnPath(aid, userID string) string {
	return "/api/v1/lite/teacher/assignments/" + aid + "/recipients/" + userID + "/return"
}

func TestReturnWritingHomework(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	path := returnPath(aid, studentID.String())
	future := time.Now().Add(72 * time.Hour).Format(time.RFC3339)

	if code, ec := errorCode(t, h, teacher, "POST", path, map[string]any{"dueAt": future}); code != http.StatusConflict || ec != "no_submission" {
		t.Fatalf("return before a version = %d %s", code, ec)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	// Past the original deadline the homework is locked.
	if _, err := pool.Exec(ctx, `UPDATE lite_assignment SET due_at = now() - interval '1 hour' WHERE id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	if code, ec := errorCode(t, h, student, "POST", "/api/v1/writings/"+atomID+"/revise", nil); code != http.StatusForbidden || ec != "writing_locked" {
		t.Fatalf("revise when locked = %d %s", code, ec)
	}

	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	if code, ec := errorCode(t, h, teacher, "POST", path, map[string]any{"dueAt": past}); code != http.StatusBadRequest || ec != "invalid_due_at" {
		t.Fatalf("past dueAt = %d %s", code, ec)
	}
	if code, ec := errorCode(t, h, teacher, "POST", path, map[string]any{"dueAt": future, "note": strings.Repeat("字", 501)}); code != http.StatusBadRequest || ec != "invalid_note" {
		t.Fatalf("long note = %d %s", code, ec)
	}

	var resp struct {
		Recipient recipientView `json:"recipient"`
	}
	if code := assignJSON(t, h, teacher, "POST", path, map[string]any{"dueAt": future, "note": "  请补充第二段的论据  "}, &resp); code != http.StatusOK {
		t.Fatalf("return = %d", code)
	}
	if resp.Recipient.Status != "returned" || resp.Recipient.ReturnNote == nil || *resp.Recipient.ReturnNote != "请补充第二段的论据" {
		t.Fatalf("return response = %+v", resp.Recipient)
	}

	// Returned: the lock uses the new deadline, so she can edit again.
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("revise after return = %d", code)
	}
	if code := assignJSON(t, h, student, "PUT", "/api/v1/writings/"+atomID+"/draft", map[string]any{"body": "补了论据。"}, nil); code != http.StatusOK {
		t.Fatalf("draft after return = %d", code)
	}

	// Returning again overwrites; an empty note is stored as NULL.
	later := time.Now().Add(96 * time.Hour).Format(time.RFC3339)
	if code := assignJSON(t, h, teacher, "POST", path, map[string]any{"dueAt": later}, &resp); code != http.StatusOK {
		t.Fatalf("second return = %d", code)
	}
	if resp.Recipient.ReturnNote != nil || resp.Recipient.ReturnDueAt == nil {
		t.Fatalf("second return = %+v", resp.Recipient)
	}
}

func TestReturnRefusals(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	future := time.Now().Add(72 * time.Hour).Format(time.RFC3339)
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil)

	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ret-other@demo.local"))
	if code, _ := errorCode(t, h, other, "POST", returnPath(aid, studentID.String()), map[string]any{"dueAt": future}); code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404", code)
	}
	stranger := createStudent(t, pool, SeedSchoolID, "ret-stranger@demo.local")
	if code, _ := errorCode(t, h, teacher, "POST", returnPath(aid, stranger.String()), map[string]any{"dueAt": future}); code != http.StatusNotFound {
		t.Fatalf("not a recipient = %d, want 404", code)
	}
	if code, _ := errorCode(t, h, teacher, "POST", returnPath(aid, "not-a-uuid"), map[string]any{"dueAt": future}); code != http.StatusNotFound {
		t.Fatalf("bad user id = %d, want 404", code)
	}
	if code, _ := errorCode(t, h, student, "POST", returnPath(aid, studentID.String()), map[string]any{"dueAt": future}); code != http.StatusForbidden && code != http.StatusNotFound {
		t.Fatalf("student calling return = %d, want 403 or 404", code)
	}

	reading := createAssignment(t, h, teacher, classID, readingAssignmentBody("读", map[string]any{"source": "text", "text": "一段正文。"}, []string{studentID.String()}))
	if code, ec := errorCode(t, h, teacher, "POST", returnPath(reading, studentID.String()), map[string]any{"dueAt": future}); code != http.StatusBadRequest || ec != "not_writing_assignment" {
		t.Fatalf("reading homework = %d %s", code, ec)
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestReturn(WritingHomework|Refusals)' -count=1 -timeout 1800s`
Expected: FAIL — `return before a version = 404` (route missing; the mux answers 404 with no code) and the refusal codes missing.

- [ ] **Step 3: Implement**

Create `apps/api/internal/api/lite_teacher_return.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

const maxReturnNoteRunes = 500

// returnLiteAssignmentRecipient handles
// POST /api/v1/lite/teacher/assignments/{aid}/recipients/{userId}/return (退回修改).
// Writing homework only; the student must have submitted at least one
// version; the new deadline must be in the future. Returning again
// overwrites returned_at, return_due_at and return_note.
func (a *API) returnLiteAssignmentRecipient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	as, ok := a.loadTeacherAssignment(w, r)
	if !ok {
		return
	}
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if as.Kind != "writing" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("not_writing_assignment", "只有写作作业可以退回修改", nil))
		return
	}
	var req struct {
		DueAt string `json:"dueAt"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	due, err := parseAssignmentDueAt(req.DueAt)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !due.After(time.Now()) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_due_at", "新的截止时间需要晚于现在", nil))
		return
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > maxReturnNoteRunes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_note", "退回说明不超过 500 字", nil))
		return
	}
	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	rc, err := qtx.GetLiteAssignmentRecipientForUpdate(ctx, sqlc.GetLiteAssignmentRecipientForUpdateParams{AssignmentID: as.ID, UserID: userID})
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	noSubmission := &httpx.APIError{Status: http.StatusConflict, Code: "no_submission", Message: "这名学生还没有提交"}
	if !rc.AtomID.Valid {
		httpx.WriteError(w, r, noSubmission)
		return
	}
	n, err := qtx.CountWritingVersions(ctx, uuid.UUID(rc.AtomID.Bytes))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if n == 0 {
		httpx.WriteError(w, r, noSubmission)
		return
	}
	if _, err := qtx.SetLiteAssignmentReturned(ctx, sqlc.SetLiteAssignmentReturnedParams{
		AssignmentID: as.ID, UserID: userID,
		ReturnDueAt: pgtype.Timestamptz{Time: due, Valid: true}, ReturnNote: notePtr,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	rows, err := a.d.Queries.ListLiteAssignmentRecipients(ctx, []uuid.UUID{as.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	now := time.Now()
	for _, row := range rows {
		if row.UserID == userID {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"recipient": newRecipientDTO(row, as.DueAt, now)})
			return
		}
	}
	httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
}
```

If sqlc named the generated params type differently, run `grep -n "GetLiteAssignmentRecipientForUpdate" internal/store/sqlc/lite_assignment.sql.go` and use that name; the query has two positional params, so it is `GetLiteAssignmentRecipientForUpdateParams{AssignmentID, UserID}`.

In `apps/api/internal/api/lite_teacher_routes.go`, after the `extractLiteAssignment` line:

```go
	mux.Handle("POST /api/v1/lite/teacher/assignments/{aid}/recipients/{userId}/return", liteTeacher(a.returnLiteAssignmentRecipient))
```

- [ ] **Step 4: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test ./internal/api -run 'TestReturn|TestTeacherAssignment' -count=1 -timeout 1800s`
Expected: PASS. If the student call answers something other than 403/404, check `RequireRole` in `lite_teacher_routes.go`; do not loosen the assertion.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/lite_teacher_return.go apps/api/internal/api/lite_teacher_routes.go \
  apps/api/internal/api/lite_assignment_return_test.go
git commit -m "feat(lite): teacher returns a submitted writing homework with a new deadline

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Version read endpoints (student and teacher) and for-atom return fields

**Files:**
- Modify: `apps/api/internal/api/writing_versions.go` (DTOs + two student handlers)
- Modify: `apps/api/internal/api/lite_student_assignments.go` (`getLiteAssignmentForAtom`)
- Modify: `apps/api/internal/api/lite_teacher_item.go` (`liteTeacherWriting` + `getLiteTeacherWritingVersion`)
- Modify: `apps/api/internal/api/api.go`, `apps/api/internal/api/lite_teacher_routes.go`
- Test: `apps/api/internal/api/writing_versions_test.go`

**Interfaces:**
- Consumes: Task 1 `ListWritingVersions`, `GetWritingVersion`; Task 3 `writingLockFacts`; Task 2 `liteassign.LockReason`.
- Produces:
  - `type writingVersionSummaryDTO struct { Number int32 "number"; Title string "title"; WordCount int32 "wordCount"; SubmittedAt string "submittedAt" }`
  - `type writingVersionDTO struct { writingVersionSummaryDTO; Body string "body" }`
  - `func writingVersionSummaries(rows []sqlc.ListWritingVersionsRow) []writingVersionSummaryDTO` (never nil)
  - `func writingVersionDTOOf(v sqlc.WritingVersion) writingVersionDTO`
  - `GET /api/v1/writings/{id}/versions` → `{"versions": [summary…newest first], "locked": bool, "lockReason": "past_due" | null}`
  - `GET /api/v1/writings/{id}/versions/{n}` → `writingVersionDTO`; 404 for a missing `n`
  - `GET /api/v1/lite/assignments/for-atom/{atomId}` → `assignment` gains `kind`, `returnedAt`, `returnDueAt`, `returnNote`, `resubmitted`
  - Teacher item payload `writing.versions` (summaries) and `writing.revising` (bool)
  - `GET /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}/versions/{n}` → `writingVersionDTO`

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/api/writing_versions_test.go`:

```go
type versionSummary struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	WordCount   int    `json:"wordCount"`
	SubmittedAt string `json:"submittedAt"`
}

func TestWritingVersionReadEndpoints(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	base := "/api/v1/writings/" + atomID
	assignJSON(t, h, student, "POST", base+"/finish", nil, nil)
	assignJSON(t, h, student, "POST", base+"/revise", nil, nil)
	assignJSON(t, h, student, "PUT", base+"/draft", map[string]any{"body": "雨下了一整夜。"}, nil)
	if code := assignJSON(t, h, student, "POST", base+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("second finish = %d", code)
	}

	var list struct {
		Versions   []versionSummary `json:"versions"`
		Locked     bool             `json:"locked"`
		LockReason *string          `json:"lockReason"`
	}
	if code := getJSON(t, h, student, base+"/versions", &list); code != http.StatusOK {
		t.Fatalf("list = %d", code)
	}
	if len(list.Versions) != 2 || list.Versions[0].Number != 2 || list.Versions[1].Number != 1 || list.Locked || list.LockReason != nil {
		t.Fatalf("list = %+v", list)
	}

	var v1 struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
	}
	if code := getJSON(t, h, student, base+"/versions/1", &v1); code != http.StatusOK || v1.Number != 1 || v1.Body != "雨下了一整天。" {
		t.Fatalf("v1 = %d %+v", code, v1)
	}
	for _, n := range []string{"3", "0", "x"} {
		if code := getJSON(t, h, student, base+"/versions/"+n, nil); code != http.StatusNotFound {
			t.Fatalf("version %s = %d, want 404", n, code)
		}
	}
	other := signInAs(t, pool, createStudent(t, pool, SeedSchoolID, "ver-other@demo.local"))
	if code := getJSON(t, h, other, base+"/versions", nil); code != http.StatusNotFound {
		t.Fatalf("other student list = %d, want 404", code)
	}

	// Locked after the deadline: the list says so.
	if _, err := pool.Exec(context.Background(), `UPDATE lite_assignment SET due_at = now() - interval '1 minute' WHERE id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	getJSON(t, h, student, base+"/versions", &list)
	if !list.Locked || list.LockReason == nil || *list.LockReason != "past_due" {
		t.Fatalf("locked list = %+v", list)
	}

	// for-atom carries the return fields.
	if _, err := pool.Exec(context.Background(), `UPDATE lite_assignment_recipient SET returned_at = now(), return_due_at = now() + interval '1 day', return_note = '补充论据' WHERE assignment_id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	var forAtom struct {
		Assignment struct {
			Kind        string  `json:"kind"`
			ReturnedAt  *string `json:"returnedAt"`
			ReturnDueAt *string `json:"returnDueAt"`
			ReturnNote  *string `json:"returnNote"`
			Resubmitted bool    `json:"resubmitted"`
		} `json:"assignment"`
	}
	getJSON(t, h, student, "/api/v1/lite/assignments/for-atom/"+atomID, &forAtom)
	if forAtom.Assignment.Kind != "writing" || forAtom.Assignment.ReturnedAt == nil || forAtom.Assignment.ReturnDueAt == nil ||
		forAtom.Assignment.ReturnNote == nil || forAtom.Assignment.Resubmitted {
		t.Fatalf("for-atom = %+v", forAtom.Assignment)
	}

	// Teacher: the item payload lists versions; the version route reads one.
	itemBase := "/api/v1/lite/teacher/classes/" + classID + "/students/" + studentID.String() + "/items/" + atomID
	var item struct {
		Writing struct {
			Versions []versionSummary `json:"versions"`
			Revising bool             `json:"revising"`
		} `json:"writing"`
	}
	if code := getJSON(t, h, teacher, itemBase, &item); code != http.StatusOK || len(item.Writing.Versions) != 2 || item.Writing.Revising {
		t.Fatalf("teacher item = %d %+v", code, item.Writing)
	}
	var tv struct {
		Body string `json:"body"`
	}
	if code := getJSON(t, h, teacher, itemBase+"/versions/2", &tv); code != http.StatusOK || tv.Body != "雨下了一整夜。" {
		t.Fatalf("teacher v2 = %d %q", code, tv.Body)
	}
	otherTeacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ver-other-teacher@demo.local"))
	if code := getJSON(t, h, otherTeacher, itemBase+"/versions/2", nil); code != http.StatusNotFound {
		t.Fatalf("other teacher version = %d, want 404", code)
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestWritingVersionReadEndpoints -count=1 -timeout 1800s`
Expected: FAIL — `list = 404`.

- [ ] **Step 3: Implement the student endpoints**

Append to `apps/api/internal/api/writing_versions.go` (add import `strconv`):

```go
type writingVersionSummaryDTO struct {
	Number      int32  `json:"number"`
	Title       string `json:"title"`
	WordCount   int32  `json:"wordCount"`
	SubmittedAt string `json:"submittedAt"`
}

type writingVersionDTO struct {
	writingVersionSummaryDTO
	Body string `json:"body"`
}

func writingVersionSummaries(rows []sqlc.ListWritingVersionsRow) []writingVersionSummaryDTO {
	out := make([]writingVersionSummaryDTO, 0, len(rows))
	for _, v := range rows {
		out = append(out, writingVersionSummaryDTO{
			Number: v.Number, Title: v.Title, WordCount: v.WordCount,
			SubmittedAt: v.SubmittedAt.Format(time.RFC3339),
		})
	}
	return out
}

func writingVersionDTOOf(v sqlc.WritingVersion) writingVersionDTO {
	return writingVersionDTO{
		writingVersionSummaryDTO: writingVersionSummaryDTO{
			Number: v.Number, Title: v.Title, WordCount: v.WordCount,
			SubmittedAt: v.SubmittedAt.Format(time.RFC3339),
		},
		Body: v.Body,
	}
}

// versionNumber reads {n}; anything that is not a positive integer is 404.
func versionNumber(r *http.Request) (int32, bool) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 1 || n > 1<<30 {
		return 0, false
	}
	return int32(n), true
}

// listWritingVersionsHandler handles GET /api/v1/writings/{id}/versions.
func (a *API) listWritingVersionsHandler(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	rows, err := a.d.Queries.ListWritingVersions(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	facts, err := writingLockFacts(ctx, a.d.Queries, at.ID, at.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	now := time.Now()
	var reason *string
	if s := liteassign.LockReason(facts, now); s != "" {
		reason = &s
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"versions":   writingVersionSummaries(rows),
		"locked":     liteassign.Locked(facts, now),
		"lockReason": reason,
	})
}

// getWritingVersionHandler handles GET /api/v1/writings/{id}/versions/{n}.
func (a *API) getWritingVersionHandler(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	n, ok := versionNumber(r)
	if !ok {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	v, err := a.d.Queries.GetWritingVersion(r.Context(), sqlc.GetWritingVersionParams{AtomID: at.ID, Number: n})
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingVersionDTOOf(v))
}
```

In `apps/api/internal/api/api.go`, after the two revise routes:

```go
	mux.Handle("GET /api/v1/writings/{id}/versions", liteOnly(a.listWritingVersionsHandler))
	mux.Handle("GET /api/v1/writings/{id}/versions/{n}", liteOnly(a.getWritingVersionHandler))
```

In `apps/api/internal/api/lite_student_assignments.go`, `getLiteAssignmentForAtom`, replace the final `WriteJSON` with:

```go
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"assignment": map[string]any{
		"id": row.ID.String(), "kind": row.Kind, "title": row.Title, "dueAt": row.DueAt.Format(time.RFC3339),
		"returnedAt": tsStringPtr(row.ReturnedAt), "returnDueAt": tsStringPtr(row.ReturnDueAt),
		"returnNote": row.ReturnNote, "resubmitted": row.Resubmitted,
	}})
```

- [ ] **Step 4: Implement the teacher parts**

In `apps/api/internal/api/lite_teacher_item.go`, inside `liteTeacherWriting`, before the final `return`:

```go
	versionRows, err := a.d.Queries.ListWritingVersions(ctx, atomID)
	if err != nil {
		return nil, err
	}
```

and add two keys to the returned map:

```go
		"versions":     writingVersionSummaries(versionRows),
		"revising":     wr.RevisingAt.Valid,
```

Add after `getLiteTeacherItem`:

```go
// getLiteTeacherWritingVersion handles
// GET /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}/versions/{n}.
// Same gate as the item page: authTeacherStudent, and the atom must be this
// student's writing. A version is her submitted text, never chat content.
func (a *API) getLiteTeacherWritingVersion(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	atomID, err := uuid.Parse(r.PathValue("atomId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	ctx := r.Context()
	at, err := a.d.Queries.GetAtom(ctx, atomID)
	if err != nil || at.UserID != userID || at.Kind != "writing" {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	n, ok := versionNumber(r)
	if !ok {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	v, err := a.d.Queries.GetWritingVersion(ctx, sqlc.GetWritingVersionParams{AtomID: atomID, Number: n})
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingVersionDTOOf(v))
}
```

In `apps/api/internal/api/lite_teacher_routes.go`, after the `items/{atomId}` line:

```go
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}/versions/{n}", liteTeacher(a.getLiteTeacherWritingVersion))
```

- [ ] **Step 5: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test ./internal/api -run 'TestWritingVersion|TestLiteItemDetail|TestForAtom' -count=1 -timeout 1800s`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/writing_versions.go apps/api/internal/api/writing_versions_test.go \
  apps/api/internal/api/lite_student_assignments.go apps/api/internal/api/lite_teacher_item.go \
  apps/api/internal/api/api.go apps/api/internal/api/lite_teacher_routes.go
git commit -m "feat(lite): read submitted writing versions (student and teacher)

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: The report's piece follows the latest version

**Files:**
- Modify: `apps/api/internal/api/atom_report.go` (`reportWithPiece`, `mergeLiveWritingFields` and its doc comment)
- Test: `apps/api/internal/api/atom_report_piece_internal_test.go`

**Interfaces:**
- Consumes: Task 1 `GetLatestWritingVersion`.
- Produces: `func mergeLiveWritingFields(fields map[string]json.RawMessage, draftBody, versionBody, title string) bool`. Rule: a non-blank `versionBody` always sets `piece`; with no version, the old rule (draft only when `piece` is missing) applies. Title rule unchanged.

- [ ] **Step 1: Update and add the failing tests**

In `apps/api/internal/api/atom_report_piece_internal_test.go`, every existing call gains an empty `versionBody` as the third argument:
- `mergeLiveWritingFields(fields, tc.draftBody, "t")` → `mergeLiveWritingFields(fields, tc.draftBody, "", "t")`
- `mergeLiveWritingFields(fields, "", "课间十分钟")` → `mergeLiveWritingFields(fields, "", "", "课间十分钟")`
- `mergeLiveWritingFields(fields, "", "   ")` → `mergeLiveWritingFields(fields, "", "", "   ")`
- `mergeLiveWritingFields(fields, "正文。", "转弯中的国家")` → `mergeLiveWritingFields(fields, "正文。", "", "转弯中的国家")`
- `mergeLiveWritingFields(after, "正文。", "新标题")` → `mergeLiveWritingFields(after, "正文。", "", "新标题")`

Append:

```go
// Once versions exist (0153), the piece is the latest submitted version:
// she can edit after 完成, and the report must show what she submitted, not
// what the report stored at generation time or her unsubmitted draft.
func TestMergeLiveWritingFieldsLatestVersionWins(t *testing.T) {
	cases := []struct {
		name        string
		stored      string
		draftBody   string
		versionBody string
		want        string
		wantChanged bool
	}{
		{"version replaces the stored piece", `{"kind":"writing","title":"t","piece":"第一版。"}`, "还没提交的修改。", "第二版。", "第二版。", true},
		{"version fills a missing piece", `{"kind":"writing","title":"t"}`, "草稿。", "第一版。", "第一版。", true},
		{"same text is not a change", `{"kind":"writing","title":"t","piece":"第二版。"}`, "", "第二版。", "第二版。", false},
		{"blank version falls back to the old rule", `{"kind":"writing","title":"t","piece":"报告里存着的正文。"}`, "草稿。", "   ", "报告里存着的正文。", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := decode(t, tc.stored)
			changed := mergeLiveWritingFields(fields, tc.draftBody, tc.versionBody, "t")
			if got := str(t, fields["piece"]); got != tc.want {
				t.Fatalf("piece = %q, want %q", got, tc.want)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
		})
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestMergeLiveWritingFields|TestReportWithPiece' -count=1`
Expected: FAIL to compile — `too many arguments in call to mergeLiveWritingFields`.

- [ ] **Step 3: Implement**

In `apps/api/internal/api/atom_report.go`, in `reportWithPiece`, after the title lookup add:

```go
	versionBody := ""
	if v, err := a.d.Queries.GetLatestWritingVersion(ctx, atomID); err == nil {
		versionBody = v.Body
	}
```

and change the merge call to `mergeLiveWritingFields(fields, draftBody, versionBody, title)`.

Replace `mergeLiveWritingFields` with:

```go
func mergeLiveWritingFields(fields map[string]json.RawMessage, draftBody, versionBody, title string) bool {
	changed := false
	if body := strings.TrimSpace(versionBody); body != "" {
		if encoded, err := json.Marshal(body); err == nil && !bytes.Equal(fields["piece"], encoded) {
			fields["piece"] = encoded
			changed = true
		}
	} else if body := strings.TrimSpace(draftBody); body != "" && !hasNonEmptyString(fields, "piece") {
		if encoded, err := json.Marshal(body); err == nil {
			fields["piece"] = encoded
			changed = true
		}
	}
	if t := strings.TrimSpace(title); t != "" {
		if encoded, err := json.Marshal(t); err == nil {
			// Not a change if it already says exactly this — a no-op rewrite
			// would re-marshal the whole blob on every read for nothing.
			if !bytes.Equal(fields["title"], encoded) {
				fields["title"] = encoded
				changed = true
			}
		}
	}
	return changed
}
```

In its doc comment, replace the paragraph starting `**piece: only when missing.**` with:

```go
// **piece: the latest submitted version, when there is one.** Since 0153 she
// can edit after 完成, so the piece is the newest writing_version body; stats
// stay as generated. A writing with no version (none should remain after the
// 0153 backfill) keeps the older rule: the draft fills `piece` only when the
// blob has none, and a stored `""` counts as missing.
```

The encoded value is compared against the stored raw JSON, so trimming matters: `json.Marshal` of the trimmed body must equal what the previous read wrote. Both paths trim, so repeated reads are stable.

- [ ] **Step 4: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestMergeLiveWritingFields|TestReportWithPiece' -count=1 -v`
Expected: PASS, including `TestMergeLiveWritingFieldsLatestVersionWins` (4 subtests) and the existing tests.

Then run the whole lite surface once: `cd apps/api && CGO_ENABLED=0 go test ./internal/api ./internal/liteassign ./internal/store -count=1 -timeout 1800s`
Expected: `ok` for all three packages.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/atom_report.go apps/api/internal/api/atom_report_piece_internal_test.go
git commit -m "fix(lite): writing report shows the latest submitted version

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Frontend API types, clients and the version diff

**Files:**
- Modify: `apps/lite-web/src/api/writings.ts`
- Modify: `apps/lite-web/src/api/assignments.ts`
- Modify: `apps/lite-web/src/shared/deadline.ts` (`AssignmentStatus`, `STATUS_LABEL`)
- Modify: `apps/lite-web/src/teacher/assignmentLogic.ts` (`STATUS_ORDER`, `STATUS_HUE`)
- Create: `apps/lite-web/src/writings/versionDiff.ts`, `apps/lite-web/src/writings/versionDiff.test.ts`
- Test: `apps/lite-web/src/api/assignments.test.ts` (append)
- Modify (fixtures only): `apps/lite-web/src/teacher/assignmentLogic.test.ts` (`recipient()`), `apps/lite-web/src/inbox/inboxLogic.test.ts` (`item()`)

**Interfaces:**
- Consumes: Task 7 JSON shapes; Task 6 return endpoint.
- Produces:
  - `writings.ts`: `Writing.revisingAt?: string | null`; `isRevising(w: Pick<Writing, "revisingAt">): boolean`; `isWritingFinished(w: Pick<Writing, "status" | "finishedAt">): boolean` (widened); `interface WritingVersionSummary { number: number; title: string; wordCount: number; submittedAt: string }`; `interface WritingVersion extends WritingVersionSummary { body: string }`; `interface WritingVersionList { versions: WritingVersionSummary[]; locked: boolean; lockReason: string | null }`; `normalizeWritingVersionList(raw: unknown): WritingVersionList`; `listWritingVersions(id): Promise<WritingVersionList>`; `getWritingVersion(id, n): Promise<WritingVersion>`; `reviseWriting(id): Promise<Writing>`; `discardWritingRevision(id): Promise<Writing>`
  - `deadline.ts`: `AssignmentStatus` adds `"returned" | "resubmitted"`; labels `已退回`, `已重新提交`
  - `assignments.ts`: `RecipientDTO` adds `returnedAt, returnDueAt, returnNote: string | null; versionCount: number`; `AssignmentInboxItem` adds `returnDueAt, returnNote: string | null`; `AssignmentForAtom` adds `kind: AssignmentKind; returnedAt, returnDueAt, returnNote: string | null; resubmitted: boolean`; `interface ReturnRecipientInput { dueAt: string; note?: string }`; `returnRecipient(aid, userId, input): Promise<RecipientDTO>`
  - `versionDiff.ts`: `type DiffPart = { kind: "same" | "add" | "del"; text: string }`; `type ParagraphDiff = { kind: "same" | "add" | "del"; text: string } | { kind: "change"; parts: DiffPart[] }`; `diffVersions(older: string, newer: string): ParagraphDiff[]`; `diffChars(older: string, newer: string): DiffPart[]`

- [ ] **Step 1: Write the failing diff tests**

Create `apps/lite-web/src/writings/versionDiff.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { diffChars, diffVersions } from "./versionDiff";

// The diff is shown as 与当前版本对比: `older` is the version she picked,
// `newer` is the latest version. "add" = in the latest only, "del" = gone from it.
describe("diffVersions", () => {
  it("marks identical texts as unchanged paragraphs", () =>
    expect(diffVersions("甲段。\n\n乙段。", "甲段。\n\n乙段。")).toEqual([
      { kind: "same", text: "甲段。" },
      { kind: "same", text: "乙段。" },
    ]));

  it("reports an added paragraph", () =>
    expect(diffVersions("甲段。", "甲段。\n\n新的一段。")).toEqual([
      { kind: "same", text: "甲段。" },
      { kind: "add", text: "新的一段。" },
    ]));

  it("reports a deleted paragraph", () =>
    expect(diffVersions("甲段。\n\n删掉的一段。\n\n乙段。", "甲段。\n\n乙段。")).toEqual([
      { kind: "same", text: "甲段。" },
      { kind: "del", text: "删掉的一段。" },
      { kind: "same", text: "乙段。" },
    ]));

  it("pairs a replaced paragraph and marks characters inside it", () =>
    expect(diffVersions("雨下了一整天。", "雨下了一整夜。")).toEqual([
      {
        kind: "change",
        parts: [
          { kind: "same", text: "雨下了一整" },
          { kind: "del", text: "天" },
          { kind: "add", text: "夜" },
          { kind: "same", text: "。" },
        ],
      },
    ]));

  it("pairs in order and leaves the extra paragraph as an addition", () =>
    expect(diffVersions("A段\n\nB段", "A段\n\nB段改\n\nC段")).toEqual([
      { kind: "same", text: "A段" },
      { kind: "change", parts: [{ kind: "same", text: "B段" }, { kind: "add", text: "改" }] },
      { kind: "add", text: "C段" },
    ]));

  it("ignores blank-line and carriage-return differences between paragraphs", () =>
    expect(diffVersions("甲段。\r\n\r\n\r\n乙段。", "甲段。\n\n乙段。")).toEqual([
      { kind: "same", text: "甲段。" },
      { kind: "same", text: "乙段。" },
    ]));

  it("handles empty inputs", () => {
    expect(diffVersions("", "")).toEqual([]);
    expect(diffVersions("", "新")).toEqual([{ kind: "add", text: "新" }]);
    expect(diffVersions("旧", "")).toEqual([{ kind: "del", text: "旧" }]);
  });
});

describe("diffChars", () => {
  it("keeps astral characters whole", () =>
    expect(diffChars("a😀b", "a😀c")).toEqual([
      { kind: "same", text: "a😀" },
      { kind: "del", text: "b" },
      { kind: "add", text: "c" },
    ]));

  it("falls back to whole-paragraph replace when the table would be too large", () => {
    const older = "甲".repeat(600);
    const newer = "乙".repeat(600);
    expect(diffChars(older, newer)).toEqual([
      { kind: "del", text: older },
      { kind: "add", text: newer },
    ]);
  });
});
```

- [ ] **Step 2: Run to see it fail**

Run: `pnpm --filter @mind-imprint/lite-web exec vitest run src/writings/versionDiff.test.ts`
Expected: FAIL — `Failed to resolve import "./versionDiff"`.

- [ ] **Step 3: Implement the diff**

Create `apps/lite-web/src/writings/versionDiff.ts`:

```ts
import { splitParagraphs } from "../reports/ArticleView";

/**
 * Version diff for the finished-writing page (与当前版本对比).
 *
 * Two levels: a longest-common-subsequence over paragraphs, then, inside each
 * replaced paragraph, a longest-common-subsequence over characters. A run of
 * deleted paragraphs followed by added ones is paired in order; unpaired
 * paragraphs stay whole additions or deletions.
 */

export type DiffPart = { kind: "same" | "add" | "del"; text: string };

export type ParagraphDiff = { kind: "same" | "add" | "del"; text: string } | { kind: "change"; parts: DiffPart[] };

type Op<T> = { op: "same" | "add" | "del"; value: T };

/** Above this many table cells the character diff is skipped: 500 × 500. */
const MAX_CELLS = 250_000;

function lcsOps<T>(a: readonly T[], b: readonly T[]): Op<T>[] {
  const n = a.length;
  const m = b.length;
  const width = m + 1;
  const dp = new Uint32Array((n + 1) * width);
  const at = (i: number, j: number): number => dp[i * width + j] ?? 0;
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i * width + j] = a[i] === b[j] ? at(i + 1, j + 1) + 1 : Math.max(at(i + 1, j), at(i, j + 1));
    }
  }
  const out: Op<T>[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ op: "same", value: a[i] as T });
      i++;
      j++;
    } else if (at(i + 1, j) >= at(i, j + 1)) {
      // Deletions first, so a replaced run reads as del… then add….
      out.push({ op: "del", value: a[i] as T });
      i++;
    } else {
      out.push({ op: "add", value: b[j] as T });
      j++;
    }
  }
  while (i < n) out.push({ op: "del", value: a[i++] as T });
  while (j < m) out.push({ op: "add", value: b[j++] as T });
  return out;
}

/** Character-level marks inside one paragraph, adjacent parts merged. */
export function diffChars(older: string, newer: string): DiffPart[] {
  const a = Array.from(older);
  const b = Array.from(newer);
  if (a.length * b.length > MAX_CELLS) {
    const parts: DiffPart[] = [];
    if (older) parts.push({ kind: "del", text: older });
    if (newer) parts.push({ kind: "add", text: newer });
    return parts;
  }
  const parts: DiffPart[] = [];
  for (const { op, value } of lcsOps(a, b)) {
    const last = parts[parts.length - 1];
    if (last && last.kind === op) last.text += value;
    else parts.push({ kind: op, text: value });
  }
  return parts;
}

export function diffVersions(older: string, newer: string): ParagraphDiff[] {
  const ops = lcsOps(splitParagraphs(older), splitParagraphs(newer));
  const out: ParagraphDiff[] = [];
  let dels: string[] = [];
  let adds: string[] = [];
  const flush = () => {
    const paired = Math.min(dels.length, adds.length);
    for (let k = 0; k < paired; k++) out.push({ kind: "change", parts: diffChars(dels[k] ?? "", adds[k] ?? "") });
    for (const text of dels.slice(paired)) out.push({ kind: "del", text });
    for (const text of adds.slice(paired)) out.push({ kind: "add", text });
    dels = [];
    adds = [];
  };
  for (const { op, value } of ops) {
    if (op === "same") {
      flush();
      out.push({ kind: "same", text: value });
    } else if (op === "del") {
      dels.push(value);
    } else {
      adds.push(value);
    }
  }
  flush();
  return out;
}
```

- [ ] **Step 4: Run the diff tests**

Run: `pnpm --filter @mind-imprint/lite-web exec vitest run src/writings/versionDiff.test.ts`
Expected: PASS (9 tests).

- [ ] **Step 5: Write the failing normalizer tests**

Append to `apps/lite-web/src/api/assignments.test.ts` (add the imports to the existing import from `./assignments` if not present: `normalizeRecipientDTO`, `normalizeAssignmentForAtom`, `normalizeInboxResponse`):

```ts
describe("return fields", () => {
  it("keeps returned and resubmitted statuses and the return columns on a recipient", () => {
    const r = normalizeRecipientDTO({
      userId: "u",
      status: "returned",
      statusLabel: "已退回",
      returnedAt: "2026-09-15T10:00:00+08:00",
      returnDueAt: "2026-09-18T22:00:00+08:00",
      returnNote: "请补充第二段的论据",
      versionCount: 2,
    });
    expect([r.status, r.returnedAt, r.returnDueAt, r.returnNote, r.versionCount]).toEqual([
      "returned",
      "2026-09-15T10:00:00+08:00",
      "2026-09-18T22:00:00+08:00",
      "请补充第二段的论据",
      2,
    ]);
    expect(normalizeRecipientDTO({ status: "resubmitted" }).status).toBe("resubmitted");
    expect(normalizeRecipientDTO({}).versionCount).toBe(0);
  });

  it("reads return fields on for-atom, with safe defaults from an older server", () => {
    expect(normalizeAssignmentForAtom({ id: "a", title: "t", dueAt: "d" })).toEqual({
      id: "a",
      kind: "writing",
      title: "t",
      dueAt: "d",
      returnedAt: null,
      returnDueAt: null,
      returnNote: null,
      resubmitted: false,
    });
    expect(normalizeAssignmentForAtom({ id: "a", kind: "reading", returnedAt: "x", resubmitted: true })?.resubmitted).toBe(true);
  });

  it("carries the return deadline and note into inbox items", () => {
    const { items } = normalizeInboxResponse({
      items: [{ type: "assignment", id: "a", status: "returned", returnDueAt: "2026-09-18T22:00:00+08:00", returnNote: "n" }],
    });
    expect([items[0]?.status, items[0]?.returnDueAt, items[0]?.returnNote]).toEqual(["returned", "2026-09-18T22:00:00+08:00", "n"]);
  });
});
```

`kind` defaults to `"writing"` on for-atom because the only caller that needs it is a writing page and an older server omits the field.

- [ ] **Step 6: Run to see them fail**

Run: `pnpm --filter @mind-imprint/lite-web exec vitest run src/api/assignments.test.ts`
Expected: FAIL — `returned` normalised to `not_started`, and missing fields.

- [ ] **Step 7: Implement the types and clients**

`apps/lite-web/src/shared/deadline.ts` — replace the status type and labels:

```ts
// The wire statuses `liteassign.StatusWithReturn` (apps/api/internal/liteassign/status.go)
// derives — never stored, always computed from started/finished/dueAt/return/now.
export type AssignmentStatus =
  | "not_started"
  | "in_progress"
  | "done"
  | "done_late"
  | "overdue"
  | "returned"
  | "resubmitted";

/** UI copy per AGENTS.md 界面文案 rule 4 (已/待/中 pairs, not folksy prose) —
 * kept in lockstep with `liteassign.StatusLabel` on the Go side. */
export const STATUS_LABEL: Record<AssignmentStatus, string> = {
  not_started: "未开始",
  in_progress: "进行中",
  done: "已完成",
  done_late: "逾期完成",
  overdue: "已逾期",
  returned: "已退回",
  resubmitted: "已重新提交",
};
```

`apps/lite-web/src/teacher/assignmentLogic.ts` — replace `STATUS_ORDER` and `STATUS_HUE`:

```ts
export const STATUS_ORDER: readonly AssignmentStatus[] = [
  "not_started",
  "in_progress",
  "done",
  "done_late",
  "overdue",
  "returned",
  "resubmitted",
];

const STATUS_HUE: Record<AssignmentStatus, string> = {
  not_started: "var(--mk-muted)",
  in_progress: "var(--mk-accent-500)",
  done: "var(--mk-success)",
  done_late: "var(--mk-warning)",
  overdue: "var(--mk-danger)",
  returned: "var(--mk-warning)",
  resubmitted: "var(--mk-success)",
};
```

`apps/lite-web/src/api/assignments.ts`:
- `KNOWN_STATUSES`: append `"returned", "resubmitted"`.
- `RecipientDTO`: add `returnedAt: string | null; returnDueAt: string | null; returnNote: string | null; versionCount: number;`
- `AssignmentInboxItem`: add `returnDueAt: string | null; returnNote: string | null;`
- Replace `AssignmentForAtom`:

```ts
export interface AssignmentForAtom {
  id: string;
  kind: AssignmentKind;
  title: string;
  dueAt: string;
  /** Set when the teacher returned this writing (退回修改). */
  returnedAt: string | null;
  returnDueAt: string | null;
  returnNote: string | null;
  /** A version was submitted after the return. */
  resubmitted: boolean;
}

export interface ReturnRecipientInput {
  /** RFC3339, in the future. */
  dueAt: string;
  note?: string;
}
```

- Add a helper next to `s`/`arr`/`obj`:

```ts
const nullableString = (v: unknown): string | null => (typeof v === "string" ? v : null);
```

- In `normalizeRecipientDTO`, add:

```ts
    returnedAt: nullableString(raw.returnedAt),
    returnDueAt: nullableString(raw.returnDueAt),
    returnNote: nullableString(raw.returnNote),
    versionCount: typeof raw.versionCount === "number" ? raw.versionCount : 0,
```

- In `normalizeInboxItem`, add:

```ts
    returnDueAt: nullableString(raw.returnDueAt),
    returnNote: nullableString(raw.returnNote),
```

- Replace `normalizeAssignmentForAtom`:

```ts
export function normalizeAssignmentForAtom(raw: unknown): AssignmentForAtom | null {
  const r = obj(raw);
  if (typeof r.id !== "string") return null;
  return {
    id: r.id,
    kind: r.kind === "reading" || r.kind === "project" ? r.kind : "writing",
    title: s(r.title),
    dueAt: s(r.dueAt),
    returnedAt: nullableString(r.returnedAt),
    returnDueAt: nullableString(r.returnDueAt),
    returnNote: nullableString(r.returnNote),
    resubmitted: r.resubmitted === true,
  };
}
```

- Append to the teacher client section:

```ts
/** 退回修改: POST …/assignments/{aid}/recipients/{userId}/return. */
export async function returnRecipient(aid: string, userId: string, input: ReturnRecipientInput): Promise<RecipientDTO> {
  const r = await apiFetch<{ recipient: unknown }>(
    `${teacherBase}/assignments/${encodeURIComponent(aid)}/recipients/${encodeURIComponent(userId)}/return`,
    { method: "POST", body: JSON.stringify(input) },
  );
  return normalizeRecipientDTO(obj(r.recipient));
}
```

`apps/lite-web/src/api/writings.ts`:
- Add to `Writing` after `finishedAt`:

```ts
  /** Set while she edits a finished writing again. `status` stays
   *  "finished" the whole time (a homework stays 已提交). */
  revisingAt?: string | null;
```

- Replace `isWritingFinished` and add `isRevising` after it:

```ts
export function isWritingFinished(w: Pick<Writing, "status" | "finishedAt">): boolean {
  return w.status === "finished" || Boolean(w.finishedAt);
}

/** She reopened a finished writing to edit it. */
export function isRevising(w: Pick<Writing, "revisingAt">): boolean {
  return typeof w.revisingAt === "string" && w.revisingAt !== "";
}
```

- Append:

```ts
export interface WritingVersionSummary {
  number: number;
  title: string;
  wordCount: number;
  submittedAt: string;
}

export interface WritingVersion extends WritingVersionSummary {
  body: string;
}

export interface WritingVersionList {
  /** Newest first. */
  versions: WritingVersionSummary[];
  /** A submitted homework past its deadline: 修改 is refused (403 writing_locked). */
  locked: boolean;
  lockReason: string | null;
}

function normalizeVersionSummary(raw: unknown): WritingVersionSummary {
  const r = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
  return {
    number: typeof r.number === "number" ? r.number : 0,
    title: typeof r.title === "string" ? r.title : "",
    wordCount: typeof r.wordCount === "number" ? r.wordCount : 0,
    submittedAt: typeof r.submittedAt === "string" ? r.submittedAt : "",
  };
}

export function normalizeWritingVersionList(raw: unknown): WritingVersionList {
  const r = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
  return {
    versions: (Array.isArray(r.versions) ? r.versions : []).map(normalizeVersionSummary).filter((v) => v.number > 0),
    locked: r.locked === true,
    lockReason: typeof r.lockReason === "string" ? r.lockReason : null,
  };
}

/** GET /api/v1/writings/{id}/versions */
export async function listWritingVersions(id: string): Promise<WritingVersionList> {
  return normalizeWritingVersionList(await apiFetch<unknown>(`/api/v1/writings/${encodeURIComponent(id)}/versions`));
}

/** GET /api/v1/writings/{id}/versions/{n} */
export async function getWritingVersion(id: string, n: number): Promise<WritingVersion> {
  const raw = await apiFetch<Record<string, unknown>>(`/api/v1/writings/${encodeURIComponent(id)}/versions/${n}`);
  return { ...normalizeVersionSummary(raw), body: typeof raw.body === "string" ? raw.body : "" };
}

/** POST /api/v1/writings/{id}/revise — 修改. 403 writing_locked when locked. */
export async function reviseWriting(id: string): Promise<Writing> {
  return apiFetch<Writing>(`/api/v1/writings/${encodeURIComponent(id)}/revise`, { method: "POST" });
}

/** POST /api/v1/writings/{id}/revise/discard — 放弃修改. */
export async function discardWritingRevision(id: string): Promise<Writing> {
  return apiFetch<Writing>(`/api/v1/writings/${encodeURIComponent(id)}/revise/discard`, { method: "POST" });
}
```

- [ ] **Step 8: Run tests and typecheck**

Run: `pnpm --filter @mind-imprint/lite-web exec vitest run src/api/assignments.test.ts src/writings/versionDiff.test.ts src/teacher/assignmentLogic.test.ts src/inbox/inboxLogic.test.ts`
Expected: PASS.

Two typed test fixtures must gain the new required fields, or typecheck fails:

In `apps/lite-web/src/teacher/assignmentLogic.test.ts`, the `recipient()` helper's object becomes:

```ts
  return {
    userId: "u1",
    displayName: "Phoebe",
    avatarColor: "",
    status: "not_started",
    statusLabel: "未开始",
    atomId: null,
    startedAt: null,
    finishedAt: null,
    seenAt: null,
    returnedAt: null,
    returnDueAt: null,
    returnNote: null,
    versionCount: 0,
    ...over,
  };
```

In `apps/lite-web/src/inbox/inboxLogic.test.ts`, add to the `item()` helper's object, before `...over`:

```ts
    returnDueAt: null,
    returnNote: null,
```

(`inboxItem` in `assignments.test.ts` is an untyped `Record<string, unknown>` and needs no change.)

Run: `pnpm --filter @mind-imprint/lite-web typecheck`
Expected: no errors. Any other `Record<AssignmentStatus, …>` literal, or a typed `RecipientDTO` / `AssignmentInboxItem` / `AssignmentForAtom` literal, reported here gets the new keys with `null` / `0` / `false`.

- [ ] **Step 9: Commit**

```bash
git add apps/lite-web/src/api/writings.ts apps/lite-web/src/api/assignments.ts apps/lite-web/src/api/assignments.test.ts \
  apps/lite-web/src/shared/deadline.ts apps/lite-web/src/teacher/assignmentLogic.ts \
  apps/lite-web/src/teacher/assignmentLogic.test.ts apps/lite-web/src/inbox/inboxLogic.test.ts \
  apps/lite-web/src/writings/versionDiff.ts apps/lite-web/src/writings/versionDiff.test.ts
git commit -m "feat(lite-web): version and return clients, paragraph diff

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: The wide finished page

**Files:**
- Create: `apps/lite-web/src/writings/finishedWriting.ts`, `apps/lite-web/src/writings/finishedWriting.test.ts`
- Create: `apps/lite-web/src/writings/FinishedWritingPage.tsx`
- Modify: `apps/lite-web/src/writings/WritingRoomHost.tsx` (load effect, finished branch; delete `FinishedWritingPanel`)

**Interfaces:**
- Consumes: Task 9 `listWritingVersions`, `getWritingVersion`, `reviseWriting`, `isRevising`, `getAssignmentForAtom`, `diffVersions`; `splitParagraphs` (`reports/ArticleView.tsx`); `formatDeadline` (`shared/deadline.ts`); `wordUnit` (`writings/wordUnit.ts`); `tintedChipStyle` (`teacher/assignmentLogic.ts`).
- Produces:
  - `finishedWriting.ts`: `type FinishedChip = "已完成" | "已提交" | "已退回" | "已锁定"`; `finishedChip(input: { assignment: AssignmentForAtom | null; locked: boolean }): FinishedChip`; `isReturnOpen(a: AssignmentForAtom): boolean`; `effectiveDueAt(a: AssignmentForAtom): string`; `versionLine(v: WritingVersionSummary, lang: string): string`; `chipHue(chip: FinishedChip): string`
  - `FinishedWritingPage` props `{ writing: Writing; onBack: () => void; onRevise: () => Promise<void> }`

- [ ] **Step 1: Write the failing tests**

Create `apps/lite-web/src/writings/finishedWriting.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { AssignmentForAtom } from "../api/assignments";
import { effectiveDueAt, finishedChip, isReturnOpen, versionLine } from "./finishedWriting";

function homework(over: Partial<AssignmentForAtom> = {}): AssignmentForAtom {
  return {
    id: "a",
    kind: "writing",
    title: "雨",
    dueAt: "2026-09-20T14:00:00Z",
    returnedAt: null,
    returnDueAt: null,
    returnNote: null,
    resubmitted: false,
    ...over,
  };
}

describe("finishedChip", () => {
  it("is 已完成 for a writing that is not homework", () =>
    expect(finishedChip({ assignment: null, locked: false })).toBe("已完成"));
  it("is 已提交 for submitted homework", () => expect(finishedChip({ assignment: homework(), locked: false })).toBe("已提交"));
  it("is 已退回 while a return is open", () =>
    expect(finishedChip({ assignment: homework({ returnedAt: "r", returnDueAt: "d" }), locked: false })).toBe("已退回"));
  it("is back to 已提交 once she resubmits", () =>
    expect(finishedChip({ assignment: homework({ returnedAt: "r", returnDueAt: "d", resubmitted: true }), locked: false })).toBe(
      "已提交",
    ));
  it("is 已锁定 whenever locked, returned or not", () => {
    expect(finishedChip({ assignment: homework(), locked: true })).toBe("已锁定");
    expect(finishedChip({ assignment: homework({ returnedAt: "r", returnDueAt: "d" }), locked: true })).toBe("已锁定");
  });
});

describe("isReturnOpen / effectiveDueAt", () => {
  it("uses the return deadline once returned", () => {
    const a = homework({ returnedAt: "2026-09-21T01:00:00Z", returnDueAt: "2026-09-23T14:00:00Z" });
    expect(isReturnOpen(a)).toBe(true);
    expect(effectiveDueAt(a)).toBe("2026-09-23T14:00:00Z");
  });
  it("keeps the return deadline after a resubmission", () =>
    expect(effectiveDueAt(homework({ returnedAt: "r", returnDueAt: "2026-09-23T14:00:00Z", resubmitted: true }))).toBe(
      "2026-09-23T14:00:00Z",
    ));
  it("uses the assignment deadline when never returned", () => {
    expect(isReturnOpen(homework())).toBe(false);
    expect(effectiveDueAt(homework())).toBe("2026-09-20T14:00:00Z");
  });
});

describe("versionLine", () => {
  it("formats number, Beijing time and word count", () =>
    expect(versionLine({ number: 3, title: "t", wordCount: 812, submittedAt: "2026-09-15T06:20:00Z" }, "zh")).toBe(
      "v3 · 9月15日 14:20 · 812 字",
    ));
  it("counts words for an English piece", () =>
    expect(versionLine({ number: 1, title: "t", wordCount: 240, submittedAt: "2026-09-15T06:20:00Z" }, "en")).toBe(
      "v1 · 9月15日 14:20 · 240 词",
    ));
});
```

- [ ] **Step 2: Run to see it fail**

Run: `pnpm --filter @mind-imprint/lite-web exec vitest run src/writings/finishedWriting.test.ts`
Expected: FAIL — `Failed to resolve import "./finishedWriting"`.

- [ ] **Step 3: Implement the pure functions**

Create `apps/lite-web/src/writings/finishedWriting.ts`:

```ts
// writings/finishedWriting.ts — pure rules behind the finished-writing page:
// which chip it shows, which deadline applies, and how a version is listed.

import type { AssignmentForAtom } from "../api/assignments";
import type { WritingVersionSummary } from "../api/writings";
import { formatDeadline } from "../shared/deadline";
import { wordUnit } from "./wordUnit";

export type FinishedChip = "已完成" | "已提交" | "已退回" | "已锁定";

/** The teacher returned it and she has not submitted since. */
export function isReturnOpen(a: AssignmentForAtom): boolean {
  return a.returnedAt !== null && !a.resubmitted;
}

/** Same rule as liteassign.EffectiveDue: the return deadline once returned. */
export function effectiveDueAt(a: AssignmentForAtom): string {
  return a.returnedAt !== null && a.returnDueAt !== null ? a.returnDueAt : a.dueAt;
}

export function finishedChip(input: { assignment: AssignmentForAtom | null; locked: boolean }): FinishedChip {
  if (input.locked) return "已锁定";
  if (!input.assignment) return "已完成";
  if (isReturnOpen(input.assignment)) return "已退回";
  return "已提交";
}

export function chipHue(chip: FinishedChip): string {
  switch (chip) {
    case "已退回":
      return "var(--mk-warning)";
    case "已锁定":
      return "var(--mk-muted)";
    default:
      return "var(--mk-success)";
  }
}

/** `v3 · 9月15日 14:20 · 812 字` — Beijing time, 词 for an English piece. */
export function versionLine(v: WritingVersionSummary, lang: string): string {
  return `v${v.number} · ${formatDeadline(v.submittedAt)} · ${v.wordCount} ${wordUnit(lang)}`;
}
```

- [ ] **Step 4: Run the tests**

Run: `pnpm --filter @mind-imprint/lite-web exec vitest run src/writings/finishedWriting.test.ts`
Expected: PASS (10 tests).

- [ ] **Step 5: Build the page**

Create `apps/lite-web/src/writings/FinishedWritingPage.tsx`:

```tsx
import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { Button } from "@/ui";
import { getAssignmentForAtom, type AssignmentForAtom } from "../api/assignments";
import { apiErrorText } from "../api/errorText";
import { getWritingVersion, listWritingVersions, type Writing, type WritingVersion, type WritingVersionList } from "../api/writings";
import { ReportPanel } from "../reports/ReportPanel";
import { splitParagraphs } from "../reports/ArticleView";
import { formatDeadline } from "../shared/deadline";
import { useAlive } from "../shared/useAlive";
import { tintedChipStyle } from "../teacher/assignmentLogic";
import { chipHue, effectiveDueAt, finishedChip, versionLine } from "./finishedWriting";
import { diffVersions, type ParagraphDiff } from "./versionDiff";

/**
 * FinishedWritingPage — `/writings/:id` once a writing is finished and not
 * being revised.
 *
 * Header: title, chip, homework line, 修改 and 报告. Left column (44rem):
 * the version being viewed, the latest by default. Right rail: 版本, and
 * 与当前版本对比 when an older version is selected. Below 1024px the rail
 * follows the text in one column. 报告 swaps the columns for ReportPanel.
 */
export function FinishedWritingPage({
  writing,
  onBack,
  onRevise,
}: {
  writing: Writing;
  onBack: () => void;
  /** Starts revising and reopens the room. Throws on failure. */
  onRevise: () => Promise<void>;
}) {
  const alive = useAlive();
  const [list, setList] = useState<WritingVersionList | null>(null);
  const [assignment, setAssignment] = useState<AssignmentForAtom | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [bodies, setBodies] = useState<Record<number, WritingVersion>>({});
  const requested = useRef<Set<number>>(new Set());
  const [selected, setSelected] = useState<number | null>(null);
  const [compare, setCompare] = useState(false);
  const [showReport, setShowReport] = useState(false);
  const [revising, setRevising] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    setList(null);
    setLoadError(null);
    setBodies({});
    requested.current = new Set();
    setSelected(null);
    setCompare(false);
    listWritingVersions(writing.id)
      .then((l) => {
        if (!alive.current) return;
        setList(l);
        setSelected(l.versions[0]?.number ?? null);
      })
      .catch((e: unknown) => {
        if (alive.current) setLoadError(apiErrorText(e));
      });
    getAssignmentForAtom(writing.id)
      .then((a) => {
        if (alive.current) setAssignment(a);
      })
      .catch(() => {
        if (alive.current) setAssignment(null);
      });
  }, [writing.id, alive]);

  const latest = list?.versions[0]?.number ?? null;

  useEffect(() => {
    for (const n of [selected, latest]) {
      if (n === null || requested.current.has(n)) continue;
      requested.current.add(n);
      getWritingVersion(writing.id, n)
        .then((v) => {
          if (alive.current) setBodies((b) => ({ ...b, [n]: v }));
        })
        .catch((e: unknown) => {
          if (alive.current) setLoadError(apiErrorText(e));
        });
    }
  }, [selected, latest, writing.id, alive]);

  const shown = selected !== null ? bodies[selected] : undefined;
  const latestBody = latest !== null ? bodies[latest] : undefined;
  const diff = useMemo(
    () => (compare && shown && latestBody && shown.number !== latestBody.number ? diffVersions(shown.body, latestBody.body) : null),
    [compare, shown, latestBody],
  );

  const locked = list?.locked ?? false;
  const chip = finishedChip({ assignment, locked });

  async function revise() {
    if (revising) return;
    setRevising(true);
    setActionError(null);
    try {
      await onRevise();
    } catch (e) {
      if (alive.current) setActionError(`修改失败：${apiErrorText(e)}`);
    } finally {
      if (alive.current) setRevising(false);
    }
  }

  const selectedSummary = list?.versions.find((v) => v.number === selected) ?? null;

  return (
    <div className="flex w-full flex-col pb-14">
      <header className="mk-rp-measure flex flex-col gap-3 pt-8">
        <div className="flex flex-wrap items-center gap-3">
          <Button variant="secondary" onClick={onBack}>
            回到写作
          </Button>
          <span className="rounded-mk-full px-2.5 py-1 text-mk-label font-semibold" style={tintedChipStyle(chipHue(chip))}>
            {chip}
          </span>
          <div className="flex flex-wrap gap-2 sm:ml-auto">
            <Button variant="secondary" onClick={() => void revise()} disabled={revising || list === null}>
              修改
            </Button>
            <Button variant={showReport ? "primary" : "secondary"} onClick={() => setShowReport((v) => !v)}>
              {showReport ? "正文" : "报告"}
            </Button>
          </div>
        </div>
        <h1 className="font-mk-piece text-mk-report-title text-mk-ink">{writing.title}</h1>
        {assignment && (
          <p className="text-mk-small text-mk-muted">
            作业 · {assignment.title} · 截止 {formatDeadline(effectiveDueAt(assignment))}
          </p>
        )}
        {actionError && (
          <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
            {actionError}
          </p>
        )}
      </header>

      {showReport ? (
        <ReportPanel kind="writing" atomId={writing.id} />
      ) : (
        <div className="mk-rp-measure mt-8 grid grid-cols-1 gap-10 lg:grid-cols-[minmax(0,44rem)_minmax(16rem,1fr)]">
          <article className="min-w-0">
            {selectedSummary && selected !== latest && (
              <p className="mb-4 text-mk-small text-mk-muted">{versionLine(selectedSummary, writing.lang)}</p>
            )}
            <VersionText version={shown} diff={diff} />
          </article>

          <aside className="flex min-w-0 flex-col gap-3">
            <h2 className="text-mk-small font-semibold text-mk-secondary">版本</h2>
            {loadError && (
              <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
                加载失败：{loadError}
              </p>
            )}
            {list === null && !loadError && <p className="text-mk-small text-mk-muted">加载中…</p>}
            <ul className="flex flex-col gap-1">
              {list?.versions.map((v) => (
                <li key={v.number}>
                  <button
                    type="button"
                    aria-pressed={v.number === selected}
                    onClick={() => {
                      setSelected(v.number);
                      if (v.number === latest) setCompare(false);
                    }}
                    className="w-full rounded-mk-sm px-3 py-2 text-left text-mk-small text-mk-ink transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                    style={v.number === selected ? SELECTED_STYLE : undefined}
                  >
                    {versionLine(v, writing.lang)}
                  </button>
                </li>
              ))}
            </ul>
            {selected !== null && latest !== null && selected !== latest && (
              <div className="flex flex-col gap-2">
                <Button variant={compare ? "primary" : "secondary"} size="sm" onClick={() => setCompare((c) => !c)}>
                  与当前版本对比
                </Button>
                {compare && (
                  <p className="flex flex-wrap gap-3 text-mk-small text-mk-muted">
                    <ins style={ADD_STYLE}>新增</ins>
                    <del style={DEL_STYLE}>删除</del>
                  </p>
                )}
              </div>
            )}
          </aside>
        </div>
      )}
    </div>
  );
}

const SELECTED_STYLE: CSSProperties = { background: "color-mix(in srgb, var(--mk-accent-500) 12%, var(--mk-surface))" };
const ADD_STYLE: CSSProperties = {
  background: "color-mix(in srgb, var(--mk-success) 20%, transparent)",
  textDecoration: "none",
};
const DEL_STYLE: CSSProperties = {
  background: "color-mix(in srgb, var(--mk-danger) 14%, transparent)",
  textDecoration: "line-through",
};
const PIECE_CLS = "whitespace-pre-wrap font-mk-piece text-mk-report-piece text-mk-ink";

function VersionText({ version, diff }: { version: WritingVersion | undefined; diff: ParagraphDiff[] | null }) {
  if (!version) return <p className="text-mk-body text-mk-muted">加载中…</p>;
  if (diff) {
    return (
      <div className="flex flex-col gap-6">
        {diff.map((d, i) => (
          <DiffParagraph key={i} d={d} />
        ))}
      </div>
    );
  }
  const paragraphs = splitParagraphs(version.body);
  if (paragraphs.length === 0) return <p className="text-mk-body text-mk-muted">这一版没有正文。</p>;
  return (
    <div className="flex flex-col gap-6">
      {paragraphs.map((p, i) => (
        <p key={i} className={PIECE_CLS}>
          {p}
        </p>
      ))}
    </div>
  );
}

function DiffParagraph({ d }: { d: ParagraphDiff }) {
  if (d.kind === "same") return <p className={PIECE_CLS}>{d.text}</p>;
  if (d.kind === "add")
    return (
      <p className={PIECE_CLS}>
        <ins style={ADD_STYLE}>{d.text}</ins>
      </p>
    );
  if (d.kind === "del")
    return (
      <p className={PIECE_CLS}>
        <del style={DEL_STYLE}>{d.text}</del>
      </p>
    );
  return (
    <p className={PIECE_CLS}>
      {d.parts.map((part, i) =>
        part.kind === "same" ? (
          <span key={i}>{part.text}</span>
        ) : part.kind === "add" ? (
          <ins key={i} style={ADD_STYLE}>
            {part.text}
          </ins>
        ) : (
          <del key={i} style={DEL_STYLE}>
            {part.text}
          </del>
        ),
      )}
    </p>
  );
}
```

- [ ] **Step 6: Wire it into the host**

In `apps/lite-web/src/writings/WritingRoomHost.tsx`:

1. Imports: change the writings import to
   `import { getWriting, isAssignedWriting, isRevising, isWritingFinished, reviseWriting, type Writing } from "../api/writings";`
   and add `import { FinishedWritingPage } from "./FinishedWritingPage";`. Remove `ReportPanel` and `Button` imports if nothing else in the file uses them (run typecheck to confirm).
2. Add a reload counter next to the other state: `const [reloadNonce, setReloadNonce] = useState(0);` and `const reload = useCallback(() => setReloadNonce((n) => n + 1), []);`
3. In the load effect, change the finished check to `if (isWritingFinished(writing) && !isRevising(writing)) {` and change the dependency array from `[writingId]` to `[writingId, reloadNonce]`.
4. Replace the `if (state.phase === "finished") { … }` block with:

```tsx
  if (state.phase === "finished") {
    return (
      <FinishedWritingPage
        writing={state.writing}
        onBack={() => navigate(liteRoutePath({ tab: "writings" }))}
        onRevise={async () => {
          await reviseWriting(writingId);
          reload();
        }}
      />
    );
  }
```

5. Delete the `FinishedWritingPanel` function at the bottom of the file.

The heartbeat line `useHeartbeat("writing", writingId, state.phase === "ready")` stays: a revising writing loads into `ready`. The server's heartbeat treats a finished writing as a no-op, so revision minutes are not counted; that is accepted for Part A.

- [ ] **Step 7: Typecheck and run the lite-web tests**

Run: `pnpm --filter @mind-imprint/lite-web typecheck && pnpm --filter @mind-imprint/lite-web test`
Expected: no type errors; all vitest files pass.

- [ ] **Step 8: Commit**

```bash
git add apps/lite-web/src/writings/finishedWriting.ts apps/lite-web/src/writings/finishedWriting.test.ts \
  apps/lite-web/src/writings/FinishedWritingPage.tsx apps/lite-web/src/writings/WritingRoomHost.tsx
git commit -m "feat(lite-web): wide finished writing page with versions and diff

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: Revising strip, locked state, 已退回 on the page and the 作业 strip

**Files:**
- Modify: `apps/lite-web/src/writings/finishedWriting.ts`, `apps/lite-web/src/writings/finishedWriting.test.ts`
- Create: `apps/lite-web/src/writings/RevisingStrip.tsx`
- Modify: `apps/lite-web/src/writings/WritingRoomHost.tsx`
- Modify: `apps/lite-web/src/writings/FinishedWritingPage.tsx`
- Modify: `apps/lite-web/src/inbox/inboxLogic.ts`, `apps/lite-web/src/inbox/inboxLogic.test.ts`, `apps/lite-web/src/inbox/AssignmentStrip.tsx`

**Interfaces:**
- Consumes: Task 9 `listWritingVersions`, `discardWritingRevision`, `isRevising`, `isWritingFinished`; Task 10 `effectiveDueAt`, `isReturnOpen`, `FinishedWritingPage`, `reload` in the host.
- Produces:
  - `finishedWriting.ts`: `LOCKED_TEXT = "已过截止时间，作业已锁定"`; `revisingStripText(n: number): string`; `discardConfirmText(n: number): string`; `returnedLine(a: AssignmentForAtom): string`; `showFinishedPage(w: Pick<Writing, "status" | "finishedAt" | "revisingAt">, locked: boolean): boolean`
  - `inboxLogic.ts`: `OPEN_STATUSES` includes `"returned"`; `stripDueAt(item: Pick<AssignmentInboxItem, "status" | "dueAt" | "returnDueAt">): string`
  - `RevisingStrip` props `{ writingId: string; onDiscarded: () => void }`

- [ ] **Step 1: Write the failing tests**

Append to `apps/lite-web/src/writings/finishedWriting.test.ts` (extend the import from `./finishedWriting` with `discardConfirmText, LOCKED_TEXT, returnedLine, revisingStripText, showFinishedPage`):

```ts
describe("revising and locked copy", () => {
  it("names the submitted version on the strip", () =>
    expect(revisingStripText(2)).toBe("已提交 v2 · 修改完成后请再次点击「完成」提交新版本"));
  it("says what 放弃修改 restores", () => expect(discardConfirmText(2)).toBe("正文与标题将恢复为 v2"));
  it("uses the spec's lock sentence", () => expect(LOCKED_TEXT).toBe("已过截止时间，作业已锁定"));
  it("shows the return deadline in Beijing time", () =>
    expect(returnedLine(homework({ returnedAt: "r", returnDueAt: "2026-09-18T14:00:00Z" }))).toBe("已退回 · 截止 9月18日 22:00"));
});

describe("showFinishedPage", () => {
  const finished = { status: "finished", finishedAt: "2026-09-15T06:00:00Z" };
  it("shows the page for a finished writing that is not being revised", () =>
    expect(showFinishedPage({ ...finished, revisingAt: null }, false)).toBe(true));
  it("opens the room while revising", () =>
    expect(showFinishedPage({ ...finished, revisingAt: "2026-09-15T07:00:00Z" }, false)).toBe(false));
  it("keeps the page when revising but locked", () =>
    expect(showFinishedPage({ ...finished, revisingAt: "2026-09-15T07:00:00Z" }, true)).toBe(true));
  it("never shows it for an open writing", () =>
    expect(showFinishedPage({ status: "active", finishedAt: null, revisingAt: null }, false)).toBe(false));
});
```

In `apps/lite-web/src/inbox/inboxLogic.test.ts`, add `stripDueAt` to the import from `./inboxLogic` and append:

```ts
describe("returned homework on the strip", () => {
  it("lists a returned writing as open", () =>
    expect(openItemsForKind([item({ id: "r", kind: "writing", status: "returned", atomId: "x" })], "writing").map((i) => i.id)).toEqual([
      "r",
    ]));
  it("does not list a resubmitted writing", () =>
    expect(openItemsForKind([item({ kind: "writing", status: "resubmitted", atomId: "x" })], "writing")).toEqual([]));
  it("shows the return deadline for a returned item", () =>
    expect(stripDueAt({ status: "returned", dueAt: "2026-09-10T14:00:00Z", returnDueAt: "2026-09-18T14:00:00Z" })).toBe(
      "2026-09-18T14:00:00Z",
    ));
  it("shows the assignment deadline otherwise", () =>
    expect(stripDueAt({ status: "in_progress", dueAt: "2026-09-10T14:00:00Z", returnDueAt: null })).toBe("2026-09-10T14:00:00Z"));
});
```

- [ ] **Step 2: Run to see them fail**

Run: `pnpm --filter @mind-imprint/lite-web exec vitest run src/writings/finishedWriting.test.ts src/inbox/inboxLogic.test.ts`
Expected: FAIL — missing exports `revisingStripText`, `stripDueAt`, and `returned` not listed as open.

- [ ] **Step 3: Implement the pure functions**

Append to `apps/lite-web/src/writings/finishedWriting.ts` (extend imports: `import { isRevising, isWritingFinished, type Writing, type WritingVersionSummary } from "../api/writings";`):

```ts
export const LOCKED_TEXT = "已过截止时间，作业已锁定";

export function revisingStripText(n: number): string {
  return `已提交 v${n} · 修改完成后请再次点击「完成」提交新版本`;
}

export function discardConfirmText(n: number): string {
  return `正文与标题将恢复为 v${n}`;
}

export function returnedLine(a: AssignmentForAtom): string {
  return `已退回 · 截止 ${formatDeadline(effectiveDueAt(a))}`;
}

/** Finished page when finished and not revising — or revising but locked,
 *  because a locked writing cannot be edited and shows its latest version. */
export function showFinishedPage(w: Pick<Writing, "status" | "finishedAt" | "revisingAt">, locked: boolean): boolean {
  return isWritingFinished(w) && (!isRevising(w) || locked);
}
```

In `apps/lite-web/src/inbox/inboxLogic.ts`, replace `OPEN_STATUSES` and add `stripDueAt`:

```ts
const OPEN_STATUSES: readonly AssignmentStatus[] = ["not_started", "in_progress", "overdue", "returned"];

/** The deadline a strip row shows: the return deadline for a returned homework. */
export function stripDueAt(item: Pick<AssignmentInboxItem, "status" | "dueAt" | "returnDueAt">): string {
  return item.status === "returned" && item.returnDueAt ? item.returnDueAt : item.dueAt;
}
```

- [ ] **Step 4: Run the tests**

Run: `pnpm --filter @mind-imprint/lite-web exec vitest run src/writings/finishedWriting.test.ts src/inbox/inboxLogic.test.ts`
Expected: PASS.

- [ ] **Step 5: Build the strip**

Create `apps/lite-web/src/writings/RevisingStrip.tsx`:

```tsx
import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { apiErrorText } from "../api/errorText";
import { discardWritingRevision, listWritingVersions } from "../api/writings";
import { useAlive } from "../shared/useAlive";
import { discardConfirmText, revisingStripText } from "./finishedWriting";

/**
 * RevisingStrip — shown in the writing room while she edits a finished
 * writing. Names the version already submitted and offers 放弃修改, which
 * restores the draft and title from that version after one confirmation.
 */
export function RevisingStrip({ writingId, onDiscarded }: { writingId: string; onDiscarded: () => void }) {
  const alive = useAlive();
  const [latest, setLatest] = useState<number | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLatest(null);
    listWritingVersions(writingId)
      .then((l) => {
        if (alive.current) setLatest(l.versions[0]?.number ?? null);
      })
      .catch((e: unknown) => {
        if (alive.current) setError(`加载失败：${apiErrorText(e)}`);
      });
  }, [writingId, alive]);

  async function discard() {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await discardWritingRevision(writingId);
      if (alive.current) onDiscarded();
    } catch (e) {
      if (!alive.current) return;
      setError(`放弃修改失败：${apiErrorText(e)}`);
      setBusy(false);
    }
  }

  if (latest === null && !error) return null;

  return (
    <div className="flex flex-wrap items-center gap-3 rounded-mk-md border border-mk-border bg-mk-surface px-4 py-2.5 text-mk-small text-mk-ink">
      {latest !== null && <span className="font-semibold">{revisingStripText(latest)}</span>}
      {latest !== null && (
        <div className="flex flex-wrap items-center gap-2 sm:ml-auto">
          {confirming ? (
            <>
              <span className="text-mk-muted">{discardConfirmText(latest)}</span>
              <Button variant="danger" size="sm" onClick={() => void discard()} disabled={busy}>
                确认放弃
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setConfirming(false)} disabled={busy}>
                取消
              </Button>
            </>
          ) : (
            <Button variant="secondary" size="sm" onClick={() => setConfirming(true)}>
              放弃修改
            </Button>
          )}
        </div>
      )}
      {error && (
        <p role="alert" className="w-full break-words font-semibold text-mk-danger">
          {error}
        </p>
      )}
    </div>
  );
}
```

- [ ] **Step 6: Wire the host**

In `apps/lite-web/src/writings/WritingRoomHost.tsx`:

1. Add `listWritingVersions` to the `../api/writings` import; add `import { showFinishedPage } from "./finishedWriting";` and `import { RevisingStrip } from "./RevisingStrip";`.
2. In the load effect, replace the finished block with:

```ts
        const writing = await getWriting(writingId);
        if (isWritingFinished(writing)) {
          // Revising but past the deadline: the room would refuse every
          // write, so the page shows the latest version instead.
          const locked = isRevising(writing) ? ((await listWritingVersions(writingId).catch(() => null))?.locked ?? false) : false;
          if (showFinishedPage(writing, locked)) {
            const draft = await getWritingDraft(writingId).catch(() => EMPTY_DRAFT);
            if (!cancelled) setState({ phase: "finished", writing, draft });
            return;
          }
        }
```

3. In the room layout, directly after the closing `</header>` and before `{roomError && (`, add:

```tsx
      {isRevising(writing) && <RevisingStrip writingId={writingId} onDiscarded={reload} />}
```

The 结构 stage renders `PlanningView` full-screen before this layout, so the strip is not shown there. A finished writing is on the 成稿 or 段落 stage in practice; this is accepted for Part A.

- [ ] **Step 7: Locked and returned on the page**

In `apps/lite-web/src/writings/FinishedWritingPage.tsx`:

1. Extend the `./finishedWriting` import with `isReturnOpen, LOCKED_TEXT, returnedLine`.
2. The 修改 button becomes `disabled={locked || revising || list === null}`.
3. After the homework line (`{assignment && (<p …>作业 · …</p>)}`), add:

```tsx
        {locked && (
          <p role="status" className="text-mk-small font-semibold text-mk-danger">
            {LOCKED_TEXT}
          </p>
        )}
        {assignment && !locked && isReturnOpen(assignment) && (
          <div className="flex flex-col gap-1 rounded-mk-md border border-mk-border bg-mk-surface px-4 py-3 text-mk-small">
            <p className="font-semibold text-mk-ink">{returnedLine(assignment)}</p>
            {assignment.returnNote && <p className="whitespace-pre-wrap text-mk-secondary">退回说明：{assignment.returnNote}</p>}
          </div>
        )}
```

- [ ] **Step 8: The 作业 strip**

In `apps/lite-web/src/inbox/AssignmentStrip.tsx`:

1. Change the inboxLogic import to `import { openItemsForKind, startButtonLabel, stripDueAt } from "./inboxLogic";`.
2. Replace the deadline paragraph with:

```tsx
              {item.dueAt && (
                <p className="mt-0.5 text-mk-small text-mk-muted">截止 {formatDeadline(stripDueAt(item))}</p>
              )}
              {item.status === "returned" && item.returnNote && (
                <p className="mt-0.5 line-clamp-2 text-mk-small text-mk-secondary">退回说明：{item.returnNote}</p>
              )}
```

3. Update the component doc comment's list to `(未开始 / 进行中 / 已逾期 / 已退回)`.

- [ ] **Step 9: Typecheck and test**

Run: `pnpm --filter @mind-imprint/lite-web typecheck && pnpm --filter @mind-imprint/lite-web test`
Expected: no type errors; all vitest files pass.

- [ ] **Step 10: Commit**

```bash
git add apps/lite-web/src/writings/finishedWriting.ts apps/lite-web/src/writings/finishedWriting.test.ts \
  apps/lite-web/src/writings/RevisingStrip.tsx apps/lite-web/src/writings/WritingRoomHost.tsx \
  apps/lite-web/src/writings/FinishedWritingPage.tsx apps/lite-web/src/inbox/inboxLogic.ts \
  apps/lite-web/src/inbox/inboxLogic.test.ts apps/lite-web/src/inbox/AssignmentStrip.tsx
git commit -m "feat(lite-web): revising strip, locked page and returned homework display

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: Teacher 退回修改 dialog

**Files:**
- Modify: `apps/lite-web/src/teacher/assignmentLogic.ts`, `apps/lite-web/src/teacher/assignmentLogic.test.ts`
- Create: `apps/lite-web/src/teacher/ReturnDialog.tsx`
- Modify: `apps/lite-web/src/teacher/AssignmentDetailPage.tsx` (recipient table 操作 cell; dialog state)

**Interfaces:**
- Consumes: Task 9 `returnRecipient`, `ReturnRecipientInput`, `RecipientDTO.versionCount`; existing `Built<T>`, `failText`, `beijingInputToISO`, `isoToBeijingInput`.
- Produces:
  - `buildReturnInput(dueInput: string, note: string, nowMs: number): Built<ReturnRecipientInput>`
  - `ReturnDialog` props `{ assignmentId: string; recipient: RecipientDTO; onClose: () => void; onReturned: (r: RecipientDTO) => void }`

- [ ] **Step 1: Write the failing test**

In `apps/lite-web/src/teacher/assignmentLogic.test.ts`, add `buildReturnInput` to the import from `./assignmentLogic` and append:

```ts
describe("buildReturnInput", () => {
  const now = Date.parse("2026-09-15T10:00:00+08:00");

  it("rejects a missing or malformed time", () => {
    expect(buildReturnInput("", "", now)).toEqual({ ok: false, error: "请填写新的截止时间" });
    expect(buildReturnInput("2026-09-18", "", now)).toEqual({ ok: false, error: "请填写新的截止时间" });
  });
  it("rejects a time that is not after now", () =>
    expect(buildReturnInput("2026-09-15T10:00", "", now)).toEqual({ ok: false, error: "新的截止时间需要晚于现在" }));
  it("rejects a note over 500 characters", () =>
    expect(buildReturnInput("2026-09-18T22:00", "字".repeat(501), now)).toEqual({ ok: false, error: "退回说明不超过 500 字" }));
  it("accepts exactly 500 characters", () =>
    expect(buildReturnInput("2026-09-18T22:00", "字".repeat(500), now).ok).toBe(true));
  it("reads the input as Beijing time, trims the note and drops an empty one", () => {
    expect(buildReturnInput("2026-09-18T22:00", "  请补充第二段的论据 ", now)).toEqual({
      ok: true,
      value: { dueAt: "2026-09-18T22:00:00+08:00", note: "请补充第二段的论据" },
    });
    expect(buildReturnInput("2026-09-18T22:00", "   ", now)).toEqual({ ok: true, value: { dueAt: "2026-09-18T22:00:00+08:00" } });
  });
});
```

- [ ] **Step 2: Run to see it fail**

Run: `pnpm --filter @mind-imprint/lite-web exec vitest run src/teacher/assignmentLogic.test.ts`
Expected: FAIL — `buildReturnInput is not a function`.

- [ ] **Step 3: Implement the builder**

In `apps/lite-web/src/teacher/assignmentLogic.ts`, add `ReturnRecipientInput` to the type import from `../api/assignments`, and add after `buildPatchInput`:

```ts
const MAX_RETURN_NOTE = 500;

/** 退回修改 request body. Messages match lite_teacher_return.go. */
export function buildReturnInput(dueInput: string, note: string, nowMs: number): Built<ReturnRecipientInput> {
  const dueAt = beijingInputToISO(dueInput);
  if (!dueAt) return { ok: false, error: "请填写新的截止时间" };
  if (Date.parse(dueAt) <= nowMs) return { ok: false, error: "新的截止时间需要晚于现在" };
  const trimmed = note.trim();
  if (Array.from(trimmed).length > MAX_RETURN_NOTE) return { ok: false, error: "退回说明不超过 500 字" };
  return { ok: true, value: trimmed ? { dueAt, note: trimmed } : { dueAt } };
}
```

- [ ] **Step 4: Run the test**

Run: `pnpm --filter @mind-imprint/lite-web exec vitest run src/teacher/assignmentLogic.test.ts`
Expected: PASS.

- [ ] **Step 5: Build the dialog**

Create `apps/lite-web/src/teacher/ReturnDialog.tsx`:

```tsx
import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { returnRecipient, type RecipientDTO } from "../api/assignments";
import { isoToBeijingInput } from "../shared/deadline";
import { useAlive } from "../shared/useAlive";
import { buildReturnInput, failText } from "./assignmentLogic";

const DEFAULT_EXTENSION_MS = 3 * 24 * 60 * 60 * 1000;

/**
 * ReturnDialog — 退回修改 for one student's submitted writing homework.
 * The student can edit and submit again until the new deadline. Returning
 * again replaces the previous deadline and note.
 */
export function ReturnDialog({
  assignmentId,
  recipient,
  onClose,
  onReturned,
}: {
  assignmentId: string;
  recipient: RecipientDTO;
  onClose: () => void;
  onReturned: (r: RecipientDTO) => void;
}) {
  const alive = useAlive();
  const [dueInput, setDueInput] = useState(() => isoToBeijingInput(new Date(Date.now() + DEFAULT_EXTENSION_MS).toISOString()));
  const [note, setNote] = useState(recipient.returnNote ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy, onClose]);

  async function submit() {
    if (busy) return;
    const built = buildReturnInput(dueInput, note, Date.now());
    if (!built.ok) {
      setError(built.error);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const updated = await returnRecipient(assignmentId, recipient.userId, built.value);
      if (alive.current) onReturned(updated);
    } catch (e) {
      if (!alive.current) return;
      setError(failText("退回", e));
      setBusy(false);
    }
  }

  const inputCls =
    "w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-body text-mk-ink outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:text-mk-muted";

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: "color-mix(in srgb, var(--mk-ink) 42%, transparent)" }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="return-dialog-title"
        className="flex max-h-full w-full max-w-[520px] flex-col gap-5 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-6 shadow-mk-lg"
      >
        <div className="flex flex-col gap-1.5">
          <h2 id="return-dialog-title" className="text-mk-h2 text-mk-ink">
            退回修改
          </h2>
          <p className="text-mk-small text-mk-muted">{recipient.displayName}</p>
          <p className="text-mk-small text-mk-secondary">退回后学生可以修改并重新提交，截止时间以此处为准。</p>
        </div>

        <label className="flex flex-col gap-1.5 text-mk-small text-mk-muted">
          新的截止时间（北京时间）
          <input type="datetime-local" value={dueInput} disabled={busy} onChange={(e) => setDueInput(e.target.value)} className={inputCls} />
        </label>

        <label className="flex flex-col gap-1.5 text-mk-small text-mk-muted">
          退回说明
          <textarea rows={4} maxLength={500} value={note} disabled={busy} onChange={(e) => setNote(e.target.value)} className={inputCls} />
        </label>

        {error && (
          <p className="break-words text-mk-small font-semibold text-mk-danger" role="alert">
            {error}
          </p>
        )}

        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={onClose} disabled={busy}>
            取消
          </Button>
          <Button variant="primary" size="sm" onClick={() => void submit()} disabled={busy}>
            {busy ? "处理中" : "确认退回"}
          </Button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 6: Wire it into the detail page**

In `apps/lite-web/src/teacher/AssignmentDetailPage.tsx`:

1. Add `import { ReturnDialog } from "./ReturnDialog";`.
2. Next to the other state: `const [returning, setReturning] = useState<RecipientDTO | null>(null);`, and add `setReturning(null);` to the effect that resets state on `assignmentId`.
3. Replace the 操作 cell (`<td …>{r.atomId ? (<Button variant="link" …>查看</Button>) : (<span …>—</span>)}</td>`) with:

```tsx
                        <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small">
                          <div className="flex items-center gap-3">
                            {r.atomId ? (
                              <Button variant="link" size="sm" onClick={() => onOpenItem(assignment.classId, r.userId, r.atomId ?? "")}>
                                查看
                              </Button>
                            ) : (
                              <span className="text-mk-muted">—</span>
                            )}
                            {assignment.kind === "writing" && r.versionCount > 0 && (
                              <Button variant="link" size="sm" onClick={() => setReturning(r)}>
                                退回修改
                              </Button>
                            )}
                          </div>
                        </td>
```

4. Just before the closing `</TeacherPage>`, add:

```tsx
      {returning && assignment && (
        <ReturnDialog
          assignmentId={assignment.id}
          recipient={returning}
          onClose={() => setReturning(null)}
          onReturned={() => {
            setReturning(null);
            setNonce((n) => n + 1);
          }}
        />
      )}
```

- [ ] **Step 7: Typecheck and test**

Run: `pnpm --filter @mind-imprint/lite-web typecheck && pnpm --filter @mind-imprint/lite-web test`
Expected: no type errors; all vitest files pass.

- [ ] **Step 8: Commit**

```bash
git add apps/lite-web/src/teacher/assignmentLogic.ts apps/lite-web/src/teacher/assignmentLogic.test.ts \
  apps/lite-web/src/teacher/ReturnDialog.tsx apps/lite-web/src/teacher/AssignmentDetailPage.tsx
git commit -m "feat(lite-web): teacher returns a writing homework from the assignment page

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: Screenshot verification (1440px / 390px)

**Files:**
- Create (one-off, not committed): `apps/lite-web/e2e/finished-writing-shots.spec.ts`
- Output: `.superpowers/tmp/finished-writing-shots/*.png` (gitignored)

**Interfaces:**
- Consumes: every endpoint and screen from Tasks 3–12; the e2e harness `apps/lite-web/e2e/run-stack.sh` (throwaway Postgres, API, lite dev server; `globalSetup` flips the school to lite).

- [ ] **Step 1: Write the harness**

Create `apps/lite-web/e2e/finished-writing-shots.spec.ts`:

```ts
import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { expect, test, type APIResponse, type Browser, type Page } from "@playwright/test";

// One-off: screenshots of the finished-writing page, the revising strip, the
// locked page, the teacher's 退回修改 dialog and the returned state. Not a
// regression test; delete after looking at the pictures.

const PG = process.env.E2E_PG_CONTAINER ?? "mindimprint-lite-e2e-pg";
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:5174";
const JOIN = process.env.E2E_JOIN_CODE ?? "DEMO-0001";
const OUT = "../../.superpowers/tmp/finished-writing-shots";

const V1 = [
  "学校后门那片空地一下雨就积水。我去年在那里摔过一跤，所以一直记得。",
  "我读到城市里的雨水花园：用下凹的绿地先把雨水接住，再慢慢渗到地下。文章说这种做法能减少路面积水。",
].join("\n\n");

const V2 = [
  "学校后门那片空地一下雨就积水。去年秋天，我在那里摔过一跤。",
  "我读到城市里的雨水花园：用下凹的绿地先把雨水接住，再慢慢渗到地下。文章引用的监测数据显示，改造后路面积水时间缩短了一半以上。",
  "但雨水花园需要定期清理落叶，否则会堵住。学校有没有人负责这件事，是我下一步要问的问题。",
].join("\n\n");

function psql(sql: string): string {
  return execFileSync("docker", ["exec", PG, "psql", "-U", "postgres", "-d", "mindimprint", "-v", "ON_ERROR_STOP=1", "-tAc", sql], {
    encoding: "utf8",
  }).trim();
}

async function ok(p: Promise<APIResponse>): Promise<APIResponse> {
  const r = await p;
  expect(r.ok(), `${r.url()} ${r.status()} ${await r.text()}`).toBeTruthy();
  return r;
}

async function account(browser: Browser, label: string) {
  const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
  const email = `shot-${label}-${tag}@demo.mindimprint.local`;
  const password = `shot-${tag}-pass`;
  const ctx = await browser.newContext({ baseURL: BASE_URL });
  await ok(ctx.request.post("/api/v1/auth/signup", { data: { email, password, display_name: label === "teacher" ? "王老师" : "Phoebe", join_code: JOIN } }));
  await ok(ctx.request.post("/api/v1/auth/signin", { data: { email, password } }));
  return { ctx, email, password };
}

async function shoot(page: Page, name: string, width: number) {
  await page.setViewportSize({ width, height: width > 1000 ? 1000 : 844 });
  await page.waitForTimeout(300);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow, `${name}: page scrolls horizontally by ${overflow}px`).toBeLessThanOrEqual(0);
  await page.screenshot({ path: `${OUT}/${width}-${name}.png`, fullPage: true });
}

test("finished writing screenshots", async ({ browser }) => {
  mkdirSync(OUT, { recursive: true });
  const teacher = await account(browser, "teacher");
  const student = await account(browser, "student");

  const teacherId = psql(`SELECT id FROM users WHERE email = '${teacher.email}'`);
  psql(`UPDATE users SET role = 'teacher' WHERE id = '${teacherId}'`);
  psql(`UPDATE enrollments SET role_in_class = 'teacher' WHERE user_id = '${teacherId}'`);
  await ok(teacher.ctx.request.post("/api/v1/auth/signin", { data: { email: teacher.email, password: teacher.password } }));
  const classId = psql(`SELECT class_id FROM enrollments WHERE user_id = '${teacherId}' LIMIT 1`);
  const studentId = psql(`SELECT id FROM users WHERE email = '${student.email}'`);

  const t = teacher.ctx.request;
  const s = student.ctx.request;
  const created = await ok(
    t.post(`/api/v1/lite/teacher/classes/${classId}/assignments`, {
      data: {
        kind: "writing",
        title: "雨水去哪儿了",
        instructions: "",
        payload: { prompt: "写一篇关于校园积水的议论文", targetWords: 800, lang: "zh" },
        dueAt: new Date(Date.now() + 2 * 86400_000).toISOString(),
        userIds: [studentId],
      },
    }),
  );
  const aid = (await created.json()).assignment.id as string;
  const atomId = (await (await ok(s.post(`/api/v1/lite/assignments/${aid}/start`))).json()).atomId as string;
  const w = `/api/v1/writings/${atomId}`;
  await ok(s.put(`${w}/setup`, { data: { lang: "zh", targetWords: 800 } }));
  await ok(s.put(`${w}/draft`, { data: { body: V1 } }));
  await ok(s.post(`${w}/finish`));
  await ok(s.post(`${w}/revise`));
  await ok(s.put(`${w}/draft`, { data: { body: V2 } }));
  await ok(s.post(`${w}/finish`));

  const page = await student.ctx.newPage();

  // 1. Finished page, v1 selected, compared with v2.
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/writings/${atomId}`);
    await page.getByRole("heading", { name: "版本", exact: true }).waitFor();
    await page.getByRole("button", { name: /^v1 · / }).click();
    await page.getByRole("button", { name: "与当前版本对比", exact: true }).click();
    await page.locator("ins").first().waitFor();
    await shoot(page, "finished-diff", width);
  }

  // 2. Revising: the room with the strip.
  await ok(s.post(`${w}/revise`));
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/writings/${atomId}`);
    await page.getByText("修改完成后请再次点击「完成」提交新版本").waitFor();
    await shoot(page, "revising", width);
  }
  await ok(s.post(`${w}/revise/discard`));

  // 3. Locked: past the deadline.
  psql(`UPDATE lite_assignment SET due_at = now() - interval '1 hour' WHERE id = '${aid}'`);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/writings/${atomId}`);
    await page.getByText("已过截止时间，作业已锁定").waitFor();
    await expect(page.getByRole("button", { name: "修改", exact: true })).toBeDisabled();
    await shoot(page, "locked", width);
  }

  // 4. Teacher returns it.
  const tp = await teacher.ctx.newPage();
  await tp.setViewportSize({ width: 1440, height: 900 });
  await tp.goto(`/assignments/${aid}`);
  await tp.getByRole("button", { name: "退回修改", exact: true }).click();
  await tp.getByRole("dialog").waitFor();
  await tp.getByLabel("退回说明").fill("第二段的数据请注明出处，并补充学校现在的排水情况。");
  await shoot(tp, "teacher-return-dialog", 1440);
  await tp.getByRole("button", { name: "确认退回", exact: true }).click();
  await tp.getByText("已退回", { exact: true }).first().waitFor();
  await shoot(tp, "teacher-returned", 1440);

  // 5. Student sees 已退回 on the page and on the 写作 strip.
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/writings/${atomId}`);
    await page.getByText("退回说明：", { exact: false }).waitFor();
    await shoot(page, "returned", width);
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/writings");
  await page.getByRole("region", { name: "作业" }).getByText("已退回", { exact: true }).waitFor();
  await shoot(page, "strip-returned", 1440);
});
```

- [ ] **Step 2: Run it**

Run (ports away from the defaults, which other projects often hold):

```bash
E2E_API_PORT=8091 E2E_WEB_PORT=5184 E2E_PG_PORT=55443 bash apps/lite-web/e2e/run-stack.sh finished-writing-shots.spec.ts
```

Expected: `1 passed`, and 11 files in `.superpowers/tmp/finished-writing-shots/`: `1440-finished-diff.png`, `390-finished-diff.png`, `1440-revising.png`, `390-revising.png`, `1440-locked.png`, `390-locked.png`, `1440-teacher-return-dialog.png`, `1440-teacher-returned.png`, `1440-returned.png`, `390-returned.png`, `1440-strip-returned.png`.

If a step times out, open the trace (`apps/lite-web/test-results/…/trace.zip`, `npx playwright show-trace`) before changing the spec. A setup call that answers 409 means the assigned writing refuses the target: send `{ lang: "zh" }` only.

- [ ] **Step 3: Look at every picture**

Open each PNG with the Read tool and check:
- `1440-finished-diff`: text column about 44rem on the left, 版本 rail on the right, both inside the 1180px measure; chip 已提交; v1 row highlighted; `<ins>` green tint and `<del>` struck through, legend 新增 / 删除 under the button.
- `390-finished-diff`: one column, rail under the text, no horizontal scroll (the harness asserts it), buttons wrap without overflow.
- `*-revising`: the strip text 「已提交 v2 · 修改完成后请再次点击「完成」提交新版本」 and 放弃修改 are visible above the room.
- `*-locked`: chip 已锁定, 「已过截止时间，作业已锁定」, 修改 greyed out, the latest version (v2) on the left.
- `1440-teacher-return-dialog`: title 退回修改, student name, reason line, both fields, 取消 / 确认退回.
- `1440-teacher-returned`: the student's row shows 已退回.
- `*-returned`: chip 已退回, `已退回 · 截止 …` and 退回说明 under the header; 修改 enabled.
- `1440-strip-returned`: the 写作 landing 作业 strip lists the homework with 已退回 and 退回说明.
- No page uses a coloured left bar; tints use `color-mix` (Tailwind alpha on `mk-*` emits no CSS).

- [ ] **Step 4: Fix what the pictures show**

Any defect is fixed in the file that owns it (Task 10, 11 or 12), then rerun Step 2 and look again. Commit each fix separately:

```bash
git add <the files you changed>
git commit -m "fix(lite-web): <what the screenshot showed>

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 5: Remove the harness**

```bash
rm apps/lite-web/e2e/finished-writing-shots.spec.ts
git status --short apps/lite-web/e2e
```

Expected: nothing listed for `apps/lite-web/e2e`.

---

## Open questions carried into implementation

1. **「完成」 vs 「完成这篇」.** The strip copy is verbatim from the spec (「…请再次点击「完成」…」), but the button in `ComposeStage.tsx` reads 完成这篇. The plan keeps both as they are; the owner decides whether one of them changes.
2. **报告 opens on the article page.** `ReportPanel` is unchanged per the spec, and its first page is `ArticleView`, which repeats the latest version in the narrow column. The header button reads 报告 / 正文 to switch back.
3. **Revision minutes.** `atom_heartbeat.go` ignores finished atoms, so time spent revising is not added to the report's minutes.
4. **结构 stage while revising.** A finished writing whose stage is `outline` opens `PlanningView` without the strip.

## Self-review

**Spec coverage (Part A):**
- A1 table, numbering, immutability → Task 1 (schema, UNIQUE), Task 3 (insert in the finish transaction; first finish keeps `finished_at`). Backfill → Task 1 migration + test. Student read endpoints → Task 7. Teacher item versions + version route → Task 7 (route adapted, Deviation 1).
- A2 `revising_at`, revise, discard, finish on revising, status stays finished, `loadOwnedAtom` writing gate with `writing_finished` / `writing_locked` → Tasks 3–4. Frontend finished page vs room + strip + 放弃修改 → Tasks 10–11. Report `piece` follows the latest version → Task 8.
- A3 lock rule as one pure function, returned to the client as `locked` + `lockReason` → Task 2 (function), Tasks 3/4 (gate, finish, revise, discard), Task 7 (list response). Late first submission stays possible → Task 3 test. Deadline mid-revision keeps the draft → Task 4 test; the page shows the latest version → Task 11 `showFinishedPage`. Locked copy and disabled 修改 → Task 11.
- A4 columns → Task 1; endpoint with writing-only, version required, future `dueAt`, overwrite → Task 6; `returned` / `resubmitted` / overdue → Tasks 2 and 5; student sees 已退回 on the strip and the page with note and due date, 修改 enabled → Task 11; teacher dialog → Task 12.
- A5 route, 1180px measure, header chips and buttons, 44rem left column, version rail with `v3 · 9月15日 14:20 · 812 字`, 与当前版本对比, one column below 1024px, 报告 → Task 10; 老师批改 panel absent until Part B → not rendered. Diff as a pure function with logic tests → Task 9. Screenshots at 1440 / 390 → Task 13.
- §5 testing items for Part A: lock rule (Task 2), status derivation (Tasks 2, 5), version numbering and backfill (Tasks 1, 3, 4), `loadOwnedAtom` writing gate (Task 4), teacher ownership on new routes (Tasks 6, 7), diff (Task 9).

**Placeholder scan:** every code step carries the code; the two "if sqlc named it differently" notes give the grep that settles it and the expected name.

**Type consistency:** `writingLockFacts(ctx, q, atomID, ownerID)` is used identically in Tasks 3, 4, 7; `returnOf(returnedAt, returnDueAt, resubmitted)` in Tasks 5 and 6 through `newRecipientDTO`; `writingVersionSummaries` / `writingVersionDTOOf` / `versionNumber` defined in Task 7 and reused there by the teacher handler; `mergeLiveWritingFields(fields, draftBody, versionBody, title)` in Task 8 and its test; frontend `WritingVersionList.locked`, `AssignmentForAtom.returnedAt/returnDueAt/returnNote/resubmitted`, `RecipientDTO.versionCount`, `isRevising`, `effectiveDueAt`, `isReturnOpen`, `showFinishedPage`, `stripDueAt`, `buildReturnInput` match between their defining task and every later use.

# Lite Teacher End · Plan 2 — Practices with Deadlines Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A lite teacher assigns one reading, writing or project (with a deadline) to a class or picked students; students see it in a 老师布置 strip and a rail inbox with a red dot, start it with one click, and the teacher sees each student's status (未开始 / 进行中 / 已完成 / 逾期完成 / 已逾期).

**Architecture:** Two new tables (`lite_assignment`, `lite_assignment_recipient`) plus `pbl_project.finished_at`. Status is derived by a pure Go function. The student's item is created lazily on `start` through creation helpers extracted from the existing reading / writing / project handlers (handlers keep their behaviour). Teacher endpoints live under `/api/v1/lite/teacher/…` (plan 1's route group); student endpoints under `/api/v1/lite/…`.

**Tech Stack:** Go (`net/http`, `pgx`, `sqlc`, `goose`), PostgreSQL, React + Vite + TypeScript + Tailwind, vitest, Playwright one-off walk.

**Spec:** `docs/superpowers/specs/2026-09-14-lite-teacher-end-design.md` §5 (and §4.1 逾期作业 column, §4.2 assignments on the student page). Plan 1 (`docs/superpowers/plans/2026-09-14-lite-teacher-end-p1-student-data.md`) must be merged first.

## Global Constraints

- Lite must never break pro: never delete or rename files; `ls` before creating files in `apps/api/internal/store/{queries,migrations}` and `apps/api/internal/api`. Extracted helpers must leave every existing handler's HTTP behaviour identical — the existing handler tests are the proof and must stay green unchanged.
- Teacher routes: registered inside `(*API).registerLiteTeacherRoutes` using its `liteTeacher` wrapper (lite edition + teacher/admin). Every route naming a class uses `assertTeacherOwnsClass`; naming an assignment loads it and checks its class the same way. Failures 404 `资源不存在`.
- Student routes: `liteOnly`; every route naming an assignment checks a recipient row exists for the session user (404 otherwise) and the assignment is not archived.
- Status vocabulary (exact strings): `未开始` `进行中` `已完成` `逾期完成` `已逾期`. Wire values (exact): `not_started` `in_progress` `done` `done_late` `overdue`.
- "Finished": reading/writing `status='finished'` (use `finished_at`); project `finished_at IS NOT NULL`, stamped once the first time status becomes `review` or `keeping`.
- Deadlines are `timestamptz`. UI inputs are Beijing time, converted with a fixed `+08:00` suffix; display formats `M月D日 HH:mm` in Beijing (fixed offset, never a tz database lookup in Go).
- Assigned project start **skips** `siteGateOpen`; `POST /api/v1/pbl/projects` keeps the gate.
- Writing extraction uses `gateway.ClassCompose` via `a.routeE`; `llm_call` recorded with `a.recordLiteLLMCall(ctx, teacherUserID, uuid.Nil, "assignment_extract", resolved, usage)`.
- UI copy per AGENTS.md § 界面文案怎么写 (nouns for labels, 请+imperative instructions, 已/待/中 states, `动词+失败：{后台原话}` errors, no literary style).
- Tests: logic only (handlers, pure functions, normalizers). UI verified by screenshots.
- Go tests: `cd apps/api && CGO_ENABLED=0 go test ./internal/... -run <Name> -timeout 1800s`; sqlc: `cd apps/api && make sqlc`. Frontend: `pnpm --filter lite-web typecheck`, `pnpm --filter lite-web test`.
- Commits: stage specific files; message ends with `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`. Do not push except in the last task.

## Rulings carried from planning

- **Assigned writing still opens `WritingSetupModal`.** `createWriting` never stamps `setup_at`; the modal pre-fills `targetWords` from the writing (`WritingSetupModal.tsx:38`) and she confirms language, length and her note. She may change the length; the teacher's assignment detail keeps showing the assigned length. Structure and planning stay hers (spec §5.4).
- **Library start reuses an unfinished reading of the same slug + tier** (the existing `startLibraryReading` dedupe). The recipient row links to that atom.
- **URL source is fetched before any row is written**, so a fetch failure leaves no orphan reading.
- **Rooms learn about their assignment from one endpoint** (`GET /api/v1/lite/assignments/for-atom/{atomId}`) rather than changing three room DTOs.

---

## File Structure

**Backend (create)**
- `apps/api/internal/store/migrations/0149_lite_assignment.sql` (next free number)
- `apps/api/internal/store/queries/lite_assignment.sql`
- `apps/api/internal/liteassign/status.go` (+ `status_test.go`) — pure status derivation
- `apps/api/internal/liteassign/payload.go` (+ `payload_test.go`) — payload types + validation
- `apps/api/internal/api/lite_create_helpers.go` — extracted creation helpers
- `apps/api/internal/api/lite_teacher_assignments.go` (+ `_test.go`) — teacher CRUD
- `apps/api/internal/api/lite_assignment_extract.go` (+ `_internal_test.go`) — paste → fields
- `apps/api/internal/api/lite_student_assignments.go` (+ `_test.go`) — inbox, seen, start, for-atom

**Backend (modify)**
- `apps/api/internal/api/library.go`, `readings.go`, `reading_source.go`, `writings.go`, `pbl_projects.go` — handlers call the extracted helpers
- `apps/api/internal/store/queries/pbl_project.sql` — add `StampPblProjectFinished`
- `apps/api/internal/store/queries/lite_teacher.sql` — overdue count + student assignments
- `apps/api/internal/api/lite_teacher_roster.go`, `lite_teacher_routes.go`, `api.go`

**Frontend (create)**
- `apps/lite-web/src/api/assignments.ts` (+ `.test.ts`) — teacher + student clients, normalizers
- `apps/lite-web/src/shared/deadline.ts` (+ `.test.ts`) — Beijing input/format helpers, status labels
- `apps/lite-web/src/teacher/AssignmentsPage.tsx`, `AssignmentForm.tsx`, `AssignmentDetailPage.tsx`, `LibraryPicker.tsx`
- `apps/lite-web/src/inbox/InboxButton.tsx`, `InboxPanel.tsx`, `AssignmentStrip.tsx`, `AssignmentLine.tsx`, `startAssignment.ts` (+ `.test.ts`)

**Frontend (modify)**
- `apps/lite-web/src/teacher/teacherRouting.ts` (+ test), `LiteTeacherShell.tsx`, `ClassPage.tsx`, `StudentPage.tsx`
- `apps/lite-web/src/LiteApp.tsx` (rail inbox button)
- `apps/lite-web/src/readings/ReadingsLanding.tsx`, `writings/WritingsLanding.tsx`, `projects/ProjectsLanding.tsx` (strips)
- `apps/lite-web/src/readings/ReadingRoom.tsx`, `writings/WritingRoomHost.tsx`, `projects/ProjectRoom.tsx` (header line)

---

### Task 1: Tables, payloads, status

**Files:**
- Create: `apps/api/internal/store/migrations/0149_lite_assignment.sql`, `apps/api/internal/store/queries/lite_assignment.sql`
- Create: `apps/api/internal/liteassign/status.go`, `status_test.go`, `payload.go`, `payload_test.go`

**Interfaces:**
- Produces:
  - `liteassign.Status(started bool, finishedAt *time.Time, dueAt, now time.Time) string` → one of `not_started|in_progress|done|done_late|overdue`.
  - `liteassign.StatusLabel(status string) string` → Chinese label.
  - `type ReadingPayload struct { Source string \`json:"source"\`; Slug string \`json:"slug,omitempty"\`; Tier *int \`json:"tier,omitempty"\`; URL string \`json:"url,omitempty"\`; Text string \`json:"text,omitempty"\` }`
  - `type WritingPayload struct { Prompt string \`json:"prompt"\`; TargetWords int \`json:"targetWords"\`; Lang string \`json:"lang"\` }`
  - `type ProjectPayload struct { DrivingQuestion string \`json:"drivingQuestion"\`; Description string \`json:"description"\` }`
  - `liteassign.ValidatePayload(kind string, raw json.RawMessage) (json.RawMessage, error)` — returns the canonical re-marshalled payload; errors are `*liteassign.PayloadError{Code, Message string}`.
  - sqlc queries (Step 5).

- [ ] **Step 1: Failing status tests**

```go
package liteassign

import (
	"testing"
	"time"
)

func TestStatus(t *testing.T) {
	due := time.Date(2026, 9, 20, 22, 0, 0, 0, time.UTC)
	before := due.Add(-time.Hour)
	after := due.Add(time.Hour)
	at := func(t time.Time) *time.Time { return &t }

	cases := []struct {
		name       string
		started    bool
		finishedAt *time.Time
		now        time.Time
		want       string
	}{
		{"not started before due", false, nil, before, "not_started"},
		{"not started exactly at due", false, nil, due, "not_started"},
		{"not started after due", false, nil, after, "overdue"},
		{"started before due", true, nil, before, "in_progress"},
		{"started after due, unfinished", true, nil, after, "overdue"},
		{"finished exactly at due", true, at(due), after, "done"},
		{"finished one second late", true, at(due.Add(time.Second)), after, "done_late"},
		{"finished early, viewed later", true, at(before), after, "done"},
	}
	for _, c := range cases {
		if got := Status(c.started, c.finishedAt, due, c.now); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestStatusLabel(t *testing.T) {
	want := map[string]string{"not_started": "未开始", "in_progress": "进行中", "done": "已完成", "done_late": "逾期完成", "overdue": "已逾期"}
	for k, v := range want {
		if StatusLabel(k) != v {
			t.Errorf("%s → %s, want %s", k, StatusLabel(k), v)
		}
	}
}
```

- [ ] **Step 2: Failing payload tests**

```go
package liteassign

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidatePayload(t *testing.T) {
	ok := []struct{ kind, raw string }{
		{"reading", `{"source":"library","slug":"coffee","tier":3}`},
		{"reading", `{"source":"library","slug":"coffee"}`},
		{"reading", `{"source":"url","url":"https://example.com/a"}`},
		{"reading", `{"source":"text","text":"一段文章"}`},
		{"writing", `{"prompt":"写一篇关于雨的记叙文","targetWords":800,"lang":"zh"}`},
		{"project", `{"drivingQuestion":"怎样让校园少用一次性杯子？","description":""}`},
	}
	for _, c := range ok {
		if _, err := ValidatePayload(c.kind, json.RawMessage(c.raw)); err != nil {
			t.Errorf("%s %s: unexpected %v", c.kind, c.raw, err)
		}
	}
	bad := []struct{ kind, raw, code string }{
		{"reading", `{"source":"library","slug":""}`, "invalid_slug"},
		{"reading", `{"source":"library","slug":"coffee","tier":6}`, "invalid_tier"},
		{"reading", `{"source":"url","url":"ftp://x"}`, "invalid_url"},
		{"reading", `{"source":"text","text":"   "}`, "empty_text"},
		{"reading", `{"source":"pdf"}`, "invalid_source"},
		{"writing", `{"prompt":"","targetWords":800,"lang":"zh"}`, "empty_prompt"},
		{"writing", `{"prompt":"x","targetWords":0,"lang":"zh"}`, "invalid_target_words"},
		{"writing", `{"prompt":"x","targetWords":800,"lang":"fr"}`, "invalid_lang"},
		{"project", `{"drivingQuestion":"  "}`, "empty_driving_question"},
		{"quiz", `{}`, "invalid_kind"},
		{"writing", `not json`, "invalid_payload"},
	}
	for _, c := range bad {
		_, err := ValidatePayload(c.kind, json.RawMessage(c.raw))
		var pe *PayloadError
		if !errors.As(err, &pe) || pe.Code != c.code {
			t.Errorf("%s %s: got %v, want code %s", c.kind, c.raw, err, c.code)
		}
	}
}

func TestValidatePayloadCanonicalises(t *testing.T) {
	out, err := ValidatePayload("writing", json.RawMessage(`{"prompt":"  写雨  ","targetWords":800,"lang":"zh","extra":1}`))
	if err != nil {
		t.Fatal(err)
	}
	var w WritingPayload
	_ = json.Unmarshal(out, &w)
	if w.Prompt != "写雨" {
		t.Fatalf("prompt not trimmed: %q", w.Prompt)
	}
	if string(out) == `{"prompt":"  写雨  ","targetWords":800,"lang":"zh","extra":1}` {
		t.Fatal("unknown field kept")
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/liteassign/ -timeout 120s`
Expected: FAIL (package empty).

- [ ] **Step 4: Implement**

`status.go`:

```go
// Package liteassign holds the pure rules of a lite teacher assignment:
// how a recipient's status is derived and what a valid payload looks like.
package liteassign

import "time"

// Status derives a recipient's status. It is never stored: the inputs are
// the recipient's started flag, the item's finish time, the deadline and now.
func Status(started bool, finishedAt *time.Time, dueAt, now time.Time) string {
	if finishedAt != nil {
		if finishedAt.After(dueAt) {
			return "done_late"
		}
		return "done"
	}
	if now.After(dueAt) {
		return "overdue"
	}
	if started {
		return "in_progress"
	}
	return "not_started"
}

var labels = map[string]string{
	"not_started": "未开始",
	"in_progress": "进行中",
	"done":        "已完成",
	"done_late":   "逾期完成",
	"overdue":     "已逾期",
}

// StatusLabel is the UI word for a wire status.
func StatusLabel(status string) string { return labels[status] }
```

`payload.go`:

```go
package liteassign

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode/utf8"
)

type PayloadError struct{ Code, Message string }

func (e *PayloadError) Error() string { return e.Code + ": " + e.Message }

func perr(code, msg string) error { return &PayloadError{Code: code, Message: msg} }

type ReadingPayload struct {
	Source string `json:"source"`
	Slug   string `json:"slug,omitempty"`
	Tier   *int   `json:"tier,omitempty"`
	URL    string `json:"url,omitempty"`
	Text   string `json:"text,omitempty"`
}

type WritingPayload struct {
	Prompt      string `json:"prompt"`
	TargetWords int    `json:"targetWords"`
	Lang        string `json:"lang"`
}

type ProjectPayload struct {
	DrivingQuestion string `json:"drivingQuestion"`
	Description     string `json:"description"`
}

const (
	maxPromptRunes = 2000
	maxTextRunes   = 50000
)

// ValidatePayload checks a payload for kind and returns it re-marshalled
// from the typed struct, so unknown fields are dropped and strings trimmed.
func ValidatePayload(kind string, raw json.RawMessage) (json.RawMessage, error) {
	switch kind {
	case "reading":
		var p ReadingPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, perr("invalid_payload", "作业设置格式错误")
		}
		p.Slug, p.URL, p.Text = strings.TrimSpace(p.Slug), strings.TrimSpace(p.URL), strings.TrimSpace(p.Text)
		switch p.Source {
		case "library":
			if p.Slug == "" {
				return nil, perr("invalid_slug", "请选择一篇文章")
			}
			if p.Tier != nil && (*p.Tier < 1 || *p.Tier > 5) {
				return nil, perr("invalid_tier", "难度档位需在 1 到 5 之间")
			}
			p.URL, p.Text = "", ""
		case "url":
			u, err := url.Parse(p.URL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return nil, perr("invalid_url", "请输入以 http 或 https 开头的链接")
			}
			p.Slug, p.Tier, p.Text = "", nil, ""
		case "text":
			if p.Text == "" {
				return nil, perr("empty_text", "请粘贴文章正文")
			}
			if utf8.RuneCountInString(p.Text) > maxTextRunes {
				return nil, perr("text_too_long", "文章正文不能超过 50000 字")
			}
			p.Slug, p.Tier, p.URL = "", nil, ""
		default:
			return nil, perr("invalid_source", "阅读来源只能是分级阅读库、链接或正文")
		}
		return json.Marshal(p)
	case "writing":
		var p WritingPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, perr("invalid_payload", "作业设置格式错误")
		}
		p.Prompt = strings.TrimSpace(p.Prompt)
		if p.Prompt == "" {
			return nil, perr("empty_prompt", "请填写写作题目")
		}
		if utf8.RuneCountInString(p.Prompt) > maxPromptRunes {
			return nil, perr("prompt_too_long", "写作题目不能超过 2000 字")
		}
		if p.TargetWords < 1 || p.TargetWords > 100000 {
			return nil, perr("invalid_target_words", "目标字数需在 1 到 100000 之间")
		}
		if p.Lang != "zh" && p.Lang != "en" {
			return nil, perr("invalid_lang", "语言只能是中文或英文")
		}
		return json.Marshal(p)
	case "project":
		var p ProjectPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, perr("invalid_payload", "作业设置格式错误")
		}
		p.DrivingQuestion, p.Description = strings.TrimSpace(p.DrivingQuestion), strings.TrimSpace(p.Description)
		if p.DrivingQuestion == "" {
			return nil, perr("empty_driving_question", "请填写驱动问题")
		}
		if utf8.RuneCountInString(p.DrivingQuestion) > 4000 {
			return nil, perr("driving_question_too_long", "驱动问题不能超过 4000 字")
		}
		return json.Marshal(p)
	default:
		return nil, perr("invalid_kind", "作业类型只能是阅读、写作或项目")
	}
}
```

The 4000 cap mirrors `maxPblIdeaRunes` in `pbl_projects.go`; the 1–100000 range mirrors `minTargetWords`/`maxTargetWords` in `writing_stage.go`.

- [ ] **Step 5: Migration + queries**

`ls apps/api/internal/store/migrations | tail -3`; use the next free number. `0149_lite_assignment.sql`:

```sql
-- +goose Up
-- 教师布置的作业（lite）。学生那一项在她点「开始」时才创建，
-- recipient.atom_id 在那一刻写入；状态不存，由截止时间与完成时间推出。
CREATE TABLE lite_assignment (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  class_id     uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  created_by   uuid NOT NULL REFERENCES users(id),
  kind         text NOT NULL CHECK (kind IN ('reading','writing','project')),
  title        text NOT NULL,
  instructions text NOT NULL DEFAULT '',
  payload      jsonb NOT NULL,
  due_at       timestamptz NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz
);
CREATE INDEX lite_assignment_class_idx ON lite_assignment (class_id, due_at DESC);

CREATE TABLE lite_assignment_recipient (
  assignment_id uuid NOT NULL REFERENCES lite_assignment(id) ON DELETE CASCADE,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  seen_at       timestamptz,
  atom_id       uuid UNIQUE REFERENCES atom(id) ON DELETE SET NULL,
  started_at    timestamptz,
  PRIMARY KEY (assignment_id, user_id)
);
CREATE INDEX lite_assignment_recipient_user_idx ON lite_assignment_recipient (user_id);

-- 项目「做完」的时刻：第一次进入回顾或保留时写入，之后不再改。
ALTER TABLE pbl_project ADD COLUMN finished_at timestamptz;

-- +goose Down
ALTER TABLE pbl_project DROP COLUMN finished_at;
DROP TABLE lite_assignment_recipient;
DROP TABLE lite_assignment;
```

`atom_id UNIQUE` — note the library dedupe ruling: if two assignments point at the same library slug+tier, the second start would try to link the same unfinished atom and violate UNIQUE. Handle in Task 5 (create a fresh reading in that case).

`ls apps/api/internal/store/queries/lite_assignment.sql` must fail. Create:

```sql
-- name: CreateLiteAssignment :one
INSERT INTO lite_assignment (class_id, created_by, kind, title, instructions, payload, due_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: AddLiteAssignmentRecipient :exec
INSERT INTO lite_assignment_recipient (assignment_id, user_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: GetLiteAssignment :one
SELECT * FROM lite_assignment WHERE id = $1;

-- name: ListLiteAssignmentsByClass :many
-- 带每种状态的计数所需的原始列；状态在 Go 里推。
SELECT a.*,
       COALESCE((SELECT count(*) FROM lite_assignment_recipient r WHERE r.assignment_id = a.id), 0)::int AS recipient_count
FROM lite_assignment a
WHERE a.class_id = $1 AND a.archived_at IS NULL
ORDER BY a.due_at DESC;

-- name: ListLiteAssignmentRecipients :many
-- 一份作业的每个学生，连同她那一项的完成时间。
SELECT r.assignment_id, r.user_id, r.seen_at, r.atom_id, r.started_at,
       u.display_name, u.avatar_color,
       COALESCE(rd.finished_at, w.finished_at, p.finished_at) AS finished_at
FROM lite_assignment_recipient r
JOIN users u ON u.id = r.user_id
LEFT JOIN reading rd ON rd.atom_id = r.atom_id
LEFT JOIN writing w ON w.atom_id = r.atom_id
LEFT JOIN pbl_project p ON p.atom_id = r.atom_id
WHERE r.assignment_id = ANY(sqlc.arg(assignment_ids)::uuid[])
ORDER BY u.display_name;

-- name: GetLiteAssignmentRecipientForUpdate :one
SELECT * FROM lite_assignment_recipient WHERE assignment_id = $1 AND user_id = $2 FOR UPDATE;

-- name: GetLiteAssignmentRecipient :one
SELECT * FROM lite_assignment_recipient WHERE assignment_id = $1 AND user_id = $2;

-- name: SetLiteAssignmentStarted :exec
UPDATE lite_assignment_recipient
SET atom_id = $3, started_at = now(), seen_at = COALESCE(seen_at, now())
WHERE assignment_id = $1 AND user_id = $2;

-- name: MarkLiteAssignmentSeen :exec
UPDATE lite_assignment_recipient SET seen_at = COALESCE(seen_at, now())
WHERE assignment_id = $1 AND user_id = $2;

-- name: RemoveLiteAssignmentRecipient :execrows
DELETE FROM lite_assignment_recipient
WHERE assignment_id = $1 AND user_id = $2 AND atom_id IS NULL AND started_at IS NULL;

-- name: CountStartedLiteAssignmentRecipients :one
SELECT count(*)::int FROM lite_assignment_recipient WHERE assignment_id = $1 AND started_at IS NOT NULL;

-- name: UpdateLiteAssignment :one
UPDATE lite_assignment
SET title = $2, instructions = $3, due_at = $4, kind = $5, payload = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveLiteAssignment :exec
UPDATE lite_assignment SET archived_at = COALESCE(archived_at, now()), updated_at = now() WHERE id = $1;

-- name: ListLiteInboxAssignments :many
-- 她的作业，未读在前，其后按截止时间。
SELECT a.id, a.kind, a.title, a.instructions, a.payload, a.due_at, a.created_at,
       r.seen_at, r.atom_id, r.started_at,
       c.name AS class_name,
       COALESCE(rd.finished_at, w.finished_at, p.finished_at) AS finished_at
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
JOIN classes c ON c.id = a.class_id
LEFT JOIN reading rd ON rd.atom_id = r.atom_id
LEFT JOIN writing w ON w.atom_id = r.atom_id
LEFT JOIN pbl_project p ON p.atom_id = r.atom_id
WHERE r.user_id = $1 AND a.archived_at IS NULL
ORDER BY (r.seen_at IS NULL) DESC, a.due_at ASC;

-- name: GetLiteAssignmentForAtom :one
SELECT a.id, a.title, a.due_at
FROM lite_assignment_recipient r
JOIN lite_assignment a ON a.id = r.assignment_id
WHERE r.atom_id = $1 AND r.user_id = $2 AND a.archived_at IS NULL;

-- name: IsEnrolledStudent :one
SELECT EXISTS (SELECT 1 FROM enrollments WHERE class_id = $1 AND user_id = $2 AND role_in_class = 'student')::bool;
```

Also append to `apps/api/internal/store/queries/pbl_project.sql` (additive):

```sql
-- name: StampPblProjectFinished :exec
-- 第一次进入回顾或保留时写入完成时间；之后再改状态不覆盖。
UPDATE pbl_project SET finished_at = now() WHERE id = $1 AND finished_at IS NULL;
```

Run: `cd apps/api && make sqlc && go build ./...` and `CGO_ENABLED=0 go test ./internal/liteassign/ -timeout 120s`
Expected: builds; liteassign PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/migrations/0149_lite_assignment.sql apps/api/internal/store/queries/lite_assignment.sql apps/api/internal/store/queries/pbl_project.sql apps/api/internal/store/sqlc/ apps/api/internal/liteassign/
git commit -m "feat(lite-teacher): 作业表、设置校验与状态推导"
```

---

### Task 2: Stamp project finished_at

**Files:**
- Modify: `apps/api/internal/api/pbl_projects.go` (`patchPblProject`)
- Create: `apps/api/internal/api/pbl_finished_test.go`

**Interfaces:**
- Consumes: `sqlc.Queries.StampPblProjectFinished(ctx, projectID uuid.UUID) error` (Task 1).
- Produces: `pbl_project.finished_at` set the first time status becomes `review` or `keeping`; unchanged afterwards (including `archived`, and going back to `running`).

- [ ] **Step 1: Failing test**

```go
package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func patchProjectStatus(t *testing.T, h http.Handler, c *http.Cookie, projectID, status string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/pbl/projects/"+projectID,
		bytes.NewReader([]byte(`{"status":"`+status+`"}`))), c))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch %s = %d body=%s", status, rec.Code, rec.Body)
	}
}

func TestPblFinishedAtStampedOnce(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	projectID, _ := createPblProjectForTest(t, pool) // helper from plan 1 Task 2
	read := func() *time.Time {
		var ts *time.Time
		if err := pool.QueryRow(context.Background(), `SELECT finished_at FROM pbl_project WHERE id=$1`, projectID).Scan(&ts); err != nil {
			t.Fatal(err)
		}
		return ts
	}
	patchProjectStatus(t, h, cookie, projectID, "running")
	if read() != nil {
		t.Fatal("running stamped finished_at")
	}
	patchProjectStatus(t, h, cookie, projectID, "review")
	first := read()
	if first == nil {
		t.Fatal("review did not stamp finished_at")
	}
	patchProjectStatus(t, h, cookie, projectID, "running")
	patchProjectStatus(t, h, cookie, projectID, "keeping")
	if got := read(); got == nil || !got.Equal(*first) {
		t.Fatalf("finished_at changed: %v → %v", first, got)
	}
}
```

Check `patchPblProject`'s success code (200 vs 204) and its body field name for status; adjust the helper.

- [ ] **Step 2: Run → FAIL** (`finished_at` stays nil).

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestPblFinishedAtStampedOnce -timeout 1800s`

- [ ] **Step 3: Implement**

In `patchPblProject`, right after the successful `SetPblProjectStatus` call and before the existing harvest enqueue:

```go
		if *req.Status == "review" || *req.Status == "keeping" {
			// 作业「按时 / 逾期」按这一刻算；只写第一次。
			if err := a.d.Queries.StampPblProjectFinished(r.Context(), id); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		}
```

Match the variable names in the handler (`req.Status` may be a `*string`; the project id variable may be named differently).

- [ ] **Step 4: Run → PASS**, plus existing PBL project tests: `-run 'TestPbl'`.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/pbl_projects.go apps/api/internal/api/pbl_finished_test.go
git commit -m "feat(lite): 项目第一次进入回顾时记下完成时间"
```

---

### Task 3: Extract creation helpers

**Files:**
- Create: `apps/api/internal/api/lite_create_helpers.go`, `apps/api/internal/api/lite_create_helpers_internal_test.go`
- Modify: `apps/api/internal/api/library.go` (`startLibraryReading`), `readings.go` (`createReading`), `writings.go` (`createWriting`), `pbl_projects.go` (`createPblProject`)

**Interfaces:**
- Produces (all on `*API`, all run their own transaction, none checks entitlement or the site gate — callers do):
  - `createLibraryReadingFor(ctx context.Context, userID uuid.UUID, slug string, tier int) (atomID uuid.UUID, resumed bool, err error)` — same dedupe as today (unfinished same slug+tier → existing id, resumed=true). Errors: `errLibraryArticleNotFound`, `errLibraryTierInvalid` (package-level sentinel errors the handler maps to its existing 404/400 responses).
  - `createLibraryReadingFreshFor(ctx, userID, slug, tier) (uuid.UUID, error)` — same, without dedupe.
  - `suggestLibraryTierFor(ctx, userID) (int, error)` — the `getLibraryShelf` computation (`finishedTop`, `abandonedTop` → `library.SuggestTier`).
  - `createReadingWithSourceFor(ctx, userID uuid.UUID, title, lang, srcURL, text string) (uuid.UUID, error)` — when `srcURL != ""` fetch first with `a.d.Fetcher.FetchReadable(ctx, srcURL)` (nil fetcher → `errFetchUnavailable`; fetch error → wrapped `errFetchFailed`), then one tx: `CreateAtom` + `CreateReading` + `UpsertReadingSource`.
  - `createWritingFor(ctx, userID uuid.UUID, idea, lang string, targetWords *int32) (uuid.UUID, error)` — the `createWriting` tx without the brought-body path; when `targetWords != nil` also `SetWritingTargetWords` inside the same tx.
  - `createPblProjectFor(ctx, userID uuid.UUID, idea string) (sqlc.PblProject, error)` — the post-gate tx.

- [ ] **Step 1: Failing internal tests**

Write `lite_create_helpers_internal_test.go` (package `api`) with an `*API` over `NewTestDB(t)` and the seed user (reuse whatever `newHeartbeatTestAtom` from plan 1 uses to build the API):

```go
func TestCreateLibraryReadingForDedupes(t *testing.T) {
	a := newInternalTestAPI(t)
	ctx := context.Background()
	slug := library.All()[0].Slug // read internal/library for the real accessor name
	first, resumed, err := a.createLibraryReadingFor(ctx, SeedUserIDInternal, slug, 3)
	if err != nil || resumed {
		t.Fatalf("first: %v resumed=%v", err, resumed)
	}
	second, resumed, err := a.createLibraryReadingFor(ctx, SeedUserIDInternal, slug, 3)
	if err != nil || !resumed || second != first {
		t.Fatalf("second: %v resumed=%v same=%v", err, resumed, second == first)
	}
	fresh, err := a.createLibraryReadingFreshFor(ctx, SeedUserIDInternal, slug, 3)
	if err != nil || fresh == first {
		t.Fatalf("fresh: %v same=%v", err, fresh == first)
	}
}

func TestCreateWritingForSetsTargetWords(t *testing.T) {
	a := newInternalTestAPI(t)
	tw := int32(800)
	id, err := a.createWritingFor(context.Background(), SeedUserIDInternal, "写一篇关于雨的记叙文", "zh", &tw)
	if err != nil {
		t.Fatal(err)
	}
	w, err := a.d.Queries.GetWriting(context.Background(), id)
	if err != nil || w.TargetWords == nil || *w.TargetWords != 800 || w.Title == "" {
		t.Fatalf("writing = %+v err=%v", w, err)
	}
}

func TestCreatePblProjectForSkipsNoGate(t *testing.T) {
	a := newInternalTestAPI(t)
	p, err := a.createPblProjectFor(context.Background(), SeedUserIDInternal, "怎样让校园少用一次性杯子？")
	if err != nil || p.Idea == "" {
		t.Fatalf("project = %+v err=%v", p, err)
	}
}
```

Replace `newInternalTestAPI`, `SeedUserIDInternal` and `library.All()` with the real internal-test helper, seed id constant and library accessor (grep `internal/api/*_internal_test.go` and `internal/library/library.go`). Adjust the writing field types to the generated struct.

- [ ] **Step 2: Run → FAIL** (helpers undefined).

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestCreate(Library|Writing|Pbl).*For' -timeout 1800s`

- [ ] **Step 3: Implement by moving code, not rewriting it**

Move each handler's transactional body into the helper verbatim, replacing HTTP writes with returned errors. The handler keeps: `UserFromContext`, `HasEntitlement`, path/body parsing and validation, the site gate (PBL), mapping helper errors to its existing responses, and its existing success response (`201 {id}` / `200 {id, resumed:true}` / `201 pblProjectDTO`). `putReadingSourceLite` is **not** changed — `createReadingWithSourceFor` reuses the same fetch + `UpsertReadingSource` calls but is only called by assignments.

- [ ] **Step 4: Run new + existing handler tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestCreate|Library|Reading|Writing|Pbl' -timeout 1800s`
Expected: PASS, with no existing test modified.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/lite_create_helpers.go apps/api/internal/api/lite_create_helpers_internal_test.go apps/api/internal/api/library.go apps/api/internal/api/readings.go apps/api/internal/api/writings.go apps/api/internal/api/pbl_projects.go
git commit -m "refactor(lite): 阅读、写作、项目的创建逻辑抽成可复用函数"
```

---

### Task 4: Teacher assignment endpoints + writing extraction

**Files:**
- Create: `apps/api/internal/api/lite_teacher_assignments.go`, `lite_teacher_assignments_test.go`, `lite_assignment_extract.go`, `lite_assignment_extract_internal_test.go`
- Modify: `apps/api/internal/api/lite_teacher_routes.go`

**Interfaces:**
- Consumes: `liteassign.ValidatePayload`, `liteassign.Status`, Task 1 queries, plan 1 `liteTeacherFixture`, `getJSON`, `assertTeacherOwnsClass`.
- Produces:
  - `POST /api/v1/lite/teacher/classes/{id}/assignments` body `{kind, title, instructions, payload, dueAt (RFC3339), userIds []string}` → `201 {"assignment": AssignmentDTO}`.
  - `GET /api/v1/lite/teacher/classes/{id}/assignments` → `{"assignments": []AssignmentSummaryDTO}`.
  - `GET /api/v1/lite/teacher/assignments/{aid}` → `{"assignment": AssignmentDTO, "recipients": []RecipientDTO}`.
  - `PATCH /api/v1/lite/teacher/assignments/{aid}` body `{title?, instructions?, dueAt?, kind?, payload?, addUserIds?, removeUserIds?}` → `200 {"assignment": AssignmentDTO}`.
  - `DELETE /api/v1/lite/teacher/assignments/{aid}` → 204 (archive).
  - `POST /api/v1/lite/teacher/assignments/extract` body `{text}` → `{"prompt": string, "targetWords": number|null, "lang": "zh"|"en"}`.
  - DTOs (JSON camelCase):
    - `AssignmentDTO{ID, ClassID, Kind, Title, Instructions string; Payload json.RawMessage; DueAt, CreatedAt string}`
    - `AssignmentSummaryDTO{AssignmentDTO fields…; Counts map[string]int}` — keys are the five wire statuses, every key present.
    - `RecipientDTO{UserID, DisplayName, AvatarColor, Status, StatusLabel string; AtomID, StartedAt, FinishedAt, SeenAt *string}`
  - `normalizeExtraction(raw string) (prompt string, targetWords *int, lang string, ok bool)` pure function.

Error codes (400): `invalid_title` (empty or > 200 runes), `invalid_due_at` (unparseable), `no_recipients`, `invalid_recipient` (any id not an enrolled student of the class), any `PayloadError.Code`; (409) `assignment_started` when changing kind/payload after any recipient started, or removing a started recipient.

- [ ] **Step 1: Failing handler tests**

```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func doJSON(t *testing.T, h http.Handler, c *http.Cookie, method, path string, body any, out any) int {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(method, path, &buf), c))
	if out != nil && rec.Code < 300 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode %s %s: %v body=%s", method, path, err, rec.Body)
		}
	}
	return rec.Code
}

func writingAssignmentBody(userIDs []string) map[string]any {
	return map[string]any{
		"kind": "writing", "title": "雨", "instructions": "",
		"payload": map[string]any{"prompt": "写一篇关于雨的记叙文", "targetWords": 800, "lang": "zh"},
		"dueAt":   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"userIds": userIDs,
	}
}

func TestTeacherCreatesAndListsAssignment(t *testing.T) {
	h, _, teacher, classID, studentID := liteTeacherFixture(t)
	var created struct{ Assignment struct{ ID string `json:"id"` } `json:"assignment"` }
	if code := doJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments",
		writingAssignmentBody([]string{studentID.String()}), &created); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	var list struct {
		Assignments []struct {
			ID     string         `json:"id"`
			Counts map[string]int `json:"counts"`
		} `json:"assignments"`
	}
	getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/assignments", &list)
	if len(list.Assignments) != 1 || list.Assignments[0].Counts["not_started"] != 1 || len(list.Assignments[0].Counts) != 5 {
		t.Fatalf("list = %+v", list)
	}
}

func TestTeacherAssignmentRejectsNonMember(t *testing.T) {
	h, pool, teacher, classID, _ := liteTeacherFixture(t)
	stranger := createStudent(t, pool, SeedSchoolID, "as-stranger@demo.local")
	if code := doJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments",
		writingAssignmentBody([]string{stranger.String()}), nil); code != http.StatusBadRequest {
		t.Fatalf("stranger recipient = %d, want 400", code)
	}
	if code := doJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments",
		writingAssignmentBody([]string{}), nil); code != http.StatusBadRequest {
		t.Fatalf("no recipients = %d, want 400", code)
	}
}

func TestTeacherAssignmentOtherTeacher404(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	var created struct{ Assignment struct{ ID string `json:"id"` } `json:"assignment"` }
	doJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments", writingAssignmentBody([]string{studentID.String()}), &created)
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "as-other@demo.local"))
	if code := getJSON(t, h, other, "/api/v1/lite/teacher/assignments/"+created.Assignment.ID, nil); code != http.StatusNotFound {
		t.Fatalf("other teacher detail = %d", code)
	}
	if code := doJSON(t, h, other, "DELETE", "/api/v1/lite/teacher/assignments/"+created.Assignment.ID, nil, nil); code != http.StatusNotFound {
		t.Fatalf("other teacher archive = %d", code)
	}
}

func TestTeacherAssignmentEditLocksAfterStart(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	var created struct{ Assignment struct{ ID string `json:"id"` } `json:"assignment"` }
	doJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments", writingAssignmentBody([]string{studentID.String()}), &created)
	aid := created.Assignment.ID

	// Before start: payload change allowed.
	if code := doJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"payload": map[string]any{"prompt": "写风", "targetWords": 600, "lang": "zh"}}, nil); code != http.StatusOK {
		t.Fatalf("pre-start payload patch = %d", code)
	}
	// Student starts.
	student := signInAs(t, pool, studentID)
	if code := doJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, nil); code != http.StatusOK {
		t.Fatalf("start = %d", code)
	}
	if code := doJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"payload": map[string]any{"prompt": "写雪", "targetWords": 600, "lang": "zh"}}, nil); code != http.StatusConflict {
		t.Fatalf("post-start payload patch = %d, want 409", code)
	}
	if code := doJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"removeUserIds": []string{studentID.String()}}, nil); code != http.StatusConflict {
		t.Fatalf("remove started recipient = %d, want 409", code)
	}
	if code := doJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"title": "雨（改）", "dueAt": time.Now().Add(72 * time.Hour).Format(time.RFC3339)}, nil); code != http.StatusOK {
		t.Fatalf("title/due patch after start = %d, want 200", code)
	}
}
```

`TestTeacherAssignmentEditLocksAfterStart` depends on Task 5's start endpoint; mark it `t.Skip("needs Task 5 start endpoint")` in this task and remove the skip in Task 5.

`lite_assignment_extract_internal_test.go`:

```go
package api

import "testing"

func TestNormalizeExtraction(t *testing.T) {
	cases := []struct {
		raw       string
		prompt    string
		words     int // 0 = nil
		lang      string
		ok        bool
	}{
		{`{"prompt":"写一篇关于雨的记叙文","targetWords":800,"lang":"zh"}`, "写一篇关于雨的记叙文", 800, "zh", true},
		{"```json\n{\"prompt\":\"Describe a storm\",\"targetWords\":300,\"lang\":\"en\"}\n```", "Describe a storm", 300, "en", true},
		{`{"prompt":"写雨","targetWords":20,"lang":"zh"}`, "写雨", 50, "zh", true},
		{`{"prompt":"写雨","targetWords":999999,"lang":"zh"}`, "写雨", 10000, "zh", true},
		{`{"prompt":"写雨","targetWords":null,"lang":"fr"}`, "写雨", 0, "zh", true},
		{`{"prompt":"","targetWords":800,"lang":"zh"}`, "", 0, "", false},
		{`not json`, "", 0, "", false},
	}
	for _, c := range cases {
		p, w, l, ok := normalizeExtraction(c.raw)
		gotWords := 0
		if w != nil {
			gotWords = *w
		}
		if ok != c.ok || (ok && (p != c.prompt || gotWords != c.words || l != c.lang)) {
			t.Errorf("%q → (%q,%d,%q,%v)", c.raw, p, gotWords, l, ok)
		}
	}
}
```

Rule for `lang` outside `zh|en`: `zh` if the prompt contains any CJK rune, else `en`.

- [ ] **Step 2: Run → FAIL** (routes / function missing).

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestTeacherAssignment|TestTeacherCreates|TestNormalizeExtraction' -timeout 1800s`

- [ ] **Step 3: Implement handlers**

`lite_teacher_assignments.go` — key rules (write the handlers around these):

```go
// loadTeacherAssignment loads {aid} and checks the caller teaches its class.
func (a *API) loadTeacherAssignment(w http.ResponseWriter, r *http.Request) (sqlc.LiteAssignment, bool) {
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.LiteAssignment{}, false
	}
	as, err := a.d.Queries.GetLiteAssignment(r.Context(), aid)
	if err != nil || as.ArchivedAt != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.LiteAssignment{}, false
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), as.ClassID); err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.LiteAssignment{}, false
	}
	return as, true
}

func payloadErrorResponse(err error) error {
	var pe *liteassign.PayloadError
	if errors.As(err, &pe) {
		return httpx.ErrBadRequest(pe.Code, pe.Message, nil)
	}
	return err
}
```

- Create: validate title (trim, 1..200 runes), `dueAt` via `time.Parse(time.RFC3339, …)`, payload via `ValidatePayload`, `userIds` non-empty, each parsed and `IsEnrolledStudent(classID, uid)` true; then in one tx `CreateLiteAssignment` + `AddLiteAssignmentRecipient` per id. Respond 201.
- List: `ListLiteAssignmentsByClass`, then one `ListLiteAssignmentRecipients(ids)` call, group by assignment, count `liteassign.Status(r.StartedAt != nil, r.FinishedAt, as.DueAt, now)`; initialise all five keys to 0.
- Detail: recipients mapped to `RecipientDTO` with `Status` and `StatusLabel`.
- Patch: load; if `kind` or `payload` present → `CountStartedLiteAssignmentRecipients > 0` ⇒ 409 `assignment_started` (`httpx.ErrConflict` takes a message; if it has no code parameter, check `httpx` for a coded conflict constructor and use it); validate the new payload against the (possibly new) kind. `addUserIds` validated like create. `removeUserIds`: `RemoveLiteAssignmentRecipient` returns rows affected; 0 for an existing started recipient ⇒ 409 (check existence with `GetLiteAssignmentRecipient` first to distinguish from "not a recipient", which is a no-op). Everything in one tx; then `UpdateLiteAssignment` with merged fields.
- Archive: `ArchiveLiteAssignment`, 204.

Register in `registerLiteTeacherRoutes`:

```go
	mux.Handle("POST /api/v1/lite/teacher/classes/{id}/assignments", liteTeacher(a.createLiteAssignment))
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/assignments", liteTeacher(a.listLiteAssignments))
	mux.Handle("GET /api/v1/lite/teacher/assignments/{aid}", liteTeacher(a.getLiteAssignment))
	mux.Handle("PATCH /api/v1/lite/teacher/assignments/{aid}", liteTeacher(a.patchLiteAssignment))
	mux.Handle("DELETE /api/v1/lite/teacher/assignments/{aid}", liteTeacher(a.archiveLiteAssignment))
	mux.Handle("POST /api/v1/lite/teacher/assignments/extract", liteTeacher(a.extractLiteAssignment))
```

Go 1.22 mux: `POST …/assignments/extract` and `GET …/assignments/{aid}` do not conflict (different methods); `PATCH/DELETE …/{aid}` never match `extract` with POST. Confirm the build does not panic on registration.

`lite_assignment_extract.go`:

```go
const assignmentExtractSystem = `你从老师粘贴的一段作业说明里提取写作作业的三项设置。
只输出 JSON：{"prompt":"","targetWords":0,"lang":"zh"}。
prompt：学生要写的题目或要求，保留老师的原话，不改写、不补充。
targetWords：老师写明的字数；写的是范围取上限；没写就填 null。
lang：作文要用的语言，中文填 zh，英文填 en。`

var cjkRune = regexp.MustCompile(`\p{Han}`)
var fence = regexp.MustCompile("(?s)^\\s*```(?:json)?\\s*(.*?)\\s*```\\s*$")

// normalizeExtraction parses the model's reply and clamps it to what the
// form accepts. Words are clamped to 50–10000; an unknown language falls
// back by script.
func normalizeExtraction(raw string) (string, *int, string, bool) {
	if m := fence.FindStringSubmatch(raw); m != nil {
		raw = m[1]
	}
	var v struct {
		Prompt      string   `json:"prompt"`
		TargetWords *float64 `json:"targetWords"`
		Lang        string   `json:"lang"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &v); err != nil {
		return "", nil, "", false
	}
	prompt := strings.TrimSpace(v.Prompt)
	if prompt == "" {
		return "", nil, "", false
	}
	var words *int
	if v.TargetWords != nil {
		n := int(*v.TargetWords)
		if n < 50 {
			n = 50
		}
		if n > 10000 {
			n = 10000
		}
		words = &n
	}
	lang := v.Lang
	if lang != "zh" && lang != "en" {
		lang = "en"
		if cjkRune.MatchString(prompt) {
			lang = "zh"
		}
	}
	return prompt, words, lang, true
}

// extractLiteAssignment handles POST /api/v1/lite/teacher/assignments/extract.
func (a *API) extractLiteAssignment(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" || utf8.RuneCountInString(text) > 4000 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_text", "请粘贴 1 到 4000 字的作业说明", nil))
		return
	}
	ctx, cancel := detachedModelCtx(r)
	defer cancel()
	resolved, err := a.routeE(ctx, gateway.ClassCompose)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{Messages: []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: assignmentExtractSystem},
		{Role: gateway.RoleUser, Content: text},
	}})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.recordLiteLLMCall(ctx, u.ID, uuid.Nil, "assignment_extract", resolved, res.Usage)
	prompt, words, lang, ok := normalizeExtraction(res.Text)
	if !ok {
		httpx.WriteError(w, r, httpx.ErrBadGateway("extract_unparsed", "模型返回的内容无法解析："+truncateRunes(res.Text, 200)))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"prompt": prompt, "targetWords": words, "lang": lang})
}
```

Check `httpx` for the real 502 constructor name and a rune-truncation helper; if neither exists, use the closest existing error constructor and write `truncateRunes` locally. The model error must surface (memory: ai-errors-must-surface-never-fake).

Also add a handler test with a scripted provider (`liteHandlerWithProvider` pattern — see `edition_test.go` and the fake providers used by lite turn tests) asserting the extract endpoint returns the parsed fields and records one `llm_call` row with purpose `assignment_extract`.

- [ ] **Step 4: Run → PASS**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestTeacherAssignment|TestTeacherCreates|TestNormalizeExtraction|TestExtract' -timeout 1800s`

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/lite_teacher_assignments.go apps/api/internal/api/lite_teacher_assignments_test.go apps/api/internal/api/lite_assignment_extract.go apps/api/internal/api/lite_assignment_extract_internal_test.go apps/api/internal/api/lite_teacher_routes.go
git commit -m "feat(lite-teacher): 布置作业 —— 创建、列表、详情、修改、归档、题目提取"
```

---

### Task 5: Student inbox, seen, start, for-atom

**Files:**
- Create: `apps/api/internal/api/lite_student_assignments.go`, `lite_student_assignments_test.go`
- Modify: `apps/api/internal/api/api.go` (four `liteOnly` routes), `lite_teacher_assignments_test.go` (remove the Task 4 skip)

**Interfaces:**
- Consumes: Task 3 helpers, Task 1 queries, `liteassign`.
- Produces:
  - `GET /api/v1/lite/inbox` → `{"items": []InboxItemDTO, "unread": int}`; `InboxItemDTO{Type:"assignment", ID, Kind, Title, Instructions, ClassName, DueAt string; Status, StatusLabel string; AtomID *string; Unread bool}`. (Plan 4 adds `Type:"parent_report"` items to the same list.)
  - `POST /api/v1/lite/assignments/{aid}/seen` → 204.
  - `POST /api/v1/lite/assignments/{aid}/start` → `200 {"kind": "reading"|"writing"|"project", "atomId": string, "projectId": string|null}` (`projectId` is the `pbl_project.id` the project room route uses).
  - `GET /api/v1/lite/assignments/for-atom/{atomId}` → `{"assignment": {"id","title","dueAt"} | null}`.

- [ ] **Step 1: Failing tests**

Build the fixture on plan 1's `liteTeacherFixture` + Task 4's `doJSON` / `writingAssignmentBody`. Add helpers creating reading-library, reading-url and project assignments.

```go
func createAssignment(t *testing.T, h http.Handler, teacher *http.Cookie, classID string, body map[string]any) string {
	t.Helper()
	var created struct{ Assignment struct{ ID string `json:"id"` } `json:"assignment"` }
	if code := doJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments", body, &created); code != http.StatusCreated {
		t.Fatalf("create assignment = %d", code)
	}
	return created.Assignment.ID
}

func TestInboxShowsUnreadThenSeen(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	student := signInAs(t, pool, studentID)
	var inbox struct {
		Items  []struct{ ID string `json:"id"`; Unread bool `json:"unread"`; Status string `json:"status"` } `json:"items"`
		Unread int `json:"unread"`
	}
	getJSON(t, h, student, "/api/v1/lite/inbox", &inbox)
	if inbox.Unread != 1 || len(inbox.Items) != 1 || !inbox.Items[0].Unread || inbox.Items[0].Status != "not_started" {
		t.Fatalf("inbox = %+v", inbox)
	}
	if code := doJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/seen", nil, nil); code != http.StatusNoContent {
		t.Fatalf("seen = %d", code)
	}
	getJSON(t, h, student, "/api/v1/lite/inbox", &inbox)
	if inbox.Unread != 0 {
		t.Fatalf("unread after seen = %d", inbox.Unread)
	}
}

func TestStartIsIdempotentUnderConcurrency(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	student := signInAs(t, pool, studentID)
	ids := make(chan string, 2)
	for i := 0; i < 2; i++ {
		go func() {
			var out struct{ AtomID string `json:"atomId"` }
			doJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, &out)
			ids <- out.AtomID
		}()
	}
	a, b := <-ids, <-ids
	if a == "" || a != b {
		t.Fatalf("concurrent starts returned %q and %q", a, b)
	}
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM atom WHERE user_id=$1 AND kind='writing'`, studentID).Scan(&n)
	if n != 1 {
		t.Fatalf("writings created = %d, want 1", n)
	}
}

func TestStartProjectBypassesHomepageGate(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	// The gate is closed for a fresh student.
	if code := doJSON(t, h, student, "POST", "/api/v1/pbl/projects", map[string]any{"idea": "x"}, nil); code != http.StatusConflict {
		t.Fatalf("own project = %d, want 409 (gate)", code)
	}
	aid := createAssignment(t, h, teacher, classID, map[string]any{
		"kind": "project", "title": "杯子", "instructions": "",
		"payload": map[string]any{"drivingQuestion": "怎样让校园少用一次性杯子？"},
		"dueAt":   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"userIds": []string{studentID.String()},
	})
	var out struct {
		Kind      string  `json:"kind"`
		ProjectID *string `json:"projectId"`
	}
	if code := doJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, &out); code != http.StatusOK || out.Kind != "project" || out.ProjectID == nil {
		t.Fatalf("start project = %d %+v", code, out)
	}
	if code := doJSON(t, h, student, "POST", "/api/v1/pbl/projects", map[string]any{"idea": "y"}, nil); code != http.StatusConflict {
		t.Fatalf("own project after assigned start = %d, want still 409", code)
	}
}

func TestStartLibraryNullTierUsesSuggestion(t *testing.T) {
	// payload {"source":"library","slug":<real slug>} with no tier → the created reading's library_tier == SuggestTier for a student with no history.
	// Read internal/library for SuggestTier(0,0)'s value and a real slug.
}

func TestStartOtherStudentAndArchived404(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	other := createStudent(t, pool, SeedSchoolID, "as-not-recipient@demo.local")
	enrollStudent(t, pool, other, classID)
	if code := doJSON(t, h, signInAs(t, pool, other), "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, nil); code != http.StatusNotFound {
		t.Fatalf("non-recipient start = %d", code)
	}
	doJSON(t, h, teacher, "DELETE", "/api/v1/lite/teacher/assignments/"+aid, nil, nil)
	student := signInAs(t, pool, studentID)
	if code := doJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, nil); code != http.StatusNotFound {
		t.Fatalf("archived start = %d", code)
	}
	var inbox struct{ Items []any `json:"items"` }
	getJSON(t, h, student, "/api/v1/lite/inbox", &inbox)
	if len(inbox.Items) != 0 {
		t.Fatalf("archived still in inbox: %+v", inbox)
	}
}

func TestForAtomReturnsAssignment(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	student := signInAs(t, pool, studentID)
	var started struct{ AtomID string `json:"atomId"` }
	doJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/start", nil, &started)
	var out struct{ Assignment *struct{ ID string `json:"id"` } `json:"assignment"` }
	getJSON(t, h, student, "/api/v1/lite/assignments/for-atom/"+started.AtomID, &out)
	if out.Assignment == nil || out.Assignment.ID != aid {
		t.Fatalf("for-atom = %+v", out)
	}
	// Another student's atom id → assignment null (not 404, not leaked).
	other := signInAs(t, pool, createStudent(t, pool, SeedSchoolID, "as-for-atom-other@demo.local"))
	getJSON(t, h, other, "/api/v1/lite/assignments/for-atom/"+started.AtomID, &out)
	if out.Assignment != nil {
		t.Fatal("for-atom leaked another student's assignment")
	}
}
```

Fill in `TestStartLibraryNullTierUsesSuggestion` with real values (it must assert, not be empty). Also add a status test: finish the started writing via `POST /api/v1/writings/{id}/finish` (check the real route) and assert the teacher detail shows `done`; set the assignment `due_at` in the past via SQL before finishing and assert `done_late`.

Fixture note: `liteTeacherFixture` sets **every** school to lite and the seed student's homepage gate must be closed — check `siteGateOpen`; if the seed data already has a published site for new students (it should not), note it.

- [ ] **Step 2: Run → FAIL**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestInbox|TestStart|TestForAtom' -timeout 1800s`

- [ ] **Step 3: Implement**

Core of `start`:

```go
// startLiteAssignment handles POST /api/v1/lite/assignments/{aid}/start.
// Idempotent: the recipient row is locked; if it already names an atom that
// atom is returned. Otherwise the item is created through the same helpers
// the student's own create buttons use, and linked in the same transaction
// scope as the lock.
func (a *API) startLiteAssignment(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	ctx := r.Context()
	as, err := a.d.Queries.GetLiteAssignment(ctx, aid)
	if err != nil || as.ArchivedAt != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	// …begin tx; qtx.GetLiteAssignmentRecipientForUpdate(aid, u.ID) (ErrNoRows → 404);
	// if rec.AtomID != nil → commit, respond with it (look up project id when kind=project);
	// else create via helper (see below), qtx.SetLiteAssignmentStarted(aid, u.ID, atomID), commit, respond.
}
```

The creation helpers run their own transactions (Task 3). Holding the recipient `FOR UPDATE` lock in an outer tx while a helper commits a separate tx is safe for idempotency: the second concurrent caller blocks on the lock until the first commits, then sees `atom_id` set. If the helper fails, roll back the outer tx and write `httpx.ErrBadRequest("start_failed", "开始失败："+err.Error(), nil)` for fetch errors; other errors pass through `httpx.WriteError`. A failure after the helper committed but before `SetLiteAssignmentStarted` leaves an unlinked item — acceptable (she can still see it in her list); log it with `slog.Warn`.

Per kind:
- reading/library: `tier := payload.Tier` else `suggestLibraryTierFor`; `createLibraryReadingFor`; if `resumed` and that atom is already linked to another recipient row (`UNIQUE` would fail — check with a query `SELECT 1 FROM lite_assignment_recipient WHERE atom_id=$1`), use `createLibraryReadingFreshFor` instead.
- reading/url: `createReadingWithSourceFor(ctx, u.ID, as.Title, "zh", url, "")`. Language: `zh` unless the assignment title contains no CJK rune, then `en`.
- reading/text: `createReadingWithSourceFor(ctx, u.ID, as.Title, lang, "", text)` with the same language rule applied to the text.
- writing: `createWritingFor(ctx, u.ID, payload.Prompt, payload.Lang, &tw)`.
- project: `createPblProjectFor(ctx, u.ID, payload.DrivingQuestion)` — no `siteGateOpen`.

Inbox: `ListLiteInboxAssignments(u.ID)` → items with `Status` from `liteassign.Status(started_at != nil, finished_at, due_at, now)`; `unread` = count of `seen_at IS NULL`.

For-atom: `GetLiteAssignmentForAtom(atomID, u.ID)`; `pgx.ErrNoRows` → `{"assignment": null}`.

Routes in `api.go` next to other lite routes:

```go
	mux.Handle("GET /api/v1/lite/inbox", liteOnly(a.getLiteInbox))
	mux.Handle("POST /api/v1/lite/assignments/{aid}/seen", liteOnly(a.markLiteAssignmentSeen))
	mux.Handle("POST /api/v1/lite/assignments/{aid}/start", liteOnly(a.startLiteAssignment))
	mux.Handle("GET /api/v1/lite/assignments/for-atom/{atomId}", liteOnly(a.getLiteAssignmentForAtom))
```

Remove the `t.Skip` added in Task 4.

- [ ] **Step 4: Run → PASS** (new tests + Task 4 tests + `-race` on the concurrency test once: `go test -race -run TestStartIsIdempotentUnderConcurrency` — CGO is required for `-race`; if CGO is unavailable, note it in the report instead).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/lite_student_assignments.go apps/api/internal/api/lite_student_assignments_test.go apps/api/internal/api/lite_teacher_assignments_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): 学生收件箱与「开始作业」"
```

---

### Task 6: Overdue column and assignments on the student page

**Files:**
- Modify: `apps/api/internal/store/queries/lite_teacher.sql`, `apps/api/internal/api/lite_teacher_roster.go`, `lite_teacher_roster_test.go`

**Interfaces:**
- Produces: `LiteRosterRowDTO.OverdueAssignments int32` (`overdueAssignments`); student page response gains `"assignments": []StudentAssignmentDTO{ID, Kind, Title, DueAt, Status, StatusLabel string; AtomID *string}` for that class.

- [ ] **Step 1: Failing tests** — create an assignment with `due_at` in the past (insert via SQL after create), unstarted: roster row `overdueAssignments == 1`; student page `assignments[0].status == "overdue"`. A finished-late assignment counts 0 overdue.

- [ ] **Step 2: Run → FAIL**

- [ ] **Step 3: Implement** — overdue is derived, so do it in Go: add query `ListLiteClassRecipientStates :many` (`class_id` → `user_id, started_at, due_at, finished_at` for non-archived assignments of that class, joined like `ListLiteAssignmentRecipients`), count `liteassign.Status(...) == "overdue"` per user, and merge into roster rows. Student page: `ListLiteStudentAssignments(class_id, user_id)` → DTO list ordered by `due_at DESC`.

- [ ] **Step 4: Run → PASS** with `-run 'TestLiteRoster|TestLiteStudentPage'`.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/queries/lite_teacher.sql apps/api/internal/store/sqlc/ apps/api/internal/api/lite_teacher_roster.go apps/api/internal/api/lite_teacher_roster_test.go
git commit -m "feat(lite-teacher): 名单显示逾期作业，学生页列出作业"
```

---

### Task 7: Frontend clients and pure helpers

**Files:**
- Create: `apps/lite-web/src/api/assignments.ts`, `assignments.test.ts`, `apps/lite-web/src/shared/deadline.ts`, `deadline.test.ts`, `apps/lite-web/src/inbox/startAssignment.ts`, `startAssignment.test.ts`
- Modify: `apps/lite-web/src/teacher/teacherRouting.ts` + test, `apps/lite-web/src/api/teacher.ts` (roster/student-page types)

**Interfaces:**
- Produces:
  - `deadline.ts`: `beijingInputToISO(v: string): string | null` (`"2026-09-20T22:00"` → `"2026-09-20T22:00:00+08:00"`; invalid → null); `isoToBeijingInput(iso: string): string`; `formatDeadline(iso: string): string` → `M月D日 HH:mm` in Beijing regardless of the browser's zone; `STATUS_LABEL: Record<AssignmentStatus, string>`; `type AssignmentStatus = "not_started" | "in_progress" | "done" | "done_late" | "overdue"`.
  - `assignments.ts`: teacher `listAssignments(classId)`, `createAssignment(classId, input)`, `getAssignment(aid)`, `patchAssignment(aid, patch)`, `archiveAssignment(aid)`, `extractWritingFields(text)`; student `getInbox()`, `markSeen(aid)`, `startAssignment(aid)`, `getAssignmentForAtom(atomId)`; types mirroring the Go DTOs.
  - `startAssignment.ts`: `roomPathForStart(r: {kind, atomId, projectId}): string` → `/readings/{atomId}` | `/writings/{atomId}` | `/projects/{projectId}` (uses `liteRoutePath`).
  - `teacherRouting.ts`: routes `{view:"assignments"}` `/assignments`, `{view:"assignmentNew"; classId?: string}` `/assignments/new` (`?class=` query not needed — pick class in the form), `{view:"assignment"; assignmentId}` `/assignments/:aid`.

- [ ] **Step 1: Failing tests**

```ts
// deadline.test.ts
import { describe, expect, it } from "vitest";
import { beijingInputToISO, formatDeadline, isoToBeijingInput } from "./deadline";

describe("deadline", () => {
  it("treats the input as Beijing time", () => {
    expect(beijingInputToISO("2026-09-20T22:00")).toBe("2026-09-20T22:00:00+08:00");
  });
  it("rejects malformed input", () => {
    expect(beijingInputToISO("2026-09-20")).toBeNull();
    expect(beijingInputToISO("")).toBeNull();
  });
  it("formats in Beijing regardless of UTC instant", () => {
    expect(formatDeadline("2026-09-20T14:00:00Z")).toBe("9月20日 22:00");
    expect(formatDeadline("2026-09-20T16:30:00Z")).toBe("9月21日 00:30");
  });
  it("round-trips to the input format", () => {
    expect(isoToBeijingInput("2026-09-20T14:00:00Z")).toBe("2026-09-20T22:00");
  });
});
```

Implement Beijing formatting by adding 8h to the UTC epoch and reading `getUTC*` fields — do not rely on `Intl` time zone data or the test machine's zone.

```ts
// startAssignment.test.ts
import { describe, expect, it } from "vitest";
import { roomPathForStart } from "./startAssignment";

describe("roomPathForStart", () => {
  it("reading", () => expect(roomPathForStart({ kind: "reading", atomId: "a1", projectId: null })).toBe("/readings/a1"));
  it("writing", () => expect(roomPathForStart({ kind: "writing", atomId: "a2", projectId: null })).toBe("/writings/a2"));
  it("project uses the project id", () => expect(roomPathForStart({ kind: "project", atomId: "a3", projectId: "p3" })).toBe("/projects/p3"));
});
```

`assignments.test.ts`: normalizer test — an inbox response missing `unread` defaults to the count of `unread: true` items; unknown `status` values map to `not_started`.

Extend `teacherRouting.test.ts`'s table with the three new routes.

- [ ] **Step 2: Run → FAIL**; **Step 3: Implement**; **Step 4: Run → PASS** (`pnpm --filter lite-web test && pnpm --filter lite-web typecheck`).

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/api/assignments.ts apps/lite-web/src/api/assignments.test.ts apps/lite-web/src/shared/deadline.ts apps/lite-web/src/shared/deadline.test.ts apps/lite-web/src/inbox/startAssignment.ts apps/lite-web/src/inbox/startAssignment.test.ts apps/lite-web/src/teacher/teacherRouting.ts apps/lite-web/src/teacher/teacherRouting.test.ts apps/lite-web/src/api/teacher.ts
git commit -m "feat(lite): 作业前端接口与截止时间工具"
```

---

### Task 8: Teacher assignment UI

**Files:**
- Create: `apps/lite-web/src/teacher/AssignmentsPage.tsx`, `AssignmentForm.tsx`, `AssignmentDetailPage.tsx`, `LibraryPicker.tsx`
- Modify: `apps/lite-web/src/teacher/LiteTeacherShell.tsx` (rail item 布置, routes), `ClassPage.tsx` (逾期作业 column + 布置作业 button), `StudentPage.tsx` (作业 section)

**Interfaces:**
- Consumes: Task 7 clients and helpers; library catalogue client already in `apps/lite-web/src/api/library.ts` (read it for the shelf/article list call and tier names 入门/基础/进阶/高阶/原文); `api.listClasses` from pro client for the class selector.

UI verified by screenshots in Task 10.

- [ ] **Step 1: AssignmentsPage** (`/assignments`): class selector (teacher's classes; remember last choice in `localStorage` wrapped in try/catch), button 布置作业 → `/assignments/new`, list of assignments: 类型 · 标题 · 截止时间 · 未开始 n · 进行中 n · 已完成 n · 逾期完成 n · 已逾期 n. Row click → detail. Empty: `暂无作业`.

- [ ] **Step 2: AssignmentForm** (`/assignments/new`):
  - Lead line (rule 5 — reason before request): `学生会在收件箱和对应页面顶部看到这份作业。`
  - 班级 (select), 类型 (three segmented buttons 阅读 / 写作 / 项目), 标题, 说明 (optional textarea), 截止时间 (`<input type="datetime-local">`, value converted with `beijingInputToISO`; label `截止时间（北京时间）`).
  - 阅读: source tabs 分级阅读库 / 链接 / 正文. 分级阅读库 → `LibraryPicker` (article list with search by title; level buttons 入门 基础 进阶 高阶 原文 plus 按学生当前水平, which sends `tier` omitted). 链接 → URL input. 正文 → textarea.
  - 写作: 题目 (textarea), 目标字数 (number), 语言 (中文 / 英文). Above them a collapsible 从作业说明提取: textarea + button 提取 → `extractWritingFields`; on success fill the three fields; on failure `提取失败：{message}`. Busy label `提取中`.
  - 项目: 驱动问题, 补充说明.
  - 学生: checklist of the class roster (`getRoster`), all checked by default, 全选 / 全不选.
  - Submit 发布作业; client-side required checks mirror the server messages; server errors shown as `发布失败：{message}`. On success navigate to the detail page.

- [ ] **Step 3: AssignmentDetailPage** (`/assignments/:aid`): header (类型 · 标题 · 截止时间 · 说明 · the settings summary, e.g. 目标字数 800 · 中文), actions 修改 (inline edit of 标题 / 说明 / 截止时间; kind/payload edit only shown when every recipient is 未开始) and 归档 (confirm `归档后学生将不再看到这份作业，已开始的内容仍保留。` / 确认归档 / 取消). Table of recipients: 学生 · 状态 (coloured chip per status using `color-mix` on accent tokens, no left bars) · 开始时间 · 完成时间 · link 查看 → plan 1 item page when `atomId` present. A 添加学生 control lists unassigned class students.

- [ ] **Step 4: Shell, class page, student page**
  - Rail: add 布置 (lucide `ClipboardList`) between 班级 and admin items; routes for the three new views.
  - ClassPage: add column 逾期作业 (value 0 shown as `0`, >0 emphasised) and a header button 布置作业 → `/assignments/new`.
  - StudentPage: section 作业 listing the student's assignments in this class with status chips and deadlines; rows with `atomId` open the item page.

- [ ] **Step 5: Typecheck + tests, commit**

```bash
pnpm --filter lite-web typecheck && pnpm --filter lite-web test
git add apps/lite-web/src/teacher/
git commit -m "feat(lite-teacher): 布置作业页面、作业详情与名单逾期列"
```

---

### Task 9: Student inbox, strips, room header line

**Files:**
- Create: `apps/lite-web/src/inbox/InboxButton.tsx`, `InboxPanel.tsx`, `AssignmentStrip.tsx`, `AssignmentLine.tsx`, `useInbox.ts`
- Modify: `apps/lite-web/src/LiteApp.tsx` (rail), `readings/ReadingsLanding.tsx`, `writings/WritingsLanding.tsx`, `projects/ProjectsLanding.tsx`, `readings/ReadingRoom.tsx`, `writings/WritingRoomHost.tsx`, `projects/ProjectRoom.tsx`

**Interfaces:**
- Consumes: Task 7 `getInbox`, `markSeen`, `startAssignment`, `getAssignmentForAtom`, `roomPathForStart`, `formatDeadline`, `STATUS_LABEL`; `navigate` from `../routing`.
- Produces: `useInbox(): { items, unread, status: "loading"|"error"|"ready", error, reload }` (refetches when `reload` is called and when the window regains focus); `openAssignment(item, reload)` helper inside `InboxPanel`/`AssignmentStrip`: `markSeen` (ignore failure), then `startAssignment` → `navigate(roomPathForStart(result))`; failure shows `开始失败：{message}` inline.

- [ ] **Step 1: useInbox + InboxButton + InboxPanel**
  - `InboxButton` sits in `LiteShell`'s rail **directly above 设置** (inside the nav, before the `mt-auto` settings button — move `mt-auto` onto the inbox button so both stay pinned at the foot). Icon `Inbox` (lucide), label 收件箱, and a red dot (8px, `background: var(--mk-danger)`, white 2px ring) at the icon's top-right when `unread > 0`. `aria-label` includes the count: `收件箱，{n} 条未读`.
  - `InboxPanel` renders via `createPortal` to `document.body` as a fixed panel next to the rail (the rail is `overflow-hidden`; anything inside it is clipped). Width 360px (max `calc(100vw - 32px)`), closes on Escape, outside click, and navigation. Title 收件箱; lead line `老师布置的作业会出现在这里。`; items unread first: kind chip, title, class name, `截止 {formatDeadline}`, status chip; unread items bold with the same red dot. Empty: `暂无消息`. Error: `加载失败：{error}` + 重试.

- [ ] **Step 2: AssignmentStrip on the three landings**
  - `AssignmentStrip({ kind })` uses `useInbox` filtered to that kind and status `not_started|in_progress|overdue`. Hidden when empty. Title `老师布置`; each row: title, `截止 {formatDeadline}`, status chip, button 开始 (not started) / 继续 (in progress or overdue with atom).
  - ReadingsLanding / WritingsLanding: mount it in the existing unwired NOTICES slot (the comment says "P4's teacher-assigned tasks … ABOVE the unfinished line"); update that comment to say it is now wired.
  - ProjectsLanding: mount it above the gate / idea block (`// 1 · 门，或者输入框`), because an assigned project is reachable even while the gate is closed.

- [ ] **Step 3: AssignmentLine in the rooms**
  - `AssignmentLine({ atomId })` calls `getAssignmentForAtom` once; renders nothing when null or on error; otherwise `老师布置 · 截止 {formatDeadline(dueAt)}` in muted small text.
  - ReadingRoom: next to the existing `来源 · …` meta line. WritingRoomHost: under `EditableTitle`. ProjectRoom: inside its `<header>` under the title (the room has the project's `atomId` on the project object — check `Project` type; if it lacks `atomId`, add it to the lite `Project` type since `pblProjectDTO` already includes it, or add it to the DTO additively).

- [ ] **Step 4: Typecheck + tests, commit**

```bash
pnpm --filter lite-web typecheck && pnpm --filter lite-web test
git add apps/lite-web/src/inbox/ apps/lite-web/src/LiteApp.tsx apps/lite-web/src/readings/ReadingsLanding.tsx apps/lite-web/src/writings/WritingsLanding.tsx apps/lite-web/src/projects/ProjectsLanding.tsx apps/lite-web/src/readings/ReadingRoom.tsx apps/lite-web/src/writings/WritingRoomHost.tsx apps/lite-web/src/projects/ProjectRoom.tsx
git commit -m "feat(lite): 收件箱、老师布置条与房间内截止提示"
```

---

### Task 10: Browser walk, pro safety, push

- [ ] **Step 1: Walk.** With local stack running (same setup as plan 1 Task 10): as teacher, publish one reading (library, 按学生当前水平), one writing (use 从作业说明提取 with a pasted paragraph such as `请写一篇 600 字左右的记叙文，题目是《一场雨》，用中文完成。`), one project, each to two students, one with a deadline 1 minute ahead. As student A: red dot visible; open inbox; start each; confirm the reading room, writing room (setup modal pre-filled with 600), project room (with the homepage gate still closed on /projects) and the header line `老师布置 · 截止 …`. Finish the reading after the 1-minute deadline. As teacher: detail shows 逾期完成 for A, 已逾期 for B; roster shows 逾期作业. Screenshot every screen at 1440×900 and 400×800 and read each screenshot (nothing blank, nothing clipped — especially the inbox panel next to the rail, copy per the rules).

- [ ] **Step 2: Pro safety.**

```bash
git diff --diff-filter=D --name-only origin/main..HEAD
git diff --diff-filter=R --name-only origin/main..HEAD
pnpm --filter web typecheck && pnpm --filter web test
cd apps/api && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -timeout 1800s
```

Expected: first two print nothing; suites green (pre-existing failures reported with evidence they fail on `origin/main`).

- [ ] **Step 3: Push.** `git fetch origin`, `git rebase origin/main`, `git push origin HEAD:main` (run as separate commands). Renumber the migration if it collides after rebase, re-run `make sqlc` and the Go tests.

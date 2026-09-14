# Lite Teacher End · Plan 4 — Parent Report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** From a student page the teacher generates a parent report for a chosen date range: a frozen facts snapshot plus an AI-drafted, code-checked set of sections in Chinese. The teacher edits the text, publishes a revocable link parents open without logging in (`/r/:token`, image export), and the student sees the published report in her inbox.

**Architecture:** One table `lite_parent_report` holds facts (frozen at generation), the last AI draft, the teacher-edited body, status, and share token. A pure package builds the facts text and validates ranges; the composer reuses plan 3's `liteweekly.CheckProse` checks. Teacher endpoints under `/api/v1/lite/teacher/…`; public read under `/api/v1/public/parent-reports/{token}`; student read under `/api/v1/lite/parent-reports/{rid}`; plan 2's inbox gains `parent_report` items.

**Tech Stack:** Go, PostgreSQL, sqlc, goose, React + TypeScript + Tailwind, `html-to-image` (already used by lite reports), vitest.

**Spec:** `docs/superpowers/specs/2026-09-14-lite-teacher-end-design.md` §7 (and DEC-10, DEC-11). Plans 1–3 merged first.

Research notes: `/Users/houyuxin/.claude/jobs/3520028e/tmp/research-p3-p4.md`.

## Global Constraints

- Lite must never break pro; `apps/web` untouched. `ls` before creating files in shared Go dirs.
- Range: `rangeStart`, `rangeEnd` are Beijing calendar dates `YYYY-MM-DD`, inclusive; default last 28 days ending yesterday; `rangeEnd` ≤ today (Beijing); span 1–366 days; errors 400 `invalid_range` `请选择 1 到 366 天之内、不晚于今天的日期范围`.
- Sections (keys and order, exact): `overview` 总体概述 · `reading` 阅读 · `writing` 写作 · `projects` 项目 · `interests` 兴趣 · `next` 下一步建议. A section whose facts are empty is omitted from the draft (`overview` and `next` are always present).
- Model: `a.routeE(ctx, gateway.ClassAssess)` + `gateway.Collect`; every attempt recorded with `a.recordLiteLLMCall(ctx, teacherUserID, uuid.Nil, "lite_parent_report", resolved, usage)`; `detachedModelCtx(r)`.
- Checks (code): `liteweekly.CheckProse` over all section texts with `AllowedCodes` = nil and codes = nil, `Corpus` = the snapshot's 金句 + item titles + keywords, `FactsText` = the snapshot facts text, `OtherNames` = display names of the other students in the class. Section length ≤ 400 runes. Retry once; after the second failure the row keeps facts only and the response carries `draftError`.
- Editing: body sections plain text, each ≤ 2000 runes, only the six keys accepted.
- Publish mints a token with the existing `newShareToken()` (`atom_report_share.go`). Revoke sets it NULL; the public GET returns 404 on the next request. Body edits after publish show immediately.
- Public response: facts + body + names + range + publisher name + published date only (no ids, no draft, no internal fields); header `X-Robots-Tag: noindex, nofollow, noarchive` (copy of `pbl_site.go`).
- Student: sees her own **published** reports only; revoking the link does not hide it from her.
- 金句 are labelled 学生原话 on every surface. Numbers shown come from facts, never parsed from prose.
- Image export: `exportPoster(node, filename)`; the offscreen offset lives on a wrapper, never the rasterised node; poster uses explicit hex colours and a system font stack.
- UI copy per AGENTS.md § 界面文案怎么写.
- Tests logic only; `LIVE_LLM=1` run of the prompt once before shipping.
- Go tests `cd apps/api && CGO_ENABLED=0 go test ./internal/... -run <Name> -timeout 1800s`; `make sqlc`; frontend `pnpm --filter lite-web typecheck && pnpm --filter lite-web test`. `Deps.Pool` has only `Begin`; tests query through the concrete test pool.
- Commits: specific files; trailer `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

---

## File Structure

**Create**
- `apps/api/internal/store/migrations/0151_lite_parent_report.sql` (next free), `queries/lite_parent_report.sql`
- `apps/api/internal/liteparent/facts.go` (+ `facts_test.go`) — facts struct, range parsing, facts text, section keys
- `apps/api/internal/agent/compose_lite_parent.go` (+ `compose_lite_parent_test.go`, `compose_lite_parent_live_test.go`)
- `apps/api/internal/api/lite_parent_report.go` (+ `lite_parent_report_test.go`) — loader + teacher endpoints
- `apps/api/internal/api/lite_parent_report_read.go` (+ `lite_parent_report_read_test.go`) — public + student + inbox items
- `apps/lite-web/src/api/parentReports.ts` (+ `.test.ts`)
- `apps/lite-web/src/parentReport/ParentReportView.tsx`, `ParentReportPoster.tsx`, `PublicParentReportPage.tsx`, `range.ts` (+ `range.test.ts`)
- `apps/lite-web/src/teacher/ParentReportsPage.tsx`, `ParentReportEditor.tsx`, `GenerateParentReportDialog.tsx`

**Modify**
- `apps/api/internal/api/lite_teacher_routes.go`, `api.go` (public + student routes), `lite_student_assignments.go` (inbox merge)
- `apps/lite-web/src/routing.ts` (+ `routing.test.ts`), `rootElementFor.tsx`, `LiteApp.tsx` (student route), `inbox/InboxPanel.tsx`, `api/assignments.ts` (inbox item union)
- `apps/lite-web/src/teacher/teacherRouting.ts` (+ test), `LiteTeacherShell.tsx`, `StudentPage.tsx`

---

### Task 1: Table, facts package

**Files:** migration, queries, `internal/liteparent/facts.go`, `facts_test.go`.

**Interfaces:**
- Produces:

```go
package liteparent

var SectionKeys = []string{"overview", "reading", "writing", "projects", "interests", "next"}
var SectionLabels = map[string]string{"overview": "总体概述", "reading": "阅读", "writing": "写作", "projects": "项目", "interests": "兴趣", "next": "下一步建议"}

type Item struct {
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	FinishedAt string `json:"finishedAt"` // YYYY-MM-DD Beijing
}
type Moment struct {
	Quote     string `json:"quote"`
	ItemTitle string `json:"itemTitle"`
}
type Keyword struct {
	Text  string `json:"text"`
	Field string `json:"field"`
	FieldLabel string `json:"fieldLabel"`
}
type Facts struct {
	StudentName string    `json:"studentName"`
	ClassName   string    `json:"className"`
	TeacherName string    `json:"teacherName"`
	RangeStart  string    `json:"rangeStart"`
	RangeEnd    string    `json:"rangeEnd"`
	Days        int       `json:"days"`
	ActiveDays  int       `json:"activeDays"`
	Minutes     int       `json:"minutes"` // -1 when no buckets exist in range
	Turns       int       `json:"turns"`
	Readings    []Item    `json:"readings"`
	Writings    []Item    `json:"writings"`
	Projects    []Item    `json:"projects"`
	Moments     []Moment  `json:"moments"`
	AssignmentsTotal   int `json:"assignmentsTotal"`
	AssignmentsOnTime  int `json:"assignmentsOnTime"`
	AssignmentsLate    int `json:"assignmentsLate"`
	AssignmentsMissed  int `json:"assignmentsMissed"`
	Keywords    []Keyword `json:"keywords"`
}

func ParseRange(start, end string, now time.Time) (s, e time.Time, err error) // Beijing dates; ErrBadRange
func DefaultRange(now time.Time) (start, end string)                            // last 28 days ending yesterday
func FactsText(f Facts) string
func Corpus(f Facts) string
func SectionsWithFacts(f Facts) []string // overview, [reading if len(Readings)>0], [writing…], [projects…], [interests if len(Keywords)>0], next
```

- [ ] **Step 1: Failing tests**

```go
func TestParseRange(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, liteweek.Beijing)
	if _, _, err := ParseRange("2026-08-15", "2026-09-13", now); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if _, _, err := ParseRange("2026-09-14", "2026-09-14", now); err != nil {
		t.Fatalf("today as end is allowed: %v", err)
	}
	for _, c := range [][2]string{
		{"2026-09-13", "2026-09-12"}, // reversed
		{"2026-09-10", "2026-09-15"}, // end in the future
		{"2025-09-01", "2026-09-13"}, // > 366 days
		{"2026/09/01", "2026-09-13"}, // format
	} {
		if _, _, err := ParseRange(c[0], c[1], now); !errors.Is(err, ErrBadRange) {
			t.Errorf("%v: want ErrBadRange, got %v", c, err)
		}
	}
}

func TestDefaultRange(t *testing.T) {
	s, e := DefaultRange(time.Date(2026, 9, 14, 10, 0, 0, 0, liteweek.Beijing))
	if s != "2026-08-17" || e != "2026-09-13" {
		t.Fatalf("default = %s..%s", s, e)
	}
}

func TestSectionsWithFacts(t *testing.T) {
	got := SectionsWithFacts(Facts{Writings: []Item{{Kind: "writing", Title: "一场雨"}}})
	want := []string{"overview", "writing", "next"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestFactsTextCarriesEveryNumber(t *testing.T) {
	f := Facts{RangeStart: "2026-08-17", RangeEnd: "2026-09-13", Days: 28, ActiveDays: 12, Minutes: 340, Turns: 86, AssignmentsTotal: 3, AssignmentsOnTime: 2, AssignmentsLate: 1}
	txt := FactsText(f)
	for _, n := range []string{"2026", "08", "17", "09", "13", "28", "12", "340", "86", "3", "2", "1"} {
		if !strings.Contains(txt, n) {
			t.Errorf("facts text missing %s: %s", n, txt)
		}
	}
}
```

28 days ending 2026-09-13 inclusive starts 2026-08-17 — the test asserts that.

- [ ] **Step 2: Migration**

```sql
-- +goose Up
-- 家长报告：老师为一名学生、一段日期生成。facts 在生成时冻结；draft 是最近一次模型草稿；
-- body 是老师改过的文字；发布后才有 share_token，撤销时置空。
CREATE TABLE lite_parent_report (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  class_id        uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  created_by      uuid NOT NULL REFERENCES users(id),
  range_start     date NOT NULL,
  range_end       date NOT NULL,
  facts           jsonb NOT NULL,
  draft           jsonb,
  body            jsonb,
  status          text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  share_token     text UNIQUE,
  published_at    timestamptz,
  student_seen_at timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX lite_parent_report_user_idx ON lite_parent_report (user_id, created_at DESC);
CREATE INDEX lite_parent_report_class_idx ON lite_parent_report (class_id, created_at DESC);

-- +goose Down
DROP TABLE lite_parent_report;
```

- [ ] **Step 3: Queries** (`lite_parent_report.sql`): `CreateLiteParentReport :one` (user_id, class_id, created_by, range_start, range_end, facts), `SetLiteParentReportDraft :one` (id, draft, body — body set only when currently NULL: `body = COALESCE(body, sqlc.arg(draft))` for first draft; a separate `ReplaceLiteParentReportBody :one` for explicit replacement), `UpdateLiteParentReportBody :one`, `GetLiteParentReport :one`, `ListLiteParentReportsByStudent :many` (user_id, class_id), `ListLiteParentReportsByClass :many` (joins users for student name), `PublishLiteParentReport :one` (status published, share_token COALESCE existing, published_at COALESCE existing), `RevokeLiteParentReportShare :one`, `GetLiteParentReportByToken :one` (published only), `GetStudentParentReport :one` (id, user_id, published), `MarkParentReportSeen :exec`, `ListStudentPublishedParentReports :many` (user_id, with class name), plus fact queries mirroring plan 3's `lite_weekly.sql` over a date range: `ParentRangeActivity :one`, `ParentRangeFinished :many`, `ParentRangeMoments :many`, `ParentRangeAssignmentStates :many`, `ParentRangeKeywords :many` (keyword text, field; first_seen_at in range; ordered by strength desc, limit 12), `ListClassStudentNames :many`.

Run `make sqlc && go build ./...`.

- [ ] **Step 4: Implement facts.go; run tests → PASS. Commit** `feat(lite-teacher): 家长报告表与事实结构`.

---

### Task 2: Facts loader + composer

**Files:** `internal/agent/compose_lite_parent.go` (+ test), loader in `internal/api/lite_parent_report.go` (+ loader test in `lite_parent_report_test.go`).

**Interfaces:**
- Produces:
  - `(*API).loadLiteParentFacts(ctx, classID, userID, teacherID uuid.UUID, start, end time.Time) (liteparent.Facts, error)` — also ensures reports exist for finished readings/writings in range via `a.ensureAtomReport(ctx, userID, atomID, kind)` before reading moments (phase 1, no model call); errors from ensure are logged and that item's moments skipped.
  - `agent.ComposeLiteParentReport(ctx context.Context, prov gateway.Provider, r gateway.Resolved, f liteparent.Facts, otherNames []string) (map[string]string, []Attempt, error)` — `Attempt` is plan 3's type in the same package.

Prompt (system): 你在为一名学生的家长写学习报告，由老师审阅后发出。只使用给出的事实，不补充事实，不评价学生的人格。输出 JSON，键为 {sections}（只输出这些键），值为该部分的正文，每部分不超过 400 字。overview 概括这段时间做了什么；next 给出 1 到 3 条家长在家可以配合的具体做法。引用学生原话时用「」并逐字照抄给出的金句。不使用事实里没有的数字。不写其他学生的名字。说明文，不用比喻、抒情和套话。 — `{sections}` replaced with the comma-joined `SectionsWithFacts(f)`.

Validation: every requested key present and non-empty, no extra keys, each ≤ 400 runes, `liteweekly.CheckProse(joined, nil, ProseCheck{Corpus: liteparent.Corpus(f), FactsText: liteparent.FactsText(f), OtherNames: otherNames})`. Retry once with the error line `上一次输出未通过校验：{err}。请重新输出。`.

- [ ] **Step 1: Failing composer tests** (scripted provider): valid → sections; missing `writing` key when writing facts exist → retry; extra key `hobbies` → retry; fabricated quote twice → error with 2 attempts.
- [ ] **Step 2: Failing loader test**: plan 1 fixture; seed inside range a finished reading with an `atom_report` moment, a finished writing without a report (loader creates it; assert an `atom_report` row now exists), 2 buckets, an assignment due in range done late, a keyword first seen in range, a second enrolled student named `王小明` → assert facts fields and that `ListClassStudentNames` minus the subject yields `["S …王小明…"]` (use the display names the fixture creates).
- [ ] **Step 3: Implement. Step 4: Run → PASS. Step 5: Commit** `feat(lite-teacher): 家长报告事实汇总与草稿生成`.

---

### Task 3: Teacher endpoints

**Files:** `internal/api/lite_parent_report.go`, `lite_parent_report_test.go`, `lite_teacher_routes.go`.

**Interfaces:**
- `POST /api/v1/lite/teacher/classes/{id}/students/{userId}/parent-reports` body `{rangeStart?, rangeEnd?}` → `201 {"report": ParentReportDTO, "draftError": string|null}`.
- `GET /api/v1/lite/teacher/classes/{id}/students/{userId}/parent-reports` → `{"reports": []ParentReportSummaryDTO}`.
- `GET /api/v1/lite/teacher/classes/{id}/parent-reports` → `{"reports": []ParentReportSummaryDTO}` (with `studentName`).
- `GET /api/v1/lite/teacher/parent-reports/{rid}` → `{"report": ParentReportDTO}`.
- `PATCH /api/v1/lite/teacher/parent-reports/{rid}` body `{"body": {key: text}}` → `{"report"}`.
- `POST /api/v1/lite/teacher/parent-reports/{rid}/redraft` body `{"replaceBody": bool}` → `{"report", "draftError"}`; 409 `already_published` when published.
- `POST /api/v1/lite/teacher/parent-reports/{rid}/publish` → `{"report"}` (idempotent; keeps an existing token).
- `DELETE /api/v1/lite/teacher/parent-reports/{rid}/share` → `{"report"}`.
- `ParentReportDTO{ID, StudentID, ClassID, RangeStart, RangeEnd, Status string; Facts liteparent.Facts; Draft, Body map[string]string; Sections []string; ShareToken, PublishedAt, StudentSeenAt *string; CreatedAt, UpdatedAt string}`; `Sections` = `SectionsWithFacts(facts)`. `ParentReportSummaryDTO` = id, studentId, studentName, rangeStart, rangeEnd, status, publishedAt, shared (bool), createdAt.
- `loadTeacherParentReport(w, r) (sqlc.LiteParentReport, bool)`: parse `{rid}`, load, `assertTeacherOwnsClass(class_id)`, and `IsEnrolledStudent(class_id, user_id)` still true; any failure 404.

- [ ] **Step 1: Failing tests** (scripted provider): generate with default range → 201, facts populated, body == draft, one `llm_call` `lite_parent_report`; generate with provider returning bad JSON twice → 201, `draftError` set, `draft` and `body` null, 2 `llm_call` rows; PATCH unknown key → 400 `invalid_section`; PATCH 2001-rune text → 400; redraft with `replaceBody:false` keeps the edited body; publish → token 32 hex; second publish same token; redraft after publish → 409; revoke → token null; other teacher on every `{rid}` route → 404; a report whose student was removed from the class → 404; invalid range → 400.
- [ ] **Step 2: Run → FAIL. Step 3: Implement + register routes. Step 4: Run → PASS. Step 5: Commit** `feat(lite-teacher): 家长报告生成、编辑、发布与撤销`.

---

### Task 4: Public read, student read, inbox items

**Files:** `internal/api/lite_parent_report_read.go`, `lite_parent_report_read_test.go`, `api.go`, `lite_student_assignments.go`.

**Interfaces:**
- `GET /api/v1/public/parent-reports/{token}` (no auth, registered next to `GET /api/v1/public/reports/{token}`) → `{"report": PublicParentReportDTO}`; `PublicParentReportDTO{StudentName, ClassName, TeacherName, RangeStart, RangeEnd, PublishedAt string; Facts liteparent.Facts; Sections []string; Body map[string]string}` — `Facts` is re-marshalled without any id fields (it has none by construction; keep it that way). Header `X-Robots-Tag: noindex, nofollow, noarchive`. Unknown or revoked token → 404.
- `GET /api/v1/lite/parent-reports/{rid}` (`liteOnly`) → same `PublicParentReportDTO` plus `"id"`; only when `user_id` = session user and status published; else 404.
- `POST /api/v1/lite/parent-reports/{rid}/seen` → 204.
- Inbox (`GET /api/v1/lite/inbox`): append items `{Type:"parent_report", ID, Title: "家长报告（{M月D日}–{M月D日}）", ClassName, PublishedAt, Unread: student_seen_at IS NULL}`; ordering: unread first, then assignments by due date, then reports by `published_at DESC`; `unread` counts both types. Assignment item fields unchanged.

- [ ] **Step 1: Failing tests**: public GET happy path has the header and no `"id"` key anywhere in the JSON (`strings.Contains(body, "\"id\"")` false); revoked → 404; re-publishing after a revoke mints a **new** token and the old token stays 404 (`PublishLiteParentReport` sets `share_token = COALESCE(share_token, $new)`, and revoke sets it NULL, so the COALESCE picks the new value); student GET own published → 200, other student → 404, own draft → 404, own revoked → 200; inbox shows the report unread, `seen` clears it, assignment items still present.
- [ ] **Step 2: Run → FAIL. Step 3: Implement. Step 4: Run → PASS. Step 5: Commit** `feat(lite): 家长报告公开链接、学生查看与收件箱`.

---

### Task 5: Frontend — shared view, public page, student page

**Files:** `api/parentReports.ts` (+ test), `parentReport/range.ts` (+ test), `ParentReportView.tsx`, `ParentReportPoster.tsx`, `PublicParentReportPage.tsx`; modify `routing.ts` (+ test), `rootElementFor.tsx`, `LiteApp.tsx`, `inbox/InboxPanel.tsx`, `api/assignments.ts`.

**Interfaces:**
- `routing.ts`: `{ tab: "parentReportPublic"; token: string }` ↔ `/r/:token` (malformed `/r` → explore, same rule as `/s`); student route `{ tab: "parentReport"; id: string }` ↔ `/parent-reports/:id`.
- `rootElementFor.tsx`: third public case mounting `PublicParentReportPage` for `parentReportPublic`.
- `range.ts`: `defaultRange(today: string): {start, end}` (28 days ending yesterday, pure calendar math on `YYYY-MM-DD`), `rangeLabel(start, end): string` → `8月17日–9月13日`, `todayBeijing(nowMs: number): string`.
- `ParentReportView({ report, variant: "public" | "student" | "teacherPreview" })` — presentational.
- Inbox item union: `type InboxItem = AssignmentInboxItem | ParentReportInboxItem` discriminated by `type`.

- [ ] **Step 1: Pure tests → FAIL → implement → PASS**

```ts
import { describe, expect, it } from "vitest";
import { defaultRange, rangeLabel, todayBeijing } from "./range";

describe("parent report range", () => {
  it("defaults to 28 days ending yesterday", () =>
    expect(defaultRange("2026-09-14")).toEqual({ start: "2026-08-17", end: "2026-09-13" }));
  it("labels a range", () => expect(rangeLabel("2026-08-17", "2026-09-13")).toBe("8月17日–9月13日"));
  it("today in Beijing crosses UTC midnight", () =>
    expect(todayBeijing(Date.UTC(2026, 8, 13, 16, 30))).toBe("2026-09-14"));
});
```

Routing tests: `/r/abc` → `{tab:"parentReportPublic", token:"abc"}`; `/r` → explore; `/parent-reports/p1` → `{tab:"parentReport", id:"p1"}`; round-trips.

- [ ] **Step 2: ParentReportView** — wide layout in the lite report style (`mk-report-*` tokens, `.mk-rp-*` rules): header (学习报告 · {studentName} · {className} · {rangeLabel}), stat tiles from facts (活跃天数, 学习时长 via `formatMinutes` with `—` for -1, 对话轮次, 完成阅读 n 篇, 完成写作 n 篇, 完成项目 n 个, 作业 按时 n / 逾期完成 n / 未完成 n — hide tiles whose source is empty), then each section in `sections` order with its label and body text (`whitespace-pre-wrap`), a 学生原话 block listing moments (quote + item title), finished items lists, interest keywords as chips grouped by field label, footer `由 {teacherName} 发布 · {publishedAt as M月D日}`. Works at 400px.

- [ ] **Step 3: PublicParentReportPage** — fetch with `credentials: "omit"`; 404 → `该报告链接已失效`; loading `加载中…`; action 保存为图片 (icon button, aria-label) → `exportPoster(posterRef.current, "学习报告-{studentName}.png")` with `ParentReportPoster` rendered inside an offscreen **wrapper** (`position:fixed; left:-99999px` on the wrapper div, never on the poster node); poster uses explicit hex colours and system fonts (copy the approach in `reports/ReportPoster.tsx`).

- [ ] **Step 4: Student view** — `LiteApp` route `parentReport` renders `ParentReportView variant="student"` from `GET /api/v1/lite/parent-reports/{id}`; on mount call `seen`. `InboxPanel`: parent report items show kind chip 报告, title, class, `发布于 M月D日`; clicking navigates to `/parent-reports/:id`.

- [ ] **Step 5: Typecheck + tests; commit** `feat(lite): 家长报告页面、公开链接页与学生查看`.

---

### Task 6: Frontend — teacher pages

**Files:** `teacher/ParentReportsPage.tsx`, `ParentReportEditor.tsx`, `GenerateParentReportDialog.tsx`; modify `teacherRouting.ts` (+ test), `LiteTeacherShell.tsx`, `StudentPage.tsx`.

**Interfaces:** routes `{view:"parentReports"}` `/parent-reports`, `{view:"parentReport"; reportId}` `/parent-reports/:rid`.

- [ ] **Step 1: Routing tests → FAIL → implement → PASS.**
- [ ] **Step 2: GenerateParentReportDialog** on StudentPage (button 生成家长报告): lead line `报告会汇总所选日期内的学习数据，并由 AI 起草文字；发布前可以修改。`; 开始日期 / 结束日期 `<input type="date">` defaulting to `defaultRange(todayBeijing(Date.now()))`; 生成 (busy 生成中); server errors `生成失败：{message}`; on success navigate to the editor. StudentPage also lists this student's reports (range, 状态 草稿/已发布, 链接 已开启/已撤销).
- [ ] **Step 3: ParentReportEditor** — two columns on wide screens (stack under 900px): left = one textarea per section (label from `SectionLabels`), autosave on blur via PATCH with `已保存` / `保存失败：{message}`; if `draftError` present show `草稿生成失败：{draftError}` above empty textareas. Actions: 重新生成草稿 (draft only; confirm `重新生成会覆盖当前文字，确定？` → `replaceBody:true`), 发布 (confirm lead `发布后家长可通过链接查看，学生也会在收件箱收到这份报告。`), after publish: link field `{origin}/r/{token}` + 复制链接, 撤销链接 (confirm `撤销后该链接立即失效，学生仍可在应用内查看。`), 重新开启链接 (publish again). Right = `ParentReportView variant="teacherPreview"` bound to the current body.
- [ ] **Step 4: ParentReportsPage** (`/parent-reports`, rail item 家长报告 with lucide `FileText`): class selector, list of reports (学生, 日期范围, 状态, 链接, 创建时间), row → editor. Empty `暂无家长报告`.
- [ ] **Step 5: Typecheck + tests; commit** `feat(lite-teacher): 家长报告编辑与列表页`.

---

### Task 7: Live prompt check, walk, whole-feature safety, push

- [ ] **Step 1: Live check.** `compose_lite_parent_live_test.go` gated on `LIVE_LLM=1` (same construction as plan 3's live test) with realistic facts (4 weeks, 2 readings incl. 《咖啡的旅程》, 1 writing 《一场雨》 with 金句 「雨落在屋檐上像敲鼓」, 1 project, keywords 天气/金融, one late assignment, classmate 王小明). Run once; record attempts and output in the task report; adjust the prompt (never the checks) at most once if the first attempt fails in both of two runs.
- [ ] **Step 2: Walk.** Teacher: generate for a seeded student, edit two sections, publish, copy link; open `/r/{token}` in a logged-out context at 1440×900 and 400×800; export the image and open the PNG to confirm it is not blank; revoke and reload the link (expect `该报告链接已失效`). Student: red dot, inbox item 报告, open, report visible after revoke. Read every screenshot.
- [ ] **Step 3: Safety + push.**

```bash
git diff --diff-filter=D --name-only origin/main..HEAD
git diff --diff-filter=R --name-only origin/main..HEAD
git diff --name-only origin/main..HEAD -- apps/web
pnpm --filter web typecheck && pnpm --filter web test
cd apps/api && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -timeout 1800s
```

First three print nothing; suites green. `git fetch origin`, `git rebase origin/main`, `git push origin HEAD:main` as separate commands.

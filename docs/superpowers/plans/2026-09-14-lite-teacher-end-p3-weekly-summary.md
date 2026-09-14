# Lite Teacher End · Plan 3 — 上周表现总结 and Class 周报 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On each student page the teacher sees 上周表现总结 · 第 N 周（M.D–M.D）: live facts, rule-based 值得表扬 / 需要建议 cards with evidence, and an AI summary with 1–3 support suggestions generated once per (student, week). A class 周报 does the same per (class, week). The teacher can page back through completed weeks.

**Architecture:** Mirrors pro's weekly report (`internal/teacher/weekly.go`, `internal/agent/compose_weekly.go`, `internal/api/teacher_weekly.go`, `apps/web/src/console/ClassWeeklyView.tsx`) on lite data. Week arithmetic in `internal/liteweek`; facts from SQL; cards from a pure package `internal/liteweekly`; one `assess` call per (student|class, week) with code-checked output and one retry; first-open-wins storage. GET never calls a model; POST composes.

**Tech Stack:** Go, PostgreSQL, sqlc, goose, React + TypeScript + Tailwind, vitest.

**Spec:** `docs/superpowers/specs/2026-09-14-lite-teacher-end-design.md` §6. Plans 1 and 2 must be merged first (this plan reads `atom_active_day`, `liteweek`, `lite_assignment*`, `liteassign.Status`, plan 1's student page and routes).

Research notes with exact signatures: `/Users/houyuxin/.claude/jobs/3520028e/tmp/research-p3-p4.md` (if that path is gone, the facts below are sufficient).

## Global Constraints

- Lite must never break pro. Do not modify `internal/teacher/*`, `internal/agent/compose_weekly.go`, `internal/api/teacher_weekly.go`, or `apps/web`. New files only in `internal/agent` (ls first).
- Week = Monday 00:00 → next Monday 00:00, Beijing fixed `+08:00`. Only **completed** weeks are shown; never the current week.
- UI label: latest completed week `上周表现总结 · 第 {ISO week} 周（{M.D}–{M.D}）`; earlier weeks `表现总结 · 第 {ISO week} 周（{M.D}–{M.D}）`. Class: `班级周报 · 第 N 周（…）`. The en dash is `–` (U+2013). ISO week number is computed from the Monday.
- Model calls: `resolved, err := a.routeE(ctx, gateway.ClassAssess)`; `gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{…})`; record with `a.recordLiteLLMCall(ctx, teacherUserID, uuid.Nil, purpose, resolved, res.Usage)`, purposes `lite_student_weekly` / `lite_class_weekly`. Record every attempt, including the retry and failed attempts that returned text.
- Use `detachedModelCtx(r)` for the POST handlers (150 s, survives client disconnect).
- Prose output checks (code, not prompt): every `evidenceCode` is one of that week's card codes; every quoted span inside 「…」 or “…” appears verbatim in the corpus (the week's 金句 + item titles + keyword texts); every run of ASCII digits in the prose appears in the facts text; no display name of another student in the class appears. Retry the call once on a failed check; after the second failure store nothing and return `prose: null` with `proseError`.
- Failure is never a wall: facts and cards always render; prose area shows `总结生成失败：{后台原话}`.
- Storage first-open-wins: `INSERT … ON CONFLICT DO NOTHING`, then re-read and return the stored row.
- Cards and evidence strings are produced in Go, never by the model.
- UI copy rules per AGENTS.md § 界面文案怎么写.
- Tests: logic only; `LIVE_LLM=1` run of both prompts once before shipping (Task 6).
- Go tests `cd apps/api && CGO_ENABLED=0 go test ./internal/... -run <Name> -timeout 1800s`; `make sqlc`; frontend `pnpm --filter lite-web typecheck && pnpm --filter lite-web test`. `Deps.Pool` only has `Begin`: tests query through the concrete pool from the test DB.
- Commits: specific files; trailer `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

---

## File Structure

**Create**
- `apps/api/internal/liteweek/completed.go` (+ `completed_test.go`)
- `apps/api/internal/liteweekly/cards.go` (+ `cards_test.go`) — facts struct, rule cards, evidence
- `apps/api/internal/liteweekly/check.go` (+ `check_test.go`) — prose checks
- `apps/api/internal/store/migrations/0150_lite_weekly_prose.sql` (next free number)
- `apps/api/internal/store/queries/lite_weekly.sql`
- `apps/api/internal/agent/compose_lite_weekly.go` (+ `compose_lite_weekly_test.go`)
- `apps/api/internal/api/lite_weekly.go` (+ `lite_weekly_test.go`)
- `apps/lite-web/src/api/weekly.ts` (+ `weekly.test.ts`)
- `apps/lite-web/src/teacher/WeekSummaryCard.tsx`, `ClassWeeklyPage.tsx`, `weekNav.ts` (+ `weekNav.test.ts`)

**Modify**
- `apps/api/internal/api/lite_teacher_routes.go`
- `apps/lite-web/src/teacher/StudentPage.tsx`, `ClassPage.tsx`, `LiteTeacherShell.tsx`, `teacherRouting.ts` (+ test)

---

### Task 1: Completed-week arithmetic

**Files:** Create `apps/api/internal/liteweek/completed.go`, `completed_test.go`.

**Interfaces:**
- Consumes: `liteweek.Beijing`, `liteweek.WeekStart` (plan 1).
- Produces: `LatestCompleted(now time.Time) time.Time` (Monday 00:00 Beijing of last week); `IsCompleted(weekStart, now time.Time) bool`; `ParseWeekStart(s string, now time.Time) (time.Time, error)` (accepts RFC3339 or `YYYY-MM-DD`; must be a Beijing Monday 00:00 and completed; empty → `LatestCompleted(now)`); `Label(weekStart time.Time) string` (`第 %d 周（%d.%d–%d.%d）`, end = weekStart+6 days); `ErrBadWeek` sentinel.

- [ ] **Step 1: Failing tests**

```go
package liteweek

import (
	"errors"
	"testing"
	"time"
)

func TestLatestCompleted(t *testing.T) {
	// Wednesday 2026-09-16 10:00 Beijing → last week started Monday 2026-09-07.
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, Beijing)
	want := time.Date(2026, 9, 7, 0, 0, 0, 0, Beijing)
	if got := LatestCompleted(now); !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// Monday 00:00:30 Beijing: the week that just ended is the latest completed.
	now = time.Date(2026, 9, 14, 0, 0, 30, 0, Beijing)
	if got := LatestCompleted(now); !got.Equal(want) {
		t.Fatalf("monday: got %v want %v", got, want)
	}
}

func TestParseWeekStart(t *testing.T) {
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, Beijing)
	if got, err := ParseWeekStart("", now); err != nil || !got.Equal(LatestCompleted(now)) {
		t.Fatalf("empty: %v %v", got, err)
	}
	if got, err := ParseWeekStart("2026-08-31", now); err != nil || got.Day() != 31 {
		t.Fatalf("date form: %v %v", got, err)
	}
	for _, bad := range []string{"2026-09-14", "2026-09-08", "garbage", "2026-09-07T00:00:00Z"} {
		// current week; a Tuesday; unparsable; UTC midnight is 08:00 Beijing, not a Beijing Monday 00:00
		if _, err := ParseWeekStart(bad, now); !errors.Is(err, ErrBadWeek) {
			t.Errorf("%s: want ErrBadWeek, got %v", bad, err)
		}
	}
}

func TestLabel(t *testing.T) {
	got := Label(time.Date(2026, 9, 7, 0, 0, 0, 0, Beijing))
	if got != "第 37 周（9.7–9.13）" {
		t.Fatalf("label = %q", got)
	}
}
```

Check the ISO week of 2026-09-07 with `time.Time.ISOWeek()` and fix the expected number if it differs; the test must assert the value `ISOWeek` returns.

- [ ] **Step 2: Run → FAIL.** `cd apps/api && CGO_ENABLED=0 go test ./internal/liteweek/ -timeout 120s`

- [ ] **Step 3: Implement**

```go
package liteweek

import (
	"errors"
	"fmt"
	"time"
)

var ErrBadWeek = errors.New("liteweek: not a completed Beijing week start")

// LatestCompleted is the Monday 00:00 (Beijing) of the last full week before now.
func LatestCompleted(now time.Time) time.Time {
	return WeekStart(now).AddDate(0, 0, -7)
}

// IsCompleted reports whether the week starting at weekStart has ended by now.
func IsCompleted(weekStart, now time.Time) bool {
	return !weekStart.AddDate(0, 0, 7).After(now)
}

// ParseWeekStart reads a ?weekStart value. Empty means the latest completed week.
func ParseWeekStart(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return LatestCompleted(now), nil
	}
	var t time.Time
	var err error
	if len(s) == len("2006-01-02") {
		t, err = time.ParseInLocation("2006-01-02", s, Beijing)
	} else {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		return time.Time{}, ErrBadWeek
	}
	t = t.In(Beijing)
	if !t.Equal(WeekStart(t)) || !IsCompleted(t, now) {
		return time.Time{}, ErrBadWeek
	}
	return t, nil
}

// Label names a week: 第 37 周（9.7–9.13）.
func Label(weekStart time.Time) string {
	ws := weekStart.In(Beijing)
	_, wk := ws.ISOWeek()
	end := ws.AddDate(0, 0, 6)
	return fmt.Sprintf("第 %d 周（%d.%d–%d.%d）", wk, int(ws.Month()), ws.Day(), int(end.Month()), end.Day())
}
```

- [ ] **Step 4: Run → PASS.** **Step 5: Commit** `feat(lite): 已结束周的计算与标签`.

---

### Task 2: Facts, rule cards, prose checks (pure)

**Files:** Create `apps/api/internal/liteweekly/cards.go`, `cards_test.go`, `check.go`, `check_test.go`.

**Interfaces:**
- Produces:

```go
type Item struct{ Kind, Title string }            // kind: reading|writing|project
type Moment struct{ Quote, ItemTitle string }

type StudentWeek struct {
	UserID, Name        string
	ActiveDays          int
	Minutes             int  // -1 = no buckets in or before this week
	Turns               int
	PrevActiveDays      int
	Finished            []Item
	AssignmentsDone     int  // due in week, done on time
	AssignmentsLate     int  // due in week, done late
	AssignmentsOverdue  int  // due in week, unfinished at week end
	Stalled             []Item // unfinished, last activity > 7 days before week end
	NewKeywords         []string
	Moments             []Moment
}

type Card struct {
	Kind     string // "praise" | "watch"
	Code     string // tag code
	Label    string // 值得表扬 / 需要建议 tag label
	Evidence string
}

func Cards(s StudentWeek) (watch *Card, praise *Card)
func FactsText(s StudentWeek, weekLabel string) string   // the text the digit check and the prompt both use
func Corpus(s StudentWeek) string                         // moments + titles + keywords, for the quote check

type ProseCheck struct {
	AllowedCodes []string
	Corpus       string
	FactsText    string
	OtherNames   []string
}
func CheckProse(text string, codes []string, c ProseCheck) error  // text = all prose fields joined; codes = evidenceCodes used
```

- [ ] **Step 1: Failing card tests** — one test per tag and the precedence rules from spec §6.2:

| Card | Code | Label | Fires when | Evidence template |
|---|---|---|---|---|
| watch | `never_used` | 本周未使用 | `ActiveDays == 0` | `本周 0 天有学习记录；上一周 {PrevActiveDays} 天。` |
| watch | `overdue` | 作业逾期 | `AssignmentsOverdue >= 1` | `本周到期的作业中有 {AssignmentsOverdue} 份未完成。` |
| watch | `dropped_off` | 活跃下降 | `ActiveDays <= PrevActiveDays-2` | `活跃天数 上一周 {PrevActiveDays} 天 → 本周 {ActiveDays} 天。` |
| watch | `stalled` | 进度停滞 | `len(Stalled) >= 1` | `「{Stalled[0].Title}」超过 7 天没有进展。` (+ `等 {n} 项` when n>1) |
| praise | `finished_on_time` | 按时完成 | `AssignmentsDone >= 1 && AssignmentsOverdue == 0` | `本周到期的作业按时完成 {AssignmentsDone} 份。` |
| praise | `new_interest` | 新的兴趣 | `len(NewKeywords) >= 1` | `兴趣树新增关键词：{join(NewKeywords[:3], "、")}。` |
| praise | `more_active` | 更加投入 | `ActiveDays >= 3 && ActiveDays >= PrevActiveDays+2` | `活跃天数 上一周 {PrevActiveDays} 天 → 本周 {ActiveDays} 天。` |

At most one watch (table order) and one praise (table order); `never_used` → `praise == nil`.

```go
func TestNeverUsedSuppressesPraise(t *testing.T) {
	w, p := Cards(StudentWeek{ActiveDays: 0, PrevActiveDays: 4, NewKeywords: []string{"金融"}})
	if w == nil || w.Code != "never_used" || p != nil {
		t.Fatalf("watch=%+v praise=%+v", w, p)
	}
	if w.Evidence != "本周 0 天有学习记录；上一周 4 天。" {
		t.Fatalf("evidence = %q", w.Evidence)
	}
}

func TestOverdueBeatsDroppedOff(t *testing.T) {
	w, _ := Cards(StudentWeek{ActiveDays: 1, PrevActiveDays: 5, AssignmentsOverdue: 2})
	if w.Code != "overdue" {
		t.Fatalf("code = %s", w.Code)
	}
}

func TestOnTimeNeedsNoOverdue(t *testing.T) {
	_, p := Cards(StudentWeek{ActiveDays: 3, AssignmentsDone: 1, AssignmentsOverdue: 1})
	if p != nil && p.Code == "finished_on_time" {
		t.Fatal("finished_on_time fired with an overdue assignment")
	}
}

func TestMoreActiveThreshold(t *testing.T) {
	if _, p := Cards(StudentWeek{ActiveDays: 3, PrevActiveDays: 1}); p == nil || p.Code != "more_active" {
		t.Fatalf("3 vs 1: %+v", p)
	}
	if _, p := Cards(StudentWeek{ActiveDays: 2, PrevActiveDays: 0}); p != nil {
		t.Fatalf("2 vs 0 should not fire: %+v", p)
	}
}

func TestStalledEvidenceCountsExtra(t *testing.T) {
	w, _ := Cards(StudentWeek{ActiveDays: 2, PrevActiveDays: 2, Stalled: []Item{{"writing", "一场雨"}, {"reading", "咖啡"}}})
	if w.Code != "stalled" || w.Evidence != "「一场雨」等 2 项超过 7 天没有进展。" {
		t.Fatalf("%+v", w)
	}
}
```

Write the remaining per-tag positive tests (`dropped_off`, `finished_on_time`, `new_interest`) the same way. Set the stalled template accordingly: one item → `「{t}」超过 7 天没有进展。`; n>1 → `「{t}」等 {n} 项超过 7 天没有进展。`.

- [ ] **Step 2: Failing check tests**

```go
func TestCheckProse(t *testing.T) {
	c := ProseCheck{
		AllowedCodes: []string{"overdue", "new_interest"},
		Corpus:       "一场雨\n雨落在屋檐上像敲鼓\n金融",
		FactsText:    "第 37 周（9.7–9.13） 活跃 3 天 学习 95 分钟 逾期 1 份",
		OtherNames:   []string{"王小明"},
	}
	ok := "本周有 1 份作业逾期。她写下「雨落在屋檐上像敲鼓」，可以请她继续完成《一场雨》。"
	if err := CheckProse(ok, []string{"overdue"}, c); err != nil {
		t.Fatalf("valid prose rejected: %v", err)
	}
	bad := map[string]struct {
		text  string
		codes []string
	}{
		"unknown code":     {"本周有 1 份作业逾期。", []string{"stalled"}},
		"fabricated quote": {"她写下「雨是天空的眼泪」。", []string{"overdue"}},
		"curly quote":      {"她说“我不想写”。", []string{"overdue"}},
		"stray digit":      {"本周学习 120 分钟。", []string{"overdue"}},
		"other student":    {"可以和王小明一起讨论。", []string{"overdue"}},
	}
	for name, b := range bad {
		if err := CheckProse(b.text, b.codes, c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
```

Digit rule: extract runs `[0-9]+` from the prose; each must appear as a whole run in `FactsText` (so `12` does not pass because `120` exists — compare against the set of runs in `FactsText`).

- [ ] **Step 3: Run → FAIL. Step 4: Implement** `cards.go` and `check.go` exactly per the tables and rules above (`FactsText` renders every numeric field and the label; `Corpus` joins moments' quotes, finished and stalled titles, and keywords with newlines). **Step 5: Run → PASS. Step 6: Commit** `feat(lite-teacher): 周总结的规则卡片与输出校验`.

---

### Task 3: Tables, fact queries, loaders

**Files:** Create `0150_lite_weekly_prose.sql`, `queries/lite_weekly.sql`, and in `apps/api/internal/api/lite_weekly.go` the loaders (handlers come in Task 5); test `lite_weekly_test.go` (loader tests).

**Interfaces:**
- Produces: `(*API).loadLiteStudentWeek(ctx, classID, userID uuid.UUID, weekStart time.Time) (liteweekly.StudentWeek, error)`; `(*API).loadLiteClassWeek(ctx, classID uuid.UUID, weekStart time.Time) ([]liteweekly.StudentWeek, error)`; sqlc `InsertLiteStudentWeeklyProse`, `GetLiteStudentWeeklyProse`, `InsertLiteClassWeeklyProse`, `GetLiteClassWeeklyProse`.

- [ ] **Step 1: Migration**

```sql
-- +goose Up
-- 教师端（lite）周总结的文字。数字每次现算，只存模型写的字；每个 (学生, 周) 只生成一次。
CREATE TABLE lite_student_weekly_prose (
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  week_start date NOT NULL,
  body       jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, week_start)
);
CREATE TABLE lite_class_weekly_prose (
  class_id   uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  week_start date NOT NULL,
  body       jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (class_id, week_start)
);

-- +goose Down
DROP TABLE lite_class_weekly_prose;
DROP TABLE lite_student_weekly_prose;
```

- [ ] **Step 2: Queries** (`lite_weekly.sql`) — parameters `user_ids uuid[]`, `class_id uuid`, `week_start timestamptz`, `week_end timestamptz`, `prev_start timestamptz`, `week_start_day date`, `week_end_day date`, `prev_start_day date`:
  - `ListLiteWeekActivity :many` — per user: active days in week and in previous week (buckets with seconds>0 ∪ student message dates, `AT TIME ZONE 'Asia/Shanghai'`), seconds in week from buckets, `has_any_bucket_by_week_end` (bucket with day < week_end_day), turns in week.
  - `ListLiteWeekFinished :many` — per user: kind, title, finished_at within [week_start, week_end) across reading/writing/project (project uses `pbl_project.finished_at`).
  - `ListLiteWeekAssignmentStates :many` — per user: `started_at, due_at, finished_at` for non-archived assignments of `class_id` with `due_at` in the week. Status at week end = `liteassign.Status(started, finishedAt, dueAt, weekEnd)` — note `now = weekEnd`, so an item finished after the week still counts as overdue for that week.
  - `ListLiteWeekStalled :many` — per user: kind, title for atoms not finished with `last_activity_at < week_end - interval '7 days'` and `created_at < week_end`.
  - `ListLiteWeekNewKeywords :many` — per user: `text_zh` with `first_seen_at` in the week.
  - `ListLiteWeekMoments :many` — per user: `atom_report.report->'moments'` and item title for items finished in the week (unmarshal in Go; take quotes as-is — they were checked against her words when the report was built).
  - Prose: `InsertLiteStudentWeeklyProse :exec` (`ON CONFLICT (user_id, week_start) DO NOTHING`), `GetLiteStudentWeeklyProse :one`, same pair for class.

Check column names against migrations (`interest_keyword.text_zh`, `first_seen_at`; `atom_report.report`).

- [ ] **Step 3: Failing loader test** — seed for one student in the plan 1 fixture, all inside last completed week (compute with `liteweek.LatestCompleted(time.Now())`): a bucket of 600 s on two days, one student message on a third day, a reading finished in the week with an `atom_report` row whose `moments` contains `{"quote":"雨落在屋檐上","where":""}`, an assignment due in the week never started, a writing with `last_activity_at` 10 days before week end, a keyword first seen in the week. Assert the loaded `StudentWeek`: `ActiveDays==3`, `Minutes==20`, `len(Finished)==1`, `AssignmentsOverdue==1`, `len(Stalled)==1`, `NewKeywords==["…"]`, `Moments[0].Quote=="雨落在屋檐上"`. A student with no buckets at all → `Minutes == -1`.

- [ ] **Step 4: Run → FAIL. Step 5: Implement loaders** (batched queries keyed by user id; the student loader calls the class loader's code path with one id). **Step 6: Run → PASS. Step 7: Commit** `feat(lite-teacher): 周总结的事实数据`.

---

### Task 4: Composer

**Files:** Create `apps/api/internal/agent/compose_lite_weekly.go`, `compose_lite_weekly_test.go`.

**Interfaces:**
- Consumes: `gateway.Provider`, `gateway.Resolved`, `gateway.Collect`, `liteweekly.*`.
- Produces:

```go
type LiteStudentWeeklyProse struct {
	Summary     string                 `json:"summary"`
	Suggestions []LiteWeeklySuggestion `json:"suggestions"`
}
type LiteWeeklySuggestion struct {
	Text         string `json:"text"`
	EvidenceCode string `json:"evidenceCode"`
}
type LiteClassWeeklyProse struct {
	Comment string                    `json:"comment"`
	Cards   []LiteClassWeeklyCardProse `json:"cards"`
}
type LiteClassWeeklyCardProse struct {
	UserID string `json:"userId"`
	Lead   string `json:"lead"`
	Action string `json:"action"`
}

// Attempt is one model call's usage and raw text, returned so the caller records every call.
type Attempt struct {
	Usage gateway.ChatUsage
	Text  string
	Err   error
}

func ComposeLiteStudentWeekly(ctx context.Context, prov gateway.Provider, r gateway.Resolved, s liteweekly.StudentWeek, weekLabel string, cards []liteweekly.Card, otherNames []string) (LiteStudentWeeklyProse, []Attempt, error)
func ComposeLiteClassWeekly(ctx context.Context, prov gateway.Provider, r gateway.Resolved, className string, weekLabel string, students []liteweekly.StudentWeek, cards map[string][]liteweekly.Card, allNames []string) (LiteClassWeeklyProse, []Attempt, error)
```

Rules:
- Student prompt (system): 你在给老师写一名学生上一周的学习总结。只使用给出的事实，不补充事实。输出 JSON `{"summary":"","suggestions":[{"text":"","evidenceCode":""}]}`。summary 不超过 150 字；suggestions 1 到 3 条，每条是老师下周可以做的一件具体的事，evidenceCode 必须是给出的卡片代码之一；没有卡片时 suggestions 为空数组。引用学生原话时用「」且逐字照抄给出的金句。不使用给出事实里没有的数字。不写其他学生的名字。说明文，不用比喻和抒情。
  User message: `liteweekly.FactsText(s, weekLabel)` + card list (`code · label · evidence`) + 金句 list.
- Validation after parse: summary non-empty and ≤ 150 runes; 0–3 suggestions (≤ 3; 0 only when there are no cards; ≥1 when there are cards); each text ≤ 120 runes; `liteweekly.CheckProse(summary + all suggestion texts, codes, ProseCheck{…})`.
- On parse or validation failure retry once (same messages plus one user line: `上一次输出未通过校验：{err}。请重新输出。`). Return all attempts.
- Class prompt mirrors pro's `weeklySystemPrompt` rules (one card prose per flagged student keyed by userId, no additions/omissions) with lite wording; comment ≤ 300 runes; lead ≤ 120; action ≤ 200; `CheckProse` over comment+leads+actions with `OtherNames` = empty (all class names are allowed in the class report) but the corpus/digit checks still apply using a class facts text (class size, active students, minutes, turns, finished counts, assignment completion rate, and each flagged student's evidence).

- [ ] **Step 1: Failing tests with a scripted provider** (see how `internal/agent` tests fake `gateway.Provider` — grep `compose_weekly_test.go`): (a) valid first reply → 1 attempt, prose returned; (b) first reply with a fabricated quote, second valid → 2 attempts, second prose returned, and the second request's messages include `上一次输出未通过校验`; (c) two invalid replies → error, 2 attempts; (d) reply in a ```json fence parses.

- [ ] **Step 2: Run → FAIL. Step 3: Implement. Step 4: Run → PASS. Step 5: Commit** `feat(lite-teacher): 周总结文字生成与重试`.

---

### Task 5: Weekly endpoints

**Files:** Modify `apps/api/internal/api/lite_weekly.go` (handlers), `lite_weekly_test.go`, `lite_teacher_routes.go`.

**Interfaces:**
- Produces:
  - `GET /api/v1/lite/teacher/classes/{id}/students/{userId}/weekly?weekStart=` → `{"weekStart": "YYYY-MM-DD", "weekLabel", "title", "isLatest": bool, "facts": {activeDays, minutes, turns, prevActiveDays, finished:[{kind,title}], assignmentsDone, assignmentsLate, assignmentsOverdue, stalled:[…], newKeywords:[…], moments:[{quote,itemTitle}]}, "cards": [{kind,code,label,evidence}], "prose": LiteStudentWeeklyProse|null, "proseReady": bool}`. `title` is the full UI title (`上周表现总结 · …` when latest, `表现总结 · …` otherwise).
  - `POST …/weekly/prose?weekStart=` → same shape plus `"proseError": string|null`. If a row exists, return it without calling the model.
  - `GET /api/v1/lite/teacher/classes/{id}/weekly?weekStart=` → `{"weekStart","weekLabel","title","isLatest","stats":{classSize,activeStudents,minutes,turns,finished,assignmentRate},"praise":[CardDTO+userId,name],"watch":[…],"prose":LiteClassWeeklyProse|null,"proseReady"}`; `assignmentRate` = done+done_late over assignments due in the week × recipients, integer percent, `-1` when none.
  - `POST /api/v1/lite/teacher/classes/{id}/weekly/prose?weekStart=` → same plus `proseError`.
  - Bad `weekStart` → 400 `invalid_week` `请选择已经结束的一周`.

- [ ] **Step 1: Failing handler tests** (plan 1 fixture + a scripted provider via `New(Deps{…, Provider: fake})` as in `liteHandlerWithProvider`; seed facts as in Task 3):
  - GET does not call the provider (fake counts calls → 0) and returns `proseReady:false`, cards derived from the seed.
  - POST with valid scripted JSON → 200, `prose.summary` set; `llm_call` has 1 row purpose `lite_student_weekly` with `user_id` = teacher; second POST → provider still called once total, same prose.
  - POST with a fabricated quote twice → `prose:null`, `proseError` non-empty, 2 `llm_call` rows, nothing stored (a following GET shows `proseReady:false`).
  - `weekStart` = current week → 400.
  - Other teacher → 404; student → 403.
  - Class: POST valid → comment stored once per (class, week).

- [ ] **Step 2: Run → FAIL. Step 3: Implement** (handlers use `authTeacherStudent` / `assertTeacherOwnsClass`, `liteweek.ParseWeekStart(r.URL.Query().Get("weekStart"), time.Now())`, loaders from Task 3, `liteweekly.Cards`, composers from Task 4, `recordLiteLLMCall` for every attempt, insert then re-read). **Step 4: Run → PASS. Step 5: Commit** `feat(lite-teacher): 上周表现总结与班级周报接口`.

---

### Task 6: Frontend, live prompt check, walk, push

**Files:** Create `apps/lite-web/src/api/weekly.ts` (+ test), `teacher/weekNav.ts` (+ test), `teacher/WeekSummaryCard.tsx`, `teacher/ClassWeeklyPage.tsx`; modify `StudentPage.tsx`, `ClassPage.tsx`, `LiteTeacherShell.tsx`, `teacherRouting.ts` (+ test).

**Interfaces:**
- `weekNav.ts`: `shiftWeek(weekStart: "YYYY-MM-DD", days: number): string` (pure date arithmetic on the calendar date, no time zones), `canGoNext(weekStart, isLatest): boolean`.
- `teacherRouting.ts`: `{view:"classWeekly"; classId}` ↔ `/classes/:classId/weekly` (must not be parsed as a student id — `weekly` is a reserved segment after the class id).

- [ ] **Step 1: Pure tests → FAIL → implement → PASS**

```ts
import { describe, expect, it } from "vitest";
import { canGoNext, shiftWeek } from "./weekNav";

describe("weekNav", () => {
  it("steps back a week across a month boundary", () => expect(shiftWeek("2026-09-07", -7)).toBe("2026-08-31"));
  it("steps forward", () => expect(shiftWeek("2026-08-31", 7)).toBe("2026-09-07"));
  it("no next from the latest week", () => expect(canGoNext("2026-09-07", true)).toBe(false));
});
```

Add `/classes/c1/weekly` to the routing test table and a case asserting `/classes/c1/students/u1` still parses as a student.

- [ ] **Step 2: WeekSummaryCard** on `StudentPage` (above the item lists): title from the response; ‹ 上一周 / 下一周 › icon buttons (next disabled when `isLatest`); fact tiles (活跃天数, 学习时长 via `formatMinutes`, 对话轮次, 完成 n 项, 作业 按时/逾期完成/逾期); cards as two groups 值得表扬 / 需要建议 with evidence text; prose block: `summary` then numbered 建议 list each followed by its card label in muted text. Prose lifecycle mirrors pro `ClassWeeklyView.tsx:148-160`: when `proseReady` is false, POST once per (student, week) per mount (a ref latch keyed by `${userId}:${weekStart}`), show `总结生成中` while pending; on `proseError` show `总结生成失败：{proseError}` and a 重试 button that POSTs again. No cards and no facts activity → `该周没有学习记录`.

- [ ] **Step 3: ClassWeeklyPage** at `/classes/:id/weekly`: title `班级周报 · …`, stat tiles (本周活跃学生 `{active}/{size} 人`, 学习时长, 对话轮次, 完成项目数, 作业完成率 `{n}%` or `—`), 班级点评 (comment, same lifecycle), 值得表扬 / 需要建议 student cards (name, tag label, evidence, lead, action; click → student page). Add a 周报 button on `ClassPage`'s header.

- [ ] **Step 4: Typecheck + tests + commit** `feat(lite-teacher): 上周表现总结卡片与班级周报页`.

- [ ] **Step 5: Live prompt check.** Write a `LIVE_LLM=1`-gated test `apps/api/internal/agent/compose_lite_weekly_live_test.go` (skip unless `os.Getenv("LIVE_LLM") == "1"`; build the real provider and resolved `assess` class the same way the existing live tests in `internal/gateway` do) that composes one student week and one class week from realistic Chinese facts (a student who finished 《一场雨》 with the 金句 「雨落在屋檐上像敲鼓」, one overdue assignment, new keyword 天气). Run it once: `cd apps/api && LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/agent -run TestLiveLiteWeekly -v -timeout 600s`. Record in the task report: attempts per compose, whether the first attempt passed checks, the output text. If the first attempt fails checks in both runs, adjust the prompt (not the checks) once and re-run; do not loosen a check to make a run pass.

- [ ] **Step 6: Walk + safety + push.** Seed a student with activity in last week; screenshot the student page card (1440×900, 400×800) and the class weekly page; read each screenshot. Then:

```bash
git diff --diff-filter=D --name-only origin/main..HEAD
git diff --diff-filter=R --name-only origin/main..HEAD
git diff --name-only origin/main..HEAD -- apps/web apps/api/internal/teacher apps/api/internal/agent/compose_weekly.go apps/api/internal/api/teacher_weekly.go
pnpm --filter web typecheck && pnpm --filter web test
cd apps/api && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -timeout 1800s
```

Expected: the first three print nothing. Then `git fetch origin`, `git rebase origin/main`, `git push origin HEAD:main` as separate commands.

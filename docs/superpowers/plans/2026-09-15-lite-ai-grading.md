# Lite 一键AI批改 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A teacher sets a rubric on a writing homework, queues AI grading for every submitted student, reviews and edits each draft grading, and sends it; the student reads sent gradings on her finished-writing page and in her inbox.

**Architecture:** The rubric lives in the writing homework payload (`liteassign`). A new pure package `internal/litegrade` owns the prompt, the reply parser and `Check`, which rejects any result that quotes words she did not write, leaves the rubric scale, or breaks the 3–5 point shape. A `lite_grading` row per submitted version (migration 0154) moves `queued → running → draft | failed → sent`; a river job (`MaxAttempts: 1`) runs the model through `a.routeE(ctx, gateway.ClassReview)` with one retry that carries the failure reasons. Teacher handlers queue, read, edit (shape check only) and send; students only ever read `sent` rows.

**Tech Stack:** Go (`net/http`, pgx v5, sqlc v1.27.0, goose, river v0.39.0), PostgreSQL, React + Vite + TypeScript + Tailwind (lite-web), vitest (logic only), Playwright (one-off screenshots).

**Spec:** `docs/superpowers/specs/2026-09-15-lite-homework-grading-and-finished-writing-design.md` (Part B only: B1–B6). Part A plan (names consumed here): `docs/superpowers/plans/2026-09-15-lite-finished-writing-versions.md`. Exploration notes: `.superpowers/tmp/explore-homework-grading.md`.

## Deviations from the spec (forced by the code)

1. **Single-writing grading route is class-scoped.** `authTeacherStudent` reads `{id}` (class) and `{userId}`; Part A made the same change for versions. Route: `POST /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}/gradings`.
2. **`lite_grading` has two more columns: `user_id` (the student) and `class_id`.** `/lite/teacher/gradings/{gid}` has no class in its path; the gate is `assertTeacherOwnsClass(class_id)` + `IsEnrolledStudent(class_id, user_id)`. `user_id` also keys the student inbox query.
3. **Dimension names are capped at 40 runes, not 20.** The spec's own English defaults are 22 (`Coherence and Cohesion`) and 30 (`Grammatical Range and Accuracy`) characters.
4. **One extra endpoint: `POST /api/v1/lite/teacher/gradings/{gid}/regrade`.** 重新批改 in the grading view regrades that row's version; the single-writing endpoint always grades the latest version, which can differ once she has submitted again.
5. **`PATCH /api/v1/lite/teacher/gradings/{gid}` body is `{content?}`.** Every PATCH sets `reviewed_at` (the spec: "set when the teacher saves or presses 标记已审阅"), so 标记已审阅 is a PATCH without content and the `reviewed` flag is not needed.
6. **Inbox grading items use the existing discriminator:** `{type: "grading", id, atomId, writingTitle, sentAt, unread}`. `kind` already means the assignment kind on inbox rows, and every row already carries `unread`.
7. **`litegrade.Check` receives the person-judging check as a function** (`Input.PersonJudging`). `personDirectedVerdict` and its prefix list live in package `api`; `litegrade` cannot import `api`. The worker passes `personDirectedVerdict`, and an internal test pins that wiring.
8. **The rubric reaches PATCH as a top-level `rubric` field.** A `rubric` key inside a PATCH `payload` is ignored and the stored rubric is carried over, so a form that resends unchanged settings after a student started is not a settings change. `"rubric": null` resets to the default. Create takes the rubric inside `payload`.
9. **A `running` row older than 15 minutes is marked `failed` ("批改超时：任务未完成") when the teacher reads it.** With `MaxAttempts: 1`, a process that dies mid-job leaves the row `running` forever; river will not run it again.
10. **The river job timeout is 6 minutes** (`Timeout` on the worker). River's default is 1 minute, shorter than two 150-second model calls.
11. **No queue → 503 `grading_queue_unavailable` 「批改队列未启动」.** `Deps.River` is nil when river fails to start; a queued row nobody runs would read 批改中 forever.
12. **The grading view is its own teacher route `/gradings/:gid`**, so it works for a homework row (back to the assignment) and for a writing graded from the item page (back to the item).

## Open questions (ruled here; revisit with the owner if wrong)

1. **Comment language:** comments, point texts and actions are written in the writing's language (`writing.lang`); an English essay gets English feedback.
2. **Editing a sent grading:** allowed. Saving a `sent` row updates it in place, sets `sent_at = now()` and clears `student_seen_at` (spec B6: "re-sent after edits updates in place and becomes unseen again"). The button reads 保存并发送.
3. **AI points must carry a quote.** The content shape allows `quote: null` (for teacher points); the owner decision says each point quotes her sentence, so `Check` rejects an AI point with no quote.
4. **The person-judging check also runs on the overall comment and dimension comments**, not only on points. It is the same prefix list; a comment 「你很懒」 is as wrong in the overall comment as in a point.
5. **The rewrite guard checks 「」 only** (spec B4). Straight or curly English quotes are not checked; the prompt tells the model to quote her words with 「」 in both languages.
6. **A regrade that fails leaves the row `failed`** with the previous `content` still stored but not editable (PATCH accepts `draft` and `sent` only). The teacher presses 重新批改 again.
7. **A student with no entitlement:** the worker marks the row failed with `当前没有可用额度` (the `ErrNotEntitled` message).
8. **The student's 老师批改 panel is absent when she has no sent grading**, the same as before Part B.

## Global Constraints

- Edition: lite only. Pro behaviour must not change. `migrations/`, `queries/`, `store/sqlc/` are shared with pro: add new files; never rename or reuse a query name.
- Migration number: `0154` (latest on this branch is `0153_lite_writing_versions.sql`).
- Rubric: `{scale: "letter" | "points", max: 1..100 (points only), dimensions: [{name ≤40 runes, note ≤200 runes}] 1..6, names unique, focus ≤500 runes}`. Letter grades `A+ A A- B+ B B- C+ C C- D`. Points grades are integer strings `0..max`.
- Default rubric (`liteassign.DefaultRubric(lang)`, scale `letter`): zh `内容 / 结构 / 语言 / 书写规范`; en `Task Response / Coherence and Cohesion / Lexical Resource / Grammatical Range and Accuracy`. A homework without `rubric` reads as the default for its `lang`.
- Grading statuses: `queued / running / draft / failed / sent` (DB CHECK). Content: `{overall: {grade, comment}, dimensions: [{name, grade, comment}], points: [{kind: "good"|"issue", quote: string|null, text, action: string|null, source: "ai"|"teacher"}]}`.
- Model calls: only in the worker, `a.routeE(ctx, gateway.ClassReview)`, metered with `a.recordLiteLLMCall(ctx, studentID, atomID, "teacher_grading", resolved, usage)` for every call, entitlement via `a.studentEntitled(ctx, studentID)`. River `MaxAttempts: 1`; the worker retries once with the failure reasons appended, then marks the row `failed` with the reasons as `error`.
- Students never see a row whose status is not `sent`; they never see `ai`, `error`, `status` or `requested_by`.
- Every teacher route checks the teacher owns the class (404 otherwise). Assignment routes go through `loadTeacherAssignment`; `{gid}` routes through `loadTeacherGrading`.
- Copy, verbatim: 「由 AI 起草，老师审阅后发送」, 「针对 v{n}」, 「批改失败：{error}」, 「重新批改会覆盖当前修改」. Buttons: 一键AI批改, 重试失败, 发送全部已审阅, 保存, 标记已审阅, 发送, 重新批改, 退回修改. Row statuses: 未提交 / 待批改 / 批改中 / 草稿 / 已审阅 / 已发送 / 批改失败. Other copy follows AGENTS.md §界面文案怎么写 (nouns for labels, `动词+失败：{后台原话}` for errors, no literary prose; rule 10 applies to comments and commit messages).
- Tests: logic tests only. Go handler tests use the testcontainers harness (`newAPITestPool`, `signInAs`, `assignJSON`, `getJSON`, `writeErrorCode`, `startWritingHomework`, `createAssignment`, `startAssignment`). Frontend: vitest for pure functions, reducers and normalizers only; UI is checked with a one-off Playwright harness and screenshots under `.superpowers/tmp/`.
- Go test command: `cd apps/api && CGO_ENABLED=0 go test <pkg> -run <pattern> -count=1 -timeout 1800s`. Do not run DB tests while another agent's e2e stack is using Docker; check `docker ps` first.
- sqlc: `cd apps/api && CGO_ENABLED=0 go tool sqlc generate`, then `git status --short internal/store/sqlc` (a failed run can exit 0 and generate nothing). macOS fallback: `CGO_CFLAGS="-DHAVE_STRCHRNUL" go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate`.
- Frontend commands: `cd apps/lite-web && pnpm vitest run <file>` and `pnpm typecheck`.
- Commits: stage specific paths, never `git add -A`. Every commit message ends with `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

## File map

Backend (`apps/api/internal/…`):
- `liteassign/rubric.go` (+ `rubric_test.go`), `liteassign/payload.go` — rubric type, defaults, validation, carry/apply for PATCH.
- `api/lite_teacher_assignments.go` — PATCH accepts `rubric`. Test: `api/lite_assignment_rubric_test.go`.
- `litegrade/content.go`, `litegrade/check.go`, `litegrade/prompt.go` (+ `check_test.go`, `content_test.go`, `prompt_test.go`) — pure grading rules.
- `store/migrations/0154_lite_grading.sql`, `store/queries/lite_grading.sql`, regenerated `store/sqlc/`.
- `api/lite_grading_run.go` (+ `lite_grading_run_internal_test.go`) — `gradeWithRetry`, `liteGradingInput`.
- `api/lite_grading_jobs.go`, `api/interest_jobs.go` — river args, worker, enqueue, registration.
- `api/lite_teacher_gradings.go` — teacher routes. Test: `api/lite_grading_test.go`.
- `api/lite_student_gradings.go`, `api/lite_student_assignments.go`, `api/lite_teacher_item.go` — student reads, inbox, item payload. Test: `api/lite_grading_student_test.go`.
- `api/lite_grading_live_test.go` — `TestLiveLiteGrading`.
- `api/api.go`, `api/lite_teacher_routes.go` — routes.

Frontend (`apps/lite-web/src/…`):
- `api/gradings.ts` (+ `gradings.test.ts`), `api/assignments.ts`, `api/teacher.ts`, `api/writings.ts` — types, clients, normalizers.
- `shared/gradingText.ts` (+ test) — quote highlight ranges.
- `teacher/gradingLogic.ts` (+ test), `teacher/rubricLogic.ts` (+ test), `teacher/assignmentLogic.ts` — status mapping, content reducer, rubric draft.
- `teacher/RubricFields.tsx`, `teacher/AssignmentForm.tsx`, `teacher/AssignmentDetailPage.tsx` — 评分标准.
- `teacher/GradingTab.tsx`, `teacher/GradingPage.tsx`, `teacher/teacherRouting.ts`, `teacher/teacherRail.ts`, `teacher/LiteTeacherShell.tsx` — 批改 tab and grading view.
- `teacher/ItemPage.tsx` — versions, 批改 button, object comment points.
- `writings/TeacherGradingPanel.tsx`, `writings/FinishedWritingPage.tsx`, `writings/finishedWriting.ts` — student panel.
- `inbox/inboxLogic.ts`, `inbox/InboxPanel.tsx` — grading rows.
- `e2e/grading-shots.spec.ts` — one-off screenshot harness, deleted at the end.

---

### Task 1: Rubric in the writing homework payload

**Files:**
- Create: `apps/api/internal/liteassign/rubric.go`
- Create: `apps/api/internal/liteassign/rubric_test.go`
- Modify: `apps/api/internal/liteassign/payload.go` (`WritingPayload`, the `"writing"` branch of `ValidatePayload`)
- Modify: `apps/api/internal/api/lite_teacher_assignments.go` (`patchLiteAssignment`)
- Test: `apps/api/internal/api/lite_assignment_rubric_test.go`

**Interfaces:**
- Produces (package `liteassign`):
  - `type Rubric struct { Scale string; Max int; Dimensions []RubricDimension; Focus string }` (JSON `scale`, `max,omitempty`, `dimensions`, `focus`)
  - `type RubricDimension struct { Name, Note string }` (JSON `name`, `note`)
  - `const ScaleLetter = "letter"`, `ScalePoints = "points"`; `var LetterGrades []string`
  - `func DefaultRubric(lang string) Rubric`
  - `func ValidateRubric(r Rubric) (Rubric, error)` — trimmed copy or `*PayloadError`
  - `func ParseRubric(raw json.RawMessage) (Rubric, error)`
  - `func EffectiveRubric(payload json.RawMessage) Rubric` — stored rubric, else `DefaultRubric(payload.lang)`
  - `func GradeInScale(r Rubric, grade string) bool`
  - `func CarryRubric(next, stored json.RawMessage) (json.RawMessage, error)`
  - `func ApplyRubric(payload, raw json.RawMessage) (json.RawMessage, error)` — `raw` `null` removes the rubric
  - `WritingPayload.Rubric *Rubric` (JSON `rubric,omitempty`)
- Produces (HTTP): `PATCH /api/v1/lite/teacher/assignments/{aid}` accepts `"rubric": Rubric | null` at top level, allowed after students started. Errors: 400 `rubric_not_writing` 「只有写作作业可以设置评分标准」, 400 with the `PayloadError` codes below.

- [ ] **Step 1: Write the failing rubric tests**

Create `apps/api/internal/liteassign/rubric_test.go`:

```go
package liteassign

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func codeOf(err error) string {
	var pe *PayloadError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

func TestDefaultRubric(t *testing.T) {
	names := func(r Rubric) string {
		out := make([]string, 0, len(r.Dimensions))
		for _, d := range r.Dimensions {
			out = append(out, d.Name)
		}
		return strings.Join(out, "/")
	}
	if got := names(DefaultRubric("zh")); got != "内容/结构/语言/书写规范" {
		t.Fatalf("zh = %s", got)
	}
	if got := names(DefaultRubric("en")); got != "Task Response/Coherence and Cohesion/Lexical Resource/Grammatical Range and Accuracy" {
		t.Fatalf("en = %s", got)
	}
	for _, lang := range []string{"zh", "en", ""} {
		r := DefaultRubric(lang)
		if r.Scale != ScaleLetter {
			t.Fatalf("%q scale = %s", lang, r.Scale)
		}
		// The defaults must pass the same validation a teacher's rubric does.
		if _, err := ValidateRubric(r); err != nil {
			t.Fatalf("default %q invalid: %v", lang, err)
		}
	}
}

func TestValidateRubric(t *testing.T) {
	dims := func(names ...string) []RubricDimension {
		out := make([]RubricDimension, 0, len(names))
		for _, n := range names {
			out = append(out, RubricDimension{Name: n})
		}
		return out
	}
	got, err := ValidateRubric(Rubric{Scale: " points ", Max: 20, Dimensions: []RubricDimension{{Name: " 论证 ", Note: " 看证据 "}}, Focus: " 重点看论证 "})
	if err != nil || got.Scale != ScalePoints || got.Max != 20 || got.Dimensions[0].Name != "论证" || got.Dimensions[0].Note != "看证据" || got.Focus != "重点看论证" {
		t.Fatalf("trimmed = %+v err=%v", got, err)
	}
	if got, _ := ValidateRubric(Rubric{Scale: "letter", Max: 50, Dimensions: dims("内容")}); got.Max != 0 {
		t.Fatalf("letter keeps max %d, want 0", got.Max)
	}

	bad := []struct {
		name string
		r    Rubric
		code string
	}{
		{"scale", Rubric{Scale: "stars", Dimensions: dims("内容")}, "invalid_rubric_scale"},
		{"max zero", Rubric{Scale: "points", Max: 0, Dimensions: dims("内容")}, "invalid_rubric_max"},
		{"max 101", Rubric{Scale: "points", Max: 101, Dimensions: dims("内容")}, "invalid_rubric_max"},
		{"no dimensions", Rubric{Scale: "letter"}, "invalid_rubric_dimensions"},
		{"seven dimensions", Rubric{Scale: "letter", Dimensions: dims("1", "2", "3", "4", "5", "6", "7")}, "invalid_rubric_dimensions"},
		{"blank name", Rubric{Scale: "letter", Dimensions: dims("  ")}, "invalid_rubric_dimension_name"},
		{"41-rune name", Rubric{Scale: "letter", Dimensions: dims(strings.Repeat("字", 41))}, "invalid_rubric_dimension_name"},
		{"duplicate", Rubric{Scale: "letter", Dimensions: dims("内容", " 内容")}, "duplicate_rubric_dimension"},
		{"long note", Rubric{Scale: "letter", Dimensions: []RubricDimension{{Name: "内容", Note: strings.Repeat("字", 201)}}}, "invalid_rubric_note"},
		{"long focus", Rubric{Scale: "letter", Dimensions: dims("内容"), Focus: strings.Repeat("字", 501)}, "invalid_rubric_focus"},
	}
	for _, c := range bad {
		if _, err := ValidateRubric(c.r); codeOf(err) != c.code {
			t.Errorf("%s: got %v, want %s", c.name, err, c.code)
		}
	}
	if _, err := ParseRubric(json.RawMessage(`{"scale":"points","max":20.5,"dimensions":[{"name":"x"}]}`)); codeOf(err) != "invalid_rubric" {
		t.Fatalf("fractional max = %v, want invalid_rubric", err)
	}
}

func TestGradeInScale(t *testing.T) {
	letter := DefaultRubric("zh")
	for _, g := range []string{"A+", "B-", "D"} {
		if !GradeInScale(letter, g) {
			t.Errorf("letter %q should be in scale", g)
		}
	}
	for _, g := range []string{"", "E", "a", "A++", "90"} {
		if GradeInScale(letter, g) {
			t.Errorf("letter %q should be out of scale", g)
		}
	}
	points := Rubric{Scale: ScalePoints, Max: 20, Dimensions: []RubricDimension{{Name: "论证"}}}
	for _, g := range []string{"0", "13", "20"} {
		if !GradeInScale(points, g) {
			t.Errorf("points %q should be in scale", g)
		}
	}
	for _, g := range []string{"21", "-1", "08", "13.5", "B"} {
		if GradeInScale(points, g) {
			t.Errorf("points %q should be out of scale", g)
		}
	}
}

func TestEffectiveCarryApplyRubric(t *testing.T) {
	noRubric := json.RawMessage(`{"prompt":"写雨","targetWords":800,"lang":"en"}`)
	if r := EffectiveRubric(noRubric); r.Dimensions[0].Name != "Task Response" {
		t.Fatalf("missing rubric reads as the en default, got %+v", r)
	}
	withRubric, err := ApplyRubric(noRubric, json.RawMessage(`{"scale":"points","max":10,"dimensions":[{"name":"论证","note":""}],"focus":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if r := EffectiveRubric(withRubric); r.Scale != ScalePoints || r.Max != 10 {
		t.Fatalf("applied = %+v", r)
	}
	// CarryRubric keeps the stored rubric whatever the next payload says.
	next := json.RawMessage(`{"prompt":"写雨","targetWords":800,"lang":"en","rubric":{"scale":"letter","dimensions":[{"name":"x","note":""}],"focus":""}}`)
	carried, err := CarryRubric(next, withRubric)
	if err != nil {
		t.Fatal(err)
	}
	if r := EffectiveRubric(carried); r.Scale != ScalePoints {
		t.Fatalf("carried = %+v, want the stored points rubric", r)
	}
	reset, err := ApplyRubric(withRubric, json.RawMessage(`null`))
	if err != nil || strings.Contains(string(reset), "rubric") {
		t.Fatalf("null reset = %s err=%v", reset, err)
	}
	if _, err := ApplyRubric(noRubric, json.RawMessage(`{"scale":"points","max":0,"dimensions":[{"name":"x"}]}`)); codeOf(err) != "invalid_rubric_max" {
		t.Fatalf("invalid apply = %v", err)
	}
}

func TestValidatePayloadWritingRubric(t *testing.T) {
	out, err := ValidatePayload("writing", json.RawMessage(`{"prompt":"写雨","targetWords":800,"lang":"zh","rubric":{"scale":"letter","dimensions":[{"name":" 内容 ","note":""}],"focus":""}}`))
	if err != nil || !strings.Contains(string(out), `"name":"内容"`) {
		t.Fatalf("rubric kept and trimmed: %s err=%v", out, err)
	}
	if _, err := ValidatePayload("writing", json.RawMessage(`{"prompt":"写雨","targetWords":800,"lang":"zh","rubric":{"scale":"x","dimensions":[{"name":"内容"}]}}`)); codeOf(err) != "invalid_rubric_scale" {
		t.Fatalf("bad rubric in payload = %v", err)
	}
	out, _ = ValidatePayload("writing", json.RawMessage(`{"prompt":"写雨","targetWords":800,"lang":"zh"}`))
	if strings.Contains(string(out), "rubric") {
		t.Fatalf("no rubric must not add one: %s", out)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd apps/api && go test ./internal/liteassign -run 'Rubric|GradeInScale' -count=1`
Expected: FAIL to build (`undefined: Rubric`, `undefined: DefaultRubric`).

- [ ] **Step 3: Write `rubric.go`**

Create `apps/api/internal/liteassign/rubric.go`:

```go
package liteassign

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Rubric is a writing homework's 评分标准. AI 批改 grades against it, and each
// grading row keeps a copy of the rubric it used.
type Rubric struct {
	Scale      string            `json:"scale"`
	Max        int               `json:"max,omitempty"`
	Dimensions []RubricDimension `json:"dimensions"`
	Focus      string            `json:"focus"`
}

type RubricDimension struct {
	Name string `json:"name"`
	Note string `json:"note"`
}

const (
	ScaleLetter = "letter"
	ScalePoints = "points"

	maxRubricDimensions   = 6
	maxDimensionNameRunes = 40 // the English defaults are up to 30 characters
	maxDimensionNoteRunes = 200
	maxRubricFocusRunes   = 500
	maxRubricPoints       = 100
)

// LetterGrades is the letter scale, highest first.
var LetterGrades = []string{"A+", "A", "A-", "B+", "B", "B-", "C+", "C", "C-", "D"}

// DefaultRubric is the rubric a writing homework without one is graded with.
func DefaultRubric(lang string) Rubric {
	if lang == "en" {
		return Rubric{Scale: ScaleLetter, Dimensions: []RubricDimension{
			{Name: "Task Response", Note: "是否回应题目的全部要求，观点是否展开并有支撑"},
			{Name: "Coherence and Cohesion", Note: "段落安排是否清楚，句与句、段与段之间是否衔接"},
			{Name: "Lexical Resource", Note: "用词是否准确、多样，搭配是否得当"},
			{Name: "Grammatical Range and Accuracy", Note: "句式是否多样，语法是否准确"},
		}}
	}
	return Rubric{Scale: ScaleLetter, Dimensions: []RubricDimension{
		{Name: "内容", Note: "立意是否明确，材料是否支撑观点"},
		{Name: "结构", Note: "段落顺序是否清楚，段与段之间是否衔接"},
		{Name: "语言", Note: "表达是否准确、通顺"},
		{Name: "书写规范", Note: "标点、错别字与格式"},
	}}
}

// ValidateRubric returns a trimmed copy, or a PayloadError naming the first problem.
func ValidateRubric(r Rubric) (Rubric, error) {
	out := Rubric{Scale: strings.TrimSpace(r.Scale), Focus: strings.TrimSpace(r.Focus)}
	switch out.Scale {
	case ScaleLetter:
	case ScalePoints:
		if r.Max < 1 || r.Max > maxRubricPoints {
			return Rubric{}, perr("invalid_rubric_max", "满分需在 1 到 100 之间")
		}
		out.Max = r.Max
	default:
		return Rubric{}, perr("invalid_rubric_scale", "评分方式只能是等级或分数")
	}
	if len(r.Dimensions) < 1 || len(r.Dimensions) > maxRubricDimensions {
		return Rubric{}, perr("invalid_rubric_dimensions", "评分维度需有 1 到 6 项")
	}
	seen := make(map[string]bool, len(r.Dimensions))
	out.Dimensions = make([]RubricDimension, 0, len(r.Dimensions))
	for _, d := range r.Dimensions {
		name, note := strings.TrimSpace(d.Name), strings.TrimSpace(d.Note)
		if name == "" || utf8.RuneCountInString(name) > maxDimensionNameRunes {
			return Rubric{}, perr("invalid_rubric_dimension_name", "维度名称不能为空，不超过 40 字")
		}
		// Check matches the model's dimensions by name, so names must be unique.
		if seen[name] {
			return Rubric{}, perr("duplicate_rubric_dimension", "维度名称不能重复")
		}
		seen[name] = true
		if utf8.RuneCountInString(note) > maxDimensionNoteRunes {
			return Rubric{}, perr("invalid_rubric_note", "维度说明不超过 200 字")
		}
		out.Dimensions = append(out.Dimensions, RubricDimension{Name: name, Note: note})
	}
	if utf8.RuneCountInString(out.Focus) > maxRubricFocusRunes {
		return Rubric{}, perr("invalid_rubric_focus", "批改重点不超过 500 字")
	}
	return out, nil
}

// ParseRubric decodes and validates a rubric sent by the teacher.
func ParseRubric(raw json.RawMessage) (Rubric, error) {
	var r Rubric
	if err := json.Unmarshal(raw, &r); err != nil {
		return Rubric{}, perr("invalid_rubric", "评分标准格式错误")
	}
	return ValidateRubric(r)
}

// EffectiveRubric is the rubric a writing homework is graded with: the stored
// one, or the default for the payload's language (homework created before
// rubrics existed has none).
func EffectiveRubric(payload json.RawMessage) Rubric {
	var p WritingPayload
	if err := json.Unmarshal(payload, &p); err == nil && p.Rubric != nil {
		return *p.Rubric
	}
	return DefaultRubric(p.Lang)
}

// GradeInScale: a letter from LetterGrades, or an integer 0..Max written
// without leading zeros.
func GradeInScale(r Rubric, grade string) bool {
	switch r.Scale {
	case ScaleLetter:
		for _, g := range LetterGrades {
			if grade == g {
				return true
			}
		}
	case ScalePoints:
		n, err := strconv.Atoi(grade)
		return err == nil && strconv.Itoa(n) == grade && n >= 0 && n <= r.Max
	}
	return false
}

// CarryRubric puts the stored payload's rubric onto the next payload. PATCH
// uses it so the rubric changes only through the top-level rubric field.
func CarryRubric(next, stored json.RawMessage) (json.RawMessage, error) {
	var p, old WritingPayload
	if json.Unmarshal(next, &p) != nil || json.Unmarshal(stored, &old) != nil {
		return nil, perr("invalid_payload", "作业设置格式错误")
	}
	p.Rubric = old.Rubric
	return json.Marshal(p)
}

// ApplyRubric sets the rubric on a writing payload; a JSON null removes it,
// so the homework reads as the default again.
func ApplyRubric(payload, raw json.RawMessage) (json.RawMessage, error) {
	var p WritingPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, perr("invalid_payload", "作业设置格式错误")
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		p.Rubric = nil
	} else {
		r, err := ParseRubric(raw)
		if err != nil {
			return nil, err
		}
		p.Rubric = &r
	}
	return json.Marshal(p)
}
```

In `apps/api/internal/liteassign/payload.go`, `WritingPayload` becomes:

```go
type WritingPayload struct {
	Prompt      string  `json:"prompt"`
	TargetWords int     `json:"targetWords"`
	Lang        string  `json:"lang"`
	Rubric      *Rubric `json:"rubric,omitempty"`
}
```

and in the `"writing"` branch of `ValidatePayload`, directly before `return json.Marshal(p)`:

```go
		if p.Rubric != nil {
			r, err := ValidateRubric(*p.Rubric)
			if err != nil {
				return nil, err
			}
			p.Rubric = &r
		}
```

- [ ] **Step 4: Run the liteassign tests**

Run: `cd apps/api && go test ./internal/liteassign -count=1`
Expected: PASS (including the existing `TestValidatePayload*`).

- [ ] **Step 5: Write the failing PATCH test**

Create `apps/api/internal/api/lite_assignment_rubric_test.go`:

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/liteassign"
)

type assignmentPayloadResp struct {
	Assignment struct {
		Payload json.RawMessage `json:"payload"`
	} `json:"assignment"`
}

// The rubric stays editable after a student started, through its own field;
// kind and payload stay locked.
func TestAssignmentRubricEditableAfterStart(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid, _, _ := startWritingHomework(t, h, pool, teacher, classID, studentID)
	path := "/api/v1/lite/teacher/assignments/" + aid
	points := map[string]any{"scale": "points", "max": 20, "dimensions": []map[string]any{{"name": "论证", "note": ""}}, "focus": "重点看论证"}

	var resp assignmentPayloadResp
	if code := assignJSON(t, h, teacher, "PATCH", path, map[string]any{"rubric": points}, &resp); code != http.StatusOK {
		t.Fatalf("patch rubric after start = %d", code)
	}
	if r := liteassign.EffectiveRubric(resp.Assignment.Payload); r.Scale != "points" || r.Max != 20 || r.Focus != "重点看论证" {
		t.Fatalf("stored rubric = %+v", r)
	}

	// Resending unchanged settings is not a change, and a rubric inside the
	// payload does not overwrite the stored one.
	same := map[string]any{"kind": "writing", "payload": map[string]any{
		"prompt": "写一篇关于雨的记叙文", "targetWords": 800, "lang": "zh",
		"rubric": map[string]any{"scale": "letter", "dimensions": []map[string]any{{"name": "x", "note": ""}}, "focus": ""},
	}}
	if code := assignJSON(t, h, teacher, "PATCH", path, same, &resp); code != http.StatusOK {
		t.Fatalf("resend settings = %d", code)
	}
	if r := liteassign.EffectiveRubric(resp.Assignment.Payload); r.Scale != "points" {
		t.Fatalf("payload rubric overwrote the stored one: %+v", r)
	}

	changed := map[string]any{"payload": map[string]any{"prompt": "换一个题目", "targetWords": 800, "lang": "zh"}}
	if code, ec := writeErrorCode(t, h, teacher, "PATCH", path, changed); code != http.StatusConflict || ec != "assignment_started" {
		t.Fatalf("prompt change after start = %d %s", code, ec)
	}
	badRubric := map[string]any{"rubric": map[string]any{"scale": "points", "max": 0, "dimensions": []map[string]any{{"name": "论证"}}}}
	if code, ec := writeErrorCode(t, h, teacher, "PATCH", path, badRubric); code != http.StatusBadRequest || ec != "invalid_rubric_max" {
		t.Fatalf("invalid rubric = %d %s", code, ec)
	}
	if code := assignJSON(t, h, teacher, "PATCH", path, map[string]any{"rubric": nil}, &resp); code != http.StatusOK {
		t.Fatalf("reset rubric = %d", code)
	}
	if r := liteassign.EffectiveRubric(resp.Assignment.Payload); r.Scale != "letter" || r.Dimensions[0].Name != "内容" {
		t.Fatalf("after reset = %+v, want the zh default", r)
	}

	reading := createAssignment(t, h, teacher, classID, readingAssignmentBody("读", map[string]any{"source": "text", "text": "一段正文。"}, []string{studentID.String()}))
	if code, ec := writeErrorCode(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+reading, map[string]any{"rubric": points}); code != http.StatusBadRequest || ec != "rubric_not_writing" {
		t.Fatalf("rubric on reading = %d %s", code, ec)
	}
}
```

(The dot-import keeps this file consistent with its neighbours; if `go vet` reports it unused, remove that import line.)

- [ ] **Step 6: Run it to see it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestAssignmentRubricEditableAfterStart -count=1 -timeout 1800s`
Expected: FAIL at `stored rubric = {Scale:letter …}` (the `rubric` field is ignored today).

- [ ] **Step 7: Accept `rubric` in PATCH**

In `apps/api/internal/api/lite_teacher_assignments.go`, `patchLiteAssignment`:

Add to the request struct:

```go
		Rubric        json.RawMessage `json:"rubric"`
```

Replace the block that starts `settingsChanged := false` and ends before `if settingsChanged {` with:

```go
	settingsChanged := false
	if req.Kind != nil || req.Payload != nil {
		if req.Kind != nil {
			params.Kind = *req.Kind
		}
		raw := json.RawMessage(locked.Payload)
		if req.Payload != nil {
			raw = req.Payload
		}
		validated, err := liteassign.ValidatePayload(params.Kind, raw)
		if err != nil {
			httpx.WriteError(w, r, payloadErrorResponse(err))
			return
		}
		// The rubric changes only through req.Rubric. Carrying the stored one
		// keeps a resent form from counting as a settings change.
		if params.Kind == "writing" && locked.Kind == "writing" {
			if validated, err = liteassign.CarryRubric(validated, locked.Payload); err != nil {
				httpx.WriteError(w, r, payloadErrorResponse(err))
				return
			}
		}
		params.Payload = validated
		settingsChanged = params.Kind != locked.Kind || !samePayload(validated, locked.Payload)
	}
	// The rubric stays editable after students start: gradings keep their own copy.
	if req.Rubric != nil {
		if params.Kind != "writing" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("rubric_not_writing", "只有写作作业可以设置评分标准", nil))
			return
		}
		withRubric, err := liteassign.ApplyRubric(params.Payload, req.Rubric)
		if err != nil {
			httpx.WriteError(w, r, payloadErrorResponse(err))
			return
		}
		params.Payload = withRubric
	}
```

- [ ] **Step 8: Run the PATCH tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestAssignmentRubricEditableAfterStart|TestTeacherAssignment' -count=1 -timeout 1800s`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/liteassign/rubric.go apps/api/internal/liteassign/rubric_test.go \
  apps/api/internal/liteassign/payload.go apps/api/internal/api/lite_teacher_assignments.go \
  apps/api/internal/api/lite_assignment_rubric_test.go
git commit -m "feat(lite): rubric on writing homework, editable after students start

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `litegrade` — content, parser, prompt and `Check`

**Files:**
- Create: `apps/api/internal/litegrade/content.go`
- Create: `apps/api/internal/litegrade/check.go`
- Create: `apps/api/internal/litegrade/prompt.go`
- Test: `apps/api/internal/litegrade/content_test.go`, `check_test.go`, `prompt_test.go`

**Interfaces:**
- Consumes: `liteassign.Rubric`, `liteassign.GradeInScale`, `liteassign.LetterGrades`, `liteassign.ScalePoints` (Task 1).
- Produces (package `litegrade`):
  - `type Content struct { Overall Overall; Dimensions []Dimension; Points []Point }`; `Overall{Grade, Comment string}`; `Dimension{Name, Grade, Comment string}`; `Point{Kind string; Quote *string; Text string; Action *string; Source string}` (JSON: `overall`, `grade`, `comment`, `dimensions`, `name`, `points`, `kind`, `quote`, `text`, `action`, `source`)
  - `const KindGood = "good"`, `KindIssue = "issue"`, `SourceAI = "ai"`, `SourceTeacher = "teacher"`, `MinPoints = 3`, `MaxPoints = 5`
  - `type Input struct { Lang, Title, Body, AssignedPrompt string; TargetWords, VersionNumber int; Rubric liteassign.Rubric; SymptomCatalog string; PersonJudging func(string) bool }`
  - `func Parse(text string) (Content, error)`; `var ErrUnparseable error`
  - `func NormalizeAI(c Content, r liteassign.Rubric) Content`; `func NormalizeTeacher(c Content, r liteassign.Rubric) Content`
  - `type Reason struct { Code, Where, Detail string }`; `func (Reason) Message() string`
  - Reason codes: `ReasonQuoteMissing`, `ReasonQuoteNotInBody`, `ReasonQuotationNotInBody`, `ReasonQuoteFromPrompt`, `ReasonGradeOutOfScale`, `ReasonDimensionNames`, `ReasonPointCount`, `ReasonNoGoodPoint`, `ReasonNoIssuePoint`, `ReasonIssueWithoutAction`, `ReasonPersonJudging`, `ReasonPointKind`, `ReasonEmptyText`, `ReasonUnparseable`, `ReasonModelCall`
  - `func Check(c Content, in Input) []Reason`; `func CheckTeacherEdit(c Content, in Input) []Reason`
  - `func SystemPrompt(in Input) string`; `func UserPrompt(in Input) string`; `func RetryNudge(rs []Reason) string`; `func JoinReasons(rs []Reason) string`

- [ ] **Step 1: Write the failing `Check` tests**

Create `apps/api/internal/litegrade/check_test.go`:

```go
package litegrade

import (
	"strings"
	"testing"

	"mindimprint/api/internal/liteassign"
)

const testBody = "学校后门那片空地一下雨就积水。去年秋天，我在那里摔过一跤。\n\n我读到城市里的雨水花园：用下凹的绿地先把雨水接住。"
const testPrompt = "写一篇关于校园积水的议论文，说明雨水花园是否适合学校。"

func sp(s string) *string { return &s }

func testInput() Input {
	return Input{
		Lang: "zh", Title: "雨水去哪儿了", Body: testBody, AssignedPrompt: testPrompt,
		VersionNumber: 1, Rubric: liteassign.DefaultRubric("zh"),
		PersonJudging: func(s string) bool { return strings.Contains(s, "你很") },
	}
}

func validContent() Content {
	return Content{
		Overall: Overall{Grade: "B+", Comment: "用「去年秋天，我在那里摔过一跤。」引出问题。"},
		Dimensions: []Dimension{
			{Name: "内容", Grade: "B+", Comment: "问题来自亲身经历。"},
			{Name: "结构", Grade: "B", Comment: "两段之间没有过渡句。"},
			{Name: "语言", Grade: "A-", Comment: "表达清楚。"},
			{Name: "书写规范", Grade: "A", Comment: "标点使用正确。"},
		},
		Points: []Point{
			{Kind: KindGood, Quote: sp("去年秋天，我在那里摔过一跤。"), Text: "用具体经历引出问题。", Source: SourceAI},
			{Kind: KindIssue, Quote: sp("我读到城市里的雨水花园：用下凹的绿地先把雨水接住。"), Text: "材料与后门空地之间没有说明联系。", Action: sp("在这句后面写一句说明雨水花园和后门空地的关系。"), Source: SourceAI},
			{Kind: KindIssue, Quote: sp("学校后门那片空地一下雨就积水。"), Text: "积水的程度没有数据。", Action: sp("补充一次积水的深度或持续时间。"), Source: SourceAI},
		},
	}
}

func codes(rs []Reason) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Code)
	}
	return out
}

func hasCode(rs []Reason, code string) bool {
	for _, r := range rs {
		if r.Code == code {
			return true
		}
	}
	return false
}

func TestCheckAcceptsValidContent(t *testing.T) {
	if rs := Check(validContent(), testInput()); len(rs) != 0 {
		t.Fatalf("valid content rejected: %v", codes(rs))
	}
	// Dimensions in another order are the same rubric.
	c := validContent()
	c.Dimensions[0], c.Dimensions[3] = c.Dimensions[3], c.Dimensions[0]
	if rs := Check(c, testInput()); len(rs) != 0 {
		t.Fatalf("reordered dimensions rejected: %v", codes(rs))
	}
}

func TestCheckRejections(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(c *Content, in *Input)
		code   string
	}{
		{"quote not a substring", func(c *Content, _ *Input) { c.Points[0].Quote = sp("去年冬天，我在那里摔过一跤。") }, ReasonQuoteNotInBody},
		{"quote missing", func(c *Content, _ *Input) { c.Points[1].Quote = nil }, ReasonQuoteMissing},
		{"quote blank", func(c *Content, _ *Input) { c.Points[1].Quote = sp("  ") }, ReasonQuoteMissing},
		{"quote only in the assigned prompt", func(c *Content, _ *Input) { c.Points[1].Quote = sp("说明雨水花园是否适合学校") }, ReasonQuoteFromPrompt},
		{"quotation in overall comment not hers", func(c *Content, _ *Input) { c.Overall.Comment = "可以改成「雨水从后门流走了」。" }, ReasonQuotationNotInBody},
		{"quotation in dimension comment not hers", func(c *Content, _ *Input) { c.Dimensions[1].Comment = "第二段开头写「因此学校需要雨水花园」会更顺。" }, ReasonQuotationNotInBody},
		{"quotation in point text not hers", func(c *Content, _ *Input) { c.Points[1].Text = "这句应该是「雨水花园能解决积水」。" }, ReasonQuotationNotInBody},
		{"quotation in action not hers", func(c *Content, _ *Input) { c.Points[2].Action = sp("把这句改成「后门空地每次积水十厘米」。") }, ReasonQuotationNotInBody},
		{"quotation of the assigned prompt", func(c *Content, _ *Input) { c.Points[2].Text = "没有回应「说明雨水花园是否适合学校」。" }, ReasonQuoteFromPrompt},
		{"overall grade outside the scale", func(c *Content, _ *Input) { c.Overall.Grade = "E" }, ReasonGradeOutOfScale},
		{"dimension grade outside the scale", func(c *Content, _ *Input) { c.Dimensions[2].Grade = "A++" }, ReasonGradeOutOfScale},
		{"points grade above max", func(c *Content, in *Input) {
			in.Rubric = liteassign.Rubric{Scale: liteassign.ScalePoints, Max: 20, Dimensions: []liteassign.RubricDimension{{Name: "论证"}}}
			c.Overall.Grade = "21"
			c.Dimensions = []Dimension{{Name: "论证", Grade: "15", Comment: "有证据。"}}
		}, ReasonGradeOutOfScale},
		{"dimension renamed", func(c *Content, _ *Input) { c.Dimensions[0].Name = "立意" }, ReasonDimensionNames},
		{"dimension missing", func(c *Content, _ *Input) { c.Dimensions = c.Dimensions[:3] }, ReasonDimensionNames},
		{"dimension added", func(c *Content, _ *Input) {
			c.Dimensions = append(c.Dimensions, Dimension{Name: "创意", Grade: "A", Comment: "有新意。"})
		}, ReasonDimensionNames},
		{"two points", func(c *Content, _ *Input) { c.Points = c.Points[:2] }, ReasonPointCount},
		{"six points", func(c *Content, _ *Input) {
			for len(c.Points) < 6 {
				c.Points = append(c.Points, c.Points[2])
			}
		}, ReasonPointCount},
		{"no good point", func(c *Content, _ *Input) { c.Points[0] = c.Points[1] }, ReasonNoGoodPoint},
		{"no issue point", func(c *Content, _ *Input) {
			c.Points[1] = c.Points[0]
			c.Points[2] = c.Points[0]
		}, ReasonNoIssuePoint},
		{"issue without action", func(c *Content, _ *Input) { c.Points[1].Action = nil }, ReasonIssueWithoutAction},
		{"issue with blank action", func(c *Content, _ *Input) { c.Points[2].Action = sp("   ") }, ReasonIssueWithoutAction},
		{"point judges her as a person", func(c *Content, _ *Input) { c.Points[1].Text = "你很粗心，没有检查。" }, ReasonPersonJudging},
		{"overall comment judges her", func(c *Content, _ *Input) { c.Overall.Comment = "你很懒。" }, ReasonPersonJudging},
		{"unknown point kind", func(c *Content, _ *Input) { c.Points[2].Kind = "note" }, ReasonPointKind},
		{"empty point text", func(c *Content, _ *Input) { c.Points[1].Text = " " }, ReasonEmptyText},
		{"empty overall comment", func(c *Content, _ *Input) { c.Overall.Comment = "" }, ReasonEmptyText},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, in := validContent(), testInput()
			tc.mutate(&c, &in)
			rs := Check(c, in)
			if !hasCode(rs, tc.code) {
				t.Fatalf("codes = %v, want %s", codes(rs), tc.code)
			}
			for _, r := range rs {
				if r.Message() == "" || r.Message() == r.Code {
					t.Fatalf("reason %s has no message", r.Code)
				}
			}
		})
	}
}

func TestCheckPointsScaleAccepts(t *testing.T) {
	c, in := validContent(), testInput()
	in.Rubric = liteassign.Rubric{Scale: liteassign.ScalePoints, Max: 20, Dimensions: []liteassign.RubricDimension{{Name: "论证"}}}
	c.Overall.Grade = "20"
	c.Dimensions = []Dimension{{Name: "论证", Grade: "0", Comment: "没有证据。"}}
	if rs := Check(c, in); len(rs) != 0 {
		t.Fatalf("points content rejected: %v", codes(rs))
	}
}

func TestCheckTeacherEditIsShapeOnly(t *testing.T) {
	in := testInput()
	c := validContent()
	// One teacher point with no quote and no action: fine for a teacher.
	c.Points = []Point{{Kind: KindIssue, Text: "第二段请补充数据来源。", Source: SourceTeacher}}
	c.Overall.Comment = "可以写「雨水花园」的来源。"
	if rs := CheckTeacherEdit(c, in); len(rs) != 0 {
		t.Fatalf("teacher edit rejected: %v", codes(rs))
	}
	bad := []struct {
		name   string
		mutate func(c *Content)
		code   string
	}{
		{"grade out of scale", func(c *Content) { c.Overall.Grade = "E" }, ReasonGradeOutOfScale},
		{"dimension names", func(c *Content) { c.Dimensions[0].Name = "立意" }, ReasonDimensionNames},
		{"teacher quote not a substring", func(c *Content) { c.Points[0].Quote = sp("雨一直下。") }, ReasonQuoteNotInBody},
		{"empty point text", func(c *Content) { c.Points[0].Text = "" }, ReasonEmptyText},
		{"kind", func(c *Content) { c.Points[0].Kind = "praise" }, ReasonPointKind},
	}
	for _, tc := range bad {
		cc := c
		cc.Points = append([]Point(nil), c.Points...)
		cc.Dimensions = append([]Dimension(nil), c.Dimensions...)
		tc.mutate(&cc)
		if rs := CheckTeacherEdit(cc, in); !hasCode(rs, tc.code) {
			t.Errorf("%s: codes = %v, want %s", tc.name, codes(rs), tc.code)
		}
	}
}

func TestJoinReasonsDedupes(t *testing.T) {
	rs := []Reason{{Code: ReasonNoGoodPoint}, {Code: ReasonNoGoodPoint}, {Code: ReasonPointCount, Detail: "2"}}
	if got := JoinReasons(rs); got != "没有优点意见；意见共 2 条，需要 3 到 5 条" {
		t.Fatalf("JoinReasons = %q", got)
	}
}
```

Create `apps/api/internal/litegrade/content_test.go`:

```go
package litegrade

import (
	"errors"
	"testing"

	"mindimprint/api/internal/liteassign"
)

func TestParse(t *testing.T) {
	fenced := "```json\n{\"overall\":{\"grade\":\"B\",\"comment\":\"x\"},\"dimensions\":[],\"points\":[{\"kind\":\"good\",\"quote\":\"a\",\"text\":\"b\",\"action\":null}]}\n```"
	c, err := Parse(fenced)
	if err != nil || c.Overall.Grade != "B" || len(c.Points) != 1 || c.Points[0].Action != nil {
		t.Fatalf("fenced = %+v err=%v", c, err)
	}
	for _, s := range []string{"", "抱歉，我无法批改。", "{not json}", "{\"overall\": 3}"} {
		if _, err := Parse(s); !errors.Is(err, ErrUnparseable) {
			t.Errorf("Parse(%q) err = %v, want ErrUnparseable", s, err)
		}
	}
}

func TestNormalize(t *testing.T) {
	r := liteassign.DefaultRubric("zh")
	c := Content{
		Overall: Overall{Grade: " B ", Comment: " 好 "},
		Dimensions: []Dimension{
			{Name: "书写规范", Grade: "A"}, {Name: "语言", Grade: "A"}, {Name: "结构", Grade: "B"}, {Name: "内容", Grade: "B"},
		},
		Points: []Point{
			{Kind: "good", Quote: sp(" 去年秋天 "), Text: " 具体 ", Action: sp("不该有"), Source: "teacher"},
			{Kind: "issue", Quote: sp("  "), Text: "x", Action: sp("  ")},
		},
	}
	ai := NormalizeAI(c, r)
	if ai.Overall.Grade != "B" || ai.Dimensions[0].Name != "内容" || ai.Dimensions[3].Name != "书写规范" {
		t.Fatalf("overall/dimension order = %+v", ai)
	}
	if ai.Points[0].Action != nil || *ai.Points[0].Quote != "去年秋天" || ai.Points[0].Source != SourceAI {
		t.Fatalf("good point = %+v", ai.Points[0])
	}
	if ai.Points[1].Quote != nil || ai.Points[1].Action != nil {
		t.Fatalf("blank quote/action must become nil: %+v", ai.Points[1])
	}
	teacher := NormalizeTeacher(Content{Points: []Point{{Kind: "issue", Text: "a", Source: "ai"}, {Kind: "issue", Text: "b"}}}, r)
	if teacher.Points[0].Source != SourceAI || teacher.Points[1].Source != SourceTeacher {
		t.Fatalf("teacher sources = %+v", teacher.Points)
	}
	if teacher.Dimensions == nil {
		t.Fatal("dimensions must marshal as [] not null")
	}
}
```

Create `apps/api/internal/litegrade/prompt_test.go`:

```go
package litegrade

import (
	"strings"
	"testing"

	"mindimprint/api/internal/liteassign"
)

func TestSystemPromptCarriesTheRubric(t *testing.T) {
	in := testInput()
	in.Rubric.Focus = "重点看论证"
	in.SymptomCatalog = "【第 1 层 · 立意】\n- topic_without_question（只有主题，没有问题）：…\n"
	p := SystemPrompt(in)
	for _, want := range []string{"内容", "结构", "语言", "书写规范", "A+ A A- B+ B B- C+ C C- D", "重点看论证", "topic_without_question", "3 到 5 条", "用中文写", "「」"} {
		if !strings.Contains(p, want) {
			t.Errorf("system prompt lacks %q", want)
		}
	}
	in.Rubric = liteassign.Rubric{Scale: liteassign.ScalePoints, Max: 20, Dimensions: []liteassign.RubricDimension{{Name: "Argument", Note: "evidence"}}}
	in.Lang = "en"
	p = SystemPrompt(in)
	for _, want := range []string{"0 到 20 的整数", "Argument：evidence", "用英文写"} {
		if !strings.Contains(p, want) {
			t.Errorf("points/en prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "%!") {
		t.Fatalf("format verbs leaked: %s", p)
	}
}

func TestUserPromptLabelsTheTeachersText(t *testing.T) {
	in := testInput()
	in.TargetWords = 800
	p := UserPrompt(in)
	if !strings.Contains(p, "作业题目（老师布置，不是学生的原文）：\n"+testPrompt) {
		t.Fatalf("assigned prompt not labelled as the teacher's: %s", p)
	}
	if !strings.Contains(p, "学生正文（第 1 版）：\n"+testBody) || !strings.Contains(p, "目标字数：800") {
		t.Fatalf("body/target missing: %s", p)
	}
	in.AssignedPrompt = ""
	if strings.Contains(UserPrompt(in), "作业题目") {
		t.Fatal("no prompt line for a writing that is not homework")
	}
}

func TestRetryNudgeListsReasons(t *testing.T) {
	n := RetryNudge([]Reason{{Code: ReasonQuoteNotInBody, Where: "第 1 条意见", Detail: "雨一直下。"}, {Code: ReasonNoGoodPoint}})
	for _, want := range []string{"第 1 条意见的引文不在正文中：「雨一直下。」", "没有优点意见", "完整的 JSON"} {
		if !strings.Contains(n, want) {
			t.Errorf("nudge lacks %q:\n%s", want, n)
		}
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd apps/api && go test ./internal/litegrade -count=1`
Expected: FAIL to build (`no Go files` / `undefined: Content`).

- [ ] **Step 3: Write `content.go`**

Create `apps/api/internal/litegrade/content.go`:

```go
// Package litegrade holds the rules of lite AI 批改: the content shape, the
// prompt, and Check. Pure functions: no database, no HTTP. The model call,
// the retry and the grading rows live in internal/api (lite_grading_*.go).
package litegrade

import (
	"encoding/json"
	"errors"
	"strings"

	"mindimprint/api/internal/liteassign"
)

const (
	KindGood      = "good"
	KindIssue     = "issue"
	SourceAI      = "ai"
	SourceTeacher = "teacher"
	MinPoints     = 3
	MaxPoints     = 5
)

// Content is one grading: what the model returns, what the teacher edits and
// what the student reads.
type Content struct {
	Overall    Overall     `json:"overall"`
	Dimensions []Dimension `json:"dimensions"`
	Points     []Point     `json:"points"`
}

type Overall struct {
	Grade   string `json:"grade"`
	Comment string `json:"comment"`
}

type Dimension struct {
	Name    string `json:"name"`
	Grade   string `json:"grade"`
	Comment string `json:"comment"`
}

// Point is one strength (good) or issue. Quote is a sentence of hers, copied
// exactly; Action is what she does next and is required on an AI issue.
type Point struct {
	Kind   string  `json:"kind"`
	Quote  *string `json:"quote"`
	Text   string  `json:"text"`
	Action *string `json:"action"`
	Source string  `json:"source"`
}

// Input is everything the prompt and Check need about one submitted version.
type Input struct {
	Lang           string
	Title          string
	Body           string // the version body: the only text that counts as hers
	AssignedPrompt string // the teacher's prompt: never counts as hers
	TargetWords    int
	VersionNumber  int
	Rubric         liteassign.Rubric
	SymptomCatalog string // the writing room's symptom table, rendered
	// PersonJudging reports a sentence that judges the student instead of the
	// text. internal/api passes personDirectedVerdict; nil skips the check.
	PersonJudging func(string) bool
}

var ErrUnparseable = errors.New("litegrade: reply is not the JSON object asked for")

// Parse reads the outermost {...} of a reply, which also strips code fences.
func Parse(text string) (Content, error) {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return Content{}, ErrUnparseable
	}
	var c Content
	if err := json.Unmarshal([]byte(text[start:end+1]), &c); err != nil {
		return Content{}, ErrUnparseable
	}
	return c, nil
}

// NormalizeAI trims a model result, marks every point as the AI's, drops the
// action from good points and orders dimensions as the rubric lists them.
func NormalizeAI(c Content, r liteassign.Rubric) Content { return normalize(c, r, true) }

// NormalizeTeacher is NormalizeAI for a teacher's edit: a point keeps source
// "ai" only if it already had it; everything else is the teacher's.
func NormalizeTeacher(c Content, r liteassign.Rubric) Content { return normalize(c, r, false) }

func normalize(c Content, r liteassign.Rubric, fromAI bool) Content {
	out := Content{Overall: Overall{Grade: strings.TrimSpace(c.Overall.Grade), Comment: strings.TrimSpace(c.Overall.Comment)}}
	dims := make([]Dimension, 0, len(c.Dimensions))
	for _, d := range c.Dimensions {
		dims = append(dims, Dimension{Name: strings.TrimSpace(d.Name), Grade: strings.TrimSpace(d.Grade), Comment: strings.TrimSpace(d.Comment)})
	}
	out.Dimensions = orderDimensions(dims, r)
	out.Points = make([]Point, 0, len(c.Points))
	for _, p := range c.Points {
		q := Point{Kind: strings.TrimSpace(p.Kind), Quote: trimPtr(p.Quote), Text: strings.TrimSpace(p.Text), Action: trimPtr(p.Action)}
		if q.Kind == KindGood {
			q.Action = nil
		}
		switch {
		case fromAI, p.Source == SourceAI:
			q.Source = SourceAI
		default:
			q.Source = SourceTeacher
		}
		out.Points = append(out.Points, q)
	}
	return out
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// orderDimensions returns ds in rubric order when the names match exactly;
// otherwise ds unchanged, and Check reports the mismatch.
func orderDimensions(ds []Dimension, r liteassign.Rubric) []Dimension {
	if len(ds) != len(r.Dimensions) {
		return ds
	}
	byName := make(map[string]Dimension, len(ds))
	for _, d := range ds {
		byName[d.Name] = d
	}
	out := make([]Dimension, 0, len(ds))
	for _, rd := range r.Dimensions {
		d, ok := byName[rd.Name]
		if !ok {
			return ds
		}
		out = append(out, d)
	}
	return out
}
```

- [ ] **Step 4: Write `check.go`**

Create `apps/api/internal/litegrade/check.go`:

```go
package litegrade

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"mindimprint/api/internal/liteassign"
)

// Reason codes. Every rejection Check makes has a name, a test and a message
// the teacher reads after 「批改失败：」.
const (
	ReasonQuoteMissing       = "quote_missing"
	ReasonQuoteNotInBody     = "quote_not_in_body"
	ReasonQuotationNotInBody = "quotation_not_in_body"
	ReasonQuoteFromPrompt    = "quote_from_prompt"
	ReasonGradeOutOfScale    = "grade_out_of_scale"
	ReasonDimensionNames     = "dimension_names_mismatch"
	ReasonPointCount         = "point_count"
	ReasonNoGoodPoint        = "no_good_point"
	ReasonNoIssuePoint       = "no_issue_point"
	ReasonIssueWithoutAction = "issue_without_action"
	ReasonPersonJudging      = "person_judging"
	ReasonPointKind          = "invalid_point_kind"
	ReasonEmptyText          = "empty_text"
	ReasonUnparseable        = "unparseable"
	ReasonModelCall          = "model_call_failed"
)

type Reason struct {
	Code   string
	Where  string // 总评 / 维度「内容」 / 第 2 条意见 …
	Detail string
}

func (r Reason) Message() string {
	switch r.Code {
	case ReasonQuoteMissing:
		return r.Where + "没有引用原文"
	case ReasonQuoteNotInBody:
		return r.Where + "的引文不在正文中：「" + r.Detail + "」"
	case ReasonQuotationNotInBody:
		return r.Where + "里引号内的文字不在正文中：「" + r.Detail + "」"
	case ReasonQuoteFromPrompt:
		return r.Where + "引用的是作业题目，不是学生的原文：「" + r.Detail + "」"
	case ReasonGradeOutOfScale:
		return r.Where + "的等级不在评分标准内：" + r.Detail
	case ReasonDimensionNames:
		return "评分维度与评分标准不一致：" + r.Detail
	case ReasonPointCount:
		return "意见共 " + r.Detail + " 条，需要 3 到 5 条"
	case ReasonNoGoodPoint:
		return "没有优点意见"
	case ReasonNoIssuePoint:
		return "没有问题意见"
	case ReasonIssueWithoutAction:
		return r.Where + "没有修改建议"
	case ReasonPersonJudging:
		return r.Where + "评价的是学生本人，不是文字"
	case ReasonPointKind:
		return r.Where + "的类型无效：" + r.Detail
	case ReasonEmptyText:
		return r.Where + "为空"
	case ReasonUnparseable:
		return "回复不是有效的 JSON"
	case ReasonModelCall:
		return "模型调用失败：" + r.Detail
	}
	return r.Code
}

// JoinReasons is the grading row's error text: messages joined by 「；」,
// duplicates once.
func JoinReasons(rs []Reason) string { return strings.Join(reasonMessages(rs), "；") }

func reasonMessages(rs []Reason) []string {
	seen := make(map[string]bool, len(rs))
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		m := r.Message()
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// quotationPattern finds 「…」 spans in the model's own prose. Any text inside
// one must be hers: this is what keeps a rewritten sentence out of a comment.
var quotationPattern = regexp.MustCompile(`「([^「」]+)」`)

// Check is the gate for a model result. A result with any reason is retried
// once and then marked failed; it never reaches the teacher as a draft.
func Check(c Content, in Input) []Reason {
	rs := shapeReasons(c, in)
	rs = append(rs, textReasons("总评", c.Overall.Comment, in, true)...)
	for _, d := range c.Dimensions {
		rs = append(rs, textReasons("维度「"+d.Name+"」的评语", d.Comment, in, true)...)
	}
	if n := len(c.Points); n < MinPoints || n > MaxPoints {
		rs = append(rs, Reason{Code: ReasonPointCount, Detail: strconv.Itoa(n)})
	}
	good, issue := false, false
	for i, p := range c.Points {
		where := fmt.Sprintf("第 %d 条意见", i+1)
		switch p.Kind {
		case KindGood:
			good = true
		case KindIssue:
			issue = true
			if blank(p.Action) {
				rs = append(rs, Reason{Code: ReasonIssueWithoutAction, Where: where})
			}
		}
		if blank(p.Quote) {
			rs = append(rs, Reason{Code: ReasonQuoteMissing, Where: where})
		} else if r := quoteReason(where, *p.Quote, in, ReasonQuoteNotInBody); r != nil {
			rs = append(rs, *r)
		}
		rs = append(rs, textReasons(where+"的说明", p.Text, in, true)...)
		if p.Action != nil {
			rs = append(rs, textReasons(where+"的修改建议", *p.Action, in, false)...)
		}
	}
	if !good {
		rs = append(rs, Reason{Code: ReasonNoGoodPoint})
	}
	if !issue {
		rs = append(rs, Reason{Code: ReasonNoIssuePoint})
	}
	return rs
}

// CheckTeacherEdit is the shape check on a teacher's PATCH: grades in scale,
// the rubric's dimension names, known point kinds, non-empty point text, and a
// quote that is null or hers. No 3–5 limit, no action requirement.
func CheckTeacherEdit(c Content, in Input) []Reason {
	rs := shapeReasons(c, in)
	for i, p := range c.Points {
		where := fmt.Sprintf("第 %d 条意见", i+1)
		if strings.TrimSpace(p.Text) == "" {
			rs = append(rs, Reason{Code: ReasonEmptyText, Where: where + "的说明"})
		}
		if !blank(p.Quote) {
			if r := quoteReason(where, *p.Quote, in, ReasonQuoteNotInBody); r != nil {
				rs = append(rs, *r)
			}
		}
	}
	return rs
}

func shapeReasons(c Content, in Input) []Reason {
	var rs []Reason
	if !liteassign.GradeInScale(in.Rubric, c.Overall.Grade) {
		rs = append(rs, Reason{Code: ReasonGradeOutOfScale, Where: "总评", Detail: c.Overall.Grade})
	}
	for _, d := range c.Dimensions {
		if !liteassign.GradeInScale(in.Rubric, d.Grade) {
			rs = append(rs, Reason{Code: ReasonGradeOutOfScale, Where: "维度「" + d.Name + "」", Detail: d.Grade})
		}
	}
	if !sameNames(c.Dimensions, in.Rubric.Dimensions) {
		names := make([]string, 0, len(c.Dimensions))
		for _, d := range c.Dimensions {
			names = append(names, d.Name)
		}
		rs = append(rs, Reason{Code: ReasonDimensionNames, Detail: strings.Join(names, "、")})
	}
	for i, p := range c.Points {
		if p.Kind != KindGood && p.Kind != KindIssue {
			rs = append(rs, Reason{Code: ReasonPointKind, Where: fmt.Sprintf("第 %d 条意见", i+1), Detail: p.Kind})
		}
	}
	return rs
}

// textReasons checks one piece of the model's prose: present (when required),
// every 「」 quotation hers, and not about her as a person.
func textReasons(where, s string, in Input, required bool) []Reason {
	var rs []Reason
	if strings.TrimSpace(s) == "" {
		if required {
			rs = append(rs, Reason{Code: ReasonEmptyText, Where: where})
		}
		return rs
	}
	for _, m := range quotationPattern.FindAllStringSubmatch(s, -1) {
		if r := quoteReason(where, m[1], in, ReasonQuotationNotInBody); r != nil {
			rs = append(rs, *r)
		}
	}
	if in.PersonJudging != nil && in.PersonJudging(s) {
		rs = append(rs, Reason{Code: ReasonPersonJudging, Where: where})
	}
	return rs
}

// quoteReason: nil when q is part of her body. Text found only in the
// teacher's assigned prompt gets its own reason: the teacher's words are never hers.
func quoteReason(where, q string, in Input, notInBody string) *Reason {
	q = strings.TrimSpace(q)
	if q == "" || strings.Contains(in.Body, q) {
		return nil
	}
	if in.AssignedPrompt != "" && strings.Contains(in.AssignedPrompt, q) {
		return &Reason{Code: ReasonQuoteFromPrompt, Where: where, Detail: q}
	}
	return &Reason{Code: notInBody, Where: where, Detail: q}
}

func sameNames(got []Dimension, want []liteassign.RubricDimension) bool {
	if len(got) != len(want) {
		return false
	}
	left := make(map[string]int, len(want))
	for _, d := range want {
		left[d.Name]++
	}
	for _, d := range got {
		if left[d.Name] == 0 {
			return false
		}
		left[d.Name]--
	}
	return true
}

func blank(s *string) bool { return s == nil || strings.TrimSpace(*s) == "" }
```

- [ ] **Step 5: Write `prompt.go`**

Create `apps/api/internal/litegrade/prompt.go`:

```go
package litegrade

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/liteassign"
)

// systemTemplate. Every "must" in it is enforced by Check; the prompt only
// saves a retry. Verbs: scale line, dimension lines, focus line, symptom
// catalog, language name.
const systemTemplate = `你在为一位写作老师起草批改。学生已经提交了这篇作文，老师会审阅、修改你的批改，再发给学生。

你只给反馈，绝不替学生改：不要重写、不要润色、不要续写，不要给出可以直接替换原文的句子。

## 评分

%s
- overall.grade 是总评等级；dimensions 里每个维度一个等级。
- 维度只能是下面这些，名称逐字照抄，不增加、不减少：
%s%s
## 意见

- points 共 3 到 5 条，至少 1 条 good（她已经做好的地方），至少 1 条 issue（需要修改的地方）。
- 每条的 quote 从她的正文里逐字照抄一句话，包括标点。
- issue 必须有 action：一句祈使句，说清她接下来要做的事。写出要做的动作，不写改好的句子。
- good 的 action 写 null。
- 描述问题时可以用下面这张表里的毛病名称，不要在输出里写 id：

%s
## 引用

- 在 comment、text、action 里提到她写的话，一律用「」括起来，并且逐字照抄正文。
- 作业题目是老师写的，不是她写的，不要用「」引用题目。
- 「」里只能是她正文里原有的文字。

## 语气

- 对着文字说，不评价学生本人的能力或态度。
- 不写客套话。

## 语言

- comment、text、action 用%s写。

只输出一个 JSON 对象，不要输出其他文字：
{"overall":{"grade":"…","comment":"…"},"dimensions":[{"name":"…","grade":"…","comment":"…"}],"points":[{"kind":"good","quote":"…","text":"…","action":null},{"kind":"issue","quote":"…","text":"…","action":"…"}]}`

func SystemPrompt(in Input) string {
	return fmt.Sprintf(systemTemplate, scaleLine(in.Rubric), dimensionLines(in.Rubric), focusLine(in.Rubric), in.SymptomCatalog, languageName(in.Lang))
}

func scaleLine(r liteassign.Rubric) string {
	if r.Scale == liteassign.ScalePoints {
		return fmt.Sprintf("- 评分方式：分数。等级写 0 到 %d 的整数，例如 \"%d\"。", r.Max, r.Max)
	}
	return "- 评分方式：等级，只能从这些里选：" + strings.Join(liteassign.LetterGrades, " ") + "。"
}

func dimensionLines(r liteassign.Rubric) string {
	var b strings.Builder
	for _, d := range r.Dimensions {
		b.WriteString("  - " + d.Name)
		if d.Note != "" {
			b.WriteString("：" + d.Note)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func focusLine(r liteassign.Rubric) string {
	if r.Focus == "" {
		return ""
	}
	return "- 老师这次的批改重点：" + r.Focus + "\n"
}

func languageName(lang string) string {
	if lang == "en" {
		return "英文"
	}
	return "中文"
}

// UserPrompt: the teacher's prompt, labelled as the teacher's, then her version.
func UserPrompt(in Input) string {
	var b strings.Builder
	if in.AssignedPrompt != "" {
		b.WriteString("作业题目（老师布置，不是学生的原文）：\n" + in.AssignedPrompt + "\n\n")
	}
	if in.Title != "" {
		b.WriteString("标题：" + in.Title + "\n")
	}
	if in.TargetWords > 0 {
		fmt.Fprintf(&b, "目标字数：%d\n", in.TargetWords)
	}
	fmt.Fprintf(&b, "\n学生正文（第 %d 版）：\n%s\n", in.VersionNumber, in.Body)
	return b.String()
}

// RetryNudge is the user turn of the one retry: what failed, then the ask.
func RetryNudge(rs []Reason) string {
	var b strings.Builder
	b.WriteString("上一份批改没有通过检查：\n")
	for _, m := range reasonMessages(rs) {
		b.WriteString("- " + m + "\n")
	}
	b.WriteString("\n请修正这些问题，重新输出完整的 JSON。")
	return b.String()
}
```

- [ ] **Step 6: Run the package tests**

Run: `cd apps/api && go test ./internal/litegrade -count=1 -v 2>&1 | tail -40`
Expected: every test PASS. If a `TestCheckRejections` case fails because a mutation also triggers a different code, the assertion is "contains", so fix the mutation, not `Check`.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/litegrade
git commit -m "feat(lite): litegrade prompt, parser and Check for AI grading

Check rejects quotes that are not in her body, quotations of the teacher's
prompt, grades outside the rubric, wrong dimensions, point counts outside
3-5, missing good or issue points, issues without an action, and sentences
that judge the student.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Migration 0154, grading queries, sqlc

**Files:**
- Create: `apps/api/internal/store/migrations/0154_lite_grading.sql`
- Create: `apps/api/internal/store/queries/lite_grading.sql`
- Regenerate: `apps/api/internal/store/sqlc/*`

**Interfaces:**
- Produces (package `sqlc`; confirm the generated names in Step 4):
  - `type LiteGrading struct { ID, AtomID, VersionID, UserID, ClassID uuid.UUID; AssignmentID pgtype.UUID; Rubric []byte; Status string; Ai, Content []byte; Error *string; RequestedBy uuid.UUID; ReviewedAt, SentAt, StudentSeenAt pgtype.Timestamptz; CreatedAt, UpdatedAt time.Time }`
  - `CreateLiteGrading(ctx, CreateLiteGradingParams{AtomID, VersionID, UserID, ClassID uuid.UUID; AssignmentID pgtype.UUID; Rubric []byte; RequestedBy uuid.UUID}) (LiteGrading, error)` — `pgx.ErrNoRows` when the version already has a row
  - `GetLiteGrading(ctx, id) (LiteGrading, error)`; `GetLiteGradingByVersion(ctx, versionID) (LiteGrading, error)`
  - `ListLiteGradingsForVersions(ctx, versionIds []uuid.UUID) ([]LiteGrading, error)`
  - `ListLatestWritingVersionsForAtoms(ctx, atomIds []uuid.UUID) ([]ListLatestWritingVersionsForAtomsRow, error)` — row `{ID, AtomID uuid.UUID; Number int32; SubmittedAt time.Time}`
  - `GetLiteGradingSource(ctx, versionID) (GetLiteGradingSourceRow, error)` — row `{VersionID, AtomID uuid.UUID; Number int32; Title, Body, Lang string; AssignedPrompt *string; TargetWords *int32}`
  - `ClaimLiteGrading(ctx, id) (LiteGrading, error)`
  - `SetLiteGradingDraft(ctx, SetLiteGradingDraftParams{Result []byte; ID uuid.UUID}) (int64, error)`
  - `SetLiteGradingFailed(ctx, SetLiteGradingFailedParams{Error *string; ID uuid.UUID}) (int64, error)`
  - `RequeueLiteGrading(ctx, RequeueLiteGradingParams{ID uuid.UUID; Rubric []byte; RequestedBy uuid.UUID}) (LiteGrading, error)`
  - `MarkStaleLiteGradingsFailed(ctx, ids []uuid.UUID) error`
  - `UpdateLiteGradingContent(ctx, UpdateLiteGradingContentParams{Content []byte; ID uuid.UUID}) (LiteGrading, error)`
  - `SendLiteGrading(ctx, id) (LiteGrading, error)`
  - `SendReviewedLiteGradings(ctx, SendReviewedLiteGradingsParams{AssignmentID pgtype.UUID; Ids []uuid.UUID}) (int64, error)`
  - `ListSentLiteGradingsForAtom(ctx, atomID) ([]ListSentLiteGradingsForAtomRow, error)` — row `{ID uuid.UUID; Rubric, Content []byte; SentAt, StudentSeenAt pgtype.Timestamptz; VersionNumber int32}`
  - `ListLiteInboxGradings(ctx, userID) ([]ListLiteInboxGradingsRow, error)` — row `{ID, AtomID uuid.UUID; SentAt, StudentSeenAt pgtype.Timestamptz; Title string}`
  - `MarkLiteGradingSeen(ctx, MarkLiteGradingSeenParams{ID, UserID uuid.UUID}) (int64, error)`

This task has no test of its own: the migration and every query are exercised by the handler tests in Tasks 5–7, and `go build` proves the generated code compiles.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0154_lite_grading.sql`:

```sql
-- +goose Up
-- 老师的 AI 批改（lite）。一行对应学生写作的一个提交版本。
-- ai：通过检查的模型结果，写入后不修改。content：老师编辑、发送的内容，初值是 ai 的副本。
-- 学生只读 status = 'sent' 的行。
-- user_id / class_id：学生与批改时所在的班级；教师接口按 class_id 判断归属。
CREATE TABLE lite_grading (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id         uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  version_id      uuid NOT NULL UNIQUE REFERENCES writing_version(id) ON DELETE CASCADE,
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  class_id        uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  assignment_id   uuid REFERENCES lite_assignment(id) ON DELETE SET NULL,
  rubric          jsonb NOT NULL,
  status          text NOT NULL DEFAULT 'queued'
                  CHECK (status IN ('queued', 'running', 'draft', 'failed', 'sent')),
  ai              jsonb,
  content         jsonb,
  error           text,
  requested_by    uuid NOT NULL REFERENCES users(id),
  reviewed_at     timestamptz,
  sent_at         timestamptz,
  student_seen_at timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (status <> 'sent' OR content IS NOT NULL)
);
CREATE INDEX lite_grading_atom_idx ON lite_grading (atom_id);
CREATE INDEX lite_grading_assignment_idx ON lite_grading (assignment_id) WHERE assignment_id IS NOT NULL;
CREATE INDEX lite_grading_user_sent_idx ON lite_grading (user_id, sent_at DESC) WHERE status = 'sent';

-- +goose Down
DROP TABLE lite_grading;
```

- [ ] **Step 2: Write the queries**

Create `apps/api/internal/store/queries/lite_grading.sql`:

```sql
-- Lite AI 批改（0154）。

-- name: CreateLiteGrading :one
-- 一个版本只有一行：已有一行时不插入，返回 no rows。
INSERT INTO lite_grading (atom_id, version_id, user_id, class_id, assignment_id, rubric, requested_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (version_id) DO NOTHING
RETURNING *;

-- name: GetLiteGrading :one
SELECT * FROM lite_grading WHERE id = $1;

-- name: GetLiteGradingByVersion :one
SELECT * FROM lite_grading WHERE version_id = $1;

-- name: ListLiteGradingsForVersions :many
SELECT * FROM lite_grading WHERE version_id = ANY(sqlc.arg(version_ids)::uuid[]);

-- name: ListLatestWritingVersionsForAtoms :many
-- 每篇写作的最新提交版本，不带正文。
SELECT DISTINCT ON (atom_id) id, atom_id, number, submitted_at
FROM writing_version
WHERE atom_id = ANY(sqlc.arg(atom_ids)::uuid[])
ORDER BY atom_id, number DESC;

-- name: GetLiteGradingSource :one
-- 批改读的那一版正文，以及写作的语言、老师的题目和目标字数。
SELECT v.id AS version_id, v.atom_id, v.number, v.title, v.body,
       w.lang, w.assigned_prompt, w.target_words
FROM writing_version v
JOIN writing w ON w.atom_id = v.atom_id
WHERE v.id = $1;

-- name: ClaimLiteGrading :one
-- 只有排队中的行可以开始，同一行不会被两个任务同时批改。
UPDATE lite_grading SET status = 'running', error = NULL, updated_at = now()
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: SetLiteGradingDraft :execrows
-- 批改成功：ai 与 content 都是这次的结果；之前的审阅作废。
UPDATE lite_grading
SET status = 'draft', ai = sqlc.arg(result), content = sqlc.arg(result),
    reviewed_at = NULL, error = NULL, updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'running';

-- name: SetLiteGradingFailed :execrows
UPDATE lite_grading SET status = 'failed', error = sqlc.arg(error), updated_at = now()
WHERE id = sqlc.arg(id) AND status IN ('queued', 'running');

-- name: RequeueLiteGrading :one
-- 重新批改：草稿或失败的行回到排队，评分标准换成当前的。已发送的行不重新批改。
UPDATE lite_grading
SET status = 'queued', error = NULL, rubric = $2, requested_by = $3, updated_at = now()
WHERE id = $1 AND status IN ('draft', 'failed')
RETURNING *;

-- name: MarkStaleLiteGradingsFailed :exec
-- 任务只跑一次（MaxAttempts 1），超时 6 分钟。进程在批改中途退出时这一行会停在 running，
-- 超过 15 分钟没有更新就判为失败。排队中的行不动：队列可能还没轮到它。
UPDATE lite_grading
SET status = 'failed', error = '批改超时：任务未完成', updated_at = now()
WHERE status = 'running' AND updated_at < now() - interval '15 minutes'
  AND id = ANY(sqlc.arg(ids)::uuid[]);

-- name: UpdateLiteGradingContent :one
-- 老师保存即算审阅。已发送的行保存后重新发送：学生那边变为未读。
UPDATE lite_grading
SET content = COALESCE(sqlc.narg(content)::jsonb, content),
    reviewed_at = now(),
    sent_at = CASE WHEN status = 'sent' THEN now() ELSE sent_at END,
    student_seen_at = CASE WHEN status = 'sent' THEN NULL ELSE student_seen_at END,
    updated_at = now()
WHERE id = sqlc.arg(id) AND status IN ('draft', 'sent') AND content IS NOT NULL
RETURNING *;

-- name: SendLiteGrading :one
UPDATE lite_grading
SET status = 'sent', sent_at = now(), student_seen_at = NULL,
    reviewed_at = COALESCE(reviewed_at, now()), updated_at = now()
WHERE id = $1 AND status IN ('draft', 'sent') AND content IS NOT NULL
RETURNING *;

-- name: SendReviewedLiteGradings :execrows
-- 发送全部已审阅：只发这份作业里已审阅的草稿。
UPDATE lite_grading
SET status = 'sent', sent_at = now(), student_seen_at = NULL, updated_at = now()
WHERE assignment_id = sqlc.arg(assignment_id) AND id = ANY(sqlc.arg(ids)::uuid[])
  AND status = 'draft' AND reviewed_at IS NOT NULL AND content IS NOT NULL;

-- name: ListSentLiteGradingsForAtom :many
-- 学生读的批改：只有已发送的行。
SELECT g.id, g.rubric, g.content, g.sent_at, g.student_seen_at, v.number AS version_number
FROM lite_grading g
JOIN writing_version v ON v.id = g.version_id
WHERE g.atom_id = $1 AND g.status = 'sent'
ORDER BY v.number DESC;

-- name: ListLiteInboxGradings :many
SELECT g.id, g.atom_id, g.sent_at, g.student_seen_at, w.title
FROM lite_grading g
JOIN writing w ON w.atom_id = g.atom_id
WHERE g.user_id = $1 AND g.status = 'sent'
ORDER BY g.sent_at DESC;

-- name: MarkLiteGradingSeen :execrows
UPDATE lite_grading SET student_seen_at = COALESCE(student_seen_at, now())
WHERE id = $1 AND user_id = $2 AND status = 'sent';
```

- [ ] **Step 3: Regenerate sqlc**

Run: `cd apps/api && CGO_ENABLED=0 go tool sqlc generate && git status --short internal/store/sqlc`
Expected: `M internal/store/sqlc/models.go` and `?? internal/store/sqlc/lite_grading.sql.go`. If nothing is listed, use the fallback command in Global Constraints.

- [ ] **Step 4: Confirm the generated names**

Run: `cd apps/api && grep -n "^type .*Params struct" -A6 internal/store/sqlc/lite_grading.sql.go && grep -n "^func (q \*Queries)" internal/store/sqlc/lite_grading.sql.go && grep -n "type LiteGrading struct" -A19 internal/store/sqlc/models.go`
Expected: the names and field types listed under **Interfaces**. Two to watch: `SetLiteGradingFailedParams.Error` is `*string` and `SendReviewedLiteGradingsParams.Ids` is `[]uuid.UUID`. If sqlc named a field differently (for example `VersionIds`), later tasks use the generated name; do not edit generated files.

- [ ] **Step 5: Build**

Run: `cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./internal/store/...`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/migrations/0154_lite_grading.sql \
  apps/api/internal/store/queries/lite_grading.sql apps/api/internal/store/sqlc
git commit -m "feat(lite): lite_grading table and queries (0154)

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Grading run — retry loop, river job, worker

**Files:**
- Create: `apps/api/internal/api/lite_grading_run.go`
- Create: `apps/api/internal/api/lite_grading_jobs.go`
- Modify: `apps/api/internal/api/interest_jobs.go` (`StartHarvestQueue`)
- Test: `apps/api/internal/api/lite_grading_run_internal_test.go`

**Interfaces:**
- Consumes: `litegrade.*` (Task 2); `sqlc.GetLiteGradingSourceRow`, `ClaimLiteGrading`, `SetLiteGradingDraft`, `SetLiteGradingFailed` (Task 3); existing `a.routeE`, `a.recordLiteLLMCall`, `a.studentEntitled`, `writingSymptomCatalog`, `personDirectedVerdict`, `gateway.Collect`.
- Produces (package `api`):
  - `func gradeWithRetry(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in litegrade.Input, record func(gateway.ChatUsage)) (litegrade.Content, []litegrade.Reason, int)` — content, reasons (empty on success), attempts made
  - `func liteGradingInput(src sqlc.GetLiteGradingSourceRow, rubric liteassign.Rubric) litegrade.Input`
  - `type LiteGradingArgs struct { GradingID uuid.UUID }` — `Kind() == "lite_grading"`, `InsertOpts() == river.InsertOpts{MaxAttempts: 1, Queue: "lite_grading"}`
  - `type LiteGradingWorker struct { river.WorkerDefaults[LiteGradingArgs]; API *API }` — `Timeout` 6 minutes; `Work` runs `a.runLiteGrading` and returns nil
  - `func (a *API) runLiteGrading(ctx context.Context, id uuid.UUID)`
  - `func (a *API) enqueueLiteGrading(ctx context.Context, id uuid.UUID) bool` — caller has checked `a.d.River != nil`
  - `func RegisterLiteGradingWorker(w *river.Workers, a *API) error`
  - `const liteGradingPurpose = "teacher_grading"`

- [ ] **Step 1: Write the failing internal test**

Create `apps/api/internal/api/lite_grading_run_internal_test.go`:

```go
package api

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/store/sqlc"
)

const runTestBody = "学校后门那片空地一下雨就积水。去年秋天，我在那里摔过一跤。\n\n我读到城市里的雨水花园：用下凹的绿地先把雨水接住。"

// runTestValid passes Check against runTestBody and the zh default rubric.
const runTestValid = `{"overall":{"grade":"B+","comment":"用「去年秋天，我在那里摔过一跤。」引出问题。"},
"dimensions":[{"name":"内容","grade":"B+","comment":"问题来自亲身经历。"},{"name":"结构","grade":"B","comment":"两段之间没有过渡句。"},{"name":"语言","grade":"A-","comment":"表达清楚。"},{"name":"书写规范","grade":"A","comment":"标点使用正确。"}],
"points":[{"kind":"good","quote":"去年秋天，我在那里摔过一跤。","text":"用具体经历引出问题。","action":null},
{"kind":"issue","quote":"我读到城市里的雨水花园：用下凹的绿地先把雨水接住。","text":"材料与后门空地之间没有说明联系。","action":"在这句后面写一句说明雨水花园和后门空地的关系。"},
{"kind":"issue","quote":"学校后门那片空地一下雨就积水。","text":"积水的程度没有数据。","action":"补充一次积水的深度或持续时间。"}]}`

func runTestScript(text string) []gateway.StreamEvent {
	return []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 100, OutputTokens: 50}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
}

func runTestInput() litegrade.Input {
	return liteGradingInput(sqlc.GetLiteGradingSourceRow{Number: 1, Title: "雨水去哪儿了", Body: runTestBody, Lang: "zh"}, liteassign.DefaultRubric("zh"))
}

func TestGradeWithRetryFirstReplyPasses(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(runTestScript(runTestValid))
	calls := 0
	c, reasons, attempts := gradeWithRetry(context.Background(), prov, gateway.Resolved{Provider: "stub"}, runTestInput(), func(gateway.ChatUsage) { calls++ })
	if len(reasons) != 0 || attempts != 1 || prov.Calls != 1 || calls != 1 {
		t.Fatalf("reasons=%v attempts=%d calls=%d metered=%d", reasons, attempts, prov.Calls, calls)
	}
	if c.Overall.Grade != "B+" || c.Points[0].Source != litegrade.SourceAI {
		t.Fatalf("content = %+v", c)
	}
}

func TestGradeWithRetryCarriesReasonsIntoTheRetry(t *testing.T) {
	bad := strings.Replace(runTestValid, `"quote":"去年秋天，我在那里摔过一跤。"`, `"quote":"去年冬天，我在那里摔过一跤。"`, 1)
	prov := gateway.NewSequenceStubProvider(runTestScript(bad), runTestScript(runTestValid))
	calls := 0
	_, reasons, attempts := gradeWithRetry(context.Background(), prov, gateway.Resolved{Provider: "stub"}, runTestInput(), func(gateway.ChatUsage) { calls++ })
	if len(reasons) != 0 || attempts != 2 || calls != 2 {
		t.Fatalf("reasons=%v attempts=%d metered=%d", reasons, attempts, calls)
	}
	msgs := prov.LastRequest.Messages
	last := msgs[len(msgs)-1]
	if last.Role != gateway.RoleUser || !strings.Contains(last.Content, "第 1 条意见的引文不在正文中：「去年冬天，我在那里摔过一跤。」") {
		t.Fatalf("retry turn = %+v", last)
	}
	if msgs[len(msgs)-2].Role != gateway.RoleAssistant || msgs[len(msgs)-2].Content != bad {
		t.Fatalf("the retry must carry the rejected reply, got %+v", msgs[len(msgs)-2])
	}
}

func TestGradeWithRetryGivesUpAfterTwo(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(runTestScript("抱歉，我无法批改。"))
	calls := 0
	_, reasons, attempts := gradeWithRetry(context.Background(), prov, gateway.Resolved{Provider: "stub"}, runTestInput(), func(gateway.ChatUsage) { calls++ })
	if attempts != 2 || prov.Calls != 2 || calls != 2 || len(reasons) != 1 || reasons[0].Code != litegrade.ReasonUnparseable {
		t.Fatalf("reasons=%v attempts=%d calls=%d metered=%d", reasons, attempts, prov.Calls, calls)
	}
}

// liteGradingInput must wire the writing room's person check and symptom
// table; litegrade cannot import them itself.
func TestLiteGradingInputWiring(t *testing.T) {
	prompt := "写一篇关于雨的记叙文"
	target := int32(800)
	in := liteGradingInput(sqlc.GetLiteGradingSourceRow{Number: 2, Title: "雨", Body: "x", Lang: "en", AssignedPrompt: &prompt, TargetWords: &target}, liteassign.DefaultRubric("en"))
	if in.PersonJudging == nil || !in.PersonJudging("你很懒") {
		t.Fatal("PersonJudging must be personDirectedVerdict")
	}
	if in.SymptomCatalog != writingSymptomCatalog("en") || in.AssignedPrompt != prompt || in.TargetWords != 800 || in.VersionNumber != 2 {
		t.Fatalf("input = %+v", in)
	}
}

func TestLiteGradingArgsRunOnce(t *testing.T) {
	opts := LiteGradingArgs{}.InsertOpts()
	if opts.MaxAttempts != 1 || opts.Queue != liteGradingQueue || (LiteGradingArgs{}).Kind() != "lite_grading" {
		t.Fatalf("insert opts = %+v", opts)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestGradeWithRetry|TestLiteGradingInputWiring|TestLiteGradingArgsRunOnce' -count=1 -short`
Expected: FAIL to build (`undefined: gradeWithRetry`). `-short` skips the package's DB tests; these tests use no database.

- [ ] **Step 3: Write `lite_grading_run.go`**

Create `apps/api/internal/api/lite_grading_run.go`:

```go
package api

// lite_grading_run.go — one AI 批改 attempt pair: call the review model, parse,
// litegrade.Check, and retry once with the reasons. No database here; the
// worker in lite_grading_jobs.go owns the row.

import (
	"context"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/store/sqlc"
)

const (
	liteGradingPurpose     = "teacher_grading"
	liteGradingCallTimeout = 150 * time.Second
	liteGradingAttempts    = 2
)

// liteGradingInput builds litegrade's input from the version being graded.
// The person check and the symptom table are the writing room's own.
func liteGradingInput(src sqlc.GetLiteGradingSourceRow, rubric liteassign.Rubric) litegrade.Input {
	prompt := ""
	if src.AssignedPrompt != nil {
		prompt = strings.TrimSpace(*src.AssignedPrompt)
	}
	target := 0
	if src.TargetWords != nil {
		target = int(*src.TargetWords)
	}
	return litegrade.Input{
		Lang: src.Lang, Title: src.Title, Body: src.Body, AssignedPrompt: prompt,
		TargetWords: target, VersionNumber: int(src.Number), Rubric: rubric,
		SymptomCatalog: writingSymptomCatalog(src.Lang),
		PersonJudging:  personDirectedVerdict,
	}
}

// gradeWithRetry makes at most two model calls. A reply that fails Check is
// sent back with the reasons (and the rejected reply, so the model can fix
// it); a failed call is retried with the same messages. record is called for
// every call, because a call costs money whatever its reply.
func gradeWithRetry(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in litegrade.Input, record func(gateway.ChatUsage)) (litegrade.Content, []litegrade.Reason, int) {
	msgs := []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: litegrade.SystemPrompt(in)},
		{Role: gateway.RoleUser, Content: litegrade.UserPrompt(in)},
	}
	var reasons []litegrade.Reason
	for attempt := 1; attempt <= liteGradingAttempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, liteGradingCallTimeout)
		res, err := gateway.Collect(callCtx, prov, resolved, gateway.ChatRequest{Messages: msgs})
		cancel()
		record(res.Usage)
		if err != nil {
			reasons = []litegrade.Reason{{Code: litegrade.ReasonModelCall, Detail: err.Error()}}
			continue
		}
		content, perr := litegrade.Parse(res.Text)
		if perr != nil {
			reasons = []litegrade.Reason{{Code: litegrade.ReasonUnparseable}}
		} else {
			content = litegrade.NormalizeAI(content, in.Rubric)
			if reasons = litegrade.Check(content, in); len(reasons) == 0 {
				return content, nil, attempt
			}
		}
		msgs = append(msgs,
			gateway.ChatMessage{Role: gateway.RoleAssistant, Content: res.Text},
			gateway.ChatMessage{Role: gateway.RoleUser, Content: litegrade.RetryNudge(reasons)},
		)
	}
	return litegrade.Content{}, reasons, liteGradingAttempts
}
```

- [ ] **Step 4: Write `lite_grading_jobs.go`**

Create `apps/api/internal/api/lite_grading_jobs.go`:

```go
package api

// lite_grading_jobs.go — the river job behind 一键AI批改. One job per grading
// row. MaxAttempts is 1 so river never runs (and bills) a job twice; the one
// retry happens inside gradeWithRetry.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/store/sqlc"
)

const (
	liteGradingQueue = "lite_grading"
	// Two calls of up to 150s each, plus the database work. River's default
	// job timeout (1 minute) would cancel the first call.
	liteGradingJobTimeout = 6 * time.Minute
)

type LiteGradingArgs struct {
	GradingID uuid.UUID `json:"grading_id"`
}

func (LiteGradingArgs) Kind() string { return "lite_grading" }

func (LiteGradingArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1, Queue: liteGradingQueue}
}

type LiteGradingWorker struct {
	river.WorkerDefaults[LiteGradingArgs]
	API *API
}

func (w *LiteGradingWorker) Timeout(*river.Job[LiteGradingArgs]) time.Duration {
	return liteGradingJobTimeout
}

// Work always reports success: the outcome, including a failure, is written
// on the grading row, and river must not retry.
func (w *LiteGradingWorker) Work(ctx context.Context, job *river.Job[LiteGradingArgs]) error {
	w.API.runLiteGrading(ctx, job.Args.GradingID)
	return nil
}

func RegisterLiteGradingWorker(w *river.Workers, a *API) error {
	return river.AddWorkerSafely(w, &LiteGradingWorker{API: a})
}

// enqueueLiteGrading inserts the job for a queued row. A failed insert marks
// the row failed with the queue's error, so the teacher reads
// 「批改失败：入队失败：…」 instead of a row that stays 批改中.
// Callers check a.d.River != nil first.
func (a *API) enqueueLiteGrading(ctx context.Context, id uuid.UUID) bool {
	if _, err := a.d.River.Insert(ctx, LiteGradingArgs{GradingID: id}, nil); err != nil {
		a.failLiteGrading(ctx, id, "入队失败："+err.Error())
		return false
	}
	return true
}

func (a *API) failLiteGrading(ctx context.Context, id uuid.UUID, msg string) {
	if _, err := a.d.Queries.SetLiteGradingFailed(ctx, sqlc.SetLiteGradingFailedParams{ID: id, Error: &msg}); err != nil {
		slog.Warn("lite grading: mark failed", "err", err, "grading_id", id)
	}
}

// runLiteGrading claims a queued row, grades its version and writes the
// result: draft with ai = content, or failed with the reasons. A row that is
// no longer queued (regraded, or already claimed) is left alone.
func (a *API) runLiteGrading(ctx context.Context, id uuid.UUID) {
	g, err := a.d.Queries.ClaimLiteGrading(ctx, id)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("lite grading: claim", "err", err, "grading_id", id)
		}
		return
	}
	entitled, err := a.studentEntitled(ctx, g.UserID)
	if err != nil {
		slog.Warn("lite grading: entitlement", "err", err, "grading_id", id)
		a.failLiteGrading(ctx, id, "服务器内部错误")
		return
	}
	if !entitled {
		a.failLiteGrading(ctx, id, httpx.ErrNotEntitled().Message)
		return
	}
	src, err := a.d.Queries.GetLiteGradingSource(ctx, g.VersionID)
	if err != nil {
		slog.Warn("lite grading: load version", "err", err, "grading_id", id)
		a.failLiteGrading(ctx, id, "服务器内部错误")
		return
	}
	var rubric liteassign.Rubric
	if err := json.Unmarshal(g.Rubric, &rubric); err != nil {
		slog.Warn("lite grading: rubric", "err", err, "grading_id", id)
		a.failLiteGrading(ctx, id, "服务器内部错误")
		return
	}
	resolved, err := a.routeE(ctx, gateway.ClassReview)
	if err != nil {
		a.failLiteGrading(ctx, id, "模型不可用："+err.Error())
		return
	}
	content, reasons, attempts := gradeWithRetry(ctx, a.d.Provider, resolved, liteGradingInput(src, rubric), func(u gateway.ChatUsage) {
		a.recordLiteLLMCall(ctx, g.UserID, g.AtomID, liteGradingPurpose, resolved, u)
	})
	if len(reasons) > 0 {
		slog.Info("lite grading: failed", "grading_id", id, "attempts", attempts, "reasons", litegrade.JoinReasons(reasons))
		a.failLiteGrading(ctx, id, litegrade.JoinReasons(reasons))
		return
	}
	result, err := json.Marshal(content)
	if err != nil {
		a.failLiteGrading(ctx, id, "服务器内部错误")
		return
	}
	if _, err := a.d.Queries.SetLiteGradingDraft(ctx, sqlc.SetLiteGradingDraftParams{ID: id, Result: result}); err != nil {
		slog.Warn("lite grading: store draft", "err", err, "grading_id", id)
	}
}
```

- [ ] **Step 5: Register the worker and its queue**

In `apps/api/internal/api/interest_jobs.go`, `StartHarvestQueue`, after `RegisterHarvestWorkers`:

```go
	if err := RegisterLiteGradingWorker(workers, a); err != nil {
		return nil, err
	}
```

and replace the `Queues:` line with:

```go
		// AI 批改 has its own queue so a class of forty never holds up
		// interest harvesting. Two workers = two gradings calling the model at once.
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 4},
			liteGradingQueue:   {MaxWorkers: 2},
		},
```

- [ ] **Step 6: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestGradeWithRetry|TestLiteGradingInputWiring|TestLiteGradingArgsRunOnce' -count=1 -short && CGO_ENABLED=0 go build ./...`
Expected: PASS, then no build output.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/lite_grading_run.go apps/api/internal/api/lite_grading_jobs.go \
  apps/api/internal/api/interest_jobs.go apps/api/internal/api/lite_grading_run_internal_test.go
git commit -m "feat(lite): river job for AI grading with one checked retry

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Teacher grading routes — list, queue, regrade, read

**Files:**
- Create: `apps/api/internal/api/lite_teacher_gradings.go`
- Modify: `apps/api/internal/api/lite_teacher_routes.go`
- Test: `apps/api/internal/api/lite_grading_test.go`

**Interfaces:**
- Consumes: Tasks 1–4; existing `loadTeacherAssignment`, `authTeacherStudent`, `loadTeacherOwnedAtom`, `assertTeacherOwnsClass`, `writeNotFoundOr`, `tsStringPtr`, `uuidStringPtr`.
- Produces (HTTP):
  - `GET /api/v1/lite/teacher/assignments/{aid}/gradings` → `{"rows": [{userId, displayName, atomId|null, version: {number, submittedAt}|null, grading: GradingSummary|null}]}`. 400 `not_writing_assignment` 「只有写作作业可以批改」.
  - `POST /api/v1/lite/teacher/assignments/{aid}/gradings` body `{retryFailed?: bool}` → `{"queued": n}`. 503 `grading_queue_unavailable` 「批改队列未启动」.
  - `POST /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}/gradings` → `{"grading": TeacherGrading}`. 409 `no_submission` 「这名学生还没有提交」, 409 `grading_sent` 「这一版的批改已发送，不能重新批改」, 409 `grading_in_progress` 「批改中」.
  - `POST /api/v1/lite/teacher/gradings/{gid}/regrade` → `{"grading": TeacherGrading}` (same 409s, 503).
  - `GET /api/v1/lite/teacher/gradings/{gid}` → `{"grading": TeacherGrading}`.
  - `GradingSummary = {id, status, overallGrade|null, error|null, reviewedAt|null, sentAt|null}`
  - `TeacherGrading = {id, classId, assignmentId|null, userId, displayName, atomId, versionNumber, latestVersionNumber, title, body, lang, rubric, status, content|null, error|null, reviewedAt|null, sentAt|null, studentSeenAt|null, updatedAt}`
- Produces (Go): `func (a *API) loadTeacherGrading(w, r) (sqlc.LiteGrading, bool)`; `func (a *API) teacherGradingResponse(w, r, g sqlc.LiteGrading)`; `type gradingSummaryDTO`; `func gradingSummaryOf(g sqlc.LiteGrading) gradingSummaryDTO`; `func (a *API) gradingRubricFor(ctx, atomID, ownerID uuid.UUID) ([]byte, pgtype.UUID, error)`.

- [ ] **Step 1: Write the failing handler tests**

Create `apps/api/internal/api/lite_grading_test.go`:

```go
package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

const gradingBody = "学校后门那片空地一下雨就积水。去年秋天，我在那里摔过一跤。\n\n我读到城市里的雨水花园：用下凹的绿地先把雨水接住。"

// gradingValidReply passes litegrade.Check against gradingBody and the zh default rubric.
const gradingValidReply = `{"overall":{"grade":"B+","comment":"用「去年秋天，我在那里摔过一跤。」引出问题。"},
"dimensions":[{"name":"内容","grade":"B+","comment":"问题来自亲身经历。"},{"name":"结构","grade":"B","comment":"两段之间没有过渡句。"},{"name":"语言","grade":"A-","comment":"表达清楚。"},{"name":"书写规范","grade":"A","comment":"标点使用正确。"}],
"points":[{"kind":"good","quote":"去年秋天，我在那里摔过一跤。","text":"用具体经历引出问题。","action":null},
{"kind":"issue","quote":"我读到城市里的雨水花园：用下凹的绿地先把雨水接住。","text":"材料与后门空地之间没有说明联系。","action":"在这句后面写一句说明雨水花园和后门空地的关系。"},
{"kind":"issue","quote":"学校后门那片空地一下雨就积水。","text":"积水的程度没有数据。","action":"补充一次积水的深度或持续时间。"}]}`

var gradingBadReply = strings.Replace(gradingValidReply, `"quote":"去年秋天，我在那里摔过一跤。"`, `"quote":"去年冬天，我在那里摔过一跤。"`, 1)

// fakeEnqueuer stands in for river: it records jobs, and the test runs them
// through the real worker.
type fakeEnqueuer struct {
	mu   sync.Mutex
	args []LiteGradingArgs
	fail error
}

func (f *fakeEnqueuer) Insert(_ context.Context, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	if a, ok := args.(LiteGradingArgs); ok {
		f.args = append(f.args, a)
	}
	return &rivertype.JobInsertResult{Job: &rivertype.JobRow{}}, nil
}

func (f *fakeEnqueuer) drain() []LiteGradingArgs {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.args
	f.args = nil
	return out
}

type gradingFixture struct {
	h         http.Handler
	a         *API
	pool      *pgxpool.Pool
	teacher   *http.Cookie
	classID   string
	studentID uuid.UUID
	enq       *fakeEnqueuer
	prov      *gateway.SequenceStubProvider
}

func gradingScript(text string) []gateway.StreamEvent {
	return []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 100, OutputTokens: 50}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
}

// newGradingFixture: liteTeacherFixture with a queue and a scripted model.
// Replies are played in order; the last one repeats.
func newGradingFixture(t *testing.T, replies ...string) *gradingFixture {
	t.Helper()
	if len(replies) == 0 {
		replies = []string{gradingValidReply}
	}
	scripts := make([][]gateway.StreamEvent, 0, len(replies))
	for _, r := range replies {
		scripts = append(scripts, gradingScript(r))
	}
	f := &gradingFixture{pool: newAPITestPool(t), enq: &fakeEnqueuer{}, prov: gateway.NewSequenceStubProvider(scripts...)}
	f.a = New(Deps{
		Queries: sqlc.New(f.pool), Pool: f.pool, Provider: f.prov,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID, River: f.enq,
	})
	f.h = f.a.Handler()
	if _, err := f.pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatal(err)
	}
	f.teacher = signInAs(t, f.pool, createTeacher(t, f.pool, SeedSchoolID, "gr-teacher@demo.local"))
	f.classID = createClassViaAPI(t, f.h, f.teacher, "Grading Class")
	f.studentID = createStudent(t, f.pool, SeedSchoolID, "gr-student@demo.local")
	enrollStudent(t, f.pool, f.studentID, f.classID)
	return f
}

// submit: the fixture student starts a writing homework, writes gradingBody and finishes (v1).
func (f *gradingFixture) submit(t *testing.T) (aid, atomID string, student *http.Cookie) {
	t.Helper()
	aid, atomID, student = startWritingHomework(t, f.h, f.pool, f.teacher, f.classID, f.studentID)
	if code := assignJSON(t, f.h, student, "PUT", "/api/v1/writings/"+atomID+"/draft", map[string]any{"body": gradingBody}, nil); code != http.StatusOK {
		t.Fatalf("draft = %d", code)
	}
	if code := assignJSON(t, f.h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	return aid, atomID, student
}

func (f *gradingFixture) runJobs(t *testing.T) {
	t.Helper()
	w := &LiteGradingWorker{API: f.a}
	for _, args := range f.enq.drain() {
		if err := w.Work(context.Background(), &river.Job[LiteGradingArgs]{JobRow: &rivertype.JobRow{}, Args: args}); err != nil {
			t.Fatalf("work: %v", err)
		}
	}
}

type gradingRowView struct {
	UserID  string  `json:"userId"`
	AtomID  *string `json:"atomId"`
	Version *struct {
		Number int `json:"number"`
	} `json:"version"`
	Grading *struct {
		ID           string  `json:"id"`
		Status       string  `json:"status"`
		OverallGrade *string `json:"overallGrade"`
		Error        *string `json:"error"`
		ReviewedAt   *string `json:"reviewedAt"`
	} `json:"grading"`
}

type teacherGradingView struct {
	ID                  string          `json:"id"`
	AssignmentID        *string         `json:"assignmentId"`
	VersionNumber       int             `json:"versionNumber"`
	LatestVersionNumber int             `json:"latestVersionNumber"`
	Body                string          `json:"body"`
	Status              string          `json:"status"`
	Content             json.RawMessage `json:"content"`
	Error               *string         `json:"error"`
	ReviewedAt          *string         `json:"reviewedAt"`
	SentAt              *string         `json:"sentAt"`
	StudentSeenAt       *string         `json:"studentSeenAt"`
}

func (f *gradingFixture) rows(t *testing.T, aid string) []gradingRowView {
	t.Helper()
	var resp struct {
		Rows []gradingRowView `json:"rows"`
	}
	if code := getJSON(t, f.h, f.teacher, "/api/v1/lite/teacher/assignments/"+aid+"/gradings", &resp); code != http.StatusOK {
		t.Fatalf("list gradings = %d", code)
	}
	return resp.Rows
}

func (f *gradingFixture) grading(t *testing.T, gid string) teacherGradingView {
	t.Helper()
	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := getJSON(t, f.h, f.teacher, "/api/v1/lite/teacher/gradings/"+gid, &resp); code != http.StatusOK {
		t.Fatalf("get grading = %d", code)
	}
	return resp.Grading
}

func (f *gradingFixture) queueAll(t *testing.T, aid string, retryFailed bool) int {
	t.Helper()
	var resp struct {
		Queued int `json:"queued"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", "/api/v1/lite/teacher/assignments/"+aid+"/gradings", map[string]any{"retryFailed": retryFailed}, &resp); code != http.StatusOK {
		t.Fatalf("queue = %d", code)
	}
	return resp.Queued
}

func countGradingCalls(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM llm_call WHERE purpose = 'teacher_grading'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLiteGradingQueueRunsOnlyEligible(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	aid, _, _ := f.submit(t)
	// A second recipient who never started: no version, nothing to queue.
	other := createStudent(t, f.pool, SeedSchoolID, "gr-other-student@demo.local")
	enrollStudent(t, f.pool, other, f.classID)
	if code := assignJSON(t, f.h, f.teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid, map[string]any{"addUserIds": []string{other.String()}}, nil); code != http.StatusOK {
		t.Fatalf("add recipient = %d", code)
	}

	rows := f.rows(t, aid)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if n := f.queueAll(t, aid, false); n != 1 {
		t.Fatalf("queued = %d, want 1", n)
	}
	if n := f.queueAll(t, aid, false); n != 0 {
		t.Fatalf("second queue = %d, want 0 (the version already has a row)", n)
	}
	var gid string
	for _, r := range f.rows(t, aid) {
		if r.UserID == f.studentID.String() {
			if r.Grading == nil || r.Grading.Status != "queued" || r.Version == nil || r.Version.Number != 1 {
				t.Fatalf("queued row = %+v", r)
			}
			gid = r.Grading.ID
		} else if r.Version != nil || r.Grading != nil {
			t.Fatalf("unstarted row = %+v", r)
		}
	}

	f.runJobs(t)
	g := f.grading(t, gid)
	if g.Status != "draft" || g.Error != nil || g.Body != gradingBody || g.VersionNumber != 1 || g.LatestVersionNumber != 1 || g.AssignmentID == nil || *g.AssignmentID != aid {
		t.Fatalf("draft = %+v", g)
	}
	var content struct {
		Overall struct{ Grade string } `json:"overall"`
		Points  []struct{ Source string } `json:"points"`
	}
	if err := json.Unmarshal(g.Content, &content); err != nil || content.Overall.Grade != "B+" || len(content.Points) != 3 || content.Points[0].Source != "ai" {
		t.Fatalf("content = %s err=%v", g.Content, err)
	}
	var ai, stored []byte
	if err := f.pool.QueryRow(context.Background(), `SELECT ai, content FROM lite_grading WHERE id = $1`, gid).Scan(&ai, &stored); err != nil || string(ai) != string(stored) {
		t.Fatalf("ai must equal content after a run: ai=%s content=%s err=%v", ai, stored, err)
	}
	if f.prov.Calls != 1 || countGradingCalls(t, f.pool) != 1 {
		t.Fatalf("provider calls = %d, llm_call rows = %d", f.prov.Calls, countGradingCalls(t, f.pool))
	}
	for _, r := range f.rows(t, aid) {
		if r.Grading != nil && (r.Grading.OverallGrade == nil || *r.Grading.OverallGrade != "B+") {
			t.Fatalf("list overall grade = %+v", r.Grading)
		}
	}
}

func TestLiteGradingRetriesOnceThenFails(t *testing.T) {
	f := newGradingFixture(t, gradingBadReply)
	aid, _, _ := f.submit(t)
	f.queueAll(t, aid, false)
	f.runJobs(t)
	gid := f.rows(t, aid)[0].Grading.ID
	g := f.grading(t, gid)
	if g.Status != "failed" || g.Error == nil || !strings.Contains(*g.Error, "的引文不在正文中：「去年冬天，我在那里摔过一跤。」") || g.Content != nil && string(g.Content) != "null" {
		t.Fatalf("failed = %+v", g)
	}
	if f.prov.Calls != 2 || countGradingCalls(t, f.pool) != 2 {
		t.Fatalf("provider calls = %d, llm_call rows = %d, want 2 and 2", f.prov.Calls, countGradingCalls(t, f.pool))
	}
	if n := f.queueAll(t, aid, false); n != 0 {
		t.Fatalf("queue without retryFailed = %d", n)
	}
	if n := f.queueAll(t, aid, true); n != 1 {
		t.Fatalf("retryFailed = %d", n)
	}
	if g := f.grading(t, gid); g.Status != "queued" || g.Error != nil {
		t.Fatalf("requeued = %+v", g)
	}
}

func TestLiteGradingSingleWritingAndRegrade(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	_, atomID, student := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"

	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK || resp.Grading.Status != "queued" {
		t.Fatalf("single = %d %+v", code, resp.Grading)
	}
	gid := resp.Grading.ID
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", single, nil); code != http.StatusConflict || ec != "grading_in_progress" {
		t.Fatalf("single while queued = %d %s", code, ec)
	}
	f.runJobs(t)

	// A reviewed draft is regraded: new ai and content, reviewed_at cleared.
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET reviewed_at = now(), content = '{"overall":{"grade":"C","comment":"x"},"dimensions":[],"points":[]}' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	regrade := "/api/v1/lite/teacher/gradings/" + gid + "/regrade"
	if code := assignJSON(t, f.h, f.teacher, "POST", regrade, nil, &resp); code != http.StatusOK || resp.Grading.Status != "queued" {
		t.Fatalf("regrade draft = %d %+v", code, resp.Grading)
	}
	f.runJobs(t)
	if g := f.grading(t, gid); g.Status != "draft" || g.ReviewedAt != nil || !strings.Contains(string(g.Content), `"B+"`) {
		t.Fatalf("after regrade = %+v %s", g, g.Content)
	}

	// A sent version is never regraded.
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'sent', sent_at = now() WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", regrade, nil); code != http.StatusConflict || ec != "grading_sent" {
		t.Fatalf("regrade sent = %d %s", code, ec)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", single, nil); code != http.StatusConflict || ec != "grading_sent" {
		t.Fatalf("single on sent version = %d %s", code, ec)
	}

	// Her v2 is a new version with no row: it can be graded.
	w := "/api/v1/writings/" + atomID
	if code := assignJSON(t, f.h, student, "POST", w+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("revise = %d", code)
	}
	if code := assignJSON(t, f.h, student, "PUT", w+"/draft", map[string]any{"body": gradingBody + "\n\n我打算问问学校谁负责清理。"}, nil); code != http.StatusOK {
		t.Fatalf("draft v2 = %d", code)
	}
	if code := assignJSON(t, f.h, student, "POST", w+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish v2 = %d", code)
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK || resp.Grading.ID == gid || resp.Grading.VersionNumber != 2 {
		t.Fatalf("single on v2 = %d %+v", code, resp.Grading)
	}
	if g := f.grading(t, gid); g.LatestVersionNumber != 2 || g.VersionNumber != 1 {
		t.Fatalf("old row versions = %+v", g)
	}
}

func TestLiteGradingStaleRunningIsFailed(t *testing.T) {
	f := newGradingFixture(t)
	aid, _, _ := f.submit(t)
	f.queueAll(t, aid, false)
	gid := f.rows(t, aid)[0].Grading.ID
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'running', updated_at = now() - interval '20 minutes' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if g := f.grading(t, gid); g.Status != "failed" || g.Error == nil || *g.Error != "批改超时：任务未完成" {
		t.Fatalf("stale running = %+v", g)
	}
}

func TestLiteGradingQueueUnavailableAndEnqueueFailure(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t) // River is nil here
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if code, ec := writeErrorCode(t, h, teacher, "POST", "/api/v1/lite/teacher/assignments/"+aid+"/gradings", nil); code != http.StatusServiceUnavailable || ec != "grading_queue_unavailable" {
		t.Fatalf("no queue = %d %s", code, ec)
	}

	f := newGradingFixture(t)
	f.enq.fail = errors.New("connection refused")
	aid2, _, _ := f.submit(t)
	if n := f.queueAll(t, aid2, false); n != 0 {
		t.Fatalf("queued with a failing queue = %d", n)
	}
	r := f.rows(t, aid2)[0]
	if r.Grading == nil || r.Grading.Status != "failed" || r.Grading.Error == nil || *r.Grading.Error != "入队失败：connection refused" {
		t.Fatalf("enqueue failure = %+v", r.Grading)
	}
}

func TestLiteGradingTeacherRoutesAreOwned(t *testing.T) {
	f := newGradingFixture(t)
	aid, atomID, student := f.submit(t)
	f.queueAll(t, aid, false)
	gid := f.rows(t, aid)[0].Grading.ID

	other := signInAs(t, f.pool, createTeacher(t, f.pool, SeedSchoolID, "gr-other-teacher@demo.local"))
	createClassViaAPI(t, f.h, other, "Other Class")
	paths := []struct{ method, path string }{
		{"GET", "/api/v1/lite/teacher/assignments/" + aid + "/gradings"},
		{"POST", "/api/v1/lite/teacher/assignments/" + aid + "/gradings"},
		{"POST", "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"},
		{"GET", "/api/v1/lite/teacher/gradings/" + gid},
		{"POST", "/api/v1/lite/teacher/gradings/" + gid + "/regrade"},
		{"GET", "/api/v1/lite/teacher/gradings/not-a-uuid"},
	}
	for _, p := range paths {
		if code, _ := writeErrorCode(t, f.h, other, p.method, p.path, nil); code != http.StatusNotFound {
			t.Errorf("other teacher %s %s = %d, want 404", p.method, p.path, code)
		}
		if code, _ := writeErrorCode(t, f.h, student, p.method, p.path, nil); code != http.StatusForbidden && code != http.StatusNotFound {
			t.Errorf("student %s %s = %d, want 403 or 404", p.method, p.path, code)
		}
	}

	// A student who left the class: her grading is no longer this teacher's.
	if _, err := f.pool.Exec(context.Background(), `DELETE FROM enrollments WHERE user_id = $1`, f.studentID); err != nil {
		t.Fatal(err)
	}
	if code, _ := writeErrorCode(t, f.h, f.teacher, "GET", "/api/v1/lite/teacher/gradings/"+gid, nil); code != http.StatusNotFound {
		t.Fatalf("left the class = %d, want 404", code)
	}
}

func TestLiteGradingRejectsReadingHomework(t *testing.T) {
	f := newGradingFixture(t)
	reading := createAssignment(t, f.h, f.teacher, f.classID, readingAssignmentBody("读", map[string]any{"source": "text", "text": "一段正文。"}, []string{f.studentID.String()}))
	for _, method := range []string{"GET", "POST"} {
		if code, ec := writeErrorCode(t, f.h, f.teacher, method, "/api/v1/lite/teacher/assignments/"+reading+"/gradings", nil); code != http.StatusBadRequest || ec != "not_writing_assignment" {
			t.Fatalf("%s reading homework = %d %s", method, code, ec)
		}
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestLiteGrading' -count=1 -timeout 1800s`
Expected: FAIL; the list route answers 404 (not registered).

- [ ] **Step 3: Write `lite_teacher_gradings.go` (read, queue, regrade)**

Create `apps/api/internal/api/lite_teacher_gradings.go`:

```go
package api

// lite_teacher_gradings.go — the teacher side of 一键AI批改: list a homework's
// gradings, queue them, regrade one, read one, edit and send (Task 6).
// No route here calls a model; the worker in lite_grading_jobs.go does.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/store/sqlc"
)

func errGradingQueueUnavailable() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusServiceUnavailable, Code: "grading_queue_unavailable", Message: "批改队列未启动"}
}

func errGradingSent() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "grading_sent", Message: "这一版的批改已发送，不能重新批改"}
}

func errGradingInProgress() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "grading_in_progress", Message: "批改中"}
}

func errNotWritingForGrading() *httpx.APIError {
	return httpx.ErrBadRequest("not_writing_assignment", "只有写作作业可以批改", nil)
}

type gradingSummaryDTO struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	OverallGrade *string `json:"overallGrade"`
	Error        *string `json:"error"`
	ReviewedAt   *string `json:"reviewedAt"`
	SentAt       *string `json:"sentAt"`
}

type gradingVersionDTO struct {
	Number      int32  `json:"number"`
	SubmittedAt string `json:"submittedAt"`
}

type gradingRowDTO struct {
	UserID      string             `json:"userId"`
	DisplayName string             `json:"displayName"`
	AtomID      *string            `json:"atomId"`
	Version     *gradingVersionDTO `json:"version"`
	Grading     *gradingSummaryDTO `json:"grading"`
}

type teacherGradingDTO struct {
	ID                  string          `json:"id"`
	ClassID             string          `json:"classId"`
	AssignmentID        *string         `json:"assignmentId"`
	UserID              string          `json:"userId"`
	DisplayName         string          `json:"displayName"`
	AtomID              string          `json:"atomId"`
	VersionNumber       int32           `json:"versionNumber"`
	LatestVersionNumber int32           `json:"latestVersionNumber"`
	Title               string          `json:"title"`
	Body                string          `json:"body"`
	Lang                string          `json:"lang"`
	Rubric              json.RawMessage `json:"rubric"`
	Status              string          `json:"status"`
	Content             json.RawMessage `json:"content"`
	Error               *string         `json:"error"`
	ReviewedAt          *string         `json:"reviewedAt"`
	SentAt              *string         `json:"sentAt"`
	StudentSeenAt       *string         `json:"studentSeenAt"`
	UpdatedAt           string          `json:"updatedAt"`
}

func gradingOverallGrade(content []byte) *string {
	if len(content) == 0 {
		return nil
	}
	var c struct {
		Overall struct {
			Grade string `json:"grade"`
		} `json:"overall"`
	}
	if json.Unmarshal(content, &c) != nil || c.Overall.Grade == "" {
		return nil
	}
	return &c.Overall.Grade
}

func gradingSummaryOf(g sqlc.LiteGrading) gradingSummaryDTO {
	return gradingSummaryDTO{
		ID: g.ID.String(), Status: g.Status, OverallGrade: gradingOverallGrade(g.Content), Error: g.Error,
		ReviewedAt: tsStringPtr(g.ReviewedAt), SentAt: tsStringPtr(g.SentAt),
	}
}

// markStaleGradings turns abandoned running rows into failed ones before a
// read. It reports whether any row it was given was running, so the caller
// knows to read again.
func (a *API) markStaleGradings(ctx context.Context, rows []sqlc.LiteGrading) (bool, error) {
	var running []uuid.UUID
	for _, g := range rows {
		if g.Status == "running" {
			running = append(running, g.ID)
		}
	}
	if len(running) == 0 {
		return false, nil
	}
	return true, a.d.Queries.MarkStaleLiteGradingsFailed(ctx, running)
}

// loadTeacherGrading loads {gid}: the caller teaches the grading's class and
// the student is still a student in it. Anything else is 404.
func (a *API) loadTeacherGrading(w http.ResponseWriter, r *http.Request) (sqlc.LiteGrading, bool) {
	ctx := r.Context()
	gid, err := uuid.Parse(r.PathValue("gid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.LiteGrading{}, false
	}
	g, err := a.d.Queries.GetLiteGrading(ctx, gid)
	if err != nil {
		writeNotFoundOr(w, r, err)
		return sqlc.LiteGrading{}, false
	}
	if _, err := a.assertTeacherOwnsClass(ctx, g.ClassID); err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.LiteGrading{}, false
	}
	enrolled, err := a.d.Queries.IsEnrolledStudent(ctx, sqlc.IsEnrolledStudentParams{ClassID: g.ClassID, UserID: g.UserID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.LiteGrading{}, false
	}
	if !enrolled {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.LiteGrading{}, false
	}
	return g, true
}

// gradingRubricFor is the rubric snapshot for a writing: its homework's
// rubric when it is homework, else the default for the writing's language.
func (a *API) gradingRubricFor(ctx context.Context, atomID, ownerID uuid.UUID) ([]byte, pgtype.UUID, error) {
	row, err := a.d.Queries.GetLiteAssignmentForAtom(ctx, sqlc.GetLiteAssignmentForAtomParams{
		AtomID: pgtype.UUID{Bytes: atomID, Valid: true}, UserID: ownerID,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, pgtype.UUID{}, err
	}
	if err == nil && row.Kind == "writing" {
		as, err := a.d.Queries.GetLiteAssignment(ctx, row.ID)
		if err != nil {
			return nil, pgtype.UUID{}, err
		}
		b, err := json.Marshal(liteassign.EffectiveRubric(as.Payload))
		return b, pgtype.UUID{Bytes: as.ID, Valid: true}, err
	}
	wr, err := a.d.Queries.GetWriting(ctx, atomID)
	if err != nil {
		return nil, pgtype.UUID{}, err
	}
	b, err := json.Marshal(liteassign.DefaultRubric(wr.Lang))
	return b, pgtype.UUID{}, err
}

// teacherGradingResponse writes {"grading": …} for one row, with the version
// text it grades and the student's latest version number.
func (a *API) teacherGradingResponse(w http.ResponseWriter, r *http.Request, g sqlc.LiteGrading) {
	ctx := r.Context()
	src, err := a.d.Queries.GetLiteGradingSource(ctx, g.VersionID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	latest, err := a.d.Queries.GetLatestWritingVersion(ctx, g.AtomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	student, err := a.d.Queries.GetUserByID(ctx, g.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"grading": teacherGradingDTO{
		ID: g.ID.String(), ClassID: g.ClassID.String(), AssignmentID: uuidStringPtr(g.AssignmentID),
		UserID: g.UserID.String(), DisplayName: student.DisplayName, AtomID: g.AtomID.String(),
		VersionNumber: src.Number, LatestVersionNumber: latest.Number,
		Title: src.Title, Body: src.Body, Lang: src.Lang,
		Rubric: json.RawMessage(g.Rubric), Status: g.Status, Content: json.RawMessage(g.Content), Error: g.Error,
		ReviewedAt: tsStringPtr(g.ReviewedAt), SentAt: tsStringPtr(g.SentAt), StudentSeenAt: tsStringPtr(g.StudentSeenAt),
		UpdatedAt: g.UpdatedAt.Format(time.RFC3339),
	}})
}

// assignmentGradingState reads a writing homework's recipients, each one's
// latest version, and the grading row of that version (stale rows marked first).
func (a *API) assignmentGradingState(ctx context.Context, as sqlc.LiteAssignment) ([]sqlc.ListLiteAssignmentRecipientsRow, map[uuid.UUID]sqlc.ListLatestWritingVersionsForAtomsRow, map[uuid.UUID]sqlc.LiteGrading, error) {
	recipients, err := a.d.Queries.ListLiteAssignmentRecipients(ctx, []uuid.UUID{as.ID})
	if err != nil {
		return nil, nil, nil, err
	}
	atomIDs := make([]uuid.UUID, 0, len(recipients))
	for _, rc := range recipients {
		if rc.AtomID.Valid {
			atomIDs = append(atomIDs, uuid.UUID(rc.AtomID.Bytes))
		}
	}
	versions := map[uuid.UUID]sqlc.ListLatestWritingVersionsForAtomsRow{}
	gradings := map[uuid.UUID]sqlc.LiteGrading{}
	if len(atomIDs) == 0 {
		return recipients, versions, gradings, nil
	}
	vrows, err := a.d.Queries.ListLatestWritingVersionsForAtoms(ctx, atomIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	versionIDs := make([]uuid.UUID, 0, len(vrows))
	for _, v := range vrows {
		versions[v.AtomID] = v
		versionIDs = append(versionIDs, v.ID)
	}
	if len(versionIDs) == 0 {
		return recipients, versions, gradings, nil
	}
	grows, err := a.d.Queries.ListLiteGradingsForVersions(ctx, versionIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	if again, err := a.markStaleGradings(ctx, grows); err != nil {
		return nil, nil, nil, err
	} else if again {
		if grows, err = a.d.Queries.ListLiteGradingsForVersions(ctx, versionIDs); err != nil {
			return nil, nil, nil, err
		}
	}
	for _, g := range grows {
		gradings[g.VersionID] = g
	}
	return recipients, versions, gradings, nil
}

// listLiteAssignmentGradings handles GET /api/v1/lite/teacher/assignments/{aid}/gradings.
func (a *API) listLiteAssignmentGradings(w http.ResponseWriter, r *http.Request) {
	as, ok := a.loadTeacherAssignment(w, r)
	if !ok {
		return
	}
	if as.Kind != "writing" {
		httpx.WriteError(w, r, errNotWritingForGrading())
		return
	}
	recipients, versions, gradings, err := a.assignmentGradingState(r.Context(), as)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows := make([]gradingRowDTO, 0, len(recipients))
	for _, rc := range recipients {
		row := gradingRowDTO{UserID: rc.UserID.String(), DisplayName: rc.DisplayName, AtomID: uuidStringPtr(rc.AtomID)}
		if rc.AtomID.Valid {
			if v, ok := versions[uuid.UUID(rc.AtomID.Bytes)]; ok {
				row.Version = &gradingVersionDTO{Number: v.Number, SubmittedAt: v.SubmittedAt.Format(time.RFC3339)}
				if g, ok := gradings[v.ID]; ok {
					s := gradingSummaryOf(g)
					row.Grading = &s
				}
			}
		}
		rows = append(rows, row)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

// queueLiteAssignmentGradings handles POST /api/v1/lite/teacher/assignments/{aid}/gradings
// (一键AI批改). It queues each recipient whose latest version has no grading
// row; with retryFailed it also requeues that version's failed row. Drafts and
// sent rows are never touched here.
func (a *API) queueLiteAssignmentGradings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	as, ok := a.loadTeacherAssignment(w, r)
	if !ok {
		return
	}
	if as.Kind != "writing" {
		httpx.WriteError(w, r, errNotWritingForGrading())
		return
	}
	var req struct {
		RetryFailed bool `json:"retryFailed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	if a.d.River == nil {
		httpx.WriteError(w, r, errGradingQueueUnavailable())
		return
	}
	u, _ := UserFromContext(ctx)
	rubric, err := json.Marshal(liteassign.EffectiveRubric(as.Payload))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	recipients, versions, gradings, err := a.assignmentGradingState(ctx, as)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	queued := 0
	for _, rc := range recipients {
		if !rc.AtomID.Valid {
			continue
		}
		v, ok := versions[uuid.UUID(rc.AtomID.Bytes)]
		if !ok {
			continue
		}
		var id uuid.UUID
		if existing, has := gradings[v.ID]; has {
			if !req.RetryFailed || existing.Status != "failed" {
				continue
			}
			g, err := a.d.Queries.RequeueLiteGrading(ctx, sqlc.RequeueLiteGradingParams{ID: existing.ID, Rubric: rubric, RequestedBy: u.ID})
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			id = g.ID
		} else {
			g, err := a.d.Queries.CreateLiteGrading(ctx, sqlc.CreateLiteGradingParams{
				AtomID: v.AtomID, VersionID: v.ID, UserID: rc.UserID, ClassID: as.ClassID,
				AssignmentID: pgtype.UUID{Bytes: as.ID, Valid: true}, Rubric: rubric, RequestedBy: u.ID,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				continue // a concurrent request created it
			}
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			id = g.ID
		}
		if a.enqueueLiteGrading(ctx, id) {
			queued++
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"queued": queued})
}

// requeueOrRefuse applies the regrade rules to an existing row: sent is never
// regraded, queued/running is already in progress, draft/failed goes back to the queue.
func (a *API) requeueOrRefuse(ctx context.Context, g sqlc.LiteGrading, requestedBy uuid.UUID) (sqlc.LiteGrading, error) {
	switch g.Status {
	case "sent":
		return sqlc.LiteGrading{}, errGradingSent()
	case "queued", "running":
		return sqlc.LiteGrading{}, errGradingInProgress()
	}
	rubric, _, err := a.gradingRubricFor(ctx, g.AtomID, g.UserID)
	if err != nil {
		return sqlc.LiteGrading{}, err
	}
	rq, err := a.d.Queries.RequeueLiteGrading(ctx, sqlc.RequeueLiteGradingParams{ID: g.ID, Rubric: rubric, RequestedBy: requestedBy})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.LiteGrading{}, errGradingInProgress()
	}
	return rq, err
}

// queueLiteWritingGrading handles
// POST /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}/gradings:
// grade one writing's latest version (create the row, or requeue a draft/failed one).
func (a *API) queueLiteWritingGrading(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	at, ok := a.loadTeacherOwnedAtom(w, r, userID)
	if !ok {
		return
	}
	if at.Kind != "writing" {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if a.d.River == nil {
		httpx.WriteError(w, r, errGradingQueueUnavailable())
		return
	}
	u, _ := UserFromContext(ctx)
	latest, err := a.d.Queries.GetLatestWritingVersion(ctx, at.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, &httpx.APIError{Status: http.StatusConflict, Code: "no_submission", Message: "这名学生还没有提交"})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rubric, assignmentID, err := a.gradingRubricFor(ctx, at.ID, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	g, err := a.d.Queries.CreateLiteGrading(ctx, sqlc.CreateLiteGradingParams{
		AtomID: at.ID, VersionID: latest.ID, UserID: userID, ClassID: classID,
		AssignmentID: assignmentID, Rubric: rubric, RequestedBy: u.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, gerr := a.d.Queries.GetLiteGradingByVersion(ctx, latest.ID)
		if gerr != nil {
			httpx.WriteError(w, r, gerr)
			return
		}
		g, err = a.requeueOrRefuse(ctx, existing, u.ID)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.enqueueLiteGrading(ctx, g.ID)
	if g, err = a.d.Queries.GetLiteGrading(ctx, g.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.teacherGradingResponse(w, r, g)
}

// regradeLiteGrading handles POST /api/v1/lite/teacher/gradings/{gid}/regrade (重新批改).
func (a *API) regradeLiteGrading(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	g, ok := a.loadTeacherGrading(w, r)
	if !ok {
		return
	}
	if a.d.River == nil {
		httpx.WriteError(w, r, errGradingQueueUnavailable())
		return
	}
	u, _ := UserFromContext(ctx)
	rq, err := a.requeueOrRefuse(ctx, g, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.enqueueLiteGrading(ctx, rq.ID)
	if rq, err = a.d.Queries.GetLiteGrading(ctx, rq.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.teacherGradingResponse(w, r, rq)
}

// getLiteGrading handles GET /api/v1/lite/teacher/gradings/{gid}.
func (a *API) getLiteGrading(w http.ResponseWriter, r *http.Request) {
	g, ok := a.loadTeacherGrading(w, r)
	if !ok {
		return
	}
	if again, err := a.markStaleGradings(r.Context(), []sqlc.LiteGrading{g}); err != nil {
		httpx.WriteError(w, r, err)
		return
	} else if again {
		if g, err = a.d.Queries.GetLiteGrading(r.Context(), g.ID); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	a.teacherGradingResponse(w, r, g)
}
```

Note `GetLatestWritingVersion` returns `sqlc.WritingVersion` with `ID` (Part A query uses `SELECT *`).

- [ ] **Step 4: Register the routes**

In `apps/api/internal/api/lite_teacher_routes.go`, after the `…/return` route:

```go
	// AI 批改. No route calls a model: the queue routes insert river jobs and
	// the worker makes the calls.
	mux.Handle("GET /api/v1/lite/teacher/assignments/{aid}/gradings", liteTeacher(a.listLiteAssignmentGradings))
	mux.Handle("POST /api/v1/lite/teacher/assignments/{aid}/gradings", liteTeacher(a.queueLiteAssignmentGradings))
	mux.Handle("POST /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}/gradings", liteTeacher(a.queueLiteWritingGrading))
	mux.Handle("GET /api/v1/lite/teacher/gradings/{gid}", liteTeacher(a.getLiteGrading))
	mux.Handle("POST /api/v1/lite/teacher/gradings/{gid}/regrade", liteTeacher(a.regradeLiteGrading))
```

- [ ] **Step 5: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestLiteGrading' -count=1 -timeout 1800s`
Expected: PASS. If `TestLiteGradingQueueRunsOnlyEligible` sees `provider calls = 0`, check that the fixture sets `EvalResolver` (the `review` class falls back to it when `Deps.Route` is nil).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/lite_teacher_gradings.go apps/api/internal/api/lite_teacher_routes.go \
  apps/api/internal/api/lite_grading_test.go
git commit -m "feat(lite): teacher queues, regrades and reads AI gradings

A sent version is never regraded; a draft regrade replaces ai and content
and clears the review.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Teacher edits and sends gradings

**Files:**
- Modify: `apps/api/internal/api/lite_teacher_gradings.go` (append)
- Modify: `apps/api/internal/api/lite_teacher_routes.go`
- Test: `apps/api/internal/api/lite_grading_test.go` (append)

**Interfaces:**
- Consumes: `loadTeacherGrading`, `teacherGradingResponse` (Task 5); `litegrade.NormalizeTeacher`, `litegrade.CheckTeacherEdit`, `litegrade.JoinReasons` (Task 2); `UpdateLiteGradingContent`, `SendLiteGrading`, `SendReviewedLiteGradings` (Task 3).
- Produces (HTTP):
  - `PATCH /api/v1/lite/teacher/gradings/{gid}` body `{content?: Content}` → `{"grading": TeacherGrading}`; sets `reviewedAt`; on a `sent` row also re-sends. 400 `invalid_grading` 「批改内容格式错误」 or 「批改内容有误：{reasons}」; 409 `grading_not_editable` 「批改中或批改失败时不能修改」.
  - `POST /api/v1/lite/teacher/gradings/{gid}/send` → `{"grading": TeacherGrading}`; 409 `grading_not_sendable` 「只有草稿或已发送的批改可以发送」.
  - `POST /api/v1/lite/teacher/assignments/{aid}/gradings/send` body `{ids: string[]}` → `{"sent": n}`; sends only reviewed drafts of this homework. 400 `invalid_ids` 「批改编号格式错误」.

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/api/lite_grading_test.go`:

```go
func gradingContent(grade string, points []map[string]any) map[string]any {
	return map[string]any{
		"overall": map[string]any{"grade": grade, "comment": "第二段请补充数据来源。"},
		"dimensions": []map[string]any{
			{"name": "内容", "grade": "B", "comment": ""}, {"name": "结构", "grade": "B", "comment": ""},
			{"name": "语言", "grade": "B", "comment": ""}, {"name": "书写规范", "grade": "B", "comment": ""},
		},
		"points": points,
	}
}

func TestLiteGradingPatchAndSend(t *testing.T) {
	f := newGradingFixture(t)
	aid, _, _ := f.submit(t)
	f.queueAll(t, aid, false)
	gid := f.rows(t, aid)[0].Grading.ID
	path := "/api/v1/lite/teacher/gradings/" + gid

	// Queued: not editable, not sendable.
	if code, ec := writeErrorCode(t, f.h, f.teacher, "PATCH", path, map[string]any{}); code != http.StatusConflict || ec != "grading_not_editable" {
		t.Fatalf("patch queued = %d %s", code, ec)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", path+"/send", nil); code != http.StatusConflict || ec != "grading_not_sendable" {
		t.Fatalf("send queued = %d %s", code, ec)
	}
	f.runJobs(t)

	teacherPoint := []map[string]any{{"kind": "issue", "quote": nil, "text": "第二段请补充数据来源。", "action": nil, "source": "teacher"}}
	rec := doJSON(t, f.h, f.teacher, "PATCH", path, mustJSON(t, map[string]any{"content": gradingContent("E", teacherPoint)}))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_grading") || !strings.Contains(rec.Body.String(), "总评的等级不在评分标准内：E") {
		t.Fatalf("bad grade = %d %s", rec.Code, rec.Body)
	}
	notHers := []map[string]any{{"kind": "issue", "quote": "雨一直下。", "text": "说明", "action": nil, "source": "teacher"}}
	rec = doJSON(t, f.h, f.teacher, "PATCH", path, mustJSON(t, map[string]any{"content": gradingContent("B", notHers)}))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "引文不在正文中") {
		t.Fatalf("teacher quote not hers = %d %s", rec.Code, rec.Body)
	}

	var resp struct {
		Grading teacherGradingView `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "PATCH", path, map[string]any{"content": gradingContent("B", teacherPoint)}, &resp); code != http.StatusOK {
		t.Fatalf("patch = %d", code)
	}
	if resp.Grading.ReviewedAt == nil || resp.Grading.Status != "draft" || !strings.Contains(string(resp.Grading.Content), `"source":"teacher"`) {
		t.Fatalf("patched = %+v %s", resp.Grading, resp.Grading.Content)
	}
	var ai []byte
	if err := f.pool.QueryRow(context.Background(), `SELECT ai FROM lite_grading WHERE id = $1`, gid).Scan(&ai); err != nil || !strings.Contains(string(ai), `"B+"`) {
		t.Fatalf("ai must not change on a teacher edit: %s err=%v", ai, err)
	}

	if code := assignJSON(t, f.h, f.teacher, "POST", path+"/send", nil, &resp); code != http.StatusOK || resp.Grading.Status != "sent" || resp.Grading.SentAt == nil {
		t.Fatalf("send = %d %+v", code, resp.Grading)
	}
	// Regrading a sent row is refused (Task 5); editing it is allowed and re-sends.
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET student_seen_at = now() WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if code := assignJSON(t, f.h, f.teacher, "PATCH", path, map[string]any{"content": gradingContent("A-", teacherPoint)}, &resp); code != http.StatusOK || resp.Grading.Status != "sent" || resp.Grading.StudentSeenAt != nil {
		t.Fatalf("edit sent = %d %+v", code, resp.Grading)
	}

	// Failed rows are not editable.
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'failed' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "PATCH", path, map[string]any{}); code != http.StatusConflict || ec != "grading_not_editable" {
		t.Fatalf("patch failed row = %d %s", code, ec)
	}
}

func TestLiteGradingSendAllReviewed(t *testing.T) {
	f := newGradingFixture(t)
	s2 := createStudent(t, f.pool, SeedSchoolID, "gr-s2@demo.local")
	s3 := createStudent(t, f.pool, SeedSchoolID, "gr-s3@demo.local")
	enrollStudent(t, f.pool, s2, f.classID)
	enrollStudent(t, f.pool, s3, f.classID)
	ids := []uuid.UUID{f.studentID, s2, s3}
	idStrings := []string{f.studentID.String(), s2.String(), s3.String()}
	aid := createAssignment(t, f.h, f.teacher, f.classID, writingAssignmentBody(idStrings))
	for _, uid := range ids {
		c := signInAs(t, f.pool, uid)
		atomID := startAssignment(t, f.h, c, aid).AtomID
		if code := assignJSON(t, f.h, c, "PUT", "/api/v1/writings/"+atomID+"/draft", map[string]any{"body": gradingBody}, nil); code != http.StatusOK {
			t.Fatalf("draft = %d", code)
		}
		if code := assignJSON(t, f.h, c, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
			t.Fatalf("finish = %d", code)
		}
	}
	if n := f.queueAll(t, aid, false); n != 3 {
		t.Fatalf("queued = %d", n)
	}
	f.runJobs(t)
	rows := f.rows(t, aid)
	gids := make([]string, 0, 3)
	for _, r := range rows {
		gids = append(gids, r.Grading.ID)
	}
	// Mark one reviewed (PATCH with no content).
	if code := assignJSON(t, f.h, f.teacher, "PATCH", "/api/v1/lite/teacher/gradings/"+gids[0], map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("mark reviewed = %d", code)
	}
	var resp struct {
		Sent int `json:"sent"`
	}
	sendAll := "/api/v1/lite/teacher/assignments/" + aid + "/gradings/send"
	if code := assignJSON(t, f.h, f.teacher, "POST", sendAll, map[string]any{"ids": gids}, &resp); code != http.StatusOK || resp.Sent != 1 {
		t.Fatalf("send all = %d sent=%d, want 1", code, resp.Sent)
	}
	statuses := map[string]string{}
	for _, r := range f.rows(t, aid) {
		statuses[r.Grading.ID] = r.Grading.Status
	}
	if statuses[gids[0]] != "sent" || statuses[gids[1]] != "draft" || statuses[gids[2]] != "draft" {
		t.Fatalf("statuses = %v", statuses)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", sendAll, map[string]any{"ids": []string{"x"}}); code != http.StatusBadRequest || ec != "invalid_ids" {
		t.Fatalf("bad ids = %d %s", code, ec)
	}

	other := signInAs(t, f.pool, createTeacher(t, f.pool, SeedSchoolID, "gr-other2@demo.local"))
	for _, p := range []struct{ method, path string }{
		{"PATCH", "/api/v1/lite/teacher/gradings/" + gids[1]},
		{"POST", "/api/v1/lite/teacher/gradings/" + gids[1] + "/send"},
		{"POST", sendAll},
	} {
		if code, _ := writeErrorCode(t, f.h, other, p.method, p.path, map[string]any{"ids": gids}); code != http.StatusNotFound {
			t.Errorf("other teacher %s %s = %d, want 404", p.method, p.path, code)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
```

(`doJSON(t, h, cookie, method, path, body string) *httptest.ResponseRecorder` is the existing helper in `workspace_library_test.go`.)

- [ ] **Step 2: Run to see them fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestLiteGradingPatchAndSend|TestLiteGradingSendAllReviewed' -count=1 -timeout 1800s`
Expected: FAIL; PATCH answers 405 (route not registered).

- [ ] **Step 3: Append the handlers**

Append to `apps/api/internal/api/lite_teacher_gradings.go` (add `"strings"`, `"mindimprint/api/internal/litegrade"` to the imports):

```go
// patchLiteGrading handles PATCH /api/v1/lite/teacher/gradings/{gid}.
// With content: a shape check (litegrade.CheckTeacherEdit), then save. Without
// content: 标记已审阅. Either way reviewed_at is set. Editing a sent row
// re-sends it. ai is never changed.
func (a *API) patchLiteGrading(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	g, ok := a.loadTeacherGrading(w, r)
	if !ok {
		return
	}
	var req struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	var content []byte
	if req.Content != nil && strings.TrimSpace(string(req.Content)) != "null" {
		var c litegrade.Content
		if err := json.Unmarshal(req.Content, &c); err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_grading", "批改内容格式错误", nil))
			return
		}
		var rubric liteassign.Rubric
		if err := json.Unmarshal(g.Rubric, &rubric); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		src, err := a.d.Queries.GetLiteGradingSource(ctx, g.VersionID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		in := liteGradingInput(src, rubric)
		c = litegrade.NormalizeTeacher(c, rubric)
		if rs := litegrade.CheckTeacherEdit(c, in); len(rs) > 0 {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_grading", "批改内容有误："+litegrade.JoinReasons(rs), nil))
			return
		}
		if content, err = json.Marshal(c); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	updated, err := a.d.Queries.UpdateLiteGradingContent(ctx, sqlc.UpdateLiteGradingContentParams{ID: g.ID, Content: content})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, &httpx.APIError{Status: http.StatusConflict, Code: "grading_not_editable", Message: "批改中或批改失败时不能修改"})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.teacherGradingResponse(w, r, updated)
}

func errGradingNotSendable() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusConflict, Code: "grading_not_sendable", Message: "只有草稿或已发送的批改可以发送"}
}

// sendLiteGrading handles POST /api/v1/lite/teacher/gradings/{gid}/send.
func (a *API) sendLiteGrading(w http.ResponseWriter, r *http.Request) {
	g, ok := a.loadTeacherGrading(w, r)
	if !ok {
		return
	}
	sent, err := a.d.Queries.SendLiteGrading(r.Context(), g.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, errGradingNotSendable())
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.teacherGradingResponse(w, r, sent)
}

// sendLiteAssignmentGradings handles POST /api/v1/lite/teacher/assignments/{aid}/gradings/send
// (发送全部已审阅). Rows of other homework, and drafts not yet reviewed, are skipped.
func (a *API) sendLiteAssignmentGradings(w http.ResponseWriter, r *http.Request) {
	as, ok := a.loadTeacherAssignment(w, r)
	if !ok {
		return
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, s := range req.IDs {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_ids", "批改编号格式错误", nil))
			return
		}
		ids = append(ids, id)
	}
	n, err := a.d.Queries.SendReviewedLiteGradings(r.Context(), sqlc.SendReviewedLiteGradingsParams{
		AssignmentID: pgtype.UUID{Bytes: as.ID, Valid: true}, Ids: ids,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"sent": n})
}
```

- [ ] **Step 4: Register the routes**

In `apps/api/internal/api/lite_teacher_routes.go`, under the grading routes from Task 5:

```go
	mux.Handle("POST /api/v1/lite/teacher/assignments/{aid}/gradings/send", liteTeacher(a.sendLiteAssignmentGradings))
	mux.Handle("PATCH /api/v1/lite/teacher/gradings/{gid}", liteTeacher(a.patchLiteGrading))
	mux.Handle("POST /api/v1/lite/teacher/gradings/{gid}/send", liteTeacher(a.sendLiteGrading))
```

- [ ] **Step 5: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestLiteGrading' -count=1 -timeout 1800s`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/lite_teacher_gradings.go apps/api/internal/api/lite_teacher_routes.go \
  apps/api/internal/api/lite_grading_test.go
git commit -m "feat(lite): teacher edits, reviews and sends AI gradings

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Student reads sent gradings; inbox; teacher item payload

**Files:**
- Create: `apps/api/internal/api/lite_student_gradings.go`
- Modify: `apps/api/internal/api/lite_student_assignments.go` (`getLiteInbox`)
- Modify: `apps/api/internal/api/lite_teacher_item.go` (`liteTeacherWriting`)
- Modify: `apps/api/internal/api/api.go` (routes, next to the writing version routes and `/lite/inbox`)
- Test: `apps/api/internal/api/lite_grading_student_test.go`

**Interfaces:**
- Consumes: `ListSentLiteGradingsForAtom`, `ListLiteInboxGradings`, `MarkLiteGradingSeen`, `GetLiteGradingByVersion` (Task 3); `gradingSummaryOf` (Task 5); the grading fixture from Task 5.
- Produces (HTTP):
  - `GET /api/v1/writings/{id}/gradings` → `{"gradings": [{id, versionNumber, rubric, content, sentAt, seen}]}` — owner only, sent rows only, newest version first.
  - `POST /api/v1/lite/inbox/gradings/{gid}/seen` → 204; 404 unless it is her sent grading.
  - `GET /api/v1/lite/inbox` items gain `{type: "grading", id, atomId, writingTitle, sentAt, unread}` after the assignment items; `unread` counts unseen gradings too.
  - Teacher item payload `writing` gains `"grading": GradingSummary | null` (the latest version's row).

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/api/lite_grading_student_test.go`:

```go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func studentGradings(t *testing.T, f *gradingFixture, student *http.Cookie, atomID string) []map[string]any {
	t.Helper()
	var resp struct {
		Gradings []map[string]any `json:"gradings"`
	}
	if code := getJSON(t, f.h, student, "/api/v1/writings/"+atomID+"/gradings", &resp); code != http.StatusOK {
		t.Fatalf("student gradings = %d", code)
	}
	return resp.Gradings
}

type inboxView struct {
	Items []struct {
		Type         string `json:"type"`
		ID           string `json:"id"`
		AtomID       string `json:"atomId"`
		WritingTitle string `json:"writingTitle"`
		SentAt       string `json:"sentAt"`
		Unread       bool   `json:"unread"`
	} `json:"items"`
	Unread int `json:"unread"`
}

// Students never see a grading that is not sent: not queued, not a draft,
// not failed, and never the ai copy or the error.
func TestStudentSeesOnlySentGradings(t *testing.T) {
	f := newGradingFixture(t)
	aid, atomID, student := f.submit(t)
	f.queueAll(t, aid, false)
	gid := f.rows(t, aid)[0].Grading.ID
	if got := studentGradings(t, f, student, atomID); len(got) != 0 {
		t.Fatalf("queued visible: %v", got)
	}
	f.runJobs(t)
	if got := studentGradings(t, f, student, atomID); len(got) != 0 {
		t.Fatalf("draft visible: %v", got)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'failed', error = 'x' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if got := studentGradings(t, f, student, atomID); len(got) != 0 {
		t.Fatalf("failed visible: %v", got)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'draft', error = NULL WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	var inbox inboxView
	getJSON(t, f.h, student, "/api/v1/lite/inbox", &inbox)
	for _, it := range inbox.Items {
		if it.Type == "grading" {
			t.Fatalf("unsent grading in the inbox: %+v", it)
		}
	}

	if code := assignJSON(t, f.h, f.teacher, "POST", "/api/v1/lite/teacher/gradings/"+gid+"/send", nil, nil); code != http.StatusOK {
		t.Fatalf("send = %d", code)
	}
	got := studentGradings(t, f, student, atomID)
	if len(got) != 1 || got[0]["id"] != gid || got[0]["versionNumber"] != float64(1) || got[0]["seen"] != false {
		t.Fatalf("sent = %v", got)
	}
	for _, hidden := range []string{"ai", "status", "error", "requestedBy", "reviewedAt"} {
		if _, ok := got[0][hidden]; ok {
			t.Fatalf("student response leaks %q: %v", hidden, got[0])
		}
	}
	var content struct {
		Overall struct{ Grade string } `json:"overall"`
	}
	raw, _ := json.Marshal(got[0]["content"])
	if json.Unmarshal(raw, &content) != nil || content.Overall.Grade != "B+" || got[0]["rubric"] == nil {
		t.Fatalf("content/rubric = %v", got[0])
	}

	// Inbox: one unread grading item; the assignment was seen when she started it.
	inbox = inboxView{}
	getJSON(t, f.h, student, "/api/v1/lite/inbox", &inbox)
	var grading *struct {
		Type         string `json:"type"`
		ID           string `json:"id"`
		AtomID       string `json:"atomId"`
		WritingTitle string `json:"writingTitle"`
		SentAt       string `json:"sentAt"`
		Unread       bool   `json:"unread"`
	}
	for i := range inbox.Items {
		if inbox.Items[i].Type == "grading" {
			grading = &inbox.Items[i]
		}
	}
	if grading == nil || grading.ID != gid || grading.AtomID != atomID || !grading.Unread || grading.SentAt == "" || grading.WritingTitle == "" || inbox.Unread != 1 {
		t.Fatalf("inbox = %+v", inbox)
	}
	if code := assignJSON(t, f.h, student, "POST", "/api/v1/lite/inbox/gradings/"+gid+"/seen", nil, nil); code != http.StatusNoContent {
		t.Fatalf("seen = %d", code)
	}
	inbox = inboxView{}
	getJSON(t, f.h, student, "/api/v1/lite/inbox", &inbox)
	if inbox.Unread != 0 {
		t.Fatalf("after seen unread = %d", inbox.Unread)
	}

	// An edit re-sends and makes it unread again.
	if code := assignJSON(t, f.h, f.teacher, "PATCH", "/api/v1/lite/teacher/gradings/"+gid, map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("re-save = %d", code)
	}
	inbox = inboxView{}
	getJSON(t, f.h, student, "/api/v1/lite/inbox", &inbox)
	if inbox.Unread != 1 {
		t.Fatalf("after re-send unread = %d, want 1", inbox.Unread)
	}

	// Another student cannot read or mark it.
	stranger := signInAs(t, f.pool, createStudent(t, f.pool, SeedSchoolID, "gr-stranger@demo.local"))
	if code := getJSON(t, f.h, stranger, "/api/v1/writings/"+atomID+"/gradings", nil); code != http.StatusNotFound {
		t.Fatalf("stranger read = %d", code)
	}
	if code, _ := writeErrorCode(t, f.h, stranger, "POST", "/api/v1/lite/inbox/gradings/"+gid+"/seen", nil); code != http.StatusNotFound {
		t.Fatalf("stranger seen = %d", code)
	}

	// The teacher's item page shows the latest version's grading.
	var item struct {
		Writing struct {
			Grading *struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"grading"`
		} `json:"writing"`
	}
	if code := getJSON(t, f.h, f.teacher, "/api/v1/lite/teacher/classes/"+f.classID+"/students/"+f.studentID.String()+"/items/"+atomID, &item); code != http.StatusOK {
		t.Fatalf("item = %d", code)
	}
	if item.Writing.Grading == nil || item.Writing.Grading.ID != gid || item.Writing.Grading.Status != "sent" {
		t.Fatalf("item grading = %+v", item.Writing.Grading)
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestStudentSeesOnlySentGradings -count=1 -timeout 1800s`
Expected: FAIL; `student gradings = 404` (route not registered, so the mux 404s).

- [ ] **Step 3: Write `lite_student_gradings.go`**

Create `apps/api/internal/api/lite_student_gradings.go`:

```go
package api

// lite_student_gradings.go — what a student sees of AI 批改: sent gradings of
// her own writing, and marking one seen from the inbox. Only status = 'sent'
// rows are ever read here (the queries filter it), and the DTO has no field
// for ai, error or status.

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

type studentGradingDTO struct {
	ID            string          `json:"id"`
	VersionNumber int32           `json:"versionNumber"`
	Rubric        json.RawMessage `json:"rubric"`
	Content       json.RawMessage `json:"content"`
	SentAt        string          `json:"sentAt"`
	Seen          bool            `json:"seen"`
}

// InboxGradingDTO is a sent grading in the student's inbox.
type InboxGradingDTO struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	AtomID       string `json:"atomId"`
	WritingTitle string `json:"writingTitle"`
	SentAt       string `json:"sentAt"`
	Unread       bool   `json:"unread"`
}

func inboxGradingItems(rows []sqlc.ListLiteInboxGradingsRow) ([]InboxGradingDTO, int) {
	out := make([]InboxGradingDTO, 0, len(rows))
	unread := 0
	for _, g := range rows {
		if !g.StudentSeenAt.Valid {
			unread++
		}
		out = append(out, InboxGradingDTO{
			Type: "grading", ID: g.ID.String(), AtomID: g.AtomID.String(), WritingTitle: g.Title,
			SentAt: g.SentAt.Time.Format(time.RFC3339), Unread: !g.StudentSeenAt.Valid,
		})
	}
	return out, unread
}

// listWritingGradingsHandler handles GET /api/v1/writings/{id}/gradings.
func (a *API) listWritingGradingsHandler(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListSentLiteGradingsForAtom(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]studentGradingDTO, 0, len(rows))
	for _, g := range rows {
		out = append(out, studentGradingDTO{
			ID: g.ID.String(), VersionNumber: g.VersionNumber,
			Rubric: json.RawMessage(g.Rubric), Content: json.RawMessage(g.Content),
			SentAt: g.SentAt.Time.Format(time.RFC3339), Seen: g.StudentSeenAt.Valid,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"gradings": out})
}

// markLiteGradingSeen handles POST /api/v1/lite/inbox/gradings/{gid}/seen.
func (a *API) markLiteGradingSeen(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	gid, err := uuid.Parse(r.PathValue("gid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	n, err := a.d.Queries.MarkLiteGradingSeen(r.Context(), sqlc.MarkLiteGradingSeenParams{ID: gid, UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if n == 0 {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Add grading items to the inbox**

In `apps/api/internal/api/lite_student_assignments.go`, `getLiteInbox`: change `items := make([]InboxItemDTO, 0, len(rows))` to `items := make([]any, 0, len(rows))` (the `append(items, InboxItemDTO{…})` line stays as it is), and directly before the final `httpx.WriteJSON`:

```go
	// Sent 批改 of her writings follow the assignments, newest first.
	gradingRows, err := a.d.Queries.ListLiteInboxGradings(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	gradingItems, gradingUnread := inboxGradingItems(gradingRows)
	for _, it := range gradingItems {
		items = append(items, it)
	}
	unread += gradingUnread
```

Update the `InboxItemDTO` doc comment's first sentence to: `InboxItemDTO is one assignment in the student's inbox (Type "assignment"); sent gradings are InboxGradingDTO (lite_student_gradings.go).`

- [ ] **Step 5: The teacher item payload**

In `apps/api/internal/api/lite_teacher_item.go`, `liteTeacherWriting`, after `versionRows` is read:

```go
	// The latest version's 批改, for the item page's 批改 button and status.
	var grading *gradingSummaryDTO
	if len(versionRows) > 0 {
		g, err := notFoundIsNil(a.d.Queries.GetLiteGradingByVersion(ctx, versionRows[0].ID))
		if err != nil {
			return nil, err
		}
		if g != nil {
			s := gradingSummaryOf(*g)
			grading = &s
		}
	}
```

and add `"grading": grading,` to the returned map.

- [ ] **Step 6: Routes**

In `apps/api/internal/api/api.go`, after the `GET /api/v1/writings/{id}/versions/{n}` line:

```go
	mux.Handle("GET /api/v1/writings/{id}/gradings", liteOnly(a.listWritingGradingsHandler))
```

and after `GET /api/v1/lite/inbox`:

```go
	mux.Handle("POST /api/v1/lite/inbox/gradings/{gid}/seen", liteOnly(a.markLiteGradingSeen))
```

- [ ] **Step 7: Run the student test and the existing inbox tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestStudentSeesOnlySentGradings|TestInbox|TestReturnedAndResubmittedStatuses|TestLiteItemDetail' -count=1 -timeout 1800s`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/api/lite_student_gradings.go apps/api/internal/api/lite_student_assignments.go \
  apps/api/internal/api/lite_teacher_item.go apps/api/internal/api/api.go \
  apps/api/internal/api/lite_grading_student_test.go
git commit -m "feat(lite): students read sent gradings and see them in the inbox

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: `TestLiveLiteGrading` against the real gateway

**Files:**
- Create: `apps/api/internal/api/lite_grading_live_test.go`

**Interfaces:**
- Consumes: `liveClass(t, class)` (`reading_coach_lensdone_live_test.go`, package `api`), `gradeWithRetry`, `liteGradingInput` (Task 4), `litegrade.Check`, `liteassign.DefaultRubric`.

- [ ] **Step 1: Write the live test**

Create `apps/api/internal/api/lite_grading_live_test.go`:

```go
package api

// lite_grading_live_test.go — AI 批改 against the real review model.
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveLiteGrading -v -count=1
//
// The stub tests only prove Check reads JSON written by hand. This test
// proves the real model, given the real prompt, returns a grading that
// passes Check within the production retry. Run it three times before Part B
// ships and judge the worst run (AGENTS.md: score the worst case).

import (
	"context"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/store/sqlc"
)

const liveGradingZH = `学校食堂每天倒掉的饭特别多。上周五中午，我在回收桶旁边站了二十分钟，数到六个桶都装满了。

光是那一天，倒掉的米饭大概能装满两个洗菜盆。食堂阿姨说，周五的剩饭总是最多，因为很多同学下午放学早，中午随便吃几口就走了。

我觉得学校可以把周五的饭量减少一些。也有同学说，饭少了会有人吃不饱。到底是什么让我们倒掉一整盘饭的时候，连眼皮都不会抬一下？`

const liveGradingEN = `Because of the food waste problem is very serious in our school, I think we should do something.
Every day the canteen throw away many rice. Last Friday I counted six bins are full.

Some students say the portions are too big. Others say the food is not tasty. In my opinion, the school should let students choose their portion size, because this is the simplest way.`

func TestLiveLiteGrading(t *testing.T) {
	cases := []struct {
		name, lang, prompt, body string
	}{
		{"zh", "zh", "写一篇议论文，讨论学校食堂的浪费问题，并提出你的建议。", liveGradingZH},
		{"en", "en", "Write an essay about food waste in your school and suggest one solution.", liveGradingEN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prov, resolved := liveClass(t, gateway.ClassReview)
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			prompt := tc.prompt
			in := liteGradingInput(sqlc.GetLiteGradingSourceRow{
				Number: 1, Title: "食堂浪费", Body: tc.body, Lang: tc.lang, AssignedPrompt: &prompt,
			}, liteassign.DefaultRubric(tc.lang))

			content, reasons, attempts := gradeWithRetry(ctx, prov, resolved, in, func(u gateway.ChatUsage) {
				t.Logf("call: in=%d out=%d", u.InputTokens, u.OutputTokens)
			})
			t.Logf("attempts = %d", attempts)
			if len(reasons) > 0 {
				t.Fatalf("grading failed after %d attempts: %s", attempts, litegrade.JoinReasons(reasons))
			}
			if rs := litegrade.Check(content, in); len(rs) > 0 {
				t.Fatalf("returned content does not pass Check: %s", litegrade.JoinReasons(rs))
			}
			t.Logf("overall %s: %s", content.Overall.Grade, content.Overall.Comment)
			for _, d := range content.Dimensions {
				t.Logf("  %s %s: %s", d.Name, d.Grade, d.Comment)
			}
			for _, p := range content.Points {
				action := ""
				if p.Action != nil {
					action = *p.Action
				}
				t.Logf("  [%s] 「%s」\n    %s\n    %s", p.Kind, *p.Quote, p.Text, action)
			}
		})
	}
}
```

- [ ] **Step 2: Confirm it skips without the environment**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestLiveLiteGrading -count=1 -short -v 2>&1 | grep -E "SKIP|PASS|FAIL|ok"`
Expected: both subtests `SKIP` ("set LIVE_LLM=1 to run live prompt checks"), package `ok`.

- [ ] **Step 3: Commit**

```bash
git add apps/api/internal/api/lite_grading_live_test.go
git commit -m "test(lite): live AI grading check on one zh and one en writing

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

The three live runs happen in Task 15, after the UI is in place, so a prompt change they force is made once.

---

### Task 9: Frontend clients and normalizers

**Files:**
- Create: `apps/lite-web/src/api/gradings.ts`
- Test: `apps/lite-web/src/api/gradings.test.ts`
- Modify: `apps/lite-web/src/api/assignments.ts` (inbox union, rubric on payload and PATCH)
- Modify: `apps/lite-web/src/api/assignments.test.ts` (grading rows kept)
- Modify: `apps/lite-web/src/api/teacher.ts` (`ItemDetail.writing.versions`, `.grading`, `.revising`)
- Modify: `apps/lite-web/src/inbox/inboxLogic.ts` (`openItemsForKind` takes the union)

**Interfaces:**
- Consumes: HTTP shapes from Tasks 5–7; `normalizeVersionSummary` (exported from `api/writings.ts`).
- Produces (`api/gradings.ts`):
  - `LETTER_GRADES`, types `GradingStatus`, `PointKind`, `RubricDimension`, `Rubric`, `GradingPoint`, `GradingContent`, `GradingSummary`, `GradingRow`, `TeacherGrading`, `StudentGrading`
  - `normalizeRubric(raw: unknown): Rubric | null`, `normalizeGradingContent(raw: unknown): GradingContent | null`, `normalizeGradingSummary(raw: unknown): GradingSummary | null`, `normalizeGradingRow`, `normalizeTeacherGrading`, `normalizeStudentGrading(raw): StudentGrading | null`
  - Clients: `listAssignmentGradings(aid)`, `queueAssignmentGradings(aid, retryFailed)`, `sendReviewedGradings(aid, ids)`, `queueWritingGrading(classId, userId, atomId)`, `getGrading(gid)`, `patchGrading(gid, content?)`, `sendGrading(gid)`, `regradeGrading(gid)`, `listWritingGradings(atomId)`, `markGradingSeen(gid)`
- Produces (`api/assignments.ts`): `GradingInboxItem {type:"grading"; id; atomId; writingTitle; sentAt; unread}`; `InboxItemDTO = AssignmentInboxItem | GradingInboxItem`; `WritingAssignmentPayload.rubric?: Rubric`; `PatchAssignmentInput.rubric?: Rubric | null`.
- Produces (`api/teacher.ts`): `ItemDetail["writing"]` gains `versions: WritingVersionSummary[]`, `grading: GradingSummary | null`, `revising: boolean`.

- [ ] **Step 1: Write the failing normalizer tests**

Create `apps/lite-web/src/api/gradings.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import {
  normalizeGradingContent,
  normalizeGradingRow,
  normalizeRubric,
  normalizeStudentGrading,
  normalizeTeacherGrading,
} from "./gradings";

describe("normalizeRubric", () => {
  it("keeps a points rubric and drops blank dimensions", () => {
    expect(
      normalizeRubric({ scale: "points", max: 20, dimensions: [{ name: "论证", note: "证据" }, { name: "" }], focus: "重点" }),
    ).toEqual({ scale: "points", max: 20, dimensions: [{ name: "论证", note: "证据" }], focus: "重点" });
  });
  it("returns null for a missing or dimensionless rubric", () => {
    expect(normalizeRubric(undefined)).toBeNull();
    expect(normalizeRubric({ scale: "letter", dimensions: [] })).toBeNull();
  });
  it("reads an unknown scale as letter", () => {
    expect(normalizeRubric({ scale: "stars", dimensions: [{ name: "内容" }] })?.scale).toBe("letter");
  });
});

describe("normalizeGradingContent", () => {
  it("maps quote/action to null when absent and source to teacher unless ai", () => {
    const c = normalizeGradingContent({
      overall: { grade: "B", comment: "x" },
      dimensions: [{ name: "内容", grade: "B", comment: "" }],
      points: [
        { kind: "good", quote: "雨", text: "具体", action: null, source: "ai" },
        { kind: "strange", text: "说明", quote: 3 },
      ],
    });
    expect(c?.points).toEqual([
      { kind: "good", quote: "雨", text: "具体", action: null, source: "ai" },
      { kind: "issue", quote: null, text: "说明", action: null, source: "teacher" },
    ]);
  });
  it("returns null for null content", () => {
    expect(normalizeGradingContent(null)).toBeNull();
  });
});

describe("normalizeGradingRow and normalizeTeacherGrading", () => {
  it("keeps a row with no version and no grading", () => {
    expect(normalizeGradingRow({ userId: "u1", displayName: "Phoebe", atomId: null, version: null, grading: null })).toEqual({
      userId: "u1",
      displayName: "Phoebe",
      atomId: null,
      version: null,
      grading: null,
    });
  });
  it("defaults a teacher grading's rubric by language when the server sends none", () => {
    const g = normalizeTeacherGrading({ id: "g1", lang: "en", status: "draft", content: null, rubric: null, versionNumber: 1 });
    expect(g.rubric.dimensions[0].name).toBe("Task Response");
    expect(g.status).toBe("draft");
  });
});

describe("normalizeStudentGrading", () => {
  it("drops a row without content", () => {
    expect(normalizeStudentGrading({ id: "g1", versionNumber: 1, content: null })).toBeNull();
  });
});
```

Append to `apps/lite-web/src/api/assignments.test.ts`, inside `describe("normalizeInboxResponse", …)`:

```ts
  it("keeps sent grading rows next to assignments", () => {
    const raw = {
      items: [
        inboxItem({ id: "a1", unread: false }),
        { type: "grading", id: "g1", atomId: "w1", writingTitle: "雨水去哪儿了", sentAt: "2026-09-15T06:20:00Z", unread: true },
      ],
      unread: 1,
    };
    const result = normalizeInboxResponse(raw);
    expect(result.items.map((it) => `${it.type}:${it.id}`)).toEqual(["assignment:a1", "grading:g1"]);
    expect(result.unread).toBe(1);
  });
```

- [ ] **Step 2: Run to see them fail**

Run: `cd apps/lite-web && pnpm vitest run src/api/gradings.test.ts src/api/assignments.test.ts`
Expected: FAIL (`Cannot find module './gradings'`; the grading row is dropped).

- [ ] **Step 3: Write `api/gradings.ts`**

Create `apps/lite-web/src/api/gradings.ts`:

```ts
// api/gradings.ts — AI 批改 clients. Shapes follow
// apps/api/internal/api/lite_teacher_gradings.go and lite_student_gradings.go.

import { apiFetch } from "./client";

export type GradingStatus = "queued" | "running" | "draft" | "failed" | "sent";
export type PointKind = "good" | "issue";
export const LETTER_GRADES = ["A+", "A", "A-", "B+", "B", "B-", "C+", "C", "C-", "D"] as const;

export interface RubricDimension {
  name: string;
  note: string;
}
export interface Rubric {
  scale: "letter" | "points";
  max?: number;
  dimensions: RubricDimension[];
  focus: string;
}
export interface GradingPoint {
  kind: PointKind;
  quote: string | null;
  text: string;
  action: string | null;
  source: "ai" | "teacher";
}
export interface GradingContent {
  overall: { grade: string; comment: string };
  dimensions: { name: string; grade: string; comment: string }[];
  points: GradingPoint[];
}
export interface GradingSummary {
  id: string;
  status: GradingStatus;
  overallGrade: string | null;
  error: string | null;
  reviewedAt: string | null;
  sentAt: string | null;
}
export interface GradingRow {
  userId: string;
  displayName: string;
  atomId: string | null;
  version: { number: number; submittedAt: string } | null;
  grading: GradingSummary | null;
}
export interface TeacherGrading {
  id: string;
  classId: string;
  assignmentId: string | null;
  userId: string;
  displayName: string;
  atomId: string;
  versionNumber: number;
  latestVersionNumber: number;
  title: string;
  body: string;
  lang: "zh" | "en";
  rubric: Rubric;
  status: GradingStatus;
  content: GradingContent | null;
  error: string | null;
  reviewedAt: string | null;
  sentAt: string | null;
  studentSeenAt: string | null;
  updatedAt: string;
}
export interface StudentGrading {
  id: string;
  versionNumber: number;
  rubric: Rubric;
  content: GradingContent;
  sentAt: string;
  seen: boolean;
}

const s = (v: unknown): string => (typeof v === "string" ? v : "");
const ns = (v: unknown): string | null => (typeof v === "string" ? v : null);
const n = (v: unknown): number => (typeof v === "number" ? v : 0);
const obj = (v: unknown): Record<string, unknown> =>
  v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
const arr = (v: unknown): Record<string, unknown>[] => (Array.isArray(v) ? v.map(obj) : []);

const STATUSES: readonly GradingStatus[] = ["queued", "running", "draft", "failed", "sent"];
const status = (v: unknown): GradingStatus =>
  (STATUSES as readonly unknown[]).includes(v) ? (v as GradingStatus) : "failed";

/** Same defaults as liteassign.DefaultRubric; used only when the server sends none. */
export function defaultRubric(lang: "zh" | "en"): Rubric {
  const names =
    lang === "en"
      ? ["Task Response", "Coherence and Cohesion", "Lexical Resource", "Grammatical Range and Accuracy"]
      : ["内容", "结构", "语言", "书写规范"];
  const notes =
    lang === "en"
      ? ["是否回应题目的全部要求，观点是否展开并有支撑", "段落安排是否清楚，句与句、段与段之间是否衔接", "用词是否准确、多样，搭配是否得当", "句式是否多样，语法是否准确"]
      : ["立意是否明确，材料是否支撑观点", "段落顺序是否清楚，段与段之间是否衔接", "表达是否准确、通顺", "标点、错别字与格式"];
  return { scale: "letter", dimensions: names.map((name, i) => ({ name, note: notes[i] })), focus: "" };
}

export function normalizeRubric(raw: unknown): Rubric | null {
  const r = obj(raw);
  const dimensions = arr(r.dimensions)
    .map((d) => ({ name: s(d.name), note: s(d.note) }))
    .filter((d) => d.name !== "");
  if (dimensions.length === 0) return null;
  const scale = r.scale === "points" ? "points" : "letter";
  return scale === "points" ? { scale, max: n(r.max), dimensions, focus: s(r.focus) } : { scale, dimensions, focus: s(r.focus) };
}

export function normalizeGradingContent(raw: unknown): GradingContent | null {
  if (!raw || typeof raw !== "object") return null;
  const r = obj(raw);
  const overall = obj(r.overall);
  return {
    overall: { grade: s(overall.grade), comment: s(overall.comment) },
    dimensions: arr(r.dimensions).map((d) => ({ name: s(d.name), grade: s(d.grade), comment: s(d.comment) })),
    points: arr(r.points).map((p) => ({
      kind: p.kind === "good" ? "good" : "issue",
      quote: ns(p.quote),
      text: s(p.text),
      action: ns(p.action),
      source: p.source === "ai" ? "ai" : "teacher",
    })),
  };
}

export function normalizeGradingSummary(raw: unknown): GradingSummary | null {
  if (!raw || typeof raw !== "object") return null;
  const r = obj(raw);
  return {
    id: s(r.id),
    status: status(r.status),
    overallGrade: ns(r.overallGrade),
    error: ns(r.error),
    reviewedAt: ns(r.reviewedAt),
    sentAt: ns(r.sentAt),
  };
}

export function normalizeGradingRow(raw: unknown): GradingRow {
  const r = obj(raw);
  const v = r.version && typeof r.version === "object" ? obj(r.version) : null;
  return {
    userId: s(r.userId),
    displayName: s(r.displayName),
    atomId: ns(r.atomId),
    version: v ? { number: n(v.number), submittedAt: s(v.submittedAt) } : null,
    grading: normalizeGradingSummary(r.grading),
  };
}

export function normalizeTeacherGrading(raw: unknown): TeacherGrading {
  const r = obj(raw);
  const lang = r.lang === "en" ? "en" : "zh";
  return {
    id: s(r.id),
    classId: s(r.classId),
    assignmentId: ns(r.assignmentId),
    userId: s(r.userId),
    displayName: s(r.displayName),
    atomId: s(r.atomId),
    versionNumber: n(r.versionNumber),
    latestVersionNumber: n(r.latestVersionNumber),
    title: s(r.title),
    body: s(r.body),
    lang,
    rubric: normalizeRubric(r.rubric) ?? defaultRubric(lang),
    status: status(r.status),
    content: normalizeGradingContent(r.content),
    error: ns(r.error),
    reviewedAt: ns(r.reviewedAt),
    sentAt: ns(r.sentAt),
    studentSeenAt: ns(r.studentSeenAt),
    updatedAt: s(r.updatedAt),
  };
}

export function normalizeStudentGrading(raw: unknown): StudentGrading | null {
  const r = obj(raw);
  const content = normalizeGradingContent(r.content);
  if (!content) return null;
  return {
    id: s(r.id),
    versionNumber: n(r.versionNumber),
    rubric: normalizeRubric(r.rubric) ?? defaultRubric("zh"),
    content,
    sentAt: s(r.sentAt),
    seen: r.seen === true,
  };
}

const teacherBase = "/api/v1/lite/teacher";
const enc = encodeURIComponent;

export async function listAssignmentGradings(aid: string): Promise<GradingRow[]> {
  const r = await apiFetch<{ rows?: unknown[] }>(`${teacherBase}/assignments/${enc(aid)}/gradings`);
  return (Array.isArray(r.rows) ? r.rows : []).map(normalizeGradingRow);
}

export async function queueAssignmentGradings(aid: string, retryFailed: boolean): Promise<number> {
  const r = await apiFetch<{ queued?: number }>(`${teacherBase}/assignments/${enc(aid)}/gradings`, {
    method: "POST",
    body: JSON.stringify({ retryFailed }),
  });
  return n(r.queued);
}

export async function sendReviewedGradings(aid: string, ids: string[]): Promise<number> {
  const r = await apiFetch<{ sent?: number }>(`${teacherBase}/assignments/${enc(aid)}/gradings/send`, {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
  return n(r.sent);
}

export async function queueWritingGrading(classId: string, userId: string, atomId: string): Promise<TeacherGrading> {
  const r = await apiFetch<{ grading: unknown }>(
    `${teacherBase}/classes/${enc(classId)}/students/${enc(userId)}/items/${enc(atomId)}/gradings`,
    { method: "POST" },
  );
  return normalizeTeacherGrading(r.grading);
}

export async function getGrading(gid: string): Promise<TeacherGrading> {
  const r = await apiFetch<{ grading: unknown }>(`${teacherBase}/gradings/${enc(gid)}`);
  return normalizeTeacherGrading(r.grading);
}

/** Without content this is 标记已审阅. */
export async function patchGrading(gid: string, content?: GradingContent): Promise<TeacherGrading> {
  const r = await apiFetch<{ grading: unknown }>(`${teacherBase}/gradings/${enc(gid)}`, {
    method: "PATCH",
    body: JSON.stringify(content ? { content } : {}),
  });
  return normalizeTeacherGrading(r.grading);
}

export async function sendGrading(gid: string): Promise<TeacherGrading> {
  const r = await apiFetch<{ grading: unknown }>(`${teacherBase}/gradings/${enc(gid)}/send`, { method: "POST" });
  return normalizeTeacherGrading(r.grading);
}

export async function regradeGrading(gid: string): Promise<TeacherGrading> {
  const r = await apiFetch<{ grading: unknown }>(`${teacherBase}/gradings/${enc(gid)}/regrade`, { method: "POST" });
  return normalizeTeacherGrading(r.grading);
}

export async function listWritingGradings(atomId: string): Promise<StudentGrading[]> {
  const r = await apiFetch<{ gradings?: unknown[] }>(`/api/v1/writings/${enc(atomId)}/gradings`);
  return (Array.isArray(r.gradings) ? r.gradings : []).map(normalizeStudentGrading).filter((g): g is StudentGrading => g !== null);
}

export async function markGradingSeen(gid: string): Promise<void> {
  await apiFetch<void>(`/api/v1/lite/inbox/gradings/${enc(gid)}/seen`, { method: "POST" });
}
```

- [ ] **Step 4: Inbox union, rubric fields, item payload**

In `apps/lite-web/src/api/assignments.ts`:

1. Add `import type { Rubric } from "./gradings";` below the existing imports.
2. `WritingAssignmentPayload` gains `rubric?: Rubric;`. `PatchAssignmentInput` gains `/** Top-level; editable after students start. null resets to the default. */ rubric?: Rubric | null;`.
3. Replace the `InboxItemDTO` alias and its comment with:

```ts
/** A sent 批改 of one of her writings. */
export interface GradingInboxItem {
  type: "grading";
  id: string;
  atomId: string;
  writingTitle: string;
  sentAt: string;
  unread: boolean;
}

export type InboxItemDTO = AssignmentInboxItem | GradingInboxItem;
```

4. Change `function normalizeInboxItem(raw: Record<string, unknown>): InboxItemDTO` to return `AssignmentInboxItem`, and add below it:

```ts
function normalizeGradingInboxItem(raw: Record<string, unknown>): GradingInboxItem {
  return {
    type: "grading",
    id: s(raw.id),
    atomId: s(raw.atomId),
    writingTitle: s(raw.writingTitle),
    sentAt: s(raw.sentAt),
    unread: raw.unread === true,
  };
}
```

5. In `normalizeInboxResponse`, replace the `items` line with:

```ts
  const items: InboxItemDTO[] = [];
  for (const it of rows) {
    if (it.type === "assignment") items.push(normalizeInboxItem(it));
    else if (it.type === "grading") items.push(normalizeGradingInboxItem(it));
  }
```

and update its doc comment's second paragraph to: "Rows of type `assignment` and `grading` are kept; any other type (a `parent_report` row from a server before 2026-09-15) is dropped, and the badge then counts the kept rows."

In `apps/lite-web/src/inbox/inboxLogic.ts`, change the import to `import type { AssignmentInboxItem, AssignmentKind, InboxItemDTO } from "../api/assignments";` and `openItemsForKind` to:

```ts
export function openItemsForKind(items: readonly InboxItemDTO[], kind: AssignmentKind): AssignmentInboxItem[] {
  return items.filter(
    (it): it is AssignmentInboxItem => it.type === "assignment" && it.kind === kind && OPEN_STATUSES.includes(it.status),
  );
}
```

In `apps/lite-web/src/api/teacher.ts`: add `import { normalizeVersionSummary, type WritingVersionSummary } from "./writings";` and `import { normalizeGradingSummary, type GradingSummary } from "./gradings";`. In `ItemDetail["writing"]` add:

```ts
    /** Submitted versions, newest first. */
    versions: WritingVersionSummary[];
    /** The latest version's 批改, or null. */
    grading: GradingSummary | null;
    revising: boolean;
```

and in `normalizeWriting` add:

```ts
    versions: (Array.isArray(r.versions) ? r.versions : []).map(normalizeVersionSummary).filter((v) => v.number > 0),
    grading: normalizeGradingSummary(r.grading),
    revising: r.revising === true,
```

In `apps/lite-web/src/inbox/InboxPanel.tsx`, keep it compiling until Task 14 renders grading rows: import `type AssignmentInboxItem` instead of `InboxItemDTO`, change `async function open(item: InboxItemDTO)` to `async function open(item: AssignmentInboxItem)`, and change the items line to:

```ts
  const items = sortUnreadFirst(inbox.items.filter((it): it is AssignmentInboxItem => it.type === "assignment"));
```

- [ ] **Step 5: Run tests and typecheck**

Run: `cd apps/lite-web && pnpm vitest run src/api src/inbox && pnpm typecheck`
Expected: tests PASS, typecheck clean. If another file reads assignment fields off `InboxItemDTO`, narrow on `it.type === "assignment"` there.

- [ ] **Step 6: Commit**

```bash
git add apps/lite-web/src/api/gradings.ts apps/lite-web/src/api/gradings.test.ts \
  apps/lite-web/src/api/assignments.ts apps/lite-web/src/api/assignments.test.ts \
  apps/lite-web/src/api/teacher.ts apps/lite-web/src/inbox/inboxLogic.ts apps/lite-web/src/inbox/InboxPanel.tsx
git commit -m "feat(lite-web): grading clients, inbox grading rows, rubric on payloads

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: Frontend pure logic — status, content reducer, highlights, rubric draft

**Files:**
- Create: `apps/lite-web/src/shared/gradingText.ts` (+ `gradingText.test.ts`)
- Create: `apps/lite-web/src/teacher/gradingLogic.ts` (+ `gradingLogic.test.ts`)
- Create: `apps/lite-web/src/teacher/rubricLogic.ts` (+ `rubricLogic.test.ts`)
- Modify: `apps/lite-web/src/teacher/assignmentLogic.ts` (`SettingsDraft.rubric`, validation, payload, patch)
- Modify: `apps/lite-web/src/teacher/assignmentLogic.test.ts` (expected writing payloads now carry the rubric)
- Modify: `apps/lite-web/src/writings/finishedWriting.ts` (+ test) — `gradingVersionLine`, `AI_ATTRIBUTION`

**Interfaces:**
- Consumes: types and `LETTER_GRADES`, `defaultRubric`, `normalizeRubric` from `api/gradings.ts` (Task 9); `splitSentences` from `writings/sentences.ts`.
- Produces:
  - `shared/gradingText.ts`: `interface QuoteRange {start; end; index}`, `quoteRanges(text, quotes: readonly (string|null)[]): QuoteRange[]`, `interface TextSegment {text; index: number|null}`, `highlightSegments(text, ranges): TextSegment[]`, `pickableSentences(text): string[]`
  - `teacher/gradingLogic.ts`: `type GradingRowStatus`, `GRADING_STATUS_LABEL`, `gradingRowStatus(row)`, `gradingStatusHue(status)`, `shouldPoll(statuses)`, `reviewedDraftIds(rows)`, `failedCount(rows)`, `pendingCount(rows)`, `REGRADE_CONFIRM`, `failureText(error)`, `sendAllConfirmText(n)`, `queuedText(n)`, `gradeInScale(rubric, grade)`, `type GradingAction`, `gradingContentReducer(state, action)`, `contentForSave(c)`, `validateGradingContent(c, rubric)`, `POLL_MS = 5000`
  - `teacher/rubricLogic.ts`: `interface RubricDraft {scale; max: string; dimensions: RubricDimension[]; focus}`, `rubricDraftOf(r)`, `rubricDraftFromPayload(payload, lang)`, `validateRubricDraft(d)`, `buildRubric(d)`, `rubricAfterLangChange(d, from, to)`
  - `writings/finishedWriting.ts`: `AI_ATTRIBUTION = "由 AI 起草，老师审阅后发送"`, `gradingVersionLine(gradingVersion, shownVersion): string | null` → `针对 v{n}` when they differ

- [ ] **Step 1: Write the failing tests**

Create `apps/lite-web/src/shared/gradingText.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { highlightSegments, pickableSentences, quoteRanges } from "./gradingText";

const body = "学校后门那片空地一下雨就积水。去年秋天，我在那里摔过一跤。\n\n我读到城市里的雨水花园。";

describe("quoteRanges", () => {
  it("finds each quote, sorted by position, keeping the quote's index", () => {
    const ranges = quoteRanges(body, ["我读到城市里的雨水花园。", null, "学校后门那片空地一下雨就积水。"]);
    expect(ranges.map((r) => r.index)).toEqual([2, 0]);
    expect(body.slice(ranges[0].start, ranges[0].end)).toBe("学校后门那片空地一下雨就积水。");
  });
  it("skips quotes that are missing, blank or not in the text", () => {
    expect(quoteRanges(body, ["", "  ", "雨一直下。"])).toEqual([]);
  });
  it("gives a repeated quote the next occurrence instead of overlapping", () => {
    const text = "雨。雨。";
    expect(quoteRanges(text, ["雨。", "雨。"])).toEqual([
      { start: 0, end: 2, index: 0 },
      { start: 2, end: 4, index: 1 },
    ]);
  });
  it("drops a quote that only overlaps an earlier one", () => {
    expect(quoteRanges(body, ["去年秋天，我在那里", "我在那里摔过一跤。"]).map((r) => r.index)).toEqual([0]);
  });
});

describe("highlightSegments", () => {
  it("covers the whole text in order", () => {
    const segs = highlightSegments(body, quoteRanges(body, ["去年秋天，我在那里摔过一跤。"]));
    expect(segs.map((s) => s.text).join("")).toBe(body);
    expect(segs.filter((s) => s.index !== null).map((s) => s.text)).toEqual(["去年秋天，我在那里摔过一跤。"]);
  });
  it("returns one plain segment with no ranges", () => {
    expect(highlightSegments("雨", [])).toEqual([{ text: "雨", index: null }]);
  });
});

describe("pickableSentences", () => {
  it("only offers sentences that are literally in the text", () => {
    const got = pickableSentences(body);
    expect(got.length).toBeGreaterThan(1);
    for (const s of got) expect(body.includes(s)).toBe(true);
  });
});
```

Create `apps/lite-web/src/teacher/gradingLogic.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { GradingContent, GradingRow, Rubric } from "../api/gradings";
import {
  contentForSave,
  gradeInScale,
  gradingContentReducer,
  gradingRowStatus,
  reviewedDraftIds,
  shouldPoll,
  validateGradingContent,
} from "./gradingLogic";

const row = (over: Partial<GradingRow>): GradingRow => ({
  userId: "u",
  displayName: "Phoebe",
  atomId: "a",
  version: { number: 1, submittedAt: "2026-09-15T06:20:00Z" },
  grading: null,
  ...over,
});
const summary = (status: string, reviewedAt: string | null = null) =>
  ({ id: "g", status, overallGrade: null, error: null, reviewedAt, sentAt: null }) as GradingRow["grading"];

describe("gradingRowStatus", () => {
  it("maps every server state to one row status", () => {
    expect(gradingRowStatus(row({ version: null }))).toBe("not_submitted");
    expect(gradingRowStatus(row({}))).toBe("pending");
    expect(gradingRowStatus(row({ grading: summary("queued") }))).toBe("running");
    expect(gradingRowStatus(row({ grading: summary("running") }))).toBe("running");
    expect(gradingRowStatus(row({ grading: summary("draft") }))).toBe("draft");
    expect(gradingRowStatus(row({ grading: summary("draft", "2026-09-15T07:00:00Z") }))).toBe("reviewed");
    expect(gradingRowStatus(row({ grading: summary("sent", "2026-09-15T07:00:00Z") }))).toBe("sent");
    expect(gradingRowStatus(row({ grading: summary("failed") }))).toBe("failed");
  });
  it("polls only while something is queued or running, and sends only reviewed drafts", () => {
    expect(shouldPoll(["draft", "queued"])).toBe(true);
    expect(shouldPoll(["draft", "failed", "sent"])).toBe(false);
    const rows = [
      row({ grading: { ...summary("draft", "t")!, id: "g1" } }),
      row({ grading: { ...summary("draft")!, id: "g2" } }),
      row({ grading: { ...summary("sent", "t")!, id: "g3" } }),
    ];
    expect(reviewedDraftIds(rows)).toEqual(["g1"]);
  });
});

const letter: Rubric = { scale: "letter", dimensions: [{ name: "内容", note: "" }], focus: "" };
const content = (): GradingContent => ({
  overall: { grade: "B", comment: "x" },
  dimensions: [{ name: "内容", grade: "B", comment: "" }],
  points: [{ kind: "issue", quote: "雨", text: "说明", action: "补充", source: "ai" }],
});

describe("gradingContentReducer", () => {
  it("edits, adds and deletes without mutating the input", () => {
    const start = content();
    let c = gradingContentReducer(start, { type: "overallGrade", value: "A-" });
    c = gradingContentReducer(c, { type: "dimensionComment", index: 0, value: "材料具体。" });
    c = gradingContentReducer(c, { type: "addPoint" });
    c = gradingContentReducer(c, { type: "pointText", index: 1, value: "请注明数据来源。" });
    c = gradingContentReducer(c, { type: "pointQuote", index: 1, value: "雨" });
    expect(start.overall.grade).toBe("B");
    expect(c.overall.grade).toBe("A-");
    expect(c.dimensions[0].comment).toBe("材料具体。");
    expect(c.points[1]).toEqual({ kind: "issue", quote: "雨", text: "请注明数据来源。", action: "", source: "teacher" });
    c = gradingContentReducer(c, { type: "pointKind", index: 0, value: "good" });
    expect(c.points[0].action).toBeNull();
    c = gradingContentReducer(c, { type: "deletePoint", index: 0 });
    expect(c.points.map((p) => p.text)).toEqual(["请注明数据来源。"]);
    expect(gradingContentReducer(c, { type: "load", content: start })).toBe(start);
  });
  it("contentForSave trims and turns blank quote/action into null", () => {
    const c = content();
    c.points[0] = { kind: "issue", quote: "  ", text: " 说明 ", action: "", source: "teacher" };
    expect(contentForSave(c).points[0]).toEqual({ kind: "issue", quote: null, text: "说明", action: null, source: "teacher" });
  });
});

describe("validateGradingContent", () => {
  it("checks grades against the scale and point texts", () => {
    expect(validateGradingContent(content(), letter)).toBeNull();
    const bad = content();
    bad.overall.grade = "E";
    expect(validateGradingContent(bad, letter)).toBe("总评的等级不在评分标准内：E");
    const empty = content();
    empty.points[0].text = " ";
    expect(validateGradingContent(empty, letter)).toBe("第 1 条意见的说明为空");
    const points: Rubric = { scale: "points", max: 20, dimensions: [{ name: "内容", note: "" }], focus: "" };
    expect(gradeInScale(points, "20")).toBe(true);
    expect(gradeInScale(points, "08")).toBe(false);
    expect(gradeInScale(points, "21")).toBe(false);
  });
});
```

Create `apps/lite-web/src/teacher/rubricLogic.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { defaultRubric } from "../api/gradings";
import { buildRubric, rubricAfterLangChange, rubricDraftFromPayload, rubricDraftOf, validateRubricDraft } from "./rubricLogic";

describe("rubric draft", () => {
  it("reads a stored rubric, or the default for the language", () => {
    expect(rubricDraftFromPayload({ rubric: { scale: "points", max: 20, dimensions: [{ name: "论证", note: "" }], focus: "" } }, "zh")).toEqual({
      scale: "points",
      max: "20",
      dimensions: [{ name: "论证", note: "" }],
      focus: "",
    });
    expect(rubricDraftFromPayload({}, "en").dimensions[0].name).toBe("Task Response");
  });
  it("validates with the server's messages", () => {
    const d = rubricDraftOf(defaultRubric("zh"));
    expect(validateRubricDraft(d)).toBeNull();
    expect(validateRubricDraft({ ...d, scale: "points", max: "0" })).toBe("满分需在 1 到 100 之间");
    expect(validateRubricDraft({ ...d, dimensions: [] })).toBe("评分维度需有 1 到 6 项");
    expect(validateRubricDraft({ ...d, dimensions: [{ name: " ", note: "" }] })).toBe("维度名称不能为空，不超过 40 字");
    expect(validateRubricDraft({ ...d, dimensions: [{ name: "内容", note: "" }, { name: "内容 ", note: "" }] })).toBe("维度名称不能重复");
    expect(validateRubricDraft({ ...d, focus: "字".repeat(501) })).toBe("批改重点不超过 500 字");
  });
  it("builds a trimmed rubric; letter scale has no max", () => {
    expect(buildRubric({ scale: "letter", max: "20", dimensions: [{ name: " 内容 ", note: " 看立意 " }], focus: " 重点 " })).toEqual({
      scale: "letter",
      dimensions: [{ name: "内容", note: "看立意" }],
      focus: "重点",
    });
    expect(buildRubric({ scale: "points", max: "20", dimensions: [{ name: "论证", note: "" }], focus: "" }).max).toBe(20);
  });
  it("switches an untouched default to the other language's default, and keeps an edited rubric", () => {
    const zh = rubricDraftOf(defaultRubric("zh"));
    expect(rubricAfterLangChange(zh, "zh", "en").dimensions[0].name).toBe("Task Response");
    const edited = { ...zh, focus: "重点看论证" };
    expect(rubricAfterLangChange(edited, "zh", "en")).toBe(edited);
  });
});
```

Append to `apps/lite-web/src/writings/finishedWriting.test.ts`:

```ts
describe("gradingVersionLine", () => {
  it("names the graded version only when the page shows another one", () => {
    expect(gradingVersionLine(2, 2)).toBeNull();
    expect(gradingVersionLine(1, 2)).toBe("针对 v1");
    expect(gradingVersionLine(1, null)).toBe("针对 v1");
  });
  it("keeps the attribution line verbatim", () => {
    expect(AI_ATTRIBUTION).toBe("由 AI 起草，老师审阅后发送");
  });
});
```

(add `AI_ATTRIBUTION, gradingVersionLine` to that file's import from `./finishedWriting`).

- [ ] **Step 2: Run to see them fail**

Run: `cd apps/lite-web && pnpm vitest run src/shared/gradingText.test.ts src/teacher/gradingLogic.test.ts src/teacher/rubricLogic.test.ts src/writings/finishedWriting.test.ts`
Expected: FAIL (modules and exports missing).

- [ ] **Step 3: Write `shared/gradingText.ts`**

```ts
// shared/gradingText.ts — where a grading's quotes sit in her text, for
// highlighting on the teacher's grading view and the student's finished page.

import { splitSentences } from "../writings/sentences";

export interface QuoteRange {
  start: number;
  end: number;
  /** Position of the quote in the list passed in (the point's index). */
  index: number;
}

/** First non-overlapping occurrence of each quote, sorted by start. A quote
 *  that is blank, missing from the text, or only overlaps earlier ranges gets
 *  no range. */
export function quoteRanges(text: string, quotes: readonly (string | null)[]): QuoteRange[] {
  const taken: QuoteRange[] = [];
  quotes.forEach((q, index) => {
    const needle = q?.trim();
    if (!needle) return;
    let from = 0;
    for (;;) {
      const start = text.indexOf(needle, from);
      if (start < 0) return;
      const end = start + needle.length;
      if (!taken.some((r) => start < r.end && r.start < end)) {
        taken.push({ start, end, index });
        return;
      }
      from = start + 1;
    }
  });
  return taken.sort((a, b) => a.start - b.start);
}

export interface TextSegment {
  text: string;
  index: number | null;
}

/** The text cut into plain and highlighted segments; joined, they are the text. */
export function highlightSegments(text: string, ranges: readonly QuoteRange[]): TextSegment[] {
  const out: TextSegment[] = [];
  let at = 0;
  for (const r of ranges) {
    if (r.start > at) out.push({ text: text.slice(at, r.start), index: null });
    out.push({ text: text.slice(r.start, r.end), index: r.index });
    at = r.end;
  }
  if (at < text.length || out.length === 0) out.push({ text: text.slice(at), index: null });
  return out;
}

/** Sentences a teacher can pick as a point's quote. Only exact substrings are
 *  offered, because the server refuses any other quote. */
export function pickableSentences(text: string): string[] {
  return splitSentences(text)
    .map((s) => s.trim())
    .filter((s) => s !== "" && text.includes(s));
}
```

- [ ] **Step 4: Write `teacher/gradingLogic.ts`**

```ts
// teacher/gradingLogic.ts — pure rules behind the 批改 tab and the grading view.

import { LETTER_GRADES, type GradingContent, type GradingPoint, type GradingRow, type PointKind, type Rubric } from "../api/gradings";

export const POLL_MS = 5000;

export type GradingRowStatus = "not_submitted" | "pending" | "running" | "draft" | "reviewed" | "sent" | "failed";

export const GRADING_STATUS_LABEL: Record<GradingRowStatus, string> = {
  not_submitted: "未提交",
  pending: "待批改",
  running: "批改中",
  draft: "草稿",
  reviewed: "已审阅",
  sent: "已发送",
  failed: "批改失败",
};

export function gradingRowStatus(row: Pick<GradingRow, "version" | "grading">): GradingRowStatus {
  if (!row.version) return "not_submitted";
  const g = row.grading;
  if (!g) return "pending";
  switch (g.status) {
    case "queued":
    case "running":
      return "running";
    case "failed":
      return "failed";
    case "sent":
      return "sent";
    default:
      return g.reviewedAt ? "reviewed" : "draft";
  }
}

export function gradingStatusHue(status: GradingRowStatus): string {
  switch (status) {
    case "sent":
    case "reviewed":
      return "var(--mk-success)";
    case "failed":
      return "var(--mk-danger)";
    case "running":
    case "draft":
      return "var(--mk-warning)";
    default:
      return "var(--mk-muted)";
  }
}

export function shouldPoll(statuses: readonly string[]): boolean {
  return statuses.some((s) => s === "queued" || s === "running");
}

export function reviewedDraftIds(rows: readonly GradingRow[]): string[] {
  return rows.filter((r) => gradingRowStatus(r) === "reviewed").map((r) => r.grading?.id ?? "");
}

export function failedCount(rows: readonly GradingRow[]): number {
  return rows.filter((r) => gradingRowStatus(r) === "failed").length;
}

export function pendingCount(rows: readonly GradingRow[]): number {
  return rows.filter((r) => gradingRowStatus(r) === "pending").length;
}

export const REGRADE_CONFIRM = "重新批改会覆盖当前修改";

export function failureText(error: string | null): string {
  return `批改失败：${error ?? "没有更多信息"}`;
}

export function sendAllConfirmText(n: number): string {
  return `将发送 ${n} 份已审阅的批改`;
}

export function queuedText(n: number): string {
  return n > 0 ? `已加入批改队列：${n} 份` : "没有待批改的作业";
}

/** Same rule as liteassign.GradeInScale. */
export function gradeInScale(rubric: Rubric, grade: string): boolean {
  if (rubric.scale === "letter") return (LETTER_GRADES as readonly string[]).includes(grade);
  if (!/^(0|[1-9]\d*)$/.test(grade)) return false;
  return Number(grade) <= (rubric.max ?? 0);
}

export type GradingAction =
  | { type: "load"; content: GradingContent }
  | { type: "overallGrade"; value: string }
  | { type: "overallComment"; value: string }
  | { type: "dimensionGrade"; index: number; value: string }
  | { type: "dimensionComment"; index: number; value: string }
  | { type: "pointKind"; index: number; value: PointKind }
  | { type: "pointQuote"; index: number; value: string | null }
  | { type: "pointText"; index: number; value: string }
  | { type: "pointAction"; index: number; value: string }
  | { type: "deletePoint"; index: number }
  | { type: "addPoint" };

function editPoint(c: GradingContent, index: number, patch: Partial<GradingPoint>): GradingContent {
  return { ...c, points: c.points.map((p, i) => (i === index ? { ...p, ...patch } : p)) };
}

function editDimension(c: GradingContent, index: number, patch: Partial<GradingContent["dimensions"][number]>): GradingContent {
  return { ...c, dimensions: c.dimensions.map((d, i) => (i === index ? { ...d, ...patch } : d)) };
}

export function gradingContentReducer(state: GradingContent, action: GradingAction): GradingContent {
  switch (action.type) {
    case "load":
      return action.content;
    case "overallGrade":
      return { ...state, overall: { ...state.overall, grade: action.value } };
    case "overallComment":
      return { ...state, overall: { ...state.overall, comment: action.value } };
    case "dimensionGrade":
      return editDimension(state, action.index, { grade: action.value });
    case "dimensionComment":
      return editDimension(state, action.index, { comment: action.value });
    case "pointKind":
      return editPoint(state, action.index, action.value === "good" ? { kind: "good", action: null } : { kind: "issue", action: "" });
    case "pointQuote":
      return editPoint(state, action.index, { quote: action.value });
    case "pointText":
      return editPoint(state, action.index, { text: action.value });
    case "pointAction":
      return editPoint(state, action.index, { action: action.value });
    case "deletePoint":
      return { ...state, points: state.points.filter((_, i) => i !== action.index) };
    case "addPoint":
      return { ...state, points: [...state.points, { kind: "issue", quote: null, text: "", action: "", source: "teacher" }] };
  }
}

const blankToNull = (v: string | null): string | null => {
  const t = v?.trim() ?? "";
  return t === "" ? null : t;
};

/** The body PATCH sends: trimmed, blank quote/action as null, good points without an action. */
export function contentForSave(c: GradingContent): GradingContent {
  return {
    overall: { grade: c.overall.grade.trim(), comment: c.overall.comment.trim() },
    dimensions: c.dimensions.map((d) => ({ name: d.name, grade: d.grade.trim(), comment: d.comment.trim() })),
    points: c.points.map((p) => ({
      kind: p.kind,
      quote: blankToNull(p.quote),
      text: p.text.trim(),
      action: p.kind === "good" ? null : blankToNull(p.action),
      source: p.source,
    })),
  };
}

/** The checks worth making before a request; the server's shape check is the authority. */
export function validateGradingContent(c: GradingContent, rubric: Rubric): string | null {
  if (!gradeInScale(rubric, c.overall.grade.trim())) return `总评的等级不在评分标准内：${c.overall.grade}`;
  for (const d of c.dimensions) {
    if (!gradeInScale(rubric, d.grade.trim())) return `维度「${d.name}」的等级不在评分标准内：${d.grade}`;
  }
  for (let i = 0; i < c.points.length; i++) {
    if (c.points[i].text.trim() === "") return `第 ${i + 1} 条意见的说明为空`;
  }
  return null;
}
```

- [ ] **Step 5: Write `teacher/rubricLogic.ts`**

```ts
// teacher/rubricLogic.ts — the 评分标准 section's draft, checks and request body.
// Messages match liteassign/rubric.go.

import { defaultRubric, normalizeRubric, type Rubric, type RubricDimension } from "../api/gradings";

export interface RubricDraft {
  scale: "letter" | "points";
  /** Raw input; only read when scale is points. */
  max: string;
  dimensions: RubricDimension[];
  focus: string;
}

export function rubricDraftOf(r: Rubric): RubricDraft {
  return { scale: r.scale, max: r.scale === "points" ? String(r.max ?? "") : "", dimensions: r.dimensions.map((d) => ({ ...d })), focus: r.focus };
}

export function rubricDraftFromPayload(payload: Record<string, unknown>, lang: "zh" | "en"): RubricDraft {
  return rubricDraftOf(normalizeRubric(payload.rubric) ?? defaultRubric(lang));
}

const runes = (s: string): number => [...s].length;

export function validateRubricDraft(d: RubricDraft): string | null {
  if (d.scale === "points") {
    const max = /^\d+$/.test(d.max.trim()) ? Number(d.max.trim()) : NaN;
    if (!(max >= 1 && max <= 100)) return "满分需在 1 到 100 之间";
  }
  if (d.dimensions.length < 1 || d.dimensions.length > 6) return "评分维度需有 1 到 6 项";
  const seen = new Set<string>();
  for (const dim of d.dimensions) {
    const name = dim.name.trim();
    if (name === "" || runes(name) > 40) return "维度名称不能为空，不超过 40 字";
    if (seen.has(name)) return "维度名称不能重复";
    seen.add(name);
    if (runes(dim.note.trim()) > 200) return "维度说明不超过 200 字";
  }
  if (runes(d.focus.trim()) > 500) return "批改重点不超过 500 字";
  return null;
}

/** The rubric for a draft that passed validateRubricDraft. */
export function buildRubric(d: RubricDraft): Rubric {
  const dimensions = d.dimensions.map((x) => ({ name: x.name.trim(), note: x.note.trim() }));
  return d.scale === "points"
    ? { scale: "points", max: Number(d.max.trim()), dimensions, focus: d.focus.trim() }
    : { scale: "letter", dimensions, focus: d.focus.trim() };
}

const sameDraft = (a: RubricDraft, b: RubricDraft): boolean => JSON.stringify(a) === JSON.stringify(b);

/** Changing the homework's language swaps an untouched default rubric for the
 *  other language's default; a rubric the teacher edited stays. */
export function rubricAfterLangChange(d: RubricDraft, from: "zh" | "en", to: "zh" | "en"): RubricDraft {
  if (from === to || !sameDraft(d, rubricDraftOf(defaultRubric(from)))) return d;
  return rubricDraftOf(defaultRubric(to));
}
```

- [ ] **Step 6: Carry the rubric in the assignment draft**

In `apps/lite-web/src/teacher/assignmentLogic.ts`:

1. Import: `import { buildRubric, rubricDraftFromPayload, rubricDraftOf, validateRubricDraft, type RubricDraft } from "./rubricLogic";` and `import { defaultRubric } from "../api/gradings";`.
2. `SettingsDraft` gains `rubric: RubricDraft;` (comment: `/** Writing only. Editable after students start (sent as PATCH rubric). */`).
3. `emptySettings` adds `rubric: rubricDraftOf(defaultRubric("zh")),`.
4. In `settingsFromAssignment`, writing branch, after `d.lang = …`: `d.rubric = rubricDraftFromPayload(payload, d.lang);`.
5. In `validateSettings`, writing branch, before `return null;`: `const rubric = validateRubricDraft(d.rubric); if (rubric) return rubric;`.
6. In `buildPayload`, writing branch: `return { prompt: d.prompt.trim(), targetWords: parseTargetWords(d.targetWords) ?? 0, lang: d.lang, rubric: buildRubric(d.rubric) };`.
7. In `buildPatchInput`, before `return { ok: true, value: patch };`:

```ts
  // The rubric is sent on its own even when kind/payload are locked: the
  // server ignores a rubric inside a PATCH payload.
  if (e.settings.kind === "writing") {
    const rubric = validateRubricDraft(e.settings.rubric);
    if (rubric) return { ok: false, error: rubric };
    patch.rubric = buildRubric(e.settings.rubric);
  }
```

In `apps/lite-web/src/teacher/assignmentLogic.test.ts`, every expectation that compares a whole writing payload, a whole writing patch, or `emptySettings()` with `toEqual` now includes the rubric. Update them by adding `rubric: defaultRubric("zh")` (payload / patch) or `rubric: rubricDraftOf(defaultRubric("zh"))` (drafts), importing both. Do not loosen the assertions to `toMatchObject`.

In `apps/lite-web/src/writings/finishedWriting.ts`, append:

```ts
/** Verbatim, on every sent 批改 the student reads. */
export const AI_ATTRIBUTION = "由 AI 起草，老师审阅后发送";

/** 「针对 v{n}」 when the grading is about a version other than the one on the page. */
export function gradingVersionLine(gradingVersion: number, shownVersion: number | null): string | null {
  return gradingVersion === shownVersion ? null : `针对 v${gradingVersion}`;
}
```

- [ ] **Step 7: Run the tests and typecheck**

Run: `cd apps/lite-web && pnpm vitest run src/shared src/teacher src/writings && pnpm typecheck`
Expected: PASS, typecheck clean.

- [ ] **Step 8: Commit**

```bash
git add apps/lite-web/src/shared/gradingText.ts apps/lite-web/src/shared/gradingText.test.ts \
  apps/lite-web/src/teacher/gradingLogic.ts apps/lite-web/src/teacher/gradingLogic.test.ts \
  apps/lite-web/src/teacher/rubricLogic.ts apps/lite-web/src/teacher/rubricLogic.test.ts \
  apps/lite-web/src/teacher/assignmentLogic.ts apps/lite-web/src/teacher/assignmentLogic.test.ts \
  apps/lite-web/src/writings/finishedWriting.ts apps/lite-web/src/writings/finishedWriting.test.ts
git commit -m "feat(lite-web): grading status, content reducer, quote highlights and rubric draft

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: 评分标准 in the homework form and the detail page

**Files:**
- Create: `apps/lite-web/src/teacher/RubricFields.tsx`
- Modify: `apps/lite-web/src/teacher/AssignmentForm.tsx` (`SettingsFields` writing branch)
- Modify: `apps/lite-web/src/teacher/AssignmentDetailPage.tsx` (edit form when settings are locked)

**Interfaces:**
- Consumes: `RubricDraft`, `rubricAfterLangChange` (Task 10); `Field`, `Segmented`, `INPUT_CLS` from `AssignmentForm.tsx`; `Button` from `@/ui`.
- Produces: `export function RubricFields({ value, onChange }: { value: RubricDraft; onChange: (update: (d: RubricDraft) => RubricDraft) => void })`.

The logic this UI depends on is tested in Task 10; this task is checked by `pnpm typecheck` here and by the screenshots in Task 15.

- [ ] **Step 1: Write `RubricFields.tsx`**

Create `apps/lite-web/src/teacher/RubricFields.tsx`:

```tsx
import { Button } from "@/ui";
import type { RubricDimension } from "../api/gradings";
import { Field, INPUT_CLS, Segmented } from "./AssignmentForm";
import type { RubricDraft } from "./rubricLogic";

const MAX_DIMENSIONS = 6;

/**
 * 评分标准 for a writing homework. AI 批改 drafts grades per dimension on this
 * scale. Stays editable after students start; each grading keeps the rubric
 * it was drafted with.
 */
export function RubricFields({
  value,
  onChange,
}: {
  value: RubricDraft;
  onChange: (update: (d: RubricDraft) => RubricDraft) => void;
}) {
  const setDimension = (index: number, patch: Partial<RubricDimension>) =>
    onChange((d) => ({ ...d, dimensions: d.dimensions.map((x, i) => (i === index ? { ...x, ...patch } : x)) }));

  return (
    <fieldset className="flex flex-col gap-4 rounded-mk-md border border-mk-border p-4">
      <legend className="px-1 text-mk-small font-bold text-mk-ink">评分标准</legend>
      <p className="text-mk-small text-mk-muted">AI 批改按评分标准起草等级和评语；学生开始后仍可修改。</p>
      <Segmented
        label="评分方式"
        options={[
          { value: "letter", label: "等级" },
          { value: "points", label: "分数" },
        ]}
        value={value.scale}
        onChange={(scale) => onChange((d) => ({ ...d, scale }))}
      />
      {value.scale === "points" && (
        <div className="sm:w-[200px]">
          <Field label="满分">
            <input
              type="number"
              min={1}
              max={100}
              step={1}
              inputMode="numeric"
              value={value.max}
              onChange={(e) => onChange((d) => ({ ...d, max: e.target.value }))}
              className={INPUT_CLS}
            />
          </Field>
        </div>
      )}
      <div className="flex flex-col gap-3">
        <span className="text-mk-label font-bold text-mk-muted">维度</span>
        {value.dimensions.map((dim, i) => (
          <div key={i} className="flex flex-col gap-2 rounded-mk-md border border-mk-border bg-mk-surface p-3 sm:flex-row sm:items-start">
            <div className="sm:w-[220px]">
              <Field label="名称">
                <input value={dim.name} maxLength={40} onChange={(e) => setDimension(i, { name: e.target.value })} className={INPUT_CLS} />
              </Field>
            </div>
            <div className="min-w-0 flex-1">
              <Field label="说明">
                <textarea value={dim.note} rows={2} maxLength={200} onChange={(e) => setDimension(i, { note: e.target.value })} className={INPUT_CLS} />
              </Field>
            </div>
            <Button
              variant="ghost"
              size="sm"
              disabled={value.dimensions.length <= 1}
              onClick={() => onChange((d) => ({ ...d, dimensions: d.dimensions.filter((_, j) => j !== i) }))}
            >
              删除
            </Button>
          </div>
        ))}
        <div>
          <Button
            variant="secondary"
            size="sm"
            disabled={value.dimensions.length >= MAX_DIMENSIONS}
            onClick={() => onChange((d) => ({ ...d, dimensions: [...d.dimensions, { name: "", note: "" }] }))}
          >
            添加维度
          </Button>
        </div>
      </div>
      <Field label="批改重点">
        <textarea value={value.focus} rows={2} maxLength={500} onChange={(e) => onChange((d) => ({ ...d, focus: e.target.value }))} className={INPUT_CLS} />
      </Field>
    </fieldset>
  );
}
```

`RubricFields.tsx` imports from `AssignmentForm.tsx` and `AssignmentForm.tsx` will import `RubricFields`. ES module cycles between components are fine here because nothing runs at import time; if the bundler warns, move `Field`, `Segmented` and `INPUT_CLS` into `teacher/formControls.tsx` and import them from there in both files.

- [ ] **Step 2: Show it in the writing settings**

In `apps/lite-web/src/teacher/AssignmentForm.tsx`: import `RubricFields` and `rubricAfterLangChange`. In the writing branch of `SettingsFields`:

- the 语言 `Segmented` `onChange` becomes `onChange={(lang) => onChange((d) => ({ ...d, lang, rubric: rubricAfterLangChange(d.rubric, d.lang, lang) }))}`;
- the `WritingExtract` `onFill` updater also sets `rubric: rubricAfterLangChange(d.rubric, d.lang, r.lang)`;
- after the closing `</div>` of the 目标字数 / 语言 row, add:

```tsx
          <RubricFields value={value.rubric} onChange={(update) => onChange((d) => ({ ...d, rubric: update(d.rubric) }))} />
```

- [ ] **Step 3: Keep it editable when settings are locked**

In `apps/lite-web/src/teacher/AssignmentDetailPage.tsx`, import `RubricFields`, and replace the `{editable ? (<SettingsFields …/>) : (<p …>已有学生开始这份作业，类型和设置不能再修改</p>)}` block with:

```tsx
              {editable ? (
                <SettingsFields
                  value={edit.settings}
                  onChange={(update) => setEdit((d) => (d ? { ...d, settings: update(d.settings) } : d))}
                />
              ) : (
                <>
                  <p className="text-mk-small text-mk-muted">已有学生开始这份作业，类型和设置不能再修改</p>
                  {edit.settings.kind === "writing" && (
                    <RubricFields
                      value={edit.settings.rubric}
                      onChange={(update) =>
                        setEdit((d) => (d ? { ...d, settings: { ...d.settings, rubric: update(d.settings.rubric) } } : d))
                      }
                    />
                  )}
                </>
              )}
```

- [ ] **Step 4: Typecheck and run the teacher tests**

Run: `cd apps/lite-web && pnpm typecheck && pnpm vitest run src/teacher`
Expected: clean, PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/teacher/RubricFields.tsx apps/lite-web/src/teacher/AssignmentForm.tsx \
  apps/lite-web/src/teacher/AssignmentDetailPage.tsx
git commit -m "feat(lite-web): 评分标准 section on writing homework

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: 批改 tab, grading view and its route

**Files:**
- Create: `apps/lite-web/src/teacher/GradingTab.tsx`
- Create: `apps/lite-web/src/teacher/GradingPage.tsx`
- Modify: `apps/lite-web/src/teacher/teacherRouting.ts` (+ `teacherRouting.test.ts`)
- Modify: `apps/lite-web/src/teacher/teacherRail.ts`
- Modify: `apps/lite-web/src/teacher/AssignmentDetailPage.tsx` (tabs 学生 / 批改; `onOpenGrading` prop)
- Modify: `apps/lite-web/src/teacher/LiteTeacherShell.tsx`

**Interfaces:**
- Consumes: Task 9 clients; Task 10 `gradingLogic`, `gradingText`; `ReturnDialog` (`{assignmentId, recipient: RecipientDTO, onClose, onReturned}`), `getAssignment`, `failText`, `errorText`, `tintedChipStyle`, `formatDeadline`, `TeacherPage`, `useAlive`.
- Produces:
  - `TeacherRoute` gains `{ view: "grading"; gradingId: string }`, path `/gradings/:gid`.
  - `GradingTab({ assignmentId, onOpenGrading })`, `GradingPage({ gradingId, onBack })` where `onBack: (g: TeacherGrading | null) => void`.
  - `AssignmentDetailPage` gains prop `onOpenGrading: (gradingId: string) => void`.

- [ ] **Step 1: Write the failing routing test**

Append to `apps/lite-web/src/teacher/teacherRouting.test.ts` (inside `describe("teacher routing", …)`; add `teacherRoutePath` to the import if it is not there):

```ts
  it("round-trips the grading view", () => {
    expect(parseTeacherRoute("/gradings/g-1")).toEqual({ view: "grading", gradingId: "g-1" });
    expect(teacherRoutePath({ view: "grading", gradingId: "g-1" })).toBe("/gradings/g-1");
    expect(isTeacherPath("/gradings/g-1")).toBe(true);
    expect(parseTeacherRoute("/gradings")).toEqual({ view: "classes" });
  });
```

Run: `cd apps/lite-web && pnpm vitest run src/teacher/teacherRouting.test.ts`
Expected: FAIL.

- [ ] **Step 2: Add the route**

In `apps/lite-web/src/teacher/teacherRouting.ts`:
- `TeacherRoute` union gains `| { view: "grading"; gradingId: string }` with the comment `// /gradings/:gid — one AI 批改, from a homework's 批改 tab or a student's item page.`;
- `parseTeacherRoute`, after the `parent-reports` block: `if (seg[0] === "gradings" && seg[1]) return { view: "grading", gradingId: seg[1] };`
- `isTeacherPath`: add `first === "gradings" ||`;
- `teacherRoutePath`: `case "grading": return \`/gradings/${enc(r.gradingId)}\`;`

In `apps/lite-web/src/teacher/teacherRail.ts`, the `assignments` case becomes:

```ts
      return route.view === "assignments" || route.view === "assignmentNew" || route.view === "assignment" || route.view === "grading";
```

Run: `cd apps/lite-web && pnpm vitest run src/teacher/teacherRouting.test.ts src/teacher/teacherRail.test.ts`
Expected: PASS.

- [ ] **Step 3: Write `GradingTab.tsx`**

```tsx
import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { listAssignmentGradings, queueAssignmentGradings, sendReviewedGradings, type GradingRow } from "../api/gradings";
import { formatDeadline } from "../shared/deadline";
import { useAlive } from "../shared/useAlive";
import { errorText, failText, tintedChipStyle } from "./assignmentLogic";
import {
  failedCount,
  failureText,
  GRADING_STATUS_LABEL,
  gradingRowStatus,
  gradingStatusHue,
  pendingCount,
  POLL_MS,
  queuedText,
  reviewedDraftIds,
  sendAllConfirmText,
  shouldPoll,
} from "./gradingLogic";

/**
 * 批改 tab of a writing homework: one row per student. Polls every 5s while
 * any row is queued or running.
 */
export function GradingTab({ assignmentId, onOpenGrading }: { assignmentId: string; onOpenGrading: (gradingId: string) => void }) {
  const alive = useAlive();
  const [rows, setRows] = useState<GradingRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmSend, setConfirmSend] = useState(false);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    listAssignmentGradings(assignmentId)
      .then((r) => {
        if (cancelled) return;
        setRows(r);
        setError(null);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
  }, [assignmentId, nonce]);

  const polling = rows !== null && shouldPoll(rows.map((r) => r.grading?.status ?? ""));
  useEffect(() => {
    if (!polling) return;
    const timer = window.setInterval(() => setNonce((n) => n + 1), POLL_MS);
    return () => window.clearInterval(timer);
  }, [polling]);

  async function queue(retryFailed: boolean) {
    if (busy) return;
    setBusy(true);
    setMessage(null);
    try {
      const n = await queueAssignmentGradings(assignmentId, retryFailed);
      if (alive.current) setMessage(queuedText(n));
    } catch (e) {
      if (alive.current) setMessage(failText(retryFailed ? "重试" : "批改", e));
    } finally {
      if (alive.current) {
        setBusy(false);
        setNonce((n) => n + 1);
      }
    }
  }

  const reviewed = rows ? reviewedDraftIds(rows) : [];

  async function sendAll() {
    if (busy || reviewed.length === 0) return;
    setBusy(true);
    setMessage(null);
    try {
      const n = await sendReviewedGradings(assignmentId, reviewed);
      if (alive.current) setMessage(`已发送 ${n} 份`);
    } catch (e) {
      if (alive.current) setMessage(failText("发送", e));
    } finally {
      if (alive.current) {
        setBusy(false);
        setConfirmSend(false);
        setNonce((n) => n + 1);
      }
    }
  }

  if (error) {
    return (
      <div className="mt-4 text-mk-small font-semibold text-mk-danger">
        加载失败：{error}{" "}
        <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
          重试
        </button>
      </div>
    );
  }
  if (rows === null) return <p className="mt-4 text-mk-small text-mk-muted">加载中…</p>;

  return (
    <section className="mt-4 flex flex-col gap-3">
      <p className="text-mk-small text-mk-muted">AI 按评分标准起草批改，老师审阅、修改后发送给学生。</p>
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="primary" size="sm" onClick={() => void queue(false)} disabled={busy || pendingCount(rows) === 0}>
          一键AI批改
        </Button>
        <Button variant="secondary" size="sm" onClick={() => void queue(true)} disabled={busy || failedCount(rows) === 0}>
          重试失败
        </Button>
        <Button variant="secondary" size="sm" onClick={() => setConfirmSend(true)} disabled={busy || reviewed.length === 0}>
          发送全部已审阅
        </Button>
        {message && (
          <span role="status" className="text-mk-small text-mk-secondary">
            {message}
          </span>
        )}
      </div>
      {confirmSend && (
        <div className="flex flex-wrap items-center gap-2 text-mk-small font-semibold text-mk-ink">
          {sendAllConfirmText(reviewed.length)}
          <Button variant="primary" size="sm" onClick={() => void sendAll()} disabled={busy}>
            确认发送
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setConfirmSend(false)} disabled={busy}>
            取消
          </Button>
        </div>
      )}
      <div className="overflow-x-auto rounded-mk-lg border border-mk-border bg-mk-surface">
        <table className="w-full min-w-[640px] border-collapse">
          <thead>
            <tr>
              {["学生", "版本", "状态", "总评", "操作"].map((h) => (
                <th key={h} className="whitespace-nowrap border-b border-mk-border px-3 py-2.5 text-left text-mk-label font-bold text-mk-muted">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => {
              const status = gradingRowStatus(r);
              return (
                <tr key={r.userId}>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small font-bold text-mk-ink">{r.displayName}</td>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">
                    {r.version ? `v${r.version.number} · ${formatDeadline(r.version.submittedAt)}` : "—"}
                  </td>
                  <td className="border-b border-mk-border px-3 py-3 text-mk-small">
                    <span className="inline-block whitespace-nowrap rounded-mk-full px-2.5 py-0.5 font-bold" style={tintedChipStyle(gradingStatusHue(status))}>
                      {GRADING_STATUS_LABEL[status]}
                    </span>
                    {status === "failed" && <p className="mt-1 break-words text-mk-danger">{failureText(r.grading?.error ?? null)}</p>}
                  </td>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">{r.grading?.overallGrade ?? "—"}</td>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small">
                    {r.grading ? (
                      <Button variant="link" size="sm" onClick={() => onOpenGrading(r.grading?.id ?? "")}>
                        查看
                      </Button>
                    ) : (
                      <span className="text-mk-muted">—</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}
```

- [ ] **Step 4: Write `GradingPage.tsx`**

```tsx
import { useEffect, useMemo, useReducer, useState, type CSSProperties } from "react";
import { ArrowLeft } from "lucide-react";
import { Button, Icon } from "@/ui";
import { getAssignment, type RecipientDTO } from "../api/assignments";
import {
  getGrading,
  LETTER_GRADES,
  patchGrading,
  regradeGrading,
  sendGrading,
  type GradingContent,
  type Rubric,
  type TeacherGrading,
} from "../api/gradings";
import { formatDeadline } from "../shared/deadline";
import { highlightSegments, pickableSentences, quoteRanges } from "../shared/gradingText";
import { useAlive } from "../shared/useAlive";
import { errorText, failText } from "./assignmentLogic";
import { INPUT_CLS } from "./AssignmentForm";
import {
  contentForSave,
  failureText,
  gradingContentReducer,
  POLL_MS,
  REGRADE_CONFIRM,
  shouldPoll,
  validateGradingContent,
} from "./gradingLogic";
import { ReturnDialog } from "./ReturnDialog";
import { TeacherPage } from "./TeacherPage";

const EMPTY: GradingContent = { overall: { grade: "", comment: "" }, dimensions: [], points: [] };
const MARK_STYLE: CSSProperties = { background: "color-mix(in srgb, var(--mk-warning) 22%, transparent)", color: "inherit" };
const PIECE_CLS = "whitespace-pre-wrap font-mk-piece text-mk-report-piece text-mk-ink";

/**
 * GradingPage — `/gradings/:gid`. Left: the graded version with quoted
 * sentences highlighted. Right: the editable grading and its actions.
 * While a teacher point picks its quote, the left side lists her sentences
 * as buttons.
 */
export function GradingPage({ gradingId, onBack }: { gradingId: string; onBack: (g: TeacherGrading | null) => void }) {
  const alive = useAlive();
  const [grading, setGrading] = useState<TeacherGrading | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [content, dispatch] = useReducer(gradingContentReducer, EMPTY);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [confirmRegrade, setConfirmRegrade] = useState(false);
  const [picking, setPicking] = useState<number | null>(null);
  const [returning, setReturning] = useState<RecipientDTO | null>(null);
  const [nonce, setNonce] = useState(0);

  function apply(g: TeacherGrading, replaceContent: boolean) {
    setGrading(g);
    if (replaceContent && g.content) {
      dispatch({ type: "load", content: g.content });
      setDirty(false);
    }
  }

  useEffect(() => {
    let cancelled = false;
    getGrading(gradingId)
      .then((g) => {
        if (cancelled) return;
        setLoadError(null);
        // A poll must not overwrite what she is typing.
        apply(g, !dirty);
      })
      .catch((e: unknown) => {
        if (!cancelled) setLoadError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
    // `dirty` is read at fetch time on purpose; it must not trigger a refetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gradingId, nonce]);

  const polling = grading !== null && shouldPoll([grading.status]);
  useEffect(() => {
    if (!polling) return;
    const timer = window.setInterval(() => setNonce((n) => n + 1), POLL_MS);
    return () => window.clearInterval(timer);
  }, [polling]);

  const edit = (action: Parameters<typeof dispatch>[0]) => {
    dispatch(action);
    setDirty(true);
  };

  const ranges = useMemo(() => (grading ? quoteRanges(grading.body, content.points.map((p) => p.quote)) : []), [grading, content.points]);

  async function run(verb: string, task: () => Promise<TeacherGrading>) {
    if (busy) return;
    setBusy(true);
    setMessage(null);
    try {
      const g = await task();
      if (alive.current) apply(g, true);
    } catch (e) {
      if (alive.current) setMessage(failText(verb, e));
    } finally {
      if (alive.current) setBusy(false);
    }
  }

  function saveContent(): Promise<TeacherGrading> {
    const body = contentForSave(content);
    const invalid = grading ? validateGradingContent(body, grading.rubric) : null;
    if (invalid) return Promise.reject(new Error(invalid));
    return patchGrading(gradingId, body);
  }

  async function openReturn() {
    if (!grading?.assignmentId) return;
    try {
      const { recipients } = await getAssignment(grading.assignmentId);
      const r = recipients.find((x) => x.userId === grading.userId);
      if (alive.current) setReturning(r ?? null);
      if (alive.current && !r) setMessage("退回失败：这名学生不在这份作业中");
    } catch (e) {
      if (alive.current) setMessage(failText("退回", e));
    }
  }

  const back = (
    <button
      type="button"
      onClick={() => onBack(grading)}
      className="flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <Icon icon={ArrowLeft} size={15} />
      返回
    </button>
  );

  if (loadError) {
    return (
      <TeacherPage>
        {back}
        <p className="mt-4 text-mk-small font-semibold text-mk-danger">加载失败：{loadError}</p>
      </TeacherPage>
    );
  }
  if (!grading) {
    return (
      <TeacherPage>
        {back}
        <p className="mt-4 text-mk-body text-mk-muted">加载中…</p>
      </TeacherPage>
    );
  }

  const editable = grading.content !== null && (grading.status === "draft" || grading.status === "sent");
  const canRegrade = grading.status === "draft" || grading.status === "failed";

  return (
    <TeacherPage>
      {back}
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <h1 className="teacher-page-title">{grading.displayName} · {grading.title}</h1>
        <span className="text-mk-small text-mk-muted">
          v{grading.versionNumber}
          {grading.latestVersionNumber > grading.versionNumber ? ` · 最新 v${grading.latestVersionNumber}` : ""}
        </span>
      </div>
      <p className="mt-1 text-mk-small text-mk-muted">
        {grading.status === "sent" && grading.sentAt
          ? `已发送 ${formatDeadline(grading.sentAt)} · ${grading.studentSeenAt ? "学生已读" : "学生未读"}`
          : grading.status === "queued" || grading.status === "running"
            ? "批改中"
            : grading.reviewedAt
              ? "已审阅"
              : grading.status === "draft"
                ? "草稿"
                : ""}
      </p>
      {grading.status === "failed" && (
        <p role="alert" className="mt-2 break-words text-mk-small font-semibold text-mk-danger">
          {failureText(grading.error)}
        </p>
      )}

      <div className="mt-6 grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,44rem)_minmax(20rem,1fr)]">
        <article className="min-w-0">
          {picking !== null ? (
            <div className="flex flex-col gap-2">
              <p className="text-mk-small text-mk-muted">请选择一句作为第 {picking + 1} 条意见的引文</p>
              {pickableSentences(grading.body).map((s, i) => (
                <button
                  key={i}
                  type="button"
                  onClick={() => {
                    edit({ type: "pointQuote", index: picking, value: s });
                    setPicking(null);
                  }}
                  className="rounded-mk-sm border border-mk-border px-3 py-2 text-left text-mk-body text-mk-ink hover:bg-mk-paper"
                >
                  {s}
                </button>
              ))}
              <div>
                <Button variant="ghost" size="sm" onClick={() => setPicking(null)}>
                  取消选择
                </Button>
              </div>
            </div>
          ) : (
            <div className={PIECE_CLS}>
              {highlightSegments(grading.body, ranges).map((seg, i) =>
                seg.index === null ? (
                  <span key={i}>{seg.text}</span>
                ) : (
                  <mark key={i} id={`grading-quote-${seg.index}`} style={MARK_STYLE}>
                    {seg.text}
                  </mark>
                ),
              )}
            </div>
          )}
        </article>

        <aside className="flex min-w-0 flex-col gap-4">
          {editable ? (
            <GradingEditor
              rubric={grading.rubric}
              content={content}
              onEdit={edit}
              onPickQuote={setPicking}
              onShowQuote={(i) => document.getElementById(`grading-quote-${i}`)?.scrollIntoView({ block: "center", behavior: "smooth" })}
            />
          ) : (
            grading.status !== "failed" && <p className="text-mk-small text-mk-muted">批改完成后可以在这里修改。</p>
          )}

          <div className="flex flex-wrap gap-2 border-t border-mk-border pt-4">
            {editable && (
              <Button variant="primary" size="sm" disabled={busy} onClick={() => void run("保存", saveContent)}>
                {grading.status === "sent" ? "保存并发送" : "保存"}
              </Button>
            )}
            {editable && grading.status === "draft" && (
              <Button variant="secondary" size="sm" disabled={busy} onClick={() => void run("审阅", () => (dirty ? saveContent() : patchGrading(gradingId)))}>
                标记已审阅
              </Button>
            )}
            {editable && grading.status === "draft" && (
              <Button
                variant="secondary"
                size="sm"
                disabled={busy}
                onClick={() =>
                  void run("发送", async () => {
                    if (dirty) await saveContent();
                    return sendGrading(gradingId);
                  })
                }
              >
                发送
              </Button>
            )}
            {canRegrade && (
              <Button
                variant="secondary"
                size="sm"
                disabled={busy}
                onClick={() => (grading.status === "draft" ? setConfirmRegrade(true) : void run("重新批改", () => regradeGrading(gradingId)))}
              >
                重新批改
              </Button>
            )}
            {grading.assignmentId && (
              <Button variant="secondary" size="sm" disabled={busy} onClick={() => void openReturn()}>
                退回修改
              </Button>
            )}
          </div>
          {confirmRegrade && (
            <div className="flex flex-wrap items-center gap-2 text-mk-small font-semibold text-mk-danger">
              {REGRADE_CONFIRM}
              <Button
                variant="danger"
                size="sm"
                disabled={busy}
                onClick={() => {
                  setConfirmRegrade(false);
                  void run("重新批改", () => regradeGrading(gradingId));
                }}
              >
                确认重新批改
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setConfirmRegrade(false)}>
                取消
              </Button>
            </div>
          )}
          {message && (
            <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
              {message}
            </p>
          )}
        </aside>
      </div>

      {returning && grading.assignmentId && (
        <ReturnDialog assignmentId={grading.assignmentId} recipient={returning} onClose={() => setReturning(null)} onReturned={() => setReturning(null)} />
      )}
    </TeacherPage>
  );
}

function GradeInput({ rubric, value, onChange, label }: { rubric: Rubric; value: string; onChange: (v: string) => void; label: string }) {
  if (rubric.scale === "letter") {
    return (
      <select aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} className={`${INPUT_CLS} w-24`}>
        {!(LETTER_GRADES as readonly string[]).includes(value) && <option value={value}>{value || "—"}</option>}
        {LETTER_GRADES.map((g) => (
          <option key={g} value={g}>
            {g}
          </option>
        ))}
      </select>
    );
  }
  return (
    <input aria-label={label} type="number" min={0} max={rubric.max} step={1} value={value} onChange={(e) => onChange(e.target.value)} className={`${INPUT_CLS} w-24`} />
  );
}

function GradingEditor({
  rubric,
  content,
  onEdit,
  onPickQuote,
  onShowQuote,
}: {
  rubric: Rubric;
  content: GradingContent;
  onEdit: (a: Parameters<typeof gradingContentReducer>[1]) => void;
  onPickQuote: (index: number) => void;
  onShowQuote: (index: number) => void;
}) {
  return (
    <div className="flex flex-col gap-4">
      <section className="flex flex-col gap-2">
        <h2 className="text-mk-small font-bold text-mk-ink">总评</h2>
        <GradeInput rubric={rubric} label="总评等级" value={content.overall.grade} onChange={(v) => onEdit({ type: "overallGrade", value: v })} />
        <textarea aria-label="总评评语" rows={3} value={content.overall.comment} onChange={(e) => onEdit({ type: "overallComment", value: e.target.value })} className={INPUT_CLS} />
      </section>

      <section className="flex flex-col gap-2">
        <h2 className="text-mk-small font-bold text-mk-ink">维度</h2>
        {content.dimensions.map((d, i) => (
          <div key={d.name} className="flex flex-col gap-1.5">
            <div className="flex items-center gap-2">
              <span className="min-w-0 flex-1 text-mk-small text-mk-ink">{d.name}</span>
              <GradeInput rubric={rubric} label={`${d.name}等级`} value={d.grade} onChange={(v) => onEdit({ type: "dimensionGrade", index: i, value: v })} />
            </div>
            <textarea aria-label={`${d.name}评语`} rows={2} value={d.comment} onChange={(e) => onEdit({ type: "dimensionComment", index: i, value: e.target.value })} className={INPUT_CLS} />
          </div>
        ))}
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-mk-small font-bold text-mk-ink">意见</h2>
        {content.points.map((p, i) => (
          <div key={i} className="flex flex-col gap-2 rounded-mk-md border border-mk-border bg-mk-surface p-3">
            <div className="flex flex-wrap items-center gap-2">
              <select aria-label="类型" value={p.kind} onChange={(e) => onEdit({ type: "pointKind", index: i, value: e.target.value === "good" ? "good" : "issue" })} className={`${INPUT_CLS} w-24`}>
                <option value="good">优点</option>
                <option value="issue">问题</option>
              </select>
              <span className="text-mk-label text-mk-muted">{p.source === "ai" ? "AI" : "老师"}</span>
              <Button variant="ghost" size="sm" onClick={() => onEdit({ type: "deletePoint", index: i })}>
                删除
              </Button>
            </div>
            <div className="flex flex-wrap items-center gap-2 text-mk-small">
              <span className="text-mk-muted">引文</span>
              {p.quote ? (
                <button type="button" onClick={() => onShowQuote(i)} className="min-w-0 text-left text-mk-ink underline">
                  「{p.quote}」
                </button>
              ) : (
                <span className="text-mk-muted">—</span>
              )}
              <Button variant="link" size="sm" onClick={() => onPickQuote(i)}>
                选择引文
              </Button>
              {p.quote && (
                <Button variant="link" size="sm" onClick={() => onEdit({ type: "pointQuote", index: i, value: null })}>
                  清除
                </Button>
              )}
            </div>
            <textarea aria-label="说明" rows={2} value={p.text} onChange={(e) => onEdit({ type: "pointText", index: i, value: e.target.value })} className={INPUT_CLS} />
            {p.kind === "issue" && (
              <textarea aria-label="修改建议" placeholder="修改建议" rows={2} value={p.action ?? ""} onChange={(e) => onEdit({ type: "pointAction", index: i, value: e.target.value })} className={INPUT_CLS} />
            )}
          </div>
        ))}
        <div>
          <Button variant="secondary" size="sm" onClick={() => onEdit({ type: "addPoint" })}>
            添加意见
          </Button>
        </div>
      </section>
    </div>
  );
}
```

- [ ] **Step 5: Tabs on the detail page and the shell route**

In `apps/lite-web/src/teacher/AssignmentDetailPage.tsx`:
- add prop `onOpenGrading: (gradingId: string) => void` and import `GradingTab`;
- add state `const [tab, setTab] = useState<"students" | "grading">("students");` and reset it to `"students"` in the `[assignmentId]` reset effect;
- directly after `<AssignmentHeader …/>` / the edit form (before `<section className="mt-8">` 学生), render the tab list only for writing homework:

```tsx
          {assignment.kind === "writing" && (
            <div role="tablist" aria-label="作业视图" className="mt-8 flex gap-2 border-b border-mk-border">
              {(["students", "grading"] as const).map((key) => (
                <button
                  key={key}
                  type="button"
                  role="tab"
                  aria-selected={tab === key}
                  onClick={() => setTab(key)}
                  className={
                    "-mb-px border-b-2 px-3 py-2 text-mk-small transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 " +
                    (tab === key ? "border-mk-accent font-bold text-mk-accent-700" : "border-transparent text-mk-muted hover:text-mk-ink")
                  }
                >
                  {key === "students" ? "学生" : "批改"}
                </button>
              ))}
            </div>
          )}
          {assignment.kind === "writing" && tab === "grading" ? (
            <GradingTab assignmentId={assignment.id} onOpenGrading={onOpenGrading} />
          ) : (
            <>
              {/* the existing 学生 and 添加学生 sections, unchanged */}
            </>
          )}
```

Move the two existing `<section className="mt-8">` blocks (学生, 添加学生) inside that fragment, unchanged.

In `apps/lite-web/src/teacher/LiteTeacherShell.tsx`: import `GradingPage`; pass `onOpenGrading={(gradingId) => go({ view: "grading", gradingId })}` to `AssignmentDetailPage`; and add:

```tsx
        {route.view === "grading" && (
          <GradingPage
            key={route.gradingId}
            gradingId={route.gradingId}
            onBack={(g) =>
              g?.assignmentId
                ? go({ view: "assignment", assignmentId: g.assignmentId })
                : g
                  ? go({ view: "item", classId: g.classId, userId: g.userId, atomId: g.atomId })
                  : go({ view: "assignments" })
            }
          />
        )}
```

- [ ] **Step 6: Typecheck and tests**

Run: `cd apps/lite-web && pnpm typecheck && pnpm vitest run src/teacher`
Expected: clean, PASS. If the lint config rejects the `eslint-disable-next-line` comment (no such rule installed), delete that comment line.

- [ ] **Step 7: Commit**

```bash
git add apps/lite-web/src/teacher/GradingTab.tsx apps/lite-web/src/teacher/GradingPage.tsx \
  apps/lite-web/src/teacher/teacherRouting.ts apps/lite-web/src/teacher/teacherRouting.test.ts \
  apps/lite-web/src/teacher/teacherRail.ts apps/lite-web/src/teacher/AssignmentDetailPage.tsx \
  apps/lite-web/src/teacher/LiteTeacherShell.tsx
git commit -m "feat(lite-web): 批改 tab and grading view for writing homework

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: Teacher item page — versions, 批改 button, object comment points

**Files:**
- Modify: `apps/lite-web/src/teacher/ItemPage.tsx` (`WritingSection` at ~line 404, its call at ~line 253, props)
- Modify: `apps/lite-web/src/teacher/ItemPage.test.ts`
- Modify: `apps/lite-web/src/teacher/LiteTeacherShell.tsx` (pass `onOpenGrading`)

**Interfaces:**
- Consumes: `ItemDetail["writing"].versions/.grading` (Task 9); `queueWritingGrading` (Task 9); `GRADING_STATUS_LABEL`, `gradingRowStatus`, `gradingStatusHue` (Task 10); `versionLine` (`writings/finishedWriting.ts`); `tintedChipStyle`.
- Produces:
  - `export function commentPointLines(points: unknown[]): CommentPointLine[]` with `CommentPointLine = { kind: "good" | "issue" | "note"; text: string; action: string | null; quote: string | null }` — string points become `note`; object points keep text, action and quote; anything else is dropped.
  - `export function itemGradingAction(versionCount: number, grading: GradingSummary | null): "none" | "grade" | "open"`.
  - `ItemPage` gains prop `onOpenGrading: (gradingId: string) => void`.

- [ ] **Step 1: Write the failing tests**

Append to `apps/lite-web/src/teacher/ItemPage.test.ts` (extend the import with `commentPointLines, itemGradingAction`):

```ts
describe("commentPointLines", () => {
  // The bug this pins: AI comment points are objects, and the page used to
  // render only string points, so teachers saw only the summaries.
  it("renders object points with their quote and action, and old string points as notes", () => {
    expect(
      commentPointLines([
        "旧格式的一条意见",
        { kind: "good", text: "用具体经历引出问题。", quote: "去年秋天，我在那里摔过一跤。" },
        { kind: "issue", text: "材料与观点之间没有说明。", action: "补一句说明。", quote: "我读到城市里的雨水花园。" },
        { kind: "issue", text: "" },
        42,
      ]),
    ).toEqual([
      { kind: "note", text: "旧格式的一条意见", action: null, quote: null },
      { kind: "good", text: "用具体经历引出问题。", action: null, quote: "去年秋天，我在那里摔过一跤。" },
      { kind: "issue", text: "材料与观点之间没有说明。", action: "补一句说明。", quote: "我读到城市里的雨水花园。" },
    ]);
  });
});

describe("itemGradingAction", () => {
  it("offers 批改 only for a submitted writing without a grading, and opens an existing one", () => {
    expect(itemGradingAction(0, null)).toBe("none");
    expect(itemGradingAction(1, null)).toBe("grade");
    expect(itemGradingAction(2, { id: "g", status: "draft", overallGrade: "B", error: null, reviewedAt: null, sentAt: null })).toBe("open");
  });
});
```

Run: `cd apps/lite-web && pnpm vitest run src/teacher/ItemPage.test.ts`
Expected: FAIL (exports missing).

- [ ] **Step 2: Implement**

In `apps/lite-web/src/teacher/ItemPage.tsx`, add imports:

```ts
import { Button } from "@/ui";
import { queueWritingGrading, type GradingSummary } from "../api/gradings";
import { versionLine } from "../writings/finishedWriting";
import { tintedChipStyle } from "./assignmentLogic";
import { GRADING_STATUS_LABEL, gradingRowStatus, gradingStatusHue } from "./gradingLogic";
```

(merge `Button` into the existing `@/ui` import). Export the helpers near `prosePendingLabel`:

```ts
export interface CommentPointLine {
  kind: "good" | "issue" | "note";
  text: string;
  action: string | null;
  quote: string | null;
}

/** 印记's comment points in both stored shapes: an old string, or the
 *  {kind, text, action, quote} object writing_comment.go writes today. */
export function commentPointLines(points: unknown[]): CommentPointLine[] {
  const out: CommentPointLine[] = [];
  for (const p of points) {
    if (typeof p === "string") {
      if (p.trim()) out.push({ kind: "note", text: p, action: null, quote: null });
      continue;
    }
    if (!p || typeof p !== "object") continue;
    const r = p as Record<string, unknown>;
    const text = typeof r.text === "string" ? r.text.trim() : "";
    if (!text) continue;
    out.push({
      kind: r.kind === "good" ? "good" : "issue",
      text,
      action: typeof r.action === "string" && r.action.trim() ? r.action : null,
      quote: typeof r.quote === "string" && r.quote.trim() ? r.quote : null,
    });
  }
  return out;
}

export function itemGradingAction(versionCount: number, grading: GradingSummary | null): "none" | "grade" | "open" {
  if (grading) return "open";
  return versionCount > 0 ? "grade" : "none";
}
```

`ItemPage` props gain `onOpenGrading: (gradingId: string) => void`; the call at ~line 253 becomes:

```tsx
      <WritingSection writing={detail.writing} classId={classId} userId={userId} atomId={atomId} onOpenGrading={onOpenGrading} />
```

Replace `function WritingSection({ writing }: …)` with a version whose signature is:

```tsx
function WritingSection({
  writing,
  classId,
  userId,
  atomId,
  onOpenGrading,
}: {
  writing: ItemDetail["writing"];
  classId: string;
  userId: string;
  atomId: string;
  onOpenGrading: (gradingId: string) => void;
}) {
  const alive = useAlive();
  const [busy, setBusy] = useState(false);
  const [gradeError, setGradeError] = useState<string | null>(null);
  if (!writing) return null;
  const { targetWords, lang, structureKey, outline, snippets, comments, versions, grading } = writing;
  const action = itemGradingAction(versions.length, grading);

  async function grade() {
    if (busy) return;
    setBusy(true);
    setGradeError(null);
    try {
      const g = await queueWritingGrading(classId, userId, atomId);
      if (alive.current) onOpenGrading(g.id);
    } catch (e) {
      if (alive.current) setGradeError(`批改失败：${e instanceof ApiError ? e.message : String(e)}`);
    } finally {
      if (alive.current) setBusy(false);
    }
  }
```

Note the hooks run before `if (!writing) return null;`, so they are unconditional. Keep the existing JSX body (heading, 目标字数 line, 提纲, 片段) and make three changes in it:

1. After the 目标字数 / 语言 line, add the versions and the 批改 control:

```tsx
      {versions.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">版本</h3>
          <ul className="mt-2 flex flex-col gap-1">
            {versions.map((v) => (
              <li key={v.number} className="text-mk-small text-mk-ink">
                {versionLine(v, lang)}
              </li>
            ))}
          </ul>
        </div>
      )}
      <div className="mt-4 flex flex-wrap items-center gap-2">
        {action === "grade" && (
          <Button variant="secondary" size="sm" disabled={busy} onClick={() => void grade()}>
            AI 批改
          </Button>
        )}
        {action === "open" && grading && (
          <>
            <span
              className="inline-block whitespace-nowrap rounded-mk-full px-2.5 py-0.5 text-mk-small font-bold"
              style={tintedChipStyle(gradingStatusHue(gradingRowStatus({ version: versions[0] ?? null, grading })))}
            >
              {GRADING_STATUS_LABEL[gradingRowStatus({ version: versions[0] ?? null, grading })]}
            </span>
            <Button variant="link" size="sm" onClick={() => onOpenGrading(grading.id)}>
              查看批改
            </Button>
          </>
        )}
        {gradeError && (
          <span role="alert" className="text-mk-small font-semibold text-mk-danger">
            {gradeError}
          </span>
        )}
      </div>
```

(`gradingRowStatus` takes `version` as `{number, submittedAt}`; `WritingVersionSummary` has both fields.)

2. Replace the AI 批注 `c.points` rendering (the `Array.isArray(c.points) && c.points.some(…)` ternary) with:

```tsx
                {commentPointLines(c.points).length > 0 && (
                  <ul className="mt-1.5 flex flex-col gap-1.5">
                    {commentPointLines(c.points).map((p, j) => (
                      <li key={j} className="text-mk-small text-mk-ink">
                        {p.kind !== "note" && <span className="text-mk-muted">{p.kind === "good" ? "优点：" : "问题："}</span>}
                        {p.quote && <span className="text-mk-muted">「{p.quote}」 </span>}
                        {p.text}
                        {p.action && <span className="block text-mk-muted">修改建议：{p.action}</span>}
                      </li>
                    ))}
                  </ul>
                )}
```

In `LiteTeacherShell.tsx`, pass `onOpenGrading={(gradingId) => go({ view: "grading", gradingId })}` to `ItemPage`.

- [ ] **Step 3: Tests and typecheck**

Run: `cd apps/lite-web && pnpm vitest run src/teacher/ItemPage.test.ts && pnpm typecheck`
Expected: PASS, clean.

- [ ] **Step 4: Commit**

```bash
git add apps/lite-web/src/teacher/ItemPage.tsx apps/lite-web/src/teacher/ItemPage.test.ts apps/lite-web/src/teacher/LiteTeacherShell.tsx
git commit -m "feat(lite-web): item page lists versions, grades a writing, shows object comment points

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 14: Student side — 老师批改 on the finished page, grading rows in the inbox

**Files:**
- Create: `apps/lite-web/src/writings/TeacherGradingPanel.tsx`
- Modify: `apps/lite-web/src/writings/FinishedWritingPage.tsx`
- Modify: `apps/lite-web/src/inbox/InboxPanel.tsx`

**Interfaces:**
- Consumes: `listWritingGradings`, `markGradingSeen`, `StudentGrading` (Task 9); `AI_ATTRIBUTION`, `gradingVersionLine` (Task 10); `quoteRanges`, `highlightSegments` (Task 10); `GradingInboxItem`, `AssignmentInboxItem` (Task 9); `navigate` (`../routing`).
- Produces: `TeacherGradingPanel({ gradings, error, shownVersion, onQuote })` with `onQuote: (versionNumber: number, quote: string) => void`; renders nothing when there is no error and no grading.

Logic used here is tested in Tasks 9–10; this task is checked by typecheck and the Task 15 screenshots.

- [ ] **Step 1: Write `TeacherGradingPanel.tsx`**

```tsx
import type { StudentGrading } from "../api/gradings";
import { formatDeadline } from "../shared/deadline";
import { AI_ATTRIBUTION, gradingVersionLine } from "./finishedWriting";

/**
 * 老师批改 in the finished page's rail: the gradings the teacher sent, newest
 * version first. Absent when there are none. A point's quote shows that
 * version on the left with the sentence highlighted.
 */
export function TeacherGradingPanel({
  gradings,
  error,
  shownVersion,
  onQuote,
}: {
  gradings: StudentGrading[];
  error: string | null;
  shownVersion: number | null;
  onQuote: (versionNumber: number, quote: string) => void;
}) {
  if (!error && gradings.length === 0) return null;
  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-mk-small font-semibold text-mk-secondary">老师批改</h2>
      {error && (
        <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
          加载失败：{error}
        </p>
      )}
      {gradings.map((g) => {
        const forLine = gradingVersionLine(g.versionNumber, shownVersion);
        return (
          <article key={g.id} className="flex flex-col gap-3 rounded-mk-md border border-mk-border bg-mk-surface p-4">
            <div className="flex flex-wrap items-baseline gap-2">
              <span className="text-mk-h3 font-bold text-mk-ink">{g.content.overall.grade}</span>
              <span className="text-mk-small text-mk-muted">总评</span>
              {forLine && <span className="text-mk-small text-mk-muted">{forLine}</span>}
              <span className="ml-auto text-mk-label text-mk-muted">{formatDeadline(g.sentAt)}</span>
            </div>
            {g.content.overall.comment && <p className="whitespace-pre-wrap text-mk-small text-mk-ink">{g.content.overall.comment}</p>}
            {g.content.dimensions.length > 0 && (
              <dl className="grid grid-cols-[auto_auto_1fr] gap-x-3 gap-y-1.5 text-mk-small">
                {g.content.dimensions.map((d) => (
                  <div key={d.name} className="contents">
                    <dt className="text-mk-muted">{d.name}</dt>
                    <dd className="font-bold text-mk-ink">{d.grade}</dd>
                    <dd className="text-mk-ink">{d.comment}</dd>
                  </div>
                ))}
              </dl>
            )}
            {g.content.points.length > 0 && (
              <ul className="flex flex-col gap-2">
                {g.content.points.map((p, i) => (
                  <li key={i} className="flex flex-col gap-1 text-mk-small">
                    <span className="text-mk-label font-bold text-mk-muted">{p.kind === "good" ? "优点" : "问题"}</span>
                    {p.quote && (
                      <button
                        type="button"
                        onClick={() => onQuote(g.versionNumber, p.quote ?? "")}
                        className="text-left text-mk-secondary underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                      >
                        「{p.quote}」
                      </button>
                    )}
                    <span className="text-mk-ink">{p.text}</span>
                    {p.action && <span className="text-mk-ink">修改建议：{p.action}</span>}
                  </li>
                ))}
              </ul>
            )}
            <p className="text-mk-label text-mk-muted">{AI_ATTRIBUTION}</p>
          </article>
        );
      })}
    </section>
  );
}
```

- [ ] **Step 2: Put it in the finished page**

In `apps/lite-web/src/writings/FinishedWritingPage.tsx`:

1. Imports: `listWritingGradings, type StudentGrading` from `../api/gradings`; `highlightSegments, quoteRanges` from `../shared/gradingText`; `TeacherGradingPanel` from `./TeacherGradingPanel`.
2. State:

```ts
  const [gradings, setGradings] = useState<StudentGrading[]>([]);
  const [gradingsError, setGradingsError] = useState<string | null>(null);
  const [highlight, setHighlight] = useState<{ version: number; quote: string } | null>(null);
```

3. In the load effect (keyed on `writing.id`), reset `setGradings([]); setGradingsError(null); setHighlight(null);` and add:

```ts
    listWritingGradings(writing.id)
      .then((g) => {
        if (alive.current) setGradings(g);
      })
      .catch((e: unknown) => {
        if (alive.current) setGradingsError(apiErrorText(e));
      });
```

4. Version buttons: in the version list `onClick`, also `setHighlight(null);`.
5. `<VersionText version={shown} diff={diff} />` becomes `<VersionText version={shown} diff={diff} highlight={highlight && highlight.version === selected ? highlight.quote : null} />`.
6. In the `<aside>`, after the 与当前版本对比 block:

```tsx
            <TeacherGradingPanel
              gradings={gradings}
              error={gradingsError}
              shownVersion={selected}
              onQuote={(version, quote) => {
                setSelected(version);
                setCompare(false);
                setHighlight({ version, quote });
              }}
            />
```

7. `VersionText` gains the `highlight` prop and, before the paragraph rendering, a branch that keeps paragraph breaks through `whitespace-pre-wrap` and scrolls to the mark:

```tsx
function VersionText({ version, diff, highlight }: { version: WritingVersion | undefined; diff: ParagraphDiff[] | null; highlight: string | null }) {
  const markRef = useRef<HTMLElement | null>(null);
  useEffect(() => {
    markRef.current?.scrollIntoView({ block: "center", behavior: "smooth" });
  }, [highlight, version?.number]);
  if (!version) return <p className="text-mk-body text-mk-muted">加载中…</p>;
  if (!diff && highlight) {
    const segments = highlightSegments(version.body, quoteRanges(version.body, [highlight]));
    return (
      <div className={PIECE_CLS}>
        {segments.map((seg, i) =>
          seg.index === null ? (
            <span key={i}>{seg.text}</span>
          ) : (
            <mark key={i} ref={markRef} style={MARK_STYLE}>
              {seg.text}
            </mark>
          ),
        )}
      </div>
    );
  }
  // …the existing diff and paragraph branches, unchanged
```

and add next to the other style constants:

```ts
const MARK_STYLE: CSSProperties = { background: "color-mix(in srgb, var(--mk-warning) 22%, transparent)", color: "inherit" };
```

Update the component's doc comment: the right rail lists 版本 and 老师批改.

- [ ] **Step 3: Grading rows in the inbox**

In `apps/lite-web/src/inbox/InboxPanel.tsx` (replacing the Task 9 stopgap):

1. Imports: `type AssignmentInboxItem, type GradingInboxItem, type InboxItemDTO` from `../api/assignments`; `markGradingSeen` from `../api/gradings`; `navigate` from `../routing`.
2. `const items = sortUnreadFirst(inbox.items);`
3. `open`:

```ts
  async function open(item: InboxItemDTO) {
    if (openingId) return;
    if (item.type === "grading") {
      await markGradingSeen(item.id).catch(() => undefined);
      inbox.reload();
      navigate(`/writings/${encodeURIComponent(item.atomId)}`);
      return;
    }
    setOpeningId(item.id);
    setStartError(null);
    const err = await openAssignment(item, inbox.reload);
    if (!alive.current) return;
    setOpeningId(null);
    if (err) setStartError(err);
  }
```

4. `<li key={item.id}>` becomes `<li key={`${item.type}:${item.id}`}>`, and its button body branches: grading rows render

```tsx
                  {item.type === "grading" ? (
                    <GradingRowBody item={item} />
                  ) : (
                    /* the existing assignment row markup, with `item` typed AssignmentInboxItem */
                  )}
```

with, at the bottom of the file:

```tsx
function GradingRowBody({ item }: { item: GradingInboxItem }) {
  return (
    <>
      <span className="flex items-center gap-2">
        <span
          className="shrink-0 rounded-mk-full px-2 py-0.5 text-mk-label text-mk-accent-700"
          style={{ background: "color-mix(in srgb, var(--mk-accent-500) 12%, var(--mk-surface))" }}
        >
          批改
        </span>
        {item.unread && (
          <span aria-label="未读" className="h-2 w-2 shrink-0 rounded-mk-full" style={{ background: "var(--mk-danger)", boxShadow: "0 0 0 2px var(--mk-surface)" }} />
        )}
        <span className={`min-w-0 flex-1 truncate text-mk-body text-mk-ink ${item.unread ? "font-semibold" : ""}`}>{item.writingTitle}</span>
      </span>
      <span className="text-mk-small text-mk-muted">发送于 {formatDeadline(item.sentAt)}</span>
    </>
  );
}
```

Move the existing assignment row markup into `function AssignmentRowBody({ item, opening }: { item: AssignmentInboxItem; opening: boolean })` so the branch stays readable; `opening` replaces `openingId === item.id`.

5. The header line 「作业会出现在这里。」 becomes 「作业和老师批改会出现在这里。」

- [ ] **Step 4: Typecheck and tests**

Run: `cd apps/lite-web && pnpm typecheck && pnpm vitest run src/inbox src/writings`
Expected: clean, PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/writings/TeacherGradingPanel.tsx apps/lite-web/src/writings/FinishedWritingPage.tsx \
  apps/lite-web/src/inbox/InboxPanel.tsx
git commit -m "feat(lite-web): students read sent 批改 on the finished page and in the inbox

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 15: Full suites, three live runs, screenshots

**Files:**
- Create (one-off, deleted in Step 7): `apps/lite-web/e2e/grading-shots.spec.ts`
- Output: `.superpowers/tmp/grading-shots/*.png`, `.superpowers/tmp/live-grading-run{1,2,3}.log`

**Interfaces:**
- Consumes: everything above; `apps/lite-web/e2e/run-stack.sh` (throwaway Postgres, API with river, lite dev server).

- [ ] **Step 1: Run every suite this plan touches**

Check `docker ps` first: if another agent's e2e stack is running, wait for it.

Run:
```bash
cd apps/api && CGO_ENABLED=0 go vet ./internal/api/ ./internal/litegrade/ ./internal/liteassign/ && \
  go test ./internal/litegrade ./internal/liteassign -count=1 && \
  CGO_ENABLED=0 go test ./internal/api -count=1 -timeout 1800s 2>&1 | tail -5
cd ../lite-web && pnpm typecheck && pnpm vitest run 2>&1 | tail -5
```
Expected: `ok` for each Go package, vitest all passed. (Do not pipe `go test` through `grep`/`tail` when you need the exit code; read the last lines for `ok`/`FAIL`.)

- [ ] **Step 2: Three live runs, judged on the worst**

```bash
set -a; . .deploy-local/env.prod; set +a
cd apps/api
for i in 1 2 3; do
  LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/api -run TestLiveLiteGrading -v -count=1 -timeout 1200s > ../../.superpowers/tmp/live-grading-run$i.log 2>&1
  tail -3 ../../.superpowers/tmp/live-grading-run$i.log
done
```

Expected: `--- PASS: TestLiveLiteGrading/zh` and `/en` in all three logs. Judge the worst run, not the best:
- Any FAIL blocks shipping. Read the logged reasons. If one reason repeats (for example quotations of the teacher's prompt), fix the prompt in `litegrade/prompt.go` once, add a unit test for the new wording, and rerun all three. If the same failure survives a second prompt change, stop and look for a checkable rule instead of writing a third paragraph (memory `prompt-twice-then-make-it-checkable`).
- `attempts = 2` in more than one of the six subtests means the first reply fails `Check` often; note the reasons in the Part B hand-off even if the runs pass, since every retry doubles the cost.
- Read the logged comments of the worst run as a teacher would: every issue has an action she can do, no action contains a rewritten sentence, the English run's comments are in English.

- [ ] **Step 3: Write the screenshot harness**

Create `apps/lite-web/e2e/grading-shots.spec.ts`:

```ts
import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { expect, test, type APIResponse, type Browser, type Page } from "@playwright/test";

// One-off: screenshots of 评分标准, the 批改 tab, the grading view, a failed
// grading, the teacher item page, the student's 老师批改 panel and the inbox
// row. Not a regression test; deleted after looking at the pictures.
// Gradings are written with psql so the pictures do not depend on a model key.

const PG = process.env.E2E_PG_CONTAINER ?? "mindimprint-lite-e2e-pg";
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:5174";
const JOIN = process.env.E2E_JOIN_CODE ?? "DEMO-0001";
const OUT = "../../.superpowers/tmp/grading-shots";

const BODY = [
  "学校后门那片空地一下雨就积水。去年秋天，我在那里摔过一跤。",
  "我读到城市里的雨水花园：用下凹的绿地先把雨水接住，再慢慢渗到地下。文章引用的监测数据显示，改造后路面积水时间缩短了一半以上。",
  "但雨水花园需要定期清理落叶，否则会堵住。学校有没有人负责这件事，是我下一步要问的问题。",
].join("\n\n");

const CONTENT = {
  overall: { grade: "B+", comment: "用「去年秋天，我在那里摔过一跤。」引出问题，第二段的材料与学校的情况之间还缺一句说明。" },
  dimensions: [
    { name: "内容", grade: "B+", comment: "问题来自亲身经历，材料有数据。" },
    { name: "结构", grade: "B", comment: "第二段到第三段之间没有过渡句。" },
    { name: "语言", grade: "A-", comment: "表达清楚。" },
    { name: "书写规范", grade: "A", comment: "标点使用正确。" },
  ],
  points: [
    { kind: "good", quote: "去年秋天，我在那里摔过一跤。", text: "用具体经历引出问题。", action: null, source: "ai" },
    { kind: "issue", quote: "文章引用的监测数据显示，改造后路面积水时间缩短了一半以上。", text: "数据没有注明来源。", action: "在这句后面写出数据来自哪篇文章、哪一年。", source: "ai" },
    { kind: "issue", quote: "学校有没有人负责这件事，是我下一步要问的问题。", text: "结尾提出了问题，但没有说明打算怎样去问。", action: "补一句你准备问谁、怎么问。", source: "ai" },
  ],
};

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

async function account(browser: Browser, label: string, name: string) {
  const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
  const email = `shot-${label}-${tag}@demo.mindimprint.local`;
  const password = `shot-${tag}-pass`;
  const ctx = await browser.newContext({ baseURL: BASE_URL });
  await ok(ctx.request.post("/api/v1/auth/signup", { data: { email, password, display_name: name, join_code: JOIN } }));
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

test("grading screenshots", async ({ browser }) => {
  mkdirSync(OUT, { recursive: true });
  const teacher = await account(browser, "teacher", "王老师");
  const phoebe = await account(browser, "phoebe", "Phoebe");
  const lin = await account(browser, "lin", "林知遥");

  const teacherId = psql(`SELECT id FROM users WHERE email = '${teacher.email}'`);
  psql(`UPDATE users SET role = 'teacher' WHERE id = '${teacherId}'`);
  psql(`UPDATE enrollments SET role_in_class = 'teacher' WHERE user_id = '${teacherId}'`);
  await ok(teacher.ctx.request.post("/api/v1/auth/signin", { data: { email: teacher.email, password: teacher.password } }));
  const classId = psql(`SELECT class_id FROM enrollments WHERE user_id = '${teacherId}' LIMIT 1`);
  const students = [phoebe, lin].map((s) => ({ ...s, id: psql(`SELECT id FROM users WHERE email = '${s.email}'`) }));

  const t = teacher.ctx.request;
  const tp = await teacher.ctx.newPage();

  // 1. 评分标准 in the new-homework form.
  await tp.setViewportSize({ width: 1440, height: 1000 });
  await tp.goto("/assignments/new");
  await tp.getByRole("button", { name: "写作", exact: true }).click();
  await tp.getByText("评分标准", { exact: true }).waitFor();
  await tp.getByText("评分标准", { exact: true }).scrollIntoViewIfNeeded();
  await shoot(tp, "rubric-form", 1440);

  const created = await ok(
    t.post(`/api/v1/lite/teacher/classes/${classId}/assignments`, {
      data: {
        kind: "writing",
        title: "雨水去哪儿了",
        instructions: "",
        payload: { prompt: "写一篇关于校园积水的议论文", targetWords: 800, lang: "zh" },
        dueAt: new Date(Date.now() + 2 * 86400_000).toISOString(),
        userIds: students.map((s) => s.id),
      },
    }),
  );
  const aid = (await created.json()).assignment.id as string;
  const atoms: string[] = [];
  for (const s of students) {
    const atomId = (await (await ok(s.ctx.request.post(`/api/v1/lite/assignments/${aid}/start`))).json()).atomId as string;
    await ok(s.ctx.request.put(`/api/v1/writings/${atomId}/draft`, { data: { body: BODY } }));
    await ok(s.ctx.request.post(`/api/v1/writings/${atomId}/finish`));
    atoms.push(atomId);
  }
  const rubric = JSON.stringify({ scale: "letter", dimensions: CONTENT.dimensions.map((d) => ({ name: d.name, note: "" })), focus: "" });
  const insert = (i: number, status: string, content: string | null, error: string | null) => {
    const versionId = psql(`SELECT id FROM writing_version WHERE atom_id = '${atoms[i]}' ORDER BY number DESC LIMIT 1`);
    return psql(
      `INSERT INTO lite_grading (atom_id, version_id, user_id, class_id, assignment_id, rubric, status, ai, content, error, requested_by)
       VALUES ('${atoms[i]}', '${versionId}', '${students[i].id}', '${classId}', '${aid}', '${rubric}'::jsonb, '${status}',
               ${content ? `'${content}'::jsonb` : "NULL"}, ${content ? `'${content}'::jsonb` : "NULL"}, ${error ? `'${error}'` : "NULL"}, '${teacherId}')
       RETURNING id`,
    );
  };
  const draftId = insert(0, "draft", JSON.stringify(CONTENT), null);
  const failedId = insert(1, "failed", null, "第 1 条意见的引文不在正文中：「去年冬天，我在那里摔过一跤。」");

  // 2. 批改 tab.
  await tp.goto(`/assignments/${aid}`);
  await tp.getByRole("tab", { name: "批改" }).click();
  await tp.getByText("草稿", { exact: true }).waitFor();
  await shoot(tp, "grading-tab", 1440);

  // 3. Grading view, draft; then picking a quote; then 390px.
  await tp.goto(`/gradings/${draftId}`);
  await tp.getByRole("heading", { name: "意见" }).waitFor();
  await shoot(tp, "grading-view", 1440);
  await tp.getByRole("button", { name: "添加意见", exact: true }).click();
  await tp.getByRole("button", { name: "选择引文", exact: true }).last().click();
  await tp.getByText("请选择一句作为第 4 条意见的引文").waitFor();
  await shoot(tp, "grading-pick-quote", 1440);
  await tp.getByRole("button", { name: "取消选择", exact: true }).click();
  await tp.reload();
  await tp.getByRole("heading", { name: "意见" }).waitFor();
  await shoot(tp, "grading-view", 390);

  // 4. Failed grading.
  await tp.setViewportSize({ width: 1440, height: 1000 });
  await tp.goto(`/gradings/${failedId}`);
  await tp.getByText(/^批改失败：/).waitFor();
  await shoot(tp, "grading-failed", 1440);

  // 5. Teacher item page writing section.
  await tp.goto(`/classes/${classId}/students/${students[0].id}/items/${atoms[0]}`);
  await tp.getByRole("button", { name: "查看批改", exact: true }).waitFor();
  await shoot(tp, "item-page", 1440);

  // 6. Send; the student reads it on the finished page and in the inbox.
  await ok(t.post(`/api/v1/lite/teacher/gradings/${draftId}/send`));
  const sp = await phoebe.ctx.newPage();
  for (const width of [1440, 390]) {
    await sp.setViewportSize({ width, height: 1000 });
    await sp.goto(`/writings/${atoms[0]}`);
    await sp.getByText("由 AI 起草，老师审阅后发送").waitFor();
    await shoot(sp, "student-grading", width);
  }
  await sp.setViewportSize({ width: 1440, height: 1000 });
  await sp.getByRole("button", { name: "「文章引用的监测数据显示，改造后路面积水时间缩短了一半以上。」" }).click();
  await sp.locator("mark").first().waitFor();
  await shoot(sp, "student-quote-highlight", 1440);
  await sp.goto("/writings");
  await sp.getByRole("button", { name: /收件箱/ }).click();
  await sp.getByRole("dialog", { name: "收件箱" }).getByText("批改", { exact: true }).waitFor();
  await shoot(sp, "student-inbox", 1440);
});
```

- [ ] **Step 4: Run it**

```bash
E2E_API_PORT=8091 E2E_WEB_PORT=5184 E2E_PG_PORT=55443 bash apps/lite-web/e2e/run-stack.sh grading-shots.spec.ts
```

Expected: `1 passed`, and in `.superpowers/tmp/grading-shots/`: `1440-rubric-form.png`, `1440-grading-tab.png`, `1440-grading-view.png`, `1440-grading-pick-quote.png`, `390-grading-view.png`, `1440-grading-failed.png`, `1440-item-page.png`, `1440-student-grading.png`, `390-student-grading.png`, `1440-student-quote-highlight.png`, `1440-student-inbox.png`.

If a locator times out, open the trace (`apps/lite-web/test-results/…/trace.zip`, `npx playwright show-trace`) and fix the locator (for example the inbox button's accessible name) before changing any product code.

- [ ] **Step 5: Look at every picture**

Open each PNG with the Read tool and check:
- `rubric-form`: 评分标准 fieldset with 等级/分数, four zh dimensions with 名称/说明, 删除, 添加维度, 批改重点.
- `grading-tab`: tabs 学生 / 批改; buttons 一键AI批改 (disabled, nothing pending), 重试失败 (enabled), 发送全部已审阅 (disabled); rows `v1 · …`, chips 草稿 and 批改失败 with 「批改失败：第 1 条意见的引文不在正文中：…」, overall `B+`.
- `grading-view` 1440: her text on the left with three highlighted quotes; right: 总评 select `B+`, four dimension rows, three point cards (优点/问题, 「引文」, 说明, 修改建议), buttons 保存 / 标记已审阅 / 发送 / 重新批改 / 退回修改.
- `grading-view` 390: one column, no horizontal scroll (asserted), text first.
- `grading-pick-quote`: the left side lists her sentences as buttons and 取消选择.
- `grading-failed`: 「批改失败：…」 and 重新批改; no editor.
- `item-page`: 版本 list, the grading chip and 查看批改; the AI 批注 list shows object points (优点/问题 with quotes), not just summaries.
- `student-grading` 1440/390: 老师批改 card under 版本 with `B+ 总评`, dimension grades, points, and 「由 AI 起草，老师审阅后发送」.
- `student-quote-highlight`: the quoted sentence marked on the left.
- `student-inbox`: a 批改 row with the writing title and 「发送于 …」.
- No coloured left bars; tints use `color-mix`; no Tailwind alpha classes on `mk-*` tokens.

- [ ] **Step 6: Fix what the pictures show**

Fix each defect in the file that owns it (Tasks 11–14), rerun Step 4 and look again. Commit each fix on its own:

```bash
git add <the files you changed>
git commit -m "fix(lite-web): <what the screenshot showed>

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 7: Remove the harness**

```bash
rm apps/lite-web/e2e/grading-shots.spec.ts
git status --short apps/lite-web/e2e
```

Expected: nothing listed for `apps/lite-web/e2e`.

---

## Self-review

**Spec coverage (Part B):**
- B1 rubric shape, letter grades, zh/en defaults, missing rubric = default, editable after start through PATCH, snapshot per grading, 评分标准 UI → Tasks 1, 3 (`rubric` column), 5 (`gradingRubricFor`), 10, 11. Name cap 40 instead of 20 → Deviation 3.
- B2 `lite_grading` columns, status CHECK, `UNIQUE(version_id)`, content shape → Tasks 2, 3 (plus `user_id`, `class_id` → Deviation 2).
- B3 batch queue of latest versions without a row (+ `retryFailed`), single-writing queue, sent never regraded, draft regrade replaces ai+content and clears `reviewed_at` after the 「重新批改会覆盖当前修改」 confirm, river `lite_grading` job with `MaxAttempts: 1`, worker `running → Check → retry with reasons → failed/draft`, `routeE(ctx, gateway.ClassReview)`, `recordLiteLLMCall(…, "teacher_grading")`, `studentEntitled`, prompt from body/title/assigned prompt/rubric/language/symptom list → Tasks 4, 5, 12.
- B4 every rejection reason as a named, tested case (quote substring, 「」 in overall/dimension/text/action, prompt-only text, grade scale, dimension names, 3–5 points, good and issue present, issue action, person-judging); teacher PATCH shape check only; `TestLiveLiteGrading` ×3 on zh and en, judged on the worst run → Tasks 2, 6, 8, 15.
- B5 批改 tab with the seven statuses, 一键AI批改 / 重试失败 / 发送全部已审阅 (confirm with count), 5s polling, grading view (highlights, click-to-scroll, edit/delete/add 意见, pick a sentence as quote, 保存 / 标记已审阅 / 发送 / 重新批改 / 退回修改 via `ReturnDialog`), 「批改失败：{error}」, item page versions + 批改 button + object points bug fix, all five teacher endpoints with ownership checks and other-teacher 404 tests → Tasks 5, 6, 9, 10, 12, 13.
- B6 student endpoint returns only sent rows without `ai` (pinned by `TestStudentSeesOnlySentGradings`), rail panel with grades, comment, points, 「由 AI 起草，老师审阅后发送」, 「针对 v{n}」, quote click shows that version highlighted, inbox grading items + seen endpoint + navigation, re-sent after edits becomes unseen → Tasks 7, 9, 10, 14.
- §5 testing items for Part B: rubric validation and defaults (1), `litegrade.Check` (2), grading state transitions and student visibility (5–7), teacher ownership (5, 6). UI screenshots of the 批改 tab and grading view (15).

**Placeholder scan:** every code step carries its code. Two steps say "keep the existing JSX" (Task 12 Step 5 moving the two 学生 sections, Task 14 Step 3 moving the assignment row into `AssignmentRowBody`); both name the exact blocks to move unchanged. Task 3 Step 4 gives the grep that settles sqlc's generated names.

**Type consistency:** `litegrade.Input` / `Check` / `CheckTeacherEdit` / `NormalizeAI` / `NormalizeTeacher` / `JoinReasons` / `RetryNudge` (Task 2) match their uses in Tasks 4, 6, 8. `gradeWithRetry(ctx, prov, resolved, in, record) (Content, []Reason, int)` and `liteGradingInput(src, rubric)` (Task 4) match Tasks 6 and 8. `LiteGradingArgs{GradingID}` / `LiteGradingWorker{API}` (Task 4) match the Task 5 fixture. `gradingSummaryOf` (Task 5) is used in Task 7. Frontend `GradingRow`, `GradingSummary`, `TeacherGrading`, `StudentGrading`, `Rubric`, `GradingContent` (Task 9) match Tasks 10–14; `gradingRowStatus({version, grading})`, `quoteRanges(text, quotes)`, `highlightSegments(text, ranges)`, `rubricAfterLangChange(d, from, to)`, `gradingVersionLine(gradingVersion, shownVersion)` match between definition and use; `TeacherRoute {view: "grading", gradingId}` matches the shell and `onOpenGrading` props in Tasks 12 and 13.

# Lite Homework Materials (Part C) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A teacher gives reading homework by uploading a document (its text is extracted in the browser and stored as the text source) or by personalised reading (the system picks one library article per student, the teacher reviews and swaps rows, each student starts on her own article).

**Architecture:** No migration and no new SQL: the reading payload is `jsonb`, validated in `liteassign`. Upload reuses `POST /api/v1/documents/extract` (lite-edition gated, not role gated, 30 MB) and adds an optional `fileName` to the `text` source. Personalised reading adds a `personalized` source with `disciplines`, `tier` and `picks`. The per-student choice is one pure function `library.PickForStudent` over the existing `library.Recommend`; the api package builds a `library.Profile` for any user id (refactored out of the student shelf), serves a class preview, validates picks against recipients, starts a pick through the existing library start, and shows each recipient's article on the homework detail.

**Tech Stack:** Go (`net/http`, pgx v5, sqlc v1.27.0 generated code, read only here), PostgreSQL, React + Vite + TypeScript + Tailwind (lite-web), vitest (logic only), Playwright (one-off screenshots).

**Spec:** `docs/superpowers/specs/2026-09-15-lite-homework-grading-and-finished-writing-design.md` (Part C only: C1, C2). Exploration notes: `.superpowers/tmp/explore-homework-grading.md`. Part B plan (runs before this one and edits the same frontend files): `docs/superpowers/plans/2026-09-15-lite-ai-grading.md`.

## Deviations from the spec (forced by the code)

1. **Preview route uses `{id}`, not `{cid}`:** `POST /api/v1/lite/teacher/classes/{id}/personalized-reading/preview`. Every lite teacher class route and `assertTeacherOwnsClass` callers read `r.PathValue("id")`.
2. **Preview response is wrapped:** `{"rows": [...]}`, like every other lite teacher list (`{"assignments": …}`, `{"roster": …}`).
3. **Each preview row also carries `suggestedTier`** (her `SuggestTier`). A pick saved with `tier: null` means "her level at start"; when the teacher swaps a row to 按学生当前水平 the table still needs a tier to display.
4. **Two more reason strings.** The spec's three do not cover (a) a student with interest data whose disciplines match no unopened article in range: 「暂无兴趣相关文章，按难度推荐」 (saying 暂无兴趣数据 would be false); (b) a student who has opened every article: the first article in the filter (else the first overall) with 「暂无未读文章」. A row the teacher swapped shows 「已更换」.
5. **Interest reasons list up to three disciplines** (`Recommendation.Why` is already capped at three), joined with 「、」.
6. **Filtering runs after ranking over the whole library.** `Recommend` is run on all articles (so the rarity discount is the shelf's), and the first result that carries a filtered discipline is taken. Running `Recommend` on the filtered subset would change the weights.
7. **The upload tab stores a `text` source.** `上传文件` is a form tab (`ReadingSource = "file"`), not a payload source; the payload is `{source: "text", text, fileName}`. A stored text source with a non-empty `fileName` reopens on the 上传文件 tab.
8. **Title fill when the extracted title is empty** (`.txt`, many `.docx`): the file name without its extension.
9. **Picks for students who are not recipients are dropped by the form**, not sent. The preview covers every enrolled student; the payload builder keeps only the checked recipients. The server still rejects any pick whose user is not a recipient (`pick_not_recipient`).

## Open questions (ruled here; revisit with the owner if wrong)

1. **Removing a recipient leaves her pick in the stored payload.** It is never used (she cannot start) and stripping it would change the payload, which makes a concurrent start return 409 「老师刚修改了这份作业」. If she is added back, her old pick applies.
2. **Picks lock with the rest of the settings** once any recipient starts (existing rule). A teacher cannot swap a not-yet-started student's article after another student started.
3. **Changing 学科筛选 or 难度 re-runs the preview** and keeps the rows the teacher swapped. Saved picks reopened in the detail edit count as swapped when they differ from the new preview.
4. **Detail column for a recipient with no pick who has not started** shows 「待推荐」 instead of computing a recommendation that may change by the time she starts.
5. **Discipline options in the form** are the tags that appear on library articles (from `GET /api/v1/library`), so no filter can select a discipline with zero articles.

## Global Constraints

- Edition: lite only. Pro behaviour must not change. This plan adds no migration, no query and no sqlc regeneration. If an executor finds a migration is needed after all, take the next free number after Part B's `0154` (so `0155`) and add new files only.
- Before Task 1: `git fetch && git rebase origin/main` is not this plan's job, but Part B must be merged into the branch first. Read the current `AssignmentForm.tsx`, `AssignmentDetailPage.tsx`, `assignmentLogic.ts`, `lite_teacher_assignments.go` and `liteassign/payload.go` before editing: Part B added `rubric` to the writing payload, a `rubric` PATCH field, `RubricFields` in `SettingsFields` and tabs on the detail page. Keep all of it.
- Reading text limit: 50000 runes (server `maxTextRunes`). File name ≤ 200 runes. Tiers 1..5. Supported files `.pdf .docx .txt .md`, ≤ 30 MB (`liteSourceFileMaxBytes`).
- `personalized` payload: `{source: "personalized", disciplines?: string[] (ids from internal/disciplines), tier?: 1..5 (absent = each student's SuggestTier), picks: {[userId]: {slug: string, tier: 1..5 | null}}}`.
- Copy, verbatim from the spec: tab 上传文件; 「已提取 {n} 字」; 「提取失败：{后台原话}」; 「提取失败：正文超过 50000 字」; tab 个性化; button 更换; reasons 「兴趣相关：{学科1}、{学科2}」, 「暂无兴趣数据，按难度推荐」, 「筛选范围内暂无合适文章」. Added copy: 「暂无兴趣相关文章，按难度推荐」, 「暂无未读文章」, 「已更换」, 「待推荐」, 「请上传文件」, 「请等待推荐列表加载完成」, labels 学科筛选 / 难度 / 学生 / 文章 / 原因 / 操作, buttons 确认选择 / 取消. Upload hint: 「支持 PDF、Word（.docx）、TXT、Markdown，不超过 30 MB。PDF 中的图片不会保留；扫描版 PDF 没有文字，无法提取。」 All other copy follows AGENTS.md §界面文案怎么写 (labels are nouns, errors `动词+失败：{后台原话}`, no literary prose; rule 10 applies to code comments and commit messages).
- Model calls: none. Reasons are built in code.
- Every new teacher route checks the teacher owns the class (404 otherwise), with an other-teacher 404 test.
- Tests: logic tests only. Go handler tests use the testcontainers harness (`newAPITestPool`, `signInAs`, `assignJSON`, `getJSON`, `writeErrorCode`, `createAssignment`, `startAssignment`, `readingAssignmentBody`, `liteTeacherFixture`, `createStudent`, `createTeacher`, `enrollStudent`). Frontend: vitest for pure helpers and normalizers only; UI is checked with a one-off Playwright harness, screenshots under `.superpowers/tmp/materials-shots/`, harness deleted afterwards.
- Pure Go test command: `cd apps/api && go test ./internal/<pkg> -run <pattern> -count=1`. DB test command: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run <pattern> -count=1 -timeout 1800s`. Check `docker ps` first; do not run DB tests while another agent's e2e stack is up.
- Frontend commands: `cd apps/lite-web && pnpm vitest run <file>` and `pnpm typecheck`.
- Commits: stage specific paths, never `git add -A`. Every commit message ends with `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

## File map

Backend (`apps/api/internal/…`):
- `liteassign/payload.go` (+ `payload_test.go`) — `fileName`, `personalized` source, `ValidateDisciplines`, `PicksOutside`.
- `library/pick.go` (+ `pick_test.go`) — `PickForStudent`, `StudentPick.Reason`.
- `api/library.go` — `interestDisciplinesIn`, `libraryProfileIn` replace `interestDisciplines(r)` and `mustUser`.
- `api/lite_personalized_reading.go` — preview handler, `personalizedTargetIn`, `personalizedRecipientReadings`.
- `api/lite_teacher_routes.go` — preview route.
- `api/lite_teacher_assignments.go` — pick recipient checks on create and PATCH; `RecipientDTO.Reading`.
- `api/lite_student_assignments.go` — start branch for `personalized`.
- Tests: `api/lite_personalized_reading_test.go` (new).

Frontend (`apps/lite-web/src/…`):
- `api/assignments.ts` (+ `assignments.test.ts`) — payload types, `RecipientReading`, `PreviewRow`, `previewPersonalizedReading`, normalizers.
- `teacher/assignmentLogic.ts` (+ `assignmentLogic.test.ts`) — extract outcome, title fill, pick rows, payload builder, summaries.
- `teacher/UploadSourceField.tsx` (new), `teacher/PersonalizedPicker.tsx` (new), `teacher/AssignmentForm.tsx`, `teacher/AssignmentDetailPage.tsx`, `teacher/AssignmentsPage.tsx`.
- `e2e/materials-shots.spec.ts` — one-off, deleted in Task 9.

---

### Task 1: Reading payload — `fileName` and the `personalized` source

**Files:**
- Modify: `apps/api/internal/liteassign/payload.go`
- Test: `apps/api/internal/liteassign/payload_test.go`

**Interfaces:**
- Consumes: `library.BySlug(slug) (library.Article, bool)`, `disciplines.ByID(id) (disciplines.Discipline, bool)`, `github.com/google/uuid`.
- Produces:
  - `type PersonalPick struct { Slug string \`json:"slug"\`; Tier *int \`json:"tier"\` }`
  - `ReadingPayload` gains `FileName string \`json:"fileName,omitempty"\``, `Disciplines []string \`json:"disciplines,omitempty"\``, `Picks map[string]PersonalPick \`json:"picks,omitempty"\``. Pick keys are canonical `uuid.UUID.String()`.
  - `func ValidateDisciplines(ids []string) ([]string, error)` — trims, drops blanks, de-duplicates in order; unknown id → `PayloadError{Code: "invalid_discipline"}`.
  - `func PicksOutside(payload json.RawMessage, recipients []uuid.UUID) bool` — true when a `personalized` payload has a pick whose user is not in `recipients`; false for every other payload.
  - Error codes: `file_name_too_long`, `invalid_discipline`, `invalid_pick_user`, `invalid_pick_slug`, `invalid_tier`, `invalid_source`.

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/liteassign/payload_test.go` (add `"mindimprint/api/internal/library"` and `"github.com/google/uuid"` to the imports):

```go
func TestReadingTextFileName(t *testing.T) {
	out, err := ValidatePayload("reading", json.RawMessage(`{"source":"text","text":"正文","fileName":"  雨水花园.pdf  "}`))
	if err != nil {
		t.Fatal(err)
	}
	var p ReadingPayload
	_ = json.Unmarshal(out, &p)
	if p.FileName != "雨水花园.pdf" {
		t.Fatalf("fileName = %q, want trimmed", p.FileName)
	}
	// 200 runes pass, 201 fail: the cap counts runes, not bytes.
	ok200 := `{"source":"text","text":"x","fileName":"` + strings.Repeat("雨", 200) + `"}`
	if _, err := ValidatePayload("reading", json.RawMessage(ok200)); err != nil {
		t.Fatalf("200-rune file name: %v", err)
	}
	bad := `{"source":"text","text":"x","fileName":"` + strings.Repeat("雨", 201) + `"}`
	var pe *PayloadError
	if _, err := ValidatePayload("reading", json.RawMessage(bad)); !errors.As(err, &pe) || pe.Code != "file_name_too_long" {
		t.Fatalf("201-rune file name: got %v", err)
	}
	// A file name on another source is dropped.
	out, _ = ValidatePayload("reading", json.RawMessage(`{"source":"url","url":"https://example.com","fileName":"a.pdf"}`))
	if strings.Contains(string(out), "fileName") {
		t.Fatalf("url payload kept fileName: %s", out)
	}
}

func TestReadingPersonalized(t *testing.T) {
	art := library.All()[0]
	uid := uuid.New()
	raw := `{"source":"personalized","disciplines":[" ` + art.Disciplines[0] + `","` + art.Disciplines[0] + `",""],"tier":3,` +
		`"picks":{"` + strings.ToUpper(uid.String()) + `":{"slug":" ` + art.Slug + ` ","tier":null}},"slug":"x","text":"y"}`
	out, err := ValidatePayload("reading", json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	var p ReadingPayload
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	if p.Source != "personalized" || p.Slug != "" || p.Text != "" || p.Tier == nil || *p.Tier != 3 {
		t.Fatalf("payload = %+v", p)
	}
	if len(p.Disciplines) != 1 || p.Disciplines[0] != art.Disciplines[0] {
		t.Fatalf("disciplines = %v, want trimmed and de-duplicated", p.Disciplines)
	}
	pick, ok := p.Picks[uid.String()]
	if !ok || pick.Slug != art.Slug || pick.Tier != nil {
		t.Fatalf("picks = %+v, want canonical key %s", p.Picks, uid)
	}
	if !strings.Contains(string(out), `"tier":null`) {
		t.Fatalf("a pick with no tier must serialise tier:null: %s", out)
	}

	// No picks and no filter is valid: every student is recommended at start.
	if _, err := ValidatePayload("reading", json.RawMessage(`{"source":"personalized"}`)); err != nil {
		t.Fatalf("bare personalized: %v", err)
	}

	bad := []struct{ raw, code string }{
		{`{"source":"personalized","disciplines":["no-such-discipline"]}`, "invalid_discipline"},
		{`{"source":"personalized","tier":6}`, "invalid_tier"},
		{`{"source":"personalized","picks":{"not-a-uuid":{"slug":"` + art.Slug + `"}}}`, "invalid_pick_user"},
		{`{"source":"personalized","picks":{"` + uid.String() + `":{"slug":"no-such-article"}}}`, "invalid_pick_slug"},
		{`{"source":"personalized","picks":{"` + uid.String() + `":{"slug":"` + art.Slug + `","tier":0}}}`, "invalid_tier"},
	}
	for _, c := range bad {
		_, err := ValidatePayload("reading", json.RawMessage(c.raw))
		var pe *PayloadError
		if !errors.As(err, &pe) || pe.Code != c.code {
			t.Errorf("%s: got %v, want %s", c.raw, err, c.code)
		}
	}
}

func TestPicksOutside(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	slug := library.All()[0].Slug
	payload, err := ValidatePayload("reading", json.RawMessage(
		`{"source":"personalized","picks":{"`+a.String()+`":{"slug":"`+slug+`"},"`+b.String()+`":{"slug":"`+slug+`"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if PicksOutside(payload, []uuid.UUID{a, b, c}) {
		t.Fatal("every pick user is a recipient, want false")
	}
	if !PicksOutside(payload, []uuid.UUID{a, c}) {
		t.Fatal("b has a pick but is not a recipient, want true")
	}
	if PicksOutside(json.RawMessage(`{"source":"text","text":"x"}`), nil) {
		t.Fatal("a text payload has no picks, want false")
	}
}
```

Also change the existing `{"reading", `{"source":"pdf"}`, "invalid_source"}` row: it stays as is (still invalid).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd apps/api && go test ./internal/liteassign -run 'TestReadingTextFileName|TestReadingPersonalized|TestPicksOutside' -count=1`
Expected: FAIL to compile (`undefined: PicksOutside`, `p.FileName undefined`).

- [ ] **Step 3: Implement**

In `apps/api/internal/liteassign/payload.go`, add the imports `"github.com/google/uuid"`, `"mindimprint/api/internal/disciplines"`, `"mindimprint/api/internal/library"`, then replace the `ReadingPayload` struct and add the new types and helpers:

```go
type ReadingPayload struct {
	Source string `json:"source"`
	Slug   string `json:"slug,omitempty"`
	Tier   *int   `json:"tier,omitempty"`
	URL    string `json:"url,omitempty"`
	Text   string `json:"text,omitempty"`
	// FileName is set when the teacher uploaded a document for a text source.
	FileName string `json:"fileName,omitempty"`
	// Disciplines and Picks belong to the personalized source. Picks is keyed
	// by the student's user id; a student with no pick is recommended at start.
	Disciplines []string                `json:"disciplines,omitempty"`
	Picks       map[string]PersonalPick `json:"picks,omitempty"`
}

// PersonalPick is the article the teacher confirmed for one student. A nil
// Tier means the student's suggested tier when she starts.
type PersonalPick struct {
	Slug string `json:"slug"`
	Tier *int   `json:"tier"`
}

const maxFileNameRunes = 200

func validTier(t *int) bool { return t == nil || (*t >= 1 && *t <= 5) }

// ValidateDisciplines trims, drops blanks and duplicates, and refuses an id
// that is not in the discipline table.
func ValidateDisciplines(ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		if _, ok := disciplines.ByID(id); !ok {
			return nil, perr("invalid_discipline", "学科不在学科表中："+id)
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

// PicksOutside reports whether a personalized payload picks an article for a
// user who is not one of recipients. Other payloads have no picks.
func PicksOutside(payload json.RawMessage, recipients []uuid.UUID) bool {
	var p ReadingPayload
	if json.Unmarshal(payload, &p) != nil || p.Source != "personalized" {
		return false
	}
	in := make(map[string]bool, len(recipients))
	for _, id := range recipients {
		in[id.String()] = true
	}
	for uid := range p.Picks {
		if !in[uid] {
			return true
		}
	}
	return false
}
```

Replace the `case "reading":` body of `ValidatePayload` with:

```go
	case "reading":
		var p ReadingPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, perr("invalid_payload", "作业设置格式错误")
		}
		p.Slug, p.URL, p.Text = strings.TrimSpace(p.Slug), strings.TrimSpace(p.URL), strings.TrimSpace(p.Text)
		p.FileName = strings.TrimSpace(p.FileName)
		switch p.Source {
		case "library":
			if p.Slug == "" {
				return nil, perr("invalid_slug", "请选择一篇文章")
			}
			if !validTier(p.Tier) {
				return nil, perr("invalid_tier", "难度档位需在 1 到 5 之间")
			}
			p.URL, p.Text, p.FileName, p.Disciplines, p.Picks = "", "", "", nil, nil
		case "url":
			if !isHTTPURL(p.URL) {
				return nil, perr("invalid_url", "请输入以 http 或 https 开头的链接")
			}
			p.Slug, p.Tier, p.Text, p.FileName, p.Disciplines, p.Picks = "", nil, "", "", nil, nil
		case "text":
			if p.Text == "" {
				return nil, perr("empty_text", "请粘贴文章正文")
			}
			if utf8.RuneCountInString(p.Text) > maxTextRunes {
				return nil, perr("text_too_long", "文章正文不能超过 50000 字")
			}
			if utf8.RuneCountInString(p.FileName) > maxFileNameRunes {
				return nil, perr("file_name_too_long", "文件名不能超过 200 字")
			}
			p.Slug, p.Tier, p.URL, p.Disciplines, p.Picks = "", nil, "", nil, nil
		case "personalized":
			if !validTier(p.Tier) {
				return nil, perr("invalid_tier", "难度档位需在 1 到 5 之间")
			}
			ds, err := ValidateDisciplines(p.Disciplines)
			if err != nil {
				return nil, err
			}
			p.Disciplines = ds
			picks := make(map[string]PersonalPick, len(p.Picks))
			for key, pick := range p.Picks {
				uid, err := uuid.Parse(strings.TrimSpace(key))
				if err != nil {
					return nil, perr("invalid_pick_user", "个性化名单中的学生无效")
				}
				pick.Slug = strings.TrimSpace(pick.Slug)
				if _, ok := library.BySlug(pick.Slug); !ok {
					return nil, perr("invalid_pick_slug", "文章不在阅读库里："+pick.Slug)
				}
				if !validTier(pick.Tier) {
					return nil, perr("invalid_tier", "难度档位需在 1 到 5 之间")
				}
				picks[uid.String()] = pick
			}
			p.Picks = picks
			p.Slug, p.URL, p.Text, p.FileName = "", "", "", ""
		default:
			return nil, perr("invalid_source", "阅读来源只能是分级阅读库、链接、正文或个性化阅读")
		}
		return json.Marshal(p)
```

Every library article has all five tiers (`library.validate` refuses anything else at load), so a tier in 1..5 always exists for a known slug.

- [ ] **Step 4: Run the package tests**

Run: `cd apps/api && go test ./internal/liteassign -count=1`
Expected: `ok  	mindimprint/api/internal/liteassign`. `go vet ./internal/liteassign` also clean (no import cycle: `library` and `disciplines` import neither `liteassign` nor `api`; confirm with `go list -deps ./internal/library | grep liteassign` printing nothing).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/liteassign/payload.go apps/api/internal/liteassign/payload_test.go
git commit -m "feat(lite): fileName on text readings and a personalized reading source

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `library.PickForStudent` — one article per student

**Files:**
- Create: `apps/api/internal/library/pick.go`
- Test: `apps/api/internal/library/pick_test.go`

**Interfaces:**
- Consumes: `library.Recommend(articles []Article, p Profile, limit int) []Recommendation`, `library.Profile{Disciplines, ReadSlugs, Tier}`, `disciplines.ByID`.
- Produces:
  - `type StudentPick struct { Article Article; Tier int; Why []string; HasInterest, OutsideFilter, AllRead bool }`
  - `func PickForStudent(articles []Article, p Profile, filter []string) (StudentPick, bool)` — `false` only when `articles` is empty.
  - `func (s StudentPick) Reason() string`

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/library/pick_test.go`:

```go
package library

import (
	"testing"

	"mindimprint/api/internal/disciplines"
)

// Synthetic library, in library order. Ids are real discipline ids so Reason
// can look up their Chinese names.
func pickArticles() []Article {
	return []Article{
		{Slug: "hist-1", Disciplines: []string{"history"}},
		{Slug: "astro-1", Disciplines: []string{"astronomy"}},
		{Slug: "astro-2", Disciplines: []string{"astronomy", "history"}},
		{Slug: "prob-1", Disciplines: []string{"probability"}},
	}
}

func zh(t *testing.T, id string) string {
	t.Helper()
	d, ok := disciplines.ByID(id)
	if !ok {
		t.Fatalf("discipline %q missing from the table", id)
	}
	return d.Zh
}

func TestPickForStudentInterest(t *testing.T) {
	p := Profile{Disciplines: map[string]float64{"astronomy": 4}, Tier: 3}
	got, ok := PickForStudent(pickArticles(), p, nil)
	if !ok || got.Article.Slug != "astro-1" || got.Tier != 3 {
		t.Fatalf("pick = %+v, want astro-1 at tier 3", got)
	}
	if want := "兴趣相关：" + zh(t, "astronomy"); got.Reason() != want {
		t.Fatalf("reason = %q, want %q", got.Reason(), want)
	}
}

func TestPickForStudentExcludesOpened(t *testing.T) {
	p := Profile{Disciplines: map[string]float64{"astronomy": 4}, ReadSlugs: map[string]bool{"astro-1": true}, Tier: 2}
	got, _ := PickForStudent(pickArticles(), p, nil)
	if got.Article.Slug != "astro-2" {
		t.Fatalf("pick = %s, want astro-2 (astro-1 already opened)", got.Article.Slug)
	}
}

func TestPickForStudentFilter(t *testing.T) {
	// Interest in astronomy, filter history: astro-2 carries both and ranks first.
	p := Profile{Disciplines: map[string]float64{"astronomy": 4}, Tier: 2}
	got, _ := PickForStudent(pickArticles(), p, []string{"history"})
	if got.Article.Slug != "astro-2" || got.OutsideFilter {
		t.Fatalf("pick = %+v, want astro-2 inside the filter", got)
	}
	// Interest in probability, filter history: nothing in range matches her
	// interest, so the ranked history article is taken with the plain reason.
	p = Profile{Disciplines: map[string]float64{"probability": 4}, Tier: 2}
	got, _ = PickForStudent(pickArticles(), p, []string{"history"})
	if got.Article.Slug != "hist-1" || got.Reason() != "暂无兴趣相关文章，按难度推荐" {
		t.Fatalf("pick = %s reason %q", got.Article.Slug, got.Reason())
	}
}

func TestPickForStudentFilterExhausted(t *testing.T) {
	// Every history article is opened: first unopened article overall, in library order.
	p := Profile{ReadSlugs: map[string]bool{"hist-1": true, "astro-2": true}, Tier: 2}
	got, _ := PickForStudent(pickArticles(), p, []string{"history"})
	if got.Article.Slug != "astro-1" || !got.OutsideFilter || len(got.Why) != 0 {
		t.Fatalf("pick = %+v, want astro-1 outside the filter", got)
	}
	if got.Reason() != "筛选范围内暂无合适文章" {
		t.Fatalf("reason = %q", got.Reason())
	}
}

func TestPickForStudentNoInterestData(t *testing.T) {
	got, _ := PickForStudent(pickArticles(), Profile{Tier: 2}, nil)
	if got.Article.Slug != "hist-1" || got.Reason() != "暂无兴趣数据，按难度推荐" {
		t.Fatalf("pick = %s reason %q", got.Article.Slug, got.Reason())
	}
	// Zero strengths are not interest data.
	got, _ = PickForStudent(pickArticles(), Profile{Disciplines: map[string]float64{"history": 0}, Tier: 2}, nil)
	if got.Reason() != "暂无兴趣数据，按难度推荐" {
		t.Fatalf("zero-strength reason = %q", got.Reason())
	}
}

func TestPickForStudentAllRead(t *testing.T) {
	read := map[string]bool{"hist-1": true, "astro-1": true, "astro-2": true, "prob-1": true}
	got, ok := PickForStudent(pickArticles(), Profile{ReadSlugs: read, Tier: 2}, []string{"probability"})
	if !ok || !got.AllRead || got.Article.Slug != "prob-1" || got.Reason() != "暂无未读文章" {
		t.Fatalf("filtered all-read pick = %+v reason %q", got, got.Reason())
	}
	got, _ = PickForStudent(pickArticles(), Profile{ReadSlugs: read, Tier: 2}, nil)
	if got.Article.Slug != "hist-1" {
		t.Fatalf("unfiltered all-read pick = %s, want the first article", got.Article.Slug)
	}
}

func TestPickForStudentTier(t *testing.T) {
	for _, c := range []struct{ in, want int }{{0, 2}, {6, 2}, {1, 1}, {5, 5}} {
		got, _ := PickForStudent(pickArticles(), Profile{Tier: c.in}, nil)
		if got.Tier != c.want {
			t.Errorf("Profile.Tier %d → pick tier %d, want %d", c.in, got.Tier, c.want)
		}
	}
	if _, ok := PickForStudent(nil, Profile{Tier: 2}, nil); ok {
		t.Error("an empty library must return ok=false")
	}
}

func TestPickForStudentReasonListsUpToThree(t *testing.T) {
	s := StudentPick{Why: []string{"astronomy", "history"}, HasInterest: true}
	if want := "兴趣相关：" + zh(t, "astronomy") + "、" + zh(t, "history"); s.Reason() != want {
		t.Fatalf("reason = %q, want %q", s.Reason(), want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd apps/api && go test ./internal/library -run TestPickForStudent -count=1`
Expected: FAIL to compile (`undefined: PickForStudent`, `undefined: StudentPick`).

- [ ] **Step 3: Implement**

Create `apps/api/internal/library/pick.go`:

```go
package library

import (
	"strings"

	"mindimprint/api/internal/disciplines"
)

// StudentPick is the one article a personalised reading homework gives a
// student, with what the teacher's preview says about it.
type StudentPick struct {
	Article Article
	Tier    int
	// Why is the matched discipline ids, at most three (from Recommend).
	Why []string
	// HasInterest: the profile carries at least one positive discipline strength.
	HasInterest bool
	// OutsideFilter: no unopened article carries a filtered discipline, so
	// the first unopened article overall was taken.
	OutsideFilter bool
	// AllRead: she has opened every article; the first article in the
	// filter (or the first overall) was taken.
	AllRead bool
}

// PickForStudent runs Recommend over the whole library (so the rarity
// discount matches the student shelf) and takes the first result that carries
// a filtered discipline. An empty filter takes the first result. ok is false
// only when articles is empty.
func PickForStudent(articles []Article, p Profile, filter []string) (StudentPick, bool) {
	if len(articles) == 0 {
		return StudentPick{}, false
	}
	pick := StudentPick{Tier: p.Tier}
	if pick.Tier < 1 || pick.Tier > 5 {
		pick.Tier = 2
	}
	for _, v := range p.Disciplines {
		if v > 0 {
			pick.HasInterest = true
			break
		}
	}
	want := make(map[string]bool, len(filter))
	for _, id := range filter {
		want[id] = true
	}
	inFilter := func(a Article) bool {
		if len(want) == 0 {
			return true
		}
		for _, id := range a.Disciplines {
			if want[id] {
				return true
			}
		}
		return false
	}

	recs := Recommend(articles, p, 0)
	for _, rec := range recs {
		if inFilter(rec.Article) {
			pick.Article, pick.Why = rec.Article, rec.Why
			return pick, true
		}
	}
	if len(recs) > 0 {
		// Unopened articles exist, none in the filter.
		for _, a := range articles {
			if !p.ReadSlugs[a.Slug] {
				pick.Article, pick.OutsideFilter = a, true
				return pick, true
			}
		}
	}
	pick.AllRead = true
	pick.Article = articles[0]
	for _, a := range articles {
		if inFilter(a) {
			pick.Article = a
			break
		}
	}
	return pick, true
}

// Reason is the line the teacher reads next to the pick. Built in code; no model.
func (s StudentPick) Reason() string {
	if s.AllRead {
		return "暂无未读文章"
	}
	if s.OutsideFilter {
		return "筛选范围内暂无合适文章"
	}
	names := make([]string, 0, len(s.Why))
	for _, id := range s.Why {
		if d, ok := disciplines.ByID(id); ok {
			names = append(names, d.Zh)
		}
	}
	if len(names) > 0 {
		return "兴趣相关：" + strings.Join(names, "、")
	}
	if s.HasInterest {
		return "暂无兴趣相关文章，按难度推荐"
	}
	return "暂无兴趣数据，按难度推荐"
}
```

- [ ] **Step 4: Run the package tests**

Run: `cd apps/api && go test ./internal/library -count=1`
Expected: `ok  	mindimprint/api/internal/library` (the existing `TestRecommend*` and data tests still pass).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/library/pick.go apps/api/internal/library/pick_test.go
git commit -m "feat(lite): pick one library article per student for personalised reading

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: A library profile for any student, and the class preview

**Files:**
- Modify: `apps/api/internal/api/library.go:88-205` (`getLibraryShelf`, `interestDisciplines`, `mustUser`)
- Create: `apps/api/internal/api/lite_personalized_reading.go`
- Modify: `apps/api/internal/api/lite_teacher_routes.go` (one route)
- Test: `apps/api/internal/api/lite_personalized_reading_test.go` (new)

**Interfaces:**
- Consumes: `library.PickForStudent`, `StudentPick.Reason` (Task 2); `liteassign.ValidateDisciplines` (Task 1); `suggestLibraryTierFromRows(rows []sqlc.ListLibraryReadingsByUserRow) int` (`lite_create_helpers.go`); `a.d.Queries.ListLiteWeekClassStudents(ctx, classID uuid.UUID) ([]sqlc.ListLiteWeekClassStudentsRow{ID, DisplayName}, error)`; `ListInterestKeywords`, `ListKeywordDisciplinesForUser`, `ListLibraryReadingsByUser`.
- Produces:
  - `func interestDisciplinesIn(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (map[string]float64, error)`
  - `func libraryProfileIn(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (library.Profile, error)` — `Tier` is her `SuggestTier`, `ReadSlugs` every article she has opened. `q` may be a transaction's queries.
  - `POST /api/v1/lite/teacher/classes/{id}/personalized-reading/preview` body `{disciplines?: string[], tier?: 1..5}` → `200 {"rows": [{userId, name, slug, title, tier, suggestedTier, reason}]}`, one row per enrolled student, ordered by display name.
  - Test helper `seedInterestFor(t, pool, userID uuid.UUID, disciplineID string)` (strength 4, confidence 1.0).

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/api/lite_personalized_reading_test.go`:

```go
package api_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/library"
)

// seedInterestFor writes one keyword routed to disciplineID for userID, at
// strength 4 and confidence 1.0, so her profile reads {disciplineID: 4}.
func seedInterestFor(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, disciplineID string) {
	t.Helper()
	d, ok := disciplines.ByID(disciplineID)
	if !ok {
		t.Fatalf("unknown discipline %q", disciplineID)
	}
	ctx := context.Background()
	var kid uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO interest_keyword (user_id, text_zh, text_en, norm, field, strength, note)
		VALUES ($1, $2, $2, $2, $3, 4, '') RETURNING id`, userID, disciplineID, d.Field).Scan(&kid); err != nil {
		t.Fatalf("seed keyword: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO keyword_discipline (keyword_id, discipline_id, confidence, how, rationale)
		VALUES ($1, $2, 1.0, 'alias', '')`, kid, disciplineID); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
}

type previewRow struct {
	UserID        string `json:"userId"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	Tier          int    `json:"tier"`
	SuggestedTier int    `json:"suggestedTier"`
	Reason        string `json:"reason"`
}

func postPreview(t *testing.T, h http.Handler, c *http.Cookie, classID string, body map[string]any) map[string]previewRow {
	t.Helper()
	var out struct {
		Rows []previewRow `json:"rows"`
	}
	if code := assignJSON(t, h, c, "POST", "/api/v1/lite/teacher/classes/"+classID+"/personalized-reading/preview", body, &out); code != http.StatusOK {
		t.Fatalf("preview = %d", code)
	}
	byUser := make(map[string]previewRow, len(out.Rows))
	for _, r := range out.Rows {
		byUser[r.UserID] = r
	}
	return byUser
}

// TestLibraryShelfStillRecommendsFromHerInterests pins the shelf through the
// profile refactor: a student with interest in one discipline gets it as the
// first recommendation's reason.
func TestLibraryShelfStillRecommendsFromHerInterests(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	d := library.All()[len(library.All())-1].Disciplines[0]
	seedInterestFor(t, pool, studentID, d)
	var shelf struct {
		Recommended []struct {
			Why []string `json:"why"`
		} `json:"recommended"`
		Tier int `json:"tier"`
	}
	if code := getJSON(t, h, signInAs(t, pool, studentID), "/api/v1/library", &shelf); code != http.StatusOK {
		t.Fatalf("shelf = %d", code)
	}
	want, _ := disciplines.ByID(d)
	if len(shelf.Recommended) == 0 || len(shelf.Recommended[0].Why) == 0 || shelf.Recommended[0].Why[0] != want.Zh || shelf.Tier != 2 {
		t.Fatalf("shelf = %+v, want first recommendation for %s at tier 2", shelf, want.Zh)
	}
}

func TestPersonalizedPreview(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	s2 := createStudent(t, pool, SeedSchoolID, "pp-s2@demo.local")
	enrollStudent(t, pool, s2, classID)
	all := library.All()

	// s1 opens the first article at tier 4 and leaves it: excluded, suggested tier 3.
	if code := assignJSON(t, h, signInAs(t, pool, s1), "POST", "/api/v1/library/"+all[0].Slug+"/levels/"+strconv.Itoa(4), nil, nil); code != http.StatusCreated {
		t.Fatalf("s1 opens an article = %d", code)
	}
	d := all[len(all)-1].Disciplines[0]
	seedInterestFor(t, pool, s2, d)
	p1 := library.Profile{ReadSlugs: map[string]bool{all[0].Slug: true}, Tier: 3}
	p2 := library.Profile{Disciplines: map[string]float64{d: 4}, ReadSlugs: map[string]bool{}, Tier: 2}

	rows := postPreview(t, h, teacher, classID, map[string]any{})
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want one per enrolled student", rows)
	}
	want1, _ := library.PickForStudent(all, p1, nil)
	r1 := rows[s1.String()]
	if r1.Slug == all[0].Slug || r1.Slug != want1.Article.Slug || r1.Title != want1.Article.ZhTitle ||
		r1.Tier != 3 || r1.SuggestedTier != 3 || r1.Reason != "暂无兴趣数据，按难度推荐" || r1.Name == "" {
		t.Fatalf("s1 row = %+v, want %s at tier 3", r1, want1.Article.Slug)
	}
	zh, _ := disciplines.ByID(d)
	r2 := rows[s2.String()]
	want2, _ := library.PickForStudent(all, p2, nil)
	if r2.Slug != want2.Article.Slug || r2.Reason != "兴趣相关："+zh.Zh || r2.Tier != 2 {
		t.Fatalf("s2 row = %+v, want %s for her interest", r2, want2.Article.Slug)
	}

	// A tier and a filter: tier overrides, suggestedTier stays hers.
	filter := []string{all[0].Disciplines[0]}
	rows = postPreview(t, h, teacher, classID, map[string]any{"tier": 5, "disciplines": filter})
	p1.Tier, p2.Tier = 5, 5
	want1, _ = library.PickForStudent(all, p1, filter)
	want2, _ = library.PickForStudent(all, p2, filter)
	if r := rows[s1.String()]; r.Slug != want1.Article.Slug || r.Reason != want1.Reason() || r.Tier != 5 || r.SuggestedTier != 3 {
		t.Fatalf("filtered s1 row = %+v, want %s %q", r, want1.Article.Slug, want1.Reason())
	}
	if r := rows[s2.String()]; r.Slug != want2.Article.Slug || r.Reason != want2.Reason() || r.SuggestedTier != 2 {
		t.Fatalf("filtered s2 row = %+v, want %s %q", r, want2.Article.Slug, want2.Reason())
	}

	path := "/api/v1/lite/teacher/classes/" + classID + "/personalized-reading/preview"
	if code, errCode := writeErrorCode(t, h, teacher, "POST", path, map[string]any{"disciplines": []string{"no-such"}}); code != http.StatusBadRequest || errCode != "invalid_discipline" {
		t.Fatalf("bad discipline = %d %s", code, errCode)
	}
	if code, errCode := writeErrorCode(t, h, teacher, "POST", path, map[string]any{"tier": 9}); code != http.StatusBadRequest || errCode != "invalid_tier" {
		t.Fatalf("bad tier = %d %s", code, errCode)
	}
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pp-other@demo.local"))
	if code := assignJSON(t, h, other, "POST", path, map[string]any{}, nil); code != http.StatusNotFound {
		t.Fatalf("other teacher preview = %d, want 404", code)
	}
	if code := assignJSON(t, h, signInAs(t, pool, s1), "POST", path, map[string]any{}, nil); code == http.StatusOK {
		t.Fatalf("student preview = %d, want refused", code)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestLibraryShelfStillRecommendsFromHerInterests|TestPersonalizedPreview' -count=1 -timeout 1800s`
Expected: the shelf test PASSES (it pins today's behaviour); `TestPersonalizedPreview` FAILS with `preview = 404` (or 405) because the route does not exist.

- [ ] **Step 3: Refactor the profile out of the shelf**

In `apps/api/internal/api/library.go` add imports `"context"` and `"mindimprint/api/internal/store/sqlc"`. In `getLibraryShelf` replace

```go
	profile := library.Profile{
		Disciplines: a.interestDisciplines(r),
```

with

```go
	// A read error leaves her interests empty, as before: the shelf must still open.
	interests, _ := interestDisciplinesIn(r.Context(), a.d.Queries, u.ID)
	profile := library.Profile{
		Disciplines: interests,
```

Replace the whole `interestDisciplines` function and `mustUser` (their only caller is the shelf; `grep -rn "mustUser(\|interestDisciplines(" apps/api/internal` must print nothing afterwards) with:

```go
// interestDisciplinesIn folds her interest tree into discipline → strength.
//
// Strength = the keyword's strength (1..5) × the keyword→discipline edge's
// confidence. A keyword linked to two disciplines adds to both.
func interestDisciplinesIn(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (map[string]float64, error) {
	keywords, err := q.ListInterestKeywords(ctx, userID)
	if err != nil {
		return nil, err
	}
	strength := make(map[uuid.UUID]float64, len(keywords))
	for _, k := range keywords {
		strength[k.ID] = float64(k.Strength)
	}
	edges, err := q.ListKeywordDisciplinesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(edges))
	for _, e := range edges {
		out[e.DisciplineID] += strength[e.KeywordID] * float64(e.Confidence)
	}
	return out, nil
}

// libraryProfileIn is everything the recommender needs about one student:
// her interests, every article she has opened, and her suggested tier. q may
// be a transaction's queries, so a start reads it on its own connection.
func libraryProfileIn(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (library.Profile, error) {
	rows, err := q.ListLibraryReadingsByUser(ctx, userID)
	if err != nil {
		return library.Profile{}, err
	}
	interests, err := interestDisciplinesIn(ctx, q, userID)
	if err != nil {
		return library.Profile{}, err
	}
	read := make(map[string]bool, len(rows))
	for _, row := range rows {
		read[row.LibrarySlug] = true
	}
	return library.Profile{Disciplines: interests, ReadSlugs: read, Tier: suggestLibraryTierFromRows(rows)}, nil
}
```

Run: `cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test ./internal/api -run 'TestLibraryShelfStillRecommendsFromHerInterests|TestLibraryStart' -count=1 -timeout 1800s`
Expected: build clean; both PASS.

- [ ] **Step 4: Add the preview handler and route**

Create `apps/api/internal/api/lite_personalized_reading.go`:

```go
package api

// lite_personalized_reading.go — personalised reading homework: the class
// preview, the article a student starts on, and each recipient's article on
// the teacher's homework detail. No model call: reasons come from
// library.StudentPick.Reason.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/liteassign"
)

type personalizedPreviewRowDTO struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Tier   int    `json:"tier"`
	// SuggestedTier is her SuggestTier, shown when a pick leaves the tier open.
	SuggestedTier int    `json:"suggestedTier"`
	Reason        string `json:"reason"`
}

// previewLitePersonalizedReading handles
// POST /api/v1/lite/teacher/classes/{id}/personalized-reading/preview.
func (a *API) previewLitePersonalizedReading(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	classID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(ctx, classID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Disciplines []string `json:"disciplines"`
		Tier        *int     `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	filter, err := liteassign.ValidateDisciplines(req.Disciplines)
	if err != nil {
		httpx.WriteError(w, r, payloadErrorResponse(err))
		return
	}
	if req.Tier != nil && (*req.Tier < 1 || *req.Tier > 5) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_tier", "难度档位需在 1 到 5 之间", nil))
		return
	}
	students, err := a.d.Queries.ListLiteWeekClassStudents(ctx, classID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	articles := library.All()
	rows := make([]personalizedPreviewRowDTO, 0, len(students))
	for _, s := range students {
		prof, err := libraryProfileIn(ctx, a.d.Queries, s.ID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		suggested := prof.Tier
		if req.Tier != nil {
			prof.Tier = *req.Tier
		}
		pick, ok := library.PickForStudent(articles, prof, filter)
		if !ok {
			continue
		}
		rows = append(rows, personalizedPreviewRowDTO{
			UserID: s.ID.String(), Name: s.DisplayName,
			Slug: pick.Article.Slug, Title: pick.Article.ZhTitle,
			Tier: pick.Tier, SuggestedTier: suggested, Reason: pick.Reason(),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": rows})
}
```

In `apps/api/internal/api/lite_teacher_routes.go`, after the `POST /api/v1/lite/teacher/assignments/{aid}/recipients/{userId}/return` line, add:

```go
	mux.Handle("POST /api/v1/lite/teacher/classes/{id}/personalized-reading/preview", liteTeacher(a.previewLitePersonalizedReading))
```

- [ ] **Step 5: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go vet ./internal/api && CGO_ENABLED=0 go test ./internal/api -run 'TestLibraryShelfStillRecommendsFromHerInterests|TestPersonalizedPreview|TestLibraryStart|TestStartLibrary' -count=1 -timeout 1800s`
Expected: `ok  	mindimprint/api/internal/api`.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/library.go apps/api/internal/api/lite_personalized_reading.go \
  apps/api/internal/api/lite_personalized_reading_test.go apps/api/internal/api/lite_teacher_routes.go
git commit -m "feat(lite): personalised reading preview per class

The interest profile is read by user id, so the teacher end can build it
for each student. The student shelf reads it the same way.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Picks must be recipients; a student starts on her article

**Files:**
- Modify: `apps/api/internal/api/lite_teacher_assignments.go` (`createLiteAssignment`, `patchLiteAssignment`)
- Modify: `apps/api/internal/api/lite_student_assignments.go` (`createAssignedItemInTx`)
- Modify: `apps/api/internal/api/lite_personalized_reading.go` (add `personalizedTargetIn`)
- Test: `apps/api/internal/api/lite_personalized_reading_test.go`

**Interfaces:**
- Consumes: `liteassign.PicksOutside`, `liteassign.ReadingPayload.Picks/Disciplines/Tier` (Task 1); `library.PickForStudent` (Task 2); `libraryProfileIn` (Task 3); `startAssignedLibraryReading(ctx, tx, qtx, userID, p liteassign.ReadingPayload) (uuid.UUID, error)`; `qtx.ListLiteAssignmentRecipients(ctx, []uuid.UUID) ([]sqlc.ListLiteAssignmentRecipientsRow, error)`.
- Produces:
  - `func personalizedTargetIn(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, p liteassign.ReadingPayload) (liteassign.ReadingPayload, error)` — a `library` payload: her pick, else the recommendation now (filter = `p.Disciplines`, tier = `p.Tier`; nil tier means `startAssignedLibraryReading` uses her suggested tier).
  - Error `400 pick_not_recipient` 「个性化名单中有学生不在这份作业的学生名单中」 on create and PATCH.
  - Test helper `libraryReadingOf(t, pool, atomID string) (slug string, tier int)`.

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/api/lite_personalized_reading_test.go`:

```go
func libraryReadingOf(t *testing.T, pool *pgxpool.Pool, atomID string) (string, int) {
	t.Helper()
	var slug string
	var tier int
	if err := pool.QueryRow(context.Background(),
		`SELECT library_slug, library_tier FROM reading WHERE atom_id = $1`, atomID).Scan(&slug, &tier); err != nil {
		t.Fatalf("reading %s: %v", atomID, err)
	}
	return slug, tier
}

func personalizedPayload(filter []string, picks map[string]any) map[string]any {
	p := map[string]any{"source": "personalized", "picks": picks}
	if filter != nil {
		p["disciplines"] = filter
	}
	return p
}

func TestPersonalizedPicksMustBeRecipients(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	s2 := createStudent(t, pool, SeedSchoolID, "pr-s2@demo.local")
	enrollStudent(t, pool, s2, classID)
	slug := library.All()[0].Slug
	path := "/api/v1/lite/teacher/classes/" + classID + "/assignments"

	// s2 is enrolled but not a recipient.
	body := readingAssignmentBody("个性化", personalizedPayload(nil, map[string]any{
		s2.String(): map[string]any{"slug": slug},
	}), []string{s1.String()})
	if code, errCode := writeErrorCode(t, h, teacher, "POST", path, body); code != http.StatusBadRequest || errCode != "pick_not_recipient" {
		t.Fatalf("create with a non-recipient pick = %d %s", code, errCode)
	}

	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("个性化", personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": slug},
	}), []string{s1.String()}))
	withS2 := map[string]any{"payload": personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": slug}, s2.String(): map[string]any{"slug": slug},
	})}
	if code, errCode := writeErrorCode(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid, withS2); code != http.StatusBadRequest || errCode != "pick_not_recipient" {
		t.Fatalf("patch with a non-recipient pick = %d %s", code, errCode)
	}
	// Adding s2 in the same request makes the pick valid.
	withS2["addUserIds"] = []string{s2.String()}
	if code := assignJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid, withS2, nil); code != http.StatusOK {
		t.Fatalf("patch adding s2 with her pick = %d", code)
	}
	// An unknown article is refused by payload validation.
	bad := readingAssignmentBody("个性化", personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": "no-such-article"},
	}), []string{s1.String()})
	if code, errCode := writeErrorCode(t, h, teacher, "POST", path, bad); code != http.StatusBadRequest || errCode != "invalid_pick_slug" {
		t.Fatalf("unknown pick slug = %d %s", code, errCode)
	}
}

func TestPersonalizedStart(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	s2 := createStudent(t, pool, SeedSchoolID, "ps-s2@demo.local")
	s3 := createStudent(t, pool, SeedSchoolID, "ps-s3@demo.local")
	enrollStudent(t, pool, s2, classID)
	enrollStudent(t, pool, s3, classID)
	all := library.All()
	filter := []string{all[len(all)-1].Disciplines[0]}

	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("个性化", personalizedPayload(filter, map[string]any{
		s1.String(): map[string]any{"slug": all[1].Slug, "tier": 4},
		s2.String(): map[string]any{"slug": all[2].Slug, "tier": nil},
	}), []string{s1.String(), s2.String()}))
	// s3 joins after the picks were made: no pick.
	if code := assignJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"addUserIds": []string{s3.String()}}, nil); code != http.StatusOK {
		t.Fatalf("add s3 = %d", code)
	}

	out := startAssignment(t, h, signInAs(t, pool, s1), aid)
	if slug, tier := libraryReadingOf(t, pool, out.AtomID); out.Kind != "reading" || slug != all[1].Slug || tier != 4 {
		t.Fatalf("s1 started %s tier %d, want %s tier 4", slug, tier, all[1].Slug)
	}
	out = startAssignment(t, h, signInAs(t, pool, s2), aid)
	if slug, tier := libraryReadingOf(t, pool, out.AtomID); slug != all[2].Slug || tier != library.SuggestTier(0, 0) {
		t.Fatalf("s2 started %s tier %d, want %s at her suggested tier", slug, tier, all[2].Slug)
	}
	want, _ := library.PickForStudent(all, library.Profile{Tier: library.SuggestTier(0, 0)}, filter)
	out = startAssignment(t, h, signInAs(t, pool, s3), aid)
	if slug, tier := libraryReadingOf(t, pool, out.AtomID); slug != want.Article.Slug || tier != library.SuggestTier(0, 0) {
		t.Fatalf("s3 started %s tier %d, want the recommendation %s", slug, tier, want.Article.Slug)
	}
	// Starting again returns the same item.
	if again := startAssignment(t, h, signInAs(t, pool, s3), aid); again.AtomID != out.AtomID {
		t.Fatalf("repeat start = %s, want %s", again.AtomID, out.AtomID)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestPersonalizedPicksMustBeRecipients|TestPersonalizedStart' -count=1 -timeout 1800s`
Expected: FAIL. The recipients test gets `201`/`200` where it wants `pick_not_recipient`; the start test fails with `start … = 500` (`lite assignment: unknown reading source personalized`).

- [ ] **Step 3: Check picks against recipients**

In `apps/api/internal/api/lite_teacher_assignments.go`, add near `errAssignmentStarted`:

```go
func errPickNotRecipient() error {
	return httpx.ErrBadRequest("pick_not_recipient", "个性化名单中有学生不在这份作业的学生名单中", nil)
}
```

In `createLiteAssignment`, right after `ids, err := a.assignmentRecipientIDs(...)` and its error check:

```go
	if liteassign.PicksOutside(payload, ids) {
		httpx.WriteError(w, r, errPickNotRecipient())
		return
	}
```

In `patchLiteAssignment`, after the `for _, uid := range addIDs { … }` loop and before `updated, err := qtx.UpdateLiteAssignment(ctx, params)`:

```go
	// Picks are checked against the recipient list this request leaves behind,
	// so a teacher can add a student and her pick in one save.
	if req.Payload != nil {
		current, err := qtx.ListLiteAssignmentRecipients(ctx, []uuid.UUID{locked.ID})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		userIDs := make([]uuid.UUID, 0, len(current))
		for _, rc := range current {
			userIDs = append(userIDs, rc.UserID)
		}
		if liteassign.PicksOutside(params.Payload, userIDs) {
			httpx.WriteError(w, r, errPickNotRecipient())
			return
		}
	}
```

(The deferred `tx.Rollback` undoes the add/remove when this refuses.)

- [ ] **Step 4: Start a personalised reading**

Append to `apps/api/internal/api/lite_personalized_reading.go` (add imports `"context"` and `"mindimprint/api/internal/store/sqlc"`):

```go
// personalizedTargetIn turns a personalized payload into the library reading
// this student starts: her pick, or, when she has none (she was added after
// the picks were saved), the recommendation computed now. A nil Tier leaves
// the tier to startAssignedLibraryReading, which uses her suggested tier.
func personalizedTargetIn(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, p liteassign.ReadingPayload) (liteassign.ReadingPayload, error) {
	if pick, ok := p.Picks[userID.String()]; ok {
		return liteassign.ReadingPayload{Source: "library", Slug: pick.Slug, Tier: pick.Tier}, nil
	}
	prof, err := libraryProfileIn(ctx, q, userID)
	if err != nil {
		return liteassign.ReadingPayload{}, err
	}
	if p.Tier != nil {
		prof.Tier = *p.Tier
	}
	pick, ok := library.PickForStudent(library.All(), prof, p.Disciplines)
	if !ok {
		return liteassign.ReadingPayload{}, errLibraryArticleNotFound
	}
	return liteassign.ReadingPayload{Source: "library", Slug: pick.Article.Slug, Tier: p.Tier}, nil
}
```

In `apps/api/internal/api/lite_student_assignments.go`, `createAssignedItemInTx`, inside `case "reading":` add a case after `case "library":`:

```go
		case "personalized":
			// Read through qtx: a start holds one pooled connection at a time.
			target, err := personalizedTargetIn(ctx, qtx, userID, p)
			if err != nil {
				return uuid.Nil, err
			}
			return startAssignedLibraryReading(ctx, tx, qtx, userID, target)
```

`fetchAssignedArticle` needs no change: it returns nil for every source but `url`.

- [ ] **Step 5: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go vet ./internal/api && CGO_ENABLED=0 go test ./internal/api -run 'TestPersonalized|TestStart|TestTeacherAssignment|TestAssignmentRubric' -count=1 -timeout 1800s`
Expected: `ok  	mindimprint/api/internal/api` (the existing start, PATCH-lock and Part B rubric tests still pass).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/lite_teacher_assignments.go apps/api/internal/api/lite_student_assignments.go \
  apps/api/internal/api/lite_personalized_reading.go apps/api/internal/api/lite_personalized_reading_test.go
git commit -m "feat(lite): start personalised reading on the student's own article

A pick must belong to a recipient. A student added after the picks were
saved is recommended an article when she starts.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Each recipient's article on the homework detail

**Files:**
- Modify: `apps/api/internal/api/lite_teacher_assignments.go` (`RecipientDTO`, `getLiteAssignment`)
- Modify: `apps/api/internal/api/lite_personalized_reading.go` (add `personalizedRecipientReadings`)
- Test: `apps/api/internal/api/lite_personalized_reading_test.go`

**Interfaces:**
- Consumes: `liteassign.ReadingPayload` (Task 1); `a.d.Queries.GetReading(ctx, atomID uuid.UUID) (sqlc.Reading{LibrarySlug string, LibraryTier int16, Title string}, error)`; `library.BySlug`.
- Produces:
  - `type RecipientReadingDTO struct { Slug string \`json:"slug"\`; Title string \`json:"title"\`; Tier *int \`json:"tier"\`; State string \`json:"state"\` }` — `State` is `started` (her reading exists: its slug and tier), `picked` (the saved pick; `tier` null means her level at start) or `pending` (no pick, not started: slug and title empty, tier null).
  - `RecipientDTO.Reading *RecipientReadingDTO \`json:"reading"\`` — set on `GET /api/v1/lite/teacher/assignments/{aid}` for a personalized reading homework, `null` everywhere else.

- [ ] **Step 1: Write the failing test**

Append to `apps/api/internal/api/lite_personalized_reading_test.go`:

```go
func TestPersonalizedDetailShowsEachArticle(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	s2 := createStudent(t, pool, SeedSchoolID, "pd-s2@demo.local")
	s3 := createStudent(t, pool, SeedSchoolID, "pd-s3@demo.local")
	enrollStudent(t, pool, s2, classID)
	enrollStudent(t, pool, s3, classID)
	all := library.All()
	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("个性化", personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": all[1].Slug, "tier": 4},
		s2.String(): map[string]any{"slug": all[2].Slug, "tier": nil},
	}), []string{s1.String(), s2.String(), s3.String()}))
	startAssignment(t, h, signInAs(t, pool, s1), aid)

	type reading struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
		Tier  *int   `json:"tier"`
		State string `json:"state"`
	}
	var detail struct {
		Recipients []struct {
			UserID  string   `json:"userId"`
			Reading *reading `json:"reading"`
		} `json:"recipients"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+aid, &detail); code != http.StatusOK {
		t.Fatalf("detail = %d", code)
	}
	got := map[string]*reading{}
	for _, rc := range detail.Recipients {
		got[rc.UserID] = rc.Reading
	}
	if r := got[s1.String()]; r == nil || r.State != "started" || r.Slug != all[1].Slug || r.Title != all[1].ZhTitle || r.Tier == nil || *r.Tier != 4 {
		t.Fatalf("s1 reading = %+v, want started %s tier 4", r, all[1].Slug)
	}
	if r := got[s2.String()]; r == nil || r.State != "picked" || r.Slug != all[2].Slug || r.Title != all[2].ZhTitle || r.Tier != nil {
		t.Fatalf("s2 reading = %+v, want picked %s with an open tier", r, all[2].Slug)
	}
	if r := got[s3.String()]; r == nil || r.State != "pending" || r.Slug != "" || r.Tier != nil {
		t.Fatalf("s3 reading = %+v, want pending", r)
	}

	// Other homework kinds carry reading: null.
	wid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{s1.String()}))
	var raw struct {
		Recipients []map[string]any `json:"recipients"`
	}
	getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+wid, &raw)
	if v, ok := raw.Recipients[0]["reading"]; !ok || v != nil {
		t.Fatalf("writing recipient reading = %v (present %v), want null", v, ok)
	}

	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pd-other@demo.local"))
	if code := getJSON(t, h, other, "/api/v1/lite/teacher/assignments/"+aid, nil); code != http.StatusNotFound {
		t.Fatalf("other teacher detail = %d, want 404", code)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestPersonalizedDetailShowsEachArticle -count=1 -timeout 1800s`
Expected: FAIL with `s1 reading = <nil>`.

- [ ] **Step 3: Implement**

In `apps/api/internal/api/lite_teacher_assignments.go`, add the field at the end of `RecipientDTO`:

```go
	// Reading is this student's article on a personalized reading homework; nil otherwise.
	Reading *RecipientReadingDTO `json:"reading"`
```

Append to `apps/api/internal/api/lite_personalized_reading.go`:

```go
// RecipientReadingDTO is one student's article on a personalized reading
// homework. State: started (her reading's slug and tier), picked (the saved
// pick; a nil Tier means her level at start) or pending (no pick yet; the
// article is chosen when she starts).
type RecipientReadingDTO struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Tier  *int   `json:"tier"`
	State string `json:"state"`
}

func libraryTitle(slug, fallback string) string {
	if art, ok := library.BySlug(slug); ok {
		return art.ZhTitle
	}
	return fallback
}

// personalizedRecipientReadings maps each recipient to her article. It returns
// nil for any payload that is not a personalized reading.
func (a *API) personalizedRecipientReadings(ctx context.Context, kind string, payload json.RawMessage, rows []sqlc.ListLiteAssignmentRecipientsRow) (map[uuid.UUID]*RecipientReadingDTO, error) {
	if kind != "reading" {
		return nil, nil
	}
	var p liteassign.ReadingPayload
	if err := json.Unmarshal(payload, &p); err != nil || p.Source != "personalized" {
		return nil, nil
	}
	out := make(map[uuid.UUID]*RecipientReadingDTO, len(rows))
	for _, row := range rows {
		if row.AtomID.Valid {
			rd, err := a.d.Queries.GetReading(ctx, uuid.UUID(row.AtomID.Bytes))
			if err != nil {
				return nil, err
			}
			tier := int(rd.LibraryTier)
			out[row.UserID] = &RecipientReadingDTO{Slug: rd.LibrarySlug, Title: libraryTitle(rd.LibrarySlug, rd.Title), Tier: &tier, State: "started"}
			continue
		}
		if pick, ok := p.Picks[row.UserID.String()]; ok {
			out[row.UserID] = &RecipientReadingDTO{Slug: pick.Slug, Title: libraryTitle(pick.Slug, pick.Slug), Tier: pick.Tier, State: "picked"}
			continue
		}
		out[row.UserID] = &RecipientReadingDTO{State: "pending"}
	}
	return out, nil
}
```

In `getLiteAssignment`, replace the recipients loop with:

```go
	readings, err := a.personalizedRecipientReadings(r.Context(), as.Kind, json.RawMessage(as.Payload), rows)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	now := time.Now()
	recipients := make([]RecipientDTO, 0, len(rows))
	for _, row := range rows {
		dto := newRecipientDTO(row, as.DueAt, now)
		dto.Reading = readings[row.UserID]
		recipients = append(recipients, dto)
	}
```

A reading homework started before Part C ships is never `personalized`, so it never reaches `GetReading`.

- [ ] **Step 4: Run the tests**

Run: `cd apps/api && CGO_ENABLED=0 go vet ./internal/api && CGO_ENABLED=0 go test ./internal/api -run 'TestPersonalized|TestTeacherAssignment|TestReturn' -count=1 -timeout 1800s`
Expected: `ok  	mindimprint/api/internal/api`. If a Part A/B test asserts the exact key set of a recipient, add `"reading"` to its expected keys (grep `returnNote` in `*_test.go` to find key-set assertions).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/lite_teacher_assignments.go apps/api/internal/api/lite_personalized_reading.go \
  apps/api/internal/api/lite_personalized_reading_test.go
git commit -m "feat(lite): homework detail shows each student's personalised article

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Frontend clients and pure logic

**Files:**
- Modify: `apps/lite-web/src/api/assignments.ts`
- Test: `apps/lite-web/src/api/assignments.test.ts`
- Modify: `apps/lite-web/src/teacher/assignmentLogic.ts`
- Test: `apps/lite-web/src/teacher/assignmentLogic.test.ts`

**Interfaces:**
- Consumes: Task 3 preview wire shape `{rows: [{userId, name, slug, title, tier, suggestedTier, reason}]}`; Task 5 `recipients[].reading`; Task 1 payload shape; `LibraryArticle`, `LibraryTag` (`api/library.ts`).
- Produces (`api/assignments.ts`):
  - `interface PersonalPick { slug: string; tier: number | null }`
  - `ReadingAssignmentPayload.source` adds `"personalized"`; fields `fileName?: string`, `disciplines?: string[]`, `picks?: Record<string, PersonalPick>`.
  - `interface RecipientReading { slug: string; title: string; tier: number | null; state: "started" | "picked" | "pending" }`; `RecipientDTO.reading: RecipientReading | null`.
  - `interface PreviewRow { userId: string; name: string; slug: string; title: string; tier: number; suggestedTier: number; reason: string }`
  - `normalizeRecipientReading(raw: unknown): RecipientReading | null`, `normalizePreviewRows(raw: unknown): PreviewRow[]`
  - `previewPersonalizedReading(classId: string, input: { disciplines: string[]; tier: number | null }): Promise<PreviewRow[]>`
- Produces (`teacher/assignmentLogic.ts`):
  - `ReadingSource = "library" | "url" | "text" | "file" | "personalized"`
  - `SettingsDraft` adds `fileName: string; disciplines: string[]; personalTier: number | null; picks: PickRow[] | null; savedPicks: Record<string, PersonalPick>`
  - `interface PickRow { userId: string; name: string; slug: string; title: string; tier: number | null; suggestedTier: number; reason: string; swapped: boolean }`
  - `interface KeptPick { slug: string; title: string; tier: number | null }`
  - `type ExtractOutcome = { ok: true; text: string; fileName: string; title: string } | { ok: false; error: string }`
  - `readExtractResult(result: { title: string; text: string }, fileName: string): ExtractOutcome`, `extractedCountText(text: string): string`, `fillTitleIfEmpty(current: string, candidate: string): string`
  - `mergePickRows(preview: PreviewRow[], personalTier: number | null, kept: Record<string, KeptPick>): PickRow[]`, `keptFromRows(rows: PickRow[] | null): Record<string, KeptPick>`, `keptFromSaved(saved: Record<string, PersonalPick>, articles: LibraryArticle[]): Record<string, KeptPick>`, `swapPick(rows: PickRow[], userId: string, next: { slug: string; tier: number | null }, articles: LibraryArticle[]): PickRow[]`, `visiblePickRows(rows: PickRow[], recipientIds: string[]): PickRow[]`, `pickTierText(row: PickRow): string`, `disciplineOptions(articles: LibraryArticle[]): LibraryTag[]`
  - `buildPayload(d: SettingsDraft, recipientIds?: string[]): AssignmentPayload`; `buildPatchInput(e: EditDraft, settingsEditable: boolean, recipientIds?: string[])`
  - `recipientReadingText(reading: RecipientReading | null): string`, `assignmentFileName(a: { kind: AssignmentKind; payload: Record<string, unknown> }): string | null`

- [ ] **Step 1: Write the failing client tests**

Append to `apps/lite-web/src/api/assignments.test.ts` (extend the import with `normalizePreviewRows`):

```ts
describe("normalizeRecipientDTO reading", () => {
  const base = { userId: "u1", displayName: "Phoebe", status: "not_started" };
  it("is null when the server sends none or an unknown state", () => {
    expect(normalizeRecipientDTO(base).reading).toBeNull();
    expect(normalizeRecipientDTO({ ...base, reading: { slug: "a", state: "later" } }).reading).toBeNull();
  });
  it("keeps a picked article with an open tier", () => {
    expect(normalizeRecipientDTO({ ...base, reading: { slug: "coral", title: "珊瑚", tier: null, state: "picked" } }).reading).toEqual({
      slug: "coral",
      title: "珊瑚",
      tier: null,
      state: "picked",
    });
  });
  it("drops a tier outside 1..5", () => {
    expect(normalizeRecipientDTO({ ...base, reading: { slug: "coral", title: "珊瑚", tier: 9, state: "started" } }).reading?.tier).toBeNull();
  });
});

describe("normalizePreviewRows", () => {
  it("drops rows without a student or an article and repairs the tier", () => {
    const rows = normalizePreviewRows({
      rows: [
        { userId: "u1", name: "Phoebe", slug: "coral", title: "珊瑚", tier: 3, suggestedTier: 2, reason: "暂无兴趣数据，按难度推荐" },
        { userId: "u2", name: "林知遥", slug: "", title: "", tier: 3, suggestedTier: 2, reason: "" },
        { name: "no id", slug: "x" },
        { userId: "u3", name: "王", slug: "nasa", title: "NASA", tier: 0, suggestedTier: 4, reason: "" },
      ],
    });
    expect(rows.map((r) => r.userId)).toEqual(["u1", "u3"]);
    expect(rows[1]).toMatchObject({ tier: 4, suggestedTier: 4 });
  });
  it("is empty for a malformed body", () => {
    expect(normalizePreviewRows(null)).toEqual([]);
  });
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `cd apps/lite-web && pnpm vitest run src/api/assignments.test.ts`
Expected: FAIL (`normalizePreviewRows` is not exported; `reading` undefined).

- [ ] **Step 3: Implement the client**

In `apps/lite-web/src/api/assignments.ts`:

```ts
export interface PersonalPick {
  slug: string;
  /** null: the student's own level when she starts. */
  tier: number | null;
}

export interface ReadingAssignmentPayload {
  source: "library" | "url" | "text" | "personalized";
  slug?: string;
  tier?: number | null;
  url?: string;
  text?: string;
  /** Set when the text came from an uploaded document. */
  fileName?: string;
  disciplines?: string[];
  picks?: Record<string, PersonalPick>;
}

/** One student's article on a personalized reading homework
 * (lite_personalized_reading.go RecipientReadingDTO). */
export interface RecipientReading {
  slug: string;
  title: string;
  tier: number | null;
  state: "started" | "picked" | "pending";
}

/** One row of POST …/classes/{id}/personalized-reading/preview. */
export interface PreviewRow {
  userId: string;
  name: string;
  slug: string;
  title: string;
  tier: number;
  suggestedTier: number;
  reason: string;
}
```

Add `reading: RecipientReading | null;` at the end of `RecipientDTO`. Add the normalizers next to the others:

```ts
const tierOrNull = (v: unknown): number | null =>
  typeof v === "number" && Number.isInteger(v) && v >= 1 && v <= 5 ? v : null;

export function normalizeRecipientReading(raw: unknown): RecipientReading | null {
  if (!raw || typeof raw !== "object") return null;
  const r = obj(raw);
  if (r.state !== "started" && r.state !== "picked" && r.state !== "pending") return null;
  return { slug: s(r.slug), title: s(r.title), tier: tierOrNull(r.tier), state: r.state };
}

export function normalizePreviewRows(raw: unknown): PreviewRow[] {
  return arr<unknown>(obj(raw).rows)
    .map(obj)
    .filter((r) => s(r.userId) !== "" && s(r.slug) !== "")
    .map((r) => {
      const suggestedTier = tierOrNull(r.suggestedTier) ?? 2;
      return {
        userId: s(r.userId),
        name: s(r.name),
        slug: s(r.slug),
        title: s(r.title),
        tier: tierOrNull(r.tier) ?? suggestedTier,
        suggestedTier,
        reason: s(r.reason),
      };
    });
}

export async function previewPersonalizedReading(
  classId: string,
  input: { disciplines: string[]; tier: number | null },
): Promise<PreviewRow[]> {
  const body = { ...(input.disciplines.length ? { disciplines: input.disciplines } : {}), ...(input.tier !== null ? { tier: input.tier } : {}) };
  const r = await apiFetch<unknown>(`${teacherBase}/classes/${encodeURIComponent(classId)}/personalized-reading/preview`, {
    method: "POST",
    body: JSON.stringify(body),
  });
  return normalizePreviewRows(r);
}
```

In `normalizeRecipientDTO` add `reading: normalizeRecipientReading(raw.reading),`.

Run: `cd apps/lite-web && pnpm vitest run src/api/assignments.test.ts`
Expected: PASS.

- [ ] **Step 4: Write the failing logic tests**

In `apps/lite-web/src/teacher/assignmentLogic.test.ts`: add `reading: null,` to the `recipient()` fixture; extend the import list with `assignmentFileName, disciplineOptions, extractedCountText, fillTitleIfEmpty, keptFromRows, keptFromSaved, mergePickRows, pickTierText, readExtractResult, recipientReadingText, swapPick, visiblePickRows, type PickRow` and add `import type { PreviewRow } from "../api/assignments";`. Append:

```ts
function article(slug: string, zhTitle: string, tags: string[] = []): LibraryArticle {
  return {
    slug,
    title: slug,
    zhTitle,
    reason: "",
    field: "science",
    tags: tags.map((id) => ({ id, zh: `学科${id}`, field: "science" })),
    coverUrl: "",
    levels: [],
    finished: false,
  };
}

function preview(over: Partial<PreviewRow> = {}): PreviewRow {
  return { userId: "u1", name: "Phoebe", slug: "coral", title: "珊瑚", tier: 2, suggestedTier: 2, reason: "暂无兴趣数据，按难度推荐", ...over };
}

describe("readExtractResult", () => {
  it("keeps the trimmed text, the file name and a title", () => {
    expect(readExtractResult({ title: "雨水花园", text: "  正文  " }, "rain.pdf")).toEqual({ ok: true, text: "正文", fileName: "rain.pdf", title: "雨水花园" });
  });
  it("uses the file name without its extension when the document has no title", () => {
    const r = readExtractResult({ title: "", text: "x" }, "校园积水调查.docx");
    expect(r.ok && r.title).toBe("校园积水调查");
  });
  it("refuses more than 50000 characters, counting characters not bytes", () => {
    expect(readExtractResult({ title: "", text: "雨".repeat(50000) }, "a.txt").ok).toBe(true);
    expect(readExtractResult({ title: "", text: "雨".repeat(50001) }, "a.txt")).toEqual({ ok: false, error: "提取失败：正文超过 50000 字" });
  });
  it("caps the file name at 200 characters", () => {
    const r = readExtractResult({ title: "t", text: "x" }, `${"名".repeat(250)}.pdf`);
    expect(r.ok && [...r.fileName].length).toBe(200);
  });
  it("counts the extracted characters", () => {
    expect(extractedCountText(" 你好 ")).toBe("已提取 2 字");
  });
  it("fills the title only when it is empty", () => {
    expect(fillTitleIfEmpty("  ", "雨水花园")).toBe("雨水花园");
    expect(fillTitleIfEmpty("第三周阅读", "雨水花园")).toBe("第三周阅读");
  });
});

describe("file and personalized settings", () => {
  it("builds a text payload with the file name from the upload tab", () => {
    const d = { ...emptySettings("reading"), readingSource: "file" as const, text: " 正文 ", fileName: "rain.pdf" };
    expect(validateSettings(d)).toBeNull();
    expect(buildPayload(d)).toEqual({ source: "text", text: "正文", fileName: "rain.pdf" });
    expect(validateSettings({ ...d, text: "" })).toBe("请上传文件");
    expect(buildPayload({ ...d, readingSource: "text" })).toEqual({ source: "text", text: "正文" });
  });
  it("reopens a stored text with a file name on the upload tab", () => {
    const d = settingsFromAssignment("reading", { source: "text", text: "正文", fileName: "rain.pdf" });
    expect(d.readingSource).toBe("file");
    expect(settingsSummary("reading", { source: "text", text: "正文", fileName: "rain.pdf" })).toBe("上传文件 · rain.pdf · 2 字");
    expect(assignmentFileName({ kind: "reading", payload: { source: "text", text: "x", fileName: "rain.pdf" } })).toBe("rain.pdf");
    expect(assignmentFileName({ kind: "reading", payload: { source: "text", text: "x" } })).toBeNull();
  });
  it("waits for the preview before a personalized homework can be saved", () => {
    const d = { ...emptySettings("reading"), readingSource: "personalized" as const };
    expect(validateSettings(d)).toBe("请等待推荐列表加载完成");
    expect(validateSettings({ ...d, picks: [] })).toBeNull();
  });
  it("sends picks for recipients only, the filter and the tier when set", () => {
    const rows = mergePickRows([preview({ userId: "u1" }), preview({ userId: "u2", slug: "nasa", title: "NASA" })], null, {});
    const d = { ...emptySettings("reading"), readingSource: "personalized" as const, picks: rows, disciplines: ["astronomy"], personalTier: 4 };
    expect(buildPayload(d, ["u2"])).toEqual({
      source: "personalized",
      disciplines: ["astronomy"],
      tier: 4,
      picks: { u2: { slug: "nasa", tier: null } },
    });
    expect(buildPayload({ ...d, disciplines: [], personalTier: null }, undefined)).toEqual({
      source: "personalized",
      picks: { u1: { slug: "coral", tier: null }, u2: { slug: "nasa", tier: null } },
    });
  });
  it("reads a stored personalized payload back", () => {
    const d = settingsFromAssignment("reading", {
      source: "personalized",
      disciplines: ["astronomy", 3],
      tier: 4,
      picks: { u1: { slug: "coral", tier: null }, u2: { slug: 7 } },
    });
    expect(d).toMatchObject({ readingSource: "personalized", disciplines: ["astronomy"], personalTier: 4, picks: null });
    expect(d.savedPicks).toEqual({ u1: { slug: "coral", tier: null } });
    expect(settingsSummary("reading", { source: "personalized", tier: 4, disciplines: ["a", "b"] })).toBe("个性化阅读 · 高阶 · 学科筛选 2 项");
    expect(settingsSummary("reading", { source: "personalized" })).toBe("个性化阅读 · 按学生当前水平");
  });
});

describe("pick rows", () => {
  const articles = [article("coral", "珊瑚", ["biology"]), article("nasa", "NASA", ["astronomy", "biology"])];

  it("takes the preview and the chosen tier", () => {
    const rows = mergePickRows([preview()], 3, {});
    expect(rows[0]).toMatchObject({ slug: "coral", tier: 3, swapped: false, reason: "暂无兴趣数据，按难度推荐" });
    expect(pickTierText(rows[0] as PickRow)).toBe("进阶");
    expect(pickTierText({ ...(rows[0] as PickRow), tier: null, suggestedTier: 2 })).toBe("基础");
  });
  it("a swap is kept when the preview is run again", () => {
    let rows = mergePickRows([preview()], null, {});
    rows = swapPick(rows, "u1", { slug: "nasa", tier: 5 }, articles);
    expect(rows[0]).toMatchObject({ slug: "nasa", title: "NASA", tier: 5, reason: "已更换", swapped: true });
    const again = mergePickRows([preview({ slug: "coral" }), preview({ userId: "u9" })], null, keptFromRows(rows));
    expect(again[0]).toMatchObject({ slug: "nasa", tier: 5, swapped: true });
    expect(again[1]).toMatchObject({ userId: "u9", swapped: false });
  });
  it("a saved pick that matches the new preview is not marked as swapped", () => {
    const kept = keptFromSaved({ u1: { slug: "coral", tier: null }, u2: { slug: "nasa", tier: 2 } }, articles);
    const rows = mergePickRows([preview({ userId: "u1" }), preview({ userId: "u2" })], null, kept);
    expect(rows[0]).toMatchObject({ slug: "coral", swapped: false });
    expect(rows[1]).toMatchObject({ slug: "nasa", title: "NASA", tier: 2, swapped: true });
  });
  it("a student who left the class is dropped", () => {
    const rows = mergePickRows([preview({ userId: "u1" })], null, { gone: { slug: "nasa", title: "NASA", tier: null } });
    expect(rows.map((r) => r.userId)).toEqual(["u1"]);
  });
  it("shows only checked recipients", () => {
    const rows = mergePickRows([preview({ userId: "u1" }), preview({ userId: "u2" })], null, {});
    expect(visiblePickRows(rows, ["u2"]).map((r) => r.userId)).toEqual(["u2"]);
  });
  it("lists each discipline tag once, in library order", () => {
    expect(disciplineOptions(articles).map((t) => t.id)).toEqual(["biology", "astronomy"]);
  });
  it("describes a recipient's article", () => {
    expect(recipientReadingText({ slug: "coral", title: "珊瑚", tier: 3, state: "started" })).toBe("珊瑚 · 进阶");
    expect(recipientReadingText({ slug: "coral", title: "", tier: null, state: "picked" })).toBe("coral · 按学生当前水平");
    expect(recipientReadingText({ slug: "", title: "", tier: null, state: "pending" })).toBe("待推荐");
    expect(recipientReadingText(null)).toBe("—");
  });
});
```

- [ ] **Step 5: Run to verify they fail**

Run: `cd apps/lite-web && pnpm vitest run src/teacher/assignmentLogic.test.ts`
Expected: FAIL (missing exports).

- [ ] **Step 6: Implement the logic**

In `apps/lite-web/src/teacher/assignmentLogic.ts`, extend the type imports with `PersonalPick, PreviewRow, RecipientReading` from `../api/assignments` and `LibraryTag` from `../api/library`. Then:

```ts
export type ReadingSource = "library" | "url" | "text" | "file" | "personalized";

export interface PickRow {
  userId: string;
  name: string;
  slug: string;
  title: string;
  /** The tier saved with the pick; null = her level at start. */
  tier: number | null;
  suggestedTier: number;
  reason: string;
  /** The teacher chose this article; a new preview keeps it. */
  swapped: boolean;
}

export interface KeptPick {
  slug: string;
  title: string;
  tier: number | null;
}
```

Add to `SettingsDraft`: `fileName: string; disciplines: string[]; personalTier: number | null; picks: PickRow[] | null; savedPicks: Record<string, PersonalPick>;` and to `emptySettings`: `fileName: "", disciplines: [], personalTier: null, picks: null, savedPicks: {},`.

Replace the `if (kind === "reading")` branch of `settingsFromAssignment`:

```ts
  if (kind === "reading") {
    const source = payload.source;
    d.readingSource = source === "url" || source === "text" || source === "personalized" ? source : "library";
    d.slug = str(payload.slug);
    d.tier = typeof payload.tier === "number" ? payload.tier : null;
    d.url = str(payload.url);
    d.text = str(payload.text);
    d.fileName = str(payload.fileName);
    if (d.readingSource === "text" && d.fileName) d.readingSource = "file";
    if (d.readingSource === "personalized") {
      d.disciplines = Array.isArray(payload.disciplines) ? payload.disciplines.filter((x): x is string => typeof x === "string") : [];
      d.personalTier = d.tier;
      d.tier = null;
      d.savedPicks = savedPicksOf(payload.picks);
    }
  }
```

with the helpers:

```ts
const cutRunes = (s: string, n: number): string => [...s].slice(0, n).join("");

function savedPicksOf(raw: unknown): Record<string, PersonalPick> {
  const out: Record<string, PersonalPick> = {};
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return out;
  for (const [uid, v] of Object.entries(raw as Record<string, unknown>)) {
    const p = v && typeof v === "object" ? (v as Record<string, unknown>) : {};
    if (typeof p.slug !== "string" || !p.slug) continue;
    out[uid] = { slug: p.slug, tier: typeof p.tier === "number" ? p.tier : null };
  }
  return out;
}

const titleOf = (a: LibraryArticle | undefined, slug: string): string => (a ? a.zhTitle || a.title : "") || slug;
```

Replace the reading part of `validateSettings`:

```ts
  if (d.kind === "reading") {
    if (d.readingSource === "library") {
      if (!d.slug.trim()) return "请选择一篇文章";
      if (d.tier !== null && (d.tier < 1 || d.tier > 5)) return "难度档位需在 1 到 5 之间";
    } else if (d.readingSource === "url") {
      if (!isHttpUrlWithHost(d.url)) return "请输入以 http 或 https 开头的链接";
    } else if (d.readingSource === "personalized") {
      if (d.picks === null) return "请等待推荐列表加载完成";
    } else {
      const text = d.text.trim();
      if (!text) return d.readingSource === "file" ? "请上传文件" : "请粘贴文章正文";
      if (runes(text) > 50000) return "文章正文不能超过 50000 字";
    }
    return null;
  }
```

Replace `buildPayload`'s signature and reading branch:

```ts
export function buildPayload(d: SettingsDraft, recipientIds?: string[]): AssignmentPayload {
  if (d.kind === "reading") {
    if (d.readingSource === "library") {
      return d.tier === null ? { source: "library", slug: d.slug.trim() } : { source: "library", slug: d.slug.trim(), tier: d.tier };
    }
    if (d.readingSource === "url") return { source: "url", url: d.url.trim() };
    if (d.readingSource === "personalized") {
      const keep = recipientIds ? new Set(recipientIds) : null;
      const picks: Record<string, PersonalPick> = {};
      for (const row of d.picks ?? []) {
        if (!keep || keep.has(row.userId)) picks[row.userId] = { slug: row.slug, tier: row.tier };
      }
      return {
        source: "personalized",
        ...(d.disciplines.length ? { disciplines: d.disciplines } : {}),
        ...(d.personalTier !== null ? { tier: d.personalTier } : {}),
        picks,
      };
    }
    const fileName = d.fileName.trim();
    return d.readingSource === "file" && fileName
      ? { source: "text", text: d.text.trim(), fileName }
      : { source: "text", text: d.text.trim() };
  }
```

(the writing and project branches stay as they are). In `buildCreateInput` change `payload: buildPayload(d)` to `payload: buildPayload(d, d.userIds)`. In `buildPatchInput` add the parameter `recipientIds?: string[]` and change `patch.payload = buildPayload(e.settings)` to `patch.payload = buildPayload(e.settings, recipientIds)`.

Add the extract and pick helpers:

```ts
export type ExtractOutcome = { ok: true; text: string; fileName: string; title: string } | { ok: false; error: string };

/** A /documents/extract result → the text source. The 50000-character cap is
 * the server's; checking it here names the problem before 发布. */
export function readExtractResult(result: { title: string; text: string }, fileName: string): ExtractOutcome {
  const text = result.text.trim();
  if (!text) return { ok: false, error: "提取失败：文件中没有读到文字" };
  if (runes(text) > 50000) return { ok: false, error: "提取失败：正文超过 50000 字" };
  const name = cutRunes(fileName.trim(), 200);
  const stem = name.replace(/\.[^.]+$/, "");
  return { ok: true, text, fileName: name, title: cutRunes(result.title.trim() || stem, 200) };
}

export function extractedCountText(text: string): string {
  return `已提取 ${runes(text.trim())} 字`;
}

export function fillTitleIfEmpty(current: string, candidate: string): string {
  return current.trim() ? current : candidate;
}

/** Preview rows → pick rows. A kept pick (a swap, or a saved pick being
 * edited) replaces the preview's article unless it is the same article and
 * tier. Students not in the preview (no longer enrolled) are dropped. */
export function mergePickRows(preview: PreviewRow[], personalTier: number | null, kept: Record<string, KeptPick>): PickRow[] {
  return preview.map((r) => {
    const base: PickRow = {
      userId: r.userId,
      name: r.name,
      slug: r.slug,
      title: r.title,
      tier: personalTier,
      suggestedTier: r.suggestedTier,
      reason: r.reason,
      swapped: false,
    };
    const k = kept[r.userId];
    if (!k || (k.slug === base.slug && k.tier === base.tier)) return base;
    return { ...base, slug: k.slug, title: k.title, tier: k.tier, reason: "已更换", swapped: true };
  });
}

export function keptFromRows(rows: PickRow[] | null): Record<string, KeptPick> {
  const out: Record<string, KeptPick> = {};
  for (const r of rows ?? []) if (r.swapped) out[r.userId] = { slug: r.slug, title: r.title, tier: r.tier };
  return out;
}

export function keptFromSaved(saved: Record<string, PersonalPick>, articles: LibraryArticle[]): Record<string, KeptPick> {
  const out: Record<string, KeptPick> = {};
  for (const [uid, p] of Object.entries(saved)) {
    out[uid] = { slug: p.slug, title: titleOf(articles.find((a) => a.slug === p.slug), p.slug), tier: p.tier };
  }
  return out;
}

export function swapPick(rows: PickRow[], userId: string, next: { slug: string; tier: number | null }, articles: LibraryArticle[]): PickRow[] {
  return rows.map((r) =>
    r.userId === userId
      ? { ...r, slug: next.slug, title: titleOf(articles.find((a) => a.slug === next.slug), next.slug), tier: next.tier, reason: "已更换", swapped: true }
      : r,
  );
}

export function visiblePickRows(rows: PickRow[], recipientIds: string[]): PickRow[] {
  const keep = new Set(recipientIds);
  return rows.filter((r) => keep.has(r.userId));
}

export function pickTierText(row: PickRow): string {
  return tierLabel(row.tier ?? row.suggestedTier);
}

/** Discipline tags that appear on library articles, each once, in library order. */
export function disciplineOptions(articles: LibraryArticle[]): LibraryTag[] {
  const seen = new Set<string>();
  const out: LibraryTag[] = [];
  for (const a of articles) {
    for (const t of Array.isArray(a.tags) ? a.tags : []) {
      if (!seen.has(t.id)) {
        seen.add(t.id);
        out.push(t);
      }
    }
  }
  return out;
}

export function recipientReadingText(reading: RecipientReading | null): string {
  if (!reading) return "—";
  if (reading.state === "pending") return "待推荐";
  return `${reading.title || reading.slug} · ${tierLabel(reading.tier)}`;
}

export function assignmentFileName(a: { kind: AssignmentKind; payload: Record<string, unknown> }): string | null {
  if (a.kind !== "reading" || a.payload.source !== "text") return null;
  const name = str(a.payload.fileName).trim();
  return name || null;
}
```

In `settingsSummary`, replace the reading branch:

```ts
  if (kind === "reading") {
    if (d.readingSource === "library") return ["分级阅读库", articleTitle || d.slug || "—", tierLabel(d.tier)].join(" · ");
    if (d.readingSource === "url") return "链接";
    if (d.readingSource === "file") return `上传文件 · ${d.fileName} · ${runes(d.text)} 字`;
    if (d.readingSource === "personalized") {
      const parts = ["个性化阅读", tierLabel(d.personalTier)];
      if (d.disciplines.length) parts.push(`学科筛选 ${d.disciplines.length} 项`);
      return parts.join(" · ");
    }
    return `正文 · ${runes(d.text)} 字`;
  }
```
- [ ] **Step 7: Run the tests and typecheck**

Run: `cd apps/lite-web && pnpm vitest run src/api/assignments.test.ts src/teacher/assignmentLogic.test.ts && pnpm typecheck`
Expected: vitest PASS. `pnpm typecheck` lists every other `RecipientDTO` object literal missing `reading` (fixtures in other tests, e.g. Part B's grading tests); add `reading: null` to each and rerun until clean. `SettingsFields` still compiles: the new sources render nothing until Tasks 7 and 8.

- [ ] **Step 8: Commit**

```bash
git add apps/lite-web/src/api/assignments.ts apps/lite-web/src/api/assignments.test.ts \
  apps/lite-web/src/teacher/assignmentLogic.ts apps/lite-web/src/teacher/assignmentLogic.test.ts
git commit -m "feat(lite-web): payload, preview and pick-row logic for homework materials

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

(Add to `git add` any test fixture files Step 7 made you touch, by path.)

---

### Task 7: 上传文件 tab

**Files:**
- Create: `apps/lite-web/src/teacher/UploadSourceField.tsx`
- Modify: `apps/lite-web/src/teacher/AssignmentForm.tsx` (`SOURCE_OPTIONS`, `SettingsFields`, the `<SettingsFields>` call)
- Modify: `apps/lite-web/src/teacher/AssignmentDetailPage.tsx` (the edit form's `<SettingsFields>` call)
- Modify: `apps/lite-web/src/teacher/AssignmentsPage.tsx` (title cell)

**Interfaces:**
- Consumes: `extractDocument(file: File): Promise<{ title: string; text: string }>` (`api/writings.ts`); `readExtractResult`, `extractedCountText`, `fillTitleIfEmpty`, `failText`, `assignmentFileName` (Task 6).
- Produces:
  - `UploadSourceField({ text, fileName, onExtracted, onTextChange }: { text: string; fileName: string; onExtracted: (r: { text: string; fileName: string; title: string }) => void; onTextChange: (text: string) => void })`
  - `SettingsFields` gains the optional prop `onExtractedTitle?: (title: string) => void`.

No new vitest here: every rule this UI depends on is in Task 6. The UI is checked in Task 9.

- [ ] **Step 1: Create the upload field**

Create `apps/lite-web/src/teacher/UploadSourceField.tsx`:

```tsx
import { useState } from "react";
import { extractDocument } from "../api/writings";
import { useAlive } from "../shared/useAlive";
import { extractedCountText, failText, readExtractResult } from "./assignmentLogic";

const LABEL_CLS = "text-mk-label font-bold text-mk-muted";
const INPUT_CLS =
  "w-full rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-small text-mk-ink outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200";

/**
 * UploadSourceField — 上传文件 for a reading homework. The browser posts the
 * file to /documents/extract (lite edition, any role, 30 MB); the text lands
 * in the text source and nothing else is stored. The extracted text stays
 * editable so the teacher can check it before 发布.
 *
 * Classes are inlined rather than imported from AssignmentForm, which imports
 * this file.
 */
export function UploadSourceField({
  text,
  fileName,
  onExtracted,
  onTextChange,
}: {
  text: string;
  fileName: string;
  onExtracted: (r: { text: string; fileName: string; title: string }) => void;
  onTextChange: (text: string) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const alive = useAlive();

  async function extract(file: File) {
    setBusy(true);
    setError(null);
    try {
      const out = readExtractResult(await extractDocument(file), file.name);
      if (!alive.current) return;
      if (out.ok) onExtracted(out);
      else setError(out.error);
    } catch (e) {
      if (alive.current) setError(failText("提取", e));
    } finally {
      if (alive.current) setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <label className="flex flex-col gap-1.5">
        <span className={LABEL_CLS}>文件</span>
        <input
          type="file"
          accept=".pdf,.docx,.txt,.md"
          disabled={busy}
          onChange={(e) => {
            const file = e.target.files?.[0];
            // Cleared so choosing the same file again runs the extraction again.
            e.target.value = "";
            if (file) void extract(file);
          }}
          className="text-mk-small text-mk-ink"
        />
      </label>
      <p className="text-mk-small text-mk-muted">
        支持 PDF、Word（.docx）、TXT、Markdown，不超过 30 MB。PDF 中的图片不会保留；扫描版 PDF 没有文字，无法提取。
      </p>
      {busy && <p className="text-mk-small text-mk-muted">提取中</p>}
      {error && (
        <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
          {error}
        </p>
      )}
      {!busy && fileName && text.trim() && (
        <p className="text-mk-small text-mk-ink">
          {fileName} · {extractedCountText(text)}
        </p>
      )}
      {fileName && (
        <label className="flex flex-col gap-1.5">
          <span className={LABEL_CLS}>正文</span>
          <textarea value={text} onChange={(e) => onTextChange(e.target.value)} rows={10} className={INPUT_CLS} />
        </label>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Wire it into the settings**

In `apps/lite-web/src/teacher/AssignmentForm.tsx`:

1. Import `import { UploadSourceField } from "./UploadSourceField";` and add `fillTitleIfEmpty` to the `./assignmentLogic` import.
2. Add `{ value: "file", label: "上传文件" },` to `SOURCE_OPTIONS` after `正文`.
3. Give `SettingsFields` the prop (keep every prop Part B added):

```tsx
export function SettingsFields({
  value,
  onChange,
  onExtractedTitle,
}: {
  value: SettingsDraft;
  onChange: (update: (d: SettingsDraft) => SettingsDraft) => void;
  /** Called with the uploaded document's title; the page fills 标题 only when it is empty. */
  onExtractedTitle?: (title: string) => void;
}) {
```

4. After the `value.readingSource === "text"` block, add:

```tsx
          {value.readingSource === "file" && (
            <UploadSourceField
              text={value.text}
              fileName={value.fileName}
              onExtracted={(r) => {
                onChange((d) => ({ ...d, text: r.text, fileName: r.fileName }));
                onExtractedTitle?.(r.title);
              }}
              onTextChange={(text) => set({ text })}
            />
          )}
```

5. In `AssignmentForm`, pass the prop:

```tsx
          <SettingsFields
            value={draft}
            onChange={(update) => setDraft((d) => ({ ...d, ...update(d) }))}
            onExtractedTitle={(title) => setDraft((d) => ({ ...d, title: fillTitleIfEmpty(d.title, title) }))}
          />
```

In `apps/lite-web/src/teacher/AssignmentDetailPage.tsx`, add `fillTitleIfEmpty` to the `./assignmentLogic` import and pass to the edit form's `<SettingsFields>`:

```tsx
                  onExtractedTitle={(title) => setEdit((d) => (d ? { ...d, title: fillTitleIfEmpty(d.title, title) } : d))}
```

- [ ] **Step 3: Show the file name on the homework list**

In `apps/lite-web/src/teacher/AssignmentsPage.tsx`, add `assignmentFileName` to the `./assignmentLogic` import and replace the title cell:

```tsx
                    <td className="border-b border-mk-border px-3 py-3 text-mk-small font-bold text-mk-ink">
                      {a.title}
                      {assignmentFileName(a) && (
                        <span className="mt-0.5 block break-all font-normal text-mk-muted">{assignmentFileName(a)}</span>
                      )}
                    </td>
```

The detail header already shows it through `settingsSummary` (`上传文件 · {fileName} · {n} 字`).

- [ ] **Step 4: Typecheck and run the logic tests**

Run: `cd apps/lite-web && pnpm typecheck && pnpm vitest run src/teacher src/api`
Expected: typecheck clean; vitest PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/teacher/UploadSourceField.tsx apps/lite-web/src/teacher/AssignmentForm.tsx \
  apps/lite-web/src/teacher/AssignmentDetailPage.tsx apps/lite-web/src/teacher/AssignmentsPage.tsx
git commit -m "feat(lite-web): upload a document as reading homework

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: 个性化 tab, 更换 and the detail column

**Files:**
- Create: `apps/lite-web/src/teacher/PersonalizedPicker.tsx`
- Modify: `apps/lite-web/src/teacher/AssignmentForm.tsx` (`SOURCE_OPTIONS`, `SettingsFields`, the `<SettingsFields>` call)
- Modify: `apps/lite-web/src/teacher/AssignmentDetailPage.tsx` (edit form props, `buildPatchInput` call, 学生 table)

**Interfaces:**
- Consumes: `previewPersonalizedReading` (Task 6), `getLibraryShelf`, `LibraryPicker({slug, tier, onChange})`, and from `assignmentLogic`: `mergePickRows`, `keptFromRows`, `keptFromSaved`, `swapPick`, `visiblePickRows`, `pickTierText`, `disciplineOptions`, `tierLabel`, `toggleId`, `errorText`, `recipientReadingText`, `buildPatchInput(e, editable, recipientIds)`.
- Produces:
  - `PersonalizedPicker({ value, onChange, classId, recipientIds }: { value: SettingsDraft; onChange: (update: (d: SettingsDraft) => SettingsDraft) => void; classId: string; recipientIds: string[] })`
  - `SettingsFields` gains required props `classId: string` and `recipientIds: string[]`.

- [ ] **Step 1: Create the picker**

Create `apps/lite-web/src/teacher/PersonalizedPicker.tsx`:

```tsx
import { useEffect, useState, type ReactNode } from "react";
import { Button } from "@/ui";
import { previewPersonalizedReading } from "../api/assignments";
import { getLibraryShelf, type LibraryArticle } from "../api/library";
import { LibraryPicker } from "./LibraryPicker";
import {
  disciplineOptions,
  errorText,
  keptFromRows,
  keptFromSaved,
  mergePickRows,
  pickTierText,
  swapPick,
  tierLabel,
  toggleId,
  visiblePickRows,
  type PickRow,
  type SettingsDraft,
} from "./assignmentLogic";

/**
 * PersonalizedPicker — 个性化 reading homework: a filter (学科筛选, 难度),
 * the class preview (one article per checked student) and 更换 per row.
 *
 * The preview runs again when the filter or the class changes; rows the
 * teacher swapped, and picks saved on this homework, are kept (mergePickRows).
 * Classes are inlined rather than imported from AssignmentForm, which imports
 * this file.
 */
export function PersonalizedPicker({
  value,
  onChange,
  classId,
  recipientIds,
}: {
  value: SettingsDraft;
  onChange: (update: (d: SettingsDraft) => SettingsDraft) => void;
  classId: string;
  recipientIds: string[];
}) {
  const [articles, setArticles] = useState<LibraryArticle[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [nonce, setNonce] = useState(0);
  const [swapping, setSwapping] = useState<PickRow | null>(null);

  // The shelf gives the discipline options and the titles of saved picks. On
  // failure the filter is hidden and titles fall back to slugs.
  useEffect(() => {
    let cancelled = false;
    getLibraryShelf()
      .then((shelf) => {
        if (!cancelled) setArticles(Array.isArray(shelf?.articles) ? shelf.articles : []);
      })
      .catch(() => {
        if (!cancelled) setArticles([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const disciplinesKey = value.disciplines.join(",");
  const tier = value.personalTier;
  const ready = articles !== null;
  useEffect(() => {
    if (!classId || !ready) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    previewPersonalizedReading(classId, { disciplines: disciplinesKey ? disciplinesKey.split(",") : [], tier })
      .then((rows) => {
        if (cancelled) return;
        onChange((d) => ({
          ...d,
          picks: mergePickRows(rows, d.personalTier, { ...keptFromSaved(d.savedPicks, articles ?? []), ...keptFromRows(d.picks) }),
        }));
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorText(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // onChange is a new function on every render; the preview depends only on
    // the class, the filter and a retry.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [classId, disciplinesKey, tier, nonce, ready]);

  const options = articles ? disciplineOptions(articles) : [];
  const rows = value.picks ? visiblePickRows(value.picks, recipientIds) : null;

  return (
    <div className="flex flex-col gap-4">
      {options.length > 0 && (
        <div className="flex flex-col gap-1.5">
          <span className="text-mk-label font-bold text-mk-muted">学科筛选</span>
          <div className="flex flex-wrap gap-2" role="group" aria-label="学科筛选">
            {options.map((t) => (
              <Chip
                key={t.id}
                active={value.disciplines.includes(t.id)}
                onClick={() => onChange((d) => ({ ...d, disciplines: toggleId(d.disciplines, t.id) }))}
              >
                {t.zh}
              </Chip>
            ))}
          </div>
          <span className="text-mk-small text-mk-muted">不选表示不限学科</span>
        </div>
      )}

      <div className="flex flex-col gap-1.5">
        <span className="text-mk-label font-bold text-mk-muted">难度</span>
        <div className="flex flex-wrap gap-2" role="group" aria-label="难度">
          {[null, 1, 2, 3, 4, 5].map((t) => (
            <Chip key={t ?? "auto"} active={tier === t} onClick={() => onChange((d) => ({ ...d, personalTier: t }))}>
              {tierLabel(t)}
            </Chip>
          ))}
        </div>
      </div>

      {error ? (
        <div className="text-mk-small font-semibold text-mk-danger">
          加载失败：{error}{" "}
          <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
            重试
          </button>
        </div>
      ) : rows === null ? (
        <div className="text-mk-small text-mk-muted">加载中…</div>
      ) : rows.length === 0 ? (
        <div className="text-mk-small text-mk-muted">暂无学生</div>
      ) : (
        <div className="overflow-x-auto rounded-mk-lg border border-mk-border bg-mk-surface">
          <table className="w-full min-w-[640px] border-collapse">
            <thead>
              <tr>
                {["学生", "文章", "难度", "原因", "操作"].map((h) => (
                  <th key={h} className="whitespace-nowrap border-b border-mk-border px-3 py-2.5 text-left text-mk-label font-bold text-mk-muted">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.userId}>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small font-bold text-mk-ink">{r.name}</td>
                  <td className="border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">{r.title}</td>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">{pickTierText(r)}</td>
                  <td className="border-b border-mk-border px-3 py-3 text-mk-small text-mk-muted">{r.reason}</td>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small">
                    <Button variant="link" size="sm" onClick={() => setSwapping(r)}>
                      更换
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {loading && rows !== null && <span className="text-mk-small text-mk-muted">处理中</span>}

      {swapping && (
        <SwapDialog
          row={swapping}
          onClose={() => setSwapping(null)}
          onConfirm={(next) => {
            const userId = swapping.userId;
            onChange((d) => ({ ...d, picks: d.picks ? swapPick(d.picks, userId, next, articles ?? []) : d.picks }));
            setSwapping(null);
          }}
        />
      )}
    </div>
  );
}

function Chip({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={
        "rounded-mk-full border px-3 py-1.5 text-mk-small transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 " +
        (active ? "border-mk-accent font-bold text-mk-accent-700" : "border-mk-border text-mk-ink hover:bg-mk-accent-50")
      }
      style={active ? { background: "color-mix(in srgb, var(--mk-accent-500) 12%, var(--mk-surface))" } : undefined}
    >
      {children}
    </button>
  );
}

function SwapDialog({
  row,
  onClose,
  onConfirm,
}: {
  row: PickRow;
  onClose: () => void;
  onConfirm: (next: { slug: string; tier: number | null }) => void;
}) {
  const [choice, setChoice] = useState<{ slug: string; tier: number | null }>({ slug: row.slug, tier: row.tier });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: "color-mix(in srgb, var(--mk-ink) 42%, transparent)" }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="swap-dialog-title"
        className="flex max-h-full w-full max-w-[640px] flex-col gap-5 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-6 shadow-mk-lg"
      >
        <div className="flex flex-col gap-1.5">
          <h2 id="swap-dialog-title" className="text-mk-h2 text-mk-ink">
            更换文章
          </h2>
          <p className="text-mk-small text-mk-muted">{row.name}</p>
        </div>
        <LibraryPicker slug={choice.slug} tier={choice.tier} onChange={setChoice} />
        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" size="sm" onClick={() => onConfirm(choice)} disabled={!choice.slug}>
            确认选择
          </Button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Wire it into the settings and the create form**

In `apps/lite-web/src/teacher/AssignmentForm.tsx`:

1. `import { PersonalizedPicker } from "./PersonalizedPicker";`
2. Add `{ value: "personalized", label: "个性化" },` as the last entry of `SOURCE_OPTIONS`.
3. Add the two required props to `SettingsFields` (next to `onExtractedTitle`):

```tsx
  /** The class whose students the personalised preview covers. */
  classId: string;
  /** Checked recipients: the preview table and the saved picks keep only these. */
  recipientIds: string[];
```

4. After the `value.readingSource === "file"` block, add:

```tsx
          {value.readingSource === "personalized" && (
            <PersonalizedPicker value={value} onChange={onChange} classId={classId} recipientIds={recipientIds} />
          )}
```

5. In `AssignmentForm`'s `<SettingsFields>` call add `classId={draft.classId}` and `recipientIds={draft.userIds}`.

- [ ] **Step 3: Wire the detail page**

In `apps/lite-web/src/teacher/AssignmentDetailPage.tsx`:

1. Add `recipientReadingText` to the `./assignmentLogic` import.
2. In the edit form's `<SettingsFields>` add `classId={assignment.classId}` and `recipientIds={recipients.map((r) => r.userId)}`.
3. In `save()`, change `buildPatchInput(edit, editable)` to `buildPatchInput(edit, editable, recipients.map((r) => r.userId))`.
4. In the 学生 table (Part B may have moved it into the 学生 tab; find the header array `["学生", "状态", "开始时间", "完成时间", "操作"]`), add the 文章 column for a personalized homework. Before the `return (`:

```tsx
  const personalized = assignment?.kind === "reading" && assignment.payload.source === "personalized";
```

Header array:

```tsx
                      {["学生", ...(personalized ? ["文章"] : []), "状态", "开始时间", "完成时间", "操作"].map((h) => (
```

After the student-name `<td>`:

```tsx
                        {personalized && (
                          <td className="border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">{recipientReadingText(r.reading)}</td>
                        )}
```

If Part B passes the table its data through a child component, pass `personalized` down as a prop the same way the assignment kind is passed.

- [ ] **Step 4: Typecheck and run the logic tests**

Run: `cd apps/lite-web && pnpm typecheck && pnpm vitest run src/teacher src/api`
Expected: typecheck clean (every `<SettingsFields>` call now passes `classId` and `recipientIds`); vitest PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/teacher/PersonalizedPicker.tsx apps/lite-web/src/teacher/AssignmentForm.tsx \
  apps/lite-web/src/teacher/AssignmentDetailPage.tsx
git commit -m "feat(lite-web): personalised reading preview, 更换 and each student's article

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Full suites and screenshots

**Files:**
- Create (one-off, deleted in Step 6): `apps/lite-web/e2e/materials-shots.spec.ts`
- Output: `.superpowers/tmp/materials-shots/*.png`

**Interfaces:**
- Consumes: everything above; `apps/lite-web/e2e/run-stack.sh` (throwaway Postgres, API, lite dev server; takes a spec file name).

- [ ] **Step 1: Run every suite this plan touches**

Check `docker ps` first: if another agent's e2e stack is running, wait for it.

```bash
cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./internal/api/ ./internal/library/ ./internal/liteassign/ && \
  go test ./internal/library ./internal/liteassign -count=1 && \
  CGO_ENABLED=0 go test ./internal/api -count=1 -timeout 1800s 2>&1 | tail -5
cd ../lite-web && pnpm typecheck && pnpm vitest run 2>&1 | tail -5
```

Expected: `ok` for each Go package; vitest all passed. Read the last lines for `ok`/`FAIL` (the pipe hides `go test`'s exit code).

- [ ] **Step 2: Write the screenshot harness**

Create `apps/lite-web/e2e/materials-shots.spec.ts`:

```ts
import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { expect, test, type APIResponse, type Browser, type Page } from "@playwright/test";

// One-off: screenshots of the 上传文件 tab (extracted and over the limit), the
// homework list with a file name, the 个性化 preview (1440 and 390), the 更换
// dialog and the detail page's 文章 column. Not a regression test; deleted
// after looking at the pictures.

const PG = process.env.E2E_PG_CONTAINER ?? "mindimprint-lite-e2e-pg";
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:5174";
const JOIN = process.env.E2E_JOIN_CODE ?? "DEMO-0001";
const OUT = "../../.superpowers/tmp/materials-shots";

const ARTICLE = [
  "校园积水调查",
  "学校后门那片空地一下雨就积水。城市里的雨水花园用下凹的绿地先把雨水接住，再慢慢渗到地下。",
  "监测数据显示，改造后路面积水时间缩短了一半以上。但雨水花园需要定期清理落叶，否则会堵住。",
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

async function account(browser: Browser, label: string, name: string) {
  const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
  const email = `shot-${label}-${tag}@demo.mindimprint.local`;
  const password = `shot-${tag}-pass`;
  const ctx = await browser.newContext({ baseURL: BASE_URL });
  await ok(ctx.request.post("/api/v1/auth/signup", { data: { email, password, display_name: name, join_code: JOIN } }));
  await ok(ctx.request.post("/api/v1/auth/signin", { data: { email, password } }));
  return { ctx, email, password, id: "" };
}

async function shoot(page: Page, name: string, width: number) {
  await page.setViewportSize({ width, height: width > 1000 ? 1000 : 844 });
  await page.waitForTimeout(300);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow, `${name}: page scrolls horizontally by ${overflow}px`).toBeLessThanOrEqual(0);
  await page.screenshot({ path: `${OUT}/${width}-${name}.png`, fullPage: true });
}

async function fillCommon(page: Page, title: string) {
  await page.getByLabel("标题", { exact: true }).fill(title);
  await page.getByLabel("截止时间（北京时间）").fill("2030-01-01T22:00");
}

test("homework materials screenshots", async ({ browser }) => {
  mkdirSync(OUT, { recursive: true });
  const teacher = await account(browser, "teacher", "王老师");
  const phoebe = await account(browser, "phoebe", "Phoebe");
  const lin = await account(browser, "lin", "林知遥");

  teacher.id = psql(`SELECT id FROM users WHERE email = '${teacher.email}'`);
  psql(`UPDATE users SET role = 'teacher' WHERE id = '${teacher.id}'`);
  psql(`UPDATE enrollments SET role_in_class = 'teacher' WHERE user_id = '${teacher.id}'`);
  await ok(teacher.ctx.request.post("/api/v1/auth/signin", { data: { email: teacher.email, password: teacher.password } }));

  // Phoebe has interest in astronomy; 林知遥 opened the first article at 高阶 and left it.
  phoebe.id = psql(`SELECT id FROM users WHERE email = '${phoebe.email}'`);
  const kid = psql(
    `INSERT INTO interest_keyword (user_id, text_zh, text_en, norm, field, strength, note) VALUES ('${phoebe.id}', '日食', 'eclipse', '日食', 'science', 4, '') RETURNING id`,
  );
  psql(`INSERT INTO keyword_discipline (keyword_id, discipline_id, confidence, how, rationale) VALUES ('${kid}', 'astronomy', 1.0, 'alias', '')`);
  const shelf = await (await ok(lin.ctx.request.get("/api/v1/library"))).json();
  await ok(lin.ctx.request.post(`/api/v1/library/${shelf.articles[0].slug}/levels/4`));

  const tp = await teacher.ctx.newPage();
  await tp.setViewportSize({ width: 1440, height: 1000 });

  // 1. Upload: extracted, then over the limit.
  await tp.goto("/assignments/new");
  await tp.getByRole("button", { name: "上传文件", exact: true }).click();
  await tp.locator('input[type="file"]').setInputFiles({ name: "校园积水调查.txt", mimeType: "text/plain", buffer: Buffer.from(ARTICLE) });
  await tp.getByText(/已提取 \d+ 字/).waitFor();
  await expect(tp.getByLabel("标题", { exact: true })).toHaveValue("校园积水调查");
  await shoot(tp, "upload-extracted", 1440);
  await tp.locator('input[type="file"]').setInputFiles({ name: "too-long.txt", mimeType: "text/plain", buffer: Buffer.from("雨".repeat(50001)) });
  await tp.getByText("提取失败：正文超过 50000 字").waitFor();
  await shoot(tp, "upload-too-long", 1440);
  await tp.locator('input[type="file"]').setInputFiles({ name: "校园积水调查.txt", mimeType: "text/plain", buffer: Buffer.from(ARTICLE) });
  await tp.getByText(/已提取 \d+ 字/).waitFor();
  await fillCommon(tp, "第三周阅读");
  await tp.getByRole("button", { name: "发布作业" }).click();
  await tp.waitForURL(/\/assignments\/(?!new)/);
  await tp.goto("/assignments");
  await tp.getByText("校园积水调查.txt").waitFor();
  await shoot(tp, "assignments-list", 1440);

  // 2. Personalised: preview at 1440 and 390, 更换, publish, detail.
  await tp.setViewportSize({ width: 1440, height: 1000 });
  await tp.goto("/assignments/new");
  await tp.getByRole("button", { name: "个性化", exact: true }).click();
  await tp.getByRole("columnheader", { name: "原因" }).waitFor();
  await tp.getByText(/兴趣相关：/).first().waitFor();
  await shoot(tp, "personalized-preview", 1440);
  await shoot(tp, "personalized-preview", 390);
  await tp.setViewportSize({ width: 1440, height: 1000 });
  await tp.getByRole("button", { name: "更换", exact: true }).first().click();
  const dialog = tp.getByRole("dialog", { name: "更换文章" });
  await dialog.getByRole("option").nth(2).click();
  await shoot(tp, "personalized-swap", 1440);
  await dialog.getByRole("button", { name: "确认选择" }).click();
  await tp.getByText("已更换").waitFor();
  await fillCommon(tp, "个性化阅读第一周");
  await tp.getByRole("button", { name: "发布作业" }).click();
  await tp.waitForURL(/\/assignments\/(?!new)/);
  await tp.getByRole("columnheader", { name: "文章" }).waitFor();
  await shoot(tp, "detail-personalized", 1440);
});
```

- [ ] **Step 3: Run it**

```bash
E2E_API_PORT=8092 E2E_WEB_PORT=5185 E2E_PG_PORT=55444 bash apps/lite-web/e2e/run-stack.sh materials-shots.spec.ts
```

Expected: `1 passed`, and in `.superpowers/tmp/materials-shots/`: `1440-upload-extracted.png`, `1440-upload-too-long.png`, `1440-assignments-list.png`, `1440-personalized-preview.png`, `390-personalized-preview.png`, `1440-personalized-swap.png`, `1440-detail-personalized.png`.

If a locator times out, open the trace (`apps/lite-web/test-results/…/trace.zip`, `npx playwright show-trace`) and fix the locator before changing product code. If the seeded `DEMO-0001` class has other students, the preview simply has more rows.

- [ ] **Step 4: Look at every picture**

Open each PNG with the Read tool and check:
- `upload-extracted`: tabs 分级阅读库 / 链接 / 正文 / 上传文件 / 个性化 with 上传文件 active; the hint about PDF images and scanned PDFs; `校园积水调查.txt · 已提取 N 字`; 标题 filled; the 正文 textarea holds the article.
- `upload-too-long`: 「提取失败：正文超过 50000 字」 in the danger colour; the previous text is still in the textarea.
- `assignments-list`: 第三周阅读 with `校园积水调查.txt` on a muted second line.
- `personalized-preview` 1440: 学科筛选 chips, 难度 chips with 按学生当前水平 active, table 学生 / 文章 / 难度 / 原因 / 操作; Phoebe's reason 「兴趣相关：天文与宇宙学」; 林知遥 at 进阶 with 「暂无兴趣数据，按难度推荐」 and not the article she opened; 更换 on each row.
- `personalized-preview` 390: no horizontal page scroll (asserted); the table scrolls inside its own box.
- `personalized-swap`: dialog 更换文章 with the student's name, the library list and 难度, buttons 取消 / 确认选择.
- `detail-personalized`: header summary `个性化阅读 · 按学生当前水平`; 学生 table with a 文章 column showing `{title} · {tier}` per student.
- No coloured left bars; tints use `color-mix`; no Tailwind alpha classes on `mk-*` tokens; copy matches Global Constraints.

- [ ] **Step 5: Fix what the pictures show**

Fix each defect in the file that owns it (Tasks 7–8), rerun Step 3 and look again. Commit each fix on its own:

```bash
git add <the files you changed>
git commit -m "fix(lite-web): <what the screenshot showed>

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 6: Remove the harness**

```bash
rm apps/lite-web/e2e/materials-shots.spec.ts
git status --short apps/lite-web/e2e
```

Expected: nothing listed for `apps/lite-web/e2e`.

---

## Self-review

**Spec coverage (Part C):**
- C1 上传文件 tab (pdf, docx, txt, md) → Task 7. Browser posts to `POST /documents/extract` (checked: `liteOnly`, edition gate only, so a lite teacher passes; 30 MB `MaxBytesReader`) → Task 7 `UploadSourceField`. Fills `text` and the title if empty → Tasks 6 (`readExtractResult`, `fillTitleIfEmpty`; file stem fallback = Deviation 8) and 7. 「已提取 {n} 字」 → `extractedCountText`. 「提取失败：{后台原话}」 → `failText("提取", e)`. Over 50000 → 「提取失败：正文超过 50000 字」 in `readExtractResult`, tested at 50000/50001. Optional `fileName` ≤200 → Task 1 (server) and Task 6 (client cap). File name on the homework list → Task 7 Step 3. Form hint on PDF images and scanned PDFs → Task 7.
- C2 `personalized` source shape → Task 1. Preview endpoint with every enrolled student, existing recommender per student (profile by user id), opened articles excluded, disciplines filter, tier override else SuggestTier, code-built reasons including the empty-interest and filter fallback cases → Tasks 2, 3 (route `{id}` = Deviation 1, wrapper = Deviation 2, `suggestedTier` = Deviation 3, extra reasons = Deviation 4). 个性化 tab with the preview table and 更换 via `LibraryPicker` per student, submit saves `picks` → Tasks 6, 8. Validation: slug exists (Task 1), pick user is a recipient (Task 4, create and PATCH). Start: pick → library start at its slug and tier, SuggestTier when null; no pick → recommendation at start → Task 4. Detail shows each student's article title and tier → Task 5, Task 8 Step 3. Ownership with other-teacher 404 → Tasks 3 and 5 (the create/PATCH/start routes keep their existing ownership tests).
- §5 testing item "personalised preview selection (filter, fallback, exclusion, tier)" → Task 2 unit tests, Task 3 handler test. UI screenshot of the personalised preview → Task 9.
- Refactor of the strength function to take a user id while the shelf keeps its behaviour → Task 3 (`interestDisciplinesIn`, pinned by `TestLibraryShelfStillRecommendsFromHerInterests`).
- Migrations: none needed (payload is `jsonb`; preview reuses `ListLiteWeekClassStudents`; detail reuses `GetReading`).

**Placeholder scan:** every code step carries its code. Two steps locate an insertion point by content rather than line number because Part B edits those files first: Task 8 Step 3 (the 学生 table header array) and Task 4 Step 3 (after the `addIDs` loop). Task 6 Step 7 and Task 5 Step 4 tell the executor how to find fixtures that break on the new `reading` field (typecheck output, `grep returnNote`).

**Type consistency:** Go — `PersonalPick{Slug, Tier *int}`, `ReadingPayload.{FileName, Disciplines, Picks}`, `ValidateDisciplines`, `PicksOutside(payload, []uuid.UUID)` (Task 1) match Tasks 3–5. `PickForStudent(articles, Profile, filter) (StudentPick, bool)` and `StudentPick.Reason()` (Task 2) match Tasks 3, 4 and the tests. `interestDisciplinesIn` / `libraryProfileIn(ctx, q, userID)` (Task 3) match Task 4's `personalizedTargetIn(ctx, qtx, userID, p)`. `RecipientReadingDTO{Slug, Title, Tier *int, State}` (Task 5) matches the wire `RecipientReading` in Task 6. TypeScript — `PreviewRow`, `PickRow`, `KeptPick`, `PersonalPick`, `RecipientReading`, `mergePickRows(preview, personalTier, kept)`, `swapPick(rows, userId, next, articles)`, `buildPayload(d, recipientIds?)`, `buildPatchInput(e, editable, recipientIds?)` are defined in Task 6 and used with the same signatures in Tasks 7–8. `SettingsFields` props `onExtractedTitle` (Task 7), `classId` and `recipientIds` (Task 8) are passed at both call sites.


# Course Catalog: Taxonomy, Structured Intro & Home Curation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add course `category` (controlled 7-vocab), a schema-driven `introduction`, and `featuredRank` home-page curation to the course catalog, split so the student-end owns taxonomy/presentation and the generator owns content production.

**Architecture:** Three nullable columns on the existing `course` table carry the new metadata; they ride out on the existing `CourseSummary` (list endpoint) so no per-course fetch is needed. The generator writes `category`/`introduction` through the existing `PUT /admin/courses/{slug}/definition` (border-validated); the student-end reads them to group the all-courses page by category, cap the home page at 6 via `featuredRank` (random fallback), and render a new course-detail landing page. The `packages/contracts` course contract is the single seam both sides consume.

**Tech Stack:** Go (`net/http` + `pgx`/`sqlc` + `goose`), PostgreSQL, TypeScript/Zod contracts, React + Vite.

**Spec:** `docs/superpowers/specs/2026-08-19-course-catalog-taxonomy-and-intro-design.md` — the plan argues from the spec; executors read both.

## Global Constraints

- **Categories are a closed vocabulary of exactly 7, NO emoji**, stored as `{slug, label}` pairs. The 7 slugs (verbatim): `stance-value` 立场与价值 · `source-check` 信源核查 · `media-literacy` 媒介与信息素养 · `self-knowledge` 自我认知 · `data-literacy` 数据素养 · `research-process` 研究流程 · `argument-writing` 论证写作. The generator MUST NOT mint an 8th; the API rejects any category not in the 7.
- **All three new fields are nullable / default-null.** Existing live courses must not break before backfill: `category=null` → 未分类 group; `introduction=null` → detail page falls back to `blurb`; `featuredRank=null` → participates in random fill.
- **`card_ids` validation stays strict** — every id must resolve in the 34-card registry (`cards.ByID`); this plan does not weaken it.
- **`featuredRank` is student-end/product-owned, NOT written by the generator.** The definition-upsert path writes only `category` + `introduction`; it never touches `featured_rank`.
- **The generator drafts; publishing stays teacher-gated** via the existing `preview`→`ship` lifecycle. No task here auto-publishes.
- **No engagement ranking (铁律②):** the home page is featured-rank + random fallback only — no personalization, no popularity, no recency ranking.
- **sqlc config** is `emit_pointers_for_null_types: true`: nullable `text`→`*string`, nullable `int`→`*int32`, `jsonb`→`[]byte` (nil = SQL NULL). Regenerate with the pinned sqlc (see Task 2), never hand-edit generated files unless the task says so.
- Course DTO JSON keys mirror the contract exactly: `category`, `introduction`, `featuredRank` (camelCase), alongside the existing snake_case summary fields.

---

### Task 1: Migration 0075 — category / introduction / featured_rank columns

**Files:**
- Create: `apps/api/internal/store/migrations/0075_course_catalog_metadata.sql`

**Interfaces:**
- Produces: three nullable columns on `course` — `category text`, `introduction jsonb`, `featured_rank int` — consumed by every later backend task.

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
-- Course catalog metadata (spec 2026-08-19): three nullable columns on the
-- existing course row. `category` is one of the 7 controlled slugs (validated
-- app-side, not a DB CHECK, so the vocabulary lives in one place — the TS
-- contract). `introduction` is the schema-driven course intro document
-- (hook/whatYouDo/takeaways/alignment/keywords), stored verbatim as jsonb and
-- border-validated app-side. `featured_rank` drives home-page curation (lower =
-- earlier; NULL = not featured → random fill). All nullable so existing/seeded
-- rows need no backfill to keep rendering.
ALTER TABLE course
  ADD COLUMN category      text,
  ADD COLUMN introduction  jsonb,
  ADD COLUMN featured_rank int;

-- +goose Down
ALTER TABLE course DROP COLUMN IF EXISTS featured_rank;
ALTER TABLE course DROP COLUMN IF EXISTS introduction;
ALTER TABLE course DROP COLUMN IF EXISTS category;
```

- [ ] **Step 2: Apply the migration against a scratch DB to verify it parses**

Run (from `apps/api`): `go test ./internal/store/... -run TestMigrations -count=1` (the store package's migration test applies every goose file in order against a testcontainer Postgres).
Expected: PASS — 0075 applies cleanly on top of 0074.

If there is no such migration test, verify by running the full store suite in Task 2 after sqlc regen; the goose files are applied at container startup there.

- [ ] **Step 3: Commit**

```bash
git add apps/api/internal/store/migrations/0075_course_catalog_metadata.sql
git commit -m "feat(course): migration 0075 — category/introduction/featured_rank columns"
```

---

### Task 2: sqlc — read/write the new columns + store mapping

**Files:**
- Modify: `apps/api/internal/store/queries/course.sql` (ListCourseRows SELECT; UpsertCourseDefinition INSERT/ON CONFLICT)
- Regenerate: `apps/api/internal/store/sqlc/course.sql.go` (via sqlc, do not hand-edit)
- Modify: `apps/api/internal/agent/coursestore.go` (`CourseSummaryRow` + `ListCourses` mapping; `UpsertCourseDefinitionInput` + `UpsertCourseDefinition`)
- Test: `apps/api/internal/agent/coursestore_test.go`

**Interfaces:**
- Consumes: the columns from Task 1.
- Produces:
  - `agent.CourseSummaryRow` gains `Category *string`, `Introduction []byte`, `FeaturedRank *int32`.
  - `agent.UpsertCourseDefinitionInput` gains `Category *string`, `Introduction []byte`.

- [ ] **Step 1: Update the two queries in `course.sql`**

In `ListCourseRows`, add the three columns to the SELECT (keep the WHERE/ORDER BY):

```sql
-- name: ListCourseRows :many
SELECT slug, branch, title, blurb, time_label, card_ids, step_count, status, cover,
       category, introduction, featured_rank
FROM course
WHERE status = 'published' OR sqlc.arg(include_preview)::bool
ORDER BY branch, title;
```

In `UpsertCourseDefinition`, add `category` + `introduction` to the column list and the conflict update (do NOT add `featured_rank` — student-end owned per Global Constraints). Params shift: definition is now `$7`, category `$8`, introduction `$9`:

```sql
-- name: UpsertCourseDefinition :one
INSERT INTO course (slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache, course_definition, category, introduction, status, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,0,'{}','{}',$7,$8,$9,'preview', now())
ON CONFLICT (slug) DO UPDATE SET
  branch = EXCLUDED.branch, title = EXCLUDED.title, blurb = EXCLUDED.blurb,
  time_label = EXCLUDED.time_label, card_ids = EXCLUDED.card_ids,
  course_definition = EXCLUDED.course_definition,
  category = EXCLUDED.category, introduction = EXCLUDED.introduction, updated_at = now()
RETURNING slug, status;
```

- [ ] **Step 2: Regenerate sqlc**

Run (from `apps/api`): `CGO_ENABLED=0 go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate` (the pinned version this repo regenerates with — see memory `course-v2-showlogic`; a different minor silently reshuffles output).
Expected: `internal/store/sqlc/course.sql.go` now has `Category *string`, `Introduction []byte`, `FeaturedRank *int32` on `ListCourseRowsRow`, and `Category *string` + `Introduction []byte` on `UpsertCourseDefinitionParams`. Verify by grep:

Run: `grep -nE 'FeaturedRank|Introduction|Category' apps/api/internal/store/sqlc/course.sql.go`
Expected: the pointer/[]byte fields appear on both structs.

- [ ] **Step 3: Extend `CourseSummaryRow` and its mapping in `coursestore.go`**

Add to the `CourseSummaryRow` struct (after `Cover string`):

```go
	Category     *string
	Introduction []byte
	FeaturedRank *int32
```

In `ListCourses`, extend the mapping to copy them:

```go
		out = append(out, CourseSummaryRow{
			Slug: r.Slug, Branch: r.Branch, Title: r.Title, Blurb: r.Blurb,
			TimeLabel: r.TimeLabel, CardIDs: r.CardIds, StepCount: int(r.StepCount),
			Status: r.Status, Cover: r.Cover,
			Category: r.Category, Introduction: r.Introduction, FeaturedRank: r.FeaturedRank,
		})
```

- [ ] **Step 4: Extend `UpsertCourseDefinitionInput` and `UpsertCourseDefinition`**

Add to the `UpsertCourseDefinitionInput` struct (find it near line 640):

```go
	Category     *string
	Introduction []byte
```

In `UpsertCourseDefinition`, pass them through to `sqlc.UpsertCourseDefinitionParams` (add after the existing fields):

```go
		Category:      in.Category,
		Introduction:  in.Introduction,
```

- [ ] **Step 5: Write a store test proving round-trip of the new fields**

Add to `coursestore_test.go` a test that upserts a definition WITH a category + introduction and reads it back through `ListCourses`:

```go
func TestUpsertCourseDefinitionPersistsCatalogMetadata(t *testing.T) {
	ctx := context.Background()
	store, cleanup := newTestStore(t) // use whatever helper the file already uses to get a *sqlcAgentStore + testcontainer
	defer cleanup()

	cat := "source-check"
	intro := []byte(`{"hook":"h","takeaways":["a","b"]}`)
	if _, err := store.UpsertCourseDefinition(ctx, UpsertCourseDefinitionInput{
		Slug: "cat-meta", Branch: "Runtime", Title: "T", Blurb: "b",
		CardIDs: []string{"craap"}, Definition: []byte(`{"schemaVersion":"2.0","course":{"id":"cat-meta","title":"T"}}`),
		Category: &cat, Introduction: intro,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	rows, err := store.ListCourses(ctx, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var got *CourseSummaryRow
	for i := range rows {
		if rows[i].Slug == "cat-meta" {
			got = &rows[i]
		}
	}
	if got == nil {
		t.Fatal("course not listed")
	}
	if got.Category == nil || *got.Category != "source-check" {
		t.Fatalf("category = %v, want source-check", got.Category)
	}
	if len(got.Introduction) == 0 {
		t.Fatal("introduction not persisted")
	}
}
```

Match the test's store-construction helper to whatever `coursestore_test.go` already uses (see `TestListCoursesReturnsSummaries` or similar at the top of the file); do not invent a new harness.

- [ ] **Step 6: Run the store + agent suites**

Run (from `apps/api`): `go test ./internal/agent/... -run 'Course' -count=1 -timeout 1800s` (testcontainers need the long timeout — see memory `revision-recording-and-walk-redo`).
Expected: PASS, including the new `TestUpsertCourseDefinitionPersistsCatalogMetadata`.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/store/queries/course.sql apps/api/internal/store/sqlc/course.sql.go apps/api/internal/agent/coursestore.go apps/api/internal/agent/coursestore_test.go
git commit -m "feat(course): persist category/introduction through the store layer"
```

---

### Task 3: Contract — category vocabulary, introduction schema, extended CourseSummary

**Files:**
- Modify: `packages/contracts/src/course.ts`
- Test: `packages/contracts/test/course.test.ts`

**Interfaces:**
- Produces (consumed by Task 4 shape-match and all web tasks):
  - `COURSE_CATEGORIES: readonly { slug, label }[]` (7 entries, declared order).
  - `CourseCategory` = `z.enum([...7 slugs])`.
  - `CourseAlignment`, `CourseIntroduction` Zod objects + inferred types.
  - `CourseSummary` gains `category` (nullable enum), `introduction` (nullable object), `featuredRank` (nullable int).

- [ ] **Step 1: Write failing tests**

Add to `packages/contracts/test/course.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { CourseSummary, CourseCategory, COURSE_CATEGORIES, CourseIntroduction } from "../src/course";

describe("course catalog metadata", () => {
  it("has exactly the 7 no-emoji categories in declared order", () => {
    expect(COURSE_CATEGORIES.map((c) => c.slug)).toEqual([
      "stance-value", "source-check", "media-literacy",
      "self-knowledge", "data-literacy", "research-process", "argument-writing",
    ]);
    for (const c of COURSE_CATEGORIES) {
      expect(c.label).not.toMatch(/\p{Emoji_Presentation}/u);
    }
  });

  it("rejects a category outside the 7", () => {
    expect(CourseCategory.safeParse("misc").success).toBe(false);
    expect(CourseCategory.safeParse("source-check").success).toBe(true);
  });

  it("defaults the three new summary fields to null when absent", () => {
    const s = CourseSummary.parse({
      slug: "x", branch: "A", title: "T", blurb: "b", time_label: "10m",
      card_ids: [], step_count: 3,
    });
    expect(s.category).toBeNull();
    expect(s.introduction).toBeNull();
    expect(s.featuredRank).toBeNull();
  });

  it("parses a full introduction with alignment + takeaways", () => {
    const intro = CourseIntroduction.parse({
      hook: "h", whatYouDo: "w", takeaways: ["t1"],
      alignment: { ib: ["TOK"], otherIntl: ["AP"], domestic: ["语文"] },
      keywords: ["k"],
    });
    expect(intro.takeaways).toEqual(["t1"]);
    expect(intro.alignment.ib).toEqual(["TOK"]);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run (from `packages/contracts`): `npx vitest run test/course.test.ts`
Expected: FAIL — `COURSE_CATEGORIES`, `CourseCategory`, `CourseIntroduction` are not exported yet.

- [ ] **Step 3: Implement in `course.ts`**

Add near the top (after the imports, before `CourseSummary`):

```ts
// The 7 course categories — a closed, no-emoji vocabulary stored as
// {slug, label} pairs so a stored slug is decoupled from its display label
// (renaming a label never migrates data). The generator may SUGGEST a slug but
// must never mint an 8th; the API rejects any slug outside this set.
export const COURSE_CATEGORIES = [
  { slug: "stance-value",     label: "立场与价值" },
  { slug: "source-check",     label: "信源核查" },
  { slug: "media-literacy",   label: "媒介与信息素养" },
  { slug: "self-knowledge",   label: "自我认知" },
  { slug: "data-literacy",    label: "数据素养" },
  { slug: "research-process", label: "研究流程" },
  { slug: "argument-writing", label: "论证写作" },
] as const;

export const CourseCategory = z.enum([
  "stance-value", "source-check", "media-literacy",
  "self-knowledge", "data-literacy", "research-process", "argument-writing",
]);
export type CourseCategory = z.infer<typeof CourseCategory>;

// 学科对标 — curriculum alignment, three tracks (IB / other international /
// domestic). Each an independent list of short labels.
export const CourseAlignment = z.object({
  ib: z.array(z.string()).default([]),
  otherIntl: z.array(z.string()).default([]),
  domestic: z.array(z.string()).default([]),
});
export type CourseAlignment = z.infer<typeof CourseAlignment>;

// The schema-driven course introduction (rendered deterministically on the
// detail page; filled by the generator). Mirrors docs/2026-08-19-courses.md's
// per-course structure: 导语 / 学生做什么 / 带走什么 / 学科对标 / 关键词.
export const CourseIntroduction = z.object({
  hook: z.string().default(""),
  whatYouDo: z.string().default(""),
  takeaways: z.array(z.string()).default([]),
  alignment: CourseAlignment.default({ ib: [], otherIntl: [], domestic: [] }),
  keywords: z.array(z.string()).default([]),
});
export type CourseIntroduction = z.infer<typeof CourseIntroduction>;
```

Then extend `CourseSummary` — add these three fields inside the existing object (after `coverUrl`):

```ts
  category: CourseCategory.nullable().default(null),
  introduction: CourseIntroduction.nullable().default(null),
  featuredRank: z.number().int().nullable().default(null),
```

- [ ] **Step 4: Run tests to verify they pass**

Run (from `packages/contracts`): `npx vitest run test/course.test.ts`
Expected: PASS.

- [ ] **Step 5: Typecheck the package (guard the export barrel)**

Run (from `packages/contracts`): `npm run typecheck`
Expected: PASS. Note (from memory `course-runtime-2.0-build`): `test/library.test.ts` has a PRE-EXISTING typecheck failure unrelated to this change — if it is the ONLY failure, it is not yours; do not fix it here. Any failure touching `course.ts` IS yours.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/course.ts packages/contracts/test/course.test.ts
git commit -m "feat(contracts): course category vocab + introduction schema + summary fields"
```

---

### Task 4: Go DTO + list handler — emit the new summary fields

**Files:**
- Modify: `apps/api/internal/api/course_dto.go` (`courseSummaryDTO` + `toCourseSummaryDTO`)
- Test: `apps/api/internal/api/course_dto_test.go` (create if absent)

**Interfaces:**
- Consumes: `agent.CourseSummaryRow.{Category, Introduction, FeaturedRank}` (Task 2).
- Produces: the `/api/v1/courses` list JSON now carries `category` (string|null), `introduction` (object|null), `featuredRank` (number|null) on each summary — matching `CourseSummary` (Task 3).

- [ ] **Step 1: Write a failing DTO test**

Create `apps/api/internal/api/course_dto_test.go`:

```go
package api

import (
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
)

func TestCourseSummaryDTOEmitsCatalogMetadata(t *testing.T) {
	cat := "source-check"
	rank := int32(2)
	a := &API{} // resolveCoverURL tolerates a nil deps cover signer for an empty cover
	dto := a.toCourseSummaryDTO(agent.CourseSummaryRow{
		Slug: "x", Branch: "A", Title: "T", Blurb: "b", TimeLabel: "10m",
		Category: &cat, Introduction: []byte(`{"hook":"h"}`), FeaturedRank: &rank,
	})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"category":"source-check"`) {
		t.Fatalf("category missing: %s", s)
	}
	if !strings.Contains(s, `"introduction":{"hook":"h"}`) {
		t.Fatalf("introduction not verbatim: %s", s)
	}
	if !strings.Contains(s, `"featuredRank":2`) {
		t.Fatalf("featuredRank missing: %s", s)
	}
}

func TestCourseSummaryDTONullsWhenAbsent(t *testing.T) {
	a := &API{}
	dto := a.toCourseSummaryDTO(agent.CourseSummaryRow{Slug: "x", Branch: "A", Title: "T"})
	b, _ := json.Marshal(dto)
	s := string(b)
	if !strings.Contains(s, `"category":null`) || !strings.Contains(s, `"introduction":null`) || !strings.Contains(s, `"featuredRank":null`) {
		t.Fatalf("absent fields must serialize as null: %s", s)
	}
}
```

If constructing `&API{}` and calling `resolveCoverURL` with an empty cover is not safe with nil deps, set `Cover: ""` (already empty here) and confirm `resolveCoverURL("")` returns `""` without touching the signer — read `resolveCoverURL` first; if it dereferences deps unconditionally, build the DTO fields in the test by calling a small pure helper instead (see Step 2 note).

- [ ] **Step 2: Run to verify it fails**

Run (from `apps/api`): `go test ./internal/api/ -run TestCourseSummaryDTO -count=1`
Expected: FAIL — `courseSummaryDTO` has no `category`/`introduction`/`featuredRank`.

- [ ] **Step 3: Extend the DTO struct and mapper**

In `course_dto.go`, add to `courseSummaryDTO` (after `CoverURL`):

```go
	Category     *string         `json:"category"`
	Introduction json.RawMessage `json:"introduction"`
	FeaturedRank *int32          `json:"featuredRank"`
```

`json.RawMessage` with a nil value marshals to `null`; a non-nil value is emitted verbatim — exactly the object|null contract.

In `toCourseSummaryDTO`, set them in the returned struct:

```go
	return courseSummaryDTO{
		Slug: r.Slug, Branch: r.Branch, Title: r.Title, Blurb: r.Blurb,
		TimeLabel: r.TimeLabel, CardIDs: cardIDs, StepCount: r.StepCount,
		CoverURL:     a.resolveCoverURL(r.Cover),
		Category:     r.Category,
		Introduction: json.RawMessage(r.Introduction),
		FeaturedRank: r.FeaturedRank,
	}
```

(`json` is already imported in this file.)

- [ ] **Step 4: Run to verify it passes**

Run (from `apps/api`): `go test ./internal/api/ -run TestCourseSummaryDTO -count=1`
Expected: PASS. (If `resolveCoverURL` forced the helper split in Step 1, run that test instead.)

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/course_dto.go apps/api/internal/api/course_dto_test.go
git commit -m "feat(course): emit category/introduction/featuredRank on the list DTO"
```

---

### Task 5: Authoring API — accept & validate category + introduction

**Files:**
- Modify: `apps/api/internal/api/course_definition_admin.go` (`putCourseDefinitionReq` + `putCourseDefinition`)
- Modify: `docs/2026-08-17-course-authoring-api-handover.md` (document the two new fields + validation)
- Test: `apps/api/internal/api/course_admin_test.go`

**Interfaces:**
- Consumes: `agent.UpsertCourseDefinitionInput.{Category, Introduction}` (Task 2); `packages/contracts` category slugs (Task 3) — mirrored as a Go allowlist here.
- Produces: `PUT /admin/courses/{slug}/definition` now accepts `category` + `introduction`, rejecting an out-of-vocab category and a non-object introduction.

- [ ] **Step 1: Write failing handler tests**

Add to `course_admin_test.go` (reuse the file's existing request helper / admin-key setup — see how `TestPutCourseDefinition*` builds requests):

```go
func TestPutCourseDefinitionRejectsUnknownCategory(t *testing.T) {
	env := newCourseAdminTestEnv(t) // whatever the file's existing helper is called
	body := map[string]any{
		"definition": json.RawMessage(`{"schemaVersion":"2.0","course":{"id":"cat-x","title":"T"}}`),
		"blurb":      "b",
		"cardIds":    []string{"craap"},
		"category":   "totally-made-up",
	}
	resp := env.putDefinition(t, "cat-x", body) // helper that sets the admin bearer + does the PUT
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 4xx for bad category", resp.StatusCode)
	}
}

func TestPutCourseDefinitionAcceptsCategoryAndIntroduction(t *testing.T) {
	env := newCourseAdminTestEnv(t)
	body := map[string]any{
		"definition":   json.RawMessage(`{"schemaVersion":"2.0","course":{"id":"cat-ok","title":"T"}}`),
		"blurb":        "b",
		"cardIds":      []string{"craap"},
		"category":     "source-check",
		"introduction": json.RawMessage(`{"hook":"h","takeaways":["a"]}`),
	}
	resp := env.putDefinition(t, "cat-ok", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
```

Adapt helper names to the ones already in `course_admin_test.go`. If the file has no reusable env/helper, mirror the exact request construction the existing `putCourseDefinition` test uses (admin bearer header + `httptest`).

- [ ] **Step 2: Run to verify they fail**

Run (from `apps/api`): `go test ./internal/api/ -run TestPutCourseDefinition -count=1 -timeout 1800s`
Expected: the two new tests FAIL (unknown category currently accepted; fields ignored).

- [ ] **Step 3: Add a category allowlist + extend the request struct**

In `course_definition_admin.go`, add a package-level allowlist (mirrors the 7 contract slugs — Global Constraints):

```go
// validCourseCategories mirrors COURSE_CATEGORIES in packages/contracts —
// the closed 7-slug vocabulary. Kept as a Go set for border validation; the
// contract remains the source of truth for the labels.
var validCourseCategories = map[string]bool{
	"stance-value": true, "source-check": true, "media-literacy": true,
	"self-knowledge": true, "data-literacy": true, "research-process": true,
	"argument-writing": true,
}
```

Extend `putCourseDefinitionReq`:

```go
	Category     string          `json:"category"`     // one of the 7 slugs, or "" to leave unset
	Introduction json.RawMessage `json:"introduction"` // schema-driven intro object, or absent
```

- [ ] **Step 4: Validate + thread through in `putCourseDefinition`**

After the existing `cardIDs` validation loop and before building the store input, add:

```go
	// Category: empty means "leave unset" (NULL); a non-empty value must be one
	// of the 7 controlled slugs — never a free-text 8th (spec Global Constraints).
	var categoryPtr *string
	if body.Category != "" {
		if !validCourseCategories[body.Category] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "未知的课程分类: "+body.Category, nil))
			return
		}
		categoryPtr = &body.Category
	}

	// Introduction: border-validate only that it is a JSON object (the deep
	// shape is the generator's Zod contract, per the border-validation rule).
	var introBytes []byte
	if len(body.Introduction) > 0 {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(body.Introduction, &probe); err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "introduction 必须是一个对象", nil))
			return
		}
		introBytes = body.Introduction
	}
```

Then add the two fields to the `UpsertCourseDefinitionInput` literal:

```go
		Category:     categoryPtr,
		Introduction: introBytes,
```

- [ ] **Step 5: Run to verify they pass**

Run (from `apps/api`): `go test ./internal/api/ -run TestPutCourseDefinition -count=1 -timeout 1800s`
Expected: PASS (both new tests + existing ones).

- [ ] **Step 6: Update the handover doc**

In `docs/2026-08-17-course-authoring-api-handover.md`, in the `PUT /admin/courses/{slug}/definition` section, document: the new optional `category` field (must be one of the 7 slugs — list them — or omitted; unknown → 400), the new optional `introduction` object field (shape: `hook`, `whatYouDo`, `takeaways[]`, `alignment{ib[],otherIntl[],domestic[]}`, `keywords[]` — must be a JSON object; deep shape validated by the generator's own contract), that `card_ids` stays registry-validated, and that `featured_rank` is NOT settable here (student-end/product-owned).

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/course_definition_admin.go apps/api/internal/api/course_admin_test.go docs/2026-08-17-course-authoring-api-handover.md
git commit -m "feat(course-authoring): accept category + introduction on definition upsert"
```

---

### Task 6: Home page — cap at 6 via featuredRank with random fallback + 查看更多

**Files:**
- Create: `apps/web/src/shell/home/selectHomeCourses.ts`
- Test: `apps/web/src/shell/home/selectHomeCourses.test.ts`
- Modify: `apps/web/src/shell/home/HomePage.tsx` (`CoursesSection` + `HomePageProps`)
- Modify: `apps/web/src/shell/StudentApp.tsx` (wire `onGoCourses`)

**Interfaces:**
- Produces: `selectHomeCourses(courses, limit, rng?) => CourseSummary[]`; `HomePageProps.onGoCourses: () => void`.
- Consumes: `CourseSummary.featuredRank` (Task 3).

- [ ] **Step 1: Write failing tests for the selection helper**

Create `apps/web/src/shell/home/selectHomeCourses.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { selectHomeCourses } from "./selectHomeCourses";
import type { CourseSummary } from "@mind-imprint/contracts";

function c(slug: string, featuredRank: number | null): CourseSummary {
  return { slug, branch: "A", title: slug, blurb: "", time_label: "", card_ids: [], step_count: 1, coverUrl: "", category: null, introduction: null, featuredRank };
}

describe("selectHomeCourses", () => {
  it("orders featured courses by rank ascending, then fills with the rest", () => {
    const all = [c("a", null), c("b", 2), c("c", 1), c("d", null)];
    const rng = () => 0; // deterministic: stable order for the unranked fill
    const out = selectHomeCourses(all, 6, rng);
    expect(out.slice(0, 2).map((x) => x.slug)).toEqual(["c", "b"]); // rank 1 then rank 2
    expect(out.map((x) => x.slug).sort()).toEqual(["a", "b", "c", "d"]);
  });

  it("caps at the limit", () => {
    const all = Array.from({ length: 10 }, (_, i) => c(String(i), null));
    expect(selectHomeCourses(all, 6, () => 0)).toHaveLength(6);
  });

  it("returns all when fewer than the limit", () => {
    expect(selectHomeCourses([c("a", null)], 6, () => 0)).toHaveLength(1);
  });
});
```

- [ ] **Step 2: Run to verify they fail**

Run (from `apps/web`): `npx vitest run src/shell/home/selectHomeCourses.test.ts`
Expected: FAIL — module does not exist.

- [ ] **Step 3: Implement the helper**

Create `apps/web/src/shell/home/selectHomeCourses.ts`:

```ts
import type { CourseSummary } from "@mind-imprint/contracts";

// selectHomeCourses picks the up-to-`limit` courses the home page shows.
// Featured courses (featuredRank != null) come first, ascending by rank; the
// remaining slots are filled from the rest in random order. When NOTHING is
// featured (the pre-curation state), this degrades to "random `limit`" — so
// "random for now, curated later" is one code path, flipped by data alone.
// `rng` is injectable for deterministic tests; defaults to Math.random.
export function selectHomeCourses(
  courses: CourseSummary[],
  limit: number,
  rng: () => number = Math.random,
): CourseSummary[] {
  const shuffle = (xs: CourseSummary[]): CourseSummary[] => {
    const a = [...xs];
    for (let i = a.length - 1; i > 0; i--) {
      const j = Math.floor(rng() * (i + 1));
      [a[i], a[j]] = [a[j], a[i]];
    }
    return a;
  };
  const ranked = courses
    .filter((c) => c.featuredRank != null)
    .sort((x, y) => (x.featuredRank as number) - (y.featuredRank as number));
  const rest = shuffle(courses.filter((c) => c.featuredRank == null));
  return [...ranked, ...rest].slice(0, limit);
}
```

- [ ] **Step 4: Run to verify they pass**

Run (from `apps/web`): `npx vitest run src/shell/home/selectHomeCourses.test.ts`
Expected: PASS.

- [ ] **Step 5: Use it in `CoursesSection` + add the 查看更多 button**

In `HomePage.tsx`: import `selectHomeCourses`. Change `CoursesSection` to accept `onGoCourses` and render the capped list plus a footer button. Replace the render body's course grid with:

```tsx
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {selectHomeCourses(courses ?? [], 6).map((c) => (
            <CourseCard key={c.slug} course={c} onClick={() => onOpenCourse(c.slug)} />
          ))}
        </div>
      )}
      {!loading && !failed && (courses ?? []).length > 0 && (
        <button
          type="button"
          onClick={onGoCourses}
          className="self-start text-mk-body font-semibold text-mk-accent-600 hover:underline"
        >
          查看更多课程 →
        </button>
      )}
```

Update the `CoursesSection` signature to `{ onOpenCourse, onGoCourses }: { onOpenCourse: (slug: string) => void; onGoCourses: () => void }`. Also change the loading skeleton count from 3 to 6 for visual consistency.

- [ ] **Step 6: Thread `onGoCourses` through props**

In `HomePage.tsx`, add to `HomePageProps`:

```ts
  /** Go to the 课程 tab (home "查看更多课程"). */
  onGoCourses: () => void;
```

Destructure it in `HomePage(...)` and pass to `<CoursesSection onOpenCourse={onOpenCourse} onGoCourses={onGoCourses} />`.

In `StudentApp.tsx`, find where `<HomePage ... />` is rendered (the `tab === ...` home branch, near line 88) and add:

```tsx
        onGoCourses={() => setTab("courses")}
```

- [ ] **Step 7: Typecheck + test**

Run (from `apps/web`): `npm run typecheck && npx vitest run src/shell/home/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/shell/home/selectHomeCourses.ts apps/web/src/shell/home/selectHomeCourses.test.ts apps/web/src/shell/home/HomePage.tsx apps/web/src/shell/StudentApp.tsx
git commit -m "feat(home): cap courses at 6 (featured + random fallback) with 查看更多"
```

---

### Task 7: All-courses page — group by the 7 categories

**Files:**
- Modify: `apps/web/src/shell/courses/CoursesView.tsx`
- Test: `apps/web/src/shell/courses/groupCoursesByCategory.test.ts`
- Create: `apps/web/src/shell/courses/groupCoursesByCategory.ts`

**Interfaces:**
- Produces: `groupCoursesByCategory(courses) => { slug, label, courses }[]` — sections in `COURSE_CATEGORIES` order, empty sections dropped, a trailing `未分类` section for null/unknown categories.
- Consumes: `COURSE_CATEGORIES`, `CourseSummary.category` (Task 3).

- [ ] **Step 1: Write failing tests for the grouping helper**

Create `apps/web/src/shell/courses/groupCoursesByCategory.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { groupCoursesByCategory } from "./groupCoursesByCategory";
import type { CourseSummary } from "@mind-imprint/contracts";

function c(slug: string, category: CourseSummary["category"]): CourseSummary {
  return { slug, branch: "A", title: slug, blurb: "", time_label: "", card_ids: [], step_count: 1, coverUrl: "", category, introduction: null, featuredRank: null };
}

describe("groupCoursesByCategory", () => {
  it("groups in declared category order and drops empty sections", () => {
    const groups = groupCoursesByCategory([c("a", "source-check"), c("b", "stance-value")]);
    expect(groups.map((g) => g.slug)).toEqual(["stance-value", "source-check"]); // declared order
    expect(groups[0].courses.map((x) => x.slug)).toEqual(["b"]);
  });

  it("puts null / unknown categories in a trailing 未分类 section", () => {
    const groups = groupCoursesByCategory([c("a", null), c("b", "source-check")]);
    const last = groups[groups.length - 1];
    expect(last.slug).toBe("__uncategorized__");
    expect(last.label).toBe("未分类");
    expect(last.courses.map((x) => x.slug)).toEqual(["a"]);
  });

  it("returns no 未分类 section when every course is categorized", () => {
    const groups = groupCoursesByCategory([c("a", "data-literacy")]);
    expect(groups.some((g) => g.slug === "__uncategorized__")).toBe(false);
  });
});
```

- [ ] **Step 2: Run to verify they fail**

Run (from `apps/web`): `npx vitest run src/shell/courses/groupCoursesByCategory.test.ts`
Expected: FAIL — module missing.

- [ ] **Step 3: Implement the grouping helper**

Create `apps/web/src/shell/courses/groupCoursesByCategory.ts`:

```ts
import type { CourseSummary } from "@mind-imprint/contracts";
import { COURSE_CATEGORIES } from "@mind-imprint/contracts";

export interface CourseGroup {
  slug: string;
  label: string;
  courses: CourseSummary[];
}

// groupCoursesByCategory buckets the catalog into the 7 categories (in their
// declared order), dropping any empty bucket, and appends a trailing 未分类
// section for courses whose category is null or not one of the 7 (the
// pre-backfill state). Order within a bucket is the input order (the list
// endpoint already sorts by branch/title).
export function groupCoursesByCategory(courses: CourseSummary[]): CourseGroup[] {
  const groups: CourseGroup[] = [];
  const known = new Set<string>(COURSE_CATEGORIES.map((c) => c.slug));
  for (const { slug, label } of COURSE_CATEGORIES) {
    const inCat = courses.filter((c) => c.category === slug);
    if (inCat.length > 0) groups.push({ slug, label, courses: inCat });
  }
  const uncategorized = courses.filter((c) => c.category == null || !known.has(c.category));
  if (uncategorized.length > 0) {
    groups.push({ slug: "__uncategorized__", label: "未分类", courses: uncategorized });
  }
  return groups;
}
```

- [ ] **Step 4: Run to verify they pass**

Run (from `apps/web`): `npx vitest run src/shell/courses/groupCoursesByCategory.test.ts`
Expected: PASS.

- [ ] **Step 5: Render grouped sections in `CoursesView`**

In `CoursesView.tsx`: import `groupCoursesByCategory` (and its `CourseGroup` type). Replace the single flat grid (the `<div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)"...}}>` block) with per-category sections. Keep the existing `CourseCard`, the progress-fetch effect, and the header untouched. New render for the non-empty case:

```tsx
          <div style={{ display: "flex", flexDirection: "column", gap: 40, marginTop: 28 }}>
            {groupCoursesByCategory(courses ?? []).map((group) => (
              <section key={group.slug}>
                <div style={{ fontSize: 18, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 16 }}>{group.label}</div>
                <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 20 }}>
                  {group.courses.map((c) => (
                    <CourseCard key={c.slug} course={c} pct={pctById[c.slug] ?? null} onOpen={() => onOpenCourse?.(c.slug)} onRestart={onRestartCourse ? () => onRestartCourse(c.slug) : undefined} />
                  ))}
                </div>
              </section>
            ))}
          </div>
```

- [ ] **Step 6: Typecheck + test**

Run (from `apps/web`): `npm run typecheck && npx vitest run src/shell/courses/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/shell/courses/groupCoursesByCategory.ts apps/web/src/shell/courses/groupCoursesByCategory.test.ts apps/web/src/shell/courses/CoursesView.tsx
git commit -m "feat(courses): group the all-courses page by the 7 categories"
```

---

### Task 8: Course detail page — render the structured introduction

**Files:**
- Create: `apps/web/src/shell/courses/CourseDetail.tsx`
- Modify: `apps/web/src/shell/courses/CoursesContainer.tsx` (add a `detail` view; route grid + deep-links through it)

**Interfaces:**
- Consumes: `CourseSummary.introduction` + `blurb` (Task 3); `api.listCourses`, `api.getCourseProgress`.
- Produces: a `detail` landing that renders the introduction (or falls back to `blurb`) and whose CTA enters the player.

- [ ] **Step 1: Build `CourseDetail`**

Create `apps/web/src/shell/courses/CourseDetail.tsx`. It loads the catalog, finds the slug, and renders the introduction deterministically; a null introduction falls back to `blurb`. The CTA calls `onStart`.

```tsx
import { useEffect, useState } from "react";
import type { CourseSummary } from "@mind-imprint/contracts";
import { api } from "@/api";
import { Button } from "@/ui";

function Chips({ items }: { items: string[] }) {
  if (items.length === 0) return null;
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
      {items.map((t, i) => (
        <span key={i} style={{ fontSize: 12.5, fontWeight: 600, color: "var(--mk-secondary)", background: "var(--mk-paper)", border: "1px solid var(--mk-border)", padding: "5px 11px", borderRadius: 999 }}>{t}</span>
      ))}
    </div>
  );
}

function AlignmentColumn({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null;
  return (
    <div style={{ flex: 1, minWidth: 180 }}>
      <div style={{ fontSize: 12, fontWeight: 700, color: "var(--mk-muted)", marginBottom: 8 }}>{title}</div>
      <ul style={{ margin: 0, paddingLeft: 18, display: "flex", flexDirection: "column", gap: 6 }}>
        {items.map((t, i) => <li key={i} style={{ fontSize: 13.5, color: "var(--mk-secondary)", lineHeight: 1.6 }}>{t}</li>)}
      </ul>
    </div>
  );
}

export function CourseDetail({ slug, onStart, onBack }: { slug: string; onStart: () => void; onBack: () => void }) {
  const [course, setCourse] = useState<CourseSummary | null | undefined>(undefined); // undefined=loading, null=not found
  const [pct, setPct] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    void api.listCourses().then((cs) => {
      if (cancelled) return;
      const found = cs.find((c) => c.slug === slug) ?? null;
      setCourse(found);
      if (found && found.step_count > 0) {
        void Promise.resolve(api.getCourseProgress(slug)).then((p) => {
          if (cancelled || !p) return;
          const n = p.completed_ordinals.length;
          setPct(n > 0 ? Math.round((n / found.step_count) * 100) : null);
        }).catch(() => {});
      }
    }).catch(() => { if (!cancelled) setCourse(null); });
    return () => { cancelled = true; };
  }, [slug]);

  if (course === undefined) {
    return <div style={{ height: "100%", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--mk-faint)", fontSize: 14 }}>正在加载课程…</div>;
  }
  if (course === null) {
    return (
      <div style={{ height: "100%", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 12, color: "var(--mk-secondary)", fontSize: 14 }}>
        <div>找不到这门课程。</div>
        <Button variant="ghost" onClick={onBack}>返回课程</Button>
      </div>
    );
  }

  const intro = course.introduction;
  const cta = pct == null ? "开始学习" : pct >= 100 ? "回顾" : "继续";

  return (
    <div style={{ height: "100%", minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 780, margin: "0 auto", padding: "40px 40px 64px" }}>
        <button type="button" onClick={onBack} style={{ background: "none", border: "none", color: "var(--mk-muted)", fontSize: 13, fontWeight: 600, cursor: "pointer", padding: 0, fontFamily: "inherit" }}>← 返回课程</button>
        <div style={{ fontSize: 12, color: "var(--mk-muted)", fontWeight: 600, marginTop: 20 }}>{course.branch} · {course.step_count} 个任务 · {course.card_ids.length} 个工具 · {course.time_label}</div>
        <h1 style={{ fontSize: 30, fontWeight: 800, color: "var(--mk-ink)", lineHeight: 1.35, marginTop: 8, letterSpacing: "-0.01em" }}>{course.title}</h1>

        {intro ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 30, marginTop: 24 }}>
            {intro.hook && <p style={{ fontSize: 15.5, color: "var(--mk-secondary)", lineHeight: 1.75 }}>{intro.hook}</p>}
            {intro.whatYouDo && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 10 }}>学生做什么</div>
                <p style={{ fontSize: 14.5, color: "var(--mk-secondary)", lineHeight: 1.75 }}>{intro.whatYouDo}</p>
              </section>
            )}
            {intro.takeaways.length > 0 && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 10 }}>带走什么</div>
                <ul style={{ margin: 0, paddingLeft: 20, display: "flex", flexDirection: "column", gap: 8 }}>
                  {intro.takeaways.map((t, i) => <li key={i} style={{ fontSize: 14.5, color: "var(--mk-secondary)", lineHeight: 1.7 }}>{t}</li>)}
                </ul>
              </section>
            )}
            {(intro.alignment.ib.length + intro.alignment.otherIntl.length + intro.alignment.domestic.length) > 0 && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 12 }}>学科对标</div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 24 }}>
                  <AlignmentColumn title="IB" items={intro.alignment.ib} />
                  <AlignmentColumn title="其他国际" items={intro.alignment.otherIntl} />
                  <AlignmentColumn title="国内" items={intro.alignment.domestic} />
                </div>
              </section>
            )}
            {intro.keywords.length > 0 && (
              <section>
                <div style={{ fontSize: 16, fontWeight: 800, color: "var(--mk-ink)", marginBottom: 12 }}>关键词</div>
                <Chips items={intro.keywords} />
              </section>
            )}
          </div>
        ) : (
          <p style={{ fontSize: 15, color: "var(--mk-secondary)", lineHeight: 1.75, marginTop: 24 }}>{course.blurb}</p>
        )}

        <div style={{ marginTop: 40 }}>
          <button type="button" onClick={onStart} style={{ display: "inline-flex", alignItems: "center", gap: 8, background: "var(--mk-accent-500)", color: "var(--mk-surface)", border: "none", padding: "12px 22px", borderRadius: 12, fontSize: 15, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
            {cta}
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Route grid + deep-links through the detail view in `CoursesContainer`**

In `CoursesContainer.tsx`:

1. Import: `import { CourseDetail } from "./CourseDetail";`
2. Extend the `View` union to add detail:

```ts
type View = { name: "grid" } | { name: "detail"; courseId: string } | { name: "player"; courseId: string } | { name: "report"; courseId: string };
```

3. Change the initial view so a deep-link opens the DETAIL landing (the universal course entry), not the player directly:

```ts
  const [view, setView] = useState<View>(initialCourseId ? { name: "detail", courseId: initialCourseId } : { name: "grid" });
```

4. Detail is a browse page — NOT immersive. Change the immersive effect so only player/report hide the chrome:

```ts
  useEffect(() => {
    onImmersiveChange?.(view.name === "player" || view.name === "report");
  }, [view.name, onImmersiveChange]);
```

5. Add the detail branch (before the `view.name === "player"` branch):

```tsx
  if (view.name === "detail") {
    return (
      <CourseDetail
        slug={view.courseId}
        onStart={() => setView({ name: "player", courseId: view.courseId })}
        onBack={() => setView({ name: "grid" })}
      />
    );
  }
```

6. Change the grid's card handler to open detail instead of the player:

```tsx
  return (
    <CoursesView
      onOpenCourse={(id) => setView({ name: "detail", courseId: id })}
      onRestartCourse={(id) => void restartAndPlay(id)}
    />
  );
```

Leave the `player`/`report` branches and `restartAndPlay` as they are (player exit → grid).

- [ ] **Step 3: Typecheck + full web unit suite**

Run (from `apps/web`): `npm run typecheck && npx vitest run`
Expected: PASS (the whole suite — this touches shared navigation).

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/shell/courses/CourseDetail.tsx apps/web/src/shell/courses/CoursesContainer.tsx
git commit -m "feat(courses): course detail page rendering the structured introduction"
```

---

### Task 9: Version tag + doc updates (handover + data-structure)

**Files:**
- Modify: `docs/2026-08-17-course-authoring-api-handover.md` (version pin refs + changelog + new request fields — completes the partial edit Task 5 made to the endpoint section)
- Modify: `docs/architecture/database-schema.md` (document the `course` catalog columns incl. the three new ones)
- Git: annotated tag `course-authoring-v1.3.0` on `main` (created at finish, after merge)

**Interfaces:**
- Consumes: the shipped behavior of Tasks 1–8 (columns, validation, contract fields).
- Produces: the named contract snapshot colleagues pin to, and the canonical schema record for the new columns.

This is a docs-and-release task with no automated test; verify by reading. Do the doc edits on the feature branch (Steps 1–2); the git tag (Step 4) is a release action the controller runs on `main` after the branch merges.

- [ ] **Step 1: Bump the version pin + add a changelog entry in the handover doc**

In `docs/2026-08-17-course-authoring-api-handover.md`:
- Update every `course-authoring-v1.2.0` pin reference to `course-authoring-v1.3.0` (the "Pinned at tag" line near the top, the "How to consume the pin" bullets, and any other `v1.2.0` mention that names the current pin — do NOT rewrite the historical changelog entries for older versions).
- Add a new changelog bullet at the TOP of the version list (above the `v1.2.0` entry), verbatim intent:

```markdown
- **`course-authoring-v1.3.0`** — ⚠ **contract + API change (additive, backward-compatible).** Two new OPTIONAL fields on `PUT /admin/courses/{slug}/definition`: **`category`** (one of the 7 controlled slugs — `stance-value`, `source-check`, `media-literacy`, `self-knowledge`, `data-literacy`, `research-process`, `argument-writing`; an unknown slug → 400; omit to leave unset) and **`introduction`** (a structured intro object: `hook`, `whatYouDo`, `takeaways[]`, `alignment{ib[],otherIntl[],domestic[]}`, `keywords[]` — must be a JSON object; deep shape is the generator's own contract). `card_ids` stays registry-validated. **`featured_rank` is NOT settable via this API** — home-page curation is product/student-end owned. `CourseSummary` (the `GET /api/v1/courses` list) gains `category` (slug|null), `introduction` (object|null), `featuredRank` (int|null). Existing definitions and older clients are unaffected.
```

- [ ] **Step 2: Document the `course` catalog columns in `database-schema.md`**

The `course` table (course v2, migration 0050 + later) is not yet in this doc. Under `## Core domain`, add a `### `course`` section at the same level and format as the neighbouring tables (a `| column | type | notes |` table). Source the exact columns/types by reading the migrations: the 0050 course-table create, `0070_course_definition.sql` (`course_definition jsonb`), `0072_course_status_cover.sql` (`status`, `cover`), and `0075_course_catalog_metadata.sql` (the three new ones). At minimum the three new columns must appear, described as:

```markdown
| `category` | text null | one of the 7 controlled category slugs (validated app-side against `packages/contracts` COURSE_CATEGORIES, not a DB CHECK); null = uncategorized |
| `introduction` | jsonb null | schema-driven course intro (`hook`/`whatYouDo`/`takeaways[]`/`alignment`/`keywords`); border-validated app-side; null → detail page falls back to `blurb` |
| `featured_rank` | int null | home-page curation order (lower = earlier); null = not featured → random fill. Product/student-end owned; never written by the authoring API |
```

Include the rest of the `course` columns (slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache, audio_manifest, course_definition, status, cover) transcribed from the migrations so the section is complete, not a fragment.

- [ ] **Step 3: Commit the doc edits**

```bash
git add docs/2026-08-17-course-authoring-api-handover.md docs/architecture/database-schema.md
git commit -m "docs(course): bump pin to v1.3.0; document category/introduction/featured_rank"
```

- [ ] **Step 4: Tag the version (release step — controller runs on `main` after merge)**

After the feature branch is merged to `main`, create the annotated tag on `main`:

```bash
git tag -a course-authoring-v1.3.0 -m "course catalog: category + structured introduction + home curation (additive)"
git push origin course-authoring-v1.3.0
```

This is deliberately the LAST action, on `main`, so the tag names the merged revision colleagues pin to — not a pre-merge branch commit.

---

## Self-Review

**1. Spec coverage:**
- Contract `category`/`introduction`/`featuredRank` → Task 3. ✅
- 7-slug no-emoji controlled vocab → Task 3 (`COURSE_CATEGORIES`) + Task 5 (Go allowlist) + Global Constraints. ✅
- DB storage (nullable) → Task 1; store/query round-trip → Task 2. ✅
- List DTO emits fields → Task 4. ✅
- Authoring API accepts + validates category/introduction, card_ids strict, featured_rank NOT settable → Task 5. ✅
- Home cap-6 featured + random fallback + 查看更多 → Task 6. ✅
- All-courses grouped by 7 categories + 未分类 tail → Task 7. ✅
- Course detail renders introduction, blurb fallback → Task 8. ✅
- Generator drafts / teacher ships (no auto-publish) → unchanged `preview`→`ship` lifecycle; Task 5 keeps `UpsertCourseDefinition`'s status contract. ✅
- No engagement ranking (铁律②) → Task 6 helper is featured+random only. ✅
- Backfill of existing courses: spec allows leaving null (fallbacks cover it) — no task needed; verified by Task 7's 未分类 section and Task 8's blurb fallback. ✅
- Version tag bumped to `course-authoring-v1.3.0`; handover doc + `database-schema.md` updated → Task 9. ✅

**2. Placeholder scan:** No TBD/TODO. Test helpers in Tasks 2/5 reference existing file harnesses by name with an explicit "adapt to the file's real helper" instruction rather than inventing one — acceptable because the exact harness name can only be read at implementation time; every other step ships real code.

**3. Type consistency:** `CourseSummaryRow` fields `Category *string` / `Introduction []byte` / `FeaturedRank *int32` (Task 2) match the sqlc `emit_pointers_for_null_types` output and the DTO mapper's use in Task 4. Contract field names `category`/`introduction`/`featuredRank` (Task 3) match the DTO JSON tags (Task 4) and the web consumers (Tasks 6–8). `selectHomeCourses`/`groupCoursesByCategory` signatures match their call sites. `View` union extension in Task 8 is internally consistent.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-08-19-course-catalog-taxonomy-and-intro.md`. Two execution options:

1. **Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — execute tasks in this session with checkpoints.

Which approach?

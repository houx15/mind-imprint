# Slice 4a · Course Model + List/Progress API — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add the backend course model (`course`/`course_step`/`course_progress`), the list/get/progress API, one seeded course, and make the Courses grid read real backend data (replacing Slice 1's mock fixture). No runtime generation yet (that's 4b).

**Architecture:** Zod `Course*` contracts; goose migration `0011_courses.sql` + seed `0012_seed_course.sql`; sqlc queries; `internal/api/course.go` handlers (session-required; courses are a shared catalog, progress is per-user); frontend `api/courses.ts` + `CoursesView` reads `api.listCourses()`.

**Tech Stack:** Go 1.26 + sqlc + goose + testcontainers; React + Vitest. Backend from `apps/api` (`make sqlc`, `go test`); web from `apps/web` (`npx vitest run`, `npx tsc`).

## Global Constraints

- **Backend + contracts + Courses-grid only.** No generation runtime, no course player (those are 4b/4c). Session required (`RequireUser`); course catalog is shared (no per-user ownership on course/step reads); `course_progress` is per-user, scoped by `user_id`.
- Contract↔DTO parity is manual — DTO JSON tags match the Zod shapes.
- **Course step carries metadata only** (`kind`/`purpose`/`assets`/`challenge_type`/`authored_content`); generated content is 4b.
- Docker for Go tests (testcontainers). `make sqlc` = `CGO_ENABLED=0 go tool sqlc generate`.
- UI copy Chinese; code identifiers English. Every task ends green + committed.

---

### Task 1: `Course*` contracts

**Files:**
- Create: `packages/contracts/src/course.ts`
- Modify: `packages/contracts/src/index.ts`
- Test: `packages/contracts/test/course.test.ts`

**Interfaces:**
- Produces: `CourseStepKind`, `CourseAsset`, `CourseStep`, `CourseSummary`, `Course`, `CourseProgress` Zod + types.

- [ ] **Step 1: Write the failing test**

Create `packages/contracts/test/course.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { Course, CourseSummary, CourseProgress } from "../src/course";

const summary = { id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "…", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 4 };
const step = { id: "s1", course_id: "co1", ordinal: 0, kind: "teaching", purpose: "建立横向溯源意识", assets: [{ id: "a0", kind: "text", title: "开场", value: "最近一张卫星图刷屏。" }], challenge_type: null, authored_content: { subtitle: "…", body: ["…"], foreground_asset_id: null } };

describe("Course contracts", () => {
  it("parses a course summary", () => { expect(CourseSummary.parse(summary).step_count).toBe(4); });
  it("parses a full course with steps", () => { expect(Course.parse({ ...summary, steps: [step] }).steps).toHaveLength(1); });
  it("rejects an unknown step kind", () => { expect(Course.parse.bind(null, { ...summary, steps: [{ ...step, kind: "quiz" }] })).toThrow(); });
  it("parses progress with completed ordinals", () => { expect(CourseProgress.parse({ course_id: "co1", current_ordinal: 2, completed_ordinals: [0, 1], updated_at: "1" }).completed_ordinals).toEqual([0, 1]); });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd packages/contracts && npx vitest run test/course.test.ts`
Expected: FAIL — cannot resolve `../src/course`.

- [ ] **Step 3: Create `course.ts`**

Create `packages/contracts/src/course.ts`:

```ts
import { z } from "zod";

export const CourseStepKind = z.enum(["teaching", "challenge"]);
export const CourseAssetKind = z.enum(["image", "text", "link"]);

export const CourseAsset = z.object({
  id: z.string(),
  kind: CourseAssetKind,
  title: z.string(),
  value: z.string(),
});

export const CourseStep = z.object({
  id: z.string(),
  course_id: z.string(),
  ordinal: z.number().int(),
  kind: CourseStepKind,
  purpose: z.string(),
  assets: z.array(CourseAsset),
  challenge_type: z.string().nullable(),
  authored_content: z.unknown(),
});

export const CourseSummary = z.object({
  id: z.string(),
  branch: z.string(),
  title: z.string(),
  blurb: z.string(),
  tasks_count: z.number().int(),
  tools_count: z.number().int(),
  time_label: z.string(),
  step_count: z.number().int(),
});

export const Course = CourseSummary.extend({ steps: z.array(CourseStep) });

export const CourseProgress = z.object({
  course_id: z.string(),
  current_ordinal: z.number().int(),
  completed_ordinals: z.array(z.number().int()),
  updated_at: z.string(),
});

export type CourseStepKind = z.infer<typeof CourseStepKind>;
export type CourseAsset = z.infer<typeof CourseAsset>;
export type CourseStep = z.infer<typeof CourseStep>;
export type CourseSummary = z.infer<typeof CourseSummary>;
export type Course = z.infer<typeof Course>;
export type CourseProgress = z.infer<typeof CourseProgress>;
```

- [ ] **Step 4: Barrel export**

In `packages/contracts/src/index.ts`, add after `export * from "./anchor";`:

```ts
export * from "./course";
```

- [ ] **Step 5: Run — expect PASS**

Run: `cd packages/contracts && npx vitest run test/course.test.ts && npx tsc --noEmit`
Expected: PASS (4 tests); tsc clean.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/course.ts packages/contracts/src/index.ts packages/contracts/test/course.test.ts
git commit -m "feat(contracts): Course/CourseStep/CourseProgress schemas"
```

---

### Task 2: DB — schema, seed, queries, sqlc, DTO

**Files:**
- Create: `apps/api/internal/store/migrations/0011_courses.sql`, `0012_seed_course.sql`
- Create: `apps/api/internal/store/queries/course.sql`
- Generated (`make sqlc`): `apps/api/internal/store/sqlc/course.sql.go` + models
- Create: `apps/api/internal/api/course_dto.go`
- Test: `apps/api/internal/api/course_store_test.go`

**Interfaces:**
- Produces (sqlc): `Course`, `CourseStep`, `CourseProgress` models; `ListCourses`, `GetCourse`, `ListCourseSteps`, `GetCourseProgress`, `UpsertCourseProgress`. DTOs `courseSummaryDTO`/`courseStepDTO`/`courseDTO`/`courseProgressDTO` + converters.

- [ ] **Step 1: Migration (schema)**

Create `apps/api/internal/store/migrations/0011_courses.sql`:

```sql
-- +goose Up
CREATE TABLE course (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    branch      text NOT NULL,
    title       text NOT NULL,
    blurb       text NOT NULL DEFAULT '',
    tasks_count int  NOT NULL DEFAULT 0,
    tools_count int  NOT NULL DEFAULT 0,
    time_label  text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE course_step (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id        uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
    ordinal          int  NOT NULL,
    kind             text NOT NULL CHECK (kind IN ('teaching','challenge')),
    purpose          text NOT NULL DEFAULT '',
    assets           jsonb NOT NULL DEFAULT '[]',
    challenge_type   text,
    authored_content jsonb NOT NULL DEFAULT '{}',
    UNIQUE (course_id, ordinal)
);
CREATE INDEX course_step_course_ordinal_idx ON course_step (course_id, ordinal);

CREATE TABLE course_progress (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id          uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
    current_ordinal    int  NOT NULL DEFAULT 0,
    completed_ordinals int[] NOT NULL DEFAULT '{}',
    updated_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, course_id)
);

-- +goose Down
DROP TABLE course_progress;
DROP TABLE course_step;
DROP TABLE course;
```

- [ ] **Step 2: Migration (seed one course)**

Create `apps/api/internal/store/migrations/0012_seed_course.sql`:

```sql
-- +goose Up
INSERT INTO course (id, branch, title, blurb, tasks_count, tools_count, time_label)
VALUES ('00000000-0000-0000-0000-0000000000c1',
        '批判性思维', '一条网络信息，该不该信',
        '从一句「卫星图显示中国让地球变绿」出发，跟着印记学会横向溯源、辨识来源、拆穿断言——把「随手一信」变成「查过再信」。',
        3, 4, '约 40 分钟');

INSERT INTO course_step (course_id, ordinal, kind, purpose, assets, challenge_type, authored_content) VALUES
('00000000-0000-0000-0000-0000000000c1', 0, 'teaching',
 '让学生意识到「随手一信」的风险，建立停一下的习惯',
 '[{"id":"a0","kind":"text","title":"开场","value":"最近一张 NASA 卫星对比图在朋友圈刷屏，说过去二十年地球「绿了 5%」，最大功劳属于中国。"}]',
 NULL,
 '{"title":"先别急着信","subtitle":"看到一条让你想转发的信息，先停一下——这一停，就是批判性思维的开始。","body":["刷到「地球绿了、中国第一」这种消息，第一反应往往是转发。","但越是让你有情绪的信息，越值得先停一下：它想让你相信什么？证据够吗？"],"foreground_asset_id":"a0"}'),
('00000000-0000-0000-0000-0000000000c1', 1, 'teaching',
 '教横向溯源：跳出单一来源，找更权威、更原始的版本',
 '[{"id":"a0","kind":"text","title":"要点","value":"不要在一篇文章里判断真假，跳出去看别人怎么说、找原始出处。"}]',
 NULL,
 '{"title":"横向溯源","subtitle":"判断一条信息可不可信，最好的办法不是盯着它，而是跳出去看别人怎么说。","body":["与其在一篇公众号文章里纠结，不如去查：NASA 的原始数据是怎么说的？有没有独立的研究？","这叫横向溯源——先广后深，不被单一来源带着走。"],"foreground_asset_id":"a0"}'),
('00000000-0000-0000-0000-0000000000c1', 2, 'challenge',
 '让学生亲手核查一处具体断言，练习分辨事实与情绪',
 '[{"id":"m0","kind":"text","title":"待核查的文章片段","value":"某科技博主综合整理的这篇文章称，过去二十年地球比从前「绿了 5%」，而其中最大的功劳属于中国。根据 2019 年的卫星观测数据，中国的植被增长占到全球四分之一以上。"}]',
 'verify_claim',
 '{"title":"现在轮到你","prompt":"下面这段话里，哪一句是可以核查的事实、哪一句在激发情绪？挑一处你最想核实的，说说你会怎么查。","anchors":[{"id":"a0","material_id":"m0","block_id":"b0","start":0,"end":0,"quote":"某科技博主综合整理","dimension":"权威性 · Authority","author":"ai","question":"这位「综合整理」的作者是谁？他有遥感或地球科学的专业背景吗？","answer":""}],"reason_hint":"写一句你为什么这么判断"}')
;

-- +goose Down
DELETE FROM course WHERE id = '00000000-0000-0000-0000-0000000000c1';
```

(Note: `course_step` rows cascade-delete with the course.)

- [ ] **Step 3: Queries**

Create `apps/api/internal/store/queries/course.sql`:

```sql
-- name: ListCourses :many
SELECT c.*, count(s.id) AS step_count
FROM course c
LEFT JOIN course_step s ON s.course_id = c.id
GROUP BY c.id
ORDER BY c.created_at;

-- name: GetCourse :one
SELECT * FROM course WHERE id = $1;

-- name: CountCourseSteps :one
SELECT count(*) FROM course_step WHERE course_id = $1;

-- name: ListCourseSteps :many
SELECT * FROM course_step WHERE course_id = $1 ORDER BY ordinal;

-- name: GetCourseProgress :one
SELECT * FROM course_progress WHERE user_id = $1 AND course_id = $2;

-- name: UpsertCourseProgress :one
INSERT INTO course_progress (user_id, course_id, current_ordinal, completed_ordinals)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, course_id) DO UPDATE
SET current_ordinal = EXCLUDED.current_ordinal,
    completed_ordinals = EXCLUDED.completed_ordinals,
    updated_at = now()
RETURNING *;
```

- [ ] **Step 4: Generate sqlc**

Run: `cd apps/api && make sqlc`
Expected: no errors; `ListCoursesRow` (course cols + `StepCount int64`), `Course`, `CourseStep` (`Assets []byte`, `ChallengeType *string`, `AuthoredContent []byte`), `CourseProgress` (`CompletedOrdinals []int32`), and the 6 query funcs generated. Note the actual generated names (esp. `ListCoursesRow.StepCount` type and `CompletedOrdinals` element type) and use them in the DTO.

- [ ] **Step 5: DTOs**

Create `apps/api/internal/api/course_dto.go`:

```go
package api

import (
	"encoding/json"

	"mindimprint/api/internal/store/sqlc"
)

type courseSummaryDTO struct {
	ID         string `json:"id"`
	Branch     string `json:"branch"`
	Title      string `json:"title"`
	Blurb      string `json:"blurb"`
	TasksCount int32  `json:"tasks_count"`
	ToolsCount int32  `json:"tools_count"`
	TimeLabel  string `json:"time_label"`
	StepCount  int    `json:"step_count"`
}

func toCourseSummaryDTO(r sqlc.ListCoursesRow) courseSummaryDTO {
	return courseSummaryDTO{
		ID: r.ID.String(), Branch: r.Branch, Title: r.Title, Blurb: r.Blurb,
		TasksCount: r.TasksCount, ToolsCount: r.ToolsCount, TimeLabel: r.TimeLabel,
		StepCount: int(r.StepCount),
	}
}

type courseStepDTO struct {
	ID              string          `json:"id"`
	CourseID        string          `json:"course_id"`
	Ordinal         int32           `json:"ordinal"`
	Kind            string          `json:"kind"`
	Purpose         string          `json:"purpose"`
	Assets          json.RawMessage `json:"assets"`
	ChallengeType   *string         `json:"challenge_type"`
	AuthoredContent json.RawMessage `json:"authored_content"`
}

func toCourseStepDTO(s sqlc.CourseStep) courseStepDTO {
	assets := json.RawMessage(s.Assets)
	if len(assets) == 0 {
		assets = json.RawMessage("[]")
	}
	ac := json.RawMessage(s.AuthoredContent)
	if len(ac) == 0 {
		ac = json.RawMessage("{}")
	}
	return courseStepDTO{
		ID: s.ID.String(), CourseID: s.CourseID.String(), Ordinal: s.Ordinal,
		Kind: s.Kind, Purpose: s.Purpose, Assets: assets,
		ChallengeType: s.ChallengeType, AuthoredContent: ac,
	}
}

type courseDTO struct {
	courseSummaryDTO
	Steps []courseStepDTO `json:"steps"`
}

type courseProgressDTO struct {
	CourseID          string  `json:"course_id"`
	CurrentOrdinal    int32   `json:"current_ordinal"`
	CompletedOrdinals []int32 `json:"completed_ordinals"`
	UpdatedAt         string  `json:"updated_at"`
}

func toCourseProgressDTO(p sqlc.CourseProgress) courseProgressDTO {
	co := p.CompletedOrdinals
	if co == nil {
		co = []int32{}
	}
	return courseProgressDTO{
		CourseID: p.CourseID.String(), CurrentOrdinal: p.CurrentOrdinal,
		CompletedOrdinals: co, UpdatedAt: p.UpdatedAt.Format(tsLayout),
	}
}
```

(If `make sqlc` typed `TasksCount`/`ToolsCount`/`Ordinal`/`CurrentOrdinal` as `int32` and `StepCount` as `int64`, the code above matches; adjust to the generated types if different.)

- [ ] **Step 6: Round-trip test**

Create `apps/api/internal/api/course_store_test.go`:

```go
package api_test

import (
	"context"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestCourseSeedAndProgressRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	courses, err := q.ListCourses(ctx)
	if err != nil || len(courses) == 0 {
		t.Fatalf("list courses: %v len=%d", err, len(courses))
	}
	co := courses[0]
	if co.Title == "" || co.StepCount == 0 {
		t.Fatalf("seed course missing title/steps: %+v", co)
	}

	steps, err := q.ListCourseSteps(ctx, co.ID)
	if err != nil || len(steps) == 0 {
		t.Fatalf("list steps: %v len=%d", err, len(steps))
	}

	prog, err := q.UpsertCourseProgress(ctx, sqlc.UpsertCourseProgressParams{
		UserID: mustUUID(SeedUserID.String()), CourseID: co.ID, CurrentOrdinal: 1, CompletedOrdinals: []int32{0},
	})
	if err != nil || prog.CurrentOrdinal != 1 {
		t.Fatalf("upsert progress: %v %+v", err, prog)
	}
	// upsert again → update path
	prog2, err := q.UpsertCourseProgress(ctx, sqlc.UpsertCourseProgressParams{
		UserID: mustUUID(SeedUserID.String()), CourseID: co.ID, CurrentOrdinal: 2, CompletedOrdinals: []int32{0, 1},
	})
	if err != nil || prog2.CurrentOrdinal != 2 || len(prog2.CompletedOrdinals) != 2 {
		t.Fatalf("upsert update: %v %+v", err, prog2)
	}
}
```

- [ ] **Step 7: Run — expect PASS + build**

Run: `cd apps/api && go build ./... && go test ./internal/api/ -run TestCourseSeedAndProgressRoundTrip`
Expected: build clean; PASS (testcontainers).

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/store/migrations/0011_courses.sql apps/api/internal/store/migrations/0012_seed_course.sql apps/api/internal/store/queries/course.sql apps/api/internal/store/sqlc/ apps/api/internal/api/course_dto.go apps/api/internal/api/course_store_test.go
git commit -m "feat(api): course/course_step/course_progress tables + seed + queries + DTO"
```

---

### Task 3: Course handlers + routes

**Files:**
- Create: `apps/api/internal/api/course.go`
- Modify: `apps/api/internal/api/api.go` (routes)
- Test: `apps/api/internal/api/course_test.go`

**Interfaces:**
- Consumes: `ListCourses`, `GetCourse`, `ListCourseSteps`, `GetCourseProgress`, `UpsertCourseProgress`; the DTOs from Task 2.
- Produces: handlers `listCourses`, `getCourse`, `getCourseProgress`, `putCourseProgress`.

- [ ] **Step 1: Write the handler test**

Create `apps/api/internal/api/course_test.go`:

```go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestCourseListGetProgress(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	// list
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses", nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("一条网络信息，该不该信")) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	var listResp struct {
		Courses []struct {
			ID        string `json:"id"`
			StepCount int    `json:"step_count"`
		} `json:"courses"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listResp)
	id := listResp.Courses[0].ID
	if listResp.Courses[0].StepCount == 0 {
		t.Fatalf("step_count 0")
	}

	// get (with steps)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+id, nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"steps"`)) || !bytes.Contains(rec.Body.Bytes(), []byte("challenge")) {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}

	// progress default (no row yet) → current_ordinal 0
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+id+"/progress", nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"current_ordinal":0`)) {
		t.Fatalf("progress default: %d %s", rec.Code, rec.Body)
	}

	// put progress
	body, _ := json.Marshal(map[string]any{"current_ordinal": 2, "completed_ordinals": []int{0, 1}})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/courses/"+id+"/progress", bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"current_ordinal":2`)) {
		t.Fatalf("put progress: %d %s", rec.Code, rec.Body)
	}

	// unknown course id → 404 on get
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/00000000-0000-0000-0000-0000000000ff", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown course: want 404 got %d", rec.Code)
	}
	_ = context.Background()
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/api && go test ./internal/api/ -run TestCourseListGetProgress`
Expected: FAIL — handlers/routes undefined (404 on `/api/v1/courses`).

- [ ] **Step 3: Implement handlers**

Create `apps/api/internal/api/course.go`:

```go
package api

import (
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

func (a *API) listCourses(w http.ResponseWriter, r *http.Request) {
	rows, err := a.d.Queries.ListCourses(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]courseSummaryDTO, 0, len(rows))
	for _, c := range rows {
		out = append(out, toCourseSummaryDTO(c))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"courses": out})
}

// loadCourse parses {id} and loads the course (404 if absent).
func (a *API) loadCourse(w http.ResponseWriter, r *http.Request) (sqlc.Course, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Course{}, false
	}
	c, err := a.d.Queries.GetCourse(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err) // ErrNoRows → 404
		return sqlc.Course{}, false
	}
	return c, true
}

func (a *API) getCourse(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loadCourse(w, r)
	if !ok {
		return
	}
	steps, err := a.d.Queries.ListCourseSteps(r.Context(), c.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	n, err := a.d.Queries.CountCourseSteps(r.Context(), c.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	stepDTOs := make([]courseStepDTO, 0, len(steps))
	for _, s := range steps {
		stepDTOs = append(stepDTOs, toCourseStepDTO(s))
	}
	dto := courseDTO{
		courseSummaryDTO: courseSummaryDTO{
			ID: c.ID.String(), Branch: c.Branch, Title: c.Title, Blurb: c.Blurb,
			TasksCount: c.TasksCount, ToolsCount: c.ToolsCount, TimeLabel: c.TimeLabel,
			StepCount: int(n),
		},
		Steps: stepDTOs,
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"course": dto})
}

func (a *API) getCourseProgress(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loadCourse(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	p, err := a.d.Queries.GetCourseProgress(r.Context(), sqlc.GetCourseProgressParams{UserID: u.ID, CourseID: c.ID})
	if err != nil {
		// no row yet → default zero progress
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": map[string]any{
			"course_id": c.ID.String(), "current_ordinal": 0, "completed_ordinals": []int32{}, "updated_at": "",
		}})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": toCourseProgressDTO(p)})
}

func (a *API) putCourseProgress(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loadCourse(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	var body struct {
		CurrentOrdinal    int32   `json:"current_ordinal"`
		CompletedOrdinals []int32 `json:"completed_ordinals"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.CompletedOrdinals == nil {
		body.CompletedOrdinals = []int32{}
	}
	p, err := a.d.Queries.UpsertCourseProgress(r.Context(), sqlc.UpsertCourseProgressParams{
		UserID: u.ID, CourseID: c.ID, CurrentOrdinal: body.CurrentOrdinal, CompletedOrdinals: body.CompletedOrdinals,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": toCourseProgressDTO(p)})
}
```

- [ ] **Step 4: Register routes**

In `apps/api/internal/api/api.go` `Handler()`, in the protected group, add:

```go
	mux.Handle("GET /api/v1/courses", protected(a.listCourses))
	mux.Handle("GET /api/v1/courses/{id}", protected(a.getCourse))
	mux.Handle("GET /api/v1/courses/{id}/progress", protected(a.getCourseProgress))
	mux.Handle("PUT /api/v1/courses/{id}/progress", protected(a.putCourseProgress))
```

- [ ] **Step 5: Run — expect PASS + build/vet/short**

Run: `cd apps/api && go test ./internal/api/ -run TestCourseListGetProgress && go build ./... && go vet ./... && go test -short ./...`
Expected: PASS; build/vet clean; short tests pass.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/course.go apps/api/internal/api/api.go apps/api/internal/api/course_test.go
git commit -m "feat(api): course list/get/progress handlers + routes"
```

---

### Task 4: Frontend — Courses grid reads real data

**Files:**
- Create: `apps/web/src/api/courses.ts`
- Modify: `apps/web/src/api/index.ts`
- Modify: `apps/web/src/shell/courses/CoursesView.tsx`
- Delete: `apps/web/src/shell/courses/fixtures.ts`
- Test: `apps/web/src/shell/courses/CoursesView.test.tsx` (update)

**Interfaces:**
- Consumes: `CourseSummary`, `CourseProgress` (contracts).
- Produces: `api.listCourses()`, `api.getCourseProgress(id)`; `CoursesView` renders real courses from the API.

- [ ] **Step 1: Create the API client**

Create `apps/web/src/api/courses.ts`:

```ts
import type { Course, CourseSummary, CourseProgress } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export async function listCourses(): Promise<CourseSummary[]> {
  const r = await apiFetch<{ courses: CourseSummary[] }>("/api/v1/courses");
  return r.courses;
}
export async function getCourse(id: string): Promise<Course> {
  const r = await apiFetch<{ course: Course }>(`/api/v1/courses/${id}`);
  return r.course;
}
export async function getCourseProgress(id: string): Promise<CourseProgress> {
  const r = await apiFetch<{ progress: CourseProgress }>(`/api/v1/courses/${id}/progress`);
  return r.progress;
}
export async function saveCourseProgress(id: string, input: { current_ordinal: number; completed_ordinals: number[] }): Promise<CourseProgress> {
  const r = await apiFetch<{ progress: CourseProgress }>(`/api/v1/courses/${id}/progress`, { method: "PUT", body: JSON.stringify(input) });
  return r.progress;
}
```

- [ ] **Step 2: Wire into the api aggregate**

In `apps/web/src/api/index.ts`: add `Course, CourseSummary, CourseProgress` to the contracts type import; `import { listCourses, getCourse, getCourseProgress, saveCourseProgress } from "./courses";`; add to `ApiClient`:

```ts
  listCourses(): Promise<CourseSummary[]>;
  getCourse(id: string): Promise<Course>;
  getCourseProgress(id: string): Promise<CourseProgress>;
  saveCourseProgress(id: string, input: { current_ordinal: number; completed_ordinals: number[] }): Promise<CourseProgress>;
```

and to the `api` object: `listCourses, getCourse, getCourseProgress, saveCourseProgress,`.

- [ ] **Step 3: Update the test**

Replace `apps/web/src/shell/courses/CoursesView.test.tsx` with a version that mocks `../../api` and asserts the real-data render:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return { ...real, api: { ...real.api, listCourses: vi.fn(), getCourseProgress: vi.fn() } };
});

import { api } from "../../api";
import { CoursesView } from "./CoursesView";

const course = { id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "从一句…出发", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 4 };

describe("CoursesView", () => {
  beforeEach(() => { vi.clearAllMocks(); });
  it("renders courses from the API", async () => {
    (api.listCourses as any).mockResolvedValue([course]);
    render(<CoursesView />);
    expect(await screen.findByText("一条网络信息，该不该信")).toBeInTheDocument();
    expect(screen.getByText("3 个任务 · 4 个工具")).toBeInTheDocument();
    expect(screen.getByText("约 40 分钟")).toBeInTheDocument();
  });
  it("renders the empty state when there are no courses", async () => {
    (api.listCourses as any).mockResolvedValue([]);
    render(<CoursesView />);
    expect(await screen.findByText("课程正在准备中，很快上线。")).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/courses/CoursesView.test.tsx`
Expected: FAIL — `CoursesView` still renders the mock fixture, not the API.

- [ ] **Step 5: Rewrite `CoursesView` to read the API**

Replace `apps/web/src/shell/courses/CoursesView.tsx`. Keep the existing header + `CourseCard` markup, but source data from the API. The new component:

```tsx
import { useEffect, useState } from "react";
import type { CourseSummary } from "@mind-imprint/contracts";
import { api } from "../../api";

const STAR = "M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z";

function CourseCard({ course, pct }: { course: CourseSummary; pct: number | null }) {
  const done = pct != null && pct >= 100;
  const tone = pct == null ? "未开始" : done ? "已学完" : "进行中";
  const cta = pct == null ? "开始学习" : done ? "回顾" : "继续";
  const toneStyle: React.CSSProperties = {
    display: "inline-flex", alignItems: "center", gap: 5, fontSize: 12, fontWeight: 700, padding: "5px 11px", borderRadius: 999,
    ...(done ? { color: "#4C9A82", background: "#E7F3EE" } : pct != null ? { color: "#2A3B7A", background: "#EDEFF9" } : { color: "#8A92A3", background: "#F1F2F5" }),
  };
  return (
    <div style={{ display: "flex", flexDirection: "column", background: "#fff", border: "1px solid #EAECF2", borderRadius: 18, overflow: "hidden", boxShadow: "0 1px 3px rgba(20,30,60,.04)" }}>
      <div style={{ position: "relative", height: 120, background: "linear-gradient(135deg,#EDEFF9,#F3F0EC)", display: "flex", alignItems: "center", justifyContent: "center" }}>
        <span style={{ position: "absolute", top: 14, left: 16, fontSize: 11, fontWeight: 700, color: "#2A3B7A", background: "rgba(255,255,255,.85)", padding: "4px 10px", borderRadius: 999 }}>{course.branch}</span>
        <div style={{ width: 56, height: 56, borderRadius: 16, background: "#fff", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "0 6px 16px rgba(42,59,122,.12)" }}>
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round"><path d={STAR} /></svg>
        </div>
      </div>
      <div style={{ flex: 1, display: "flex", flexDirection: "column", padding: "18px 22px 20px" }}>
        <div style={{ fontSize: 18, fontWeight: 800, color: "#1C2333", lineHeight: 1.4 }}>{course.title}</div>
        <div style={{ fontSize: 13, color: "#6B7384", lineHeight: 1.66, marginTop: 8 }}>{course.blurb}</div>
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 14, fontSize: 12, color: "#8A92A3", fontWeight: 600 }}>
          <span>{course.tasks_count} 个任务 · {course.tools_count} 个工具</span>
          <span style={{ display: "inline-flex", alignItems: "center", gap: 5 }}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#9AA1B0" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>
            {course.time_label}
          </span>
        </div>
        {pct != null && (
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
            <div style={{ flex: 1, height: 6, background: "#EEF0F4", borderRadius: 999, overflow: "hidden" }}>
              <div style={{ width: `${pct}%`, height: "100%", background: "#2A3B7A" }} />
            </div>
            <span style={{ fontSize: 12, color: "#8A92A3", fontWeight: 700, flex: "none" }}>{pct}%</span>
          </div>
        )}
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginTop: "auto", paddingTop: 16 }}>
          <span style={toneStyle}>{tone}</span>
          <button type="button" style={{ display: "inline-flex", alignItems: "center", gap: 6, background: "#2A3B7A", color: "#fff", border: "none", padding: "9px 15px", borderRadius: 10, fontSize: 13, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
            {cta}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  );
}

export function CoursesView() {
  const [courses, setCourses] = useState<CourseSummary[] | null>(null);
  const [pctById, setPctById] = useState<Record<string, number | null>>({});

  useEffect(() => {
    let cancelled = false;
    void api.listCourses().then((cs) => {
      if (cancelled) return;
      setCourses(cs);
      cs.forEach((c) => {
        void api.getCourseProgress(c.id).then((p) => {
          if (cancelled || c.step_count === 0) return;
          const pct = Math.round((p.completed_ordinals.length / c.step_count) * 100);
          setPctById((m) => ({ ...m, [c.id]: p.completed_ordinals.length > 0 ? pct : null }));
        }).catch(() => {});
      });
    }).catch(() => { if (!cancelled) setCourses([]); });
    return () => { cancelled = true; };
  }, []);

  return (
    <div style={{ height: "100%", minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 1000, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 13, color: "#8A92A3", fontWeight: 600 }}>课程</div>
        <div style={{ fontSize: 28, fontWeight: 800, color: "#1C2333", marginTop: 6, letterSpacing: "-0.01em" }}>系统地学会一种思考方式</div>
        <div style={{ fontSize: 14, color: "#6B7384", marginTop: 8, lineHeight: 1.6, maxWidth: 560 }}>每一门课都是一段 AI 带着你走的学习旅程——有讲解，也有你亲自上手的挑战。学完，去「批判思维」工作台把它用在你自己的问题上。</div>
        {courses != null && courses.length === 0 ? (
          <div style={{ fontSize: 14, color: "#9AA1B0", marginTop: 28 }}>课程正在准备中，很快上线。</div>
        ) : (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 20, marginTop: 28 }}>
            {(courses ?? []).map((c) => <CourseCard key={c.id} course={c} pct={pctById[c.id] ?? null} />)}
          </div>
        )}
      </div>
    </div>
  );
}
```

Then delete the fixture: `git rm apps/web/src/shell/courses/fixtures.ts`.

- [ ] **Step 6: Run — expect PASS + tsc + full suite**

Run: `cd apps/web && npx vitest run src/shell/courses/CoursesView.test.tsx && npx tsc --noEmit && npx vitest run`
Expected: PASS; tsc clean; full suite green (no other file imports `fixtures.ts` — confirm by grep if tsc flags it).

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/api/courses.ts apps/web/src/api/index.ts apps/web/src/shell/courses/CoursesView.tsx apps/web/src/shell/courses/CoursesView.test.tsx
git rm apps/web/src/shell/courses/fixtures.ts
git commit -m "feat(web): Courses grid reads real backend courses + progress (drop mock fixture)"
```

---

## Self-Review

**Spec coverage:** course/course_step/course_progress model (T2) ✅; list/get/progress API (T3) ✅; one seeded course with steps + assets + authored_content (T2 seed) ✅; Courses grid reads real data, mock dropped (T4) ✅; contracts (T1) ✅. Generation runtime + player + report are 4b/4c/4d (not here). ✅

**Placeholder scan:** none — complete SQL/Go/TS + exact commands. `make sqlc` generated-name caveats are flagged for the executor to reconcile.

**Type consistency:** DTO JSON tags (`tasks_count`/`tools_count`/`time_label`/`step_count`/`completed_ordinals`/`authored_content`) match the Zod `Course*` (T1) and the frontend usage (T4). `UpsertCourseProgressParams{UserID,CourseID,CurrentOrdinal,CompletedOrdinals []int32}` used in T2 test + T3 handler. `ListCoursesRow.StepCount` consumed in the DTO (T2) and handler (T3). Course reads require a session but no per-user ownership (shared catalog); progress is user-scoped.

**Ordering:** T1 contract → T2 DB (seed+queries+sqlc+DTO) → T3 handlers (consume T2) → T4 frontend (consume T3 API + T1 types).

## Note for executor
- `make sqlc` names are authoritative. Likely: `sqlc.ListCoursesRow` (embeds course cols + `StepCount int64`), `sqlc.CourseStep.Assets []byte`/`ChallengeType *string`/`AuthoredContent []byte`, `sqlc.CourseProgress.CompletedOrdinals []int32`, `GetCourseProgressParams{UserID,CourseID}`. If any differ, adjust the DTO/handler/test to the generated names — do not fight the generator.
- `int[]` maps to `[]int32` in pgx/sqlc by default; the seed/test use `[]int32{...}`.
- After dropping `fixtures.ts`, grep `MOCK_COURSES`/`fixtures` under `apps/web/src` to confirm nothing else imports it.

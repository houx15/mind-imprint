# Slice 4b · Course Step Render Runtime — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Generate each course step's content server-side within fixed templates — teaching via `Collect`+parse, challenge by reusing the Slice-3 `AnchorGenerator` on the step's text asset — cached per step with the step's `authored_content` as the mandatory fallback, exposed via `POST /courses/{id}/steps/{ordinal}/render`.

**Architecture:** A `RenderedStep` contract; a `course_step_render` cache table + step-by-ordinal query; `agent.RenderCourseStep` (in package `agent`, reusing `AnchorGenerator`/`stripFences`/`Material`); a render handler that entitlement-gates, reads cache, generates on miss, caches generated results (never caches the authored fallback), and always returns something.

**Tech Stack:** Go 1.26 (gateway `Collect`, `AnchorGenerator`, `materialize`, sqlc, testcontainers) + Zod. From `apps/api`: `make sqlc`, `go test`.

## Global Constraints

- **Backend + contract only.** Generation is server-side (client never calls the model). The render endpoint is token-spending → **entitlement-gated** (`HasEntitlement`, like `evaluate.go`/`turn.go`).
- **Every step always renders:** generation failure / no usable asset / parse failure → return the step's `authored_content` (`source:"authored"`). Only `source:"generated"` results are cached (a transient failure must not pin the fallback).
- **Reuse, don't reimplement:** teaching uses `gateway.Collect` + the in-package `stripFences`; challenge uses `agent.NewAnchorGenerator` on a `Material` built from the step's first text asset via `materialize.Segment`, with a synthetic `cards.Spec` whose step titles are the challenge dimensions.
- Flagship model for course generation (`a.d.EvalResolver`), per spec.
- Every task ends green + committed. `make sqlc` names are authoritative.

---

### Task 1: `RenderedStep` contract + cache table + step-by-ordinal query

**Files:**
- Modify: `packages/contracts/src/course.ts` (+ `RenderedStep`), `packages/contracts/test/course.test.ts`
- Create: `apps/api/internal/store/migrations/0013_course_step_render.sql`
- Modify: `apps/api/internal/store/queries/course.sql` (+ 3 queries)
- Generated: sqlc
- Test: `apps/api/internal/api/course_render_store_test.go`

**Interfaces:**
- Produces: Zod `RenderedStep`; sqlc `CourseStepRender`, `GetCourseStepByOrdinal`, `GetCourseStepRender`, `UpsertCourseStepRender`.

- [ ] **Step 1: Contract test + schema**

Add to `packages/contracts/test/course.test.ts` (inside the existing `describe`):

```ts
  it("parses a rendered step", async () => {
    const { RenderedStep } = await import("../src/course");
    expect(RenderedStep.parse({ ordinal: 0, kind: "teaching", template: "teaching", content: { subtitle: "x" }, source: "generated" }).source).toBe("generated");
  });
```

Add to `packages/contracts/src/course.ts` (before the type exports):

```ts
export const RenderedStep = z.object({
  ordinal: z.number().int(),
  kind: CourseStepKind,
  template: z.enum(["teaching", "challenge"]),
  content: z.unknown(),
  source: z.enum(["generated", "authored"]),
});
```
and add `export type RenderedStep = z.infer<typeof RenderedStep>;` in the types block.

Run: `cd packages/contracts && npx vitest run test/course.test.ts && npx tsc --noEmit` → PASS.

- [ ] **Step 2: Migration**

Create `apps/api/internal/store/migrations/0013_course_step_render.sql`:

```sql
-- +goose Up
CREATE TABLE course_step_render (
    course_step_id uuid PRIMARY KEY REFERENCES course_step(id) ON DELETE CASCADE,
    content        jsonb NOT NULL,
    source         text  NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE course_step_render;
```

- [ ] **Step 3: Queries**

Append to `apps/api/internal/store/queries/course.sql`:

```sql
-- name: GetCourseStepByOrdinal :one
SELECT * FROM course_step WHERE course_id = $1 AND ordinal = $2;

-- name: GetCourseStepRender :one
SELECT * FROM course_step_render WHERE course_step_id = $1;

-- name: UpsertCourseStepRender :one
INSERT INTO course_step_render (course_step_id, content, source)
VALUES ($1, $2, $3)
ON CONFLICT (course_step_id) DO UPDATE
SET content = EXCLUDED.content, source = EXCLUDED.source, created_at = now()
RETURNING *;
```

- [ ] **Step 4: Generate sqlc**

Run: `cd apps/api && make sqlc`
Expected: `CourseStepRender` model (`Content []byte`, `Source string`), `GetCourseStepByOrdinalParams{CourseID,Ordinal}`, `GetCourseStepRender`, `UpsertCourseStepRenderParams{CourseStepID,Content,Source}` generated.

- [ ] **Step 5: Round-trip test**

Create `apps/api/internal/api/course_render_store_test.go`:

```go
package api_test

import (
	"bytes"
	"context"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestCourseStepRenderCacheRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	courses, _ := q.ListCourses(ctx)
	step, err := q.GetCourseStepByOrdinal(ctx, sqlc.GetCourseStepByOrdinalParams{CourseID: courses[0].ID, Ordinal: 0})
	if err != nil {
		t.Fatalf("get step: %v", err)
	}
	if _, err := q.GetCourseStepRender(ctx, step.ID); err == nil {
		t.Fatal("expected no cache row yet")
	}
	r, err := q.UpsertCourseStepRender(ctx, sqlc.UpsertCourseStepRenderParams{CourseStepID: step.ID, Content: []byte(`{"subtitle":"x"}`), Source: "generated"})
	if err != nil || !bytes.Contains(r.Content, []byte("subtitle")) {
		t.Fatalf("upsert render: %v %s", err, r.Content)
	}
	got, err := q.GetCourseStepRender(ctx, step.ID)
	if err != nil || got.Source != "generated" {
		t.Fatalf("get render: %v %+v", err, got)
	}
}
```

Run: `cd apps/api && go build ./... && go test ./internal/api/ -run TestCourseStepRenderCacheRoundTrip` → PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/course.ts packages/contracts/test/course.test.ts apps/api/internal/store/migrations/0013_course_step_render.sql apps/api/internal/store/queries/course.sql apps/api/internal/store/sqlc/ apps/api/internal/api/course_render_store_test.go
git commit -m "feat(api): RenderedStep contract + course_step_render cache + step-by-ordinal query"
```

---

### Task 2: `agent.RenderCourseStep`

**Files:**
- Create: `apps/api/internal/agent/course.go`
- Test: `apps/api/internal/agent/course_test.go`

**Interfaces:**
- Produces:
  - `type CourseAsset struct { ID, Kind, Title, Value string }`
  - `type CourseStepInput struct { Ordinal int32; Kind, Purpose string; Assets []CourseAsset; ChallengeType *string; AuthoredContent json.RawMessage }`
  - `type RenderedStep struct { Ordinal int32; Kind, Template string; Content json.RawMessage; Source string }` (json tags `ordinal,kind,template,content,source`)
  - `func RenderCourseStep(ctx context.Context, in CourseStepInput, provider gateway.Provider, resolver gateway.KeyResolver) RenderedStep`

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/agent/course_test.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func stubResolver(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil }

func contains(raw json.RawMessage, sub string) bool { return strings.Contains(string(raw), sub) }

func TestRenderTeachingParsesGenerated(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"title":"先别急着信","subtitle":"停一下。","body":["第一段。","第二段。"],"foreground_asset_id":"a0"}`},
		{Kind: gateway.EventDone},
	}
	in := CourseStepInput{Ordinal: 0, Kind: "teaching", Purpose: "建立停一下的习惯",
		Assets:          []CourseAsset{{ID: "a0", Kind: "text", Title: "开场", Value: "一张卫星图刷屏。"}},
		AuthoredContent: json.RawMessage(`{"title":"AUTH","subtitle":"a","body":["b"],"foreground_asset_id":null}`)}
	got := RenderCourseStep(context.Background(), in, gateway.NewStubProvider(script), stubResolver)
	if got.Source != "generated" || got.Template != "teaching" {
		t.Fatalf("want generated teaching, got %+v", got)
	}
	if !contains(got.Content, "先别急着信") {
		t.Fatalf("content not from model: %s", got.Content)
	}
}

func TestRenderTeachingFallsBackOnGarbage(t *testing.T) {
	script := []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "not json"}, {Kind: gateway.EventDone}}
	in := CourseStepInput{Ordinal: 0, Kind: "teaching", Purpose: "p",
		Assets:          []CourseAsset{{ID: "a0", Kind: "text", Value: "x"}},
		AuthoredContent: json.RawMessage(`{"title":"AUTH","subtitle":"a","body":["b"]}`)}
	got := RenderCourseStep(context.Background(), in, gateway.NewStubProvider(script), stubResolver)
	if got.Source != "authored" || !contains(got.Content, "AUTH") {
		t.Fatalf("want authored fallback, got %+v", got)
	}
}

func TestRenderChallengeUsesAnchorsFromAsset(t *testing.T) {
	ct := "verify_claim"
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `[{"block_id":"b0","quote":"某科技博主综合整理","dimension":"权威性","question":"作者是谁？"}]`},
		{Kind: gateway.EventDone},
	}
	in := CourseStepInput{Ordinal: 2, Kind: "challenge", Purpose: "核查一处断言", ChallengeType: &ct,
		Assets:          []CourseAsset{{ID: "m0", Kind: "text", Title: "片段", Value: "某科技博主综合整理的文章称地球绿了 5%。"}},
		AuthoredContent: json.RawMessage(`{"title":"现在轮到你","prompt":"哪句是事实？","anchors":[],"reason_hint":"说说理由"}`)}
	got := RenderCourseStep(context.Background(), in, gateway.NewStubProvider(script), stubResolver)
	if got.Template != "challenge" || got.Source != "generated" {
		t.Fatalf("want generated challenge, got %+v", got)
	}
	if !contains(got.Content, "作者是谁？") || !contains(got.Content, "现在轮到你") {
		t.Fatalf("challenge content wrong: %s", got.Content)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/api && go test ./internal/agent/ -run TestRender`
Expected: FAIL — `RenderCourseStep`/`CourseStepInput` undefined.

- [ ] **Step 3: Implement `course.go`**

Create `apps/api/internal/agent/course.go`:

```go
package agent

import (
	"context"
	"encoding/json"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/materialize"
)

type CourseAsset struct {
	ID    string
	Kind  string
	Title string
	Value string
}

type CourseStepInput struct {
	Ordinal         int32
	Kind            string
	Purpose         string
	Assets          []CourseAsset
	ChallengeType   *string
	AuthoredContent json.RawMessage
}

type RenderedStep struct {
	Ordinal  int32           `json:"ordinal"`
	Kind     string          `json:"kind"`
	Template string          `json:"template"`
	Content  json.RawMessage `json:"content"`
	Source   string          `json:"source"`
}

func authoredStep(in CourseStepInput, template string) RenderedStep {
	c := in.AuthoredContent
	if len(c) == 0 {
		c = json.RawMessage("{}")
	}
	return RenderedStep{Ordinal: in.Ordinal, Kind: in.Kind, Template: template, Content: c, Source: "authored"}
}

// RenderCourseStep produces a step's content within a fixed template, generating
// server-side with a mandatory authored fallback so a step always renders.
func RenderCourseStep(ctx context.Context, in CourseStepInput, provider gateway.Provider, resolver gateway.KeyResolver) RenderedStep {
	if in.Kind == "challenge" {
		return renderChallenge(ctx, in, provider, resolver)
	}
	return renderTeaching(ctx, in, provider, resolver)
}

func renderTeaching(ctx context.Context, in CourseStepInput, provider gateway.Provider, resolver gateway.KeyResolver) RenderedStep {
	authored := authoredStep(in, "teaching")
	resolved, err := resolver(ctx)
	if err != nil {
		return authored
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: teachingPrompt(in)},
			{Role: gateway.RoleUser, Content: assetsText(in.Assets)},
		},
		MaxTokens: 1200,
	}
	res, err := gateway.Collect(ctx, provider, resolved, req)
	if err != nil {
		return authored
	}
	var tc struct {
		Title             string   `json:"title"`
		Subtitle          string   `json:"subtitle"`
		Body              []string `json:"body"`
		ForegroundAssetID *string  `json:"foreground_asset_id"`
	}
	if json.Unmarshal([]byte(stripFences(res.Text)), &tc) != nil || tc.Title == "" || tc.Subtitle == "" || len(tc.Body) == 0 {
		return authored
	}
	raw, err := json.Marshal(tc)
	if err != nil {
		return authored
	}
	return RenderedStep{Ordinal: in.Ordinal, Kind: in.Kind, Template: "teaching", Content: raw, Source: "generated"}
}

func renderChallenge(ctx context.Context, in CourseStepInput, provider gateway.Provider, resolver gateway.KeyResolver) RenderedStep {
	authored := authoredStep(in, "challenge")
	var frame struct {
		Title      string `json:"title"`
		Prompt     string `json:"prompt"`
		ReasonHint string `json:"reason_hint"`
	}
	_ = json.Unmarshal(in.AuthoredContent, &frame)

	var text string
	for _, a := range in.Assets {
		if a.Kind == "text" {
			text = a.Value
			break
		}
	}
	if text == "" {
		return authored
	}
	blocks := materialize.Segment(text)
	mBlocks := make([]MaterialBlock, 0, len(blocks))
	for _, b := range blocks {
		mBlocks = append(mBlocks, MaterialBlock{ID: b.ID, Text: b.Text})
	}
	materials := []Material{{ID: "m0", Title: frame.Title, Blocks: mBlocks}}
	spec := cards.Spec{Name: frame.Title, Steps: challengeDimensions(in.ChallengeType)}

	anchors, err := NewAnchorGenerator(provider, resolver).Generate(ctx, spec, materials)
	if err != nil || len(anchors) == 0 {
		return authored
	}
	content := map[string]any{"title": frame.Title, "prompt": frame.Prompt, "anchors": anchors, "reason_hint": frame.ReasonHint}
	raw, err := json.Marshal(content)
	if err != nil {
		return authored
	}
	return RenderedStep{Ordinal: in.Ordinal, Kind: in.Kind, Template: "challenge", Content: raw, Source: "generated"}
}

func teachingPrompt(in CourseStepInput) string {
	return "你是一名循循善诱的老师。这一步的教学目的是：" + in.Purpose +
		"。请基于下面的素材，写一小段讲解。\n" +
		"只输出 JSON：{\"title\":\"这页的小标题\",\"subtitle\":\"一句话的口播导语\",\"body\":[\"第一段\",\"第二段\"],\"foreground_asset_id\":\"要重点展示的素材 id 或 null\"}。不要输出任何多余文字。"
}

func assetsText(assets []CourseAsset) string {
	out := "# 素材\n"
	for _, a := range assets {
		out += "[" + a.ID + " · " + a.Kind + "] " + a.Title + "：" + a.Value + "\n"
	}
	return out
}

func challengeDimensions(challengeType *string) []cards.Step {
	if challengeType != nil && *challengeType == "verify_claim" {
		return []cards.Step{
			{Title: "权威性 · Authority"},
			{Title: "准确性 · Accuracy"},
			{Title: "目的性 · Purpose"},
		}
	}
	return []cards.Step{{Title: "仔细看这段材料"}}
}
```

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/api && go test ./internal/agent/ -run TestRender`
Expected: PASS (3 real tests).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/course.go apps/api/internal/agent/course_test.go
git commit -m "feat(api): RenderCourseStep — teaching Collect+parse, challenge via AnchorGenerator, authored fallback"
```

---

### Task 3: Render endpoint (cache + generate + entitlement)

**Files:**
- Create: `apps/api/internal/api/course_render.go`
- Modify: `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/course_render_test.go`

**Interfaces:**
- Consumes: `RenderCourseStep` (Task 2), the cache queries + `GetCourseStepByOrdinal` (Task 1), `HasEntitlement`.
- Produces: handler `renderCourseStep` at `POST /api/v1/courses/{id}/steps/{ordinal}/render`.

- [ ] **Step 1: Write the handler test**

Create `apps/api/internal/api/course_render_test.go`:

```go
package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestRenderCourseStepGeneratesAndCaches(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	courses, _ := q.ListCourses(context.Background())
	id := courses[0].ID.String()

	// Stub provider returns valid teaching JSON for ordinal 0 (a teaching step).
	stub := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"title":"先别急着信","subtitle":"停一下。","body":["一段。"],"foreground_asset_id":"a0"}`},
		{Kind: gateway.EventDone},
	})
	h := New(Deps{Queries: q, Pool: pool, Provider: stub,
		ChatResolver: func(context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil },
		EvalResolver: func(context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil }}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+id+"/steps/0/render", nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"generated"`)) || !bytes.Contains(rec.Body.Bytes(), []byte("先别急着信")) {
		t.Fatalf("render: %d %s", rec.Code, rec.Body)
	}

	// Cache row now exists.
	step, _ := q.GetCourseStepByOrdinal(context.Background(), sqlc.GetCourseStepByOrdinalParams{CourseID: courses[0].ID, Ordinal: 0})
	if _, err := q.GetCourseStepRender(context.Background(), step.ID); err != nil {
		t.Fatalf("expected cache row after render: %v", err)
	}

	// Unknown ordinal → 404.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+id+"/steps/99/render", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown ordinal: want 404 got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/api && go test ./internal/api/ -run TestRenderCourseStep`
Expected: FAIL — route/handler undefined (404 on the render path even for ordinal 0).

- [ ] **Step 3: Implement the handler**

Create `apps/api/internal/api/course_render.go`:

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

func (a *API) renderCourseStep(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loadCourse(w, r)
	if !ok {
		return
	}
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
	ord, err := strconv.Atoi(r.PathValue("ordinal"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	step, err := a.d.Queries.GetCourseStepByOrdinal(r.Context(), sqlc.GetCourseStepByOrdinalParams{CourseID: c.ID, Ordinal: int32(ord)})
	if err != nil {
		httpx.WriteError(w, r, err) // ErrNoRows → 404
		return
	}

	// Cache hit → return it.
	if cached, err := a.d.Queries.GetCourseStepRender(r.Context(), step.ID); err == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rendered": map[string]any{
			"ordinal": step.Ordinal, "kind": step.Kind,
			"template": step.Kind, "content": json.RawMessage(cached.Content), "source": cached.Source,
		}})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}

	// Miss → generate.
	assets := []agent.CourseAsset{}
	_ = json.Unmarshal(step.Assets, &assets) // best-effort; empty on error
	in := agent.CourseStepInput{
		Ordinal: step.Ordinal, Kind: step.Kind, Purpose: step.Purpose,
		Assets: assets, ChallengeType: step.ChallengeType, AuthoredContent: json.RawMessage(step.AuthoredContent),
	}
	rendered := agent.RenderCourseStep(r.Context(), in, a.d.Provider, a.d.EvalResolver)

	// Cache only successful generations (never pin the authored fallback).
	if rendered.Source == "generated" {
		_, _ = a.d.Queries.UpsertCourseStepRender(r.Context(), sqlc.UpsertCourseStepRenderParams{
			CourseStepID: step.ID, Content: []byte(rendered.Content), Source: rendered.Source,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rendered": rendered})
}
```

Note: `agent.CourseAsset`'s JSON tags — the step's `assets` jsonb is `[{"id","kind","title","value"}]`. Add JSON tags to `agent.CourseAsset` in `course.go` (Task 2) so `json.Unmarshal(step.Assets, &assets)` maps correctly: `ID string \`json:"id"\``, `Kind string \`json:"kind"\``, `Title string \`json:"title"\``, `Value string \`json:"value"\``. **Update Task 2's `CourseAsset` struct to carry these json tags** (add them when implementing Task 2).

- [ ] **Step 4: Register the route**

In `apps/api/internal/api/api.go` protected group, add:

```go
	mux.Handle("POST /api/v1/courses/{id}/steps/{ordinal}/render", protected(a.renderCourseStep))
```

- [ ] **Step 5: Run — expect PASS + build/vet/short**

Run: `cd apps/api && go test ./internal/api/ -run TestRenderCourseStep && go build ./... && go vet ./... && go test -short ./...`
Expected: PASS; clean.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/course_render.go apps/api/internal/api/api.go apps/api/internal/api/course_render_test.go
git commit -m "feat(api): course step render endpoint (cache + generate + entitlement + fallback)"
```

---

## Self-Review

**Spec coverage:** render endpoint (T3) ✅; teaching Collect+parse + challenge via AnchorGenerator + authored fallback (T2) ✅; per-step cache, only-cache-generated (T3) ✅; entitlement gate (T3) ✅; RenderedStep contract + cache table + step-by-ordinal (T1) ✅; flagship resolver (T3 uses `EvalResolver`) ✅.

**Placeholder scan:** T2 Step 1 flags a leftover empty stub to remove — explicit instruction, not a code placeholder. Everything else is complete code.

**Type consistency:** `agent.CourseAsset` json tags (`id/kind/title/value`) match the `assets` jsonb + the CourseAsset contract; `CourseStepInput`/`RenderedStep` produced in T2, consumed in T3. `GetCourseStepByOrdinalParams{CourseID,Ordinal int32}`, `UpsertCourseStepRenderParams{CourseStepID,Content,Source}` from T1 used in T3. `stripFences`/`Material`/`MaterialBlock`/`NewAnchorGenerator`/`cards.Step` reused from Slice 3 (same `agent` package).

**Ordering:** T1 (contract+cache+query) → T2 (runtime, independent) → T3 (handler, consumes T1+T2).

## Note for executor
- **When implementing Task 2, add the json tags to `CourseAsset`** (`id/kind/title/value`) — Task 3's handler unmarshals `step.Assets` into `[]agent.CourseAsset` and relies on them.
- `make sqlc` names are authoritative; if `GetCourseStepByOrdinalParams`/`UpsertCourseStepRenderParams` differ, adjust T3.
- The render handler reuses `a.loadCourse` (from 4a) for the `{id}` 404, then the step-by-ordinal for the `{ordinal}` 404.

# N1 · Close the Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the project-creation funnel that doesn't exist today — a student pastes an assignment, `POST /projects` atomically creates the project + its S0 onboarding nodes from a board-static 0457 fixture, they land in a live S0 任务解码 station, submit their restate + weak-picks, and reach the existing write→finish loop.

**Architecture:** Migration-free (rides `graph_node`'s open `type`). A Go-embedded 0457 onboarding fixture feeds a new atomic `POST /projects` handler (project row + `assignment_brief`/`rubric_translation`/`milestone_plan` nodes) and a new `POST /projects/{id}/onboarding` handler (persists a `task_restatement` node + event). The projection surfaces the new fields; the web gains a directory + create flow and wires the existing (display-only) OnboardingView to persist.

**Tech Stack:** Go (`net/http`, sqlc, pgx tx), PostgreSQL, React + Vite + TS, Zod, vitest + @testing-library/react, testcontainers-go.

## Global Constraints

- **No model call** anywhere in N1 (fixture-driven); the endpoints write **zero `llm_call` rows**.
- **No migration / no schema change.** `graph_node.type` is open (`CHECK type <> ''`, migration 0017); `author` ∈ {`student`,`ai`,`imported`}. Node types used: `assignment_brief` (author `imported`), `rubric_translation` (`ai`), `milestone_plan` (`ai`), `task_restatement` (`student`).
- **Atomic creation:** project + its 3 seed nodes are ONE pgx transaction (`a.d.Pool.Begin` → `a.d.Queries.WithTx(tx)` → `tx.Commit`, `defer tx.Rollback`), mirroring `materials.go:107-138`.
- **Owner isolation:** create uses the session user; onboarding-submit + all reads go through the existing `loadOwnedProject` (404 hides other users').
- **`HasEntitlement(ctx, u)` gates both new write endpoints** (402 if not entitled), like every write path.
- **过程即数据:** the student's restate persists as BOTH a `task_restatement` node AND an `onboarding_restated` event.
- **Single source:** the 0457 onboarding content lives in ONE Go-embedded fixture; the web receives it via the projection, never re-encodes it.
- `make sqlc` from `apps/api` (`CGO_ENABLED=0`) — but N1 adds **no** query, so sqlc likely won't change; never hand-edit `internal/store/sqlc/*`.
- Go tests: `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`. Run FULL packages, never `-run` subsets, for the final gate. Web tests from `apps/web`, contracts from `packages/contracts`. Inline SVG only.
- **NEVER `git add` a bare dir or the pre-existing untracked user files** (`M package.json` + untracked `docs/` and repo-root files). Add only each task's exact files.
- The 0457 fixture's `restate_prompt` must be **generic** (usable for any assignment), NOT the seed's China-specific text.

---

### Task 1: 0457 onboarding fixture + loader

**Files:**
- Create: `apps/api/internal/onboarding/fixtures/0457.json`
- Create: `apps/api/internal/onboarding/onboarding.go`
- Test: `apps/api/internal/onboarding/onboarding_test.go`

**Interfaces:**
- Produces: `onboarding.Load(qualification string) (onboarding.Fixture, bool)` where `Fixture{ RestatePrompt string; Rows []Row; Steps []string }` and `Row{ Official, Plain string; Weak bool }` (JSON tags `restate_prompt`/`rows`/`steps`, `official`/`plain`/`weak`). Consumed by Task 2's create handler.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/onboarding/onboarding_test.go`:

```go
package onboarding

import "testing"

func TestLoad0457(t *testing.T) {
	f, ok := Load("0457")
	if !ok {
		t.Fatal("Load(\"0457\") = false, want a fixture")
	}
	if len(f.RestatePrompt) < 10 {
		t.Errorf("RestatePrompt too short: %q", f.RestatePrompt)
	}
	if len(f.Rows) != 4 {
		t.Fatalf("Rows = %d, want 4", len(f.Rows))
	}
	if f.Rows[0].Official == "" || f.Rows[0].Plain == "" {
		t.Errorf("row 0 has empty official/plain: %+v", f.Rows[0])
	}
	if len(f.Steps) == 0 {
		t.Error("Steps empty")
	}
}

func TestLoadUnknownQualification(t *testing.T) {
	if _, ok := Load("9999"); ok {
		t.Error("Load(\"9999\") = true, want false (only 0457 exists)")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/onboarding/`
Expected: FAIL — package/`Load` doesn't exist.

- [ ] **Step 3: Create the fixture JSON**

Create `apps/api/internal/onboarding/fixtures/0457.json` (content authored from seed 0018's rubric rows/steps, but with a GENERIC restate prompt):

```json
{
  "restate_prompt": "用你自己的话说说：这道题到底在问什么？你打算怎么回答？评分标准里，你觉得最容易被忽略的是哪一条？",
  "rows": [
    { "official": "Analysis of different perspectives", "plain": "能从不同视角分析，不只罗列观点", "weak": true },
    { "official": "Use & evaluation of evidence / sources", "plain": "用可信来源，并说清它可不可信", "weak": true },
    { "official": "Personal response & reflection", "plain": "给出自己的判断，并回看研究过程", "weak": true },
    { "official": "Communication & organisation", "plain": "结构清楚、表达清晰", "weak": false }
  ],
  "steps": ["立题", "找素材", "评估来源", "搭论证", "成稿", "反思归档"]
}
```

- [ ] **Step 4: Write the loader**

Create `apps/api/internal/onboarding/onboarding.go`:

```go
// Package onboarding provides board-static S0 任务解码 content used to seed a
// new project's onboarding graph nodes at creation time. No model call — the
// content is authored per qualification and embedded. This is the single-source
// seam that multi-board (N4) extends by adding more fixture files + Load cases.
package onboarding

import (
	_ "embed"
	"encoding/json"
)

//go:embed fixtures/0457.json
var fixture0457 []byte

// Row is one rubric criterion translated to plain language. Weak is a suggested
// watch-flag from the fixture — distinct from the student's own picks.
type Row struct {
	Official string `json:"official"`
	Plain    string `json:"plain"`
	Weak     bool   `json:"weak"`
}

// Fixture is the S0 onboarding content for one qualification.
type Fixture struct {
	RestatePrompt string   `json:"restate_prompt"`
	Rows          []Row    `json:"rows"`
	Steps         []string `json:"steps"`
}

// Load returns the onboarding fixture for a qualification, or ok=false if none
// exists. Only 0457 is authored today (N4 adds boards).
func Load(qualification string) (Fixture, bool) {
	if qualification != "0457" {
		return Fixture{}, false
	}
	var f Fixture
	if err := json.Unmarshal(fixture0457, &f); err != nil {
		return Fixture{}, false
	}
	return f, true
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/onboarding/`
Expected: PASS (no testcontainers needed — pure).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/onboarding/onboarding.go apps/api/internal/onboarding/fixtures/0457.json apps/api/internal/onboarding/onboarding_test.go
git commit -m "feat(n1): 0457 onboarding fixture + loader"
```

---

### Task 2: `POST /api/v1/projects` — atomic create

**Files:**
- Create: `apps/api/internal/api/project_create.go`
- Modify: `apps/api/internal/api/api.go` (add the POST route)
- Test: `apps/api/internal/api/project_create_test.go`

**Interfaces:**
- Consumes: `onboarding.Load` (Task 1); `a.d.Pool`, `a.d.Queries` (sqlc `CreateProjectParams{UserID uuid.UUID, Qualification, Title string, Deadline pgtype.Timestamptz, BoardCfgVer int32}` → `Project`; `InsertGraphNodeParams{ProjectID uuid.UUID, Type string, Body []byte, Author string, SpanRef []byte}`); `HasEntitlement`, `UserFromContext`, `httpx`.
- Produces: `POST /api/v1/projects` returning `201 {"id": "<uuid>"}`. The created project has `qualification="0457"` and 3 graph nodes (`assignment_brief`/`imported`, `rubric_translation`/`ai`, `milestone_plan`/`ai`).

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/api/project_create_test.go` (harness mirrors `ability_test.go` / other api_test files — `newAPITestPool`, `signInSeed`, `withCookie`, `Deps`/`New(...).Handler()`):

```go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func TestCreateProject_SeedsOnboardingNodes(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	body := strings.NewReader(`{"title":"我的论文","prompt":"讨论社交媒体对青少年注意力的影响"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct{ ID string `json:"id"` }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	pid := uuid.MustParse(out.ID)

	// The project exists, owned by the seed user, qualification 0457.
	var qual string
	if err := pool.QueryRow(context.Background(),
		`SELECT qualification FROM project WHERE id=$1`, pid).Scan(&qual); err != nil {
		t.Fatalf("project row missing: %v", err)
	}
	if qual != "0457" {
		t.Errorf("qualification = %q, want 0457", qual)
	}
	// Exactly the three onboarding nodes, correct types + authors.
	rows, err := pool.Query(context.Background(),
		`SELECT type, author FROM graph_node WHERE project_id=$1 ORDER BY type`, pid)
	if err != nil { t.Fatalf("query nodes: %v", err) }
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var typ, author string
		if err := rows.Scan(&typ, &author); err != nil { t.Fatal(err) }
		got[typ] = author
	}
	if got["assignment_brief"] != "imported" || got["rubric_translation"] != "ai" || got["milestone_plan"] != "ai" {
		t.Fatalf("onboarding nodes wrong: %+v", got)
	}
}

func TestCreateProject_EmptyPromptRejected(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(`{"prompt":""}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty prompt = %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestCreateProject`
Expected: FAIL — route 404.

- [ ] **Step 3: Write the handler**

Create `apps/api/internal/api/project_create.go`:

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/onboarding"
	"mindimprint/api/internal/store/sqlc"
)

// createProject is the funnel entry: it atomically creates a project and seeds
// its S0 任务解码 onboarding nodes from the board-static fixture. No model call.
func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		Title  string `json:"title"`
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_prompt", "请先贴上任务要求。", nil))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "未命名论文"
	}

	const qualification = "0457"
	fx, ok := onboarding.Load(qualification)
	if !ok {
		httpx.WriteError(w, r, errors.New("onboarding fixture missing for qualification "+qualification))
		return
	}
	rubricBody, _ := json.Marshal(map[string]any{"restate_prompt": fx.RestatePrompt, "rows": fx.Rows})
	planBody, _ := json.Marshal(map[string]any{"steps": fx.Steps})
	briefBody, _ := json.Marshal(map[string]any{"text": prompt})

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	proj, err := qtx.CreateProject(r.Context(), sqlc.CreateProjectParams{
		UserID: u.ID, Qualification: qualification, Title: title,
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, n := range []struct {
		typ, author string
		body        []byte
	}{
		{"assignment_brief", "imported", briefBody},
		{"rubric_translation", "ai", rubricBody},
		{"milestone_plan", "ai", planBody},
	} {
		if _, err := qtx.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
			ProjectID: proj.ID, Type: n.typ, Body: n.body, Author: n.author, SpanRef: nil,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": proj.ID.String()})
}
```

**httpx idiom (verified):** errors go through `httpx.WriteError(w, r, err)`. Use `httpx.ErrBadRequest(code, msg, nil)` for 400 (like `materials.go:71`/`signup.go:35`), `httpx.ErrNotEntitled()` for the entitlement deny (like `materials.go`; it is a 403 `not_entitled` — the codebase's canonical entitlement refusal, use it verbatim rather than a 402), and a plain `errors.New(...)` for the never-should-happen missing-fixture case (WriteError maps unknown errors to 500).

- [ ] **Step 4: Register the route**

In `apps/api/internal/api/api.go`, beside the project routes (after `GET /api/v1/projects`):

```go
	mux.Handle("POST /api/v1/projects", protected(a.createProject))
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestCreateProject`
Expected: PASS. Then run the FULL package once before committing: `go test -p 1 ./internal/api/`.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/project_create.go apps/api/internal/api/api.go apps/api/internal/api/project_create_test.go
git commit -m "feat(n1): POST /projects — atomic create + fixture-seeded onboarding nodes"
```

---

### Task 3: `POST /api/v1/projects/{id}/onboarding` — persist restate + picks

**Files:**
- Create: `apps/api/internal/api/onboarding_submit.go`
- Modify: `apps/api/internal/api/api.go` (add the route)
- Test: `apps/api/internal/api/onboarding_submit_test.go`

**Interfaces:**
- Consumes: `loadOwnedProject` (returns `(uuid.UUID, bool)`), `HasEntitlement`, `a.d.Queries.InsertGraphNode`, `agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)` → `AppendEvent(agent.EventRow{ProjectID uuid.UUID, Surface, Type string, Payload []byte})`.
- Produces: `POST /api/v1/projects/{id}/onboarding` — persists a `task_restatement`/`student` node `{restate, weak_picks}` + an `onboarding_restated` studio event `{text}`; returns `200`.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/api/onboarding_submit_test.go`. It first creates a project via the Task-2 endpoint (so a real project exists), then submits onboarding:

```go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func createProjectForTest(t *testing.T, h http.Handler, cookie *http.Cookie) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects",
		strings.NewReader(`{"title":"T","prompt":"某个任务要求"}`)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed create = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct{ ID string `json:"id"` }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.ID
}

func TestSubmitOnboarding_PersistsNodeAndEvent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/onboarding",
		strings.NewReader(`{"restate":"这道题在问社交媒体是否影响注意力","weakPicks":[0,2]}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var nodes, events int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='task_restatement' AND author='student'`, pid).Scan(&nodes)
	if nodes != 1 {
		t.Errorf("task_restatement nodes = %d, want 1", nodes)
	}
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE project_id=$1 AND type='onboarding_restated'`, pid).Scan(&events)
	if events != 1 {
		t.Errorf("onboarding_restated events = %d, want 1", events)
	}
}

func TestSubmitOnboarding_ShortRestateRejected(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/onboarding",
		strings.NewReader(`{"restate":"太短","weakPicks":[]}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short restate = %d, want 400", rec.Code)
	}
}
```

**(Verified:** the event-stream table is `event`, singular — `0016_refactor2_foundations.sql:140`. The test's `FROM event` is correct.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestSubmitOnboarding`
Expected: FAIL — route 404.

- [ ] **Step 3: Write the handler**

Create `apps/api/internal/api/onboarding_submit.go`:

```go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// submitOnboarding persists the student's S0 work: their own restate (≥15 runes)
// and which rubric rows they judged weakest. Recorded as BOTH a student graph
// node (so it survives reload / feeds the graph) and a studio event (过程即数据 →
// the assessor). No model call.
func (a *API) submitOnboarding(w http.ResponseWriter, r *http.Request) {
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
	projectID, ok := a.loadOwnedProject(w, r) // writes 404 itself on miss
	if !ok {
		return
	}
	var req struct {
		Restate   string `json:"restate"`
		WeakPicks []int  `json:"weakPicks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	if utf8.RuneCountInString(req.Restate) < 15 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("restate_too_short", "用自己的话多写一点（至少 15 字）。", nil))
		return
	}
	if len(req.WeakPicks) > 2 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("too_many_picks", "最多选 2 条。", nil))
		return
	}
	if req.WeakPicks == nil {
		req.WeakPicks = []int{}
	}

	nodeBody, _ := json.Marshal(map[string]any{"restate": req.Restate, "weak_picks": req.WeakPicks})
	if _, err := a.d.Queries.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "task_restatement", Body: nodeBody, Author: "student", SpanRef: nil,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Best-effort telemetry, exactly like studioturn's prompt_sent event.
	payload, _ := json.Marshal(map[string]string{"text": req.Restate})
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "onboarding_restated", Payload: payload,
	}); err != nil {
		slog.Warn("onboarding submit: append event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
}
```

- [ ] **Step 4: Register the route**

In `apps/api/internal/api/api.go`, with the other `POST /api/v1/projects/{id}/...` routes:

```go
	mux.Handle("POST /api/v1/projects/{id}/onboarding", protected(a.submitOnboarding))
```

- [ ] **Step 5: Run to verify pass + full package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestSubmitOnboarding` then the FULL package `go test -p 1 ./internal/api/`.
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/onboarding_submit.go apps/api/internal/api/api.go apps/api/internal/api/onboarding_submit_test.go
git commit -m "feat(n1): POST /projects/{id}/onboarding — persist restate + weak-picks (node + event)"
```

---

### Task 4: Projection — surface assignment + student restate

**Files:**
- Modify: `apps/api/internal/studio/projection.go` (`projectOnboarding`)
- Modify: `apps/api/internal/studio/dto.go` (`OnboardingDTO`)
- Modify: `apps/api/internal/studio/dto_parity_test.go` (add the 3 fields)
- Test: `apps/api/internal/studio/projection_test.go` (add a case) OR a new `onboarding_projection_test.go`

**Interfaces:**
- Consumes: `d.Nodes` (graph nodes) with the new types `assignment_brief` (body `{text}`) and `task_restatement` (body `{restate, weak_picks}`).
- Produces: `OnboardingDTO` gains `AssignmentText string json:"assignmentText"`, `StudentRestate string json:"studentRestate"`, `StudentWeakPicks []int json:"studentWeakPicks"`.

- [ ] **Step 1: Write the failing test**

Add to a studio test file (new `apps/api/internal/studio/onboarding_projection_test.go`):

```go
package studio

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestProjectOnboarding_ReadsAssignmentAndLatestRestate(t *testing.T) {
	// ProjectData.Nodes is []sqlc.GraphNode (projection.go:21); Body is []byte.
	d := ProjectData{Nodes: []sqlc.GraphNode{
		{Type: "assignment_brief", Body: []byte(`{"text":"讨论 X"}`)},
		{Type: "rubric_translation", Body: []byte(`{"restate_prompt":"P","rows":[{"official":"O","plain":"p","weak":true}]}`)},
		{Type: "task_restatement", Body: []byte(`{"restate":"第一版","weak_picks":[0]}`)},
		{Type: "task_restatement", Body: []byte(`{"restate":"最终版","weak_picks":[1,2]}`)},
	}}
	ob := projectOnboarding(d)
	if ob.AssignmentText != "讨论 X" {
		t.Errorf("AssignmentText = %q, want 讨论 X", ob.AssignmentText)
	}
	if ob.StudentRestate != "最终版" {
		t.Errorf("StudentRestate = %q, want 最终版 (latest node wins)", ob.StudentRestate)
	}
	if len(ob.StudentWeakPicks) != 2 || ob.StudentWeakPicks[0] != 1 {
		t.Errorf("StudentWeakPicks = %v, want [1 2]", ob.StudentWeakPicks)
	}
	if ob.RestatePrompt != "P" || len(ob.RubricRows) != 1 {
		t.Errorf("existing rubric fields regressed: %+v", ob)
	}
}
```

**(Verified:** `ProjectData.Nodes` is `[]sqlc.GraphNode` (`projection.go:21`); `sqlc.GraphNode` has `Type string` and `Body []byte`. The test uses those directly.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/ -run TestProjectOnboarding`
Expected: FAIL — fields don't exist.

- [ ] **Step 3: Extend the DTO**

In `apps/api/internal/studio/dto.go`, add to `OnboardingDTO` (after `PlanSteps`):

```go
	AssignmentText   string `json:"assignmentText"`
	StudentRestate   string `json:"studentRestate"`
	StudentWeakPicks []int  `json:"studentWeakPicks"`
```

- [ ] **Step 4: Extend the reader**

In `apps/api/internal/studio/projection.go`, in `projectOnboarding`: initialize `StudentWeakPicks: []int{}` in the `ob := OnboardingDTO{…}` literal, and add two cases to the `switch n.Type`:

```go
		case "assignment_brief":
			var body struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(n.Body, &body) == nil && body.Text != "" {
				ob.AssignmentText = body.Text
			}
		case "task_restatement":
			var body struct {
				Restate   string `json:"restate"`
				WeakPicks []int  `json:"weak_picks"`
			}
			// Nodes arrive ordered by created_at (ListGraphNodesByProject),
			// so the last task_restatement seen wins — latest restate.
			if json.Unmarshal(n.Body, &body) == nil {
				ob.StudentRestate = body.Restate
				ob.StudentWeakPicks = body.WeakPicks
				if ob.StudentWeakPicks == nil {
					ob.StudentWeakPicks = []int{}
				}
			}
```

- [ ] **Step 5: Update dto-parity + run studio package**

Add the 3 new fields to whatever `dto_parity_test.go` asserts for `OnboardingDTO` (match its existing style). Then run the FULL studio package:
Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/studio/projection.go apps/api/internal/studio/dto.go apps/api/internal/studio/dto_parity_test.go apps/api/internal/studio/onboarding_projection_test.go
git commit -m "feat(n1): projection surfaces assignmentText + student restate/weak-picks"
```

---

### Task 5: Contracts — onboarding fields + create/submit bodies

**Files:**
- Modify: `packages/contracts/src/studioState.ts` (`OnboardingFx` + new schemas)
- Test: `packages/contracts/test/n1Contracts.test.ts` (create; note contracts tests live in `test/`, not `src/`)

**Interfaces:**
- Produces: `OnboardingFx` gains `assignmentText`, `studentRestate`, `studentWeakPicks`; new `CreateProjectBody`, `CreateProjectResult`, `OnboardingSubmitBody`. Consumed by Task 6.

- [ ] **Step 1: Write the failing test**

Create `packages/contracts/test/n1Contracts.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { OnboardingFx, CreateProjectBody, OnboardingSubmitBody } from "../src/studioState";

describe("N1 contracts", () => {
  it("OnboardingFx parses the new fields", () => {
    const ob = OnboardingFx.parse({
      restatePrompt: "P", rubricRows: [], planSteps: [],
      assignmentText: "讨论 X", studentRestate: "我的理解", studentWeakPicks: [0, 2],
    });
    expect(ob.assignmentText).toBe("讨论 X");
    expect(ob.studentWeakPicks).toEqual([0, 2]);
  });
  it("CreateProjectBody requires a non-empty prompt", () => {
    expect(() => CreateProjectBody.parse({ prompt: "" })).toThrow();
    expect(CreateProjectBody.parse({ prompt: "x" }).title).toBeUndefined();
  });
  it("OnboardingSubmitBody shape", () => {
    const b = OnboardingSubmitBody.parse({ restate: "r", weakPicks: [1] });
    expect(b.weakPicks).toEqual([1]);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd packages/contracts && npx vitest run test/n1Contracts.test.ts`
Expected: FAIL — `assignmentText` unknown / new schemas missing.

- [ ] **Step 3: Extend `OnboardingFx` + add schemas**

In `packages/contracts/src/studioState.ts`, add the 3 fields to the `OnboardingFx` object (after `planSteps`):

```ts
  assignmentText: z.string(),
  studentRestate: z.string(),
  studentWeakPicks: z.array(z.number().int()),
```

And append, near the end of the file:

```ts
export const CreateProjectBody = z.object({
  title: z.string().optional(),
  prompt: z.string().min(1),
});
export type CreateProjectBody = z.infer<typeof CreateProjectBody>;

export const CreateProjectResult = z.object({ id: z.string() });
export type CreateProjectResult = z.infer<typeof CreateProjectResult>;

export const OnboardingSubmitBody = z.object({
  restate: z.string(),
  weakPicks: z.array(z.number().int()),
});
export type OnboardingSubmitBody = z.infer<typeof OnboardingSubmitBody>;
```

(`studioState.ts` is already barrel-exported via `export * from "./studioState"` — verify in `index.ts`; if not, add it.)

- [ ] **Step 4: Run to verify pass**

Run: `cd packages/contracts && npx vitest run test/n1Contracts.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/studioState.ts packages/contracts/test/n1Contracts.test.ts
git commit -m "feat(n1): contracts — onboarding fields + create/submit bodies"
```

---

### Task 6: Web API clients — createProject + submitOnboarding

**Files:**
- Modify: `apps/web/src/api/projects.ts`
- Modify: `apps/web/src/api/index.ts` (facade: 2 methods + type import)
- Test: `apps/web/src/api/projects.test.ts` (create if absent; else extend)

**Interfaces:**
- Consumes: `CreateProjectResult`, `apiFetch` (POST via `{ method: "POST", body: JSON.stringify(...) }`; sets `Content-Type` itself).
- Produces: `createProject(body: { title?: string; prompt: string }): Promise<{ id: string }>`; `submitOnboarding(projectId: string, body: { restate: string; weakPicks: number[] }): Promise<void>`. Both on the `ApiClient` facade.

- [ ] **Step 1: Write the failing test**

Create/extend `apps/web/src/api/projects.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { createProject, submitOnboarding } from "./projects";

afterEach(() => { vi.restoreAllMocks(); });

describe("createProject", () => {
  it("posts the body and parses the id", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ id: "p-123" }), { status: 201, headers: { "Content-Type": "application/json" } }));
    const out = await createProject({ title: "T", prompt: "讨论 X" });
    expect(out.id).toBe("p-123");
    const [, init] = spy.mock.calls[0]!;
    expect(init?.method).toBe("POST");
    expect(JSON.parse(init?.body as string).prompt).toBe("讨论 X");
  });
});

describe("submitOnboarding", () => {
  it("posts restate + weakPicks", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValue(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));
    await submitOnboarding("p-1", { restate: "我的理解够长了吗", weakPicks: [0, 1] });
    const [url, init] = spy.mock.calls[0]!;
    expect(String(url)).toContain("/projects/p-1/onboarding");
    expect(JSON.parse(init?.body as string).weakPicks).toEqual([0, 1]);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run src/api/projects.test.ts`
Expected: FAIL — exports don't exist.

- [ ] **Step 3: Add the clients**

In `apps/web/src/api/projects.ts` add (and add `CreateProjectResult` to the `@mind-imprint/contracts` import):

```ts
import { CreateProjectResult, DualAxisReport, StudioProjection } from "@mind-imprint/contracts";

export async function createProject(body: { title?: string; prompt: string }): Promise<{ id: string }> {
  const raw = await apiFetch<unknown>(`/api/v1/projects`, { method: "POST", body: JSON.stringify(body) });
  return CreateProjectResult.parse(raw);
}

export async function submitOnboarding(projectId: string, body: { restate: string; weakPicks: number[] }): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/onboarding`, { method: "POST", body: JSON.stringify(body) });
}
```

- [ ] **Step 4: Wire the facade**

In `apps/web/src/api/index.ts`: import `createProject, submitOnboarding` from `./projects`; add to the `ApiClient` interface `createProject(body: { title?: string; prompt: string }): Promise<{ id: string }>;` and `submitOnboarding(projectId: string, body: { restate: string; weakPicks: number[] }): Promise<void>;`; add both to the `api` object literal. (Additive — don't disturb existing entries.)

- [ ] **Step 5: Run test + typecheck**

Run: `cd apps/web && npx vitest run src/api/projects.test.ts && npx tsc --noEmit`
Expected: test PASS; `tsc` no NEW errors (one pre-existing `interactionPrimitive.test.ts` error is known/unrelated).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/api/projects.ts apps/web/src/api/index.ts apps/web/src/api/projects.test.ts
git commit -m "feat(n1): web createProject + submitOnboarding clients + facade"
```

---

### Task 7: Directory + create flow

**Files:**
- Create: `apps/web/src/studio/Directory.tsx`
- Modify: `apps/web/src/studio/StudioContainer.tsx` (directory-first routing + create handler)
- Test: `apps/web/src/studio/Directory.test.tsx`

**Interfaces:**
- Consumes: `ProjectListItem` (from `../api/projects`), the `createProject` client (via `api`).
- Produces: `Directory` — a presentational component `Directory({ projects, onOpen, onCreate, creating }: { projects: ProjectListItem[]; onOpen: (id: string) => void; onCreate: (body: { title: string; prompt: string }) => void; creating: boolean })`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/studio/Directory.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { Directory } from "./Directory";

describe("Directory", () => {
  it("lists projects and opens one", () => {
    const onOpen = vi.fn();
    render(<Directory projects={[{ id: "p1", title: "我的论文", qualLabel: "0457", activeStation: "任务解码" }]} onOpen={onOpen} onCreate={() => {}} creating={false} />);
    fireEvent.click(screen.getByText("我的论文"));
    expect(onOpen).toHaveBeenCalledWith("p1");
  });

  it("creates from the new-project form", () => {
    const onCreate = vi.fn();
    render(<Directory projects={[]} onOpen={() => {}} onCreate={onCreate} creating={false} />);
    fireEvent.click(screen.getByRole("button", { name: /新建论文/ }));
    fireEvent.change(screen.getByPlaceholderText(/贴上|任务|题目/), { target: { value: "讨论社交媒体" } });
    fireEvent.click(screen.getByRole("button", { name: /开始|创建|新建论文/ }).closest("form") ? screen.getByRole("button", { name: /^开始$|创建/ }) : screen.getByText(/开始/));
    expect(onCreate).toHaveBeenCalled();
    expect(onCreate.mock.calls[0][0].prompt).toBe("讨论社交媒体");
  });
});
```

**Note:** the second test's selectors are illustrative — the implementer should make the component's real labels/placeholders match (a `新建论文` toggle button, a title input, a prompt textarea with a placeholder containing 贴/题目/任务, and a submit button labeled e.g. `开始`), and tighten the test selectors to those exact strings. Keep the assertions (onOpen called with id; onCreate called with `{prompt}`).

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run src/studio/Directory.test.tsx`
Expected: FAIL — cannot resolve `./Directory`.

- [ ] **Step 3: Write the Directory component**

Create `apps/web/src/studio/Directory.tsx` — binding dc.html `:669-708` (写作工作室 / 项目工作室 tabs with the latter disabled "即将上线"; 你的论文 section with subtitle "从贴题目开始，AI 陪你一站站把论证走扎实。"; a 新建论文 button that reveals a title input + a "贴上任务要求" textarea + a submit; a list of the student's projects each opening on click). Presentational only — all data/effects live in StudioContainer. Inline styles matching the sibling components' palette (`#2A3B7A` primary, `#EAECF2` borders, `#1C2333` text). Disable submit while `creating` or the prompt is empty.

- [ ] **Step 4: Run to verify the component test passes**

Run: `cd apps/web && npx vitest run src/studio/Directory.test.tsx`
Expected: PASS.

- [ ] **Step 5: Wire StudioContainer directory-first**

In `apps/web/src/studio/StudioContainer.tsx`: replace the auto-open-`list[0]` behavior. Add `const [openId, setOpenId] = useState<string | null>(null)` and `const [creating, setCreating] = useState(false)`. On mount, fetch `listProjects()` into state; do NOT auto-open. Render:
- when `openId == null` → `<Directory projects={projects} onOpen={setOpenId} onCreate={handleCreate} creating={creating} />` (this replaces the current `empty` placeholder block at `:277-286`);
- when `openId != null` → the existing studio render, but keyed on `openId` (load the projection for `openId` in the projection-loading effect instead of `list[0]`), plus a "← 返回" affordance that calls `setOpenId(null)`.
`handleCreate = async (body) => { setCreating(true); try { const { id } = await api.createProject(body); const list = await api.listProjects(); setProjects(list); setOpenId(id); } finally { setCreating(false); } }`.
Keep the existing projection-load / callbacks logic intact — only change what selects the active project (from `list[0]` to `openId`) and what renders when nothing is open.

- [ ] **Step 6: Run the full web suite + typecheck**

Run: `cd apps/web && npx vitest run && npx tsc --noEmit`
Expected: full suite green (prior count + new tests); `tsc` no new errors. Fix any StudioContainer test fixtures that assumed auto-open (update them to open a project first, mirroring the new flow).

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/studio/Directory.tsx apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/Directory.test.tsx
git commit -m "feat(n1): directory + create flow (新建论文 → paste prompt → open project)"
```

---

### Task 8: OnboardingView — submit + hydrate

**Files:**
- Modify: `apps/web/src/studio/views/OnboardingView.tsx`
- Test: `apps/web/src/studio/views/OnboardingView.test.tsx` (create if absent)

**Interfaces:**
- Consumes: the projection's onboarding fields (`assignmentText`, `restatePrompt`, `rubricRows`, `studentRestate`, `studentWeakPicks`), the `submitOnboarding` client (via `api` or a callback prop).

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/studio/views/OnboardingView.test.tsx`. Render OnboardingView with onboarding data that includes a prior `studentRestate` and `assignmentText`; assert the assignment shows, the restate textarea is prefilled, and submitting (after ≥15 chars) calls the submit path. Match the component's real prop shape (inspect `OnboardingView`'s props — it currently takes `data`/`state.views.onboarding`; thread a `projectId` + `onSubmit` or use `api.submitOnboarding` directly and spy on it). Keep assertions: (a) `assignmentText` rendered; (b) textarea initial value = `studentRestate`; (c) submit disabled under 15 runes, enabled at/over; (d) submit calls `submitOnboarding` with the restate + picks.

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run src/studio/views/OnboardingView.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Wire submit + hydrate**

In `OnboardingView.tsx`: (a) render `data.assignmentText` (the pasted task) above the restate box when non-empty; (b) initialize the restate `useState` from `data.studentRestate` and the weak-pick selection from `data.studentWeakPicks` (instead of empty/local-only); (c) add a submit control ("记下我的理解") enabled when the restate is ≥15 runes (`[...restate].length >= 15`), which calls `api.submitOnboarding(projectId, { restate, weakPicks })` and then triggers a projection refresh (reuse the container's refresh path — pass an `onSubmitted` callback or call the existing reload). Keep the rubric-row toggle UX; it now feeds `weakPicks` (≤2). Do not touch the S1/S2 stub branches.

- [ ] **Step 4: Run test + full suite + typecheck**

Run: `cd apps/web && npx vitest run src/studio/views/OnboardingView.test.tsx && npx vitest run && npx tsc --noEmit`
Expected: target PASS; full web suite green; `tsc` no new errors.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/views/OnboardingView.tsx apps/web/src/studio/views/OnboardingView.test.tsx
git commit -m "feat(n1): OnboardingView persists restate + weak-picks, hydrates from projection"
```

---

## Notes for the whole-branch review

- **No migration / no `llm_call`**: confirm N1 added neither a migration nor any model call; the create + onboarding endpoints are pure DB writes.
- **Atomicity**: the create handler's project + 3 nodes must be one tx — a mid-way node failure must leave no orphan project.
- **Cross-layer parity**: `OnboardingDTO` (Go) ⟷ `OnboardingFx` (Zod) ⟷ OnboardingView reads — the 3 new fields must match names/types end-to-end.
- **Owner isolation**: create = session user; onboarding-submit via `loadOwnedProject`.
- **Funnel reachability**: a fresh student (no seed project) can create → land at S0 with live fixture data → submit → reach the write loop. The `StudioContainer` restructure must not break deep-linking into an existing project.
- **Codebase idioms (pinned in the plan, verify they held)**: `httpx.WriteError`+`ErrBadRequest`/`ErrNotEntitled` (not a `WriteErrorCode`), the `event` table (singular), `ProjectData.Nodes = []sqlc.GraphNode` with `Body []byte`. The still-open verify-notes the implementers must resolve are UI-only: `dto_parity_test.go`'s assertion style (Task 4) and the exact `OnboardingView` prop shape (Task 8).
- **Deferred (N6, do NOT fix here)**: live gate counts, event-model normalization, `GetCardInstance` scoping, anchor labels, preview-snapshot drift, terminal idempotency, `finishProject` lock, report re-POST-on-422, Enter-to-send, dc.html copy, finish-status badge in the directory.

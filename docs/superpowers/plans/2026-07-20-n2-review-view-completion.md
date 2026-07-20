# N2 · 评估 View Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the student-writable 评估 view — self-score card + S0↔S6 prediction reveal + reflection retro editor — on top of the shipped 就绪度 gauge, all driven by the single `review_criteria` taxonomy.

**Architecture:** Migration-free. Rewrite N1's 0457 onboarding fixture rows to plain-language `review_criteria` (so S0 `weak_picks` index the tested criteria). Two new student-write endpoints (`self_score` + `reflection` graph nodes, mirroring N1's `onboarding_submit`) + three read projections (`projectSelfScore`/`projectPrediction`/`projectReflection`) → new `StudioProjection` DTOs → Zod mirrors → three new `ReviewView` cards fed from `state` with write callbacks threaded on the existing `review` prop.

**Tech Stack:** Go (`net/http`, sqlc, pgx), PostgreSQL, React + Vite + TS, Zod, vitest + @testing-library/react, testcontainers-go.

## Global Constraints

- **No model call anywhere; no migration.** Config/fixture-driven DB writes only; zero `llm_call` rows. New node types `self_score`/`reflection` ride `graph_node`'s open `type` (migration 0017).
- **One taxonomy — `review_criteria`** (`sk.ReviewCriteria`, `writing-project.json:8–13`): `表D 来源与证据(4)`, `表E 分析(4)`, `表F 评估(3)`, `表H 表达与组织(3)`, in config order. Gauge, S0 prediction, and self-score all key off it; `weak_picks[i] ↔ review_criteria[i] ↔ gauge[i]`.
- **RL-3**: gauge + prediction reveal are never a predicted grade; `overlap` is a descriptive count. **RL-4**: self-score + retro are STUDENT writes — the platform never authors reflective text. **RL-5**: no aggregate score.
- **Owner isolation + entitlement**: both write endpoints call `a.loadOwnedProject(w, r)` FIRST (404 hides others'), THEN `HasEntitlement` (403 `ErrNotEntitled`). httpx idiom: `httpx.WriteError` + `httpx.ErrBadRequest(code,msg,nil)` — NO `WriteErrorCode`.
- **过程即数据**: self-score + reflection each persist as a node AND a best-effort event (`slog.Warn` on event failure, not a request failure — mirrors `onboarding_submit.go`).
- **Single-source constants**: bands `["还需努力","基本达到","稳了"]` and the retro prompts live as Go constants surfaced via the projection; the web never re-encodes them.
- **Shared-schema discipline** (N1 lesson): adding required `StudioProjection` fields breaks EVERY StudioProjection fixture across contracts + web — update all of them and run the FULL contracts + web suites, not the narrow test.
- Go tests: `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`; FULL packages for the gate. Web from `apps/web`, contracts from `packages/contracts` (tests in `test/`). Inline SVG only.
- **NEVER `git add` a bare dir or the pre-existing untracked user files** (`M package.json` + untracked `docs/` and repo-root files). Add only each task's exact files.

---

### Task 1: Rewrite the 0457 fixture rows to `review_criteria`

**Files:**
- Modify: `apps/api/internal/onboarding/fixtures/0457.json` (the `rows` array only)
- Test: `apps/api/internal/onboarding/onboarding_test.go` (extend the existing test)

**Interfaces:**
- Produces: the 0457 onboarding fixture's 4 `rows` are now plain-language `review_criteria` in `review_criteria` order, so `weak_picks` (0–3) index the tested criteria. No signature change to `onboarding.Load`.

- [ ] **Step 1: Update the failing test**

In `apps/api/internal/onboarding/onboarding_test.go`, extend `TestLoad0457` to assert the rows now match the review criteria order — add after the existing row assertions:

```go
	wantOfficial := []string{"来源与证据（表D）", "分析（表E）", "评估（表F）", "表达与组织（表H）"}
	for i, w := range wantOfficial {
		if f.Rows[i].Official != w {
			t.Errorf("row %d official = %q, want %q (review_criteria order)", i, f.Rows[i].Official, w)
		}
	}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/onboarding/`
Expected: FAIL — rows still carry the old English criteria names.

- [ ] **Step 3: Rewrite the fixture rows**

In `apps/api/internal/onboarding/fixtures/0457.json`, replace the `rows` array (keep `restate_prompt` and `steps`):

```json
  "rows": [
    { "official": "来源与证据（表D）", "plain": "用可信来源，并说清它可不可信", "weak": true },
    { "official": "分析（表E）", "plain": "能从不同视角分析，不只罗列观点", "weak": true },
    { "official": "评估（表F）", "plain": "权衡取舍、指出局限，而不是各打五十大板", "weak": false },
    { "official": "表达与组织（表H）", "plain": "结构清楚、表达清晰", "weak": false }
  ],
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/onboarding/`
Expected: PASS (still 4 rows, non-empty, generic prompt — N1's other assertions hold).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/onboarding/fixtures/0457.json apps/api/internal/onboarding/onboarding_test.go
git commit -m "feat(n2): unify 0457 onboarding rows onto review_criteria"
```

---

### Task 2: `POST /projects/{id}/self-score` endpoint

**Files:**
- Create: `apps/api/internal/api/self_score.go`
- Modify: `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/self_score_test.go`

**Interfaces:**
- Consumes: `a.loadOwnedProject`, `HasEntitlement`, `a.d.Queries.InsertGraphNode` (`sqlc.InsertGraphNodeParams{ProjectID uuid.UUID, Type, Body, Author string→[]byte...}`), `agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool).AppendEvent(agent.EventRow{...})`, `skills.ByID("writing-project")` for the valid criterion codes.
- Produces: `POST /api/v1/projects/{id}/self-score` — persists a `self_score`/`student` node body `{"scores":[{"code","band"}]}` + a `self_scored` event; returns 200.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/api/self_score_test.go`. Reuse the `createProjectForTest` helper (already in `onboarding_submit_test.go`, same package):

```go
package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func TestSubmitSelfScore_PersistsNodeAndEvent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/self-score",
		strings.NewReader(`{"scores":[{"code":"表D","band":2},{"code":"表E","band":0}]}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("self-score = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var nodes, events int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='self_score' AND author='student'`, pid).Scan(&nodes)
	if nodes != 1 {
		t.Errorf("self_score nodes = %d, want 1", nodes)
	}
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE project_id=$1 AND type='self_scored'`, pid).Scan(&events)
	if events != 1 {
		t.Errorf("self_scored events = %d, want 1", events)
	}
	// body content — catches a code/band field drift
	var body []byte
	_ = pool.QueryRow(context.Background(),
		`SELECT body FROM graph_node WHERE project_id=$1 AND type='self_score'`, pid).Scan(&body)
	if !strings.Contains(string(body), `"表D"`) || !strings.Contains(string(body), `"band":2`) {
		t.Errorf("self_score body missing scores: %s", body)
	}
}

func TestSubmitSelfScore_RejectsBadBandOrCode(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	for _, bad := range []string{`{"scores":[{"code":"表D","band":9}]}`, `{"scores":[{"code":"表Z","band":1}]}`} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/self-score", strings.NewReader(bad)), cookie))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s = %d, want 400", bad, rec.Code)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestSubmitSelfScore`
Expected: FAIL — route 404.

- [ ] **Step 3: Write the handler**

Create `apps/api/internal/api/self_score.go`:

```go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

// submitSelfScore persists the student's per-criterion self-assessment (RL-4:
// the student's own judgement, not the system grading them; RL-5: never an
// aggregate score). One band (0..2) per review criterion. No model call.
func (a *API) submitSelfScore(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r) // 404 hides other users'
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
	var req struct {
		Scores []struct {
			Code string `json:"code"`
			Band int    `json:"band"`
		} `json:"scores"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	valid := map[string]bool{}
	if sk, ok := skills.ByID("writing-project"); ok {
		for _, c := range sk.ReviewCriteria {
			valid[c.Code] = true
		}
	}
	for _, s := range req.Scores {
		if !valid[s.Code] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_code", "未知的评分维度", nil))
			return
		}
		if s.Band < 0 || s.Band > 2 {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_band", "档位超出范围", nil))
			return
		}
	}
	nodeBody, _ := json.Marshal(map[string]any{"scores": req.Scores})
	if _, err := a.d.Queries.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "self_score", Body: nodeBody, Author: "student", SpanRef: nil,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "self_scored", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("self-score: append event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
}
```

Confirm `skills.ByID` returns `(skills.Skill, bool)` (grep `func ByID` in `apps/api/internal/skills/`) — if the signature differs (e.g. returns only `Skill`, or takes no ok), match the real one; other handlers already resolve `skills.ByID("writing-project")`, so copy their call form.

- [ ] **Step 4: Register the route**

In `apps/api/internal/api/api.go`, with the `POST /api/v1/projects/{id}/...` routes:

```go
	mux.Handle("POST /api/v1/projects/{id}/self-score", protected(a.submitSelfScore))
```

- [ ] **Step 5: Run to verify pass + full package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestSubmitSelfScore` then full `go test -p 1 ./internal/api/`.
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/self_score.go apps/api/internal/api/api.go apps/api/internal/api/self_score_test.go
git commit -m "feat(n2): POST /projects/{id}/self-score — persist per-criterion self-assessment"
```

---

### Task 3: `POST /projects/{id}/reflection` endpoint

**Files:**
- Create: `apps/api/internal/api/reflection.go`
- Modify: `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/reflection_test.go`

**Interfaces:**
- Produces: `POST /api/v1/projects/{id}/reflection` — persists a `reflection`/`student` node body `{"text"}` (the node the S6 `reflect_archive` gate names) + a `reflection_written` event; returns 200. Rejects `text` < 20 runes.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/api/reflection_test.go`:

```go
package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func TestSubmitReflection_PersistsNodeAndEvent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/reflection",
		strings.NewReader(`{"text":"我一开始以为证据够了，被追问后才发现来源单一，于是补了两个反方来源。"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("reflection = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var nodes, events int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM graph_node WHERE project_id=$1 AND type='reflection' AND author='student'`, pid).Scan(&nodes)
	if nodes != 1 {
		t.Errorf("reflection nodes = %d, want 1", nodes)
	}
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM event WHERE project_id=$1 AND type='reflection_written'`, pid).Scan(&events)
	if events != 1 {
		t.Errorf("reflection_written events = %d, want 1", events)
	}
}

func TestSubmitReflection_RejectsTooShort(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/reflection", strings.NewReader(`{"text":"太短了"}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short reflection = %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestSubmitReflection`
Expected: FAIL — route 404.

- [ ] **Step 3: Write the handler**

Create `apps/api/internal/api/reflection.go`:

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

// submitReflection persists the student's S6 研究回顾 (RL-4: written by the
// student, the platform never authors reflective text). This is the `reflection`
// node the reflect_archive gate names. No model call.
func (a *API) submitReflection(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
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
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	if utf8.RuneCountInString(req.Text) < 20 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("reflection_too_short", "多写一点你的真实回顾（至少 20 字）。", nil))
		return
	}
	nodeBody, _ := json.Marshal(map[string]any{"text": req.Text})
	if _, err := a.d.Queries.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "reflection", Body: nodeBody, Author: "student", SpanRef: nil,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	payload, _ := json.Marshal(map[string]string{"text": req.Text})
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "reflection_written", Payload: payload,
	}); err != nil {
		slog.Warn("reflection: append event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
}
```

- [ ] **Step 4: Register the route**

```go
	mux.Handle("POST /api/v1/projects/{id}/reflection", protected(a.submitReflection))
```

- [ ] **Step 5: Run to verify pass + full package**

Run: `-run TestSubmitReflection` then the full `./internal/api/` package.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/reflection.go apps/api/internal/api/api.go apps/api/internal/api/reflection_test.go
git commit -m "feat(n2): POST /projects/{id}/reflection — persist student retro (S6 gate node)"
```

---

### Task 4: self-score + reflection projections

**Files:**
- Modify: `apps/api/internal/studio/projection.go` (2 new projections + `Project()` wiring)
- Modify: `apps/api/internal/studio/dto.go` (2 new DTOs + `StudioProjection` fields)
- Modify: `apps/api/internal/studio/dto_parity_test.go` (if it asserts top-level `StudioProjection` keys, add the new ones)
- Test: `apps/api/internal/studio/review_projection_test.go` (create)

**Interfaces:**
- Consumes: `d.Nodes` (`[]sqlc.GraphNode`, `Body []byte`), `sk.ReviewCriteria` (`[]skills.ReviewCriterion{Code,Name string; Points int}`).
- Produces: `SelfScoreDTO{ Dims []SelfScoreDimDTO{Code,Name string; Band int}; Bands []string }` (band -1 = unpicked); `ReflectionDTO{ Text string; Prompts []string }`. Added to `StudioProjection` as `SelfScore`/`Reflection`. Consumed by Task 6 (Zod) + Task 8 (view).

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/studio/review_projection_test.go`:

```go
package studio

import (
	"testing"

	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

func testSkill(t *testing.T) skills.Skill {
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill missing")
	}
	return sk
}

func TestProjectSelfScore_LatestWinsAndUnpicked(t *testing.T) {
	sk := testSkill(t)
	d := ProjectData{Nodes: []sqlc.GraphNode{
		{Type: "self_score", Body: []byte(`{"scores":[{"code":"表D","band":0}]}`)},
		{Type: "self_score", Body: []byte(`{"scores":[{"code":"表D","band":2},{"code":"表E","band":1}]}`)},
	}}
	ss := projectSelfScore(sk, d)
	if len(ss.Dims) != len(sk.ReviewCriteria) {
		t.Fatalf("dims = %d, want %d", len(ss.Dims), len(sk.ReviewCriteria))
	}
	byCode := map[string]int{}
	for _, dim := range ss.Dims {
		byCode[dim.Code] = dim.Band
	}
	if byCode["表D"] != 2 { // latest node wins
		t.Errorf("表D band = %d, want 2", byCode["表D"])
	}
	if byCode["表F"] != -1 { // never picked
		t.Errorf("表F band = %d, want -1 (unpicked)", byCode["表F"])
	}
	if len(ss.Bands) != 3 {
		t.Errorf("bands = %v, want 3 labels", ss.Bands)
	}
}

func TestProjectReflection_LatestText(t *testing.T) {
	d := ProjectData{Nodes: []sqlc.GraphNode{
		{Type: "reflection", Body: []byte(`{"text":"第一版反思"}`)},
		{Type: "reflection", Body: []byte(`{"text":"最终反思"}`)},
	}}
	rf := projectReflection(d)
	if rf.Text != "最终反思" {
		t.Errorf("text = %q, want 最终反思 (latest)", rf.Text)
	}
	if len(rf.Prompts) == 0 {
		t.Error("prompts empty")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/ -run 'TestProjectSelfScore|TestProjectReflection'`
Expected: FAIL — functions undefined.

- [ ] **Step 3: Add the DTOs**

In `apps/api/internal/studio/dto.go`, add (near `GaugeDTO`):

```go
// SelfScoreDimDTO is one review criterion the student rates against the mark
// scheme. Band is 0..2 (还需努力/基本达到/稳了), or -1 when not yet picked.
// RL-5: the student's own per-criterion judgement, never an aggregate grade.
type SelfScoreDimDTO struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Band int    `json:"band"`
}

type SelfScoreDTO struct {
	Dims  []SelfScoreDimDTO `json:"dims"`
	Bands []string          `json:"bands"`
}

// ReflectionDTO is the S6 研究回顾: the student's own text (RL-4, AI never
// authors it) + the prompt chips the platform offers.
type ReflectionDTO struct {
	Text    string   `json:"text"`
	Prompts []string `json:"prompts"`
}
```

Add to the `StudioProjection` struct (dto.go), after `Readiness`:

```go
	SelfScore  SelfScoreDTO  `json:"selfScore"`
	Reflection ReflectionDTO `json:"reflection"`
```

- [ ] **Step 4: Add the projections + constants + wire `Project()`**

In `apps/api/internal/studio/projection.go`, add the constants + two functions:

```go
// selfScoreBands are the 3 universal self-assessment levels (band index 0..2).
var selfScoreBands = []string{"还需努力", "基本达到", "稳了"}

// retroPrompts nudge causal reflection (RL-4: prompts only, never prose).
var retroPrompts = []string{
	"哪一步真正改变了你的判断？为什么？",
	"如果重来一次，你会在哪一步做得不同？",
	"有没有一个证据或反例，让你不得不修改原来的想法？",
}

// projectSelfScore projects the 先自己评一评 card: one dim per review criterion
// with the student's latest picked band (or -1 unpicked). Bands are universal.
func projectSelfScore(sk skills.Skill, d ProjectData) SelfScoreDTO {
	picked := map[string]int{}
	for _, n := range d.Nodes {
		if n.Type != "self_score" {
			continue
		}
		var body struct {
			Scores []struct {
				Code string `json:"code"`
				Band int    `json:"band"`
			} `json:"scores"`
		}
		if json.Unmarshal(n.Body, &body) != nil {
			continue
		}
		for _, s := range body.Scores { // last self_score node wins
			picked[s.Code] = s.Band
		}
	}
	dims := make([]SelfScoreDimDTO, 0, len(sk.ReviewCriteria))
	for _, c := range sk.ReviewCriteria {
		band := -1
		if b, ok := picked[c.Code]; ok {
			band = b
		}
		dims = append(dims, SelfScoreDimDTO{Code: c.Code, Name: c.Name, Band: band})
	}
	return SelfScoreDTO{Dims: dims, Bands: selfScoreBands}
}

// projectReflection projects the 写一段研究回顾 card: the latest student
// reflection text + the prompt chips.
func projectReflection(d ProjectData) ReflectionDTO {
	text := ""
	for _, n := range d.Nodes {
		if n.Type != "reflection" {
			continue
		}
		var body struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(n.Body, &body) == nil {
			text = body.Text // latest wins (nodes ordered by created_at)
		}
	}
	return ReflectionDTO{Text: text, Prompts: retroPrompts}
}
```

In `Project()` (projection.go), add to the returned `StudioProjection{...}` literal after `Readiness: projectReadiness(sk, d),`:

```go
		SelfScore:  projectSelfScore(sk, d),
		Reflection: projectReflection(d),
```

- [ ] **Step 5: Run studio package + fix dto_parity**

If `dto_parity_test.go` asserts the top-level `StudioProjection` key set, add `selfScore`/`reflection` (and, after Task 5, `prediction`). Run the FULL studio package:
Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/studio/projection.go apps/api/internal/studio/dto.go apps/api/internal/studio/dto_parity_test.go apps/api/internal/studio/review_projection_test.go
git commit -m "feat(n2): projectSelfScore + projectReflection projections"
```

---

### Task 5: prediction reveal projection

**Files:**
- Modify: `apps/api/internal/studio/projection.go` (`projectPrediction` + `Project()` wiring)
- Modify: `apps/api/internal/studio/dto.go` (`PredictionDTO` + field)
- Modify: `apps/api/internal/studio/dto_parity_test.go` (add `prediction` if it asserts keys)
- Test: `apps/api/internal/studio/prediction_projection_test.go` (create)

**Interfaces:**
- Consumes: `projectOnboarding(d).StudentWeakPicks []int` (the S0 prediction, N1), `projectReadiness(sk, d) []GaugeDTO`, `sk.ReviewCriteria`.
- Produces: `PredictionDTO{ Predicted []PredCritDTO{Code,Name}; Actual []PredCritDTO; Overlap int; Revealed bool }`. `Predicted` = weak-pick criteria; `Actual` = non-full gauges (only when revealed); `Overlap` = count in both; `Revealed` = a board review has informed the gauge. Added to `StudioProjection` as `Prediction`.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/studio/prediction_projection_test.go`:

```go
package studio

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestProjectPrediction_PreReviewNotRevealed(t *testing.T) {
	sk := testSkill(t)
	// S0 predicted 表D + 表E weakest (indices 0,1); no review yet.
	d := ProjectData{Nodes: []sqlc.GraphNode{
		{Type: "task_restatement", Body: []byte(`{"restate":"...","weak_picks":[0,1]}`)},
	}}
	p := projectPrediction(sk, d)
	if len(p.Predicted) != 2 || p.Predicted[0].Code != "表D" || p.Predicted[1].Code != "表E" {
		t.Fatalf("predicted = %+v, want 表D+表E", p.Predicted)
	}
	if p.Revealed {
		t.Error("revealed = true pre-review, want false")
	}
	if len(p.Actual) != 0 {
		t.Errorf("actual = %+v, want [] pre-review", p.Actual)
	}
}
```

(A revealed-state test needs a seeded snapshot + board `review_item` intervention — the api-package endpoint test covers the live-review path; here the projection is unit-tested for the pre-review branch. Add a revealed-branch unit test if `projectReadiness`'s inputs can be built cheaply from `ProjectData` in the studio package — check how `projectReadiness` reads `d.Interventions`/`d.LatestSnapshot` and construct one if straightforward; otherwise note the revealed branch is covered by an api-level test.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/ -run TestProjectPrediction`
Expected: FAIL — `projectPrediction` undefined.

- [ ] **Step 3: Add the DTO**

In `dto.go`:

```go
// PredCritDTO is one review criterion by code+name (used in the prediction reveal).
type PredCritDTO struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// PredictionDTO is the S0↔S6 reveal: the criteria the student predicted weakest
// at S0 vs. the criteria the board review actually found weak. Overlap is a
// descriptive self-knowledge count, NEVER a score (RL-3/RL-5). Actual/Revealed
// are empty/false until a whole-draft review has informed the gauge.
type PredictionDTO struct {
	Predicted []PredCritDTO `json:"predicted"`
	Actual    []PredCritDTO `json:"actual"`
	Overlap   int           `json:"overlap"`
	Revealed  bool          `json:"revealed"`
}
```

Add to `StudioProjection` (after `Reflection`):

```go
	Prediction PredictionDTO `json:"prediction"`
```

- [ ] **Step 4: Add the projection + wire `Project()`**

In `projection.go`:

```go
// projectPrediction pairs the student's S0 predicted-weakest criteria
// (task_restatement.weak_picks, indices into review_criteria) with the criteria
// the board review actually found non-full. Read-only; no new capture.
func projectPrediction(sk skills.Skill, d ProjectData) PredictionDTO {
	out := PredictionDTO{Predicted: []PredCritDTO{}, Actual: []PredCritDTO{}}
	// Predicted: weak_picks → review_criteria[i].
	predictedCodes := map[string]bool{}
	for _, i := range projectOnboarding(d).StudentWeakPicks {
		if i >= 0 && i < len(sk.ReviewCriteria) {
			c := sk.ReviewCriteria[i]
			out.Predicted = append(out.Predicted, PredCritDTO{Code: c.Code, Name: c.Name})
			predictedCodes[c.Code] = true
		}
	}
	// Actual: non-full gauges — only meaningful once a review has informed them.
	gauges := projectReadiness(sk, d)
	revealed := false
	for _, g := range gauges {
		if g.Lit > 0 || g.Note != "" {
			revealed = true
			break
		}
	}
	out.Revealed = revealed
	if revealed {
		for _, g := range gauges {
			if g.Level != "full" {
				out.Actual = append(out.Actual, PredCritDTO{Code: g.Code, Name: g.Name})
				if predictedCodes[g.Code] {
					out.Overlap++
				}
			}
		}
	}
	return out
}
```

In `Project()`, after `Reflection: projectReflection(d),`:

```go
		Prediction: projectPrediction(sk, d),
```

- [ ] **Step 5: Run studio package**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/studio/projection.go apps/api/internal/studio/dto.go apps/api/internal/studio/dto_parity_test.go apps/api/internal/studio/prediction_projection_test.go
git commit -m "feat(n2): projectPrediction — S0↔S6 predicted-vs-actual reveal"
```

---

### Task 6: contracts — Fx schemas + submit bodies + fixture updates

**Files:**
- Modify: `packages/contracts/src/studioState.ts` (3 Fx schemas + `StudioProjection` fields + 2 submit bodies)
- Modify: every StudioProjection fixture that omits the 3 new fields (contracts `test/studioState.test.ts`; web `apps/web/src/studio/fixtures.ts`, `apps/web/src/studio/StudioContainer.test.tsx`, `apps/web/src/api/projects.test.ts`)
- Test: `packages/contracts/test/n2Contracts.test.ts` (create)

**Interfaces:**
- Produces: `SelfScoreFx`/`PredictionFx`/`ReflectionFx` Zod + `StudioProjection` gains `selfScore`/`prediction`/`reflection`; `SelfScoreSubmitBody = { scores: [{code, band:int}] }`, `ReflectionSubmitBody = { text }`. Consumed by Tasks 7–8.

- [ ] **Step 1: Write the failing test**

Create `packages/contracts/test/n2Contracts.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { SelfScoreFx, PredictionFx, ReflectionFx, SelfScoreSubmitBody } from "../src/studioState";

describe("N2 contracts", () => {
  it("SelfScoreFx parses dims + bands", () => {
    const s = SelfScoreFx.parse({ dims: [{ code: "表D", name: "来源与证据", band: 2 }, { code: "表E", name: "分析", band: -1 }], bands: ["还需努力", "基本达到", "稳了"] });
    expect(s.dims[1]!.band).toBe(-1);
  });
  it("PredictionFx parses", () => {
    const p = PredictionFx.parse({ predicted: [{ code: "表D", name: "来源与证据" }], actual: [], overlap: 0, revealed: false });
    expect(p.revealed).toBe(false);
  });
  it("ReflectionFx parses", () => {
    expect(ReflectionFx.parse({ text: "回顾", prompts: ["为什么"] }).prompts.length).toBe(1);
  });
  it("SelfScoreSubmitBody validates band type", () => {
    expect(SelfScoreSubmitBody.parse({ scores: [{ code: "表D", band: 1 }] }).scores[0]!.band).toBe(1);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd packages/contracts && npx vitest run test/n2Contracts.test.ts`
Expected: FAIL — schemas missing.

- [ ] **Step 3: Add the schemas**

In `packages/contracts/src/studioState.ts`, add (near the other `*Fx` schemas):

```ts
export const SelfScoreFx = z.object({
  dims: z.array(z.object({ code: z.string(), name: z.string(), band: z.number().int() })),
  bands: z.array(z.string()),
});
export type SelfScoreFx = z.infer<typeof SelfScoreFx>;

export const PredictionFx = z.object({
  predicted: z.array(z.object({ code: z.string(), name: z.string() })),
  actual: z.array(z.object({ code: z.string(), name: z.string() })),
  overlap: z.number().int(),
  revealed: z.boolean(),
});
export type PredictionFx = z.infer<typeof PredictionFx>;

export const ReflectionFx = z.object({ text: z.string(), prompts: z.array(z.string()) });
export type ReflectionFx = z.infer<typeof ReflectionFx>;

export const SelfScoreSubmitBody = z.object({
  scores: z.array(z.object({ code: z.string(), band: z.number().int() })),
});
export type SelfScoreSubmitBody = z.infer<typeof SelfScoreSubmitBody>;

export const ReflectionSubmitBody = z.object({ text: z.string() });
export type ReflectionSubmitBody = z.infer<typeof ReflectionSubmitBody>;
```

Add the 3 fields to the `StudioProjection` object (after `readiness`):

```ts
  selfScore: SelfScoreFx,
  prediction: PredictionFx,
  reflection: ReflectionFx,
```

- [ ] **Step 4: Update every StudioProjection fixture**

Grep for fixtures embedding a full StudioProjection and add the 3 fields. In `packages/contracts/test/studioState.test.ts`, `apps/web/src/studio/fixtures.ts`, `apps/web/src/studio/StudioContainer.test.tsx`, `apps/web/src/api/projects.test.ts` — for each object with a `readiness:` key, add:

```ts
  selfScore: { dims: [], bands: ["还需努力", "基本达到", "稳了"] },
  prediction: { predicted: [], actual: [], overlap: 0, revealed: false },
  reflection: { text: "", prompts: [] },
```

(Run `grep -rn "readiness:" packages/contracts/test apps/web/src` to find them ALL — this is the shared-schema wave; missing one fails the full suite.)

- [ ] **Step 5: Run the FULL contracts suite**

Run: `cd packages/contracts && npx vitest run`
Expected: all green (the new test + every StudioProjection fixture updated).

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/studioState.ts packages/contracts/test/n2Contracts.test.ts packages/contracts/test/studioState.test.ts
git commit -m "feat(n2): contracts — selfScore/prediction/reflection Fx + submit bodies"
```

(The web fixture files touched in Step 4 are committed in Task 8 with the web changes, OR commit them here if the contracts change alone would red the web build — prefer committing the web fixtures alongside their package in Task 7/8. If committing here, add those paths too.)

---

### Task 7: web clients — submitSelfScore + submitReflection

**Files:**
- Modify: `apps/web/src/api/projects.ts`
- Modify: `apps/web/src/api/index.ts` (facade)
- Modify: any web StudioProjection fixtures not yet updated (`fixtures.ts`, `StudioContainer.test.tsx`, `projects.test.ts`) — if Task 6 didn't commit them
- Test: `apps/web/src/api/projects.test.ts` (extend)

**Interfaces:**
- Produces: `submitSelfScore(projectId, body: { scores: {code:string;band:number}[] }): Promise<void>`; `submitReflection(projectId, body: { text: string }): Promise<void>`. On the `ApiClient` facade.

- [ ] **Step 1: Write the failing test**

Add to `apps/web/src/api/projects.test.ts`:

```ts
import { submitSelfScore, submitReflection } from "./projects";

describe("submitSelfScore / submitReflection", () => {
  it("posts self-score", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValue(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));
    await submitSelfScore("p1", { scores: [{ code: "表D", band: 2 }] });
    const [url, init] = spy.mock.calls[0]!;
    expect(String(url)).toContain("/projects/p1/self-score");
    expect(JSON.parse(init!.body as string).scores[0].band).toBe(2);
  });
  it("posts reflection", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValue(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));
    await submitReflection("p1", { text: "我的回顾至少二十个字这样才够长可以通过校验规则" });
    expect(String(spy.mock.calls[0]![0])).toContain("/projects/p1/reflection");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run src/api/projects.test.ts`
Expected: FAIL — exports missing.

- [ ] **Step 3: Add the clients**

In `apps/web/src/api/projects.ts`:

```ts
export async function submitSelfScore(projectId: string, body: { scores: { code: string; band: number }[] }): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/self-score`, { method: "POST", body: JSON.stringify(body) });
}

export async function submitReflection(projectId: string, body: { text: string }): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/reflection`, { method: "POST", body: JSON.stringify(body) });
}
```

- [ ] **Step 4: Wire the facade**

In `apps/web/src/api/index.ts`: import both; add to the `ApiClient` interface and the `api` object (additive).

- [ ] **Step 5: Run test + typecheck + full api suite**

Run: `cd apps/web && npx vitest run src/api/ && npx tsc --noEmit`
Expected: PASS; tsc no new errors. (If web StudioProjection fixtures still lack the 3 fields, add them now so the suite is green.)

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/api/projects.ts apps/web/src/api/index.ts apps/web/src/api/projects.test.ts apps/web/src/studio/fixtures.ts apps/web/src/studio/StudioContainer.test.tsx
git commit -m "feat(n2): web submitSelfScore + submitReflection clients + facade"
```

---

### Task 8: ReviewView — self-score + prediction + retro cards

**Files:**
- Modify: `apps/web/src/studio/views/ReviewView.tsx`
- Modify: `apps/web/src/studio/state.ts` (expose the 3 new projection fields as `StudioState` view data / types)
- Modify: `apps/web/src/studio/ViewFrame.tsx`, `apps/web/src/studio/StudioShell.tsx`, `apps/web/src/studio/StudioContainer.tsx` (thread `onSelfScore`/`onReflection` on the existing `review` prop, like `onFinish`)
- Test: `apps/web/src/studio/views/ReviewView.test.tsx` (create)

**Interfaces:**
- Consumes: `SelfScoreFx`/`PredictionFx`/`ReflectionFx` (via `state`), `submitSelfScore`/`submitReflection` (via handlers).
- Produces: a 4-card 评估 view (gauge + prediction reveal + self-score + retro).

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/studio/views/ReviewView.test.tsx`. Render `ReviewView` with the existing gauge props PLUS `selfScore`/`prediction`/`reflection` data + `onSelfScore`/`onReflection` mocks. Assert: (a) the prediction reveal shows the predicted criteria names and, when `revealed:false`, a "跑完…对照" pre-review line; (b) the self-score card renders one row per dim with 3 band chips, and clicking a band calls `onSelfScore` with `{scores:[{code,band}]}`; (c) the retro textarea hydrates from `reflection.text`, the submit is disabled under 20 runes and calls `onReflection({text})` at/over. Build the props explicitly (don't rely on a shared fixture).

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run src/studio/views/ReviewView.test.tsx`
Expected: FAIL — new props/cards absent.

- [ ] **Step 3: Extend ReviewView**

Add to `ReviewViewProps`: `selfScore: SelfScoreFx`, `prediction: PredictionFx`, `reflection: ReflectionFx`, `onSelfScore: (body: { scores: { code: string; band: number }[] }) => void`, `onReflection: (body: { text: string }) => void` (import the Fx types from `../state` or `@mind-imprint/contracts`). Below the existing gauge grid, render three cards, reusing the file's existing card style idiom (white card, `#ECEEF3` border, radius 14):
1. **Prediction reveal** — "开头你预测最弱的是 {predicted names}"; if `prediction.revealed` also "跑完这轮，评分表上实际最弱的是 {actual names}" + a gentle overlap line ("你的预测和实际吻合 {overlap} 项" — descriptive, not a score); else "跑完整稿体检后，这里会对照实际". If `predicted` is empty, "你在开头还没有预测最弱项".
2. **先自己评一评** — badge "已评 {count of band≥0}/{dims.length}", subcopy "对照上面的就绪度，给自己每一块打个档". One row per `selfScore.dims`: the dim name + `selfScore.bands.map((label, band) => chip)`; the picked band (`dim.band`) is highlighted; clicking a chip calls `onSelfScore({ scores: [{ code: dim.code, band }] })`.
3. **写一段研究回顾** — subcopy "这段反思由你自己写——印记只提供问题，不代笔。" + `reflection.prompts` chips + a value-bound textarea (state seeded from `reflection.text`) + a "记下我的反思" submit enabled when `[...text].length >= 20`, calling `onReflection({ text })`.
Keep the finish button block below, unchanged.

- [ ] **Step 4: Thread the data + callbacks**

- `state.ts`: expose `selfScore`/`prediction`/`reflection` on `StudioState` (mirror how `readiness`/`review` are surfaced from the projection), re-exporting the Fx types.
- `ViewFrame.tsx`: the ReviewView render passes `gauges={state...readiness}` today; add `selfScore`/`prediction`/`reflection` from `state`, and `onSelfScore`/`onReflection` from the `review` prop object (extend `review?: { finishing; finishError; onFinish; onSelfScore; onReflection }`).
- `StudioShell.tsx`: forward the two new callbacks through its `review` prop (mirror `onFinish`).
- `StudioContainer.tsx`: define `const submitSelfScore = async (body) => { if (!projectId) return; await api.submitSelfScore(projectId, body); await refetchProject(); }` and the same for reflection; add `submitSelfScore`/`submitReflection` to the container's `api` `Pick`; pass them into `<StudioShell>`'s `review` object.

- [ ] **Step 5: Run the target test + full web suite + tsc**

Run: `cd apps/web && npx vitest run src/studio/views/ReviewView.test.tsx && npx vitest run && npx tsc --noEmit`
Expected: target PASS; full web suite green; tsc no new errors. Update any ReviewView-rendering test fixture that now needs the new props.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/studio/views/ReviewView.tsx apps/web/src/studio/views/ReviewView.test.tsx apps/web/src/studio/state.ts apps/web/src/studio/ViewFrame.tsx apps/web/src/studio/StudioShell.tsx apps/web/src/studio/StudioContainer.tsx
git commit -m "feat(n2): 评估 view — self-score + prediction reveal + retro cards"
```

---

## Notes for the whole-branch review

- **No migration / no `llm_call`**: confirm N2 added neither; the 2 endpoints are pure DB writes.
- **One taxonomy**: `weak_picks[i]`, `self_score` code, and gauge all key off `review_criteria` in the same order — verify the prediction reveal compares like-for-like (the fixture rewrite in Task 1 is what makes this true).
- **Cross-layer parity**: `SelfScoreDTO`/`PredictionDTO`/`ReflectionDTO` (Go) ⟷ `SelfScoreFx`/`PredictionFx`/`ReflectionFx` (Zod) ⟷ ReviewView reads — names/types match; band -1 sentinel preserved.
- **Shared-schema wave**: every StudioProjection fixture across contracts + web carries the 3 new fields (Task 6/7) — a scan for `readiness:` without `selfScore:` should return nothing in source (dist artifacts excluded).
- **RL held**: gauge/prediction never a grade; self-score + retro are student writes; overlap is a descriptive count.
- **S6 gate**: the `reflection` node the retro writes is the one `reflect_archive` names — confirm the type string matches (`reflection`).
- **Deferred (do NOT build here)**: AI-usage declaration (N2d), export forks (N2e), the reflection evidence-pack, `weakness_prediction` gate alignment, assessor/report aggregation of self-score/reflection.

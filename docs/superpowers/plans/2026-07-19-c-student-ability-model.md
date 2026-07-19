# C · Student-Level Ability Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Aggregate the student's per-session DualAxis reports into a current-standing 能力素养 model (deterministic projection, no LLM) and surface it in a restored tabbed 成长报告 (学习记录 + 能力素养).

**Architecture:** A pure Go projection `ability.Aggregate([]Sample) Model` merges the stored `agent.Report`s (recency-weighted depth levels with a low-N guard; autonomy as observation sums; SOLO as a distribution). A read-only `GET /growth/ability` endpoint owner-filters the student's evaluations, runs the projection, and returns it — no model call, no cost. A Zod `AbilityModel` mirrors it; a new `<AbilityModel>` web component renders it inside a tabbed 成长报告.

**Tech Stack:** Go (`net/http`, `pgx`, sqlc), React + Vite + TypeScript + Zod contracts, Postgres. Reuses B's `agent.Report` / `rubric.DepthDims()` / contracts `DualAxisReport` — no rubric or per-session-model change.

**Spec:** `docs/superpowers/specs/2026-07-19-c-student-ability-model-design.md`

## Global Constraints

- **RL-5:** no combined total across axes; no rank/grade/测验分数; every block shows its evidence/session count.
- **Axiom:** a depth dim's level requires **≥2 contributing sessions** (else level `-1` = 「证据不足 · 需更多任务」); 智识自主 never gets a level; 元认知 never gets a single level; the three blocks never combine.
- **Deterministic:** NO LLM call, NO cost — the endpoint writes zero `llm_call` rows (a test asserts this).
- **Evidence rule:** a session contributes to a depth dim only when that dim's score is **≥1**; a `0` is excluded from both the merge and the evidence count.
- **Recency weight:** contributing scores ordered oldest→newest `s_0…s_{k-1}`; weight `w_i = 0.6^(k-1-i)` (`DECAY = 0.6`); `level = round(Σ w_i·s_i / Σ w_i)`.
- **Owner-isolation:** only the session user's evaluations aggregate (mirror `ListGrowthHistory`'s per-scope owner joins).
- **Single-source reuse:** consume `agent.Report`, `rubric.DepthDims()`, contracts `DualAxisReport`; add no rubric/model.
- `make sqlc` from `apps/api` after editing `queries/*.sql`; never hand-edit `internal/store/sqlc/*`.
- Full Go packages (`cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`), never `-run` subsets, for query/endpoint/projection changes.
- Web/contracts tests from their OWN dirs. Icons inline SVG, never lucide-react.
- Direct-merge to `main` + push (no PR). NEVER `git add` a whole directory — name files. Pre-existing `M package.json` + untracked user files under `docs/`/repo root are NOT ours.
- No migration (reads existing `evaluations`; all rows are DualAxis post-0027).

## File Structure

**Create:**
- `apps/api/internal/ability/ability.go` — `Sample`, `Model` (+ sub-types), `Aggregate`.
- `apps/api/internal/ability/ability_test.go` — projection units.
- `apps/api/internal/api/ability.go` — `getAbilityModel` handler.
- `apps/api/internal/api/ability_test.go` — endpoint test.
- `packages/contracts/src/ability.ts` — `AbilityModel` Zod schema.
- `packages/contracts/test/ability.test.ts` — contract test.
- `apps/web/src/api/ability.ts` — `getAbilityModel` client.
- `apps/web/src/shell/growth/AbilityModel.tsx` — the ability component.
- `apps/web/src/shell/growth/AbilityModel.test.tsx` — component test.

**Modify:**
- `apps/api/internal/store/queries/evaluation.sql` — add `ListEvaluationsByUser` (then `make sqlc`).
- `apps/api/internal/api/api.go` — register `GET /api/v1/growth/ability`.
- `packages/contracts/src/index.ts` — `export * from "./ability"`.
- `apps/web/src/api/index.ts` — wire `getAbilityModel` into the facade.
- `apps/web/src/shell/growth/GrowthReport.tsx` — tab bar; extract history list into a 学习记录 tab; add 能力素养 tab.
- `apps/web/src/shell/growth/GrowthReport.test.tsx` — tab switching (if the file exists; else add to AbilityModel test).

---

### Task 1: `ability.Aggregate` projection (pure)

**Files:**
- Create: `apps/api/internal/ability/ability.go`
- Create: `apps/api/internal/ability/ability_test.go`

**Interfaces:**
- Consumes: `agent.Report` (fields `DepthAxis.Dims[].{Code,Score}`, `AutonomyAxis.{AdversaryInvites,AnchoredSignals,PromptedSignals}`, `PromptLens.BoundarySettings`, `Solo[].{Level,Initiative}`), `rubric.DepthDims() []rubric.AxisDim` (fields `ID,Name,Anchors map[string]string`).
- Produces: `ability.Sample{Report agent.Report; CreatedAt time.Time}`; `ability.Model` (+ `DepthAbility`,`AutonomyAbility`,`Metacognition`); `ability.Aggregate(samples []Sample) Model`.

- [ ] **Step 1: Write the failing test** `apps/api/internal/ability/ability_test.go`:

```go
package ability

import (
	"testing"
	"time"

	"mindimprint/api/internal/agent"
)

func rep(depth map[string]int, boundary, adv int, anchored, prompted []string, solo []agent.SoloRow) agent.Report {
	dims := make([]agent.DepthDimScore, 0, len(depth))
	for code, sc := range depth {
		dims = append(dims, agent.DepthDimScore{Code: code, Score: sc})
	}
	return agent.Report{
		DepthAxis:    agent.DepthAxis{Dims: dims},
		AutonomyAxis: agent.AutonomyAxis{AdversaryInvites: adv, AnchoredSignals: anchored, PromptedSignals: prompted},
		PromptLens:   agent.PromptLens{BoundarySettings: boundary},
		Solo:         solo,
	}
}

func at(day int) time.Time { return time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC) }

func TestAggregateDepthRecencyWeightedAndLowNGuard(t *testing.T) {
	// D1 contributes [1 (older), 3 (newer)] → weighted (0.6*1 + 1*3)/1.6 = 2.25 → level 2, evidence 2.
	// D3 contributes only [2] once → below the 2-session guard → level -1.
	// D4 scores 0 twice → 0 is not evidence → level -1, evidence 0.
	samples := []Sample{
		{Report: rep(map[string]int{"D1": 1, "D3": 2, "D4": 0}, 0, 0, nil, nil, nil), CreatedAt: at(1)},
		{Report: rep(map[string]int{"D1": 3, "D4": 0}, 0, 0, nil, nil, nil), CreatedAt: at(2)},
	}
	m := Aggregate(samples)
	if m.TotalSessions != 2 {
		t.Fatalf("totalSessions = %d, want 2", m.TotalSessions)
	}
	if len(m.Depth) != 4 {
		t.Fatalf("depth dims = %d, want 4 (D1/D3/D4/D5 always)", len(m.Depth))
	}
	byCode := map[string]DepthAbility{}
	for _, d := range m.Depth {
		byCode[d.Code] = d
	}
	if d := byCode["D1"]; d.Level != 2 || d.EvidenceCount != 2 {
		t.Fatalf("D1 = level %d evidence %d, want level 2 evidence 2", d.Level, d.EvidenceCount)
	}
	if byCode["D1"].LevelLabel == "" {
		t.Fatalf("D1 level label empty, want the score-2 anchor text")
	}
	if d := byCode["D3"]; d.Level != -1 || d.EvidenceCount != 1 {
		t.Fatalf("D3 = level %d evidence %d, want level -1 evidence 1 (low-N)", d.Level, d.EvidenceCount)
	}
	if d := byCode["D4"]; d.Level != -1 || d.EvidenceCount != 0 {
		t.Fatalf("D4 = level %d evidence %d, want level -1 evidence 0 (score 0 excluded)", d.Level, d.EvidenceCount)
	}
	if d := byCode["D5"]; d.Level != -1 || d.EvidenceCount != 0 {
		t.Fatalf("D5 = level %d evidence %d, want level -1 evidence 0 (never scored)", d.Level, d.EvidenceCount)
	}
}

func TestAggregateAutonomyAndMetacognition(t *testing.T) {
	samples := []Sample{
		{Report: rep(nil, 2, 0, []string{"R1"}, []string{"R3"},
			[]agent.SoloRow{{Level: "L3", Initiative: "自发"}, {Level: "L4", Initiative: "引导后"}}), CreatedAt: at(1)},
		{Report: rep(nil, 1, 0, []string{"R1", "R8"}, nil,
			[]agent.SoloRow{{Level: "L3", Initiative: "引导后"}}), CreatedAt: at(2)},
	}
	m := Aggregate(samples)
	if m.Autonomy.BoundarySettings != 3 || m.Autonomy.AdversaryInvites != 0 {
		t.Fatalf("autonomy sums = %+v, want boundary 3 adversary 0", m.Autonomy)
	}
	if m.Autonomy.AnchoredSignals != 3 || m.Autonomy.PromptedSignals != 1 {
		t.Fatalf("autonomy signals = anchored %d prompted %d, want 3/1", m.Autonomy.AnchoredSignals, m.Autonomy.PromptedSignals)
	}
	if m.Metacognition.HighestSolo != "L4" {
		t.Fatalf("highestSolo = %q, want L4", m.Metacognition.HighestSolo)
	}
	if m.Metacognition.Distribution["L3"] != 2 || m.Metacognition.Distribution["L4"] != 1 {
		t.Fatalf("distribution = %v, want L3:2 L4:1", m.Metacognition.Distribution)
	}
	if m.Metacognition.Spontaneous != 1 || m.Metacognition.Prompted != 2 {
		t.Fatalf("solo initiative = spont %d prompted %d, want 1/2", m.Metacognition.Spontaneous, m.Metacognition.Prompted)
	}
}

func TestAggregateEmpty(t *testing.T) {
	m := Aggregate(nil)
	if m.TotalSessions != 0 || len(m.Depth) != 4 {
		t.Fatalf("empty model = sessions %d depth %d, want 0 / 4", m.TotalSessions, len(m.Depth))
	}
	for _, d := range m.Depth {
		if d.Level != -1 {
			t.Fatalf("empty depth %s level %d, want -1", d.Code, d.Level)
		}
	}
	if m.Metacognition.Distribution == nil {
		t.Fatalf("distribution nil, want initialized empty map")
	}
}
```

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && go test ./internal/ability/...`
Expected: FAIL (package/functions do not exist).

- [ ] **Step 3: Implement** `apps/api/internal/ability/ability.go`:

```go
// Package ability projects a student's per-session DualAxis reports into a
// current-standing 能力素养 model. Pure — no store, no LLM (RL-5: the person-level
// view is a merge of many sessions' evidence, never a single-session 档位).
package ability

import (
	"math"
	"sort"
	"strconv"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/rubric"
)

// decay is the per-session recency weight base: the newest contributing session
// weighs 1, each older one 0.6× the next.
const decay = 0.6

type Sample struct {
	Report    agent.Report
	CreatedAt time.Time
}

// DepthAbility is one scored depth dim's merged current standing. Level -1 means
// "证据不足 · 需更多任务" (fewer than 2 contributing sessions) — the axiom made literal.
type DepthAbility struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Level         int    `json:"level"`
	LevelLabel    string `json:"levelLabel"`
	EvidenceCount int    `json:"evidenceCount"`
}

// AutonomyAbility is 智识自主 aggregated as observation counts — never a level.
type AutonomyAbility struct {
	Sessions         int `json:"sessions"`
	BoundarySettings int `json:"boundarySettings"`
	AdversaryInvites int `json:"adversaryInvites"`
	AnchoredSignals  int `json:"anchoredSignals"`
	PromptedSignals  int `json:"promptedSignals"`
}

// Metacognition is 跨轴 SOLO aggregated as a distribution — never a single level.
type Metacognition struct {
	HighestSolo  string         `json:"highestSolo"`
	Distribution map[string]int `json:"distribution"`
	Spontaneous  int            `json:"spontaneous"`
	Prompted     int            `json:"prompted"`
}

type Model struct {
	TotalSessions int             `json:"totalSessions"`
	Depth         []DepthAbility  `json:"depth"`
	Autonomy      AutonomyAbility `json:"autonomy"`
	Metacognition Metacognition   `json:"metacognition"`
}

var soloLevels = []string{"L1", "L2", "L3", "L4"}

// Aggregate merges the samples into a current-standing model. Defensive: sorts by
// CreatedAt ascending so recency weighting holds regardless of input order.
func Aggregate(samples []Sample) Model {
	sorted := make([]Sample, len(samples))
	copy(sorted, samples)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].CreatedAt.Before(sorted[j].CreatedAt) })

	m := Model{TotalSessions: len(sorted)}
	m.Metacognition.Distribution = map[string]int{"L1": 0, "L2": 0, "L3": 0, "L4": 0}

	// Depth: one merged level per depth dim, in rubric order (always length 4).
	for _, dim := range rubric.DepthDims() {
		var scores []int // contributing (>=1) in oldest->newest order
		for _, s := range sorted {
			for _, d := range s.Report.DepthAxis.Dims {
				if d.Code == dim.ID && d.Score >= 1 {
					scores = append(scores, d.Score)
				}
			}
		}
		da := DepthAbility{Code: dim.ID, Name: dim.Name, EvidenceCount: len(scores), Level: -1}
		if len(scores) >= 2 {
			k := len(scores)
			var num, den float64
			for i, sc := range scores {
				w := math.Pow(decay, float64(k-1-i))
				num += w * float64(sc)
				den += w
			}
			da.Level = int(math.Round(num / den))
			da.LevelLabel = dim.Anchors[strconv.Itoa(da.Level)]
		}
		m.Depth = append(m.Depth, da)
	}

	// Autonomy + metacognition: sum/collect across all sessions.
	for _, s := range sorted {
		m.Autonomy.Sessions++
		m.Autonomy.BoundarySettings += s.Report.PromptLens.BoundarySettings
		m.Autonomy.AdversaryInvites += s.Report.AutonomyAxis.AdversaryInvites
		m.Autonomy.AnchoredSignals += len(s.Report.AutonomyAxis.AnchoredSignals)
		m.Autonomy.PromptedSignals += len(s.Report.AutonomyAxis.PromptedSignals)
		for _, row := range s.Report.Solo {
			if _, ok := m.Metacognition.Distribution[row.Level]; ok {
				m.Metacognition.Distribution[row.Level]++
			}
			if row.Initiative == "自发" {
				m.Metacognition.Spontaneous++
			} else {
				m.Metacognition.Prompted++
			}
		}
	}
	// highest SOLO present
	for i := len(soloLevels) - 1; i >= 0; i-- {
		if m.Metacognition.Distribution[soloLevels[i]] > 0 {
			m.Metacognition.HighestSolo = soloLevels[i]
			break
		}
	}
	return m
}
```

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/api && go test ./internal/ability/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/ability/ability.go apps/api/internal/ability/ability_test.go
git commit -m "feat(c): ability.Aggregate projection (recency-weighted, low-N guard)"
```

---

### Task 2: `ListEvaluationsByUser` query

**Files:**
- Modify: `apps/api/internal/store/queries/evaluation.sql`
- Test: `apps/api/internal/store/evaluations_by_user_query_test.go` (create)

**Interfaces:**
- Produces (after `make sqlc`): `Queries.ListEvaluationsByUser(ctx, userID uuid.UUID) ([]ListEvaluationsByUserRow, error)` where each row has `Scores []byte` and `CreatedAt time.Time`.

- [ ] **Step 1: Add the query** to `apps/api/internal/store/queries/evaluation.sql` (append at end):

```sql
-- C ability model: EVERY evaluation the caller owns across all three scopes
-- (not latest-per-scope like ListGrowthHistory), oldest-first, for cross-session
-- aggregation. Owner-filtered through each scope's own join.

-- name: ListEvaluationsByUser :many
SELECT scores, created_at FROM (
  (SELECT e.scores AS scores, e.created_at AS created_at
   FROM evaluations e JOIN project p ON p.id = e.project_id
   WHERE e.project_id IS NOT NULL AND p.user_id = @user_id)
  UNION ALL
  (SELECT e.scores, e.created_at
   FROM evaluations e JOIN course_session cs ON cs.id = e.session_id
   WHERE e.session_id IS NOT NULL AND cs.user_id = @user_id)
  UNION ALL
  (SELECT e.scores, e.created_at
   FROM evaluations e JOIN chat_thread t ON t.id = e.thread_id
   WHERE e.thread_id IS NOT NULL AND t.user_id = @user_id)
) rows
ORDER BY created_at ASC;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc`
Expected: `internal/store/sqlc/evaluation.sql.go` gains `ListEvaluationsByUser` + `ListEvaluationsByUserRow`. Never hand-edit generated files.

- [ ] **Step 3: Write the query test** `apps/api/internal/store/evaluations_by_user_query_test.go`. Mirror `growth_history_query_test.go`'s harness (same test pool + seed helpers). Seed 2 project evaluations for the seeded student (different `created_at`) and 1 for a DIFFERENT user; assert the seeded student gets exactly 2 rows, oldest-first, and the other user's row is excluded:

```go
package store_test

// Mirror growth_history_query_test.go exactly for pool/seed setup (newAPITestPool
// equivalent, the seeded-student UUID, and the project/user insert helpers it uses).
// Only the assertions below are C-specific. If growth_history_query_test.go seeds a
// project via a helper, reuse it; otherwise insert school→class→user→project inline
// the way that test does, then insert evaluations rows directly.

func TestListEvaluationsByUser_OwnerFilteredAllRows(t *testing.T) {
	// seed: student S with 2 project evaluations (created_at day 1 and day 2),
	// scores = the two JSON blobs below; a different user U with 1 evaluation.
	// Assert q.ListEvaluationsByUser(S) == 2 rows, ordered created_at ASC,
	// and none of them is U's.
	// (Use scores = []byte(`{"depthAxis":{"dims":[{"code":"D1","score":3}],"subtotal":3}}`)
	//  — any valid JSON; the query does not parse it.)
}
```

Fill in the seed/asserts by copying `growth_history_query_test.go`'s exact seed calls (it already seeds owned + other-user evaluation rows for its cross-surface owner test — reuse that scaffolding; the only difference is you insert two rows for the owner and assert count 2 + ascending order).

- [ ] **Step 4: Run the full store package — expect PASS**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/queries/evaluation.sql apps/api/internal/store/sqlc/evaluation.sql.go apps/api/internal/store/sqlc/querier.go apps/api/internal/store/evaluations_by_user_query_test.go
git commit -m "feat(c): ListEvaluationsByUser query (all owner rows, oldest-first)"
```

(`git status` first — `make sqlc` may also touch `models.go`; stage exactly the generated files it changed, by name, plus the two source files.)

---

### Task 3: `GET /api/v1/growth/ability` endpoint

**Files:**
- Create: `apps/api/internal/api/ability.go`
- Modify: `apps/api/internal/api/api.go` (route)
- Create: `apps/api/internal/api/ability_test.go`

**Interfaces:**
- Consumes: `Queries.ListEvaluationsByUser` (Task 2), `ability.Aggregate`/`ability.Sample` (Task 1), `UserFromContext`, `httpx.WriteJSON`/`WriteError`, `agent.Report`.
- Produces: `(a *API) getAbilityModel(w,r)` returning the JSON of `ability.Model`; route `GET /api/v1/growth/ability`.

- [ ] **Step 1: Write the failing endpoint test** `apps/api/internal/api/ability_test.go`:

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestGetAbilityModel_EmptyStateNoModelCall — a fresh student with no evaluations
// gets a 200 empty model (length-4 depth all insufficient), and NO llm_call row is
// written (deterministic projection, no model call).
func TestGetAbilityModel_EmptyStateNoModelCall(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/growth/ability", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /growth/ability = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var m struct {
		TotalSessions int `json:"totalSessions"`
		Depth         []struct {
			Code  string `json:"code"`
			Level int    `json:"level"`
		} `json:"depth"`
		Metacognition struct {
			Distribution map[string]int `json:"distribution"`
		} `json:"metacognition"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode ability model: %v — body=%s", err, rec.Body)
	}
	if m.TotalSessions != 0 || len(m.Depth) != 4 {
		t.Fatalf("empty model = sessions %d depth %d, want 0 / 4", m.TotalSessions, len(m.Depth))
	}
	for _, d := range m.Depth {
		if d.Level != -1 {
			t.Fatalf("empty depth %s level %d, want -1", d.Code, d.Level)
		}
	}
	if m.Metacognition.Distribution == nil {
		t.Fatalf("distribution nil, want initialized map")
	}
	// deterministic: no model call happened.
	if n := countAllLLMCalls(t, pool); n != 0 {
		t.Fatalf("llm_call rows = %d, want 0 (ability is a pure projection)", n)
	}
}

// countAllLLMCalls counts every llm_call row (the ability endpoint must add none).
func countAllLLMCalls(t *testing.T, pool interface {
	QueryRow(ctx any, sql string, args ...any) any
}) int {
	t.Helper()
	return 0 // replaced below with the real pool query — see note.
}
```

> IMPLEMENTER NOTE: replace the placeholder `countAllLLMCalls` with a real count using the same `pgxpool.Pool` query style as `countLLMCalls` in `project_finish_test.go` (which counts `llm_call` rows for a project id). Here count ALL rows: `SELECT count(*) FROM llm_call`. Use the concrete `*pgxpool.Pool` type and `pool.QueryRow(context.Background(), "SELECT count(*) FROM llm_call").Scan(&n)` exactly as `countLLMCalls` does — do not invent the interface shape shown above.

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && go build ./internal/api/...`
Expected: FAIL (`getAbilityModel` route + handler missing).

- [ ] **Step 3: Implement the handler** `apps/api/internal/api/ability.go`:

```go
package api

import (
	"encoding/json"
	"net/http"

	"mindimprint/api/internal/ability"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// getAbilityModel projects the caller's whole cross-session DualAxis history into
// a current-standing 能力素养 model. Read-only, owner-filtered, NO model call, NO
// cost (RL-5: a merge of many sessions' evidence, never a single-session 档位).
func (a *API) getAbilityModel(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListEvaluationsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	samples := make([]ability.Sample, 0, len(rows))
	for _, row := range rows {
		var report agent.Report
		if err := json.Unmarshal(row.Scores, &report); err != nil {
			continue // a malformed row must not sink the aggregate
		}
		samples = append(samples, ability.Sample{Report: report, CreatedAt: row.CreatedAt})
	}
	httpx.WriteJSON(w, http.StatusOK, ability.Aggregate(samples))
}
```

- [ ] **Step 4: Register the route** in `apps/api/internal/api/api.go` — directly after the growth/history line (`api.go:56`):

```go
	mux.Handle("GET /api/v1/growth/ability", protected(a.getAbilityModel))
```

- [ ] **Step 5: Run the full API package — expect PASS**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/ability.go apps/api/internal/api/api.go apps/api/internal/api/ability_test.go
git commit -m "feat(c): GET /growth/ability endpoint (deterministic, no cost)"
```

---

### Task 4: Contracts `AbilityModel`

**Files:**
- Create: `packages/contracts/src/ability.ts`
- Create: `packages/contracts/test/ability.test.ts`
- Modify: `packages/contracts/src/index.ts`

**Interfaces:**
- Produces: `AbilityModel` Zod schema + inferred type mirroring `ability.Model` (camelCase): `totalSessions`, `depth: [{code,name,level,levelLabel,evidenceCount}]`, `autonomy: {sessions,boundarySettings,adversaryInvites,anchoredSignals,promptedSignals}`, `metacognition: {highestSolo,distribution: Record<string,number>,spontaneous,prompted}`.

- [ ] **Step 1: Write the failing test** `packages/contracts/test/ability.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { AbilityModel } from "../src/ability";

const sample = {
  totalSessions: 6,
  depth: [
    { code: "D1", name: "任务理解与问题表述", level: 2, levelLabel: "给出任务背景与目标，但约束或验收标准仍模糊", evidenceCount: 4 },
    { code: "D3", name: "证据与信源意识", level: -1, levelLabel: "", evidenceCount: 1 },
    { code: "D4", name: "论证结构意识", level: 3, levelLabel: "识别缺失 warrant", evidenceCount: 3 },
    { code: "D5", name: "反馈理解与修改理由", level: -1, levelLabel: "", evidenceCount: 0 },
  ],
  autonomy: { sessions: 6, boundarySettings: 11, adversaryInvites: 0, anchoredSignals: 18, promptedSignals: 9 },
  metacognition: { highestSolo: "L4", distribution: { L1: 0, L2: 1, L3: 7, L4: 2 }, spontaneous: 3, prompted: 7 },
};

describe("AbilityModel", () => {
  it("parses a valid ability model", () => {
    const m = AbilityModel.parse(sample);
    expect(m.depth.length).toBe(4);
    expect(m.depth[1]!.level).toBe(-1); // insufficient
    expect(m.metacognition.distribution.L3).toBe(7);
  });
  it("rejects a non-number level", () => {
    expect(() => AbilityModel.parse({ ...sample, depth: [{ ...sample.depth[0], level: "two" }] })).toThrow();
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd packages/contracts && npx vitest run test/ability.test.ts`
Expected: FAIL (module missing).

- [ ] **Step 3: Implement** `packages/contracts/src/ability.ts`:

```ts
import { z } from "zod";

// AbilityModel: the student-level 能力素养 model — a cross-session merge of the
// DualAxis reports. Mirrors apps/api/internal/ability.Model (camelCase). RL-5: the
// three blocks never combine into a total; a depth level of -1 = 证据不足 (fewer than
// 2 contributing sessions); 智识自主/元认知 carry no level.
export const AbilityDepth = z.object({
  code: z.string(),
  name: z.string(),
  level: z.number().int(),        // -1 = insufficient, else 0..3
  levelLabel: z.string(),
  evidenceCount: z.number().int().min(0),
});

export const AbilityModel = z.object({
  totalSessions: z.number().int().min(0),
  depth: z.array(AbilityDepth),
  autonomy: z.object({
    sessions: z.number().int().min(0),
    boundarySettings: z.number().int().min(0),
    adversaryInvites: z.number().int().min(0),
    anchoredSignals: z.number().int().min(0),
    promptedSignals: z.number().int().min(0),
  }),
  metacognition: z.object({
    highestSolo: z.string(),
    distribution: z.record(z.number().int()),
    spontaneous: z.number().int().min(0),
    prompted: z.number().int().min(0),
  }),
});
export type AbilityModel = z.infer<typeof AbilityModel>;
```

- [ ] **Step 4: Export from the barrel** `packages/contracts/src/index.ts` — near the other growth exports:

```ts
export * from "./ability";
```

- [ ] **Step 5: Run — expect PASS**

Run: `cd packages/contracts && npx vitest run test/ability.test.ts && npx tsc --noEmit`
Expected: PASS (tsc: only the known pre-existing `interactionPrimitive.test.ts` error, nothing new).

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/ability.ts packages/contracts/src/index.ts packages/contracts/test/ability.test.ts
git commit -m "feat(c): contracts AbilityModel schema"
```

---

### Task 5: Web api client + facade

**Files:**
- Create: `apps/web/src/api/ability.ts`
- Modify: `apps/web/src/api/index.ts`
- Test: `apps/web/src/api/ability.test.ts` (create)

**Interfaces:**
- Consumes: `AbilityModel` (Task 4), `apiFetch`.
- Produces: `getAbilityModel(): Promise<AbilityModel>`; facade `api.getAbilityModel`.

- [ ] **Step 1: Write the failing test** `apps/web/src/api/ability.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { getAbilityModel } from "./ability";

afterEach(() => { vi.restoreAllMocks(); });

describe("getAbilityModel", () => {
  it("parses the ability model", async () => {
    const body = {
      totalSessions: 2,
      depth: [
        { code: "D1", name: "任务理解与问题表述", level: 2, levelLabel: "x", evidenceCount: 2 },
        { code: "D3", name: "证据与信源意识", level: -1, levelLabel: "", evidenceCount: 1 },
        { code: "D4", name: "论证结构意识", level: -1, levelLabel: "", evidenceCount: 0 },
        { code: "D5", name: "反馈理解与修改理由", level: -1, levelLabel: "", evidenceCount: 0 },
      ],
      autonomy: { sessions: 2, boundarySettings: 3, adversaryInvites: 0, anchoredSignals: 3, promptedSignals: 1 },
      metacognition: { highestSolo: "L3", distribution: { L1: 0, L2: 0, L3: 2, L4: 0 }, spontaneous: 1, prompted: 1 },
    };
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    const m = await getAbilityModel();
    expect(m.depth.length).toBe(4);
    expect(m.totalSessions).toBe(2);
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/api/ability.test.ts`
Expected: FAIL (module missing).

- [ ] **Step 3: Implement** `apps/web/src/api/ability.ts`:

```ts
import { AbilityModel } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// C: the student-level 能力素养 model — a deterministic cross-session merge. Read-only.
export async function getAbilityModel(): Promise<AbilityModel> {
  const raw = await apiFetch<unknown>(`/api/v1/growth/ability`);
  return AbilityModel.parse(raw);
}
```

- [ ] **Step 4: Wire the facade** `apps/web/src/api/index.ts` — mirror `getGrowthHistory`'s three touch points (import, the `Api` interface method, the object literal). Add:
  - import: `import { getAbilityModel } from "./ability";`
  - interface: `getAbilityModel(): Promise<AbilityModel>;` (import the `AbilityModel` type alongside the other contract types already imported there)
  - object: `getAbilityModel,`

- [ ] **Step 5: Run — expect PASS**

Run: `cd apps/web && npx vitest run src/api/ability.test.ts && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/api/ability.ts apps/web/src/api/index.ts apps/web/src/api/ability.test.ts
git commit -m "feat(c): web getAbilityModel client + facade"
```

---

### Task 6: `<AbilityModel>` component

**Files:**
- Create: `apps/web/src/shell/growth/AbilityModel.tsx`
- Create: `apps/web/src/shell/growth/AbilityModel.test.tsx`

**Interfaces:**
- Consumes: `api.getAbilityModel()`, the `AbilityModel` contract type.
- Produces: `export function AbilityModel(): JSX.Element` (self-fetching, mirrors how `GrowthReport` self-fetches).

- [ ] **Step 1: Write the failing test** `apps/web/src/shell/growth/AbilityModel.test.tsx`:

```tsx
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AbilityModel } from "./AbilityModel";
import { api } from "../../api";

afterEach(() => { vi.restoreAllMocks(); });

const model = {
  totalSessions: 6,
  depth: [
    { code: "D1", name: "任务理解与问题表述", level: 2, levelLabel: "背景与目标清楚", evidenceCount: 4 },
    { code: "D3", name: "证据与信源意识", level: -1, levelLabel: "", evidenceCount: 1 },
    { code: "D4", name: "论证结构意识", level: 3, levelLabel: "识别缺失 warrant", evidenceCount: 3 },
    { code: "D5", name: "反馈理解与修改理由", level: -1, levelLabel: "", evidenceCount: 0 },
  ],
  autonomy: { sessions: 6, boundarySettings: 11, adversaryInvites: 0, anchoredSignals: 18, promptedSignals: 9 },
  metacognition: { highestSolo: "L4", distribution: { L1: 0, L2: 1, L3: 7, L4: 2 }, spontaneous: 3, prompted: 7 },
};

describe("AbilityModel", () => {
  it("renders the merged caption, a scored dim, and the low-N guard", async () => {
    vi.spyOn(api, "getAbilityModel").mockResolvedValue(model as never);
    render(<AbilityModel />);
    await waitFor(() => expect(screen.getByText(/不是测验分数/)).toBeTruthy());
    expect(screen.getByText(/任务理解与问题表述/)).toBeTruthy();
    expect(screen.getAllByText(/证据不足/).length).toBeGreaterThan(0); // D3 (1 session) + D5 (0)
    expect(screen.getByText(/对手邀请/)).toBeTruthy(); // autonomy panel
    expect(screen.getByText(/L4/)).toBeTruthy();       // metacognition highest
  });

  it("renders an empty state when there are no sessions", async () => {
    vi.spyOn(api, "getAbilityModel").mockResolvedValue({ ...model, totalSessions: 0, depth: model.depth.map((d) => ({ ...d, level: -1, evidenceCount: 0 })) } as never);
    render(<AbilityModel />);
    await waitFor(() => expect(screen.getByText(/还没有足够的数据/)).toBeTruthy());
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/growth/AbilityModel.test.tsx`
Expected: FAIL (module missing).

- [ ] **Step 3: Implement** `apps/web/src/shell/growth/AbilityModel.tsx`. Render (following the binding 能力素养 tab structure + design tokens; inline SVG radar over the 4 depth dims on a 0–3 scale). Skeleton:

```tsx
import { useEffect, useState } from "react";
import type { AbilityModel as AbilityModelT } from "@mind-imprint/contracts";
import { api } from "../../api";

const RADAR_MAX = 3; // depth scores are 0..3

// four spokes at 12/3/6/9 o'clock; value 0..3 → radius fraction. Insufficient (-1) → center.
function radarPoints(levels: number[], cx: number, cy: number, r: number): string {
  const angles = [-90, 0, 90, 180]; // degrees, clockwise from top
  return levels
    .map((lv, i) => {
      const frac = lv < 0 ? 0 : lv / RADAR_MAX;
      const a = (angles[i]! * Math.PI) / 180;
      return `${cx + Math.cos(a) * r * frac},${cy + Math.sin(a) * r * frac}`;
    })
    .join(" ");
}

export function AbilityModel() {
  const [model, setModel] = useState<AbilityModelT | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const m = await api.getAbilityModel();
        if (!cancelled) setModel(m);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, []);

  if (error) return <div style={{ padding: 24, color: "#B0432E" }}>{error}</div>;
  if (!model) return <div style={{ padding: 24, color: "#9AA1B0" }}>正在整理你的能力画像…</div>;
  if (model.totalSessions === 0) {
    return (
      <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "34px 24px", textAlign: "center" }}>
        <div style={{ fontSize: 14.5, fontWeight: 700, color: "#3A4256" }}>还没有足够的数据</div>
        <div style={{ fontSize: 13.5, color: "#6B7384", marginTop: 8, lineHeight: 1.7 }}>完成更多任务后，你的能力画像会在这里浮现。</div>
      </div>
    );
  }

  const cx = 140, cy = 135, r = 100;
  const levels = model.depth.map((d) => d.level);

  return (
    <div>
      {/* caption (binding text, verbatim) */}
      <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px" }}>
        <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333" }}>AI 批判性思维 · 能力素养模型</div>
        <div style={{ fontSize: 12.5, color: "#8A92A3", marginTop: 5, lineHeight: 1.6 }}>
          等级来自每次任务评估的归并，不是测验分数。已汇集 {model.totalSessions} 次会话。
        </div>
        {/* depth radar over the 4 scored dims */}
        <div style={{ display: "flex", justifyContent: "center", marginTop: 10 }}>
          <svg viewBox="0 0 280 270" width="100%" style={{ maxWidth: 330 }}>
            {[0.33, 0.66, 1].map((ring) => (
              <polygon key={ring} points={radarPoints([RADAR_MAX * ring, RADAR_MAX * ring, RADAR_MAX * ring, RADAR_MAX * ring], cx, cy, r)} fill="none" stroke="#ECEEF4" strokeWidth="1" />
            ))}
            <polygon points={radarPoints(levels, cx, cy, r)} fill="rgba(42,59,122,.14)" stroke="#2A3B7A" strokeWidth="2" strokeLinejoin="round" />
            {model.depth.map((d, i) => {
              const a = ([-90, 0, 90, 180][i]! * Math.PI) / 180;
              return <text key={d.code} x={cx + Math.cos(a) * (r + 16)} y={cy + Math.sin(a) * (r + 16)} textAnchor="middle" fontSize="10.5" fontWeight="600" fill="#6B7384">{d.name}</text>;
            })}
          </svg>
        </div>
      </div>

      {/* per-dim depth list */}
      <div style={{ marginTop: 12 }}>
        {model.depth.map((d) => (
          <div key={d.code} style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, padding: "13px 16px", marginBottom: 9 }}>
            <div style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
              <span style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{d.name}</span>
              {d.level < 0 ? (
                <span style={{ fontSize: 12, color: "#8A6D3B", background: "#FDF7EC", border: "1px solid #E3CFA4", borderRadius: 999, padding: "2px 9px" }}>证据不足 · 需更多任务</span>
              ) : (
                <span style={{ fontSize: 12, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", borderRadius: 999, padding: "2px 9px" }}>Lv{d.level} · {d.evidenceCount} 次</span>
              )}
            </div>
            {d.level >= 0 && d.levelLabel ? <div style={{ fontSize: 12.5, color: "#6B7384", marginTop: 6, lineHeight: 1.6 }}>{d.levelLabel}</div> : null}
          </div>
        ))}
      </div>

      {/* 智识自主 observation panel (no level) */}
      <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, padding: "16px", marginTop: 4 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>智识自主</span>
          <span style={{ fontSize: 11, color: "#B16A18", background: "#F5E5CE", borderRadius: 999, padding: "1px 8px" }}>观察 · 不计分</span>
        </div>
        <div style={{ fontSize: 12.5, color: "#4C5653", marginTop: 8, lineHeight: 1.7 }}>
          跨 {model.autonomy.sessions} 次会话：边界设定 ×{model.autonomy.boundarySettings} · 对手邀请 ×{model.autonomy.adversaryInvites} · 自发信号 {model.autonomy.anchoredSignals} / 引导后 {model.autonomy.promptedSignals}
        </div>
      </div>

      {/* 跨轴 元认知 distribution panel */}
      <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, padding: "16px", marginTop: 12 }}>
        <div style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>元认知 · SOLO 分布</div>
        <div style={{ fontSize: 12.5, color: "#4C5653", marginTop: 8, lineHeight: 1.7 }}>
          最高 {model.metacognition.highestSolo || "—"}　·　L1 {model.metacognition.distribution.L1 ?? 0} · L2 {model.metacognition.distribution.L2 ?? 0} · L3 {model.metacognition.distribution.L3 ?? 0} · L4 {model.metacognition.distribution.L4 ?? 0}　·　自发 {model.metacognition.spontaneous} / 引导后 {model.metacognition.prompted}
        </div>
      </div>
    </div>
  );
}
```

(If `apps/web` uses a shared design-token module, prefer it over the inline hex above; otherwise these match `GrowthReport.tsx`'s existing palette. Keep the radar inline SVG.)

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/web && npx vitest run src/shell/growth/AbilityModel.test.tsx && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/growth/AbilityModel.tsx apps/web/src/shell/growth/AbilityModel.test.tsx
git commit -m "feat(c): <AbilityModel> component (depth radar + observation panels)"
```

---

### Task 7: Tabbed 成长报告 (学习记录 + 能力素养)

**Files:**
- Modify: `apps/web/src/shell/growth/GrowthReport.tsx`
- Modify/Create: `apps/web/src/shell/growth/GrowthReport.test.tsx`

**Interfaces:**
- Consumes: `<AbilityModel>` (Task 6). The existing history rendering stays; it just moves under a 学习记录 tab.

- [ ] **Step 1: Write/extend the failing test** `apps/web/src/shell/growth/GrowthReport.test.tsx` — assert both tabs exist, 学习记录 shows the history (default), and clicking 能力素养 renders the ability view:

```tsx
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { GrowthReport } from "./GrowthReport";
import { api } from "../../api";

afterEach(() => { vi.restoreAllMocks(); });

const emptyAbility = {
  totalSessions: 0,
  depth: ["D1", "D3", "D4", "D5"].map((code) => ({ code, name: code, level: -1, levelLabel: "", evidenceCount: 0 })),
  autonomy: { sessions: 0, boundarySettings: 0, adversaryInvites: 0, anchoredSignals: 0, promptedSignals: 0 },
  metacognition: { highestSolo: "", distribution: { L1: 0, L2: 0, L3: 0, L4: 0 }, spontaneous: 0, prompted: 0 },
};

describe("GrowthReport tabs", () => {
  it("defaults to 学习记录 and switches to 能力素养", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([]);
    vi.spyOn(api, "getAbilityModel").mockResolvedValue(emptyAbility as never);
    render(<GrowthReport />);
    // both tabs present
    expect(screen.getByRole("button", { name: /学习记录/ })).toBeTruthy();
    const abilityTab = screen.getByRole("button", { name: /能力素养/ });
    // default tab is history (empty-state copy from the history hub)
    await waitFor(() => expect(screen.getByText(/还没有报告/)).toBeTruthy());
    // switch
    fireEvent.click(abilityTab);
    await waitFor(() => expect(screen.getByText(/还没有足够的数据/)).toBeTruthy());
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/growth/GrowthReport.test.tsx`
Expected: FAIL (no tab buttons yet).

- [ ] **Step 3: Refactor `GrowthReport.tsx`** — keep the current history rendering intact, but wrap it in a tab shell. Extract the existing history body (the `entries` fetch + hero + list) into a local `LearningRecord()` component (verbatim move — do NOT change its behavior or copy, incl. the "还没有报告" empty state), then:

```tsx
export function GrowthReport() {
  const [tab, setTab] = useState<"learning" | "ability">("learning");
  const tabStyle = (active: boolean) => ({
    padding: "8px 16px", borderRadius: 999, border: "none", cursor: "pointer", fontFamily: "inherit",
    fontSize: 13.5, fontWeight: 700,
    background: active ? "#2A3B7A" : "transparent", color: active ? "#fff" : "#6B7384",
  });
  return (
    <div style={{ flex: 1, minHeight: 0, height: "100%", overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
        <div style={{ display: "flex", gap: 8, marginBottom: 18 }}>
          <button type="button" style={tabStyle(tab === "learning")} onClick={() => setTab("learning")}>学习记录</button>
          <button type="button" style={tabStyle(tab === "ability")} onClick={() => setTab("ability")}>能力素养</button>
        </div>
        {tab === "learning" ? <LearningRecord /> : <AbilityModel />}
      </div>
    </div>
  );
}
```

Move the current outer scroll container's padding into the wrapper as shown (so `LearningRecord` renders just the hero + list, and `AbilityModel` renders under the same width). Import `AbilityModel` from `./AbilityModel`. The `LearningRecord` component keeps the existing `useEffect`/`entries`/`HistoryRow` logic unchanged.

- [ ] **Step 4: Run the web report tests + tsc — expect PASS**

Run: `cd apps/web && npx vitest run src/shell/growth/ && npx tsc --noEmit`
Expected: PASS (the new tab test + the untouched history behavior), tsc clean.

- [ ] **Step 5: Run the FULL web + contracts suites — expect PASS**

Run: `cd packages/contracts && npx vitest run && cd ../../apps/web && npx vitest run`
Expected: PASS across both (nothing else regressed).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/shell/growth/GrowthReport.tsx apps/web/src/shell/growth/GrowthReport.test.tsx
git commit -m "feat(c): tabbed 成长报告 — 学习记录 + 能力素养"
```

---

## Self-Review

**Spec coverage:**
- §1a depth merge (recency-weighted, ≥1 evidence rule, low-N guard) → Task 1.
- §1b autonomy sums → Task 1. §1c SOLO distribution → Task 1. §1d structural separation → Tasks 1 (no cross-axis field) + 6 (three visual blocks).
- §2.1 pure projection → Task 1. §2.2 query → Task 2. §2.3 endpoint (no cost) → Task 3. §2.4 contract + client → Tasks 4–5.
- §3.1 tabbed restructure → Task 7. §3.2 `<AbilityModel>` (radar + 3 panels + 证据不足 + empty) → Task 6.
- §4 tests + invariants → each task's tests; deterministic/no-cost asserted in Task 3; owner-isolation in Task 2; low-N in Tasks 1/6.
- §5 out-of-scope (工具卡, trajectory, cross-student) → not built.

**Placeholder scan:** the only deferred-to-implementer bits point at concrete existing code to copy (`growth_history_query_test.go` seed scaffolding in Task 2; `countLLMCalls` real form in Task 3 — the plan flags the placeholder explicitly and names the exact pattern to use). No config `…` or TODO in shipped code.

**Type consistency:** `ability.Model`/`DepthAbility`/`AutonomyAbility`/`Metacognition` field names + json tags (Task 1) match the Zod `AbilityModel` (Task 4), the endpoint decode (Task 3), and the component/client fixtures (Tasks 5–6): `totalSessions`, `depth[].{code,name,level,levelLabel,evidenceCount}`, `autonomy.{sessions,boundarySettings,adversaryInvites,anchoredSignals,promptedSignals}`, `metacognition.{highestSolo,distribution,spontaneous,prompted}`. `Aggregate` signature identical in Tasks 1 and 3. Level `-1` sentinel consistent everywhere.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-19-c-student-ability-model.md`.

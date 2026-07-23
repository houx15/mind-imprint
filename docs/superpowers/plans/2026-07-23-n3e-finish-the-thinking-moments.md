# N3e · Finish the Thinking Moments — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the last two N3 items — a search-plan tool card that critiques the student's own retrieval directions, and the R-9 framework reveal in the 成长报告 工具卡 tab — closing the N3 family.

**Architecture:** Component A is a new `matrix` card (`search-plan`) whose rows are seeded server-side from the student's `preregistration` graph node; it needs one small renderer fix (parameterize the matrix row-noun, hardcoded 视角 today) and a new structural summon branch. Component B is pure-web: `ToolkitCards.tsx` reads each completed card's methodology from `CARD_REGISTRY` and shows the named thinking move. No migration, no gate, no LLM, no server change for Component B.

**Tech Stack:** Go (`net/http`, `pgx`, sqlc), React + Vite + TypeScript, Zod contracts, card JSON single-source in `packages/contracts/cards/`.

**Spec:** `docs/superpowers/specs/2026-07-23-n3e-finish-the-thinking-moments-design.md`

## Global Constraints

- **Card JSON authored ONLY in `packages/contracts/cards/`**, mirrored into `apps/api/internal/cards/specs/` by `cd apps/api && make sync-cards`. **NEVER hand-edit `specs/`.**
- **New card = new JSON + (here) one contained renderer fix.** Do not add a new primitive or a new matrix host. Reuse the `matrix` primitive.
- **NO migration** — `framework_fill` exists since 0016, `preregistration` nodes since N3d. No new column/table.
- **NO gate touched, NO new gate item** — the producerless-gate guard (`apps/api/internal/skills/producers_test.go`) must stay green.
- **NO LLM call added.** Summon is structural (classifier). Seeding is a store read + write. Component B is a spec lookup.
- **铁律 1** — the card guides with questions, never answers, never 替学生定论. **铁律 2** — offer never a wall; no badge/score/streak/celebration; the reveal is a plain naming. **铁律 4** — the critique becomes process data (the card_instance the assessor already reads).
- **Go tests:** from `apps/api`, `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...` — **FULL packages, never `-run` subsets** for card/summon/projection changes.
- **Web tests:** from `apps/web`, `npm test` and `npx tsc --noEmit`. **Contracts:** from `packages/contracts`, `npm test` (tests in `test/`).
- **Never `git add` a whole directory** — the pre-existing `M package.json` and untracked `docs/` files are NOT ours. Add named files only.
- **Icons inline SVG**; never import `lucide-react`.

---

## File Structure

- `packages/contracts/cards/search-plan.json` — **new**, the card (Component A).
- `packages/contracts/src/registry.ts` — **modify**, register the card in `DEFAULT_RAW`.
- `apps/api/internal/cards/specs/search-plan.json` — **generated** by `make sync-cards` (never hand-edited).
- `apps/web/src/primitives/matrix/Matrix.tsx` — **modify**, parameterize the row-noun.
- `apps/web/src/studio/StudioMatrixCard.tsx` — **modify**, pass `row_noun` through.
- `apps/api/internal/agent/classifier.go` — **modify**, add the `search-plan` summon branch.
- `apps/api/internal/api/studioturn.go` — **modify**, seed the matrix rows on surface (new `seedSearchPlanAnchors` helper + a `matrix` branch in `streamAction`).
- `apps/web/src/shell/growth/ToolkitCards.tsx` — **modify**, the R-9 reveal (Component B).
- `docs/2026-07-20-student-platform-remaining-work.md` — **modify**, mark N3e DONE (Task 6).

---

## Task 1: The search-plan card JSON + registration

**Files:**
- Create: `packages/contracts/cards/search-plan.json`
- Modify: `packages/contracts/src/registry.ts` (imports block ~line 33; `DEFAULT_RAW` ~line 72)
- Generated: `apps/api/internal/cards/specs/search-plan.json` (via `make sync-cards`)
- Test: `packages/contracts/test/registry.test.ts` (existing file — add a case; if absent, create it)

**Interfaces:**
- Produces: card id `"search-plan"`, `primitive: "matrix"`, `target_type: "project"`, cols `evidence_type` / `blind_spot` / `disconfirm`, `params.row_noun: "检索方向"`, `consolidation: "reveal_framework_after_completion"`. Later tasks (3, 4, 5) reference this id and these col ids verbatim.

- [ ] **Step 1: Write the card JSON.** Create `packages/contracts/cards/search-plan.json` with EXACTLY this content:

```json
{
  "id": "search-plan",
  "category": "溯源与多视角",
  "name": "检索方向审视",
  "name_en": "Search-Plan Audit",
  "purpose": "在真正去查之前，逐条想清楚每个检索方向会给你哪一类证据、又系统性地漏掉什么",
  "trigger_condition": "学生已经写下检索计划（打算去哪找证据），但没有想过每个方向的偏差与盲区",
  "trigger_keywords": ["去哪找", "检索", "找证据", "搜", "查资料"],
  "priority": "P1",
  "disclosure_tier": "tier-1",
  "age_band": ["MYP", "DP"],
  "interaction_type": "画布导图卡",
  "rubric_dims": ["D3"],
  "related": ["sift", "craap", "perspective-matrix"],
  "body_status": "full",
  "rubric_tags": ["D3"],
  "primitive": "matrix",
  "target_type": "project",
  "params": {
    "cols": [
      { "id": "evidence_type", "label": "会给什么", "q": "这个方向最可能给你哪一类证据？" },
      { "id": "blind_spot", "label": "看不见什么", "q": "它系统性地看不见什么？绕开了谁、绕开了哪种反面情况？" },
      { "id": "disconfirm", "label": "能否证伪", "q": "如果你的结论其实是错的，这个方向找得到反证吗，还是只会印证你？" }
    ],
    "min_items": 1,
    "row_prompt": "你检索计划里的一条方向",
    "row_noun": "检索方向"
  },
  "completion": [{ "kind": "matrix_complete" }],
  "graph_effects": [],
  "consolidation": "reveal_framework_after_completion",
  "intrusiveness_cap": "I3",
  "steps": [
    {
      "key": "rows",
      "title": "逐条审视检索方向",
      "disclose": "always",
      "methodology": {
        "why": "多数「找错证据」不是懒，而是只往会同意你的地方找——检索计划里每个方向都带着它自己的偏差和盲区，先看清，再去查，比查完一堆再发现全是一边的声音要省力得多。",
        "how": "对每条方向问三件事：它会给你哪一类证据、它看不见什么、以及它能不能证伪你——如果一个方向永远只会印证你，它就不是在帮你查证，是在帮你说服自己。",
        "when": "在你按这份计划真正开始检索之前。"
      },
      "fields": [
        { "type": "repeatable_group", "key": "directions", "label": "每行一条检索方向", "item_fields": [
          { "type": "text", "key": "direction", "label": "检索方向" },
          { "type": "textarea", "key": "evidence_type", "label": "会给你哪一类证据？" },
          { "type": "textarea", "key": "blind_spot", "label": "看不见什么？" },
          { "type": "textarea", "key": "disconfirm", "label": "能不能证伪你？" }
        ] }
      ]
    }
  ]
}
```

- [ ] **Step 2: Register it in the web registry.** In `packages/contracts/src/registry.ts`, add the import (keep alphabetical grouping — after the `scienceKnowing` / before `sift` import is fine):

```ts
import searchPlan from "../cards/search-plan.json";
```

and add to `DEFAULT_RAW` (near the other溯源 cards):

```ts
  "search-plan": searchPlan,
```

- [ ] **Step 3: Write the failing test.** In `packages/contracts/test/registry.test.ts`, add:

```ts
import { describe, it, expect } from "vitest";
import { CARD_REGISTRY } from "../src/registry";

describe("search-plan card", () => {
  it("is in the registry and is a project-scoped matrix card with the R-9 consolidation", () => {
    const spec = CARD_REGISTRY["search-plan"];
    expect(spec).toBeDefined();
    expect(spec!.primitive).toBe("matrix");
    expect(spec!.target_type).toBe("project");
    expect(spec!.consolidation).toBe("reveal_framework_after_completion");
    const params = spec!.params as { cols: { id: string }[]; row_noun: string };
    expect(params.cols.map((c) => c.id)).toEqual(["evidence_type", "blind_spot", "disconfirm"]);
    expect(params.row_noun).toBe("检索方向");
  });
});
```

- [ ] **Step 4: Run it to verify it fails.**

Run: `cd packages/contracts && npm test -- registry`
Expected: FAIL — either `spec` is undefined (not yet imported) or, if you skipped Step 2, `loadRegistry` never saw it.

- [ ] **Step 5: Make it pass.** Confirm Steps 1–2 are done; `loadRegistry()` validates the JSON against the Zod `CardSpec` at import and throws if the id≠key or a required field is missing. Re-run:

Run: `cd packages/contracts && npm test -- registry`
Expected: PASS. If it throws `Invalid card "search-plan": …`, fix the JSON per the Zod error (every `steps[].methodology` field is required non-empty; `rubric_tags` is required).

- [ ] **Step 6: Mirror to Go + verify the Go catalog loads it.**

Run: `cd apps/api && make sync-cards`
Then confirm the card is embedded and resolvable:

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/cards/...`
Expected: PASS (the cards package loads every embedded spec at init; a malformed `search-plan.json` would fail this). If the package has a catalog/`ByID` test, it now includes `search-plan`.

- [ ] **Step 7: Commit.**

```bash
git add packages/contracts/cards/search-plan.json packages/contracts/src/registry.ts packages/contracts/test/registry.test.ts apps/api/internal/cards/specs/search-plan.json
git commit -m "feat(n3e): add the search-plan card (matrix, project-scoped)"
```

---

## Task 2: Parameterize the matrix row-noun

The matrix primitive hardcodes 视角 in three UI strings; a 检索方向 card would show the wrong noun. Add an optional `row_noun` (default 视角, so `perspective-matrix` is unchanged) and thread it through.

**Files:**
- Modify: `apps/web/src/primitives/matrix/Matrix.tsx` (props + 3 strings)
- Modify: `apps/web/src/studio/StudioMatrixCard.tsx` (read `params.row_noun`, pass it)
- Test: `apps/web/src/primitives/matrix/Matrix.test.tsx` (existing or new); `apps/web/src/studio/StudioMatrixCard.test.tsx` (existing)

**Interfaces:**
- Consumes: `search-plan.json`'s `params.row_noun` (Task 1).
- Produces: `Matrix` gains a `rowNoun: string` prop; `StudioMatrixCard` reads `p.row_noun ?? "视角"` and passes it.

- [ ] **Step 1: Write the failing test.** In `apps/web/src/primitives/matrix/Matrix.test.tsx` add:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Matrix } from "./Matrix";

const cols = [{ id: "a", label: "A", q: "qa" }];

describe("Matrix row-noun", () => {
  it("uses the given rowNoun in the add button and progress line", () => {
    render(
      <Matrix cols={cols} state={{ rows: [] }} minItems={1} rowPrompt="p" rowNoun="检索方向"
        onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
    );
    expect(screen.getByText(/添加一个检索方向/)).toBeInTheDocument();
    expect(screen.getByText(/已完成 0 个检索方向/)).toBeInTheDocument();
  });

  it("defaults nothing — the noun is always supplied by the host (perspective-matrix passes 视角)", () => {
    render(
      <Matrix cols={cols} state={{ rows: [] }} minItems={1} rowPrompt="p" rowNoun="视角"
        onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
    );
    expect(screen.getByText(/添加一个视角/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run it to verify it fails.**

Run: `cd apps/web && npm test -- Matrix`
Expected: FAIL — `rowNoun` is not a prop; the strings are hardcoded 视角.

- [ ] **Step 3: Add the prop and thread it.** In `apps/web/src/primitives/matrix/Matrix.tsx`:
  - Add `rowNoun: string;` to `MatrixProps` (after `rowPrompt`).
  - Destructure `rowNoun` in the `Matrix({ ... })` signature.
  - Replace the three hardcoded strings:
    - add button `添加一个视角` → `` `添加一个${rowNoun}` ``
    - progress line `已完成 {completeCount} 个视角 · 还差 {remaining} 个` → `` `已完成 ${completeCount} 个${rowNoun} · 还差 ${remaining} 个` `` (keep it plain text — 铁律 2)
    - remove aria-label `` `删除第 ${index + 1} 个视角` `` → `` `删除第 ${index + 1} 个${rowNoun}` ``

- [ ] **Step 4: Pass it from the host.** In `apps/web/src/studio/StudioMatrixCard.tsx`:
  - In the `p` type add `row_noun?: string;`.
  - After `const rowPrompt = p.row_prompt ?? "";` add `const rowNoun = p.row_noun ?? "视角";`.
  - Add `rowNoun={rowNoun}` to the `<Matrix ... />` props.

- [ ] **Step 5: Run tests to verify they pass.**

Run: `cd apps/web && npm test -- Matrix StudioMatrixCard && npx tsc --noEmit`
Expected: PASS, tsc clean. Existing perspective-matrix tests still pass (default 视角 preserved).

- [ ] **Step 6: Commit.**

```bash
git add apps/web/src/primitives/matrix/Matrix.tsx apps/web/src/studio/StudioMatrixCard.tsx apps/web/src/primitives/matrix/Matrix.test.tsx
git commit -m "feat(n3e): parameterize the matrix row-noun (default 视角)"
```

---

## Task 3: The summon branch

Add a project-scoped branch to `agent.SurfaceCardCandidates` that offers `search-plan` once the student has written a search plan (`preregistration` node exists) and no `search-plan` instance has ever been seen. Model it exactly on the `perspective-matrix` branch (`apps/api/internal/agent/classifier.go` ~line 229–243), and place it BEFORE that branch.

**Files:**
- Modify: `apps/api/internal/agent/classifier.go`
- Test: `apps/api/internal/agent/classifier_test.go` (existing)

**Interfaces:**
- Consumes: `GraphView` (`g.Nodes` with `Type`, `g.CardInstances` with `CardID`). A `preregistration` node's presence is the trigger — the `directions` are read later (Task 4), not here.
- Produces: a `Candidate{Verb:"surface_card", AnchorKind:"project", AnchorID:"", CardID:"search-plan", Reason:"search plan not yet interrogated"}`.

- [ ] **Step 1: Write the failing test.** In `apps/api/internal/agent/classifier_test.go` add (match the file's existing GraphView-construction helper style):

```go
func TestSurfaceCardCandidates_SearchPlan(t *testing.T) {
	base := GraphView{Nodes: []GraphNodeView{{ID: "n1", Type: "preregistration", Author: "student"}}}

	// Fires when a preregistration node exists and no search-plan instance seen.
	got := SurfaceCardCandidates(base)
	if !hasCardCandidate(got, "search-plan") {
		t.Fatalf("expected search-plan candidate, got %+v", got)
	}

	// Suppressed once ANY search-plan instance exists (incl. skipped).
	for _, status := range []string{"proposed", "active", "completed", "skipped"} {
		g := base
		g.CardInstances = []CardInstanceView{{ID: "c1", CardID: "search-plan", Status: status}}
		if status == "proposed" || status == "active" {
			// the in-flight guard already suppresses everything; assert no panic + no dup
		}
		if hasCardCandidate(SurfaceCardCandidates(g), "search-plan") {
			t.Fatalf("search-plan should be suppressed when an instance is %q", status)
		}
	}

	// No preregistration node → no offer.
	if hasCardCandidate(SurfaceCardCandidates(GraphView{}), "search-plan") {
		t.Fatal("search-plan must not fire without a preregistration node")
	}
}

// hasCardCandidate reports whether any candidate surfaces cardID.
func hasCardCandidate(cands []Candidate, cardID string) bool {
	for _, c := range cands {
		if c.Verb == "surface_card" && c.CardID == cardID {
			return true
		}
	}
	return false
}
```

> If a `hasCardCandidate` helper or `CardInstanceView` field names differ in the file, adapt to the existing ones — do not introduce a second helper if one exists.

- [ ] **Step 2: Run it to verify it fails.**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/ -run TestSurfaceCardCandidates_SearchPlan`
Expected: FAIL — no search-plan candidate produced.

- [ ] **Step 3: Add the branch.** In `apps/api/internal/agent/classifier.go`, add a const near `perspectiveMatrixCardID` (~line 71):

```go
	searchPlanCardID = "search-plan"
```

Then, immediately BEFORE the `perspective-matrix` branch (~line 229, the `perspectiveMatrixSeen` block), add:

```go
	// Project-scoped search-plan surface (N3e): offer once the student has
	// written a search plan (a `preregistration` node exists) and no
	// search-plan instance has ever been seen. Placed before perspective-matrix
	// because S1 (framing / search plan) is strictly upstream of
	// evaluate_perspectives; the loop acts on cands[0], so list order is
	// priority. Suppression keys on instance existence in ANY status — a
	// project-scoped card mints no per-material edge, and an offer is never a
	// wall (铁律 2): once she has said no, we never ask again.
	hasPrereg := false
	for _, n := range g.Nodes {
		if n.Type == "preregistration" {
			hasPrereg = true
		}
	}
	searchPlanSeen := false
	for _, ci := range g.CardInstances {
		if ci.CardID == searchPlanCardID {
			searchPlanSeen = true
		}
	}
	// !anyEvaluated: search-plan is a PRE-sourcing card (its `when` is 「在你
	// 按这份计划真正开始检索之前」). Retire it the moment she evaluates her first
	// source, so it never becomes cands[0] at S4 and hijacks toulmin. This also
	// makes it mutually exclusive with perspective-matrix/toulmin (both require
	// anyEvaluated), so no priority conflict is possible. `anyEvaluated` is the
	// graph-wide fact computed just above for those branches.
	if hasPrereg && !anyEvaluated && !searchPlanSeen {
		out = append(out, Candidate{
			Verb:       "surface_card",
			AnchorKind: "project",
			AnchorID:   "",
			CardID:     searchPlanCardID,
			Reason:     "search plan not yet interrogated",
		})
	}
```

> Confirm the local accumulator is named `out` (it is at the perspective-matrix branch); match it.

- [ ] **Step 4: Run the test to verify it passes.**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/ -run TestSurfaceCardCandidates_SearchPlan`
Expected: PASS.

- [ ] **Step 5: Run the FULL agent package** (classifier change — no `-run` subset).

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/`
Expected: PASS. No existing classifier test regresses.

- [ ] **Step 6: Commit.**

```bash
git add apps/api/internal/agent/classifier.go apps/api/internal/agent/classifier_test.go
git commit -m "feat(n3e): summon search-plan when a preregistration node exists"
```

---

## Task 4: Seed the rows from her plan

When the search-plan card surfaces, seed one matrix row per search direction so she critiques her *own* plan rather than re-typing it. This mirrors `surfaceAnchors` exactly (build → persist → carry on the SSE `card` frame), so the seed reaches the client on FIRST surface AND on reload.

**Files:**
- Modify: `apps/api/internal/api/studioturn.go` (add `seedSearchPlanAnchors`; add a branch in `streamAction`'s `surface_card` case)
- Test: `apps/api/internal/api/studioturn_test.go` (existing) or a focused new `apps/api/internal/api/search_plan_seed_test.go`

**Interfaces:**
- Consumes: `search-plan` card id (Task 1); `SetCardInstanceAnchors(ctx, projectID, id uuid.UUID, anchors []byte)` on `agent.AgentStore` (loop.go:134); `a.d.Queries.ListGraphNodesByProject(ctx, projectID)` (returns `[]sqlc.GraphNode` ordered by `created_at`); the `preregistration` body shape `{ "directions": []string }` (framing.go:107, projection.go:417-424).
- Produces: seeded anchors of shape `{ quote: <direction>, dimension: "evidence_type", answer: "", author: "student" }` — one per direction — persisted on the card_instance and returned as JSON for the `card` frame.

- [ ] **Step 1: Write the failing test.** Create `apps/api/internal/api/search_plan_seed_test.go` (package `api_test`, same as `walk_s0_s6_test.go`). Reuse that file's harness helpers verbatim — `newAPITestPool`, `signInSeed`, `createProjectForTest`, `withCookie`, and especially `surfaceWalkCard(t, h, pool, cookie, projectID, wantCardID, prompt)`, which drives the real `POST /turn`, asserts the card surfaced, and **returns the surfaced card_instance's anchors** (`[]agent.Anchor`) — exactly the seeded rows. Do NOT stand up a second harness or a DB-scan helper; the anchors come back from `surfaceWalkCard`.

```go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// setupProjectAtS1WithSearchPlan: create → onboarding → framing with a
// two-direction search plan. Returns everything the surface helpers need.
// Mirrors TestWalk_S0ToS6_FreshProject's setup (walk_s0_s6_test.go:279-330).
func setupProjectAtS1WithSearchPlan(t *testing.T, dirs [2]string) (h http.Handler, cookie *http.Cookie, pid string, pool *pgxpool.Pool) {
	t.Helper()
	pool = newAPITestPool(t)
	cookie = signInSeed(t, pool)
	h = New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(),
		SpecByID: cards.ByID,
	}).Handler()
	pid = createProjectForTest(t, h, cookie)
	postOK(t, h, cookie, "/api/v1/projects/"+pid+"/onboarding", `{"restate":"x","weakPicks":[0,2]}`)
	postOK(t, h, cookie, "/api/v1/projects/"+pid+"/framing",
		`{"terms":[{"term":"可持续发展","definition":"资源使用不损害后代满足自身需求的能力这是环境定义"},{"term":"中国角色","definition":"中国政策与产出对全球环境指标造成的净影响这是国家定义"},{"term":"世界","definition":"全球尺度而非仅中国境内的地理与生态范围这是空间定义"}],"answers":["中国的可再生能源投入使全球减排加快"],"searchPlan":["`+dirs[0]+`","`+dirs[1]+`"]}`)
	return h, cookie, pid, pool
}

// postOK is a thin 200-asserting POST helper. If walk_s0_s6_test.go already
// has an equivalent, use that and delete this.
func postOK(t *testing.T, h http.Handler, cookie *http.Cookie, path, body string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", path, strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST %s = %d; body=%s", path, rec.Code, rec.Body)
	}
}

func TestSearchPlanSeed_OneRowPerDirection(t *testing.T) {
	dirs := [2]string{"查NASA卫星植被数据", "查中国官方碳排放文件"}
	h, cookie, pid, pool := setupProjectAtS1WithSearchPlan(t, dirs)

	// The turn surfaces the search-plan card; surfaceWalkCard returns its anchors.
	_, _, anchors := surfaceWalkCard(t, h, pool, cookie, pid, "search-plan", "我打算开始找证据了")

	got := map[string]string{}
	for _, a := range anchors {
		got[a.Quote] = a.Answer
	}
	if len(got) != 2 || got[dirs[0]] != "" || got[dirs[1]] != "" {
		t.Fatalf("expected two empty-answer seeded rows keyed by direction, got %+v", got)
	}
}

- [ ] **Step 2: Run it to verify it fails.**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestSearchPlanSeed`
Expected: FAIL — the surfaced matrix card carries `[]` anchors (no seed yet).

- [ ] **Step 3: Add the seed helper.** In `apps/api/internal/api/studioturn.go`, add:

```go
// seedSearchPlanAnchors seeds the search-plan matrix card's rows from the
// student's own retrieval plan: one row per `preregistration` direction, so she
// critiques the plan she wrote rather than re-typing it. Mirrors surfaceAnchors'
// contract exactly — build, persist on the card_instance, return the JSON to
// carry on the same SSE `card` frame — and degrades to (nil,false) on any error
// so the card still surfaces (with no seeded rows) rather than failing the turn.
//
// The anchor shape is what apps/web/src/primitives/matrix/serialize.ts's
// anchorsToMatrixState rehydrates into a labelled, empty-celled row: a non-blank
// `quote` (the direction) and a `dimension` that is a real column id
// ("evidence_type", cols[0]); `answer` stays "" so the server completion
// predicate (firstIncompleteMatrixRow) never counts the seed as done until she
// fills the cells.
func (a *API) seedSearchPlanAnchors(ctx context.Context, store agent.AgentStore, projectID uuid.UUID, cardInstanceID string) ([]byte, bool) {
	nodes, err := a.d.Queries.ListGraphNodesByProject(ctx, projectID)
	if err != nil {
		return nil, false
	}
	// Newest preregistration node wins (nodes are ordered by created_at asc;
	// take the last preregistration seen). Its body is {directions:[]string}.
	var directions []string
	for _, n := range nodes {
		if n.Type != "preregistration" {
			continue
		}
		var body struct {
			Directions []string `json:"directions"`
		}
		if json.Unmarshal(n.Body, &body) == nil {
			directions = body.Directions // last wins
		}
	}
	if len(directions) == 0 {
		return nil, false
	}
	// Build agent.Anchor directly so the persisted shape is byte-for-byte what
	// the reader unmarshals. Only quote/dimension/answer matter to the matrix
	// serializer; the zero-valued positional fields (start/end/…) are ignored.
	seeds := make([]agent.Anchor, 0, len(directions))
	for _, d := range directions {
		if d = strings.TrimSpace(d); d != "" {
			seeds = append(seeds, agent.Anchor{Quote: d, Dimension: "evidence_type", Answer: "", Author: "student"})
		}
	}
	if len(seeds) == 0 {
		return nil, false
	}
	raw, err := json.Marshal(seeds)
	if err != nil {
		return nil, false
	}
	cid, err := uuid.Parse(cardInstanceID)
	if err != nil {
		return nil, false
	}
	if err := store.SetCardInstanceAnchors(ctx, projectID, cid, raw); err != nil {
		return nil, false
	}
	return raw, true
}
```

> Ensure `strings`, `encoding/json`, `github.com/google/uuid` are imported in the file (surfaceAnchors already uses them).

- [ ] **Step 4: Wire it into `streamAction`.** In `apps/api/internal/api/studioturn.go`, in the `case action.Kind == "surface_card":` block, extend the anchor-building to cover the matrix seed. Change:

```go
		if spec.Primitive == "annotate" || spec.Primitive == "compare" {
			if raw, ok := a.surfaceAnchors(ctx, store, projectID, spec, action.CardInstanceID, action.MaterialID); ok {
				anchors = raw
			}
		}
```

to:

```go
		if spec.Primitive == "annotate" || spec.Primitive == "compare" {
			if raw, ok := a.surfaceAnchors(ctx, store, projectID, spec, action.CardInstanceID, action.MaterialID); ok {
				anchors = raw
			}
		} else if spec.ID == "search-plan" {
			// N3e: seed the matrix rows from her preregistration directions so
			// she critiques her own plan (spec §2.3b). Same persist-and-carry
			// contract as surfaceAnchors; degrades to [] if there is no plan.
			if raw, ok := a.seedSearchPlanAnchors(ctx, store, projectID, action.CardInstanceID); ok {
				anchors = raw
			}
		}
```

- [ ] **Step 5: Run the test to verify it passes.**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestSearchPlanSeed`
Expected: PASS — two empty-answer seeded rows persisted.

- [ ] **Step 6: Run the FULL api package** (surface/seed change).

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/`
Expected: PASS.

- [ ] **Step 7: Commit.**

```bash
git add apps/api/internal/api/studioturn.go apps/api/internal/api/search_plan_seed_test.go
git commit -m "feat(n3e): seed search-plan matrix rows from her preregistration directions"
```

---

## Task 5: The R-9 framework reveal (Component B)

In the 成长报告 工具卡 tab, show — for each completed card whose spec sets `consolidation` — the named thinking move (the card's methodology `why`). Pure web: read from `CARD_REGISTRY`. The framework *name* is already the card title (`spec.name`, shown), so the reveal's added content is the `why` (先做再命名 — the transferable insight).

**Files:**
- Modify: `apps/web/src/shell/growth/ToolkitCards.tsx`
- Test: `apps/web/src/shell/growth/ToolkitCards.test.tsx` (existing)

**Interfaces:**
- Consumes: `CARD_REGISTRY[cardId].consolidation` (optional string) and `CARD_REGISTRY[cardId].steps[0].methodology.why` (present on every card).

- [ ] **Step 1: Write the failing test.** In `apps/web/src/shell/growth/ToolkitCards.test.tsx` add a case (mock `api.getGrowthCards` to return a card whose registry spec has `consolidation` set, e.g. `perspective-matrix`, and one without — if every seeded card has it, assert the reveal text renders for a card that has it):

```tsx
it("shows the thinking-move reveal (methodology why) for a card with consolidation", async () => {
  vi.spyOn(api, "getGrowthCards").mockResolvedValue([
    { cardId: "perspective-matrix", uses: 1, surfaces: ["project"], lastUsed: "2026-07-23T00:00:00Z" },
  ]);
  render(<ToolkitCards />);
  // The reveal is labelled and carries the spec's methodology.why.
  expect(await screen.findByText(/你练的思路/)).toBeInTheDocument();
});
```

> Adapt the import of `api` and the render harness to the existing test file's setup.

- [ ] **Step 2: Run it to verify it fails.**

Run: `cd apps/web && npm test -- ToolkitCards`
Expected: FAIL — no reveal text rendered.

- [ ] **Step 3: Enrich + render the reveal.** In `apps/web/src/shell/growth/ToolkitCards.tsx`:
  - Extend `Enriched` and `enrich`:

```ts
type Enriched = CollectedCard & { name: string; purpose: string; category: string; move: string | null };

function enrich(cards: CollectedCard[]): Enriched[] {
  const out: Enriched[] = [];
  for (const c of cards) {
    const spec = CARD_REGISTRY[c.cardId];
    if (!spec) continue;
    const move = spec.consolidation ? (spec.steps[0]?.methodology.why ?? null) : null;
    out.push({ ...c, name: spec.name, purpose: spec.purpose, category: spec.category, move });
  }
  return out;
}
```

  - In the card render (after the `usageLine` div, ~line 97), add the reveal when `c.move` is set — a plain, quiet block, no badge/score (铁律 2):

```tsx
{c.move && (
  <div style={{ marginTop: 10, paddingTop: 10, borderTop: "1px solid #F1F2F6" }}>
    <div style={{ fontSize: 11, fontWeight: 700, color: "#4C9A82", marginBottom: 4 }}>你练的思路</div>
    <div style={{ fontSize: 12, color: "#6B7384", lineHeight: 1.6 }}>{c.move}</div>
  </div>
)}
```

- [ ] **Step 4: Run tests to verify they pass.**

Run: `cd apps/web && npm test -- ToolkitCards && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 5: Commit.**

```bash
git add apps/web/src/shell/growth/ToolkitCards.tsx apps/web/src/shell/growth/ToolkitCards.test.tsx
git commit -m "feat(n3e): reveal the named thinking move on collected cards (R-9)"
```

---

## Task 6: Acceptance walk + tracker + final gates

Prove the whole Component A path end-to-end on a fresh funnel project, mark N3e done, and run every suite.

**Files:**
- Test: `apps/api/internal/api/search_plan_seed_test.go` (extend) or the existing walk test
- Modify: `docs/2026-07-20-student-platform-remaining-work.md`

- [ ] **Step 1: Write the end-to-end assertion.** Add `TestSearchPlan_SurfaceFillComplete` to `search_plan_seed_test.go`, reusing the real walk helpers (`surfaceWalkCard`, `activateWalkCard`, `submitWalkCard`, `assertCardCompleted`). Drive the full path: setup at S1 with a search plan → surface `search-plan` (returns cid + seeded anchors) → activate → fill all three columns of the FIRST seeded row (build the anchors client-side, exactly as `matrixStateToAnchors` would — one anchor per non-empty cell, `quote` = the row's direction) → submit → assert completed AND `framework_fill` written.

```go
func TestSearchPlan_SurfaceFillComplete(t *testing.T) {
	dirs := [2]string{"查NASA卫星植被数据", "查中国官方碳排放文件"}
	h, cookie, pid, pool := setupProjectAtS1WithSearchPlan(t, dirs)

	cid, _, _ := surfaceWalkCard(t, h, pool, cookie, pid, "search-plan", "我打算开始找证据了")
	activateWalkCard(t, h, cookie, pid, cid)

	// Fill one row (min_items:1) fully: one anchor per column, keyed by the
	// direction (row label). Mirrors serialize.ts matrixStateToAnchors.
	filled := []agent.Anchor{
		{Quote: dirs[0], Dimension: "evidence_type", Answer: "官方口径的排放与能源统计", Author: "student"},
		{Quote: dirs[0], Dimension: "blind_spot", Answer: "看不到独立第三方对治理成效的质疑", Author: "student"},
		{Quote: dirs[0], Dimension: "disconfirm", Answer: "只会印证官方叙述，找不到反证", Author: "student"},
	}
	submitWalkCard(t, h, cookie, pid, cid, filled)
	assertCardCompleted(t, pool, cid)

	// R-9: CompleteCard wrote the consolidation payload into framework_fill.
	row, err := sqlc.New(pool).GetCardInstance(context.Background(), mustUUID(cid))
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if s := strings.TrimSpace(string(row.FrameworkFill)); s == "" || s == "{}" || s == "null" {
		t.Fatalf("expected framework_fill written on completion, got %q", row.FrameworkFill)
	}
}
```

> `agent.Anchor`, `mustUUID`, and `context` are already used by `walk_s0_s6_test.go` in this package — add them to this file's imports. `agent.Anchor`'s field names (`Quote`/`Dimension`/`Answer`/`Author`) match `surfaceWalkCard`'s return type.

- [ ] **Step 2: Run the full acceptance test.**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestSearchPlan`
Expected: PASS — the card surfaces seeded, fills, completes, and writes `framework_fill`.

- [ ] **Step 3: Run EVERY suite (the real gate).**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./...
cd ../../apps/web && npm test && npx tsc --noEmit
cd ../packages/contracts && npm test
```
Expected: all green. The producerless-gate guard (`internal/skills`) passes (no gate item added). Card canonical↔mirror byte-identical (`make sync-cards` reproduces `search-plan.json`).

- [ ] **Step 4: Update the tracker.** In `docs/2026-07-20-student-platform-remaining-work.md`, change the N3 row / N3e note to DONE with the commit range, and add an `### N3e · DONE — merged` section summarizing: the search-plan card (matrix, seeded rows, row-noun param), the R-9 reveal in the 工具卡 tab (pure-web, all 6 cards + search-plan), and the known limits (framework_fill still unread; reveal is a record not an in-moment beat; seeded rows are a summon-time snapshot).

- [ ] **Step 5: Commit.**

```bash
git add apps/api/internal/api/search_plan_seed_test.go docs/2026-07-20-student-platform-remaining-work.md
git commit -m "test(n3e): end-to-end search-plan walk + mark N3e done"
```

---

## Notes for the executor

- **The task-1 spike is already resolved in this plan.** The spec flagged "does the seeded card render on first open?" — Task 4 answers it: `streamAction` persists-and-carries anchors on the SSE `card` frame for annotate/compare today; the new `search-plan` branch does the same, so first-surface and reload both deliver seeded rows. The `seed_rows_from` fallback (spec §2.3b) is therefore NOT needed; do not build it.
- **Component B leaves `framework_fill` written-but-unread by design** (spec §3.3). Do not add a projection field or migration to read it.
- **No new gate item, ever.** If any step tempts you to add `search-plan` to a gate's item list, stop — it is enrichment, not a producer (spec §2.5).

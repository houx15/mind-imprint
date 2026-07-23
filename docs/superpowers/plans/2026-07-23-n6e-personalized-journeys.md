# N6-E · Template-driven personalized journeys — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an LLM compose a per-student journey at project creation by *waiving* a subset of the fixed writing-project stations, honestly and re-openably, without any migration or gate-shape change.

**Architecture:** A journey is a **waived-set** (a list of contract ids) stored in the project's existing `plan` graph-node jsonb. The planner (`Route`/`AdvanceAll`) treats a waived contract as *satisfied for successors but never confirmed solid*; the studio projection renders it as a distinct `waived` rail state that doesn't block finish and is re-openable. At creation, a mid-tier LLM call reads the pasted material and returns which stations to waive (fail-safe: any failure ⇒ empty waived-set ⇒ today's full journey).

**Tech Stack:** Go (`net/http`, `pgx`, sqlc — but NO new queries here, all reads reuse `GetPlanNode`/`UpsertPlan`), React+TS+Vite (apps/web), Zod contracts (`packages/contracts`), the `gateway` LLM client.

## Global Constraints

- The client NEVER calls a model directly; the compose call is server-side in `apps/api`; keys only in server env; never in git/logs/thrown-or-rendered errors/data/eval payloads.
- **NO migration.** Waived-set rides the existing `plan` graph-node jsonb `{route, reason, waived}`. NO new sqlc query, NO hand-edit of `apps/api/internal/store/sqlc/*`.
- **NO card JSON change; NO skill JSON change** — selection needs no template edit (`packages/contracts/skills/writing-project.json` stays byte-identical).
- Every LLM call metered (档位+token+成本) via `RecordLLMCall`, **including empty/rejected output**. The compose call is **mid-tier** (`a.d.ChatResolver`), NOT flagship — assessment is the only never-downgrade flagship path.
- **DEC-3 preserved:** nothing here records a `student_written`/`human` gate item; `AdvanceAll` still never marks a non-machine item; a **waived station is never `Confirmed` solid**.
- Icons inline SVG; never import `lucide-react`.
- **NEVER `git add` a whole directory** — a pre-existing `M package.json` and untracked `docs/` files are NOT ours. Stage only the exact files each step names.
- Go tests: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...` — run **FULL packages, never `-run` subsets** for planner/gate/projection changes (the enumeration/projection tax has bitten every prior gate-touching slice). Web: `cd apps/web && npm test` + `npx tsc --noEmit`. Contracts: `cd packages/contracts && npm test` + `npx tsc --noEmit`.
- The Bash hook blocks `/dev/null` redirects — do not use `2>/dev/null` etc.

---

### Task 1: Waived-aware planner + store seam

**Files:**
- Modify: `apps/api/internal/agent/planner.go` (add `Waived` to plan body; `Route` gains a `waived` param; `AdvanceAll` treats waived as satisfied; `writePlan` preserves waived)
- Modify: `apps/api/internal/agent/loop.go:169-176` (add `LoadWaived`/`SetWaived` to the `AgentStore` interface; update the `Route(...)` call at loop.go:277)
- Modify: `apps/api/internal/agent/agentstore.go` (implement `LoadWaived`/`SetWaived` over `GetPlanNode`/`UpsertPlan`)
- Modify: `apps/api/internal/agent/loop_test.go` (extend the fake `AgentStore` with `LoadWaived`/`SetWaived`)
- Test: `apps/api/internal/agent/planner_test.go`

**Interfaces:**
- Produces:
  - `func Route(sk skills.Skill, reports map[string]GateReport, waived map[string]bool) []string` — waived nil-safe.
  - On `AgentStore`: `LoadWaived(ctx context.Context, projectID uuid.UUID) (map[string]bool, error)` and `SetWaived(ctx context.Context, projectID uuid.UUID, waived []string) error`.
  - Plan-node body canonical shape `{route []string, reason string, waived []string}`.
- Consumes: existing `GetPlanNode(ctx, projectID)` (returns the plan `sqlc.GraphNode` with `.Body`, `pgx.ErrNoRows` if none) and `UpsertPlan(ctx, projectID, body []byte)`.

- [ ] **Step 1: Write the failing tests**

Add to `apps/api/internal/agent/planner_test.go`. These use the same `skills.ByID("writing-project")` skill and the existing fake store used elsewhere in the package. Replace the fake-store construction with whatever the file's existing helper is (grep the file for how other tests build gate reports / the fake); the assertions are the point:

```go
func TestRouteSkipsWaivedAndTreatsItSatisfied(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// No gate solid anywhere: without waiving, only the root (decode_task) is reachable.
	reports := map[string]GateReport{}
	base := Route(sk, reports, nil)
	if len(base) == 0 || base[0] != "decode_task" {
		t.Fatalf("baseline route should start at decode_task, got %v", base)
	}
	// Waive decode_task + frame_question + evaluate_perspectives: the first
	// reachable KEPT station is evaluate_sources, and no waived id appears.
	waived := map[string]bool{"decode_task": true, "frame_question": true, "evaluate_perspectives": true}
	r := Route(sk, reports, waived)
	for _, id := range r {
		if waived[id] {
			t.Fatalf("route must not contain a waived contract, got %v", r)
		}
	}
	if len(r) == 0 || r[0] != "evaluate_sources" {
		t.Fatalf("route head should be evaluate_sources (first kept reachable), got %v", r)
	}
}
```

For `AdvanceAll`, add a test that waiving the front stations lets a kept middle station advance once its own gate clears, and that a waived station is never confirmed. Use the package's existing store-fake pattern for `AdvanceAll` (grep `planner_test.go` / `loop_test.go` for `AdvanceAll(` to copy the harness). The assertions:

```go
func TestAdvanceAllTreatsWaivedAsSatisfiedNeverConfirmsIt(t *testing.T) {
	// Fixture: writing-project skill; waive decode_task, frame_question,
	// evaluate_perspectives; make evaluate_sources' gate fully satisfied in the
	// graph. Expect: evaluate_sources advances; NONE of the three waived ids
	// appear in `advanced`; the fake store never received a ConfirmGate for a
	// waived contract.
	// (Build with the same fake AgentStore the package already uses; seed
	// LoadWaived to return the three-way waived map.)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/ -run 'TestRouteSkipsWaived|TestAdvanceAllTreatsWaived'`
Expected: FAIL — `Route` takes 2 args (compile error) / `LoadWaived` undefined.

- [ ] **Step 3: Add `Waived` to the plan body + `Route` signature**

In `planner.go`, add a package-level plan-body type and update `Route` + `writePlan`:

```go
// planBody is the canonical shape of the project's single `plan` graph-node.
// Route/Reason are the advisory plan; Waived is N6-E's per-project journey —
// the contract ids the composer (or the student, by re-opening) has set aside.
type planBody struct {
	Route  []string `json:"route"`
	Reason string   `json:"reason"`
	Waived []string `json:"waived,omitempty"`
}

func waivedSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}
```

Change `Route` (nil-safe `waived`):

```go
func Route(sk skills.Skill, reports map[string]GateReport, waived map[string]bool) []string {
	order, err := sk.TopoOrder()
	if err != nil {
		return nil
	}
	var route []string
	for _, id := range order {
		if reports[id].Solid || waived[id] {
			continue // finished, or waived out of this journey
		}
		reachable := true
		for _, req := range sk.Contracts[id].Requires {
			r := reports[req]
			if !(r.Solid || r.Status == "machine_clear" || waived[req]) {
				reachable = false
				break
			}
		}
		if reachable {
			route = append(route, id)
		}
	}
	return route
}
```

- [ ] **Step 4: Preserve waived in `writePlan`; thread waived into both `Route` callers**

In `writePlan` (planner.go), read the current waived-set before recomputing, and write it back:

```go
func writePlan(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, reason string) ([]string, error) {
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}
	recorded, err := deps.Store.ListGateStates(ctx, projectID)
	if err != nil {
		return nil, err
	}
	waived, err := deps.Store.LoadWaived(ctx, projectID)
	if err != nil {
		return nil, err
	}
	reports := ReconcileGates(sk, g, recorded)
	route := Route(sk, reports, waived)
	waivedList := make([]string, 0, len(waived))
	for id := range waived {
		waivedList = append(waivedList, id)
	}
	sort.Strings(waivedList) // deterministic body
	body, err := json.Marshal(planBody{Route: route, Reason: reason, Waived: waivedList})
	if err != nil {
		return nil, err
	}
	if err := deps.Store.UpsertPlan(ctx, projectID, body); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{"route": route, "reason": reason})
	if err != nil {
		return nil, err
	}
	evType := "plan_written"
	if reason != "intake" {
		evType = "plan_revised"
	}
	if err := deps.Store.AppendEvent(ctx, EventRow{
		ProjectID: projectID, Surface: "studio", Type: evType, Payload: payload,
	}); err != nil {
		return nil, err
	}
	return route, nil
}
```

Add `"sort"` to `planner.go` imports if not present. In `loop.go:277`, thread waived (loaded once — it is cheap and this is the only per-turn read added):

```go
		gateReports = ReconcileGates(*deps.Skill, g, recorded)
		waived, werr := deps.Store.LoadWaived(ctx, projectID)
		if werr != nil {
			return nil, werr
		}
		route := Route(*deps.Skill, gateReports, waived)
		checkGateCands = CheckGateCandidates(route, gateReports)
```

- [ ] **Step 5: Make `AdvanceAll` treat waived as satisfied, never confirm it**

In `AdvanceAll` (planner.go), load waived and seed the `solid` map with it:

```go
func AdvanceAll(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill) ([]string, error) {
	order, err := sk.TopoOrder()
	if err != nil {
		return nil, err
	}
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}
	recorded, err := deps.Store.ListGateStates(ctx, projectID)
	if err != nil {
		return nil, err
	}
	waived, err := deps.Store.LoadWaived(ctx, projectID)
	if err != nil {
		return nil, err
	}
	solid := make(map[string]bool, len(order))
	for _, id := range order {
		// A waived contract counts as satisfied for successors, but is NEVER
		// itself advanced/confirmed (the `if solid[id] { continue }` below skips
		// it before Advance can run) — waived ≠ done (DEC-3, 铁律 4).
		solid[id] = recorded[id].Confirmed || waived[id]
	}

	var advanced []string
	for _, id := range order {
		if solid[id] {
			continue
		}
		ready := true
		for _, req := range sk.Contracts[id].Requires {
			if !solid[req] {
				ready = false
				break
			}
		}
		if !ready {
			continue
		}
		if len(CheckGate(sk, id, g, recorded[id]).Missing) > 0 {
			continue
		}
		ok, err := Advance(ctx, deps, projectID, sk, id)
		if err != nil {
			return nil, err
		}
		if ok {
			solid[id] = true
			advanced = append(advanced, id)
		}
	}
	if len(advanced) > 0 {
		if _, err := Replan(ctx, deps, projectID, sk, "advanced"); err != nil {
			return nil, err
		}
	}
	return advanced, nil
}
```

- [ ] **Step 6: Add `LoadWaived`/`SetWaived` to the interface + sqlc impl + fake**

In `loop.go` `AgentStore` interface (near the existing `UpsertPlan(...)` line ~175), add:

```go
	LoadWaived(ctx context.Context, projectID uuid.UUID) (map[string]bool, error)
	SetWaived(ctx context.Context, projectID uuid.UUID, waived []string) error
```

In `agentstore.go`, implement both over the existing plan-node accessors (place next to `UpsertPlan`):

```go
// LoadWaived reads the project's waived-set from the plan graph-node body.
// A project with no plan node yet (fresh, pre-Replan) has an empty journey =
// full template. Never errors on absence.
func (s *sqlcAgentStore) LoadWaived(ctx context.Context, projectID uuid.UUID) (map[string]bool, error) {
	node, err := s.q.GetPlanNode(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	var b struct {
		Waived []string `json:"waived"`
	}
	if json.Unmarshal(node.Body, &b) != nil {
		return map[string]bool{}, nil // malformed body → treat as full journey
	}
	m := make(map[string]bool, len(b.Waived))
	for _, id := range b.Waived {
		m[id] = true
	}
	return m, nil
}

// SetWaived writes the waived-set into the plan node, PRESERVING any existing
// route/reason so a compose/re-open does not clobber a computed route (and a
// later Replan does not clobber the waived-set — writePlan reloads it).
func (s *sqlcAgentStore) SetWaived(ctx context.Context, projectID uuid.UUID, waived []string) error {
	body := planBody{Reason: "journey", Waived: waived}
	if node, err := s.q.GetPlanNode(ctx, projectID); err == nil {
		var cur planBody
		if json.Unmarshal(node.Body, &cur) == nil {
			body.Route = cur.Route
			if cur.Reason != "" {
				body.Reason = cur.Reason
			}
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return s.UpsertPlan(ctx, projectID, raw)
}
```

Confirm `agentstore.go` already imports `encoding/json`, `errors`, and `github.com/jackc/pgx/v5` (it does — `UpsertPlan` uses `pgx.ErrNoRows` and `errors.Is`). In `loop_test.go`, add matching methods to the fake `AgentStore` (grep the file for the struct that implements the interface — likely `fakeAgentStore` / `fakeStore`). Give it a `waived map[string]bool` field defaulting to empty, `LoadWaived` returning it, `SetWaived` storing it:

```go
func (f *fakeAgentStore) LoadWaived(_ context.Context, _ uuid.UUID) (map[string]bool, error) {
	if f.waived == nil {
		return map[string]bool{}, nil
	}
	return f.waived, nil
}
func (f *fakeAgentStore) SetWaived(_ context.Context, _ uuid.UUID, w []string) error {
	f.waived = map[string]bool{}
	for _, id := range w {
		f.waived[id] = true
	}
	return nil
}
```

(If other fakes in the package also implement `AgentStore` — grep for a compile error naming them — add the two methods to each. The compiler will list every one.)

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/`
Expected: PASS (full agent package — the `Route` signature change and interface additions ripple across the package's other tests; they must all still compile and pass).

- [ ] **Step 8: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/agent/planner.go apps/api/internal/agent/loop.go apps/api/internal/agent/agentstore.go apps/api/internal/agent/loop_test.go apps/api/internal/agent/planner_test.go
git commit -m "feat(n6e): waived-aware planner + LoadWaived/SetWaived store seam"
```

---

### Task 2: `ComposeJourney` — the LLM composition (pure of I/O beyond the gateway)

**Files:**
- Create: `apps/api/internal/agent/journey.go`
- Test: `apps/api/internal/agent/journey_test.go`

**Interfaces:**
- Consumes: `gateway.Provider`, `gateway.KeyResolver`, `gateway.Collect`, `gateway.ChatRequest`/`ChatMessage`/`RoleSystem`/`RoleUser`, `gateway.Resolved`, `gateway.ChatUsage`, `skills.Skill`, `stripFences` (already in `agent/anchors.go`, same package).
- Produces:
  - `type JourneyDecision struct { ID string `json:"id"`; Keep bool `json:"keep"`; Reason string `json:"reason"` }`
  - `type ComposeResult struct { Waived []string; Decisions []JourneyDecision; Resolved gateway.Resolved; Usage gateway.ChatUsage }`
  - `func ComposeJourney(ctx context.Context, provider gateway.Provider, resolver gateway.KeyResolver, sk skills.Skill, pasted string) ComposeResult`

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/agent/journey_test.go`. Use the package's existing fake provider (grep `internal/agent/*_test.go` for how `Generate`/coach tests build a `gateway.Provider` fake that returns canned text; reuse that helper). The behaviors:

```go
package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/skills"
)

func TestComposeJourneyWaivesUnkeptContracts(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// Fake provider returns a decision list keeping only S3..S6.
	reply := `[
	  {"id":"decode_task","keep":false,"reason":"她已解码任务"},
	  {"id":"frame_question","keep":false,"reason":"题目已定"},
	  {"id":"evaluate_perspectives","keep":false,"reason":"视角已想过"},
	  {"id":"evaluate_sources","keep":true,"reason":""},
	  {"id":"build_argument","keep":true,"reason":""},
	  {"id":"draft_polish","keep":true,"reason":""},
	  {"id":"reflect_archive","keep":true,"reason":""}]`
	provider, resolver := fakeProviderReturning(reply) // package helper
	res := ComposeJourney(context.Background(), provider, resolver, sk, "（她贴了一篇写了一半的草稿）")
	got := map[string]bool{}
	for _, id := range res.Waived {
		got[id] = true
	}
	for _, id := range []string{"decode_task", "frame_question", "evaluate_perspectives"} {
		if !got[id] {
			t.Fatalf("expected %s waived, got %v", id, res.Waived)
		}
	}
	if len(res.Waived) != 3 {
		t.Fatalf("expected exactly 3 waived, got %v", res.Waived)
	}
	if res.Resolved.Provider == "" {
		t.Fatalf("a real call happened; Resolved must be populated for metering")
	}
}

func TestComposeJourneyFailSafeToFullJourney(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	for name, reply := range map[string]string{
		"malformed":     "not json at all",
		"empty":         "[]",
		"unknown_id":    `[{"id":"not_a_contract","keep":false,"reason":"x"}]`,
		"partial_cover": `[{"id":"decode_task","keep":false,"reason":"x"}]`, // must cover ALL contracts
	} {
		t.Run(name, func(t *testing.T) {
			provider, resolver := fakeProviderReturning(reply)
			res := ComposeJourney(context.Background(), provider, resolver, sk, "题目")
			if len(res.Waived) != 0 {
				t.Fatalf("%s: fail-safe must waive nothing, got %v", name, res.Waived)
			}
			if res.Resolved.Provider == "" {
				t.Fatalf("%s: a real call happened; must still be meterable", name)
			}
		})
	}
}

func TestComposeJourneyAllKeepIsFullJourneyNotFailure(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// Well-formed, covers all, keeps all → empty waived-set, still a success.
	reply := `[
	  {"id":"decode_task","keep":true,"reason":""},
	  {"id":"frame_question","keep":true,"reason":""},
	  {"id":"evaluate_perspectives","keep":true,"reason":""},
	  {"id":"evaluate_sources","keep":true,"reason":""},
	  {"id":"build_argument","keep":true,"reason":""},
	  {"id":"draft_polish","keep":true,"reason":""},
	  {"id":"reflect_archive","keep":true,"reason":""}]`
	provider, resolver := fakeProviderReturning(reply)
	res := ComposeJourney(context.Background(), provider, resolver, sk, "题目")
	if len(res.Waived) != 0 {
		t.Fatalf("all-keep should yield empty waived-set, got %v", res.Waived)
	}
	if len(res.Decisions) != 7 {
		t.Fatalf("expected 7 decisions recorded, got %d", len(res.Decisions))
	}
}
```

If `fakeProviderReturning` does not already exist in the package's tests, add it in `journey_test.go` modelled on the existing anchor/coach provider fake (a `gateway.Provider` whose completion returns the given text, and a `gateway.KeyResolver` returning a non-empty `gateway.Resolved{Provider:"fake", ...}`). Grep `internal/agent/anchors_test.go` / `coach_test.go` for the exact fake shapes and copy them.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/ -run TestComposeJourney`
Expected: FAIL — `ComposeJourney` undefined.

- [ ] **Step 3: Implement `ComposeJourney`**

Create `apps/api/internal/agent/journey.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// JourneyDecision is the model's per-contract verdict when composing a
// student's journey (N6-E). Reason is the LLM's own rationale — recorded as a
// claim in the journey_composed event, never a measurement.
type JourneyDecision struct {
	ID     string `json:"id"`
	Keep   bool   `json:"keep"`
	Reason string `json:"reason"`
}

// ComposeResult is ComposeJourney's return: the ids to waive, the full decision
// list (for the event), and the call's Resolved/Usage for metering. Resolved
// is populated whenever a real model call happened — callers meter on
// Resolved.Provider != "" even when Waived is empty (fail-safe or all-keep).
type ComposeResult struct {
	Waived    []string
	Decisions []JourneyDecision
	Resolved  gateway.Resolved
	Usage     gateway.ChatUsage
}

// ComposeJourney asks a mid-tier model which of the template's stations this
// student can skip, given what she pasted. FAIL-SAFE: any failure — resolver
// error, provider error, unparseable reply, a decision set that does not
// EXACTLY cover the skill's contracts, or an unknown id — yields an empty
// waived-set (today's full journey). It never errors; the caller always gets a
// result it can meter. 铁律 2: waiving is only ever an offer, and re-open (Task
// 5) is the student's escape, so a permissive composer is safe.
func ComposeJourney(ctx context.Context, provider gateway.Provider, resolver gateway.KeyResolver, sk skills.Skill, pasted string) ComposeResult {
	resolved, err := resolver(ctx)
	if err != nil {
		return ComposeResult{}
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: composePrompt(sk)},
			{Role: gateway.RoleUser, Content: "学生贴进来的内容：\n" + pasted},
		},
		MaxTokens: 1200,
	}
	res, cerr := gateway.Collect(ctx, provider, resolved, req)
	if cerr != nil {
		// A resolver succeeded but the call failed before returning usage: there
		// is nothing meaningful to meter, and nothing to waive.
		return ComposeResult{Resolved: resolved}
	}
	out := ComposeResult{Resolved: resolved, Usage: res.Usage}

	var decisions []JourneyDecision
	if json.Unmarshal([]byte(stripFences(res.Text)), &decisions) != nil {
		return out // malformed → full journey
	}
	// The decision set must EXACTLY cover the skill's contracts — no missing, no
	// extra, no unknown id. Anything else is treated as a malformed reply and
	// falls back to the full journey rather than acting on a partial verdict.
	want := map[string]bool{}
	for id := range sk.Contracts {
		want[id] = false
	}
	for _, d := range decisions {
		seen, ok := want[d.ID]
		if !ok || seen {
			return out // unknown or duplicate id → full journey
		}
		want[d.ID] = true
	}
	for _, covered := range want {
		if !covered {
			return out // a contract went unmentioned → full journey
		}
	}

	var waived []string
	for _, d := range decisions {
		if !d.Keep {
			waived = append(waived, d.ID)
		}
	}
	sort.Strings(waived)
	out.Decisions = decisions
	out.Waived = waived
	return out
}

func composePrompt(sk skills.Skill) string {
	order, _ := sk.TopoOrder()
	var b strings.Builder
	b.WriteString("你是一名批判性思维写作教练。下面是一条完整的写作项目流程，共若干环节，按先后顺序排列。")
	b.WriteString("学生带着自己的任务进来，可能已经完成了其中一些环节。请根据她贴进来的内容判断：哪些环节她已经实质做过、可以跳过（keep=false），哪些还需要走一遍（keep=true）。\n")
	b.WriteString("拿不准时保留（keep=true）——跳过只是一个建议，学生随时能把某个环节重新打开。\n\n环节：\n")
	for _, id := range order {
		c := sk.Contracts[id]
		b.WriteString("- id=" + id + "：" + c.Title)
		if len(c.Produces) > 0 {
			b.WriteString("（产出：" + strings.Join(c.Produces, "、") + "）")
		}
		b.WriteString("\n")
	}
	b.WriteString("\n只输出一个 JSON 数组，必须恰好包含上面每一个 id，各一次，形如 ")
	b.WriteString(`[{"id":"环节id","keep":true,"reason":"一句话理由"}]。不要输出任何多余文字。`)
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/ -run TestComposeJourney`
Expected: PASS. Then run the full package once: `... go test ./internal/agent/` → PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/agent/journey.go apps/api/internal/agent/journey_test.go
git commit -m "feat(n6e): ComposeJourney — mid-tier journey composition with fail-safe"
```

---

### Task 3: Wire compose into `createProject`

**Files:**
- Modify: `apps/api/internal/api/project_create.go`
- Test: `apps/api/internal/api/project_create_test.go` (create if absent; grep for an existing create-project test first and extend it)

**Interfaces:**
- Consumes: `agent.ComposeJourney` (Task 2), `agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)`, `store.SetWaived` + `store.RecordLLMCall` + `store.AppendEvent` (Task 1 + existing), `a.d.Provider`, `a.d.ChatResolver`.
- Produces: after a successful create, the project's plan node carries the composed waived-set and a `journey_composed` event exists.

- [ ] **Step 1: Write the failing test**

The endpoint test suite uses a real Postgres (testcontainers) + a fake/mux provider. Grep `internal/api/*_test.go` for how other tests inject a canned provider reply (e.g. the studio-turn or spot-check tests set `a.d.Provider` / resolver to a fake). Add:

```go
func TestCreateProjectComposesJourney(t *testing.T) {
	// Arrange an API whose provider returns a decision list waiving S0..S2.
	// POST /api/v1/projects with a prompt, then GET the projection and assert
	// stations S0,S1,S2 render state "waived" and S3 renders "current".
	// Also assert a journey_composed event was appended.
}

func TestCreateProjectFullJourneyWhenComposeFails(t *testing.T) {
	// Provider returns malformed text → project created, NO station waived
	// (every S0..S3 state is done/current/locked as today), create still 201.
}
```

Model the request/GET plumbing on the existing create-project + get-projection endpoint tests in the same package (they already spin the API + parse `StudioProjection`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/ -run TestCreateProjectComposes`
Expected: FAIL — no waived stations (compose not wired).

- [ ] **Step 3: Wire compose after commit**

In `project_create.go`, after `if err := tx.Commit(...)` succeeds and before the final `WriteJSON`, add a best-effort compose block. It must NEVER fail the create (the project already exists):

```go
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// N6-E: compose a per-student journey from the pasted prompt. Best-effort —
	// the project already exists; any failure leaves the full journey (today's
	// behavior). The call is metered even when it waives nothing or is rejected.
	a.composeJourney(r.Context(), proj.ID, prompt)

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": proj.ID.String()})
}

// composeJourney runs the mid-tier journey composer and, if it yields a waived
// set, persists it + records a journey_composed event. Every branch that made
// a real model call is metered (purpose "compose_journey"). Fail-safe by
// construction: it only ever calls SetWaived with a non-empty set, so no path
// here can strand the student.
func (a *API) composeJourney(ctx context.Context, projectID uuid.UUID, prompt string) {
	sk, ok := skills.ByID("writing-project")
	if !ok {
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	res := agent.ComposeJourney(ctx, a.d.Provider, a.d.ChatResolver, sk, prompt)
	if res.Resolved.Provider != "" {
		if err := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "compose_journey",
			Resolved: res.Resolved, PromptTokens: int32(res.Usage.InputTokens), CompletionTokens: int32(res.Usage.OutputTokens),
		}); err != nil {
			slog.Warn("compose journey: record llm usage failed", "err", err, "project_id", projectID)
		}
	}
	if len(res.Waived) == 0 {
		return // full journey (fail-safe or all-keep) — nothing to persist
	}
	if err := store.SetWaived(ctx, projectID, res.Waived); err != nil {
		slog.Warn("compose journey: set waived failed", "err", err, "project_id", projectID)
		return
	}
	payload, _ := json.Marshal(map[string]any{"waived": res.Waived, "decisions": res.Decisions})
	if err := store.AppendEvent(ctx, agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "journey_composed", Payload: payload,
	}); err != nil {
		slog.Warn("compose journey: append event failed", "err", err, "project_id", projectID)
	}
}
```

Add imports to `project_create.go`: `"log/slog"`, `"github.com/google/uuid"`, `"mindimprint/api/internal/agent"`, `"mindimprint/api/internal/skills"` (keep the existing `encoding/json`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/ -run TestCreateProject`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/project_create.go apps/api/internal/api/project_create_test.go
git commit -m "feat(n6e): compose journey at project creation (best-effort, metered)"
```

---

### Task 4: Projection — `waived` rail state, head skip, waived-aware `canFinish`

**Files:**
- Modify: `apps/api/internal/studio/projection.go` (`planBody`/`planWaived`, `projectStations`, `Project`'s `canFinish`)
- Modify: `apps/api/internal/studio/dto.go:118-125` (doc the new `state` value)
- Modify: `packages/contracts/src/studioState.ts:14` (`StationState` enum add `"waived"`)
- Test: `apps/api/internal/studio/projection_test.go`

**Interfaces:**
- Consumes: the plan-node body `{route, reason, waived}` (Task 1).
- Produces: `StationDTO.State == "waived"`; `canFinish` true iff every non-waived station is done (waived-S5 arm) or `whole_draft_review` solid (default arm).

- [ ] **Step 1: Write the failing tests**

Add to `projection_test.go` (build `ProjectData` with a plan node whose body sets `waived`). Grep the file for an existing helper that assembles `ProjectData` + a plan node; reuse it.

```go
func TestProjectStations_WaivedRendersDistinctNotDone(t *testing.T) {
	// Plan node body: {"waived":["decode_task","frame_question","evaluate_perspectives"]}.
	// Assert S0,S1,S2 have state "waived" (NOT "done"), and S3 (evaluate_sources)
	// is "current" — the head skips the waived front.
}

func TestCanFinish_WaivedS6DoesNotWall(t *testing.T) {
	// Full journey EXCEPT reflect_archive waived; whole_draft_review solid.
	// Default arm still applies (draft_polish not waived) → canFinish true.
}

func TestCanFinish_WaivedDraftPolishUsesAllDoneRule(t *testing.T) {
	// draft_polish waived. canFinish true iff every non-waived station is done.
	// Case A: some earlier station still current/locked → false.
	// Case B: every non-waived station done → true.
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/studio/ -run 'TestProjectStations_Waived|TestCanFinish_Waived'`
Expected: FAIL — no `waived` state; `canFinish` ignores waiving.

- [ ] **Step 3: Add `planWaived` + `Waived` to the studio plan body**

In `projection.go`, extend the local `planBody` (line ~45) and add a reader:

```go
type planBody struct {
	Route  []string `json:"route"`
	Reason string   `json:"reason"`
	Waived []string `json:"waived"`
}

func planWaived(plan *sqlc.GraphNode) map[string]bool {
	if plan == nil {
		return map[string]bool{}
	}
	var b planBody
	if err := json.Unmarshal(plan.Body, &b); err != nil {
		return map[string]bool{}
	}
	m := make(map[string]bool, len(b.Waived))
	for _, id := range b.Waived {
		m[id] = true
	}
	return m
}
```

- [ ] **Step 4: Render `waived` in `projectStations`**

In `projectStations`, read waived and branch first; skip waived in the head fallback:

```go
	route := planRoute(d.Plan)
	waived := planWaived(d.Plan)
	head := "" // the plan's current contract
	if len(route) > 0 {
		head = route[0]
	} else {
		for _, id := range order { // no plan: first non-solid, non-waived in topo order
			if !reports[id].Solid && !waived[id] {
				head = id
				break
			}
		}
	}

	stations := make([]StationDTO, 0, len(order))
	current := ""
	for i, id := range order {
		c := sk.Contracts[id]
		rep := reports[id]
		st := StationDTO{Code: stationCode(i), Name: c.Title, View: c.View}
		switch {
		case waived[id]:
			st.State = "waived" // 已跳过 · 可恢复 — honest (not "done"), re-openable
		case rep.Solid:
			st.State = "done"
			if inRoute(route, id) {
				st.Backflow = true
			}
		case id == head:
			st.State = "current"
			current = st.Code
		default:
			st.State = "locked"
		}
		if total := gateTotal(c); total > 0 {
			st.Gate = &GateDTO{Total: total, Passed: gatePassed(rep, recorded[id])}
		}
		stations = append(stations, st)
	}
	if current == "" && len(stations) > 0 { // fully done/waived: last non-waived is current
		current = stations[len(stations)-1].Code
		for i := len(stations) - 1; i >= 0; i-- {
			if stations[i].State != "waived" {
				current = stations[i].Code
				break
			}
		}
	}
	return stations, current, nil
```

- [ ] **Step 5: Waived-aware `canFinish` in `Project`**

Replace the `canFinish` line (projection.go:475) with:

```go
	// canFinish: default arm keys on the S5 whole-draft review (unchanged
	// behavior — S6 reflection stays optional). If draft_polish itself is
	// waived, whole_draft_review can never be produced, so fall back to the
	// general rule: every non-waived station is done. A waived station never
	// walls finish (铁律 2).
	waived := planWaived(d.Plan)
	canFinish := !finished
	if canFinish {
		if waived["draft_polish"] {
			canFinish = allDoneOrWaived(stations)
		} else {
			canFinish = recordedGates["draft_polish"].Items["whole_draft_review"] == "solid"
		}
	}
```

Add the helper near `projectStations`:

```go
// allDoneOrWaived reports whether every station is either finished or waived —
// i.e. nothing is still current or locked. Used by canFinish when the writing
// station itself is waived.
func allDoneOrWaived(stations []StationDTO) bool {
	for _, st := range stations {
		if st.State != "done" && st.State != "waived" {
			return false
		}
	}
	return true
}
```

- [ ] **Step 6: Document the DTO value + add the Zod enum member**

In `dto.go`, update the `StationDTO.State` field comment to list `"waived"` alongside `"done"/"current"/"locked"` (add an inline `// "done"|"current"|"locked"|"waived"` comment on the `State` line). In `packages/contracts/src/studioState.ts:14`:

```ts
export const StationState = z.enum(["done", "current", "locked", "waived"]);
```

- [ ] **Step 7: Run tests to verify they pass**

Run the **FULL** studio package (the rail-state change ripples to enumeration/projection tests — this is the exact tax that broke prior slices):
`cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/studio/`
Then contracts: `cd packages/contracts && npm test && npx tsc --noEmit`
Expected: PASS on both. If any pre-existing projection test enumerates station states and now sees `waived`, update it deliberately (do not just bump a number — rewrite the rationale, per the N6-sweep lesson).

- [ ] **Step 8: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/studio/projection.go apps/api/internal/studio/dto.go apps/api/internal/studio/projection_test.go packages/contracts/src/studioState.ts
git commit -m "feat(n6e): projection renders waived stations + waived-aware canFinish"
```

---

### Task 5: Re-open endpoint (`POST /projects/{id}/journey/reopen/{code}`)

**Files:**
- Create: `apps/api/internal/api/journey.go`
- Modify: `apps/api/internal/api/api.go` (register the route near the other `POST /api/v1/projects/{id}/...` handlers, ~line 60)
- Test: `apps/api/internal/api/journey_test.go`

**Interfaces:**
- Consumes: `a.loadOwnedProject`, `skills.ByID`, `sk.TopoOrder`, `agent.NewSqlcAgentStore`, `store.LoadWaived`/`SetWaived`, `agent.Replan`, `agent.AgentDeps`.
- Produces: `POST /api/v1/projects/{id}/journey/reopen/{code}` — un-waives one station by rail code (`S0`..`S6`), re-plans, appends `journey_reopened`. Idempotent.

Rationale for keying by **code** not contract id (a deviation from the spec's `{contractId}`): the web rail already identifies stations by code (`S0`..`S6`), the server maps code→contract via `TopoOrder()[idx]` exactly as `projectStations` does, and this avoids both leaking internal contract ids to the client and adding a DTO field. Noted here so a reviewer expecting `{contractId}` sees the intent.

- [ ] **Step 1: Write the failing test**

```go
func TestReopenStationUnwaives(t *testing.T) {
	// Create a project; SetWaived(["decode_task","frame_question"]).
	// POST /api/v1/projects/{id}/journey/reopen/S1 → 200.
	// GET projection: S1 (frame_question) no longer "waived" (now "current" or
	// "locked" per gate state), S0 still "waived".
	// A journey_reopened event exists.
}

func TestReopenStationRejectsBadCodeAndForeignProject(t *testing.T) {
	// reopen/S9 → 400; reopen on another user's project → 404 (loadOwnedProject).
	// reopen/S0 when S0 is NOT waived → 200 no-op (idempotent), still not waived.
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/ -run TestReopenStation`
Expected: FAIL — route not registered (404 for a valid case).

- [ ] **Step 3: Implement the handler**

Create `apps/api/internal/api/journey.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
)

// reopenStation un-waives one station of a composed journey, by rail code
// (S0..S6). The journey is imposed at creation, but re-opening is the
// student's escape hatch (铁律 2): a waived station is never a permanent wall.
// Idempotent — re-opening a station that is not waived is a no-op 200.
func (a *API) reopenStation(w http.ResponseWriter, r *http.Request) {
	id, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	if len(code) != 2 || code[0] != 'S' {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_code", "环节编号不对", nil))
		return
	}
	idx, err := strconv.Atoi(code[1:])
	if err != nil || idx < 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_code", "环节编号不对", nil))
		return
	}
	sk, skOK := skills.ByID("writing-project")
	if !skOK {
		httpx.WriteError(w, r, httpx.ErrBadRequest("skill_missing", "流程未配置", nil))
		return
	}
	order, err := sk.TopoOrder()
	if err != nil || idx >= len(order) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_code", "环节编号不对", nil))
		return
	}
	contract := order[idx]

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	waived, err := store.LoadWaived(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !waived[contract] {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"reopened": false}) // idempotent no-op
		return
	}
	next := make([]string, 0, len(waived))
	for c := range waived {
		if c != contract {
			next = append(next, c)
		}
	}
	if err := store.SetWaived(r.Context(), id, next); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	deps := agent.AgentDeps{Store: store, Skill: &sk}
	if _, err := agent.Replan(r.Context(), deps, id, sk, "reopened"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	payload, _ := json.Marshal(map[string]any{"contract": contract, "code": code})
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: id, Surface: "studio", Type: "journey_reopened", Payload: payload,
	}); err != nil {
		// Telemetry only — the re-open already succeeded.
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reopened": true})
}
```

Confirm `httpx.ErrBadRequest` has signature `ErrBadRequest(code, msg string, details any)` (matches `project_create.go`'s usage) — if the third arg differs, match the local convention.

- [ ] **Step 4: Register the route**

In `api.go`, next to the other project sub-routes (~line 61):

```go
	mux.Handle("POST /api/v1/projects/{id}/journey/reopen/{code}", protected(a.reopenStation))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/ -run TestReopenStation`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/journey.go apps/api/internal/api/api.go apps/api/internal/api/journey_test.go
git commit -m "feat(n6e): re-open endpoint un-waives one station (student escape hatch)"
```

---

### Task 6: Web — render `waived` rail state + re-open action + paste-box copy

**Files:**
- Modify: `apps/web/src/studio/state.ts` (the `StationState` type already re-exports from contracts — confirm `"waived"` flows through; grep for a local literal union to widen)
- Modify: `apps/web/src/api/projects.ts` (add `reopenStation`)
- Modify: `apps/web/src/studio/StudioContainer.tsx` (add `reopenStation` to the `StudioApi` pick + an `onReopenStation` callback that POSTs then `refetchProject()`)
- Modify: `apps/web/src/studio/StudioShell.tsx` (thread `onReopenStation` to `StationRail`)
- Modify: `apps/web/src/studio/StationRail.tsx` (render the `waived` state + a 恢复 button)
- Modify: the create-funnel paste prompt copy (grep `apps/web/src` for the 新建论文 / paste-prompt placeholder text; likely in `StudioContainer.tsx` or a `NewProject*`/directory component)
- Test: `apps/web/test/studio/StationRail.test.tsx` (create; the repo's tests live under `apps/web/test/**` and import via the `@/` alias — Task 9 of the N6 sweep)

**Interfaces:**
- Consumes: `StationState` now includes `"waived"` (Task 4); `POST /projects/{id}/journey/reopen/{code}` (Task 5).
- Produces: `api.reopenStation(projectId, code)`; `StationRailProps.onReopen(code)`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/test/studio/StationRail.test.tsx`:

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { StationRail } from "@/studio/StationRail";
import type { Station } from "@/studio/state";

const stations: Station[] = [
  { code: "S0", name: "任务解码", view: "评估", state: "waived" },
  { code: "S3", name: "信源评估", view: "素材", state: "current" },
] as Station[];

describe("StationRail waived", () => {
  it("renders a waived station as 已跳过 · 可恢复 and calls onReopen", () => {
    const onReopen = vi.fn();
    render(<StationRail stations={stations} active="S3" focus={false} onSelect={() => {}} onReopen={onReopen} />);
    expect(screen.getByText(/已跳过/)).toBeInTheDocument();
    fireEvent.click(screen.getByText(/恢复/));
    expect(onReopen).toHaveBeenCalledWith("S0");
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/web && npx vitest run test/studio/StationRail.test.tsx`
Expected: FAIL — `onReopen` prop unknown / no 恢复 control.

- [ ] **Step 3: Widen the `StationState` type**

In `apps/web/src/studio/state.ts`, ensure the station state union includes `"waived"`. If it re-exports `StationState` from `packages/contracts`, the Zod change (Task 4) already covers it — verify `npx tsc --noEmit` is clean after Task 4's contract rebuild. If there is a hand-written literal union locally, add `| "waived"`.

- [ ] **Step 4: Render the waived state + 恢复 button in `StationRail`**

Add `onReopen` to props and a waived branch. In `StationRailProps`:

```tsx
export type StationRailProps = {
  stations: Station[];
  active: StationCode;
  focus: boolean;
  onSelect: (code: StationCode) => void;
  onReopen: (code: StationCode) => void;
};
```

Add a `waived` const alongside the others (line ~94) and render its chip inside the `expanded` block, after the `{locked && <LockIcon />}` line and the backflow block:

```tsx
          const waived = st.state === "waived";
```

Adjust the number-bubble/label styling so a waived station reads muted (not green/done): include `waived` in the muted-label test —

```tsx
                    <span style={{ fontSize: 13.5, fontWeight: cur || isActive ? 800 : 700, color: locked || waived ? "#AEB4C2" : "#1C2333" }}>
```

and after the backflow block add:

```tsx
                  {waived && (
                    <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 7 }}>
                      <span style={{ fontSize: 10.5, fontWeight: 700, color: "#9AA1B0" }}>已跳过</span>
                      <button
                        type="button"
                        onClick={(e) => { e.stopPropagation(); onReopen(st.code); }}
                        style={{
                          fontSize: 10.5, fontWeight: 700, color: "#2A3B7A",
                          background: "#EDEFF9", border: "none", borderRadius: 999,
                          padding: "3px 9px", cursor: "pointer",
                        }}
                      >
                        恢复
                      </button>
                    </div>
                  )}
```

(`e.stopPropagation()` keeps the row's `onSelect` preview from also firing.) Also make the connector line neutral for waived (it currently greens only on `done` — leave as-is; waived falls to the neutral `#EAECF2`, which is correct).

- [ ] **Step 5: Add the API client method + wire the callback**

In `apps/web/src/api/projects.ts`, add (match the file's existing `post`/`request` helper style):

```ts
export async function reopenStation(projectId: string, code: string): Promise<void> {
  await apiFetch(`/api/v1/projects/${projectId}/journey/reopen/${code}`, { method: "POST" });
}
```

Wire it into the `api` object the studio uses (grep `projects.ts` for how `finishProject`/`attestGate` are exported into the `api` namespace and mirror it). In `StudioContainer.tsx`, add `"reopenStation"` to the `StudioApi` Pick union (line ~18), and add a callback passed down to the shell:

```tsx
  const onReopenStation = async (code: StationCode) => {
    if (!openId) return;
    try {
      await api.reopenStation(openId, code);
    } catch (e) {
      // best-effort; a failed reopen leaves the station waived (no worse state)
    }
    void refetchProject();
  };
```

Add `onReopenStation` to the `callbacks` object the shell receives (grep `StudioContainer.tsx` for where `onSelectStation` is placed in that object and add alongside).

- [ ] **Step 6: Thread through `StudioShell`**

In `StudioShell.tsx:211`, pass `onReopen`:

```tsx
        <StationRail stations={state.stations} active={state.activeStation} focus={state.focusMode} onSelect={callbacks.onSelectStation} onReopen={callbacks.onReopenStation} />
```

Add `onReopenStation` to the shell's `callbacks` prop type (grep for where `onSelectStation` is typed in the shell's props and add `onReopenStation: (code: StationCode) => void;`).

- [ ] **Step 7: Paste-box copy invites existing work**

Grep `apps/web/src` for the 新建论文 paste placeholder / prompt label (the create funnel). Update the placeholder/help copy so it invites pasting existing work, e.g. change a placeholder like `贴上任务要求…` to `贴上任务要求；如果你已经有思路、资料或初稿，也一起贴进来——我会据此帮你规划环节。`. Keep it one field; no new inputs.

- [ ] **Step 8: Run tests + typecheck**

Run: `cd apps/web && npm test && npx tsc --noEmit`
Expected: PASS (full web suite — the `StationState` widening + any fixture using stations must still typecheck; a `waived` fixture is optional but the union must accept it).

- [ ] **Step 9: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/src/studio/state.ts apps/web/src/api/projects.ts apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/StudioShell.tsx apps/web/src/studio/StationRail.tsx apps/web/test/studio/StationRail.test.tsx
# plus the create-funnel copy file the grep in Step 7 identified — add it explicitly by path
git commit -m "feat(n6e): web renders waived stations + 恢复 re-open + paste-box copy"
```

---

### Task 7: End-to-end acceptance walk + tracker/memory update

**Files:**
- Create: `apps/api/internal/api/journey_walk_test.go`
- Modify: `docs/2026-07-20-student-platform-remaining-work.md` (mark N6-E DONE; add an N6-E section)
- Modify: `/Users/houyuxin/.claude/projects/-Users-houyuxin-08Coding-mind-imprint/memory/student-platform-finishing.md` + `MEMORY.md` (record N6-E DONE)

**Interfaces:**
- Consumes: everything above, driven through **client-reachable HTTP endpoints only** (the N3d/N3f discipline — never reach into gate internals or POST a shape the web client can't produce).

- [ ] **Step 1: Write the acceptance test**

Model it on `apps/api/internal/api/walk_s0_s6_test.go` (the existing full-walk test). The new walk:

```go
func TestJourneyWalk_WaivedFrontCompletesAndFinishes(t *testing.T) {
	// 1. Arrange provider to waive decode_task, frame_question, evaluate_perspectives.
	// 2. POST /projects → project with S0..S2 waived (assert via GET projection).
	// 3. Walk S3→S6 to completion on client-reachable endpoints only
	//    (materials/open, spot-check, card submits, order-review, sign, etc. —
	//    copy the S3+ portion of walk_s0_s6_test.go).
	// 4. Assert CanFinish true and POST finish succeeds — with three stations
	//    never done, proving waived counts as satisfied for finish.
}

func TestJourneyWalk_ReopenReintroducesStation(t *testing.T) {
	// Same waived front; POST journey/reopen/S1; GET projection → S1 no longer
	// "waived" (it re-enters the route as current/locked per gate state).
}
```

- [ ] **Step 2: Run it to verify it fails, then passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/ -run TestJourneyWalk`
Expected: FAIL first only if a wiring gap remains; otherwise PASS (all machinery landed in Tasks 1–5). If it fails, the failure is the real integration signal — fix at root cause, do not weaken the assertion.

- [ ] **Step 3: Full-suite gate**

Run all three suites (the enumeration/projection tax means only the full runs are trustworthy):
```
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...
cd apps/web && npm test && npx tsc --noEmit
cd packages/contracts && npm test && npx tsc --noEmit
```
Expected: all green.

- [ ] **Step 4: Update the tracker + memory**

In `docs/2026-07-20-student-platform-remaining-work.md`: flip the N6 row's N6-E mention to DONE, and add an `### N6-E · DONE` section summarizing: journey = waived-set over the fixed template; migration-free (plan graph-node jsonb); compose at creation (mid-tier, metered, fail-safe to full journey); waived = honest+re-openable, counts-as-satisfied but never confirmed; canFinish two-arm rule; reopen by rail code. Record known limits (bare-prompt ⇒ full journey; waived upstream leaves output un-produced; reasons are unverified claims).

In memory `student-platform-finishing.md` + the `MEMORY.md` pointer line: append N6-E DONE with the commit range.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/journey_walk_test.go docs/2026-07-20-student-platform-remaining-work.md
git commit -m "test(n6e): S0-S6 walk with a waived front + reopen; tracker DONE"
```

(Memory files live outside the repo — write them with the Write tool, not `git add`.)

---

## Self-Review

**1. Spec coverage:**
- Decision 1 (selection only, no reorder, no new gates) → Tasks 1/2 (waived-set model; `ComposeJourney` only chooses keep/waive). ✓
- Decision 2 (compose + impose at creation, stable) → Task 3 (compose in `createProject`; no re-compose path). ✓
- Decision 3 (infer from pasted material; paste-box copy) → Tasks 2/3 (input = `prompt`) + Task 6 Step 7 (copy). ✓
- Decision 4 (waived = visible + re-openable, doesn't block finish, ≠ done) → Task 4 (`waived` state, `canFinish`) + Task 5 (reopen). ✓
- Decision 5 (no mandatory spine) → Task 2 composes over all contracts with no floor. ✓
- Storage migration-free via plan node → Task 1 (`LoadWaived`/`SetWaived` over `GetPlanNode`/`UpsertPlan`). ✓
- `writePlan` preserves waived → Task 1 Step 4. ✓
- Machine changes (Route/AdvanceAll/projection/reopen) → Tasks 1/4/5. ✓
- Metering incl. rejected/empty; mid-tier not flagship → Task 3. ✓
- DEC-3 / waived never confirmed → Task 1 Step 5 (`if solid[id] { continue }` before Advance). ✓
- Testing strategy (planner/projection/compose units, acceptance, full-suite) → Tasks 1/2/4/7. ✓

**2. Placeholder scan:** No TBD/TODO. Every code step shows real code. The two "grep for the existing fake/helper" instructions are unavoidable (the fakes' exact names live in files I did not fully read); each names the exact file and what to copy, and the compiler enumerates any missed implementor. Not a logic placeholder.

**3. Type consistency:** `Route(sk, reports, waived map[string]bool)` used identically at both callers (planner.go, loop.go) and in tests. `planBody{Route,Reason,Waived}` identical in agent (`planner.go`) and studio (`projection.go`) — deliberately mirrored, matching the pre-existing duplication. `LoadWaived`/`SetWaived`/`ComposeJourney`/`ComposeResult`/`JourneyDecision` names identical across producer and consumers. `StationState` gains `"waived"` in Zod + Go + web in lockstep. Reopen keyed by `code` (S0..S6) consistently in Task 5 (handler) and Task 6 (client/rail).

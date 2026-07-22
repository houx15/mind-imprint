# N3f · Finish the Walk (S3 → S6) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the six producerless gate items at S3/S4/S6 real producers, so a student who creates a project through the funnel can walk it to a finished state.

**Architecture:** Three `student_written` items are attested by the endpoint that already persists the writing (N3d's attestation-by-endpoint). Two `human` items become student-ordered AI spot-checks modelled on 整稿体检, made idempotent by a fingerprint over what the check reads. S6's `human` item becomes a signed declaration projected from data that already exists — no model call. Plus one shared-infra bug fix and a guard test that fails the suite for any future producerless gate item.

**Tech Stack:** Go (`net/http`, `pgx`, `sqlc`), React + Vite + TypeScript, Zod contracts, PostgreSQL. No new dependencies.

## Global Constraints

- **No migration.** Every seam exists: `intervention.type` is free text, `intervention.anchor` is jsonb, `graph_node.type` is free text. If you believe you need a migration, stop and escalate.
- **The client never calls a model.** All LLM calls go through `apps/api`; keys live only in server env and must never enter git, logs, thrown or rendered errors, data storage, or evaluation payloads.
- **Card JSON is authored only in `packages/contracts/cards/`**, mirrored by `cd apps/api && make sync-cards`. **Never hand-edit `apps/api/internal/cards/specs/`.** This slice edits no card JSON at all.
- **Never hand-edit `apps/api/internal/store/sqlc/*`.** Run `make sqlc` from `apps/api` (needs `CGO_ENABLED=0` on macOS). This slice adds no queries, so `make sqlc` should be a no-op.
- **铁律 1 (AI 克制):** the AI never authors student content. A spot-check's `fix` is direction, never a paste-ready sentence.
- **铁律 2 (不操纵):** no score, no grade, no celebration, no streak, no badge. An offer is never a wall — neither spot-check blocks her from working, only from marking a station finished.
- **铁律 3 (一次只问一个)** and **铁律 4 (过程即数据):** every order, item, disposition and signature lands in the event stream.
- **RL-1:** a review writes only intervention rows, never prose into the student's draft.
- **RL-5:** gates check that she *did* the work, never how well.
- **Every LLM call is metered** (档位 + token + 成本) via `RecordLLMCall`, **including calls whose output is rejected by enforcement.**
- **Assessment runs flagship and is never downgraded.** Spot-checks use the same resolver as `orderReview` (`a.d.ChatResolver`).
- **Icons are inline SVG.** Never import `lucide-react`.
- **Go tests:** from `apps/api`, `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`. Run **full packages, never `-run` subsets**, for any change touching cards, gates, or projections.
- **Web tests:** from `apps/web`, `npm test` and `npx tsc --noEmit`. **Contracts tests:** from `packages/contracts`, `npm test` (tests live in `test/`, not `src/`).
- **Never `git add` a whole directory.** The pre-existing `M package.json` and the untracked files under `docs/` and the repo root are NOT part of this branch. Add named files only.
- Binding design doc for all UI: `docs/design/思维印记_工作区.dc.html`.
- Spec: `docs/superpowers/specs/2026-07-22-n3f-finish-the-walk-design.md`.

## File Structure

**Create:**
- `apps/api/internal/api/attest.go` — the three S3/S4 attestations: pure predicates + one API method.
- `apps/api/internal/api/attest_test.go`
- `apps/api/internal/agent/spotcheck.go` — `SpotCheckItem`, `ProposeSpotCheck`, the two posture prompts, `SpotCheckFingerprint`.
- `apps/api/internal/agent/spotcheck_test.go`
- `apps/api/internal/api/spotcheck.go` — the SSE endpoint, input builders, replay, persistence, gate flip.
- `apps/api/internal/api/spotcheck_test.go`
- `apps/api/internal/api/declaration.go` — the four counters + the sign endpoint.
- `apps/api/internal/api/declaration_test.go`
- `apps/web/src/studio/WorkOrder.tsx` — the shared work-order item extracted from `WritingView`.
- `apps/web/src/studio/WorkOrder.test.tsx`
- `apps/web/src/studio/SpotCheckPanel.tsx` — the order button + item list, used at S3 and S4.
- `apps/web/src/studio/SpotCheckPanel.test.tsx`
- `apps/api/internal/skills/producers_test.go` — the producerless-gate guard.
- `apps/api/internal/api/walk_s0_s6_test.go` — the fresh-project acceptance walk.

**Modify:**
- `apps/api/internal/api/projectcards.go` — call the attestation before `advanceGates`.
- `apps/api/internal/api/api.go` — two new routes.
- `apps/api/internal/agent/anchors.go`, `apps/api/internal/agent/prompt.go` — the L1 block-id fix.
- `apps/api/internal/studio/projection.go`, `apps/api/internal/studio/dto.go` — `spotChecks` + `declaration` projections.
- `packages/contracts/src/studioState.ts` — `SpotCheckItem`, `SpotCheckFx`, `DeclarationFx`, submit bodies.
- `apps/web/src/studio/views/WritingView.tsx` — use the extracted component.
- `apps/web/src/studio/views/ReviewView.tsx` — the declaration block.
- `apps/web/src/studio/ViewFrame.tsx`, `apps/web/src/studio/StudioContainer.tsx`, `apps/web/src/studio/state.ts`, `apps/web/src/api/writing.ts`, `apps/web/src/api/index.ts` — wiring.

---

## Task 1: Attest the three S3/S4 `student_written` items

**Files:**
- Create: `apps/api/internal/api/attest.go`
- Create: `apps/api/internal/api/attest_test.go`
- Modify: `apps/api/internal/api/projectcards.go:334`

**Interfaces:**
- Consumes: `agent.NewSqlcAgentStore`, `store.ListGateStates`, `store.UpsertGateState` (the merge pattern in `api/advance.go:54–69`).
- Produces: `func (a *API) attestS3S4(ctx context.Context, projectID uuid.UUID)` — best-effort, logs on error, returns nothing. Called by later tasks' tests as the producer of `source_risk_notes`, `warrants`, `steelman`.

**Context you need:** a material is "evaluated" when an `evaluated-as` edge runs from the material to an evidence node whose body carries `source_quality.risk_note` (`studio/projection.go:770–778, 872–875`). A Toulmin slot's node **type equals the slot id** (`claim`/`warrant`/`evidence`/`counter`/`concession`) and counts as written only when `body.text` is non-blank after trimming (`studio/projection.go:692–703`). The `counter` slot is 反方 · 钢人 — the steelman. The standalone `steelman` card has **no `graph_effects`** and mints no node, so a completed `steelman` card instance is the second accepted producer for that item.

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/api/attest_test.go`:

```go
package api

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// If a helper of this name already exists in the api package's tests, delete
// this one and use the existing one — do not define a second.
func pgID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

func TestAllArticlesHaveRiskNote(t *testing.T) {
	matA, matB := uuid.New(), uuid.New()
	nodeA, nodeB := uuid.New(), uuid.New()

	article := func(id uuid.UUID) sqlc.Material { return sqlc.Material{ID: pgID(id), Kind: "article"} }
	evidence := func(id uuid.UUID, body string) sqlc.GraphNode {
		return sqlc.GraphNode{ID: pgID(id), Type: "evidence", Body: []byte(body)}
	}
	evaluatedAs := func(m, n uuid.UUID) sqlc.GraphEdge {
		return sqlc.GraphEdge{Type: "evaluated-as", FromKind: "material", FromID: pgID(m), ToKind: "graph_node", ToID: pgID(n)}
	}
	withNote := `{"source_quality":{"risk_note":"入口来源，不能直接引用。"}}`
	blankNote := `{"source_quality":{"risk_note":"   "}}`

	tests := []struct {
		name      string
		materials []sqlc.Material
		nodes     []sqlc.GraphNode
		edges     []sqlc.GraphEdge
		want      bool
	}{
		{
			name:      "every article has a risk note",
			materials: []sqlc.Material{article(matA), article(matB)},
			nodes:     []sqlc.GraphNode{evidence(nodeA, withNote), evidence(nodeB, withNote)},
			edges:     []sqlc.GraphEdge{evaluatedAs(matA, nodeA), evaluatedAs(matB, nodeB)},
			want:      true,
		},
		{
			name:      "one article still unevaluated",
			materials: []sqlc.Material{article(matA), article(matB)},
			nodes:     []sqlc.GraphNode{evidence(nodeA, withNote)},
			edges:     []sqlc.GraphEdge{evaluatedAs(matA, nodeA)},
			want:      false,
		},
		{
			name:      "whitespace-only risk note does not count",
			materials: []sqlc.Material{article(matA)},
			nodes:     []sqlc.GraphNode{evidence(nodeA, blankNote)},
			edges:     []sqlc.GraphEdge{evaluatedAs(matA, nodeA)},
			want:      false,
		},
		{
			// Unlike every_source_evaluated (gate.go:54), which passes
			// vacuously by design, an empty dossier has NOT done the S3 work.
			name:      "no article materials at all is not satisfied",
			materials: []sqlc.Material{{ID: pgID(matA), Kind: "draft"}},
			want:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := allArticlesHaveRiskNote(tc.materials, tc.nodes, tc.edges); got != tc.want {
				t.Errorf("allArticlesHaveRiskNote = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSlotHasText(t *testing.T) {
	nodes := []sqlc.GraphNode{
		{ID: pgID(uuid.New()), Type: "warrant", Body: []byte(`{"text":"遥感叶面积上升不等于生态质量上升，中间这一步需要说明。"}`)},
		{ID: pgID(uuid.New()), Type: "counter", Body: []byte(`{"text":"   "}`)},
	}
	if !slotHasText(nodes, "warrant") {
		t.Error("warrant slot with text should count as written")
	}
	if slotHasText(nodes, "counter") {
		t.Error("whitespace-only counter slot must not count as written")
	}
	if slotHasText(nodes, "concession") {
		t.Error("absent slot must not count as written")
	}
}

func TestSteelmanCardCounts(t *testing.T) {
	completed := []sqlc.CardInstance{{CardID: "steelman", Status: "completed"}}
	skipped := []sqlc.CardInstance{{CardID: "steelman", Status: "skipped"}}
	if !hasCompletedSteelmanCard(completed) {
		t.Error("a completed steelman card is a producer for the steelman item")
	}
	if hasCompletedSteelmanCard(skipped) {
		t.Error("a skipped steelman card must not satisfy the steelman item")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ 2>&1 | head -20
```

Expected: FAIL — `undefined: allArticlesHaveRiskNote`, `undefined: slotHasText`, `undefined: hasCompletedSteelmanCard`.

- [ ] **Step 3: Write `apps/api/internal/api/attest.go`**

```go
package api

// attest.go — N3f Task 1. Three S3/S4 student_written gate items whose
// WRITING already had a producer but whose ATTESTATION did not, so no real
// student could ever clear evaluate_sources or build_argument. Same
// attestation-by-endpoint pattern as N3d (advance.go's attestReconLogged):
// the endpoint that persists the writing records the item — no checkbox.
//
// RL-5: these check that she DID the work, never how well. There is
// deliberately no length floor — the CRAAP card's own
// field_written_by(risk_note, student) completion predicate is already the
// floor, and a second, different threshold on the same text would let a card
// complete while its gate item stayed missing with nothing on screen
// explaining the discrepancy.

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// riskNoteText reads body.source_quality.risk_note off a minted evidence node.
// Mirrors studio/projection.go's riskNote — kept local because the projection
// one is unexported and this package must not depend on projection internals.
func riskNoteText(body []byte) string {
	var b struct {
		SourceQuality map[string]string `json:"source_quality"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return ""
	}
	return b.SourceQuality["risk_note"]
}

// allArticlesHaveRiskNote reports whether EVERY kind:"article" material has an
// evaluated-as edge to a node carrying a non-blank risk_note.
//
// Unlike the every_source_evaluated machine predicate (agent/gate.go:54),
// which passes vacuously over an empty material list by design, this returns
// false when the project has no article materials: an empty dossier has not
// done S3's work, and marking the item solid there would let a student clear
// 信源评估 having evaluated nothing.
func allArticlesHaveRiskNote(materials []sqlc.Material, nodes []sqlc.GraphNode, edges []sqlc.GraphEdge) bool {
	byID := make(map[string]sqlc.GraphNode, len(nodes))
	for _, n := range nodes {
		byID[uuid.UUID(n.ID.Bytes).String()] = n
	}
	evidence := map[string]sqlc.GraphNode{}
	for _, e := range edges {
		if e.Type != "evaluated-as" || e.FromKind != "material" {
			continue
		}
		if n, ok := byID[uuid.UUID(e.ToID.Bytes).String()]; ok {
			evidence[uuid.UUID(e.FromID.Bytes).String()] = n
		}
	}
	articles := 0
	for _, m := range materials {
		if m.Kind != "article" {
			continue
		}
		articles++
		n, ok := evidence[uuid.UUID(m.ID.Bytes).String()]
		if !ok || strings.TrimSpace(riskNoteText(n.Body)) == "" {
			return false
		}
	}
	return articles > 0
}

// slotHasText reports whether a Toulmin slot's node exists with non-blank
// body.text. Slot id == node type, and "written" means the same thing the 结构
// pane means by "done" (studio/projection.go:692–703) — one definition of
// written, so the gate and the screen can never disagree.
func slotHasText(nodes []sqlc.GraphNode, nodeType string) bool {
	for _, n := range nodes {
		if n.Type != nodeType {
			continue
		}
		var b struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(n.Body, &b) == nil && strings.TrimSpace(b.Text) != "" {
			return true
		}
	}
	return false
}

// hasCompletedSteelmanCard reports whether the standalone 钢人卡 was completed.
// That card has no graph_effects and mints no node, so without this a student
// who did the steelman card and not the Toulmin counter slot would write a
// steelman and get no credit for it. A SKIPPED card never counts.
func hasCompletedSteelmanCard(cards []sqlc.CardInstance) bool {
	for _, c := range cards {
		if c.CardID == "steelman" && c.Status == "completed" {
			return true
		}
	}
	return false
}

// attestS3S4 recomputes the three S3/S4 student_written items from graph state
// and records them. Like advanceGates, gate state is DERIVED and fully
// recomputable, so a failure here must never fail the student's write — the
// next gate-affecting write recomputes it from scratch. Hence: logs, returns
// nothing.
//
// Both sets AND clears each item (attestGate's set/delete pattern): deleting a
// material or blanking a slot must be able to re-open the gate. A gate that
// can only ever close is a gate that lies after a deletion.
func (a *API) attestS3S4(ctx context.Context, projectID uuid.UUID) {
	materials, err := a.d.Queries.ListMaterialsByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		slog.Warn("attest s3s4: list materials", "err", err, "project_id", projectID.String())
		return
	}
	nodes, err := a.d.Queries.ListGraphNodesByProject(ctx, projectID)
	if err != nil {
		slog.Warn("attest s3s4: list nodes", "err", err, "project_id", projectID.String())
		return
	}
	edges, err := a.d.Queries.ListGraphEdgesByProject(ctx, projectID)
	if err != nil {
		slog.Warn("attest s3s4: list edges", "err", err, "project_id", projectID.String())
		return
	}
	cards, err := a.d.Queries.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		slog.Warn("attest s3s4: list cards", "err", err, "project_id", projectID.String())
		return
	}

	want := map[string]map[string]bool{
		"evaluate_sources": {
			"source_risk_notes": allArticlesHaveRiskNote(materials, nodes, edges),
		},
		"build_argument": {
			"warrants": slotHasText(nodes, "warrant"),
			"steelman": slotHasText(nodes, "counter") || hasCompletedSteelmanCard(cards),
		},
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	recorded, err := store.ListGateStates(ctx, projectID)
	if err != nil {
		slog.Warn("attest s3s4: list gate states", "err", err, "project_id", projectID.String())
		return
	}
	for contract, items := range want {
		rec := recorded[contract]
		if rec.Items == nil {
			rec.Items = map[string]string{}
		}
		for name, ok := range items {
			if ok {
				rec.Items[name] = "solid"
			} else {
				delete(rec.Items, name)
			}
		}
		if err := store.UpsertGateState(ctx, projectID, contract, rec); err != nil {
			slog.Warn("attest s3s4: upsert gate state", "err", err, "contract", contract, "project_id", projectID.String())
		}
	}
}
```

Add `"github.com/jackc/pgx/v5/pgtype"` to the import block. Verify the exact
names and parameter types of `ListGraphEdgesByProject` and
`ListCardInstancesByProject` in `apps/api/internal/store/sqlc/` before writing
the calls — some take `pgtype.UUID` and some take `uuid.UUID`; match what is
generated rather than assuming.

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ 2>&1 | tail -5
```

Expected: PASS.

- [ ] **Step 5: Wire it into the card-submit path**

In `apps/api/internal/api/projectcards.go`, immediately before the existing
`a.advanceGates(r.Context(), projectID)` at line 334:

```go
	// N3f: the three S3/S4 student_written items are attested from graph
	// state, so they must be recomputed BEFORE advanceGates decides which
	// contracts are now finished — otherwise the submit that completes the
	// last risk_note attests it but does not advance until the NEXT write.
	a.attestS3S4(r.Context(), projectID)
	a.advanceGates(r.Context(), projectID)
```

- [ ] **Step 6: Run the full api and agent packages**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ ./internal/agent/ 2>&1 | tail -10
```

Expected: `ok` for both.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/attest.go apps/api/internal/api/attest_test.go apps/api/internal/api/projectcards.go
git commit -m "feat(n3f): attest source_risk_notes / warrants / steelman from graph state"
```

---

## Task 2: `SpotCheckItem` + `ProposeSpotCheck`

**Files:**
- Create: `apps/api/internal/agent/spotcheck.go`
- Create: `apps/api/internal/agent/spotcheck_test.go`

**Interfaces:**
- Consumes: `gateway.Collect`, `gateway.ChatRequest/ChatMessage/RoleSystem/RoleUser`, `enforcement.BannedPhrasing` — all used exactly as `agent/review.go:126–176` uses them.
- Produces:
  - `type SpotCheckItem struct { TargetID, TargetName, Evidence, Missing, Fix string }` with json tags `target_id`, `target_name`, `evidence`, `missing`, `fix`.
  - `type SpotCheckTarget struct { ID, Name, Detail string }`
  - `func ProposeSpotCheck(ctx context.Context, prov gateway.Provider, r gateway.Resolved, station string, targets []SpotCheckTarget) ([]SpotCheckItem, gateway.ChatUsage, error)`
  - `const SpotCheckSources = "evaluate_sources"` and `const SpotCheckArgument = "build_argument"`

**Design constraints, verbatim from the spec §5.3:** no `points` field (0457
readiness data, meaningless here), **no band and no verdict** — `MaterialDTO`
deliberately carries no credibility field because 「a 可信/存疑 judgment has no
honest producer and would have to be fabricated」 (`studio/dto.go:200–205`), and
a band would reintroduce exactly that judgment through a side door.
`TargetName` is resolved server-side from the id, never model-authored.

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/agent/spotcheck_test.go`:

```go
package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func spotTargets() []SpotCheckTarget {
	return []SpotCheckTarget{
		{ID: "m1", Name: "NASA 全球变绿观测", Detail: "档位：一手数据；作用与风险：仅是遥感叶面积。"},
		{ID: "m2", Name: "BP 世界能源统计", Detail: "档位：机构报告；作用与风险：（未写）"},
	}
}

func TestProposeSpotCheckResolvesNamesServerSide(t *testing.T) {
	prov := &stubProvider{text: `[
	  {"target_id":"m1","target_name":"模型瞎编的名字","evidence":"写了作用与风险","missing":"没说清遥感口径的局限","fix":"补一句这条数据不能回答什么"},
	  {"target_id":"m2","evidence":"档位已定","missing":"还没写作用与风险","fix":"写这条在论证里承担什么"}
	]`}
	items, _, err := ProposeSpotCheck(context.Background(), prov, gateway.Resolved{Provider: "stub"}, SpotCheckSources, spotTargets())
	if err != nil {
		t.Fatalf("ProposeSpotCheck: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].TargetName != "NASA 全球变绿观测" {
		t.Errorf("TargetName = %q — must be resolved server-side, never taken from the model", items[0].TargetName)
	}
	if items[1].TargetName != "BP 世界能源统计" {
		t.Errorf("TargetName = %q, want the server-resolved name", items[1].TargetName)
	}
}

func TestProposeSpotCheckDropsUnknownTargets(t *testing.T) {
	prov := &stubProvider{text: `[
	  {"target_id":"m1","evidence":"e","missing":"m","fix":"f"},
	  {"target_id":"ghost","evidence":"e","missing":"m","fix":"f"}
	]`}
	items, _, err := ProposeSpotCheck(context.Background(), prov, gateway.Resolved{Provider: "stub"}, SpotCheckSources, spotTargets())
	if err != nil {
		t.Fatalf("ProposeSpotCheck: %v", err)
	}
	if len(items) != 1 || items[0].TargetID != "m1" {
		t.Fatalf("items = %+v, want only the known target", items)
	}
}

func TestProposeSpotCheckRejectsWholeOrderOnBannedPhrasing(t *testing.T) {
	// One violating field must reject the WHOLE order — the same
	// all-or-nothing discipline ProposeReview uses (review.go:162-165).
	prov := &stubProvider{text: `[
	  {"target_id":"m1","evidence":"ok","missing":"ok","fix":"ok"},
	  {"target_id":"m2","evidence":"ok","missing":"ok","fix":"` + bannedSampleForTest() + `"}
	]`}
	items, usage, err := ProposeSpotCheck(context.Background(), prov, gateway.Resolved{Provider: "stub"}, SpotCheckSources, spotTargets())
	if err == nil {
		t.Fatal("expected the whole order to be rejected")
	}
	if items != nil {
		t.Error("no items may be returned when enforcement rejects the order")
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		t.Error("usage must be populated even on rejection — a rejected call still cost money")
	}
}

func TestSpotCheckPromptsDifferByStation(t *testing.T) {
	src := spotCheckSystemPrompt(SpotCheckSources)
	arg := spotCheckSystemPrompt(SpotCheckArgument)
	if src == arg {
		t.Fatal("the two stations must use different postures")
	}
	for _, p := range []string{src, arg} {
		if !strings.Contains(p, "绝不替学生改写句子") {
			t.Error("every posture must carry the RL-1 iron rule verbatim")
		}
	}
	if strings.Contains(src, "分点") || strings.Contains(arg, "分点") {
		t.Error("spot-check postures must not ask for points — that is 0457 readiness data")
	}
}
```

`stubProvider` and `bannedSampleForTest` must exist in the agent package's
test helpers. Check `apps/api/internal/agent/review_test.go` for the existing
stub provider first and reuse it verbatim rather than defining a second one;
if the existing stub has a different name, use that name. For
`bannedSampleForTest`, read `apps/api/internal/agent/enforcement/` to find a
real banned phrase and inline it as a string literal in the test instead of
adding a helper.

- [ ] **Step 2: Run to verify failure**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ 2>&1 | head -20
```

Expected: FAIL — `undefined: ProposeSpotCheck`, `undefined: SpotCheckTarget`.

- [ ] **Step 3: Write `apps/api/internal/agent/spotcheck.go`**

```go
package agent

// spotcheck.go — N3f Task 2. The S3/S4 station spot-checks: 信源体检 and
// 论证体检. Structurally the same move as 整稿体检 (review.go) one and two
// stations earlier, which is what makes the two `human` gate items
// source_quality_spot_check / warrant_quality_spot_check satisfiable at all.
//
// A human gate item is satisfied by an adjudicating voice the student summons
// — the reading whole_draft_review/orderReview already established
// (api/writing.go:363-394). When a teacher-facing surface exists, these are
// where a real teacher routes in; the gate item names do not change.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
)

// The two contracts that have a spot-check. Any other contract id is a 404 at
// the endpoint, so this route can never mint interventions against a station
// with no check defined.
const (
	SpotCheckSources  = "evaluate_sources"
	SpotCheckArgument = "build_argument"
)

// SpotCheckTarget is one thing the check examines: a source (S3) or a Toulmin
// slot (S4). Detail is the already-projected description the prompt carries —
// building it is the caller's job, so this file stays free of storage shapes.
type SpotCheckTarget struct {
	ID     string
	Name   string
	Detail string
}

// SpotCheckItem is one work-order row from a station spot-check.
//
// Deliberately has NO band and NO points. Points is 0457 readiness-gauge data
// (Slice 9), specific to the whole-draft review. A band would be a quality
// verdict on her sources, and the dossier deliberately carries no credibility
// field because such a judgment "has no honest producer and would have to be
// fabricated" (studio/dto.go:200-205) — a band here would smuggle it back in.
type SpotCheckItem struct {
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	Evidence   string `json:"evidence"`
	Missing    string `json:"missing"`
	Fix        string `json:"fix"` // advice only, never a rewritten sentence (RL-1)
}

// spotCheckItemWire is the model's per-item contract. target_name is absent by
// design: the name is resolved server-side from the id, the same discipline
// ProposeReview uses for CriterionName — the model never invents labels.
type spotCheckItemWire struct {
	TargetID string `json:"target_id"`
	Evidence string `json:"evidence"`
	Missing  string `json:"missing"`
	Fix      string `json:"fix"`
}

const spotCheckPostureSources = `你是 IB/国际课程研究过程的「信源体检」考官。学生已经把她评估过的来源摆在这里。
只做一件事：逐条指出这条来源的「作用与风险」写到了什么程度、还缺什么——是没说清它能回答什么，
还是没说清它不能回答什么，还是根本没有交代它在论证里承担的角色。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要补什么/要想清楚什么"的方向，
不能是可直接粘贴的成品句子。绝不给这条来源下「可信 / 存疑」这类结论——那是学生自己的判断。
一次只输出 JSON 数组，每条来源一个对象。`

const spotCheckPostureArgument = `你是 IB/国际课程论证结构的「论证体检」考官。学生已经把她的论证骨架摆在这里。
只做一件事：逐个位置指出这一步写到了什么程度、还缺什么——理据有没有把证据到主张之间的推理写出来，
反方是不是被写成了最强的版本（还是打了稻草人），让步有没有真的转折回来。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要补哪一步/要想清楚什么"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个位置一个对象。`

// spotCheckSystemPrompt selects the station's posture. Pure — no I/O — so
// posture selection is unit-testable without a model.
func spotCheckSystemPrompt(station string) string {
	if station == SpotCheckArgument {
		return spotCheckPostureArgument
	}
	return spotCheckPostureSources
}

// ProposeSpotCheck asks the flagship model for one station's work order, then
// runs the enforcement stack on every field before returning. A single
// banned-phrasing violation rejects the WHOLE order (nothing returned, nothing
// persisted) — the same all-or-nothing discipline as ProposeReview and the
// coach. Usage is populated whenever Collect succeeded, even on a later
// rejection, so the caller can still record 档位+token+成本: a rejected call
// still cost money.
func ProposeSpotCheck(ctx context.Context, prov gateway.Provider, r gateway.Resolved, station string, targets []SpotCheckTarget) ([]SpotCheckItem, gateway.ChatUsage, error) {
	name := make(map[string]string, len(targets))
	lines := make([]string, 0, len(targets))
	for _, t := range targets {
		name[t.ID] = t.Name
		lines = append(lines, fmt.Sprintf("[%s] %s —— %s", t.ID, t.Name, t.Detail))
	}
	user := strings.Join(lines, "\n")

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: spotCheckSystemPrompt(station)},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var wires []spotCheckItemWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &wires); err != nil {
		return nil, usage, fmt.Errorf("agent: spot-check output not a JSON array: %w", err)
	}
	items := make([]SpotCheckItem, 0, len(wires))
	for _, wv := range wires {
		nm, known := name[wv.TargetID]
		if !known {
			continue // ignore targets the caller didn't ask about
		}
		for _, field := range []string{wv.Evidence, wv.Missing, wv.Fix} {
			if field == "" {
				continue
			}
			if rule := enforcement.BannedPhrasing(field); rule != nil {
				return nil, usage, fmt.Errorf("agent: spot-check output rejected by banned-phrasing rule %q", rule.Name)
			}
		}
		items = append(items, SpotCheckItem{
			TargetID: wv.TargetID, TargetName: nm,
			Evidence: wv.Evidence, Missing: wv.Missing, Fix: wv.Fix,
		})
	}
	if len(items) == 0 {
		return nil, usage, fmt.Errorf("agent: spot-check produced no usable items")
	}
	return items, usage, nil
}
```

- [ ] **Step 4: Run to verify pass**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ 2>&1 | tail -5
```

Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/spotcheck.go apps/api/internal/agent/spotcheck_test.go
git commit -m "feat(n3f): ProposeSpotCheck — 信源体检 / 论证体检 postures, no band, no points"
```

---

## Task 3: The fingerprint

**Files:**
- Modify: `apps/api/internal/agent/spotcheck.go`
- Modify: `apps/api/internal/agent/spotcheck_test.go`

**Interfaces:**
- Produces: `func SpotCheckFingerprint(targets []SpotCheckTarget) string` — hex SHA-256, stable across process runs, order-sensitive because the caller always builds targets in a deterministic order.

**Why this exists:** 整稿体检 is one-snapshot-one-review, anchored on the
snapshot id the student explicitly committed. S3/S4 have no commit action, so
the fingerprint IS the snapshot: pressing again with unchanged work re-streams
the stored items and makes zero model calls, while improving a risk_note makes
the check orderable again. That satisfies 「loops are normal operation; gates
govern what unlocks, never what may be revisited」.

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/agent/spotcheck_test.go`:

```go
func TestSpotCheckFingerprintStability(t *testing.T) {
	a := []SpotCheckTarget{{ID: "m1", Name: "NASA", Detail: "作用与风险：仅是遥感叶面积。"}}
	if SpotCheckFingerprint(a) != SpotCheckFingerprint(a) {
		t.Fatal("fingerprint must be stable for identical input")
	}
	changedDetail := []SpotCheckTarget{{ID: "m1", Name: "NASA", Detail: "作用与风险：补充了口径说明。"}}
	if SpotCheckFingerprint(a) == SpotCheckFingerprint(changedDetail) {
		t.Error("rewriting what the check reads must change the fingerprint")
	}
	added := []SpotCheckTarget{
		{ID: "m1", Name: "NASA", Detail: "作用与风险：仅是遥感叶面积。"},
		{ID: "m2", Name: "BP", Detail: "作用与风险：总量仍高。"},
	}
	if SpotCheckFingerprint(a) == SpotCheckFingerprint(added) {
		t.Error("adding a target must change the fingerprint")
	}
	// The NAME is display-only; it must still participate, because renaming a
	// source changes what the student sees in the work order.
	renamed := []SpotCheckTarget{{ID: "m1", Name: "NASA (v2)", Detail: "作用与风险：仅是遥感叶面积。"}}
	if SpotCheckFingerprint(a) == SpotCheckFingerprint(renamed) {
		t.Error("renaming a target must change the fingerprint")
	}
}

func TestSpotCheckFingerprintIsNotConcatenationAmbiguous(t *testing.T) {
	// A naive strings.Join without a separator would hash these identically.
	x := []SpotCheckTarget{{ID: "ab", Name: "c", Detail: "d"}}
	y := []SpotCheckTarget{{ID: "a", Name: "bc", Detail: "d"}}
	if SpotCheckFingerprint(x) == SpotCheckFingerprint(y) {
		t.Error("field boundaries must be unambiguous in the hashed serialization")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ 2>&1 | head -10
```

Expected: FAIL — `undefined: SpotCheckFingerprint`.

- [ ] **Step 3: Implement**

Append to `apps/api/internal/agent/spotcheck.go` (and add `crypto/sha256` and
`encoding/hex` to the imports):

```go
// SpotCheckFingerprint hashes exactly what a spot-check reads, so an unchanged
// order can be answered from storage with zero model calls. This is
// orderReview's one-snapshot-one-review guard with the snapshot id generalized
// to a content hash, because S3/S4 have no commit action to anchor on.
//
// The serialization is length-prefixed, not delimiter-joined: a student's
// risk_note may contain any character, so no separator byte is safe, and a
// naive join would let two different dossiers collide.
func SpotCheckFingerprint(targets []SpotCheckTarget) string {
	h := sha256.New()
	write := func(s string) {
		fmt.Fprintf(h, "%d:", len(s))
		h.Write([]byte(s))
	}
	for _, t := range targets {
		write(t.ID)
		write(t.Name)
		write(t.Detail)
	}
	return hex.EncodeToString(h.Sum(nil))
}
```

- [ ] **Step 4: Run to verify pass**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ 2>&1 | tail -5
```

Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/spotcheck.go apps/api/internal/agent/spotcheck_test.go
git commit -m "feat(n3f): SpotCheckFingerprint — content hash replaces the snapshot id"
```

---

## Task 4: The spot-check endpoint

**Files:**
- Create: `apps/api/internal/api/spotcheck.go`
- Create: `apps/api/internal/api/spotcheck_test.go`
- Modify: `apps/api/internal/api/api.go` (one route, after line 70)

**Interfaces:**
- Consumes: `agent.ProposeSpotCheck`, `agent.SpotCheckFingerprint`, `agent.SpotCheckTarget`, `agent.SpotCheckItem`, `agent.SpotCheckSources`, `agent.SpotCheckArgument` (Tasks 2–3); `a.attestS3S4` (Task 1) is NOT called here.
- Produces: `func (a *API) orderSpotCheck(w http.ResponseWriter, r *http.Request)`; `func studio.SpotCheckTargets(d studio.ProjectData, station string) []agent.SpotCheckTarget` (in `apps/api/internal/studio/spotcheck.go` — see "Target builders" below); `func spotCheckItemsFor(ctx, q, projectID, station, fingerprint string) []agent.SpotCheckItem` (in `api`).

**Route:**

```go
mux.Handle("POST /api/v1/projects/{id}/contracts/{contractId}/spot-check", protected(a.orderSpotCheck))
```

**Model this handler on `orderReview` (`api/writing.go:267–433`) step for step:**
`loadOwnedProject` → validate `contractId` against the two allowed constants
(anything else → `httpx.ErrNotFound("资源不存在")`) → build targets → compute
fingerprint → read existing items for that fingerprint → **entitlement check
before the stream, only when a model call will happen** → `gateway.NewSSEWriter`
→ `studioEmitter` + `startHeartbeat` → replay-and-return when items exist →
resolve → `ProposeSpotCheck` → `RecordLLMCall` **even when the call was
rejected** → persist each item as an intervention → flip the gate **only if at
least one item persisted** → append event → `advanceGates` → `em.Review` →
`em.Done`.

**Persistence:** reuse `store.InsertReviewIntervention` with
`Type` set through a new sibling method rather than overloading `review_item`.
Add to `apps/api/internal/agent/agentstore.go`:

```go
// SpotCheckInterventionRow is one persisted station spot-check item. Anchor
// carries {"station":…,"fingerprint":…} — the additive-anchor-field seam Slice
// 8b used for `voice`, so no migration. No card_instance_id: a station
// spot-check is not a card submission.
type SpotCheckInterventionRow struct {
	ProjectID uuid.UUID
	Anchor    []byte
	Body      string
}

func (s *sqlcAgentStore) InsertSpotCheckIntervention(ctx context.Context, row SpotCheckInterventionRow) error {
	_, err := s.q.InsertIntervention(ctx, sqlc.InsertInterventionParams{
		ProjectID: row.ProjectID,
		Type:      "spot_check_item",
		Anchor:    row.Anchor,
		Body:      row.Body,
	})
	return err
}
```

Check `InsertInterventionParams` for whether `Criterion` and `Level` are
pointer fields; if they are non-nullable in the generated struct, pass the
zero value explicitly rather than omitting them. Add
`InsertSpotCheckIntervention` to the `AgentStore` interface next to
`InsertReviewIntervention`.

**Gate flip** — copy the guard and its reasoning from `writing.go:374–394`:

```go
	// Mark the station's human gate item solid ONLY if at least one item
	// actually persisted. If every insert failed, the gate must NOT be marked
	// solid: a solid gate with zero visible items makes the gate and the UI
	// disagree, and a phantom "already ordered" record would make every later
	// attempt a silent no-op with nothing to show for it.
	if len(persisted) > 0 {
		itemName := map[string]string{
			agent.SpotCheckSources:  "source_quality_spot_check",
			agent.SpotCheckArgument: "warrant_quality_spot_check",
		}[station]
		// Guard on the item actually being in the contract's Gate.Human, so a
		// skill-config edit can never leave this writing a name no gate reads.
		if c, ok := sk.Contracts[station]; ok {
			for _, hi := range c.Gate.Human {
				if hi == itemName {
					recorded, gerr := store.ListGateStates(r.Context(), projectID)
					if gerr != nil {
						slog.Warn("spot check: list gate states", "err", gerr)
						break
					}
					rec := recorded[station]
					if rec.Items == nil {
						rec.Items = map[string]string{}
					}
					rec.Items[itemName] = "solid"
					if err := store.UpsertGateState(r.Context(), projectID, station, rec); err != nil {
						slog.Warn("spot check: record gate item", "err", err)
					}
					break
				}
			}
		}
		if err := store.AppendEvent(r.Context(), agent.EventRow{
			ProjectID: projectID, Surface: "studio", Type: "spot_check_ordered",
			Payload: mustJSON(map[string]any{"station": station, "items": len(persisted)}),
		}); err != nil {
			slog.Warn("spot check: append event", "err", err)
		}
		a.advanceGates(r.Context(), projectID)
	}
```

**Metering:** `RecordLLMCall` with `Surface: "studio"`, `Purpose: "spot_check"`
— a distinct purpose from `order_review` so per-station cost stays legible in
`llm_usage`.

**Target builders — these live in `studio`, not `api`.**

Task 5's projection must compute `orderable` by comparing the **current**
fingerprint against the stored one, which means it needs the same target list
this handler feeds the model. `studio` cannot import `api`, and two
independent target builders would let the projection's `orderable` and the
handler's fingerprint drift apart — the exact "computed twice, differently"
failure this codebase has hit before (see `materialByCardInstance`'s comment,
`studio/projection.go:219–223`).

So build them once, in `apps/api/internal/studio/spotcheck.go`, over
`ProjectData` — which is where source-log, material, edge and node reads
already live. `studio` already imports `agent` (`projection.go:12`), so the
return type is fine:

```go
// SpotCheckTargets builds what a station's spot-check reads. Deterministic
// order — the fingerprint depends on it, so unchanged work must always
// serialize identically. Returns an empty slice when the station has nothing
// to check.
func SpotCheckTargets(d ProjectData, station string) []agent.SpotCheckTarget
```

The handler loads `ProjectData` the same way the projection endpoint does and
calls this; the projection calls it too. One definition, no drift.

- **S3 (`evaluate_sources`)**: one target per `kind:"article"` material, ordered
  by the material's `created_at` then id. `Name` = the material title.
  `Detail` = 「档位：{tier}；一句话收获：{takeaway}；作用与风险：{risk_note or 「（未写）」}；
  {横向核查过 | 未横向核查}」, reading `tier`/`takeaway`/`lateral_read` from
  `ListSourceLogByProject` and `risk_note` via the `evaluated-as` edge exactly
  as Task 1's `allArticlesHaveRiskNote` does.
- **S4 (`build_argument`)**: one target per Toulmin slot **in the card spec's
  slot order** (`cards.ByID("toulmin").Params.Slots`), skipping slots with no
  node text. `Name` = the slot's `Role` (核心主张 / 理据 · 推理 / …).
  `Detail` = the slot's `body.text`.

Both return an empty slice when nothing qualifies; the handler then answers
with `httpx.ErrBadRequest("nothing_to_check", "还没有可以体检的内容。", nil)`
**before** opening the stream, so an empty station never costs a model call.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/api/spotcheck_test.go` with a table-driven test
covering: (a) an unknown `contractId` returns 404 and makes no model call;
(b) an empty station returns `nothing_to_check` and makes no model call;
(c) a successful order persists N interventions, marks the station's human
item solid, and appends one `spot_check_ordered` event; (d) **re-ordering with
an unchanged fingerprint returns the same items and increments no LLM-call
count**; (e) when every insert fails the gate item is NOT solid.

Follow the existing integration-test harness in `apps/api/internal/api/` — read
one existing SSE endpoint test (search for `orderReview` in `*_test.go`) and
mirror its setup, provider stub, and SSE-reading helper exactly. Do not invent
a second harness.

- [ ] **Step 2: Run to verify failure**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ 2>&1 | head -20
```

Expected: FAIL — the route is unregistered (404 on the success case).

- [ ] **Step 3: Implement `apps/api/internal/api/spotcheck.go` and register the route**

- [ ] **Step 4: Run to verify pass**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ ./internal/agent/ 2>&1 | tail -10
```

Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/spotcheck.go apps/api/internal/api/spotcheck_test.go apps/api/internal/api/api.go apps/api/internal/agent/agentstore.go
git commit -m "feat(n3f): station spot-check endpoint — fingerprint replay, gate flip, metered"
```

---

## Task 5: Contracts + projection for the spot-checks

**Files:**
- Modify: `packages/contracts/src/studioState.ts`
- Modify: `apps/api/internal/studio/dto.go`
- Modify: `apps/api/internal/studio/projection.go`
- Test: `packages/contracts/test/studioState.test.ts`, `apps/api/internal/studio/projection_test.go`

**Interfaces:**
- Produces (Zod, and the matching Go DTO with identical json tags):

```ts
export const SpotCheckItem = z.object({
  interventionId: z.string(),
  targetId: z.string(),
  targetName: z.string(),
  evidence: z.string(),
  missing: z.string(),
  fix: z.string(),
  disposition: Disposition.nullable(),
});
export type SpotCheckItem = z.infer<typeof SpotCheckItem>;

export const SpotCheckFx = z.object({
  items: z.array(SpotCheckItem),
  orderable: z.boolean(),
});
export type SpotCheckFx = z.infer<typeof SpotCheckFx>;
```

added to `StudioProjection` as a **flat, required top-level member**, mirroring
how N3d added `framing`/`perspectives`:

```ts
  spotChecks: z.object({
    evaluateSources: SpotCheckFx,
    buildArgument: SpotCheckFx,
  }),
```

Use the existing `Disposition` shape that `WritingReviewItem.disposition` already
uses — read `packages/contracts/src/studioState.ts:208–220` and reuse it
verbatim rather than defining a parallel type.

**Note the two shapes are not the same struct.** `agent.SpotCheckItem` (Task 2)
is what the model produces and what gets marshalled into the intervention
`body`. The projected DTO is that struct **plus** `interventionId` (from the
intervention row) and `disposition` (from `d.Dispositions`) — exactly the
relationship `agent.ReviewItem` has with `WritingReviewItem`. Do not add
`interventionId` or `disposition` to `agent.SpotCheckItem`.

**`orderable` is computed server-side**: true when the station's current
fingerprint differs from the fingerprint stored on its items, or when it has
no items. The button's enabled state must never be a client guess about
whether pressing it would cost money.

Compute the current fingerprint with `agent.SpotCheckFingerprint(
studio.SpotCheckTargets(d, station))` — the **same** builder Task 4's handler
uses, defined in `apps/api/internal/studio/spotcheck.go`. Do not write a second
target builder here: if the projection's notion of "what the check reads"
diverges from the handler's, `orderable` will disagree with what actually
happens when the button is pressed.

Edge case to get right: when the station has **no** targets at all,
`orderable` is **false** (there is nothing to check), not true.

- [ ] **Step 1: Write the failing tests** — a contracts test asserting a
      projection lacking `spotChecks` fails to parse and that a well-formed one
      passes; a Go projection test asserting `orderable` is false when the
      stored fingerprint matches current state and true after a risk_note
      changes.

- [ ] **Step 2: Run to verify failure**

```bash
cd packages/contracts && npm test 2>&1 | tail -10
cd ../../apps/api && CGO_ENABLED=0 go test ./internal/studio/ 2>&1 | head -10
```

Expected: both FAIL.

- [ ] **Step 3: Implement the Zod types, the Go DTO, and `projectSpotChecks`.**

`projectSpotChecks` reads `d.Interventions` for `Type == "spot_check_item"`,
unmarshals `Body` into the DTO (the body IS the marshalled item — one
`json.Unmarshal`, never string-splitting, exactly as
`reviewItemFromIntervention` does), groups by the anchor's `station`, and
attaches dispositions from `d.Dispositions` the same way the writing
projection attaches them to review items.

- [ ] **Step 4: Run to verify pass**

```bash
cd packages/contracts && npm test 2>&1 | tail -5
cd ../../apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/studio/ 2>&1 | tail -5
```

Expected: `ok` / passing.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/studioState.ts packages/contracts/test/studioState.test.ts apps/api/internal/studio/dto.go apps/api/internal/studio/projection.go apps/api/internal/studio/projection_test.go
git commit -m "feat(n3f): spotChecks projection + contracts, orderable computed server-side"
```

---

## Task 6: Extract the shared work-order item

**Files:**
- Create: `apps/web/src/studio/WorkOrder.tsx`
- Create: `apps/web/src/studio/WorkOrder.test.tsx`
- Modify: `apps/web/src/studio/views/WritingView.tsx:126–270`

**Interfaces:**
- Produces:

```tsx
export type WorkOrderRow = {
  interventionId: string;
  label: string;                 // criterion (review) | targetName (spot-check)
  band?: string;                 // review only — OPTIONAL, never blank-stringed
  evidence: string;
  missing: string;
  fix: string;
  disposition: { action: "accept" | "rewrite" | "reject"; reason: string } | null;
};

export function WorkOrderItem({
  row,
  onDisposition,
}: {
  row: WorkOrderRow;
  onDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
}): JSX.Element;
```

This is a **pure move** of `WritingView.tsx`'s existing `WorkOrderItem`
(lines 162–270) plus the `BAND_TONE` / `bandChipStyle` / `REVIEW_KEYS` /
`runeCount` helpers it depends on (lines 126–160). Copy them verbatim; change
only what the interface change requires.

**The one behavioural change:** `band` is optional. When absent, render **no
chip at all** — not a chip with an empty string. A spot-check row has no band
by design (`agent/spotcheck.go`'s `SpotCheckItem` has no band field), and an
empty chip would put a blank pill where the review shows a band.

Keep the ≥15-rune reason rule, its three states of hint copy, and the exact
`REVIEW_KEYS` labels (保持原样 / 我来改 / 说明为什么不改) unchanged — they are
verbatim from `dc.html:2260`.

- [ ] **Step 1: Write the failing test**

```tsx
// apps/web/src/studio/WorkOrder.test.tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { WorkOrderItem, type WorkOrderRow } from "./WorkOrder";

const base: WorkOrderRow = {
  interventionId: "iv1",
  label: "表F 评估",
  evidence: "两个来源可信。",
  missing: "没写出各自的作用与风险。",
  fix: "在信源档案里补上每条的作用与风险",
  disposition: null,
};

describe("WorkOrderItem", () => {
  it("renders a band chip when band is present", () => {
    render(<WorkOrderItem row={{ ...base, band: "5–6 段" }} />);
    expect(screen.getByText("5–6 段")).toBeInTheDocument();
  });

  it("renders NO chip when band is absent (spot-check rows have no band)", () => {
    const { container } = render(<WorkOrderItem row={base} />);
    // The label must be the only text in the header row.
    expect(screen.getByText("表F 评估")).toBeInTheDocument();
    expect(container.querySelectorAll("span")).toHaveLength(1);
  });

  it("requires a ≥15-rune reason before a disposition can be recorded", async () => {
    const onDisposition = vi.fn();
    render(<WorkOrderItem row={base} onDisposition={onDisposition} />);
    await userEvent.click(screen.getByText("我来改"));
    const box = screen.getByPlaceholderText("写下你的理由（至少 15 字）");
    await userEvent.type(box, "太短了");
    expect(screen.getByRole("button", { name: "记录处置" })).toBeDisabled();
    await userEvent.clear(box);
    await userEvent.type(box, "这条来源只能说明现象存在，不能说明成因，我要改写这一段。");
    await userEvent.click(screen.getByRole("button", { name: "记录处置" }));
    expect(onDisposition).toHaveBeenCalledWith("iv1", "rewrite", expect.stringContaining("只能说明现象存在"));
  });
});
```

- [ ] **Step 2: Run to verify failure**

```bash
cd apps/web && npx vitest run src/studio/WorkOrder.test.tsx 2>&1 | tail -10
```

Expected: FAIL — cannot resolve `./WorkOrder`.

- [ ] **Step 3: Create `WorkOrder.tsx` and rewire `WritingView.tsx`**

`WritingView` maps its `WritingReviewItem`s into `WorkOrderRow`
(`label: item.criterion`, `band: item.band`) and renders the imported
component. Delete the now-dead local `WorkOrderItem`, `BAND_TONE`,
`bandChipStyle`, `REVIEW_KEYS`, and `runeCount` from `WritingView.tsx`.

- [ ] **Step 4: Run the full web suite and the type check**

```bash
cd apps/web && npm test 2>&1 | tail -10 && npx tsc --noEmit
```

Expected: all pass, `tsc` silent. `WritingView.test.tsx` must still pass
unchanged — if it does not, the extraction changed behaviour and must be
corrected rather than the test relaxed.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/WorkOrder.tsx apps/web/src/studio/WorkOrder.test.tsx apps/web/src/studio/views/WritingView.tsx
git commit -m "refactor(n3f): extract shared WorkOrderItem with an optional band chip"
```

---

## Task 7: The S3 and S4 spot-check panels

**Files:**
- Create: `apps/web/src/studio/SpotCheckPanel.tsx`
- Create: `apps/web/src/studio/SpotCheckPanel.test.tsx`
- Modify: `apps/web/src/studio/ViewFrame.tsx` (素材 branch at 278–338; 结构 branch at 339–347)
- Modify: `apps/web/src/studio/StudioContainer.tsx`, `apps/web/src/studio/state.ts`, `apps/web/src/api/writing.ts`, `apps/web/src/api/index.ts`

**Interfaces:**
- Consumes: `WorkOrderItem`, `WorkOrderRow` (Task 6); `SpotCheckFx` (Task 5).
- Produces:

```tsx
export function SpotCheckPanel({
  title,          // "信源体检" | "论证体检"
  data,           // SpotCheckFx
  onOrder,        // () => void
  onDisposition,  // (interventionId, action, reason) => void
  pending,        // boolean — an order is in flight
}: { ... }): JSX.Element;
```

One component, two call sites. The trigger button mirrors 整稿体检's placement
and label styling (`dc.html:1103`); the empty state mirrors `dc.html:1149`'s
copy shape — a sentence saying what the check does and that it is
re-orderable only after the work changes. Write that copy in Chinese, in the
same register as the surrounding views. **No examiner-voice switcher** —
voices are a whole-draft-review affordance and there is no design for them
here.

When `data.orderable` is false and items exist, the button is disabled with a
note that there is nothing new to check yet. That is a fact about state, not a
scolding — keep it neutral (铁律 2).

**Client:** add to `apps/web/src/api/writing.ts` a function mirroring the
existing `orderReview` client (same SSE consumption, same error envelope
handling):

```ts
export function orderSpotCheck(projectId: string, contractId: string): Promise<void>;
```

and declare it on the `Api` interface in `apps/web/src/api/index.ts`. After the
stream completes, **refetch the projection** — N3d's Important finding I2 was a
fire-and-forget write whose gate closed server-side while the rail kept lying
until reload. Do not repeat it.

- [ ] **Step 1: Write the failing tests** — cover: items render through
      `WorkOrderItem`; the order button calls `onOrder`; the button is disabled
      with the neutral note when `orderable` is false and items exist; the
      empty state renders when there are no items.

- [ ] **Step 2: Run to verify failure**

```bash
cd apps/web && npx vitest run src/studio/SpotCheckPanel.test.tsx 2>&1 | tail -10
```

Expected: FAIL — cannot resolve `./SpotCheckPanel`.

- [ ] **Step 3: Implement the component and wire both call sites.**

In `ViewFrame.tsx`, render the panel below `SourceDossier` in the 素材 branch
and below `StructureView` in the 结构 branch. It must render in the
**non-compare** 素材 branch (line 326) as well as inside the compare branch's
dossier view (line 286) — a student in cross-check mode has not left S3.

- [ ] **Step 4: Run the full web suite and type check**

```bash
cd apps/web && npm test 2>&1 | tail -10 && npx tsc --noEmit
```

Expected: all pass, `tsc` silent.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/SpotCheckPanel.tsx apps/web/src/studio/SpotCheckPanel.test.tsx apps/web/src/studio/ViewFrame.tsx apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/state.ts apps/web/src/api/writing.ts apps/web/src/api/index.ts
git commit -m "feat(n3f): 信源体检 / 论证体检 panels at S3 and S4"
```

---

## Task 8: The S6 AI-usage declaration — backend

**Files:**
- Create: `apps/api/internal/api/declaration.go`
- Create: `apps/api/internal/api/declaration_test.go`
- Modify: `apps/api/internal/api/api.go` (one route)
- Modify: `apps/api/internal/studio/projection.go`, `apps/api/internal/studio/dto.go`

**Interfaces:**
- Produces:
  - `type DeclarationCounts struct { Asks, Dispositions, CardsSpontaneous, CardsPrompted, AiWrittenProse int }` with json tags `asks`, `dispositions`, `cardsSpontaneous`, `cardsPrompted`, `aiWrittenProse`.
  - `func countDeclaration(d studio.ProjectData) DeclarationCounts` — pure, unit-testable with no DB.
  - `func (a *API) signDeclaration(w http.ResponseWriter, r *http.Request)`.
- Route: `mux.Handle("POST /api/v1/projects/{id}/declaration/sign", protected(a.signDeclaration))`

**Counter sources** (all data that already exists — **no model call**):

| Field | Source |
|---|---|
| `Asks` | count of `prompt_sent` events in `d.Events` (`api/assessment.go:169` already reads this type) |
| `Dispositions` | `len(d.Dispositions)` |
| `CardsSpontaneous` / `CardsPrompted` | the same derivation `projectEquipment` uses (`studio/projection.go:260–262`): 提示后 when an intervention links the card instance, else 自发 |
| `AiWrittenProse` | `0` — see below |

**`AiWrittenProse` is a claim by construction, not a measurement.** It holds
because RL-1 makes the review's `fix` field advice that is never written into
the draft, and because no code path places model text into the edit buffer.
Write that reasoning as a comment on the field, naming the invariant, so that
if it is ever broken the declaration does not go on quietly printing 0. A
future slice that adds any AI-to-draft path **must** either measure this
counter or delete it.

**Why its own endpoint and not `attestGate`:** `attestGate`
(`api/writing.go:228–238`) is deliberately restricted to a contract's
`student_written` names so a caller cannot forge a machine or human item. That
restriction stays. `signDeclaration` recomputes the counters server-side,
persists them into a `graph_node` of type `declaration` with
`author: "student"`, and only then sets `reflect_archive`'s
`declaration_signed` item to solid, followed by `advanceGates`. The signature
therefore records **what was signed** rather than pointing at a live view that
keeps moving — 过程即数据.

Persist the node and flip the gate in **one transaction** (`a.d.Pool.Begin` +
`a.d.Queries.WithTx`), the pattern `createProject` uses
(`api/project_create.go:61–96`): a signature recorded with no counters, or
counters with no signature, are both worse than a clean failure. `advanceGates`
runs **after** the commit.

Also add a `declaration` member to the studio projection so the view can render
the counters before signing. Its DTO is `DeclarationCounts` **plus** a `signed`
bool read from the recorded `reflect_archive` gate item — so the projected
shape has six fields and matches Task 9's `DeclarationFx` exactly:

```go
type DeclarationDTO struct {
	Asks             int  `json:"asks"`
	Dispositions     int  `json:"dispositions"`
	CardsSpontaneous int  `json:"cardsSpontaneous"`
	CardsPrompted    int  `json:"cardsPrompted"`
	AiWrittenProse   int  `json:"aiWrittenProse"`
	Signed           bool `json:"signed"`
}
```

Before signing, the counters are computed live. **After** signing, they are
read back from the persisted `declaration` node, not recomputed — otherwise the
screen would show numbers that drift away from what she actually signed.

- [ ] **Step 1: Write the failing tests** — a pure unit test for
      `countDeclaration` over a hand-built `studio.ProjectData` (asserting the
      spont/prompted split matches `projectEquipment`'s own rule and that
      `AiWrittenProse` is 0), plus an integration test that signing persists a
      `declaration` node, marks `declaration_signed` solid, and that a second
      sign is idempotent rather than minting a second node.

- [ ] **Step 2: Run to verify failure**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ 2>&1 | head -20
```

Expected: FAIL.

- [ ] **Step 3: Implement.**

- [ ] **Step 4: Run to verify pass**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ ./internal/studio/ 2>&1 | tail -10
```

Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/declaration.go apps/api/internal/api/declaration_test.go apps/api/internal/api/api.go apps/api/internal/studio/projection.go apps/api/internal/studio/dto.go
git commit -m "feat(n3f): AI 使用申报单 — counters projected, signature persists what was signed"
```

---

## Task 9: The S6 declaration — contracts + UI

**Files:**
- Modify: `packages/contracts/src/studioState.ts`
- Modify: `apps/web/src/studio/views/ReviewView.tsx`
- Modify: `apps/web/src/studio/StudioContainer.tsx`, `apps/web/src/api/writing.ts`, `apps/web/src/api/index.ts`
- Test: `apps/web/src/studio/views/ReviewView.test.tsx`

**Interfaces:**
- Produces:

```ts
export const DeclarationFx = z.object({
  asks: z.number().int(),
  dispositions: z.number().int(),
  cardsSpontaneous: z.number().int(),
  cardsPrompted: z.number().int(),
  aiWrittenProse: z.number().int(),
  signed: z.boolean(),
});
export type DeclarationFx = z.infer<typeof DeclarationFx>;
```

added to `StudioProjection` as a required top-level `declaration` member, and
`signDeclaration(projectId: string): Promise<void>` on the `Api` interface.

**UI is binding-design work.** Build to `dc.html:1211–1223` exactly: the
`#FBF7EF` / `#F0E6D2` warm card, the 「AI 使用申报单」 heading in `#8A6520`, the
「自动生成 · 待你签名」 pill, and the 2-column grid of `{k, v}` rows. The four
labels are verbatim from `dc.html:2179–2181`:
`提问 / 追问`, `三键处置（接受/改/拒）`, `工具卡调用（自发/提示后）`, `AI 代写正文`.
Values render as `23 次`, `7 次`, `4 / 2`, `0 次` respectively.

Add the signature control the design's pill implies but does not draw: a single
confirm below the grid. After signing, the pill reads 已签名 and the control is
replaced by the signed state — no re-signing, no undo affordance invented here.
Inline SVG only if an icon is needed; never `lucide-react`.

- [ ] **Step 1: Write the failing test** — assert the four labels render with
      the projected values, that the sign control calls the client, and that a
      signed declaration renders the signed state instead of the control.

- [ ] **Step 2: Run to verify failure**

```bash
cd apps/web && npx vitest run src/studio/views/ReviewView.test.tsx 2>&1 | tail -10
```

Expected: FAIL.

- [ ] **Step 3: Implement.**

- [ ] **Step 4: Run the full suites**

```bash
cd packages/contracts && npm test 2>&1 | tail -5
cd ../../apps/web && npm test 2>&1 | tail -10 && npx tsc --noEmit
```

Expected: all pass, `tsc` silent.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/studioState.ts apps/web/src/studio/views/ReviewView.tsx apps/web/src/studio/views/ReviewView.test.tsx apps/web/src/studio/StudioContainer.tsx apps/web/src/api/writing.ts apps/web/src/api/index.ts
git commit -m "feat(n3f): AI 使用申报单 renders and signs in 评估"
```

---

## Task 10: The L1 block-id collision

**Files:**
- Modify: `apps/api/internal/agent/prompt.go:66–79`
- Modify: `apps/api/internal/agent/anchors.go:83–92, 117–130`
- Test: `apps/api/internal/agent/anchors_test.go`, `apps/api/internal/agent/prompt_test.go`

**The bug:** `materialize.Segment` numbers blocks `b0, b1, …` **per material**
(`materialize/segment.go:28` — its own comment says "stable within a
material"), but `blockLookup` is a flat `map[blockID]` across all materials, so
the last material wins; and `BuildMaterialContext` renders every material's
blocks with colliding `[b0]` labels, so the model cannot disambiguate either.
`projectMaterials` (`api/studioturn.go:410`) passes **all** of a project's
materials, so a project with ≥2 materials mints CRAAP anchors against the wrong
article. Only guidance level L1 resolves block ids (`anchors.go:117`); L2 never
does and L3 makes no model call — which is why N3c's guidance-fade work fixed
this shape for L2/L3 and left the beginner path as the only broken one.

**Shared code:** `Course` calls the same generator (`agent/course.go:125`) but
passes exactly one material (`course.go:122`), so it cannot hit the collision
today. Its tests must stay green regardless — run the full agent package.

- [ ] **Step 1: Write the failing tests**

```go
func TestBuildMaterialContextQualifiesBlockIDs(t *testing.T) {
	materials := []Material{
		{ID: "mat-a", Title: "NASA 观测", Blocks: []MaterialBlock{{ID: "b0", Text: "叶面积指数上升。"}}},
		{ID: "mat-b", Title: "BP 统计", Blocks: []MaterialBlock{{ID: "b0", Text: "煤炭消费仍在上升。"}}},
	}
	got := BuildMaterialContext(materials)
	if strings.Count(got, "[b0]") > 0 {
		t.Error("bare [b0] labels collide across materials — ids must be material-qualified")
	}
	for _, want := range []string{"[mat-a:b0]", "[mat-b:b0]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing qualified label %s in:\n%s", want, got)
		}
	}
}

func TestParseAnchorGenL1ResolvesTheRightMaterial(t *testing.T) {
	materials := []Material{
		{ID: "mat-a", Title: "NASA 观测", Blocks: []MaterialBlock{{ID: "b0", Text: "叶面积指数上升。"}}},
		{ID: "mat-b", Title: "BP 统计", Blocks: []MaterialBlock{{ID: "b0", Text: "煤炭消费仍在上升。"}}},
	}
	text := `[{"block_id":"mat-a:b0","quote":"叶面积指数上升","dimension":"authority","question":"这条数据出自谁？"}]`
	out, err := parseAnchorGen(text, cards.Spec{}, materials, GuidanceL1)
	if err != nil {
		t.Fatalf("parseAnchorGen: %v", err)
	}
	if out[0].MaterialID != "mat-a" {
		t.Errorf("MaterialID = %q, want mat-a — a flat lookup resolves this to the LAST material", out[0].MaterialID)
	}
	if out[0].BlockID != "b0" {
		t.Errorf("BlockID = %q — the stored block id stays material-local", out[0].BlockID)
	}
	if out[0].Start == 0 && out[0].End == 0 {
		t.Error("offsets must resolve against the correct material's block text")
	}
}
```

Match `cards.Spec{}` and `MaterialBlock` to their real definitions before
writing — read `apps/api/internal/agent/prompt.go:50–60` for `MaterialBlock`
and the existing `anchors_test.go` for how `parseAnchorGen` is already called.

- [ ] **Step 2: Run to verify failure**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ 2>&1 | head -20
```

Expected: FAIL on both.

- [ ] **Step 3: Implement**

In `prompt.go`, emit `"[" + m.ID + ":" + blk.ID + "] "`. In `anchors.go`, key
`blockLookup` by the qualified id and split the model's `block_id` on the last
`:` to recover `(materialID, blockID)` — storing the **material-local** block
id on the anchor, since that is what the web uses to slice block text.

Also update the L1 instruction string at `anchors.go:315` so the example
`block_id` shows the qualified form; a prompt that demonstrates `"b0"` while
the context prints `mat-a:b0` teaches the model to emit the ambiguous form.

- [ ] **Step 4: Run the FULL agent package** (course shares this code)

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./internal/agent/ 2>&1 | tail -10
```

Expected: `ok` — including every course test.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/prompt.go apps/api/internal/agent/anchors.go apps/api/internal/agent/anchors_test.go apps/api/internal/agent/prompt_test.go
git commit -m "fix(n3f): material-qualify L1 block ids — ≥2 materials anchored to the wrong article"
```

---

## Task 11: The producerless-gate guard

**Files:**
- Create: `apps/api/internal/skills/producers_test.go`

**This is the single highest-value artifact in the slice.** This defect class
has now shipped three times — S1/S2 (found in N3d), S3/S4 and S6 (found here) —
and each time it was invisible because the seeded demo project made the rail
look alive.

**What it is and is not:** a tripwire, not a proof. Someone can still satisfy
it by adding a map entry pointing at a function that does not really record the
item. What it makes impossible is the failure that actually shipped: adding a
gate item to the skill JSON and never thinking about a producer at all. The map
entry forces the thought and names the producer where the next reader looks.

- [ ] **Step 1: Write the test**

```go
package skills_test

// producers_test.go — N3f Task 11. Every gate item in every skill must have a
// producer. See the plan's Task 11 for why this exists and what it does not
// prove.

import (
	"testing"

	"mindimprint/api/internal/skills"
)

// gateItemProducers names the function that records each non-machine gate
// item. Adding a gate item to a skill's JSON without adding a line here fails
// this test — that is the point.
var gateItemProducers = map[string]string{
	// S0 decode_task
	"weakness_prediction": "api.submitOnboarding",
	"milestone_plan":      "api.submitOnboarding",
	// S1 frame_question
	"research_question":  "api.createProject (title node) + api.submitFraming",
	"provisional_answer": "api.submitFraming",
	"preregistration":    "api.submitFraming",
	"terms_defined":      "api.submitFraming",
	// S2 evaluate_perspectives
	"recon_logged":            "api.attestReconLogged (via logSourceOpen)",
	"sources_per_perspective": "api.attestGate (explicit student confirm)",
	// S3 evaluate_sources
	"source_risk_notes":         "api.attestS3S4",
	"source_quality_spot_check": "api.orderSpotCheck",
	// S4 build_argument
	"warrants":                   "api.attestS3S4",
	"steelman":                   "api.attestS3S4",
	"warrant_quality_spot_check": "api.orderSpotCheck",
	// S5 draft_polish
	"citations_matched":  "api.attestGate",
	"whole_draft_review": "api.orderReview",
	// S6 reflect_archive
	"reflection":         "api.submitReflection",
	"declaration_signed": "api.signDeclaration",
}

// machineKinds is the closed set agent/gate.go's evalMachineItem handles. A
// kind outside it falls through to that switch's default and fails closed at
// runtime, which no test would otherwise catch until a student hit it.
var machineKinds = map[string]bool{
	"node_present":           true,
	"node_count_at_least":    true,
	"no_orphan_evidence":     true,
	"no_unsupported_claim":   true,
	"no_single_sourced_claim": true,
	"every_source_evaluated": true,
}

func TestEveryGateItemHasAProducer(t *testing.T) {
	for _, sk := range skills.All() {
		for contractID, c := range sk.Contracts {
			for _, m := range c.Gate.Machine {
				if !machineKinds[m.Kind] {
					t.Errorf("%s/%s: machine kind %q is not handled by evalMachineItem — it will fail closed at runtime",
						sk.ID, contractID, m.Kind)
				}
			}
			for _, name := range append(append([]string{}, c.Gate.StudentWritten...), c.Gate.Human...) {
				if _, ok := gateItemProducers[name]; !ok {
					t.Errorf("%s/%s: gate item %q has NO registered producer — a student can never clear this station. "+
						"Wire a producer, then add it to gateItemProducers in this file.",
						sk.ID, contractID, name)
				}
			}
		}
	}
}
```

`skills.All()` may not exist — check `apps/api/internal/skills/skill.go` for
the loader's actual enumeration API (it may be a registry map or a
`LoadAll()`); use whatever is there. If no enumeration exists, iterate the
embedded spec filenames directly rather than adding a new exported function
just for the test.

- [ ] **Step 2: Run — it must PASS immediately**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/skills/ 2>&1 | tail -10
```

Expected: `ok`. If it fails, a gate item is still producerless and the earlier
tasks are incomplete — fix the producer, never the map.

- [ ] **Step 3: Verify the guard actually bites**

Temporarily add `"fake_item"` to `writing-project.json`'s `draft_polish`
`student_written` array in **both** `packages/contracts/skills/` and
`apps/api/internal/skills/specs/`, re-run, confirm the test FAILS with the
diagnostic, then revert both files and confirm `git diff` is clean for them.

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/skills/producers_test.go
git commit -m "test(n3f): fail the suite for any gate item with no producer"
```

---

## Task 12: The fresh-project S0 → S6 acceptance walk

**Files:**
- Create: `apps/api/internal/api/walk_s0_s6_test.go`

**Two rules, both learned from N3d's failure:**

1. **Never the seeded demo project.** Migration
   `0018_seed_demo_project.sql` hand-writes `confirmed_solid` gate rows; a test
   against it proves nothing about live code paths. Create the project with
   `POST /api/v1/projects`.
2. **Only endpoints the web client actually calls.** N3d's acceptance test
   POSTed `/materials/{mid}/open` directly — a request no S2 UI state could
   generate — and so passed over a station the student could not clear. Before
   using an endpoint in this test, confirm a control that calls it exists on
   screen at that station by grepping `apps/web/src` for the client function.

The walk: create project → submit onboarding (S0) → submit framing (S1) →
submit perspectives + add sources + open a source + attest
`sources_per_perspective` (S2) → complete CRAAP per source + order 信源体检 (S3)
→ complete the Toulmin slots + order 论证体检 (S4) → commit a snapshot + attest
citations + order 整稿体检 (S5) → submit reflection + sign the declaration (S6).

Assert after each station that `GET /api/v1/projects/{id}` reports that
station `done` and the next one `current` — not merely that the final state is
reachable, so a regression names the station it broke at.

- [ ] **Step 1: Write the test.** Reuse the existing integration harness in
      `apps/api/internal/api/` — read the N3d walk test (search `*_test.go` for
      `submitFraming` or `perspectives`) and extend its pattern rather than
      building a second harness. Use the stub provider for every model call so
      the test is deterministic and free.

- [ ] **Step 2: Run**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./internal/api/ 2>&1 | tail -10
```

Expected: `ok`. A failure here names the first station a real student cannot
clear — treat it as a product bug, not a test bug.

- [ ] **Step 3: Commit**

```bash
git add apps/api/internal/api/walk_s0_s6_test.go
git commit -m "test(n3f): fresh-project walk S0→S6 through client-reachable endpoints only"
```

---

## Task 13: Spec amendments and tracker update

**Files:**
- Modify: `docs/superpowers/specs/2026-07-22-n3f-finish-the-walk-design.md`
- Modify: `docs/2026-07-20-student-platform-remaining-work.md`

**The three spec corrections the code required (§4, §5.7, §10) were already
applied when this plan was written** — do not redo them. Verify they are
present and consistent with what shipped, and amend further only if
implementation diverged from the spec.

**Tracker:** add an `### N3f · DONE` section following N3d's format —
the finding, what shipped, what the whole-branch review caught, what was found
and deliberately not fixed, and what carried forward. Update the N3 row in the
decomposition table. Do **not** mark N3e or N4 as changed.

- [ ] **Step 1: Amend the spec's §4, §5.7 and §10.**
- [ ] **Step 2: Write the tracker's N3f section and update the N3 row.**
- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-07-22-n3f-finish-the-walk-design.md docs/2026-07-20-student-platform-remaining-work.md
git commit -m "docs(n3f): amend spec to match the code; record N3f in the tracker"
```

---

## Final Gate (controller runs these, not a subagent)

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./... 2>&1 | tail -25
cd ../../apps/web && npm test 2>&1 | tail -5 && npx tsc --noEmit
cd ../../packages/contracts && npm test 2>&1 | tail -5
```

Then verify branch hygiene by hand:

- `git diff --stat main...HEAD` lists no file under `apps/api/internal/store/migrations/`.
- `git diff main...HEAD -- packages/contracts/cards/ apps/api/internal/cards/specs/` is **empty** — this slice edits no card JSON.
- `packages/contracts/skills/writing-project.json` and
  `apps/api/internal/skills/specs/writing-project.json` are **unmodified** and
  byte-identical to each other (Task 11 Step 3 must have been reverted).
- `cd apps/api && make sqlc` leaves the tree clean.
- No `lucide-react` import anywhere in the diff.
- `git status --short` shows only the pre-existing `M package.json` and the
  untracked `docs/` and root files that were there before this branch.

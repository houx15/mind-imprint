# 印记陪读 · Read-Together Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move source-reading out of the crowded 3-pane studio into a focused, demo-style read-together surface where the AI draws one worked example on a sentence and the student underlines their *own* evidence, powered by our existing 36-card library, anchor contract, and gateway.

**Architecture:** Backend gains a reading *router* (LLM over a ~15-card catalog + a deterministic downgrade-only gate) and a *selection evaluator* (uniform 3-check rubric + program-owned verdict + evidence-integrity filter), exposed via two new material/card endpoints that reuse the existing card-instance lifecycle. Frontend gains a `ReadingRoom` surface (a `useState` view-swap — the app has **no router**) that reuses the `Annotate` primitive and renders the tool-card *inline, hanging on a sentence* that moves from the AI's example to the student's pick.

**Tech Stack:** Go (`net/http` + `pgx`/sqlc, `gateway.Collect`, `go:embed` card specs), React + Vite + TS (Vitest + RTL), Zod contracts in `packages/contracts`.

**Spec:** `docs/superpowers/specs/2026-07-27-reading-together-redesign-design.md` (authoritative).

## Global Constraints

- **Client never holds the model key.** All LLM calls go through the backend `gateway.Collect`; every real call records `Resolved`/`Usage` (`GenerateResult` pattern in `agent/anchors.go:45-49`). Router/eval use the flagship resolver `gateway.NewEvalKeyResolver` (evaluation never downgrades).
- **Card spec single source of truth = the registry.** The reading deck is a *subset* of existing specs (`cards.Catalog()`/`cards.ByID`), never a second hand-written list of card definitions. No new card JSON in this plan.
- **Anchors use RUNE offsets** (`utf8.RuneCountInString`), not bytes/UTF-16 — see `agent/anchors.go:114-126`, `contracts/src/anchor.ts:9-13`. Reuse `computeOffsets`.
- **Program owns the verdict; the model never grades itself.** Verdict is recomputed from the 3 checks server-side. A finding may cite only a span the student actually picked (evidence-integrity filter). Never render a `(0,0)` "lights up nothing" anchor.
- **Focus is the product.** One active card at a time; card lives *inside* the article, never a rail; one primary button; deck invisible to the student; only the current step shown.
- **No router dependency, no DB schema change.** `ReadingRoom` is a `useState` surface swap (mirror `StudioContainer`'s `openId`). Router pacing state is derived per-turn from existing card-instance/graph state, not stored.
- **Card status enum** (DB check constraint, `store/migrations/0001_init.sql:77`): `'proposed' | 'active' | 'completed' | 'skipped'`. The `evaluating`/`feedback` sub-states are **client-only** loop states, not persisted statuses.
- **Reading deck (spec §18), exact ids:** `craap`, `sift`, `fact-opinion-value`, `argument-map`, `toulmin`, `steelman`, `concession`, `data-literacy`, `opcvl`, `framing`, `spin-detector`, `cda`, `perspective-matrix`, `certainty-spectrum`, `science-knowing`.
- **Verdict labels (exact):** `strong`→`高度匹配`, `partial`→`部分匹配`, `rethink`→`暂不匹配`. **Verdict rule:** `target == miss` → `rethink`; all three `pass` → `strong`; else `partial`.
- **Test commands:** backend `cd apps/api && go test ./...` (subagents: `timeout: 600000`); web `cd apps/web && pnpm test` and `pnpm typecheck`; contracts `cd packages/contracts && pnpm test`. Run FULL Go packages, never `-run` subsets, for the final gate.

---

## File Structure

**Backend (`apps/api/internal/agent/`)** — new, each one responsibility:
- `reading_deck.go` — the ~15-card router catalog derived from the registry.
- `reading_router.go` — the LLM router (`RouteReading`) + JSON contract + deterministic fallback.
- `reading_gate.go` — pure downgrade-only restraint gate + example-anchor resolver.
- `reading_eval.go` — `EvaluateSelection` + `verdictFromChecks` + integrity filter + deterministic fallback.

**Backend (`apps/api/internal/api/`)** — new handlers:
- `readturn.go` — `POST /materials/{mid}/read-turn` (SSE) — router summon scoped to one material.
- `readeval.go` — `POST /cards/{cid}/evaluate` (JSON) — evaluate the student's picked sentence.

**Contracts (`packages/contracts/src/`)**:
- `reading.ts` — `SelectionEval`, `SelectionCheck`, `ReadTurnBody`, `EvaluateBody` Zod schemas + exports in `index.ts`.

**Frontend (`apps/web/src/`)**:
- `api/reading.ts` — `readTurn` (SSE generator), `evaluateCardSelection` (JSON).
- `studio/reading/ReadingRoom.tsx` — the focused surface (coach left, article right).
- `studio/reading/HangingCard.tsx` — the inline card that anchors to a span.
- `studio/reading/readingLoop.ts` — the client loop state machine.
- `studio/reading/ReadingRoom.css` (or Tailwind inline) — the ~39:61 layout + serif article + connector line.

**Retire/reconcile:** `studio/material/SourceDossier.tsx` (article mode → reading room; list mode stays), `studio/CoachRail.tsx` (reading-path card mount + `CraapPlaceholder`), the `sift_craap` triple identity.

---

## Task 1: Reading deck catalog

**Files:**
- Create: `apps/api/internal/agent/reading_deck.go`
- Test: `apps/api/internal/agent/reading_deck_test.go`

**Interfaces:**
- Consumes: `cards.Catalog()`, `cards.ByID(id)` (`cards/loader.go:185-221`); `cards.Spec` fields `ID`, `Name`, `TriggerCondition`, `Purpose` (`cards/loader.go:18-40`).
- Produces: `type ReadingCard struct { CardID, Name, Trigger string }`; `var ReadingDeckIDs []string`; `func ReadingDeck() ([]ReadingCard, error)`.

- [ ] **Step 1: Write the failing test**

```go
package agent

import "testing"

func TestReadingDeck_CoversAllIDsFromRegistry(t *testing.T) {
	deck, err := ReadingDeck()
	if err != nil {
		t.Fatalf("ReadingDeck error: %v", err)
	}
	if len(deck) != len(ReadingDeckIDs) {
		t.Fatalf("deck size = %d, want %d", len(deck), len(ReadingDeckIDs))
	}
	byID := map[string]ReadingCard{}
	for _, c := range deck {
		byID[c.CardID] = c
	}
	for _, id := range ReadingDeckIDs {
		c, ok := byID[id]
		if !ok {
			t.Fatalf("deck missing id %q", id)
		}
		if c.Name == "" || c.Trigger == "" {
			t.Fatalf("deck entry %q has empty name/trigger: %+v", id, c)
		}
	}
}

func TestReadingDeck_HasFifteenCards(t *testing.T) {
	if len(ReadingDeckIDs) != 15 {
		t.Fatalf("ReadingDeckIDs = %d, want 15 (spec §18)", len(ReadingDeckIDs))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestReadingDeck`
Expected: FAIL — `undefined: ReadingDeck` / `undefined: ReadingDeckIDs`.

- [ ] **Step 3: Write the implementation**

```go
package agent

import (
	"fmt"

	"mindimprint/api/internal/cards"
)

// ReadingCard is one entry in the read-together router catalog: the id the
// router may summon, plus the human name and the one-line "when to reach for
// this" the router reasons over. Triggers are sourced from the card spec
// (single source of truth) — never hand-written here.
type ReadingCard struct {
	CardID  string
	Name    string
	Trigger string
}

// ReadingDeckIDs is the fixed set of reading-room cards (spec §18): source-check
// (craap, sift) + deep reading. All are sentence-based, so all fit the
// hang-on-sentence + you-find-the-evidence mechanic. Growing the deck later =
// adding an id here; no page or mechanic change.
var ReadingDeckIDs = []string{
	"craap", "sift",
	"fact-opinion-value", "argument-map", "toulmin", "steelman", "concession",
	"data-literacy", "opcvl", "framing", "spin-detector", "cda",
	"perspective-matrix", "certainty-spectrum", "science-knowing",
}

// ReadingDeck resolves ReadingDeckIDs against the card registry. It errors if
// any id is missing — the deck must never drift from the specs that back it.
func ReadingDeck() ([]ReadingCard, error) {
	out := make([]ReadingCard, 0, len(ReadingDeckIDs))
	for _, id := range ReadingDeckIDs {
		spec, ok := cards.ByID(id)
		if !ok {
			return nil, fmt.Errorf("reading deck id %q not found in registry", id)
		}
		trigger := spec.TriggerCondition
		if trigger == "" {
			trigger = spec.Purpose
		}
		out = append(out, ReadingCard{CardID: spec.ID, Name: spec.Name, Trigger: trigger})
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run TestReadingDeck`
Expected: PASS. (If a `TestReadingDeck_CoversAllIDsFromRegistry` failure names a missing id, that id's spec JSON is absent — confirm it exists in `apps/api/internal/cards/specs/`; all 15 were verified present on 2026-07-27.)

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/reading_deck.go apps/api/internal/agent/reading_deck_test.go
git commit -m "feat(reading): router catalog derived from the card registry"
```

---

## Task 2: Reading router (LLM over the catalog)

**Files:**
- Create: `apps/api/internal/agent/reading_router.go`
- Test: `apps/api/internal/agent/reading_router_test.go`

**Interfaces:**
- Consumes: `gateway.Provider`, `gateway.KeyResolver`, `gateway.Collect` (`gateway/collect.go:10`), `gateway.ChatRequest`/`ChatMessage`/`RoleSystem`/`RoleUser` (`gateway/types.go`), `gateway.StubProvider`/`NewStubProvider` (`gateway/stub.go`), `stripFences` (`agent/anchors.go:70`), `ReadingCard` (Task 1).
- Produces:
```go
type FocusSpan struct { BlockID, Quote string }
type PacingState struct {
	OpenCard              bool
	TurnsSinceLastPropose int
	RecentlySkipped       []string
	CompletedCards        []string
	HasNewFocus           bool
}
type ReadingRouteInput struct {
	StudentText    string
	FocusedSpans   []FocusSpan
	RecentTurns    []string
	Catalog        []ReadingCard
	ScaffoldLevels map[string]int
	Pacing         PacingState
}
type ReadingDecision struct {
	Decision       string   // "respond" | "hint" | "summon"
	CardID         string
	Reason         string   // student-facing nudge text
	ExampleBlockID string   // summon only
	ExampleQuote   string   // summon only; must be verbatim from the block
	ExampleWhy     string
	FollowupPlan   []string // <=2 secondary card ids, queued not shown
}
func RouteReading(ctx context.Context, p gateway.Provider, resolver gateway.KeyResolver, in ReadingRouteInput) (ReadingDecision, gateway.Resolved, gateway.ChatUsage, error)
```

- [ ] **Step 1: Write the failing test** (`reading_router_test.go`)

```go
package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func stubResolver() gateway.KeyResolver {
	return func(ctx context.Context) (gateway.Resolved, error) {
		return gateway.Resolved{Provider: "deepseek", Model: "x", Tier: "flagship"}, nil
	}
}

func TestRouteReading_ParsesSummon(t *testing.T) {
	script := []gateway.StreamEvent{{Delta: `{"decision":"summon","card_id":"argument-map",` +
		`"reason":"这句像是一个没给证据的结论","example_block_id":"b1",` +
		`"example_quote":"因此这项政策必然失败","example_why":"它用'必然'下了强结论",` +
		`"followup_plan":["fact-opinion-value"]}`}}
	p := gateway.NewStubProvider(script)
	in := ReadingRouteInput{
		StudentText: "这段读着怪怪的",
		Catalog:     []ReadingCard{{CardID: "argument-map", Name: "论证地图", Trigger: "结论缺证据时"}},
	}
	d, resolved, _, err := RouteReading(context.Background(), p, stubResolver(), in)
	if err != nil {
		t.Fatalf("RouteReading error: %v", err)
	}
	if d.Decision != "summon" || d.CardID != "argument-map" || d.ExampleQuote != "因此这项政策必然失败" {
		t.Fatalf("unexpected decision: %+v", d)
	}
	if len(d.FollowupPlan) != 1 || d.FollowupPlan[0] != "fact-opinion-value" {
		t.Fatalf("followup plan not parsed: %+v", d.FollowupPlan)
	}
	if resolved.Provider != "deepseek" {
		t.Fatalf("resolved not returned for a real call: %+v", resolved)
	}
}

func TestRouteReading_FallsBackToRespondOnGarbage(t *testing.T) {
	p := gateway.NewStubProvider([]gateway.StreamEvent{{Delta: "not json at all"}})
	d, _, _, err := RouteReading(context.Background(), p, stubResolver(),
		ReadingRouteInput{StudentText: "hi"})
	if err != nil {
		t.Fatalf("RouteReading should not error on garbage, got %v", err)
	}
	if d.Decision != "respond" {
		t.Fatalf("garbage should degrade to respond, got %+v", d)
	}
}

func TestRouteReading_ResolverErrorDegradesToRespond(t *testing.T) {
	p := gateway.NewStubProvider(nil)
	badResolver := gateway.KeyResolver(func(ctx context.Context) (gateway.Resolved, error) {
		return gateway.Resolved{}, context.DeadlineExceeded
	})
	d, resolved, _, err := RouteReading(context.Background(), p, badResolver, ReadingRouteInput{})
	if err != nil {
		t.Fatalf("resolver error should degrade, not error: %v", err)
	}
	if d.Decision != "respond" || resolved.Provider != "" {
		t.Fatalf("want respond + no resolved, got %+v / %+v", d, resolved)
	}
}
```

> Confirm `gateway.StreamEvent`'s text field name before running — the anchors/coach tests already drive `NewStubProvider`; match the field they use (grep `apps/api/internal/agent/anchors_test.go` for `StreamEvent{`). If it is `Text`/`Content` rather than `Delta`, use that name in the script literals above.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestRouteReading`
Expected: FAIL — `undefined: RouteReading` (+ the types).

- [ ] **Step 3: Write the implementation**

```go
package agent

import (
	"context"
	"encoding/json"
	"strings"

	"mindimprint/api/internal/gateway"
)

// (types FocusSpan, PacingState, ReadingRouteInput, ReadingDecision — as in Interfaces above)

// routerReply is the model's raw JSON contract (snake_case on the wire).
type routerReply struct {
	Decision       string   `json:"decision"`
	CardID         string   `json:"card_id"`
	Reason         string   `json:"reason"`
	ExampleBlockID string   `json:"example_block_id"`
	ExampleQuote   string   `json:"example_quote"`
	ExampleWhy     string   `json:"example_why"`
	FollowupPlan   []string `json:"followup_plan"`
}

// respond is the safe fallback: reply in chat, summon nothing.
var respond = ReadingDecision{Decision: "respond"}

// RouteReading asks the flagship model which reading card (if any) to summon on
// this turn. It NEVER errors on a bad reply or a resolver failure — it degrades
// to "respond" so the reading room stays usable. The program-side gate
// (ApplyReadingGate) is authoritative over pacing/ordering; this function only
// produces the model's raw proposal.
func RouteReading(ctx context.Context, p gateway.Provider, resolver gateway.KeyResolver, in ReadingRouteInput) (ReadingDecision, gateway.Resolved, gateway.ChatUsage, error) {
	resolved, err := resolver(ctx)
	if err != nil {
		return respond, gateway.Resolved{}, gateway.ChatUsage{}, nil
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildRouterPrompt(in)},
			{Role: gateway.RoleUser, Content: buildRouterUser(in)},
		},
		MaxTokens: 400,
	}
	res, err := gateway.Collect(ctx, p, resolved, req)
	if err != nil {
		return respond, gateway.Resolved{}, gateway.ChatUsage{}, nil
	}
	var reply routerReply
	if perr := json.Unmarshal([]byte(stripFences(res.Text)), &reply); perr != nil {
		return respond, resolved, res.Usage, nil
	}
	d := ReadingDecision{
		Decision: reply.Decision, CardID: reply.CardID, Reason: reply.Reason,
		ExampleBlockID: reply.ExampleBlockID, ExampleQuote: reply.ExampleQuote,
		ExampleWhy: reply.ExampleWhy, FollowupPlan: reply.FollowupPlan,
	}
	switch d.Decision {
	case "respond", "hint", "summon":
	default:
		d = respond
	}
	if len(d.FollowupPlan) > 2 {
		d.FollowupPlan = d.FollowupPlan[:2]
	}
	return d, resolved, res.Usage, nil
}

func buildRouterPrompt(in ReadingRouteInput) string {
	var b strings.Builder
	b.WriteString("你是一名批判性阅读教练，正在和学生一起读一篇文章。基于学生此刻的表达和她正在看的原文，判断是否要请出一张“思维卡”，帮助她更深入地读这篇（不是替她下结论）。\n")
	b.WriteString("克制阶梯：多数时候只需正常回答(respond)；表达和某张卡有合理联系但意图还不明确时给轻提示(hint)；表达清楚、且能在原文里找到一处示范句时才正式请出(summon)。一次只请一张。\n")
	b.WriteString("可用的卡（只能从这些里选）：\n")
	for _, c := range in.Catalog {
		b.WriteString("- " + c.CardID + "（" + c.Name + "）：" + c.Trigger + "\n")
	}
	b.WriteString("\n只输出 JSON：{\"decision\":\"respond|hint|summon\",\"card_id\":\"...\",\"reason\":\"给学生看的一句话，说明为什么此刻值得看这张卡\",\"example_block_id\":\"summon时给出示范句所在的block id\",\"example_quote\":\"summon时给出该block里的一句原文（必须逐字来自原文）\",\"example_why\":\"用不超过两句话解释这句为什么适合这张卡\",\"followup_plan\":[\"最多两张后续卡的id\"]}。respond/hint 时 card_id 可留空、example 字段留空。不要输出任何多余文字。")
	return b.String()
}

func buildRouterUser(in ReadingRouteInput) string {
	var b strings.Builder
	if in.StudentText != "" {
		b.WriteString("学生说：" + in.StudentText + "\n")
	}
	if len(in.FocusedSpans) > 0 {
		b.WriteString("她正在看的原文：\n")
		for _, s := range in.FocusedSpans {
			b.WriteString("[" + s.BlockID + "] " + s.Quote + "\n")
		}
	}
	if len(in.RecentTurns) > 0 {
		b.WriteString("最近对话：\n" + strings.Join(in.RecentTurns, "\n") + "\n")
	}
	return b.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run TestRouteReading`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/reading_router.go apps/api/internal/agent/reading_router_test.go
git commit -m "feat(reading): LLM router over the card catalog with respond fallback"
```

---

## Task 3: Restraint gate + example-anchor resolver (pure)

**Files:**
- Create: `apps/api/internal/agent/reading_gate.go`
- Test: `apps/api/internal/agent/reading_gate_test.go`

**Interfaces:**
- Consumes: `ReadingDecision`, `PacingState` (Task 2); `Anchor`, `MaterialBlock` (`agent/anchors.go:17`, `agent/prompt.go:57`); `computeOffsets` (`agent/anchors.go:119`).
- Produces:
```go
const proposalBreathingTurns = 2 // mirrors the demo's PROPOSAL_BREATHING_TURNS
const skipCooldownTurns = 3      // mirrors the demo's SKIP_COOLDOWN_TURNS (see note)
type OrderingGuard struct { AllowCraap, AllowSift bool }
func ApplyReadingGate(d ReadingDecision, pacing PacingState, ordering OrderingGuard) ReadingDecision
func ResolveExampleAnchor(d ReadingDecision, materialID string, blocks []MaterialBlock) (Anchor, bool)
```

> Note on `skipCooldownTurns`: `PacingState` carries `RecentlySkipped` as a set (card ids skipped inside the cooldown window); the turns arithmetic is done by the caller (Task 5) when it builds the set, so the gate only checks membership. The const documents the window the caller uses.

- [ ] **Step 1: Write the failing test** (`reading_gate_test.go`)

```go
package agent

import "testing"

func summon(card string) ReadingDecision {
	return ReadingDecision{Decision: "summon", CardID: card, ExampleBlockID: "b1", ExampleQuote: "因此它必然失败"}
}

func TestApplyReadingGate_OpenCardSuppressesNewSummon(t *testing.T) {
	got := ApplyReadingGate(summon("argument-map"), PacingState{OpenCard: true}, OrderingGuard{AllowCraap: true, AllowSift: true})
	if got.Decision != "respond" {
		t.Fatalf("open card must suppress summon, got %+v", got)
	}
}

func TestApplyReadingGate_BreathingRoomDowngradesSummonToHint(t *testing.T) {
	got := ApplyReadingGate(summon("framing"), PacingState{TurnsSinceLastPropose: 1}, OrderingGuard{})
	if got.Decision != "hint" {
		t.Fatalf("within breathing window summon→hint, got %+v", got)
	}
}

func TestApplyReadingGate_SkipCooldownSuppresses(t *testing.T) {
	got := ApplyReadingGate(summon("cda"), PacingState{RecentlySkipped: []string{"cda"}}, OrderingGuard{})
	if got.Decision != "respond" {
		t.Fatalf("recently-skipped card must be suppressed, got %+v", got)
	}
}

func TestApplyReadingGate_CompletedWithoutNewFocusSuppresses(t *testing.T) {
	got := ApplyReadingGate(summon("toulmin"), PacingState{CompletedCards: []string{"toulmin"}, HasNewFocus: false}, OrderingGuard{})
	if got.Decision != "respond" {
		t.Fatalf("completed card without new focus must be suppressed, got %+v", got)
	}
}

func TestApplyReadingGate_OrderingBlocksSiftBeforeCraap(t *testing.T) {
	got := ApplyReadingGate(summon("sift"), PacingState{}, OrderingGuard{AllowCraap: true, AllowSift: false})
	if got.Decision != "respond" {
		t.Fatalf("sift not allowed yet must be suppressed, got %+v", got)
	}
}

func TestApplyReadingGate_AllowsCleanSummon(t *testing.T) {
	got := ApplyReadingGate(summon("argument-map"), PacingState{TurnsSinceLastPropose: 5, HasNewFocus: true}, OrderingGuard{AllowCraap: true, AllowSift: true})
	if got.Decision != "summon" {
		t.Fatalf("clean summon must pass, got %+v", got)
	}
}

func TestResolveExampleAnchor_RejectsNonVerbatimQuote(t *testing.T) {
	blocks := []MaterialBlock{{ID: "b1", Text: "气候在变化。因此这项政策必然失败。"}}
	_, ok := ResolveExampleAnchor(ReadingDecision{Decision: "summon", ExampleBlockID: "b1", ExampleQuote: "这句不在原文里"}, "m1", blocks)
	if ok {
		t.Fatalf("non-verbatim quote must be rejected")
	}
}

func TestResolveExampleAnchor_ComputesRuneOffsets(t *testing.T) {
	blocks := []MaterialBlock{{ID: "b1", Text: "气候在变化。因此这项政策必然失败。"}}
	a, ok := ResolveExampleAnchor(ReadingDecision{Decision: "summon", CardID: "argument-map", ExampleBlockID: "b1", ExampleQuote: "因此这项政策必然失败", ExampleWhy: "用了必然"}, "m1", blocks)
	if !ok {
		t.Fatalf("verbatim quote must resolve")
	}
	if a.Start != 6 || a.End != 6+len([]rune("因此这项政策必然失败")) {
		t.Fatalf("rune offsets wrong: start=%d end=%d", a.Start, a.End)
	}
	if a.Author != "ai" || a.MaterialID != "m1" || a.BlockID != "b1" || a.Question == "" {
		t.Fatalf("anchor shape wrong: %+v", a)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run 'TestApplyReadingGate|TestResolveExampleAnchor'`
Expected: FAIL — undefined `ApplyReadingGate` / `ResolveExampleAnchor` / `OrderingGuard`.

- [ ] **Step 3: Write the implementation**

```go
package agent

import "strconv"

const (
	proposalBreathingTurns = 2
	skipCooldownTurns      = 3
)

// OrderingGuard is the source-check ordering prior, derived from the graph by
// the caller (Task 5) using SurfaceCardCandidates' rules: don't summon SIFT
// before the material has a CRAAP evaluation; don't summon CRAAP on a material
// already evaluated. The router may not override this.
type OrderingGuard struct {
	AllowCraap bool
	AllowSift  bool
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// ApplyReadingGate is the deterministic, authoritative restraint layer. It can
// only DOWNGRADE the model's decision (summon→hint→respond), never upgrade.
// Order matters: hard suppressions first, breathing-room softening last.
func ApplyReadingGate(d ReadingDecision, pacing PacingState, ordering OrderingGuard) ReadingDecision {
	if d.Decision == "respond" {
		return d
	}
	// One-active mutex: while a card is open, nothing new fires.
	if pacing.OpenCard {
		return respond
	}
	if d.Decision == "summon" {
		// Skip cooldown.
		if contains(pacing.RecentlySkipped, d.CardID) {
			return respond
		}
		// New-span reuse: a completed card won't re-open without new focus.
		if contains(pacing.CompletedCards, d.CardID) && !pacing.HasNewFocus {
			return respond
		}
		// Source-check ordering guard.
		if d.CardID == "sift" && !ordering.AllowSift {
			return respond
		}
		if d.CardID == "craap" && !ordering.AllowCraap {
			return respond
		}
		// Breathing room: no NEW proposal within the window — soften to a hint.
		if pacing.TurnsSinceLastPropose < proposalBreathingTurns {
			return ReadingDecision{Decision: "hint", CardID: d.CardID, Reason: d.Reason}
		}
	}
	return d
}

// ResolveExampleAnchor turns a summon's example (block_id + quote) into a real,
// verbatim-validated L1 anchor. ok=false means the quote is not a verbatim
// substring of the named block — the caller must retry the router once, then
// degrade to respond. NEVER build a (0,0) anchor here (that is the "lights up
// nothing" bug this replaces).
func ResolveExampleAnchor(d ReadingDecision, materialID string, blocks []MaterialBlock) (Anchor, bool) {
	if d.Decision != "summon" || d.ExampleQuote == "" {
		return Anchor{}, false
	}
	var text string
	found := false
	for _, b := range blocks {
		if b.ID == d.ExampleBlockID {
			text, found = b.Text, true
			break
		}
	}
	if !found {
		return Anchor{}, false
	}
	start, end := computeOffsets(text, d.ExampleQuote)
	if end <= start { // (0,0) means "not a substring" — reject.
		return Anchor{}, false
	}
	q := d.ExampleWhy
	if q == "" {
		q = "先看这处示范，再换你在文章里找一句自己的证据。"
	}
	return Anchor{
		ID: "ex0", MaterialID: materialID, BlockID: d.ExampleBlockID,
		Start: start, End: end, Quote: d.ExampleQuote,
		Dimension: d.CardID, Author: "ai", Question: q,
	}, true
}

var _ = strconv.Itoa // reserved for multi-example ids if the deck later needs them
```

> Delete the trailing `var _ = strconv.Itoa` line and the `strconv` import if the reviewer flags it as unused — it is a hook for future multi-example anchors and carries no behavior. (Removing it is the YAGNI-correct choice; keeping the import unused will not compile.)

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run 'TestApplyReadingGate|TestResolveExampleAnchor'`
Expected: PASS (all eight). If Step 3's `strconv` line was removed, also remove the import.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/reading_gate.go apps/api/internal/agent/reading_gate_test.go
git commit -m "feat(reading): downgrade-only restraint gate + verbatim example-anchor resolver"
```

---

## Task 4: Selection evaluator (3-check rubric + program verdict + integrity)

**Files:**
- Create: `apps/api/internal/agent/reading_eval.go`
- Test: `apps/api/internal/agent/reading_eval_test.go`

**Interfaces:**
- Consumes: `gateway.*` (as Task 2), `cards.Spec`, `Anchor`, `stripFences`, `gateway.StubProvider`.
- Produces:
```go
type SelectionCheck struct { Key, Label, Status, Evidence, Explanation string }
type SelectionEval struct {
	Verdict, VerdictLabel, VerdictReason string
	Checks                               []SelectionCheck
	Finding, Judgment, Support, Caveat, NextStep string
	SpanIDs                              []string
}
func verdictFromChecks(checks []SelectionCheck) (verdict, label string)
func EvaluateSelection(ctx context.Context, p gateway.Provider, resolver gateway.KeyResolver, spec cards.Spec, dimension string, studentSpan Anchor) (SelectionEval, gateway.Resolved, gateway.ChatUsage, error)
```
- Check keys are exactly `target` / `evidence` / `centrality` with labels `找对对象` / `看得到线索` / `线索足够关键`.

- [ ] **Step 1: Write the failing test** (`reading_eval_test.go`)

```go
package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

func TestVerdictFromChecks_Rules(t *testing.T) {
	miss := []SelectionCheck{{Key: "target", Status: "miss"}, {Key: "evidence", Status: "pass"}, {Key: "centrality", Status: "pass"}}
	if v, l := verdictFromChecks(miss); v != "rethink" || l != "暂不匹配" {
		t.Fatalf("target miss → rethink, got %s/%s", v, l)
	}
	allPass := []SelectionCheck{{Key: "target", Status: "pass"}, {Key: "evidence", Status: "pass"}, {Key: "centrality", Status: "pass"}}
	if v, l := verdictFromChecks(allPass); v != "strong" || l != "高度匹配" {
		t.Fatalf("all pass → strong, got %s/%s", v, l)
	}
	mixed := []SelectionCheck{{Key: "target", Status: "pass"}, {Key: "evidence", Status: "partial"}, {Key: "centrality", Status: "pass"}}
	if v, l := verdictFromChecks(mixed); v != "partial" || l != "部分匹配" {
		t.Fatalf("mixed → partial, got %s/%s", v, l)
	}
}

func TestEvaluateSelection_VerdictIsProgramOwned_NotModel(t *testing.T) {
	// Model lies: claims "strong" while target is a miss. Program must override.
	script := []gateway.StreamEvent{{Delta: `{"verdict":"strong",` +
		`"checks":[{"key":"target","status":"miss","evidence":"","explanation":"选错了对象"},` +
		`{"key":"evidence","status":"pass","evidence":"必然失败","explanation":"有强词"},` +
		`{"key":"centrality","status":"pass","evidence":"必然失败","explanation":"是关键"}],` +
		`"finding":"这是一个没给证据的结论","judgment":"论证跳跃","support":"用了必然","caveat":"","next_step":"找找它的证据"}`}}
	p := gateway.NewStubProvider(script)
	span := Anchor{ID: "s0", Quote: "因此这项政策必然失败", Dimension: "logic"}
	ev, _, _, err := EvaluateSelection(context.Background(), p, stubResolver(), cards.Spec{ID: "argument-map", Name: "论证地图"}, "logic", span)
	if err != nil {
		t.Fatalf("EvaluateSelection error: %v", err)
	}
	if ev.Verdict != "rethink" {
		t.Fatalf("program must override model's lie to rethink, got %q", ev.Verdict)
	}
	if len(ev.SpanIDs) != 1 || ev.SpanIDs[0] != "s0" {
		t.Fatalf("finding must cite the student's span only, got %+v", ev.SpanIDs)
	}
}

func TestEvaluateSelection_DropsNonVerbatimEvidenceSnippet(t *testing.T) {
	script := []gateway.StreamEvent{{Delta: `{"checks":[` +
		`{"key":"target","status":"pass","evidence":"因此这项政策必然失败","explanation":"对"},` +
		`{"key":"evidence","status":"pass","evidence":"这段文字并不在学生选的句子里","explanation":"x"},` +
		`{"key":"centrality","status":"pass","evidence":"必然失败","explanation":"关键"}],` +
		`"finding":"f","judgment":"j","support":"s","caveat":"","next_step":"n"}`}}
	p := gateway.NewStubProvider(script)
	span := Anchor{ID: "s0", Quote: "因此这项政策必然失败"}
	ev, _, _, _ := EvaluateSelection(context.Background(), p, stubResolver(), cards.Spec{ID: "x"}, "d", span)
	for _, c := range ev.Checks {
		if c.Key == "evidence" && c.Evidence != "" {
			t.Fatalf("non-verbatim evidence snippet must be dropped, got %q", c.Evidence)
		}
	}
}

func TestEvaluateSelection_DeterministicFallbackOnGarbage(t *testing.T) {
	p := gateway.NewStubProvider([]gateway.StreamEvent{{Delta: "garbage"}})
	span := Anchor{ID: "s0", Quote: "因此这项政策必然失败"}
	ev, _, _, err := EvaluateSelection(context.Background(), p, stubResolver(), cards.Spec{ID: "x"}, "d", span)
	if err != nil {
		t.Fatalf("garbage must not error (deterministic fallback keeps the loop alive): %v", err)
	}
	if len(ev.Checks) != 3 || ev.Verdict == "" {
		t.Fatalf("fallback must still produce 3 checks + a verdict, got %+v", ev)
	}
	if len(ev.SpanIDs) != 1 || ev.SpanIDs[0] != "s0" {
		t.Fatalf("fallback must still cite the student's span, got %+v", ev.SpanIDs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run 'TestVerdictFromChecks|TestEvaluateSelection'`
Expected: FAIL — undefined `verdictFromChecks` / `EvaluateSelection` / `SelectionEval`.

- [ ] **Step 3: Write the implementation**

```go
package agent

import (
	"context"
	"encoding/json"
	"strings"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

// (types SelectionCheck, SelectionEval — as in Interfaces)

var checkLabels = map[string]string{"target": "找对对象", "evidence": "看得到线索", "centrality": "线索足够关键"}

var verdictLabels = map[string]string{"strong": "高度匹配", "partial": "部分匹配", "rethink": "暂不匹配"}

// verdictFromChecks derives the grade from the three checks — PROGRAM-owned, the
// model's own "verdict" field is ignored. target miss → rethink; all pass →
// strong; else partial (spec §11).
func verdictFromChecks(checks []SelectionCheck) (string, string) {
	byKey := map[string]string{}
	for _, c := range checks {
		byKey[c.Key] = c.Status
	}
	if byKey["target"] == "miss" {
		return "rethink", verdictLabels["rethink"]
	}
	if byKey["target"] == "pass" && byKey["evidence"] == "pass" && byKey["centrality"] == "pass" {
		return "strong", verdictLabels["strong"]
	}
	return "partial", verdictLabels["partial"]
}

type evalReply struct {
	Checks []struct {
		Key, Status, Evidence, Explanation string
	} `json:"checks"`
	Finding, Judgment, Support, Caveat, NextStep string
}

// normalizeStatus clamps the model's status onto the closed set.
func normalizeStatus(s string) string {
	switch s {
	case "pass", "partial", "miss":
		return s
	default:
		return "partial"
	}
}

// EvaluateSelection judges the student's picked sentence against the active
// card's lens using the uniform 3-check rubric. The verdict is recomputed from
// the checks (never trusted from the model); evidence snippets that are not
// verbatim from the student's span are dropped; the finding may cite ONLY the
// student's span. On any model/parse failure it degrades to a deterministic
// evaluator so the loop stays alive.
func EvaluateSelection(ctx context.Context, p gateway.Provider, resolver gateway.KeyResolver, spec cards.Spec, dimension string, studentSpan Anchor) (SelectionEval, gateway.Resolved, gateway.ChatUsage, error) {
	resolved, err := resolver(ctx)
	if err != nil {
		return fallbackEval(studentSpan), gateway.Resolved{}, gateway.ChatUsage{}, nil
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildEvalPrompt(spec, dimension)},
			{Role: gateway.RoleUser, Content: "学生从文章里选的句子：「" + studentSpan.Quote + "」"},
		},
		MaxTokens: 700,
	}
	res, err := gateway.Collect(ctx, p, resolved, req)
	if err != nil {
		return fallbackEval(studentSpan), gateway.Resolved{}, gateway.ChatUsage{}, nil
	}
	var reply evalReply
	if perr := json.Unmarshal([]byte(stripFences(res.Text)), &reply); perr != nil || len(reply.Checks) == 0 {
		return fallbackEval(studentSpan), resolved, res.Usage, nil
	}
	checks := make([]SelectionCheck, 0, 3)
	for _, c := range reply.Checks {
		ev := ""
		if c.Evidence != "" && strings.Contains(studentSpan.Quote, c.Evidence) { // integrity: verbatim only
			ev = c.Evidence
		}
		checks = append(checks, SelectionCheck{
			Key: c.Key, Label: checkLabels[c.Key], Status: normalizeStatus(c.Status),
			Evidence: ev, Explanation: c.Explanation,
		})
	}
	verdict, label := verdictFromChecks(checks)
	return SelectionEval{
		Verdict: verdict, VerdictLabel: label, VerdictReason: reply.Finding,
		Checks:  checks,
		Finding: reply.Finding, Judgment: reply.Judgment, Support: reply.Support,
		Caveat:  reply.Caveat, NextStep: reply.NextStep,
		SpanIDs: []string{studentSpan.ID}, // integrity: the student's span only
	}, resolved, res.Usage, nil
}

func buildEvalPrompt(spec cards.Spec, dimension string) string {
	return "你是一名批判性阅读教练。学生用「" + spec.Name + "」这张卡，从文章里选了一句她认为相关的证据。" +
		"针对维度「" + dimension + "」，评估她的选句，只输出 JSON：\n" +
		"{\"checks\":[{\"key\":\"target\",\"status\":\"pass|partial|miss\",\"evidence\":\"必须逐字来自她选的句子\",\"explanation\":\"一句话\"}," +
		"{\"key\":\"evidence\",\"status\":\"...\",\"evidence\":\"...\",\"explanation\":\"...\"}," +
		"{\"key\":\"centrality\",\"status\":\"...\",\"evidence\":\"...\",\"explanation\":\"...\"}]," +
		"\"finding\":\"她这句读出了什么\",\"judgment\":\"她的论断\",\"support\":\"支撑\",\"caveat\":\"保留\",\"next_step\":\"下一步只做一件事\"}\n" +
		"target=她是否找对了对象；evidence=句子里有没有可直接引用的线索；centrality=线索是否足够关键。不要输出多余文字。"
}

// fallbackEval keeps the loop alive with a neutral, honest partial verdict when
// the model is unavailable. It never fabricates specifics — evidence snippets
// stay empty and the finding is generic.
func fallbackEval(studentSpan Anchor) SelectionEval {
	checks := []SelectionCheck{
		{Key: "target", Label: checkLabels["target"], Status: "partial", Explanation: "先记下你的选择，我们一起再看这句和任务的关系。"},
		{Key: "evidence", Label: checkLabels["evidence"], Status: "partial", Explanation: "看看这句里最关键的词是哪一个。"},
		{Key: "centrality", Label: checkLabels["centrality"], Status: "partial", Explanation: "这条线索足以支撑你的判断吗？"},
	}
	verdict, label := verdictFromChecks(checks)
	return SelectionEval{
		Verdict: verdict, VerdictLabel: label,
		VerdictReason: "先把你的发现记下来。", Checks: checks,
		Finding: "你选了这句作为证据。", NextStep: "回到文章，标出这句里最关键的一处线索。",
		SpanIDs: []string{studentSpan.ID},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run 'TestVerdictFromChecks|TestEvaluateSelection'`
Expected: PASS (all four).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/reading_eval.go apps/api/internal/agent/reading_eval_test.go
git commit -m "feat(reading): selection evaluator with program-owned verdict + evidence integrity"
```

---

## Task 5: Backend endpoints — read-turn (SSE) + evaluate (JSON)

**Files:**
- Create: `apps/api/internal/api/readturn.go`, `apps/api/internal/api/readeval.go`
- Modify: `apps/api/internal/api/api.go:69-86` (route registration)
- Test: `apps/api/internal/api/reading_endpoints_test.go`

**Interfaces:**
- Consumes: `RouteReading`, `ApplyReadingGate`, `ResolveExampleAnchor`, `EvaluateSelection`, `ReadingDeck`, `OrderingGuard`, `PacingState`, `FocusSpan` (Tasks 1-4); `SurfaceCardCandidates` + `craapCardID`/`siftCardID` (`classifier.go`); `store.LoadGraph`, `store.CreateCardInstance`, `store.GetCardInstance`, `store.SetCardInstanceAnchors`, `store.CountCompletedCardUsesByUser` (`agentstore.go`); `GuidanceFor` (`guidance.go`); `studioEmitter` + `em.Card`/`em.Intervention`/`em.Text` (`studioturn.go:28-99`, `gateway/sse.go`); `cards.ByID`; `agent.Material`/`MaterialBlock`; the flagship resolver `gateway.NewEvalKeyResolver(a.cfg)` (`keyresolver.go:49`).
- Produces routes:
  - `POST /api/v1/projects/{id}/materials/{mid}/read-turn` → `a.postReadingTurn` (SSE)
  - `POST /api/v1/projects/{id}/cards/{cid}/evaluate` → `a.evaluateProjectCard` (JSON)

**Behavior contract — `postReadingTurn`:**
1. Parse `{id}`,`{mid}`, load project + graph (`store.LoadGraph`), verify `mid` is a material in the project (mirror `prepareSourceAnnotation`, `materials.go:276-345`).
2. Parse body `{ student_text string, focused_spans []{block_id,quote} }`.
3. Build `ReadingRouteInput`: `Catalog` = `ReadingDeck()`; `ScaffoldLevels[cardID]` = `store.CountCompletedCardUsesByUser(ctx,u.ID,cardID)` for each deck id; `Pacing` from the graph — `OpenCard` = any card instance `proposed|active`; `RecentlySkipped`/`CompletedCards` from card-instance statuses on this material; `HasNewFocus` = `len(focused_spans)>0`; `TurnsSinceLastPropose` = a large constant for now (no per-turn counter persisted — document this as the one pacing input approximated; breathing-room still fires within a single session because `OpenCard` covers the common case).
4. `OrderingGuard` from `SurfaceCardCandidates(g)` restricted to `mid`: `AllowCraap` = a craap candidate fired for `mid`; `AllowSift` = a sift candidate fired for `mid`.
5. `RouteReading(...)` → record LLM call if `resolved.Provider != ""` (reuse the existing `a.recordLLMCall` path used by `surfaceAnchors`; grep `studioturn.go` for how it records — replicate exactly).
6. `ApplyReadingGate(...)`. If `summon`: `ResolveExampleAnchor(...)`; if `!ok`, retry `RouteReading` **once**, re-resolve; if still `!ok`, downgrade to `respond`.
7. On `summon`: `CreateCardInstance(ctx, projectID, materialUUID, cardID, "")` → `SetCardInstanceAnchors(...)` with the single example anchor (JSON array) → `em.Card(instanceID, cardID, spec.Name, anchorsJSON, mid)`.
8. On `hint`: `em.Intervention("", decision.Reason, "", decision.CardID, "hint")`.
9. On `respond`: `em.Text(...)` with a short coach reply (or just `em.Done`). Always end with the SSE `done` frame.

**Behavior contract — `evaluateProjectCard`:**
1. Parse `{id}`,`{cid}`, load the card instance (`GetCardInstance`), verify project ownership.
2. Parse body `{ block_id, start, end, quote, dimension }` → build `studentSpan := agent.Anchor{ID:"sel0", MaterialID:<ci.material>, BlockID, Start, End, Quote, Dimension, Author:"student"}`.
3. `spec, _ := cards.ByID(ci.CardID)`.
4. `EvaluateSelection(ctx, a.provider, gateway.NewEvalKeyResolver(a.cfg), spec, dimension, studentSpan)` → record LLM call if `resolved.Provider != ""`.
5. Persist the eval + the student span onto the instance for the later confirm/mint: marshal `SelectionEval` and `SetCardInstanceFramework(ctx, projectID, ci.ID, evalJSON)` (framework column already exists, `agentstore.go:301`). Do NOT flip status — confirm/save is the existing `submitProjectCard`.
6. Respond `200 application/json` with the `SelectionEval` (camelCase — mirror the `MaterialSource` response convention; the TS side Zod-parses it, Task 8).

- [ ] **Step 1: Write the failing test** (`reading_endpoints_test.go`) — table-driven HTTP test using the existing api-package test harness. Mirror an existing endpoint test that spins a test server with a stub provider and a real/seeded store; grep `apps/api/internal/api/*_test.go` for the harness that builds an `*API` with `NewStubProvider` (e.g. `materials_test.go` or `studioturn_test.go`). Assert:

```go
// Pseudocode shape — fill with the real harness constructor found in the api tests.
func TestPostReadingTurn_SummonEmitsCardFrame(t *testing.T) {
	// stub router reply → summon argument-map with a verbatim example quote
	// POST /materials/{mid}/read-turn
	// assert an SSE "card" frame with card_id "argument-map" and a non-empty anchors array
}
func TestPostReadingTurn_OpenCardSuppresses(t *testing.T) {
	// seed a proposed card instance → assert no new "card" frame (respond/done only)
}
func TestEvaluateProjectCard_ReturnsProgramVerdict(t *testing.T) {
	// stub eval reply claiming "strong" while target=miss
	// POST /cards/{cid}/evaluate → assert JSON body verdict == "rethink"
}
```

> If no reusable api-test server harness exists, the pure logic is already covered by Tasks 2-4; in that case make these thin handler tests using `httptest.NewRequest`/`ResponseRecorder` against `a.postReadingTurn` with an `*API` built from the same fixtures other api tests use. Do NOT add testcontainers if the existing api tests avoid them for handler-shape assertions.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run 'TestPostReadingTurn|TestEvaluateProjectCard'` (subagent: `timeout: 600000`)
Expected: FAIL — undefined handlers / 404 routes.

- [ ] **Step 3: Implement the two handlers + register routes**

Write `readturn.go` and `readeval.go` per the behavior contracts above, copying the request-parsing, project-load, ownership-check, `recordLLMCall`, and `studioEmitter` setup **verbatim from `studioturn.go`/`materials.go`** (do not invent new patterns). Register in `api.go` right after the existing card routes:

```go
	mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/read-turn", protected(a.postReadingTurn))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/evaluate", protected(a.evaluateProjectCard))
```

- [ ] **Step 4: Run the tests + the full api package**

Run: `cd apps/api && go test ./internal/api/` (subagent: `timeout: 600000`)
Expected: PASS, no regressions.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/readturn.go apps/api/internal/api/readeval.go apps/api/internal/api/api.go apps/api/internal/api/reading_endpoints_test.go
git commit -m "feat(reading): read-turn (SSE summon) + evaluate (JSON) endpoints"
```

---

## Task 6: Contracts — SelectionEval + request bodies

**Files:**
- Create: `packages/contracts/src/reading.ts`
- Modify: `packages/contracts/src/index.ts` (export the new schemas)
- Test: `packages/contracts/test/reading.test.ts` (mirror an existing contracts test)

**Interfaces:**
- Produces (Zod + inferred TS types):
```ts
export const SelectionCheck = z.object({ key: z.string(), label: z.string(), status: z.enum(["pass","partial","miss"]), evidence: z.string(), explanation: z.string() });
export const SelectionEval = z.object({
  verdict: z.enum(["strong","partial","rethink"]), verdictLabel: z.string(), verdictReason: z.string(),
  checks: z.array(SelectionCheck), finding: z.string(), judgment: z.string(), support: z.string(),
  caveat: z.string(), nextStep: z.string(), spanIds: z.array(z.string()),
});
export const ReadTurnBody = z.object({ student_text: z.string(), focused_spans: z.array(z.object({ block_id: z.string(), quote: z.string() })) });
export const EvaluateBody = z.object({ block_id: z.string(), start: z.number().int(), end: z.number().int(), quote: z.string(), dimension: z.string() });
```

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { SelectionEval } from "../src/reading";

describe("SelectionEval", () => {
  it("parses a program-verdict payload", () => {
    const ok = SelectionEval.parse({
      verdict: "rethink", verdictLabel: "暂不匹配", verdictReason: "对象没找对",
      checks: [{ key: "target", label: "找对对象", status: "miss", evidence: "", explanation: "x" }],
      finding: "f", judgment: "j", support: "s", caveat: "", nextStep: "n", spanIds: ["s0"],
    });
    expect(ok.verdict).toBe("rethink");
  });
  it("rejects an unknown verdict", () => {
    expect(() => SelectionEval.parse({ verdict: "amazing" } as any)).toThrow();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/contracts && pnpm test -- reading`
Expected: FAIL — cannot find `../src/reading`.

- [ ] **Step 3: Write `reading.ts` (schemas above) + add `export * from "./reading";` to `index.ts`.** Confirm the JSON the Go handler emits is camelCase (`verdictLabel`, `nextStep`, `spanIds`) to match this schema — the Go response struct tags in Task 5 must serialize to exactly these keys.

- [ ] **Step 4: Run test + typecheck**

Run: `cd packages/contracts && pnpm test && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/reading.ts packages/contracts/src/index.ts packages/contracts/test/reading.test.ts
git commit -m "feat(reading): SelectionEval + read-turn/evaluate contract schemas"
```

---

## Task 7: API client — readTurn + evaluateCardSelection

**Files:**
- Create: `apps/web/src/api/reading.ts`
- Modify: `apps/web/src/api/index.ts` (add to `ApiClient` interface + `api` object)
- Test: `apps/web/test/api/reading.test.ts`

**Interfaces:**
- Consumes: `apiFetch`, `API_BASE` (`api/client.ts`), `parseSSE` (`api/sse.ts`), `mapStudioFrame`/`StudioTurnEvent` (`api/studioTurn.ts`), `SelectionEval` (Task 6).
- Produces:
```ts
export async function* readTurn(projectId: string, materialId: string, body: { student_text: string; focused_spans: { block_id: string; quote: string }[] }): AsyncGenerator<StudioTurnEvent>;
export async function evaluateCardSelection(projectId: string, cid: string, body: { block_id: string; start: number; end: number; quote: string; dimension: string }): Promise<SelectionEval>;
```

- [ ] **Step 1: Write the failing test** — mirror an existing `apps/web/test/api/*.test.ts` (mock `fetch`/`apiFetch`). Assert `readTurn` yields a mapped `card` event from an SSE stream and `evaluateCardSelection` Zod-parses the JSON.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && pnpm test -- reading`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement** — `readTurn` copies the streaming generator pattern verbatim from `submitProjectCard` (`api/projectCards.ts:17-38`), POSTing to `/api/v1/projects/${projectId}/materials/${materialId}/read-turn`. `evaluateCardSelection` copies the `apiFetch` + `.parse` pattern from `addMaterial` (`api/materials.ts:11-19`), POSTing to `/api/v1/projects/${projectId}/cards/${cid}/evaluate` and returning `SelectionEval.parse(raw)`. Add both to the `ApiClient` interface (`api/index.ts:59-86`) and the `api` const (`:123-143`).

- [ ] **Step 4: Run test + typecheck**

Run: `cd apps/web && pnpm test -- reading && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/api/reading.ts apps/web/src/api/index.ts apps/web/test/api/reading.test.ts
git commit -m "feat(reading): web api client for read-turn + evaluate"
```

---

## Task 8: ReadingRoom surface + view-swap + article render (no card yet)

**Files:**
- Create: `apps/web/src/studio/reading/ReadingRoom.tsx`, `apps/web/src/studio/reading/ReadingRoom.css`
- Modify: `apps/web/src/studio/StudioContainer.tsx` (add `readingMaterialId` state + view-swap + back), `apps/web/src/studio/material/SourceDossier.tsx` (list-row click sets reading material instead of opening article-mode in place)
- Test: `apps/web/test/studio/reading/ReadingRoom.test.tsx`, and extend `apps/web/test/studio/StudioContainer.test.tsx`

**Interfaces:**
- Consumes: `Annotate` + `anchorToSpan` (moved/imported from `SourceDossier`), `MaterialSource`/`Anchor` (contracts).
- Produces:
```ts
export type ReadingRoomProps = {
  source: MaterialSource;
  onBack: () => void;
  // loop props added in Task 10; this task renders article + coach shell + back only
};
export function ReadingRoom(props: ReadingRoomProps): JSX.Element;
```
- In `StudioContainer`: `const [readingMaterialId, setReadingMaterialId] = useState<string | null>(null);` a source-list click sets it; when set (and the source exists in `state.views.material.sources`), render `<ReadingRoom source={...} onBack={() => { setReadingMaterialId(null); refetchProject(); }} />` **instead of** `<StudioShell .../>` (mirror the `openId==null ? <Directory> : <studio>` swap at `StudioContainer.tsx:502-505`).

- [ ] **Step 1: Write the failing test** (`ReadingRoom.test.tsx`)

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { MaterialSource } from "@mind-imprint/contracts";
import { ReadingRoom } from "@/studio/reading/ReadingRoom";

const SOURCE: MaterialSource = {
  id: "m1", title: "NASA 气候报告", sourceUrl: "", kind: "article", origin: "nasa.gov",
  blocks: [{ id: "b0", text: "全球平均气温持续上升。" }, { id: "b1", text: "因此这项政策必然失败。" }],
  locked: false, role: "", tier: "", takeaway: "", anchors: [], timeSpentS: 0,
  lateralRead: false, isLateralInstrument: false, siftSkipped: false, lateralRelation: "", lateralJudgment: "",
};

describe("ReadingRoom", () => {
  it("renders the article and returns via back", () => {
    const onBack = vi.fn();
    render(<ReadingRoom source={SOURCE} onBack={onBack} />);
    expect(screen.getByText("NASA 气候报告")).toBeInTheDocument();
    expect(screen.getByText(/全球平均气温持续上升/)).toBeInTheDocument();
    fireEvent.click(screen.getByText(/返回工作区/));
    expect(onBack).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && pnpm test -- ReadingRoom`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement `ReadingRoom`** — a two-column surface. Left: a `← 返回工作区` button + a thin coach column placeholder (`<div className="coach-col">`). Right: the article — reuse `<Annotate blocks={source.blocks} state={{material_id: source.id, spans: source.anchors.map(anchorToSpan).filter(Boolean)}} activeSpanId={null} onSelectSpan={()=>{}} />`. Add `ReadingRoom.css` with the `~39:61` grid (`grid-template-columns: minmax(320px,.78fr) minmax(600px,1.22fr)`), serif article (`--content-width:736px; font: 17px/1.95 serif-stack`), and the single-column collapse `@media (max-width:980px)`. Move `anchorToSpan` to a shared import (export it from `SourceDossier` or a small `annotateSpan.ts`; keep one definition — DRY). Wire the view-swap + source-list click in `StudioContainer`/`SourceDossier`.

- [ ] **Step 4: Run test + typecheck + the existing StudioContainer test**

Run: `cd apps/web && pnpm test -- ReadingRoom StudioContainer && pnpm typecheck`
Expected: PASS (extend `StudioContainer.test.tsx` to assert clicking a source row swaps to the reading room — `screen.getByText(/返回工作区/)`).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/reading/ apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/material/SourceDossier.tsx apps/web/test/studio/
git commit -m "feat(reading): ReadingRoom surface + view-swap + article render"
```

---

## Task 9: Inline hanging card + anchor-move

**Files:**
- Create: `apps/web/src/studio/reading/HangingCard.tsx`
- Modify: `apps/web/src/studio/reading/ReadingRoom.tsx` (render the card inline, after the anchor paragraph), `ReadingRoom.css` (connector line)
- Test: `apps/web/test/studio/reading/HangingCard.test.tsx`

**Interfaces:**
- Produces:
```ts
export type HangingCardStatus = "proposed" | "active" | "evaluating" | "feedback";
export type HangingCardProps = {
  cardName: string;
  status: HangingCardStatus;
  exampleWhy: string;          // AI's explanation of the example (proposed)
  eval?: SelectionEval | null; // feedback
  onStartPick: () => void;     // proposed → active
  onConfirm: () => void;       // feedback → completed
  onRepick: () => void;        // feedback → active
};
export function HangingCard(props: HangingCardProps): JSX.Element;
// exported helper — the signature move:
export function anchorBlockId(exampleBlockId: string, studentBlockId: string | null, status: HangingCardStatus): string;
```
- `anchorBlockId`: returns `studentBlockId` when `studentBlockId` is set AND status ∈ {`active`,`evaluating`,`feedback`}; else `exampleBlockId`. (Demo `app.js:422-424`.) This decides which paragraph the card hangs under.

- [ ] **Step 1: Write the failing test**

```tsx
import { describe, it, expect } from "vitest";
import { anchorBlockId } from "@/studio/reading/HangingCard";

describe("anchorBlockId (the anchor-move)", () => {
  it("hangs under the AI example while proposed", () => {
    expect(anchorBlockId("bEx", "bStu", "proposed")).toBe("bEx");
  });
  it("moves under the student's block once picking/active", () => {
    expect(anchorBlockId("bEx", "bStu", "active")).toBe("bStu");
    expect(anchorBlockId("bEx", "bStu", "feedback")).toBe("bStu");
  });
  it("stays on the example if the student has not picked yet", () => {
    expect(anchorBlockId("bEx", null, "active")).toBe("bEx");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && pnpm test -- HangingCard`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement `HangingCard`** (the `anchorBlockId` helper exactly as specified, plus the card body with **one** primary button per status — `proposed`: "看懂示范，开始选句" → `onStartPick`; `active`: instruction "在文章里点出你自己的证据句" (no primary button; selection drives it); `evaluating`: a spinner "印记正在看你的选择…"; `feedback`: the 3 checks + verdict + `nextStep`, primary "记下这条发现" → `onConfirm`, secondary "重新选一句" → `onRepick`). Method notes / example-why in a collapsed `<details>`. In `ReadingRoom`, render `<HangingCard>` immediately after the paragraph whose block id equals `anchorBlockId(...)`, with a `.lens-connector` line (CSS: a 2px vertical rule + dot). Only ever render ONE card (focus mandate).

- [ ] **Step 4: Run test + typecheck**

Run: `cd apps/web && pnpm test -- HangingCard && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/reading/HangingCard.tsx apps/web/src/studio/reading/ReadingRoom.tsx apps/web/src/studio/reading/ReadingRoom.css apps/web/test/studio/reading/HangingCard.test.tsx
git commit -m "feat(reading): inline hanging card that moves from example to student pick"
```

---

## Task 10: Reading loop controller — wire the full cycle

**Files:**
- Create: `apps/web/src/studio/reading/readingLoop.ts`
- Modify: `apps/web/src/studio/reading/ReadingRoom.tsx` (consume the loop; wire select-your-own-sentence + submit + confirm)
- Test: `apps/web/test/studio/reading/readingLoop.test.ts`, `apps/web/test/studio/reading/ReadingRoom.loop.test.tsx`

**Interfaces:**
- Consumes: `readTurn`, `evaluateCardSelection` (Task 7); `activateProjectCard`, `submitProjectCard`, `skipProjectCard` (existing `api/projectCards.ts`); `SelectionEval`, `Anchor` (contracts); `Annotate` select-mode (`onCreateSpan`, `CreatedSpan`).
- Produces a `useReadingLoop(projectId, source, api)` hook returning `{ status, cardName, exampleAnchor, exampleWhy, studentSpan, eval, sendTurn(text), startPick(), pickSentence(span), confirm(), repick(), skip() }`. `status` is the client state machine (`idle`→`proposed`→`active`→`evaluating`→`feedback`→`idle`).

**State machine (the loop, spec §9):**
- `sendTurn(text)`: call `readTurn(...)`; on a `card` event → set `status:"proposed"`, store the example anchor + `exampleWhy` (nudge). On `intervention` → append a coach hint (stay `idle`). On `done`/`respond` → stay `idle`.
- `startPick()`: `activateProjectCard(projectId, cardInstanceId)` → `status:"active"` (article enters select-mode for every non-example sentence).
- `pickSentence(span)`: store `studentSpan`; `status:"evaluating"`; call `evaluateCardSelection(projectId, cid, {block_id:span.blockId,start:span.start,end:span.end,quote:span.text,dimension:cardId})` → set `eval`, `status:"feedback"`.
- `confirm()`: `submitProjectCard(projectId, cid, { field_values:{}, event_trace:[], anchors:[studentSpanAsAnchor] })` (drain the generator; on `done` with `cardStatus:"completed"`) → `status:"idle"`, clear card, refetch. Offer the next queued follow-up lens if any (store `followupPlan`).
- `repick()`: `status:"active"`, clear `studentSpan`+`eval`.
- Guardrail: example sentence is not selectable (reject a pick whose block+range equals the example); single active card only.

- [ ] **Step 1: Write the failing test** (`readingLoop.test.ts`) — drive the hook with a fake `api` (object literal, `as any`) whose `readTurn` yields a `card` event and `evaluateCardSelection` resolves a `rethink` eval; assert the status transitions `idle→proposed→active→evaluating→feedback` and that `confirm()` calls `submitProjectCard` and returns to `idle`. Use `@testing-library/react`'s `renderHook` + `act`.

```ts
import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useReadingLoop } from "@/studio/reading/readingLoop";

const fakeApi = {
  readTurn: async function* () {
    yield { type: "card", cardInstanceId: "ci1", cardId: "argument-map", nudgeText: "这句像结论没给证据", anchors: [{ id: "ex0", material_id: "m1", block_id: "b1", start: 0, end: 10, quote: "因此这项政策必然失败", dimension: "argument-map", author: "ai", question: "先看这处示范", answer: "" }], materialId: "m1" };
    yield { type: "done" };
  },
  activateProjectCard: vi.fn(async () => {}),
  evaluateCardSelection: vi.fn(async () => ({ verdict: "rethink", verdictLabel: "暂不匹配", verdictReason: "", checks: [], finding: "f", judgment: "", support: "", caveat: "", nextStep: "n", spanIds: ["sel0"] })),
  submitProjectCard: async function* () { yield { type: "done", cardStatus: "completed" }; },
} as any;

describe("useReadingLoop", () => {
  it("runs proposed→active→evaluating→feedback→idle", async () => {
    const { result } = renderHook(() => useReadingLoop("p1", { id: "m1", blocks: [{ id: "b1", text: "因此这项政策必然失败" }] } as any, fakeApi));
    await act(async () => { await result.current.sendTurn("这段怪怪的"); });
    expect(result.current.status).toBe("proposed");
    await act(async () => { await result.current.startPick(); });
    expect(result.current.status).toBe("active");
    await act(async () => { await result.current.pickSentence({ blockId: "b1", start: 0, end: 5, text: "因此这项政策必然失败" }); });
    expect(result.current.status).toBe("feedback");
    expect(result.current.eval?.verdict).toBe("rethink");
    await act(async () => { await result.current.confirm(); });
    expect(result.current.status).toBe("idle");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && pnpm test -- readingLoop`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement `useReadingLoop`** per the state machine, and wire it into `ReadingRoom`: the coach composer calls `sendTurn`; the article `<Annotate selectMode={status==="active" ? {dimension:cardId,onCancel:repick} : null} onCreateSpan={pickSentence} />`; `HangingCard` gets `status`/`eval`/`onStartPick`/`onConfirm`/`onRepick`. Add a `ReadingRoom.loop.test.tsx` that renders the room with the fake api and walks the visible flow (example appears → click "开始选句" → select a sentence → feedback shows verdict → click "记下这条发现").

- [ ] **Step 4: Run tests + typecheck**

Run: `cd apps/web && pnpm test -- reading && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/reading/readingLoop.ts apps/web/src/studio/reading/ReadingRoom.tsx apps/web/test/studio/reading/
git commit -m "feat(reading): read-together loop controller wired end-to-end"
```

---

## Task 11: Retire the old reading path + reconcile sift_craap identity

**Files:**
- Modify: `apps/web/src/studio/material/SourceDossier.tsx` (delete article-mode block `:316-424` + `activateSource` article path; keep list mode + `AddSourceForm` + `SourceLog`), `apps/web/src/studio/CoachRail.tsx` (remove the reading-path card mount for `annotate`/`compare` + `CraapPlaceholder` when `activeView === "素材"`; the studio no longer summons reading cards into the rail), `apps/web/src/studio/ViewFrame.tsx` (the 素材 view renders the source LIST only; the compare-mode dossier branch `:307-314` is removed — SIFT now runs in the reading room)
- Modify/verify: `apps/api/internal/agent/classifier.go` — confirm `SurfaceCardCandidates` no longer needs to fire craap/sift on the studio turn path for reading (it stays as the ordering guard consumed by Task 5; the studio `postProjectTurn` should NOT also surface these into the rail — verify and, if it does, gate it off for deck cards).
- Reconcile: the `sift_craap` triple identity — pick ONE of: (a) keep `craap`/`sift` as the interaction and repoint `cardMeth` (`projection.go:295-304`) so the methodology text shown matches the split cards; or (b) document that `sift_craap` remains ONLY as course/chat content, never summoned on the reading path. Do NOT leave a student seeing `sift_craap` methodology for a `craap` interaction.
- Test: update `apps/web/test/studio/material/SourceDossier.test.tsx` (article-mode assertions move to `ReadingRoom.test.tsx`); update any `CoachRail`/`ViewFrame` tests that asserted the old mount.

**Interfaces:** none new — this is deletion + reconciliation. The constraint: nothing that was reachable before is silently lost — SIFT/CRAAP now run in the reading room; the source list, add-source, and source-log stay in the studio.

- [ ] **Step 1: Run the existing suites to capture the pre-change baseline**

Run: `cd apps/web && pnpm test` and `cd apps/api && go test ./...` (subagent: `timeout: 600000`)
Expected: green baseline (note any pre-existing failures so they aren't attributed to this task).

- [ ] **Step 2: Delete the article-mode/rail-mount/compare-dossier code paths** listed above. For each deletion, update or move the covering test rather than deleting coverage.

- [ ] **Step 3: Reconcile `sift_craap`** — implement the chosen option (a) or (b); add/adjust a test on `projection.go`'s `cardMeth` (Go) or the methodology modal (web) asserting a `craap` interaction shows craap-consistent methodology.

- [ ] **Step 4: Run FULL suites**

Run: `cd apps/web && pnpm test && pnpm typecheck` and `cd apps/api && go test ./...` (subagent: `timeout: 600000`)
Expected: PASS. No orphaned imports (`CraapPlaceholder`, `StudioAnnotateCard` on the reading path) — typecheck must be clean.

- [ ] **Step 5: Commit**

```bash
git add -A apps/web/src/studio apps/web/test/studio apps/api/internal/agent apps/api/internal/studio
git commit -m "refactor(reading): retire in-studio reading path; reconcile sift_craap identity"
```

---

## Task 12: Full-suite verification + focus/integrity audit

**Files:** none (verification task); fix-forward any regressions in the touched files.

- [ ] **Step 1: Backend — full packages**

Run: `cd apps/api && go test ./...` (subagent: `timeout: 600000`)
Expected: PASS. Specifically confirm `./internal/agent` and `./internal/api` green.

- [ ] **Step 2: Web + contracts — full suites + typecheck**

Run: `cd packages/contracts && pnpm test && pnpm typecheck` then `cd apps/web && pnpm test && pnpm typecheck`
Expected: PASS.

- [ ] **Step 3: Focus + integrity manual audit** (checklist against spec §8/§11 — assert each in code or a test):
  - Only ONE card renders in the reading room at any time (grep `HangingCard` render sites; must be a single conditional).
  - The deck (15 ids) is never rendered as a list to the student (no `ReadingDeckIDs.map` in any component).
  - `ResolveExampleAnchor` rejects non-verbatim quotes (Task 3 test) — no `(0,0)` anchor reaches the client.
  - Verdict is program-owned (Task 4 test) — the model's `verdict` field is never read into `SelectionEval.verdict`.
  - `SelectionEval.spanIds` contains only the student's span (Task 4 test).

- [ ] **Step 4: Commit any fixes**

```bash
git add -A
git commit -m "test(reading): full-suite green + focus/integrity audit"
```

- [ ] **Step 5: Finish the branch** — invoke `superpowers:finishing-a-development-branch` (verify tests → present merge/PR options). Per project convention, reading-track work has been direct-merged to `main` after a green whole-branch review; confirm with the user before merging.

---

## Self-Review (against the spec)

**Spec coverage:**
- §7 page/navigation → Task 8 (view-swap, no router; back to studio).
- §8 focus layout → Tasks 8 (grid/serif), 9 (inline card, one primary button), 12 (audit).
- §9 loop + anchor-move → Task 9 (`anchorBlockId`), Task 10 (state machine).
- §10 router → Tasks 1 (catalog), 2 (`RouteReading`), 3 (gate + example validation), 5 (endpoint wiring, ordering guard, retry-once).
- §11 evaluation/integrity → Task 4 (3 checks, `verdictFromChecks`, evidence filter, fallback), Task 12 (audit).
- §12 reading outcome → Task 5 (persist eval on instance), Task 10 (`confirm()` → `submitProjectCard` mints via existing lifecycle).
- §13 data model (no schema change) → reused `card_instances.anchors`/`framework`, no migration. ✓
- §14 error handling → Task 2 (respond fallback), Task 3 (example retry→respond), Task 4 (deterministic eval fallback), Task 5 (best-effort like `prepareSourceAnnotation`).
- §16 seams / §6 retire → Task 11.
- §18 deck → Task 1 (exact 15 ids).

**Placeholder scan:** Task 5's `TurnsSinceLastPropose` is documented as an intentional approximation (no persisted per-turn counter; mutex covers the common case), not a TODO. Task 3's `strconv` hook has an explicit remove-if-flagged instruction. Task 5's HTTP test is prose-shaped because the exact api-test harness constructor must be read from the repo — flagged with the fallback (`httptest` handler test) so there is no ambiguity about what to write.

**Type consistency:** `ReadingDecision`, `PacingState`, `OrderingGuard`, `SelectionEval`/`SelectionCheck`, `anchorBlockId` signatures are identical across the tasks that produce and consume them. Verdict labels/rule stated once in Global Constraints and reused. Check keys `target|evidence|centrality` consistent (Task 4 ↔ Task 6 enum ↔ Task 9 render).

**Gaps found & closed:** the spec's "route" was corrected to a view-swap (spec §7 amended 2026-07-27); Task 5 notes the one pacing input (`TurnsSinceLastPropose`) that has no persisted source and how it degrades safely.

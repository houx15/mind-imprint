# Slice 3b · Agent Anchor Generation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** When the agent summons an **annotation-mode** card, load the task's materials and generate one material-anchored guiding question per card dimension (via an injectable `AnchorGenerator`), persist them on the proposed card, and inject material context into the turn prompt. Deterministic fallback when generation fails or there's no material. Stub-tested; no real-model dependency in tests.

**Architecture:** A domain `Material`/`MaterialBlock` + `TurnStore.ListMaterials`/`SetCardAnchors` seams; a `BuildMaterialContext` prompt helper; an `AnchorGenerator` interface on `TurnDeps` (production impl = `gateway.Collect` + fence-strip + parse + one retry, mirroring `eval.go`; offsets computed server-side; per-dimension unanchored fallback), injected so tests use a fake. `RunTurn`'s summon branch calls it for annotation cards and writes anchors via `SetCardAnchors`.

**Tech Stack:** Go 1.26, gateway `Collect`/`StubProvider`, sqlc, testcontainers. From `apps/api`: `go test ./...`.

## Global Constraints

- **Backend only.** No UI, no contract change (uses 3a's `anchors`/`mode`). Ownership unaffected (turn is already owner-scoped upstream).
- **Anchor `quote` is authoritative for highlighting; numeric `start`/`end` are advisory** (computed server-side as byte offset via `strings.Index`, `0` if not found) — the frontend (3c) locates spans by `quote` to avoid Go-byte vs JS-UTF16 offset mismatch.
- **Annotation gating:** anchors are generated only when `spec.Mode == "annotation"`. Non-annotation cards behave exactly as today (no anchors written).
- **Graceful degradation:** generation failure/empty, or annotation card with no material → deterministic fallback (one anchor per dimension, `block_id:""`, `author:"ai"`, `question` from the step title). The card always works.
- Tests use the gateway `StubProvider` + a fake `AnchorGenerator`; the parse/offset/fallback logic is deterministic.
- Every task ends green + committed.

---

### Task 1: Material domain seam + prompt injection

**Files:**
- Modify: `apps/api/internal/agent/turn.go` (add `Material`/`MaterialBlock`, `TurnStore.ListMaterials`+`SetCardAnchors`, sqlc adapter methods)
- Modify: `apps/api/internal/agent/prompt.go` (add `BuildMaterialContext`)
- Modify: `apps/api/internal/agent/turn_test.go` (the existing `fakeTurnStore` must satisfy the widened interface)
- Test: `apps/api/internal/agent/material_context_test.go`

**Interfaces:**
- Produces: `agent.Material{ ID string; Title string; Blocks []MaterialBlock }`, `agent.MaterialBlock{ ID, Text string }`; `TurnStore.ListMaterials(ctx, taskID) ([]Material, error)`; `TurnStore.SetCardAnchors(ctx, cardInstanceID, taskID uuid.UUID, anchors []byte) error`; `BuildMaterialContext(materials []Material) string` (empty string when no materials).

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/agent/material_context_test.go`:

```go
package agent

import (
	"strings"
	"testing"
)

func TestBuildMaterialContextIncludesBlockIDs(t *testing.T) {
	mats := []Material{
		{ID: "m1", Title: "卫星图看中国变绿", Blocks: []MaterialBlock{
			{ID: "b0", Text: "最近一张 NASA 卫星图刷屏。"},
			{ID: "b1", Text: "文章称地球绿了 5%。"},
		}},
	}
	out := BuildMaterialContext(mats)
	for _, want := range []string{"卫星图看中国变绿", "b0", "b1", "地球绿了 5%", "最近一张 NASA"} {
		if !strings.Contains(out, want) {
			t.Errorf("material context missing %q\n---\n%s", want, out)
		}
	}
}

func TestBuildMaterialContextEmpty(t *testing.T) {
	if BuildMaterialContext(nil) != "" {
		t.Fatal("no materials should yield empty context")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/api && go test ./internal/agent/ -run TestBuildMaterialContext`
Expected: FAIL — undefined `Material`, `MaterialBlock`, `BuildMaterialContext`.

- [ ] **Step 3: Add domain types + prompt helper**

In `apps/api/internal/agent/prompt.go`, add:

```go
// Material is the agent's view of a task material (see slice-2 material table).
type Material struct {
	ID     string
	Title  string
	Blocks []MaterialBlock
}

// MaterialBlock is one addressable paragraph of a material.
type MaterialBlock struct {
	ID   string
	Text string
}

// BuildMaterialContext renders the task's materials (with block ids) as a
// context block the model can reference when anchoring questions. Empty when
// there are no materials.
func BuildMaterialContext(materials []Material) string {
	if len(materials) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# 学生正在读/写的材料（可引用其中的 block_id 与原句）\n")
	for _, m := range materials {
		b.WriteString("## 材料：" + m.Title + "\n")
		for _, blk := range m.Blocks {
			b.WriteString("[" + blk.ID + "] " + blk.Text + "\n")
		}
	}
	return b.String()
}
```

(Ensure `"strings"` is imported in `prompt.go`.)

- [ ] **Step 4: Widen `TurnStore` + adapter**

In `apps/api/internal/agent/turn.go`:

4a. Add to the `TurnStore` interface:

```go
	ListMaterials(ctx context.Context, taskID uuid.UUID) ([]Material, error)
	SetCardAnchors(ctx context.Context, cardInstanceID, taskID uuid.UUID, anchors []byte) error
```

4b. Add the sqlc adapter methods (after `CreateProposedCard`):

```go
func (s *sqlcTurnStore) ListMaterials(ctx context.Context, taskID uuid.UUID) ([]Material, error) {
	rows, err := s.q.ListMaterialsByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]Material, 0, len(rows))
	for _, r := range rows {
		m := Material{ID: r.ID.String(), Title: r.Title}
		if len(r.Blocks) > 0 {
			_ = json.Unmarshal(r.Blocks, &m.Blocks)
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *sqlcTurnStore) SetCardAnchors(ctx context.Context, cardInstanceID, taskID uuid.UUID, anchors []byte) error {
	_, err := s.q.SetCardAnchors(ctx, sqlc.SetCardAnchorsParams{ID: cardInstanceID, TaskID: taskID, Anchors: anchors})
	return err
}
```

`MaterialBlock`'s json tags must match the stored jsonb (`{id,text}`) — add tags to the struct in `prompt.go`:

```go
type MaterialBlock struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
```

4c. In `apps/api/internal/agent/turn_test.go` (this file is `package agent_test` — use **qualified** `agent.` names), the existing `fakeTurnStore` must now implement the two new methods. Add two fields to the `fakeTurnStore` struct — `materials []agent.Material` and `anchorsWritten []byte` — and these methods:

```go
func (f *fakeTurnStore) ListMaterials(_ context.Context, _ uuid.UUID) ([]agent.Material, error) {
	return f.materials, nil
}
func (f *fakeTurnStore) SetCardAnchors(_ context.Context, _, _ uuid.UUID, anchors []byte) error {
	f.anchorsWritten = anchors
	return nil
}
```

(These fields are used in Task 3; adding them now keeps the widened interface satisfied and the package compiling. Struct literals like `&fakeTurnStore{history: ...}` still work.)

- [ ] **Step 5: Run — expect PASS + build**

Run: `cd apps/api && go build ./... && go test ./internal/agent/ -run TestBuildMaterialContext`
Expected: build clean (all `TurnStore` implementers satisfy the widened interface); tests PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/agent/turn.go apps/api/internal/agent/prompt.go apps/api/internal/agent/turn_test.go apps/api/internal/agent/material_context_test.go
git commit -m "feat(api): agent material seam (ListMaterials/SetCardAnchors) + material prompt context"
```

---

### Task 2: `AnchorGenerator` (Collect+parse + offsets + fallback)

**Files:**
- Create: `apps/api/internal/agent/anchors.go`
- Test: `apps/api/internal/agent/anchors_test.go`

**Interfaces:**
- Produces:
  - `type Anchor struct { ID, MaterialID, BlockID string; Start, End int; Quote, Dimension, Author, Question, Answer string }` with json tags matching the contract (`id, material_id, block_id, start, end, quote, dimension, author, question, answer`).
  - `type AnchorGenerator interface { Generate(ctx, spec cards.Spec, materials []Material) ([]Anchor, error) }`.
  - `func NewAnchorGenerator(provider gateway.Provider, resolver gateway.KeyResolver) AnchorGenerator`.
  - Unexported `computeOffsets(blocks []MaterialBlock, blockID, quote string) (start, end int)`; `fallbackAnchors(spec, materials) []Anchor`; `parseAnchorGen(text string, spec, materials) ([]Anchor, error)`.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/agent/anchors_test.go`:

```go
package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

func annotationSpec() cards.Spec {
	return cards.Spec{
		ID: "sift_craap", Name: "CRAAP", Mode: "annotation",
		Steps: []cards.Step{{Key: "authority", Title: "权威性 · Authority"}, {Key: "accuracy", Title: "准确性 · Accuracy"}},
	}
}

func sampleMaterials() []Material {
	return []Material{{ID: "m1", Title: "文章", Blocks: []MaterialBlock{
		{ID: "b0", Text: "某科技博主综合整理的这篇文章称，地球绿了 5%。"},
	}}}
}

func TestParseAnchorGenComputesOffsetsFromQuote(t *testing.T) {
	raw := "```json\n[{\"block_id\":\"b0\",\"quote\":\"某科技博主综合整理\",\"dimension\":\"权威性 · Authority\",\"question\":\"这位作者是权威吗？\"}]\n```"
	got, err := parseAnchorGen(raw, annotationSpec(), sampleMaterials())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].BlockID != "b0" || got[0].Author != "ai" || got[0].Question == "" {
		t.Fatalf("bad anchor: %+v", got)
	}
	if got[0].Quote != "某科技博主综合整理" || got[0].Start != 0 {
		t.Fatalf("offset/quote wrong: %+v", got[0])
	}
	if got[0].MaterialID != "m1" {
		t.Fatalf("material id not resolved: %+v", got[0])
	}
}

func TestParseAnchorGenRejectsUnknownBlock(t *testing.T) {
	raw := `[{"block_id":"zzz","quote":"x","dimension":"d","question":"q"}]`
	if _, err := parseAnchorGen(raw, annotationSpec(), sampleMaterials()); err == nil {
		t.Fatal("expected error for unknown block_id")
	}
}

func TestFallbackAnchorsOnePerDimension(t *testing.T) {
	got := fallbackAnchors(annotationSpec(), nil)
	if len(got) != 2 || got[0].Dimension != "权威性 · Authority" || got[0].BlockID != "" || got[0].Author != "ai" {
		t.Fatalf("bad fallback: %+v", got)
	}
}

func TestGenerateUsesStubThenPersistsAnchors(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `[{"block_id":"b0","quote":"某科技博主综合整理","dimension":"权威性 · Authority","question":"作者是谁？"}]`},
		{Kind: gateway.EventDone},
	}
	gen := NewAnchorGenerator(gateway.NewStubProvider(script), func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })
	got, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Question != "作者是谁？" {
		t.Fatalf("generate wrong: %+v", got)
	}
}

func TestGenerateFallsBackWhenModelReturnsGarbage(t *testing.T) {
	script := []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "not json at all"}, {Kind: gateway.EventDone}}
	gen := NewAnchorGenerator(gateway.NewStubProvider(script), func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })
	got, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials())
	if err != nil {
		t.Fatalf("fallback must not error: %v", err)
	}
	if len(got) != 2 { // one per dimension
		t.Fatalf("want 2 fallback anchors, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/api && go test ./internal/agent/ -run "TestParseAnchorGen|TestFallback|TestGenerate"`
Expected: FAIL — undefined `parseAnchorGen`, `fallbackAnchors`, `NewAnchorGenerator`, `Anchor`.

- [ ] **Step 3: Implement `anchors.go`**

Create `apps/api/internal/agent/anchors.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

// Anchor is the agent's view of a card anchor (mirrors the TS Anchor contract).
type Anchor struct {
	ID         string `json:"id"`
	MaterialID string `json:"material_id"`
	BlockID    string `json:"block_id"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	Quote      string `json:"quote"`
	Dimension  string `json:"dimension"`
	Author     string `json:"author"`
	Question   string `json:"question"`
	Answer     string `json:"answer"`
}

// AnchorGenerator produces material-anchored guiding questions for an annotation card.
type AnchorGenerator interface {
	Generate(ctx context.Context, spec cards.Spec, materials []Material) ([]Anchor, error)
}

type llmAnchorGenerator struct {
	provider gateway.Provider
	resolver gateway.KeyResolver
}

// NewAnchorGenerator returns the production generator (LLM Collect + parse, with
// a deterministic per-dimension fallback).
func NewAnchorGenerator(provider gateway.Provider, resolver gateway.KeyResolver) AnchorGenerator {
	return &llmAnchorGenerator{provider: provider, resolver: resolver}
}

// genItem is the model's per-anchor JSON contract (offsets are computed server-side).
type genItem struct {
	BlockID   string `json:"block_id"`
	Quote     string `json:"quote"`
	Dimension string `json:"dimension"`
	Question  string `json:"question"`
}

func stripFences(text string) string {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	return c
}

// blockLookup indexes material blocks by block id → (materialID, text).
func blockLookup(materials []Material) map[string][2]string {
	m := map[string][2]string{}
	for _, mat := range materials {
		for _, b := range mat.Blocks {
			m[b.ID] = [2]string{mat.ID, b.Text}
		}
	}
	return m
}

// computeOffsets returns the byte offsets of quote within the block's text (0,0
// when not found — quote stays authoritative for the UI).
func computeOffsets(text, quote string) (int, int) {
	i := strings.Index(text, quote)
	if i < 0 {
		return 0, 0
	}
	return i, i + len(quote)
}

func parseAnchorGen(text string, spec cards.Spec, materials []Material) ([]Anchor, error) {
	var items []genItem
	if err := json.Unmarshal([]byte(stripFences(text)), &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errString("empty anchors")
	}
	lut := blockLookup(materials)
	out := make([]Anchor, 0, len(items))
	for i, it := range items {
		ref, ok := lut[it.BlockID]
		if !ok {
			return nil, errString("unknown block_id: " + it.BlockID)
		}
		start, end := computeOffsets(ref[1], it.Quote)
		out = append(out, Anchor{
			ID: "a" + strconv.Itoa(i), MaterialID: ref[0], BlockID: it.BlockID,
			Start: start, End: end, Quote: it.Quote, Dimension: it.Dimension,
			Author: "ai", Question: it.Question, Answer: "",
		})
	}
	return out, nil
}

// fallbackAnchors builds one unanchored ai anchor per card dimension (step title).
func fallbackAnchors(spec cards.Spec, materials []Material) []Anchor {
	matID := ""
	if len(materials) > 0 {
		matID = materials[0].ID
	}
	out := make([]Anchor, 0, len(spec.Steps))
	for i, st := range spec.Steps {
		out = append(out, Anchor{
			ID: "a" + strconv.Itoa(i), MaterialID: matID, BlockID: "",
			Dimension: st.Title, Author: "ai",
			Question: "从「" + st.Title + "」这个角度看这份材料，你注意到什么？",
			Answer:   "",
		})
	}
	return out
}

func (g *llmAnchorGenerator) Generate(ctx context.Context, spec cards.Spec, materials []Material) ([]Anchor, error) {
	resolved, err := g.resolver(ctx)
	if err != nil {
		return fallbackAnchors(spec, materials), nil
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildAnchorPrompt(spec)},
			{Role: gateway.RoleUser, Content: BuildMaterialContext(materials)},
		},
		MaxTokens: 1500,
	}
	res, err := gateway.Collect(ctx, g.provider, resolved, req)
	if err != nil {
		return fallbackAnchors(spec, materials), nil
	}
	anchors, perr := parseAnchorGen(res.Text, spec, materials)
	if perr != nil || len(anchors) == 0 {
		return fallbackAnchors(spec, materials), nil
	}
	return anchors, nil
}

func buildAnchorPrompt(spec cards.Spec) string {
	var dims strings.Builder
	for _, st := range spec.Steps {
		dims.WriteString("- " + st.Title + "\n")
	}
	return "你是一名批判性思维教练。学生正在读下面这份材料。请针对「" + spec.Name +
		"」的每个维度，在材料里挑出一处最相关的原句，提出一个指向那句话的具体引导问题。\n" +
		"维度：\n" + dims.String() +
		"\n只输出 JSON 数组，每个元素形如 {\"block_id\":\"b0\",\"quote\":\"材料里的原句片段\",\"dimension\":\"维度名\",\"question\":\"你的问题\"}。" +
		"block_id 必须来自材料，quote 必须是该 block 里的原文片段。不要输出任何多余文字。"
}

// errString is a tiny error helper (avoids importing errors just for this file).
type errString string

func (e errString) Error() string { return string(e) }
```

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/api && go test ./internal/agent/ -run "TestParseAnchorGen|TestFallback|TestGenerate"`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/anchors.go apps/api/internal/agent/anchors_test.go
git commit -m "feat(api): AnchorGenerator — Collect+parse, server offsets, per-dimension fallback"
```

---

### Task 3: Wire anchor generation + material context into RunTurn

**Files:**
- Modify: `apps/api/internal/agent/turn.go` (`TurnDeps.AnchorGen`; inject material context; generate+persist anchors on annotation summon)
- Modify: `apps/api/internal/api/turn.go` (wire `AnchorGen` into `TurnDeps`)
- Modify: `apps/api/internal/api/api.go` (`Deps` gains an `AnchorGen` for wiring) — OR construct in `postTurn`; see step 3c
- Test: `apps/api/internal/agent/turn_test.go` (add annotation-summon test)

**Interfaces:**
- Consumes: `AnchorGenerator` (Task 2), `TurnStore.ListMaterials/SetCardAnchors` + `BuildMaterialContext` (Task 1), `spec.Mode` (3a).
- Produces: `TurnDeps.AnchorGen AnchorGenerator`; `RunTurn` injects material context into the prompt and, on an annotation-mode summon, writes generated anchors before emitting the card event.

- [ ] **Step 1: Write the failing test**

Add to `apps/api/internal/agent/turn_test.go` (`package agent_test` — qualified `agent.` names; add `"strings"` to the imports). This reuses the file's real `fakeTurnStore` (struct literal, now with the Task-1 `materials`/`anchorsWritten` fields) and `fakeSSE`, and scripts a summon the same way `TestRunTurnProposesCardAndPersists` does:

```go
type fakeAnchorGen struct{ anchors []agent.Anchor }

func (f fakeAnchorGen) Generate(_ context.Context, _ cards.Spec, _ []agent.Material) ([]agent.Anchor, error) {
	return f.anchors, nil
}

func TestRunTurnWritesAnchorsForAnnotationCard(t *testing.T) {
	store := &fakeTurnStore{
		materials: []agent.Material{{ID: "m1", Title: "T", Blocks: []agent.MaterialBlock{{ID: "b0", Text: "原句。"}}}},
	}
	spec := cards.Spec{ID: "sift_craap", Name: "CRAAP", Mode: "annotation", Steps: []cards.Step{{Key: "a", Title: "权威性"}}}
	stub := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "先核查来源。"},
		{Kind: gateway.EventToolUse, ToolUse: &gateway.StreamToolUse{ID: "t1", Name: "summon_card", ArgsJSON: `{"card_id":"sift_craap","reason":"r","nudge_text":"n"}`}},
		{Kind: gateway.EventDone, StopReason: gateway.StopToolCall},
	})
	deps := agent.TurnDeps{
		Store:       store,
		Provider:    stub,
		KeyResolver: func(context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil },
		Catalog:     []cards.Spec{spec},
		SpecByID:    func(id string) (cards.Spec, bool) { if id == "sift_craap" { return spec, true }; return cards.Spec{}, false },
		SSE:         &fakeSSE{},
		AnchorGen:   fakeAnchorGen{anchors: []agent.Anchor{{ID: "a0", BlockID: "b0", Dimension: "权威性", Author: "ai", Question: "可信吗？"}}},
	}
	if err := agent.RunTurn(context.Background(), deps, uuid.New(), "看看这个"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(store.anchorsWritten), "可信吗？") {
		t.Fatalf("anchors not persisted: %s", store.anchorsWritten)
	}
}
```

The assertion that matters: after an annotation-mode summon, the fake store's `anchorsWritten` (set by `SetCardAnchors`) contains the generated question. (The unit `fakeTurnStore.CreateProposedCard` returns a random UUID and `SetCardAnchors` records the bytes — no DB needed.)

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/api && go test ./internal/agent/ -run TestRunTurnWritesAnchors`
Expected: FAIL — `TurnDeps` has no `AnchorGen`; anchors never written.

- [ ] **Step 3: Wire `RunTurn`**

3a. In `apps/api/internal/agent/turn.go`, add to `TurnDeps`:

```go
	AnchorGen AnchorGenerator
```

3b. Inject material context into the prompt. After `systemPrompt := BuildSystemPrompt(deps.Catalog)` (line ~80), add:

```go
	materials, _ := deps.Store.ListMaterials(ctx, taskID)
	if mc := BuildMaterialContext(materials); mc != "" {
		systemPrompt = systemPrompt + "\n\n" + mc
	}
```

3c. In the summon branch, right after `CreateProposedCard` succeeds (line ~148, before `AppendAssistantMessage`), generate + persist anchors for annotation cards:

```go
				spec, _ := deps.SpecByID(args.CardID)
				if spec.Mode == "annotation" && deps.AnchorGen != nil {
					anchors, gerr := deps.AnchorGen.Generate(ctx, spec, materials)
					if gerr == nil && len(anchors) > 0 {
						if raw, merr := json.Marshal(anchors); merr == nil {
							_ = deps.Store.SetCardAnchors(ctx, cardInstanceID, taskID, raw)
						}
					}
				}
```

(Anchors persistence is best-effort — a failure must not abort the turn; the card still proposes.)

- [ ] **Step 4: Wire the production generator**

In `apps/api/internal/api/turn.go` where `agent.TurnDeps{...}` is built (around line 122), add:

```go
		AnchorGen: agent.NewAnchorGenerator(a.d.Provider, a.d.ChatResolver),
```

(Reuses the chaperone provider + resolver — documented in the spec. No `Deps` change needed if `Provider`/`ChatResolver` are already on `a.d`; they are.)

- [ ] **Step 5: Run — expect PASS**

Run: `cd apps/api && go test ./internal/agent/ -run TestRunTurn`
Expected: PASS (the new anchors test + all existing RunTurn tests, which use non-annotation specs or a nil `AnchorGen` and are unaffected).

- [ ] **Step 6: Full backend build + vet + short**

Run: `cd apps/api && go build ./... && go vet ./... && go test -short ./...`
Expected: clean; short tests pass.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/agent/turn.go apps/api/internal/api/turn.go apps/api/internal/agent/turn_test.go
git commit -m "feat(api): generate + persist anchors on annotation-card summon; inject material context"
```

---

## Self-Review

**Spec coverage:** material loaded into turn + injected with block ids (T1) ✅; anchor generation via Collect+parse with server offsets + per-dimension fallback (T2) ✅; annotation-gated generation + persist on summon (T3) ✅; graceful degradation (T2 fallback + T3 best-effort) ✅; stub/fake-tested, no real model (T2/T3) ✅.

**Placeholder scan:** none — T3 Step 1's test now uses the real `turn_test.go` helpers (`&fakeTurnStore{}`, `fakeSSE`, the `StreamToolUse` summon script) with correct `agent_test` package qualification. Complete code throughout.

**Type consistency:** `Anchor` json tags (T2) match the contract + 3a's Go validation. `AnchorGenerator.Generate(ctx, cards.Spec, []Material)` identical in the interface (T2), the fake (T3), and the production impl (T2). `TurnStore.ListMaterials/SetCardAnchors` defined T1, consumed T3. `TurnDeps.AnchorGen` defined T3, wired in api/turn.go T3.

**Ordering:** T1 seam+context → T2 generator (independent) → T3 wire (consumes T1+T2).

## Note for executor

- `turn_test.go` already has a `fakeTurnStore` and summon-script/SSE helpers — Task 1 widens the fake with two methods + two fields; Task 3 reuses the existing summon test scaffolding. Read the file first and reuse its real helper names; the brief's helper names in T3 Step 1 are illustrative.
- Anchor generation runs the chaperone model a second time per annotation summon (documented latency cost). Fallback guarantees the card always has anchors.

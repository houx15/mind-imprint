# Slice 7 — 结构 view + `graph` primitive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the third hand-built interaction primitive (`graph`, a typed-slot argument builder) + the Toulmin card as pure C2 config over it, and wire the already-scaffolded S4 结构 view live end-to-end (summon → render → mint → gate).

**Architecture:** The Toulmin card carries its five slots (claim/warrant/evidence/counter/concession) in the envelope's **anchors** — one text anchor per slot + one source anchor per cited material, all keyed by `dimension: <slotId>`. This reuses the anchor spine wholesale: `EvaluateCompletion` and `GraphEffects` gain only additive `switch` cases (the 6c `lateral_source_present`/`cross_check` pattern), no signature changes. The card auto-surfaces from graph state (S4 reachable + no `claim` node) via `SurfaceCardCandidates`, renders center-pane in `StructureView`, and mints `claim`/`warrant`/`evidence`/`counter`/`concession` nodes + `supports`/`cites` edges through the existing atomic `CompleteCard` path. `GraphState` (already in contract) is the frontend primitive's working type, serialized to anchors on lock.

**Tech Stack:** Go (`net/http`, sqlc, testcontainers), TypeScript + React + vitest, Zod contracts (`packages/contracts`).

## Global Constraints

- **Client never calls a model directly**; API keys are server-side only (`apps/api`).
- **Go tests run serialized on a quiet Docker daemon:** `CGO_ENABLED=0 go test -p 1 ./...`. The 6c "hang" was testcontainer contention from concurrent runs, not a bug — do not run other suites simultaneously.
- **Binding Chinese design copy is verbatim.** The five slot roles/questions come verbatim from `S4META` (design HTML ~L1947); the gate banner copy from `StructureView.tsx` is already verbatim. If a test disagrees with the copy, fix the test, never the copy.
- **Nothing invented / no fabricated AI verdict.** Every slot's node text is `author: "student"` (enforcement rejects `author != student` at write). No AI authors slot text.
- **RL-2:** no agent-reachable path creates a material. The `cites` edge targets are sourced only from the student's CRAAP-locked source chips (a closed set the UI supplies), never free text.
- **Card spec single source of truth is the canonical JSON** in `packages/contracts/cards/`; Go embeds a **synced mirror** `apps/api/internal/cards/specs/`. Every card-JSON change runs `make sync-cards`; `TestMirrorMatchesCanonical` guards the mirror.
- **New card = new JSON, no renderer edit** — except this slice, which adds the `graph` primitive renderer once (the primitive itself is the hand-built exception the architecture allows).
- **Icons are inline SVG** — never `lucide-react`. No orphaned files/props/imports.
- **Docs in English** except literal Chinese UI copy. Secrets never enter logs, errors, migrations, or payloads.
- **Direct-merge to main + push** (no PRs), via `finishing-a-development-branch`, after the whole-branch review.
- **Never `git add` a whole directory** that contains the user's untracked working files; stage explicit paths.

---

## File structure

**Create:**
- `apps/web/src/primitives/graph/Graph.tsx` — the five-slot renderer over `GraphState` (+ `Graph.test.tsx`).
- `apps/web/src/primitives/graph/serialize.ts` — `graphStateToAnchors` / `anchorsToGraphState` (+ `serialize.test.ts`).
- `apps/web/src/studio/StudioToulminCard.tsx` — studio host binding the primitive to the active card_instance (+ test).
- `packages/contracts/cards/toulmin.json` + mirror `apps/api/internal/cards/specs/toulmin.json`.

**Modify:**
- `apps/api/internal/cards/loader.go` — add `Slots []Slot` to `Params` (+ `Slot` struct).
- `apps/api/internal/agent/card_completion.go` — `graph_slots_complete` case.
- `apps/api/internal/agent/card_effects.go` — `toulmin` case in `GraphEffects`.
- `apps/api/internal/agent/classifier.go` — Toulmin surface rule.
- `apps/api/internal/skills/specs/writing-project.json` — drop `no_single_sourced_claim` from `build_argument`; add `toulmin` to `cards` + `build_argument.repertoire`.
- `apps/web/src/studio/state.ts` — extend `StructureCardFx` with interactive fields.
- `apps/web/src/studio/views/StructureView.tsx` — replace dead stubs with the live primitive.
- `apps/web/src/studio/StudioContainer.tsx` — project the active Toulmin card into `views.structure`; submit path.

---

### Task 1: Toulmin card JSON + `Slot` params struct

**Files:**
- Modify: `apps/api/internal/cards/loader.go` (the `Params` struct, ~L45)
- Create: `packages/contracts/cards/toulmin.json`
- Create (via sync): `apps/api/internal/cards/specs/toulmin.json`
- Test: `apps/api/internal/cards/loader_test.go`; `packages/contracts/test/cardSpec.test.ts` (existing, must stay green); `apps/api/internal/cards/mirror_test.go` (`TestMirrorMatchesCanonical`)

**Interfaces:**
- Produces: `cards.Params.Slots []cards.Slot`, where `Slot{ID string; Role string; NeedSrc bool; Q string}` with JSON tags `id`/`role`/`needSrc`/`q`. Card id `"toulmin"`, `primitive: "graph"`, `target_type: "project"`, completion `[{kind: graph_slots_complete}]`, graph_effects `[{kind: toulmin}]`. Consumed by Tasks 2, 3, 5, 7.

- [ ] **Step 1: Write the failing Go test for `Slot` parsing**

Add to `apps/api/internal/cards/loader_test.go`:

```go
func TestToulminSlotsParse(t *testing.T) {
	spec, err := Load("toulmin")
	if err != nil {
		t.Fatalf("load toulmin: %v", err)
	}
	if spec.Primitive != "graph" {
		t.Fatalf("primitive = %q, want graph", spec.Primitive)
	}
	if got := len(spec.Params.Slots); got != 5 {
		t.Fatalf("slots = %d, want 5", got)
	}
	byID := map[string]Slot{}
	for _, s := range spec.Params.Slots {
		byID[s.ID] = s
	}
	if !byID["evidence"].NeedSrc || byID["claim"].NeedSrc {
		t.Fatalf("needSrc wrong: evidence must need a source, claim must not")
	}
	if byID["claim"].Role != "核心主张" {
		t.Fatalf("claim role = %q, want 核心主张", byID["claim"].Role)
	}
}
```

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/cards/ -run TestToulminSlotsParse`
Expected: FAIL — `spec.Params.Slots` undefined (and toulmin not found).

- [ ] **Step 3: Add the `Slot` struct + `Slots` field**

In `apps/api/internal/cards/loader.go`, inside `type Params struct` (after `LateralDimension`):

```go
	// Slots is the graph primitive's typed-slot config (Slice 7 Toulmin card):
	// one entry per argument role the student authors. Empty for every
	// non-graph card.
	Slots []Slot `json:"slots"`
}

// Slot is one typed argument role in a graph-primitive card. ID is the node
// type it mints (claim/warrant/evidence/counter/concession); Role is the
// verbatim design label; NeedSrc requires ≥1 cited source material; Q is the
// coach's guiding question for the slot.
type Slot struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	NeedSrc bool   `json:"needSrc"`
	Q       string `json:"q"`
}
```

(Close the original `Params` brace correctly — the `}` above closes `Params`, then `Slot` follows.)

- [ ] **Step 4: Author `packages/contracts/cards/toulmin.json`**

Slot `role`/`q` are verbatim from `S4META`. `steps` methodology is **reused/adapted from the existing authored cards** (`argument-map.json` for claim/warrant/evidence, `steelman.json` for counter, `concession.json` for concession) — do not invent new pedagogy. Each step needs ≥1 `fields` entry (a `textarea`) to satisfy `CardSpec` validation; the graph primitive renders `params.slots`, not `steps.fields`.

```json
{
  "id": "toulmin",
  "category": "论证结构",
  "name": "论证构建卡（图尔敏）",
  "name_en": "Toulmin Argument Builder",
  "purpose": "把论点搭成能立住的结构：主张、理据、证据、反方与让步，一步步写成句子",
  "trigger_condition": "学生核完来源、要开始把观点搭成完整论证结构时",
  "trigger_keywords": ["论证", "主张", "理据", "反方", "让步", "搭论证"],
  "priority": "P0",
  "disclosure_tier": "tier-1",
  "age_band": ["MYP", "DP"],
  "interaction_type": "画布导图卡",
  "rubric_dims": ["D4", "D5", "D7"],
  "related": ["argument-map", "steelman", "concession"],
  "body_status": "full",
  "rubric_tags": ["D4", "D5", "D7"],
  "primitive": "graph",
  "target_type": "project",
  "params": {
    "slots": [
      { "id": "claim",      "role": "核心主张",   "needSrc": false, "q": "你要论证的核心判断，用一句话说清。" },
      { "id": "warrant",    "role": "理据 · 推理", "needSrc": true,  "q": "为什么这些证据能支撑主张？把中间的推理写出来。" },
      { "id": "evidence",   "role": "支撑证据",   "needSrc": true,  "q": "挑一条证据，用自己的话概括它如何支撑主张。" },
      { "id": "counter",    "role": "反方 · 钢人", "needSrc": false, "q": "坐到反方席——对方最硬的一张牌是什么？" },
      { "id": "concession", "role": "让步 · 转折", "needSrc": true,  "q": "承认反方后如何转折？给一条能核查的证据。" }
    ]
  },
  "completion": [
    { "kind": "graph_slots_complete" }
  ],
  "graph_effects": [
    { "kind": "toulmin" }
  ],
  "consolidation": "reveal_framework_after_completion",
  "intrusiveness_cap": "I3",
  "steps": [
    {
      "key": "claim",
      "title": "核心主张",
      "methodology": {
        "why": "论证读起来「有道理」常常是结构感骗了你——先把你要证明的那一句话钉死，后面才有靶心。",
        "how": "用一句话写清你要论证的核心判断，不堆修饰、不留模糊。",
        "when": "在动手搭论证结构的第一步。"
      },
      "fields": [
        { "type": "textarea", "key": "claim", "label": "你要论证的核心判断，用一句话说清。" }
      ]
    },
    {
      "key": "warrant",
      "title": "理据 · 推理",
      "methodology": {
        "why": "证据不会自己说话——中间那步「为什么这些证据能支撑主张」如果不写出来，读者就得替你脑补，论证就有跳步。",
        "how": "把证据到主张之间的推理链条明确写出来，指明它默认了什么前提。",
        "when": "当你有了证据，却还没说清它凭什么支撑主张时。"
      },
      "fields": [
        { "type": "textarea", "key": "warrant", "label": "为什么这些证据能支撑主张？把中间的推理写出来。" }
      ]
    },
    {
      "key": "evidence",
      "title": "支撑证据",
      "methodology": {
        "why": "读者没法验证的断言，examiner 会当成未支撑——每个主张都要接到你核过的可信来源上。",
        "how": "从信源评估里锁定的来源中挑一条，用自己的话概括它如何支撑主张。",
        "when": "在写完主张、要给它找落脚点时。"
      },
      "fields": [
        { "type": "textarea", "key": "evidence", "label": "挑一条证据，用自己的话概括它如何支撑主张。" }
      ]
    },
    {
      "key": "counter",
      "title": "反方 · 钢人",
      "methodology": {
        "why": "只说对自己有利的，读者会觉得你没看见反方——论证显得片面。先把反方最强的版本说出来，反而证明你看全了。",
        "how": "坐到反方席，把对方最硬的那张牌用最强的方式说出来，别打稻草人。",
        "when": "当你只论证了自己一方、还没认真对待反方时。"
      },
      "fields": [
        { "type": "textarea", "key": "counter", "label": "坐到反方席——对方最硬的一张牌是什么？" }
      ]
    },
    {
      "key": "concession",
      "title": "让步 · 转折",
      "methodology": {
        "why": "先承认反方最强的事实，再转折反驳，论证从「片面」变「可信」，结论更站得住。",
        "how": "真诚承认反方那个事实（让步），再给一条能核查的证据说明它为什么不足以推翻你的主张（转折）。",
        "when": "在写出反方最强点之后，要以退为进时。"
      },
      "fields": [
        { "type": "textarea", "key": "concession", "label": "承认反方后如何转折？给一条能核查的证据。" }
      ]
    }
  ]
}
```

- [ ] **Step 5: Sync the mirror + run the contract suite**

Run: `make sync-cards`
Run: `npm --prefix packages/contracts test -- cardSpec`
Expected: PASS (toulmin validates against `CardSpec`).

- [ ] **Step 6: Run the Go tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/cards/`
Expected: PASS — `TestToulminSlotsParse` and `TestMirrorMatchesCanonical` green.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/cards/loader.go apps/api/internal/cards/loader_test.go packages/contracts/cards/toulmin.json apps/api/internal/cards/specs/toulmin.json
git commit -m "feat(refactor2): Toulmin card JSON + graph Slot params (Slice 7 Task 1)"
```

---

### Task 2: `graph_slots_complete` completion predicate

**Files:**
- Modify: `apps/api/internal/agent/card_completion.go` (`EvaluateCompletion`, ~L18)
- Test: `apps/api/internal/agent/card_completion_test.go`

**Interfaces:**
- Consumes: `cards.Params.Slots` (Task 1); `Anchor{Dimension, Answer, MaterialID string}`.
- Produces: a new `switch` case `"graph_slots_complete"` in `EvaluateCompletion(spec cards.Spec, anchors []Anchor) (bool, []string)` — no signature change. Missing slot ids are appended in `params.slots` order.

- [ ] **Step 1: Write the failing test**

Add to `apps/api/internal/agent/card_completion_test.go`:

```go
func toulminSpec(t *testing.T) cards.Spec {
	t.Helper()
	s, err := cards.Load("toulmin")
	if err != nil {
		t.Fatalf("load toulmin: %v", err)
	}
	return s
}

func TestGraphSlotsComplete(t *testing.T) {
	spec := toulminSpec(t)

	// One text anchor per slot; needSrc slots (warrant/evidence/concession)
	// also get a source anchor. All five text anchors ≥12 chars.
	full := []Anchor{
		{Dimension: "claim", Answer: "中国的政策在净效果上让全球更可持续。", Author: "student"},
		{Dimension: "warrant", Answer: "卫星植被数据到可持续判断之间的推理如下所述。", Author: "student"},
		{Dimension: "warrant", MaterialID: "m_nasa", Author: "student"},
		{Dimension: "evidence", Answer: "NASA 观测显示中国主导了全球变绿的增量。", Author: "student"},
		{Dimension: "evidence", MaterialID: "m_nasa", Author: "student"},
		{Dimension: "counter", Answer: "反方最强点：中国碳排放总量全球第一。", Author: "student"},
		{Dimension: "concession", Answer: "承认排放第一，但人均与历史累积远低于发达国家。", Author: "student"},
		{Dimension: "concession", MaterialID: "m_bp", Author: "student"},
	}
	if ok, missing := EvaluateCompletion(spec, full); !ok {
		t.Fatalf("full graph should complete, missing=%v", missing)
	}

	// Drop the evidence source anchor -> evidence slot is unsourced.
	noSrc := append([]Anchor{}, full[:4]...)
	noSrc = append(noSrc, full[5:]...) // skip the evidence source anchor
	ok, missing := EvaluateCompletion(spec, noSrc)
	if ok {
		t.Fatalf("evidence with no source must not complete")
	}
	if len(missing) != 1 || missing[0] != "evidence" {
		t.Fatalf("missing = %v, want [evidence]", missing)
	}

	// Short claim text (<12 chars) -> claim slot incomplete.
	shortClaim := append([]Anchor{}, full...)
	shortClaim[0].Answer = "太短"
	if ok, _ := EvaluateCompletion(spec, shortClaim); ok {
		t.Fatalf("claim under 12 chars must not complete")
	}
}
```

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestGraphSlotsComplete`
Expected: FAIL — the predicate is unrecognized, so a card with no matching case reports complete (or the missing list is empty) → assertions fail.

- [ ] **Step 3: Add the predicate case + helper**

In `EvaluateCompletion`'s `switch pred.Kind`, add:

```go
		case "graph_slots_complete":
			for _, slot := range spec.Params.Slots {
				if !slotComplete(anchors, slot) {
					missing = append(missing, slot.ID)
				}
			}
```

Add the helper (below `tagAnswered`), using the existing rune-count idiom:

```go
// slotComplete reports whether a graph-primitive slot is done: some anchor on
// the slot's dimension carries a student sentence of ≥12 runes, and — when the
// slot needs a source — some anchor on the slot's dimension carries a
// non-empty material_id. Text anchors (material_id "") and source anchors
// (answer "") never collide.
func slotComplete(anchors []Anchor, slot cards.Slot) bool {
	hasText := false
	hasSource := false
	for _, a := range anchors {
		if a.Dimension != slot.ID {
			continue
		}
		if utf8.RuneCountInString(strings.TrimSpace(a.Answer)) >= 12 {
			hasText = true
		}
		if strings.TrimSpace(a.MaterialID) != "" {
			hasSource = true
		}
	}
	return hasText && (!slot.NeedSrc || hasSource)
}
```

(`utf8` and `strings` are already imported in this file.)

- [ ] **Step 4: Run it — expect PASS**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestGraphSlotsComplete`
Expected: PASS.

- [ ] **Step 5: Confirm CRAAP/SIFT completion unchanged**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestEvaluateCompletion`
Expected: PASS (the additive case does not touch `every_tag_present`/`field_written_by`/`lateral_source_present`).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/agent/card_completion.go apps/api/internal/agent/card_completion_test.go
git commit -m "feat(refactor2): graph_slots_complete completion predicate (Slice 7 Task 2)"
```

---

### Task 3: `toulmin` graph effect (mint)

**Files:**
- Modify: `apps/api/internal/agent/card_effects.go` (`GraphEffects`, the `switch effect.Kind`)
- Test: `apps/api/internal/agent/card_effects_test.go`

**Interfaces:**
- Consumes: `cards.Params.Slots` (Task 1); `Anchor`; `MintNode{Type, Author string; Body map[string]any}`; `MintEdge{Type, FromKind, FromID, ToKind, ToID string}`; the `"$new:<idx>"` placeholder convention (`CompleteCard` resolves both endpoints).
- Produces: a `"toulmin"` case in `GraphEffects(spec cards.Spec, materialID string, anchors []Anchor) ([]MintNode, []MintEdge)` — no signature change. Mints one node per slot with a text anchor; a `supports` edge evidence→claim; a `cites` edge per source anchor.

- [ ] **Step 1: Write the failing test**

Add to `apps/api/internal/agent/card_effects_test.go`:

```go
func TestToulminGraphEffect(t *testing.T) {
	spec := toulminSpec(t) // helper from card_completion_test.go (same package)
	anchors := []Anchor{
		{Dimension: "claim", Answer: "中国的政策在净效果上让全球更可持续。", Author: "student"},
		{Dimension: "warrant", Answer: "从植被数据到可持续判断的推理链条。", Author: "student"},
		{Dimension: "warrant", MaterialID: "m_nasa", Author: "student"},
		{Dimension: "evidence", Answer: "NASA 观测显示中国主导了全球变绿增量。", Author: "student"},
		{Dimension: "evidence", MaterialID: "m_nasa", Author: "student"},
		{Dimension: "counter", Answer: "反方最强点：碳排放总量全球第一。", Author: "student"},
		{Dimension: "concession", Answer: "承认排放第一，但人均与历史累积远低。", Author: "student"},
		{Dimension: "concession", MaterialID: "m_bp", Author: "student"},
	}

	nodes, edges := GraphEffects(spec, "", anchors)

	// Five nodes, one per slot, all student-authored, body carries the text.
	if len(nodes) != 5 {
		t.Fatalf("nodes = %d, want 5", len(nodes))
	}
	idxByType := map[string]int{}
	for i, n := range nodes {
		if n.Author != "student" {
			t.Fatalf("node %s author = %q, want student", n.Type, n.Author)
		}
		if n.Body["text"] == "" || n.Body["text"] == nil {
			t.Fatalf("node %s has empty text", n.Type)
		}
		idxByType[n.Type] = i
	}
	for _, want := range []string{"claim", "warrant", "evidence", "counter", "concession"} {
		if _, ok := idxByType[want]; !ok {
			t.Fatalf("missing node type %q", want)
		}
	}

	// Exactly one supports edge, evidence -> claim, both placeholder endpoints.
	supports := 0
	for _, e := range edges {
		if e.Type != "supports" {
			continue
		}
		supports++
		if e.FromID != mintRef(idxByType["evidence"]) || e.ToID != mintRef(idxByType["claim"]) {
			t.Fatalf("supports edge = %s->%s, want evidence->claim placeholders", e.FromID, e.ToID)
		}
		if e.FromKind != "graph_node" || e.ToKind != "graph_node" {
			t.Fatalf("supports endpoints must be graph_node")
		}
	}
	if supports != 1 {
		t.Fatalf("supports edges = %d, want 1", supports)
	}

	// One cites edge per source anchor (warrant m_nasa, evidence m_nasa,
	// concession m_bp), each graph_node -> material with a real material id.
	cites := map[string]int{}
	for _, e := range edges {
		if e.Type != "cites" {
			continue
		}
		if e.FromKind != "graph_node" || e.ToKind != "material" {
			t.Fatalf("cites endpoints wrong: %s->%s", e.FromKind, e.ToKind)
		}
		cites[e.ToID]++
	}
	if cites["m_nasa"] != 2 || cites["m_bp"] != 1 {
		t.Fatalf("cites = %v, want m_nasa:2 m_bp:1", cites)
	}
}
```

If a `mintRef(i int) string` helper does not already exist in the package, add it beside the placeholder logic in `card_effects.go`:

```go
// mintRef builds the deterministic placeholder a MintEdge uses to point at the
// i-th node in the returned nodes slice; CompleteCard resolves it to the real id.
func mintRef(i int) string { return fmt.Sprintf("$new:%d", i) }
```

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestToulminGraphEffect`
Expected: FAIL — `GraphEffects` yields nothing for the unrecognized `toulmin` effect.

- [ ] **Step 3: Add the `toulmin` case**

In `GraphEffects`'s `switch effect.Kind`, add (after the existing cases):

```go
		case "toulmin":
			// One node per slot that has a text anchor; index each by slot id
			// so the supports edge can reference claim/evidence by placeholder.
			nodeIdx := make(map[string]int, len(spec.Params.Slots))
			for _, slot := range spec.Params.Slots {
				text := slotText(anchors, slot.ID)
				if text == "" {
					continue
				}
				nodes = append(nodes, MintNode{
					Type:   slot.ID,
					Author: "student",
					Body:   map[string]any{"text": text},
				})
				nodeIdx[slot.ID] = len(nodes) - 1
			}
			// supports: evidence -> claim (both placeholders).
			if ei, ok := nodeIdx["evidence"]; ok {
				if ci, ok := nodeIdx["claim"]; ok {
					edges = append(edges, MintEdge{
						Type: "supports", FromKind: "graph_node", FromID: mintRef(ei),
						ToKind: "graph_node", ToID: mintRef(ci),
					})
				}
			}
			// cites: one edge per source anchor on any minted slot.
			for _, a := range anchors {
				if strings.TrimSpace(a.MaterialID) == "" {
					continue
				}
				si, ok := nodeIdx[a.Dimension]
				if !ok {
					continue
				}
				edges = append(edges, MintEdge{
					Type: "cites", FromKind: "graph_node", FromID: mintRef(si),
					ToKind: "material", ToID: a.MaterialID,
				})
			}
```

Add the text helper (near the top of the file or beside `sourceQuality`):

```go
// slotText returns the first non-empty student sentence anchored on the given
// slot dimension (the text anchor; source anchors carry an empty answer).
func slotText(anchors []Anchor, slotID string) string {
	for _, a := range anchors {
		if a.Dimension == slotID {
			if t := strings.TrimSpace(a.Answer); t != "" {
				return t
			}
		}
	}
	return ""
}
```

(`strings` and `fmt` are already imported in `card_effects.go`.)

- [ ] **Step 4: Run it — expect PASS**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run 'TestToulminGraphEffect|TestGraphEffects'`
Expected: PASS (existing `promote`/`cross_check` effect tests unaffected).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/card_effects.go apps/api/internal/agent/card_effects_test.go
git commit -m "feat(refactor2): toulmin graph effect mints claim/evidence/warrant/counter/concession (Slice 7 Task 3)"
```

---

### Task 4: S4 gate contract — drop `no_single_sourced_claim`, register `toulmin`

**Files:**
- Modify: `apps/api/internal/skills/specs/writing-project.json` (`build_argument`, ~L71; top-level `cards`, L5)
- Test: `apps/api/internal/skills/*_test.go` (existing skill-load/gate tests must stay green); add a focused reconciliation assertion if a natural home exists.

**Interfaces:**
- Consumes: the `toulmin` mint (Task 3) producing `evidence -[supports]-> claim` + `concession` node.
- Produces: S4 (`build_argument`) machine gate = `no_orphan_evidence` + `no_unsupported_claim` + `node_present{concession}`; `toulmin` present in `cards` and `build_argument.repertoire`.

- [ ] **Step 1: Edit the contract**

In `build_argument.gate.machine`, delete the line `{ "kind": "no_single_sourced_claim" }`. The block becomes:

```json
      "gate": {
        "machine": [
          { "kind": "no_orphan_evidence" },
          { "kind": "no_unsupported_claim" },
          { "kind": "node_present", "type": "concession" }
        ],
        "student_written": ["warrants", "steelman"],
        "human": ["warrant_quality_spot_check"]
      }
```

Add `"toulmin"` to `build_argument.repertoire`: `"repertoire": ["toulmin", "concession", "steelman"]`.

Add `"toulmin"` to the top-level `"cards"` array (L5): `"cards": ["craap", "sift", "concession", "steelman", "toulmin"]`.

- [ ] **Step 2: Write/adjust the reconciliation test**

If `apps/api/internal/agent/gate_test.go` already builds a GraphView for S4, add:

```go
func TestBuildArgumentGateSolidAfterToulminMint(t *testing.T) {
	// claim <-supports- evidence, plus a concession node: the three remaining
	// S4 predicates (no_orphan_evidence, no_unsupported_claim, node_present
	// concession) must all pass; a single supporting source is now acceptable.
	g := GraphView{
		Nodes: []GraphNodeView{
			{ID: "c1", Type: "claim"},
			{ID: "e1", Type: "evidence"},
			{ID: "cc1", Type: "concession"},
		},
		Edges: []GraphEdgeView{
			{Type: "supports", FromKind: "graph_node", FromID: "e1", ToKind: "graph_node", ToID: "c1"},
		},
	}
	for _, kind := range []string{"no_orphan_evidence", "no_unsupported_claim"} {
		ok, msg := evalMachineItem(GateItem{Kind: kind}, g)
		if !ok {
			t.Fatalf("%s failed: %s", kind, msg)
		}
	}
	ok, _ := evalMachineItem(GateItem{Kind: "node_present", Type: "concession"}, g)
	if !ok {
		t.Fatalf("concession node_present failed")
	}
}
```

Use the real predicate-eval entry point and GraphView constructors from the package (read `gate.go` + an existing `gate_test.go` case for the exact names — `evalMachineItem`/`GraphNodeView` are illustrative; match what the file uses).

- [ ] **Step 3: Run the skill + gate tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/skills/ ./internal/agent/ -run 'Gate|Skill|Contract|Reconcile|BuildArgument'`
Expected: PASS. Any test that asserted the old four-predicate S4 gate must be updated to the three-predicate gate (the copy/behavior change is intended per spec decision 3).

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/skills/specs/writing-project.json apps/api/internal/agent/gate_test.go
git commit -m "feat(refactor2): S4 gate = card completion (drop no_single_sourced_claim), register toulmin (Slice 7 Task 4)"
```

---

### Task 5: Toulmin surface rule in the classifier

**Files:**
- Modify: `apps/api/internal/agent/classifier.go` (`SurfaceCardCandidates`, ~L91; `const` block ~L51)
- Test: `apps/api/internal/agent/classifier_test.go`

**Interfaces:**
- Consumes: `GraphView{Nodes, Edges, Materials, CardInstances}`; the S4-reachable signal. A `claim` node marks S4 begun.
- Produces: `toulminCardID = "toulmin"`; `SurfaceCardCandidates` appends a `surface_card` candidate for `toulmin` when S4 is reachable, no `claim` node exists, and (existing guard) no card_instance is proposed/active.

**Note on "S4 reachable":** the cheapest honest signal available inside `SurfaceCardCandidates` (which sees only the graph, not the skill) is *"S3 has produced its structural output"* — i.e., at least one `cross-checked-by` **or** `evaluated-as` edge exists (a source has been evaluated), meaning the student is past raw intake and into evaluation. Toulmin surfaces once **any** source is evaluated and **no `claim` node exists yet**. This deliberately does not re-derive the full gate DAG in the classifier (that lives in `ReconcileGates`); it is a coarse "there is evaluated material to argue from, and no argument yet" trigger. Document this reasoning in a code comment.

- [ ] **Step 1: Write the failing test**

Add to `apps/api/internal/agent/classifier_test.go`:

```go
func TestSurfaceToulminWhenEvaluatedNoClaim(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
		Nodes:     []GraphNodeView{{ID: "q1", Type: "source_quality"}},
		Edges: []GraphEdgeView{
			{Type: "evaluated-as", FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "q1"},
		},
	}
	cands := SurfaceCardCandidates(g)
	if !hasCard(cands, "toulmin") {
		t.Fatalf("expected toulmin surfaced when a source is evaluated and no claim exists; got %v", cands)
	}
}

func TestNoToulminOnceClaimExists(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
		Nodes: []GraphNodeView{
			{ID: "q1", Type: "source_quality"},
			{ID: "c1", Type: "claim"},
		},
		Edges: []GraphEdgeView{
			{Type: "evaluated-as", FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "q1"},
		},
	}
	if hasCard(SurfaceCardCandidates(g), "toulmin") {
		t.Fatalf("toulmin must not surface once a claim node exists")
	}
}
```

Add the `hasCard` helper if absent:

```go
func hasCard(cands []Candidate, cardID string) bool {
	for _, c := range cands {
		if c.Verb == "surface_card" && c.CardID == cardID {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run 'Toulmin.*Claim|SurfaceToulmin'`
Expected: FAIL — no toulmin candidate emitted.

- [ ] **Step 3: Add the surface rule**

Add the const beside `craapCardID`/`siftCardID`:

```go
	// toulminCardID surfaces once the student has evaluated material to argue
	// from (any evaluated-as edge) and has not yet started an argument (no
	// claim node). It is project-scoped, not per-material, so it is decided
	// after the per-material loop, from graph-wide facts.
	toulminCardID = "toulmin"
```

In `SurfaceCardCandidates`, after the per-material loop and before `return out`, add:

```go
	// Project-scoped Toulmin surface (Slice 7): the argument builder is not
	// about one material, so it is decided from graph-wide state, not inside
	// the per-material loop. Trigger once ANY source is evaluated (there is
	// something to argue from) and NO claim node exists yet (no argument
	// started). The in-flight guard at the top of this function already
	// suppresses it while any card_instance is proposed/active.
	anyEvaluated := false
	hasClaim := false
	for _, e := range g.Edges {
		if e.Type == "evaluated-as" && e.FromKind == "material" {
			anyEvaluated = true
		}
	}
	for _, n := range g.Nodes {
		if n.Type == "claim" {
			hasClaim = true
		}
	}
	if anyEvaluated && !hasClaim {
		out = append(out, Candidate{
			Verb:       "surface_card",
			AnchorKind: "project",
			AnchorID:   "",
			CardID:     toulminCardID,
			Reason:     "sources evaluated but no argument started",
		})
	}
```

(If `Candidate.AnchorKind == "project"` needs to be an accepted value downstream, verify the surfacing/lifecycle path tolerates a non-material anchor — see Task 6, which exercises the full path. If the lifecycle requires a material anchor, this is where the project-target adaptation lands; make it in Task 6's scope and reference it here.)

- [ ] **Step 4: Run it — expect PASS**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run 'Toulmin|Surface'`
Expected: PASS — including the existing CRAAP/SIFT surface tests (the new block runs after, and the top-of-function in-flight guard is unchanged).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/classifier.go apps/api/internal/agent/classifier_test.go
git commit -m "feat(refactor2): surface Toulmin when sources evaluated and no claim yet (Slice 7 Task 5)"
```

---

### Task 6: End-to-end summon → submit → mint → gate (the 6c-lesson task)

**Files:**
- Test: the existing Go integration test that drives the real project turn/submit controller for CRAAP/SIFT (find it: `grep -rln "surface_card\|submitProjectCard\|DoneCard\|TestE2E" apps/api/internal`). Add a Toulmin case there so it uses the same seeded-project harness.
- Modify (only if the harness proves it necessary): `apps/api/internal/agent/card_lifecycle.go` and/or the summon SSE path, to tolerate a **project-targeted** card (empty `materialID`, empty anchor set) — see spec §5.1 / R2b.

**Interfaces:**
- Consumes: everything from Tasks 1–5.
- Produces: proof that a project seeded at S4 with an evaluated source and no claim (1) surfaces `toulmin` with `materialID == ""` and `anchors == []`, (2) accepts a complete graph submit, (3) mints the five nodes + `supports` + `cites` edges, (4) flips `build_argument` to solid.

- [ ] **Step 1: Write the end-to-end test**

Model it on the CRAAP/SIFT end-to-end test in the same file. Skeleton (adapt names to the real harness — read the existing test first):

```go
func TestE2EToulminBuildsArgument(t *testing.T) {
	// Arrange: seed a project whose graph has one article material evaluated
	// (an evaluated-as edge to a source_quality node) and no claim node — the
	// exact precondition Task 5 triggers on.
	env := newProjectTestEnv(t) // whatever the CRAAP e2e uses
	env.seedEvaluatedSource(t, "NASA Earth Observatory")

	// Act 1: run a turn; assert the controller surfaces toulmin, project-scoped.
	card := env.turnUntilCard(t, "toulmin")
	if card.MaterialID != "" {
		t.Fatalf("toulmin materialID = %q, want empty (project-targeted)", card.MaterialID)
	}
	if len(card.Anchors) != 0 {
		t.Fatalf("toulmin should surface with no pre-generated anchors, got %d", len(card.Anchors))
	}

	// Act 2: submit a complete graph envelope (five text anchors ≥12 chars;
	// evidence + concession each with a source anchor to a real material id).
	status := env.submitCard(t, card.CardInstanceID, completeToulminAnchors(env))
	if status != "completed" {
		t.Fatalf("submit status = %q, want completed", status)
	}

	// Assert: nodes + edges minted, gate solid.
	g := env.graph(t)
	for _, typ := range []string{"claim", "warrant", "evidence", "counter", "concession"} {
		if env.countNodes(g, typ) != 1 {
			t.Fatalf("node %s count = %d, want 1", typ, env.countNodes(g, typ))
		}
	}
	if !env.edgeExists(g, "supports", "evidence", "claim") {
		t.Fatalf("missing supports edge evidence->claim")
	}
	if env.stationState(t, "build_argument") != "done" {
		t.Fatalf("build_argument gate should be solid after mint")
	}
}
```

`completeToulminAnchors(env)` builds the same anchor set as Task 2's `full`, with `MaterialID` set to a real seeded material id (so the `cites` targets resolve — R4).

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/... -run TestE2EToulminBuildsArgument`
Expected: FAIL — first at the surface step or the project-target assertions.

- [ ] **Step 3: Make it pass**

Only the minimum: if the summon/lifecycle path assumes a material anchor and errors on the project-scoped candidate, adapt it to tolerate empty `materialID`/anchors (spec §5.1). Do **not** widen scope beyond making the real path work for a project-targeted, anchor-less card. If the CRAAP/SIFT harness helpers (`seedEvaluatedSource`, `turnUntilCard`, `submitCard`) don't exist by these names, use whatever the existing e2e test uses; do not invent a parallel harness.

- [ ] **Step 4: Run the full agent + integration suite (quiet Docker)**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./...`
Expected: PASS. Run nothing else against Docker concurrently.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal
git commit -m "test(refactor2): end-to-end Toulmin summon→submit→mint→gate; project-targeted card path (Slice 7 Task 6)"
```

(Stage explicit `apps/api/internal` paths — never a directory holding untracked user files.)

---

### Task 7: `graph` primitive component + GraphState⇄anchors serialization

**Files:**
- Create: `apps/web/src/primitives/graph/serialize.ts` + `serialize.test.ts`
- Create: `apps/web/src/primitives/graph/Graph.tsx` + `Graph.test.tsx`
- Reference (read for pattern): `apps/web/src/studio/StudioAnnotateCard.tsx` (controlled-card + `handleLock` anchor build), `packages/contracts/src/interactionPrimitive.ts` (`GraphState`).

**Interfaces:**
- Consumes: `GraphState` (`{nodes: GraphNodeUnit[]; edges: GraphEdgeUnit[]}`), `Anchor`, the card spec `params.slots`.
- Produces:
  - `graphStateToAnchors(state: GraphState): Anchor[]` — one text anchor per node (`dimension = node.type`, `answer = node.text`, `author = "student"`, `material_id = ""`), plus one source anchor per `cites` edge (`dimension = <from node>.type`, `material_id = edge.to`, `answer = ""`, `author = "student"`).
  - `anchorsToGraphState(anchors: Anchor[], slots: Slot[]): GraphState` — inverse: text anchors → nodes; source anchors → `cites` edges; a synthetic `supports` edge evidence→claim when both nodes exist.
  - `Graph` React component: `{ slots, state, lockedSources, onChange, onLock, onSkip }`. Renders the five-slot list per the design; every node/edge it writes is `author: "student"`.

- [ ] **Step 1: Write the failing serialization test**

`apps/web/src/primitives/graph/serialize.test.ts`:

```ts
import { expect, test } from "vitest";
import { graphStateToAnchors, anchorsToGraphState } from "./serialize";
import type { GraphState } from "@mind-imprint/contracts";

const slots = [
  { id: "claim", role: "核心主张", needSrc: false, q: "" },
  { id: "evidence", role: "支撑证据", needSrc: true, q: "" },
];

test("round-trips text + source through anchors", () => {
  const state: GraphState = {
    nodes: [
      { id: "claim", type: "claim", text: "核心判断一句话说清楚。", author: "student" },
      { id: "evidence", type: "evidence", text: "证据如何支撑主张。", author: "student" },
    ],
    edges: [
      { id: "e1", from: "evidence", to: "m_nasa", type: "cites" },
      { id: "e2", from: "evidence", to: "claim", type: "supports" },
    ],
  };
  const anchors = graphStateToAnchors(state);
  // two text anchors + one source anchor (supports edge is not an anchor)
  expect(anchors.filter((a) => a.answer !== "")).toHaveLength(2);
  expect(anchors.filter((a) => a.material_id !== "")).toHaveLength(1);
  expect(anchors.every((a) => a.author === "student")).toBe(true);

  const back = anchorsToGraphState(anchors, slots);
  expect(back.nodes.map((n) => n.type).sort()).toEqual(["claim", "evidence"]);
  expect(back.edges.find((e) => e.type === "cites")?.to).toBe("m_nasa");
  expect(back.edges.some((e) => e.type === "supports" && e.from === "evidence" && e.to === "claim")).toBe(true);
});
```

- [ ] **Step 2: Run it — expect FAIL** (module not found)

Run: `npm --prefix apps/web test -- serialize`
Expected: FAIL.

- [ ] **Step 3: Implement `serialize.ts`**

```ts
import type { Anchor, GraphState, GraphNodeUnit } from "@mind-imprint/contracts";

type Slot = { id: string; role: string; needSrc: boolean; q: string };

let seq = 0;
const nid = () => `ga_${++seq}`;

export function graphStateToAnchors(state: GraphState): Anchor[] {
  const out: Anchor[] = [];
  for (const n of state.nodes) {
    out.push({
      id: nid(), material_id: "", block_id: "", start: 0, end: 0, quote: "",
      dimension: n.type, author: "student", question: "", answer: n.text,
    });
  }
  const typeOf = new Map(state.nodes.map((n) => [n.id, n.type]));
  for (const e of state.edges) {
    if (e.type !== "cites") continue; // supports is structural, re-derived on load
    out.push({
      id: nid(), material_id: e.to, block_id: "", start: 0, end: 0, quote: "",
      dimension: typeOf.get(e.from) ?? e.from, author: "student", question: "", answer: "",
    });
  }
  return out;
}

export function anchorsToGraphState(anchors: Anchor[], slots: Slot[]): GraphState {
  const nodes: GraphNodeUnit[] = [];
  const edges: GraphState["edges"] = [];
  for (const slot of slots) {
    const text = anchors.find((a) => a.dimension === slot.id && a.answer.trim() !== "");
    if (!text) continue;
    nodes.push({ id: slot.id, type: slot.id, text: text.answer, author: "student" });
    for (const src of anchors.filter((a) => a.dimension === slot.id && a.material_id !== "")) {
      edges.push({ id: nid(), from: slot.id, to: src.material_id, type: "cites" });
    }
  }
  const hasEvidence = nodes.some((n) => n.type === "evidence");
  const hasClaim = nodes.some((n) => n.type === "claim");
  if (hasEvidence && hasClaim) {
    edges.push({ id: nid(), from: "evidence", to: "claim", type: "supports" });
  }
  return { nodes, edges };
}
```

- [ ] **Step 4: Run it — expect PASS**

Run: `npm --prefix apps/web test -- serialize`
Expected: PASS.

- [ ] **Step 5: Write the failing `Graph.tsx` component test**

`apps/web/src/primitives/graph/Graph.test.tsx`:

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { Graph } from "./Graph";

const slots = [
  { id: "claim", role: "核心主张", needSrc: false, q: "你要论证的核心判断，用一句话说清。" },
  { id: "evidence", role: "支撑证据", needSrc: true, q: "挑一条证据，用自己的话概括它如何支撑主张。" },
];
const lockedSources = [{ id: "m_nasa", name: "NASA Earth Observatory" }];

test("renders one row per slot with its verbatim role and question", () => {
  render(<Graph slots={slots} state={{ nodes: [], edges: [] }} lockedSources={lockedSources} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />);
  expect(screen.getByText("核心主张")).toBeInTheDocument();
  expect(screen.getByText("支撑证据")).toBeInTheDocument();
});

test("lock is gated until every slot has text and needSrc slots have a source", () => {
  const onLock = vi.fn();
  let state = { nodes: [], edges: [] } as any;
  const onChange = (s: any) => { state = s; };
  const { rerender } = render(<Graph slots={slots} state={state} lockedSources={lockedSources} onChange={onChange} onLock={onLock} onSkip={() => {}} />);
  const lock = screen.getByRole("button", { name: /锁定|完成/ });
  expect(lock).toBeDisabled();
  // (drive slot text + source selection via the rendered controls, rerender with
  //  the updated state, then assert the button enables and onLock fires — mirror
  //  StudioAnnotateCard.test.tsx's fill-then-lock flow.)
});
```

- [ ] **Step 6: Run it — expect FAIL** (component not found)

Run: `npm --prefix apps/web test -- Graph`
Expected: FAIL.

- [ ] **Step 7: Implement `Graph.tsx`**

Build the controlled component from the design (`StructureView.tsx` L64–180 `RoleCard` is the exact visual target — reuse its markup, but make the source chips and textarea live). Props: `{ slots: Slot[]; state: GraphState; lockedSources: {id,name}[]; onChange(state); onLock(); onSkip() }`. For each slot: role chip + status + the coach question + (needSrc) the source chips (toggle → add/remove a `cites` edge in `onChange`) + the textarea (edit → update the node's `text`). Completion mirror: `canLock = slots.every(s => hasText(s) && (!s.needSrc || hasSource(s)))`, computed over `state`. Every node written carries `author: "student"`. When `!canLock`, disable the lock button; for a slot with no offered sources at all, show a short 跳过 hint (the `StudioAnnotateCard` dead-button-explanation pattern — do not leave a dead control unexplained).

- [ ] **Step 8: Run it — expect PASS**

Run: `npm --prefix apps/web test -- Graph serialize`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/primitives/graph packages/contracts
git commit -m "feat(refactor2): graph primitive (typed-slot argument builder) + GraphState<->anchors serialize (Slice 7 Task 7)"
```

---

### Task 8: Wire StructureView live (center-pane render + submit)

**Files:**
- Create: `apps/web/src/studio/StudioToulminCard.tsx` (+ test) — the studio host that binds the active `toulmin` card_instance to `Graph`, serializes on lock, submits.
- Modify: `apps/web/src/studio/views/StructureView.tsx` — render `StudioToulminCard` when there is an active toulmin card; keep the deferred placeholder only when there is genuinely no card.
- Modify: `apps/web/src/studio/state.ts` — `StructureCardFx` gains the fields the live view needs (or the view reads the active card directly; pick the smaller change).
- Modify: `apps/web/src/studio/StudioContainer.tsx` — surface the active toulmin card to the structure view (mirror how the material view receives its active CRAAP card), and route the lock through the existing project-card submit path (`submitProjectCard` / `studioTurn`), retiring the local card only on `cardStatus === "completed"` (the `studioTurn.ts` contract).
- Reference (read): `apps/web/src/studio/StudioContainer.tsx` active-card wiring (~L77–229), `apps/web/src/studio/conversation.ts` `activeCardToState`, `apps/web/src/api/studioTurn.ts`.

**Interfaces:**
- Consumes: `Graph` + serialize (Task 7); the active card_instance from the conversation snapshot (`convSnapshot.card`); `lockedSources` = the CRAAP-locked materials from `views.material`.
- Produces: a live S4 that surfaces, renders center-pane, submits, mints, and flips the gate — the frontend half of Task 6's backend proof.

- [ ] **Step 1: Write the failing StudioToulminCard test**

`apps/web/src/studio/StudioToulminCard.test.tsx`:

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { StudioToulminCard } from "./StudioToulminCard";

const spec = { id: "toulmin", primitive: "graph", params: { slots: [
  { id: "claim", role: "核心主张", needSrc: false, q: "Q1" },
  { id: "evidence", role: "支撑证据", needSrc: true, q: "Q2" },
] } } as any;
const lockedSources = [{ id: "m_nasa", name: "NASA" }];

test("submits serialized anchors on lock", () => {
  const onSubmit = vi.fn();
  render(<StudioToulminCard spec={spec} cardInstanceId="ci1" lockedSources={lockedSources} onSubmit={onSubmit} onSkip={() => {}} />);
  // fill claim text, evidence text + source (drive the rendered controls),
  // click 锁定, assert onSubmit called with an anchors array containing a
  // student-authored claim text anchor and an evidence source anchor.
  // (mirror StudioAnnotateCard.test.tsx's fill-then-lock assertions)
});
```

- [ ] **Step 2: Run it — expect FAIL**

Run: `npm --prefix apps/web test -- StudioToulminCard`
Expected: FAIL (component missing).

- [ ] **Step 3: Implement `StudioToulminCard.tsx`**

Holds `GraphState` in `useState` (seeded from `anchorsToGraphState(instance.anchors, slots)` for rehydrate — the FIX-E rehydration lesson). Renders `Graph`. On lock: `onSubmit({ ...newEnvelope(spec.id, ""), anchors: graphStateToAnchors(state) })` (the `StudioAnnotateCard.handleLock` shape). On skip: `onSkip(scaffold.event_trace)`.

- [ ] **Step 4: Wire it into StructureView + StudioContainer**

`StructureView` renders `StudioToulminCard` when an active toulmin card exists; else the existing deferred placeholder. `StudioContainer` passes the active card + `lockedSources` down, and routes `onSubmit` through the same project-card submit the material view uses, retiring the local card only on `cardStatus === "completed"`.

- [ ] **Step 5: Run the web suite + tsc**

Run: `npm --prefix apps/web test`
Run: `npm --prefix apps/web run build` (or the repo's `tsc` gate)
Expected: PASS, no type errors, no orphaned props/imports.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/studio apps/web/src/primitives
git commit -m "feat(refactor2): live-wire S4 structure view — center-pane Toulmin card, submit→mint (Slice 7 Task 8)"
```

---

## After all tasks

- [ ] **Whole-branch review** (mandatory, most-capable model): run `scripts/review-package "$(git merge-base main HEAD)" HEAD` and dispatch the final reviewer with the printed path. This has caught a cross-layer defect for five consecutive slices (6c's unsummonable card lived in exactly the seam Task 6 now covers) — do not skip it. Dispatch ONE fix subagent with the complete findings list.
- [ ] **Finish the branch** via `superpowers:finishing-a-development-branch`: full Go suite (`CGO_ENABLED=0 go test -p 1 ./...`, quiet Docker) + web `vitest` + contracts + `tsc` all green → merge to main + push → update `docs/2026-07-11-whole-product-refactor-roadmap.md` (Slice 7 ☑ + per-slice log) and the `whole-product-refactor-2026-07` memory.

---

## Self-review

**Spec coverage:** §1 primitive shape → Task 7. §2 primitive/state/events → Task 7. §2.1 anchor carrier → Tasks 2/3/7. §3 Toulmin card → Task 1. §3.1 completion → Task 2. §3.2 mint → Task 3. §4 no signature change → Tasks 2/3 (verified: signatures untouched). §5 summon+render+e2e → Tasks 5/6/8. Gate change (decision 3) → Task 4. §7 testing → each task + the after-all suite. §8 R1 → Task 2/3 (CRAAP/SIFT stay green). R2/R2b → Task 6. R4 (cites target resolves) → Task 3 test + Task 6 real material ids. Out-of-scope (map⇄outline, concession/steelman migration) → not implemented, correct.

**Placeholder scan:** the two frontend component tests (Graph lock-flow, StudioToulminCard submit) carry prose "drive the controls" comments rather than full fill sequences — deliberate, because the exact control queries depend on the markup reused from `StudioAnnotateCard`/`RoleCard`; the implementer mirrors that file's proven fill-then-lock test. Every backend task carries complete code. No "TBD"/"handle edge cases".

**Type consistency:** `Slot{ID,Role,NeedSrc,Q}` (Go) ↔ `{id,role,needSrc,q}` (JSON/TS) consistent across Tasks 1/2/3/5/7. `graphStateToAnchors`/`anchorsToGraphState` names consistent Tasks 7/8. `toulminCardID="toulmin"` matches the card id in Task 1. Anchor field names (`dimension`/`answer`/`material_id`/`author`) match the Go struct and TS contract read in grounding. `supports`/`cites` edge types consistent Tasks 3/4/7.

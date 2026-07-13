# Slice 6c — SIFT lateral reading · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship SIFT lateral reading — a `compare` primitive, SIFT as pure C2 config over it, a `cross_check` mint that records the student's own judgment, and the S3 gate that machine-checks lateral reading actually happened.

**Architecture:** `compare` is the second C1 primitive; its renderer *composes* the existing `Annotate` component rather than forking it. SIFT is card JSON only — it must touch zero renderer code. Card completion mints a `cross_check` graph node (never promoting the lateral source to `evidence`) and flips `source_log_entry.lateral_read`, both inside one new transactional store call that also fixes a pre-existing partial-mint hazard.

**Tech Stack:** Go (net/http, pgx, sqlc, goose, testcontainers) · TypeScript + React + vitest · Zod contracts (`packages/contracts`).

**Spec:** `docs/superpowers/specs/2026-07-13-slice-6c-sift-lateral-design.md` — read §2, §3, §4 before starting.

## Global Constraints

- **The client never calls a model directly.** All LLM calls go through `apps/api`; keys are server-side only. Never add a model call to `apps/web`.
- **No invented verdict.** The AI never judges a source's credibility and never picks the `relation` (印证 / 反驳 / 限定). If a value has no honest producer but the student, the student produces it. Do not render 可信 / 存疑 / 偏弱 anywhere.
- **RL-2 is structural.** The agent has no path that creates a material. Do not add one.
- **Binding Chinese UI copy is verbatim.** Copy in this plan comes from `docs/design/思维印记_工作区.dc.html` and `docs/工具包库/05-信息素养_SIFT横向核查卡.md`. If a test disagrees with the copy, the test is wrong.
- **Icons are inline SVG.** Never import `lucide-react` or any icon package.
- **Go tests must be serialized:** `CGO_ENABLED=0 go test -p 1 ./...` from `apps/api`. Parallel runs hang on Docker/testcontainers contention.
- **No orphans.** No unused files, props, exports, or imports left behind.
- **Docs in English**, except literal Chinese UI copy.
- **Never** put secrets in logs, errors, migrations, or payloads.
- **No migration in this slice.** `source_log_entry.lateral_read` and `.tier` already exist (migration `0016`). If you think you need a migration, you have misread the spec — stop and escalate.

---

## File Structure

| File | Responsibility |
|---|---|
| `packages/contracts/src/interactionPrimitive.ts` | + `ComparePair`, `CompareState` (C1) |
| `packages/contracts/cards/sift.json` | SIFT as C2 config over `compare` (rewrite of the legacy card) |
| `apps/api/internal/cards/loader.go` | + `Params.LateralDimension` |
| `apps/api/internal/agent/card_completion.go` | + `lateral_source_present` predicate |
| `apps/api/internal/agent/card_effects.go` | + `cross_check` graph effect; `crossCheckBody` |
| `apps/api/internal/agent/card_lifecycle.go` | material disambiguation (`checkedMaterialID`); mint via one transactional call |
| `apps/api/internal/agent/agentstore.go` | + `CommitCardMint` (atomic: nodes + edges + framework + source-log) |
| `apps/api/internal/agent/loop.go` | Store interface + `CommitCardMint` |
| `apps/api/internal/store/queries/source_log.sql` | + `MarkSourceLateralRead` |
| `packages/contracts/skills/writing-project.json` | S3 gate: `lateral_read_logged` self-attestation → `node_present(cross_check)` machine item |
| `apps/api/internal/studio/{projection,dto}.go` | `MaterialDTO.lateralRead`; `SourceLogDTO` 已横向核查; compare projection for the active card |
| `apps/web/src/primitives/compare/Compare.tsx` | the primitive — composes two `Annotate` panes + pair links |
| `apps/web/src/studio/StudioCompareCard.tsx` | the coach-rail card for a `compare` card |
| `apps/web/src/studio/{ViewFrame,material/SourceDossier}.tsx` | split pane when a compare card is active; the 正在核对 chip |

---

### Task 1: `compare` primitive contract (C1)

**Files:**
- Modify: `packages/contracts/src/interactionPrimitive.ts`
- Test: `packages/contracts/test/interactionPrimitive.test.ts`

**Interfaces:**
- Consumes: `Author`, `AnnotateState`, `AnnotateSpan` (already in this file).
- Produces: `ComparePair`, `CompareState` — consumed by Task 9 (renderer) and Task 10 (studio).

- [ ] **Step 1: Write the failing tests**

Append to `packages/contracts/test/interactionPrimitive.test.ts`:

```ts
import { CompareState, ComparePair } from "../src/interactionPrimitive";

const leftPane = {
  material_id: "mat-blog",
  spans: [{ id: "s1", block_ref: "b1", tag: "claim", note: "", author: "ai" as const }],
};

describe("CompareState", () => {
  it("accepts a right pane that is null — the empty right pane IS the assignment", () => {
    const parsed = CompareState.parse({ left: leftPane, right: null, pairs: [] });
    expect(parsed.right).toBeNull();
  });

  it("accepts a student-authored pair linking a left span to a right span", () => {
    const parsed = CompareState.parse({
      left: leftPane,
      right: { material_id: "mat-nasa", spans: [{ id: "s2", block_ref: "b9", tag: "find", note: "", author: "student" }] },
      pairs: [{ id: "p1", l_span: "s1", r_span: "s2", note: "NASA 只说绿化面积，没说可持续性", relation: "qualifies", author: "student" }],
    });
    expect(parsed.pairs[0].relation).toBe("qualifies");
  });

  it("rejects an AI-authored pair — the relation and the note are the student's judgment", () => {
    expect(() =>
      ComparePair.parse({ id: "p1", l_span: "s1", r_span: "s2", note: "x", relation: "corroborates", author: "ai" }),
    ).toThrow();
  });

  it("rejects a relation outside the closed set", () => {
    expect(() =>
      ComparePair.parse({ id: "p1", l_span: "s1", r_span: "s2", note: "x", relation: "debunks", author: "student" }),
    ).toThrow();
  });
});
```

- [ ] **Step 2: Run it and watch it fail**

Run: `cd packages/contracts && npx vitest run test/interactionPrimitive.test.ts`
Expected: FAIL — `CompareState` is not exported.

- [ ] **Step 3: Implement**

Append to `packages/contracts/src/interactionPrimitive.ts` (after `GraphState`):

```ts
// compare (C1): two materials side by side with paired annotations. `right` is
// nullable by design — the empty right pane is not a loading state, it is the
// assignment SIFT exists to resolve. Every pair is student-authored: the
// relation and the note are her judgment and have no other honest producer.
export const CompareRelation = z.enum(["corroborates", "contradicts", "qualifies"]);
export type CompareRelation = z.infer<typeof CompareRelation>;

export const ComparePair = z.object({
  id: z.string().min(1),
  l_span: z.string().min(1),
  r_span: z.string().min(1),
  note: z.string(),
  relation: CompareRelation,
  author: z.literal("student"),
});
export type ComparePair = z.infer<typeof ComparePair>;

export const CompareState = z.object({
  left: AnnotateState,
  right: AnnotateState.nullable(),
  pairs: z.array(ComparePair),
});
export type CompareState = z.infer<typeof CompareState>;
```

Export from `packages/contracts/src/index.ts` alongside the existing primitive exports.

- [ ] **Step 4: Run tests — expect PASS**

Run: `cd packages/contracts && npx vitest run`

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/interactionPrimitive.ts packages/contracts/src/index.ts packages/contracts/test/interactionPrimitive.test.ts
git commit -m "feat(refactor2): CompareState — the second C1 primitive contract"
```

---

### Task 2: SIFT as C2 card config

**Files:**
- Modify: `packages/contracts/cards/sift.json` (the legacy card is replaced by the C2 authoring)
- Test: `packages/contracts/test/cards.info-literacy.test.ts`

**Interfaces:**
- Produces: card id `sift`, `primitive: "compare"`, `params.lateral_dimension: "find"`, dimensions `stop` / `investigate` / `find` / `trace_origin` / `relation` / `tier_after`. Tasks 3–5 (Go completion/effects) and Task 10 (rail) all key off these exact dimension names.

**Context:** read `docs/工具包库/05-信息素养_SIFT横向核查卡.md` — the four steps and their copy come from there, not from your imagination. `packages/contracts/cards/craap.json` is the reference C2 authoring (`primitive` + `params` + `completion` + `graph_effects` + `steps[].methodology`).

- [ ] **Step 1: Write the failing test**

Append to `packages/contracts/test/cards.info-literacy.test.ts`:

```ts
import sift from "../cards/sift.json";
import { CardSpec } from "../src/cardSpec";

describe("sift card (C2 over compare)", () => {
  it("validates against the C2 card spec", () => {
    expect(() => CardSpec.parse(sift)).not.toThrow();
  });

  it("binds the compare primitive and declares which dimension carries the lateral source", () => {
    expect(sift.primitive).toBe("compare");
    expect(sift.params.lateral_dimension).toBe("find");
  });

  it("requires a real lateral source and a student-written trace — not a claim of having read laterally", () => {
    expect(sift.completion).toContainEqual({ kind: "lateral_source_present" });
    expect(sift.completion).toContainEqual({ kind: "field_written_by", field: "trace_origin", author: "student" });
  });

  it("mints a cross_check and never promotes the lateral source to evidence", () => {
    expect(sift.graph_effects).toEqual([{ kind: "cross_check" }]);
    expect(JSON.stringify(sift.graph_effects)).not.toContain("promote");
  });

  it("offers the relation as the student's own closed choice — the AI never picks it", () => {
    const relation = sift.steps.flatMap((s) => s.fields).find((f) => f.key === "relation");
    expect(relation?.type).toBe("single_choice");
    expect(relation?.options).toEqual(["印证", "反驳", "限定"]);
  });
});
```

- [ ] **Step 2: Run it and watch it fail**

Run: `cd packages/contracts && npx vitest run test/cards.info-literacy.test.ts`
Expected: FAIL — the legacy `sift.json` has no `primitive` / `params.lateral_dimension` / `graph_effects`.

- [ ] **Step 3: Rewrite `packages/contracts/cards/sift.json`**

Keep the existing legacy metadata keys (`id`, `category`, `name`, `name_en`, `purpose`, `trigger_condition`, `trigger_keywords`, `priority`, `disclosure_tier`, `age_band`, `interaction_type`, `rubric_dims`, `related`, `body_status`, `rubric_tags`) — copy them from the current file, do not invent new values. Add the C2 block:

```json
  "primitive": "compare",
  "target_type": "material.source",
  "params": {
    "lateral_dimension": "find",
    "tags": ["stop", "investigate", "find", "trace_origin"],
    "tag_prompts": {
      "stop": "先别急着信也别急着转。你现在的第一反应是什么？",
      "investigate": "另开一个标签，搜这个来源/作者是谁。它是机构、个人，还是营销号？",
      "find": "找一个**独立的**来源，看它怎么说同一件事。",
      "trace_origin": "这条说法/数据/图片，最原始的出处在哪？"
    }
  },
  "completion": [
    { "kind": "lateral_source_present" },
    { "kind": "field_written_by", "field": "trace_origin", "author": "student" }
  ],
  "graph_effects": [{ "kind": "cross_check" }],
  "observe": [
    { "when": "tag=find AND note_len<15", "move": { "verb": "post_intervention", "level": "I2" } }
  ],
  "consolidation": "reveal_framework_after_completion",
  "intrusiveness_cap": "I3",
```

Then author `steps` — four steps, keys `stop` / `investigate` / `find` / `trace`, each with a `methodology` block (`why` / `how` / `when` / `example`) in the voice of `craap.json`, drawing its content from `docs/工具包库/05-信息素养_SIFT横向核查卡.md` (§工具包库解释, §分步脚本, §就地小讲解). Fields:

- `stop`: `{ "type": "textarea", "key": "stop", "label": "你的第一反应是什么？（先写下来——待会儿要回来对照）", "rows": 2 }`
- `investigate`: `{ "type": "textarea", "key": "investigate", "label": "这个来源是谁？机构、个人，还是营销号？", "rows": 2 }`
- `find`: the lateral step —
  ```json
  { "type": "textarea", "key": "find", "label": "这个独立来源怎么说同一件事？", "rows": 3 },
  { "type": "single_choice", "key": "relation", "label": "它和原来的说法是什么关系？", "options": ["印证", "反驳", "限定"] }
  ```
- `trace`: `{ "type": "textarea", "key": "trace_origin", "label": "最原始的出处是哪里？", "rows": 2 }` and the re-tier:
  ```json
  { "type": "single_choice", "key": "tier_after", "label": "横向查过之后，你把它放在信源金字塔的哪一层？", "options": ["原始证据", "一手报道", "二手评论", "营销/观点"] }
  ```

The `tier_after` options are the pyramid from the tool card (§渲染要点) — verbatim, in that order.

- [ ] **Step 4: Run tests — expect PASS**

Run: `cd packages/contracts && npx vitest run`
Also run `npm run sync-cards` if the repo has a card-sync target (check `package.json`); the Go side `go:embed`s the same JSON files, so no copy step should be needed — verify with `cd apps/api && CGO_ENABLED=0 go build ./...`.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/cards/sift.json packages/contracts/test/cards.info-literacy.test.ts
git commit -m "feat(refactor2): SIFT authored as C2 config over the compare primitive"
```

---

### Task 3: Material disambiguation (`params.lateral_dimension`)

**Files:**
- Modify: `apps/api/internal/cards/loader.go` (`Params`)
- Modify: `apps/api/internal/agent/card_lifecycle.go` (`anchoredMaterialID` → `checkedMaterialID`)
- Test: `apps/api/internal/agent/card_lifecycle_test.go`

**Interfaces:**
- Produces:
  - `cards.Params.LateralDimension string` (json `lateral_dimension`)
  - `func checkedMaterialID(spec cards.Spec, anchors []Anchor) string`
  - `func lateralAnchor(spec cards.Spec, anchors []Anchor) (Anchor, bool)`
  - Both consumed by Tasks 4, 5, 7.

**Why this task exists (read this):** `anchoredMaterialID` returns *the first anchor carrying a material id*, and its comment states the assumption out loud: *"every anchor on one card_instance targets the same material."* **SIFT is the first card that violates it** — its anchors span two materials by design. Left alone, the mint attaches the cross-check to whichever material happens to sort first in the array: a coin flip between the source under review and the source used to check it, failing silently. The fix is **declaration, not inference**.

- [ ] **Step 1: Write the failing tests**

In `apps/api/internal/agent/card_lifecycle_test.go`:

```go
func TestCheckedMaterialID_IgnoresAnchorOrder(t *testing.T) {
	spec := cards.Spec{ID: "sift", Params: cards.Params{LateralDimension: "find"}}
	// The lateral anchor sorts FIRST — the trap. The checked material must
	// still be the source under review, not the source used to check it.
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-nasa", Dimension: "find", Answer: "NASA 只说绿化面积", Author: "student"},
		{ID: "a2", MaterialID: "mat-blog", Dimension: "stop", Answer: "有点夸张", Author: "student"},
	}
	if got := checkedMaterialID(spec, anchors); got != "mat-blog" {
		t.Fatalf("checked material = %q, want mat-blog (the source under review)", got)
	}
	lat, ok := lateralAnchor(spec, anchors)
	if !ok || lat.MaterialID != "mat-nasa" {
		t.Fatalf("lateral anchor = %+v ok=%v, want mat-nasa", lat, ok)
	}
}

func TestCheckedMaterialID_CardWithoutLateralDimension_Unchanged(t *testing.T) {
	// CRAAP and every card that exists today: no lateral_dimension, so this
	// must degenerate to exactly the old first-anchor behavior.
	spec := cards.Spec{ID: "craap"}
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "authority", Answer: "机构", Author: "student"},
	}
	if got := checkedMaterialID(spec, anchors); got != "mat-blog" {
		t.Fatalf("checked material = %q, want mat-blog", got)
	}
	if _, ok := lateralAnchor(spec, anchors); ok {
		t.Fatal("a card with no lateral_dimension must have no lateral anchor")
	}
}
```

- [ ] **Step 2: Run and watch it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/ -run TestCheckedMaterialID -v`
Expected: FAIL — `checkedMaterialID` undefined; `cards.Params` has no `LateralDimension`.

- [ ] **Step 3: Implement**

In `apps/api/internal/cards/loader.go`, add to `Params`:

```go
	// LateralDimension names the dimension whose anchor carries a DIFFERENT
	// material than the card's own (SIFT's lateral source). Empty for every
	// single-material card, which is all of them except compare cards.
	LateralDimension string `json:"lateral_dimension"`
```

In `apps/api/internal/agent/card_lifecycle.go`, replace `anchoredMaterialID` (delete it — no orphans) with:

```go
// checkedMaterialID reads the material the card is ABOUT — the source under
// review. For a single-material card (no lateral_dimension: every card but a
// compare card) this is the first anchored material, exactly as before. For a
// compare card, whose anchors span two materials by design, the lateral
// anchor is excluded by declaration — never by array order, which is a coin
// flip.
func checkedMaterialID(spec cards.Spec, anchors []Anchor) string {
	for _, a := range anchors {
		if a.MaterialID == "" {
			continue
		}
		if spec.Params.LateralDimension != "" && a.Dimension == spec.Params.LateralDimension {
			continue
		}
		return a.MaterialID
	}
	return ""
}

// lateralAnchor returns the anchor carrying the lateral source (the source
// used to check the card's own source). Only compare cards declare one.
func lateralAnchor(spec cards.Spec, anchors []Anchor) (Anchor, bool) {
	if spec.Params.LateralDimension == "" {
		return Anchor{}, false
	}
	for _, a := range anchors {
		if a.Dimension == spec.Params.LateralDimension && a.MaterialID != "" {
			return a, true
		}
	}
	return Anchor{}, false
}
```

Update the `CompleteCard` call site: `materialID := checkedMaterialID(spec, anchors)`.

- [ ] **Step 4: Run — expect PASS, and the whole agent package still green**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/`

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/cards/loader.go apps/api/internal/agent/card_lifecycle.go apps/api/internal/agent/card_lifecycle_test.go
git commit -m "fix(refactor2): identify a card's checked material by declaration, not anchor order"
```

---

### Task 4: `lateral_source_present` completion predicate

**Files:**
- Modify: `apps/api/internal/agent/card_completion.go`
- Test: `apps/api/internal/agent/card_completion_test.go`

**Interfaces:**
- Consumes: `checkedMaterialID`, `lateralAnchor` (Task 3); `cards.CompletionPredicate` (existing).
- Produces: completion kind `"lateral_source_present"`.

**The point of this predicate:** a written claim of having read laterally is not lateral reading. It passes only when a *different, real* source is in the project and she has written what it says. The AI cannot satisfy it — it has no path that creates a material.

- [ ] **Step 1: Write the failing tests**

In `apps/api/internal/agent/card_completion_test.go`:

```go
func siftSpec() cards.Spec {
	return cards.Spec{
		ID:         "sift",
		Params:     cards.Params{LateralDimension: "find"},
		Completion: []cards.CompletionPredicate{{Kind: "lateral_source_present"}},
	}
}

func TestLateralSourcePresent_RequiresADifferentMaterial(t *testing.T) {
	// She wrote a "find" answer, but it is anchored to the SAME source she is
	// checking. That is not lateral reading — it is reading the page again.
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "stop", Answer: "夸张", Author: "student"},
		{ID: "a2", MaterialID: "mat-blog", Dimension: "find", Answer: "文章自己说的", Author: "student"},
	}
	complete, missing := EvaluateCompletion(siftSpec(), anchors)
	if complete {
		t.Fatal("must not complete: the 'lateral' source is the same material")
	}
	if len(missing) != 1 || missing[0] != "find" {
		t.Fatalf("missing = %v, want [find]", missing)
	}
}

func TestLateralSourcePresent_RequiresANonEmptyAnswer(t *testing.T) {
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "stop", Answer: "夸张", Author: "student"},
		{ID: "a2", MaterialID: "mat-nasa", Dimension: "find", Answer: "   ", Author: "student"},
	}
	if complete, _ := EvaluateCompletion(siftSpec(), anchors); complete {
		t.Fatal("must not complete: a source was added but she said nothing about it")
	}
}

func TestLateralSourcePresent_CompletesWithARealOtherSource(t *testing.T) {
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "stop", Answer: "夸张", Author: "student"},
		{ID: "a2", MaterialID: "mat-nasa", Dimension: "find", Answer: "NASA 只说绿化面积，没说可持续", Author: "student"},
	}
	complete, missing := EvaluateCompletion(siftSpec(), anchors)
	if !complete {
		t.Fatalf("must complete: a real other source is in the project; missing = %v", missing)
	}
}
```

- [ ] **Step 2: Run and watch it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/ -run TestLateralSourcePresent -v`
Expected: FAIL — the unknown kind is skipped, so `EvaluateCompletion` returns `complete=true` vacuously. (That vacuity is exactly what these tests exist to kill.)

- [ ] **Step 3: Implement**

Add a case to `EvaluateCompletion`'s switch in `apps/api/internal/agent/card_completion.go`:

```go
		case "lateral_source_present":
			if !lateralSourcePresent(spec, anchors) {
				missing = append(missing, spec.Params.LateralDimension)
			}
```

and the predicate:

```go
// lateralSourcePresent reports whether the student has actually read
// laterally: an anchor on the card's lateral dimension, carrying a material
// that is NOT the one under review, with something written about it. A claim
// of having read laterally is not lateral reading — and the AI cannot satisfy
// this, because no agent path creates a material (RL-2).
func lateralSourcePresent(spec cards.Spec, anchors []Anchor) bool {
	lat, ok := lateralAnchor(spec, anchors)
	if !ok {
		return false
	}
	if strings.TrimSpace(lat.Answer) == "" {
		return false
	}
	return lat.MaterialID != checkedMaterialID(spec, anchors)
}
```

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/`

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/card_completion.go apps/api/internal/agent/card_completion_test.go
git commit -m "feat(refactor2): lateral_source_present — completion requires a real independent source"
```

---

### Task 5: The `cross_check` graph effect

**Files:**
- Modify: `apps/api/internal/agent/card_effects.go`
- Test: `apps/api/internal/agent/card_effects_test.go`

**Interfaces:**
- Consumes: `checkedMaterialID`, `lateralAnchor` (Task 3); `MintNode`, `MintEdge` (existing).
- Produces: graph_effect kind `"cross_check"`; node type `"cross_check"`; edge types `"cross-checked-by"` and `"cites"`. Task 8's gate config keys off the node type `cross_check`.

**Read spec §4.1 first.** SIFT must **not** promote the lateral source to `evidence`: promotion means *"evaluated, may be cited,"* and a source that arrived ten seconds ago has been evaluated by nobody. Promoting here would let a source become citable without evaluation and hollow out the `every_source_evaluated` gate.

- [ ] **Step 1: Write the failing tests**

In `apps/api/internal/agent/card_effects_test.go`:

```go
func TestGraphEffects_CrossCheck(t *testing.T) {
	spec := cards.Spec{
		ID:           "sift",
		Params:       cards.Params{LateralDimension: "find"},
		GraphEffects: []cards.GraphEffect{{Kind: "cross_check"}},
	}
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "stop", Answer: "标题很夸张", Author: "student"},
		{ID: "a2", MaterialID: "mat-nasa", Dimension: "find", Answer: "NASA 只讲绿化面积", Author: "student"},
		{ID: "a3", MaterialID: "mat-blog", Dimension: "relation", Answer: "限定", Author: "student"},
		{ID: "a4", MaterialID: "mat-blog", Dimension: "trace_origin", Answer: "NASA Earth Observatory 2019", Author: "student"},
		{ID: "a5", MaterialID: "mat-blog", Dimension: "tier_after", Answer: "二手评论", Author: "student"},
	}

	nodes, edges := GraphEffects(spec, "mat-blog", anchors)

	if len(nodes) != 1 || nodes[0].Type != "cross_check" {
		t.Fatalf("nodes = %+v, want exactly one cross_check node", nodes)
	}
	if nodes[0].Author != "student" {
		t.Fatalf("cross_check author = %q, want student — the relation is her judgment", nodes[0].Author)
	}
	if nodes[0].Body["relation"] != "限定" {
		t.Fatalf("relation = %v, want 限定 (the student's own choice)", nodes[0].Body["relation"])
	}
	if nodes[0].Body["trace_origin"] != "NASA Earth Observatory 2019" {
		t.Fatalf("trace_origin = %v", nodes[0].Body["trace_origin"])
	}
	if nodes[0].Body["tier_after"] != "二手评论" {
		t.Fatalf("tier_after = %v", nodes[0].Body["tier_after"])
	}

	// Two edges: the checked source -> the cross_check -> the lateral source.
	if len(edges) != 2 {
		t.Fatalf("edges = %+v, want 2", edges)
	}
	if edges[0].Type != "cross-checked-by" || edges[0].FromID != "mat-blog" || edges[0].ToID != "$new:0" {
		t.Fatalf("edge[0] = %+v, want mat-blog --cross-checked-by--> $new:0", edges[0])
	}
	// The placeholder on the edge SOURCE is the one an implementer is likely to
	// get wrong; CompleteCard resolves both endpoints (card_lifecycle.go).
	if edges[1].Type != "cites" || edges[1].FromID != "$new:0" || edges[1].ToID != "mat-nasa" {
		t.Fatalf("edge[1] = %+v, want $new:0 --cites--> mat-nasa", edges[1])
	}
}

func TestGraphEffects_CrossCheck_NeverPromotesTheLateralSource(t *testing.T) {
	spec := cards.Spec{
		ID:           "sift",
		Params:       cards.Params{LateralDimension: "find"},
		GraphEffects: []cards.GraphEffect{{Kind: "cross_check"}},
	}
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "stop", Answer: "夸张", Author: "student"},
		{ID: "a2", MaterialID: "mat-nasa", Dimension: "find", Answer: "NASA 讲绿化", Author: "student"},
	}
	nodes, _ := GraphEffects(spec, "mat-blog", anchors)
	for _, n := range nodes {
		if n.Type == "evidence" {
			t.Fatal("a cross-check must never promote the lateral source to evidence — it has been evaluated by nobody")
		}
	}
}
```

- [ ] **Step 2: Run and watch it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/ -run TestGraphEffects_CrossCheck -v`
Expected: FAIL — `GraphEffects` skips any kind that is not `promote`, so it returns no nodes.

- [ ] **Step 3: Implement**

In `apps/api/internal/agent/card_effects.go`, restructure `GraphEffects`'s loop from `if effect.Kind != "promote" { continue }` into a `switch effect.Kind` with the existing `promote` body in one case, and add:

```go
		case "cross_check":
			lat, ok := lateralAnchor(spec, anchors)
			if !ok {
				continue
			}
			nodes = append(nodes, MintNode{
				Type:   "cross_check",
				Author: "student",
				Body:   crossCheckBody(anchors),
			})
			ref := fmt.Sprintf("$new:%d", len(nodes)-1)
			edges = append(edges,
				// the source under review --was checked by--> this cross-check
				MintEdge{Type: "cross-checked-by", FromKind: "material", FromID: materialID, ToKind: "graph_node", ToID: ref},
				// ...which --cites--> the independent source she went and found
				MintEdge{Type: "cites", FromKind: "graph_node", FromID: ref, ToKind: "material", ToID: lat.MaterialID},
			)
```

and the body builder:

```go
// crossCheckBody carries what the student produced by reading laterally: the
// relation SHE chose (印证/反驳/限定 — the AI never picks it), where she traced
// the claim to, her first reaction, and the pyramid tier she landed on after
// checking. tier_before is filled by the caller (Task 7) from the source log,
// which is where her ingestion-time tier lives.
func crossCheckBody(anchors []Anchor) map[string]any {
	body := map[string]any{}
	for _, dim := range []string{"stop", "investigate", "find", "relation", "trace_origin", "tier_after", "revised_judgment"} {
		for _, a := range anchors {
			if a.Dimension == dim && strings.TrimSpace(a.Answer) != "" {
				body[dim] = a.Answer
				break
			}
		}
	}
	return body
}
```

Also fix the stale doc comment on `MintNode` / `MintEdge`: it claims *"MintEdge.ToID carries a deterministic placeholder"* — `CompleteCard` resolves `$new:` on **both** endpoints (`card_lifecycle.go:117,121`), and `cross_check` relies on that. Correct the comment to say both.

- [ ] **Step 4: Run — expect PASS, whole package green**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/`

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/card_effects.go apps/api/internal/agent/card_effects_test.go
git commit -m "feat(refactor2): cross_check graph effect — the lateral read becomes a real node"
```

---

### Task 6: Make the card mint atomic (`CommitCardMint`)

**Files:**
- Modify: `apps/api/internal/agent/loop.go` (the `Store` interface)
- Modify: `apps/api/internal/agent/agentstore.go` (+ `CommitCardMint`, + pool)
- Modify: `apps/api/internal/agent/card_lifecycle.go` (`CompleteCard` calls it once)
- Modify: `apps/api/cmd/api/main.go` and any other `NewSqlcAgentStore` call site
- Modify: the in-memory store fakes in `apps/api/internal/agent/loop_test.go` (and any other file implementing `Store`)
- Test: `apps/api/internal/store/refactor2_runtime_sqlc_test.go` (testcontainers — a real tx)

**Interfaces:**
- Produces:
  ```go
  type CardMint struct {
      Nodes     []MintNode
      Edges     []MintEdge
      Framework []byte
  }
  CommitCardMint(ctx context.Context, projectID, cardInstanceID uuid.UUID, m CardMint) error
  ```
  Task 7 extends `CardMint` with the source-log write.

**Why (read this):** `CompleteCard` currently inserts the node, then the edge, then sets `framework_fill`, as three separate calls with **no transaction**. The idempotency guard reads `framework_fill`, which is written **last** — so a failure between the node insert and the framework write leaves a partial mint, and the retry mints a **duplicate** node and edge. This is a pre-existing latent bug (CRAAP has it today). Slice 6c's mint is three writes instead of two and adds a source-log write in Task 7, so it must be atomic. Fixing it here is not scope creep — it is the precondition for the guarantee the spec makes.

- [ ] **Step 1: Write the failing test**

In `apps/api/internal/store/refactor2_runtime_sqlc_test.go` (testcontainers; follow the existing setup helpers in that file):

```go
// A mint that fails partway must leave NOTHING behind — otherwise the retry
// mints a duplicate evidence node, because the idempotency guard
// (framework_fill) is only written at the very end.
func TestCommitCardMint_IsAtomic(t *testing.T) {
	// ... use the file's existing harness to build a project + material + card_instance ...

	// An edge whose target material does not exist -> the edge insert fails.
	err := store.CommitCardMint(ctx, projectID, cardID, agent.CardMint{
		Nodes:     []agent.MintNode{{Type: "evidence", Author: "student", Body: map[string]any{"x": "y"}}},
		Edges:     []agent.MintEdge{{Type: "evaluated-as", FromKind: "material", FromID: uuid.New().String(), ToKind: "graph_node", ToID: "$new:0"}},
		Framework: []byte(`{"strategy":"reveal_framework_after_completion"}`),
	})
	if err == nil {
		t.Fatal("expected the mint to fail on a dangling edge endpoint")
	}

	// The node must have been rolled back with it.
	nodes := listGraphNodes(t, ctx, pool, projectID)
	if len(nodes) != 0 {
		t.Fatalf("partial mint survived: %d node(s) left behind — a retry would duplicate them", len(nodes))
	}
}
```

- [ ] **Step 2: Run and watch it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/store/ -run TestCommitCardMint -v`
Expected: FAIL — `CommitCardMint` undefined. (Once it compiles against the *old* non-transactional path, it fails on the leftover node — that is the bug.)

- [ ] **Step 3: Implement**

Give the store a pool so it can begin a transaction:

```go
type sqlcAgentStore struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func NewSqlcAgentStore(q *sqlc.Queries, pool *pgxpool.Pool) *sqlcAgentStore {
	return &sqlcAgentStore{q: q, pool: pool}
}
```

Update every `NewSqlcAgentStore` call site (`apps/api/cmd/api/main.go` and tests — grep for it).

```go
// CommitCardMint writes EVERYTHING a completed card produces — the minted
// nodes, their edges, and the consolidation framework — in ONE transaction.
// The framework write is the idempotency guard (isFrameworkSet), and it lands
// LAST, so before this method the sequence was: insert node, insert edge, set
// guard, with no transaction. A failure in the middle left a partial mint that
// the retry duplicated. Now it is all-or-nothing.
func (s *sqlcAgentStore) CommitCardMint(ctx context.Context, projectID, cardInstanceID uuid.UUID, m CardMint) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)

	nodeIDs := make(map[int]uuid.UUID, len(m.Nodes))
	for i, n := range m.Nodes {
		id, err := insertGraphNodeQ(ctx, qtx, projectID, n)
		if err != nil {
			return err
		}
		nodeIDs[i] = id
	}
	for _, e := range m.Edges {
		fromID, err := resolveMintRef(e.FromID, nodeIDs)
		if err != nil {
			return err
		}
		toID, err := resolveMintRef(e.ToID, nodeIDs)
		if err != nil {
			return err
		}
		e.FromID, e.ToID = fromID.String(), toID.String()
		if err := insertGraphEdgeQ(ctx, qtx, projectID, e); err != nil {
			return err
		}
	}
	if err := setCardInstanceFrameworkQ(ctx, qtx, projectID, cardInstanceID, m.Framework); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
```

Refactor the existing `InsertGraphNode` / `InsertGraphEdge` / `SetCardInstanceFramework` method bodies into `insertGraphNodeQ(ctx, q *sqlc.Queries, ...)` -style helpers taking a `*sqlc.Queries`, so both the plain methods and the tx path share one implementation — do not copy-paste the SQL mapping. Keep `InsertGraphNode`/`InsertGraphEdge` on the interface: `SurfaceCard` still uses `InsertGraphEdge` for its card_instance→material edge.

Rewrite `CompleteCard`'s tail to compute everything first, then commit once:

```go
	nodes, edges := GraphEffects(spec, materialID, anchors)
	framework, err := json.Marshal(ConsolidationPayload(spec))
	if err != nil {
		return false, err
	}
	if err := deps.Store.CommitCardMint(ctx, row.ProjectID, cardInstanceID, CardMint{
		Nodes: nodes, Edges: edges, Framework: framework,
	}); err != nil {
		return false, err
	}
	return true, nil
```

Add `CommitCardMint` to the `Store` interface in `loop.go` and to every fake implementing it (grep `InsertGraphNode` to find them). The fakes should record the mint and apply it in-memory so existing CRAAP tests keep asserting on minted nodes.

- [ ] **Step 4: Run — expect PASS, and the whole suite green**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./...`
The existing CRAAP mint tests must still pass — this is a refactor with one behavior change (atomicity).

- [ ] **Step 5: Commit**

```bash
git add apps/api/
git commit -m "fix(refactor2): make the card mint atomic — partial mints no longer duplicate on retry"
```

---

### Task 7: The source-log write (`lateral_read` + the re-tier)

**Files:**
- Modify: `apps/api/internal/store/queries/source_log.sql` (+ `MarkSourceLateralRead`)
- Modify: `apps/api/internal/agent/agentstore.go` (`CardMint.LateralRead`, applied inside the same tx)
- Modify: `apps/api/internal/agent/card_lifecycle.go` (fill it; read `tier_before` from the log)
- Modify: `apps/api/internal/agent/loop.go` (`GetSourceLogByMaterial` on the Store, if not already there)
- Test: `apps/api/internal/store/refactor2_runtime_sqlc_test.go`

**Interfaces:**
- Consumes: `CardMint` (Task 6), `lateralAnchor` / `checkedMaterialID` (Task 3), `crossCheckBody` (Task 5).
- Produces: `CardMint.LateralRead *LateralRead` where `type LateralRead struct { MaterialID uuid.UUID; TierAfter string }`.

**Read spec §4.3 and §5.** `lateral_read` flips on the **checked** source — the source that *was* laterally read. The lateral source is the *instrument*, not the subject; its own log entry is untouched. And `tier` is the student's ingestion-time pyramid tier from 6b: SIFT re-reads it as `tier_before`, so a change between it and `tier_after` is an *observable, attributable* stance change — the T2/T4 evidence Slice 10 exists to find. If the tier did not change, nothing special is emitted: a revision that did not happen is not recorded as one.

- [ ] **Step 1: Write the failing test**

In `apps/api/internal/store/refactor2_runtime_sqlc_test.go`:

```go
func TestCommitCardMint_FlipsLateralReadOnTheCheckedSourceOnly(t *testing.T) {
	// ... harness: a project with TWO materials, each with a source_log_entry;
	//     checked = the blog (tier 二手评论), lateral = the NASA page ...

	err := store.CommitCardMint(ctx, projectID, cardID, agent.CardMint{
		Nodes:       []agent.MintNode{{Type: "cross_check", Author: "student", Body: map[string]any{"relation": "限定"}}},
		Edges:       []agent.MintEdge{{Type: "cross-checked-by", FromKind: "material", FromID: blogID.String(), ToKind: "graph_node", ToID: "$new:0"}},
		Framework:   []byte(`{"strategy":"reveal_framework_after_completion"}`),
		LateralRead: &agent.LateralRead{MaterialID: blogID, TierAfter: "二手 · 需追源"},
	})
	if err != nil {
		t.Fatal(err)
	}

	blog := getSourceLog(t, ctx, pool, blogID)
	if !blog.LateralRead {
		t.Fatal("the CHECKED source must be marked laterally read")
	}
	if blog.Tier != "二手 · 需追源" {
		t.Fatalf("tier = %q, want the re-tier she landed on after checking", blog.Tier)
	}

	nasa := getSourceLog(t, ctx, pool, nasaID)
	if nasa.LateralRead {
		t.Fatal("the LATERAL source is the instrument, not the subject — its own entry must be untouched")
	}
}
```

- [ ] **Step 2: Run and watch it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/store/ -run TestCommitCardMint_Flips -v`
Expected: FAIL — `CardMint` has no `LateralRead` field.

- [ ] **Step 3: Implement**

Add to `apps/api/internal/store/queries/source_log.sql`:

```sql
-- name: MarkSourceLateralRead :exec
-- The source that WAS laterally read (not the source used to do it). tier is
-- overwritten only when the student re-tiered it after checking; an empty
-- tier_after leaves her ingestion-time tier alone.
UPDATE source_log_entry
SET lateral_read = true,
    tier = CASE WHEN @tier_after::text = '' THEN tier ELSE @tier_after::text END
WHERE project_id = @project_id AND material_id = @material_id;
```

Run `make sqlc` (which is `CGO_ENABLED=0 go tool sqlc generate`). Note sqlc does **not** delete stale generated files — check `git status` for surprises.

Extend `CardMint`:

```go
// LateralRead is the source-log side of a cross_check: the CHECKED source is
// marked laterally read and re-tiered to the pyramid level she landed on after
// checking. Written inside CommitCardMint's transaction, so the graph node
// (which the gate reads) and the log row (which the dossier chip and the
// ledger read) cannot disagree.
type LateralRead struct {
	MaterialID uuid.UUID
	TierAfter  string
}
```

and apply it in `CommitCardMint`, before the commit:

```go
	if m.LateralRead != nil {
		if err := qtx.MarkSourceLateralRead(ctx, sqlc.MarkSourceLateralReadParams{
			ProjectID:  pgtype.UUID{Bytes: projectID, Valid: true},
			MaterialID: pgtype.UUID{Bytes: m.LateralRead.MaterialID, Valid: true},
			TierAfter:  m.LateralRead.TierAfter,
		}); err != nil {
			return err
		}
	}
```

In `CompleteCard`, populate it only for a card that actually minted a cross-check, and fold `tier_before` into the node body by reading the checked source's current log tier **before** the mint:

```go
	var lateral *LateralRead
	for _, n := range nodes {
		if n.Type != "cross_check" {
			continue
		}
		checkedUUID, err := uuid.Parse(materialID)
		if err != nil {
			return false, fmt.Errorf("card: checked material id %q: %w", materialID, err)
		}
		before, err := deps.Store.GetSourceLogByMaterial(ctx, checkedUUID)
		if err == nil {
			n.Body["tier_before"] = before.Tier
		}
		tierAfter, _ := n.Body["tier_after"].(string)
		lateral = &LateralRead{MaterialID: checkedUUID, TierAfter: tierAfter}
	}
```

(`GetSourceLogByMaterial` already exists as a sqlc query from 6b — add it to the agent `Store` interface if it is not there yet. A missing log entry is not fatal: `tier_before` is simply absent.)

Pass `LateralRead: lateral` in the `CardMint`.

- [ ] **Step 4: Run — expect PASS, whole suite green**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./...`

- [ ] **Step 5: Commit**

```bash
git add apps/api/
git commit -m "feat(refactor2): a cross_check marks the checked source laterally read and records the re-tier"
```

---

### Task 8: The S3 gate machine-checks lateral reading

**Files:**
- Modify: `packages/contracts/skills/writing-project.json` (contract `evaluate_sources`)
- Modify: `apps/api/internal/skills/specs/writing-project.json` (generated mirror — via `make sync-skills`, never hand-edited)
- Test: `apps/api/internal/agent/gate_test.go`

**Interfaces:**
- Consumes: node type `cross_check` (Task 5).
- Produces: no new Go symbols. **This task must add zero lines to `gate.go`.**

**Read spec §7.** `node_present` is already in the closed machine-predicate set (`skills.MachineKinds`) and already takes a `type`, so lateral reading becomes a machine item with **pure skill config**. Today `lateral_read_logged` sits under `student_written` — the student *ticks that she did it*. After this task the machine can see it, so it moves tier. `attemptedFor` already returns `true` for `node_present` (positive presence, not a vacuous negation), so it needs no change either.

If you find yourself adding a `lateral_read_present` kind to `MachineKinds`, stop: that is the design we rejected. Use `node_present`.

- [ ] **Step 1: Write the failing tests**

In `apps/api/internal/agent/gate_test.go`:

```go
func TestS3Gate_RequiresARealCrossCheck(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "mat-blog", Kind: "article"}},
		Nodes:     []GraphNodeView{{ID: "n1", Type: "evidence"}},
		Edges:     []GraphEdgeView{{Type: "evaluated-as", FromID: "mat-blog", ToID: "n1"}},
	}
	item := skills.MachineItem{Kind: "node_present", Type: "cross_check"}
	if pass, _ := EvalMachineItemForTest(item, g); pass {
		t.Fatal("S3 must not pass on a CRAAP alone — she has not left the page")
	}

	g.Nodes = append(g.Nodes, GraphNodeView{ID: "n2", Type: "cross_check"})
	if pass, missing := EvalMachineItemForTest(item, g); !pass {
		t.Fatalf("S3 must pass once a real cross-check exists: %s", missing)
	}
}

func TestS3Gate_LateralReadIsNoLongerSelfAttested(t *testing.T) {
	sk := mustLoadWritingProjectSkill(t) // use the package's existing loader helper
	s3 := sk.Contracts["evaluate_sources"]
	for _, item := range s3.Gate.StudentWritten {
		if item == "lateral_read_logged" {
			t.Fatal("lateral_read_logged is still a student_written item — the machine can see it now")
		}
	}
	found := false
	for _, m := range s3.Gate.Machine {
		if m.Kind == "node_present" && m.Type == "cross_check" {
			found = true
		}
	}
	if !found {
		t.Fatal("S3's machine tier must require a cross_check node")
	}
}
```

(Match the exact `GraphView` / `Skill` field names used elsewhere in `gate_test.go` — read the file first.)

- [ ] **Step 2: Run and watch it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/ -run TestS3Gate -v`
Expected: FAIL — S3 has no `cross_check` machine item and still lists `lateral_read_logged` under `student_written`.

- [ ] **Step 3: Implement**

In `packages/contracts/skills/writing-project.json`, contract `evaluate_sources`:

```json
        "machine": [
          { "kind": "every_source_evaluated" },
          { "kind": "no_single_sourced_claim" },
          { "kind": "node_present", "type": "cross_check" }
        ],
        "student_written": ["source_risk_notes"],
```

(Remove `"lateral_read_logged"` from `student_written` — it has been promoted, not deleted.)

Then run `make sync-skills` from the repo root to regenerate `apps/api/internal/skills/specs/writing-project.json`. Never hand-edit the mirror.

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/ ./internal/skills/`
Then confirm you changed no Go gate code: `git diff --stat apps/api/internal/agent/gate.go` must be **empty**.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/skills/writing-project.json apps/api/internal/skills/specs/writing-project.json apps/api/internal/agent/gate_test.go
git commit -m "feat(refactor2): S3 machine-checks lateral reading — pure skill config, zero gate-engine change"
```

---

### Task 9: Studio projection — the dossier learns about lateral reading

**Files:**
- Modify: `apps/api/internal/studio/dto.go` (`MaterialDTO`, `SourceLogEntryDTO`)
- Modify: `apps/api/internal/studio/projection.go` (`projectMaterials`)
- Modify: `apps/api/internal/studio/load.go` if the graph nodes/edges are not already loaded there
- Test: `apps/api/internal/studio/projection_test.go`, `apps/api/internal/studio/dto_parity_test.go`

**Interfaces:**
- Consumes: node type `cross_check`, edge type `cross-checked-by` (Task 5); `source_log_entry.lateral_read` (Task 7).
- Produces: `MaterialDTO.lateralRead bool` (JSON key `lateralRead`) and `SourceLogEntryDTO.lateralRead bool`. Task 11 renders both.

**Read spec §6.** The chip is **derived, never decorated**: `正在核对 · 需横向阅读` shows when an **active (open, uncompleted) card exists on this material** AND the material has **no cross-check**. Both halves are facts about state. There is still **no credibility verdict anywhere** — do not add one.

- [ ] **Step 1: Write the failing test**

In `apps/api/internal/studio/projection_test.go` (follow the file's existing fixture style):

```go
func TestProjectMaterials_LateralReadIsDerivedFromTheMint(t *testing.T) {
	// A material with a source_log_entry whose lateral_read is true projects
	// as laterally read; one without does not. Nothing is invented: the flag
	// is written by the cross_check mint and read here.
	got := projectMaterials(/* ... fixture: blog laterally read, nasa not ... */)

	blog := findMaterial(t, got, "mat-blog")
	if !blog.LateralRead {
		t.Fatal("the checked source must project as laterally read")
	}
	nasa := findMaterial(t, got, "mat-nasa")
	if nasa.LateralRead {
		t.Fatal("the lateral source is the instrument, not the subject")
	}
}
```

Extend `dto_parity_test.go` so the new fields are covered by the existing Go↔TS parity assertion.

- [ ] **Step 2: Run and watch it fail**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/studio/ -v`
Expected: FAIL — `MaterialDTO` has no `LateralRead`.

- [ ] **Step 3: Implement**

Add `LateralRead bool \`json:"lateralRead"\`` to `MaterialDTO` and to the source-log entry DTO, and populate both from the `source_log_entry` rows `Load` already fetches (6b). Mirror the field in `packages/contracts/src/studioState.ts` (`MaterialSource`, and the source-log entry type) so the parity test passes.

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/studio/` and `cd packages/contracts && npx vitest run`

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/studio/ packages/contracts/src/studioState.ts
git commit -m "feat(refactor2): project lateralRead onto the dossier and the source log"
```

---

### Task 10: The `compare` primitive renderer

**Files:**
- Create: `apps/web/src/primitives/compare/Compare.tsx`
- Create: `apps/web/src/primitives/compare/Compare.test.tsx`
- Read first: `apps/web/src/primitives/annotate/` — the component you are composing.

**Interfaces:**
- Consumes: `CompareState` (Task 1); the existing `Annotate` component.
- Produces:
  ```tsx
  export function Compare(props: {
    state: CompareState;
    onAddLateralSource: () => void;   // opens 6b's 添加信源 — Compare never ingests
  }): JSX.Element
  ```

**The rule (spec §2, R3):** `Compare` **composes** `Annotate` — one instance per pane. If you find yourself duplicating span segmentation or highlight logic, stop: that is the fork this design exists to prevent, and it will be rejected in review. If `Annotate` cannot be reused as-is, fix `Annotate`'s props rather than copying it.

- [ ] **Step 1: Write the failing tests**

`apps/web/src/primitives/compare/Compare.test.tsx`:

```tsx
const left = { material_id: "mat-blog", spans: [{ id: "s1", block_ref: "b1", tag: "claim", note: "", author: "ai" as const }] };

it("renders the empty right pane as an assignment, not a spinner", () => {
  render(<Compare state={{ left, right: null, pairs: [] }} onAddLateralSource={vi.fn()} />);
  expect(screen.getByText("去找一个独立的来源")).toBeInTheDocument();
  expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
});

it("invites the student to add the source — it never fetches one itself", async () => {
  const onAdd = vi.fn();
  render(<Compare state={{ left, right: null, pairs: [] }} onAddLateralSource={onAdd} />);
  await userEvent.click(screen.getByRole("button", { name: "添加信源" }));
  expect(onAdd).toHaveBeenCalledTimes(1);
});

it("renders both materials once a lateral source exists", () => {
  const right = { material_id: "mat-nasa", spans: [{ id: "s2", block_ref: "b9", tag: "find", note: "", author: "student" as const }] };
  render(<Compare state={{ left, right, pairs: [] }} onAddLateralSource={vi.fn()} />);
  expect(screen.getAllByTestId("compare-pane")).toHaveLength(2);
});
```

- [ ] **Step 2: Run and watch it fail**

Run: `cd apps/web && npx vitest run src/primitives/compare/`
Expected: FAIL — the module does not exist.

- [ ] **Step 3: Implement**

Two panes side by side, each an `Annotate` instance rendering that pane's material blocks and spans; `data-testid="compare-pane"` on each. When `state.right` is null, the right pane renders the assignment state — the heading `去找一个独立的来源`, one line of guidance, and an `添加信源` button wired to `onAddLateralSource`. Pair links draw between `l_span` and `r_span`. Styling follows the dossier's existing card/blocks in `apps/web/src/studio/material/`. Inline SVG only.

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/web && npx vitest run src/primitives/compare/`

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/primitives/compare/
git commit -m "feat(refactor2): the compare primitive — two panes, composed from Annotate"
```

---

### Task 11: Wire SIFT into the Studio

**Files:**
- Create: `apps/web/src/studio/StudioCompareCard.tsx` (+ test)
- Modify: `apps/web/src/studio/ViewFrame.tsx` — render `Compare` in the 素材 center pane when the active card's primitive is `compare`
- Modify: `apps/web/src/studio/material/SourceDossier.tsx` — the `正在核对 · 需横向阅读` chip; `已横向核查` in the ledger
- Modify: `apps/web/src/studio/StudioContainer.tsx` — the lateral-source flow reuses 6b's `addMaterial`, then refetches
- Test: `apps/web/src/studio/StudioContainer.test.tsx`

**Interfaces:**
- Consumes: `Compare` (Task 10); `addMaterial` from `apps/web/src/api/materials.ts` (6b, unchanged); `MaterialSource.lateralRead` (Task 9).

**Read this — it is the lesson 6b paid for.** The Slice-6b whole-branch review found that locking a card changed *nothing on screen*: the server minted correctly, but the client never re-asked, so the chip never updated and the highlights blinked out. `StudioContainer` already has `refetchProject()` with a request-generation guard and `conv.dropFirst(n)` — **use them**. Completing a SIFT card must refetch, and a test must assert the chip clears. Do not add a second refetch path.

- [ ] **Step 1: Write the failing test**

In `apps/web/src/studio/StudioContainer.test.tsx`, following the file's existing `within(dossier-source-list)` scoping style:

```tsx
it("clears 需横向阅读 once the cross-check lands", async () => {
  // 1st GET: an active SIFT card on the blog, lateralRead false.
  // 2nd GET (after submit): no active card, lateralRead true.
  renderStudio();
  const dossier = await screen.findByTestId("dossier-source-list");
  expect(within(dossier).getByText(/需横向阅读/)).toBeInTheDocument();

  await userEvent.click(screen.getByRole("button", { name: "锁定这张卡" }));

  await waitFor(() => {
    expect(within(dossier).queryByText(/需横向阅读/)).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run and watch it fail**

Run: `cd apps/web && npx vitest run src/studio/StudioContainer.test.tsx`

- [ ] **Step 3: Implement**

`StudioCompareCard` is the coach-rail sibling of `StudioAnnotateCard`: it renders the card's anchors (chip = dimension, question, answer), plus the `relation` single-choice and the `tier_after` single-choice, and the 添加信源 affordance for the lateral step. It must **not** copy `StudioAnnotateCard`'s `anchors[0].material_id` shortcut — a compare card's anchors span two materials, and the lateral anchor carries the *lateral* material id (Task 3's trap, client-side).

`ViewFrame` picks the renderer off the active card's `primitive` — `annotate` → today's path, `compare` → `Compare`. Do not branch on the card **id**; branching on `sift` specifically would break the architectural claim this slice exists to prove.

`SourceDossier` renders the chip from state: an active card on this material AND `!lateralRead` → `正在核对 · 需横向阅读` (verbatim, with the design's second line `点亮的句子 = 印记标出的可疑处`). The ledger shows `已横向核查` on entries with `lateralRead`.

- [ ] **Step 4: Run — expect PASS, whole web suite green**

Run: `cd apps/web && npx vitest run && npx tsc --noEmit`

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/
git commit -m "feat(refactor2): SIFT live in the Studio — split pane, lateral ingestion, derived chip"
```

---

### Task 12: Sweep, gate, docs

**Files:**
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md` (Slice 6 row → 6c ☑; per-slice log entry)
- Modify: `docs/superpowers/specs/2026-07-13-slice-6c-sift-lateral-design.md` (correct §4.2 if the implementation diverged)
- Modify: `AGENTS.md` **only if** this slice introduced a constraint a future contributor must not violate

- [ ] **Step 1: Orphan sweep**

Confirm nothing was left behind:

```bash
grep -rn "anchoredMaterialID\|lateral_read_logged" apps/api packages --include=*.go --include=*.json
```
Expected: no hits outside the spec/plan docs (both were removed, not shadowed).

Confirm the architectural claim held — SIFT touched zero renderer code beyond the primitive:

```bash
git diff --stat main...HEAD -- apps/web/src/cards/
```
Expected: **empty**. If it is not, the primitive boundary is wrong; escalate rather than rationalizing.

- [ ] **Step 2: Full gate**

```bash
cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test -p 1 ./...
cd ../web && npx vitest run && npx tsc --noEmit
cd ../../packages/contracts && npx vitest run
```
All green. Record the counts.

- [ ] **Step 3: Update the roadmap**

Slice 6's status cell → `☑` (6c done, Slice 6 complete). Add the per-slice log entry: spec + plan links, what landed, the two pre-existing defects this slice fixed (the anchor-order material trap; the non-transactional mint), and the carry-forwards from spec §10.

- [ ] **Step 4: Commit**

```bash
git add docs/
git commit -m "docs(refactor2): Slice 6c complete — SIFT lateral reading live"
```

---

## Self-Review (done)

**Spec coverage.** §2 primitive → T1, T10. §2.1 projection → T1, T10. §2.2 `lateral_dimension` → T3. §3 card config → T2. §3.1 completion → T4. §4 mint → T5 (+ T6 atomicity, T7 log). §5 before/after tier → T7. §6 UI → T9, T11. §7 gate → T8. §8 risks → T5 (R1 asserted in the gate test), T10 (R3 stated as a review rule), T12 (R4 asserted by the `git diff --stat` check). §9 testing → distributed. §10 carry-forwards → T12.

**Type consistency.** `checkedMaterialID` / `lateralAnchor` (T3) are used with those exact names in T4, T5, T7. `CardMint` (T6) is extended by `LateralRead` (T7) — same struct, same file. `MaterialDTO.lateralRead` (T9) is the key T11 reads. Card dimensions (`stop` / `investigate` / `find` / `relation` / `trace_origin` / `tier_after`) are fixed in T2 and referenced identically in T4, T5, T7, T11.

**Known gap, deliberately left:** Tasks 6, 7, 9 lean on test harness helpers that already exist in their files (`listGraphNodes`, `getSourceLog`, the studio fixtures). The implementer must read the file and match its existing style rather than inventing a parallel harness — called out in each task.

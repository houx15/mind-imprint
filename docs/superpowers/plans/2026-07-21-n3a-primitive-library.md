# N3a · Complete the Interaction-Primitive Library — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `sort`, `scale`, and `matrix` as first-class C1 interaction
primitives (3/6 → 6/6), each bound to one thinking-framework card.

**Architecture:** All three persist into the **existing** `Anchor` shape on
`card_instance.anchors` — `quote` = the item/row, `dimension` = the
bucket/stop/column, `answer` = the student's reason/cell. No migration, no
`Anchor` contract change, no LLM call. Backend gains two closed-set completion
predicates and one graph effect; frontend gains three `primitives/*` modules
(mirroring `primitives/graph/`) and three `Studio*Card` hosts rendered in the
CoachRail beside the existing annotate/compare hosts.

**Tech Stack:** Go (`apps/api`), TypeScript/Zod (`packages/contracts`),
React + Vite (`apps/web`).

Spec: `docs/superpowers/specs/2026-07-21-n3a-primitive-library-design.md`.

## Global Constraints

- **Client never calls a model.** This slice adds ZERO LLM calls.
  `apps/api/internal/api/studioturn.go`'s `spec.Primitive == "annotate" ||
  spec.Primitive == "compare"` anchor-seeding branch is **untouched** — the
  three new primitives seed deterministically from `params`, like `graph`.
- **No migration, no `sqlc` change, no `Anchor` field added.**
  `graph_node.type` is an open type (migration 0017 `CHECK type <> ''`), so
  minting `perspective` nodes needs no schema change.
- **Card JSON is authored in `packages/contracts/cards/` and mirrored into
  `apps/api/internal/cards/specs/` by `cd apps/api && make sync-cards`.**
  NEVER hand-edit `apps/api/internal/cards/specs/*.json`.
- **Every anchor these primitives write has `author: "student"`.** The AI
  parameterizes the primitive; it never picks a bucket, places an item, or
  authors a perspective (铁律 1 · AI 克制; RL-4).
- **铁律 2:** no scores, streaks, badges, or completion celebration. Remaining
  work is reported plainly ("还差 N 行").
- **Additive only:** every new field on `cards.Params` / `CompletionPredicate`
  is optional so the 33 legacy cards parse unchanged.
- **Test gates.** Go: `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0
  go test -p 1 ./...` from `apps/api`, **FULL packages, never `-run` subsets**.
  Web: `npm test` from `apps/web`. Contracts: `npm test` from
  `packages/contracts` (tests live in `test/`, not `src/`).
- **Icons are inline SVG.** Never import `lucide-react`.
- **NEVER `git add` a whole directory.** Stage named files only. The
  pre-existing `M package.json` and the untracked files under `docs/` and the
  repo root are NOT part of this work.

---

### Task 1: `cards.Params` + `CompletionPredicate` config fields

**Files:**
- Modify: `apps/api/internal/cards/loader.go`
- Test: `apps/api/internal/cards/loader_test.go` (or the existing spec-parse test file in that package)

**Interfaces:**
- Consumes: nothing.
- Produces: `cards.Bucket`, `cards.Axis`, `cards.Params.Buckets`,
  `cards.Params.Cols`, `cards.Params.MinItems`, `cards.CompletionPredicate.Min`
  — consumed by Tasks 2, 3, 4.

- [ ] **Step 1: Write the failing test**

In the cards package test file, add a test that unmarshals a spec fixture
carrying the new params and asserts they parse, plus a test that an existing
legacy card (no C2 block) still parses with zero-valued new fields:

```go
func TestSpecParsesSortScaleMatrixParams(t *testing.T) {
	raw := []byte(`{
	  "id": "x", "name": "X",
	  "primitive": "sort",
	  "params": {
	    "buckets": [{"id":"事实","label":"可查证的事实","hint":"能被独立核查的陈述"}],
	    "cols": [{"id":"position","label":"立场主张","q":"这个视角主张什么？"}],
	    "min_items": 3
	  },
	  "completion": [{"kind":"items_bucketed","tags":["事实"],"min":3}]
	}`)
	var s Spec
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s.Params.Buckets) != 1 || s.Params.Buckets[0].ID != "事实" ||
		s.Params.Buckets[0].Label != "可查证的事实" || s.Params.Buckets[0].Hint != "能被独立核查的陈述" {
		t.Fatalf("buckets: %+v", s.Params.Buckets)
	}
	if len(s.Params.Cols) != 1 || s.Params.Cols[0].ID != "position" ||
		s.Params.Cols[0].Label != "立场主张" || s.Params.Cols[0].Q != "这个视角主张什么？" {
		t.Fatalf("cols: %+v", s.Params.Cols)
	}
	if s.Params.MinItems != 3 {
		t.Fatalf("min_items: %d", s.Params.MinItems)
	}
	if len(s.Completion) != 1 || s.Completion[0].Min != 3 {
		t.Fatalf("completion min: %+v", s.Completion)
	}
}

func TestLegacyCardStillParsesWithZeroValuedNewParams(t *testing.T) {
	s, ok := ByID("steelman")
	if !ok {
		t.Fatal("steelman spec missing")
	}
	if len(s.Params.Buckets) != 0 || len(s.Params.Cols) != 0 || s.Params.MinItems != 0 {
		t.Fatalf("legacy card picked up C2 params: %+v", s.Params)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/cards/`
Expected: FAIL — `s.Params.Buckets` undefined (compile error).

- [ ] **Step 3: Add the fields**

In `apps/api/internal/cards/loader.go`, inside `Params`, after `Slots`:

```go
	// Buckets is the sort/scale primitive's target vocabulary. For a sort card
	// the order is presentational; for a scale card it is the axis order and
	// is meaningful (left-to-right = the continuum). Empty for every
	// non-sort/scale card.
	Buckets []Bucket `json:"buckets"`

	// Cols is the matrix primitive's FIXED column axis. Matrix rows are
	// student-authored (identifying whose view is missing is the thinking),
	// so there is deliberately no Rows counterpart. Empty for non-matrix cards.
	Cols []Axis `json:"cols"`

	// MinItems is the minimum number of bucketed items (sort/scale) or
	// complete rows (matrix) the card requires. 0 means no minimum.
	MinItems int `json:"min_items"`
```

and, after the `Slot` type:

```go
// Bucket is one target of a sort/scale primitive: ID is the value persisted
// as Anchor.Dimension, Label is the verbatim design label, Hint is the short
// disambiguating gloss shown under the label.
type Bucket struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Hint  string `json:"hint"`
}

// Axis is one column of a matrix primitive: ID is the value persisted as
// Anchor.Dimension, Label is the column header, Q is the guiding question
// shown with the header.
type Axis struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Q     string `json:"q"`
}
```

and, inside `CompletionPredicate`:

```go
	// Min is the threshold for count-based predicates (items_bucketed). 0 for
	// every other predicate kind.
	Min int `json:"min"`
```

- [ ] **Step 4: Run tests to verify they pass**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/cards/`
Expected: PASS (full package).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/cards/loader.go apps/api/internal/cards/loader_test.go
git commit -m "feat(n3a): cards.Params gains buckets/cols/min_items + predicate min"
```

---

### Task 2: `items_bucketed` + `matrix_complete` completion predicates

**Files:**
- Modify: `apps/api/internal/agent/card_completion.go`
- Test: `apps/api/internal/agent/card_completion_test.go`

**Interfaces:**
- Consumes: `cards.Params.Buckets/Cols/MinItems`, `cards.CompletionPredicate.Min` (Task 1).
- Produces: two new `EvaluateCompletion` predicate kinds — consumed by the card
  JSON in Task 4.

**Semantics (binding):**
- `items_bucketed` serves BOTH `sort` and `scale`. It counts anchors whose
  `Dimension` is in `pred.Tags` AND whose `Answer` is non-empty (after trim).
  Complete when `count >= pred.Min`. Anchors whose dimension is NOT in the
  vocabulary are **ignored, not rejected** — that is exactly how
  `certainty-spectrum`'s separate `rewrite` anchor coexists with its five
  stops. On failure `missing` gets the single stable token `"items"` (no
  individual tag is "the" missing one).
- `matrix_complete` groups anchors by `Quote` (the row label). A row counts
  only when EVERY column in `spec.Params.Cols` has an anchor for that row with
  a non-empty `Answer`. Complete when the count of complete rows
  `>= spec.Params.MinItems`. `missing` gets the first incomplete row's quote
  (declaration order = first-seen anchor order); when there are simply too few
  rows it gets `"rows"`.

- [ ] **Step 1: Write the failing tests**

```go
func TestItemsBucketedCountsOnlyInVocabularyAnsweredAnchors(t *testing.T) {
	spec := cards.Spec{Completion: []cards.CompletionPredicate{
		{Kind: "items_bucketed", Tags: []string{"事实", "观点"}, Min: 2},
	}}
	anchors := []Anchor{
		{Dimension: "事实", Answer: "可以去核查"},
		{Dimension: "观点", Answer: " "},          // blank answer — not counted
		{Dimension: "rewrite", Answer: "改写句"},   // out of vocabulary — ignored
	}
	complete, missing := EvaluateCompletion(spec, anchors)
	if complete {
		t.Fatal("expected incomplete: only 1 of 2 items bucketed")
	}
	if len(missing) != 1 || missing[0] != "items" {
		t.Fatalf("missing = %v, want [items]", missing)
	}

	anchors = append(anchors, Anchor{Dimension: "观点", Answer: "需要给理由"})
	if complete, _ := EvaluateCompletion(spec, anchors); !complete {
		t.Fatal("expected complete at min")
	}
}

func TestItemsBucketedCoexistsWithFieldWrittenBy(t *testing.T) {
	spec := cards.Spec{Completion: []cards.CompletionPredicate{
		{Kind: "items_bucketed", Tags: []string{"强证据"}, Min: 1},
		{Kind: "field_written_by", Field: "rewrite", Author: "student"},
	}}
	placed := []Anchor{{Dimension: "强证据", Answer: "多个独立来源"}}
	if complete, missing := EvaluateCompletion(spec, placed); complete || len(missing) != 1 || missing[0] != "rewrite" {
		t.Fatalf("want incomplete missing [rewrite], got complete=%v missing=%v", complete, missing)
	}
	withRewrite := append(placed, Anchor{Dimension: "rewrite", Author: "student", Answer: "中国很可能……"})
	if complete, _ := EvaluateCompletion(spec, withRewrite); !complete {
		t.Fatal("expected complete once rewrite is written")
	}
}

func TestMatrixCompleteRequiresEveryCellOfEnoughRows(t *testing.T) {
	spec := cards.Spec{
		Params: cards.Params{
			Cols:     []cards.Axis{{ID: "position"}, {ID: "grounds"}, {ID: "blind_spot"}},
			MinItems: 2,
		},
		Completion: []cards.CompletionPredicate{{Kind: "matrix_complete"}},
	}
	// One complete row, one row missing blind_spot.
	anchors := []Anchor{
		{Quote: "政府", Dimension: "position", Answer: "治理有决心"},
		{Quote: "政府", Dimension: "grounds", Answer: "植树与限排政策"},
		{Quote: "政府", Dimension: "blind_spot", Answer: "回避了排放总量"},
		{Quote: "环保组织", Dimension: "position", Answer: "进展不足"},
		{Quote: "环保组织", Dimension: "grounds", Answer: "碳排放全球第一"},
	}
	complete, missing := EvaluateCompletion(spec, anchors)
	if complete {
		t.Fatal("expected incomplete: 环保组织 has no blind_spot cell")
	}
	if len(missing) != 1 || missing[0] != "环保组织" {
		t.Fatalf("missing = %v, want [环保组织]", missing)
	}

	anchors = append(anchors, Anchor{Quote: "环保组织", Dimension: "blind_spot", Answer: "低估了转型速度"})
	if complete, _ := EvaluateCompletion(spec, anchors); !complete {
		t.Fatal("expected complete once every cell is filled")
	}
}

func TestMatrixCompleteReportsRowsWhenTooFewRows(t *testing.T) {
	spec := cards.Spec{
		Params:     cards.Params{Cols: []cards.Axis{{ID: "position"}}, MinItems: 2},
		Completion: []cards.CompletionPredicate{{Kind: "matrix_complete"}},
	}
	anchors := []Anchor{{Quote: "政府", Dimension: "position", Answer: "治理有决心"}}
	complete, missing := EvaluateCompletion(spec, anchors)
	if complete || len(missing) != 1 || missing[0] != "rows" {
		t.Fatalf("want incomplete missing [rows], got complete=%v missing=%v", complete, missing)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/agent/ -run TestItemsBucketed`
Expected: FAIL — predicate kinds unhandled, so `EvaluateCompletion` reports complete.

- [ ] **Step 3: Implement**

In `EvaluateCompletion`'s switch, after the `graph_slots_complete` case:

```go
		case "items_bucketed":
			if bucketedCount(anchors, pred.Tags) < pred.Min {
				missing = append(missing, "items")
			}
		case "matrix_complete":
			if row, ok := firstIncompleteMatrixRow(spec, anchors); ok {
				missing = append(missing, row)
			}
```

and, at the end of the file:

```go
// bucketedCount counts anchors the student actually placed: dimension in the
// card's bucket/stop vocabulary AND a non-empty reason. Anchors outside the
// vocabulary are IGNORED, not rejected — certainty-spectrum's `rewrite` anchor
// rides the same anchor list as its five spectrum stops and must not make the
// placement predicate unsatisfiable.
func bucketedCount(anchors []Anchor, vocab []string) int {
	in := make(map[string]bool, len(vocab))
	for _, t := range vocab {
		in[t] = true
	}
	n := 0
	for _, a := range anchors {
		if in[a.Dimension] && strings.TrimSpace(a.Answer) != "" {
			n++
		}
	}
	return n
}

// firstIncompleteMatrixRow reports why a matrix card is not done yet. A row is
// keyed by Anchor.Quote (the student's perspective label — a matrix's rows are
// student-authored, so the row's identity IS what she wrote) and counts only
// when every column in spec.Params.Cols has a non-empty answer for it.
// Returns ("<row quote>", true) for the first incomplete row in first-seen
// order, ("rows", true) when there are simply fewer complete rows than
// MinItems, and ("", false) when the card is done.
func firstIncompleteMatrixRow(spec cards.Spec, anchors []Anchor) (string, bool) {
	var order []string
	filled := map[string]map[string]bool{}
	for _, a := range anchors {
		row := a.Quote
		if row == "" || strings.TrimSpace(a.Answer) == "" {
			continue
		}
		if _, seen := filled[row]; !seen {
			filled[row] = map[string]bool{}
			order = append(order, row)
		}
		filled[row][a.Dimension] = true
	}
	completeRows := 0
	for _, row := range order {
		done := true
		for _, col := range spec.Params.Cols {
			if !filled[row][col.ID] {
				done = false
				break
			}
		}
		if done {
			completeRows++
			continue
		}
		// An incomplete row is the most actionable thing to name — but only
		// once we already have enough rows started; otherwise "rows" is the
		// honest answer (see below).
		if len(order) >= spec.Params.MinItems {
			return row, true
		}
	}
	if completeRows < spec.Params.MinItems {
		return "rows", true
	}
	return "", false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/agent/`
Expected: PASS — the FULL agent package, not just the new tests.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/card_completion.go apps/api/internal/agent/card_completion_test.go
git commit -m "feat(n3a): items_bucketed + matrix_complete completion predicates"
```

---

### Task 3: `perspectives` graph effect + `anchorSteps` quote fallback

**Files:**
- Modify: `apps/api/internal/agent/card_effects.go`
- Modify: `apps/api/internal/agent/refeed.go`
- Test: `apps/api/internal/agent/card_effects_test.go`, `apps/api/internal/agent/refeed_test.go`

**Interfaces:**
- Consumes: `cards.Params.Cols/MinItems` (Task 1), the same row rule as
  `firstIncompleteMatrixRow` (Task 2).
- Produces: the `perspectives` graph-effect kind — referenced by
  `perspective-matrix.json` in Task 4.

- [ ] **Step 1: Write the failing tests**

```go
func TestGraphEffectsPerspectivesMintsOneNodePerCompleteRow(t *testing.T) {
	spec := cards.Spec{
		Params:       cards.Params{Cols: []cards.Axis{{ID: "position"}, {ID: "grounds"}}},
		GraphEffects: []cards.GraphEffect{{Kind: "perspectives"}},
	}
	anchors := []Anchor{
		{Quote: "政府", Dimension: "position", Answer: "治理有决心"},
		{Quote: "政府", Dimension: "grounds", Answer: "植树与限排政策"},
		{Quote: "环保组织", Dimension: "position", Answer: "进展不足"}, // incomplete → skipped
	}
	nodes, edges := GraphEffects(spec, "", anchors)
	if len(edges) != 0 {
		t.Fatalf("perspectives mints no edges, got %d", len(edges))
	}
	if len(nodes) != 1 {
		t.Fatalf("want 1 node (only the complete row), got %d", len(nodes))
	}
	n := nodes[0]
	if n.Type != "perspective" || n.Author != "student" {
		t.Fatalf("node type/author = %s/%s", n.Type, n.Author)
	}
	if n.Body["text"] != "政府" {
		t.Fatalf("body text = %v, want 政府", n.Body["text"])
	}
	cells, ok := n.Body["cells"].(map[string]string)
	if !ok || cells["position"] != "治理有决心" || cells["grounds"] != "植树与限排政策" {
		t.Fatalf("body cells = %v", n.Body["cells"])
	}
}

func TestAnchorStepsLabelsFallBackToQuoteBeforeDimension(t *testing.T) {
	inst := CardInstance{Anchors: []Anchor{
		// no Question — a sort/scale/matrix anchor. The sentence (quote) is
		// the only useful label; the dimension is already the step title.
		{Quote: "中国碳排放全球第一", Dimension: "事实", Answer: "可以去核查"},
		// no Question and no Quote — falls all the way back to the dimension.
		{Dimension: "观点", Answer: "需要给理由"},
	}}
	steps := anchorSteps(inst)
	if len(steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(steps))
	}
	if steps[0].Title != "事实" || steps[0].Answers[0].Label != "中国碳排放全球第一" {
		t.Fatalf("step0 = %+v", steps[0])
	}
	if steps[1].Answers[0].Label != "观点" {
		t.Fatalf("step1 label = %q, want 观点", steps[1].Answers[0].Label)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/agent/ -run 'TestGraphEffectsPerspectives|TestAnchorStepsLabels'`
Expected: FAIL — no nodes minted; label falls back to dimension not quote.

- [ ] **Step 3: Implement**

In `card_effects.go`, add a case to the `GraphEffects` switch, after `"toulmin"`:

```go
		case "perspectives":
			// One node per COMPLETE row of the matrix (same row rule as
			// matrix_complete: keyed by the student's own perspective label,
			// every column filled). A half-filled row is a real state we keep
			// on the card_instance, but it is not yet a perspective the
			// workspace graph should assert. No edges — perspectives are
			// free-standing project nodes; nothing links them today (the S2
			// perspective view that reads them is N3d).
			for _, row := range completeMatrixRows(spec, anchors) {
				nodes = append(nodes, MintNode{
					Type:   "perspective",
					Author: "student",
					Body:   map[string]any{"text": row.Label, "cells": row.Cells},
				})
			}
```

and, at the end of the file:

```go
// matrixRow is one complete student-authored matrix row.
type matrixRow struct {
	Label string
	Cells map[string]string
}

// completeMatrixRows returns the matrix rows every column of which the student
// filled, in first-seen order. Shares its row rule with
// firstIncompleteMatrixRow (card_completion.go): a row is keyed by
// Anchor.Quote and a cell is Anchor.Dimension -> Anchor.Answer.
func completeMatrixRows(spec cards.Spec, anchors []Anchor) []matrixRow {
	var order []string
	cells := map[string]map[string]string{}
	for _, a := range anchors {
		if a.Quote == "" || strings.TrimSpace(a.Answer) == "" {
			continue
		}
		if _, seen := cells[a.Quote]; !seen {
			cells[a.Quote] = map[string]string{}
			order = append(order, a.Quote)
		}
		cells[a.Quote][a.Dimension] = a.Answer
	}
	var out []matrixRow
	for _, label := range order {
		done := true
		for _, col := range spec.Params.Cols {
			if cells[label][col.ID] == "" {
				done = false
				break
			}
		}
		if done {
			out = append(out, matrixRow{Label: label, Cells: cells[label]})
		}
	}
	return out
}
```

(Add the `strings` import to `card_effects.go` if it is not already there.)

In `refeed.go`, inside `anchorSteps`, replace:

```go
		label := a.Question
		if label == "" {
			label = dim
		}
```

with:

```go
		// question → quote → dimension. Annotate/compare anchors carry an
		// AI-authored question; sort/scale/matrix anchors carry none, and for
		// them the quote (the sentence sorted / the item placed / the
		// perspective row) is the only thing that makes the refed answer
		// legible — the dimension is already the step title.
		label := a.Question
		if label == "" {
			label = a.Quote
		}
		if label == "" {
			label = dim
		}
```

- [ ] **Step 4: Run tests to verify they pass**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/agent/`
Expected: PASS — FULL package (the refeed change touches existing refeed tests).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/card_effects.go apps/api/internal/agent/card_effects_test.go apps/api/internal/agent/refeed.go apps/api/internal/agent/refeed_test.go
git commit -m "feat(n3a): perspectives graph effect + refeed quote label fallback"
```

---

### Task 4: The three card JSONs (2 upgrades + 1 new)

**Files:**
- Modify: `packages/contracts/cards/fact-opinion-value.json`
- Modify: `packages/contracts/cards/certainty-spectrum.json`
- Create: `packages/contracts/cards/perspective-matrix.json`
- Regenerate: `apps/api/internal/cards/specs/*.json` via `cd apps/api && make sync-cards`
- Test: the existing card-catalog / registry tests in `packages/contracts/test/` and `apps/api/internal/cards/`

**Interfaces:**
- Consumes: Task 1's params fields, Task 2's predicate kinds, Task 3's effect kind.
- Produces: `perspective-matrix` card id; the three `primitive` bindings the
  frontend forks on in Tasks 9–10.

**Before writing:** find how cards are registered in `packages/contracts`
(there is an index/registry that imports every card JSON — a new card must be
added there too) and how the Go side discovers them (`//go:embed specs/*.json`
— discovery is automatic once the file is mirrored). Check whether any test
asserts an exact card count; if so, update it.

- [ ] **Step 1: Write the failing test**

In the cards package (`apps/api/internal/cards`), assert the three bindings
exist and are internally consistent:

```go
func TestNewPrimitiveCardsAreWiredConsistently(t *testing.T) {
	for _, tc := range []struct {
		id, primitive, predicate string
	}{
		{"fact-opinion-value", "sort", "items_bucketed"},
		{"certainty-spectrum", "scale", "items_bucketed"},
		{"perspective-matrix", "matrix", "matrix_complete"},
	} {
		s, ok := ByID(tc.id)
		if !ok {
			t.Fatalf("%s: spec missing", tc.id)
		}
		if s.Primitive != tc.primitive {
			t.Fatalf("%s: primitive = %q, want %q", tc.id, s.Primitive, tc.primitive)
		}
		found := false
		for _, p := range s.Completion {
			if p.Kind == tc.predicate {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: no %s completion predicate", tc.id, tc.predicate)
		}
	}

	// sort/scale: every completion tag must be a declared bucket, else the
	// card can never complete.
	for _, id := range []string{"fact-opinion-value", "certainty-spectrum"} {
		s, _ := ByID(id)
		declared := map[string]bool{}
		for _, b := range s.Params.Buckets {
			declared[b.ID] = true
		}
		if len(declared) == 0 {
			t.Fatalf("%s: no buckets", id)
		}
		for _, p := range s.Completion {
			if p.Kind != "items_bucketed" {
				continue
			}
			if p.Min <= 0 {
				t.Fatalf("%s: items_bucketed min = %d", id, p.Min)
			}
			for _, tag := range p.Tags {
				if !declared[tag] {
					t.Fatalf("%s: completion tag %q is not a declared bucket", id, tag)
				}
			}
		}
	}

	// matrix: cols + a positive MinItems + the perspectives effect.
	m, _ := ByID("perspective-matrix")
	if len(m.Params.Cols) != 3 || m.Params.MinItems < 2 {
		t.Fatalf("perspective-matrix params: %+v", m.Params)
	}
	if len(m.GraphEffects) != 1 || m.GraphEffects[0].Kind != "perspectives" {
		t.Fatalf("perspective-matrix graph_effects: %+v", m.GraphEffects)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/cards/`
Expected: FAIL — `perspective-matrix: spec missing`.

- [ ] **Step 3: Author the JSON**

**`packages/contracts/cards/fact-opinion-value.json`** — keep every existing
key and the whole `steps` array unchanged; insert the C2 block after
`"rubric_tags"`:

```json
  "primitive": "sort",
  "target_type": "project",
  "params": {
    "buckets": [
      { "id": "事实",     "label": "可查证的事实", "hint": "能被独立核查的陈述" },
      { "id": "观点",     "label": "需论证的观点", "hint": "成立需要给出理由" },
      { "id": "价值判断", "label": "藏价值的判断", "hint": "带「应该/好坏」，藏着价值排序" }
    ],
    "min_items": 3,
    "item_prompt": "把一句话贴进来或写下来",
    "reason_prompt": "检验/理由（它能被查证吗？需要理由吗？藏了「应该」吗？）"
  },
  "completion": [
    { "kind": "items_bucketed", "tags": ["事实", "观点", "价值判断"], "min": 3 }
  ],
  "consolidation": "reveal_framework_after_completion",
  "intrusiveness_cap": "I2",
```

**`packages/contracts/cards/certainty-spectrum.json`** — same treatment; the
five stops are the card's own existing `spectrum` stops, verbatim:

```json
  "primitive": "scale",
  "target_type": "project",
  "params": {
    "buckets": [
      { "id": "个人猜测", "label": "个人猜测", "hint": "只有直觉，没有证据" },
      { "id": "有据推断", "label": "有据推断", "hint": "有证据，但推理仍可争议" },
      { "id": "强证据",   "label": "强证据",   "hint": "多个独立来源支撑" },
      { "id": "科学共识", "label": "科学共识", "hint": "领域内已达成共识" },
      { "id": "逻辑必然", "label": "逻辑必然", "hint": "由定义或推理必然为真" }
    ],
    "min_items": 1,
    "item_prompt": "你的结论是什么？",
    "reason_prompt": "为什么是这个位置？支撑它的证据类型是什么？",
    "rewrite_prompt": "用与该位置相称的语气词重写结论句（可能 / 大概 / 很可能 / 几乎确定 / 必然）"
  },
  "completion": [
    { "kind": "items_bucketed", "tags": ["个人猜测", "有据推断", "强证据", "科学共识", "逻辑必然"], "min": 1 },
    { "kind": "field_written_by", "field": "rewrite", "author": "student" }
  ],
  "consolidation": "reveal_framework_after_completion",
  "intrusiveness_cap": "I2",
```

**`packages/contracts/cards/perspective-matrix.json`** — new file. Match the
shape of a full existing card (id / category / name / name_en / purpose /
trigger_condition / trigger_keywords / priority / disclosure_tier / age_band /
interaction_type / rubric_dims / related / body_status / rubric_tags / steps),
plus the C2 block. Real content, no lorem ipsum:

```json
{
  "id": "perspective-matrix",
  "category": "溯源与多视角",
  "name": "视角对照矩阵",
  "name_en": "Perspective Matrix",
  "purpose": "把「不同视角」从一句口号变成一张能填的表：每个视角主张什么、凭什么、又看不见什么",
  "trigger_condition": "学生只从一个视角论证，或把「有人认为」当成了对立面却说不出对方的依据",
  "trigger_keywords": ["不同视角", "有人认为", "另一方面", "多角度", "利益相关"],
  "priority": "P0",
  "disclosure_tier": "tier-1",
  "age_band": ["MYP", "DP"],
  "interaction_type": "画布导图卡",
  "rubric_dims": ["D4", "D7"],
  "related": ["belief-spectrum", "steelman", "toulmin"],
  "body_status": "full",
  "rubric_tags": ["D4", "D7"],
  "primitive": "matrix",
  "target_type": "project",
  "params": {
    "cols": [
      { "id": "position",   "label": "立场主张", "q": "这个视角主张什么？用一句话说清。" },
      { "id": "grounds",    "label": "依据",     "q": "它凭什么这么主张？它最强的依据是什么？" },
      { "id": "blind_spot", "label": "盲区",     "q": "这个视角看不见什么？它绕开了哪个事实？" }
    ],
    "min_items": 2,
    "row_prompt": "这是谁的视角？（例：地方政府 / 环保组织 / 受影响居民 / 能源企业）"
  },
  "completion": [
    { "kind": "matrix_complete" }
  ],
  "graph_effects": [
    { "kind": "perspectives" }
  ],
  "consolidation": "reveal_framework_after_completion",
  "intrusiveness_cap": "I3",
  "steps": [
    {
      "key": "rows",
      "title": "列出视角",
      "disclose": "always",
      "methodology": {
        "why": "「多视角」最常见的失败不是没写，而是只写了**两个都属于自己那一边**的视角——真正的对立面被悄悄跳过了。先把「这件事上谁会不同意」摊开列出来，分析才有对象。",
        "how": "沿着**利益**和**代价**去找：谁受益、谁承担代价、谁负责、谁没有发言权。每个都是一行。",
        "when": "当你的论证从头到尾只有一个声音时。"
      },
      "fields": [
        { "type": "repeatable_group", "key": "perspectives", "label": "每行一个视角", "item_fields": [
          { "type": "text",     "key": "who",        "label": "这是谁的视角？" },
          { "type": "textarea", "key": "position",   "label": "它主张什么？" },
          { "type": "textarea", "key": "grounds",    "label": "它的依据是什么？" },
          { "type": "textarea", "key": "blind_spot", "label": "它看不见什么？" }
        ] }
      ]
    }
  ]
}
```

Then register the new card in the `packages/contracts` card index and run
`cd apps/api && make sync-cards`.

- [ ] **Step 4: Run the tests**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/cards/ ./internal/agent/
cd ../../packages/contracts && npm test
```
Expected: PASS both. If a card-count assertion fails, update it to 34.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/cards/fact-opinion-value.json packages/contracts/cards/certainty-spectrum.json packages/contracts/cards/perspective-matrix.json apps/api/internal/cards/specs/ apps/api/internal/cards/loader_test.go
git commit -m "feat(n3a): bind sort/scale/matrix to fact-opinion-value, certainty-spectrum, perspective-matrix"
```

(Staging `apps/api/internal/cards/specs/` is the ONE directory-level `git add`
this plan permits — it is entirely generated by `make sync-cards`. Verify with
`git status --short` that only the three mirrored card files changed.)

---

### Task 5: Zod state schemas for the three primitives

**Files:**
- Modify: `packages/contracts/src/interactionPrimitive.ts`
- Test: `packages/contracts/test/interactionPrimitive.test.ts` (create if absent)

**Interfaces:**
- Produces: `SortItem`/`SortState`, `ScaleItem`/`ScaleState`,
  `MatrixRow`/`MatrixState` (types + schemas), consumed by Tasks 6–8.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it } from "vitest";
import { SortState, ScaleState, MatrixState } from "../src/interactionPrimitive";

describe("sort/scale/matrix states", () => {
  it("parses a sort state", () => {
    const s = SortState.parse({
      items: [{ id: "s1", text: "中国碳排放全球第一", bucket: "事实", reason: "可以去核查", author: "student" }],
    });
    expect(s.items[0].bucket).toBe("事实");
  });

  it("parses a scale state with a rewrite", () => {
    const s = ScaleState.parse({
      items: [{ id: "i1", text: "中国让地球更可持续", stop: "有据推断", reason: "证据只覆盖绿化", author: "student" }],
      rewrite: "中国很可能在绿化上做出了最大贡献",
    });
    expect(s.rewrite).toContain("很可能");
  });

  it("parses a matrix state", () => {
    const s = MatrixState.parse({
      rows: [{ id: "r1", label: "环保组织", cells: { position: "进展不足" }, author: "student" }],
    });
    expect(s.rows[0].cells.position).toBe("进展不足");
  });

  it("rejects a row with a non-string cell", () => {
    expect(() => MatrixState.parse({ rows: [{ id: "r1", label: "x", cells: { position: 1 }, author: "student" }] })).toThrow();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run from `packages/contracts`: `npm test`
Expected: FAIL — `SortState` is not exported.

- [ ] **Step 3: Implement**

Append to `packages/contracts/src/interactionPrimitive.ts` (mirroring the
GraphState block's style, with a comment explaining the Anchor mapping):

```ts
// sort (C1): statements dropped into a fixed vocabulary of buckets, each with
// the student's own test/reason. Persists 1:1 into Anchor[]:
// quote = text, dimension = bucket, answer = reason, author = student.
export const SortItem = z.object({
  id: z.string().min(1),
  text: z.string(),
  bucket: z.string(),
  reason: z.string(),
  author: Author,
});
export type SortItem = z.infer<typeof SortItem>;
export const SortState = z.object({ items: z.array(SortItem) });
export type SortState = z.infer<typeof SortState>;

// scale (C1): items placed on an ORDERED axis of named stops. Same Anchor
// mapping as sort (dimension = stop); `rewrite` is a separate student-written
// field carried on its own anchor (dimension "rewrite") so it can ride the
// existing field_written_by completion predicate.
export const ScaleItem = z.object({
  id: z.string().min(1),
  text: z.string(),
  stop: z.string(),
  reason: z.string(),
  author: Author,
});
export type ScaleItem = z.infer<typeof ScaleItem>;
export const ScaleState = z.object({ items: z.array(ScaleItem), rewrite: z.string() });
export type ScaleState = z.infer<typeof ScaleState>;

// matrix (C1): student-authored rows x fixed columns. `id` is a client-only
// React key — the row's PERSISTED identity is `label` (Anchor.quote), which is
// what completion groups on and what the perspectives graph effect names the
// node. cells maps column id -> the student's cell text (Anchor.dimension ->
// Anchor.answer).
export const MatrixRow = z.object({
  id: z.string().min(1),
  label: z.string(),
  cells: z.record(z.string()),
  author: Author,
});
export type MatrixRow = z.infer<typeof MatrixRow>;
export const MatrixState = z.object({ rows: z.array(MatrixRow) });
export type MatrixState = z.infer<typeof MatrixState>;
```

Confirm these are exported from the package barrel (`packages/contracts/src/index.ts`)
the same way `GraphState` is.

- [ ] **Step 4: Run tests to verify they pass**

Run from `packages/contracts`: `npm test` (FULL suite) and `npx tsc --noEmit`.
Expected: PASS, tsc clean.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/interactionPrimitive.ts packages/contracts/src/index.ts packages/contracts/test/interactionPrimitive.test.ts
git commit -m "feat(n3a): contracts — SortState/ScaleState/MatrixState"
```

---

### Task 6: `primitives/sort/` — serializer + component

**Files:**
- Create: `apps/web/src/primitives/sort/serialize.ts`
- Create: `apps/web/src/primitives/sort/Sort.tsx`
- Create: `apps/web/src/primitives/sort/index.ts`
- Test: `apps/web/src/primitives/sort/serialize.test.ts`, `apps/web/src/primitives/sort/Sort.test.tsx`

**Interfaces:**
- Consumes: `SortState`, `SortItem`, `Anchor` (Task 5 / contracts).
- Produces:
  - `export type Bucket = { id: string; label: string; hint: string }`
  - `export function sortStateToAnchors(state: SortState): Anchor[]`
  - `export function anchorsToSortState(anchors: Anchor[], buckets: Bucket[]): SortState`
  - `export function Sort(props: { buckets: Bucket[]; state: SortState; minItems: number; itemPrompt: string; reasonPrompt: string; onChange: (s: SortState) => void; onLock: () => void; onSkip: () => void }): JSX.Element`

**Read first:** `apps/web/src/primitives/graph/serialize.ts` and
`apps/web/src/primitives/graph/Graph.tsx` — this module mirrors their shape,
naming, module-local `nid()` counter, and comment style exactly.

**Serializer semantics (binding):**
- `sortStateToAnchors`: one anchor per item — `{id: nid(), material_id: "",
  block_id: "", start: 0, end: 0, quote: item.text, dimension: item.bucket,
  author: "student", question: "", answer: item.reason}`. Items with a blank
  `text` are dropped (an empty row the student never filled is not data).
- `anchorsToSortState`: the inverse. An anchor whose `dimension` is not a
  declared bucket id is **dropped** (defensive against config drift, mirroring
  how `anchorsToGraphState` iterates slots rather than raw anchors). Order is
  the anchor order.
- Round-trip is exact for well-formed state.

**Component:** an add-row list. Each row: the sentence (textarea,
`itemPrompt` placeholder), a chip row of the buckets (label + hint, single
select, no default selection), the reason (textarea, `reasonPrompt`
placeholder), and a remove control (inline SVG ×). Below the list: `＋ 添加一句`,
a plain progress line `已归类 N 句 · 还差 M 句` (no celebration, no bar — 铁律 2),
and 「完成并钉到过程树」/「跳过这张卡」 buttons mirroring `Graph.tsx`'s footer.
The lock button is disabled until `N >= minItems` and every filled row has a
bucket and a non-empty reason.

- [ ] **Step 1: Write the failing serializer test**

Cover: state→anchors field mapping (quote/dimension/answer/author) · blank-text
rows dropped · anchors→state drops out-of-vocabulary dimensions · exact
round-trip on a two-item state.

- [ ] **Step 2: Run to verify it fails**

Run from `apps/web`: `npm test -- src/primitives/sort`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement `serialize.ts` + `index.ts`**

- [ ] **Step 4: Run the serializer test — PASS**

- [ ] **Step 5: Write the failing component test**

Cover: renders one row per item and every bucket chip · picking a bucket calls
`onChange` with that item's `bucket` set · `＋ 添加一句` appends an empty row ·
the lock button is disabled below `minItems` and enabled at it · `onLock` and
`onSkip` fire. Use `@testing-library/react` per the existing
`primitives/graph/Graph.test.tsx`.

- [ ] **Step 6: Implement `Sort.tsx`, run the tests — PASS**

- [ ] **Step 7: Run the FULL web suite + typecheck**

Run from `apps/web`: `npm test` then `npx tsc --noEmit`.
Expected: PASS, clean.

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/primitives/sort/
git commit -m "feat(n3a): sort primitive — serializer + component"
```

---

### Task 7: `primitives/scale/` — serializer + component

**Files:**
- Create: `apps/web/src/primitives/scale/serialize.ts`, `Scale.tsx`, `index.ts`
- Test: `apps/web/src/primitives/scale/serialize.test.ts`, `Scale.test.tsx`

**Interfaces:**
- Consumes: `ScaleState`, `ScaleItem`, `Anchor`; reuses the `Bucket` type shape
  (re-declare locally — do NOT import across primitive modules; `graph` and
  `compare` are likewise independent).
- Produces:
  - `export function scaleStateToAnchors(state: ScaleState): Anchor[]`
  - `export function anchorsToScaleState(anchors: Anchor[], stops: Bucket[]): ScaleState`
  - `export function Scale(props: { stops: Bucket[]; state: ScaleState; minItems: number; itemPrompt: string; reasonPrompt: string; rewritePrompt: string; onChange: (s: ScaleState) => void; onLock: () => void; onSkip: () => void }): JSX.Element`

**Serializer semantics (binding):**
- `scaleStateToAnchors`: one anchor per item (`quote` = text, `dimension` =
  stop, `answer` = reason, `author` = "student"), **plus**, when
  `state.rewrite` is non-blank, ONE extra anchor `{dimension: "rewrite",
  author: "student", answer: state.rewrite, quote: ""}`. That extra anchor is
  what `field_written_by{field:"rewrite"}` reads server-side.
- `anchorsToScaleState`: items come from anchors whose `dimension` is a
  declared stop; the anchor with `dimension === "rewrite"` populates
  `state.rewrite` (never an item). Blank-text items are dropped on serialize.

**Component:** a horizontal axis of the ordered stops (a row of stop labels
with a hairline connecting them, leftmost = first stop), each item rendered as
a chip the student assigns to a stop (click a stop to place the selected item),
plus the item text, its reason, and — below the axis — the `rewritePrompt`
textarea. Same footer/progress/disabled-lock discipline as `Sort`. The lock
button additionally requires a non-blank rewrite when `rewritePrompt` is
non-empty (mirroring the card's second completion predicate so the UI does not
let her lock a card the backend will call incomplete).

- [ ] **Step 1: Write the failing serializer test**

Cover: the rewrite anchor is emitted separately and only when non-blank ·
`anchorsToScaleState` routes the `rewrite` anchor to `state.rewrite` and NOT
into `items` · round-trip exact including rewrite · out-of-vocabulary stops dropped.

- [ ] **Step 2: Run to verify it fails** — `npm test -- src/primitives/scale`

- [ ] **Step 3: Implement `serialize.ts` + `index.ts`**

- [ ] **Step 4: Run the serializer test — PASS**

- [ ] **Step 5: Write the failing component test**

Cover: renders every stop in declaration order · clicking a stop assigns it to
the item and calls `onChange` · the rewrite textarea calls `onChange` · lock
disabled with a blank rewrite, enabled once filled · `onSkip` fires.

- [ ] **Step 6: Implement `Scale.tsx`, run the tests — PASS**

- [ ] **Step 7: Run the FULL web suite + `npx tsc --noEmit`**

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/primitives/scale/
git commit -m "feat(n3a): scale primitive — serializer + component"
```

---

### Task 8: `primitives/matrix/` — serializer + component

**Files:**
- Create: `apps/web/src/primitives/matrix/serialize.ts`, `Matrix.tsx`, `index.ts`
- Test: `apps/web/src/primitives/matrix/serialize.test.ts`, `Matrix.test.tsx`

**Interfaces:**
- Consumes: `MatrixState`, `MatrixRow`, `Anchor`.
- Produces:
  - `export type Col = { id: string; label: string; q: string }`
  - `export function matrixStateToAnchors(state: MatrixState): Anchor[]`
  - `export function anchorsToMatrixState(anchors: Anchor[], cols: Col[]): MatrixState`
  - `export function Matrix(props: { cols: Col[]; state: MatrixState; minItems: number; rowPrompt: string; onChange: (s: MatrixState) => void; onLock: () => void; onSkip: () => void }): JSX.Element`

**Serializer semantics (binding):**
- `matrixStateToAnchors`: one anchor PER CELL — `{quote: row.label, dimension:
  colId, answer: cellText, author: "student", material_id: "", block_id: "",
  start: 0, end: 0, question: ""}`. Rows with a blank `label` are dropped
  entirely; blank cells emit no anchor. Cell order follows `cols` order within
  each row, rows in state order.
- `anchorsToMatrixState`: group by `quote` in first-seen order; each group
  becomes a row `{id: nid(), label: quote, cells: {dimension: answer}, author:
  "student"}`. Anchors whose `dimension` is not a declared column are dropped.
  Rows are rehydrated even when incomplete (a half-filled matrix must survive
  a reload).
- Round-trip is exact modulo the regenerated `id`s — assert on
  `label`/`cells`, not on `id`.

**Component:** a **stacked row-card list**, NOT an HTML table — the CoachRail
column is 388px wide and a 3-column grid is unreadable there. Each row-card:
the perspective name (text input, `rowPrompt` placeholder) as the card header,
then one labeled textarea per column showing `col.label` with `col.q` as the
placeholder, plus a remove control (inline SVG ×). Below: `＋ 添加一个视角`, the
plain progress line `已完成 N 个视角 · 还差 M 个`, and the same footer buttons.
Lock is disabled until at least `minItems` rows have a non-blank label AND
every column filled.

- [ ] **Step 1: Write the failing serializer test**

Cover: cell-per-anchor mapping · blank-label rows dropped · blank cells emit no
anchor · `anchorsToMatrixState` groups by quote in first-seen order and
rehydrates an INCOMPLETE row · out-of-vocabulary dimensions dropped ·
round-trip on label/cells.

- [ ] **Step 2: Run to verify it fails** — `npm test -- src/primitives/matrix`

- [ ] **Step 3: Implement `serialize.ts` + `index.ts`**

- [ ] **Step 4: Run the serializer test — PASS**

- [ ] **Step 5: Write the failing component test**

Cover: renders one card per row and one field per column, with `col.label`
visible and `col.q` as the placeholder · typing a cell calls `onChange` with
`cells[colId]` set · `＋ 添加一个视角` appends a row · lock disabled at 1
complete row when `minItems` is 2, enabled at 2 · `onSkip` fires.

- [ ] **Step 6: Implement `Matrix.tsx`, run the tests — PASS**

- [ ] **Step 7: Run the FULL web suite + `npx tsc --noEmit`**

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/primitives/matrix/
git commit -m "feat(n3a): matrix primitive — serializer + component"
```

---

### Task 9: `StudioSortCard` + `StudioScaleCard` + CoachRail fork

**Files:**
- Create: `apps/web/src/studio/StudioSortCard.tsx`, `StudioScaleCard.tsx`
- Modify: `apps/web/src/studio/CoachRail.tsx`
- Test: `apps/web/src/studio/StudioSortCard.test.tsx`, `StudioScaleCard.test.tsx`, `apps/web/src/studio/CoachRail.test.tsx`

**Interfaces:**
- Consumes: Tasks 6–7's modules; `newEnvelope` from `../cards/envelopeReducer`.
- Produces: two host components rendered by the CoachRail's primitive fork.

**Read first:** `apps/web/src/studio/StudioToulminCard.tsx` — both hosts mirror
it exactly: same props shape (`spec`, `cardInstanceId`, `anchors = []`,
`onSubmit`, `onSkip`), the same lazy `useState` seed from persisted anchors,
the same `seededFor` ref re-seed guard keyed on `cardInstanceId`, and the same
`onSubmit({ ...newEnvelope(spec.id, ""), anchors: <serializer>(state) })` lock.
Config is read from `spec.params` and NEVER hardcoded:

```ts
const p = (spec.params ?? {}) as {
  buckets?: Bucket[]; min_items?: number;
  item_prompt?: string; reason_prompt?: string; rewrite_prompt?: string;
};
```

The header block (工具卡 · category chip + name + purpose line) matches
`StudioToulminCard`'s.

- [ ] **Step 1: Write the failing host tests**

For each host: it seeds working state from persisted anchors (render with a
two-anchor `anchors` prop, assert the item text and bucket are shown) · it
re-seeds when `cardInstanceId` changes (rerender with a new id and empty
anchors, assert the old rows are gone) · locking calls `onSubmit` with an
envelope whose `card_id` is the spec id and whose `anchors` round-trip the
state · skipping calls `onSkip` with the scaffold's `event_trace`.

- [ ] **Step 2: Run to verify they fail** — `npm test -- src/studio/StudioSortCard src/studio/StudioScaleCard`

- [ ] **Step 3: Implement both hosts**

- [ ] **Step 4: Run the host tests — PASS**

- [ ] **Step 5: Write the failing CoachRail fork test**

In `CoachRail.test.tsx`, mirroring its existing annotate/compare fork tests:
an active card with `spec.primitive === "sort"` renders `StudioSortCard`
(assert on a distinctive string, e.g. the bucket labels), and one with
`"scale"` renders `StudioScaleCard`.

- [ ] **Step 6: Add the fork branches**

In `CoachRail.tsx`, insert BEFORE the `card.spec.primitive === "graph"` branch
(order is irrelevant to behavior but keep the new branches grouped with the
other rail-rendered primitives, i.e. after `compare`):

```tsx
          ) : card.spec.primitive === "sort" ? (
            <StudioSortCard
              spec={card.spec}
              cardInstanceId={card.cardInstanceId}
              anchors={card.anchors}
              onSubmit={(env) => onSubmitCard?.(env)}
              onSkip={(eventTrace) => onSkipCard?.(eventTrace)}
            />
          ) : card.spec.primitive === "scale" ? (
            <StudioScaleCard
              spec={card.spec}
              cardInstanceId={card.cardInstanceId}
              anchors={card.anchors}
              onSubmit={(env) => onSubmitCard?.(env)}
              onSkip={(eventTrace) => onSkipCard?.(eventTrace)}
            />
```

with the matching imports at the top.

- [ ] **Step 7: Run the FULL web suite + `npx tsc --noEmit`** — PASS, clean

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/studio/StudioSortCard.tsx apps/web/src/studio/StudioSortCard.test.tsx apps/web/src/studio/StudioScaleCard.tsx apps/web/src/studio/StudioScaleCard.test.tsx apps/web/src/studio/CoachRail.tsx apps/web/src/studio/CoachRail.test.tsx
git commit -m "feat(n3a): StudioSortCard + StudioScaleCard hosts + CoachRail fork"
```

---

### Task 10: `StudioMatrixCard` + CoachRail fork

**Files:**
- Create: `apps/web/src/studio/StudioMatrixCard.tsx`
- Modify: `apps/web/src/studio/CoachRail.tsx`
- Test: `apps/web/src/studio/StudioMatrixCard.test.tsx`, `apps/web/src/studio/CoachRail.test.tsx`

**Interfaces:**
- Consumes: Task 8's module; the same host contract as Task 9.
- Produces: the third fork branch — after this, all six C1 primitives render.

Params read: `{ cols?: Col[]; min_items?: number; row_prompt?: string }`.

- [ ] **Step 1: Write the failing host test**

Same four cases as Task 9's hosts, using a two-row / three-column fixture.
Additionally: an INCOMPLETE persisted row (label + one cell) rehydrates and the
lock stays disabled.

- [ ] **Step 2: Run to verify it fails** — `npm test -- src/studio/StudioMatrixCard`

- [ ] **Step 3: Implement `StudioMatrixCard.tsx`**

- [ ] **Step 4: Run the host test — PASS**

- [ ] **Step 5: Write the failing CoachRail test** — `primitive === "matrix"` renders `StudioMatrixCard`

- [ ] **Step 6: Add the fork branch** (same shape as Task 9's, placed with them)

- [ ] **Step 7: Run the FULL web suite + `npx tsc --noEmit`** — PASS, clean

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/studio/StudioMatrixCard.tsx apps/web/src/studio/StudioMatrixCard.test.tsx apps/web/src/studio/CoachRail.tsx apps/web/src/studio/CoachRail.test.tsx
git commit -m "feat(n3a): StudioMatrixCard host + CoachRail fork — C1 library complete 6/6"
```

---

## Final gate (whole branch, before review)

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...
cd ../../packages/contracts && npm test && npx tsc --noEmit
cd ../../apps/web && npm test && npx tsc --noEmit
```

All three must be green with FULL packages/suites — never `-run` subsets for
the gate. `git status --short` must show no changes outside the files this plan
names (plus the pre-existing `M package.json` and untracked user files, which
are NOT ours).

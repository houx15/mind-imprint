# Slice 4 — Skill format + gate engine + planner + intake — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Author the writing-project S0–S6 journey as a C5 skill (a contract DAG in config) and give the runtime a gate engine, a deterministic planner (route/replan/advance), and intake — proving a project type is a *skill install* with zero new runtime code per skill.

**Architecture:** A new pure-config `skill` Go package (types + `go:embed` loader + DAG validation), mirroring how `cards` works. The runtime logic lives in the existing `agent` package: pure gate predicates + `CheckGate` + `ReconcileGates` + `Route` (no DB), and store-backed `Intake`/`Plan`/`Replan`/`Advance` over the `AgentStore` seam. Storage reuses the Slice-0 `graph_node` `plan`/`gate_state` types and the `imported` author value — **no new migration**. The writing-project skill JSON is single-source in `packages/contracts/skills/`, synced to `apps/api/internal/skills/specs/` by a new `syncskills` tool, exactly like cards.

**Tech Stack:** Go (`net/http` era pkg, `embed`, `encoding/json`, sqlc/pgx, testcontainers), no new deps. Config JSON. No UI, no model call, no transport.

## Global Constraints

- **No new migration.** `graph_node.type` already allows `plan` and `gate_state`; `graph_node.author` already allows `imported`. Write zero DDL.
- **No model call, no UI, no transport.** All logic is pure functions or store-backed orchestration over fixtures + testcontainers, matching Slices 2–3.
- **DEC-3 is structural.** `CheckGate` returns `Status ∈ {empty, partial, machine_clear}` and **never** `solid`. `solid` exists only as `GateReport.Solid`, mirrored from a recorded `gate_state.confirmed_solid`. `Advance` may write `confirmed_solid=true` only after every `student_written`/`human` item is *recorded* solid (it never marks one itself); a machine-only gate advances on `machine_clear`.
- **Machine gate items are a closed typed set** whose *names* live in the `skill` package (`skill.MachineKinds`) and whose *evaluation* lives in `agent`: `node_present{type}` · `node_count_at_least{type,n}` · `no_orphan_evidence` · `no_unsupported_claim` · `no_single_sourced_claim` · `every_source_evaluated`. A new kind is a runtime change (§5.7 escape hatch); authoring a skill only references existing kinds.
- **Provenance = the `imported` author value.** Intake mints `graph_node{author:'imported'}`; imported content never auto-satisfies a `student_written` item.
- **Single-source config.** Never hand-edit `internal/skills/specs/`; edit `packages/contracts/skills/` and run `make sync-skills`.
- **Icons/keys/secrets rules unchanged** (no client secrets; server-only). Not exercised in this slice.
- **Commit message trailer:** end every commit body with `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.
- **Run all Go commands from `apps/api/`.** `go test ./...` for the suite; `-short` skips testcontainers.

---

## File structure

- `apps/api/internal/skills/skill.go` — **new**: `Skill`/`Contract`/`Gate`/`MachineItem` types, `MachineKinds`, `Catalog`/`ByID`/`Load`, `Validate`, `TopoOrder`.
- `apps/api/internal/skills/skill_test.go` — **new**: loader + validation + topo tests.
- `apps/api/internal/skills/specs/*.json` — **new (generated mirror)**: writing-project.json.
- `packages/contracts/skills/writing-project.json` — **new (canonical)**: the S0–S6 skill.
- `apps/api/tools/syncskills/main.go` — **new**: mirror canonical skills → embeddable specs.
- `apps/api/Makefile` — **modify**: add `sync-skills`.
- `apps/api/internal/agent/gate.go` — **new**: machine predicates, `CheckGate`, `GateReport`, `RecordedGate`, `ReconcileGates`.
- `apps/api/internal/agent/gate_test.go` — **new**: predicate + CheckGate + reconcile + DEC-3 tests.
- `apps/api/internal/agent/planner.go` — **new**: `Route`, `Intake`, `Plan`, `Replan`, `Advance`, `IntakeCandidate`.
- `apps/api/internal/agent/planner_test.go` — **new**: route/intake/plan/advance tests (fake store).
- `apps/api/internal/agent/loop.go` — **modify**: `AgentStore` gains gate/plan methods; `RunAgentStep` handles a `check_gate` candidate.
- `apps/api/internal/agent/classifier.go` — **modify**: add `CheckGateCandidates`.
- `apps/api/internal/agent/agentstore.go` — **modify**: sqlc adapter for the new store methods.
- `apps/api/internal/agent/loop_test.go` — **modify**: extend `fakeAgentStore` with gate/plan state.
- `apps/api/internal/store/queries/graph.sql` — **modify**: gate_state/plan read + body-update queries.
- `apps/api/internal/store/sqlc/*` — **generated** by `make sqlc`.
- `apps/api/internal/agent/refactor2_slice4_sqlc_test.go` — **new**: testcontainers round-trip.

---

## Task 1: The `skill` package — types, loader, validation, topo order

**Files:**
- Create: `apps/api/internal/skills/skill.go`
- Create: `apps/api/internal/skills/skill_test.go`
- Create: `apps/api/internal/skills/specs/_placeholder.json` — content `{"id":"_placeholder","kind":"project","contracts":{}}`. `//go:embed specs/*.json` needs ≥1 match to compile; Task 3's `make sync-skills` deletes every non-canonical `.json` in the mirror, so this file is transient scaffolding only.

**Interfaces:**
- Produces: `skills.Skill{ID string; Kind string; Contracts map[string]Contract; Cards []string; Vocabulary string; Intake json.RawMessage}`, `skills.Contract{Requires,Produces []string; View string; Repertoire []string; Gate Gate}`, `skills.Gate{Machine []MachineItem; StudentWritten,Human []string}`, `skills.MachineItem{Kind,Type string; N int}`, `skills.MachineKinds map[string]bool`, `skills.Load([]byte)(Skill,error)`, `(Skill).Validate() error`, `(Skill).TopoOrder()([]string,error)`, `skills.Catalog()([]Skill,error)`, `skills.ByID(string)(Skill,bool)`.

- [ ] **Step 1: Write the failing test** — `skill_test.go`

```go
package skills

import "testing"

func linearSkill() Skill {
	return Skill{
		ID:   "t",
		Kind: "project",
		Contracts: map[string]Contract{
			"a": {Gate: Gate{Machine: []MachineItem{{Kind: "node_present", Type: "x"}}}},
			"b": {Requires: []string{"a"}},
			"c": {Requires: []string{"b"}},
		},
	}
}

func TestValidate_AcceptsAcyclicResolvedDAG(t *testing.T) {
	if err := linearSkill().Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_RejectsDanglingRequires(t *testing.T) {
	s := linearSkill()
	s.Contracts["b"] = Contract{Requires: []string{"nope"}}
	if err := s.Validate(); err == nil {
		t.Fatal("want error for dangling requires")
	}
}

func TestValidate_RejectsCycle(t *testing.T) {
	s := Skill{ID: "t", Kind: "project", Contracts: map[string]Contract{
		"a": {Requires: []string{"b"}},
		"b": {Requires: []string{"a"}},
	}}
	if err := s.Validate(); err == nil {
		t.Fatal("want error for cyclic requires")
	}
}

func TestValidate_RejectsUnknownMachineKind(t *testing.T) {
	s := linearSkill()
	s.Contracts["a"] = Contract{Gate: Gate{Machine: []MachineItem{{Kind: "bogus"}}}}
	if err := s.Validate(); err == nil {
		t.Fatal("want error for unknown machine kind")
	}
}

func TestTopoOrder_IsDeterministicAndRespectsRequires(t *testing.T) {
	order, err := linearSkill().TopoOrder()
	if err != nil {
		t.Fatalf("TopoOrder: %v", err)
	}
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	if !(pos["a"] < pos["b"] && pos["b"] < pos["c"]) {
		t.Fatalf("order violates requires: %v", order)
	}
	// determinism: same input, same output
	order2, _ := linearSkill().TopoOrder()
	for i := range order {
		if order[i] != order2[i] {
			t.Fatalf("non-deterministic order: %v vs %v", order, order2)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/skills/ -run 'Validate|Topo' -v`
Expected: FAIL — package/types undefined.

- [ ] **Step 3: Write `skill.go`**

```go
// Package skills loads the embedded C5 skill catalog — a project/course type
// authored as a contract DAG (agent-spec §5.1). The specs are a generated
// mirror of packages/contracts/skills; never hand-edit specs/.
package skills

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed specs/*.json
var specFS embed.FS

// MachineKinds is the closed set of machine gate-item kinds (Slice 4). The
// names live here (config validation); the evaluation lives in the agent
// package. Adding a kind is a runtime change, not skill authoring.
var MachineKinds = map[string]bool{
	"node_present":          true,
	"node_count_at_least":   true,
	"no_orphan_evidence":    true,
	"no_unsupported_claim":  true,
	"no_single_sourced_claim": true,
	"every_source_evaluated": true,
}

// MachineItem is one machine-checkable gate item — a closed-set predicate over
// the workspace graph. Type/N are read only by the kinds that use them
// (node_present / node_count_at_least).
type MachineItem struct {
	Kind string `json:"kind"`
	Type string `json:"type"`
	N    int    `json:"n"`
}

// Gate is a contract's three-tier gate (agent-spec §5.1). Machine items are
// computed; student_written/human items are recorded externally (DEC-3).
type Gate struct {
	Machine        []MachineItem `json:"machine"`
	StudentWritten []string      `json:"student_written"`
	Human          []string      `json:"human"`
}

// Contract is one milestone in the DAG.
type Contract struct {
	Requires   []string `json:"requires"`
	Produces   []string `json:"produces"`
	View       string   `json:"view"`
	Repertoire []string `json:"repertoire"`
	Gate       Gate     `json:"gate"`
}

// Skill is a project/course type: a contract DAG plus card references and
// pedagogy metadata. Intake is the declared procedure (run by the planner);
// Slice 4 keeps it as raw config it does not interpret.
type Skill struct {
	ID         string              `json:"id"`
	Kind       string              `json:"kind"`
	Contracts  map[string]Contract `json:"contracts"`
	Intake     json.RawMessage     `json:"intake"`
	Vocabulary string              `json:"vocabulary"`
	Cards      []string            `json:"cards"`
}

// Load parses and validates one skill JSON blob.
func Load(data []byte) (Skill, error) {
	var s Skill
	if err := json.Unmarshal(data, &s); err != nil {
		return Skill{}, fmt.Errorf("parse skill: %w", err)
	}
	if err := s.Validate(); err != nil {
		return Skill{}, err
	}
	return s, nil
}

// Validate rejects a malformed contract DAG at load: unknown kind/machine-kind,
// dangling requires, or a cycle (via TopoOrder).
func (s Skill) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("skill: empty id")
	}
	if s.Kind != "project" && s.Kind != "course" {
		return fmt.Errorf("skill %s: bad kind %q", s.ID, s.Kind)
	}
	for id, c := range s.Contracts {
		for _, req := range c.Requires {
			if _, ok := s.Contracts[req]; !ok {
				return fmt.Errorf("skill %s: contract %s requires unknown %s", s.ID, id, req)
			}
		}
		for _, m := range c.Gate.Machine {
			if !MachineKinds[m.Kind] {
				return fmt.Errorf("skill %s: contract %s unknown machine kind %q", s.ID, id, m.Kind)
			}
		}
	}
	if _, err := s.TopoOrder(); err != nil {
		return err
	}
	return nil
}

// TopoOrder returns the contract ids in a deterministic topological order
// (requires-before-dependent). Ties broken by id for stability. Errors on a
// cycle.
func (s Skill) TopoOrder() ([]string, error) {
	indeg := map[string]int{}
	for id := range s.Contracts {
		indeg[id] = 0
	}
	for id, c := range s.Contracts {
		for _, req := range c.Requires {
			if _, ok := s.Contracts[req]; ok {
				indeg[id]++
			}
		}
	}
	var order []string
	for len(order) < len(s.Contracts) {
		// pick the lowest-id node with indeg 0 not yet emitted → determinism
		var ready []string
		for id, d := range indeg {
			if d == 0 {
				ready = append(ready, id)
			}
		}
		if len(ready) == 0 {
			return nil, fmt.Errorf("skill %s: contract DAG has a cycle", s.ID)
		}
		sort.Strings(ready)
		pick := ready[0]
		order = append(order, pick)
		delete(indeg, pick)
		for id, c := range s.Contracts {
			if _, done := indeg[id]; !done {
				continue
			}
			for _, req := range c.Requires {
				if req == pick {
					indeg[id]--
				}
			}
		}
	}
	return order, nil
}

// Catalog reads and parses every embedded skill, sorted by id.
func Catalog() ([]Skill, error) {
	entries, err := specFS.ReadDir("specs")
	if err != nil {
		return nil, fmt.Errorf("read embedded skills: %w", err)
	}
	var out []Skill
	for _, e := range entries {
		if e.IsDir() || len(e.Name()) < 5 || e.Name()[len(e.Name())-5:] != ".json" {
			continue
		}
		data, err := specFS.ReadFile("specs/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		s, err := Load(data)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", e.Name(), err)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ByID returns the embedded skill with the given id, if present.
func ByID(id string) (Skill, bool) {
	all, err := Catalog()
	if err != nil {
		return Skill{}, false
	}
	for _, s := range all {
		if s.ID == id {
			return s, true
		}
	}
	return Skill{}, false
}
```

**Embed note:** the `_placeholder.json` (Files list above) is what makes `//go:embed specs/*.json` compile in Task 1; Task 3's `make sync-skills` deletes it. `Catalog()` will parse it as a valid empty skill until then — harmless (Task 1's tests build skills inline, not from the catalog).

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/skills/ -run 'Validate|Topo' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/skills/
git commit -m "feat(api): skills package — C5 skill types, loader, DAG validation, topo order

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: `syncskills` tool + `sync-skills` target

**Files:**
- Create: `apps/api/tools/syncskills/main.go`
- Modify: `apps/api/Makefile:1` (add `sync-skills` to `.PHONY` and a target)
- Create: `packages/contracts/skills/` (dir; JSON authored in Task 3)

**Interfaces:**
- Produces: a `go run ./tools/syncskills` command mirroring `packages/contracts/skills/*.json` → `internal/skills/specs/`.

- [ ] **Step 1: Write `syncskills/main.go`** (copy of `synccards`, retargeted)

```go
// Command syncskills mirrors the canonical skill JSON specs from
// packages/contracts/skills into internal/skills/specs so they can be embedded.
// It is the ONLY writer of the mirror; never hand-edit internal/skills/specs.
//
// Run from the apps/api module root: `go run ./tools/syncskills` (or `make sync-skills`).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const canonicalRel = "../../packages/contracts/skills"
const mirrorRel = "internal/skills/specs"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "syncskills:", err)
		os.Exit(1)
	}
}

func run() error {
	entries, err := os.ReadDir(canonicalRel)
	if err != nil {
		return fmt.Errorf("read canonical dir %s: %w", canonicalRel, err)
	}
	if err := os.MkdirAll(mirrorRel, 0o755); err != nil {
		return fmt.Errorf("mkdir mirror: %w", err)
	}
	mirrorEntries, err := os.ReadDir(mirrorRel)
	if err != nil {
		return fmt.Errorf("read mirror dir: %w", err)
	}
	for _, e := range mirrorEntries {
		if strings.HasSuffix(e.Name(), ".json") {
			if err := os.Remove(filepath.Join(mirrorRel, e.Name())); err != nil {
				return fmt.Errorf("remove stale %s: %w", e.Name(), err)
			}
		}
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(canonicalRel, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(mirrorRel, e.Name()), data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", e.Name(), err)
		}
		count++
	}
	fmt.Printf("syncskills: mirrored %d skill specs\n", count)
	return nil
}
```

- [ ] **Step 2: Edit `apps/api/Makefile`** — add the target after `sync-cards`:

```makefile
.PHONY: sync-cards sync-skills sqlc migrate-up test run

# Mirror the canonical card JSON specs into the embeddable directory.
sync-cards:
	go run ./tools/synccards

# Mirror the canonical skill JSON specs into the embeddable directory.
sync-skills:
	go run ./tools/syncskills
```

- [ ] **Step 3: Verify the tool builds** (no canonical JSON yet — expect a clean "mirrored 0" or a missing-dir error until Task 3; create the empty canonical dir now)

Run: `mkdir -p packages/contracts/skills && cd apps/api && go build ./tools/syncskills`
Expected: builds clean.

- [ ] **Step 4: Commit**

```bash
git add apps/api/tools/syncskills/main.go apps/api/Makefile packages/contracts/skills/
git commit -m "build(api): syncskills tool + make sync-skills target

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: Author the writing-project skill (S0–S6) + sync + load test

**Files:**
- Create: `packages/contracts/skills/writing-project.json`
- Generate: `apps/api/internal/skills/specs/writing-project.json` (via `make sync-skills`)
- Delete: `apps/api/internal/skills/specs/_placeholder.json` (sync removes it)
- Modify: `apps/api/internal/skills/skill_test.go` (add the writing-project load test)

**Interfaces:**
- Consumes: `skills.ByID`, `skills.Skill.Validate` (Task 1); `cards.ByID` (registry, for the card-ref resolution assertion).

- [ ] **Step 1: Write the failing test** — append to `skill_test.go`:

```go
func TestWritingProject_LoadsValidatesAndReferencesRealCards(t *testing.T) {
	s, ok := ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill not embedded")
	}
	if s.Kind != "project" {
		t.Fatalf("kind = %q", s.Kind)
	}
	for _, want := range []string{"decode_task", "frame_question", "evaluate_perspectives",
		"evaluate_sources", "build_argument", "draft_polish", "reflect_archive"} {
		if _, ok := s.Contracts[want]; !ok {
			t.Fatalf("missing contract %s", want)
		}
	}
	// requires chain holds (build_argument after evaluate_sources)
	order, err := s.TopoOrder()
	if err != nil {
		t.Fatalf("TopoOrder: %v", err)
	}
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	if pos["evaluate_sources"] >= pos["build_argument"] {
		t.Fatal("build_argument must come after evaluate_sources")
	}
}
```

Card-ref resolution lives in the `cards`-aware package to avoid a `skills→cards` dep; assert it in `agent` — add to `apps/api/internal/agent/planner_test.go` in Task 6, or as a standalone here if `cards` import is acceptable. **Decision:** keep `skills` dep-free; the card-ref-resolves assertion goes in `agent` Task 6.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/skills/ -run WritingProject -v`
Expected: FAIL — skill not embedded.

- [ ] **Step 3: Author `packages/contracts/skills/writing-project.json`**

Faithful to product spec §5.2 (S0–S6 gate items) and the spec's §3 table. A linear `requires` chain; the S2–S4 loop is unlock-not-revisit, not a DAG cycle. Machine items use only the closed set; semantically-hard items go under `student_written`/`human`.

```json
{
  "id": "writing-project",
  "kind": "project",
  "vocabulary": "per-board",
  "cards": ["craap", "sift", "concession", "steelman"],
  "intake": { "procedure": "decompose_imported_to_graph" },
  "contracts": {
    "decode_task": {
      "requires": [],
      "produces": ["rubric_translation", "weakness_prediction", "milestone_plan"],
      "view": "评估",
      "repertoire": [],
      "gate": {
        "machine": [
          { "kind": "node_present", "type": "rubric_translation" },
          { "kind": "node_count_at_least", "type": "weakness_prediction", "n": 2 }
        ],
        "student_written": ["milestone_plan"],
        "human": []
      }
    },
    "frame_question": {
      "requires": ["decode_task"],
      "produces": ["research_question", "provisional_answer", "preregistration"],
      "view": "结构",
      "repertoire": [],
      "gate": {
        "machine": [
          { "kind": "node_present", "type": "research_question" },
          { "kind": "node_present", "type": "provisional_answer" },
          { "kind": "node_present", "type": "preregistration" }
        ],
        "student_written": ["terms_defined"],
        "human": []
      }
    },
    "evaluate_perspectives": {
      "requires": ["frame_question"],
      "produces": ["perspective"],
      "view": "素材",
      "repertoire": [],
      "gate": {
        "machine": [
          { "kind": "node_count_at_least", "type": "perspective", "n": 2 }
        ],
        "student_written": ["recon_logged", "sources_per_perspective"],
        "human": []
      }
    },
    "evaluate_sources": {
      "requires": ["evaluate_perspectives"],
      "produces": ["evidence"],
      "view": "素材",
      "repertoire": ["craap", "sift"],
      "gate": {
        "machine": [
          { "kind": "every_source_evaluated" },
          { "kind": "no_single_sourced_claim" }
        ],
        "student_written": ["source_risk_notes", "lateral_read_logged"],
        "human": ["source_quality_spot_check"]
      }
    },
    "build_argument": {
      "requires": ["evaluate_sources"],
      "produces": ["claim", "concession"],
      "view": "结构",
      "repertoire": ["concession", "steelman"],
      "gate": {
        "machine": [
          { "kind": "no_orphan_evidence" },
          { "kind": "no_unsupported_claim" },
          { "kind": "no_single_sourced_claim" },
          { "kind": "node_present", "type": "concession" }
        ],
        "student_written": ["warrants", "steelman"],
        "human": ["warrant_quality_spot_check"]
      }
    },
    "draft_polish": {
      "requires": ["build_argument"],
      "produces": ["word_budget_ok"],
      "view": "写作",
      "repertoire": [],
      "gate": {
        "machine": [
          { "kind": "node_present", "type": "word_budget_ok" }
        ],
        "student_written": ["citations_matched"],
        "human": ["whole_draft_review"]
      }
    },
    "reflect_archive": {
      "requires": ["draft_polish"],
      "produces": ["reflection"],
      "view": "评估",
      "repertoire": [],
      "gate": {
        "machine": [],
        "student_written": ["reflection"],
        "human": ["declaration_signed"]
      }
    }
  }
}
```

- [ ] **Step 4: Sync and run the test**

Run: `cd apps/api && make sync-skills && go test ./internal/skills/ -run 'WritingProject|Validate|Topo' -v`
Expected: `syncskills: mirrored 1 skill specs`, then PASS. Confirm `_placeholder.json` is gone.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/skills/writing-project.json apps/api/internal/skills/specs/ apps/api/internal/skills/skill_test.go
git commit -m "feat(contracts): writing-project skill — S0–S6 as a C5 contract DAG

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: Machine gate predicates (closed set, pure)

**Files:**
- Create: `apps/api/internal/agent/gate.go`
- Create: `apps/api/internal/agent/gate_test.go`

**Interfaces:**
- Consumes: `GraphView`/`GraphNodeView`/`GraphEdgeView`/`MaterialView` (runtime.go); `skills.MachineItem`.
- Produces: `func evalMachineItem(item skills.MachineItem, g GraphView) (pass bool, missing string)` (unexported; exercised via `CheckGate` in Task 5 but unit-tested directly here through a thin exported test shim `EvalMachineItemForTest`).

- [ ] **Step 1: Write the failing test** — `gate_test.go`

```go
package agent

import (
	"testing"

	"mindimprint/api/internal/skills"
)

func TestEvalMachineItem_NodePresentAndCount(t *testing.T) {
	g := GraphView{Nodes: []GraphNodeView{
		{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"},
	}}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "node_present", Type: "perspective"}, g); !pass {
		t.Fatal("node_present should pass")
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "node_present", Type: "concession"}, g); pass {
		t.Fatal("node_present(concession) should fail")
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "node_count_at_least", Type: "perspective", N: 2}, g); !pass {
		t.Fatal("count>=2 should pass")
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "node_count_at_least", Type: "perspective", N: 3}, g); pass {
		t.Fatal("count>=3 should fail")
	}
}

func TestEvalMachineItem_ArgumentGraphPredicates(t *testing.T) {
	// claim c1 supported by two distinct evidence e1,e2 → healthy.
	// evidence e3 orphaned (no supports edge) → no_orphan_evidence fails.
	g := GraphView{
		Nodes: []GraphNodeView{
			{ID: "c1", Type: "claim"}, {ID: "e1", Type: "evidence"},
			{ID: "e2", Type: "evidence"}, {ID: "e3", Type: "evidence"},
		},
		Edges: []GraphEdgeView{
			{FromKind: "graph_node", FromID: "e1", ToKind: "graph_node", ToID: "c1", Type: "supports"},
			{FromKind: "graph_node", FromID: "e2", ToKind: "graph_node", ToID: "c1", Type: "supports"},
		},
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "no_orphan_evidence"}, g); pass {
		t.Fatal("e3 is orphaned → should fail")
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "no_unsupported_claim"}, g); !pass {
		t.Fatal("c1 has support → should pass")
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "no_single_sourced_claim"}, g); !pass {
		t.Fatal("c1 has two evidence → should pass")
	}

	// A claim with a single supporting evidence fails no_single_sourced_claim.
	single := GraphView{
		Nodes: []GraphNodeView{{ID: "c9", Type: "claim"}, {ID: "e9", Type: "evidence"}},
		Edges: []GraphEdgeView{{FromKind: "graph_node", FromID: "e9", ToKind: "graph_node", ToID: "c9", Type: "supports"}},
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "no_single_sourced_claim"}, single); pass {
		t.Fatal("single-sourced claim → should fail")
	}
}

func TestEvalMachineItem_EverySourceEvaluated(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}, {ID: "m2", Kind: "article"}},
		Edges: []GraphEdgeView{
			{FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "ev1", Type: "evaluated-as"},
		},
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "every_source_evaluated"}, g); pass {
		t.Fatal("m2 not evaluated → should fail")
	}
	g.Edges = append(g.Edges, GraphEdgeView{FromKind: "material", FromID: "m2", ToKind: "graph_node", ToID: "ev2", Type: "evaluated-as"})
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "every_source_evaluated"}, g); !pass {
		t.Fatal("all sources evaluated → should pass")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run EvalMachineItem -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Write `gate.go`** (predicates + the test shim)

```go
package agent

import (
	"fmt"

	"mindimprint/api/internal/skills"
)

// evalMachineItem evaluates one closed-set machine gate predicate over the
// graph (agent-spec §5.1; Slice 4's closed set). Returns pass + a
// human-readable "what's missing" when it fails. A new kind is a runtime
// change — an unknown kind fails closed with a diagnostic.
func evalMachineItem(item skills.MachineItem, g GraphView) (bool, string) {
	switch item.Kind {
	case "node_present":
		for _, n := range g.Nodes {
			if n.Type == item.Type {
				return true, ""
			}
		}
		return false, fmt.Sprintf("缺少 %s", item.Type)
	case "node_count_at_least":
		c := 0
		for _, n := range g.Nodes {
			if n.Type == item.Type {
				c++
			}
		}
		if c >= item.N {
			return true, ""
		}
		return false, fmt.Sprintf("%s 需要至少 %d 个（当前 %d）", item.Type, item.N, c)
	case "no_orphan_evidence":
		for _, n := range g.Nodes {
			if n.Type == "evidence" && !hasOutgoingSupports(n.ID, g) {
				return false, "存在未连到主张的证据"
			}
		}
		return true, ""
	case "no_unsupported_claim":
		for _, n := range g.Nodes {
			if n.Type == "claim" && len(supportingEvidence(n.ID, g)) == 0 {
				return false, "存在没有证据支撑的主张"
			}
		}
		return true, ""
	case "no_single_sourced_claim":
		for _, n := range g.Nodes {
			if n.Type == "claim" && len(supportingEvidence(n.ID, g)) < 2 {
				return false, "存在只靠单一来源的主张"
			}
		}
		return true, ""
	case "every_source_evaluated":
		for _, m := range g.Materials {
			if m.Kind != "article" {
				continue
			}
			if !hasEvaluatedEdge(m.ID, g) {
				return false, "有来源尚未做来源评估"
			}
		}
		return true, ""
	default:
		return false, fmt.Sprintf("未知的机器判据 %q", item.Kind)
	}
}

func hasOutgoingSupports(nodeID string, g GraphView) bool {
	for _, e := range g.Edges {
		if e.Type == "supports" && e.FromKind == "graph_node" && e.FromID == nodeID {
			return true
		}
	}
	return false
}

// supportingEvidence returns the distinct evidence node ids that support a
// claim via a "supports" edge (evidence -> claim). Slice-4 proxy for source
// distinctness (refined to distinct source materials in Slice 7).
func supportingEvidence(claimID string, g GraphView) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range g.Edges {
		if e.Type == "supports" && e.ToKind == "graph_node" && e.ToID == claimID && e.FromKind == "graph_node" {
			if !seen[e.FromID] {
				seen[e.FromID] = true
				out = append(out, e.FromID)
			}
		}
	}
	return out
}

func hasEvaluatedEdge(materialID string, g GraphView) bool {
	for _, e := range g.Edges {
		if e.Type == "evaluated-as" && e.FromKind == "material" && e.FromID == materialID {
			return true
		}
	}
	return false
}

// EvalMachineItemForTest exposes evalMachineItem for the gate_test.go unit
// tests without widening the package API surface elsewhere.
func EvalMachineItemForTest(item skills.MachineItem, g GraphView) (bool, string) {
	return evalMachineItem(item, g)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run EvalMachineItem -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/gate.go apps/api/internal/agent/gate_test.go
git commit -m "feat(api): machine gate predicates — closed set over the workspace graph

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5: `CheckGate` + `GateReport` + `RecordedGate` (DEC-3)

**Files:**
- Modify: `apps/api/internal/agent/gate.go` (add the aggregation)
- Modify: `apps/api/internal/agent/gate_test.go` (add CheckGate tests)

**Interfaces:**
- Produces: `type RecordedGate struct{ Confirmed bool; Items map[string]string }`, `type ItemResult struct{ Name, Kind string; Pass bool; Missing string }`, `type GateReport struct{ Contract, Status string; Solid bool; Items []ItemResult; Missing []string }`, `func CheckGate(sk skills.Skill, contractID string, g GraphView, rec RecordedGate) GateReport`.
- Status is one of `"empty"|"partial"|"machine_clear"` — **never** `"solid"` (DEC-3). `Solid` mirrors `rec.Confirmed`.

- [ ] **Step 1: Write the failing test** — append to `gate_test.go`:

```go
func TestCheckGate_MachineClearNeverSolid(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// evaluate_perspectives needs 2 perspectives (machine) + student items.
	g := GraphView{Nodes: []GraphNodeView{
		{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"},
	}}
	// no recorded non-machine items, not confirmed
	r := CheckGate(sk, "evaluate_perspectives", g, RecordedGate{})
	if r.Status != "machine_clear" {
		t.Fatalf("Status = %q, want machine_clear", r.Status)
	}
	if r.Solid {
		t.Fatal("DEC-3: CheckGate must never report Solid without a recorded confirmation")
	}
	// missing lists the owed student_written items
	if len(r.Missing) == 0 {
		t.Fatal("want student_written items reported as missing")
	}
}

func TestCheckGate_EmptyAndPartial(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	empty := CheckGate(sk, "evaluate_perspectives", GraphView{}, RecordedGate{})
	if empty.Status != "empty" {
		t.Fatalf("Status = %q, want empty", empty.Status)
	}
	partial := CheckGate(sk, "evaluate_perspectives", GraphView{
		Nodes: []GraphNodeView{{ID: "p1", Type: "perspective"}}, // only 1 of 2
	}, RecordedGate{})
	if partial.Status != "partial" {
		t.Fatalf("Status = %q, want partial", partial.Status)
	}
}

func TestCheckGate_SolidOnlyFromRecordedConfirmation(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	g := GraphView{Nodes: []GraphNodeView{{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"}}}
	r := CheckGate(sk, "evaluate_perspectives", g, RecordedGate{
		Confirmed: true,
		Items:     map[string]string{"recon_logged": "solid", "sources_per_perspective": "solid"},
	})
	if !r.Solid {
		t.Fatal("recorded confirmation → Solid true")
	}
	if r.Status == "solid" {
		t.Fatal("DEC-3: Status enum never carries solid; Solid is a separate recorded flag")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run CheckGate -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Add to `gate.go`**

```go
// RecordedGate is the externally-recorded (non-machine) status of a gate,
// read from its gate_state node body. Confirmed is the external "solid"
// (a passed challenge / human / explicit student confirmation — DEC-3); Items
// maps each student_written/human item name to "solid"|"flagged-weak"|"".
type RecordedGate struct {
	Confirmed bool
	Items     map[string]string
}

// ItemResult is one gate item's outcome in a report.
type ItemResult struct {
	Name    string
	Kind    string // "machine" | "student_written" | "human"
	Pass    bool
	Missing string
}

// GateReport is CheckGate's verdict. Status is the MACHINE computation and
// never exceeds "machine_clear" (DEC-3); Solid mirrors the recorded external
// confirmation. Missing is the ordered list of what remains.
type GateReport struct {
	Contract string
	Status   string // "empty" | "partial" | "machine_clear"
	Solid    bool
	Items    []ItemResult
	Missing  []string
}

// CheckGate evaluates a contract's gate: machine items over the graph, merged
// with the recorded status of student_written/human items. It never returns
// "solid" as a machine Status (DEC-3) — Solid is a separate recorded flag.
func CheckGate(sk skills.Skill, contractID string, g GraphView, rec RecordedGate) GateReport {
	c := sk.Contracts[contractID]
	rep := GateReport{Contract: contractID, Solid: rec.Confirmed}

	machinePass := true
	anyPass := false
	for _, m := range c.Gate.Machine {
		pass, missing := evalMachineItem(m, g)
		name := m.Kind
		if m.Type != "" {
			name = m.Kind + ":" + m.Type
		}
		rep.Items = append(rep.Items, ItemResult{Name: name, Kind: "machine", Pass: pass, Missing: missing})
		if pass {
			anyPass = true
		} else {
			machinePass = false
			rep.Missing = append(rep.Missing, missing)
		}
	}
	for _, tier := range []struct {
		names []string
		kind  string
	}{{c.Gate.StudentWritten, "student_written"}, {c.Gate.Human, "human"}} {
		for _, name := range tier.names {
			sat := rec.Items[name] == "solid"
			rep.Items = append(rep.Items, ItemResult{Name: name, Kind: tier.kind, Pass: sat})
			if sat {
				anyPass = true
			} else {
				rep.Missing = append(rep.Missing, name+" 待完成")
			}
		}
	}

	switch {
	case machinePass:
		rep.Status = "machine_clear"
	case anyPass:
		rep.Status = "partial"
	default:
		rep.Status = "empty"
	}
	return rep
}
```

Note: a gate with **no** machine items (e.g. `reflect_archive`) has `machinePass == true` vacuously → `machine_clear`, with the student/human items still in `Missing` and `Solid` false until recorded. That is correct: the structural bar is met, the human/student bar is owed.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run 'CheckGate|EvalMachineItem' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/gate.go apps/api/internal/agent/gate_test.go
git commit -m "feat(api): CheckGate — three-tier gate report, DEC-3 (machine never solid)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 6: `ReconcileGates` + `Route` (pure) + card-ref assertion

**Files:**
- Modify: `apps/api/internal/agent/gate.go` (add `ReconcileGates`)
- Create: `apps/api/internal/agent/planner.go` (add `Route` here; Intake/Plan/Advance land in Tasks 8–9)
- Create: `apps/api/internal/agent/planner_test.go`

**Interfaces:**
- Produces: `func ReconcileGates(sk skills.Skill, g GraphView, recorded map[string]RecordedGate) map[string]GateReport`, `func Route(sk skills.Skill, reports map[string]GateReport) []string`.
- Reachability: a contract is routable iff every `requires` report is `Solid` or `Status=="machine_clear"`, and the contract itself is not `Solid`. Route = routable-and-unmet in `TopoOrder`.

- [ ] **Step 1: Write the failing test** — `planner_test.go`

```go
package agent

import (
	"testing"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/skills"
)

func TestWritingProject_CardsResolveInRegistry(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	for _, id := range sk.Cards {
		if _, ok := cards.ByID(id); !ok {
			t.Fatalf("writing-project references unknown card %q", id)
		}
	}
	for cid, c := range sk.Contracts {
		for _, id := range c.Repertoire {
			if _, ok := cards.ByID(id); !ok {
				t.Fatalf("contract %s repertoire references unknown card %q", cid, id)
			}
		}
	}
}

func TestRoute_RespectsRequiresAndStartsFromFrontier(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// Nothing done: only decode_task (no requires) is routable.
	reports := map[string]GateReport{}
	for id := range sk.Contracts {
		reports[id] = GateReport{Contract: id, Status: "empty"}
	}
	route := Route(sk, reports)
	if len(route) == 0 || route[0] != "decode_task" {
		t.Fatalf("route should start at decode_task, got %v", route)
	}
	for _, id := range route {
		if id == "build_argument" {
			t.Fatal("build_argument must not be routable before evaluate_sources clears")
		}
	}
}

func TestRoute_UnlocksNextWhenPredecessorMachineClear(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	reports := map[string]GateReport{}
	for id := range sk.Contracts {
		reports[id] = GateReport{Contract: id, Status: "empty"}
	}
	reports["decode_task"] = GateReport{Contract: "decode_task", Status: "machine_clear", Solid: true}
	route := Route(sk, reports)
	// decode_task is Solid → excluded; frame_question now routable.
	for _, id := range route {
		if id == "decode_task" {
			t.Fatal("solid contract must be excluded from the route")
		}
	}
	if route[0] != "frame_question" {
		t.Fatalf("frame_question should be the new frontier, got %v", route)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run 'Route|CardsResolve' -v`
Expected: FAIL — `Route`/`ReconcileGates` undefined.

- [ ] **Step 3: Add `ReconcileGates` to `gate.go`**

```go
// ReconcileGates runs CheckGate for every contract against the reconstructed
// state — the "owe every gate" diff (agent-spec §5.2). recorded may be nil.
func ReconcileGates(sk skills.Skill, g GraphView, recorded map[string]RecordedGate) map[string]GateReport {
	out := make(map[string]GateReport, len(sk.Contracts))
	for id := range sk.Contracts {
		out[id] = CheckGate(sk, id, g, recorded[id])
	}
	return out
}
```

- [ ] **Step 4: Create `planner.go` with `Route`**

```go
// Package agent — planner.go: the deterministic Project planner (agent-spec
// §5.2). Slice 4 computes the route from gate state + the contract DAG; the
// flagship model-judgment layer (prioritization, route_to_course on stalls)
// is a documented later seam.
package agent

import (
	"mindimprint/api/internal/skills"
)

// Route is the advisory route: the unmet-and-reachable contracts in
// topological DAG order. A contract is reachable iff every `requires` gate is
// machine_clear-or-solid (work may begin once predecessors are structurally
// sound); "unmet" means not yet Solid. Pure function.
func Route(sk skills.Skill, reports map[string]GateReport) []string {
	order, err := sk.TopoOrder()
	if err != nil {
		return nil
	}
	var route []string
	for _, id := range order {
		if reports[id].Solid {
			continue // finished
		}
		reachable := true
		for _, req := range sk.Contracts[id].Requires {
			r := reports[req]
			if !(r.Solid || r.Status == "machine_clear") {
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

- [ ] **Step 5: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run 'Route|CardsResolve|CheckGate|EvalMachineItem' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/agent/gate.go apps/api/internal/agent/planner.go apps/api/internal/agent/planner_test.go
git commit -m "feat(api): ReconcileGates (owe every gate) + deterministic Route over the DAG

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 7: Store methods — gate_state + plan nodes (sqlc + adapter + fake)

**Files:**
- Modify: `apps/api/internal/store/queries/graph.sql`
- Generate: `apps/api/internal/store/sqlc/*` (via `make sqlc`)
- Modify: `apps/api/internal/agent/loop.go` (extend `AgentStore` interface)
- Modify: `apps/api/internal/agent/agentstore.go` (adapter methods)
- Modify: `apps/api/internal/agent/loop_test.go` (extend `fakeAgentStore`)

**Interfaces:**
- Produces on `AgentStore`:
  - `ListGateStates(ctx, projectID uuid.UUID) (map[string]RecordedGate, error)`
  - `UpsertGateState(ctx, projectID uuid.UUID, contract string, rec RecordedGate) error`
  - `UpsertPlan(ctx, projectID uuid.UUID, body []byte) error`
- Uses new sqlc queries `ListGateStateNodes`, `GetGateStateNode`, `GetPlanNode`, `UpdateGraphNodeBody`, plus existing `InsertGraphNode`.
- The gate_state body JSON shape: `{"contract":"…","status":"…","confirmed_solid":bool,"items":{"name":"solid"}}`.

- [ ] **Step 1: Write the failing test** — append to `loop_test.go` (fake-level round-trip):

```go
func TestFakeStore_GateStateAndPlanRoundTrip(t *testing.T) {
	f := &fakeAgentStore{}
	pid := uuid.New()
	if err := f.UpsertGateState(context.Background(), pid, "decode_task",
		RecordedGate{Confirmed: true, Items: map[string]string{"milestone_plan": "solid"}}); err != nil {
		t.Fatalf("UpsertGateState: %v", err)
	}
	// upsert again (same contract) must not duplicate
	_ = f.UpsertGateState(context.Background(), pid, "decode_task", RecordedGate{Confirmed: true})
	states, err := f.ListGateStates(context.Background(), pid)
	if err != nil {
		t.Fatalf("ListGateStates: %v", err)
	}
	if len(states) != 1 || !states["decode_task"].Confirmed {
		t.Fatalf("want one confirmed gate_state, got %+v", states)
	}
	if err := f.UpsertPlan(context.Background(), pid, []byte(`{"route":["frame_question"]}`)); err != nil {
		t.Fatalf("UpsertPlan: %v", err)
	}
	if f.upsertPlanCalls != 1 {
		t.Fatalf("want 1 plan upsert, got %d", f.upsertPlanCalls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run GateStateAndPlanRoundTrip -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Add sqlc queries to `graph.sql`**

```sql
-- name: GetGateStateNode :one
SELECT * FROM graph_node
WHERE project_id = $1 AND type = 'gate_state' AND body->>'contract' = $2
LIMIT 1;

-- name: ListGateStateNodes :many
SELECT * FROM graph_node
WHERE project_id = $1 AND type = 'gate_state'
ORDER BY created_at, id;

-- name: GetPlanNode :one
SELECT * FROM graph_node
WHERE project_id = $1 AND type = 'plan'
ORDER BY created_at, id
LIMIT 1;

-- name: UpdateGraphNodeBody :one
UPDATE graph_node SET body = $2 WHERE id = $1 RETURNING *;
```

Run: `cd apps/api && make sqlc` (regenerates `internal/store/sqlc`). Expected: clean.

- [ ] **Step 4: Extend the `AgentStore` interface in `loop.go`**

Add to the interface:

```go
	// Gate/plan graph-node state (Slice 4). gate_state is one graph_node per
	// (project, contract) keyed on body->>'contract'; plan is one per project.
	ListGateStates(ctx context.Context, projectID uuid.UUID) (map[string]RecordedGate, error)
	UpsertGateState(ctx context.Context, projectID uuid.UUID, contract string, rec RecordedGate) error
	UpsertPlan(ctx context.Context, projectID uuid.UUID, body []byte) error
```

- [ ] **Step 5: Implement adapter methods in `agentstore.go`**

```go
// gateStateBody is the gate_state graph_node body shape.
type gateStateBody struct {
	Contract       string            `json:"contract"`
	Status         string            `json:"status"`
	ConfirmedSolid bool              `json:"confirmed_solid"`
	Items          map[string]string `json:"items"`
}

func (s *sqlcAgentStore) ListGateStates(ctx context.Context, projectID uuid.UUID) (map[string]RecordedGate, error) {
	rows, err := s.q.ListGateStateNodes(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]RecordedGate, len(rows))
	for _, r := range rows {
		var b gateStateBody
		if err := json.Unmarshal(r.Body, &b); err != nil {
			continue // a malformed body is treated as no recorded state
		}
		out[b.Contract] = RecordedGate{Confirmed: b.ConfirmedSolid, Items: b.Items}
	}
	return out, nil
}

func (s *sqlcAgentStore) UpsertGateState(ctx context.Context, projectID uuid.UUID, contract string, rec RecordedGate) error {
	body, err := json.Marshal(gateStateBody{
		Contract: contract, ConfirmedSolid: rec.Confirmed, Items: rec.Items,
	})
	if err != nil {
		return err
	}
	existing, err := s.q.GetGateStateNode(ctx, sqlc.GetGateStateNodeParams{ProjectID: projectID, Body: contract})
	if err == nil {
		_, err = s.q.UpdateGraphNodeBody(ctx, sqlc.UpdateGraphNodeBodyParams{ID: existing.ID, Body: body})
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = s.q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "gate_state", Body: body, Author: "ai",
	})
	return err
}

func (s *sqlcAgentStore) UpsertPlan(ctx context.Context, projectID uuid.UUID, body []byte) error {
	existing, err := s.q.GetPlanNode(ctx, projectID)
	if err == nil {
		_, err = s.q.UpdateGraphNodeBody(ctx, sqlc.UpdateGraphNodeBodyParams{ID: existing.ID, Body: body})
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = s.q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "plan", Body: body, Author: "ai",
	})
	return err
}
```

Add imports `errors` and `github.com/jackc/pgx/v5` to `agentstore.go`. **Verify** the generated `GetGateStateNodeParams` field for `body->>'contract' = $2` is named `Body` (sqlc names it after the column expression); if sqlc names it differently (e.g. `Column2`), use that generated name. Confirm by reading the regenerated `graph.sql.go` before writing this method.

- [ ] **Step 6: Extend `fakeAgentStore` in `loop_test.go`**

Add fields + methods:

```go
	gateStates     map[string]RecordedGate // keyed by contract
	upsertPlanCalls int
	lastPlanBody    []byte
	gateAttemptEvents []EventRow
```

```go
func (f *fakeAgentStore) ListGateStates(context.Context, uuid.UUID) (map[string]RecordedGate, error) {
	if f.gateStates == nil {
		return map[string]RecordedGate{}, nil
	}
	return f.gateStates, nil
}

func (f *fakeAgentStore) UpsertGateState(_ context.Context, _ uuid.UUID, contract string, rec RecordedGate) error {
	if f.gateStates == nil {
		f.gateStates = map[string]RecordedGate{}
	}
	f.gateStates[contract] = rec
	return nil
}

func (f *fakeAgentStore) UpsertPlan(_ context.Context, _ uuid.UUID, body []byte) error {
	f.upsertPlanCalls++
	f.lastPlanBody = body
	return nil
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd apps/api && go test ./internal/agent/ -run 'GateStateAndPlanRoundTrip|Loop|Card|Route|CheckGate' -short -v`
Expected: PASS (all existing agent tests still green — the interface grew, the fake implements it).

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/store/queries/graph.sql apps/api/internal/store/sqlc/ apps/api/internal/agent/loop.go apps/api/internal/agent/agentstore.go apps/api/internal/agent/loop_test.go
git commit -m "feat(api): store — gate_state/plan graph-node upsert + read (sqlc + adapter + fake)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 8: `Intake` — mint imported nodes + reconcile + first plan

**Files:**
- Modify: `apps/api/internal/agent/planner.go`
- Modify: `apps/api/internal/agent/planner_test.go`

**Interfaces:**
- Produces: `type IntakeCandidate struct{ Type, Text string }`, `func Intake(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, candidates []IntakeCandidate) ([]string, error)` returning the first route.
- Uses `deps.Store.InsertGraphNode` (author `imported`), `LoadGraph`, `ListGateStates`, `UpsertPlan`, `AppendEvent`.

- [ ] **Step 1: Write the failing test** — append to `planner_test.go`:

```go
func TestIntake_MintsImportedNodesAndWritesFirstPlan(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{}
	deps := AgentDeps{Store: f}
	pid := uuid.New()

	// A mid-way arrival: a research question + provisional answer already written.
	route, err := Intake(context.Background(), deps, pid, sk, []IntakeCandidate{
		{Type: "research_question", Text: "中国的经济转型是否让地球更可持续？"},
		{Type: "provisional_answer", Text: "部分是。"},
	})
	if err != nil {
		t.Fatalf("Intake: %v", err)
	}
	if f.insertGraphNodeCalls != 2 {
		t.Fatalf("want 2 imported nodes, got %d", f.insertGraphNodeCalls)
	}
	if f.lastImportedAuthor != "imported" {
		t.Fatalf("imported nodes must carry author=imported, got %q", f.lastImportedAuthor)
	}
	if f.upsertPlanCalls != 1 {
		t.Fatal("Intake must write the first plan")
	}
	// route starts at decode_task (nothing there yet); frame_question is only
	// partial (no preregistration) so it is not Solid — imported content does
	// not skip a gate.
	if len(route) == 0 || route[0] != "decode_task" {
		t.Fatalf("route should start at decode_task, got %v", route)
	}
}

func TestIntake_ImportedDoesNotSatisfyStudentWritten(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{}
	deps := AgentDeps{Store: f}
	pid := uuid.New()
	_, err := Intake(context.Background(), deps, pid, sk, []IntakeCandidate{
		{Type: "milestone_plan", Text: "我的计划"}, // imported, not student-authored-in-tool
	})
	if err != nil {
		t.Fatalf("Intake: %v", err)
	}
	// decode_task still owes its machine items AND milestone_plan is not solid.
	g, _ := f.LoadGraph(context.Background(), pid)
	rep := CheckGate(sk, "decode_task", g, RecordedGate{})
	if rep.Solid {
		t.Fatal("imported milestone_plan must not make decode_task solid")
	}
}
```

Add fake plumbing so `LoadGraph` reflects minted nodes and captures the imported author:

```go
	minted            []GraphNodeView
	lastImportedAuthor string
```

Extend the fake `InsertGraphNode` (currently a stub) to append to `minted` + record author, and `LoadGraph` to merge `f.graph.Nodes` with `f.minted`. **Note for implementer:** `InsertGraphNode` is currently shared with Slice-3 card-effects tests that only count calls — appending to `minted` and setting `lastImportedAuthor` is additive and does not break those assertions.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run Intake -v`
Expected: FAIL — `Intake` undefined.

- [ ] **Step 3: Implement `Intake` in `planner.go`**

```go
import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"mindimprint/api/internal/skills"
)

// IntakeCandidate is one already-decomposed piece of incoming material. The
// model step that turns raw prose into candidates is out of Slice-4 scope; the
// planner mints what it is handed.
type IntakeCandidate struct {
	Type string // "claim" | "evidence" | "research_question" | ... a graph_node type
	Text string
}

// Intake maps incoming material onto the graph (author=imported), reconciles
// every gate against the reconstructed state ("owe every gate", agent-spec
// §5.2), writes the first plan artifact, and returns the initial route.
func Intake(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, candidates []IntakeCandidate) ([]string, error) {
	for _, c := range candidates {
		if _, err := deps.Store.InsertGraphNode(ctx, projectID, MintNode{
			Type:   c.Type,
			Author: "imported",
			Body:   map[string]any{"text": c.Text},
		}); err != nil {
			return nil, err
		}
	}
	route, err := writePlan(ctx, deps, projectID, sk, "intake")
	if err != nil {
		return nil, err
	}
	return route, nil
}

// writePlan reconciles gates, computes the route, upserts the plan artifact,
// and appends the plan event. Shared by Intake and Replan.
func writePlan(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, reason string) ([]string, error) {
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}
	recorded, err := deps.Store.ListGateStates(ctx, projectID)
	if err != nil {
		return nil, err
	}
	reports := ReconcileGates(sk, g, recorded)
	route := Route(sk, reports)
	body, err := json.Marshal(map[string]any{"route": route, "reason": reason})
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

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run 'Intake|Route|CheckGate' -short -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/planner.go apps/api/internal/agent/planner_test.go
git commit -m "feat(api): Intake — imported nodes onto the graph + first plan (owe every gate)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 9: `Plan`/`Replan` + `Advance`

**Files:**
- Modify: `apps/api/internal/agent/planner.go`
- Modify: `apps/api/internal/agent/planner_test.go`

**Interfaces:**
- Produces: `func Replan(ctx, deps, projectID, sk, reason string) ([]string, error)` (wraps `writePlan`), `func Advance(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, contractID string) (bool, error)`.
- `Advance` returns `(true,nil)` and writes `gate_state{confirmed_solid:true}` + a `gate_attempt` event only when the machine items pass **and** every student_written/human item is recorded solid; otherwise `(false,nil)` + a `gate_attempt` event listing what's missing. It never marks a non-machine item itself (DEC-3).

- [ ] **Step 1: Write the failing test** — append to `planner_test.go`:

```go
func TestAdvance_MachineOnlyGateAdvancesOnMachineClear(t *testing.T) {
	// A trivial one-machine-item skill: advancing needs only machine_clear.
	sk := skills.Skill{ID: "mini", Kind: "project", Contracts: map[string]skills.Contract{
		"only": {Gate: skills.Gate{Machine: []skills.MachineItem{{Kind: "node_present", Type: "x"}}}},
	}}
	f := &fakeAgentStore{graph: GraphView{Nodes: []GraphNodeView{{ID: "n", Type: "x"}}}}
	deps := AgentDeps{Store: f}
	pid := uuid.New()
	ok, err := Advance(context.Background(), deps, pid, sk, "only")
	if err != nil || !ok {
		t.Fatalf("Advance = %v, %v; want true", ok, err)
	}
	states, _ := f.ListGateStates(context.Background(), pid)
	if !states["only"].Confirmed {
		t.Fatal("machine-only gate should be confirmed solid on advance")
	}
}

func TestAdvance_RefusesWhenMachineItemMissing(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{graph: GraphView{}} // no perspectives
	deps := AgentDeps{Store: f}
	ok, err := Advance(context.Background(), deps, uuid.New(), sk, "evaluate_perspectives")
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if ok {
		t.Fatal("must refuse: machine items missing")
	}
	if f.lastEvent.Type != "gate_attempt" {
		t.Fatalf("want a gate_attempt event recording what's missing, got %q", f.lastEvent.Type)
	}
}

func TestAdvance_RefusesWhenStudentItemUnrecorded_DEC3(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// machine items pass, but student_written items unrecorded → refuse.
	f := &fakeAgentStore{graph: GraphView{Nodes: []GraphNodeView{
		{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"},
	}}}
	deps := AgentDeps{Store: f}
	ok, _ := Advance(context.Background(), deps, uuid.New(), sk, "evaluate_perspectives")
	if ok {
		t.Fatal("DEC-3: machine may not advance a gate whose student_written items are unrecorded")
	}
}

func TestAdvance_PassesWhenAllItemsSatisfied(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{
		graph:      GraphView{Nodes: []GraphNodeView{{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"}}},
		gateStates: map[string]RecordedGate{"evaluate_perspectives": {Items: map[string]string{"recon_logged": "solid", "sources_per_perspective": "solid"}}},
	}
	deps := AgentDeps{Store: f}
	ok, err := Advance(context.Background(), deps, uuid.New(), sk, "evaluate_perspectives")
	if err != nil || !ok {
		t.Fatalf("Advance = %v,%v; want true", ok, err)
	}
}
```

`Advance` reads the recorded gate for the contract via `ListGateStates` (the fake already returns its `gateStates`). **Note:** `TestAdvance_PassesWhenAllItemsSatisfied` relies on `Advance` merging the pre-recorded `Items` with a fresh `Confirmed:true` — verify the fake's `UpsertGateState` overwrites the map entry (it does).

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run Advance -v`
Expected: FAIL — `Advance` undefined.

- [ ] **Step 3: Implement `Replan` + `Advance` in `planner.go`**

```go
// Replan recomputes and rewrites the route, recording the reason as a
// plan_revised event (agent-spec §5.2).
func Replan(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, reason string) ([]string, error) {
	return writePlan(ctx, deps, projectID, sk, reason)
}

// Advance is the blocking unlock (DEC-8): it confirms a contract's gate solid
// only when every item is satisfied — machine items computed AND every
// student_written/human item recorded solid. It never marks a non-machine
// item itself (DEC-3). A machine-only gate advances on machine_clear. Records
// a gate_attempt event either way.
func Advance(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, contractID string) (bool, error) {
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return false, err
	}
	recorded, err := deps.Store.ListGateStates(ctx, projectID)
	if err != nil {
		return false, err
	}
	rec := recorded[contractID]
	report := CheckGate(sk, contractID, g, rec)

	var missing []string
	if report.Status != "machine_clear" {
		missing = append(missing, report.Missing...)
	} else {
		c := sk.Contracts[contractID]
		for _, name := range append(append([]string{}, c.Gate.StudentWritten...), c.Gate.Human...) {
			if rec.Items[name] != "solid" {
				missing = append(missing, name+" 待完成")
			}
		}
	}

	if len(missing) > 0 {
		payload, _ := json.Marshal(map[string]any{"contract": contractID, "result": "blocked", "missing": missing})
		if err := deps.Store.AppendEvent(ctx, EventRow{ProjectID: projectID, Surface: "studio", Type: "gate_attempt", Payload: payload}); err != nil {
			return false, err
		}
		return false, nil
	}

	rec.Confirmed = true
	if err := deps.Store.UpsertGateState(ctx, projectID, contractID, rec); err != nil {
		return false, err
	}
	payload, _ := json.Marshal(map[string]any{"contract": contractID, "result": "passed"})
	if err := deps.Store.AppendEvent(ctx, EventRow{ProjectID: projectID, Surface: "studio", Type: "gate_attempt", Payload: payload}); err != nil {
		return false, err
	}
	return true, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run 'Advance|Intake|Route|CheckGate' -short -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/planner.go apps/api/internal/agent/planner_test.go
git commit -m "feat(api): Replan + Advance — DEC-8 blocking unlock, DEC-3 preserved

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 10: Wire `check_gate` into the loop + §5.7 second-skill acceptance

**Files:**
- Modify: `apps/api/internal/agent/classifier.go` (add `CheckGateCandidates`)
- Modify: `apps/api/internal/agent/loop.go` (`RunAgentStep` handles a `check_gate` candidate; ordering)
- Modify: `apps/api/internal/agent/runtime.go` (`Action.Kind` gains `"check_gate"`; carry the report)
- Modify: `apps/api/internal/agent/planner_test.go` (§5.7 test) + `loop_test.go` (wiring test)

**Interfaces:**
- Produces: `func CheckGateCandidates(sk skills.Skill, route []string, reports map[string]GateReport) []Candidate` — for the first routed contract not `machine_clear`, one `check_gate` candidate (`Verb:"check_gate"`, `AnchorKind:"contract"`, `AnchorID:contractID`, `Level:"I1"`).
- `RunAgentStep` gains a `deps.Skill skills.Skill` + `deps.Route`/reports path. **Decision:** to avoid overloading `RunAgentStep`'s Slice-2 signature and its many existing tests, add the skill to `AgentDeps` as an optional field `Skill *skills.Skill`; when nil (Slice-2/3 callers), the check_gate path is skipped entirely. When set, after `SurfaceCardCandidates` and before `CandidateMoves`, append `CheckGateCandidates`. Ordering: `surface_card` > `check_gate` > `post_intervention`.
- `Action{Kind:"check_gate", GateReport *GateReport}` (or carry `Criterion`/`Missing`); no model call, no enforcement.

- [ ] **Step 1: Write the failing tests**

`planner_test.go` — §5.7 acceptance (a second skill, zero new runtime code):

```go
func TestSecondSkill_ReconcileRouteAdvanceWithZeroNewRuntimeCode(t *testing.T) {
	// A bare "note-to-self" project skill: one contract, one machine item, no
	// cards — authored inline, never embedded — runs through the exact same
	// ReconcileGates / Route / Advance the writing-project skill uses.
	sk := skills.Skill{ID: "note-to-self", Kind: "project", Contracts: map[string]skills.Contract{
		"jot": {Gate: skills.Gate{Machine: []skills.MachineItem{{Kind: "node_present", Type: "note"}}}},
	}}
	if err := sk.Validate(); err != nil {
		t.Fatalf("inline skill invalid: %v", err)
	}
	f := &fakeAgentStore{}
	deps := AgentDeps{Store: f}
	pid := uuid.New()

	route, err := Intake(context.Background(), deps, pid, sk, []IntakeCandidate{{Type: "note", Text: "记一笔"}})
	if err != nil {
		t.Fatalf("Intake: %v", err)
	}
	if len(route) != 1 || route[0] != "jot" {
		t.Fatalf("route = %v, want [jot]", route)
	}
	ok, err := Advance(context.Background(), deps, pid, sk, "jot")
	if err != nil || !ok {
		t.Fatalf("Advance = %v,%v; want true (machine-only gate, node present)", ok, err)
	}
}
```

`loop_test.go` — wiring:

```go
func TestLoop_CheckGateCandidateEmitsReportNoModelCall(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	g := GraphView{} // decode_task empty → its gate is not machine_clear
	f := &fakeAgentStore{graph: g}
	deps := AgentDeps{Store: f, Skill: &sk}
	action, err := RunAgentStep(context.Background(), deps, uuid.New(), Trigger{Kind: "T-B"})
	if err != nil {
		t.Fatalf("RunAgentStep: %v", err)
	}
	if action == nil || action.Kind != "check_gate" {
		t.Fatalf("want a check_gate action, got %+v", action)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && go test ./internal/agent/ -run 'SecondSkill|CheckGateCandidate' -v`
Expected: FAIL — `Skill` field / `CheckGateCandidates` / `check_gate` handling undefined.

- [ ] **Step 3: Add `CheckGateCandidates` to `classifier.go`**

```go
// CheckGateCandidates proposes a structural check_gate for the contract the
// route currently points at, when that contract's gate is not yet
// machine_clear. It is a no-model action (like surface_card): the loop runs
// CheckGate and records the report. At most one candidate.
func CheckGateCandidates(sk skills.Skill, route []string, reports map[string]GateReport) []Candidate {
	for _, id := range route {
		if reports[id].Status != "machine_clear" {
			return []Candidate{{
				Verb:       "check_gate",
				AnchorKind: "contract",
				AnchorID:   id,
				Level:      "I1",
				Reason:     "gate not yet machine-clear",
			}}
		}
	}
	return nil
}
```

- [ ] **Step 4: Extend `AgentDeps` + `Action` + `RunAgentStep`**

In `loop.go`, add to `AgentDeps`: `Skill *skills.Skill`. In `runtime.go`, add to `Action`: `GateReport *GateReport` and document `Kind:"check_gate"`. In `RunAgentStep`, after building `cands` from `SurfaceCardCandidates(g)` and before `CandidateMoves(g)`:

```go
	cands := SurfaceCardCandidates(g)
	if deps.Skill != nil {
		recorded, err := deps.Store.ListGateStates(ctx, projectID)
		if err != nil {
			return nil, err
		}
		reports := ReconcileGates(*deps.Skill, g, recorded)
		route := Route(*deps.Skill, reports)
		cands = append(cands, CheckGateCandidates(*deps.Skill, route, reports)...)
	}
	cands = append(cands, CandidateMoves(g)...)
```

Then handle the new verb before the `post_intervention` path:

```go
	if c.Verb == "check_gate" {
		recorded, err := deps.Store.ListGateStates(ctx, projectID)
		if err != nil {
			return nil, err
		}
		report := CheckGate(*deps.Skill, c.AnchorID, g, recorded[c.AnchorID])
		payload, err := json.Marshal(map[string]any{"contract": c.AnchorID, "status": report.Status, "missing": report.Missing})
		if err != nil {
			return nil, err
		}
		if err := deps.Store.AppendEvent(ctx, EventRow{ProjectID: projectID, Surface: "studio", Type: "gate_checked", Payload: payload}); err != nil {
			return nil, err
		}
		return &Action{Kind: "check_gate", GateReport: &report}, nil
	}
```

**Ordering note:** `SurfaceCardCandidates` is prepended, `check_gate` next, `CandidateMoves`/observe last — so `surface_card` > `check_gate` > `post_intervention`, matching the spec. Add `"mindimprint/api/internal/skills"` to `loop.go` imports.

- [ ] **Step 5: Run the tests**

Run: `cd apps/api && go test ./internal/agent/ -run 'SecondSkill|CheckGateCandidate|Loop' -short -v`
Expected: PASS (existing `Loop` tests unaffected — `deps.Skill` is nil for them).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/agent/classifier.go apps/api/internal/agent/loop.go apps/api/internal/agent/runtime.go apps/api/internal/agent/planner_test.go apps/api/internal/agent/loop_test.go
git commit -m "feat(api): wire check_gate into the loop (no-model) + §5.7 second-skill acceptance

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 11: Testcontainers integration — intake → reconcile → route → advance round-trip

**Files:**
- Create: `apps/api/internal/agent/refactor2_slice4_sqlc_test.go`

**Interfaces:**
- Consumes: the same testcontainers harness the Slice-2/3 sqlc tests use (`refactor2_cards_loop_sqlc_test.go` / `loop_sqlc_test.go`) — reuse their Postgres bring-up helper + a seeded project.

- [ ] **Step 1: Read the existing sqlc test harness**

Read `apps/api/internal/agent/refactor2_cards_loop_sqlc_test.go` and `loop_sqlc_test.go` to reuse the container/pool/`sqlc.New` + project-seed helpers (do not invent a second harness). Match their build tag / `-short` skip convention exactly.

- [ ] **Step 2: Write the integration test** (against real Postgres)

```go
func TestSlice4_IntakeReconcileRouteAdvance_Postgres(t *testing.T) {
	if testing.Short() {
		t.Skip("skip: testcontainers")
	}
	// <bring up pg + run migrations + seed a project> via the shared helper.
	// store := NewSqlcAgentStore(q); deps := AgentDeps{Store: store}
	sk, _ := skills.ByID("writing-project")

	// Intake mints imported nodes + writes the plan node.
	route, err := Intake(ctx, deps, projectID, sk, []IntakeCandidate{
		{Type: "rubric_translation", Text: "把评分表翻译成人话"},
		{Type: "weakness_prediction", Text: "Table B"},
		{Type: "weakness_prediction", Text: "Table D"},
	})
	if err != nil { t.Fatal(err) }
	if len(route) == 0 || route[0] != "decode_task" {
		t.Fatalf("route = %v", route)
	}

	// The plan node persisted (one per project).
	plan, err := q.GetPlanNode(ctx, projectID)
	if err != nil { t.Fatalf("GetPlanNode: %v", err) }
	if plan.Type != "plan" { t.Fatalf("plan node type = %q", plan.Type) }

	// decode_task machine items now pass (rubric_translation + 2 weakness_prediction);
	// record its student item + advance → gate_state persisted, confirmed.
	if err := store.UpsertGateState(ctx, projectID, "decode_task",
		RecordedGate{Items: map[string]string{"milestone_plan": "solid"}}); err != nil { t.Fatal(err) }
	ok, err := Advance(ctx, deps, projectID, sk, "decode_task")
	if err != nil || !ok { t.Fatalf("Advance = %v,%v", ok, err) }

	states, err := store.ListGateStates(ctx, projectID)
	if err != nil { t.Fatal(err) }
	if !states["decode_task"].Confirmed {
		t.Fatal("decode_task should be confirmed solid after advance")
	}

	// Re-plan: frame_question is now the frontier.
	route2, err := Replan(ctx, deps, projectID, sk, "advanced decode_task")
	if err != nil { t.Fatal(err) }
	if len(route2) == 0 || route2[0] != "frame_question" {
		t.Fatalf("route2 = %v, want frame_question frontier", route2)
	}
}
```

Fill the `<bring up …>` block from the harness read in Step 1. Assert one `gate_state` row per (project, contract) after two `UpsertGateState` calls on the same contract (idempotent upsert).

- [ ] **Step 3: Run the integration test**

Run: `cd apps/api && go test ./internal/agent/ -run Slice4_IntakeReconcileRouteAdvance -v`
Expected: PASS (Docker/testcontainers required).

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/agent/refactor2_slice4_sqlc_test.go
git commit -m "test(api): Slice 4 testcontainers — intake/reconcile/route/advance round-trip

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Final gate (before the whole-branch review)

- [ ] `cd apps/api && go build ./... && go vet ./...` — clean.
- [ ] `cd apps/api && go test ./... -short` — all unit tests green (skills + agent + the rest).
- [ ] `cd apps/api && go test ./internal/agent/ -run 'Slice4|Refactor2|Loop|Card'` — testcontainers green (Docker up).
- [ ] `cd apps/api && make sync-skills` is idempotent (no diff after a second run).
- [ ] Legacy paths untouched: `turn.go`/`prompt.go`/`refeed.go` and the migrations directory unchanged.

## Self-review notes (plan author)

- **Spec coverage:** skill loader+validation (T1) · writing-project JSON (T3) · gate engine + DEC-3 (T4,T5) · intake/reconcile (T6,T8) · route/replan/advance (T6,T9) · check_gate wiring (T10) · §5.7 acceptance (T10) · no-migration storage (T7) · testcontainers (T11). All spec §0–§11 items map to a task.
- **Type consistency:** `RecordedGate`/`GateReport`/`Candidate.Verb="check_gate"`/`Action.Kind="check_gate"`/`AgentDeps.Skill *skills.Skill` are used identically across T5–T11.
- **Known approximation (documented):** `no_single_sourced_claim` is "≥2 distinct supporting evidence nodes" in Slice 4, refined to distinct source *materials* in Slice 7 when the graph primitive + intake-minted materials land (spec §10 open question, carried forward).
- **sqlc field-name caveat (T7):** the generated param name for `body->>'contract' = $2` must be confirmed against the regenerated `graph.sql.go` before writing the adapter — do not assume `Body`.

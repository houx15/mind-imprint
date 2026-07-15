# Slice 7b — Structure Projection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Project the completed Toulmin argument back into the S4 结构 pane — after the student locks the card, show the five role cards with her sentences and the green gate banner.

**Architecture:** One new server derivation (`projectStructure`) reads the Toulmin card spec's five slots for order/labels and the minted graph nodes for status/preview, adds a `structure` field to `StudioProjection` (Go DTO + Zod contract), and the frontend maps it into `views.structure` — which `StructureView` already renders. The now-unreachable inline-edit branch of `RoleCard` is deleted.

**Tech Stack:** Go (`net/http`, `sqlc`, testcontainers), TypeScript + React + vitest, Zod contracts.

**Spec:** `docs/superpowers/specs/2026-07-15-slice-7b-structure-projection-design.md`

## Global Constraints

- Derive-never-decorate: a `done` card exists ONLY because a Toulmin slot node was minted; its preview is the student's `body.text` read back verbatim. Never invent a status, sentence, or ordering.
- Discriminate Toulmin slot nodes by `type == slot.id && body.text != ""` — CRAAP's `evidence` node (`body.source_quality`, no `text`) must never be mistaken for the evidence slot.
- Slot order + role labels come from the `toulmin` card spec (`spec.Params.Slots`) via the `specByID` the projection already receives — never a second hardcoded list.
- Pre-mint: `structure` is an empty slice, so `StructureView`'s `cards.length === 0` branch keeps today's placeholder. No empty five-card skeleton up front.
- Read-only: no re-open / re-edit affordance for a locked argument.
- The wire DTO JSON shape (Go `dto.go`) must match the contract (`packages/contracts/src/studioState.ts`) exactly — the parity test guards this.
- Go tests run serialized on a quiet Docker: `CGO_ENABLED=0 go test -p 1 ./...`. Card/gate/projection changes run the FULL package, never a `-run` subset.

---

### Task 1: Backend `projectStructure` + DTO + wiring

**Files:**
- Modify: `apps/api/internal/studio/dto.go` (add `StructureCardDTO`, add `Structure` field to `StudioProjection`)
- Modify: `apps/api/internal/studio/projection.go` (add `projectStructure`, wire into `Project`)
- Test: `apps/api/internal/studio/projection_test.go` (add `TestProjectStructure`)
- Test: `apps/api/internal/studio/dto_parity_test.go` (add `structure` to key sets)

**Interfaces:**
- Consumes: `cards.ByID(id string) (cards.Spec, bool)` — the loader; `cards.Spec.Params.Slots []cards.Slot` where `type Slot struct { ID, Role string; NeedSrc bool; Q string }`. `ProjectData.Nodes []sqlc.GraphNode` where a node has `Type string` and `Body []byte`.
- Produces: `func projectStructure(specByID func(string) (cards.Spec, bool), d ProjectData) []StructureCardDTO`; `type StructureCardDTO struct { ID, Role, Status, Preview string }` with JSON keys `id`/`role`/`status`/`preview`; `StudioProjection.Structure []StructureCardDTO json:"structure"`.

- [ ] **Step 1: Write the failing test**

Add to `apps/api/internal/studio/projection_test.go`:

```go
func TestProjectStructure(t *testing.T) {
	textNode := func(typ, text string) sqlc.GraphNode {
		return sqlc.GraphNode{Type: typ, Body: []byte(`{"text":"` + text + `"}`)}
	}
	order := []string{"claim", "warrant", "evidence", "counter", "concession"}

	// (a) no Toulmin nodes -> empty slice (pane keeps its placeholder).
	if got := projectStructure(cards.ByID, ProjectData{}); len(got) != 0 {
		t.Fatalf("no nodes: want empty, got %d cards", len(got))
	}

	// (b) all five slot nodes minted -> five done cards in spec-slot order.
	d := ProjectData{Nodes: []sqlc.GraphNode{
		textNode("claim", "主张句"), textNode("warrant", "理据句"),
		textNode("evidence", "证据句"), textNode("counter", "反方句"),
		textNode("concession", "让步句"),
	}}
	got := projectStructure(cards.ByID, d)
	if len(got) != 5 {
		t.Fatalf("five nodes: want 5 cards, got %d", len(got))
	}
	previews := map[string]string{"claim": "主张句", "warrant": "理据句", "evidence": "证据句", "counter": "反方句", "concession": "让步句"}
	for i, c := range got {
		if c.ID != order[i] {
			t.Fatalf("card %d id = %q, want %q (slot order)", i, c.ID, order[i])
		}
		if c.Status != "done" {
			t.Fatalf("card %s status = %q, want done", c.ID, c.Status)
		}
		if c.Preview != previews[c.ID] {
			t.Fatalf("card %s preview = %q, want %q", c.ID, c.Preview, previews[c.ID])
		}
		if c.Role == "" {
			t.Fatalf("card %s has empty role label", c.ID)
		}
	}

	// (c) a CRAAP evidence node (source_quality, no text) with no Toulmin
	// nodes -> still empty (the evidence slot is NOT falsely done).
	craap := ProjectData{Nodes: []sqlc.GraphNode{{Type: "evidence", Body: []byte(`{"source_quality":{"risk_note":"x"}}`)}}}
	if got := projectStructure(cards.ByID, craap); len(got) != 0 {
		t.Fatalf("craap-only: want empty, got %d cards", len(got))
	}

	// (d) claim + evidence minted, rest absent -> those two done, rest empty,
	// list present.
	partial := ProjectData{Nodes: []sqlc.GraphNode{textNode("claim", "主张句"), textNode("evidence", "证据句")}}
	got = projectStructure(cards.ByID, partial)
	if len(got) != 5 {
		t.Fatalf("partial: want 5 cards, got %d", len(got))
	}
	doneSet := map[string]bool{"claim": true, "evidence": true}
	for _, c := range got {
		wantDone := doneSet[c.ID]
		if wantDone && c.Status != "done" {
			t.Fatalf("partial: card %s status = %q, want done", c.ID, c.Status)
		}
		if !wantDone && c.Status != "empty" {
			t.Fatalf("partial: card %s status = %q, want empty", c.ID, c.Status)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/ -run TestProjectStructure`
Expected: FAIL — `undefined: projectStructure` (and `StructureCardDTO`).

- [ ] **Step 3: Add the DTO**

In `apps/api/internal/studio/dto.go`, add the struct (after `ActiveCardDTO`):

```go
// StructureCardDTO is one role in the S4 argument (论证构建). status/preview
// are DERIVED from the Toulmin mint: a card is "done" (with the student's own
// sentence as preview) only because a graph node typed for its slot carries a
// body.text she wrote; otherwise "empty". Nothing here invents an argument —
// same derive-never-decorate contract as MaterialDTO. The list is present only
// once the argument exists in the graph; before that it is empty and the 结构
// pane keeps its placeholder.
type StructureCardDTO struct {
	ID      string `json:"id"`      // slot id: claim/warrant/evidence/counter/concession
	Role    string `json:"role"`    // 核心主张 / 理据 · 推理 / 支撑证据 / 反方 · 钢人 / 让步 · 转折
	Status  string `json:"status"`  // "done" | "empty"
	Preview string `json:"preview"` // the student's sentence (body.text); "" when empty
}
```

And add the field to `StudioProjection` (after `ActiveCard *ActiveCardDTO`):

```go
	// Structure projects the five Toulmin argument slots (S4 论证构建) once the
	// argument has been minted; empty until then so the pane keeps its
	// placeholder. See projectStructure (projection.go).
	Structure []StructureCardDTO `json:"structure"`
```

- [ ] **Step 4: Add `projectStructure` and wire it in**

In `apps/api/internal/studio/projection.go`, add (near `projectMaterials`):

```go
// projectStructure projects the five Toulmin argument slots into 论证构建 role
// cards. Slot order + role labels come from the toulmin card spec (single
// source of truth); status/preview come from the minted graph nodes. A slot is
// "done" only when a node typed for it carries a body.text the student wrote —
// which excludes CRAAP's source_quality evidence node (no text key), so the
// evidence slot is never falsely done. Returns an empty slice until at least
// one slot node exists, so the 结构 pane keeps its placeholder before the
// argument is built.
func projectStructure(specByID func(string) (cards.Spec, bool), d ProjectData) []StructureCardDTO {
	spec, ok := specByID("toulmin")
	if !ok || len(spec.Params.Slots) == 0 {
		return []StructureCardDTO{}
	}
	text := map[string]string{}
	for _, n := range d.Nodes {
		if _, seen := text[n.Type]; seen {
			continue
		}
		var b struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(n.Body, &b) == nil && strings.TrimSpace(b.Text) != "" {
			text[n.Type] = b.Text
		}
	}
	any := false
	out := make([]StructureCardDTO, 0, len(spec.Params.Slots))
	for _, slot := range spec.Params.Slots {
		card := StructureCardDTO{ID: slot.ID, Role: slot.Role, Status: "empty"}
		if t, ok := text[slot.ID]; ok {
			card.Status, card.Preview = "done", t
			any = true
		}
		out = append(out, card)
	}
	if !any {
		return []StructureCardDTO{}
	}
	return out
}
```

Add `"strings"` to the imports if not already present (it is not — add it). Then wire it into the `Project` return literal (after `ActiveCard: ...`):

```go
		ActiveCard:    projectActiveCard(d, materialByCardInstance(d)),
		Structure:     projectStructure(specByID, d),
	}, nil
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/ -run TestProjectStructure`
Expected: PASS.

- [ ] **Step 6: Update the parity test**

In `apps/api/internal/studio/dto_parity_test.go`, the `TestStudioProjectionJSONKeys` top-level `want` list must include `structure` (kept sorted). Change:

```go
	want := []string{"activeCard", "activeStation", "coach", "materials", "onboarding", "project", "stations"}
```

to:

```go
	want := []string{"activeCard", "activeStation", "coach", "materials", "onboarding", "project", "stations", "structure"}
```

The test constructs a `StudioProjection` literal near the top of the function; add a `Structure` field to it so the key is present when marshalled:

```go
		ActiveCard: &ActiveCardDTO{
			CardInstanceID: "ci1", CardID: "sift", Status: "active",
			Anchors: []json.RawMessage{json.RawMessage(`{"id":"a1"}`)}, MaterialID: "m1",
		},
		Structure: []StructureCardDTO{{ID: "claim", Role: "核心主张", Status: "done", Preview: "主张句"}},
```

Then, after the existing `materials[0]` key assertion, add an assertion of the structure card's key set. Insert after the `assertKeys(t, materials[0], ...)` block:

```go
	var structure []json.RawMessage
	if err := json.Unmarshal(mTop["structure"], &structure); err != nil {
		t.Fatal(err)
	}
	if len(structure) != 1 {
		t.Fatalf("structure len = %d, want 1", len(structure))
	}
	// structure card: {id,role,status,preview} — must match
	// packages/contracts/src/studioState.ts StructureCard exactly.
	assertKeys(t, structure[0], []string{"id", "preview", "role", "status"})
```

(`mTop` is the `map[string]json.RawMessage` the test already unmarshals the top-level object into. If the block that unmarshals `mTop["materials"]` uses a different local variable name for the top map, reuse that same name.)

- [ ] **Step 7: Run the studio package in full**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/studio/`
Expected: PASS (unit tests; the testcontainers roundtrip test skips under no-Docker with `testing.Short()` only if `-short`; run without `-short` if Docker is up, otherwise the container tests run — either way the non-DB tests must pass).

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/studio/dto.go apps/api/internal/studio/projection.go apps/api/internal/studio/projection_test.go apps/api/internal/studio/dto_parity_test.go
git commit -m "feat(refactor2): project the completed Toulmin argument into the 结构 DTO"
```

---

### Task 2: Real-Postgres e2e — projection over a genuine mint

**Files:**
- Modify: `apps/api/internal/agent/refactor2_cards_loop_sqlc_test.go` (extend `TestRefactor2CardsLoop_ToulminBuildsArgument`)

**Interfaces:**
- Consumes: `studio.Load(ctx, q, projectID) (studio.ProjectData, error)`, `studio.Project(sk skills.Skill, specByID, d) (studio.StudioProjection, error)`, `cards.ByID`, `skills.ByID("writing-project")`. The test is `package agent_test` (external), so importing `mindimprint/api/internal/studio` introduces no cycle.
- Produces: nothing consumed downstream — this is a proof.

This test already drives the real summon→submit→mint→gate loop and, at its end, asserts `build_argument`'s machine tier clears. Add a projection assertion after that: load + project the same real rows and confirm the five `done` structure cards carry the student's sentences.

- [ ] **Step 1: Add the studio import**

In the `import (...)` block of `apps/api/internal/agent/refactor2_cards_loop_sqlc_test.go`, add:

```go
	"mindimprint/api/internal/studio"
```

- [ ] **Step 2: Write the failing assertion**

At the END of `TestRefactor2CardsLoop_ToulminBuildsArgument` (after the existing
`if after.Status != "machine_clear"` block), add:

```go
	// Slice 7b: the same minted rows must project into five done structure
	// cards — the completed argument shown back in the 结构 pane. Proves the
	// projection over a genuine mint, not hand-built nodes.
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("skills.ByID(writing-project) not found")
	}
	pd, err := studio.Load(ctx, q, project.ID)
	if err != nil {
		t.Fatalf("studio.Load: %v", err)
	}
	proj, err := studio.Project(sk, cards.ByID, pd)
	if err != nil {
		t.Fatalf("studio.Project: %v", err)
	}
	if len(proj.Structure) != 5 {
		t.Fatalf("projected structure = %d cards, want 5", len(proj.Structure))
	}
	for _, c := range proj.Structure {
		if c.Status != "done" {
			t.Fatalf("structure card %s = %q, want done (argument fully minted)", c.ID, c.Status)
		}
		if c.Preview == "" {
			t.Fatalf("structure card %s has empty preview", c.ID)
		}
	}
```

If `skills` is not already imported in this file, add `"mindimprint/api/internal/skills"` to the import block. (It is already imported — the file's header lists it — so only add if the compiler reports it missing.) `ctx`, `q`, and `project` are already in scope from earlier in the test.

- [ ] **Step 3: Run the test to verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/ -run TestRefactor2CardsLoop_ToulminBuildsArgument`
Expected: PASS (requires Docker/testcontainers). If Docker is down the test skips — in that case note it and run it once Docker is available; do NOT mark the task done on a skip.

- [ ] **Step 4: Run the full agent package**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/refactor2_cards_loop_sqlc_test.go
git commit -m "test(refactor2): assert the minted Toulmin argument projects into 5 done structure cards"
```

---

### Task 3: Contracts — `StructureCard` + `StudioProjection.structure`

**Files:**
- Modify: `packages/contracts/src/studioState.ts` (add `StructureCard`, add `structure` to `StudioProjection`)
- Test: `packages/contracts/test/studioState.test.ts` (create if absent; else add cases)

**Interfaces:**
- Consumes: `z` from zod (already imported in the file).
- Produces: `export const StructureCard` / `export type StructureCard`; `StudioProjection.structure: z.array(StructureCard)`.

- [ ] **Step 1: Write the failing test**

Check whether `packages/contracts/test/studioState.test.ts` exists. If it does, add the cases below to it; if not, create it with:

```ts
import { describe, it, expect } from "vitest";
import { StructureCard, StudioProjection } from "../src/studioState";

describe("StructureCard", () => {
  it("parses a valid done card", () => {
    const card = { id: "claim", role: "核心主张", status: "done", preview: "主张句" };
    expect(StructureCard.parse(card)).toEqual(card);
  });
  it("rejects an unknown status", () => {
    expect(() => StructureCard.parse({ id: "claim", role: "核心主张", status: "active", preview: "" })).toThrow();
  });
});

describe("StudioProjection", () => {
  it("requires a structure array", () => {
    const base = {
      project: { title: "t", qualLabel: "0457 个人报告" },
      stations: [],
      activeStation: "S4",
      coach: { anchor: "a", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [] },
      materials: [],
      activeCard: null,
    };
    expect(() => StudioProjection.parse(base)).toThrow(); // missing structure
    expect(StudioProjection.parse({ ...base, structure: [] }).structure).toEqual([]);
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd packages/contracts && npx vitest run test/studioState.test.ts`
Expected: FAIL — `StructureCard` is not exported / `structure` not required.

- [ ] **Step 3: Add the schema**

In `packages/contracts/src/studioState.ts`, add before `export const StudioProjection`:

```ts
// StructureCard is one role in the S4 argument (论证构建), projected once the
// Toulmin card is locked. status/preview are derived from the minted graph
// nodes server-side (studio.projectStructure) — never invented. The array is
// empty until the argument exists, so the 结构 pane keeps its placeholder.
export const StructureCard = z.object({
  id: z.string(),
  role: z.string(),
  status: z.enum(["done", "empty"]),
  preview: z.string(),
});
export type StructureCard = z.infer<typeof StructureCard>;
```

And add the field inside `StudioProjection` (after `activeCard: ActiveCard.nullable(),`):

```ts
  structure: z.array(StructureCard),
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd packages/contracts && npx vitest run test/studioState.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/studioState.ts packages/contracts/test/studioState.test.ts
git commit -m "feat(refactor2): add StructureCard + structure field to the StudioProjection contract"
```

---

### Task 4: Frontend — wire `structure` + remove the dead inline-edit branch

**Files:**
- Modify: `apps/web/src/studio/state.ts` (align `StructureCardFx` to the contract type)
- Modify: `apps/web/src/studio/StudioContainer.tsx` (`toStudioState`: `structure: p.structure`)
- Modify: `apps/web/src/studio/views/StructureView.tsx` (delete the `active` branch of `RoleCard`)
- Modify: `apps/web/src/studio/fixtures.ts` (the `evidence` card: `active` → `done`)
- Test: `apps/web/src/studio/views/StructureView.test.tsx` (drop the two active-branch cases)

**Interfaces:**
- Consumes: `StructureCard` from `@mind-imprint/contracts` (Task 3); `StudioProjection.structure` (Task 3).
- Produces: `StudioState.views.structure: StructureCardFx[]` where `StructureCardFx = StructureCard`.

- [ ] **Step 1: Align the frontend type**

In `apps/web/src/studio/state.ts`, replace the local `StructureCardFx` definition:

```ts
export type StructureCardFx = {
  id: string;
  role: string;         // 核心主张 / 理据·推理 / 支撑证据 / 反方·钢人 / 让步·转折
  status: "done" | "active" | "empty";
  preview?: string;     // collapsed text when done
  question?: string;    // AI question when active
};
```

with a re-export of the contract type (add `StructureCard` to the existing `import type { ... } from "@mind-imprint/contracts"` list at the top of the file, then):

```ts
// The five S4 argument role cards are the wire StructureCard verbatim — one
// shape across the boundary. status is only "done" | "empty"; the live
// inline-edit state the old "active" value modeled is now the full-pane
// StudioToulminCard, not a row in this list.
export type StructureCardFx = StructureCard;
```

- [ ] **Step 2: Run the type-check / tests to see what breaks**

Run: `cd apps/web && npx tsc --noEmit`
Expected: FAIL — `fixtures.ts` sets `status: "active"` and a `question` field, neither valid on the new type; `StructureView.tsx` reads `card.status === "active"` and `card.question`.

- [ ] **Step 3: Fix the fixture**

In `apps/web/src/studio/fixtures.ts`, change the `evidence` structure card from the active state to a done state:

```ts
      {
        id: "evidence",
        role: "支撑证据",
        status: "done",
        preview:
          "Chen 等（Nature Sustainability, 2019）用卫星数据证实，2000 年以来全球新增绿叶面积中约四分之一来自中国，主要由农业集约化与植树造林驱动。",
      },
```

- [ ] **Step 4: Remove the dead `active` branch in `RoleCard`**

In `apps/web/src/studio/views/StructureView.tsx`, inside `RoleCard`:

- Delete the `const active = card.status === "active";` line.
- In `statusLabel`/`statusColor`/`statusBg`, drop the `active` arm: `const statusLabel = done ? "已完成" : "待开始";` and likewise `statusColor = done ? "#4C9A82" : "#AEB4C2";`, `statusBg = done ? "#E7F3EE" : "#F1F2F5";`.
- In the card `<div>` style, replace `border: active ? "1px solid #D7DCF3" : "1px solid #ECEEF3"` with `border: "1px solid #ECEEF3"`.
- Delete the entire `{active && ( <> ... </> )}` block (the Bean question bubble, the `① 选择相关素材` label, the disabled 素材选择器 placeholder, the `② 基于素材` label, and the disabled `<textarea>`).
- Keep the `{done && card.preview && (...)}` and `{empty && (...)}` blocks unchanged.
- Remove the now-unused `Bean` import if nothing else in the file uses it (grep the file first; `StudioToulminCard` has its own imports — check `StructureView.tsx` only).

Then in `StructureView`, change `allClean`:

```ts
  const allClean = cards.every((c) => c.status === "done");
```

- [ ] **Step 5: Wire the projection**

In `apps/web/src/studio/StudioContainer.tsx`, in `toStudioState`, change `structure: []` to `structure: p.structure`, and update the stale comment above the function so it no longer lists `structure` among the deferred views:

```ts
// Map the lean wire projection into the frontend view-model, stubbing the
// deferred center-pane views (writing/review → Slices 8/9). material (6b) and
// structure (7b) are live — projected straight from the server.
function toStudioState(p: StudioProjection): StudioState {
  return {
    project: p.project,
    stations: p.stations,
    activeStation: p.activeStation,
    focusMode: false,
    coach: p.coach,
    views: {
      material: p.materials,
      structure: p.structure,
      writing: { draft: "", mode: "edit" },
      review: [],
      onboarding: p.onboarding,
    },
  };
}
```

- [ ] **Step 6: Fix the test file**

In `apps/web/src/studio/views/StructureView.test.tsx`:

- Delete the test `"shows the collapsed preview for done cards and the AI question bubble for the active card"` (lines 30–36) — the active bubble no longer exists.
- Delete the test `"shows the disabled 选择相关素材/写成句子 hint on the active card"` (lines 38–42) — that hint was the dead branch.
- Add a focused done-preview test in their place:

```ts
  it("shows the collapsed preview sentence for done cards", () => {
    render(<StructureView cards={STUDIO_FIXTURE.views.structure} />);
    const doneCard = STUDIO_FIXTURE.views.structure.find((c) => c.status === "done")!;
    expect(screen.getByText(doneCard.preview!)).toBeInTheDocument();
  });
```

- The remaining tests (`renders the gate banner and the 5 role cards`, `shows the orange not-clean banner when a card is empty`, `shows the green all-clean banner when no card is empty`, `renders the deferred shell placeholder ... when cards is empty`) stay as-is; they pass with the fixture now holding done+empty statuses (the fixture still has `warrant`/`concession` empty, so the orange banner case holds).

- [ ] **Step 7: Run type-check + the frontend tests**

Run: `cd apps/web && npx tsc --noEmit && npx vitest run src/studio/views/StructureView.test.tsx src/studio/StudioContainer.test.tsx`
Expected: PASS. (If `StudioContainer.test.tsx` does not exist, run just the StructureView test; a broad `npx vitest run` is fine too.)

- [ ] **Step 8: Run the full web test suite**

Run: `cd apps/web && npx vitest run`
Expected: PASS — confirms no other consumer of `StructureCardFx`/`views.structure` broke.

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/studio/state.ts apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/views/StructureView.tsx apps/web/src/studio/views/StructureView.test.tsx apps/web/src/studio/fixtures.ts
git commit -m "feat(refactor2): render the projected Toulmin argument in the 结构 pane; drop the dead inline-edit branch"
```

---

## Notes for the executor

- **Full-package runs, not `-run` subsets, for the final gate.** Slice 7's two hidden regressions came from `-run`-scoped runs. Before finishing, run `cd apps/api && CGO_ENABLED=0 go test -p 1 ./...` and `cd apps/web && npx vitest run` and `cd packages/contracts && npx vitest run`.
- **No card/skill JSON changes** in this slice, so `make sync-cards` / `make sync-skills` are not needed. If a task unexpectedly edits a `packages/contracts/cards/*.json` or `skills/*.json`, re-sync and re-run the mirror guard tests.
- **Docker for Tasks 1 (roundtrip) & 2.** The studio roundtrip and the agent e2e need testcontainers. A skip is not a pass — surface it and complete the run when Docker is up.

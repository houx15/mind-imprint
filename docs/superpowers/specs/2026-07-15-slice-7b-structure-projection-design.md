# Slice 7b — Project the completed argument back into the 结构 view

**Status:** design approved (2026-07-15)
**Carry-forward from:** Slice 7 (`docs/superpowers/specs/2026-07-15-slice-7-structure-graph-design.md`)
**Roadmap:** `docs/2026-07-11-whole-product-refactor-roadmap.md`

## 1. Problem

Slice 7 shipped the `graph` primitive, the Toulmin card, and the live S4 结构
builder. It left one honest gap, sanctioned by the whole-branch review as a
follow-up: **after the student locks the Toulmin card and the mint writes the
five argument nodes, the 结构 pane does not show the built argument.** It falls
back to the centered placeholder.

The completed-state UI already exists and is dead code:

- `apps/web/src/studio/views/StructureView.tsx` already renders the five
  `RoleCard`s (a `done` card shows its preview sentence; an `empty` card shows
  待开始) and the `GateBanner` (green when every card is non-empty).
- `apps/web/src/studio/ViewFrame.tsx:275` already passes `state.views.structure`
  into `StructureView`.

It is dead because nothing ever fills `views.structure`:

- `StudioProjection` — both `apps/api/internal/studio/dto.go` and its contract
  mirror `packages/contracts/src/studioState.ts` — has **no `structure` field**.
- `apps/web/src/studio/StudioContainer.tsx` `toStudioState` hard-stubs
  `structure: []`.

So the fix is a single new backend derivation + one wiring line + targeted
cleanup of the now-unreachable inline-edit branch.

## 2. Locked decisions

1. **Pre-mint pane stays the placeholder.** The projection returns an empty
   `structure` slice until the argument exists in the graph, so
   `StructureView`'s existing `cards.length === 0` branch keeps rendering
   today's centered placeholder. The five role cards appear only once the
   argument is minted. (This preserves Slice 7's placeholder-until-summon
   decision; it does NOT show an empty five-card skeleton up front.)

2. **The projection is derive-never-decorate.** A `done` card exists only
   because the student's Toulmin mint wrote a slot node; its preview is her own
   sentence read back verbatim (`body.text`). No status, no sentence, and no
   ordering is invented. This mirrors `projectMaterials`' contract exactly.

3. **Read-only.** The projected argument is a record, not an editor — no
   re-open / re-edit affordance (product invariant: 右侧过程树是只读记录). The
   Toulmin card is one-shot: once a `claim` node exists the classifier never
   re-surfaces it (`agent/classifier.go` gates on `!hasClaim`), so the completed
   state is stable.

4. **Discriminate Toulmin slot nodes by `type == slot.id && body.text != ""`.**
   CRAAP's `promote` also mints an `evidence`-typed node, but its body is
   `{source_quality: {...}}` with no `text` key; the Toulmin evidence slot node
   is `{text: "..."}`. Only the Toulmin `toulmin` graph-effect mints
   `claim/warrant/counter/concession` nodes at all. So the `body.text`-present
   test cleanly selects the built argument and never mistakes a CRAAP orphan
   evidence node for the evidence slot. (This is the same orphan-evidence node
   that made `no_orphan_evidence` unsatisfiable in Slice 7 — here it is simply
   filtered out, never surfaced.)

5. **Slot order and role labels come from the Toulmin card spec, not a second
   hardcoded list.** `projectStructure` reads `spec.Params.Slots` via the
   `specByID("toulmin")` the projection already receives — one source of truth
   for the five slots (id, role label, order), consistent with the card/skill
   single-source-of-truth rule. If the spec is absent (should never happen) the
   projection returns an empty slice (placeholder), never a partial/guessed set.

## 3. Backend

### 3.1 New DTO (`apps/api/internal/studio/dto.go`)

```go
// StructureCardDTO is one role in the S4 argument (论证构建). status/preview
// are DERIVED from the Toulmin mint: a card is "done" (with the student's own
// sentence as preview) only because a graph node typed for its slot carries a
// body.text she wrote; otherwise "empty". Nothing here invents an argument —
// same derive-never-decorate contract as MaterialDTO. The list is present only
// once the argument exists in the graph; before that it is empty and the pane
// keeps its placeholder.
type StructureCardDTO struct {
	ID      string `json:"id"`      // slot id: claim/warrant/evidence/counter/concession
	Role    string `json:"role"`    // 核心主张 / 理据·推理 / 支撑证据 / 反方·钢人 / 让步·转折
	Status  string `json:"status"`  // "done" | "empty"
	Preview string `json:"preview"` // the student's sentence (body.text); "" when empty
}
```

Add `Structure []StructureCardDTO json:"structure"` to `StudioProjection`
(after `ActiveCard`).

### 3.2 New projection (`apps/api/internal/studio/projection.go`)

```go
// projectStructure projects the five Toulmin argument slots into 论证构建
// role cards. Slot order + role labels come from the toulmin card spec (single
// source of truth); status/preview come from the minted graph nodes. Returns
// an empty slice until at least one slot node exists, so the 结构 pane keeps
// its placeholder before the argument is built.
func projectStructure(specByID func(string) (cards.Spec, bool), d ProjectData) []StructureCardDTO {
	spec, ok := specByID("toulmin")
	if !ok || len(spec.Params.Slots) == 0 {
		return []StructureCardDTO{}
	}
	// slot id -> the student's sentence on the first minted node of that type
	// that carries a non-empty body.text (excludes CRAAP's source_quality
	// evidence node, which has no text key — decision 4).
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

Wire it into `Project`: `Structure: projectStructure(specByID, d)`.

`text[n.Type]` is keyed on node type; only one node per slot type can carry a
`text` body in practice (the mint is one-shot), and the `seen` guard makes the
first win deterministically if that ever changes.

### 3.3 Parity (`apps/api/internal/studio/dto_parity_test.go`)

- Add `structure` to the top-level `want` key set (kept sorted).
- Assert `StructureCardDTO`'s JSON key set `["id", "preview", "role", "status"]`.

## 4. Contracts (`packages/contracts/src/studioState.ts`)

```ts
export const StructureCard = z.object({
  id: z.string(),
  role: z.string(),
  status: z.enum(["done", "empty"]),
  preview: z.string(),
});
export type StructureCard = z.infer<typeof StructureCard>;
```

Add `structure: z.array(StructureCard)` to `StudioProjection`.

`apps/web/src/studio/state.ts`'s `StructureCardFx` is trimmed to match the wire
shape — drop the unused `question?` field (only the dead inline-edit branch read
it). Prefer re-exporting the contract `StructureCard` type so there is one shape;
keep the `StructureCardFx` name if other modules import it, aliased to the
contract type.

## 5. Frontend

### 5.1 Wiring (`apps/web/src/studio/StudioContainer.tsx`)

`toStudioState`: `structure: p.structure` (was `[]`). Update the stale comment
that lists structure among the "deferred center-pane views".

### 5.2 Cleanup (`apps/web/src/studio/views/StructureView.tsx`)

The `RoleCard` `active` branch (the disabled textarea + "素材选择器（Slice 7
接入）" placeholder + the AI-question Bean bubble) is now unreachable: the active
Toulmin card takes over the whole pane as `StudioToulminCard` before the card
list renders, and the projection only ever emits `done`/`empty`. Remove the
`active` branch and the `active`/`question` locals. Keep `done` (preview) and
`empty` (待开始), the gate banner, and the header count. `allClean` becomes
`cards.every((c) => c.status === "done")` (equivalent to `!== "empty"` once the
only remaining statuses are done/empty — pick the positive form for clarity).

## 6. Testing

- **Studio unit (`projection_test.go`):** table — (a) no Toulmin nodes → empty
  slice; (b) five minted slot nodes (claim/warrant/evidence/counter/concession,
  each `{text}`) → five `done` cards in spec-slot order, previews = the
  sentences; (c) a CRAAP `evidence` node (`{source_quality}`) present but no
  Toulmin nodes → still empty (evidence slot not falsely `done`); (d) claim +
  evidence minted, others absent → claim/evidence `done`, rest `empty`, list
  present.
- **Real-Postgres e2e:** extend `TestRefactor2CardsLoop_ToulminBuildsArgument`
  (or the studio `roundtrip_test.go`) so that after the real mint,
  `studio.Project` returns five `done` structure cards with the student's
  sentences and `allClean` would hold. This proves the projection over genuinely
  minted rows, not hand-built nodes — the Slice 7 lesson that the summon→mint→
  project loop must be exercised end-to-end.
- **Parity:** as in §3.3.
- **Frontend (`StructureView.test.tsx`):** five `done` cards render their
  previews and the green gate banner; an empty `cards` array renders the
  placeholder (guard against a regression to a false green banner on 0 cards).
  Remove any test asserting the deleted `active` inline branch.
- **Contracts:** `StructureCard` parses valid rows and rejects a bad `status`;
  `StudioProjection` requires `structure`.

## 7. Out of scope

- No new interaction, no re-edit of a locked argument, no source chips on the
  `done` cards (the design's collapsed/done state shows only the sentence).
- The standalone legacy `concession`/`steelman` cards remain unmigrated — that
  is a separate carry-forward, untouched here.
- No change to the Toulmin mint, the gate, the classifier, or the S4 gate math.
```
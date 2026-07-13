# Slice 6c — SIFT lateral reading · design

> Whole-product refactor #2, Slice 6 (Material + source log), part c.
> Predecessors: Slice 6 keystone (CRAAP fill→mint live) and Slice 6b
> (素材 center pane, project-scoped ingestion, source log).
> Spec authority: `docs/2026-07-11-agent-spec.md` (C1 primitives, C2 card
> format), `docs/2026-07-11-product-spec.md` (S3 evaluate-sources),
> `docs/工具包库/05-信息素养_SIFT横向核查卡.md` (the tool card),
> `docs/design/思维印记_工作区.dc.html` (binding UI).

---

## 1. What this slice is

CRAAP is **vertical**: one source, five dimensions, drilled down. SIFT is
**horizontal**: leave the page, see what *other* sources say about this one,
trace the claim to its origin, then come back and revise your judgment.

> 判断一个来源，要看它**外面**的人怎么说它。 — `05-信息素养_SIFT横向核查卡.md`

Slice 6b made the 素材 view real and gave the student her own ingestion path
(添加信源). 6c uses that path as the *mechanism* of lateral reading: SIFT is
not complete until an independent source has actually entered the project.

This slice delivers:

1. The **`compare` primitive** — the second primitive in the C1 library, and
   the first new one since Slice 1.
2. **SIFT as pure C2 config** over `compare` — the "new card = new JSON, zero
   renderer code" claim, re-proven on a primitive that is not `annotate`.
3. The **`cross_check` mint** — SIFT completion writes a real graph node + two
   edges, and flips `source_log_entry.lateral_read`.
4. The **S3 machine gate** finally machine-checking lateral reading (today it
   is a self-attested `student_written` item).
5. The design's **`正在核对 · 需横向阅读`** chip, derived from real state.

**Non-goals.** No new ingestion code (6b's endpoint is reused as-is). No
citation surface (RL-2's citation half stays in Slice 8). No perspective map
(Slice 7). No AI verdict on any source, ever.

---

## 2. The `compare` primitive (C1)

Agent-spec §C1 defines it as *"two materials side by side with paired
annotations"*, state `{left, right, pairs: [{l_span, r_span, note}]}`. We keep
that shape and add three fields the spec's sketch omits: `id` (pairs are
addressable), `author` (every mutable unit is authored — C1's own rule), and
`relation` (§4.2 — the student's judgment, which needs a home in the primitive
state so the renderer never has to read card `field_values`). The Zod contract
lands in `packages/contracts/src/interactionPrimitive.ts` beside `AnnotateState`
/ `GraphState`:

```ts
export const ComparePair = z.object({
  id: z.string().min(1),
  l_span: z.string().min(1),      // AnnotateSpan.id on the left material
  r_span: z.string().min(1),      // AnnotateSpan.id on the right material
  note: z.string(),               // what the right source says about the left claim
  relation: z.enum(["corroborates", "contradicts", "qualifies"]),
  author: Author,                 // student for every real pair
});
export const CompareState = z.object({
  left: AnnotateState,            // the source under review
  right: AnnotateState.nullable(),// the lateral source; null until she finds one
  pairs: z.array(ComparePair),
});
```

`right` is **nullable by design**: the empty right pane is the state SIFT
exists to resolve. It is not a loading state — it is the assignment.

**The renderer composes, it does not reimplement.** `Compare.tsx` renders two
`Annotate` panes (the Slice-1 component, unchanged) plus a pair-link layer.
If the compare renderer needs to duplicate span segmentation, the composition
is wrong. Left pane = the source under review; right pane = the lateral source,
or, before one exists, the empty state pointing at 6b's 添加信源.

**Authorship.** Left-pane spans may be `ai` (the coach's generated anchors —
the suspicious sentences). Every **pair** is `student`: the relation and the
note have no honest producer but her. The AI may not create a pair.

### 2.1 CompareState is projected, not stored

The persisted card_instance state is unchanged from 6b: **anchors +
`field_values`**. `CompareState` is the *primitive's view* of that state,
projected on read — there is no second storage shape and no migration:

| CompareState | projected from |
|---|---|
| `left.material_id` / `left.spans` | the card's own material; anchors whose `material_id` **is** it |
| `right` | anchors whose `material_id` is **not** it (`null` when there are none) |
| `pairs[].l_span` / `r_span` | a right anchor paired with the left anchor sharing its `dimension` |
| `pairs[].note` | the right anchor's `answer` — what she says the lateral source says |
| `pairs[].relation` | the anchor whose `dimension` is `relation` |

**Anchors are the answer carrier — `field_values` is not involved.** The
runtime's pure functions (`EvaluateCompletion`, `GraphEffects`) are defined over
anchors and never see `field_values`; CRAAP's `risk_note` already rides this
way, and the coach rail already renders an answer per anchor keyed by
`dimension`. SIFT's `relation` and `trace_origin` are therefore anchors like any
other answer. Routing them through `field_values` instead would force a
signature change through three call sites and buy nothing.

One SIFT card carries one `relation`, so a card mints exactly one `cross_check`
node (§4.2). Pairs remain a list because the primitive is general — a future
card may pair several spans — but SIFT's completion needs only one.

### 2.2 Which material is the card's own? (`params.lateral_dimension`)

`CompleteCard` today calls `anchoredMaterialID(anchors)`, which returns *the
first anchor carrying a material id*. Its own comment states the assumption:
*"every anchor on one card_instance targets the same material."*

**SIFT is the first card that violates it.** Its anchors span two materials by
design. Left alone, the mint would attach the cross-check to whichever material
happened to sort first in the anchor array — a coin flip between the source
under review and the source used to check it, failing silently and differently
per card. (Same species as Slice 6b's `card_instances.task_id` trap: an
invariant that held until the slice that broke it, with nothing that fails
loudly.)

**The fix is declaration, not inference.** The C2 `params` block gains one
field:

```json
"params": { "lateral_dimension": "find" }
```

- **lateral material** = the `material_id` of the anchor whose `dimension` ==
  `params.lateral_dimension`.
- **checked material** = the `material_id` of the first anchor whose
  `dimension` != `params.lateral_dimension`.

A card with no `lateral_dimension` (every card that exists today) degenerates to
exactly the current behavior — a required regression test, not an assumption.

---

## 3. SIFT as C2 config

`packages/contracts/cards/sift.json`, `primitive: "compare"`, four steps
mirroring the tool card (`05-信息素养_SIFT横向核查卡.md` §如何交互):

| step | key | what she does | why it is not the AI's |
|---|---|---|---|
| Stop | `stop` | writes her **first reaction** before investigating | this is the *before* half of the judgment; recorded so the *after* can be compared to it |
| Investigate | `investigate` | who published this — 机构 / 个人 / 营销号 | the AI may point at where to look, never at the answer |
| Find | `find` | **an independent source enters the project**, and she writes what it says about the claim | the AI has no ingestion path (RL-2, structural) |
| Trace | `trace` | the original provenance of the claim | 追到原始出处 — she writes it |

The card ends by sending her back to re-tier the source (§5).

**Acceptance (the architectural claim, same shape as Slice 3 §5.7):** SIFT
touches **zero renderer code**. Its JSON, plus the `compare` primitive built in
§2, plus the `cross_check` effect in §4, are the whole of it. If shipping SIFT
requires an edit to a renderer, the primitive boundary is wrong — fix the
primitive, do not special-case the card.

### 3.1 Completion

```json
"completion": [
  { "kind": "lateral_source_present" },
  { "kind": "field_written_by", "field": "trace_origin", "author": "student" }
]
```

`lateral_source_present` (new completion kind, `card_completion.go`) is
satisfied only when the anchor carrying `params.lateral_dimension` (§2.2) has a
`material_id` that differs from the card's checked material **and** a non-empty
`answer`. In words: *a real, other source is in the project and she has said
what it says.*

This is the hinge of the slice. A written claim of having read laterally is not
lateral reading. The AI cannot satisfy this predicate — it has no path that
creates a material (verified in 6b; re-verified here).

### 3.2 How the lateral material rides in

6b established that **anchors carry their own `material_id`** (there is no
`material_id` column on `card_instances`). SIFT needs no new transport: the
anchor for the `find` step simply carries the lateral source's id. The
right-pane spans are exactly the anchors whose `material_id` ≠ the card's
material.

---

## 4. The mint (`cross_check`)

### 4.1 What it does NOT do

SIFT does **not** promote the lateral source to `evidence`. Promotion is
CRAAP's effect and it means *"this source has been evaluated and may be
cited."* A source that entered the project ten seconds ago as a cross-check has
been evaluated by nobody. Promoting it here would let a source become citable
without evaluation and would quietly hollow out the `every_source_evaluated`
gate. It stays a material until she CRAAPs it.

### 4.2 What it does

`graph_effects: [{ "kind": "cross_check" }]` mints, in one transaction:

```
MintNode  { Type: "cross_check", Author: "student", Body: {
              relation, note, trace_origin, tier_before, tier_after } }
MintEdge  { material(checked)  --cross-checked-by-->  $new:0 }
MintEdge  { $new:0             --cites-->             material(lateral) }
```

`relation` is the student's own choice — 印证 / 反驳 / 限定
(`corroborates` / `contradicts` / `qualifies`). **The AI never picks it.** Per
6b's binding rule, a judgment with no honest producer does not get produced.

**Runtime change required:** only that `GraphEffects` learns the `cross_check`
kind (today it handles `promote` and silently skips everything else).

The placeholder resolver needs **no** change: `CompleteCard` already calls
`resolveMintRef` on *both* `e.FromID` and `e.ToID` (`card_lifecycle.go:117,121`),
so a `$new:` on an edge's source already resolves. Only the `MintEdge` doc
comment claims otherwise ("MintEdge.ToID carries the placeholder") — it is
stale, and this slice corrects it.

### 4.3 The source log

The same transaction sets `lateral_read = true` on the **checked** source's
`source_log_entry` — the source that *was* laterally read. The lateral source is
the instrument, not the subject; its own entry is untouched.

This closes an orphan: `lateral_read` has existed since Slice 0's migration
`0016` and has been written by nobody and read by nobody. From this slice it is
written by the mint and read by the dossier chip (§6) and the 检索日志 ledger.

**Two records, one act, one transaction.** The graph node is what the *gate*
reads; the log column is what the *ledger and chip* read. They cannot drift —
both are written by `CompleteCard` inside the same tx — and a test asserts they
agree.

---

## 5. The before/after judgment

The tool card's last move: 回到原说法，让学生用金字塔给它定位，并下一个修正后的判断.

6b already stores the student's ingestion-time pyramid tier in
`source_log_entry.tier`. So SIFT's closing step is not new data — it is a
**second reading of data we already have**: she re-tiers the source, and if the
tier changed, that change is *observable and attributable*.

> 一手报道 → 二手 · 需追源

That is a stance change she produced by her own lateral read — the T2/T4
thinking-leap evidence the assessor (Slice 10) exists to find, obtained for the
cost of one column that already exists. `tier_before` and `tier_after` both go
into the `cross_check` node body; the log entry's `tier` is updated to
`tier_after`.

If the tier does not change, nothing is emitted. A revision that did not happen
is not recorded as one.

---

## 6. UI (binding design)

`docs/design/思维印记_工作区.dc.html:1052` already carries the chip:

```
正在核对 · 需横向阅读     点亮的句子 = 印记标出的可疑处
```

**Derivation (no decoration).** The chip shows when, and only when: an **active
(open, uncompleted) card exists on this material** AND the material has **no
`cross_check`** yet. `正在核对` is the card being open; `需横向阅读` is the
missing cross-check. Both halves are facts about state, not opinions about the
source. When the cross-check lands, the chip goes.

This follows 6b's rule exactly: the dossier's chips are derived, never
decorated, and there is **no credibility verdict anywhere** — 可信 / 存疑 / 偏弱
have no honest producer and are not rendered.

The 检索日志 ledger (6b) gains one derived mark per entry: **已横向核查** when
`lateral_read` is true. Nothing else in the ledger changes.

---

## 7. The gate

`packages/contracts/skills/writing-project.json`, contract `evaluate_sources`
(S3), today:

```json
"machine": [ {"kind": "every_source_evaluated"}, {"kind": "no_single_sourced_claim"} ],
"student_written": ["source_risk_notes", "lateral_read_logged"],
```

`lateral_read_logged` is **self-attested** — the student ticks that she did it.
After this slice the machine can see it, so it moves tier:

```json
"machine": [
  {"kind": "every_source_evaluated"},
  {"kind": "no_single_sourced_claim"},
  {"kind": "node_present", "type": "cross_check"}
],
"student_written": ["source_risk_notes"],
```

**No new machine predicate.** `node_present` already exists in the closed set
(`skills.MachineKinds`) and already takes a type. The gate change is therefore
**pure skill config** — one line in the JSON plus its synced Go mirror
(`make sync-skills`), and zero lines in `gate.go`. `attemptedFor` already
returns `true` for `node_present` (it is a positive-presence predicate, not a
vacuous negation), so no change there either.

DEC-3 is unaffected: the machine tier still caps at `machine_clear`. Whether the
lateral read was any *good* remains the human item
(`source_quality_spot_check`).

---

## 8. Risks and decisions

**R1 — Adding sources makes `every_source_evaluated` harder.** A lateral source
is a material of kind `article`, so the existing gate will expect a CRAAP on it
too. This is pre-existing 6b behavior (any ingested source does this), but 6c
makes adding sources routine, so it will bite more often.
**Decision: no exemption.** A source in the project is a source. If she intends
to lean on it, it deserves evaluation; if she does not, it still belongs in her
log. Special-casing lateral sources would create a category of source that is
in the project but invisible to the gate — exactly the kind of hole that makes
a gate decorative.

**R2 — The empty right pane could read as "broken".** Mitigation: it is an
explicit assignment state with the 添加信源 affordance in it, not a spinner and
not a blank. Copy comes from the design.

**R3 — `compare` could be built as a fork of `annotate`.** That would be the
easy path and it would rot: two span renderers drifting apart. The primitive
**composes** the existing `Annotate` component. This is a review checkpoint, not
a suggestion.

**R4 — SIFT could be authored to need renderer edits.** Then the primitive
boundary is wrong. The acceptance criterion (§3) is the guard: if the card
cannot ship as config, fix `compare`, not the card.

**R5 — The relation could be inferred by the AI** ("this source contradicts
your claim"). It must not be. It is a judgment; she makes it. The AI's role is
to ask her what she sees.

---

## 9. Testing

- **Contracts:** `CompareState` accepts the C1 shape; rejects a pair with a
  non-student author; `right: null` is valid. SIFT JSON validates against the
  C2 card spec and its `primitive` is `compare`.
- **Go — completion:** `lateral_source_present` is RED when the only anchors
  share the card's material id; RED when the other-material anchor has an empty
  `answer`; GREEN when a real other-material anchor with an answer exists.
- **Go — material disambiguation (§2.2):** with anchors spanning two materials,
  the checked material is the one declared by `lateral_dimension`, **not** the
  first in the array — a test that fails if the anchor order is reversed. A card
  with no `lateral_dimension` (CRAAP) resolves exactly as it does today.
- **Go — mint:** `cross_check` produces one node and two edges with the
  placeholder resolved on **both** endpoints; `relation` and both tiers land in
  the node body; `lateral_read` flips on the checked entry and **not** on the
  lateral one; the lateral source is **not** promoted to `evidence`.
- **Go — gate:** S3's machine tier fails without a `cross_check` node and passes
  with one; `every_source_evaluated` still scans the lateral source (R1 is
  asserted, not assumed).
- **Web — primitive:** `Compare` renders both panes from `CompareState`; the
  right pane's empty state renders when `right` is null; pairs link the two
  spans.
- **Web — card:** SIFT surfaces in the coach rail, the center pane splits, and
  completing it refetches the projection (the Slice-6b whole-branch lesson: the
  client must learn the mint landed — assert the chip clears).
- **RL-2:** a test asserting no agent-reachable path mints a material (the 6b
  test, extended to the SIFT flow).

---

## 10. Carry-forwards

- The **search-plan card** (S2) still needs its own design — deferred from 6b,
  still deferred.
- **RL-2's citation half** (citations only from the log) needs a citation
  surface — Slice 8.
- The **perspective map** (S2) is graph-backed — Slice 7.
- Stored `event` rows keep `type`/`surface` as DB columns while the Zod variants
  are flat; Slice 10's assessor must merge columns + payload before validating
  (carried from 6b, unchanged).
- `event.lateral_read` (the optional field on the C4 `source_opened` event)
  remains unused; the fact now lives on the log row and the graph. Slice 10
  decides whether the event field earns its place or is dropped.

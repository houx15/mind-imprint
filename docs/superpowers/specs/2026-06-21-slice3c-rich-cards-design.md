# Slice 3c — Rich Card Interactions · Design & Spec

> Status: design self-approved (user delegated autonomous execution, waived interactive review). Date: 2026-06-21.
> S3c is the roadmap's last named slice ("later" / breadth). It upgrades the 10 `body_status:"stub"` cards to real interactions. The platform is already end-to-end runnable (S1–S6); S3c is non-critical-path breadth.

## The core mechanism (unchanged architecture)

The card runtime is schema-driven: `packages/contracts/src/primitives.ts` defines field types as a discriminated union on `type`; `apps/web/src/cards/fieldRegistry.tsx` maps `type → component`; `CardRenderer` renders `step.fields[]` via the registry; `field_values[key]` + trace events flow through `onField`. **Adding an interaction = (1) add a primitive to the union, (2) add a field component, (3) register it, (4) flip the card JSON `body_status` to `full` and replace the placeholder textarea with real fields.** The renderer itself never changes per-card. This is the validation standard for "schema-driven": new card = new JSON, new interaction = one new primitive reused across cards.

## Decomposition: 5 family sub-slices (by `interaction_type`)

The 10 stub cards group into 5 interaction families. Each family = one sub-slice (own plan → TDD → merge), ordered by increasing complexity so the highest-leverage, lowest-risk primitives land first and the pattern is proven before the hard ones:

| Sub-slice | Family (`interaction_type`) | Cards | New primitive(s) |
|---|---|---|---|
| **3c-A** | 量表光谱 spectrum | belief-spectrum, certainty-spectrum | `spectrum` |
| **3c-B** | 分类标注 classify | spin-detector, science-knowing | `classify` |
| **3c-C** | 步骤引导 stepped-guide | aok-methods, corpus-hook | `show_if` (field/step modifier) |
| **3c-D** | 画布导图 canvas-map | argument-map, money-trail, source-map | `node_map` |
| **3c-E** | 角色模拟 role-play | ethics-roleplay | `role_play` |

Each sub-slice's interaction is designed faithfully to the card's `docs/工具包库/*.md` (methodology + 如何交互 + 渲染要点 + AI 克制红线), but the visual language is lifted from the existing design system (the `mk-*` Tailwind tokens + the established field components like `RatingField`), since the binding `思维印记_工作区.dc.html` does not draw these interactions. Restraint red-lines from each card's 红线 are preserved (AI never decides for the student).

## Global Constraints (every sub-slice)

- Verification gate per task: `pnpm -r typecheck` (tsc `--noEmit`) **and** `pnpm -r test` green. vitest does NOT typecheck — run `pnpm -r typecheck` explicitly.
- New primitives are additive to the `FieldPrimitive` discriminated union; existing primitives and the standard envelope (`CardInstance.field_values` is `z.record(z.unknown())`) are unchanged, so no migration. A new primitive that is also valid inside `repeatable_group` must be added to `ItemField` too.
- Each new field component conforms to `FieldProps<F> = { field, value, onChange }`; value persists to `field_values[key]` via the existing `onField`; no `CardRenderer`/store/envelope change except where a sub-slice explicitly needs a renderer feature (3c-C `show_if`).
- Accessibility: interactive primitives expose proper ARIA roles and keyboard operation (not pointer-only), so they are testable and usable.
- Un-stubbing a card = flip its JSON `body_status:"stub"→"full"` and replace the single placeholder textarea with the real fields. The registry count (33 cards) and all existing tests stay green.
- Restraint (四条铁律): the AI proposes/sets up but never concludes for the student; no addictive mechanics.

---

## Sub-slice 3c-A — `spectrum` primitive (DETAILED — implement first)

**Goal:** a labeled continuous-looking axis the student positions a marker on, to escape binary thinking and calibrate certainty. Serves certainty-spectrum directly and belief-spectrum compositionally.

### New primitive `spectrum`

Add to `packages/contracts/src/primitives.ts`:
```ts
export const SpectrumField = z.object({
  type: z.literal("spectrum"),
  ...base,                       // key, label
  stops: z.array(z.string()).min(2),   // labeled positions; first/last are the poles
});
```
- Add `SpectrumField` to BOTH the `ItemField` union (so it works inside `repeatable_group`, which belief-spectrum needs) and the top-level `FieldPrimitive` union.
- **Value semantics:** `field_values[key]` = an integer index `0..stops.length-1` (which stop the marker is on), or `undefined` when untouched. (Discrete labeled stops, not a free 0–100 float — matches certainty-spectrum's `scale` render spec.)

### Component `apps/web/src/cards/fields/SpectrumField.tsx`

- A horizontal track with `stops.length` tick positions and the stop labels; a marker sits on the current index.
- **Implemented as `role="radiogroup"` + one `role="radio"` per stop** (mirrors the proven `RatingField` idiom — discrete labeled stops are semantically choices, not a continuous position; more accessible + testable than a raw slider). `aria-checked` on the current index; the radiogroup is keyboard-focusable (`tabIndex={0}`). (Earlier draft said `role="slider"`; the radiogroup idiom supersedes it.)
- Interaction: click a stop to set that index; ArrowLeft/ArrowRight (+ Home/End, ArrowUp/Down aliases) move the marker and call `onChange(index)`, clamped to `[0, max]`. (Pointer-drag is a future enhancement; click + keyboard achieves positioning and is testable.)
- Untouched (`value` not a number) → no marker filled / marker at neither end with a muted hint; first interaction sets the index.
- Styling matches `RatingField` idiom (mk tokens: `bg-mk-primary` for the active marker/filled track, `#EEF0F4` track, `#9AA1B0` muted labels).

### Register

Add `spectrum: SpectrumField` to `apps/web/src/cards/fieldRegistry.tsx`.

### Un-stub the two cards (flip `body_status` to `full`, real fields)

**`certainty-spectrum.json`** — steps[0].fields:
```
- text  key="claim"   label="你的结论是什么？"
- spectrum key="position" label="把它放到确定度光谱上" stops=["个人猜测","有据推断","强证据","科学共识","逻辑必然"]
- textarea key="why" label="为什么是这个位置？支撑它的证据类型是什么？"
- textarea key="rewrite" label="用与该位置相称的语气词，重写你的结论句（可能 / 大概 / 很可能 / 几乎确定 / 必然）"
```
(rubric_dims D6/D9 already in JSON; keep.)

**`belief-spectrum.json`** — steps[0].fields:
```
- text key="issue" label="争议议题是什么？"
- repeatable_group key="stances" label="把各方立场放到光谱上" item_fields=[
    text     key="who"      label="是谁/哪个立场",
    spectrum key="position" label="在光谱上的位置" stops=["这一极","偏这边","中间","偏那边","那一极"],
    textarea key="believes" label="它相信什么",
    textarea key="evidence" label="它引什么证据",
    textarea key="interest" label="背后有什么利益/背景" ]
- spectrum key="self"   label="我现在站这里" stops=["这一极","偏这边","中间","偏那边","那一极"]
- textarea key="self_reason" label="我站这里的理由（他们的分歧到底在证据、价值，还是利益？）"
```
(rubric_dims D4 already in JSON; keep.)

### Tests (TDD)
- contracts: `SpectrumField` parses (valid stops ≥2; rejects <2); `FieldPrimitive` and `ItemField` accept `spectrum`; the two un-stubbed card JSONs parse via `CardSpec` and are `body_status:"full"`; registry still loads 33.
- component: renders stops + labels; `role="slider"` with correct aria values; click a stop calls `onChange(index)`; ArrowRight from index i → `onChange(i+1)` (clamped at max), ArrowLeft clamps at 0; renders the marker at `value`.
- fieldRegistry: `spectrum` resolves to `SpectrumField`.
- Harness/registry count tests stay green (still 33 cards; 2 fewer stubs).

---

## Sub-slice 3c-B — `criteria_check` primitive (REFINED)

**Family:** 分类标注 (spin-detector, science-knowing). On reaching it, the faithful design is:
- **science-knowing** needs a NEW primitive **`criteria_check`** — a fixed list of named criteria, each judged on a small shared scale, with the four standards 可证伪/对照/可重复/同行评审 assessed against a claim. Schema: `{ type:"criteria_check", key, label, criteria: string[], levels: string[] }`; value = `number[]` (length `criteria.length`, each = chosen level index; absent/`-1` = untouched). Component = one row per criterion + a per-row radiogroup of `levels` (reuses the radio idiom; immutable array update like `multi_choice`). Added to `FieldPrimitive` only (top-level; not nested).
- **spin-detector** needs NO new primitive — it composes from existing primitives: a `repeatable_group` for the 漂绿「说的 vs 做的」table ({claim textarea, actual textarea}) and a `repeatable_group` for FLICC snippet-tagging ({snippet textarea, single_choice over the 5 tactics 假专家/逻辑谬误/不可能的标准/挑拣证据/阴谋论}). Both steps shown (no mode-toggle dependency on `show_if`).

Restraint: both cards set up the criteria/labels and ask the student to judge/tag + justify; neither pre-judges "这是伪科学 / 这家在漂绿". (science-knowing rubric_dims D2/D5/D9; spin-detector D2/D4/D5 — already in JSON.)

## Sub-slice 3c-C — `show_if` stepped-guide (REFINED)

**Family:** 步骤引导 (aok-methods, corpus-hook).
- **`show_if` is a field MODIFIER, not a new field type:** an optional `{ key: string; equals: string }` added to the field `base` (so every primitive may carry it; no new `FieldType`, no `fieldRegistry` change). It is evaluated ONLY at the step level in `CardRenderer`'s `StepFields` — a field renders iff `!show_if || values[show_if.key] === show_if.equals`. This is the ONE sub-slice that touches `CardRenderer`. (Inside `repeatable_group` items `show_if` is not evaluated — documented; no card needs it there.)
- **aok-methods** uses it: a `single_choice` `subject` (数学/人文社科/艺术) reveals discipline-specific `textarea`s via `show_if: {key:"subject", equals:"数学"}` etc. (math: 证明vs证据/找反例/公理前提; human: 社科五问; arts: 文本依据).
- **corpus-hook** needs NO new feature — composes from existing primitives: a `repeatable_group` of hooks ({locator, question, answer}) + a `repeatable_group` of recommended reading-cards (six elements) + a direction textarea.

Restraint: aok-methods gives the discipline's standard + guiding questions but never completes the proof/interpretation/social-science judgment; corpus-hook poses anchor questions + recommends pre-vetted cards but never answers the hook or pushes un-vetted open content. (aok-methods D2/D5; corpus-hook D1/D5 — already in JSON.)

## Sub-slice 3c-D — 画布导图 + 角色模拟, composed (REFINED — final S3c sub-slice)

On reaching the last two families, the faithful engineering call is **composition, not a new primitive**. All four remaining cards' interaction DATA is structured lists that the existing primitives (`text`/`textarea`/`single_choice`/`repeatable_group`) express directly; the "node canvas with connecting lines / drag" and "multi-role live dialogue" are VISUAL/real-time layers, not the thinking the card elicits. Building a free-form node/edge drag-canvas or a dialogue engine would be gold-plating the data doesn't require and the riskiest code in S3c. So 3c-D un-stubs all four with existing primitives (rich structured forms faithful to each `.md`), and the canvas/dialogue visualizations are logged as **future enhancements** (same posture as deferring spectrum's pointer-drag). This finishes S3c — all 10 stub cards → `full`.

- **argument-map** (论证地图): claim text + `repeatable_group` reasons {论据, 隐藏假设, `single_choice` 谬误标签, 谬误依据} + 最弱环节 + 补强.
- **money-trail** (资金链溯源): claim text + `repeatable_group` chain {节点, `single_choice` 这是哪一环, 钱/利益来源} + `single_choice` 利益-结论一致性 + 这说明什么.
- **source-map** (3D 溯源导图): `repeatable_group` nodes {材料/观点, `single_choice` 节点类型, `single_choice` 偏见标注, 关系说明} + 同源识别 + 判断是否改变.
- **ethics-roleplay** (角色博弈): 情景与核心问题 + `repeatable_group` roles {角色, 利益, 责任, 盲区} + 收口（AI 该介入到哪一步？谁该负责？）.

Restraint: argument-map highlights the weakest link but the student strengthens it; money-trail flags interest alignment but concludes "被资助 ≠ 必假，要交叉验证" not "凡被资助即假"; source-map labels bias without declaring a source unusable; ethics-roleplay sets the scenario/roles but the student reasons each stance and the closing — AI never resolves the dilemma. (Rubric dims already in each JSON.)

## Future enhancements (logged, out of S3c scope)

Free-form node/edge **drag canvas** for the 画布导图 cards (argument/money/source); **multi-turn live role-play dialogue** for ethics-roleplay; **pointer-drag** on `spectrum`; a `node_map` / `role_play` primitive if/when those visual layers are built. All non-blocking; the composed forms are fully functional now.

## Out of scope (S3c)

Pointer-drag/free-canvas polish (click + keyboard first), new rubric dims, eval changes, the deferred S6-review minors (tracked separately), real backend. Each sketch (B–E) is refined into its own spec section + plan when its sub-slice begins.

## Conclusion

S3c upgrades the 10 stub cards across 5 family sub-slices, each adding at most one reused primitive and flipping card JSON to `full`, all schema-driven with zero per-card renderer code. 3c-A (`spectrum`) is fully specified and implemented first; B–E follow in complexity order, each refined and built in turn. Graceful degradation holds throughout: any not-yet-built family's cards remain `stub` (proposable, openable, submittable).

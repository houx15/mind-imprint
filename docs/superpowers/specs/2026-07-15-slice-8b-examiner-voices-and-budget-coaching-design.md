# Slice 8b — examiner-voice switching + budget-deletion coaching (S5) · design

> Builds directly on the Slice 8 keystone (`docs/superpowers/specs/2026-07-15-slice-8-writing-surface-design.md`).
> The keystone shipped the 写作 station: silent edit buffer → immutable snapshot → student-triggered
> 整稿体检 (one default board examiner voice) → three-key disposition → the S5 gate. This slice adds the
> two deferred pieces of that station: **switchable examiner voices** and **over-budget deletion coaching**.

## 1. Scope (locked in brainstorm)

**In scope:**
1. **Examiner-voice switching** on 整稿体检 — the default board voice plus three generic voices
   (hostile sceptic / friendly non-specialist / word-count executioner), seeing the *same snapshot*
   through different examiners.
2. **Budget-deletion coaching** — a deterministic count-vs-band verdict surfaced on every snapshot,
   plus an over-budget deletion lens folded into 整稿体检 that frames cuts in mark-scheme language.

**Explicitly deferred (NOT this slice):**
- **Board-specific passes** (EE evaluation-density scan, AP organization×connection diagnostic). These
  are per-board S5 variants, but only the `writing-project` (Cambridge 0457) skill is seeded — there is
  no EE or AP skill to drive them. The product spec files EE's evaluation-density scan under Phase 2
  (the EE/EPQ board pack). They belong to those board-pack slices, where a board skill provides the
  config a pass needs. Building the mechanism now = speculative infrastructure with no data to exercise
  it end-to-end (YAGNI).

**No migration.** Everything rides existing tables: reviews are `intervention` rows whose `anchor` is a
free-form `jsonb`, and voice is a new key inside that anchor. The word-budget band is existing skill
config. Nothing new is stored beyond what already exists.

## 2. Red lines this slice must not cross

- **RL-1 (structural): the AI never writes prose.** The voices change *tone and lens*, never output
  shape. Every voice's work-order still writes only `review_item` intervention rows (typed advice). The
  over-budget deletion lens asks *diagnostic questions* ("这段背景在向哪张表交证据？") — never "删掉这段"
  and never a rewritten/substitute sentence. The existing banned-phrasing enforcement runs per voice,
  all-or-nothing, and rejects the whole review on any violation.
- **克制 (AI restraint):** budget coaching is a *question about which criterion a paragraph serves*, not
  an instruction. The student decides what to cut. The deterministic verdict is a neutral fact (count vs
  band), not a nag.
- **Cost is recorded:** every real model call (each voice's first run) records 档位+token+成本 via
  `RecordLLMCall`. A replay (already-run voice on the same snapshot) makes **no** model call.

## 3. Locked decisions

1. **Voices are a fixed enum in Go, not skill config.**
   `board | sceptic | layperson | executioner`. The three generic postures are board-agnostic by
   definition, so they are hardcoded posture prompts. `board` is the *existing* `reviewPosturePrompt`
   verbatim (the keystone's default). No skill-JSON change for voices; a future board overriding its own
   examiner register is a board-pack concern, out of scope here.
2. **Per-(snapshot, voice) idempotency via the anchor.** The review anchor becomes
   `{kind:"draft_snapshot", id:<sid>, voice:<voice>}`. Replay filters by `(sid, voice)`. Each voice
   caches independently; re-running an already-run voice replays persisted rows with no second model
   call. Anchors written by the keystone (no `voice` key) read back as `board` (see §7 back-compat).
3. **Voice is a query param.** `POST /api/v1/projects/{id}/snapshots/{sid}/review?voice=sceptic`. Absent
   or unknown → `board` (keeps every existing call valid). Validated against the enum server-side.
4. **The `whole_draft_review` human gate is satisfied by the first review of *any* voice** (with
   `len(persisted) > 0`) — unchanged from the keystone. Voices are alternate lenses on the same
   engagement, not separate gate requirements.
5. **Budget verdict is deterministic, from `CountWords`, no model.** A pure helper
   `agent.BudgetVerdict(wc, band) → (state, delta)`: `state ∈ {in, over, under}`, and `delta` is a
   **non-negative magnitude** — words past `max` when `over`, words short of `min` when `under`, `0`
   when `in` (direction is read from `state`, so the UI renders `超出 {delta}` / `还差 {delta}`
   directly). Carried on the **projection** snapshot as `budget:{state, delta}` (`count`/`min`/`max`
   are already present as the snapshot's `wordCount` and the projection's top-level `wordBudget`, so
   they are not duplicated). The commit response is unchanged — `StudioContainer` refetches the
   projection after every commit, so the verdict reaches the client that way.
6. **The over-budget deletion lens rides 整稿体检.** When the reviewed snapshot's `state == over`,
   `ProposeReview` appends a deletion-lens instruction to the posture. It reuses the review's existing
   段落⇄评分表 mapping (the only place that mapping is computed). Composes with any voice; the
   `executioner` voice + over-budget is the sharpest pairing but the lens fires on over-budget
   regardless of the chosen voice.
7. **Projection carries all run voices for the current snapshot**, keyed by voice, so the pills can show
   which voices are already cached and switching between cached voices is instant. Still one additional
   query (the existing latest-snapshot review query, no longer collapsed to a single voice).

## 4. UI (extends the binding design's 写作 view)

The binding design (`思维印记_工作区.dc.html`, 写作 view ~L1092–1155) draws the header row
(编辑/预览 tabs · `snapshotMeta` · 整稿体检 button) and the work-order panel. This slice adds two things
in the design's own idiom — no restyle of existing elements:

- **Voice pills** — a 4-segment control reusing the 编辑·安静 / 预览·批注 segmented styling, placed in the
  header row immediately left of the 整稿体检 button. Labels: `考官` (board) · `怀疑` (sceptic) · `外行`
  (layperson) · `字数` (executioner). The selected pill uses the active-tab treatment; a small dot/✓ on
  a pill marks a voice already cached for the current snapshot. Picking a voice does not run a review;
  clicking 整稿体检 runs (or replays) the currently selected voice.
- **Budget-enriched `snapshotMeta`** — the existing meta line gains a budget clause:
  `第 N 版快照 · MM-DD 提交 · 只读 · 超出 340 字` (over) / `· 还差 210 字` (under) / `· 在预算内` (in).
  Over-budget uses a warning color token consistent with the design; in-band is neutral.
- **Budget note in the work-order** — when the run was over budget, a single framed line at the top of
  the work-order panel states the deletion frame (e.g. `超预算 340 字 · 删减决策按「这段在向哪张表交证据」来做`),
  and the model's per-paragraph "which table does this serve" observations appear as ordinary work-order
  advice rows (no new row type).

## 5. Backend

### 5.1 `agent/review.go`
- `type Voice string` with consts `VoiceBoard`, `VoiceSceptic`, `VoiceLayperson`, `VoiceExecutioner`,
  and `ParseVoice(string) Voice` (unknown/empty → `VoiceBoard`).
- Four posture prompts. `VoiceBoard` = the current `reviewPosturePrompt` verbatim. The three generic
  postures each keep the same iron rule (绝不改写/绝不示范句/绝不续写, JSON-array-of-criteria output) and
  differ only in stance:
  - `sceptic` — a hostile examiner who distrusts every claim and demands the evidence be shown, not
    asserted; names where a claim outruns its support.
  - `layperson` — a friendly non-specialist outside the field who flags unexplained jargon, undefined
    terms, and unstated leaps ("I don't know this area — where did this step come from?").
  - `executioner` — a word-count examiner who asks, criterion by criterion, whether each stretch earns
    its words; ruthless about passages that serve no table.
- `ProposeReview(ctx, prov, r, criteria, paragraphs, graphSummary, voice, overBudget)`:
  - selects the posture by `voice`;
  - when `overBudget`, appends the deletion-lens instruction to the system content: for the lowest
    criterion-value paragraphs, name which table (if any) they serve and pose it as the student's cut
    decision — advice only, never "删除" and never a substitute sentence;
  - everything else (JSON parse, server-resolved criterion names, per-field banned-phrasing
    all-or-nothing, usage returned even on rejection) is unchanged.

### 5.2 `api/writing.go` — `orderReview`
- Parse `voice := agent.ParseVoice(r.URL.Query().Get("voice"))`.
- Anchor: `{kind:"draft_snapshot", id:sid, voice:string(voice)}`.
- Replay filter: `reviewItemsForSnapshot(ctx, q, projectID, sid, voice)` — the anchor decode now also
  matches `voice`, treating a **missing** anchor `voice` as `board` (back-compat with keystone rows).
- Over-budget: recompute `state` from `CountWords(snap.Content)` against the skill band; pass
  `overBudget := state == "over"` to `ProposeReview`.
- Idempotent replay (existing rows for this `(sid, voice)`) → stream them, **no** model call, no new
  `llm_call`. Live run → one model call, persist per-voice rows, record `llm_call`, satisfy the human
  gate on `len(persisted) > 0` (all unchanged except the voice-scoped anchor/filter).

### 5.3 `commitSnapshot` — unchanged
The commit response keeps its Slice-8 shape. The budget verdict is projected (§6.1), not returned
here; `StudioContainer` already refetches the projection on every commit.

### 5.4 Enforcement
- Re-verify the three generic postures against `enforcement.BannedPhrasing`. The keystone added
  `rewritten-sentence-zh`. Add a rule only if a generic voice's phrasing needs guarding that the
  existing rules miss; do not add rules speculatively.

## 6. Projection + contracts

### 6.1 `studio/dto.go`, `projection.go`, `load.go`
- `WritingSnapshotDTO` gains `budget WritingBudgetDTO` (`{state, delta}`, camelCase JSON), filled from
  `agent.BudgetVerdict(wc, sk.WordBudget)`.
- `WritingReviewItemDTO` gains `voice string` — the parity-friendly realization of "keyed by voice":
  rather than a `map[voice][]item` (awkward across Go/Zod with enum keys), the review stays a **flat
  `items` array** and every item is self-describing via its `voice`. The frontend derives both the
  current-voice work-order (filter `items` by the selected voice) and the cached-voice set (the distinct
  `voice`s present) from that one array — so "all run voices carried" and "instant switch between cached
  voices" both hold.
- The latest-snapshot review is no longer collapsed to one voice: `projectWriting` includes **every**
  `review_item` row anchored to the latest snapshot, tagging each with the anchor's `voice` (missing →
  `board`). Dispositions attach to items as today. `WritingReviewDTO.Ordered` is dropped (the frontend
  derives per-voice "ordered" from whether any item carries that voice).

### 6.2 `packages/contracts/src/studioState.ts`
- `WritingBudget` (`state: "in"|"over"|"under"`, `delta: number`) as a required field on
  `WritingSnapshot`.
- `WritingReviewItem` gains `voice: "board"|"sceptic"|"layperson"|"executioner"`.
- `WritingProjection.review` becomes `{ items: WritingReviewItem[] }` (the `ordered` boolean is
  dropped), matching the DTO byte-for-byte (guarded by `dto_parity_test`).

## 7. Back-compatibility

- Keystone review rows have anchors without a `voice` key. Both the replay filter (§5.2) and the
  projection grouping (§6.1) treat a missing anchor `voice` as `board`. No backfill, no migration.
- Existing frontend/e2e calls to `…/review` without `?voice=` resolve to `board` and hit exactly the
  keystone code path.

## 8. Testing

- **Go / `agent`:** `ParseVoice` mapping (each string + unknown + empty → board); `ProposeReview`
  selects distinct postures per voice; over-budget appends the deletion instruction; banned-phrasing
  still rejects a rewrite in any voice (all-or-nothing); usage returned on rejection.
- **Go / `api`:** review with `?voice=sceptic` persists rows whose anchor carries `voice:"sceptic"`;
  a second `?voice=sceptic` replays with `llm_call` count unchanged; `?voice=board` and `?voice=sceptic`
  on the same snapshot are independent caches (two `llm_call` rows, two disjoint row sets); a keystone
  row (anchor without voice) is served under `board`; an over-budget snapshot passes `overBudget=true`
  to `ProposeReview`.
- **Go / `agent`:** `BudgetVerdict` returns correct `state`/`delta` for in/over/under (and a nil band).
- **Go / `studio`:** projection tags review items with voice; missing anchor voice → `board`; the
  snapshot `budget` verdict is present; DTO parity vs Zod (`dto_parity_test`).
- **Web:** voice pills render + selection state + cached-voice marker; picking a voice then 整稿体检
  calls `…/review?voice=…`; `snapshotMeta` shows the correct budget clause per state; over-budget
  work-order shows the budget note; switching to a cached voice renders without a fetch.
- **Contracts:** `WritingBudget` + voiced review item schemas.

## 9. Acceptance

On the seeded Phoebe 0457 project: commit a snapshot at 2,340 words → `snapshotMeta` reads
`… · 超出 340 字`. Run 整稿体检 as `考官` → the board work-order plus a budget note framing cuts by table.
Switch to `怀疑` and run again → a sharper, claim-attacking work-order on the *same* snapshot, cached
independently (a second `llm_call`); switch back to `考官` → instant replay, no new call. All voices'
first runs satisfy `whole_draft_review`. Commit a snapshot at 1,780 words → `· 在预算内`, no deletion
lens. RL-1 holds across every voice: no work-order field is ever a rewritten sentence or a "delete
this" instruction.

## 10. Out of scope / carry-forward

- Board-specific passes (EE evaluation-density, AP org×connection) → EE/EPQ and AP board-pack slices.
- The keystone's own carry-forward (preview renders the live buffer, not committed snapshot content) is
  unchanged by this slice and remains open.
- Voice choice is per-run, not persisted as a project preference; last-used-voice memory is not built.

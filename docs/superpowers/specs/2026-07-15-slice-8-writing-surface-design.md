# Slice 8 (keystone) — Writing surface + whole-draft review (S5)

**Status:** design approved (2026-07-15)
**Roadmap:** `docs/2026-07-11-whole-product-refactor-roadmap.md` (Slice 8)
**Sources of truth:** product-spec §10 (the writing surface), §12 (the whole-draft
review), §13 (process record); the binding design `docs/design/思维印记_工作区.dc.html`
(写作 view, lines 1092–1175 + JS 2258–2340); the `writing-project` skill's `draft_polish`
S5 contract.

## 1. Problem

S4 (结构) is live and projects the completed Toulmin argument (Slice 7/7b). S5 (写作)
is the next station and its center pane is still a stub: `WritingView` renders a
**read-only** textarea and a **disabled** 整稿体检 button, and
`StudioContainer.toStudioState` hard-stubs `writing: { draft: "", mode: "edit" }`.
Nothing persists, no snapshot is ever minted, and the whole-draft review does not exist.

The S5 gate is already specified in the skill and is currently unsatisfiable:

```jsonc
"draft_polish": {
  "requires": ["build_argument"],
  "produces": ["word_budget_ok"],
  "view": "写作", "title": "成稿打磨",
  "gate": {
    "machine": [{ "kind": "node_present", "type": "word_budget_ok" }],
    "student_written": ["citations_matched"],
    "human": ["whole_draft_review"]
  }
}
```

Nothing mints a `word_budget_ok` node, so the machine gate can never clear; nothing
runs a `whole_draft_review`. This slice makes the S5 loop real and closes those gates.

## 2. Scope

**Keystone — the full S5 loop, end-to-end:**

1. A **live silent edit buffer** (persists, zero AI write path — RL-1).
2. **Commit → immutable draft snapshot** (a paste mints the identical object — §10).
3. **Preview** render of the committed snapshot.
4. Student-triggered **整稿体检** (`order_review`) producing a **typed work-order**
   through the enforcement stack — the review never writes prose.
5. **One snapshot, one review** (a new review requires a new snapshot).
6. **Three-key disposition** per work-order item (保持原样 / 我来改 / 说明为什么不改 + reason).
7. The **word-budget machine gate**: an in-band commit mints `word_budget_ok`.
8. The **`citations_matched` student attestation** (student_written gate item).

Ships **one default board examiner voice** (no switcher).

**Deferred to Slice 8b:** examiner-voice *switching* + the three generic voices
(hostile sceptic / friendly non-specialist / word-count executioner); board-specific
passes (EE evaluation-density scan, AP organization×connection diagnostic); richer
budget-deletion coach prompts.

**Out of scope entirely:** span-anchored margin comments (the binding design's preview
shows a **work-order block**, not margin annotations); the five-skin readiness gauge
(Slice 9 评估 — the review's criteria here are minimal labels, not the band engine);
**automated** citation front-to-back matching (keystone uses the student attestation
only); multi-snapshot version-history UI + snapshot diffs feeding the process record
(Slice 10).

## 3. Locked decisions

1. **The review is a new first-class runtime action, not an extension of the coach.**
   A new C3 verb `order_review` + Go runtime `Action{Kind:"review"}`, a dedicated
   endpoint, its own typed output — kept separate from the ambient one-at-a-time coach
   (一次只问一个) which has a different trigger, cardinality, and output shape. It is
   **not** the isolated assessor (Slice 10, a different engine over event-stream
   projections); the whole-draft review is formative, student-triggered, and never a
   grade (RL-3).

2. **RL-1 is enforced structurally by absence.** No endpoint, verb, or runtime action
   ever writes the edit buffer or a prose node with model output. The buffer write
   endpoint takes student text only. The review action writes **only `intervention`
   rows** (typed advice anchored to the snapshot) — never a prose/claim node. The `fix`
   field is advice ("补上「可持续」的定义"), never a rewritten sentence, and the
   banned-phrasing regression suite guards that line.

3. **`word_budget_ok` is minted on snapshot commit, deterministically, no model.** Word
   count within the skill's band → upsert a `word_budget_ok` graph node
   (`author:"ai"`, matching the gate_state/plan system-node convention; body
   `{word_count, min, max}`). It is a typed marker, not prose — the authorship guard
   (which rejects AI-authored prose) does not apply. Out of band → no node (removed if
   previously present), so the machine gate honestly reflects the latest snapshot.

4. **Word-budget band is skill config, seeded 0457 = {min:1500, max:2000}.** Added as
   `word_budget` on the `writing-project` skill; the commit path and the projection both
   read it. Single source of truth; tunable in JSON.

5. **Review criteria are a minimal label list on the skill, not the readiness engine.**
   `review_criteria` on the `writing-project` skill = the 0457 tables the design shows
   (表D 来源与证据 / 表E 分析 / 表F 评估 / 表H 表达与组织), each `{code, name}`. The
   review model assesses the draft against exactly this set. The five-skin band engine
   (product-spec §11) stays in Slice 9.

6. **One snapshot, one review — idempotent.** If review interventions already exist for
   a snapshot, the endpoint returns them without a second model call. A new review
   requires committing a new snapshot. This is the §12 "revision is the price of the
   next review" rule.

7. **The three review keys map onto the existing disposition enum (no migration).**
   我来改 → `rewrite`, 保持原样 → `accept`, 说明为什么不改 → `reject`. The design's
   Chinese labels render in the UI; the reason (≥15 runes) is stored verbatim through
   the existing 5c disposition endpoint. Slightly lossy on the 3-way value vs the
   review's own semantics; acceptable for the keystone (the reason carries the signal).

## 4. Backend — buffer + immutable snapshots

`draft_snapshot` and `edit_buffer` tables exist since Slice 0. **No migration.** New
`apps/api/internal/store/queries/writing.sql`:

- `UpsertEditBuffer(project_id, content)` — one row per project (`edit_buffer_project_key`
  unique index); update-on-conflict, bumping `updated_at`.
- `GetEditBuffer(project_id)` — returns content ("" when absent).
- `InsertDraftSnapshot(project_id, seq, content, span_index)` — `seq` computed as
  `COALESCE(MAX(seq),0)+1` for the project (respect `draft_snapshot_project_seq_key`).
- `GetLatestSnapshot(project_id)` — highest `seq`, or none.
- `GetSnapshot(id, project_id)` — project-scoped fetch (404-no-leak).

Endpoints (`apps/api/internal/api/`, project-owner gated, 404-not-403):

- **`PUT /projects/{id}/buffer`** — body `{content}`; upserts. No entitlement gate (no
  model call). Debounced autosave target. Returns 204.
- **`POST /projects/{id}/snapshots`** — body `{content}` (from buffer or paste). In one
  transaction: compute `seq`, compute `span_index` (paragraph char-offset spans),
  `InsertDraftSnapshot`; count words; if `min ≤ words ≤ max` upsert `word_budget_ok`
  node else delete any existing one; append a `version_saved` event. Returns 201 with
  the snapshot DTO. Best-effort event/warn on failure never fails the commit.

Word count = rune-aware token count over the content (CJK-aware: count CJK characters
individually plus whitespace-delimited runs for latin — a helper `CountWords` with its
own unit tests; the exact rule is documented in the helper and stable across callers).

## 5. Backend — the `order_review` action

- Add **`order_review`** to the C3 `Verb` enum (`packages/contracts/src/agentOutput.ts`)
  and mirror in the Go verb handling. New runtime `Action{Kind:"review", SnapshotID,
  ReviewItems}`.
- **`POST /projects/{id}/snapshots/{sid}/review`** (SSE) — ownership 404-no-leak;
  **entitlement-gated before the stream** (model call). Idempotency: if
  `intervention`s of `type:"review_item"` anchored to `sid` exist, stream them back and
  stop (no model call).
- The flagship model reads: the snapshot's paragraphs; `review_criteria` from the skill;
  a compact graph summary (which claims/evidence/`word_budget_ok` exist). It returns a
  typed work-order `[{criterion_code, band, evidence, missing, fix}]` (fix optional).
- **Full enforcement stack** on the output: output-check (full-width 。！？),
  banned-phrasing, authorship. A rejected output persists **nothing** and streams no
  items (silence-legal), same discipline as the coach.
- Persist each surviving item as one `intervention`: `type:"review_item"`, `criterion` =
  `"表E 分析"`-style label, `body` = evidence + missing, `level` = band, `anchor` =
  `{kind:"draft_snapshot", id:sid}`, `output_check_verdict` set. Append a review event.
  Record the LLM call via the `RecordLLMCall` seam (档位/token/成本, per AGENTS.md).
- Ordering a review satisfies the S5 **human** gate item `whole_draft_review` (the
  student has ordered and can now disposition).

**`citations_matched` attestation — a new student_written write path.** The gate_state
*storage* exists (`UpsertGateState` writes a `RecordedGate` whose `Items` map holds
per-item `"solid"`), but **nothing currently records a student_written item as solid** —
the planner's `Advance` explicitly never marks non-machine items (`planner.go:117`,
`gate.go:200` reads `rec.Items[name] == "solid"` but no writer sets it). So the keystone
adds a **minimal, no-model** endpoint **`POST /projects/{id}/gate/{contractId}/attest`**
body `{item, confirmed}` (project-owner gated) that upserts the gate_state `Items[item]`
to `"solid"`/`""` via the existing `UpsertGateState` storage, restricted to the
contract's own `student_written` item names (rejects any other name). S5 uses it for
`citations_matched`; the mechanism is general to student_written items.

## 6. Contracts + projection

**Zod** (`packages/contracts/src/studioState.ts`) — new **required** `writing` field on
`StudioProjection`:

```ts
export const WritingSnapshot = z.object({
  id: z.string(), seq: z.number().int(),
  committedAt: z.string(), wordCount: z.number().int(), inBand: z.boolean(),
});
export const WritingReviewItem = z.object({
  interventionId: z.string(), criterion: z.string(), band: z.string(),
  evidence: z.string(), missing: z.string(), fix: z.string(),
  disposition: z.object({ action: z.enum(["accept","reject","rewrite"]),
                          reason: z.string() }).nullable(),
});
export const WritingProjection = z.object({
  buffer: z.string(),
  latestSnapshot: WritingSnapshot.nullable(),
  wordBudget: z.object({ min: z.number().int(), max: z.number().int() }),
  citationsMatched: z.boolean(),
  review: z.object({ ordered: z.boolean(), items: z.array(WritingReviewItem) }),
});
```

**Go DTO** (`studio/dto.go`) parity + `projectWriting` (`studio/projection.go`): reads
`edit_buffer`, the latest `draft_snapshot`, the `review_item` interventions ⋈ their
`disposition`s, the `citations_matched` gate item, and `word_budget` from the loaded
skill. `dto_parity_test.go` extended (top-level `writing` key + nested key sets), same
discipline as `structure` in 7b.

## 7. Frontend — the live 写作 view

- `WritingView` binds the **editable** textarea to `writing.buffer`; `onChange` →
  debounced `PUT /buffer`. Silent: the coach rail emits nothing while editing.
- A **commit** control (from the buffer, or a paste) → `POST /snapshots` → switches to
  preview. `snapshotMeta` renders "第 N 版快照 · 提交 · 只读" from `latestSnapshot`.
  Preview renders the snapshot paragraphs (design markup verbatim).
- **整稿体检** is enabled once a snapshot exists → SSE review → renders the **work-order
  block** verbatim to the design: per item the criterion label, band chip, evidence /
  missing line, and `fix` with the 三键 保持原样 / 我来改 / 说明为什么不改.
- The three keys post to the **existing** disposition endpoint (mapping per decision 7);
  reason ≥15 runes, into 成长记录. A `citations_matched` attestation control writes the
  student_written gate item.
- `StudioContainer.toStudioState` un-stubs `writing: p.writing`; the stale
  deferred-views comment updated.

New frontend files: `api/writing.ts` (buffer PUT, snapshot POST, review SSE client),
`studio/writing.ts` (view controller state if needed). `WritingView` props change from
`{draft, mode}` to the `WritingProjection` shape; `state.ts` adopts the contract type.

## 8. Testing

- **Go `writing_test`** (real Postgres): buffer upsert idempotency; snapshot `seq`
  monotonic + content immutable; `word_budget_ok` minted in-band and **absent/removed**
  out-of-band; `CountWords` unit table (CJK + latin + mixed + empty).
- **Go `order_review` e2e** (real Postgres): commit → review → asserts N `review_item`
  interventions with band/criterion/anchor; an enforcement-rejected (banned-phrase)
  output persists nothing and streams nothing; a second review POST returns the same
  rows (idempotency, no new model call); an `order_review` verb round-trips the
  enforcement stack.
- **Gate reconcile:** after an in-band commit + a review, `ReconcileGates` reports S5
  machine `word_budget_ok` present and the `whole_draft_review` human item available;
  `citations_matched` flips on attestation.
- **contracts:** `WritingProjection` parses valid, rejects a bad `disposition.action`
  and a missing `writing` on `StudioProjection`.
- **web:** edit→commit→preview→review→disposition render; buffer autosave debounce;
  `StudioContainer` wiring; a review-block-hidden-until-snapshot guard.

## 9. Acceptance

A student writes in the buffer (silent), commits an in-band snapshot, orders a whole-
draft review, sees the per-criterion work order, dispositions an item with a reason,
attests citations matched — and the S5 gate reconciles (`word_budget_ok` machine +
`whole_draft_review` human + `citations_matched` student_written). RL-1 holds: no path
writes prose on the student's behalf. This is the S5 loop working end-to-end in the real
product code, proven by a real-Postgres e2e crossing commit → review → disposition.

## 10. Carry-forwards

- **8b:** examiner-voice switching + 3 generic voices; board-specific passes (EE
  density, AP org×connection); richer budget-deletion prompts.
- **Slice 9:** the five-skin readiness gauge (the review's criteria labels here are the
  minimal seam).
- **Slice 10:** automated citation front-to-back matching (keystone = attestation);
  multi-snapshot version history + snapshot diffs feeding the process record.
- Inherited, untouched: search-plan card design (S2); the perspective map (graph-backed,
  Slice 7 family); stored `event` rows keep `type`/`surface` as DB columns while the Zod
  variants are flat (Slice 10 assessor must merge before validating); project-scope
  `GetCardInstance`; unique index on `chat_thread.seeded_project_id`; onboarding live
  producer; live `gate` passed/total counts.

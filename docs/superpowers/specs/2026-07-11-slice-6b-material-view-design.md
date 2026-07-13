# Slice 6b — Material view, project-scoped ingestion, source log

**Status:** approved (2026-07-13) · **Depends on:** Slice 6 (CRAAP fill→mint), Slice 5d (routing cutover)
**Roadmap line:** Slice 6 (S2/S3) — "material center-pane view + project-scoped ingestion + source-log S2"

---

## 1. Goal

The 素材 center pane stops being a stub. After 6b:

1. The **source dossier renders the project's real materials** (not a frontend fixture), with each
   source's state derived from what the student actually did.
2. The student can **add a source themselves** — paste a URL (fetched + segmented) or paste the text.
   The AI has no ingestion path at all.
3. Every source is **logged** (URL, title, student's one-line takeaway, tier, time spent), shown as a
   检索日志 ledger under the dossier.
4. The live CRAAP card's **anchors highlight in the article body**, lighting up the seam Slice 6 T7
   landed dormant (`SourceDossier` already accepts `anchors`; nothing passes them).

---

## 2. Current state (what exists, what's stubbed)

| Unit | State |
|---|---|
| `SourceDossier.tsx` (list + article/summary read view, `Annotate` spans, `anchors` prop, `onEvent` seam) | **Built.** Fed by a fixture; `anchors` and `onEvent` are never passed. |
| `StudioContainer.toStudioState` | `views.material: []` — hard stub. |
| `ViewFrame` | Renders `<SourceDossier sources={state.views.material} />`; passes no `anchors`, no `onEvent`. |
| `StudioShell` | Already holds the live `card` (with `anchors`) and passes it to `CoachRail` only. |
| `internal/materialize` (`FetchReadable` SSRF-guarded + `Segment`) | **Built, no caller.** Preserved through 5d for exactly this slice. |
| `CreateProjectMaterial`, `ListMaterialsByProject` | Queries exist; `CreateProjectMaterial` has no production caller. |
| `source_log_entry` table (migration 0016) | **Table exists, zero queries, zero writers.** |
| `AppendEvent` query | Exists; no HTTP route reaches it. |
| Demo seed (`0018`) materials | Two rows with `blocks: '[]'` — **empty**. The real article text lives only in `apps/web/src/studio/material/fixtures.ts`. |
| CRAAP mint (`GraphEffects`) | Mints an `evidence` node with `body.source_quality` (per-dimension answers + `risk_note`) and an `evaluated-as` edge `material → graph_node`. |

---

## 3. The projection: `materials[]` on the wire

`studio.Load` additionally loads `ListMaterialsByProject` and `ListSourceLogByProject`.
`studio.Project` emits a new top-level `materials` array on `StudioProjection`.

```ts
// packages/contracts/src/studioState.ts
export const MaterialSource = z.object({
  id: z.string(),
  title: z.string(),
  sourceUrl: z.string(),            // "" for pasted text
  kind: z.string(),                 // material.kind: "article"
  origin: z.string(),               // material.source: "fetched" | "pasted"
  blocks: z.array(z.object({ id: z.string(), text: z.string() })),
  locked: z.boolean(),
  role: z.string(),                 // 作用与风险; "" when unwritten
  tier: z.string(),                 // student-supplied at ingestion; "" if none
  takeaway: z.string(),             // student-supplied at ingestion; "" if none
  anchors: z.array(Anchor),         // persisted card_instance.anchors for this material
});
```

**Derivation rules — every field has a real producer, nothing is invented:**

| Field | Derived from |
|---|---|
| `locked` | An `evaluated-as` edge exists with `from_kind='material'`, `from_id=<material.id>`. That edge is minted only when the CRAAP card completes (`GraphEffects`). |
| `role` | The evidence node that edge points to: `body.source_quality.risk_note`. Absent → `""`. |
| `tier`, `takeaway` | The material's `source_log_entry` row (student wrote them at ingestion). Absent → `""`. |
| `anchors` | Every `card_instances.anchors` entry across the project whose own `material_id` equals this material's id. (`card_instances` has no `material_id` column — the anchor object carries it.) |

**Deliberately NOT produced.** The binding design's `偏弱` chip (`思维印记_工作区.dc.html:2196`) and the
current fixture's `可信 / 存疑` chip have **no honest producer** — both are a verdict *on* a source, which
only an AI could fabricate here. They are not rendered. The chip is state-derived and binary, matching the
design's own state machine: `待评估` → `✓ 已锁定`. Carry-forward: if a verdict chip is wanted, it must come
from a student judgment field on the CRAAP card, designed on purpose.

**Display fallbacks (binding copy, from the design):**
- `role == ""` → render `尚未写「作用与风险」` (design `:2195`).
- `locked` → chip `✓ 已锁定` (green, design `:2227`); else chip `待评估` (design `:2195`).

---

## 4. Ingestion — student-only, by construction

**`POST /api/v1/projects/{id}/materials`** (protected, ownership-checked via `loadOwnedProject`).

```jsonc
// one of:
{ "url": "https://…",  "takeaway": "…", "tier": "一手数据" }
{ "title": "…", "text": "…", "takeaway": "…", "tier": "二手 · 需追源" }
```

- **URL path:** `materialize.FetchReadable(ctx, url)` → `(title, text)`; `materialize.Segment(text)` → blocks.
  `source = "fetched"`, `source_url = url`. A fetch error (bad scheme, blocked IP, non-HTML, bad status,
  empty extraction) returns an **honest 400** — never a material with empty blocks — via the existing
  `httpx.ErrBadRequest("fetch_failed", "取不到这个链接的正文，可以直接把正文粘进来。", nil)`. (No 422 helper
  exists in `httpx`; 6b reuses the established 400 + stable-code envelope rather than inventing one.)
- **Text path:** `Segment(text)` → blocks. `source = "pasted"`, `source_url = ""`. Empty segmentation →
  `httpx.ErrBadRequest("empty_body", "正文是空的。", nil)`.
- **Both paths, one transaction:** `CreateProjectMaterial` + `CreateSourceLogEntry`. If either fails, neither
  row lands — a material with no log entry would be a source that was never "opened", which is precisely the
  state RL-2 forbids.
- `task_id`: the legacy FK. Migration 0020 drops its NOT NULL on `material` and the handler writes NULL — see
  §8, Risk 1.
- Returns `201` with the new `MaterialSource` DTO (same shape the projection emits).

**Why no agent tool.** The AI is given no ingestion capability — not a discouraged one, an absent one. This is
RL-2's ingestion half (*"the AI supplies no ready-made material"*) enforced structurally rather than by prompt,
and 铁律 1 (AI 克制) applied to research: the student searches, themselves.

**Frontend.** A 添加信源 affordance in the dossier header opens an inline form: a URL field *or* a
paste-the-text field, plus 一句话摘要 (required) and 层级 (single choice). On success the dossier refetches
the project and shows the new source. The design has no add-affordance (it shows sources already collected),
so this is a small extension in the design's own idiom — the same card/rounded-input vocabulary.

---

## 5. The source log

**New queries** (`internal/store/queries/source_log.sql`):

- `CreateSourceLogEntry` — `(project_id, url, title, takeaway, tier)`; `opened_at` defaults to now,
  `time_spent_s` starts at 0.
- `ListSourceLogByProject` — ordered by `opened_at`.
- `AddSourceTimeSpent` — `UPDATE source_log_entry SET time_spent_s = time_spent_s + $2 WHERE id = $1` (accumulates).

The log entry is keyed to its material by URL for `fetched` sources; for `pasted` sources the URL column holds
`""`. To make the join unambiguous, **migration 0020 adds a nullable `material_id uuid REFERENCES material(id)
ON DELETE CASCADE` to `source_log_entry`** — the existing columns stay untouched (the table has never been
written to, so there is no data to migrate).

**`POST /api/v1/projects/{id}/materials/{mid}/open`** with `{ "time_spent_s": 42 }`:
- `AddSourceTimeSpent` on that material's log entry, and
- `AppendEvent` a `source_opened` event (`{surface:"studio", url, time_spent_s}`) — the first HTTP path that
  reaches the `event` table, which Slice 10's assessor reads.
- Best-effort semantics: a logging failure returns 204 anyway and warns. Losing a timing sample must never
  break a student's reading.

**Client.** `SourceDossier` already emits `source_opened` through its unwired `onEvent` seam. 6b wires it: the
dossier starts a timer when a source opens and posts the elapsed seconds when the student goes back to the list
(or the component unmounts). Time is measured client-side; the server only accumulates.

**检索日志 panel.** Below the dossier list: one row per log entry — title/URL, 停留 Nm, the student's 一句话摘要,
and the tier chip. Header: `检索日志 · 已记录 N 条`. This is the ledger, and it is the visible half of the
promise that the student did the finding.

---

## 6. The anchor highlight seam

`StudioShell` already receives the live `card` (which carries `anchors`) and threads it to `CoachRail`. 6b also
threads it to `ViewFrame`, which passes `anchors` to `SourceDossier` — the prop it has accepted since Slice 6 T7
and that nothing has ever passed.

Two anchor sources merge in the dossier, and they must not fight:
- **Live** — `card.anchors` from the open CRAAP card (the questions the student is answering right now).
- **Persisted** — `material.anchors` from the projection (a completed card's answered anchors, so highlights
  survive a reload).

Rule: the live card's anchors **win** for the material they target (same `material_id`), because they carry the
student's in-progress answers; the projection's anchors render for every other material. `SourceDossier`'s
existing `anchorToSpan` already filters by `material_id` and drops source-level anchors (`risk_note`), so the
merge is a `Map` keyed by anchor id, live-first — no renderer change.

---

## 7. The seed

Migration `0020` (idempotent, `ON CONFLICT DO NOTHING` / targeted `UPDATE`):

1. Adds `material_id` to `source_log_entry` (§5).
2. **Fills the demo project's two materials with their real blocks.** The blog article (《卫星图看中国变绿》,
   3 paragraphs) and the NASA / Nature Sustainability finding currently live in
   `apps/web/src/studio/material/fixtures.ts`. That content moves into the seed verbatim — it is real content
   (Chen et al. 2019, Nature Sustainability, DOI 10.1038/s41893-019-0220-7), per AGENTS.md's no-lorem-ipsum rule.
   Today those rows carry `blocks: '[]'`, which means the CRAAP anchor generator has been running against
   **empty text**.
3. Seeds a `source_log_entry` per seeded material so the demo project's 检索日志 isn't empty on first load.

`apps/web/src/studio/material/fixtures.ts` and its test are then **deleted**; `SourceFixture` is replaced by the
contract's `MaterialSource` throughout. No orphan may remain.

---

## 8. Risks

**Risk 1 — the legacy `task_id` NOT NULL FK.** `material.task_id` is still NOT NULL (Slice-3 debt; the task
*surface* is gone but the column isn't). A live ingestion handler must supply one. Options considered:
(a) drop the NOT NULL in 0020; (b) reuse the seeded anchor task. **Decision: (a) — drop the NOT NULL.** The
carry-forward is explicitly unblocked (5d retired the task surface), reusing a seeded anchor task in a
production write path would be a lie that outlives this slice, and every real project after project-creation
lands will have no task at all. 0020 sets `material.task_id` nullable and the ingestion handler writes NULL.
`CreateProjectMaterial` takes `task_id` as `pgtype.UUID` already. *(Scope note: this drops the constraint on
`material` only — `card_instances.task_id` stays NOT NULL and remains a carry-forward.)*

**Risk 2 — fetching arbitrary URLs from the server.** `materialize` already guards this (scheme allow-list,
blocked-IP guard against loopback/link-local/private ranges, non-HTML rejection, status check, size limits) and
is unit-tested. 6b adds no new network capability; it wires an audited one. The handler must be behind
`HasEntitlement` like every other token/network-consuming endpoint.

**Risk 3 — a stubbed projection passing its own test.** The `locked` / `role` derivation is the slice's real
claim. Its test must **fail against a hard-coded projection** — assert against a material whose CRAAP card was
completed *through the real mint path* (surface → fill → submit → `CompleteCard`), and check that `role` equals
the `risk_note` the test wrote. See §9.

---

## 9. Testing strategy

**Go (testcontainers, `-p 1`):**
1. `POST /materials` with a URL served by an `httptest` server → 201; a `material` row with segmented blocks
   (>1 block for a multi-paragraph body) and a `source_log_entry` row with the student's takeaway + tier.
2. `POST /materials` with a URL the fetcher rejects (loopback / non-HTML) → 400 `fetch_failed`, **and neither
   row exists** (transaction proof).
3. `POST /materials/{mid}/open {time_spent_s: 30}` twice → `time_spent_s == 60` (accumulates, not overwrites)
   and two `source_opened` events in `event`.
4. **Projection derivation (the non-vacuous one):** seed a project + material → surface a CRAAP card → submit
   answered anchors incl. a `risk_note` → `GET /projects/{id}` → the material comes back `locked: true` with
   `role` equal to the submitted risk-note text and non-empty `anchors`. A second, un-evaluated material in the
   same project comes back `locked: false, role: ""`. *This must fail if `locked`/`role` are hard-coded — the
   implementer proves it by reverting the derivation to a constant and watching it go red.*
5. Ownership: another user's project id → 404 on both new endpoints.

**Frontend (vitest):**
6. `StudioContainer` maps the projection's `materials` into `views.material` (no fixture import survives
   anywhere in `src/`).
7. The add-source form posts `{url, takeaway, tier}` and renders the returned source in the list; a 422 renders
   the honest error copy, not a blank source.
8. Opening a source then going back posts `time_spent_s > 0` to the open endpoint.
9. The live card's anchors highlight spans **only** in the material they target — a second source in the list
   shows none. (Break the `material_id` filter → test fails.)
10. A material with `role: ""` renders 尚未写「作用与风险」; a locked one renders `✓ 已锁定`.

**Contracts:** `MaterialSource` round-trips; `dto_parity_test.go` extended so the Go DTO and the Zod schema
cannot drift.

---

## 10. Acceptance

1. A student with the seeded project opens 工作室 → S3 信源评估 → sees **two real sources** with real article
   text, both 待评估 (neither has a completed CRAAP card yet), and a 检索日志 with two entries.
2. She clicks 添加信源, pastes a URL, writes a 一句话摘要, picks a 层级 → the source appears in the dossier and
   in the log, with the article's real paragraphs readable.
3. She opens the blog, the coach surfaces CRAAP, and the anchored spans **light up in the article on the left**
   while she answers them on the right.
4. She locks the card → the source's chip flips to `✓ 已锁定` and its 作用与风险 line shows what *she* wrote.
5. Nothing in the running system reads `apps/web/src/studio/material/fixtures.ts`, because it no longer exists.

---

## 11. Carry-forwards (out of scope, named)

- **The search-plan card** (`search plan → the AI questions the plan → the student searches`) — no card exists;
  "the AI questions the plan" is a new coach behavior needing its own design. Not in 6b.
- **RL-2's citation half** (citations addable *only* from the log) — there is no citation surface until Slice 8
  (写作). The log 6b builds is the data that enforcement will read.
- **The 偏弱 verdict chip** — no honest producer (§3).
- **The S2 perspective map** (视角与素材, design `:930-956`) — graph-node-backed with its own gate; belongs with
  Slice 7's graph work.
- **Lateral-read flag** (`source_log_entry.lateral_read`) — written by SIFT (6c), not 6b.
- **`card_instances.task_id` NOT NULL** — still Slice-3 debt; 6b drops it on `material` only (§8).

# S2 · Reading sub-agent contract (brief-in / takeaways-out)

> Build spec for **S2** of the multi-agent architecture (`2026-07-29-multi-agent-architecture-design.md` §3, §6, §11).
> Goal: formalize the reading room as the reusable **context-isolated sub-agent** — a purposeful **brief-in** at entry, and a compact **takeaways-out** rollup that lands on the reading-list entry and folds one line into the spine. Plus a per-source `phase_tag`.

## 0. What exists (verified) — and the decision

The reading room is **already structurally isolated**, so S2 is *not* "move the conversation out." It is adding the two missing halves of the §6 contract.

- **Isolated read-together loop already exists.** `ReadingRoom.tsx` → `POST …/materials/{mid}/read-turn` (`readturn.go:60`, SSE) runs `agent.RouteReading` (`reading_router.go`) + `ApplyReadingGate` (`reading_gate.go`) grounded **only on that material's blocks**. It never enters the main continuous thread. The main thread's spine projection (`buildSpineProjection`, `projectcoach.go`) reads only the reading-*list index* (title + decision + credibility), never article bodies. The "which sources to read" chat is the Library `FloatingCoach` on the main thread (`/coach scope=find_sources`) — correctly the other side of the boundary.
- **The 3-check verdict already produces per-selection 5-field evals.** `evaluateProjectCard` (`readeval.go:85`) → `agent.EvaluateSelection` (`reading_eval.go:74`) → `SelectionEval{Verdict, VerdictLabel, VerdictReason, Checks, Finding, Judgment, Support, Caveat, NextStep, SpanIDs}` (`reading_eval.go:16-21`). Verdict is **program-owned** from 3 checks (`verdictFromChecks`, `reading_eval.go`), never trusted from the model. Confirmed via `submitProjectCard`; confirmed findings accumulate in 阅读成果 (`ReadingOutcomes.tsx`).
- **What does NOT exist:** a brief-in struct, a per-source rolled-up takeaway object, `phase_tag`, or any endpoint that returns the sub-agent's takeaways. The reading-list `reference` row (`0036_workspace_redesign.sql:46-67`) has `credibility / evaluation / decision / material_id` but no `phase_tag / takeaway / takeaway_finalized_at`.
- **The isolation template to mirror:** the assessment sub-agent — `BuildAssessmentInput` (pure digest, `assess_input.go:68`) → `AssessReport` (ONE isolated flagship call, `assess_report.go:379`) → `RecordLLMCall` + persist (`assessment.go:64-125`). S2's finalize path follows the same **pure-input-builder → one isolated call → compact struct → meter+persist** shape.

**Decision:** S2 = (a) a **brief-in** captured/pre-filled at source entry, persisted on `reference`, injected into every read-turn so the coach's questions are purposeful; (b) a **takeaways-out** finalize path — record fields *assembled* from the student's confirmed work, synthesis fields *authored by the student* (AI may seed), written to `reference` and folded into the spine; (c) a **source lifecycle** (`未读 → 在读 → 已归纳`, finalize optional/non-blocking/updatable). No SSE rewrite of the read-turn loop; no rabbit-hole tree (S3).

## 1. Source lifecycle (the frame)

Each `reference` with a `material_id` has a derived state:

- **未读** — no `material_id` (never entered the room).
- **在读** — `material_id` set, `takeaway_finalized_at IS NULL`. The "exited partway" state. Reading is resumable (all card instances / outcomes persisted); nothing is fabricated; the spine still projects partial progress.
- **已归纳** — `takeaway_finalized_at IS NOT NULL`. Takeaway landed on the reference; `proposal_impact` folded into the spine.

**Finalize is per-source, optional, non-blocking, and updatable.** Re-entering a 已归纳 source resumes the room; re-finalizing **supersedes** (UPDATE, not first-open-wins — reading is iterative, deliberately unlike S1's immutable proposal prose). A source may live in 在读 indefinitely; phases are moments, not gates (§7).

## 2. Data (migration 0039)

Additive columns on `reference` (the reading-list entry the projection already reads):

```sql
ALTER TABLE reference ADD COLUMN reading_reason        text;         -- brief-in: why read THIS source (student-authored, pre-filled)
ALTER TABLE reference ADD COLUMN reading_focus         text;         -- brief-in: optional student-stated points
ALTER TABLE reference ADD COLUMN phase_tag             text;         -- which project moment this source serves (enum §5)
ALTER TABLE reference ADD COLUMN takeaway              jsonb;        -- takeaways-out: the 5-field object (§4), NULL until finalized
ALTER TABLE reference ADD COLUMN takeaway_finalized_at timestamptz;  -- drives 在读 vs 已归纳
```

No CHECK on `phase_tag` (values validated in the contracts layer, per house style — Go validates the envelope, Zod owns the enum). The legacy `source_log_entry.takeaway` (single string, time-on-source ledger) stays untouched; the S2 object is the new structured `reference.takeaway`.

## 3. Brief-in

`{ project_scope, qualification, reason_for_reading, proposal_snapshot, reading_focus? }` (§6).

### 3a. Capture at entry (`workspace_library.go`)
- `enterReading` (`workspace_library.go:495`) already mints the `MaterialSource`. Extend its response with a deterministic **`suggested_reason`** — templated from the proposal objective (`GetProjectProposal`) + the reference title, **no LLM call**. The client shows it as an editable one-liner and a `phase_tag` single-choice (default guessed from the reference's `decision`/`classification`, student overrides).
- New endpoint **`PUT /projects/{id}/references/{rid}/reading-brief`** `{ reading_reason, reading_focus?, phase_tag }` → persists on `reference`. Editable any time (banner in the room). 克制: the student authors the intention; the AI only *seeds* the default text.

### 3b. Injection into the read-turn (`readturn.go` + `reading_router.go`)
Extend `ReadingRouteInput` (`reading_router.go:24`) with a small brief block:
```go
Brief struct {
    Reason       string // reference.reading_reason
    Focus        string // reference.reading_focus
    PhaseTag     string // reference.phase_tag (label)
    ProposalSnap string // relevant proposal dims, one line — from GetProjectProposal
}
```
`postReadingTurn` (`readturn.go:237`) fills it from the reference's stored brief + the spine proposal. The router system prompt uses it so questions become purposeful ("你在找的是反驳还是印证？") instead of generic. Deterministic gate (`ApplyReadingGate`) is unchanged. A missing brief (legacy/blank) degrades to today's behavior.

## 4. Takeaways-out (finalize)

Compact object written to `reference.takeaway`:
```jsonc
{
  "findings":        [ "…" ],                 // assembled: confirmed SelectionEval.Finding (this material)
  "credibility":     { "verdict": "…", "why": "…" }, // assembled: CRAAP/SIFT program verdict + reason
  "key_quotes":      [ { "quote": "…", "why": "…" } ], // assembled: student-picked spans + per-selection why
  "new_leads":       [ "…" ],                 // student-authored (AI may seed) — rabbit-hole branches
  "proposal_impact": "…"                      // student-authored (AI may seed) — how this moves the argument
}
```

**Split-hybrid authorship.** Record fields (`findings / credibility / key_quotes`) are the student's already-confirmed work — assembled server-side, **no AI re-guess**. Synthesis fields (`new_leads / proposal_impact`) are the genuinely-new thinking — **the student authors them**, the AI only seeds a suggestion she overwrites.

### 4a. Draft (AI seed) — `GET /projects/{id}/references/{rid}/takeaway-draft`
- Assemble record fields deterministically from this material's confirmed card instances (`SelectionEval` via `card_instance.framework`, status confirmed/submitted) + `key_quotes` from picked `SpanIDs` → block text.
- ONE isolated compose call (mid-tier `ChatResolver` — this **organizes student content, it does not grade**; downgrade-allowed) producing **suggested** `new_leads` + `proposal_impact`, constrained by a 克制 system prompt: *rephrase/organize only, never add a conclusion the student did not reach.* Meter `Purpose="reading_takeaway_draft"`.
- Returns `{ findings, credibility, key_quotes, suggested_new_leads, suggested_proposal_impact }`. No persist, no `finalized_at`.
- Mirror of the assessment template: pure input digest (`buildReadingTakeawayInput`) → one isolated call (`agent.ComposeReadingTakeaway`) → compact struct.

### 4b. Finalize — `POST /projects/{id}/references/{rid}/finalize-reading`
- Body `{ new_leads, proposal_impact }` (the student's final words, seeded or rewritten). Record fields are **re-assembled server-side** at finalize (source of truth = the confirmed outcomes, not the client).
- Persist the full object → `reference.takeaway`, set `takeaway_finalized_at = now()` (UPDATE — supersedes on re-finalize). **No LLM call** (student already authored the synthesis). The one-line `proposal_impact` is now on the reference and thus in the projection (§6).
- Best-effort, non-blocking. Append an activity-log entry ("归纳了《…》"). Idempotent update.

## 5. phase_tag

Small enum, agent-proposed-at-entry / student-overridable, owned by the contracts layer:

`立题探索 · 背景理解 · 支持论点 · 反例检验 · 方法参考`

Purpose: tags *why* a source is in the project — feeds S3's phase-tagged source graph / rabbit-hole tree. In S2 it is captured, shown as a chip in the Library, and carried in the projection. Default guess at entry from `reference.decision`/`classification`; the student's pick wins.

## 6. Spine projection (`projectcoach.go` — extend the 文献库 block)

Today `buildSpineProjection` lists each reference as `title ｜decision ｜可信度`. Extend per state:
- **已归纳** → `title ｜<phase_tag> ｜印记：<proposal_impact one-liner>` (the durable result the main agent should carry).
- **在读** → `title ｜<phase_tag> ｜在读·已确认 N 条发现`（latest finding truncated）— so the main agent isn't blind to in-progress reading and can gently offer to help finalize (never blocks).
- **未读** → unchanged (`title ｜decision`).

Also surface the structured `takeaway` + `phase_tag` on the `MaterialSource` wire (`projection.go:1187-1219` MaterialDTO / `packages/contracts/src/studioState.ts:111-161`) for the Library preview. Keep the existing `takeaway: string` field for back-compat; add the structured object + `phase_tag` alongside.

## 7. Frontend

- **Entry:** intention one-liner (pre-filled `suggested_reason`, editable) + `phase_tag` single-choice → `putReadingBrief`. Shown as a persistent banner in the room ("你读这篇是为了：…").
- **Reading room (`ReadingRoom.tsx`):** unchanged read-together loop; add a **完成这篇** action → finalize panel: read-only preview of assembled record fields + two editable synthesis fields (seeded via `getTakeawayDraft`) → confirm → `postFinalizeReading`. Source flips to 已归纳.
- **Library (`ReadingBlock.tsx`):** reference rows show a `phase_tag` chip + 在读/已归纳 state; Preview renders the structured takeaway when finalized.
- **api client (`workspace/api/workspace.ts`):** `getReadingBrief`(if needed)/`putReadingBrief`, `getTakeawayDraft`, `postFinalizeReading` + zod types.

## 8. Contracts (`packages/contracts`)

- `PhaseTag` enum (§5), `ReadingBrief`, `ReadingTakeaway` (5-field object), `TakeawayDraft` (assembled + suggested synthesis). Extend `MaterialSource` with `phaseTag?` + `takeaway?` (structured), keeping the legacy string field.

## 9. Tests + deploy

- **Go (api, testcontainers):** brief persist + `suggested_reason` templating; brief injection into `ReadingRouteInput` (fake provider asserts the brief reaches the router prompt); record-field assembly correctness (confirmed outcomes → findings/quotes/credibility); finalize persist + **re-finalize supersede**; projection 在读 vs 已归纳 lines; metering `reading_takeaway_draft` (draft spends, finalize does not).
- **agent:** `ComposeReadingTakeaway` 克制 enforcement (organizes, does not conclude) + `buildReadingTakeawayInput` digest.
- **contracts:** new zod types parse/round-trip.
- **web:** brief banner render; finalize flow (draft → edit synthesis → confirm → 已归纳).
- Full suites (contracts, `go ./...`, web), then deploy (0039 applies via `run --rm api -migrate-up`) + smoke: enter a source → brief persists → read-turn reflects purpose → finalize → takeaway on reference → projection shows 印记 line.

## 10. Explicitly deferred (not S2)

- **Rabbit-hole exploration tree / `branch` links / new_leads surfaced as a graph → S3.** S2 stores `new_leads` as data only.
- Cross-phase card proposing / compaction backstop → **S4**.
- Review finalization + AI-interaction retrospective → **S5**.
- Retiring the legacy `source_log_entry.takeaway` string or `prepareSourceAnnotation` dead path → cleanup, not now.

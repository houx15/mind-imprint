# Slice 4a — Essay research stage: the 证据地图 (the warren graph) + saturation gate · Design

> **Position:** the first sub-slice of slice 4 (写正文 sub-machine). Behavioral source of truth = `docs/2026-08-09-all-statuses.md §5 (reading, after proposal) + §6 stage-1 (research-complete)`. This builds the **first essay stage — research — in the reading room**. **Framing (user):** the warren graph (兔子洞地图) *is* the 证据地图 — it was built for this from the start. 4a does not add a parallel evidence panel; it grows the warren into the full evidence map: seed it from the proposal, type evidence as 支持/反驳, capture a per-paper note in the paper sidebar, triage papers, and review each sub-question's saturation. It also **relocates 完成提案 → reading/research** (correcting Phase B's direct proposal→essay-writing jump).

## One-sentence goal

After the proposal, the student goes to the reading room and grows their 证据地图 (the warren graph, seeded with their research question + its sub-questions): 印记 hands them **one task at a time** (start from sub-question 1), they gather and read papers, tag each paper's evidence 支持/反驳 under its sub-question and note what it argues + where it can appear; when they think a sub-question is done, 印记 reviews whether its evidence is **saturated** (strong support + a real challenge + diminishing returns) — advisory, then they write that claim or read on.

## 铁律 (carried)

1. **AI never does the thinking.** 印记 hands out tasks, proposes searches, and reviews saturation; the student reads, judges, tags, and notes. AI never fabricates a source or an evidence tag.
2. **No manipulation.** Saturation is strong-advisory — the student can write a claim or finish research even if 印记 thinks a sub-question is thin (recorded).
3. **One thing at a time** — 印记 walks the sub-questions one at a time (§5/§6); a saturation review reports on one sub-question when the student asks.
4. **Process is data** — searches, adoptions, triage, what was archived — all recorded.
5. **Never downgrade evaluation** — the saturation reviewer runs on **flagship reasoning** (`EvalResolver`); search-direction proposals run on the fast model.

## Design principle (why the graph is the map)

§6: research is *"the construction of a 证据地图, through comprehensive searching, reading materials and composing the structure between materials."* A pile of read papers isn't research; **organizing evidence against the questions it answers, seeing support vs challenge, and knowing where each question is still thin** is. The warren graph already models exactly this — question nodes, papers under them, typed relations (`子问题`/`支持`/`反驳·张力`), a per-node "文献 N 篇", 印记-proposes/student-confirms. 4a completes it into the evidence map and adds the reasoning that reads it.

---

## Pillar 1 · The essay track + relocating 完成提案 → research

### `EssayTrack` on `StudioState` (jsonb, no migration)

```go
type EssayStage string
const ( EssayResearch EssayStage = "research"; EssayStatement EssayStage = "statement"; EssaySubmission EssayStage = "submission" )

type EssayTrack struct {
    Stage EssayStage `json:"stage"` // 4a lands "research"; 4b statement; 4c submission
}
// StudioState gains: EssayTrack *EssayTrack `json:"essayTrack,omitempty"`
```

The **main RQ = `proposal.objective`; the sub-questions = `StudioState.ProposalTrack.SubQuestions`** (from slice 3a). No re-entry — the map is seeded from the proposal (Pillar 2).

### Relocating the flow (supersede Phase B)

Today 完成提案 → `advanceStatusTo("essay")` → `FlowEssay`/`StageBodyWriting` → the writing surface. Correct flow (§5/§6): **完成提案 → reading/research first.**
- On proposal finish, the server sets `EssayTrack.Stage = "research"` and the seed runs (Pillar 2); the `nextStep`/advance opens the **reading room** (`Surface: ToolReading`), not the writing surface.
- `nextStepFor(FlowProposal, …, finishPart)` changes from `{"开始写正文", FlowEssay, ToolWriting}` → `{"去做研究", FlowEssay, ToolReading}` (coach.go:690). `FlowEssay` stays the status; the reading room is a surface within it. The writing surface (大纲/片段/正文) is opened by the statement stage (4b).

---

## Pillar 2 · Seed the warren from the proposal (the map's backbone)

On entering research (proposal-finish, once, idempotent), the server seeds the warren so the student sees their own question structure drawn for them:

- A **root question lead** for the main RQ (`proposal.objective`), if not already present.
- A **root question lead per sub-question** (`ProposalTrack.SubQuestions`), each linked to the main RQ by a **confirmed `子问题` `question_edge`** (they come from the confirmed proposal, so confirmed not proposed).
- Idempotent: re-entry doesn't duplicate (match by text / a seeded marker on the lead).
- Reuse `exploration_lead` + `question_edge` (no new node/edge types). A new store helper `SeedEvidenceMapFromProposal(projectID)`; run from the proposal-finish/advance path.

Papers are gathered under each sub-question via the **existing dig/adopt flow** (connectedReferenceId), giving the "文献 N 篇" per sub-question and the Level-2 paper view — unchanged.

---

## Pillar 3 · Per-paper evidence: nature (支持/反驳), note (in the sidebar), triage

### Evidence nature — reuse the graph's own edge words

A paper under a sub-question is typed **支持** or **反驳·张力** — the same vocabulary the warren edges already use. This makes the map show, per sub-question, what backs it vs what challenges it, and is exactly what the saturation review reads. Stored on the paper's reference (Pillar 3 schema); shown as a small colored marker on the paper in the Level-2 list / sidebar. (Untagged = not yet judged.)

### Per-paper note — in the paper sidebar, never on the graph (user)

When a paper node is selected, `ExplorationSidebar`'s `PaperMeta` already shows title/authors/year/abstract/进入阅读室. 4a adds a **note section** there: the structured evidence note — **key argument** (what this source argues), **key evidence** (the data/finding), **placement** (where it can appear in the essay), plus the 支持/反驳 nature. The map stays clean; the note lives in the sidebar (§来源信息 comes from the reference metadata already shown).

### Triage — red / yellow

A student often searches a batch, then reads one by one. Each gathered-but-unread paper carries a **triage**: **red = 必读 (must-read)**, **yellow = 待定 (to-decide)**. Shown as a dot on the paper; set from the sidebar / results list. (Separate from the existing `reading_status` to_read/reading/done, which is about progress; triage is about priority/relevance before reading.)

### Archive / trim — interesting but not related

A paper that's "interesting but not really related" is **archived** (reuse the lead `pruned` status / a soft archive) — it leaves the active map but isn't deleted. An "archive" action in the sidebar.

### Schema (migration — extend `reference`)

```sql
-- migration 00NN_reference_evidence.sql
ALTER TABLE reference
  ADD COLUMN triage           text NOT NULL DEFAULT '',  -- ''|red|yellow
  ADD COLUMN evidence_nature  text NOT NULL DEFAULT '',  -- ''|support|challenge
  ADD COLUMN evidence_argument  text NOT NULL DEFAULT '',
  ADD COLUMN evidence_finding   text NOT NULL DEFAULT '',
  ADD COLUMN evidence_placement text NOT NULL DEFAULT '';
```

sqlc: `SetReferenceEvidence(id, nature, argument, finding, placement)`, `SetReferenceTriage(id, triage)`, `ArchiveReference(id)`. Contract `reading.ts`'s `Reference` gains the optional evidence fields. (The freeform `reading_note` column stays as-is.)

---

## Pillar 4 · One task at a time + search-direction proposals

- **One task at a time (§5/§6):** on entering research, 印记 does NOT throw a wall of per-sub-question search buttons. It proposes **one task** — "先从子问题 1 开始：找能支持/挑战它的材料" — and walks the sub-questions in order as each is dealt with. A lightweight "current research task" surfaced in the coach + a highlight on the active sub-question node.
- **Search directions (only for the current task):** for the active sub-question, 印记 can offer 2–3 keyword directions with a why (reuse `agent.ComposeExplorationGuide`, scoped to that sub-question + its current evidence) — but only for the one task, not all sub-questions at once.

---

## Pillar 5 · Saturation review + flexible writing advance

### The reviewer (flagship) — run when the student says a sub-question is done

New `agent.ReviewEvidenceSaturation(ctx, prov, resolved, in) (SubQuestionVerdict, usage, error)` for ONE sub-question, mirroring `ReviewFramework`. Input: the sub-question text + its papers with their nature/argument/finding. Output: `{ saturated bool, why string, gaps []string }`. The prompt judges the three things the user named:
1. **Enough strong, reliable material** that supports the claim.
2. **At least one** limitation / challenging material / substitute explanation / different perspective (支持 alone is not saturated — the 撞反例 ethos).
3. **The real test — diminishing returns:** new reading is largely repeating or echoing what's already gathered.

Playful loading line ("印记正在核对这条线的证据够不够扎实…"). `EvalResolver`.

### The gate + flexible advance (§6)

- `GET /projects/{id}/evidence-map` → `{ mainQuestion, subQuestions:[{ id, text, papers:[{ id, title, nature, triage, note? }], saturated? }] }` (the map projected from the seeded warren).
- `POST /projects/{id}/evidence-map/subquestions/{sqId}/review` → runs the saturation reviewer for that sub-question; returns the verdict (recomputed on demand, surfaced, not stored long-term).
- **Flexible writing (user):** a student can **write a claim as soon as its sub-question is ready**, OR **finish all research first**. So the research→statement advance is not one global gate:
  - Per-sub-question: once a sub-question is reviewed/ready, a "去写这条论点" affordance advances into the statement stage focused on that claim (4b wires the per-claim writing; 4a just records the sub-question as research-ready + offers the tap).
  - Whole: a "研究做完了，开始写作" tap sets `EssayTrack.Stage = "statement"` and opens the writing surface.
  - Both are student taps (铁律②); the saturation review only advises.

Metered `purpose="evidence_saturation"`. Best-effort: nil resolver/error → no verdict, the map + advance still work.

---

## Model routing

| Role | Model | What |
|---|---|---|
| saturation reviewer | **flagship** (`EvalResolver`) | judges one sub-question's evidence (support + challenge + diminishing returns) |
| current-task search directions | fast (`FastChatResolver`) | 2–3 keywords + why for the active sub-question |

---

## Mapping to existing code (reuse vs new)

**Reuse (the warren IS the map):**
- `exploration_lead` + `question_edge` + the dig/adopt flow + `WarrenMap` / `ExplorationView` / `ExplorationSidebar` — the graph, the papers, the Level-2 view, the paper sidebar.
- `ProposalTrack.SubQuestions` + `proposal.objective` — the seed source.
- `agent.ComposeExplorationGuide` — the current-task search directions.
- `ReviewFramework` pattern — the saturation reviewer mirrors it.
- The proposal→essay advance seam (`nextStepFor`, `WritingBlock.doFinishWriting`) — re-pointed to reading + research.

**New:**
- `agent.EssayTrack{Stage}` on StudioState (jsonb, no migration).
- Migration: `reference` gains triage + evidence fields; sqlc setters + archive; `SeedEvidenceMapFromProposal`.
- `agent.ReviewEvidenceSaturation` + `SubQuestionVerdict` (flagship).
- Endpoints: `evidence-map` (GET), per-paper evidence/triage/archive setters, `evidence-map/subquestions/{id}/review`, the current-task search-direction call, the research-ready / research-done advances.
- Web: `PaperMeta` gains the note section + nature + triage + archive; the active-sub-question highlight + current-task line; `evidenceMap.ts` client; the 完成提案→reading redirect.

---

## Doc consistency check (`all-statuses.md §5 + §6 stage-1`)

| Doc point | This slice | Verdict |
|---|---|---|
| 完成提案 → research in the reading room (not straight to writing) | Pillar 1 relocate | ✅ |
| Research initialized with the main question + 2–4 sub-questions | Pillar 2 seed from proposal | ✅ |
| The warren/graph IS the 证据地图 | Whole slice (grow the warren) | ✅ (user framing) |
| Each paper: 来源信息 / key argument / key evidence + tags (which sub-question, where it can appear) | Pillar 3 note (sidebar) + nature + placement; sub-question = the node it hangs under | ✅ |
| Support vs challenge structure | Pillar 3 支持/反驳 typing | ✅ |
| One task at a time (start from sub-question 1); not many buttons | Pillar 4 | ✅ |
| Tag new papers during exploration (red must-read / yellow to-decide) | Pillar 3 triage | ✅ |
| Archive interesting-but-not-related | Pillar 3 archive | ✅ |
| Saturation = strong support + limitation/challenge/substitute/perspective + new-reading-repeats | Pillar 5 reviewer (3 criteria) | ✅ |
| Reviewed when the student thinks it's done | Pillar 5 (student-triggered) | ✅ |
| Write one claim once its sub-question has enough, OR finish all research then write | Pillar 5 flexible advance | ✅ |
| AI proposes new keywords (per active sub-question) | Pillar 4 search directions | ✅ (current task only) |
| needs-resources box on the reading page | slice 5 (reading-page UI) | ⏭️ |

## Out of scope / follow-ups

- 4b: the statement stage (essay track: outline → per-claim argument paragraphs with PEE/toulmin/argument-map + 3b 批注 → synthesis → conclusion → 论证结构). The per-claim entry from 4a's "去写这条论点" lands here.
- 4c: submission (引言/conclusion/compose/polish loop + finish→review).
- Cross-cutting (in 4b): doc-key the 批注 storage so the essay gets its own 批注.
- Slice 5: reading-page needs-resources + search-guidance UI.
- Deeper auto-capture (reading a source in the reading loop auto-filling the evidence note) — 4a keeps the sidebar note manual; tighter capture later.
- No fancy layout editor, no submitting for the student, no gamification.

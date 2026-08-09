# Slice 4a — Essay research stage: the 证据地图 (evidence map) + saturation gate · Design

> **Position:** the first sub-slice of slice 4 (写正文 sub-machine) in the [status-machine architecture spec](2026-08-09-status-machine-writing-and-cards-design.md). Behavioral source of truth = `docs/2026-08-09-all-statuses.md §5 (reading, after proposal) + §6 stage-1 (research-complete)`. This builds the **first essay stage — research — which happens in the reading room**: the student reads sources and builds a **证据地图** (evidence tagged by sub-question + where it can appear), 印记 proposes search directions per sub-question, and a reasoning reviewer judges each sub-question's **saturation**; once all sub-questions are saturated, research is complete and the essay advances to the statement stage (slice 4b). It also **relocates 完成提案 → reading/research** (correcting Phase B's direct proposal→essay-writing jump).

## One-sentence goal

After the proposal, the student doesn't jump into writing — they go to the reading room to **build an evidence map**: for the main research question and its 2–4 sub-questions (carried from the proposal), they gather sources, and for each note the 来源信息 / key argument / key evidence and tag it to a sub-question + where it can appear; 印记 suggests where to search next and reviews when each sub-question has enough, well-structured evidence to write from.

## 铁律 (carried, hard constraints)

1. **AI never does the thinking for the student.** 印记 proposes *search directions* and *reviews* saturation; the student reads, judges, and records the evidence. AI never fabricates a source or an evidence entry.
2. **No manipulation.** The saturation review is strong-advisory — the student can proceed to writing even if 印记 thinks a sub-question is thin (recorded as process data). "研究够了" is the student's call, 印记's review is a suggestion.
3. **One thing at a time** for coach guidance; a saturation review legitimately reports on all sub-questions at once.
4. **Process is data.** Which sources the student read, what they tagged where, what they skipped — all recorded.
5. **Never downgrade evaluation.** The saturation reviewer runs on the **flagship reasoning** model (`EvalResolver`); coach search-direction proposals run on the fast model.

## Design principle (why an evidence map, not just notes)

`all-statuses.md §6` is explicit: research is *"the construction of a 证据地图, through comprehensive searching, reading materials and composing the structure between materials"*. The pedagogy is that a student who has read widely but left it as a pile of notes hasn't done the hard part — **organizing evidence against the questions it answers, and seeing where each question is still thin.** So the evidence map is structured by sub-question, each evidence entry carries *what it argues, what evidence backs it, and where in the essay it can appear*, and the saturation review is per-sub-question. The map is the deliverable of the research stage and the raw material the statement stage (4b) writes from.

---

## Pillar 1 · The essay track + relocating 完成提案 → research

### `EssayTrack` on `StudioState` (jsonb, no migration)

Mirroring `ProposalTrack` (3a), the essay gets a track — but its top dimension is the **stage** (§6's three), not a step index:

```go
type EssayStage string
const ( EssayResearch EssayStage = "research"; EssayStatement EssayStage = "statement"; EssaySubmission EssayStage = "submission" )

type EssayTrack struct {
    Stage EssayStage `json:"stage"` // 4a lands "research"; 4b adds statement; 4c submission
    // research-stage state is the 证据地图 (its entries live in their own table, Pillar 2);
    // saturation verdicts are computed on demand, not stored here.
}
// StudioState gains: EssayTrack *EssayTrack `json:"essayTrack,omitempty"`
```

The **sub-questions and the main RQ come from the proposal** — `StudioState.ProposalTrack.SubQuestions` (3a) and `proposal.objective`. The evidence map is initialized against them (§5: *"Initialized with the main question and 2-4 subquestions"*). 4b lets them be modified; 4a reads them.

### Relocating the flow (supersede Phase B)

Today 完成提案 → `advanceStatusTo("essay")` → `FlowEssay`/`StageBodyWriting` → the essay writing surface (大纲/片段/正文). Per §5/§6 + the architecture spec, the correct flow is **完成提案 → reading/research** first:

- On proposal finish, the server sets `EssayTrack.Stage = "research"` and the `nextStep` / advance opens the **reading room** (`Surface: ToolReading`), not the writing surface. (`FlowEssay` stays the status; the reading room is a surface within it — the map confirmed reading is a room, not a status.)
- `nextStepFor(FlowProposal, …, finishPart)` changes from `{"开始写正文", FlowEssay, ToolWriting}` to `{"去做研究", FlowEssay, ToolReading}` (coach.go:690). `WritingBlock.doFinishWriting`'s `advanceStatusTo("essay")` still fires; the difference is the surface the machine opens + the essay track landing in `research`.
- The essay **writing surface** (大纲/片段/正文) is what the statement stage (4b) opens; in 4a, entering FlowEssay in the `research` stage opens the reading room.

---

## Pillar 2 · The 证据地图 data model (`evidence_entry`, a migration)

An evidence entry is structured enough to warrant a real table (not jsonb): it is queried by sub-question, it is the statement stage's input, and it accretes as the student reads.

```sql
-- migration 00NN_evidence_entry.sql
CREATE TABLE evidence_entry (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id      uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
  sub_question_id text NOT NULL,          -- the ProposalTrack sub-question id it answers ("" = main RQ / unfiled)
  source_ref_id   uuid,                   -- optional link to a reference/material row (NULL = manual source)
  source_note     text NOT NULL DEFAULT '',-- 来源信息 (author/title/where it's from) when not a linked reference
  argument        text NOT NULL DEFAULT '',-- the key argument this source makes
  evidence        text NOT NULL DEFAULT '',-- the key evidence/data
  placement       text NOT NULL DEFAULT '',-- where it can appear in the essay (§6's second tag)
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX evidence_entry_project_subq ON evidence_entry (project_id, sub_question_id);
```

- **Tags (§6):** `sub_question_id` (which sub-question) + `placement` (where it can appear) are the two tags §6 names. `source_ref_id` links to a reading-room source when the student came from reading one; `source_note` holds free 来源信息 otherwise.
- sqlc: `ListEvidenceEntries(project_id)`, `InsertEvidenceEntry(...)`, `UpdateEvidenceEntry(id, …)`, `DeleteEvidenceEntry(id)`.
- Contract `packages/contracts/src/evidence.ts`: `EvidenceEntry = { id, subQuestionId, sourceRefId?, sourceNote, argument, evidence, placement }`.

### How entries are created

- **From the reading room** (primary): while reading a source, the student records an evidence entry (source auto-linked via `source_ref_id`, 来源信息 prefilled from the reference). This is the "each paper, note down 来源信息, key arguments, key evidence + tags" flow (§6).
- **Manually**: an "add evidence" affordance in the 证据地图 panel for a source read elsewhere (source_note free text).

*(4a MVP: a structured add/edit form for evidence entries, tagged to a sub-question + placement. Deeper auto-capture from the existing reading loop / hang-cards is a follow-up — flagged as an open question below.)*

---

## Pillar 3 · The reading-room 证据地图 panel + search-direction proposals

The reading room stays the same "cool" room (§5); 4a adds two things:

1. **The 证据地图 panel** — organized by sub-question. For the main RQ + each of the 2–4 sub-questions (from the proposal), a section listing its evidence entries (source · argument · evidence · placement) + an "add evidence" form. This is where the student builds and sees the map. A per-sub-question saturation indicator (from Pillar 4) shows how each is doing.
2. **Search-direction proposals (per sub-question)** — 印记 suggests 2–3 keywords with a "why" for a chosen sub-question (§5: *"AI would give 2-3 possible keywords with a why, student can search with one click"*). Reuse/extend the existing `agent.ComposeExplorationGuide` (`exploration_guide.go`), scoped to a sub-question + the current evidence for it. The `检索方向审视` (search-plan) card stays AI-side (per the slice-1 catalog decision).

*(The §5 "needs-resources box" on the reading page is slice 5's UI cleanup; 4a focuses on the evidence map + saturation. The needs-resources box already exists on the writing page from 3a.)*

---

## Pillar 4 · Saturation review + research-complete gate

### The reviewer (flagship)

New `agent.ReviewEvidenceSaturation(ctx, prov, resolved, in) ([]SubQuestionVerdict, usage, error)`, mirroring `ReviewFramework`. Input: the main RQ + each sub-question with its evidence entries. Output per sub-question: `{ subQuestionId, saturated bool, why string, gaps []string }` — is there enough, well-structured evidence to write this sub-question's argument, and if not, what's missing. Playful loading line ("印记正在核对每个子问题的证据地图…"). Runs on `EvalResolver`.

### The gate + advance

- `GET /projects/{id}/evidence-map` → `{ subQuestions:[{id,text, entries:[…], verdict?}], mainQuestion }` — the map + (optionally) the last saturation verdicts.
- `POST /projects/{id}/evidence-map/review` → runs `ReviewEvidenceSaturation`, returns the per-sub-question verdicts (not persisted long-term; recomputed on demand, like the framework verdict is surfaced-then-cleared).
- **Research complete** = the student taps "研究够了，开始写作" (a one-tap advance; 铁律②) → `EssayTrack.Stage = "statement"` and the machine opens the essay writing surface (which 4b builds out). The saturation review is *advisory* — it colors the indicators and 印记's nudge, but never blocks the tap. §6: *"finished when each subquestion is fully explored with a good 证据地图 structure (ai reviews it)"* — the review informs; the student proceeds.

Endpoints metered `purpose="evidence_saturation"`. Best-effort: nil resolver / error → no verdicts, the map + advance still work.

---

## Model routing

| Role | Model | What |
|---|---|---|
| saturation reviewer | **flagship** (`EvalResolver`) | reads the evidence map, judges per-sub-question saturation |
| search-direction proposals | fast (`FastChatResolver`) | 2–3 keywords + why per sub-question |

---

## Mapping to existing code (reuse vs new)

**Reuse:**
- `ProposalTrack.SubQuestions` + `proposal.objective` — the sub-questions + main RQ the map is built against (no re-entry).
- The reading room (`apps/web/src/studio/reading/*`, `ExplorationView`) — the 证据地图 panel is added within it.
- `agent.ComposeExplorationGuide` (`exploration_guide.go`) — extended/scoped for per-sub-question search directions.
- `ReviewFramework` pattern (`framework_review.go`) — the saturation reviewer mirrors it.
- The proposal→essay advance seam (`nextStepFor` coach.go:690, `WritingBlock.doFinishWriting`) — re-pointed to the reading room + research stage.

**New:**
- `agent.EssayTrack{Stage}` on StudioState (jsonb, no migration for the track).
- `evidence_entry` table (migration) + sqlc CRUD; contract `evidence.ts`.
- `agent.ReviewEvidenceSaturation` (flagship) + `SubQuestionVerdict`.
- Endpoints: `evidence-map` (GET), `evidence-map/entries` (POST/PATCH/DELETE), `evidence-map/review` (POST), a per-sub-question `evidence-map/search-directions` (POST); the research→statement advance.
- Web: `apps/web/src/api/evidenceMap.ts`; an `EvidenceMapPanel` in the reading room (per-sub-question sections + add/edit entry form + saturation indicators + search-direction chips); the 完成提案→reading redirect.

---

## Doc consistency check (against `all-statuses.md §5 + §6 stage-1`)

| Doc point | This slice | Verdict |
|---|---|---|
| 完成提案 → do research in the reading room (not straight to writing) | Pillar 1 relocate | ✅ |
| Research initialized with the main question + 2–4 sub-questions | Pillar 2/3 read from ProposalTrack | ✅ |
| Each paper: note 来源信息 / key arguments / key evidence + tags (which sub-question, where it can appear) | Pillar 2 `evidence_entry` fields | ✅ |
| Construct a 证据地图 | Pillar 3 panel organized by sub-question | ✅ |
| AI proposes new research keywords (per sub-question, 2–3 + why, one-click search) | Pillar 3 search-direction proposals | ✅ |
| AI reviews if each sub-question's 证据地图 is saturated | Pillar 4 saturation reviewer | ✅ |
| Finished when each sub-question is fully explored (ai reviews) → then statement | Pillar 4 gate + advance to statement | ✅ (advisory, student taps) |
| 检索方向审视 is AI-side | Pillar 3 (search-plan stays AI-side) | ✅ |
| needs-resources box on the reading page | deferred to slice 5 (reading-page UI) | ⏭️ |
| auto-capture evidence from the existing reading loop / hang-cards | 4a MVP = structured add/edit form; deeper auto-capture deferred | ⚠️ open question (below) |

## Open design questions for you (before I plan)

1. **Evidence-entry fields** — I proposed `{source (linked ref or free 来源信息), argument, evidence, placement, sub-question}`. Is that the right structure, or do you want more/fewer fields (e.g. a credibility/limitation note, a "how it advances the next part" like the proposal sub-question cards)?
2. **How entries are created** — 4a MVP is a structured add/edit form in the 证据地图 panel (tag to a sub-question + placement). Do you want it tied more tightly into the existing reading loop (e.g. reading a source → hang a card → it becomes an evidence entry), or is the standalone form fine for 4a with tighter integration later?
3. **Saturation criteria** — should the reviewer judge each sub-question purely on its evidence entries, or also consider counter-evidence / perspective diversity (a sub-question isn't "saturated" until it has both supporting and challenging evidence)? The latter is more faithful to the platform's 撞反例 ethos.
4. **The 证据地图 vs the existing warren/exploration graph** — the reading room already has a question-lead graph (兔子洞地图). Should the 证据地图 be a *new panel* (my proposal), or should it reuse/extend the warren so sub-questions and evidence live in one graph?

## Out of scope / follow-ups

- 4b: the statement stage (essay track: outline from RQ+sub-questions → per-claim argument paragraphs with PEE/toulmin/argument-map + 3b 批注 → synthesis → conclusion → 论证结构).
- 4c: the submission stage (引言/conclusion/compose/polish loop + finish→review).
- Cross-cutting (scheduled in 4b): doc-key the 批注 storage (currently proposal-only `type='proposal_annotation'`) so the essay gets its own 批注.
- Slice 5: reading-page needs-resources box + search-guidance box UI cleanup.
- No fancy layout editor, no submitting for the student, no gamification (铁律 carried).

# Slice 3a — Proposal-writing sub-machine (guide-step track + 9-part scaffold) · Design

> **Position:** the first half (3a) of slice 3 in the [status-machine architecture spec](2026-08-09-status-machine-writing-and-cards-design.md). Behavioral source of truth = `docs/2026-08-09-all-statuses.md §4 (4.Writing proposal)`. This is the **spine** of the proposal-writing sub-machine: the guide-step track (`WritingTrack`) + the proposal 9-part scaffold + free/guided modes + writing-page UI cleanup. **The 批注 annotation primitive (layered, colored teacher-style comments) is deferred to slice 3b** — here the "我写好了 (I'm done)" action uses the **existing reasoning reviewer** (suggestions surfaced in chat) as an interim; 3b upgrades it to anchored, colored annotations.

## One-sentence goal

Give the proposal document a **deterministic guide-step track**: before writing, the student picks free / guided; in guided mode, 印记 hands out a **colored guide card** for each of the 9 parts in turn (an AI-generated guiding question + an English example + two buttons "我依然有问题 / 我写好了"); the student writes each part themselves (铁律①), and advancing is the student's tap (铁律②).

## 铁律 (carried, hard constraints)

1. **AI never writes the body text** — a guide card gives only a guiding question + an example; the student writes. The example is an English demonstration for a **different** prompt, never the answer to the current one.
2. **No manipulation** — the student chooses free/guided and may switch mid-way (switching only changes `mode`, never discards written content); "我写好了 / finish" is never blocked by the review — insisting on advancing is recorded as process data.
3. **One question at a time** — a guide card shows only the current step.
4. **Process is data** — skipping the guide, writing free, ignoring suggestions all leave a trace.
5. **Never downgrade evaluation** — "我写好了" review runs on `EvalResolver` (flagship reasoning); coach guidance runs on `FastChatResolver` (fast model).

---

## Pillar 1 · The guide-step track (new data on `StudioState`)

`WritingTrack` is a **dimension** of each writable document (not a new status). This slice lands only the **proposal** track; the essay track is slice 4.

```go
// package agent
type WriteMode string
const ( ModeUnset WriteMode = ""; ModeFree WriteMode = "free"; ModeGuided WriteMode = "guided" )

type WritingTrack struct {
    Mode       WriteMode      `json:"mode"`                 // ""=not chosen yet; free / guided
    StepIndex  int            `json:"stepIndex"`            // current step in guided (0..len-1); free stays 0
    Started    bool           `json:"started"`              // guided: student tapped "开始写作" (saw the outline intro)
    StepGuides map[int]string `json:"stepGuides,omitempty"` // step → generated+cached guide-card JSON (avoids re-spend)
}
```

Attached as:

```go
type StudioState struct {
    // …existing fields…
    ProposalTrack *WritingTrack `json:"proposalTrack,omitempty"` // slice 3a; the essay track is slice 4
}
```

- **No migration**: `StudioState` is read/written whole as `studio_state` jsonb (`GetStudioState`/`SetStudioState`), so a new field is persisted automatically and exposed via `GET /projects/{id}/studio-state`.
- The frontend read schema `packages/contracts/src/studioState.ts` adds an optional `proposalTrack` (Zod `.optional()`); old rows without it are safe.

### The 9-part template (deterministic, derived in Go, not stored)

`ProposalTemplate()` returns the fixed 9 steps (`all-statuses.md §4 "golden standard"` + architecture-spec Pillar 1):

| # | key | Title | Template-question skeleton (fed to the AI to generate the guiding question) |
|---|---|---|---|
| 0 | `understanding` | 对题目的理解 (Understanding the prompt) | requirements / keywords + working definitions / assumptions / what's contested — **not a paraphrase**, must show the student's own view |
| 1 | `question-scope` | 研究问题与范围 (Question & scope) | the RQ, focus & scope, which subject/AOK/perspective; neither too broad nor too narrow |
| 2 | `thesis` | 暂定论点 (Working thesis) | conditional: if A then it holds, if B then constrained by C |
| 3 | `research-plan` | 研究计划 (Research plan) | first define 2–4 sub-questions; for each: what it resolves, how it relates to the central question, current view + supporting material, how it advances the next part (*see "Handling the research plan" below*) |
| 4 | `resources` | 资源 (Resources) | initial material already gathered: what it is, its source, which sub-question it supports, its limits |
| 5 | `challenges` | 可能的挑战 (Challenges) | strongest alternative explanation / rebuttal cases / other groups' perspectives / under what conditions the thesis changes |
| 6 | `method` | 研究方法 (Method) | which materials, how to analyze/compare, judgment criteria (varies by TOK/EE/AP) |
| 7 | `feasibility` | 可行性/限制/伦理 (Feasibility/limits/ethics) | materials reachable? time enough? scope right? privacy/copyright/bias? |
| 8 | `expected` | 预期结果 (Expected result) | — |

*Motivation is NOT a separate step* — it is carried by the framework's 缘由 (reason) dimension (architecture-spec decision).

### Handling the research plan (step 3) — an explicit simplification in this slice

`all-statuses.md §4` requires: the research plan first defines 2–4 sub-questions, **then one guide card per sub-question** (a dynamic expansion); and if the student "has no working thesis / no idea for sub-questions," it should suggest literature exploration first (a "begin literature exploration" button).

**This slice's (3a) handling:** `research-plan` stays a **single guide step** whose card explicitly asks the student to "first list 2–4 related sub-questions, then write the four things for each," and offers a **"go explore literature first" button** on the card (→ `open_reading`). The **per-sub-question dynamic card expansion, and a `stepIndex` that grows with the number of sub-questions, are deferred** (recorded under "Out of scope / follow-ups" below). This keeps `stepIndex` a static 0..8 nine-step track — a controllable, testable spine. This is a known deviation from §4, flagged in the "Doc consistency check" section.

---

## Pillar 2 · Guide-card content (AI-generated + cached)

Each step's **guide card** is: `{ prompt: guiding question (Chinese, applied to the current prompt), example: English example, refHint?: which part of the framework to reference }`.

- **Generation:** via `FastChatResolver` (fast model) — given `{title, the 4 framework dims, step key + template-question skeleton}`, it produces the guiding question (applied to the current prompt) + an **English example** (a demonstration for a *different* prompt, not the current answer — 铁律①).
- **Caching:** written into `WritingTrack.StepGuides[stepIndex]` (a JSON string) after generation; re-entering the step reads the cache with no re-spend. Switching back to revise does not regenerate.
- **Contract:** `packages/contracts/src/proposalGuide.ts` adds `GuideStep { key, title, prompt, example, refHint? }` + `ProposalGuideStep` (carrying `index/total/mode/started`).

### Endpoints

- `GET /projects/{id}/proposal-track` → the current track state (mode/stepIndex/started/total) + the current step's guide card (when guided AND started; otherwise empty). Read path; lazily generates + caches the current step's guide card (generation spends → records an `llm_call`, `purpose="proposal_guide"`).
- `POST /projects/{id}/proposal-track/mode` `{mode:"free"|"guided"}` → pick the mode. When guided, `started=false` (the outline intro comes first).
- `POST /projects/{id}/proposal-track/start` → guided: `started=true`, `stepIndex=0` (the student tapped "开始写作" after the outline intro).
- `POST /projects/{id}/proposal-track/advance` `{dir:"next"|"prev"}` → guided: move stepIndex (clamp 0..8). Past the last step it does NOT auto-finish — finishing goes through the existing 完成提案 (`finishWriting("proposal")`).

> Generation and advancing are **deterministic system steps**, not manipulation, and carry no confirmation gate ([[tielu-scope-writing-only-2026-08-09]]).

---

## Pillar 3 · Writing-page UI (`all-statuses.md §4 page` cleanup, proposal surface)

On `ProsePane` (the existing proposal surface):

1. **Remove the top icon bar**: the current toolbar's preview/word-count/save-status — per §4 "please delete the top icons, only keep word count and save status and put these two right lower corner" → **move word count + save status to a lower-right overlay**; keep the preview toggle but make it unobtrusive (lower-right or out of the way).
2. **Free/guided selection gate**: on first entry to the proposal surface with `mode==""`, the main area shows a **choice card**: "Do you want to write it yourself, or have 印记 walk you through it step by step?" with two buttons. §4: if the student has already written a proposal (buffer non-empty), default to suggesting free.
3. **Guided outline intro + 开始写作**: after choosing guided, the main area first shows 印记's intro to "a good proposal structure" (the 9-part overview) + a "开始写作" button (→ `start`).
4. **Guided guide-card scaffold**: once `started`, a **colored guide card** floats at the top of the main area (current step: title + AI guiding question + English example + refHint), with two buttons below:
   - **我依然有问题 (Still stuck)** → pushes this step's guiding question into the coach conversation (fast-model guidance).
   - **我写好了 (I'm done)** → triggers a reasoning review of **this part** (3a interim: `EvalResolver` returns `{ready,why,suggestions}`, surfaced as a 印记 bubble in the conversation, reusing slice 2's `ReviewVerdict` contract + rendering path), then `advance(next)` to the next step.
   - A light "prev / next" nav (the student may move back and forth freely — 铁律②).
5. **needs-resources box**: a small "need to look something up" box on the writing page (lower-right, or inside the guide card) where the student jots a point to explore, with a button that jumps to the reading room via `open_reading` (§4 line 101 / §5). This slice lands only this one box on the writing page; the reading-page box + search-guidance box are slice 5 (UI cleanup).

**§4 page items NOT done in this slice (deferred):** the left multi-tab reference panel (提案要点/阅读笔记/**AI批注**) — the AI批注 tab depends on 3b's annotation primitive; "send to AI" on selection already exists on the essay surface (`DraftPane`'s selPop), so proposal-surface select-to-send folds into 3b; the "let AI comment first" suggestion modal on finish depends on 3b's annotations, so 完成提案 keeps its existing modal for this slice.

---

## Mapping to existing code

**Reuse:**
- `StudioState` jsonb read/write (`GetStudioState`/`SetStudioState`, no migration).
- `ProsePane` (proposal surface) — layer the choice gate / outline intro / guide card / needs-resources box on top of it.
- The `ReviewVerdict` contract (slice 2) + the 印记-bubble rendering (`appendFrameworkVerdict`'s path) — reused for the "我写好了" interim review.
- `FastChatResolver` (guide-card generation + "我依然有问题" answers); `EvalResolver` ("我写好了" review).
- The `open_reading` tool (needs-resources box jump).
- The coach conversation store (`useStudioChat`) — inject guiding questions / review suggestions into the conversation.

**New:**
- `agent.WritingTrack` + `agent.ProposalTemplate()` + guide-card generator `agent.GenerateProposalGuideStep(...)` (fast model) + `agent.ReviewProposalPart(...)` (flagship, reusing `ReviewFramework`'s structure).
- REST: the four `proposal-track` endpoints (GET / mode / start / advance).
- Contracts: `proposalGuide.ts` (GuideStep / ProposalGuideStep); `studioState.ts` gains `proposalTrack?`.
- Frontend: a `useProposalTrack` hook + a `ProposalGuide` component (choice gate / outline intro / guide-card scaffold) + the needs-resources box; `ProsePane` toolbar cleanup (word-count/save to lower-right).

---

## Doc consistency check (against `all-statuses.md §4`)

| §4 point | This slice | Verdict |
|---|---|---|
| Two modes free / guided, chosen before start; default free if already written | Pillar 3.2 choice gate, suggest free when buffer non-empty | ✅ consistent |
| Guided: AI first intros a good proposal structure (generates outline) + "开始写作" | Pillar 3.3 outline intro + start | ✅ consistent |
| Guide card = colored, clear guiding question, English example, two buttons "我依然有问题/我写好了" | Pillar 2 + 3.4 | ✅ consistent |
| Guiding question AI-generated, applied to current prompt, example prepared (English) | Pillar 2 fast-model gen + cache | ✅ consistent |
| Nine parts (Introduction=3 + 6) | Pillar 1 template | ✅ consistent |
| Research plan: first 2–4 sub-questions, **one card per sub-question**; if no ideas, suggest exploring first | Pillar 1 "Handling the research plan" | ⚠️ **simplified**: 3a keeps a single step + in-card decomposition + explore button; **per-sub-question dynamic expansion deferred**. Flagged to user. |
| Writing page: remove top icons, word-count + save status lower-right | Pillar 3.1 | ✅ consistent |
| needs-resources box (writing page), jumps to exploration | Pillar 3.5 | ✅ writing-page box; reading-page box is slice 5 |
| Left multi-tab: 提案要点/阅读笔记/**AI批注** | deferred to 3b (AI批注 needs the annotation primitive) | ⏭️ explicitly handed to 3b |
| Select "send to AI"; "let AI comment first" suggestion modal on finish | deferred to 3b (needs annotations); finish keeps existing modal | ⏭️ explicitly handed to 3b |
| "我写好了" triggers AI comment/check (批注) | 3a interim = reasoning review giving suggestions in chat; 3b upgrades to colored anchored annotations | ⚠️ interim implementation, completed in 3b |

**The one substantive deviation** = deferring the research plan's "one card per sub-question" dynamic expansion. Everything else is a scope split **explicitly handed to 3b / slice 5**, not a behavioral conflict. If the user considers the research-plan dynamic cards a must for 3a, come back and adjust.

## Out of scope / follow-ups

- 3b: the layered, colored **批注** annotation primitive (structure/paragraph/sentence, green/blue/red + anchors) + AI批注 ref tab + the pre-finish "comment first" suggestion modal + proposal-surface select-to-send.
- Follow-up enhancement: the research-plan step's **per-sub-question dynamic card** expansion (a growing stepIndex).
- Slice 4: the essay three-stage track (research/statement/submission).
- Slice 5: the reading-page needs-resources box + search-guidance box + left multi-tab reference panel cleanup.
- No fancy layout editor, no submitting on the student's behalf, no addictive gamification (铁律 carried).

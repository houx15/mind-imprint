# Slice 3a — Proposal-writing sub-machine (guide-step track + dynamic 9-part scaffold) · Design

> **Position:** the first half (3a) of slice 3 in the [status-machine architecture spec](2026-08-09-status-machine-writing-and-cards-design.md). Behavioral source of truth = `docs/2026-08-09-all-statuses.md §4 (4.Writing proposal)`. This is the **spine** of the proposal-writing sub-machine: the guide-step track (`WritingTrack`) + the proposal 9-part scaffold **with a dynamic, per-sub-question research-plan** + free/guided modes + writing-page UI cleanup. **The 批注 annotation primitive (layered, colored teacher-style comments) is deferred to slice 3b** — here the "我写好了 (I'm done)" action uses the **existing reasoning reviewer** (suggestions surfaced in chat) as an interim; 3b upgrades it to anchored, colored annotations.

## One-sentence goal

Give the proposal document a **deterministic-but-content-derived guide-step track**: before writing, the student picks free / guided; in guided mode, 印记 hands out a **colored guide card** for each part in turn — and the research plan is not one card but a **carefully-guided sub-question decomposition followed by one card per sub-question** — with an AI-generated guiding question + an English example + two buttons ("我依然有问题 / 我写好了"). The student writes each part themselves (铁律①); advancing is the student's tap (铁律②).

## Why the research plan is the heart of this slice (design principle)

For a student, the research plan is the hardest and most valuable part of the proposal, and it is where the platform earns its keep. It cannot be a single line or a single card:

1. **Deciding the 2–4 sub-questions is itself a supported thinking task.** The support is: how do you break your key research question into a set of sub-questions that are (a) actually researchable, (b) correlated with each other, and (c) constitutive parts of the key question (a connected chain, not a list of topics)? If the student has no working thesis or no idea for sub-questions, we suggest literature exploration first rather than forcing an answer.
2. **Each sub-question then needs its own support, one at a time.** Per `all-statuses.md §4`, every sub-question card guides several hard things: what this sub-question resolves, how it relates to the central question, the student's current view + which materials/evidence would support it, and how it advances the next part. We do these **one sub-question per card, one at a time** — never bundled.

Therefore the track's step list is **dynamic**: its length grows with the number of sub-questions the student defines. `stepIndex` walks the derived list; it is never a fixed 0..8. (An earlier draft of this spec froze it at 0..8 for implementation convenience — that was wrong and is corrected here.)

## 铁律 (carried, hard constraints)

1. **AI never writes the body text** — a guide card gives only a guiding question + an example; the student writes. The example is an English demonstration for a **different** prompt, never the answer to the current one.
2. **No manipulation** — the student chooses free/guided and may switch mid-way (switching only changes `mode`, never discards written content); "我写好了 / finish" is never blocked by the review — insisting on advancing is recorded as process data.
3. **One question at a time** — a guide card shows only the current step (and, at a sub-question card, only that one sub-question).
4. **Process is data** — skipping the guide, writing free, ignoring suggestions all leave a trace.
5. **Never downgrade evaluation** — "我写好了" review runs on `EvalResolver` (flagship reasoning); coach guidance runs on `FastChatResolver` (fast model).

---

## Pillar 1 · The guide-step track (new data on `StudioState`)

`WritingTrack` is a **dimension** of each writable document (not a new status). This slice lands only the **proposal** track; the essay track is slice 4.

```go
// package agent
type WriteMode string
const ( ModeUnset WriteMode = ""; ModeFree WriteMode = "free"; ModeGuided WriteMode = "guided" )

type SubQuestion struct {
    ID   string `json:"id"`   // stable id minted server-side; keys the per-sub-question guide + snippet
    Text string `json:"text"` // the student's own wording
}

type WritingTrack struct {
    Mode         WriteMode         `json:"mode"`                   // ""=not chosen yet; free / guided
    Started      bool              `json:"started"`                // guided: student tapped "开始写作" (saw the outline intro)
    StepIndex    int               `json:"stepIndex"`              // index into the DERIVED step list (dynamic length); free stays 0
    SubQuestions []SubQuestion     `json:"subQuestions,omitempty"` // the 2–4 sub-questions; drives the per-sub-question expansion
    StepGuides   map[string]string `json:"stepGuides,omitempty"`   // step KEY → generated+cached guide-card JSON (avoids re-spend). Keyed by stable key, NOT int index (indices shift as sub-questions change).
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

### The derived step list (deterministic in Go, content-derived, not stored)

`DeriveProposalSteps(track WritingTrack) []Step` builds the guided step list from the fixed 9 golden-standard parts (`all-statuses.md §4`) with the **research-plan part expanded** into a define step + one step per sub-question:

```go
type StepKind string
const ( KindFixed StepKind = "fixed"; KindSubqDefine StepKind = "subq-define"; KindSubq StepKind = "subq" )

type Step struct {
    Key           string   // "understanding" | ... | "research-plan" | "subq:<id>" | ...
    Title         string
    Kind          StepKind
    SubQuestionID string   // set only for KindSubq
}
```

Order:

| position | key | Kind | Title | Template-question skeleton (fed to the AI) |
|---|---|---|---|---|
| Intro-1 | `understanding` | fixed | 对题目的理解 (Understanding the prompt) | requirements / keywords + working definitions / assumptions / what's contested — **not a paraphrase**, must show the student's own view |
| Intro-2 | `question-scope` | fixed | 研究问题与范围 (Key question & scope) | the key RQ, focus & scope, which subject/AOK/perspective; neither too broad nor too narrow |
| Intro-3 | `thesis` | fixed | 暂定论点 (Working thesis) | conditional: if A then it holds, if B then constrained by C |
| RP-0 | `research-plan` | subq-define | 研究计划 · 定子问题 (Define sub-questions) | **the heart**: guide the student to 2–4 sub-questions that are researchable, correlated, and constitutive of the key RQ (a connected chain, not a topic list). If no working thesis / no ideas → suggest literature exploration first (a "go explore literature first" button → `open_reading`). |
| RP-i | `subq:<id>` | subq | 子问题 i (Sub-question i) | for THIS sub-question: what it resolves, how it relates to the central RQ, the student's current view + which materials/evidence support it, how it advances the next part. **One sub-question per card.** |
| … | (repeat RP-i for each sub-question) | | | |
| 5 | `resources` | fixed | 资源 (Resources) | initial material gathered: what it is, its source, which sub-question it supports, its limits |
| 6 | `challenges` | fixed | 可能的挑战 (Challenges) | strongest alternative explanation / rebuttal cases / other groups' perspectives / when the thesis changes |
| 7 | `method` | fixed | 研究方法 (Method) | which materials, how to analyze/compare, judgment criteria (varies by TOK/EE/AP) |
| 8 | `feasibility` | fixed | 可行性/限制/伦理 (Feasibility/limits/ethics) | materials reachable? time enough? scope right? privacy/copyright/bias? |
| 9 | `expected` | fixed | 预期结果 (Expected result) | — |

- Derived list length = 8 fixed steps + 1 subq-define step + **N** sub-question steps (N = 2..4) = **9 + N**. `stepIndex` walks `0..(9+N-1)`.
- *Motivation is NOT a separate step* — carried by the framework's 缘由 (reason) dimension (architecture-spec decision).
- Before the student has defined sub-questions, `DeriveProposalSteps` yields the 3 intro steps + the `research-plan` define step + the 5 tail steps (no `subq:*` steps yet); once `SubQuestions` is set, the `subq:*` steps appear between the define step and `resources`.

---

## Pillar 2 · Guide-card content (AI-generated + cached)

Each step's **guide card** is: `{ prompt: guiding question (Chinese, applied to the current prompt/RQ/sub-question), example: English example, refHint?: which part of the framework/RQ to reference }`.

- **Generation:** via `FastChatResolver` (fast model). Context depends on the step kind:
  - fixed step → `{title, the 4 framework dims, step key + template skeleton}`.
  - `research-plan` (subq-define) → the key RQ + working thesis + framework dims; the prompt teaches **how to decompose into researchable, correlated, constitutive sub-questions**, with an English example decomposition (for a different prompt).
  - `subq:<id>` → the key RQ + THIS sub-question's text + the sibling sub-questions (so the card can speak to how it relates to the others and advances the chain).
- **Caching:** written into `WritingTrack.StepGuides[stepKey]` (a JSON string) after generation; re-entering reads the cache with no re-spend. Switching back to revise does not regenerate. Editing a sub-question's text invalidates that `subq:<id>` cache entry (regenerate next visit).
- **Contract:** `packages/contracts/src/proposalGuide.ts` adds `GuideCard { prompt, example, refHint? }` + `ProposalGuideStep { key, title, kind, index, total, mode, started, card, subQuestions }` (the current derived step + list length + the defined sub-questions, so the define step can render them).

### Endpoints

- `GET /projects/{id}/proposal-track` → the current track state (mode / stepIndex / started / total / subQuestions) + the current step's guide card (when guided AND started; else empty). Read path; lazily generates + caches the current step's guide card (generation spends → records an `llm_call`, `purpose="proposal_guide"`).
- `POST /projects/{id}/proposal-track/mode` `{mode:"free"|"guided"}` → pick the mode. When guided, `started=false` (the outline intro comes first).
- `POST /projects/{id}/proposal-track/start` → guided: `started=true`, `stepIndex=0`.
- `POST /projects/{id}/proposal-track/subquestions` `{subQuestions:[{id?,text}]}` → set/replace the 2–4 sub-questions (student-defined text; server mints ids for new ones, preserves ids for edited ones, drops removed ones — invalidating their guide/snippet). This **re-derives the step list**. Used at the `research-plan` define step. Deterministic (the student's own content) — no confirmation gate.
- `POST /projects/{id}/proposal-track/advance` `{dir:"next"|"prev"}` → guided: move stepIndex over the **derived** list (clamp `0..total-1`). Advancing off the `research-plan` define step requires 2–4 sub-questions defined **or** the student choosing to explore first (never hard-blocked — 铁律②: a student who insists is let through, recorded). Past the last step it does NOT auto-finish — finishing goes through the existing 完成提案 (`finishWriting("proposal")`).

Where does each part's text go? Each guided step writes into the same per-doc proposal buffer the free mode uses — the guided scaffold is a lens over the one proposal document, not a second store. (Sub-question snippets are sections of that document; the exact in-buffer representation is a plan-level detail — a labeled section per step key — kept consistent with how the free surface reads/writes the buffer.)

> Generation, re-derivation, and advancing are **deterministic system steps**, not manipulation, and carry no confirmation gate ([[tielu-scope-writing-only-2026-08-09]]).

---

## Pillar 3 · Writing-page UI (`all-statuses.md §4 page` cleanup, proposal surface)

On `ProsePane` (the existing proposal surface):

1. **Remove the top icon bar**: per §4 "please delete the top icons, only keep word count and save status and put these two right lower corner" → **move word count + save status to a lower-right overlay**; keep the preview toggle but make it unobtrusive.
2. **Free/guided selection gate**: on first entry with `mode==""`, the main area shows a **choice card**: "Do you want to write it yourself, or have 印记 walk you through it step by step?" — two buttons. Default-suggest *free* if a proposal is already written (buffer non-empty).
3. **Guided outline intro + 开始写作**: after choosing guided, the main area first shows 印记's intro to "a good proposal structure" (the 9-part overview, noting that the research plan will be broken into your own sub-questions) + a "开始写作" button (→ `start`).
4. **Guided guide-card scaffold**: once `started`, a **colored guide card** floats at the top of the main area for the current step (title + AI guiding question + English example + refHint), with:
   - **我依然有问题 (Still stuck)** → pushes this step's guiding question into the coach conversation (fast-model guidance).
   - **我写好了 (I'm done)** → triggers a reasoning review of **this part** (3a interim: `EvalResolver` returns `{ready,why,suggestions}`, surfaced as a 印记 bubble, reusing slice 2's `ReviewVerdict` contract + rendering path), then `advance(next)`.
   - A light "prev / next" nav (the student may move back and forth freely — 铁律②).
5. **The sub-question define step (`research-plan`) is a distinct surface**, not a plain textarea card: it shows the AI decomposition guidance + an **editable list of 2–4 sub-questions** (add/edit/remove rows, 2–4 enforced softly) that POSTs to `…/subquestions`, plus the **"go explore literature first" button** (→ `open_reading`) for a student with no ideas yet. Confirming the sub-questions expands the track and advances into the first sub-question card.
6. **needs-resources box**: a small "need to look something up" box on the writing page (lower-right or inside the guide card) where the student jots a point to explore, with a button that jumps to the reading room via `open_reading` (§4 line 101 / §5). This slice lands only this one box on the writing page; the reading-page box + search-guidance box are slice 5.

**§4 page items NOT done in this slice (deferred):** the left multi-tab reference panel (提案要点/阅读笔记/**AI批注**) — the AI批注 tab depends on 3b's annotation primitive; proposal-surface select-to-send folds into 3b; the "let AI comment first" suggestion modal on finish depends on 3b's annotations, so 完成提案 keeps its existing modal for this slice.

---

## Mapping to existing code

**Reuse:**
- `StudioState` jsonb read/write (`GetStudioState`/`SetStudioState`, no migration).
- `ProsePane` (proposal surface) — layer the choice gate / outline intro / guide card / sub-question editor / needs-resources box on top of it.
- The `ReviewVerdict` contract (slice 2) + the 印记-bubble rendering (`appendFrameworkVerdict`'s path) — reused for the "我写好了" interim review.
- `FastChatResolver` (guide-card generation + "我依然有问题" answers); `EvalResolver` ("我写好了" review).
- The `open_reading` tool (needs-resources / explore-first jumps).
- The coach conversation store (`useStudioChat`) — inject guiding questions / review suggestions into the conversation.

**New:**
- `agent.WritingTrack` + `agent.SubQuestion` + `agent.DeriveProposalSteps()` + guide-card generator `agent.GenerateProposalGuideStep(...)` (fast model, step-kind-aware) + `agent.ReviewProposalPart(...)` (flagship, reusing `ReviewFramework`'s structure).
- REST: the five `proposal-track` endpoints (GET / mode / start / subquestions / advance).
- Contracts: `proposalGuide.ts` (GuideCard / ProposalGuideStep / SubQuestion); `studioState.ts` gains `proposalTrack?`.
- Frontend: a `useProposalTrack` hook + a `ProposalGuide` component (choice gate / outline intro / guide-card scaffold / **sub-question editor**) + the needs-resources box; `ProsePane` toolbar cleanup.

---

## Doc consistency check (against `all-statuses.md §4`)

| §4 point | This slice | Verdict |
|---|---|---|
| Two modes free / guided, chosen before start; default free if already written | Pillar 3.2 choice gate, suggest free when buffer non-empty | ✅ |
| Guided: AI first intros a good proposal structure (generates outline) + "开始写作" | Pillar 3.3 outline intro + start | ✅ |
| Guide card = colored, clear guiding question, English example, two buttons "我依然有问题/我写好了" | Pillar 2 + 3.4 | ✅ |
| Guiding question AI-generated, applied to current prompt, example prepared (English) | Pillar 2 fast-model gen + cache | ✅ |
| Nine parts (Introduction=3 + 6) | Pillar 1 derived list | ✅ |
| Research plan: **first define 2–4 sub-questions (discussed carefully), then one card per sub-question**, each guiding several parts; if no ideas, suggest exploring first | Pillar 1 (`subq-define` + dynamic `subq:*` steps) + Pillar 2 (per-sub-question generation) + Pillar 3.5 (sub-question editor + explore-first button) | ✅ **now compliant** (the earlier single-card simplification is removed) |
| One sub-question at a time | Pillar 1 one `subq:*` step per sub-question; 铁律③ | ✅ |
| Writing page: remove top icons, word-count + save status lower-right | Pillar 3.1 | ✅ |
| needs-resources box (writing page), jumps to exploration | Pillar 3.6 | ✅ writing-page box; reading-page box is slice 5 |
| Left multi-tab: 提案要点/阅读笔记/**AI批注** | deferred to 3b (AI批注 needs the annotation primitive) | ⏭️ handed to 3b |
| Select "send to AI"; "let AI comment first" suggestion modal on finish | deferred to 3b (needs annotations); finish keeps existing modal | ⏭️ handed to 3b |
| "我写好了" triggers AI comment/check (批注) | 3a interim = reasoning review giving suggestions in chat; 3b upgrades to colored anchored annotations | ⚠️ interim, completed in 3b |

No behavioral deviation from §4 remains in this slice. The only items not built here (AI批注 tab, select-to-send, comment-first modal, and the colored anchored form of "我写好了" feedback) all depend on the 批注 primitive and are explicitly handed to **3b**.

## Out of scope / follow-ups

- 3b: the layered, colored **批注** annotation primitive (structure/paragraph/sentence, green/blue/red + anchors) + AI批注 ref tab + the pre-finish "comment first" suggestion modal + proposal-surface select-to-send.
- Slice 4: the essay three-stage track (research/statement/submission).
- Slice 5: the reading-page needs-resources box + search-guidance box + left multi-tab reference panel cleanup.
- No fancy layout editor, no submitting on the student's behalf, no addictive gamification (铁律 carried).

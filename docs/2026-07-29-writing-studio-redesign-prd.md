# Writing Studio Redesign · PRD

> Date: 2026-07-29 · Status: design locked (prototype UI-complete), pending real build
> Replaces the old writing studio (the six-station pipeline). This is the single hand-off document to carry into the real-build phase after context is cleared.

---

## 0. How to use this document

- **This is the spec for the "Writing Studio".** The old writing studio (the six-station pipeline in `apps/web/src/studio/views/*`) is **wholly replaced** by this design.
- **The clickable prototype is the source of truth for visuals and interaction**: `apps/web/src/proto/`, mounted via `?proto` (see `Root.tsx`). Pure front-end, mock data, no backend. Read this doc alongside it.
  - Example project: `http://localhost:5231/?proto`
  - Brand-new empty project: once inside, click the bottom-left "原型视角 → 全新项目" (Prototype view → New project) toggle.
- **Not in production**: free to change anything, **no backwards-compatibility to consider**. Old studio code may be deleted.
- **The existing "Reading Room" is kept and reused** (`apps/web/src/studio/reading/`, i.e. the already-shipped "印记陪读 / read-together" focused surface) — in the new design it is the deep-reading layer of the Read room. **Do not rewrite it.**

For the real build, each phase still runs spec → plan → build; the backend follows `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md` (smart gateway, envelope persistence, async evaluation all server-side).

---

## 1. Why redesign

The old writing studio was a **linear six-station pipeline**: 立题 (framing) → 视角与素材 (perspectives/sources) → 信源评估 (source eval) → 论证构建 (argument) → 写作 (writing) → 评估 (assessment). Each station was a gate with its own form / gate-check / readiness gauge.

Two root problems, from the student's point of view:

1. **It doesn't match how writing actually works.** Real academic writing is **recursive and thesis-driven**: start with a rough position → read a little → start writing to figure out what you think → read to test it → rewrite. Thesis and outline **co-evolve**; nobody "finishes all the reading, then writes." A pipeline that forces "finish A before B" fights the student.
2. **It feels like "homework about the homework."** Every station makes you fill panels before you get to do the real thing; friction is amplified into a burden — this is the source of "rigid / makes students feel bad."

Conclusion: the container was wrong. **Rooms (places) replace stations (steps).**

---

## 2. The core redesign: a four-room workspace

**A project = a workspace of four "rooms" the student moves between freely.**

| Room | Chinese label | What the student does here |
|---|---|---|
| **Project Management** | 项目管理 | Kick-off (clarify goal/reason/activities/resources) · Plan (kanban/gantt) · Activity log |
| **Read** | 阅读 | Manage sources (Zotero-style library) + enter the "Reading Room" for sentence-by-sentence co-reading (reused) |
| **Write** | 写作 | Outline (bullets/mind-map) + body (one panel: write or upload) |
| **Review** | 回顾 | Student writes their own reflection + 印记's "your mind-imprint" mirror |

Three mental models:

- **A room is a "place," not a "step."** A persistent left nav rail keeps every room one click away; you land in Project Management. The student moves back and forth at will: plan a bit, go read, start writing, read more, write more, finally review.
- **Doorway navigation**: click a plan card in Project Management (tagged read/write/review) to drop straight into the matching room.
- **The unifying spine — each room quietly produces the deliverable the school requires.** This is the core adoption reason over Google Docs + ChatGPT: *it grows the coursework you must submit out of the work you actually do.* Real deliverables are in `docs/reference/real student essay writing/` (Yutong Hu's actual EPQ artifacts):

| Real deliverable file | Room | Our output |
|---|---|---|
| `timescale.xlsx` | Project Management | Gantt / WBS timescale |
| `Yutong Hu record form.docx` | Project Management | Activity log (production log) |
| `资源评估表.xlsx` (resource eval table) | Read | Annotated Bibliography |
| `Yutong Hu P+A.docx` (Proposal part) | Project Management | Proposal (EPQ §1–§4) |
| `Yutong Hu P+A.docx` (Assignment part) | Write | The body (student-written) |
| `Yutong Hu PPT.pptx` | (Stage 2, out of scope) | Presentation / defense |

---

## 3. The shell

- Persistent **thin left nav rail**: back to "全部项目 (all projects)" → project title + qualification tag (e.g. "TOK · 拓展论文风格") → the four rooms (numbered 01–04, Chinese + English names). Active room highlighted (left accent bar + tint background).
- Land in **Project Management**. All rooms switch instantly (no router, view-swap, consistent with the existing directory→studio and studio→Reading-Room swaps).
- Both project states must be supported: **example (with progress)** and **brand-new empty project** (see each room's empty state).

Prototype: `apps/web/src/proto/StudioPrototype.tsx`, `Icon.tsx` (`BLOCK_META`).

---

## 4. Room specs (prototype is authoritative)

### 4.1 Project Management (项目管理)
Prototype: `apps/web/src/proto/blocks/PlanBlock.tsx`

**Two phases, sharing one home.**

**Phase A · Forming** — the blank-page antidote: a calm conversation that turns "talking" into a "kick-off (开题)".
- Center: dialogue (one question at a time). **印记 guides the student to think through the "four kick-off dimensions,"** filling the right-hand panel as they talk.
- **Bilingual guidance**: a 中 / EN toggle; 印记 can guide in Chinese or English.
- Right-hand "开题 (kick-off)" panel = the four proposal dimensions, **each takes shape as you talk, all editable, none forced**:
  1. **Objective (目标)** — what question to answer / what to learn to do
  2. **Reason (缘由)** — why do this (linked subjects, interest, future)
  3. **Activities & timescale (活动与时间)** — how you plan to do it (grows into the plan)
  4. **Resources (资源)** — which books / journals / data / tools you need
  - A soft "x/4 touched" coverage indicator at top (not a gate). **These four dimensions feed assessment whether or not a formal report is ever written.**
- Two actions:
  - **Generate project plan (生成项目计划)** (primary, terracotta) → flips to Phase B.
  - **Write proposal (写开题报告，可选)** (prominent button) → opens a **writing surface (student writes it themselves — NOT AI-generated)**, sectioned per EPQ §1–§4 (title/objective/responsibilities, reasons, activities & timescale, resources), exportable. Available once ≥2 dimensions are covered.
  - ⚠️ Decision: the proposal has exactly one home = Project Management (NOT a third tab inside Write).

**Phase B · Working** — after generating, **the kanban becomes the home you see each time.** A slim "goal strip" up top (qualification + title + expandable reason/objective). Three-view toggle + export + "聊聊计划 (talk about the plan)" (back to Phase A):

- **Kanban (看板)**: columns 待办/进行中/完成 (todo/doing/done). Cards tagged read/write/review + stage. **Cards drag between columns** (real drag-and-drop). The todo column has "+ 添加任务 (add task)". Cards click through into the matching room (doorway).
- **Gantt (甘特图)**: **grouped by WBS stage** (stage 1 / stage 2…), day-based horizontal axis. **Bars drag to reschedule; a right-edge handle resizes duration** (real pointer drag + resize). Color by type, opacity by status.
- **Activity Log (活动日志)**: a real EPQ deliverable. Dated "what happened" entries; **partly auto-seeded** (from the student's platform actions: opening a source, drafting a paragraph, moving a task) tagged "自动 (auto)", **partly student-written** tagged "我记的 (mine)". "记一笔 (add a note)" to append. **Exported here in Project Management** (not in Review).
- **Export**: the plan (with stage/status/timing) and the activity log are exportable (the real build aligns formats to `timescale.xlsx` / `record form.docx`).

**Task types (tags)**: currently read/write/review. The real WBS (see the user-provided timescale + `timescale.xlsx`) also includes "PPT / defense / resource form" etc. — **the real build may broaden the type set** (e.g. research·read / write / produce / evaluate·review / admin) or allow custom tags. The prototype uses three types + stage grouping for now.

### 4.2 Read (阅读)
Prototype: `apps/web/src/proto/blocks/ReadingBlock.tsx`; **reuse** `apps/web/src/studio/reading/ReadingRoom.tsx` (do not rewrite).

**Two layers:**

**Layer 1 · The Library (Zotero-style)** — four zones + a floating AI:
- **Left: collections + tags** (categorization). Collections (folders, **nestable** multi-level) are the primary categorization; tags cross-cut. **The collections rail folds/expands** (collapse the whole rail to a thin strip + parent collections have a caret to fold their sub-collections). **Drag a reference row onto a collection to file it** (real drag).
- **Center: the reference table** (scales to many). Columns: title / source·date / notes / credibility. Rows multi-select → batch **export annotated bibliography / add to collection**. Rows are draggable.
- **Right: thin preview** (selected item). **All metadata is edited right here, in place**: title / author / classification / date / url / **author credentials** / tags / **should I use it (该用·待定·不用 / use·maybe·drop)** / **credibility evaluation (selector + notes)**. Includes "进入阅读室 (enter Reading Room)".
- **Floating AI** (bottom-right "问印记·找资料 / ask 印记 to find sources"): coach-the-hunt — **only gives direction / keywords / reliability judgment; never fetches or retrieves sources for the student.** The deep co-reading AI stays in the Reading Room, so in the library the AI floats on demand rather than occupying a permanent column.
- **Add a source**: modal — paste link/DOI (印记 fetches metadata) · upload file · manual entry; choose which collection it lands in. New sources arrive with blank metadata to fill in place.
- **Empty state**: brand-new project → "你的文献库还是空的 (your library is empty)" + add first source + floating 印记.

**Annotated Bibliography export** = the library's core export, aligned to the columns of the school's `资源评估表.xlsx`: Resource / Classification / Authors / **Author Credentials** / journal·website / Relevance (cited parts = reading notes, quote→finding) / Reliability evaluation / **Should I use this**.

**Layer 2 · The Reading Room** — **already shipped, reused**: focused coach-left / article-right, hang-a-lens-on-a-sentence, you-find-the-evidence, program-owned three-check verdict. Enter it from the library's "进入阅读室". Real build: wire the library's "进入阅读室" to the existing Reading Room (the existing implementation already runs the real SSE gateway + `useReadingLoop`).

### 4.3 Write (写作)
Prototype: `apps/web/src/proto/blocks/WritingBlock.tsx`

- Top **goal strip**: a one-line, always-visible thesis/objective + "看开题 → (see kick-off)" link back to Project Management (Write does **not** make a third proposal tab).
- **Two tabs: Outline (提纲) / Write (写作).**
- **Outline**, two views over the same data, two-way synced, both editable:
  - **大纲 (bullets)**: nested bullet points, inline edit + hover controls (← promote / → indent / + add / × delete).
  - **思维导图 (mind map)**: the same outline laid out as a tree (the title as the central node, expanding rightward by level, curved connectors, colored by depth); nodes are inline-editable.
- **Write**: **one panel.** Mode toggle **写在这里 (write here, Markdown + word count)** ⇄ **我在别处写了 (I wrote it elsewhere — upload Word/PDF/MD)**.
- **Right AI rail (印记·陪你写 / writes with you)**: talk about the outline, poke logic, hit it with counter-examples. **It does not write the body for you** (design principle). **This is where thinking-cards will surface in the future** (see §6).

### 4.4 Review (回顾)
Prototype: `apps/web/src/proto/blocks/ReviewBlock.tsx`

**A mirror, not a report card. The student writes for themselves first; the AI mirror supports (user chose option A).**

- **Left (main): the student's reflection.** Dimensioned prompts following the real EPQ/TOK reflection arc (based on a real student reflection, see §Appendix B):
  1. **Goal & achievement (目标与达成)** — what was the goal, how much achieved. **This question anchors back to the objective the student wrote at kick-off** (closing the loop).
  2. **Methods & data (方法与数据)** — which sources/data/methods, how they supported the judgment.
  3. **Problems faced (遇到的问题)** — where it got stuck, how they pushed through, whether they nearly gave up.
  4. **Limitations (局限)** — what's still lacking, where a critic would strike first.
  5. **Gains & future (收获与未来)** — what was gained (knowledge/skills/attitude), how they'd continue/improve.
  - "完成回顾 (finish review)" → archives and **quietly generates the process assessment**, recorded into the growth report (visible to teacher/parent); **to the student, nothing is graded here.**
- **Right (support): 你的思维印记 (your mind-imprint / the mirror)** — 印记's narrative assembled from the whole process (how the thesis grew / how reading fed writing / where the student figured it out vs. leaned on 印记 / which thinking-cards were summoned) + **带走这两点 (carry-forwards, two takeaways)**. Labeled "供你参考——不是评分 (for reference, not a grade)".
- The rubric/process assessment runs underneath and feeds the existing teacher/parent reports (see the assessment-teacher-end-program memory).

---

## 5. Data model (sketch, from the prototype's `protoData.ts`)

The real build lands in Postgres + `packages/contracts` Zod contracts; below is the shape reference.

- `Proposal { objective, reason, activities, resources }` (four kick-off dimensions)
- `PlanItem { id, title, tag: read|write|review, column: todo|doing|done, stage, refId?, start(day), days }`
- `LogEntry { id, date, text, source: auto|me }` (activity log)
- `Reference { id, title, kind(classification), author, credentials, year, url, tags[], collectionId, credibility?: strong|mixed|weak, takeaway(evaluation), notes: {quote,finding}[], decision: use|maybe|drop|null, pending?, searchHints? }`
- `Collection { id, name, parentId? }` (nestable)
- `OutlineNode { id, text, depth }` (outline, flat-with-depth)
- Reflection: `{label, q, anchor?}[]` + answers; mirror `{title, body}[]` + carryForwards[]

---

## 6. Alignment with the four design principles (铁律)

1. **AI restraint**: 印记 never settles conclusions for the student, **never writes the body, never gives answers, never fetches sources**; writing/reading/kick-off are all "student does it, AI thinks alongside." Thinking-cards (SIFT/Toulmin/concession etc., the product moat) are **summoned into the room by the AI at the right moment** (the Reading Room already implements "hang a card on a sentence"; the Write room is the next landing spot); triggering is automatic, opening is confirmed by the student.
2. **No manipulation**: no streaks / leaderboards / push notifications.
3. **One question at a time**: every dialogue (kick-off / find-sources / write-along) is short and restrained.
4. **Process is data**: skips, letting the AI answer directly, where the student got stuck — all recorded → activity log (partly auto-seeded) + process assessment.

---

## 7. Reuse / replace / remove

- **Reuse**: the Reading Room `studio/reading/*` (印记陪读, incl. `useReadingLoop`, SSE gateway wiring). The library's "进入阅读室" wires to it.
- **Replace**: the entire `studio/views/*` (six stations) + the station/gate/readiness logic in `StudioContainer` + `StationRail`/`ViewFrame` and other station navigation. Replaced by the four-room shell + four blocks.
- **Remove**: the station pipeline, readiness gauges, per-station spot-check/attestation panels (their value is distributed into the new rooms: sourcing → Reading Room; argument → Write AI + thinking-cards; assessment → Review).
- Since this is not in production and has no compatibility burden, the studio layer can be rewritten directly, preserving the existing backend gateway / assessment / org capabilities.

---

## 8. Explicit non-goals

- ❌ **Fetching/retrieving sources for the student** (coach-the-hunt only). An integrated search is a different product bet, out of scope for now.
- ❌ A rich-text document editor / layout / submission — the body is only a Markdown box or an upload; the right-hand process record is read-only. We occupy the "thinking" layer; we do not own the student's document.
- ❌ Addictive gamification.
- ❌ The Stage-2 PPT/defense output (corresponding to `PPT.pptx`) is out of scope this round.

---

## 9. Hand-off to the real build (graduation)

- The prototype is **pure front-end mock**; the real build wires each block to real state + the backend gateway (system prompt / refeed / turn loop / summon_card all server-side), persisting the **standard envelope** → process tree + assessment.
- **The Reading Room is already real** (runs through the gateway); the main new work is the server wiring + persistence for the library layer + Project Management + Write + Review.
- Suggested phasing: ① shell + Project Management (kick-off + plan + log) ② Read library (reusing the Reading Room) ③ Write (outline + body, write AI / thinking-cards) ④ Review (reflection + mirror + assessment snapshot). Each phase its own spec→plan→build.
- Export formats align to the real deliverables: `timescale.xlsx` / `record form.docx` / `资源评估表.xlsx` / proposal (`P+A.docx` first half).

---

## 10. Open decisions / TBD

- A reference lives in **one collection + many tags** (chosen, Zotero-classic); allowing a reference in multiple collections is a later addition.
- The proposal writing surface currently **pre-fills** the short four-dimension phrases the student gave in the chat (their own words, not AI-generated, to expand from); starting fully blank is an easy change if preferred.
- Whether to broaden task types to 5 + custom tags (see §4.1).
- The exact surfacing interaction for thinking-cards in the Write room (aligning to the Reading Room's "hang a card" paradigm).

---

## Appendix A · Prototype file map

```
apps/web/src/proto/
  StudioPrototype.tsx   Shell (left rail + room routing + example/new toggle)
  Icon.tsx              line-icons + BLOCK_META (the four room names)
  protoData.ts          all mock data + types (Phoebe-anchored content)
  blocks/
    PlanBlock.tsx       Project Management (4 kick-off dims + kanban/gantt/log + proposal writer)
    ReadingBlock.tsx    Read library (collections/table/preview/floating AI/empty state/annotated-bib export)
    WritingBlock.tsx    Write (goal strip + outline[bullets⇄mind-map] + body + AI rail)
    ReviewBlock.tsx     Review (dimensioned reflection + mind-imprint mirror + carry-forwards)
Root.tsx                ?proto → StudioPrototype
```
Mount: `?proto`; empty project: once inside, bottom-left "原型视角 → 全新项目".

## Appendix B · The real student reflection (the basis for Review's voice)

Source: an EPQ student (China's digital economy). The arc = goal → whether achieved → what data was used → problems faced (once wanted to give up, curiosity kept them going) → limitations → future plans + gains (data generalization, charts, time management, decision-making skills; strengthened passion for math/economics). The five dimensioned prompts in the Review room are designed from this. The full text and real deliverables are in `docs/reference/real student essay writing/`.

## Appendix C · Demo-anchored content

Uses Phoebe / "中国的发展让地球更可持续了吗？ (Has China's development made the planet more sustainable?)" (TOK · extended-essay style). Real content throughout — no lorem ipsum.

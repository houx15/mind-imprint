# Lite Edition — Feedback Backlog (2026-08-28)

> **Status:** captured, not yet specced. This is the durable record of one round of
> product feedback given on 2026-08-28, after a full end-to-end walk of both lite
> loops on production. Nothing here is designed yet; each sub-project below gets
> its own `spec → plan → build` cycle.
>
> **All four sub-projects must eventually ship.** Order is negotiable; completeness
> is not. When a sub-project is specced, link its spec from its heading here and
> tick its items as they land.

## Why this round exists

A full walk of both loops on `mind-lite.uni-robot.cn` (reading: 带读 5/5 + a lens
card + 完成这篇; writing: setup → planning → 段落 → 成稿 → 请印记看看 → 完成这篇)
reached both terminal screens and confirmed:

- **Neither report exists.** Both terminal screens render a literal placeholder —
  `这次阅读的报告还在路上。` / `这篇写作的报告还在路上。` — and the 「看报告」
  button in both history drawers navigates to exactly that placeholder.
- The loop completes, but in the user's words: *"we have finished a full loop but
  it is not interesting and appealing yet."*

---

## A · Reading room — depth

The reading room's shell is right (*"I love the current box!"*). What is missing is
depth inside it.

Delivered by: spec `docs/superpowers/specs/2026-08-28-lite-reading-room-depth-design.md`,
plan `docs/superpowers/plans/2026-08-28-lite-reading-room-depth.md`.

**Shipped 2026-08-29.** The surprise: A1 turned out to be *wiring*, not invention — the
aimed-summon path with a verbatim-validated example already existed in Go
(`agent.RouteReading` + `ResolveExampleAnchor`) and was dead code from the UI, because
`ReadingCoachPanel` replaced the room's own chat branch. And the 示范 → 你来选句 mechanic
inside the lens card already did exactly what was asked, down to rejecting a click on the
AI's own example sentence.

- [x] **A1 · 带读 must be able to summon a support card onto a specific passage.**
      Today the lens cards are only reachable by the student opening 透镜库 herself.
      带读 should be able to point at *this sentence / this paragraph*, analyze that
      particular part as a demonstration, and then hand the same move back to the
      student to perform on a different passage. (*"guide students to a particular
      sentence/paragraph in the paper, to analyze that particular part, and let
      students do similar analysis now? I didn't see it."*)
      - Note: the 示范 → 你来选句 mechanic already exists inside the lens card. The
        gap is that 带读 cannot *trigger* it or aim it at a chosen block.

- [x] **A2 · The closing step becomes a task, not a text question.**
      Today step 5 is 回答几个问题 — typed answers. It should instead invite an
      action performed *in the article*: go find the sentence that does X, go find
      the keywords that signal Y. (*"not textual questions, but we can invite
      students do something like find some texts, or find some keywords etc."*)

- [x] **A3 · The route should bend toward the student's own experience.**
      The whole 带读 process should make reading more interesting and continuously
      invite the student to connect the text to her own experience or thinking.

- [x] **A4 · Reading should be able to hand off to writing.**
      After finishing a reading, sometimes invite the student to write something
      about that article or that topic. (Bridge into the writing loop.)

---

## B · Writing room — redesign

The weakest surface. The user's verdict on the current 段落 guidance: *"our snippets
is not real guidance, it is even not good as pro version."*

Delivered by: spec `docs/superpowers/specs/2026-08-28-lite-writing-room-deepening-design.md`,
plan `docs/superpowers/plans/2026-08-28-lite-writing-room-deepening.md`.

- [x] **B0 · The plan must produce a whole essay skeleton, not just claims.**
      Today the map produces 中心论点 → 分论点 → 论据 and the 段落 stage renders one
      textarea per node — so there is nowhere for an **introduction, a hook, or a
      conclusion** to live, yet those are exactly what turns key points into a full
      paper. (*"we get the sub fields and key points and make those the whole thing
      of snippets. what about introduction? hooks? conclusions? etc."*)
      - Code note: `apps/api/internal/api/writing_plan.go:120` makes `role` a
        free-form model-invented string, and `writingPlanMaxDepth` (`:65`) caps the
        map at 中心论点 → 分论点 → 论据.

- [x] **B1 · Real guidance questions, in a readable layout.**
      The guidance must actually guide, and must be *legible* — *"a large paragraph
      of small texts is not easy to read."* Beautiful, clear layout for the
      questions.
      - Code note: `GuideBox` (`apps/lite-web/src/writings/GuideBox.tsx`) already
        exists and renders numbered questions, but only appears on-demand behind the
        `卡住了？` button, which is why it reads as absent.

- [x] **B2 · English writing needs expression support.**
      For EN writing, offer sentence structures / ways of expressing the thing the
      student is trying to say.

- [x] **B3 · 「深入一层」 opens a side AI chat.**
      Guidance questions will sometimes not be enough. The student can click to dive
      deeper, opening a side chat that pushes further on examples, expression,
      logic, etc. (*Open question in the feedback: should this be a sub-agent?*)

- [x] **B4 · Per-snippet AI comments, concrete and traceable.**
      Like the pro version: one general sentence plus specific key points, each able
      to **trace back to the original writing** it refers to.

- [x] **B5 · 成稿 becomes a real writing surface.**
      Not a box. The full left side is the writing space, with generous padding and
      good typography; the snippets sit alongside it by default.

- [x] **B6 · The title is editable at any time.**

- [x] **B7 · The full-draft AI comment renders in the sidebar** — clear,
      good-looking, and traceable back to the passage it is about.
      - Note: the review call itself is already strong. On the walk it produced a
        four-section critique (结构 / 论证 / 证据 / 语言) that caught a real defect in
        the draft. It is the *presentation and persistence* that is missing — it
        appears once on 成稿 and does not survive to the finished screen.

---

## C+D · The two reports

**One sub-project, two outputs.** They share the stats pipeline, the
"shining moments" extraction pass, the poster renderer, the image export, and the
public share route. Splitting them would build all of that twice.

Both reports must be **colorful, clear, not verbose, but interesting and
appealing** — the user's stated goal is 分享欲: the student should *want* to show it.
Both should surface the student's shining moments and their effort.

Delivered by: spec `docs/superpowers/specs/2026-08-29-lite-reports-design.md`,
plan `docs/superpowers/plans/2026-08-29-lite-reports.md`.

**Shipped 2026-08-29.** Two discoveries shaped it. (1) **Nothing in lite recorded
duration** — no heartbeat, no session table, only point-in-time stamps — so the
minutes had to be built: a visibility-gated, server-clamped heartbeat, plus a
capped-gap estimate so sessions finished before this shipped don't read 0 分钟.
(2) **Half of what looks like her writing is not hers**: the 发现 at finalize is AI
prose, highlight quotes are the article's, and since sub-project A a student
message carries sentences she *pointed at* as leading `>` lines. Stripping those is
where R4 is enforced — and a whole-branch review found a live leak in it (a
hard-wrapped paragraph makes a multi-line quote whose later lines were never
prefixed), now closed on both the client and the server.

### C · Reading report

- [x] **C1 · Stats:** reading time (min), AI chat turns, reading notes count.
- [x] **C2 · One thing to take away.** Open in the feedback — could be one important
      sentence, or the student's own most striking thought, or several columns that
      get filled in only if the student has something for them. *Design question,
      not yet decided.*
- [x] **C3 · My reading notes.**
- [x] **C4 · Export as an image** — good-looking, not verbose.
- [x] **C5 · Shining moments, Lark-meeting-notes style.** Lark surfaces a speaker's
      金句; do the same for the student's own best lines. Export with the student's
      name and their effort noted. (*"it would have fantastic effect!"*)
- [x] **C6 · Opt-in public share.** If the student agrees, a QR code others can scan
      to view her reading notes publicly.

### D · Writing report

- [x] **D1 · Stats:** words written, time spent, AI chat turns.
- [x] **D2 · Their most shining points.**
- [x] **D3 · Their gains.**
- [x] **D4 · Appealing enough to create 分享欲**, showing shining moments and effort.
- [x] **D5 · Opt-in public share.** If the student agrees, a QR code to view her
      writing publicly.

---

## 🚨 Cross-cutting rule — 印记 must talk like a teacher, not like an AI

**Ruling (user, firm, 2026-08-28):** *"generally your guidance towards students are
all this AI-feeling and not a human-feeling language."*

The example that triggered it. Rejected:

> 有人会从一个具体场景切进去，有人直接抛个问题。你这篇你想怎么进？

What a real teacher says:

> 对于一篇文章来说，有意思的开头非常重要。留悬念、设问、开篇直接叙述等，都是常见的
> 方式。你想尝试哪一种？或者需要我给几个具体的案例我们一起来学习一下这几种方法吗？

**The four things the teacher's version does, and the AI's doesn't:**

1. **States why it matters** — one line of stakes, so the question isn't arbitrary.
2. **Names the real methods** — 留悬念 / 设问 / 开篇直接叙述. The vocabulary is what
   she is here to learn; withholding it is not humility, it is teaching nothing.
3. **Offers a genuine choice**, with room to decline.
4. **Offers to teach** — 「要不要我举几个例子，咱们一起看看」. The door to instruction
   stays open instead of leaving her alone with a question.

**Mechanical cause, and the fix.** `writing_plan.go`'s output contract caps
`reply` at 不超过 120 字 and says 不要一次问好几个问题. Under those two rules the
model *cannot* state stakes, name methods, and offer to teach — so it emits the
clipped shrug above. The prompt manufactures the AI voice. Any spec in this
backlog that touches a student-facing prompt must revisit that cap.

**This applies everywhere**, not just B0: B1's guidance questions, B3's side chat,
B4/B7's comments, A's reading coach, and the prose in both reports.

**Interaction with 铁律③ (一次只问一个).** Still holds — one *question* per turn.
It never meant "say as little as possible". Stakes + method names + an offer to
show examples is one question with the teaching around it.

### This revises the 2026-08-27 ruling on naming patterns

That ruling said: *name the pattern only AFTER she produces it — handing her those
words as options is what made the old screen a form.* What was wrong there was the
**vessel, not the vocabulary**. A screen of labeled tiles is a form; a teacher
naming the common ways and offering to show examples is teaching. Same words,
opposite thing.

- 印记 **may** name 留悬念 / 设问 / 起承转合 / PEE / 让步 etc. in conversation.
- 印记 **may not** render them as a picker, menu, or tile grid. That ruling stands.

### Falls out of it: examples must be borrowed material

When 印记 offers 具体的案例 to teach a technique, those examples must be drawn from
something **other than the student's own piece**. An "example of a 留悬念 opening"
written about her topic *is* her opening, authored by AI — 铁律① violated through
the back door. Examples teach the technique on borrowed material; she writes hers.
This is checkable and belongs in every spec that lets 印记 demonstrate.

---

## ⏸ Model routing — a deliberate later exercise (2026-08-28)

**Direction (user):** *"for proper places we can use no reasoning deepseek. and later
we will schedule a time to test different models to reach a balance between speed,
performance, and cost. but this is not current focus."*

So: **do not re-route models ad hoc.** It gets its own session, with a benchmark
across speed / quality / cost, and the decisions are made against measurements
rather than intuition. This section exists so that session starts from a map.

### Where lite stands today

**Every lite room call resolves `resolveEval` — the flagship, never-downgrade
tier.** Lite uses `resolveFast` nowhere, although it exists (`proposal_track.go:103`)
and pro's lighter endpoints already use it (`essay_statement.go:94`,
`placement.go:68`, `question_card.go:78`, `search_guidance.go:61`).

| Call | Site | First read on whether it needs reasoning |
|---|---|---|
| 规划 turn | `writing_plan.go:441` | Asks one question, compresses HER words into nodes. Structured JSON. Plausibly the strongest candidate for a cheaper tier. |
| 引导框 (batch + single) | `writing_guide.go:745`, `:597` | Deliberately flagship — its comment argues asking a good question about a half-formed argument is the hardest reasoning in the room. **Also the one that times out.** Needs measurement, not a guess. |
| 段落/成稿 评语 | `writing_comment.go:253`, `writing_compose.go:231` | Genuinely hard: on the production walk it caught a real defect (an example that attacked over-dense planting rather than large-scale planting). Keep flagship until proven otherwise. |
| 深入一层 | `writing_deepen.go:303` | Socratic follow-up on one block, narrow context. Mid candidate. |
| 带读 coach / plan | `reading_coach.go:342`, `reading_plan.go:242` | Untouched by sub-project B; measure alongside A. |

### The concrete problem this would also solve

`/guide` can exceed **the handler's own 150s deadline** (observed: 1m52s fine,
>2m30s ⇒ 502). The lite e2e walk flakes on it, and from the test side it looks
identical to a client bug — so it costs debugging time as well as wall-clock. A
faster tier on that call, a longer deadline, or both — but decide it with numbers.

### Worth knowing before that session

Every real LLM call already writes a `llm_call` row with tier, tokens and cost, and
`llm_usage` unions it for org-level totals. So the benchmark does not need new
instrumentation — the cost side can be measured from data the platform already
keeps.

---

## ✅ Cross-cutting rulings — DECIDED (user, 2026-08-28)

These cut across A and C+D. All four were put to the user and answered; they are
binding on every spec below.

### R1 · Public sharing of a minor's work

**Anyone with the link, revocable, no expiry.** An unguessable random URL, no login
required. The student can turn sharing off at any time and the link dies
immediately. Her name may appear (she asked for the exported picture to carry it).

Consequences for the spec: the share token must be unguessable (not the atom id),
revocation must be a real state that the public route re-checks on every request
(no caching a revoked page), and the public route must expose **only** what the
report shows — never the transcript, never her account, never anything else she
owns.

### R2 · 铁律② and "shining moments"

**AI selects and displays them directly.** The user overruled the softer
"AI proposes, student confirms" option deliberately.

The line that keeps this inside 铁律②: what is displayed is **her own words**,
selected — never a score, a rank, a streak, a badge, or a comparison to anyone
else. Selection is editorial, not evaluative. And the thing that actually leaves
the platform still requires her explicit opt-in (R1), so nothing is published
about her without her acting.

### R3 · Reading → writing is a suggestion, and the suggestion is the product

**Not** a pre-seeded writing project, and **not** a third writing surface inside the
reading room. It is a suggestion — but a substantial one. In the user's words:

> *"just a suggestion, but suggestion is very important, some interesting questions
> would grow from this reading — why xxxx, what is xxx, how people view xxx, etc."*

So the deliverable is **questions grown from this particular reading** — real,
specific, article-derived questions worth writing about (为什么…… / 什么是…… /
别人怎么看……), not a generic 「去写点什么吧」 link. Generic questions are the failure
mode here, and the spec must make them structurally hard to emit.

### R4 · The reading report's 金句 are her own words only

Only sentences the student typed herself (her notes, her replies to 带读) may appear
as 金句 on the report or the exported picture. A striking sentence from the article
may not — an exported, shareable picture must never put the author's words under the
student's name. If she wrote little, the picture shows little; that is the honest
outcome and the design must look right when it happens.

---

## Also found on the walk (small, unassigned)

- [x] **「看报告」 is a dead label** — fixed with C+D: both placeholder lines are
      deleted and the label now leads to a real report.
- [x] **The reading's own output is no longer stranded.** The report now carries
      「我用透镜查到的」: for each submitted lens, the sentence **she picked out of the
      article** and the 发现 the room drew from it, in a section visually distinct
      from 金句 so neither is mistaken for the other.
      🚨 **Ruling that unblocked this (product owner, 2026-08-29):** I had kept these
      off the report by over-applying 铁律① — "AI never writes her prose" governs
      **her essay**, not what a report may show her about her reading. *"this
      principle is only related with her written essay."* **A sentence she selected
      is her work; the selecting is the thinking.** Degraded/canned findings are
      excluded — `framework_fill.degraded` exists precisely so a report can drop the
      ones no model ever read.

- [x] **可信度 shows 尚未评估 in lite** — fixed with sub-project A (Task 11): gated off by a
      new `credibility` capability, so pro — which does produce a verdict — keeps the field.
- [x] **The 我的写作 drawer got the shelf redesign** — filter chips, the unfinished
      dot, recency sort and a `?limit=` cap, mirroring 我的阅读. Required surfacing
      `atom.last_activity_at` on `writingDTO` first: it was always maintained for
      writing atoms but never exposed, so the drawer had nothing honest to sort by
      (and the landing's 「继续写」 shortcut had been picking by `updated_at`, which
      does not move while she is actually drafting — so it often opened the wrong
      piece).

---

## Found while closing the backlog — PRO side, out of scope here

- [ ] **`readingOutcomesFromCards` (pro) surfaces degraded findings.**
      `apps/api/internal/api/reading_takeaway.go:70-73` unmarshals `framework_fill`
      into `selectionEvalDTO` — which carries `Degraded` — but appends `ev.Finding`
      unconditionally. So when a lens evaluation degrades, `agent.fallbackEval`'s
      canned 「你选了这句作为证据。」 can surface in pro's reading takeaway as if it
      were real analysis. Same class as the bug fixed on the lite report
      (`0fa6229e`); `reading_lens.go`'s own comment calls the degraded path
      "a COMMON path, not a rare one". Confirmed, deliberately left untouched —
      it is pro code and this backlog is the lite edition's.

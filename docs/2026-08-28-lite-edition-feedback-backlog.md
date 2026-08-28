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

- [ ] **A1 · 带读 must be able to summon a support card onto a specific passage.**
      Today the lens cards are only reachable by the student opening 透镜库 herself.
      带读 should be able to point at *this sentence / this paragraph*, analyze that
      particular part as a demonstration, and then hand the same move back to the
      student to perform on a different passage. (*"guide students to a particular
      sentence/paragraph in the paper, to analyze that particular part, and let
      students do similar analysis now? I didn't see it."*)
      - Note: the 示范 → 你来选句 mechanic already exists inside the lens card. The
        gap is that 带读 cannot *trigger* it or aim it at a chosen block.

- [ ] **A2 · The closing step becomes a task, not a text question.**
      Today step 5 is 回答几个问题 — typed answers. It should instead invite an
      action performed *in the article*: go find the sentence that does X, go find
      the keywords that signal Y. (*"not textual questions, but we can invite
      students do something like find some texts, or find some keywords etc."*)

- [ ] **A3 · The route should bend toward the student's own experience.**
      The whole 带读 process should make reading more interesting and continuously
      invite the student to connect the text to her own experience or thinking.

- [ ] **A4 · Reading should be able to hand off to writing.**
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

### C · Reading report

- [ ] **C1 · Stats:** reading time (min), AI chat turns, reading notes count.
- [ ] **C2 · One thing to take away.** Open in the feedback — could be one important
      sentence, or the student's own most striking thought, or several columns that
      get filled in only if the student has something for them. *Design question,
      not yet decided.*
- [ ] **C3 · My reading notes.**
- [ ] **C4 · Export as an image** — good-looking, not verbose.
- [ ] **C5 · Shining moments, Lark-meeting-notes style.** Lark surfaces a speaker's
      金句; do the same for the student's own best lines. Export with the student's
      name and their effort noted. (*"it would have fantastic effect!"*)
- [ ] **C6 · Opt-in public share.** If the student agrees, a QR code others can scan
      to view her reading notes publicly.

### D · Writing report

- [ ] **D1 · Stats:** words written, time spent, AI chat turns.
- [ ] **D2 · Their most shining points.**
- [ ] **D3 · Their gains.**
- [ ] **D4 · Appealing enough to create 分享欲**, showing shining moments and effort.
- [ ] **D5 · Opt-in public share.** If the student agrees, a QR code to view her
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

## Cross-cutting rulings still needed

These cut across C+D and must be decided before either report is specced.

1. **Public sharing of minors' work.** Who can open a shared QR link, for how long,
   whether it can be revoked, whether a teacher or school gates it, and whether the
   student's real name appears. This is minors' names and schoolwork leaving the
   platform, so it needs a deliberate answer rather than a default.

2. **铁律② and "shining moments."** An exportable artifact of the student's *own*
   work is not a slot-machine mechanic, and 铁律② should not be over-applied (see
   the standing ruling that 铁律 scope is student body text, not orchestration).
   But *AI selecting what is impressive about you* can drift into flattery, and a
   report engineered for 分享欲 sits adjacent to the retention mechanics 铁律② rules
   out (连胜 / 排行榜 / 徽章 / 推送). Decide the line once, explicitly.

---

## Also found on the walk (small, unassigned)

- [ ] **「看报告」 is a dead label** in both history drawers — it navigates to the
      "报告还在路上" placeholder. Until the reports exist it should not promise one.
- [ ] **The reading's own output is stranded at the finish line.** The finalize modal
      shows 发现 and 关键引句 from the student's lens cards; the finished screen shows
      only 我的收获. The lens findings, key quotes, and the whole 带读 transcript are
      stored but unreachable. (C will likely absorb this.)
- [ ] **可信度 shows 尚未评估 in lite** — the field is rendered in the finalize modal
      but lite has no producer for it.
- [ ] **The 我的写作 drawer never got the shelf redesign.** It still uses the old
      two-section layout with the 11px 「还没写完 · N」 label — the exact thing that
      was called out and fixed for 我的阅读 on 2026-08-28 (`57524de5`). Missing there:
      filter chips, the unread dot, time sorting, and the `?limit=` cap.

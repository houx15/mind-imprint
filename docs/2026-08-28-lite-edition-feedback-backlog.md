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

- [ ] **B0 · The plan must produce a whole essay skeleton, not just claims.**
      Today the map produces 中心论点 → 分论点 → 论据 and the 段落 stage renders one
      textarea per node — so there is nowhere for an **introduction, a hook, or a
      conclusion** to live, yet those are exactly what turns key points into a full
      paper. (*"we get the sub fields and key points and make those the whole thing
      of snippets. what about introduction? hooks? conclusions? etc."*)
      - Code note: `apps/api/internal/api/writing_plan.go:120` makes `role` a
        free-form model-invented string, and `writingPlanMaxDepth` (`:65`) caps the
        map at 中心论点 → 分论点 → 论据.

- [ ] **B1 · Real guidance questions, in a readable layout.**
      The guidance must actually guide, and must be *legible* — *"a large paragraph
      of small texts is not easy to read."* Beautiful, clear layout for the
      questions.
      - Code note: `GuideBox` (`apps/lite-web/src/writings/GuideBox.tsx`) already
        exists and renders numbered questions, but only appears on-demand behind the
        `卡住了？` button, which is why it reads as absent.

- [ ] **B2 · English writing needs expression support.**
      For EN writing, offer sentence structures / ways of expressing the thing the
      student is trying to say.

- [ ] **B3 · 「深入一层」 opens a side AI chat.**
      Guidance questions will sometimes not be enough. The student can click to dive
      deeper, opening a side chat that pushes further on examples, expression,
      logic, etc. (*Open question in the feedback: should this be a sub-agent?*)

- [ ] **B4 · Per-snippet AI comments, concrete and traceable.**
      Like the pro version: one general sentence plus specific key points, each able
      to **trace back to the original writing** it refers to.

- [ ] **B5 · 成稿 becomes a real writing surface.**
      Not a box. The full left side is the writing space, with generous padding and
      good typography; the snippets sit alongside it by default.

- [ ] **B6 · The title is editable at any time.**

- [ ] **B7 · The full-draft AI comment renders in the sidebar** — clear,
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

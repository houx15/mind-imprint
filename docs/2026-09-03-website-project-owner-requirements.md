# The personal website project — owner requirements, 2026-09-03

Everything the product owner said in the review session of 2026-09-03, written
down before any design or code. This document is the thing the design spec has
to satisfy. Where a later design contradicts it, this document wins.

Context: slice S5 (`SiteStudio`) shipped on 2026-09-03 and was reviewed the same
day. The verdict was that it fails on interaction and, more seriously, on
structure. Both sets of points are below.

---

## 1 · Interaction requirements

Stated as a list, in her order:

1. **Do not let students enter forms.**
2. **Interactable.**
3. **Gamification.**
4. **Clear.**
5. **Students have the chance to think deeply.**
6. **Collect data about the websites they love.**
7. **Let them search, and give AI the URLs.**
8. **Tell their own stories** — what story they want to tell.
9. **A hero image** — what would that be? Let AI generate it for him.
10. **A personal introduction** — what style?
11. **Final adjustments** if they want.
12. **Issue — ship.**
13. **Modification — any time.**

On the 闯关 (level/quest) metaphor:

> "I think 闯关 is great. But it should not be feeling like entering several
> forms. But I think you are going to make that. It is AI guided journey, but it
> can have a 闯关 metaphor."

So: **the engine is an AI-guided journey. 闯关 is the metaphor it wears.** The
failure mode she named in advance is five 关 that are each a form with a
progress bar on top.

## 2 · The journey

Her five stages, as she described them. This replaces the three-step
样子 / 内容 / 发布 flow that shipped.

### Stage 1 — Think about my story from the audience's view

Compose an audience. Who is he or she? Why do they know you? What are they
willing to see? What do you want to show them? What feeling do you want to
bring them?

- Draw a **persona board** with **AI-generated photos** and **tagged keywords**.
- Then draw a **story board** based on that.
- Output of this stage: **several keywords** that define the audience and the
  website's feeling.

### Stage 2 — Learn what a personal website looks like

Through **example learning** and **imagination**.

- Her favourite website, and how it works.
- Different style examples.
- What did the student get out of them? What is **common** across them, and
  what can also be **unique**?
- Then **compose the structure in a mindmap** — what her website will be like.
- Then **verify it against the keywords** from stage 1.

### Stage 3 — AI needs your materials

- Scan her history: "we have got these — do you have more to add? You can just
  tell me the story."
- Besides that: what is a **colour palette**, and what is her choice?
- **Style?**
- **Hero image** — does she need one? It can be generated here. "Your website
  is becoming real."

### Stage 4 — AI's turn: generating

### Stage 5 — Her turn: modify every detail, with the review tool

## 3 · The structural failure

Her words:

> "These tools, many have been built in pbl room. Including the AI's guided
> journey, generated task list. But seems you didn't use any. So I didn't see
> the things I required, collaboration card, review card, etc. Personal website
> is a first project, so it is a question-defined project, but you make it an
> independent thing which is not related with pbl."

Three separate charges, all of them true:

1. **The room's machinery was ignored.** The PBL project room already has an
   AI-guided journey, a generated task list (the dynamic plan), and ten tools
   with real surfaces: `observe` 观察日记, `board` 头脑风暴, `reframe` 问题识别,
   `ideas` 解决方案, `review` 审核助手, `decide` 理性决策, `structure` 结构审查,
   `split` 分工建议, `lookback` 项目复盘, `keep` 长期迭代.
2. **So the cards she had asked for never appear** — the collaboration card
   (`split` 分工建议) and the review card (`review` 审核助手) among them.
3. **The website is the first project, so it is a question-defined project.**
   It was built as an independent thing unrelated to PBL.

The proof of the charge is one line: `projects/ProjectSurface.tsx:42` returned
`<SiteStudio>` whenever `kind === "website"`, so the project room never
rendered. The website project created a `pbl_project` row and then bypassed
everything attached to it — no coach, no plan, no tools, no board, no review, no
分工, no 复盘. The publish gate made that screen mandatory, which made it worse:
the first thing every student met was the one screen with none of the product in
it.

## 4 · The directive

> "Write down all my points. Then start fixing the website to a real pbl project
> with defined topic and suggested (special ai prompt) routine. Solve all the
> problems I identified. You don't need the old personal website pages — you can
> retire them."

Three instructions:

- The website becomes **a real PBL project** with a **defined topic** and a
  **suggested routine**, the routine carried by a **special AI prompt**.
- Solve every problem in §1–§3.
- The old personal website pages are **retired**, not preserved. Nothing in the
  shipped `SiteStudio` flow is a constraint on the new design.

## 5 · Two rulings this session settles

**The three fixed layouts stop being the endpoint.** Spec
`2026-09-01-pbl-project-room-design.md` §15 required three layouts —
`Essay` / `Ledger` / `Magazine` — with her justified choice visible. Stage 2 now
ends with her composing her own structure in a mindmap, and stage 4 generates
from that. Both cannot be the spine. The three layouts become **style examples
in stage 2** (studied next to the real sites she pasted) and typographic bases
for generation. She no longer picks from a menu of three; she builds one. §15 is
amended, not quietly worked around.

**Prose style, when writing to her:** plain English. No mannered prose. Product
copy stays Chinese and follows `AGENTS.md § 界面文案怎么写`; that is separate.

---

## Appendix · The anti-form rule

Derived in this session, from her "it should not be feeling like entering several
forms", and kept as the review checklist for every stage of the journey.

Each stage must have all three:

1. **AI moves first.** When a stage opens, the first thing on screen is
   something 印记 brought — analysis of pages she pasted, three drafts, an
   image, her own past work. Never an empty input box.
2. **Her action is a judgment, not a fill-in.** Pick, reject, drag, cross out,
   reorder. Typing only where she genuinely has something of her own to say.
3. **The page visibly grows right after her action**, and 印记 replies with
   something that proves it read her.

Hard limits, checked at review time:

- Max **one** input on screen at a time. Two labelled boxes side by side means
  that stage failed; redo it.
- **No noun-slot labels.** 「你是谁」「标签」「开场一段」are deleted. If we want
  her words, 印记 asks a specific question instead.
- **Across the whole journey she types in at most 3 places.** Everything else is
  picking, dragging, rejecting. This is a number, not a feeling.
- **Unlock conditions are real** (three collected sites before stage 2 opens),
  never greyed-out decoration. Consistent with
  `2026-09-01-pbl-project-room-design.md` §12「置灰的按钮是装饰」.

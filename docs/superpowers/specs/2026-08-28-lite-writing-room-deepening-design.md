# Lite Writing Room — Deepening (Design Spec)

**Date:** 2026-08-28
**Status:** approved in conversation, ready for an implementation plan
**Backlog:** `docs/2026-08-28-lite-edition-feedback-backlog.md` (sub-project **B**)
**Supersedes nothing; revises one ruling** — see *Standing rulings*, below.

---

## 1. Why this exists

A full end-to-end walk of the lite writing loop on production
(`mind-lite.uni-robot.cn`, 2026-08-28) completed without error and produced a
finished piece. The verdict on it was still:

> *"our snippets is not real guidance, it is even not good as pro version."*
> *"the full paper writing is also not good."*
> *"we have finished a full loop but it is not interesting and appealing yet."*

Nothing in this room is broken. It is under-taught and under-designed. This spec
addresses the room only; the reading room (**A**) and the two reports (**C+D**)
are separate sub-projects with their own specs.

## 2. Scope

In scope — the eight items filed as **B0–B7** in the backlog:

| # | One line |
|---|---|
| B0 | The plan must be able to produce a whole piece, not only claims |
| B1 | Guidance is present on arrival, and it teaches |
| B2 | English writing gets sentence patterns, not a model paragraph |
| B3 | 深入一层 opens a Socratic sub-agent, scoped to one block |
| B4 | Per-block comments: one summary line + concrete points, each traceable |
| B5 | 成稿 becomes a real writing surface |
| B6 | The title is hers, and editable anywhere |
| B7 | The whole-draft comment lives in the sidebar, traceable, and persists |

**Not in scope.** Merging 段落 and 成稿 (explicitly rejected: *"B5 means, for
成稿, make the left part a better writing experience. not merging."*). Switching
印记 to converse in English for an English piece. Any report surface. Any
reading-room change.

## 3. Standing rulings this obeys

1. **铁律① — AI never writes her prose.** Enforced by output *type* wherever
   possible, not by asking the model to behave.
2. **AI never authors an outline in lite** (2026-08-27, firm). `POST
   /outline/generate` stays deleted.
3. **结构 is planning, not a picker** (2026-08-27, firm). No skeleton library,
   no tile grid of structures.
4. **Product copy stays plain and short** in *labels*. This is about UI chrome,
   not about how 印记 teaches — see the revision below.
5. **Lite must never break pro.** Any shared file is called out explicitly.

### The one ruling this revises

The 2026-08-27 ruling *"name the pattern only AFTER she produces it — handing her
those words as options is what made the old screen a form"* is **narrowed to the
vessel, not the vocabulary**:

- 印记 **may** name accurate methods in **conversation** (留悬念 / 设问 /
  开门见山 / 并列 / 递进 / 正反 / PEE / 让步 / 因果), state why they matter, and
  offer to show examples.
- 印记 **may not** render them as a picker, menu, or tile grid.

Naming a pattern *after* she produces it remains the best teaching move; it is
simply no longer a prohibition on teaching one earlier.

### The cross-cutting copy rule (2026-08-28, firm)

> *"generally your guidance towards students are all this AI-feeling and not a
> human-feeling language."*

Every student-facing turn 印记 takes must do four things:

1. **Say why it matters** — one line of stakes, so the question isn't arbitrary.
2. **Name the real methods** — accurate, standard 语文 terms. The vocabulary is
   what she is here to learn; withholding it teaches nothing.
3. **Offer a genuine choice**, with room to decline.
4. **Offer to teach** — 「要不要我举几个例子，咱们一起看看」.

Rejected: 「有人会从一个具体场景切进去，有人直接抛个问题。你这篇你想怎么进？」
Correct: 「对于一篇文章来说，有意思的开头非常重要。留悬念、设问、开篇直接叙述等，
都是常见的方式。你想尝试哪一种？或者需要我给几个具体的案例我们一起来学习一下这几种
方法吗？」

**Rigor requirement.** A teacher is rigorous about terms. Method names and
definitions must be *accurate and standard*, never improvised — which is why they
live in a reviewed library (§5.2) rather than being generated per request.

**Mechanical cause to fix first.** `apps/api/internal/api/writing_plan.go`'s
output contract caps `reply` at 不超过 120 字 and instructs 不要一次问好几个问题.
Under those two rules the model *cannot* state stakes, name methods and offer to
teach — it emits the clipped shrug above. **The cap must be raised** (to ~200 字)
in every student-facing prompt this spec touches. 铁律③ still holds: one
*question* per turn. It never meant "say as little as possible".

**Examples must be borrowed material.** When 印记 offers 具体的案例, they are drawn
from the library, about topics other than hers. An "example 留悬念 opening"
written about her own topic *is* her opening, authored by AI — 铁律① through the
back door. This is enforced structurally by the examples being pre-authored
content, not model output (§5.2).

---

## 4. B0 — the whole piece, co-created

### The problem, precisely

`writing_plan.go`'s prompt walks an implicit ladder — thesis → reasons → evidence
— and stops. `writingPlanMaxDepth` (`writing_plan.go:65`) documents the map as
中心论点 → 分论点 → 论据. So the map can only grow claims, 段落 builds one slot per
map node, and the student gets boxes for the body of the essay and no box for how
it opens or how it lands.

### What we will not do

Insert 开头/钩子/结尾 nodes from a template, or offer a picker of opening types.
Both are killed by rulings 2 and 3.

### The design

印记 gains **judgment about a whole piece** and applies it to *her* map every turn.
The prompt carries, as 印记's own knowledge, what a finished piece must do for a
reader: earn attention at the top, actually argue in the middle, leave the reader
holding something at the end. This sits beside the spine and point-pattern
pedagogy already in that prompt — as knowledge, never as a checklist to march
through and never as a menu.

Each turn, 印记 reads the whole map and raises **the one thing this piece most
needs next.** For one student that is *"读者一上来不知道为什么要关心这件事"*; for
another it is *"理由二和理由三其实是同一条"*, or *"理由一底下什么都没有"*.
Sometimes the answer is that nothing is missing and she should go write.

**Co-creation runs in both directions:**

- 印记 may **offer a shape**, in teacher language, with method names and an offer
  of examples (§3's copy rule). Ruling 2 permits giving general structures; it
  forbids authoring her outline.
- She may **push back and reshape** — 「我不想要开头，我想先讲我那次的事」 — and
  that becomes the piece's shape. 印记 works with it rather than steering her back
  to a canonical form.
- Node text is **always** the compression of a sentence *she* said. Already
  structural: the plan endpoint is add-only, and a node with an invented
  `parentId` is dropped rather than reparented.

Two students therefore get two different skeletons. A narrative does not come out
shaped like an argument. Some pieces open on a scene; some open cold on the claim,
because she decided so.

### Structural consequences

- Opening and landing nodes are **top-level siblings at `depth=0`**, ordered by
  `position` — the opening before 中心论点, the landing after. `writing_outline`
  already encodes tree *and* order, so 段落 hands out slots in document order.
  **No schema change.**
- **The prompt line 「加在最上层（中心论点）就留空字符串」 must change** — it
  currently teaches the model that the only root is the thesis.
- **`MindMap.tsx` must be verified against multiple `depth=0` nodes.** It has
  only ever rendered one. This is a correctness requirement of this spec, not a
  nice-to-have.

### Accepted trade-off

A student who leaves planning early is never asked, and arrives at 段落 with no
opening slot. That stands: planning is not a gate, she can 加一段, and a forced
slot is the template we rejected. The whole-paper skeleton is a **tendency, not a
guarantee**, and that is deliberate.

---

## 5. B1 — guidance that is present, and teaches

### 5.1 Behaviour

**Generated up front, in one batched call**, when she opens 段落 — guidance for
every block at once, at roughly the cost of a single 卡住了？ click today.
Persisted (§8), so a reload does not re-spend it.

A guide box contains four parts, in this order:

| Part | What it is |
|---|---|
| 这一段要做的事 | One line on what this kind of block must accomplish for a reader |
| 常见的几种写法 | Accurate method names from the library, with our one-line definitions |
| 想一想 | 2–4 questions, in her own material |
| 要不要看几个例子 | Opens the library's worked examples for the named methods |

`卡住了？` remains as a **regenerate** affordance, not as the only way in.

### 5.2 The vocabulary library (new, curated, not generated)

A reviewed JSON set — the single source of truth for every method name 印记 is
allowed to use — shipped the way card specs already are: `go:embed` on the server,
imported at build time on the client.

Each entry carries:

- `id`, `name` (the accurate 语文 term), `applies_to` (opening / body / closing /
  any), `definition` (one line, ours),
- `examples[]` — two or three **pre-authored** illustrations on topics unrelated
  to any student's piece,
- for English: `patterns[]` — sentence frames with slots (§7).

**Three properties this buys, all structural rather than prompt-level:**

1. Examples are borrowed material *by construction* — the 铁律① back door in §3
   is closed by the data, not by instruction.
2. Terminology is accurate and stable, because we wrote it and reviewed it. 印记
   **selects and tunes; it never invents a term.** A method not in the library
   cannot be named.
3. Showing examples costs nothing.

**This is not the skeleton library that was deleted on 2026-08-27.** That one was
a picker that chose the shape of *her* essay. This is a bag of teaching examples
印记 reaches into mid-conversation; it never touches her structure and is never
rendered as a chooser.

### 5.3 The 铁律① guarantee, honestly split

`writing_guide.go`'s `parseWritingGuide` currently drops any entry not ending in
`？`/`?`. That is how 铁律① is enforced by *type* rather than by manners, and it
**must not be weakened**:

- `questions` keeps the `？` filter, unchanged.
- `这一段要做的事` and `写法` are new prose fields about the block's *job* and about
  *technique* — never about her topic. They are not covered by the `？` filter;
  their protection is that they are meta-level by construction, plus review.
- `examples` are **not model output at all** — they are library references. The
  model may return example ids; unknown ids are dropped.

### 5.4 Layout

The current box is a numbered list at `mk-body` (14px) inside a tinted panel —
*"a large paragraph of small texts is not easy to read."* The new box uses real
hierarchy: each of the four parts visually separated, one idea per line, questions
at `mk-body-lg` (16px/1.75) with generous spacing between them, method names as
readable inline emphasis rather than 11px `mk-label` chips.

---

## 6. B4 / B7 — comments that point back at her own sentences

### One shape, two zoom levels

> **一句总的话** — one summary sentence.
> Then **2–4 concrete points**, each carrying **an exact quote from her own text**.

Used for a single block (**B4**) and for the whole draft (**B7**). One object,
one renderer, one storage table.

### Traceability

The quote *is* the trace. Clicking a point highlights that sentence in her writing
surface and scrolls it into view.

**Server-side validation:** every quote must be a **literal substring** of the text
being commented on. Any point whose quote is not found is **dropped**, not
rendered with a best guess. A comment can therefore never point at a sentence she
never wrote. This is the same discipline pro applies in its report generator
(`ValidateRefs`).

### B7 also fixes an existing defect

Today 请印记看看's result is genuinely strong — on the walk it produced a
four-section critique (结构 / 论证 / 证据 / 语言) that caught a real defect in the
draft — and then **evaporates**: it renders once on 成稿 and never reaches the
finished screen. Persisted (§8), it survives, and sub-project **D**'s writing
report gets it for free.

Comments render in the 成稿 **right rail**, not inline in the draft.

---

## 7. B2 — English expression support

### What is there now, and why it goes

EN blocks offer 示范段落 — a whole model paragraph **on her own topic**
(`SnippetsStage.tsx`'s `ExemplarBlock`, and `generateWritingSnippetExemplar`).
The component is carefully built so nothing inside it can write to her textarea,
but the content itself is a paragraph about her thesis that she can retype. It is
also not what was asked for.

### What replaces it: 表达方式

Sentence **patterns with slots**, selected by block role, from the library:

> **Conceding, then turning** — `While it is true that ___, this does not mean ___.`
> **Qualifying a claim** — `To a large extent, ___ — though ___.`
> **Introducing evidence** — `A 2019 study of ___ found that ___.`

A pattern with blanks **cannot be pasted as an argument**; she must supply the
thinking to complete it. This is the same guarantee-by-type discipline as the `？`
filter, and it is *stronger* protection than the exemplar it replaces.

Selection is by block role: a 让步 block offers concessive patterns; an evidence
block offers citation patterns.

`generateWritingSnippetExemplar` and its route are **removed**.

### Deliberately unchanged

印记 still discusses an English piece **in Chinese** (`writing_plan.go`: 这篇用英文
写，但你和她用中文讨论). Whether that should change is a real question and is
explicitly out of scope here.

---

## 8. B5 / B6 — the writing surface and the title

### 8.1 B5 — 成稿 becomes a page, not a box

**Layout.** Left pane is the writing surface at full height. Right rail holds her
段落 blocks and 印记's comments — the snippets are *there* by default, not summoned.

**The draft is present on arrival.** 从段落拼出初稿 stops being a prerequisite; the
draft is assembled when she arrives. The button remains, demoted to
`从段落重新拼一次`, for when she has gone back and changed 段落.

**Re-assembly guard (silent-data-loss class).** Re-assembling after she has edited
on 成稿 would destroy that editing without a trace — the same shape as the historic
snippet-position bug. Therefore: re-pull offers itself freely **only when the draft
is unchanged since assembly**; otherwise it must confirm and state exactly what it
will overwrite.

**Typography.**

| Property | Value | Why |
|---|---|---|
| Size / leading | **new token `mk-prose` = 17px / 1.9** | The current box uses `mk-body` 14px/1.6 — chrome type doing a page's job. 1.9 leading matters more for Chinese: the glyphs are dense and full-height. |
| Measure | `max-width: 68ch` | `ch` is the width of a half-width digit, so 68ch is ≈34 Chinese characters *and* ≈68 Latin characters per line — both in the comfortable band. One value serves both languages. |
| Top padding | 48px | The first line must not sit jammed under the header. |
| Side padding | 32px, column centred in the pane | |
| Bottom padding | ~40vh | So the last paragraph can rest mid-screen rather than pinned to the window edge. This single detail is most of what makes a long-form editor feel calm. |
| Chrome | No border, **no focus ring** | A ring around the whole page is chrome logic applied to a page. |
| Surfaces | Writing area `--mk-paper`; right rail `--mk-surface` | The page reads as paper, the rail as furniture. |
| Caret / selection | accent / `--mk-accent-100` | |

`mk-prose` is added to `apps/web/tailwind.config.ts` (shared with pro). Adding a
`fontSize` key is purely additive and cannot alter pro's rendering, but it is a
shared file and is called out for that reason.

**Accepted limitation — it stays a `<textarea>`.** A textarea cannot space
paragraphs differently from lines. The alternative is `contentEditable`, which is
the first step toward the fancy document editor 铁律 rules out. At 1.9 leading a
blank line yields a ~32px gap, which is how iA Writer and Bear look. This is not a
compromise the student will perceive.

**Consequence for B4/B7.** Because it is a textarea, click-to-highlight cannot
highlight *inside* it. It requires a **mirrored layer** behind the textarea with
identical typography rendering the highlight spans. The two typographies must be
defined **once and shared** — if they drift by a single pixel the highlight lands
on the wrong line.

**Autosave.** Today 保存中… renders as text and **shifts the layout under her
hands**. It becomes a fixed-position, low-contrast marker that never reflows the
page.

**Placeholder.** `拼出来的初稿会出现在这里——你也可以直接在这儿写、改。` describes our
plumbing. It is replaced with something a teacher would say while handing someone
a blank page.

### 8.2 B6 — the title

The h1 is currently her raw idea sentence (「我想写：城市该不该大规模种行道树来降
温」) — a note to self, not a title. It becomes **click-to-edit, saved on blur,
available in all three steps**.

Her original idea sentence is separately preserved: it already lives in
`atom_message` as her own setup turn, so every downstream prompt keeps seeing what
she originally wanted while `title` becomes a real title she owns.

---

## 9. B3 — 深入一层, a Socratic sub-agent

### Who is talking

The **same 印记**. A right drawer headed 「印记 · 关于这一块」 — no second name, no
second avatar, no second character. This is a deliberate constraint: the reading
room spent a full session collapsing two AIs into one, and this must not
re-introduce them.

### Architecture

**Spawned server-side by the main agent**, not assembled by the client. This is
lite's first instance of the north-star pattern (*one continuous 印记 +
context-isolated sub-agents*).

The brief handed over:

- the paper's title
- the **structure** — the whole map, so it can see where this block sits
- the **current block** — role, heading, and what she has written in it so far
- the guide questions already shown, so it does not re-ask them
- `lang`
- the vocabulary library

**Deliberately excluded: the planning transcript.** That exclusion is what makes
the sub-agent focused and cheap, and it is a testable property (§11).

### Charter

A Socratic teacher: it asks, it works with what she says, it teaches from the
library, and it obeys §3's copy rule. Its examples come from the library's
borrowed material.

### Triggers

From a guide box, and from a comment point when she wants to push on it.

### Persistence

Turns persist **scoped to the block** — reopening the block returns to the
conversation.

### The guarantee, stated honestly

The endpoint has **no write path** to `writing_outline` or `writing_snippet`, so
it *cannot* author her outline or her text — that part is structural. But it is a
free-form chat, so the "output must be questions" type guarantee used in the guide
box does not apply here; that rests on the prompt, exactly as the main 印记's chat
already does. Handing it the library instead of letting it compose illustrations
is the main thing that narrows the surface. This is a known, accepted limit — not
an oversight.

---

## 10. Data model

One migration, entirely additive.

| Change | Purpose |
|---|---|
| `atom_message.block_id text NULL` | B3 thread scope. Mirrors `atom_card.block_id`, which already does exactly this. `NULL` = the room's main thread; set = a sub-agent thread on that outline node. |
| `writing_outline.guide jsonb NULL` | B1 persisted guidance. No new table. |
| **new table** `writing_comment` | B4 + B7. Columns: `id`, `writing_id`, `snippet_id` (nullable), `scope` (`block`\|`draft`), `summary text`, `points jsonb`, `created_at`. |

**Sequence numbers stay in one space per atom**, so `atom_message_seq_idx`
(`UNIQUE (atom_id, seq)`) is untouched and global ordering remains well-defined.

**No schema change** for B0 (uses existing `depth` + `position`), B2 or the library
(JSON files), B5 or B6.

---

## 11. Risks

### 11.1 The one that matters most — `atom_message` scope leak

`atom_message` has had **no scope column** since it was created
(`0092_atom_substrate.sql:29`). Every existing query that reads it therefore
assumes every row belongs to the main thread. The moment `block_id` exists, **any
read left un-audited will pull sub-agent chatter into 印记's planning window** —
the room's main context silently poisoned by side conversations, with nothing
failing loudly.

The migration is the easy half. **The implementation plan must enumerate every
`atom_message` read by name** and state what each does about `block_id`. "Update
the queries" is not an acceptable plan step here.

### 11.2 `MindMap.tsx` and multiple roots

It has only ever rendered one `depth=0` node. B0 produces several. Must be
verified, not assumed.

### 11.3 The e2e walk breaks silently

`apps/lite-web/e2e/writing-walk.spec.ts` broke **twice in one day** during the
2026-08-27 redesign, both times while still *looking* green — it was asserting
against endpoints that had been deleted. A broken e2e walk is worse than none,
because it reads as coverage. It is updated **in the same commit** as the room
change, never after.

`apps/lite-web/tsconfig.json` includes only `["src","test"]`, so **e2e is not
type-checked** by the normal command. It needs an explicit
`npx tsc --noEmit … e2e/<file>` plus `npx playwright test --list`.

### 11.4 Mirrored-layer drift

If the textarea's typography and the highlight layer's typography are defined in
two places, they will drift and highlights will land on the wrong line. Define
once, share.

### 11.5 Shared files

`apps/web/tailwind.config.ts` is pro's. The `mk-prose` addition is additive, but
the full pro suite (1291 tests) plus typecheck plus bundle runs before this ships,
per the standing rule.

---

## 12. Testing

**Go**

- `parseWritingGuide` applies the `？` filter to `questions` **and only** to
  `questions`; the new prose fields survive; unknown example ids are dropped.
- The comment validator drops any point whose quote is not a literal substring of
  the commented text, and keeps the ones that are.
- The B3 brief builder contains title, map and block, and **does not contain the
  planning transcript**.
- Main-thread `atom_message` reads exclude rows with a non-null `block_id`.
- Plan turns still cannot add a node whose text the model invented, and still
  cannot update or delete a node.

**Lite web (vitest)**

- The guide box renders all four parts, and renders with guidance already present
  on first paint (not after a click).
- Clicking a comment point anchors to the correct span in the mirrored layer.
- `从段落重新拼一次` refuses to silently overwrite an edited draft.
- The title edits and persists from all three steps.

**e2e** — the writing walk is extended to cover: guidance present on arrival, a
top-level opening node reaching 段落 as a slot, and a comment tracing back to a
real sentence.

**Pro** — full suite + typecheck + bundle.

---

## 13. Deferred, with reasons

| Item | Why not now |
|---|---|
| 印记 conversing in English for EN pieces | Real question, orthogonal to this room's problems |
| Merging 段落 and 成稿 | Explicitly rejected by the user |
| Guidance that refreshes as she writes | Considered and declined: N calls per block, and questions that move under her mid-sentence |
| Reading-room depth (**A**), the two reports (**C+D**) | Separate sub-projects, separate specs |

# The Lite Writing Room: From Blank Boxes to a Scaffold

**Date:** 2026-08-27
**Scope:** `apps/api` (migration 0100 + five endpoints), `apps/lite-web` (the whole 写作 room), both landings.

## What the walk found

The room had shipped, and it worked in the sense that every endpoint returned
200. Walking it on production as a real student found four things, none of
which a unit test would have caught, because none of them were failures.

1. **The room was silent.** Open a new writing and your own opening sentence
   sits alone in the coach rail. Nothing greets you, nothing frames the task,
   nothing happens until you speak first. There was no "AI guiding feeling"
   because there was no guidance — not bad guidance, none.
2. **目标字数 appeared broken.** `PUT /target-words` returned **200** and
   stored the value correctly. But nothing confirmed the save, and no screen
   ever displayed the number again. From the student's side, typing 800 and
   clicking 定下来 did nothing at all. A silent success is indistinguishable
   from a failure.
3. **The tool cards were parked on screen permanently** — all four, in the
   rail, whether or not anything called for them. Pushed, not offered.
4. **Freely-typed ideas were forced to `lang: "zh"`.** Only the preset topic
   tiles passed a real language, and 示范段落 is gated on `lang === "en"`, so
   a student who typed her own English essay idea silently lost every
   English-only affordance with no way to discover why.

And one thing the walk didn't need to find, because it was in the code: 大纲
had a 「帮我拟一份候选」 button that sent her whole transcript to the model and
got a finished outline back. She then "edited" something already written for
her.

## The ruling

> **AI never directly generates the outline. AI helps students think, gives
> general structures, and guides students to fill each block with their own
> thoughts and experiences.**

This is narrower than AGENTS.md's general 铁律 boundary, which explicitly
permits deriving an outline from a stated research question as a
deterministic system step. That permission holds for **pro**, where the
student already has a research question and the outline is scaffolding around
it. It does **not** hold for lite, where the outline *is* the thing being
learned. Generating it steals the lesson.

The rest of that sentence — *"gives general structures, and guides students to
fill each block"* — is what took two attempts to get right. See below.

## What replaced it — twice

The first replacement was a **fixed library of skeletons**: the model picked
one (立场式议论, 让步式议论, …) and the blocks were laid out with generic
labels. It shipped, and seeing it on production immediately showed the problem:

> 你的立场 / 反方最强的说法 / 你承认它哪一部分是对的 / 你的回应 / 结论
> — this looks very rigid and not easy to understand.
>
> 结构 is like planning, is not a fixed one, but guide students to think, then
> compose the structure.

「你承认它哪一部分是对的」 is not a phrase you say to a 13-year-old, and a
shelf of four skeleton cards is still a form. So the library went too.

**结构 is now a full-screen planning conversation with a live mind map.**

印记 asks one question at a time — first the one sentence this piece is really
making, then what she will use to show it, then what goes under each point —
and every answer grows onto a canvas at the right. The panel appears only once
the map has something on it, the way an artifact pane does: an empty panel
from the first second is a promise the screen hasn't kept.

The map is `writing_outline` rendered as a tree (`depth` + `position` already
encode one), so when she leaves for 段落 **the thing she planned IS the
outline**. No conversion step, nothing lost between them, and no new table.

### The hard guarantee

A planning turn can only ever **ADD**. `writing_plan.go` contains no update
and no delete call, so 印记 can put her sentence on the map but can never
rewrite or remove one — that is hers, through the PUT her own edits use.
Pinned by a test that drives two turns and asserts the first node comes back
byte-identical. A node whose `parentId` the model invented is dropped rather
than reparented: specific-and-wrong is worse than absent, and she can always
say it again.

### Where the writing pedagogy lives

In the prompt, as 印记's own knowledge — never as a menu:

- **Spines**: 立场式 / 起承转合 / 钩子式 / 记叙
- **How one point gets made**: 并列, 递进, 正反, 举例（PEE）, 让步, 因果

The teaching move is that a pattern is **named only after she produces it**.
She gives three parallel reasons, then hears 「你这三条是并列的」. Handing her
those words as options to pick from is what made the old screen a form.
Students don't fail at naming structures; they fail at having anything to put
in one, so the questions extract material first.

The third level (evidence under a point) is recommended where a point sounds
hollow and never required, and 去写 is available from the first render.
Planning is a surface, not a gate.

## The guiding box

> *"snippets is important, the key is the AI-generated guiding box, instead of
> letting students write paragraph by paragraph."*

Every block now carries 「卡住了？」 → `POST /outline/{oid}/guide`, which
returns **2–4 questions** about that block, grounded in her own material and
in what she wrote in the sibling blocks.

Questions, and only questions. `parseWritingGuide` drops any entry that does
not end in a question mark before it ever reaches the client. This is the
point: a demonstration paragraph can be pasted into an essay, a question
cannot. 铁律① is enforced by the output *type*, not by asking the model
nicely.

The response shape is `{questions}` and nothing else. There is deliberately no
second field a sentence could arrive in.

## The rest

- **Setup dialog on entry**: language, optional length, and — instead of a
  文体 selector — an open box, *"还想说点什么？"*. A lite student may not know
  what 文体 means, and a dropdown of words she cannot parse is a worse start
  than no question. The model infers genre from her own sentences. The note is
  stored as one of *her* turns in the transcript, not in a column of its own,
  so every downstream prompt sees it for free.
- **The coach speaks first** (`POST /opening`): names her topic back, says what
  the next step is, asks one answerable question. Idempotent server-side — a
  refresh replays the greeting rather than re-greeting her, and costs nothing.
  Deliberately does *not* lock her composer while it runs.
- **目标字数 is visible**: a header chip showing `已写 320 / 800`, live, click
  to edit. `countWords` is imported from pro's own counter rather than
  reimplemented — the last hand-rolled copy counted characters and read ~5×
  high on English.
- **The tool cards are gone from this room entirely.** Not just the shelf —
  the deck, the summon door (`writing_lens.go`), the five writing-scoped card
  routes, all of it. Pro's own writing surface barely used them, and a student
  stuck on a paragraph wants a question, not a form. The reading room keeps its
  学科透镜: those are used *against an article*, which gives them something real
  to bite on.
- **Four stages became three**: 结构 / 段落 / 成稿. 构思's only content was the
  dead word-count box; what it was *supposed* to host is now the planning
  conversation itself.
- **The map is a canvas, not a list.** Dot-grid ground, branches from flexbox,
  connectors as measured cubic beziers in an SVG underlay (borders cannot
  curve, and hand-rolled tree arithmetic breaks the moment a node wraps to
  three lines). A new node animates in from the direction its connector grows,
  its edge draws itself via `pathLength={1}`, and the canvas pans to whatever
  just appeared. Leaves stack below their parent so three Chinese columns fit
  a side panel.
- **Language is asked, not guessed.** `lang` at creation is now provisional and
  the dialog overwrites it before anything else happens.

## The tiles

The landing shelves' tiles were restyled *"more technical-feeling, instead of a
bad left-border line box"*, and the two module-private near-duplicates became
one shared `PromptTile`.

- The 3px coloured left stripe read as a highlighter mark on a document — a
  stationery gesture. Colour survives as one small square in the meta row.
- Lift-and-shadow on hover made each tile a brochure. Hover now moves a single
  hairline along the top edge and resolves a corner arrow; the tile does not
  move.
- The pill tag became a monospaced, letter-spaced label beside a zero-padded
  index. Tabular numerals are most of what makes a surface read as an
  instrument.

## Migration 0100

Additive only — no column is dropped or retyped, so pro is untouched.

- `writing.structure_key`, `writing.setup_at`
- `writing_outline.role` — the generic label, kept strictly apart from `text`
  (hers). Merged, there would be no way to tell the template from her
  thinking, and that distinction is exactly what the process report reads.
- Existing `stage='ideate'` rows → `'outline'`; the column default follows.
  The CHECK constraint is deliberately left permissive; `validWritingStages` in
  Go is the enforcement point, so a stale client gets a clean 400 rather than
  parking a writing on a stage with no page behind it.
- Every existing writing gets `setup_at = now()`, so nobody mid-piece is
  ambushed by a dialog.

## Verification

- Go: full suite green, including new `writing_structure_test.go` /
  `writing_setup_test.go`. The load-bearing one is
  `TestApplyWritingStructure_LaysOutRolesWithEmptyText` — it fails if any block
  ever arrives pre-filled.
- lite-web: 90 tests green (was 82).
- **pro: 1291 tests green, typecheck clean, bundle builds** — the standing
  "lite must never break pro" check.
- `internal/store`: four TestWritingStore_* failures caught by the full suite
  and fixed — the default stage moved to `outline`, and ReplaceWritingOutline
  gained a parallel `roles` array whose omission NULL-pads against a NOT NULL
  column. That second failure is the parallel-array guard doing its job.
- The Playwright walk (`apps/lite-web/e2e/writing-walk.spec.ts`) was rewritten:
  it had been silently broken by this redesign, waiting on the deleted
  `/outline/generate` and driving a four-step map. It now asserts the four
  mechanical 铁律 proofs against a real API and database.

## Not done

- The reading room needs the same treatment, and it is the same idea: a task
  list (general scan → focal paragraphs → lens → information → questions) plus
  per-paragraph tools — 翻译 / 关键单词 / 语法 / 写作解析 for English, 成语修辞
  / 案例 / 结构解析 for Chinese. The task list should come from a **fixed
  library of routines** with the AI selecting and tuning one, for the same
  three reasons the writing skeletons are fixed. Then "generated according to
  student ability and text difficulty" grows the selection inputs rather than
  forcing a rewrite. Spec pending.

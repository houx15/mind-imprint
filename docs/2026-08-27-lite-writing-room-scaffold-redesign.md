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

## What replaced it

The model's job went from **authoring** to **selecting**.

`writing_structures.go` holds a fixed library of skeletons — 立场式议论,
让步式议论, 说明文, 记叙文, and four English counterparts. Each is a list of
generic block labels: 「你的立场」「反方最强的说法」「你的回应」. Identical for
every student in the school. Not one word of any block is about her topic.

Three reasons it is data and not a model call:

- **It has to be generic.** A model asked to invent block names will
  inevitably produce ones carrying her subject ("手机对睡眠的影响") — and at
  that moment the AI has done her thinking. Hardcoded, that cannot drift.
- **It has to be stable.** A student writing her second argument essay should
  recognise the skeleton from the first. Serving fresh "AI inspiration" each
  time teaches her to depend on novelty, not to learn a shape.
- **It has to be cheap.** Laying out a skeleton is deterministic. It should
  not cost a model call.

What the model still does: `POST /structure/recommend` picks **one key from
that table** and gives a one-line reason about *shape*, never content. If it
invents a key, or picks the other language's, the response is rejected outright
— no silent substitution of a default, because a substituted default is
indistinguishable from a real recommendation and she would never know the
coach hadn't looked.

The 铁律 line is expressed as data, not as a prompt: applying a skeleton writes
`text = ""` for every block. There is no field anywhere on that path that could
hold a thesis.

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

The guide may also nominate one tool card, which is rendered as an offer she
taps — the same propose-then-confirm shape the cards already use (铁律②).

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
- **The card shelf is gone**, replaced by one small 工具卡 button plus the
  guide's nominations.
- **Four stages became three**: 结构 / 段落 / 成稿. 构思's only content was the
  dead word-count box; what it was *supposed* to host now happens where it
  belongs — in the opening line, and in the per-block questions at the moment
  each block is actually being written.
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

## Not done

- The reading room needs the same treatment, and it is the same idea: a task
  list (general scan → focal paragraphs → lens → information → questions) plus
  per-paragraph tools — 翻译 / 关键单词 / 语法 / 写作解析 for English, 成语修辞
  / 案例 / 结构解析 for Chinese. The task list should come from a **fixed
  library of routines** with the AI selecting and tuning one, for the same
  three reasons the writing skeletons are fixed. Then "generated according to
  student ability and text difficulty" grows the selection inputs rather than
  forcing a rewrite. Spec pending.

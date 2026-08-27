# Lite Reading Room: Task List + Paragraph Tools

**Date:** 2026-08-27
**Status:** ✅ BUILT AND LIVE (2026-08-27) — `05854287` (backend + migration
0101) and `23ca3bcd` (UI).

Walked on production against a real English article: 印记 picked *Close Read*,
singled out b2/b3 — the mechanism paragraphs, not paragraph one — and 语法 on
b2 isolated the genuinely hard sentence, naming its 主干, the 破折号
apposition, and the reduced relative clause.

Deviations from this design, all decided during the build:

- **No ability signal, as predicted.** Routine selection uses the article's
  language and content only. Adding ability later costs nothing structural,
  which was the whole reason for a fixed library.
- **Language is detected from the article, never asked.** She already said what
  she wants to read by pasting it; a picker on top of that is a question whose
  answer is sitting right there.
- **The tools open from a small 拆开这一段 button, not a paragraph click** —
  clicking a paragraph already means "quote this one" in this room, and
  overloading a working gesture to add a new one breaks both.
- **The quiz step ships as a prompt, not a graded exercise.** It is a task-list
  step with a label and no checking path. Making it checkable is a separate
  decision — open question 2 below is still open.
**Sibling:** `docs/2026-08-27-lite-writing-room-scaffold-redesign.md` (shipped)

## The idea in one line

The same shape the writing room just took: **the AI builds the scaffold, the
student fills it with their own thinking.** Writing gets a structure skeleton
plus per-block guiding questions. Reading gets a **task list** plus
**per-paragraph tools**.

## Why reading needs it

Lite students are not IB students with a research question. Handed an article
and a chat box, a middle-schooler does one of two things: reads it once and
says "读完了", or asks the AI what it means. Neither is reading.

The product note that started this:

> *for lite-level students, they first need to be taught about the paragraphs.*

That is the missing rung. Before you can read *through a lens* — the thing the
room already does well — you have to be able to read a paragraph: what it
says, what's hard about it, what it's doing in the argument. The room currently
assumes that skill instead of teaching it.

## Part 1 — Paragraph tools

The article is already split into addressable blocks (`Block` in
`reading_source.go`; annotations already anchor on `blockId`), so the anchor
exists. Clicking a paragraph opens a small tool row, and the tools differ by
language because the skills differ by language:

**English paragraph**
| Tool | What it returns |
|---|---|
| 翻译 | A faithful Chinese rendering of this paragraph. |
| 关键单词讲解 | The 3–5 words actually worth learning here, each with meaning in context — not a dictionary dump. |
| 语法讲解 | The one or two structures that make this paragraph hard to parse. |
| 写作解析 | What this paragraph is *doing* — claim, example, concession, transition. |

**Chinese paragraph**
| Tool | What it returns |
|---|---|
| 成语 / 修辞运用 | The figures actually used here, and their effect. |
| 案例 | The concrete instances this paragraph rests on, and what they're evidence *for*. |
| 结构解析 | The paragraph's move within the piece. |

### 铁律 check

These are **explanatory**, and that is why they are safe. 铁律① forbids the AI
writing the student's own prose. Explaining someone *else's* published
paragraph is not that — it is what a teacher does, and withholding it would
just make the room less useful without making it more honest.

The line to hold: these tools explain **the article**. None of them may ever
touch the student's own 摘要 / 批注 / takeaway. Enforced structurally — every
tool is keyed on a `blockId` of the article and has no write path into any
student-authored field.

### Cost

These are cheap, frequently-clicked, per-paragraph calls. Two consequences:

1. **Cache by (blockId, tool).** A paragraph's translation does not change.
   Persist the result and replay it — a student re-opening 翻译 on the same
   paragraph must never pay twice. This also makes the tools feel instant on
   second visit, which matters more than it sounds.
2. **Chaperone tier**, not flagship. These are explanation, not judgement.

## Part 2 — The task list

> *an intelligent reading coach, I think, would generate a task list after
> students giving a paragraph. this task list, in the later, would be generated
> according to students' ability and the paper's difficulty.*

The shape named in that note:

```
general scan  →  understanding
look at one/two focal paragraphs
read through a lens
think about information
finish the questions
```

### The architectural decision: a fixed routine library

The task list must come from a **fixed library of reading routines**, with the
AI **selecting and tuning** one — never inventing tasks freely. Same three
reasons the writing skeletons are fixed, and they transfer exactly:

- **Generic.** A model asked to invent tasks will produce ones carrying the
  article's content ("找出作者对碳排放的三个论据") — and at that point the AI
  has done the noticing, which was the task. A routine step says *"找出这一段
  最难的一句话"*; it cannot pre-empt her.
- **Stable.** A student's fourth article should feel like the same method as
  her first. That recognition IS the learning. Fresh AI-invented task lists
  teach nothing but novelty.
- **Cheap and honest.** Laying out a routine is deterministic. The one model
  call picks which routine and tunes its parameters.

What the AI *does* choose, per article:
- **which routine** fits (a dense argumentative piece and a narrative want
  different ones);
- **which 1–2 paragraphs are focal** — this is real judgement, grounded in the
  article, and it is the single most valuable thing the coach can contribute;
- **which lens** to offer;
- **how many steps** to include (short piece → trim the routine).

Crucially: *"later, generated according to students' ability and the paper's
difficulty"* then costs **nothing structural**. Ability and difficulty become
additional inputs to the same selection call. The routine library does not
change. That is the whole reason to build it this way now.

### Proposed routines (v1)

| Key | For | Steps |
|---|---|---|
| `scan-focus-lens` | Default. Argumentative / expository. | 通读一遍 → 精读 1–2 段 → 用一个透镜再看 → 这篇给了你什么信息 → 回答问题 |
| `narrative-trace` | Story, memoir, reportage. | 发生了什么 → 关键转折那一段 → 作者想让你感觉到什么 → 你信吗 |
| `en-close-read` | Short, dense English. | 整体大意 → 逐段过关键词与句法 → 作者的态度 → 用自己的话复述 |

Each step declares its `kind` so the runtime knows what to render:
`read` / `focus_block` / `lens` / `reflect` / `quiz`. Adding a routine is a
data change; adding a *kind* is a code change. Same test as the tool cards.

### Progress, not gating

铁律②. The task list is a **map** — every step is always clickable, skipping is
recorded rather than prevented, and there is no "you haven't unlocked step 3".
Identical to the writing stage map, for identical reasons.

## Data model (sketch)

- `reading.routine_key text NOT NULL DEFAULT ''` — which routine, mirroring
  `writing.structure_key`.
- `reading_task` — one row per step: `atom_id`, `position`, `kind`, `label`,
  `block_id` (nullable, for `focus_block`), `status`
  (`pending|done|skipped`), `completed_at`. Status is process evidence, so
  skipping is data, not a hole.
- `reading_block_note` — the paragraph-tool cache: `atom_id`, `block_id`,
  `tool`, `body`, `created_at`, unique on (`atom_id`,`block_id`,`tool`).
  **Model output, clearly marked as such** — it must never be mistaken for her
  annotation, in the schema or in the report.

## Endpoints (sketch)

```
GET  /readings/routines?lang=            fixed library
POST /readings/{id}/routine/recommend    model picks + names focal blocks
POST /readings/{id}/routine              lay out the task list
POST /readings/{id}/tasks/{tid}/done     mark a step (or ?skip=1)
POST /readings/{id}/blocks/{bid}/explain {tool} → cached explanation
```

## Open questions for sign-off

1. **Does the task list replace the current room, or sit beside it?** The room
   already has 摘要 / 批注 / 透镜 / takeaway, and they work. My recommendation:
   the task list becomes the room's **spine** and those existing surfaces
   become what individual steps open — not a parallel panel competing with
   them.
2. **Quiz step.** *"finish the questions"* — are these comprehension questions
   the AI generates from the article (answerable and checkable), or reflection
   prompts with no right answer? These are different features. My read is the
   former for lite, but it is worth confirming, since checkable questions mean
   a grading path and the first place in lite where the AI says "不对".
3. **Ability signal.** Nothing in lite currently measures reading ability.
   Until something does, routine selection uses text difficulty only. Worth
   confirming that's acceptable for v1 rather than blocking on an ability
   model.

## What this is not

Not a course. The course runtime already exists and is a different thing —
authored slices, a fixed script. This is a scaffold generated per article,
around the student's own material.

---

# Amendment · 2026-08-27, later the same day

Everything above shipped, and then walking it produced two rulings that
supersede parts of it. The original text stays as written — it is the record
of how we got here — but where the two disagree, this section is what is in
the product.

## Ruling 1 — 印记 leads. She does not manage stages.

> *merge the tasks with the AI bar. at start the AI begins with 让我来带你详细
> 阅读这篇文章吧, clicks 开始button. then AI generates the plan, introduces the
> plan, then we will enter a stage directly. student doesn't handle the stages
> themselves, but the AI directs these. so 是否完成，进行到哪一步, is guided by
> AI.*

This retires **Part 2's checklist as an interface**. The routine library, the
`reading_task` rows and their `pending|done|skipped` status all survive
unchanged — what dies is the student operating them.

Concretely:

- No 做完了 / 跳过 buttons, and no clickable steps. The step list is rendered
  small, above the conversation, as **progress she can see** — not controls
  she works. `POST /tasks/{tid}/done` is gone.
- The coach's reply carries `advance` (`""` / `"done"` / `"skipped"`),
  clamped server-side to those three. It decides whether her answer counted.
- Skipping did not disappear; it moved into language. She says 这步跳过吧 and
  the coach records `skipped` without arguing. 铁律④ is intact — the skip is
  still data — and 铁律② is better served than before: a student who wants
  out of a step should not have to find the button that admits it.
- The plan is generated on demand at the first turn, so 开始 is the only
  thing she ever presses to begin.

This also answers Open question 1: the task list is neither a spine nor a
parallel panel. It is the coach's own agenda, shown for legibility.

## Ruling 2 — 想一想 and 仿写 are instruments, and 印记 picks them up

> *in 带读, AI 拆解 one paragraph, propose some guiding questions, or let
> students mock that to write a paragraph* … *for the 仿写 or questions, these
> are tools and AI can decide how to guide students to read deeper. though
> different interactions.*

Part 1 described paragraph tools as explanatory only — 翻译 / 关键单词 / 语法 /
写作解析 for English, 成语修辞 / 案例 / 结构解析 for Chinese. Two more join
them, and they are not explanations:

- **想一想** — guiding questions about this paragraph.
- **仿写** — an invitation to write one of her own on the same pattern.

Both are language-neutral, so they exist on every article.

They are also the point at which the coach stops suggesting and starts
teaching. The coach's reply carries a `tool` alongside `focusBlock`, and the
panel that opens **runs that tool itself**. If it merely lit up a button she
had to find, the coach would have made a suggestion; teaching means the
explanation is already there when she looks.

Validation is entirely server-side and entirely by construction. A tool is
dropped before she ever sees it if it does not exist, if its language does
not match the article, or if the turn named no paragraph to apply it to. The
coach cannot invent an instrument it does not have.

### 铁律① under 仿写

仿写 is the one tool that touches writing, so the line is drawn in the data
rather than in the prompt: **the 仿写 renderer has no field a sample paragraph
could live in.** It can name the pattern and set the task; it has nowhere to
put a model-written paragraph even if it tried. 想一想 is enforced the same
way — every entry that does not end in `？` or `?` is dropped
entry-by-entry, so a "question" that is secretly an assertion cannot survive
the parse.

The general rule this is an instance of, and the one to reach for next time:
**enforce restraint in the output type, not in the prompt's manners.**

## Fix on the same walk — paragraph ordinals

The coach is told to say 「第几段」 and never `b1`/`b2`, but the prompt used to
list paragraphs as bare ids, leaving it to map ids onto ordinals in its head.
On production it split: the prose said 第三段 while `focusBlock` came back
`b4`, so a tool opened on a paragraph the sentence had not named. Both prompts
now label each paragraph `b3（第3段）`, counting every block so the ordinal
keeps matching her screen. The inference is removed rather than the model
asked to be careful.

## Still open

Open question 2 (are the quiz questions checkable and graded?) is still
open, and is now narrower: 想一想 answered the *reflection* half, so what
remains is only whether lite ever gets a right/wrong path. Open question 3
(no ability signal yet; selection uses text difficulty only) is unchanged.

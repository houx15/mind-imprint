# Lite 阅读房间 · 深度 — Design Spec

> **Sub-project A** of `docs/2026-08-28-lite-edition-feedback-backlog.md`.
> Sub-project B (写作房间) shipped at `main @ 250bce3d` / `cc0eaf19`.
> C+D (the two reports) follow this one.
>
> **Approval:** the user gave blanket approval for this sequence — *"you can go
> through different parts, follow your pace, write spec/plan and use subagent
> driven implementation and move to a next one and continue"* — after answering
> the four clarifying questions recorded as R1–R4 in the backlog. R3 binds this
> spec directly.

## The problem

The reading room's shell is right. The user's words: *"I love the current box!"*
What is missing is **depth inside it** — four specific gaps:

1. 带读 can point at a paragraph and open a paragraph tool on it, but it **cannot
   summon a lens card** aimed at a sentence, demonstrate the analysis there, and
   hand the same move back to her. She can only reach a lens by opening 透镜库
   herself.
2. The closing step is *回答几个问题* — typed answers. It should be a **task
   performed in the article**: go find the sentence that does X.
3. Nothing in the route bends toward **her own experience**.
4. Reading **dead-ends**. Finishing produces a takeaway and nothing else; the
   article never becomes something she might write about.

## What is already there (and why this is smaller than it looks)

The map that produced this spec found that most of the machinery exists:

- **The demonstrate → you-do mechanic is built and enforced.** A lens card at
  `status="proposed"` shows an AI-grounded example sentence with 「为什么是这句」;
  at `status="active"` she must click a *different* sentence, and a click that
  overlaps the AI's example is **rejected** (`HangingCard.tsx:106-135`, overlap
  check in `ReadingRoom.tsx`). This is exactly the move A1 asks for. It is only
  the *trigger* that is missing.
- **Aiming a card at a block, with a verbatim-validated example, already exists
  in Go.** `agent.RouteReading` + `agent.ResolveExampleAnchor`
  (`reading_gate.go:71-99`) validate `ExampleQuote` against the article before a
  card is minted. It is reachable at `POST /readings/{id}/turn` — but
  `ReadingCoachPanel` only ever calls `/coach`, and `ReadingRoom` replaces its
  own chat branch whenever `renderCoach` is supplied. **The whole path is dead
  code from the lite UI's perspective.**
- **The coach already aims at paragraphs.** `readingCoachReply` carries
  `FocusBlock` and `Tool`, both validated against real block ids and the tool
  table (`reading_coach.go:224-272`).
- **She can already quote a sentence to the coach** — the composer carries
  quote chips (`ReadingCoachPanel.tsx:161-169`).

So A1 is *wiring a validated mechanism to the surface that replaced it*, not
inventing one.

---

## The design

### A1 · 带读 summons a lens onto a paragraph

`readingCoachReply` gains one field:

```go
type readingCoachReply struct {
	Reply      string `json:"reply"`
	Advance    string `json:"advance"`
	FocusBlock string `json:"focusBlock"`
	Tool       string `json:"tool"`
	// Lens is the reading-deck card id the coach reaches for this turn, if
	// any. A lens is the heavier instrument: the paragraph tools explain a
	// paragraph, a lens makes her perform an analysis on one of her own
	// choosing. Empty on most turns.
	Lens string `json:"lens"`
}
```

**Validation drops rather than fails** — the discipline the existing fields
already follow. `Lens` survives only if all of these hold:

| Check | Why | Where it already lives |
|---|---|---|
| id is in `agent.ReadingDeck()` | a card no renderer can resolve | `inReadingDeck` |
| `readingOrderingGuard` allows it | CRAAP-before-SIFT | `reading_turn.go:253-270` |
| no lens is currently open | the one-open mutex | scan + `atom_card_one_open_idx` |
| `FocusBlock` is non-empty and valid | a lens with no paragraph to aim at is the un-aimed lens she already had | `parseReadingCoachReply` |

The last row is the one that makes A1 *the thing that was asked for*. An
un-aimed coach summon would be indistinguishable from her opening 透镜库
herself. **The coach must name the paragraph, or it doesn't get the lens.**

**Minting.** The body of `liteSummonCard` (`reading_lens.go:95-240`) is
extracted into a reusable helper:

```go
// summonReadingLens mints one lens card for an atom, grounding an illustrative
// example first. preferBlock, when non-empty and real, restricts the grounding
// call to that single paragraph so the example lands where the caller aimed —
// which is what separates the coach's aimed summon from the student's
// open-ended one from 透镜库.
func (a *API) summonReadingLens(
	ctx context.Context, userID uuid.UUID, atomID uuid.UUID,
	cardID string, preferBlock string,
) (summonedLens, error)
```

`preferBlock` is implemented by **filtering the `blocks` slice passed to
`agent.ProposeCardExample` down to that one block**. Block ids are
position-derived and preserved, so the returned anchor's `BlockID` is still
correct and `ResolveExampleAnchor`'s guarantees are untouched. No change to any
`agent` package function signature.

**Origin.** The coach's summon records `origin='router'`, not `'student'`. She
did not choose this lens; recording otherwise would falsify the one autonomy
signal on the row (铁律④). Both values already exist — no migration.

**She still confirms.** The card arrives `proposed`; 打开 remains a separate,
recorded step. 铁律② is untouched: the trigger is automatic, the opening is hers.

**Failure is silent and safe.** If minting fails or is declined (mutex lost,
ordering guard), the coach's `reply` still stands and the turn completes without
a card. She is never shown an error about an instrument she didn't ask for.

### A2 · The closing step becomes a hunt

A new `readingTaskKind`:

```go
// 找一找：不是打字回答，是回到文章里把某样东西点出来。
taskHunt readingTaskKind = "hunt"
```

The routine library's own comment sets the rule: *"Adding a KIND is a code
change; adding a ROUTINE is a data change."* So this is a code change, and all
four routines swap their trailing `taskQuiz` step for `taskHunt`. **`taskQuiz`
is removed from the library and from the kind enum** — leaving both kinds alive
would mean the model can pick a routine that still ends in typed questions,
which is the thing being replaced.

The step's `Detail` says what kind of hunt, in the routine's own voice:

| Routine | Closing step |
|---|---|
| `zh-scan-focus-lens` | 「回去找一句」 · 在文章里点出**最能撑住作者观点的那一句**。点出来，我们看看它撑不撑得住。 |
| `zh-narrative` | 「回去找一句」 · 点出**你觉得写得最好的那一句**——不是最重要的，是最好的。 |
| `en-close-read` | 「Go find it」 · 在文章里点出**你觉得最难、但现在读懂了的那一句**。 |
| `en-argument` | 「回去找一句」 · 点出**作者最没说服你的那一句**。 |

**The wire change that makes it real.** `POST /readings/{id}/coach` gains an
optional structured field:

```ts
type CoachTurnBody = {
  text: string;
  picks?: { blockId: string; quote: string }[];
};
```

Today the composer's quote chips are inlined into `text` as markdown
blockquotes (`ReadingCoachPanel.tsx:102-112`) — the server cannot tell a
sentence she *pointed at* from one she *typed*. With `picks`, it can:

**Server-side validation (structural, not prompt manners):** every pick is kept
only if `blockId` is a real block **and** `quote` is a **literal substring of
that block's text**. Invalid picks are dropped. This is the same validator shape
as B4's comment quotes (`validateCommentPoints`), for the same reason: a
guarantee you can check beats a guarantee you asked for.

The surviving picks are rendered into the prompt as their own section, and the
coach is told, for a `hunt` step:

> 这一步她必须**真的在文章里点出句子**。她只是说「我觉得是第三段那句」——那是
> 说的，不是点的，advance 留空，请她点出来。

`text` keeps carrying the blockquotes as well, so the transcript still reads
correctly and nothing regresses if the client is old.

**Client.** When the current task's kind is `hunt`, `ReadingCoachPanel` shows a
standing hint above the composer — 「在文章里点出那一句，点了就会出现在这里」 —
and the existing quote chips become the answer channel. She may still type. The
hunt is an invitation with a clear affordance, not a locked door (铁律②).

### A3 · One step that is hers

A second new kind:

```go
// 联系你自己：把这篇跟她自己的经历、见过的事、原本的想法接上。
taskConnect readingTaskKind = "connect"
```

**Every routine gets exactly one.** That is the guarantee, and it is a *type*
guarantee, not a prompt instruction: a routine without a `connect` step is a
routine the library doesn't contain. A prompt asking the coach to "connect to
her experience sometimes" is a wish; a step in the plan she can see on the rail
is a commitment.

| Routine | Connect step |
|---|---|
| `zh-scan-focus-lens` | 「你见过这件事吗」 · 这篇讲的事，你自己身边、新闻里、或者别的书里，有没有碰到过？想到什么说什么。 |
| `zh-narrative` | 「换成你呢」 · 如果是你在那个位置上，你会怎么做？跟他一样吗？ |
| `en-close-read` | 「你原来是怎么想的」 · 读之前你对这件事是什么印象？读完变了没有？ |
| `en-argument` | 「你站哪边」 · 读之前你自己是什么立场？作者动摇你了吗，还是让你更确定了？ |

It is placed **after** the reflect step and **before** the hunt: she works out
what the text says, then what it means to her, then goes back into the text one
last time.

The coach prompt gains a short section on how to lead a `connect` step — most
importantly that **there is no wrong answer here and nothing to check**, so it
must not evaluate what she says, only receive it and go on.

### A4 · The reading grows questions worth writing about

Bound by **R3** (backlog): a suggestion, not a seeded project — but the
suggestion is the product.

> *"just a suggestion, but suggestion is very important, some interesting
> questions would grow from this reading — why xxxx, what is xxx, how people
> view xxx, etc."*

**New endpoint, generate-if-absent:**

```
GET /api/v1/readings/{id}/questions   → { questions: [{ id, text, anchorQuote, anchorBlock }] }
```

Called by the finished screen. On a finished reading with no stored questions,
it takes a `pg_advisory_xact_lock` on the atom id, re-checks inside the same
transaction, makes **one** flagship call, validates, and stores. The lock is
before the provider call — the double-charge lesson from B's `/opening` race,
applied at the design stage this time rather than after paying for it twice.

**Three to five questions, each anchored.** The model returns
`{text, anchorQuote}` per question, and the server keeps a question **only if
`anchorQuote` is a literal substring of the article body**. This is what makes
genericity structurally hard: 「你怎么看待环保？」 cannot cite a sentence in this
article, so it cannot survive. The anchor is also shown — the question arrives
attached to the line that provoked it, which is what makes it feel grown rather
than generated.

If fewer than two questions survive validation, none are shown. A thin, generic
suggestion is worse than no suggestion.

**Where it goes.** A self-contained `<ReadingQuestions>` component on the
finished screen, under 我的收获. C+D will re-mount this same component inside the
report — which is why it is built standalone now rather than inlined.

**What 去写一写 does.** Navigates to `/writings` with the question text
pre-filled in the new-writing box. **Nothing else carries over** — not the
article, not her notes, not a plan. R3 chose "she starts fresh"; pre-filling the
topic makes the suggestion actionable without pre-seeding the work.

### A5 · Two small things the walk found here

- **可信度 shows 尚未评估, always.** The field is rendered by the shared
  `FinalizeReadingPanel` but lite has no producer — `credibility.verdict` is
  hard-coded `""` (`readingRoom.ts:239-241`). Gate it off with a new
  `credibility: false` on `LITE_READING_CAPABILITIES`, the mechanism the panel
  already uses for `proposalImpact`. **Do not delete the field** — pro produces
  it. (See the standing rule: a lite task can break pro through shared files.)
- **The 120-字 cap.** `readingCoachSystem` says 说话要短。不超过 120 个字 — the
  exact mechanical cause the backlog identified for the AI-voice copy that was
  rejected in the writing room. Raised to **200 字**, and the prompt gains the
  same teacher's-move guidance B0 got: state why it matters, name the real
  thing, offer a choice, offer to show. 铁律③ is untouched — one *question* per
  turn was never "say as little as possible".

---

## Data model — migration `0103_reading_depth.sql`

```sql
-- reading_task.kind gains 'hunt' and 'connect', and loses 'quiz'.
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt'));
UPDATE reading_task SET kind = 'hunt' WHERE kind = 'quiz';

CREATE TABLE reading_question (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id      uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  position     integer NOT NULL,
  text         text NOT NULL,
  anchor_quote text NOT NULL,
  anchor_block text NOT NULL DEFAULT '',
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX reading_question_pos_idx ON reading_question (atom_id, position);
```

The `UPDATE` matters: existing in-flight readings have `quiz` rows, and the new
CHECK would otherwise be unsatisfiable for them. Migrating them to `hunt` is
right — the hunt is what that step should have been.

## Guarantees, by type rather than by manners

The property this spec is built on, carried over from B:

| Requirement | How it is guaranteed |
|---|---|
| The coach's lens is aimed at a paragraph | `Lens` is dropped unless `FocusBlock` is valid |
| The example is really in the article | `agent.ResolveExampleAnchor`, unchanged |
| She analyses a *different* sentence than the demo | overlap rejection in the card, unchanged |
| A hunt is answered by pointing, not asserting | `picks[].quote` must be a literal substring of the block |
| Every route touches her own experience | `taskConnect` is in every routine; a routine without one does not exist |
| Follow-up questions are about *this* article | each carries an `anchorQuote` that must be a literal substring |
| No route ends in typed quiz questions | `taskQuiz` is deleted from the enum |
| The questions call is charged once | advisory lock taken **before** the provider call |

## Non-goals

- **No new writing surface in the reading room.** R3 ruled out 写一写-in-place.
- **No seeded writing project.** R3.
- **No new lens cards.** The deck is what it is; this spec changes who can reach
  for it.
- **No reading report.** That is C, next.
- **No model re-routing.** Deferred to its own benchmarked session; this spec
  keeps every call on the tier it is on today.

## Testing

- **Go, unit:** `parseReadingCoachReply` drops a `Lens` that is unknown / out of
  order / un-aimed / while a card is open; `picks` validation keeps a literal
  substring and drops a paraphrase; `buildReadingTasks` produces exactly one
  `connect` and one trailing `hunt` for every routine in the library; the
  question validator drops an un-anchored question and returns none when fewer
  than two survive.
- **Go, integration (testcontainers):** the coach turn mints a lens aimed at the
  named block; a second concurrent `GET /questions` makes exactly one provider
  call (assert on the fake provider's call count, not on timing).
- **lite-web, vitest:** the hunt hint appears only on a `hunt` step; picks are
  sent structurally *and* inlined in `text`; `<ReadingQuestions>` renders
  nothing below two questions; 去写一写 navigates with the topic pre-filled.
- **e2e:** extend `coach-walk.spec.ts` through a coach-summoned lens and a hunt
  answered by clicking a paragraph.
- **Regression:** pro's full suite must stay green — `FinalizeReadingPanel` and
  `ReadingRoom` are shared files.

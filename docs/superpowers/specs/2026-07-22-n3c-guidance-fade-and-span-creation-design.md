# N3c · Guidance fade + student span-creation — design

> Slice N3c of the student-platform finishing decomposition
> (`docs/2026-07-20-student-platform-remaining-work.md`). Follows N3a (the C1
> primitive library, `27492fd`) and N3b (the moment classifier + live refeed,
> `eb0c419`).

## 1. What this closes

Slice 6 shipped the CRAAP fill at **guidance level L1** and pinned the ladder it
sits on (`docs/superpowers/specs/2026-07-11-slice-6-craap-fill-mint-design.md`
§ *The fill model*, lines 39–41):

| level | who elicits the question | who locates the span | the student |
|---|---|---|---|
| **L1 · novice** | AI | AI | answers |
| **L2** | AI | **student** | locates + answers |
| **L3** | **student** (from angles) | **student** | elicits + locates + answers |

L2 and L3 were deferred as "future config, not a rewrite", and three seams were
deliberately left open for them: `author` on every span, an authorship-agnostic
`StudioAnnotateCard`, and a single isolated generation site
(`api/studioturn.go:298 surfaceAnchors`). N3c builds L2 and L3 on those seams.

There is **no binding `.dc.html`** for span selection — the design doc never drew
this interaction. Everything visual below is new design, justified against the
铁律 rather than copied from a pinned mockup.

## 2. The level is carried by the data — no new field anywhere

The load-bearing decision. The level is not a column, not an API field, not a
prop: it is **derivable from the anchors as surfaced**.

| anchor as surfaced | level | renderer behavior |
|---|---|---|
| `author:"ai"`, `question != ""`, span filled | **L1** | answer box only (today, byte-identical) |
| `author:"student"`, `question != ""`, **span blank** | **L2** | answer box + 「去文章里选出这句」 |
| `author:"student"`, **`question == ""`**, span blank | **L3** | question input + answer box + 「去文章里选出这句」 |

"Span blank" means `block_id == "" && end == start`.

Consequences, all of them good:

- **No migration**, no new `card_instance` column, no contracts change for the
  level, no new SSE frame field.
- `StudioAnnotateCard` stays authorship-agnostic in exactly the sense the Slice 6
  spec meant — it renders what it is handed; what it is handed now differs.
- The projection/reload path needs no change: persisted anchors already carry
  `author`, `question`, `block_id`, `start`, `end`, so a reload re-derives the
  same level with no extra read.

### 2.1 What `author` means on an anchor

Stated explicitly, because the field is now load-bearing in a second way:
`author` = **who did the thinking work the anchor represents** (eliciting the
question and locating the sentence). The *answer* is always the student's, at
every level. L1 → `"ai"`. L2/L3 → `"student"`.

Nothing downstream branches on this in a way that breaks: `GraphEffects`
(`card_effects.go:38`) mints with a hardcoded `Author: "student"` and never
filters anchors by author; `tagAnswered` (`card_completion.go`) only reads
`Dimension` + `Answer`; `fieldWrittenBy` is used only for `risk_note`, which the
client appends with `author:"student"` at every level already. The one visible
effect is desirable: `AUTHOR_MARK_STYLE` in `Annotate.tsx` already renders
student spans green and AI spans purple, so her own spans read as hers.

## 3. The fade has a real producer

A level with no producer is dead code — the failure mode this repo has shipped
three times (`SourceDossier.tsx` names two of them). So the level is **computed
live at surface time**, not authored in card JSON.

`surfaceAnchors` counts her **`status='completed'` instances of this card,
PROJECT SCOPE ONLY, across all her projects**:

```
uses := store.CountCompletedCardUsesByUser(ctx, userID, spec.ID)
level := L1 + min(uses, 2)      // 0 → L1, 1 → L2, ≥2 → L3
```

- **Project scope only — this is NOT the same count the 工具卡 growth tab
  shows her.** `ListCollectedCardsByUser` (`card_instance.sql:91`, which backs
  that tab) unions three scopes: project, course session, and chat thread. The
  fade's own query, `CountCompletedCardUsesByUser`, unions **project only**.
  This is a deliberate divergence, not an oversight — see the next two bullets
  for why.
- **The counter must genuinely reflect real work, and only the project scope
  can promise that.** `projectcards.go:301` sets `completed` only on a
  *satisfying* submit — it runs the card's own completion predicate
  (`CompleteCard`) first, so a project-scope "completed" row means she
  actually did whatever the card asks for (for `craap`/annotate: answered
  every dimension, and — at L2/L3 — went through the locate/elicit flow this
  same design adds). Chat and course cards make **no such promise**: `chat.go`
  and `course_session.go` both write `status='completed'` **unconditionally**
  on submit — no completion predicate, no `CompleteCard`, and (being thin by
  design) no anchors at all. A student can open the in-chat CRAAP offer twice
  and submit two anchor-less sheets — nothing located, nothing answered in
  the sense the card is for — and, if this counter unioned that scope in, her
  very first real CRAAP card in the Studio would surface at L3 (no AI
  question, no AI-circled span) having never once done the card properly.
  That would make the ladder's whole premise false. Restricting to project
  scope is what keeps "she has completed this card N times" meaning "she has
  done the card's actual work N times."
- **User-scoped across ALL her projects, deliberately.** A second project
  must not reset her to novice — the fade tracks the student, not the
  project, so `uses` still sums across every project she owns. Only the
  course/chat UNION branches are dropped, not the cross-project scope.
- **One new narrow sqlc query.** `ListCollectedCardsByUser` returns per-card
  usage across three surfaces and is wrong to reuse even setting scope aside
  (it aggregates everything, not one card_id); a dedicated
  `CountCompletedCardUsesByUser(user_id, card_id)` is one row, one
  index-friendly count, project-scope only.
- **Scope: `spec.Primitive == "annotate"` only.** Compare/SIFT stays L1 — its
  lateral read is already the student's own work, and its anchor generation is
  additionally constrained (lateral-dimension drop, `studioturn.go:315`).
  Widening the fade to compare is a separate design conversation.

### 3.1 The fade is silent (铁律 2)

No level badge, no 「你已解锁 L2」, no progress bar, no celebration. The card
simply asks differently. The only new copy is *instructional* — 「这次自己在文章
里找出那句话」 — never congratulatory, and never a comparison to other students.
A fade that announces itself is a streak mechanic wearing a pedagogy costume.

## 4. L2 needs its own prompt — it is not a stripped L1

An L1 question points **at** a sentence («这句说 NASA 数据显示…，谁在背后支撑这个
说法？»). Reused at L2 it would reference a sentence she has not found yet, and
would hand her the answer to the locating task in the question text. So
generation goes level-aware:

- **L1** — unchanged. `{block_id, quote, dimension, question}`, offsets computed
  server-side, `author:"ai"`. Same prompt, same parse, same fallback, same
  metering. **Zero behavior change** for a first-time student.
- **L2** — the model is asked for `{dimension, question}` only, with the
  question phrased span-independently. Parse skips block/offset resolution.
  `author:"student"`, span blank. Still one metered LLM call.
- **L3** — **no LLM call at all.** The anchors are built deterministically from
  `spec.Params.Tags`: dimension only, `question:""`, span blank,
  `author:"student"`. The scaffold fading also removes the spend.

**This is not an unmetering hole.** The known unmetering defect class
(`surfaceAnchors`/`renderChallenge` bailing before `RecordLLMCall`) is about a
call that *happened* and went unrecorded. At L3 no call happens — there is
nothing to meter. The existing metering block is untouched and still runs
whenever `result.Resolved.Provider != ""`.

Tag vocabulary validation (`parseAnchorGen`'s `every_tag_present` guard) applies
unchanged at L2 — an off-vocabulary dimension is still a parse failure falling
back to `fallbackAnchors`. `fallbackAnchors` gains the same level-awareness, so
a degraded L2/L3 surface degrades to the *right level*, not back to L1.

## 5. Locating is never a wall (铁律 2)

**No completion predicate changes.** `tagAnswered` still requires only a
non-empty `Answer`, so:

- a located span is **never** required server-side;
- a card can therefore never strand `active` because she could not find a
  sentence — the exact failure `StudioAnnotateCard`'s own comments (lines 26–45)
  were written to prevent;
- L2/L3 add no new way for a submit to be rejected.

The client asks for the span and offers an explicit escape per dimension:
「找不到合适的句子」. Taking it clears the ask for that dimension and lets her
lock. **Friction is converted into signal, not eliminated** (铁律 4) — the miss
is recorded (§6) and the anchor persists span-less, which is itself visible in
the process tree.

At L3 the question is likewise not gated: an unanswered *question* field simply
means the anchor's `question` stays empty, and completion still keys off
`Answer`. The client requires the answer (as today) and nothing more.

## 6. Recording it — two additive `TraceEvent` kinds

`TraceEvent` is a **closed discriminated union on both sides**: the Zod
`z.discriminatedUnion` (`packages/contracts/src/envelope.ts:4`) and Go's
`traceKinds` set (`internal/api/cards.go:37`), which rejects unknown kinds with a
400. They must move in lockstep or every L2/L3 submit fails validation.

Two kinds are added:

```ts
{ kind: "span_located",   dimension: string, block_id: string, at: string }
{ kind: "span_not_found", dimension: string, at: string }
```

This is a change to the standard envelope, which AGENTS.md says not to make
casually. It is justified and deliberately **additive**: existing traces still
validate against the widened union, no existing kind changes shape, and the
alternative (overloading `field_change.path` with `anchor.<id>.span_not_found`)
would encode a type inside a string and lose the located-vs-gave-up distinction
that is the whole point of recording it.

## 7. Two pre-existing bugs this slice must resolve

### 7.1 Span offsets are wrong on Chinese material (live bug)

`computeOffsets` (`anchors.go:93`) returns Go **byte** offsets
(`strings.Index` + `len(quote)`). `segmentBlock` (`primitives/annotate/
segment.ts:33`) consumes them as JS `String.prototype.slice` arguments, i.e.
**UTF-16 code-unit** indices. The material is Chinese
(`studio/fixtures.ts:29` — 「过去二十年里发生了一件几乎没人注意到的事…」), where
one character is 3 bytes and 1 code unit. Every AI anchor whose quote is found
therefore highlights **the wrong text**, roughly 3× further into the block, or
falls off the end.

N3c cannot add a second span producer on an undefined coordinate system, so:

**`Anchor.start`/`Anchor.end` are rune (Unicode code point) indices.** Go counts
with `utf8.RuneCountInString`; the web slices with code-point-aware slicing
(`Array.from(text).slice(...).join("")`), not `String.prototype.slice`. Both
sides get a CJK test. Documented on the Zod `Anchor` and the Go `Anchor` struct.

Runes over UTF-16 code units: rune indexing is what Go expresses naturally, and
`Array.from` gives JS the same unit for free — including for non-BMP characters
(emoji in a pasted source), where UTF-16 and runes diverge.

**Precisely which half fixes what** (established while implementing, and worth
stating so nobody over-credits the web change): the visible wrong-highlight bug
was **entirely Go-side** — it emitted bytes. Fixing that alone corrects every
realistic Chinese article, because BMP CJK is 1 rune = 1 UTF-16 code unit, so
`String.prototype.slice` would have agreed with rune offsets anyway. The web's
code-point slicing therefore fixes **only** the non-BMP case (an emoji in a
pasted source shifts every later offset and can split a surrogate pair). It is
worth doing — a second span producer lands in this same slice and the two sides
must not merely happen to agree — but it is hardening, not the bug fix.

Persisted anchors written before this change carry byte offsets and will be
reinterpreted as rune offsets. Those values are already wrong; the project has
one seeded mock student and no production data, so no backfill is written.

### 7.2 `SourceDossier` owns `openId` privately

Selecting a sentence requires the article to be open in the center pane while
the card is in the coach rail. `SourceDossier` holds `openId` in local state
with no way for a parent to open a specific source. The 「去文章里选出这句」
button needs to (a) switch the station to 素材 and (b) open the card's material.
`SourceDossier` gains an **optional controlled** `openSourceId` prop; when
absent, behavior is unchanged.

## 8. Cross-pane wiring

`StudioContainer` is already the cross-pane coordination point (it owns
`pendingAnchors` and the lifted `lateralMaterialId` for exactly this reason).
It gains one piece of transient state:

```ts
locating: { anchorId: string; dimension: string; materialId: string } | null
```

Flow: card 「去文章里选出这句」 → container sets `locating`, switches station to
素材, forces `openSourceId` to the card's material → `Annotate` renders in
select-mode (a hint bar naming the dimension, and a cancel) → she selects text →
`rangeToSpan` yields `{blockId, start, end, text}` → container writes
`{block_id, start, end, quote}` onto that anchor in the live conversation card
and clears `locating` → the rail shows the sentence under the dimension.

**Deferred, and corrected here rather than left as an unmet claim:** this
section originally promised the article would highlight her sentence green
(student authorship) *immediately*. It does not. The located span lives in
`StudioContainer` state and never reaches `ViewFrame`'s `anchors`, so the
article shows nothing until the card is submitted and the project refetched.
Her confirmation in the meantime is in the rail (the dimension shows the
sentence she picked), which is adequate but is not what this paragraph
described. Threading the in-progress span into the article pane is a small,
self-contained follow-up.

`rangeToSpan` (new `primitives/annotate/selection.ts`) reads
`window.getSelection()`, walks up to the enclosing `[data-block-id]`, and
accumulates rune offsets across the rendered run `<span>`/`<mark>` nodes. It
returns `null` — a no-op, never an error — for a collapsed selection, a
selection spanning two blocks, or a selection outside any block.

`Annotate` gains `data-block-id` on each block and two optional props
(`selectMode`, `onCreateSpan`). **With them absent its render and behavior are
byte-identical to today**, which is what keeps L1 and every other `Annotate`
consumer untouched.

## 9. Out of scope

- **Free annotation outside a card** — marking any sentence she likes, with her
  own tag, unprompted. A different feature with its own storage question (no
  card instance to hang it on); not the ladder.
- **Chat and Course surfaces** — their card runtime is thin and carries no
  anchors (Slice 11/12).
- **Compare/SIFT** — stays L1 (§3).
- **The assessor consuming the authorship signal** — that student-located spans
  should read as stronger D2/D3 evidence is real, and belongs with the rubric
  work in N4/N6, not here.
- **Any per-card cap on the ladder** (`params.max_guidance`). YAGNI until a card
  actually needs to stay L1; the primitive scope in §3 already bounds it.

## 10. Verification

- Go: full packages, `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./...`
- Web: `npm test` + `npx tsc --noEmit` from `apps/web`
- Contracts: `npm test` from `packages/contracts`
- One real-DB E2E driving the fade: the same student surfacing the same card
  three times gets L1, then L2, then L3 anchors — the test that proves the
  producer is live and not a fixture.

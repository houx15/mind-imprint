# Reading Room — Adopt the demo's disciplinary lenses (fix "透镜库 always 没找到好例子")

## The bug (root cause)

Clicking any deep-reading card in the 透镜库 nearly always returns
`这篇文章里我一时没找到适合这副透镜的好例子…`.

Flow: `LensLibrary.onPick(id)` → `loop.summonCard` → `POST …/materials/{mid}/summon-card`
→ `agent.ProposeCardExample` makes **one flagship LLM call that must return a single
verbatim sentence from the article "最能示范这副透镜"**, then checks the quote is an
*exact substring* of the block. Fail → hard fallback (`summoncard.go:193-197`).

The deep-reading deck (`reading_deck.go:23`) is our **writing tool cards** —
`argument-map` ("把一段论证拆成结构图"), `toulmin` ("把论点搭成结构"),
`steelman`/`concession` (write a paragraph), `cda` ("对比两份对立文本"),
`framing`/`perspective-matrix` (sort/matrix). Distilling *those* to one illustrative
verbatim sentence is a category mismatch → the model returns a paraphrase (fails the
exact-substring check) or nothing groundable → fallback every time. The router path
(read-together) hides this by silently degrading to plain chat; the student-initiated
summon has no graceful path, so the failure is glaringly visible.

## The fix (adopt the demo, decided with the user)

`docs/reference/mind-imprint-card-agent-demo` separates **透镜 (从什么角度看)** from
**方法 (具体做什么)**. Its 9 disciplinary lenses are *purpose-built* for the
pick-one-sentence mechanic (each carries `taskPrompt` / `selectionHint` / `exampleFocus`).

Decision (user): **make the 透镜库 the 9 disciplinary lenses; keep the tool cards for the
writing studio; layer tool cards as a lens's "methods" LATER.** Plus: a summon must never
hard-refuse.

### Lens catalog (ported from demo `cards.js`)

| id (ours) | 学科 | family | rubric_dims |
|---|---|---|---|
| `lens-logic`        | 逻辑学     | reasoning     | D5 |
| `lens-methods`      | 科学方法论 | reasoning     | D1,D2,D5 |
| `lens-society`      | 社会学     | institutions  | D4,D6 |
| `lens-law`          | 法学       | institutions  | D4,D6 |
| `lens-economics`    | 经济学     | institutions  | D5,D6 |
| `lens-ethics`       | 伦理学     | institutions  | D3,D4 |
| `lens-history`      | 历史学     | context       | D2,D6 |
| `lens-communication`| 传播学     | context       | D3,D6 |
| `lens-systems`      | 系统科学   | context       | D5,D6 |

Families → display groups: reasoning = 推理与证据 · institutions = 人与制度 · context = 语境与系统.
Source-check (craap, sift) stays its own group and still gates deep reading (SIFT needs CRAAP).

## Slices

### Slice 1 · contracts (`packages/contracts`)
- `cardSpec.ts`: add **optional** `reading_lens`:
  ```ts
  reading_lens: z.object({
    family: z.enum(["reasoning","institutions","context"]),
    task_prompt: z.string().min(1),     // "选一句…" what to pick
    selection_hint: z.string().min(1),  // "留意…" how to spot it
    example_focus: z.string().min(1),   // what the AI example should highlight
    method_ids: z.array(z.string()).optional(), // tool cards that operationalize it (future methods layer)
  }).optional()
  ```
- Author 9 lens JSONs in `packages/contracts/cards/lens-*.json`. Each is a valid `CardSpec`
  (one honest step + methodology + one field — INERT in the reading room, which renders only
  name/exampleWhy/eval, never steps), category `学科透镜`, `reading_lens` block, `related`/
  `method_ids` → existing tool cards, `rubric_dims` per table.
- Register all 9 in `registry.ts` (import + `DEFAULT_RAW`).

### Slice 2 · Go embed sync (`apps/api/internal/cards`)
- **Copy the 9 JSONs to `specs/`** (hand-synced duplicate — there is NO sync script; the two
  dirs are identical copies. Adding to only one = go:embed/registry drift.)
- `loader.go` `Spec`: add `ReadingLens *ReadingLens` + struct mirroring the Zod shape
  (snake_case json tags).

### Slice 3 · Go reading agent (`apps/api/internal/agent`)
- `reading_deck.go` `ReadingDeckIDs`: `craap, sift` + the 9 `lens-*` ids (drop the 13 tool
  cards). `ReadingDeck()` surfaces `reading_lens.task_prompt` as the router trigger when present.
- `reading_card_example.go` `buildCardExamplePrompt`: when `spec.ReadingLens != nil`, build a
  lens-tailored prompt from `task_prompt` + `selection_hint` + `example_focus` (fits the
  mechanic); fall back to purpose/trigger otherwise.
- `anchors.go` `computeOffsets`: **punctuation/whitespace-tolerant** match — try exact
  `strings.Index` first, then a normalized retry (fold full/half-width CJK punctuation + collapse
  spaces) mapping back to original rune offsets. Never returns a wrong span; still (0,0) on true miss.
- **Graceful degrade** (`summoncard.go`): when `ProposeCardExample` fails, DON'T bail — mint the
  card_instance with zero anchors and emit a `card` frame with an inviting `nudge_text`
  ("这副透镜——直接在文章里挑一句你觉得最能用它的话。") so the student goes straight to picking her own
  sentence. Keep the fallback message ONLY for true errors (materials fetch / persistence).

### Slice 4 · web (`apps/web/src/studio/reading`)
- `readingDeck.ts`: `READING_DECK_IDS` = `craap, sift` + 9 `lens-*`. Group deep-reading by
  `reading_lens.family` into the 3 families; keep source-check group.
- `LensLibrary.tsx`: render 信源体检 + 3 family sections (推理与证据 / 人与制度 / 语境与系统).
- `readingLoop.ts` / `ReadingRoom.tsx`: when a `card` frame arrives with **no anchors**, start at
  `active` (skip `proposed`) and hang the card under the first article block so it is always
  visible; student picks her own sentence directly.
- Reading-room-only: ensure the 9 lenses don't surface in the writing-studio decision layer
  (extend the existing `SuppressSurfaceCardIDs`/reading-only mechanism used for craap/sift).

### Slice 5 · tests + build
- contracts: registry loads 9 lenses; `reading_lens` parses; deck grouping.
- Go: deck resolves 9 lenses; `buildCardExamplePrompt` uses lens fields; `computeOffsets`
  punctuation tolerance; summon degrade emits a card (not the fallback) on example failure.
- web: `readingDeck` family grouping; LensLibrary renders groups; example-less loop start.
- `go test ./...`, contracts, web test + build all green.

## Deferred (not this slice)
- The **methods layer** (surfacing a lens's `method_ids` tool cards as "具体做什么" inside a lens) —
  data is wired (`method_ids`), UI/flow is a follow-up.
- Rabbit hole in the reading list — user is still deciding; will be a **function**, not a card.

# Lite 阅读房间 · 深度 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give 带读 the ability to summon a lens onto a paragraph, turn the closing step into a hunt performed in the article, put one step about her own experience into every routine, and grow real follow-up questions out of the article when she finishes.

**Architecture:** Almost everything is wiring validated machinery to a surface that replaced it. The coach's JSON reply gains a `lens` field with drop-don't-fail validation; `liteSummonCard`'s body is extracted into a helper the coach can call with a preferred block; the routine library gains two step kinds and loses one; the coach request gains structured `picks` so the server can tell a sentence she *pointed at* from one she *typed*; and one new generate-if-absent endpoint produces anchored follow-up questions.

**Tech Stack:** Go (`net/http`, `pgx`, sqlc **pinned `@v1.27.0`**, goose), PostgreSQL + testcontainers, React + Vite + TypeScript + Tailwind, vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-08-28-lite-reading-room-depth-design.md`

## Global Constraints

- **Never break pro.** `apps/web/src/studio/reading/ReadingRoom.tsx`, `HangingCard.tsx`, `FinalizeReadingPanel.tsx` and `apps/web/src/rooms/capabilities.ts` are shared with the pro edition. Changes there must be **additive and capability-gated**; never delete or repurpose a field pro produces. Pro's full suite must stay green.
- **印记 talks like a teacher.** State why it matters, name the real thing, offer a genuine choice, offer to show. Never clipped AI-shrug copy. 铁律③ (一次只问一个) means one *question*, never "say as little as possible".
- **Guarantee by output TYPE, not by prompt manners.** Anything the spec promises must be enforced by validation the server performs, not by an instruction the model is asked to follow.
- **AI never writes her prose** (铁律①). The coach explains, questions and demonstrates on the article's own sentences; it never authors her answers.
- **Never charge the provider twice for one thing.** Any generate-if-absent endpoint takes `pg_advisory_xact_lock` **before** the provider call and re-checks inside the same transaction.
- **Drop, don't fail.** An invalid model-supplied field is dropped and the turn still completes. She never sees an error about an instrument she didn't ask for.
- **Model routing is frozen.** Every call stays on the tier it uses today (`a.d.EvalResolver` for coach/plan). Do not introduce `resolveFast` anywhere.
- **Run only the targeted tests named in your task.** The controller runs the full suites at the end. Go tests need `-timeout 1800s` and `CGO_ENABLED=0`.
- Never `git add -A`; stage the specific files you changed.

---

### Task 1: Migration 0103 + `reading_question` queries

**Files:**
- Create: `apps/api/internal/store/migrations/0103_reading_depth.sql`
- Modify: `apps/api/internal/store/queries/reading.sql`
- Generated (do not hand-edit): `apps/api/internal/store/sqlc/*`

**Interfaces:**
- Consumes: nothing.
- Produces: `sqlc.ReadingQuestion` row type; `Queries.ListReadingQuestions(ctx, atomID) ([]ReadingQuestion, error)`; `Queries.InsertReadingQuestion(ctx, InsertReadingQuestionParams{AtomID uuid.UUID; Position int32; Text string; AnchorQuote string; AnchorBlock string}) (ReadingQuestion, error)`. `reading_task.kind` now accepts `'connect'` and `'hunt'` and rejects `'quiz'`.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0103_reading_depth.sql`:

```sql
-- +goose Up
-- 阅读的两处加深。
--
-- 一、收尾那一步从「回答几个问题」变成「回去找一句」。打字回答的问题，她可以
-- 凭印象答；回到文章里点出一句，她必须真的再读一遍。所以 quiz 不是被改名，是
-- 被替换——枚举里不留它，就没有哪套读法还能以打字问答收尾。
--
-- 二、每套读法里都有一步是她自己的（connect）。这是**类型**层面的保证，不是
-- prompt 里的一句叮嘱：库里不存在没有这一步的读法。
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
UPDATE reading_task SET kind = 'hunt' WHERE kind = 'quiz';
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','connect','hunt'));

-- 读完之后从这篇文章里长出来的问题。每一条都拴着原文里的一句话：
-- 拴不住的问题就是泛泛而谈，落库前会被丢掉。
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

-- +goose Down
DROP TABLE reading_question;
ALTER TABLE reading_task DROP CONSTRAINT reading_task_kind_check;
UPDATE reading_task SET kind = 'quiz' WHERE kind IN ('hunt','connect');
ALTER TABLE reading_task ADD CONSTRAINT reading_task_kind_check
  CHECK (kind IN ('read','focus_block','lens','reflect','quiz'));
```

Note the ordering inside Up: the `UPDATE` runs **after** the old constraint is dropped and **before** the new one is added, so neither constraint is ever violated mid-migration.

- [ ] **Step 2: Add the queries**

Append to `apps/api/internal/store/queries/reading.sql`:

```sql
-- name: ListReadingQuestions :many
SELECT * FROM reading_question WHERE atom_id = $1 ORDER BY position;

-- name: InsertReadingQuestion :one
INSERT INTO reading_question (atom_id, position, text, anchor_quote, anchor_block)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
```

- [ ] **Step 3: Regenerate sqlc**

Run from the repo root:

```bash
CGO_ENABLED=0 go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate -f apps/api/sqlc.yaml
```

The version pin is mandatory — an unpinned sqlc has produced incompatible output in this repo before. Expected: new `ReadingQuestion` struct and the two methods in `apps/api/internal/store/sqlc/`.

- [ ] **Step 4: Verify it compiles**

Run: `cd apps/api && CGO_ENABLED=0 go build ./...`
Expected: builds clean. (`reading_routines.go` still references `taskQuiz`; that is Task 2's job and does not break the build, since the Go const is independent of the DB constraint.)

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0103_reading_depth.sql apps/api/internal/store/queries/reading.sql apps/api/internal/store/sqlc
git commit -m "feat(lite): 找一找 and 联系你自己 become real step kinds"
```

---

### Task 2: The routine library gains `connect` and `hunt`, loses `quiz`

**Files:**
- Modify: `apps/api/internal/api/reading_routines.go`
- Modify: `apps/api/internal/api/reading_plan_test.go`
- Modify: `apps/lite-web/src/api/readingRoom.ts:325` (the doc comment listing kinds)

**Interfaces:**
- Consumes: Task 1's enum.
- Produces: `taskConnect readingTaskKind = "connect"`, `taskHunt readingTaskKind = "hunt"`; `taskQuiz` no longer exists. Every entry in `readingRoutines` contains exactly one `taskConnect` step and ends with exactly one `taskHunt` step.

- [ ] **Step 1: Write the failing test**

Add to `apps/api/internal/api/reading_plan_test.go`:

```go
// Every routine in the library carries one step that is hers and ends by
// sending her back into the article. This is the TYPE-level guarantee the
// spec rests on: a routine without a connect step is a routine that does not
// exist, which is a stronger promise than any line of prompt.
func TestEveryRoutineHasConnectAndEndsInHunt(t *testing.T) {
	for _, r := range readingRoutines {
		connects := 0
		hunts := 0
		for _, s := range r.Steps {
			switch s.Kind {
			case taskConnect:
				connects++
			case taskHunt:
				hunts++
			}
			if string(s.Kind) == "quiz" {
				t.Fatalf("routine %s still has a quiz step", r.Key)
			}
		}
		if connects != 1 {
			t.Errorf("routine %s: want exactly 1 connect step, got %d", r.Key, connects)
		}
		if hunts != 1 {
			t.Errorf("routine %s: want exactly 1 hunt step, got %d", r.Key, hunts)
		}
		if last := r.Steps[len(r.Steps)-1]; last.Kind != taskHunt {
			t.Errorf("routine %s: want the last step to be a hunt, got %q", r.Key, last.Kind)
		}
		if len(strings.TrimSpace(r.Steps[len(r.Steps)-1].Detail)) == 0 {
			t.Errorf("routine %s: the hunt step must say what to go find", r.Key)
		}
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestEveryRoutineHasConnectAndEndsInHunt -timeout 1800s`
Expected: FAIL — `undefined: taskConnect`.

- [ ] **Step 3: Rewrite the kinds and the library**

In `apps/api/internal/api/reading_routines.go`, replace the `taskQuiz` const with:

```go
	// 联系你自己：把这篇跟她自己的经历、见过的事、原本的想法接上。这一步没有
	// 对错，也不检查——它存在的意义是让这篇文章跟她本人有关系。
	taskConnect readingTaskKind = "connect"
	// 找一找：不是打字回答，是回到文章里把某样东西点出来。收尾用它，因为
	// 打字的答案可以凭印象给，点出来的句子不能。
	taskHunt readingTaskKind = "hunt"
```

Then rewrite the four routines' tails. `zh-scan-focus-lens`:

```go
			{Kind: taskReflect, Label: "这篇给了你什么", Detail: "用你自己的话说：读完之后，你知道了什么以前不知道的？"},
			{Kind: taskConnect, Label: "你见过这件事吗", Detail: "这篇讲的事，你自己身边、新闻里、或者别的书里，有没有碰到过？想到什么说什么，这一步没有标准答案。"},
			{Kind: taskHunt, Label: "回去找一句", Detail: "在文章里点出最能撑住作者观点的那一句。点出来，我们一起看看它撑不撑得住。"},
```

`zh-narrative`:

```go
			{Kind: taskReflect, Label: "作者想让你有什么感觉", Detail: "他是靠什么让你有这种感觉的？"},
			{Kind: taskConnect, Label: "换成你呢", Detail: "如果是你在那个位置上，你会怎么做？跟他一样吗？说说你的理由。"},
			{Kind: taskHunt, Label: "回去找一句", Detail: "在文章里点出你觉得写得最好的那一句——不是最重要的，是最好的。"},
```

`en-close-read`:

```go
			{Kind: taskReflect, Label: "用你自己的话复述", Detail: "不看原文，用中文把这篇讲一遍。"},
			{Kind: taskConnect, Label: "你原来是怎么想的", Detail: "读之前你对这件事是什么印象？读完之后变了没有？"},
			{Kind: taskHunt, Label: "回去找一句", Detail: "在文章里点出你觉得最难、但现在读懂了的那一句。"},
```

`en-argument`:

```go
			{Kind: taskReflect, Label: "你信吗", Detail: "哪一步你觉得站得住，哪一步你觉得他跳过去了？"},
			{Kind: taskConnect, Label: "你站哪边", Detail: "读之前你自己是什么立场？作者动摇你了吗，还是让你更确定了？"},
			{Kind: taskHunt, Label: "回去找一句", Detail: "在文章里点出作者最没说服你的那一句。"},
```

Add `"strings"` to the test file's imports if it is not already there.

- [ ] **Step 4: Fix the two existing tests that assert `quiz`**

In `reading_plan_test.go`, the fixture JSON at ~:87 and ~:154/:160 uses `"kind":"quiz"`, and `wantKinds` at ~:111 and ~:166 expects `"quiz"`. Update both fixtures to `"hunt"` and both `wantKinds` slices to the routine's real new kind sequence — read the routine the fixture selects and copy its kinds in order. Do not guess: the second test deliberately checks that a model inventing a seventh step is ignored, so its expected list must match the routine's actual length.

- [ ] **Step 5: Run the reading-plan tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestEveryRoutineHasConnectAndEndsInHunt|TestReadingPlan|TestPlanReading|TestBuildReadingTasks' -v -timeout 1800s`
Expected: PASS. **Verify from the `=== RUN` / `--- PASS` line count that your `-run` pattern actually matched the tests you edited** — a pattern that matches nothing also "passes".

- [ ] **Step 6: Update the TS doc comment**

In `apps/lite-web/src/api/readingRoom.ts:325`, change the comment listing kinds to `'read' | 'focus_block' | 'lens' | 'reflect' | 'connect' | 'hunt'`.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/reading_routines.go apps/api/internal/api/reading_plan_test.go apps/lite-web/src/api/readingRoom.ts
git commit -m "feat(lite): every 读法 has one step that is hers, and ends by sending her back into the text"
```

---

### Task 3: `parseReadingCoachReply` learns `lens`

**Files:**
- Modify: `apps/api/internal/api/reading_coach.go`
- Modify: `apps/api/internal/api/reading_coach_test.go` (it already exists — add to it)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `readingCoachReply.Lens string`; the parser signature becomes

```go
func parseReadingCoachReply(text string, valid map[string]bool, lang string, lensOK func(cardID string) bool) (readingCoachReply, bool)
```

`lensOK` is supplied by the caller and answers "may this card id be summoned right now" (deck membership + ordering guard + one-open mutex). Task 5 supplies the real one; tests supply fakes.

- [ ] **Step 1: Write the failing test**

Create/extend `apps/api/internal/api/reading_coach_test.go`:

```go
func TestParseReadingCoachReplyLens(t *testing.T) {
	valid := map[string]bool{"b1": true, "b2": true}
	allow := func(id string) bool { return id == "craap" }

	cases := []struct {
		name string
		json string
		want string
	}{
		{"aimed and allowed", `{"reply":"看这段","advance":"","focusBlock":"b2","lens":"craap"}`, "craap"},
		// An un-aimed lens is indistinguishable from her opening 透镜库
		// herself — the whole point of the coach summoning one is that it
		// lands on a paragraph it just talked about.
		{"un-aimed is dropped", `{"reply":"来看看来源","advance":"","focusBlock":"","lens":"craap"}`, ""},
		{"not allowed right now", `{"reply":"x","advance":"","focusBlock":"b1","lens":"sift"}`, ""},
		{"unknown id", `{"reply":"x","advance":"","focusBlock":"b1","lens":"nope"}`, ""},
		{"absent", `{"reply":"x","advance":"","focusBlock":"b1"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseReadingCoachReply(tc.json, valid, "zh", allow)
			if !ok {
				t.Fatalf("parse failed for %s", tc.json)
			}
			if got.Lens != tc.want {
				t.Errorf("lens = %q, want %q", got.Lens, tc.want)
			}
			if got.Reply == "" {
				t.Error("a dropped lens must not take the reply down with it")
			}
		})
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestParseReadingCoachReplyLens -timeout 1800s`
Expected: FAIL — too many arguments / unknown field `Lens`.

- [ ] **Step 3: Implement**

Add the field to the struct:

```go
	// Lens is the reading-deck card id the coach reaches for this turn, if
	// any. A paragraph tool explains a paragraph; a lens makes her perform an
	// analysis on a sentence she chooses herself. Empty on most turns.
	Lens string `json:"lens"`
```

Add the parameter and, at the end of `parseReadingCoachReply` (after the tool checks), the validation:

```go
	// A lens must be aimed. An un-aimed coach summon is exactly the 透镜库
	// she already has — what makes this the thing the product asked for is
	// that it lands on the paragraph the coach just talked about.
	if got.Lens != "" && (got.FocusBlock == "" || lensOK == nil || !lensOK(got.Lens)) {
		got.Lens = ""
	}
```

Update **every** existing call site to pass a temporary `func(string) bool { return false }` — `postReadingCoachTurn` (`reading_coach.go:368`) **and** the pre-existing calls in `reading_coach_test.go`. Task 5 replaces the handler's. Compile first (`go build ./...` then `go vet ./internal/api/`) to find them all rather than grepping by eye.

- [ ] **Step 4: Run the coach tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestParseReadingCoachReply' -v -timeout 1800s`
Expected: PASS, and the pre-existing `parseReadingCoachReply` tests still pass.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/reading_coach.go apps/api/internal/api/reading_coach_test.go
git commit -m "feat(lite): 带读 may name a lens, but only one it has aimed"
```

---

### Task 4: Extract `summonReadingLens`, with a preferred block

**Files:**
- Modify: `apps/api/internal/api/reading_lens.go`

**Interfaces:**
- Consumes: nothing.
- Produces:

```go
type summonedLens struct {
	Card    *cardDTO // nil when the summon was declined
	Nudge   string   // the "why this sentence" line, or the no-example hint
	Decline string   // non-empty when declined: the sentence to say instead
}

func (a *API) summonReadingLens(
	ctx context.Context,
	userID, atomID uuid.UUID,
	cardID, preferBlock, origin string,
) (summonedLens, error)
```

- [ ] **Step 1: Confirm the existing behaviour is covered before touching it**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'Summon|Lens' -v -timeout 1800s`
Record which tests exist and that they pass. This is a **pure refactor** — those same tests must still pass at Step 4 with no edits. If no such tests exist, say so in your report and proceed.

- [ ] **Step 2: Extract**

Move the body of `liteSummonCard` from the `catalog, err := agent.ReadingDeck()` line through the `CreateAtomCard` result handling into `summonReadingLens`, with these changes and no others:

- Take `ctx` as a parameter instead of deriving `detachedModelCtx(r)` inside; `liteSummonCard` derives it and passes it in.
- Take `origin` as a parameter instead of the hardcoded `cardOriginStudent`; `liteSummonCard` passes `cardOriginStudent`.
- The three decline paths (`liteSummonBusyReply`, `liteSummonSiftFirst`, `liteSummonCraapDone`, `liteSummonUnknownCard`) return `summonedLens{Decline: <that string>}, nil` instead of writing to a `ResponseWriter`.
- Real errors return `summonedLens{}, err`.
- Just before the `ProposeCardExample` call, narrow the blocks when a block is preferred:

```go
	blocks := materialBlocks(SplitBlocks(src.Body))
	// Aiming the grounding call: handing it ONE paragraph is what makes the
	// example land where the caller pointed. Block ids are position-derived
	// and preserved by the filter, so the returned anchor's BlockID is still
	// correct and ResolveExampleAnchor's guarantee is untouched. An unknown
	// preferBlock falls through to the whole article rather than to nothing.
	if preferBlock != "" {
		for _, b := range blocks {
			if b.ID == preferBlock {
				blocks = []agent.MaterialBlock{b}
				break
			}
		}
	}
```

`liteSummonCard` becomes: load atom → entitlement → decode → `detachedModelCtx` → call `summonReadingLens(ctx, u.ID, at.ID, cardID, "", cardOriginStudent)` → on `Decline` call `liteSummonDecline(w, res.Decline)` → on error `httpx.WriteError` → else write the same `liteTurnDTO` it writes today.

- [ ] **Step 3: Add the aiming test**

```go
func TestSummonReadingLensPrefersOneBlock(t *testing.T) {
	// A fake provider records the prompt it was given; grounding a card
	// example over a single-paragraph article can only cite that paragraph.
	// Asserting on the anchor's BlockID (not on prompt text) keeps this a
	// test of behaviour rather than of wording.
	t.Skip("integration: covered by Task 5's coach-summon test")
}
```

Leave this as an explicit skip with that reason rather than writing a weak unit test — the real assertion lives in Task 5 where a provider is already faked.

- [ ] **Step 4: Verify the refactor changed nothing**

Run the exact command from Step 1 again.
Expected: the same tests, still passing, unedited.
Also run: `cd apps/api && CGO_ENABLED=0 go build ./...`

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/reading_lens.go
git commit -m "refactor(lite): one way to mint a lens, aimable by its caller"
```

---

### Task 5: The coach mints the lens it named

**Files:**
- Modify: `apps/api/internal/api/reading_coach.go`
- Modify/Create: `apps/api/internal/api/reading_coach_summon_test.go`

**Interfaces:**
- Consumes: Task 3's `lensOK` parameter, Task 4's `summonReadingLens`.
- Produces: the coach turn response DTO gains `card` (`*cardDTO`, omitempty) and `nudge` (`string`, omitempty).

- [ ] **Step 1: Write the failing integration test**

In `apps/api/internal/api/reading_coach_summon_test.go`, using this package's existing testcontainers + fake-provider harness. **Read `apps/api/internal/api/reading_lens_test.go` and `reading_plan_test.go` first and copy their setup verbatim** — they already build a lite reading atom with an article, a fake provider, and a plan. Do not invent a new harness:

```go
// The coach names a lens and a paragraph; the room gets a card aimed there.
func TestCoachTurnMintsAimedLens(t *testing.T) {
	// Arrange: a reading with a 3-paragraph article, a plan whose current
	// step is a lens step, and a fake provider whose coach reply is:
	//   {"reply":"这条来源值得查一下","advance":"","focusBlock":"b2","lens":"craap"}
	// and whose card-example reply grounds a sentence.
	//
	// Assert:
	//   1. the response carries a card with status "proposed"
	//   2. the card's block_id is "b2"
	//   3. origin is "router" — she did not choose this lens
	//   4. the reply text is unchanged by the summon
}
```

Fill in the arrange/assert with the harness's real helpers. Every numbered assertion must be a real `if`/`t.Errorf`, not a comment.

- [ ] **Step 2: Run it to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestCoachTurnMintsAimedLens -v -timeout 1800s`
Expected: FAIL — the response has no card.

- [ ] **Step 3: Implement**

In `postReadingCoachTurn`, build the real `lensOK` before parsing:

```go
	cardRows, err := a.d.Queries.ListAtomCards(ctx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	deck, deckErr := agent.ReadingDeck()
	ordering := readingOrderingGuard(cardRows)
	anyOpen := false
	for _, c := range cardRows {
		if c.Status == "proposed" || c.Status == "active" {
			anyOpen = true
			break
		}
	}
	// The coach may only reach for a lens the room could actually open right
	// now. Checking here rather than after the call means a refused summon
	// never reaches her as a card that silently failed to appear.
	lensOK := func(id string) bool {
		if deckErr != nil || anyOpen || !inReadingDeck(deck, id) {
			return false
		}
		if id == "sift" && !ordering.AllowSift {
			return false
		}
		if id == "craap" && !ordering.AllowCraap {
			return false
		}
		return true
	}
```

Pass it to `parseReadingCoachReply`. After the reply is persisted and the task status applied, and **only if `got.Lens != ""`**:

```go
	var cardOut *cardDTO
	nudge := ""
	if got.Lens != "" {
		// 铁律④ — origin is 'router': SHE did not pick this lens, and the
		// autonomy signal on the row must say so. Failure is silent: the
		// coach's words still stand, she simply doesn't get the instrument.
		res, serr := a.summonReadingLens(ctx, u.ID, at.ID, got.Lens, got.FocusBlock, cardOriginRouter)
		if serr != nil {
			slog.Info("lite coach: lens summon failed; turn stands without it",
				"atom_id", at.ID, "card_id", got.Lens,
				"request_id", httpx.RequestIDFromContext(r.Context()), "err", serr)
		} else if res.Card != nil {
			cardOut, nudge = res.Card, res.Nudge
		}
	}
```

Add `Card *cardDTO \`json:"card,omitempty"\`` and `Nudge string \`json:"nudge,omitempty"\`` to the coach turn response struct and populate them.

Use the same context the rest of the turn's model work uses (the detached model context), not `r.Context()` — the summon makes a provider call.

- [ ] **Step 4: Run it**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestCoachTurn|TestParseReadingCoachReply' -v -timeout 1800s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/reading_coach.go apps/api/internal/api/reading_coach_summon_test.go
git commit -m "feat(lite): 带读 hands her the lens, aimed at the paragraph it just named"
```

---

### Task 6: `picks` — the server can tell pointing from typing

**Files:**
- Modify: `apps/api/internal/api/reading_coach.go`
- Modify: `apps/api/internal/api/reading_coach_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: request body `{"text": string, "picks": [{"blockId": string, "quote": string}]}`; helper

```go
// validateReadingPicks keeps only picks that point at a real paragraph AND
// quote it literally. Same shape, and the same reason, as the writing room's
// comment-quote validator: a guarantee you can check beats one you asked for.
func validateReadingPicks(picks []readingPick, blocks []Block) []readingPick
```

- [ ] **Step 1: Write the failing test**

```go
func TestValidateReadingPicks(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "中国的碳排放总量位居世界第一。"},
		{ID: "b2", Text: "但人均排放仍低于多数发达国家。"},
	}
	got := validateReadingPicks([]readingPick{
		{BlockID: "b1", Quote: "碳排放总量位居世界第一"}, // literal substring — kept
		{BlockID: "b2", Quote: "人均排放低于发达国家"},   // paraphrase — dropped
		{BlockID: "b9", Quote: "中国的碳排放总量"},       // no such block — dropped
		{BlockID: "b1", Quote: "  "},                  // empty — dropped
		{BlockID: "b1", Quote: "但人均排放仍低于多数发达国家。"}, // right words, wrong block — dropped
	}, blocks)
	if len(got) != 1 {
		t.Fatalf("kept %d picks, want 1: %+v", len(got), got)
	}
	if got[0].BlockID != "b1" || got[0].Quote != "碳排放总量位居世界第一" {
		t.Errorf("kept the wrong pick: %+v", got[0])
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestValidateReadingPicks -timeout 1800s`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement**

```go
type readingPick struct {
	BlockID string `json:"blockId"`
	Quote   string `json:"quote"`
}

func validateReadingPicks(picks []readingPick, blocks []Block) []readingPick {
	byID := make(map[string]string, len(blocks))
	for _, b := range blocks {
		byID[b.ID] = b.Text
	}
	out := make([]readingPick, 0, len(picks))
	for _, p := range picks {
		q := strings.TrimSpace(p.Quote)
		if q == "" {
			continue
		}
		body, ok := byID[p.BlockID]
		if !ok || !strings.Contains(body, q) {
			continue
		}
		out = append(out, readingPick{BlockID: p.BlockID, Quote: q})
	}
	return out
}
```

Add `Picks []readingPick \`json:"picks"\`` to the coach request struct, validate them against `SplitBlocks(src.Body)`, and render the survivors into the prompt in `buildReadingCoachPrompt` as their own section, immediately before 她刚说的话:

```
【她在文章里点出来的句子】
第2段：「中国的碳排放总量位居世界第一」
```

Use the same 第几段 numbering the prompt already uses for paragraphs — never `b2` (the existing prompt is explicit that block ids must never be spoken to her, and the same reasoning applies to what the model is shown as speakable).

If no picks survive, omit the section entirely rather than printing an empty heading.

- [ ] **Step 4: Run it**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestValidateReadingPicks|TestReadingCoachPrompt|TestParseReadingCoachReply' -v -timeout 1800s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/reading_coach.go apps/api/internal/api/reading_coach_test.go
git commit -m "feat(lite): a sentence she pointed at is not the same as one she typed"
```

---

### Task 7: The coach prompt learns to teach, to hunt, and to hand over a lens

**Files:**
- Modify: `apps/api/internal/api/reading_coach.go` (`readingCoachSystem`)
- Modify: `apps/api/internal/api/reading_coach_test.go`

**Interfaces:**
- Consumes: Tasks 3, 5, 6.
- Produces: no new exported surface.

- [ ] **Step 1: Write the failing test**

```go
// The prompt is a product surface: these clauses are what stops the coach
// reading as a clipped AI, and what makes the hunt a hunt. Pinning them keeps
// a later edit from quietly deleting a ruling.
func TestReadingCoachSystemCarriesTheRulings(t *testing.T) {
	for _, want := range []string{
		"200 个字",     // the raised cap (铁律③ is one QUESTION, not one sentence)
		"找一找",        // the hunt step's own section
		"联系你自己",     // the connect step's own section
		"lens",        // the lens field is documented in the output contract
	} {
		if !strings.Contains(readingCoachSystem, want) {
			t.Errorf("readingCoachSystem no longer mentions %q", want)
		}
	}
	if strings.Contains(readingCoachSystem, "120 个字") {
		t.Error("the 120-字 cap is the mechanical cause of the AI voice; it must be gone")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestReadingCoachSystemCarriesTheRulings -timeout 1800s`
Expected: FAIL.

- [ ] **Step 3: Edit the prompt**

Four edits to `readingCoachSystem`:

1. **The cap.** Replace `说话要短。不超过 120 个字。` with:

```
- 说话要短，但**短不等于什么都不说**。不超过 200 个字。
  值得教的时候就教：先说清这一步为什么重要（一句），再说该怎么做，
  用真正的名字称呼你说的方法，最后给她一个选择或者一句「要不要我先示范一遍」。
  「一次只问一个」说的是**问题**只问一个，不是话只说一句。
```

2. **The output contract** gains the field:

```
{"reply":"你要对她说的话","advance":"","focusBlock":"","tool":"","lens":""}
```

with, after the `tool` line:

```
- lens：透镜卡的 id。你要她**亲手做一遍某种分析**的时候用它，见下。不用就留空。
```

3. **A new section, after 段落工具:**

```
## 透镜：让她自己做一遍

段落工具是你讲给她听；透镜是她自己动手。用法只有一种，但这一种很重要：

先在 reply 里挑出这一段里的**某一句**，当着她的面把这种分析做一遍——
这一句为什么可疑 / 为什么有力 / 它在干什么——然后在 lens 里写下那张卡的 id。
她的屏幕上会出现这副透镜，先给她看你刚才的示范，再请她**在文章别的地方
自己找一句**做同样的事。

可用的透镜：

%s

规矩：
- **给 lens 就必须同时给 focusBlock**，而且是你 reply 里刚讲的那一段。
  没有落点的透镜等于没有——她自己去透镜库点也是一样的东西。
- 一轮最多一副。屏幕上已经开着一副的时候，不要再给。
- 读法清单走到 lens 那一步的时候，这是首选动作；别的时候，只有在她卡住、
  或者某一段特别值得她自己做一遍时才用。
```

The `%s` is a rendered list of the reading deck (id + name + when to reach for it), formatted like the paragraph-tool list already is. Render it from `agent.ReadingDeck()` — **do not hand-write a second copy of the deck** (the spec-registry single-source rule).

4. **A new section on the two new step kinds:**

```
## 两种特别的步骤

**联系你自己（connect）** —— 这一步没有标准答案，也没有什么要检查的。
她说什么都算。你的活儿是接住她说的，问一句让她多说一点，然后往下走。
**不要评价她的经历，不要把她的话拉回文章的「正确理解」上。** 这一步存在的理由
就是让这篇文章跟她本人有关系；你一纠正，它就变回了阅读理解。

**找一找（hunt）** —— 这一步她必须**真的在文章里点出一句**。
她点出来的句子会单独给你（【她在文章里点出来的句子】）。
- 她点了 → 接住那一句，说说它好在哪儿 / 站不站得住，advance 给 "done"。
- 她只是说「我觉得是第三段那句」，却没有点 → 那是说的，不是点的。
  advance 留空，告诉她在文章里把那句划出来或者点一下，它会自己出现在对话里。
- 她点的句子跟你想的不一样 → **那不是错**。先认真看她点的这一句，
  很多时候她的理由比你预设的更有意思。
```

- [ ] **Step 4: Run it**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestReadingCoach' -v -timeout 1800s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/reading_coach.go apps/api/internal/api/reading_coach_test.go
git commit -m "feat(lite): the coach may teach at length, hunt properly, and hand over a lens"
```

---

### Task 8: `GET /readings/{id}/questions` — questions grown from the article

**Files:**
- Create: `apps/api/internal/api/reading_questions.go`
- Create: `apps/api/internal/api/reading_questions_test.go`
- Modify: `apps/api/internal/api/api.go` (one route line)

**Interfaces:**
- Consumes: Task 1's queries.
- Produces: `GET /api/v1/readings/{id}/questions` → `{"questions":[{"id","text","anchorQuote","anchorBlock"}]}`; helper `validateReadingQuestions(qs []readingQuestionDraft, body string) []readingQuestionDraft`.

- [ ] **Step 1: Write the failing validator test**

```go
func TestValidateReadingQuestions(t *testing.T) {
	body := "中国的碳排放总量位居世界第一。\n\n但人均排放仍低于多数发达国家。"
	got := validateReadingQuestions([]readingQuestionDraft{
		{Text: "人均排放和总量，哪个更该被用来衡量责任？", AnchorQuote: "人均排放仍低于多数发达国家"},
		{Text: "你怎么看待环保？", AnchorQuote: "环境保护很重要"}, // not in the article — dropped
		{Text: "总量第一意味着什么？", AnchorQuote: "碳排放总量位居世界第一"},
		{Text: "  ", AnchorQuote: "中国的碳排放总量"},           // no question — dropped
	}, body)
	if len(got) != 2 {
		t.Fatalf("kept %d, want 2: %+v", len(got), got)
	}
}

// Fewer than two survivors means the model produced generalities. A thin,
// generic suggestion is worse than no suggestion — so show nothing.
func TestValidateReadingQuestionsNeedsTwo(t *testing.T) {
	body := "中国的碳排放总量位居世界第一。"
	got := validateReadingQuestions([]readingQuestionDraft{
		{Text: "总量第一意味着什么？", AnchorQuote: "碳排放总量位居世界第一"},
		{Text: "你觉得环保重要吗？", AnchorQuote: "环保重要"},
	}, body)
	if len(got) != 0 {
		t.Fatalf("want none when fewer than two survive, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestValidateReadingQuestions -timeout 1800s`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement the validator and the handler**

```go
type readingQuestionDraft struct {
	Text        string `json:"text"`
	AnchorQuote string `json:"anchorQuote"`
}

// validateReadingQuestions keeps a question only if the sentence it claims to
// have grown from is literally in the article. This is what makes a generic
// question structurally impossible rather than merely discouraged: 「你怎么看
// 待环保？」 cannot cite a line in this piece, so it cannot survive. Under two
// survivors, none are shown at all.
func validateReadingQuestions(qs []readingQuestionDraft, body string) []readingQuestionDraft {
	out := make([]readingQuestionDraft, 0, len(qs))
	for _, q := range qs {
		text := strings.TrimSpace(q.Text)
		quote := strings.TrimSpace(q.AnchorQuote)
		if text == "" || quote == "" || !strings.Contains(body, quote) {
			continue
		}
		out = append(out, readingQuestionDraft{Text: text, AnchorQuote: quote})
		if len(out) == 5 {
			break
		}
	}
	if len(out) < 2 {
		return nil
	}
	return out
}
```

Handler shape:

1. `loadOwnedReadingAtom`; entitlement check.
2. `ListReadingQuestions` — if non-empty, return them. (The cheap read path: no transaction, no lock.)
3. Otherwise open a transaction, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))` keyed on the atom id, then **re-run `ListReadingQuestions` inside that transaction** — a racing request that already generated them wins and this one returns its rows without calling the provider. This ordering is the whole point: the lock is taken **before** the provider call, not after.
4. Load the source; make one `a.d.EvalResolver` call via `gateway.Collect`; `recordLiteLLMCall(ctx, u.ID, at.ID, "read_questions", resolved, usage)`.
5. Validate; insert survivors by position; commit; return.

Use the repo's existing transaction helper (read `apps/api/internal/api/writing_setup.go:~250-300` — its `pg_advisory_xact_lock` + re-check-inside-the-tx + provider-call ordering is the reference implementation for this pattern in this codebase; copy that shape exactly, including the order of the three).

Prompt: 3–5 questions, each `{text, anchorQuote}`, each question something she could go and *write* about, `anchorQuote` copied **verbatim** from the article. Follow the teacher's-voice rule — the questions are hers to consider, not a quiz. Explicitly instruct: 不要问「你怎么看待X」这种放在任何一篇文章后面都成立的问题；每一条都必须是**这一篇**才问得出来的.

Route: `mux.Handle("GET /api/v1/readings/{id}/questions", liteOnly(a.getReadingQuestions))` next to the other reading routes.

- [ ] **Step 4: Add the single-charge integration test**

```go
// Two concurrent first-opens must cost exactly one provider call. The lock is
// taken before the call, so the loser re-reads rows rather than paying again.
func TestReadingQuestionsChargesOnce(t *testing.T) {
	// Fire two GETs concurrently against a finished reading with no stored
	// questions; assert the fake provider's call count == 1 and both
	// responses carry the same question ids.
}
```

Assert on the fake provider's **call count**, never on timing. **Read `apps/api/internal/api/writing_race_test.go` first** — it is this repo's reference implementation of exactly this test (two concurrent first-opens, one charge) and its structure should be reused rather than reinvented.

- [ ] **Step 5: Run**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestValidateReadingQuestions|TestReadingQuestions' -v -timeout 1800s`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/reading_questions.go apps/api/internal/api/reading_questions_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): the article grows questions worth writing about"
```

---

### Task 9: The coach panel points, and knows a lens arrived

**Files:**
- Modify: `apps/lite-web/src/api/readingRoom.ts`
- Modify: `apps/lite-web/src/readings/ReadingCoachPanel.tsx`
- Modify: `apps/lite-web/src/readings/ReadingRoomHost.tsx`
- Modify: `apps/web/src/studio/reading/ReadingRoom.tsx` (additive only)
- Create: `apps/lite-web/test/coachPicks.test.tsx`

**Interfaces:**
- Consumes: Tasks 5, 6.
- Produces: `postReadingCoachTurn(id, text, picks?)` returning `{..., card, nudge}`; `ReadingCoachSlot.quotes[].blockId?: string`; `ReadingCoachSlot.onCardSummoned?: () => void`.

- [ ] **Step 1: Write the failing test**

`apps/lite-web/test/coachPicks.test.tsx`:

```tsx
it("sends the quoted sentences as structured picks, and still inlines them in the text", async () => {
  // Render ReadingCoachPanel with a started transcript and a slot carrying
  // one quote { key: "q1", quote: "碳排放总量位居世界第一", blockId: "b2" }.
  // Type "我觉得是这句" and send.
  // Assert the fetch body parses to:
  //   text containing both "> 碳排放总量位居世界第一" and "我觉得是这句"
  //   picks === [{ blockId: "b2", quote: "碳排放总量位居世界第一" }]
});

it("shows the hunt hint only while the current step is a hunt", async () => {
  // tasks: [{...kind:"hunt", status:"pending"}] → hint present
  // tasks: [{...kind:"reflect", status:"pending"}] → hint absent
});
```

The hint's accessible text: `在文章里点出那一句，点了就会出现在这里`.

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/lite-web && pnpm vitest run test/coachPicks.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

1. `readingRoom.ts`: `postReadingCoachTurn(id, text, picks: {blockId: string; quote: string}[] = [])`, sending `JSON.stringify({ text, picks })` and returning `card: raw.card ?? null, nudge: raw.nudge ?? ""` alongside the existing fields. Type `card` as the existing lite card DTO type already declared in this file.
2. `ReadingRoom.tsx`: add `blockId?: string` to `ReadingCoachSlot.quotes[]` and `onCardSummoned?: () => void` to `ReadingCoachSlot`. Populate `blockId` where the quote chips are built (the drag-select handler already knows which block the selection is in). Wire `onCardSummoned` to whatever the room already calls to reload its cards after `loop.summonCard` — reuse that function, do not write a second refresh path. **Both additions are optional fields; pro passes no `renderCoach` and is unaffected.**
3. `ReadingCoachPanel.tsx`: `send()` builds `picks` from `slot.quotes` (dropping ones with no `blockId`) and passes them; keeps the existing blockquote inlining unchanged. After a successful turn, `if (res.card) slot.onCardSummoned?.()`.
4. The hunt hint: derive `const hunting = tasks.find(t => t.status === "pending")?.kind === "hunt"` and render a single line above the quote chips when `hunting && !slot.locked`.

- [ ] **Step 4: Run**

Run: `cd apps/lite-web && pnpm vitest run test/coachPicks.test.tsx test/readingRoomHost.test.tsx test/blockToolsAutoRun.test.tsx`
Expected: PASS.

- [ ] **Step 5: Typecheck both apps** (a shared file changed)

Run: `cd apps/lite-web && pnpm typecheck` then `cd ../web && pnpm typecheck`
Expected: both clean.

- [ ] **Step 6: Commit**

```bash
git add apps/lite-web/src/api/readingRoom.ts apps/lite-web/src/readings/ReadingCoachPanel.tsx apps/lite-web/src/readings/ReadingRoomHost.tsx apps/web/src/studio/reading/ReadingRoom.tsx apps/lite-web/test/coachPicks.test.tsx
git commit -m "feat(lite): pointing at a sentence reaches 印记 as a pick, not as prose"
```

---

### Task 10: `<ReadingQuestions>` on the finished screen

**Files:**
- Create: `apps/lite-web/src/readings/ReadingQuestions.tsx`
- Modify: `apps/lite-web/src/api/readingRoom.ts`
- Modify: `apps/lite-web/src/readings/ReadingRoomHost.tsx`
- Modify: `apps/lite-web/src/writings/WritingsLanding.tsx`
- Create: `apps/lite-web/test/readingQuestions.test.tsx`

**Interfaces:**
- Consumes: Task 8's endpoint.
- Produces: `<ReadingQuestions readingId={string} />`; `getReadingQuestions(id)`; the sessionStorage handoff key `lite:writing-idea`.

- [ ] **Step 1: Write the failing test**

```tsx
it("renders nothing at all when the server returns no questions", async () => { /* … */ });

it("shows each question with the sentence it grew from", async () => {
  // two questions returned → both texts visible, both anchorQuotes visible
});

it("去写一写 stashes the question and navigates to /writings", async () => {
  // click the first question's 去写一写
  // assert sessionStorage.getItem("lite:writing-idea") === that question's text
  // assert window.location.pathname === "/writings"
});

it("the writings landing picks the stashed idea up exactly once", async () => {
  // seed sessionStorage, render WritingsLanding
  // assert the textarea's value is the stashed text
  // assert sessionStorage.getItem("lite:writing-idea") === null afterwards
});
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/lite-web && pnpm vitest run test/readingQuestions.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

`readingRoom.ts`:

```ts
export type ReadingQuestion = { id: string; text: string; anchorQuote: string; anchorBlock: string };

export async function getReadingQuestions(id: string): Promise<ReadingQuestion[]> {
  const raw = await apiFetch<{ questions: ReadingQuestion[] }>(
    `/api/v1/readings/${encodeURIComponent(id)}/questions`,
  );
  return raw.questions ?? [];
}
```

`ReadingQuestions.tsx`: fetch on mount using the `useAlive` hook (`apps/lite-web/src/shared/useAlive.ts`) — **not** a `useRef` latch with a per-invocation `cancelled` flag, which is the StrictMode trap this codebase already paid for once. Render `null` while loading and `null` on empty. Otherwise a heading — 「读完这篇，还能往下想」 — then one card per question: the question in body-large, and beneath it the anchor quote in muted small type prefixed 「从这句想到的：」, and a 去写一写 button.

The 去写一写 handler:

```tsx
function writeAbout(text: string) {
  try {
    sessionStorage.setItem(WRITING_IDEA_KEY, text);
  } catch {
    // Private mode / storage disabled: the navigation still helps her, she
    // just retypes the question. Never let a storage failure eat the click.
  }
  navigate("/writings");
}
```

Export `export const WRITING_IDEA_KEY = "lite:writing-idea";` from `ReadingQuestions.tsx` and import it in `WritingsLanding.tsx`, which on mount does a read-and-clear into its existing `idea` state:

```tsx
useEffect(() => {
  try {
    const stashed = sessionStorage.getItem(WRITING_IDEA_KEY);
    if (stashed) {
      sessionStorage.removeItem(WRITING_IDEA_KEY);
      setIdea(stashed);
    }
  } catch {
    /* storage unavailable — nothing to pick up */
  }
}, []);
```

Read-and-clear, not read: the suggestion is for this arrival, not for every future visit to 写作.

Mount `<ReadingQuestions readingId={readingId} />` inside `FinishedReadingPanel` (`ReadingRoomHost.tsx:~409`), directly **below** the 我的收获 block, and delete the placeholder line 「这次阅读的报告还在路上。…」 only if questions render — no: **leave the placeholder line in place**. C+D replaces it with the real report; removing it now would leave the screen claiming nothing is coming.

- [ ] **Step 4: Run**

Run: `cd apps/lite-web && pnpm vitest run test/readingQuestions.test.tsx test/readingRoomHost.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/readings/ReadingQuestions.tsx apps/lite-web/src/api/readingRoom.ts apps/lite-web/src/readings/ReadingRoomHost.tsx apps/lite-web/src/writings/WritingsLanding.tsx apps/lite-web/test/readingQuestions.test.tsx
git commit -m "feat(lite): finishing a reading leaves her with questions, not a full stop"
```

---

### Task 11: 可信度 stops claiming 尚未评估

**Files:**
- Modify: `apps/web/src/rooms/capabilities.ts`
- Modify: `apps/web/src/studio/reading/FinalizeReadingPanel.tsx`
- Modify: `apps/lite-web/test/readingRoomCapabilities.test.tsx`

**Interfaces:**
- Consumes: nothing.
- Produces: `RoomCapabilities.credibility: boolean`; `true` in `PRO_CAPABILITIES`, `false` in `LITE_READING_CAPABILITIES`.

- [ ] **Step 1: Write the failing test**

Add to `apps/lite-web/test/readingRoomCapabilities.test.tsx`:

```tsx
it("does not show 可信度 in lite — there is no producer for it, so 尚未评估 is a lie", () => {
  // render FinalizeReadingPanel with LITE_READING_CAPABILITIES
  // expect(screen.queryByText(/可信度/)).toBeNull();
});

it("still shows 可信度 under pro capabilities", () => {
  // render with PRO_CAPABILITIES → the label is present
});
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/lite-web && pnpm vitest run test/readingRoomCapabilities.test.tsx`
Expected: FAIL on the first case.

- [ ] **Step 3: Implement**

Add `credibility: boolean` to the `RoomCapabilities` type; `true` in `PRO_CAPABILITIES` (and therefore `DEMO_CAPABILITIES`), `false` in `LITE_READING_CAPABILITIES`, with a comment saying why: lite has no producer, so the field could only ever read 尚未评估. In `FinalizeReadingPanel.tsx:~84-95`, wrap the 可信度 row in `caps.credibility && (…)`, mirroring exactly how `proposalImpact` is already gated in the same file.

- [ ] **Step 4: Run both suites**

Run: `cd apps/lite-web && pnpm vitest run test/readingRoomCapabilities.test.tsx` and `cd ../web && pnpm vitest run` (the panel is pro's file — its own tests must stay green).
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/rooms/capabilities.ts apps/web/src/studio/reading/FinalizeReadingPanel.tsx apps/lite-web/test/readingRoomCapabilities.test.tsx
git commit -m "fix(lite): don't print 尚未评估 for a verdict lite never evaluates"
```

---

### Task 12: e2e — a coach-summoned lens and a hunt answered by pointing

**Files:**
- Modify: `apps/lite-web/e2e/coach-walk.spec.ts`

**Interfaces:**
- Consumes: every prior task.

- [ ] **Step 1: Extend the walk**

Add to `coach-walk.spec.ts`, following the file's existing route-mocking style (do not switch to a live backend):

```ts
test("带读 hands her a lens aimed at the paragraph it just named", async ({ page }) => {
  // Mock POST /coach to return { reply, focusBlock: "b2", card: {…status:"proposed", block_id:"b2"}, nudge }
  // Assert the hanging card appears and its 示范 sentence is inside paragraph 2.
});

test("a hunt step is answered by clicking a paragraph", async ({ page }) => {
  // Plan whose pending step kind is "hunt".
  // Assert the hint 在文章里点出那一句 is visible.
  // Select a sentence in the article so a quote chip appears, then send.
  // Assert the POST body carried picks[0].blockId and picks[0].quote.
});
```

- [ ] **Step 2: Run the reading e2e specs**

Run: `cd apps/lite-web && pnpm exec playwright test -c e2e/playwright.config.ts coach-walk.spec.ts reading-walk.spec.ts`
Expected: PASS. If a pre-existing spec fails for a reason unrelated to this plan, report it rather than editing it into passing.

- [ ] **Step 3: Commit**

```bash
git add apps/lite-web/e2e/coach-walk.spec.ts
git commit -m "test(lite): walk the aimed lens and the hunt"
```

---

## Final verification (controller, after Task 12)

- `cd apps/api && CGO_ENABLED=0 go test ./... -timeout 1800s` — all packages.
- `cd apps/lite-web && pnpm test && pnpm typecheck && pnpm build`
- `cd apps/web && pnpm test && pnpm typecheck && pnpm build` — **pro must be green**; four shared files were touched.
- Update `docs/2026-08-28-lite-edition-feedback-backlog.md`: tick A1–A4, link this spec and plan from the A heading, and move the 可信度 line out of "Also found on the walk".

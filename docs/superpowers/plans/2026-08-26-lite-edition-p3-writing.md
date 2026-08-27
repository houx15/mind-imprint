# Lite Edition P3 (Writing) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a the student at a lite school type a thought into one box, talk it through clearly with the AI, walk through the four steps 构思 (ideate) → 大纲 (outline) → 段落 (paragraphs) → 成稿 (compose), **write every character themselves**, and come away with feedback and a lite report.

**Architecture:** Writing is the second form of `atom`. **Not a single shared-foundation table needs to be newly created** — `atom` / `atom_message` / `atom_card` / `atom_annotation` / `atom_report` are all reused as-is; writing only adds its own four dedicated tables. The AI layer continues to be reused unchanged (P1 already proved `internal/agent` needs zero modification). The frontend reuses `apps/lite-web`'s shell and `apps/web`'s components and design tokens.

**Tech Stack:** Go (`net/http`, `pgx`, `sqlc`, `goose`), PostgreSQL, React + Vite + TypeScript + Tailwind, Playwright.

**Spec:** `docs/superpowers/specs/2026-08-26-lite-edition-writings-readings-design.md` §6.2

## Global Constraints

- **Migration numbers continue on from P1** (P1 ultimately used up to `0098`, so this phase starts at `0099`). First run `ls apps/api/internal/store/migrations | sort | tail -3` to confirm the current highest number before writing anything.
- **Regenerate sqlc: `cd apps/api && make sqlc`**; never hand-edit `internal/store/sqlc/`.
- 🚨 **Implementers only run targeted tests** (`-run TestX -timeout 1800s`). **Never run the full Go test package** — every integration test spins up its own Postgres container, and a full-package run exceeds ten minutes, past the foreground command limit. Full-package verification runs in the background, coordinated by the controller.
- **Never `git add -A`**; only stage the files listed for this task.
- **An ownership failure is always 404** (`httpx.ErrNotFound("资源不存在")`), never 403.
- `{id}` **is always the atom id**; every handler goes through `loadOwnedWritingAtom` first.
- `apps/api/internal/api` is **the same Go package** shared with pro: names like `putOutline` / `getDraft` / `listSnippets` **are already taken by pro**. lite must always add a `lite` prefix or `Lite` suffix — grep before starting.
- **铁律① (Iron Rule ①) is this phase's red line**: the AI never writes the student's body text. Outline derivation and 「合成全文」("compose the full draft") are **deterministic system steps** (explicitly allowed by AGENTS.md); composing only concatenates fragments the student has already written, **never inventing a single new character**; the English exemplar paragraph is explicitly labeled, separated from the draft, has no insertion entry point in the UI, and **is never written into any draft table**.
- **铁律②**: stages are a map, not a gate; no streaks, leaderboards, or push notifications.
- **铁律④**: skipping a stage is allowed, but **it must leave a trace** and flow into the report's deterministic-facts section.
- `mk-*` are bare CSS variables: `bg-mk-x/NN` never produces any CSS — use `linear-gradient` / `color-mix` / `box-shadow`.

---

### Task 1: The Four Writing-Specific Tables

**Files:**
- Create: `apps/api/internal/store/migrations/00XX_writing_tables.sql` (numbering per Global Constraints)
- Create: `apps/api/internal/store/queries/writing.sql`
- Regenerate: `apps/api/internal/store/sqlc/`
- Test: `apps/api/internal/store/writing_store_test.go`

**Interfaces:**
- Consumes: P1's `atom` table
- Produces: tables `writing` / `writing_outline` / `writing_snippet` / `writing_draft`, plus the sqlc methods `CreateWriting` / `GetWriting` / `ListWritingsByUser` / `RenameWriting` / `SetWritingStage` / `SetWritingTargetWords` / `SetWritingFinished` / `ReplaceWritingOutline` / `ListWritingOutline` / `UpsertWritingSnippet` / `ListWritingSnippets` / `UpsertWritingDraft` / `GetWritingDraft`

- [ ] **Step 1: Write a failing test**

Write it in the shape of `apps/api/internal/store/atom_store_test.go` (P1 Task 1's output), using the package's existing pool helper (`newStoreTestPool`). Must cover:

1. After `CreateWriting`, `stage` defaults to `'ideate'`, `target_words` is NULL, and `status` defaults to `'active'`.
2. `ListWritingsByUser` returns only the caller's own rows, ordered by `atom.created_at DESC`.
3. **Cascade**: after `DELETE FROM atom`, all three tables — outline / snippet / draft — end up empty, the same invariant as reading.
4. The `stage` CHECK rejects invalid values (e.g. `'drafting'`).

- [ ] **Step 2: Run the test to confirm it fails**

```bash
cd apps/api && go test ./internal/store/ -run TestWritingStore -timeout 1800s
```

- [ ] **Step 3: Write the migration**

Take the table structure verbatim from spec §6.2.4. All four tables use `atom_id … REFERENCES atom(id) ON DELETE CASCADE`. The Down migration DROPs in reverse dependency order.

The `stage` CHECK must be `('ideate','outline','snippets','draft','finished')` — **five values, not four**: `finished` is a terminal state, coexisting with `status`'s `finished` (`status` says this piece is done; `stage` says how far along she's gotten).

- [ ] **Step 4: Write the queries, regenerate, run to pass, commit**

```bash
cd apps/api && make sqlc && go test ./internal/store/ -run TestWritingStore -timeout 1800s
git add apps/api/internal/store/migrations/ apps/api/internal/store/queries/writing.sql apps/api/internal/store/sqlc/ apps/api/internal/store/writing_store_test.go
git commit -m "feat(lite): writing atom tables — outline, snippets, draft"
```

---

### Task 2: Writing Atom CRUD (One Box Goes In)

**Files:**
- Create: `apps/api/internal/api/writings.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/writings_test.go`

**Interfaces:**
- Consumes: W1; P1's `liteOnly`, `liteHandler` test helpers
- Produces:
  - `POST /api/v1/writings` — body `{idea, lang}` → `201 {id}`
  - `GET /api/v1/writings` → `{writings:[writingDTO]}`
  - `GET|PATCH /api/v1/writings/{id}`
  - `writingDTO` = `{id,title,lang,stage,targetWords,status,createdAt,updatedAt,finishedAt}`
  - `func (a *API) loadOwnedWritingAtom(w, r) (sqlc.Atom, bool)` — reused by all of W3–W7

- [ ] **Step 1: Write a failing test**

Key behaviors, asserted one by one:

- `POST /writings` creates the atom using **`idea`**: `idea` is the sentence the student typed into the box, **it simultaneously becomes the initial `title`** (truncated to 200 runes), and **is written into the conversation as the first `atom_message` (role='student')** — because the first sentence of 「先聊」("talk first") is exactly this one she said, and it shouldn't vanish.
- An empty `idea` → 400 `missing_idea`. This differs from reading: reading can be created first and have content pasted in later; writing has nothing to talk about without an idea.
- A newly created item has `stage` `'ideate'` and `targetWords` `null`.
- The list contains only the caller's own items, newest first.
- An unknown id → 404; a `writing`-form atom accessed via `/readings/{id}` → 404, and vice versa (**cross-form isolation**, the same invariant as P1 Task 3's wrong-kind assertion).

- [ ] **Step 2-5: run failing → implement → run passing → commit**

Implementation notes: create `atom(kind='writing')` + `writing` + the first `atom_message`, **in the same transaction**.

```bash
cd apps/api && go test ./internal/api/ -run 'TestCreateWriting|TestListWritings|TestGetWriting' -timeout 1800s
```

---

### Task 3: Stage and Length

**Files:**
- Create: `apps/api/internal/api/writing_stage.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/writing_stage_test.go`

**Interfaces:**
- Produces:
  - `POST /api/v1/writings/{id}/stage` — body `{stage}` → `200 writingDTO`
  - `PUT /api/v1/writings/{id}/target-words` — body `{targetWords}` → `200 writingDTO`

- [ ] **Step 1: Write a failing test**

This is the easiest place to get wrong this phase; the tests need to pin the design intent down hard:

- **Stages are a map, not a gate**: jumping straight from `ideate` to `snippets` (skipping `outline`) → **200**, not 400. 铁律②: no gating.
- **But a skip must leave a trace** (铁律④): every stage change writes one `atom_message` (role='system') whose content records from→to. Assert this record exists, **including when skipping**.
- **Going backward is allowed**: `snippets` → `outline` → 200. It's normal for the student to want to go back and fill in the outline.
- An invalid stage value → 400.
- `targetWords` must be a positive integer with an upper bound (e.g. 1..100000); out of range → 400.
- `targetWords` can be set at any stage — although the spec says it's settled during 构思, it's **not enforced**; same principle, a map not a gate.

- [ ] **Step 2-5: run failing → implement → run passing → commit**

---

### Task 4: A Writing 陪练 (Coach) Turn (Reusing the AI Brain)

**Files:**
- Create: `apps/api/internal/api/writing_turn.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/writing_turn_test.go`

**Interfaces:**
- Produces: `POST /api/v1/writings/{id}/turn` → `{reply, decision, card|null, nudge, hintCardId}`; `GET /api/v1/writings/{id}/messages`

- [ ] **Step 1: Read the AI layer first, decide which entry point to reuse**

P1's reading uses `agent.RouteReading`, whose input is 「文章 + the student的话」("article + what the student said"). Writing has no article; what it has is **the idea, the outline, and the fragments already written**.

```bash
grep -rn "^func " apps/api/internal/agent/*.go | grep -iv test | grep -iE "coach|route|guide|propose" | head -20
sed -n '1,60p' apps/api/internal/api/reading_turn.go
```

**Determine this and state it explicitly in the report**: whether to reuse an existing writing/陪练 entry point (if one exists), or to assemble a writing input shaped the same way as `RouteReading`. **Whichever path is chosen, `internal/agent` must not be modified** — if you find you must change it, report BLOCKED; that's a genuine architectural discovery.

- [ ] **Step 2: Write a failing test**

Must cover, the same hard requirements as P1 Task 7:

1. A normal turn: one the student message and one AI message go into `atom_message`, **seq is contiguous**, same transaction.
2. **A model failure is always 502 `ai_dialogue_failed`, never a canned reply** (an established rule of this repo).
3. **`RecentTurns` must be windowed explicitly** (a named constant); lite has no compaction. Reuse P1's `recentTurnsWindow` or define a separate one for writing and explain why.
4. Metering: `surface="lite"`, `purpose="writing_turn"`, `atom_id` recorded on `llm_call`.
5. The whole turn runs on `context.WithoutCancel(r.Context())` + a timeout (P1 Task 7's lesson: a dropped connection would mean 「钱花了、什么都没记下」 — "the money got spent and nothing got recorded").

- [ ] **Step 3-6: implement → run passing → commit**

---

### Task 5: Outline

**Files:**
- Create: `apps/api/internal/api/writing_outline.go`
- Test: `apps/api/internal/api/writing_outline_test.go`

**Interfaces:**
- Produces: `GET|PUT /api/v1/writings/{id}/outline`; `POST /api/v1/writings/{id}/outline/generate`

- [ ] **Step 1: Write a failing test**

- `PUT` is a **full replace** (same semantics as pro's outline): wipe everything and reinsert in array order, `position` = the array index, `depth` clamped to 0..2. The test must distinguish 「全量替换」("full replace") from 「合并」("merge") — PUT three items, then PUT two, and assert only two remain.
- `POST /outline/generate` derives the outline from **what the student has already said** (the content of `atom_message` rows with role='student', plus `title`). **This is a deterministic system step plus one model call, not writing body text on the student's behalf** — it produces structure, not sentences. After generation it **does not auto-overwrite** the existing outline: it returns a candidate, which the student confirms via `PUT`. Assert: generate does not modify the database.
- If `targetWords` is already set, pass it to the model as a granularity signal; generation also works without it (no gate).
- Model failure → 502 `ai_dialogue_failed`.

- [ ] **Step 2-5: implement → run passing → commit**

---

### Task 6: Snippets and the English Exemplar

**Files:**
- Create: `apps/api/internal/api/writing_snippets.go`
- Test: `apps/api/internal/api/writing_snippets_test.go`

**Interfaces:**
- Produces: `GET|PUT /api/v1/writings/{id}/snippets`; `POST /api/v1/writings/{id}/snippets/{sid}/exemplar`

- [ ] **Step 1: Write a failing test — 铁律① is upheld or broken right here**

- A snippet's `text` **comes only from the student**. Assert: no endpoint ever writes model output into `writing_snippet.text`.
- `POST .../exemplar` generates the **English exemplar paragraph**:
  - **Only available when `lang='en'`**; `lang='zh'` → 400 `exemplar_not_available`.
  - In the response body the exemplar text sits in a **separate field** (e.g. `{exemplar: "..."}`), and **never** gets mixed into the snippet response.
  - **Assert against the database**: after the call, `writing_snippet.text` is unchanged by a single character, and **no table anywhere** stores this exemplar. This is the single most important test this phase — it's the mechanical proof of 铁律①.
- Guiding questions are returned as blocks (`{prompts: [...]}`), and likewise never enter a draft table.

- [ ] **Step 2-5: implement → run passing → commit**

---

### Task 7: Compose, Feedback, Finish

**Files:**
- Create: `apps/api/internal/api/writing_compose.go`
- Test: `apps/api/internal/api/writing_compose_test.go`

**Interfaces:**
- Produces: `POST /api/v1/writings/{id}/compose`; `GET|PUT /api/v1/writings/{id}/draft`; `POST /api/v1/writings/{id}/review`; `POST /api/v1/writings/{id}/finish`

- [ ] **Step 1: Write a failing test**

- **`compose` never calls the model**. Assert: inject a panicking provider into `Deps`, and `compose` still succeeds — if it ever called the model, the test would blow up. This is the mechanical proof of 「只拼接、不新造一个字」("only concatenate, never invent a single new character").
- `compose`'s output = the student's fragments concatenated in `position` order (a blank line between paragraphs). Assert that every paragraph in the resulting `body` can be found verbatim in some snippet.
- After `compose`, `PUT /draft` can still be used to keep editing it yourself.
- `review` gives feedback on the whole piece: **returns commentary, does not modify `writing_draft.body`**. Assert the body is unchanged by a single character after the call. Model failure → 502.
- `finish` gates on a **non-empty draft** (400 `missing_draft`), is idempotent, and sets `status='finished'` and `stage='finished'`.
- **Once finished, the server must reject every further write** (the same gate as P1 Task 17, the `loadOwnedWritingAtom` counterpart of `loadOwnedReadingAtom`): every non-GET request is **403 `writing_finished`**, reads still work as usual, and `POST /finish` itself is exempt to stay idempotent. **This is not optional** — 铁律④ makes the process record into evidence, and P2's lite report is generated directly from these rows; if writes are still possible after finishing, the report could end up contradicting the state it's based on. P1 originally implemented this only on the frontend, and review caught it and added the server-side gate afterward; writing must not repeat that mistake. Assert: after finishing, `PUT /snippets`, `POST /turn`, `PUT /draft`, `POST /review` are all 403, while `GET /draft` and `GET /messages` are still 200.

- [ ] **Step 2-5: implement → run passing → commit**

---

### Task 8: Writing Frontend (Landing Page + Writing Page)

**Files:**
- Create: `apps/lite-web/src/writings/*`
- Modify: `apps/lite-web/src/LiteApp.tsx` (replace 「即将上线」 ("coming soon") on the Writing tab)
- Test: `apps/lite-web/test/writings*.test.tsx`

**What it should look like** (the same skeleton as the reading landing page, see P1 Task 16's output, and directly reuse its components):

- Centered greeting + **one box**: 「今天想写点什么」 ("what do you want to write today") — type directly into it and it begins; it's not a form.
- **Suggested topics** (for when you don't know what to write), a **我的写作 (My Writing)** panel (unfinished on top → continue; finished → report), and a **hint bar** (count of unfinished items / a slot for the teacher tasks).
- Once inside: **talk first** (dialogue), with a **four-stage map** shown beside it (构思 / 大纲 / 段落 / 成稿), the current stage highlighted, **clickable to jump** (a map, not a gate).
- Outline stage: generate a candidate + the student edits + confirm.
- Paragraph stage: write paragraph by paragraph following the outline, with guiding questions appearing as blocks; **the English exemplar sits in a visually separate container, labeled 「示范」("exemplar"), with no insert button of any kind**.
- Compose stage: compose → the student can keep editing → request feedback.
**Reuse boundary (finalized 2026-08-27, read this fully before touching anything — this one is different from reading):**

Reading could be reused wholesale because `ReadingRoom` was already a **self-contained, props-driven** room, with the 陪练 inside it, so `ReadingRoomHost` can just mount it by passing in an `api` and `capabilities`.

**Writing has no room like that.** Writing is `apps/web/src/workspace/blocks/WritingBlock.tsx` (2677 lines), and it can only mount inside `WorkspaceContainer.tsx` (2072 lines). Every piece you'd want — `CoachRail` (dialogue + input box), `DraftPane` (the body-text editor), `OutlinePane`, `SnippetsPane` — is **module-private, with no export**, so none of them can be imported from outside at all. And `CoachRail` is portaled into `AiPanel` via `useStudioAiSlot()`, with its dialogue content coming from `useStudioChat()` — both of these contexts are provided only by `WorkspaceContainer`; outside of it, the whole component silently renders empty.

So:

- ❌ **Do not host `WritingBlock`** — it is not a room that can be mounted independently.
- ❌ **Do not modify `WritingBlock.tsx` / `WorkspaceContainer.tsx` for the sake of exporting things**. Hard product constraint: not one line of pro's functionality may change. Turning 陪练 from an ambient context into props that get injected is exactly the kind of high-risk change that 「看起来只是加代码、实际改了 pro 行为」 ("looks like it's just adding code, but actually changes pro's behavior"). If you find yourself wanting to touch these two files → report BLOCKED.
- ✅ **Assemble it yourself out of the primitives below, which are already exported and genuinely independent** (lite's writing page layout is supposed to differ from pro's anyway — the product has explicitly said it doesn't want pro's top title bar and stage bar):

| Purpose | Module | Notes |
|---|---|---|
| **工具卡 (tool card) rendering** | `@/studio/StudioCardSheet` | props `{spec, onSubmit, onSkip, persistKey}`, **doesn't need `projectId`**. This is the actual renderer pro uses; 「卡片与 pro 完全一致」("the card behaves exactly like pro's") is achieved by directly reusing it, not by imitating it. |
| Card standard envelope | `@/studio/compileCard` (`compileCardEnvelope`) | Same one as the backend contract |
| Dialogue area | `@/studio/ai/ChatLog`, `Composer`, `ChatMarkdown` | Independent primitives, no context dependency |
| Base UI + design tokens | `@/ui`, `mk-*` | **Don't invent a new color palette**; `mk-*` are bare CSS variables, and no Tailwind alpha syntax (`bg-mk-x/50`) produces any CSS — use `color-mix()` |
| Landing-page skeleton | P1 Task 16's components | First confirm they can actually generalize; forcing components written for readings onto writings is worse than writing one of each |

What gets duplicated is only **the layout assembly**; the parts that must stay identical (card rendering, the envelope, markdown) are already shared modules to begin with.

- [ ] Steps: implement → `pnpm --filter @mind-imprint/lite-web test && typecheck && build` → **walk through it in a real browser and screenshot it** → commit

---

### Task 9: Writing End-to-End Walkthrough

**Files:** `apps/lite-web/e2e/writing-walk.spec.ts`

One complete walkthrough: open writing → type a thought into the box → AI responds → set the length → generate and confirm the outline → write two paragraph snippets → compose → request feedback → finish.

The assertions must include **mechanical proof for two 铁律**:

1. In the composed body text, **every paragraph can be found in text the student actually typed** — no sentence appears out of nowhere.
2. When writing in English, the exemplar paragraph **exists on the page**, but is **not in the draft box**, and **no button can insert it there**.

```bash
pnpm --filter @mind-imprint/lite-web exec playwright test -c e2e/playwright.config.ts
```

---

## Self-Check (against the spec, after writing the plan)

| Spec §6.2 item | Which task it lands in |
|---|---|
| One box in, write the idea directly | W2 (`idea` creates the atom and becomes the first message) |
| Talk first once inside | W4 |
| Four stages 构思/大纲/段落/成稿 | W3 (state and traces), W8 (map UI) |
| Length settled during 构思 | W3 (`target-words`) |
| Outline derivation + the student can edit | W5 |
| Guided fragment writing | W6 |
| English exemplar (never enters the draft) | W6 (server-side proof), W9 (on-page proof) |
| Compose the full text (concatenation only) | W7 (panic-provider proof) |
| AI feedback | W7 |
| Suggested topics / history / unfinished / the teacher-task slot | W8 |
| Lite report | **Not done this phase** — belongs to P2's `atom_report`, same as the reading report; writing hooks in when that lands |
| Stages are a map not a gate + skips leave a trace | W3 |

**External names that must be verified by hand at implementation time**: the current highest migration number (W1); the real signature of the reusable writing/陪练 entry point in `internal/agent` (W4 Step 1); handler names pro already occupies (grep before starting each task); the landing-page component names produced by P1 Task 16 (W8).

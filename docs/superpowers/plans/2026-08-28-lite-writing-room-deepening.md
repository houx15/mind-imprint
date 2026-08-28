# Lite Writing Room Deepening — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make lite's writing room teach — a plan that can shape a whole piece, guidance that is present and names real methods, comments that quote her own sentences, a Socratic sub-agent on one block, and 成稿 typeset as a page.

**Architecture:** One additive migration adds a thread scope to `atom_message`, persisted guidance to `writing_outline`, and a `writing_comment` table. A new embedded vocabulary library (JSON, `go:embed` + build-time TS import) becomes the only source of method names and examples, so terminology is accurate and examples are borrowed material by construction. Backend endpoints then grow one at a time, and the lite frontend is rebuilt on top of them.

**Tech Stack:** Go (`net/http`, `pgx`, `sqlc` **pinned `@v1.27.0`**, `goose`), PostgreSQL, React + Vite + TypeScript + Tailwind, vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-08-28-lite-writing-room-deepening-design.md`
**Backlog:** `docs/2026-08-28-lite-edition-feedback-backlog.md` (sub-project **B**)

## Global Constraints

- **铁律① — AI never writes her prose.** Enforce by output *type* wherever possible, not by prompt manners.
- **AI never authors an outline in lite.** `POST /outline/generate` stays deleted; do not reintroduce.
- **结构 is planning, not a picker.** No skeleton library, no tile grid of structures.
- **印记 may name accurate methods in conversation; it may never render them as a picker.**
- **Every student-facing turn does four things:** says why it matters, names the real methods, offers a genuine choice, offers to teach with examples. Rejected style: 「有人会从一个具体场景切进去，有人直接抛个问题。你这篇你想怎么进？」
- **Method names must come from the vocabulary library.** 印记 selects and tunes; it never invents a term.
- **Examples must be borrowed material** — pre-authored, about topics other than the student's.
- **Raise every student-facing prompt's reply cap from `不超过 120 字` to `不超过 200 字`.** 铁律③ still holds: one *question* per turn.
- **UI labels stay plain and short** (「打开」 not 「把这张卡叫出来」). This constrains chrome, not how 印记 teaches.
- **Lite must never break pro.** `apps/web/tailwind.config.ts` is shared. Run pro's full suite before shipping.
- **Never `git add -A`.** Stage the exact files each task names.
- **Go tests:** run from `apps/api` with `CGO_ENABLED=0` and `-timeout 1800s`.
- **e2e is NOT type-checked** by the normal command (`apps/lite-web/tsconfig.json` includes only `["src","test"]`). Check it explicitly.

---

## File Structure

**New — data**
- `packages/contracts/vocab/methods.json` — the vocabulary library. Single source of truth: `go:embed`ed by the server, imported at build time by the client.
- `apps/api/internal/store/migrations/0102_writing_room_deepening.sql`

**New — Go**
- `apps/api/internal/vocab/vocab.go` — embeds and validates `methods.json`; exposes lookup by id and by `applies_to`.
- `apps/api/internal/api/writing_comment.go` — comment generation, quote validation, persistence (B4 + B7).
- `apps/api/internal/api/writing_deepen.go` — the block-scoped Socratic sub-agent (B3).

**Modified — Go**
- `apps/api/internal/store/queries/atom.sql` — `ListAtomMessages` becomes scope-safe; two new queries.
- `apps/api/internal/store/queries/writing.sql` — guide persistence, comment CRUD.
- `apps/api/internal/api/writing_plan.go` — whole-piece judgment, multi-root, raised cap.
- `apps/api/internal/api/writing_guide.go` — four-part guide, batch, persisted.
- `apps/api/internal/api/writing_snippets.go` — remove the exemplar endpoint.
- `apps/api/internal/api/writing_compose.go` — review returns a comment object.
- `apps/api/internal/api/api.go` — routes.

**New — lite frontend**
- `apps/lite-web/src/writings/ProseSurface.tsx` — the textarea + mirrored highlight layer. **Owns the shared typography constant.**
- `apps/lite-web/src/writings/CommentPanel.tsx` — renders a comment, click-to-trace.
- `apps/lite-web/src/writings/VocabExamples.tsx` — library examples and EN patterns.
- `apps/lite-web/src/writings/DeepenDrawer.tsx` — B3's drawer.
- `apps/lite-web/src/writings/EditableTitle.tsx` — B6.

**Modified — lite frontend**
- `apps/web/tailwind.config.ts` — add `mk-prose` (shared with pro; additive only).
- `apps/lite-web/src/writings/GuideBox.tsx`, `SnippetsStage.tsx`, `ComposeStage.tsx`, `MindMap.tsx`, `WritingRoomHost.tsx`, `src/api/writingRoom.ts`.
- `apps/lite-web/e2e/writing-walk.spec.ts`.

---

## Task 1: Migration and scope-safe queries

The spec's §11.1 names this the highest risk: `atom_message` has never had a scope, so all seven `ListAtomMessages` call sites assume every row is the main thread. **The fix is to make the default safe rather than to edit seven callers** — `ListAtomMessages` itself filters `block_id IS NULL`, so every existing caller becomes correct with no edit, and a new query serves block threads.

Enumerated `ListAtomMessages` call sites (all become correct automatically): `writing_setup.go:198`, `reading_turn.go:106`, `reading_turn.go:150`, `writing_turn.go:236`, `writing_plan.go:349`, `writing_guide.go:258`, `reading_coach.go:333`.

`AppendAtomMessage` is left **unchanged** so its eight call sites need no edit; `block_id` defaults to NULL. A new append query serves block threads.

**Files:**
- Create: `apps/api/internal/store/migrations/0102_writing_room_deepening.sql`
- Modify: `apps/api/internal/store/queries/atom.sql`
- Modify: `apps/api/internal/store/queries/writing.sql`
- Test: `apps/api/internal/api/writing_scope_test.go`

**Interfaces:**
- Produces: sqlc methods `ListAtomMessages(ctx, atomID)` (now main-thread only), `ListAtomBlockMessages(ctx, ListAtomBlockMessagesParams{AtomID, BlockID})`, `AppendAtomBlockMessage(ctx, AppendAtomBlockMessageParams{AtomID, Seq, Role, Content, BlockID})`, `SetWritingOutlineGuide(ctx, SetWritingOutlineGuideParams{ID, Guide})`, `CreateWritingComment`, `ListWritingComments`, `GetLatestWritingDraftComment`.

- [ ] **Step 1: Write the migration**

`apps/api/internal/store/migrations/0102_writing_room_deepening.sql`:

```sql
-- +goose Up
-- B3: a block-scoped thread for the 深入一层 sub-agent. Mirrors atom_card.block_id
-- (0092), which already scopes a row to one block of an atom.
--
-- NULL = the room's own thread. Set = a sub-agent conversation about one
-- writing_outline node. Seq stays in ONE space per atom, so atom_message_seq_idx
-- (UNIQUE (atom_id, seq)) is untouched and global ordering stays well-defined.
ALTER TABLE atom_message ADD COLUMN block_id text;
CREATE INDEX atom_message_block_idx ON atom_message (atom_id, block_id, seq);

-- B1: guidance persisted so opening 段落 does not re-spend a model call.
-- Sibling of `role` (0100), NOT of `text`: role/guide are scaffold, text is hers.
-- Nothing composes a draft from this column — writing_draft is built from
-- writing_snippet.text only.
ALTER TABLE writing_outline ADD COLUMN guide jsonb;

-- B4 + B7: one shape at two zoom levels. snippet_id NULL = a comment on the
-- whole draft. points is [{"text":"...","quote":"..."}]; every quote has already
-- been verified as a literal substring of the commented text before insert.
CREATE TABLE writing_comment (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  snippet_id uuid REFERENCES writing_snippet(id) ON DELETE CASCADE,
  scope      text NOT NULL CHECK (scope IN ('block','draft')),
  summary    text NOT NULL,
  points     jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX writing_comment_atom_idx ON writing_comment (atom_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS writing_comment;
ALTER TABLE writing_outline DROP COLUMN IF EXISTS guide;
DROP INDEX IF EXISTS atom_message_block_idx;
ALTER TABLE atom_message DROP COLUMN IF EXISTS block_id;
```

- [ ] **Step 2: Make `ListAtomMessages` scope-safe and add the new queries**

In `apps/api/internal/store/queries/atom.sql`, replace the `ListAtomMessages` block with:

```sql
-- name: ListAtomMessages :many
-- The room's OWN thread only. The `block_id IS NULL` filter is the whole safety
-- property of 0102: seven callers across reading and writing read this query and
-- every one of them means "the main conversation". Making the default safe is why
-- none of them needed editing when block scoping arrived — do not remove it, and
-- do not add a variant that omits it.
SELECT * FROM atom_message WHERE atom_id = $1 AND block_id IS NULL ORDER BY seq;

-- name: ListAtomBlockMessages :many
SELECT * FROM atom_message
WHERE atom_id = $1 AND block_id = $2
ORDER BY seq;

-- name: AppendAtomBlockMessage :one
INSERT INTO atom_message (atom_id, seq, role, content, block_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
```

Leave `AppendAtomMessage`, `NextAtomMessageSeq` and `CountAtomEvidence` untouched.

- [ ] **Step 3: Add the writing queries**

Append to `apps/api/internal/store/queries/writing.sql`:

```sql
-- name: SetWritingOutlineGuide :exec
UPDATE writing_outline SET guide = $2 WHERE id = $1;

-- name: CreateWritingComment :one
INSERT INTO writing_comment (atom_id, snippet_id, scope, summary, points)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListWritingComments :many
SELECT * FROM writing_comment WHERE atom_id = $1 ORDER BY created_at DESC;

-- name: GetLatestWritingDraftComment :one
SELECT * FROM writing_comment
WHERE atom_id = $1 AND scope = 'draft'
ORDER BY created_at DESC
LIMIT 1;
```

- [ ] **Step 4: Regenerate sqlc**

```bash
cd apps/api && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate
```

Expected: `internal/store/sqlc/` regenerates with the new methods and `AtomMessage.BlockID pgtype.Text`. The version pin is mandatory — an unpinned sqlc has produced incompatible output before.

- [ ] **Step 5: Write the failing scope test**

`apps/api/internal/api/writing_scope_test.go`:

```go
package api

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// The safety property of migration 0102: a sub-agent turn must be invisible to
// every reader of the room's own thread. If this fails, side conversations leak
// into 印记's planning window and nothing else fails loudly.
func TestListAtomMessages_ExcludesBlockScopedTurns(t *testing.T) {
	ctx := context.Background()
	d, q := newTestDeps(t)
	atomID := seedWritingAtom(t, d)

	if _, err := q.AppendAtomMessage(ctx, AppendAtomMessageParams{
		AtomID: atomID, Seq: 1, Role: "student", Content: "主线里的话",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.AppendAtomBlockMessage(ctx, AppendAtomBlockMessageParams{
		AtomID: atomID, Seq: 2, Role: "student", Content: "只属于这一块的话",
		BlockID: pgtype.Text{String: "block-1", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	main, err := q.ListAtomMessages(ctx, atomID)
	if err != nil {
		t.Fatal(err)
	}
	if len(main) != 1 || main[0].Content != "主线里的话" {
		t.Fatalf("main thread = %d rows %v, want only the main-thread turn", len(main), contents(main))
	}

	scoped, err := q.ListAtomBlockMessages(ctx, ListAtomBlockMessagesParams{
		AtomID: atomID, BlockID: pgtype.Text{String: "block-1", Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 1 || scoped[0].Content != "只属于这一块的话" {
		t.Fatalf("block thread = %d rows %v, want only the block turn", len(scoped), contents(scoped))
	}
}
```

Reuse the existing package helpers for `newTestDeps` / `seedWritingAtom`; if the package names them differently, match the convention already used in `writing_turn_test.go` rather than inventing new ones. Add a small `contents` helper in this file if the package has none.

- [ ] **Step 6: Run it and watch it fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestListAtomMessages_ExcludesBlockScopedTurns -timeout 1800s -v
```

Expected before the migration is applied by the test harness: FAIL. After Steps 1–4 it should PASS — if it passes before you write it, the harness is not applying 0102 and that must be fixed first.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/store/migrations/0102_writing_room_deepening.sql \
        apps/api/internal/store/queries/atom.sql \
        apps/api/internal/store/queries/writing.sql \
        apps/api/internal/store/sqlc \
        apps/api/internal/api/writing_scope_test.go
git commit -m "feat(lite): give atom_message a thread scope, safe side up

ListAtomMessages itself now filters block_id IS NULL, so all seven of its
callers stay correct without being touched. The alternative — editing each
one — would have been correct exactly until someone forgot."
```

---

## Task 2: The vocabulary library

The library is the single source of truth for every method name 印记 may use, and its examples are borrowed material *by construction* — closing the 铁律① back door with data rather than with prompt instructions.

**Files:**
- Create: `packages/contracts/vocab/methods.json`
- Create: `apps/api/internal/vocab/vocab.go`
- Test: `apps/api/internal/vocab/vocab_test.go`

**Interfaces:**
- Produces (Go): `vocab.All() []Method`, `vocab.ByID(id string) (Method, bool)`, `vocab.For(appliesTo string) []Method`, with `type Method struct { ID, Name, AppliesTo, Definition string; Examples []Example; Patterns []Pattern }`, `type Example struct { Topic, Text string }`, `type Pattern struct { Label, Frame string }`.
- Produces (TS, Task 10 consumes): the same JSON imported directly.

- [ ] **Step 1: Write the library**

`packages/contracts/vocab/methods.json`. `applies_to` is one of `opening` / `body` / `closing` / `any`. Every `examples[].topic` must be unrelated to typical student essay topics so it can never read as their content. This is the starter set; it is complete enough to ship and is meant to grow.

```json
{
  "methods": [
    {
      "id": "opening_suspense",
      "name": "留悬念",
      "applies_to": "opening",
      "definition": "先抛出一个还没有答案的情况，让读者为了知道结果而读下去。",
      "examples": [
        { "topic": "南极科考", "text": "1912 年 1 月，斯科特一行到达南极点，却发现雪地里已经插着一面旗子。那面旗子改变了他们回程的每一个决定。" }
      ],
      "patterns": []
    },
    {
      "id": "opening_question",
      "name": "设问",
      "applies_to": "opening",
      "definition": "开篇提一个真问题，然后用全文来回答它。问题要具体，大到无法回答的问题不算设问。",
      "examples": [
        { "topic": "地铁票价", "text": "一张地铁票卖三块钱，可运一个人真正花掉的是七块。剩下那四块，是谁在付？" }
      ],
      "patterns": []
    },
    {
      "id": "opening_direct",
      "name": "开门见山",
      "applies_to": "opening",
      "definition": "第一句就把结论说出来，后面全部用来支撑它。适合读者已经关心这件事的时候。",
      "examples": [
        { "topic": "校车安全", "text": "校车该不该装安全带，其实早有答案：该装，而且早就该装了。" }
      ],
      "patterns": []
    },
    {
      "id": "point_parallel",
      "name": "并列",
      "applies_to": "body",
      "definition": "几条互不依赖的理由并排放。稳，但要小心几条一样重就没有高潮。",
      "examples": [
        { "topic": "图书馆延长开放", "text": "延长开放有三个理由：夜里的座位一直不够、住得远的人白天来不了、闭馆太早反而把人赶去了更吵的地方。" }
      ],
      "patterns": []
    },
    {
      "id": "point_progressive",
      "name": "递进",
      "applies_to": "body",
      "definition": "是什么 → 为什么 → 怎么办。后一层必须依赖前一层，否则它其实是并列。",
      "examples": [
        { "topic": "路灯改造", "text": "这条路晚上太暗；暗是因为灯距按二十年前的车流定的；所以要改的不是换灯泡，是重新算灯距。" }
      ],
      "patterns": []
    },
    {
      "id": "point_contrast",
      "name": "正反",
      "applies_to": "body",
      "definition": "一正一反两个例子放在一起，差别本身就是论证。两个例子要在其他条件上尽量相似。",
      "examples": [
        { "topic": "两个菜市场", "text": "东街的市场留了装卸区，早晨不堵；西街的没留，同样的车流每天堵四十分钟。" }
      ],
      "patterns": []
    },
    {
      "id": "point_pee",
      "name": "举例（PEE）",
      "applies_to": "body",
      "definition": "观点（Point）→ 例子（Example）→ 解释（Explanation）。最常被漏掉的是第三步：说清这个例子为什么能证明那个观点。",
      "examples": [
        { "topic": "自行车道", "text": "隔离带比划线管用。某条路加了隔离带之后，机动车占道从每天十几起降到零。因为划线只是提示，隔离带是物理上进不去。" }
      ],
      "patterns": []
    },
    {
      "id": "point_concession",
      "name": "让步",
      "applies_to": "body",
      "definition": "先真诚承认对方最强的那一点，再说明它为什么不足以推翻你的结论。承认得越具体，转折越有力。",
      "examples": [
        { "topic": "夜市管理", "text": "夜市确实吵，住在楼上的人休息不好，这一点不能装作没有。但吵的是通宵那几家，不是整条街，所以该管的是时间，不是取消。" }
      ],
      "patterns": []
    },
    {
      "id": "point_causal",
      "name": "因果",
      "applies_to": "body",
      "definition": "指出前因如何导致后果。要小心「先后」不等于「因果」，中间的机制得说出来。",
      "examples": [
        { "topic": "早自习", "text": "早自习提前半小时，迟到反而变多了——因为最早的一班公交没有跟着提前。" }
      ],
      "patterns": []
    },
    {
      "id": "closing_return",
      "name": "回扣开头",
      "applies_to": "closing",
      "definition": "结尾回到开头那个场景或问题，用全文得到的东西重新回答一次。读者会觉得这篇是完整的。",
      "examples": [
        { "topic": "地铁票价", "text": "所以再看开头那四块钱：它不是补贴谁的善意，是我们共同决定要不要让这座城市跑得起来。" }
      ],
      "patterns": []
    },
    {
      "id": "closing_scope",
      "name": "收束到可做的事",
      "applies_to": "closing",
      "definition": "把大结论收回到一件具体、可执行的事上。比喊口号更有力，也更难反驳。",
      "examples": [
        { "topic": "校车安全", "text": "不必等全套法规。先把已有的校车按最旧的一批排序，今年换掉前二十辆，就是可以做的第一步。" }
      ],
      "patterns": []
    },
    {
      "id": "en_concession",
      "name": "Conceding, then turning",
      "applies_to": "body",
      "definition": "English concessive frames — admit the opposing point, then limit it.",
      "examples": [],
      "patterns": [
        { "label": "Admit then limit", "frame": "While it is true that ___, this does not mean ___." },
        { "label": "Grant the strongest case", "frame": "Critics are right that ___. What this overlooks, however, is ___." }
      ]
    },
    {
      "id": "en_qualify",
      "name": "Qualifying a claim",
      "applies_to": "any",
      "definition": "English hedging frames — state how strongly the claim is meant.",
      "examples": [],
      "patterns": [
        { "label": "Degree", "frame": "To a large extent, ___ — though ___." },
        { "label": "Bounded claim", "frame": "In the case of ___, ___; it is less clear whether ___." }
      ]
    },
    {
      "id": "en_evidence",
      "name": "Introducing evidence",
      "applies_to": "body",
      "definition": "English frames for bringing a source in and saying what it shows.",
      "examples": [],
      "patterns": [
        { "label": "Cite a study", "frame": "A 2019 study of ___ found that ___." },
        { "label": "Say what it shows", "frame": "This suggests that ___, because ___." }
      ]
    }
  ]
}
```

- [ ] **Step 2: Write the failing loader test**

`apps/api/internal/vocab/vocab_test.go`:

```go
package vocab

import "testing"

// The library is the ONLY place 印记 may get a method name from, so a malformed
// entry is a product defect, not a runtime inconvenience — it must fail at load.
func TestLoad_EveryMethodIsUsable(t *testing.T) {
	all := All()
	if len(all) < 10 {
		t.Fatalf("library has %d methods, want at least 10", len(all))
	}
	seen := map[string]bool{}
	for _, m := range all {
		if m.ID == "" || m.Name == "" || m.Definition == "" {
			t.Errorf("method %+v is missing id, name or definition", m)
		}
		if seen[m.ID] {
			t.Errorf("duplicate method id %q", m.ID)
		}
		seen[m.ID] = true
		switch m.AppliesTo {
		case "opening", "body", "closing", "any":
		default:
			t.Errorf("method %q has applies_to %q, want opening|body|closing|any", m.ID, m.AppliesTo)
		}
		// Borrowed material by construction: an example with no topic cannot be
		// checked for being about someone else's subject.
		for _, ex := range m.Examples {
			if ex.Topic == "" || ex.Text == "" {
				t.Errorf("method %q has an example missing topic or text", m.ID)
			}
		}
		for _, p := range m.Patterns {
			if p.Frame == "" {
				t.Errorf("method %q has a pattern with no frame", m.ID)
			}
		}
		if len(m.Examples) == 0 && len(m.Patterns) == 0 {
			t.Errorf("method %q teaches nothing: no examples and no patterns", m.ID)
		}
	}
}

func TestByID_AndFor(t *testing.T) {
	if _, ok := ByID("point_concession"); !ok {
		t.Fatal(`ByID("point_concession") not found`)
	}
	if _, ok := ByID("no_such_method"); ok {
		t.Fatal("ByID returned ok for an unknown id")
	}
	openings := For("opening")
	if len(openings) == 0 {
		t.Fatal(`For("opening") returned nothing`)
	}
	for _, m := range openings {
		if m.AppliesTo != "opening" && m.AppliesTo != "any" {
			t.Errorf("For(\"opening\") returned %q with applies_to %q", m.ID, m.AppliesTo)
		}
	}
}
```

- [ ] **Step 3: Run it and watch it fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/vocab/ -timeout 1800s -v
```

Expected: FAIL — package `vocab` does not exist.

- [ ] **Step 4: Write the loader**

`apps/api/internal/vocab/vocab.go`:

```go
// Package vocab is the single source of truth for the method names 印记 is
// allowed to use, their definitions, and the examples it may show.
//
// Two properties this package exists to guarantee:
//
//   - TERMINOLOGY IS OURS. A teacher is rigorous about terms; a model improvising
//     「递进式论证法」 on one turn and 「层进」 on the next teaches nothing. 印记
//     selects from this file and tunes; it never invents a term. An id the model
//     returns that is not in here is dropped by the caller.
//   - EXAMPLES ARE BORROWED MATERIAL. Every example here is pre-authored about a
//     topic no student is writing on. An "example 留悬念 opening" generated about
//     her own thesis IS her opening, authored by AI — 铁律① through the back door.
//     Shipping the examples as data closes that door structurally.
//
// The same JSON is imported at build time by apps/lite-web, so the two ends can
// never disagree about what a method is called.
package vocab

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed ../../../../packages/contracts/vocab/methods.json
var methodsJSON []byte

type Example struct {
	Topic string `json:"topic"`
	Text  string `json:"text"`
}

type Pattern struct {
	Label string `json:"label"`
	Frame string `json:"frame"`
}

type Method struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	AppliesTo  string    `json:"applies_to"`
	Definition string    `json:"definition"`
	Examples   []Example `json:"examples"`
	Patterns   []Pattern `json:"patterns"`
}

var loaded []Method
var byID map[string]Method

func init() {
	var doc struct {
		Methods []Method `json:"methods"`
	}
	if err := json.Unmarshal(methodsJSON, &doc); err != nil {
		panic(fmt.Sprintf("vocab: methods.json is not valid: %v", err))
	}
	loaded = doc.Methods
	byID = make(map[string]Method, len(loaded))
	for _, m := range loaded {
		byID[m.ID] = m
	}
}

func All() []Method { return loaded }

func ByID(id string) (Method, bool) {
	m, ok := byID[id]
	return m, ok
}

// For returns the methods usable at a position in the piece. "any" is always
// included, because a method that fits everywhere fits here too.
func For(appliesTo string) []Method {
	out := make([]Method, 0, len(loaded))
	for _, m := range loaded {
		if m.AppliesTo == appliesTo || m.AppliesTo == "any" {
			out = append(out, m)
		}
	}
	return out
}
```

If the relative `go:embed` path does not resolve (embed cannot escape the module root), instead copy the JSON into `apps/api/internal/vocab/methods.json` via a `go:generate` line and embed it locally — but keep `packages/contracts/vocab/methods.json` as the editable source, and add a test that the two files are byte-identical so they cannot drift.

- [ ] **Step 5: Run the tests**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/vocab/ -timeout 1800s -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/vocab/methods.json apps/api/internal/vocab/
git commit -m "feat(lite): give 印记 a vocabulary it did not invent

Method names, definitions and examples ship as reviewed data. Two things
follow that a prompt could only ask for: the terms are accurate and stable
across pieces, and every example is about someone else's topic — so an
example opening can never quietly be her opening."
```

---

## Task 3: B0 — the plan can shape a whole piece

**Files:**
- Modify: `apps/api/internal/api/writing_plan.go` (system prompt ~line 95–120; `writingPlanMaxDepth` line 65)
- Test: `apps/api/internal/api/writing_plan_test.go`

**Interfaces:**
- Consumes: `vocab.All()` from Task 2.
- Produces: plan turns may now add nodes with `parentId:""` that are not the thesis; `writing_outline` rows at `depth=0` with `position` ordering the document.

- [ ] **Step 1: Write the failing test**

Append to `apps/api/internal/api/writing_plan_test.go`:

```go
// B0: an opening is a top-level sibling of the thesis, ordered before it — not a
// child of it. The old prompt taught the model that the only depth-0 node IS the
// thesis, so this is the assertion that the teaching changed.
func TestPlanTurn_AcceptsATopLevelOpeningBesideTheThesis(t *testing.T) {
	// Existing thesis at depth 0, position 0.
	// Model adds an opening with parentId "" — it must be stored at depth 0 and
	// must NOT be reparented under the thesis or dropped.
	out := planTurnWith(t,
		`{"reply":"记下了。","add":[{"parentId":"","text":"夏天路上晒得受不了","role":"开头"}]}`)

	var roots []string
	for _, n := range out.Outline {
		if n.Depth == 0 {
			roots = append(roots, n.Role)
		}
	}
	if len(roots) < 2 {
		t.Fatalf("depth-0 nodes = %v, want the thesis AND the opening as siblings", roots)
	}
}
```

Match the existing helper style in this file (`writing_plan_test.go:46` already stubs a model reply the same way); if the file has no `planTurnWith` helper, extract one from the existing test rather than duplicating its setup.

- [ ] **Step 2: Run it and watch it fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestPlanTurn_AcceptsATopLevelOpeningBesideTheThesis -timeout 1800s -v
```

Expected: FAIL.

- [ ] **Step 3: Change the prompt**

Three edits in `writing_plan.go`'s system prompt:

1. Replace `- parentId：父节点的 id，逐字取自下面【当前的图】里给出的 id。加在最上层（中心论点）就留空字符串。` with:

```
- parentId：父节点的 id，逐字取自下面【当前的图】里给出的 id。留空字符串＝加在最上层。
  最上层不止中心论点：开头、结尾也都是最上层的块，按它们在文章里的先后排。
```

2. Raise the cap: `- reply：不超过 120 字，一次一个问题。` becomes `- reply：不超过 200 字，一次一个问题。`

3. Replace the ladder section with whole-piece judgment. Add, in the prompt's teaching section:

```
## 你心里要装着「一整篇」

一篇写完的文章要为读者做三件事：**开头让他愿意读下去，中间真的在论证，
结尾让他带走点东西。** 这是你的判断力，不是一张要逐项打勾的清单。

每一轮，你看一眼整张图，只挑**这篇现在最需要的那一件事**说。可能是
「读者一上来不知道为什么要关心这件事」，可能是「理由二和理由三其实是同一条」，
也可能是「理由一底下什么都没有」。**有时候答案是什么都不缺，让她去写。**

开头和结尾要等主体有了再谈——不知道要把人领进哪里，就没法决定怎么开门。

## 怎么说话（这条比什么都重要）

你是老师，不是问答机器。每次开口都要做到四件事：
① 说清这件事为什么重要；② 说出真正的方法名（下面【可用的方法】里的，别自己造词）；
③ 给她一个真的选择，她也可以不选；④ 主动提出可以举例子一起看。

不要这样说：「有人会从一个具体场景切进去，有人直接抛个问题。你这篇你想怎么进？」
要这样说：「对于一篇文章来说，有意思的开头非常重要。留悬念、设问、开门见山等，
都是常见的方式。你想尝试哪一种？或者需要我给几个具体的案例我们一起来学习一下
这几种方法吗？」

一次仍然只问**一个**问题——「一次只问一个」说的是问题的数量，从来不是让你少说话。
```

4. Add the library to the prompt in `buildWritingPlanPrompt`, after the map:

```go
b.WriteString("\n【可用的方法】（只能用这里的名字，别造新词）\n")
for _, m := range vocab.All() {
    if m.AppliesTo == "opening" || m.AppliesTo == "closing" || m.AppliesTo == "body" {
        b.WriteString("- " + m.Name + "（" + m.AppliesTo + "）：" + m.Definition + "\n")
    }
}
```

- [ ] **Step 4: Widen the depth ceiling comment**

`writingPlanMaxDepth` (line 65) keeps its value; update its comment, which currently reads *"the map's depth ceiling: 中心论点 → 分论点 → 论据"*, to say that depth 0 now holds the opening, the thesis and the landing as document-ordered siblings.

- [ ] **Step 5: Run the tests**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestPlanTurn -timeout 1800s -v
```

Expected: PASS, including the pre-existing add-only tests — a plan turn still cannot update or delete a node.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/writing_plan.go apps/api/internal/api/writing_plan_test.go
git commit -m "feat(lite): 印记 plans a whole piece, not just its claims

The prompt walked thesis → reasons → evidence and stopped, so the map could
only grow claims and 段落 handed out boxes for the body of an essay with no
way in and no way out.

It now carries judgment about a finished piece and spends each turn on the
one thing THIS piece most needs — which is sometimes nothing. Openings and
landings are top-level siblings in document order, so they cost no schema.

Also raises the reply cap to 200字: at 120 the model could not say why
something matters, name the method and offer an example, so it produced the
clipped shrug we kept mistaking for the model's personality."
```

---

## Task 4: B1 — guidance present on arrival, and teaching

**Files:**
- Modify: `apps/api/internal/api/writing_guide.go`
- Modify: `apps/api/internal/api/api.go` (add the batch route)
- Test: `apps/api/internal/api/writing_guide_internal_test.go`

**Interfaces:**
- Consumes: `vocab.For`, `SetWritingOutlineGuide`.
- Produces: `type writingGuideResult struct { Job string; MethodIDs []string; Questions []string }` serialised as `{"job":..., "methods":[{"name":...,"definition":...,"examples":[...],"patterns":[...]}], "questions":[...]}`; route `POST /api/v1/writings/{id}/guide` (all blocks, batched).

**Conflict this task must resolve, deliberately.** `guideWritingBlock`'s doc comment currently says *"PERSISTS NOTHING… storing them would put model prose inside the writing's own record, where a later reader could mistake it for hers."* The spec overrides this: guidance is persisted in `writing_outline.guide`, a **sibling of `role` (scaffold), not of `text` (hers)** — the same separation migration 0100 established. The ruling is safe only because nothing composes a draft from the outline. **Update that comment** so the codebase does not carry a stale rationale, and add the test in Step 5 that pins the reason.

- [ ] **Step 1: Write the failing parser tests**

In `writing_guide_internal_test.go`:

```go
// The ？ filter is the security boundary and it must survive the guide growing
// prose fields: a declarative sentence in `questions` is a sentence for her essay.
func TestParseWritingGuide_QuestionFilterAppliesToQuestionsOnly(t *testing.T) {
	got, ok := parseWritingGuide(`{
      "job":"这一段要让读者相信「便宜」这个说法不成立。",
      "method_ids":["point_contrast","no_such_method"],
      "questions":["你见过哪条街上的树长不开？","你可以写：树会抢水。","这跟成本有什么关系？"]
    }`)
	if !ok {
		t.Fatal("parse failed")
	}
	if len(got.Questions) != 2 {
		t.Fatalf("questions = %v, want the declarative one dropped", got.Questions)
	}
	if got.Job == "" {
		t.Error("job was dropped; the prose field must survive the filter")
	}
	// An id the model invented is not a method. Dropping it is what keeps
	// terminology ours.
	if len(got.MethodIDs) != 1 || got.MethodIDs[0] != "point_contrast" {
		t.Fatalf("method ids = %v, want only the known one", got.MethodIDs)
	}
}

func TestParseWritingGuide_FailsWhenNoQuestionsSurvive(t *testing.T) {
	if _, ok := parseWritingGuide(`{"job":"x","method_ids":[],"questions":["你可以写：手机让人分心。"]}`); ok {
		t.Fatal("parse succeeded with zero surviving questions; a guide with no questions teaches her nothing")
	}
}
```

- [ ] **Step 2: Run and watch fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestParseWritingGuide -timeout 1800s -v
```

Expected: FAIL — `Job` and `MethodIDs` do not exist.

- [ ] **Step 3: Grow the result type and the filter**

In `writing_guide.go`, replace the type and add id filtering inside `parseWritingGuide`, keeping the existing `？` loop **exactly as it is**:

```go
type writingGuideResult struct {
	// Job is one line on what this KIND of block must accomplish for a reader.
	// It is about the block's job, never about her topic, which is why it is not
	// covered by the ？ filter below.
	Job string `json:"job"`
	// MethodIDs are ids from the vocab library. Unknown ids are dropped in the
	// parser: 印记 selects and tunes, it never invents a term.
	MethodIDs []string `json:"method_ids"`
	Questions []string `json:"questions"`
}
```

After the existing question loop, add:

```go
	keptIDs := make([]string, 0, len(got.MethodIDs))
	for _, id := range got.MethodIDs {
		if _, known := vocab.ByID(strings.TrimSpace(id)); known {
			keptIDs = append(keptIDs, strings.TrimSpace(id))
		}
	}
	got.MethodIDs = keptIDs
```

- [ ] **Step 4: Update the system prompt**

`writingGuideSystem` gains the same four-part copy rule as Task 3 Step 3 (copy it verbatim — the engineer may be reading tasks out of order), plus:

```
输出 JSON：{"job":"…","method_ids":["…"],"questions":["…？"]}
- job：一句话说清这一块要为读者做成什么事。说的是这一块的任务，不是她的内容。
- method_ids：从【可用的方法】里挑 1–3 个适合这一块的，只给 id。
- questions：2–4 个问题，每一个都必须以问号结尾。问的是她的材料，不是抽象概念。
```

And append `vocab.For(...)` to `buildWritingGuidePrompt`, selected by the block's `role` (map an unrecognised role to `"body"`).

- [ ] **Step 5: Add the batch endpoint and persistence**

New handler `guideWritingBlocks` for `POST /api/v1/writings/{id}/guide`: loads the outline, calls the model **once** with all blocks, and writes each block's guide via `SetWritingOutlineGuide`. `GET /outline` returns the stored guide so the client paints it on first render.

Route in `api.go`, beside line 263:

```go
mux.Handle("POST /api/v1/writings/{id}/guide", liteOnly(a.guideWritingBlocks))
```

Keep `POST /outline/{oid}/guide` as the single-block **regenerate**.

Replace the `PERSISTS NOTHING` paragraph with:

```go
// PERSISTED, deliberately — and this reverses an earlier decision, so the reason
// matters. The old comment argued that storing model prose in the writing's own
// record would let a later reader mistake it for hers. That risk is real and is
// handled by WHERE it is stored: `guide` is a sibling of `role` (scaffold), not
// of `text` (hers) — the separation migration 0100 established for exactly this.
// Nothing composes a draft from the outline; writing_draft is built from
// writing_snippet.text alone, and TestComposeDraft_NeverIncludesGuideText pins it.
```

- [ ] **Step 6: Pin the reason with a test**

```go
// The guide is scaffold and must never reach her prose. This is the test the
// persistence decision rests on — if it ever fails, revert to not persisting.
func TestComposeDraft_NeverIncludesGuideText(t *testing.T) {
	// Seed an outline block whose stored guide contains a distinctive string,
	// and a snippet with her own text. Compose, then assert the guide string is
	// absent from the composed draft body.
}
```

Fill this in against the existing compose test setup in `writing_compose_test.go`; the assertion is `strings.Contains(draft.Body, guideMarker) == false`.

- [ ] **Step 7: Run the tests**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run "TestParseWritingGuide|TestComposeDraft" -timeout 1800s -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/api/writing_guide.go apps/api/internal/api/writing_guide_internal_test.go \
        apps/api/internal/api/writing_compose_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): guidance that is already there, and names the method

Batched for the whole skeleton on arrival, so it costs about what one 卡住了？
click cost and she never has to find a button to be taught.

The ？ filter is untouched and still applies to questions alone — the new prose
fields are about the block's job and about technique, never about her topic.
Method ids the model invents are dropped, which is how the terminology stays
ours.

Reverses the old 'PERSISTS NOTHING' decision and says why in the code: guide
sits beside role (scaffold), not beside text (hers), and a test pins that the
composed draft can never contain it."
```

---

## Task 5: B4 + B7 — comments that quote her own sentences

**Files:**
- Create: `apps/api/internal/api/writing_comment.go`
- Create: `apps/api/internal/api/writing_comment_test.go`
- Modify: `apps/api/internal/api/writing_compose.go` (`reviewWritingDraft` returns a comment)
- Modify: `apps/api/internal/api/api.go`

**Interfaces:**
- Produces: `type CommentPoint struct { Text string; Quote string }`, `type Comment struct { ID string; Scope string; SnippetID *string; Summary string; Points []CommentPoint; CreatedAt string }`; `validateCommentPoints(points []CommentPoint, source string) []CommentPoint`; routes `POST /api/v1/writings/{id}/snippets/{sid}/comment` and `GET /api/v1/writings/{id}/comments`; `POST /review` response becomes `{"comment": Comment}`.

- [ ] **Step 1: Write the failing validator test**

`apps/api/internal/api/writing_comment_test.go`:

```go
package api

import "testing"

// The quote IS the trace. A point whose quote is not literally in her text would
// send her to a sentence she never wrote — "specific, confident and wrong", the
// one failure mode this room refuses. Drop it; never guess.
func TestValidateCommentPoints_DropsQuotesSheNeverWrote(t *testing.T) {
	source := "我家楼下那条路就是这样，两排树掘得密密麻麻，长了几年也还是瘦瘦的一根杆。"
	in := []CommentPoint{
		{Text: "这个例子是过密，不是数量多。", Quote: "两排树掘得密密麻麻"},
		{Text: "这里缺出处。", Quote: "根据 2019 年的一项研究"},
		{Text: "空引用。", Quote: ""},
	}
	got := validateCommentPoints(in, source)
	if len(got) != 1 {
		t.Fatalf("kept %d points %v, want only the one whose quote is really in her text", len(got), got)
	}
	if got[0].Quote != "两排树掘得密密麻麻" {
		t.Fatalf("kept the wrong point: %+v", got[0])
	}
}

func TestValidateCommentPoints_KeepsEveryRealQuote(t *testing.T) {
	source := "第一句。第二句。第三句。"
	in := []CommentPoint{{Text: "a", Quote: "第一句。"}, {Text: "b", Quote: "第三句。"}}
	if got := validateCommentPoints(in, source); len(got) != 2 {
		t.Fatalf("kept %d, want 2", len(got))
	}
}
```

- [ ] **Step 2: Run and watch fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestValidateCommentPoints -timeout 1800s -v
```

Expected: FAIL — undefined `CommentPoint`, `validateCommentPoints`.

- [ ] **Step 3: Write the validator**

In `apps/api/internal/api/writing_comment.go`:

```go
// validateCommentPoints keeps only the points whose quote appears LITERALLY in
// the text being commented on.
//
// This is the same discipline pro applies to its evaluation report's references
// (ValidateRefs), and it is a structural guarantee rather than a request to the
// model: whatever it invents, a comment can only ever point at a sentence she
// actually wrote. Dropping is right and re-prompting is wrong — a fuzzy match
// that lands on the neighbouring sentence is worse than one fewer point.
func validateCommentPoints(points []CommentPoint, source string) []CommentPoint {
	out := make([]CommentPoint, 0, len(points))
	for _, p := range points {
		q := strings.TrimSpace(p.Quote)
		if q == "" || !strings.Contains(source, q) {
			continue
		}
		p.Quote = q
		out = append(out, p)
	}
	return out
}
```

- [ ] **Step 4: Write the generation handler**

`commentOnSnippet` for `POST /writings/{id}/snippets/{sid}/comment`, following `guideWritingBlock`'s shape exactly: `loadOwnedWritingAtom` → `HasEntitlement` → 150s timeout → `resolveEval` → `gateway.Collect` → `recordLiteLLMCall(..., "block_comment", ...)`. It asks for `{"summary":"…","points":[{"text":"…","quote":"…"}]}`, runs `validateCommentPoints` against the snippet text, and persists via `CreateWritingComment` with `scope='block'`.

The system prompt carries the four-part copy rule verbatim, plus: 每一条都必须引用她原文里的一句话，逐字照抄，不要改标点.

- [ ] **Step 5: Make `reviewWritingDraft` return the same shape**

`reviewWritingDraft` currently returns `{"feedback": "<prose>"}`, which renders once and evaporates. Change it to produce the same `{summary, points}` JSON, validate against the draft body, persist with `scope='draft'`, and respond `{"comment": Comment}`. Add `GET /api/v1/writings/{id}/comments`.

Routes in `api.go`:

```go
mux.Handle("POST /api/v1/writings/{id}/snippets/{sid}/comment", liteOnly(a.commentOnSnippet))
mux.Handle("GET /api/v1/writings/{id}/comments", liteOnly(a.listWritingComments))
```

- [ ] **Step 6: Run the tests**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run "TestValidateCommentPoints|TestReview" -timeout 1800s -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/writing_comment.go apps/api/internal/api/writing_comment_test.go \
        apps/api/internal/api/writing_compose.go apps/api/internal/api/api.go
git commit -m "feat(lite): comments that can only point at sentences she wrote

One summary line plus concrete points, each carrying a literal quote. A point
whose quote is not really in her text is dropped rather than rendered with a
best guess — a trace that lands on the neighbouring sentence is worse than one
fewer point.

请印记看看 now produces the same object and keeps it. Its critique was already
the best thing in the room and it was being thrown away on every navigation."
```

---

## Task 6: B3 — the Socratic sub-agent on one block

**Files:**
- Create: `apps/api/internal/api/writing_deepen.go`
- Create: `apps/api/internal/api/writing_deepen_test.go`
- Modify: `apps/api/internal/api/api.go`

**Interfaces:**
- Consumes: `ListAtomBlockMessages`, `AppendAtomBlockMessage` (Task 1); `vocab.For` (Task 2).
- Produces: `buildDeepenBrief(wr sqlc.Writing, outline []sqlc.WritingOutline, block sqlc.WritingOutline, snippetText string, guideQuestions []string) string`; routes `POST /api/v1/writings/{id}/outline/{oid}/deepen` and `GET /api/v1/writings/{id}/outline/{oid}/deepen`.

- [ ] **Step 1: Write the failing brief test**

```go
// The brief is what makes this a sub-agent rather than a second general chat:
// it carries the map and this block, and it deliberately does NOT carry the
// planning transcript. That exclusion is the whole cost saving and the whole
// reason it stays on topic, so it is pinned here.
func TestBuildDeepenBrief_CarriesTheMapAndBlockButNotTheTranscript(t *testing.T) {
	wr := sqlc.Writing{Title: "城市该不该大规模种行道树", Lang: "zh"}
	outline := []sqlc.WritingOutline{
		{Text: "该种，但要先定谁长期养", Role: "中心论点", Depth: 0, Position: 0},
		{Text: "维护年年花钱", Role: "一条理由", Depth: 1, Position: 1},
	}
	block := outline[1]

	brief := buildDeepenBrief(wr, outline, block, "更麻烦的是维护。", []string{"这笔钱谁出？"})

	for _, want := range []string{"城市该不该大规模种行道树", "该种，但要先定谁长期养", "维护年年花钱", "更麻烦的是维护。", "这笔钱谁出？"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief is missing %q", want)
		}
	}
	if strings.Contains(brief, "PLANNING_TRANSCRIPT_MARKER") {
		t.Error("brief carries the planning transcript; it must not")
	}
}
```

- [ ] **Step 2: Run and watch fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestBuildDeepenBrief -timeout 1800s -v
```

Expected: FAIL — undefined `buildDeepenBrief`.

- [ ] **Step 3: Write the brief builder and handler**

`writing_deepen.go` header comment:

```go
// 深入一层 — the same 印记, opened onto ONE block.
//
// The student meets no second character: the drawer is headed 「印记 · 关于这一块」,
// with no new name and no new avatar. The reading room spent a whole session
// collapsing two AIs into one; this must not quietly undo that.
//
// Under the hood it is a context-isolated sub-agent, spawned server-side and
// briefed with: the title, the whole map (so it can see where this block sits),
// this block's role/heading/text, and the guide questions already shown so it
// does not re-ask them. It is NOT given the planning transcript — that exclusion
// is what keeps it focused and cheap, and TestBuildDeepenBrief pins it.
//
// WHAT IS GUARANTEED, HONESTLY: this endpoint has no write path to
// writing_outline or writing_snippet, so it CANNOT author her outline or her
// prose — that part is structural. But it is a free-form chat, so the guide
// box's "output must be questions" type guarantee does not apply here; that
// rests on the prompt, exactly as the main 印记's chat already does. Giving it
// the vocab library instead of letting it compose illustrations is what narrows
// the surface. Known limit, accepted deliberately.
```

The POST handler: load atom → entitlement → read `ListAtomBlockMessages(atomID, oid)` for history → `NextAtomMessageSeq` → append her turn with `AppendAtomBlockMessage` → `resolveEval` → `gateway.Collect` → `recordLiteLLMCall(..., "block_deepen", ...)` → append the reply with `AppendAtomBlockMessage`. The GET returns the block's thread.

System prompt: the four-part copy rule verbatim, plus 你是在帮她想这一块，用苏格拉底式的追问：问出她已经知道但还没说出来的东西。绝不替她写句子。你可以从【可用的方法】里举例子——那些例子讲的都是别的题目，不是她的。

Routes:

```go
mux.Handle("POST /api/v1/writings/{id}/outline/{oid}/deepen", liteOnly(a.deepenWritingBlock))
mux.Handle("GET /api/v1/writings/{id}/outline/{oid}/deepen", liteOnly(a.getWritingBlockThread))
```

- [ ] **Step 4: Add the isolation test**

```go
// A deepen turn must be invisible to the room's own thread — the top risk of
// migration 0102 expressed at the endpoint rather than at the query.
func TestDeepenTurn_DoesNotEnterTheRoomThread(t *testing.T) {
	// POST a deepen turn, then GET /writings/{id}/messages and assert the deepen
	// turn's content is absent.
}
```

Fill in against the existing handler-test setup in `writing_turn_test.go`.

- [ ] **Step 5: Run the tests**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run "TestBuildDeepenBrief|TestDeepenTurn" -timeout 1800s -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/writing_deepen.go apps/api/internal/api/writing_deepen_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): 深入一层 — the same 印记, briefed on one block

Spawned server-side with the title, the map, this block and the questions
already asked. Deliberately without the planning transcript, which is what
keeps it focused and cheap, and there is a test that says so.

The student meets no second character. The honest limit is written into the
file: it cannot touch her outline or her prose, but its chat has no
output-type filter the way the guide box does."
```

---

## Task 7: B2 — English patterns replace the model paragraph

**Files:**
- Modify: `apps/api/internal/api/writing_snippets.go` (remove `generateWritingSnippetExemplar`)
- Modify: `apps/api/internal/api/api.go` (remove line 266's route)
- Modify: `apps/lite-web/src/api/writingRoom.ts` (remove `WritingExemplar`, `generateWritingSnippetExemplar`)

- [ ] **Step 1: Delete the exemplar endpoint and its route**

Remove the handler, the route at `api.go:266`, and any now-unused prompt constant. Delete its tests.

- [ ] **Step 2: Verify nothing references it**

```bash
grep -rn "exemplar\|Exemplar" apps/api/internal apps/lite-web/src apps/lite-web/e2e apps/lite-web/test
```

Expected: no matches. Any leftover is a compile or test break waiting to happen.

- [ ] **Step 3: Build and test**

```bash
cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test ./internal/api/ -timeout 1800s
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/api/writing_snippets.go apps/api/internal/api/api.go apps/lite-web/src/api/writingRoom.ts
git commit -m "refactor(lite): drop 示范段落, the paragraph she could retype

It was a model paragraph about HER thesis. The component around it was built
so nothing could write to her textarea, but the content itself was always the
back door. English support moves to sentence frames with blanks, which she
cannot paste as an argument — guarantee by type, the same discipline as the ？
filter, and stronger than what it replaces."
```

---

## Task 8: `mk-prose` and the shared prose typography

**Files:**
- Modify: `apps/web/tailwind.config.ts` (`fontSize` block, lines 55–64)
- Create: `apps/lite-web/src/writings/ProseSurface.tsx`
- Test: `apps/lite-web/test/proseSurface.test.tsx`

**Interfaces:**
- Produces: `PROSE_TYPOGRAPHY` (a frozen style object), and `<ProseSurface value onChange highlight placeholder />` where `highlight?: string | null` is a literal substring to mark.

- [ ] **Step 1: Add the token**

In `apps/web/tailwind.config.ts`, inside `fontSize`, after `"mk-body-lg"`:

```ts
        // Composition surfaces only — the writing page, and the mirrored layer
        // that highlights inside it. 14px mk-body is chrome type; a page needs
        // page type. 1.9 leading matters more for Chinese than for Latin: the
        // glyphs are dense and full-height.
        "mk-prose":["17px",{lineHeight:"1.9"}],
```

This file is shared with pro. Adding a `fontSize` key is purely additive and cannot change any existing class, but pro's full suite runs in Task 13.

- [ ] **Step 2: Write the failing test**

`apps/lite-web/test/proseSurface.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProseSurface, PROSE_TYPOGRAPHY } from "../src/writings/ProseSurface";

describe("ProseSurface", () => {
  // The highlight layer sits BEHIND the textarea and must be typeset
  // identically. If the two ever differ by a pixel the highlight lands on the
  // wrong line, so they read from one frozen object rather than two style props.
  it("typesets the textarea and the highlight layer from the same object", () => {
    const { container } = render(
      <ProseSurface value={"第一段。\n\n第二段。"} onChange={() => {}} highlight="第二段。" />,
    );
    const ta = container.querySelector("textarea")!;
    const layer = container.querySelector("[data-prose-layer]") as HTMLElement;
    for (const k of ["fontSize", "lineHeight", "letterSpacing", "padding"] as const) {
      expect(layer.style[k]).toBe(ta.style[k]);
      expect(layer.style[k]).toBe(String(PROSE_TYPOGRAPHY[k]));
    }
  });

  it("marks the highlighted substring and nothing else", () => {
    render(<ProseSurface value={"第一段。第二段。"} onChange={() => {}} highlight="第二段。" />);
    expect(screen.getByText("第二段。", { selector: "mark" })).toBeTruthy();
  });

  it("renders no mark when there is no highlight", () => {
    const { container } = render(<ProseSurface value="第一段。" onChange={() => {}} highlight={null} />);
    expect(container.querySelector("mark")).toBeNull();
  });
});
```

- [ ] **Step 3: Run and watch fail**

```bash
cd apps/lite-web && npx vitest run test/proseSurface.test.tsx
```

Expected: FAIL — module not found.

- [ ] **Step 4: Write `ProseSurface`**

Key requirements, all from spec §8.1:

```tsx
/**
 * ProseSurface — the writing page.
 *
 * A textarea, not a contentEditable. A textarea cannot space paragraphs
 * differently from lines; contentEditable can, and is also the first step
 * toward the document editor 铁律 rules out. At 1.9 leading a blank line
 * already yields a ~32px gap, which is how iA Writer and Bear look, so the
 * limitation is not one she will perceive.
 *
 * Because it IS a textarea, a comment cannot highlight inside it. The mirrored
 * layer behind it renders the same text with the same metrics and marks the
 * quoted span. 🔑 Both read PROSE_TYPOGRAPHY — define the metrics twice and
 * they drift, and the highlight lands on the wrong line.
 */
export const PROSE_TYPOGRAPHY = Object.freeze({
  fontSize: "17px",
  lineHeight: "1.9",
  letterSpacing: "0",
  padding: "48px 32px 40vh",
});
```

The container is `relative`, `max-w-[68ch]`, `mx-auto`, on `bg-mk-paper`. The layer is absolutely positioned, `whitespace-pre-wrap`, `aria-hidden`, `pointer-events-none`, `data-prose-layer`. The textarea is transparent-background, `resize-none`, **no border and no focus ring**, caret and selection in accent. Scroll positions are synced so the layer tracks the textarea.

- [ ] **Step 5: Run the tests**

```bash
cd apps/lite-web && npx vitest run test/proseSurface.test.tsx && npx tsc --noEmit -p tsconfig.json
```

Expected: PASS, clean typecheck.

- [ ] **Step 6: Commit**

```bash
git add apps/web/tailwind.config.ts apps/lite-web/src/writings/ProseSurface.tsx apps/lite-web/test/proseSurface.test.tsx
git commit -m "feat(lite): a page to write on, and one place its metrics live

17px/1.9 at 68ch — which lands at ~34 Chinese characters and ~68 Latin ones
per line, so one measure serves both languages. No border, no focus ring: a
ring around a whole page is chrome logic applied to a page.

The highlight layer and the textarea read the same frozen object, because two
copies of these metrics drift and a drifted highlight points at the wrong
sentence."
```

---

## Task 9: The guide box teaches

**Files:**
- Modify: `apps/lite-web/src/writings/GuideBox.tsx`
- Create: `apps/lite-web/src/writings/VocabExamples.tsx`
- Modify: `apps/lite-web/src/api/writingRoom.ts` (`WritingBlockGuide` grows `job` and `methods`)
- Test: `apps/lite-web/test/guideBox.test.tsx`

- [ ] **Step 1: Write the failing test**

```tsx
it("renders all four parts, and questions at reading size", () => {
  render(
    <GuideBox
      guide={{
        job: "这一段要让读者相信「便宜」这个说法不成立。",
        methods: [{ id: "point_contrast", name: "正反", definition: "一正一反两个例子放在一起。", examples: [{ topic: "两个菜市场", text: "东街留了装卸区…" }], patterns: [] }],
        questions: ["你见过哪条街上的树长不开？", "这跟成本有什么关系？"],
      }}
      onDismiss={() => {}}
      onDeepen={() => {}}
    />,
  );
  expect(screen.getByText(/这一段要做的事/)).toBeTruthy();
  expect(screen.getByText("正反")).toBeTruthy();
  expect(screen.getByText(/一正一反两个例子/)).toBeTruthy();
  expect(screen.getByRole("button", { name: /看几个例子/ })).toBeTruthy();
  expect(screen.getByRole("button", { name: /深入一层/ })).toBeTruthy();
  // The old box put everything at mk-body 14px, which is what made it unreadable.
  expect(screen.getByText("你见过哪条街上的树长不开？").className).toContain("text-mk-body-lg");
});

it("shows the borrowed example only after she asks, with its topic named", async () => {
  // click 看几个例子 → the example text appears AND its topic is labelled, so it
  // can never read as a suggestion about her own piece.
});
```

- [ ] **Step 2: Run and watch fail**, then **Step 3: implement**, then **Step 4: run green**.

```bash
cd apps/lite-web && npx vitest run test/guideBox.test.tsx
```

Layout per spec §5.4: four visually separated parts, one idea per line, questions at `text-mk-body-lg` with `gap-3`, method names as readable inline emphasis — **not** 11px `mk-label` chips. `VocabExamples` labels each example with its `topic` so it reads as borrowed.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/writings/GuideBox.tsx apps/lite-web/src/writings/VocabExamples.tsx \
        apps/lite-web/src/api/writingRoom.ts apps/lite-web/test/guideBox.test.tsx
git commit -m "feat(lite): the guide box says why, names the method, offers examples

Four parts instead of a bare question list, and the questions are finally at
reading size. Examples carry the topic they are about, so a borrowed example
can never be mistaken for a suggestion about her own piece."
```

---

## Task 10: Comments in the UI, and the trace

**Files:**
- Create: `apps/lite-web/src/writings/CommentPanel.tsx`
- Modify: `apps/lite-web/src/api/writingRoom.ts`
- Test: `apps/lite-web/test/commentPanel.test.tsx`

- [ ] **Step 1: Write the failing test**

```tsx
it("traces a point back to the sentence it is about", async () => {
  const onTrace = vi.fn();
  render(<CommentPanel comment={{ id: "c1", scope: "draft", summary: "整体清楚。", points: [{ text: "这个例子是过密，不是数量多。", quote: "两排树掘得密密麻麻" }], createdAt: "" }} onTrace={onTrace} />);
  await userEvent.click(screen.getByRole("button", { name: /这个例子是过密/ }));
  expect(onTrace).toHaveBeenCalledWith("两排树掘得密密麻麻");
});

it("renders the summary before the points", () => { /* order assertion */ });
```

- [ ] **Steps 2–4: fail → implement → green**

```bash
cd apps/lite-web && npx vitest run test/commentPanel.test.tsx
```

`onTrace(quote)` is lifted to the stage, which passes it to `ProseSurface`'s `highlight`.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/writings/CommentPanel.tsx apps/lite-web/src/api/writingRoom.ts apps/lite-web/test/commentPanel.test.tsx
git commit -m "feat(lite): click a comment, land on the sentence it means"
```

---

## Task 11: 成稿 becomes a page; 段落 gets guidance and comments

**Files:**
- Modify: `apps/lite-web/src/writings/ComposeStage.tsx`
- Modify: `apps/lite-web/src/writings/SnippetsStage.tsx`
- Create: `apps/lite-web/src/writings/DeepenDrawer.tsx`
- Create: `apps/lite-web/src/writings/EditableTitle.tsx`
- Modify: `apps/lite-web/src/writings/WritingRoomHost.tsx`, `MindMap.tsx`
- Test: `apps/lite-web/test/composeStage.test.tsx`, `apps/lite-web/test/mindMap.test.tsx`

- [ ] **Step 1: Write the failing tests**

```tsx
// Spec §8.1: re-assembling over her edits is the silent-data-loss shape this
// room has already been bitten by once (the snippet position bug).
it("refuses to re-assemble over an edited draft without confirming", async () => {
  render(<ComposeStage {...props} initialBody="她自己改过的正文" />);
  await userEvent.type(screen.getByRole("textbox"), "又加了一句");
  await userEvent.click(screen.getByRole("button", { name: "从段落重新拼一次" }));
  expect(screen.getByRole("dialog")).toHaveTextContent(/会覆盖/);
});

it("shows the draft on arrival without needing 拼初稿", () => {
  render(<ComposeStage {...props} initialBody="拼好的正文" />);
  expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("拼好的正文");
});

// Spec §4: B0 puts several nodes at depth 0. MindMap has only ever seen one.
it("renders every depth-0 node, in document order", () => {
  render(<MindMap outline={[
    { id: "a", text: "夏天路上晒得受不了", role: "开头", depth: 0, position: 0 },
    { id: "b", text: "该种，但要先定谁长期养", role: "中心论点", depth: 0, position: 1 },
    { id: "c", text: "谁来养这件事得先定", role: "结尾", depth: 0, position: 2 },
  ]} />);
  expect(screen.getByText("夏天路上晒得受不了")).toBeTruthy();
  expect(screen.getByText("该种，但要先定谁长期养")).toBeTruthy();
  expect(screen.getByText("谁来养这件事得先定")).toBeTruthy();
});
```

- [ ] **Step 2: Run and watch them fail**

```bash
cd apps/lite-web && npx vitest run test/composeStage.test.tsx test/mindMap.test.tsx
```

- [ ] **Step 3: Rebuild `ComposeStage`**

Two panes: left `ProseSurface`; right rail with her 段落 blocks and `CommentPanel`. The draft is fetched and shown on arrival. `从段落拼出初稿` becomes `从段落重新拼一次` and, when the body differs from what was last assembled, opens a confirm dialog naming what it will overwrite. `请印记看看` renders into the rail. Autosave becomes a fixed, non-reflowing marker. Placeholder is warm rather than plumbing.

- [ ] **Step 4: Wire 段落 and the drawer**

`SnippetsStage` paints the stored guide on first render, keeps `卡住了？` as regenerate, adds 请印记看看这一段 (Task 5's endpoint) and 深入一层 (opens `DeepenDrawer`). Remove the exemplar UI. `EditableTitle` is placed in the room header and calls the existing `PATCH /api/v1/writings/{id}`.

- [ ] **Step 5: Fix `MindMap` for multiple roots**

Render all `depth === 0` nodes as document-ordered siblings.

- [ ] **Step 6: Run tests and typecheck**

```bash
cd apps/lite-web && npx vitest run && npx tsc --noEmit -p tsconfig.json
```

Expected: all green.

- [ ] **Step 7: Commit**

```bash
git add apps/lite-web/src/writings apps/lite-web/test
git commit -m "feat(lite): 成稿 is a page, 段落 teaches, the map has more than one root

The draft is simply there when she arrives. Re-assembling asks first when she
has edited, because doing it silently is how she would lose an afternoon and
only notice much later.

MindMap had only ever rendered one depth-0 node; an opening and a landing are
siblings of the thesis, so it now renders all of them in document order."
```

---

## Task 12: The e2e walk

The August notes record this walk breaking **twice in one day** while still looking green — both times asserting against deleted endpoints. It is updated here, in the same branch, never after.

**Files:**
- Modify: `apps/lite-web/e2e/writing-walk.spec.ts`

- [ ] **Step 1: Remove assertions against deleted surfaces**

```bash
grep -n "exemplar\|示范段落\|从段落拼出初稿\|请印记看看" apps/lite-web/e2e/writing-walk.spec.ts
```

Update each hit: 示范段落 is gone, 从段落拼出初稿 is now 从段落重新拼一次, and 请印记看看's result is a comment in the rail rather than prose under the buttons.

- [ ] **Step 2: Add coverage for the new behaviour**

```ts
// Guidance is present without being asked for — the whole point of B1. If this
// ever needs a click to pass, the regression is exactly the one we fixed.
await expect(page.getByText("这一段要做的事").first()).toBeVisible({ timeout: 60_000 });

// A comment traces back to a real sentence in her own draft.
await page.getByRole("button", { name: "请印记看看" }).click();
const point = page.locator("[data-comment-point]").first();
await expect(point).toBeVisible({ timeout: 180_000 });
await point.click();
await expect(page.locator("[data-prose-layer] mark")).toBeVisible();
```

**Timeout trap** (learned on 2026-08-28): never wait on an absence that is already true — `expect(x).toHaveCount(0)` resolves instantly before the thing has even mounted and silently passes the whole model turn by. Always wait on the arrival.

- [ ] **Step 3: Type-check and list**

e2e is **not** covered by the normal typecheck (`tsconfig.json` includes only `["src","test"]`):

```bash
cd apps/lite-web && npx tsc --noEmit e2e/writing-walk.spec.ts && npx playwright test --list
```

- [ ] **Step 4: Run the walk against a real stack**

```bash
cd apps/lite-web && ./e2e/run-stack.sh
```

Expected: green. Real model calls, so allow several minutes.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/e2e/writing-walk.spec.ts
git commit -m "test(lite): walk the room that now teaches

Guidance asserted as present on arrival rather than after a click, since
needing the click is precisely the regression. Waits key on arrival, never on
an absence that is already true."
```

---

## Task 13: Full verification

- [ ] **Step 1: Go**

```bash
cd apps/api && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -timeout 1800s
```

- [ ] **Step 2: Lite**

```bash
cd apps/lite-web && npx vitest run && npx tsc --noEmit -p tsconfig.json
```

- [ ] **Step 3: Pro — the standing rule, because `tailwind.config.ts` is shared**

```bash
cd apps/web && npx vitest run && npx tsc --noEmit -p tsconfig.json && npm run build
```

Expected: 1291 tests green, clean typecheck, clean bundle.

- [ ] **Step 4: Confirm the scope risk is actually closed**

```bash
grep -rn "ListAtomMessages\|ListAtomBlockMessages" apps/api/internal/api/
```

Every `ListAtomMessages` call site must be one that means *the room's own thread*. Any new caller wanting block turns must use `ListAtomBlockMessages`.

- [ ] **Step 5: Update the backlog**

Tick B0–B7 in `docs/2026-08-28-lite-edition-feedback-backlog.md` and link this plan and the spec from the **B** heading.

- [ ] **Step 6: Commit**

```bash
git add docs/2026-08-28-lite-edition-feedback-backlog.md
git commit -m "docs(lite): sub-project B is done; A and C+D are still open"
```

---

## Self-review notes

**Spec coverage.** B0 → Task 3. B1 → Tasks 4, 9. B2 → Tasks 2, 7, 9. B3 → Tasks 1, 6, 11. B4 → Tasks 5, 10. B5 → Tasks 8, 11. B6 → Task 11 (backend already exists: `PATCH /api/v1/writings/{id}` → `renameWriting`, `api.go:253`). B7 → Tasks 5, 10, 11. Data model §10 → Task 1. Risks §11.1 → Tasks 1, 13. §11.2 → Task 11. §11.3 → Task 12. §11.4 → Task 8. §11.5 → Task 13.

**Conflict found and resolved during planning.** `guideWritingBlock`'s doc comment asserts *"PERSISTS NOTHING"* with a stated reason. The spec persists guidance. Task 4 resolves this explicitly — persistence sits beside `role` (scaffold), not `text` (hers), and a test pins that a composed draft can never contain guide text — and rewrites the stale comment.

**Design decision made during planning.** The spec called for auditing all seven `ListAtomMessages` call sites. The plan instead makes the query itself scope-safe, so all seven are correct untouched. This is strictly safer: an audit is correct until someone forgets, and a safe default cannot be forgotten.

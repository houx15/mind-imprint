# Lite 阅读报告 + 写作报告 — Design Spec

> **Sub-projects C+D** of `docs/2026-08-28-lite-edition-feedback-backlog.md`, and the
> last of the four. B (写作房间) shipped `250bce3d`/`cc0eaf19`; A (阅读深度) shipped
> `cbbf055e`.
>
> **One spec, two outputs.** They share the stats pipeline, the shining-moments pass,
> the poster renderer, the image export and the public share route. Splitting them
> would build all of that twice.
>
> **Binding rulings** R1–R4 are recorded in the backlog and reproduced where they bite.

## What the user asked for

> **Reading report:** *"colorful, clear, not verbose, but interesting and appealing.
> with reading time (min), AI chat turns, reading notes count. one thing to take away…
> my reading notes can export a picture, which should not be verbose, but be good
> looking. like lark meeting notes they would conclude some 金句 made by the speaker.
> if we can show some the children's shining moments on the report, it can be
> exported, with students' name and effort be noted. it would have fantastic effect!
> if students agree, can scan a qrcode to show their reading notes (be made public)"*
>
> **Writing report:** *"also, need to be appealing, let people have 分享欲, show
> students' shining moments and their endeavor. words written, time spent, ai chat
> turns. their most shining points. their gains. etc. if students agree to share, can
> scan a code to view their writings (be made public)"*

Today both terminal screens render a literal placeholder — 「这次阅读的报告还在路上。」
/ 「这篇写作的报告还在路上。」 — and 「看报告」 in both history drawers is a dead label
pointing at exactly that.

## Two discoveries that change the design

**1. There is no time tracking at all.** Nothing in the lite substrate records duration
— no heartbeat, no session table, no `active_seconds`. Only point-in-time stamps
(`atom.created_at`, `atom.last_activity_at`, and `created_at` on messages/cards/
annotations/snippets/comments). The user asked for a *number of minutes* on both
reports, so one has to be produced. See **Time** below.

**2. Half of what looks like the student's writing is not hers.** This is the load-bearing
fact for R4, and it is not obvious from the UI:

| Field | Whose words |
|---|---|
| `reading_takeaway.text` | **hers** (plain PUT, no AI) |
| `atom_annotation.note` | **hers** |
| `atom_annotation.quote` | the **article's** — text she highlighted |
| `atom_card.field_values` | **hers** (her typed answers inside a lens) |
| `atom_card.framework_fill.finding` | **AI's** — this is the 发现 shown at finalize |
| `atom_card.anchors[].quote` | the **article's** — the sentence she picked |
| `reading_question.text` | **AI's** |
| `reading_block_note.body` | **AI's** — about the article |
| `writing_snippet.text` / `writing_draft.body` / `writing_outline.text` | **hers** |
| `writing_outline.role` / `.guide` / `writing_comment.*` | **AI's** |
| `atom_message` where `role='student'` | **hers — but see below** |

**The trap inside the last row.** Since sub-project A, a student message carries the
sentences she pointed at in the article as leading `> ` blockquote lines. Those are the
**article's** words sitting inside a row whose role says `student`. A 金句 pass that
took `atom_message.content` at face value would put the author's sentence under the
student's name on an exportable, shareable picture — the exact thing R4 forbids.

**So: the corpus is built by stripping every leading-`>` line from student messages.**
This is not a nicety; it is R4's enforcement point.

---

## Architecture

One table, one generator, two projections. Following pro's `evaluation_report` shape
(deterministic facts + LLM prose, best-effort degradation, three-state envelope) but
much smaller.

### Storage — migration `0105_lite_reports.sql`

```sql
ALTER TABLE atom ADD COLUMN active_seconds integer NOT NULL DEFAULT 0;

CREATE TABLE atom_report (
  atom_id      uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('reading','writing')),
  report       jsonb NOT NULL,
  share_token  text UNIQUE,
  shared_at    timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_report_share_idx ON atom_report (share_token) WHERE share_token IS NOT NULL;
```

`share_token` NULL means not shared. Revoking sets it back to NULL, which is what makes
revocation real rather than cosmetic (R1) — the public route resolves by token, so a
revoked link resolves to nothing on the very next request.

### The report envelope (v1)

Both kinds produce the same shape; sections that do not apply are absent, never empty.

```ts
type LiteReport = {
  version: 1;
  kind: "reading" | "writing";
  title: string;
  studentName: string;      // for the exported picture — the user asked for it
  finishedAt: string;       // ISO
  stats: { key: string; label: string; value: number; unit: string }[];
  moments: { quote: string; where: string }[];   // HER OWN WORDS ONLY (R4)
  keep: { label: string; text: string } | null;  // C2 · 这次最值得记住的
  gains: string[];                                // AI prose, 2–4 short lines
};
```

### Generation

`GET /readings/{id}/report` and `GET /writings/{id}/report`, generate-if-absent:

1. ownership + `HasEntitlement`;
2. **the atom must be finished** — an unfinished one returns `{"report": null}`, generates
   nothing and stamps nothing (the lesson A paid for: a generate-if-absent endpoint
   reachable early burns its one generation on partial data, permanently);
3. cheap read: a stored report is returned as-is;
4. otherwise `pg_advisory_xact_lock`, **re-check inside the transaction**, then the
   deterministic half, then **one** flagship call, then store and commit.

The lock is taken **before** the provider call. This project has now paid twice for one
model call; the ordering is not negotiable.

**Stats are deterministic.** They are computed in Go from rows, never asked of a model.

**One model call, best-effort.** It produces `moments`, `keep` and `gains` together —
they are one editorial judgment over one corpus, and three calls would cost three times
as much to no benefit. If it fails or returns nothing usable, the report still stores and
renders with stats and her own 收获; the prose sections are simply absent. A report is
never blocked on prose.

### R4's enforcement, structurally

The corpus handed to the model is assembled **only** from the "hers" rows above, with
leading-`>` lines stripped from student messages. Then every returned `quote` is kept
only if it is a **literal substring of that corpus**. Same validator shape as the writing
room's comment quotes and A's question anchors, for the same reason: a guarantee you can
check beats one you asked for.

If no moment survives, the section is absent. A thin session shows a thin report; that is
the honest outcome and the design must look right when it happens.

### R2 — who picks the shining moments

**印记 picks them and shows them.** The user overruled the softer "propose, then she
confirms". What keeps this inside 铁律②: what is displayed is **her own words**,
selected — never a score, a rank, a streak, a badge, or a comparison to anyone. Selection
is editorial, not evaluative. And nothing leaves the platform without her explicit
opt-in (R1), so no publication happens without her acting.

---

## Time

No duration data exists, so this spec creates the smallest honest thing that does.

**Primary — a heartbeat.** `POST /readings/{id}/heartbeat` and the writing twin post
`{seconds}` while the room is mounted and `document.visibilityState === "visible"`.
The server adds it to `atom.active_seconds`, **clamping each call to at most 120s** so a
tampered or delayed client cannot inflate the number. The client posts every 60s and
stops when the tab is hidden — so an abandoned open tab does not accrue an hour of
"focus".

**Fallback — capped-gap, for every session that predates this.** When
`active_seconds = 0`, estimate from the event trail (message / card / annotation /
snippet / comment timestamps) by summing consecutive gaps, **capping each gap at 5
minutes**. Without this, every reading and writing finished before this ships — including
the demo account's — would display 「0 分钟」, which reads as broken in exactly the walk
someone would run first.

**Honesty in the label.** It is 专注时长, not 总时长, and the report never claims
precision it does not have. Silent reading before her first action is invisible to both
methods; that is a real limit, stated here rather than papered over.

## Stats, per kind

**Reading** — 专注时长 (min) · 和印记聊了 N 轮 · 笔记 N 条 · 读完 N 步
`N 轮` counts `atom_message` rows with `role='student'` and `block_id IS NULL`.
`笔记` counts `atom_annotation` rows (her margin notes), not highlights without a note.

**Writing** — 写了 N 字 · 专注时长 (min) · 和印记聊了 N 轮 · 改了 N 段
`N 字` counts **characters for `lang='zh'` and whitespace-separated words for `lang='en'`**.
This project has already shipped a word count that counted characters for English and was
~5× wrong; the counter must branch on `writing.lang`, and it must be shared with the
client's existing `countWords` rather than re-implemented beside it.
`N 轮` counts student messages across the main thread **and** the 深入一层 block threads,
since both are conversations she had.

---

## The two surfaces

### The report page

Replaces the placeholder on both finished screens, and makes 「看报告」 in both history
drawers真. Requirements from the user: **colorful, clear, not verbose, appealing** — the
stated goal is 分享欲.

Direction: a warm poster, not a dashboard. Big numerals for the four stats; 金句 as
pull-quotes with generous space, in the article/essay's own voice-size rather than
shrunk into a list; her 收获 in her own words, quoted as hers; 3–4 short gain lines. No
tables, no dense paragraphs, no ladder of scores.

**It must look right when thin.** A student who wrote two sentences gets a report with
two stats and one quote, and it must still look composed rather than broken. Every
section is independently absent-able.

**No score, no grade, no rank, no comparison, no streak** (铁律②). This is a record of
what she did, not a verdict on it.

### The exported picture

A dedicated poster component rendered offscreen at a fixed size (1080×1440), exported to
PNG via `html-to-image` and downloaded. It carries her name, the title, the date, the
stats and up to three 金句 — nothing else. The user's benchmark: *"like lark meeting
notes… with students' name and effort be noted."*

Constraints:
- **System font stack only.** The export rasterizes through an SVG `foreignObject`,
  which does not load external fonts; a web font would silently fall back and ship a
  differently-typeset picture than the one she previewed.
- Tailwind `mk-*` tokens are bare CSS custom properties, so **alpha syntax
  (`bg-mk-x/30`) emits no CSS at all**. Use `color-mix()` or explicit tokens, or the
  poster exports with invisible backgrounds.
- The export must be verified by an assertion on the produced data URL (non-trivial
  length, correct mime), not by "the button did not throw".

### The public share page

`POST …/report/share` mints an unguessable token (32 hex from `crypto/rand` — never the
atom id); `DELETE …/report/share` revokes by setting it NULL. The report page shows the
link and a QR code (generated client-side from the URL) once sharing is on, and a plain
「停止分享」 that takes it down.

`GET /api/v1/public/reports/{token}` is registered **without** `protected`/`liteOnly`,
following the existing bare-`mux.Handle`-plus-internal-gate shape used by the admin-key
routes. It returns the report JSON **and nothing else**: no account, no transcript, no
article, no draft body, no ids that address anything else she owns. A revoked or unknown
token is a plain 404.

The public page lives at `/s/:token` and **must render before `LiteApp` calls
`getMe()`** — today the auth gate runs before any route is inspected, so an
unauthenticated visitor would be shown the sign-in screen instead of the shared page.
The check happens on `window.location.pathname` before boot.

**R1, restated as it binds here:** anyone with the link, no login, no expiry, revocable
at any time, her name may appear. She is a minor, so the compensating controls are that
the token is unguessable, revocation is immediate and total, and the payload is the
report and only the report.

---

## Non-goals

- **No teacher/parent view, no dashboard, no cohort anything.** One student, one session.
- **No score, grade, rank, badge, streak, or comparison** (铁律②).
- **No PDF.** Pro has one; the user asked for a picture.
- **No editing of the report.** It is a record.
- **No re-generation button.** First open wins, as pro's report does.
- **No model re-routing.** Still deferred to its own benchmarked session.

## Testing

- **Go, unit:** the corpus builder strips leading-`>` lines from student messages
  (R4's enforcement point — its own test); the moment validator drops a quote that is not
  a literal substring; the word counter branches on `lang` and matches the client's
  `countWords` on shared fixtures; the capped-gap estimator caps a long gap and sums
  short ones; heartbeat clamps a 10-minute claim to 120s.
- **Go, integration:** an unfinished atom generates nothing and stamps nothing; two
  concurrent first-opens cost exactly one provider call (assert the fake provider's
  count, never timing); a revoked token 404s; the public payload contains no field
  outside the report envelope.
- **lite-web:** the report renders with one stat and no moments without breaking; the
  QR/link only appear when sharing is on; 停止分享 removes them; the poster export
  produces a PNG data URL of non-trivial length.
- **e2e:** finish a reading → report appears → share → open `/s/:token` **in a fresh
  context with no session** and see it → revoke → the same URL 404s.
- **Regression:** pro's full suite stays green; `apps/web/` changes stay additive.

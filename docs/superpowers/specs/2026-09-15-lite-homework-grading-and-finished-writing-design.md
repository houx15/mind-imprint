# Lite · finished writing, AI 批改, homework materials — design

Date: 2026-09-15 · Status: approved in chat by the owner, pending spec review
Edition: lite only (`apps/lite-web`, lite routes in `apps/api`). Pro must not change behaviour
(see memory `lite-must-not-break-pro`: shared `queries/`, `migrations/`, sqlc dirs).

Exploration notes: `.superpowers/tmp/explore-homework-grading.md` (main @ `dca31680`).

## 1. What the owner asked for

1. Teachers give reading homework: upload a document, pick from our library, or give
   personalised reading (the system picks per student).
2. 一键AI批改: AI drafts feedback on students' writings, the teacher reviews and edits it,
   then sends it to the students.
3. The student's finished-writing page is incomplete: it is a narrow phone-width column;
   students cannot edit after 完成 (they should, except a submitted homework past its
   deadline); there is no version history; there is nowhere to read teacher feedback.

## 2. Owner decisions (2026-09-15)

| Question | Decision |
|---|---|
| What 批改 produces | Teacher-style: a rubric grade per dimension + overall grade, an overall comment, 3–5 points (strengths and issues), each point quoting her sentence, issues carry an action. AI never rewrites her sentences. Teacher edits, deletes, adds, then sends. |
| Editing a submitted homework before the deadline | Stays 已提交. Pressing 完成 again creates a new submitted version. Teacher sees and grades the latest submitted version; older versions stay in history. |
| After the deadline | Read-only, but the teacher can 退回修改 one student with a new due date. |
| Rubric | Built-in default per language; the teacher can adjust it per homework. |
| Personalised reading | System picks one library article per student; the teacher reviews the list and can swap rows before assigning. |
| AI attribution on the student side | Show 「由 AI 起草，老师审阅后发送」 on every teacher 批改. |
| Build order | Part A → Part B → Part C, one plan each. |

## 3. What exists today (the parts this design builds on)

- 作业: `lite_assignment` + `lite_assignment_recipient` (migration 0149). Reading sources
  `library` / `url` / `text` (≤50000 chars) in `liteassign/payload.go`. Status is derived,
  never stored: `liteassign/status.go` → not_started / in_progress / done / done_late /
  overdue, from `started_at`, the item's `finished_at` and `due_at`. Nothing locks at `due_at`.
- Writing: `writing.status` active|finished, `finished_at` set once; `writing_draft` is one
  row overwritten by autosave. No history.
- Finished page: `WritingRoomHost.tsx` → `FinishedWritingPanel` → `ReportPanel` →
  `reports/ArticleView.tsx:58` `max-w-[34rem]` (the narrow column).
- Server lock: `loadOwnedAtom` (`readings.go`) returns 403 `writing_finished` on every
  non-GET request to a finished writing.
- AI review: `POST /writings/{id}/review` → `collectWritingComment` + `validateCommentPoints`
  (quotes must be literal substrings), class `review`, stored in `writing_comment`
  (no author, no status, visible to the student at once).
- Teacher: `authTeacherStudent`; item page `teacher/ItemPage.tsx` (bug: it renders only
  string points, so AI comment points never show).
- Library: `library/articles.json` (48 × 5 tiers), recommendation `library.go` (needs the
  current request's user today), `SuggestTier`. Document extraction
  `POST /documents/extract` (.pdf .docx .txt .md).

---

## Part A · Finished-writing page (plan A)

### A1. Versions

New table `writing_version`:

| column | type | note |
|---|---|---|
| id | uuid pk | |
| atom_id | uuid fk atom | |
| number | int | 1, 2, 3 … per atom, `UNIQUE(atom_id, number)` |
| title | text | title at submission |
| body | text | draft body at submission |
| word_count | int | same counting function the writing room uses |
| submitted_at | timestamptz | |

- `POST /writings/{id}/finish` inserts the next version in the same transaction that sets
  status. The first finish still sets `finished_at` (it keeps deciding 按时 / 逾期); later
  finishes only add versions.
- Migration backfill: every writing with `status = 'finished'` gets version 1 from its
  current draft, `submitted_at = finished_at`.
- Versions are immutable. No delete.

Read endpoints (student, owner only):
- `GET /writings/{id}/versions` → `[{number, title, wordCount, submittedAt}]`, newest first.
- `GET /writings/{id}/versions/{n}` → the full version.

Teacher equivalents inside the existing lite teacher item payload (versions list) plus
`GET /lite/teacher/students/{uid}/writings/{atomId}/versions/{n}` behind `authTeacherStudent`.

### A2. Editing after 完成

- New column `writing.revising_at timestamptz NULL`.
- `POST /writings/{id}/revise` sets it (refused when locked, see A3).
- `POST /writings/{id}/finish` on a revising writing clears it and adds a version.
- `POST /writings/{id}/revise/discard` (放弃修改) restores `writing_draft.body` and title from
  the latest version and clears `revising_at`.
- The status stays `finished` the whole time: a homework stays 已提交 while she edits.
- `loadOwnedAtom`, writing branch only: a non-GET request on a finished writing is allowed
  iff `revising_at IS NOT NULL` and the writing is not locked. Otherwise 403
  `writing_finished` (not revising) or 403 `writing_locked` (locked). The reading branch is
  unchanged.
- Frontend: `WritingRoomHost` shows the finished page when `finished && !revising`, the
  writing room otherwise. While revising, the room shows a strip:
  「已提交 v{n} · 修改完成后请再次点击「完成」提交新版本」 with a 放弃修改 button.
- The report's `piece` follows the latest version (`reportWithPiece` reads
  `writing_version` instead of the live draft). Report stats stay as generated.

### A3. The lock rule

One pure function, used by the server gate and returned to the client as `locked` +
`lockReason`:

```
locked(now) =
  writing is linked to a lite_assignment recipient row
  AND the writing has at least one version
  AND now > effectiveDue
effectiveDue = recipient.return_due_at if recipient.returned_at is set, else assignment.due_at
```

- A homework not yet submitted is never locked (late submission stays possible, shown as
  已逾期提交, as today).
- Writings that are not homework are never locked.
- If the deadline passes mid-revision, the next write returns 403 `writing_locked`. The page
  shows the latest submitted version; unsubmitted changes stay in `writing_draft` and
  reappear if the teacher hands it back.
- Locked page copy: 「已过截止时间，作业已锁定」; 修改 is disabled.

### A4. 退回修改 (teacher)

- New columns on `lite_assignment_recipient`: `returned_at timestamptz NULL`,
  `return_due_at timestamptz NULL`, `return_note text NULL` (≤500).
- `POST /lite/teacher/assignments/{aid}/recipients/{uid}/return {dueAt, note?}`: writing
  homework only, recipient must have a version, `dueAt` in the future. Returning again
  overwrites the three columns.
- `liteassign.Status` gains two values for writing homework:
  - `returned` (已退回): `returned_at` set and no version submitted after it, now ≤ `return_due_at`
  - `resubmitted` (已重新提交): a version with `submitted_at > returned_at`
  - returned and past `return_due_at` with no new version → `overdue`
- The student sees 已退回 on the 作业 strip and the finished page, with the note and the new
  due date, and 修改 is enabled.

### A5. The page

Route stays `/writings/:id`. Width uses the report's 1180px measure (`.mk-rp-measure`).

- **Header:** title, status chip (已完成 / 已提交 / 已退回 / 已锁定), the homework title and
  due date when linked, buttons 修改 and 报告.
- **Left column** (reading measure, about 44rem): the version being viewed, rendered as
  paragraphs. Default = latest version.
- **Right rail:**
  - 版本: list `v3 · 9月15日 14:20 · 812 字`. Choosing one shows it on the left, with
    「与当前版本对比」 showing paragraph-level additions and deletions against the latest version.
  - 老师批改: the sent 批改 cards (Part B). Before Part B ships this panel is absent, not empty.
- **Below 1024px:** one column, rail content under the text.
- **报告** opens the existing `ReportPanel` (unchanged apart from `piece`).
- The diff is a pure frontend function (paragraph LCS, then character-level marks inside
  changed paragraph pairs) with logic tests.
- The layout is checked with Playwright screenshots at 1440px and 390px, not jsdom assertions.

---

## Part B · 一键AI批改 (plan B)

### B1. Rubric

Stored in the writing homework payload as `rubric` (validated in `liteassign/payload.go`):

```
rubric = {
  scale: "letter" | "points",
  max: int,             // points only, 1..100
  dimensions: [{ name: string ≤20, note: string ≤200 }],   // 1..6
  focus: string ≤500    // e.g. 重点看论证
}
```

- Letter grades: `A+ A A- B+ B B- C+ C C- D`.
- Defaults (`liteassign.DefaultRubric(lang)`), scale `letter`:
  - zh: 内容 / 结构 / 语言 / 书写规范
  - en: Task Response / Coherence and Cohesion / Lexical Resource / Grammatical Range and Accuracy
- A homework created before this ships has no rubric; it is read as the default for its language.
- The rubric is editable after students start. `PATCH` accepts `rubric` separately from the
  locked kind/payload fields. Each grading row snapshots the rubric it used.
- UI: a 评分标准 section in the writing homework form (rename, add, remove dimensions; scale;
  max; focus).

### B2. Data

New table `lite_grading`:

| column | type | note |
|---|---|---|
| id | uuid pk | |
| atom_id | uuid fk | the student's writing |
| version_id | uuid fk writing_version | the exact text graded, `UNIQUE(version_id)` |
| assignment_id | uuid NULL fk | set when the writing is homework |
| rubric | jsonb | snapshot |
| status | text | `queued` / `running` / `draft` / `failed` / `sent`, CHECK |
| ai | jsonb NULL | validated AI result, never edited |
| content | jsonb NULL | what the teacher edits and sends; starts as a copy of `ai` |
| error | text NULL | failure reason shown to the teacher |
| requested_by | uuid | teacher |
| reviewed_at | timestamptz NULL | set when the teacher saves or presses 标记已审阅 |
| sent_at | timestamptz NULL | last send |
| student_seen_at | timestamptz NULL | |
| created_at / updated_at | timestamptz | |

`content` shape:

```
{
  overall:    { grade: string, comment: string },
  dimensions: [{ name: string, grade: string, comment: string }],
  points:     [{ kind: "good"|"issue", quote: string|null, text: string, action: string|null,
                 source: "ai"|"teacher" }]
}
```

### B3. Running it

- `POST /lite/teacher/assignments/{aid}/gradings {retryFailed?: bool}` queues one job per
  recipient that has a latest version with no grading row (and, with `retryFailed`, rows in
  `failed`). Returns `{queued: n}`.
- `POST /lite/teacher/students/{uid}/writings/{atomId}/gradings` queues one job for that
  writing's latest version. Rubric = the homework's if linked, else the default for its language.
- A version that already has a row in `sent` is not regraded. A `draft` row can be regraded
  (replaces `ai` and `content`, clears `reviewed_at`) after the teacher confirms
  「重新批改会覆盖当前修改」.
- A river job `lite_grading` (pattern: `interest_jobs.go`), `MaxAttempts: 1` so river never
  charges twice; the worker does its own single retry.
- Worker: status `running` → build prompt (version body, title, assigned prompt if any,
  rubric, language, the writing room's symptom list for issue naming) →
  `a.routeE(ctx, gateway.ClassReview)` → parse → `litegrade.Check`. On failure, retry once
  with the failure reasons appended. Still failing → status `failed`, `error` = the reasons.
  On success → status `draft`, `ai = content = result`.
- Metering: `recordLiteLLMCall(studentID, atomID, "teacher_grading")`; entitlement via
  `studentEntitled`.
- Prompt and check live in a new package `internal/litegrade` (pure, no HTTP), worker and
  handlers in `internal/api`.

### B4. `litegrade.Check`: what code verifies

A result is rejected, with a named reason, when:
- a point's `quote` is not a literal substring of the version body;
- any 「」 quotation inside `overall.comment`, a dimension comment, a point's `text` or
  `action` is not a literal substring of the version body. This is the rewrite guard. Text
  that matches only the teacher's assigned prompt counts as not hers (memory
  `lite-teacher-end-2026-09-15`: teacher text is never her words);
- a grade is outside the rubric scale, or the dimension names are not exactly the rubric's names;
- there are fewer than 3 or more than 5 points, or no `good` point, or no `issue` point;
- an `issue` has an empty `action`;
- a point judges her as a person (reuse the writing-comment check).

Teacher edits (`PATCH`) run a shape check only: grades in scale, dimension names match, a
teacher point's `quote` is either null or a substring. No 3–5 limit for the teacher.

Before Part B ships, run `LIVE_LLM=1 go test ./internal/api -run TestLiveLiteGrading` three
times on one zh and one en writing; judge the worst run.

### B5. Teacher UI

- **Homework detail page → new tab 批改** (writing homework only): one row per student:
  name, 版本 (v2 · submitted time), status (未提交 / 待批改 / 批改中 / 草稿 / 已审阅 /
  已发送 / 批改失败), overall grade.
  - Buttons: **一键AI批改** (queues all eligible), **重试失败**, **发送全部已审阅** (sends
    every row with `reviewed_at` and status `draft`; confirms with the count).
  - The tab polls every 5s while any row is `queued` or `running`.
- **Row → grading view:**
  - Left: her version text with quoted sentences highlighted; clicking a point scrolls to its quote.
  - Right: editable card for the overall grade and comment, dimension grades and comments, and
    points (edit, delete, add 意见; a teacher point may select a sentence on the left as its quote).
  - Buttons: 保存, 标记已审阅, 发送, 重新批改, 退回修改 (opens the A4 dialog).
  - Failed rows show 「批改失败：{error}」 and 重新批改.
- **Student item page (`ItemPage.tsx`):** the writing view gets the version list and a 批改
  button that uses the single-writing endpoint. Also fix the bug that drops object comment points.

Teacher endpoints:
- `GET /lite/teacher/assignments/{aid}/gradings`
- `GET /lite/teacher/gradings/{gid}`
- `PATCH /lite/teacher/gradings/{gid} {content, reviewed?}`
- `POST /lite/teacher/gradings/{gid}/send`
- `POST /lite/teacher/assignments/{aid}/gradings/send {ids}`

All check that the teacher owns the class of the writing's owner (`authTeacherStudent`) and,
for assignment routes, the assignment's class.

### B6. Student side

- `GET /writings/{id}/gradings` returns only `sent` rows: `content`, rubric, version number, `sentAt`.
  Never `ai`, drafts, queued or failed rows.
- The finished page rail shows 老师批改:
  - overall grade, dimension grades, comment, points;
  - the line 「由 AI 起草，老师审阅后发送」;
  - 「针对 v{n}」 when the page shows a different version.
  - Clicking a point's quote shows that version on the left and highlights the sentence.
- Inbox: `GET /lite/inbox` gains items `{kind: "grading", gradingId, writingTitle, sentAt, seen}`.
  Opening marks `student_seen_at` (`POST /lite/inbox/gradings/{gid}/seen`) and navigates
  to `/writings/:id`.
- A 批改 re-sent after edits updates in place and becomes unseen again.

---

## Part C · Homework materials (plan C)

### C1. Upload a document

- Reading homework form: a fourth source tab 上传文件 (pdf, docx, txt, md).
- The browser posts the file to `POST /documents/extract` and fills source `text` with the
  result, and the title if empty. It shows 「已提取 {n} 字」.
- Payload `text` source gains optional `fileName` (≤200) so the teacher's homework list shows
  the file name.
- Errors: 「提取失败：{后台原话}」; over 50000 characters → 「提取失败：正文超过 50000 字」.
- Known limits, stated in the form hint: images inside a PDF are not kept; a scanned PDF has
  no text (the extractor already reports this).

### C2. Personalised reading

- New reading source `personalized`:

  ```
  { source: "personalized", disciplines: string[] (optional filter),
    tier: 1..5 | null (null = each student's SuggestTier),
    picks: { [userId]: { slug: string, tier: 1..5 | null } } }
  ```

- `POST /lite/teacher/classes/{cid}/personalized-reading/preview {disciplines?, tier?}` →
  `[{userId, name, slug, title, tier, reason}]` for every enrolled student.
  - Article: the existing recommender run for that student (refactor the strength function in
    `library.go` to take a user id). Articles she has opened are excluded, filtered by
    `disciplines` when given.
  - Tier: `tier` if given, else her `SuggestTier`.
  - Reason, built in code, no model:
    - 「兴趣相关：{学科1}、{学科2}」
    - no interest data → 「暂无兴趣数据，按难度推荐」
    - nothing left in the filter → the first unopened article overall, with 「筛选范围内暂无合适文章」
- Form: choosing 个性化 shows the preview table; each row has 更换, which opens the existing
  `LibraryPicker` for that student. Submitting saves `picks`.
- Validation: every slug exists; every pick's user is a recipient.
- Start (`lite_student_assignments.go`): a pick → the library start for that slug and tier
  (SuggestTier when null). A recipient without a pick (joined later) gets the recommendation
  computed at start.
- The teacher's homework detail shows each student's article title and tier.

---

## 4. Out of scope

- 批改 for reading or project homework; grades in parent reports or weekly summaries.
- A shared rubric library across homeworks.
- File attachments on homework beyond extracting reading text.
- Diversifying personalised picks across a class.
- Any pro-edition change.

## 5. Testing

Logic tests only (AGENTS.md):
- lock rule
- status derivation with returned / resubmitted
- version numbering and backfill
- the `loadOwnedAtom` writing gate
- rubric validation and defaults
- `litegrade.Check`, every rejection reason
- grading state transitions and "students never see non-sent rows"
- teacher ownership on every new route
- personalised preview selection (filter, fallback, exclusion, tier)
- the version diff function

UI is verified with Playwright screenshots of the finished page (1440 / 390), the 批改 tab and
grading view, and the personalised preview. Plan B owes the `LIVE_LLM` run in B4.

## 6. Delivery

- Branch `lite-homework-grading`, one plan per part, rebased on main before each plan (a
  refactor job is running on main and touches `teacher/ItemPage.tsx` and `reports/`).
- Migrations take the next free numbers at build time (0153+ as of `dca31680`).
- The deploy runs the migrations with the new API container.
- `docs/2026-08-09-all-statuses.md` covers pro's writing-project flow, not the lite writing
  room, so it does not constrain Part A.

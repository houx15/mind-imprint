# Lite teacher end — design

> Status: design approved by the product owner in brainstorm, 2026-09-14.
> Scope: `apps/lite-web` + lite-only paths in `apps/api`. Pro (`apps/web`, pro teacher console) is not changed.
> Build: one spec, four plans, built in order (§8).

## 1. What this builds

A teacher end for the lite edition, with four features:

1. **Student data.** A teacher sees each student's readings, writings and projects: totals (time, AI turns, counts), each item's outputs and shining moments, and the student's interest tree.
2. **Practices with deadlines.** A teacher assigns one reading, one writing or one project (PBL) to a class or to picked students, with a deadline. Students receive it and act on it.
3. **Parent report.** A teacher generates a shareable report on one student over a chosen date range. AI drafts, the teacher edits, then publishes a link.
4. **Student summary.** A per-student 上周表现总结 (plus a class 周报), so the teacher can give individual support.

### Decisions from the brainstorm

| # | Decision | Source |
|---|---|---|
| DEC-1 | Teachers see **outputs and moments, never the raw chat transcript** with 印记. Turn *counts* are shown. | Owner choice |
| DEC-2 | Classes, join codes, teacher invites work **the same as pro**: admin issues teacher invites, teacher creates classes and shares the join code. Existing `/api/v1/classes*` endpoints and pro console class views are reused. | Owner: "same as pro" |
| DEC-3 | Architecture mirrors pro: a **teacher shell inside the lite app**, selected by role (pro: `AppShell` → `ConsoleShell` in `apps/web`). | Owner: "similar as pro version" |
| DEC-4 | An assignment goes to a **class by default; the teacher can untick students**. | Owner choice |
| DEC-5 | A reading practice is a **library article (+ level) or the teacher's own link / text**. | Owner choice |
| DEC-6 | A writing practice is **prompt + length + language**. The teacher may instead paste or type a paragraph; we extract the three fields for the teacher to confirm. | Owner choice |
| DEC-7 | An assigned project **bypasses the first-project homepage gate**. The student's own projects still require the homepage. | Owner choice |
| DEC-8 | An assignment is done **when the student finishes it** (existing finish states). No teacher grading. | Owner choice |
| DEC-9 | Students see assignments in **two places**: a strip at the top of 阅读 / 写作 / 项目, and an **inbox** in the lower-left of the nav rail with a red dot while anything is unread; clicking an item jumps to the work. | Owner choice |
| DEC-10 | Parent report: **AI drafts, teacher edits, then publishes** a revocable share link. The teacher picks the date range (default last 4 weeks). | Owner choice |
| DEC-11 | A published parent report **also appears in the student's inbox**, read-only. | Owner choice |
| DEC-12 | The student summary follows **pro's weekly pattern**: last completed week, numbers live, AI text generated once per (student, week) on first open and cached. The UI names the week (上周表现总结 · 第 N 周). | Owner: "similar as pro … indicate it in the UI" |
| DEC-13 | A student may still finish after the deadline; the item is marked 逾期完成. | Assumption stated in brainstorm, not objected to |
| DEC-14 | A finished reading/writing whose report was never generated gets it generated on first teacher open. | Assumption stated in brainstorm, not objected to |

### Earlier decisions this reverses (deliberately)

- `2026-06-26-p3-1-org-backend-rbac-design.md`: teachers see aggregates only, never content. Lite teachers now see student outputs (not chat — DEC-1).
- `2026-07-24-d1/d2`: "no teacher → student action of any kind". Lite teachers now assign practices.
- `2026-08-29-lite-reports-design.md`: "no teacher/parent view". Lite now has both.
- Pro's `ClassWeeklyView` states 作业布置与提交在学校自己的平台完成. That stays true for pro; pro is not changed.

### Not in scope

Teacher grading or comments on submissions; email / WeChat notifications; chat transcripts; teacher-authored library articles; class-wide parent reports; changes to pro.

---

## 2. Current state (verified 2026-09-14)

- **Org.** `users.role ∈ student|teacher|admin`, `enrollments(user_id, class_id, role_in_class)`, `classes.join_code`, `teacher_invites`. `assertTeacherOwnsClass` and `authTeacherStudent` (`internal/api/teacher_read.go`) already answer "may this teacher see this student" through class membership.
- **Lite teacher today.** `LiteApp.tsx` checks edition, then renders `LiteShell` for every role — a teacher at a lite school gets the student app. `liteOnly` checks edition, not role.
- **Pro teacher queries** (`teacher.sql`, `teacher_weekly.sql`) read pro tables (`project`, `evaluations`, `event`). Lite writes none of those; for a lite class they return empty.
- **Lite data per student** is all stored on `atom` (`kind ∈ reading|writing|project`, `user_id`, `active_seconds`, `last_activity_at`) and its children: `reading*`, `writing*`, `pbl_*`, `atom_annotation`, `atom_card`, `atom_message` (turns = `role='student'`), `atom_report` (lazy, share token), `interest_keyword` + `keyword_source` + `keyword_discipline`, `llm_call` (`user_id`, `atom_id`).
- **Gaps.** PBL has no heartbeat (`active_seconds` stays 0). `pbl_project` has no `finished_at`. Assignments / deadlines exist nowhere. `atom_report` rows exist only if the student opened the report.
- **Ownership helpers** (`loadOwnedAtomRow`, `loadOwnedPblProject`) check the session user; `loadOwnedAtom` also bumps `last_activity_at` on non-GET. Teacher reads must use none of them.

---

## 3. Shell and access (plan 1)

### 3.1 Frontend

- `LiteApp.tsx`: after the edition decision, `user.role ∈ {teacher, admin}` → `LiteTeacherShell`; otherwise `LiteShell` (unchanged).
- `LiteTeacherShell` uses the same rail look as `LiteShell`. Rail items: **班级** · **布置** · **家长报告** · (admin only) **教师** / **导入** / **概览** · **设置** at the foot.
- Class list / create / rename / rotate join code / remove student reuse pro console components (`ClassesView`, the class header of `ClassDetailView`) through the existing `@/` alias. Admin items reuse `TeachersView`, `ImportView`, `OverviewView`. Any prop needed to host them in lite is an optional prop whose default keeps pro's behaviour (lite-must-not-break-pro).
- Routes (URL-based, like the student shell): `/classes`, `/classes/:cid`, `/classes/:cid/students/:uid`, `/classes/:cid/students/:uid/items/:atomId`, `/classes/:cid/weekly`, `/assignments`, `/assignments/new`, `/assignments/:aid`, `/parent-reports`, `/parent-reports/:rid`, `/settings`. `routing.ts` gains a teacher route parser; public routes (`/s/`, `/p/`, new `/r/`) stay pre-login.

### 3.2 Backend

- All new teacher routes live under `/api/v1/lite/teacher/…` and are wrapped `liteOnly` **and** `RequireRole("teacher","admin")`.
- Every route that names a student goes through `authTeacherStudent(classID, userID)` (404 on miss). Every route that names an atom additionally checks `atom.user_id = userID`.
- Existing student lite routes are unchanged.
- New query files are named `lite_teacher.sql`, `lite_assignment.sql`, `lite_weekly.sql`, `lite_parent_report.sql` (none exist today; `ls` again before creating).

---

## 4. Student data (plan 1)

### 4.1 Class roster — `GET /api/v1/lite/teacher/classes/{cid}/roster`

One row per enrolled student, computed live (no model):

| Column | Derivation |
|---|---|
| 学生 | `users.display_name` |
| 最近活跃 | `max(atom.last_activity_at)` |
| 本周活跃天数 | distinct Beijing dates of `atom_message.created_at` (role student) in the current week |
| 学习时长 | `sum(atom.active_seconds)` → minutes |
| AI 对话轮次 | `count(atom_message) where role='student'` across all threads of her atoms |
| 阅读 / 写作 / 项目 | finished / total per kind |
| 逾期作业 | count of recipient rows in status 已逾期 |

Columns sort client-side. Clicking a row opens the student page.

### 4.2 Student page — `GET /api/v1/lite/teacher/classes/{cid}/students/{uid}`

Returns: header stats (the roster row plus 本周 time/turns); lists of readings, writings, projects (title, status, level/kind, time, turns, finished_at, `assignmentId` if it came from one); her assignments in this class with status. The page also shows the 上周表现总结 card (§6) and the interest tree.

### 4.3 Interest tree — `GET /api/v1/lite/teacher/classes/{cid}/students/{uid}/tree`

Same payload as `GET /api/v1/interest/tree`, built by the same code with the student's id. Frontend: `TreeView` rendered with this data source in a **read-only mode** — quiz entry, `DigSection`, and proposals hidden. `useInterestTree` gains an optional fetcher; default keeps the session URL.

### 4.4 Item detail — `GET /api/v1/lite/teacher/classes/{cid}/students/{uid}/items/{atomId}`

Outputs and moments only (DEC-1). No `atom_message.content` leaves this endpoint.

- **Reading:** title, library slug + level or source URL, status, finished_at, stats (the reading report stat keys), highlights and notes (`atom_annotation`), takeaway (`reading_takeaway`), lens card outputs (`atom_card.field_values`), report `moments` (金句) and `keep`.
- **Writing:** title, target words, language, structure key, her outline text, snippets, final draft (`writing_draft.body`), AI comments (`writing_comment`), stats, report `moments` and `keep`.
- **Project:** idea, status, plan steps (done / total, titles), tool outputs (`pbl_tool_instance.result`), settled artifacts, keep entries, course assignments with her takeaways, homepage link if published.
- **Report on first teacher open (DEC-14):** if the atom is a finished reading/writing without an `atom_report` row, call the same internal `ensureAtomReport` the student path uses (not the owner-only handler). The `llm_call` row is recorded with `user_id` = the student and `purpose='lite_report'`, as on the student path. Failure → the item renders without report sections and shows 报告生成失败：{后台原话}.
- Stat labels resolve client-side from `stat.key` via `reports/statLabels.ts` (stored blobs never supply labels).

### 4.5 Time: PBL heartbeat and per-day buckets

- Add `POST /api/v1/pbl/projects/{id}/heartbeat`, same 120-second clamp as readings/writings, called by the project room while visible.
- `atom.active_seconds` is a running total and cannot be split by week. Add

  ```sql
  CREATE TABLE atom_active_day (
    atom_id uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
    day     date NOT NULL,              -- Beijing date (fixed +08:00)
    seconds integer NOT NULL DEFAULT 0,
    PRIMARY KEY (atom_id, day)
  );
  ```

  Every heartbeat (reading, writing, project) adds the same clamped seconds to `atom.active_seconds` **and** upserts today's bucket, in one statement pair inside the existing handler. Students see no change.
- Weekly minutes (§4.1 本周, §6) = sum of buckets in the window. Weeks before this migration have no buckets and show 学习时长 as "—", never an estimate. An item with `active_seconds = 0` also shows "—".
- 活跃天数 = distinct Beijing dates that have a bucket with `seconds > 0` or a student `atom_message`.

---

## 5. Practices with deadlines (plan 2)

### 5.1 Data

```sql
CREATE TABLE lite_assignment (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  class_id     uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  created_by   uuid NOT NULL REFERENCES users(id),
  kind         text NOT NULL CHECK (kind IN ('reading','writing','project')),
  title        text NOT NULL,
  instructions text NOT NULL DEFAULT '',
  payload      jsonb NOT NULL,
  due_at       timestamptz NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz
);

CREATE TABLE lite_assignment_recipient (
  assignment_id uuid NOT NULL REFERENCES lite_assignment(id) ON DELETE CASCADE,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  seen_at       timestamptz,
  atom_id       uuid UNIQUE REFERENCES atom(id) ON DELETE SET NULL,
  started_at    timestamptz,
  PRIMARY KEY (assignment_id, user_id)
);

ALTER TABLE pbl_project ADD COLUMN finished_at timestamptz;
```

Migration number = next free at plan time (0147 is the latest on `origin/main` as of 2026-09-14).

`payload` shape per kind, validated in Go at the boundary:

- reading: `{ "source": "library", "slug": "...", "tier": 1..5 | null }` — `null` means each student gets `library.SuggestTier` for her history; or `{ "source": "url", "url": "..." }`; or `{ "source": "text", "text": "..." }`.
- writing: `{ "prompt": "...", "targetWords": int, "lang": "zh"|"en" }`.
- project: `{ "drivingQuestion": "...", "description": "..." }`.

`pbl_project.finished_at` is set once, the first time `status` moves into `review` or `keeping` (never re-stamped, same rule as `SetReadingFinished`).

### 5.2 Status (derived, pure function, tested)

| Status | Rule |
|---|---|
| 未开始 | `atom_id IS NULL` and `now ≤ due_at` |
| 进行中 | atom exists, not finished, `now ≤ due_at` |
| 已完成 | finished_at ≤ due_at |
| 逾期完成 | finished_at > due_at |
| 已逾期 | not finished and `now > due_at` |

"Finished" = reading/writing `status='finished'`; project `finished_at IS NOT NULL`.

### 5.3 Teacher flow

- **List** `GET /api/v1/lite/teacher/classes/{cid}/assignments` — title, kind, due, counts per status.
- **Create** `POST /api/v1/lite/teacher/classes/{cid}/assignments` — `{kind, title, instructions, payload, dueAt, userIds[]}`. `userIds` defaults (in the UI) to every enrolled student; the server rejects ids not enrolled as students in the class. Creates the assignment and recipient rows in one transaction.
- **Writing extraction** `POST /api/v1/lite/teacher/assignments/extract` — `{text}` → `{prompt, targetWords, lang}`. One `compose` call (`gateway.ClassCompose`), JSON output, normalizer clamps `targetWords` to 50–10000 and `lang` to the closed set. The form is pre-filled; the teacher confirms or edits. Failure → form stays empty with 提取失败：{后台原话}.
- **Reading source picker:** library browser (article + level, or 按学生当前水平), or a URL field, or a text area.
- **Detail** `GET /api/v1/lite/teacher/assignments/{aid}` — settings + one row per recipient (student, status, started_at, finished_at, link to the item detail).
- **Edit** `PATCH /api/v1/lite/teacher/assignments/{aid}` — title, instructions, dueAt always; `payload` and `kind` only while no recipient has `atom_id`; add recipients anytime; remove a recipient only while that recipient has no `atom_id`.
- **Archive** `DELETE /api/v1/lite/teacher/assignments/{aid}` → `archived_at = now()`. Archived assignments disappear from students' strips and inbox; items already started stay hers.
- Authorization: the teacher must own `class_id` (`assertTeacherOwnsClass`).

### 5.4 Student flow

- **Inbox** `GET /api/v1/lite/inbox` — her non-archived assignments (with status and due) and her published parent reports (§7.5), unread first. `unread` = `seen_at IS NULL`.
- **Rail inbox button** (`LiteShell`): placed directly above 设置, label 收件箱, red dot when `unread > 0`. Opens a panel listing items; clicking an item marks it seen and navigates.
- **Tab strips:** at the top of 阅读 / 写作 / 项目 landings, a 老师布置 strip lists her open (not finished) assignments of that kind: title, 截止 {M月D日 HH:mm}, status. Hidden when empty.
- **Mark seen** `POST /api/v1/lite/assignments/{aid}/seen`.
- **Start** `POST /api/v1/lite/assignments/{aid}/start` → `{kind, atomId}`. Idempotent: `SELECT … FOR UPDATE` on the recipient row; if `atom_id` is set, return it. Otherwise create the item through the existing internal creation code, then set `atom_id`, `started_at`, `seen_at`:
  - reading / library → the logic behind `POST /api/v1/library/{slug}/levels/{tier}`, tier from payload or `SuggestTier`;
  - reading / url or text → the logic behind `POST /api/v1/readings` + `PUT /api/v1/readings/{id}/source`;
  - writing → the logic behind `POST /api/v1/writings` (idea = prompt, lang) + `PUT /api/v1/writings/{id}/target-words`; structure and planning stay hers in the writing room;
  - project → the logic behind `POST /api/v1/pbl/projects` with idea = driving question, **skipping `siteGateOpen`** (DEC-7). The gate remains in the public create handler untouched.
  - Failure → nothing is written to the recipient row; the UI shows 开始失败：{后台原话}.
- **Room header line:** the reading, writing and project GET payloads gain an optional `assignment {title, dueAt}`; when present the room header shows 老师布置 · 截止 {M月D日}. The reading room is shared with pro, so this is an optional prop defaulting to absent.
- **Writing-flow check:** before implementing the writing start path, plan 2 must be checked against `docs/2026-08-09-all-statuses.md` (writing statuses), per AGENTS.md.

---

## 6. 上周表现总结 and class 周报 (plan 3)

Mirrors pro D2 / the 2026-08-14 two-view model, on lite data.

### 6.1 Week

- Last **completed** Monday 00:00 → next Monday 00:00, **Beijing time as a fixed +08:00 offset** (the prod image has no tzdata; `LoadLocation` would silently fall back to UTC). Pure function `liteweek.Completed(now)` plus prev/next, tested.
- The teacher can page back through completed weeks; never into the current week.
- UI label: `上周表现总结 · 第 {ISO week} 周（{M.D}–{M.D}）` for the latest week; `表现总结 · 第 N 周（…）` for earlier weeks.

### 6.2 Facts (deterministic, no model)

Per (student, week), computed live:

- active days and minutes (from `atom_active_day`, §4.5), AI turns (student `atom_message` rows in the window);
- readings / writings / projects finished in the week (titles);
- assignments due in the week: 已完成 / 逾期完成 / 已逾期;
- stalled items: not finished, `last_activity_at` older than 7 days at week end;
- 金句 from `atom_report.moments` of items finished in the week (her literal words, already verified at report time);
- same numbers for the previous week (deltas).

Rule cards (pure Go, closed tag set, each with a fixed evidence template written in code, never by the model):

| Card | Tag | Fires when |
|---|---|---|
| 需要建议 | `never_used` | active days = 0 |
| 需要建议 | `overdue` | ≥ 1 assignment due in the week is 已逾期 at week end |
| 需要建议 | `dropped_off` | active days ≤ previous week's active days − 2 |
| 需要建议 | `stalled` | ≥ 1 unfinished item with `last_activity_at` more than 7 days before week end |
| 值得表扬 | `finished_on_time` | ≥ 1 assignment due in the week is 已完成 and none is 已逾期 |
| 值得表扬 | `new_interest` | ≥ 1 `interest_keyword.first_seen_at` in the week |
| 值得表扬 | `more_active` | active days ≥ 3 and ≥ previous week's active days + 2 |

A student gets at most one 需要建议 card (precedence in table order) and at most one 值得表扬 card (precedence in table order); `never_used` suppresses praise. The facts passed to the model include the week label and date range, so dates in prose pass the digit check.

### 6.3 Prose

- One `assess` call per (student, week): `POST /api/v1/lite/teacher/classes/{cid}/students/{uid}/weekly/prose?weekStart=`. `GET …/weekly?weekStart=` never calls a model; it returns facts, cards, and stored prose if present.
- The frontend requests prose on first view when `proseReady=false` (same latch as pro `ClassWeeklyView`, at most once per mount).
- Output: `summary` (≤ 150 字) and `suggestions[1..3]`, each `{text, evidenceCode}` where `evidenceCode` must be one of the week's card codes.
- Checks before storing (retry the call once on failure): every `evidenceCode` exists; any quoted span (「…」 or “…”) appears verbatim in that week's 金句 or item titles; no person name other than the student's; every digit sequence in the prose appears in the facts.
- Stored forever: `lite_student_weekly_prose(user_id, week_start, body jsonb, created_at, PRIMARY KEY(user_id, week_start))`.
- Failure after retry → 200 with prose absent; the card shows facts and the line 总结生成失败：{后台原话}. `llm_call` is recorded either way.

### 6.4 Class 周报

`GET/POST /api/v1/lite/teacher/classes/{cid}/weekly[/prose]` — class stats (active students / class size, minutes, turns, items finished, assignment completion rate), 值得表扬 / 需要建议 student cards from §6.2, and one `assess` class comment per (class, week) in `lite_class_weekly_prose(class_id, week_start, body jsonb, created_at)`. Same checks and failure handling as §6.3.

---

## 7. Parent report (plan 4)

### 7.1 Data

```sql
CREATE TABLE lite_parent_report (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- the student
  class_id      uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  created_by    uuid NOT NULL REFERENCES users(id),
  range_start   date NOT NULL,
  range_end     date NOT NULL,
  facts         jsonb NOT NULL,          -- frozen snapshot
  draft         jsonb,                   -- last AI draft
  body          jsonb,                   -- teacher-edited sections
  status        text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  share_token   text UNIQUE,
  published_at  timestamptz,
  student_seen_at timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
```

### 7.2 Generate

`POST /api/v1/lite/teacher/classes/{cid}/students/{uid}/parent-reports` `{rangeStart, rangeEnd}` (default last 28 days, max 366 days, dates in Beijing time).

1. Build the **facts snapshot** (no model): stats for the range (minutes, turns, active days, items finished per kind); finished items with titles; 金句 from their stored reports (her words only; reports missing for finished items are generated first, as in §4.4); assignment completion in range; top interest keywords per branch with first-seen dates in range.
2. One `assess` call writes sections for parents, in Chinese: `overview` 总体概述 · `reading` 阅读 · `writing` 写作 · `projects` 项目 · `interests` 兴趣 · `next` 下一步建议. A section whose facts are empty is omitted.
3. Same checks as §6.3 (quotes verbatim from facts, digits present in facts, no other names), retry once. Stored in `draft` and copied into `body`.
4. Failure → the report row exists with facts only; the editor shows 草稿生成失败：{后台原话} and empty editable sections.

### 7.3 Edit and publish (teacher)

- `GET /api/v1/lite/teacher/parent-reports/{rid}`; list per student and per class.
- `PATCH …/{rid}` `{body}` — plain text per section (no rich formatting).
- `POST …/{rid}/redraft` — new AI draft from the same facts; allowed only while `status='draft'`; overwrites `body` after the teacher confirms in the UI.
- `POST …/{rid}/publish` — sets `status='published'`, `published_at`, mints a 32-hex `share_token` (crypto/rand). After publish, `body` edits are still allowed and take effect on the public page immediately.
- `DELETE …/{rid}/share` — `share_token = NULL`; the public link stops working on the next request. The report stays visible to the teacher and the student.
- Authorization: `authTeacherStudent(class_id, user_id)` on every route.

### 7.4 Public page

- `GET /api/v1/public/parent-reports/{token}` — no auth, returns facts + body (no ids, no bookkeeping), `X-Robots-Tag: noindex`.
- `apps/lite-web` route `/r/:token` → `PublicParentReportPage`, pre-login like `/s/:token`. Wide colourful layout in the lite report style (`mk-report-*` tokens); stat tiles, sections, 金句 labelled as 她的原话, interest keywords, a header naming the student, class, date range and "由 {老师} 发布". PNG export reuses the lite report poster export (offset on a wrapper, per the blank-PNG lesson).

### 7.5 Student view (DEC-11)

- A published report appears in her inbox: 老师发布了一份家长报告（{range}）.
- `GET /api/v1/lite/parent-reports/{rid}` — subject only (`user_id = session user`), published only; `POST …/{rid}/seen` sets `student_seen_at`. Renders the same page component, read-only, regardless of share-link state.

---

## 8. Build order

Each plan is written after the previous plan ships, and each is independently useful.

| Plan | Contents | Student-side change |
|---|---|---|
| 1 | §3 shell + class management reuse, §4 roster / student page / item detail / tree, PBL heartbeat, `atom_active_day` buckets | Heartbeat writes only (nothing visible) |
| 2 | §5 assignments, inbox (assignments), tab strips, start flow, room header line, `pbl_project.finished_at` | Yes |
| 3 | §6 上周表现总结 + class 周报 | No |
| 4 | §7 parent report, public page, inbox item | Inbox item |

## 9. UI copy

All teacher and student strings follow AGENTS.md § 界面文案怎么写. Labels are nouns (截止时间, 学习时长, 对话轮次); buttons are actions (创建作业, 发布, 撤销链接, 重新生成草稿); states use 已/待/中 pairs (§5.2); errors are 动词+失败：{后台原话}; no literary style.

## 10. Testing

Logic tests only (AGENTS.md); UI is verified with a one-off Playwright walk and screenshots per plan.

- **Authorization:** teacher of another class → 404; student calling a teacher route → 403; teacher at a pro school → 404; atom not belonging to the named student → 404; student reading another student's parent report → 404.
- **Transcript never leaks:** the item-detail DTO builder has no path that reads `atom_message.content` (test asserts a seeded message's content is absent from the JSON).
- **Assignment status** derivation table (§5.2), including the exact `due_at` boundary.
- **Start:** idempotent under two concurrent calls (one atom); tier = `SuggestTier` when payload tier is null; project start bypasses the homepage gate while `POST /pbl/projects` still returns 409 for the same student.
- **Edit locks:** payload change rejected once any recipient started; removing a started recipient rejected.
- **Week window:** fixed +08:00, Monday boundary at 23:59/00:00 Beijing, never the current week.
- **Day buckets:** a heartbeat at 23:59:30 and one at 00:00:30 Beijing land in two different `day` rows; the 120 s clamp applies to both the total and the bucket.
- **Rule cards:** each tag's threshold and precedence.
- **Prose checks:** fabricated quote, stray digit, unknown name, unknown evidence code each rejected.
- **Extraction normalizer:** clamps and closed-set language.
- **Share revoke:** public GET 404 after DELETE.
- **Live prompt check:** each new `assess`/`compose` prompt runs once with `LIVE_LLM=1` before shipping (a stub test only proves the parser reads JSON we wrote).
- **Pro safety** after each plan: `git diff --diff-filter=D --name-only` empty, modified shared files purely additive, pro's own test suites green.

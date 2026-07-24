# Spec D2 · Class Weekly Report — design

Date: 2026-07-24
Status: approved (brainstorm complete)
Depends on: Spec B (canonical `agent.Report`), Spec D1 (teacher read-path, merged `b64f9d4`)
Binding design: `docs/design/teacher end/project/思维印记 教师端.dc.html` — HOME screen, lines 75–232; view-model `homeVals()` lines 1554–1568; fixture data `CLASS` / `ROSTER` lines 1428–1512.
Program context: `docs/2026-07-24-assessment-teacher-end-program.md` §5 Spec D.

---

## 1. What this spec builds

The teacher's 班级周报 — the "ten minutes a week" screen. It is the second and last
half of Spec D: D1 shipped the read-path (roster → student → deep report) over data
that already existed; **D2 is where all of Spec D's net-new pipeline risk lives.**

Delivered:

1. A weekly usage aggregation over the event stream (four stat cards, week-over-week deltas).
2. A closed-vocabulary 特征标签 detector — deterministic rules over (usage window + canonical report) that decide **who** appears on the 值得表扬 / 需要建议 cards and **with what evidence**.
3. One flagship LLM composition per class per week that writes the Bean 班级点评 and the cards' plain-language wording — and nothing else.
4. The 班级思维维度 block: D-axis distribution across the class + A-axis class mean.
5. The screen itself, landing inside a class beside D1's roster.

### Explicitly NOT in scope

- No export / print (that is Spec E).
- No 👍/👎 calibration control — absent from the binding design, dropped in D1 for the same reason.
- No cross-class or cross-school comparison, no ranking of any kind (贯穿性不变式).
- No teacher → student action of any kind. 单向只读 holds: nothing on this screen reaches a student.
- No river/worker scheduling. Generation is lazy, on the teacher's first open.
- No change to the assessment engine, the rubric, or the canonical object's shape.

---

## 2. Decisions taken in the brainstorm

| # | Decision | Rationale |
|---|---|---|
| DEC-1 | **Numbers live, prose cached.** Every number is computed on read from SQL; only the LLM-written words are stored. | A teacher opening on Wednesday sees Wednesday's class. Nothing about the aggregation needs to be frozen. |
| DEC-2 | **Prose is generated once per (class, week) — first open wins.** It is never regenerated. | User's explicit choice. Cheapest and fully reproducible. Consequence and its fix: §6.4. |
| DEC-3 | **Rules pick; the model only writes.** Who is flagged, which tag, and which evidence is quoted are all deterministic. | 教师端铁律③「每个判断带证据」 + 敢于空白. A model that picks who to flag produces judgments a teacher cannot check. |
| DEC-4 | **Add `event.course_id` and emit `step_viewed`.** | Course page-turns are recorded only into the mutable `course_progress.completed_ordinals` array, with no timestamp — the 完成课程节 card is otherwise unqueryable. Additive migration copying the 0024/0025 precedent. |
| DEC-5 | **Week = Monday 00:00 UTC → now; delta vs the same elapsed offset last week.** | D1's roster already shows 本周活跃 over exactly this window and the two screens sit adjacent. Comparing week-to-date against a full previous week would paint every mid-week open red. |
| DEC-6 | **The top-up call (§6.4) is accepted.** | Keeps DEC-2's semantics (nothing already written is ever rewritten) while preventing a card from rendering with no wording at all. |
| DEC-7 | **Two D1 carry-forwards are closed here** (§9): `weekWindow` becomes a tested pure function shared by both screens; "latest report" becomes cross-surface for both the roster and the distribution. | D2 is the spec that makes both defects visible. |

---

## 3. Data-availability findings (verified against the code)

These were established by reading the code, not assumed. They are the reason the
spec has the shape it does.

- **`event`** (`0016_refactor2_foundations.sql:140`) carries `user_id`, `surface ∈ {studio,course,chat}`, an open `type`, `payload jsonb`, `created_at`, and a scope (`project_id` / `session_id` / `thread_id`, `event_scope_ck` requiring ≥1, `NOT VALID`).
- **Turn types**: `prompt_sent` is emitted for studio (`studioturn.go:203`) *and* chat (`chat.go:200`); course turns are `course_message` (`course_step.go:307`). D1 pinned 对话轮次 = `type IN ('prompt_sent','course_message')`. D2 reuses that 口径 **verbatim**.
- **Course page-turns are NOT events.** `RecordCourseStepViewed` (`course.sql:44`) upserts an int array; the render endpoint (`course_render.go:51`) holds no `course_session`, so a course-surface event there would violate `event_scope_ck` today. Hence DEC-4.
- **Existing course events** (`phase_advanced`, `course_finished`, `card_surfaced`, `course_message`) are session-scoped and only exist for runtime course sessions — too narrow to stand in for 完成课程节.
- **`evaluations`** carries `created_at` and one of `project_id` / `session_id` / `thread_id`, so "reports generated this week" and "latest report per student, any surface" are both directly queryable.
- **`agent.Report`** (`assess_report.go:152`) exposes `DepthAxis[].Level/LevelRange/Evidence` and `AutonomyAxis[].Level/Opportunity/Evidence` — every rule threshold and every quoted evidence string in §5 reads a real field.
- **Tiering**: `gateway.NewEvalKeyResolver` is the flagship, never-downgraded resolver; `Purpose` is a free string on the `llm_call` row (existing values include `assessment`, `classify`, `compose_journey`, `course_render`).
- **Seed reality**: migration 0029's events are all `now() - interval '<hours>'` — every one lands in the current week. Without §8.3 the screen shows 1 active day for everyone and a `+N` delta off a zero baseline.

---

## 4. Architecture

Three layers, each independently testable, in dependency order:

```
SQL aggregation           →  internal/store/queries/teacher_weekly.sql
   (per-student window facts, class counts, latest + previous report per student)
        ↓
Deterministic rule layer  →  internal/teacher/weekly.go   (PURE — no db, no http, no model)
   (tags, precedence, evidence excerpting, distribution, means, deltas, week labels)
        ↓
Composition layer         →  internal/agent/compose_weekly.go  (the ONE model call)
   (fact sheet in → {comment, per-card lead/action, depthNote, autonomyNote} out)
        ↓
HTTP                      →  internal/api/teacher_weekly.go   (GET read-only, POST spends)
```

The rule layer is the contract between SQL and the model: it consumes rows, emits a
fact sheet, and every number the screen shows is one of its outputs. The model can
only see the fact sheet, so it cannot introduce a student, a tag, or a number that
the rules did not produce.

---

## 5. The deterministic rule layer

### 5.0 One derivation per quantity

D1 already ships `teacher.DBadge` / `teacher.ABadge` (both `func(agent.Report) string`).
D2 needs the same quantities as numbers, for thresholds and for the class aggregates.
It must not derive them a second time. Two numeric helpers are extracted into
`internal/teacher`, and **`DBadge`/`ABadge` are refactored to call them**:

```
AMean(r agent.Report) (float64, bool)    // mean over supplied signals; false when none
DLevels(r agent.Report) (min, max int, ok bool)   // rated D dims; false when none
```

The rule thresholds, the bucketing, the class mean, and the badge strings then all
read one implementation. Refactoring `DBadge`/`ABadge` must leave their output
byte-identical — D1's badge tests are the guard.

### 5.1 Inputs (one struct per student)

```
StudentWeek {
  UserID, DisplayName, AvatarColor
  ActiveDays, Turns          // this window
  PrevActiveDays, PrevTurns  // the same elapsed offset last week
  ReportsThisWeek int        // evaluations created in this window
  Latest   *agent.Report     // nil when the student has never been assessed
  Previous *agent.Report     // nil when the student has fewer than 2 reports
  LatestSurface, LatestScopeID  // so a card can deep-link to D1's report view
}
```

### 5.2 Tag vocabulary (closed)

**Watch (需要建议) — evaluated in this order, first match wins:**

| code | label | fires when |
|---|---|---|
| `never_used` | 本周未使用 | `ActiveDays == 0` |
| `dropped_off` | 本周掉线 | `ActiveDays > 0` **and** `ActiveDays <= PrevActiveDays - 2` **and** `ReportsThisWeek == 0` |
| `outsourced_judgment` | 判断在外包 | `Latest != nil` **and** A-axis mean (supplied signals only) `<= 1.0` |
| `no_boundaries` | 从不设界 | `Latest != nil` **and** A3 (边界主权) `Level == 0` **and** `Opportunity != "not_supplied"` |
| `stuck_at_start` | 停在起步档 | `Latest != nil` **and** every rated D dim `<= L2` **and** at least half the rated dims are `L1` |

**Praise (值得表扬) — evaluated only if no watch tag fired, in this order:**

| code | label | fires when |
|---|---|---|
| `depth_up` | 深度升档 | `Previous != nil` **and** max D level in `Latest` > max D level in `Previous` |
| `more_autonomous` | 更愿意自己想 | `Previous != nil` **and** A mean(`Latest`) − A mean(`Previous`) `>= 0.5` |

**One card per student, maximum.** Watch takes precedence over praise because a
teacher who has ten minutes must see the student who has outsourced their judgment,
even in a week where that student also improved. Display order is the reverse —
praise renders first, per the binding caption 「先看值得表扬的，再看需要给建议的」.

A student with `Latest == nil` can only earn `never_used` or `dropped_off`. **No
report, no judgment** — this is 敢于空白, not an oversight.

### 5.3 Evidence (never model-written)

Each tag has one deterministic evidence source. The string is either excerpted
**verbatim** from the canonical report or is a bare statement of fact:

| code | evidence |
|---|---|
| `never_used` | `本周 0 天活动记录；上周 {PrevActiveDays} 天。` |
| `dropped_off` | `活跃天数 上周 {PrevActiveDays} 天 → 本周 {ActiveDays} 天，本周无新生成报告。` |
| `outsourced_judgment` | `A 轴 {mean}/5。` + the `Evidence` string of the lowest-level supplied A signal, verbatim. |
| `no_boundaries` | A3's `Evidence` string, verbatim. |
| `stuck_at_start` | The `Evidence` string of the lowest-level D dim, verbatim. |
| `depth_up` | `{prevBadge} → {latestBadge}。` + the `Evidence` of the dim that rose, verbatim. |
| `more_autonomous` | `A 轴 {prevMean} → {latestMean}。` + the `Evidence` of the most-improved A signal, verbatim. |

If the source `Evidence` string is empty, the card renders the factual half alone —
never a fabricated substitute.

### 5.4 The four stat cards

| # | label | value | unit | foot (static caption) |
|---|---|---|---|---|
| 1 | 本周活跃学生 | students with ≥1 event in window | `/ {classSize} 人` | 登录并有活动的学生 |
| 2 | 生成能力报告 | evaluations created in window, owned by class students | 份 | 来自项目、对话与课程 |
| 3 | AI 对话轮次 | events with `type IN ('prompt_sent','course_message')` | 轮 | 反映本周使用强度 |
| 4 | 完成课程节 | `COUNT(DISTINCT (user_id, course_id, payload->>'ordinal'))` over `step_viewed` | 节 | 平台内自学课程 |

Card 4 counts **distinct** steps so a page re-render cannot inflate it.

Each card carries `delta` = value(this window) − value(previous window) and a
direction. **Deviation from the binding design, taken deliberately:** the design's
`mk()` treats any non-negative delta as "up" and renders a green up-arrow, so a
delta of 0 would show as a green `+0`. We render `±0` in neutral grey instead. An
up-arrow on an unchanged metric is a false signal, and this screen's whole value is
that a teacher can trust the arrows.

### 5.5 班级思维维度

**D-axis distribution** — four buckets (起步 L1 / 发展 L2 / 熟练 L3 / 优秀 L4), colored
from `badgeColor.ts`, counted over students with a report. A student is bucketed by
the **highest level among their rated D dims** — i.e. the upper bound of the `DBadge`
range the roster already shows them. This is chosen for cross-screen consistency:
the badge a teacher reads in the roster and the bucket they land in here agree by
construction. `已评估 {n} 人`; students without a report are counted nowhere and
stated nowhere.

**A-axis class mean** — the mean of each rated student's own A mean (over supplied
signals only), to one decimal. **Computed in Go and only in Go.** The web renders the
string it is given. This is the direct lesson of D1's whole-branch review, where the
same mean derived twice (Go `%.1f` half-to-even vs JS `Math.round` half-up) showed
`4.2` and `4.3` on adjacent screens.

**A-axis delta** — the mean, over students holding ≥2 reports, of (A mean of `Latest` −
A mean of `Previous`), to one decimal. When no student has two reports the delta is
`—` (em-dash U+2014, D1's unrated marker) and the model is told there is no delta.

**Bucket changes** — for each student with ≥2 reports whose bucket differs between
`Previous` and `Latest`, the fact sheet carries `{name, from, to}`. This is what lets
the composed `depthNote` say 「上周熟练档 1 人 → 本周 2 人（吴桐升档）」 without the model
inventing it.

### 5.6 Week label

`第 {ISO week} 周（{M.D}–{M.D}）` over the full Monday–Sunday range, plus `asOf` = now,
matching the binding design's `weekLabel` / `asOf`. The data is week-to-date; the label
names the week, and `asOf` states the cut.

---

## 6. The composition layer

### 6.1 The call

One structured call per class per week. Provider resolved through
`gateway.NewEvalKeyResolver` (**flagship, never downgraded**), recorded as an
`llm_call` row with `Purpose: "class_weekly"`, mirroring how the assessor is metered.
Cost is recorded even when the output is subsequently rejected.

### 6.2 Input: the fact sheet

Built entirely from §5. Contains: class name and size, week label, the ordered card
list (`{userId, name, tagCode, tagLabel, evidence, kind}`), the D bucket counts, the
bucket changes, the A mean and delta, and the count of rated students.

**It does not contain the class-level usage counters** (turns, active-student count,
report count, course-step count). Those tick continuously; under DEC-2 the prose is
written once and never refreshed, so any prose that cited them would be wrong by
Wednesday. The stat cards speak for their own numbers. The one exception is a card's
own evidence line (`dropped_off` legitimately contains `5 天 → 2 天`), which the model
may restate in **that card's** `lead` only.

### 6.3 Output and validation

```
{ comment: string,
  depthNote: string,
  autonomyNote: string,
  cards: [ { userId, lead, action } ] }
```

Rejected — and treated as a failure per §6.5 — if any of:

- a `userId` appears that is not in the fact sheet, or a fact-sheet card is missing;
- the prose contains a person name that is not a student in the fact sheet;
- `comment`, `depthNote`, or `autonomyNote` exceeds its length cap;
- the prose contains a bare internal code (`/\b[DA][1-6]\b/`). Teachers *may* see the
  codes — they are labelled on the deep report screen — but 说人话 governs this
  screen's prose: it must name the behaviour, not the code.

The model writes **wording only**. It cannot add, drop, reorder, or reclassify a card;
the rule layer's output is what renders.

### 6.4 First-open-wins, and the gap it opens

Under DEC-2 the row is written once. The rule layer, however, runs live on every open,
so a card can first appear on Thursday — and that card would have no `lead` and no
「怎么开口」 at all, because Monday's row has no entry for it. A blank card on the one
screen whose purpose is telling a teacher what to say is worse than a stale comment.

**Top-up (DEC-6).** When the rule layer produces a card whose `(userId, tagCode)` key
is absent from the stored `cards` array, one small composition call — carrying only
those cards' facts — composes just them, and the results are **appended**. Existing
entries, `comment`, `depthNote`, and `autonomyNote` are never rewritten. DEC-2's
semantics hold: nothing already written is regenerated. A card whose student is no
longer flagged simply stops rendering; its stored entry is left in place, harmless.

### 6.5 Failure is never a wall

If the call errors, times out, or the output fails validation, the endpoint returns
**200 with `prose: null`**, cost still recorded. The screen renders every number and
every card with its tag and its evidence, under 「本周点评暂未生成」. The teacher's ten
minutes survive an LLM outage — the evidence-bearing part of this screen never
depended on the model.

**No cards and no rated students → no call at all.** An empty class has nothing to say
about; spending a flagship call to be told so is waste.

---

## 7. Storage

Migration numbers are `0031`, `0032`, `0033` (latest on main is `0030`).

### 7.1 Migration 0031 — `event.course_id` + `step_viewed`

```sql
ALTER TABLE event ADD COLUMN course_id uuid REFERENCES course(id) ON DELETE CASCADE;
CREATE INDEX event_course_created_idx ON event (course_id, created_at);
ALTER TABLE event DROP CONSTRAINT event_scope_ck;
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (num_nonnulls(project_id, session_id, thread_id, course_id) >= 1) NOT VALID;
```

`NOT VALID` for the same reason 0024/0025 gave: pre-existing unscoped rows have
nothing to backfill from and stay grandfathered, while every new insert is enforced.
The Down mirrors 0025's shape — delete the course-only-scoped rows first, because the
restored narrower CHECK would otherwise reject them and abort the whole Down.

`course_render.go` emits `{surface:'course', type:'step_viewed', course_id, payload:{ordinal}}`
beside the existing `RecordCourseStepViewed` call, **best-effort** like the metering
write already there: a failed event must never fail a student's page render.

### 7.2 Migration 0032 — `class_weekly_prose`

```sql
CREATE TABLE class_weekly_prose (
  class_id      uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  week_start    date NOT NULL,
  comment       text NOT NULL,
  depth_note    text NOT NULL,
  autonomy_note text NOT NULL,
  cards         jsonb NOT NULL DEFAULT '[]',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (class_id, week_start)
);
```

`week_start` is the UTC Monday date, so the PK *is* the first-open-wins lock. Insert
is `ON CONFLICT DO NOTHING`; the top-up is the only writer that ever touches an
existing row, and it only appends to `cards` (and bumps `updated_at`).

**Concurrency:** two teachers opening simultaneously may both call the model; the
loser's insert is discarded and it returns the winner's row. Cost is bounded at one
wasted call per class per week, which is cheaper than a lock.

### 7.3 Migration 0033 — seed extension

Extend 0029's seed backwards so the screen is honestly demonstrable on a fresh dev DB:
spread this week's events across several days per student, add a previous-week arm
(so deltas are real and `dropped_off` can fire), and give at least one student a second
older evaluation (so `depth_up` / `more_autonomous` / bucket-change / A-delta all have a
baseline). 罗一 keeps zero events — `never_used` must have a real subject.

---

## 8. HTTP

### 8.1 `GET /api/v1/classes/{id}/weekly-report`

Read-only. **Makes no model call under any circumstance.**

```
WeeklyReportDTO {
  weekLabel, weekStart, weekEnd, asOf
  classSize
  stats:    [ { key, label, value, unit, foot, delta, deltaDir } ]   // 4, in design order
  praise:   [ Card ]
  watch:    [ Card ]
  depth:    { buckets: [ { code, label, count } ], ratedCount, note }
  autonomy: { mean, delta, ratedCount, note }
  comment:  string | null
  proseReady: bool
}
Card {
  userId, displayName, avatarColor
  tagCode, tagLabel, kind            // "praise" | "watch"
  evidence                           // deterministic, always present
  lead, action                       // "" until composed
  hasReport, reportSurface, reportScopeId
}
```

`note`, `comment`, `lead`, `action` are `""`/`null` until the prose exists.

### 8.2 `POST /api/v1/classes/{id}/weekly-report/prose`

The only endpoint that spends. Returns the same `WeeklyReportDTO`. Behaviour:
row exists and covers every current card → return it, no call. Row absent → compose,
insert `ON CONFLICT DO NOTHING`, return. Row exists but cards are missing → top-up (§6.4).
No cards and no rated students → return with `prose: null`, no call.

### 8.3 Tenancy

Both handlers are `teacherOrAdmin` + `assertTeacherOwnsClass`, exactly as D1. Every
authorization failure is an existence-hiding 404; a student hitting either gets 403
from the role guard. No new tenancy primitive is introduced.

---

## 9. D1 carry-forwards closed here

**`weekWindow` moves to `internal/teacher`** as an exported pure function with its own
unit test (closing D1's "no isolated `weekWindow` test" carry-forward), and gains
`PrevWindow(now)` = last Monday 00:00 UTC → the same elapsed offset. D1's handler keeps
calling it. The roster's 本周活跃 and the weekly report's window are then *literally the
same function* — the divergence class D1's whole-branch review caught cannot recur here.

**"Latest report" becomes cross-surface.** D1's roster reads the latest *project*
evaluation only. A class distribution built on that would silently omit every
course/chat-only student from 「已评估 N 人」 — the same student would be 已评估 on one
screen and unrated on the next. One query returns each student's latest evaluation
across all three scopes, and **both** the roster and the distribution read it.

**Cost, stated plainly:** this modifies `ListClassRosterReport` and
`GetLatestProjectScoresForStudent` — queries D1 shipped — and their tests. Any task
touching them runs the full Go package, never a `-run` subset.

---

## 10. Web

`ClassDetailView` keeps the class header and gains two sub-tabs: **周报** (default) and
**全部学生**. D1's roster table extracts unchanged into `ClassRosterTable.tsx`; the new
`ClassWeeklyView.tsx` renders the report. Card actions reuse D1's existing
`onOpenStudent` / `onOpenReport` handlers — no new navigation state beyond the sub-tab.

The distribution's bucket colors come from `badgeColor.ts`. The A mean and delta are
rendered as the strings the server sent; **the web derives no score, mean, or badge.**

Empty states, verbatim: no cards at all → 「本周没有需要特别关注的学生」; no rated students
→ the distribution block renders 「暂无可计入的证据」 and the mean renders `—`; prose absent
→ 「本周点评暂未生成」. A failed fetch renders a distinct error with a retry — never an
empty-looking class (D1 whole-branch finding 4).

---

## 11. Testing

**`internal/teacher` (pure, fast):** a table test per tag including its non-firing
boundary; precedence (watch beats praise; watch order); the unrated path (no report →
only usage tags); the A mean excluding `not_supplied`; bucketing by the badge's upper
bound; delta arithmetic including the no-baseline `—`; `WeekWindow`/`PrevWindow`
including a DST-irrelevant UTC pin and a Monday-boundary case.

**`internal/agent`:** fact-sheet construction; each validation rejection (unknown
userId, missing card, bare code, over-length); the success path against a stubbed
provider.

**`internal/api` (full package, testcontainers):** tenancy for both endpoints
(cross-owner → 404, student → 403); GET makes no model call; POST generate-once;
concurrent insert returns the winner; the top-up appends without rewriting;
the failure path returns 200 with cards and no prose; the empty class makes no call;
the seeded class produces the expected cards.

**Web:** the two sub-tabs; card rendering with and without prose; all three empty
states; the fetch-error state; and an assertion that the rendered mean is the server
string.

Full suites, no `-run` subsets — this spec touches queries, migrations, and a seed.

---

## 12. Invariants this spec must not break

- 单向只读 — no teacher action reaches a student.
- 每个判断带证据 — every card carries evidence excerpted from the record; no evidence, no card.
- 敢于空白 — absent evidence renders 「暂无可计入的证据」 / `—`, never a fabricated substitute.
- RL-5 — the two axes never combine into a total. The D distribution and the A mean are separate readings; no composite class score exists anywhere in this spec.
- 评估走旗舰，绝不降级 — the composition call resolves flagship, is metered, and is never retried at a lower tier.
- 客户端绝不直连模型 — the only model call is server-side, behind the POST.
- 说人话 — composed prose names behaviours, not internal codes (§6.3).
- 一个生产者，多个投影 — this spec computes **no** new assessment. It projects `agent.Report` and the event stream, nothing more.

# 思维印记 · E2E Pre-Deploy Test Plan

> **Purpose:** a repeatable, breadth-first **live** end-to-end pass that walks every
> user-facing function of the current product against the real stack, run **before
> deploying to a server**. Not a CI gate; a release-readiness gate.

**Author role:** test engineer. **Date:** 2026-07-25.
**Supersedes:** the stale coverage of `apps/web/e2e/golden-path.spec.ts` (one journey,
written before the whole assessment / teacher-end / parent-report program, 课程, 聊天,
and 成长报告 existed).

---

## 1. Strategy (decided)

- **Live-only.** Every model-touching journey runs through the **real DeepSeek gateway**.
  No fake-provider seam. This is the point: we want the API key exercised end to end
  before we ship.
- **Breadth-first.** Every function in the inventory (§3) is walked by **at least one**
  live journey. A short list of critical spines gets a deep walk; everything else gets a
  thin smoke.
- **Structural assertions, never prose.** The model is nondeterministic, so we assert
  *shape and behavior*: an envelope pinned a process-tree node, an evaluation rendered a
  real rubric, a report's prose is non-empty **and passes the leak/numberless guards**.
  We never assert exact wording.
- **State-building spine, then read the state.** Projections (成长报告, teacher signals,
  weekly report, parent report) can only be validated after a student has generated data.
  The suite is ordered so the **student legs build state**, then the **teacher/admin/parent
  legs read it** — the same cross-role chaining `golden-path` already uses.
- **Two lenses, one stack.** Coverage is written twice from different angles: **function
  suites** (§4) assert each endpoint/surface works; **journey scripts** (§5) assert the *loop* —
  an actor arrives, acts, and the right layout/data/status appears for them and everyone
  downstream. The journeys are the release-confidence spine; the function suites are the
  completeness backstop that catches anything a journey walks past.
- **This does not replace the unit/integration suites.** Those (Go `internal/*`, web
  vitest, contracts) stay the fast gate and run **first**. E2E is the final live layer.

### Non-goals
- No CI/PR wiring (there are no PRs — this is a pre-deploy checklist run by hand).
- No fake-LLM determinism, no coverage metrics, no load/perf testing.
- Real audio playback and multimodal are **not** headless-assertable — they move to the
  manual exploratory addendum (§7).

---

## 2. Environment & tooling

Extends the existing harness **`apps/web/e2e/run-stack.sh`** (throwaway Postgres →
migrate+seed → real Go API on :8080 → web dev on :5173 → Playwright), plus
`apps/web/e2e/playwright.config.ts` and `helpers.ts`.

- **Seed:** admin (`admin@demo.mindimprint.local`), Phoebe student, one seeded class,
  one info-literacy course (per current seed migrations).
- **Key:** `apps/api/.env.local` with a real `DEEPSEEK_API_KEY` (see `RUNBOOK.md`).
- **Deterministic specs** (auth, registration, tenancy) run with **no key**.
- **Live specs** need the key; flagship evaluation can take up to ~90s — timeouts are
  generous and `retries:1` absorbs one summon-variance flake.

### Cost of one full run
A full live pass makes roughly **30–50 model calls**, of which a handful are **flagship**
(`deepseek-v4-pro`): each project evaluation, each course terminal assessment, each weekly
report prose, and two parent-report prose. Budget accordingly; this is a real spend, which
is why it's a pre-deploy gate, not a per-change loop.

---

## 3. Function inventory (what must be covered)

Grounded in the current route table (`apps/api/internal/api/api.go`) and web surfaces.

### Student (rail: 聊天 · 课程 · 工作室 · 成长报告 · 设置)
| Surface | Functions |
|---|---|
| Auth/onboarding | signup, verify-email, signin, signout, register-with-join-code |
| 工作室 (project) | create (funnel) · onboarding→framing→perspectives→structure→writing→review · turn (chaperone) · card activate/skip/submit · 6 card primitives (annotate/compare/matrix/scale/sort/toulmin) · materials ingest + source-open · edit buffer · commit snapshot · gate attest · spot-check · order review (整稿体检) · self-score · reflection · declaration sign · finish · assessment (你的思维印记) · journey reopen |
| 课程 | list · get · progress get/put · render step · session start/restart/get · ask (问印记) · advance (floor gate) · card submit/skip · terminal assessment |
| 聊天 | threads list/create · messages · turn · card submit/skip · assessment |
| 成长报告 | history (学习记录) · cards (工具卡) · ability (能力素养 radar) |
| 语音 | TTS · ASR |
| 设置 | signout |

### Teacher (console: 概览 · 班级 · 教师 · 导入)
| Surface | Functions |
|---|---|
| Onboarding | invite-code signup |
| Classes/roster | create class · list · get · patch · remove enrollment · roster-report |
| Drill-down | student detail · student report by `{surface}/{scopeId}` · evidence map (client-derived) |
| 班级周报 (D2) | GET weekly-report (numbers, no spend) · POST prose (one spend, first-open-wins) |
| 家长报告 project (E1) | GET · POST prose · printable · leak/numberless guard |
| 家长报告 stage (E2) | GET `{weekStart}` · POST prose · printable · guard |

### Admin
overview · teacher-invites create/list · CSV import · list teachers · assign/remove class teacher.

### Cross-cutting
tenancy (foreign teacher/parent GET → 404) · cost-metering (GET zero `llm_call`, POST one) ·
restraint law (card proposed, never auto-opened) · RL-5 (no A-number / level code reaches parent).

---

## 4. Test suites (breadth-first, live)

Each spec lists **state → steps → pass criteria**. Filenames extend `apps/web/e2e/`.

### Deterministic (no key) — run first, fast

**`auth.spec.ts`** *(exists)* — seeded admin login → 概览; wrong password stays; logged-out shows auth.
**`registration.spec.ts`** *(exists)* — invalid join code rejected (org invariant).
**`tenancy.spec.ts`** *(new)* —
- State: two teachers in two classes (seed or register), each with a student.
- Steps: teacher A requests teacher B's student detail / student report / parent report / weekly-report.
- Pass: every cross-class GET returns **404** (tenancy hidden), never 200/403 with data.

### Live student spine (builds state)

**`student-project.spec.ts`** *(new — extends golden-path's student leg to the full walk)* —
- State: fresh student registered with a join code (empty).
- Steps: funnel create → onboarding→framing→perspectives→structure→writing→review; drive at
  least one **live chaperone turn** that yields a `summon_card`; **open** the card (restraint:
  assert it did **not** auto-open), fill, submit → node pins; send a refeed turn → AI reply
  renders; commit a snapshot → order **整稿体检**; self-score → reflection → sign declaration →
  finish → generate assessment.
- Pass: card proposed-not-opened; SIFT/CRAAP envelope pins a process-tree node; refeed reply
  streams without error; 整稿体检 returns review items (advice, never written to draft);
  assessment dialog **你的思维印记** renders a real rubric (per-dim levels + evidence).
- Note: this is the primary **state builder** — the seeded/this student now has a completed,
  evaluated project the teacher/parent legs read.

**`student-course.spec.ts`** *(new)* —
- State: seeded info-literacy course.
- Steps: list → start session → walk 演示→引导 steps → submit **and** skip a course card →
  ask 问印记 (live) → hit `advance` with the floor **unmet** (expect blocked, **zero** new
  model call) → satisfy floor → advance → 独立 → 练一手 transfer self-check → 回看 →
  completion; then **restart** session.
- Pass: advance blocked until structural floor met; skip counts as satisfied (offer ≠ wall);
  ask returns a live reply; terminal assessment renders; restart clears session + progress.

**`student-chat.spec.ts`** *(new)* —
- State: fresh/seeded student.
- Steps: create thread → send a plain turn → send a message containing a **link** → classifier
  offers a CRAAP card in-thread → confirm-open → submit → generate chat assessment.
- Pass: coach reply renders (planner off, `reply` output); link triggers a **surface_card**
  offer (confirm-to-open, not auto); thin card submit records; assessment renders.

**`student-growth.spec.ts`** *(new — reads state built above)* —
- State: student with ≥1 completed project (from `student-project`) and, ideally, ≥2 sessions.
- Steps: open 成长报告 → 学习记录 / 工具卡 / 能力素养 tabs.
- Pass: 学习记录 lists the completed project; 工具卡 shows the submitted card grouped by
  category; **能力素养 radar renders 6 spokes** (D-depth) with no NaN, and reflects
  cross-session aggregate — plus the **empty state** (a fresh student shows 敢于空白, no crash).

**`student-voice.spec.ts`** *(new — smoke)* —
- State: course AskPanel (问印记) with voice affordance.
- Steps: trigger TTS on a reply; exercise ASR entry.
- Pass: TTS endpoint returns audio bytes without error; ASR endpoint reachable. **Risk:** per
  N5a some rebuilt surfaces render voice **inert** — if so, this spec asserts the *endpoint*
  works and flags the UI wiring gap rather than failing the release (see §7).

### Live teacher/admin/parent legs (read the built state)

**`teacher-class.spec.ts`** *(new — merges golden-path's teacher+admin tail, refreshed)* —
- State: after student legs ran (roster has signals).
- Steps: admin mints teacher invite → teacher signs up → creates class → reads join code →
  (student already joined in spine) → open roster → drill into student detail + evidence map →
  patch class → remove-then-reconfirm an enrollment.
- Pass: roster row shows the student with **non-zero integer** signal cells; drill-down renders
  the evidence map from real events; admin **概览** stat cards show non-zero real counts.

**`teacher-weekly-report.spec.ts`** *(new — D2)* —
- State: class with ≥1 active student this week.
- Steps: GET weekly report (numbers) → assert **no `llm_call` added** → POST prose → assert
  **exactly one flagship `llm_call`** added → re-open (GET again, then POST again) → assert
  **still one** (first-open-wins) → also open a **thin/empty** week.
- Pass: live numbers correct vs seeded events; prose non-empty, is warm rule-layer wording;
  cost invariants hold; thin week renders 敢于空白 without spend beyond the one compose.

**`parent-report.spec.ts`** *(new — E1 project + E2 stage)* —
- State: student with a completed evaluated project (project mode) and a usage week (stage mode).
- Steps: teacher opens 家长报告·项目 → GET (no spend) → POST prose (one flagship spend) →
  re-open (no new spend) → printable overlay; repeat for 家长报告·阶段 (`{weekStart}` = current).
- Pass: printable parent projection renders; prose non-empty; **leak guard** — output text
  matches **none** of `\bL[1-4]\b`, `\b[DA][1-6]\b`, `given_*`, `SOLO`; **RL-5 numberless** —
  no A-axis number and no ability level number anywhere on the parent surface; first-open-wins.

**`admin-import.spec.ts`** *(new)* —
- State: fresh org / seeded admin.
- Steps: 导入 → upload a small CSV roster → confirm import → verify students appear on the
  target class roster; generate + list a teacher invite; assign a teacher to a class, then remove.
- Pass: imported students enroll (school_id + enrollment in one tx — no orphan accounts);
  invite generated and listed; assignment reflected then reverted.

### Cross-role spine (keep)

**`golden-path.spec.ts`** *(exists — refresh selectors, keep as the one continuous spine)* —
admin → teacher → student project → evaluation → teacher signals → admin overview. Retained as
the single smoke that proves the whole cross-role lifecycle wires together in one run.

---

## 5. User-journey scripts (the full loop)

§4 proves each **function** works. This section proves the **loop**: an actor arrives,
does a thing, and the *right layout/data/status appears in the right place* — for them and
for the people downstream. These are narrative walkthroughs; each step names the **actor**,
the **action**, and the **expected observable**. They reuse §2's stack and §8's helpers, and
they are the primary release-confidence artifact — a real cohort's path through the product.

Assertion note: observables below are described at UI-behavior altitude (what a human sees);
exact selectors/copy get pinned when the spec is written. Anchors verified against current UI:
new student lands on **工作室** with **"还没有项目"**; 成长报告 empty shows **"还没有"**.

### J1 — New student, first arrival → first value  *(single actor)*
`journey-new-student.spec.ts`

| # | Actor · Action | Expected observable (layout / data / status) |
|---|---|---|
| 1 | Student registers with a class join code | Auto-signed-in; lands on **工作室**; rail shows 聊天·课程·工作室·成长报告·设置 |
| 2 | Looks around before doing anything | 工作室 shows **"还没有项目"** empty state; 成长报告 tabs all show **"还没有"** empty states; **no crash on any empty tab** |
| 3 | Creates a project via the funnel | Onboarding prompt **"你想搞懂什么？"** appears; project now exists |
| 4 | Sends first chaperone turn (live) | AI reply renders; a `summon_card` may be **proposed** — and is **not auto-opened** (restraint law) |
| 5 | Opens + submits the card | Card sheet opens only on click; on submit → **process tree grows a pinned node**; chat shows **"已完成 · 已钉到过程树"** |
| 6 | Sends a refeed turn | AI reply rides the completed-card history; reply renders without error |
| 7 | Opens 成长报告 again | 学习记录 now lists **this in-progress project** (state changed from empty → populated) |

**Loop proven:** empty-state → action → the same surface now reflects the action; the process
tree and 成长报告 update live from what the student just did.

### J2 — Finish a course → where does it surface  *(single actor)*
`journey-course-loop.spec.ts`

| # | Actor · Action | Expected observable |
|---|---|---|
| 1 | Student opens 课程, starts a session | 演示 phase renders; session status = active |
| 2 | Walks 演示→引导, submits one card, **skips** one | Both advance; skip is accepted (offer ≠ wall) |
| 3 | Asks 问印记 (live) | Live reply in the ask panel |
| 4 | Reaches 独立 → 练一手 → 回看 → completes | Course status flips to **完成**; **回看** view is reachable |
| 5 | Opens 成长报告 | 学习记录 reflects the course activity; **工具卡** shows the dispositioned card under its category; 能力素养 radar unaffected-or-updated but **never NaN** |
| 6 | Restarts the session | Session + progress **cleared**; course walkable again from the top |

**Loop proven:** course completion is not a dead end — it lands in 成长报告 and 回看, and a
restart truly resets state.

### J3 — Finish a chat → where does it surface  *(single actor)*
`journey-chat-loop.spec.ts`

| # | Actor · Action | Expected observable |
|---|---|---|
| 1 | Student opens 聊天, creates a thread | Empty thread; coach-alone (no project context) |
| 2 | Sends a plain message (live) | `reply` renders; **no** card auto-opens |
| 3 | Sends a message with a **link** | Classifier offers a CRAAP card **in-thread**; confirm-to-open, not auto |
| 4 | Opens + submits the offered card | Thin card submit recorded in the thread |
| 5 | Generates chat assessment | Assessment renders for the thread |

**Loop proven:** the standalone chat surface produces its own dispositions + assessment,
isolated from the project surface.

### J4 — Full cohort loop: 1 teacher + 4 students in different states  *(the capstone)*
`journey-cohort.spec.ts` — the headline pre-deploy journey. It proves **registration
correctness, per-student attribution, live propagation, and mixed-usage aggregation** in one run.

**Setup**
| # | Actor · Action | Expected observable |
|---|---|---|
| 1 | Admin mints a teacher invite | Invite code `T-XXXXXX` shown |
| 2 | **Teacher** registers with the invite | Lands on teacher console; 班级 tab present |
| 3 | Teacher creates a class, reads the join code | Join code `XXXX-XXXX` shown; roster empty |
| 4 | **4 students** register with that one join code | Each auto-signs-in to 工作室; each belongs to **this class + this school** (org invariant) |

**Students do deliberately different work** (this is the state spread)
| Student | Does | Their end status |
|---|---|---|
| A (rich) | Full project → card → refeed → **finish + evaluation** | completed, evaluated |
| B (course) | Completes a course session | course done |
| C (chat) | One chat thread + card + assessment | chat done |
| D (idle) | Registers, does **nothing** | zero activity |

**Teacher sees the data update correctly**
| # | Actor · Action | Expected observable |
|---|---|---|
| 5 | Teacher opens the class roster | **4 rows**; A has **non-zero** signals, D has **zero**; B/C show their respective activity — **each student's work on their own row only (attribution)** |
| 6 | Drills into A | Evidence map renders from A's real events; A's evaluation reachable |
| 7 | Drills into D | Renders **敢于空白** cleanly (no data, no crash) |
| 8 | Generates 班级周报 | Live numbers match the cohort; the **rule layer flags the right students**; the idle student handled without breaking the report; **exactly one** flagship `llm_call` for the prose |
| 9 | Opens 家长报告 for A (project + stage) | Printable parent projection; prose passes **leak guard** (`\bL[1-4]\b`/`\b[DA][1-6]\b`) and is **RL-5 numberless** |
| 10 | **Admin** opens 概览 | Stat cards reflect **+1 teacher, +1 class, +4 students** with non-zero real counts |

**Loop proven end-to-end:** four people register into one org, do four different things, and the
teacher/admin surfaces show **each person's own data, correctly attributed and aggregated** —
the full "students work → teacher sees the truth" circuit that a real deployment must get right.

---

## 6. State-matrix coverage

The states you called out, mapped to the spec that exercises each:

| Dimension | States | Covered by |
|---|---|---|
| Student lifecycle | new/empty · mid · completed+evaluated | project + growth |
| Card disposition | filled · **skipped** · proposed-not-opened | project + course + chat |
| Course progress | not-started · floor-unmet (blocked) · completed · restarted | course |
| Chat | plain thread · link→card offer | chat |
| Cross-session | single vs multi-session ability merge | growth |
| Class | empty · thin · rich (signals) | teacher-class + weekly + tenancy |
| Report generation | not-generated · generated · re-opened · thin (敢于空白) | weekly + parent |
| Access control | in-class vs foreign | tenancy |
| Cost | GET zero-spend · POST one-spend | weekly + parent |

---

## 7. Run order (the pre-deploy checklist)

Run top to bottom; stop and fix on the first hard failure. Two tracks share the stack: the
**function suites** (§4, completeness backstop) and the **journey scripts** (§5, loop
confidence). The journeys are the release-confidence spine; the function suites catch anything
a journey walks past.

- [ ] **0. Unit/integration gate (no key):** full Go (`internal/*`), web vitest, contracts — all green.
- [ ] **1. Deterministic E2E (no key):** `smoke` · `auth` · `registration` · `tenancy`.
- [ ] **2. Journey — single-actor loops (live):** `journey-new-student` (J1) → `journey-course-loop` (J2) → `journey-chat-loop` (J3). Proves each surface's own arrive→act→see loop.
- [ ] **3. Journey — full cohort loop (live):** `journey-cohort` (J4). The capstone: 1 teacher + 4 students → work → teacher/admin see correctly-attributed data.
- [ ] **4. Function backstop — student (live):** `student-project` · `student-course` · `student-chat` · `student-growth` (anything J1–J4 didn't assert directly).
- [ ] **5. Function backstop — teacher/admin (live):** `teacher-class` · `teacher-weekly-report` · `parent-report` · `admin-import`.
- [ ] **6. Cross-role smoke:** `golden-path`.
- [ ] **7. Voice smoke:** `student-voice`.
- [ ] **8. Manual exploratory addendum (§8).**
- [ ] **9. Green? Tag the build and deploy.**

One command per group via `bash apps/web/e2e/run-stack.sh <spec…>`; the harness boots/tears
down the throwaway stack each run. Note: J4 already builds a rich cohort, so steps 4–5 can seed
off J4's state or run standalone — either way each function is asserted at least once.

---

## 8. Live-model & environment risks + manual addendum

Things E2E can't fully prove headless — verify by hand before deploy:

- **Summon variance** — `tool_choice=auto`; occasional "model declined to summon" is variance,
  not a break. Specs nudge-once + `retries:1`. If a summon spec fails twice, investigate.
- **Voice** — TTS/ASR endpoints are covered; **actual audio playback** and the live speaker-id /
  mp3-vs-pcm path (N5a open) need a manual listen. Confirm 问印记 按住说话 records + plays.
- **Multimodal input** — deferred in-product; not automated. Manually confirm any image/file
  affordance behaves (or is correctly absent).
- **Email verification** — confirm the dev signup→verify path matches the server config you'll
  deploy with (auto-verify in dev vs real SMTP in prod). A prod SMTP misconfig won't surface in
  the throwaway stack.
- **Secrets hygiene** — spot-check server logs/responses during the run: **no** key, DSN, or
  session secret in any rendered error or log line.
- **Visual rendering** — reports/printable overlays: eyeball one 家长报告 and one 你的思维印记 for
  layout, not just DOM presence.

---

## 9. What to build (delta from today's `apps/web/e2e/`)

Existing: `smoke`, `auth`, `registration`, `golden-path`, `helpers.ts`, `run-stack.sh`, `RUNBOOK.md`.

**Journey specs (§5 — build these first, they carry release confidence):**
`journey-new-student` · `journey-course-loop` · `journey-chat-loop` · `journey-cohort`.

**Function-backstop specs (§4):** `tenancy` · `student-project` · `student-course` ·
`student-chat` · `student-growth` · `student-voice` · `teacher-class` ·
`teacher-weekly-report` · `parent-report` · `admin-import`.

Extend `helpers.ts` with: `registerStudent`/`registerTeacher` (already partly present),
`createProjectViaFunnel`, `walkStation`, `fillAndSubmitCard`, `startCourseSession`,
`completeCourse`, `openThread`, `openReport`, `readRosterRow` — so each spec reads as a journey,
not a pile of selectors. `journey-cohort` especially needs `registerStudent` to run 4×.
Refresh `golden-path` selectors against current DOM (e.g. the student workspace tab is **工作室**,
not the stale **批判思维**).

Update `RUNBOOK.md` to document the §7 run order and the §8 manual addendum.

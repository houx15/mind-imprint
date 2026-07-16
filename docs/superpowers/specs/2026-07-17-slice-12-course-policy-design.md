# Slice 12 — Course policy + surface alignment · design

| | |
|---|---|
| **Slice** | 12 (whole-product refactor #2), depends on 3 (card contract) + 10 (assessor) |
| **Keystone** | Coach-as-script-executor: the 课程 surface runs on the agent runtime under Course policy |
| **Authority** | `docs/2026-07-11-agent-spec.md` §5.3 / §5.5 / §5.6 (technical) · `docs/design/思维印记_工作区.dc.html` lines 215–410 (**binding UI**) · `docs/2026-07-11-product-spec.md` (pedagogy, red lines) |
| **Roadmap row** | 12 — "Course policy + surface alignment … Mostly configuration." |

---

## 1. Where we are

课程 is the last pre-runtime surface. Today it is a linear step player:
`course` / `course_step` / `course_progress` (migrations 0011–0013), an LLM page
renderer (`agent.RenderCourseStep` → teaching/challenge templates, cached, with a
mandatory authored fallback), and a React player that pages through ordinals and
PUTs progress. There is no agent runtime in it: no dialogue, no cards, no events,
no phase concept. The 成长报告 (Slice 10) cannot see course work at all, because
course work never enters the event stream.

The binding design already contains the missing surface. `dc.html` 215–410 draws
the player as a **content stage + 问印记 ask panel** (330px, collapsible to a 46px
rail, a `正在看：{{ askContext }}` pill, authored ask chips, a text composer, and a
按住说话 voice button), and labels every page with a **`pgStage`** stage name. That
stage label is the phase; the ask panel is the dialogue. Neither is built.

This slice wires the runtime under the surface the design already specifies.

## 2. Decisions

### DEC-12.1 — Phases wrap steps (the layer is additive)

`course_step` rows, `authored_content`, the teaching/challenge renderer, and the
`course_render` cache survive **unchanged** as authored *content*. The skill's
phases are a new layer *above* them: **pages are the content unit, phases are the
runtime unit.**

A course skill is authored as C5 config (`kind:"course"`), its contracts being
session phases in a **binding order** expressed as a linear `requires` chain. This
slice authors one, for the one seeded course (`…0000c1`, 批判性思维 ·
「一条网络信息，该不该信」, 3 steps):

| phase | wraps steps | cards | floor (machine) | soft condition (coach judges) |
|---|---|---|---|---|
| `demonstrate` 演示 | 0, 1 | — | `steps_viewed [0,1]` | 学生能说出「随手一信」的风险 |
| `guided` 引导 | — (card is the page) | `craap` | `card_dispositioned craap` | 学生对这条说法做完了一次真实的溯源 |
| `independent` 独立 | 2 | — | `student_turns_at_least 1` | 学生给出判断并说得出理由 |
| `reflect` 回看 | — | — | `student_turns_at_least 1` | 学生说得出这次学到的方法 |

`guided` and `reflect` wrap no step: their page is the card sheet / the dialogue
itself. A phase with `steps: []` is legal and renders its phase page from the
skill's authored `page` block (title + subtitle + body), not from `course_step`.

**No migration touches the course content tables.** `course_progress` also stays
as-is (it is page position); phase position lives on `course_session` (§3).

### DEC-12.2 — Advance = structural floor (Go) + coach judgment (model)

agent-spec §7 open question #4 is real: "the goal of guided practice is met" is
softer than a machine gate. We split it, and are explicit about which half holds
which guarantee:

- **The floor is enforcement, below the policy** (agent-spec §6 discipline). A new
  closed set of **course floor kinds** — `steps_viewed` · `card_dispositioned` ·
  `student_turns_at_least` — is *named* in `skills` (config validation) and
  *evaluated* in `agent`, mirroring Slice 4's `MachineKinds` split. The coach
  physically cannot advance past an unmet floor: `RunCourseStep` checks it before
  the model is called and returns without one. This is what makes §5.3's hard
  limit ("cannot reorder or skip phases") structural rather than a prompt request.
- **The soft condition is the coach's.** With the floor met, the coach is given the
  phase's authored condition string and decides: `advance` (the phase goal is met)
  or `reply` (what is still owed, in the ask panel). This is the pedagogy, and it
  is model judgment — as DEC-3 has it for gates, machine judgment alone never
  declares a phase pedagogically complete.

Advance is **forward-only and one phase at a time**: an `advance` output naming
anything other than the current phase's single successor is rejected by the
runtime, not just discouraged by the prompt.

**A floor must never dead-end the student.** The `guided` floor is
`card_dispositioned`, not `card_completed`: a card is an *offer* (铁律 2), so a
student who skips it must still be able to move on — a floor that only a completed
card can satisfy would turn the offer into a wall. The floor therefore guarantees
the student **met** the card, not that they complied; the skip is recorded as a
signal (过程即数据) and the coach's soft-condition judgment carries the quality
question. Same reasoning for `steps_viewed`: it asserts the page was reached, not
that it was understood.

**Floor inputs are server-side.** `steps_viewed` reads
`course_progress.completed_ordinals` — the existing, already-persisted page
position — never a client-reported list. A floor the client can assert is not a
floor.

### DEC-12.3 — `advance` becomes a typed output (this slice's C3 seam)

`advance` is already in the C3 `Verb` enum but has never had an `AgentOutput`
member — exactly the gap `reply` had before Slice 11. Add, to both sides of the
seam:

- Zod (`packages/contracts/src/agentOutput.ts`), a 7th union member:
  `z.object({ type: z.literal("advance"), to: z.string().min(1) })` — anchor-free.
- Go (`enforcement.AgentOutput`): a `To string \`json:"to,omitempty"\`` field;
  `"advance": true` in `validOutputTypes`; `case "advance"` requiring a non-empty
  `To`.

Banned-phrasing runs on `advance` like everything else (its body is empty, so it
passes trivially — the check stays uniform). **No OutputCheck echo pass**: as in
Chat, Course has no draft to echo.

Slice 4's `planner.Advance` (a Go function, Project-only) is untouched and
unrelated: in Course the *verb* belongs to the coach (§5.3), in Project the
planner holds it. Both remain true; they are different code paths.

### DEC-12.4 — Session scope: sibling tables, mirroring 0022

Migration `0023` (additive, reversible):

```sql
CREATE TABLE course_session (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    course_id  uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
    skill_id   text NOT NULL,
    phase      text NOT NULL,
    status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, course_id)
);

CREATE TABLE course_message (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES course_session(id) ON DELETE CASCADE,
    phase      text NOT NULL,
    role       text NOT NULL CHECK (role IN ('student','assistant')),
    content    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX course_message_session_created_idx ON course_message (session_id, created_at);

ALTER TABLE material       ADD COLUMN session_id uuid REFERENCES course_session(id) ON DELETE CASCADE;
ALTER TABLE card_instances ADD COLUMN session_id uuid REFERENCES course_session(id) ON DELETE CASCADE;
CREATE INDEX material_session_created_idx       ON material (session_id, created_at);
CREATE INDEX card_instances_session_created_idx ON card_instances (session_id, created_at);

ALTER TABLE material       DROP CONSTRAINT material_scope_ck;
ALTER TABLE card_instances DROP CONSTRAINT card_instances_scope_ck;
ALTER TABLE material       ADD CONSTRAINT material_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id, session_id) >= 1);
ALTER TABLE card_instances ADD CONSTRAINT card_instances_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id, session_id) >= 1);
```

Course/Chat/Project stay clean siblings — each surface owns its scope column and
its store seam, and existing rows satisfy the widened CHECK unchanged. The
`intervention` table is **not** touched: the course coach's output is a
`course_message`, not an anchored intervention (the same call DEC-11.2 made).

`UNIQUE (user_id, course_id)` = one session per student per course; re-entering a
course resumes it. Restart is out of scope.

### DEC-12.5 — The next-arrow at a phase boundary asks; it never forces

The binding design is student-paced (prev/next arrows); §5.3 has the coach pace.
DEC-12.1 reconciles them: **inside** a phase, the arrows page content freely, with
no lock and no model call. When next would **cross a phase boundary**, the arrow
becomes a request to the coach (`request_advance`). If the floor is unmet, or the
coach judges the goal unmet, the student stays on the page and the coach's answer
appears in the ask panel saying what is still owed — auto-expanding the panel if
collapsed, because an unanswered request is worse than an expanded panel.

No modal, no forced interruption, no streak, no lock UI (铁律 2). Backward paging
is never gated: gates govern unlock, never revisit.

### DEC-12.6 — Rename Chat's three scoped-graph types to surface-neutral names

Slice 11 named three types for the only surface that then existed:
`ThreadMaterial` · `ThreadCard` · `ChatCardOffer`. They are not chat-shaped — they
are the minimal projection of *any* scoped graph, and Course needs exactly them.
Rename mechanically, in one commit, before the course runtime lands:
`ThreadMaterial` → `ScopedMaterial`, `ThreadCard` → `ScopedCard`,
`ChatCardOffer` → `CardOffer`. Contained to `apps/api/internal/agent` +
`internal/api`; the wire is unaffected (the web DTO field names come from JSON
tags, which do not change). Reusing a `Chat*` type from the Course runtime would
be the alternative, and it would misname the seam permanently.

## 3. Contracts + skill config

`packages/contracts/skills/info-literacy-course.json` (single source of truth; Go
reads the `go:embed`ed mirror via `make sync-skills`, exactly as
`writing-project.json` does).

The C5 `Skill`/`Contract` types gain course-only optional fields. All are
`omitempty`/optional — `writing-project.json` is unchanged and must keep loading:

```jsonc
{
  "id": "info-literacy-course",
  "kind": "course",
  "course_id": "00000000-0000-0000-0000-0000000000c1",
  "cards": ["craap"],
  "contracts": {
    "demonstrate": {
      "requires": [], "produces": [], "title": "演示",
      "goal": "让学生意识到「随手一信」的风险，建立停一下的习惯",
      "steps": [0, 1],
      "repertoire": [],
      "ask_chips": ["为什么不能只看一篇文章？", "横向溯源到底怎么做？"],
      "floor": [{ "kind": "steps_viewed", "steps": [0, 1] }],
      "soft_condition": "学生能说出「随手一信」的风险，或提出了一个真实的疑问",
      "gate": { "machine": [], "student_written": [], "human": [] }
    },
    "guided": {
      "requires": ["demonstrate"], "title": "引导",
      "goal": "在这条真实说法上，带学生做一次完整的信源辨识",
      "steps": [],
      "page": {
        "title": "现在，对这条说法做一次溯源",
        "subtitle": "印记会陪你走一遍——但判断是你的。",
        "body": ["「卫星图显示，过去二十年地球变绿，主要是中国的功劳。」"]
      },
      "cards": ["craap"],
      "anchor_material": {
        "title": "待核实的说法",
        "text": "卫星图显示，过去二十年地球变绿，主要是中国的功劳。"
      },
      "ask_chips": ["这张卡要我做什么？", "我该先查哪一步？"],
      "floor": [{ "kind": "card_dispositioned", "card_id": "craap" }],
      "soft_condition": "学生对这条说法做完了一次真实的溯源，不是走过场",
      "gate": { "machine": [], "student_written": [], "human": [] }
    },
    "independent": {
      "requires": ["guided"], "title": "独立", "steps": [2],
      "goal": "学生独立给出判断，并说得出理由",
      "ask_chips": ["我可以说「不确定」吗？"],
      "floor": [{ "kind": "student_turns_at_least", "n": 1 }],
      "soft_condition": "学生给出了判断并说得出理由，理由指向证据而不是感觉",
      "gate": { "machine": [], "student_written": [], "human": [] }
    },
    "reflect": {
      "requires": ["independent"], "title": "回看", "steps": [],
      "goal": "让方法从这次经验里抽象出来",
      "page": { "title": "回看这一趟", "subtitle": "你刚才用的，是一套可以带走的方法。", "body": [] },
      "ask_chips": ["下次遇到类似的信息，我该先做什么？"],
      "floor": [{ "kind": "student_turns_at_least", "n": 1 }],
      "soft_condition": "学生说得出这次用的方法，而不只是这次的结论",
      "gate": { "machine": [], "student_written": [], "human": [] }
    }
  }
}
```

New optional `Contract` fields (Zod + Go): `goal`, `steps []int`, `page`,
`cards []string`, `anchor_material`, `ask_chips []string`, `floor []FloorItem`,
`soft_condition`. New optional `Skill` field: `course_id`.

`FloorItem` = `{ kind, steps []int, card_id string, n int }`; fields are read only
by the kind that uses them, mirroring `MachineItem`.

**Validation at `Load` (`skills`):** every `floor[].kind` ∈ the closed
`CourseFloorKinds` set; every `cards[]` entry ∈ `Skill.Cards`; and for
`kind == "course"` the `requires` chain must be **linear** — each contract has ≤1
requirer and ≤1 requiree, exactly one root — because "binding order" is not
expressible as a general DAG. A non-linear course skill is a config error, caught
at load, not at runtime. `kind == "project"` keeps today's general-DAG validation.

## 4. Runtime — `apps/api/internal/agent/course_step.go`

Mirrors Slice 11's `chat_step.go` in shape; nothing from `RunAgentStep` (which is
project-graph-coupled) is reused.

```go
type CourseStore interface {
    GetSession(ctx context.Context, sessionID uuid.UUID) (CourseSession, error)
    SetSessionPhase(ctx context.Context, sessionID uuid.UUID, phase string) error
    LoadPhaseHistory(ctx context.Context, sessionID uuid.UUID, phase string, limit int) ([]ChatTurn, error)
    CountStudentTurns(ctx context.Context, sessionID uuid.UUID, phase string) (int, error)
    CreateSessionMessage(ctx context.Context, sessionID uuid.UUID, phase, role, content string) (uuid.UUID, error)
    ListSessionCards(ctx context.Context, sessionID uuid.UUID) ([]ScopedCard, error)
    CreateSessionCardInstance(ctx context.Context, sessionID uuid.UUID, cardID string, materialID uuid.UUID) (uuid.UUID, error)
    CreateSessionMaterial(ctx context.Context, sessionID uuid.UUID, title, text string) (uuid.UUID, error)
    ListSessionMaterials(ctx context.Context, sessionID uuid.UUID) ([]ScopedMaterial, error)
    ViewedSteps(ctx context.Context, userID, courseID uuid.UUID) ([]int32, error)  // course_progress.completed_ordinals
    InsertUserEvent(ctx context.Context, userID uuid.UUID, surface, typ string, payload []byte) error
    RecordCourseLLMCall(ctx context.Context, userID uuid.UUID, resolved gateway.Resolved, prompt, completion int32) error
}

type CourseDeps struct {
    Store     CourseStore
    Provider  gateway.Provider
    Resolved  gateway.Resolved
    Skill     skills.Skill
    UserID    uuid.UUID
    SessionID uuid.UUID
}

type CourseIntent string // "ask" | "request_advance"

type CourseStepResult struct {
    Reply    string     // "" = silence (enforcement reject)
    Advanced string     // "" = stayed; else the new phase id
    Offer    *CardOffer // session card surfaced at I4
}

func RunCourseStep(ctx context.Context, deps CourseDeps, intent CourseIntent, studentMessage string) (CourseStepResult, error)
```

**Pure, separately-tested helpers:**

- `NextPhase(sk skills.Skill, cur string) (string, bool)` — the linear successor.
- `CheckFloor(items []skills.FloorItem, st FloorState) (unmet []skills.FloorItem)` —
  `FloorState{ViewedSteps []int; DispositionedCards []string; StudentTurns int}`
  (`DispositionedCards` = session cards in status `completed` **or** `skipped`, per
  DEC-12.2). Pure; the closed-set evaluator. An unknown kind is treated as
  **unmet** (fail closed), even though `Load` rejects it — defense in depth.
- `CourseCardCandidate(sk skills.Skill, phase string, cards []ScopedCard) (cardID string, ok bool)` —
  the phase's declared card, unless a session card_instance for it exists in ANY
  status (surface once, mirroring Chat's offer discipline).

**`RunCourseStep` flow**

1. Load the session; resolve the current phase's `Contract` from the skill.
2. `intent == "ask"`: persist the student `course_message`; build context; call
   `ProposeCourseReply`; expect `reply` (or `advance`, which is ignored on an
   `ask` — the coach does not advance on a question) ; persist the assistant
   message. If the phase declares a card and `CourseCardCandidate` says surface,
   mint the anchor material (once, deduped by title) + a `proposed` session
   card_instance at I4, and return it as an `Offer`.
3. `intent == "request_advance"`: compute `FloorState` entirely from the store
   (`ViewedSteps` / session cards / `CountStudentTurns`); `CheckFloor`. **Unmet → no model call**, an
   authored reply naming what is owed, persisted as an assistant message, no
   advance. Met → `ProposeCourseReply` with the soft condition and the successor
   phase id; on `advance` whose `to` == the successor, `SetSessionPhase` + emit
   `phase_advanced`; any other `to`, or a `reply`, leaves the phase unchanged and
   shows the coach's reply.
4. Enforcement reject (banned phrasing / invalid output) → **silence**: no
   message, no advance, not an error return — and the `llm_call` is still recorded
   (tokens were spent). Identical to Chat's metering-on-reject.

**Coach — `apps/api/internal/agent/course_coach.go`**

`courseCoachPosturePrompt`: the Course variant per §5.3 — may explain and
demonstrate (teaching is the point, the most permissive of the three postures),
still never produces the student's assessed deliverable, one question at a time,
and the consolidation rule holds (a card's abstract framework is revealed *after*
use, never before). Plus the hard limits as stated posture: cannot reorder or skip
phases, cannot substitute cards.

`BuildCourseContext(script CourseScript, phase Contract, history []ChatTurn, cardSummary, intentHint string) string` — pure,
its own test. The **course context recipe** (§5.3) and nothing more: the session
script + current phase goal + this phase's dialogue history + the active card
instance. Explicitly **not** the project graph.

`ProposeCourseReply(ctx, prov, r, ctxStr) (enforcement.AgentOutput, gateway.ChatUsage, error)` —
mirrors `ProposeChatReply`: system+user turns, full enforcement stack, usage
populated even on reject. Runs on the **chaperone (mid-tier) resolver**, like the
chat coach — not the flagship.

## 5. Endpoints — `apps/api/internal/api/course_session.go`

Six routes, all `protected`, all entitlement-gated before any token is spent, all
ownership-checked 404-not-403 via `loadOwnedSession`:

| route | does |
|---|---|
| `POST /api/v1/courses/{id}/session` | get-or-create the session (idempotent on `UNIQUE(user_id,course_id)`); returns `CourseSessionDTO` |
| `GET /api/v1/courses/{id}/session` | the session + phase + messages (404 if none) |
| `POST /api/v1/courses/{id}/session/ask` | SSE turn, `intent="ask"` |
| `POST /api/v1/courses/{id}/session/advance` | SSE turn, `intent="request_advance"`; no body — the floor's inputs are all server-side |
| `POST /api/v1/courses/{id}/session/cards/{cid}/submit` | thin submit: persist `field_values`/`event_trace`, flip to `completed`. **No** graph_effects, **no** refeed, **no** competence — Slice 11's thin-card precedent, for the same reason (competence is dormant platform-wide: `card_competence` still has zero update sites) |
| `POST /api/v1/courses/{id}/session/cards/{cid}/skip` | flip to `skipped` |

SSE frames reuse Chat's vocabulary so the web client's accumulate-deltas pattern
carries over: `event: text {delta}` · `event: card {card_instance_id, card_id,
material_id}` · **`event: phase {to}`** (new) · `event: done` · `event: error`.

`CourseSessionDTO{id, courseId, phase, phaseTitle, status, messages[]}`;
`CourseMessageDTO{id, phase, role, content, createdAt}`. Web client:
`apps/web/src/api/courseSession.ts` + Zod in `packages/contracts/src/course.ts`
(`CourseSession`, `CourseMessage`, `CoursePhase`).

The five existing course routes are untouched.

## 6. UI — `apps/web/src/shell/courses/`

Binding: `dc.html` 215–410. Copy is **verbatim**: 问印记 · 随时打断我，问任何问题 ·
正在看：{context} · 你可能想问 · 输入你的问题…… · 按住说话，问老师.

- `AskPanel.tsx` (new) — 330px expanded / 46px collapsed rail with the vertical
  问印记 label; Bean in `playerBranchColor`; the `正在看：` pill (context = the
  current phase title, per `askContext`); authored `ask_chips` from the skill,
  clickable → sends as a message; composer; the 按住说话 button **renders and is
  inert** (voice is deferred, exactly as Chat's multimodal icons are).
- `CoursePlayer.tsx` (modified) — mounts the panel; renders `pgStage` = the phase
  title on the header and above the page title; pages phase-authored `page` blocks
  for step-less phases and `course_step` renders otherwise; the next-arrow calls
  `advance` at a phase boundary and pages locally otherwise; `event: phase` moves
  the phase and auto-expands the panel on a refusal-to-advance.
- The card offer mounts `StudioCardSheet` as an overlay with
  `{spec: CARD_REGISTRY[cardId], onSubmit → submitCourseCard, onSkip → skipCourseCard}`
  (Slice 11 precedent — the sheet proved decoupled). Confirm-to-open holds: the
  offer is a card in the ask panel with an explicit 接受 before the sheet mounts.

Icons inline SVG, never lucide-react.

## 7. Evidence + cost

Course events enter the **same** append-only stream at **full** weight (§5.5),
`surface="course"`: `course_session_started` · `course_message` (with
`unprompted` set from intent: an `ask` is student-initiated) · `card_surfaced` ·
`card_completed` · `phase_advanced`. Full weight is the whole point — course
practice is real evidence, unlike Chat's supplementary tier.

Every real model call records one `llm_call` row: `surface="course"`,
`purpose="coach"`, `project_id` NULL, `user_id` set, **including on enforcement
reject**. The existing `purpose="course_render"` path is untouched, so course cost
remains single-counted across both purposes and `llm_usage` needs no change.

## 8. Red lines

- **RL-1** holds structurally: Course has **no write path to a draft at all** —
  no `edit_buffer`, no `draft_snapshot`, no deliverable. Banned-phrasing still
  runs on every output.
- **铁律 2 (不操纵)**: the card is an offer with confirm-to-open; a refused advance
  is a sentence in a panel, never a modal or a lock; no streaks/leaderboards.
- **铁律 4 / 过程即数据**: a skipped card and a refused advance are both recorded.
- **DEC-3 discipline**: machine judgment (the floor) never declares a phase
  pedagogically met — it only refuses. The positive call is the coach's.

## 9. Non-goals (genuinely absent, not stubbed)

Assessor aggregation of course evidence into 成长报告 (Slice 10's projection is
project-scoped; course events land in the stream but nothing reads them yet) ·
terminal-assessment challenge + its machine-never-`solid` adjudication · golden /
banned example packs per phase · voice (按住说话 renders, inert) · per-card
competence writes (dormant platform-wide) · course→project seeding · multi-course
authoring (one skill, one seeded course) · session restart · the classifier's
course moment predicates beyond the phase's declared card.

## 10. Tests

- `skills`: course skill loads; linear-chain validation rejects a branch and a
  cycle; unknown floor kind rejected; unknown card ref rejected; `writing-project`
  still loads unchanged (regression).
- `contracts`: `advance` accepted with `to`, rejected without; the other 6 union
  members unchanged.
- `enforcement`: `ValidateOutput` accepts `advance`, rejects empty `to`; banned
  phrasing unchanged.
- `agent` (pure): `NextPhase` linear walk + terminal; `CheckFloor` per kind, unmet
  set, unknown-kind-fails-closed, **a skipped card satisfies `card_dispositioned`**
  (the no-dead-end guarantee); `CourseCardCandidate` surface-once.
- `agent`: the DEC-12.6 rename compiles with Chat's tests unchanged in behavior.
- `agent` (fake store): ask → reply persisted; ask → card offer minted once, not
  twice; advance with unmet floor → **no LLM call**, no phase change; advance with
  met floor + `advance` output → phase moves + event; `advance` naming a
  non-successor → phase unchanged; banned reply → silence but `llm_call` recorded.
- `api` (testcontainers): session get-or-create idempotent; ownership 404 on all
  six routes; entitlement gate before SSE; SSE frame shapes; thin submit writes no
  graph_effects.
- `web`: AskPanel renders binding copy verbatim, chips send, 按住说话 inert;
  CoursePlayer pages inside a phase without a network call, calls advance at the
  boundary, auto-expands on refusal; card offer requires 接受 before the sheet.
- Gate: `CGO_ENABLED=0 go test -p 1 ./...` (FULL packages), contracts, web, tsc,
  `make sqlc` + `make sync-skills` no drift.

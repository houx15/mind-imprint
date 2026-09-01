# PBL project room — design

> 2026-09-01 · The first of three sub-projects taking `/eco` into lite for real.
> Order agreed with the product owner: **项目 → 兴趣树 → 探索宇宙**.
>
> **Revision 3**, same day. R2 followed the owner's eight-point review (§22);
> R3 folds in the two Codex design documents and is recorded in §23.
>
> Source material: `docs/2026-08-31-eco-to-lite-build-brief.md`,
> `docs/2026-08-30-ecosystem-prototype-spec.md` (revisions v1–v9),
> `docs/2026-08-31-pbl-tool-model.md`, the owner's own words in
> `docs/2026-09-01-eco-lite-original-intent-verbatim.md`, and — binding on
> §9/§10/§12 — `docs/02-project-conception-and-dynamic-planning-design.md` and
> `docs/03-agent-loop-and-thinking-session-protocols.md`.
>
> Scope check: this is a new lite surface, not the writing-project flow, so the
> `docs/2026-08-09-all-statuses.md` consistency gate in AGENTS.md does not apply.
> Checked, not assumed.

---

## 1 · What this builds

A room where a student brings a real idea, 印记 helps her turn it into
something, and the two of them work through it one step at a time — with the
judgement staying hers throughout.

The prototype at `apps/lite-web/src/eco/projects/` settled the shape across
nine rounds. This document says how it becomes real: server-side orchestration,
Postgres state, live model calls, sub-agents for production work, and the gates
enforced where they cannot be clicked past.

## 2 · The surface

**项目 is a single page, not a room with a sidebar.** Reading and writing each
concern one thing at a time, so their history belongs on a landing page. A
student accumulates many questions at once — some still being talked about,
some underway, some finished, some being kept alive — so the project surface
has to show that spread.

The page has two parts:

1. **A big input box**, the same shape as the reading and writing rooms'. She
   puts an idea, an observation, a complaint, a question into it. Not a form,
   not a track picker.
2. **A kanban of everything she has going**, columned by status (§3). A
   timeline view is the alternative rendering of the same data; the kanban is
   the default because status is what she acts on.

On submit, 印记 reads what she wrote and **decides the project type** —
research, design, website, making, investigation — rather than asking her to
classify her own idea before she has one. Then a modal asks for the two things
only she can give: **a name and a cover**.

## 3 · Project lifecycle

| Status | What it means |
|---|---|
| `talking` | The idea is being turned into something workable. No plan yet. |
| `running` | A plan exists and steps are being worked. |
| `review` | The work is done; 复盘 is happening. |
| `keeping` | Finished and being maintained. |
| `archived` | Closed. Still readable, still hers. |

Two of these deserve stating plainly, because they are where this product
differs from a task app:

**复盘 is a status, not a button.** A project that ends without one is a project
she cannot learn from twice. It gets its own conversation, its own tools (§6),
and it is what produces the process record.

**Maintenance is help, not hosting.** 印记 helps her keep something alive —
notices what has gone stale, asks what changed, drafts the update. We do not
become the backend for her outputs. The single exception is her own website,
which we host because we said we would (§9).

## 4 · The first project is her website

If a student has no site yet, her first project is building one. Not offered —
it is what the first project is.

The reason is that the website is the one artifact this product hosts and the
one that makes everything else visible. It gives her somewhere for her readings,
her writings and her later projects to land, and it is a genuine build with a
real audience at the end.

What the project actually contains:

- **Research into what a blog is.** Real structures and real styles, shown
  rather than described. The six reference sites in spec v9 (lilianweng,
  5ime.cn, d-d.design, keyork, terrifyzhao, lixiaolai) are the starting stock.
- **A general structure of blogs, and the styles available**, taught — the
  identity block, the section names people actually use, a post list with a
  real meta row, 站点信息, a plain footer. This is teaching content, authored,
  not generated fresh each time.
- **Her thinking about the contents.** What goes on it is hers. This is the
  part that is not automatable and the reason the project is worth doing.
- **Preview → modify → online**, looped until she is happy.
- **After it is live, she can modify or rebuild it at any time.** Publishing is
  not a terminal state; the site project moves to `keeping`.

## 5 · What we do, and what we hand off

Product-owner correction, 2026-09-01 — the first draft of this spec overstated
the boundary into a doctrine:

> it is not that we always only gives spec. we only do this when we don't have
> the ability, and we don't need to be 万能.

So the rule is capability, not principle:

| | |
|---|---|
| **We do it** | Generate images · write documents and drafts · build static websites · structure and review her thinking |
| **We hand off a spec** | Anything needing a running system, a live service, or a platform that already exists and does it better |

Handing off is a real move, not an apology. When she needs an app built, 印记
takes the thinking all the way to a spec she can drive Claude Code or Codex
with, and teaches her how to drive it. When she needs a questionnaire fielded,
印记 designs it and she runs it on 问卷星. What comes back is hers, and the
judging of what came back is the part we care about most.

The plan model already carries this. `PlanStep` has `iBring` / `youBring` /
`then` — what 印记 does, what she does, and what she reports back. A handoff is
that split with an external tool named in it.

## 6 · The method layer — pending

**Owned by the product owner; detailed design arriving within a day.** This
section records the shape so the rest can be built against it.

A project does not go "she states an aim → 印记 proposes roads". Something has
to happen first, and it is made of general, reusable methods:

1. Turn an idea or observation into a workable plan
2. Review
3. Make a reasoned decision
4. Design the structure
5. Collaborate well
6. 复盘 a project
7. Keep something alive

These are **methods, not steps** — 印记 reaches for the one the moment needs,
the way it already reaches for a card in the reading room. The set is open.

Two kinds of tool sit under this layer, on the owner's timeline:

- **Thinking tools** — support design, decision and judgement during
  collaboration with the AI. Decided within two days.
- **Real-world tools** — support activities that happen away from the screen.
  Finalised within the week.

Nothing in this build hard-codes a sequence of methods. The step loop asks the
method layer what fits; until the layer lands, that call site returns the
prototype's behaviour.

### 6.0 · Built on 2026-09-01/02 — the method layer is no longer pending

The owner's detailed design landed as `docs/2026-09-01-pbl-detail.md`, and all
seven stages are now built. This section's "pending" framing is superseded by
what follows; the seven methods became **ten tools**, because stage one is four
different moments and collapsing them would restore the fixed step-chain.

| Tool | 界面 | Kind | Stage |
|---|---|---|---|
| `observe` | 出去看看 | world | ① |
| `board` | 便签板 | thinking | ① |
| `reframe` | 把问题说清楚 | thinking | ① |
| `ideas` | 想办法 | thinking | ① |
| `review` | 审一遍 | thinking | ② |
| `decide` | 做决定 | thinking | ③ |
| `structure` | 先看结构 | thinking | ④ |
| `split` | 分工 | thinking | ⑤ |
| `lookback` | 复盘 | thinking | ⑥ |
| `keep` | 上线之后 | thinking | ⑦ |

**The two kinds are data, not vocabulary.** `thinking` finishes at the screen;
`world` means she leaves and comes back days later. Two code paths branch on it:
印记 stops talking after a `world` tool, and an unresolved `world` tool is normal
where an unresolved `thinking` tool is abandonment. Without the distinction 印记
guesses, and half its guesses are nagging.

**Every surface shares one frame** (`ToolFrame`): what to do, why now (印记's own
sentence), and what is still missing before it can be finished. Those three are
the owner's "clear / motivated / willing to take part" turned into code.

**Language rule, binding:** in-product text names what the student is doing,
never the method it comes from. 「把问题说清楚」, never "reframe"; 「先看结构」,
never "information architecture". A test in `internal/pbl/tools_test.go` guards
the labels.

**⚠️ Stage ③ (做决定) was an empty heading in the owner's document.** The design
built — options → what matters here → wins and hurts → choose, plus 「what would
change my mind」 — is mine and awaits her review. The last field is the load-
bearing one: it is what makes a decision reviewable in stage ⑥, and 复盘 asks it
back by name.

**Still 占位:** photo / voice / drawing capture on 出去看看 (needs OSS, S4);
per-kind artifact rendering beyond documents and images.

### 6.1 · What arrived on 2026-09-01

Two design documents, written with Codex:

- `docs/02-project-conception-and-dynamic-planning-design.md` — how a project
  starts from a photo, a complaint, an interest, a question or a ready-made
  solution, and how the plan keeps changing afterwards.
- `docs/03-agent-loop-and-thinking-session-protocols.md` — the agent's behaviour
  protocol: context layers, the per-turn decision order, four session contracts,
  Plan Check, and the failure modes to design against.

They settle much of what this section was waiting for, and they are now binding
on §9 (the graded plan model), §10 (sessions, their contracts, the router) and
§12 (the gates). Two things to note about how they landed:

**The 「student status → default action」 routing table** in doc 02 §四 replaces
「method #1」 as I had framed it. There is no single idea-to-plan procedure;
there are nine entry states, and 印记 continues from wherever she already is. A
student who arrives with a finished solution is not walked back through design
thinking, and a student with nothing is not asked to pick a topic.

**Still not designed, and the owner says so:** the concrete interaction — the
brainstorming room, the reframe card, the research hint. Doc 03 §十二 does give
the shared primitives (one-sentence cards with source and certainty marks, a
few choices at a time, a compare tray, drag-to-merge, the plan diff, undo by
saying so in the conversation), which is enough to build the substrate against.

⚠️ **Those primitives may already be the answer to §13.** A card that holds one
sentence, carries 看到 / 猜到 / 资料显示 / 我现在猜的, and can be dragged and
merged is a different object from a panel of labelled input fields. The rejected
thing was the form, not the card.

## 7 · Orchestration

A new package, `apps/api/internal/pbl`.

Rejected: adding a `pbl-project.json` beside `writing-project.json` in
`internal/skills`. That package loads a **fixed contract DAG** — authored
milestones with machine gates (`node_present`, `no_orphan_evidence`) over a
graph of evidence nodes. A PBL plan is generated at runtime, edited by the
student, and changed mid-flight; it is instance data, not config. Forcing one
into the other reintroduces exactly the fixed step-chain the prototype spent
nine rounds removing.

Also rejected: orchestrating on the client. It breaks the rule that the
decision layer is server-side, loses per-call cost accounting, and makes the
process unevaluable.

Reused rather than re-implemented: `internal/gateway` (provider mux, SSE,
`llm_call` cost rows), `internal/agent` (tool loop, refeed, the sub-agent
pattern), `internal/oss`, and the session/org checks in `internal/api`.

## 8 · What 印记 can do

印记 has no shell, no filesystem and no arbitrary execution. Its powers are a
closed list, and each one is a deliberate product decision.

| Tool | Effect |
|---|---|
| `open_session` | propose a session off any anchor — untyped, or one of the four kinds (§10.4) |
| `close_session` | write back its result; refused without one (§10.4) |
| `revise_plan` | progress and **local** changes only — applied, announced in a line, undoable (§9.1) |
| `propose_plan_change` | a **structural** change: lands in `pending_plan_changes`, never on the live plan (§9.1) |
| `set_step_status` | move the living tasklist through its seven states (§9.3) |
| `generate_image` | DashScope 通义万相 → OSS (§11) |
| `write_html` | delegate to a sub-agent that returns a static page |
| `write_document` | delegate to a sub-agent that returns a draft, report or **spec** |
| `hand_over` | put a produced thing on the table for judgement, with `guessed[]` and `admits[]` |
| `handoff_tool` | name an external tool, why it fits, and what she brings back |
| `summon_tool` | **endpoint retained, interaction deferred** (§13) |

`write_html` and `write_document` are sub-agent calls, following the pattern
already in `evalreport/subagents.go` and `agent/reportgen.go`: a
context-isolated call with its own `purpose`, metered as its own `llm_call`
row. The main thread stays a conversation; production work happens beside it.

Everything not on this list happens as prose in the chat. A question that is
just a question gets asked one at a time.

## 9 · The plan is a living tasklist

Cowork's shape, and a correction to the prototype: the plan is not approved once
and then frozen. It is an ongoing tasklist with per-step status that 印记 revises
as the project teaches them both something.

What survives from the prototype unchanged: **nothing runs before she has
approved the first plan**, and every step names what she has to **decide**.

### 9.1 · Changes are graded, not all-or-nothing

Revised 2026-09-01 against `docs/03-agent-loop-and-thinking-session-protocols.md`
§十.1. An earlier draft of this spec said 印记 revises freely as long as it gives
a reason. That is right for small changes and wrong for large ones, and the
grading is the part that matters:

| Grade | Example | What happens |
|---|---|---|
| **Progress** | a step finishes, a file is produced | applied silently, no interruption |
| **Local** | one step splits in two, order shifts | applied, announced in one line, **undoable** |
| **Structural** | the question, the people, the solution, the success criteria or the scope changes | **must go through Plan Check** and get her decision |
| **Fork** | two directions are both worth keeping | 印记 proposes a branch; she decides |

The failure this prevents has a name — 隐形重规划: 印记 quietly swapping the
project's goal because new information arrived. So an unconfirmed structural
change lands in `pending_plan_changes` and **never overwrites the live plan**.

### 9.2 · Plans are versioned

`v0.1`, `v0.2`. A version records a change in *understanding or decision*, never
a task ticking over. Old versions, the evidence behind each change, and her
reasons stay readable — a rejected direction that leaves no trace is a lesson
thrown away.

### 9.3 · Step status

Seven, from the same doc: `已确定 / 暂定 / 等待调查结果 / 等待学生决定 / 已完成 /
已修改 / 已取消`. The two waiting states carry most of the value: they let the
board say *why* nothing is moving, which is the thing a stuck student cannot
usually articulate.

### 9.4 · Plan Check

The surface a structural change goes through. It shows four things and nothing
else: **what we just learned**, **what 印记 proposes to change** (as a diff —
新增 / 修改 / 删除 / 暂缓, never a re-display of the whole plan), **why**, each
item tied to concrete evidence, and **what she has to decide**.

Her options are always: accept · accept with edits · keep the current plan · try
both · decide later and gather more evidence first. 「Keep the current plan」 is
a first-class outcome, recorded with her reason — not a failure to engage.

## 10 · Sessions — hook questions, digging, and the named modes

> **Renamed 2026-09-01.** This was 「branches」. `docs/03-...` already uses
> 分支 / branch for a **project fork** (两个方向都值得保留), which is a different
> object, and two meanings on one word would have collided in the schema.
> A think-deeply side thread is now a **session**, and 「branch」 is left to mean
> the fork.
>
> The rename is also a simplification, not a workaround: a free-form dig *is* a
> lightweight Thinking Session. So there is one object — `pbl_session` — with a
> `kind`. A dig is an untyped session; 观察日记 / Reframe / 头脑风暴 / Plan Check
> are typed ones with a contract. One router, one write-back path, one place a
> student learns the interaction.

The prototype opened a side thread only off an approach. The brief is wider:

> whenever need think, design, review, make decisions, provide hook questions,
> ask students to think deeply, students can generate a new chatting branch to
> think deeply.

So the anchor is polymorphic — `approach | hook | step | artifact | free` — and
any 印记 message may carry a hook question that opens a session when tapped. She
can also open one unprompted, from anywhere, and 印记 may propose one when the
router fires (§10.5).

**A session does not close without a write-back.** For an untyped dig that is a
`takeaway`; for a typed one it is that kind's contract (§10.5). Without it the
digging was just reading, and nothing comes back to the thread that spawned it.
This is the rule that prevents 方法论表演 — completing the ritual while the
project's question, plan and evidence stay exactly as they were.

### 10.1 · How the conversation is stored

Checked against the schema on 2026-09-01 rather than assumed.

**`atom_message` carries it, and `pbl_thread_item` is not needed.** The
precedent already exists: migration 0102 added `block_id` for the writing
room's per-block sub-agent, where `NULL` means the room's own thread and a set
value means a side conversation. Sessions are the same shape:

- `atom_message.session_id uuid REFERENCES pbl_session(id)` — `NULL` is the
  project's main thread, set is one session.
- `CHECK (block_id IS NULL OR session_id IS NULL)` — two nullable scope columns
  on one table need to say out loud that they are mutually exclusive.
- `payload` (0106) carries the items that are not prose: a step divider, an
  artifact handover, a handoff card, a plan diff. `role='system'` covers them.
- `pbl_session` holds what belongs to the session rather than to any message:
  its `kind`, its anchor, the question that opened it, its `parent_id`, and the
  `takeaway` that closes it.

**Seq stays in one space per atom**, as 0102 established. Both readings stay
correct: the main thread is `WHERE session_id IS NULL ORDER BY seq`, one session
is `WHERE session_id = $1 ORDER BY seq`. Interleaving does not corrupt either,
because gaps in a filtered sequence are still ordered.

**Nesting: one nullable self-reference.** `pbl_session.parent_id` →
`pbl_session.id`. Product-owner decision, 2026-09-01: sessions nest, and
recording the parent is enough. Three constraints make that safe rather than
merely cheap:

- **A depth cap of 3.** Context assembly walks the parent chain every turn; an
  uncapped chain is a cost that stays invisible until someone nests eight deep.
- **A child's takeaway returns to its PARENT**, not to the main thread. Skipping
  a level delivers a conclusion without the context that produced it.
- **Reject a parent that does not exist or that would exceed the cap.** For a
  tree built one node at a time, that is the whole cycle-prevention story.

### 10.2 · 🚨 The seq race becomes reachable here

`NextAtomMessageSeq` is `SELECT COALESCE(MAX(seq),0)+1 FROM atom_message WHERE
atom_id = $1`, run inside the turn's transaction. There is no `FOR UPDATE`
anywhere in `queries/`. Under READ COMMITTED two concurrent transactions both
read the same max and both insert it; `atom_message_seq_idx` (UNIQUE) rejects
one, and that turn dies **after** the model call was already paid for, losing
the student's message.

Today this is close to unreachable: one room holds one conversation, and the
composer is disabled while a turn is in flight. **Sessions make concurrent
conversations on one atom the designed behaviour** — digging in a side thread
while the main one is mid-stream is the entire point — so the race stops being
theoretical.

The fix is local and does not change the schema: take a row lock on the atom
(`SELECT id FROM atom WHERE id = $1 FOR UPDATE`) at the top of the append
transaction, so appends to one project serialize. Appends are not
high-frequency; this costs nothing that matters. Belt and braces, retry once on
a unique violation.

This must land with the session work in S2, not after it.

### 10.3 · What the model sees

A session exists so thinking can go deep without dragging the whole project
through it, which makes context assembly part of the design:

- **Inside a session:** that session's turns, its anchor, its parent's takeaway
  if it has a parent, and the project's aim. Not the main thread's full
  history, and not sibling sessions.
- **In the main thread:** the main thread's turns, plus the **write-backs** of
  closed sessions. Never every turn of every session.

When a session closes, its write-back is appended to its **parent** thread —
the main thread for a top-level session, the parent session for a nested one —
as a message with a payload marking where it came from. That is what makes a
session matter: the next turn sees what she concluded without seeing her
working.

### 10.4 · Session kinds and their contracts

From `docs/03-agent-loop-and-thinking-session-protocols.md` §十三. Each kind
declares what triggers it, what she does, what 印记 does, and — the part that
makes it real — **what it writes back**.

| kind | Trigger | Her core act | Writes back |
|---|---|---|---|
| `free` | she taps a hook, or opens one herself | dig | a `takeaway` |
| `observation` | no direction, or time to return to reality | capture, correct, decide whether to open it up | observations, guesses, a 线头 |
| `reframe` | a contradiction, a question that is too big, an answer already baked into the question | judge which frame is worth taking | frame, people, needs, HMW, open unknowns |
| `brainstorm` | several directions are needed | propose, transform, compare, choose | idea cards, criteria, a Next Bet |
| `plan_check` | the plan's structure is about to change | accept / edit / keep / fork | a new plan version, or a recorded decision to keep |

Every kind's minimum exit includes **「she kept the current understanding, and
here is her reason」**. A session that changes nothing is a valid session; a
session that records nothing is not.

### 10.5 · The router

`docs/03` calls this the per-turn decision order, and it is ours to build. Each
turn, in order:

1. Is there a safety, privacy or permission problem? Handle the boundary first.
2. Can we advance something she has already decided? Then do real work.
3. Is a decision missing that would change what happens next? Ask only that.
4. Is there a signal that needs structured thinking? Propose the fitting session.
5. Did the plan's structure change? Prepare a Plan Check.
6. Otherwise carry on with the current step.

The order exists to stop 印记 delaying action because it could always ask one
more question. Two rules ride along: never two high-load sessions back to back,
and a proposal she declined does not return without a new reason.

**A trigger is not an interruption.** 印记 names the specific reason and offers
the choice — *「我们原来以为…，但刚才四个人都没有看到地图。要用两分钟重新看一下
问题吗？」* If she says no, the project continues and the unresolved
contradiction is recorded in the plan rather than dropped.

## 11 · Images

Aliyun DashScope (通义万相). Same account as ECS, OSS and CDN, so no new vendor
and no new region, and it holds the China-first rule. Server-side key only.

🚨 **The generated image goes to OSS. The database stores the object key and
nothing else.** No bytes, no base64, no data URI, in any column — not in
`pbl_artifact.payload`, not anywhere. The endpoint's job is: call DashScope,
put the result through the existing OSS upload path, record the key, return the
CDN URL. Image bytes in Postgres would bloat every row read that touches an
artifact, break the report and export paths that already assume small rows, and
put binaries in the backup stream.

The same rule holds for anything else binary this surface ever produces.

## 12 · Gates, enforced on the server

The prototype enforced these by disabling buttons. A disabled button is
decoration. These live in the handlers, and the endpoint refuses:

1. An artifact does not settle without a `why`.
2. A decision needs both `why` and `gave_up`.
3. A session does not close without its write-back (§10.4).
4. A plan step with an empty `decide` is rejected at validation and regenerated.
   A step where she decides nothing is a step she should not sit through.
5. Nothing runs before she has approved the first plan.
6. A plan revision is rejected without a stated reason.
7. 🚨 **A structural change cannot reach the live plan.** `propose_plan_change`
   writes to `pending_plan_changes` only; the live plan moves when a Plan Check
   records her decision. This is 隐形重规划 made impossible rather than merely
   discouraged — it is the one gate that keeps the plan hers.
8. A session nests at most 3 deep, and its parent must exist (§10.1).

## 13 · Tools: endpoint retained, interaction deferred

**The card interaction as built is rejected.** Product-owner ruling,
2026-09-01:

> I think previous card system is a failure, because no one would like those
> form-like things. we do need a tool box, but the detailed interaction may
> need to be redesigned.

Clarified in the same review: **defer only the interaction, keep the endpoint.**

So this build ships:

- `summon_tool` as a working server-side tool call, and `pbl_tool_instance` as a
  real table recording that a tool was summoned, why, and what came of it;
- **no authored tool specs, no form renderer, no `CardSurface` port.**

A summoned tool is recorded and fed back to the conversation. What it *looks
like* to the student is the open design question, and it is the owner's.

## 14 · Artifacts

`kind ∈ { options, draft, spec, image, site, html }`.

`site` and `pbl_site` are two different things: an artifact of kind `site` is
one **round of handover** — this is what I built, here is what I guessed, here
is what is still wrong — while `pbl_site` is the one live page her accepted
rounds produced. Rounds accumulate; the page is singular.

Every artifact arrives with what 印记 **guessed** and what it **admits** is
still wrong with its own work. A handover that hides its assumptions can only
be accepted, never reviewed — and reviewing is the point.

## 15 · Her website

Design settled in spec revisions v8 and v9; this build changes none of it.

- Three genuinely different layouts — `Essay`, `Ledger`, `Magazine` — not one
  layout in three colourways. Her justified choice has to be visible.
- `site/` may not import the app's UI kit (`Btn`, `Panel`, any `mk-*` token).
  The moment it does, the page starts looking like the product that made it.
- The banner is the site's own drawn artwork, never a crop of one of her
  projects. Those do different jobs.
- Content comes from her real readings, writings and projects.

**Visibility: an unlisted, revocable link, `noindex`.** Product-owner decision,
2026-09-01, reusing the pattern lite already ships for report sharing
(`/s/:token`). She gets a real address to send to a parent or a friend, and a
minor's name and school do not become searchable under our domain.

`projects/PageStudio.tsx` — the dead six-step wizard with no entry point — is
deleted rather than ported.

## 16 · Data model

**A project is an atom.** Lite already has a substrate: `atom` carries
identity, ownership, `last_activity_at`, `active_seconds` and the heartbeat,
and both `reading` and `writing` are detail tables keyed by `atom_id`. A
project is the third one, `atom.kind = 'project'`.

This is worth more than tidiness. It inherits, already built and tested: the
`recordLiteLLMCall` metering path (`llm_call.atom_id`, migration 0094),
`loadOwnedAtom`'s ownership check, activity and heartbeat, `atom_message` for
the main conversation, and `atom_report` when evaluation arrives. A root table
of its own would be a second substrate standing beside an identical one, and
the second one would be the untested one. `atom` is lite-only (migration 0092),
so widening its `kind` does not reach pro.

Consequence: **`pbl_thread_item` may not be needed.** S2 should first check
whether `atom_message` plus a payload column carries thread items — the
precedent is migration 0106's `coach_card_payload`.

The remaining tables are prefixed `pbl_`. The prefix is deliberate: the
sharpest way a lite change breaks pro is by **creating** a name that already
exists, and pro owns `project`.

| Table | Holds |
|---|---|
| `pbl_project` | keyed by `atom_id` — idea text, type, name, cover, **status** (§3) |
| `pbl_plan_step` | ordinal, title, blurb, goal, you_bring, i_bring, **decide**, then, **status**, off, mine |
| `pbl_plan_revision` | what changed, why, when — the plan's own history |
| `pbl_approach` | one road 印记 proposed: shape, how, **costs**, needs |
| `pbl_decision` | chosen approach + `why` + `gave_up` |
| ~~`pbl_thread_item`~~ | **Dropped.** `atom_message` + `payload` carries it — see §10.1 |
| `pbl_session` | kind, anchor kind + ref, the question, `parent_id`, `takeaway`, closed_at |
| ~~`pbl_branch_turn`~~ | **Dropped.** `atom_message.session_id` carries it — see §10.1 |
| `pbl_plan_version` | version number, what changed, why, whose decision |
| `pbl_pending_change` | a proposed structural change awaiting Plan Check — never applied to the live plan (§9.1) |
| `pbl_decision` | what she accepted, edited, kept or deferred, and her reason |
| `pbl_artifact` | kind, payload (text/JSON only — binaries live in OSS as a key, §11), `guessed[]`, `admits[]`, verdict, `why` |
| `pbl_tool_instance` | a summoned tool, its reason, its result (§13) |
| `pbl_site` | her website: content, layout, publish token, revoked_at |
| `pbl_review` | 复盘: what she concluded, what she would do differently |

Every model call on this surface writes an `llm_call` row with tier, tokens and
cost, per AGENTS.md — sub-agent calls included, each under its own purpose.

## 17 · Frontend

`apps/lite-web/src/projects/`, a third tab beside 阅读 and 写作.

Ported from the prototype: the workbench shape (chat left, panel right, panel
widens), `Planner`, `StepIntro`, `Roads`, `BranchTalk`, `Overview`,
`CoverPicker`, and `site/` verbatim with its no-UI-kit rule intact.

New, with no prototype to port from: the kanban landing page, the big input
box, the type-detection handoff into the name-and-cover modal, the living
tasklist with per-step status, and the 复盘 surface.

Not ported: `store.tsx`, the `sessionStorage` layer, the timer theatre in
`Make.tsx`, the scripted replies in `data/plan.ts`, the invented news, and every
card form surface.

## 18 · Dark theme

Token work lands in the first slice so every new surface is born theme-aware
rather than retrofitted. Retrofitting pro's existing screens is a separate pass.

🚨 `mk-*` tokens are bare CSS variables, so **every** Tailwind alpha syntax
emits no CSS at all. Use `color-mix` or `linear-gradient`.

## 19 · Testing

Logic tests only: plan validation, plan-revision rules, status transitions, gate
enforcement, artifact settle rules, session write-back, publish-token issue and
revoke, cost accounting, and the request normalizers. No
assertion-per-rendered-element suites — they break on every honest redesign and
catch nothing.

The UI is checked by looking at it in a real browser with a Playwright
screenshot. On 2026-08-30 lite had 344 green tests sitting on top of an exported
report PNG that was completely blank.

## 20 · Slices

| | |
|---|---|
| S1 | tables · the kanban landing · big input box · type detection · name-and-cover modal · dark-theme tokens |
| S2 | the living tasklist (versioned, graded, `pending_plan_changes`) · step loop · chat · the seq fix · sessions (nestable, write-back enforced) · the router · Plan Check |
| S3 | sub-agent production · artifacts · the settle-with-a-reason gate · `handoff_tool` · `summon_tool` endpoint |
| S4 | images — DashScope → OSS |
| S5 | her website · research and teaching content · renderer · preview → modify → online · revoke |
| S6 | 复盘 and `keeping` |

The method layer (§6) lands into S2 and S3 when the owner's design arrives.

### Status, 2026-09-01

| | |
|---|---|
| S1 | ✅ shipped — landing, kanban, type detection, name-and-cover, dark theme |
| S2 | ✅ shipped — sessions, versioned plan, Plan Check, router, the turn, the room |
| S3 | ✅ backend — artifacts with the disclosure and reason gates, plus the `summon_tool` endpoint and `pbl_tool_instance`. **Frontend 占位**: per-kind artifact rendering waits on the interaction design |
| S4 | not started — images |
| S5 | not started — her website |
| S6 | not started — 复盘 and `keeping` |

What the room deliberately does **not** have: the brainstorming room, the
reframe card and the research hint. They are named as 占位 at the foot of
`ProjectRoom.tsx`, and they arrive as new `atom_message.payload` kinds plus a
panel view — nothing built has to be undone to admit them.

## 21 · Open questions

1. **The method layer and the two tool kinds.** Owner, 1–7 days. Everything is
   built so it can arrive without rework.
2. **Research has no retrieval.** 🚨 The backend has no search provider of any
   kind, and `agent/search_guidance.go` records the current posture as
   「help pick WHERE to look, never fetch for the student」. So "印记 does
   research" today means model knowledge with no sources — which in a product
   that teaches source evaluation is the worst of the three options. Needs a
   decision: add a search provider, keep the advisory posture, or accept
   unsourced synthesis and label it loudly.
3. **Kanban or timeline as default.** Kanban assumed, both are the same data.
4. **Evaluation.** A finished project should produce a process evaluation, as
   readings and writings already do. It wants the interest tree's keyword layer
   to be worth much, so it belongs after sub-project two.

## 22 · What changed in revision 2

The product owner's eight-point review, and what each point moved:

1. **First project is always the website** → new §4; site work moved out of a
   late slice into a first-class project type with teaching content.
2. **Big input box, AI decides the type, modal for name and cover** → new §2;
   the prototype's track picker is demoted.
3. **Not "aim → approaches"; a method layer comes first** → new §6; the step
   loop now calls into a layer rather than owning a sequence.
4. **The boundary is capability, not doctrine** → §5 rewritten. We build images,
   documents and static sites ourselves.
5. **Defer the card interaction, keep the endpoint** → §13; `summon_tool` and
   `pbl_tool_instance` are back in, the form renderer stays out.
6. **No bash; sub-agents for HTML and documents; the plan is a living tasklist**
   → §8 and §9 rewritten.
7. **复盘 and continual maintenance** → §3 statuses, `pbl_review`, S6. Recorded
   with the owner's distinction: we help her maintain, we are not the backend
   for her outputs.
8. **A kanban across projects, one page, not a sidebar** → §2.

## 23 · What changed in revision 3

Folding in `docs/02-…` and `docs/03-…`, plus two product-owner answers.

1. **Sessions nest, and the parent is enough** (owner, 2026-09-01):
   `pbl_session.parent_id`, with a depth cap of 3, a write-back that returns to
   the PARENT rather than the main thread, and validation that the parent
   exists. §10.1.
2. **「branch」 was overloaded and is now split.** Doc 03 uses it for a project
   fork; this spec used it for a think-deeply side chat. The side chat became a
   **session**, and 「branch」 keeps the fork meaning. §10.
3. **A dig and a Thinking Session are the same object.** One `pbl_session` with
   a `kind` — untyped for a free dig, typed for 观察日记 / Reframe / 头脑风暴 /
   Plan Check. One router, one write-back path, one interaction to learn. §10.4.
4. **The plan model is graded, versioned, and staged.** Progress applies
   silently, local changes apply with a line and an undo, structural changes go
   through Plan Check, and a fork proposes a branch. Unconfirmed structural
   changes live in `pending_plan_changes` and never touch the live plan. Seven
   step statuses, not four. §9.
5. **The router is ours to build.** The owner's read was that the agent loop is
   already realised; what exists is the turn loop, tool-calling and refeed. The
   per-turn decision order, the session router, the write-back contracts and
   Plan Check do not exist. Confirmed with the owner and added to S2. §10.5.
6. **「Method #1」 was the wrong frame.** Doc 02 §四 has nine entry states, not
   one idea-to-plan procedure. 印记 continues from wherever the student already
   is — a student holding a finished solution is not walked back through design
   thinking. §6.1.
7. **Two new gates**: a structural change cannot reach the live plan, and a
   session nests at most 3 deep. §12.7, §12.8.

Still open: the concrete interaction for the brainstorming room, the reframe
card and the research hint — the owner's, and not blocking the substrate.

# PBL project room — design

> 2026-09-01 · The first of three sub-projects taking `/eco` into lite for real.
> Order agreed with the product owner: **项目 → 兴趣树 → 探索宇宙**.
>
> Revision 2, same day, after the product owner's eight-point review. The
> changes are recorded in §17 so the reasons survive.
>
> Source material: `docs/2026-08-31-eco-to-lite-build-brief.md`,
> `docs/2026-08-30-ecosystem-prototype-spec.md` (revisions v1–v9),
> `docs/2026-08-31-pbl-tool-model.md`, and the product owner's own words in
> `docs/2026-09-01-eco-lite-original-intent-verbatim.md`.
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
| `open_branch` | suggest a think-deeply thread off any anchor (§10) |
| `revise_plan` | add, reorder, drop or rewrite steps **mid-flight** (§9) |
| `set_step_status` | move the living tasklist forward |
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
and then frozen. It is an ongoing tasklist with per-step status that 印记 can
revise as the project teaches them both something.

What survives from the prototype unchanged: **nothing runs before she has
approved the first plan**, and every step names what she has to **decide**.

What changes: after approval, `revise_plan` may add, drop, reorder or rewrite
steps. Every revision is recorded with its reason and shown as a change to the
list she already agreed to, never as a silent replacement. 过程即数据 — a plan
that changed three times is a more honest record than one that never did.

## 10 · Hook questions and branches

The prototype opened a branch only off an approach. The brief is wider:

> whenever need think, design, review, make decisions, provide hook questions,
> ask students to think deeply, students can generate a new chatting branch to
> think deeply.

So the anchor is polymorphic — `approach | hook | step | artifact` — and any
印记 message may carry a hook question that opens a branch when tapped. She can
also open one unprompted, from anywhere.

A branch does not close without a `takeaway`. Without one the digging was just
reading, and nothing comes back to the main thread.

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
3. A branch does not close without a `takeaway`.
4. A plan step with an empty `decide` is rejected at validation and regenerated.
   A step where she decides nothing is a step she should not sit through.
5. Nothing runs before she has approved the first plan.
6. A plan revision is rejected without a stated reason.

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
| `pbl_thread_item` | the main conversation: says / step / make / handoff — **only if `atom_message` cannot carry it** |
| `pbl_branch` | a think-deeply thread: polymorphic anchor + `takeaway` |
| `pbl_branch_turn` | messages inside one branch |
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
enforcement, artifact settle rules, branch closure, publish-token issue and
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
| S2 | the living tasklist · step loop · chat · hook questions · branches with takeaways |
| S3 | sub-agent production · artifacts · the settle-with-a-reason gate · `handoff_tool` · `summon_tool` endpoint |
| S4 | images — DashScope → OSS |
| S5 | her website · research and teaching content · renderer · preview → modify → online · revoke |
| S6 | 复盘 and `keeping` |

The method layer (§6) lands into S2 and S3 when the owner's design arrives.

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

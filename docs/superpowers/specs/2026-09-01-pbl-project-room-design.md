# PBL project room — design

> 2026-09-01 · The first of three sub-projects taking `/eco` into lite for real.
> Order agreed with the product owner: **项目 → 兴趣树 → 探索宇宙**.
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

A room where a student brings a real aim, 印记 proposes steps, and the two of
them work through those steps one at a time — with the judgement staying hers
at every step.

The prototype at `apps/lite-web/src/eco/projects/` settled the shape across
nine rounds. This document says how it becomes real: server-side orchestration,
Postgres state, live model calls, and the gates enforced where they cannot be
clicked past.

## 2 · The boundary

Stated by the product owner on 2026-09-01 and binding on everything below:

> 印记 takes the thinking as far as a **spec**, then hands off to the right
> tool. We do not rebuild what already exists.

| Need | What we do |
|---|---|
| A questionnaire | 印记 designs it; she runs it on 问卷星 or similar; she brings results back |
| A system, a script, an app | 印记 takes her to a spec; she drives Claude Code or Codex; she brings the result back |
| Pictures | **We generate them.** |
| Her personal website | **We host it. This one is ours.** |

This boundary is not a limitation to apologise for. It is the product saying
where it adds something: the thinking layer, and the one artifact that is
genuinely hers to keep.

The boundary needs no new plan concept. `PlanStep` already carries `iBring` /
`youBring` / `then` — what 印记 does, what she does, and what she reports back.
A handoff is that split with an external tool named in it: 印记's half ends at
the spec, hers is executing it elsewhere, `then` is the meeting point.

## 3 · Deferred: the tool cards

**The card system as it stands is rejected.** Product-owner ruling, 2026-09-01:

> I think previous card system is a failure, because no one would like those
> form-like things. we do need a tool box, but the detailed interaction may
> need to be redesigned. so donot set up empty form-like cards now.

Therefore this build ships **no cards at all**: no authored library, no
`summon_card`, no `mint_card`, no `CardSurface` port, no card tables. A toolbox
is still wanted; its interaction is an open design question that the product
owner will settle separately.

Nothing here forecloses it. `pbl_thread_item.kind` is a text column, so adding
a tool item later is a migration rather than a redesign. We do not build a seam
for an undesigned thing.

Consequence to keep in view: AGENTS.md still describes 工具卡 as one of the two
distinctive capabilities. That framing is not being changed here — the ruling is
that this particular *interaction* failed, not that the toolbox idea is dead.

## 4 · Orchestration

A new package, `apps/api/internal/pbl`.

Rejected: adding a `pbl-project.json` beside `writing-project.json` in
`internal/skills`. That package loads a **fixed contract DAG** — authored
milestones with machine gates (`node_present`, `no_orphan_evidence`) over a
graph of evidence nodes. A PBL plan is generated at runtime and edited by the
student; it is instance data, not config. Forcing one into the other would
reintroduce exactly the fixed step-chain the prototype spent nine rounds
removing, and would make the gate vocabulary check a graph that does not exist.

Also rejected: orchestrating on the client. It breaks the rule that the
decision layer is server-side, loses per-call cost accounting, and makes the
process unevaluable.

What `pbl` reuses rather than re-implements:

- `internal/gateway` — provider mux, SSE, `llm_call` cost rows
- `internal/agent` — the tool loop and refeed primitives
- `internal/oss` — presigned upload and CDN read
- the session/auth and org membership checks already in `internal/api`

## 5 · Data model

New tables, all prefixed `pbl_`. The prefix is deliberate: the sharpest way a
lite change breaks pro is by **creating** a name that already exists, and pro
owns `project`.

| Table | Holds |
|---|---|
| `pbl_project` | aim, cover, status, owner, timestamps |
| `pbl_approach` | one road 印记 proposed: shape, how, **costs**, needs |
| `pbl_decision` | chosen approach + `why` + `gave_up` |
| `pbl_plan_step` | ordinal, title, blurb, goal, you_bring, i_bring, **decide**, then, opens, off, mine |
| `pbl_thread_item` | the project's main conversation: says / step / make / handoff |
| `pbl_branch` | a think-deeply side thread: polymorphic anchor + `takeaway` |
| `pbl_branch_turn` | messages inside one branch |
| `pbl_artifact` | kind, payload, `guessed[]`, `admits[]`, verdict, `why` |
| `pbl_site` | her website: content, layout choice, publish token, revoked_at |

Every model call on this surface writes an `llm_call` row with tier, tokens and
cost, per AGENTS.md. No exceptions for streaming turns.

## 6 · What 印记 can call

The turn loop is the existing gateway. What is new is the tool set. **Anything
not on this list happens as prose in the chat** — the panel has to earn its
place, and a question that is just a question gets asked one at a time.

| Tool | Effect |
|---|---|
| `propose_roads` | 2–3 approaches, each naming its own `costs` unprompted |
| `propose_plan` | ordered steps, each with a `decide` |
| `hand_over` | an artifact, with `guessed[]` and `admits[]` both required |
| `handoff_tool` | names the external tool, why it fits, and what she brings back |
| `open_branch` | opens a think-deeply thread off any anchor |

## 7 · Hook questions and branches

The prototype opened a branch only off an approach. The product owner's brief
is wider:

> whenever need think, design, review, make decisions, provide hook questions,
> ask students to think deeply, students can generate a new chatting branch to
> think deeply.

So the anchor becomes polymorphic — `approach | hook | step | artifact` — and
any 印记 message may carry a hook question that opens a branch when tapped.

A branch does not close without a `takeaway`. Without one the digging was just
reading, and nothing comes back to the main thread.

## 8 · Gates, enforced on the server

The prototype enforced these by disabling buttons. A disabled button is
decoration. These live in the handlers, and the endpoint refuses:

1. An artifact does not settle without a `why`.
2. A decision needs both `why` and `gave_up`.
3. A branch does not close without a `takeaway`.
4. A generated plan step with an empty `decide` is rejected at validation and
   regenerated. A step where she decides nothing is a step she should not be
   sitting through.
5. Nothing runs before she has approved the plan.

## 9 · Artifacts

`kind ∈ { options, draft, spec, image, site }`.

`site` and `pbl_site` are two different things and the distinction matters: an
artifact of kind `site` is one **round of handover** — this is what I built,
here is what I guessed, here is what is still wrong — while `pbl_site` is the
one live page that her accepted rounds have produced. Rounds accumulate; the
page is singular.

`spec` is new and carries the boundary: it is the deliverable for everything we
have decided not to build. It renders as a document she can read, revise, copy
and take to another tool.

Every artifact arrives with what 印记 **guessed** and what it **admits** is
still wrong with its own work. A handover that hides its assumptions can only
be accepted, never reviewed — and reviewing is the point.

## 10 · Images

Aliyun DashScope (通义万相). Same account as ECS, OSS and CDN, so no new vendor
and no new region, and it holds the China-first rule. Server-side key only.
Output goes through the existing OSS presigned path and is served from CDN.

## 11 · Her website

Design settled in spec revisions v8 and v9; this build changes none of it.

- Three genuinely different layouts — `Essay`, `Ledger`, `Magazine` — not one
  layout in three colourways. Her justified choice has to be visible.
- `site/` may not import the app's UI kit (`Btn`, `Panel`, any `mk-*` token).
  The moment it does, the page starts looking like the product that made it.
- The banner is the site's own drawn artwork, never a crop of one of her
  projects. Those do different jobs.
- Content comes from her real readings, writings and projects.

**Visibility: an unlisted, revocable link, `noindex`.** Product-owner decision,
2026-09-01. This reuses the pattern lite already ships for report sharing
(`/s/:token`). She gets a real address to send to a parent or a friend, and a
minor's name and school do not become searchable under our domain. She can
revoke it.

`projects/PageStudio.tsx` — the dead six-step wizard with no entry point — is
deleted rather than ported.

## 12 · Frontend

`apps/lite-web/src/projects/`, a third tab beside 阅读 and 写作.

Ported from the prototype: the workbench shape (chat left, panel right, panel
widens), `Planner`, `StepIntro`, `Roads`, `BranchTalk`, `Overview`,
`CoverPicker`, and `site/` verbatim with its no-UI-kit rule intact.

Not ported: `store.tsx`, the `sessionStorage` layer, the timer theatre in
`Make.tsx`, the scripted replies in `data/plan.ts`, the invented news, and every
card surface. Those existed so the design could be settled without a backend.

## 13 · Dark theme

Token work lands in the first slice so every new surface is born theme-aware
rather than retrofitted. Retrofitting pro's existing screens is a separate pass.

🚨 `mk-*` tokens are bare CSS variables, so **every** Tailwind alpha syntax
emits no CSS at all. Use `color-mix` or `linear-gradient`.

## 14 · Testing

Logic tests only: plan validation, gate enforcement, artifact settle rules,
branch closure, publish-token issue and revoke, cost accounting, and the
request normalizers. No assertion-per-rendered-element suites — they break on every
honest redesign and catch nothing.

The UI is checked by looking at it in a real browser with a Playwright
screenshot. On 2026-08-30 lite had 344 green tests sitting on top of an exported
report PNG that was completely blank.

## 15 · Slices

| | |
|---|---|
| S1 | tables · create a project · generated plan · her approval before anything runs · dark-theme tokens |
| S2 | step loop · chat · hook questions · branches with takeaways |
| S3 | artifacts · the settle-with-a-reason gate · `handoff_tool` |
| S4 | images — DashScope → OSS |
| S5 | her website · renderer · publish and revoke |

Deferred pending the product owner's redesign: the toolbox.

## 16 · Open questions

1. **The toolbox interaction.** Owned by the product owner. Everything else
   here is built so that it can arrive later without rework.
2. **Where a project's aim comes from.** The prototype offered both a track
   picker and a free composer. Both are ported; whether one wins is a question
   to answer after watching a student use it, not before.
3. **Evaluation.** A finished project should produce a process evaluation, as
   readings and writings already do. Out of scope here; it needs the interest
   tree's keyword layer to be worth much, so it belongs after sub-project two.

# 多智能体架构 · 思维印记 as one continuous project

> North-star design draft (2026-07-29). Turns "AI 工具卡在对的位置" into a system: **one continuous main agent, a small set of context-isolated sub-agents, and a durable project spine that chat folds into.**
> Companion: `2026-07-29-card-placement-map.md` (which tool at which moment). Aligns with the backend north-star in `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`.

## 1. Principles (unchanged 铁律 + one new axis)

- **AI 克制** — the agent hands thinking back; never concludes for the student.
- **一次只问一个 · 不操纵 · 过程即数据** — the four 铁律 hold.
- **我们占「思考」这一层** — we don't own the student's document; the spine records *thinking*, not the artifact.
- **NEW governing axis — context engineering.** The product is a long, continuous project. The architecture exists to keep **one coherent conversation** affordable and focused. Its levers: **memory** (cross-project, who the student is) → **spine** (per-project durable state) → **rolling window** (recent raw chat), with **sub-agent isolation** (heavy blobs out), **compaction** (chat → spine), and **projection** (compact context per turn). Everything below is an application of these six.
- **Three things the agent wields, kept distinct:** **skills** (the agent's own procedures — *how it does things*), **卡 cards** (student-facing thinking tools — *what it hands the student to do*), **透镜 lenses** (a reading sub-type). Skills are the runtime; cards/lenses are content it deploys.

## 2. Mental model — one agent, many tools, one spine

There is **one 印记** the student talks to, from "here's my homework" to writing. 立题 / 计划 / 文献库 / 读文章 / 写作 / 回顾 are **surfaces (moments/tools)**, not gates and not separate agents — students are continuous and jump around. The **project spine** is the durable state every surface reads and writes; the **conversation** is a rolling window folded into the spine as it goes.

```
                       ┌─────────────────────────────────────────┐
   student  ◄────────► │        MAIN AGENT (印记, one thread)       │
  (any surface)        │  sys-prompt(克制) + spine projection +     │
                       │  rolling chat window + active surface +   │
                       │  that surface's card/lens deck            │
                       └───┬───────────────┬───────────────┬───────┘
              in-thread skills        spawns sub-agent   spawns sub-agent
        (proposal-shape, plan-gen,   (context isolation)  (context isolation)
         outline, writing-feedback,        │                    │
         rabbit-hole, card-summon)         ▼                    ▼
                       ┌───────────────────────┐   ┌───────────────────────┐
                       │  READING SUB-AGENT      │   │  ASSESSMENT SUB-AGENT  │
                       │  per source             │   │  at 回顾 / finish       │
                       │  in: scope+why+proposal │   │  in: full process       │
                       │  isolated on: full text │   │  isolated on: everything│
                       │  out: takeaways         │   │  out: rubric + 印记 prose│
                       └───────────┬────────────┘   └───────────┬────────────┘
                                   └──────── write back ─────────┘
                                          THE PROJECT SPINE
        metadata · proposal · plan · reading-list(phase-tagged + takeaways)
        · outline(+resources) · activity log · conversation digest
```

## 3. Agent topology — the isolation test

**Rule:** spin off a sub-agent ONLY when a task must *ingest something big the main thread should never keep* and can *return something small and structured*. Otherwise it's an in-thread skill (the main agent thinking with a focused prompt over the spine it already holds).

| Unit | Kind | Why | In | Out |
|---|---|---|---|---|
| **Main agent (印记)** | continuous orchestrator | the single thread; owns spine + rolling window; routes surfaces; calls skills/cards; runs compaction | spine projection + rolling chat + surface | dialogue + spine writes |
| **Reading a source** | **sub-agent** | full article (10k+ tok) must never enter the main thread | project scope + why-read-this + relevant proposal | takeaways (findings, credibility, key quotes, new leads, proposal-impact) |
| **Assessment** | **sub-agent** | ingests the whole process to grade; flagship, never-downgrade | full spine + folded history + card outcomes | rubric (D6/A6/lenses) + 思维印记 prose |
| proposal-shape · plan-gen · outline · writing-feedback · rabbit-hole · card-summon | **in-thread skills** | operate on the spine the agent already has — isolation buys nothing | focused prompt over spine | structured spine delta or reply |

Notes:
- **回顾 splits.** The *reflective conversation* (incl. the new "复盘我与 AI 的互动" component) is the **main agent** on the review surface. The *graded pass* is the **assessment sub-agent**. Review is where the project **finishes** → it is the major finalization/compaction point (§5).
- The reading sub-agent is the reusable template; assessment already works this way today.

## 4. The context stack — memory → spine → window

Three durability tiers. Each turn's context is *engineered* from them, not dumped.

- **Memory (cross-project, student-level).** Who the student is: qualification, recurring thinking patterns, prior projects and their 印记, standing preferences, growth over time. Outlives any one project; personalizes the coach and feeds the growth/teacher/parent reports. Written slowly (a project's assessment distills into a memory delta), read as a thin projection.
- **Spine (per-project, §below).** The durable state of *this* project.
- **Rolling window (in-flight).** Recent raw turns; older ones already folded into the spine (§5).

Guardrail: memory holds *thinking patterns and process*, never a store of the student's documents (铁律 — we occupy the thinking layer). Memory writes are inspectable; nothing manipulative accumulates.

### The project spine

Everything the agent needs for *this* project, structured, small:

| Spine slice | Contents | ~exists today |
|---|---|---|
| **metadata** | target, scope, qualification, status | `project` |
| **proposal** | objective / reason / activities / resources — *matures over time* | `project_proposal` |
| **plan** | milestones + per-step duration + **完成度校验** (do these finish the project?) | `plan_item` (needs milestone/duration/completeness fields) |
| **reading list** | sources, each **phase-tagged** (立题时用…, reusable later), status, **takeaways**, rabbit-hole branch links | `reference`/`collection` (needs phase_tag + takeaway + branch) |
| **outline** | nodes + **attached resources** | `outline_node` (needs resource attach) |
| **activity log** | the process record | `activity_log_entry` |
| **conversation digest** | folded older chat (see §5) | **new** |

**Projection, not dump.** Each main-agent turn sees a *compact projection* of the spine (proposal summary, plan status, reading-list index with one-line takeaways, outline skeleton, recent activity) — full details are fetched on demand via a skill. The projection is itself a context-management lever.

## 5. Compaction contract — the spine IS the target

Never "summarize chat into a blob." Continuously **convert conversation into durable structure**, keeping only a rolling window of raw turns.

1. **Event-driven (primary): fold an artifact into the spine the moment it solidifies.**
   - Proposal v1 finalized → the shaping dialogue distills into `proposal`; those raw turns leave the window.
   - A reading sub-agent returns → takeaways land on the source; **its transcript + the article never entered the main thread at all.**
   - Plan generated / outline section settled / a card completed → folded likewise.
2. **Finalization point: 回顾 / finish.** The project completes here → assessment sub-agent runs, and the whole process compacts into the evaluation + 思维印记. This is your "review = compaction point OR agent" — it is **both**: a compaction boundary *and* where the assessment agent fires.
3. **Size-threshold backstop:** if the raw window still exceeds a token budget, fold the oldest turns into `conversation digest`. **Not time-based** — time is the wrong axis.

Because the two heavy blobs (article bodies, full-process grading) are isolated in sub-agents, the main thread stays mostly *dialogue + spine* and compaction pressure is low.

## 6. The reading sub-agent contract (the reusable template)

**Brief in** (from orchestrator, small):
```
{ project_scope, qualification,
  reason_for_reading,          // why THIS source, here, now
  proposal_snapshot,           // relevant dims only
  reading_focus? }             // optional student-stated points
```
**Isolated on:** the full material blocks + the reading card/lens deck. Runs the existing read-together loop (hang card on sentence → student finds evidence → 3-check verdict).

**Takeaways out** (to spine, small):
```
{ findings: [...],             // what she concluded, in her words
  credibility: verdict+why,    // CRAAP/SIFT outcome
  key_quotes: [{quote, why}],  // the sentences that matter
  new_leads: [...],            // rabbit-hole branches this surfaced
  proposal_impact: string }    // how this should move the proposal/argument
```
The orchestrator writes these onto the reading-list entry and folds `proposal_impact` (one line) into the spine. **The article body and the reading dialogue are discarded from the main thread.**

## 7. Phases as moments (not gates)

- **Focus transitions** are AI-detected, **student-confirmed** (铁律: triggering is automatic, *opening* is the student's). e.g. mid-goal-chat the AI hears "I think I want to do X" → offers to move into proposal-forming; a pasted link → offers the reading room.
- **Reading room reachable from anywhere**, including mid-立题. It's briefed from the spine, so it feels continuous; it writes back takeaways.
- **Phase-tags** on reading-list sources record provenance ("用于立题") without locking reuse.
- **Summary-on-return**: opening a project regenerates a compact paragraph from the spine (same first-open-prose pattern as the mirror / parent reports).

## 8. Skills · cards · lenses — three registries

| | What | Who runs it | Visible to student? | Examples |
|---|---|---|---|---|
| **Skills** | the agent's own procedures — *how it does a thing* | the agent (some spawn a sub-agent, most in-thread) | no (it just happens) | proposal-shape, plan-gen, rabbit-hole-guide, reading-loop, assess, compact, summary-on-return |
| **卡 Cards** | student-facing thinking tools — *what she does* | the student, on a surface | yes (she confirms opening) | T00–T38 per `card-placement-map.md` |
| **透镜 Lenses** | disciplinary reading angles (a card sub-type) | the student, in the reading room | yes | the 9 学科透镜 |

- **Skills already exist as a runtime abstraction** (the "one-runtime + skills + policies" refactor; Go `internal/skills`, `.claude/skills`). The in-thread "skills" in §3 map onto that registry; a sub-agent (reading, assessment) is a **skill that happens to isolate context**. So §3's topology is really *skills, some of which spawn sub-agents*.
- **T23–T27 (SOLO/agency-gradient/gradual-release/one-question/AI-restraint/model-routing) are skills/policies, not cards** — the agent's teaching behavior, never in a student deck.
- The active surface exposes its native card/lens deck; the orchestrator summons into that surface with that surface's interaction. **Later:** propose cards/lenses *in chat* across phases (克制 ladder → respond / hint / summon).

## 9. What we have vs. what's new

**Have:** most spine tables (0036), a working reading room (flagship gateway), isolated assessment (flagship never-downgrade), metering/entitlement, per-room `/coach {scope}`, a **skills runtime** (`internal/skills`), growth/teacher/parent report history per student (proto-memory).

**New / to build:**
1. **One continuous conversation** store per project (replaces per-room coach scopes as separate threads) + the projection.
2. **Compaction contract** — fold-on-solidify + size backstop + `conversation digest`.
3. **Reading room formalized as brief-in / takeaways-out sub-agent** (today it writes back only card outcomes, not takeaways) — i.e. a context-isolating skill.
4. **Spine field additions** — plan milestones/duration/completeness; reading-list phase_tag + takeaway + branch links; outline resource attach.
5. **AI-driven focus transitions** + summary-on-return.
6. **Student-level memory** — promote the scattered growth/report history into a first-class cross-project memory (projection in, assessment-distilled deltas out).
7. **Skills as the orchestrator's registry** — the sub-agents and in-thread procedures registered/routed uniformly (some isolate context, some don't).

## 10. Decisions (resolved by principle: student-experience first · performance > cost · educational · non-manipulative)

- **D1 · Conversation store = one append-only thread, surface-tagged.** Continuity IS the experience; each room renders its slice via tags. (student-experience)
- **D2 · Generous always-on projection, deep-fetch on demand.** Proposal summary + plan status + reading-list index (one-line takeaways) + outline skeleton + recent activity + a thin memory slice ride every turn; full detail is a skill call. The coach must never feel amnesiac — and performance outweighs the extra tokens. (performance > cost)
- **D3 · Takeaway schema = the five fields (§6), extensible.** Reused as the return shape for future context-isolating skills.
- **D4 · Focus transitions decided by the main model inline, student-confirmed.** No separate per-turn classifier — the main model already holds the context, so its judgment is better (performance) and cheaper (one call, not two); the student still confirms opening (铁律). (performance + 铁律)
- **D5 · Memory = patterns/process/growth only, never documents.** Primary write = at project finish, the assessment distills one memory delta (high-signal, low-noise, low-cost); a light touch may capture a durably stated preference (e.g. "IB Bio HL"). Inspectable, student/teacher-visible, carries no engagement-optimizing signal. (educational + non-manipulation 铁律)
- **D6 · Two nested routing layers.** An **operational skill-router** (which skill to run; does it isolate into a sub-agent?) wraps the **pedagogical 克制 ladder** (respond / hint / summon a student card). Card-summon is one skill; 克制 stays pure pedagogy inside it, free to evolve independently of efficiency-driven skill routing. (keeps 克制 clean; performance)

## 11. Slicing

Suggested slices (each spec → build → verify, per house style):
- **S1 · Spine + continuous thread**: conversation store + projection + fold-on-solidify for proposal/plan; summary-on-return.
- **S2 · Reading sub-agent contract**: brief-in / takeaways-out; write takeaways to reading list; phase-tags.
- **S3 · Reading-list / rabbit-hole surface**: exploration tree over the source graph + P3 cards.
- **S4 · Compaction backstop + digest**; cross-phase card proposing.
- **S5 · Review finalization**: reflective conversation + AI-interaction retrospective feeding the assessment sub-agent.

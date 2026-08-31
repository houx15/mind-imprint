# Building the /eco prototype into real lite features — handoff brief

> For the **next session**. A previous session built the `/eco` prototype to a
> settled state; this is everything it hands over: what the prototype is, what it
> is not, what already exists in the real codebase, which mistakes will make you
> reimplement a working system, and where to cut the first slice.
>
> **Read these three before touching any code:**
> 1. This file.
> 2. `docs/2026-08-30-ecosystem-prototype-spec.md` — the prototype spec, with nine
>    rounds of revisions (v1–v9). Each round records why the design is what it is
>    and why the previous version was rejected.
> 3. `docs/2026-08-31-pbl-tool-model.md` — when a tool runs inside the chat and
>    when it earns the right-hand panel.
>
> Every hard constraint in `AGENTS.md` still applies. This file only adds what
> `AGENTS.md` does not cover.

---

## 0 · Where the prototype lives

`/eco` exists **only on the `worktree-reading-cards` branch**. There is no
`apps/lite-web/src/eco/` on `origin/main`.

```
git fetch origin
git checkout worktree-reading-cards
```

The product owner has decided the build continues **on this branch** — it does
not need to be merged to main first.

---

## 1 · What the prototype is, and is not

`apps/lite-web/src/eco/` is roughly **15,600 lines and entirely front-end**:
state in `sessionStorage` (`store.tsx`, key `mk-eco-proto-v6`), no API, no model
calls, waiting faked with timers, branch replies scripted, news invented.

**What it settles is shape, behaviour and copy** — all three have been through
nine review rounds. It is **not** an implementation to port.

### Do not port

| Not this | Because |
|---|---|
| The `sessionStorage` model in `store.tsx` | Real state belongs in Postgres, via Go |
| `data/news.ts` (816 lines of invented news) | Where news comes from is unresolved — see §6 |
| The timer theatre in `Make.tsx` | Real streaming replaces it |
| The scripted branch replies in `data/plan.ts` | Real model calls replace them |
| `projects/PageStudio.tsx` | The old six-step homepage wizard; already unreachable in the prototype, awaiting a decision to delete |
| The 「先看看做完的样子」 toggle in `page/MyPage.tsx` | Demo-only affordance |

### Do keep

- **The copy.** Two of the nine rounds were spent on it — one purging invented
  jargon, one after the page was rejected for not reading like a personal site.
  Rewriting the copy throws those reviews away.
- **The three site layouts** (`site/Essay.tsx`, `Ledger.tsx`, `Magazine.tsx`) and
  the drawn `Banner` in `site/parts.tsx`. They were built against six real
  personal sites and can move into the real implementation nearly as-is.
- **The interaction order**: one question at a time; 印记 states what it guessed
  before handing work over; nothing settles without a written reason.

---

## 2 · Five surfaces, very different costs

| Surface | Directory | The hard part |
|---|---|---|
| 世界 (news planets) | `world/` | **Where the news comes from is unresolved** — likely the most expensive piece |
| 我的树 (keyword growth) | `home/` | Data already exists in lite (readings, writings); mostly projection and layout |
| 项目 (the PBL workbench) | `projects/` | **The core**, and where most of the genuinely new work is |
| 我的主页 / the built site | `page/`, `site/` | The deliverable — see §4 for the architecture call |
| 印记 drawer | `coach/` | Reuses the existing coach; cheapest |

---

## 3 · What the real codebase already has (the prototype knows none of this)

**This section is the main reason this file exists.** Follow the prototype
literally and you will reimplement several systems that already work.

### 3.1 `summon_card` already exists server-side

`apps/api/internal/api/summoncard.go`, plus `agent/prompt.go`,
`agent/orchestrator.go`, `agent/messages.go`. The decision layer — the system
prompt offering the model a `summon_card` tool, the model calling it with a
stated reason — is implemented. **Do not build a second one.**

### 3.2 lite's reading room already runs the whole card loop

`apps/lite-web/src/readings/CoachCard.tsx`, `ReadingCoachPanel.tsx`, and
`api/readingRoom.ts` (`postReadingCoachTurn`, `listReadingCards`, `coachCardOf`,
`coachAnswerOf`).

**summon → render → submit → feed back into the conversation** is live in lite
today. The project room should copy that seam rather than invent a parallel one.

### 3.3 🚨 The prototype's card model and the shipped one are different

**This is the first decision to make, before any code.**

Shipped: **34 JSON specs** in `packages/contracts/cards/`, mirrored exactly in
`apps/api/internal/cards/specs/`. The schema is rich — `primitive`,
`target_type`, `params`, `completion`, `graph_effects`, `observe`, `rubric_dims`,
`disclosure_tier`, `trigger_condition`, `trigger_keywords`.

Prototype: **20 cards invented in `data/cards.ts`**, with a much thinner shape —
`kind`, `surface`, `payoff`, and `fields` drawn from
`text | textarea | choice | multi | rows`.

Shipped `placement` values are `reading`, `reading-toolkit`, `cross-cutting` and
`status`. **There is no `project` placement.**

So the prototype's cards are *not* a matter of adding a few JSON files. Two
routes:

- **(a)** Map the 20 onto the existing primitives (`annotate`, `matrix`, `sort`,
  `compare`, `graph`); drop what will not map. Smallest change, sacrifices some
  of the prototype's interactions.
- **(b)** Add a `form`-style primitive (exactly the prototype's field vocabulary)
  and a `placement: "project"`. Larger change, but all 20 come across intact.

**(b) is the better fit** — the prototype's field primitives are precisely what
`AGENTS.md` means by "a new card is a new JSON file and never new renderer code".
But this is a product decision: **ask, do not pick one and start**.

Related: `surface: chat | panel` — whether a tool is asked inside the chat or
opens the right-hand panel — is a prototype invention with no server-side
equivalent. It is the central conclusion of
`docs/2026-08-31-pbl-tool-model.md` and worth preserving.

### 3.4 Card JSON is mirrored in two places

`packages/contracts/cards/` and `apps/api/internal/cards/specs/` must change
together. Edit one and the other end goes quietly out of sync.

---

## 4 · One architectural recommendation: how the built site is produced

In the prototype, 印记 "building the site" is fake — `site/BuiltSite.tsx` is a
React component. For the real thing there are two routes, and the second is
strongly preferred:

- ❌ **Have the model emit HTML/CSS.** Attractive, but: each round of her
  feedback becomes an unreviewable code diff; the layout breaks unpredictably;
  the injection surface is wide; tokens are expensive.
- ✅ **Structured content plus fixed templates.** The model fills the
  `SiteContent` model in `data/site.ts` (name, one-line role, post blurbs,
  project descriptions, site stats…) and picks a layout; rendering stays with the
  finished `Essay` / `Ledger` / `Magazine` components.

The second route lands exactly where this product needs it: **each round of her
feedback becomes a concrete change to a content model**, so "here is what changed
in this version" can be shown line by line — which is the whole design intent of
the three build rounds (see spec §v8 D: 印记's admissions have to become visible
pixels).

**Publishing**: lite already has a public-link mechanism (`/s/:token` — see how
`apps/lite-web/src/rootElementFor.tsx` mounts `PublicReportPage` outside
`LiteApp`). Build `/p/:handle` on that pattern rather than inventing another.

---

## 5 · Suggested slices

Each ships independently. **S1–S4 is the demo spine**; S5 and S6 can wait.

| Slice | Content | Notes |
|---|---|---|
| **S1** | Project data model + room shell, **no AI yet** | Create a project, generate a plan from a template, persist the chat log, open/close the right panel. Real DB, real routing |
| **S2** | Cards inside the project room | Reuse the seam from §3.2 — **blocked on the §3.3 decision** |
| **S3** | 印记's outputs: the `options` and `draft` kinds | The two cheap ones first; make "nothing settles without a reason" real |
| **S4** | The built site | `SiteContent` + three templates + a published URL (see §4) |
| **S5** | The questionnaire artifact | Needs real recipients and real replies; heaviest, and cuttable |
| **S6** | Tree + world | Independent of the project flow; can run in parallel or later |

---

## 6 · Questions for the product owner — do not guess

1. **Does the /eco 项目 flow fall under `docs/2026-08-09-all-statuses.md`?** That
   document is the single source of truth for the **writing project** lifecycle;
   /eco's PBL project is a different one. `AGENTS.md` requires any plan touching
   the writing-project flow to be validated against it clause by clause.
   **Settle whether this counts before designing any state machine.**
2. **Where does the news on the 世界 surface come from?** A real feed, human
   curation, or an editor tool? The cost differs by an order of magnitude.
3. **Which of the 20 prototype cards are really existing specs?** `craap` and
   `concession` clearly overlap. Reuse those instead of building duplicates.
4. **Delete `PageStudio.tsx`?** It has no entry point in the prototype any more.
5. **Route (a) or (b) in §3.3.**

---

## 7 · Gates, for every slice

```
# front-end
cd apps/lite-web && npx tsc --noEmit && npx vitest run

# back-end (both flags matter — they are scar tissue)
cd apps/api && CGO_ENABLED=0 go test ./... -timeout 1800s
```

And one gate that is not automated but matters more:

> **Look at the UI in a real browser. Take a Playwright screenshot, then
> actually look at the image.** The lesson from 2026-08-30: lite had 344 green
> tests sitting on top of an exported report PNG that was completely blank.
> "Logic tests only, no assertion per rendered element" is a hard rule — see
> `AGENTS.md`.

---

## 8 · The rules most easily broken

- **The client never calls a model directly.** Keys live server-side only; every
  call records tier, tokens and cost.
- **lite must never break pro.** Shared Go packages, `queries/`, `migrations/`,
  the sqlc directories — the classic way to destroy pro code from a lite task is
  to **create a file that already exists**.
- **AI failures must surface.** Never return a plausible canned sentence. If a
  model call or a parse fails, say it failed.
- **No `不是…而是` antithesis in copy**, Chinese or English. Positive
  declaratives only.
- **The scope of 铁律 (important).** "The AI never writes it for her" governs
  **the student's own body text only**. On the project side the product owner
  explicitly revised this: **印记 may do the work — write the code, lay out the
  page, generate the images — while the judgement stays hers.** Do not re-apply
  "the AI must not produce anything" to the artifact layer; that layer *is* the
  design.

---

## 9 · First message to paste into the new session

> I want to build the `/eco` prototype into real lite features. The prototype is
> on the `worktree-reading-cards` branch under `apps/lite-web/src/eco/` (it does
> not exist on main), and we are building on that branch.
>
> Read `docs/2026-08-31-eco-to-lite-build-brief.md` first, then the two documents
> it points to. Then do **one thing**: produce an implementation plan for S1
> (project data model + room shell, no AI yet). In the plan, answer the five
> questions in §6 of the brief explicitly — for the ones that need my decision,
> list them and ask me rather than picking one and proceeding.
>
> Do not write code yet.

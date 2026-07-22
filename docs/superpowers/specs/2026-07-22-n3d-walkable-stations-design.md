# N3d · Walkable stations (S0–S2) — Design

Date: 2026-07-22
Slice: N3d of the student-platform finishing decomposition
(`docs/2026-07-20-student-platform-remaining-work.md`)

---

## 1. What is actually broken

The tracker lists N3d as three items (R-9 reveal, search-plan card, S2 perspective
view). Reading the code first turned up something larger underneath them.

**`agent.Advance` has zero production call sites.** It is the only function that
sets `RecordedGate.Confirmed`, and `Confirmed` is the only thing
`agent.CheckGate` reads into `GateReport.Solid`. `internal/studio/projection.go`
builds the station rail off `Solid`: the first non-solid contract is `head` →
`state:"current"`, everything after it is `state:"locked"`. So for any project
that a student actually creates, **no station ever becomes `done` and the head
stays pinned at S0 forever.**

The only reason nobody has seen this is that
`internal/store/migrations/0018_seed_demo_project.sql:56–60` hand-writes
`gate_state` nodes with `"confirmed_solid":true` for `decode_task`,
`frame_question`, `evaluate_perspectives` and `evaluate_sources`. The demo project
starts at S4 because the migration says so, not because anything computed it.

Underneath that, the producers are missing too:

| Station | Gate demands | Live producer |
|---|---|---|
| S0 `decode_task` | `rubric_translation`, `weakness_prediction` ×2, `milestone_plan` (SW) | `rubric_translation` only (`project_create.go:78`). The onboarding submit mints `task_restatement`, a type no gate names. |
| S1 `frame_question` | `research_question`, `provisional_answer`, `preregistration`, `terms_defined` (SW) | **none of the four, anywhere in the repo** |
| S2 `evaluate_perspectives` | `perspective` ×2, `recon_logged` (SW), `sources_per_perspective` (SW) | `perspective` only, via N3a's `perspective-matrix` card |

(SW = `student_written`, recorded externally per DEC-3.)

And both S1 and S2 render a literal placeholder — `ShellView` in
`apps/web/src/studio/views/OnboardingView.tsx:183`, reading
「此环节的深入交互将在后续切片接入」 — even though `docs/design/思维印记_工作区.dc.html:874–958`
specifies both screens in full.

This is the same defect class this project has already shipped three times (a
level with no live producer is dead code), one layer up: **the station chain has
no live producer.**

## 2. Scope

**In:** make S0 → S1 → S2 → S3 walkable by a real student, for the first time.
Every gate item on those three stations gets an honest producer, the two
placeholder views become the screens the binding design specifies, and gate
advancement gets a live caller.

**Out, deferred to an N3e:**
- **The search-plan card** (the AI questioning her retrieval plan). It needs its
  own coach design — a new card spec, a prompt, and a summon rule. S1's
  「打算去哪找证据」 panel ships as her own writing, which is what the design shows;
  the card attaches to it later without reshaping anything here.
- **The R-9 summing-up framework reveal.** `framework_fill` is written server-side
  (`agent/card_lifecycle.go:77`) and read by nobody. Orthogonal to stations.

**Not in scope and not deferred — deliberately unchanged:** no migration, no LLM
call, no new gate predicate kind, no change to `writing-project.json`'s contract
definitions. Every gate item listed in §1 is satisfied as written.

## 3. The advance path

### 3.1 `agent.AdvanceAll`

New in `internal/agent/planner.go`:

```go
// AdvanceAll confirms every contract that is now genuinely finished, in DAG
// order, and returns the ids it newly advanced.
func AdvanceAll(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill) ([]string, error)
```

Walks `sk.TopoOrder()`. A contract advances iff:

1. it is not already `Solid`, **and**
2. every id in its `Requires` is `Solid` — either already recorded, or advanced
   earlier in this same walk, **and**
3. `CheckGate(...).Missing` is empty.

Condition (2) is the important one: `Advance` on its own only inspects a
contract's own gate items, so without it a later station could flip `done` while
its predecessor was still `current`. Note this is a *stricter* rule than
`Route`'s reachability, which admits a `machine_clear` predecessor — work may
begin once predecessors are structurally sound, but a station is only *finished*
behind finished predecessors.

The walk is single-pass in topological order, so one call can carry several
stations when a single write satisfies more than one. It reuses `Advance` for the
actual confirm + `gate_attempt` event, so DEC-3 still holds: nothing here records
a `student_written` or `human` item.

Ends by calling `Replan(..., "advanced")` when at least one contract advanced, so
the plan's route (and therefore the projection's `head`) is recomputed in the
same request.

### 3.2 Call sites

A thin API-layer helper:

```go
// advanceGates runs AdvanceAll best-effort. Gate state is derived and
// recomputable, so a failure here must never fail the student's write —
// the next gate-affecting write retries it.
func (a *API) advanceGates(ctx context.Context, projectID uuid.UUID)
```

Logs `slog.Warn` on error and returns nothing. Called at the end of, and only
of, the writes that can change gate state:

- `submitOnboarding` (S0 nodes + attest)
- `submitFraming` (new, S1)
- `submitPerspectives` (new, S2)
- `logSourceOpen` (attests `recon_logged`)
- `attestGate` (the generic student attestation)
- `submitReflection` (S6)
- `submitProjectCard` (card mints feed S2–S4 counts)

Deliberately *not* called from read paths or from `ingestMaterial` (adding a
source never satisfies a gate; `every_source_evaluated` can only get harder).

## 4. S0 任务解码 — the two missing producers

Both land in `submitOnboarding`, which already receives everything needed.

**`weakness_prediction` ×2.** One node per weak pick, `author:"student"`, body
`{"index": i, "plain": <the rubric row's plain-language text>, "origin":
"station_view"}`. The row text is read from the project's existing
`rubric_translation` node so the node is legible to the assessor on its own; if
that node is missing the body carries `index` alone.

Re-submitting must not inflate the count past the `n≥2` gate, so the write is
**delete-then-insert within one transaction**: delete this project's
`weakness_prediction` nodes carrying `origin:"station_view"`, then insert the
current picks. The existing `≤2` validation stays; a student who picks one gets
one node and the gate simply has not passed yet. She is never blocked from
submitting — 铁律 2.

**`milestone_plan` (student_written).** Attested to `"solid"` on the same submit.
S0's deliverable is her restate plus her weakness picks; the milestone plan is
the board-static scaffold she accepts by proceeding. This is exactly the shape
`submitReflection` already uses — an endpoint responding to the student's own
action records the item; `Advance` never does.

## 5. S1 立题

Binding design: `dc.html:874–928`. Three panels below the read-only
RESEARCH QUESTION banner.

### 5.1 `research_question`

Minted at **project creation** (`project_create.go`, inside the existing
transaction, alongside `assignment_brief` / `rubric_translation` /
`milestone_plan`): `type:"research_question"`, `author:"student"`, body
`{"text": <project title>}`. The title is the question she typed in N1's funnel;
attributing it to her is accurate, and the banner renders it read-only exactly as
the design does.

Accepted consequence: projects created *before* this slice have no such node. The
only ones that exist are the seeded demo (gates pre-confirmed by 0018, so it is
unaffected) and local dev projects. No backfill, no migration.

### 5.2 关键概念 · 我的定义 → `term_definition`

**The terms are hers, not a fixed list.** The design hard-codes three terms
(`"sustainable"` / `"China's role"` / `"the world"`) derived from the demo's own
title. A real project has no such list, and the only ways to produce one are an
LLM call or a board fixture that cannot know her question — so instead she names
which words in her own question need defining, then defines them. This is
strictly better against 铁律 1 (the AI never authors her content) and it is why
the panel gains an 「添加一个关键词」 control the design does not have.

Everything else is the design verbatim, including its own quality chip driven by
rune count on the trimmed definition:

| condition | chip | colours |
|---|---|---|
| empty | 待定义 | `#9AA1B0` on `#F1F2F5` |
| `< 15` runes | 偏模糊，再具体点 | `#B8892F` on `#FBF4E2` |
| `≥ 15` runes | ✓ 可检验 | `#4C9A82` on `#E7F3EE` |

Gate chip: 「本环节门禁 · 已定义 {n}/3」, `n` = terms whose definition is ≥15 runes.

Node: `type:"term_definition"`, `author:"student"`, body `{"term", "definition",
"origin":"station_view"}`.

### 5.3 可能的核心论点 → `provisional_answer`

The design's add/delete list, verbatim (「你自己拟——之后在 S4 逐条验证。」). One
node per non-empty line: `type:"provisional_answer"`, body `{"text",
"origin":"station_view"}`.

### 5.4 打算去哪找证据 → `preregistration`

Static chips in the design; here they are hers to write and remove. One node for
the whole list: `type:"preregistration"`, body `{"directions": [...],
"origin":"station_view"}`. Pre-registering where you will look *before* you look
is precisely what the node type names.

This is where the deferred search-plan card attaches.

### 5.5 The endpoint

`POST /api/v1/projects/{id}/framing`

```json
{ "terms": [{"term": "...", "definition": "..."}],
  "answers": ["..."],
  "searchPlan": ["..."] }
```

One transaction: delete this project's `term_definition` / `provisional_answer` /
`preregistration` nodes carrying `origin:"station_view"`, insert the submitted
set (dropping blank rows), and record `terms_defined` → `"solid"` when at least 3
terms have ≥15-rune definitions (clearing it otherwise, so an emptied definition
un-attests honestly). Appends a `framing_written` studio event. Then
`advanceGates`.

No minimum is enforced on the request itself — she can save a half-finished S1 and
come back. The gate, not the form, is what is unfinished.

## 6. S2 视角与素材

Binding design: `dc.html:930–958`.

### 6.1 Perspectives

The design's three levels, verbatim:

| key | label | colour |
|---|---|---|
| `national` | 国家视角 | `#4C9A82` |
| `global_for` | 全球视角 · 支持 | `#2A3B7A` |
| `global_against` | 全球视角 · 反方 | `#C96F4F` |

Coverage chip 「✓ 已覆盖 本地/国家 + 全球」 shows when at least one `national` and
at least one non-`national` perspective exist — the design's own
`s2HasNat && s2HasGlobal` rule.

Node: `type:"perspective"`, `author:"student"`, body `{"text", "level",
"origin":"station_view"}`.

**`perspective-matrix` also mints `perspective` nodes** (`card_effects.go:122`,
body `{text, cells}` — no level, no origin). Two consequences:

- The S2 write is delete-then-insert **scoped to `origin:"station_view"`**, so it
  can never delete what a card minted. This is the reason the marker exists at
  all.
- Card-minted perspectives render in the same list, **read-only**, tagged
  「来自工具卡」 and without level chips (they have no level to show). They count
  toward the `n≥2` gate exactly as her own do — the graph does not care which
  surface asserted them.

### 6.2 The two attestations — one checkbox, not four

Across S0–S2 there are four `student_written` items. Four labelled confirm boxes
is the worksheet R-9 exists to forbid («Four boxes labelled Claim / Counterclaim /
Evidence / Implication is a worksheet», product-spec §283). So three of the four
are recorded by the endpoint that persists the corresponding student writing,
when the structural minimum is met, and only one is an explicit control:

| item | recorded by | condition |
|---|---|---|
| `milestone_plan` | `submitOnboarding` | she submitted S0 (§4) |
| `terms_defined` | `submitFraming` | ≥3 definitions ≥15 runes (§5.5) |
| `recon_logged` | `logSourceOpen` | the project's 检索日志 has ≥1 entry |
| `sources_per_perspective` | explicit confirm | — |

`recon_logged` rides 6b's existing source-log ledger: opening and logging a source
*is* the recon, and `logSourceOpen` is the exact moment it happened. One call
site, no new control, and a student who logged sources before this slice existed
picks it up on her next open.

`sources_per_perspective` gets the one explicit control, because no structural
signal for it exists and inventing one would be fabrication. Copy:
「每条视角我都找到了至少一条素材」. It uses the **existing generic**
`POST /projects/{id}/gate/{contractId}/attest` endpoint (already used for
`citations_matched`), which is reversible — unchecking clears the item. Enabled
only once ≥2 perspectives and ≥1 material exist; before that it renders with the
reason, not hidden.

### 6.3 One deliberate departure from the binding design

S2's design has no way to add a source — yet the station is named 视角与素材, its
`view` tag is 素材, and its own gate demands sources per perspective. As drawn,
the gate is unreachable by construction.

So the S2 view renders, under the perspective list, the existing
`AddSourceForm` plus a compact list of the project's material titles. Both are
components that already exist; no new ingestion path (RL-2 — S2 does not ingest,
it reuses `material.onAdd`). The full dossier with its anchors and locate
machinery stays at S3 where it belongs.

### 6.4 The endpoint

`POST /api/v1/projects/{id}/perspectives`

```json
{ "perspectives": [{"text": "...", "level": "national"}] }
```

One transaction: delete `origin:"station_view"` perspectives, insert the
submitted non-blank rows, append a `perspectives_written` event, then
`advanceGates`. `level` is validated against the three-key enum; an unknown level
is a 400.

## 7. Contracts and projection

Additive only — no existing field changes shape.

`packages/contracts/src/studioState.ts`:

```ts
export const TermDefinition = z.object({ term: z.string(), definition: z.string() });
export const FramingFx = z.object({
  researchQuestion: z.string(),
  terms: z.array(TermDefinition),
  answers: z.array(z.string()),
  searchPlan: z.array(z.string()),
});
export const PerspectiveLevel = z.enum(["national", "global_for", "global_against"]);
export const PerspectiveRow = z.object({
  text: z.string(),
  level: z.string(),      // "" for card-minted rows
  editable: z.boolean(),  // false for card-minted rows
});
export const PerspectivesFx = z.object({
  rows: z.array(PerspectiveRow),
  sourcesPerPerspective: z.boolean(),  // the recorded attestation
});
```

plus `FramingSubmitBody` / `PerspectivesSubmitBody`, and two new members on
`StudioProjection.views`: `framing`, `perspectives`. Mirrored in Go as
`FramingDTO` / `PerspectiveRowDTO` / `PerspectivesDTO` with
`projectFraming(d)` / `projectPerspectives(d)` in `internal/studio/projection.go`,
following `projectOnboarding`'s shape (read the nodes, defend on unmarshal
failure, never fabricate).

`sourcesPerPerspective` is read off the recorded gate state
(`RecordedGatesFromNodes(d.GateStates)["evaluate_perspectives"].Items[...] ==
"solid"`), the same way `canFinish` reads `whole_draft_review`.

## 8. Web

- `apps/web/src/studio/views/FramingView.tsx` (new) — S1.
- `apps/web/src/studio/views/PerspectivesView.tsx` (new) — S2.
- `OnboardingView.tsx` — **`ShellView` is deleted**, along with its
  「此环节的深入交互将在后续切片接入」 copy. After this slice nothing renders it, and a
  dead placeholder that no route can reach is exactly the kind of thing that
  quietly comes back.
- `ViewFrame.tsx` — the `isOnboarding = S0 || S1 || S2` collapse is replaced by
  three distinct branches: S0 → `OnboardingView`, S1 → `FramingView`, S2 →
  `PerspectivesView`. `StationView` gains no new enum member; the routing is by
  station code, as it already is.
- `StudioContainer.tsx` — two new callbacks (`onSubmitFraming`,
  `onSubmitPerspectives`) plus reuse of the existing `attestGate` client for
  `sources_per_perspective`, each followed by the existing `refetchProject`.
- `apps/web/src/api/projects.ts` — `submitFraming`, `submitPerspectives`.

Both views hold local draft state and save explicitly (a 记下 button per panel),
matching `S0View`'s existing submit shape rather than introducing autosave.

## 9. What this does NOT claim

- It does not make the gate *smart*. Every predicate is unchanged; a student can
  satisfy S1 with three thin definitions. The gate checks that she did the work,
  never how well — that judgment belongs to the assessor and to her own
  self-score, and putting a quality bar in a gate would make the machine the
  grader (RL-5).
- It does not touch S3–S6, whose gates already have producers.
- It does not add a 「进入下一环节」 button. The binding design has none: S4's banner
  reports 「本环节门禁通过。可以进成稿打磨了。」 and she clicks the next station in the
  rail herself. Advancement is a consequence of her work, not a thing she must
  ask for.

## 10. Acceptance

1. A project created through N1's funnel can be walked S0 → S1 → S2 → S3 with no
   hand-written database rows, and each station's rail entry turns `done` as its
   gate closes. This is the acceptance test that could not have passed before this
   slice at all.
2. `agent.AdvanceAll` never confirms a contract whose `requires` is unconfirmed —
   proven with a fixture whose S2 gate is satisfied while S1's is not.
3. Re-submitting S0 with one weak pick twice leaves exactly one
   `weakness_prediction` node, not two.
4. Submitting S2 does not delete perspectives minted by `perspective-matrix`, and
   those perspectives count toward the `n≥2` gate.
5. `git diff` over the branch contains no file under `internal/store/migrations/`,
   and no hand-edit under `internal/store/sqlc/` (a fresh `make sqlc` reproduces
   the one generated change byte-identically).
6. `ShellView` and its placeholder copy no longer exist in the repo.

## 11. Risks

- **The `origin` marker is load-bearing.** If a future writer of `perspective`
  nodes forgets it, nothing breaks; if a future writer *adds* it wrongly, S2's
  delete-then-insert eats those nodes. The marker is documented at both the query
  and the effect site, and the delete query names its types explicitly rather
  than taking a caller-supplied list.
- **`AdvanceAll` runs on every gate-affecting write** — one `LoadGraph` +
  `ListGateStates` per write. These are the same two reads the turn loop already
  does per turn, on projects bounded by a single student's work. Acceptable; if it
  ever is not, the fix is to scope the walk to the head contract.
- **S0's `milestone_plan` attestation is thin.** She accepts a plan she cannot yet
  edit. If a later slice lets her edit the plan, the attestation moves to that
  edit; the item name does not change.

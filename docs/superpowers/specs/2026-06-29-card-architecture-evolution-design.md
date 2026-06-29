# Card Architecture Evolution — Design Spec

> **Authored:** 2026-06-29. **Status:** design complete, pending user review.
> Covers discussion-queue topic **#2 (card architecture)** — reframed from a scalability *review* into a
> *model evolution*: cards gain an in-card AI tutor and move to runtime storage.
> **North-star:** `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`.
> **Related:** `docs/superpowers/specs/2026-06-29-card-summon-decision-layer-design.md` (`summon_when` block),
> `docs/superpowers/specs/2026-06-29-evaluation-model-design.md` (`event_trace` → eval signal).
> **Knowingly updates a hard constraint:** the AGENTS.md `go:embed` *build-time single-source* model is
> superseded by runtime DB storage (see §6). The *client-never-calls-the-model* constraint is **preserved** (§4).

## 1. Purpose & scope

The original topic was "can the schema-driven runtime scale?" The real question turned out deeper: **should a card
change shape** — (1) carry *AI inside it*, not just the chaperone outside; and (2) live as *runtime data* rather than
a build-embedded file? This spec answers those two. It defines the card's anatomy, the in-card AI surface and its
load-bearing boundary, and the storage seam.

- **In scope:** card anatomy (teaching vs entry zones), the in-card tutor agent and its method-only boundary, the
  data/behavior storage split.
- **Out of scope (deferred by decision):** the *teacher authoring workflow* (what teachers provide, the doc→card
  pipeline, review/publish); a future *student-facing platform*. We design the **runtime**, not the **authoring pipeline**.

## 2. Current state (baseline)

- **9 field primitives** (not 7): `text`, `textarea`, `single_choice`, `multi_choice`, `rating`, `link_check`,
  `spectrum`, `criteria_check`, `repeatable_group` (`packages/contracts/src/primitives.ts`). The set has already grown
  twice — adding a primitive is not rare.
- **Schema-driven renderer + a `card_id → component` escape-hatch.** Pure-schema cards render via `fieldRegistry`
  (`apps/web/src/cards/fieldRegistry.tsx`); 3 cards use bespoke JSX via `customRenderers.ts` (`belief-spectrum`,
  `sift_craap`, `emotional-alignment`). Custom renderers receive the same `CardBodyProps` and write only through
  `onField(path,value)` — **envelope output is identical** whether a card is pure-schema or custom (cosmetic divergence,
  structural uniformity).
- **Envelope is locked, boundary-validated.** Go validates only the outer shape (`field_values` object, `event_trace`
  array of known kinds — `apps/api/internal/api/cards.go`); inner truth stays in the Zod contract (`envelope.ts`).
- **Build-time dual source.** `packages/contracts/cards/*.json` (web import) + `apps/api/internal/cards/specs/*.json`
  (`go:embed`), kept in sync by `tools/synccards/main.go`. 33 cards, 0 stubs.
- **Multimodal ≈ none.** No image/audio/file primitive. `multimodal-decode` is a text card in disguise: paste a URL into
  a `link_check` box and describe the media in a `textarea`. No player, preview, or upload.
- **AI is entirely outside the card.** The chaperone summons a card; the student fills it locally; the envelope refeeds.
  The card runtime itself makes no model calls.

## 3. Card anatomy (locked)

A card splits into two zones plus the external chaperone:

```
┌─ CARD ───────────────────────────────────────────────┐
│  TEACHING ZONE   figure + text (NOT video)            │
│                  + a small AI TUTOR beside it         │  ← AI lives here
│ ─────────────────────────────────────────────────────│
│  ENTRY ZONE      the declarative form (9 primitives)  │  ← NO AI; captures the
│                  → standard envelope                  │     student's application
└───────────────────────────────────────────────────────┘
        CHAPERONE (outside, unchanged) orchestrates the task
```

- **Teaching zone** — authored **figure + text** (deliberately *not* a video), with a **co-located tutor agent** the
  student can converse with to *understand the tool*. "Like coming to an expert."
- **Entry zone** — the existing declarative form. **No in-zone AI.** This keeps the zone teacher-authorable later and
  keeps the envelope clean.
- **Chaperone** — unchanged, outside, runs the whole task.

## 4. The in-card tutor (locked intent; mechanics deferred)

The tutor is the chosen form of "AI-enhanced interactive." On the L0–L3 interaction ladder we considered (L0 static /
L1 AI authors but static runtime / L2 AI reacts inside a field / L3 card is a mini-dialogue), the choice is **L3 scoped
to the teaching zone** — a real in-card agent, but confined to teaching.

**Load-bearing boundary rule (the single most important rule in this spec):**

> **The tutor teaches the *tool/method* and must never do the student's actual *task-thinking*.**
> The `sift_craap` tutor explains *how* lateral reading works; it must **not** evaluate the student's specific source
> for them — that application is the thinking the **form** captures. Three surfaces, three responsibilities:
> **tutor teaches the tool · the form captures the application · the chaperone runs the task.** No overlap.

This scoping is what makes an in-card agent **law-safe**. A whole-card agent (L3 spanning entry too) was **ruled out**:
it would compete with the chaperone's 一次只问一个 rhythm and is engaging-by-construction (brushes 不操纵).

Design intent, locked (the *how* is carry-forward, §9):
- **One tutor runtime, grounded per-card.** Not 33 bespoke agents — a single runtime grounded by each card's
  teacher-authored teaching content + a fixed method-only guardrail. (The unlock for future teacher-authoring: teachers
  supply *content*; the platform supplies the *agent*.)
- **Pull, not push** — student-invoked, mirroring the card law 触发自动，打开由学生确认.
- **Gateway-only** — the tutor is a backend LLM call through the existing gateway; the client **never** calls the model
  directly (constraint preserved). Tier: chaperone mid-tier (assistive, downgradable) — *not* the eval flagship.
- **过程即数据** — tutor dialogue lands in `event_trace` as a new event kind; "asked the tutor 3 questions about lateral
  reading" is itself an eval signal (engagement with learning the method).

## 5. Interaction ceiling — decisions

- **Chosen:** in-card AI = the teaching-zone tutor (L3-scoped). Entry zone stays declarative (no AI).
- **Ruled out:** L3 spanning the whole card (entry + teaching) — fights 一次只问一个 and 不操纵.
- **Generalizes the escape-hatch, doesn't fight it:** "this card needs something special" is increasingly answered by
  *declared* config (a tutor, a field primitive) rather than bespoke JSX. JSX remains available but dev-only.

## 6. Storage — data / behavior split (locked)

Teacher-authored-at-runtime cards cannot be build-embedded, so storage moves. The seam:

- **Database = what a card *is*** (data/config): form-field spec, teaching content (text + figure refs), tutor grounding
  text, `summon_when`, rubric tags.
- **Files = how a card *behaves*** (interaction design as code): field primitives, custom renderers (escape-hatch), the
  tutor runtime. Version-controlled, dev-owned.
- **A live card = the join** of a DB record and the interaction code it references — by field type, or a `renderer_key`
  for a custom card. This **generalizes the escape-hatch**: pure-schema card = record whose field types map to primitive
  components; custom card = record + `renderer_key`; tutor = one runtime grounded by the record's teaching content.

Consequences:
- **DB is the single runtime source for *all* cards** (platform + future teacher cards) — no fork. The 33 repo JSON files
  change job from `go:embed` payload to **seed fixtures** loaded at provisioning; they stay version-controlled.
- **Validation shifts build-time → author-time.** The Zod contract validates a card *when written*, before it persists.
  The renderer trusts validated specs; Go keeps boundary-validation.
- **Figures force object storage.** Teaching-zone figures are assets → the deferred **object-storage/OSS** item is now on
  the table (figure refs in the record, bytes in object storage).
- **Small migration.** The *data* leaves `go:embed` for the DB; the *code* (`fieldRegistry`, `customRenderers`) is already
  in files and does not move. `synccards` and the `go:embed` dual-source retire once seed-loading replaces them.

## 7. Constraint updates

- **Superseded:** AGENTS.md "卡 JSON 为单一共享真相源：前端构建期 import，Go `go:embed` 同一批文件" — replaced by DB
  runtime storage with repo JSON as seed (§6). The *single-source* spirit is **kept** (one DB record per card; no fork).
- **Preserved:** client never calls the model directly — the in-card tutor routes through the backend gateway (§4).
- **Extended:** the standard envelope gains a tutor-dialogue `event_trace` kind (boundary-validated like the others); the
  inner contract stays in `packages/contracts`.

## 8. Non-goals

- ❌ Teacher authoring workflow (content intake, doc→card compilation, review/publish) — deferred.
- ❌ Future student-facing platform — deferred.
- ❌ AI in the entry zone / whole-card agent (L3-spanning) — ruled out (§5).
- ❌ Client-direct model calls — never (§7).
- ❌ Video teaching content — explicitly figure+text+tutor instead (§3).

## 9. Carry-forward / open items

- **Tutor runtime mechanics** — the concrete *how*: grounding mechanism (teaching content → system prompt / retrieval),
  the method-only guardrail prompt, turn/cost bounds, the `event_trace` kind, gateway call-type, tier resolution. This is
  the next design pass before any build.
- **Concrete card data model** — exact DB record fields (form spec / teaching content / figure refs / tutor grounding /
  `summon_when` / rubric tags), author-time Zod validation, the seed-migration from the 33 JSON files.
- **Object storage / OSS** — figure assets; shared with the broader post-P4 OSS item.
- **Escape-hatch governance at scale** — when a card earns a `renderer_key` (custom JSX) vs a new declared primitive vs
  pure schema — a written rule once the catalog grows past 33.
- **Rubric tag reconciliation** — card `rubric_tags` vs the eval spec's 10-dim set (shared carry-forward with the eval
  and summon specs).

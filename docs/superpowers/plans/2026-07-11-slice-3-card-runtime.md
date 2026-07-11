# Slice 3 — Card Runtime (CRAAP over `annotate`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the C2 card runtime — `surface_card`, completion over primitive state, `graph_effects`, consolidation, `observe`→coach, three-key disposition — with **CRAAP as pure config over `annotate`**, over fixtures, no UI. Acceptance: a card is config; the runtime touches zero interface code and a second card needs zero new runtime code.

**Architecture:** Extend the Go `cards` loader with typed C2 fields (mirror Slice 0's TS `CardSpec`); author CRAAP's C2 fields in `packages/contracts/cards/craap.json` (synced to `apps/api/internal/cards/specs`). New runtime in `apps/api/internal/agent` reusing the Slice-2 loop/classifier/coach + Slice-0 enforcement/graph/`card_instance`/`disposition`. Card state = `card_instance.anchors` (`agent.Anchor`: Dimension/Answer/Author) + `framework_fill`. Design: `docs/superpowers/specs/2026-07-11-slice-3-card-runtime-design.md`.

**Tech Stack:** Go (pgx/sqlc) + testcontainers; card JSON config.

## Global Constraints

- **Extend, don't fork:** add C2 fields to the Go `cards.Spec` (one card type). **Do NOT hand-edit `apps/api/internal/cards/specs/*`** — edit `packages/contracts/cards/craap.json` then `make sync-cards`. Reuse `agent.Anchor`, the Slice-2 loop/classifier/coach, Slice-0 enforcement/graph queries/`card_instance`/`disposition` tables.
- **Do NOT touch** the legacy form-card path (web `CardRenderer`/`cardSheetHost`), `RunTurn`, or `prompt.go`. No new migration (Slice-0 tables suffice) — only new sqlc queries.
- **Completion/graph-effects/observe kinds are a small CLOSED typed set** — a new *card* composes them as config; a new *kind* is a deliberate runtime change.
- Machine never marks a card `solid` (DEC-3); consolidation reveals the framework only **after** completion (R-9); dispositions require a **≥15-char** reason. No secrets.
- **Commands** (from `apps/api`): `make sync-cards` after editing card JSON; `make sqlc` after query edits; `go build ./...`; `go vet ./...`; `gofmt -w`; `go test ./... -short`; integration tests without `-short` (Docker up). Also `pnpm --filter @mind-imprint/contracts test` if `craap.json` changes (registry test).
- **TDD:** failing test → confirm fail → implement → confirm pass → commit per task.

---

## Task 1: Extend the Go card spec with C2 fields + author CRAAP config

**Files:**
- Modify: `apps/api/internal/cards/loader.go` (add typed C2 fields)
- Modify: `packages/contracts/cards/craap.json` (add the C2 binding; keep `steps`); then `make sync-cards`
- Test: `apps/api/internal/cards/c2_test.go`

**Interfaces:**
- Produces on `cards.Spec`: `Primitive`, `TargetType string`; `Params struct{ Tags []string; TagPrompts map[string]string }`; `Completion []CompletionPredicate{ Kind string; Tags []string; Field, Author string }`; `GraphEffects []GraphEffect{ Kind, From, To, With string }`; `Observe []ObserveRule{ When, Verb, Level string }`; `Consolidation, IntrusivenessCap string`.

- [ ] **Step 1: Failing test** — `cards.ByID("craap")` returns a spec with `Primitive=="annotate"`, `Params.Tags` = the five CRAAP dims, `TagPrompts["authority"]` non-empty, ≥2 `Completion` predicates (one `every_tag_present`, one `field_written_by` student `risk_note`), one `GraphEffect` (`promote` material→evidence), ≥1 `Observe` rule, `Consolidation` non-empty.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — add the structs/fields to `loader.go` (all additive, JSON-tagged). Add the C2 block to `packages/contracts/cards/craap.json` (primitive/target_type/params/completion/graph_effects/observe/consolidation/intrusiveness_cap per design §2), keeping `steps`. Run `make sync-cards`. Confirm `pnpm --filter @mind-imprint/contracts test` stays green (the TS `CardSpec` already accepts these optional fields from Slice 0).
- [ ] **Step 4: Test → PASS** (`go test ./internal/cards/...`), `go vet`, `gofmt`.
- [ ] **Step 5: Commit** — `feat(api): C2 card fields on the Go loader + CRAAP annotate config`.

---

## Task 2: Completion predicates + observe evaluation (pure)

**Files:**
- Create: `apps/api/internal/agent/card_completion.go`
- Test: `apps/api/internal/agent/card_completion_test.go`

**Interfaces:**
- Consumes: `cards.Spec` (Task 1), `agent.Anchor`.
- Produces: `EvaluateCompletion(spec cards.Spec, anchors []Anchor) (complete bool, missing []string)` and `ObserveCandidates(spec cards.Spec, cardInstanceID string, anchors []Anchor) []Candidate`.

- [ ] **Step 1: Failing test** — `EvaluateCompletion`: anchors with a non-empty student `Answer` for each required tag (Dimension) + a student `risk_note` anchor ⇒ `(true, nil)`; missing a required tag ⇒ `(false, ["authority"])`; a required tag present but `Author != "student"` on `risk_note` ⇒ incomplete. `ObserveCandidates`: an `authority` anchor with `len(Answer) < 15` ⇒ one `Candidate{Verb:"post_intervention", AnchorID: cardInstanceID or node, Level:"I2"}`; a strong answer ⇒ none.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — `EvaluateCompletion` walks `spec.Completion`: `every_tag_present` ⇒ each tag has an anchor with `Dimension==tag` and non-empty `Answer`; `field_written_by` ⇒ an anchor matching `Field` (by Dimension/Question) with `Author==Author` and non-empty `Answer`; collect `missing`. `ObserveCandidates` evaluates each `spec.Observe` rule's `When` (support the CRAAP `tag=X AND note_len<N` form via a tiny matcher) over the anchors → `Candidate`. Pure, no DB.
- [ ] **Step 4: Test → PASS**, `go vet`, `gofmt`.
- [ ] **Step 5: Commit** — `feat(api): card completion predicates + observe-rule candidates`.

---

## Task 3: graph_effects + consolidation (pure)

**Files:**
- Create: `apps/api/internal/agent/card_effects.go`
- Test: `apps/api/internal/agent/card_effects_test.go`

**Interfaces:**
- Produces: `MintNode{Type, Author string; Body map[string]any}`, `MintEdge{Type, FromKind, FromID, ToKind, ToID string}`, `GraphEffects(spec cards.Spec, materialID string, anchors []Anchor) ([]MintNode, []MintEdge)`, and `ConsolidationPayload(spec cards.Spec) map[string]any`.

- [ ] **Step 1: Failing test** — CRAAP `GraphEffects` over a completed anchor set + `materialID` ⇒ exactly one `MintNode{Type:"evidence", Author:"student"}` whose `Body` carries `source_quality` (derived from the CRAAP dimension answers/ratings) + one `MintEdge{Type:"evaluated-as", FromKind:"material", FromID: materialID, ToKind:"graph_node"...}` (or a deterministic placeholder id the caller resolves). `ConsolidationPayload` ⇒ a non-empty framework map (the CRAAP methodology). Effects for a non-`promote` spec ⇒ empty.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — `GraphEffects` interprets each `promote` effect: build one evidence `MintNode` (Body includes `source_quality` summarizing the CRAAP verdict/ratings from the anchors) + the `evaluated-as` `MintEdge` from the material. Keep node id resolution to the caller (the loop assigns real ids on insert; the edge references them). `ConsolidationPayload` returns the framework (methodology) as a map. Pure.
- [ ] **Step 4: Test → PASS**, `go vet`, `gofmt`.
- [ ] **Step 5: Commit** — `feat(api): card graph_effects (mint evidence) + consolidation payload`.

---

## Task 4: sqlc — card_instance (project-scoped) + disposition

**Files:**
- Create: `apps/api/internal/store/queries/card_instance.sql`, `apps/api/internal/store/queries/disposition.sql`
- Regenerate: `apps/api/internal/store/sqlc/*`
- Test: `apps/api/internal/store/refactor2_cards_sqlc_test.go`

**Interfaces:**
- Produces: `CreateProjectCardInstance` (project_id, card_id, contract_ref, status → id), `SetCardInstanceAnchors`, `SetCardInstanceFramework`, `SetCardInstanceStatus`, `GetCardInstance`, `ListCardInstancesByProject`; `InsertDisposition` (intervention_id, action, reason → id).

- [ ] **Step 1: Failing test** (`-short`-guarded, `TestRefactor2Cards...`): seed a project; `CreateProjectCardInstance` (status `proposed`) → set anchors → set status `active` → set framework → `GetCardInstance` reflects each; `InsertDisposition` on an intervention round-trips.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** the annotated queries; `make sqlc`; commit generated files. (These are the project-scoped card_instance ops; the old task-scoped `CreateProposedCard` stays legacy.)
- [ ] **Step 4: Test → PASS** (`go test ./internal/store/ -run Refactor2Cards`).
- [ ] **Step 5: Commit** — `feat(api): sqlc for project card_instance lifecycle + disposition`.

---

## Task 5: Loop wiring — surface_card, observe, disposition, effects

**Files:**
- Modify: `apps/api/internal/agent/classifier.go` (add the surface-card predicate), `apps/api/internal/agent/loop.go` (dispatch surface_card + observe + effects), `apps/api/internal/agent/agentstore.go` (new store ops)
- Create: `apps/api/internal/agent/card_lifecycle.go` (`SurfaceCard`, `CompleteCard`, `RecordDisposition`)
- Test: `apps/api/internal/agent/card_lifecycle_test.go` (unit, fake store) + a testcontainers pass in `apps/api/internal/agent/` and the **§5.7 second-card** test.

**Interfaces:**
- Consumes: everything above. Extends `AgentStore` with card_instance + disposition + graph-mint ops; `RunAgentStep` now also handles a `surface_card` candidate; adds `RecordDisposition(ctx, deps, interventionID, action, reason) error` and `CompleteCard(ctx, deps, cardInstanceID) error`.

- [ ] **Step 1: Failing test** (unit, fake store + stub provider):
  - Classifier: a source material with no evaluation card_instance + no evidence node ⇒ a `surface_card(craap)` candidate; an already-evaluated source ⇒ none.
  - `RunAgentStep` on a surface_card candidate ⇒ creates a `proposed` card_instance on the material (no model call needed for surfacing) and returns an Action; persists nothing else.
  - `CompleteCard` on a fixture card_instance whose anchors satisfy completion ⇒ applies `GraphEffects` (one evidence node + edge inserted) and sets `framework_fill`; status is not auto-`solid`.
  - `RecordDisposition`: a ≥15-char reason inserts a row; a short reason returns an error and inserts nothing.
  - **§5.7:** author a trivial second card spec inline (a `note` card over `annotate` with a single `every_tag_present` completion), surface + complete it through the SAME functions with **zero new runtime code**.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — the surface-card classifier predicate; `RunAgentStep` decides among candidates (surface_card vs post_intervention — surfacing an unevaluated source outranks nagging; document the ordering); `SurfaceCard` creates the card_instance; `ObserveCandidates` feed the coach for `active` cards; `CompleteCard` runs `EvaluateCompletion` → on complete applies `GraphEffects` (insert nodes/edges, resolving ids) + `ConsolidationPayload` → `framework_fill`; `RecordDisposition` enforces ≥15 chars then inserts. Extend the sqlc `AgentStore` adapter with the new ops. On any enforcement error, persist nothing (as Slice 2).
- [ ] **Step 4: Test → PASS** (`go test ./internal/agent/ -run 'Card|Loop|SecondCard'`; the testcontainers pass without `-short`). Run `go test ./... -short`.
- [ ] **Step 5: Commit** — `feat(api): wire surface_card + observe + completion/effects + disposition into the loop`.

---

## Task 6: Verification + roadmap log

**Files:**
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md` (Slice 3 ☑ + per-slice log; note the legacy form-card retirement rides with Slice 5).

- [ ] **Step 1:** From `apps/api`: `go build ./...`; `go vet ./...`; `go test ./... -short`; the testcontainers passes (`-run Refactor2Cards`, `-run Card|SecondCard`); confirm the legacy form-card path + `RunTurn` untouched (`git diff --name-only main...HEAD` shows no web/form/turn.go changes). Confirm `pnpm --filter @mind-imprint/contracts test` green.
- [ ] **Step 2:** Confirm acceptance (design §8): CRAAP runs as config end-to-end (surface→complete→evidence node→framework→disposition); the second card needs zero new runtime code; machine never `solid`; consolidation after completion; ≥15-char disposition.
- [ ] **Step 3:** Update the roadmap: Slice 3 ☑, link spec + plan, commit range.
- [ ] **Step 4: Commit** — `docs(refactor2): Slice 3 card runtime complete — log + status`.

---

## Self-review notes

- **Spec coverage:** C2 card in Go + CRAAP config (T1) · completion + observe (T2) · graph_effects + consolidation (T3) · card_instance/disposition sqlc (T4) · surface_card/observe/completion/disposition wiring + §5.7 (T5) · verification (T6). All design §2–§6 map to a task.
- **Deferred by design:** UI/transport for the student's answers (Slice 5) · SIFT/lateral (Slice 6) · Toulmin/`graph` (Slice 7) · planner (Slice 4) · gate-state entry-stage derivation (Slice 4).
- **No-orphan check:** one Go card type (extended), one card JSON (extended), reuse of Slice-2 loop + Slice-0 tables; legacy form path + `RunTurn` untouched, retired in Slice 5.
- **Type consistency:** `cards.Spec` C2 fields defined once (T1) and consumed by T2/T3/T5; `agent.Anchor` is the completion/observe input; `Candidate`/`AgentStore` extended, not duplicated.

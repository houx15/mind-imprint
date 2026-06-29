# Card-Summon Decision Layer & Agent Responsibilities — Design Spec

> **Authored:** 2026-06-29. **Status:** design complete, pending user review.
> Covers discussion-queue topics **#1 (how AI summons cards)** + **#3 (what the agent does each turn)**.
> **North-star:** `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`.
> **Shares infrastructure with:** `docs/superpowers/specs/2026-06-29-evaluation-model-design.md` (the rule-signal layer).
> **Hard constraint honored:** the trigger truth lives in the card spec; the decision layer derives from the registry
> (no hand-written second source — avoids `trigger_condition` drift).

## 1. Goal & scope

Make the agent's card-summoning **great**: the *right* card at the *right* moment, *dependably*, and *scaling* as the
library grows (33 → 100+ cards). Three failure modes to fix:
- **Recall** — don't miss a fitting moment (today `tool_choice="auto"` + a "怕越界" instinct under-fires).
- **Precision** — pick the *best* card when several fit (33 cards in-prompt → dilution).
- **Scale** — work at 100+ cards without bloating every prompt or diluting the decision.

In scope: the **summon decision layer** + the **agent's per-turn responsibilities**. Out of scope: rebuilding the
chaperone into a multi-step planner (not prioritized); changing the card *runtime*/renderer.

## 2. Current state (baseline)

- **Reactive, single-shot turn loop** (`agent/turn.go:RunTurn`): student message → one LLM stream → propose one card
  (first `summon_card` call) or text reply. No multi-step loop.
- **Summoning is fully LLM-in-prompt:** the *entire* catalog (every card's 何时用/能帮他) is pasted into the system
  prompt each turn (`prompt.go:BuildCatalogText`); `tool_choice="auto"` (`gateway/deepseek.go:84`). Probabilistic.
- **Restraint ladder** is a prompt menu (`prompt.go:18–48`); "一次只问一个" is prompt-only.
- **Process-blind:** the agent doesn't know which cards were used, where the student is, or any eval signal.
- Cards are `//go:embed`-ed JSON specs (`cards/loader.go`); `tier` chaperone = `deepseek-chat` (downgradable),
  eval = `deepseek-reasoner` (never downgrade) (`gateway/keyresolver.go`).

## 3. Design — deterministic retrieve → decide, with full stage-2 autonomy

```
[student turn]
  ① RULE LAYER (shared with eval) ─► process-state signals
  │     source_introduced? source_unvetted? position_stated? counter_unhandled?
  │     argument_produced? verbatim_copy? key_node_reached? cards_used[] ...
  ② STAGE-1 GATE (deterministic): card.summon_when matches the signals?  ─► candidate set
  ③ POLICY (deterministic): drop used/skipped cards, cooldown, min_turn, one-per-turn ─► ≤ k candidates (k≈3)
  ④ STAGE-2 CHAPERONE (the existing LLM call): sees ONLY the k candidates' NL 何时用;
        tool_choice="auto" over the narrowed enum → summons the best, or coaches in text.
        (empty candidate set → just coach; a card is NEVER forced.)
```

- **No extra LLM call / no added latency:** stages ① ② ③ are deterministic Go; stage ④ is today's single chaperone call.
- **Why full autonomy at ④ still fixes recall:** under-firing was a *dilution* problem. Stage-2 now sees ~3 sharply
  gated candidates instead of 33, so its autonomous fire-decision is far sharper. The existing "贴合就递，别犹豫"
  instruction rides along, and the gate preserves the LLM's genuine "is this the right moment" judgment (铁律 #1 克制).

## 4. `summon_when` — the card-side gating block (single source of truth)

Each card JSON gains a machine-evaluable block (schema-driven config; renderer untouched; "新增卡 = 新增 JSON"):

```json
// sift_craap.json
"summon_when": { "signals": ["source_introduced", "source_unvetted"], "rubric_dims": ["D2","D3"], "min_turn": 1 }
// concession.json
"summon_when": { "signals": ["position_stated", "counter_unhandled"], "rubric_dims": ["D4"] }
```

- **`signals`** — a list of named booleans from the **closed signal vocabulary** (§5). **AND semantics** (all must
  hold). Kept a closed AND-vocabulary, NOT a mini rule-engine (maintainability). A genuinely multi-situation card may
  later carry a **list** of `summon_when` blocks (OR across blocks) — deferred until a real card needs it.
- **`rubric_dims`** — links the card to the evaluation dimensions it targets. Used as the eval linkage and as the hook
  for future **weak-dimension-driven summoning** (the Agent↔eval loop) — wiring present, behavior deferred.
- **`min_turn`** (optional) — earliest turn the card may be gated in.

## 5. Signal vocabulary (single source + drift guard)

- The **closed, documented vocabulary** of named process-state signals is defined **once by the rule layer** (the same
  layer that feeds evaluation — one signal infrastructure serves both summon-gating and eval anchoring).
- Signals are deterministic booleans derived from messages + `card_instances` (e.g. `source_introduced`,
  `source_unvetted`, `position_stated`, `counter_unhandled`, `argument_produced`, `verbatim_copy_detected`,
  `key_node_reached`). Exact enumeration is finalized in the implementation plan.
- **Build-time validation:** every card's `summon_when.signals` must reference only known vocabulary signals — a
  compile/build check fails otherwise. This is what kills `trigger_condition` drift at the source.

## 6. Process-state policy (deterministic, stage ③)

Now possible because the agent finally knows process state (from `card_instances` + history):
- **Dedup:** never re-gate a card already `completed` or `skipped` for this task.
- **Cooldown:** minimum turns between two summons.
- **`min_turn` floor:** respect each card's earliest-turn.
- **One card per turn:** unchanged (stage ④ captures the first `summon_card`).
- All policy is deterministic Go applied to the candidate set before stage ④.

## 7. Agent per-turn responsibilities (#3)

The turn loop stays the **reactive, single-shot, one-question-at-a-time chaperone** — no rebuild into a planner. The
change is that it is **no longer process-blind**: it knows which cards were used and where the student is, and (via
`rubric_dims`) carries the wiring for future weak-dimension-driven summoning. The restraint ladder, refeed of resolved
cards, and model tiering are unchanged.

## 8. Migration

- Cards **without** a `summon_when` block are treated as **always-candidate** (today's behavior) until authored — so
  the 33-card backfill is incremental and nothing breaks mid-migration.
- Backfill `summon_when` card-by-card; each card moves from always-candidate to gated when its block lands.

## 9. Non-goals

- ❌ No extra per-turn LLM decision call (latency); stage-1 is deterministic.
- ❌ No multi-step planner / agentic loop — the chaperone stays single-shot reactive.
- ❌ No hand-written signal→card mapping table (second source of truth) — gating lives in the card spec.
- ❌ No forced/deterministic summon — stage-2 keeps autonomy; a card is never forced (铁律 #1, #2).
- ❌ No renderer/runtime change — `summon_when` is config the decision layer reads.

## 10. Carry-forward / open items

- **Signal vocabulary enumeration** — finalize the exact closed set + each signal's deterministic definition in the plan.
- **`summon_when` authoring** — backfill all 33 existing cards (incremental, per §8).
- **`k` tuning** — candidate cap (start k≈3); revisit with telemetry.
- **Under-fire telemetry** — log "gated-candidate-existed-but-no-summon" to tune the stage-2 prompt with evidence.
- **Weak-dimension-driven summoning** — the Agent↔eval loop via `rubric_dims` (deferred; wiring present).
- **Card↔dimension tag reconciliation** — shared with the eval spec's carry-forward (`rubric_tags`/`rubric_dims` drift).

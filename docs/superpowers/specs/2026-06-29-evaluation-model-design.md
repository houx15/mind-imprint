# Evaluation Model — Design Spec

> **Authored:** 2026-06-29. **Status:** design complete, pending user review → feeds P4 (async eval) implementation.
> **Working record:** `docs/2026-06-28-evaluation-model-design-notes.md` (full decision log).
> **Detailed instrument:** `docs/2026-06-28-cognitive-model-rubric.md` (v2 — the binding L1–L4 anchors).
> **North-star:** `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`. Supersedes the
> "暂定" evaluation data model in `docs/architecture/database-schema.md` and the 仅你可见 narrative rule in the PRD (§7c).

## 1. Purpose & scope

Mind Imprint's differentiator is **formative evaluation of *good AI use*** — guarding against cognitive offloading
(delegating thinking to AI → lower cognitive ability). This spec defines the cognitive model we measure, how we
evaluate it from process data, how we store it, when it runs, and how we present it. Two evidence sources are
**combined** (not chosen between): the student's **chat messages** and their **card actions** (standard envelopes).
Two methods are **combined**: deterministic **rules** (cheap, hard facts) and a flagship **eval LLM** (judgment).

## 2. Cognitive model (locked)

**3-level taxonomy** — the 2 faces and 4 categories are the *same tree* at different depths (a clean partition; no
dimension overlaps). Faces/categories are **pure rollup metadata** (no scoring, no DB columns); only the **10
dimensions** are scored.

```
🚀 生成式驾驭 Driving AI & your own thinking
   ├─ 意图与编排 Intent & Orchestration   → D1 提问清晰度, D10 协作编排
   └─ 推理与论证 Reasoning & Argument      → D4 多视角与让步, D5 论证拆解, D7 论证质量
🛡️ 批判式防护 Not being captured
   ├─ 信息素养   Information Literacy       → D2 信源辨识, D3 横向验证
   └─ AI 元认知与边界 AI Metacognition & Boundaries → D6 反思与元认知, D8 信息再生产, D9 AI 边界与伦理
```

- **Audience mapping:** 2 faces = student-facing narrative/identity; 4 categories = teacher-facing diagnostics;
  10 dimensions = scored leaves (SOLO L1–L4).
- **Scoring model:** each dimension resolves to **N/A** or **L1–L4** (萌芽/发展中/熟练/卓越). **N/A = insufficient
  evidence** (never infer L1 from silence); **L1 requires a positive tell**. N/A is a first-class outcome.
- **Observability invariant:** only behaviorally observable signals are scored (text + card actions); internal
  stances are scored via observable traces.
- **Scope:** per-task evaluation is the immutable **source of truth**; the longitudinal student profile is a
  **projection** over a student's task-evals (no separate profile state).
- The full, binding L1–L4 anchors (observable, signal-tagged) + the single-owner **tie-break rules** live in
  `2026-06-28-cognitive-model-rubric.md` (v2). Key dimension changes from the production 9D baseline: **+D10
  协作编排**; **D8/D9 re-split** (D8 = provenance/attribution, D9 = epistemic verification); **D5 re-scoped**
  (dissect a given/AI argument incl. hidden premises/fallacies) vs **D7** (own argument quality); "reflective-vs-
  default adoption" folded into **D6**; re-steering folded into **D10**.

## 3. Verification (V1–V6) — passed

- **V1–V5** (distinctness, coverage, monotonicity, observability, boundary): 3 independent adversarial audits → rubric
  rewritten to v2 (single-owner de-confliction, observable proxies, N/A floor).
- **V6 inter-rater reliability (empirical):** v2 scored on 3 controlled transcripts (strong/weak/narrow) × 2 independent
  raters → **28/30 exact (93%), 30/30 within-1-level; N/A 11/11 perfect**; discrimination monotonic on both faces;
  tie-break rules held (trust-AI→D9 not D2; copy→D8 only). The model is validated, not just designed.
- **Ongoing:** re-run V6 whenever the rubric version changes; keep the fixture transcripts as a regression set.

## 4. Evaluation pipeline — hybrid anchor (locked)

Per-task flagship eval is **affordable** (per-task, not per-turn). So rules are for **reliability + speed**, not cost-
substitution. The **flagship LLM is the sole scorer**; deterministic rules feed it ground-truth facts.

```
[task transcript: messages + card envelopes]
   ├─► RULE PASS (deterministic, no LLM) ─► 客观信号 block + N/A candidates
   └─► EVAL LLM (flagship · full rubric · full transcript · + 客观信号) ─► {per-dim score|N/A, note} + narrative
```

- **Rule pass = HARD facts only** (soft/semantic judgment stays with the LLM): per-card `{status, op_count,
  filled/empty fields}`; count of student-introduced sources (vs AI-introduced); max verbatim student↔AI overlap
  (copy detection); student own-material contribution; message/turn counts; derived **N/A-candidates**.
- **Injection contract (anti-hallucination):** the 客观信号 block is *facts the LLM must respect but may interpret* —
  it may override an N/A-candidate if the dialogue shows the behavior, but may **not** assert a fact contradicting the
  signals (e.g. credit sources beyond the counted number). This is the core LLM-as-judge reliability guard.
- **Models:** chaperone mid-tier may downgrade; **eval flagship never downgrades** (platform rule). Today
  `deepseek-reasoner`, full transcript, MaxTokens ≥ 8000.

## 5. Data model (locked)

Extend the existing `evaluations` table — **no new tables**:
- `scores jsonb` = `[{dim_id, level, note}]`, where **`level ∈ {L1,L2,L3,L4,NA}`** (NA added).
- `narrative text` — diagnosis + one next-step.
- **`signals jsonb`** (new) — the rule-computed hard facts that anchored this eval (audit trail + cheap teacher reuse).
- **`rubric_version text`** (new) — which rubric produced the scores (safe longitudinal comparison as the rubric evolves).
- Unchanged: `model, tier, prompt_tokens, completion_tokens, cost_estimate, status (queued|running|done|failed),
  error, created_at, completed_at`.
- **No** faces/categories columns (static rubric metadata; rollup computed at display). **No** profile table (the
  profile is a read-time projection over a student's task-evals). Each run = a new **immutable snapshot**; latest shown.

## 6. Trigger + async path (locked, revisit with cost data)

**Two cadences, two cost tiers — decouple *compute* from *show*:**
- **Cheap rule signals → every turn** (~free, silent): live process tree + teacher freshness + LLM anchor.
- **Flagship LLM eval → async + infrequent**, shown to the student **on-demand**.

**Trigger mirrors the card restraint law** (「触发自动，打开由学生确认」):
- **Auto-compute** (async river job, non-interrupting) on a **milestone = first completed card OR N substantive turns**
  (whichever first), **debounced + capped** (a few flagship calls/task; never per-turn).
- **Offer, don't push, the reveal** — a quiet "你的思维印记有新内容" indicator the student opens by choice. **No live
  ticking score, no badges/streaks/numbers** (铁律 #2 不操纵).
- Plus always-on **on-demand pull** + re-eval.
- **Rationale:** we don't own task completion and many tasks stay unfinished, so we evaluate *accumulated substance*
  at milestones; the **N/A verdict** makes partial tasks read *in progress*, not broken/punished. The first milestone
  is the proactive "wow" that demonstrates value before the student thinks to ask.
- **Async mechanics (P4/river):** `POST /evaluate` enqueues a job → `status=queued` → worker runs rule pass → LLM →
  `status=done` → frontend polls `GET /evaluation`.

## 7. Presentation + privacy (locked)

- **Student — 你的思维印记:** headline = 2 faces; drill = 4 categories → 10 dims (L1–L4); **N/A renders neutrally**
  ("本次未涉及", greyed — never a low/zero bar); + narrative. **Growth/trend view** (longitudinal projection) = fast-
  follow after the per-task snapshot.
- **Teacher:** per-student **levels** (category rollup, drillable to 10 dims) + the **diagnostic narrative** + class
  aggregates + trends. **Teacher does NOT see the raw AI transcript.**
- **Privacy boundary:** teacher sees the *assessment* (levels + narrative); the *raw process* (the student's AI
  dialogue) stays student-private. This deliberately updates the PRD's 仅你可见 (narrative now student + teacher;
  transcript student-only).

## 8. Non-goals

- ❌ No per-turn LLM scoring (too slow/expensive); no live-updating score dashboard (铁律 #2).
- ❌ No separate longitudinal-profile table (projection only); no faces/categories DB columns (rollup metadata).
- ❌ No teacher access to raw dialogue; no gamification (streaks/badges/leaderboards).
- ❌ Rule pass does not attempt semantic judgments (left to the LLM).

## 9. Carry-forward / open items

- **Card↔dimension tag drift:** reconcile each card's `rubric_tags` against this 10-dim set at build (current card
  tags may predate it, e.g. "D1_来源意识" vs rubric D2). The `[CARD]` signal tags in the rubric are opportunistic.
- **Cost calibration:** revisit the milestone definition, debounce window, and cap once real cost/latency data exists.
- **Growth/trend projection math:** how per-task evals roll up into a trend (and how N/A and rubric_version changes
  are handled in the rollup) — design in the presentation fast-follow.
- **Rule-signal persistence reuse:** define exactly which `signals` fields the teacher dashboard surfaces.

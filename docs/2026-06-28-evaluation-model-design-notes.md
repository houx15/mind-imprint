# Evaluation Model — Design Notes (Living Record)

> **Started:** 2026-06-28. **Status:** 🟡 brainstorming in progress (topic #5 in `2026-06-28-discussion-queue.md`).
> **Current state (2026-06-29):** cognitive model LOCKED & empirically validated (10 dims / 3-level taxonomy / hybrid
> scope); rubric detailed → `2026-06-28-cognitive-model-rubric.md` v2 (passed V1–V5 adversarial + V6 inter-rater);
> evaluation pipeline = hybrid anchor (rules pre-compute hard facts → anchor flagship LLM). **NEXT: step 5 — realize
> it** (evaluation data model, trigger + async P4/river, student/teacher presentation of the 3-level rollup).
> **Purpose of this doc:** a *progressively maintained* record of every locked decision and open question for
> the evaluation / process-assessment model, so detail isn't lost across a long discussion. Updated as we go.
> The final design spec (→ `docs/superpowers/specs/`) is written at the end of brainstorming from this record.

---

## 0. Framing & purpose (locked)

- We build **formative / process evaluation**, not summative grading. We are uniquely positioned to **evaluate and
  guide students' *good use of AI***. Motivation: recent research that people who offload thinking to AI may suffer
  lower cognitive ability and academic performance — so "are you thinking *with* AI or delegating *to* it" is the core.
- **Two evidence sources** (to be *combined*, not chosen between): (1) the student's **text messages** to the AI;
  (2) the student's **answers/actions on the cards** (the standard envelope: `field_values` + `event_trace`).
- **Two evaluation methods** (to be *combined*): (a) cheap **rules** (length, question use, etc.); (b) expensive
  **eval LLM** scoring the dialogue. Balance is a later step.

## 1. Observability invariant (locked)

> **We only score what is behaviorally observable in the process** (text messages + card actions). Internal stances
> (e.g. "ownership/accountability") are **not scored directly** — we score their *observable traces*. Example: the
> trace of ownership is **proposing critiques or refinements of the AI's output** instead of accepting it passively.

## 2. Cognitive model — 3-level taxonomy (locked)

Dimensions are **largely independent** (no D-X-requires-D-Y dependency) → the set is **unsequenced**. The only
structure is **taxonomic grouping** (for coherence + display), *not* a sequence or dependency tree. Crucially, the
2 faces and the 4 categories are the **same tree at different depths** — a clean partition, no dimension overlaps.

```
🚀 生成式驾驭  Driving AI & your own thinking   (bring your substance, drive the work, produce your own thinking)
   ├─ 意图与编排  Intent & Orchestration   → D1, D10
   └─ 推理与论证  Reasoning & Argument      → D4, D5, D7

🛡️ 批判式防护  Not being captured            (don't get captured, verify, stay aware)
   ├─ 信息素养    Information Literacy       → D2, D3
   └─ AI 元认知与边界  AI Metacognition & Boundaries → D6, D8, D9
```

**Each level serves a different audience:**
| Level | Layer | Audience | Role |
|---|---|---|---|
| **2 faces** | narrative / identity | student-facing | headline of 你的思维印记 ("are you driving, or being captured?") |
| **4 categories** | diagnostic | teacher-facing | resolution ("weak on Information Literacy specifically") |
| **10 dimensions** | scored leaves | the model | the only things actually scored (SOLO L1–L4) |

> Faces & categories are **pure rollup metadata** — they cost nothing. We only ever *score* the 10 dimensions;
> the data model just tags each dimension with `category` + `face`. UI collapses/expands per audience.

## 3. The 10 dimensions — current baseline anchors (SOLO L1–L4)

SOLO labels: **L1 萌芽 · L2 发展中 · L3 熟练 · L4 卓越**.
> D1–D9 anchors below are the *current* production baseline (`packages/contracts/src/rubric.ts:12–31`). They will be
> **refined for observability (V4)** in the detailed-rubric step. D10 is new; D9 gets sharpened (see §5).

### 🚀 生成式驾驭 — 意图与编排
- **D1 提问清晰度** (ATL 思维 · QUEST-Q · input)
  - L1 直接抛一句话问题，不给 AI 任何背景或目标
  - L2 给一点背景，但目标/约束模糊，常需 AI 反问澄清
  - L3 主动提供任务背景、目标与约束，问题具体可执行
  - L4 结构化拆解需求，分步追问并根据回答迭代提问
- **D10 协作编排 (Orchestration & Contribution)** — *NEW*. Generative counterpart to the defensive AI strand.
  - L1 萌芽 把 AI 当答案机器——直接要答案，不带入自己的东西，被动接收
  - L2 发展中 被提示时才提供一些背景/材料，但不主导协作怎么进行
  - L3 熟练 主动带入自己的信息/想法/结构/资源；把合适的子任务（头脑风暴、检索、批判）交给 AI，思考留给自己；主动提出修正
  - L4 卓越 刻意*设计*协作——编排 AI 的角色、管理上下文、复用/沉淀可重用资产（skills/agents），分清什么该留、什么该弃

### 🚀 生成式驾驭 — 推理与论证
- **D4 多视角与让步** (QUEST-E · 论证评估)
  - L1 只站自己一方，无视反方 · L2 提到反方但轻描淡写 / 稻草人 · L3 主动找反方并正面回应 · L4 构建反方最强论证(steelman)后再让步反驳
- **D5 论证拆解** (QUEST-U · 论证分析)
  - L1 把观点当事实，不分论点论据 · L2 能复述但不辨结构 · L3 能识别论点-论据-假设结构 · L4 识别隐藏前提与论证谬误
- **D7 论证质量** (QUEST-S · ATL 沟通 · output)
  - L1 只堆观点 / 复制 AI 原话，无论点-论据结构 · L2 有结论但论据零散，结构不完整 · L3 论点-论据-解释结构完整，引用有出处 · L4 结构严谨且回应反方，论证链条经得起追问

### 🛡️ 批判式防护 — 信息素养
- **D2 信源辨识** (CRAAP)
  - L1 完全信任 AI / 来源，从不追问出处 · L2 偶尔问「真的吗？」但不深入 · L3 主动要求论据，能识别来源等级 · L4 主动交叉验证，识别信源之间的利益关系与冲突
- **D3 横向验证** (SHEG 横向阅读)
  - L1 只看单一来源，不另开查证 · L2 想到要多看，但没真去找 · L3 主动多源对照，找到 2+ 独立来源 · L4 溯到原始出处，比较各源权威性与一致性

### 🛡️ 批判式防护 — AI 元认知与边界
- **D6 反思与元认知** (ATL 反思 · TOK 认知者与知识)
  - L1 不觉察自己被 AI 影响 · L2 事后偶尔回顾 · L3 主动校准信心，觉察思维盲点 · L4 觉察自己作为认知者的位置，迁移方法
- **D8 信息再生产** (ATL 媒介伦理 · 学术诚信 · output)
  - L1 整段照搬 AI 输出，不标注、不改写 · L2 偶尔改写，但分不清哪些是 AI、哪些是自己的 · L3 明确区分 AI 贡献与个人加工，主动声明 AI 使用 · L4 在 AI 基础上有独立判断与增量，诚信声明清晰可核
- **D9 AI 边界与伦理** (TOK 知识与技术 · 伦理使用 · output) — *to be sharpened, see §5*
  - L1 把 AI 当全知，不质疑其可能出错或编造 · L2 知道 AI 会错，但不主动核查 · L3 主动核查 AI 可能幻觉处，识别其知识边界 · L4 系统性评估 AI 局限与伦理风险，按场景决定是否/如何用

## 4. Model scope — Hybrid (locked)

- **Per-task evaluation = source of truth.** Each task produces an immutable evaluation (that task's 10-dim scores +
  narrative).
- **Longitudinal student profile = projection.** The profile is *derived* by rolling up the sequence of per-task
  evals — not separately stored/mutated. Matches the platform's "project, don't duplicate" pattern (process tree,
  `llm_usage` view). Shows growth (e.g. D2 L2→L3 over a month) without dual-write consistency risk.

## 5. Dimension-set changes from the production baseline (locked)

- **+ D10 协作编排** added (the generative/orchestration face the old 9D missed; absorbs "state need clearly" stance,
  "bring your own substance", "design the collaboration", with "knows agents/skills, what to save/manage" as the L4 ceiling).
- **No standalone "Output Ownership"** dimension — not observable. Its observable trace (critique/refine the AI's
  output) folds into a **sharpened D9**.
- Result: **10 dimensions** total (old D1–D9 + new D10).

### 5a. Post-verification decisions (2026-06-29) — see `2026-06-28-cognitive-model-rubric.md` v2

Three independent adversarial audits (V1–V5) found the v1 anchors had real overlap & observability flaws. Resolutions:
- **Stay at 10 dims** — no swap, no new dimension.
- **D5 kept & re-scoped** (user: finding hidden premises/fallacies is important) → D5 = dissect a *given/AI/source*
  argument; D7 = quality of the *student's own* argument. Clean input-vs-output split.
- **No "Agenda Autonomy" dim** (user: *adopting AI's framing after clear thought is healthy* — not a defect). The
  real signal = **reflective vs default adoption**, folded into **D6** observably (default-adopt no-hedge = L1;
  adopt-after-stating-reasoning = L3).
- **D8 / D9 axis re-split:** D8 = provenance/attribution (mark AI-vs-self); D9 = epistemic verification (catch
  errors, revise/reject false output). My earlier D9-sharpen had collided them.
- **Single-owner tie-break rules** added (copy-paste→D8; redo-because-wrong→D9 vs redo-for-format→D10;
  counter-arguments→D4 only; multi-source→D3 vs single-source→D2; multi-turn→D10 vs single-prompt→D1).
- **Re-steering after a weak answer** (a missing driving signal) folded into **D10 L3**.
- **N/A verdict added** — absence ≠ L1; L1 now needs a positive tell. (user: yes)
- D6 rewritten on **observable proxies** (can't observe "non-awareness" directly).

## 6. Validity bar — V1–V6 (proposed; verification pending)

The detailed rubric must pass these (this is also our step-2 "test it"):
- **V1 · Distinctness (no overlap):** each dimension measures something the others don't; a behavior maps to one
  *primary* dimension. (adversarial overlap test)
- **V2 · Coverage (exhaustive, no dead weight):** the 10 span "good AI use in research"; nothing major missing, no
  dimension that never fires.
- **V3 · Meaningful monotonic progression:** L1<L2<L3<L4 is a real increase in *AI-use quality* (better use, not more words).
- **V4 · Observability:** every anchor phrased as a behavior visible in process data (text + card actions). If no
  signal can be pointed to, rewrite the anchor.
- **V5 · Boundary discrimination:** adjacent levels (esp. L2 vs L3) are reliably distinguishable.
- **V6 · Inter-rater reliability:** two independent evaluators (LLM-vs-LLM, or LLM-vs-human) scoring the same
  transcript land close.

**Convention (recommended, pending confirm):** each detailed anchor cites *which signal source* proves it —
**text-message evidence** vs **card-action evidence** — front-loading the two-source combination and making V4/V6 concrete.

## 7. Brainstorm sequence & where we are

1. ✅ **Lock the cognitive model** — content (10 dims) + structure (3-level taxonomy) + scope (hybrid). *(§1–§5)*
2. ✅ **Detail + verify the rubric** — full L1–L4 observable anchors for all 10 (rubric v2). Passed V1–V5 adversarial
   (3 independent agents) + the 2 boundary fuzzes sharpened.
2b. ✅ **V6 empirical test PASSED (2026-06-29).** Scored rubric v2 on 3 controlled transcripts (strong/weak/narrow),
   2 independent raters each: **inter-rater 28/30 exact (93%), 30/30 within-1-level; N/A 11/11 perfect;** discrimination
   monotonic on both faces (生成 weak1.0<narrow3.0<strong3.4; 防护 weak1.0<narrow2.5<strong3.5); tie-break rules held
   (trust-AI→D9 not D2; copy→D8 only). Fixtures in scratchpad.
3. ✅ **Ideal evaluation method** (ignore cost) — flagship LLM + full rubric + full transcript (text + card envelopes)
   → per-dimension score-or-N/A. Validated by 2b.
4. ✅ **Balance cost / perf / speed** — hybrid anchor: deterministic rules pre-compute hard facts → anchor flagship LLM
   (see §9). Per-task flagship is affordable; rules are for reliability + speed, not cost-substitution.
5. ✅ **Realize it** — data model (5a), trigger + async/milestone path (5b), presentation + privacy boundary (5c). *(§10)*
   **All 5 steps complete → consolidate into a formal spec, then feed P4 build when chosen.**

## 9. Evaluation pipeline — Step 4 (cost/perf/speed)

- **Per-task flagship eval is affordable** (eval is per-task, not per-turn → a few calls/student/week). The platform
  already runs ~the ideal method (`deepseek-reasoner` over full transcript + rubric). So rules are **not** a cost
  substitute — they're for **reliability** (ground-truth facts the LLM shouldn't infer; deterministic N/A) and **speed/UX**.
- **Decision (locked): hybrid anchor.** Flagship LLM is the sole scorer; deterministic rules pre-compute **hard facts**
  injected as a structured "客观信号" block to anchor the LLM and prevent evidence hallucination.
- **Rule pass = HARD facts only** (no brittle keyword/semantic guessing — that stays with the LLM):
  - Card: per summoned card → {status, op_count from event_trace, filled/empty fields}.
  - Sources: count of distinct URLs/sources the *student* introduced (vs AI-introduced).
  - Copy: max verbatim overlap between a student message and any prior AI message (→ D8 paste detection).
  - Contribution: did the student post a URL or a long original text block before any AI substantive reply.
  - Counts: student message count, turn count.
  - Derived N/A-candidates: 0 student-sources → D2/D3 N/A-candidate; no reused content → D8 N/A-candidate; etc.
- **Injection contract (anti-hallucination):** the 客观信号 block is framed as *facts the LLM must respect but may
  interpret* — it may override an N/A-candidate if the dialogue shows the behavior, but may NOT claim a fact that
  contradicts the signals (e.g. credit sources beyond the counted number). *(signal set confirmed 2026-06-29)*

## 10. Step 5 — Realize it

### 5a · Evaluation data model (locked)
- Extend existing `evaluations`: `level` enum gains **`NA`**; add **`signals jsonb`** (rule-computed hard facts that
  anchored the eval) + **`rubric_version text`**. No new tables, no faces/categories columns (static rubric metadata),
  no profile table (the longitudinal profile is a *projection* over a student's task-evals).
- Each eval run = a new **immutable snapshot** row; latest shown; re-eval allowed (creates another snapshot).

### 5b · Trigger + async path (locked, "for now")
- **Two cadences, two cost tiers** (decouple *compute* from *show*):
  - **Cheap rule signals → every turn**, ~free, silent (powers live process tree + teacher freshness + anchors the LLM).
  - **Flagship LLM eval → async + infrequent**, shown to the student **on-demand**.
- **Trigger = milestone, mirroring the card restraint law** (「触发自动，打开由学生确认」):
  - **Auto-compute** (async river job, non-interrupting) on a **milestone = first completed card OR N substantive
    turns** (whichever first), **debounced + capped** (a few flagship calls/task, never per-turn).
  - **Offer, don't push, the reveal** — a quiet "你的思维印记有新内容" indicator the student opens by choice. No live
    ticking score, no badges/streaks/numbers (铁律 #2).
  - Plus always-on **on-demand pull** + re-eval.
- **Why milestone (not idle/completion):** we don't own task completion; many tasks stay unfinished. Milestone +
  the **N/A verdict** means a partial task evaluates gracefully (scored where there's evidence, N/A elsewhere — looks
  *in progress*, not broken/punished). First milestone = the proactive "wow" that demonstrates value early.
- **Revisit** the exact milestone/cadence/cap once real cost data exists (user note).

### 5c · Presentation (locked)
- **Student — 你的思维印记** (upgrade EvalModal to the 3-level taxonomy): headline = 2 faces; drill = 4 categories →
  10 dims (L1–L4); **N/A renders neutrally** ("本次未涉及", greyed — never a low/zero bar); + narrative (diagnosis +
  one next step). **Growth/trend view** (longitudinal projection across tasks) = fast-follow after the per-task snapshot.
- **Teacher** sees: per-student rubric **levels** (category rollup, drillable to 10 dims) + the **diagnostic narrative**
  + class aggregates + trends. Teacher does **NOT** see the raw AI **transcript** (the student's thinking process stays private).
- **PRD update (deliberate):** the narrative was 仅你可见; now **student + teacher**, with the **transcript student-private**.
  The privacy boundary = teacher sees the *assessment* (levels + narrative), not the *raw process* (dialogue).

> **Step 5 complete.** Full evaluation-model design is locked end-to-end (steps 1–5). Next: consolidate into a formal
> spec (`docs/superpowers/specs/`), then it's ready to feed P4 (async eval) implementation when the user chooses to build.

## 8. Open questions / parking lot

- Exact `evaluation` data-model fields (currently provisional: `scores[]` + `narrative`) — revisit in step 5.
- How rules-based cheap signals and LLM scores *combine* per dimension — step 3/4.
- Presentation specifics for the 3-level rollup in 你的思维印记 (student) and the teacher roster — step 5.
- Profile projection math (how per-task evals roll up to a trend) — step 5.

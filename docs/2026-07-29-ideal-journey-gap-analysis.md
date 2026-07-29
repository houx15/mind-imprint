# Gap Analysis — Product vs. the Educational-Scientist Ideal Journey

> Compares 思维印记 (after S1–S5) against `docs/2026-07-29-interaction-mock.csv` — a 519-row simulated ideal journey (Phoebe / "Did China make the Earth more sustainable?") authored as the pedagogical gold standard.
> Companion: `2026-07-29-card-placement-map.md` (this reconciles its canon gaps against the ideal's *actual usage*).

## 0. How to read the ideal journey (the numbers)

- **519 rows** = 198 interaction turns + 321 internal 批注 (evaluation/annotation). The AI *reply* is short and restrained; most rows are the student thinking or the rubric annotating.
- **460 student-initiated vs 47 AI-prompted** — overwhelmingly student-driven. Scaffolding density is **低 (low)** almost everywhere; **高 (high)** concentrates in reading (P2), where the AI supplies card *skeletons* the student fills (**套版, 43×**).
- **Phase distribution:** P2 资料/文献 237 · P4 产出与分析 142 · P5 反馈与修订 71 · P3 方法/RQ 46 · P6 展示/答辩/反思 22 · P1 任务进入 1.
- **Phases are non-linear:** `1→2→3→2→3→4→5→6` — the student loops from method back to reading.
- **The AI's invariant behavior:** never concludes, never writes the deliverable, checks *function / boundary / overclaim*, asks **one** question, **holds red lines** (refuses to watch a video / fetch a source for her), lets the student drive.

## 1. The ideal's tool fingerprint (lens/card tags, by phase)

| Phase | Top tools the ideal uses | count |
|---|---|---|
| P2 reading | **SIFT 41**, 利益相关方 8, 数据三问 6, CRAAP 2, To-What-Degree 2, 论证解剖/warrant 2, 话语解码 1, 主张追问 1 | — |
| P3 method/RQ | (almost none — conversational RQ refinement; To-What-Degree 1) | — |
| P4 writing | 数据三问 4, 主张追问 3, AI克制 2, 论证解剖/warrant 2, 段落功能检查 1, 证据预算 1, 红线词表 1, 未打开来源红线 1 | — |
| P5 review | 未打开来源红线 2, 主张追问 2, 证据预算 1, 数据三问 1, 引用审计 1, AI伦理审计 1, 段落功能检查 1 | — |
| P6 defense | (Socratic self-defense on source function+boundary; no new tools) | — |

**Cross-cutting discipline the ideal runs constantly:** a **source log with function columns** — *why opened · can show · cannot show · where it goes* (the 套版 43×), and **口径 (data-caliber) tracking** (annual vs cumulative, territorial vs consumption-based).

## 2. What the product already matches (strong coverage)

| Ideal behavior | Product (slice) | Verdict |
|---|---|---|
| One continuous, restrained coach; one-question-at-a-time; never writes the deliverable | Continuous per-project coach + `projectCoachPosturePrompt` (S1) | ✅ strong |
| Non-linear phase revisits without amnesia | One continuous session + spine projection + compaction (S1/S4) | ✅ strong |
| Deep per-sentence reading with SIFT / CRAAP / disciplinary angles (P2/P4) | Reading room (hang-card-on-sentence + you-find-evidence) + 9 学科透镜 + SIFT/CRAAP | ✅ strong (SIFT is the ideal's #1 tool, 41×) |
| Purposeful reading brief → compact takeaways | Reading sub-agent brief-in/takeaways-out (S2) | ✅ |
| Rabbit-hole: which sources, how they branch, dangling to connect/prune | Exploration graph + 深挖一层 guide (S3) | ✅ (richer than the ideal's flat "rabbit hole log") |
| 数据三问 / data literacy; 论证解剖 / warrant | data-literacy + argument-map cards | ✅ |
| Program proposes a tool at the right moment (克制 ladder) | Cross-phase card proposing (S4) | ✅ |
| 复盘我与 AI 的互动 + AI-use statement (used-for / not-used-for) | AI-interaction retrospective (S5) | ✅✅ — and **grounded in the real event record**, not the student's memory (arguably *exceeds* the ideal, which self-audits from recall) |
| Defense-readiness review conversation ("review不是polish") | Review-room coach thread scope=reflection (S5) | ✅ |
| Process-as-data + rubric assessment (DualAxis + lenses) | Assessment sub-agent (flagship) + growth report | ✅ |

**Net:** the *architecture and posture* of the ideal are well realized. The AI's restraint, the reading depth, the continuity, and the review/AI-use retrospective all land.

## 3. Gaps — where the ideal does something the product lacks or only partially supports

Ranked by how load-bearing the gap is in the ideal journey.

### G1 · Source-function log (why-opened / can-show / cannot-show / where) — **the ideal's evidence backbone; 43× 套版** · HIGH
The ideal's spine is a per-source table with **function columns**: *why I opened it · what it can at most show · what it cannot show (esp. no big conclusion) · which paragraph it lands in · its status (used / candidate / removed)*. This is what keeps her from source-stacking and over-claiming, and it drives the entire P6 defense.
- **We have:** references + S2 reading-takeaway (findings / credibility / key_quotes / new_leads / proposal_impact).
- **Missing:** the explicit **can-show / cannot-show / where-in-essay / status** framing as a first-class, per-source card the AI supplies as a skeleton (套版) and the student fills. S2's takeaway is adjacent but is a *rollup*, not the running *function-and-boundary ledger*.
- **Recommendation:** a **信源功能卡 (source-function card)** — fields: 打开理由 · 最多能证明 · 不能证明 · 拟用段落 · 状态(used/candidate/removed). Fires on `source_opened` in the reading room / 文献库. This is the single highest-leverage gap; it also feeds the assessor and the P6 defense directly.

### G2 · 口径 / data-caliber tracking — **appears at every data source** · HIGH (for data-heavy topics)
The ideal repeatedly separates *CO2 vs GHG · fossil/industry vs land-use · annual vs cumulative · territorial vs consumption-based* and refuses to let numbers be mixed. No product tool encodes this.
- **Recommendation:** a **口径卡 (data-caliber card)** attached to a data source — a small matrix the student fills so downstream claims can't silently mix calibers. Complements data-literacy (数据三问).

### G3 · Plan-first: 证据预算 + 段落功能检查 — **P4's opening move; the plan gap the card-map already flagged** · HIGH
Before any prose, the ideal writes a **paragraph-order + per-paragraph function + word budget**, and the AI checks *function and over-reach only, never writes*. The card-placement map §4 already flags **P2/plan cards are nearly empty**.
- **We have:** the writing room + outline; a `plan` room.
- **Missing:** a **写作计划卡 (evidence-budget + paragraph-function)** — per-paragraph {function · word budget · which sources land here}, with the AI checking function/over-reach. Directly matches the ideal's P4 opening and the card-map's flagged plan-card gap.

### G4 · Claim-interrogation 主张追问 (T34) · MEDIUM
Used 3× (P4/P5) to pressure-test a thesis to its boundary ("full yes越界 / no抹掉正面证据 → qualified yes"). The card-map flags our `question-card` as too generic (≈T35, not T34).
- **Recommendation:** a dedicated **主张追问卡** (claim → what would over-reach it / under-reach it → the bounded version). Now cheaply reachable via S4 cross-phase proposing.

### G5 · Stakeholder map 利益相关方 (T38) · MEDIUM
8× in reading (P2) to place a source's interest/vantage. Card-map flags `perspective-matrix ≠ stakeholder`.
- **Recommendation:** build **利益相关者地图 (T38)** as a reading-room card.

### G6 · Integrity review pack: 引用审计 + 未打开来源红线 · MEDIUM
P5's review order is claim → evidence → **citation/paraphrase** → **AI-use** → package. S5 shipped the **AI-use** audit. Still missing: **引用审计** (quote vs paraphrase, no公众号-as-body-evidence) and the **未打开来源红线** (a source with no URL/open-reason can only be *candidate*, never *used*).
- **Recommendation:** an **引用审计卡** + a hard **未打开来源红线** check (derivable from the G1 source-function log's status column — so G1 unlocks this cheaply).

### G7 · Explicit AI red-line refusals · LOW–MEDIUM
The ideal AI *refuses* to watch a video / fetch a source for her, then she does it herself (boundary repair). Our 克制 posture forbids writing the deliverable but doesn't enumerate these artifact-level refusals.
- **Recommendation:** add explicit red-line refusals to the coach posture (won't watch/read *for* you, won't fetch, won't predict scores) — cheap, high-fidelity to the ideal.

## 4. Summary

- **Posture & architecture: matched.** Restraint, one-question, reading depth, continuity, rabbit-hole, cross-phase proposing, and the AI-use retrospective/defense-readiness review are all realized; S5's retrospective is **grounded in the real event record**, arguably beyond the ideal.
- **The gaps are almost all one shape: the evidence-function discipline.** G1 (source-function log), G2 (data-caliber), G3 (evidence-budget/paragraph-function), G6 (citation/unopened-source) are the ideal's *evidence backbone* — the discipline that turns "resources" into "bounded, placed, warranted evidence" (the student's own closing 印记). This is the product's biggest opportunity, and it's coherent: **G1 is the keystone** (its status column unlocks G6; its function columns feed G3 and the assessor).
- **These are all card-shaped** (new JSON, no renderer work — per the 铁律), and now **cheaply deployable via S4 cross-phase proposing**. Recommended build order: **G1 → G3 → G6 → G2 → G4 → G5 → G7**.
- **Cross-reference:** G3/G4/G5 are exactly the T34/T38 + plan-card gaps the card-placement map already flagged — the ideal journey *confirms and prioritizes* that backlog with real usage frequencies.

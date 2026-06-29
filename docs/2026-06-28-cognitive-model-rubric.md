# Cognitive Model — Detailed Observable Rubric (v2, post-verification)

> **v2** incorporates the V1–V5 adversarial audit (3 independent agents) + user decisions.
> Companion to `2026-06-28-evaluation-model-design-notes.md`.
> Still **10 dimensions** (no swap, no new dim). Changes from v1: every overlap de-conflicted via single-owner
> rules; D6 rewritten on observable proxies; D5 re-scoped (analyze others'/AI's arguments) vs D7 (your own);
> D8 = provenance / D9 = epistemic verification (clean axis split); re-steering folded into D10; **N/A verdict added**.

## Scoring model (applies to every dimension)

Each dimension resolves to **N/A** or **L1–L4** (SOLO: L1 萌芽 · L2 发展中 · L3 熟练 · L4 卓越).
- **N/A = insufficient evidence.** The task/session produced no behavior bearing on this dimension. **Never infer
  L1 from silence** — a short or narrow task should read N/A, not a wall of L1s.
- **L1 requires a *positive tell*** — an observed low-quality behavior — not mere absence of the high-quality one.

Signal tags: `[TEXT]` = chat-message evidence · `[CARD]` = card-envelope evidence (`field_values`/`event_trace`/`status`)
· `[BOTH]`. (Card-channel tags are opportunistic — reconcile against each card's `rubric_tags` at build; see parking lot.)

## Discriminant tie-break rules (the backbone of V1 — one behavior, one owner)

| Behavior | Primary owner | Not |
|---|---|---|
| Copied AI output verbatim | **D8** (provenance) | not D7/D9/D10 — only *also* D9 if the copied content was false/unchecked |
| Asked AI to redo/fix its output | **D9** if because it's *wrong/hallucinated*; **D10** if for *format/role/scope* (workflow) | — |
| Recognizing claim–evidence structure | **D5** if dissecting a *given/AI/source* argument; **D7** if it's the *student's own* argument | — |
| Used 2+ sources / cross-checked | **D3** | not D2 |
| Judged one source's credibility | **D2** | not D3 |
| Engaged a counter-argument | **D4** | D7 never scores counter-engagement |
| Multi-turn decomposition / iteration | **D10** | D1 is single-prompt only |
| Uncritically trusting the AI | **D9** (AI as oracle) / **D6** (unreflective adoption) | D2 is about *external sources*, not the AI |

---

## 🚀 生成式驾驭 (Driving AI & your own thinking)

### 意图与编排 (Intent & Orchestration)

**D1 提问清晰度** — *single-prompt clarity (iteration → D10)*
- **N/A** 几乎不会触发（每个会话都有开场问）。
- **L1** 开场问句只有一句话，无背景/目标/方向约束（纯字数/格式要求不计为实质约束）。 `[TEXT]`
- **L2** 给了背景或目标之一，但约束/期望模糊，AI 需反问澄清。 `[TEXT]`
- **L3** 单次提问即含背景+目标+约束，问题具体可执行。 `[TEXT]`
- **L4** 单次提问还分层给出子问题与期望产出格式，便于 AI 精准应答。 `[TEXT]`

**D10 协作编排** — *Orchestration & Contribution; now owns cross-turn re-steering*
- **N/A** 仅发一条消息且未用卡，无可判断的协作行为。
- **L1** 把 AI 当答案机器：直接要成品，不带入自己的材料，对回复不追问、不调整。 `[BOTH]`
- **L2** 被 AI 追问后才补充自己的材料；不主动规划协作步骤。 `[BOTH]`
- **L3** 未经提示就带入自己的草稿/链接/提纲，并**跨轮驱动改进**——回复不满意时缩小范围/给例子/调整任务，把子任务（检索/头脑风暴/批判）指派给 AI。 `[BOTH]`
- **L4** 跨步骤编排 AI 角色、管理上下文、复用/沉淀可重用结构（模板/skills/agents），分清留与弃。 `[BOTH]`
> *vs D9:* D10 steers for **process fit** (scope/format/role). D9 corrects for **being wrong**.

### 推理与论证 (Reasoning & Argument)

**D4 多视角与让步** — *owns ALL counter-argument engagement*
- **N/A** 任务无可争议的主张（如纯事实查询）。
- **L1** 给出立场但完全不提反方。 `[BOTH]`
- **L2** 提到反方却轻描淡写或稻草人化。 `[BOTH]`
- **L3** 主动取得反方观点并正面回应。 `[BOTH]` (CARD：让步卡填入真实反例+回应)
- **L4** 先把反方强化为最强论证 (steelman) 再让步反驳。 `[BOTH]`

**D5 论证拆解** — *re-scoped: dissect a GIVEN / AI / source argument*
- **N/A** 没有可供拆解的论证（自己的或他人的）。
- **L1** 复述某来源/AI 的结论，却当作事实、不分论点与论据。 `[BOTH]`
- **L2** 能准确复述该论证，但不标出论点/论据/假设。 `[TEXT]`
- **L3** 对所读材料或 AI 回复，明确标出论点-论据-假设结构，或指出来源对证据的扭曲/断章取义。 `[BOTH]`
- **L4** 进一步指出其**未明说的隐藏前提**，或**点名某一具体谬误类型**（如以偏概全、滑坡、诉诸权威）。 `[BOTH]` ← *the high-value rung*
> *vs D7:* D5 = analyze *someone else's / the AI's* argument. D7 = quality of the *student's own* argument.

**D7 论证质量** — *produce-own only (copy-paste → D8, counter-engagement → D4)*
- **N/A** 本次会话学生未产出自己的论证。
- **L1** 自己的论证只堆结论或观点，无论点-论据支撑。 `[TEXT]`
- **L2** 有明确结论，但论据零散、claim 与 evidence 未连接。 `[TEXT]`
- **L3** 自己的论证：论点-论据-解释结构完整，引用有出处。 `[BOTH]`
- **L4** 论证链严密、经得起追问。 `[BOTH]`

---

## 🛡️ 批判式防护 (Not being captured)

### 信息素养 (Information Literacy)

**D2 信源辨识** — *single-source credibility only (cross-verify → D3; AI-trust → D9)*
- **N/A** 本次会话未引入任何外部来源。
- **L1** 引入了来源却从不追问其出处/可信度。 `[BOTH]`
- **L2** 偶尔问「这可靠吗？」但不深入。 `[TEXT]`
- **L3** 主动要求出处，并能判断**单一来源**的等级/资质。 `[BOTH]` (CARD：CRAAP/SIFT 字段)
- **L4** 识别该来源的立场、资助或利益冲突。 `[BOTH]`

**D3 横向验证** — *multi-source comparison*
- **N/A** 全程只有一个来源（或没有）。
- **L1** 只用单一来源，未另开查证。 `[BOTH]`
- **L2** 口头说「该多看几个来源」但未真正找第二个。 `[TEXT]`
- **L3** 主动多源对照，引入 2+ 独立来源。 `[BOTH]`
- **L4** 溯到原始出处，比较各源权威性与一致性。 `[BOTH]`

### AI 元认知与边界 (AI Metacognition & Boundaries)

**D6 反思与元认知** — *rewritten on observable proxies; reflective-adoption built in*
- **N/A** 会话过短 / 无出现可显示反思的决策点。
- **L1** 直接采用 AI 的措辞或方向，无任何犹豫、限定或保留（默认式采纳）。 `[TEXT]`
- **L2** 事后才回顾「也许该……」，但当时未自检。 `[TEXT]`
- **L3** 当场陈述自己的不确定/信心，或点名一个思维盲点；**采用 AI 方向前先说明自己的理由（反思式采纳）**。 `[BOTH]`
- **L4** 觉察并把方法迁移到新子问题/新情境。 `[TEXT]`
> User principle: *adopting AI's framing after clear thought is healthy.* L3 rewards **reflective** adoption;
> L1 is **default** adoption with no hedge. This closes the "agenda-capture" gap without a new dimension.

**D8 信息再生产** — *PROVENANCE / attribution owner; canonical copy-paste detector*
- **N/A** 本次会话学生未复用任何内容。
- **L1** 整段照搬 AI 输出，不标注、不改写。 `[TEXT]`
- **L2** 有改写，但未标明哪些来自 AI、哪些是自己。 `[TEXT]`
- **L3** 明确区分 AI 贡献与个人加工，主动声明 AI 使用。 `[TEXT]`
- **L4** 诚信声明清晰可核，标注 AI 贡献边界。 `[TEXT]`
> Re-split: D8 owns *attribution* ("did you mark AI-vs-self"). Independent judgment/increment lives in D7/D10.

**D9 AI 边界与伦理** — *EPISTEMIC verification owner*
- **N/A** AI 本次未做出可核查的事实主张。
- **L1** 把 AI 当全知，对其事实主张不质疑是否出错/编造。 `[TEXT]`
- **L2** 口头承认「AI 可能不准」，但不采取核查动作。 `[TEXT]`
- **L3** 主动核查 AI 的可疑/可能幻觉处，发现问题即指出。 `[TEXT]`
- **L4** 因核查结果**实质修订或拒用** AI 的产出。 `[TEXT]`
> Re-split: D9 = correction triggered by *being wrong*. (Workflow-steering → D10; attribution → D8.)

---

## Verification status (V1–V6)
- **V1 Distinctness:** addressed via the tie-break table + single-owner re-scoping. *Re-test recommended on v2.*
- **V2 Coverage:** re-steering folded into D10 L3; agenda-capture folded into D6 L1/L3. D5 retained (user).
- **V3 Monotonic:** D1 L4 pulled back on-axis; D5/D6 ladders rebuilt.
- **V4 Observability:** D6 rewritten on proxies; all L1 anchors now positive tells; N/A added.
- **V5 Boundary:** L2/L3 sharpened (esp. D6, D9 via *stated* fallibility vs *action*).
- **V6 Inter-rater reliability:** empirical — run v2 twice on the Phoebe golden-path transcript and compare.

## Parking lot
- Reconcile each `[CARD]` tag against the actual card library's `rubric_tags` at build (current card tags may
  predate this dimension set — possible drift, e.g. cards tagging "D1_来源意识" vs rubric D2).

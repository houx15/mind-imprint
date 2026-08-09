# Product Spec: The Writing Studio (AI 写作工作室)

| | |
|---|---|
| **Status** | Draft v3 — merged |
| **Author** | Product |
| **Last updated** | July 2026 |
| **Scope** | The Studio module (练). Course library (学) and Growth Report (测) covered in `思维印记_论文板块产品设计文档_v3.md` |
| **Supersedes** | v2 (`archive/workbench-spec-v2.md`), v1 (`archive/workbench-spec-v1.md`) |
| **Merged from** | `思维印记_论文板块产品设计文档_v3.md` (content, pedagogy, compliance, evaluation) × workbench-spec v2 (interaction architecture) |

---

## 0. What changed in v3

v3 merges the 论文板块 design document into this spec. **Where the two conflicted, 论文板块 wins.** Its seven-qualification rubric research is the moat; this spec supplies the interaction spine.

**Adopted from 论文板块 (new here):** the S0–S6 meta-essay journey with gates · the four compliance red lines · 装备栏 (student-invocable tool rail) · 保真层 validator rule · 言语化门控 · 三键处置 · search log / 兔子洞日志 · 卡壳救援三级 · AI 验货挑战 · 考官人设 · AI 使用申报单 · 孤儿证据 + 裸主张 · word-count budget · five readiness-gauge regimes · six-dimension rubric (D1–D6) · event stream · 思维跳跃 T1–T7 · 深×自由四象限 · three report versions · Cambridge-first MVP.

**Retained from v2 (they have no equivalent):** the State Evaluator · the card as a staged experience over four views · scaffolding fade · Draft Snapshot immutability with edit/preview · the first-entry recognition moment · situated memory on return · `resolve_remediation()` · the zero-code config contract · "a new review requires a new snapshot."

**Three reversals, stated plainly so they can be found and undone:**

| Reversal | Was | Now | Cost |
|---|---|---|---|
| **R-A. Gates replace "checklist, not pipeline."** | v1/v2 §5: no gated steps; **reverse entry first-class** — a student could paste 800 words before any structured thinking. | S0–S6 stations; a gate must be passed to unlock the next station (DEC-8). | **Reverse entry is removed.** A student arriving with a finished draft must still walk the journey. This was your own v1 decision, not an outside proposal. |
| **R-B. Teacher surface ships in v1.** | v2 DEC-2: no teacher accounts; export is the wedge. | Teacher report version + 师判 gate items + teacher controls (DEC-2, revised). | Brings identity, student–teacher relationships, and a second surface into v1 scope. |
| **R-C. Cards are student-invocable.** | v2 R-4 / v0.2 §5.3: cards surface only when the AI decides; no tool rail. | 装备栏 — cards are buttons; every click tagged 自发 / 提示后 (DEC-10). | None. This was a **bug fix**: our north-star metric (§14) was unmeasurable without it. |

`D1`–`D7` (decisions) are renamed **`DEC-1`–`DEC-11`** throughout, to end the collision with the six assessment dimensions `D1`–`D6`, which are platform-wide and predate this spec.

---

## 1. Overview

The Studio is where a student brings a real, high-stakes writing task — an IGCSE 0457 Individual Report, a GPR 9239 Essay, an EPQ report, an AP Seminar IWA, an AP Research paper, a TOK essay, an EE — and works it from prompt to finished draft. **The AI questions, challenges, and diagnoses. It never writes.**

The Studio is one of three pillars:

> **论文板块 = 论文课程库（学什么）× AI 写作工作室（怎么练）× 思维成长报告（测什么、向谁证明）**

It is **not** a chat tutor, **not** a writing assistant, and **not** a self-writing platform. It is a structured workspace where thinking becomes inspectable artifacts, and the coach intervenes on the state of those artifacts.

**Why it can exist at all:** across all seven qualifications, teacher feedback is *institutionally locked* (IB: one draft, one comment; Cambridge: near-total prohibition on graded feedback; AP: process guidance only; EPQ OCR: oral feedback only). The feedback vacuum is structural. **Self-assessment ability + out-of-school process feedback is the only space the exam boards leave open.** We do not compete for the right to mark a final draft — that is a violation. We train the student to become their own examiner.

---

## 2. The four red lines

Inherited from the platform, tightened for the essay context. These are not guidelines.

**RL-1 — The AI never writes essay content.** No sentence that could be pasted into the essay. Rewriting advice appears only as **two options plus the student's stated reason** (keep as written / student revises / explain why not). Text quality ≠ mastery of thinking.

**RL-2 — The AI never generates a citation, and never paraphrases a source the student has not opened.** Citations may only be added from sources present in the student's own search log, actually opened. One rule closes hallucinated citations and academic dishonesty simultaneously.

**RL-3 — Readiness ≠ predicted grade (就绪度 ≠ 预估分).** The platform outputs *where your draft sits against the official descriptor*, never *what grade you would get*. This is both the differentiation from RevisionDojo and the compliance safety margin. It appears in product copy and in sales scripts.

**RL-4 — Reflection is written by the student.** The platform supplies material packs (timestamps, verbatim quotes, decision records) and prompting questions. It never drafts an EE reflection statement, an EPQ evaluation chapter, an AP PREP response, or a GPR reflective piece — **those texts are themselves scored** (EE Criterion E = 4 marks; 9239 research log = 10 marks; POD rows 3 and 7 = 6 marks).

---

## 3. Goals

- **G1.** A student with a real title produces meaningful thinking within 60 seconds of first entry.
- **G2.** Every student input is a reusable piece of their eventual essay. Zero "filling forms for the platform's sake."
- **G3.** Guidance is always anchored to this student's specific title, specific sources, and the specific board's criteria. Generic advice is a defect.
- **G4.** Process is captured as a byproduct of work, yielding (a) formative assessment data and (b) integrity evidence compatible with official process documents (TOK PPF, EE RPF, 9239 research log, EPQ Production Log / Activity Log / PPR, AP PREP & checkpoints).
- **G5.** The student retains authorship at all times. Every AI suggestion is refusable, and refusing well is scored as a strength (T3).

## 4. Non-Goals

- Team/presentation components (AP TMP/IMP delivery, GPR C3, POD presentation). Different interaction form; out of scope for v1.
- A global free-form chat assistant (R-8).
- A competitive document editor. The `写作` view is a plain buffer; we recommend the student's own editor (§9).
- **Inline AI writing assistance of any kind** — autocomplete, ghost text, "apply suggestion," "rewrite this." Not a scope cut; a violation of RL-1.
- Predicted grades, ever (RL-3).
- Mobile. Desktop web only for v1.

---

## 5. Core model: the meta-essay journey (S0–S6)

**Design decision:** rather than build seven pipelines, abstract **one journey**. Qualification differences collapse into sprint packs, gate parameters, and the readiness-gauge rendering layer. Seven products, one core; a new qualification costs one sprint pack plus one parameter set.

| Station | Name | Core artifacts (versioned, exportable) | Gate (must pass to unlock next) | Default view | Dimensions |
|---|---|---|---|---|---|
| **S0** | 任务解码 Task decoding | Rubric translation table + milestone plan | Student restates *in their own words* what this artifact is assessed on; flags their 2 weakest criteria | 评估 | D1 |
| **S1** | 立题 Framing the question | Research-question worksheet: title + operationalized key terms + feasibility check | Single interrogative; key concepts have testable definitions; **pre-registration**: "what evidence would change my answer?" | 结构 | D1 D4 |
| **S2** | 视角与素材 Perspectives & sources | Perspective map + search plan + 兔子洞日志 | ≥2 perspective lines **at different levels** (per qualification parameter); ≥2 opened sources per line | 素材 | D3 D4 |
| **S3** | 信源评估 Source evaluation | Source dossier — one evaluation card per source | 100% evaluation-card completion; each carries a written **「作用与风险」**; key claims lateral-read | 素材 | D3 |
| **S4** | 论证构建 Argument construction | Toulmin argument map ⇄ outline | **The hardest gate.** No orphan evidence, no naked claims; every backbone **warrant verbalized**; **steelman test** passed; concession paragraph has a node | 结构 | D2 D4 |
| **S5** | 成稿打磨 Drafting & polish | Draft Snapshot chain + word budget | Word count in the compliant band; citations doubly matched; whole-draft review passed | 写作 | D1 D5 |
| **S6** | 反思归档 Reflection & archive | Reflection material pack + AI usage declaration + process-document export | Student personally drafts the reflection; declaration signed | 评估 | D6 |

**回流 (backflow) is a positive signal.** Any station's output may overturn an upstream one — S4 revealing the argument cannot carry the title sends the student back to S1. The system records this as **有据修正 (evidence-based revision)**, never as failure. This is what AP Research's Chief Reader means by "research is iterative and recursive," and it is where thinking leap **T2** most often occurs.

> **Consequence of the gate model (R-A).** Reverse entry — pasting a finished draft and having the checklist back-fill — is **removed**. A student who arrives with prose already written must still walk S0→S5. If this proves to be the dominant real-world entry, revisit DEC-8.

### 5.1 Per-qualification station notes (difference lives in parameters, not structure)

- **S0** — 0457/9239: embed the official mark scheme, translated row by row. TOK: 6-title workbench + adverb ladder. AP Seminar: **stimulus workbench** (cross-material theme matrix — lock a theme shared by ≥2 stimulus materials *before* framing the question, protecting the IWA's legitimate parentage). AP Research: band-portrait comparison. EE: new/old guide switched by enrolment year.
- **S1** — EPQ: hard **operationalization** check (examiner-report trap: "野孩子能否恢复" without defining *recovery*) + hour estimate (guards OCR's named too-wide/too-narrow extremes). EE interdisciplinary: 双学科概念桥. AP-R: gap identification (the 4→5 band divide). AP Seminar: **scope-narrowness switch** (IWA row 2 is 0/5 — 「印度的水污染」passes, 「空气、水与土地污染」fails).
- **S2** — 0457: mandatory local/national **+** global double layer. GPR: mandatory globally contrasting perspectives. TOK: "perspective" = AOK and epistemic stance. **All AP paths use the official definition: 「视角 = 通过论证传达的观点」— facts, topics, lenses, and generalized stakeholders ("teachers," "students") do not count.**
- **S3** — AP paths add a **credibility-phrase check** ("记者 John Doe 解释道" fails to establish credibility → prompt for academic credential, institution, or method) and an **old-data justification check**. Encyclopedias and dictionaries are marked as non-qualifying sources.
- **S4** — AP-R: **alignment checker** (question ⇄ method ⇄ conclusion; move one point, the other two are re-flagged). GPR: two-perspective balance check (AO2a's 3 marks are lost to favouritism). AP Seminar: **organization × connection dual diagnostic** (IWA row 4's official 8-mark phrasing: *one of the two, not both*).
- **S5** — word budget parameterized. EE: **evaluation-density scan** (Criterion D is 8 marks and lives at the end; the 4,001st word is not read — overrun sacrifices the heaviest criterion first). EPQ: de-emphasize word count, check that all four AOs have evidence in the report.
- **S6** — export template forks: 9239 research log · EPQ three-board logs · EE RPF material (viva voce briefing) · AP PREP material and checkpoint simulation.

---

## 6. Object model

| Object | Definition | Example |
|---|---|---|
| **Project** | One writing task: qualification + title/question + deadline. Root container. | "TOK May 2027, Title 3" |
| **Station** | One of S0–S6. Holds gate items. | S4 论证构建 |
| **Gate item** | One condition for passing a station. States: `empty` / `draft` / `flagged-weak` / `solid`. Machine-judged (机判), verbalization-judged (言语化), or teacher-judged (师判). | "Backbone warrant verbalized" |
| **Artifact** | A student-created unit of thinking. Typed, versioned. | Position statement, Toulmin node, source evaluation card, reflection entry |
| **Draft Snapshot** | The student's prose at a commit point — pasted, or committed from the `写作` buffer. **Immutable.** One commit = one version = one reviewable unit. | "Counterclaim paragraph, v2, Oct 14" |
| **Edit buffer** | Mutable scratch text in `写作`. Private, silent, **not a record**. The AI never writes to it. | — |
| **Material** | A text with span indices: a source, or a Draft Snapshot. Interventions anchor to spans within it. | A NASA press release |
| **Card** | The platform's atomic thinking tool. Declares a target, an ordered list of **stages**, rubric tags, per-qualification vocabulary. **Never rendered as a form**; it is an input to the AI. `card_id` routes between Studio and Course. | 钢人卡, SIFT, CRAAP, 让步段卡, Toulmin 卡 |
| **Stage** | One step of a card: a `view` from the closed set of four, a `goal`, an `entry_condition`, AI behavior guidance. | `{ view: "素材", goal: "every claim has a checked source" }` |
| **Intervention** | One AI question or diagnosis. Anchored to an artifact or span; tagged with the criterion it serves; typed output (§15.3). | Margin note on ¶3: "This describes rather than argues — 0457 Table B" |
| **Disposition** | The student's response to an intervention: `accept` / `reject` / `rewrite`, each with a written reason (≥15 chars). | — |
| **Process Record** | Auto-generated projection of artifact version history + event stream. Not directly editable. | "Position revised twice after counter-evidence" |

---

## 7. Interaction laws (交互铁律) and AI behavior requirements

### 7.1 The six laws — each with a machine-executable definition

1. **只提问不代写.** No AI output may contain a complete sentence directly pasteable into the essay. **保真层 rule:** if AI output is declarative AND its semantic similarity to the student's current document topic exceeds threshold → intercept and rewrite as a question. **Exception: 「别处示例」** — concrete examples drawn from *another domain*, which cannot be pasted.
2. **锚定学生文本.** Every AI question must quote a specific fragment of the student's own document, argument map, or log, with a highlight backlink. Unanchored questions are **defects, not weak questions**: *"你觉得还有别的角度吗？"* enters the **违规话术库** and is used as a regression test.
3. **一次一个问题 + 进度可见.** One question per AI message. The station shows gate progress ("本站门禁 3 项，已过 1").
4. **言语化门控.** Three things must be typed by the student's own hand: **warrant · steelman · each source's 「作用与风险」**. Enforcement: those fields reject pasted AI message text. (Pasting one's own notes or source quotations is allowed; quotation marks required.)
5. **起草期延迟反馈，交稿前开放迭代.** In S5 drafting mode the AI makes **no** per-sentence comment — this prevents the "write a sentence, beg for approval" dependency loop. The student triggers **整稿体检** manually. The review says *which paragraph is submitting evidence to which table, and what is missing*. It never rewrites a sentence.
6. **三键处置皆留痕.** Every AI suggestion carries **accept / reject / rewrite**; any choice requires a written reason (≥15 chars). `rewrite` means *the student rewrites*. The reason text is the primary evidence source for D5 and for thinking leaps T3/T5.

### 7.2 Behavior requirements

- **R-1. The AI never generates essay content.** Enforced at the system level (§15.3), not by prompt convention. See RL-1, RL-4.
- **R-2. No unanchored interventions.** Anchored to an artifact, gate item, or span. Generic advice is a defect (law 2).
- **R-3. Rubric-tagged.** Every intervention carries the criterion it serves, in the board's own vocabulary.
- **R-4. AI-initiated surfacing is state-triggered, not message-triggered.** The coach surfaces a card because of workspace state, never because the student asked in chat. **Student-initiated invocation via 装备栏 is not merely permitted — it is the behavior the product exists to produce**, and is the primary signal for R-6 and the north star (DEC-10).
- **R-5. Anti-checklist-theater.** A source evaluation is incomplete until the student answers "does this change how you use it?" Mechanical tool completion without consequence for the argument is flagged, not rewarded.
- **R-6. Scaffolding fade.** Per-card, per-student. Early: direct questions. Then: "what should you be asking yourself here?" before revealing the card. Eventually: intervene only on missed critical checks. Fade consumes the 自发/提示后 signal (§14.1) and is shared with the Course module (§15.2). Cold start solved by DEC-5.
- **R-7. Intervention quality is an evaluated system.** Each card ships with graded good/bad instantiation examples, authored with examiner-experienced teachers, plus the **违规话术库** of forbidden phrasings. Both are standing regression suites (§18).
- **R-8. No global chat.** Conversation exists only in the context of a selected artifact or station.
- **R-9. The card template is an input to the AI, never a form for the student.** The AI applies the card's dimensions to the student's actual material and produces concrete, span-anchored questions, and invites the student to raise questions from angles the card does not cover. The card's abstract framework is revealed only at **consolidation**, after use, as the transferable takeaway.

---

## 8. The Coach

### 8.1 An orchestrator, not a chatbot

> **The station is the map** (where I am, what this stage demands). **The card is the rails** (how to think about one thing). **The coach is the driver** (which card, when, entering at which stage, how hard to push).

The coach's entire tool surface:

| Tool | Purpose |
|---|---|
| `surface_card(card_id, target, entry_stage)` | Run a staged thinking framework. Entry stage derived from gate-item state, not chosen freely. |
| `post_intervention(anchor, criterion, body)` | One anchored, rubric-tagged question or diagnosis. Typed and constrained (§15.3). |
| `check_gate(station)` | Evaluate gate items; report what is missing. |
| `route_to_course(card_id)` | Send the student to a remediation card practice when stuck (§8.5, §15.2). |
| `invite_commit(gate_item)` | Ask for a Draft Snapshot of a specific section. |
| `order_review(snapshot_id)` | Run 整稿体检 on a snapshot (§12). |

The coach decides *which tool, on which target, now* by reading workspace state — the **State Evaluator** (§15.1). It emits **exactly one next action** (law 3).

### 8.2 The four views

The center adopts one of four views. **A new card is a sequence over these four, never new UI code** (§15.4).

| View | Shows | Station default |
|---|---|---|
| **结构 `structure`** | Toulmin argument map ⇄ outline. **孤儿证据** (evidence connected to no claim) and **裸主张** (claims with no evidence) are always visibly flagged. Map nodes ⇄ draft paragraphs bidirectionally anchored. | S1, S4 |
| **素材 `material`** | A source text with span highlights, margin questions, student annotations; the source dossier; 兔子洞日志. | S2, S3 |
| **写作 `writing`** | The draft, in **edit** (silent) or **preview** (annotated) mode. §9. | S5 |
| **评估 `review`** | The readiness gauge (§11), rubric translation table, whole-draft review results, reflection pack. | S0, S6 |

Two things are deliberately **not** views: **the coach** (right rail, always present, always asking) and **consolidation** (R-9's framework reveal — an overlay at card completion, not a destination).

The active station sets the default view; **the student may switch freely at any time.** The gate governs *progression*, not *looking*.

### 8.3 装备栏 — the equipment rail (DEC-10)

Beneath the coach, on the right: **tool cards as clickable buttons.** Clicking expands the scaffold and writes an event.

- Each card shows the student's proficiency: **自发调用 vs 提示后调用**.
- **Spontaneity rule:** an invocation is **自发** if the AI has not mentioned that card within the preceding **3 turns**; otherwise **提示后**.
- This is the instrumentation the north-star metric requires (§14). Without a student-invocable rail, "unprompted critical-thinking action" cannot occur and cannot be measured.

Rationale for reversing v2 here: the coach's job is to make itself unnecessary. R-6's endpoint — the student acts without being asked — needs an affordance for acting without being asked.

### 8.4 Card stages: elicit → interrogate → challenge

A card declares ordered stages; each stage declares a view, a goal, and AI behavior. **Entry stage is derived from the gate item's state**, not chosen:

| Gate item state | Enters at | Coach behavior |
|---|---|---|
| `empty` | **Elicit** | Question the student into producing the artifact, in their own words. |
| `draft` | **Interrogate** | Anchor onto what they wrote and test it. |
| `flagged-weak` | **Challenge** | Sharper interrogation of the specific weakness. |
| `solid` | *(do not surface)* | Except spot-checks under fade (R-6). |

Cards requiring external material — SIFT×CRAAP on a source, concession-check on a snapshot — have no elicit stage and do not surface until material exists.

**Two guards, both load-bearing:**
- **Elicit must never generate content (RL-1).** "Who would disagree, and what's their best reason?" is coaching. "For example, you could argue that…" is writing the essay. No suggested counterclaims, no candidate examples, no menus to pick from. The student's answer *is* the artifact. The 保真层 validator is tuned hardest here.
- **Elicit is not a form (R-9).** Elicit happens in the coach rail, one question at a time. Four boxes labelled Claim / Counterclaim / Evidence / Implication is a worksheet.

### 8.5 卡壳救援三级 — the stuck rescue ladder

1. **Rephrase** the question.
2. **Cross-domain analogy** (「别处示例」) — a concrete example from another field, unusable as text.
3. **Two directions**, student picks one and states why.

All three exhausted → route to a Course card practice (§15.2), or flag for the teacher. Routing out is **conservative**: only on repeated weakness on the same `card_id`, or explicit student request. Essay projects run for weeks; momentum is the scarce resource, and an AI that offers a lesson at every stumble is an interruption machine. The student always returns to the exact artifact they left.

### 8.6 AI 验货挑战 — the quality-check challenge

The AI occasionally emits a suggestion explicitly labelled **「待质检」** containing an identifiable defect (over-generalization, source-tier misjudgment, unsupported causal claim). The student must catch and explain it. Catch rate feeds the autonomy panel (§14.3) and thinking leap T7.

This trains the competence the whole platform exists for: *working with AI without deferring to it.* Frequency is teacher-controlled, low by default.

---

## 9. The writing surface (DEC-6)

The Studio **recommends the student use their own editor** — Google Docs, Word, whatever they already write in — and gives them no reason to prefer ours. They may also write or paste here. Both routes produce **identical Draft Snapshots**.

- **Edit — silent.** A plain buffer: paragraphs and headings, nothing else. **The coach says nothing while the student writes** (law 5). Live commentary trains score-chasing prose.
- **Preview — where the coach speaks.** Read-only, span-anchored margin questions and diagnoses.
- **Committing to preview mints an immutable Draft Snapshot.** Pasting from outside mints the same object. The diff sequence is the version history feeding the Process Record.

> **The AI has no write access to the buffer. Ever.** No insert, no "apply suggestion," no "rewrite this," no autocomplete, no ghost text. Since an editable field now exists, RL-1 is an **enforced rule, not an architectural fact** — see §15.3. **Inline AI suggestion is the specific way this dies**, because nobody experiences it as "the AI wrote my essay." It is exactly that.

**We will lose a feature comparison against Google Docs.** Accept it. The answer to that gap is always better import/export, never a better editor.

**Word budget.** Parameterized per qualification (their 表一). Deletion decisions are made **in rubric language**: *"Table B currently has one paragraph of evidence, worth 5 marks. Which table do these 300 words of background serve?"* The word guillotine is real: TOK stops at 1,600; EE stops reading at word 4,001 — and **Criterion D, the heaviest at 8 marks, lives at the end.**

---

## 10. Station scripts

Unified fields: entry condition / AI opener / anchored question bank / gate adjudication / prohibited behavior. Two are given in full; the rest live in the sprint packs.

### S3 信源评估 — worked example

Each selected source gets one evaluation card: CRAAP five + pyramid tier + a written **「作用与风险」**. Key claims trigger lateral reading. AP paths add credibility-phrase and old-data checks.

**Gate:** 机判 — 100% card completion, ≥1 lateral read on key claims. 师判 — spot-check evaluation quality (guards against template praise).

> **AI:** 你把这篇《中国可再生能源装机量全球第一》标为核心证据。CRAAP 卡「A 权威性」你打了高分，理由是「大网站」。这条数据往上追，原始出处是谁？
> **学生:** 好像是转载的……我找一下。原始是国家能源局年度报告。
> **AI:** 好。那「大网站」和「国家能源局报告」哪个该进评估卡？
> **学生:** 能源局，官方一手数据。
> **AI:** 一手数据解决「数字准不准」。你的论证要用它证明「更可持续」——装机量大，等于更可持续吗？中间还差什么？
> **学生:** ……差实际发电占比？装了不等于用得多。
> **AI:** 这句话值得进「作用与风险」栏。现在用横向阅读卡：找一条**不是**中国官方口径的来源，看它怎么谈装机与消纳的差距。

*(Event stream: `source_opened` ×2, `card_clicked[横向阅读]`, `verbalization_submitted[作用与风险]` — D3 自发到位 candidate evidence.)*

### S4 论证构建 — the hardest gate

**Gate, four parts:** ① no orphan evidence, no naked claims (机判) ② every backbone warrant verbalized (机判 existence + 师判 quality) ③ **steelman test** passed — the student writes the strongest version of the opposing case, the AI cross-examines once *in the persona of a reader who holds it*, the student responds ④ the concession paragraph has a node on the map.

**Prohibited:** the AI never writes the steelman. Its counter-questions must draw on sources the student has already collected, or common knowledge. **It never fabricates data.**

> **AI:** 你的核心主张是「中国的绿色转型让地球更可持续」。换你坐到反方席：一个真诚认为相反结论的人，手里**最硬**的一张牌是什么？注意，不是最容易反驳的那张。
> **学生:** 煤电还在新增？
> **AI:** 这是一张牌。你日志里那篇讲「新增煤电作为调峰备用」的来源，反方会怎么用它、你打算怎么接？
> **学生:** 反方会说：一边装风电一边批煤电，说明转型是叠加不是替代，总排放还在涨。我的回应……看人均和趋势拐点？
> **AI:** 两句都写下来——反方版本进钢人段，你的回应进让步段。最后一问：如果三年后数据表明煤电不是「备用」而是持续满发，你的判断会怎么变？
> **学生:** 那结论就得从「正在让地球更可持续」降级为「有转型意愿但尚未兑现」。

*(A **conditional reversal statement** — top-grade evidence for thinking leap T4.)*

### S2 视角与素材 — the strictest retrieval rules

**Flow:** student submits a **search plan** → AI questions the plan → **the student searches, themselves** → every opened source is auto-logged (URL, dwell time, one-line takeaway) → the AI advises on *search direction only*, from the log.

**Hard rules (RL-2):** the AI supplies no ready-made material, paraphrases no source the student has not opened, and generates **no bibliography entry**. Citations can only be added from sources actually opened in the log.

---

## 11. The readiness gauge — five regimes

**RL-3 governs: never a predicted grade.** The gauge shows where the draft sits against official descriptors.

| Regime | Used by | Character | Gauge form |
|---|---|---|---|
| 逐表点分制 | 0457 (eight tables) | Independent tables; **numerals are hard thresholds**; positive marks start at "identify" | Eight-lamp panel + count-ups ("评估点 3/4") |
| AO-方面网格制 | 9239 C2/C4; EPQ (all boards) | Per-AO aspects/bands; descriptors ladder (upper band presumes all of the lower) | Aspect lamps + band-language slider ("你在『not always consistently』还是『embedded throughout』？") |
| 行-定点制（含二值行） | AP Seminar IRR / IWA / TMP | Each row has 2–4 fixed points; **some rows are 0/5 binary** | Switches + step selector. **A binary row is a switch, not a slider** — "达标/未达标 + 缺什么" |
| 整体判档制 | AP Research paper | Single 1–5; six attributes must cross together | Band-portrait comparison: 3 vs 4 vs 5 side by side on the same attribute |
| 全球印象制 | TOK Essay | One driving question, five bands, **adverbs decide the band** | Single needle + adverb-ladder circling (sustained / specific / effectively — how many are present?) |

This table is the engineering basis for **"one core, seven skins"**: the gate engine and evidence logic are shared; only the gauge renderer switches. It also corrects v2's config claim — see §15.4.

---

## 12. 整稿体检 — whole-draft review

- Triggered **by the student**, on a Draft Snapshot. Never ambient (law 5).
- Returns: (a) paragraph ⇄ rubric-table mapping — *which paragraph submits evidence to which table, and what is missing*; (b) word budget; (c) presence of intermediate judgments; (d) citation double-match.
- **Never rewrites a sentence.** Rewriting advice appears only as: keep as written / student revises / explain why not — and the student states a reason (law 6).
- **One snapshot, one review. A new review requires a new snapshot.** Revision is the price of the next review. Prevents feedback farming.
- **考官人设 (examiner personas):** switchable. Qualification-specific (corpus = that board's examiner-report stock phrases — AQA's "does the log show the research journey and its decisions"; OCR's "the one or two missed opportunities") or three generic: 苛刻的怀疑者 / 善意的外行 / 词数刽子手.
- Qualification modes: EE runs the **evaluation-density scan** (evaluative sentences per paragraph; evaluation crowded into the final paragraph raises an alarm). AP runs the **organization × connection dual diagnostic** and the attribution-phrase scan (unanchored "研究表明……" flagged one by one).

---

## 13. Process record, integrity, and compliance

- The Process Record is generated from artifact version history and the event stream: position revisions, counterclaims added after challenge, source evaluations that changed how a source was used, snapshot diffs, mid-project Course detours.
- **The thinking record, not the prose history, carries the evidentiary weight.** This is why the product can decline to own the student's editor without weakening its integrity claim. Keystroke history proves little — AI prose pastes into any editor. The only editor-ownership that would add forensic power is **surveillance**, which this product refuses: it is formative, not forensic.
- **Mid-project learning detours are the most valuable evidence in the file.** "Got stuck on counterclaims → learned the tool → came back and revised" is precisely the narrative EE Criterion E and the 9239 research log award marks for. Capture the round trip.
- Process metrics measure **thinking-quality signals** (position revised after counter-evidence; evaluation altered source usage), never activity counts (cards opened, notes written). Activity counts are gameable and pedagogically meaningless.
- **Export forks by qualification** (their 表二): TOK PPF interaction notes · EE RPF material + viva voce briefing · 9239 research log (the only component where the log is **directly worth 10 marks**) · EPQ Production Log / Activity Log / PPR · AP PREP material + checkpoint preparation.
- **AI 使用申报单.** Auto-summarized from the ledger — interaction types, frequency, dispositions — and **signed by the student**. This is the artifact IB / JCQ / AP Capstone policy actually asks for.
- **RL-4 enforcement point:** the reflection editor is **completely read-only to the AI.** The AI may question what the student has written. It may not supply any pasteable text.

---

## 14. Assessment and the Growth Report

### 14.1 Event stream

| Event | Key fields | Feeds |
|---|---|---|
| `prompt_sent` | text, station, quoted fragment | six-dimension text annotation |
| `card_clicked` | card, **自发 / 提示后** (AI mentioned it within prior 3 turns?) | initiative gradient, tool proficiency, R-6 fade |
| `gate_attempt` | station, result (pass / fail / rework), missing items | D1 D4; backflow events |
| `suggestion_disposition` | accept / reject / rewrite, reason text | D5; T3 T5 |
| `verbalization_submitted` | type (warrant / steelman / 作用与风险), text | D2 D3 D4 quality |
| `source_opened` / `citation_added` | URL, dwell time, source tier, lateral-read triggered | D3; the ledger |
| `version_saved` | snapshot id, diff summary | argument-map evolution |
| `rescue_triggered` | level, did the gate pass afterwards | D2 (dependence vs leverage) |
| `stance_change_logged` | old stance, new stance, student's attribution text | T2 T4 |

### 14.2 思维跳跃 T1–T7 — the report's protagonist

| # | Name | Trigger signal |
|---|---|---|
| T1 | 主动性跃迁 | a card previously only 提示后 is invoked 自发 for the first time |
| T2 | 立场修正 | `stance_change_logged` with attribution to a specific source or event |
| T3 | 主体性宣示 | `disposition = reject` with a substantive argued reason |
| T4 | 反方拥抱 | steelman passed first attempt, or a conditional-reversal statement written |
| T5 | 自我识破 | the student names a flaw in their own argument or source **before any AI prompt** |
| T6 | 工具迁移 | a card invoked 自发 in a context different from where it was taught |
| T7 | 边界觉察 | the student declares a task should not be given to AI, or catches a 验货 defect |

**Each leap = one timestamped before/after card of the student's own words.** The report's front page is the leap timeline.

> 没有跳跃不硬凑 —— 「本周期未见跳跃」如实呈现，并给下一步建议。**报告的信用来自它敢于空着。**

T1 and T5 *are* the north-star metric, operationalized. §14.1's spontaneity rule is what makes them computable — instrument it from day one.

### 14.3 深 × 自由四象限

- **Vertical — 概念深度:** six-dimension total, mapped to SOLO L1–L4.
- **Horizontal — 批判自主指数:** weighted from reject+rewrite share, verbatim-adoption rate (inverted), steelman first-pass rate, evidence-based stance revisions (T2), 验货 catch rate. Initial weights 0.25 / 0.25 / 0.2 / 0.15 / 0.15, iterated against calibration samples.
- **Upper-left quadrant — 「精致的囚徒」 (deep but not autonomous):** says elegant things, accepts every AI suggestion. Triggers teacher intervention advice (raise 验货 frequency, add steelman practice).
- **Red line: the two axes are never combined into a single score.** Narratives to parents lead with the leap timeline; the quadrant is secondary.

### 14.4 Report versions (DEC-2, revised)

- **Student:** leap timeline (front page) · argument-map evolution · six-dimension radar with representative evidence · 自发 vs 提示后 · tool proficiency · **exam-view mapping** ("your S3 work most resembles 0457 Table E's 7–8 descriptor" — rubric language, never a score) · two next actions.
- **Teacher:** all of the above + risk zone (integrity flags, dependency patterns, 精致的囚徒 alert) + class cross-section.
- **Archive / declaration:** process-document material pack + AI usage declaration, for submission to school or board.

**Compliance restated:** the pack supplies material for the student to write official documents. The platform drafts no scored reflective text (RL-4).

### 14.5 North-star metric

**Unprompted critical-thinking action rate** — frequency of source evaluation / counterclaim construction performed with no card surfaced beforehand. Computed from `card_clicked[自发]` and T1/T5.

- Measured **within-project** (second half vs first half) and **across Course card practices** via shared `card_competence` (§15.2). Because bidirectional routing means practices are visited organically mid-project, the two reinforce each other.
- Cross-project trend becomes a lagging metric once students hold two projects.
- **Guardrails:** AI-generated essay content rate = 0 (audited); student-initiated questions per session trending up; interventions rated "generic" in QA sampling < 5%.

---

## 15. Architecture

### 15.1 The State Evaluator — one decision engine

Law 3 (one next action), R-4 (state-triggered surfacing), R-6 (fade) and `check_gate` are one computation.

- **Input:** gate-item states across S0–S6; the artifact graph (nodes, links, versions); per-`card_id` competence/fade state; project metadata (qualification, title, deadline, calendar).
- **Output:** exactly one `next_action`, expressed as one of the coach's six tools (§8.1); zero or more anchored, rubric-tagged, typed interventions. When surfacing a card, **entry stage is derived from gate-item state, never chosen freely** (§8.4).
- **Trigger:** workspace state change (artifact created / edited / linked / committed), never a chat message.
- **Model routing:** cheap model for state detection and trigger predicates; flagship for authoring intervention text and for evaluation. Evaluation never downgrades.

### 15.2 Card contract shared with Course

- **Shared:** the card registry (pedagogical intent, trigger predicates, AI behavior guidance, rubric tags, per-qualification vocabulary) and **per-student per-card competence/fade state**.
- **Not shared:** rendering runtimes, layout, navigation. Studio and Course ship independently.
- **`card_id` is the routing key.** There is no separate skill taxonomy (DEC-7).
- **Both directions:** Course → Studio (a card practised arrives with real competence state; fade cold start solved). Studio → Course (stuck → the card practice that teaches it → return to the exact artifact).
- **Seam:** `resolve_remediation(card_id) → { deep_link, estimated_minutes } | null`. `null` (no course teaches this card) falls back to coaching.
- **Confirmed by 论文板块 §3.1:** courses X1–X12 each declare 核心工具卡, and Appendix A maps station × card × dimension × qualification. **Courses are composed of card practices.** DEC-7 is no longer provisional.

### 15.3 RL-1 / R-1 enforcement — the highest-severity invariant

An edit buffer now exists (§9), so this is enforced, not architectural.

1. **Typed output only.** `{ type: "question" | "diagnostic" | "reference", anchor_id, span?, criterion, body }`. **No output type can carry insertable prose.** A `reference` may only quote the student's own prior artifact, verbatim, with provenance.
2. **The AI has no write path to the edit buffer.** Forbidden at the UI layer and in code review: insert, "apply suggestion," "rewrite this," autocomplete, ghost text, inline completion.
3. **保真层 validator.** Declarative output + semantic similarity to the student's current topic above threshold → intercept, rewrite as a question. **「别处示例」 exception:** concrete examples from another domain pass, because they cannot be pasted.
4. **违规话术库.** A regression suite of forbidden phrasings (unanchored questions, suggested counterclaims, candidate examples). A question that could be asked of any essay fails; a question that could only be asked of *this* paragraph passes.
5. **Audit log.** 100% of interventions persisted with type, anchor, criterion, validator verdict. The guardrail metric is computed from QA sampling of this log.
6. **RL-4 point:** the reflection editor is read-only to the AI.

### 15.4 Configuration contract — corrected

v2 claimed "a new qualification = zero code." **§11 falsifies that:** a 0/5 binary row is not a slider with two stops.

> **Within an existing scoring regime**, a new qualification requires only config — checklist/gate parameters, card vocabulary localization, criteria file, calendar, recognition content — and **zero application code**.
> **A new regime** requires a new gauge renderer. There are five (§11). Build them as five renderers behind one interface; config declares which regime it uses.

**Card criterion (unchanged):** adding a card requires only a config file declaring target, stages, per-stage view and AI behavior — where every `view` is one of the four. **Zero new UI components.** A card that appears to need a fifth view is a design conversation, not a config change.

### 15.5 Data model

`project` · `station` · `gate_item` (state, judge type) · `artifact` (typed, versioned) · `draft_snapshot` (immutable, span-indexed) · `edit_buffer` (scratch, not a record) · `material` (span index) · `source_log_entry` (URL, dwell, tier, takeaway, lateral-read flag) · `card_instance` (anchors: span, author=AI|student, dimension, question, answer; `summary_fill` at consolidation) · `intervention` (typed, anchored, criterion, validator verdict) · `disposition` (accept/reject/rewrite, reason text) · `card_competence` (per student per `card_id`, shared with Course) · `event` (§14.1) · `process_record` (projection).

**Config (static, versioned):** card registry · gate parameters per qualification · criteria files · gauge regime declaration · recognition content · sprint packs.

---

## 16. First entry and returning

### 16.1 First entry (no project yet)

A single input: **"Paste your title or question."** No dashboard, no feature tour.

The system must answer within seconds, with recognition value, *before asking the student to do anything*:

> "This is Title 3, May 2027 session. Students writing this title most often stumble on treating 'evidence' as self-explanatory. Ready to take it apart?"

**Acceptance bar: the first ten seconds must convey "this product knows my exact assignment," not "this is an AI tool."**

Recognition content is hand-authored for prescribed titles and AI-composed from criteria for self-devised ones. **Note the cost of the Cambridge-first MVP (DEC-9): TOK is the only one of the seven with prescribed titles.** In 0457 the student devises a research question from a 22-topic list; in GPR entirely. Cambridge-first means the long-tail recognition path must work on day one. This is the accepted trade for a working readiness gauge.

### 16.2 Returning session

One sentence of situated memory, not a menu:

> "Last time you were halfway through the counterclaim for History and got stuck finding an example. Pick up there?"

One primary CTA. Station progress visible behind it. Deadline-aware nudging tied to the exam calendar.

---

## 17. v1 scope and MVP (DEC-9)

**Phase 1 (~3 months) — Cambridge double pack + Studio core.**
- 0457 Individual Report sprint pack + GPR 9239 Essay sprint pack.
- Studio core: S0–S6, gate engine, dual view (文档 ⇄ 论证图), the ledger, 三键处置, 装备栏.
- Lead courses X1 拆题术 / X2 好问题的诞生 / X6 论证的解剖 / X7 钢人与让步.
- Growth Report 2.0: leap timeline + exam-view mapping.

**Why Cambridge, not TOK:** 0457's eight tables and 9239's AO-aspect grid are the only two of seven with **fully public per-descriptor level statements** — the only qualifications where the readiness gauge can operate at per-descriptor fidelity on day one. Moderate word counts. The existing anchor sample (「中国是否让地球更可持续」) is already a GP-style question, so calibration samples migrate at zero cost. TOK's global-impression regime yields a single needle — the weakest possible first demonstration of the gauge. **Accepted cost:** the §16.1 recognition moment is weakest here (self-devised titles).

**Phase 2 (+3 months) — long-cycle pair.** EE sprint pack (new/old dual track, timeline engine, RPF material pack, evaluation-density scan) and EPQ (three-board document forks, log engine, band slider), sharing the "long cycle + process-scored" timeline infrastructure. TOK flagship pack (6-title workbench refreshed each session, band-training simulator, **考季题目校验器** — answering a previous session's title scores 0).

**Phase 3 (+3 months) — AP double pack and the compliance showcase.** AP Seminar (stimulus workbench, binary-row switch panel, checkpoint simulation) and AP Research (band portraits, alignment checker, POD seven-row simulation). AP's AI policy is the most explicit of the seven; once the compliance mode is proven here it becomes the market statement: **"an AI writing tool that dares to enter an AP classroom."**

**Data actions:** one golden-sample journey per qualification, walked end to end internally → benchmark (100–300 annotated real dialogues in the essay context) → teacher-correction reflow.

**Risks:** compliance → compliance mode + declaration + RL-3/RL-4; annual August review of AP policy pages, JCQ guidance, IB academic integrity policy. Hallucinated citations → S2 hard rules. ESL output → bilingual scaffolding (thinking may be in Chinese; **output language = exam language**) + translationese contrast practice. Over-promising → "readiness ≠ predicted grade" written into copy and sales scripts. Report inflation → axes never combined, leap narrative first, the report dares to be blank.

---

## 18. Standing curriculum operations

Recurring workstreams with hard external deadlines. Staff them explicitly; they are not one-time authoring tasks.

1. **The card library across the essay lifecycle.** The single largest workstream and the one most likely to be underestimated. Source evaluation is one card among many; every station needs cards, each with elicit and interrogate behaviors authored separately. **Card quality is product quality** — a generic question fails R-7 no matter how good the engine is.
2. **Card golden sets + the 违规话术库** (R-7): graded good/bad instantiations **per card, per stage**, plus forbidden phrasings, authored with examiner-experienced teachers. Scales with (1).
3. **Recognition content** (§16.1), refreshed per session. TOK's six titles publish early September.
4. **Rubric asset maintenance.** The seven-qualification official archive is a moat only while current: annual AP scoring guidelines, EE parallel-period end (old track retires after 2026-11), 9239's next version (from 2029), TOK guide movements.
5. **Exemplar recalibration** for the readiness gauge, per session as examiner reports and grade thresholds publish.

---

## 19. Decisions

**DEC-1 — Argument map: Toulmin-typed nodes, slot-bound to gate items, with a freeform scratch area.**
Orphan evidence and naked claims are always visible. Slots give F1 traceability and legibility for weaker students; scratch preserves expressiveness; nodes are promoted from scratch into slots.

**DEC-2 — A teacher report ships in v1.** *(Reversal R-B.)* Read-only report version + risk zone + class cross-section + teacher controls (验货 frequency) + 师判 gate items. Their gate model requires teacher judgment at nearly every station, so the teacher surface is not deferrable. Full grading workflow and notifications remain out of scope.

**DEC-3 — Automated state assessment may never mark a gate item `solid`.**
A false `solid` (telling a student they are done when they are not) is far more harmful than a false `empty`. Machine judgment may set `draft` or `flagged-weak`; `solid` requires a passed challenge or explicit confirmation.

**DEC-4 — Never a number. Bands, evidence, and the nearest exemplar.** *(= RL-3.)*
Point scores under holistic marking are dishonest. Output is a descriptor position and a work order.

**DEC-5 — Fade cold start solved by shared per-card competence.**
Signals transfer from Course via `card_competence` keyed by `card_id`. No Course history for a card → fade starts at the most scaffolded level.

**DEC-6 — A minimal, plain `写作` view with silent edit and speaking preview. We recommend the student's own editor.**
Committing to preview mints an immutable Draft Snapshot identical to a paste. RL-1 and law 5 become *enforced rules* rather than architectural facts; **inline AI suggestion is the specific failure mode.**

**DEC-7 — No skill taxonomy. The card is the atom; `card_id` is the routing key.** *(No longer provisional — confirmed by 论文板块 §3.1.)*
Reintroduction condition: SIFT and CRAAP both shipping with transfer desired between them. That is a registry field, not schema surgery. The failure mode of deferring is *over*-scaffolding, which annoys; under-scaffolding harms.

**DEC-8 — The S0–S6 journey is gated.** *(Reversal R-A.)*
A gate must be passed to unlock the next station. **Reverse entry is removed.** 回流 (backflow) remains and is recorded as 有据修正, never failure. Revisit if reverse entry proves to be the dominant real-world entry.

**DEC-9 — Cambridge double pack first (0457 + GPR 9239), not TOK.** *(Reversal of v2 §12.)*
Only these two publish per-descriptor level statements, so only these two let the readiness gauge run at full fidelity on day one. Accepted cost: the recognition moment (§16.1) is weakest where titles are self-devised, and TOK is the only qualification that prescribes them.

**DEC-10 — Cards are student-invocable via 装备栏.** *(Reversal R-C — a bug fix.)*
R-4 governs AI-initiated surfacing only. Without a student-invocable rail, "unprompted critical-thinking action" cannot occur, so the north-star metric and R-6's fade endpoint were both unmeasurable. Spontaneity rule: 自发 if the AI has not mentioned the card within the preceding 3 turns.

**DEC-11 — Zero-code config holds within a scoring regime, not across regimes.**
Five regimes (§11), five gauge renderers behind one interface. A new qualification inside an existing regime is config only.

---

## 20. Remaining open questions

1. **Does removing reverse entry (DEC-8) survive contact with real students?** The most likely first session is a student who already has 800 words. Instrument the first-session entry point and watch.
2. What is the re-entry experience after a mid-project Course detour — straight back to the artifact, or a one-line "here's what you just learned, now apply it" bridge? The bridge is probably worth it, but must not read as a quiz.
3. What is the smallest commit the review engine can say something useful about — a paragraph, or does diagnosis need surrounding context? This sets the invitation copy on the gate.
4. What is the *minimum* recognition content that clears §16.1's bar for a self-devised 0457 / GPR question, where nothing is hand-authored? **This is now on the critical path because of DEC-9.**
5. Does the Chinese coaching toggle apply to interventions only, or also to gate items and rubric vocabulary? (Rubric vocabulary is the board's own; translating it may harm transfer to the exam.)
6. Retention/deletion policy for the Process Record, which is simultaneously integrity evidence and student personal data.

---

## 21. Handoff

### For the designer — in this order

1. **First-entry recognition moment** (§16.1). The ten seconds that decide whether this feels like "it knows my assignment" or "another AI tool."
2. **The four views** (§8.2) — 结构 / 素材 / 写作 / 评估. Every card that will ever exist is a sequence over these four. Design them as a set, and design the transitions.
3. **装备栏 + coach rail** (§8.3). Two things live on the right: what the coach asks (AI-initiated) and what the student can reach for (self-initiated). The 自发/提示后 proficiency signal must be visible to the student without becoming a score.
4. **`写作`'s two modes** (§9). **Edit** must feel calm and empty — no AI presence, no suggestion affordance, nothing to accept. **Preview** is where the margin fills. The transition is the moment the student submits prose to scrutiny; make it feel like that.
5. **The 结构 view** (§8.2): Toulmin map ⇄ outline, with **孤儿证据** and **裸主张** always visibly flagged. This is van Gelder's ≈0.8 SD effect, productized.
6. **A card running end to end** (§8.4): elicit in the coach rail (one question at a time, never a form) → place it → challenge → the **consolidation overlay**. It must feel like one coach moving with you, not four features stitched together. Pedagogical heart; most iterations.
7. **The gate** (§5): station progress ("本站门禁 3 项，已过 1"), what a failed gate looks like (a work order, never a scold), and what 回流 looks like (a positive event, marked 有据修正).
8. **The readiness gauge, five regimes** (§11). Start with 逐表点分制 (0457's eight lamps + count-ups) — it ships first. A binary row is a **switch**, not a slider.
9. **三键处置** (law 6) and the **验货挑战** (§8.6) — the two interactions that teach AI-collaboration literacy.
10. **Growth Report** (§14): the leap timeline is the front page. Design what "本周期未见跳跃" looks like — the report must be dignified when it is blank.

### For the coder — in this order

1. **State Evaluator** (§15.1) — one engine for next-action, gate checking, card surfacing, fade. Instrument the 自发/提示后 signal (§14.1) from day one, or §14.5 cannot be computed at all.
2. **Card runtime = a stage machine over the four views** (§8.4). Cards are config. Acceptance test: a new card ships as a config file with **zero new UI components**.
3. **RL-1 enforcement** (§15.3) — highest-severity invariant. Typed output; **no write path to the edit buffer**; 保真层 validator; 违规话术库 regression suite; audit log. Tune hardest against the elicit stage.
4. **Gate engine** (§5) with 机判 / 言语化 / 师判 item types, and the paste-blocking enforcement of law 4.
5. **Event stream + ledger** (§14.1). Everything downstream — leaps, quadrant, report, declaration — is a projection of this.
6. **Card contract + registry**, shared with Course; `card_competence`; `resolve_remediation(card_id)` behind an interface, handling `null`.
7. **`写作` view**: silent edit buffer (client-owned, no model access) + preview renderer. Commit mints an immutable Draft Snapshot — identical whether written here or pasted.
8. **Gauge renderers**, five behind one interface (§15.4). Ship 逐表点分制 first.
9. **Search log subsystem** (§10, S2): search plan → opened-source auto-logging → citations addable *only* from the log (RL-2).
10. **Process Record projection + export forks + AI usage declaration** (§13).

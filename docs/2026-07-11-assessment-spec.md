# Product Spec: Assessment & Growth

| | |
|---|---|
| **Status** | Draft v1 |
| **Author** | Product |
| **Last updated** | July 2026 |
| **Scope** | The platform-wide assessment system: rubrics, the evaluation engine, tool-card signals, and the teacher dashboard. Spans all three surfaces — Course, Studio, and Chat. |
| **Source** | `思维印记_评估平台介绍(1).docx` (colleague's iterated version) |
| **Relates to** | `workbench-spec.md` (the Writing Studio's §14 is the essay-vertical view of this system) · `思维印记_论文板块产品设计文档_v3.md` |

---

## 0. Where this sits

Assessment is **not** a Studio feature. It is a platform-wide layer with one student/teacher/class model, one evaluation engine, one tool-card library, and one dashboard, shared across modules. **Swapping module means swapping rubric content, not swapping platform.**

That single fact drives the whole architecture: the Course, the Studio, and Chat all emit the same kind of evidence (student utterances + tool-card use + behavior), and the same engine scores it against whichever rubric the current module declares.

> Content layer (**what you practise**, the course library) → rubric (**what is measured**) → dashboard (**what the teacher sees and does**).

**One reconciliation is owed up front.** This document's rubric has **nine** dimensions for AI critical thinking; the essay vertical (Studio §14, from 论文板块) uses **six**. They overlap but are not the same set. §9 proposes how they relate and asks you to decide. Until then, treat the nine-dimension rubric here as the platform default and the six as the essay projection.

---

## 1. Three principles (unchanged since the platform's founding)

1. **Measure how you think, not what you know.** The AI already knows the content better than the student. What we measure is the *path*: given the same question, the same AI, the same sources, which different routes do different students take? That divergence is the real ability in the AI age.
2. **Make critical thinking measurable.** Cut the abstract idea into concrete, observable dimensions; grade each on four levels aligned to **SOLO** (§2), so a level has a defensible basis rather than a teacher's gut feeling. The system scores from behavioral signals; the teacher makes the final call.
3. **Tool cards are the visible scaffold, and the click is data.** We do not compete on "how smart the AI is" — a smarter AI makes a more passive student. We make the student actively reach for a way of thinking. Each dimension has one or more clickable **tool cards**, and the act of clicking is itself an assessment signal (§4).

---

## 2. The four levels (SOLO-aligned)

Every dimension is graded L1–L4, mapped one-to-one onto SOLO's prestructural → extended-abstract progression.

| Level | Meaning |
|---|---|
| **L1 Emerging** | prestructural |
| **L2 Developing** | uni/multistructural |
| **L3 Proficient** | relational |
| **L4 Excellent** | extended abstract |

The levels are defined by **observable behavior**, not by impression. Worked example, for the *source identification* dimension:

| Level | Observable behavior |
|---|---|
| L1 | Trusts AI output completely; never questions the source. |
| L2 | Occasionally asks "is that true?" but does not pursue it. |
| L3 | Actively asks for evidence; can rank source tiers. |
| L4 | Cross-verifies laterally on their own; spots the interests and conflicts *between* sources. |

Every dimension ships with a behavior ladder like this. The ladders are the executable definition of the rubric — they are what the engine (§5) and the teacher score against.

---

## 3. The rubric library

The platform began with two rubrics and grew into a full TOK engine. They share one engine, one dashboard, one tool-card library.

### 3.1 AI critical thinking — nine dimensions, input → process → output

Organized not by difficulty but by the **learning loop**: every interaction with AI is one complete cycle (take in → process → produce), isomorphic with the OECD Learning Compass's Anticipation–Action–Reflection.

| Layer | Dim | Core question | Framework anchor |
|---|---|---|---|
| **Input** | **CT-D1** Prompt clarity (提问清晰度) | Can they give the AI enough context? | ATL Thinking · QUEST-Q |
| Input | **CT-D2** Source identification (信源辨识) | Can they judge how trustworthy an AI answer is? | Media/information literacy · COR/MIL |
| Input | **CT-D3** Lateral verification (横向验证) | Can they cross-check across sources and trace to origin? | ATL Research · lateral reading (SHEG) |
| **Process** | **CT-D4** Perspectives & concession (多视角与让步) | Can they actively seek the opposing case? | QUEST-E · argument evaluation |
| Process | **CT-D5** Argument decomposition (论证拆解) | Can they see the claim–evidence structure? | QUEST-U · argument analysis |
| Process | **CT-D6** Reflection & metacognition (反思与元认知) | Can they notice they are being influenced by the AI? | ATL Reflection · TOK knower & knowing |
| **Output** | **CT-D7** Argument quality (论证质量) | Does the output meet a structural standard? | QUEST-S · ATL Communication |
| Output | **CT-D8** Information reproduction (信息再生产) | Do they distinguish AI's contribution from their own? | ATL media ethics · academic integrity |
| Output | **CT-D9** AI boundaries & ethics (AI 边界与伦理) | Can they see the AI's limits? | TOK knowledge & technology · ethical use |

### 3.2 OPCVL source evaluation — twelve dimensions

Built on the five OPCVL core dimensions, then extended outward in three layers.

| Layer | Dim | Dimension |
|---|---|---|
| **Core (OPCVL)** | HS-D1 | Origin (来源) |
| Core | HS-D2 | Purpose (目的) |
| Core | HS-D3 | Content (内容) |
| Core | HS-D4 | Value (价值) |
| Core | HS-D5 | Limitation (局限) |
| **Multi-source** | HS-D6 | Corroboration (对照) |
| Multi-source | HS-D7 | Contextualization (脉络) |
| Multi-source | HS-D8 | Empathy (共情) |
| **Reasoning** | HS-D9 | Causation & change (因果与变迁) |
| Reasoning | HS-D10 | Historical significance (历史意义) |
| **Output** | HS-D11 | Synthesis argument (综合论证) |
| Output | **HS-D12** | **Historiographical awareness (史观自觉)** |

**HS-D12 is the soul of the framework** — awareness of one's own position as the analyst — and maps directly onto TOK's core theme, "the knower and knowing." (The Studio's spot-the-flaw and reasoned-refusal signals are the AI-vertical analogue of the same self-awareness.)

### 3.3 AOK courses extend the rubric

The five Areas of Knowledge courses push measurement into each discipline's method: history via sources, science via evidence, mathematics via proof, human sciences via bounded science, the arts via warranted interpretation. Each course declares which dimensions it lights up.

### 3.4 One engine, many rubrics

A module is a rubric plus its courses on the shared rails. AI critical thinking (9), OPCVL (12), and — see §9 — the essay vertical are all rubrics addressing one engine, one dashboard, one tool-card library. Adding a rubric is content work, not platform work.

---

## 4. Tool cards are the measurement spine

This is the platform's most specific and most counterintuitive design. Every dimension carries one or more clickable **tool cards**. Clicking does two things:

1. **The AI switches mode** — from an assistant that gives answers to a scaffold that thinks *with* the student. Click the SIFT card and the AI walks the four lateral-check steps; click the concession card and the AI gives the strongest opposing argument instead of agreeing.
2. **The click itself is recorded as assessment data.** The system knows precisely: *this student used the SIFT card 14 times this week and the lateral-reading card zero times.* This behavioral signal is more reliable than semantic analysis of text.

About ten of the cards are **transferable meta-skill cards**, not bound to any subject: SIFT check, CRAAP, source pyramid, lateral reading, discourse analysis (CDA), concession paragraph, well-formed argument, PEE sandwich, rabbit-hole log, information-ethics checklist. They are reused freely across modules — the concession card learned in an AI-collaboration course is invoked directly in a history course or an argumentative essay. **That reuse is the literal definition of the IB ATL "transfer" skill, and it is what no single-subject product can offer.**

> **This is the same card system as the Studio and the Course** (workbench-spec DEC-7: the card is the atom, `card_id` routes across surfaces). The click-as-signal here *is* the Studio's `card_clicked` event with its 自发/提示后 tag (DEC-10). The two documents describe the same mechanism from two ends; build it once.

---

## 5. The evaluation engine and data strategy

The engine is a backend LLM system that, for each student utterance, produces a rubric annotation + tool-card recognition + behavior tags. Rollout is three steps, each lower-risk than a from-scratch model.

| Stage | Approach |
|---|---|
| **MVP** | Train nothing. Large model + few-shot prompting: five finely annotated **anchor samples** become the prompt's examples, and the model annotates new dialogues in the same format. Changing the rubric means changing the prompt — very fast iteration. |
| **Validation** | Build a human-annotated benchmark (100–300 real teacher–student dialogues). Every prompt change or model swap is run against it for accuracy, while teacher corrections accumulate. |
| **Scale** | Fine-tune a small model on the accumulated real annotations to cut cost and latency, keeping the data in a private deployment to meet school privacy requirements. |

**The anchor samples double as the executable definition of "good critical thinking":**

- **Phoebe** — a full L1→L4 growth spectrum.
- **Marcus** — passive, L1–L2.
- **Ethan** — the red-line opportunist: intent to have the work done for him, wrapped in rationalizing talk.
- **Eliza** — OPCVL L3→L4 with very high historiographical awareness; a cross-module transfer exemplar.

**The data sedimentation layer:** every dialogue, every card use, every teacher correction is recorded. This is the platform's long-term asset — training data for the model and material for education research. The system does not only record; it nudges, and it learns (each teacher correction makes it more accurate).

> These are the same anchor samples cited in the Studio spec (R-7 golden sets) and the same model-routing split (cheap model for detection, flagship for scoring, evaluation never downgraded). Consistent; build once.

---

## 6. The teacher dashboard — from observation to action

A K–12 teacher may manage 30–40 students and have minutes a day. So the dashboard has three layers on one principle: **the higher the layer, the more it focuses on *where the problem is*; the lower, the more on *what actually happened* — and every layer carries an AI-generated, executable next step.**

| Layer | The question it answers | What you see and do |
|---|---|---|
| **Class** | What happened in the class this week; what should next week's lesson cover? | AI insight bar (this week's shared weakness + a suggestion for the next lesson) · a 9-dimension × whole-class heatmap · a **risk column** (integrity red lines / missing method / needs a private word). One click: **generate a lesson outline.** |
| **Individual** | How is this student overall; where do they need a breakthrough? | A strengths-and-weaknesses teaching-advice bar · a rubric-level bar chart · a 14-week growth trajectory · tool-card usage preferences (the AI auto-links these to the student's weak dimensions). |
| **Single conversation** | What actually happened that one time? | A turn-by-turn replay (each turn tagged with dimension + tool card) · three auto-identified **highlight moments** (a metacognitive breakthrough is starred and weighted) · a three-action set: save a note / share with a parent / generate targeted practice. |

Every layer turns the dashboard from *observation* into *action*: the last two actions (share with parent, generate practice) are AI-triggered downstream tasks.

> This is the substance behind the Studio spec's DEC-2 (a read-only teacher report in v1). DEC-2 said *that* a teacher surface ships; this section says *what is on it*. The risk column and the 师判 gate items in the Studio are the same idea.

---

## 7. Academic alignment — complete TOK coverage (the moat)

An IB deputy head, an AP Capstone coordinator, or an A-Level coordinator asks one question: *which globally recognized competence frameworks do you align to — is there an academic basis, or a startup's inspiration?* This is the answer.

The core claim: we did not invent a new set of abilities. International curricula already wrote critical thinking into their frameworks; those abilities could only ever be assessed "by feel" from low-frequency signals — essays, presentations, discussions. We turned them, for the first time, into high-frequency, quantifiable, interveneable process signals. The strongest card is **complete TOK coverage**:

- **Practisable, measurable, complete TOK** — all five Areas of Knowledge (history via OPCVL, natural science, mathematics, human science, the arts), each with its own courses and measurable dimensions.
- **All four framework pillars** — scope · perspective · methods & tools · ethics — each mapped to courses and dimensions, using the current 2022 framework (not the removed Ways of Knowing).
- **Ethics as a cross-cutting dimension** — an ethics course × the ethical dimension of each AOK, exactly as current TOK treats ethics.
- **Reflexivity as the soul** — CT-D6 metacognition + OPCVL HS-D12 historiographical awareness + the "knower's perspective" course = the measurable landing point of TOK's core theme.

Alignment beyond TOK:

| Framework | Core skill it builds | Our mapping | Coverage |
|---|---|---|---|
| IB TOK 2022 | Knowledge framework + five AOKs + "the knower and knowing" | Five AOK courses + knowledge tools + ethics line + CT-D6 / HS-D12 | Strong (complete) |
| IB ATL | Thinking / research / communication / social / self-management | CT-D1–D9 hit thinking + research; tool cards = transfer | Strong |
| AP Capstone (QUEST) | Question → Understand → Evaluate → Synthesize → Team | Q↔D1, U↔D5, E↔D4, S↔D7/D11 | Strong |
| Historical thinking (SHEG / Seixas / AP) | Sourcing / contextualization / corroboration + causation/change/significance | OPCVL HS-D1–D12 item by item (incl. reasoning layer) | Strong |
| SOLO taxonomy | Grading thinking quality (prestructural → extended abstract) | The academic basis for L1–L4 | Strong |
| OECD Learning Compass 2030 | Anticipation–Action–Reflection + student agency | Isomorphic with the input–process–output loop; tool cards = agency | Strong |
| China core competencies (核心素养) | Critical questioning · information awareness · rational thinking · reflection | A localization for the public-school system (CT-D2/D3/D4/D5/D6) | Strong |

---

## 8. What makes it fundamentally different

| Dimension | Traditional AI tutor | Plagiarism detector | The Mark of Thinking |
|---|---|---|---|
| Core goal | Help the student get answers right | Catch ghostwriting | Keep the student thinking in the AI age |
| What is assessed | Answer accuracy | Text similarity | The thinking path and tool use |
| Value to the teacher | Less workload | After-the-fact punishment | Diagnosis + teaching decisions |
| Value to the student | Finish homework fast | Deterrence | A visible trajectory of growing ability |
| Stance toward AI | Dependence (stronger is better) | Neutral | A tool (AI is the training material) |

> One line: other products let the AI finish the work; we keep the student thinking in the AI age. That is the moat that is hard to copy.

---

## 9. Reconciliation with the Studio and 论文板块 specs

This is the section that needs your decision, not just your reading.

### 9.1 Nine dimensions vs six — the central question

The essay vertical (Studio §14, from 论文板块) named six dimensions: task understanding · agency · evidence & sources · argument structure · feedback comprehension · metacognition. This platform document names **nine** (§3.1). They overlap but diverge:

| Essay six-dim | Platform nine-dim (CT) |
|---|---|
| task understanding | ≈ CT-D1 prompt clarity (loosely) |
| agency | (expressed through tool-card initiative, not a CT dimension) |
| evidence & sources | = CT-D2 + CT-D3 |
| argument structure | = CT-D5 + CT-D7 |
| feedback comprehension | *(no CT equivalent — this was essay-specific: what the student does with AI feedback)* |
| metacognition | = CT-D6 |
| — | CT-D4 perspectives & concession *(new)* |
| — | CT-D8 information reproduction / AI-vs-self *(new)* |
| — | CT-D9 AI boundaries & ethics *(new)* |

**Recommendation:** make the **nine-dimension CT rubric the platform canon**, and treat the essay six-dim as a *reporting rollup* of it, not a parallel system. The nine are richer and more AI-collaboration-focused, and they are what the shared engine, dashboard, and framework-alignment table are already built on. The one genuinely essay-specific idea the six had — *feedback comprehension* (what a student does with a suggestion) — should be added as a **tenth CT dimension** or folded into CT-D6, rather than lost. It is measured by the Studio's accept/reject/rewrite-with-reason signal, which is real and worth keeping.

The Studio stations then map cleanly onto CT dimensions rather than needing their own rubric:

| Station | Lights up |
|---|---|
| S1 Frame the question | CT-D1 |
| S2 Perspectives & sources | CT-D2, CT-D3, CT-D4 |
| S3 Evaluate the sources | CT-D2, CT-D3 |
| S4 Build the argument | CT-D4, CT-D5, CT-D7 |
| S5 Draft & polish | CT-D7, feedback comprehension |
| S6 Reflect & archive | CT-D6, CT-D8, CT-D9 |

**This needs a decision** because it changes what the Studio's §14 reports against and what the growth report's radar shows.

### 9.2 The reporting layer composes on top

The Studio spec added a *reporting layer* the platform document does not have: thinking leaps T1–T7, the depth-vs-independence chart, the three report versions, the north-star metric. These are not a competing rubric — they are **projections of the same evidence** this document scores. The mapping:

| Studio reporting artifact | Assessment substrate here |
|---|---|
| Thinking leaps T1, T5 (first unprompted use, caught-it-first) | CT-D6 metacognition + the 自发/提示后 tool-card signal (§4) |
| Leap T2, T4 (stance revision, owning the counter-case) | CT-D4 perspectives & concession |
| Leap T3, T7 (reasoned refusal, knowing what not to ask) | CT-D8, CT-D9 |
| Depth axis | six/nine-dimension total → SOLO L1–L4 (§2) |
| Independence axis | tool-card initiative + refusal signals |

So: **this document is the rubric + engine + dashboard layer; the Studio's §14 is the reporting layer on top of it.** They compose. Neither is redundant.

### 9.3 The `D` naming collision — resolve it now

There are now four different things numbered with `D`: CT dimensions (D1–D9), OPCVL dimensions (D1–D12), the essay six-dim (D1–D6), and the Studio's decisions (renamed DEC earlier). Proposed convention, to be applied across all specs:

- AI critical-thinking dimensions → **CT-D1 … CT-D9** (used in this document).
- OPCVL dimensions → **HS-D1 … HS-D12**.
- Decisions → **DEC-n**.
- Retire the bare essay `D1–D6` in favor of the CT mapping (§9.1).

### 9.4 Convergences already in both specs (build once)

- **Tool cards as behavioral signal** = the Studio's `card_clicked` with 自发/提示后 (DEC-10). Same event.
- **Anchor samples** Phoebe / Marcus / Ethan / Eliza = the Studio's R-7 golden sets. Same assets.
- **Three-layer dashboard** = the substance of DEC-2's teacher report.
- **Model strategy** (few-shot MVP → benchmark → fine-tune; cheap-vs-flagship routing) = the Studio's §16.1 routing. Same engine.

---

## 10. Open questions

1. **Adopt the nine-dimension CT rubric as canon (§9.1)?** If yes, the Studio's §14 and the growth-report radar retarget from six dimensions to nine (+ feedback comprehension). This is the decision that gates the rest.
2. Where does **feedback comprehension** live — a tenth CT dimension, or folded into CT-D6? It is essay-specific but real, and it is the only thing the six-dim had that the nine lacks.
3. **OPCVL in v1?** The Cambridge-first MVP (Studio DEC-9) is the essay vertical. Does the 12-dimension OPCVL rubric ship in v1 at all, or is it a later module? The framework-alignment moat leans on it; the MVP scope may not include it.
4. **Dashboard privacy across surfaces.** The dashboard aggregates Studio *and* Chat evidence (Chat is supplementary and transparent, per workbench-spec §2.2). Does the teacher see Chat-derived signals, or only Studio ones? Ties to workbench-spec §20 Q7.
5. **Level calibration ownership.** The L1–L4 behavior ladders per dimension are a standing authoring workstream (like the card golden sets). Who owns them, and how are they versioned against framework updates?

---

## 11. Glossary

| Term | Meaning |
|---|---|
| **Dimension** | One observable facet of critical thinking, graded L1–L4. CT-D* for AI critical thinking, HS-D* for OPCVL. |
| **Level (L1–L4)** | The SOLO-aligned grade of one dimension, defined by a behavior ladder. |
| **Rubric** | A set of dimensions for one module. The platform holds several on one engine. |
| **Tool card** | A clickable thinking scaffold whose use is both a mode switch and an assessment signal. The same card object as the Studio and Course. |
| **Anchor sample** | A fully annotated exemplar dialogue (Phoebe, Marcus, Ethan, Eliza) that is both a few-shot example for the engine and an executable definition of a level. |
| **Behavior ladder** | The four observable-behavior definitions (L1–L4) for one dimension. |
| **Historiographical awareness (HS-D12)** | Awareness of one's own position as the analyst; the OPCVL soul, mapped to TOK's knower-and-knowing. |

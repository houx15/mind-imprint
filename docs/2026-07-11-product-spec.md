# Product Spec: The Writing Studio

| | |
|---|---|
| **Status** | Draft v3.2 — merged, plain language |
| **Author** | Product |
| **Last updated** | July 2026 |
| **Scope** | Product navigation (the two top-level tabs) · the **Chat** surface · the **Writing Studio** (写作工作室), the first module of the Project Space tab. Course library and Growth Report are in `思维印记_论文板块产品设计文档_v3.md` |
| **Supersedes** | v3.1 (`archive/workbench-spec-v3.1.md`), v3, v2, v1 |
| **Merged from** | `思维印记_论文板块产品设计文档_v3.md` (content, pedagogy, compliance, assessment) × workbench-spec v2 (interaction architecture) |

---

## 0. How to read this

**On language.** An invented term is worth having only if it names something that recurs *and* has no plain equivalent. Most don't. Where the source documents coined a name for an ordinary thing, this spec uses the ordinary word. The terms that survive are in the glossary (§22); everything else is said plainly.

**On authority.** Where this spec and 论文板块 v3 conflicted, 论文板块 won. Its seven-qualification rubric research is the moat; this spec supplies the interaction architecture.

---

## 1. What changed in v3

**Adopted from 论文板块:** the S0–S6 journey with gates · the four red lines · student-invocable tool cards · the output check · the student-written requirement · accept/reject/rewrite with reasons · the source log · getting unstuck · spot-the-flaw · examiner voices · the AI usage declaration · unused evidence and unsupported claims · the word budget · five kinds of mark scheme · the six assessment dimensions · the event stream · thinking leaps · the depth-vs-independence chart · three report versions · Cambridge-first MVP.

**Retained from v2** (they have no equivalent): the State Evaluator · cards as staged experiences over four views · scaffolding fade · Draft Snapshots with silent edit and speaking preview · the first-entry recognition moment · situated memory on return · `resolve_remediation()` · the config contract · "a new review requires a new snapshot."

**Also in v3.2:** a top-level navigation model (two tabs — Chat and Project Space — §2.1), the **Chat** surface specified (§2.2, DEC-12), and the seven stations deepened with the thinking a student actually does at each one (§5.2), the S0↔S6 prediction loop, the living plan, and the S2–S4 research loop.

**Three reversals**, stated plainly so they can be found and undone:

| | Was | Now | Cost |
|---|---|---|---|
| **R-A** | No gated steps. **Reverse entry was first-class** — a student could paste 800 words before any structured thinking. | S0–S6; a gate must pass before the next station unlocks (DEC-8). | **Reverse entry is removed.** A student arriving with a finished draft must still walk the journey. This was your own v1 decision, not an outside proposal. |
| **R-B** | No teacher surface in v1; the export was the school wedge. | A read-only teacher report ships in v1 (DEC-2). | Brings identity and a second surface into v1 scope. Forced by the gates: many gate items need teacher judgment. |
| **R-C** | Cards surfaced only when the AI decided. No tool rail. | Students can invoke any card themselves (DEC-10). | None. This was a **bug fix** — see DEC-10. |

Decisions are numbered **DEC-1…DEC-11**, so nothing collides with the six assessment dimensions **D1–D6**, which are platform-wide and predate this spec.

---

## 2. Overview

The Studio is where a student brings a real, high-stakes writing task — an IGCSE 0457 Individual Report, a GPR 9239 Essay, an EPQ report, an AP Seminar IWA, an AP Research paper, a TOK essay, an EE — and works it from prompt to finished draft. **The AI questions, challenges, and diagnoses. It never writes.**

It is one of three pillars: the **course library** (what to learn) × the **Studio** (how to practise) × the **growth report** (what is measured, and to whom it is shown).

The Studio is **not** a chat tutor, **not** a writing assistant, and **not** a self-writing platform. It is a structured workspace where thinking becomes inspectable artifacts, and the coach intervenes based on the state of those artifacts.

**Why it can exist at all.** Across all seven qualifications, teacher feedback is *institutionally locked*: IB allows one comment on one draft; Cambridge near-totally prohibits graded feedback on externally assessed work; AP permits process guidance only; OCR's EPQ allows spoken feedback and nothing written. The feedback vacuum is structural, not incidental. **Self-assessment plus out-of-school process feedback is the only space the boards leave open.** We never compete for the right to mark a final draft — that is a violation. We train the student to become their own examiner.

### 2.1 The two top-level tabs

The product has two top-level tabs, siblings at the same level (the way Chat and Cowork sit side by side):

- **Chat (聊天)** — a free, open-ended AI conversation. Where thinking starts. §2.2.
- **Project Space (工作室)** — structured work on a concrete deliverable. It holds sub-tabs; in v1 there is one, the **Writing Studio (写作工作室)**, which is what §3 onward specifies. Future project types (e.g. a research or design project space) slot in beside it as further sub-tabs.

So everything after §2 describes the Writing Studio — one module inside Project Space — except where it says otherwise.

### 2.2 The Chat surface

**Purpose.** An open thinking space for what isn't yet a project: a half-formed question, a concept the student wants to understand, an article they want to talk through. It is where a student goes *before* they know they have an essay.

**Shape.**

- A free conversation box with persistent, revisitable **history** (threads). This is the one place in the product with a conversation list — the Studio deliberately has none (R-8, DEC-12).
- **Multimodal input:** attach files and images; send voice (speech-to-text).
- The AI's posture is **guiding, not answering** — the same ethos as the Studio, one notch more permissive in scope. Chat may *explain and inform*, because it is a learning space; it still will not *produce the student's assessed deliverable*. **RL-1 and RL-4 hold in Chat** — a student cannot route around the red lines by moving here.
- **Cards can surface in Chat.** When the AI recognizes a thinking moment, it can bring up a card — source evaluation on a link the student pasted, steelman on an opinion they stated. Same card runtime as the Studio and the Course: the card is the atom across all three surfaces (DEC-7).
- **Chat history is a source of evaluation.** Chat events feed the six dimensions (§14.1) and can surface thinking leaps, as *supplementary* evidence — weaker and less structured than Studio evidence, but real. This carries a hard obligation, below.

**Transparency (non-negotiable).** A free chat box that is also on the record must say so. This product refuses surveillance (§13); Chat that quietly graded the student would break that promise. The student must know Chat contributes to their growth record, and may need control over it — see §20.

**Relationship to the Studio.** Chat is where thinking starts; the Studio is where it gets serious. The natural path is **Chat → start a project**: a conversation that turns out to be about a real assignment can seed a Studio project, carrying its useful fragments in as the first artifacts. Not required for v1, but the reason the two tabs are siblings rather than strangers.

---

## 3. The four red lines

Inherited from the platform, tightened for essays. These are not guidelines.

**RL-1 — The AI never writes essay content.** No sentence a student could paste into the essay. Advice about rewriting appears only as two options plus the student's stated reason: *keep as written* / *student revises* / *explain why not*. Text quality is not mastery of thinking.

**RL-2 — The AI never generates a citation, and never paraphrases a source the student has not opened.** Citations may only be added from sources the student actually opened, recorded in their source log. One rule closes hallucinated citations and academic dishonesty at once.

**RL-3 — Readiness, never a predicted grade.** The platform reports where a draft sits against the official descriptors. It never reports what grade it would earn. This is both the difference from RevisionDojo and the compliance safety margin, and it belongs in product copy and sales scripts.

**RL-4 — Reflection is written by the student.** The platform supplies material — timestamps, the student's own words, decision records — and prompting questions. It never drafts an EE reflection statement, an EPQ evaluation chapter, an AP PREP response, or a GPR reflective piece, **because those texts are themselves scored**: EE Criterion E is 4 marks, the 9239 research log is 10, POD rows 3 and 7 are 6.

---

## 4. Goals and non-goals

**Goals**

- **G1.** A student with a real title produces meaningful thinking within 60 seconds of first entry.
- **G2.** Every input the student makes is a reusable piece of their eventual essay. Nothing is filled in for the platform's sake.
- **G3.** Guidance is always anchored to this student's title, this student's sources, this board's criteria. Generic advice is a defect.
- **G4.** Process is captured as a byproduct of working, producing both formative assessment data and integrity evidence that fits the official process documents (TOK PPF, EE RPF, the 9239 research log, EPQ logs, AP PREP and checkpoints).
- **G5.** The student keeps authorship throughout. Every AI suggestion can be refused, and refusing well is scored as a strength.

**Non-goals**

- Team and presentation components (AP TMP/IMP, GPR C3, the POD presentation). Different interaction form; out of scope for v1.
- A global free-form chat assistant.
- A competitive document editor. The writing view is a plain buffer, and we recommend the student's own editor (§10).
- **Inline AI writing help of any kind** — autocomplete, ghost text, "apply suggestion," "rewrite this." Not a scope cut; a violation of RL-1.
- Predicted grades, ever.
- Mobile. Desktop web only for v1.

---

## 5. The journey: seven stations

We build one journey for all seven qualifications; the differences between boards collapse into sprint packs, gate parameters, and how progress is displayed. A new qualification costs one sprint pack and one set of parameters.

But a straight S0→S6 line misrepresents how essays actually get written. The true shape is three movements:

- **Frame it (S0–S1):** decide what the task is, and what question you are answering.
- **Loop it (S2–S4):** find sources, evaluate them, build the argument — then go round again. A missing perspective discovered at S4 sends you back to S2; a source that collapses under evaluation at S3 sends you back to search. **This runs several times.**
- **Close it (S5–S6):** assemble the prose, then reflect and archive.

So **S2–S4 is a loop, not three steps in a row**, and going back inside it is the normal rhythm of research, not an exception. The gate on S4 does not test a single pass — it tests whether you have looped *enough*: no unused evidence, no unsupported claim, no claim resting on one source, the strongest counter-case faced. When those hold, you have looped enough.

| Station | Name | What the student produces | Gate — must pass to unlock the next station | Default view |
|---|---|---|---|---|
| **S0** | Decode the task | Rubric translation table; milestone plan | Restates *in their own words* what this piece is assessed on; **predicts their two weakest criteria** (tested at S6) | 评估 |
| **S1** | Frame the question | Research question; operational definitions; provisional answer; feasibility check | A single question, not a compound one; key terms have testable definitions; **a one-sentence provisional answer committed**; states in advance what evidence would change it | 结构 |
| **S2** | Perspectives and sources | Recon notes; perspective map; search plan; source log | A recon pass logged (marked *recon, not evidence*); at least two perspective lines **at different levels**, each backed by at least two opened sources | 素材 |
| **S3** | Evaluate the sources | One evaluation card per source | Every source evaluated (vertical); **key claims checked laterally across other sources; no claim rests on a single source**; each source carries a note on what it does and where it is risky | 素材 |
| **S4** | Build the argument | Toulmin map ⇄ outline | **The hardest gate. Every claim is a full proposition**; no unused evidence, no unsupported claims, no single-sourced claim; every backbone warrant written by the student; the steelman test passed; the concession paragraph has a place on the map | 结构 |
| **S5** | Draft and polish | Draft snapshots; word budget | Word count inside the legal band; citations match front to back; whole-draft review passed | 写作 |
| **S6** | Reflect and archive | Reflection material pack; AI usage declaration; process-document export | The student drafts the reflection personally; the declaration is signed | 评估 |

### 5.1 Two threads that run through the whole journey

**The prediction loop (S0 ↔ S6).** At S0, before doing anything, the student predicts the two criteria they will be weakest on. They have no project data yet, so this is a genuine prediction, not a lookup. At S6, we place that prediction next to what actually happened: *you predicted you'd be weakest on Table E and Table D; you were actually weakest on Table B — what didn't you see about yourself?* The distance between predicted and actual weakness is a real, cheap-to-compute measure of self-knowledge (D6), and it is the only thing that makes S0's self-assessment more than a warm-up. Today S0 and S6 don't talk to each other; this connects them.

**The living plan.** S0 produces a milestone plan. It must not be written once and abandoned — that is the single most common way students lose process marks. OCR's top EPQ band is literally *"embedded throughout"*: monitoring and risk woven through the whole project, and its examiners call a coloured-in Gantt chart *performative*. The 9239 research log is worth **10 marks** for recording process and reflecting on decisions. So the plan is **touched at every station transition** — what changed, what you now expect, what you had to drop — and those touches *are* the log entries that earn those marks. Planning is not an S0 artifact; it is a thread.

### 5.2 What the student actually does at each station

**S0 — Decode the task.** Read the mark scheme and translate it into plain language: what is actually assessed, table by table. Name the areas the task will cover. Predict the two criteria you will be weakest on (the prediction loop). Sketch a first milestone plan. The trap: a first-timer has no prior data to source "my weak points" from, which is exactly why S0 asks for a *prediction* to be tested later, not a diagnosis.

**S1 — Frame the question.** Decompose the task into its key concepts and give each a testable definition — the operationalization check (asking whether a feral child can *recover* without defining recovery is the classic failure). Commit a **provisional answer** in one sentence, so everything downstream is *revising your own idea* — that ownership is what makes the work feel like yours, not the platform's. Then **pre-register**: what evidence would make you change that answer?

The correction to the intuitive version: **do not fix your key arguments here.** A student who locks their arguments before reading goes and finds sources that confirm them — confirmation bias, the exact failure this product exists to prevent. A provisional answer *without* pre-registration is bias; *with* it, it is a hypothesis. That difference is the whole game.

One structural note with scheduling consequences: **framing is two different acts depending on the board.** TOK hands you a prescribed title, so S1 is *unpacking* it. 0457, GPR, EPQ, and EE make you *devise* your own question. Those need different cards and different gates — and because Cambridge ships first (DEC-9), **devise is the default we build, not unpack.**

**S2 — Perspectives and sources.** Viewpoints first, then sources — because if you search first, your viewpoints become whatever the first page of results happened to contain. But there is an honest chicken-and-egg: a student often *cannot* name the viewpoints before reading anything, because they don't yet know the terrain. So S2 opens with a **reconnaissance pass**: read enough to learn what the argument is even about, logged but marked *recon, not evidence*. Then map the viewpoints (0457 requires a local/national one **and** a global one; GPR requires globally contrasting ones). Then search with intent — and now gaps are visible: "seven sources, six of them Anglo-American media; who speaks for the local level?"

**S3 — Evaluate the sources.** Two rhythms, not one. **Vertically (CRAAP):** focus on one source — read it, judge currency, authority, accuracy, purpose, lock a verdict, write what it does for your argument and where it is risky, move on. **Laterally (SIFT):** *leave* the source and check its central claim across other sources. These are opposite moves, and your tool card is named for both — 横着看 and 竖着看. The lateral rhythm has a different unit: **the claim, not the source.** A claim resting on a single source is a risk however excellent that source is — a third pathology beside unused evidence and unsupported claims, the **single-sourced claim**, cheap to detect and exactly what 0457's Table E and the IWA's row 5 punish. One refinement to "lock it": a source's *verdict* can lock, but its *role in the argument* cannot — S4 will change what it is for.

**S4 — Build the argument.** Attach sources to claims; see which claims the evidence will not actually carry, and revise them. And the sharpest requirement: **every node on the map must be a full proposition — something that could be true or false.** "Coal power" is not a claim; "China's continued coal build-out shows the transition is additive, not substitutive" is. That is a cheap, checkable gate condition, and it is what makes the warrant requirement mean anything. The consequence: **S4 is where the load-bearing sentences of the essay actually get written** — the map→essay pipeline working as designed, which is why the usual description of S5 is wrong.

**S5 — Draft and polish.** Not "compose the whole article" — S4 already produced the spine sentences. S5 is **assemble, sequence, place the concession, add the intermediate judgments, match the citations front to back, and cut.** Cutting is the underrated one, and where the word limit bites. Notice where the marks actually live: 0457's Table G turns on the single word *cohesive* between L4 and L5; Table H's 5 marks for citation matching are the most commonly forfeited in the whole paper. Neither is "composing."

**S6 — Reflect and archive.** Evaluate yourself against the mark scheme — placed **right next to the system's readiness display**, because the gap between "where I think I am" and "where the descriptors say I am" *is* the metacognition being measured. Then write the retro, and it must pass the causal test examiners impose: not "I did X, then Y" (a diary — EE gives it no marks) but "X made me change Y, because Z." We are uniquely able to help honestly here: hand the student their own S1 pre-registration beside what actually changed, and let them write the paragraph. Evidence from us, prose from them (RL-4).

### 5.3 Going back a step is the loop working

Any station may overturn an earlier one — S4 revealing that the argument cannot carry the title sends the student back to S1. The system records this as **evidence-based revision**, never as failure. Inside the S2–S4 loop it is not even a step back; it is the loop turning. This is what AP Research's Chief Reader means by "research is iterative and recursive," and it is where thinking leap **T2** most often happens.

> **What the gates cost (R-A).** Reverse entry — pasting a finished draft and having the system back-fill what is missing — is **removed**. A student who arrives with prose already written must still walk S0→S5. If that turns out to be the common first session, revisit DEC-8.

### 5.4 What changes per qualification (parameters, not structure)

- **S0** — 0457 and 9239: embed the official mark scheme, translated row by row. TOK: a workbench for the six prescribed titles, plus the adverb ladder. AP Seminar: a **stimulus workbench** — lock a theme shared by at least two stimulus materials *before* framing the question, which is what protects the IWA's legitimate parentage. AP Research: side-by-side band portraits. EE: new or old guide, switched by enrolment year.
- **S1** — EPQ: a hard check that key terms are defined (the examiner-report trap: asking whether a feral child can *recover* without defining recovery) and an estimate of hours (guards the too-wide and too-narrow extremes OCR names). EE interdisciplinary: a concept bridge between the two subjects. AP Research: name the gap in the literature — the difference between band 4 and band 5. AP Seminar: a **narrowness switch**, because IWA row 2 is pass/fail — "water pollution in India" passes, "air, water and land pollution" fails.
- **S2** — 0457: a local or national perspective **and** a global one, both required. GPR: globally contrasting perspectives required. TOK: a perspective is an area of knowledge and an epistemic stance. **All AP paths use the official definition: a perspective is a point of view conveyed through an argument.** Facts, topics, lenses, and generalized stakeholders ("teachers," "students") do not count.
- **S3** — AP paths add a **credibility check** ("journalist John Doe explains…" does not establish credibility — prompt for the credential, institution, or method) and an **old-data check**. Encyclopedias and dictionaries are marked as not qualifying.
- **S4** — AP Research: an **alignment checker** across question ⇄ method ⇄ conclusion; move one point and the other two are re-flagged. GPR: a balance check between the two perspectives, since AO2a's three marks are lost to favouritism. AP Seminar: an **organization × connection** dual diagnostic, because IWA row 4's official wording awards 12 only when both hold and 8 when only one does.
- **S5** — word budget parameterized. EE: an **evaluation-density scan**. Criterion D is worth 8 marks, it lives at the end, and the examiner stops reading at word 4,001 — so overrunning sacrifices the heaviest criterion first. EPQ: de-emphasize word count; check instead that all four assessment objectives have evidence somewhere in the report.
- **S6** — the export forks: the 9239 research log · EPQ's three different board logs · EE reflection material and a viva voce briefing · AP PREP material and checkpoint preparation.

---

## 6. Object model

| Object | Definition |
|---|---|
| **Project** | One writing task: qualification + title + deadline. |
| **Station** | One of S0–S6. Holds gate items. |
| **Gate item** | One condition for completing a station. States: `empty` / `draft` / `flagged-weak` / `solid`. Judged by machine, by the student writing it, or by a teacher. |
| **Artifact** | A student-made unit of thinking, typed and versioned: position statement, Toulmin node, source evaluation card, reflection entry. |
| **Draft snapshot** | The student's prose at a commit point — pasted, or committed from the writing buffer. **Immutable.** One commit = one version = one reviewable unit. |
| **Edit buffer** | Mutable scratch text in the writing view. Private, silent, **not a record**. The AI never writes to it. |
| **Material** | Any text with span indices: a source, or a draft snapshot. Interventions anchor to spans within it. |
| **Card** | The platform's atomic thinking tool — SIFT, CRAAP, steelman, concession paragraph, Toulmin. Declares a target, an ordered list of stages, rubric tags, and per-board vocabulary. **Never rendered as a form**; it is an input to the AI. `card_id` routes between Studio and Course. |
| **Stage** | One step inside a card: a view, a goal, an entry condition, and how the AI should behave. |
| **Intervention** | One AI question or diagnosis. Anchored to an artifact or a span, tagged with the criterion it serves. |
| **Disposition** | What the student did with an intervention: accept, reject, or rewrite — each with a written reason. |
| **Process record** | An auto-generated projection of version history and the event stream. Not directly editable. |

---

## 7. How the AI must behave

### 7.1 The six rules, each with a machine-checkable definition

1. **Ask, never write.** No AI output may contain a complete sentence the student could paste into the essay. **The output check:** if the AI's output is a declarative sentence *and* its semantic similarity to the student's current topic exceeds a threshold, intercept it and rewrite it as a question. **One exception:** concrete examples drawn from *another subject*, which cannot be pasted.
2. **Anchor to the student's own text.** Every AI question must quote a specific fragment of the student's document, argument map, or log, with a highlight that links back. An unanchored question is a **defect, not a weak question**: *"Have you considered other angles?"* goes on the banned-phrasing list and becomes a regression test.
3. **One question at a time, with visible progress.** One question per message. The station shows how many gate items remain.
4. **Three things must be written by the student.** The **warrant**, the **steelman**, and each source's *what it does and where it is risky*. Enforcement: those fields reject pasted AI text. The student may paste their own notes or a source quotation, in quotation marks.
5. **Silence while drafting; iteration before submission.** While the student is composing, the AI says nothing — otherwise students write a sentence and beg for approval, which is a dependency loop. The student triggers the **whole-draft review** themselves. It reports which paragraph is submitting evidence to which criterion, and what is missing. It never rewrites a sentence.
6. **Every suggestion gets a response and a reason.** Accept, reject, or rewrite — any of the three requires a written reason of at least fifteen characters. *Rewrite* means the student rewrites. Those reasons are the main evidence for D5 (feedback comprehension) and for thinking leaps T3 and T5.

### 7.2 Requirements

- **R-1. The AI never generates essay content.** Enforced at the system level (§16.3), not by prompt convention. See RL-1 and RL-4.
- **R-2. No unanchored interventions.** Every utterance attaches to an artifact, a gate item, or a span.
- **R-3. Tagged to a criterion**, phrased in the board's own vocabulary.
- **R-4. The AI surfaces cards based on workspace state, not because the student asked in chat.** This governs *AI-initiated* surfacing only. **Students may invoke any card themselves at any time** — that is the behavior the product exists to produce (DEC-10).
- **R-5. No going through the motions.** A source evaluation is incomplete until the student answers "does this change how you use it?" Completing a tool with no consequence for the argument is flagged, not rewarded.
- **R-6. Scaffolding fades, per card, per student.** Early: direct questions. Then: "what should you be asking yourself here?" before revealing the card. Eventually: intervene only when a critical check is missed. Fade consumes the unprompted-use signal (§15.1) and is shared with the Course module.
- **R-7. Intervention quality is an evaluated system.** Every card ships with graded good and bad examples, authored with examiner-experienced teachers, plus the banned-phrasing list. Both are standing regression suites (§19).
- **R-8. No global chat *inside the Studio*.** Within the Studio, conversation exists only in the context of a selected artifact or station — free chat here would dissolve the anchoring the Studio depends on. The free **Chat** surface (§2.2) is a *separate top-level tab*, not part of the Studio. Both are true; see DEC-12.
- **R-9. A card is an input to the AI, never a form for the student.** The AI applies the card's dimensions to the student's actual material, produces concrete anchored questions, and invites the student to raise questions the card does not cover. The card's abstract framework is revealed only **after** it has been used, as the transferable takeaway.

---

## 8. The coach

### 8.1 An orchestrator, not a chatbot

> **The station is the map** — where I am and what this stage demands. **The card is the rails** — how to think about one thing. **The coach is the driver** — which card, when, entering at which stage, and how hard to push.

The coach's entire tool surface:

| Tool | Purpose |
|---|---|
| `surface_card(card_id, target, entry_stage)` | Run a card. The entry stage is derived from gate state, not chosen. |
| `post_intervention(anchor, criterion, body)` | One anchored, criterion-tagged question or diagnosis. |
| `check_gate(station)` | Evaluate gate items; report what is missing. |
| `route_to_course(card_id)` | Send the student to a course practice when stuck (§8.5). |
| `invite_commit(gate_item)` | Ask for a draft snapshot of a specific section. |
| `order_review(snapshot_id)` | Run the whole-draft review (§13). |

The coach decides *which tool, on which target, now* by reading workspace state — the **State Evaluator** (§16.1). It emits **exactly one next action**.

### 8.2 The four views

The centre of the screen shows one of four views. **A new card is a sequence over these four, never new UI code.**

| View | What it shows | Stations |
|---|---|---|
| **结构 Structure** | The Toulmin argument map, convertible to an outline. Three pathologies are always visibly flagged: **unused evidence** (a source connected to no claim), **unsupported claims** (a claim with no evidence), and **single-sourced claims** (a claim resting on one source only). Every node must be a full proposition (§5.2, S4). Map nodes and draft paragraphs are anchored both ways. | S1, S4 |
| **素材 Material** | A source text with highlights, margin questions, and the student's annotations; the source dossier; the source log. | S2, S3 |
| **写作 Writing** | The draft, in **edit** mode (silent) or **preview** mode (annotated). §10. | S5 |
| **评估 Review** | The readiness display (§12), the rubric translation table, whole-draft review results, the reflection pack. | S0, S6 |

Two things are deliberately **not** views: **the coach**, which lives on the right and is always present; and the **consolidation moment**, when a card's framework is finally revealed — an overlay at card completion, not a place you navigate to.

The station sets the default view. **The student may switch views freely at any time.** The gate governs progression, not looking.

### 8.3 Tool cards: the student can reach for them

The cards live on the right, beneath the coach, as buttons. Clicking one opens its scaffold and records an event.

- Each card shows the student's proficiency with it: **used unprompted** versus **used after a hint**.
- **The rule for telling them apart:** an invocation counts as unprompted if the AI has not mentioned that card in the preceding **three turns**.

This is why the reversal in DEC-10 was a bug fix rather than a preference. Our north-star metric is *unprompted critical-thinking actions*, and R-6's fade ends when the student acts without being asked. **If only the AI could surface a card, no student could ever act unprompted, and neither the metric nor the fade signal could ever be nonzero.** The coach's job is to make itself unnecessary; that requires an affordance for not needing it.

### 8.4 A card runs in stages

A card declares ordered stages; each stage declares a view, a goal, and how the AI behaves. **The entry stage is derived from the gate item's state**, never chosen freely:

| Gate item state | Enters at | What the coach does |
|---|---|---|
| `empty` | **Draw out** | Question the student into producing the artifact, in their own words. |
| `draft` | **Interrogate** | Anchor onto what they wrote and test it. |
| `flagged-weak` | **Challenge** | Sharper interrogation of that specific weakness. |
| `solid` | *(does not surface)* | Except occasional spot-checks as scaffolding fades. |

Cards that need external material — SIFT and CRAAP on a source, the concession check on a draft — have no *draw out* stage and do not surface until material exists.

**Two guards, both load-bearing:**

- **Drawing out must never become writing (RL-1).** "Who would disagree, and what is their best reason?" is coaching. "For example, you could argue that…" is writing the essay. No suggested counterclaims, no candidate examples, no menus to choose from. The student's answer *is* the artifact. The output check is tuned hardest here.
- **Drawing out is not a form (R-9).** It happens in the coach's rail, one question at a time. Four boxes labelled Claim / Counterclaim / Evidence / Implication is a worksheet.

### 8.5 Getting unstuck: three steps

1. **Rephrase** the question.
2. **An example from another subject** — concrete, illuminating, and impossible to paste.
3. **Two directions.** The student picks one and says why.

If all three fail, route to a course practice, or flag for the teacher. Routing out is **conservative**: only on repeated weakness with the same card, or when the student explicitly asks. Essay projects run for weeks and momentum is the scarce resource; an AI that offers a lesson at every stumble is an interruption machine. The student always returns to the exact artifact they left.

### 8.6 Spot-the-flaw

Now and then the AI offers a suggestion **explicitly labelled as unverified**, containing a real and identifiable defect — an overgeneralization, a source graded at the wrong tier, a causal claim with no support. The student has to catch it and say what is wrong.

This trains the one competence the whole platform exists for: **working with AI without deferring to it.** The catch rate feeds the independence axis (§15.3) and thinking leap T7. Frequency is teacher-controlled, low by default.

---

## 9. Station scripts

Two are given in full; the rest live in the sprint packs. Each script has the same fields: entry condition, AI opener, anchored question bank, gate adjudication, prohibited behavior.

### S2 — Perspectives and sources (the strictest retrieval rules)

The student submits a **search plan** → the AI questions the plan → **the student searches, themselves** → every source they open is logged automatically (URL, time spent, one-line takeaway) → the AI advises on *search direction only*, from the log.

**Hard rules (RL-2):** the AI supplies no ready-made material, paraphrases no source the student has not opened, and generates **no bibliography entry**. Citations can only be added from sources actually opened.

### S3 — Evaluate the sources

Each selected source gets one evaluation card: the five CRAAP dimensions, a pyramid tier, and a written note on *what this source does for my argument and where it is risky*. Key claims trigger lateral reading. AP paths add the credibility and old-data checks.

**Gate.** By machine: every card complete, at least one lateral read on key claims. By teacher: spot-check the quality, which guards against template praise.

> **AI:** 你把这篇《中国可再生能源装机量全球第一》标为核心证据。CRAAP 卡「权威性」你打了高分，理由是「大网站」。这条数据往上追，原始出处是谁？
> **学生:** 好像是转载的……我找一下。原始是国家能源局年度报告。
> **AI:** 好。那「大网站」和「国家能源局报告」哪个该进评估卡？
> **学生:** 能源局，官方一手数据。
> **AI:** 一手数据解决「数字准不准」。你的论证要用它证明「更可持续」——装机量大，等于更可持续吗？中间还差什么？
> **学生:** ……差实际发电占比？装了不等于用得多。
> **AI:** 这句话值得写进「作用与风险」。现在用横向阅读卡：找一条**不是**中国官方口径的来源，看它怎么谈装机与消纳的差距。

*Events recorded: two sources opened, the lateral-reading card used, the risk note written by hand — candidate evidence of unprompted work on D3.*

### S4 — Build the argument (the hardest gate)

**The gate has four parts.** ① No unused evidence, no unsupported claims (machine). ② Every backbone warrant written by the student (machine checks it exists; a teacher checks it is any good). ③ **The steelman test**: the student writes the strongest version of the opposing case; the AI cross-examines once, in the persona of a reader who actually holds it; the student answers. ④ The concession paragraph has a node on the map.

**Prohibited:** the AI never writes the steelman. Its counter-questions must draw on sources the student has already collected, or on common knowledge. **It never invents data.**

> **AI:** 你的核心主张是「中国的绿色转型让地球更可持续」。换你坐到反方席：一个真诚认为相反结论的人，手里**最硬**的一张牌是什么？注意，不是最容易反驳的那张。
> **学生:** 煤电还在新增？
> **AI:** 这是一张牌。你日志里那篇讲「新增煤电作为调峰备用」的来源，反方会怎么用它、你打算怎么接？
> **学生:** 反方会说：一边装风电一边批煤电，说明转型是叠加不是替代，总排放还在涨。我的回应……看人均和趋势拐点？
> **AI:** 两句都写下来——反方版本进钢人段，你的回应进让步段。最后一问：如果三年后数据表明煤电不是「备用」而是持续满发，你的判断会怎么变？
> **学生:** 那结论就得从「正在让地球更可持续」降级为「有转型意愿但尚未兑现」。

*The last line is a **conditional reversal** — the strongest available evidence for thinking leap T4.*

---

## 10. The writing surface

The Studio **recommends the student use their own editor** — Google Docs, Word, whatever they already write in — and gives them no reason to prefer ours. They may also write or paste here. Both routes produce **identical draft snapshots**.

- **Edit mode is silent.** A plain buffer: paragraphs and headings, nothing else. **The coach says nothing while the student writes.** Live commentary trains prose written to chase marks.
- **Preview mode is where the coach speaks.** Read-only, with margin questions anchored to spans.
- **Committing to preview mints an immutable draft snapshot.** Pasting from outside mints the same object. The sequence of diffs is the version history that feeds the process record.

> **The AI has no write access to the buffer. Ever.** No insert, no "apply suggestion," no "rewrite this," no autocomplete, no ghost text. Because an editable field now exists, RL-1 is an **enforced rule rather than an architectural fact** (§16.3). **Inline AI suggestion is the specific way this dies**, because nobody experiences it as "the AI wrote my essay." It is exactly that.

**We will lose a feature comparison against Google Docs.** Accept it. The answer to that gap is always better import and export, never a better editor.

**The word budget.** Parameterized per qualification, and the limits have teeth: TOK stops at 1,600 words; the EE examiner stops reading at word 4,001, and **Criterion D — the heaviest, at 8 marks — lives at the end.** Deletion decisions are made in the language of the mark scheme: *"Table B currently has one paragraph of evidence, worth 5 marks. Which table do these 300 words of background serve?"*

---

## 11. Boards, mark schemes, and how progress is shown

**RL-3 governs: never a predicted grade.** The display shows where the draft sits against the official descriptors.

The seven qualifications mark in **five structurally different ways**, so progress is shown five different ways.

| Kind of mark scheme | Used by | Character | How progress is shown |
|---|---|---|---|
| Table-by-table points | 0457 (eight tables) | Tables are independent; **stated numbers are hard thresholds**; marks start at "identify" | Eight lamps plus counters — "evaluation points: 3 of 4" |
| Grid of objectives and aspects | 9239 C2/C4; EPQ, all boards | Bands per aspect; descriptors ladder, so the upper band presumes everything in the lower | Aspect lamps plus a band-language slider — *"are you at 'not always consistently' or at 'embedded throughout'?"* |
| Fixed points per row, some pass/fail | AP Seminar IRR, IWA, TMP | Each row has 2–4 fixed values; **some rows are 0 or 5, nothing between** | Switches and step selectors. **A pass/fail row is a switch, not a slider** — "met / not met, and what is missing" |
| One holistic band | AP Research paper | A single 1–5; six attributes must cross together | Band portraits side by side: what 3, 4 and 5 look like on the same attribute |
| One global impression | TOK essay | One driving question, five bands, and **the adverbs decide the band** | A single needle plus the adverb ladder — how many of *sustained*, *specific*, *effectively* are present? |

This table is the engineering basis for **one core, seven skins**: the gate engine and the evidence logic are shared, and only the progress display switches. It also corrects a claim v2 made — see §16.4.

---

## 12. The whole-draft review

- **The student triggers it**, on a draft snapshot. Never ambient.
- It returns: which paragraph submits evidence to which criterion and what is missing; the word budget; whether intermediate judgments exist; whether citations match front to back.
- **It never rewrites a sentence.** Advice appears only as *keep as written* / *student revises* / *explain why not* — and the student gives a reason.
- **One snapshot, one review. A new review requires a new snapshot.** Revision is the price of the next review; this prevents farming feedback on unchanged prose.
- **Examiner voices**, switchable: the board's own (a corpus of that board's examiner-report stock phrases — AQA's *"does the log show the research journey and its decisions"*; OCR's *"the one or two missed opportunities"*), or one of three generic ones — the hostile sceptic, the friendly non-specialist, the word-count executioner.
- Board-specific passes: EE runs the evaluation-density scan and raises an alarm when evaluation is crowded into the last paragraph. AP runs the organization × connection diagnostic and flags every unanchored *"studies show…"*.

---

## 13. Process record, integrity, and compliance

- The process record is generated from version history and the event stream: revised positions, counterclaims added after a challenge, source evaluations that changed how a source was used, snapshot diffs, and course detours taken mid-project.
- **The thinking record, not the prose history, carries the evidentiary weight.** This is why the product can decline to own the student's editor without weakening its integrity claim. Keystroke history proves little — AI prose pastes into any editor. The only kind of editor-ownership that *would* add forensic power is **surveillance**, which this product refuses. It is formative, not forensic.
- **A mid-project detour to learn something is the most valuable evidence in the file.** "Got stuck on counterclaims → went and learned the tool → came back and revised" is precisely the narrative that EE Criterion E and the 9239 research log award marks for. Capture the whole round trip.
- Process metrics measure **thinking quality** — a position revised after counter-evidence, an evaluation that changed how a source was used — and never activity counts. Cards opened and notes written are gameable and pedagogically meaningless.
- **Exports fork by board:** TOK PPF interaction notes · EE reflection material and viva voce briefing · the 9239 research log, the only component where the log is **directly worth 10 marks** · EPQ's three board-specific logs · AP PREP material and checkpoint preparation.
- **The AI usage declaration** is summarized automatically from the ledger — types of interaction, frequency, what the student did with each suggestion — and **signed by the student**. This is the artifact that IB, JCQ, and AP Capstone policy actually ask for.
- **RL-4's enforcement point:** the reflection editor is **completely read-only to the AI.** The AI may question what the student has written. It may not supply any text to paste.

---

## 14. What we measure

### 14.1 The six dimensions

Platform-wide, and they predate this spec: **D1** task understanding · **D2** agency · **D3** evidence and sources · **D4** argument structure · **D5** feedback comprehension · **D6** metacognition. Each is scored L1–L4, aligned to SOLO.

### 14.2 The event stream

| Event | Key fields | Feeds |
|---|---|---|
| `prompt_sent` | text, station, quoted fragment | dimension scoring |
| `card_clicked` | card, **unprompted or after a hint** (did the AI mention it in the last three turns?) | initiative, tool proficiency, fade |
| `gate_attempt` | station, result, what was missing | D1, D4; going-back events |
| `suggestion_disposition` | accept / reject / rewrite, reason text | D5; leaps T3, T5 |
| `verbalization_submitted` | type (warrant, steelman, risk note), text | D2, D3, D4 quality |
| `source_opened` / `citation_added` | URL, time spent, source tier, lateral read triggered | D3; the ledger |
| `version_saved` | snapshot id, diff summary | argument-map evolution |
| `rescue_triggered` | which step, did the gate pass afterwards | D2 — leaning on help versus using it |
| `stance_change_logged` | old stance, new stance, the student's own attribution | leaps T2, T4 |
| `chat_message` | thread id, role, modality (text / voice / file / image), quoted fragment | dimension scoring (**supplementary**), leaps — from the Chat surface (§2.2) |

Chat contributes *supplementary* evidence: weaker and less structured than Studio evidence, weighted accordingly, and — per §2.2 — only with the student's awareness that Chat is on the record.

### 14.3 Thinking leaps

The report's protagonist. Seven of them:

| # | Name | What triggers it |
|---|---|---|
| **T1** | First unprompted use | a card previously only used after hints is invoked unprompted |
| **T2** | Position revised on evidence | a stance change attributed to a specific source or event |
| **T3** | Reasoned refusal | a rejected suggestion with a substantive argued reason |
| **T4** | Owning the counter-case | the steelman passes first try, or a conditional reversal is written |
| **T5** | Caught it first | the student names a flaw in their own argument or source **before any AI prompt** |
| **T6** | Carried the tool to new ground | a card used unprompted in a context different from where it was taught |
| **T7** | Knowing what not to ask | the student says a task should not be given to AI, or catches a planted flaw |

**Each leap is one timestamped card showing the student's own words, before and after.** The report's front page is the timeline of leaps.

> No leap is manufactured. "No leaps this cycle" is shown honestly, with a suggested next step. **The report's credibility comes from its willingness to be blank.**

T1 and T5 *are* the north-star metric, made computable. The unprompted-use rule in §14.2 is what makes them computable — instrument it from day one.

### 14.4 Depth versus independence

- **Vertical — depth of thinking:** the six-dimension total, mapped to SOLO L1–L4.
- **Horizontal — independence:** weighted from the share of suggestions rejected or rewritten, verbatim adoption rate (inverted), steelman first-pass rate, evidence-based stance revisions (T2), and the spot-the-flaw catch rate. Starting weights 0.25 / 0.25 / 0.2 / 0.15 / 0.15, refined against calibration samples.
- **The upper-left quadrant is the warning region: deep but not independent.** The student says elegant things and accepts every AI suggestion — 「精致的囚徒」, the elegant prisoner. It triggers a teacher prompt: raise the spot-the-flaw frequency, add steelman practice.
- **The two axes are never combined into a single score.** Narratives to parents lead with the leap timeline; the chart is secondary.

### 14.5 The three reports

- **Student:** the leap timeline on the front page · how the argument map evolved · a six-dimension radar with representative evidence · unprompted versus prompted · tool proficiency · **where the work sits against the mark scheme** ("your S3 work most resembles 0457 Table E's 7–8 descriptor" — rubric language, never a score) · two next actions.
- **Teacher:** all of the above, plus a risk area (integrity flags, dependency patterns, the elegant-prisoner alert) and a class cross-section.
- **Archive:** the process-document material pack and the AI usage declaration, for submission to the school or the board.

The pack supplies material for the student to write official documents. **The platform drafts no scored reflective text** (RL-4).

### 14.6 The north-star metric

**Unprompted critical-thinking actions** — how often a student evaluates a source or constructs a counterclaim with no card surfaced beforehand. Computed from unprompted card use and from leaps T1 and T5.

- Measured **within a project** (second half versus first half) and **across course practices**, using shared per-card competence. Because the student visits practices organically mid-project, the two reinforce each other.
- The cross-project trend becomes a lagging metric once students hold two projects.
- **Guardrails:** AI-generated essay content rate = 0, audited; student-initiated questions per session trending up; interventions rated "generic" in QA sampling below 5%.

---

## 15. First entry and coming back

### 15.1 First entry

A single input: **"Paste your title or question."** No dashboard, no feature tour.

The system must answer within seconds, with recognition value, *before asking the student to do anything*:

> "This is Title 3, May 2027 session. Students writing this title most often stumble on treating 'evidence' as self-explanatory. Ready to take it apart?"

**The bar: the first ten seconds must convey "this product knows my exact assignment," not "this is an AI tool."**

Recognition content is hand-authored where titles are prescribed, and composed from the criteria where they are not. **Note what the Cambridge-first MVP costs here (DEC-9): TOK is the only one of the seven that prescribes titles.** In 0457 the student devises a research question from a 22-topic list; in GPR, entirely. So Cambridge-first means the harder path — recognition for a self-devised question — has to work on day one. That is the accepted trade for a readiness display that actually works.

### 15.2 Coming back

One sentence of situated memory, not a menu:

> "Last time you were halfway through the counterclaim for History and got stuck finding an example. Pick up there?"

One primary action. Station progress visible behind it. Deadline-aware nudges tied to the exam calendar.

---

## 16. Architecture

### 16.1 The State Evaluator — one decision engine

"One next action," state-triggered card surfacing, scaffolding fade, and gate checking are one computation. Build it once.

- **Input:** gate states across S0–S6; the artifact graph; per-card competence; project metadata (board, title, deadline, calendar).
- **Output:** exactly one next action, expressed as one of the coach's six tools; zero or more anchored, criterion-tagged interventions. When it surfaces a card, **the entry stage is derived from gate state, never chosen freely**.
- **Trigger:** a change in workspace state — an artifact created, edited, linked, or committed. Never a chat message.
- **Model routing:** a cheap model for state detection and trigger predicates; the flagship for writing intervention text and for assessment. Assessment never downgrades.

### 16.2 The card contract, shared with Course

- **Shared:** the card registry (intent, trigger predicates, AI behavior guidance, rubric tags, per-board vocabulary) and **per-student, per-card competence**.
- **Not shared:** rendering, layout, navigation. Studio and Course ship independently.
- **`card_id` is the routing key.** There is no separate skill taxonomy (DEC-7).
- **Both directions.** Course → Studio: a card already practised arrives with real competence, which solves the cold start for fade. Studio → Course: stuck → the practice that teaches that card → back to the exact artifact.
- **The seam:** `resolve_remediation(card_id) → { deep_link, estimated_minutes } | null`. A `null` — no course teaches this card — falls back to coaching.
- Confirmed by 论文板块 §3.1: courses X1–X12 each declare their core cards, and Appendix A maps station × card × dimension × qualification. **Courses are composed of card practices.**

### 16.3 Enforcing "the AI never writes" — the highest-severity invariant

An edit buffer now exists, so this is enforced, not architectural.

1. **Typed output only.** `{ type: "question" | "diagnostic" | "reference", anchor_id, span?, criterion, body }`. **No output type can carry insertable prose.** A `reference` may only quote the student's own prior artifact, verbatim, with provenance.
2. **The AI has no write path to the edit buffer.** Forbidden in the UI and in code review: insert, "apply suggestion," "rewrite this," autocomplete, ghost text, inline completion.
3. **The output check.** A declarative sentence whose semantic similarity to the student's current topic exceeds a threshold is intercepted and rewritten as a question. **Exception:** concrete examples from another subject, which cannot be pasted.
4. **The banned-phrasing list.** A regression suite of forbidden moves: unanchored questions, suggested counterclaims, candidate examples. A question that could be asked of any essay fails; a question that could only be asked of *this* paragraph passes.
5. **Audit log.** Every intervention persisted with type, anchor, criterion, and the output check's verdict. The guardrail metric is sampled from this log.
6. The reflection editor is read-only to the AI (RL-4).

### 16.4 The configuration contract, corrected

v2 claimed a new qualification costs zero code. **§11 falsifies that**: a pass/fail row is not a slider with two stops.

> **Within one kind of mark scheme**, a new qualification requires only configuration — gate parameters, card vocabulary, criteria file, calendar, recognition content — and **zero application code**.
> **A new kind of mark scheme** requires a new progress display. There are five. Build them as five renderers behind one interface; configuration declares which one applies.

**For cards, unchanged:** a new card is a config file declaring its target, its stages, and per-stage view and AI behavior, where every view is one of the four. **Zero new UI components.** A card that appears to need a fifth view is a design conversation, not a config change.

### 16.5 Data model

`project` · `station` · `gate_item` (state, who judges it) · `artifact` (typed, versioned) · `draft_snapshot` (immutable, span-indexed) · `edit_buffer` (scratch, not a record) · `material` (span index) · `source_log_entry` (URL, time spent, tier, takeaway, lateral-read flag) · `card_instance` (anchors: span, author = AI or student, dimension, question, answer; plus the framework the student fills in at consolidation) · `intervention` (typed, anchored, criterion, check verdict) · `disposition` (accept / reject / rewrite, reason) · `card_competence` (per student, per card; shared with Course) · `event` (§14.2) · `process_record` (a projection) · `chat_thread` and `chat_message` (the Chat surface, §2.2; `chat_message` carries modality and attachments, and may optionally link to a `project` it seeded).

**Configuration**, static and versioned: the card registry · gate parameters per qualification · criteria files · which progress display applies · recognition content · sprint packs.

---

## 17. v1 scope and the roadmap

**Phase 1 (~3 months) — the Cambridge pair, plus the Studio core.**
0457 Individual Report and GPR 9239 Essay sprint packs. Studio core: S0–S6, the gate engine, the dual view (document ⇄ argument map), the ledger, accept/reject/rewrite, student-invocable tool cards. Four lead courses: unpacking the mark scheme; the birth of a good question; the anatomy of an argument; steelman and concession. Growth report: the leap timeline and the mapping to mark-scheme language.

**Why Cambridge, not TOK.** 0457's eight tables and 9239's aspect grid are the only two of the seven with **fully public, per-descriptor level statements** — the only two where the readiness display can run at per-descriptor fidelity on day one. Word counts are moderate. The existing calibration sample (「中国是否让地球更可持续」) is already a Global Perspectives-style question, so it migrates at zero cost. TOK's global-impression marking yields a single needle, which is the weakest possible first demonstration. **Accepted cost:** §15.1's recognition moment is at its weakest where titles are self-devised.

**Phase 2 (+3 months) — the long-cycle pair.** EE (new and old guides in parallel, a timeline engine, the reflection material pack, the evaluation-density scan) and EPQ (three board-specific document forks, a log engine, the band-language slider), sharing the long-cycle, process-scored timeline infrastructure. The TOK flagship pack: a workbench for the six titles refreshed each session, band training, and a **checker that the student is answering this session's title** — answering a previous session's title scores zero.

**Phase 3 (+3 months) — the AP pair, and the compliance showcase.** AP Seminar (stimulus workbench, pass/fail switch panel, checkpoint simulation) and AP Research (band portraits, alignment checker, seven-row defence simulation). AP's AI policy is the most explicit of the seven; once compliance is proven there, it becomes the market statement: **an AI writing tool that dares to enter an AP classroom.**

**Data actions:** one golden-sample journey per qualification, walked end to end internally → a benchmark of 100–300 annotated real dialogues → teacher corrections flowing back in.

**Risks.** Compliance → the compliance mode, the declaration, RL-3 and RL-4; review AP policy pages, JCQ guidance and IB academic integrity policy every August. Hallucinated citations → S2's hard rules. Second-language output → bilingual scaffolding (thinking may be in Chinese; **the output language is the exam's**) plus translationese contrast practice. Over-promising → "readiness, not predicted grades" written into copy and sales scripts. Report inflation → axes never combined, leaps first, and a report that dares to be blank.

---

## 18. What has to be built and kept building

These are recurring workstreams with hard external deadlines, not one-time authoring tasks. Staff them.

1. **The card library across the whole essay lifecycle.** The largest workstream, and the one most likely to be underestimated. Source evaluation is one card among many; every station needs cards, and each needs its *draw out* and *interrogate* behaviors authored separately. **Card quality is product quality** — a generic question fails R-7 no matter how good the engine is.
2. **Golden sets and the banned-phrasing list**, per card and per stage, authored with examiner-experienced teachers. Scales with (1).
3. **Recognition content**, refreshed each session. TOK's six titles publish in early September.
4. **The rubric archive.** Seven qualifications' official documents are a moat only while current: AP's annual scoring guidelines, the end of the EE parallel period (the old track retires after November 2026), 9239's next version from 2029, and movement in the TOK guide.
5. **Exemplar recalibration** for the readiness display, each session as examiner reports and grade thresholds publish.

---

## 19. Decisions

**DEC-1 — The argument map uses Toulmin-typed nodes, bound to gate items, with a freeform scratch area.** Unused evidence and unsupported claims are always visible. Slots give traceability and legibility for weaker students; scratch preserves expressiveness; nodes are promoted from scratch into slots.

**DEC-2 — A read-only teacher report ships in v1.** *(Reversal R-B.)* Plus the risk area, the class cross-section, teacher control of spot-the-flaw frequency, and teacher-judged gate items. The gates require teacher judgment at nearly every station, so the teacher surface is not deferrable. A full grading workflow and notifications remain out of scope.

**DEC-3 — Automated judgment may never mark a gate item `solid`.** Telling a student they are finished when they are not is far more harmful than telling them they are not finished when they are. Machines may set `draft` or `flagged-weak`; `solid` requires a passed challenge or an explicit confirmation.

**DEC-4 — Never a number.** Bands, the evidence that places the draft there, and the nearest exemplar. Point scores under holistic marking are dishonest. *(= RL-3.)*

**DEC-5 — Fade's cold start is solved by shared per-card competence.** Signals transfer from Course. With no course history for a card, fade starts at the most scaffolded level.

**DEC-6 — A plain writing view: edit is silent, preview speaks. We recommend the student's own editor.** Committing to preview mints an immutable snapshot identical to a paste. RL-1 and the silence-while-drafting rule become *enforced rules* rather than architectural facts; **inline AI suggestion is the specific failure mode.**

**DEC-7 — No skill taxonomy. The card is the atom, and `card_id` is the routing key.** *(No longer provisional — confirmed by 论文板块 §3.1.)* Reintroduce a skill layer only when SIFT and CRAAP both ship and proficiency should transfer between them; that is a registry field, not schema surgery. The cost of deferring is *over*-scaffolding, which annoys; under-scaffolding harms.

**DEC-8 — The journey is gated.** *(Reversal R-A.)* A gate must pass before the next station unlocks. **Reverse entry is removed.** Going back a step remains, recorded as evidence-based revision, never failure. Revisit if reverse entry proves to be the common real-world entry.

**DEC-9 — The Cambridge pair ships first (0457 + GPR 9239), not TOK.** *(Reversal of v2.)* Only these two publish per-descriptor level statements, so only these two let the readiness display run at full fidelity on day one. Accepted cost: the recognition moment is weakest where titles are self-devised, and TOK is the only board that prescribes them.

**DEC-10 — Students can invoke any card themselves.** *(Reversal R-C — a bug fix, not a preference.)* R-4 governs AI-initiated surfacing only. Without a student-invocable rail, an unprompted critical-thinking action cannot occur, so both the north-star metric and fade's endpoint were unmeasurable. An invocation counts as unprompted if the AI has not mentioned that card in the preceding three turns.

**DEC-11 — Zero-code configuration holds within a kind of mark scheme, not across kinds.** Five kinds, five progress displays behind one interface. A new qualification inside an existing kind is configuration only.

**DEC-12 — There is a free Chat surface, and the Studio still has no global chat. Both are true.** *(New in v3.2.)* R-8 forbids a global chat *inside the Studio*, where every utterance must anchor to an artifact or station. Chat (§2.2) is a *separate top-level tab* for open thinking that isn't yet a project. Different jobs, different surfaces — they do not contradict. The red lines (RL-1, RL-4) and the anti-surveillance stance (§13) apply in Chat unchanged, and Chat's contribution to assessment is supplementary and transparent to the student.

---

## 20. Open questions

1. **Does removing reverse entry (DEC-8) survive contact with real students?** The most likely first session is a student who already has 800 words. Instrument the first-session entry point and watch.
2. What happens when a student comes back from a mid-project course detour — straight back to the artifact, or a one-line "here's what you just learned, now apply it" bridge? The bridge is probably worth it, but it must not read as a quiz.
3. What is the smallest commit the review can say something useful about — a paragraph, or does diagnosis need surrounding context? This decides the wording of the invitation on the gate.
4. What is the *minimum* recognition content that clears §15.1's bar for a self-devised 0457 or GPR question, where nothing is hand-authored? **DEC-9 puts this on the critical path.**
5. Does the Chinese coaching toggle apply to interventions only, or also to gate items and mark-scheme vocabulary? The vocabulary is the board's own, and translating it may hurt transfer to the exam.
6. What is the retention and deletion policy for the process record, which is simultaneously integrity evidence and a student's personal data?
7. **Chat as an evaluation source — how much control does the student get?** Awareness is mandatory (§2.2). Beyond that: can a student mark a thread "off the record"? Does an off-record thread lose card surfacing too, or just the evaluation feed? The anti-surveillance stance (§13) pushes toward giving control; the evaluation completeness pushes against. Decide before Chat ships.
8. **Chat → project hand-off:** when a chat seeds a Studio project, which fragments carry over, and does the student curate them or does the system propose them?

---

## 21. Handoff

### For the designer, in this order

0. **The top-level shell** (§2.1): two tabs, Chat and Project Space, with Project Space holding the Writing Studio as its first sub-tab. Design the shell so a second project type can join later without redesign.
1. **The first-entry recognition moment** (§15.1). Ten seconds decide whether this feels like "it knows my assignment" or "another AI tool."
2. **The four views** (§8.2) — 结构 / 素材 / 写作 / 评估. Every card that will ever exist is a sequence over these four. Design them as a set, and design the transitions between them.
3. **The coach and the tool cards** (§8.3). Two things live on the right: what the coach asks, and what the student can reach for. The unprompted-versus-prompted signal must be visible to the student without turning into a score.
4. **The writing view's two modes** (§10). Edit must feel calm and empty — no AI presence, no suggestion to accept, nothing to click. Preview is where the margin fills. The transition between them is the moment a student submits prose to scrutiny; make it feel like that.
5. **The structure view** (§8.2): the Toulmin map and its outline, with **unused evidence** and **unsupported claims** always visibly flagged. This is van Gelder's ≈0.8 SD effect, productized.
6. **A card running end to end** (§8.4): drawn out in the coach's rail, one question at a time and never a form → placed on the map → challenged → the framework finally revealed. It must feel like one coach moving with you, not four features stitched together. This is the pedagogical heart; give it the most iterations.
7. **The gate** (§5): progress within a station, what a failed gate looks like (a work order, never a scolding), and what going back a step looks like (a positive event).
8. **The progress display, five kinds** (§11). Start with 0457's eight lamps and counters — it ships first. **A pass/fail row is a switch, not a slider.**
9. **Accept / reject / rewrite** and **spot-the-flaw** (§8.6) — the two interactions that teach a student how to work with AI.
10. **The growth report** (§14). The leap timeline is the front page. Design what "no leaps this cycle" looks like: the report has to be dignified when it is blank.
11. **The Chat surface** (§2.2): a free conversation with thread history, file/image attach, and voice input. Two things to get right — the moment a card surfaces inside an otherwise-free chat (it should feel like an offer, not an interruption), and how the "this is on the record" disclosure reads (honest, not chilling).

### For the coder, in this order

1. **The State Evaluator** (§16.1) — one engine for the next action, gate checking, card surfacing, and fade. Instrument the unprompted-versus-prompted signal from day one, or §14.6 cannot be computed at all.
2. **The card runtime** — a stage machine over the four views. Cards are configuration. Acceptance test: a new card ships as a config file with **zero new UI components**.
3. **Enforcing "the AI never writes"** (§16.3) — the highest-severity invariant. Typed output; **no write path to the edit buffer**; the output check; the banned-phrasing regression suite; the audit log. Tune it hardest against the *draw out* stage.
4. **The gate engine** (§5), with machine-judged, student-written, and teacher-judged item types — including the paste-blocking that enforces rule 4.
5. **The event stream and the ledger** (§14.2). Everything downstream — leaps, the chart, the report, the declaration — is a projection of this.
6. **The card registry** shared with Course; per-card competence; `resolve_remediation(card_id)` behind an interface, handling `null`.
7. **The writing view**: a silent, client-owned edit buffer with no model access, plus the preview renderer. A commit mints an immutable snapshot — identical whether written here or pasted.
8. **Five progress displays behind one interface** (§16.4). Ship the table-by-table one first.
9. **The source log** (§9, S2): search plan → automatic logging of opened sources → citations addable *only* from the log (RL-2).
10. **The process record projection**, the export forks, and the AI usage declaration (§13).
11. **The Chat surface** (§2.2): threaded history; multimodal input (files, images, speech-to-text); card surfacing reusing the same runtime; chat events into the stream (§14.2) as *supplementary, transparent* evidence; and the optional `chat_thread → project` seeding link.

---

## 22. Glossary

Only terms that name something recurring with no plain equivalent.

| Term | Meaning |
|---|---|
| **Station** | One of the seven stages of the journey, S0–S6. |
| **Gate** | The conditions that must be satisfied before a station is complete and the next unlocks. |
| **Card** | A thinking tool — SIFT, CRAAP, steelman, concession paragraph, Toulmin. The platform's atomic unit, shared by the Studio and the Course. |
| **Stage** | One step inside a card: draw out, interrogate, or challenge. |
| **View** | One of the four centre screens: 结构 structure, 素材 material, 写作 writing, 评估 review. |
| **Draft snapshot** | An immutable commit of the student's prose. One commit, one version, one reviewable unit. |
| **Warrant** | The unstated bridge from evidence to claim. Toulmin's term; the student must write it. |
| **Steelman** | The strongest honest version of the opposing case — not the easiest one to knock down. |
| **Concession paragraph** | Conceding the strongest counter-evidence, then explaining why the judgment survives it. |
| **Thinking leap** | A moment of visible growth, captured in the student's own words, before and after. |
| **Readiness** | Where a draft sits against the official descriptors. **Never a predicted grade.** |
| **The elegant prisoner** | A student who thinks deeply but accepts every AI suggestion. The warning quadrant. |

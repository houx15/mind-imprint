# Product Spec: The Writing Workbench

| | |
|---|---|
| **Status** | Draft v2 |
| **Author** | Product |
| **Last updated** | July 2026 |
| **Scope** | Workbench module (Course module covered in separate spec) |
| **Supersedes** | Draft v1 (archived at `archive/workbench-spec-v1.md`); the Workbench sections of `思维印记_产品重构规格_v0.2.md` |

---

## 0. What changed in v2

v1's core design instincts were right and are preserved unchanged: **checklist not pipeline** (§5), **reverse entry as first-class** (§5), **every input flows into the essay** (F1), **the AI never writes content** (R-1), **anti-checklist-theater** (R-5), **process record as integrity evidence** (§9), and **TOK-first** (§12). Do not relitigate these.

v2 does four things:

1. **Resolves all five open questions into decisions** (§15). Prototyping both options is a non-decision that costs a cycle; each is now decided with rationale, and can be overridden explicitly.
2. **Fixes two contradictions with the platform architecture** — the Course relationship (§4, §11.2), and the draft-editor question (§12.1), which is resolved *against* owning an editor: drafts enter as read-only **Draft Snapshots**.
3. **Adds the missing architecture layer** (§11): a single State Evaluator, system-level enforcement of R-1, and a configuration contract that makes "new exam system = zero code" testable rather than aspirational.
4. **Fixes an unmeasurable north-star metric** (§13) and names the recurring curriculum operations this product commits to (§14).

Both prior sign-off items are now resolved. **§11.2 (shared card contract with Course) is signed off**, and extended to cover *bidirectional* routing: students reach the Course from inside the Workbench when they get stuck. **§12.1 is resolved against owning an editor** — we occupy the thinking layer; students write wherever they like and paste drafts back in as read-only snapshots.

---

## 1. Overview

The Workbench is where a student brings a real, high-stakes writing task — a TOK essay title, an EE research question, an A-Level GPR essay, an AP Seminar IWA — and works it from prompt to finished draft. The AI acts as a coach that questions, challenges, and diagnoses; it never writes content on the student's behalf.

The Workbench is **not** a chat tutor and **not** a writing assistant. It is a structured workspace where the student's thinking becomes visible, inspectable artifacts — a position statement, argument nodes, source evaluations, a draft — and the AI intervenes based on the state of those artifacts.

**Relationship to the platform.** The product has two pillars: the **Course** (learn a thinking tool systematically, in scaffolded practice) and the **Workbench** (apply it to your own real, high-stakes work). The **card** is the atom shared by both: courses are built from card practices, and the Workbench surfaces the same cards on the student's real work (§11.2, D7). They do not share a UI runtime in v1.

## 2. Problem

Students facing critical-thinking writing assessments (IB TOK/EE, AP Seminar, A-Level GPR) have three problems:

1. **They don't know what to do next.** These tasks span weeks to months with no prescribed procedure. A blank page and a 1,600–5,000 word target produce paralysis.
2. **They cannot get iterative feedback.** All four exam systems restrict teacher feedback by rule (IB: one draft, one comment; Cambridge: near-total prohibition; AP: process guidance only). The feedback vacuum is structural, not incidental.
3. **Generic AI chat makes them worse.** ChatGPT-style tools either write for the student (an academic-integrity violation under all four systems' 2023+ AI policies) or produce generic advice untethered from the specific title and rubric.

Chat as a primary interface fails here: it puts the "what next?" burden back on the student, produces no persistent structure across a multi-week project, and generates conversation logs — poor data for both process assessment and integrity evidence.

## 3. Goals

- **G1.** A student with a real title can start producing meaningful thinking within 60 seconds of first entry.
- **G2.** Every student input is a reusable piece of their eventual essay. Zero "filling forms for the platform's sake."
- **G3.** The AI's guidance is always anchored to the student's specific title, specific materials, and the specific exam's scoring criteria — never generic.
- **G4.** The student's process is captured automatically as a byproduct of work, producing (a) formative assessment data and (b) integrity evidence compatible with official process documents (TOK PPF, EE RPF/RRS, AP checkpoints, GPR research log).
- **G5.** The student retains control at all times: any order of work, any entry point, AI suggestions ignorable.

## 4. Non-Goals

- Team/presentation components (AP TMP/IMP delivery, GPR Component 3). Different interaction form; out of scope for v1.
- A global, free-form AI chat assistant. Explicitly excluded (see R-8).
- **A shared UI runtime with the Course module.** *(Revised from v1.)* The Workbench and Course render differently and ship independently. However, they **do** share a card registry and per-card competence state, and a student can route from the Workbench into a Course card practice mid-project — see §11.2. v1's blanket "no shared runtime" was too strong: it broke the learn→apply bridge, made R-6 (fade) cold-start unsolvable, and foreclosed just-in-time remediation during a multi-week writing task.
- Real-time per-sentence scoring. Assessment is an explicit "submit for review" action (§10).
- Mobile. Desktop web only for v1; long-form writing is a desktop activity.
- Teacher accounts, grading workflow, notifications. *(See §15, D2.)*
- **A text editor of any kind.** We never own the student's writing surface. Students write wherever they like; drafts enter the Workbench as read-only **Draft Snapshots** (§12.1). We are a thinking platform for writers, not a writing platform.

## 5. Core Design Decision: Checklist, Not Pipeline

**What is fixed:** the *Completeness Checklist* — the set of things a passing essay must contain, derived from the exam's own assessment criteria. For a TOK essay: a direct answer to the title as worded; justified choice of two AOKs; for each AOK, a supported claim, a serious counterclaim, and a real-world example; an evaluated resolution; correct scope (≤1,600 words). Checklists are per-exam-system configuration, authored by curriculum experts, versioned against official syllabi.

**What is adaptive:** the order and the next action. There are no gated steps. The AI continuously reads the state of the workspace and recommends exactly one next action — small, concrete, completable in ~10 minutes. The student may always ignore it and work anywhere.

**Why:** real writing is non-linear. Forced sequence causes abandonment; pure AI improvisation with no stable structure causes disorientation. The checklist provides certainty ("I know what's missing"), free ordering provides autonomy, single-next-action provides momentum. All three feelings are required.

**Consequence: "reverse entry" is a first-class flow.** A student may paste or write 800 words of raw draft first; the system parses it and lights up the checklist retroactively ("You already have a position and one strong example. You're missing a counterclaim — right now this reads as the one-sided descriptive essay examiner reports warn about."). Parsing confidence rules: see §15, D3.

## 6. Object Model

| Object | Definition | Examples |
|---|---|---|
| **Project** | One writing task: exam system + title/question + deadline. Root container. | "TOK May 2027, Title 3" |
| **Checklist item** | One required element of the finished essay. States: `empty` / `draft` / `flagged-weak` / `solid`. | "Counterclaim for AOK 1" |
| **Artifact** | A student-created unit of thinking. Typed. All artifacts are versioned. | Position statement, argument node (claim/counterclaim/evidence/implication), source card, reflection entry |
| **Draft Snapshot** | One paste of the student's prose, written elsewhere. **Immutable and read-only.** One paste = one version = one reviewable unit. Scoped to a checklist item or section, not necessarily the whole essay. | "Counterclaim paragraph, v2, pasted Oct 14" |
| **Material** | A text the student is reading or writing, with span indices. Interventions and card anchors attach to spans within it. | A pasted source article; a Draft Snapshot |
| **Card** | The atomic unit of the whole platform: a curriculum-authored thinking tool — pedagogical intent + trigger predicates + AI behavior guidance + rubric tags + per-exam vocabulary. **Never rendered as a form to the student** — it is an input to the AI. `card_id` is the routing key between Workbench and Course; per-student competence/fade state is keyed to it. | Source evaluation card (CRAAP-based; surfaces as RAVEN language in AP, credibility/relevance/provenance in GPR) |
| **Card practice** | A card exercised inside a Course, on curated material, with teaching scaffolding. Independently addressable and re-enterable. The target of Workbench→Course routing. | "Counterclaim card practice, Course 2, scene 4" |
| **Intervention** | One AI question or comment. Must be anchored to a specific artifact **or span**, and tagged with the rubric criterion it serves. Typed output (§11.3). | Margin note on draft ¶3: "This describes rather than argues — Criterion: analysis" |
| **Process Record** | Auto-generated projection of artifact version history. Not directly editable. **Supersedes the "process tree" of spec v0.2.** | "Position revised twice after counter-evidence; 3 source evaluations changed how sources were used" |

## 7. User Experience

### 7.1 First entry (no project yet)

A single input: **"Paste your title or question."** No dashboard, no feature tour.

On paste, the system must respond within seconds with recognition value — before asking the student to do anything:

> "This is Title 3, May 2027 session. Students writing this title most often stumble on treating 'evidence' as self-explanatory. Ready to take it apart?"

Recognition content (title identification, known pitfalls, examiner-report insights for this title type) is pre-authored by the curriculum team for high-traffic titles (all 6 TOK prescribed titles each session) and AI-composed from criteria for long-tail questions (EE, GPR self-devised titles). **Acceptance bar: the first ten seconds must convey "this product knows my exact assignment," not "this is an AI tool."**

### 7.2 First working minute

The AI asks exactly one question: *"Before anything else — in one sentence, what's your gut answer?"*

Rationale: everything that follows is revision of a committed position. The student experiences the project as "stress-testing and upgrading my own idea," never "completing the platform's procedure." On submission of that sentence, the first checklist item lights to `draft`, and the AI issues its first challenge ("You have a position. Let's see if it holds — who would disagree, and what's their best reason?").

### 7.3 The main workspace

Three persistent regions:

- **Left rail — the Completeness Checklist.** Always visible. Items show state at a glance (lit / dim / flagged). Clicking an item focuses the workspace on it. This is the student's map and the product's answer to "how much is left?"
- **Center — the workspace.** Two switchable views over the same underlying data:
  - **Think view:** **slot-structured** argument map. Each checklist item owns a slot; claims, counterclaims, evidence and implications are nodes placed into slots. A freeform **scratch area** holds unassigned nodes; nodes are promoted from scratch into slots. Source cards link to argument nodes; sources cited by no node render dimmed. *(Decision D1, §15.)*
  - **Draft view:** the student's pasted **Draft Snapshots**, rendered read-only with span-anchored AI margin comments. Nothing here is editable — to change the prose, the student changes it in their own tool and pastes again. The argument map exports as a working outline on demand; on paste, snapshot paragraphs are linked back to map nodes (link-on-paste, using the reverse-entry parser of §5).
- **Right rail — the coach.** Shows the single recommended next action, and the contextual conversation for whatever artifact is currently selected. There is no global chat.

### 7.4 Returning session

First screen on reopen is one sentence of situated memory, not a menu:

> "Last time you were halfway through the counterclaim for History and got stuck finding an example. Pick up there?"

One primary CTA (resume), checklist visible behind it. Deadline-aware nudging tied to the exam calendar (title release dates, school internal deadlines the student enters, final submission dates).

### 7.5 Friction rules (anti-annoyance requirements)

- **F1.** Every field the student fills must flow into the essay. Because we hold no draft (§12.1), the flow is **outward, not inward**: the argument map exports as a working outline, concept definitions export as intro raw material, source-card evaluations export as evaluation sentences — all in the student's own words, carried into whatever tool they write in. If a proposed input has no path into the final essay, cut it from the design.
- **F2.** The recommended next action is always one item, always concrete, always ~10 minutes. Never a menu of options, never "work on your essay."
- **F3.** Any artifact can be created in any order, including a full draft before any structured thinking (reverse entry, §5).
- **F4.** Skipping a recommendation carries no penalty and no repeated nagging; the AI re-anchors to wherever the student goes.
- **F5. A next action may be a Course card practice — but routing out of the Workbench is conservative.** Essay projects run for weeks and momentum is the scarce resource; an AI that offers a lesson every time a student struggles is an interruption machine. Route to Course only on (a) **repeated** weakness on the same `card_id`, or (b) explicit student request ("I don't know how to do this"). Never on first difficulty. The card practice must be small enough to satisfy F2 (~10 minutes) and must return the student to the exact artifact they left.

### 7.6 The write-elsewhere loop

Students write in Google Docs, Word, Notion — wherever they already write. The Workbench does not compete for that surface. The loop:

1. **Export.** When the checklist shows enough thinking to draft a section, the student exports their argument map as a working outline, in their own words (F1).
2. **Write elsewhere.** Outside the product. We neither watch nor care how.
3. **Paste back.** The student pastes prose in. **One paste = one Draft Snapshot = one version = one reviewable unit.**
4. **Read and diagnose.** The snapshot renders read-only with span-anchored margin comments; the checklist back-fills from it (D3 rules apply); paragraphs link to map nodes.
5. **Revise → paste again.** Each new paste is a new snapshot. The diff sequence is the draft's version history.

**Paste granularity is a design commitment, not a preference.** The failure mode of this model is a student who pastes once, at the deadline, receives one review, and has used us as an essay grader. Prevent it structurally: the checklist invites **per-item, per-section pastes** ("paste your counterclaim paragraph") as the natural unit. A whole-essay paste is permitted but is never what the product asks for.

**Two rules that keep the seam from leaking:**
- **Snapshots are immutable.** No inline editing, not even typo fixes. Allowing "just small edits" is how a product acquires an editor by increments.
- **A new review requires a new snapshot** (§10). Students cannot re-run feedback on unchanged prose until they like the answer. This is R-5's anti-theater principle applied to review.

## 8. AI Behavior Requirements

- **R-1. The AI never generates essay content.** It cannot produce sentences, outlines, claims, examples, or paraphrases for insertion. Output types are limited to: questions, diagnostic comments, and references to the student's own prior artifacts. **Enforced at the system level** — mechanism specified in §11.3, not left to prompt convention.
- **R-2. No unanchored interventions.** Every AI utterance attaches to a specific artifact, checklist item, **or text span**. Generic advice is a defect.
- **R-3. Rubric-tagged.** Every intervention carries the assessment criterion it serves, phrased in the exam system's own vocabulary.
- **R-4. State-triggered, not message-triggered.** Cards surface because of workspace state (two claims and no counterclaim; a source card with an unexplained "reliable" verdict), not because the student asked. Implemented by the State Evaluator (§11.1).
- **R-5. Anti-checklist-theater.** Cards that collect an evaluation must force the "so what": a source evaluation is incomplete until the student answers "does this change how you use it?" Mechanical tool completion without consequence for the argument is flagged, not rewarded.
- **R-6. Scaffolding fade.** Per-card, per-student: early projects get direct questions; as competence signals accumulate, the AI first asks "what should you be asking yourself here?" before revealing the card; eventually it intervenes only on missed critical checks. Fade state is tracked per `card_id`, not globally, and is **shared with the Course module** (§11.2), which solves cold start.
- **R-7. Intervention quality is an evaluated system.** Each card ships with a graded set of good/bad instantiation examples (authored with examiner-experienced teachers). AI-generated questions are regression-tested against this set. A question that could be asked of any essay ("is your source reliable?") fails; a question that could only be asked of this student's paragraph passes. This is a standing eval harness, not a one-time QA pass (§14).
- **R-8. No global chat.** Conversation exists only in the context of a selected artifact.
- **R-9. The card template is an input to the AI, never a form for the student.** The AI applies the card's dimensions to the student's actual material and produces concrete, span-anchored questions; it also invites the student to raise their own questions from angles the card does not cover. The card's abstract structure is shown to the student only at **summing-up**, after use, as the transferable takeaway.

## 9. Process Record & Integrity

- The Process Record is generated from artifact version history: position revisions, counterclaim additions following AI challenge, source evaluations that changed argument usage, and Draft Snapshot diffs.
- **The thinking record, not the prose history, is what carries evidentiary weight** (§12.1). This is why the product can decline to own the student's editor without weakening its integrity claim.
- Exportable in formats aligned to official process requirements: TOK PPF interaction notes, EE RRS extracts / RPF reflection raw material, AP checkpoint conversation preparation, GPR research log entries (forward-looking plan format, per examiner-report guidance).
- Positioning: this record is the student's *evidence of independent thinking* — the defense against AI-authorship suspicion, and the raw material for the reflection components that are themselves scored (EE Criterion E: 4 marks; GPR research log: 10 marks; GPR reflection AOs: 15–20%).
- Process metrics measure thinking quality signals (position revised after counter-evidence; evaluation altered source usage), never activity counts (cards opened, notes written). Activity counts are gameable and pedagogically meaningless.
- **Mid-project learning detours are recorded, and they are the most valuable evidence in the file.** "Got stuck on counterclaims → went and learned the tool → came back and revised the argument" (§11.2, F5) is precisely the narrative that EE Criterion E and the GPR research log award marks for. Capture the round trip: the weakness that triggered it, the exercise taken, and the artifact revision that followed.
- **The Process Record export is also the school-facing wedge.** It reaches the integrity-anxious buyer without building a teacher platform (§15, D2).

## 10. Assessment ("Submit for Review")

- Assessment is an explicit student action on a **Draft Snapshot**, not ambient scoring. Rationale: per-sentence live scores train score-chasing prose and cannot be honest under holistic marking (TOK). The snapshot model enforces this structurally (§12.1).
- **One snapshot, one review. A new review requires a new snapshot.** Students cannot re-request feedback on unchanged prose until they get an answer they like; revision is the price of the next review. Reviews may be requested per section, not only on a whole essay.
- A review returns: (a) criterion-by-criterion diagnostic **bands** calibrated against official exemplars and examiner reports; (b) each diagnostic point deep-links back to the artifact/paragraph to fix; (c) a process report snapshot.
- **Never a point score.** For holistically marked systems, output is a band plus the criterion evidence plus the nearest calibrated exemplar, expressed qualitatively ("this reads like the 5–6 band because…"). No probabilities, no false precision. *(Decision D4, §15.)*
- Review output is a work order, not a verdict: every flagged weakness pairs with a recommended next action in the workspace.
- Calibration is a standing workstream: scoring engine benchmarked against officially published graded exemplars per system; recalibrated each session when new examiner reports and thresholds publish (§14).

## 11. Architecture

> This section is for engineering. It exists because v1 stated requirements (F2, R-1, R-4, R-6, and "new exam = config only") without specifying the mechanisms that make them true.

### 11.1 The State Evaluator — one decision engine, not several

F2 (single next action), R-4 (state-triggered cards) and R-6 (fade) are three faces of the same computation. Build **one** component.

- **Input:** checklist item states; the artifact graph (nodes, links, versions); per-`card_id` competence/fade state; project metadata (exam system, title, deadline, calendar).
- **Output:** exactly one `next_action` (concrete, ~10 min, anchored); zero or more `interventions` (anchored, rubric-tagged, typed); an optional `card_surface` decision.
- **Trigger:** runs on **workspace state change** (artifact created / edited / linked / version committed), never on a chat message.
- **Model routing:** cheap model for state detection and trigger predicates; flagship model for authoring the intervention text. (Same chaperone/evaluation split established in the platform spec.)

Building this once prevents two overlapping systems and gives a single place to tune pedagogy.

### 11.2 Card contract shared with Course — bidirectional *(signed off)*

v1's "no shared runtime" Non-Goal broke three things: the learn→apply bridge that is the product's spine, R-6's cold start (v1's own Open Q5 was a symptom), and just-in-time remediation.

**Share the contract, not the UI.**

- **Shared:** the **card registry** (pedagogical intent, trigger predicates, AI behavior guidance, rubric tags, per-exam vocabulary) and **per-student per-card competence/fade state**.
- **Not shared (v1):** rendering runtimes, layout, navigation. Course and Workbench ship independently.

There is **no separate skill taxonomy.** The card is the atom (D7). Both pillars address the same `card_id`.

#### Both directions matter

- **Course → Workbench (learn, then apply).** A card practised in Course arrives in the Workbench with real competence state; fade cold start is solved (D5).
- **Workbench → Course (stuck, then learn).** Essay writing is a multi-week task, not a single sitting. When the State Evaluator finds repeated weakness on card C, or the student says "I don't know how to do this," the next action may be the Course **card practice** for C. On completion, the student returns to the exact artifact they left, with updated competence state. Governed by F5.

#### `card_id` is the routing key

The State Evaluator already identifies *which card* the student is failing to apply. Remediation routing therefore costs almost nothing — provided the Course module can answer one question.

> **The seam (build against this).** The Workbench emits only `card_id`. The Course module exposes:
>
> `resolve_remediation(card_id) → { deep_link, estimated_minutes } | null`
>
> The Workbench neither knows nor cares how courses are internally composed. It requires only that the returned target is **independently enterable, re-enterable, and short enough to satisfy F2 (~10 minutes)**, and that a `null` (no remediation exists for this card) is handled gracefully by falling back to coaching.

**Pending (education design).** How courses relate to cards — composed of card practices, or referencing cards some other way — is undecided and is the education designer's call (§16.1). It changes the *implementation* of `resolve_remediation`, and nothing else in this spec. Do not block the Workbench build on it.

**Known gap:** only weaknesses that map to a card can route. Checklist items with no card behind them (e.g. "justified choice of two AOKs") get coaching, not a lesson. Accepted for v1; the `null` return covers it.

#### Consequences

- Adding a card to the registry serves both pillars.
- Mid-project Course visits are captured in the Process Record as reflection evidence (§9) — the exam systems already pay marks for exactly this narrative.
- Course exercises become an **organically visited** repeated-measure surface for the north-star metric, which TOK-only v1 otherwise cannot measure (§13).

### 11.3 R-1 enforcement (system level)

R-1 is the product's central promise and a guardrail metric ("AI-generated essay content rate = 0, audited"). Prompts will not hold it. Enforce it structurally:

1. **Typed output only.** The model returns structured objects: `{ type: "question" | "diagnostic" | "reference", anchor_id, span?, rubric_criterion, body }`. **No output type exists that can carry insertable prose.** A `reference` may only quote the student's own prior artifact, verbatim, with provenance.
2. **No UI affordance — because there is no editable draft field at all.** Draft Snapshots are immutable (§12.1); interventions render as read-only margin comments. Model output has nowhere to land. This is the strongest form of the guarantee: R-1 holds by architecture, not by validator.
3. **Validator.** Every `body` passes a validator before display: reject declarative topic-content beyond a short threshold; reject imperative-with-content ("Write: 'Knowledge in history is…'"); reject prose that would function as an essay sentence.
4. **Audit log.** 100% of interventions persisted with type, anchor, rubric tag, and validator verdict. The guardrail metric is computed from QA sampling of this log.

### 11.4 Exam systems are configuration — and this is testable

§12's claim that v1.5 pipelines "only change checklist config, card vocabulary, and calendar" must be an architectural acceptance criterion, not a hope.

**Acceptance criterion:** *Adding a new exam system (EE, GPR, AP Seminar) requires only: a checklist config, a card-vocabulary localization, a rubric/criteria file, a calendar file, and recognition content. It must require **zero changes to application code**.*

This mirrors the platform rule that a new card is a new config file and never a change to the renderer. If a new exam forces code changes, the abstraction is wrong — fix it before v1.5.

### 11.5 Data model deltas

On top of the platform model: `project`, `checklist_item` (with state), `artifact` (typed, versioned), `draft_snapshot` (immutable, span-indexed), `material` (with span index), `card_instance` (anchors: span, author=AI|student, dimension, question, answer; plus `summary_fill` captured at summing-up), `intervention` (typed, anchored, rubric-tagged, validator verdict), `card_competence` (per student per `card_id`, shared with Course), `process_record` (projection, not a table of record).

## 12. v1 Scope

**In:** TOK Essay pipeline only. All six May-2027 prescribed titles with hand-built recognition content. Checklist, think/write views, source cards, coach rail, submit-for-review, process record export (PPF format). English UI; Chinese-language coaching toggle for comprehension (essay work remains in English — non-English responses score zero across all four systems).

**Why TOK first:** every candidate worldwide writes the same 6 titles on the same calendar (titles publish early September for May session) — maximal leverage for hand-authored depth content, a natural acquisition moment, and the weakest existing support (no itemized rubric, teachers least equipped to coach it). Accepted trade-off: holistic scoring makes score-gain claims hardest to substantiate; mitigated by exemplar-calibrated review (§10).

### 12.1 We do not own an editor *(resolved)*

The earlier platform rule — "we are not an editor; the student's document lives in their own world" — **stands.** An intermediate draft of this spec argued the opposite, on the grounds that integrity evidence requires holding the document. That argument fails:

- **The integrity evidence is the thinking record, not the prose history.** What demonstrates independent thought is the position committed on day one, the counterclaim added after challenge, the source evaluation that changed how a source was used, the Course detour and the revision that followed (§9). Those artifacts we own. A draft's keystroke history proves little: a student can paste AI-written prose into our editor exactly as easily as into Google Docs.
- **The only editor-ownership that *would* add integrity value is surveillance** — typing cadence, paste detection, draft-back forensics. This product refused surveillance on principle: it is formative, not forensic. So owning the editor buys nothing we are willing to use.

Two benefits fall out of not owning it:

- **R-1 becomes structural.** With a read-only Draft Snapshot there is no editable field for model output to land in. "The AI never writes your essay" stops being a rule enforced by validators and becomes a fact of the architecture. An owned editor would mean permanently resisting the pull toward an "apply suggestion" button.
- **§10 becomes structural.** We already rejected ambient per-sentence scoring because it trains score-chasing prose. The paste cadence enforces that decision rather than relying on product discipline.

**What we accept losing:** live map↔draft linking degrades to link-on-paste (which is the reverse-entry parser we already build); and there is a visible seam where the student leaves to write. Both are cheap next to an editor we would have to maintain, and lose against Google Docs, forever.

**Condition under which this would be revisited:** only if the product ever chose to become forensic about authorship. It will not. This decision is therefore stable.

**Next (v1.5+):** EE pipeline (new 2027 guide: framework/RQ refinement stages, RRS capture, interdisciplinary pathway support), then GPR essay, then AP Seminar IWA (January stimulus-packet integration). Each reuses the full interaction model; only checklist config, cards vocabulary, and calendar change — enforced by §11.4.

## 13. Success Metrics

- **Activation:** % of new projects reaching a committed position statement in first session (target: >70%).
- **Momentum:** median next-action completion rate; week-2 project return rate.
- **Learning (north star):** **unprompted critical-thinking action rate** — frequency of students performing source evaluation / counterclaim construction *before* any card triggers.
  - **Measurement fix (v1).** TOK-only v1 gives each student roughly *one* project, so the original cross-project trend is unmeasurable. For v1, measure: (a) **within-project** — unprompted actions in the second half of a project vs the first; (b) **across Course card practices**, using shared `card_competence` (§11.2) as the repeated-measure surface. Because bidirectional routing (F5) means card practices are visited *organically mid-project*, (a) and (b) reinforce each other rather than requiring separate instrumentation. Cross-project trend becomes a lagging metric from v1.5, when students hold both TOK and EE projects.
  - Instrumentation requirement: the State Evaluator must log "student performed the action card C teaches, with no prior surfacing of C," or this metric cannot be computed at all. Design it in; do not bolt it on.
- **Outcome:** criterion-level band improvement between first and final submit-for-review per project; (lagging) reported exam grades vs. school baseline.
- **Guardrails:** AI-generated essay content rate = 0 (audited via §11.3 log); student-initiated questions per session trending up; % of interventions rated "generic" in QA sampling < 5%.

## 14. Standing curriculum operations

This product commits to three **recurring** content workstreams. They are not one-time authoring tasks, and they have hard external deadlines. Staff them explicitly.

1. **Recognition content** (§7.1): 6 TOK prescribed titles, re-authored each session. Titles publish early September; content must ship before students start.
2. **Card golden sets** (R-7): graded good/bad instantiation examples per card, authored with examiner-experienced teachers, maintained as the regression suite for intervention quality.
3. **Exemplar recalibration** (§10): the scoring engine re-benchmarked each session when new examiner reports and grade thresholds publish.

## 15. Decisions (formerly Open Questions)

**D1 — Argument map: slot-structured, with a freeform scratch area.** *(Was Q1.)*
Slots per checklist item, plus an unassigned scratch space; nodes are promoted from scratch into slots. Rationale: F1 requires every input to have a traceable path into the essay, which a freeform canvas obscures; the target user (paralysed, weaker students) needs legibility; freeform expressiveness is preserved by scratch. Do not prototype both — that defers the decision by a cycle at the cost of the thing F1 is protecting.

**D2 — No teacher surface in v1; the Process Record export is the school wedge.** *(Was Q2.)*
Teacher accounts pull in a second identity system, student–teacher relationships, grading workflow, and notifications. The integrity-anxious buyer is reached today by a student handing over the exported PPF/RRS-format record. Revisit teacher dashboards after v1, informed by which export formats schools actually ask for.

**D3 — Reverse-entry parsing: an asymmetric-cost rule, not an accuracy threshold.** *(Was Q3.)*
A false `solid` (telling a student they are done when they are not) is far more harmful than a false `empty`. Therefore: **reverse-entry parsing may set a checklist item to `draft` or `flagged-weak` at most, never to `solid`.** `solid` requires either an AI challenge passed or explicit student confirmation. Ship when the false-`empty` rate is tolerable to users, not when a parsing-accuracy number is hit.

**D4 — Holistic uncertainty: report a band with evidence and the nearest exemplar; never a number.** *(Was Q4.)*
Point scores under holistic marking are dishonest. Output is a band, the criterion evidence that places it there, and the nearest calibrated exemplar, always paired with a work order (§10).

**D5 — Fade cold start is solved by shared per-card competence state.** *(Was Q5.)*
Competence signals transfer from the Course via shared `card_competence` keyed by `card_id` (§11.2). Where a student has no Course history for a card, fade starts at the most scaffolded level (direct question).

**D6 — We do not own an editor. Drafts enter as immutable Draft Snapshots.** *(New in v2; reverses an intermediate draft of §12.1.)*
The integrity evidence is the thinking record, not the prose history; the only editor-ownership that would strengthen integrity is surveillance, which this product refuses. Not owning the draft makes R-1 structural and enforces §10's no-ambient-scoring principle by architecture. Students write wherever they like; one paste = one snapshot = one version = one review. Full rationale in §12.1; loop in §7.6.

**D7 — No skill taxonomy. The card is the atom, and `card_id` is the routing key.** *(New in v2. **Provisional** — the Course↔card relationship is pending a decision with the education designer. Do not build the Course side against this yet; see §16.1.)*
A separate `skill_id` layer only pays off when two different cards train the same competence and mastery of one should fade the other. That does not happen in v1 (one source-evaluation card, one counterclaim card), and per-exam vocabulary differences are handled by localizing a single card, not by a layer above it.
- **Settled and safe to build:** within the Workbench, competence/fade state is keyed by `card_id`, and the State Evaluator emits "student is stuck on card C." Nothing here depends on how courses are composed.
- **Pending:** whether a course is *composed of* card practices, or merely *references* cards some other way. That is the education designer's call. The Workbench is insulated from it by the resolver interface (§11.2).
- **Reintroduction condition for `skill_id`:** the day SIFT and CRAAP both ship and proficiency should transfer between them. That is a field added to the card registry — configuration, not schema surgery — so deferring is cheap.
- **The failure mode of deferring errs safe:** without a skill layer, a student proficient at CRAAP is treated as a novice when SIFT surfaces. That is *over*-scaffolding, which annoys; the opposite error, under-scaffolding, harms. Same asymmetric-cost reasoning as D3.

## 16. Remaining open questions

**16.1 — Blocking the Course side only: what is the relationship between a course and a card?** *(Owner: education designer. Not yet decided.)*
Is a course *composed of* card practices, or does it reference cards some other way? This determines the implementation of `resolve_remediation(card_id)` (§11.2) and the provisional status of D7. **It does not block the Workbench build** — the Workbench emits `card_id` and consumes the resolver. Revisit D7 and the Course spec once decided.

Everything below is non-blocking:

1. What is the re-entry experience after a mid-project Course detour (F5)? Drop the student straight back into the artifact, or show a one-line "here's what you just learned, now apply it" bridge? The bridge is probably worth it, but it must not read as a quiz.
2. What is the smallest paste the review engine can say something useful about (§7.6)? A single paragraph, or does diagnosis need surrounding context? This sets the invitation copy on the checklist.
3. What is the *minimum* recognition content that clears the §7.1 acceptance bar for a long-tail EE question, where nothing is hand-authored?
4. Does the Chinese coaching toggle apply to interventions only, or also to the checklist and rubric vocabulary? (Rubric vocabulary is the exam's own; translating it may harm transfer to the exam.)
5. What is the retention/deletion policy for the Process Record, given it is simultaneously integrity evidence and student personal data?

## 17. Handoff

### For the designer — build next, in this order
1. **First-entry recognition moment** (§7.1). The ten seconds that decide whether this feels like "it knows my assignment" or "another AI tool." Highest-value screen in the product.
2. **The main workspace** (§7.3): left checklist rail with four item states; Think view as **slots + scratch** (D1); Draft view showing immutable snapshots with **read-only** span-anchored margin comments — the absence of any editing or "insert" affordance must be visually obvious, it is a feature, not an omission (R-1, §12.1).
3. **The write-elsewhere loop** (§7.6): the export-outline handoff, the paste-back moment, and the per-section paste invitation. Getting the checklist to invite small, frequent pastes — rather than one whole-essay dump at the deadline — is what keeps this product from degrading into an essay grader.
4. **Coach rail**: single next action, never a menu (F2); artifact-scoped conversation only, no global chat (R-8).
5. **Card in use** (R-9): AI's span-anchored questions on the student's real material → student's own questions → the abstract framework revealed at summing-up. This is the pedagogical heart; give it the most iterations.
6. **Returning session** situated-memory screen (§7.4).
7. **The stuck → learn → return loop** (F5, §11.2): how the coach offers a Course exercise without it feeling like a detour or a punishment, and how the student lands back on the exact artifact afterwards.
8. **Submit-for-review results** as a *work order*, not a report card (§10) — bands, evidence, deep links to fix.

### For the coder — build next, in this order
1. **State Evaluator** (§11.1) — one engine for next-action, card surfacing, and fade. Instrument the north-star signal from day one (§13).
2. **Card contract + registry**, shared with Course; `card_competence` store keyed by `card_id`; call the Course module's `resolve_remediation(card_id)` behind an interface, and handle `null` by falling back to coaching. Do not couple to the Course's internal composition — it is undecided (§11.2, §16.1, F5).
3. **R-1 enforcement**: typed output schema, no insert affordance, validator, audit log (§11.3).
4. **Exam config loader** with the zero-code acceptance criterion (§11.4).
5. **Artifact versioning → Process Record projection → PPF export** (§9).
6. **Draft Snapshot store + read-only annotated viewer**: immutable snapshots, span anchoring, link-on-paste to map nodes, snapshot diffs for the Process Record (§7.6, §12.1). Cheap — and it must never grow into an editor.

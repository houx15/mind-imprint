# Writing Skills Research and Coder Handoff

Research date: September 10, 2026  
Audience: Developer extending an existing student writing product  
Scope: Shared writing support, Chinese writing, English writing, and implementation recommendations

## 1. Product context and main recommendation

The product already follows this process:

**An initial idea → discussion that develops the idea → short pieces of writing → revision and organization → a complete piece.**

The purpose of this research is to identify useful mechanisms for individual steps in that process. A repository does not need to implement the entire process to be valuable.

Keep the existing workflow and selectively adapt the strongest mechanisms:

- **Shared discussion:** Idea Interviewer and Story Coach.
- **Chinese writing:** Master Writing Skill for learning and practising specific writing techniques; My Literary Moment for eliciting real material; Qifeng Writing for selecting an appropriate revision task.
- **English writing:** Writing Scaffolding for paragraph support; Coach IELTS Writing for language diagnosis and targeted practice; Explicit Instruction Sequence Builder for moving from demonstration to independent writing.
- **Building the complete piece:** Draft Review Kit for structural checks, combined with a new interaction that lets students organize their own fragments.

In this document, a **fragment** means a short piece of student writing: a sentence, a few sentences, a scene, or a paragraph. Its size should depend on the learner and the immediate task.

The workflow should allow movement in both directions. Writing a fragment may reveal a better idea; reviewing several fragments may reveal missing material that requires another discussion. A complete outline does not have to precede the first fragment.

### Evidence status

The findings below come from inspection of public GitHub READMEs, `SKILL.md` files, and selected supporting references. Research combined English and Chinese web searches with GitHub repository searches and inspection of linked files.

No common multi-turn benchmark, classroom trial, or learning-outcome evaluation was run. “Priority” means relevance to this product and specificity of the documented mechanism. It does not mean proven effectiveness, reliable grading, or verified installation compatibility. This search is not exhaustive.

**Observed mechanism** describes what a source instructs an agent to do. **Recommended adaptation** describes our proposed use in this product. Adaptations are not claims that the repositories already implement them.

## 2. Shared mechanisms

These mechanisms can support both writing languages, even where the original instructions or examples are in English. Cross-language use still needs testing.

| Repository / Skill | Observed mechanism | Fit in our process | Recommended adaptation and limitations |
|---|---|---|---|
| [nadiem99/claude-writing-skills — Idea Interviewer](https://github.com/nadiem99/claude-writing-skills/blob/main/skills/pipeline/interview/SKILL.md) | Asks one question at a time; develops a concrete experience into a claim, explanation, evidence, counterargument, and reader relevance. Reads existing material and saves interview material. | Initial idea → discussion | Reuse response-dependent questioning and the preservation of student material. Allow an earlier transition to a fragment. The original typically runs 8–12 questions before outlining and sometimes pushes the writer to take a firm position; neither behavior should be universal for students. |
| [jwynia/agent-skills — Story Coach](https://github.com/jwynia/agent-skills/blob/main/skills/creative/fiction/core/story-coach/SKILL.md) | Diagnoses where the writer is stuck, asks relevant questions, introduces a framework when useful, and returns the writer to a concrete writing action. Explicitly avoids generating story prose. | Discussion → fragment; fragment → next attempt | Reuse the requirement that coaching leads back to student writing. Adapt fiction-specific questions for other genres. Its strict ban on drafting needs a carefully limited exception for teaching demonstrations, particularly in English language instruction. |
| [GarethManning/education-agent-skills — Self-Efficacy Builder Sequence](https://github.com/GarethManning/education-agent-skills/blob/main/skills/wellbeing-motivation-agency/self-efficacy-builder-sequence/SKILL.md) | Includes a worked sequence from speaking and seeing a transcription, to writing one sentence, three sentences, a paragraph with a verbal plan, and an independent paragraph. Reduces task size when the learner struggles. | Discussion → first written fragment | Useful when a student can explain an idea orally but avoids writing. It is a teacher planning Skill, with an example spanning weeks. It explicitly does not solve all underlying language or cognitive difficulties. Adapt the task progression, not the complete classroom script. |
| [EveryInc/draft-review-kit — Developmental Edit](https://github.com/EveryInc/draft-review-kit/blob/main/skills/dev-edit/SKILL.md) | Summarizes each subsection in one sentence, flags material that does not serve it, and compares the piece's actual message with the opening's promise. | Multiple fragments → coherent complete piece | Turn editorial diagnostics into student decisions about purpose, order, missing material, and revision. The original reviews drafts; it does not provide the complete fragment-assembly interaction. Its evidence checklist also needs an additional check of source reliability and relevance. |
| [nadiem99/claude-writing-skills — Writing Coach](https://github.com/nadiem99/claude-writing-skills/blob/main/skills/pipeline/coach/SKILL.md) | Checks thesis, structure, and argument before evaluating style; provides text-specific feedback and prioritizes major issues. | Fragment review; complete draft review | Reuse the order of diagnosis. Adapt the standards to genre and age. Its preferred openings and voice conventions reflect a particular adult publication style. |

### Shared interaction recommendations

1. Start from what the student has already said or written. Do not ask for information that is already available.
2. Default to one focused question or one writing action per turn. Select the next action from the student's response rather than a fixed interview checklist.
3. Move into writing when enough material exists for a useful fragment. Do not require every student to finish a full interview or outline first.
4. Preserve the student's original language and ideas across discussion, fragments, and revisions.
5. Diagnose the barrier before choosing support: missing material, unclear thinking, difficulty starting, language difficulty, or uncertainty about revision.
6. Keep demonstrations distinguishable from student work. Give the student a subsequent action that requires their own writing.

These are proposed product behaviors, not a tested combined workflow.

## 3. Chinese writing findings

Chinese writing support should emphasize idea development, real material, examples, structure, sentence choices, rhetoric, and revision. Vocabulary and grammatical accuracy still matter. Adult newsletter or social-media style should not become a universal standard for school writing.

| Priority | Repository | Observed mechanism | Recommended use | Limitations / adaptation required |
|---|---|---|---|---|
| High | [yutongcai0628/master-writing-skill](https://github.com/yutongcai0628/master-writing-skill) | Extracts writing techniques from a book or coherent long text and can generate a personal writing coach. The generated coach asks the learner to practise 1–2 techniques, write first, receive focused feedback, and compare revisions. | Connect exemplar reading to a specific fragment-writing task. | The main Skill analyzes source texts; coaching is specified in its generated-coach workflow. Prepare reusable method cards rather than requiring each student to run a full book analysis. |
| High for elicitation | [JustZeroX/skill-my-literary-moment](https://github.com/JustZeroX/skill-my-literary-moment) | Tracks experience, feeling, and reflection; asks one question at a time; skips known information; limits follow-ups; accepts “I don't know” or “I didn't feel much.” Restricts invented events and details. | Help students find concrete material for diaries, personal narratives, and observations. | The original writes the final prose after questioning. Replace that transition with a student fragment task. Do not import its entire adult prose style. |
| High for diagnosis | [Yuriloll/qifeng-writing](https://github.com/Yuriloll/qifeng-writing) | Maps observable problems to techniques across content, structure, narrative, argument, paragraphs, and sentences. Selects a small set of techniques relevant to the actual problem. | Choose the next revision task from the student's fragment. | Designed for adult articles and social posts, often with direct rewriting. Convert the selected technique into an action for the student. |
| Supporting | [tidego/chinese-prose-style-skill](https://github.com/tidego/chinese-prose-style-skill) | Prioritizes facts and author intent; checks the central question, paragraph focus, references, sentence relationships, terminology, and repetition. | Support clear explanations, arguments, and transitions between fragments. | An editorial rule set with limited teaching interaction. Add student attempts, hints, and revision checks. |
| Supporting | [chunyifish/cap-writing-coach](https://github.com/chunyifish/cap-writing-coach) | Gives textual evidence, prioritized revision suggestions, short examples, and self-check questions. Addresses task interpretation and material before surface language. | Reuse the feedback format and revision priorities. | Based on Taiwan's junior-high writing assessment framework. Do not apply its scoring directly to a different curriculum. Stronger for existing drafts than early idea discussion. |
| Framework reference | [hezkvectory/hermes-edu-skills — reading-writing](https://github.com/hezkvectory/hermes-edu-skills/tree/main/skills/reading-writing) | Provides primary, junior, and senior Chinese writing entries covering task interpretation, material selection, structure, fragments, feedback, and further practice. | Reference for organizing task types and grade-level entry points. | The inspected teaching instructions are brief and substantially similar across grades. Grade labels alone do not establish detailed developmental adaptation. |
| Lower for this workflow | [zdyya/writer-skill](https://github.com/zdyya/writer-skill) | Six-stage content workflow: idea clarification, research, drafting, fact-checking, visuals, and review. | Selectively reuse angle selection and material verification. | AI-generated drafting is central. Some modes impose adult stylistic conventions such as required casual expressions or cultural metaphors. These are not general student-writing requirements. |

### Chinese source files to read first

- [Master Writing: personal-coach-skill.md](https://github.com/yutongcai0628/master-writing-skill/blob/main/references/personal-coach-skill.md). Defines method cards, appropriate and inappropriate uses, short exercises, student-first drafting, and focused feedback.
- [Master Writing: scoring-rubric.md](https://github.com/yutongcai0628/master-writing-skill/blob/main/references/scoring-rubric.md). Specifies one effective technique, one major improvement, a one-sentence demonstration, a return to the exemplar, and second-draft comparison. Some example revisions add scene details; require student confirmation of such details in factual personal writing. Its scoring weights are author-defined.
- [Qifeng: technique-router.md](https://github.com/Yuriloll/qifeng-writing/blob/main/references/technique-router.md). Useful mappings include adjective-only characterization → actions and choices; unsupported claims → evidence and explanation; abrupt paragraphs → logical relationships and reference chains.
- [My Literary Moment: SKILL.md](https://github.com/JustZeroX/skill-my-literary-moment/blob/main/SKILL.md). Contains concrete rules for tracking known information, limiting questions, accepting sparse answers, and ending elicitation.

### Recommended Chinese feedback unit

For one fragment, identify a specific effective choice, select one high-impact issue, explain its effect on the reader, and ask for a bounded revision. Add a short demonstration only when it helps the learner act. Review whether the revision addressed the intended problem while preserving the student's meaning.

Rhetoric should serve a purpose in the current text. Avoid a default instruction to add a metaphor, quotation, parallel structure, or elevated ending to every fragment.

## 4. English writing findings

English writing needs the same support for ideas, material, and structure, plus help expressing those ideas in a second language. Separate the language used for instruction from the language of the writing task. A learner may discuss an idea in Chinese and then write the fragment in English.

| Priority | Repository | Observed mechanism | Recommended use | Limitations / adaptation required |
|---|---|---|---|---|
| High | [edu-ai-builders/language-learning-sop-kit — writing-scaffolding](https://github.com/edu-ai-builders/language-learning-sop-kit/tree/main/writing-scaffolding) | Produces an HTML sequence for thinking, brainstorming, choosing a structure, writing, and feedback. Each writing slot combines a Chinese instruction, selected arguments, an English starter, and a writing area. | Put the student's material and just enough language support beside the current fragment. | Some arguments are generated in advance and structures are relatively fixed. Feedback relies on keyword rules; connector presence does not prove coherent reasoning. Do not treat particular sentence patterns as automatic high-score signals. |
| High for targeted practice | [KevinYe0725/ielts-writing-coach — coach-ielts-writing](https://github.com/KevinYe0725/ielts-writing-coach/tree/main/.agents/skills/coach-ielts-writing) | Diagnoses one core target from draft evidence, explains a transferable rule, and sequences recognition, repair, generation in different contexts, and paragraph integration. Separates immediate application, delayed retention, and transfer. | Build a small language or reasoning exercise around a real problem in the current fragment. | The original begins with a complete IELTS Task 2 essay and includes a 60-minute practice paper with delayed feedback. Extract the capabilities and exercise patterns; redesign their size and timing for ongoing co-writing. Local Skill mode does not implement all web features. |
| High for teaching sequence | [GarethManning/education-agent-skills — explicit-instruction-sequence-builder](https://github.com/GarethManning/education-agent-skills/blob/main/skills/explicit-instruction/explicit-instruction-sequence-builder/SKILL.md) | Models a writing decision, guides a new attempt with increasing student participation, then asks for independent production. Includes analytical topic sentences, sentence frames, and vocabulary support. | Teach one sentence or paragraph capability when the student needs more than a correction. | A teacher lesson-planning Skill; examples include English literature analysis. Convert classroom scripts into short student interactions. Do not import fixed classroom timing or success thresholds without evaluation. |
| Supporting | [24kchengYe/human-skill-tree — 01-k12-languages](https://github.com/24kchengYe/human-skill-tree/blob/master/skills/01-k12-languages/SKILL.md) | Uses noticing, explanation, practice, and production in context. Addresses articles, tense, agreement, and other difficulties for Chinese learners; recommends focusing on a small number of errors. | Reference for contextual language support and gradual reduction of scaffolds. | Broad teaching instructions rather than a detailed writing interaction. The inspected file lacks the usual `name`/`description` YAML header; installation compatibility was not verified. Exam and vocabulary-count claims were not validated. |
| Supporting | [wanziwan666-crypto/english-learning-tools — english-writing](https://github.com/wanziwan666-crypto/english-learning-tools/tree/main/english-writing) | Combines an expression collection, timed writing, annotations anchored to original text, revision and rewriting pages, and progress records. | Reference for original-text feedback, expression reuse, and version comparison. | IELTS essay orientation. The browser-to-agent workflow includes copying and pasting; it is not an immediately embeddable live writing component. |
| Framework reference | [hezkvectory/hermes-edu-skills — reading-writing](https://github.com/hezkvectory/hermes-edu-skills/tree/main/skills/reading-writing) | Junior and senior English writing entries cover task response, paragraph structure, grammar, linking, expression suggestions, and further practice. | Reference for task organization. | The inspected entries provide limited detail on error-specific hints, student repair, and follow-up branching. |
| Lower for this workflow | [caztangpro/grammar-skill](https://github.com/caztangpro/grammar-skill) | Presents the original text, a correction, error categories, rule explanations, and tips. | Borrow the concise explanation format where useful. | Requires checking every English message. That could interrupt idea exploration. It lacks a student-first repair sequence and transfer practice. |

### English source files to read first

- [IELTS Coach: exercise-contracts.md](https://github.com/KevinYe0725/ielts-writing-coach/blob/main/.agents/skills/coach-ielts-writing/references/exercise-contracts.md). Defines concrete capabilities: comparison structures, verb forms, sentence boundaries, agreement, articles, collocations, word forms, argument development, paragraph functions, and references.
- [IELTS Coach: lesson-design.md](https://github.com/KevinYe0725/ielts-writing-coach/blob/main/.agents/skills/coach-ielts-writing/references/lesson-design.md). Describes meaning-preserving repair, independent generation in different contexts, and integration into a paragraph.
- [Writing Scaffolding: SKILL.md](https://github.com/edu-ai-builders/language-learning-sop-kit/blob/main/writing-scaffolding/SKILL.md). Shows how a writing slot combines content, instructions, and language support, as well as the limitations of rule-based feedback.

### Recommended English support unit

First establish what the student means. Then determine whether the immediate barrier is vocabulary, collocation, sentence construction, grammatical accuracy, or content organization. Offer the smallest useful intervention, let the student attempt the wording, and check both meaning and language.

Keep hard errors separate from optional stylistic improvements. A simpler correct sentence may be appropriate; complexity should support the intended relationship rather than become a target in itself.

## 5. Implementation recommendations

This section is a proposal for the coder. It describes behavior to implement within the existing architecture; no application code or runtime was inspected for this handoff.

### 5.1 Keep a shared workflow with language-specific support

The shared workflow handles idea development, student material, fragment creation, revision, and assembly. Chinese and English support should change the diagnosis, teaching resources, and prompts used at each step.

| Current need | Shared next action | Chinese-specific support | English-specific support |
|---|---|---|---|
| A broad idea with little material | Ask for one useful detail, example, or reason | Real experiences, observation, perspective, suitable examples | Accept initial ideas in Chinese or simple English; separate idea development from wording |
| Enough material to start | Ask for a manageable fragment | A scene, action, explanation, or argument using the student's material | A sentence or short paragraph with optional vocabulary or a sentence starter |
| Student is stuck while writing | Diagnose the barrier and adjust the task | Material selection, detail, sentence choices, narrative or argument movement | Vocabulary, collocation, clause relationships, tense, agreement, or paragraph logic |
| Fragment needs revision | Select a high-impact issue and ask for a new attempt | Precision, reader effect, evidence, sentence rhythm, appropriate rhetoric | Meaning-preserving correction, sentence construction, logical linking, then optional style |
| Several fragments exist | Clarify their roles, compare orders, identify gaps | Narrative progression, argument sequence, emphasis, opening and ending | The same checks, plus references, tense consistency, and links between paragraphs |

### 5.2 Add an explicit fragment-to-complete-piece interaction

This is the largest gap in the inspected candidates. Many move from an outline to AI drafting, or review an already complete draft. The product needs students to participate in building the complete piece from their existing work.

Recommended sequence:

1. Show the existing fragments and ask what each contributes.
2. Let the student name or confirm each fragment's role.
3. Help the student choose an order and explain its effect on the reader.
4. Identify a missing explanation, example, scene, or connection.
5. Ask the student to write the missing fragment or transition.
6. Review the assembled piece for coherence and alignment with its intended purpose.

Preserve the original fragments. A proposed order or demonstrated transition should remain distinguishable from the student's chosen version. Draft Review Kit provides useful diagnostics for this step, but this complete interaction is our recommendation.

### 5.3 Retain enough context to make feedback specific

Use the application's existing data structures where possible. The following information is useful; it is not a mandated new database schema:

- Writing language, preferred instruction language, learner level, genre, and task constraints.
- The current idea, intended meaning, and relevant student-provided material.
- Fragment versions and their relationship to the student's source material.
- The active learning target and the exact text that motivated it.
- What support was provided: question, hint, word bank, starter, demonstration, or direct revision.
- The student's subsequent attempt and whether it addressed the target without changing the meaning.
- The proposed and student-selected arrangement of fragments, with remaining gaps.

These records allow the system to distinguish a supported correction from later independent use. Do not infer mastery from a polished AI-assisted draft.

### 5.4 Start with a small set of mechanisms

| Order | Mechanism to adapt | Main sources | Observable result |
|---|---|---|---|
| 1 | Response-dependent idea discussion and an early transition to writing | Idea Interviewer; Story Coach | The student produces a first fragment based on their own material. |
| 2 | One focused fragment-revision loop | Master Writing; Qifeng | Feedback identifies a concrete issue and the student attempts the revision. |
| 3 | English language support inside that loop | Writing Scaffolding; IELTS Coach; Explicit Instruction | The student writes or repairs a sentence with appropriate support and returns to the paragraph. |
| 4 | Student-controlled fragment assembly | Draft Review Kit; Chinese Prose Style | The student organizes existing fragments and writes a missing connection or section. |
| 5 | Reduced-support follow-up | IELTS Coach; teaching-sequence references | A later attempt shows what the student can do with less help. |

Use complete source folders when evaluating a Skill that refers to supporting files. Before copying third-party content into the product, check the license at the selected revision. Preserve a source revision for reproducible evaluation; the links in this report point to moving branches.

## 6. Illustrative interactions

The examples below are proposed combinations of the researched mechanisms. They were not produced by a test run. The Chinese example is translated into English for this handoff.

### Chinese: turn an abstract description into a concrete fragment

**Student idea:** “I want to write about how frugal my grandfather is.”

The assistant asks for one recent occasion that led to this description. The student supplies an object, an action, and the circumstances. The assistant uses those details to ask for two or three sentences before requiring an outline.

If the fragment repeatedly labels the grandfather as “frugal” without showing anything, the next task is to let an action convey the characteristic. A short exemplar can explain the technique. The student revises one sentence and adds another using their own material. The assistant checks the result, then invites another fragment or further discussion.

The assistant must not invent dialogue, motives, or sensory details to improve the prose. An ordinary feeling or an unresolved reflection is acceptable.

**Source combination:** My Literary Moment for elicitation; Master Writing for method-based practice and limited demonstration; Qifeng for diagnosis.

### English: express the intended relationship correctly

**Intended meaning:** “Taking the bus is cheap, but it is sometimes slow.”  
**Student attempt:** “Although buses are cheap, but they are sometimes slow.”

The assistant confirms that the student wants to contrast cost and speed. It focuses on how the subordinate clause connects to the main clause, provides a brief explanation and a different-content example if needed, and asks the student to repair the original sentence.

After the repair, the student adds an example and develops a short paragraph. Later, a different topic can provide a natural opportunity to use a similar relationship with less support. Immediate correction and later independent use are recorded separately.

**Source combination:** contextual language instruction, IELTS Coach's repair and transfer patterns, and Explicit Instruction's gradual handover.

## 7. Proposed evaluation before broader integration

These checks have not been run. Evaluate the selected mechanisms on the same scripted student situations, followed by real learner testing where feasible.

| Test situation | Behavior to inspect |
|---|---|
| Broad Chinese idea such as “perseverance matters” | Does the next question elicit usable material or a clearer claim? Does the assistant avoid prematurely supplying a complete essay? |
| Student has rich material but keeps discussing | Does the assistant recognize an opportunity to write a fragment? |
| Student says “I have no deeper reflection” | Does it accept the answer and continue without inventing a moral or emotion? |
| Chinese fragment has rhetoric but unclear meaning | Does feedback address meaning and relationships before adding stylistic devices? |
| English student has a clear idea but cannot form a sentence | Does it distinguish missing words from collocation or syntax difficulties? |
| English sentence contains several errors | Does it choose a manageable target and provide a chance for student repair? |
| Student repairs a sentence with help | Does later independent use remain separate from assisted success? |
| Student already has three fragments | Does it preserve them and support student choices about order, gaps, and additions? |

Record whether each turn uses the student's latest information, gives a feasible next action, produces student writing, preserves intended meaning, and makes assistance visible. Keep suggested behavior, observed behavior, and unobserved outcomes separate.

## 8. Suggested reading order for the coder

1. **Shared flow:** Idea Interviewer and Story Coach.
2. **Chinese fragment loop:** Master Writing's `personal-coach-skill.md` and `scoring-rubric.md`, then Qifeng's `technique-router.md`.
3. **English fragment loop:** Writing Scaffolding's `SKILL.md`, then IELTS Coach's `exercise-contracts.md` and `lesson-design.md`.
4. **Teaching when the learner needs more support:** the two Education Agent Skills linked above.
5. **Complete-piece assembly:** Draft Review Kit's `dev-edit` and Chinese Prose Style.

The immediate deliverable for implementation should be a tested path from a student's idea to their own first fragment, revision, and assembled piece, with separate Chinese and English support at the points where it is needed.

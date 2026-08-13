# Evaluation Report Structure — `EvaluationReport` v1

> **Status:** design spec (approved 2026-08-13). This defines the **standard data structure** of a mind-imprint process-evaluation report — the contract between (a) the *generation algorithm* (a colleague's work; **out of scope here**) that produces the report, and (b) the *renderer* (web + PDF) and *storage* that consume it.
>
> **Companion docs:** `docs/2026-08-13-evaluation-design.md` (the product/visual brief this spec formalizes) · `docs/2026-08-11-evaluation-and-teacher-code-map.md` (where evaluation code lives today) · `docs/2026-08-11-evaluation-data-storage-guide.md` (where data lands). The **dual-axis model** (D1–D6 / A1–A6 / lenses) is defined in `apps/api/internal/rubric/dualaxis.json` + `packages/contracts/src/rubric.ts` and is *referenced*, not redefined, here.

---

## 1. Purpose & scope

**This is Evaluation v1 — the real one.** There is **no backward compatibility**. Once `EvaluationReport` ships, the old split — `DualAxisReport` + `mirror` (你的思维印记) + `summary` — is **retired**; those three payloads fold into this single object.

**In scope (this spec):**
- The canonical `EvaluationReport` object: every field, its type, enum, constraints, and **source-of-truth tag**.
- The database shape for storing/retrieving it.
- The build sequence: static render → DB + retrieve path → write path stubbed.

**Out of scope (explicitly):**
- **How the report is generated.** A colleague is exploring that algorithm. This spec is the *output contract* they target — it tells them what fields to compute, generate, and link. It says nothing about *how*.
- The web/PDF *looking* (a separate design session builds on this).
- 你的思维印记 as a cross-project aggregate (see §8).

**One report = one project.** Teacher and student render the **same** object; only the surrounding chrome differs.

---

## 2. Principles carried from the product brief

1. **One canonical object.** Single source of truth = a Zod contract in `packages/contracts`; Go stores it as `jsonb` and **boundary-validates** (validates the outer envelope — `version`, ids, that sections are the right kind — while the deep shape's truth stays in the Zod contract). This mirrors the platform's existing card-envelope storage rule (see `AGENTS.md`).
2. **Facts are computed, prose is phrased.** Every numeric/date/count field is deterministically computed and the model must **not** invent it. The model only phrases narrative fields over those facts. This matches the platform-wide invariant (`compose_weekly.go`, `compose_parent_stage.go` validators today).
3. **Evidence links back.** Wherever the report cites the student's own work (D/A evidence, events, material usage, prompts), it carries a **structured reference** (`{id, label, ts?}`) so the UI renders standalone *and* can deep-link to the real chat turn / graph node / material.
4. **Two axes never combine into a total.** Depth (D1–D6, level 1–4) and Autonomy (A1–A6, band 0–5) are reported separately; there is no composite score. Levels/bands are shown by **color, not number** in the UI (the current model isn't calibrated enough to surface a bare digit) — but the number is still stored.

---

## 3. Conventions

### 3.1 Source-of-truth tag (every field)

| Tag | Meaning | Rule for the generator |
|---|---|---|
| **FACT** | Deterministically computed from the event log / project record. | Compute it. The model must never write or alter it. |
| **PROSE** | Narrative text the model writes. | Generate it, grounded in FACTs and REFs. |
| **MODEL** | A model *judgment* expressed as a bounded scalar/enum (e.g. a D-level 1–4, a prompt-quality label). | The model decides within the allowed range. |
| **REF** | A reference to another record, carrying a denormalized label. | Emit `{id, label, ts?}` (see §3.2). |

### 3.2 Reference shape (`Ref`)

Everywhere the report points at a real record:

```ts
type Ref = {
  id: string;        // FACT — event / material / message / card id, resolvable in the DB
  label: string;     // FACT-derived — short human text, baked in so the UI renders without a lookup
  ts?: string;       // FACT — ISO 8601, when relevant (events, prompts)
};
```

`id` prefixes name the referent so the UI knows where to deep-link: `event:*`, `material:*`, `message:*` (a chat turn), `node:*` (graph node), `card:*` (tool card / subagent). The renderer treats an unresolvable `id` as non-fatal — it still shows `label`.

### 3.3 Prose highlighting

Prose fields that call for keyword emphasis (`abstract.overview`, `abstract.suggestionParagraph`) use an inline **markdown-subset**: `**…**` marks emphasis, and that is the *only* markup honored. No separate highlight arrays; no other markdown (no headings/lists/links inside a prose field). The renderer bolds/tints emphasized spans.

### 3.4 Naming & format

- JSON keys are **camelCase** (matches existing `packages/contracts`).
- All timestamps are **ISO 8601 UTC** strings.
- Enums are lower-kebab strings (listed in full below); the renderer owns their display labels.

---

## 4. The `EvaluationReport` object

```ts
type EvaluationReport = {
  version: 1;                       // FACT — schema version, literal 1
  reportId: string;                 // FACT
  projectId: string;                // FACT
  student: { id: string; name: string };   // FACT

  basics: Basics;
  abstract: Abstract;
  events: EventEntry[];
  materials: MaterialEntry[];
  depth: DepthDimResult[];          // exactly D1–D6, in order
  autonomy: AutonomyDimResult[];    // exactly A1–A6, in order
  promptLens: PromptLens;
  toolUsage: ToolUsageEntry[];
  risks: RiskEntry[];

  generatedAt: string;              // FACT — when the generator produced this report
};
```

Section-by-section below. Every leaf field carries its tag.

### 4.1 `basics`

```ts
type Basics = {
  title: string;        // FACT — the project/paper title
  type: string;         // FACT — assignment type label, e.g. "Extended Essay", "AP Research", "IA"
  startDate: string;    // FACT — project created
  endDate: string | null; // FACT — project finished (null while unfinished)

  milestones: {         // FACT — each a nullable ISO timestamp; null = not yet reached
    started: string | null;           // project start
    frameworkFinished: string | null; // 立题完成、点击「生成计划」(generate_plan) 的时刻 — see gaps doc G1
    proposalFinished: string | null;  // 提案 done (writing_finish doc_kind=proposal)
    writingFinished: string | null;   // 正文 done (writing_finish doc_kind=essay)
    projectFinished: string | null;   // whole project done → evaluation
  };

  counters: {           // FACT — all deterministically counted
    aiTurns: number;         // ALL ai calls: coach chat + the 3 subagents + ai comments (llm_call count)
    materialsRead: number;   // materials the student actually opened/read
    wordsWritten: number;    // body-text length; CJK chars + non-CJK whitespace words — see gaps doc G6
    aiCommentCount: number;  // ai 批注 count — needs new storage, see gaps doc G2
    editCount: number;       // recorded edits/revisions (draft_snapshot.seq + revision events)
  };
};
```

> Milestone names align with the status machine (`studioflow.go`: Topic → Framework → Proposal → Essay → Review) and `writing_finish`. They are **derived facts**, not new state.

### 4.2 `abstract`

```ts
type Abstract = {
  overview: string;             // PROSE (highlight) — the key-journey paragraph
  materialSentence: string;     // PROSE — one sentence on source/material work
  writingSentence: string;      // PROSE — one sentence on the writing process
  aiSentence: string;           // PROSE — one sentence on AI-usage boundaries
  suggestionParagraph: string;  // PROSE (highlight) — next-practice suggestion, one paragraph
  suggestionSentences: string[];// PROSE — 3–4 concrete next-time sentences
  recommendedCourses: {         // 0–3 entries
    courseId: string;           // REF (course:*) — id
    reason: string;             // PROSE — why this course, for this student
  }[];
};
```

### 4.3 `events[]` — the merged timeline

One entry per meaningful **segment** of activity (chat exchange, a reading session, adding a graph node, a writing pass, a review), in chronological order. The generator **merges and summarizes** — an entry is a rolled-up segment, not one raw log line.

```ts
type EventEntry = {
  ts: string;         // FACT — ISO, segment start
  kind: EventKind;    // FACT — enum below
  summary: string;    // PROSE — < 20 words, what happened in this segment
  aiTurns: number;    // FACT — ai calls within this segment
  ref?: Ref;          // REF (optional) — anchor record for deep-link
};

type EventKind =
  | "chat"       // coach conversation
  | "reading"    // reading-room session
  | "graph"      // exploration-map action (add/relate a source or lead)
  | "writing"    // a writing/revision pass
  | "review"     // review / self-assessment / reflection
  | "milestone"; // a stage transition (framework/proposal/writing/project finished)
```

### 4.4 `materials[]`

```ts
type MaterialEntry = {
  materialId: string;   // REF (material:*) — id
  addedAt: string;      // FACT — when added
  source: string;       // FACT — title / publisher, e.g. "Nature Sustainability"
  url: string | null;   // FACT — link (null if none)
  usedIn: Ref | null;   // REF — the exploration question it supports (node:*); claim-level linkage is a gap, see gaps doc G3
  comment: string;      // PROSE — what it shows / can't show; its argumentative function
};
```

### 4.5 `depth[]` — Cognitive Depth (认知深度), D1–D6

Exactly six entries, `D1`…`D6`, in order. **Dimension names & definitions are NOT in the report** — they live in the rubric contract (`packages/contracts/src/rubric.ts`); the renderer joins by `id`. The report carries only this student's result per dimension.

```ts
type DepthDimResult = {
  id: "D1" | "D2" | "D3" | "D4" | "D5" | "D6";  // FACT
  level: 1 | 2 | 3 | 4;   // MODEL — Delphi L1–L4; shown as color, not number
  summary: string;        // PROSE — one sentence on this student's status on this dimension
  evidence: Ref[];        // REF — events/chats that support the judgment (constrained id selection — see gaps doc G5)
  suggestion: string;     // PROSE — one sentence of suggestion
};
```

Dimension map (definitions authoritative in `dualaxis.json`): **D1** 任务理解与问题表述 · **D2** 证据与信源 · **D3** 论证结构 · **D4** 视角与偏见 · **D5** 反馈处理与修订 · **D6** 反思与元认知.

### 4.6 `autonomy[]` — Intellectual Autonomy (智识自主), A1–A6

Exactly six entries, `A1`…`A6`, in order. Same pattern as depth, but a **band 0–5** instead of a level.

```ts
type AutonomyDimResult = {
  id: "A1" | "A2" | "A3" | "A4" | "A5" | "A6";  // FACT
  band: 0 | 1 | 2 | 3 | 4 | 5;   // MODEL — event-counted band; shown as color, not number
  summary: string;        // PROSE
  evidence: Ref[];        // REF
  suggestion: string;     // PROSE
};
```

Dimension map: **A1** 方向自主 · **A2** 发起自主 · **A3** 边界主权 · **A4** 对抗与检验 · **A5** 判断与署名 · **A6** 求真优先.

### 4.7 `promptLens`

A curated set of the student's own prompts worth praising or improving. **Candidate sources:** coach chat (`chat_message` role=`user`) plus the three student-facing subagents only — **review agent / card agent / reading-room subagent** (see gaps doc G4). Other internal subagents (search/placement) are excluded.

```ts
type PromptLens = {
  summary: string;        // PROSE — a paragraph on the student's prompting overall
  prompts: {              // 3–10 entries
    stage: string;        // FACT — phase/stage the prompt occurred in (e.g. "立题" / "写作")
    studentPrompt: Ref;   // REF (message:*) — id + the prompt text as label + ts
    quality: "good" | "needs-improvement"; // MODEL
    comment: string;      // PROSE — what makes it good / weak
    suggestion: string;   // PROSE — how to strengthen it
  }[];
};
```

### 4.8 `toolUsage[]` — tool cards & subagents

Covers tool cards **and the three subagents only** — review agent / card agent / reading-room subagent (same scope as §4.7, gaps doc G4).

```ts
type ToolUsageEntry = {
  toolId: string;   // REF (card:*) — tool-card id or subagent name
  name: string;     // FACT — display name
  stage: string;    // FACT — where in the journey it was used
  purpose: string;  // FACT — the tool's standing purpose (from the card registry)
  summary: string;  // PROSE — how THIS student used it, and to what effect
};
```

### 4.9 `risks[]` — risky-behavior summary

Behaviors that may brush against the platform's iron laws / good-practice norms.

```ts
type RiskEntry = {
  type: RiskType;      // FACT — enum
  behaviour: string;   // PROSE — what the student did (may cite a ref inline via label)
  ref?: Ref;           // REF (optional) — the anchoring record
  suggestion: string;  // PROSE — how to avoid/repair it
};

type RiskType =
  | "ai-ghostwrite"       // AI 代劳（正文/判断被代写）
  | "missing-source"      // 缺少信源
  | "argument-logic"      // 论证逻辑
  | "data-scope"          // 数据口径
  | "rabbit-hole-offtopic"; // 兔子洞跑题
```

---

## 5. Database design

### 5.1 Table

New migration (`apps/api/internal/store/migrations/`), one **latest report per project**:

```sql
CREATE TABLE evaluation_report (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES task(id) ON DELETE CASCADE,
    version      int  NOT NULL DEFAULT 1,
    report       jsonb NOT NULL,     -- the EvaluationReport object
    created_at   timestamptz NOT NULL DEFAULT now()
);
-- retrieval reads the newest row for a project
CREATE INDEX evaluation_report_project_created_idx
    ON evaluation_report (project_id, created_at DESC);
```

We **append** rows (a regenerate makes a new row); retrieval always takes the newest. (No unique-per-project constraint — keeps history cheap; if we later want single-latest semantics we add it then.)

### 5.2 Read path — **implement fully now**

- sqlc query `GetLatestEvaluationReport(project_id)` → newest `report` jsonb.
- `GET /api/v1/projects/{id}/evaluation-report` → `getEvaluationReport` handler → returns the stored object verbatim (Go boundary-validates the envelope on the way out). **Read-no-call:** never invokes a model, mirrors the existing `getAssessment` rule.
- Seed **one mock report row** (the Phoebe anchor, §7) so the read path returns real data for the static/demo build.

### 5.3 Write path — **stub / seam only now**

- Define `InsertEvaluationReport(project_id, version, report)` sqlc query and a thin internal seam (`storeEvaluationReport`) so it exists and is typed — **but do not build the generation logic.** That is the colleague's algorithm; this contract is the interface they fill. The finish-trigger wiring (`project_finish.go`) that *calls* the generator stays a stub returning "not yet implemented" until they land it.

### 5.4 Contract location

- `packages/contracts/src/evaluationReport.ts` — the Zod schema + inferred TS type = **single source of truth**.
- Go mirror: types under `apps/api/internal/studio` (or a new `internal/evalreport`), boundary-validation only; the deep contract truth stays in Zod.

---

## 6. Build sequence

1. **Contract + static render.** Land `evaluationReport.ts`; build the new report page from a **mock `EvaluationReport`** typed by it, replacing today's report content. Proves shape + looking with zero backend. *(Looking is designed in the next session; this step just wires the data in.)*
2. **DB + read path.** Migration + `GetLatestEvaluationReport` + `GET …/evaluation-report` + seed the mock row. Point the page at the endpoint.
3. **Write seam stubbed.** `InsertEvaluationReport` query + seam defined; generation left to the colleague.

Teacher end (`TeacherReportView.tsx`) consumes the same endpoint/object.

---

## 7. Worked example (Phoebe anchor)

A minimal-but-real instance (trimmed to one entry per list) using the canonical demo scenario — *中国是否让地球变得更可持续？* Real content, no lorem.

```json
{
  "version": 1,
  "reportId": "rep-9f2c",
  "projectId": "e691f53a-…",
  "student": { "id": "user:phoebe", "name": "Phoebe" },
  "basics": {
    "title": "中国是否让地球变得更可持续？",
    "type": "Extended Essay",
    "startDate": "2026-08-01T09:00:00Z",
    "endDate": "2026-08-06T16:30:00Z",
    "milestones": {
      "started": "2026-08-01T09:00:00Z",
      "frameworkFinished": "2026-08-02T11:00:00Z",
      "proposalFinished": "2026-08-03T14:00:00Z",
      "writingFinished": "2026-08-06T15:00:00Z",
      "projectFinished": "2026-08-06T16:30:00Z"
    },
    "counters": { "aiTurns": 42, "materialsRead": 7, "wordsWritten": 812, "aiCommentCount": 9, "editCount": 6 }
  },
  "abstract": {
    "overview": "Phoebe 在该项目中与 AI 深度配合，同时保留了完整的独立思考。她看到题目联想到最近读过的一篇公众号文章，**没有直接把它当作可靠信源**，而是持续追溯到 **NASA Ames、Nature Sustainability、OWID 排放数据**，并额外关注了反方论证。",
    "materialSentence": "Phoebe 共找到 7 条来源，并准确识别了各条来源之间的论证关系，厘清了每条能说明什么、不能说明什么。",
    "writingSentence": "学生共进行 6 次文章打磨，结论从「中国在可持续上全面领先」逐步收束到「在可再生投资上领先、但人均与历史排放仍是短板」。",
    "aiSentence": "学生明确了 AI 使用边界，写作与判断由自己完成，AI 只做辅助与建议。",
    "suggestionParagraph": "建议下一次**主动识别 materials 中与当前论证相关性较弱的来源**，尝试收进兔子洞；这样在论证写作时会更聚焦。",
    "suggestionSentences": [
      "如果继续扩展：可单独开一段讨论 per-capita / historical emissions，但不要塞进 800 词正文。",
      "若重新加入 MEE 材料：补齐真实 URL、打开理由、摘录与来源功能，并说明它是 policy context，不是独立结果证明。",
      "若更严谨地引用视频：保留观看时间点、人物、场景与自己的笔记，避免 AI 生成的视频细节。"
    ],
    "recommendedCourses": [
      { "courseId": "course:source-triage", "reason": "她的来源相关性判断已不错，这门课能把「哪些该进兔子洞」变成可复用的判断标准。" }
    ]
  },
  "events": [
    { "ts": "2026-08-01T09:05:00Z", "kind": "chat", "summary": "粘入公众号链接，与印记讨论题目边界，收敛出研究问题。", "aiTurns": 6, "ref": { "id": "message:t1", "label": "研究问题成形", "ts": "2026-08-01T09:20:00Z" } }
  ],
  "materials": [
    { "materialId": "material:nasa1", "addedAt": "2026-08-02T10:14:00Z", "source": "NASA Ames", "url": "https://…", "usedIn": { "id": "node:claim-2", "label": "植被增绿主张" }, "comment": "一手遥感数据，支撑「增绿」但不能直接证明可持续性。" }
  ],
  "depth": [
    { "id": "D1", "level": 3, "summary": "能把宽题目限定为可回答的子问题，并连接个人阅读经验。", "evidence": [ { "id": "message:t1", "label": "把大题拆为排放/投资/治理三条子问题", "ts": "2026-08-01T09:20:00Z" } ], "suggestion": "下次可显式写出被排除的子问题及原因。" }
  ],
  "autonomy": [
    { "id": "A3", "band": 4, "summary": "多次给 AI 设边界，拒绝代写并说明理由。", "evidence": [ { "id": "message:t18", "label": "拒绝让步段代写，要求只给结构提示", "ts": "2026-08-04T13:02:00Z" } ], "suggestion": "把边界口径固定成一句可复用的话，迁移到下个项目。" }
  ],
  "promptLens": {
    "summary": "Phoebe 的提问从宽泛逐步变得具体、带证据，尤其擅长要求 AI 给出反方。",
    "prompts": [
      { "stage": "写作", "studentPrompt": { "id": "message:t22", "label": "帮我找出这段论证里最容易被反驳的一步，并给出反例", "ts": "2026-08-04T13:40:00Z" }, "quality": "good", "comment": "主动求反、限定范围，不外包判断。", "suggestion": "可再补一句要求 AI 标注反例的信源强度。" }
    ]
  },
  "toolUsage": [
    { "toolId": "card:craap", "name": "CRAAP 溯源体检", "stage": "阅读", "purpose": "对来源做可靠性体检", "summary": "用它把公众号追溯到 NASA/Nature，剔除了两条转述来源。" }
  ],
  "risks": [
    { "type": "data-scope", "behaviour": "早期把「碳排放总量第一」与「人均排放」混用了一次。", "ref": { "id": "message:t9", "label": "总量/人均口径混用", "ts": "2026-08-03T10:10:00Z" }, "suggestion": "陈述排放时固定口径，先声明是总量还是人均。" }
  ],
  "generatedAt": "2026-08-06T16:31:00Z"
}
```

---

## 8. Open / deferred

- **你的思维印记 (top of 成长报告 tab)** is a **cross-project aggregate**, separate from this per-project `EvaluationReport`. For v1 the tab surfaces the latest report's `abstract.overview` there; a dedicated aggregate object is a later, separate design.
- **Lenses (6 过程 lenses)** from the dual-axis model are not a top-level section in v1 (the brief didn't ask for them as their own block). If needed later, they slot in beside `depth`/`autonomy` using the same result shape.
- **PDF export** structure (document-style, per-section intro ¶ + table) is designed in the visuals session; it reads from this same object.

---

## Content depth & visual design (approved 2026-08-13)

**Content depth.** The structure is locked — no new fields. But every field must be filled **abundantly**, to the depth of the two reference reports `docs/reference/2026-08-13-report-example-detail.md` and `-simple.md` (one depth for web and PDF; both render the same object). Concretely: `evidence[]` items carry the student's *actual quote* + a specific observation + the **boundary it respects** ("不能证明什么"), not a bare label; `materials[].comment` states what a source proves **and cannot** prove; dimension `summary` is the 观察小结; risks/prompts are framed as **observation, never a grade**. The pedagogical signature is the recurring **边界/cannot-support** framing.

**Visual reference:** `docs/reference/2026-08-13-eval-report-mockup.html` (open in a browser). Built on the live design system — implementers should lift its CSS/layout.

Key visual decisions:
- **Full-width**, warm-paper ground; left rail = a **ruler 目录** (9 ticks, scroll-synced, active tick enlarges + accent; click to jump).
- **Type scale = `tailwind.config` `mk-*` only** (display 32 / h1 24 / h2 18 / h3 16 / body-lg 16 / body 14 / small 12 / label 11 uppercase). No off-scale sizes.
- **Cards:** object cards = 10px radius + `shadow-mk-md`, no border; list rows (materials, dimensions, tools, risks) = hairline (8px + border + `shadow-mk-xs`).
- **Per-section visualization:** header = title + type + date + **milestone stepper** + 5 **counter tiles** (macaron-tinted Lucide icons); events = **vertical timeline** (color-coded kind dots + AI×N); materials = **credibility color-bar cards** + final-status chip + cannot-support line; **D (05) and A (06) are separate sections**, each a **2-column grid of dimension cards, default-expanded**, level shown by **color depth only** (D = lake/teal ramp L1–L4, A = taro/purple ramp 0–5) with a 浅→深 legend, **no number**; prompts = observation + D/A-domain chips (no good/bad grade); tools = gradient-cover cards; risks = type chip + behaviour + processing, warm-warning toned.
- Teacher end renders the same object/page in teacher chrome.

---

## Appendix A — Data sourcing map (feasible fields)

Where each field's value comes from in the live schema (see `docs/2026-08-11-evaluation-data-storage-guide.md`). Fields with an open sourcing gap are marked **→ Gn** and detailed in the gaps doc (`docs/2026-08-13-evaluation-data-gaps.md`).

| Field | Tag | Source |
|---|---|---|
| `basics.title` / `type` / `startDate` | FACT | `project.title` / `qualification` / `created_at` |
| `basics.endDate` | FACT | `evaluations.created_at` (project-finish) |
| `milestones.started` | FACT | `project.created_at` |
| `milestones.frameworkFinished` | FACT | generate_plan click **→ G1** |
| `milestones.proposalFinished` / `writingFinished` | FACT | `writing_finish` (doc_kind proposal / essay) |
| `milestones.projectFinished` | FACT | `evaluations` / project status → finished |
| `counters.aiTurns` | FACT | `llm_call` row count |
| `counters.materialsRead` | FACT | `reference.reading_status='done'` / `material_id` not null / `source_log_entry` |
| `counters.wordsWritten` | FACT | `draft_snapshot`/`edit_buffer` content **→ G6** (counting rule) |
| `counters.aiCommentCount` | FACT | **→ G2** (new storage needed) |
| `counters.editCount` | FACT | `draft_snapshot.seq` + revision-recording events |
| `abstract.*` | PROSE | flagship LLM over the journey (= today's mirror/summary) |
| `abstract.recommendedCourses[].courseId` | REF | course catalog |
| `events[]` | FACT+PROSE | `activity_log_entry` + `event` + `chat_message`, LLM-summarized per segment |
| `materials[]` | FACT+PROSE | `reference` (+ `exploration_lead` for `usedIn` **→ G3**) |
| `depth[]` / `autonomy[]` | MODEL+PROSE+REF | dual-axis evaluator over the journey; evidence via **→ G5** (constrained id selection) |
| `promptLens[]` | FACT+MODEL+PROSE | `chat_message` role=user + 3 subagents **→ G4** |
| `toolUsage[]` | FACT+PROSE | `card_instances` + card registry + 3 subagents **→ G4** |
| `risks[]` | FACT+PROSE+REF | LLM inference over `disposition` / `intervention.output_check_verdict` / `reference.decision` |

**Everything not marked → Gn is directly derivable from stored data today.** The six gaps are pre-work for the generation phase, tracked in `docs/2026-08-13-evaluation-data-gaps.md`.
```

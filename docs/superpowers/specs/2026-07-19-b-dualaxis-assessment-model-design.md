# B · DualAxis Assessment Model — Design Spec

> Replaces the flat 10-dimension CT rubric with the two-axis DualAxis model across
> all three session-report surfaces, in one spec covering all four layers
> (dual-axis taxonomy · SOLO per-round · 提示词透镜 · report layout).

**Status:** approved (brainstorm 2026-07-19). Follows A1/A2/A3 (all merged; see
`docs/superpowers/specs/2026-07-17-course-session-report-design.md`,
`…/2026-07-18-chat-session-report-design.md`,
`…/2026-07-18-a3-project-terminal-and-growth-history-design.md`).

**Authoritative reference:** `docs/EVALUATION-0717-Student-A-AI-Interaction-Report-v3-DualAxis.html`
— **content and structure only** (its visual styling does not carry; the report renders
in the app's existing report visual language). The user directed: *follow this reference
for the dimensions and the report structure.* Where this reference and the binding
`docs/design/思维印记_工作区.dc.html` (the 9-dim 能力素养 radar) conflict, **the DualAxis
reference wins** — an explicit user override of design-source-of-truth for this model.

---

## 0. Why

Slice 10 shipped an isolated flagship assessor scoring a **flat 10-dim CT rubric**
(`ct-rubric.json`, D1–D10, each L1–L4|NA + evidence) plus a growth narrative, feeding
all three session reports (project/course/chat) and A3's history hub. The DualAxis model
is a deliberate replacement of that taxonomy and reading: it separates **认知深度**
(scored) from **智识自主** (observed, never scored), forbids any cross-axis total, adds
**SOLO per-round** judging and a **提示词透镜** that grades the prompts rather than the
student. This settles the long-standing 9-vs-10-vs-6 dimension inconsistency (memory
`cross-surface-assessment`): the DualAxis 6-dim two-axis taxonomy becomes canonical.

**Downstream consequence (out of scope for B, recorded here):** this supersedes the
binding 9-dim 能力素养 radar (`工作区.dc.html:1565`). **C** (student-level ability model)
must redesign that radar to the DualAxis structure when it builds the cross-session
aggregation. B changes nothing in C; it only makes the per-session model DualAxis.

**No compatibility burden:** the product is not in use. The migration drops existing
evaluation rows and reshapes storage to the DualAxis shape. No `schemaVersion`, no union
type, no legacy renderer — one schema, one renderer, clean slate.

---

## 1. The model — dimensions, axes, scoring

Six dimensions keep stable IDs from the reference and gain an `axis` tag + a `scoring`
mode. Three scoring modes coexist by design.

### 第一轴 · 认知深度 — scored 0–3, subtotal / 12

| ID | 维度 | scoring |
|----|------|---------|
| D1 | 任务理解与问题表述 | 0–3 |
| D3 | 证据与信源意识 | 0–3 |
| D4 | 论证结构意识 | 0–3 |
| D5 | 反馈理解与修改理由 | 0–3 |

`depthAxis.subtotal = Σ(D1,D3,D4,D5)`, out of 12. **This is the only number in the
report.** It lives inside one axis; the axiom permits a within-axis subtotal and forbids
any cross-axis total.

### 第二轴 · 智识自主 — unscored (observation only)

| ID | 维度 | output |
|----|------|--------|
| D2 | 学生主体性 / AI 依赖度 | observation text; A类(能力锚定) signals vs 引导后 signals; 对手邀请 count |

No score field at all. Rendered with an 「观察」 badge.

### 跨轴 · 元认知 — descriptive (both facets)

| ID | 维度 | output |
|----|------|--------|
| D6 | 元认知与反思 | depth-facet SOLO level (e.g. L3) + autonomy-facet (自发 vs 引导后), in prose |

No standalone number. Rendered with a 「跨轴」 badge.

### The axiom — verbatim

Ships in the config, the assessor prompt, and the report header, byte-for-byte:

> 两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定

### Depth-dim 0–3 anchors (authored)

`0` = 证据不足 / 未见该行为(rendered neutrally, off the scored bar). `1`→`3` ascending.
Drawn where they overlap onto today's ladders (folding, e.g., 信源辨识 + 横向验证 into D3).

- **D1 任务理解与问题表述** — `1` 直接抛问题，不给背景或目标 · `2` 给一点背景，但目标/约束模糊 · `3` 主动交代任务背景、目标与验收标准，并能把绝对化命题改成有限定的判断。
- **D3 证据与信源意识** — `1` 完全信任来源，不追出处 · `2` 能追到原始来源并识别来源等级 · `3` 溯到一手出处、识别反方证据，并能谈来源立场与证据适用范围。
- **D4 论证结构意识** — `1` 把观点当事实，不分论点论据 · `2` 能拆 claim–evidence–reasoning · `3` 识别证据与结论间缺失的 warrant，并能构建让步段协调主张与反主张。
- **D5 反馈理解与修改理由** — `1` 被动接受或整段照搬 AI 输出 · `2` 能采纳部分建议 · `3` 能分别说明采纳与拒绝的理由，主体性清晰、先自改再求反馈。

Autonomy `D2` carries an `observationGuide`; cross-axis `D6` carries a `guide`; neither
has scored anchors.

---

## 2. Data flow

### 2.1 Input — enriched per-round stream

Today's `AssessmentInput.Timeline []string` (flattened `"order. type: text"`) cannot drive
SOLO or the lens. Add a structured, ordered per-round student-turn stream:

```go
// Round is one student turn in order: the student's prompt plus what the AI
// asked/did around it. Recovered from the event stream (student-message events),
// which already carries this data — today it is flattened into Timeline.
type Round struct {
    N          int    // 1-based round index in the session
    StudentPrompt string
    AiContext  string // the AI ask/act framing this turn (for judging 发起方)
}
```

`AssessmentInput` gains `Rounds []Round`. Existing fields (`CardUses`, `Dispositions`,
`GateProgress`, `SnapshotCount`, `WordCounts`, `ReviewBands`, `GraphSummary`, `Timeline`)
stay — they feed depth scoring and 建议. `BuildAssessmentInput` (and A1/A2's
`buildAssessmentInputFromEvidence`, which project/course/chat share) extends to populate
`Rounds` from the same event slice. Surface-agnostic: every surface produces an event
stream.

### 2.2 One isolated flagship call

A single `Assess` flagship call emits the **entire** structured report as one JSON.
Consistent with: 旗舰绝不降级 (via each surface's `EvalResolver`), the isolated-assessor
seam (own endpoints; never the coach loop), and cost recorded once per surface via the
existing `RecordLLMCall` with `Purpose:"assessment"` (cost recorded even on reject). No
second model call.

The system prompt carries: the dual-axis rubric ladders (depth anchors, autonomy guide,
cross guide), the axiom, the SOLO discipline note (准确性是门槛不是刻度；序数不作均值；
单次判层为事件级证据), and the 提示词透镜 tiers P0–P3. The user turn carries the digest +
the per-round stream.

### 2.3 Output DTO — axis-structured, no cross-axis total

```
Assessment {
  depthAxis    { dims:[ {code,name,score:0-3,evidence,promptEvidence?} ]  subtotal:int /12 }
  autonomyAxis { dim:{ code,name,observation, anchoredSignals[], promptedSignals[],
                       adversaryInvites:int, promptEvidence? } }
  crossAxis    { dim:{ code,name, depthLevel:SoloLevel, initiative, prose, promptEvidence? } }
  solo         [ {round, excerpt, level:SoloLevel, rationale, initiative:自发|引导后} ]
  promptLens   { directiveRounds:int, totalRounds:int, boundarySettings:int, adversaryInvites:int,
                 questions:[ {title,body} × 3 ],           // 一问/二问/三问
                 bestPrompt:{ round, quote, annotation },
                 takeaway:{ quote, annotation },           // 带走的一条升级提示 (P4 模板)
                 perRound:[ {round, tier:P0-P3, label} ] }
  timeline     [ {round, task, prompt, pTag, dimTags[]} ]  // 交互证据
  keyEvidence  [ {label, quote} ]                          // 关键原话
  guidance     { anchored, prompted, risk, nextSteps:[ {title,body} ] }  // 建议
  narrative    string   // 总览 summary
  axiom        string   // verbatim
  generatedAt  string
}
```

**Axiom enforced structurally, not only textually.** No field sums across axes. The only
number is `depthAxis.subtotal`, validated `== Σ depth scores` and `≤ 12`. `autonomyAxis`
and `crossAxis` carry no score field — the type makes 「两轴合成总分」 unrepresentable.

### 2.4 Error handling / enforcement

- Output must be JSON matching the DTO; parse failure → assessment rejected (cost recorded).
- Banned-phrasing (`enforcement.BannedPhrasing`) runs over **every free-text field**
  (`narrative` + all `evidence`/`observation`/`rationale`/`annotation`/`prose`/`quote`
  fields). Any hit rejects the whole report; cost still recorded.
- Missing/invalid depth score → `0` (证据不足). Missing SOLO/cross level → `NA`.
- Depth dims not returned by the model are emitted at score `0` in canonical order;
  the report always covers all six dims and both axes.

---

## 3. Config single-source + contracts

### 3.1 `packages/contracts/src/dualaxis.json` — the single source

Replaces `ct-rubric.json`. TS-imported by the web build, `go:embed`-ed by the Go engine,
mirrored by `make sync-rubric` (never hand-edit the Go copy). Shape:

```jsonc
{
  "id": "dualaxis",
  "name": "AI 批判性思维 · 双轴模型",
  "axiom": "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  "axes": {
    "depth":    { "name": "认知深度", "scoring": "score", "max": 3 },
    "autonomy": { "name": "智识自主", "scoring": "observation" },
    "cross":    { "name": "元认知",   "scoring": "descriptive" }
  },
  "dimensions": [
    { "id":"D1","axis":"depth","name":"任务理解与问题表述","anchors":{"0":"…","1":"…","2":"…","3":"…"} },
    { "id":"D3","axis":"depth","name":"证据与信源意识","anchors":{ … } },
    { "id":"D4","axis":"depth","name":"论证结构意识","anchors":{ … } },
    { "id":"D5","axis":"depth","name":"反馈理解与修改理由","anchors":{ … } },
    { "id":"D2","axis":"autonomy","name":"学生主体性 / AI 依赖度","observationGuide":"…" },
    { "id":"D6","axis":"cross","name":"元认知与反思","guide":"…" }
  ],
  "promptTiers": [
    {"tier":"P0","label":"应答轮"},{"tier":"P1","label":"要成品"},
    {"tier":"P2","label":"要判断"},{"tier":"P3","label":"要过程·设边界"}
  ],
  "soloLevels": [
    {"level":"L1","name":"单点"},{"level":"L2","name":"多点"},
    {"level":"L3","name":"关联"},{"level":"L4","name":"抽象扩展"}
  ]
}
```

Full anchor text per §1 is authored in the plan (no placeholders).

### 3.2 Go — `internal/rubric`

`rubric.go` parses the new shape: `Axis`, `Dimension{ID, Axis, Name, Anchors map, ObservationGuide, Guide}`,
`DualAxis{ID, Name, Axiom, Axes, Dimensions, PromptTiers, SoloLevels}`, `mustParse` panics
on a malformed embed. `CT()` → `Model()` (or keep `CT()` returning the DualAxis model —
name TBD in plan, one accessor). Helpers: `DepthDims()`, `AutonomyDim()`, `CrossDim()`.

### 3.3 Contracts — `rubric.ts`, `assessment.ts`

- `rubric.ts` — replace `CT_RUBRIC`/`FULL_RUBRIC` with the DualAxis model + typed
  validators (axis-tagged dims; `DepthScore` = int 0–3; `PromptTier` = enum P0–P3).
  Keep `SoloLevel` (L1–L4|NA) — now used by the SOLO layer and cross-axis `depthLevel`,
  not per-dimension. Import-time validation (`assertModelComplete`): every depth dim has
  all four 0–3 anchors; autonomy has `observationGuide`; cross has `guide`.
- `assessment.ts` — replace the flat `Assessment` with the §2.3 axis-structured schema.
  Widest single change; the reason B is one spec. Mirrors the Go `AssessmentDTO`
  byte-for-byte (camelCase).
- Sequence the removal of `ct-rubric.json` after its last importer is gone, so no commit
  is left red (the additive-then-remove ordering A3 used for `generateAssessment`).

---

## 4. Rendering + storage

### 4.1 One shared `<DualAxisReport>` component

Renders 总览 → 双轴读数 (three axis groups; each depth/autonomy/cross dimension card,
with its per-dim 提示词证据 line, `absent` variant when none) → SOLO 判层 table →
提示词透镜 (stats + 一问/二问/三问 + 本次最佳提示词 + 带走的升级提示 + 口径说明) →
交互证据 timeline (per-round: prompt + P-tag + dim-tags) → 关键原话 → 建议
(A类/引导后/风险 + 下一步). **思维导图 is deferred** (redundant with the project's existing
interactive argument-graph). Swapped into all four call sites — `ReviewView` (project),
`CourseReport`, `ChatReport`, `GrowthReport` (A3 history hub, embedded) — replacing the
flat renderer. DRY: one component, identical across surfaces (per "all three, uniform").
Layout = DualAxis reference structure, in the app's design tokens; icons inline SVG,
never lucide.

### 4.2 Storage — clean-slate migration

`evaluations.scores` (JSON) holds the report doc; the new shape is a richer nested doc in
the same column. A migration **clears existing evaluation rows** (seed/demo artifacts,
un-upgradable under one-time-no-regenerate) so only DualAxis rows exist going forward.
No `schemaVersion`, no union, no legacy renderer. If any column reshape genuinely
simplifies the DualAxis storage it may be included; default is JSON-doc-in-`scores`
unchanged, rows cleared.

---

## 5. Testing + invariants

**Go**
- `dualaxis.json` parse: axis tags present, every depth dim has complete 0–3 anchors,
  autonomy/cross carry their guides.
- `assess_test`: dual-axis JSON parses; missing/invalid depth score → 0, missing level → NA;
  banned-phrasing over every text field rejects the whole report (cost recorded); the
  **structural axiom checks** — no cross-axis total field exists, `subtotal == Σ depth
  scores` and `≤ 12`.
- `assess_input`: `Rounds` built correctly and in order from the event stream.
- Per-surface endpoint tests (project finish / course / chat assessment): each yields a
  DualAxis report at `tier=="flagship"`, cost-on-reject holds.
- Migration test: legacy rows cleared; table usable.

**Contracts**
- `rubric.ts` import-validation (parse + complete anchors), `assessment.ts` round-trip,
  Go↔TS DTO parity test.

**Web**
- `<DualAxisReport>` unit tests: every section renders; depth subtotal shown as `/12`;
  autonomy shows **no** score; axiom present verbatim; SOLO table rows; lens counts.
- Each of the four surfaces renders `<DualAxisReport>`.

**Invariants (→ plan Global Constraints, verbatim values)**
- **RL-5:** diagnostic only; no rank, no人级档位判定, no aggregate. The *only* number is
  the within-axis depth subtotal `/12`.
- **Axiom verbatim** in config + prompt + report header: 「两轴永不合成总分；单次会话为
  事件级证据，不构成人级档位判定」.
- **旗舰绝不降级:** assessor via `EvalResolver`; tests assert persisted `tier=="flagship"`.
- **Cost-on-reject:** every assessment call records `llm_call` (`Purpose:"assessment"`)
  even when rejected.
- **Single-source:** `dualaxis.json` is the one truth; Go embed byte-mirrors the TS import
  via `make sync-rubric`; never hand-edit the Go copy.
- **One flagship call, never the coach loop.**
- **Uniform across surfaces:** identical `<DualAxisReport>` for project/course/chat.

**Build/process constraints**
- `make sqlc` from `apps/api`; never hand-edit `internal/store/sqlc/*`. `make sync-rubric`
  after editing `dualaxis.json`.
- Full Go packages (`CGO_ENABLED=0 go test -p 1 ./...`, `DOCKER_HOST=unix:///var/run/docker.sock`),
  never `-run` subsets, for migration/query/projection/endpoint/config changes.
- Web/contracts tests from their own dirs.
- Direct-merge to `main` + push (no PR). Never `git add` a whole directory — name files
  (pre-existing `M package.json` + untracked user files under `docs/` and repo root are
  not ours).
- Icons inline SVG, never lucide-react.

---

## 6. Out of scope (deferred)

- **C** — student-level cross-session ability model + DualAxis radar redesign (this spec
  only makes the per-session model DualAxis; the 9-dim radar redesign is C's).
- **思维导图** section of the reference (redundant with the argument-graph).
- OPCVL / any second rubric.
- Non-blocking A3 carry-forwards (double-submit lock on finish, `CostNumeric(cost,true)`,
  untested `HasEntitlement`, blunt `/生成/` test) — unrelated to B.

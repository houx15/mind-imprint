# Spec B · 规范评估引擎 + 契约 — 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

**Goal:** 把评估生产者从旧 DualAxis 形状（4 深度维 0–3 + 单一自主观察 + SOLO + P0–P3 透镜）重塑成 Spec A 终稿的规范对象（D6 L1–L4 · A6 0–5 · 6 透镜 · 官方投影通用槽 · 交互证据 · 作品与过程），跨全部四层，clean-slate，保持全套件绿。

**Architecture:** 单一真相源 `dualaxis.json`（TS import + Go embed，`make sync-rubric` 镜像）→ `internal/rubric` 解析 → `internal/agent` 一次隔离旗舰调用 `AssessReport` 产出规范对象 → 存 `evaluations.scores`(JSON, =`agent.Report`) → `studio.ReportDTO`(+generatedAt) 上线 → 一处共享 `<DualAxisReport>` 渲染，四个学生面共用。形状定义在**三处并行**（Zod `dualAxisReport.ts`、Go `agent.Report`+`reportWire`、Go `studio.ReportDTO`）必须逐字节对齐。

**Tech Stack:** Go（net/http, pgx, sqlc, goose, `go:embed`）· React+Vite+TS+Zod · Postgres。

上游：`docs/superpowers/specs/2026-07-24-a-finalized-assessment-model-design.md`（模型终稿）· `docs/superpowers/specs/2026-07-24-b-canonical-assessment-engine-design.md`（B 设计）。

## Global Constraints（每个任务隐含继承，逐字取值）

- **公理 verbatim**（进 `dualaxis.json` + 评估器 system prompt + 报告 DTO 头）：`两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定`
- **RL-5：** 无跨轴聚合字段；深度无小计；唯一百分数字是 `officialProjection.readiness.score`（项目面，note 内含「不与 D/A 双轴合成」）。
- **三形状对齐：** Zod `DualAxisReport`（+generatedAt）⇄ Go `studio.ReportDTO`（+generatedAt）⇄ Go `agent.Report`（**无 generatedAt**，持久化形状）。camelCase 逐字节。
- **旗舰绝不降级：** 评估器经各面 `EvalResolver`；端点测试断言持久化 `tier=="flagship"`。
- **一次调用、绝不入 coach loop。** cost 经 `RecordLLMCall(Purpose:"assessment")` 记一次，**reject 也记**。
- **单一真相源：** 只改 `packages/contracts/src/dualaxis.json`，再 `make sync-rubric` 生成 Go 副本；**绝不手改** `apps/api/internal/rubric/dualaxis.json`。
- **禁止杜撰学生提示词：** 交互证据/透镜只能引「逐轮学生提示词」原文；某轮为空则该数组项省略。
- **enforcement.BannedPhrasing** 跑遍每个自由文本字段，任一命中拒整份（cost 已记）。
- **构建：** `make sqlc` from `apps/api`，绝不手改 `internal/store/sqlc/*`；`make sync-rubric` after editing `dualaxis.json`。
- **测试：** 改配置/迁移/查询/投影/端点一律跑**全包** `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`（绝不 `-run` 子集）；web `cd apps/web && npm test` + `npx tsc --noEmit`；契约 `cd packages/contracts && npm test` + `npx tsc --noEmit`。**子代理在前台跑测试**（后台会 stall）。
- **分支实现，不自动合 main**（等用户回来批 merge）。绝不 `git add` 整目录（`M package.json`、`docs/`、repo 根下未跟踪文件非本次产物——只 `git add` 具名文件）。图标 inline SVG，绝不 lucide-react。Bash hook 禁 `/dev/null` 重定向。

## 规范对象（三形状的权威字段表）

**Go `agent.Report`（持久化，无 generatedAt）：**
```go
type DepthDim struct {
    Code string `json:"code"`; Name string `json:"name"`
    Level string `json:"level"`                 // L1|L2|L3|L4|NA
    LevelRange string `json:"levelRange,omitempty"`
    Evidence string `json:"evidence"`; PromptEvidence string `json:"promptEvidence"`
}
type AutonomySignal struct {
    Code string `json:"code"`; Name string `json:"name"`
    Level int `json:"level"`                     // 0..5
    Opportunity string `json:"opportunity"`      // given_taken|given_not_taken|not_supplied
    Evidence string `json:"evidence"`; PromptEvidence string `json:"promptEvidence"`
}
type LensStat struct { Label string `json:"label"`; Value string `json:"value"` }
type Lens struct { Code string `json:"code"`; Name string `json:"name"`; Level int `json:"level"`; Evidence string `json:"evidence"` }
type PromptLens struct { Stats []LensStat `json:"stats"`; Lenses []Lens `json:"lenses"`; Note string `json:"note"` }
type InteractionRow struct { Round int `json:"round"`; Student string `json:"student"`; AiSummary string `json:"aiSummary"`; Signal string `json:"signal"` }
type NextStep struct { Title string `json:"title"`; Task string `json:"task"` }
type Guidance struct { NextSteps []NextStep `json:"nextSteps"` }
type OfficialComponent struct { Name, Judgement, Reason string }          // json: name/judgement/reason
type OfficialAlignment struct { Item, Standard, Performance, Impact string } // json: item/standard/performance/impact
type OfficialStandardRef struct { ID string `json:"id"`; Name string `json:"name"` }
type OfficialReadiness struct { Score int `json:"score"`; Note string `json:"note"` } // 0..100
type OfficialProjection struct {
    Standard OfficialStandardRef `json:"standard"`
    Components []OfficialComponent `json:"components"`
    Alignment []OfficialAlignment `json:"alignment"`
    Readiness OfficialReadiness `json:"readiness"`
}
type WorkSample struct { Title string `json:"title"`; Text string `json:"text"` }
type ProcessMaterial struct { Name, Status, Diagnosis string }             // json: name/status/diagnosis
type WorkAndProcess struct { WorkSamples []WorkSample `json:"workSamples"`; ProcessMaterials []ProcessMaterial `json:"processMaterials"` }
type Report struct {
    DepthAxis []DepthDim `json:"depthAxis"`
    AutonomyAxis []AutonomySignal `json:"autonomyAxis"`
    PromptLens PromptLens `json:"promptLens"`
    InteractionEvidence []InteractionRow `json:"interactionEvidence"`
    Narrative string `json:"narrative"`
    Guidance Guidance `json:"guidance"`
    Axiom string `json:"axiom"`
    OfficialProjection *OfficialProjection `json:"officialProjection,omitempty"`
    WorkAndProcess *WorkAndProcess `json:"workAndProcess,omitempty"`
}
```
**`studio.ReportDTO`** = 上述所有字段 + `GeneratedAt string \`json:"generatedAt"\``（由 `row.CreatedAt` 填）。
**Zod `DualAxisReport`** = 同 camelCase + `generatedAt: z.string()`；`officialProjection`/`workAndProcess` 为 `.optional()`。深度 `level:z.enum(["L1","L2","L3","L4","NA"])`；自主 `level:z.number().int().min(0).max(5)`，`opportunity:z.enum(["given_taken","given_not_taken","not_supplied"])`；lens `level:z.number().int().min(0).max(5)`；`readiness.score:z.number().int().min(0).max(100)`。全部 `.strict()`。
**`reportWire`（模型输出）**：depth dims 按 code（无 name/无 levelRange 可选）、autonomy signals 按 code（无 name）、lenses 按 code（无 name）；无 axiom、无 generatedAt。引擎补 name（rubric）+ axiom（config）。

---

### Task 1: 配置 `dualaxis.json` + Go `rubric` 包

**Files:**
- Modify: `packages/contracts/src/dualaxis.json`（全量重写为新形状，见下）
- Modify: `apps/api/internal/rubric/dualaxis.go`（类型 + 访问器重写）
- Modify: `apps/api/internal/rubric/dualaxis_test.go`（断言新形状）
- Generate: `apps/api/internal/rubric/dualaxis.json`（**经 `make sync-rubric`，不手改**）

**新 `dualaxis.json` 内容**（authored；源 Spec A §3–§6）：把 `/private/tmp/claude-501/-Users-houyuxin-08Coding-mind-imprint/6c7188f0-e2b4-48c0-976c-0f3048faf591/scratchpad/dualaxis-authored.json` 的内容作为权威文本写入 `packages/contracts/src/dualaxis.json`（顶层键：`id,name,axiom,depth[6],autonomy[6],autonomyBand,opportunityRule,lenses[6],lensNote,standards[1]`；每 depth 维含 `id,name,means,anchors{L1,L2,L3,L4}`，D6 额外 `reflectionRule`；每 autonomy 信号含 `id,name,means,event`；每 lens 含 `id,name,guide`；standards[0] = ap-research，含 `components[4]{name,scale,口径}` 与 `alignmentItems[5]`）。实现者必须逐字采用该文件文本，不得改写锚点措辞。

**Interfaces (Produces):**
```go
type DepthDim struct { ID, Name, Means string; Anchors map[string]string; ReflectionRule string }
type AutonomySignal struct { ID, Name, Means, Event string }
type Lens struct { ID, Name, Guide string }
type OfficialComponentSpec struct { Name, Scale, Kou string } // json: name/scale/口径
type OfficialStandard struct { ID, Name string; Components []OfficialComponentSpec; AlignmentItems []string }
type DualAxis struct {
    ID, Name, Axiom string
    Depth []DepthDim; Autonomy []AutonomySignal
    AutonomyBand, OpportunityRule string
    Lenses []Lens; LensNote string
    Standards []OfficialStandard
}
func Model() DualAxis
func DepthDims() []DepthDim
func AutonomySignals() []AutonomySignal
func Lenses() []Lens
func AutonomyBand() string
func Standard(id string) (OfficialStandard, bool)
```
删除旧 `Axis`/`AxisDim`/`PromptTier`/`SoloLevel`/`AutonomyDim()`/`CrossDim()`/`dimsByAxis`。

- [ ] **Step 1: 写失败测试** — `dualaxis_test.go`：`DepthDims()` 返回 6 且每维 `Anchors` 含 `L1/L2/L3/L4` 非空；`AutonomySignals()` 返回 6 且每信号 `Event` 非空；`Lenses()` 返回 6；`AutonomyBand()` 非空；`Standard("ap-research")` ok 且 `Components` 4 项、`AlignmentItems` 5 项；`Standard("nope")` !ok；`Model().Axiom == "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定"`。
- [ ] **Step 2: 跑测试确认失败**（编译失败/断言失败）。Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/rubric/`
- [ ] **Step 3: 写 `packages/contracts/src/dualaxis.json`** 新内容（authored 文本）。
- [ ] **Step 4: `make sync-rubric`** 生成 Go 副本。Run: `cd apps/api && make sync-rubric`
- [ ] **Step 5: 重写 `dualaxis.go`** 类型 + 访问器（上 Interfaces）。
- [ ] **Step 6: 跑测试确认通过。** Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/rubric/`
- [ ] **Step 7: Commit**（具名文件：两个 dualaxis.json + dualaxis.go + dualaxis_test.go）。

---

### Task 2: 契约 `rubric.ts` + `dualAxisReport.ts`

**Files:**
- Modify: `packages/contracts/src/rubric.ts`（模型 schema + `assertModelComplete` 改新形状；删 `SoloLevel`/`ScoredLevel`/`SOLO_LABELS`/`DualAxisDimension`/`DualAxisModel` 中过时部分，改为 depth/autonomy/lenses/standards）
- Modify: `packages/contracts/src/dualAxisReport.ts`（规范对象重写）
- Modify: `packages/contracts/test/rubric.test.ts`、`packages/contracts/test/dualAxisReport.test.ts`
- Check: `packages/contracts/src/growthHistory.ts`（嵌 `DualAxisReport`，形状变但引用不变；其 test fixture 需换新形状 → `packages/contracts/test/growthHistory.test.ts`）
- Check: `packages/contracts/src/index.ts`（barrel，导出名若变需同步）

**Interfaces (Consumes):** Task 1 的 `dualaxis.json` 形状。**(Produces):** `DualAxisReport` Zod（供 web 5 个 api 客户端 + Go parity）。

- [ ] **Step 1: 写失败测试** — `dualAxisReport.test.ts`：有效 sample（6 depth `level:"L3"`、6 autonomy `level:3,opportunity:"given_taken"`、promptLens `stats[3]`+`lenses[6]`+`note`、interactionEvidence、narrative、guidance.nextSteps、axiom、generatedAt，含 optional officialProjection+workAndProcess）parse 通过；缺 optional 超集 parse 通过；depth `level:"L9"` reject；autonomy `level:6` reject；`opportunity:"foo"` reject；额外字段 reject（strict）。`rubric.test.ts`：`assertModelComplete` 对完整模型通过、对缺锚点抛错。
- [ ] **Step 2: 跑确认失败。** Run: `cd packages/contracts && npm test`
- [ ] **Step 3: 重写 `rubric.ts` + `dualAxisReport.ts`**（按上「规范对象」Zod 定义；`.strict()`）。
- [ ] **Step 4: 更新 `growthHistory.test.ts` fixture** 为新 report 形状；`index.ts` 导出对齐。
- [ ] **Step 5: 跑确认通过。** Run: `cd packages/contracts && npm test && npx tsc --noEmit`
- [ ] **Step 6: Commit**（具名）。

---

### Task 3: Go 引擎 DTO + 生产者 `assess_report.go`

**Files:**
- Modify: `apps/api/internal/agent/assess_report.go`（DTO 全量重写为「规范对象」；`reportWire`；`AssessReport` 规范化/校验/enforcement/nil-guard）
- Modify: `apps/api/internal/agent/assess_report_test.go`

**Interfaces (Consumes):** `rubric.DualAxis` + `rubric.DepthDims/AutonomySignals/Lenses/Standard`。**(Produces):** `agent.Report`（上「规范对象」Go 定义）+ `AssessReport(ctx, prov, r, m rubric.DualAxis, in AssessmentInput) (Report, gateway.ChatUsage, error)`。

**规范化规则：**
- 深度：按 code 索引 wire；六维全覆盖，缺失→`Level:"NA"`；`Level` ∉ {L1,L2,L3,L4,NA} → "NA"；`Name` 从 rubric；`LevelRange` 透传。
- 自主：按 code 索引；六信号全覆盖，缺失→`Level:0, Opportunity:"not_supplied"`；`Level` clamp 0–5；`Opportunity` ∉ 三枚举 → "given_taken"；`Name` 从 rubric。
- 透镜：`Lenses` 按 code 六项全覆盖，缺失→level0，clamp 0–5，`Name` 从 rubric；`Stats` 透传（截/补至 3 项：不足留空 label/value，超出取前 3）；`Note` 从 config `lensNote` 填（引擎补）。
- 官方投影：仅当 `in.ProjectProjection` 为真时透传（`Readiness.Score` clamp 0–100；`Standard` 缺失→从 `rubric.Standard("ap-research")` 补 id/name）；否则置 `nil`。`WorkAndProcess` 同样仅项目面透传，否则 `nil`。
- 交互证据/guidance.nextSteps 透传。`Axiom` 从 `m.Axiom`。
- enforcement 跑遍：depth.Evidence/PromptEvidence、autonomy.Evidence/PromptEvidence、lens.Evidence、promptLens.Note、stats.Label/Value、interactionEvidence.Student/AiSummary/Signal、narrative、nextSteps.Title/Task、（项目面）officialProjection.components.Reason + alignment.Performance/Impact + readiness.Note、workAndProcess.workSamples.Text + processMaterials.Diagnosis。任一命中→拒整份（cost 已记，由调用方记）。
- `AnchoredNilGuards`：所有 slice nil→[]（DepthAxis/AutonomyAxis/PromptLens.Stats/PromptLens.Lenses/InteractionEvidence/Guidance.NextSteps；及项目面 Components/Alignment/WorkSamples/ProcessMaterials 当超集非 nil 时）。

- [ ] **Step 1: 写失败测试** — 新 `goodReportWire` fixture（6 depth by code、6 autonomy、promptLens stats+lenses、interactionEvidence、narrative、nextSteps；另一 fixture 带 officialProjection+workAndProcess）。断言：六 depth 全覆盖且 Name 从 rubric、缺失维 Level=="NA"、越界 level→NA；六 autonomy 全覆盖、缺失→level0/not_supplied、level 越界 clamp、opportunity 越界→given_taken；lenses 六项、level clamp；`in.ProjectProjection=false` 时 `OfficialProjection==nil && WorkAndProcess==nil`；`=true` 时透传且 readiness clamp 0–100；banned-phrasing 命中任一文本字段→err；Axiom == config 值；nil-guard 使空 slice 为 []。
- [ ] **Step 2: 跑确认失败。** Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/`
- [ ] **Step 3: 重写 `assess_report.go`**（DTO + wire + AssessReport + normalizers + nil-guard；删 `DepthAxis{Subtotal}`/`Solo`/`normSolo`/`normTier`/`clampScore`→改 `clampLevel`/旧 lens 类型）。
- [ ] **Step 4: 跑确认通过。** Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/`（**注意：** 此步会因 `assess_report_prompt.go`/`assess_input.go` 仍旧形而编译失败——Task 3 与 Task 4 同属 agent 包，实现者应把 Task 4 的 prompt/input 改动一并纳入使包编译，或按下方 Task 4 顺序连续完成后再整体验证。控制器可将 Task 3+4 合并为一次 dispatch。）
- [ ] **Step 5: Commit**（具名）。

> **控制器注记：** Task 3 与 Task 4 触碰同一 Go 包且相互依赖编译，**合并为一次 implementer dispatch**（先写 DTO+producer，再写 prompt+input，最后整体 `go test ./internal/agent/` 绿）。分列仅为逻辑清晰。

---

### Task 4: Go prompt `assess_report_prompt.go` + 输入 `assess_input.go`

**Files:**
- Modify: `apps/api/internal/agent/assess_report_prompt.go`（system prompt 从新 rubric 构建；输出格式 JSON 模板；user input 加项目作品片段）
- Modify: `apps/api/internal/agent/assess_input.go`（`AssessmentInput` 加 `ProjectProjection bool` + `WorkSamples []string`；`BuildAssessmentInput` 签名相应扩展，末位追加以减小 call-site 破坏）
- Modify: `apps/api/internal/agent/assess_input_test.go`

**新 system prompt 结构**（`assessReportSystemPrompt(m rubric.DualAxis, projectProjection bool)`）：
- posture（保留「AI 克制/禁杜撰/只输出 JSON」，改写双轴说明为：深度 D1–D6 判 L1–L4；自主 A1–A6 按计数带判 0–5；透镜对提示词判 0–5；公理 verbatim）。
- 逐维锚点：`for d in DepthDims(): "%s %s：L1 %s ｜ L2 %s ｜ L3 %s ｜ L4 %s"`。
- 自主计数带：`AutonomyBand()` + 每信号 `"%s %s：可计入事件=%s"`（`d.Event`）+ `OpportunityRule`。
- 透镜：每 `Lenses()` 项 `"%s %s：%s"`。
- 若 `projectProjection`：加官方投影段（`Standard("ap-research")` 的 components scale/口径 + alignmentItems + 「训练折算仅作作品就绪度，不与 D/A 合成」+ 作品与过程要点）。
- 输出格式 JSON 模板（wire 形；depthAxis 数组 by code；autonomyAxis 数组 by code 含 opportunity；promptLens{stats,lenses,note?}；interactionEvidence；narrative；guidance.nextSteps；**仅项目面** officialProjection+workAndProcess）。
- user input：现有 digest/rounds/... + `if len(in.WorkSamples)>0` 追加「作品片段」。

- [ ] **Step 1: 写失败测试**（`assess_input_test.go` + prompt 断言）：`AssessmentInput` 携带 `ProjectProjection`/`WorkSamples`；`assessReportSystemPrompt(m,true)` 含 "L4"、"计数带"、"官方投影"/"AP Research"、公理原文；`(m,false)` 不含官方投影段。
- [ ] **Step 2: 跑确认失败。**
- [ ] **Step 3: 改 `assess_input.go` + `assess_report_prompt.go`。**
- [ ] **Step 4: 跑确认通过。** Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/`（Task 3+4 合并 dispatch 时此为整体验证点，全 agent 包绿）。
- [ ] **Step 5: Commit**（具名）。

---

### Task 5: `studio.ReportDTO` + 存储迁移

**Files:**
- Modify: `apps/api/internal/studio/report_dto.go`（`ReportDTO` = `agent.Report` 全字段 + `GeneratedAt`；`ToReportDTO(r agent.Report, createdAt time.Time) ReportDTO`）
- Modify: `apps/api/internal/studio/report_dto_test.go`
- Create: `apps/api/internal/store/migrations/0028_canonical_assessment_clean_slate.sql`（`DELETE FROM evaluations;` up；down 注释无法复原，用无害 no-op 或 `-- irreversible`）
- Test: 迁移经既有 migration 测试框架自动覆盖（`org_migrate_test.go` 类）；若无专门断言可加最小 up/down 可跑测试。

**Interfaces (Consumes):** `agent.Report`。**(Produces):** `ReportDTO`（Zod `DualAxisReport` 的 Go 镜像，+generatedAt）。

- [ ] **Step 1: 写失败测试** — `report_dto_test.go`：`ToReportDTO` 携全部新字段 + `GeneratedAt` 非空；marshal JSON 含 `"depthAxis"`/`"autonomyAxis"`/`"promptLens"`/`"interactionEvidence"`/`"generatedAt"`/`"axiom"`；**不含** `"subtotal"`/`"solo"`；autonomy 元素含 `"opportunity"`；officialProjection nil 时 JSON 无该键（omitempty）。
- [ ] **Step 2: 跑确认失败。** Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/`
- [ ] **Step 3: 重写 `report_dto.go`** + 建迁移 `0028`。
- [ ] **Step 4: 跑确认通过（全包，因迁移）。** Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/studio/ ./internal/store/`
- [ ] **Step 5: Commit**（具名）。

---

### Task 6: 输入构建器 + 端点接线（全 Go 绿）

**Files:**
- Modify: `apps/api/internal/api/assessment.go`（`buildAssessmentInputFromProject` 设 `ProjectProjection=true` + 填 `WorkSamples`（从草稿快照/作品片段，若无则空）；`reportDTOFromEvaluationRow` 随 DTO 变更）
- Modify: `apps/api/internal/api/course_assessment_input.go`（`buildAssessmentInputFromEvidence` 设 `ProjectProjection=false`）
- Modify: `apps/api/internal/api/chat_assessment.go`、`course_assessment.go`（若 `BuildAssessmentInput` 签名变，call-site 同步）
- Modify: 端点测试 fixtures：`apps/api/internal/api/assessment_test.go`、`course_assessment_input_test.go`、`project_finish_test.go`、`chat_assessment_test.go`/`course_assessment_test.go`（若存在，换新 wire fixture）
- Modify: `apps/api/internal/api/growth_history.go`（若直接 unmarshal `agent.Report`，字段自动兼容；确认无显式旧字段引用）

**Interfaces (Consumes):** Task 3/4 `AssessReport`/`AssessmentInput`；Task 5 `ReportDTO`。

- [ ] **Step 1: 更新端点测试 fixtures** 为新 wire 形；断言：project finish 返回新 `ReportDTO`（含 6 depth level、officialProjection 非空）；chat/course 返回 officialProjection 缺省；持久化 `tier=="flagship"`；cost-on-reject 保持。
- [ ] **Step 2: 跑确认失败。** Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/`
- [ ] **Step 3: 改输入构建器 + call-sites。**
- [ ] **Step 4: 跑全包确认通过。** Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
- [ ] **Step 5: Commit**（具名）。

---

### Task 7: Web 共享渲染器 `<DualAxisReport>` + 四面 + 客户端

**Files:**
- Modify: `apps/web/src/shell/report/DualAxisReport.tsx`（重写渲染新内核 + 项目超集；defer 证据地图）
- Modify: 共享 fixture 与四面测试：`apps/web/test/shell/report/DualAxisReport.test.tsx`、`test/shell/growth/GrowthReport.test.tsx`、`test/shell/chat/ChatReport.test.tsx`、`test/shell/courses/CourseReport.test.tsx`
- Check: api 客户端 `apps/web/src/api/{assessment,growth,chatAssessment,courseAssessment,projects}.ts`（类型源自契约，通常无需改代码；`npx tsc` 验证）

**渲染结构（学生投影，说人话）：**
- 总览：`narrative` + `axiom`（报告头 verbatim）。
- D 轴：六维卡，`level` 徽章（L1–L4，NA 显示「暂无可计入的证据」，`levelRange` 若有并显）、`evidence`、`promptEvidence`（有则）。**无小计。**
- A 轴：六信号卡，`level` 0–5 显示；`opportunity=="not_supplied"` → 显「暂无·机会未提供」不显数字；`evidence`。
- 提示词透镜：3 `stats` 卡 + 6 `lenses` 卡（level 0–5 + evidence）+ `note`。
- 交互证据：`interactionEvidence[]` 逐轮（round + student + aiSummary + signal）。
- 下一步：`guidance.nextSteps[]`（title + task）。
- **项目面**（`officialProjection` 存在）：官方投影（components 判定 + alignment 逐条 + readiness /100 + note）+ 作品与过程（workSamples + processMaterials）。**证据地图 defer**（Spec D）。
- 图标 inline SVG。

- [ ] **Step 1: 更新共享 fixture + 四面测试**（新 report 形；断言：D 轴显示 L1–L4 且**无 /12**；A 轴显示 0–5；opportunity not_supplied 显「机会未提供」；透镜 3 stats + 6 lenses；项目 fixture 显官方投影 readiness /100 + note 含「不与 D/A」；公理 verbatim；chat/course fixture 无官方投影）。
- [ ] **Step 2: 跑确认失败。** Run: `cd apps/web && npm test`
- [ ] **Step 3: 重写 `DualAxisReport.tsx`。**
- [ ] **Step 4: 跑确认通过 + 类型。** Run: `cd apps/web && npm test && npx tsc --noEmit`
- [ ] **Step 5: Commit**（具名）。

---

### Task 8: 全套件集成 + 收尾

**Files:** 无新增；跨包验证 + 补漏。

- [ ] **Step 1: `make sync-rubric` 确认 Go 副本最新**（Task 1 后若 config 再动）。
- [ ] **Step 2: 全 Go 包。** Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
- [ ] **Step 3: 契约。** Run: `cd packages/contracts && npm test && npx tsc --noEmit`
- [ ] **Step 4: Web。** Run: `cd apps/web && npm test && npx tsc --noEmit`
- [ ] **Step 5:** 若任一红：定位漏改消费者（对照 Explore map 的 15 测试文件清单），修复，重跑该套件全包。
- [ ] **Step 6:** 确认无遗留旧标识符：`grep -rn "subtotal\|SoloRow\|promptTiers\|AutonomyDim\|CrossDim\|depthLevel" apps/api/internal packages/contracts/src apps/web/src`（除迁移历史/注释外应为空）。
- [ ] **Step 7: Commit** 收尾（若有）。

## Self-Review 附注
- 三形状对齐：每次 DTO 改动后，Zod parse（web 客户端 strict）是最后防线——parity 测试 + 端点测试双证。
- clean-slate：0028 删行；无 schemaVersion/union/legacy 渲染器。
- 删除项（subtotal/solo/旧 lens/guidance 三字段）是「跟随终稿设计」的实质缩减，已在 Spec B §1 记录，留待用户回归可增补。
- 证据地图 defer 到 Spec D（过程图确定性投影，非评估器输出）。

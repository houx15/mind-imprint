# Spec B · 规范评估引擎 + 契约（canonical producer）— 设计

日期：2026-07-24
状态：设计草案（自主推进；用户离开前授权 "go ahead with plan and subagent driven implementation"）
上游：`docs/superpowers/specs/2026-07-24-a-finalized-assessment-model-design.md`（Spec A，模型终稿 + 投影规则 —— 本文的唯一真相源）。
程序分解：`docs/2026-07-24-assessment-teacher-end-program.md`。

## 0. 本 spec 做什么

把评估**生产者**从旧 DualAxis 形状（4 深度维 0–3 + 单一自主观察 + SOLO + P0–P3 透镜）重塑成 Spec A 的**规范对象**（D6 L1–L4 · A6 0–5 · 6 透镜 · 官方投影通用槽 · 交互证据 · 作品与过程），跨全部四层：`dualaxis.json` 配置 → `internal/rubric` → `internal/agent` 引擎+prompt → `packages/contracts` 契约 → 存储迁移 → **共享的 `<DualAxisReport>` 渲染器**（一处组件，四个学生面共用）。

**吸收原 Spec C 的机械部分。** clean-slate 契约重写会同时打断四个学生渲染面；无法在一个提交里让它们保持编译。沿用 2026-07-19 spec 的成例（"一 spec 覆盖四层，换进全部四个 call site"），B 把**共享渲染器**一并迁到新形状——四个学生面因共用这一个组件而同步一致。程序分解里的 "C" 由此在 B 内基本完成；C 若有剩余（面别专属打磨）另议。

## 1. 取代与删除（faithful to "follow the newest design"）

整体取代 2026-07-19 的已实现模型。**终稿设计里没有、因此删除：**
- 深度 `/12 小计`（`depthAxis.subtotal`）——深度改 L1–L4，无小计。
- 自主"绝不打分"——A 轴现 0–5 计带。
- **SOLO 逐轮判层**（`solo[]` + `soloLevels` 配置 + 跨轴 `crossAxis.depthLevel`）——终稿无此表；元认知并入 D6。
- 透镜的 `questions(一问/二问/三问)` / `bestPrompt` / `takeaway` / `perRound(P0–P3)` / `promptTiers` 配置——终稿透镜 = 3 统计卡 + 6 透镜卡。
- `guidance` 的 `anchored/prompted/risk`——终稿"下一步"仅 `nextSteps[]`。
- 单一 `autonomyAxis` 观察维、单一 `crossAxis` ——被 A1–A6 六信号取代。

> 这些删除是对既有学生内容的实质缩减，均出自"跟随终稿设计"。若用户回来想保留某项（如透镜的"带走一条提示"），可作 C 的增量补回。

## 2. 规范对象（Go DTO ⇄ Zod，camelCase 逐字节镜像）

```
Assessment {
  // ── 通用内核（chat/course/project 都产出）──
  depthAxis:    [ 6 × { code, name, level:"L1"|"L2"|"L3"|"L4"|"NA", levelRange?:string, evidence, promptEvidence } ]
  autonomyAxis: [ 6 × { code, name, level:int 0–5, opportunity:"given_taken"|"given_not_taken"|"not_supplied",
                        evidence, promptEvidence } ]
  promptLens:   { stats:[ 3 × {label, value} ],
                  lenses:[ 6 × {code, name, level:int 0–5, evidence} ],
                  note:string }
  interactionEvidence: [ {round:int, student, aiSummary, signal} ]
  narrative:    string
  guidance:     { nextSteps:[ {title, task} ] }
  axiom:        string
  generatedAt:  string

  // ── 项目专属超集（chat/course 为 null）──
  officialProjection?: { standard:{id, name},
                         components:[ {name, judgement, reason} ],
                         alignment:[ {item, standard, performance, impact} ],
                         readiness:{ score:int 0–100, note } }
  workAndProcess?:     { workSamples:[ {title, text} ],
                         processMaterials:[ {name, status, diagnosis} ] }
}
```

**公理结构化保证：** 无任何字段跨 D/A 相加；深度无小计；唯一百分数字是 `officialProjection.readiness.score`（项目面，作品就绪度，note 内固定标注"不与 D/A 双轴合成"）。

**NA / 机会供给语义：**
- 深度 `level:"NA"` = 暂无可计入的证据（不等于 L1）。可选 `levelRange`（如 "L3-L4"）仅供展示，`level` 取单一主级。
- 自主 `opportunity:"not_supplied"` = 平台未提供机会（欠账），投影渲染"暂无·机会未提供"，忽略 `level`；`given_not_taken` 才是学生信号（0/1 级）。

**`evidenceMap`（证据地图）不在本对象。** 它是**过程图的确定性投影**，在渲染 teacher 视图的 Spec D 里由图装配，非评估器输出——避免膨胀单次旗舰调用。

## 3. 配置 `packages/contracts/src/dualaxis.json`（单一真相源）

`go:embed` + TS import + `make sync-rubric` 镜像到 Go 端（绝不手改 Go 副本）。新形状：

```jsonc
{
  "id": "dualaxis",
  "name": "AI 批判性思维 · 双轴模型",
  "axiom": "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  "depth":   [ { "id":"D1","name":"任务理解与问题表述","anchors":{"L1":"…","L2":"…","L3":"…","L4":"…"} }, … 6 条 ],
  "autonomy":[ { "id":"A1","name":"方向自主","event":"一个可计入事件是…","bandGuide":"0级…1级…2级…3级…4级…5级…" }, … 6 条 ],
  "lenses":  [ { "id":"L_five","name":"五个决定完整度","guide":"…" }, … 6 条 ],
  "bands":   { "autonomy":"通用计数带：0级 0个；1级 1个偶发/引导后；2级 2–3个部分自发；3级 3–4个多自发跨多阶段；4级 ~5–6个自发跨≥3阶段部分形成修订；5级 ~7+持续自发跨全程稳定修订，顶格谨慎" },
  "standards":[ { "id":"ap-research","name":"AP Research",
                  "components":[ {"name":"Academic Paper","scale":"1–5 档"},
                                 {"name":"POD","scale":"/24（Research Design/3, Argument/6, Reflect/3, Engage/6, Oral Defense/6）"},
                                 {"name":"训练用折算","scale":"/100（作品就绪度，不与 D/A 合成）"},
                                 {"name":"诚信/真实性","scale":"低/中/高"} ],
                  "alignmentItems":[ "Through-course inquiry","Academic Paper 1–5 档","官方论文必要元素","POD 七行 rubric","PREP/真实性" ] } ]
}
```

全部锚点/带/口径文案在**计划**里 authored（无占位），源自 Spec A §3–§6。

## 4. Go — `internal/rubric`

`dualaxis.go` 解析新形状：`DepthDim{ID,Name,Anchors:map[L1..L4]}`、`AutonomySignal{ID,Name,Event,BandGuide}`、`Lens{ID,Name,Guide}`、`OfficialStandard{ID,Name,Components,AlignmentItems}`、`DualAxis{ID,Name,Axiom,Depth,Autonomy,Lenses,AutonomyBand,Standards}`。访问器：`DepthDims()`、`AutonomySignals()`、`Lenses()`、`Standard(id) (OfficialStandard, bool)`、`AutonomyBand()`。`mustParse` 对坏 embed panic。删除 `PromptTier`/`SoloLevel`/`AxisDim`/`AutonomyDim`/`CrossDim`。

## 5. Go — `internal/agent` 引擎 + prompt

### 5.1 输入
`AssessmentInput` 增 `ProjectProjection bool`（或 `Kind`）标记是否产出项目超集。`Rounds`/`CardUses`/`Dispositions`/`GateProgress`/`SnapshotCount`/`WordCounts`/`ReviewBands`/`GraphSummary`/`Timeline` 保留。项目面额外传 `WorkSamples`（作品片段）供作品与过程 + 官方投影判定（若事件流已有草稿快照/整稿体检可复用）。

### 5.2 一次隔离旗舰调用
`AssessReport` 仍是**一次** flagship JSON。system prompt 载：D1–D6 的 L1–L4 锚点、A1–A6 的计数带规则、6 透镜口径、公理、（仅项目）官方投影标准口径（`Standard("ap-research")`）。user turn 载 digest + 逐轮流（+ 项目作品片段）。cost 经 `RecordLLMCall(Purpose:"assessment")` 记一次，reject 也记。

### 5.3 输出 DTO + 校验
- 深度：按 code 索引模型返回，**六维全覆盖**（缺失→`level:"NA"`），`level` 规范到 `{L1,L2,L3,L4,NA}`（越界→NA），`levelRange` 透传。
- 自主：**六信号全覆盖**（缺失→`level:0, opportunity:"not_supplied"`），`level` clamp 0–5，`opportunity` 规范到三枚举（越界→`given_taken`）。
- 透镜：`lenses` 六项全覆盖（缺失→level 0），`level` clamp 0–5；`stats` 透传（3 项）。
- 官方投影：仅当 `ProjectProjection` 为真时产出并校验；`readiness.score` clamp 0–100；否则字段为 `nil`（chat/course）。
- 交互证据 / 作品与过程：透传，nil-guard 成 `[]`。
- **enforcement.BannedPhrasing** 跑遍所有自由文本字段（evidence/promptEvidence/narrative/nextSteps/interactionEvidence 三文本/官方投影 reason·performance·impact·note/作品与过程 text·diagnosis/透镜 evidence·note/stats.label）；任一命中拒整份（cost 已记）。
- 铁律·禁止杜撰学生提示词（沿用现 posture）：交互证据只能引逐轮原文，无则省略该轮。

## 6. 契约 `packages/contracts`

- `dualAxisReport.ts` 重写为 §2 对象（保留 `DualAxisReport` 名——仍是双轴）。深度 `level` = `z.enum(["L1","L2","L3","L4","NA"])`；自主 `level` = `z.number().int().min(0).max(5)`，`opportunity` = 三枚举；`officialProjection`/`workAndProcess` 为 `.optional()`（或 `.nullable()`）。删除 `PromptTier`/`SoloRow` 及 `subtotal`/`solo`/旧 lens 字段。import-time 校验保留在 `rubric.ts`（每深度维四锚点、每自主信号有 bandGuide、六透镜、standards 完整）。
- `rubric.ts` 随 `dualaxis.json` 新形状更新导出与校验；删除 `SoloLevel`/`PromptTier` 常量（除非别处仍用——见 Explore 消费者图）。
- Go↔TS DTO parity 测试更新。
- 移除 `dualaxis.json` 旧字段后，按"先加后删"顺序让每个提交不留红。

## 7. 存储

`evaluations.scores`(JSON) 仍存整份报告文档，新形状是更富的嵌套文档。迁移**清空既有 evaluation 行**（种子/demo 产物，one-time-no-regenerate 下不可升级），只留新形状。无 `schemaVersion`、无 union、无 legacy 渲染器。`make sqlc` 生成，绝不手改 sqlc 副本。最新迁移号见 Explore 图（在其后 +1）。

## 8. Web — 共享 `<DualAxisReport>`（学生投影）

一处组件、四面共用（ReviewView 项目 / CourseReport / ChatReport / GrowthReport）。渲染新内核：总览(narrative) → D 轴六维(L1–L4 徽章 + 证据 + 提示词证据) → A 轴六信号(0–5 + opportunity 态 + 证据) → 提示词透镜(3 统计 + 6 透镜) → 交互证据(逐轮) → 下一步。**项目面额外**渲染官方投影 + 作品与过程；**证据地图仍 defer**（与论证图冗余，且属 D）。学生 register 说人话。图标 inline SVG，绝不 lucide。web api 客户端类型随契约更新。

## 9. 测试 + 不变式（→ 计划 Global Constraints，逐字取值）

**Go**：`dualaxis.json` 解析（六深度维各四锚点、六自主信号有 band、六透镜、standards 完整）；`assess_test`（新 JSON 解析；缺失深度→NA、缺失自主→level0/not_supplied、缺失透镜→0；越界规范；官方投影仅项目面产出、非项目面为 nil；banned-phrasing 跑遍所有文本字段并拒整份 cost-on-reject；**结构公理**：无跨轴总分字段，`readiness.score` 是唯一百分数）；`assess_input`（`Rounds` 有序、`ProjectProjection` 正确）；每面端点测试产出规范对象且 `tier=="flagship"`；迁移测试（旧行清空、表可用）。
**契约**：`rubric.ts` import 校验、`dualAxisReport.ts` round-trip、Go↔TS parity。
**Web**：`<DualAxisReport>` 各段渲染；深度显示 L1–L4 且**无小计**；自主显示 0–5 且家长投影另测（Spec E）；官方投影仅项目面；公理 verbatim。四面各渲染该组件。

**不变式（verbatim）**：
- 公理「两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定」进 config + prompt + 报告头。
- RL-5：唯一数字是项目面 `readiness.score/100`；无跨轴聚合。
- 旗舰绝不降级（`EvalResolver`，测 `tier=="flagship"`）；一次调用、绝不入 coach loop。
- cost-on-reject：每次评估记 `llm_call`(`Purpose:"assessment"`) 即使 reject。
- 单一真相源：`dualaxis.json` 唯一，Go embed 经 `make sync-rubric` 逐字节镜像，绝不手改 Go 副本。
- 四面统一同一 `<DualAxisReport>`。

**构建/流程**：`make sqlc` / `make sync-rubric` from `apps/api`｜`packages/contracts`；改迁移/查询/投影/端点/配置一律跑**全包** `CGO_ENABLED=0 go test -p 1 ./...`（`DOCKER_HOST=unix:///var/run/docker.sock`），绝不 `-run` 子集；web/契约在各自目录测；**分支实现，不自动合 main**（等用户回来批 merge）；绝不 `git add` 整目录（`M package.json` 与 `docs/`、repo 根下未跟踪文件非本次产物）；图标 inline SVG。

## 10. 明确不做（延后）
- **证据地图**渲染与装配 → Spec D（过程图确定性投影）。
- 教师端四屏、班级聚合、使用/特征标签分析、👍/👎 → Spec D。
- 家长投影渲染 + 打印 → Spec E。
- 官方投影新标准（EPQ/EE）—— 只留通用槽，本期只 AP Research。
- A 计数带阈值的标注样本校准 —— 本期用 authored 带。

## 11. 留给计划/实现的决定（已定默认）
- 官方投影推导 = 旗舰整体判断（prompt 载 AP 标准口径），非机械公式。
- 单一 writing-project 模板下 `ProjectProjection` 恒真、standard 恒 `ap-research`；非项目面（chat/course）恒 nil。
- 旧契约一步切换、clean slate，无兼容期。

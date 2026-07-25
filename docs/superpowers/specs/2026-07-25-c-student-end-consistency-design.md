# Spec C · 学生端一致性（student-end consistency）· 设计文档

日期：2026-07-25
状态：设计定稿（待用户 review → 写实现计划 → SDD）
上游：`docs/2026-07-24-assessment-teacher-end-program.md`（路线图）· Spec A（终稿模型 `8627a65`）· Spec B（规范引擎+契约+学生渲染 `458dcba`）
性质：**纯投影收尾**。无 LLM、无迁移、无 sqlc、无新契约概念。全部读取引擎已产出的规范对象。

---

## 0. 背景与结论：×4 迁移已经完成

路线图把 Spec C 定为「四个学生报告面迁到规范对象的学生投影」。**这项迁移在 Spec B 里已经完成**：四个学生面已全部渲染共享 `<DualAxisReport>` 并消费新契约 `dualAxisReport.ts`：

| 学生面 | 位置 | 渲染 | 子集 |
|---|---|---|---|
| GrowthReport | 成长报告 → 学习记录 tab | `<DualAxisReport>` | 内核（历史条目自带 superset 时含之） |
| DualAxisReport（项目评估） | 项目 | `<DualAxisReport>` | 内核 + 项目超集 |
| ChatReport | 聊天报告 | `<DualAxisReport>` | 内核 |
| CourseReport | 课程报告 | `<DualAxisReport>` | 内核 |

因此 C 的原始主体已随 B 出货。**本 spec 只收尾 B 留下的三个口袋**——一个真正坏掉的面（能力素养）+ 两处 A 轴渲染缺口 + 一次一致性核查。

终稿模型 §9.3 明确学生投影 = 「学生看自己的内核（+ 自己项目的超集），说人话 register」；§10 注解**把学生端排除在说人话 lint 之外**（「学生端本就用部分术语作教学」）；§273 的「旧 DualAxis 契约弃用节奏」开放问题已被 B 的 clean-slate 迁移 0028 一步切换解决。三者共同把 bucket 3 压成一次只读核查。

---

## 1. 决策记录（DEC）

- **DEC-1 · 能力素养只修不重设计。** 保持它是 per-session 报告的确定性归并投影；不引入新概念、不调 LLM。用户明确选了 "repair + realign"，非 "redesign"。
- **DEC-2 · 删除元认知面板。** SOLO 已从规范对象消失；其真源 D6（反思与元认知）已经是六根深度辐条之一，也在逐维列表里。独立的「元认知 · SOLO 分布」面板冗余且带死数字，整体删除（含 Go 结构体与契约字段）。
- **DEC-3 · A 轴渲染修复仅动学生端。** 教师端 `TeacherReportView` 是 D 出货的平行渲染器，本 spec 不动它。`given_not_taken` 的「机会已给·未接住」在学生自省取景里最有意义（机会供给先于判定）。
- **DEC-4 · 诚实命名。** 能力素养里 `anchoredSignals`/`promptedSignals` 实际是 `given_taken`/`given_not_taken` 计数、UI 却标成「自发信号/引导后」——语义错误。改名为 `opportunitiesTaken`/`opportunitiesMissed`，UI 改「把握机会/错过机会」。
- **DEC-5 · 无迁移无 sqlc。** 能力素养是纯投影，读现有 `evaluations_by_user` 查询；契约形状变化不触库。但它仍是投影改动 → Go 跑全包。

---

## 2. Bucket 1 — 修复能力素养（person-level ability）模型

### 2.1 现状与坏点（B 遗留）

`internal/ability/ability.go` 头注释自陈：「real ability-model redesign deferred to Spec C」。具体坏点：

1. **雷达坏渲染（live bug）。** 深度在 B 里 4 维→6 维，但 `AbilityModel.tsx` 的雷达与轴标签写死 4 根辐条（`[-90, 0, 90, 180][i]`）。`i=4,5`（D5/D6）取到 `undefined` 角度 → `Math.cos(NaN)` → 多边形与文字标签落在 `NaN` 坐标。**当前用户可见地坏着。** `RADAR_MAX=3`（注释「0..3」）也与新的 1–4 序数不符。
2. **死数字。** `spontaneous`/`prompted` 结构性恒为 0（其源——已退役的 per-row SOLO Initiative tag——不复存在），UI 却在「SOLO 分布」标题下打印「自发 0 / 引导后 0」。
3. **误标 A 轴。** `anchoredSignals`/`promptedSignals` 是 `given_taken`/`given_not_taken` 计数，被 UI 标成「自发信号/引导后」。

### 2.2 Go `internal/ability/ability.go` 改动

**删除元认知整块：**
- 删 `Metacognition` 结构体（`HighestSolo` / `Distribution` / `Spontaneous` / `Prompted`）。
- 删 `Model.Metacognition` 字段。
- 删包级 `var soloLevels = []string{"L1","L2","L3","L4"}`。
- 删 `Aggregate` 里填 `Distribution`/`HighestSolo` 的 D6 循环与末尾的 highest 扫描。

**自主字段诚实改名（`AutonomyAbility`）：**
- `AnchoredSignals int `json:"anchoredSignals"`` → `OpportunitiesTaken int `json:"opportunitiesTaken"``（来源 `case "given_taken"`）。
- `PromptedSignals int `json:"promptedSignals"`` → `OpportunitiesMissed int `json:"opportunitiesMissed"``（来源 `case "given_not_taken"`）。
- `BoundarySettings`（A3 level 求和）/ `AdversaryInvites`（A4 level 求和）/ `Sessions` 保持不变——这些是诚实的 A3/A4 信号强度与会话计数。

**深度块保持不变**：仍是 6 维、`levelToInt` L1–L4→1–4、`-1`=证据不足（<2 次贡献会话）、`decay=0.6` 近因加权。

改动后 `Model`：
```go
type Model struct {
	TotalSessions int             `json:"totalSessions"`
	Depth         []DepthAbility  `json:"depth"`
	Autonomy      AutonomyAbility `json:"autonomy"`
}
```

### 2.3 契约 `packages/contracts/src/ability.ts` 改动

- 删 `metacognition` 对象。
- `autonomy`：`anchoredSignals`→`opportunitiesTaken`，`promptedSignals`→`opportunitiesMissed`；`boundarySettings`/`adversaryInvites`/`sessions` 不变。
- 修正 `AbilityDepth.level` 陈旧注释：`-1 = insufficient, else 0..3` → `else 1..4`。

### 2.4 Web `apps/web/src/shell/growth/AbilityModel.tsx` 改动

- **雷达 6 辐条化。** `radarPoints` 按索引算角度：`angle_i = -90 + i * (360 / levels.length)`（不再读写死数组）。三圈环形多边形改为传 6 个 `RADAR_MAX*ring` 值（用 `model.depth.length` 生成）。逐维文字标签循环同样按 `-90 + i*60` 计算，不再索引写死数组。
- **`RADAR_MAX` 3 → 4**；注释「depth scores are 0..3」→「depth ordinals are 1..4」；「depth radar over the 4 scored dims」→「over the 6 depth dims」。
- **删除「元认知 · SOLO 分布」面板**整块。
- **自主观察面板改标签**：`边界设定 ×{boundarySettings} · 对手邀请 ×{adversaryInvites} · 自发信号 {anchoredSignals} / 引导后 {promptedSignals}` → `边界设定 ×{boundarySettings} · 对手邀请 ×{adversaryInvites} · 把握机会 {opportunitiesTaken} / 错过机会 {opportunitiesMissed}`。「观察 · 不计分」徽章保留。

### 2.5 测试改动

- `internal/ability/ability_test.go`：删元认知断言（现 line ~100–104 的 `Spontaneous/Prompted == 0`）；把断言里的 `AnchoredSignals`/`PromptedSignals` 引用改为 `OpportunitiesTaken`/`OpportunitiesMissed`（值与语义不变——仍是 `given_taken`=3 / `given_not_taken`=1 之类）。深度雷达序数不变，深度断言不动。新增一条断言：`Model` 不再含 metacognition（编译层已保证；用一条「6 维 depth 顺序 = rubric 顺序」的既有/新断言守住深度长度=6，供雷达 6 辐条对齐）。
- `internal/api/ability_test.go`：若断言了 `metacognition` JSON 字段，删除之；改名字段的断言随之更新。
- 前端：`AbilityModel` 的渲染测试（若存在）更新为不再期待元认知面板、期待 6 辐条 polygon 点数、期待新标签文案。若当前无该组件测试，新增一个最小渲染测试：喂 6 维 depth + 自主计数，断言（a）radar polygon 有 6 个坐标对且无 `NaN`；（b）出现「把握机会/错过机会」文案；（c）不出现「SOLO 分布」。

---

## 3. Bucket 2 — A 轴渲染缺口（共享 `<DualAxisReport>`，仅学生端）

契约里 `AutonomySignal.opportunity = "given_taken" | "given_not_taken" | "not_supplied"`，且带 `promptEvidence`（均已由 B 定义，无需改契约）。

### 3.1 现状

`AutonomySignalCard`（`apps/web/src/shell/report/DualAxisReport.tsx:54`）只区分两态：
- `not_supplied` → 灰「暂无·机会未提供」。
- 其它（`given_taken` **和** `given_not_taken` 一并）→ 蓝「Lv N」。

且从不渲染 `a.promptEvidence`（深度维 `DepthDimCard` 有渲染，line 47–48）。

### 3.2 改动

**三态化 `AutonomySignalCard`**，按 `a.opportunity`：
- `not_supplied` → 灰「暂无·机会未提供」（不变）。
- `given_taken` → 蓝「Lv N」（不变）。
- `given_not_taken` → **新增琥珀态**：`Lv N` 数值旁一枚琥珀色徽章「机会已给·未接住」（fg `#B0682A` / bg `#F7ECDD` 一类暖琥珀，与既有蓝/灰区分）。让「机会供给先于判定」可读：一次 miss 读作 miss，而非低分。

**渲染 `promptEvidence`**：在 evidence 行之后，`a.promptEvidence` 非空时补一行「提示词证据：{a.promptEvidence}」，样式对齐 `DepthDimCard` 的 promptEvidence 行。

无契约改动、无后端改动。仅动 `DualAxisReport.tsx` 与其渲染测试。

### 3.3 测试

`DualAxisReport` 渲染测试新增/更新：喂三个自主信号分别为 `given_taken`/`given_not_taken`/`not_supplied`，断言（a）`given_not_taken` 卡出现「机会已给·未接住」而 `given_taken` 卡不出现；（b）带 `promptEvidence` 的自主信号渲染出「提示词证据：」行；（c）`not_supplied` 卡仍出现「暂无·机会未提供」。

---

## 4. Bucket 3 — 一致性核查（预期零代码）

一次只读核查，产出一段 checklist 记入本 spec 的执行报告；**只有查出真实缺口才升级为任务**，否则记「clean」收尾。核查项：

1. **子集正确**：ChatReport / CourseReport 传入的 report **不含** `officialProjection` / `workAndProcess`（内核-only）；项目 DualAxisReport 含超集。共享组件已用 `officialProjection ?` / `workAndProcess ?` 条件渲染，核查后端在 chat/course scope 确实不发这两块（Spec B 的项目超集约定）。
2. **无教师专属泄漏**：学生共享组件不出现教师专属取景（内部代码 D4/A5 作为「代码」展示、👍/👎 校准、证据侧滑抽屉）。学生端保留其既有 register（§10 注解），故**不**做说人话 lint。
3. **四面同源**：四面都吃同一 `DualAxisReport` 契约，无残留旧字段/旧视图读取。

若三项全过，Bucket 3 在实现计划里体现为一个「核查任务」，其交付物是核查记录本身。

---

## 5. 架构与不变式

- **一个生产者，多投影**：能力素养是 per-session `agent.Report[]` 的**跨会话确定性归并**投影；per-session 报告面是单会话投影。二者都不重算、不调模型。
- **RL-5 两轴永不合成总分**：能力素养三块（现为深度 + 自主，删元认知后两块）永不合成总分；深度 `level=-1`=证据不足；自主carry 计数不 carry 等级。共享报告里唯一的百分比仍是 `officialProjection.readiness.score` 且其 note 自带不合成免责。
- **机会供给先于判定**：Bucket 2 让 `given_not_taken`（机会已给·未接住）在视觉上独立于 `given_taken`——这正是该原则的落点。
- **敢于空白**：深度 `NA`/`-1`、自主 `not_supplied` 三态均已渲染「暂无」，不硬凑。
- 客户端绝不直连模型；本 spec 不新增任何 LLM 调用。

---

## 6. 文件清单与测试基线

**改动文件：**
- `apps/api/internal/ability/ability.go`（删元认知块 + 自主改名）
- `apps/api/internal/ability/ability_test.go`（删元认知断言 + 改名引用）
- `apps/api/internal/api/ability_test.go`（若断言元认知/旧字段名则更新）
- `packages/contracts/src/ability.ts`（删 metacognition + 自主改名 + 修注释）
- `apps/web/src/shell/growth/AbilityModel.tsx`（雷达 6 辐条 + 删面板 + 改标签）
- `apps/web/src/shell/report/DualAxisReport.tsx`（自主卡三态 + promptEvidence）
- 前端渲染测试（AbilityModel + DualAxisReport）

**不动：** 任何迁移 / sqlc / 后端查询 / 教师端 `TeacherReportView` / 契约 `dualAxisReport.ts`（其自主形状已够用）。

**测试基线（沿用既有约束）：**
- Go：`cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`（投影改动→全包，绝不 `-run` 子集）。**注意 subagent 的 Bash 工具 120s 自动后台化，`internal/api` 需 ~210s，dispatch 必须传 `timeout: 600000`。**
- 契约：`cd packages/contracts && npm test` + `npx tsc --noEmit`。
- Web：`cd apps/web && npm test` + `npx tsc --noEmit`。

---

## 7. 明确不做

- ❌ 能力素养重设计（新聚合 / 时间趋势 / per-surface 拆分）——用户选了 repair，非 redesign。
- ❌ 把 A 轴渲染修复镜像到教师端——DEC-3 学生端 only。
- ❌ 学生端说人话 lint——§10 注解豁免学生端。
- ❌ 任何评估内容新增——只投影 B 的产出。

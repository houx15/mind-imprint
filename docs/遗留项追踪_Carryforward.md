# 遗留项追踪 · Carry-forward Tracker（保证端到端不丢）

> 目的：把每个 slice 里**明确推迟**的事都登记到一个未来 slice，确保最终能跑出一个端到端可用的平台。
> 维护：每个 slice 完成时更新本表 + `slice-progress` 记忆。版本 2026-06-21（S3b 后）。

## 路线（剩余 slice）

| Slice | 块 | 一句话 | 端到端价值 |
|---|---|---|---|
| **S4**（进行中）| B5 | 过程树（确定性实时派生） | PRD §2 #3：过程长成可读的树 |
| **S5** | B6 | 评估那一刀 + 「你的思维印记」 | 护城河（PRD §11）；并做**语义树归并** |
| **S6** | B7 | 应用外壳 | 把全部串成可用 app（导航/主目录/记录页/设置页/mock 认证）|
| **3c**（later）| B1 扩展 | 富交互卡 | 10 张 stub 卡的真实交互 |

> 跑通 S4→S5→S6 即得到端到端：**新建任务 → 粘链接 → 陪练对话 → AI 调卡 → 填卡 → 过程树实时长 → 评估那一刀 → 记录页**。3c 是卡的广度扩展，不在端到端关键路径上（stub 卡当前可提议、可打开占位、可提交，优雅降级）。

## 遗留项 → 目标 slice（逐条登记）

| 遗留项 | 来自 | → 目标 slice | 端到端必需？ |
|---|---|---|---|
| **设置页**：真实 LLM 配置 UI（替代 dev `SettingsPanel`）| S2/S3b dev 入口 | **S6** | 是（用户得能填 key） |
| **非关闭式 key-gate 弹窗**（缺 `mk.llmConfig` 时强制先填+测连接）| S2.5 头脑风暴 | **S6** | 是（首跑体验） |
| **config 加固**：`mk.llmConfig` 改 zod 解析（去掉 `as`）；store mutators 返回 post-`parse` 值 | S2.5 | **S6**（设置页拥有 config）| 否（健壮性） |
| **主目录/首页**：任务列表 + 新建任务 + 粘文章链接 | 设计 HTML `showDirectory` | **S6** | 是（任务入口） |
| **目录↔工作区路由**（面包屑「返回所有任务」）| S3b | **S6** | 是 |
| **记录页**：`recCards`（卡使用计数，聚合 card_instance）/ `recLearning`（成长叙述，来自评估）/ `recAbility`（能力图——本期出轻量静态版，全技能树不做）| 设计 HTML `isRecords` | **S6** | 是（PRD §2 展示面）|
| **mock 认证**：login/register/bind 屏（点击即进，无后端）| 设计 HTML `authScreen` | **S6** | 否（demo 可跳过/mock）|
| **导航 a11y**（`role="tab"`/`aria-selected`）| S2.5/S6 | **S6** | 否 |
| **评估触发 + showEval/evalLoading「你的思维印记」UI** | PRD §11/§14 步骤7 | **S5** | 是（护城河）|
| **语义树归并**：`sub_question` 分枝 / `key_knowledge` / `attempt` 节点 + 归并打型（eval LLM 顺手做）| S4 | **S5** | 是（树的完整形态）|
| **评估增强树是否落库** vs 重派生 | S4 | **S5** 定 | 否 |
| **10 张 stub 卡的真实交互**（量表光谱/角色模拟/画布导图/分类标注/媒体回放/条件分支）| S3a | **3c** | 否（优雅降级）|
| **新原语**：`show_if`（条件步，aok-methods 需要）、spectrum/slider | S3a/3c | **3c** | 否 |
| **卡内容打磨**：emotional-alignment 运行期安全元数据（human_in_loop/privacy）、rabbit-hole 锚点 UX、learning-report 轨迹预填、checkpoint spot-error 字段 | S3a/3b review | **3c** | 否 |
| **一轮多卡**：re-feed 后续 `summon_card` 当前被丢弃（MVP 一轮一卡）| S3b review | **3c** | 否（MVP 足够）|
| **关卡=跳过不可逆**：scrim 误点即记 skipped；加「仅关闭不决定」语义 | S3b review | **3c** | 否 |
| **ChatLog index keys**；S1 raw-hex token 合并 / ITEM_COMPONENTS 去重 / ActiveSheet steps[0] 假设 | S3b/S1 review | **3c / cleanup** | 否（纯整洁）|

## 明确不做（非遗漏，是范围决定）

- 真后端 / 服务端 key / 计量（前端直连 BYO-key，§0.1 覆盖 PRD §16）。
- 真实登录鉴权（只 mock）。
- 全技能树（技能树本期不做；记录页 recAbility 至多轻量静态）。
- LLM 流式输出（非流式，陪练回复短）。
- 教师端 / 文档编辑器 / RAG 路由（PRD §4「明确不做」）。

## 结论

跑通 **S4 + S5 + S6** 即得到一个**端到端可用**的平台（覆盖 PRD §2 全部四条成功判据）。**3c** 与各 review 的 minor 项都是非阻塞的广度/打磨，登记在册、择期推进，不影响端到端主动脉。无孤儿项阻塞端到端。

# 卡片 / 透镜 · 落位地图（Card & Lens Placement Map）

> Authoritative map for "AI 工具卡在对的位置、以对的交互出现". Draft for discussion — decisions still open (see §6).
> Related: `2026-07-29-reading-lenses-adopt-demo.md` (the 9 lenses, shipped on `fix/reading-lenses`).

## 0. Two tool types, one placement rule

We have **two kinds of AI tools**, both summonable (by the AI, or later proposed in chat):

- **卡 (Cards)** — the T00–T38 canon: named thinking tools ("具体做什么"). ~36 built today.
- **透镜 (Lenses)** — the 9 disciplinary reading angles ("从什么角度看"). Shipped, KEPT.

**Placement rule:** a tool belongs to one or more **phases (rooms/moments)**, and **the phase supplies the interaction**. Placement is **many-to-many** — e.g. 问题漏斗 / 主张追问 / TOK四支柱 serve *both* 立题 and 写作; the same card renders in each room's own shape. A card's "home" is where it's most native; "also-fits" phases summon it too.

## 1. The phases (situations) and their native interaction

| # | 阶段 (room/moment) | 这一步在做什么 | 交互形态 |
|---|---|---|---|
| P1 | **立题**（动机·目标·proposal） | 把模糊动机收敛成可研究的问题 | 对话塑形 + 填写四问 |
| P2 | **计划**（plan） | 把问题拆成可执行的研究路径 | 生成/编排任务、里程碑 |
| P3 | **文献库**（reading list·探索资源） | 搜集、分诊、扩展相关资源 | **兔子洞探索树** + 检索/分诊卡 |
| P4 | **读文章**（reading room） | 深读一篇，逐句思考 | 选句 + 挂卡 + 你找证据（**卡 + 透镜都在此**） |
| P5 | **写作**（writing snippets） | 把观点搭成能立住的段落 | 构建/填充结构（图/表单） |
| P6 | **提纲**（outline） | 组织论证顺序与层级 | 排列/缩进结构 |
| P7 | **回顾**（review） | 复盘思考质量 **+ 复盘与 AI 的互动**（新增） | 反思 + 互动回溯 |
| P8 | **一般对话**（chat） | 情绪、提示词、AI 使用习惯 | 轻提示 |
| P0 | **AI 内部规则**（*非学生卡*） | Agent 自己的教学策略 | 系统 prompt / 路由，不进任何卡库 |

## 2. Master placement — our current cards → canon → phase(s)

Legend: ●=home · ○=also-fits · T#=canon id (≈ means our card is close but not an exact canon match) · **gap** flags a canon tool we haven't built.

| 我们的卡 (id) | 名称 | 约等于 canon | P1立题 | P2计划 | P3文献库 | P4读文章 | P5写作 | P6提纲 | P7回顾 | P8对话 |
|---|---|---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| craap | 信源辨识 CRAAP/CRRAAB | T01/T02 | | | ○ | ● | | | | |
| sift | 横向核查 SIFT | T04/T05 | | | ● | ● | | | | |
| fact-opinion-value | 事实/观点/价值 | T00 | | | | ● | ○ | | | |
| opcvl | OPCVL 史料评估 | T06 | | | ○ | ● | | | | |
| cda | 话语分析 CDA | T07 | | | | ● | | | | |
| spin-detector | 漂绿 + FLICC | T08/T16 | | | ○ | ● | | | | |
| argument-map | 论证地图（结构+谬误） | T09 | | | | ○ | ● | ○ | | |
| data-literacy | 数据与统计素养 | T15 | | | ○ | ● | | | | |
| multimodal-decode | 多模态解构 | T32/T37 | | | ○ | ● | | | | |
| money-trail | 资金链溯源 | T30 | | | ● | ○ | | | | |
| corpus-hook | 语料钩子·精问阅读 | ≈T33 | | | ● | ○ | | | | |
| source-map | 3D 溯源导图 | ≈T36 | | ○ | ● | | | | | |
| search-plan | 检索方向审视 | ≈T36 | | ○ | ● | | | | | |
| belief-spectrum | 立场光谱 | ≈T38 | | | ○ | ● | | | | |
| perspective-matrix | 视角对照矩阵 | ≈T38/T14 | | | ○ | ● | ○ | | | |
| certainty-spectrum | 确定度光谱 | ≈T13 | | | | ● | ○ | | | |
| framing | 语言框定 | ≈T07 | | | | ● | | | | |
| pee | PEE 写作 | T10 | | | | | ● | ○ | | |
| toulmin | 论证构建（图尔敏） | ≈T11 | | | | | ● | ● | | |
| concession | 让步段·以退为进 | T12 | | | | | ● | | | |
| steelman | 让步段·反方最强 | T12 | | | | ○ | ● | | | |
| ethics-lenses | 伦理判断三镜头 | T31 | | | | ● | ○ | | | |
| ethics-roleplay | 伦理情景·角色博弈 | ≈T31 | ○ | | | ○ | | | | ○ |
| knower-perspective | 认知者视角·自欺自审 | ≈T20 | | | | ○ | | | ● | |
| metacognition | 元认知收口 | ≈T18 | | | | | | | ● | |
| checkpoint | 无 AI 回放·方法迁移 | ≈T22 | | | | | | | ● | |
| learning-report | 学习报告·AI 使用声明 | — | | | | | | | ● | |
| question-card | 提问卡 | ≈T35 | ● | ○ | ○ | | | | | |
| rabbit-hole | 兔子洞·兴趣雷达 | T28 | | | ● | | | | | |
| emotional-alignment | 情感对齐 | — | ○ | | | | | | ○ | ● |
| ai-boundary | AI 边界与幻觉核查 | — | | | | | | | ○ | ● |
| ai-collaboration | 与 AI 协作保持判断 | — | | | | | | | ○ | ● |
| ai-decision-tree | 负责任使用 AI 决策树 | — | | | | | | | ○ | ● |
| aok-methods | 其他学科方法集 | ≈T21 | ○ | | | ○ | | | | |
| science-knowing | 科学怎么算知道 | ≈T21 | | | | ● | | | | |
| **lens-**\* (×9) | 学科透镜 | — (lens) | | | | ● | | | | |

## 3. Canon coverage — T00–T38 → have / gap

**Have (≈built):** T00 事实观点价值 · T01/02 CRAAP · T04 SIFT · T05 横向阅读(≈sift) · T06 OPCVL · T07 话语解码 · T08 FLICC(≈spin) · T09 论证解剖(≈argument-map) · T10 PEE · T12 让步阶梯(concession/steelman) · T13 程度论证(≈certainty-spectrum) · T15 数据三问(≈data-literacy) · T16 数据谣言(≈spin) · T20 反身性三问(≈knower) · T28 兔子洞 · T30 资金链 · T31 三镜头伦理 · T32 多模态 · T36 检索矩阵(≈search-plan/source-map).

**Gaps (canon tool, not built):**
- **T03 信息金字塔**（reading list 分诊）
- **T11 漏斗式论证**（写作/提纲）
- **T14 比较论证**（写作/提纲；perspective-matrix 只覆盖视角对比，不是论证对比）
- **T17 五种自疑** · **T18 五招自审** · **T19 推理六动作**（回顾/元认知）
- **T21 TOK四支柱**（跨阶段知识框架；aok/science-knowing 只是碎片）
- **T22 SOLO思考层级**（回顾·思维深度）
- **T29 GONE**（信源评估助记）
- **T33 文献精读**（读文章·学术精读；corpus-hook 只是钩子）
- **T34 主张追问**（立题/写作；question-card 偏泛提问）
- **T35 问题漏斗**（立题）
- **T37 视觉核验**（读文章·图像核验；multimodal 只是解构）
- **T38 利益相关者地图**（读文章/计划；perspective-matrix 不等于 stakeholder）

**Our extras (not in canon, keep):** AI 伦理三卡、checkpoint、learning-report、emotional-alignment、source-map、corpus-hook、belief-spectrum、framing、9 lenses.

**Internal-only (P0, never a student card):** T23 主动性梯度 · T24 逐步放手 · T25 一次只问一个 · T26 AI克制阶梯 · T27 模型路由.

## 4. The gap you flagged — 计划 (P2) is nearly empty

Only source-map / search-plan lean toward P2, and both are really P3 (search). We have **no card that helps build a good research plan**. Candidate new plan cards to consider:

- **研究里程碑卡** — turn the objective into 3–5 milestones on a timeline (feeds the Gantt).
- **范围界定卡 (scope/feasibility)** — narrow scope to what's doable in the time/资源 you have; surface what to cut.
- **方法选择卡** — pick how you'll investigate each sub-question (read / data / interview / compare) — bridges plan → reading list.

## 5. Reading list (P3) — how the multiple tools sit together (your Q3)

The 文献库 page has an ambient guide + several summonable tools. Proposed arrangement:

- **Backbone = the floating 印记 as the 兔子洞 exploration tree (T28).** It renders *what you've read and how sources branch*; a "read-but-unused" paper is a **dangling branch** it nudges you to connect or prune; "dig into this branch" = it proposes the next *necessary* direction (points, doesn't fetch/decide).
- **Attach cards to the exploration act, not a sentence:** 检索矩阵(T36) and 信息金字塔(T03) fire when you're *searching / triaging* ("what else do I need, and is this source-tier enough?"); 横向阅读(T05·SIFT-lateral)、资金链溯源(T30)、数据谣言(T16) fire when *vetting a candidate source before it earns a branch*.
- **Boundary with P4:** the reading list is "which sources, and how they connect"; opening one source crosses into the reading room (P4), where the per-sentence cards + lenses live. Rabbit-hole stays about the *forest*; reading room is the *tree*.

## 6. Open decisions (for you)

1. **Naming migration** — adopt the T00–T38 stems as the canonical names, rename our ≈-matches, and keep our extras + lenses. Confirm the ≈-mappings in §2 (some are judgment calls: certainty-spectrum→T13? perspective-matrix→T38 vs T14?).
2. **Which gaps to build, in what order** — I flagged ~14 canon gaps + 3 new plan cards. Likely priority: **P2 plan cards** (your gap) + **P4 reading gaps** (T33 文献精读, T37 视觉核验) since P4 is live.
3. **Many-to-many rendering** — a card in two phases needs two interaction renderings (e.g. 论证解剖 = pick-sentence in P4, build-structure in P5). Confirm we accept per-phase interaction variants rather than one fixed shape per card.
4. **Who places a card** — today the AI summons within a room's deck. You mentioned "later enable AI to propose cards/lenses during chat" — is cross-phase proposing (AI in P1 chat suggests a P4 lens) in scope soon, or later?

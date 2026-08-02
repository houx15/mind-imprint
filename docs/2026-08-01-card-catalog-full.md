# 思维印记 · 工具卡全库清单（2026-08-01）

系统内**全部 45 张**思维工具卡，含中文名、英文名、说明（purpose）、类别，以及每张卡的 **`asset_id`**（封面素材 T 号）与 **asset_name**（该素材的概念名）。
机器真相源：`packages/contracts/cards/*.json` 的 `asset_id` 字段。素材取舍逻辑与「有封面但无卡」的 17 个素材见 [card-asset-coverage](2026-08-01-card-asset-coverage.md)。

**匹配总览：** ✅ 已配 `asset_id` **22** · ⚠️ 待定（有候选素材，尚未配）**8** · — 暂无封面 **15**。
原则：内容一致但命名不同 → 以**素材名**为准；无法匹配 → 不设 `asset_id`（图鉴用文字封面回退）。「置信中/暂定」请同事复核封面语义与卡内容是否为同一物；「待定」的候选也请确认后再补 `asset_id`。

---

## 一览表（asset_id / asset_name 单列）

| 卡 id | 中文名 | 英文名 | 类别 | asset_id | asset_name | 匹配状态 |
|---|---|---|---|---|---|---|
| `emotional-alignment` | 情感对齐卡（内在小人·情绪电量） | Emotional Alignment (Inner Parts / Energy Meter) | 探究启动 | — | — | — 暂无封面 |
| `question-card` | 提问卡 | Question Card | 探究启动 | T35 | 问题漏斗 | ✅ 已配（置信中） |
| `rabbit-hole` | 兔子洞·兴趣雷达卡 | Rabbit Hole / Interest Radar | 探究启动 | T28 | 兔子洞阅读 | ✅ 已配（置信高） |
| `cda` | 话语分析卡 CDA | Critical Discourse Analysis | 信息素养 | T07 | 话语解码 | ✅ 已配（置信高） |
| `craap` | 信源辨识卡 CRAAP / CRRAAB | Source Evaluation (CRAAP / CRRAAB) | 信息素养 | T01 | CRAAP | ✅ 已配（置信高） |
| `money-trail` | 资金链溯源卡 | Follow the Money | 信息素养 | T30 | 资金链溯源 | ✅ 已配（置信高） |
| `multimodal-decode` | 多模态解构卡 | Multimodal Decode | 信息素养 | T32 | 多模态解析 | ✅ 已配（置信高） |
| `sift` | 横向核查卡 SIFT | SIFT Lateral Reading | 信息素养 | T04 | SIFT | ✅ 已配（置信高） |
| `sift_craap` | SIFT×CRAAP 信息核查 | SIFT × CRAAP Source Check | 信息素养 | — | — | — 暂无封面 |
| `spin-detector` | 营销与否认套路卡（漂绿 + FLICC） | Greenwashing & Denial Tactics | 信息素养 | T08 | FLICC | ✅ 已配（置信高） |
| `belief-spectrum` | 立场光谱卡（Belief Spectrum） | Belief Spectrum | 溯源与多视角 | — | — | ⚠️ 待定 · 候选 T13? |
| `corpus-hook` | 语料钩子卡（含精问阅读推荐） | Corpus Hooks & Curated Reading | 溯源与多视角 | — | — | ⚠️ 待定 · 候选 T33 文献精读? |
| `perspective-matrix` | 视角对照矩阵 | Perspective Matrix | 溯源与多视角 | — | — | ⚠️ 待定 · 候选 T38 利益相关者地图? |
| `search-plan` | 检索方向审视 | Search-Plan Audit | 溯源与多视角 | T36 | 检索矩阵 | ✅ 已配（置信中） |
| `source-map` | 3D 溯源导图卡 | 3D Source-Tracing Map | 溯源与多视角 | — | — | ⚠️ 待定 · 候选 T38 利益相关者地图? |
| `argument-map` | 论证地图卡（结构 + 谬误） | Argument Map (Structure & Fallacies) | 知识工具 | T09 | 论证解剖 | ✅ 已配（置信中） |
| `certainty-spectrum` | 确定度光谱卡 | Certainty Spectrum | 知识工具 | — | — | ⚠️ 待定 · 候选 T13 程度论证? |
| `concession` | 让步段 · 以退为进 | Concession / Steelman | 知识工具 | T12 | 让步阶梯 | ✅ 已配（置信高） |
| `data-literacy` | 数据与统计素养卡 | Data & Statistics Literacy | 知识工具 | T15 | 数据三问 | ✅ 已配（置信中） |
| `fact-opinion-value` | 事实/观点/价值判断卡 | Fact / Opinion / Value | 知识工具 | T00 | 事实观点价值 | ✅ 已配（置信高） |
| `framing` | 语言框定卡 | Language Framing | 知识工具 | — | — | — 暂无封面 |
| `pee` | PEE 写作卡 | PEE Paragraph (Point-Evidence-Explanation) | 知识工具 | T10 | PEE三明治段落 | ✅ 已配（置信高） |
| `steelman` | 让步段·反方最强卡 | Steelman / Concession | 知识工具 | — | — | ⚠️ 待定 · 候选 T34 主张追问? / 与 T12 共用? |
| `toulmin` | 论证构建卡（图尔敏） | Toulmin Argument Builder | 论证结构 | — | — | ⚠️ 待定 · 候选 T34 主张追问? |
| `aok-methods` | 其他学科方法卡集（数学·人文社科·艺术） | AOK Methods (Math / Human Sciences / Arts) | AOK | T21 | TOK四支柱 | ✅ 已配（置信中） |
| `opcvl` | OPCVL 史料评估卡 | OPCVL Source Evaluation | AOK | T06 | OPCVL | ✅ 已配（置信高） |
| `science-knowing` | 科学怎么算知道卡 | How Science Knows | AOK | — | — | — 暂无封面 |
| `ai-boundary` | AI 边界与幻觉核查卡 | AI Boundary & Hallucination Check | AI伦理 | — | — | ⚠️ 待定 · 候选 T26 AI克制阶梯?（语义有差） |
| `ai-collaboration` | 与 AI 协作保持判断卡 | Stay-in-Charge with AI | AI伦理 | T23 | 主动性梯度 | ✅ 已配（置信中） |
| `ai-decision-tree` | 负责任使用 AI 决策树卡 | Responsible AI Decision Tree | AI伦理 | T27 | 模型路由 | ✅ 已配（置信中） |
| `ethics-lenses` | 伦理判断三镜头卡 | Three Ethical Lenses | AI伦理 | T31 | 三镜头伦理 | ✅ 已配（置信高） |
| `ethics-roleplay` | 伦理情景·角色博弈卡 | Ethics Scenario & Role Play | AI伦理 | — | — | — 暂无封面 |
| `checkpoint` | Checkpoint 无 AI 回放·方法迁移卡 | No-AI Checkpoint & Transfer | 反身性与元认知 | T24 | 逐步放手 | ✅ 已配（置信中·暂定） |
| `knower-perspective` | 认知者视角·自欺自审卡 | The Knower's Perspective | 反身性与元认知 | T20 | 反身性三问 | ✅ 已配（置信中） |
| `metacognition` | 元认知收口卡 | Metacognition Wrap-up | 反身性与元认知 | T18 | 五招自审 | ✅ 已配（置信中·暂定） |
| `learning-report` | 学习报告·AI 使用声明卡 | Learning Report & AI-Use Statement | 成长与沉淀 | — | — | — 暂无封面 |
| `lens-communication` | 传播学：表达方式怎样影响判断？ | Communication | 学科透镜 | — | — | — 暂无封面 |
| `lens-economics` | 经济学：激励与成本如何变化？ | Economics | 学科透镜 | — | — | — 暂无封面 |
| `lens-ethics` | 伦理学：哪些价值正在冲突？ | Ethics | 学科透镜 | — | — | — 暂无封面 |
| `lens-history` | 历史学：这个判断依赖什么背景？ | History | 学科透镜 | — | — | — 暂无封面 |
| `lens-law` | 法学：权利与程序清楚吗？ | Law | 学科透镜 | — | — | — 暂无封面 |
| `lens-logic` | 逻辑学：推理有没有跳步？ | Logic | 学科透镜 | — | — | — 暂无封面 |
| `lens-methods` | 科学方法论：证据可靠吗？ | Scientific-Method | 学科透镜 | — | — | — 暂无封面 |
| `lens-society` | 社会学：谁得到机会，谁承担代价？ | Sociology | 学科透镜 | — | — | — 暂无封面 |
| `lens-systems` | 系统科学：后果会怎样传导？ | Systems-Science | 学科透镜 | — | — | — 暂无封面 |

---

## 逐卡详情（按类别）

## 探究启动

### 情感对齐卡（内在小人·情绪电量）
- **卡 id**：`emotional-alignment`
- **英文名**：Emotional Alignment (Inner Parts / Energy Meter)
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：在「激发兴趣」之前的第 -1 关：先让孩子感到被理解，找到他为什么「断电」，再慢慢把他带回好奇心

### 提问卡
- **卡 id**：`question-card`
- **英文名**：Question Card
- **asset_id**：`T35`
- **asset_name（素材概念名）**：问题漏斗
- **匹配状态**：✅ 已配（置信中）
- **说明**：在探究的最起点，先激活学生自己的经验、直觉和疑问，再让 AI 帮他延伸，而不是一上来就把题目交给 AI

### 兔子洞·兴趣雷达卡
- **卡 id**：`rabbit-hole`
- **英文名**：Rabbit Hole / Interest Radar
- **asset_id**：`T28`
- **asset_name（素材概念名）**：兔子洞阅读
- **匹配状态**：✅ 已配（置信高）
- **说明**：捕捉学生愿意「继续往下挖」的点，把模糊兴趣翻译成可探索方向，并把一次任务自然带向下一个洞


## 信息素养

### 话语分析卡 CDA
- **卡 id**：`cda`
- **英文名**：Critical Discourse Analysis
- **asset_id**：`T07`
- **asset_name（素材概念名）**：话语解码
- **匹配状态**：✅ 已配（置信高）
- **说明**：用 Fairclough 三维框架，看穿一段话怎么用词、怎么站位来潜移默化地说服甚至支配你

### 信源辨识卡 CRAAP / CRRAAB
- **卡 id**：`craap`
- **英文名**：Source Evaluation (CRAAP / CRRAAB)
- **asset_id**：`T01`
- **asset_name（素材概念名）**：CRAAP
- **匹配状态**：✅ 已配（置信高）
- **说明**：对单一来源做纵向体检：从时效、相关、权威、准确、目的与偏见几个维度，把「能不能引用」变成一组可判断的问题

### 资金链溯源卡
- **卡 id**：`money-trail`
- **英文名**：Follow the Money
- **asset_id**：`T30`
- **asset_name（素材概念名）**：资金链溯源
- **匹配状态**：✅ 已配（置信高）
- **说明**：追踪谁出的钱，看清利益如何扭曲内容——一个看似中立的结论，背后可能是被资助的叙事

### 多模态解构卡
- **卡 id**：`multimodal-decode`
- **英文名**：Multimodal Decode
- **asset_id**：`T32`
- **asset_name（素材概念名）**：多模态解析
- **匹配状态**：✅ 已配（置信高）
- **说明**：拆解画面、配乐、剪辑、语速如何联手塑造你的情绪和立场——看穿视听语言的说服

### 横向核查卡 SIFT
- **卡 id**：`sift`
- **英文名**：SIFT Lateral Reading
- **asset_id**：`T04`
- **asset_name（素材概念名）**：SIFT
- **匹配状态**：✅ 已配（置信高）
- **说明**：不要盯着可疑页面本身较劲——离开这一页、另开标签，看别人怎么评价这个来源，再回来下判断

### SIFT×CRAAP 信息核查
- **卡 id**：`sift_craap`
- **英文名**：SIFT × CRAAP Source Check
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：先横向找更多来源(SIFT)，必要时再纵向深挖单一材料(CRAAP)

### 营销与否认套路卡（漂绿 + FLICC）
- **卡 id**：`spin-detector`
- **英文名**：Greenwashing & Denial Tactics
- **asset_id**：`T08`
- **asset_name（素材概念名）**：FLICC
- **匹配状态**：✅ 已配（置信高）
- **说明**：两套套路识别器：漂绿——看穿绿色营销里「说的」和「做的」对不上；FLICC——识别否认科学的五类常见手法


## 溯源与多视角

### 立场光谱卡（Belief Spectrum）
- **卡 id**：`belief-spectrum`
- **英文名**：Belief Spectrum
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：⚠️ 待定 · 候选 T13?
- **说明**：把对错两派摊成一条连续光谱，定位各方与自己

### 语料钩子卡（含精问阅读推荐）
- **卡 id**：`corpus-hook`
- **英文名**：Corpus Hooks & Curated Reading
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：⚠️ 待定 · 候选 T33 文献精读?
- **说明**：教师/课程设计者提前在语料里「埋钩子」：学生读/看到特定位置，AI 自动触发追问；并按学生兴趣，从预设语料库推荐精问阅读卡片

### 视角对照矩阵
- **卡 id**：`perspective-matrix`
- **英文名**：Perspective Matrix
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：⚠️ 待定 · 候选 T38 利益相关者地图?
- **说明**：把「不同视角」从一句口号变成一张能填的表：每个视角主张什么、凭什么、又看不见什么

### 检索方向审视
- **卡 id**：`search-plan`
- **英文名**：Search-Plan Audit
- **asset_id**：`T36`
- **asset_name（素材概念名）**：检索矩阵
- **匹配状态**：✅ 已配（置信中）
- **说明**：在真正去查之前，逐条想清楚每个检索方向会给你哪一类证据、又系统性地漏掉什么

### 3D 溯源导图卡
- **卡 id**：`source-map`
- **英文名**：3D Source-Tracing Map
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：⚠️ 待定 · 候选 T38 利益相关者地图?
- **说明**：把一串聊天记录和零散材料，转成一张立体的导图：概念、来源、观点、利益相关方、偏见、兴趣点各成节点，看清谁支持谁、谁出钱、谁有偏见


## 知识工具

### 论证地图卡（结构 + 谬误）
- **卡 id**：`argument-map`
- **英文名**：Argument Map (Structure & Fallacies)
- **asset_id**：`T09`
- **asset_name（素材概念名）**：论证解剖
- **匹配状态**：✅ 已配（置信中）
- **说明**：把一段论证拆成「论点—论据—假设」的结构图，并对照常见逻辑谬误，看它到底站不站得住

### 确定度光谱卡
- **卡 id**：`certainty-spectrum`
- **英文名**：Certainty Spectrum
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：⚠️ 待定 · 候选 T13 程度论证?
- **说明**：不同知识有不同的确定度天花板——把一个结论放到光谱上，并用相称的语气表达它

### 让步段 · 以退为进
- **卡 id**：`concession`
- **英文名**：Concession / Steelman
- **asset_id**：`T12`
- **asset_name（素材概念名）**：让步阶梯
- **匹配状态**：✅ 已配（置信高）
- **说明**：在论证里先承认反方最强的事实，再转折反驳，使论证更有力、结构更完整

### 数据与统计素养卡
- **卡 id**：`data-literacy`
- **英文名**：Data & Statistics Literacy
- **asset_id**：`T15`
- **asset_name（素材概念名）**：数据三问
- **匹配状态**：✅ 已配（置信中）
- **说明**：对每个数字问三件事——从哪来、跟什么比、想让你得出什么；并识别图表里的误导手法

### 事实/观点/价值判断卡
- **卡 id**：`fact-opinion-value`
- **英文名**：Fact / Opinion / Value
- **asset_id**：`T00`
- **asset_name（素材概念名）**：事实观点价值
- **匹配状态**：✅ 已配（置信高）
- **说明**：把一段话里的陈述分成三类——能查证的事实、需论证的观点、藏着价值的判断，是一切清晰思考的地基

### 语言框定卡
- **卡 id**：`framing`
- **英文名**：Language Framing
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：看见措辞、委婉、隐喻、预设、主语选择如何在你没察觉时塑造你的想法——并试着换成中性词

### PEE 写作卡
- **卡 id**：`pee`
- **英文名**：PEE Paragraph (Point-Evidence-Explanation)
- **asset_id**：`T10`
- **asset_name（素材概念名）**：PEE三明治段落
- **匹配状态**：✅ 已配（置信高）
- **说明**：把一段论证写成 Point（要点）→ Evidence（论据）→ Explanation（解释）的结构，让学生自己搭骨架，而不是让 AI 代写

### 让步段·反方最强卡
- **卡 id**：`steelman`
- **英文名**：Steelman / Concession
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：⚠️ 待定 · 候选 T34 主张追问? / 与 T12 共用?
- **说明**：不打稻草人——先把反方最强的版本说出来，再回应它，让论证从「片面」变「可信」


## 论证结构

### 论证构建卡（图尔敏）
- **卡 id**：`toulmin`
- **英文名**：Toulmin Argument Builder
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：⚠️ 待定 · 候选 T34 主张追问?
- **说明**：把论点搭成能立住的结构：主张、理据、证据、反方与让步，一步步写成句子


## AOK

### 其他学科方法卡集（数学·人文社科·艺术）
- **卡 id**：`aok-methods`
- **英文名**：AOK Methods (Math / Human Sciences / Arts)
- **asset_id**：`T21`
- **asset_name（素材概念名）**：TOK四支柱
- **匹配状态**：✅ 已配（置信中）
- **说明**：三个学科各有一套「怎么算知道」，按任务学科调出对应子卡：数学靠证明、人文社科靠有限度的科学、艺术靠有据解读

### OPCVL 史料评估卡
- **卡 id**：`opcvl`
- **英文名**：OPCVL Source Evaluation
- **asset_id**：`T06`
- **asset_name（素材概念名）**：OPCVL
- **匹配状态**：✅ 已配（置信高）
- **说明**：用 IB 历史的 OPCVL 框架评估史料：来源、目的、内容、价值、局限——并升到双源对照与史观自觉

### 科学怎么算知道卡
- **卡 id**：`science-knowing`
- **英文名**：How Science Knows
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：用四条标准判断一个主张算不算可靠的科学：可证伪、对照、可重复、同行评审——科学是「暂定但可靠」


## AI伦理

### AI 边界与幻觉核查卡
- **卡 id**：`ai-boundary`
- **英文名**：AI Boundary & Hallucination Check
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：⚠️ 待定 · 候选 T26 AI克制阶梯?（语义有差）
- **说明**：AI 会「一本正经地编」。这张卡把 AI 的边界和可能的幻觉，变成学生练查证和判断的最佳时刻

### 与 AI 协作保持判断卡
- **卡 id**：`ai-collaboration`
- **英文名**：Stay-in-Charge with AI
- **asset_id**：`T23`
- **asset_name（素材概念名）**：主动性梯度
- **匹配状态**：✅ 已配（置信中）
- **说明**：在用 AI 时守住自己的判断——识别 8 种常见的「协作失败模式」，别让 AI 从助手变成替你思考的人

### 负责任使用 AI 决策树卡
- **卡 id**：`ai-decision-tree`
- **英文名**：Responsible AI Decision Tree
- **asset_id**：`T27`
- **asset_name（素材概念名）**：模型路由
- **匹配状态**：✅ 已配（置信中）
- **说明**：在动手前先问「这件事要不要用 AI、用到哪一步」——把判断前置到使用之前

### 伦理判断三镜头卡
- **卡 id**：`ethics-lenses`
- **英文名**：Three Ethical Lenses
- **asset_id**：`T31`
- **asset_name（素材概念名）**：三镜头伦理
- **匹配状态**：✅ 已配（置信高）
- **说明**：用三个镜头看同一个「应不应该」：后果（结果好坏）、义务（规则与权利）、品格（一个好人会怎么做），再加两个检验

### 伦理情景·角色博弈卡
- **卡 id**：`ethics-roleplay`
- **英文名**：Ethics Scenario & Role Play
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：把 AI 伦理做成游戏：在真实情景里扮演不同角色（开发者/用户/监管者/受影响者…），从各自的利益与盲区出发判断「AI 该介入到什么程度」


## 反身性与元认知

### Checkpoint 无 AI 回放·方法迁移卡
- **卡 id**：`checkpoint`
- **英文名**：No-AI Checkpoint & Transfer
- **asset_id**：`T24`
- **asset_name（素材概念名）**：逐步放手
- **匹配状态**：✅ 已配（置信中·暂定）
- **说明**：在关键节点做一次轻量验证：不靠 AI，用自己的话复述刚才的论点，或把同一方法迁移到新材料——回答产品最核心的问题「到底是学生学到了，还是 AI 学到了？」

### 认知者视角·自欺自审卡
- **卡 id**：`knower-perspective`
- **英文名**：The Knower's Perspective
- **asset_id**：`T20`
- **asset_name（素材概念名）**：反身性三问
- **匹配状态**：✅ 已配（置信中）
- **说明**：把同一把尺子先量向自己：识别五种自欺（确认/动机/立场/回声室/部落），再用五招自审——「我是不是只信我想信的？」

### 元认知收口卡
- **卡 id**：`metacognition`
- **英文名**：Metacognition Wrap-up
- **asset_id**：`T18`
- **asset_name（素材概念名）**：五招自审
- **匹配状态**：✅ 已配（置信中·暂定）
- **说明**：任务收尾时，回看自己怎么想的：校准信心、找出隐藏前提、指出自己论证里最弱的一环。


## 成长与沉淀

### 学习报告·AI 使用声明卡
- **卡 id**：`learning-report`
- **英文名**：Learning Report & AI-Use Statement
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：任务结束时，除了成果，还自动整理出第二份产出：我如何用 AI、AI 帮在哪、哪些是我自己的判断——这是平台对学校/家长/作品集最关键的交付物。


## 学科透镜

### 传播学：表达方式怎样影响判断？
- **卡 id**：`lens-communication`
- **英文名**：Communication
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：观察标题、标签、隐喻和叙述角度怎样突出一部分事实，同时把另一部分移出视野。

### 经济学：激励与成本如何变化？
- **卡 id**：`lens-economics`
- **英文名**：Economics
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：观察规则怎样改变人的选择，并追踪显性成本、机会成本、收益分配和外部影响。

### 伦理学：哪些价值正在冲突？
- **卡 id**：`lens-ethics`
- **英文名**：Ethics
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：识别一项主张所保护和牺牲的价值，比较公平、伤害、自主、尊严与责任之间的取舍。

### 历史学：这个判断依赖什么背景？
- **卡 id**：`lens-history`
- **英文名**：History
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：把主张放回它形成的时间与制度背景，观察哪些条件发生了变化，哪些惯性仍在延续。

### 法学：权利与程序清楚吗？
- **卡 id**：`lens-law`
- **英文名**：Law
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：区分权利、义务、责任和程序，检查一项规则由谁制定、如何执行、能否申诉。

### 逻辑学：推理有没有跳步？
- **卡 id**：`lens-logic`
- **英文名**：Logic
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：把一句话拆成证据、隐藏前提和结论，检查结论的力度有没有超过现有依据。

### 科学方法论：证据可靠吗？
- **卡 id**：`lens-methods`
- **英文名**：Scientific-Method
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：从概念定义、测量方式、样本、变量和误差出发，判断研究或模型结果能够支持多强的结论。

### 社会学：谁得到机会，谁承担代价？
- **卡 id**：`lens-society`
- **英文名**：Sociology
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：从社会位置、制度安排、资源差异和权力关系出发，比较同一规则对不同群体的影响。

### 系统科学：后果会怎样传导？
- **卡 id**：`lens-systems`
- **英文名**：Systems-Science
- **asset_id**：（未配）
- **asset_name（素材概念名）**：—
- **匹配状态**：— 暂无封面
- **说明**：把行动放入相互连接的系统，追踪直接影响、延迟后果、反馈循环和系统边界。


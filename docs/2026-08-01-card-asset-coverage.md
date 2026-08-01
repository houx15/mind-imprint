# 工具卡 ↔ 封面素材 覆盖报告（2026-08-01）

同事提供的封面素材：`docs/reference/cards/colorways/{light,cyber-sage,cyber-slate,cyber-warm}/`，命名 `T00..T38`（39 个概念）。
卡注册表：`packages/contracts/cards/*.json`（45 张）。

**映射原则（用户定）：** 内容一致但命名不同 → 以**素材名**为准；无法匹配 → 卡不设 `asset_id`（图鉴用文字封面回退）；素材没有对应卡 → 记在本报告。
**结论：** 22 张卡已配 `asset_id`；23 张卡无素材；17 个素材无卡。映射是**可迭代的数据**——本报告是待确认清单。

---

## A. 已配对（22）— 已写入卡 JSON 的 `asset_id`

| T | 素材概念 | 卡 id | 卡名 | 置信 |
|---|---|---|---|---|
| T00 | 事实观点价值 | `fact-opinion-value` | 事实/观点/价值判断卡 | 高 |
| T01 | CRAAP | `craap` | 信源辨识卡 CRAAP / CRRAAB | 高 |
| T04 | SIFT | `sift` | 横向核查卡 SIFT | 高 |
| T06 | OPCVL | `opcvl` | OPCVL 史料评估卡 | 高 |
| T07 | 话语解码 | `cda` | 话语分析卡 CDA | 高 |
| T08 | FLICC | `spin-detector` | 营销与否认套路卡（漂绿 + FLICC） | 高 |
| T09 | 论证解剖 | `argument-map` | 论证地图卡（结构 + 谬误） | 中 |
| T10 | PEE三明治段落 | `pee` | PEE 写作卡 | 高 |
| T12 | 让步阶梯 | `concession` | 让步段 · 以退为进 | 高 |
| T15 | 数据三问 | `data-literacy` | 数据与统计素养卡 | 中 |
| T18 | 五招自审 | `metacognition` | 元认知收口卡 | 中（暂定） |
| T20 | 反身性三问 | `knower-perspective` | 认知者视角·自欺自审卡 | 中 |
| T21 | TOK四支柱 | `aok-methods` | 其他学科方法卡集 | 中 |
| T23 | 主动性梯度 | `ai-collaboration` | 与 AI 协作保持判断卡 | 中 |
| T24 | 逐步放手 | `checkpoint` | Checkpoint 无 AI 回放·方法迁移卡 | 中（暂定） |
| T27 | 模型路由 | `ai-decision-tree` | 负责任使用 AI 决策树卡 | 中 |
| T28 | 兔子洞阅读 | `rabbit-hole` | 兔子洞·兴趣雷达卡 | 高 |
| T30 | 资金链溯源 | `money-trail` | 资金链溯源卡 | 高 |
| T31 | 三镜头伦理 | `ethics-lenses` | 伦理判断三镜头卡 | 高 |
| T32 | 多模态解析 | `multimodal-decode` | 多模态解构卡 | 高 |
| T35 | 问题漏斗 | `question-card` | 提问卡 | 中 |
| T36 | 检索矩阵 | `search-plan` | 检索方向审视 | 中 |

「暂定/中」的建议同事复核封面语义与卡内容是否确为一物。

---

## B. 无素材的卡（23）— 未设 `asset_id`，图鉴用文字封面

有**可能配对**（待同事确认是否为同一张的两个名字）：

| 卡 id | 卡名 | 可能素材 |
|---|---|---|
| `certainty-spectrum` | 确定度光谱卡 | T13 程度论证（ToWhatDegree）? |
| `perspective-matrix` | 视角对照矩阵 | T38 利益相关者地图? |
| `source-map` | 3D 溯源导图卡 | T38 利益相关者地图? |
| `toulmin` | 论证构建卡（图尔敏） | T34 主张追问? |
| `steelman` | 让步段·反方最强卡 | T34 主张追问? / 与 T12 共用? |
| `ai-boundary` | AI 边界与幻觉核查卡 | T26 AI克制阶梯?（语义有差） |
| `corpus-hook` | 语料钩子卡 | T33 文献精读? |
| `belief-spectrum` | 立场光谱卡 | T13? |

**暂无对应素材**（含 9 张学科透镜，属 demo「读一起」家族，本就未必有单卡封面）：
`sift_craap`（SIFT×CRAAP 复合，概念上用 T01/T04）、`emotional-alignment`、`ethics-roleplay`、`framing`、`learning-report`、`science-knowing`、`lens-logic`、`lens-methods`、`lens-society`、`lens-law`、`lens-economics`、`lens-ethics`、`lens-history`、`lens-communication`、`lens-systems`。

---

## C. 无卡的素材（17）— 有封面但注册表没有这张卡

| T | 素材概念 | 说明 |
|---|---|---|
| T02 | CRRAAB | CRAAP 的扩展变体，被 `craap` 卡涵盖，无独立卡 |
| T03 | 信息金字塔 | 无卡 |
| T05 | 横向阅读 | 即 SIFT 的技法，被 `sift` 涵盖 |
| T11 | 漏斗式论证 | 无卡 |
| T13 | 程度论证（ToWhatDegree） | 无卡（或 → `certainty-spectrum`，见 B） |
| T14 | 比较论证 | 无卡 |
| T16 | 数据谣言图鉴 | 无卡（与 `data-literacy`/`spin-detector` 相邻） |
| T17 | 五种自疑 | 无卡 |
| T19 | 推理六动作 | 无卡 |
| T22 | SOLO思考层级 | 无卡 |
| T25 | 一次只问一个 | 无卡（是产品铁律③，未必做成学生卡） |
| T26 | AI克制阶梯 | 无卡（是印记系统 prompt 的核心，未必做成学生卡） |
| T29 | GONE | 无卡 |
| T33 | 文献精读 | 无卡（或 → `corpus-hook`，见 B） |
| T34 | 主张追问 | 无卡（或 → `toulmin`/`steelman`，见 B） |
| T37 | 视觉核验 | 无卡（反向图片检索类） |
| T38 | 利益相关者地图 | 无卡（或 → `perspective-matrix`/`source-map`，见 B） |

---

## 建议下一步（非本轮阻塞）
1. 同事复核 A 表「中/暂定」8 项与 B 表「可能配对」8 项，确认后补/改 `asset_id`。
2. C 表中有教学价值的（如 T14 比较论证、T34 主张追问、T38 利益相关者地图）可后续新增卡 JSON（一张卡 = 一份 JSON，不改渲染器）。
3. `light` 配色的多版本（女生版/男生版/vN）当前每 T 取一张为准；如需按性别/版本切换，另议。

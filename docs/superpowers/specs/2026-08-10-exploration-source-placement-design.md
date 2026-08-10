# 探索图谱 · 来源归位（Source Placement）设计

> 让「兔子洞地图」成为学生**全部**来源探索的忠实记录：加来源时就为它归位到某个（子）问题，归不了的落进「未归类」节点。

**Goal:** 学生在阅读室加进的任何一篇来源（贴链接 / DOI / 手动 / 上传），都要在加进的当下被「归位」——挂到主问题或某个子问题下（成为该问题洞里的一个文献节点），或明确落进地图上的一个「未归类」系统节点。印记在归位时**主动建议**最合适的问题，但确认永远是学生点一下（铁律②）。

**Architecture:** 复用 `adoptExploration` 的「建 connected lead」逻辑，新增一个「把已有 reference 挂到某问题」的 attach 端点；新增一个「印记为一篇来源建议归属问题」的 suggest 端点（服务端网关，中档模型、reasoning-off、记 `llm_call`）。前端在 `AddSourceModal` 成功建 reference 后插入一步「归位」；`WarrenMap` 增一个中性「未归类」节点，点开是一张未归位来源清单，每条可「进入阅读室」或「挂到问题下」。

**Tech Stack:** React + React Flow (`@xyflow/react`) 前端；Go (`net/http` + `pgx`/`sqlc`) 后端；Zod 契约。所有 LLM 调用走后端网关（客户端绝不直连模型）。

## Global Constraints

- **客户端绝不直连模型。** 印记的归属建议走后端 `POST …/exploration/suggest-placement`，记 `llm_call`（档位 + token + 成本），在 `HasEntitlement` 接缝之后。
- **铁律②（不操纵）：** 归位建议是**预高亮**，不是自动挂。挂 / 落进未归类都由学生点一下。归位属于「确定性的系统整理」，不是学生正文写作，不触犯铁律①。
- **papers-never-roots 不变式：** attach 出来的文献 lead 一定带 `parentLeadId`（挂在某问题下），绝不为 root。
- **行为真相源：** `docs/2026-08-09-all-statuses.md` §5（reading）+ §6.reading。§130 明写每篇来源要标注「which sub question, where it can appear」——本设计即其一半（which sub question）的落地；「where it can appear」（正文哪一段/哪个 claim）不在本期范围。
- **不改「悬空来源」语义：** `computeDanglingSourceIds`（已读但没跟进）保持原样，喂给 coach projection 和 `explorationSignal` 徽标。本设计的「未归类」是**另一个**集合，独立计算，不复用它。

---

## 1. 背景 · 为什么要做（现状的三层不可见）

在冷走查的真实 prod 数据里，探索图谱里从来没有出现过任何文献节点（每个项目 `paper_leads = 0`）。排查确认**不是数据丢失**（`reference` 行都在），而是「一篇来源进不进图谱」有三条互斥的命运，只有最后一条能进图谱：

| 来源怎么进来的 | 建了什么 | 在图谱里可见？ |
|---|---|---|
| 加链接 / DOI / 手动 / coach 精选（`createReference`） | 只有一行 `reference`（悬空） | **否**——从不渲染 |
| 读过但没跟进（有 `material_id`、无 connected lead） | `reference` + engaged material | 否——只进 `danglingSourceIds` 计数（`computeDanglingSourceIds` 要求 `material_id`，见 `exploration.go:185`） |
| 深挖 → 采纳（`adoptExploration`） | `reference` + connected `exploration_lead`（`parent_lead_id` + `connected_reference_id`） | **是**——真正的文献节点 |

于是学生自己贴进来的来源（哪怕读了）永远不在「兔子洞地图」上；地图只画了「深挖-采纳」那个子集。对下一步的过程评估这是硬伤：**图谱不是来源探索的忠实记录**。本设计补上这条缺口。

---

## 2. 数据模型（不加表，复用现有列）

现有 `exploration_lead` 已足够（见 prod 结构核对）：

- **主问题**：root 问题 lead（`parent_lead_id IS NULL`，`connected_reference_id IS NULL`）。
- **子问题**：嵌套问题 lead（`parent_lead_id` 指向某问题，`connected_reference_id IS NULL`）——prod 已有此形态。
- **文献节点**：`connected_reference_id` 有值的 lead，挂在某问题 / 子问题下。
- **未归类来源**：`reference` 行，**没有**任何非 pruned lead 以它为 `connected_reference_id`。**读没读都算**（与 `danglingSourceIds` 的关键区别：不要求 `material_id`）。

「归位一篇来源到某问题」= 建一个 `connected_reference_id = 该 reference` 且 `parent_lead_id = 该问题` 的 lead（`status:"connected"`，`origin:"manual"`——学生自己的归位；`position` = 该 parent 下兄弟数）。这与 `adoptExploration` 唯一的差别是：**不新建 reference，而是挂一个已存在的 reference**。

**未归类集合的计算放在前端。** `ExplorationView` 已同时持有 `references: Reference[]`（ReadingBlock 的 `getLibrary`）与 `view.leads`。未归类 = `references` 里，其 id 不在「任一非 pruned lead 的 `connectedReferenceId`」集合中的那些。无需新增服务端字段。（服务端 `getExploration` 不变。）

---

## 3. 后端端点

### 3a. Attach —— 把已有 reference 挂到某问题

`POST /api/v1/projects/{id}/exploration/attach`，body `{ referenceId: string, parentLeadId: string }`，返回 `201 { lead, reference }`。

- IDOR：`referenceId` 与 `parentLeadId` 都要属于本 project（复用 `GetExplorationLeadForProject` + 一次 reference 归属校验）。
- `parentLeadId` 必须是**问题** lead（`connected_reference_id IS NULL`）——不能把来源挂到另一篇来源下（保持「问题→文献」两层语义清晰）。挂到非问题 lead 返回 400。
- 事务内：算 `position`（该 parent 下兄弟数）→ `CreateExplorationLead(Status:"connected", Origin:"manual", ConnectedReferenceID: ref.ID, ParentLeadID: parent, Position, Text: ref.Title)`。**不**新建 reference。
- 幂等宽容：若该 (referenceId, parentLeadId) 已有非 pruned lead，直接返回既有 lead（不重复挂）。
- 路由注册在 `apps/api/internal/api/api.go`（紧挨 `adoptExploration` 那行 116）。

### 3b. Suggest placement —— 印记为一篇来源建议归属问题

`POST /api/v1/projects/{id}/exploration/suggest-placement`，body `{ referenceId: string }`，返回 `200 { leadId: string | null, reason: string }`。

- 取该 reference 的 `title` + `abstract`（+ 已抓到的 journal/year）与本 project 的全部**问题** lead（主问题 + 子问题，`connected_reference_id IS NULL`），让模型选一个最贴合的问题 id，或返回 `null`（都不贴 → 建议未归类）。`reason` 一句话，中文，给学生看。
- **中档模型、reasoning-off、小 token**（跟 coach 一致的便宜档）。记一行 `llm_call`。在 `HasEntitlement` 之后。
- 无问题时（早期探索）直接返回 `{ leadId: null, reason: "" }`，不调用模型。
- 契约：`packages/contracts/src/exploration.ts` 新增 `PlacementSuggestion = z.object({ leadId: z.string().nullable(), reason: z.string() })`。attach 沿用现有「inline 类型」风格（与 adopt 一致，不强加 zod）。

---

## 4. 前端 surfaces

### 4a. 加来源即归位（`AddSourceModal`，`ReadingBlock.tsx`）

`submit()` 建完 reference 后，不再直接关闭，而是进入 modal 内的**「归位」子步**：

- 触发 `suggest-placement`（印记想一下，一行 loading 提示）。
- 展示：本 project 的问题作为可点 chip——**主问题**为顶层、**子问题**缩进其下；印记建议的那个**预高亮**并附 `reason`；末尾一个 **「先放进未归类」** 选项。
- 学生点一个问题 → 调 `attach` → 该来源立即成为那个问题洞里的文献节点；点「未归类」或本就无问题 → 结束，来源留在未归类节点里。
- 一次一个决定（铁律③）。归位后刷新图谱 + 库。

> 复用点：`dig→采纳` 已把「新建 reference + 挂 lead」打通；这里是「已有 reference + 挂 lead」，UI 上就是同一个「归位选择器」组件（见 4c）。

### 4b. 「未归类」节点（`WarrenMap.tsx` + `warrenLayout.ts`）

- 仅当存在 ≥1 篇未归类来源时，在 map 上追加**一个**中性系统节点：**「未归类 · N 篇」**（`NEUTRAL` 主题、固定位置、不参与拖拽持久化、不参与问题间连边、`onZoom`/删除逻辑跳过它）。
- 点它 → 进入一个类 Level-2 的**「未归类」面板**（有「← 返回兔子洞地图」返回键）：主区是未归位来源的**清单**（不是 mindmap），每条卡片给两个动作——**进入阅读室**（复用 `onEnterReading`）、**挂到问题下**（弹 4c 的归位选择器，带印记建议）。挂完这条从清单里消失、出现在对应问题洞里。
- 右侧沿用现有「两页 aux 侧栏」——未选中时是 controls（检索方向 / 理一理材料 / …），不额外造第三种布局。

### 4c. 归位选择器（新共享组件）

`PlacementPicker`：入参 `{ questions: {id,text,parentId}[], suggestedLeadId, reason, onPick(leadId | null) }`。渲染主问题 + 缩进子问题的 chip 列表 + 「先放进未归类」；预高亮建议项、显示 `reason`。add-time modal 与「未归类」面板共用它。

---

## 5. 一致性校验（对照 all-statuses.md）

- §5 reading「Initialized with the main question and 2-4 subquestions … finished when each subquestion is fully explored with a good 证据地图」——归位把来源系到（子）问题上，正是「每个子问题被充分探索」得以度量的前提。✅
- §130「each paper … its correlation with the paper (which sub question, where it can appear)」——本设计落地「which sub question」。「where it can appear（正文哪段/哪个 claim）」标注**明确不在本期**。✅（无冲突）
- §118 AI's role「give suggestions for what is good to read, what may be not correlated with the current question」——`suggest-placement`（贴哪个问题 / 都不贴）与此同源。✅

---

## 6. 测试

- **Go 单测**：`attach` happy-path（建 connected lead、position、origin=manual、text=title）；attach 到非问题 lead → 400；跨 project referenceId/parentLeadId → 404/403；幂等（重复 attach 同一对返回既有 lead）。`suggest-placement`：无问题 → 不调模型、返回 null；有问题 → 返回集合内的某 leadId 或 null（用假 LLM）。
- **前端单测**：`PlacementPicker` 渲染主/子问题层级 + 预高亮建议 + 未归类；`AddSourceModal` 建 ref 后进入归位子步、点问题触发 attach；`WarrenMap` 仅在有未归类来源时显示「未归类」节点、其 count 正确、点它进入清单面板；未归类集合的纯计算（含「已读但未挂」与「未读」都算）。
- **契约**：`PlacementSuggestion` zod round-trip。

---

## 7. 明确不做（本期）

- ❌ 「where it can appear」——把来源标到正文某个 claim/段落（§130 的另一半）。留待写作面结构成型后单独一期。
- ❌ 一篇来源多问题归属的 UI（数据模型天然支持多 lead，但 UI 一次挂一个；未归类清单只要它挂到**任一**问题就移除）。
- ❌ 改「悬空来源」`danglingSourceIds` 语义 / coach projection。
- ❌ 拖拽把节点从未归类拖到问题上（用选择器点选，不做拖拽）。
- ❌ 自动挂（无学生确认）——违铁律②，永不做。

---

## 8. 待定 / 开放项

- 「未归类」是工作名，最终命名待定（不改 map 名「兔子洞地图」）。
- `suggest-placement` 的模型档位：先按 coach 同档（中档 reasoning-off）；若建议质量不足再评估。

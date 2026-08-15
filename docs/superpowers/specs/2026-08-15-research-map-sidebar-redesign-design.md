# 研究图谱 · AI 搜索侧栏重组 — Design

**状态：** 已评审通过（2026-08-15）。**决定：** §5 的「待加入候选搜相似/被引」——给 `dig` 加 `doi` 入参，走真正的 OpenAlex 相似/被引。
**目标：** 把研究图谱（探索图谱）右侧「AI 搜索侧栏」的层级理顺——用**同一个「单篇论文详情」布局**服务所有入口，四个视图串成一条清晰的状态机，单篇详情内能继续搜「相关 / 被引」，并标出这篇是否已在图谱里。

---

## 1. 问题（当前为什么乱）

同一块右侧栏由**两套并行的视图状态机**各管一半，于是布局重复、详情有两种长相：

| 关注点 | 当前位置 | 长相 |
|---|---|---|
| 搜索下钻（AI 建议→搜索→结果→详情） | `ExplorationView.tsx` 的 `controlsColumn`，state=`searchStage`（`idle`/`directions`/`list`/`detail`） | detail 是「窄小卡」，信息挤在一起 |
| 图谱节点详情（点图里的论文节点） | `ExplorationSidebar.tsx`，state=`sidebarState`（`ai`/`node`/`results`） | `PaperMeta` 是好布局：论文徽标 + 阅读状态 + 带标签的 `<dl>` 作者/年份/期刊/链接 + 摘要 + 打开原文/进入阅读室 + 证据笔记 |
| 结果列表 | 三处重复渲染：`searchStage:"list"` 行、`ResultsPanel` 卡、以及 detail 两版 | — |

关键差异：
- 搜索结果详情绑定的是**临时的 `DigCandidate`**（`{doi,title,authors,year,journal,abstract,url}`，还没进图谱）。
- 节点详情 `PaperMeta` 绑定的是**已持久化的 `Reference` + `ExplorationLead`**（已在图谱）。
- 「找相似/找它引用的/找引用它的」（`onDig("similar"|"citation"|"cited")`）**只在节点详情里有**，搜索详情的窄卡里没有。
- **「是否已在图谱」当前没有被追踪**——搜索候选不与现有 references/leads 比对。

---

## 2. 目标视图状态机（统一后）

一套状态，四个视图。侧栏任意时刻只处于其一：

```
default ──「让印记建议检索方向」──▶ keywords ──点某个关键词──▶ results ──点某篇──▶ paper
   ▲            │                        │                      │            │
   │            └── 检索卡/直接搜 ───────┘                      │            │
   └──────────────── 返回 ◀──────────────────────────────────┘            │
                                          ▲   点「相关/被引」结果 ───────────┘
                                          └── paper 内搜索又产出 results（同一 results 视图，循环）
```

- **(a) default**：印记建议区（`让印记建议检索方向` → `proposeDirections`）、检索卡入口、`ExplorationReviewBox`、`NeedsResourcesBox`、propose-relations。**保留现状**。
- **(b) keywords**：建议关键词列表（`{keyword, why}`），每条一个「搜索」按钮。
- **(c) results**：搜索结果**列表**（可点的行），点进 → paper。加载态 `RabbitHoleLoader`。**唯一的列表渲染器**（合并掉 `ResultsPanel` 卡 + `searchStage:"list"` 行）。
- **(d) paper**：**唯一的单篇详情组件**（见 §3），有两态：
  - **已在图谱**：显示阅读状态 + 证据笔记 + 进入阅读室；主操作是「进入阅读室 / 打开原文」。
  - **待加入**：主操作是「采纳到当前问题 / 收进未归类」。
  - 两态都提供：**找相似 / 找它引用的 / 找引用它的** → 产出 results（回到 (c)，形成循环）。

**图谱节点点击**也进入同一个 (d) paper 视图（已在图谱态）——不再有第二种详情布局。

---

## 3. 统一的 `<PaperDetail>` 组件

一个组件，能从**两种数据源**渲染，靠一个归一化的 view-model：

```ts
interface PaperView {
  key: string;              // doi || url || lead.id — 用于 added 判定与 React key
  title: string;
  authors?: string; year?: string; journal?: string;
  abstract?: string; url?: string; doi?: string;
  // 已在图谱时才有：
  reference?: Reference;    // 阅读状态、证据笔记、进入阅读室
  lead?: ExplorationLead;
  added: boolean;           // 是否已在图谱（见 §4）
}
```

- 来自 `DigCandidate` → `added=false`，无 reference/lead。
- 来自图谱节点（`Reference`+`ExplorationLead`）→ `added=true`。
- 布局采用现有 `PaperMeta` 的好布局（论文徽标 + 带标签 `<dl>` + 摘要 + 链接），**替换掉** `ExplorationView.tsx` 的窄小卡。
- 证据笔记 `EvidenceNote` 子面板：**仅 `added` 时**渲染（未加入的候选还没有 reference 承载证据）。
- 底部操作条随 `added` 切换（进入阅读室 vs 采纳/收进未归类）；两态都有「找相似/被引」。

`PaperMeta`（`ExplorationSidebar.tsx:322-462`）为基础抽出该组件；窄卡（`ExplorationView.tsx:846-910`）删除。

---

## 4. 「已在图谱 / 待加入」判定

当前不存在此状态。加入纯前端派生（沿用 `unfiledReferences`/`selectedRef` 的思路 `ExplorationView.tsx:51-57,601-605`）：

```
addedKey(ref)      = ref.doi || ref.url
candidateAddedBy   = new Set(references
                       .filter(r => 有非 pruned 的 connected lead 或在 unfiled)
                       .map(addedKey).filter(Boolean))
candidate.added    = candidateAddedBy.has(candidate.doi || candidate.url)
```

- 匹配键优先 `doi`，回退 `url`（都做小写/trim 归一）。
- 命中 → 该候选在 paper 视图显示「已在图谱」态，主操作变「进入阅读室」，「采纳」置灰/隐藏（避免重复加入）。

---

## 5. 复用的 API（无需新增后端）

全部已存在，仅前端重接线：

| 用途 | 前端 | 端点 |
|---|---|---|
| 关键词建议 | `proposeSearchGuidance` | `POST .../search-guidance` |
| 搜索 / 相似 / 被引 | `digExploration({keyword?}/{leadId,mode})` | `POST .../exploration/dig`（`similar/citation/cited`，OpenAlex） |
| 加入新论文 | `adoptCandidate` | `POST .../exploration/adopt` |
| 关联已有 ref | `attachReference` | `POST .../exploration/attach` |
| 证据/分诊/归档 | `setReferenceEvidence/...` | 现有 |

**注意**：`digExploration` 的「相似/被引」目前只接受 `leadId`（已在图谱的节点）。**待加入**的候选没有 lead，要从它搜相似/被引，需要按 `doi`（或标题）驱动一次 dig——需确认 `digExploration` 是否支持 `doi` 入参，或搜索时以关键词=标题回退。**这是唯一可能需要小改后端的点**，实现期确认。

---

## 6. 影响文件（实现期细化）

- `apps/web/src/workspace/blocks/exploration/PaperDetail.tsx`（**新**，从 `PaperMeta` 抽出，双数据源）。
- `ExplorationSidebar.tsx`：`NodePanel`/`PaperMeta` → 复用 `PaperDetail`；`ResultsPanel` 与 `searchStage:"list"` 合并为唯一结果列表。
- `ExplorationView.tsx`：`controlsColumn` 的 `searchStage` 与 `sidebarState` 收敛为一条视图状态机；删窄卡；接入 added 判定；paper 视图挂「相似/被引」。
- 可能：`exploration.go` 的 `digExploration` 支持无 lead（按 doi/标题）搜相似/被引。

## 7. 明确不做

- 不新增花哨交互、不动图谱布局（`WarrenMap`/`QuestionMindmap` 不变）。
- 不改「学生确认才加入」（铁律①）——采纳仍是学生动作。
- 不做跨会话的「已读过某篇」全局去重（只在当前项目内判 added）。

---

**评审问题**：§5 那个「待加入候选如何搜相似/被引」——接受「以标题为关键词回退搜索」，还是要我给 `dig` 加 `doi` 入参走真正的 OpenAlex 相似/被引？

# 阅读与写作：完成之后还能回看，报告值得发出去

> 2026-09-16 · lite edition (`apps/lite-web` + `apps/api` 的 lite 路径)
> 产品负责人 2026-09-16 提的三件事，一次做完。

## 这一版要解决的三件事

产品负责人的原话：

1. 「now once student finished, they can only see the report, which is really
   simple, and they cannot go back to view their chat history. but we want that
   student can view all history.」
2. 「and the report and published writing, can be made public.」
3. 「the current report is really not abundant and cannot ignite my willing of
   sharing.」她给的参照是别的产品分享出来的那种页面：一句「这是我和 xx 一起阅读
   的第 x 篇文章！」，一个大标题，一条总数据（时长、对话轮数、笔记数），然后是
   金句卡。

## 现状（读代码 + 真渲染一份报告确认过，不是推的）

- **回看是真的没有。** 一次读完的阅读打开的是 `FinishedReadingPanel`
  （`apps/lite-web/src/readings/ReadingRoomHost.tsx`），它在注释里写明自己
  「READ-ONLY BY CONSTRUCTION」——没有房间，因此没有对话、没有正文、没有任何
  回看入口。写作那边是 `FinishedWritingPage`，同样只有报告（成稿是它自己的一页）。
  **但数据一直都在**：`GET /api/v1/readings/{id}/messages` 和写作那条孪生接口
  早就存在（`api/readingRoom.ts:333`、`api/writingRoom.ts:348`），只是没有任何
  界面去读它。
- **公开分享早就有了，而且比我们以为的多。** `SharePanel` 发 `/s/:token` 并画
  二维码，随时可撤；写作的 `/s/:token` 是她的成稿本身，`/s/:token/record` 是
  记录。所以第 2 件事缺的不是链接，是**入口太隐蔽**，以及**「发布」这件事没有
  落点**。
- **报告不缺版式，缺的是内容和身份。** 现在的阅读报告是一份排得不错的手记：
  masthead、大标题、数据带、我的收获、金句、我的笔记、透镜、这次的收获。问题是：
  - 页面的标题是**文章的标题**，于是它读起来像那篇文章的页面，不像她的记录；
  - 没有任何累计感——第几篇、和谁一起读的，一个字都没有；
  - **12 轮对话在页面上是一个数字「12」**。这一版真正稀缺的内容就在这里。

## 决定（产品负责人 2026-09-16 逐条选的）

| 问题 | 决定 |
|---|---|
| 对话记录谁能看 | 她自己随时能回看；**公开与否是一个单独的勾选**，默认关，随时可改，撤链接就一起没了 |
| 「成品写作可以公开」 | **两件都要**：分享入口做明显 + 成稿有「发布」状态并列在她的主页上 |
| 报告加什么 | 开场一句 + 数据带 + 金句卡；**对话里的转折时刻**；**她读的那篇文章的入口**。不做跨篇对比 |

不做跨篇对比是她自己划掉的——理由也成立：铁律说报告是记录不是评判，一条会涨会跌
的线很容易变成打分。

---

## 一 · 回看

### 1.1 完成页变成三格

`FinishedReadingPanel` 与 `FinishedWritingPage` 在现有的那条细横条下面多一个分段
切换器，切的是**同一页**里的三屏：

- 阅读：**报告** · **对话** · **原文**
- 写作：**报告** · **对话** · **成稿**

标签是名词，不是句子（界面文案规则 1）。默认停在**报告**——她刚完成时想看的是
结果；回看是她第二次来才要的东西。

切换器只改这一页里显示哪一屏，**不进 URL**。理由：`/readings/:id` 已经是一条会被
分享、会被教师端引用的深链接，多一段 `?view=` 会让「同一条链接对不同人打开不同
屏」这件事变成新的一类 bug，而这三屏之间的切换没有需要被链接的价值。

### 1.2 对话那一屏

数据：`GET /readings/{id}/messages`（写作走 `/writings/{id}/messages`），已存在，
不加接口。

渲染规则：

- 按 `seq` 顺序，她的话和印记的话各自成条，**每条都标明是谁说的**。
- **只读是结构性的，不是禁用出来的**：这一屏里没有输入框、没有重试、没有任何会
  发请求的控件。它连一个能写的路径都不存在。
- 一条带卡片的消息（`payload.card`）渲染成**一行静态说明**
  （`印记给了一张卡片 · 挑句子`）加上她当时填的答案，**不挂真卡片组件**——那些
  组件是能操作的，在一个只读页面上摆一张能点的卡片，是给她一个按下去什么都不会
  发生的按钮。
- **她自己写的字一个字都不许截断**（2026-09-12 的教训：切到 400 字之后她跟印记说
  了三次「我的字被截断了」）。长消息就是长消息，页面滚。

### 1.3 原文那一屏（阅读）

`reading_source` 的正文，按现有的 `SplitBlocks` 分段渲染，只读。她读的那篇本来
就在我们手上，这一屏不需要任何新数据。`excerptOnly` 为真时（星图来的那种只有
导语的源），照常渲染它有的那部分，并摆出现成的「跳转原网站」那一条。

成稿那一屏写作已经有了（`FinishedWritingPage` 自己就是），这里只是把它收进同一个
切换器里，不新建页面。

---

## 二 · 报告

### 2.1 开场一句：`ordinal`

报告顶上多一句：

> 这是 Phoebe 和印记一起读的第 8 篇文章！

**存的是数字，不是句子。** `liteReportDTO` 加一个 `ordinal int`，值是她**完成这一
篇时**已完成的阅读数（写作同理）。句子由前端拼：她自己看是「我和印记一起读的第 8
篇文章」，公开页上访客看到的是「Phoebe 和印记一起读的第 8 篇文章」。

两条理由：

- 报告是**存下来的整块 JSON**，一句话一旦写进去就永远改不动了（这条在报告的统计
  标签上已经踩过一次，`statLabels.ts` 就是为此存在的）。
- 序号**冻结**在完成的那一刻。重新数会让她三个月前那份报告今天变成「第 20 篇」，
  那是在改她的过去。

感叹号留着——这是她给的原话，而且完成一篇确实是规则 9 说的那种「真正的节点」。

计数用两条新查询（`reading` / `writing` 各一条，`status = 'finished'`），在
`buildReadingReportDTO` / `buildWritingReportDTO` 的那个事务里数。

### 2.2 数据带上移

`ReportVisualSummary` 不改，位置改：现在它在一张占了半屏的插画下面，要挪到开场
那一句旁边。插画留着但让位——报告的主角是她做的那些事，不是那张图。

### 2.3 转折时刻（这一版真正新的内容）

三条候选做法，选第二条：

- **(A) 让模型引对话原话。** 最省事，也是这个仓库已经栽过两次的那个坑：它会引她
  没说过的句子，而且一个来回有两个人说话，印记那一侧也要验。
- **(B) 让模型只回答轮次编号，正文由服务端从库里取。** ← 采用。
- (C) 纯确定性挑（比如挑最长的那几轮）。挑不出「转折」，只能挑出「话多」。

具体形状：

- 喂给模型的用户 prompt 多一块 `【对话记录】`，**按她的发言编号**（1、2、3…），
  每条后面跟印记紧接着的那一条回复。这一块**明说只用来挑编号**，里面的句子不进
  `corpus.Text`，因此仍然**结构上不可能**被引成金句——R4 的那道墙一动不动。
- 模型回 `"turningPoints": [{"turn": 14, "why": "她在这里改了主意"}]`，最多 3 条。
- 服务端拿 `turn` 去**原始消息**里取她那条和印记那条的**逐字原文**，编号不在这次
  喂进去的窗口里就整条丢掉，重复的编号只留第一条。
- 于是「引错」这件事不存在：模型唯一能写的字是那句 `why`。

存进报告的形状：

```go
type reportTurningPoint struct {
    Turn    int    `json:"turn"`
    Why     string `json:"why"`     // 模型写的那一句，唯一由模型产出的文字
    Student string `json:"student"` // 逐字，来自 atom_message
    Coach   string `json:"coach"`   // 逐字，来自 atom_message；可能为空
}
```

**成本是零**——它是 `generateReportProse` 那个已经在跑的 `assess` 调用上多出来的
第四个字段，不新增调用。prompt 里给模型的 `【对话记录】` 可以按每条 300 字截断
（那只是给它挑编号用的），但**渲染到报告上的是库里的完整原文，不截断**。

渲染：每条一块，她的话在上、印记的话在下，各自标明是谁说的，`why` 作为那一块的
小标题。

### 2.4 文章入口（阅读）

报告上多一块：标题 · 来源站点 · 原文链接（`reading_source.source_url` 有才摆）·
一段短摘录。

**公开页上永远不放全文。** 分级阅读库是第三方素材，报告是她的记录，不是一次转载。
摘录取正文第一段，上限 200 字；她自己划过的那些句子本来就已经在「我的笔记」和
「我用透镜查到的」两节里逐字摆着，那才是这篇文章在她这份报告上的真正分量。

写作的报告没有这一块——她的成品本来就在 `piece` 里。

---

## 三 · 公开

### 3.1 分享做成一颗真按钮

完成页上现在的分享是角上一个图标。改成一颗带字的按钮（**分享**），和导出并排。
理由很朴素：她说这份报告点不起她发出去的欲望——而发出去这件事本身现在要先找到
一个图标。

### 3.2 对话公开是一个单独的勾选

- 迁移 **0174**：`ALTER TABLE atom_report ADD COLUMN include_transcript boolean
  NOT NULL DEFAULT false;`
- `SharePanel` 在链接生成之后多一个勾选：**公开我和印记的对话**。默认不勾。
  勾了之后 `POST .../report/share` 带上这一位（已分享的报告改这一位是同一条接口，
  token 不变——重新发一个 token 会悄悄弄坏她已经发出去的链接）。
- 撤销分享时这一位跟着回到 false：一条撤掉又重开的链接，不该继承上一次的公开
  范围。
- 公开负载只有在这一位为真时才带 `transcript`，**每一条都带说话人**。

🚨 **这一条推翻了 `atom_report_share.go` 顶上写死的那条规矩**：

> The payload is the report and nothing else: no account, **no transcript**,
> no article, no draft body…

那句话是当初的绑定裁定（R1）。现在产品负责人明确要她能选择公开对话，所以那段注释
**要改写成记录这次反转和它的日期**，而不是留一段和代码相反的话在文件顶上——一段
说谎的注释比没有注释更危险。改写后的那段要保留它真正还成立的部分：token 不可猜、
撤销立刻生效、**没有第三件东西能顺带溜进这个负载**。

`TestPublicPayloadCarriesNothingExtra` 钉着公开负载的键集合，这次要**明确地**改它，
并在测试里写清楚为什么多了这一个键。

### 3.3 成稿发布 → 她的主页

现状比想象的近：`ListSiteWritingsByUser` / `ListSiteReadingsByUser`
（`queries/pbl_site.sql`）**已经把她完成的每一篇都列在主页上了**，但只是一行死
标题，点不开。

所以「发布」不需要新表：**一篇成稿有 share token 就是已发布**。

- 主页数据里每一项多一个 `publicUrl`（有 token 才有）。
- `/p/:token` 上新增一块**原生渲染**的作品区，列出已发布的那些，每张卡片链到
  `/s/:token`。**阅读和写作同样对待**：两者都有 share token 这一件事，发布走的是
  同一条路。主页原有的那两张列表（她完成过什么）不动，新的作品区只收已发布的，
  因此「完成」和「公开」在主页上是分开的两件事。
- 🚨 **这一块必须在 iframe 外面。** `PublicSitePage` 用
  `sandbox="allow-scripts"` 挂她生成的那个站，那个沙箱里的链接**根本跳不动**
  （没有 `allow-popups`、也没有 `allow-top-navigation-by-user-activation`）。
  把作品区做在 iframe 外面，既不用放宽一个渲染模型生成 HTML 的沙箱，链接也
  一定能用。现成的先例就在同一个文件里：`ProcessComparison` 就是挂在 iframe
  外面的一块。
- 她自己那一面（`/site`，`MySitePage`）在每篇完成的作品上给一颗**发布/停止发布**，
  走的是和报告分享同一条接口。没发布的照常只有她自己看得见。

---

## 四 · 不做

- 跨篇对比、任何会涨会跌的线、分数、排名、徽章、连续天数。
- 公开页上的文章全文。
- 教师端读对话——2026-09-14 那条裁定不动：老师看得见她产出的一切，唯独读不了她
  和印记的对话。这一版给的是**她自己**的选择权，不是给别人开的口子。
- 把三屏做成三条 URL。

---

## 五 · 怎么验

只写「读代码看不出对错」的测试（house rule），不给每个渲染元素写断言：

**Go**

1. `ordinal` 冻结：同一个 atom 再取一次报告，数字不变；她又完成一篇之后，旧报告
   里的数字仍然不变。
2. 轮次编号：超出窗口的编号、负数、重复编号、指向印记而不是她的编号，全部被丢掉；
   留下的那几条的 `student`/`coach` 与 `atom_message` 里的原文**逐字相同**。
3. 公开负载：`include_transcript` 为 false 时，返回的 JSON 里**没有** `transcript`
   这个键（断在真的 JSON 上，不是断在结构体上）；为 true 时每条都带说话人。
4. 撤销分享把 `include_transcript` 归 false。
5. 文章摘录不超过上限，且公开负载里不含正文全文。

**前端（少量）**

6. 卡片消息在只读对话里渲染成静态行，`coachCardOf` 认得的五种类型都不挂真组件。
7. 开场那句在「她自己看」和「访客看」两种语境下的人称是对的。

**真的看一眼**

8. Playwright 截图：新报告、只读对话、公开页（含 `transcript` 开与关两种）。
   上一次报告改版在 344 个绿测试底下导出了一张**全白**的 PNG；这一条不能省。

**LIVE_LLM**

9. `LIVE_LLM=1` 跑一次真模型，确认 `turningPoints` 回得来、编号落在窗口里、
   `why` 不是空话。prompt 改了就要真跑一次——这是 2026-09-03 那条规矩。

---

## 六 · 会动到的文件

**apps/api**
`internal/api/atom_report.go`（ordinal / turningPoints / articleEntry / prompt）·
`internal/api/report_facts.go`（编号过的对话块，**不进 corpus**）·
`internal/api/atom_report_share.go`（`include_transcript` + 公开负载 + 改写那段
裁定注释）· `internal/api/pbl_site.go`（`publicUrl`）·
`internal/store/queries/atom.sql`（计数、share 带一位）·
`internal/store/queries/pbl_site.sql`（列表带 token）·
`internal/store/migrations/0174_atom_report_include_transcript.sql`

**apps/lite-web**
`readings/ReadingRoomHost.tsx` · `writings/FinishedWritingPage.tsx` ·
`reports/TranscriptView.tsx`（新）· `reports/ReportView.tsx` ·
`reports/SharePanel.tsx` · `reports/PublicReportPage.tsx` ·
`site/PublicSitePage.tsx` · `mysite/MySitePage.tsx` · `api/reports.ts`

**共享包**：无。卡 JSON 契约不动。

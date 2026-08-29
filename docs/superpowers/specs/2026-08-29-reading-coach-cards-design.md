# 带读的卡片：印记 提问，她动手 · Design

> 阅读房间的 带读 现在只有一种质地：印记 说一段话，她打字回答。
> 这份 spec 把「一步」从一段散文变成**印记 在回复里递出来的卡片**——
> 卡片的内容由 印记 现场写，选项是文章里真实的句子，她用点、选、指来回答。

**Status:** approved in conversation 2026-08-29 · supersedes one written doctrine
(see 「被推翻的旧规矩」 below).

---

## 为什么改

产品负责人贴出的一段真实 带读 记录：

> 这篇只有四段，我们分几步走：先通读，再精读第2、4段，接着换个角度想，最后聊聊
> 你自己。现在先走第一步——通读。从第一段往下读，留意温差、成因、办法各在哪几段，
> 读完告诉我一声就行。
>
> **读完了**
>
> 好，通读就到这。现在看第二段，这段专门讲城市热岛的形成原因。请把你能找到的原因
> 用笔圈出来，数一数作者一共列了几条，然后告诉我。

他的判断：

> this doesn't feel good, because it feels very text-heavy and students won't have
> patience to read it thoroughly.

问题不在文案太长。问题是**每一步、不管它想让她做什么，都走同一条通道：一段散文，
用打字回答**。房间因此只有一种质地。连着读四轮这样的话，她是在一个聊天框里写作业。

而且这段散文里有一半是**重述已有的结构化数据**：`reading_task` 行本来就带
`label`（"精读重点段"）和 `detail`，模型只是把它们又说了一遍。

## 一句话的方案

**印记 的回复 = 一句短话 + 若干张卡片。** 卡片由 印记 现场撰写，材料取自文章本身，
她通过点/选/指来回答；她的回答结构化地回灌，印记 顺着她的回答问下一级。

---

## 四条设计裁定（都来自 2026-08-29 的对话，按顺序）

### 裁定一：卡片是 印记 提出来的，不是固定的东西

> *"card is AI's propose. instead of fixed thing. the AI gives an interesting
> response, which has cards"*

这正是平台自己的心智模型——AGENTS.md 写的「工具卡 = tool-use 循环里由人来执行的
工具」——只不过阅读房间今天只用它召唤过透镜。**类型固定，内容由 印记 现场写。**

固定类型换来的东西值得单独说：她读到第四篇文章时，应该认得出这就是第一篇那张卡。
认得出的是**形状**，不是内容。

### 裁定二：AI 提供脚手架不是越界（推翻旧规矩）

`reading_routines.go:17-21` 原本写着：模型不许生成带文章内容的任务，因为
「找出作者关于碳排放的三个论据」这句话一出口，「注意到什么」这件事已经被 AI 做完了。

产品负责人推翻了它：

> *"this is ridiculous. why? AI provides scaffolding. you are banning scaffolding."*

**说对了。指出哪里值得看，正是老师大部分时候在做的事。**
「这段在讲成因，你来数几条」没有替她注意，它把她带到值得注意的地方。

⚠️ **实施时必须改掉 `reading_routines.go:14-26` 那段注释**，不能在树里留一条禁止
我们正在建的东西的规矩。

**但这次推翻是有边界的，实施时不要扩大化。** 那段注释里三条理由，倒下的只有第一条，
而且只在「卡片内容」这一层倒下：

| 旧理由 | 结局 |
|---|---|
| 1. 必须通用（模型不许写带文章内容的东西） | **对「有哪些步骤」仍然成立**——routine 库继续写死，步骤本身继续是通用的。**对「步骤里 印记 递给她什么」不再成立**——那是脚手架，由 印记 现场写。 |
| 2. 必须稳定（她要认得出方法） | **完全活着**，由裁定一的「固定类型」承担：认得出的是卡片的形状。 |
| 3. 必须便宜且诚实（铺流程不该烧模型调用） | **完全活着**：routine 的挑选仍然是确定性的，卡片只在本来就要发生的那次 turn 调用里顺带产出。 |

### 裁定三：卡片的材料来自文章

产品负责人否掉了我提的 `count`（「作者列了几条原因？」`( 2 )( 3 )( 4 )( 5+ )`）：

> *"this is bad ... easily to 乱选, and not directly correlated with reading."*

这条批评推广开来，成了整套卡片的准入测试：

> **一张卡片值得建，当且仅当它的答案不进文章就产生不出来。**

`count` 两头都不及格：四个按钮是 25% 蒙中率，而且就算蒙对了「3条」，那也只是一个
数字，不是她读过的证据。按同一把尺，抽象选项的 `single_choice`
（「这段是在解释原因还是举例子」）同样不合格——不读一句话也能猜。

所以**卡片的材料是文章本身**，不是抽象标签。这正是 `hunt` 那条已经写下的道理：
「打字的答案可以凭印象给，点出来的句子不能。」

### 裁定四：问题是梯子，不是考试

> *"the questions are ladders, we use these to support students to dive to think
> deep to connect his own interests. we do use them as gate sometimes to avoid
> students' 胡乱阅读, but we donot serve as an exam, 不要变成标准化考试, we think
> each child's thoughts are precious"*

这一条化解了裁定三差点带来的反效果：`match_spans`、`order_spans` 这类卡有标准答案，
**有标准答案的卡离打分只差一步**。

关键在于**锚定和开放各管一件事，两件可以同时要**：

- 卡片的材料取自文章 → 她**不读就答不出**。这就是那道门，而且这道门已经够了。
- 卡片问的是**她的判断，不是正确答案** → 没有东西可批改，没有东西可打分。

「这四句里哪一句最能撑住作者的观点」有标准答案。
「这四句里哪一句你读着最不服气」没有——**而她照样得把四句都读完才能回答**。
同样的门，不是考试。

routine 库里本来就有这个直觉：`zh-narrative` 写的是「点出你觉得**写得最好**的那一句
——不是最重要的，是最好的」；`en-close-read` 写的是「点出你觉得最难、但现在读懂了的
那一句」。卡片要把这件事**系统化**。

---

## 梯子长什么样

以那篇城市热岛为例：

```
印记：这四段里第二段在讲成因。

┌─────────────────────────────────────────┐
│ 这几句都在讲成因。哪一句你读着最不服气？ │
│  ○ 「城市地表以沥青和混凝土为主……」      │
│  ○ 「建筑密集阻碍了夜间散热……」          │
│  ○ 「空调外机把热量排到室外……」          │
└─────────────────────────────────────────┘

        ↓ 她选了第三句

印记：你挑的是空调那句。
┌─────────────────────────────────────────┐
│ 你不服气的是它说的事实，还是它下的结论？ │
│  ○ 事实不对        ○ 结论跨太大          │
└─────────────────────────────────────────┘

        ↓

印记：那你自己呢——
┌─────────────────────────────────────────┐
│ 你在城市里最热的一次是什么时候？跟他说的 │
│ 对得上吗？（写一句就行）                 │
└─────────────────────────────────────────┘
```

**第一级是门**：四句真实原文，绕不过去。
**第二级把她自己的反驳磨清楚**。
**第三级落到她的生活里**。
全程没有批改，而且 印记 是**顺着她的回答**往下问的，不是照着脚本走。

---

## 卡片类型（v1）

类型固定、内容由 印记 现场写。**每一条的问题都必须是她的判断，不能有唯一正解。**

| 类型 | 她做什么 | 为什么蒙不出来 |
|---|---|---|
| `choose_span` | 印记 给出 3–4 句**这篇文章里真实的句子**，她挑一句 | 要挑就得四句都读 |
| `pick_in_article` | 回文章里去，把某样东西**点出来** | 答案本身就是文章里的一个位置 |
| `short_text` | 用她自己的话写一句 | 蒙不出来（可能空泛，但不是猜） |

**v1 只做这三种。** `multi_choice`（多选）和 `order_spans`（排序）先不做：契约立起来
之后它们只是同一个渲染器的变体，而 `order_spans` 天然带标准答案，要单独想清楚怎么
问才不像考试。

**明确不做：** `count`（裁定三否掉）、任何带 `answer_key` 的卡、任何给卡片答案打分或
判对错的东西。

---

## 契约与保证

### 卡片不带答案

印记 **不预先声明正确答案**。她答完，答案结构化地回灌成一条学生消息
（例如 `她选了：「空调外机把热量排到室外……」`），印记 在下一轮回应它。

理由三条：她答完本来就要走一次模型调用，所以不多花钱；沿用现有的 turn loop；
而且避免模型提前把一个错答案钉死。**这也是「没有东西可批改」在机制上的落实**——
系统里根本不存在一个可以拿来比对的正确答案。

### 用输出类型来保证，不靠 prompt 的礼貌

沿用透镜选句、金句语料同一套做法（`validateReadingPicks` / `validateReadingQuestions`
的先例）：每张卡片过一个校验器，**不合格就丢掉，不渲染残卡**。

- `choose_span` 的每个选项**必须是某个 block 的字面子串**（literal substring）。
  模型编出来的句子因此进不了渲染。
- 选项 2–4 个，互不相同，非空。
- 问题文本有长度上限。
- `pick_in_article` 的目标必须落在真实 block 上。
- 一次回复最多 N 张卡（避免一屏铺满）。

### 印记 永远不渲染对/错

没有 ✓/✗，没有「答对了」，没有分数、没有连胜、没有进度百分比之外的任何计量
（铁律②）。印记 回应她**选了什么**，然后问下一级。

---

## 「你读这篇是为了：点击填写…」

产品负责人第二个要求：

> *"change this: 你读这篇是为了：点击填写… to an active current step, which has
> some animation."*

调查结论支持这么做，而且理由比「不好看」硬得多：

- 这个 bar 在 `apps/web/src/studio/reading/ReadingRoom.tsx:558-610`，是
  `renderCoach` 接缝**上面**的兄弟节点，所以 lite 原样渲染它。
- 它旁边的「这篇材料用在哪个阶段」下拉框已经被 `caps.mode !== "lite"` 挡掉了
  （同文件 `:596`），所以 lite 学生看到的是一个孤零零的「你读这篇是为了」。
- **在 lite 里它是只写不读的**：写进 `reading_brief` 表，唯一读回它的
  prompt 是 `reading_turn.go:177` 的透镜 `readTurn`，而那个调用只从房间自己的
  composer 触发——正是 lite 用 `renderCoach` 换掉的那条分支。它喂不到任何 prompt、
  报告或评估。

所以：**lite 里把这个位置换成「当前这一步」的活指示器**，带动效。

🚨 **pro 必须不受影响。** pro 那边这个 bar 是活的（喂 `reading_turn.go` 的 prompt，
并且被 `apps/web/test/studio/reading/ReadingRoom.test.tsx:173-234` 覆盖）。

**做法在第 0 步之后就变了，以那边为准**：不是在共享文件里加 `caps.mode` 分支，而是
lite 有了自己的房间文件，那段 JSX **压根没被抄过来**。pro 的文件一个字不改。
（本节写在分家决定之前，保留是因为「为什么这个位置该换掉」的论证仍然成立。）

动效遵循既有约定：仿 `.mk-blockbar`（`apps/lite-web/src/index.css:225-249`，130ms，
`cubic-bezier(0.2, 0, 0, 1)`，`translateY(4px) scale(0.97)` → none），并且必须带
`prefers-reduced-motion` 的关闭开关。
⚠️ `mk-*` 是**裸 CSS 变量**，所有 Tailwind 透明度语法（`bg-mk-x/40`）都不出 CSS，
必须用 `color-mix(...)`。

---

## 散文要缩短

卡片如果只是**加**在现有散文旁边，房间只会更满。所以同一批改动里：

- 系统 prompt 改成：**步骤的指令由卡片承担**，印记 的话是对**她刚做的事**的真实回应，
  不是把 `label`/`detail` 再说一遍。
- 印记 回复的字数上限收紧。
- 「印记 must talk like a teacher」那条规矩仍然管用（有分量、有真实方法名、给选择）
  ——它管的是**回应**的质地，不是重复步骤说明。

---

## 第 0 步：先把 lite 的阅读房间分出来

产品负责人 2026-08-29：

> *"because I'm afraid, we are making them more different, and we may affect pro
> version. maybe it's time for us to separate."*

**同意，而且先做这一步再做卡片。** 判断依据不是 `caps.mode` 分支的数量（只有 2 处），
而是 lite 今天已经在**编造 pro 形状的 props** 来迁就一个它并不共享的接口
（`ReadingRoomHost.tsx:291-293`）：

```tsx
// A lite reading has no project and no reference row — the atom id is
// the only addressing unit, and the api object ignores both anyway.
projectId={readingId}
referenceId={readingId}
```

它还传了一个自己从来不读的 `phaseTag`。这才是该分家的信号。

### 为什么这件事比看上去便宜

**贵的部分本来就已经拆出去了。** 文章表面（span、mark、划选成引用）是
`primitives/annotate` 的 `Annotate`；`HangingCard` / `LensLibrary` /
`ReadingOutcomes` / `FinalizeReadingPanel` 也都是独立叶子组件。分家复制的是**组合**，
不是逻辑——而正在分化的恰恰就是组合。

**lite 的房间会变小，不是变大。** 它可以扔掉自己从来不用的东西：brief bar、
阶段下拉框、证据笔记、`TraceSourcePanel`/追来源、pro 的聊天分支和它的 starter row。
估计 400–500 行，对比 pro 的 1096 行。

**卡片功能因此 0 次触碰 `apps/web`。** 不需要 `renderBrief` slot，不需要 `caps.mode`
分支——lite 直接改自己的文件。上面那个 A/B 选择题自动消失。

### 怎么分

- 新文件 `apps/lite-web/src/readings/ReadingRoom.tsx`，从 pro 的复制过来再删减。
- **继续通过 `@/` 复用叶子组件**（vite alias + tsconfig paths 已经配好）：
  `Annotate`、`HangingCard`、`LensLibrary`、`ReadingOutcomes`、`FinalizeReadingPanel`、
  `ChatMarkdown`、`useReadingLoop`，以及共享的 `ReadingRoom.css`。
- 删掉 lite 用不到的：brief bar（`:558-596`）、阶段下拉框（`:597-616`）、证据笔记、
  `TraceSourcePanel` 与 追来源 按钮、pro 自己的 chat log + composer + starter row。
- **把 `renderCoach` / `renderBlockAside` / `onBlockPick` 的转接拆掉**，直接内联成
  lite 自己的结构——这三个 slot 存在的唯一理由就是当时不想 fork。
- 不再编造 `projectId` / `referenceId`：lite 的房间只接 `readingId`。
- **新样式一律写进 lite 自己的样式表**，不动共享 CSS。

### pro 一个字都不改

🚨 **pro 的 `apps/web/src/studio/reading/ReadingRoom.tsx` 保持原样**，连那三个此后
没人用的 slot 也留着。删它们是**以后可选的**第二步清理，放进这一期只会让 pro 冒险而
换不到任何好处。

### 两条必须写下来的代价

1. **分家只是缩小爆炸半径，不是消除它。** 共享叶子组件照样能弄坏 pro。消失的是
   「改 lite 阅读功能要去编辑一个 pro 正在渲染的文件」这件事。
2. **两个房间会漂移。** 在一边修的 bug 不会自动到另一边。这是明码标价的代价。

## 实现架构（调查后定稿）

### 卡片存在哪：`atom_message.payload jsonb`（新列），**不是 `atom_card`**

`atom_card` 看起来是天然的家，实际上是陷阱，两条硬理由：

1. **`card_id` 必须在注册表里解析得到**——Go 侧 `cards.ByID`（`//go:embed specs/*.json`），
   TS 侧 `CARD_REGISTRY[cardId]`。`summonReadingLens` 明确拒绝解析不到的 id
   （`reading_lens.go:169-175`：「宁可降级，也不铸一张没有渲染器认得的卡」）。
   印记 现场写的卡在两边都没有家。
2. 🚨 **`atom_card_one_open_idx`**（`0096_atom_card_one_open.sql`）是数据库级的唯一
   偏索引：`(atom_id) WHERE status IN ('proposed','active')`。聊天卡片如果以 open 状态
   落在 `atom_card`，**会把这篇文章的透镜召唤全部堵死**；反过来也一样——`lensOK`
   （`reading_coach.go:574-594`）本来就在 `anyOpen` 时拒绝发透镜。一屏上两套
   「同时只能开一个」会互相打架。

所以：**给 `atom_message` 加一列 `payload jsonb`**（默认 NULL）。卡片挂在提出它的那条
AI 消息上，她的回答挂在她那条学生消息上。三个好处：天然按时间排序、刷新即复原、
完全绕开上面两个陷阱。`ListAtomMessages` 现有的 `block_id IS NULL` 过滤不受影响。

### 卡片不走 `CardRenderer` / `CardSpec`

`apps/web/src/cards/CardRenderer.tsx` 确实存在、确实能从 lite import（vite alias
`@/` → `apps/web/src/`）。**但不要用它**：`CardSpec` 的 Zod 要求
`id / category / name / purpose / trigger_condition / steps.min(1) / rubric_tags`
全部齐备（`packages/contracts/src/cardSpec.ts:86-95`），要让模型现场编出这些字段，
只是为了渲染三个选项。**用一个 lite 自己的窄类型**，新组件
`apps/lite-web/src/readings/CoachCard.tsx`。

（记一笔：房间里那张挂在段落下的透镜卡是 `HangingCard.tsx`，**手写的四态机**，
本来就没走 `CardRenderer`。所以「阅读房间会渲染卡片」和「平台有 schema 驱动的卡运行时」
今天是两件互不相干的事——本期不去合并它们。）

### 线上契约

模型输出的 JSON 增加一个可选字段（`reading_coach.go:77-88` 的输出格式段要同步改）：

```json
{"reply":"…","advance":"","focusBlock":"","tool":"","lens":"",
 "card":{"type":"choose_span","prompt":"哪一句你读着最不服气？",
         "options":[{"blockId":"b2","quote":"<必须是 b2 里的字面子串>"}]}}
```

⚠️ **绝对不要在 prompt 文本里新增字面量 `%s`**。`buildReadingCoachSystem`
（`reading_coach.go:383-387`）用的是 `strings.Replace(..., "%s", ..., 1)` 而不是
`Sprintf`，任何新出现的 `%s` 都会被第一次替换悄悄吃掉。段落工具菜单用 `%s`、
透镜菜单用 `%LENS%`，就是为了这个才特意分成两个不同的 token。

校验放在 `parseReadingCoachReply`（`reading_coach.go:414-468`）的 `return got, true`
之前，**照抄 `validateReadingPicks`（`:156-174`）的写法**：

- 每个 option 的 `quote` 必须是它自己那个 block 的字面子串（`strings.Contains`）。
- 存活选项 < 2 → **整张卡片丢掉**（`validateReadingQuestions` 的下限先例：
  「一个空泛的建议比没有更糟」）。
- 选项 ≤ 4、互不相同、非空；`prompt` 有字数上限。
- 不合格静默丢弃，**不报错、不渲染残卡**——turn 本身照常返回。

### 她的回答必须写进 transcript

🚨 今天 `postReadingCoachTurn` 在 `studentText == ""` 时**根本不写学生消息行**
（`reading_coach.go:620-628`）。而下一轮的上下文只由 `atom_message` + 结构化 `picks`
组成（`buildReadingCoachPrompt:238-347`）。所以纯点击的回答对第 N+1 轮**是不可见的**。

因此：她点完之后，服务端**合成一条学生消息**。

🚨 **合成的内容里，引用文章的每一行都必须带 `> ` 前缀。** 这不是风格问题：
- `hasHuntPickEvidence`（`:202-236`）靠重新解析 `> ` 开头的行来判断「她到底有没有指」；
- `report_facts.go` 的 `stripQuotedLines` 靠同一个前缀把文章原文挡在「她自己的话」
  语料之外。

**这个 bug 已经发生过一次**（2026-08-29 报告那次 R4 泄漏：`ReadingCoachPanel` 只给
引用的第一行加了 `> `，多行引用的后几行漏进了她的语料，可以印在她署名的图片上）。
卡片选项**就是**文章原句，是同一条地雷的第二次机会。逐行加前缀，并且在测试里钉死。

### 渲染

- 走 `ChatMessage.node`（`apps/web/src/studio/ai/ChatLog.tsx:22-29`）。`ChatBubble`
  对 `role:"system"` 是**无气泡外壳**渲染（`:74-80`），pro 的四个房间已经用这条路子
  塞 `CardTurnChip`。**共享 pro 代码一行都不用改。**
- lite 侧把本地消息类型放宽成 `LiteMessage | CoachCardMessage`
  （`ReadingCoachPanel.tsx:144-152` 现在已经是全部走 `node` 了）。
- ⚠️ **滚动**：`ChatLog` 只在 `messages.length` 变化时自动滚（`ChatLog.tsx:55-60`），
  `ReadingCoachPanel` 自己也滚一次（`:156-160`）。卡片是**原地长高/变矮**的
  （选中、反馈展开），条数不变 → 不会重新滚。需要显式处理。

### 当前步指示器

分家之后这件事没有任何机关可言：lite 房间里那段 brief bar 在第 0 步就**根本没被抄
过来**，位置空出来给当前步指示器。不需要 slot，不需要 `caps.mode`，不需要新的
capability flag（`capabilities.ts:47-58` 那个没人用的 `comprehensionCheck` 继续别碰）。

指示器读 `tasks`——`ReadingRoomHost` 已经握着它，当前步就是
`tasks.find(t => t.status === "pending")`，和服务端 `currentReadingTask`
（`reading_coach.go:392-399`）同一条规则。

🚨 **pro 那段 JSX（`apps/web/.../ReadingRoom.tsx:558-596`）一行都不要动**，它在 pro
是活功能，且被 `apps/web/test/studio/reading/ReadingRoom.test.tsx:173-234` 覆盖。

### 顺带修掉的一件事

`ReadingPlanRail` 挂在 `lg:` 才显示的侧栏里（`ReadingRoomHost.tsx:283`），所以**窄屏
今天根本没有任何步骤界面**。当前步指示器因此不只是「搬个位置」，它是小屏上的第一个
步骤界面。

## 不在本期

- `multi_choice` / `order_spans` 两种卡型。
- 让卡片进入报告（报告目前只讲她自己的话与透镜发现）。
- 专注计时器。产品负责人提过「scan for one minute」，我提出**只记录、不设卡**的版本
  （挡住进度的倒计时属于铁律②禁止的强制机制），这次对话没有再回到它——**本期不做**。
- pro 阅读房间的任何行为变化。

---

## 全局约束（每个任务都隐含带着这一条）

🚨 **`apps/web/` 下面不允许有任何文件被修改。** 这是整份 spec 里唯一一条可以用机器
判定的 pro 安全保证，也是产品负责人要分家的全部理由：

```
git diff --name-only <base>..HEAD | grep '^apps/web/'   # 必须为空
```

lite 通过 `@/` **import** 共享叶子组件，新样式写进 lite 自己的样式表。任何一个任务只要
觉得「顺手改一下 apps/web 会更简单」，那就是走错了——停下来，在 lite 侧解决。

（`packages/contracts` 同理：除非契约真的变了，否则不要动。）

## 验收

### 第 0 步（分家）

1. lite 的阅读房间由 `apps/lite-web/src/readings/ReadingRoom.tsx` 渲染，不再 import
   `@/studio/reading/ReadingRoom`。
2. 它不再编造 `projectId` / `referenceId`。
3. **`apps/web/` 零改动**（上面那条 grep 为空），pro 的
   `ReadingRoom.test.tsx` 全绿。
4. lite 现有的三个 e2e 走查（`reading-walk` / `coach-walk` / `report-walk`）**行为不变**
   ——分家是一次纯搬迁，学生看到的东西一模一样。

### 第 1 步（卡片）

一次真实走查，读一篇四段的中文文章：

5. 带读 开始后，第一条 印记 回复里**带着一张卡片**，而不是一段三行的散文。
6. `choose_span` 的每个选项都能在文章里**逐字找到**。
7. 点一个选项就能作答——**不需要打字**——并且她的选择结构化地回灌，印记 的下一句
   回应的是她选的那一项。
8. 卡片和她的回答在**刷新后仍在**。
9. 整个过程中没有出现 ✓/✗、分数、正确答案。
10. 「你读这篇是为了」在 lite 里消失，取而代之的是当前步骤指示器。
11. 🚨 **卡片选项进入 transcript 时逐行带 `> ` 前缀**——用一条测试钉死，因为这条
    地雷已经炸过一次。
12. 透镜召唤在有聊天卡片的情况下**依然能用**（没有踩到 `atom_card_one_open_idx`）。

# 觉醒协议 · 重建

2026-09-19。取代 2026-09-02 的七屏兴趣测试（`apps/lite-web/src/tree/quiz/`、
迁移 0119 `interest_quiz`）。

参考设计：`docs/reference/觉醒协议-大模型兴趣探索版-2026-09-18/`。

---

## 1 · 为什么重建

旧的七屏测试停在「长出几个词」。它没有下一步，也不和树上已有的数据发生关系：
做第二次和做第一次问的是同样的问题，产物也一样。

参考设计里有一条完整的探究链（9 个节点：SPARK → FOCUS → CONNECT → TENSION →
QUESTION → READ → THINK → CREATE → PROJECT），产出是研究问题、阅读方向、学科、
作品设想。这些正好是阅读室、探索地图、写作项目要的输入。

### 参考设计里没有的东西

**它的模型对话是假的。** 解码后的游戏文档不发任何网络请求
（`fetch` / `XMLHttpRequest` / `/api` 均为 0 处）。「印记助手」的每一句话都由
五条正则给学生的文本打分，选中一个 profile，再套字符串模板拼出来。
`server.py` 里那次真的 DashScope 调用只被外层 shell 用于平台桥接，9 节点终端
从未连过真模型。

结论：9 节点流程是**未经验证的设计**。prompt 由我们自己写，且必须在上线前跑
`LIVE_LLM=1`（memory: prompt-output-must-be-verifiable）。

**它的 `ready` 和 9 个节点互相矛盾。** `server.py` 的 system prompt 写着
「ready 是语义成熟度信号……不必等到第 9 个流程节点」，而终端按固定 9 步推进。
两者对「她什么时候算做完了」的判断不一致。本设计取消模型自判 `ready`：
节点推进由服务端决定。

---

## 2 · 保留全部阶段

阶段顺序照参考设计，不删减：

| # | 屏 | 内容 | 模型调用 |
|---|---|---|---|
| 1 | boot | 开场剧情，打字机叙述，可跳过 | 无 |
| 2 | world | 序章：加入 / 暂不加入 | 无 |
| 3 | warning | 印记提醒，引出历史档案 | 无 |
| 4 | archive | 历史档案 01 认知让步：三条证据 + 确认题 | 无 |
| 5 | deck | AI 底牌三实验：顺 / 换 / 偏 | 无 |
| 6 | rejoin | 翻完三张牌，重新决定 | 无 |
| 7 | observer | 观察者路线（选择暂不加入时） | 无 |
| 8 | energy | 能量卡牌：靠近 / 沉浸 / 延续 / 轻松 | 无 |
| 9 | navigator | 选印记助手：NOVA / SAGE / KIRO | 无 |
| 10 | terminal | 9 节点兴趣探询 | 每轮一次 `dialogue` |
| 11 | lens | NOTICE → WONDER → TEST 三层 | 无 |
| 12 | challenge | 选一种验证方式 | 无 |
| 13 | talent | 天赋卡牌：选 5 张 → 三堆 | 无 |
| 14 | report | 报告 | 一次 `compose` |

分支：第 2 屏选「加入」直接进 8；选「暂不加入」走 3 → 4 → 5 → 6，在 6 处重新
决定。第 7 屏是主动退出的落点，随时可以回来。

**模型调用只发生在两个地方。** 前七屏一次都不发，所以一个学生走完开场不花钱。

---

## 3 · 数据模型

迁移 0181。旧的 `interest_quiz` 表保留（历史作答是数据），不再有新写入。

```
awakening_run        一次完整作答
awakening_turn       终端里的一轮（节点 / 她的话 / 回复 / 分析）
awakening_report     生成一次的报告，可分享
```

`awakening_run` 关键列：

- `stage` —— 她走到哪一屏。中途退出留一行 `finished_at IS NULL`，下次可以接着走。
- `route` —— `joined` / `observer`，第 2 屏与第 6 屏的选择。
- `energy_profile` jsonb —— 能量卡牌的结果，喂给终端第 1 个节点。
- `talent` jsonb —— 选中的 5 张卡及三堆归属。
- `navigator` —— NOVA / SAGE / KIRO。
- `attempt_no` —— 她的第几次。第 1 次走全程，第 2 次起走短线。

`awakening_turn` 是终端的逐轮记录，同时是**语料**：报告里的每一句引文、树上每个
词的 evidence，都要能在这张表她自己写的那一列里逐字查到。

---

## 4 · 算法：这条链怎么闭合

### 4.1 树只收闭表里的词

迁移 0134 把树的词表关成了闭表：`packages/contracts/interests/interests.json`
里 249 条，每条带 `field` 和 `disciplines[]`。那次迁移把生产库里的词整表归档
之后清空，理由写在迁移注释里。

参考设计产出的是自由文本（`interest_object`、`disciplines: ["心理学"]`）。
它**不能直接种进树**。中间必须有一步选词：从 249 条里挑，不许新造。

### 4.2 两次调用，各管一件事

参考设计把对话和分析放在同一次调用里。这里拆开：

1. **对话**（`gateway.ClassDialogue`，每轮一次）—— 她当场读到的那句回应，
   一次只问一个问题。产出只有文本。
2. **选词**（`gateway.ClassCompose`，结束时一次）—— 输入是她 9 轮写下的全部
   文字，输出是闭表里的 id + 她的原话作 evidence。复用
   `interest.ParseHarvestReply` 与 `interest.KeepGrounded`。

拆开还解决了 `ready` 的矛盾：节点推进由服务端算，模型不再自己宣布结束。

### 4.3 evidence 必须逐字可查

`KeepGrounded` 要求 evidence 是她自己写的文字的子串，否则这个词不落库。
参考设计的 prompt 写的是「学生原话支持的具体行为」——「支持」这个说法允许转述，
转述过不了逐字比对，结果是一个词都长不出来。prompt 改写成「原样摘录」，
代码这一侧照旧强制。

语料 = `awakening_turn.student_text` 的全部，**不含模型说过的话**
（memory: prompt-twice-then-make-it-checkable）。

### 4.4 三种写回

采集今天只会做一件事：种一个词。这里分三种：

| 判定 | 含义 | 机制 |
|---|---|---|
| `confirm` | 树上已有的词又出现了 | 新增一条 `keyword_source` → 强度上升 |
| `grow` | 闭表里的新词 | 新增一行 `interest_keyword` |
| `open` | 她一个词都没有的主枝 | 记在报告里，交给树上已有的空枝邀请 |

判定在服务端做，不问模型：选词结果里的 id 和她树上已有的 id 求交集，
交集是 `confirm`，差集是 `grow`。

强度机制已经就位：`strengthLadder = [1,1,2,3,3,4,4,4,5]` 按来源条数取值，
`keyword_source` 的 `UNIQUE (keyword_id, kind, ref_id)` 用这次 run 的 id 作
`ref_id`，所以第二次作答给同一个词再添一条来源，而不是覆盖。

### 4.5 终端知道她的树

`buildTerminalContext` 带上一份树简报：强度最高的若干个词、哪几根主枝是空的、
上次作答在什么时候。

- 第一次：第 1 个节点照常问「最近有什么让你主动点开」。
- 第二次起：第 1 个节点改成从她树上已有的词出发，问那条线还在不在。

树简报由服务端算好后写进 prompt，**不让模型每轮自己数**
（memory: hardcoded-thresholds-vs-user-set-scale）。

---

## 5 · 报告

一次 `compose` 调用生成，落 `awakening_report`，之后只读不重算
（照 `ensureAtomReport` 的 advisory lock + 复查 + 生成一次）。

六块：

1. **她在追什么** —— 这次动了的词，每个词下面是她自己的那句话
2. **可能的驱动力** —— `motivation_hypotheses`，带 confidence，按假设显示
3. **她的问题** —— `research_question`
4. **怎么靠近** —— 天赋三堆
5. **下一步** —— 阅读 / 写作 / 项目
6. **和上次比**（第二次起）—— 哪些词变强了、哪些是新的、问题变了没有

第 5 块的阅读**只从真实库里选**：用 `analysis.disciplines` 对
`apps/api/internal/library/articles.json`（48 篇，带同一套 discipline id）做匹配。
匹配不到就写「库里暂时没有对得上的材料」，不补文案、不编标题。

导出复用 `apps/lite-web/src/reports/exportPoster.ts`（html-to-image → PNG），
分享复用 `atom_report_share.go` 的 token 机制。

---

## 6 · 资产

参考设计目录里 `assets/` 共 45 个文件，游戏真正引用的只有 21 个
（其余 24 个属于那份平台 mock —— 而那份 mock 是我们自己的 lite-web 构建产物，
整个丢弃）。文档里没有 base64 资产，3.9MB 是两份被 base64 的 HTML 文档。

要上传的 21 个：

- 图 4 个：`nova-assistant.png`（1MB，转 webp）、`education-warning.jpg`、
  `legacy-archive-background.svg`、`legacy-wasteland-background.svg`
- 音 17 个：`{hot,sage,dark}-{quote,lens,connect,challenge,result}`
  + `system-intro-legacy` + `system-warning`

走公开桶 + CDN（`mind-open` / `mind-assets.uni-robot.cn`），路径
`awakening/v1/…`，不走 `internal/oss` 的签名下载：一张背景图的 URL 过期没有意义。
前端一份 manifest 模块作单一真相源。

音频默认关闭，一个常驻开关，任何一屏都不等音频。

---

## 7 · 门槛

进门不是在树上盖一层对话框。树本身暗下去，让位给这个房间，一次动作，不跳路由。
房间里不出现 lite 的导航、页头、返回按钮。出来是同一个动作反过来，落在报告上，
回到暖色。

---

## 8 · 删除

- `apps/lite-web/src/tree/quiz/`（整个目录）
- `apps/api/internal/interest/quiz.go` 及其测试
- `apps/api/internal/api/interest_quiz.go` 及其测试
- 三个 `/api/v1/interest/quiz*` 路由

`interest_quiz` 表和历史行保留。

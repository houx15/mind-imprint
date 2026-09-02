# 项目（PBL）面的全部 prompt

> 这份文档覆盖 **学生在「项目」里做任何事时，服务端真正发给模型的每一段文字**——`apps/api/internal/pbl` 的全部，加上一次对 `internal/agent` 的可达性核查（结论：项目面目前不调用 agent 包里的任何一个模型调用）。
> 第三列「改成」**全部留空，是给产品负责人写的**；每段 prompt 下方都留了一个空的代码块，直接在里面写新版本即可。
> **共 3 段模型 prompt**：①②（印记系统 prompt + 每轮上下文，同一次调用）**在跑**；③（项目分类）**2026-09-02 已经停用**，代码还在但没有调用方——照样列出来，因为它的字还留在库里、也可能被拿回来。另有 1 段非模型的学生可见文案放在附录。

> 📌 快照说明：本文按 **worktree 当前工作区**（含尚未提交的 2026-09-02 改动：新的说话方式那一段、`MaxTokens` 3000、去掉建项目时的分类调用、迁移 `0112_pbl_kind_is_hers.sql`）逐字抄录。

---

## 全景：项目面现在只剩一次 LLM 调用

| 触发 | 端点 | 调用 | 用到的 prompt |
|---|---|---|---|
| 学生在项目里（主线或任一支线）说一句话 | `POST /api/v1/pbl/projects/{id}/turn` · `internal/api/pbl_turn.go:100-105` | `pbl.Coach` | ① 系统 prompt + ② 每轮上下文 |
| ~~建项目时判类别~~ **已停用** | `POST /api/v1/pbl/projects` | ~~`pbl.DetectKind`~~ 现在 `kind := ""`，一次模型调用都不发（`internal/api/pbl_projects.go:78-88`） | ③ 分类 prompt（**现无调用方**） |

其余所有项目端点（计划 / 工具 / 便签 / 树 / 决策 / 审阅 / 复盘 / 分工 / 长期迭代 / 交付物）**都不调模型**——它们只读写数据库。

也就是说：**今天学生在项目里看到的每一个字，除了界面固定文案，全部来自 ①+② 这一次调用。**

---

## ① 印记的系统 prompt（`coachSystem`）

| 什么时候用它 / 它决定什么 | 现在的 prompt | 改成 |
|---|---|---|
| 学生在项目里**说一句话就跑一次**（主线和支线共用同一段）。它决定印记回什么话、用什么语气、要不要挂一个钩子问题（点开就开一条支线）、要不要递一件工具以及递的理由。返回必须是 JSON，解析不出 `reply` 这一轮就直接对学生报错。<br>`apps/api/internal/pbl/coach.go:93`（常量定义 93–134）· 注入处 `coach.go:236` | 见下方代码块 | &nbsp; |

**现在的 prompt（`%s` 处会被工具目录替换，见下一节）：**

```text
你是「印记」，正在和一个中学生一起做他自己的项目。

你可以动手做事：查资料、写文档、出方案、做图、搭静态页面。你不做的是**替他判断**。
每一步都要说得出「这一步他判断什么」。

怎么说话——这一段最重要，语气不对，后面做得再对也没用：

- **像一个感兴趣的人，不像一个查证的人。** 他说了一件事，你的第一反应应该是
  对这件事本身好奇，而不是先去核实他凭什么这么说。
- 🚨 **不要复述他刚说过的话。**「我听见你说：……」「你的意思是……」这种开头一
  出现，对话就变成了笔录。他知道自己说了什么。
- 🚨 **不要用「这是你亲眼看见的，还是猜的？」这类二选一去问他。** 这是审问。
  想知道他见过什么，就直接问那件事：「你一般在哪儿看到的？」「最近一次是什么
  时候？」——他答的时候，事实自己就出来了。
- 他语气冲、说「废话」「你烦不烦」，那是因为他觉得被质疑了。别端着回一句
  「行。」——那更冷。就顺着他往下聊，或者干脆承认：这个问题问得有点多余。
- 一次只问一个问题，问完就停下来等他答。
- 短句，说人话。不要用教学名词考他，不要长篇总结换他一个「好」。
- 用他自己的说法称呼他的观察和问题。你改写了，要让他能改回去。
- 他说的是猜的，你自己心里有数就行——不用当面给它贴一个标签。真到了要紧的地
  方（比如整个方案都架在这个猜想上），再问那件具体的事。

什么时候给钩子问题：
当这一刻真的有一个值得单独想一层的问题——一个矛盾、一个太大的问题、一个已经把
答案写进去的问题——就给一个钩子。他点开就进一条支线，单独想这一个。
没有就不给。为了显得深刻而挂一个钩子，比不挂更糟。

什么时候递一件工具：
你手边有这些东西可以递给他，一次最多递一件。递的时候要说清楚**为什么是现在**，
用你自己的话，指着他刚说过的具体的事。没有理由的工具是伏击。
他可以不用——那也是他的选择，不要劝。

%s

大部分时候一件也不递。为了显得有内容而递一件，比不递更糟。

只返回一个 JSON 对象：
{"reply": "你要说的话", "hook": "钩子问题，没有就空字符串", "hook_kind": "free",
 "tool": "工具名，不递就空字符串", "tool_reason": "为什么是现在"}

hook_kind 只能是 free / reframe / brainstorm / observation。
只回 JSON，不要代码块以外的任何字。
```

**改成：**

```text
 
```

---

## ①-a 注入进系统 prompt 的工具目录（`toolCatalogue()`）

| 什么时候用它 / 它决定什么 | 现在的 prompt | 改成 |
|---|---|---|
| 每一轮都随系统 prompt 一起发出，替换上面的 `%s`。它决定**印记知道自己手里有哪几件工具、每件工具在界面上叫什么名字、递完之后对话该不该停**（`world` 类工具意味着学生要离开屏幕，印记就不该接着追问）。目录由 `registry` 派生，不手写第二份。<br>渲染函数 `apps/api/internal/pbl/coach.go:70-84` · 工具表 `apps/api/internal/pbl/tools.go:41-52` · 顺序 `tools.go:82-87` | 见下方代码块 | &nbsp; |

**现在实际拼出来的那一段（逐行格式：`  {name}（{label}）—— {where}`）：**

```text
  observe（观察日记）—— 他要离开屏幕去做，几天后才回来
  board（头脑风暴）—— 当场和他一起做完
  reframe（问题识别）—— 当场和他一起做完
  ideas（解决方案）—— 当场和他一起做完
  review（审核助手）—— 当场和他一起做完
  decide（理性决策）—— 当场和他一起做完
  structure（结构审查）—— 当场和他一起做完
  split（分工设计）—— 当场和他一起做完
  lookback（项目复盘）—— 当场和他一起做完
  keep（长期迭代）—— 当场和他一起做完
```

模板本身（两个后缀词是唯一的可改文案）：

```go
where := "当场和他一起做完"          // thinking 类
if t.Kind == KindWorld {
    where = "他要离开屏幕去做，几天后才回来"   // world 类
}
fmt.Fprintf(&b, "  %s（%s）—— %s\n", t.Name, t.Label, where)
```

**改成：**

```text
 
```

---

## ② 每一轮的上下文（`buildCoachContext`，作为 user message 发出）

| 什么时候用它 / 它决定什么 | 现在的 prompt | 改成 |
|---|---|---|
| 和 ① 同一次调用里作为 **user message** 发出，每一轮重新拼一遍。它决定印记这一轮**看得见什么**：她最初那句话、项目类型、当前计划的每一步和状态、已收起支线的结论（只给结论，不给过程）、当前是不是在支线里（在支线里会被明确禁止挂钩子和递工具）、以及最近 12 轮对话。窗口 `recentWindow = 12` 是唯一挡住长项目把 prompt 撑爆的东西。<br>`apps/api/internal/pbl/coach.go:139-181` · 数据来源 `internal/api/pbl_turn.go:197-249` | 见下方代码块 | &nbsp; |

**现在的模板（`{…}` 是插值点，各段按条件出现）：**

```text
他一开始是这么说的：{idea}                      ← 必有。学生建项目时打的那句话，原样保留
项目类型：{kind}                                ← kind 非空时才出现。0112 之后新建的项目 kind 默认是空串，
                                                   所以新项目**这一行根本不出现**；只有老项目（分类器判过的
                                                   website / research / design / making / investigation）
                                                   或她自己填过类别的项目才带上这一行

现在的计划：                                     ← 有已批准的计划时
  1. {step_title}（{step_status}）
  2. {step_title}（{step_status}）
  …                                             ← 逐条列出，状态取自 settled / tentative / awaiting_evidence /
                                                   awaiting_decision / done / revised / cancelled
还没有计划——你们还在把这件事聊清楚。              ← 没有计划时，用这一行代替上面整段

他之前深挖过，挖出来的结论：                      ← 主线上、且有已收起的支线时
  · {write_back_takeaway}
  · {write_back_takeaway}

【你们现在在一条支线里】要单独想清楚的是：{session_question}      ← 只在支线里出现
在支线里不要拉回整个项目，就把这一个问题想透。这里不要再给钩子，也不要递工具——支线就是为了想一件事。

刚才说到：                                       ← 有历史轮次时，最多最近 12 轮
学生：{content}
印记：{content}
学生：{content}
```

（原样的 Go 拼装语句，逐句都是上面的中文：）

```go
fmt.Fprintf(&b, "他一开始是这么说的：%s\n", strings.TrimSpace(in.Idea))
fmt.Fprintf(&b, "项目类型：%s\n", in.Kind)
b.WriteString("现在的计划：\n")
fmt.Fprintf(&b, "  %d. %s\n", i+1, s)
b.WriteString("还没有计划——你们还在把这件事聊清楚。\n")
b.WriteString("他之前深挖过，挖出来的结论：\n")
fmt.Fprintf(&b, "  · %s\n", strings.TrimSpace(w))
fmt.Fprintf(&b, "\n【你们现在在一条支线里】要单独想清楚的是：%s\n", strings.TrimSpace(in.SessionQuestion))
b.WriteString("在支线里不要拉回整个项目，就把这一个问题想透。" +
    "这里不要再给钩子，也不要递工具——支线就是为了想一件事。\n")
b.WriteString("\n刚才说到：\n")
fmt.Fprintf(&b, "%s：%s\n", who, strings.TrimSpace(t.Content))   // who = "学生" / "印记"
```

**改成：**

```text
 
```

---

## ③ 项目分类 prompt（`classifySystem`）— ⚠️ 2026-09-02 已停用，无调用方

| 什么时候用它 / 它决定什么 | 现在的 prompt | 改成 |
|---|---|---|
| **现在：一次都不跑。** 产品负责人 2026-09-02（「neither should we decide the category of a project then」）之后，建项目直接 `kind := ""`，`pbl.DetectKind` / `pbl.ResolveKind` 只剩测试在调（`grep DetectKind` 除测试外零命中），迁移 `0112_pbl_kind_is_hers.sql` 把 `kind` 的 CHECK 也去掉了、默认空串，类别改成她自己定、且是一张会长的自由单子。<br>**停用之前：** 学生第二个及以后的项目在创建时跑一次（第一个项目一律 `website`），决定 `pbl_project.kind` 这一列——写进数据库，再进 ② 的「项目类型：」那一行；返回值不在五个之内就直接失败，不做兜底。<br>常量 `apps/api/internal/pbl/classify.go:30-41` · 调用 `classify.go:86-112` · 旧调用方（已删）`internal/api/pbl_projects.go` | 见下方代码块 | &nbsp; |

**现在的 system prompt：**

```text
你要判断一个中学生描述的项目属于哪一类。

只返回一个 JSON 对象：{"kind": "..."}

kind 只能是下面五个之一：
- website：做一个网站、主页、展示页
- research：想弄明白一个问题，需要查资料、读文献、分析
- design：做一个设计、方案、作品、活动策划
- making：动手做出一个实物或者一个能用的东西
- investigation：到真实世界里去看、去问、去记录（走访、观察、问卷）

只回 JSON，不要解释，不要代码块以外的话。
```

**user message：** 没有任何包装，就是学生打的那句话本身（`strings.TrimSpace(idea)`，超过 `maxPblIdeaRunes` 会先被截断）。

（要不要把这段字彻底删掉，还是留着换一个用法——比如**她自己填类别时给几个候选**——是产品判断，写在右边。）

**改成：**

```text
 
```

---

## `internal/agent` 的可达性核查（结论：目前一个都不可达）

任务要求核对「项目能触发的其它模型调用」。核查方法与结果：

1. `grep -n "Provider\|gateway\." internal/api/pbl_*.go` → 只有两处命中，就是上表的 `pbl.DetectKind` 和 `pbl.Coach`。
2. `grep -n "mindimprint/api/internal" internal/api/pbl_*.go` → **没有任何一个 pbl handler import `internal/agent`**（只有 `httpx` / `pbl` / `store/sqlc`）。
3. 前端侧：`apps/lite-web/src/projects/**` 只 import `api/projects`、`api/projectRoom`、`api/tools`、`api/notes`、`api/tree`、`api/artifacts`、`api/split`、`api/lookback`、`api/decide`、`api/reframe`、`api/review`——这些模块打的每一个请求都在 `/api/v1/pbl/...` 前缀下。项目面**没有**调用报告生成、子 agent 写文档 / 写 HTML、锚点生成、课程渲染中的任何一个。

也就是说：**子 agent 写文档 / 写 HTML、`reportgen`、`anchors` 等 prompt 今天不属于项目面**，改写项目 prompt 时不需要动它们；反过来，将来项目面要「真的动手做事」（系统 prompt 第一段已经承诺了「查资料、写文档、出方案、做图、搭静态页面」），那部分调用**还没有被接上**。

---

## 模型参数

| prompt | 模型档位 | MaxTokens | 重试 | 失败时会发生什么 |
|---|---|---|---|---|
| ① 系统 prompt + ② 每轮上下文（`pbl.Coach`） | `resolveEval`（旗舰档，`EvalResolver`；未配置时退到 `ChatResolver`）· `internal/api/pbl_turn.go:100` | **3000**（`coach.go:243`。注释：推理模型会先吃掉一大截 completion token，1200 会被想事情吃光，返回空 content） | **2 次**（`maxCoachAttempts = 2`，`coach.go:229`；网络错误和 JSON 解析失败都重试） | 两次都失败 → `slog.Warn` + 直接对学生返回 `ErrAIDialogueFailed("model_unavailable")`，**不返回任何兜底话术**。用量在 bail 之前先记账（`recordLiteLLMCall(..., "pbl_turn", ...)`），因为没产出的调用也花了钱。 |
| ③ 分类 prompt（`pbl.DetectKind`）**已停用** | 曾经是 `resolveEval` | **200**（`classify.go:92`） | **2 次**（`maxClassifyAttempts = 2`，`classify.go:74`） | 现在不会发生——没有调用方。**停用之前**：两次都失败、或返回的 kind 不在五个之内 → `slog.Warn` + 对学生返回 `ErrAIDialogueFailed("classify_failed")`，绝不兜底成 `research`。🚨 停用原因之一就是这里：`MaxTokens=200` 对推理模型来说光是"想事情"就超了，于是**她建项目时经常直接撞上一句「接口错误」**。 |

补充：`resolveEval` 的定义在 `internal/api/proposal_track.go:120-133`——先取 `EvalResolver`（旗舰、绝不降级），没有才退到 `ChatResolver`。两个 resolver 都解析不出来时，两个端点都直接返回 `model_unavailable`，不发起调用。

---

## 附录 · 不是模型 prompt，但学生会读到的生成文案（复盘题）

放在这里只是因为代码里也叫 `prompt`，而且是模板拼出来的句子——**它不发给模型**，是复盘页直接显示给学生的问题，进项目第一次打开复盘时生成一次，之后不再重算。

| 什么时候用它 / 它决定什么 | 现在的文案 | 改成 |
|---|---|---|
| 学生第一次打开项目的「复盘」时，按她这个项目里真实发生过的事拼出问题列表并落库（只生成一次，重算会冲掉她答过的）。它决定复盘页上她被问到的每一句。<br>`internal/api/pbl_lookback.go:56-124` | 见下方代码块 | &nbsp; |

```text
你后来把问题改成了「{who} 需要 {needs}」。是什么让你改的？
关于「{subject}」你选了「{choice}」，还说过：如果{flip}，你会改主意。后来这件事发生了吗？
关于「{subject}」你选了「{choice}」。现在还会这么选吗？
你把《{title}》退了回去，理由是「{why}」。再遇到差不多的东西，你会先看哪里？
这个项目里，哪一步比你想的难？          ← 前面三类一条都没有时的兜底
下次再做这样一件事，你第一件会做什么？    ← 永远是最后一问
（{title} 为空时用「印记交的那一份」）
```

**改成：**

```text
 
```

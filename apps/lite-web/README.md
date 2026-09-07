# apps/lite-web · 轻量版前端

思维印记轻量版的浏览器端。React + Vite + TypeScript + Tailwind，纯渲染 + API 客户端——
**不持有密钥、不直连模型**，所有 LLM 调用都在 `apps/api`（Go）那一侧。

线上：<https://mind-lite.uni-robot.cn>　API：<https://mind-api.uni-robot.cn>

这份文件的用途只有一个：**让你在三十秒内定位到你要改的那个文件。**
产品理念、四条铁律、界面文案的九条规矩在仓库根目录的 `AGENTS.md`，不在这里重复。

> **本项目当前的 LITE 设计约束：永远只运行原型，不在本地尝试运行后端或完整产品。**
> 新开发内容严格放在 `apps/lite-web/src/eco/`；线上对应页面只用于参考和源码定位。
> 分支与 Pull Request 流程见本文末尾的「LITE 原型专项协作规则」。

---

## 目录

- [先读这一段：三件会咬人的事](#先读这一段三件会咬人的事)
- [跑原型](#跑原型)
- [怎么找代码](#怎么找代码)
- [目录地图](#目录地图)
- [五个 tab，逐个展开](#五个-tab逐个展开)
- [项目（PBL）· 十件工具的总表](#项目pbl-十件工具的总表)
- [横切关注点](#横切关注点)
- [测试](#测试)
- [构建与部署](#构建与部署)
- [常见改动落在哪](#常见改动落在哪)
- [已知陷阱](#已知陷阱)

---

## 先读这一段：三件会咬人的事

### 1. `@/` 不是本目录，它指向 `apps/web/src`

```
@/…      →  apps/web/src/…      ← 完整版（pro）的源码
@lite/…  →  apps/lite-web/src/… ← 本目录
```

配在三个地方，改一处必须同时改另外两处，否则会出现「本地跑得起来但构建失败」或者反过来：

| 文件 | 管什么 |
|---|---|
| `vite.config.ts` | dev server 与生产构建 |
| `tsconfig.json` 的 `paths` | `pnpm typecheck` |
| `vitest.config.ts` | 单元测试 |

轻量版复用了完整版的设计令牌、`@/ui` 原语（`Icon` / `Pebble` / `AccentProvider` …）、
`@/shell/auth/AuthScreen`、`@/shell/settings/SettingsView`。
`tailwind.config.ts` 的 `content` 也必须包含 `../web/src/**`——**漏掉这一条，
阅读室会渲染成没有样式的裸 DOM**（那些 class 写在 `apps/web` 里，Tailwind 扫不到就不生成）。

> 🚨 **改 `apps/web` 之前想清楚。** 那是完整版的代码，两个产品一起编译它。
> 轻量版不能因为自己方便就改 pro 的行为——需要新令牌就在 `apps/lite-web/tailwind.config.ts`
> 里 extend（那个文件的头注释解释了为什么）。

### 2. 没有路由库

`src/routing.ts` 就是全部路由：`parseLiteRoute(pathname)` 解析，`liteRoutePath(route)` 反解，
`navigate(path)` 推 History 并派发一次 `popstate`；`LiteApp` 监听 `popstate` 重新求值。

**加一条路由 = 改 `LiteRoute` 联合类型 + `parseLiteRoute` + `liteRoutePath` 三处。**
TypeScript 的 switch 穷尽检查会帮你找到剩下的。

### 3. 印记看不见前端传给它的东西

这是全仓库最容易写出「看着做完了、其实没通」的地方。

工具面板的 `onFinish(result, summary)` **只回写对话气泡**。真正喂给印记的是
`apps/api/internal/api/pbl_refeed.go`——它**每一轮都重新读数据库表**。

> **没写进表的东西，印记永远不知道。**

所以做完一件工具要走完整条链，任何一环断掉都长得像功能已完成：

```
算 → 存（migration + query）→ DTO（Go handler 的 struct）→ 客户端（api/*.ts）→ 回灌（pbl_refeed.go）
```

历史上三种反复出现的断法：**存了但 DTO 没暴露** / **后端写好了但界面没调** / **算了但没存**。

---

## 跑原型

### 强制边界

本阶段只运行 `/eco/*` 原型，不运行后端或完整产品：

- 不启动 `apps/api`；
- 不启动 PostgreSQL、Docker Compose 或生产部署服务；
- 不运行完整版 `apps/web`；
- 不运行 LITE 的登录主壳、真实 API 流程或依赖后端的端到端链路；
- 不为了让原型启动而修改 `apps/lite-web/src/eco/` 以外的产品文件。

### 推荐流程

在仓库根目录安装依赖（仅首次或依赖发生变化时）：

```bash
pnpm install --frozen-lockfile
```

随后只启动 LITE 前端开发服务器，并访问 `/eco/*` 对应入口：

```bash
pnpm --filter @mind-imprint/lite-web dev
```

> 启动前确认当前任务对应的入口仍然存在于 `src/eco/`。如果 `/eco/*` 入口已从 `rootElementFor.tsx` 移除，不要为恢复入口修改范围外文件；先说明并等待确认。

原型只应使用 mock 数据、本地状态和原型内的静态资源。原型运行不需要 `apps/api`、数据库或真实密钥。

### 不运行的命令

除非你明确要求改变本协作边界，否则不要运行以下命令：

```bash
cd apps/api && make run
cd apps/api && make migrate-up
pnpm --filter @mind-imprint/web dev
pnpm --filter @mind-imprint/lite-web e2e
pnpm --filter @mind-imprint/lite-web build
```

上面最后三类命令分别可能启动完整版、访问需要后端的端到端流程，或构建并不等同于原型预览；本阶段不执行。

**环境变量**（构建期烘进 bundle）：

| 变量 | 含义 | 生产镜像里的值 |
|---|---|---|
| `VITE_API_BASE_URL` | Go API 的公开源。空 = 同源 | `https://mind-api.uni-robot.cn` |
| `VITE_PRO_APP_URL` | 完整版学校的学生登错了这里时送去哪 | `https://mind-web.uni-robot.cn` |

---

## 怎么找代码

三条路线，按你手上有什么线索选。

### 路线 A · 按 URL 找

入口是 `src/main.tsx` → `src/rootElementFor.tsx`。
**这里分派四个互不相干的 app**，因为后三个的访客根本没有 session：

| 路径 | 挂载 | 说明 |
|---|---|---|
| `/eco/*` | `src/eco/EcoApp.tsx` | ⚠️ **生态原型**，纯静态 mock，无 API 无 session。见「已知陷阱」 |
| `/p/:token` | `src/site/PublicSitePage.tsx` | 她的个人主页，访客那一面。一个人只有一个 |
| `/s/:token` | `src/reports/PublicReportPage.tsx` | 一次阅读/写作的报告分享页。`/s/:token/record` 是「这一篇怎么写出来的」 |
| 其余全部 | `src/LiteApp.tsx` | 需要登录的主壳 |

`LiteApp` 内部（路由定义在 `src/routing.ts`）：

| 路径 | 组件 |
|---|---|
| `/`、`/explore`、**任何无法识别的路径** | `explore/ExploreView.tsx` |
| `/readings` | `readings/ReadingsLanding.tsx` |
| `/readings/:id` | `readings/ReadingRoomHost.tsx` → `readings/ReadingRoom.tsx` |
| `/writings` | `writings/WritingsLanding.tsx` |
| `/writings/:id` | `writings/WritingRoomHost.tsx` |
| `/projects` | `projects/ProjectsLanding.tsx` |
| `/projects/:id` | `projects/ProjectSurface.tsx` → `ProjectRoom.tsx` 或 `SiteStudio.tsx` |
| `/tree` | `tree/TreeView.tsx` |
| `/tree/quiz` | `tree/quiz/AwakeningQuiz.tsx`（满屏，**不带导航轨**） |
| `/settings` | `@/shell/settings/SettingsView`（来自 pro） |

> 落地页是**探索**，不是阅读。未知路径也落到探索，所以一条过期或手敲错的 URL 永远不会死掉。
> 导航顺序本身在说一句话：探索（找到）→ 阅读 → 写作 → 项目 → 我的树（因此长出来）。

### 路线 B · 按功能找

| 我要改… | 去 |
|---|---|
| 左边那条导航轨、tab 顺序、登录后的外壳 | `src/LiteApp.tsx` |
| 一句报错文案 | `src/api/errorText.ts`（永远带上后台原话） |
| 颜色 | `src/shared/tone.ts` + `apps/web/src/index.css` 里的令牌 |
| 明暗主题 | `src/shared/theme.ts`（`bootTheme()` 在首次渲染前跑） |
| 某件工具的界面 | `src/projects/tools/surfaces/<Tool>.tsx` |
| 工具邀请卡上那一行「这是干什么的」 | `src/projects/tools/registry.tsx` 的 `TOOL_TASKS` |
| 工具面板里「打开了现在做什么」那一句 | 各自的界面文件（**故意不共用**，抄两份一定漂移） |
| 报告 / 海报 / 分享 | `src/reports/` |
| 她的个人主页三个版式 | `src/site/themes.ts` + `Essay.tsx` / `Ledger.tsx` / `Magazine.tsx` |
| 兴趣树的形状与动画 | `src/tree/geometry.ts` · `tree.css` · `useFitScale.ts` |

### 路线 C · 按后端端点找

`src/api/` 一个文件对一组 Go handler，**每个文件的头注释都写明了它照着哪个 Go 文件**。
改后端形状时按这张表回来改客户端：

| 客户端 | 后端（`apps/api/internal/api/`，除注明外） |
|---|---|
| `api/client.ts` | `internal/httpx/errors.go`（错误信封 `{error:{code,message,details}}`） |
| `api/auth.ts` | 共享鉴权端点（与 pro 同一个 session cookie） |
| `api/projects.ts` | `pbl_projects.go` |
| `api/projectRoom.ts` | `pbl_sessions.go` · `pbl_turn.go` · `pbl_plan.go` |
| `api/tools.ts` | `pbl_artifacts.go` · `internal/pbl/tools.go` |
| `api/notes.ts` | `pbl_board.go` |
| `api/mission.ts` | `pbl_mission.go` |
| `api/reframe.ts` | `pbl_reframe.go` |
| `api/review.ts` | `pbl_review.go` |
| `api/decide.ts` | `pbl_decide.go` |
| `api/tree.ts` | `pbl_tree.go` |
| `api/split.ts` | `pbl_split.go` |
| `api/lookback.ts` | `pbl_lookback.go` · `pbl_keep.go` |
| `api/artifacts.ts` | `pbl_artifacts.go` |
| `api/site.ts` | `pbl_site.go` |
| `api/readings.ts` · `api/readingRoom.ts` | `readings.go` · `reading_source.go` |
| `api/writings.ts` · `api/writingRoom.ts` | `writings.go` |
| `api/explore.ts` | `explore.go` |
| `api/interest.ts` | `interest.go` · `interest_dig.go` · `interest_harvest.go` |
| `api/interestQuiz.ts` | `interest_quiz.go` |
| `api/reports.ts` | 报告端点 |
| `api/oss.ts` | 预签名上传（OSS 直传，CDN 读） |

> 前端路径是 `/projects`，API 却在 `/api/v1/pbl/projects`——`pbl` 前缀是为了不和 pro 的
> `/api/v1/projects` 撞，而学生的地址栏没有这个冲突要避。

---

## 目录地图

```
src/
├─ main.tsx              入口。bootTheme() 然后 createRoot
├─ rootElementFor.tsx    四个 disjoint app 的分派（见路线 A）
├─ LiteApp.tsx           登录 boot + 导航轨 + tab 分派
├─ routing.ts            全部路由逻辑（无路由库）
├─ index.css             @import pro 的令牌 + lite 自己的 .mk-* 类
│
├─ api/                  每个文件 = 一组 Go handler 的客户端（见路线 C）
├─ shared/               tone / theme / Progress / PromptTile / useAlive / useHeartbeat
│
├─ explore/              探索 · 今日新闻星图
├─ readings/             阅读室（lite 自己的，2026-08-29 从 pro fork 出来）
├─ writings/             写作室（用独立原语现搭的，没有可复用的 pro 房间）
├─ projects/             项目 / PBL —— 最大的一块，见专门一节
│  └─ tools/surfaces/    十件工具，一个文件一件
├─ tree/                 我的树 · 兴趣关键词模型
│  └─ quiz/              觉醒协议（兴趣测试，七屏）
├─ reports/              一次会话的报告、海报、分享
├─ site/                 她的个人主页（三个版式 + /p/:token 访客面）
│
├─ eco/                  ⚠️ 生态原型。mock 数据，和上面所有东西都不通
└─ test/setup.ts         vitest 的 setup
```

---

## 五个 tab，逐个展开

### 探索 · `src/explore/`

每天五颗星，从科学源抓来、模型选出。**这是每天回来看一眼的理由。**

| 文件 | 干什么 |
|---|---|
| `ExploreView.tsx` | 整个星图页 |
| `Planet.tsx` | 一颗星球 |
| `NewsSheet.tsx` | 一颗星被打开。**先读，再问，最后才是收藏** |
| `useExploreToday.ts` | 今天的星图数据 |
| `explore.css` | 星图的动画与布局 |

### 阅读 · `src/readings/`

| 文件 | 干什么 |
|---|---|
| `ReadingsLanding.tsx` | 前门。历史在这一页的抽屉里（`ReadingHistoryPanel.tsx`），**不在侧边栏** |
| `ReadingRoomHost.tsx` → `ReadingRoom.tsx` | 房间本体 |
| `ReadingCoachPanel.tsx` | 带读：印记主导，她不用管阶段 |
| `CoachCard.tsx` | 印记把这一步递到她手上让她点 |
| `BlockToolbar.tsx` / `BlockToolsPanel.tsx` | 点一段，工具浮在手边 / 把这一段拆给她看 |
| `ReadingPlanDial.tsx` · `StepIndicator.tsx` | 进度盘 / 第几步 |
| `ReadingQuestions.tsx` | 读完之后留下的问题 |
| `ThinkingFold.tsx` | 印记这一轮的思考过程，默认折起来 |
| `LiteChatMarkdown.tsx` | 聊天气泡的 markdown 渲染 |
| `recommendations.ts` | 「不知道读什么？」的种子数据 |

### 写作 · `src/writings/`

三步：**结构 → 段落 → 成稿**（指示器在 `StageMap.tsx`）。

| 文件 | 干什么 |
|---|---|
| `WritingsLanding.tsx` | 前门（骨架同阅读） |
| `WritingRoomHost.tsx` | 房间本体 |
| `WritingSetupModal.tsx` | 新建之后的第一屏 |
| `PlanningView.tsx` + `MindMap.tsx` | 结构：满屏的规划对话 + 右侧生长的画布 |
| `SnippetsStage.tsx` | 段落 |
| `ComposeStage.tsx` + `ProseSurface.tsx` | 成稿：**她自己写**（铁律①：AI 绝不代写正文） |
| `CommentPanel.tsx` | 印记的结构化批注 |
| `DeepenDrawer.tsx` | 深入一层：只谈某一个 block 的支线 |
| `GuideBox.tsx` · `VocabExamples.tsx` | 引导框 / 方法的实例 |
| `NamePieceModal.tsx` · `EditableTitle.tsx` | 起名 / 标题 |
| `topics.ts` | 「不知道写什么？」的种子数据 |

### 项目 · `src/projects/` → 见下一节

### 我的树 · `src/tree/`

从她真正做完的东西（阅读 / 写作 / 项目）长出来的关键词模型。

| 文件 | 干什么 |
|---|---|
| `TreeView.tsx` | 整页：暗底上一株发光的结构 |
| `useInterestTree.ts` · `liveTree.ts` | 真数据 → 那张画。读 `GET /api/v1/interest/tree` |
| `geometry.ts` · `useFitScale.ts` | 形状 / 缩放 |
| `KeywordDrawer.tsx` · `DigSection.tsx` | 一个关键词被打开 / 继续深挖 |
| `quiz/AwakeningQuiz.tsx` + `quiz/content.ts` | 觉醒协议七屏 + 它的内容 |

> 🚨 取不到数据时它**什么都不渲染**。绝不能回退到示例词——那等于把十六个不属于她的
> 关键词挂在一张写着「这就是你的模型」的图上。

---

## 项目（PBL）· 十件工具的总表

打开一个项目：`ProjectSurface.tsx` 按 `kind` 分派——
`website` → `SiteStudio.tsx`（主页项目），其余 → `ProjectRoom.tsx`。

`ProjectRoom` 是 Cowork 的形状：**左边对话，右边正在做的东西。**

```
ProjectRoom.tsx           左栏对话 + 右栏（WorkPanel）+ 中间可拖的缝
├─ Says.tsx               把印记说的一段话按它本来的形状排出来（要点 → 真的 <ul>）
├─ PaneResizer.tsx        右栏左边那道可以拖的缝
├─ usePaneWidth.ts        右栏宽度，她自己拖，记下来
├─ tools/ToolInvite.tsx   印记把一件工具递到对话里（外加 AwayCard：要出门做的事）
└─ WorkPanel.tsx          右边这一栏
   ├─ PlanPanel.tsx       活的待办清单 + 有东西等确认时的 Plan Check
   ├─ tools/ToolFrame.tsx 每件工具共用的外壳（标题、任务那一句、收工按钮）
   ├─ tools/wide.tsx      把工具铺开占满整个房间
   └─ tools/registry.tsx  工具名 → 界面
```

**十件工具。** 后端 `apps/api/internal/pbl/tools.go` 管「属于哪一类」，
前端 `tools/registry.tsx` 管「长什么样」——**同一张表的两半**。
`kind=world` 的意思是她要离开屏幕，做完再回来。

| 工具 | 标签 | 界面 | 用到的客户端 | 后端 handler | kind | 必须配对的产出 |
|---|---|---|---|---|---|---|
| `observe` | 观察日记 | `surfaces/Observe.tsx` | `mission` `notes` `oss` | `pbl_mission.go` | **world** | — |
| `board` | 头脑风暴 | `surfaces/Board.tsx` | `notes` `oss` | `pbl_board.go` | thinking | — |
| `reframe` | 问题识别 | `surfaces/Reframe.tsx` | `reframe` `notes` | `pbl_reframe.go` | thinking | — |
| `ideas` | 解决方案 | `surfaces/Ideas.tsx` | `notes` | `pbl_board.go` | thinking | — |
| `review` | 审核助手 | `surfaces/Review.tsx` | `review` `artifacts` | `pbl_review.go` | thinking | `artifact` |
| `decide` | 理性决策 | `surfaces/Decide.tsx` | `decide` | `pbl_decide.go` | thinking | `decision` |
| `structure` | 结构审查 | `surfaces/Structure.tsx` | `tree` `notes` | `pbl_tree.go` | thinking | `structure` |
| `split` | 分工建议 | `surfaces/Split.tsx` | `split` `projectRoom` `auth` | `pbl_split.go` | thinking | `substeps` |
| `lookback` | 项目复盘 | `surfaces/Lookback.tsx` | `lookback` | `pbl_lookback.go` | thinking | — |
| `keep` | 长期迭代 | `surfaces/Keep.tsx` | `lookback` | `pbl_keep.go` | thinking | — |

**加一件工具**：`tools.go` 加一行 ＋ `registry.tsx` 加两行（`TOOL_TASKS` 与 `TOOL_SURFACES` 各一处）
＋ 一个 `surfaces/` 文件 ＋ 在 `pbl_refeed.go` 里让它的产出回灌到印记。

表里没有的工具也能被召出来（工具箱是开放的），这时界面退回一张朴素卡片——功能少，但不会白屏。

> **每件工具都必须闭环**：有一个收工信号，**并且**它的内容能到印记那里。
> 少了后半句，她做完之后印记像什么都没发生一样。参见开头第 3 条。

---

## 横切关注点

### 颜色 · `src/shared/tone.ts`

七档马卡龙令牌，每档三个值：

```
--mk-<c>      实色——小圆点、进度弧、描边这种要「实」的地方
--mk-<c>-bg   淡底——整块背景
--mk-<c>-fg   深字——压在淡底上读得清
```

档名：`mist` `taro` `peach` `matcha` `berry` `lake` `butter`。
对错、成败这一类**状态**用 semantic 那组，不用马卡龙（「答过了」是状态，不是类别）。

两条硬规矩：

1. **不许自己编十六进制。** 写死的颜色不跟暗色模式走，暗色下会亮得刺眼。
2. **不许用左侧色条。** 一整列卡片每张挂一条竖色带，列表一长就成一排栅栏，
   颜色喧宾夺主反而看不出内容。用 `-bg` 淡底 ＋ `-fg` 深字。

### 报错 · `src/api/errorText.ts`

`apiErrorText(err)`——**永远带上后台真正说了什么**，学生和我们看到同一句。
文案格式是「动词＋失败：{后台原话}」，见 `AGENTS.md` 第 8 条。

### 界面文案

九条规矩全在 `AGENTS.md` § 界面文案怎么写。最常犯的错是**把界面写成了对话**：
印记在对话里说话，界面只负责给东西命名。标签是名词（`结论`，不是`你的结论，一句话`），
按钮说会发生什么（`确认选择`，不是`就这么定`），状态用 `已 / 待 / 中`。

### 其它

| 文件 | 干什么 |
|---|---|
| `shared/theme.ts` | 明暗。`bootTheme()` 在首次渲染前跑，暗色用户不会先闪一下白 |
| `shared/useAlive.ts` | 「这个组件还在屏幕上吗」，扛得住 StrictMode 的双次挂载 |
| `shared/useHeartbeat.ts` | 报告里「花了多少分钟」的客户端那一半 |
| `shared/Progress.tsx` · `shared/PromptTile.tsx` | 工具面共用的进度盘 / 两个 landing 的「不知道读什么？」瓦片 |

---

## 测试

```bash
pnpm test                       # 全量
pnpm test -- routing            # 按文件名筛
pnpm typecheck
```

测试放两处，**分工是刻意的**：

- `test/*.test.tsx` —— 需要渲染的（Testing Library ＋ jsdom）
- `src/**/*.test.ts` —— 纯逻辑，和被测文件放在一起

**只写「读代码看不出对错」的测试。** 纯函数与它的边界、reducer、请求响应的整形、解析器。
**不要**为每个组件写「标题渲染了 / 类名存在 / prop 传下去了 / 列表有 N 项」这类断言——
写起来和维护起来都贵，每次正常改版都会碎，而且抓不到真正会坏的东西。

> 🚨 2026-08-30 的教训：344 个测试全绿，而导出的报告 PNG 是**全白的**。
> **UI 要用真浏览器看**（Playwright 截图），不是靠 jsdom 断言——jsdom 看不见布局、
> `sandbox`、光栅化。

### e2e

```bash
pnpm e2e                                                    # 本地
playwright test -c e2e/online.config.ts                     # 打生产
```

| 文件 | 说明 |
|---|---|
| `e2e/playwright.config.ts` | 本地。端口 **5174**（好让它和 pro 的 5173 并存），串行 ＋ `retries:1` |
| `e2e/online.config.ts` | **同一批 spec 打生产**。`retries:0`（重试会再走一次真模型、再造真数据），trace/video/screenshot 全开 |
| `e2e/run-stack.sh` | 本地整套栈的生命周期由这个脚本管，所以 config 里没有 `webServer` 段 |
| `e2e/globalSetup.ts` | 把种子学校改成 **lite 版**并登录一次。少了这步整条 walk 会在空白落地页上失败，看起来像前端 bug |
| `e2e/*-walk.spec.ts` | 按面分：homepage / reading / writing / projects / tools / card / coach / report |

---

## 构建与部署

```bash
bash .deploy-local/deploy.sh lite    # 在仓库根目录跑
```

> 🚨 部署脚本会 SSH 上去 **reset 到 `origin/main`**——**没推的代码不会上线。**
> 先 push 再 deploy。

`.deploy-local/` 装着服务器凭据，**在 `.gitignore` 里**，所以你新 clone 的仓库里没有这个目录。
需要部署权限找项目负责人拿；密钥只在 `apps/api` 的服务端 env，绝不进 git、日志或错误信息。

`Dockerfile` 的**构建上下文必须是仓库根目录**，不是本目录：轻量版从 `apps/web/src`
引房间组件，还依赖 `@mind-imprint/contracts`，上下文缩到 `apps/lite-web` 两个都看不见。

```bash
docker build -f apps/lite-web/Dockerfile \
  --build-arg VITE_API_BASE_URL=https://mind-api.uni-robot.cn \
  -t mindimprint-lite-web .
```

产物是一个 nginx 静态站（`nginx.conf`：SPA fallback 到 `index.html`）。

---

## 常见改动落在哪

| 要做的事 | 动这些文件 |
|---|---|
| 加一个 tab | `routing.ts`（三处）＋ `LiteApp.tsx` 的 `TABS` 与 `main` 分派 |
| 加一条不需要登录的公开页 | `routing.ts` ＋ `rootElementFor.tsx`（**不要**塞进 `LiteApp`） |
| 加一件工具 | `internal/pbl/tools.go` ＋ `tools/registry.tsx` ＋ `surfaces/<X>.tsx` ＋ `pbl_refeed.go` |
| 后端加了一个字段 | migration → `queries/*.sql` → Go DTO struct → `api/*.ts` 的 interface → 界面 → `pbl_refeed.go`。**走完五步** |
| 改一句学生看得见的字 | 先读 `AGENTS.md` § 界面文案怎么写，再改 |
| 加一个颜色 | 不加。用 `shared/tone.ts` 里已有的档 |
| 加一个字号 | `tailwind.config.ts` 的 `extend.fontSize`（**不要**动 `apps/web` 的 base） |

---

## 已知陷阱

### `src/eco/` 是原型，和真代码不通

`/eco/*` 下的整个 app 跑在 **mock 数据**上，没有 API、没有 session。
它存在的目的是给产品负责人看交互，做完就该和 `rootElementFor.tsx` 里那一行分支一起删掉。

> ⚠️ `src/eco/site/` 和 `src/site/` 长得几乎一样，**改之前确认你在哪一边**：
> `src/site/` 是真的（`/p/:token`），`src/eco/site/` 是原型里的那份。
> 同理 `src/eco/data/tree.ts` 是 mock 关键词，真的兴趣树在 `src/tree/`。
>
> 另外，`docs/2026-08-30-ecosystem-prototype-spec.md` 已经**过时**，不要照着它做。

### Tailwind 的 `mk-*` 令牌不能用透明度语法

令牌是裸 CSS 变量（`--mk-taro: #…`），所以 `bg-mk-taro/40`、`text-mk-ink/60` 这一类
**一行 CSS 都不会生成**——不报错，就是没有样式。要透明度用 `color-mix()` 或 `linear-gradient()`：

```tsx
style={{ background: "color-mix(in srgb, var(--mk-taro) 40%, transparent)" }}
```

### 同一变体下的同一属性，赢家由生成顺序决定，不由 className 字符串顺序决定

```tsx
// ❌ 坏的：lg:bg-transparent 和 lg:bg-mk-surface 是同一变体下的同一条属性。
//    谁赢由 Tailwind 生成样式表的顺序决定——线上赢的是 transparent，
//    于是聊天文字直接透过面板显了出来。
className={`lg:bg-transparent ${wide ? "lg:bg-mk-surface" : ""}`}

// ✅ 好的：每个分支只发出一个 bg-*
className={wide ? "lg:bg-mk-surface" : "lg:bg-transparent"}
```

`position`（`lg:static` / `lg:absolute`）同理。这条是从同一个文件里两次独立事故总结出来的。

### 「测试全绿 ＋ 类型干净 ＋ 上次验过」不是功能能用的证据

2026-09-03 线上走查修掉的十个缺陷，**每一个都通过了以上三项**。
其中两个是**在生产环境从来没有工作过**的功能（结构图的分层配色；她挑的那条方案传到印记）。

判断标准只有一条：**在生产环境里，用真数据，亲眼看见它工作。**

---

## LITE 原型专项协作规则

### 工作范围

- LITE 游戏化设计和原型新增内容只放在 `apps/lite-web/src/eco/`。
- 可以读取线上 LITE 页面来核对现状，也可以根据你的截图定位对应源码；线上页面只作参考，不直接修改线上内容。
- 除本 README 与仓库根目录 `AGENTS.md` 的协作规则外，不修改 `src/eco/` 以外的产品代码、样式、素材、配置、文档或部署文件。

### 根据截图定位源码

当你提供线上某个部分的截图时，按以下顺序处理：

1. 先确认截图对应的线上 URL、入口和交互状态；
2. 读取线上页面，核对截图中的文字、布局、按钮和状态；
3. 用截图中的独特文案、URL 片段、组件名或数据字段搜索仓库；
4. 对照 `apps/lite-web/src/` 中的源码，确认页面入口、相关组件、样式和数据来源；
5. 记录线上现状与 LITE 原型目标的差异；
6. 只在 `apps/lite-web/src/eco/` 内实现新的设计。

源码定位记录可以使用以下模板：

```text
截图 / 页面：
线上 URL：
页面状态：
源码入口：
相关组件：
相关样式 / 数据：
线上现状与原型目标的差异：
本次允许新增或修改的文件：apps/lite-web/src/eco/...
```

### 分支与 Pull Request

- LITE 工作必须在独立分支上进行；禁止直接在 `main` 上修改或向 `main` 推送。
- 当前协作分支：`feat/lite-eco-game-design`。
- 后续如拆分新的设计单元，从最新 `main` 创建 `feat/lite-<topic>` 或 `design/lite-<topic>` 分支。
- 提交前必须检查：

```bash
git status
git diff
git diff --name-only main...HEAD
```

- 范围检查必须确认产品文件只位于 `apps/lite-web/src/eco/`；本 README 与根目录 `AGENTS.md` 是协作文档例外。
- 完成可审阅的设计单元后，推送独立分支并提交 Pull Request，目标为 `main`，请求开发者 **houx15** 审阅并合并。
- 不自行合并 Pull Request，不绕过 `houx15` 的审阅。

## 相关文档

| 文档 | 内容 |
|---|---|
| `AGENTS.md`（仓库根） | 四条铁律、硬约束、**界面文案的九条规矩** |
| `docs/思维印记_Demo_PRD.md` | 产品规格 |
| `docs/architecture/` | 数据库 schema · API 设计 · Go 最佳实践 |
| `docs/2026-09-02-ui-wording-table.md` | 444 行文案的逐条改写，九条规矩的来源 |
| `apps/api/internal/pbl/` | 印记的 prompt（`coach.go`）、工具表（`tools.go`）；回灌在 `../api/pbl_refeed.go` |

# 轻量版（Lite Edition）· 原子底座与阅读 / 写作 — 设计 Spec

> 2026-08-26 · 状态：待实现。
>
> **这是程序级（program-level）设计**，覆盖 P1–P3 的整体形状。与仓库既有做法一致，**每一期各自再出 spec → plan → build**；紧接其后的实现计划只覆盖 §10 的 P1。

## 1. 背景与目标

现有产品服务 IB / AP Seminar 学生，围绕一条重量级的**项目生命周期**（立题 → 管理 → 阅读 → 写作 → 回顾）展开。新客户群是**普通学校**：他们不做研究项目，也不需要证据图、提案轨道、essay-track 阶段机。他们要的是**一次阅读练习**、**一次写作练习**——各自独立、可重复、当堂就能完成。

轻量版因此是**独立前端站点** + **独立的一套后端表**。它与现有产品共享的是**能力**，不是**数据结构**。

### 目标

- 独立前端站点，左栏只有两项：**写作**、**阅读**。
- **复用 AI 能力、前端设计与设计 token**；**不复用** project 的表与 handler。
- 阅读新增**理解检测**：读完后 AI 出几道题，验证学生是否读懂。
- 新的**简版报告**，取代项目级的 `EvaluationReport`。
- 写作支持中英双语；**英文写作提供示范段落**。
- 账号按**学校**的 edition 分流。
- **为未来的 AI 聊天、AI 项目预留底座**：加一种新形态 = 加一张表 + 复用底座，而不是把消息、卡片、批注、报告再写一遍。

### 非目标（本轮不做）

- 不迁移现有 pro 的 `project` 到新底座。现有项目的数据结构原样不动；将来若要让 project 成为 atom 的一种，另开 spec。
- 教师布置阅读任务、课堂内小聊天、小型 PBL —— 底座为它们留好形状，但不实现。
- 轻量版不设 图鉴 / 课程 / 项目 三个 tab。
- 不改动现有版本（pro）的任何行为。
- AI 不代写正文（见 §2）。

## 2. 守住的设计铁律

| 铁律 | 在轻量版中的落法 |
|---|---|
| ① AI 绝不代写正文 | 提纲生成与「合成全文」都是**确定性系统步骤**（AGENTS.md 明确允许）：合成只拼接学生自己写的片段，不新造一个字。英文示范段落**明确标注为示范**，与草稿在结构与视觉上分离，永不自动插入。 |
| ② 不操纵 | 无连胜、无排行榜、无徽章、无推送。工具卡自动触发但由学生确认打开，与现状一致。 |
| ③ 一次只问一个 | 陪练对话与引导问题 block 均沿用现有节奏（`ApplyReadingGate` 的 pacing 原样复用）。 |
| ④ 过程即数据 | 跳过工具卡、让 AI 直接答，同样被记录，并进入简版报告的确定性事实区。 |

## 3. 复用边界（本设计的核心判断）

**复用能力，不复用数据结构。** 这条边界之所以成立，是因为阅读的 AI 层本来就与存储无关：

```go
// apps/api/internal/agent/reading_router.go
type ReadingRouteInput struct {
    StudentText    string
    Article        string        // 文章正文，纯字符串
    FocusedSpans   []FocusSpan
    RecentTurns    []string
    Catalog        []ReadingCard
    ScaffoldLevels map[string]int
    Pacing         PacingState
    Brief          ReadingBrief
}
func RouteReading(ctx, p gateway.Provider, resolver gateway.KeyResolver, in ReadingRouteInput) (ReadingDecision, ...)
```

**`ReadingRouteInput` 里没有任何 project 引用**，`RouteReading` / `ApplyReadingGate` / `ResolveExampleAnchor` / `ReadingDeck` 全是纯函数。handler 只要把这些值从自己的表里装配出来即可——AI 这一层一行都不用改。

| 复用 | 不复用 |
|---|---|
| `internal/agent` 的阅读大脑：`RouteReading`、`ApplyReadingGate`、`ResolveExampleAnchor`、`ReadingDeck`、相关 prompt | pro 的 handler（`/projects/*` 一个都不碰） |
| `internal/gateway`：模型路由、降级、token 与成本计量 | project 生命周期（立题 / 计划 / 证据图 / 探索 / essay-track / 回顾） |
| `internal/docextract`：真实 PDF / DOCX 正文抽取 | `project` / `reference` / `material` / `card_instances` / `outline_node` / `snippet` / `draft_snapshot` 等 project 子表 |
| `internal/cards` + `packages/contracts`：卡 spec 与标准信封契约 | `EvaluationReport` 项目级报告 |
| 前端房间组件、交互设计、**设计 token（`mk-*`）与 CSS** | pro 的 shell、导航、图鉴、课程 |

## 4. 领域模型：原子底座

### 4.1 为什么要底座

阅读之后是写作，再之后是 **AI 聊天**与 **AI 项目**。这四者不同的是各自的专属字段，**相同的是四件事**：一条对话消息流、一批工具卡实例、一层批注、一份小报告。若按形态各写一套，这四件事就要写四遍、修四遍。

因此：**薄薄一层共享身份 + 共享机制，专属字段各自成表。**

```
atom                身份：id / kind / user_id / created_at
├── reading         阅读专属字段                （writing / chat / project 之后并列加入）
├── atom_message    对话消息流        ← 四种形态共用
├── atom_card       工具卡实例（标准信封）  ← 四种形态共用
├── atom_annotation 批注层            ← 四种形态共用
└── atom_report     简版报告          ← 四种形态共用
```

加一种新形态 = **加一张专属表 + 复用四张共享表**。共享表用**真外键**指向 `atom`，不使用 `(owner_kind, owner_id)` 这类多态列——那会丢掉引用完整性。

### 4.2 身份与形态

```sql
CREATE TABLE atom (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind       text NOT NULL CHECK (kind IN ('reading','writing')),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_user_kind_idx ON atom (user_id, kind, created_at DESC);

CREATE TABLE reading (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title       text NOT NULL DEFAULT '',
  lang        text NOT NULL CHECK (lang IN ('zh','en')),
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);
```

> `kind` 的 CHECK 随形态增加而放宽（`'chat'`、`'project'`），一行迁移的事。`writing` 表结构与 `reading` 同形，随 P3 落地。

### 4.3 共享机制

```sql
-- 对话消息流。seq 由服务端分配，(atom_id, seq) 唯一，保证顺序可靠。
CREATE TABLE atom_message (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  seq        integer NOT NULL,
  role       text NOT NULL CHECK (role IN ('student','ai','system')),
  content    text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX atom_message_seq_idx ON atom_message (atom_id, seq);

-- 工具卡实例。标准信封结构与 pro 一致（这是过程树与评估的共同地基，不得随意改）；
-- Go 只做边界校验（status 枚举 / id / field_values 为对象 / event_trace 为数组），
-- 内层深结构的真相仍归 packages/contracts 的 Zod 契约。
CREATE TABLE atom_card (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id      uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  card_id      text NOT NULL,
  block_id     text,                    -- 悬挂在正文哪一段（阅读）
  status       text NOT NULL CHECK (status IN ('proposed','active','submitted','skipped')),
  field_values jsonb NOT NULL DEFAULT '{}'::jsonb,
  event_trace  jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  submitted_at timestamptz
);
CREATE INDEX atom_card_atom_idx ON atom_card (atom_id, created_at);

-- 批注层：学生划的线与写的话。
CREATE TABLE atom_annotation (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  block_id   text NOT NULL,
  span       jsonb NOT NULL,            -- {start,end}，与前端 Annotate 的 span 同形
  quote      text NOT NULL DEFAULT '',
  note       text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_annotation_atom_idx ON atom_annotation (atom_id, created_at);

-- 简版报告（P2 起用）。沿用既有 evaluation 的原子 ON CONFLICT 单飞 + 三态 + 轮询。
CREATE TABLE atom_report (
  atom_id      uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  status       text NOT NULL CHECK (status IN ('pending','ready','failed')),
  payload      jsonb,
  error        text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  generated_at timestamptz
);
```

### 4.4 阅读专属

```sql
-- 文章正文：一次阅读只有一篇。body 是抽取后的可读正文（复用 internal/docextract）。
CREATE TABLE reading_source (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title       text NOT NULL DEFAULT '',
  body        text NOT NULL,
  source_url  text,
  bib         jsonb,
  ingested_at timestamptz NOT NULL DEFAULT now()
);

-- 读前定位：为什么读、读什么。直接喂给 ReadingRouteInput.Brief。
CREATE TABLE reading_brief (
  atom_id        uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  phase_tag      text,
  reading_reason text NOT NULL DEFAULT '',
  reading_focus  text NOT NULL DEFAULT '',
  updated_at     timestamptz NOT NULL DEFAULT now()
);

-- 我的收获：学生自己的话。
CREATE TABLE reading_takeaway (
  atom_id    uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  text       text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- 理解检测（P2）。answerKey 只留服务端，不下发；判分在服务端做。
CREATE TABLE reading_check (
  atom_id      uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  questions    jsonb NOT NULL,
  answers      jsonb,
  score        integer,
  total        integer,
  generated_at timestamptz NOT NULL DEFAULT now(),
  answered_at  timestamptz
);
```

### 4.5 成本归属

`llm_call` 已有 `project_id`（可空，course/chat 调用即为 NULL）与 `surface` 判别列。轻量版的调用记 `project_id = NULL`、`surface = 'lite'`，并新增一列指回原子：

```sql
ALTER TABLE llm_call ADD COLUMN atom_id uuid REFERENCES atom(id) ON DELETE SET NULL;
CREATE INDEX llm_call_atom_created_idx ON llm_call (atom_id, created_at DESC);
```

`llm_usage` 视图按 `user_id` / tier / tokens / cost 聚合，**不读 project_id**，因此学校维度的成本汇总无需改动即可把轻量版算进去。

### 4.6 站点分流：edition 属于**学校**

**一所学校买的是轻量版还是现有版本，校内账号随之确定。** 学生注册时凭 join code 进入某个班级 → 班级属于某学校 → 该学校的 edition 决定这个账号进哪个站。因此**不在 `users` 上加列**。

```sql
ALTER TABLE schools ADD COLUMN edition text NOT NULL DEFAULT 'pro'
  CHECK (edition IN ('pro','lite'));
```

组织不变式不变：每个账号仍必属某学校 + ≥1 班级，注册仍需 join code。

**服务端判定：** session principal（`apps/api/internal/api/auth.go` 的 `api.User`）已经携带 `SchoolID`，鉴权闸据此取学校的 edition，不需要任何新的账号字段。

**`/auth/me` 不回传 edition。** 前端不需要知道它：将来每所学校使用各自的子域名，域名本身已经决定了进哪个站；走错站的请求由服务端一律 404（与归属失败同语义，不泄漏另一侧的存在）。

> 若将来需要「同校个别账号走另一边」，再加一个可空的 `users.edition` 覆盖层即可——本轮按 YAGNI 不做。

## 5. API

`{id}` 一律是 **atom id**。全部为轻量版自己的 handler，**不透传、不复用 pro 的 handler**。

```
GET    /api/v1/readings                     我的阅读列表
POST   /api/v1/readings                     {title?, lang} → 建 atom + reading（同一事务）
GET    /api/v1/readings/{id}                原子 + source 概要 + 进度
PATCH  /api/v1/readings/{id}                改标题
PUT    /api/v1/readings/{id}/source         贴正文 / 上传文件 / 贴链接
GET    /api/v1/readings/{id}/source         正文与分段（blocks）
GET    /api/v1/readings/{id}/brief
PUT    /api/v1/readings/{id}/brief
GET    /api/v1/readings/{id}/messages
POST   /api/v1/readings/{id}/turn           陪练一轮：装配 ReadingRouteInput → RouteReading
                                            → ApplyReadingGate；可能返回 summon 决策
GET    /api/v1/readings/{id}/annotations
POST   /api/v1/readings/{id}/annotations
POST   /api/v1/readings/{id}/cards/{cid}/activate
POST   /api/v1/readings/{id}/cards/{cid}/skip
POST   /api/v1/readings/{id}/cards/{cid}/submit
GET    /api/v1/readings/{id}/takeaway
PUT    /api/v1/readings/{id}/takeaway
POST   /api/v1/readings/{id}/finish

-- P2
GET    /api/v1/readings/{id}/check
POST   /api/v1/readings/{id}/check/generate
POST   /api/v1/readings/{id}/check/answer
GET    /api/v1/readings/{id}/report
POST   /api/v1/readings/{id}/report/generate
```

### 5.1 鉴权

- 原子端点仅本人可读写；归属失败一律 **404**，不泄漏存在性（沿用既有约定）。
- 所属学校 `edition = 'lite'` 的账号访问 `/projects/*` → 404；`edition = 'pro'` 的账号访问 `/readings|writings/*` → 404。语义与归属失败一致，不新增错误码。
- 消耗 token 的端点前照旧过 `HasEntitlement(ctx, user)`。

## 6. 两条主动线

### 6.1 阅读

```
新建 → 放入文本（粘贴 / 上传 / 链接）
     → 精读：透镜库、段落上悬挂工具卡、锚点选取、批注、AI 引导
     → 我的收获（takeaway，学生自己的话）
     → 理解检测（P2）
     → 简版报告（P2）
```

交互与 pro 阅读室**一致**（同样的透镜、工具卡、批注、一次只问一个的节奏），因为前端组件与 AI 大脑都是同一套。差别只在两处：**没有立题**，所以收尾不写「新线索 / 对立题的影响」，改为「我的收获 + 理解检测」；**没有证据图与探索**，所以相关面板不出现。

### 6.2 写作

写作与阅读同形：**一个框进去，一次专注的任务出来**。

#### 6.2.1 入口

落地页与阅读同一套骨架（见 §7.2b）：居中的招呼 + 一个框 + 推荐 + 右上角「我的写作」+ 提示条。

**框里直接写「你想写点什么」**——不是填表，不是先选体裁。学生把念头打进去（一句话就够），就进入写作页。

落地页同样提供：**推荐题目**（不知道写什么时）、**历史**（未完成在上、点击继续；已完成点击看报告）、**教师布置的写作任务**（P4 接线，位置先留）。

#### 6.2.2 进去之后：先聊，再写

进入写作页，**AI 先和学生讨论这个想法**——不是立刻给提纲，也不是立刻让写。一次只问一个（铁律③）。

然后 AI 把整件事拆成**四个阶段**，并把学生所在的位置显示出来：

```
构思 → 大纲 → 段落 → 成稿
```

**构思**：就这个题目**简单地聊**——试着把它和**证据、想法、或故事**连起来。这一阶段还必须敲定一件事：**这篇要写多长**（字数量级）。长度决定后面提纲的粒度与段落的数量，不先定下来，后面每一步都在猜。

**大纲**：由学生已经说出口的东西**派生**出提纲，学生可改。提纲生成是确定性的系统步骤，AGENTS.md 明确允许，**不是代写**。

**段落**：**引导式片段写作**——一段一段来，引导问题以 block 形式出现在旁边。**英文写作附示范段落**：明确标注为示范，与草稿在结构与视觉上分离，界面不提供任何一键插入，示范文本不写入任何草稿表（铁律①）。

**成稿**：把学生自己写的片段**拼成全文**——只拼接，不新造一个字。然后 AI 给整篇的反馈（结构、论证、清楚不清楚），仍然不代写。

最后：完成 → 简版报告（§8）。

#### 6.2.3 阶段是显示出来的，不是关卡

四个阶段是**给学生看的地图**，让他知道自己在哪、下一步是什么。**不做强制关卡**：学生想先写一段再回头补提纲，允许；想跳过构思直接写，允许——但**跳过被记录**（铁律④），并进入简版报告的确定性事实区。

> 与 pro 的 essay-track 的区别：pro 是研究论文的阶段机，带证据图与提案轨道；这里只有四步，没有立题、没有证据图、没有 essay-track 状态机。共享的是**底座**（`atom` / `atom_message` / `atom_card` / `atom_annotation`）与**AI 能力**，不是流程。

#### 6.2.4 数据

`writing` 与 `reading` 同为 `atom` 的一种形态（§4.2），专属表：

```sql
CREATE TABLE writing (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title       text NOT NULL DEFAULT '',
  lang        text NOT NULL CHECK (lang IN ('zh','en')),
  stage       text NOT NULL DEFAULT 'ideate'
              CHECK (stage IN ('ideate','outline','snippets','draft','finished')),
  target_words integer,                    -- 构思阶段敲定的篇幅，NULL 表示还没定
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);

CREATE TABLE writing_outline (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  text       text NOT NULL,
  depth      integer NOT NULL DEFAULT 0,
  position   integer NOT NULL
);
CREATE INDEX writing_outline_atom_idx ON writing_outline (atom_id, position);

CREATE TABLE writing_snippet (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id     uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  outline_id  uuid REFERENCES writing_outline(id) ON DELETE SET NULL,
  position    integer NOT NULL,
  text        text NOT NULL DEFAULT '',
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX writing_snippet_atom_idx ON writing_snippet (atom_id, position);

-- 成稿：由片段拼成，可再编辑。只有学生的字。
CREATE TABLE writing_draft (
  atom_id    uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  body       text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);
```

`atom_message` 承载「先聊」与各阶段的陪练对话；`atom_card` 承载写作用的工具卡；`atom_report` 承载简版报告——**四张共享表一张都不用新建**，这正是 §4.1 底座的回报。

#### 6.2.5 API（形状与阅读一致）

```
GET/POST  /api/v1/writings                  列表 / 新建（body: {idea, lang}）
GET/PATCH /api/v1/writings/{id}
POST      /api/v1/writings/{id}/turn        陪练一轮（复用 AI 层）
GET       /api/v1/writings/{id}/messages
POST      /api/v1/writings/{id}/stage       推进 / 回退阶段（记录，不设关卡）
PUT       /api/v1/writings/{id}/target-words
GET/PUT   /api/v1/writings/{id}/outline
GET/PUT   /api/v1/writings/{id}/snippets
POST      /api/v1/writings/{id}/compose     把片段拼成成稿（确定性，不调模型）
GET/PUT   /api/v1/writings/{id}/draft
POST      /api/v1/writings/{id}/review      AI 给整篇反馈
POST      /api/v1/writings/{id}/finish
GET/POST  /api/v1/writings/{id}/report[/generate]
```

`{id}` 一律是 atom id。鉴权与阅读同：归属失败 404，跨 edition 404。


## 7. 前端

### 7.1 `apps/lite-web`

独立目录、独立 `vite.config.ts` / `index.html` / `tailwind.config` / Dockerfile / nginx / 部署脚本，部署到独立域名。

依赖 `@mind-imprint/contracts` 与 `@mind-imprint/web`（`apps/web` 取包名并以 `exports` 暴露源码），**从源码引入房间组件与设计 token，不复制**。

两处必须在 P1 处理的构建陷阱：

- **Tailwind purge：** lite 的 `content` glob 必须包含 `../web/src/**`，否则房间样式被裁掉。
- **路径别名：** lite 的 vite / tsconfig 必须复刻 web 的 `@/` → `apps/web/src`，否则房间内部的 `@/…` 引入全部解析失败。
- 另注意 `mk-*` token 是**裸 CSS 变量**：所有 Tailwind alpha 语法（`bg-mk-x/NN`）**不产出任何 CSS**，须用 `linear-gradient` / `color-mix`，并在真实浏览器中验证。

### 7.2 Shell

`LiteApp.tsx`，**左侧边栏 + tab 切换，可自动折叠为纯图标**（与既有前端同形），URL 路由沿用既有「不引 router 库」的写法：

- **写作** `/writings` —— 落地是一个大对话框（AI App 形状）：学生把想法打进那一个框里，写作即开始；下方是历史。
- **阅读** `/readings` —— 落地是提交面（粘贴 / 上传文章）+ 历史列表。`/readings/:id` 进入阅读室。

无 图鉴、无 课程、无 项目。

### 7.2b 阅读落地页 —— 门面的形状（2026-08-26 用户看过真机后定）

落地页要像一个 **AI 产品**，不是一张表单：

1. **居中的招呼**：AI 问「Hi，今天要读点什么」，**「读」字带设计**（手绘书本 / 圈词 / 手写体）——整页的视觉锚点。
2. **粘贴框**：边框带**轻微辉光**（呼吸感）。可贴链接、贴整篇正文，**也可上传 DOCX / PDF**。
3. **不知道读什么？**：今日推荐。本期种子数据；将来接样本库与教师布置的任务。
4. **右上角入口 → 我的阅读**：**未完成在最上面**，点击继续；已完成在下方，点击看报告。历史**不进左栏**——左栏是「工作的种类」（阅读 / 写作），把子视图塞进去会打乱这套语义。
5. **提示条**：有未完成时提示「你有 N 篇还没读完」；教师任务的位置留出来（P4 接线）。

> 发现历史的路径有两条，所以入口可以收起来：提示条是**主动提醒**，右上角图标是**归档入口**。这也让落地页保持单一目的——开始读一篇。

**阅读旅程本身不变**（透镜、工具卡、批注、一次只问一个）。**将来**再加逐段的 AI 辅助：分析写法、叙事、关键词、结构，或**朗读**（`/voice/tts` 已存在，可复用）。

**不采用 Cowork 式「顶部切换 + 下方会话列表」。** 那类布局服务于**并行工作**——许多线程同时活着，列表即工作区；而读一篇文章、写一篇东西是**聚焦任务**，同一时刻只有一件事在手上，侧边栏只是导航。tab 形态 + 自动折叠成图标，既保持导航清晰，又把横向空间还给阅读室（正文、陪练、悬挂卡片三者都吃宽度）。

### 7.3 房间能力对象

房间组件目前散落着对 project 生命周期事实的直接读取，`demoMode` 已是这一模式的先例。把它泛化成一个对象，房间只读它：

```ts
export type RoomCapabilities = {
  mode: 'pro' | 'lite' | 'demo'
  plan, evidenceMap, explorationLeads, proposalImpact, essayTrack: boolean  // lite: false
  comprehensionCheck, exemplars: boolean                                    // lite: true
}
```

房间的数据出入口一律走注入的 `api` 对象——lite 传自己的客户端，pro 传现有的，组件本身不知道背后是哪套表。

## 8. 简版报告（P2/P3）

`packages/contracts` 新增 `AtomReport` 契约（v1）：`overview`（MODEL）+ `facts`（FACT，确定性）+ `strengths`（2 条）+ `nextTime`（1 条）。

每一项事实都来自已有记录，不新增采集：字数（`agent.CountWords`）、用了哪几张卡（`atom_card` 已提交行）、提问轮次（`atom_message` 中 role='student'）、理解检测得分（`reading_check`）。缺失的字段**省略而非填零**，不估算。

综述与建议 = **一次旗舰调用**（对比项目报告的四次）。不做等级、不做排名（铁律②）。前端渲染必须走 markdown renderer；契约对可空数组用 `.nullish().transform(v => v ?? [])`（既有教训：严格 `z.array` 遇到存量 JSON `null` 会让整份报告解析失败）。

## 9. 风险与验证

1. **信封漂移**：`atom_card` 与 pro 的 `card_instances` 是两张表，信封结构必须保持一致——它是过程树与评估的共同地基。防线：两侧共用 `packages/contracts` 的 Zod 契约，Go 侧只做边界校验，并有一条断言两侧信封同形的契约测试。
2. **`ReadingRouteInput` 装配失真**：AI 大脑复用得再干净，若 `Article` 分段、`FocusedSpans`、`RecentTurns` 装配得与 pro 语义不同，AI 表现就会退化。防线：装配逻辑单独成函数并单测，用与 pro 相同的分段规则。
3. **共享组件回归 pro**：lite 与 pro 共用房间组件。防线：能力对象 + 每个被触及的组件在 lite / pro 两套能力集下都要有测试。
4. **铁律① 漂移**：英文示范是唯一可能滑向代写的地方。防线：示范文本不入任何草稿表，界面无插入入口，两者都要有测试断言。
5. **测试时长**：本仓库每个 Go 集成测试都会启动一个独立 Postgres 容器，整包运行 10 分钟以上，**超过前台命令上限**。整包验证必须由 controller 统一在后台跑，实现者只跑定向 `-run`。

**测试面：** Go 单测覆盖原子 CRUD、装配函数、鉴权（本人 / 跨 edition）、信封边界校验、报告三态单飞；契约测试覆盖 `AtomReport` 与信封同形；前端组件在两套能力集下测试；e2e 走一条完整阅读（贴文 → 精读 → 收获）与一条完整写作。Go 测试统一 `-timeout 1800s` + `CGO_ENABLED=0`。

## 10. 分期

| 期 | 内容 |
|---|---|
| **P1 · 底座 + 阅读** | `atom` / `reading` / `atom_message` / `atom_card` / `atom_annotation` / `reading_source` / `reading_brief` / `reading_takeaway` 建表；`schools.edition` + 分流闸；`llm_call.atom_id`；阅读全部端点（含 `/turn` 装配并调用 `RouteReading`、透镜召唤、选句评价、锚点持久化）；`apps/lite-web` + 能力对象 + shell；**DOCX/PDF 上传**；**阅读落地页重设计**（招呼 + 辉光框 + 推荐 + 我的阅读面板 + 提示条）；端到端走查 |
| **P2 · 阅读收尾** | `reading_check` 理解检测（完成时出题验证是否真读懂）；`atom_report` + 简版报告（统计 + 阅读笔记 + 互动小结） |
| **P3 · 写作** | `writing` / `writing_outline` / `writing_snippet` / `writing_draft`；一个框进去 → AI 先聊想法 → 四阶段（构思 / 大纲 / 段落 / 成稿）；构思阶段敲定篇幅；英文示范；写作落地页（推荐题目 + 历史 + 未完成 + 教师任务位）；写作报告 |
| **P4 · 以后** | 教师布置阅读与写作任务（含截止日期与落地页提醒）；逐段 AI 辅助（写法 / 叙事 / 关键词 / 结构 / 朗读，复用 `/voice/tts`）；样本库；`chat` 原子；AI 项目原子 |

每一期独立 spec → plan → build。

## 11. 已定的决策

| 问题 | 结论 |
|---|---|
| 轻量版与 pro 的关系 | 同一后端服务、同一 Postgres、同一鉴权与组织；**前端独立站点，后端表独立** |
| 是否借用 `project` 行做存储容器 | **否**（2026-08-26 用户明确否决）。原子自持存储，`project.kind` 方案已回滚 |
| 复用什么 | AI 函数（阅读大脑本就与存储无关）、前端设计与设计 token、卡 spec 与信封契约、docextract、gateway 计量 |
| 为什么要 `atom` 底座 | AI 聊天与 AI 项目在路上；消息流 / 工具卡 / 批注 / 报告这四件事不该写四遍 |
| 是否迁移现有 project 到底座 | 本轮**不迁移**；将来若做，另开 spec |
| 工具卡是否保留 | 保留在房间内，与 pro 一致；轻量版左栏不设 图鉴 / 课程 tab |
| 过程评估 | 项目级 `EvaluationReport` 不用于轻量版；新设 `AtomReport` |
| 谁写正文 | 学生写每一个字；AI 给提纲建议与引导问题；英文写作提供**示范**（永不进入草稿） |
| edition 放在哪 | **学校**（`schools.edition`），注册时随 join code 的班级→学校定下；不加 `users` 列，不下发 `/auth/me` |

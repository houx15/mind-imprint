# 轻量版（Lite Edition）· 写作与阅读原子 — 设计 Spec

> 2026-08-26 · 状态：待实现。
>
> **这是程序级（program-level）设计**，覆盖 P1–P3 的整体形状。与仓库既有做法一致，**每一期各自再出 spec → plan → build**；紧接其后的实现计划只覆盖 §9 的 P1。

## 1. 背景与目标

现有产品服务 IB / AP Seminar 学生，围绕一条重量级的**项目生命周期**（立题 → 管理 → 阅读 → 写作 → 回顾）展开。新客户群是**普通学校**：他们不做研究项目，也不需要证据图、提案轨道、essay-track 阶段机。他们需要的是**单次的写作练习**和**单次的阅读练习**。

轻量版因此是一个**独立的前端站点**（独立域名、独立 shell、独立构建），账号按 edition 路由到轻量版或现有版本。

### 目标

- 独立前端站点，左栏只有两项：**写作**、**阅读**。
- **阅读室与写作室复用现有实现**——同样的透镜、同样的工具卡、同样的 AI 引导、同样的批注层。不复制、不分叉。
- 阅读新增**理解检测**：读完后 AI 出几道题，验证学生是否读懂。
- 新的**简版报告**，取代项目级的 `EvaluationReport`。
- 写作支持中英双语；**英文写作提供示范段落**。
- 账号按 `edition` 分流到轻量版 / 现有版本。

### 非目标（本轮不做）

- 教师布置阅读任务、课堂内小聊天、小型 PBL 项目 —— **为它们预留模型形状，但不实现**。
- 轻量版不设 图鉴 / 课程 / 项目 三个 tab。
- 不改动现有版本（pro）的任何行为。
- AI 不代写正文（见 §2）。

## 2. 守住的设计铁律

| 铁律 | 在轻量版中的落法 |
|---|---|
| ① AI 绝不代写正文 | 提纲生成与「合成全文」都是**确定性系统步骤**（AGENTS.md 明确允许）：合成只拼接学生自己写的片段，不新造一个字。英文示范段落**明确标注为示范**，与草稿在结构与视觉上分离，永不自动插入。 |
| ② 不操纵 | 无连胜、无排行榜、无徽章、无推送。工具卡自动触发但由学生确认打开，与现状一致。 |
| ③ 一次只问一个 | 陪练对话与引导问题 block 均沿用现有节奏。 |
| ④ 过程即数据 | 跳过工具卡、让 AI 直接答，同样被记录，并进入简版报告的确定性事实区。 |

## 3. 领域模型

### 3.1 原子：`writing` 与 `reading` 是兄弟表

不使用 `kind` 判别列，而是**每类原子一张表**——因为 `chat` 与 `project` 之后会作为兄弟加入，判别列会持续膨胀。

```sql
CREATE TABLE writing (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title        text NOT NULL DEFAULT '',
  lang         text NOT NULL CHECK (lang IN ('zh','en')),
  status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
  container_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz
);
CREATE INDEX writing_user_created_idx ON writing (user_id, created_at DESC);
CREATE UNIQUE INDEX writing_container_idx ON writing (container_id);

CREATE TABLE reading (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title        text NOT NULL DEFAULT '',
  lang         text NOT NULL CHECK (lang IN ('zh','en')),
  status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
  container_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
  reference_id uuid REFERENCES reference(id) ON DELETE SET NULL,  -- 容器内那一条 reference（阅读室按 project+reference 寻址）
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz
);
CREATE INDEX reading_user_created_idx ON reading (user_id, created_at DESC);
CREATE UNIQUE INDEX reading_container_idx ON reading (container_id);
```

### 3.2 隐藏容器：`project.kind`

阅读室与写作室依赖的每一张子表（`reference` / `material` / `card_instances` / `outline_node` / `snippet` / `draft_snapshot`，以及批注、锚点、陪练轮次所挂载的行）都外键到 `project`。因此**每个原子静默持有一行 `project` 作为存储锚点**，前端永不展示。

```sql
ALTER TABLE project ADD COLUMN kind text NOT NULL DEFAULT 'project'
  CHECK (kind IN ('project','container'));
CREATE INDEX project_kind_idx ON project (kind);
```

**代价：** `project` 这个表名变得略微名不副实（它其实是「工作区容器」）。
**收益：** 陪练循环、工具卡循环、批注层一行都不用分叉——这正是「复用而非复制」的要求。

> 已评估并否决的替代方案：给全部子表加 `owner_kind/owner_id`。收益仅是命名纯净，代价是十余张表的迁移 + 每个 handler 都要接受两种 owner。

**⚠️ 这是本设计最高风险项。** 见 §8.1 的强制审计清单：任何读 `project` 的既有查询若不加 `kind = 'project'` 过滤，容器行就会泄漏进项目列表、教师看板与成本汇总。

### 3.3 理解检测

```sql
CREATE TABLE reading_check (
  reading_id   uuid PRIMARY KEY REFERENCES reading(id) ON DELETE CASCADE,
  questions    jsonb NOT NULL,          -- [{id, prompt, kind:'mcq'|'short', options?, answerKey?}]
  answers      jsonb,                   -- [{id, response, correct?, feedback?}]
  score        int,
  total        int,
  generated_at timestamptz NOT NULL DEFAULT now(),
  answered_at  timestamptz
);
```

`answerKey` 只在服务端保留，**不下发**给客户端（判分在服务端做）。

### 3.4 简版报告

一张表服务两类原子，用「恰好一个非空」约束换取引用完整性 + 未来可扩展：

```sql
CREATE TABLE atom_report (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  writing_id   uuid REFERENCES writing(id) ON DELETE CASCADE,
  reading_id   uuid REFERENCES reading(id) ON DELETE CASCADE,
  status       text NOT NULL CHECK (status IN ('pending','ready','failed')),
  payload      jsonb,
  error        text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  generated_at timestamptz,
  CONSTRAINT atom_report_one_owner CHECK (num_nonnulls(writing_id, reading_id) = 1)
);
CREATE UNIQUE INDEX atom_report_writing_idx ON atom_report (writing_id) WHERE writing_id IS NOT NULL;
CREATE UNIQUE INDEX atom_report_reading_idx ON atom_report (reading_id) WHERE reading_id IS NOT NULL;
```

生成沿用现有 `evaluation` 的**原子 `ON CONFLICT` 单飞 + 三态 + 轮询**生命周期（见 `retire-old-evaluation-pipeline` 的既有实现），不重新发明。

### 3.5 站点分流：edition 属于**学校**，不属于账号

**一所学校买的是轻量版还是现有版本，校内账号随之确定。** 学生注册时凭 join code 进入某个班级 → 班级属于某学校 → 该学校的 edition 就决定了这个账号进哪个站。因此**不在 `users` 上加列**，edition 在注册那一刻就由所属学校定下，不是一个可以随账号漂移的属性。

```sql
ALTER TABLE schools ADD COLUMN edition text NOT NULL DEFAULT 'pro'
  CHECK (edition IN ('pro','lite'));
```

组织不变式不变：每个账号仍必属某学校 + ≥1 班级，注册仍需 join code。

**服务端判定：** session principal（`apps/api/internal/api/auth.go` 的 `api.User`）已经携带 `SchoolID`，鉴权闸据此取学校的 edition，不需要任何新的账号字段。

**`/auth/me` 不回传 edition。** 前端不需要知道它：将来每所学校使用各自的子域名，域名本身已经决定了进哪个站；走错站的请求由服务端一律 404（与归属失败同语义，不泄漏另一侧的存在）。

> 若将来确实需要「同校个别账号走另一边」，再加一个可空的 `users.edition` 作为覆盖层即可，届时不影响本设计——本轮按 YAGNI 不做。

## 4. API

### 4.1 原子形状的新端点

```
GET    /api/v1/writings                     我的写作列表
POST   /api/v1/writings                     {title?, lang} → 建原子 + 建容器（同一事务）
GET    /api/v1/writings/{id}
PATCH  /api/v1/writings/{id}                改标题
POST   /api/v1/writings/{id}/finish
GET    /api/v1/writings/{id}/report
POST   /api/v1/writings/{id}/report/generate

GET    /api/v1/readings                     我的阅读列表
POST   /api/v1/readings                     {title?, lang} → 建原子 + 建容器 + 建 reference
GET    /api/v1/readings/{id}
PATCH  /api/v1/readings/{id}
POST   /api/v1/readings/{id}/text           粘贴正文 / 上传文件 / 贴链接
POST   /api/v1/readings/{id}/finish
GET    /api/v1/readings/{id}/check
POST   /api/v1/readings/{id}/check/generate
POST   /api/v1/readings/{id}/check/answer
GET    /api/v1/readings/{id}/report
POST   /api/v1/readings/{id}/report/generate
```

`POST /readings/{id}/text` 是既有 `paste-content` / `ingest-file` 的薄封装，复用 `internal/docextract` 的真实 PDF/DOCX 解析。

### 4.2 房间端点透传 —— 唯一的接缝

**不重新声明任何 handler。** `apps/api/internal/api/projects.go:220` 的 `loadOwnedProjectRow` 是全部 107 个调用点（其注释自述覆盖 ~140 个 mutating handler）唯一把 `r.PathValue("id")` 变成 project id 的地方，`is_demo` 只读约束也挂在这里。

做法：

```
/api/v1/writings/{id}/room/…   → 既有 /projects/{pid}/… handler
/api/v1/readings/{id}/room/…   → 同上
```

1. 新增中间件 `withAtomContainer(kind)`：解析 `{id}` → 载入原子 → 校验 `user_id` 归属 → 把 `container_id` 放进 request context。
2. `loadOwnedProjectRow` 增加约 15 行：**若 context 中带有容器 id，用它取代 `PathValue("id")`**；其余存在性 / 归属 / demo 语义原样保留。
3. 房间路由在 atom 前缀下**用同一个 handler 函数再注册一次**，一张路由表，零逻辑复制。

单个函数改动，零 handler 编辑，pro 路由完全不动。

### 4.3 鉴权

- 原子端点：仅本人可读写（沿用「归属失败一律 404、不泄漏存在性」的既有约定）。
- 所属学校 `edition = 'lite'` 的账号访问 `/projects/*` → 404；所属学校 `edition = 'pro'` 的账号访问 `/writings|readings/*` → 404。语义与归属失败一致，不新增错误码。
- 容器 project 永远不可经 `/projects/{id}` 直接访问：`loadOwnedProjectRow` 在**没有**容器 context 时，遇到 `kind = 'container'` 的行直接 404。
- 消耗 token 的端点前照旧过 `HasEntitlement(ctx, user)`。

## 5. 两条主动线

### 5.1 阅读原子

```
新建 → 放入文本（粘贴 / 上传 / 链接）
     → 精读：透镜库、段落上悬挂工具卡、锚点选取、批注、AI 引导  ← 与 pro 完全一致
     → 我的收获（takeaway，学生自己的话）
     → 理解检测（新）
     → 简版报告
```

**唯一与 pro 不同之处：收尾。** pro 的收尾写「新线索」与「对立题的影响」并回灌证据图；轻量版没有立题，因此 `FinalizeReadingPanel` 需要一个 lite 变体——它同时也是理解检测的落点。收尾之前的一切原封不动。

### 5.2 写作原子

```
新建 → 大对话框：说出你的想法
     → AI 生成提纲（学生可改；确定性系统步骤，非代写）
     → 分段写片段：引导问题以 block 形式出现；英文写作附示范段落
     → 合成全文（只拼学生自己的片段）
     → AI 反馈
     → 简版报告
```

**英文示范段落的约束（铁律① 的执行细则）：** 示范渲染在与草稿分离的容器中，带明确的「示范」标识；界面不提供任何一键插入 / 复制到草稿的入口；示范文本不写入 `snippet` / `draft` 任何一张表。

## 6. 前端

### 6.1 `apps/lite-web`

独立目录、独立 `vite.config.ts` / `index.html` / `tailwind.config` / Dockerfile / nginx / 部署脚本（对照 `.deploy-local/deploy-site.sh` 的既有形状），部署到独立域名。

依赖：
- `@mind-imprint/contracts`
- `@mind-imprint/web` —— `apps/web` 取得包名并以 `"exports": { "./src/*": "./src/*" }` 暴露源码；lite 直接从源码引入房间组件，不复制。

两处已知构建陷阱，必须在 P1 就处理：
- **Tailwind purge：** lite 的 `content` glob 必须包含 `../web/src/**`，否则房间样式被裁掉。
- **路径别名：** lite 的 vite/tsconfig 必须复刻 web 的 `@/` → `apps/web/src`，否则房间内部的 `@/…` 引入解析失败。
- 另注意 `mk-*` token 是裸 CSS 变量，**所有 Tailwind alpha 语法（`bg-mk-x/NN`）不产出任何 CSS**，须用 `linear-gradient` / `color-mix`，并在真实浏览器中验证。

### 6.2 Shell

`LiteApp.tsx`，左栏两项，URL 路由沿用现有「不引 router 库」的写法：

- **写作** `/writings` —— 落地是一个大对话框（AI App 形状）：学生把想法打进那一个框里，写作即开始；旁边是历史切换。`/writings/:id` 进入写作室。
- **阅读** `/readings` —— 落地是提交面（粘贴 / 上传文章）+ 历史列表。`/readings/:id` 进入阅读室。

无 图鉴、无 课程、无 项目。

### 6.3 房间能力对象

房间目前散落着对项目生命周期事实的直接读取，`demoMode` 已经是这一模式的先例（`0081_project_is_demo` → `WorkspaceContainer` / `ReadingRoom` / `WritingBlock` / `ReadingBlock` / `ReviewBlock` / `StudioChat`）。把它泛化为一个对象：

```ts
export type RoomCapabilities = {
  mode: 'pro' | 'lite' | 'demo'
  plan: boolean               // lite: false
  evidenceMap: boolean        // lite: false
  explorationLeads: boolean   // lite: false
  proposalImpact: boolean     // lite: false
  essayTrack: boolean         // lite: false
  comprehensionCheck: boolean // lite: true（阅读）
  exemplars: boolean          // lite: true（英文写作）
}
```

`demoMode: boolean` 收敛进 `mode`。房间只读这一个对象，不再自己去问项目处在哪个阶段。

## 7. 简版报告

`packages/contracts` 新增 `AtomReport` 契约（v1）：

```ts
{
  version: 1,
  overview: string,                    // MODEL · 一到两句综述
  facts: {                             // FACT · 确定性，无模型参与
    words?: number,
    revisions?: number,
    cardsUsed: string[],
    questionsAsked?: number,
    readingMinutes?: number,
    checkScore?: { correct: number; total: number },
  },
  strengths: [{ title: string; evidence: string }],   // MODEL · 2 条，须落在 facts 上
  nextTime: { title: string; why: string },           // MODEL · 1 条
}
```

每一项事实的来源必须是已存在的记录，不新增采集：

| 字段 | 来源 | 适用 |
|---|---|---|
| `words` | `agent.CountWords`（注意前端两处计数器须与之一致——既有教训：`countWords` 曾按字符计数，英文偏差约 5×） | 写作 |
| `revisions` | `revision_checkpoint` 行数 | 写作 |
| `cardsUsed` | `card_instances` 中已提交的卡 | 两者 |
| `questionsAsked` | 陪练轮次中学生发起的轮数 | 两者 |
| `readingMinutes` | 阅读室会话活跃时长；**若 P1 未采集则整字段省略**，不估算、不补写 | 阅读 |
| `checkScore` | `reading_check.score / total` | 阅读 |

- **`facts` 全部确定性算出**，模型无从幻觉；缺失的字段一律省略而非填零。
- 综述 / 两点做得好 / 一点下次可以试试 = **一次旗舰调用**（对比项目报告的四次），成本约为其五分之一。
- 沿用既有 `ValidateRefs` 思路校验引用。
- **不做等级、不做分数排名**（铁律②）；理解检测的得分是学生自己的答题结果，据实呈现。
- 前端渲染必须走 markdown renderer（既有教训：新学生可见文本绕过 renderer 会漏出字面 `**bold**`）。
- 契约用 `.nullish().transform(v => v ?? [])` 处理可空数组（既有教训：严格 `z.array` 遇到存量 JSON `null` 会整份报告解析失败）。

## 8. 风险与验证

### 8.1 容器泄漏（最高风险）

`project` 表的每一个既有读取点都必须加 `kind = 'project'` 过滤。强制审计范围：

- `apps/api/internal/store/queries/project.sql`
- `apps/api/internal/store/queries/teacher.sql`
- `apps/api/internal/store/queries/org.sql`
- 以及三者生成的 `sqlc/*.sql.go`

漏掉任何一处，容器行会污染项目列表、教师看板、班级周报与组织成本汇总。**实现计划中这一项单独成一个 task，并配一条断言「容器行不出现在任一列表/聚合」的集成测试。**

### 8.2 共享组件回归 pro

lite 与 pro 共用房间组件，lite 的改动可能打坏 pro。防线：能力对象 + **每个被触及的房间组件在两套能力集下都要有测试**。

### 8.3 铁律① 漂移

英文示范功能是唯一可能滑向代写的地方。防线：示范文本不入任何草稿表，且界面无插入入口——两者都要有测试断言。

### 8.4 测试

- **Go 单测：** 原子 CRUD；原子 + 容器同事务创建（失败则整体回滚）；`withAtomContainer` 解析与归属；跨 edition 鉴权；容器不可经 `/projects/{id}` 直达；报告生成的三态与单飞。
- **契约测试：** `AtomReport` 的 round-trip 与可空字段容错。
- **Web 单测：** 被触及的房间组件在 lite / pro 两套能力集下渲染与交互。
- **e2e：** 轻量站上一条完整阅读走查（提交文章 → 精读 → 收获 → 理解检测 → 报告）与一条完整写作走查（想法 → 提纲 → 片段 → 合成 → 反馈 → 报告）。
- Go 测试照旧 `-timeout 1800s` + `CGO_ENABLED=0`；卡 spec 改动会重排 AI system-prompt golden，须跑 `internal/agent`。

## 9. 分期

| 期 | 内容 |
|---|---|
| **P1 · 地基** | 迁移（`reading` / `project.kind` / `schools.edition`）；原子 CRUD + `withAtomContainer` 透传；容器泄漏审计与测试；`apps/lite-web` 脚手架 + 构建/部署；能力对象；**阅读原子跑通至「我的收获」**（复用阅读室；收尾此期仍走 pro 面板，P2 再换 lite 变体） |
| **P2 · 阅读收尾** | `FinalizeReadingPanel` 的 lite 变体；理解检测（出题 / 答题 / 判分）；阅读简版报告 |
| **P3 · 写作** | 写作落地大对话框 + 历史；提纲生成、片段写作、合成全文、AI 反馈；英文示范段落；写作简版报告 |
| **P4 · 以后** | 教师布置阅读任务；`chat` 原子；小型 PBL `project` 原子 |

每一期独立 spec → plan → build。

## 10. 已解决的开放问题

| 问题 | 结论 |
|---|---|
| 轻量版与 pro 的关系 | 同一后端、同一 Postgres、同一鉴权与组织；前端必须独立站点 |
| 是否叫 practice | 否——`writing` / `reading` 各自成表，为未来的 `chat` / `project` 留出兄弟位 |
| 谁写正文 | 学生写每一个字；AI 给提纲建议与引导问题 block；英文写作提供示范 |
| 工具卡是否保留 | **保留在房间内**，与 pro 完全一致；只是轻量版左栏不设 图鉴 / 课程 tab |
| 过程评估 | 项目级 `EvaluationReport` 不用于轻量版；新设 `AtomReport` 简版报告 |
| 阅读室与 pro 的差距 | 收尾之前完全一致；收尾因无立题而需 lite 变体 |
| 隐藏容器 | 采用——以 `project` 表名的轻微失真，换取房间逻辑零分叉 |
| edition 放在哪 | **学校**（`schools.edition`），注册时随 join code 的班级→学校定下。不加 `users` 列，不下发 `/auth/me`；将来每校各自的子域名本身即已分流 |

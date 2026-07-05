# Per Aspera 官网 · 设计规格（Design Spec）

> 日期：2026-07-05 · 状态：已定稿，待实施
> 一个**独立**的营销站，为 Per Aspera（一所面向问题解决者的思维学院 + 研究院）而建。
> 与 `apps/web` / `apps/api` / `apps/site`（思维印记产品站）完全解耦；思维印记作为 Per Aspera 研究院的旗舰项目在 `/institute` 页展示。

## 1. 目标与范围

- 建一个对标 [astranova.org](https://www.astranova.org/#high-school) 视觉风格的、**中英全站双语**、移动端优先、**美观**的营销站。
- 完整 v2 五页结构 + 两篇研究长文页。
- 真实表单提交（留资）落 Supabase，部署到 Vercel。

**权威文案来源：** `docs/astranova/PerAspera官网文案v2-多页版.md`（v2 五页版为准）+ `PerAspera官网文案v1.md`（补充：创始人信全文、完整 FAQ 8 条）。研究长文数据源：同目录 `ad-astra-调研报告.html`、`astra-nova高中申请与真实案例.html`。

**明确不做：** 不复制 Astra Nova 的申请流程。我们**不**要求家长/学生提交思辨视频或家长信——那是 Astra 的流程，不是我们的。

## 2. 关键决策（已与用户确认）

| 项 | 决策 |
|---|---|
| 位置 | `apps/peraspera`（独立 Astro app，纳入 pnpm workspace） |
| 页面 | `/` · `/programs` · `/institute` · `/about` · `/apply` + `/institute/ad-astra` + `/institute/schools` |
| 双语 | 全站 zh（`/`）+ en（`/en/`），镜像 `apps/site` 的 i18n（`t(lang,zh,en)`） |
| 视觉 | 编辑式极简，暖白纸面 + 近黑墨色 + **深墨蓝**强调色 |
| 字体 | 强力 grotesk 标题 + 干净 sans 正文；中文 Noto Sans SC |
| 提交 | **真实**——Astro `/api/*` serverless → Supabase `leads` 表，service key 仅服务端 |
| 部署 | **Vercel**（`@astrojs/vercel`，hybrid：静态页 + 按需 API） |
| 环境 | `.env.vercel` 在仓库根（已 gitignore ✓）；本地 `apps/peraspera/.env`（gitignore）；Vercel env 从该文件灌入 |

## 3. 申请漏斗（核心改动）

Astra 的申请 = 视频 + 家长信。**我们的漏斗 = 留资 → 说明会 → 一对一面谈。**

- `/apply` 页 = **留资 / 预约说明会**页。表单字段：
  - 家长/孩子称呼（`contact_name`）
  - 联系方式（`contact`，微信/手机/邮箱，自由文本）
  - 孩子年龄（`child_age`，可选）
  - 意向（`interest`：冲刺营 / 长线学院 / 先了解一下）
  - 想问的问题（`message`，可选）
- 提交 → `POST /api/lead` → Supabase `leads` 表 → 我们跟进安排说明会 + 面谈。
- 首页底部 CTA、导航按钮相应改写为「预约说明会」，删除「2 分钟思辨视频 + 一页家长信」的旧文案。

## 4. 信息架构（页面 → 区块）

导航（全站固定）：课程 `/programs` · 研究院 `/institute` · 关于我们 `/about` · 语言切换 · 【按钮】预约说明会 `/apply`
页脚（全站固定）：非关联声明（中英）+ 联系方式 + 社媒位 + 版权。

### `/` 首页
1. Hero：大标语 + 双 CTA + **倒计时条**（距 2026-10-15）
2. 三支柱概览（课程 · 研究院 · 理念，三张大卡）
3. 我们相信什么（4 条信念卡，中英对照）
4. 精选 FAQ（手风琴，3 条）
5. 底部 CTA（预约说明会）

### `/programs` 课程
1. 页首标语
2. 冲刺营 `#sprint`：立场声明（pull-quote）+ 倒计时 + 双轨卡片 + 横向时间线 + 透明收费卡 + 筛选声明
3. 长线学院 `#academy`：为什么存在 + 四条课程线卡 + 三档展开卡 + 本学期课程单（静态表）

### `/institute` 研究院
1. 页首标语
2. 思维印记（旗舰）：一句话 + 四步工作原理 + 设计红线三卡 + Demo/预约演示 CTA + 它从哪来
   - 内容须与思维印记真实模型一致：两面（生成式驾驭 / 批判式防护）× 十维 × SOLO L1–L4 × 主动性梯度；评估私密、只给学生本人、始终用最强模型、安静提供不推送。
3. 公开研究：导语 + 研究 1（Ad Astra 完全解读 → `/institute/ad-astra`）+ 研究 2（全球同类学校 → `/institute/schools`）+ 研究 3（我们的立场，短文内联）

### `/about` 关于我们
1. 页首标语
2. 两位创始人卡（陈玉洁 / 侯煜欣）
3. 创始人信（长文，取自 v2 末段 + v1 全文）
4. 名字的由来
5. 完整 FAQ（手风琴，8 条，取自 v1 第 9 节）

### `/apply` 预约说明会
1. 标语 + 漏斗说明（留资 → 说明会 → 面谈）
2. 留资表单（见 §3）
3. 提交成功态

### `/institute/ad-astra` · `/institute/schools`（Phase D）
两篇研究长文，从同目录 HTML 报告**重排 + 翻译**，保留全部来源链接（可信度核心）。含交互时间线 / 对比表。

## 5. 技术架构

### 5.1 应用形态
- Astro 5 静态站 + `@astrojs/vercel` 适配器（hybrid）。营销页 `prerender = true`（默认全静态）；仅 `src/pages/api/*` 设 `export const prerender = false` 走 Vercel serverless。
- 纳入 `pnpm-workspace.yaml`。`apps/peraspera/package.json` name = `peraspera`。

### 5.2 双语（镜像 apps/site）
- `src/i18n/utils.ts`：`locales`、`getLang`、`t(lang, zh, en)`、`localePath`、`switchLocaleUrl`。zh 为真相源，en 镜像。
- 路由：canonical 页在 `src/pages/*.astro`（zh）；英文在 `src/pages/en/*.astro`，各自 `import` 同一个页面组件并传 `lang`。或用 Astro i18n routing——采用与 apps/site 相同的「en 子目录 + 复用组件」做法以保持一致。
- `astro.config.mjs` 配 `i18n`（locales `["zh","en"]`, defaultLocale `zh`, `prefixDefaultLocale: false`），`site`，vercel adapter。

### 5.3 内容即数据（单一真相源，翻译锁步）
- `src/content/<page>.ts` 每页一个 TS 模块，导出结构化对象，**每个字符串是 `{ zh, en }` 对**（或用 `t()` 在组件里就地取）。避免维护两份平行 HTML、避免 zh/en 漂移。
- 长文案（创始人信、FAQ、研究长文）也进 content 模块，组件只负责渲染。

### 5.4 组件
共享 UI（`src/components/`）：
- `Base.astro`（layout，head/hreflang/字体/reveal 脚本）、`Nav.astro`、`Footer.astro`、`Icon.astro` + `IconSprite.astro`
- `Countdown.astro`（内联 client script，目标 2026-10-15；截止后切文案）
- `Accordion.astro`（FAQ，纯 CSS/JS 渐进增强）
- `Timeline.astro`（横向时间线）
- `TierCards.astro`（三档展开卡）
- `BeliefCard`、`PullQuote`（立场声明大字）、`Section`、`StatTable`（收费/学费/对比表）
- `LeadForm.astro`（留资表单 + client fetch 到 `/api/lead`）
页面区块组件按需拆到 `src/components/sections/` 与 `src/components/pages/`（同 apps/site 结构）。

### 5.5 后端（真实提交）
- `src/pages/api/lead.ts`：`prerender=false`，`POST` JSON → 校验字段 → 用 `SUPABASE_SERVICE_KEY` 服务端插入 `leads` → 返回 `{ ok: true }`。做基本反滥用（必填校验、长度上限、honeypot 字段）。
- Supabase 表 `leads`：`id uuid pk default gen_random_uuid()`, `created_at timestamptz default now()`, `contact_name text`, `contact text not null`, `child_age text`, `interest text`, `message text`, `locale text`, `source text`, `user_agent text`。RLS 开启且**无** public insert 策略——写入只走 service key（绕过 RLS）。迁移 SQL 放 `apps/peraspera/supabase/migrations/`。
- **密钥只在服务端**：service key / DATABASE_URL 绝不进客户端 bundle、日志、错误信息。客户端只 POST 表单 JSON。

### 5.6 设计系统（新）
- 调色板：`--paper:#FAF8F4`（暖白）、`--ink:#14161A`（近黑）、`--primary` 深墨蓝（约 `#1E3A5F` / `#22406E`，实施时定标）、少量中性灰、`--line` 描边。强调色克制，仅用于 CTA、link、倒计时、强调字。
- 字体：标题强力 grotesk（如 Space Grotesk / Archivo / Sohne 替代——用可 self-host 或 Google Fonts 的开源 grotesk）；正文干净 sans；中文 Noto Sans SC。经 `<link>` 或 self-host，避免 CSP 问题（本站无 CSP，Google Fonts 可用，同 apps/site）。
- 版式：大量留白、大标题（clamp 响应式）、清晰分区节奏、strategic italic/大写强调。
- 动效：reveal-on-scroll（IntersectionObserver，`html.js` 渐进增强，`prefers-reduced-motion` 关闭），克制。
- `src/styles/global.css` 一份设计系统（tokens + 组件类），移动端优先，与 apps/site 的组织方式一致但**全新视觉**。

## 6. 文案铁律

- **禁止「不是…而是」/「not X but Y」反义对举句式**，一律改写为正面陈述句（见团队反馈；apps/site 已做过同样清洗）。
- mockup 用真实内容，禁 lorem ipsum。
- 双语范围：**全站全译**。zh 为真相源，en 为高质量意译（非直译腔），语气自然。
- 页脚非关联声明中英双语，逐字采用 v2 §6 文案。

## 7. 部署（Vercel）

- `@astrojs/vercel` adapter，`output` 采用默认静态 + API route 按需。
- Vercel 项目 root = `apps/peraspera`（monorepo，设 Root Directory）。build = `pnpm build`（astro build）。
- 环境变量：从 `.env.vercel` 灌入 Vercel（Supabase URL/anon/service、DATABASE_URL）。`VERCEL_TOKEN` 仅用于 CLI 部署，不设为项目 env。
- 本地：`apps/peraspera/.env`（gitignore）供 `astro dev` 用。
- 倒计时、表单在预览部署上验证。

## 8. 分期实施

每期独立可 build / 可部署：

- **Phase A** — scaffold app + workspace 接线 + i18n + 设计系统（tokens/字体）+ `Base`/`Nav`/`Footer`/`Countdown` + **首页**（zh+en）。终态：首页双语可跑、倒计时动、视觉定调。
- **Phase B** — **课程页** + **关于页**（含创始人信、完整 FAQ、时间线、双轨/三档/收费卡）。
- **Phase C** — **研究院页** + **预约说明会页** + `/api/lead` + Supabase `leads` 表 + 迁移。终态：留资端到端可提交。
- **Phase D** — 两篇**研究长文页**（从两份 HTML 报告重排 + 翻译，保留来源链接）。
- **部署** — 接 Vercel，预览验证，环境变量灌入。

## 9. 验证 / 测试

- 每期后：`pnpm --filter peraspera build` + `astro check` 通过、无 TS/构建错误。
- E2E：本地 `astro dev` 起站，逐页逐语言点检——导航、语言切换往返、倒计时、手风琴、时间线、三档展开、表单提交（`/api/lead` 落 Supabase 一条测试记录并清理）。
- 文案自查：无「不是…而是」；zh/en 齐全无占位；来源链接可点。
- 移动端断点检查（≤760px）。
- 部署后在 Vercel 预览上复跑表单提交与倒计时。

## 10. 风险 / 注意

- `NEXT_PUBLIC_` 前缀是 Next 约定；Astro/Vite 默认只暴露 `PUBLIC_` 前缀给客户端。**保持 Supabase 全服务端**即可回避——客户端不需要任何 Supabase 变量。API route 用 `process.env` / `import.meta.env` 读取（Vercel 运行时注入）。
- 视频/大文件：本期不做上传（漏斗已改为留资）。
- 研究长文体量大（翻译 + 重排），故置于 Phase D 单列。
- Supabase 项目须已存在且 `leads` 表迁移已应用（用 `.env.vercel` 里的连接）。若线上不可达，API route 需返回友好错误且不泄露密钥。

# 工具卡图鉴 + 熟练度系统 · 设计

> 权威骨架见 `AGENTS.md`（四条铁律）。本文是「工具卡」从「已收集清单」升级为「完整图鉴 + 熟练度」的设计。
> 决定人：用户（houyuxin）授权自主推进，user-first；本设计据此定稿并实施。

## 目标（用户原话拆解）

1. **卡片有元数据**：每张卡适用于哪些阶段，应在卡的详情里显示；卡有自己的元数据；学生对每张卡有**学习历史**与**熟练度**。
2. **成长报告 → 工具卡** 变成一个**图鉴**：
   - 展示**全部**卡片，包括**没遇到过**的。
   - **已学**的卡 = **彩色**封面；**未遇到**的 = **灰度**。
   - 已学卡默认显示 ⭐ 表示熟练度；**hover** 显示半透明描述；**click** 打开详情（modal）。
   - 已学详情：卡的细节 + 一个**示例** + 一个**跳到相关课程**的链接 + 学生的**使用历史/心得**。
   - 未遇到详情：只有卡细节、示例、跳课程链接（**无历史**）。
3. **4 种配色**，学生可自选（per-user）。
4. **所有素材经 OSS API 上传**，不进数据库（`docs/2026-07-27-oss-storage-developer-guide.md`）。
5. **熟练度分数 + 算法**：结合「课程学习」与「真实使用」；用得越好分越高；与直觉一致；可后续迭代。未来卡的交互可依熟练度不同。
6. 素材出现的其他位置：课程若有自己的卡集，**课程结课报告**可出现卡封面（good-to-have）；**项目侧栏不出现**；弹出卡详情的半透明封面头（good-to-have，非必须）。

## 现状（调研结论）

- **卡注册表**：`packages/contracts/cards/*.json`（45 张）→ `CARD_REGISTRY`（`src/registry.ts`）；Go 侧 `internal/cards/specs/`（45 张，`sync-cards` 镜像，`go:embed`）。卡已带 `category`/`stage`/`subject`/`name_en`/每步 `methodology.example`。
- **封面素材**：`docs/reference/cards/colorways/{light,cyber-sage,cyber-slate,cyber-warm}/`，命名 `T00..T38`。`cyber-*` 每个 T 恰好 1 张；`light` 有 `女生版/男生版/中性版/vN` 变体（取一张为准）。
- **素材↔注册表非满射**：需人工判断的映射 + 一份「未匹配」报告（本轮产出 `docs/2026-08-01-card-asset-coverage.md`）。原则：**内容一致但命名不同 → 以素材名为准**；无法匹配 → 留空并入报告；素材没有对应卡 → 也入报告。
- **熟练度原料已存在**：
  - `card_instances`（`status ∈ proposed/active/completed/skipped`；surface 由 project/course/chat 的父连接推导；`created_at`）。
  - `ListCollectedCardsByUser`：每卡 uses/surfaces/lastUsed（仅 completed）。
  - `CountCompletedCardUsesByUser`：**项目域**真实完成计数（已按卡的 completion 谓词门控，真实反映做过的工作）——这是「真实使用」最干净的信号。
  - 课程学习：`course_progress.completed_ordinals` / `course_session.status='finished'`。
- **课程↔卡**：skill JSON 带 `course_id` + `cards:[...]`（当前仅 `info-literacy-course` 教 `craap`）。→「哪门课教卡 X」可反查。
- **偏好存储**：无 settings 表；`users.avatar_color` 是先例 → 新增 `users.card_theme`。

## 架构决定

### D1 卡元数据（契约，单一真相源）
`CardSpec` 增量可选字段（不破坏现有卡）：
- `asset_id?: string` — 素材 T 号（如 `"T01"`），无设计则**不设**（留空）。
- `example?: string` — 图鉴详情用的示例；缺省时前端回退到首个 `steps[].methodology.example`，再回退 `methodology.how`。
（`stage` 已存在，用于「适用阶段」；不新增。）

### D2 封面素材（OSS）
- 上传脚本把 4 个 colorway 的 `T00..T38`（`light` 每 T 取一张）经 **admin upload** 传到 OSS（`web_resource` scope），产出 **manifest**：`assetId → { light, "cyber-sage", "cyber-slate", "cyber-warm" }`（各为 objectKey）。
- manifest 作为 `apps/api/internal/cards/covers-manifest.json`（`go:embed`）提交。objectKey 非机密，可入 git。
- 图鉴接口按学生所选 theme 批量 resolve 成短时签名 URL 返回；OSS 未配置（dev 无 key）→ coverUrl 空，前端回退到「文字封面」（同样覆盖无 `asset_id` 的卡）。

### D3 熟练度算法（v1，直觉一致、可迭代）
每 (student, card) 计算 0–100 分与 0–5 星。三成分：
- **课程基础** `courseComp ∈ {0,1}`：教这张卡的课程已结课（`course_session.status='finished'` 或该卡在 course 域 completed）→ 1。「学过方法」。
- **真实使用** `useComp = min(1, log2(1+projCompletions)/log2(1+SAT))`，`SAT=8`：项目域真实完成数（`CountCompletedCardUsesByUser` 的全卡版）。对数曲线 → 边际递减、会饱和（不会无限涨）。
- **跨面迁移** `breadthComp = distinctSurfaces/3`：在 project/course/chat 用过几种面。
`score = round(100 * (0.35*courseComp + 0.5*useComp + 0.15*breadthComp))`。
- **是否遇到**：任一 surface 有 completed，或课程已结课 → encountered=true（彩色）；否则未遇到（灰度、无星）。
- **星级**：`stars = 0` if 未遇到；否则 `1 + floor(score/20)` 截断到 1..5（遇到即 ≥1 星）。
> 直觉核对：只上过课没练过 → 约 35 分 ≈ 2 星；练过 3~4 次且跨面 → 4 星；练满 8 次+跨面+上过课 → 5 星。用得多且真、跨情境迁移 = 高分。纯 chat/course 无谓词的水完成不计入 useComp（沿用 `CountCompletedCardUsesByUser` 的项目域口径）。

### D4 配色偏好（per-user）
迁移 `0047`：`users.card_theme text NOT NULL DEFAULT 'light'`（CHECK in 4 值）。端点 `GET /me`（若已有）或新增 `PUT /me/card-theme`。契约加类型。

### D5 UI（工具卡 tab 重构）
- 顶部 4 色 swatch 选择器（写回 card_theme）。
- 全部卡按 `category`（沿用 `CATEGORY_ORDER`）分组，每卡渲染**封面瓦片**：encountered 彩色 + ⭐ 叠加 + hover 半透明描述；未遇到灰度（CSS filter）。无 coverUrl → 文字封面。
- click → **详情 modal**：半透明封面头（good-to-have）+ 名称 + purpose + methodology(why/how/when) + 示例 + 「去学这张卡的课程 →」（有课才显示）；已学额外显示使用历史（uses/surfaces/lastUsed）+ 心得。
- 未遇到：同上但去掉历史。

### D6 课程结课报告封面（good-to-have，末slice）
`CourseReport` 为该课 `cards[]` 展示封面。资源允许则做。

## 分片
- **S0 数据+素材**：asset_id 映射写入卡 JSON（+ `sync-cards` 镜像）；产出覆盖报告 `docs/2026-08-01-card-asset-coverage.md`；OSS 上传脚本 → 生成并提交 `covers-manifest.json`。
- **S1 后端图鉴+熟练度**：`GET /cards/catalog?theme=` 返回每卡 {元数据, coverUrl(所选theme), proficiency{encountered,score,stars,uses,surfaces,lastUsed}, courseId?}。纯读、无模型。新增全卡项目完成数查询 + 课程已学集合。契约类型。
- **S2 配色偏好**：迁移 0047 + `PUT /me/card-theme` + 契约。
- **S3 UI 图鉴**：重写 `ToolkitCards.tsx`（swatch + 图鉴 + 详情 modal + 课程链接 + 灰度/星/hover）。
- **S4 课程报告封面**（good-to-have）。

每片：build → 子代理评审 → 修 → commit；末尾全分支评审；然后 merge main、push、deploy。

## 铁律核对
- 图鉴是**只读**呈现学生已发生的使用与课程学习；不制造上瘾（无连胜/排行榜/推送；星级是描述性熟练度快照，不是排行、不推送）。✅ 铁律②
- 熟练度只由**真实完成**与课程学习推导，过程即数据（跳过/水完成的口径已按项目域门控）。✅ 铁律④
- 素材只在图鉴/详情/课程报告出现，项目侧栏不出现。✅
- 密钥只在服务端；OSS admin key 不入 git/日志/前端。✅

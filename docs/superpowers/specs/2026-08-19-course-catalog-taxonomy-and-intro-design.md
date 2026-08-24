# 课程目录：分类、结构化介绍与首页策展 · 设计 spec

> 状态：设计定稿，待用户复核。日期：2026-08-19。
> 上游权威：`docs/2026-08-19-courses.md`（33 门课程的总览、分类表、逐门介绍）；
> 授权契约接缝：`docs/2026-08-17-course-authoring-api-handover.md`（教师端课程生成器对接的 `PUT /admin/courses/{slug}/definition`）。

## 目标（一句话）

给课程目录补上**分类**与**结构化课程介绍**两项数据，让学生端把 33 门（逐步上线的）课程按分类组织、首页只展示至多 6 门，并把「谁生产内容 / 谁决定呈现」这条协作边界固化在共享契约里。

## 组织原则（这条决定了所有归属）

> **学生端拥有「分类法 + 呈现」；生成器拥有「内容生产」；课程定义契约是两者之间唯一的接缝。**

- 呈现 / 策展 / 分类归属 → 学生端（`apps/web` + 契约中的受控词表）。
- 封面、工具卡映射、介绍文案 → 教师端生成器（经授权 API 落库）。
- 双方都不各写一份形状，都读 `packages/contracts/src/course.ts`。这是把卡片已有的「新增 = 新增 JSON、不改渲染器」纪律扩展到课程。

## 现状盘点（已存在，不在本期构建）

对照 `packages/contracts/src/course.ts` 与 `apps/api/internal/agent/coursestore.go`：

- **封面**：`CourseSummary.coverUrl` 已有；`ship` 端点发布时原子写入 cover。✅ 已上线。
- **课程工具卡**：`card_ids` / `cardIds` 已是课程↔卡片关系，贯穿 store 与课程报告。✅ 已上线。
- **一句话简介**：`blurb` 已有——但它是一行摘要，不是本期要加的结构化介绍。

于是本期真正新增的数据只有三项：`category`、结构化 `introduction`、首页策展用的 `featuredRank`。全部落在 `course.ts` 一个文件。

## 数据契约（`packages/contracts/src/course.ts`）

### 1. 分类：受控词表（7 个），无 emoji

以「稳定 ascii slug + zh 显示标签」成对存储。生成器发 slug，渲染端显示 label；日后改标签不动存储、不做迁移。**生成器绝不新造第 8 个分类**——与卡片 registry 同一条防漂移铁律。

```ts
export const COURSE_CATEGORIES = [
  { slug: "stance-value",      label: "立场与价值" },
  { slug: "source-check",      label: "信源核查" },
  { slug: "media-literacy",    label: "媒介与信息素养" },
  { slug: "self-knowledge",    label: "自我认知" },
  { slug: "data-literacy",     label: "数据素养" },
  { slug: "research-process",  label: "研究流程" },
  { slug: "argument-writing",  label: "论证写作" },
] as const;

export const CourseCategory = z.enum([
  "stance-value", "source-check", "media-literacy",
  "self-knowledge", "data-literacy", "research-process", "argument-writing",
]);
```

`docs/2026-08-19-courses.md` 的「分类」表给出 33 门课的初始 slug 归属（去掉 emoji 后一一对应）。

### 2. 结构化介绍：schema 驱动，不是 markdown blob

对齐 `docs/2026-08-19-courses.md` 每门课已有的固定结构：导语 → 学生做什么 → 带走什么 → 学科对标（IB / 其他国际 / 国内）→ 关键词。生成器**填** schema，学生端**确定性渲染**——每门课的详情页结构一致，新增课程不碰渲染代码。

```ts
export const CourseAlignment = z.object({
  ib: z.array(z.string()).default([]),
  otherIntl: z.array(z.string()).default([]),   // 其他国际（A-Level / AP / IGCSE …）
  domestic: z.array(z.string()).default([]),    // 国内课标
});

export const CourseIntroduction = z.object({
  hook: z.string().default(""),                 // 导语（叙事钩子）
  whatYouDo: z.string().default(""),            // 学生做什么
  takeaways: z.array(z.string()).default([]),   // 带走什么（要点）
  alignment: CourseAlignment.default({}),        // 学科对标
  keywords: z.array(z.string()).default([]),    // 关键词
});
```

### 3. 首页策展：`featuredRank`

可空整数。**同一段逻辑覆盖「暂时随机」与「日后策展」**：设了 rank 的课程按 rank 升序占据首页至多 6 个槽位；一个都没设时，学生端回退为随机选 6。因此「暂时随机」= 尚无策展数据，翻到「策展」是纯数据变更（设 rank），不是代码变更。有序的 rank 也天然表达「学生该先学的 6 门」这种柔性顺序，而非成瘾式排序。

### 4. 扩展后的 `CourseSummary`

```ts
export const CourseSummary = z.object({
  slug: z.string(),
  branch: z.string(),
  title: z.string(),
  blurb: z.string(),
  time_label: z.string(),
  card_ids: z.array(z.string()),
  step_count: z.number().int(),
  coverUrl: z.string().optional().default(""),
  // 新增：
  category: CourseCategory.nullable().default(null),      // 未分类的历史课程为 null
  introduction: CourseIntroduction.nullable().default(null),
  featuredRank: z.number().int().nullable().default(null),
});
```

三项全部 `nullable` / 有默认值——已上线的少数课程在未回填前不报错，与既有 nil-slice 处理一致（见 memory：`recommendedCourses: null` 报告渲染教训）。

## 学生端（`apps/web`）——拥有分类法 + 呈现

1. **首页**：至多 6 门课程卡片。取 `featuredRank` 非空者按升序；不足或全空时随机补/取 6。底部「查看更多 →」跳转全部课程页。
2. **全部课程页**：按 7 个分类分组展示；`category` 为 null 的课程归入「未分类」尾组（过渡期用，回填后消失）。分类顺序 = `COURSE_CATEGORIES` 声明顺序。
3. **课程详情**：确定性渲染 `introduction`（hook / whatYouDo / takeaways / alignment 三栏 / keywords）。`introduction` 为 null 时回退到 `blurb`。

数据流：`GET /courses`（列表）已返回 `CourseSummary` 数组，扩展字段随之带出；首页/全部课程页/详情页都从同一列表接口取数，无需新端点。

## 教师端 / 生成器——拥有内容生产

1. `PUT /admin/courses/{slug}/definition` 扩展受理 `category` + `introduction`（封面、`card_ids` 已受理）。
2. **校验**（Go 侧，边界校验）：
   - `card_ids` 每一项必须在 34 张卡 registry 内，非法 id 整体拒绝（沿用既有 registry 白名单）。
   - `category` 必须是 7 个 slug 之一，否则拒绝——不接受自由文本分类。
   - `introduction` 只校验外层形状（对象 / 数组），内层深结构真相归 `packages/contracts` 的 Zod 契约。
3. **生成器只出草稿**：生成器起草 `introduction` + 卡片映射 + 分类建议 → 教师复核 → 走既有 `ship`。**绝不自动发布**——这是授权侧对铁律①的回声：生成器起草，教师拥有终稿。
4. 更新 `docs/2026-08-17-course-authoring-api-handover.md`：记录两个新字段的形状、7 个分类 slug、`card_ids` 白名单与 `category` 枚举校验行为。

## 两条独立构建轨（只由契约相连）

- **轨 A（学生端呈现）**：契约扩展 + 首页 cap-6/featured + 全部课程页分类分组 + 详情页 `introduction` 渲染。可即刻构建：对已上线的少数课程手工回填 `category` / `introduction` / `featuredRank`（或先留 null 看回退行为）。
- **轨 B（授权 API 扩展）**：`PUT …/definition` 受理 + 校验两个新字段 + handover 文档更新，供同事的生成器对接。

两轨的唯一交汇点是 `packages/contracts/src/course.ts` 的契约定义——先把它定对，两边各自消费。

## 错误处理与边界

- 未回填字段：`category=null` → 归「未分类」；`introduction=null` → 详情页回退 `blurb`；`featuredRank=null` → 参与随机。三者都不得抛错。
- 非法 `card_ids` / `category`：授权 API 在写入前拒绝并返回明确错误，不静默丢弃。
- 首页少于 6 门可展示课程：展示现有的即可，不补占位。

## 测试

- 契约（`packages/contracts/test/course.test.ts`）：`CourseCategory` 枚举、`CourseIntroduction` 默认值、`CourseSummary` 三新字段的 nullable 解析。
- Go：`PUT …/definition` 受理合法 `category`/`introduction`；拒绝非法 `category` slug 与非 registry `card_ids`。
- 学生端：首页 featured 升序 + 空则随机的选择逻辑；全部课程页按声明顺序分组 + 未分类尾组；详情页 `introduction` 渲染与 null 回退。

## 明确不做

- ❌ 分类带 emoji（用户明确否决）。
- ❌ 首页算法化 / 个性化排序（铁律②）——只做策展 rank + 随机回退。
- ❌ 生成器自动发布或自造新分类。
- ❌ 富文本 / markdown 自由介绍——介绍是 schema 驱动的结构化对象。

# 工具卡选择信号 (Card Selection Signal) · 设计文档

> Layer A of a three-layer card-architecture improvement.
> B (instruction richness) and C (escape-hatch JSX, PoC = belief-spectrum) are deferred to follow-up specs.

**日期：** 2026-06-22
**分支：** `feat/card-selection-signal`

## 背景与问题

工具卡的「用不用 / 用哪张」由 LLM 经 `summon_card(card_id, reason, nudge_text)` 单函数决策（`apps/web/src/agent/prompt.ts`）。这本身是对的——符合「决策层用目录 + 分类，不做检索」的设计。

**真正的 bug 在喂给模型的信号。** 决策层只能看到目录文本（`buildCatalogText`），而目录每张卡只渲染一行 `trigger_condition`：

```
· sift — 横向核查卡 SIFT [tier-0·P0]：学生面对一条来路不明的说法/截图/视频，倾向于停在原页面判断真假
```

根因链：
- `CatalogEntry`（`packages/contracts/src/registry.ts:75`）的类型里**没有 `purpose`**。
- `deriveCatalog`（同文件 `:97`）从 registry 派生目录时**丢弃了 `purpose`**。
- 因此 `buildCatalogText` 无 `purpose` 可渲染，模型从未看到任何一张卡「能为学生做什么」。

`purpose` 是 `CardSpec` 的**必填字段**（`cardSpec.ts:24`），33 张卡全部已写好——只是在派生目录时被丢掉了。

**证据：** 用真实系统 prompt + `summon_card` 工具对实时 DeepSeek 探针 12 次，全部 text-only、0 召唤。模型在「只有症状、没有作用」的薄信号下，叠加重度克制的 prompt，默认永不召唤。

`trigger_keywords`（`cardSpec.ts:33`，如 sift 的 `["是真的吗","截图"]`）是**死元数据**——schema 里有，决策层从不读取。本次不为它接线（关键词匹配正是要避免的方向）。

## 目标

让决策层同时看到每张卡的**何时用（trigger_condition）**与**能帮什么（purpose）**，使模型能把学生当下处境对齐到卡的适用情形 + 产出价值，从而在真正贴合时召唤——而不改变「克制、按需」的铁律。

**非目标（留给 Layer B/C）：**
- 新增需要逐卡撰写的字段（如「何时*不*用」「产出什么」）——Layer B。
- 更丰富的分步教学内容（intro/example/why）——Layer B。
- 每卡自定义 JSX 渲染器（PoC = belief-spectrum）——Layer C。
- 不碰标准信封结构（过程树 / 评估的共同地基）。

## 设计

三处改动，全部围绕「停止丢弃一个已存在的字段」。

### 1. `packages/contracts/src/registry.ts` — 目录携带 `purpose`

- `CatalogEntry` 类型新增 `purpose: string`。
- `deriveCatalog` 映射里新增 `purpose: c.purpose`。

目录仍从 registry 派生（不手写第二份），符合「卡 spec 单一真相源在 registry」。

### 2. `apps/web/src/agent/prompt.ts` — `buildCatalogText` 渲染两个维度

每张卡从一行变为带两个标注的小块：

```
· sift｜横向核查卡 SIFT（步骤引导卡）
   何时用：学生面对来路不明的说法/截图/视频，倾向于停在原页面判断真假
   能帮他：离开这一页、另开标签，看别人怎么评价这个来源，再回来下判断
```

- `interaction_type` 存在时作为括注；不存在则省略。
- 仍按 `category` 分组（保留 `【分类】` 表头）。
- `[tier·prio]` 元数据对模型决策无用，从渲染中移除以降噪（数据仍在 registry，不删字段）。

### 3. `apps/web/src/agent/prompt.ts` — 系统 prompt 一行微调

在「# 工具卡（按需，不是每次）」段落保留全部克制语句，增补一句，明确目录每条现含「何时用 / 能帮他」两栏，且当学生处境**同时贴合两者**时，提议该卡是好的陪练，而非违反克制（铁律 #1 针对的是「不替学生定论」，不是「永不提议工具」）。

## 测试策略 (TDD)

- **contracts**：`deriveCatalog` 的测试断言派生条目含 `purpose` 且等于卡的 `purpose`。
- **prompt**：`buildCatalogText` 的测试断言输出含某卡的 `purpose` 文本与 `trigger_condition` 文本，且仍含 `【分类】` 表头。
- **门禁**：`pnpm -r typecheck && pnpm -r test` 全绿。
- **实时验证（非自动化，但是真正的证据）**：用与之前相同的探针、相同的 system-prompt 构造方式，但用新目录文本，对实时 DeepSeek 重跑召唤探针；对比 0/12 → 新召唤率。结果记录在 PR / 追踪里。

## 风险

- **过度召唤**：更强的信号可能让模型召唤更频繁。缓解：prompt 仍保留全部克制语句；实时探针校准——若召唤过频，回调 prompt 措辞，不回退信号。
- **目录变长 token**：每卡多一行 purpose。33 张卡可接受；若超预算，再议（非本期问题）。

## 验证结果（实时 DeepSeek，2026-06-22）

用临时探针（构造真实 `buildSystemPrompt(demoCatalog(...))` + `summonCardTool`，命中后即停）对实时模型测召唤率：

| 场景 | 修复前 | 修复后 |
|---|---|---|
| 冷开场单句（全部 7 张卡） | 0/12 | 3/14（信号明确的卡会召唤：emotional-alignment、learning-report） |
| 一次实质交谈后（5 张研究类卡） | — | 5/10，且每次都召唤**正确的卡**（sift→sift、belief-spectrum→belief-spectrum、ethics→ethics） |

**结论：** 喂入 `purpose` 让模型能识别贴合度——召唤从「永不」变为「在真正贴合时召唤正确的卡」。冷开场单句对研究类卡仍刻意保持低召唤（教练先陪练，铁律 #1）；真实多轮交互约 50%。

**遗留（Layer B 候选）：** `data-literacy`、`concession` 两轮仍未召唤——其 trigger/purpose 文本或需 Layer B 的更丰富描述符（如「何时不用」「产出什么」）来加强。

> 探针为临时文件（会发起实时调用），验证后已删除，未进入 git。Key 仅来自 gitignored `apps/web/.env.local`。

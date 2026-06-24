# 工具卡：四个修复 + 富教学体验 — 设计

> 状态：设计定稿，待用户复核 → 转 writing-plans。
> 背景：在「迈向多平台」的大重构之前，先把当前版本里五个体验问题修掉。前两项是确诊过的 bug，#4/#5 是陪练回路的小改，#3 是一个完整的富教学体验（视觉已用 visual-companion 验收）。

**Goal:** 修掉两个确诊 bug（卡片跨会话不可点、误触跳过），让模型更主动地递卡且能「一段话 + 一张卡」同回合出现，并为 SIFT×CRAAP 和「内在小人」两张卡做出像老师带课一样的教学弹窗 + 可交互填写体验。

**Tech Stack:** React 18 + Vite + TS + Tailwind（apps/web）；Zod 契约（packages/contracts）；Vitest + Testing Library。门禁：`pnpm -r typecheck && pnpm -r test`。

## Global Constraints（每个任务都隐含遵守）

- **标准信封结构不改。** `CardInstance` 的 `field_values` / `event_trace`（`field_change`/`step_expand`/`note_open`/`skip`/`submit`）/ `status`(`proposed`/`active`/`completed`/`skipped`) 是过程树+评估的共同地基，一律不动。教学弹窗的「被打开」复用既有 `note_open` 事件记录，不新增事件种类。
- **自定义渲染器只能经 `onField` 写 `field_values`，且 key 必须与卡 JSON 完全一致**；最终由 `CardSheetHost` 经 `envelopeReducer` 组装出与默认渲染器一模一样的信封（沿用 belief-spectrum 的护栏测试范式）。
- **新增卡 = 新增 JSON**：本次不改这条；teaching/custom-fill 是「逃生舱」按 card_id 注册，缺省回落到 schema 渲染，不影响其余 31 张卡。
- **API key 只在客户端 `.env.local`（gitignored）**，绝不入 git / 日志 / 抛错 / 信封。临时联调脚本用完即删、绝不提交。
- **四条设计铁律**：AI 克制不替学生定论；不操纵（触发自动、打开由学生确认）；一次只问一个；过程即数据（跳过也是信号——但必须是**有意的**跳过）。
- **视觉语言（本次已定稿）**：A1 居中弹窗（顶部步骤条 + 居中舞台 + 底部圆点/上一步下一步）；手绘角色插画风（内联 SVG，无 emoji，无资源管线）；App 调色板（ink #1C2333、primary #2A3B7A/#EBF0FF、green #4C9A82/#E7F3EE、warm #C9743C/#FBEBDD、rose #C2557A/#FBE7EF、line #EAECF2、bg #F3F4F8）。

---

## #1 — 卡片在开了第二个会话后不可点（确诊 bug）

**根因（已确认）：** `WorkspaceContainer` 用 `useMemo(..., [taskId])` 在切换任务时**重建整个 conversation 对象**，新对象 `pendingCardId: undefined`。而 `createConversation.ts:190` 的 `openCard()` 只设 `phase: "card_active"`，**从不设置 `pendingCardId`**——它默默依赖同一 conversation 生命周期里 `proposal_pending` 步骤遗留的值。一旦切到第二个任务再切回，遗留值没了，点击仍然显示的提议卡片只翻 phase，`WorkspaceView.tsx:68` 的 `activeCard` 解析为 `undefined` → 底部 sheet 不渲染 → 表现为「点不开」。

**修复：** `openCard` 改为 `setState({ phase: "card_active", pendingCardId: cardInstanceId })`，使其自洽、不依赖任何前序状态。

**测试：** 新建一个「重建后的」conversation（fresh，从未经过 proposal_pending），store 里放一张 `proposed` 卡 → 调 `openCard(id)` → 断言 `pendingCardId === id` 且 `phase === "card_active"`，且 `WorkspaceView` 能渲染出 sheet。

---

## #2 — 误触卡外就被标记「跳过」（确诊 bug）

**根因（已确认）：** scrim 的 `onClick={handleClose}`，且 `handleClose → onClose → conversation.skipCard()` 直接写 `status: "skipped"` + skip 事件；X 与「取消」按钮同样走这条。没有「关闭但不跳过」，也没有一个**有意**跳过的入口。

**决策（用户选定）：关闭 ≠ 跳过；跳过是独立的有意按钮。**

- 新增 conversation 方法 `closeCard(cardInstanceId)`：仅关闭 sheet——`setState({ phase: "idle", pendingCardId: undefined })`，**不改 store、不调 LLM、不记 skip**。卡片保持可重新打开。
- `CardSheetHost`：
  - scrim 点击 / X / 「取消」→ `onClose`（关闭，不跳过）。
  - 新增一个**低强调但明确**的「跳过这张卡」入口（footer 左侧文字按钮，rose 色），→ 新 prop `onSkip` → `conversation.skipCard`。它仍然记录 skip 信号（过程即数据）。一次点击即生效（已是有意动作，无需二次确认）。
- `WorkspaceView` 接线：`onClose={(id) => conversation.closeCard(id)}`、`onSkip={(id) => void conversation.skipCard(id)}`。
- 聊天里的提议 chip 在 `proposed` **或** `active` 状态下都可「打开卡」（关闭后卡片停在 `active`，必须仍可重开）。确认 `viewModel`/`ChatLog` 的提议状态映射满足这点。

**测试：** scrim/X/取消 → 调 `closeCard` 而非 `skipCard`；「跳过这张卡」→ 调 `skipCard`；`closeCard` 后卡片仍 `active` 且可重开。更新既有断言 `skipCard on close button` 为 `closeCard`。

---

## #4 — 模型不够主动递卡（更主动但仍克制）

**决策（用户选定）：更主动、但保留克制护栏。** 纯 prompt 改动，不碰回路结构。

- `prompt.ts` 系统提示：去掉「绝大多数轮次，普通陪练就够了 / 不是默认动作」这类把递卡说成稀有例外的措辞；改为正面框定——「当此刻的处境**贴合**某卡的『何时用』时，递这张卡就是好陪练」。**保留**护栏：一次最多一张；不够贴合就别提议；**打开由学生确认**；不替学生定论。
- `summonCardTool` 的 `description`：从「当且仅当…绝大多数轮次不需要调用」软化为「当此刻贴合某卡适用情形时提议」。
- **测试：** 断言新正面措辞出现、旧「稀有例外」措辞消失；断言克制护栏句仍在（一次一张 / 打开由学生确认）。（召唤率是模型行为，不做硬断言；live smoke 保持不变。）

---

## #5 — 一回合里既给文字解释、又给卡

**根因（已确认）：** `runTurn` 命中 summon 时只保留工具入参 `nudge_text`，把模型同时写的 `result.text`（解释）**丢弃**了——是 either/or。底层 API（Anthropic/OpenAI 适配器）本就支持「text + tool_use 同条」，能力在、回路没用。

**决策：单条 assistant 消息同时承载 解释文 + 卡提议。**

- `runTurn` 命中 summon 时：写入的 assistant 消息 `content = result.text`（解释，可能为空），`tool_call = summonCardCall`（`nudge_text` 在 args 里）。——即把 `content` 的语义从「nudge」改回「模型的解释正文」。
- `viewModel.messagesToItems`：遇到带 `tool_call` 的提议消息，若 `content.trim()` 非空，**先发一个 `ai_text` item**（解释气泡），**再发 `proposal` item**（卡片，nudge 取自 args）。空 content 则只发 proposal——与今天 UX 一致。
- `messageMapping`（回灌 LLM）：
  - 已解决（completed/skipped）：仍 assistant{content + toolCalls} + 配对 tool_result；`serializeMessage` 已对空 content 省略 text block，安全。
  - 未解决（pending）：仍以纯文本回灌，但用 `m.content || nudge_text` 兜底，**绝不回灌空 assistant 消息**（nudge_text 从 tool_call args 取）。
- **测试：** summon 且 content 非空 → 两个 item（ai_text 在前、proposal 在后）；content 空 → 仅 proposal；messageMapping 未解决且 content 空 → 回灌文本回落为 nudge_text（非空）。

---

## #3 — 富教学体验：SIFT×CRAAP 与 内在小人（emotional-alignment）

两张卡各得到：**(a) 一个「给我讲讲这个」教学弹窗**（A1 居中、手绘角色、分章节带互动），**(b) 一个可交互的填写渲染器**（复用同一套插画资产，输出标准信封）。其余 31 张卡不变。

### 共享资产（先建，被 a/b 复用）

`apps/web/src/cards/teaching/assets/` —— 纯内联 SVG React 组件，无资源管线：
- `SiftIcons.tsx`：Stop / Investigate / Find / Trace / CRAAP 五个图标（手绘角色风）。
- `InnerPartsCast.tsx`：六个小人角色组件（保护者/完美/拖延/焦虑/讨好/小大人），每个含正面 + 背面（「它在替你挡什么」）。
- `Battery.tsx`：1–5 格情绪电量（可受控渲染，低电量 rose）。
- `Radar.tsx`：CRAAP 五维雷达（受 5 个 0–5 值驱动，长出形状）。
- `SourceChip.tsx`：来源小卡（可信/存疑/不可信配色）。

### (a) 教学弹窗

`apps/web/src/cards/teaching/`：
- `types.ts`：`TeachingModule = { cardId; title; category; chapters: TeachingChapter[] }`；`TeachingChapter = { key; label; badge?; Component: ComponentType }`。章节组件**自包含**——互动状态是本地、临时的「练习」，**不写信封**（用的是抛弃式示例内容，如「中国是否让地球更可持续」头条）。
- `TeachingModal.tsx`：A1 外壳——居中弹窗浮于暗背景；顶部步骤条（章节 label + 完成/当前态）；居中舞台渲染当前章节组件；底部圆点 + 上一步/下一步；右上关闭。管理当前章节 index。Esc/点背景关闭弹窗（仅关弹窗，不影响其下的填写 sheet）。
- `teachingRegistry.ts`：`Record<cardId, TeachingModule>` + `pickTeaching(cardId): TeachingModule | undefined`。
- `modules/SiftCraapTeaching.tsx`：5 章——Stop（呼吸圈 + 一句反问）、Investigate（把示例来源拖进 可信/存疑/不可信）、Find（弱来源 vs Nature 并排滑入）、Trace（顺转发链剥到源头）、**CRAAP（独立整屏进阶章**，雷达为主角，与前四章等重，仅以「★ 进阶」门控）。
- `modules/InnerPartsTeaching.tsx`：章节——情绪电量（拖电量、角色表情随之变）、六个小人（点击翻面看保护意图）、安全岛 & 3 分钟小行动（温柔收尾，强调留空也 OK、今天到此也被尊重）。

### (b) 卡内集成 + 可交互填写

- `CardBodyProps` 增加可选 `hideMethodology?: boolean`；`CardRenderer` 在其为真时跳过逐步「方法」面板。
- `CardSheetHost`：`const teaching = pickTeaching(spec.id)`。若有：
  - 在 sheet 顶部渲染醒目的「给我讲讲这个」按钮（取代这两张卡的小「方法」提示）；点击打开 `TeachingModal`（本地 `showTeaching` 状态）。
  - **首次打开** teaching 时调用既有 `handleNote(firstStepKey)` → 记 `note_open`（复用，过程即数据，不改信封）。
  - 向 Body 传 `hideMethodology`，隐藏这两张卡的逐步方法面板（教学已由弹窗承担）。
- 自定义填写渲染器，注册进 `customRenderers.ts`（沿用 `pickCardBody` 逃生舱）：
  - `SiftCraapRenderer`（`sift_craap`）：各 SIFT 步配图标与分区；Investigate 的 `sources`（repeatable_group）以来源行 + verdict 分段选择呈现；CRAAP 五项 `rating` 用**可拖拽雷达**写 `currency/relevance/authority/accuracy/purpose`(1–5)；`stop`/`better`(textarea)、`trace`(link_check) 沿用。字段 key 与 JSON 完全一致。
  - `InnerPartsRenderer`（`emotional-alignment`）：`energy_level`(rating)→电量控件；`inner_part_choice`(single_choice)→**可点选的小人角色卡**（写回的值是 JSON 里那条**完整选项字符串**）；`not_want_reason`(textarea)→温柔留白输入；`micro_action_choice`(single_choice)→两枚温柔按钮。
  - 两者都**仅经 `onField` 写既有 key**，由 `CardSheetHost`→`envelopeReducer` 出标准信封。
- **护栏测试（每张卡一条，仿 belief-spectrum）：** 经 `CardSheetHost` 跑完一遍填写+提交，断言信封为标准结构——`status: "completed"`、`field_values` 只含该卡 schema 的 key、`event_trace` 含 `field_change`/`submit`。
- **教学弹窗测试：** `pickTeaching` 命中两卡、未命中回 undefined；`TeachingModal` 章节前进/后退；打开教学触发一次 `note_open`；有 teaching 的卡隐藏逐步方法面板、无 teaching 的卡保持不变。

---

## File Structure（新增/改动一览）

**改动：**
- `apps/web/src/agent/createConversation.ts` — #1 openCard 设 pendingCardId；#2 新增 closeCard；#5 runTurn 存 result.text。
- `apps/web/src/agent/prompt.ts` — #4 系统提示 + 工具描述。
- `apps/web/src/agent/messageMapping.ts` — #5 未解决分支空 content 回落 nudge_text。
- `apps/web/src/workspace/viewModel.ts` — #5 提议消息额外发 ai_text item。
- `apps/web/src/workspace/WorkspaceView.tsx` — #2 onClose→closeCard、新增 onSkip。
- `apps/web/src/workspace/CardSheetHost.tsx` — #2 关闭/跳过分离；#3 「给我讲讲这个」+ hideMethodology。
- `apps/web/src/cards/CardRenderer.tsx` — #3 hideMethodology prop。
- `apps/web/src/cards/customRenderers.ts` — #3 注册两张卡的填写渲染器。

**新增：**
- `apps/web/src/cards/teaching/assets/{SiftIcons,InnerPartsCast,Battery,Radar,SourceChip}.tsx`
- `apps/web/src/cards/teaching/{types,TeachingModal,teachingRegistry}.tsx|ts`
- `apps/web/src/cards/teaching/modules/{SiftCraapTeaching,InnerPartsTeaching}.tsx`
- `apps/web/src/cards/renderers/{SiftCraapRenderer,InnerPartsRenderer}.tsx`
- 对应测试文件。

## 实现顺序（自然分组，给 plan 用）

1. **#1 + #2**（bug 修复，互不依赖，回路内小改）。
2. **#4 + #5**（陪练回路 + prompt）。
3. **#3-资产**（共享 SVG 组件，先建被复用）。
4. **#3-教学弹窗**（壳 + 两个 module + 卡内「给我讲讲这个」集成 + hideMethodology）。
5. **#3-填写渲染器**（两张卡的自定义填写 + 护栏测试）。

## Out of Scope（本次明确不做）

- 其余 31 张卡的教学弹窗 / 自定义填写（仅这两张样例）。
- 资源管线 / 图片导入（一律内联 SVG）。
- 改标准信封结构、改评估、教师端、登录鉴权。
- 教学弹窗里的练习数据**不**入信封（仅 `note_open` 记录「学生深入看过方法」）。

## 风险

- **重构churn：** 用户已知大重构在即仍选「full rich build now」。教学弹窗壳与按 card_id 的逃生舱注册较可能平移；最易被重做的是具体章节互动与填写控件的视觉。资产组件（SVG）可复用性高。
- **#5 消息映射边界：** 空 content + 未解决提议必须兜底为 nudge_text，否则回灌空 assistant 消息——已在测试覆盖。

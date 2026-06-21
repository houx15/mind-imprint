# Slice 3b — 决策层 + `summon_card` 回灌循环 + 工作区 (B3 + B4) · Design

> 日期 2026-06-21 ｜ Blocks B3（决策 + summon_card 循环）+ B4（工作区容器）｜ 前置 B0、B1、B2、B2.5、**3a（卡全库导入，33 张已并入 main）**。
> 注：原 S3 拆为 3a（卡数据，✅）/ 3b（本稿）/ 3c（富交互卡，later）。本稿在 3a 之后**修订**：决策层从「2 张卡」改为在**全 33 张**上路由（§5）。
> 双真相源：UI 一切以 `docs/design/思维印记_工作区.dc.html` 为准（逐像素）；非视觉（卡契约、标准信封、克制阶梯、决策层规则）以 `docs/思维印记_Demo_PRD.md` §7/§8/§13.2 为准，架构按 roadmap v0.2 §0.1（前端直连/无后端/BYO-key）。
> 用户决定（2026-06-21）：① B3+B4 作为**一个 slice**；② 卡填写内容以**确定性、schema 派生的 JSON 结构**作为 `tool_result` 回灌（不再用 LLM 二次总结）；③ 决策层目录**全 33 张内联**，demo 工作区用 `demoCatalog` 偏置 demo 卡（§5）。

---

## 1. Goal

让 Phoebe 场景的主动脉（PRD §14 步骤 1–5）在真实 LLM 下稳定走通：学生在工作区与 AI 对话 → AI 在命中时刻调用 `summon_card` → 学生确认打开、填写/跳过卡 → 填写内容回灌 → AI 继续克制陪练。全部前端直连、落 B2.5 浏览器存储。

**本 slice 不含：** 过程树**树体**（S4/B5，仅渲染可折叠面板骨架 + 空态）、评估（S5/B6）、应用外壳/导航/主目录/认证（S6/B7，本 slice 用 dev 入口独立跑工作区，沿用既往 slice 的 DevApp 模式）。

---

## 2. 架构总览

四层，复用既有骨架，**不改冻结契约**（`CardInstance`/`Message` 形状不动）：

1. **LLM 层（扩展 B2）** — 给 `llm/` 加 tool-use：`ChatTool`/`ToolCall` 类型、`ChatRequest.tools`、`ChatResult.toolCalls`/`stopReason`，openai + anthropic 两种格式的请求塑形、tool_call 解析、tool_result 回传塑形。注册**单一** `summon_card` 工具。
2. **决策层（B3-prompt）** — `buildSystemPrompt(catalog)`：教练阶梯系统 prompt（克制 + 教练手段 + Markdown，工具卡是按需手段非默认）+ 从 registry 派生的**按分类组织**的紧凑目录（先选分类→再选卡）。`summon_card` 的 `card_id` 枚举 = 目录里的卡 id 集合。
3. **回灌序列化（contracts）** — `serializeCardForRefeed(spec, instance)`：纯函数，schema 派生，把卡填写内容变成回灌 JSON 结构。一函数覆盖全卡库。
4. **编排器（B3-loop）** — `createConversation(store, deps)` 控制器 + `useConversation` hook，跑「学生发言 → LLM → 调卡/文本 → 渲染 → 填写 → 回灌 → 继续」状态机。
5. **工作区 UI（B4）** — `WorkspaceView` 及子组件，照 HTML 渲染：面包屑 / 对话主轴（三种消息：学生 / AI 文本 / 提议）/ composer / 树骨架 / 底部抽屉宿主（挂 S1 `CardRenderer` 激活态）。

```
student types ──► createConversation.send()
                    │ store.appendMessage(user)
                    │ build LLM messages from store  ◄──────────────┐
                    ▼                                               │
                 llm.chat({ system, messages, tools:[summon_card] }) │
                    │                                               │
        ┌───────────┴─────────────┐                                │
     text reply              toolCall summon_card                   │
        │                         │ create CardInstance(proposed)   │
   store assistant msg            │ store assistant proposal msg    │
        │                         ▼                                 │
        │                  WorkspaceView renders inline proposal    │
        │                         │ student 打开卡 / 暂不            │
        │                    openCard ─► bottom sheet (CardRenderer) │
        │                         │ fill (S1 envelope reducer)      │
        │                    submitCard / skipCard                  │
        │                         │ store.putCard(completed|skipped)│
        │                         │ serializeCardForRefeed ─────────┘
        ▼                         ▼ (tool_result paired to tool-call id)
   render in chat            re-enter loop ► AI 继续克制陪练（一次一问）
```

---

## 3. 消息 ↔ 卡 的关联（不动冻结契约）

B2.5 的 `Message.tool_call` 是 `z.unknown().nullable()`，S2.5 carry-forward 明确「S3 owns its real shape」。S3 在 contracts 定义其 Zod schema，并在读取时 `parse`（落实 S2.5 「validate on read」）：

```ts
// packages/contracts/src/summonCard.ts
export const SummonCardArgs = z.object({
  card_id: z.string(),       // 必须是目录中的卡 id（解析后再校验属于 registry）
  reason: z.string(),        // 为什么此刻该用这张卡（内部记录，不展示给学生）
  nudge_text: z.string(),    // 给学生看的一句提议语
});
export const SummonCardCall = z.object({
  id: z.string(),                 // provider 侧 tool-call id —— 用于 tool_result 配对
  name: z.literal("summon_card"),
  args: SummonCardArgs,
  card_instance_id: z.string(),   // 关联这条提议消息 → 它创建的 CardInstance
});
export type SummonCardArgs = z.infer<typeof SummonCardArgs>;
export type SummonCardCall = z.infer<typeof SummonCardCall>;
```

- **一条提议** = 一条 `role:"assistant"` 的 `Message`，其 `tool_call` 为 `SummonCardCall`；`content` 存 `nudge_text`（也作为兜底展示）。
- 该提议对应的 `CardInstance`（按 `card_instance_id` 查）持有实时状态：`proposed | active | completed | skipped`。
- `CardInstance` / `Message` 的字段形状**不变**。

---

## 4. LLM 层：实现保留的 tools（两种格式）

### 4.1 类型（`apps/web/src/llm/types.ts` 增量）

```ts
export interface ChatTool {
  name: string;
  description: string;
  parameters: Record<string, unknown>; // JSON Schema（object）
}
export interface ToolCall { id: string; name: string; args: Record<string, unknown> }

export interface ChatRequest {
  messages: ChatMessage[];
  tools?: ChatTool[];
  maxTokens?: number;
  temperature?: number;
}
export type StopReason = "stop" | "tool_call" | "length" | "other";
export interface ChatResult {
  text: string;                 // 文本部分（无文本时为 ""）
  toolCalls?: ToolCall[];       // 解析出的 tool 调用（无则 undefined）
  stopReason?: StopReason;
  usage?: ChatUsage;
  raw?: unknown;
}
```

`ChatMessage` 需要能携带 tool 角色/结果。为不破坏既有 `ChatMessage {role,content}`，新增可选字段：

```ts
export type ChatRole = "system" | "user" | "assistant" | "tool";
export interface ChatMessage {
  role: ChatRole;
  content: string;
  toolCalls?: ToolCall[];   // assistant 发起的调用（回放历史时用）
  toolCallId?: string;      // role:"tool" 的结果，配对到某次调用 id
}
```

### 4.2 适配器塑形

| | openai 格式 | anthropic 格式 |
|---|---|---|
| 工具声明 | `tools:[{type:"function",function:{name,description,parameters}}]` | `tools:[{name,description,input_schema}]` |
| assistant 发起调用 | `message.tool_calls:[{id,function:{name,arguments(JSON string)}}]` | `content:[{type:"tool_use",id,name,input}]` |
| 解析为 `ToolCall` | `id`,`name`,`args=JSON.parse(arguments)` | `id`,`name`,`args=input` |
| 回传工具结果 | 一条 `{role:"tool",tool_call_id,content}` 消息 | 一条 `role:"user"` 消息，块 `{type:"tool_result",tool_use_id,content}` |
| 回放 assistant 调用 | `{role:"assistant",content:null,tool_calls:[…]}` | `{role:"assistant",content:[{type:"tool_use",…}]}` |
| `stopReason` | `finish_reason:"tool_calls"→"tool_call"` | `stop_reason:"tool_use"→"tool_call"` |

两适配器各自把统一的 `ChatMessage[]`（含 `tool` 角色 / `toolCalls`）翻译成各自线格式；anthropic 仍照 S2：所有 `system` 消息抬到顶层 `system`。错误/密钥安全沿用 S2（key 只在 header，绝不进解析结果/日志）。

### 4.3 `summon_card` 工具定义（单一函数）

```ts
function summonCardTool(catalog: Catalog): ChatTool // name:"summon_card"
// parameters: { card_id: { enum: catalog.map(c=>c.id) }, reason:string, nudge_text:string }, required 三者
```

---

## 5. 决策层：系统 prompt + 目录　👀 待你审阅

`apps/web/src/agent/prompt.ts`（新目录 `agent/` 装决策层 + 编排器）：

```ts
buildCatalogText(catalog: Catalog): string   // 按 category 分组：每组一个【分类】标题，组内每卡一行「· id — name [tier·priority]：trigger_condition」
buildSystemPrompt(catalog: Catalog): string  // 教练阶梯（克制 + 教练手段 + markdown）+ 分类目录（全 33 张内联）
demoCatalog(full: Catalog): Catalog          // 给 demo 工作区用：偏置 demo 卡——剔除/后置 3 张近义库卡(sift/craap/steelman)，保证 Phoebe 主动脉里 sift_craap/concession 胜出
```

**草拟系统 prompt（中文，待你逐字审阅 / 改写——这是铁律 #1 的载体）：**

```
# 角色
你是「思维印记」里的思维陪练——更像一位**导师 / 教练**，服务国际课程（IB）方向的学生。学生带着自己真实的任务（论文、项目、课题、阅读）来。你的价值不是当一台答案机，而是在协作中把「思考」交回给他自己，让他离开时比来时更会想。

# 你怎么帮（克制，但不是只会反问）
- **不替他定论、不替他写、不替他判对错好坏。** 该他想的，别替他想完。
- 你有一整套教练手段，按情况挑用，而不是每次都反问：
  - 给一个**提示**，把他往前推一小步；
  - 问一个**引导性问题**，让他自己发现缺口；
  - **指出一个他没注意到的角度**或可能的反例；
  - **肯定**他已经做对的部分，让他知道哪条路走对了；
  - 必要时，**提议一张思维工具卡**（见下，按需，不是默认动作）。
- **聚焦一步。** 一次只推进一个焦点，简短、口语；别一口气抛一堆问题或长篇大论——保护他的思考节奏。
- **善用排版。** 用 Markdown 让重点一眼可见：`**加粗**`关键词，必要时配小标题 / 列表 / `>` 引用。突出重点，但整体仍简短。

# 工具卡（按需，不是每次）
工具卡只是你众多手段中的一种，**不是默认动作**。绝大多数轮次，普通陪练就够了。
- 只有当学生此刻的处境**正好命中**某张卡的适用情形时，才用 `summon_card` 提议——一次最多一张；拿不准、不够贴合，就**别提议**，继续正常陪练。
- 先按**分类**判断他现在卡在哪一类问题上，再在该类里挑最贴合的那一张。若多张卡都贴合，优先更**综合 / 更贴合当前任务**的那张。
- `reason` 写给系统看（为什么此刻贴合）；`nudge_text` 写给学生看（一句自然、邀请式、不命令的话）。
- 学生**婉拒 / 跳过**一张卡时，尊重他，继续陪练，**不要反复弹**同一张卡。
- 学生**提交**一张卡后，你会拿到他填写内容的结构化结果。基于他**自己写下的**东西继续——先接住他的思考，再就其中**一处**往前推一步。

# 可用的思维工具卡目录（按分类）
{{catalog}}
```

> 审阅要点：导师/教练的语气是否到位（不只是反问）；工具卡是否被摆在「按需的一种手段」而非默认；Markdown 强调是否合适；summon_card 会不会过度/不足触发；nudge 风格；是否需要补学科/语言（中英）指示。

### 目录策略（33 张卡）与近义卡偏置

S3a 后 registry 有 **33 张卡**（31 库卡 + 2 demo 卡）。`buildCatalogText` 仍**整体内联**完整分类目录：33 张 × 每张一行 ≈ 33 行 ≈ 2–3k tokens，对系统 prompt 完全可承受，不构成膨胀。每行带 `[tier·priority]` 注记，决策层按「先选分类 → 再在该类挑最贴合的一张」两层走（prompt 层指令，非 schema 变更；`summon_card` 仍单函数）。

**近义卡偏置（不改卡数据）：** 因保留 demo 卡（`sift_craap`/`concession`）与库卡（`sift`/`craap`/`steelman`）并存，demo 工作区喂给决策层的是 `demoCatalog(full)`——把 3 张近义库卡**剔除或后置**，保证 Phoebe 主动脉里 demo 卡胜出。库卡的真实 tier/priority **不动**（`sift`/`craap` 仍是库设计里的 tier-0 常驻核心）；偏置只发生在 demo 实例的目录装配层。另加一条软规则进系统 prompt：「若多张卡都贴合，优先更**综合 / 更贴合当前任务**的那张」。

**`related` 容错（S3a carry-forward）：** 决策层 / 任何 `related` 遍历必须容忍指向 registry 中不存在的 id（库 frontmatter 可能引用未建卡），不得因悬空邻居抛错。

**渐进式披露扩展点（卡库 ≫ 33 时启用，本 slice 仍不建，YAGNI）：** 把目录降为**只列分类**（每类一句「何时适用」）+ 新增 `browse_cards(category)` 工具按需拉取该类卡详情再 `summon_card`。`card_id` 始终按 registry 校验（§3/§7 兜底），不依赖固定 enum，故「全内联 → browse 按需」的切换不改契约。这是 README §渐进式披露 A（tier-0 常驻 / tier-1·2 浮现）的工程落地，留待真实卡库规模再做。

---

## 6. 回灌序列化（contracts，schema 派生）

```ts
// packages/contracts/src/refeed.ts
export interface RefeedAnswer { label: string; value: unknown } // value: string|number|Array<Record<string,unknown>>
export interface RefeedStep { title: string; answers: RefeedAnswer[] }
export interface RefeedPayload {
  card_id: string; card_name: string;
  status: "completed" | "skipped";
  steps?: RefeedStep[]; // skipped 时省略
}
export function serializeCardForRefeed(spec: CardSpec, instance: CardInstance): RefeedPayload;
```

规则（纯函数，确定性）：
- `status:"skipped"` → 只回 `{card_id,card_name,status}`，告诉 AI 学生已婉拒（配合铁律 #4，AI 不再追弹）。
- 否则遍历 `spec.steps[].fields[]`：每个 field 用其 `label` 配 `instance.field_values[step.key][field.key]`：
  - `text`/`textarea`/`link_check`/`single_choice` → 原始值（字符串）。
  - `rating` → 数字。
  - `multi_choice` → 字符串数组。
  - `repeatable_group` → 数组，每项是 `{item_field.label: value}` 的对象（用 item_fields 的 label 重映射 key）。
- 缺省 / 未填字段：跳过该 answer（不塞空串），保持回灌简洁。
- 编排器最终 `JSON.stringify(payload)` 作为 tool_result 的 content。

单测锚定真实 SIFT 信封（NASA / Nature Sustainability / repeatable_group 多来源 / 跳过态）。

---

## 7. 编排器：`createConversation` + `useConversation`

`apps/web/src/agent/createConversation.ts`，仿 B2.5 的 `createStore`/`useStore`：

```ts
interface ConversationDeps {
  store: Store;
  chat: (cfg: Partial<LlmConfig>, req: ChatRequest) => Promise<ChatResult>; // = llm client
  config: Partial<LlmConfig>;
  registry: Record<string, CardSpec>;
  now?: () => string; genId?: () => string;
}
interface Conversation {
  getSnapshot(): ConvState;     // { taskId, phase, pendingCardId?, error? }
  subscribe(l: () => void): () => void;
  send(text: string): Promise<void>;
  openCard(cardInstanceId: string): void;   // proposed → active（挂底部抽屉）
  submitCard(cardInstanceId: string, finalInstance: CardInstance): Promise<void>; // → completed → 回灌 → 续聊
  skipCard(cardInstanceId: string): Promise<void>;  // → skipped → 回灌(skipped) → 续聊
}
function createConversation(deps: ConversationDeps): Conversation;
```

`phase`：`idle | awaiting_llm | proposal_pending | card_active | error`。UI 据此禁用 composer 等。

### store→LLM 消息映射（关键，👀 图示）

每次要调 LLM 时，从 store 的该任务消息流**重放**为 `ChatMessage[]`，前置 `system`：

```
store messages（按序）                    ►  LLM ChatMessage[]
─────────────────────────────────────────────────────────────
(system 由 buildSystemPrompt 生成，置顶)
user   "我看到一篇…(含链接)"              ►  {role:"user", content}
assistant 提议(tool_call=SummonCardCall)  ►  {role:"assistant", toolCalls:[{id,name,args}]}
   └─ 查 CardInstance(card_instance_id):
      · completed/skipped:                ►  {role:"tool", toolCallId:id,
                                              content: JSON.stringify(serializeCardForRefeed(spec,ci))}
      · proposed/active(未结算):           ►  （不追加 tool 结果；此时也不会触发新的 LLM 调用——
                                              编排器只在「学生发言」或「卡结算」后才调 LLM）
assistant 文本                            ►  {role:"assistant", content}
```

不变式：每个 assistant 的 tool_use 调用，若其卡已结算，则其后紧跟一条配对 `tool` 结果（同 `id`）；未结算的提议永远是消息流的**末尾**（不会有「悬空 tool_use 后面又接别的」）。这保证两种 provider 的协议都合法。

### 状态机要点
- `send` 仅在 `idle` 可用（`proposal_pending`/`card_active` 时 composer 禁用或允许继续聊由 UI 决定——默认：提议未决时仍可发文字，提议保留在流里，体现「过程即数据」，不强迫）。MVP：`proposal_pending` 时允许继续 `send`（AI 会在系统 prompt 约束下不追弹）。
- 一轮 LLM 若返回多个 toolCalls：MVP 只取第一个 `summon_card`，其余忽略并记一条 console 警告（demo 一次一卡）。
- `card_id` 不在 registry：丢弃该 toolCall，记录警告，按普通文本处理（决策层目录由 registry 派生，理论上不会发生，做兜底）。
- 错误（LlmError）：`phase:"error"`，存错误信息，UI 显示重试；不抛进渲染。

---

## 8. 工作区 UI（B4）— 逐像素照 HTML

`apps/web/src/workspace/`：

- **`WorkspaceView`** — `showWorkspace` 整块：顶部面包屑（`← 返回所有任务` · 任务标题 · `进行中` 徽标 · `已用 {completedCount} 张工具卡`），中部 `flex` 分对话区 + 树面板，底部抽屉宿主。
- **`ChatLog`** + **消息视图模型**：把 store `Message[]` + 关联 `CardInstance` 映射为渲染项：
  - `isStudent`：右侧深蓝气泡；若 content 含链接 → 链接 chip（`hasLink`/`link`）。
  - `isAIText`：头像 + 白气泡。
  - `isProposal`：内联提议卡（`建议工具卡` 标签 + `category` + `cardName` + `nudge`），按关联 CardInstance 状态出三态：`isProposed`（打开卡 / 暂不，先继续）、`isCompleted`（已完成 · 已钉到过程树）、`isSkipped`（已跳过（已记录为信号）+ 仍可打开）。
- **`Composer`**（`footerComposer`）：textarea（占位「把你的想法发给陪练……」）+ 发送按钮；`awaiting_llm` 时禁用/转圈。
- **`TreePanel`**（骨架）：`treeOpen`/`treeClosed` 折叠切换 + 标题「过程树 · 只读」+ **空态** 文案「边做边长 · 随评估归并枝节」。**不渲染派生节点**（S4）。
- **`CardSheetHost`**（`hasActiveCard`）：scrim + 底部抽屉，头部（`现在轮到你想` · category · 卡名 · purpose · 「这个工具怎么用」· 关闭），抽屉体**挂载已有 S1 `<CardRenderer>` 激活态**，提交/跳过回调接 `submitCard`/`skipCard`。**不重写卡渲染。**

样式全用 `mk-*` token；内容用真实 Phoebe 素材，零 lorem。

> 注：S1 已有 `ProposalBubble` 组件，但 HTML 的提议是**对话内联**形态（在 ChatLog 里）。B4 的提议按 HTML 内联实现；若 S1 `ProposalBubble` 形状一致则复用，否则在 ChatLog 内按 HTML 实现内联提议视图（渲染器仍是 schema 驱动，不含卡专属分支）。实施时以 HTML 为准。

---

## 9. dev 入口

仿前几个 slice：在 `DevApp` 增一个「工作区」tab，挂一个用真实 localStorage store + 真实 `chat`（读设置页 config）+ `CARD_REGISTRY` 装配的 `WorkspaceView`，预置 Phoebe 任务种子。这样无需 B7 外壳即可端到端手测主动脉（有 key 时）。

---

## 10. 测试（TDD）

- **适配器**（扩 S2 mock-fetch 单测）：两格式各测——带 tools 的请求塑形、tool_call 解析（→`ToolCall`）、`tool` 结果消息回传塑形、`stopReason` 映射、回放 assistant 调用的塑形。
- **序列化器**：纯单测，锚真实 SIFT 信封——四步全填、repeatable_group 多来源、未填字段跳过、跳过态。
- **prompt/目录**：`buildCatalogText` 对全 33 张每卡一行（含 trigger_condition + tier/priority 注记）、按 category 分组；`summonCardTool` 的 enum == 传入目录的卡 id 集合；`demoCatalog(full)` 剔除/后置 `sift`/`craap`/`steelman`，且仍含 `sift_craap`/`concession`；`related` 遍历对悬空 id 不抛错。
- **编排器**（核心，对 **fake LLM** + 内存 store）：脚本化「先 toolCall 后文本」——断言：① user/assistant/提议消息按序入库；② CardInstance 生命周期 proposed→active→completed/skipped；③ 续聊时重放的 `messages` 含配对到正确 tool-call id 的 `tool` 结果，其 content 为序列化 payload；④ 多 toolCall 只取首个；⑤ 未知 card_id 兜底；⑥ LlmError → phase:error。
- **工作区**（RTL + fake LLM）：发消息 → 提议内联出现 → 打开卡（底部抽屉挂 CardRenderer）→ 填 → 提交 → 提议转「已完成」且 AI 跟进文本渲染；跳过路径；树骨架折叠；composer 在 awaiting 时禁用。

---

## 11. Global constraints（贯穿每个 task）

- 验收门槛：`pnpm -r typecheck` + `pnpm -r test` 全绿；`vitest`/`vite build` 不做类型检查，类型安全只由 `tsc --noEmit` 保证——每个改 web/contracts 的 task 各自跑 typecheck。
- 前端直连、无后端、BYO-key；key 只在请求 header，**绝不**进 `raw`/错误/日志/渲染（沿用 S2 已验证的安全边界）。
- 卡 spec 单一真相源在 registry；目录从它派生；`summon_card` 单函数，不一卡一 tool。
- 新增卡 = 新增 JSON，不改渲染器/序列化器代码（schema 驱动）；回灌序列化对全卡库通用。
- 冻结契约不动：`CardInstance`/`Message` 字段形状不变；`tool_call` 的具体形状由本 slice 在 contracts 定义并**读取时 parse**。
- 标准信封是脊椎：卡激活/填写/提交复用 S1 `envelopeReducer` 与 `CardRenderer`，不另写一套。
- 四条铁律：克制（系统 prompt + 确定性回灌，不替学生定论）· 不操纵（卡自动提议但「打开」由学生点确认；工具卡是按需手段而非默认动作）· 一次只问一个 —— 落地为「**聚焦一步**」：简短、不堆问题；教练手段含提示 / 引导问题 / 指方向 / 肯定 / 工具卡，不限于反问 · 过程即数据（跳过也回灌为 skipped 信号）。
- UI 逐像素照 `docs/design/思维印记_工作区.dc.html`，用 `mk-*` token + 真实 Phoebe 内容。

---

## 12. Out of scope / carry-forward

- 过程树**树体** → S4/B5（本 slice 仅面板骨架 + 空态）。
- 评估 → S5/B6。应用外壳（导航/主目录/记录页/设置页/认证 mock）→ S6/B7（本 slice 用 DevApp 入口）。
- 暂不做：真流式、重试退避、一轮多卡 / 并行 tool call（demo 一次一卡）、tool_call 之外的 function calling 用法。
- carry-forward：
  - **S4**：过程树从 store 的 `card_instance`+`message`（含 `SummonCardCall` 的 reason/nudge）派生；本 slice 已把关联信息存全。
  - **S6**：把 `WorkspaceView` 从 DevApp 入口接进真实外壳路由；DevApp tabs 仍缺 `role="tab"`/`aria-selected`。
  - 若将来要「一轮多卡」或并行 tool call，需扩状态机与映射不变式。

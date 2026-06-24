# 工具卡：修复 + 富教学体验 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修两个确诊 bug（卡片跨会话不可点、误触跳过），让模型更主动递卡且能「文字+卡」同回合，并为 SIFT×CRAAP 与「内在小人」两张卡做出富教学弹窗 + 可交互填写。

**Architecture:** 三层解耦不变。Bug 修复落在 conversation/视图层；#4/#5 落在陪练回路与消息映射；#3 用「按 card_id 注册的逃生舱」叠加：共享内联 SVG 资产 → 教学弹窗（A1 壳 + 两个 module）→ 自定义填写渲染器（经 `onField` 出标准信封）。缺省一律回落到现有 schema 渲染，其余 31 张卡零影响。

**Tech Stack:** React 18 + Vite + TS + Tailwind（apps/web）；Zod 契约（packages/contracts）；Vitest + Testing Library。

**设计来源：** `docs/superpowers/specs/2026-06-24-card-bugfixes-and-rich-teaching-design.md`。验收过的视觉 mockup（含可直接移植的 SVG）在 `.superpowers/brainstorm/55230-1782285283/content/`：`asset-style.html`、`sift-interactions.html`、`inner-parts-cast.html`、`teaching-modal-centered.html`。

## Global Constraints（每个任务都隐含遵守）

- **标准信封不改：** `CardInstance` 的 `field_values` / `event_trace`(`field_change`/`step_expand`/`note_open`/`skip`/`submit`) / `status`(`proposed`/`active`/`completed`/`skipped`) 不动。教学弹窗「被打开」复用既有 `note_open`，不新增事件种类。
- **自定义渲染器只经 `onField` 写 `field_values`，key 必须与卡 JSON 完全一致**；信封由 `CardSheetHost`→`envelopeReducer` 组装。
- **逃生舱按 card_id 注册，缺省回落** schema 渲染；新增 module/renderer 不改其余卡。
- **视觉语言（定稿）：** A1 居中弹窗（顶部步骤条 + 居中舞台 + 底部圆点 + 上一步/下一步）；手绘角色插画风（内联 SVG，无 emoji，无资源管线）；调色板 ink `#1C2333`、primary `#2A3B7A`/`#EBF0FF`、green `#4C9A82`/`#E7F3EE`、warm `#C9743C`/`#FBEBDD`、rose `#C2557A`/`#FBE7EF`、line `#EAECF2`、bg `#F3F4F8`。
- **铁律：** 不替学生定论；触发自动、打开由学生确认；一次只问一个；过程即数据（跳过是信号，但必须**有意**）。
- **门禁：** 每个任务结束前 `pnpm -r typecheck && pnpm -r test` 必须全绿。API key 只在 gitignored `.env.local`，绝不入 git/日志/抛错/信封。
- **不做：** 其余 31 张卡的教学/自定义填写；资源管线；改信封/评估/教师端/登录。教学练习数据**不**入信封（仅 `note_open`）。

---

### Task 1: #1 — `openCard` 自洽设置 `pendingCardId`

**Files:**
- Modify: `apps/web/src/agent/createConversation.ts:190-196`
- Test: `apps/web/src/agent/createConversation.test.ts`（已存在，追加用例）

**Interfaces:**
- Consumes: `Conversation.openCard(cardInstanceId: string): void`、`ConvState { phase; pendingCardId? }`。
- Produces: `openCard` 后 `getSnapshot()` 返回 `{ phase: "card_active", pendingCardId: cardInstanceId }`。

- [ ] **Step 1: 写失败测试** — 模拟「重建后的」conversation（从未经过 proposal_pending），store 里直接放一张 proposed 卡，点开它。

```ts
// 追加到 createConversation.test.ts
it("openCard sets pendingCardId so a re-created conversation can still open a card", () => {
  const store = makeTestStore();              // 见文件内现有 helper
  const task = store.createTask({ title: "t" });
  const ci = newEnvelope("sift_craap", task.id, () => "2026-01-01T00:00:00.000Z", () => "ci-1");
  store.putCard(ci);                          // proposed，未经过本会话的 proposal_pending
  const conv = createConversation({
    store, chat: async () => ({ text: "", toolCalls: [] }),
    config: {}, registry: CARD_REGISTRY, catalog: CATALOG, taskId: task.id,
  });
  conv.openCard("ci-1");
  expect(conv.getSnapshot()).toMatchObject({ phase: "card_active", pendingCardId: "ci-1" });
});
```
> 若文件无 `makeTestStore`/`CATALOG` helper，复用该文件顶部已有的 setup（参照现有用例的 store 构造方式），保持一致。

- [ ] **Step 2: 跑测试确认 RED** — `pnpm --filter @mind-imprint/web test -- createConversation` → 失败：`pendingCardId` 为 `undefined`。

- [ ] **Step 3: 实现** — 改 `openCard`：

```ts
openCard(cardInstanceId: string): void {
  const ci = store.getCard(cardInstanceId);
  if (!ci) throw new Error(`[createConversation] unknown card instance "${cardInstanceId}"`);
  const activated = envelopeReducer(ci, { type: "activate" }, now);
  store.putCard(activated);
  setState({ phase: "card_active", pendingCardId: cardInstanceId });
},
```

- [ ] **Step 4: 跑测试确认 GREEN**，并跑全量 `pnpm -r test` 确认无回归。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/agent/createConversation.ts apps/web/src/agent/createConversation.test.ts
git commit -m "fix(cards): openCard sets pendingCardId so re-created conversations can reopen cards"
```

---

### Task 2: #2 — 新增 `conversation.closeCard`（关闭不跳过）

**Files:**
- Modify: `apps/web/src/agent/createConversation.ts`（`Conversation` 接口 + 实现）
- Test: `apps/web/src/agent/createConversation.test.ts`

**Interfaces:**
- Produces: `Conversation.closeCard(cardInstanceId: string): void` — 仅关闭 sheet：`setState({ phase: "idle", pendingCardId: undefined })`，**不改 store、不调 LLM、不记 skip**；卡片维持 `active`，可重开。

- [ ] **Step 1: 写失败测试**

```ts
it("closeCard dismisses the sheet without skipping or calling the LLM", async () => {
  const chat = vi.fn(async () => ({ text: "x", toolCalls: [] }));
  const store = makeTestStore();
  const task = store.createTask({ title: "t" });
  const ci = newEnvelope("sift_craap", task.id, () => "2026-01-01T00:00:00.000Z", () => "ci-1");
  store.putCard(ci);
  const conv = createConversation({ store, chat, config: {}, registry: CARD_REGISTRY, catalog: CATALOG, taskId: task.id });
  conv.openCard("ci-1");
  conv.closeCard("ci-1");
  expect(conv.getSnapshot()).toMatchObject({ phase: "idle", pendingCardId: undefined });
  expect(store.getCard("ci-1")!.status).toBe("active");       // 未变 skipped
  expect(store.getCard("ci-1")!.event_trace.some((e) => e.kind === "skip")).toBe(false);
  expect(chat).not.toHaveBeenCalled();                         // 未调 LLM
});
```

- [ ] **Step 2: 跑测试确认 RED** — `closeCard is not a function`。

- [ ] **Step 3: 实现** — 在 `Conversation` 接口加 `closeCard(cardInstanceId: string): void;`，并在返回对象里实现（放在 `openCard` 之后）：

```ts
closeCard(_cardInstanceId: string): void {
  // 关闭 sheet，不记跳过、不调 LLM；卡片保持 active，可重新打开。
  setState({ phase: "idle", pendingCardId: undefined });
},
```

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/agent/createConversation.ts apps/web/src/agent/createConversation.test.ts
git commit -m "feat(cards): add conversation.closeCard — dismiss sheet without recording a skip"
```

---

### Task 3: #2 — CardSheetHost 分离「关闭」与「跳过」+ 视图接线

**Files:**
- Modify: `apps/web/src/workspace/CardSheetHost.tsx`（新增 `onSkip` prop；scrim/X/取消 走 `onClose`；footer 加「跳过这张卡」走 `onSkip`）
- Modify: `apps/web/src/workspace/WorkspaceView.tsx:249-256`（`onClose`→`closeCard`，新增 `onSkip`→`skipCard`）
- Test: `apps/web/src/workspace/CardSheetHost.test.tsx`（新建）、`apps/web/src/workspace/WorkspaceView.test.tsx`（更新既有 close 断言）

**Interfaces:**
- Consumes: `conversation.closeCard`、`conversation.skipCard`（Task 2 + 既有）。
- Produces: `CardSheetHost` props 增 `onSkip: (cardInstanceId: string) => void`；`onClose` 语义改为「仅关闭」。

- [ ] **Step 1: 写失败测试**（CardSheetHost.test.tsx 新建）

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CardSheetHost } from "./CardSheetHost";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";

function setup() {
  const onClose = vi.fn(); const onSkip = vi.fn(); const onSubmit = vi.fn();
  const spec = CARD_REGISTRY["sift_craap"];
  const ci = newEnvelope("sift_craap", "t1", () => "2026-01-01T00:00:00.000Z", () => "ci-1");
  render(<CardSheetHost cardInstance={ci} spec={spec} onSubmit={onSubmit} onClose={onClose} onSkip={onSkip} />);
  return { onClose, onSkip, onSubmit };
}

it("X (关闭) closes without skipping", async () => {
  const { onClose, onSkip } = setup();
  await userEvent.click(screen.getByRole("button", { name: "关闭" }));
  expect(onClose).toHaveBeenCalledWith("ci-1");
  expect(onSkip).not.toHaveBeenCalled();
});

it("取消 closes without skipping", async () => {
  const { onClose, onSkip } = setup();
  await userEvent.click(screen.getByRole("button", { name: "取消" }));
  expect(onClose).toHaveBeenCalledWith("ci-1");
  expect(onSkip).not.toHaveBeenCalled();
});

it("跳过这张卡 records a deliberate skip", async () => {
  const { onClose, onSkip } = setup();
  await userEvent.click(screen.getByRole("button", { name: "跳过这张卡" }));
  expect(onSkip).toHaveBeenCalledWith("ci-1");
  expect(onClose).not.toHaveBeenCalled();
});
```

- [ ] **Step 2: 跑测试确认 RED** — 无「跳过这张卡」按钮 / `onSkip` 未定义。

- [ ] **Step 3: 实现** — CardSheetHost：
  - props 类型加 `onSkip: (cardInstanceId: string) => void;`，解构里加 `onSkip`。
  - `handleClose` 注释改为「仅关闭，不跳过」（行为已是调用 `onClose`，语义正确，无需改调用）。
  - 新增 `function handleSkip() { onSkip(cardInstance.id); }`。
  - footer 左侧把现有的提示文案替换为「跳过这张卡」文字按钮（rose 色，低强调），放在原「填写过程会被采集…」文案左边或取代之：

```tsx
<button
  type="button"
  onClick={handleSkip}
  style={{ background:"none", border:"none", color:"#C2557A", fontSize:"13px",
    fontWeight:600, cursor:"pointer", padding:"10px 0", fontFamily:"inherit" }}
>
  跳过这张卡
</button>
```
  > 原 footer 右侧「取消 / 提交」保持不变；「取消」仍 `onClick={handleClose}`。scrim、X 不变（已是 `handleClose`）。

- [ ] **Step 4: 实现视图接线** — WorkspaceView.tsx 的 `<CardSheetHost>`：

```tsx
<CardSheetHost
  cardInstance={activeCard}
  spec={activeSpec}
  onSubmit={(id, final) => void conversation.submitCard(id, final)}
  onClose={(id) => conversation.closeCard(id)}
  onSkip={(id) => void conversation.skipCard(id)}
/>
```

- [ ] **Step 5: 更新既有测试** — WorkspaceView.test.tsx 里断言「关闭按钮调用 skipCard」的用例（约 line 565）改为断言调用 `closeCard`：

```tsx
it("calls conversation.closeCard when the bottom sheet close button is clicked", async () => {
  const cards = [makeCardInstance({ status: "active" })];
  const store = makeStore({ cards });
  const conv = makeConversation("card_active", "ci-sift");   // 该 helper 的 mock 需含 closeCard
  render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
  await userEvent.click(screen.getByRole("button", { name: /关闭/ }));
  expect(conv.closeCard).toHaveBeenCalledWith("ci-sift");
});
```
  > 在该文件的 `makeConversation` mock 工厂里给返回对象补 `closeCard: vi.fn()`（与 `skipCard` 并列）。

- [ ] **Step 6: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/workspace/CardSheetHost.tsx apps/web/src/workspace/WorkspaceView.tsx apps/web/src/workspace/CardSheetHost.test.tsx apps/web/src/workspace/WorkspaceView.test.tsx
git commit -m "feat(cards): close ≠ skip — backdrop/X/取消 just close, explicit 跳过这张卡 records the skip"
```

---

### Task 4: #4 — 更主动的递卡 prompt（仍克制）

**Files:**
- Modify: `apps/web/src/agent/prompt.ts:53-60`（工具卡段）、`:75`（tool description）
- Test: `apps/web/src/agent/prompt.test.ts`（更新既有断言）

**Interfaces:**
- Produces: `buildSystemPrompt(catalog)` 含正面措辞「贴合就递」、保留「打开由学生确认」「一次最多一张」；不再含「绝大多数轮次，普通陪练就够了」。`summonCardTool().description` 不再含「绝大多数轮次不需要调用」。

- [ ] **Step 1: 写/改失败测试** — prompt.test.ts：

```ts
it("system prompt frames cards as 'offer when it fits', not a rare exception", () => {
  const p = buildSystemPrompt(CATALOG);
  expect(p).toContain("贴合就递");
  expect(p).toContain("打开由学生确认");
  expect(p).toContain("一次最多一张");
  expect(p).not.toContain("绝大多数轮次，普通陪练就够了");
});

it("summon_card tool description is invitational, not 'rarely call'", () => {
  expect(summonCardTool(CATALOG).description).not.toContain("绝大多数轮次不需要调用");
});
```
  > 若该文件已有断言旧句「不是永不递工具」，改为新句「不是把工具藏起来」（见下）。

- [ ] **Step 2: 跑测试确认 RED**。

- [ ] **Step 3: 实现** — 把 prompt.ts 的 `# 工具卡（按需，不是每次）` 整段（第 53–60 行）替换为：

```
# 工具卡（贴合就递，别犹豫）
工具卡是你手里一种有力的手段。当此刻的处境**贴合**某张卡时，把它递给他就是好陪练——别因为「怕越界」就压着不给。
- 目录里每张卡都标了「何时用」(适用情形) 和「能帮他」(这张卡能给学生什么)。当学生此刻的处境贴合某卡这两栏时，就用 \`summon_card\` 提议它——这不违反克制；克制是指不替他定论，**不是把工具藏起来**。
- **一次最多一张**；同时贴合多张时，挑最综合 / 最贴合当前任务的那张。真的没有贴合的卡，就正常陪练，别硬塞。
- 先按**分类**判断他现在卡在哪一类问题上，再在该类里挑最贴合的那一张。
- \`reason\` 写给系统看（为什么此刻贴合）；\`nudge_text\` 写给学生看（一句自然、邀请式、不命令的话）。**打开由学生确认**——你只是提议。
- 学生**婉拒 / 跳过**一张卡时，尊重他，继续陪练，**不要反复弹**同一张卡。
- 学生**提交**一张卡后，你会拿到他填写内容的结构化结果。基于他**自己写下的**东西继续——先接住他的思考，再就其中**一处**往前推一步。
```

  并把 `summonCardTool` 的 description 改为：

```ts
description: "当此刻贴合某卡的适用情形时，提议这张卡。一次最多一张。",
```

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/agent/prompt.ts apps/web/src/agent/prompt.test.ts
git commit -m "feat(agent): reframe card-summoning prompt to 'offer when it fits', keep restraint guardrails"
```

---

### Task 5: #5 — runTurn 保留模型解释文（content = result.text）

**Files:**
- Modify: `apps/web/src/agent/createConversation.ts:141-146`（summon 分支的 appendMessage）
- Test: `apps/web/src/agent/createConversation.test.ts`

**Interfaces:**
- Produces: summon 命中时，store 里的 assistant 消息 `content = result.text`（解释，可能 `""`），`tool_call.args.nudge_text` 仍是短 nudge。

- [ ] **Step 1: 写失败测试**

```ts
it("keeps the model's explanatory text alongside a summoned card", async () => {
  const chat = async () => ({
    text: "这条说法值得先核一下来源。",                          // 模型的解释正文
    toolCalls: [{ id: "tc1", name: "summon_card",
      args: { card_id: "sift_craap", reason: "r", nudge_text: "要不要用这张卡溯源？" } }],
  });
  const store = makeTestStore();
  const task = store.createTask({ title: "t" });
  const conv = createConversation({ store, chat, config: {}, registry: CARD_REGISTRY, catalog: CATALOG, taskId: task.id });
  await conv.send("中国让地球更可持续吗");
  const msgs = store.listMessages(task.id);
  const assistant = msgs.find((m) => m.role === "assistant" && m.tool_call)!;
  expect(assistant.content).toBe("这条说法值得先核一下来源。");   // 解释保留，而非被 nudge 覆盖
});
```

- [ ] **Step 2: 跑测试确认 RED** — 当前 `content` 是 `nudge_text`。

- [ ] **Step 3: 实现** — 把 summon 分支里的 appendMessage 改为：

```ts
store.appendMessage({
  task_id: taskId,
  role: "assistant",
  content: result.text,        // 解释正文（可能为空）；nudge_text 在 tool_call.args 里
  tool_call: summonCardCall,
});
```

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/agent/createConversation.ts apps/web/src/agent/createConversation.test.ts
git commit -m "feat(agent): keep model explanation text on a summon (content = result.text)"
```

---

### Task 6: #5 — viewModel 同时渲染解释气泡 + 卡提议

**Files:**
- Modify: `apps/web/src/workspace/viewModel.ts:71-90`
- Test: `apps/web/src/workspace/viewModel.test.ts`

**Interfaces:**
- Consumes: assistant 消息 `content`(解释) + `tool_call`(含 nudge)。
- Produces: 一条提议消息可产出两个 item —— 若 `content` 非空且不等于 nudge，先 `ai_text` 再 `proposal`；否则仅 `proposal`。

- [ ] **Step 1: 写失败测试**

```ts
it("emits an ai_text bubble before the proposal when the model also explained", () => {
  const msg: Message = {
    id: "m1", task_id: "t1", role: "assistant",
    content: "先核一下来源。",
    tool_call: { id: "tc1", name: "summon_card", card_instance_id: "ci1",
      args: { card_id: "sift_craap", reason: "r", nudge_text: "用这张卡？" } },
    created_at: "2026-01-01T00:00:00.000Z",
  };
  const ci = newEnvelope("sift_craap", "t1", () => msg.created_at, () => "ci1");
  const items = messagesToItems([msg], () => ci, (id) => CARD_REGISTRY[id]);
  expect(items[0]).toEqual({ kind: "ai_text", text: "先核一下来源。" });
  expect(items[1]).toMatchObject({ kind: "proposal", nudge: "用这张卡？" });
});

it("emits only the proposal when there is no separate explanation", () => {
  const msg: Message = {
    id: "m1", task_id: "t1", role: "assistant", content: "",
    tool_call: { id: "tc1", name: "summon_card", card_instance_id: "ci1",
      args: { card_id: "sift_craap", reason: "r", nudge_text: "用这张卡？" } },
    created_at: "2026-01-01T00:00:00.000Z",
  };
  const ci = newEnvelope("sift_craap", "t1", () => msg.created_at, () => "ci1");
  const items = messagesToItems([msg], () => ci, (id) => CARD_REGISTRY[id]);
  expect(items).toHaveLength(1);
  expect(items[0]).toMatchObject({ kind: "proposal" });
});
```

- [ ] **Step 2: 跑测试确认 RED**。

- [ ] **Step 3: 实现** — 在 viewModel 命中 `spec !== undefined` 分支里，push proposal **之前**插入：

```ts
if (spec !== undefined) {
  const explanation = msg.content.trim();
  if (explanation && explanation !== args.nudge_text.trim()) {
    items.push({ kind: "ai_text", text: msg.content });
  }
  const item: ProposalItem = {
    kind: "proposal",
    cardInstanceId: card_instance_id,
    category: spec.category,
    cardName: spec.name,
    nudge: args.nudge_text,
    status: ci ? toProposalStatus(ci.status) : "proposed",
  };
  items.push(item);
  continue;
}
```
  > `explanation !== nudge` 的守卫让旧数据（历史里 content==nudge）不会重复显示。

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/viewModel.ts apps/web/src/workspace/viewModel.test.ts
git commit -m "feat(chat): render explanation bubble + card proposal from one assistant turn"
```

---

### Task 7: #5 — messageMapping 未解决分支不回灌空消息

**Files:**
- Modify: `apps/web/src/agent/messageMapping.ts:43-49`
- Test: `apps/web/src/agent/messageMapping.test.ts`

**Interfaces:**
- Consumes: 未解决提议消息（`content` 可能为空，nudge 在 `call.args.nudge_text`）。
- Produces: 未解决分支回灌 `content || call.args.nudge_text`，永不为空字符串。

- [ ] **Step 1: 写失败测试**

```ts
it("falls back to nudge_text when an unresolved proposal has empty content", () => {
  const call = { id: "tc1", name: "summon_card", card_instance_id: "ci1",
    args: { card_id: "sift_craap", reason: "r", nudge_text: "用这张卡？" } };
  const msg: Message = { id: "m1", task_id: "t1", role: "assistant", content: "",
    tool_call: call, created_at: "2026-01-01T00:00:00.000Z" };
  const ci = newEnvelope("sift_craap", "t1", () => msg.created_at, () => "ci1"); // proposed/active = 未解决
  const out = buildLlmMessages({ systemPrompt: "s", messages: [msg],
    cardById: () => ci, specById: (id) => CARD_REGISTRY[id] });
  const assistant = out.find((m) => m.role === "assistant")!;
  expect(assistant.content).toBe("用这张卡？");          // 非空
});
```

- [ ] **Step 2: 跑测试确认 RED** — 当前回灌 `content` 为 `""`。

- [ ] **Step 3: 实现** — 未解决分支：

```ts
} else {
  // UNRESOLVED：以纯文本回灌，content 为空时回落 nudge_text，绝不回灌空 assistant 消息。
  result.push({ role: "assistant", content: m.content || call.args.nudge_text });
}
```

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/agent/messageMapping.ts apps/web/src/agent/messageMapping.test.ts
git commit -m "fix(agent): unresolved proposal refeed falls back to nudge_text, never empty"
```

---

### Task 8: #3 — 共享内联 SVG 资产组件

**Files:**
- Create: `apps/web/src/cards/teaching/assets/SiftIcons.tsx`、`InnerPartsCast.tsx`、`Battery.tsx`、`Radar.tsx`、`SourceChip.tsx`
- Test: `apps/web/src/cards/teaching/assets/assets.test.tsx`

**Interfaces (Produces):**
- `SiftIcons`: 具名导出 `StopIcon`、`InvestigateIcon`、`FindIcon`、`TraceIcon`、`CraapIcon`，每个 `(props: { size?: number }) => JSX.Element`。
- `InnerPartsCast`: 具名导出 `INNER_PARTS: { key: string; name: string; quote: string; protects: string; Avatar: ComponentType<{ size?: number }> }[]`，六项，`key` ∈ `protector|perfect|procrastinator|anxious|pleaser|littleadult`。
- `Battery`: `({ level, max=5, size? }: { level: number; max?: number; size?: number }) => JSX.Element`（level 1..max；≤2 用 rose）。
- `Radar`: `({ values, labels }: { values: number[]; labels: string[] }) => JSX.Element`（values 0..5，五维）。
- `SourceChip`: `({ name, verdict }: { name: string; verdict: "ok"|"q"|"bad" }) => JSX.Element`。

> **SVG 来源：** 直接移植已验收 mockup 里的 `<svg>`：角色与图标见 `.superpowers/brainstorm/55230-1782285283/content/inner-parts-cast.html` 与 `sift-interactions.html`；电量格子见 inner-parts-cast.html 的 `.cells`/`.cell`；雷达见 sift-interactions.html 的 CRAAP `<polygon>`；来源 chip 见 sift-interactions.html 的 `.src`。颜色用 Global Constraints 调色板。

- [ ] **Step 1: 写失败测试**

```tsx
import { render } from "@testing-library/react";
import { StopIcon, InvestigateIcon, FindIcon, TraceIcon, CraapIcon } from "./SiftIcons";
import { INNER_PARTS } from "./InnerPartsCast";
import { Battery } from "./Battery";
import { Radar } from "./Radar";
import { SourceChip } from "./SourceChip";

it("renders all five SIFT icons as svg", () => {
  for (const Icon of [StopIcon, InvestigateIcon, FindIcon, TraceIcon, CraapIcon]) {
    const { container } = render(<Icon size={48} />);
    expect(container.querySelector("svg")).toBeTruthy();
  }
});

it("exposes the six inner parts with names, quotes and avatars", () => {
  expect(INNER_PARTS.map((p) => p.key)).toEqual(
    ["protector","perfect","procrastinator","anxious","pleaser","littleadult"]);
  for (const p of INNER_PARTS) {
    expect(p.name).toBeTruthy(); expect(p.quote).toBeTruthy(); expect(p.protects).toBeTruthy();
    const { container } = render(<p.Avatar size={48} />);
    expect(container.querySelector("svg")).toBeTruthy();
  }
});

it("battery renders `level` filled cells out of max", () => {
  const { container } = render(<Battery level={2} max={5} />);
  expect(container.querySelectorAll('[data-cell="on"]').length).toBe(2);
});

it("radar and source chip render", () => {
  expect(render(<Radar values={[4,3,5,2,4]} labels={["时效","相关","权威","准确","目的"]} />).container.querySelector("svg")).toBeTruthy();
  expect(render(<SourceChip name="NASA" verdict="ok" />).container.textContent).toContain("NASA");
});
```

- [ ] **Step 2: 跑测试确认 RED** — 模块不存在。

- [ ] **Step 3: 实现五个资产文件** — 移植 mockup SVG。要点：`Battery` 每格渲染 `<span data-cell={i < level ? "on" : "off"} .../>`（供测试计数）；`INNER_PARTS` 的六项 `name/quote/protects` 取自 `emotional-alignment.json` 的选项文案（如 `protector` name「保护者」、quote「别去冒险，可能会受伤」、protects 写一句「它在替你挡住可能的受伤」）；`Radar` 用 `values` 计算 polygon 顶点。全部纯展示，无本地状态写信封。

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/cards/teaching/assets/
git commit -m "feat(cards): shared inline-SVG teaching assets (SIFT icons, inner-parts cast, battery, radar, source chip)"
```

---

### Task 9: #3 — CardRenderer 支持 `hideMethodology`

**Files:**
- Modify: `apps/web/src/cards/CardRenderer.tsx`（`Props`/`CardBodyProps` + 渲染）
- Test: `apps/web/src/cards/cardRenderer.methodology.test.tsx`（已存在，追加用例）

**Interfaces:**
- Produces: `CardBodyProps` 增可选 `hideMethodology?: boolean`；为真时 `CardRenderer` 不渲染任何 `MethodologyPanel`。

- [ ] **Step 1: 写失败测试**

```tsx
it("hides every 方法 panel when hideMethodology is set", () => {
  const spec = CARD_REGISTRY["sift_craap"];
  render(<CardRenderer card={spec} values={{}} onField={() => {}} onExpandStep={() => {}} hideMethodology />);
  expect(screen.queryByRole("button", { name: "方法" })).toBeNull();
});
```

- [ ] **Step 2: 跑测试确认 RED** — 仍渲染「方法」按钮。

- [ ] **Step 3: 实现** — `Props` 加 `hideMethodology?: boolean;`；`CardRenderer` 解构出它并传给两处渲染，把 `<MethodologyPanel .../>` 包成条件：在 `always` section 与 `OnDemandStep` 里 `{!hideMethodology && <MethodologyPanel step={step} onNote={onNote} />}`。`OnDemandStep` 的参数类型用 `Omit<Props, "card">` 已含新字段，透传即可。

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/cards/CardRenderer.tsx apps/web/src/cards/cardRenderer.methodology.test.tsx
git commit -m "feat(cards): CardBodyProps.hideMethodology to suppress per-step 方法 panels"
```

---

### Task 10: #3 — 教学模块类型 + 注册表

**Files:**
- Create: `apps/web/src/cards/teaching/types.ts`、`apps/web/src/cards/teaching/teachingRegistry.ts`
- Test: `apps/web/src/cards/teaching/teachingRegistry.test.ts`

**Interfaces (Produces):**
```ts
// types.ts
import type { ComponentType } from "react";
export type TeachingChapter = { key: string; label: string; badge?: string; Component: ComponentType };
export type TeachingModule = { cardId: string; title: string; category: string; chapters: TeachingChapter[] };
// teachingRegistry.ts
export const teachingRegistry: Record<string, TeachingModule>;        // 初始为空 {}
export function pickTeaching(cardId: string): TeachingModule | undefined;
```

- [ ] **Step 1: 写失败测试**

```ts
import { pickTeaching, teachingRegistry } from "./teachingRegistry";
it("returns undefined for cards without a teaching module", () => {
  expect(pickTeaching("nonexistent")).toBeUndefined();
});
it("registry is keyed by cardId", () => {
  for (const [k, v] of Object.entries(teachingRegistry)) expect(v.cardId).toBe(k);
});
```

- [ ] **Step 2: 跑测试确认 RED** — 模块不存在。

- [ ] **Step 3: 实现** — `types.ts` 如上；`teachingRegistry.ts`：

```ts
import type { TeachingModule } from "./types";
export const teachingRegistry: Record<string, TeachingModule> = {};   // Task 12/13 填充
export function pickTeaching(cardId: string): TeachingModule | undefined {
  return teachingRegistry[cardId];
}
```

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/cards/teaching/types.ts apps/web/src/cards/teaching/teachingRegistry.ts apps/web/src/cards/teaching/teachingRegistry.test.ts
git commit -m "feat(cards): teaching-module types + card_id registry (pickTeaching)"
```

---

### Task 11: #3 — TeachingModal 外壳（A1：步骤条 + 舞台 + 导航）

**Files:**
- Create: `apps/web/src/cards/teaching/TeachingModal.tsx`
- Test: `apps/web/src/cards/teaching/TeachingModal.test.tsx`

**Interfaces:**
- Consumes: `TeachingModule`（Task 10）。
- Produces: `TeachingModal({ module, onClose }: { module: TeachingModule; onClose: () => void })`。一次渲染一章，顶部步骤条高亮当前章，底部「上一步/下一步」+ 圆点；首章禁用「上一步」，末章「下一步」变「完成」并调 `onClose`；右上「关闭」调 `onClose`；点暗背景 scrim 关闭。

- [ ] **Step 1: 写失败测试**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeachingModal } from "./TeachingModal";
import type { TeachingModule } from "./types";

const mod: TeachingModule = { cardId: "x", title: "测试课", category: "测试", chapters: [
  { key: "a", label: "第一章", Component: () => <p>章节A内容</p> },
  { key: "b", label: "第二章", Component: () => <p>章节B内容</p> },
]};

it("shows the first chapter and advances to the next", async () => {
  const onClose = vi.fn();
  render(<TeachingModal module={mod} onClose={onClose} />);
  expect(screen.getByText("章节A内容")).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: "下一步" }));
  expect(screen.getByText("章节B内容")).toBeTruthy();
});

it("last chapter's 完成 closes the modal", async () => {
  const onClose = vi.fn();
  render(<TeachingModal module={mod} onClose={onClose} />);
  await userEvent.click(screen.getByRole("button", { name: "下一步" }));
  await userEvent.click(screen.getByRole("button", { name: "完成" }));
  expect(onClose).toHaveBeenCalled();
});

it("close button closes the modal", async () => {
  const onClose = vi.fn();
  render(<TeachingModal module={mod} onClose={onClose} />);
  await userEvent.click(screen.getByRole("button", { name: "关闭" }));
  expect(onClose).toHaveBeenCalled();
});
```

- [ ] **Step 2: 跑测试确认 RED**。

- [ ] **Step 3: 实现** — `useState(0)` 记当前章 index；按 A1 mockup（`teaching-modal-centered.html` 的 A1）布局：居中弹窗浮于 `rgba(22,28,46,.34)` scrim；顶部步骤条遍历 `module.chapters` 标 done/on；舞台渲染 `chapters[idx].Component`；底部上一步（idx>0 可用）/ 圆点 / 下一步（末章文案「完成」→ `onClose`）。右上「关闭」按钮 `aria-label="关闭"` → `onClose`。scrim `onClick={onClose}`。

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/cards/teaching/TeachingModal.tsx apps/web/src/cards/teaching/TeachingModal.test.tsx
git commit -m "feat(cards): TeachingModal shell — centered A1 stepper + stage + nav"
```

---

### Task 12: #3 — SIFT×CRAAP 教学模块（5 章互动）

**Files:**
- Create: `apps/web/src/cards/teaching/modules/SiftCraapTeaching.tsx`
- Modify: `apps/web/src/cards/teaching/teachingRegistry.ts`（注册 `sift_craap`）
- Test: `apps/web/src/cards/teaching/modules/SiftCraapTeaching.test.tsx`

**Interfaces:**
- Consumes: Task 8 资产（`SiftIcons`、`Radar`、`SourceChip`）、`TeachingModule` 类型。
- Produces: `SiftCraapTeaching: TeachingModule`（`cardId:"sift_craap"`，5 章：`stop|investigate|find|trace|craap`，label 用 mockup 文案）。互动状态全是章节组件内部本地 state，**不写信封**。

- [ ] **Step 1: 写失败测试**

```tsx
import { render, screen } from "@testing-library/react";
import { SiftCraapTeaching } from "./SiftCraapTeaching";
import { pickTeaching } from "../teachingRegistry";

it("is a 5-chapter module registered under sift_craap", () => {
  expect(SiftCraapTeaching.cardId).toBe("sift_craap");
  expect(SiftCraapTeaching.chapters.map((c) => c.key)).toEqual(
    ["stop","investigate","find","trace","craap"]);
  expect(pickTeaching("sift_craap")).toBe(SiftCraapTeaching);
});

it("each chapter component renders without crashing", () => {
  for (const ch of SiftCraapTeaching.chapters) {
    const { unmount } = render(<ch.Component />);
    unmount();
  }
});
```

- [ ] **Step 2: 跑测试确认 RED**。

- [ ] **Step 3: 实现** — 按 `sift-interactions.html` 实现 5 章组件（移植其 DOM/SVG/交互意图）：
  - `Stop`：呼吸圈（CSS scale 动画）+ 一行「你打算用这条信息说明什么？」示例输入（本地 state）。
  - `Investigate`：示例来源 `SourceChip` 拖/点进 可信/存疑/不可信（本地分类 state；用点击切换即可，不必真 DnD）。
  - `Find`：弱来源 vs Nature 并排对比（点击「看更权威版本」滑入）。
  - `Trace`：转发链节点，点击逐层剥到「原始论文」。
  - `Craap`：整屏进阶章，五个滑杆驱动 `Radar`（本地 `number[5]` state，与前四章等重）。
  在文件末尾导出 `export const SiftCraapTeaching: TeachingModule = { cardId:"sift_craap", title:"SIFT×CRAAP 信息核查", category:"信息素养", chapters:[...] }`，并在 `teachingRegistry.ts` 顶部 import 后写 `teachingRegistry["sift_craap"] = SiftCraapTeaching;`（或在对象字面量里登记）。

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/cards/teaching/modules/SiftCraapTeaching.tsx apps/web/src/cards/teaching/teachingRegistry.ts apps/web/src/cards/teaching/modules/SiftCraapTeaching.test.tsx
git commit -m "feat(cards): SIFT×CRAAP teaching module — 5 illustrated interactive chapters"
```

---

### Task 13: #3 — 内在小人 教学模块

**Files:**
- Create: `apps/web/src/cards/teaching/modules/InnerPartsTeaching.tsx`
- Modify: `apps/web/src/cards/teaching/teachingRegistry.ts`（注册 `emotional-alignment`）
- Test: `apps/web/src/cards/teaching/modules/InnerPartsTeaching.test.tsx`

**Interfaces:**
- Consumes: Task 8 资产（`Battery`、`InnerPartsCast`）。
- Produces: `InnerPartsTeaching: TeachingModule`（`cardId:"emotional-alignment"`，章节 `energy|cast|closing`）。

- [ ] **Step 1: 写失败测试**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InnerPartsTeaching } from "./InnerPartsTeaching";
import { pickTeaching } from "../teachingRegistry";

it("is registered under emotional-alignment with the expected chapters", () => {
  expect(InnerPartsTeaching.cardId).toBe("emotional-alignment");
  expect(InnerPartsTeaching.chapters.map((c) => c.key)).toEqual(["energy","cast","closing"]);
  expect(pickTeaching("emotional-alignment")).toBe(InnerPartsTeaching);
});

it("the cast chapter shows the six parts and flips to reveal what each protects", async () => {
  const cast = InnerPartsTeaching.chapters.find((c) => c.key === "cast")!;
  render(<cast.Component />);
  expect(screen.getByText("保护者")).toBeTruthy();
  await userEvent.click(screen.getByText("保护者"));
  expect(screen.getByText(/替你挡/)).toBeTruthy();   // 翻面后的保护意图文案
});
```

- [ ] **Step 2: 跑测试确认 RED**。

- [ ] **Step 3: 实现** — 按 `inner-parts-cast.html`：
  - `energy`：`Battery` + 拖动改 level（本地 state），低电量配文「先承认这一点」。
  - `cast`：遍历 `INNER_PARTS` 渲染六张角色卡，点击翻面显示 `p.protects`（用本地 `flippedKey` state）。
  - `closing`：安全岛（可留空）+ 3 分钟小行动两枚温柔按钮，强调「留空也 OK / 今天到此也被尊重」。
  导出 `InnerPartsTeaching` 并在 registry 登记 `emotional-alignment`。

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/cards/teaching/modules/InnerPartsTeaching.tsx apps/web/src/cards/teaching/teachingRegistry.ts apps/web/src/cards/teaching/modules/InnerPartsTeaching.test.tsx
git commit -m "feat(cards): 内在小人 teaching module — energy meter + 6-character flip cast"
```

---

### Task 14: #3 — CardSheetHost 接入「给我讲讲这个」+ 隐藏方法面板

**Files:**
- Modify: `apps/web/src/workspace/CardSheetHost.tsx`
- Test: `apps/web/src/workspace/CardSheetHost.teaching.test.tsx`（新建）

**Interfaces:**
- Consumes: `pickTeaching`（Task 10）、`TeachingModal`（Task 11）、注册过的两个 module（Task 12/13）、`hideMethodology`（Task 9）。
- Produces: 有 teaching 的卡顶部显示「给我讲讲这个」按钮；点击打开 `TeachingModal`；首次打开记一次 `note_open`（复用 `handleNote`）；并向 `Body` 传 `hideMethodology`。无 teaching 的卡：行为完全不变。

- [ ] **Step 1: 写失败测试**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CardSheetHost } from "./CardSheetHost";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";

function renderCard(cardId: string, onSubmit = vi.fn()) {
  const ci = newEnvelope(cardId, "t1", () => "2026-01-01T00:00:00.000Z", () => "ci-1");
  render(<CardSheetHost cardInstance={ci} spec={CARD_REGISTRY[cardId]}
    onSubmit={onSubmit} onClose={vi.fn()} onSkip={vi.fn()} />);
}

it("shows 给我讲讲这个 and opens the teaching modal for sift_craap", async () => {
  renderCard("sift_craap");
  const btn = screen.getByRole("button", { name: "给我讲讲这个" });
  await userEvent.click(btn);
  expect(screen.getByRole("button", { name: "关闭" })).toBeTruthy();   // 弹窗打开
});

it("a card without a teaching module shows no 给我讲讲这个 and keeps 方法 panels", () => {
  renderCard("concession");                                            // 无 teaching 的卡
  expect(screen.queryByRole("button", { name: "给我讲讲这个" })).toBeNull();
  expect(screen.getAllByRole("button", { name: "方法" }).length).toBeGreaterThan(0);
});

it("teaching cards hide their per-step 方法 panels", () => {
  renderCard("sift_craap");
  expect(screen.queryByRole("button", { name: "方法" })).toBeNull();
});
```

- [ ] **Step 2: 跑测试确认 RED**。

- [ ] **Step 3: 实现** — CardSheetHost：
  - 顶部 import `pickTeaching`、`TeachingModal`。
  - 组件内：`const teaching = pickTeaching(spec.id);` `const [showTeaching, setShowTeaching] = useState(false);`
  - header 区（`spec.purpose` 下方）当 `teaching` 存在时渲染按钮：

```tsx
{teaching && (
  <button type="button" onClick={() => { handleNote(spec.steps[0].key); setShowTeaching(true); }}
    style={{ marginTop:"10px", display:"inline-flex", alignItems:"center", gap:"7px",
      background:"#EBF0FF", color:"#2A3B7A", border:"none", borderRadius:"10px",
      padding:"8px 14px", fontSize:"13px", fontWeight:700, cursor:"pointer", fontFamily:"inherit" }}>
    给我讲讲这个
  </button>
)}
```
  - `<Body ... hideMethodology={!!teaching} />`（给 Body 传新 prop）。
  - 在最外层 return 末尾、`</div>` 之前挂弹窗：`{showTeaching && teaching && <TeachingModal module={teaching} onClose={() => setShowTeaching(false)} />}`。
  > `note_open` 复用既有 `handleNote`，不改信封结构（满足「过程即数据」）。

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/CardSheetHost.tsx apps/web/src/workspace/CardSheetHost.teaching.test.tsx
git commit -m "feat(cards): 给我讲讲这个 entry opens TeachingModal; teaching cards hide 方法 panels, record note_open"
```

---

### Task 15: #3 — SIFT×CRAAP 自定义填写渲染器

**Files:**
- Create: `apps/web/src/cards/renderers/SiftCraapRenderer.tsx`
- Modify: `apps/web/src/cards/customRenderers.ts`（注册 `sift_craap`）
- Test: `apps/web/src/cards/renderers/SiftCraapRenderer.test.tsx`

**Interfaces:**
- Consumes: `CardBodyProps`（Task 9 后含 `hideMethodology`）、资产 `SiftIcons`/`Radar`、既有字段组件（`TextAreaField`/`LinkCheckField`/`RepeatableGroupField` 可复用，见 `fieldRegistry`）。
- Produces: `SiftCraapRenderer: ComponentType<CardBodyProps>`。**只**经 `onField` 写既有 key：`stop`(textarea)、`sources`(repeatable_group: `[{name,type,verdict}]`)、`better`(textarea)、`trace`(link_check)、`currency/relevance/authority/accuracy/purpose`(rating 1–5)。

- [ ] **Step 1: 写护栏失败测试**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SiftCraapRenderer } from "./SiftCraapRenderer";
import { pickCardBody } from "../customRenderers";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

const ALLOWED = new Set(["stop","sources","better","trace","currency","relevance","authority","accuracy","purpose"]);

it("is registered for sift_craap", () => {
  expect(pickCardBody("sift_craap")).toBe(SiftCraapRenderer);
});

it("writes ONLY schema field keys via onField (standard envelope guardrail)", async () => {
  const writes: string[] = [];
  render(<SiftCraapRenderer card={CARD_REGISTRY["sift_craap"]} values={{}}
    onField={(k) => writes.push(k)} onExpandStep={() => {}} />);
  // 在 Stop 输入框写一笔
  await userEvent.type(screen.getByLabelText(/你打算用这条信息说明什么/), "测试");
  for (const k of writes) expect(ALLOWED.has(k)).toBe(true);
  expect(writes).toContain("stop");
});

it("CRAAP radar sliders write rating keys 1..5", async () => {
  const vals: Record<string, unknown> = {};
  render(<SiftCraapRenderer card={CARD_REGISTRY["sift_craap"]} values={vals}
    onField={(k, v) => { vals[k] = v; }} onExpandStep={() => {}} />);
  const currency = screen.getByLabelText(/Currency/);   // 滑杆/可点的评分控件，带 aria-label
  await userEvent.click(currency);
  expect(["currency"]).toContain(Object.keys(vals).find((k) => k === "currency"));
});
```
> 若用 `RepeatableGroupField` 复用，sources 行内字段 onChange 已经走 `onField("sources", nextArray)`，key 天然合规。重点是别引入计划外的 key。

- [ ] **Step 2: 跑测试确认 RED**。

- [ ] **Step 3: 实现** — 渲染五个分区，每区配 `SiftIcons` 图标 + 标题；
  - Stop：复用 `TextAreaField`（label 取自 JSON `stop`），`onChange={(v)=>onField("stop",v)}`。
  - Investigate：复用 `RepeatableGroupField`（field 取 `card.steps[0].fields` 里 key=`sources` 的那个），`onChange={(v)=>onField("sources",v)}`；其上方可放说明文案。
  - Find：`TextAreaField`（`better`）。Trace：`LinkCheckField`（`trace`）。
  - CRAAP（on_demand 步）：五维用 `Radar` 展示 + 每维一个可点评分（1–5），点击写 `onField("currency"|...|"purpose", n)`；评分控件加 `aria-label`（如「Currency 时效性」）。
  从 `card.steps` 按 key 取字段定义，避免硬编码 label。文件末 `export function SiftCraapRenderer(props: CardBodyProps) {...}`；在 `customRenderers.ts` 的对象里加 `"sift_craap": SiftCraapRenderer`。

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/cards/renderers/SiftCraapRenderer.tsx apps/web/src/cards/customRenderers.ts apps/web/src/cards/renderers/SiftCraapRenderer.test.tsx
git commit -m "feat(cards): SIFT×CRAAP custom fill renderer (icons + CRAAP radar), standard envelope guardrail"
```

---

### Task 16: #3 — 内在小人 自定义填写渲染器

**Files:**
- Create: `apps/web/src/cards/renderers/InnerPartsRenderer.tsx`
- Modify: `apps/web/src/cards/customRenderers.ts`（注册 `emotional-alignment`）
- Test: `apps/web/src/cards/renderers/InnerPartsRenderer.test.tsx`

**Interfaces:**
- Consumes: `CardBodyProps`、资产 `Battery`/`InnerPartsCast`、既有 `TextAreaField`。
- Produces: `InnerPartsRenderer: ComponentType<CardBodyProps>`。只写既有 key：`energy_level`(rating 1–5)、`inner_part_choice`(single_choice，值为 JSON 选项**完整字符串**)、`not_want_reason`(textarea)、`micro_action_choice`(single_choice)。

- [ ] **Step 1: 写护栏失败测试**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InnerPartsRenderer } from "./InnerPartsRenderer";
import { pickCardBody } from "../customRenderers";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

const ALLOWED = new Set(["energy_level","inner_part_choice","not_want_reason","micro_action_choice"]);

it("is registered for emotional-alignment", () => {
  expect(pickCardBody("emotional-alignment")).toBe(InnerPartsRenderer);
});

it("character cards write the exact JSON option string into inner_part_choice", async () => {
  const vals: Record<string, unknown> = {};
  render(<InnerPartsRenderer card={CARD_REGISTRY["emotional-alignment"]} values={vals}
    onField={(k, v) => { vals[k] = v; }} onExpandStep={() => {}} />);
  await userEvent.click(screen.getByText("保护者"));
  expect(vals["inner_part_choice"]).toBe("保护者——「别去冒险，可能会受伤」");  // 完整选项串
});

it("writes only schema keys (guardrail)", async () => {
  const writes: string[] = [];
  render(<InnerPartsRenderer card={CARD_REGISTRY["emotional-alignment"]} values={{}}
    onField={(k) => writes.push(k)} onExpandStep={() => {}} />);
  await userEvent.click(screen.getByLabelText(/电量/));   // 电量控件
  for (const k of writes) expect(ALLOWED.has(k)).toBe(true);
});
```

- [ ] **Step 2: 跑测试确认 RED**。

- [ ] **Step 3: 实现** — 四区：
  - 电量：`Battery` 控件，点/拖第 n 格 → `onField("energy_level", n)`；控件加 `aria-label="电量"`。
  - 小人选择：遍历 `card.steps` 里 key=`inner_part_choice` 字段的 `options`，与 `INNER_PARTS` 对应渲染角色卡（用 option 字符串里「——」前的名字匹配 `INNER_PARTS[i].name`，或直接按顺序对齐）；点击写 `onField("inner_part_choice", option)`（**整条 option 字符串**），高亮选中。
  - 安全岛：`TextAreaField`（`not_want_reason`，强调可留空）。
  - 小行动：该字段 `options` 两项渲染两枚按钮，点击写 `onField("micro_action_choice", option)`。
  从 `card.steps` 取字段定义；注册进 `customRenderers.ts`。

- [ ] **Step 4: 跑测试确认 GREEN**；`pnpm -r test` 全绿。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/cards/renderers/InnerPartsRenderer.tsx apps/web/src/cards/customRenderers.ts apps/web/src/cards/renderers/InnerPartsRenderer.test.tsx
git commit -m "feat(cards): 内在小人 custom fill renderer (battery + character cards), standard envelope guardrail"
```

---

## 收尾（全部任务后）

- [ ] 跑全量门禁：`pnpm -r typecheck && pnpm -r test` 全绿。
- [ ] 手动 smoke（可选）：`?demo` 画廊里打开 sift_craap / emotional-alignment，确认「给我讲讲这个」弹窗 5 章/3 章可走、填写控件可用、提交后过程树生长；旧卡（如 concession）方法面板与填写不变。
- [ ] 更新 carry-forward 追踪（`docs/遗留项追踪_Carryforward.md`）与记忆 `card-architecture-layers.md`：记下教学弹窗 + 两张富卡已落地、按 card_id 逃生舱、close≠skip、文字+卡同回合、更主动 prompt。

## 自查（写完计划对照 spec）

- **覆盖：** #1→T1；#2→T2+T3；#4→T4；#5→T5+T6+T7；#3 资产→T8、hideMethodology→T9、注册表/类型→T10、壳→T11、两 module→T12/T13、卡内集成→T14、两自定义填写→T15/T16。无缺口。
- **类型一致：** `CardBodyProps.hideMethodology?`（T9）被 T14/T15/T16 使用；`TeachingModule`/`pickTeaching`（T10）被 T11/T12/T13/T14 使用；`conversation.closeCard`（T2）被 T3 使用；资产具名导出（T8）被 T12/T13/T15/T16 使用——签名前后一致。
- **信封安全：** 自定义填写仅经 `onField` 写既有 key（T15/T16 护栏测试钉死）；教学练习不写信封，仅 `note_open`（T14）。

import { describe, it, expect, vi } from "vitest";
import { CARD_REGISTRY, deriveCatalog } from "@mind-imprint/contracts";
import type { ChatRequest, ChatResult } from "../llm/types";
import { LlmError } from "../llm/LlmError";
import { createStore } from "../store/createStore";
import { makeMemoryStorage } from "../store/storage";
import { demoCatalog } from "./prompt";
import { createConversation } from "./createConversation";

// ── helpers ────────────────────────────────────────────────────────────────────

type FakeChat = {
  fn: (config: object, req: ChatRequest) => Promise<ChatResult>;
  calls: ChatRequest[];
};

function makeFakeChat(script: ChatResult[]): FakeChat {
  let idx = 0;
  const calls: ChatRequest[] = [];
  const fn = async (_config: object, req: ChatRequest): Promise<ChatResult> => {
    calls.push(req);
    const result = script[idx++];
    if (result === undefined) throw new Error(`makeFakeChat: out of scripted results (call ${idx})`);
    return result;
  };
  return { fn, calls };
};

function makeStore() {
  return createStore({
    storage: makeMemoryStorage(),
    now: () => "2026-01-01T00:00:00.000Z",
    genId: (() => { let n = 0; return () => `id_${++n}`; })(),
  });
}

const registry = CARD_REGISTRY;
const catalog = demoCatalog(deriveCatalog(registry));

const VALID_CARD_ID = "sift_craap"; // known in registry

function makeSummonCardResult(cardId = VALID_CARD_ID): ChatResult {
  return {
    text: "来核实一下来源？",
    toolCalls: [
      {
        id: "tc_1",
        name: "summon_card",
        args: { card_id: cardId, reason: "信息素养", nudge_text: "来核实一下来源？" },
      },
    ],
    stopReason: "tool_call",
  };
}

function makeTextResult(text = "继续加油！"): ChatResult {
  return { text, stopReason: "stop" };
}

// ── test setup helpers ─────────────────────────────────────────────────────────

function makeConv(store: ReturnType<typeof makeStore>, fakeChat: FakeChat) {
  const task = store.createTask({ title: "test task", seed: null });
  let nowIdx = 0;
  const conv = createConversation({
    store,
    chat: fakeChat.fn as any,
    config: { format: "anthropic", baseUrl: "http://localhost", model: "test", apiKey: "k" },
    registry,
    catalog,
    taskId: task.id,
    now: () => `2026-01-01T0${nowIdx++}:00:00.000Z`,
    genId: (() => { let n = 0; return () => `gen_${++n}`; })(),
  });
  return { conv, taskId: task.id };
}

// ── tests ──────────────────────────────────────────────────────────────────────

describe("createConversation", () => {
  it("send → proposal: store has user + assistant proposal messages, proposed CardInstance, phase=proposal_pending", async () => {
    const store = makeStore();
    const fakeChat = makeFakeChat([makeSummonCardResult()]);
    const { conv, taskId } = makeConv(store, fakeChat);

    await conv.send("我看到一篇关于中国可持续性的文章");

    const state = conv.getSnapshot();
    expect(state.phase).toBe("proposal_pending");
    expect(state.pendingCardId).toBeDefined();

    const messages = store.listMessages(taskId);
    expect(messages).toHaveLength(2);

    const [userMsg, assistantMsg] = messages;
    expect(userMsg!.role).toBe("user");
    expect(userMsg!.content).toBe("我看到一篇关于中国可持续性的文章");

    expect(assistantMsg!.role).toBe("assistant");
    expect(assistantMsg!.content).toBe("来核实一下来源？");
    expect(assistantMsg!.tool_call).not.toBeNull();

    // validate tool_call is a valid SummonCardCall
    const tc = assistantMsg!.tool_call as any;
    expect(tc.name).toBe("summon_card");
    expect(tc.id).toBe("tc_1");
    expect(tc.args.card_id).toBe(VALID_CARD_ID);
    expect(tc.card_instance_id).toBe(state.pendingCardId);

    // Proposed CardInstance in store
    const ci = store.getCard(state.pendingCardId!);
    expect(ci).toBeDefined();
    expect(ci!.status).toBe("proposed");
    expect(ci!.card_id).toBe(VALID_CARD_ID);
    expect(ci!.task_id).toBe(taskId);
  });

  it("openCard → phase=card_active, instance status=active", async () => {
    const store = makeStore();
    const fakeChat = makeFakeChat([makeSummonCardResult()]);
    const { conv } = makeConv(store, fakeChat);

    await conv.send("test");

    const { pendingCardId } = conv.getSnapshot();
    expect(pendingCardId).toBeDefined();

    conv.openCard(pendingCardId!);

    const state = conv.getSnapshot();
    expect(state.phase).toBe("card_active");

    const ci = store.getCard(pendingCardId!);
    expect(ci!.status).toBe("active");
  });

  it("submitCard → instance completed, second chat receives tool result with status=completed, follow-up appended, phase=idle", async () => {
    const store = makeStore();
    const followUpText = "很好，继续写论证！";
    const fakeChat = makeFakeChat([makeSummonCardResult(), makeTextResult(followUpText)]);
    const { conv, taskId } = makeConv(store, fakeChat);

    await conv.send("test");
    const { pendingCardId } = conv.getSnapshot();
    conv.openCard(pendingCardId!);

    // Build a completed instance
    const originalCi = store.getCard(pendingCardId!)!;
    const completedCi = {
      ...originalCi,
      status: "completed" as const,
      completed_at: "2026-01-01T01:00:00.000Z",
      field_values: { sift: { stop: "found_NASA" } },
    };

    await conv.submitCard(pendingCardId!, completedCi);

    const state = conv.getSnapshot();
    expect(state.phase).toBe("idle");

    // Instance is completed in store
    const ci = store.getCard(pendingCardId!);
    expect(ci!.status).toBe("completed");

    // Second chat call received messages with tool result
    const secondReq = fakeChat.calls[1]!;
    const toolMsg = secondReq.messages.find((m) => m.role === "tool");
    expect(toolMsg).toBeDefined();
    const payload = JSON.parse(toolMsg!.content);
    expect(payload.status).toBe("completed");
    expect(toolMsg!.toolCallId).toBe("tc_1");

    // Follow-up message appended
    const messages = store.listMessages(taskId);
    const lastMsg = messages[messages.length - 1]!;
    expect(lastMsg.role).toBe("assistant");
    expect(lastMsg.content).toBe(followUpText);
    expect(lastMsg.tool_call).toBeNull();
  });

  it("skipCard → instance skipped, tool result status=skipped, follow-up appended, phase=idle", async () => {
    const store = makeStore();
    const followUpText = "好的，我们继续吧！";
    const fakeChat = makeFakeChat([makeSummonCardResult(), makeTextResult(followUpText)]);
    const { conv, taskId } = makeConv(store, fakeChat);

    await conv.send("test");
    const { pendingCardId } = conv.getSnapshot();

    await conv.skipCard(pendingCardId!);

    const state = conv.getSnapshot();
    expect(state.phase).toBe("idle");

    // Instance is skipped
    const ci = store.getCard(pendingCardId!);
    expect(ci!.status).toBe("skipped");

    // Second chat has tool result with status=skipped
    const secondReq = fakeChat.calls[1]!;
    const toolMsg = secondReq.messages.find((m) => m.role === "tool");
    expect(toolMsg).toBeDefined();
    const payload = JSON.parse(toolMsg!.content);
    expect(payload.status).toBe("skipped");

    // Follow-up appended
    const messages = store.listMessages(taskId);
    const lastMsg = messages[messages.length - 1]!;
    expect(lastMsg.role).toBe("assistant");
    expect(lastMsg.content).toBe(followUpText);
  });

  it("plain text reply (no toolCalls) → no CardInstance created, phase=idle", async () => {
    const store = makeStore();
    const fakeChat = makeFakeChat([makeTextResult("试着想一想...")]);
    const { conv, taskId } = makeConv(store, fakeChat);

    await conv.send("我的问题是什么？");

    const state = conv.getSnapshot();
    expect(state.phase).toBe("idle");
    expect(state.pendingCardId).toBeUndefined();

    const cards = store.listCards(taskId);
    expect(cards).toHaveLength(0);

    const messages = store.listMessages(taskId);
    const lastMsg = messages[messages.length - 1]!;
    expect(lastMsg.role).toBe("assistant");
    expect(lastMsg.content).toBe("试着想一想...");
    expect(lastMsg.tool_call).toBeNull();
  });

  it("multiple toolCalls → only first summon_card is used (one CardInstance)", async () => {
    const store = makeStore();
    const multiResult: ChatResult = {
      text: "核实来源",
      toolCalls: [
        { id: "tc_1", name: "summon_card", args: { card_id: VALID_CARD_ID, reason: "r1", nudge_text: "n1" } },
        { id: "tc_2", name: "summon_card", args: { card_id: "concession", reason: "r2", nudge_text: "n2" } },
      ],
      stopReason: "tool_call",
    };
    const consoleWarn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const fakeChat = makeFakeChat([multiResult]);
    const { conv, taskId } = makeConv(store, fakeChat);

    await conv.send("test");

    const cards = store.listCards(taskId);
    expect(cards).toHaveLength(1);
    expect(cards[0]!.card_id).toBe(VALID_CARD_ID);

    consoleWarn.mockRestore();
  });

  it("unknown card_id in toolCall → no CardInstance, treated as plain text, phase=idle", async () => {
    const store = makeStore();
    const unknownCardResult: ChatResult = {
      text: "test text",
      toolCalls: [
        { id: "tc_x", name: "summon_card", args: { card_id: "nonexistent_card_xyz", reason: "r", nudge_text: "n" } },
      ],
      stopReason: "tool_call",
    };
    const fakeChat = makeFakeChat([unknownCardResult]);
    const { conv, taskId } = makeConv(store, fakeChat);

    await conv.send("test");

    const state = conv.getSnapshot();
    expect(state.phase).toBe("idle");
    expect(state.pendingCardId).toBeUndefined();

    const cards = store.listCards(taskId);
    expect(cards).toHaveLength(0);

    // Still appends a text message
    const messages = store.listMessages(taskId);
    const lastMsg = messages[messages.length - 1]!;
    expect(lastMsg.role).toBe("assistant");
    expect(lastMsg.tool_call).toBeNull();
  });

  it("chat throws LlmError → phase=error, error message set, nothing thrown", async () => {
    const store = makeStore();
    const fakeChat: FakeChat = {
      fn: async () => { throw new LlmError("API 超时", { status: 408 }); },
      calls: [],
    };
    const { conv } = makeConv(store, fakeChat);

    await expect(conv.send("test")).resolves.toBeUndefined();

    const state = conv.getSnapshot();
    expect(state.phase).toBe("error");
    expect(state.error).toContain("API 超时");
  });

  it("re-feed throws LlmError after submitCard → phase=error, nothing thrown", async () => {
    const store = makeStore();
    // First call returns a summon_card proposal; second call (re-feed) throws LlmError
    let callCount = 0;
    const fakeChat: FakeChat = {
      fn: async (_config: object, req: ChatRequest): Promise<ChatResult> => {
        callCount++;
        if (callCount === 1) return makeSummonCardResult();
        throw new LlmError("re-feed 超时", { status: 503 });
      },
      calls: [],
    };
    const { conv } = makeConv(store, fakeChat);

    await conv.send("test");
    const { pendingCardId } = conv.getSnapshot();
    expect(pendingCardId).toBeDefined();
    conv.openCard(pendingCardId!);

    const originalCi = store.getCard(pendingCardId!)!;
    const completedCi = {
      ...originalCi,
      status: "completed" as const,
      completed_at: "2026-01-01T01:00:00.000Z",
      field_values: {},
    };

    // submitCard must resolve (not throw) even when re-feed errors
    await expect(conv.submitCard(pendingCardId!, completedCi)).resolves.toBeUndefined();

    const state = conv.getSnapshot();
    expect(state.phase).toBe("error");
    expect(state.error).toContain("re-feed 超时");
  });

  it("re-feed throws LlmError after skipCard → phase=error, nothing thrown", async () => {
    const store = makeStore();
    let callCount = 0;
    const fakeChat: FakeChat = {
      fn: async (_config: object, req: ChatRequest): Promise<ChatResult> => {
        callCount++;
        if (callCount === 1) return makeSummonCardResult();
        throw new LlmError("skip re-feed 失败", { status: 500 });
      },
      calls: [],
    };
    const { conv } = makeConv(store, fakeChat);

    await conv.send("test");
    const { pendingCardId } = conv.getSnapshot();
    expect(pendingCardId).toBeDefined();

    // skipCard must resolve (not throw) even when re-feed errors
    await expect(conv.skipCard(pendingCardId!)).resolves.toBeUndefined();

    const state = conv.getSnapshot();
    expect(state.phase).toBe("error");
    expect(state.error).toContain("skip re-feed 失败");
  });

  it("subscribe notifies on state change", async () => {
    const store = makeStore();
    const fakeChat = makeFakeChat([makeTextResult("好")]);
    const { conv } = makeConv(store, fakeChat);

    const snapshots: string[] = [];
    const unsub = conv.subscribe(() => {
      snapshots.push(conv.getSnapshot().phase);
    });

    await conv.send("hello");
    unsub();

    // Should have transitioned: awaiting_llm → idle
    expect(snapshots).toContain("awaiting_llm");
    expect(snapshots).toContain("idle");
  });
});

import { describe, it, expect } from "vitest";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { CardInstance } from "@mind-imprint/contracts";
import type { ChatRequest, ChatResult } from "../llm/types";
import { createStore } from "../store/createStore";
import { makeMemoryStorage } from "../store/storage";
import { runEvaluation } from "./runEvaluation";

// ── helpers ────────────────────────────────────────────────────────────────────

type FakeChat = {
  fn: (config: object, req: ChatRequest) => Promise<ChatResult>;
  calls: { config: object; req: ChatRequest }[];
};

function makeFakeChat(script: (ChatResult | "malformed")[]): FakeChat {
  let idx = 0;
  const calls: { config: object; req: ChatRequest }[] = [];
  const fn = async (config: object, req: ChatRequest): Promise<ChatResult> => {
    calls.push({ config, req });
    const result = script[idx++];
    if (result === undefined) throw new Error(`makeFakeChat: out of scripted results (call ${idx})`);
    if (result === "malformed") {
      return { text: "not json", stopReason: "stop" };
    }
    return result;
  };
  return { fn, calls };
}

function makeValidOutput() {
  return JSON.stringify({
    scores: [
      { dim_id: "D2", level: "L4", note: "Phoebe 溯源到 NASA" },
      { dim_id: "D3", level: "L4", note: "横向验证完成" },
      { dim_id: "D4", level: "L3", note: "多视角处理" },
      { dim_id: "D5", level: "L2", note: "论证拆解" },
      { dim_id: "D6", level: "L3", note: "元认知反思" },
    ],
    narrative: "Phoebe 展现出良好的来源意识，下一步建议深挖隐含假设。",
  });
}

function makeValidChatResult(): ChatResult {
  return { text: makeValidOutput(), stopReason: "stop" };
}

function makeStore() {
  return createStore({
    storage: makeMemoryStorage(),
    now: () => "2026-01-01T00:00:00.000Z",
    genId: (() => { let n = 0; return () => `id_${++n}`; })(),
  });
}

function makeCompletedSiftCard(taskId: string): CardInstance {
  return {
    id: "ci_sift_1",
    card_id: "sift_craap",
    task_id: taskId,
    parent_node_id: null,
    status: "completed",
    field_values: {
      sift: {
        stop: "想引用文章说明中国可再生能源领先",
        sources: [{ name: "NASA Earth Observatory", type: "官方", verdict: "可信" }],
        better: "NASA 数据可信",
        trace: "https://earthobservatory.nasa.gov",
      },
    },
    event_trace: [
      { kind: "step_expand", step_key: "sift", at: "2026-01-01T00:01:00.000Z" },
      { kind: "submit", at: "2026-01-01T00:02:00.000Z" },
    ],
    rubric_tags: ["D1_来源意识"],
    created_at: "2026-01-01T00:00:00.000Z",
    completed_at: "2026-01-01T00:02:00.000Z",
  };
}

function makeDeps(store: ReturnType<typeof makeStore>, fakeChat: FakeChat, taskId: string) {
  return {
    store,
    chat: fakeChat.fn as any,
    config: {
      format: "anthropic" as const,
      baseUrl: "http://localhost",
      model: "claude-3-sonnet",
      apiKey: "sk-test-key",
      evalModel: "claude-opus-4",
    },
    registry: CARD_REGISTRY,
    taskId,
    now: () => "2026-01-01T10:00:00.000Z",
  };
}

// ── tests ──────────────────────────────────────────────────────────────────────

describe("runEvaluation", () => {
  it("valid JSON output → returns an Evaluation, persisted in store, chat called with evalModel", async () => {
    const store = makeStore();
    const task = store.createTask({ title: "Phoebe task", seed: null });
    const taskId = task.id;

    // Seed messages
    store.appendMessage({ task_id: taskId, role: "user", content: "我想研究中国可持续性" });
    store.appendMessage({ task_id: taskId, role: "assistant", content: "来核实一下来源？" });

    // Seed completed card
    store.putCard(makeCompletedSiftCard(taskId));

    const fakeChat = makeFakeChat([makeValidChatResult()]);
    const deps = makeDeps(store, fakeChat, taskId);

    const evaluation = await runEvaluation(deps);

    // Returns an Evaluation with correct shape
    expect(evaluation.task_id).toBe(taskId);
    expect(evaluation.scores).toHaveLength(5);
    expect(evaluation.scores[0]!.dim_id).toBe("D2");
    expect(evaluation.scores[0]!.level).toBe("L4");
    expect(evaluation.narrative).toContain("Phoebe");
    expect(evaluation.created_at).toBe("2026-01-01T10:00:00.000Z");

    // Persisted in store
    const persisted = store.getLatestEvaluation(taskId);
    expect(persisted).toBeDefined();
    expect(persisted).toEqual(evaluation);

    // Chat was called exactly once with evalModel
    expect(fakeChat.calls).toHaveLength(1);
    const call = fakeChat.calls[0]!;
    expect((call.config as any).model).toBe("claude-opus-4");

    // System message contains a rubric dimension name
    const sysMsg = call.req.messages.find((m) => m.role === "system");
    expect(sysMsg).toBeDefined();
    expect(sysMsg!.content).toContain("信源辨识"); // D2 name from FULL_RUBRIC

    // User message contains a card name from the registry
    const userMsg = call.req.messages.find((m) => m.role === "user");
    expect(userMsg).toBeDefined();
    expect(userMsg!.content).toContain("SIFT×CRAAP"); // card name from registry
  });

  it("first response malformed, second valid → succeeds with one retry (chat called twice)", async () => {
    const store = makeStore();
    const task = store.createTask({ title: "retry task", seed: null });
    const taskId = task.id;

    store.appendMessage({ task_id: taskId, role: "user", content: "test" });
    store.putCard(makeCompletedSiftCard(taskId));

    const fakeChat = makeFakeChat(["malformed", makeValidChatResult()]);
    const deps = makeDeps(store, fakeChat, taskId);

    const evaluation = await runEvaluation(deps);

    // Succeeds despite first failure
    expect(evaluation.task_id).toBe(taskId);
    expect(evaluation.scores).toHaveLength(5);
    expect(evaluation.narrative).toBeTruthy();

    // Chat was called exactly twice (original + one retry)
    expect(fakeChat.calls).toHaveLength(2);

    // Persisted
    const persisted = store.getLatestEvaluation(taskId);
    expect(persisted).toEqual(evaluation);
  });

  it("both responses malformed → throws 评估输出解析失败", async () => {
    const store = makeStore();
    const task = store.createTask({ title: "fail task", seed: null });
    const taskId = task.id;

    store.appendMessage({ task_id: taskId, role: "user", content: "test" });

    const fakeChat = makeFakeChat(["malformed", "malformed"]);
    const deps = makeDeps(store, fakeChat, taskId);

    await expect(runEvaluation(deps)).rejects.toThrow("评估输出解析失败");

    // Chat was called exactly twice
    expect(fakeChat.calls).toHaveLength(2);

    // Nothing persisted
    const persisted = store.getLatestEvaluation(taskId);
    expect(persisted).toBeUndefined();
  });

  it("json wrapped in ```json fence → parsed correctly", async () => {
    const store = makeStore();
    const task = store.createTask({ title: "fence task", seed: null });
    const taskId = task.id;

    store.appendMessage({ task_id: taskId, role: "user", content: "test" });

    const fencedOutput = "```json\n" + makeValidOutput() + "\n```";
    const fakeChat = makeFakeChat([{ text: fencedOutput, stopReason: "stop" }]);
    const deps = makeDeps(store, fakeChat, taskId);

    const evaluation = await runEvaluation(deps);

    expect(evaluation.scores).toHaveLength(5);
    expect(evaluation.narrative).toContain("Phoebe");
  });

  it("uses now() for created_at, and defaults to new Date() when not provided", async () => {
    const store = makeStore();
    const task = store.createTask({ title: "now task", seed: null });
    const taskId = task.id;

    store.appendMessage({ task_id: taskId, role: "user", content: "test" });

    const fakeChat = makeFakeChat([makeValidChatResult()]);
    const { now: _now, ...depsWithoutNow } = makeDeps(store, fakeChat, taskId);

    const before = new Date().toISOString();
    const evaluation = await runEvaluation(depsWithoutNow);
    const after = new Date().toISOString();

    // created_at should be between before and after
    expect(evaluation.created_at >= before).toBe(true);
    expect(evaluation.created_at <= after).toBe(true);
  });

  it("uses config.model when evalModel is not set", async () => {
    const store = makeStore();
    const task = store.createTask({ title: "model fallback", seed: null });
    const taskId = task.id;

    store.appendMessage({ task_id: taskId, role: "user", content: "test" });

    const fakeChat = makeFakeChat([makeValidChatResult()]);
    const deps = {
      store,
      chat: fakeChat.fn as any,
      config: {
        format: "anthropic" as const,
        baseUrl: "http://localhost",
        model: "claude-sonnet-base",
        apiKey: "sk-test",
        // no evalModel
      },
      registry: CARD_REGISTRY,
      taskId,
      now: () => "2026-01-01T10:00:00.000Z",
    };

    await runEvaluation(deps);

    expect((fakeChat.calls[0]!.config as any).model).toBe("claude-sonnet-base");
  });

  it("sets a generous maxTokens so reasoning models don't truncate the eval JSON", async () => {
    const store = makeStore();
    const task = store.createTask({ title: "maxTokens test", seed: null });
    const taskId = task.id;

    store.appendMessage({ task_id: taskId, role: "user", content: "test" });

    const fakeChat = makeFakeChat([makeValidChatResult()]);
    const deps = makeDeps(store, fakeChat, taskId);

    await runEvaluation(deps);

    expect(fakeChat.calls[0]!.req.maxTokens).toBe(8000);
  });
});

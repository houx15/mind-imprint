import { describe, it, expect } from "vitest";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { ChatRequest, ChatResult } from "../llm/types";
import { createStore } from "../store/createStore";
import { makeMemoryStorage } from "../store/storage";
import { createEvaluator } from "./createEvaluator";

// ── helpers ────────────────────────────────────────────────────────────────────

function makeStore() {
  return createStore({
    storage: makeMemoryStorage(),
    now: () => "2026-01-01T00:00:00.000Z",
    genId: (() => {
      let n = 0;
      return () => `id_${++n}`;
    })(),
  });
}

const registry = CARD_REGISTRY;

const FAKE_EVAL_RESPONSE = JSON.stringify({
  scores: [
    { dim_id: "D2", level: "L3", note: "主动要求论据" },
    { dim_id: "D3", level: "L2", note: "想到要多看" },
    { dim_id: "D4", level: "L3", note: "主动找反方" },
    { dim_id: "D5", level: "L2", note: "能复述但不辨结构" },
    { dim_id: "D6", level: "L2", note: "事后偶尔回顾" },
  ],
  narrative: "学生展示了良好的思维过程。",
});

function makeFakeChat(): (config: object, req: ChatRequest) => Promise<ChatResult> {
  return async (_config: object, _req: ChatRequest): Promise<ChatResult> => {
    return { text: FAKE_EVAL_RESPONSE, stopReason: "stop" };
  };
}

function makeThrowingChat(message: string): (config: object, req: ChatRequest) => Promise<ChatResult> {
  return async (_config: object, _req: ChatRequest): Promise<ChatResult> => {
    throw new Error(message);
  };
}

function makeEvaluator(chat: (config: object, req: ChatRequest) => Promise<ChatResult>) {
  const store = makeStore();
  const task = store.createTask({ title: "test task", seed: null });
  // Add a minimal message so assembleEvalInput has something to work with
  store.appendMessage({ task_id: task.id, role: "user", content: "测试消息" });

  return {
    ev: createEvaluator({
      store,
      chat: chat as any,
      config: { format: "anthropic", baseUrl: "http://localhost", model: "test", apiKey: "k" },
      registry,
      taskId: task.id,
      now: () => "2026-01-01T00:00:00.000Z",
    }),
    store,
    taskId: task.id,
  };
}

// ── tests ──────────────────────────────────────────────────────────────────────

describe("createEvaluator", () => {
  it("initial phase is idle", () => {
    const { ev } = makeEvaluator(makeFakeChat());
    expect(ev.getSnapshot().phase).toBe("idle");
  });

  it("run() transitions running then done with evaluation set", async () => {
    const { ev } = makeEvaluator(makeFakeChat());

    const phases: string[] = [];
    ev.subscribe(() => {
      phases.push(ev.getSnapshot().phase);
    });

    await ev.run();

    const state = ev.getSnapshot();
    expect(state.phase).toBe("done");
    expect(state.evaluation).toBeDefined();
    expect(state.evaluation!.task_id).toBeDefined();
    expect(state.evaluation!.narrative).toBe("学生展示了良好的思维过程。");

    // Should have seen running then done
    expect(phases).toContain("running");
    expect(phases).toContain("done");
  });

  it("run() with throwing chat → phase=error, error message set, run() resolves (does not throw)", async () => {
    const { ev } = makeEvaluator(makeThrowingChat("chat 崩了"));

    await expect(ev.run()).resolves.toBeUndefined();

    const state = ev.getSnapshot();
    expect(state.phase).toBe("error");
    expect(state.error).toContain("chat 崩了");
    expect(state.evaluation).toBeUndefined();
  });

  it("subscriber fires on each phase change", async () => {
    const { ev } = makeEvaluator(makeFakeChat());

    const snapshots: string[] = [];
    const unsub = ev.subscribe(() => {
      snapshots.push(ev.getSnapshot().phase);
    });

    await ev.run();
    unsub();

    expect(snapshots).toContain("running");
    expect(snapshots).toContain("done");
  });

  it("unsubscribe stops notifications", async () => {
    const { ev } = makeEvaluator(makeFakeChat());

    let callCount = 0;
    const unsub = ev.subscribe(() => {
      callCount++;
    });
    unsub();

    await ev.run();

    expect(callCount).toBe(0);
  });

  it("getSnapshot() returns stable reference when nothing changed (idle, no action)", () => {
    const { ev } = makeEvaluator(makeFakeChat());

    const snap1 = ev.getSnapshot();
    const snap2 = ev.getSnapshot();
    expect(snap1).toBe(snap2);
  });

  it("getSnapshot() returns stable reference between two reads after run() settles (no second run)", async () => {
    const { ev } = makeEvaluator(makeFakeChat());

    await ev.run();
    // Nothing changed between these two reads — same object reference is required
    // for useSyncExternalStore stable-snapshot contract
    const snap1 = ev.getSnapshot();
    const snap2 = ev.getSnapshot();
    expect(snap1).toBe(snap2);
    expect(snap1.phase).toBe("done");
  });

  it("getSnapshot() returns new reference after a transition", async () => {
    const { ev } = makeEvaluator(makeFakeChat());

    const snapBefore = ev.getSnapshot();
    await ev.run();
    const snapAfter = ev.getSnapshot();

    expect(snapBefore).not.toBe(snapAfter);
  });

  it("run() sets error.message for non-Error throws", async () => {
    const store = makeStore();
    const task = store.createTask({ title: "test task", seed: null });
    store.appendMessage({ task_id: task.id, role: "user", content: "test" });

    const ev = createEvaluator({
      store,
      chat: (async () => { throw "string error"; }) as any,
      config: { format: "anthropic", baseUrl: "http://localhost", model: "test", apiKey: "k" },
      registry,
      taskId: task.id,
      now: () => "2026-01-01T00:00:00.000Z",
    });

    await ev.run();

    const state = ev.getSnapshot();
    expect(state.phase).toBe("error");
    expect(state.error).toBe("string error");
  });

  it("evaluation is stored in the store after successful run", async () => {
    const { ev, store, taskId } = makeEvaluator(makeFakeChat());

    await ev.run();

    const stored = store.getLatestEvaluation(taskId);
    expect(stored).toBeDefined();
    expect(stored!.task_id).toBe(taskId);
  });
});

import { describe, it, expect } from "vitest";
import { createStore } from "../store/createStore";
import { createConversation } from "./createConversation";
import type { TurnEvent } from "../api";

function fakeApi(script: TurnEvent[]) {
  const calls: { userInput?: string }[] = [];
  return {
    calls,
    async *runTurn(_t: string, userInput?: string) { calls.push({ userInput }); for (const e of script) yield e; },
    async activateCard(_t: string, _c: string) { return undefined as never; },
    async submitCard(_t: string, _c: string, env: any) { return env; },
    async skipCard(_t: string, _c: string, _e: any) { return undefined as never; },
  };
}

const task = { id: "t1", title: "t", seed: null, status: "active" as const, created_at: "1", last_active_at: "1" };

describe("createConversation (API/SSE)", () => {
  it("send streams text into a single assistant message and ends idle", async () => {
    const store = createStore({}); store.putTask(task);
    const api = fakeApi([{ type: "text", delta: "He" }, { type: "text", delta: "llo" }, { type: "done", messageId: "m1" }]);
    const conv = createConversation({ api: api as never, store, taskId: "t1" });
    await conv.send("hi");
    const msgs = store.listMessages("t1");
    expect(msgs.map((m) => m.role)).toEqual(["user", "assistant"]);
    expect(msgs[1]!.content).toBe("Hello");
    expect(conv.getSnapshot().phase).toBe("idle");
    expect(api.calls[0]!.userInput).toBe("hi");
  });

  it("a card event creates a proposed card + proposal_pending and ends the turn", async () => {
    const store = createStore({}); store.putTask(task);
    const anchor = { id: "a0", material_id: "m0", block_id: "b0", start: 0, end: 3, quote: "原句", dimension: "权威性", author: "ai" as const, question: "可信吗？", answer: "" };
    const api = fakeApi([{ type: "text", delta: "先溯源" }, { type: "card", cardInstanceId: "c1", cardId: "sift_craap", nudgeText: "一起?", anchors: [anchor] }]);
    const conv = createConversation({ api: api as never, store, taskId: "t1" });
    await conv.send("引用公众号");
    expect(store.getCard("c1")!.status).toBe("proposed");
    // Anchors delivered on the card event are applied immediately (keystone renders on open).
    expect(store.getCard("c1")!.anchors).toHaveLength(1);
    expect(store.getCard("c1")!.anchors[0]!.question).toBe("可信吗？");
    expect(conv.getSnapshot()).toMatchObject({ phase: "proposal_pending", pendingCardId: "c1" });
    const assistant = store.listMessages("t1").find((m) => m.role === "assistant")!;
    expect((assistant.tool_call as any).card_instance_id).toBe("c1");
  });

  it("submitCard fires a continuation turn (no user input)", async () => {
    const store = createStore({}); store.putTask(task);
    store.putCard({ id: "c1", card_id: "sift_craap", task_id: "t1", parent_node_id: null, status: "active", field_values: {}, event_trace: [], rubric_tags: [], anchors: [], created_at: "1", completed_at: null });
    const api = fakeApi([{ type: "text", delta: "很好" }, { type: "done", messageId: "m2" }]);
    const conv = createConversation({ api: api as never, store, taskId: "t1" });
    const final = { ...store.getCard("c1")!, status: "completed" as const, field_values: { sift: { stop: "x" } } };
    await conv.submitCard("c1", final);
    expect(api.calls.at(-1)!.userInput).toBeUndefined(); // continuation turn
    expect(conv.getSnapshot().phase).toBe("idle");
  });

  it("an error event sets phase error with the safe message", async () => {
    const store = createStore({}); store.putTask(task);
    const api = fakeApi([{ type: "error", code: "not_entitled", message: "无额度" }]);
    const conv = createConversation({ api: api as never, store, taskId: "t1" });
    await conv.send("hi");
    expect(conv.getSnapshot()).toMatchObject({ phase: "error", error: "无额度" });
  });

  it("a mid-stream error removes the partial assistant message from the store", async () => {
    const store = createStore({}); store.putTask(task);
    const api = fakeApi([
      { type: "text", delta: "partial" },
      { type: "error", code: "internal_error", message: "boom" },
    ]);
    const conv = createConversation({ api: api as never, store, taskId: "t1" });
    await conv.send("hi");
    const msgs = store.listMessages("t1");
    expect(msgs.map((m) => m.role)).toEqual(["user"]);
    expect(msgs.find((m) => m.role === "assistant")).toBeUndefined();
    expect(conv.getSnapshot().phase).toBe("error");
  });
});

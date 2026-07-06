import { describe, it, expect } from "vitest";
import { createStore } from "./createStore";
import { makeMemoryStorage } from "./storage";
import type { CardInstance } from "@mind-imprint/contracts";

function fixedClock() {
  let n = 0;
  return () => `2026-06-21T10:00:0${n++}.000Z`;
}
function seqId(prefix: string) {
  let n = 0;
  return () => `${prefix}_${++n}`;
}
function freshStore() {
  return createStore({ storage: makeMemoryStorage(), now: fixedClock(), genId: seqId("x") });
}

function card(id: string, task_id: string): CardInstance {
  return {
    id, card_id: "sift_craap", task_id, parent_node_id: null,
    status: "proposed", field_values: {}, event_trace: [], rubric_tags: [], anchors: [],
    created_at: "2026-06-21T09:00:00.000Z", completed_at: null,
  };
}

describe("createStore — messages", () => {
  it("appends messages in insertion order, filtered by task", () => {
    const store = freshStore();
    const t = store.createTask({ title: "气候", seed: null });
    store.appendMessage({ task_id: t.id, role: "user", content: "一" });
    store.appendMessage({ task_id: t.id, role: "assistant", content: "二" });
    store.appendMessage({ task_id: "other", role: "user", content: "别的" });
    const msgs = store.listMessages(t.id);
    expect(msgs.map((m) => m.content)).toEqual(["一", "二"]);
  });

  it("defaults tool_call to null and bumps the parent task last_active_at", () => {
    const store = freshStore();
    const t = store.createTask({ title: "气候", seed: null });
    const before = store.getTask(t.id)!.last_active_at;
    const m = store.appendMessage({ task_id: t.id, role: "user", content: "x" });
    expect(m.tool_call).toBeNull();
    expect(store.getTask(t.id)!.last_active_at).not.toBe(before);
  });

  it("preserves a provided tool_call payload", () => {
    const store = freshStore();
    const t = store.createTask({ title: "气候", seed: null });
    const m = store.appendMessage({ task_id: t.id, role: "assistant", content: "", tool_call: { name: "summon_card" } });
    expect(m.tool_call).toEqual({ name: "summon_card" });
  });
});

describe("createStore — cards", () => {
  it("upserts a card by id without duplicating", () => {
    const store = freshStore();
    store.putCard(card("c_1", "t_1"));
    store.putCard({ ...card("c_1", "t_1"), status: "completed" });
    expect(store.listCards("t_1")).toHaveLength(1);
    expect(store.getCard("c_1")!.status).toBe("completed");
  });

  it("lists cards filtered by task", () => {
    const store = freshStore();
    store.putCard(card("c_1", "t_1"));
    store.putCard(card("c_2", "t_2"));
    expect(store.listCards("t_1").map((c) => c.id)).toEqual(["c_1"]);
  });
});

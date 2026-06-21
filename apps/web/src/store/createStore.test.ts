import { describe, it, expect, vi } from "vitest";
import { createStore } from "./createStore";
import { makeMemoryStorage, STORE_KEY } from "./storage";

function fixedClock() {
  let n = 0;
  return () => `2026-06-21T10:00:0${n++}.000Z`;
}
function seqId(prefix: string) {
  let n = 0;
  return () => `${prefix}_${++n}`;
}
function opts(storage = makeMemoryStorage()) {
  return { storage, now: fixedClock(), genId: seqId("t") };
}

describe("createStore — tasks", () => {
  it("creates a task with injected id, time, and active status", () => {
    const store = createStore(opts());
    const t = store.createTask({ title: "气候", seed: null });
    expect(t.id).toBe("t_1");
    expect(t.status).toBe("active");
    expect(t.created_at).toBe(t.last_active_at);
    expect(store.getTask("t_1")).toEqual(t);
    expect(store.listTasks()).toHaveLength(1);
  });

  it("updateTask applies the patch and bumps last_active_at", () => {
    const store = createStore(opts());
    const t = store.createTask({ title: "气候", seed: null });
    const updated = store.updateTask(t.id, { status: "evaluated" });
    expect(updated.status).toBe("evaluated");
    expect(updated.last_active_at).not.toBe(t.last_active_at);
  });

  it("updateTask throws on an unknown id", () => {
    const store = createStore(opts());
    expect(() => store.updateTask("nope", { title: "x" })).toThrow();
  });
});

describe("createStore — reactivity", () => {
  it("notifies subscribers on a write and changes the snapshot reference", () => {
    const store = createStore(opts());
    const before = store.getSnapshot();
    const listener = vi.fn();
    const unsub = store.subscribe(listener);
    store.createTask({ title: "气候", seed: null });
    expect(listener).toHaveBeenCalledTimes(1);
    expect(store.getSnapshot()).not.toBe(before);
    unsub();
    store.createTask({ title: "第二个", seed: null });
    expect(listener).toHaveBeenCalledTimes(1); // not called after unsubscribe
  });

  it("getSnapshot is stable across reads with no write between", () => {
    const store = createStore(opts());
    store.createTask({ title: "气候", seed: null });
    expect(store.getSnapshot()).toBe(store.getSnapshot());
  });
});

describe("createStore — persistence (刷新不丢)", () => {
  it("a fresh store over the same storage reads back the data", () => {
    const storage = makeMemoryStorage();
    const a = createStore({ storage, now: fixedClock(), genId: seqId("t") });
    a.createTask({ title: "气候", seed: "https://x" });
    const b = createStore({ storage });
    expect(b.listTasks()).toHaveLength(1);
    expect(b.listTasks()[0]!.title).toBe("气候");
  });
});

describe("createStore — load policy", () => {
  it("missing blob boots empty without warning", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const store = createStore(opts());
    expect(store.listTasks()).toHaveLength(0);
    expect(warn).not.toHaveBeenCalled();
    warn.mockRestore();
  });

  it("non-JSON blob boots empty with a warning", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const storage = makeMemoryStorage();
    storage.setItem(STORE_KEY, "{not json");
    const store = createStore({ storage });
    expect(store.listTasks()).toHaveLength(0);
    expect(warn).toHaveBeenCalledTimes(1);
    warn.mockRestore();
  });

  it("schema-invalid blob boots empty with a warning", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const storage = makeMemoryStorage();
    storage.setItem(STORE_KEY, JSON.stringify({ version: 1, tasks: [{ bad: true }], messages: [], cards: [] }));
    const store = createStore({ storage });
    expect(store.listTasks()).toHaveLength(0);
    expect(warn).toHaveBeenCalledTimes(1);
    warn.mockRestore();
  });
});

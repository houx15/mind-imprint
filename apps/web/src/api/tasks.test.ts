import { describe, it, expect, vi } from "vitest";
import { listTasks, createTask, getTask } from "./tasks";

const ok = (body: unknown) => vi.fn(async () => new Response(JSON.stringify(body), { status: 200 }));

describe("tasks api", () => {
  it("listTasks unwraps {tasks}", async () => {
    vi.stubGlobal("fetch", ok({ tasks: [{ id: "t1" }] }));
    expect(await listTasks()).toEqual([{ id: "t1" }]);
  });
  it("createTask POSTs and unwraps {task}", async () => {
    const spy = ok({ task: { id: "t2", title: "x" } });
    vi.stubGlobal("fetch", spy);
    const t = await createTask({ title: "x", seed: null });
    expect(t.id).toBe("t2");
    expect((spy.mock.calls[0] as unknown as [string, RequestInit])[1].method).toBe("POST");
  });
  it("getTask normalizes absent tool_call to null", async () => {
    vi.stubGlobal("fetch", ok({ task: { id: "t3" }, messages: [{ id: "m1", task_id: "t3", role: "assistant", content: "hi", created_at: "z" }], cards: [] }));
    const d = await getTask("t3");
    expect(d.messages[0]!.tool_call).toBeNull();
  });
});

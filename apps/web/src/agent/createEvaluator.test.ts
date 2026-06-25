import { describe, it, expect } from "vitest";
import { createStore } from "../store/createStore";
import { createEvaluator } from "./createEvaluator";

const task = { id: "t1", title: "t", seed: null, status: "active" as const, created_at: "1", last_active_at: "1" };

describe("createEvaluator (API)", () => {
  it("run() calls the API, stores the evaluation, and ends done", async () => {
    const store = createStore({}); store.putTask(task);
    const evaluation = { task_id: "t1", scores: [{ dim_id: "D1", level: "L3", note: "n" }], narrative: "印记", created_at: "z" };
    const api = { async runEvaluation() { return evaluation; } };
    const ev = createEvaluator({ api: api as never, store, taskId: "t1" });
    await ev.run();
    expect(ev.getSnapshot()).toMatchObject({ phase: "done", evaluation });
    expect(store.getLatestEvaluation("t1")!.narrative).toBe("印记");
  });
  it("run() surfaces an error", async () => {
    const store = createStore({}); store.putTask(task);
    const api = { async runEvaluation() { throw new Error("评估失败"); } };
    const ev = createEvaluator({ api: api as never, store, taskId: "t1" });
    await ev.run();
    expect(ev.getSnapshot()).toMatchObject({ phase: "error", error: "评估失败" });
  });
});

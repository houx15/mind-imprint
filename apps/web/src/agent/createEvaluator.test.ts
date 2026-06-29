import { describe, it, expect, vi } from "vitest";
import { createEvaluator } from "./createEvaluator";
import type { Evaluation } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";

function fakeStore(): Store {
  return { putEvaluation: vi.fn() } as unknown as Store;
}
const ev = (status: Evaluation["status"]): Evaluation => ({
  id: "ev1", task_id: "t1", status, scores: [], narrative: "n",
  created_at: "2026-06-29T00:00:00.000Z", completed_at: null,
});

describe("createEvaluator (async polling)", () => {
  const noWait = async () => {};

  it("polls queued → running → done and resolves to phase 'done'", async () => {
    const getEvaluation = vi.fn()
      .mockResolvedValueOnce(ev("running"))
      .mockResolvedValueOnce(ev("done"));
    const api = { runEvaluation: vi.fn().mockResolvedValue(ev("queued")), getEvaluation };
    const evaluator = createEvaluator({ api, store: fakeStore(), taskId: "t1", wait: noWait });
    await evaluator.run();
    expect(evaluator.getSnapshot().phase).toBe("done");
    expect(evaluator.getSnapshot().evaluation?.status).toBe("done");
  });

  it("resolves to 'error' when the eval status becomes 'failed'", async () => {
    const api = {
      runEvaluation: vi.fn().mockResolvedValue(ev("queued")),
      getEvaluation: vi.fn().mockResolvedValue(ev("failed")),
    };
    const evaluator = createEvaluator({ api, store: fakeStore(), taskId: "t1", wait: noWait });
    await evaluator.run();
    expect(evaluator.getSnapshot().phase).toBe("error");
  });

  it("times out to 'error' if it never reaches a terminal status", async () => {
    const api = {
      runEvaluation: vi.fn().mockResolvedValue(ev("queued")),
      getEvaluation: vi.fn().mockResolvedValue(ev("running")),
    };
    const evaluator = createEvaluator({ api, store: fakeStore(), taskId: "t1", wait: noWait, maxAttempts: 3 });
    await evaluator.run();
    expect(evaluator.getSnapshot().phase).toBe("error");
  });
});

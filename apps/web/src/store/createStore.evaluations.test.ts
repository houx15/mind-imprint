import { describe, it, expect } from "vitest";
import { createStore } from "./createStore";
import { makeMemoryStorage, STORE_KEY } from "./storage";
import type { Evaluation } from "@mind-imprint/contracts";

function ev(task_id: string, at: string, narrative: string): Evaluation {
  return { task_id, scores: [{ dim_id: "D2", level: "L4", note: "n" }], narrative, created_at: at };
}

describe("store evaluations", () => {
  it("appends + lists + latest by created_at", () => {
    const store = createStore({ storage: makeMemoryStorage() });
    store.putEvaluation(ev("t_1", "2026-06-21T10:00:00.000Z", "first"));
    store.putEvaluation(ev("t_1", "2026-06-21T11:00:00.000Z", "second"));
    store.putEvaluation(ev("t_2", "2026-06-21T10:30:00.000Z", "other"));
    expect(store.listEvaluations("t_1")).toHaveLength(2);
    expect(store.getLatestEvaluation("t_1")!.narrative).toBe("second");
    expect(store.getLatestEvaluation("t_x")).toBeUndefined();
  });

  it("loads a legacy blob with no evaluations field (backward compatible)", () => {
    const storage = makeMemoryStorage();
    storage.setItem(STORE_KEY, JSON.stringify({ version: 1, tasks: [], messages: [], cards: [] }));
    const store = createStore({ storage });
    expect(store.listEvaluations("t_1")).toEqual([]);
  });
});

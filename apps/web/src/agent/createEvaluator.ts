import type { Evaluation } from "@mind-imprint/contracts";
import type { ApiClient } from "../api";
import type { Store } from "../store/createStore";

export type EvalPhase = "idle" | "running" | "done" | "error";
export interface EvalState { phase: EvalPhase; evaluation?: Evaluation; error?: string }

export interface EvaluatorDeps {
  api: Pick<ApiClient, "runEvaluation" | "getEvaluation">;
  store: Store;
  taskId: string;
  pollIntervalMs?: number;
  maxAttempts?: number;
  wait?: (ms: number) => Promise<void>;
}

export interface Evaluator {
  getSnapshot(): EvalState;
  subscribe(listener: () => void): () => void;
  run(): Promise<void>;
}

const defaultWait = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

export function createEvaluator(deps: EvaluatorDeps): Evaluator {
  const { api, store, taskId } = deps;
  const pollIntervalMs = deps.pollIntervalMs ?? 2500;
  const maxAttempts = deps.maxAttempts ?? 40;
  const wait = deps.wait ?? defaultWait;

  let state: EvalState = { phase: "idle" };
  const listeners = new Set<() => void>();
  function setState(next: Partial<EvalState>): void {
    const merged = { ...state, ...next };
    if ((Object.keys(merged) as (keyof EvalState)[]).some((k) => merged[k] !== state[k])) {
      state = merged;
      listeners.forEach((l) => l());
    }
  }

  return {
    getSnapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    async run() {
      setState({ phase: "running", error: undefined });
      try {
        // POST enqueues (202) and returns a queued/in-flight row; then poll until terminal.
        let evaluation = await api.runEvaluation(taskId);
        for (let attempt = 0; attempt < maxAttempts; attempt++) {
          if (evaluation.status === "done") {
            store.putEvaluation(evaluation);
            setState({ phase: "done", evaluation });
            return;
          }
          if (evaluation.status === "failed") {
            setState({ phase: "error", error: "评估失败，请重试" });
            return;
          }
          await wait(pollIntervalMs);
          const latest = await api.getEvaluation(taskId);
          if (latest) evaluation = latest;
        }
        setState({ phase: "error", error: "评估超时，请重试" });
      } catch (e) {
        setState({ phase: "error", error: e instanceof Error ? e.message : String(e) });
      }
    },
  };
}

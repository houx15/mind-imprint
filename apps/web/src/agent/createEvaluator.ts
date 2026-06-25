import type { Evaluation } from "@mind-imprint/contracts";
import type { ApiClient } from "../api";
import type { Store } from "../store/createStore";

export type EvalPhase = "idle" | "running" | "done" | "error";
export interface EvalState { phase: EvalPhase; evaluation?: Evaluation; error?: string }

export interface EvaluatorDeps { api: Pick<ApiClient, "runEvaluation">; store: Store; taskId: string }

export interface Evaluator {
  getSnapshot(): EvalState;
  subscribe(listener: () => void): () => void;
  run(): Promise<void>;
}

export function createEvaluator(deps: EvaluatorDeps): Evaluator {
  const { api, store, taskId } = deps;
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
        const evaluation = await api.runEvaluation(taskId);
        store.putEvaluation(evaluation);
        setState({ phase: "done", evaluation });
      } catch (e) {
        setState({ phase: "error", error: e instanceof Error ? e.message : String(e) });
      }
    },
  };
}

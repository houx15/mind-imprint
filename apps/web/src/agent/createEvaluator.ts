import type { Evaluation, CardSpec } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import type { ChatFn, RunEvaluationDeps } from "./runEvaluation";
import type { LlmConfig } from "../llm/types";
import { runEvaluation } from "./runEvaluation";

export type EvalPhase = "idle" | "running" | "done" | "error";

export interface EvalState {
  phase: EvalPhase;
  evaluation?: Evaluation;
  error?: string;
}

export interface Evaluator {
  getSnapshot(): EvalState;
  subscribe(listener: () => void): () => void;
  run(): Promise<void>;
}

export interface EvaluatorDeps {
  store: Store;
  chat: ChatFn;
  config: Partial<LlmConfig>;
  registry: Record<string, CardSpec>;
  taskId: string;
  now?: () => string;
}

export function createEvaluator(deps: EvaluatorDeps): Evaluator {
  let state: EvalState = { phase: "idle" };
  const listeners = new Set<() => void>();

  function setState(next: Partial<EvalState>): void {
    const nextState = { ...state, ...next };
    const changed = (Object.keys(nextState) as (keyof EvalState)[]).some(
      (k) => nextState[k] !== state[k],
    );
    state = nextState;
    if (changed) {
      listeners.forEach((l) => l());
    }
  }

  const runEvalDeps: RunEvaluationDeps = {
    store: deps.store,
    chat: deps.chat,
    config: deps.config,
    registry: deps.registry,
    taskId: deps.taskId,
    now: deps.now,
  };

  return {
    getSnapshot(): EvalState {
      return state;
    },

    subscribe(listener: () => void): () => void {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },

    async run(): Promise<void> {
      setState({ phase: "running", error: undefined });
      try {
        const evaluation = await runEvaluation(runEvalDeps);
        setState({ phase: "done", evaluation });
      } catch (e) {
        setState({
          phase: "error",
          error: e instanceof Error ? e.message : String(e),
        });
      }
    },
  };
}

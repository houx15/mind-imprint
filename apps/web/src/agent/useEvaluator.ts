import { useSyncExternalStore } from "react";
import type { Evaluator, EvalState } from "./createEvaluator";

export function useEvaluator(ev: Evaluator): EvalState {
  return useSyncExternalStore(ev.subscribe, ev.getSnapshot);
}

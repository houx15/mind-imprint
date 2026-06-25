import type { Evaluation } from "@mind-imprint/contracts";
import { apiFetch, ApiError } from "./client";

export async function runEvaluation(taskId: string): Promise<Evaluation> {
  const r = await apiFetch<{ evaluation: Evaluation }>(`/api/v1/tasks/${taskId}/evaluate`, { method: "POST" });
  return r.evaluation;
}

export async function getEvaluation(taskId: string): Promise<Evaluation | null> {
  try {
    const r = await apiFetch<{ evaluation: Evaluation }>(`/api/v1/tasks/${taskId}/evaluation`);
    return r.evaluation;
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

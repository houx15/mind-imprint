import { DualAxisReport } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// Reads the growth report (Task 6's GET .../assessment). The server returns
// a bare JSON `null` (200, never 204/404) when no assessment has been
// generated yet for this project — an honest "nothing here" the empty state
// renders directly, not an error.
export async function getAssessment(projectId: string): Promise<DualAxisReport | null> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/assessment`);
  if (raw == null) return null;
  return DualAxisReport.parse(raw);
}

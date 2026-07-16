import { Assessment } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// Reads the growth report (Task 6's GET .../assessment). The server returns
// a bare JSON `null` (200, never 204/404) when no assessment has been
// generated yet for this project — an honest "nothing here" the empty state
// renders directly, not an error.
export async function getAssessment(projectId: string): Promise<Assessment | null> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/assessment`);
  if (raw == null) return null;
  return Assessment.parse(raw);
}

// Runs the flagship assessor (Task 6's POST .../assessment) and returns the
// freshly persisted report — fails loud on schema drift, same posture as
// writing.ts's commitSnapshot.
export async function generateAssessment(projectId: string): Promise<Assessment> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/assessment`, { method: "POST" });
  return Assessment.parse(raw);
}

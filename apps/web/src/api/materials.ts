import { MaterialSource } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// The only two request shapes ingestMaterial accepts (apps/api/internal/api/
// materials.go's ingestMaterialReq): a URL to fetch, or pasted title+text.
// takeaway/tier are always the student's own words.
export type AddMaterialBody =
  | { url: string; takeaway: string; tier: string }
  | { title: string; text: string; takeaway: string; tier: string };

export async function addMaterial(projectId: string, body: AddMaterialBody): Promise<MaterialSource> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/materials`, {
    method: "POST",
    body: JSON.stringify(body),
  });
  return MaterialSource.parse(raw); // fail loud on drift
}

export async function logSourceOpen(projectId: string, materialId: string, timeSpentS: number): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/materials/${materialId}/open`, {
    method: "POST",
    body: JSON.stringify({ time_spent_s: timeSpentS }),
  });
}

// Fires when a source is OPENED for reading: asks the backend to surface the
// source's evaluation card (CRAAP / SIFT) and generate the article's flagged-
// sentence anchors, so "印记 reads it with you" — highlights + interactive card
// — appears on the next project fetch. Best-effort server-side (idempotent, no
// re-summon), so the caller refetches project state after it resolves.
export async function prepareSourceAnnotation(projectId: string, materialId: string): Promise<boolean> {
  const res = await apiFetch<{ surfaced?: boolean }>(`/api/v1/projects/${projectId}/materials/${materialId}/annotate`, {
    method: "POST",
  });
  return res?.surfaced ?? false;
}

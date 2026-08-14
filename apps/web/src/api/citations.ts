import { apiFetch } from "./client";

// citations.ts — G3 · record a source→section citation link when the student
// inserts a material into an essay section. Piggybacks the existing
// insert-at-caret action (no new UI). Best-effort provenance recording; the
// caller fires it fire-and-forget so a failed record never blocks writing.
export async function recordCitation(projectId: string, referenceId: string, section: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/citations`, {
    method: "POST",
    body: JSON.stringify({ referenceId, section }),
  });
}

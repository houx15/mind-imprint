import { z } from "zod";
import { DraftAnnotation } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// proposalAnnotations.ts — slice 3b · the 批注 client. 批注 are layered, colored,
// view-only teacher annotations rendered in the left panel.

const AnnotationsResp = z.object({ annotations: z.array(DraftAnnotation) });

// slice 4b · doc-scoped: proposal (default) or essay 批注.
export async function getProposalAnnotations(projectId: string, doc: "proposal" | "essay" = "proposal"): Promise<DraftAnnotation[]> {
  const q = doc === "essay" ? "?doc=essay" : "";
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-annotations${q}`);
  return AnnotationsResp.parse(raw).annotations;
}

// The whole-draft "AI check".
export async function reviewProposalAnnotations(projectId: string): Promise<DraftAnnotation[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-annotations/review`, { method: "POST" });
  return AnnotationsResp.parse(raw).annotations;
}

// G2 · record that the student OPENED / clicked a specific 批注 — the "did she
// engage with AI feedback" signal (D5 · 反馈处理与修订). Fire-and-forget; a failed
// record never disrupts the jump-to-anchor it rides alongside.
export async function recordAnnotationOpen(projectId: string, annotationId: string, doc: "proposal" | "essay" = "proposal"): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/annotations/open`, {
    method: "POST",
    body: JSON.stringify({ annotationId, doc }),
  });
}

import { z } from "zod";
import { DraftAnnotation } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// proposalAnnotations.ts — slice 3b · the 批注 client. 批注 are layered, colored,
// view-only teacher annotations rendered in the left panel.

const AnnotationsResp = z.object({ annotations: z.array(DraftAnnotation) });

export async function getProposalAnnotations(projectId: string): Promise<DraftAnnotation[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-annotations`);
  return AnnotationsResp.parse(raw).annotations;
}

// The whole-draft "AI check".
export async function reviewProposalAnnotations(projectId: string): Promise<DraftAnnotation[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-annotations/review`, { method: "POST" });
  return AnnotationsResp.parse(raw).annotations;
}

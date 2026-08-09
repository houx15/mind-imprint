import { z } from "zod";
import { ProposalGuideStep, DraftAnnotation } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// essayStatement.ts — slice 4b · the essay statement-track client (reuses the
// ProposalGuideStep wire shape — the step DTO is doc-agnostic). The review
// returns essay 批注.

export async function getEssayStatement(projectId: string): Promise<ProposalGuideStep> {
  return ProposalGuideStep.parse(await apiFetch<unknown>(`/api/v1/projects/${projectId}/essay-statement`));
}

export async function startEssayStatement(projectId: string): Promise<ProposalGuideStep> {
  return ProposalGuideStep.parse(await apiFetch<unknown>(`/api/v1/projects/${projectId}/essay-statement/start`, { method: "POST" }));
}

export async function advanceEssayStatement(projectId: string, dir: "next" | "prev"): Promise<ProposalGuideStep> {
  return ProposalGuideStep.parse(
    await apiFetch<unknown>(`/api/v1/projects/${projectId}/essay-statement/advance`, {
      method: "POST",
      body: JSON.stringify({ dir }),
    }),
  );
}

export async function reviewEssayPart(projectId: string, stepKey?: string): Promise<DraftAnnotation[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/essay-statement/review`, {
    method: "POST",
    body: JSON.stringify({ stepKey: stepKey ?? "" }),
  });
  return z.object({ annotations: z.array(DraftAnnotation) }).parse(raw).annotations;
}

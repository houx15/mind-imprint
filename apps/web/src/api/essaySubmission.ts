import { ProposalGuideStep } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// essaySubmission.ts — slice 4c · the essay submission-track client (reuses the
// ProposalGuideStep wire shape — the step DTO is doc-agnostic). Steps: 引言 → 结论
// → 成文 → 润色. 成文/润色 assemble + review the whole draft on the writing surface.

export async function getEssaySubmission(projectId: string): Promise<ProposalGuideStep> {
  return ProposalGuideStep.parse(await apiFetch<unknown>(`/api/v1/projects/${projectId}/essay-submission`));
}

export async function startEssaySubmission(projectId: string): Promise<ProposalGuideStep> {
  return ProposalGuideStep.parse(await apiFetch<unknown>(`/api/v1/projects/${projectId}/essay-submission/start`, { method: "POST" }));
}

export async function advanceEssaySubmission(projectId: string, dir: "next" | "prev"): Promise<ProposalGuideStep> {
  return ProposalGuideStep.parse(
    await apiFetch<unknown>(`/api/v1/projects/${projectId}/essay-submission/advance`, {
      method: "POST",
      body: JSON.stringify({ dir }),
    }),
  );
}

import { z } from "zod";
import { ProposalGuideStep, DraftAnnotation, type SubQuestion } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// proposalTrack.ts — slice 3a · the proposal-writing guide-step track client.
// The track (free/guided + dynamic per-sub-question steps) is server-derived; each
// call returns the fresh ProposalGuideStep so the UI updates in one round-trip.

export async function getProposalTrack(projectId: string): Promise<ProposalGuideStep> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-track`);
  return ProposalGuideStep.parse(raw);
}

export async function setProposalMode(projectId: string, mode: "free" | "guided"): Promise<ProposalGuideStep> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-track/mode`, {
    method: "POST",
    body: JSON.stringify({ mode }),
  });
  return ProposalGuideStep.parse(raw);
}

export async function startProposalGuide(projectId: string): Promise<ProposalGuideStep> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-track/start`, { method: "POST" });
  return ProposalGuideStep.parse(raw);
}

export async function setSubQuestions(projectId: string, subQuestions: SubQuestion[]): Promise<ProposalGuideStep> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-track/subquestions`, {
    method: "POST",
    body: JSON.stringify({ subQuestions }),
  });
  return ProposalGuideStep.parse(raw);
}

export async function advanceProposalStep(projectId: string, dir: "next" | "prev"): Promise<ProposalGuideStep> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-track/advance`, {
    method: "POST",
    body: JSON.stringify({ dir }),
  });
  return ProposalGuideStep.parse(raw);
}

// §4 gap G10 · jump to an absolute step index (the parts overview).
export async function jumpProposalStep(projectId: string, to: number): Promise<ProposalGuideStep> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-track/advance`, {
    method: "POST",
    body: JSON.stringify({ to }),
  });
  return ProposalGuideStep.parse(raw);
}

// The "我写好了" review of the current part (slice 3b): the flagship reviewer
// returns layered colored 批注 rendered view-only in the left panel. `stepKey`
// optional — defaults to the current step server-side.
export async function reviewProposalPart(projectId: string, stepKey?: string): Promise<DraftAnnotation[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/proposal-track/review`, {
    method: "POST",
    body: JSON.stringify({ stepKey: stepKey ?? "" }),
  });
  return z.object({ annotations: z.array(DraftAnnotation) }).parse(raw).annotations;
}

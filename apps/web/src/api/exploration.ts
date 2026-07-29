import type { LeadStatus } from "@mind-imprint/contracts";
import { ExplorationView, ExplorationLead, ExplorationGuide } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// S3 rabbit-hole exploration: thin API client mirroring reading.ts's
// Zod-parse-every-response discipline. Field names are already camelCase
// end-to-end (Go's explorationLeadDTO / ExplorationView wire shape), so
// bodies pass straight through with no snake_case translation.

export async function getExploration(projectId: string): Promise<ExplorationView> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration`);
  return ExplorationView.parse(raw);
}

export async function createLead(projectId: string, text: string): Promise<ExplorationLead> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/leads`, {
    method: "POST",
    body: JSON.stringify({ text }),
  });
  return ExplorationLead.parse((raw as { lead: unknown }).lead);
}

export async function patchLead(
  projectId: string,
  lid: string,
  patch: { text?: string; status?: LeadStatus; connectedReferenceId?: string | null },
): Promise<ExplorationLead> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/leads/${lid}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
  return ExplorationLead.parse((raw as { lead: unknown }).lead);
}

export async function deleteLead(projectId: string, lid: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/exploration/leads/${lid}`, {
    method: "DELETE",
  });
}

export async function digDeeper(projectId: string): Promise<ExplorationGuide> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/guide`, {
    method: "POST",
    body: JSON.stringify({}),
  });
  return ExplorationGuide.parse(raw);
}

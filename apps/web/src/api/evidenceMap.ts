import { EvidenceMap, SubQuestionVerdict, Reference } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// evidenceMap.ts — slice 4a · the 证据地图 client. The map is the seeded warren;
// this reads its per-sub-question rollup, sets a paper's evidence facets, and
// runs the per-sub-question saturation review. The evidence/triage/archive
// endpoints return the updated Reference DTO directly (not wrapped).

export async function getEvidenceMap(projectId: string): Promise<EvidenceMap> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/evidence-map`);
  return EvidenceMap.parse(raw);
}

export async function reviewSubQuestion(projectId: string, sqId: string): Promise<SubQuestionVerdict> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/evidence-map/subquestions/${sqId}/review`, { method: "POST" });
  return SubQuestionVerdict.parse(raw);
}

export type ReferenceEvidence = { nature: "" | "support" | "challenge"; argument: string; finding: string; placement: string };

export async function setReferenceEvidence(projectId: string, rid: string, ev: ReferenceEvidence): Promise<Reference> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/references/${rid}/evidence`, {
    method: "PATCH",
    body: JSON.stringify(ev),
  });
  return Reference.parse(raw);
}

export async function setReferenceTriage(projectId: string, rid: string, triage: "" | "red" | "yellow"): Promise<Reference> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/references/${rid}/triage`, {
    method: "PATCH",
    body: JSON.stringify({ triage }),
  });
  return Reference.parse(raw);
}

export async function archiveReference(projectId: string, rid: string, archived = true): Promise<Reference> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/references/${rid}/archive`, {
    method: "POST",
    body: JSON.stringify({ archived }),
  });
  return Reference.parse(raw);
}

// The essay stage flip (§6 flexible advance): research → statement opens the
// writing surface. The caller then re-applies studio state to switch rooms.
// claimId (slice 4b-2) deep-links the student onto that sub-question's claim step.
export async function advanceEssayStage(
  projectId: string,
  stage: "statement" | "submission",
  claimId?: string,
): Promise<{ stage: string; surface: string }> {
  return apiFetch<{ stage: string; surface: string }>(`/api/v1/projects/${projectId}/essay-track/advance-stage`, {
    method: "POST",
    body: JSON.stringify({ stage, ...(claimId ? { claimId } : {}) }),
  });
}

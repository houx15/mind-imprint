import { z } from "zod";
import type { LeadStatus, Reference, QuestionEdgeLabel, QuestionEdgeStatus } from "@mind-imprint/contracts";
import { ExplorationView, ExplorationLead, ExplorationGuide, DigResult, DigCandidate, QuestionEdge } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// S3 rabbit-hole exploration: thin API client mirroring reading.ts's
// Zod-parse-every-response discipline. Field names are already camelCase
// end-to-end (Go's explorationLeadDTO / ExplorationView wire shape), so
// bodies pass straight through with no snake_case translation.

export async function getExploration(projectId: string): Promise<ExplorationView> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration`);
  return ExplorationView.parse(raw);
}

// createLead adds a manual lead — top-level, or a 分支 under parentLeadId (#12).
export async function createLead(projectId: string, text: string, opts?: { parentLeadId?: string }): Promise<ExplorationLead> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/leads`, {
    method: "POST",
    body: JSON.stringify({ text, parentLeadId: opts?.parentLeadId ?? null }),
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

// digDeeper — 深挖. With no opts it points off the whole graph (深挖一层); with a
// leadId (+ optional thought) it focuses on ONE lead, carrying the student's own
// thinking (#12/#13).
export async function digDeeper(projectId: string, opts?: { leadId?: string; thought?: string }): Promise<ExplorationGuide> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/guide`, {
    method: "POST",
    body: JSON.stringify(opts ?? {}),
  });
  return ExplorationGuide.parse(raw);
}

export async function digExploration(projectId: string, opts: { leadId?: string; keyword?: string }): Promise<DigResult> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/dig`, { method: "POST", body: JSON.stringify(opts) });
  return DigResult.parse(raw);
}

export async function adoptCandidate(projectId: string, candidate: DigCandidate, opts?: { parentLeadId?: string }): Promise<{ lead: ExplorationLead; reference: Reference }> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/adopt`, { method: "POST", body: JSON.stringify({ candidate, parentLeadId: opts?.parentLeadId ?? null }) });
  return raw as { lead: ExplorationLead; reference: Reference };
}

// B2 · question_edge lifecycle (create/relabel/confirm/dismiss) — the labeled
// edges between top-level question leads. createEdge is only reachable from
// the student's own map action, so the server confirms it on arrival (铁律①:
// an edge the student typed herself needs no separate confirm step).

export async function createEdge(
  projectId: string,
  edge: { fromLeadId: string; toLeadId: string; label: QuestionEdgeLabel },
): Promise<QuestionEdge> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/edges`, {
    method: "POST",
    body: JSON.stringify(edge),
  });
  return QuestionEdge.parse((raw as { edge: unknown }).edge);
}

export async function patchEdge(
  projectId: string,
  eid: string,
  patch: { label?: QuestionEdgeLabel; status?: QuestionEdgeStatus },
): Promise<QuestionEdge> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/edges/${eid}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
  return QuestionEdge.parse((raw as { edge: unknown }).edge);
}

export async function deleteEdge(projectId: string, eid: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/exploration/edges/${eid}`, {
    method: "DELETE",
  });
}

// B3 · proposeEdges asks 印记 to propose labeled edges between the project's
// own root question nodes. Every returned edge already has status:"proposed"
// (铁律①: AI proposes, student confirms via patchEdge's existing
// status:"confirmed" path) — this call never lands a confirmed edge itself.
const ProposeEdgesResponse = z.object({ edges: z.array(QuestionEdge) });

export async function proposeEdges(projectId: string): Promise<QuestionEdge[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/edges/propose`, {
    method: "POST",
  });
  return ProposeEdgesResponse.parse(raw).edges;
}

import { CreateProjectResult, DualAxisReport, StudioProjection } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export type ProjectListItem = {
  id: string;
  title: string;
  qualLabel: string;
  activeStation: string;
};

export async function listProjects(): Promise<ProjectListItem[]> {
  const res = await apiFetch<{ projects: ProjectListItem[] }>("/api/v1/projects");
  return res.projects;
}

export async function getProject(id: string): Promise<StudioProjection> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}`);
  return StudioProjection.parse(raw); // fail loud on drift
}

// A3: the project terminal — closes out the project and returns the
// freshly persisted growth report; this is the one-time terminal action,
// not a repeatable generation endpoint.
export async function finishProject(id: string): Promise<DualAxisReport> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/finish`, { method: "POST" });
  return DualAxisReport.parse(raw);
}

export async function createProject(body: { title?: string; prompt: string }): Promise<{ id: string }> {
  const raw = await apiFetch<unknown>(`/api/v1/projects`, { method: "POST", body: JSON.stringify(body) });
  return CreateProjectResult.parse(raw);
}

export async function submitOnboarding(projectId: string, body: { restate: string; weakPicks: number[] }): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/onboarding`, { method: "POST", body: JSON.stringify(body) });
}

export async function submitSelfScore(projectId: string, body: { scores: { code: string; band: number }[] }): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/self-score`, { method: "POST", body: JSON.stringify(body) });
}

export async function submitReflection(projectId: string, body: { text: string }): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/reflection`, { method: "POST", body: JSON.stringify(body) });
}

// N3d Task 9: S1 立题's whole-panel submit — mirrors submitOnboarding's shape
// (a plain DB write, not autosave); terms/answers/searchPlan travel verbatim.
export async function submitFraming(
  projectId: string,
  body: { terms: { term: string; definition: string }[]; answers: string[]; searchPlan: string[] },
): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/framing`, { method: "POST", body: JSON.stringify(body) });
}

// N3d Task 9: S2 视角与素材's whole-panel submit — level is one of
// national/global_for/global_against (the binding design's own three levels,
// dc.html:2158), kept as a plain string here (not the Zod enum) so a
// perspective-matrix-minted row with no level can still round-trip.
export async function submitPerspectives(
  projectId: string,
  body: { perspectives: { text: string; level: string }[] },
): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/perspectives`, { method: "POST", body: JSON.stringify(body) });
}

// N6-E Task 6: re-opens a `waived` station (the journey composer skipped it
// for this student) so she can walk it after all — mirrors submitOnboarding/
// submitPerspectives above (a plain POST, no body, no response to parse).
export async function reopenStation(projectId: string, code: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/journey/reopen/${code}`, { method: "POST" });
}

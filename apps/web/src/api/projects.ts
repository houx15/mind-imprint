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

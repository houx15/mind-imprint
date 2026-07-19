import { DualAxisReport, StudioProjection } from "@mind-imprint/contracts";
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

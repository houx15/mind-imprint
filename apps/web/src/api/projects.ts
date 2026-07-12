import { StudioProjection } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export type ProjectListItem = {
  id: string;
  title: string;
  qualLabel: string;
  activeStation: string;
};

export async function listProjects(): Promise<ProjectListItem[]> {
  return apiFetch<ProjectListItem[]>("/api/v1/projects");
}

export async function getProject(id: string): Promise<StudioProjection> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}`);
  return StudioProjection.parse(raw); // fail loud on drift
}

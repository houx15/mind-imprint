import { apiFetch } from "./client";

export interface ClassSummary {
  id: string;
  name: string;
  join_code: string;
  school_id: string;
  created_at: string;
}

export interface RosterStudent {
  id: string;
  display_name: string;
  email: string;
  last_active_at: string | null;
  task_count: number;
  evaluation_count: number;
  card_count: number;
}

export interface ClassDetail {
  class: ClassSummary;
  roster: RosterStudent[];
}

export async function listClasses(): Promise<ClassSummary[]> {
  const r = await apiFetch<{ classes: ClassSummary[] }>("/api/v1/classes");
  return r.classes;
}

export async function createClass(input: { name: string }): Promise<ClassSummary> {
  const r = await apiFetch<{ class: ClassSummary }>("/api/v1/classes", {
    method: "POST",
    body: JSON.stringify({ name: input.name }),
  });
  return r.class;
}

export async function getClass(id: string): Promise<ClassDetail> {
  return apiFetch<ClassDetail>(`/api/v1/classes/${id}`);
}

export async function renameClass(id: string, name: string): Promise<ClassSummary> {
  const r = await apiFetch<{ class: ClassSummary }>(`/api/v1/classes/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ name }),
  });
  return r.class;
}

export async function regenerateJoinCode(id: string): Promise<ClassSummary> {
  const r = await apiFetch<{ class: ClassSummary }>(`/api/v1/classes/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ regenerate_join_code: true }),
  });
  return r.class;
}

export async function removeEnrollment(id: string, userId: string): Promise<void> {
  await apiFetch<void>(`/api/v1/classes/${id}/enrollments/${userId}`, { method: "DELETE" });
}

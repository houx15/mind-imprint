import { apiFetch } from "./client";
import type { Teacher } from "./classes";

export type { Teacher };

export interface Overview {
  counts: {
    student: number;
    teacher: number;
    class: number;
    project: number;
    evaluation: number;
    active_student: number;
  };
  usage_by_tier: { tier: string; prompt_tokens: number; completion_tokens: number; cost: string }[];
}

export interface TeacherInvite {
  id: string;
  code: string;
  expires_at: string;
  created_at: string;
  email?: string;
}

export interface ImportRow {
  class: string;
  teacher_email?: string;
  student_email?: string;
}

export interface ImportResult {
  classes: { name: string; join_code: string }[];
  teacher_invites: { email: string; code: string }[];
}

export async function getOverview(): Promise<Overview> {
  return apiFetch<Overview>("/api/v1/admin/overview");
}

export async function listTeacherInvites(): Promise<TeacherInvite[]> {
  const r = await apiFetch<{ invites: TeacherInvite[] }>("/api/v1/admin/teacher-invites");
  return r.invites;
}

export async function createTeacherInvite(input: { email?: string; expires_days?: number }): Promise<{ code: string; expires_at: string }> {
  return apiFetch<{ code: string; expires_at: string }>("/api/v1/admin/teacher-invites", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function adminImport(rows: ImportRow[]): Promise<ImportResult> {
  return apiFetch<ImportResult>("/api/v1/admin/import", {
    method: "POST",
    body: JSON.stringify({ rows }),
  });
}

export async function listTeachers(): Promise<Teacher[]> {
  const r = await apiFetch<{ teachers: Teacher[] }>("/api/v1/admin/teachers");
  return r.teachers;
}

export async function assignTeacher(classId: string, teacherUserId: string): Promise<{ teachers: Teacher[] }> {
  return apiFetch<{ teachers: Teacher[] }>(`/api/v1/classes/${classId}/teachers`, {
    method: "POST",
    body: JSON.stringify({ teacher_user_id: teacherUserId }),
  });
}

export async function removeTeacher(classId: string, userId: string): Promise<void> {
  await apiFetch<void>(`/api/v1/classes/${classId}/teachers/${userId}`, { method: "DELETE" });
}

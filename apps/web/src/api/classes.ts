import { apiFetch } from "./client";

export interface ClassSummary {
  id: string;
  name: string;
  join_code: string;
  school_id: string;
  created_at: string;
  grade: string;
  grade_label: string;
}

// 与后端闭表（apps/api/internal/api/class_grade.go 的 classGrades）一致。
// 放在这里而不是某一个页面里：pro 的班级列表、pro 的班级详情、lite 的教师端
// 三处都要用同一份，而 lite 的 `@/*` 解析到 apps/web/src，所以它拿得到。
// 第一项代表「不填」—— 它是一个合法值，不是占位符。
export const CLASS_GRADE_OPTIONS: { value: string; label: string }[] = [
  { value: "", label: "未填写" },
  { value: "junior1", label: "初一" },
  { value: "junior2", label: "初二" },
  { value: "junior3", label: "初三" },
  { value: "senior1", label: "高一" },
  { value: "senior2", label: "高二" },
  { value: "senior3", label: "高三" },
];

export interface RosterStudent {
  id: string;
  display_name: string;
  email: string;
  last_active_at: string | null;
  project_count: number;
  evaluation_count: number;
  card_count: number;
}

export interface Teacher {
  id: string;
  display_name: string;
  email: string;
}

export interface ClassDetail {
  class: ClassSummary;
  roster: RosterStudent[];
  teachers: Teacher[];
}

export async function listClasses(): Promise<ClassSummary[]> {
  const r = await apiFetch<{ classes: ClassSummary[] }>("/api/v1/classes");
  return r.classes;
}

export async function createClass(input: { name: string; teacher_user_id?: string; grade?: string }): Promise<ClassSummary> {
  const body: Record<string, unknown> = { name: input.name };
  if (input.teacher_user_id) body.teacher_user_id = input.teacher_user_id;
  if (input.grade) body.grade = input.grade;
  const r = await apiFetch<{ class: ClassSummary }>("/api/v1/classes", {
    method: "POST",
    body: JSON.stringify(body),
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

// 🚨 grade 传空串是合法的，意思是把年级清掉（后端 validateClassGrade 认空串）。
// 不要在这里加 `if (!grade) return`。
export async function setClassGrade(id: string, grade: string): Promise<ClassSummary> {
  const r = await apiFetch<{ class: ClassSummary }>(`/api/v1/classes/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ grade }),
  });
  return r.class;
}

export async function removeEnrollment(id: string, userId: string): Promise<void> {
  await apiFetch<void>(`/api/v1/classes/${id}/enrollments/${userId}`, { method: "DELETE" });
}

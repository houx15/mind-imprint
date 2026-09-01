import { apiFetch } from "./client";

// api/split.ts —— 分工。形状读自 apps/api/internal/api/pbl_split.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

export type Owner = "yinji" | "student" | "both";

export interface Substep {
  id: string;
  title: string;
  /** 印记提的分工，和它的理由。 */
  owner: Owner;
  reason: string;
  /** 她改成了什么，以及为什么。null = 她没动。 */
  studentOwner: Owner | null;
  studentReason: string;
  status: "todo" | "doing" | "done";
  confirmedAt: string | null;
  ordinal: number;
}

export const OWNER_LABELS: Record<Owner, string> = {
  student: "你做",
  yinji: "印记做",
  both: "一起做",
};

export function listSubsteps(projectId: string, stepId: string): Promise<Substep[]> {
  return apiFetch<Substep[]>(`${base(projectId)}/steps/${stepId}/substeps`);
}

export function proposeSubsteps(
  projectId: string,
  stepId: string,
  substeps: { title: string; owner: Owner; reason: string }[],
): Promise<Substep[]> {
  return apiFetch<Substep[]>(`${base(projectId)}/steps/${stepId}/substeps`, {
    method: "POST",
    body: JSON.stringify({ substeps }),
  });
}

/** 改一格。reason 是必须的——服务端也拦。 */
export function reassign(
  projectId: string,
  substepId: string,
  owner: Owner,
  reason: string,
): Promise<Substep> {
  return apiFetch<Substep>(`${base(projectId)}/substeps/${substepId}/reassign`, {
    method: "POST",
    body: JSON.stringify({ owner, reason }),
  });
}

export function confirmSubstep(projectId: string, substepId: string): Promise<Substep> {
  return apiFetch<Substep>(`${base(projectId)}/substeps/${substepId}/confirm`, { method: "POST" });
}

export function setSubstepStatus(
  projectId: string,
  substepId: string,
  status: "todo" | "doing" | "done",
): Promise<Substep> {
  return apiFetch<Substep>(`${base(projectId)}/substeps/${substepId}`, {
    method: "PATCH",
    body: JSON.stringify({ status }),
  });
}

/** 这一格最后归谁：她改过就听她的。 */
export function effectiveOwner(s: Substep): Owner {
  return s.studentOwner ?? s.owner;
}

/**
 * 印记领走了多少。
 *
 * 显示这个数字，是为了让"AI 帮我做了几乎全部"这件事**在她做决定之前**就看得
 * 见。事后统计没有用；她要在还能改的时候看见。
 */
export function shareOfWork(substeps: Substep[]): { yinji: number; total: number } {
  const total = substeps.length;
  const yinji = substeps.filter((s) => effectiveOwner(s) === "yinji").length;
  return { yinji, total };
}

export function splitTodo(substeps: Substep[]): string {
  if (substeps.length === 0) return "还没有分工";
  const unconfirmed = substeps.filter((s) => !s.confirmedAt).length;
  return unconfirmed ? `还有 ${unconfirmed} 格没定` : "";
}

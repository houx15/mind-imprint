import { apiFetch } from "./client";

// api/decide.ts —— 做一个决定。形状读自 apps/api/internal/api/pbl_decide.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

export interface DecisionOption {
  id: string;
  label: string;
  /** 赢在哪。 */
  wins: string;
  /** 疼在哪。 */
  hurts: string;
  author: "student" | "yinji";
  ordinal: number;
}

export interface DecisionCriterion {
  id: string;
  label: string;
  author: "student" | "yinji";
  ordinal: number;
}

export interface Decision {
  id: string;
  subject: string;
  choice: string;
  why: string;
  gaveUp: string;
  /** 什么会让她改主意。写得出这一句，这才是一个判断而不是一次表态。 */
  flip: string;
  settledAt: string | null;
  options: DecisionOption[];
  criteria: DecisionCriterion[];
  createdAt: string;
}

export function listDecisions(projectId: string): Promise<Decision[]> {
  return apiFetch<Decision[]>(`${base(projectId)}/decisions`);
}

export function openDecision(
  projectId: string,
  body: {
    subject: string;
    sessionId?: string;
    options?: { label: string; wins?: string; hurts?: string; author?: string }[];
    criteria?: { label: string; author?: string }[];
  },
): Promise<Decision> {
  return apiFetch<Decision>(`${base(projectId)}/decisions`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function addOption(
  projectId: string,
  decisionId: string,
  body: { label: string; wins?: string; hurts?: string },
): Promise<Decision> {
  return apiFetch<Decision>(`${base(projectId)}/decisions/${decisionId}/options`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function updateOption(
  projectId: string,
  optionId: string,
  patch: { label?: string; wins?: string; hurts?: string },
): Promise<Decision> {
  return apiFetch<Decision>(`${base(projectId)}/options/${optionId}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

export function addCriterion(
  projectId: string,
  decisionId: string,
  label: string,
): Promise<Decision> {
  return apiFetch<Decision>(`${base(projectId)}/decisions/${decisionId}/criteria`, {
    method: "POST",
    body: JSON.stringify({ label }),
  });
}

export function removeCriterion(projectId: string, criterionId: string): Promise<Decision> {
  return apiFetch<Decision>(`${base(projectId)}/criteria/${criterionId}`, { method: "DELETE" });
}

export function settleDecision(
  projectId: string,
  decisionId: string,
  body: { choice: string; why: string; gaveUp?: string; flip: string },
): Promise<Decision> {
  return apiFetch<Decision>(`${base(projectId)}/decisions/${decisionId}/settle`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/**
 * 还差什么才能定。
 *
 * 顺序不是随便排的：先有得选，再说清什么重要，再看每个选项赢在哪，最后才选。
 * 反过来做——先选好再补理由——正是这件工具想拦住的那种"决定"。
 */
export function decisionTodo(
  d: Decision | null,
  draft: { choice: string; why: string; flip: string },
): string {
  if (!d) return "还没开始";
  if (d.options.length < 2) return "至少两个选项，一个不叫选";
  if (d.criteria.length === 0) return "先说清这件事上什么最重要";
  const bare = d.options.filter((o) => !o.wins.trim() || !o.hurts.trim()).length;
  if (bare) return `还有 ${bare} 个选项没说赢在哪、疼在哪`;
  if (!draft.choice.trim()) return "选一个";
  if (!draft.why.trim()) return "为什么选它";
  if (!draft.flip.trim()) return "什么会让你改主意";
  return "";
}

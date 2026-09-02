import { apiFetch } from "./client";

// api/decide.ts —— 理性决策。形状读自 apps/api/internal/api/pbl_decide.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

/**
 * 一个待选项。**印记提的**——这件工具出现的时刻，正是 AI 抛出几条路让她定。
 * 所以卡片上有标题，也有一段说明：光一个标题，她判断不了。
 */
export interface DecisionOption {
  id: string;
  label: string;
  description: string;
  ordinal: number;
}

export interface Decision {
  id: string;
  subject: string;
  choice: string;
  /** 为什么选它。 */
  why: string;
  /** 为什么不选别的。这一句才说明她真的比较过。 */
  whyNot: string;
  settledAt: string | null;
  options: DecisionOption[];
  createdAt: string;
}

export function listDecisions(projectId: string): Promise<Decision[]> {
  return apiFetch<Decision[]>(`${base(projectId)}/decisions`);
}

/** 印记提一个待定的决定：一句"在定什么" + 几个选项。 */
export function openDecision(
  projectId: string,
  body: {
    subject: string;
    sessionId?: string;
    options: { label: string; description?: string }[];
  },
): Promise<Decision> {
  return apiFetch<Decision>(`${base(projectId)}/decisions`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function settleDecision(
  projectId: string,
  decisionId: string,
  body: { choice: string; why: string; whyNot: string },
): Promise<Decision> {
  return apiFetch<Decision>(`${base(projectId)}/decisions/${decisionId}/settle`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/** 当前这个还没定的决定。同一时刻最多摆一个在她面前。 */
export function openDecisionOf(all: Decision[]): Decision | null {
  const live = all.filter((d) => !d.settledAt);
  return live[live.length - 1] ?? null;
}

/**
 * 还差什么才能确认。
 *
 * 顺序就是她该走的顺序：先选，再说为什么选它，最后说为什么放掉别的。
 * 最后那一句是这件工具真正教的东西——选中一个不难。
 */
export function decisionTodo(
  d: Decision | null,
  draft: { choice: string; why: string; whyNot: string },
): string {
  if (!d) return "暂时没有需要决策的内容";
  if (!draft.choice.trim()) return "请选择一个方案";
  if (!draft.why.trim()) return "为什么选它";
  if (!draft.whyNot.trim()) return "为什么不选别的";
  return "";
}

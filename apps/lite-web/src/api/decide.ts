import { apiFetch } from "./client";
import { toneAt, type Tone } from "../shared/tone";

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
  /** 谁提的这条路。她自己加的那条是「这些都不对，我要的是另一样」。 */
  author: "yinji" | "student";
  /** 她排的名次，1 是第一。0 = 还没排过。 */
  studentRank: number;
  ordinal: number;
}

export interface Decision {
  id: string;
  subject: string;
  choice: string;
  /** 为什么选它。 */
  why: string;
  /** 什么情况会让她改主意。复盘时拿它对照。 */
  flip: string;
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

/**
 * 她自己往里加一条路。
 *
 * 🚨 印记给的三条不是全集。「在别人摆好的选项里挑一个」和「决定」是两回事——
 * 后者包含「这些都不对，我要的是另一样」。
 */
export function addOption(
  projectId: string,
  decisionId: string,
  body: { label: string; description: string },
): Promise<Decision> {
  return apiFetch<Decision>(`${base(projectId)}/decisions/${decisionId}/options`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/**
 * 她把几条路排出来的顺序，按名次从高到低。
 *
 * 🚨 一次收全部：名次是个整体，逐条发会在中途留下两个第一名。
 */
export function rankOptions(
  projectId: string,
  decisionId: string,
  order: string[],
): Promise<Decision> {
  return apiFetch<Decision>(`${base(projectId)}/decisions/${decisionId}/rank`, {
    method: "POST",
    body: JSON.stringify({ order }),
  });
}

export function settleDecision(
  projectId: string,
  decisionId: string,
  body: { choice: string; why: string; whyNot: string; flip?: string },
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
  if (!draft.why.trim()) return "选择原因";
  if (!draft.whyNot.trim()) return "未选方案的原因";
  return "";
}

/**
 * 每条路一个颜色。
 *
 * 🚨 产品负责人 2026-09-03：「colorful, interactive」。原来几个选项长得一模一样
 * ——同一个灰框、同一号字。要她「把几条路放在一起比」，而屏幕上它们根本分不开，
 * 她只能顺着读下来，读到哪个顺眼点哪个。给了颜色和编号，它们才成为几个**东西**，
 * 才谈得上比较；下面「放掉的」那几张也才认得出谁是谁。
 *
 * 五个色相循环，和便签板同一套（见 api/notes.ts），学生在两块界面上看到的是
 * 同一种视觉语言。
 */
export function optionTone(index: number): Tone {
  return toneAt(index);
}

/** 选项的编号：A、B、C…… 比「选项 1」短，也比原文标签好指。 */
export function optionTag(index: number): string {
  return String.fromCharCode(65 + (index % 26));
}

/**
 * 把「为什么放掉每一条」拼成一句存下来。
 *
 * 🚨 后端存的是一个 whyNot 字符串，而她现在是一条一条分开答的——分开答才逼她
 * 真的面对每一条，一个大框只会得到一句「其他的都不太合适」。拼的时候带上标签，
 * 回灌给印记的那一句才说得清她放掉的是哪个。
 */
export function joinWhyNot(entries: { label: string; why: string }[]): string {
  return entries
    .map((e) => ({ label: e.label.trim(), why: e.why.trim() }))
    .filter((e) => e.why !== "")
    .map((e) => `${e.label}：${e.why}`)
    .join("；");
}

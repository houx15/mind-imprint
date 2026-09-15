import { CARD_REGISTRY } from "@mind-imprint/contracts";

export type OutputLine = { label: string; text: string };

// Summarize only fields whose meaning is defined by current tool payloads.
// Unknown fields stay available in the complete record; never guess their meaning.
const LABELS: Record<string, string> = {
  title: "标题", body: "正文", text: "内容", summary: "摘要", reason: "原因",
  why: "原因", idea: "选定方案", outline: "提纲", choice: "选择", label: "名称",
  keywords: "关键词", noteCount: "便签数量", count: "想法数量", brought: "已记录材料数",
  nodes: "提纲条目数", entries: "反馈条目数", mine: "学生负责的任务数", total: "任务总数",
  url: "链接", hasHero: "已生成头图", published: "已发布", guessed: "待核实的假设", admits: "当前局限",
};
const VERDICTS: Record<string, string> = { kept: "保留", revise: "需要修改", dropped: "不采用" };

export function summarizeOutput(value: unknown, labels = LABELS): OutputLine[] {
  if (typeof value === "string") return value.trim() ? [{ label: "内容", text: value }] : [];
  if (!value || typeof value !== "object" || Array.isArray(value)) return [];
  return Object.entries(value).flatMap(([key, v]) => {
    if (key === "verdict" && typeof v === "string" && typeof VERDICTS[v] === "string") return [{ label: "审核结论", text: VERDICTS[v] }];
    const label = labels[key];
    if (typeof label !== "string" || v === null || v === undefined || v === "") return [];
    if (typeof v === "string" || typeof v === "number") return [{ label, text: String(v) }];
    if (typeof v === "boolean") return [{ label, text: v ? "是" : "否" }];
    if (Array.isArray(v) && v.length && v.every(item => typeof item === "string")) return [{ label, text: v.join("；") }];
    return [];
  });
}

export function lensFieldLabels(title: string): Record<string, string> {
  const spec = Object.values(CARD_REGISTRY).find(card => card.name === title);
  return Object.fromEntries(spec?.steps.flatMap(step => step.fields.map(field => [field.key, field.label])) ?? []);
}

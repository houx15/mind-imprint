import type { Task, CardInstance, CardSpec, CardStatus } from "@mind-imprint/contracts";

export type ProcessNodeType =
  | "task_root" | "sub_question" | "card_use"
  | "attempt" | "key_knowledge" | "concession" | "reflection";

export interface ProcessNode {
  id: string;
  type: ProcessNodeType;
  parent_id: string | null;
  title: string;
  sub?: string;
  ref_id?: string;
  card_id?: string;
  status?: CardStatus;
  at?: string;
}

function nodeTypeForCard(spec: CardSpec | undefined, card_id: string): ProcessNodeType {
  if (card_id === "concession" || spec?.id === "concession" || spec?.id === "steelman") return "concession";
  if (spec?.category?.includes("反身性")) return "reflection";
  return "card_use";
}

function firstNonEmptyValue(field_values: Record<string, unknown>): string | undefined {
  for (const step of Object.values(field_values)) {
    if (step && typeof step === "object") {
      for (const v of Object.values(step as Record<string, unknown>)) {
        if (typeof v === "string" && v.trim()) return v.trim();
      }
    }
  }
  return undefined;
}

function cardSub(card: CardInstance): string {
  if (card.status === "skipped") return "已跳过（已记录为信号）";
  if (card.status === "completed") {
    const summary = firstNonEmptyValue(card.field_values);
    return summary ? (summary.length > 40 ? summary.slice(0, 40) + "…" : summary) : "已完成";
  }
  return "进行中";
}

export function deriveProcessTree(opts: { task: Task; cards: CardInstance[]; registry: Record<string, CardSpec> }): ProcessNode[] {
  const { task, cards, registry } = opts;
  const root: ProcessNode = {
    id: task.id, type: "task_root", parent_id: null,
    title: task.title, sub: task.seed ?? undefined, at: task.created_at,
  };
  const cardNodes = [...cards]
    .sort((a, b) => (a.created_at < b.created_at ? -1 : a.created_at > b.created_at ? 1 : 0))
    .map((card): ProcessNode => {
      const spec = registry[card.card_id];
      return {
        id: card.id,
        type: nodeTypeForCard(spec, card.card_id),
        parent_id: task.id,
        title: spec?.name ?? card.card_id,
        sub: cardSub(card),
        ref_id: card.id,
        card_id: card.card_id,
        status: card.status,
        at: card.created_at,
      };
    });
  return [root, ...cardNodes];
}

import type { Message, CardInstance, CardSpec } from "@mind-imprint/contracts";
import { SummonCardCall, serializeCardForRefeed } from "@mind-imprint/contracts";

export interface AssembleEvalInputOptions {
  messages: Message[];
  cards: CardInstance[];
  registry: Record<string, CardSpec>;
}

/**
 * Pure function: assembles the evaluation LLM input text from a task's
 * conversation messages and card instances.
 *
 * Returns a plain string with two sections:
 *   ## 对话 — readable conversation transcript
 *   ## 工具卡 — per-card envelope dump (name + status + filled content + event_trace count)
 */
export function assembleEvalInput(opts: AssembleEvalInputOptions): string {
  const { messages, cards, registry } = opts;

  const parts: string[] = [];

  // ── Section 1: Conversation transcript ──────────────────────────────────

  parts.push("## 对话");
  parts.push("");

  for (const m of messages) {
    if (m.role === "user") {
      parts.push(`学生：${m.content}`);
    } else if (m.role === "assistant") {
      const parsed = SummonCardCall.safeParse(m.tool_call);
      if (parsed.success) {
        // Message carries a summon_card proposal
        parts.push(`陪练（建议工具卡）：${m.content}`);
      } else {
        parts.push(`陪练：${m.content}`);
      }
    }
    // system and tool messages are skipped in the eval transcript
  }

  // ── Section 2: Tool card envelopes ──────────────────────────────────────

  parts.push("");
  parts.push("## 工具卡");

  if (cards.length === 0) {
    parts.push("");
    parts.push("（本次对话未触发工具卡）");
  }

  for (const card of cards) {
    const spec = registry[card.card_id];
    const displayName = spec ? spec.name : card.card_id;
    const statusLabel = card.status === "skipped" ? "跳过" : card.status;

    parts.push("");
    parts.push(`【${displayName}】 ${statusLabel}`);

    if (card.status === "completed" || card.status === "skipped") {
      if (spec) {
        const payload = serializeCardForRefeed(spec, card);
        parts.push(JSON.stringify(payload));
      } else {
        // No spec in registry — emit a minimal envelope noting the status
        const fallback = {
          card_id: card.card_id,
          card_name: card.card_id,
          status: card.status,
        };
        if (card.status === "skipped") {
          parts.push(`（跳过，无规格信息）`);
        }
        parts.push(JSON.stringify(fallback));
      }
    }

    parts.push(`（共 ${card.event_trace.length} 个操作事件）`);
  }

  return parts.join("\n");
}

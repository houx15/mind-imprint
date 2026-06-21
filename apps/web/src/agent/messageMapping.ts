import type { Message, CardInstance, CardSpec } from "@mind-imprint/contracts";
import { SummonCardCall, serializeCardForRefeed } from "@mind-imprint/contracts";
import type { ChatMessage } from "../llm/types";

export interface BuildLlmMessagesOptions {
  systemPrompt: string;
  messages: Message[];
  cardById: (id: string) => CardInstance | undefined;
  specById: (cardId: string) => CardSpec | undefined;
}

/**
 * Pure function: replays the stored conversation into the ChatMessage[]
 * sent to the LLM, pairing each card proposal's tool_use with its tool_result.
 */
export function buildLlmMessages(opts: BuildLlmMessagesOptions): ChatMessage[] {
  const { systemPrompt, messages, cardById, specById } = opts;

  const result: ChatMessage[] = [{ role: "system", content: systemPrompt }];

  for (const m of messages) {
    if (m.role === "user") {
      result.push({ role: "user", content: m.content });
    } else if (m.role === "assistant") {
      const parsed = SummonCardCall.safeParse(m.tool_call);
      if (parsed.success) {
        const call = parsed.data;
        // Assistant message with a summon_card tool call
        result.push({
          role: "assistant",
          content: m.content,
          toolCalls: [{ id: call.id, name: "summon_card", args: call.args }],
        });
        // Look up the linked CardInstance
        const ci = cardById(call.card_instance_id);
        if (ci && (ci.status === "completed" || ci.status === "skipped")) {
          const spec = specById(ci.card_id)!;
          result.push({
            role: "tool",
            content: JSON.stringify(serializeCardForRefeed(spec, ci)),
            toolCallId: call.id,
          });
        }
        // If proposed/active (unresolved), append nothing — it's always the tail
      } else {
        // Plain assistant message (no tool_call)
        result.push({ role: "assistant", content: m.content });
      }
    } else if (m.role === "system") {
      result.push({ role: "system", content: m.content });
    }
  }

  return result;
}

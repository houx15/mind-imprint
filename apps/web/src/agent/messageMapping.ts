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
        // Look up the linked CardInstance to determine resolution status
        const ci = cardById(call.card_instance_id);
        if (ci && (ci.status === "completed" || ci.status === "skipped")) {
          // RESOLVED: emit tool_use + paired tool_result so the wire format is legal
          result.push({
            role: "assistant",
            content: m.content,
            toolCalls: [{ id: call.id, name: "summon_card", args: call.args }],
          });
          const spec = specById(ci.card_id)!;
          result.push({
            role: "tool",
            content: JSON.stringify(serializeCardForRefeed(spec, ci)),
            toolCallId: call.id,
          });
        } else {
          // UNRESOLVED (proposed/active): emit as plain coaching text — no toolCalls.
          // When the LLM is actually called an unresolved proposal is never the tail
          // (a user message follows it), so emitting tool_use without a tool_result
          // would make the message list wire-illegal for both OpenAI and Anthropic.
          result.push({ role: "assistant", content: m.content });
        }
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

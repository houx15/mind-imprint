import { describe, it, expect } from "vitest";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Message, CardInstance } from "@mind-imprint/contracts";
import { buildLlmMessages } from "./messageMapping";

const call = { id: "c1", name: "summon_card", args: { card_id: "sift_craap", reason: "r", nudge_text: "n" }, card_instance_id: "ci_1" };
function msg(p: Partial<Message>): Message {
  return { id: "m", task_id: "t", role: "user", content: "", tool_call: null, created_at: "2026-06-21T10:00:00.000Z", ...p };
}
function ci(status: CardInstance["status"]): CardInstance {
  return { id: "ci_1", card_id: "sift_craap", task_id: "t", parent_node_id: null, status, field_values: { sift: { stop: "x" } }, event_trace: [], rubric_tags: [], created_at: "2026-06-21T10:00:00.000Z", completed_at: null };
}
const specById = (id: string) => CARD_REGISTRY[id];

it("a completed proposal yields assistant tool_call + paired tool result", () => {
  const messages = [
    msg({ role: "user", content: "我看到一篇文章" }),
    msg({ role: "assistant", content: "先核来源？", tool_call: call }),
  ];
  const out = buildLlmMessages({ systemPrompt: "SYS", messages, cardById: () => ci("completed"), specById });
  expect(out[0]!).toEqual({ role: "system", content: "SYS" });
  expect(out[2]!.role).toBe("assistant");
  expect(out[2]!.toolCalls![0]!.id).toBe("c1");
  expect(out[3]!.role).toBe("tool");
  expect(out[3]!.toolCallId).toBe("c1");
  expect(JSON.parse(out[3]!.content).status).toBe("completed");
});

it("an unresolved proposal is the tail (no tool result appended)", () => {
  const messages = [msg({ role: "assistant", content: "先核来源？", tool_call: call })];
  const out = buildLlmMessages({ systemPrompt: "SYS", messages, cardById: () => ci("proposed"), specById });
  expect(out[out.length - 1]!.role).toBe("assistant");
  expect(out.some((m) => m.role === "tool")).toBe(false);
});

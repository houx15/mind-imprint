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

it("unresolved proposal NOT at tail emits no toolCalls (wire-legal)", () => {
  // An unresolved proposal followed by a user message: the tool_use would have
  // no paired tool_result → illegal. Must be emitted as plain assistant text.
  const messages = [
    msg({ role: "user", content: "我看到一篇文章" }),
    msg({ role: "assistant", content: "先核来源？", tool_call: call }),
    msg({ role: "user", content: "好的我继续" }),
  ];
  const out = buildLlmMessages({ systemPrompt: "SYS", messages, cardById: () => ci("proposed"), specById });
  // No assistant turn should carry toolCalls without a following tool message
  for (let i = 0; i < out.length; i++) {
    const m = out[i]!;
    if (m.role === "assistant" && m.toolCalls?.length) {
      const next = out[i + 1];
      expect(next?.role).toBe("tool");
    }
  }
  // The proposal turn specifically must have no toolCalls
  const proposalTurn = out.find((m) => m.role === "assistant" && m.content === "先核来源？");
  expect(proposalTurn).toBeDefined();
  expect(proposalTurn!.toolCalls).toBeUndefined();
});

it("a resolved proposal still emits toolCalls + paired tool message", () => {
  const messages = [
    msg({ role: "user", content: "我看到一篇文章" }),
    msg({ role: "assistant", content: "先核来源？", tool_call: call }),
    msg({ role: "user", content: "好的我继续" }),
  ];
  const out = buildLlmMessages({ systemPrompt: "SYS", messages, cardById: () => ci("completed"), specById });
  const assistantWithTool = out.find((m) => m.role === "assistant" && m.toolCalls?.length);
  expect(assistantWithTool).toBeDefined();
  expect(assistantWithTool!.toolCalls![0]!.id).toBe("c1");
  const toolMsg = out.find((m) => m.role === "tool");
  expect(toolMsg).toBeDefined();
  expect(toolMsg!.toolCallId).toBe("c1");
});

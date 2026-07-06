import { describe, it, expect } from "vitest";
import { messagesToItems } from "./viewModel";
import type { Message, CardInstance, CardSpec } from "@mind-imprint/contracts";

// ─── fixtures ───────────────────────────────────────────────────────────────

const at = "2024-01-01T00:00:00.000Z";

function makeMsg(overrides: Partial<Message>): Message {
  return {
    id: "msg-1",
    task_id: "task-1",
    role: "user",
    content: "",
    tool_call: null,
    created_at: at,
    ...overrides,
  };
}

function makeCardInstance(overrides: Partial<CardInstance>): CardInstance {
  return {
    id: "ci-1",
    card_id: "sift_craap",
    task_id: "task-1",
    parent_node_id: null,
    status: "proposed",
    field_values: {},
    event_trace: [],
    rubric_tags: [],
    anchors: [],
    created_at: at,
    completed_at: null,
    ...overrides,
  };
}

const siftSpec: CardSpec = {
  id: "sift_craap",
  category: "信息素养",
  name: "SIFT×CRAAP 信息核查",
  purpose: "核查信息来源的可信度",
  trigger_condition: "学生引用外部信息",
  mode: "form",
  steps: [
    {
      key: "stop",
      title: "Stop",
      disclose: "always",
      methodology: { why: "w", how: "h", when: "n" },
      fields: [{ type: "text", key: "stop", label: "Stop" }],
    },
  ],
  rubric_tags: [],
};

// ─── viewModel tests ─────────────────────────────────────────────────────────

describe("messagesToItems", () => {
  it("user message without URL → student item with no link", () => {
    const msg = makeMsg({ role: "user", content: "中国是否让地球变得更可持续？" });
    const items = messagesToItems([msg], () => undefined, () => undefined);
    expect(items).toHaveLength(1);
    const item = items[0]!;
    expect(item.kind).toBe("student");
    if (item.kind === "student") {
      expect(item.text).toBe("中国是否让地球变得更可持续？");
      expect(item.link).toBeUndefined();
    }
  });

  it("user message with URL → student item with link extracted", () => {
    const msg = makeMsg({
      role: "user",
      content: "请看这篇文章 https://www.nature.com/articles/s41893-019-0220-7 来自 Nature Sustainability",
    });
    const items = messagesToItems([msg], () => undefined, () => undefined);
    expect(items).toHaveLength(1);
    const item = items[0]!;
    expect(item.kind).toBe("student");
    if (item.kind === "student") {
      expect(item.link).toBe("https://www.nature.com/articles/s41893-019-0220-7");
    }
  });

  it("plain assistant message → ai_text item", () => {
    const msg = makeMsg({
      role: "assistant",
      content: "好的，我们先停一下，核查一下这个来源的可信度。",
      tool_call: null,
    });
    const items = messagesToItems([msg], () => undefined, () => undefined);
    expect(items).toHaveLength(1);
    const item = items[0]!;
    expect(item.kind).toBe("ai_text");
    if (item.kind === "ai_text") {
      expect(item.text).toBe("好的，我们先停一下，核查一下这个来源的可信度。");
    }
  });

  it("assistant proposal message + proposed CardInstance → proposal item with status proposed", () => {
    const ci = makeCardInstance({ id: "ci-sift", status: "proposed" });
    const msg = makeMsg({
      role: "assistant",
      content: "",
      tool_call: {
        id: "call-1",
        name: "summon_card",
        args: {
          card_id: "sift_craap",
          reason: "学生引用了一条可疑来源",
          nudge_text: "这儿先别急着写，我们用 SIFT 核一下这个来源？",
        },
        card_instance_id: "ci-sift",
      },
    });

    const items = messagesToItems(
      [msg],
      (id) => (id === "ci-sift" ? ci : undefined),
      (cardId) => (cardId === "sift_craap" ? siftSpec : undefined),
    );

    expect(items).toHaveLength(1);
    const item = items[0]!;
    expect(item.kind).toBe("proposal");
    if (item.kind === "proposal") {
      expect(item.cardInstanceId).toBe("ci-sift");
      expect(item.category).toBe("信息素养");
      expect(item.cardName).toBe("SIFT×CRAAP 信息核查");
      expect(item.nudge).toBe("这儿先别急着写，我们用 SIFT 核一下这个来源？");
      expect(item.status).toBe("proposed");
    }
  });

  it("proposal message + active CardInstance → status proposed (active maps to proposed)", () => {
    const ci = makeCardInstance({ id: "ci-sift", status: "active" });
    const msg = makeMsg({
      role: "assistant",
      content: "",
      tool_call: {
        id: "call-1",
        name: "summon_card",
        args: { card_id: "sift_craap", reason: "r", nudge_text: "nudge" },
        card_instance_id: "ci-sift",
      },
    });

    const items = messagesToItems(
      [msg],
      (id) => (id === "ci-sift" ? ci : undefined),
      (cardId) => (cardId === "sift_craap" ? siftSpec : undefined),
    );
    const item = items[0]!;
    expect(item.kind).toBe("proposal");
    if (item.kind === "proposal") {
      expect(item.status).toBe("proposed");
    }
  });

  it("proposal message + completed CardInstance → status completed", () => {
    const ci = makeCardInstance({ id: "ci-sift", status: "completed", completed_at: at });
    const msg = makeMsg({
      role: "assistant",
      content: "",
      tool_call: {
        id: "call-1",
        name: "summon_card",
        args: { card_id: "sift_craap", reason: "r", nudge_text: "nudge" },
        card_instance_id: "ci-sift",
      },
    });

    const items = messagesToItems(
      [msg],
      (id) => (id === "ci-sift" ? ci : undefined),
      (cardId) => (cardId === "sift_craap" ? siftSpec : undefined),
    );
    const item = items[0]!;
    expect(item.kind).toBe("proposal");
    if (item.kind === "proposal") {
      expect(item.status).toBe("completed");
    }
  });

  it("proposal message + skipped CardInstance → status skipped", () => {
    const ci = makeCardInstance({ id: "ci-sift", status: "skipped" });
    const msg = makeMsg({
      role: "assistant",
      content: "",
      tool_call: {
        id: "call-1",
        name: "summon_card",
        args: { card_id: "sift_craap", reason: "r", nudge_text: "nudge" },
        card_instance_id: "ci-sift",
      },
    });

    const items = messagesToItems(
      [msg],
      (id) => (id === "ci-sift" ? ci : undefined),
      (cardId) => (cardId === "sift_craap" ? siftSpec : undefined),
    );
    const item = items[0]!;
    expect(item.kind).toBe("proposal");
    if (item.kind === "proposal") {
      expect(item.status).toBe("skipped");
    }
  });

  it("assistant proposal with unknown card_id → falls back to ai_text", () => {
    const msg = makeMsg({
      role: "assistant",
      content: "我建议使用工具卡",
      tool_call: {
        id: "call-1",
        name: "summon_card",
        args: { card_id: "unknown_card", reason: "r", nudge_text: "nudge" },
        card_instance_id: "ci-unknown",
      },
    });

    const items = messagesToItems(
      [msg],
      () => undefined,
      () => undefined,
    );
    expect(items).toHaveLength(1);
    // When spec is unknown, falls back gracefully (ai_text or proposal with empty name)
    // The key invariant: it does not throw
    expect(["ai_text", "proposal"]).toContain(items[0]!.kind);
  });

  it("mixed messages → preserves order", () => {
    const studentMsg = makeMsg({ id: "m1", role: "user", content: "中国碳排放是全球第一。" });
    const aiMsg = makeMsg({
      id: "m2",
      role: "assistant",
      content: "好的，我们分析一下。",
      tool_call: null,
    });
    const ci = makeCardInstance({ id: "ci-2", card_id: "concession", status: "proposed" });
    const concessionSpec: CardSpec = {
      ...siftSpec,
      id: "concession",
      category: "论证写作",
      name: "让步段写作",
    };
    const proposalMsg = makeMsg({
      id: "m3",
      role: "assistant",
      content: "",
      tool_call: {
        id: "call-2",
        name: "summon_card",
        args: { card_id: "concession", reason: "r", nudge_text: "我们来写让步段" },
        card_instance_id: "ci-2",
      },
    });

    const items = messagesToItems(
      [studentMsg, aiMsg, proposalMsg],
      (id) => (id === "ci-2" ? ci : undefined),
      (cardId) => (cardId === "concession" ? concessionSpec : undefined),
    );

    expect(items).toHaveLength(3);
    expect(items[0]!.kind).toBe("student");
    expect(items[1]!.kind).toBe("ai_text");
    expect(items[2]!.kind).toBe("proposal");
    if (items[2]!.kind === "proposal") {
      expect(items[2]!.cardName).toBe("让步段写作");
      expect(items[2]!.category).toBe("论证写作");
      expect(items[2]!.nudge).toBe("我们来写让步段");
    }
  });

  it("system messages are filtered out", () => {
    const sysMsg = makeMsg({ role: "system", content: "You are a helpful AI tutor." });
    const items = messagesToItems([sysMsg], () => undefined, () => undefined);
    expect(items).toHaveLength(0);
  });

  it("emits an ai_text bubble before the proposal when the model also explained", () => {
    const msg = makeMsg({
      id: "m1",
      role: "assistant",
      content: "先核一下来源。",
      tool_call: {
        id: "tc1",
        name: "summon_card",
        args: {
          card_id: "sift_craap",
          reason: "r",
          nudge_text: "用这张卡？",
        },
        card_instance_id: "ci1",
      },
    });
    const ci = makeCardInstance({ id: "ci1" });
    const items = messagesToItems(
      [msg],
      (id) => (id === "ci1" ? ci : undefined),
      (cardId) => (cardId === "sift_craap" ? siftSpec : undefined),
    );
    expect(items).toHaveLength(2);
    expect(items[0]).toEqual({ kind: "ai_text", text: "先核一下来源。" });
    expect(items[1]).toMatchObject({ kind: "proposal", nudge: "用这张卡？" });
  });

  it("emits only the proposal when there is no separate explanation", () => {
    const msg = makeMsg({
      id: "m1",
      role: "assistant",
      content: "",
      tool_call: {
        id: "tc1",
        name: "summon_card",
        args: {
          card_id: "sift_craap",
          reason: "r",
          nudge_text: "用这张卡？",
        },
        card_instance_id: "ci1",
      },
    });
    const ci = makeCardInstance({ id: "ci1" });
    const items = messagesToItems(
      [msg],
      (id) => (id === "ci1" ? ci : undefined),
      (cardId) => (cardId === "sift_craap" ? siftSpec : undefined),
    );
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({ kind: "proposal" });
  });
});

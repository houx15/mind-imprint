import { describe, it, expect } from "vitest";
import type { Message, CardInstance, CardSpec } from "@mind-imprint/contracts";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { assembleEvalInput } from "./evalInput";

// ── helpers ─────────────────────────────────────────────────────────────────

function msg(p: Partial<Message>): Message {
  return {
    id: "m1",
    task_id: "task_phoebe",
    role: "user",
    content: "",
    tool_call: null,
    created_at: "2026-06-20T09:00:00.000Z",
    ...p,
  };
}

function makeCard(
  status: CardInstance["status"],
  fieldValues: Record<string, unknown> = {}
): CardInstance {
  return {
    id: "ci_sift_1",
    card_id: "sift_craap",
    task_id: "task_phoebe",
    parent_node_id: null,
    status,
    field_values: fieldValues,
    event_trace: [
      { kind: "step_expand", step_key: "sift", at: "2026-06-20T09:01:00.000Z" },
      { kind: "field_change", path: "sift.stop", at: "2026-06-20T09:02:00.000Z" },
      { kind: "submit", at: "2026-06-20T09:05:00.000Z" },
    ],
    rubric_tags: ["D1_来源意识"],
    created_at: "2026-06-20T09:00:00.000Z",
    completed_at: "2026-06-20T09:05:00.000Z",
  };
}

// Real Phoebe transcript fixtures
const PHOEBE_MESSAGES: Message[] = [
  msg({
    id: "m1",
    role: "user",
    content:
      "我在研究[中国是否让地球变得更可持续]，找到一篇文章说中国在可再生能源方面领先全球，我想直接引用它。",
  }),
  msg({
    id: "m2",
    role: "assistant",
    content:
      "这个问题很重要。在引用前，我们先来核一下这个来源的可信度——你知道这篇文章是哪里发布的吗？",
  }),
  msg({
    id: "m3",
    role: "user",
    content:
      "是一篇 NASA 的报告，还有 Nature Sustainability 上的一篇论文，我找到了两个来源。",
  }),
  msg({
    id: "m4",
    role: "assistant",
    content: "很好，NASA 和 Nature Sustainability 都是权威来源。你有没有注意到发布时间？",
  }),
];

// Completed sift_craap card with NASA content
const COMPLETED_SIFT_CARD = makeCard("completed", {
  sift: {
    stop: "中国可再生能源占全球领先，想引用说明中国政策在可持续发展上的正面影响",
    sources: [
      { name: "NASA Earth Observatory", type: "官方", verdict: "可信" },
      { name: "Nature Sustainability 2023", type: "学者/机构", verdict: "可信" },
      { name: "CCTV 国际频道", type: "主流媒体", verdict: "存疑" },
    ],
    better: "NASA 和 Nature Sustainability 的数据一致，确认中国太阳能装机容量全球第一",
    trace: "https://earthobservatory.nasa.gov/images/china-renewable",
  },
});

// Skipped card fixture
const SKIPPED_CARD: CardInstance = {
  id: "ci_skip_1",
  card_id: "sift_craap",
  task_id: "task_phoebe",
  parent_node_id: null,
  status: "skipped",
  field_values: {},
  event_trace: [
    { kind: "skip", at: "2026-06-20T09:03:00.000Z" },
  ],
  rubric_tags: [],
  created_at: "2026-06-20T09:02:00.000Z",
  completed_at: null,
};

const registry: Record<string, CardSpec> = CARD_REGISTRY;

// ── tests ────────────────────────────────────────────────────────────────────

describe("assembleEvalInput", () => {
  it("includes a 学生：line and an 陪练：line from the transcript", () => {
    const output = assembleEvalInput({
      messages: PHOEBE_MESSAGES,
      cards: [],
      registry,
    });

    expect(output).toContain("学生：");
    expect(output).toContain("陪练：");
    expect(output).toContain("NASA");
  });

  it("contains the ## 对话 section header", () => {
    const output = assembleEvalInput({
      messages: PHOEBE_MESSAGES,
      cards: [],
      registry,
    });

    expect(output).toContain("## 对话");
  });

  it("contains the ## 工具卡 section header", () => {
    const output = assembleEvalInput({
      messages: PHOEBE_MESSAGES,
      cards: [],
      registry,
    });

    expect(output).toContain("## 工具卡");
  });

  it("includes the card name and filled NASA value for a completed sift_craap card", () => {
    const output = assembleEvalInput({
      messages: PHOEBE_MESSAGES,
      cards: [COMPLETED_SIFT_CARD],
      registry,
    });

    // Card name from spec
    expect(output).toContain("SIFT×CRAAP 信息核查");
    // A filled value from the card (NASA source)
    expect(output).toContain("NASA");
    // Event trace count note
    expect(output).toContain("3 个操作事件");
  });

  it("includes 跳过 for a skipped card", () => {
    const output = assembleEvalInput({
      messages: PHOEBE_MESSAGES,
      cards: [SKIPPED_CARD],
      registry,
    });

    expect(output).toContain("跳过");
  });

  it("falls back to card_id when spec is missing from registry", () => {
    const output = assembleEvalInput({
      messages: [],
      cards: [{ ...SKIPPED_CARD, card_id: "nonexistent_card_xyz" }],
      registry,
    });

    expect(output).toContain("nonexistent_card_xyz");
  });

  it("skips system messages in the conversation section", () => {
    const messagesWithSystem: Message[] = [
      msg({ id: "sys1", role: "system", content: "You are a tutor." }),
      msg({ id: "u1", role: "user", content: "Phoebe 的问题" }),
    ];

    const output = assembleEvalInput({
      messages: messagesWithSystem,
      cards: [],
      registry,
    });

    // System message content should not appear as 学生：or 陪练：
    expect(output).not.toContain("学生：You are a tutor.");
    expect(output).not.toContain("陪练：You are a tutor.");
    // User message should appear
    expect(output).toContain("学生：Phoebe 的问题");
  });
});

import { describe, it, expect } from "vitest";
import { deriveCardUsage } from "./cardUsage";
import type { CardInstance, CardSpec } from "@mind-imprint/contracts";

const registry = {
  sift_craap: { id: "sift_craap", name: "SIFT×CRAAP", category: "信息素养" },
  concession: { id: "concession", name: "让步段", category: "知识工具" },
} as unknown as Record<string, CardSpec>;

function ci(card_id: string, id: string): CardInstance {
  return { id, card_id, task_id: "t1", parent_node_id: null, status: "completed",
    field_values: {}, event_trace: [], rubric_tags: [], anchors: [], created_at: "2026-06-01T00:00:00Z", completed_at: null };
}

describe("deriveCardUsage", () => {
  it("empty → []", () => {
    expect(deriveCardUsage([], registry)).toEqual([]);
  });
  it("groups used cards by category with counts + badges", () => {
    const groups = deriveCardUsage(
      [ci("sift_craap", "a"), ci("sift_craap", "b"), ci("sift_craap", "c"), ci("concession", "d")],
      registry,
    );
    const info = groups.find((g) => g.name === "信息素养")!;
    expect(info.cards[0]!.name).toBe("SIFT×CRAAP");
    expect(info.cards[0]!.usage).toBe("用过 3 次");
    expect(info.cards[0]!.badge).toBe("常用");
    const know = groups.find((g) => g.name === "知识工具")!;
    expect(know.cards[0]!.badge).toBe("已用");
  });
  it("skips unknown card_ids without throwing", () => {
    expect(deriveCardUsage([ci("ghost", "x")], registry)).toEqual([]);
  });
});

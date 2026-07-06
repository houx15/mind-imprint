import { describe, it, expect } from "vitest";
import { serializeCardForRefeed } from "../src/refeed";
import { CARD_REGISTRY } from "../src/registry";
import type { CardInstance } from "../src/envelope";

const sift = CARD_REGISTRY.sift_craap!;

function inst(status: CardInstance["status"], field_values: Record<string, unknown>): CardInstance {
  return {
    id: "ci_1", card_id: "sift_craap", task_id: "t_1", parent_node_id: null,
    status, field_values, event_trace: [], rubric_tags: [], anchors: [],
    created_at: "2026-06-21T10:00:00.000Z", completed_at: null,
  };
}

describe("serializeCardForRefeed", () => {
  it("skipped → only identity + status, no steps", () => {
    const p = serializeCardForRefeed(sift, inst("skipped", {}));
    expect(p).toEqual({ card_id: "sift_craap", card_name: sift.name, status: "skipped" });
  });

  it("completed → labels paired with field_values, repeatable_group remapped to label:value", () => {
    const p = serializeCardForRefeed(sift, inst("completed", {
      sift: {
        stop: "证明中国让地球更可持续",
        sources: [
          { name: "NASA", type: "官方", verdict: "可信" },
          { name: "Nature Sustainability", type: "学者/机构", verdict: "可信" },
        ],
        better: "原始研究来自 NASA / Nature Sustainability",
        trace: "https://www.nature.com/...",
      },
    }));
    expect(p.card_id).toBe("sift_craap");
    expect(p.status).toBe("completed");
    const siftStep = p.steps!.find((s) => s.title === sift.steps[0]!.title)!;
    // the Stop textarea answer is paired with its label
    expect(siftStep.answers.find((a) => a.label.startsWith("Stop"))!.value).toBe("证明中国让地球更可持续");
    // the repeatable_group is an array of {item-label: value}
    const sources = siftStep.answers.find((a) => a.label.startsWith("Investigate"))!.value as Array<Record<string, unknown>>;
    expect(sources[0]).toEqual({ "来源": "NASA", "类型": "官方", "可信？": "可信" });
  });

  it("omits unfilled fields (no empty answers)", () => {
    const p = serializeCardForRefeed(sift, inst("completed", { sift: { stop: "x" } }));
    const siftStep = p.steps!.find((s) => s.title === sift.steps[0]!.title)!;
    expect(siftStep.answers.every((a) => a.value !== undefined && a.value !== "")).toBe(true);
  });
});

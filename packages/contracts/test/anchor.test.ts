import { describe, it, expect } from "vitest";
import { Anchor } from "../src/anchor";
import { CardInstance } from "../src/envelope";
import { CardSpec } from "../src/cardSpec";

const anchor = {
  id: "a0", material_id: "m1", block_id: "b0", start: 3, end: 9,
  quote: "美航局发现", dimension: "权威性 · Authority", author: "ai",
  question: "这处「美航局发现」——转载者是权威吗？", answer: "",
};

describe("Anchor + envelope/spec extensions", () => {
  it("parses a valid anchor", () => {
    expect(Anchor.parse(anchor)).toMatchObject({ id: "a0", author: "ai" });
  });
  it("rejects an unknown author", () => {
    expect(Anchor.safeParse({ ...anchor, author: "teacher" }).success).toBe(false);
  });
  it("CardInstance.anchors defaults to [] when omitted", () => {
    const ci = CardInstance.parse({
      id: "c1", card_id: "sift_craap", task_id: "t1", parent_node_id: null,
      status: "proposed", field_values: {}, event_trace: [], rubric_tags: [],
      created_at: "1", completed_at: null,
    });
    expect(ci.anchors).toEqual([]);
  });
  it("CardInstance carries anchors when present", () => {
    const ci = CardInstance.parse({
      id: "c1", card_id: "sift_craap", task_id: "t1", parent_node_id: null,
      status: "completed", field_values: {}, event_trace: [], rubric_tags: [],
      anchors: [anchor], created_at: "1", completed_at: "2",
    });
    expect(ci.anchors).toHaveLength(1);
  });
  it("CardSpec.mode defaults to form", () => {
    const spec = CardSpec.parse({
      id: "x", category: "c", name: "n", purpose: "p", trigger_condition: "t",
      steps: [{ key: "s", title: "T", disclose: "always", methodology: { why: "w", how: "h", when: "n" }, fields: [{ type: "text", key: "f", label: "L" }] }],
      rubric_tags: [],
    });
    expect(spec.mode).toBe("form");
  });
});

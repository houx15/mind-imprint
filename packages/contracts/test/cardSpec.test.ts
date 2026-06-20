import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";

const valid = {
  id: "concession", category: "知识工具", name: "让步段", purpose: "以退为进",
  trigger_condition: "出现反例却想忽略", rubric_tags: ["D5_论证结构"],
  steps: [{
    key: "concession", title: "让步段四步", disclose: "always", methodology_note: "先退一步再反驳",
    fields: [{ type: "text", key: "thesis", label: "中心论点" }],
  }],
};

describe("CardSpec", () => {
  it("accepts a valid card", () => {
    expect(CardSpec.safeParse(valid).success).toBe(true);
  });
  it("rejects an invalid disclose value", () => {
    const bad = { ...valid, steps: [{ ...valid.steps[0], disclose: "sometimes" }] };
    expect(CardSpec.safeParse(bad).success).toBe(false);
  });
  it("rejects a step with no fields", () => {
    const bad = { ...valid, steps: [{ ...valid.steps[0], fields: [] }] };
    expect(CardSpec.safeParse(bad).success).toBe(false);
  });
});

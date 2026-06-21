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

const baseStep = { key: "s", title: "T", disclose: "always", methodology_note: "", fields: [{ type: "textarea", key: "x", label: "L" }] };
const minimal = { id: "c", category: "信息素养", name: "n", purpose: "", trigger_condition: "", steps: [baseStep], rubric_tags: [] };

describe("CardSpec metadata extension", () => {
  it("still accepts a card with no metadata (back-compat)", () => {
    expect(CardSpec.safeParse(minimal).success).toBe(true);
  });
  it("accepts full routing metadata", () => {
    const withMeta = { ...minimal, name_en: "N", priority: "P0", disclosure_tier: "tier-0",
      age_band: ["MYP","DP"], trigger_keywords: ["a"], interaction_type: "步骤引导卡",
      rubric_dims: ["D1"], related: ["other"], body_status: "stub" };
    expect(CardSpec.safeParse(withMeta).success).toBe(true);
  });
  it("rejects an unknown priority", () => {
    expect(CardSpec.safeParse({ ...minimal, priority: "P9" }).success).toBe(false);
  });
  it("rejects an unknown interaction_type", () => {
    expect(CardSpec.safeParse({ ...minimal, interaction_type: "全息卡" }).success).toBe(false);
  });
  it("rejects an unknown body_status", () => {
    expect(CardSpec.safeParse({ ...minimal, body_status: "draft" }).success).toBe(false);
  });
});

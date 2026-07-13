import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import craap from "../cards/craap.json";
import sift from "../cards/sift.json";
import moneyTrail from "../cards/money-trail.json";
import spinDetector from "../cards/spin-detector.json";
import multimodalDecode from "../cards/multimodal-decode.json";
import cda from "../cards/cda.json";

const batch: Record<string, unknown> = {
  "craap": craap,
  "sift": sift,
  "money-trail": moneyTrail,
  "spin-detector": spinDetector,
  "multimodal-decode": multimodalDecode,
  "cda": cda,
};

describe("信息素养 cards", () => {
  for (const [key, raw] of Object.entries(batch)) {
    it(`${key} is a valid CardSpec with routing metadata`, () => {
      const parsed = CardSpec.safeParse(raw);
      expect(parsed.success).toBe(true);
      if (!parsed.success) return;
      expect(parsed.data.id).toBe(key);
      expect(parsed.data.priority).toBeDefined();
      expect(parsed.data.disclosure_tier).toBeDefined();
      expect(parsed.data.interaction_type).toBeDefined();
      expect(parsed.data.trigger_keywords?.length).toBeGreaterThan(0);
    });
  }
});

describe("sift card (C2 over compare)", () => {
  it("validates against the C2 card spec", () => {
    expect(() => CardSpec.parse(sift)).not.toThrow();
  });

  it("binds the compare primitive and declares which dimension carries the lateral source", () => {
    expect(sift.primitive).toBe("compare");
    expect(sift.params.lateral_dimension).toBe("find");
  });

  it("requires a real lateral source and a student-written trace — not a claim of having read laterally", () => {
    expect(sift.completion).toContainEqual({ kind: "lateral_source_present" });
    expect(sift.completion).toContainEqual({ kind: "field_written_by", field: "trace_origin", author: "student" });
  });

  it("mints a cross_check and never promotes the lateral source to evidence", () => {
    expect(sift.graph_effects).toEqual([{ kind: "cross_check" }]);
    expect(JSON.stringify(sift.graph_effects)).not.toContain("promote");
  });

  it("offers the relation as the student's own closed choice — the AI never picks it", () => {
    const relation = sift.steps.flatMap((s) => s.fields).find((f) => f.key === "relation");
    expect(relation?.type).toBe("single_choice");
    expect(relation?.options).toEqual(["印证", "反驳", "限定"]);
  });
});

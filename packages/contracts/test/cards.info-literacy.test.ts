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

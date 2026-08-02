import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import factOpinionValue from "../cards/fact-opinion-value.json";
import argumentMap from "../cards/argument-map.json";
import pee from "../cards/pee.json";
import dataLiteracy from "../cards/data-literacy.json";

const batch: Record<string, unknown> = {
  "fact-opinion-value": factOpinionValue,
  "argument-map": argumentMap,
  pee,
  "data-literacy": dataLiteracy,
};

describe("知识工具 (Knowledge Tools) cards", () => {
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

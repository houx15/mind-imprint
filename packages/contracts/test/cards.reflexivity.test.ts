import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import knowerPerspective from "../cards/knower-perspective.json";
import metacognition from "../cards/metacognition.json";
import checkpoint from "../cards/checkpoint.json";

const batch = {
  "knower-perspective": knowerPerspective,
  "metacognition": metacognition,
  "checkpoint": checkpoint,
};

describe("反身性与元认知 cards", () => {
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

import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import aiBoundary from "../cards/ai-boundary.json";
import ethicsLenses from "../cards/ethics-lenses.json";
import aiDecisionTree from "../cards/ai-decision-tree.json";

const batch = {
  "ai-boundary": aiBoundary,
  "ethics-lenses": ethicsLenses,
  "ai-decision-tree": aiDecisionTree,
};

describe("AI伦理 cards", () => {
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

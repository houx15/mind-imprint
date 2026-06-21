import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import aiCollaboration from "../cards/ai-collaboration.json";
import aiBoundary from "../cards/ai-boundary.json";
import ethicsLenses from "../cards/ethics-lenses.json";
import ethicsRoleplay from "../cards/ethics-roleplay.json";
import aiDecisionTree from "../cards/ai-decision-tree.json";

const batch = {
  "ai-collaboration": aiCollaboration,
  "ai-boundary": aiBoundary,
  "ethics-lenses": ethicsLenses,
  "ethics-roleplay": ethicsRoleplay,
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

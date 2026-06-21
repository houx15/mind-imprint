import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import questionCard from "../cards/question-card.json";
import rabbitHole from "../cards/rabbit-hole.json";
import emotionalAlignment from "../cards/emotional-alignment.json";

const batch = {
  "question-card": questionCard,
  "rabbit-hole": rabbitHole,
  "emotional-alignment": emotionalAlignment,
};

describe("探究启动 cards", () => {
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

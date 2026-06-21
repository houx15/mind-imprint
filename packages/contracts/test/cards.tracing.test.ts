import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import sourceMap from "../cards/source-map.json";
import beliefSpectrum from "../cards/belief-spectrum.json";
import corpusHook from "../cards/corpus-hook.json";

const batch = {
  "source-map": sourceMap,
  "belief-spectrum": beliefSpectrum,
  "corpus-hook": corpusHook,
};

describe("溯源与多视角 cards", () => {
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

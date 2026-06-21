import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import siftCraap from "../cards/sift_craap.json";
import concession from "../cards/concession.json";

describe("demo cards carry routing metadata", () => {
  for (const [key, raw] of Object.entries({ sift_craap: siftCraap, concession })) {
    it(`${key} is valid with metadata + body_status full`, () => {
      const p = CardSpec.safeParse(raw);
      expect(p.success).toBe(true);
      if (!p.success) return;
      expect(p.data.priority).toBeDefined();
      expect(p.data.disclosure_tier).toBeDefined();
      expect(p.data.interaction_type).toBeDefined();
      expect(p.data.trigger_keywords?.length).toBeGreaterThan(0);
      expect(p.data.body_status).toBe("full");
    });
  }
});

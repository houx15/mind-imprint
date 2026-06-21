import { describe, it, expect } from "vitest";
import { CARD_REGISTRY, deriveCatalog } from "../src/index";

describe("card library is complete", () => {
  it("registry holds all 33 cards (31 library + 2 demo)", () => {
    expect(Object.keys(CARD_REGISTRY).length).toBe(33);
  });

  it("every card carries core routing metadata and a valid body_status", () => {
    for (const card of Object.values(CARD_REGISTRY)) {
      expect(card.priority, card.id).toBeDefined();
      expect(card.disclosure_tier, card.id).toBeDefined();
      expect(card.interaction_type, card.id).toBeDefined();
      expect((card.trigger_keywords ?? []).length, card.id).toBeGreaterThan(0);
      if (card.body_status !== undefined) {
        expect(["full", "stub"]).toContain(card.body_status);
      }
    }
  });

  it("catalog projects all 33 cards", () => {
    expect(deriveCatalog(CARD_REGISTRY).length).toBe(33);
  });

  it("the 2 demo cards and key library cards coexist", () => {
    for (const id of ["sift_craap", "concession", "sift", "craap", "steelman"]) {
      expect(CARD_REGISTRY[id], id).toBeDefined();
    }
  });
});

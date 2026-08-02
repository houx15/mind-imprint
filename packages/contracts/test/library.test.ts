import { describe, it, expect } from "vitest";
import { CARD_REGISTRY, deriveCatalog } from "../src/index";

describe("card library is complete", () => {
  it("registry holds all 34 cards (24 library + 1 demo + 9 reading lenses)", () => {
    expect(Object.keys(CARD_REGISTRY).length).toBe(34);
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

  it("all cards are fully built — no stubs remain (S3c complete)", () => {
    const stubs = Object.values(CARD_REGISTRY).filter((c) => c.body_status === "stub").map((c) => c.id);
    expect(stubs).toEqual([]);
  });

  it("catalog projects all 34 cards", () => {
    expect(deriveCatalog(CARD_REGISTRY).length).toBe(34);
  });

  it("the demo card and key library cards coexist", () => {
    for (const id of ["concession", "sift", "craap", "toulmin"]) {
      expect(CARD_REGISTRY[id], id).toBeDefined();
    }
  });
});

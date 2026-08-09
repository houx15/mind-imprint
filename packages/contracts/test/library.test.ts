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

describe("card interaction taxonomy (re-catalog 2026-08-09)", () => {
  it("question-card is sub-agent, learning-report is function, others default to form", () => {
    expect(CARD_REGISTRY["question-card"].interaction).toBe("sub-agent");
    expect(CARD_REGISTRY["learning-report"].interaction).toBe("function");
    expect(CARD_REGISTRY["pee"].interaction ?? "form").toBe("form");
  });

  it("every card carries a valid placement tag", () => {
    const valid = new Set(["status", "reading", "reading-toolkit", "cross-cutting"]);
    for (const card of Object.values(CARD_REGISTRY)) {
      expect(valid.has(card.placement ?? ""), `${card.id}: ${card.placement}`).toBe(true);
    }
  });

  it("catalog projects placement + interaction for all 34 cards", () => {
    const cat = deriveCatalog(CARD_REGISTRY);
    expect(cat.length).toBe(34);
    const qc = cat.find((c) => c.id === "question-card")!;
    expect(qc.interaction).toBe("sub-agent");
    expect(qc.placement).toBe("status");
    const cda = cat.find((c) => c.id === "cda")!;
    expect(cda.placement).toBe("reading-toolkit");
  });
});

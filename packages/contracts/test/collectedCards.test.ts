import { describe, it, expect } from "vitest";
import { CollectedCards } from "../src/collectedCards";

describe("CollectedCards", () => {
  it("parses a representative payload", () => {
    const parsed = CollectedCards.parse({
      cards: [
        { cardId: "concession", uses: 2, surfaces: ["project"], lastUsed: "2026-07-18T00:00:00Z" },
        { cardId: "opcvl", uses: 1, surfaces: ["course"], lastUsed: "2026-07-17T00:00:00Z" },
      ],
    });
    expect(parsed.cards.length).toBe(2);
    expect(parsed.cards[0]!.uses).toBe(2);
  });

  it("rejects a card missing a required field", () => {
    expect(() => CollectedCards.parse({ cards: [{ cardId: "x", uses: 1, surfaces: [] }] })).toThrow();
  });
});

import { describe, it, expect } from "vitest";
import { readingDeck, groupReadingDeck, READING_DECK_IDS } from "@/studio/reading/readingDeck";

describe("readingDeck", () => {
  it("is source-check (craap, sift) + the 9 disciplinary lenses", () => {
    expect(READING_DECK_IDS).toEqual([
      "craap",
      "sift",
      "lens-logic",
      "lens-methods",
      "lens-society",
      "lens-law",
      "lens-economics",
      "lens-ethics",
      "lens-history",
      "lens-communication",
      "lens-systems",
    ]);
  });

  it("resolves every deck id against the registry (no drift)", () => {
    // readingDeck() skips ids missing from the registry — so a length short of
    // READING_DECK_IDS means an id drifted out of the contract.
    expect(readingDeck()).toHaveLength(READING_DECK_IDS.length);
  });

  it("every deep-reading entry carries a lens family; source-check does not", () => {
    const deck = readingDeck();
    for (const e of deck) {
      if (e.id === "craap" || e.id === "sift") {
        expect(e.family).toBeUndefined();
      } else {
        expect(e.family).toBeDefined();
      }
    }
  });

  it("groups into 信源体检 + the three lens families in display order", () => {
    const { sourceCheck, families } = groupReadingDeck();
    expect(sourceCheck.map((e) => e.id)).toEqual(["craap", "sift"]);
    expect(families.map((g) => g.key)).toEqual(["reasoning", "institutions", "context"]);
    expect(families.map((g) => g.label)).toEqual(["推理与证据", "人与制度", "语境与系统"]);
    // 2 reasoning + 4 institutions + 3 context = 9 lenses.
    expect(families.map((g) => g.entries.length)).toEqual([2, 4, 3]);
  });
});

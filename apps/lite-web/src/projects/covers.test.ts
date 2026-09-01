import { describe, expect, it } from "vitest";
import { PROJECT_KINDS } from "../api/projects";
import { COVER_GLYPHS, COVER_GROUNDS, defaultCover, groundById, resolveCover } from "./covers";

describe("cover vocabulary", () => {
  it("offers the full 8 × 16 set with no duplicates", () => {
    expect(COVER_GROUNDS).toHaveLength(8);
    expect(COVER_GLYPHS).toHaveLength(16);
    expect(new Set(COVER_GROUNDS.map((g) => g.id)).size).toBe(8);
    expect(new Set(COVER_GLYPHS).size).toBe(16);
  });

  it("proposes a cover for every kind, drawn from the vocabulary", () => {
    for (const kind of PROJECT_KINDS) {
      const c = defaultCover(kind);
      expect(COVER_GROUNDS.map((g) => g.id)).toContain(c.ground);
      expect(COVER_GLYPHS).toContain(c.glyph);
    }
  });

  it("gives different kinds different grounds, so a board is not one colour", () => {
    const grounds = new Set(PROJECT_KINDS.map((k) => defaultCover(k).ground));
    expect(grounds.size).toBeGreaterThan(1);
  });
});

// The server stores whatever string it is sent — it does not police this
// vocabulary. So an id the client has never heard of must still render.
describe("unknown values still render", () => {
  it("falls back to the first ground rather than nothing", () => {
    expect(groundById("chartreuse")).toEqual(COVER_GROUNDS[0]);
    expect(groundById("")).toEqual(COVER_GROUNDS[0]);
  });

  it("keeps a known ground", () => {
    expect(groundById("matcha").id).toBe("matcha");
  });

});

// This is the regression guard for a bug the unit tests could not have caught
// and a screenshot did: an uncovered project rendered coral on the board and
// blue in the modal, so the card changed colour the instant she saved. Both
// surfaces now resolve through resolveCover, so the invariant is that an EMPTY
// cover resolves to exactly the same thing defaultCover proposes.
describe("resolveCover is the single fallback", () => {
  it("resolves an empty cover to the kind's default, not to the first ground", () => {
    for (const kind of PROJECT_KINDS) {
      const fallback = defaultCover(kind);
      const got = resolveCover(kind, "", "");
      expect(got.ground.id).toBe(fallback.ground);
      expect(got.glyph).toBe(fallback.glyph);
    }
  });

  it("treats whitespace as empty", () => {
    const got = resolveCover("research", "   ", "  ");
    expect(got.ground.id).toBe(defaultCover("research").ground);
    expect(got.glyph).toBe(defaultCover("research").glyph);
  });

  it("keeps what she actually chose", () => {
    const got = resolveCover("research", "matcha", "✿");
    expect(got.ground.id).toBe("matcha");
    expect(got.glyph).toBe("✿");
  });

  it("still survives a ground id it has never heard of", () => {
    expect(resolveCover("design", "chartreuse", "✿").ground).toEqual(COVER_GROUNDS[0]);
  });
});

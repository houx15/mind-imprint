import { describe, it, expect } from "vitest";
import { pickTeaching, teachingRegistry } from "@/cards/teaching/teachingRegistry";

describe("pickTeaching", () => {
  it("returns undefined for cards without a teaching module", () => {
    expect(pickTeaching("nonexistent")).toBeUndefined();
  });

  it("registry is keyed by cardId", () => {
    for (const [k, v] of Object.entries(teachingRegistry)) {
      expect(v.cardId).toBe(k);
    }
  });
});

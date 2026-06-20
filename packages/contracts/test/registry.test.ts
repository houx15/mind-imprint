import { describe, it, expect } from "vitest";
import { loadRegistry, deriveCatalog } from "../src/registry";

describe("loadRegistry", () => {
  it("loads both bundled cards", () => {
    const reg = loadRegistry();
    expect(Object.keys(reg).sort()).toEqual(["concession", "sift_craap"]);
  });
  it("throws with the card id when a card is invalid", () => {
    const broken = { sift_craap: { id: "sift_craap", category: "信息素养" } };
    expect(() => loadRegistry(broken)).toThrow(/sift_craap/);
  });
  it("throws when the map key does not match card.id", () => {
    const reg = loadRegistry();
    const mismatched = { wrong_key: reg.sift_craap };
    expect(() => loadRegistry(mismatched as never)).toThrow(/does not match/);
  });
});

describe("deriveCatalog", () => {
  it("projects one trigger_condition line per card and nothing stale", () => {
    const cat = deriveCatalog(loadRegistry());
    expect(cat).toHaveLength(2);
    expect(cat.every((c) => typeof c.trigger_condition === "string" && c.trigger_condition.length > 0)).toBe(true);
    expect(Object.keys(cat[0]!).sort()).toEqual(["category", "id", "name", "trigger_condition"]);
  });
});

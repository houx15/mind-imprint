import { describe, it, expect } from "vitest";
import { loadRegistry, deriveCatalog } from "../src/registry";

describe("loadRegistry", () => {
  it("loads the full registry including the 2 demo cards", () => {
    const reg = loadRegistry();
    expect(Object.keys(reg)).toHaveLength(33);
    expect(reg.sift_craap).toBeDefined();
    expect(reg.concession).toBeDefined();
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
    expect(cat).toHaveLength(33);
    expect(cat.every((c) => typeof c.trigger_condition === "string" && c.trigger_condition.length > 0)).toBe(true);
    expect(Object.keys(cat[0]!).sort()).toEqual(["category", "disclosure_tier", "id", "interaction_type", "name", "priority", "trigger_condition", "trigger_keywords"]);
  });
  it("deriveCatalog projects routing metadata", () => {
    const cat = deriveCatalog(loadRegistry());
    const sift = cat.find((c) => c.id === "sift_craap");
    expect(sift).toBeDefined();
    expect(sift!.priority).toBe("P0");
    expect(sift!.disclosure_tier).toBe("tier-0");
    expect(sift!.trigger_keywords?.length).toBeGreaterThan(0);
    expect(sift!.interaction_type).toBe("步骤引导卡");
  });
});

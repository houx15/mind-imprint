import { describe, it, expect } from "vitest";
import { loadRegistry, deriveCatalog } from "../src/registry";

describe("loadRegistry", () => {
  it("loads the full registry including the demo card", () => {
    const reg = loadRegistry();
    expect(Object.keys(reg)).toHaveLength(34);
    expect(reg.concession).toBeDefined();
  });
  it("throws with the card id when a card is invalid", () => {
    const broken = { concession: { id: "concession", category: "知识工具" } };
    expect(() => loadRegistry(broken)).toThrow(/concession/);
  });
  it("throws when the map key does not match card.id", () => {
    const reg = loadRegistry();
    const mismatched = { wrong_key: reg.concession };
    expect(() => loadRegistry(mismatched as never)).toThrow(/does not match/);
  });
});

describe("search-plan card", () => {
  it("is in the registry and is a project-scoped matrix card with the R-9 consolidation", () => {
    const spec = loadRegistry()["search-plan"];
    expect(spec).toBeDefined();
    expect(spec!.primitive).toBe("matrix");
    expect(spec!.target_type).toBe("project");
    expect(spec!.consolidation).toBe("reveal_framework_after_completion");
    const params = spec!.params as { cols: { id: string }[]; row_noun: string };
    expect(params.cols.map((c) => c.id)).toEqual(["evidence_type", "blind_spot", "disconfirm"]);
    expect(params.row_noun).toBe("检索方向");
  });
});

describe("deriveCatalog", () => {
  it("projects one trigger_condition line per card and nothing stale", () => {
    const cat = deriveCatalog(loadRegistry());
    expect(cat).toHaveLength(34);
    expect(cat.every((c) => typeof c.trigger_condition === "string" && c.trigger_condition.length > 0)).toBe(true);
    expect(Object.keys(cat[0]!).sort()).toEqual(["category", "disclosure_tier", "id", "interaction_type", "name", "priority", "purpose", "trigger_condition", "trigger_keywords"]);
  });
  it("every step of every card has structured methodology (Layer B)", () => {
    const reg = loadRegistry();
    for (const card of Object.values(reg)) {
      for (const step of card.steps) {
        const where = `${card.id}/${step.key}`;
        expect(step.methodology, where).toBeDefined();
        expect(step.methodology!.why.length, where).toBeGreaterThan(0);
        expect(step.methodology!.how.length, where).toBeGreaterThan(0);
        expect(step.methodology!.when.length, where).toBeGreaterThan(0);
      }
    }
  });
  it("deriveCatalog projects routing metadata", () => {
    const cat = deriveCatalog(loadRegistry());
    const c = cat.find((c) => c.id === "concession");
    expect(c).toBeDefined();
    expect(c!.priority).toBe("P0");
    expect(c!.disclosure_tier).toBe("tier-1");
    expect(c!.trigger_keywords?.length).toBeGreaterThan(0);
    expect(c!.interaction_type).toBe("步骤引导卡");
    expect(typeof c!.purpose).toBe("string");
    expect(c!.purpose.length).toBeGreaterThan(0);
  });
});

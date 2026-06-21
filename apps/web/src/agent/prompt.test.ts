import { describe, it, expect } from "vitest";
import { deriveCatalog, CARD_REGISTRY } from "@mind-imprint/contracts";
import { buildCatalogText, buildSystemPrompt, demoCatalog, summonCardTool } from "./prompt";

const full = deriveCatalog(CARD_REGISTRY);

describe("decision layer", () => {
  it("catalog text groups by category and lists each card's trigger_condition", () => {
    const txt = buildCatalogText(full);
    expect(txt).toContain("信息素养");
    expect(txt).toContain("sift_craap");
    // each line carries the trigger_condition
    expect(txt).toMatch(/sift_craap.*：.+/);
  });
  it("system prompt embeds the catalog and the restraint rules", () => {
    const p = buildSystemPrompt(full);
    expect(p).toContain("不替他定论");
    expect(p).toContain("summon_card");
    expect(p).toContain("sift_craap");
  });
  it("demoCatalog drops the 3 library twins but keeps the demo cards", () => {
    const d = demoCatalog(full).map((c) => c.id);
    expect(d).not.toContain("sift");
    expect(d).not.toContain("craap");
    expect(d).not.toContain("steelman");
    expect(d).toContain("sift_craap");
    expect(d).toContain("concession");
    expect(d.length).toBe(full.length - 3);
  });
  it("summon_card tool exposes card_id enum = catalog ids and the 3 required args", () => {
    const tool = summonCardTool(demoCatalog(full));
    expect(tool.name).toBe("summon_card");
    const params = tool.parameters as any;
    expect(params.properties.card_id.enum).toEqual(demoCatalog(full).map((c) => c.id));
    expect(params.required).toEqual(["card_id", "reason", "nudge_text"]);
  });
});

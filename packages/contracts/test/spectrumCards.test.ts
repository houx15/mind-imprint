import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import certainty from "../cards/certainty-spectrum.json";
import belief from "../cards/belief-spectrum.json";

describe("spectrum cards un-stubbed", () => {
  it("certainty-spectrum is full and parses with a spectrum field", () => {
    const c = CardSpec.parse(certainty);
    expect(c.body_status).toBe("full");
    const types = c.steps[0]!.fields.map((f) => f.type);
    expect(types).toContain("spectrum");
  });
  it("belief-spectrum is full and nests a spectrum inside a repeatable_group", () => {
    const c = CardSpec.parse(belief);
    expect(c.body_status).toBe("full");
    const group = c.steps[0]!.fields.find((f) => f.type === "repeatable_group");
    expect(group).toBeTruthy();
    // @ts-expect-error narrow at runtime
    expect(group.item_fields.some((f: { type: string }) => f.type === "spectrum")).toBe(true);
  });
});

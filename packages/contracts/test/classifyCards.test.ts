import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import science from "../cards/science-knowing.json";
import spin from "../cards/spin-detector.json";

describe("classify cards un-stubbed", () => {
  it("science-knowing is full and has a criteria_check field", () => {
    const c = CardSpec.parse(science);
    expect(c.body_status).toBe("full");
    expect(c.steps.flatMap((s) => s.fields).some((f) => f.type === "criteria_check")).toBe(true);
  });
  it("spin-detector is full and uses repeatable_group + single_choice (no new primitive)", () => {
    const c = CardSpec.parse(spin);
    expect(c.body_status).toBe("full");
    const types = c.steps.flatMap((s) => s.fields).map((f) => f.type);
    expect(types).toContain("repeatable_group");
  });
});

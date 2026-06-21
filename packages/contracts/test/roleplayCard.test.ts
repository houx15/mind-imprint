import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import roleplay from "../cards/ethics-roleplay.json";

describe("ethics-roleplay un-stubbed (composed)", () => {
  it("is full and has a repeatable_group of roles + a closing field", () => {
    const c = CardSpec.parse(roleplay);
    expect(c.body_status).toBe("full");
    const fields = c.steps.flatMap((s) => s.fields);
    expect(fields.some((f) => f.type === "repeatable_group" && f.key === "roles")).toBe(true);
    expect(fields.some((f) => f.key === "closing")).toBe(true);
  });
});

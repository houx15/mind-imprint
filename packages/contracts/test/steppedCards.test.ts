import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import aok from "../cards/aok-methods.json";
import corpus from "../cards/corpus-hook.json";

describe("stepped-guide cards un-stubbed", () => {
  it("aok-methods is full and has show_if fields gated on a subject single_choice", () => {
    const c = CardSpec.parse(aok);
    expect(c.body_status).toBe("full");
    const fields = c.steps.flatMap((s) => s.fields);
    expect(fields.some((f) => f.type === "single_choice" && f.key === "subject")).toBe(true);
    expect(fields.some((f) => (f as { show_if?: unknown }).show_if)).toBe(true);
  });
  it("corpus-hook is full and uses repeatable_group (existing primitives)", () => {
    const c = CardSpec.parse(corpus);
    expect(c.body_status).toBe("full");
    expect(c.steps.flatMap((s) => s.fields).some((f) => f.type === "repeatable_group")).toBe(true);
  });
});

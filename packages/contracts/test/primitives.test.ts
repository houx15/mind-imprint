import { describe, it, expect } from "vitest";
import { FieldPrimitive } from "../src/primitives";

describe("FieldPrimitive", () => {
  it("accepts a textarea field", () => {
    expect(FieldPrimitive.safeParse({ type: "textarea", key: "stop", label: "Stop" }).success).toBe(true);
  });
  it("accepts single_choice with options", () => {
    expect(FieldPrimitive.safeParse({ type: "single_choice", key: "v", label: "可信？", options: ["可信", "存疑"] }).success).toBe(true);
  });
  it("rejects single_choice with empty options", () => {
    expect(FieldPrimitive.safeParse({ type: "single_choice", key: "v", label: "x", options: [] }).success).toBe(false);
  });
  it("accepts rating with a scale", () => {
    expect(FieldPrimitive.safeParse({ type: "rating", key: "c", label: "Currency", scale: 5 }).success).toBe(true);
  });
  it("accepts a repeatable_group of simple item_fields", () => {
    const ok = FieldPrimitive.safeParse({
      type: "repeatable_group", key: "sources", label: "来源",
      item_fields: [{ type: "text", key: "name", label: "来源" }],
    });
    expect(ok.success).toBe(true);
  });
  it("rejects an unknown field type", () => {
    expect(FieldPrimitive.safeParse({ type: "slider", key: "x", label: "x" }).success).toBe(false);
  });
});

import { describe, it, expect } from "vitest";
import { SpectrumField, FieldPrimitive, ItemField, RepeatableGroupField, CriteriaCheckField } from "../src/primitives";

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
  it("rejects a nested repeatable_group inside item_fields", () => {
    expect(FieldPrimitive.safeParse({
      type: "repeatable_group", key: "outer", label: "Outer",
      item_fields: [{ type: "repeatable_group", key: "inner", label: "Inner", item_fields: [] }],
    }).success).toBe(false);
  });
  it("rejects multi_choice with empty options", () => {
    expect(FieldPrimitive.safeParse({ type: "multi_choice", key: "v", label: "x", options: [] }).success).toBe(false);
  });
  it("rejects rating with a non-positive scale", () => {
    expect(FieldPrimitive.safeParse({ type: "rating", key: "c", label: "Currency", scale: 0 }).success).toBe(false);
  });
});

describe("SpectrumField", () => {
  const ok = { type: "spectrum", key: "pos", label: "位置", stops: ["低", "中", "高"] };
  it("parses a valid spectrum field", () => {
    expect(SpectrumField.parse(ok)).toEqual(ok);
  });
  it("requires at least 2 stops", () => {
    expect(SpectrumField.safeParse({ ...ok, stops: ["只有一个"] }).success).toBe(false);
  });
  it("is accepted by the FieldPrimitive union", () => {
    expect(FieldPrimitive.parse(ok)).toEqual(ok);
  });
  it("is accepted as an ItemField (usable inside repeatable_group)", () => {
    expect(ItemField.parse(ok)).toEqual(ok);
    const group = { type: "repeatable_group", key: "g", label: "G", item_fields: [ok] };
    expect(RepeatableGroupField.parse(group)).toEqual(group);
  });
});

describe("CriteriaCheckField", () => {
  const ok = { type: "criteria_check", key: "sci", label: "四标准", criteria: ["可证伪", "对照"], levels: ["满足", "不满足"] };
  it("parses a valid criteria_check field", () => {
    expect(CriteriaCheckField.parse(ok)).toEqual(ok);
  });
  it("requires at least 2 criteria and at least 2 levels", () => {
    expect(CriteriaCheckField.safeParse({ ...ok, criteria: ["只一个"] }).success).toBe(false);
    expect(CriteriaCheckField.safeParse({ ...ok, levels: ["只一个"] }).success).toBe(false);
  });
  it("is accepted by the FieldPrimitive union", () => {
    expect(FieldPrimitive.parse(ok)).toEqual(ok);
  });
});

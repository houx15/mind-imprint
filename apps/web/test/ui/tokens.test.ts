import { describe, it, expect } from "vitest";
import { MACARONS, SEMANTIC, NEUTRAL, RADIUS } from "@/ui/tokens";

describe("design tokens", () => {
  it("has all 7 macaron colors with tint pairs", () => {
    expect(Object.keys(MACARONS)).toEqual(
      ["peach", "butter", "matcha", "lake", "mist", "taro", "berry"],
    );
    expect(MACARONS.peach).toEqual({ base: "#F6B26B", bg: "#FDE7D3", fg: "#9A5A22" });
    expect(MACARONS.berry).toEqual({ base: "#E896A6", bg: "#FCE7EB", fg: "#A63A50" });
  });
  it("keeps semantic colors independent (danger is not accent)", () => {
    expect(SEMANTIC.danger.base).toBe("#D64541");
    expect(SEMANTIC.success.base).toBe("#5FA97E");
  });
  it("uses warm neutrals", () => {
    expect(NEUTRAL.paper).toBe("#FBF8F4");
    expect(NEUTRAL.ink).toBe("#33302E");
    expect(NEUTRAL.border).toBe("#EFE7DD");
  });
  it("tightens radius (sm workhorse = 8)", () => {
    expect(RADIUS.sm).toBe(8);
    expect(RADIUS.lg).toBe(12);
  });
});

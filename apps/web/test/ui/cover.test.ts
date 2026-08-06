import { describe, it, expect } from "vitest";
import { coverGradient, coverGradientStyle } from "@/ui/cover";
import { MACARONS } from "@/ui/tokens";

const MACARON_NAMES = Object.keys(MACARONS);
const HEX_COLOR = /#[0-9A-Fa-f]{6}/g;

describe("coverGradient", () => {
  it("is deterministic: same seed always yields the same result", () => {
    const a = coverGradient("中国是否让地球更可持续");
    const b = coverGradient("中国是否让地球更可持续");
    expect(a).toEqual(b);
  });

  it("picks a macaron that is one of the 7 known names", () => {
    const { macaron } = coverGradient("some project title");
    expect(MACARON_NAMES).toContain(macaron);
  });

  it("returns a linear-gradient string with two hex colors", () => {
    const { background } = coverGradient("essay on climate change");
    expect(background).toMatch(/^linear-gradient\(135deg, .+\)$/);
    const hexColors = background.match(HEX_COLOR) ?? [];
    expect(hexColors.length).toBe(2);
  });

  it("uses the chosen macaron's own base/bg tokens for the gradient stops", () => {
    const { background, macaron } = coverGradient("another seed");
    const { base, bg } = MACARONS[macaron as keyof typeof MACARONS];
    expect(background).toBe(`linear-gradient(135deg, ${base}, ${bg})`);
  });

  it("produces at least 2 distinct macarons across a handful of distinct seeds", () => {
    const seeds = [
      "Phoebe's essay",
      "中国是否让地球更可持续",
      "CRAAP source check",
      "concession paragraph",
      "reading room notes",
      "rabbit hole dig",
      "toulmin argument map",
      "outline draft v2",
    ];
    const macarons = new Set(seeds.map((s) => coverGradient(s).macaron));
    expect(macarons.size).toBeGreaterThanOrEqual(2);
  });

  it("coverGradientStyle returns a React CSSProperties backgroundImage", () => {
    const style = coverGradientStyle("essay on climate change");
    expect(style.backgroundImage).toBe(coverGradient("essay on climate change").background);
  });
});

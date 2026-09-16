import { describe, expect, it } from "vitest";
import { contrastRatio, readableText } from "./readableText";
const rgb = (hex: string) => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16));
describe("readable site text", () => {
  it("keeps secondary text legible across light, dark and weak generated palettes", () => {
    for (const [ink, paper] of [["#E2E8F0", "#101420"], ["#23201C", "#F4F0E6"], ["#EEEEEE", "#FFFFFF"], ["#777777", "#888888"]]) {
      const result = readableText(ink!, paper!);
      const bg = rgb(paper!), fg = rgb(result.ink);
      const blended = fg.map((c, i) => c * result.minimum / 100 + bg[i]! * (1 - result.minimum / 100));
      expect(contrastRatio(blended, bg)).toBeGreaterThanOrEqual(4.5);
    }
  });
  it("preserves already readable chosen ink", () => {
    expect(readableText("#E2E8F0", "#101420").ink).toBe("#E2E8F0");
  });
});

import { describe, expect, it } from "vitest";
import { cardTagStyle } from "./WeekSummaryCard";

// Contrast is not visible to jsdom. At 78% hue the praise and watch tag text
// read 3.34:1 and 2.76:1 on the tint in lite light mode; the 40% ink mix reads
// 5.97:1 and 5.46:1. Pin the mix so a restyle cannot drift back.
describe("cardTagStyle", () => {
  it("pins the praise and watch text colours to 40% hue in ink", () => {
    expect(cardTagStyle("praise").color).toBe("color-mix(in srgb, var(--mk-success) 40%, var(--mk-ink))");
    expect(cardTagStyle("watch").color).toBe("color-mix(in srgb, var(--mk-warning) 40%, var(--mk-ink))");
  });
  it("keeps the 14% tint background", () => {
    expect(cardTagStyle("praise").background).toBe("color-mix(in srgb, var(--mk-success) 14%, var(--mk-surface))");
    expect(cardTagStyle("watch").background).toBe("color-mix(in srgb, var(--mk-warning) 14%, var(--mk-surface))");
  });
});

import { describe, it, expect } from "vitest";
import { deriveActivityCalendar } from "./activityCalendar";

const NOW = new Date("2026-06-21T12:00:00.000Z");

describe("deriveActivityCalendar", () => {
  it("returns 119 cells over 17 weeks", () => {
    const r = deriveActivityCalendar([], NOW);
    expect(r.cells).toHaveLength(119);
    expect(r.weeks).toBe(17);
  });
  it("empty input → all lowest swatch, 0 active days", () => {
    const r = deriveActivityCalendar([], NOW);
    expect(r.activeDays).toBe(0);
    expect(r.cells.every((c) => c.style.includes("#EDEFF4"))).toBe(true);
  });
  it("counts a day with events as active and raises its intensity", () => {
    const day = "2026-06-20T09:00:00.000Z";
    const r = deriveActivityCalendar(
      [{ created_at: day }, { created_at: day }, { created_at: day }], NOW,
    );
    expect(r.activeDays).toBe(1);
    const active = r.cells.filter((c) => !c.style.includes("#EDEFF4"));
    expect(active).toHaveLength(1);
    expect(active[0]!.style).toContain("#97A3D2"); // 3 events → level 3 swatch
  });
  it("ignores events outside the 17-week window", () => {
    const old = "2024-01-01T00:00:00.000Z";
    const r = deriveActivityCalendar([{ created_at: old }], NOW);
    expect(r.activeDays).toBe(0);
  });
});

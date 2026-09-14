import { describe, expect, it } from "vitest";
import { canGoNext, shiftWeek, splitWeekTitle } from "./weekNav";

describe("splitWeekTitle", () => {
  it("splits the week label off the end of the server title", () =>
    expect(splitWeekTitle("上周班级周报 · 第 37 周（9.7–9.13）", "第 37 周（9.7–9.13）")).toEqual({
      head: "上周班级周报 · ",
      label: "第 37 周（9.7–9.13）",
    }));
  it("keeps a title that does not end with the label whole", () =>
    expect(splitWeekTitle("表现总结", "第 37 周（9.7–9.13）")).toEqual({ head: "表现总结", label: "" }));
  it("keeps the title whole with an empty label", () =>
    expect(splitWeekTitle("表现总结 · x", "")).toEqual({ head: "表现总结 · x", label: "" }));
});

describe("weekNav", () => {
  it("steps back a week across a month boundary", () => expect(shiftWeek("2026-09-07", -7)).toBe("2026-08-31"));
  it("steps forward", () => expect(shiftWeek("2026-08-31", 7)).toBe("2026-09-07"));
  it("no next from the latest week", () => expect(canGoNext("2026-09-07", true)).toBe(false));

  it("has a next week from an earlier week", () => expect(canGoNext("2026-08-31", false)).toBe(true));

  // The day count, not the local clock: a DST change or a UTC offset in the
  // browser must not move the date by one.
  it("crosses a year boundary", () => expect(shiftWeek("2026-01-05", -7)).toBe("2025-12-29"));
  it("crosses a leap day", () => expect(shiftWeek("2028-02-28", 7)).toBe("2028-03-06"));
});

import { describe, expect, it } from "vitest";
import { defaultRange, monthDay, publishedMonthDay, rangeLabel, todayBeijing } from "./range";

describe("parent report range", () => {
  it("defaults to 28 days ending today", () =>
    expect(defaultRange("2026-09-14")).toEqual({ start: "2026-08-18", end: "2026-09-14" }));
  it("labels a range", () => expect(rangeLabel("2026-08-17", "2026-09-13")).toBe("8月17日–9月13日"));
  it("today in Beijing crosses UTC midnight", () =>
    expect(todayBeijing(Date.UTC(2026, 8, 13, 16, 30))).toBe("2026-09-14"));

  // Calendar math on YYYY-MM-DD, not local-time Date arithmetic: month and
  // year boundaries are where a hand-rolled day count goes wrong.
  it("crosses a year boundary", () =>
    expect(defaultRange("2026-01-10")).toEqual({ start: "2025-12-14", end: "2026-01-10" }));
  it("crosses a leap day", () =>
    expect(defaultRange("2028-03-01")).toEqual({ start: "2028-02-03", end: "2028-03-01" }));
  it("names both years when the range spans two", () =>
    expect(rangeLabel("2025-12-13", "2026-01-09")).toBe("2025年12月13日–2026年1月9日"));
  it("still in Beijing's day before UTC midnight", () =>
    expect(todayBeijing(Date.UTC(2026, 8, 13, 15, 59))).toBe("2026-09-13"));
});

describe("monthDay", () => {
  it("reads a calendar date", () => expect(monthDay("2026-09-03")).toBe("9月3日"));
  it("is empty for a malformed date", () => expect(monthDay("9/3")).toBe(""));
});

describe("publishedMonthDay", () => {
  // 2026-09-13T16:30Z is already 9月14日 in Beijing, whatever zone the
  // parent's phone is set to.
  it("uses the Beijing date of an instant", () =>
    expect(publishedMonthDay("2026-09-13T16:30:00Z")).toBe("9月14日"));
  it("is empty for an unparsable instant", () => expect(publishedMonthDay("")).toBe(""));
});

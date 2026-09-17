import { describe, expect, it } from "vitest";
import {
  dayAllowed,
  dueShortcuts,
  formatValue,
  minuteOptions,
  monthGrid,
  parseValue,
  shiftMonth,
  todayBeijing,
  valueLabel,
  withDay,
} from "./dateFieldLogic";

/** Beijing wall clock → epoch ms (fixed +08:00). */
const bj = (y: number, m: number, d: number, hh = 10, mi = 0) => Date.UTC(y, m - 1, d, hh - 8, mi);

describe("parseValue / formatValue", () => {
  it("reads both value shapes and writes them back", () => {
    expect(parseValue("2026-09-18T21:05")).toEqual({ y: 2026, m: 9, d: 18, hh: 21, mi: 5 });
    expect(parseValue("2026-09-18")).toEqual({ y: 2026, m: 9, d: 18, hh: 0, mi: 0 });
    expect(formatValue({ y: 2026, m: 9, d: 8, hh: 7, mi: 5 }, true)).toBe("2026-09-08T07:05");
    expect(formatValue({ y: 2026, m: 9, d: 8, hh: 7, mi: 5 }, false)).toBe("2026-09-08");
  });
  it("rejects impossible and malformed values", () => {
    for (const v of ["", "2026-02-30", "2026-13-01", "2026-09-18T24:00", "2026/09/18", "2026-09-18T21"]) {
      expect(parseValue(v)).toBeNull();
    }
    expect(parseValue("2028-02-29")).not.toBeNull();
  });
});

describe("todayBeijing", () => {
  it("is Beijing's date, which runs 8 hours ahead of UTC", () => {
    expect(todayBeijing(Date.UTC(2026, 8, 17, 16, 30))).toEqual({ y: 2026, m: 9, d: 18 });
    expect(todayBeijing(Date.UTC(2026, 8, 17, 15, 59))).toEqual({ y: 2026, m: 9, d: 17 });
  });
});

describe("monthGrid", () => {
  it("covers the month in six Monday-first weeks", () => {
    const g = monthGrid(2026, 9); // 2026-09-01 is a Tuesday
    expect(g).toHaveLength(42);
    expect(g[0]).toEqual({ y: 2026, m: 8, d: 31, inMonth: false });
    expect(g[1]).toEqual({ y: 2026, m: 9, d: 1, inMonth: true });
    expect(g.filter((c) => c.inMonth)).toHaveLength(30);
    expect(g[41]).toEqual({ y: 2026, m: 10, d: 11, inMonth: false });
  });
  it("starts on the 1st when the month starts on a Monday", () => {
    expect(monthGrid(2026, 6)[0]).toEqual({ y: 2026, m: 6, d: 1, inMonth: true });
  });
  it("crosses the year when shifting months", () => {
    expect(shiftMonth(2026, 12, 1)).toEqual({ y: 2027, m: 1 });
    expect(shiftMonth(2026, 1, -1)).toEqual({ y: 2025, m: 12 });
  });
});

describe("dayAllowed", () => {
  it("honours min and max, either of which may be absent", () => {
    const day = { y: 2026, m: 9, d: 17 };
    expect(dayAllowed(day, "2026-09-17", "2026-09-17")).toBe(true);
    expect(dayAllowed(day, "2026-09-18")).toBe(false);
    expect(dayAllowed(day, undefined, "2026-09-16")).toBe(false);
    expect(dayAllowed(day, "", "")).toBe(true);
  });
});

describe("valueLabel", () => {
  const today = { y: 2026, m: 9, d: 17 };
  it("shows date, weekday and time; the year only when it differs", () => {
    expect(valueLabel("2026-09-18T21:00", true, today)).toBe("9月18日 周五 21:00");
    expect(valueLabel("2027-01-04T08:30", true, today)).toBe("2027年1月4日 周一 08:30");
    expect(valueLabel("2026-09-18", false, today)).toBe("9月18日 周五");
    expect(valueLabel("", true, today)).toBe("");
  });
});

describe("withDay", () => {
  it("keeps the time already set, or uses the default", () => {
    const day = { y: 2026, m: 9, d: 21 };
    expect(withDay("2026-09-18T08:15", day, { hh: 21, mi: 0 })).toEqual({ ...day, hh: 8, mi: 15 });
    expect(withDay("", day, { hh: 21, mi: 0 })).toEqual({ ...day, hh: 21, mi: 0 });
  });
});

describe("dueShortcuts", () => {
  const labels = (now: number) => dueShortcuts(now).map((s) => `${s.label}=${s.value}`);
  it("on a Thursday morning: today, tomorrow (Friday), next Monday", () => {
    expect(labels(bj(2026, 9, 17))).toEqual([
      "今天 21:00=2026-09-17T21:00",
      "明天 周五=2026-09-18T21:00",
      "下周一=2026-09-21T21:00",
    ]);
  });
  it("drops today once 21:00 has passed", () => {
    expect(labels(bj(2026, 9, 17, 21, 30))[0]).toBe("明天 周五=2026-09-18T21:00");
  });
  it("on a Monday: this Friday and next Monday", () => {
    expect(labels(bj(2026, 9, 14))).toEqual([
      "今天 21:00=2026-09-14T21:00",
      "明天 周二=2026-09-15T21:00",
      "本周五=2026-09-18T21:00",
      "下周一=2026-09-21T21:00",
    ]);
  });
  it("on a Sunday: the coming Friday is 下周五, and 下周一 is already 明天", () => {
    expect(labels(bj(2026, 9, 20))).toEqual([
      "今天 21:00=2026-09-20T21:00",
      "明天 周一=2026-09-21T21:00",
      "下周五=2026-09-25T21:00",
    ]);
  });
  it("on a Saturday: 下周五 and 下周一", () => {
    expect(labels(bj(2026, 9, 19)).slice(2)).toEqual(["下周五=2026-09-25T21:00", "下周一=2026-09-21T21:00"]);
  });
});

describe("minuteOptions", () => {
  it("offers quarter hours, plus a value's own minute", () => {
    expect(minuteOptions(null)).toEqual([0, 15, 30, 45]);
    expect(minuteOptions(30)).toEqual([0, 15, 30, 45]);
    expect(minuteOptions(10)).toEqual([0, 10, 15, 30, 45]);
  });
});

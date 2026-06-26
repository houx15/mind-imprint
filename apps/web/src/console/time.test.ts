import { describe, it, expect } from "vitest";
import { shortDate } from "./time";

describe("shortDate", () => {
  it("slices the date portion of an ISO timestamp", () => {
    expect(shortDate("2026-06-26T13:45:00Z")).toBe("2026-06-26");
  });
});

import { relativeTime } from "./time";

describe("relativeTime", () => {
  const now = Date.parse("2026-06-26T12:00:00Z");
  it("returns 从未 for null", () => {
    expect(relativeTime(null, now)).toBe("从未");
  });
  it("returns 刚刚 under a minute", () => {
    expect(relativeTime("2026-06-26T11:59:30Z", now)).toBe("刚刚");
  });
  it("returns minutes", () => {
    expect(relativeTime("2026-06-26T11:30:00Z", now)).toBe("30 分钟前");
  });
  it("returns hours", () => {
    expect(relativeTime("2026-06-26T10:00:00Z", now)).toBe("2 小时前");
  });
  it("returns days", () => {
    expect(relativeTime("2026-06-24T12:00:00Z", now)).toBe("2 天前");
  });
  it("falls back to a date for older than a week", () => {
    expect(relativeTime("2026-06-01T12:00:00Z", now)).toBe("2026-06-01");
  });
});

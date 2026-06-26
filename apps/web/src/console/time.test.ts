import { describe, it, expect } from "vitest";
import { shortDate } from "./time";

describe("shortDate", () => {
  it("slices the date portion of an ISO timestamp", () => {
    expect(shortDate("2026-06-26T13:45:00Z")).toBe("2026-06-26");
  });
});

import { describe, expect, it } from "vitest";
import { parseLiteRoute, readingPath, writingPath } from "@lite/routing";

describe("parseLiteRoute", () => {
  it("defaults to the readings tab", () => expect(parseLiteRoute("/")).toEqual({ tab: "readings" }));
  it("reads a reading id", () =>
    expect(parseLiteRoute("/readings/abc-123")).toEqual({ tab: "readings", readingId: "abc-123" }));
  it("knows the writings tab", () => expect(parseLiteRoute("/writings")).toEqual({ tab: "writings" }));
  it("reads a writing id", () =>
    expect(parseLiteRoute("/writings/abc-123")).toEqual({ tab: "writings", writingId: "abc-123" }));
  it("round-trips a reading path", () =>
    expect(parseLiteRoute(readingPath("xyz"))).toEqual({ tab: "readings", readingId: "xyz" }));
  it("round-trips a writing path", () =>
    expect(parseLiteRoute(writingPath("xyz"))).toEqual({ tab: "writings", writingId: "xyz" }));
});

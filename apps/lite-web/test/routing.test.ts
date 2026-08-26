import { describe, expect, it } from "vitest";
import { parseLiteRoute, readingPath } from "@lite/routing";

describe("parseLiteRoute", () => {
  it("defaults to the readings tab", () => expect(parseLiteRoute("/")).toEqual({ tab: "readings" }));
  it("reads a reading id", () =>
    expect(parseLiteRoute("/readings/abc-123")).toEqual({ tab: "readings", readingId: "abc-123" }));
  it("knows the writings tab", () => expect(parseLiteRoute("/writings")).toEqual({ tab: "writings" }));
  it("round-trips", () => expect(parseLiteRoute(readingPath("xyz"))).toEqual({ tab: "readings", readingId: "xyz" }));
});

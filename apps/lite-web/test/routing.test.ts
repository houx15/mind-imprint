import { describe, expect, it } from "vitest";
import { liteRoutePath, parseLiteRoute, readingPath, writingPath } from "@lite/routing";

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

  it("reads a share token", () =>
    expect(parseLiteRoute("/s/abc")).toEqual({ tab: "share", token: "abc" }));
  // A bare `/s` with no token has nothing to fetch — it must fall back to
  // the readings default like any other malformed path, NOT produce
  // `{tab:"share", token: undefined}`. Named here so dropping the `second ?`
  // guard in `parseLiteRoute`'s `"s"` case fails loudly instead of silently.
  it("does not treat a bare /s as a share route", () =>
    expect(parseLiteRoute("/s")).toEqual({ tab: "readings" }));
  it("round-trips a share path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "share", token: "abc" }))).toEqual({
      tab: "share",
      token: "abc",
    }));
});

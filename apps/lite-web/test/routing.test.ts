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

  // A shared link is two pages: the article at `/s/:token`, and the record of
  // writing it at `/s/:token/record`. The bare token defaults to the article
  // — that is what the link was sent for.
  it("reads a share token", () =>
    expect(parseLiteRoute("/s/abc")).toEqual({ tab: "share", token: "abc", view: "article" }));
  it("reads the record sub-page", () =>
    expect(parseLiteRoute("/s/abc/record")).toEqual({ tab: "share", token: "abc", view: "record" }));
  // Anything else after the token is a typo or a stale deep link. It lands on
  // the article rather than dead-ending — same "never a dead end" rule the
  // unknown-path fallback follows.
  it("falls back to the article for an unknown sub-page", () =>
    expect(parseLiteRoute("/s/abc/whatever")).toEqual({ tab: "share", token: "abc", view: "article" }));
  // A bare `/s` with no token has nothing to fetch — it must fall back to
  // the readings default like any other malformed path, NOT produce
  // `{tab:"share", token: undefined}`. Named here so dropping the `second ?`
  // guard in `parseLiteRoute`'s `"s"` case fails loudly instead of silently.
  it("does not treat a bare /s as a share route", () =>
    expect(parseLiteRoute("/s")).toEqual({ tab: "readings" }));
  it("round-trips a share path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "share", token: "abc", view: "article" }))).toEqual({
      tab: "share",
      token: "abc",
      view: "article",
    }));
  it("round-trips the record path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "share", token: "abc", view: "record" }))).toEqual({
      tab: "share",
      token: "abc",
      view: "record",
    }));
  // `/p/:token` — 她自己的主页，访客那一面（S5）。和 `/s/` 是两种东西：`/s/`
  // 是一次阅读或写作的记录，会有很多条；`/p/` 是她这个人的主页，只有一个。
  it("reads a personal-page token", () =>
    expect(parseLiteRoute("/p/abc")).toEqual({ tab: "page", token: "abc" }));
  // 同 `/s`：没有 token 就没有东西可取，落回默认页，而不是造出一个 token 为
  // 空的路由。
  it("does not treat a bare /p as a page route", () =>
    expect(parseLiteRoute("/p")).toEqual({ tab: "readings" }));
  it("round-trips a personal-page path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "page", token: "abc" }))).toEqual({
      tab: "page",
      token: "abc",
    }));
});

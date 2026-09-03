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

  // 我的树 (兴趣树). It is a REAL lite route, not the `/eco/tree` prototype
  // path — a student reaches it from the rail, and the prototype's own switch
  // hard-navigates here. Pinned so a future edit to `parseLiteRoute` cannot
  // quietly send `/tree` back to the readings default, which would look like
  // "the tab does nothing" rather than like a bug.
  it("knows the tree tab", () => expect(parseLiteRoute("/tree")).toEqual({ tab: "tree" }));
  it("tolerates a trailing slash on the tree tab", () =>
    expect(parseLiteRoute("/tree/")).toEqual({ tab: "tree" }));
  it("round-trips the tree path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "tree" }))).toEqual({ tab: "tree" }));
  // `/eco/tree` belongs to the prototype shell and must NOT be swallowed by
  // the lite route table: `rootElementFor` hands every `/eco/*` path to
  // `EcoRoot` before `parseLiteRoute` ever sees it, and a lite parse that
  // claimed it would be a silent takeover.
  it("does not claim the prototype's tree path", () =>
    expect(parseLiteRoute("/eco/tree")).toEqual({ tab: "readings" }));

  // 觉醒协议是 tree 这条 tab 下的一屏，不是自己的顶层 tab。
  it("knows the quiz sub-page", () =>
    expect(parseLiteRoute("/tree/quiz")).toEqual({ tab: "tree", quiz: true }));
  it("round-trips the quiz path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "tree", quiz: true }))).toEqual({
      tab: "tree",
      quiz: true,
    }));
  // 一个手敲错的子路径不该把她丢回阅读室——她要去的是树，那就给她树。
  it("falls back to the tree itself for an unknown sub-page", () =>
    expect(parseLiteRoute("/tree/whatever")).toEqual({ tab: "tree" }));
  it("the bare tree path carries no quiz flag", () =>
    expect(liteRoutePath({ tab: "tree" })).toBe("/tree"));

  // 探索 (今日新闻星图). 排在 rail 的第一格，但 `/` 仍然落在阅读 —— 见
  // LiteApp 的注释：一屏可能生成失败的星图不该是每个学生的落地页。
  it("knows the explore tab", () =>
    expect(parseLiteRoute("/explore")).toEqual({ tab: "explore" }));
  it("round-trips the explore path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "explore" }))).toEqual({ tab: "explore" }));
  it("still lands on readings at the root", () =>
    expect(parseLiteRoute("/")).toEqual({ tab: "readings" }));
});

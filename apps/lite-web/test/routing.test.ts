import { describe, expect, it } from "vitest";
import { liteRoutePath, parseLiteRoute, readingPath, writingPath } from "@lite/routing";

describe("parseLiteRoute", () => {
  // The learning home is additive; /explore and all workroom links remain valid.
  it("lands on the learning home at the root", () =>
    expect(parseLiteRoute("/")).toEqual({ tab: "home" }));
  it("tolerates index.html at the root", () =>
    expect(parseLiteRoute("/index.html")).toEqual({ tab: "home" }));
  it("still opens the readings tab from its own path", () =>
    expect(parseLiteRoute("/readings")).toEqual({ tab: "readings" }));
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
  // the default landing surface like any other malformed path, NOT produce
  // `{tab:"share", token: undefined}`. Named here so dropping the `second ?`
  // guard in `parseLiteRoute`'s `"s"` case fails loudly instead of silently.
  it("does not treat a bare /s as a share route", () =>
    expect(parseLiteRoute("/s")).toEqual({ tab: "explore" }));
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
    expect(parseLiteRoute("/p/abc")).toEqual({ tab: "page", token: "abc", view: "home" }));
  // 同 `/s`：没有 token 就没有东西可取，落回落地页，而不是造出一个 token 为
  // 空的路由。
  it("does not treat a bare /p as a page route", () =>
    expect(parseLiteRoute("/p")).toEqual({ tab: "explore" }));
  it("round-trips a personal-page path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "page", token: "abc", view: "home" }))).toEqual({
      tab: "page",
      token: "abc",
      view: "home",
    }));

  it("round-trips a personal-page work collection", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "page", token: "abc", view: "works" }))).toEqual({
      tab: "page",
      token: "abc",
      view: "works",
    }));

  // `/r/:token` and `/parent-reports/:id` were removed on 2026-09-15 (there is
  // no parent end). An old link, bare or with a token, lands on 探索.
  it("sends a removed parent report link to explore", () => {
    expect(parseLiteRoute("/r/abc")).toEqual({ tab: "explore" });
    expect(parseLiteRoute("/r")).toEqual({ tab: "explore" });
  });
  it("sends a removed student parent report path to explore", () => {
    expect(parseLiteRoute("/parent-reports/p1")).toEqual({ tab: "explore" });
    expect(parseLiteRoute("/parent-reports")).toEqual({ tab: "explore" });
  });

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
    expect(parseLiteRoute("/eco/tree")).toEqual({ tab: "explore" }));

  // 觉醒协议是 tree 这条 tab 下的一屏，不是自己的顶层 tab。
  it("knows the awakening sub-page", () =>
    expect(parseLiteRoute("/tree/awakening")).toEqual({ tab: "tree", awakening: true }));
  it("round-trips the awakening path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "tree", awakening: true }))).toEqual({
      tab: "tree",
      awakening: true,
    }));
  // 一份已经生成的报告有自己的路径，所以她能把它收藏、也能刷新。
  it("round-trips a report path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "tree", reportRunId: "abc-123" }))).toEqual({
      tab: "tree",
      reportRunId: "abc-123",
    }));
  // 一个手敲错的子路径不该把她丢回阅读室——她要去的是树，那就给她树。
  it("falls back to the tree itself for an unknown sub-page", () =>
    expect(parseLiteRoute("/tree/whatever")).toEqual({ tab: "tree" }));
  it("the bare tree path carries no quiz flag", () =>
    expect(liteRoutePath({ tab: "tree" })).toBe("/tree"));

  // 探索 (今日新闻星图) —— rail 的第一格，也是落地页。
  it("knows the explore tab", () =>
    expect(parseLiteRoute("/explore")).toEqual({ tab: "explore" }));
  it("round-trips the explore path", () =>
    expect(parseLiteRoute(liteRoutePath({ tab: "explore" }))).toEqual({ tab: "explore" }));
  // 一条过期或手敲错的 URL 落到探索，不是死掉。
  it("falls back to explore for an unknown path", () =>
    expect(parseLiteRoute("/whatever-this-is")).toEqual({ tab: "explore" }));
});

import { describe, expect, it } from "vitest";
import { coursePath, liteRoutePath, parseLiteRoute, type LiteRoute } from "./routing";

/**
 * routing.test.ts —— parse/format 这一对必须互为逆。
 *
 * 值得测的原因：这两个函数是分开写的两张表，加一条 tab 要改两处，漏改一处不会
 * 报错 —— 表现是「点这个 tab 地址栏变了，刷新一下回到落地页」，而这种 bug 只有
 * 用手点才碰得到。
 */

const ROUTES: LiteRoute[] = [
  { tab: "explore" },
  { tab: "readings" },
  { tab: "readings", readingId: "r-1" },
  { tab: "writings" },
  { tab: "writings", writingId: "w-1" },
  { tab: "projects" },
  { tab: "projects", projectId: "p-1" },
  { tab: "courses" },
  { tab: "courses", slug: "vibe-coding" },
  { tab: "tree" },
  { tab: "tree", quiz: true },
  { tab: "settings" },
];

describe("lite routing", () => {
  it("round-trips every route through its path", () => {
    for (const route of ROUTES) {
      expect(parseLiteRoute(liteRoutePath(route))).toEqual(route);
    }
  });

  it("parses /courses and /courses/:slug", () => {
    expect(parseLiteRoute("/courses")).toEqual({ tab: "courses" });
    expect(parseLiteRoute("/courses/vibe-coding")).toEqual({
      tab: "courses",
      slug: "vibe-coding",
    });
  });

  it("encodes a slug that needs it", () => {
    const path = coursePath("a b/c");
    expect(parseLiteRoute(path)).toEqual({ tab: "courses", slug: "a b/c" });
  });

  it("keeps an unknown path on 探索", () => {
    expect(parseLiteRoute("/nope")).toEqual({ tab: "explore" });
  });});

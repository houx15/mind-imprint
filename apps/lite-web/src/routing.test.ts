import { afterEach, describe, expect, it } from "vitest";
import { coursePath, liteRoutePath, navigate, parseLiteRoute, type LiteRoute } from "./routing";

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
  });
});

describe("navigate", () => {
  afterEach(() => {
    window.history.replaceState(null, "", "/");
  });

  // The lite teacher shell's assignment route carries a `?tab=grading`
  // query hint on top of the same pathname — `navigate`'s no-op guard used
  // to compare `pathname` alone, so moving between two paths that differ
  // ONLY by that query silently did nothing (no pushState, no popstate),
  // which read as "返回 is broken" even though the route object was correct.
  it("navigates when only the query string differs from the current location", () => {
    window.history.replaceState(null, "", "/assignments/a1?tab=grading");
    let pops = 0;
    const onPop = () => pops++;
    window.addEventListener("popstate", onPop);
    try {
      navigate("/assignments/a1");
      expect(window.location.pathname + window.location.search).toBe("/assignments/a1");
      expect(pops).toBe(1);
    } finally {
      window.removeEventListener("popstate", onPop);
    }
  });

  it("is a no-op when the target is byte-for-byte the current location, query included", () => {
    window.history.replaceState(null, "", "/assignments/a1?tab=grading");
    let pops = 0;
    const onPop = () => pops++;
    window.addEventListener("popstate", onPop);
    try {
      navigate("/assignments/a1?tab=grading");
      expect(window.location.pathname + window.location.search).toBe("/assignments/a1?tab=grading");
      expect(pops).toBe(0);
    } finally {
      window.removeEventListener("popstate", onPop);
    }
  });
});

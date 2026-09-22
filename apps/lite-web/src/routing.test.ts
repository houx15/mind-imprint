import { afterEach, describe, expect, it, vi } from "vitest";
import {
  beforeNavigate,
  coursePath,
  listenForNavigation,
  liteRoutePath,
  navigate,
  parseLiteRoute,
  type LiteRoute,
} from "./routing";

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
  { tab: "tree", awakening: true },
  { tab: "tree", reportRunId: "9f1c2b3a-0000-4000-8000-000000000001" },
  { tab: "settings" },
  { tab: "page", token: "site-token", view: "home" },
  { tab: "page", token: "site-token", view: "works" },
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

  it("keeps the public homepage and work collection as distinct refresh-safe routes", () => {
    expect(parseLiteRoute("/p/site-token")).toEqual({ tab: "page", token: "site-token", view: "home" });
    expect(parseLiteRoute("/p/site-token/works")).toEqual({ tab: "page", token: "site-token", view: "works" });
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

// 🚨 两组用例都要留着（2026-09-16 合并）。上面这一组管的是「只差一个查询串
// 也要真的导航」（教师端 ?tab=grading），下面那两条管的是导航守卫：编辑器
// 存完了才让路由走。它们测的是同一个 navigate 的两件不同的事。
it("waits for an editor save, rejects failed saves, and commits only the latest destination", async () => {
  const pushState = vi.fn();
  vi.stubGlobal("window", { location: { pathname: "/projects/p", search: "" }, history: { pushState }, dispatchEvent: vi.fn() });
  vi.stubGlobal("PopStateEvent", class { constructor(public type: string) {} });
  let release!: () => void;
  const done = new Promise<void>((resolve) => { release = resolve; });
  let remove = beforeNavigate(() => done);
  try {
    navigate("/readings"); navigate("/projects");
    expect(pushState).not.toHaveBeenCalled();
    release();
    await vi.waitFor(() => expect(pushState).toHaveBeenCalledTimes(1));
    expect(pushState).toHaveBeenLastCalledWith({ __liteHistoryIndex: 1 }, "", "/projects");
    remove();
    remove = beforeNavigate(async () => { throw new Error("save failed"); });
    navigate("/courses");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(pushState).toHaveBeenCalledTimes(1);
  } finally { remove(); vi.unstubAllGlobals(); }
});

it("keeps an editor mounted during history saves and reverses a failed traversal", async () => {
  const original = { state: window.history.state, path: window.location.href };
  window.history.replaceState({ __liteHistoryIndex: 1 }, "", "/projects/p");
  const changed = vi.fn();
  const stop = listenForNavigation(changed);
  let release!: () => void;
  const saved = new Promise<void>((resolve) => { release = resolve; });
  let remove = beforeNavigate(() => saved);
  const go = vi.spyOn(window.history, "go").mockImplementation(() => {});
  try {
    window.history.replaceState({ __liteHistoryIndex: 0 }, "", "/projects");
    window.dispatchEvent(new PopStateEvent("popstate"));
    expect(changed).not.toHaveBeenCalled();
    release();
    await vi.waitFor(() => expect(changed).toHaveBeenCalledTimes(1));
    remove();
    remove = beforeNavigate(async () => { throw new Error("offline"); });
    window.history.replaceState({ __liteHistoryIndex: -1 }, "", "/readings");
    window.dispatchEvent(new PopStateEvent("popstate"));
    await vi.waitFor(() => expect(go).toHaveBeenCalledWith(1));
    expect(changed).toHaveBeenCalledTimes(1);
    window.history.replaceState({ __liteHistoryIndex: 0 }, "", "/projects");
    window.dispatchEvent(new PopStateEvent("popstate"));
    expect(changed).toHaveBeenCalledTimes(1);
  } finally {
    remove(); stop(); go.mockRestore();
    window.history.replaceState(original.state, "", original.path);
  }
});

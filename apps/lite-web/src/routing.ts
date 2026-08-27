// routing — the lite shell's tiny, dependency-free URL model. Mirrors
// apps/web/src/shell/routing.ts in shape (root-relative paths, no router
// library, History API driven) but with the lite tab vocabulary: 阅读
// (readings) and 写作 (writings) — no 首页/课程/我, no project lifecycle.
//
// Design notes (same reasoning as the pro shell's routing.ts):
//  - Paths are ALWAYS root-relative ("/…"), never absolute URLs.
//  - No router library; LiteApp drives this module directly via
//    `parseLiteRoute` on load/popstate and `navigate` to push new paths.

export type LiteRoute =
  | { tab: "readings"; readingId?: string }
  | { tab: "writings"; writingId?: string }
  // 设置 is a route, not a rail tab: it is reached from the account button at
  // the foot of the rail, and while it is open neither 阅读 nor 写作 is the
  // active tab. Keeping it in the same union is what lets Back leave settings
  // and land exactly where the student was.
  | { tab: "settings" };

/** Parse a browser pathname into a lite route. Unknown paths fall back to the
 * readings tab (the lite shell's landing surface), so a stale or hand-typed
 * URL never dead-ends. Trailing slashes and `/index.html` are tolerated;
 * segments are URL-decoded. */
export function parseLiteRoute(pathname: string): LiteRoute {
  const cleaned = pathname
    .replace(/\/index\.html$/i, "/")
    .replace(/^\/+/, "")
    .replace(/\/+$/, "");
  const segments = cleaned.length === 0 ? [] : cleaned.split("/").map(decodeSegment);
  const [first, second] = segments;

  switch (first) {
    case undefined:
    case "":
    case "readings":
      return second ? { tab: "readings", readingId: second } : { tab: "readings" };
    case "writings":
      return second ? { tab: "writings", writingId: second } : { tab: "writings" };
    case "settings":
      return { tab: "settings" };
    default:
      return { tab: "readings" };
  }
}

/** The canonical root-relative path for a route. Inverse of `parseLiteRoute`
 * on the routes it produces. */
export function liteRoutePath(route: LiteRoute): string {
  switch (route.tab) {
    case "readings":
      return route.readingId ? `/readings/${encodeSegment(route.readingId)}` : "/readings";
    case "writings":
      return route.writingId ? `/writings/${encodeSegment(route.writingId)}` : "/writings";
    case "settings":
      return "/settings";
  }
}

/** The canonical path for the settings page. */
export function settingsPath(): string {
  return "/settings";
}

/** The canonical path for a single reading — used by the landing page after
 * `createReading` to route into it, and by `parseLiteRoute`'s inverse. */
export function readingPath(id: string): string {
  return `/readings/${encodeSegment(id)}`;
}

/** The canonical path for a single writing — used by the landing page after
 * `createWriting` to route into it, and by `parseLiteRoute`'s inverse. */
export function writingPath(id: string): string {
  return `/writings/${encodeSegment(id)}`;
}

/** Push a new root-relative path onto the History stack and dispatch a
 * synthetic `popstate` so listeners (LiteApp) re-derive the route — matching
 * how a real Back/Forward navigation notifies them. A no-op if `path` is
 * already the current pathname (avoids piling up duplicate history entries
 * or firing a redundant popstate on repeat calls). */
export function navigate(path: string): void {
  if (window.location.pathname === path) return;
  window.history.pushState(null, "", path);
  window.dispatchEvent(new PopStateEvent("popstate"));
}

function encodeSegment(value: string): string {
  return encodeURIComponent(value);
}

function decodeSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

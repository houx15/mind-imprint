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
  // 项目 (PBL). The frontend path is `/projects` even though the API lives at
  // `/api/v1/pbl/projects` — the `pbl` prefix exists to keep lite's endpoints
  // clear of pro's `/api/v1/projects`, and a student's URL bar has no such
  // collision to avoid.
  | { tab: "projects"; projectId?: string }
  // 设置 is a route, not a rail tab: it is reached from the account button at
  // the foot of the rail, and while it is open neither 阅读 nor 写作 is the
  // active tab. Keeping it in the same union is what lets Back leave settings
  // and land exactly where the student was.
  | { tab: "settings" }
  // `/s/:token` — a shared report's public link. Not a rail tab at all: it is
  // opened by someone with NO session (a parent scanning a QR code), so it
  // is never routed through `LiteApp` at all — `rootElementFor.tsx` (the
  // composition root, called from `main.tsx`) parses the pathname itself and
  // mounts `PublicReportPage` directly for `tab === "share"`, `LiteApp`
  // otherwise. See that module's own comment for why the choice lives there
  // instead of as an early return inside `LiteApp`.
  //
  // `view` splits the shared link into two REAL pages, because a shared
  // writing is an article first: `/s/:token` is her piece, set like an
  // article and nothing else; `/s/:token/record` is 这一篇是怎么写出来的 —
  // the stats, 金句 and 收获. They were one long scroll once, which made the
  // article read as the preamble to a dashboard. A reading has no article, so
  // its `/s/:token` renders the record directly and `view` is ignored.
  | { tab: "share"; token: string; view: "article" | "record" }
  // `/p/:token` — 她自己的主页，访客那一面。和 `/s/:token` 一样，打开它的人
  // 没有账号也没有 session（她把链接发给了家人或朋友），所以它同样不经过
  // `LiteApp`：`rootElementFor.tsx` 自己解析 pathname，直接挂 `PublicSitePage`。
  //
  // 为什么是 `/p/` 而不是 `/s/`：这两条链接是两种东西。`/s/` 是一次阅读或写作
  // 的记录，一篇一条、会有很多条；`/p/` 是她这个人的主页，只有一个。
  | { tab: "page"; token: string };

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
  const [first, second, third] = segments;

  switch (first) {
    case undefined:
    case "":
    case "readings":
      return second ? { tab: "readings", readingId: second } : { tab: "readings" };
    case "writings":
      return second ? { tab: "writings", writingId: second } : { tab: "writings" };
    case "projects":
      return second ? { tab: "projects", projectId: second } : { tab: "projects" };
    case "settings":
      return { tab: "settings" };
    case "p":
      // 和 `/s` 同样的道理：没有 token 就不是一条主页链接，落回默认页而不是
      // 造出一个 token 为空的路由。
      if (!second) return { tab: "readings" };
      return { tab: "page", token: second };
    case "s":
      // A malformed `/s` with no token is not a share route — it has nothing
      // to fetch — so it falls through to the readings default like any
      // other unrecognized path, rather than producing a route with an empty
      // token.
      //
      // Only the exact segment `record` opens the record; anything else after
      // the token is a typo or a stale deep link and lands on the article,
      // which is the page the link was sent for. Never a dead end.
      if (!second) return { tab: "readings" };
      return { tab: "share", token: second, view: third === "record" ? "record" : "article" };
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
    case "projects":
      return route.projectId ? `/projects/${encodeSegment(route.projectId)}` : "/projects";
    case "settings":
      return "/settings";
    case "page":
      return `/p/${encodeSegment(route.token)}`;
    case "share":
      return route.view === "record"
        ? `/s/${encodeSegment(route.token)}/record`
        : `/s/${encodeSegment(route.token)}`;
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

/** The canonical path for a single project — used by the landing page after
 * `createProject` (and after she names it) to route into it. */
export function projectPath(id: string): string {
  return `/projects/${encodeSegment(id)}`;
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

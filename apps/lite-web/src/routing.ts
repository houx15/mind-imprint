// routing — the lite shell's tiny, dependency-free URL model. Mirrors
// apps/web/src/shell/routing.ts in shape (root-relative paths, no router
// library, History API driven) but with the lite tab vocabulary: 探索
// (explore) · 阅读 (readings) · 写作 (writings) · 项目 (projects) · 我的树 (tree) — no 课程,
// and none of pro's project lifecycle.
//
// Design notes (same reasoning as the pro shell's routing.ts):
//  - Paths are ALWAYS root-relative ("/…"), never absolute URLs.
//  - No router library; LiteApp drives this module directly via
//    `parseLiteRoute` on load/popstate and `navigate` to push new paths.

export type LiteRoute =
  // 探索 (今日新闻星图). 每天五颗星，从十二个科学源抓来、模型选出。排在阅读
  // 前面，因为它是那条链子的起点：找到 → 读 → 写 → 做。
  | { tab: "explore" }
  | { tab: "readings"; readingId?: string }
  | { tab: "writings"; writingId?: string }
  // 项目 (PBL). The frontend path is `/projects` even though the API lives at
  // `/api/v1/pbl/projects` — the `pbl` prefix exists to keep lite's endpoints
  // clear of pro's `/api/v1/projects`, and a student's URL bar has no such
  // collision to avoid.
  | { tab: "projects"; projectId?: string }
  // 我的树 (兴趣树). Her keyword model, grown from what she actually finished —
  // 阅读 / 写作 / 项目. A real tab, not a prototype surface: it reads
  // `GET /api/v1/interest/tree` and renders nothing when that fails, because a
  // tree that falls back to sample words hangs sixteen keywords that are not
  // hers on a picture captioned 「这就是你的模型」.
  // `quiz` 打开觉醒协议（兴趣测试），`/tree/quiz`。它是**同一条 tab 下的一屏**
  // 而不是自己的顶层 tab：入口在树上，做完了回到树上，导航栏里不该多出一格
  // 只在冷启动时有意义的东西。
  | { tab: "tree"; quiz?: boolean }
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
  | { tab: "share"; token: string; view: "article" | "record" };

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
    case "tree":
      return second === "quiz" ? { tab: "tree", quiz: true } : { tab: "tree" };
    case "explore":
      return { tab: "explore" };
    case "settings":
      return { tab: "settings" };
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
    case "tree":
      return route.quiz ? "/tree/quiz" : "/tree";
    case "explore":
      return "/explore";
    case "settings":
      return "/settings";
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

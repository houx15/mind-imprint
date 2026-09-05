// routing — the lite shell's tiny, dependency-free URL model. Mirrors
// apps/web/src/shell/routing.ts in shape (root-relative paths, no router
// library, History API driven) but with the lite tab vocabulary: 探索
// (explore) · 阅读 (readings) · 写作 (writings) · 项目 (projects) · 课程 (courses) ·
// 我的主页 (mysite), and none of pro's project lifecycle.
//
// 2026-09-05：`tree` 不再是导航上的一格。探索地图与兴趣树合并成同一格里的两屏，
// 由顶部的切换器换（`explore/SkyTab.tsx`）—— 树是她已经有的，地图是她还没走过
// 的，分成两格之后学生要自己把这条线接起来。`/tree` 仍然是一条真链接，直接落在
// 树那一屏。腾出来的那一格给了「我的主页」。
//
// 课程 was added 2026-09-04. It shares pro's paths and pro's player; what a lite
// student sees is narrowed server-side by course.audience, not here.
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
  // 课程. Same paths as pro (`/courses`, `/courses/:slug`) because it is the
  // same catalog and the same runtime player — only the audience filter differs,
  // and that is decided server-side by the school's edition. A course link can
  // therefore be pasted between the two editions and still resolve.
  | { tab: "courses"; slug?: string }
  // 我的树 (兴趣树). Her keyword model, grown from what she actually finished —
  // 阅读 / 写作 / 项目. A real tab, not a prototype surface: it reads
  // `GET /api/v1/interest/tree` and renders nothing when that fails, because a
  // tree that falls back to sample words hangs sixteen keywords that are not
  // hers on a picture captioned 「这就是你的模型」.
  // `quiz` 打开觉醒协议（兴趣测试），`/tree/quiz`。它是**同一条 tab 下的一屏**
  // 而不是自己的顶层 tab：入口在树上，做完了回到树上，导航栏里不该多出一格
  // 只在冷启动时有意义的东西。
  | { tab: "tree"; quiz?: boolean }
  // 我的主页。**她自己那一面**，不是 `/p/:token` 那个访客页。
  //
  // 2026-09-05 加：主页发布之后项目进 keeping，她随时能回来改，但在这之前回到
  // 那一页的路只有「项目室 → 主页项目 → 侧栏那一行」。也就是说她必须先想起
  // 自己的主页是一个项目，才找得到它。
  | { tab: "mysite" }
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

/** Parse a browser pathname into a lite route.
 *
 * 🚨 **落地页是探索（今日新闻星图），不是阅读**（2026-09-03 改）。
 *
 * 原来 `/` 落在阅读室，因为那时探索地图还是原型、星图还是 mock 数据。现在它是
 * 真的：每天五颗从二十一个源里挑出来的星，每颗带一个她可以自己追问的问题。
 * **这才是每天回来看一眼的理由** —— 而阅读室是「我手头有一篇要读」时才去的
 * 地方，它是任务的入口，不是产品的入口。
 *
 * 导航顺序也因此说得通：探索（找到）→ 阅读 → 写作 → 项目 → 我的树（因此长出来）。
 *
 * 未知路径同样落到探索，所以一条过期或手敲错的 URL 永远不会死掉。容忍尾部斜杠
 * 与 `/index.html`；路径段会被 URL 解码。 */
export function parseLiteRoute(pathname: string): LiteRoute {
  const cleaned = pathname
    .replace(/\/index\.html$/i, "/")
    .replace(/^\/+/, "")
    .replace(/\/+$/, "");
  const segments = cleaned.length === 0 ? [] : cleaned.split("/").map(decodeSegment);
  const [first, second, third] = segments;

  switch (first) {
    // `/` 落在探索。星图当天生成失败时它有自己的空状态（说出后台原话 + 重试），
    // 所以哪怕没出星图，落地页也不是一片莫名其妙的白。
    case undefined:
    case "":
      return { tab: "explore" };
    case "readings":
      return second ? { tab: "readings", readingId: second } : { tab: "readings" };
    case "writings":
      return second ? { tab: "writings", writingId: second } : { tab: "writings" };
    case "projects":
      return second ? { tab: "projects", projectId: second } : { tab: "projects" };
    case "courses":
      return second ? { tab: "courses", slug: second } : { tab: "courses" };
    case "tree":
      return second === "quiz" ? { tab: "tree", quiz: true } : { tab: "tree" };
    case "site":
      return { tab: "mysite" };
    case "explore":
      return { tab: "explore" };
    case "settings":
      return { tab: "settings" };
    case "p":
      // 和 `/s` 同样的道理：没有 token 就不是一条主页链接，落回落地页而不是
      // 造出一个 token 为空的路由。
      if (!second) return { tab: "explore" };
      return { tab: "page", token: second };
    case "s":
      // A malformed `/s` with no token is not a share route — it has nothing
      // to fetch — so it falls through to the default landing surface like
      // any other unrecognized path, rather than producing a route with an
      // empty token.
      //
      // Only the exact segment `record` opens the record; anything else after
      // the token is a typo or a stale deep link and lands on the article,
      // which is the page the link was sent for. Never a dead end.
      if (!second) return { tab: "explore" };
      return { tab: "share", token: second, view: third === "record" ? "record" : "article" };
    default:
      return { tab: "explore" };
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
    case "courses":
      return route.slug ? `/courses/${encodeSegment(route.slug)}` : "/courses";
    case "tree":
      return route.quiz ? "/tree/quiz" : "/tree";
    case "explore":
      return "/explore";
    case "mysite":
      return "/site";
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

/** The canonical path for a single course. Used by the 项目 room when she opens
 * a course 印记 assigned, and by `parseLiteRoute`'s inverse. */
export function coursePath(slug: string): string {
  return `/courses/${encodeSegment(slug)}`;
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

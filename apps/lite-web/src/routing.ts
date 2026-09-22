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
  | { tab: "home" }
  // 探索 (今日新闻星图). 每天五颗星，从十二个科学源抓来、模型选出。排在阅读
  // 前面，因为它是那条链子的起点：找到 → 读 → 写 → 做。
  | { tab: "explore" }
  // 阅读。`library` 打开分级阅读库（`/readings/library`）—— 和觉醒协议在
  // 「我的树」下的做法一样，它是**同一条 tab 下的一屏**，不是自己的顶层
  // tab：入口在阅读室的落地页上，挑完一篇就进阅读室，导航栏里不该多出一格。
  //
  // 「library」不可能和一个阅读 id 撞车：id 是 UUID。
  | { tab: "readings"; readingId?: string; library?: boolean }
  // 写作。`library` 打开写作题库（`/writings/library`）—— 和阅读那边同一个形状。
  // 「library」不可能和一个写作 id 撞车：id 是 UUID。
  | { tab: "writings"; writingId?: string; library?: boolean }
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
  // `awakening` 打开觉醒协议，`/tree/awakening`；`reportRunId` 直接打开一份
  // 已经生成的报告，`/tree/report/:runId`。两者都是**同一条 tab 下的一屏**
  // 而不是自己的顶层 tab：入口在树上，走完回到树上，导航栏里不该多出一格。
  //
  // 它有真实的 URL 而不是纯前端状态，因为这一趟可能走十五分钟：刷新、误触
  // 返回键、第二天从历史记录点回来，都该落在同一个地方。房间本身是覆盖在
  // 树上面的一层（LiteApp），所以进门仍然是一次动作，不是一次整页跳转。
  | { tab: "tree"; awakening?: boolean; reportRunId?: string }
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
// 2026-09-15: `/r/:token` (a parent report's public link) and
// `/parent-reports/:id` (the student's copy) are gone. There is no parent end:
// the teacher exports the report as a picture. Both paths now fall through to
// the explore fallback like any unknown path.

/** Root and index.html open the learning home. Unknown and malformed public
 * paths retain the explore fallback; all learning deep links persist. */
export function parseLiteRoute(pathname: string): LiteRoute {
  const cleaned = pathname
    .replace(/\/index\.html$/i, "/")
    .replace(/^\/+/, "")
    .replace(/\/+$/, "");
  const segments =
    cleaned.length === 0 ? [] : cleaned.split("/").map(decodeSegment);
  const [first, second, third] = segments;

  switch (first) {
    case undefined:
    case "":
      return { tab: "home" };
    case "readings":
      if (second === "library") return { tab: "readings", library: true };
      return second
        ? { tab: "readings", readingId: second }
        : { tab: "readings" };
    case "writings":
      if (second === "library") return { tab: "writings", library: true };
      return second
        ? { tab: "writings", writingId: second }
        : { tab: "writings" };
    case "projects":
      return second
        ? { tab: "projects", projectId: second }
        : { tab: "projects" };
    case "courses":
      return second ? { tab: "courses", slug: second } : { tab: "courses" };
    case "tree":
      if (second === "awakening") return { tab: "tree", awakening: true };
      if (second === "report" && third) return { tab: "tree", reportRunId: third };
      return { tab: "tree" };
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
      return {
        tab: "share",
        token: second,
        view: third === "record" ? "record" : "article",
      };
    default:
      return { tab: "explore" };
  }
}

/** The canonical root-relative path for a route. Inverse of `parseLiteRoute`
 * on the routes it produces. */
export function liteRoutePath(route: LiteRoute): string {
  switch (route.tab) {
    case "home":
      return "/";
    case "readings":
      if (route.library) return "/readings/library";
      return route.readingId
        ? `/readings/${encodeSegment(route.readingId)}`
        : "/readings";
    case "writings":
      if (route.library) return "/writings/library";
      return route.writingId
        ? `/writings/${encodeSegment(route.writingId)}`
        : "/writings";
    case "projects":
      return route.projectId
        ? `/projects/${encodeSegment(route.projectId)}`
        : "/projects";
    case "courses":
      return route.slug ? `/courses/${encodeSegment(route.slug)}` : "/courses";
    case "tree":
      if (route.awakening) return "/tree/awakening";
      if (route.reportRunId) return `/tree/report/${encodeSegment(route.reportRunId)}`;
      return "/tree";
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

/** 分级阅读库那一屏。 */
export function readingLibraryPath(): string {
  return "/readings/library";
}

/** 写作题库那一屏。 */
export function writingLibraryPath(): string {
  return "/writings/library";
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
 * synthetic `popstate` so listeners (LiteApp, LiteTeacherShell) re-derive
 * the route — matching how a real Back/Forward navigation notifies them.
 * A no-op if `path` is already the current location (avoids piling up
 * duplicate history entries or firing a redundant popstate on repeat calls).
 *
 * 🚨 **比的是 `pathname + search`，不是 `pathname`。** 只差一个查询串的地址
 * （教师端那个 `?tab=grading`）用 pathname 比会被判成「哪儿也没去」，然后被
 * 静默丢掉。见 currentLocation。
 *
 * 🚨 这一段是两条线合起来的（2026-09-16）：导航守卫那一套来自 PBL 那条线，
 * 「带查询串比较」来自已经上线的教师端那一条。合的时候**守卫里的每一处比较
 * 都要跟着换成带查询串的那种** —— 只换 navigate 开头那一个，
 * `?tab=grading` 仍然进不了 history。 */
const navigationGuards = new Set<() => Promise<void>>();
let navigationRequest = 0;
let committedPath: string | undefined;
const historyIndexKey = "__liteHistoryIndex";

/** 当前地址，带查询串。导航里所有「到了没有」的判断都走它。 */
function currentLocation(): string {
  return window.location.pathname + window.location.search;
}

export function flushNavigationGuards(): Promise<void> {
  return Promise.all([...navigationGuards].map((save) => Promise.resolve().then(save))).then(() => {});
}

/** Keep the editor mounted until a browser history traversal has saved it. */
export function listenForNavigation(onChange: () => void): () => void {
  let index: number = window.history.state?.[historyIndexKey] ?? 0;
  let path = window.location.pathname + window.location.search + window.location.hash;
  let state = { ...window.history.state, [historyIndexKey]: index };
  window.history.replaceState(state, "", path);
  committedPath = currentLocation();
  let restoring: number | null = null;
  const accept = () => {
    index = window.history.state?.[historyIndexKey] ?? 0;
    path = window.location.pathname + window.location.search + window.location.hash;
    state = { ...window.history.state, [historyIndexKey]: index };
    window.history.replaceState(state, "", path);
    committedPath = currentLocation();
    onChange();
  };
  const onPop = () => {
    const targetIndex = window.history.state?.[historyIndexKey];
    if (restoring !== null && targetIndex === restoring) { restoring = null; return; }
    const request = ++navigationRequest;
    if (navigationGuards.size === 0) { accept(); return; }
    void flushNavigationGuards().then(() => {
      if (request === navigationRequest) accept();
    }).catch(() => {
      if (request !== navigationRequest) return;
      if (typeof targetIndex === "number" && targetIndex !== index) {
        restoring = index;
        window.history.go(index - targetIndex);
      } else {
        // Older history entries may predate index tagging. Keep the visible
        // editor and URL together without guessing a traversal direction.
        window.history.replaceState(state, "", path);
      }
    });
  };
  window.addEventListener("popstate", onPop);
  return () => { navigationRequest++; window.removeEventListener("popstate", onPop); };
}

/** Register an active editor's save operation. Rejection keeps its inputs mounted. */
export function beforeNavigate(save: () => Promise<void>): () => void {
  navigationGuards.add(save);
  return () => { navigationGuards.delete(save); };
}

export function navigate(path: string): void {
  const request = ++navigationRequest;
  // 🚨 `committedPath` 只有在装了 listenForNavigation 之后才有值。合并这两条线
  // 的时候（2026-09-16）这里一度写成 `&& committedPath === path`，于是**没装
  // 监听的那些页面**（教师端就是）永远不满足这一条，「已经在这儿了」的 no-op
  // 整个失效，每点一次都多一条 history。没装监听就退回「地址一样就是没动」。
  const settled = committedPath === undefined || committedPath === path;
  if (currentLocation() === path && settled) return;
  const commit = () => {
    if (request !== navigationRequest) return;
    if (currentLocation() !== path) {
      const index = (window.history.state?.[historyIndexKey] ?? 0) + 1;
      window.history.pushState({ [historyIndexKey]: index }, "", path);
    }
    window.dispatchEvent(new PopStateEvent("popstate"));
  };
  if (navigationGuards.size === 0) { commit(); return; }
  void flushNavigationGuards().then(commit).catch(() => { /* The editor displays its save error. */ });
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

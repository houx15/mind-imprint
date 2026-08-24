import type { NavTab } from "./Nav";

// routing — a tiny, dependency-free URL model for the student shell. It maps the
// browser path ⇄ the shell's top-level navigation so a refresh, a copied link,
// or the Back/Forward buttons land on the same page (and, for 项目/课程, the same
// open project/course).
//
// Design notes:
//  - Paths are ALWAYS root-relative (a leading "/"), never absolute URLs — the
//    domain is intentionally never referenced here, so moving to a new domain
//    later needs no code change. Everything works off `location.pathname` + the
//    History API.
//  - The shell has no router library by design; StudentApp drives this module
//    directly. The open-project/open-course ids reuse the shell's existing
//    `pending*` deep-link machinery, so this layer only has to parse/format.
//  - Sub-tabs (评估报告 / 学习记录 / 图鉴) are deliberately NOT in the URL; they
//    default to their first pane on load.

export type AppRoute =
  | { tab: "home" }
  | { tab: "projects"; projectId?: string }
  | { tab: "courses"; slug?: string }
  | { tab: "me" };

/** The tab a route belongs to (handy for callers that only need the top tab). */
export function routeTab(route: AppRoute): NavTab {
  return route.tab;
}

/** Parse a browser pathname into a route. Unknown paths fall back to home, so a
 * stale or hand-typed URL never dead-ends. Trailing slashes and `/index.html`
 * are tolerated; segments are URL-decoded. */
export function parsePath(pathname: string): AppRoute {
  // Normalise: drop a leading slash, a trailing slash, and a trailing
  // "index.html" (nginx serves it at "/", but the address bar may still show it).
  const cleaned = pathname
    .replace(/\/index\.html$/i, "/")
    .replace(/^\/+/, "")
    .replace(/\/+$/, "");
  const segments = cleaned.length === 0 ? [] : cleaned.split("/").map(decodeSegment);
  const [first, second] = segments;

  switch (first) {
    case undefined:
    case "":
    case "home":
      return { tab: "home" };
    case "projects":
      return second ? { tab: "projects", projectId: second } : { tab: "projects" };
    case "courses":
      return second ? { tab: "courses", slug: second } : { tab: "courses" };
    case "me":
      return { tab: "me" };
    default:
      return { tab: "home" };
  }
}

/** The canonical root-relative path for a route. Inverse of `parsePath` on the
 * routes `parsePath` produces. Home is "/". */
export function routePath(route: AppRoute): string {
  switch (route.tab) {
    case "home":
      return "/";
    case "projects":
      return route.projectId ? `/projects/${encodeSegment(route.projectId)}` : "/projects";
    case "courses":
      return route.slug ? `/courses/${encodeSegment(route.slug)}` : "/courses";
    case "me":
      return "/me";
  }
}

function encodeSegment(value: string): string {
  return encodeURIComponent(value);
}

function decodeSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    // A malformed %-escape (e.g. a bare "%") — keep the raw segment rather than
    // throwing, so a garbled URL still resolves (to home, via the switch above).
    return value;
  }
}
